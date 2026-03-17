package main

import (
	"testing"
	"time"
)

// ── Rate Limiter Tests ────────────────────────────────────────────────────────

func TestTokenBucket_AllowsUnderBurst(t *testing.T) {
	b := NewTokenBucket(10, 5)
	allowed := 0
	for i := 0; i < 5; i++ {
		if b.Allow() { allowed++ }
	}
	if allowed != 5 { t.Errorf("expected 5 allowed, got %d", allowed) }
}

func TestTokenBucket_BlocksAfterBurst(t *testing.T) {
	b := NewTokenBucket(1, 2)
	b.Allow(); b.Allow()
	if b.Allow() { t.Error("expected block after burst exhausted") }
}

func TestTokenBucket_RefillsOverTime(t *testing.T) {
	b := NewTokenBucket(1000, 1)
	b.Allow()
	time.Sleep(5 * time.Millisecond)
	if !b.Allow() { t.Error("expected allow after refill") }
}

func TestRateLimiter_IsolatesPerKey(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	rl.Allow("a") // exhaust a
	if !rl.Allow("b") { t.Error("b should still be allowed") }
}

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(100, 10)
	for i := 0; i < 10; i++ {
		if !rl.Allow("client-1") { t.Errorf("expected allow at %d", i) }
	}
}

func TestRateLimiter_BlocksAfterBurst(t *testing.T) {
	rl := NewRateLimiter(1, 2)
	rl.Allow("c"); rl.Allow("c")
	if rl.Allow("c") { t.Error("expected block after burst") }
}

// ── Auth Store Tests ──────────────────────────────────────────────────────────

func TestAuthStore_ValidatesKnownKey(t *testing.T) {
	a := NewAuthStore()
	clientID, ok := a.Validate("test-key-1234")
	if !ok { t.Error("expected valid key") }
	if clientID != "client-test" { t.Errorf("wrong client ID: %s", clientID) }
}

func TestAuthStore_RejectsUnknownKey(t *testing.T) {
	a := NewAuthStore()
	_, ok := a.Validate("unknown-key")
	if ok { t.Error("expected invalid key") }
}

func TestAuthStore_Register(t *testing.T) {
	a := NewAuthStore()
	a.Register("new-key", "new-client")
	clientID, ok := a.Validate("new-key")
	if !ok { t.Error("expected registered key to be valid") }
	if clientID != "new-client" { t.Errorf("wrong client: %s", clientID) }
}

func TestAuthStore_PlatformKeyValid(t *testing.T) {
	a := NewAuthStore()
	_, ok := a.Validate("platform-key-5678")
	if !ok { t.Error("expected platform key to be valid") }
}

// ── Validation Tests ──────────────────────────────────────────────────────────

func TestValidate_ChatNeedsPromptOrMessages(t *testing.T) {
	req := &InferenceRequest{TaskType: TaskChat}
	if err := validateRequest(req); err == nil {
		t.Error("expected error for chat with no prompt or messages")
	}
}

func TestValidate_ChatWithPromptOK(t *testing.T) {
	req := &InferenceRequest{TaskType: TaskChat, Prompt: "hello"}
	if err := validateRequest(req); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_ChatWithMessagesOK(t *testing.T) {
	req := &InferenceRequest{TaskType: TaskChat, Messages: []Message{{Role: "user", Content: "hi"}}}
	if err := validateRequest(req); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_SummarizeNeedsPrompt(t *testing.T) {
	req := &InferenceRequest{TaskType: TaskSummarize}
	if err := validateRequest(req); err == nil {
		t.Error("expected error for summarize with no prompt")
	}
}

func TestValidate_SummarizeWithPromptOK(t *testing.T) {
	req := &InferenceRequest{TaskType: TaskSummarize, Prompt: "article text"}
	if err := validateRequest(req); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_EmbedNeedsPromptOrQuery(t *testing.T) {
	req := &InferenceRequest{TaskType: TaskEmbed}
	if err := validateRequest(req); err == nil {
		t.Error("expected error for embed with nothing")
	}
}

func TestValidate_EmbedWithQueryOK(t *testing.T) {
	req := &InferenceRequest{TaskType: TaskEmbed, Query: "search query"}
	if err := validateRequest(req); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_RerankNeedsDocumentsAndQuery(t *testing.T) {
	req := &InferenceRequest{TaskType: TaskRerank, Query: "query"}
	if err := validateRequest(req); err == nil {
		t.Error("expected error for rerank without documents")
	}
}

func TestValidate_RerankOK(t *testing.T) {
	req := &InferenceRequest{
		TaskType:  TaskRerank,
		Query:     "query",
		Documents: []string{"doc1", "doc2"},
	}
	if err := validateRequest(req); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_ClassifyNeedsPrompt(t *testing.T) {
	req := &InferenceRequest{TaskType: TaskClassify}
	if err := validateRequest(req); err == nil {
		t.Error("expected error for classify with no prompt")
	}
}

func TestValidate_NegativeMaxTokens(t *testing.T) {
	req := &InferenceRequest{TaskType: TaskChat, Prompt: "hi", MaxTokens: -1}
	if err := validateRequest(req); err == nil {
		t.Error("expected error for negative max_tokens")
	}
}

func TestValidate_ZeroMaxTokensOK(t *testing.T) {
	req := &InferenceRequest{TaskType: TaskChat, Prompt: "hi", MaxTokens: 0}
	if err := validateRequest(req); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ── Helper Tests ──────────────────────────────────────────────────────────────

func TestNewID_Unique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := newID()
		if ids[id] { t.Errorf("duplicate ID: %s", id) }
		ids[id] = true
	}
}

func TestNewID_Length(t *testing.T) {
	id := newID()
	if len(id) != 16 { t.Errorf("expected 16 chars, got %d", len(id)) }
}

func TestGetEnv_Present(t *testing.T) {
	t.Setenv("TEST_GW_ENV", "hello")
	if getEnv("TEST_GW_ENV", "fallback") != "hello" {
		t.Error("expected env value")
	}
}

func TestGetEnv_Missing(t *testing.T) {
	if getEnv("GW_MISSING_XYZ", "default") != "default" {
		t.Error("expected fallback")
	}
}

func TestInferTask_Chat(t *testing.T) {
	gw := NewGateway("http://localhost:8081")
	_ = gw // just test construction
}

func TestGatewayMetrics_Snapshot(t *testing.T) {
	m := &GatewayMetrics{}
	m.TotalRequests = 5
	snap := m.snapshot()
	if snap["total_requests"] != 5 {
		t.Errorf("expected 5, got %d", snap["total_requests"])
	}
}
// bucket allows
