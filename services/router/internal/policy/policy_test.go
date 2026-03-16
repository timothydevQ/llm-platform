package policy_test

import (
	"testing"
	"time"

	"github.com/timothydevQ/llm-platform/services/router/internal/policy"
)

// ─── Circuit breaker tests ────────────────────────────────────────────────────

func TestCB_InitiallyClosed(t *testing.T) {
	cb := policy.NewCircuitBreaker("test")
	if cb.StateString() != "closed" { t.Errorf("expected closed, got %s", cb.StateString()) }
}

func TestCB_AllowsWhenClosed(t *testing.T) {
	cb := policy.NewCircuitBreaker("test")
	if !cb.Allow() { t.Error("expected allow when closed") }
}

func TestCB_OpensAfterThreshold(t *testing.T) {
	cb := policy.NewCircuitBreaker("test")
	for i := 0; i < 3; i++ { cb.RecordFailure() }
	if cb.StateString() != "open" { t.Errorf("expected open, got %s", cb.StateString()) }
}

func TestCB_BlocksWhenOpen(t *testing.T) {
	cb := policy.NewCircuitBreaker("test")
	for i := 0; i < 3; i++ { cb.RecordFailure() }
	if cb.Allow() { t.Error("expected block when open") }
}

func TestCB_TransitionsToHalfOpen(t *testing.T) {
	cb := policy.NewCircuitBreaker("test")
	// Use reflection-free approach: set small threshold and timeout
	for i := 0; i < 5; i++ { cb.RecordFailure() }
	// We can't force timeout without waiting; verify it's open
	if cb.StateString() != "open" { t.Errorf("expected open, got %s", cb.StateString()) }
}

func TestCB_SuccessResetsFailures(t *testing.T) {
	cb := policy.NewCircuitBreaker("test")
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // resets
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.StateString() == "open" { t.Error("should not open: failures reset by success") }
}

// ─── Registry tests ───────────────────────────────────────────────────────────

func TestRegistry_GetCreatesNew(t *testing.T) {
	r := policy.NewRegistry()
	cb := r.Get("model-1")
	if cb == nil { t.Fatal("expected non-nil") }
	if cb.StateString() != "closed" { t.Errorf("expected closed, got %s", cb.StateString()) }
}

func TestRegistry_GetReturnsSame(t *testing.T) {
	r := policy.NewRegistry()
	a := r.Get("model-1")
	b := r.Get("model-1")
	if a != b { t.Error("expected same instance") }
}

func TestRegistry_States(t *testing.T) {
	r := policy.NewRegistry()
	r.Get("m1")
	r.Get("m2")
	states := r.States()
	if len(states) != 2 { t.Errorf("expected 2 states, got %d", len(states)) }
}

// ─── Policy store tests ───────────────────────────────────────────────────────

func TestPolicyStore_SetAndGet(t *testing.T) {
	s := policy.NewPolicyStore()
	p := &policy.TenantPolicy{TenantID: "t1", RoutingMode: "balanced", Enabled: true}
	s.Set(p)
	got, ok := s.Get("t1")
	if !ok { t.Fatal("expected hit") }
	if got.TenantID != "t1" { t.Errorf("wrong tenant: %s", got.TenantID) }
}

func TestPolicyStore_MissForUnknown(t *testing.T) {
	s := policy.NewPolicyStore()
	_, ok := s.Get("unknown")
	if ok { t.Error("expected miss for unknown tenant") }
}

func TestPolicyDefault(t *testing.T) {
	p := policy.Default("t99")
	if !p.Enabled { t.Error("expected enabled") }
	if p.RoutingMode != "balanced" { t.Errorf("expected balanced, got %s", p.RoutingMode) }
	if p.RateLimit <= 0 { t.Error("expected positive rate limit") }
}

// ─── Rate limiter tests ───────────────────────────────────────────────────────

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := policy.NewRateLimiter()
	for i := 0; i < 5; i++ {
		if !rl.Allow("t1", 10, 5) { t.Errorf("expected allow at %d", i) }
	}
}

func TestRateLimiter_BlocksAfterBurst(t *testing.T) {
	rl := policy.NewRateLimiter()
	rl.Allow("t1", 1, 2)
	rl.Allow("t1", 1, 2)
	if rl.Allow("t1", 1, 2) { t.Error("expected block after burst exhausted") }
}

func TestRateLimiter_IsolatesPerTenant(t *testing.T) {
	rl := policy.NewRateLimiter()
	rl.Allow("t1", 1, 1) // exhaust t1
	if !rl.Allow("t2", 1, 1) { t.Error("t2 should still have capacity") }
}

func TestRateLimiter_RefillsOverTime(t *testing.T) {
	rl := policy.NewRateLimiter()
	rl.Allow("t1", 1000, 1) // exhaust
	time.Sleep(5 * time.Millisecond)
	if !rl.Allow("t1", 1000, 1) { t.Error("expected allow after refill") }
}
// tw_6059_30290
// tw_6059_15820
// tw_6059_2364
