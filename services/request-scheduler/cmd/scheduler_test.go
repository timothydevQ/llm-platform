package main

import (
	"fmt"
	"testing"
	"time"
)

// ── Priority Queue Tests ──────────────────────────────────────────────────────

func TestPQ_EnqueueAndDequeue(t *testing.T) {
	pq := NewPriorityQueue(100)
	req := &QueuedRequest{ID: "r1", Priority: 1}
	if !pq.Enqueue(req) { t.Error("expected enqueue success") }
	got := pq.Dequeue()
	if got == nil { t.Fatal("expected non-nil dequeue") }
	if got.ID != "r1" { t.Errorf("wrong ID: %s", got.ID) }
}

func TestPQ_HighPriorityDequeuesFirst(t *testing.T) {
	pq := NewPriorityQueue(100)
	pq.Enqueue(&QueuedRequest{ID: "low", Priority: 0})
	pq.Enqueue(&QueuedRequest{ID: "high", Priority: 2})
	pq.Enqueue(&QueuedRequest{ID: "normal", Priority: 1})
	first := pq.Dequeue()
	if first.ID != "high" { t.Errorf("expected high priority first, got %s", first.ID) }
}

func TestPQ_NormalBeforeLow(t *testing.T) {
	pq := NewPriorityQueue(100)
	pq.Enqueue(&QueuedRequest{ID: "low", Priority: 0})
	pq.Enqueue(&QueuedRequest{ID: "normal", Priority: 1})
	first := pq.Dequeue()
	if first.ID != "normal" { t.Errorf("expected normal before low, got %s", first.ID) }
}

func TestPQ_ReturnsNilWhenEmpty(t *testing.T) {
	pq := NewPriorityQueue(100)
	if pq.Dequeue() != nil { t.Error("expected nil from empty queue") }
}

func TestPQ_LoadShedding(t *testing.T) {
	pq := NewPriorityQueue(2)
	pq.Enqueue(&QueuedRequest{ID: "r1"})
	pq.Enqueue(&QueuedRequest{ID: "r2"})
	result := pq.Enqueue(&QueuedRequest{ID: "r3"})
	if result { t.Error("expected load shedding when queue full") }
}

func TestPQ_Len(t *testing.T) {
	pq := NewPriorityQueue(100)
	if pq.Len() != 0 { t.Error("expected empty queue") }
	pq.Enqueue(&QueuedRequest{ID: "r1"})
	if pq.Len() != 1 { t.Error("expected 1 item") }
}

func TestPQ_LenByPriority(t *testing.T) {
	pq := NewPriorityQueue(100)
	pq.Enqueue(&QueuedRequest{ID: "h1", Priority: 2})
	pq.Enqueue(&QueuedRequest{ID: "h2", Priority: 2})
	pq.Enqueue(&QueuedRequest{ID: "n1", Priority: 1})
	pq.Enqueue(&QueuedRequest{ID: "l1", Priority: 0})
	high, normal, low := pq.LenByPriority()
	if high != 2 { t.Errorf("expected 2 high, got %d", high) }
	if normal != 1 { t.Errorf("expected 1 normal, got %d", normal) }
	if low != 1 { t.Errorf("expected 1 low, got %d", low) }
}

func TestPQ_DrainInOrder(t *testing.T) {
	pq := NewPriorityQueue(100)
	pq.Enqueue(&QueuedRequest{ID: "low", Priority: 0})
	pq.Enqueue(&QueuedRequest{ID: "high", Priority: 2})
	pq.Enqueue(&QueuedRequest{ID: "normal", Priority: 1})

	order := []string{}
	for pq.Len() > 0 {
		req := pq.Dequeue()
		order = append(order, req.ID)
	}
	expected := []string{"high", "normal", "low"}
	for i, id := range expected {
		if order[i] != id { t.Errorf("expected %s at position %d, got %s", id, i, order[i]) }
	}
}

// ── Batcher Tests ─────────────────────────────────────────────────────────────

func TestBatcher_ProcessesSingleRequest(t *testing.T) {
	dispatched := make(chan *Batch, 10)
	dispatchFn := func(b *Batch) []*BatchResult {
		dispatched <- b
		results := make([]*BatchResult, len(b.Requests))
		for i, req := range b.Requests {
			results[i] = &BatchResult{RequestID: req.ID, Content: "ok"}
		}
		return results
	}

	cfg := DefaultBatcherConfig()
	cfg.MaxWaitMs = 50
	batcher := NewAdaptiveBatcher(cfg, dispatchFn)
	batcher.metrics = &SchedulerMetrics{}

	req := &QueuedRequest{ID: "r1", TaskType: TaskChat, Prompt: "hello"}
	ch := batcher.Enqueue(req)

	select {
	case result := <-ch:
		if result.RequestID != "r1" { t.Errorf("wrong request ID: %s", result.RequestID) }
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for batch result")
	}
}

func TestBatcher_BatchesMultipleRequests(t *testing.T) {
	var batchSizes []int
	var mu sync.Mutex
	dispatchFn := func(b *Batch) []*BatchResult {
		mu.Lock()
		batchSizes = append(batchSizes, len(b.Requests))
		mu.Unlock()
		results := make([]*BatchResult, len(b.Requests))
		for i, req := range b.Requests {
			results[i] = &BatchResult{RequestID: req.ID, Content: "ok"}
		}
		return results
	}

	cfg := DefaultBatcherConfig()
	cfg.MaxWaitMs = 50
	cfg.MaxBatchSize = 4
	batcher := NewAdaptiveBatcher(cfg, dispatchFn)
	batcher.metrics = &SchedulerMetrics{}

	channels := make([]chan *BatchResult, 3)
	for i := 0; i < 3; i++ {
		req := &QueuedRequest{ID: fmt.Sprintf("r%d", i), TaskType: TaskEmbed, Prompt: fmt.Sprintf("doc%d", i)}
		channels[i] = batcher.Enqueue(req)
	}

	for i, ch := range channels {
		select {
		case result := <-ch:
			if result == nil { t.Errorf("nil result for request %d", i) }
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timeout for request %d", i)
		}
	}
}

func TestBatcher_TriggersOnMaxBatchSize(t *testing.T) {
	dispatched := make(chan struct{}, 10)
	dispatchFn := func(b *Batch) []*BatchResult {
		dispatched <- struct{}{}
		results := make([]*BatchResult, len(b.Requests))
		for i, req := range b.Requests {
			results[i] = &BatchResult{RequestID: req.ID}
		}
		return results
	}

	cfg := DefaultBatcherConfig()
	cfg.MaxBatchSize = 3
	cfg.MaxWaitMs = 10000 // very long wait — dispatch only on size
	batcher := NewAdaptiveBatcher(cfg, dispatchFn)
	batcher.metrics = &SchedulerMetrics{}

	for i := 0; i < 3; i++ {
		req := &QueuedRequest{ID: fmt.Sprintf("r%d", i), TaskType: TaskChat, Prompt: "hi"}
		batcher.Enqueue(req)
	}

	select {
	case <-dispatched:
		// dispatch fired on batch size trigger
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected dispatch triggered on max batch size")
	}
}

func TestBatcher_QueueDepths(t *testing.T) {
	dispatchFn := func(b *Batch) []*BatchResult {
		time.Sleep(100 * time.Millisecond)
		results := make([]*BatchResult, len(b.Requests))
		for i, req := range b.Requests { results[i] = &BatchResult{RequestID: req.ID} }
		return results
	}
	cfg := DefaultBatcherConfig()
	cfg.MaxWaitMs = 10000
	batcher := NewAdaptiveBatcher(cfg, dispatchFn)
	batcher.metrics = &SchedulerMetrics{}

	depths := batcher.QueueDepths()
	if depths == nil { t.Error("expected non-nil depths") }
}

// ── Scheduler Metrics Tests ───────────────────────────────────────────────────

func TestSchedulerMetrics_AvgBatchSize_NoData(t *testing.T) {
	m := &SchedulerMetrics{}
	if m.AvgBatchSize() != 0 { t.Error("expected 0 with no data") }
}

func TestSchedulerMetrics_AvgBatchSize(t *testing.T) {
	m := &SchedulerMetrics{
		TotalBatchedItems: 20,
		TotalBatches:      4,
	}
	if m.AvgBatchSize() != 5.0 { t.Errorf("expected 5.0, got %f", m.AvgBatchSize()) }
}

// ── Batcher Config Tests ──────────────────────────────────────────────────────

func TestDefaultBatcherConfig(t *testing.T) {
	cfg := DefaultBatcherConfig()
	if cfg.MaxBatchSize <= 0 { t.Error("expected positive max batch size") }
	if cfg.MaxWaitMs <= 0 { t.Error("expected positive max wait ms") }
	if cfg.LatencyTarget <= 0 { t.Error("expected positive latency target") }
}

func TestGetEnv_Scheduler(t *testing.T) {
	t.Setenv("TEST_SCHED_KEY", "val")
	if getEnv("TEST_SCHED_KEY", "fb") != "val" { t.Error("expected val") }
	if getEnv("SCHED_MISSING_XYZ", "fb") != "fb" { t.Error("expected fallback") }
}

func TestNewID_Scheduler(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := newID()
		if ids[id] { t.Errorf("duplicate ID: %s", id) }
		ids[id] = true
	}
}
// pq enqueue
// pq high first
// pq normal before low
// pq nil empty
// pq load shed
// pq len
// pq len by priority
// pq drain order
// batcher single
// batcher multi
// batcher max size
// batcher depths
// metrics avg
// metrics calc
// default config
