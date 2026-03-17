package queue_test

import (
	"testing"
	"time"

	inferencev1 "github.com/timothydevQ/llm-platform/gen/inference/v1"
	"github.com/timothydevQ/llm-platform/services/scheduler/internal/queue"
)

func makeItem(priority inferencev1.Priority) *queue.Item {
	return &queue.Item{
		Req: &inferencev1.InferenceRequest{
			RequestId: "req-" + priority.String(),
			Priority:  priority,
		},
		EnqueuedAt: time.Now(),
		ResultCh:   make(chan *queue.Result, 1),
	}
}

// ─── ModelQueue tests ─────────────────────────────────────────────────────────

func TestModelQueue_EnqueueAndDrain(t *testing.T) {
	q := queue.NewModelQueue(100)
	q.Enqueue(makeItem(inferencev1.PriorityNormal))
	items := q.Drain(10)
	if len(items) != 1 { t.Errorf("expected 1, got %d", len(items)) }
}

func TestModelQueue_EmptyDrainReturnsEmpty(t *testing.T) {
	q := queue.NewModelQueue(100)
	items := q.Drain(10)
	if len(items) != 0 { t.Errorf("expected 0, got %d", len(items)) }
}

func TestModelQueue_HighPriorityFirst(t *testing.T) {
	q := queue.NewModelQueue(100)
	q.Enqueue(makeItem(inferencev1.PriorityLow))
	q.Enqueue(makeItem(inferencev1.PriorityHigh))
	q.Enqueue(makeItem(inferencev1.PriorityNormal))
	items := q.Drain(1)
	if items[0].Req.Priority != inferencev1.PriorityHigh {
		t.Errorf("expected high first, got %v", items[0].Req.Priority)
	}
}

func TestModelQueue_CriticalHigherThanHigh(t *testing.T) {
	q := queue.NewModelQueue(100)
	q.Enqueue(makeItem(inferencev1.PriorityHigh))
	q.Enqueue(makeItem(inferencev1.PriorityCritical))
	items := q.Drain(1)
	if items[0].Req.Priority != inferencev1.PriorityCritical {
		t.Errorf("expected critical first, got %v", items[0].Req.Priority)
	}
}

func TestModelQueue_NormalBeforeLow(t *testing.T) {
	q := queue.NewModelQueue(100)
	q.Enqueue(makeItem(inferencev1.PriorityLow))
	q.Enqueue(makeItem(inferencev1.PriorityNormal))
	items := q.Drain(1)
	if items[0].Req.Priority != inferencev1.PriorityNormal {
		t.Errorf("expected normal before low, got %v", items[0].Req.Priority)
	}
}

func TestModelQueue_LoadShedding(t *testing.T) {
	q := queue.NewModelQueue(2)
	q.Enqueue(makeItem(inferencev1.PriorityNormal))
	q.Enqueue(makeItem(inferencev1.PriorityNormal))
	ok := q.Enqueue(makeItem(inferencev1.PriorityNormal))
	if ok { t.Error("expected load shedding when queue full") }
}

func TestModelQueue_DepthTracking(t *testing.T) {
	q := queue.NewModelQueue(100)
	if q.Depth() != 0 { t.Error("expected 0 initially") }
	q.Enqueue(makeItem(inferencev1.PriorityNormal))
	if q.Depth() != 1 { t.Error("expected 1 after enqueue") }
	q.Drain(1)
	if q.Depth() != 0 { t.Error("expected 0 after drain") }
}

func TestModelQueue_DepthByLane(t *testing.T) {
	q := queue.NewModelQueue(100)
	q.Enqueue(makeItem(inferencev1.PriorityHigh))
	q.Enqueue(makeItem(inferencev1.PriorityHigh))
	q.Enqueue(makeItem(inferencev1.PriorityNormal))
	q.Enqueue(makeItem(inferencev1.PriorityLow))
	h, n, l := q.DepthByLane()
	if h != 2 { t.Errorf("expected 2 high, got %d", h) }
	if n != 1 { t.Errorf("expected 1 normal, got %d", n) }
	if l != 1 { t.Errorf("expected 1 low, got %d", l) }
}

func TestModelQueue_DrainNMoreThanAvailable(t *testing.T) {
	q := queue.NewModelQueue(100)
	q.Enqueue(makeItem(inferencev1.PriorityNormal))
	items := q.Drain(10)
	if len(items) != 1 { t.Errorf("drain should return available items, got %d", len(items)) }
}

func TestModelQueue_Stats(t *testing.T) {
	q := queue.NewModelQueue(100)
	q.Enqueue(makeItem(inferencev1.PriorityNormal))
	q.Drain(1)
	s := q.Stats()
	if s.Enqueued != 1 { t.Errorf("expected 1 enqueued, got %d", s.Enqueued) }
	if s.Dispatched != 1 { t.Errorf("expected 1 dispatched, got %d", s.Dispatched) }
}

func TestModelQueue_DroppedCountsOnShed(t *testing.T) {
	q := queue.NewModelQueue(1)
	q.Enqueue(makeItem(inferencev1.PriorityNormal))
	q.Enqueue(makeItem(inferencev1.PriorityNormal)) // shed
	s := q.Stats()
	if s.Dropped != 1 { t.Errorf("expected 1 dropped, got %d", s.Dropped) }
}

// ─── Registry tests ───────────────────────────────────────────────────────────

func TestRegistry_GetCreatesQueue(t *testing.T) {
	r := queue.NewRegistry(100)
	q := r.Queue("model-1")
	if q == nil { t.Error("expected non-nil queue") }
}

func TestRegistry_GetReturnsSameInstance(t *testing.T) {
	r := queue.NewRegistry(100)
	q1 := r.Queue("model-1")
	q2 := r.Queue("model-1")
	if q1 != q2 { t.Error("expected same queue instance") }
}

func TestRegistry_AllDepths(t *testing.T) {
	r := queue.NewRegistry(100)
	r.Queue("model-1").Enqueue(makeItem(inferencev1.PriorityNormal))
	r.Queue("model-2").Enqueue(makeItem(inferencev1.PriorityNormal))
	r.Queue("model-2").Enqueue(makeItem(inferencev1.PriorityHigh))
	depths := r.AllDepths()
	if depths["model-1"] != 1 { t.Errorf("expected 1 for model-1, got %d", depths["model-1"]) }
	if depths["model-2"] != 2 { t.Errorf("expected 2 for model-2, got %d", depths["model-2"]) }
}

func TestRegistry_QueueDepth_Unknown(t *testing.T) {
	r := queue.NewRegistry(100)
	if r.QueueDepth("unknown") != 0 { t.Error("expected 0 for unknown model") }
}

func TestRegistry_AllStats(t *testing.T) {
	r := queue.NewRegistry(100)
	r.Queue("model-1").Enqueue(makeItem(inferencev1.PriorityNormal))
	stats := r.AllStats()
	if _, ok := stats["model-1"]; !ok { t.Error("expected stats for model-1") }
}

func TestQueueStats_TotalDepth(t *testing.T) {
	s := queue.QueueStats{DepthHigh: 2, DepthNormal: 3, DepthLow: 1}
	if s.TotalDepth() != 6 { t.Errorf("expected 6, got %d", s.TotalDepth()) }
}
// tw_6059_29283
// tw_6059_8675
// tw_6059_6376
