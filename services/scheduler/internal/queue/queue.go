// Package queue implements per-model priority queues with load shedding.
// Each model has three priority lanes (high/normal/low) so that critical
// requests are not blocked behind bulk traffic.
package queue

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	inferencev1 "github.com/timothydevQ/llm-platform/gen/inference/v1"
)

// ─── Queue item ───────────────────────────────────────────────────────────────

// Item is a request waiting in a model queue.
type Item struct {
	Req        *inferencev1.InferenceRequest
	ExecutorAddr string
	EnqueuedAt time.Time
	ResultCh   chan *Result
}

// Result is the response for a queued item.
type Result struct {
	Resp    *inferencev1.InferenceResponse
	Err     error
	WaitMs  float64
}

// ─── Priority lanes ───────────────────────────────────────────────────────────

// ModelQueue is a 3-lane priority queue for one model.
type ModelQueue struct {
	mu       sync.Mutex
	high     []*Item
	normal   []*Item
	low      []*Item
	maxDepth int

	// Metrics
	enqueued  int64
	dropped   int64
	dispatched int64
}

func NewModelQueue(maxDepth int) *ModelQueue {
	return &ModelQueue{maxDepth: maxDepth}
}

// Enqueue adds an item to the appropriate priority lane.
// Returns false if the queue is full (load shedding).
func (q *ModelQueue) Enqueue(item *Item) bool {
	q.mu.Lock()
	total := len(q.high) + len(q.normal) + len(q.low)
	if total >= q.maxDepth {
		q.mu.Unlock()
		atomic.AddInt64(&q.dropped, 1)
		return false
	}
	switch item.Req.Priority {
	case inferencev1.PriorityCritical, inferencev1.PriorityHigh:
		q.high = append(q.high, item)
	case inferencev1.PriorityNormal:
		q.normal = append(q.normal, item)
	default:
		q.low = append(q.low, item)
	}
	q.mu.Unlock()
	atomic.AddInt64(&q.enqueued, 1)
	return true
}

// Drain removes up to n items from the queue, respecting priority order.
func (q *ModelQueue) Drain(n int) []*Item {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []*Item
	for len(out) < n {
		if len(q.high) > 0 {
			out = append(out, q.high[0])
			q.high = q.high[1:]
		} else if len(q.normal) > 0 {
			out = append(out, q.normal[0])
			q.normal = q.normal[1:]
		} else if len(q.low) > 0 {
			out = append(out, q.low[0])
			q.low = q.low[1:]
		} else {
			break
		}
	}
	atomic.AddInt64(&q.dispatched, int64(len(out)))
	return out
}

// Depth returns the current total depth.
func (q *ModelQueue) Depth() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.high) + len(q.normal) + len(q.low)
}

// DepthByLane returns (high, normal, low) counts.
func (q *ModelQueue) DepthByLane() (int, int, int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.high), len(q.normal), len(q.low)
}

// Stats returns a snapshot of queue metrics.
func (q *ModelQueue) Stats() QueueStats {
	h, n, l := q.DepthByLane()
	return QueueStats{
		DepthHigh:   h,
		DepthNormal: n,
		DepthLow:    l,
		Enqueued:    atomic.LoadInt64(&q.enqueued),
		Dropped:     atomic.LoadInt64(&q.dropped),
		Dispatched:  atomic.LoadInt64(&q.dispatched),
	}
}

type QueueStats struct {
	DepthHigh   int
	DepthNormal int
	DepthLow    int
	Enqueued    int64
	Dropped     int64
	Dispatched  int64
}

func (s QueueStats) TotalDepth() int { return s.DepthHigh + s.DepthNormal + s.DepthLow }

// ─── Multi-model queue registry ───────────────────────────────────────────────

// Registry manages per-model queues.
type Registry struct {
	mu       sync.RWMutex
	queues   map[string]*ModelQueue
	maxDepth int
}

func NewRegistry(maxDepth int) *Registry {
	return &Registry{queues: make(map[string]*ModelQueue), maxDepth: maxDepth}
}

func (r *Registry) Queue(modelID string) *ModelQueue {
	r.mu.RLock()
	if q, ok := r.queues[modelID]; ok {
		r.mu.RUnlock()
		return q
	}
	r.mu.RUnlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if q, ok := r.queues[modelID]; ok {
		return q
	}
	q := NewModelQueue(r.maxDepth)
	r.queues[modelID] = q
	return q
}

// AllDepths returns the queue depth for every known model.
func (r *Registry) AllDepths() map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]int, len(r.queues))
	for id, q := range r.queues {
		out[id] = q.Depth()
	}
	return out
}

// AllStats returns stats for all queues.
func (r *Registry) AllStats() map[string]QueueStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]QueueStats, len(r.queues))
	for id, q := range r.queues {
		out[id] = q.Stats()
	}
	return out
}

// QueueDepth implements scoring.QueueDepthReporter.
func (r *Registry) QueueDepth(modelID string) int {
	r.mu.RLock()
	q, ok := r.queues[modelID]
	r.mu.RUnlock()
	if !ok { return 0 }
	return q.Depth()
}

// Ensure registry implements the interface used by the scorer.
var _ fmt.Stringer = (*Registry)(nil)

func (r *Registry) String() string {
	depths := r.AllDepths()
	return fmt.Sprintf("queue.Registry{models=%d, depths=%v}", len(depths), depths)
}
// sq_179
// sq_180
// sq_181
// sq_182
// sq_183
// sq_184
// sq_185
// sq_186
// sq_187
// sq_188
// sq_189
// sq_190
// sq_191
// sq_192
// sq_193
// sq_194
// sq_195
// sq_196
// sq_197
// sq_198
// sq_199
// sq_200
// sq_201
// sq_202
// sq_203
// sq_204
