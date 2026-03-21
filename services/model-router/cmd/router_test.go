package main

import (
	"testing"
	"time"
)

// ── Circuit Breaker Tests ─────────────────────────────────────────────────────

func TestCB_InitiallyClosed(t *testing.T) {
	cb := NewCircuitBreaker("test-model")
	if cb.State() != "closed" { t.Errorf("expected closed, got %s", cb.State()) }
}

func TestCB_AllowsWhenClosed(t *testing.T) {
	cb := NewCircuitBreaker("test-model")
	if !cb.Allow() { t.Error("expected allow when closed") }
}

func TestCB_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker("test-model")
	for i := 0; i < 3; i++ { cb.RecordFailure() }
	if cb.State() != "open" { t.Errorf("expected open after 3 failures, got %s", cb.State()) }
}

func TestCB_BlocksWhenOpen(t *testing.T) {
	cb := NewCircuitBreaker("test-model")
	for i := 0; i < 3; i++ { cb.RecordFailure() }
	if cb.Allow() { t.Error("expected block when open") }
}

func TestCB_HalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker("test-model")
	cb.threshold = 1
	cb.timeout = 10 * time.Millisecond
	cb.RecordFailure()
	time.Sleep(20 * time.Millisecond)
	cb.Allow() // triggers half-open
	if cb.State() == "open" { t.Error("expected transition from open after timeout") }
}

func TestCB_ClosesAfterSuccessThreshold(t *testing.T) {
	cb := NewCircuitBreaker("test-model")
	cb.threshold = 1
	cb.timeout = 10 * time.Millisecond
	cb.RecordFailure()
	time.Sleep(20 * time.Millisecond)
	cb.Allow()
	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.State() != "closed" { t.Errorf("expected closed, got %s", cb.State()) }
}

func TestCB_ResetsFailuresOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker("test-model")
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // reset
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() == "open" { t.Error("failures should have been reset") }
}

// ── Canary Tests ──────────────────────────────────────────────────────────────

func TestCanary_DisabledByDefault(t *testing.T) {
	c := NewCanaryConfig()
	if c.ShouldUseCanary("gpt-small") { t.Error("canary should be disabled by default") }
}

func TestCanary_EnabledAfterConfigure(t *testing.T) {
	c := NewCanaryConfig()
	c.Configure("gpt-large", "gpt-medium", 1.0) // 100% canary
	if !c.ShouldUseCanary("gpt-large") { t.Error("expected canary at 100%%") }
}

func TestCanary_WrongModelNotAffected(t *testing.T) {
	c := NewCanaryConfig()
	c.Configure("gpt-large", "gpt-medium", 1.0)
	if c.ShouldUseCanary("gpt-small") { t.Error("canary should not affect non-primary models") }
}

func TestCanary_GetCanary(t *testing.T) {
	c := NewCanaryConfig()
	c.Configure("gpt-large", "gpt-medium", 0.5)
	if c.GetCanary() != "gpt-medium" { t.Errorf("expected gpt-medium, got %s", c.GetCanary()) }
}

func TestCanary_DisableStopsRouting(t *testing.T) {
	c := NewCanaryConfig()
	c.Configure("gpt-large", "gpt-medium", 1.0)
	c.Disable()
	if c.ShouldUseCanary("gpt-large") { t.Error("canary should be disabled") }
}

func TestCanary_ZeroTrafficNeverRoutes(t *testing.T) {
	c := NewCanaryConfig()
	c.Configure("gpt-large", "gpt-medium", 0.0)
	routed := 0
	for i := 0; i < 100; i++ {
		if c.ShouldUseCanary("gpt-large") { routed++ }
	}
	if routed > 0 { t.Errorf("expected 0 canary routes at 0%%, got %d", routed) }
}

// ── Routing Tests ─────────────────────────────────────────────────────────────

func TestRouter_ModelsForTask_Chat(t *testing.T) {
	r := NewRouter()
	models := r.modelsForTask(TaskChat)
	if len(models) == 0 { t.Error("expected models for chat task") }
}

func TestRouter_ModelsForTask_Embed(t *testing.T) {
	r := NewRouter()
	models := r.modelsForTask(TaskEmbed)
	if len(models) == 0 { t.Error("expected models for embed task") }
	for _, m := range models {
		hasEmbed := false
		for _, task := range m.Tasks {
			if task == TaskEmbed { hasEmbed = true }
		}
		if !hasEmbed { t.Errorf("model %s does not support embed", m.ID) }
	}
}

func TestRouter_ModelsForTask_Rerank(t *testing.T) {
	r := NewRouter()
	models := r.modelsForTask(TaskRerank)
	if len(models) == 0 { t.Error("expected models for rerank task") }
}

func TestRouter_RouteChat(t *testing.T) {
	r := NewRouter()
	req := &InferenceRequest{ID: "r1", TaskType: TaskChat, Prompt: "hello"}
	decision, err := r.Route(req)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if decision.Model == nil { t.Error("expected model in decision") }
}

func TestRouter_RouteEmbed(t *testing.T) {
	r := NewRouter()
	req := &InferenceRequest{ID: "r2", TaskType: TaskEmbed, Query: "search term"}
	decision, err := r.Route(req)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if decision.Model.ID != "embed-v2" {
		t.Errorf("expected embed-v2, got %s", decision.Model.ID)
	}
}

func TestRouter_RouteRerank(t *testing.T) {
	r := NewRouter()
	req := &InferenceRequest{ID: "r3", TaskType: TaskRerank, Query: "q", Documents: []string{"d1"}}
	decision, err := r.Route(req)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if decision.Model.ID != "rerank-v1" {
		t.Errorf("expected rerank-v1, got %s", decision.Model.ID)
	}
}

func TestRouter_FallbackWhenPrimaryOpen(t *testing.T) {
	r := NewRouter()
	// Open circuit on gpt-small
	for i := 0; i < 3; i++ { r.breakers["gpt-small"].RecordFailure() }
	req := &InferenceRequest{ID: "r4", TaskType: TaskChat, Prompt: "hi", CostBudget: "low"}
	decision, err := r.Route(req)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if !decision.IsFallback { t.Error("expected fallback routing") }
}

func TestRouter_UnknownTaskReturnsError(t *testing.T) {
	r := NewRouter()
	req := &InferenceRequest{ID: "r5", TaskType: "unknown_task"}
	_, err := r.Route(req)
	if err == nil { t.Error("expected error for unknown task") }
}

func TestRouter_AllCBsOpenReturnsError(t *testing.T) {
	r := NewRouter()
	// Open all chat model circuits
	for _, m := range r.models {
		for _, task := range m.Tasks {
			if task == TaskEmbed {
				for i := 0; i < 3; i++ { r.breakers[m.ID].RecordFailure() }
			}
		}
	}
	req := &InferenceRequest{ID: "r6", TaskType: TaskEmbed, Query: "q"}
	_, err := r.Route(req)
	if err == nil { t.Error("expected error when all circuits open") }
}

func TestRouter_CircuitBreakerStates(t *testing.T) {
	r := NewRouter()
	states := r.CircuitBreakerStates()
	if len(states) == 0 { t.Error("expected circuit breaker states") }
	for _, state := range states {
		if state != "closed" { t.Errorf("expected all closed initially, got %s", state) }
	}
}

// ── SelectTier Tests ──────────────────────────────────────────────────────────

func TestSelectTier_LowBudgetSmall(t *testing.T) {
	req := &InferenceRequest{CostBudget: "low", Prompt: "hi"}
	if selectTier(req) != TierSmall { t.Error("expected small for low budget") }
}

func TestSelectTier_HighBudgetLarge(t *testing.T) {
	req := &InferenceRequest{CostBudget: "high", Prompt: "hi"}
	if selectTier(req) != TierLarge { t.Error("expected large for high budget") }
}

func TestSelectTier_ShortPromptSmall(t *testing.T) {
	req := &InferenceRequest{Prompt: "hello"}
	if selectTier(req) != TierSmall { t.Error("expected small for short prompt") }
}

func TestSelectTier_LongPromptLarge(t *testing.T) {
	prompt := make([]byte, 2500)
	for i := range prompt { prompt[i] = 'a' }
	req := &InferenceRequest{Prompt: string(prompt)}
	if selectTier(req) != TierLarge { t.Error("expected large for long prompt") }
}

func TestSelectTier_LowLatencyTargetSmall(t *testing.T) {
	req := &InferenceRequest{LatencyTarget: 200, Prompt: "medium length prompt here"}
	if selectTier(req) != TierSmall { t.Error("expected small for low latency target") }
}

func TestRouter_RecordSuccessAndFailure(t *testing.T) {
	r := NewRouter()
	r.RecordSuccess("gpt-small")
	r.RecordFailure("gpt-small")
	// Just verify no panic
}

func TestGetEnv_Router(t *testing.T) {
	t.Setenv("TEST_ROUTER_KEY", "val")
	if getEnv("TEST_ROUTER_KEY", "fb") != "val" { t.Error("expected val") }
	if getEnv("ROUTER_MISSING_XYZ", "fb") != "fb" { t.Error("expected fallback") }
}
// cb initial
// cb allows
// cb opens
