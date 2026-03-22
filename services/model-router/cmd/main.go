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

// ── Domain (shared with gateway) ──────────────────────────────────────────────

type TaskType string

const (
	TaskChat      TaskType = "chat"
	TaskSummarize TaskType = "summarize"
	TaskEmbed     TaskType = "embed"
	TaskRerank    TaskType = "rerank"
	TaskClassify  TaskType = "classify"
	TaskModerate  TaskType = "moderate"
)

type ModelTier string

const (
	TierSmall  ModelTier = "small"
	TierMedium ModelTier = "medium"
	TierLarge  ModelTier = "large"
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
	Priority      int               `json:"priority,omitempty"`
	CostBudget    string            `json:"cost_budget,omitempty"`
	LatencyTarget int               `json:"latency_target_ms,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	ReceivedAt    time.Time         `json:"received_at"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type InferenceResponse struct {
	RequestID    string    `json:"request_id"`
	TaskType     TaskType  `json:"task_type"`
	ModelUsed    string    `json:"model_used"`
	ModelTier    ModelTier `json:"model_tier"`
	Content      string    `json:"content,omitempty"`
	Embedding    []float64 `json:"embedding,omitempty"`
	Scores       []float64 `json:"scores,omitempty"`
	TokensUsed   int       `json:"tokens_used"`
	LatencyMs    float64   `json:"latency_ms"`
	CachedResult bool      `json:"cached_result"`
	FallbackUsed bool      `json:"fallback_used"`
	Cost         float64   `json:"cost_usd"`
}

// ── Model Definition ──────────────────────────────────────────────────────────

type Model struct {
	ID           string
	Name         string
	Tier         ModelTier
	Tasks        []TaskType
	CostPer1kTok float64  // USD per 1k tokens
	MaxTokens    int
	AvgLatencyMs float64
	WorkerURL    string
}

var modelRegistry = []*Model{
	{
		ID: "gpt-small", Name: "GPT-Small", Tier: TierSmall,
		Tasks:        []TaskType{TaskChat, TaskSummarize, TaskClassify, TaskModerate},
		CostPer1kTok: 0.0002, MaxTokens: 4096, AvgLatencyMs: 200,
		WorkerURL: "http://worker-simulator:8083",
	},
	{
		ID: "gpt-medium", Name: "GPT-Medium", Tier: TierMedium,
		Tasks:        []TaskType{TaskChat, TaskSummarize, TaskClassify, TaskModerate},
		CostPer1kTok: 0.002, MaxTokens: 8192, AvgLatencyMs: 500,
		WorkerURL: "http://worker-simulator:8083",
	},
	{
		ID: "gpt-large", Name: "GPT-Large", Tier: TierLarge,
		Tasks:        []TaskType{TaskChat, TaskSummarize},
		CostPer1kTok: 0.02, MaxTokens: 32768, AvgLatencyMs: 1200,
		WorkerURL: "http://worker-simulator:8083",
	},
	{
		ID: "embed-v2", Name: "Embed-v2", Tier: TierSmall,
		Tasks:        []TaskType{TaskEmbed},
		CostPer1kTok: 0.0001, MaxTokens: 8192, AvgLatencyMs: 50,
		WorkerURL: "http://worker-simulator:8083",
	},
	{
		ID: "rerank-v1", Name: "Rerank-v1", Tier: TierSmall,
		Tasks:        []TaskType{TaskRerank},
		CostPer1kTok: 0.0002, MaxTokens: 4096, AvgLatencyMs: 100,
		WorkerURL: "http://worker-simulator:8083",
	},
}

// ── Circuit Breaker ───────────────────────────────────────────────────────────

type CBState int

const (
	CBClosed   CBState = iota
	CBOpen
	CBHalfOpen
)

type CircuitBreaker struct {
	mu          sync.Mutex
	state       CBState
	failures    int
	successes   int
	threshold   int
	successReq  int
	timeout     time.Duration
	lastFailure time.Time
	modelID     string
}

func NewCircuitBreaker(modelID string) *CircuitBreaker {
	return &CircuitBreaker{
		modelID:    modelID,
		threshold:  3,
		successReq: 2,
		timeout:    20 * time.Second,
		state:      CBClosed,
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case CBOpen:
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.state = CBHalfOpen
			cb.successes = 0
			return true
		}
		return false
	default:
		return true
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	if cb.state == CBHalfOpen {
		cb.successes++
		if cb.successes >= cb.successReq {
			cb.state = CBClosed
			slog.Info("Circuit breaker closed", "model", cb.modelID)
		}
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.successes = 0
	cb.lastFailure = time.Now()
	if cb.state == CBHalfOpen || cb.failures >= cb.threshold {
		cb.state = CBOpen
		slog.Warn("Circuit breaker opened", "model", cb.modelID)
	}
}

func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case CBOpen:
		return "open"
	case CBHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

// ── Canary Deployment ─────────────────────────────────────────────────────────

type CanaryConfig struct {
	mu          sync.RWMutex
	primary     string  // primary model ID
	canary      string  // canary model ID
	trafficPct  float64 // 0.0-1.0 fraction to canary
	enabled     bool
}

func NewCanaryConfig() *CanaryConfig {
	return &CanaryConfig{}
}

func (c *CanaryConfig) Configure(primary, canary string, pct float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.primary = primary
	c.canary = canary
	c.trafficPct = pct
	c.enabled = true
	slog.Info("Canary configured", "primary", primary, "canary", canary, "pct", pct)
}

func (c *CanaryConfig) Disable() {
	c.mu.Lock()
	c.enabled = false
	c.mu.Unlock()
}

func (c *CanaryConfig) ShouldUseCanary(modelID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.enabled || modelID != c.primary {
		return false
	}
	return pseudoRand() < c.trafficPct
}

func (c *CanaryConfig) GetCanary() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.canary
}

// ── Routing Engine ────────────────────────────────────────────────────────────

type RoutingDecision struct {
	Model        *Model
	Reason       string
	IsFallback   bool
	IsCanary     bool
}

type Router struct {
	models   []*Model
	breakers map[string]*CircuitBreaker
	canary   *CanaryConfig
	mu       sync.RWMutex
	metrics  *RouterMetrics
}

type RouterMetrics struct {
	Routed         int64
	Fallbacks      int64
	CBRejections   int64
	CostSaved      float64 // USD saved by routing to smaller models
	CanaryRequests int64
	mu             sync.Mutex
}

func NewRouter() *Router {
	breakers := make(map[string]*CircuitBreaker)
	for _, m := range modelRegistry {
		breakers[m.ID] = NewCircuitBreaker(m.ID)
	}
	return &Router{
		models:   modelRegistry,
		breakers: breakers,
		canary:   NewCanaryConfig(),
		metrics:  &RouterMetrics{},
	}
}

func (r *Router) modelsForTask(task TaskType) []*Model {
	var out []*Model
	for _, m := range r.models {
		for _, t := range m.Tasks {
			if t == task {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// selectTier decides which tier to use based on request characteristics
func selectTier(req *InferenceRequest) ModelTier {
	// Cost budget override
	switch req.CostBudget {
	case "low":
		return TierSmall
	case "high":
		return TierLarge
	}

	// Latency target
	if req.LatencyTarget > 0 && req.LatencyTarget < 300 {
		return TierSmall // need fast response
	}

	// Complexity heuristic: longer prompts → larger model
	promptLen := len(req.Prompt)
	for _, msg := range req.Messages {
		promptLen += len(msg.Content)
	}
	switch {
	case promptLen > 2000:
		return TierLarge
	case promptLen > 500:
		return TierMedium
	default:
		return TierSmall
	}
}

func (r *Router) Route(req *InferenceRequest) (*RoutingDecision, error) {
	candidates := r.modelsForTask(req.TaskType)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no models available for task %s", req.TaskType)
	}

	targetTier := selectTier(req)

	// Find best model at target tier
	for _, model := range candidates {
		if model.Tier != targetTier {
			continue
		}
		cb := r.breakers[model.ID]
		if !cb.Allow() {
			atomic.AddInt64(&r.metrics.CBRejections, 1)
			continue
		}

		// Check canary
		if r.canary.ShouldUseCanary(model.ID) {
			canaryID := r.canary.GetCanary()
			for _, cm := range r.models {
				if cm.ID == canaryID && r.breakers[cm.ID].Allow() {
					atomic.AddInt64(&r.metrics.CanaryRequests, 1)
					return &RoutingDecision{Model: cm, Reason: "canary", IsCanary: true}, nil
				}
			}
		}

		return &RoutingDecision{Model: model, Reason: fmt.Sprintf("tier=%s", targetTier)}, nil
	}

	// Fallback: try any available model for this task
	for _, model := range candidates {
		if !r.breakers[model.ID].Allow() {
			continue
		}
		atomic.AddInt64(&r.metrics.Fallbacks, 1)
		slog.Warn("Fallback routing", "request_id", req.ID, "model", model.ID, "task", req.TaskType)
		return &RoutingDecision{Model: model, Reason: "fallback", IsFallback: true}, nil
	}

	return nil, fmt.Errorf("all models for task %s are unavailable (circuit breakers open)", req.TaskType)
}

func (r *Router) RecordSuccess(modelID string) {
	r.mu.RLock()
	cb := r.breakers[modelID]
	r.mu.RUnlock()
	if cb != nil {
		cb.RecordSuccess()
	}
}

func (r *Router) RecordFailure(modelID string) {
	r.mu.RLock()
	cb := r.breakers[modelID]
	r.mu.RUnlock()
	if cb != nil {
		cb.RecordFailure()
	}
}

func (r *Router) CircuitBreakerStates() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]string)
	for id, cb := range r.breakers {
		out[id] = cb.State()
	}
	return out
}

// ── Worker Client ─────────────────────────────────────────────────────────────

type WorkerClient struct {
	client *http.Client
}

func NewWorkerClient() *WorkerClient {
	return &WorkerClient{client: &http.Client{Timeout: 30 * time.Second}}
}

func (wc *WorkerClient) Infer(model *Model, req *InferenceRequest) (*InferenceResponse, error) {
	body, _ := json.Marshal(map[string]any{
		"model_id":   model.ID,
		"task_type":  req.TaskType,
		"prompt":     req.Prompt,
		"messages":   req.Messages,
		"documents":  req.Documents,
		"query":      req.Query,
		"max_tokens": req.MaxTokens,
		"request_id": req.ID,
	})
	httpReq, _ := http.NewRequest("POST", model.WorkerURL+"/v1/infer", strings.NewReader(string(body)))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := wc.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("worker %s unavailable: %w", model.ID, err)
	}
	defer resp.Body.Close()

	var result InferenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Cache Client ──────────────────────────────────────────────────────────────

type CacheClient struct {
	baseURL string
	client  *http.Client
}

func NewCacheClient(baseURL string) *CacheClient {
	return &CacheClient{baseURL: baseURL, client: &http.Client{Timeout: 2 * time.Second}}
}

func (cc *CacheClient) Get(key string) (*InferenceResponse, bool) {
	resp, err := cc.client.Get(cc.baseURL + "/v1/cache?key=" + key)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, false
	}
	defer resp.Body.Close()
	var result InferenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false
	}
	return &result, true
}

func (cc *CacheClient) Set(key string, resp *InferenceResponse) {
	body, _ := json.Marshal(map[string]any{"key": key, "value": resp})
	httpReq, _ := http.NewRequest("POST", cc.baseURL+"/v1/cache", strings.NewReader(string(body)))
	httpReq.Header.Set("Content-Type", "application/json")
	cc.client.Do(httpReq)
}

// ── Router Handler ────────────────────────────────────────────────────────────

type handler struct {
	router *Router
	worker *WorkerClient
	cache  *CacheClient
}

func cacheKey(req *InferenceRequest) string {
	return fmt.Sprintf("%s:%s", req.TaskType, req.Prompt+req.Query)
}

func (h *handler) route(w http.ResponseWriter, r *http.Request) {
	var req InferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	start := time.Now()

	// Cache lookup
	key := cacheKey(&req)
	if cached, ok := h.cache.Get(key); ok {
		cached.CachedResult = true
		cached.LatencyMs = float64(time.Since(start).Milliseconds())
		slog.Info("Cache hit", "request_id", req.ID, "task", req.TaskType)
		writeJSON(w, http.StatusOK, cached)
		return
	}

	// Route
	decision, err := h.router.Route(&req)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	// Call worker
	resp, err := h.worker.Infer(decision.Model, &req)
	if err != nil {
		h.router.RecordFailure(decision.Model.ID)
		slog.Error("Worker inference failed", "model", decision.Model.ID, "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error":   "model worker unavailable",
			"model":   decision.Model.ID,
			"request_id": req.ID,
		})
		return
	}

	h.router.RecordSuccess(decision.Model.ID)
	atomic.AddInt64(&h.router.metrics.Routed, 1)

	resp.RequestID = req.ID
	resp.TaskType = req.TaskType
	resp.ModelUsed = decision.Model.ID
	resp.ModelTier = decision.Model.Tier
	resp.FallbackUsed = decision.IsFallback
	resp.LatencyMs = float64(time.Since(start).Milliseconds())

	// Estimate cost
	if resp.TokensUsed > 0 {
		resp.Cost = float64(resp.TokensUsed) / 1000.0 * decision.Model.CostPer1kTok
	}

	// Cache the result for deterministic tasks
	if req.TaskType == TaskEmbed || req.TaskType == TaskRerank || req.TaskType == TaskClassify {
		h.cache.Set(key, resp)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *handler) getModels(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"models":           modelRegistry,
		"circuit_breakers": h.router.CircuitBreakerStates(),
	})
}

func (h *handler) configureCanary(w http.ResponseWriter, r *http.Request) {
	var cfg struct {
		Primary    string  `json:"primary"`
		Canary     string  `json:"canary"`
		TrafficPct float64 `json:"traffic_pct"`
	}
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid config"})
		return
	}
	h.router.canary.Configure(cfg.Primary, cfg.Canary, cfg.TrafficPct)
	writeJSON(w, http.StatusOK, map[string]string{"status": "canary configured"})
}

func (h *handler) stats(w http.ResponseWriter, _ *http.Request) {
	m := h.router.metrics
	writeJSON(w, http.StatusOK, map[string]any{
		"routed":          atomic.LoadInt64(&m.Routed),
		"fallbacks":       atomic.LoadInt64(&m.Fallbacks),
		"cb_rejections":   atomic.LoadInt64(&m.CBRejections),
		"canary_requests": atomic.LoadInt64(&m.CanaryRequests),
		"circuit_breakers": h.router.CircuitBreakerStates(),
	})
}

func (h *handler) liveness(w http.ResponseWriter, _ *http.Request)  { writeJSON(w, http.StatusOK, map[string]string{"status": "alive"}) }
func (h *handler) readiness(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, map[string]string{"status": "ready"}) }
func (h *handler) metricsHandler(w http.ResponseWriter, _ *http.Request) {
	m := h.router.metrics
	fmt.Fprintf(w, "router_requests_routed %d\n", atomic.LoadInt64(&m.Routed))
	fmt.Fprintf(w, "router_fallbacks %d\n", atomic.LoadInt64(&m.Fallbacks))
	fmt.Fprintf(w, "router_cb_rejections %d\n", atomic.LoadInt64(&m.CBRejections))
	fmt.Fprintf(w, "router_canary_requests %d\n", atomic.LoadInt64(&m.CanaryRequests))
}

// ── Helpers ───────────────────────────────────────────────────────────────────

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

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

var _ = math.Pi

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	router := NewRouter()
	worker := NewWorkerClient()
	cacheURL := getEnv("CACHE_SERVICE_URL", "http://cache-service:8084")
	cache := NewCacheClient(cacheURL)
	h := &handler{router: router, worker: worker, cache: cache}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/route", methodHandler(map[string]http.HandlerFunc{"POST": h.route}))
	mux.HandleFunc("/v1/models", methodHandler(map[string]http.HandlerFunc{"GET": h.getModels}))
	mux.HandleFunc("/v1/canary", methodHandler(map[string]http.HandlerFunc{"POST": h.configureCanary}))
	mux.HandleFunc("/v1/stats", methodHandler(map[string]http.HandlerFunc{"GET": h.stats}))
	mux.HandleFunc("/healthz/live", h.liveness)
	mux.HandleFunc("/healthz/ready", h.readiness)
	mux.HandleFunc("/metrics", h.metricsHandler)

	port := getEnv("HTTP_PORT", "8081")
	srv := &http.Server{
		Addr:         net.JoinHostPort("", port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		slog.Info("Model Router started", "port", port)
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

func pseudoRand() float64 {
	b := make([]byte, 8)
	rand.Read(b)
	val := float64(b[0])/255.0
	return val
// task types
// model struct
// model registry
// gpt-small
// gpt-medium
// gpt-large
// embed-v2
// rerank-v1
// cb state
// cb struct
// cb allow
// cb success
// cb failure
// cb state str
// canary config
// canary configure
// canary use
// routing decision
// router struct
// models for task
// select tier
// tier budget
// tier latency
// tier prompt len
// route method
// fallback routing
// canary routing
// record success
// cb states
// worker client
// cache client
// route handler
// cost estimate
// canary handler
// stats handler
// health
// metrics handler
