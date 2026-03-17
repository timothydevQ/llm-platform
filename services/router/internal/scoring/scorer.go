// Package scoring implements multi-dimensional model scoring for routing
// decisions. Each candidate model receives scores across five dimensions:
// latency, cost, health, queue depth, and policy. The combined score
// determines which model handles the request.
package scoring

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// ─── Model descriptor ─────────────────────────────────────────────────────────

type ModelRecord struct {
	ModelID      string
	Version      string
	Tier         string // "small"|"medium"|"large"
	Tasks        []string
	CostPer1k    float64
	AvgLatencyMs int
	MaxTokens    int
	ExecutorAddr string
	Enabled      bool
}

func (m *ModelRecord) SupportsTask(task string) bool {
	for _, t := range m.Tasks {
		if t == task {
			return true
		}
	}
	return false
}

// ─── Health tracker ───────────────────────────────────────────────────────────

// HealthTracker maintains rolling error rates and latency percentiles per model.
type HealthTracker struct {
	mu      sync.RWMutex
	records map[string]*modelHealth
}

type modelHealth struct {
	// Circular buffer for last 100 requests (success=1, failure=0)
	outcomes   [100]int32
	latencies  [100]float64 // ms
	head       int
	total      int64
	errors     int64
	p99Latency float64
	lastUpdate time.Time
}

func NewHealthTracker() *HealthTracker {
	return &HealthTracker{records: make(map[string]*modelHealth)}
}

func (h *HealthTracker) get(modelID string) *modelHealth {
	r, ok := h.records[modelID]
	if !ok {
		r = &modelHealth{lastUpdate: time.Now()}
		h.records[modelID] = r
	}
	return r
}

func (h *HealthTracker) RecordSuccess(modelID string, latencyMs float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.get(modelID)
	r.outcomes[r.head%100] = 1
	r.latencies[r.head%100] = latencyMs
	r.head++
	atomic.AddInt64(&r.total, 1)
	r.lastUpdate = time.Now()
	h.updateP99(r)
}

func (h *HealthTracker) RecordFailure(modelID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.get(modelID)
	r.outcomes[r.head%100] = 0
	r.latencies[r.head%100] = 0
	r.head++
	atomic.AddInt64(&r.total, 1)
	atomic.AddInt64(&r.errors, 1)
	r.lastUpdate = time.Now()
}

func (h *HealthTracker) updateP99(r *modelHealth) {
	var lats []float64
	for _, l := range r.latencies {
		if l > 0 {
			lats = append(lats, l)
		}
	}
	if len(lats) == 0 {
		return
	}
	// Simple insertion sort for small slice
	for i := 1; i < len(lats); i++ {
		for j := i; j > 0 && lats[j] < lats[j-1]; j-- {
			lats[j], lats[j-1] = lats[j-1], lats[j]
		}
	}
	idx := int(math.Ceil(float64(len(lats))*0.99)) - 1
	if idx < 0 {
		idx = 0
	}
	r.p99Latency = lats[idx]
}

// ErrorRate returns error rate over last 100 requests (0.0–1.0).
func (h *HealthTracker) ErrorRate(modelID string) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	r, ok := h.records[modelID]
	if !ok {
		return 0
	}
	window := min(int(atomic.LoadInt64(&r.total)), 100)
	if window == 0 {
		return 0
	}
	errors := 0
	for i := 0; i < window; i++ {
		if r.outcomes[i] == 0 {
			errors++
		}
	}
	return float64(errors) / float64(window)
}

func (h *HealthTracker) P99Latency(modelID string) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if r, ok := h.records[modelID]; ok {
		return r.p99Latency
	}
	return 0
}

// ─── Queue depth proxy ────────────────────────────────────────────────────────

// QueueDepthReporter is implemented by the scheduler to report current depths.
type QueueDepthReporter interface {
	QueueDepth(modelID string) int
}

type noopQueue struct{}

func (noopQueue) QueueDepth(_ string) int { return 0 }

// ─── Scorer ───────────────────────────────────────────────────────────────────

// ScoringMode controls the weight of each scoring dimension.
type ScoringMode string

const (
	ModeLatencyOptimized ScoringMode = "latency_optimized"
	ModeCostOptimized    ScoringMode = "cost_optimized"
	ModeBalanced         ScoringMode = "balanced"
)

type weights struct {
	latency float64
	cost    float64
	health  float64
	queue   float64
	policy  float64
}

var modeWeights = map[ScoringMode]weights{
	ModeLatencyOptimized: {latency: 0.50, cost: 0.10, health: 0.25, queue: 0.10, policy: 0.05},
	ModeCostOptimized:    {latency: 0.10, cost: 0.50, health: 0.20, queue: 0.10, policy: 0.10},
	ModeBalanced:         {latency: 0.25, cost: 0.25, health: 0.25, queue: 0.15, policy: 0.10},
}

// CandidateScore is the scoring result for one model.
type CandidateScore struct {
	ModelID      string
	Version      string
	ExecutorAddr string
	TotalScore   float64
	LatencyScore float64
	CostScore    float64
	HealthScore  float64
	QueueScore   float64
	PolicyScore  float64
	RolloutPct   float64
	Reason       string
}

// ScoringRequest carries everything needed to score candidates.
type ScoringRequest struct {
	Task            string
	CostBudget      string // "low"|"medium"|"high"|""
	LatencyTargetMs int32
	PromptLen       int
	TenantMode      ScoringMode
	AllowedModels   map[string]bool // nil = all allowed
	RolloutWeights  map[string]float64
}

// Scorer scores model candidates for a request.
type Scorer struct {
	health  *HealthTracker
	queues  QueueDepthReporter
}

func NewScorer(health *HealthTracker, queues QueueDepthReporter) *Scorer {
	if queues == nil {
		queues = noopQueue{}
	}
	return &Scorer{health: health, queues: queues}
}

// Score scores all candidate models and returns them ranked best-first.
func (s *Scorer) Score(req *ScoringRequest, candidates []*ModelRecord) []*CandidateScore {
	mode := req.TenantMode
	if mode == "" {
		mode = ModeBalanced
	}
	w := modeWeights[mode]
	if _, ok := modeWeights[mode]; !ok {
		w = modeWeights[ModeBalanced]
	}

	var results []*CandidateScore
	for _, m := range candidates {
		// Hard filters
		if !m.Enabled { continue }
		if !m.SupportsTask(req.Task) { continue }
		if req.AllowedModels != nil && len(req.AllowedModels) > 0 {
			if !req.AllowedModels[m.ModelID] { continue }
		}

		ls := s.latencyScore(m, req)
		cs := s.costScore(m, req)
		hs := s.healthScore(m)
		qs := s.queueScore(m)
		ps := s.policyScore(m, req)
		rollout := req.RolloutWeights[m.ModelID]
		if rollout == 0 { rollout = 1.0 }

		total := (w.latency*ls + w.cost*cs + w.health*hs + w.queue*qs + w.policy*ps) * rollout

		results = append(results, &CandidateScore{
			ModelID:      m.ModelID,
			Version:      m.Version,
			ExecutorAddr: m.ExecutorAddr,
			TotalScore:   total,
			LatencyScore: ls,
			CostScore:    cs,
			HealthScore:  hs,
			QueueScore:   qs,
			PolicyScore:  ps,
			RolloutPct:   rollout,
			Reason:       describeReason(m, req),
		})
	}

	// Sort descending by total score
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].TotalScore > results[j-1].TotalScore; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
	return results
}

// latencyScore: 1.0 = best (fastest), 0.0 = worst (slowest)
func (s *Scorer) latencyScore(m *ModelRecord, req *ScoringRequest) float64 {
	// Penalise models exceeding latency target
	if req.LatencyTargetMs > 0 && int32(m.AvgLatencyMs) > req.LatencyTargetMs {
		return 0.0
	}
	// Scale: small=1.0, medium=0.6, large=0.3
	switch m.Tier {
	case "small":  return 1.0
	case "medium": return 0.6
	default:       return 0.3
	}
}

// costScore: 1.0 = cheapest, 0.0 = most expensive
func (s *Scorer) costScore(m *ModelRecord, req *ScoringRequest) float64 {
	switch req.CostBudget {
	case "low":
		if m.Tier == "small"  { return 1.0 }
		if m.Tier == "medium" { return 0.3 }
		return 0.0
	case "high":
		if m.Tier == "large"  { return 1.0 }
		if m.Tier == "medium" { return 0.7 }
		return 0.3
	}
	// Inverse of cost (normalised)
	maxCost := 0.02
	return math.Max(0, 1.0-m.CostPer1k/maxCost)
}

// healthScore: 1.0 = fully healthy, 0.0 = high error rate
func (s *Scorer) healthScore(m *ModelRecord) float64 {
	errRate := s.health.ErrorRate(m.ModelID)
	return math.Max(0, 1.0-errRate*5) // error_rate > 0.2 → 0
}

// queueScore: 1.0 = empty queue, 0.0 = heavily loaded
func (s *Scorer) queueScore(m *ModelRecord) float64 {
	depth := s.queues.QueueDepth(m.ModelID)
	if depth <= 0 { return 1.0 }
	// Score drops linearly: depth=50→0
	return math.Max(0, 1.0-float64(depth)/50.0)
}

// policyScore: based on prompt length vs model capability
func (s *Scorer) policyScore(m *ModelRecord, req *ScoringRequest) float64 {
	// Penalise if prompt is longer than 80% of model's max context
	threshold := int(float64(m.MaxTokens) * 3 * 4) // tokens≈chars/4, context chars
	if req.PromptLen > threshold {
		return 0.2
	}
	return 1.0
}

func describeReason(m *ModelRecord, req *ScoringRequest) string {
	if req.CostBudget == "low" && m.Tier == "small" {
		return "cost-budget=low→small"
	}
	if req.CostBudget == "high" && m.Tier == "large" {
		return "cost-budget=high→large"
	}
	if req.LatencyTargetMs > 0 && int32(m.AvgLatencyMs) <= req.LatencyTargetMs {
		return "latency-target-met"
	}
	return "balanced-score"
}

func min(a, b int) int {
	if a < b { return a }
	return b
}
// fx_469
// fx_470
