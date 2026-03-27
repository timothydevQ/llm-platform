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
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	_ "github.com/timothydevQ/llm-platform/gen/codec"
	inferencev1 "github.com/timothydevQ/llm-platform/gen/inference/v1"
	executionv1 "github.com/timothydevQ/llm-platform/gen/execution/v1"
	"github.com/timothydevQ/llm-platform/services/router/internal/policy"
	"github.com/timothydevQ/llm-platform/services/router/internal/repo"
	"github.com/timothydevQ/llm-platform/services/router/internal/middleware"
	"github.com/timothydevQ/llm-platform/services/router/internal/scoring"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	_ "modernc.org/sqlite"
)

// ── Canary state ──────────────────────────────────────────────────────────────

type canaryState struct {
	mu      sync.RWMutex
	configs []*repo.RolloutConfig
}

func (c *canaryState) reload(cfgs []*repo.RolloutConfig) {
	c.mu.Lock()
	c.configs = cfgs
	c.mu.Unlock()
}

// RolloutWeight returns the rollout weight for each model based on canary config.
// Models not in any canary get weight 1.0; canary models get weight = canaryPct.
func (c *canaryState) rolloutWeights() map[string]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	weights := make(map[string]float64)
	for _, cfg := range c.configs {
		if !cfg.Enabled {
			continue
		}
		// Primary model gets (1 - canaryPct) weight; canary gets canaryPct
		weights[cfg.BaseModelID] = 1.0 - cfg.CanaryPct
		weights[cfg.CanaryModelID] = cfg.CanaryPct
	}
	return weights
}

// ── Executor connection pool ───────────────────────────────────────────────────

type execPool struct {
	mu    sync.RWMutex
	conns map[string]*grpc.ClientConn
}

func newExecPool() *execPool { return &execPool{conns: make(map[string]*grpc.ClientConn)} }

func (p *execPool) client(addr string) (executionv1.ExecutorServiceClient, error) {
	p.mu.RLock()
	if c, ok := p.conns[addr]; ok {
		p.mu.RUnlock()
		return executionv1.NewExecutorServiceClient(c), nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.conns[addr]; ok {
		return executionv1.NewExecutorServiceClient(c), nil
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial executor %s: %w", addr, err)
	}
	p.conns[addr] = conn
	return executionv1.NewExecutorServiceClient(conn), nil
}

func (p *execPool) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.conns {
		c.Close()
	}
}

// ── Router server ─────────────────────────────────────────────────────────────

type RouterMetrics struct {
	Routed      int64
	Fallbacks   int64
	CBBlocked   int64
	Canary      int64
	CacheHits   int64
	Errors      int64
}

type RouterServer struct {
	store    *repo.Store
	scorer   *scoring.Scorer
	health   *scoring.HealthTracker
	breakers *policy.Registry
	limiter  *policy.RateLimiter
	policies *policy.PolicyStore
	canary   *canaryState
	pool     *execPool
	metrics  RouterMetrics
	models   []*scoring.ModelRecord
	mu       sync.RWMutex // guards models
}

func NewRouterServer(store *repo.Store) (*RouterServer, error) {
	models, err := store.LoadModels()
	if err != nil {
		return nil, fmt.Errorf("load models: %w", err)
	}

	rollouts, _ := store.LoadRollouts()
	cs := &canaryState{}
	cs.reload(rollouts)

	health := scoring.NewHealthTracker()
	scorer := scoring.NewScorer(health, nil)

	srv := &RouterServer{
		store:    store,
		scorer:   scorer,
		health:   health,
		breakers: policy.NewRegistry(),
		limiter:  policy.NewRateLimiter(),
		policies: policy.NewPolicyStore(),
		canary:   cs,
		pool:     newExecPool(),
		models:   models,
	}
	// Background reload
	go srv.reloadLoop()
	return srv, nil
}

func (s *RouterServer) reloadLoop() {
	tick := time.NewTicker(30 * time.Second)
	for range tick.C {
		models, err := s.store.LoadModels()
		if err == nil {
			s.mu.Lock()
			s.models = models
			s.mu.Unlock()
		}
		rollouts, err := s.store.LoadRollouts()
		if err == nil {
			s.canary.reload(rollouts)
		}
	}
}

func (s *RouterServer) Route(ctx context.Context, req *inferencev1.InferenceRequest) (*inferencev1.InferenceResponse, error) {
	start := time.Now()
	traceID := newID()

	// Load tenant policy
	tenantID := req.TenantId
	if tenantID == "" {
		tenantID = "tenant-default"
	}

	var tPolicy *policy.TenantPolicy
	if p, ok := s.policies.Get(tenantID); ok {
		tPolicy = p
	} else {
		var err error
		tPolicy, err = s.store.LoadTenantPolicy(tenantID)
		if err != nil || tPolicy == nil {
			tPolicy = policy.Default(tenantID)
		}
		s.policies.Set(tPolicy)
	}

	// Rate limit check
	if !s.limiter.Allow(tenantID, tPolicy.RateLimit, tPolicy.BurstLimit) {
		return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded for tenant %s", tenantID)
	}

	// Build scoring request
	promptLen := len(req.Prompt)
	for _, m := range req.Messages {
		promptLen += len(m.Content)
	}

	rolloutWeights := s.canary.rolloutWeights()

	sreq := &scoring.ScoringRequest{
		Task:            req.TaskType.String(),
		CostBudget:      req.CostBudget,
		LatencyTargetMs: req.LatencyTargetMs,
		PromptLen:       promptLen,
		TenantMode:      scoring.ScoringMode(tPolicy.RoutingMode),
		AllowedModels:   tPolicy.AllowedModels,
		RolloutWeights:  rolloutWeights,
	}

	// Filter candidates by circuit breaker state
	s.mu.RLock()
	models := s.models
	s.mu.RUnlock()

	var candidates []*scoring.ModelRecord
	for _, m := range models {
		if s.breakers.Get(m.ModelID).Allow() {
			candidates = append(candidates, m)
		} else {
			atomic.AddInt64(&s.metrics.CBBlocked, 1)
		}
	}

	// Score candidates
	ranked := s.scorer.Score(sreq, candidates)
	if len(ranked) == 0 {
		atomic.AddInt64(&s.metrics.Errors, 1)
		return nil, status.Errorf(codes.Unavailable, "no available models for task %s", req.TaskType.String())
	}

	primary := ranked[0]
	isFallback := len(ranked) > 0 && primary != ranked[0]
	isCanary := rolloutWeights[primary.ModelID] > 0 && rolloutWeights[primary.ModelID] < 1.0

	// Dispatch to executor
	execClient, err := s.pool.client(primary.ExecutorAddr)
	if err != nil {
		s.breakers.Get(primary.ModelID).RecordFailure()
		s.health.RecordFailure(primary.ModelID)
		atomic.AddInt64(&s.metrics.Errors, 1)
		return nil, status.Errorf(codes.Unavailable, "executor dial failed: %v", err)
	}

	execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	execResp, err := execClient.Execute(execCtx, &executionv1.ExecuteRequest{
		RequestId:  req.RequestId,
		ModelId:    primary.ModelID,
		ModelVersion: primary.Version,
		TaskType:   req.TaskType,
		Prompt:     req.Prompt,
		Messages:   req.Messages,
		Documents:  req.Documents,
		Query:      req.Query,
		MaxTokens:  req.MaxTokens,
	})
	if err != nil {
		s.breakers.Get(primary.ModelID).RecordFailure()
		s.health.RecordFailure(primary.ModelID)
		slog.Error("executor failed", "model", primary.ModelID, "err", err)
		atomic.AddInt64(&s.metrics.Errors, 1)
		return nil, status.Errorf(codes.Internal, "executor error: %v", err)
	}

	latencyMs := float64(time.Since(start).Milliseconds())
	s.breakers.Get(primary.ModelID).RecordSuccess()
	s.health.RecordSuccess(primary.ModelID, latencyMs)
	atomic.AddInt64(&s.metrics.Routed, 1)
	if isCanary { atomic.AddInt64(&s.metrics.Canary, 1) }
	if isFallback { atomic.AddInt64(&s.metrics.Fallbacks, 1) }

	// Find model record for cost calculation
	var modelRecord *scoring.ModelRecord
	for _, m := range models {
		if m.ModelID == primary.ModelID { modelRecord = m; break }
	}
	costUSD := 0.0
	if modelRecord != nil {
		costUSD = float64(execResp.TokensOutput) / 1000.0 * modelRecord.CostPer1k
	}

	resp := &inferencev1.InferenceResponse{
		RequestId:    req.RequestId,
		TraceId:      traceID,
		TaskType:     req.TaskType,
		ModelId:      execResp.ModelId,
		ModelTier:    tierEnum(primary.Version),
		Content:      execResp.Content,
		Embedding:    execResp.Embedding,
		Scores:       execResp.Scores,
		TokensInput:  execResp.TokensInput,
		TokensOutput: execResp.TokensOutput,
		LatencyMs:    latencyMs,
		FallbackUsed: isFallback,
		IsCanary:     isCanary,
		CostUsd:      costUSD,
		ExecutorId:   primary.ExecutorAddr,
	}

	// Async audit log
	go s.store.LogRequest(&repo.LogEntry{
		RequestID:    req.RequestId,
		TraceID:      traceID,
		TenantID:     tenantID,
		TaskType:     req.TaskType.String(),
		ModelID:      execResp.ModelId,
		TokensInput:  execResp.TokensInput,
		TokensOutput: execResp.TokensOutput,
		LatencyMs:    latencyMs,
		CostUSD:      costUSD,
		FallbackUsed: isFallback,
		IsCanary:     isCanary,
		RoutingMode:  tPolicy.RoutingMode,
	})

	slog.Info("routed",
		"request_id", req.RequestId,
		"model", primary.ModelID,
		"score", primary.TotalScore,
		"latency_ms", latencyMs,
		"tokens_out", execResp.TokensOutput,
		"cost_usd", costUSD,
		"canary", isCanary,
		"trace_id", traceID)

	return resp, nil
}

// ── gRPC service registration ─────────────────────────────────────────────────

const routerServiceName = "routing.v1.RouterService"

func registerRouter(grpcSrv *grpc.Server, srv *RouterServer) {
	grpcSrv.RegisterService(&grpc.ServiceDesc{
		ServiceName: routerServiceName,
		HandlerType: (*RouterServer)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "Route",
			Handler: func(s any, ctx context.Context, dec func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
				in := new(inferencev1.InferenceRequest)
				if err := dec(in); err != nil { return nil, err }
				if i == nil { return s.(*RouterServer).Route(ctx, in) }
				info := &grpc.UnaryServerInfo{Server: s, FullMethod: "/" + routerServiceName + "/Route"}
				return i(ctx, in, info, func(ctx context.Context, req any) (any, error) {
					return s.(*RouterServer).Route(ctx, req.(*inferencev1.InferenceRequest))
				})
			},
		}},
		Streams: []grpc.StreamDesc{},
	}, srv)
}

// ── HTTP admin ─────────────────────────────────────────────────────────────────

func (s *RouterServer) httpAdmin() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz/live",  func(w http.ResponseWriter, _ *http.Request) { jsonResp(w, 200, map[string]string{"status": "alive"}) })
	mux.HandleFunc("/healthz/ready", func(w http.ResponseWriter, _ *http.Request) { jsonResp(w, 200, map[string]string{"status": "ready"}) })

	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, _ *http.Request) {
		models, _ := s.store.LoadModels()
		jsonResp(w, 200, map[string]any{
			"models":           models,
			"circuit_breakers": s.breakers.States(),
		})
	})

	mux.HandleFunc("/v1/rollout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			rollouts, _ := s.store.LoadRollouts()
			jsonResp(w, 200, rollouts)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		var cfg repo.RolloutConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			jsonResp(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if err := s.store.UpsertRollout(&cfg); err != nil {
			jsonResp(w, 500, map[string]string{"error": err.Error()})
			return
		}
		rollouts, _ := s.store.LoadRollouts()
		s.canary.reload(rollouts)
		jsonResp(w, 200, map[string]string{"status": "configured"})
	})

	mux.HandleFunc("/v1/stats", func(w http.ResponseWriter, _ *http.Request) {
		jsonResp(w, 200, map[string]any{
			"routed":           atomic.LoadInt64(&s.metrics.Routed),
			"fallbacks":        atomic.LoadInt64(&s.metrics.Fallbacks),
			"cb_blocked":       atomic.LoadInt64(&s.metrics.CBBlocked),
			"canary":           atomic.LoadInt64(&s.metrics.Canary),
			"errors":           atomic.LoadInt64(&s.metrics.Errors),
			"circuit_breakers": s.breakers.States(),
			"window_1h":        s.store.WindowStats(time.Hour),
		})
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "router_routed %d\n", atomic.LoadInt64(&s.metrics.Routed))
		fmt.Fprintf(w, "router_fallbacks %d\n", atomic.LoadInt64(&s.metrics.Fallbacks))
		fmt.Fprintf(w, "router_cb_blocked %d\n", atomic.LoadInt64(&s.metrics.CBBlocked))
		fmt.Fprintf(w, "router_canary %d\n", atomic.LoadInt64(&s.metrics.Canary))
		fmt.Fprintf(w, "router_errors %d\n", atomic.LoadInt64(&s.metrics.Errors))
	})
	return mux
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	dbPath   := getenv("DB_PATH",   "/data/router.db")
	grpcPort := getenv("GRPC_PORT", "50052")
	httpPort := getenv("HTTP_PORT", "8081")
	execAddr := getenv("EXECUTOR_ADDR", "model-executor:50051")

	store, err := repo.Open(dbPath)
	if err != nil {
		slog.Error("open store", "err", err)
		os.Exit(1)
	}
	if err := store.Seed(execAddr); err != nil {
		slog.Warn("seed", "err", err)
	}

	srv, err := NewRouterServer(store)
	if err != nil {
		slog.Error("init router", "err", err)
		os.Exit(1)
	}
	defer srv.pool.close()

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		slog.Error("listen", "port", grpcPort, "err", err)
		os.Exit(1)
	}
	icMetrics := &middleware.InterceptorMetrics{}
	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.Recovery(),
			middleware.RequestID(),
			middleware.Logging(slog.Default()),
			middleware.Metrics(icMetrics),
			middleware.DeadlineCheck(),
		),
	)
	registerRouter(grpcSrv, srv)

	httpSrv := &http.Server{
		Addr: ":" + httpPort, Handler: srv.httpAdmin(),
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("Router gRPC started", "port", grpcPort)
		if err := grpcSrv.Serve(lis); err != nil {
			slog.Error("grpc serve", "err", err)
		}
	}()
	go func() {
		slog.Info("Router HTTP admin started", "port", httpPort)
		httpSrv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	grpcSrv.GracefulStop()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpSrv.Shutdown(ctx)
}

// ── Helpers ───────────────────────────────────────────────────────────────────


func tierEnum(_ string) inferencev1.ModelTier {
	return inferencev1.TierSmall // simplified; real impl reads from record
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func getenv(k, fb string) string {
	if v := os.Getenv(k); v != "" { return v }
	return fb
}

func jsonResp(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
// tw_6059_6805
// tw_6059_14010
// tw_6059_17241
// tw_6059_29938
// tw_6059_13530
// tw_6059_31111
// tw_6059_13590
// tw_6059_9462
// tw_6059_6367
// tw_6059_4055
// tw_6059_19092
// tw_6059_21593
// tw_6059_16453
// tw_6059_8244
// tw_6059_16270
// tw_6059_30133
// tw_6059_31007
// tw_6059_25682
// tw_6059_16379
