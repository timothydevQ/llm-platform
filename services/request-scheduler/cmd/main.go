package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
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
)

type QueuedRequest struct {
	ID          string
	TaskType    TaskType
	Prompt      string
	Priority    int       // 0=low, 1=normal, 2=high
	EnqueuedAt  time.Time
	ResponseCh  chan *BatchResult
}

type BatchResult struct {
	RequestID string
	Content   string
	Error     error
	LatencyMs float64
}

type Batch struct {
	ID       string
	Requests []*QueuedRequest
	TaskType TaskType
	FormedAt time.Time
}

// ── Adaptive Batcher ──────────────────────────────────────────────────────────

type BatcherConfig struct {
	MaxBatchSize    int
	MaxWaitMs       int     // max time to wait for more requests
	MinWaitMs       int     // min time before dispatching
	LatencyTarget   int     // p99 latency target in ms
}

func DefaultBatcherConfig() BatcherConfig {
	return BatcherConfig{
		MaxBatchSize:  16,
		MaxWaitMs:     30,
		MinWaitMs:     5,
		LatencyTarget: 500,
	}
}

type AdaptiveBatcher struct {
	mu          sync.Mutex
	cfg         BatcherConfig
	queues      map[TaskType][]*QueuedRequest
	metrics     *SchedulerMetrics
	dispatchFn  func(*Batch) []*BatchResult
}

func NewAdaptiveBatcher(cfg BatcherConfig, dispatchFn func(*Batch) []*BatchResult) *AdaptiveBatcher {
	ab := &AdaptiveBatcher{
		cfg:        cfg,
		queues:     make(map[TaskType][]*QueuedRequest),
		metrics:    &SchedulerMetrics{},
		dispatchFn: dispatchFn,
	}
	go ab.batchLoop()
	return ab
}

func (ab *AdaptiveBatcher) Enqueue(req *QueuedRequest) chan *BatchResult {
	req.ResponseCh = make(chan *BatchResult, 1)
	ab.mu.Lock()
	ab.queues[req.TaskType] = append(ab.queues[req.TaskType], req)
	qLen := len(ab.queues[req.TaskType])
	ab.mu.Unlock()
	atomic.AddInt64(&ab.metrics.Enqueued, 1)

	// If queue is full, trigger immediate dispatch
	if qLen >= ab.cfg.MaxBatchSize {
		go ab.dispatchTask(req.TaskType)
	}

	return req.ResponseCh
}

func (ab *AdaptiveBatcher) batchLoop() {
	ticker := time.NewTicker(time.Duration(ab.cfg.MaxWaitMs) * time.Millisecond)
	for range ticker.C {
		ab.mu.Lock()
		tasks := make([]TaskType, 0, len(ab.queues))
		for task, reqs := range ab.queues {
			if len(reqs) > 0 {
				tasks = append(tasks, task)
			}
		}
		ab.mu.Unlock()
		for _, task := range tasks {
			ab.dispatchTask(task)
		}
	}
}

func (ab *AdaptiveBatcher) dispatchTask(task TaskType) {
	ab.mu.Lock()
	reqs := ab.queues[task]
	if len(reqs) == 0 {
		ab.mu.Unlock()
		return
	}
	// Take up to MaxBatchSize requests
	batchSize := len(reqs)
	if batchSize > ab.cfg.MaxBatchSize {
		batchSize = ab.cfg.MaxBatchSize
	}
	batch := reqs[:batchSize]
	ab.queues[task] = reqs[batchSize:]
	ab.mu.Unlock()

	b := &Batch{
		ID:       newID(),
		Requests: batch,
		TaskType: task,
		FormedAt: time.Now(),
	}

	start := time.Now()
	results := ab.dispatchFn(b)
	latency := time.Since(start).Milliseconds()

	// Deliver results
	for i, res := range results {
		if i < len(batch) {
			res.LatencyMs = float64(latency)
			batch[i].ResponseCh <- res
		}
	}

	atomic.AddInt64(&ab.metrics.BatchesDispatched, 1)
	atomic.AddInt64(&ab.metrics.RequestsProcessed, int64(len(batch)))

	// Update avg batch size
	ab.metrics.mu.Lock()
	ab.metrics.TotalBatchedItems += int64(len(batch))
	ab.metrics.TotalBatches++
	ab.metrics.mu.Unlock()

	slog.Info("Batch dispatched",
		"batch_id", b.ID,
		"task", task,
		"size", len(batch),
		"latency_ms", latency)
}

func (ab *AdaptiveBatcher) QueueDepths() map[string]int {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	out := make(map[string]int)
	for task, reqs := range ab.queues {
		out[string(task)] = len(reqs)
	}
	return out
}

// ── Priority Queue ────────────────────────────────────────────────────────────

type PriorityQueue struct {
	mu       sync.Mutex
	high     []*QueuedRequest
	normal   []*QueuedRequest
	low      []*QueuedRequest
	maxSize  int
}

func NewPriorityQueue(maxSize int) *PriorityQueue {
	return &PriorityQueue{maxSize: maxSize}
}

func (pq *PriorityQueue) Enqueue(req *QueuedRequest) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	total := len(pq.high) + len(pq.normal) + len(pq.low)
	if total >= pq.maxSize {
		return false // queue full — load shedding
	}
	switch req.Priority {
	case 2:
		pq.high = append(pq.high, req)
	case 1:
		pq.normal = append(pq.normal, req)
	default:
		pq.low = append(pq.low, req)
	}
	return true
}

func (pq *PriorityQueue) Dequeue() *QueuedRequest {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	if len(pq.high) > 0 {
		req := pq.high[0]
		pq.high = pq.high[1:]
		return req
	}
	if len(pq.normal) > 0 {
		req := pq.normal[0]
		pq.normal = pq.normal[1:]
		return req
	}
	if len(pq.low) > 0 {
		req := pq.low[0]
		pq.low = pq.low[1:]
		return req
	}
	return nil
}

func (pq *PriorityQueue) Len() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return len(pq.high) + len(pq.normal) + len(pq.low)
}

func (pq *PriorityQueue) LenByPriority() (high, normal, low int) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return len(pq.high), len(pq.normal), len(pq.low)
}

// ── Scheduler Metrics ─────────────────────────────────────────────────────────

type SchedulerMetrics struct {
	Enqueued          int64
	BatchesDispatched int64
	RequestsProcessed int64
	LoadShedded       int64
	TotalBatchedItems int64
	TotalBatches      int64
	mu                sync.Mutex
}

func (m *SchedulerMetrics) AvgBatchSize() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.TotalBatches == 0 {
		return 0
	}
	return float64(m.TotalBatchedItems) / float64(m.TotalBatches)
}

// ── Scheduler Service ─────────────────────────────────────────────────────────

type SchedulerService struct {
	batcher  *AdaptiveBatcher
	pq       *PriorityQueue
	metrics  *SchedulerMetrics
}

func NewSchedulerService() *SchedulerService {
	metrics := &SchedulerMetrics{}

	dispatchFn := func(batch *Batch) []*BatchResult {
		results := make([]*BatchResult, len(batch.Requests))
		for i, req := range batch.Requests {
			// Simulate inference — in production this calls the model router
			results[i] = &BatchResult{
				RequestID: req.ID,
				Content:   fmt.Sprintf("Batch response for: %s", req.Prompt),
			}
		}
		return results
	}

	batcher := NewAdaptiveBatcher(DefaultBatcherConfig(), dispatchFn)
	batcher.metrics = metrics

	return &SchedulerService{
		batcher: batcher,
		pq:      NewPriorityQueue(10000),
		metrics: metrics,
	}
}

func (s *SchedulerService) Submit(req *QueuedRequest) (*BatchResult, error) {
	if req.ID == "" {
		req.ID = newID()
	}
	req.EnqueuedAt = time.Now()

	// Priority queue for ordering
	if !s.pq.Enqueue(req) {
		atomic.AddInt64(&s.metrics.LoadShedded, 1)
		return nil, fmt.Errorf("queue full — request load shedded")
	}

	// Batcher for throughput
	respCh := s.batcher.Enqueue(req)

	select {
	case result := <-respCh:
		return result, result.Error
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timed out in scheduler")
	}
}

func (s *SchedulerService) Stats() map[string]any {
	high, normal, low := s.pq.LenByPriority()
	return map[string]any{
		"enqueued":           atomic.LoadInt64(&s.metrics.Enqueued),
		"batches_dispatched": atomic.LoadInt64(&s.metrics.BatchesDispatched),
		"requests_processed": atomic.LoadInt64(&s.metrics.RequestsProcessed),
		"load_shedded":       atomic.LoadInt64(&s.metrics.LoadShedded),
		"avg_batch_size":     s.metrics.AvgBatchSize(),
		"queue_depth": map[string]int{
			"high":   high,
			"normal": normal,
			"low":    low,
			"total":  high + normal + low,
		},
		"queue_depths_by_task": s.batcher.QueueDepths(),
	}
}

// ── HTTP Handler ──────────────────────────────────────────────────────────────

type handler struct{ svc *SchedulerService }

func (h *handler) submit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt   string   `json:"prompt"`
		TaskType TaskType `json:"task_type"`
		Priority int      `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.Prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt required"})
		return
	}
	if req.TaskType == "" {
		req.TaskType = TaskChat
	}

	qr := &QueuedRequest{
		Prompt:   req.Prompt,
		TaskType: req.TaskType,
		Priority: req.Priority,
	}

	result, err := h.svc.Submit(qr)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) stats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.Stats())
}

func (h *handler) liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (h *handler) readiness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *handler) metricsHandler(w http.ResponseWriter, _ *http.Request) {
	stats := h.svc.Stats()
	fmt.Fprintf(w, "scheduler_enqueued %v\n", stats["enqueued"])
	fmt.Fprintf(w, "scheduler_batches_dispatched %v\n", stats["batches_dispatched"])
	fmt.Fprintf(w, "scheduler_requests_processed %v\n", stats["requests_processed"])
	fmt.Fprintf(w, "scheduler_load_shedded %v\n", stats["load_shedded"])
	fmt.Fprintf(w, "scheduler_avg_batch_size %v\n", stats["avg_batch_size"])
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
	svc := NewSchedulerService()
	h := &handler{svc: svc}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/submit", methodHandler(map[string]http.HandlerFunc{"POST": h.submit}))
	mux.HandleFunc("/v1/stats", methodHandler(map[string]http.HandlerFunc{"GET": h.stats}))
	mux.HandleFunc("/healthz/live", h.liveness)
	mux.HandleFunc("/healthz/ready", h.readiness)
	mux.HandleFunc("/metrics", h.metricsHandler)

	port := getEnv("HTTP_PORT", "8082")
	srv := &http.Server{
		Addr:         net.JoinHostPort("", port),
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		slog.Info("Request Scheduler started", "port", port)
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
// queued request
// batch result
// batch struct
// batcher config
// default config
// adaptive batcher
// enqueue
// batch loop
// dispatch task
// immediate dispatch
// queue depths
// priority queue
// pq enqueue
// pq dequeue
