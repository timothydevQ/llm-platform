package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	_ "github.com/timothydevQ/llm-platform/gen/codec"
	inferencev1 "github.com/timothydevQ/llm-platform/gen/inference/v1"
	"github.com/timothydevQ/llm-platform/services/api-gateway/internal/admission"
	"github.com/timothydevQ/llm-platform/services/api-gateway/internal/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	_ "modernc.org/sqlite"
)

// ─── Metrics ──────────────────────────────────────────────────────────────────

type gatewayMetrics struct {
	Total        int64
	AuthFailed   int64
	Admitted     int64
	AdmitFailed  int64
	RateLimited  int64
	Routed       int64
	Streamed     int64
	Errors       int64
}

// ─── Router gRPC client ───────────────────────────────────────────────────────

type routerClient struct {
	conn *grpc.ClientConn
}

func dialRouter(addr string) (*routerClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &routerClient{conn: conn}, nil
}

func (rc *routerClient) Route(ctx context.Context, req *inferencev1.InferenceRequest) (*inferencev1.InferenceResponse, error) {
	resp := new(inferencev1.InferenceResponse)
	err := rc.conn.Invoke(ctx, "/routing.v1.RouterService/Route", req, resp)
	return resp, err
}

func (rc *routerClient) Close() { rc.conn.Close() }

// ─── Gateway ──────────────────────────────────────────────────────────────────

type Gateway struct {
	authStore *auth.Store
	admitter  *admission.Admission
	router    *routerClient
	m         gatewayMetrics
	startedAt time.Time
}

func NewGateway(db *sql.DB, router *routerClient) *Gateway {
	return &Gateway{
		authStore: auth.NewStore(db),
		admitter:  admission.New(admission.DefaultConfig()),
		router:    router,
		startedAt: time.Now(),
	}
}

// ─── Request parsing ──────────────────────────────────────────────────────────

type httpRequest struct {
	Prompt          string                      `json:"prompt,omitempty"`
	Messages        []*inferencev1.ChatMessage  `json:"messages,omitempty"`
	Documents       []string                    `json:"documents,omitempty"`
	Query           string                      `json:"query,omitempty"`
	MaxTokens       int32                       `json:"max_tokens,omitempty"`
	Stream          bool                        `json:"stream,omitempty"`
	Priority        int32                       `json:"priority,omitempty"`
	CostBudget      string                      `json:"cost_budget,omitempty"`
	LatencyTargetMs int32                       `json:"latency_target_ms,omitempty"`
	Metadata        map[string]string           `json:"metadata,omitempty"`
}

func taskFromPath(path string) inferencev1.TaskType {
	switch {
	case strings.HasSuffix(path, "/chat"):      return inferencev1.TaskChat
	case strings.HasSuffix(path, "/summarize"): return inferencev1.TaskSummarize
	case strings.HasSuffix(path, "/embed"):     return inferencev1.TaskEmbed
	case strings.HasSuffix(path, "/rerank"):    return inferencev1.TaskRerank
	case strings.HasSuffix(path, "/classify"):  return inferencev1.TaskClassify
	case strings.HasSuffix(path, "/moderate"):  return inferencev1.TaskModerate
	default:                                     return inferencev1.TaskUnspecified
	}
}

func extractKey(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.Header.Get("X-API-Key")
}

// ─── HTTP handlers ────────────────────────────────────────────────────────────

func (g *Gateway) handleInference(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	atomic.AddInt64(&g.m.Total, 1)

	// ─── Auth ────────────────────────────────────────────────────────────────
	key := extractKey(r)
	principal, err := g.authStore.Validate(key)
	if err != nil {
		atomic.AddInt64(&g.m.AuthFailed, 1)
		slog.Warn("auth failed", "ip", r.RemoteAddr)
		jsonError(w, http.StatusUnauthorized, "invalid or missing API key")
		return
	}

	// ─── Parse ───────────────────────────────────────────────────────────────
	var hr httpRequest
	if err := json.NewDecoder(r.Body).Decode(&hr); err != nil {
		atomic.AddInt64(&g.m.AdmitFailed, 1)
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	reqID := newID()
	grpcReq := &inferencev1.InferenceRequest{
		RequestId:       reqID,
		IdempotencyKey:  r.Header.Get("X-Idempotency-Key"),
		TaskType:        taskFromPath(r.URL.Path),
		Prompt:          hr.Prompt,
		Messages:        hr.Messages,
		Documents:       hr.Documents,
		Query:           hr.Query,
		MaxTokens:       hr.MaxTokens,
		Stream:          hr.Stream,
		Priority:        inferencev1.Priority(hr.Priority),
		CostBudget:      hr.CostBudget,
		LatencyTargetMs: hr.LatencyTargetMs,
		TenantId:        principal.TenantID,
		ApiKeyId:        principal.KeyID,
		Metadata:        hr.Metadata,
		ReceivedAtMs:    start.UnixMilli(),
	}

	// ─── Admission ───────────────────────────────────────────────────────────
	if err := g.admitter.Admit(grpcReq); err != nil {
		atomic.AddInt64(&g.m.AdmitFailed, 1)
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	atomic.AddInt64(&g.m.Admitted, 1)

	// ─── Streaming ───────────────────────────────────────────────────────────
	if grpcReq.Stream {
		g.handleSSE(w, r, grpcReq, start)
		return
	}

	// ─── Unary routing ───────────────────────────────────────────────────────
	deadline := time.UnixMilli(grpcReq.DeadlineUnixMs)
	ctx, cancel := context.WithDeadline(r.Context(), deadline)
	defer cancel()

	resp, err := g.router.Route(ctx, grpcReq)
	if err != nil {
		atomic.AddInt64(&g.m.Errors, 1)
		st, _ := status.FromError(err)
		slog.Error("routing failed", "request_id", reqID, "code", st.Code(), "msg", st.Message())
		switch st.Code().String() {
		case "ResourceExhausted":
			jsonError(w, http.StatusTooManyRequests, st.Message())
		case "Unavailable":
			jsonError(w, http.StatusServiceUnavailable, "model service unavailable")
		default:
			jsonError(w, http.StatusInternalServerError, "inference failed")
		}
		return
	}

	atomic.AddInt64(&g.m.Routed, 1)
	w.Header().Set("X-Request-ID", reqID)
	w.Header().Set("X-Trace-ID", resp.TraceId)
	w.Header().Set("X-Response-Time-Ms", fmt.Sprintf("%d", time.Since(start).Milliseconds()))

	slog.Info("served",
		"request_id", reqID,
		"task", grpcReq.TaskType.String(),
		"model", resp.ModelId,
		"tenant", principal.TenantID,
		"latency_ms", resp.LatencyMs,
		"tokens_out", resp.TokensOutput,
		"cost_usd", resp.CostUsd,
		"cached", resp.Cached)

	jsonOK(w, resp)
}

func (g *Gateway) handleSSE(w http.ResponseWriter, r *http.Request, req *inferencev1.InferenceRequest, start time.Time) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, http.StatusInternalServerError, "streaming not supported by this transport")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Request-ID", req.RequestId)
	w.WriteHeader(http.StatusOK)

	// Simulate streaming from the model by routing as unary then chunking
	// In production this would be a server-streaming gRPC call from the router
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	resp, err := g.router.Route(ctx, req)
	if err != nil {
		fmt.Fprintf(w, "data: {\"error\":%q,\"request_id\":%q}\n\n", err.Error(), req.RequestId)
		flusher.Flush()
		return
	}

	// Chunk the content word by word
	words := strings.Fields(resp.Content)
	if len(words) == 0 {
		words = []string{""}
	}
	for i, word := range words {
		select {
		case <-r.Context().Done():
			return
		default:
		}
		chunk := &inferencev1.StreamChunk{
			RequestId: req.RequestId,
			TraceId:   resp.TraceId,
			Token:     word + " ",
			Done:      i == len(words)-1,
		}
		if chunk.Done {
			chunk.TokensOut = resp.TokensOutput
			chunk.LatencyMs = float64(time.Since(start).Milliseconds())
		}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		time.Sleep(20 * time.Millisecond)
	}

	atomic.AddInt64(&g.m.Streamed, 1)
}

// ─── HTTP server setup ────────────────────────────────────────────────────────

func (g *Gateway) handler() http.Handler {
	mux := http.NewServeMux()

	for _, task := range []string{"chat", "summarize", "embed", "rerank", "classify", "moderate"} {
		mux.HandleFunc("/v1/"+task, postOnly(g.handleInference))
	}

	mux.HandleFunc("/healthz/live",  func(w http.ResponseWriter, _ *http.Request) { jsonOK(w, map[string]string{"status": "alive"}) })
	mux.HandleFunc("/healthz/ready", func(w http.ResponseWriter, _ *http.Request) { jsonOK(w, map[string]string{"status": "ready"}) })
	mux.HandleFunc("/v1/stats",      getOnly(g.statsHandler))
	mux.HandleFunc("/metrics",       g.metricsHandler)

	return mux
}

func (g *Gateway) statsHandler(w http.ResponseWriter, _ *http.Request) {
	jsonOK(w, map[string]any{
		"total":        atomic.LoadInt64(&g.m.Total),
		"auth_failed":  atomic.LoadInt64(&g.m.AuthFailed),
		"admitted":     atomic.LoadInt64(&g.m.Admitted),
		"admit_failed": atomic.LoadInt64(&g.m.AdmitFailed),
		"rate_limited": atomic.LoadInt64(&g.m.RateLimited),
		"routed":       atomic.LoadInt64(&g.m.Routed),
		"streamed":     atomic.LoadInt64(&g.m.Streamed),
		"errors":       atomic.LoadInt64(&g.m.Errors),
		"uptime_ms":    time.Since(g.startedAt).Milliseconds(),
	})
}

func (g *Gateway) metricsHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "gateway_requests_total %d\n",    atomic.LoadInt64(&g.m.Total))
	fmt.Fprintf(w, "gateway_auth_failed %d\n",        atomic.LoadInt64(&g.m.AuthFailed))
	fmt.Fprintf(w, "gateway_admitted %d\n",           atomic.LoadInt64(&g.m.Admitted))
	fmt.Fprintf(w, "gateway_admit_failed %d\n",       atomic.LoadInt64(&g.m.AdmitFailed))
	fmt.Fprintf(w, "gateway_routed %d\n",             atomic.LoadInt64(&g.m.Routed))
	fmt.Fprintf(w, "gateway_streamed %d\n",           atomic.LoadInt64(&g.m.Streamed))
	fmt.Fprintf(w, "gateway_errors %d\n",             atomic.LoadInt64(&g.m.Errors))
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	dbPath     := getenv("DB_PATH",     "/data/gateway.db")
	routerAddr := getenv("ROUTER_ADDR", "router:50052")
	httpPort   := getenv("HTTP_PORT",   "8080")

	db, err := openDB(dbPath)
	if err != nil {
		slog.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	router, err := dialRouter(routerAddr)
	if err != nil {
		slog.Error("dial router", "addr", routerAddr, "err", err)
		os.Exit(1)
	}
	defer router.Close()

	gw := NewGateway(db, router)

	srv := &http.Server{
		Addr:         net.JoinHostPort("", httpPort),
		Handler:      gw.handler(),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 90 * time.Second,
	}

	go func() {
		slog.Info("API Gateway started", "port", httpPort, "router", routerAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
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

// ─── Helpers ──────────────────────────────────────────────────────────────────

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL")
	if err != nil {
		return nil, err
	}
	// Create minimal schema needed by auth store
	db.Exec(`
	CREATE TABLE IF NOT EXISTS tenants (
		tenant_id TEXT PRIMARY KEY, name TEXT, routing_mode TEXT DEFAULT 'balanced',
		rate_limit_rps INTEGER DEFAULT 50, burst_limit INTEGER DEFAULT 100, enabled INTEGER DEFAULT 1
	);
	CREATE TABLE IF NOT EXISTS api_keys (
		key_id TEXT PRIMARY KEY, key_hash TEXT UNIQUE, tenant_id TEXT,
		name TEXT, enabled INTEGER DEFAULT 1, last_used TEXT
	);
	INSERT OR IGNORE INTO tenants VALUES ('tenant-default','Default','balanced',50,100,1);
	INSERT OR IGNORE INTO api_keys VALUES ('k1','test-key-1234','tenant-default','Test Key',1,NULL);
	INSERT OR IGNORE INTO api_keys VALUES ('k2','platform-key-5678','tenant-default','Platform Key',1,NULL);
	`)
	return db, nil
}

func postOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h(w, r)
	}
}

func getOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h(w, r)
	}
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
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
// tw_6059_21417
// tw_6059_1305
// tw_6059_21757
// tw_6059_23561
// tw_6059_294
// tw_6059_18363
// tw_6059_10724
// tw_6059_22837
// tw_6059_17166
// tw_6059_22313
// tw_6059_7968
// tw_6059_29270
