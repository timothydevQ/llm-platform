// Package policy provides circuit breakers and tenant routing policy enforcement.
package policy

import (
	"sync"
	"sync/atomic"
	"time"
)

// ─── Circuit breaker ──────────────────────────────────────────────────────────

type cbState int32

const (
	cbClosed   cbState = 0
	cbOpen     cbState = 1
	cbHalfOpen cbState = 2
)

// CircuitBreaker is a per-model circuit breaker with configurable thresholds.
type CircuitBreaker struct {
	mu          sync.Mutex
	state       cbState
	failures    int
	successes   int
	threshold   int           // failures before opening
	successReq  int           // successes in half-open before closing
	timeout     time.Duration // how long to stay open
	lastFail    time.Time
	modelID     string
}

func NewCircuitBreaker(modelID string) *CircuitBreaker {
	return &CircuitBreaker{
		modelID:    modelID,
		threshold:  3,
		successReq: 2,
		timeout:    20 * time.Second,
	}
}

// Allow returns true if a request should be forwarded to this model.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case cbOpen:
		if time.Since(cb.lastFail) > cb.timeout {
			cb.state = cbHalfOpen
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
	if cb.state == cbHalfOpen {
		cb.successes++
		if cb.successes >= cb.successReq {
			cb.state = cbClosed
		}
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.successes = 0
	cb.lastFail = time.Now()
	if cb.state == cbHalfOpen || cb.failures >= cb.threshold {
		cb.state = cbOpen
	}
}

func (cb *CircuitBreaker) StateString() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case cbOpen:     return "open"
	case cbHalfOpen: return "half-open"
	default:         return "closed"
	}
}

// ─── Breaker registry ─────────────────────────────────────────────────────────

// Registry manages per-model circuit breakers.
type Registry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
}

func NewRegistry() *Registry {
	return &Registry{breakers: make(map[string]*CircuitBreaker)}
}

func (r *Registry) Get(modelID string) *CircuitBreaker {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cb, ok := r.breakers[modelID]; ok {
		return cb
	}
	cb := NewCircuitBreaker(modelID)
	r.breakers[modelID] = cb
	return cb
}

func (r *Registry) States() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]string, len(r.breakers))
	for id, cb := range r.breakers {
		out[id] = cb.StateString()
	}
	return out
}

// ─── Tenant policy ────────────────────────────────────────────────────────────

// TenantPolicy is the routing policy for one tenant.
type TenantPolicy struct {
	TenantID      string
	RoutingMode   string // "latency_optimized"|"cost_optimized"|"balanced"
	AllowedModels map[string]bool
	RateLimit     int     // req/s
	BurstLimit    int
	Enabled       bool
}

// PolicyStore caches tenant policies with a short TTL.
type PolicyStore struct {
	mu      sync.RWMutex
	entries map[string]*policyEntry
}

type policyEntry struct {
	policy    *TenantPolicy
	fetchedAt time.Time
}

const policyTTL = 30 * time.Second

func NewPolicyStore() *PolicyStore {
	return &PolicyStore{entries: make(map[string]*policyEntry)}
}

func (s *PolicyStore) Get(tenantID string) (*TenantPolicy, bool) {
	s.mu.RLock()
	e, ok := s.entries[tenantID]
	s.mu.RUnlock()
	if !ok || time.Since(e.fetchedAt) > policyTTL {
		return nil, false
	}
	return e.policy, true
}

func (s *PolicyStore) Set(p *TenantPolicy) {
	s.mu.Lock()
	s.entries[p.TenantID] = &policyEntry{policy: p, fetchedAt: time.Now()}
	s.mu.Unlock()
}

// Default returns a permissive default policy for unknown tenants.
func Default(tenantID string) *TenantPolicy {
	return &TenantPolicy{
		TenantID:    tenantID,
		RoutingMode: "balanced",
		RateLimit:   50,
		BurstLimit:  100,
		Enabled:     true,
	}
}

// ─── Rate limiter ─────────────────────────────────────────────────────────────

// RateLimiter is a per-tenant token bucket rate limiter.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens   float64
	maxBurst float64
	rate     float64 // tokens/second
	lastFill time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{buckets: make(map[string]*bucket)}
}

func (rl *RateLimiter) Allow(tenantID string, rateRps, burst int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	b, ok := rl.buckets[tenantID]
	if !ok {
		b = &bucket{tokens: float64(burst), maxBurst: float64(burst), rate: float64(rateRps), lastFill: time.Now()}
		rl.buckets[tenantID] = b
	}
	// Update rate if policy changed
	b.rate = float64(rateRps)
	b.maxBurst = float64(burst)
	now := time.Now()
	elapsed := now.Sub(b.lastFill).Seconds()
	b.tokens = min64(b.maxBurst, b.tokens+elapsed*b.rate)
	b.lastFill = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// ─── Reject metrics ───────────────────────────────────────────────────────────

type RejectMetrics struct {
	RateLimited int64
	CBBlocked   int64
	PolicyDenied int64
}

func (m *RejectMetrics) AddRateLimited()   { atomic.AddInt64(&m.RateLimited, 1) }
func (m *RejectMetrics) AddCBBlocked()     { atomic.AddInt64(&m.CBBlocked, 1) }
func (m *RejectMetrics) AddPolicyDenied()  { atomic.AddInt64(&m.PolicyDenied, 1) }

func min64(a, b float64) float64 {
	if a < b { return a }
	return b
}
// tw_6059_25631
// tw_6059_8537
// tw_6059_20132
// tw_6059_25146
// tw_6059_30215
// tw_6059_28970
// tw_6059_8088
// tw_6059_17986
// tw_6059_27769
// tw_6059_29299
// tw_6059_14666
// tw_6059_5909
// tw_6059_25840
// tw_6059_7170
// tw_6059_8152
// tw_6059_1784
// tw_6059_19680
// tw_6059_25446
// tw_6059_7143
// tw_6059_15465
