package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ── Domain ────────────────────────────────────────────────────────────────────

type TaskType string

const (
	TaskChat      TaskType = "chat"
	TaskSummarize TaskType = "summarize"
	TaskEmbed     TaskType = "embed"
	TaskRerank    TaskType = "rerank"
	TaskClassify  TaskType = "classify"
	TaskModerate  TaskType = "moderate"
)

type InferRequest struct {
	ModelID    string    `json:"model_id"`
	TaskType   TaskType  `json:"task_type"`
	Prompt     string    `json:"prompt"`
	Messages   []Message `json:"messages,omitempty"`
	Documents  []string  `json:"documents,omitempty"`
	Query      string    `json:"query,omitempty"`
	MaxTokens  int       `json:"max_tokens,omitempty"`
	RequestID  string    `json:"request_id"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type InferResponse struct {
	RequestID  string    `json:"request_id"`
	ModelID    string    `json:"model_id"`
	TaskType   TaskType  `json:"task_type"`
	Content    string    `json:"content,omitempty"`
	Embedding  []float64 `json:"embedding,omitempty"`
	Scores     []float64 `json:"scores,omitempty"`
	TokensUsed int       `json:"tokens_used"`
	LatencyMs  float64   `json:"latency_ms"`
}

// ── Model Config ──────────────────────────────────────────────────────────────

type ModelConfig struct {
	ID           string
	EmbedDim     int
	AvgLatencyMs int
	TokensPerMs  float64 // tokens generated per ms
}

var modelConfigs = map[string]*ModelConfig{
	"gpt-small":  {ID: "gpt-small", EmbedDim: 768, AvgLatencyMs: 200, TokensPerMs: 0.5},
	"gpt-medium": {ID: "gpt-medium", EmbedDim: 1024, AvgLatencyMs: 500, TokensPerMs: 0.3},
	"gpt-large":  {ID: "gpt-large", EmbedDim: 1536, AvgLatencyMs: 1200, TokensPerMs: 0.15},
	"embed-v2":   {ID: "embed-v2", EmbedDim: 1536, AvgLatencyMs: 50, TokensPerMs: 2.0},
	"rerank-v1":  {ID: "rerank-v1", EmbedDim: 768, AvgLatencyMs: 100, TokensPerMs: 1.0},
}

// ── Worker ────────────────────────────────────────────────────────────────────

type WorkerStatus int

const (
	WorkerHealthy  WorkerStatus = iota
	WorkerDegraded              // slow but available
	WorkerDown                  // unavailable
)

type Worker struct {
	mu           sync.RWMutex
	status       WorkerStatus
	latencyJitter float64 // multiplier on base latency (1.0 = normal)
	metrics      *WorkerMetrics
}

type WorkerMetrics struct {
	Requests     int64
	Tokens       int64
	Errors       int64
	TotalLatency int64 // ms
}

func (m *WorkerMetrics) AvgLatencyMs() float64 {
	reqs := atomic.LoadInt64(&m.Requests)
	if reqs == 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&m.TotalLatency)) / float64(reqs)
}

func (m *WorkerMetrics) TokensPerSec() float64 {
	latency := atomic.LoadInt64(&m.TotalLatency)
	if latency == 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&m.Tokens)) / float64(latency) * 1000
}

func NewWorker() *Worker {
	return &Worker{
		status:       WorkerHealthy,
		latencyJitter: 1.0,
		metrics:      &WorkerMetrics{},
	}
}

func (w *Worker) SetStatus(s WorkerStatus) {
	w.mu.Lock()
	w.status = s
	switch s {
	case WorkerDegraded:
		w.latencyJitter = 3.0
	case WorkerDown:
		w.latencyJitter = 0
	default:
		w.latencyJitter = 1.0
	}
	w.mu.Unlock()
	slog.Info("Worker status changed", "status", s)
}

func (w *Worker) SetJitter(j float64) {
	w.mu.Lock()
	w.latencyJitter = j
	w.mu.Unlock()
}

func (w *Worker) Infer(req *InferRequest) (*InferResponse, error) {
	w.mu.RLock()
	status := w.status
	jitter := w.latencyJitter
	w.mu.RUnlock()

	if status == WorkerDown {
		atomic.AddInt64(&w.metrics.Errors, 1)
		return nil, fmt.Errorf("worker is down")
	}

	cfg, ok := modelConfigs[req.ModelID]
	if !ok {
		// Default to small config
		cfg = modelConfigs["gpt-small"]
	}

	start := time.Now()

	// Simulate inference latency
	latency := time.Duration(float64(cfg.AvgLatencyMs)*jitter) * time.Millisecond
	time.Sleep(latency)

	resp := &InferResponse{
		RequestID: req.RequestID,
		ModelID:   req.ModelID,
		TaskType:  req.TaskType,
	}

	switch req.TaskType {
	case TaskChat, TaskSummarize:
		resp.Content = generateChatResponse(req)
		resp.TokensUsed = estimateTokens(resp.Content)

	case TaskEmbed:
		text := req.Prompt
		if text == "" {
			text = req.Query
		}
		resp.Embedding = generateEmbedding(text, cfg.EmbedDim)
		resp.TokensUsed = estimateTokens(text)

	case TaskRerank:
		resp.Scores = rerankDocuments(req.Query, req.Documents)
		resp.TokensUsed = estimateTokens(req.Query) + len(req.Documents)*20

	case TaskClassify, TaskModerate:
		resp.Content = classifyText(req.Prompt)
		resp.TokensUsed = estimateTokens(req.Prompt)
	}

	resp.LatencyMs = float64(time.Since(start).Milliseconds())

	atomic.AddInt64(&w.metrics.Requests, 1)
	atomic.AddInt64(&w.metrics.Tokens, int64(resp.TokensUsed))
	atomic.AddInt64(&w.metrics.TotalLatency, int64(resp.LatencyMs))

	return resp, nil
}

// ── Inference Helpers ─────────────────────────────────────────────────────────

func generateChatResponse(req *InferRequest) string {
	prompt := req.Prompt
	if len(req.Messages) > 0 {
		prompt = req.Messages[len(req.Messages)-1].Content
	}
	if len(prompt) > 50 {
		prompt = prompt[:50] + "..."
	}
	return fmt.Sprintf("Response to: %q — Generated by model %s at %s",
		prompt, req.ModelID, time.Now().Format(time.RFC3339))
}

func generateEmbedding(text string, dim int) []float64 {
	// Deterministic pseudo-embedding based on text hash
	embedding := make([]float64, dim)
	hash := 0
	for _, c := range text {
		hash = hash*31 + int(c)
	}
	for i := range embedding {
		val := math.Sin(float64(hash+i)) * math.Cos(float64(hash*i+1))
		embedding[i] = val
	}
	// L2 normalize
	var norm float64
	for _, v := range embedding {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}
	return embedding[:min(dim, 8)] // return first 8 dims for compactness
}

func rerankDocuments(query string, documents []string) []float64 {
	scores := make([]float64, len(documents))
	for i, doc := range documents {
		// Simple relevance score: ratio of query words found in doc
		queryWords := strings.Fields(strings.ToLower(query))
		docLower := strings.ToLower(doc)
		matches := 0
		for _, w := range queryWords {
			if strings.Contains(docLower, w) {
				matches++
			}
		}
		if len(queryWords) > 0 {
			scores[i] = float64(matches) / float64(len(queryWords))
		}
	}
	return scores
}

func classifyText(text string) string {
	lower := strings.ToLower(text)
	if strings.ContainsAny(lower, "hate violence harm threat") {
		return `{"label":"harmful","confidence":0.92}`
	}
	if strings.ContainsAny(lower, "great awesome excellent wonderful") {
		return `{"label":"positive","confidence":0.87}`
	}
	if strings.ContainsAny(lower, "bad terrible horrible awful") {
		return `{"label":"negative","confidence":0.85}`
	}
	return `{"label":"neutral","confidence":0.75}`
}

func estimateTokens(text string) int {
	// ~4 chars per token approximation
	tokens := len(text) / 4
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── HTTP Handler ──────────────────────────────────────────────────────────────

type handler struct{ worker *Worker }

func (h *handler) infer(w http.ResponseWriter, r *http.Request) {
	var req InferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.TaskType == "" {
		req.TaskType = TaskChat
	}
	if req.RequestID == "" {
		req.RequestID = newID()
	}

	resp, err := h.worker.Infer(&req)
	if err != nil {
		slog.Error("Inference failed", "model", req.ModelID, "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error":      err.Error(),
			"model_id":   req.ModelID,
			"request_id": req.RequestID,
		})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *handler) setStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status string  `json:"status"`
		Jitter float64 `json:"jitter,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	switch req.Status {
	case "healthy":
		h.worker.SetStatus(WorkerHealthy)
	case "degraded":
		h.worker.SetStatus(WorkerDegraded)
	case "down":
		h.worker.SetStatus(WorkerDown)
	}
	if req.Jitter > 0 {
		h.worker.SetJitter(req.Jitter)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": req.Status})
}

func (h *handler) stats(w http.ResponseWriter, _ *http.Request) {
	m := h.worker.metrics
	h.worker.mu.RLock()
	status := h.worker.status
	jitter := h.worker.latencyJitter
	h.worker.mu.RUnlock()

	statusStr := "healthy"
	if status == WorkerDegraded { statusStr = "degraded" }
	if status == WorkerDown { statusStr = "down" }

	writeJSON(w, http.StatusOK, map[string]any{
		"status":            statusStr,
		"latency_jitter":    jitter,
		"requests":          atomic.LoadInt64(&m.Requests),
		"tokens_generated":  atomic.LoadInt64(&m.Tokens),
		"errors":            atomic.LoadInt64(&m.Errors),
		"avg_latency_ms":    m.AvgLatencyMs(),
		"tokens_per_second": m.TokensPerSec(),
	})
}

func (h *handler) liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (h *handler) readiness(w http.ResponseWriter, r *http.Request) {
	h.worker.mu.RLock()
	status := h.worker.status
	h.worker.mu.RUnlock()
	if status == WorkerDown {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "down"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *handler) metricsHandler(w http.ResponseWriter, _ *http.Request) {
	m := h.worker.metrics
	fmt.Fprintf(w, "worker_requests_total %d\n", atomic.LoadInt64(&m.Requests))
	fmt.Fprintf(w, "worker_tokens_total %d\n", atomic.LoadInt64(&m.Tokens))
	fmt.Fprintf(w, "worker_errors_total %d\n", atomic.LoadInt64(&m.Errors))
	fmt.Fprintf(w, "worker_avg_latency_ms %f\n", m.AvgLatencyMs())
	fmt.Fprintf(w, "worker_tokens_per_second %f\n", m.TokensPerSec())
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func methodHandler(handlers map[string]http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h, ok := handlers[strings.ToUpper(r.Method)]; ok {
			h(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	worker := NewWorker()
	h := &handler{worker: worker}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/infer", methodHandler(map[string]http.HandlerFunc{"POST": h.infer}))
	mux.HandleFunc("/v1/status", methodHandler(map[string]http.HandlerFunc{"POST": h.setStatus}))
	mux.HandleFunc("/v1/stats", methodHandler(map[string]http.HandlerFunc{"GET": h.stats}))
	mux.HandleFunc("/healthz/live", h.liveness)
	mux.HandleFunc("/healthz/ready", h.readiness)
	mux.HandleFunc("/metrics", h.metricsHandler)

	port := getEnv("HTTP_PORT", "8083")
	srv := &http.Server{
		Addr:         net.JoinHostPort("", port),
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		slog.Info("Worker Simulator started", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
// task types
// infer request
// infer response
// model config
// model configs
// worker status
// worker struct
