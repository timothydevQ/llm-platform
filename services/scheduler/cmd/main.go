package main

import (
	"context"
	"database/sql"
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
	"github.com/timothydevQ/llm-platform/services/scheduler/internal/batcher"
	"github.com/timothydevQ/llm-platform/services/scheduler/internal/queue"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	_ "modernc.org/sqlite"
)

// ─── Scheduler service ────────────────────────────────────────────────────────

type SchedulerMetrics struct {
	Enqueued    int64
	Dispatched  int64
	LoadShedded int64
	Errors      int64
}

type SchedulerService struct {
	queues   *queue.Registry
	batchers map[string]*batcher.Batcher
	bMu      sync.RWMutex
	metrics  *batcher.BatchMetrics
	sMetrics SchedulerMetrics
	cfg      batcher.Config
	db       *sql.DB
	execAddr string
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewSchedulerService(db *sql.DB, execAddr string) *SchedulerService {
	ctx, cancel := context.WithCancel(context.Background())
	return &SchedulerService{
		queues:   queue.NewRegistry(10000),
		batchers: make(map[string]*batcher.Batcher),
		metrics:  &batcher.BatchMetrics{},
		cfg:      batcher.DefaultConfig(),
		db:       db,
		execAddr: execAddr,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (s *SchedulerService) getBatcher(modelID, execAddr string) *batcher.Batcher {
	s.bMu.RLock()
	b, ok := s.batchers[modelID]
	s.bMu.RUnlock()
	if ok { return b }

	s.bMu.Lock()
	defer s.bMu.Unlock()
	if b, ok := s.batchers[modelID]; ok { return b }

	q := s.queues.Queue(modelID)
	b = batcher.New(modelID, execAddr, q, s.cfg, s.metrics, s.db)
	b.Start(s.ctx)
	s.batchers[modelID] = b
	slog.Info("batcher started", "model", modelID, "executor", execAddr)
	return b
}

// Schedule enqueues a request and waits for the batcher to dispatch it.
func (s *SchedulerService) Schedule(
	ctx context.Context,
	req *inferencev1.InferenceRequest,
	modelID, execAddr string,
) (*inferencev1.InferenceResponse, error) {
	atomic.AddInt64(&s.sMetrics.Enqueued, 1)

	s.getBatcher(modelID, execAddr)
	q := s.queues.Queue(modelID)

	item := &queue.Item{
		Req:          req,
		ExecutorAddr: modelID, // batcher uses this as model_id in execute call
		EnqueuedAt:   time.Now(),
		ResultCh:     make(chan *queue.Result, 1),
	}

	if !q.Enqueue(item) {
		atomic.AddInt64(&s.sMetrics.LoadShedded, 1)
		return nil, status.Errorf(codes.ResourceExhausted,
			"queue full for model %s (depth=%d)", modelID, q.Depth())
	}

	// Wait for result with deadline
	deadline := time.Now().Add(60 * time.Second)
	if req.DeadlineUnixMs > 0 {
		deadline = time.UnixMilli(req.DeadlineUnixMs)
	}
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()

	select {
	case result := <-item.ResultCh:
		if result.Err != nil {
			atomic.AddInt64(&s.sMetrics.Errors, 1)
			return nil, result.Err
		}
		atomic.AddInt64(&s.sMetrics.Dispatched, 1)
		return result.Resp, nil
	case <-timer.C:
		atomic.AddInt64(&s.sMetrics.Errors, 1)
		return nil, status.Errorf(codes.DeadlineExceeded, "request %s timed out in scheduler", req.RequestId)
	case <-ctx.Done():
		return nil, status.Errorf(codes.Canceled, "request %s cancelled", req.RequestId)
	}
}

func (s *SchedulerService) Stop() {
	s.cancel()
	s.bMu.Lock()
	for _, b := range s.batchers {
		b.Stop()
	}
	s.bMu.Unlock()
}

func (s *SchedulerService) Stats() map[string]any {
	return map[string]any{
		"enqueued":         atomic.LoadInt64(&s.sMetrics.Enqueued),
		"dispatched":       atomic.LoadInt64(&s.sMetrics.Dispatched),
		"load_shedded":     atomic.LoadInt64(&s.sMetrics.LoadShedded),
		"errors":           atomic.LoadInt64(&s.sMetrics.Errors),
		"avg_batch_size":   s.metrics.AvgBatchSize(),
		"batches":          s.metrics.BatchesDispatched,
		"queue_depths":     s.queues.AllDepths(),
	}
}

// ─── gRPC server (wraps SchedulerService) ────────────────────────────────────

type schedulerServer struct {
	svc *SchedulerService
}

// ScheduleRequest is the message received by the scheduler gRPC endpoint.
type ScheduleRequest struct {
	Request      *inferencev1.InferenceRequest `json:"request"`
	ModelID      string                         `json:"model_id"`
	ExecutorAddr string                          `json:"executor_addr"`
}

func (s *schedulerServer) Schedule(ctx context.Context, req *ScheduleRequest) (*inferencev1.InferenceResponse, error) {
	if req.Request == nil {
		return nil, status.Errorf(codes.InvalidArgument, "request is required")
	}
	return s.svc.Schedule(ctx, req.Request, req.ModelID, req.ExecutorAddr)
}

const schedulerServiceName = "scheduling.v1.SchedulerService"

func registerScheduler(grpcSrv *grpc.Server, srv *schedulerServer) {
	grpcSrv.RegisterService(&grpc.ServiceDesc{
		ServiceName: schedulerServiceName,
		HandlerType: (*schedulerServer)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "Schedule",
			Handler: func(s any, ctx context.Context, dec func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
				in := new(ScheduleRequest)
				if err := dec(in); err != nil { return nil, err }
				if i == nil { return s.(*schedulerServer).Schedule(ctx, in) }
				info := &grpc.UnaryServerInfo{Server: s, FullMethod: "/" + schedulerServiceName + "/Schedule"}
				return i(ctx, in, info, func(ctx context.Context, req any) (any, error) {
					return s.(*schedulerServer).Schedule(ctx, req.(*ScheduleRequest))
				})
			},
		}},
		Streams: []grpc.StreamDesc{},
	}, srv)
}

// ─── HTTP admin ───────────────────────────────────────────────────────────────

func httpAdmin(svc *SchedulerService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz/live",  func(w http.ResponseWriter, _ *http.Request) { jsonOK(w, map[string]string{"status": "alive"}) })
	mux.HandleFunc("/healthz/ready", func(w http.ResponseWriter, _ *http.Request) { jsonOK(w, map[string]string{"status": "ready"}) })
	mux.HandleFunc("/v1/stats",      func(w http.ResponseWriter, _ *http.Request) { jsonOK(w, svc.Stats()) })
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		stats := svc.Stats()
		fmt.Fprintf(w, "scheduler_enqueued %v\n",      stats["enqueued"])
		fmt.Fprintf(w, "scheduler_dispatched %v\n",    stats["dispatched"])
		fmt.Fprintf(w, "scheduler_load_shedded %v\n",  stats["load_shedded"])
		fmt.Fprintf(w, "scheduler_avg_batch_size %v\n", stats["avg_batch_size"])
	})
	return mux
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	dbPath   := getenv("DB_PATH",       "/data/scheduler.db")
	grpcPort := getenv("GRPC_PORT",     "50053")
	httpPort := getenv("HTTP_PORT",     "8082")
	execAddr := getenv("EXECUTOR_ADDR", "model-executor:50051")

	db, err := openDB(dbPath)
	if err != nil {
		slog.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	svc := NewSchedulerService(db, execAddr)
	defer svc.Stop()

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}
	grpcSrv := grpc.NewServer()
	registerScheduler(grpcSrv, &schedulerServer{svc: svc})

	httpSrv := &http.Server{
		Addr: ":" + httpPort, Handler: httpAdmin(svc),
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("Scheduler gRPC started", "port", grpcPort)
		grpcSrv.Serve(lis)
	}()
	go func() {
		slog.Info("Scheduler HTTP started", "port", httpPort)
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

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL")
	if err != nil { return nil, err }
	db.Exec(`
	CREATE TABLE IF NOT EXISTS batch_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		batch_id TEXT, model_id TEXT, task_type TEXT,
		batch_size INTEGER, wait_ms REAL, dispatch_ms REAL, flush_reason TEXT,
		created_at TEXT DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_batch_model ON batch_log(model_id, created_at DESC);
	`)
	return db, nil
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func getenv(k, fb string) string {
	if v := os.Getenv(k); v != "" { return v }
	return fb
}
// rf_483
// rf_484
