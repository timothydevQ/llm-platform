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
	TaskChat       TaskType = "chat"
	TaskSummarize  TaskType = "summarize"
	TaskEmbed      TaskType = "embed"
	TaskRerank     TaskType = "rerank"
	TaskClassify   TaskType = "classify"
	TaskModerate   TaskType = "moderate"
)

type ModelTier string

const (
	TierSmall  ModelTier = "small"   // fast, cheap
	TierMedium ModelTier = "medium"  // balanced
	TierLarge  ModelTier = "large"   // powerful, expensive
)

type InferenceRequest struct {
	ID            string            `json:"id"`
	TaskType      TaskType          `json:"task_type"`
	Prompt        string            `json:"prompt"`
	Messages      []Message         `json:"messages,omitempty"`
	Documents     []string          `json:"documents,omitempty"`
	Query         string            `json:"query,omitempty"`
	MaxTokens     int               `json:"max_tokens,omitempty"`
	Stream        bool              `json:"stream,omitempty"`
	Priority      int               `json:"priority,omitempty"` // 0=low, 1=normal, 2=high
	CostBudget    string            `json:"cost_budget,omitempty"` // "low", "medium", "high"
	LatencyTarget int               `json:"latency_target_ms,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	ReceivedAt    time.Time         `json:"received_at"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type InferenceResponse struct {
	RequestID    string        `json:"request_id"`
	TaskType     TaskType      `json:"task_type"`
	ModelUsed    string        `json:"model_used"`
	ModelTier    ModelTier     `json:"model_tier"`
	Content      string        `json:"content,omitempty"`
	Embedding    []float64     `json:"embedding,omitempty"`
	Scores       []float64     `json:"scores,omitempty"`
	TokensUsed   int           `json:"tokens_used"`
	LatencyMs    float64       `json:"latency_ms"`
	CachedResult bool          `json:"cached_result"`
	FallbackUsed bool          `json:"fallback_used"`
	Cost         float64       `json:"cost_usd"`
}

// ── Token Bucket Rate Limiter ─────────────────────────────────────────────────

type TokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	maxBurst float64
	rate     float64
	lastFill time.Time
}

func NewTokenBucket(rate, burst float64) *TokenBucket {
	return &TokenBucket{tokens: burst, maxBurst: burst, rate: rate, lastFill: time.Now()}
}

func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(b.lastFill).Seconds()
	b.tokens = math.Min(b.maxBurst, b.tokens+elapsed*b.rate)
	b.lastFill = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*TokenBucket
	rate    float64
	burst   float64
}

func NewRateLimiter(rate, burst float64) *RateLimiter {
	rl := &RateLimiter{buckets: make(map[string]*TokenBucket), rate: rate, burst: burst}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	b, ok := rl.buckets[key]
	if !ok {
		b = NewTokenBucket(rl.rate, rl.burst)
		rl.buckets[key] = b
	}
	rl.mu.Unlock()
	return b.Allow()
}

func (rl *RateLimiter) cleanup() {
	for range time.NewTicker(5 * time.Minute).C {
		rl.mu.Lock()
		for k, b := range rl.buckets {
			b.mu.Lock()
			if time.Since(b.lastFill) > 10*time.Minute {
				delete(rl.buckets, k)
			}
			b.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}

// ── API Key Auth ──────────────────────────────────────────────────────────────

type AuthStore struct {
	mu   sync.RWMutex
	keys map[string]string // key → clientID
}

func NewAuthStore() *AuthStore {
	store := &AuthStore{keys: make(map[string]string)}
	// Pre-register a test key
	store.keys["test-key-1234"] = "client-test"
	store.keys["platform-key-5678"] = "client-platform"
	return store
}

func (a *AuthStore) Validate(key string) (string, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	clientID, ok := a.keys[key]
	return clientID, ok
}

func (a *AuthStore) Register(key, clientID string) {
	a.mu.Lock()
	a.keys[key] = clientID
	a.mu.Unlock()
}

// ── Request Metrics ───────────────────────────────────────────────────────────

type GatewayMetrics struct {
	TotalRequests    int64
	AuthFailures     int64
	RateLimited      int64
	ValidationErrors int64
	Routed           int64
	CacheHits        int64
	Fallbacks        int64
	Errors           int64
	LatencySumMs     int64
}

func (m *GatewayMetrics) snapshot() map[string]int64 {
	return map[string]int64{
		"total_requests":    atomic.LoadInt64(&m.TotalRequests),
		"auth_failures":     atomic.LoadInt64(&m.AuthFailures),
		"rate_limited":      atomic.LoadInt64(&m.RateLimited),
		"validation_errors": atomic.LoadInt64(&m.ValidationErrors),
		"routed":            atomic.LoadInt64(&m.Routed),
		"cache_hits":        atomic.LoadInt64(&m.CacheHits),
		"fallbacks":         atomic.LoadInt64(&m.Fallbacks),
		"errors":            atomic.LoadInt64(&m.Errors),
	}
}

// ── Model Router Client ───────────────────────────────────────────────────────

type RouterClient struct {
	baseURL string
	client  *http.Client
}

func NewRouterClient(baseURL string) *RouterClient {
	return &RouterClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (rc *RouterClient) Route(req *InferenceRequest) (*InferenceResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest("POST", rc.baseURL+"/v1/route", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := rc.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("router unavailable: %w", err)
	}
	defer resp.Body.Close()

	var result InferenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Gateway Handler ───────────────────────────────────────────────────────────

type Gateway struct {
	auth      *AuthStore
	limiter   *RateLimiter
	router    *RouterClient
	metrics   *GatewayMetrics
	startTime time.Time
}

func NewGateway(routerURL string) *Gateway {
	return &Gateway{
		auth:      NewAuthStore(),
		limiter:   NewRateLimiter(50, 100), // 50 req/s, burst 100
		router:    NewRouterClient(routerURL),
		metrics:   &GatewayMetrics{},
		startTime: time.Now(),
	}
}

func (g *Gateway) extractKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.Header.Get("X-API-Key")
}

func (g *Gateway) extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.Split(xff, ",")[0]
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func (g *Gateway) inferTask(r *http.Request) TaskType {
	path := r.URL.Path
	switch {
	case strings.HasSuffix(path, "/chat"):
		return TaskChat
	case strings.HasSuffix(path, "/summarize"):
		return TaskSummarize
	case strings.HasSuffix(path, "/embed"):
		return TaskEmbed
	case strings.HasSuffix(path, "/rerank"):
		return TaskRerank
	case strings.HasSuffix(path, "/classify"):
		return TaskClassify
	case strings.HasSuffix(path, "/moderate"):
		return TaskModerate
	default:
		return TaskChat
	}
}

func (g *Gateway) handleInference(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	atomic.AddInt64(&g.metrics.TotalRequests, 1)

	// Auth
	key := g.extractKey(r)
	clientID, ok := g.auth.Validate(key)
	if !ok {
		atomic.AddInt64(&g.metrics.AuthFailures, 1)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid API key"})
		return
	}

	// Rate limit per client
	if !g.limiter.Allow(clientID) {
		atomic.AddInt64(&g.metrics.RateLimited, 1)
		w.Header().Set("Retry-After", "1")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		return
	}

	// Parse request
	var req InferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		atomic.AddInt64(&g.metrics.ValidationErrors, 1)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Assign request ID and metadata
	req.ID = newID()
	req.ReceivedAt = time.Now()
	if req.TaskType == "" {
		req.TaskType = g.inferTask(r)
	}
	if req.Metadata == nil {
		req.Metadata = make(map[string]string)
	}
	req.Metadata["client_id"] = clientID
	req.Metadata["request_ip"] = g.extractIP(r)

	// Validate
	if err := validateRequest(&req); err != nil {
		atomic.AddInt64(&g.metrics.ValidationErrors, 1)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Handle streaming separately
	if req.Stream {
		g.handleStream(w, r, &req)
		return
	}

	// Route to model router
	resp, err := g.router.Route(&req)
	if err != nil {
		atomic.AddInt64(&g.metrics.Errors, 1)
		slog.Error("Routing failed", "request_id", req.ID, "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error":      "inference service unavailable",
			"request_id": req.ID,
		})
		return
	}

	atomic.AddInt64(&g.metrics.Routed, 1)
	if resp.CachedResult {
		atomic.AddInt64(&g.metrics.CacheHits, 1)
	}
	if resp.FallbackUsed {
		atomic.AddInt64(&g.metrics.Fallbacks, 1)
	}

	resp.LatencyMs = float64(time.Since(start).Milliseconds())
	slog.Info("Request completed",
		"request_id", req.ID,
		"task", req.TaskType,
		"model", resp.ModelUsed,
		"latency_ms", resp.LatencyMs,
		"cached", resp.CachedResult)

	writeJSON(w, http.StatusOK, resp)
}

func (g *Gateway) handleStream(w http.ResponseWriter, r *http.Request, req *InferenceRequest) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Request-ID", req.ID)

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
		return
	}

	// Simulate streaming tokens from model worker
	words := strings.Fields(req.Prompt)
	if len(words) == 0 {
		words = []string{"This", "is", "a", "streaming", "response", "from", "the", "LLM", "platform."}
	}

	tokenCount := 0
	for _, word := range words {
		select {
		case <-r.Context().Done():
			return
		default:
		}
		fmt.Fprintf(w, "data: {\"token\":%q,\"request_id\":%q}\n\n", word+" ", req.ID)
		flusher.Flush()
		tokenCount++
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Fprintf(w, "data: {\"done\":true,\"request_id\":%q,\"tokens_used\":%d}\n\n", req.ID, tokenCount)
	flusher.Flush()
	atomic.AddInt64(&g.metrics.Routed, 1)
}

func (g *Gateway) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"uptime_ms": time.Since(g.startTime).Milliseconds(),
	})
}

func (g *Gateway) liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (g *Gateway) readiness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (g *Gateway) metricsHandler(w http.ResponseWriter, _ *http.Request) {
	snap := g.metrics.snapshot()
	for k, v := range snap {
		fmt.Fprintf(w, "gateway_%s %d\n", k, v)
	}
}

func (g *Gateway) statsHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, g.metrics.snapshot())
}

// ── Validation ────────────────────────────────────────────────────────────────

func validateRequest(req *InferenceRequest) error {
	switch req.TaskType {
	case TaskChat:
		if len(req.Messages) == 0 && req.Prompt == "" {
			return fmt.Errorf("chat requires messages or prompt")
		}
	case TaskSummarize:
		if req.Prompt == "" {
			return fmt.Errorf("summarize requires prompt")
		}
	case TaskEmbed:
		if req.Prompt == "" && req.Query == "" {
			return fmt.Errorf("embed requires prompt or query")
		}
	case TaskRerank:
		if len(req.Documents) == 0 || req.Query == "" {
			return fmt.Errorf("rerank requires documents and query")
		}
	case TaskClassify, TaskModerate:
		if req.Prompt == "" {
			return fmt.Errorf("%s requires prompt", req.TaskType)
		}
	}
	if req.MaxTokens < 0 {
		return fmt.Errorf("max_tokens cannot be negative")
	}
	return nil
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
	routerURL := getEnv("MODEL_ROUTER_URL", "http://model-router:8081")
	gw := NewGateway(routerURL)

	mux := http.NewServeMux()

	// Inference endpoints
	for _, task := range []string{"chat", "summarize", "embed", "rerank", "classify", "moderate"} {
		t := task
		mux.HandleFunc("/v1/"+t, methodHandler(map[string]http.HandlerFunc{"POST": gw.handleInference}))
	}

	// Ops endpoints
	mux.HandleFunc("/v1/stats", methodHandler(map[string]http.HandlerFunc{"GET": gw.statsHandler}))
	mux.HandleFunc("/health", gw.health)
	mux.HandleFunc("/healthz/live", gw.liveness)
	mux.HandleFunc("/healthz/ready", gw.readiness)
	mux.HandleFunc("/metrics", gw.metricsHandler)

	port := getEnv("HTTP_PORT", "8080")
	srv := &http.Server{
		Addr:         net.JoinHostPort("", port),
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		slog.Info("Inference Gateway started", "port", port, "router", routerURL)
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

var _ = math.Pi
// task types
// inference request
// inference response
// token bucket
// bucket allow
// rate limiter
// rl allow
// rl cleanup
// auth store
