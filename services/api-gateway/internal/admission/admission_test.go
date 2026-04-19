package admission_test

import (
	"strings"
	"testing"
	"time"

	inferencev1 "github.com/timothydevQ/llm-platform/gen/inference/v1"
	"github.com/timothydevQ/llm-platform/services/api-gateway/internal/admission"
)

func newAdmission() *admission.Admission {
	return admission.New(admission.DefaultConfig())
}

func chatReq(prompt string) *inferencev1.InferenceRequest {
	return &inferencev1.InferenceRequest{
		RequestId: "r1", TaskType: inferencev1.TaskChat, Prompt: prompt,
	}
}

// ─── Task validation ──────────────────────────────────────────────────────────

func TestAdmit_ChatWithPrompt(t *testing.T) {
	a := newAdmission()
	if err := a.Admit(chatReq("hello")); err != nil { t.Errorf("unexpected: %v", err) }
}

func TestAdmit_ChatWithMessages(t *testing.T) {
	a := newAdmission()
	req := &inferencev1.InferenceRequest{
		TaskType: inferencev1.TaskChat,
		Messages: []*inferencev1.ChatMessage{{Role: "user", Content: "hi"}},
	}
	if err := a.Admit(req); err != nil { t.Errorf("unexpected: %v", err) }
}

func TestAdmit_ChatNoContentRejectsError(t *testing.T) {
	a := newAdmission()
	req := &inferencev1.InferenceRequest{TaskType: inferencev1.TaskChat}
	if err := a.Admit(req); err == nil { t.Error("expected error for empty chat") }
}

func TestAdmit_EmbedWithQuery(t *testing.T) {
	a := newAdmission()
	req := &inferencev1.InferenceRequest{TaskType: inferencev1.TaskEmbed, Query: "q"}
	if err := a.Admit(req); err != nil { t.Errorf("unexpected: %v", err) }
}

func TestAdmit_EmbedNoContentRejectsError(t *testing.T) {
	a := newAdmission()
	req := &inferencev1.InferenceRequest{TaskType: inferencev1.TaskEmbed}
	if err := a.Admit(req); err == nil { t.Error("expected error") }
}

func TestAdmit_RerankOK(t *testing.T) {
	a := newAdmission()
	req := &inferencev1.InferenceRequest{
		TaskType:  inferencev1.TaskRerank,
		Query:     "q",
		Documents: []string{"d1", "d2"},
	}
	if err := a.Admit(req); err != nil { t.Errorf("unexpected: %v", err) }
}

func TestAdmit_RerankNoQuery(t *testing.T) {
	a := newAdmission()
	req := &inferencev1.InferenceRequest{TaskType: inferencev1.TaskRerank, Documents: []string{"d1"}}
	if err := a.Admit(req); err == nil { t.Error("expected error: no query") }
}

func TestAdmit_RerankNoDocs(t *testing.T) {
	a := newAdmission()
	req := &inferencev1.InferenceRequest{TaskType: inferencev1.TaskRerank, Query: "q"}
	if err := a.Admit(req); err == nil { t.Error("expected error: no docs") }
}

func TestAdmit_ClassifyNeedsPrompt(t *testing.T) {
	a := newAdmission()
	req := &inferencev1.InferenceRequest{TaskType: inferencev1.TaskClassify}
	if err := a.Admit(req); err == nil { t.Error("expected error") }
}

func TestAdmit_UnspecifiedTask(t *testing.T) {
	a := newAdmission()
	req := &inferencev1.InferenceRequest{TaskType: inferencev1.TaskUnspecified, Prompt: "hi"}
	if err := a.Admit(req); err == nil { t.Error("expected error for unspecified task") }
}

// ─── Size limits ──────────────────────────────────────────────────────────────

func TestAdmit_OversizePromptRejected(t *testing.T) {
	cfg := admission.DefaultConfig()
	cfg.MaxPromptBytes = 100
	a := admission.New(cfg)
	req := chatReq(strings.Repeat("x", 200))
	if err := a.Admit(req); err == nil { t.Error("expected error for oversize prompt") }
}

func TestAdmit_NormalSizeAccepted(t *testing.T) {
	a := newAdmission()
	req := chatReq(strings.Repeat("x", 1000))
	if err := a.Admit(req); err != nil { t.Errorf("unexpected: %v", err) }
}

// ─── Token limits ─────────────────────────────────────────────────────────────

func TestAdmit_NegativeMaxTokensRejected(t *testing.T) {
	a := newAdmission()
	req := chatReq("hi")
	req.MaxTokens = -1
	if err := a.Admit(req); err == nil { t.Error("expected error for negative max_tokens") }
}

func TestAdmit_ExcessiveMaxTokensRejected(t *testing.T) {
	cfg := admission.DefaultConfig()
	cfg.MaxTokens = 512
	a := admission.New(cfg)
	req := chatReq("hi")
	req.MaxTokens = 1000
	if err := a.Admit(req); err == nil { t.Error("expected error for excessive max_tokens") }
}

func TestAdmit_ZeroMaxTokensNormalised(t *testing.T) {
	a := newAdmission()
	req := chatReq("hi")
	req.MaxTokens = 0
	a.Admit(req)
	if req.MaxTokens == 0 { t.Error("expected max_tokens normalised to default") }
}

// ─── Document limits ──────────────────────────────────────────────────────────

func TestAdmit_TooManyDocsRejected(t *testing.T) {
	cfg := admission.DefaultConfig()
	cfg.MaxDocuments = 3
	a := admission.New(cfg)
	docs := make([]string, 4)
	req := &inferencev1.InferenceRequest{TaskType: inferencev1.TaskRerank, Query: "q", Documents: docs}
	if err := a.Admit(req); err == nil { t.Error("expected error for too many docs") }
}

// ─── Deadline normalisation ───────────────────────────────────────────────────

func TestAdmit_NoDeadlineGetsDefault(t *testing.T) {
	a := newAdmission()
	req := chatReq("hi")
	req.DeadlineUnixMs = 0
	a.Admit(req)
	if req.DeadlineUnixMs == 0 { t.Error("expected deadline to be set") }
}

func TestAdmit_DeadlineCappedAtMax(t *testing.T) {
	cfg := admission.DefaultConfig()
	cfg.DeadlineMax = 5 * time.Second
	a := admission.New(cfg)
	req := chatReq("hi")
	req.DeadlineUnixMs = time.Now().Add(300 * time.Second).UnixMilli() // too far
	a.Admit(req)
	expected := time.Now().Add(5 * time.Second).UnixMilli()
	if req.DeadlineUnixMs > expected+200 { t.Error("deadline not capped") }
}

// ─── Metadata initialisation ──────────────────────────────────────────────────

func TestAdmit_MetadataInitialised(t *testing.T) {
	a := newAdmission()
	req := chatReq("hi")
	req.Metadata = nil
	a.Admit(req)
	if req.Metadata == nil { t.Error("expected metadata initialised") }
}

// ─── DeadlineRemaining ────────────────────────────────────────────────────────

func TestDeadlineRemaining_Zero(t *testing.T) {
	req := &inferencev1.InferenceRequest{DeadlineUnixMs: 0}
	d := admission.DeadlineRemaining(req)
	if d <= 0 { t.Error("expected positive default deadline") }
}

func TestDeadlineRemaining_Future(t *testing.T) {
	req := &inferencev1.InferenceRequest{
		DeadlineUnixMs: time.Now().Add(10 * time.Second).UnixMilli(),
	}
	d := admission.DeadlineRemaining(req)
	if d < 9*time.Second { t.Errorf("expected ~10s, got %v", d) }
}
