// Package batcher implements adaptive dynamic batching.
//
// The core tradeoff: waiting longer before dispatching increases batch size
// (better GPU utilisation / throughput) but adds latency. The batcher
// adjusts the wait window based on current queue depth and p99 latency
// against the configured SLO.
//
//   queue_depth = 0-5   →  wait = MaxWaitMs  (accumulate more)
//   queue_depth = 5-20  →  wait = MaxWaitMs/2
//   queue_depth > 20    →  dispatch immediately (system is under load)
package batcher

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	inferencev1 "github.com/timothydevQ/llm-platform/gen/inference/v1"
	executionv1 "github.com/timothydevQ/llm-platform/gen/execution/v1"
	"github.com/timothydevQ/llm-platform/services/scheduler/internal/queue"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ─── Config ───────────────────────────────────────────────────────────────────

type Config struct {
	MaxBatchSize   int
	MaxWaitMs      int     // upper bound on batching window
	MinWaitMs      int     // lower bound (even under load, wait at least this long)
	P99SLOMs       float64 // target p99 latency; tighten window if exceeded
	DispatchTimeout time.Duration
}

func DefaultConfig() Config {
	return Config{
		MaxBatchSize:    16,
		MaxWaitMs:       30,
		MinWaitMs:       5,
		P99SLOMs:        500,
		DispatchTimeout: 30 * time.Second,
	}
}

// ─── Batch metrics ────────────────────────────────────────────────────────────

type BatchMetrics struct {
	BatchesDispatched int64
	RequestsProcessed int64
	TotalBatchSize    int64 // for avg calculation
	TotalBatches      int64
	LoadShedded       int64
	mu                sync.Mutex
}

func (m *BatchMetrics) Record(batchSize int) {
	atomic.AddInt64(&m.BatchesDispatched, 1)
	atomic.AddInt64(&m.RequestsProcessed, int64(batchSize))
	m.mu.Lock()
	m.TotalBatchSize += int64(batchSize)
	m.TotalBatches++
	m.mu.Unlock()
}

func (m *BatchMetrics) AvgBatchSize() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.TotalBatches == 0 { return 0 }
	return float64(m.TotalBatchSize) / float64(m.TotalBatches)
}

// latency histogram (approximate p99 from last 1000 batches)
type latencyHist struct {
	mu      sync.Mutex
	samples []float64
	head    int
	cap     int
}

func newLatencyHist(cap int) *latencyHist {
	return &latencyHist{samples: make([]float64, cap), cap: cap}
}

func (h *latencyHist) Record(ms float64) {
	h.mu.Lock()
	h.samples[h.head%h.cap] = ms
	h.head++
	h.mu.Unlock()
}

func (h *latencyHist) P99() float64 {
	h.mu.Lock()
	n := h.head
	if n > h.cap { n = h.cap }
	vals := make([]float64, n)
	copy(vals, h.samples[:n])
	h.mu.Unlock()
	if n == 0 { return 0 }
	// sort ascending
	for i := 1; i < n; i++ {
		for j := i; j > 0 && vals[j] < vals[j-1]; j-- {
			vals[j], vals[j-1] = vals[j-1], vals[j]
		}
	}
	idx := int(math.Ceil(float64(n)*0.99)) - 1
	if idx < 0 { idx = 0 }
	return vals[idx]
}

// ─── Batcher ──────────────────────────────────────────────────────────────────

// DispatchFn is called with a batch of items and the executor gRPC client.
// Implementations must set the result on each item's ResultCh.
type DispatchFn func(ctx context.Context, items []*queue.Item, execClient executionv1.ExecutorServiceClient)

// Batcher drives the batch-dispatch loop for one model.
type Batcher struct {
	modelID     string
	execAddr    string
	q           *queue.ModelQueue
	cfg         Config
	dispatchFn  DispatchFn
	metrics     *BatchMetrics
	latency     *latencyHist
	execConn    *grpc.ClientConn
	execClient  executionv1.ExecutorServiceClient
	connOnce    sync.Once
	cancel      context.CancelFunc
	db          *sql.DB
}

func New(modelID, execAddr string, q *queue.ModelQueue, cfg Config, metrics *BatchMetrics, db *sql.DB) *Batcher {
	return &Batcher{
		modelID:  modelID,
		execAddr: execAddr,
		q:        q,
		cfg:      cfg,
		dispatchFn: defaultDispatch,
		metrics:  metrics,
		latency:  newLatencyHist(1000),
		db:       db,
	}
}

func (b *Batcher) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	b.cancel = cancel
	go b.loop(ctx)
}

func (b *Batcher) Stop() {
	if b.cancel != nil { b.cancel() }
	if b.execConn != nil { b.execConn.Close() }
}

func (b *Batcher) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Adaptive wait window
		waitMs := b.adaptiveWait()
		time.Sleep(time.Duration(waitMs) * time.Millisecond)

		// Drain up to MaxBatchSize
		items := b.q.Drain(b.cfg.MaxBatchSize)
		if len(items) == 0 {
			continue
		}

		b.dispatch(ctx, items)
	}
}

// adaptiveWait calculates the wait window in ms.
func (b *Batcher) adaptiveWait() int {
	depth := b.q.Depth()
	p99 := b.latency.P99()

	if depth > 20 || p99 > b.cfg.P99SLOMs*1.5 {
		return b.cfg.MinWaitMs
	}
	if depth > 5 {
		return b.cfg.MaxWaitMs / 2
	}
	return b.cfg.MaxWaitMs
}

func (b *Batcher) dispatch(ctx context.Context, items []*queue.Item) {
	start := time.Now()

	execClient, err := b.execClientLazy()
	if err != nil {
		for _, item := range items {
			item.ResultCh <- &queue.Result{Err: fmt.Errorf("executor dial: %w", err)}
		}
		return
	}

	ctx, cancel := context.WithTimeout(ctx, b.cfg.DispatchTimeout)
	defer cancel()

	b.dispatchFn(ctx, items, execClient)

	latencyMs := float64(time.Since(start).Milliseconds())
	b.latency.Record(latencyMs)
	b.metrics.Record(len(items))
	b.logBatch(len(items), float64(b.adaptiveWait()), latencyMs)

	slog.Info("batch dispatched",
		"model", b.modelID,
		"size", len(items),
		"wait_ms", b.adaptiveWait(),
		"dispatch_ms", latencyMs,
		"p99_ms", b.latency.P99())
}

func (b *Batcher) execClientLazy() (executionv1.ExecutorServiceClient, error) {
	var connErr error
	b.connOnce.Do(func() {
		conn, err := grpc.NewClient(b.execAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			connErr = err
			return
		}
		b.execConn = conn
		b.execClient = executionv1.NewExecutorServiceClient(conn)
	})
	if connErr != nil { return nil, connErr }
	return b.execClient, nil
}

func (b *Batcher) logBatch(batchSize int, waitMs, dispatchMs float64, flushReason ...string) {
	if b.db == nil { return }
	reason := "timer"
	if len(flushReason) > 0 { reason = flushReason[0] }
	b.db.Exec(`
		INSERT INTO batch_log (batch_id, model_id, task_type, batch_size, wait_ms, dispatch_ms, flush_reason)
		VALUES (?,?,?,?,?,?,?)`,
		newID(), b.modelID, "mixed", batchSize, waitMs, dispatchMs, reason)
}

func (b *Batcher) P99LatencyMs() float64 { return b.latency.P99() }

// ─── Default dispatch implementation ─────────────────────────────────────────

func defaultDispatch(ctx context.Context, items []*queue.Item, client executionv1.ExecutorServiceClient) {
	var wg sync.WaitGroup
	for _, item := range items {
		wg.Add(1)
		go func(it *queue.Item) {
			defer wg.Done()
			waitMs := float64(time.Since(it.EnqueuedAt).Milliseconds())
			execResp, err := client.Execute(ctx, &executionv1.ExecuteRequest{
				RequestId:  it.Req.RequestId,
				ModelId:    it.ExecutorAddr, // ModelID was stored in ExecutorAddr by the dispatcher
				TaskType:   it.Req.TaskType,
				Prompt:     it.Req.Prompt,
				Messages:   it.Req.Messages,
				Documents:  it.Req.Documents,
				Query:      it.Req.Query,
				MaxTokens:  it.Req.MaxTokens,
			})
			if err != nil {
				it.ResultCh <- &queue.Result{Err: err, WaitMs: waitMs}
				return
			}
			it.ResultCh <- &queue.Result{
				WaitMs: waitMs,
				Resp: &inferencev1.InferenceResponse{
					RequestId:    execResp.RequestId,
					ModelId:      execResp.ModelId,
					Content:      execResp.Content,
					Embedding:    execResp.Embedding,
					Scores:       execResp.Scores,
					TokensOutput: execResp.TokensOutput,
					TokensInput:  execResp.TokensInput,
					LatencyMs:    execResp.LatencyMs,
					QueueWaitMs:  waitMs,
				},
			}
		}(item)
	}
	wg.Wait()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func newID() string {
	b := make([]byte, 8)
	// Use time-based pseudo-random for DB logging (not security-sensitive)
	t := time.Now().UnixNano()
	for i := range b {
		b[i] = byte(t >> (i * 8))
	}
	return fmt.Sprintf("%x", b)
}

var _ = math.Ceil // keep math imported
// sb_206
// sb_207
// sb_208
// sb_209
// sb_210
// sb_211
// sb_212
// sb_213
// sb_214
// sb_215
// sb_216
