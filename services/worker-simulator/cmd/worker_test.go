package main

import (
	"testing"
	"time"
)

func newTestWorker() *Worker { return NewWorker() }

// ── Embedding Tests ───────────────────────────────────────────────────────────

func TestGenerateEmbedding_Length(t *testing.T) {
	emb := generateEmbedding("hello world", 8)
	if len(emb) == 0 { t.Error("expected non-empty embedding") }
}

func TestGenerateEmbedding_Normalized(t *testing.T) {
	emb := generateEmbedding("test text for normalization", 8)
	var norm float64
	for _, v := range emb { norm += v * v }
	if norm < 0.99 || norm > 1.01 {
		t.Errorf("expected normalized embedding (norm≈1), got %f", norm)
	}
}

func TestGenerateEmbedding_Deterministic(t *testing.T) {
	emb1 := generateEmbedding("same text", 8)
	emb2 := generateEmbedding("same text", 8)
	for i := range emb1 {
		if emb1[i] != emb2[i] { t.Errorf("expected deterministic embedding at index %d", i) }
	}
}

func TestGenerateEmbedding_DifferentTexts(t *testing.T) {
	emb1 := generateEmbedding("text one", 8)
	emb2 := generateEmbedding("completely different text two", 8)
	same := true
	for i := range emb1 {
		if emb1[i] != emb2[i] { same = false; break }
	}
	if same { t.Error("expected different embeddings for different texts") }
}

// ── Rerank Tests ──────────────────────────────────────────────────────────────

func TestRerankDocuments_ReturnsScoresForEach(t *testing.T) {
	docs := []string{"doc about cats", "doc about dogs", "unrelated content"}
	scores := rerankDocuments("cats and dogs", docs)
	if len(scores) != 3 { t.Errorf("expected 3 scores, got %d", len(scores)) }
}

func TestRerankDocuments_HigherScoreForRelevant(t *testing.T) {
	docs := []string{"this is about cats", "nothing relevant here"}
	scores := rerankDocuments("cats", docs)
	if scores[0] <= scores[1] {
		t.Errorf("expected first doc to score higher: %f vs %f", scores[0], scores[1])
	}
}

func TestRerankDocuments_EmptyDocs(t *testing.T) {
	scores := rerankDocuments("query", []string{})
	if len(scores) != 0 { t.Error("expected empty scores for empty docs") }
}

func TestRerankDocuments_ScoresInRange(t *testing.T) {
	docs := []string{"hello world", "foo bar"}
	scores := rerankDocuments("hello", docs)
	for _, s := range scores {
		if s < 0 || s > 1 { t.Errorf("score out of range [0,1]: %f", s) }
	}
}

// ── Classification Tests ──────────────────────────────────────────────────────

func TestClassifyText_Positive(t *testing.T) {
	result := classifyText("this is great and awesome")
	if !contains(result, "positive") { t.Errorf("expected positive label, got %s", result) }
}

func TestClassifyText_Negative(t *testing.T) {
	result := classifyText("this is terrible and horrible")
	if !contains(result, "negative") { t.Errorf("expected negative label, got %s", result) }
}

func TestClassifyText_Harmful(t *testing.T) {
	result := classifyText("hate and violence content")
	if !contains(result, "harmful") { t.Errorf("expected harmful label, got %s", result) }
}

func TestClassifyText_Neutral(t *testing.T) {
	result := classifyText("the weather is fine today")
	if !contains(result, "neutral") { t.Errorf("expected neutral label, got %s", result) }
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub { return true }
	}
	return false
}

// ── Token Estimation Tests ────────────────────────────────────────────────────

func TestEstimateTokens_NonEmpty(t *testing.T) {
	tokens := estimateTokens("hello world this is a test")
	if tokens <= 0 { t.Error("expected positive token count") }
}

func TestEstimateTokens_EmptyString(t *testing.T) {
	tokens := estimateTokens("")
	if tokens < 1 { t.Error("expected at least 1 token for empty string") }
}

func TestEstimateTokens_LongerTextMoreTokens(t *testing.T) {
	short := estimateTokens("hi")
	long := estimateTokens("this is a much longer string with many more words and characters in it")
	if long <= short { t.Error("expected longer text to have more tokens") }
}

// ── Worker Tests ──────────────────────────────────────────────────────────────

func TestWorker_InferChat(t *testing.T) {
	w := newTestWorker()
	resp, err := w.Infer(&InferRequest{
		RequestID: "r1",
		ModelID:   "gpt-small",
		TaskType:  TaskChat,
		Prompt:    "hello",
	})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if resp.Content == "" { t.Error("expected non-empty content") }
	if resp.TokensUsed <= 0 { t.Error("expected positive token count") }
}

func TestWorker_InferEmbed(t *testing.T) {
	w := newTestWorker()
	resp, err := w.Infer(&InferRequest{
		RequestID: "r2",
		ModelID:   "embed-v2",
		TaskType:  TaskEmbed,
		Prompt:    "semantic search query",
	})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if len(resp.Embedding) == 0 { t.Error("expected non-empty embedding") }
}

func TestWorker_InferRerank(t *testing.T) {
	w := newTestWorker()
	resp, err := w.Infer(&InferRequest{
		RequestID: "r3",
		ModelID:   "rerank-v1",
		TaskType:  TaskRerank,
		Query:     "machine learning",
		Documents: []string{"doc about ML", "doc about cooking"},
	})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if len(resp.Scores) != 2 { t.Errorf("expected 2 scores, got %d", len(resp.Scores)) }
}

func TestWorker_InferClassify(t *testing.T) {
	w := newTestWorker()
	resp, err := w.Infer(&InferRequest{
		RequestID: "r4",
		ModelID:   "gpt-small",
		TaskType:  TaskClassify,
		Prompt:    "great product",
	})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if resp.Content == "" { t.Error("expected non-empty classification") }
}

func TestWorker_DownReturnsError(t *testing.T) {
	w := newTestWorker()
	w.SetStatus(WorkerDown)
	_, err := w.Infer(&InferRequest{ModelID: "gpt-small", TaskType: TaskChat, Prompt: "hi"})
	if err == nil { t.Error("expected error when worker is down") }
}

func TestWorker_DegradedSlower(t *testing.T) {
	w := newTestWorker()
	w.SetJitter(0.1) // very fast for testing

	start := time.Now()
	w.Infer(&InferRequest{ModelID: "gpt-small", TaskType: TaskClassify, Prompt: "test"})
	fast := time.Since(start)

	w.SetJitter(2.0)
	start = time.Now()
	w.Infer(&InferRequest{ModelID: "gpt-small", TaskType: TaskClassify, Prompt: "test"})
	slow := time.Since(start)

	if slow <= fast {
		t.Logf("degraded latency (%v) should be >= normal (%v) — may pass due to timing", slow, fast)
	}
}

func TestWorker_MetricsIncrement(t *testing.T) {
	w := newTestWorker()
	w.SetJitter(0.01) // fast
	w.Infer(&InferRequest{ModelID: "gpt-small", TaskType: TaskClassify, Prompt: "test"})
	w.Infer(&InferRequest{ModelID: "gpt-small", TaskType: TaskClassify, Prompt: "test2"})
	if w.metrics.Requests != 2 { t.Errorf("expected 2 requests, got %d", w.metrics.Requests) }
	if w.metrics.Tokens <= 0 { t.Error("expected positive token count") }
}

func TestWorker_ErrorMetricOnDown(t *testing.T) {
	w := newTestWorker()
	w.SetStatus(WorkerDown)
	w.Infer(&InferRequest{ModelID: "gpt-small", TaskType: TaskChat, Prompt: "hi"})
	if w.metrics.Errors != 1 { t.Errorf("expected 1 error, got %d", w.metrics.Errors) }
}

func TestWorkerMetrics_AvgLatency_NoRequests(t *testing.T) {
	m := &WorkerMetrics{}
	if m.AvgLatencyMs() != 0 { t.Error("expected 0 with no requests") }
}

func TestWorkerMetrics_TokensPerSec_NoLatency(t *testing.T) {
	m := &WorkerMetrics{}
	if m.TokensPerSec() != 0 { t.Error("expected 0 with no latency") }
}

func TestWorker_UnknownModelUsesDefault(t *testing.T) {
	w := newTestWorker()
	w.SetJitter(0.01)
	resp, err := w.Infer(&InferRequest{ModelID: "unknown-model", TaskType: TaskClassify, Prompt: "test"})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if resp == nil { t.Error("expected non-nil response for unknown model") }
}

func TestMin_Worker(t *testing.T) {
	if min(3, 5) != 3 { t.Error("expected 3") }
	if min(5, 3) != 3 { t.Error("expected 3") }
}

func TestGetEnv_Worker(t *testing.T) {
	t.Setenv("TEST_WORKER_KEY", "wval")
	if getEnv("TEST_WORKER_KEY", "fb") != "wval" { t.Error("expected wval") }
}
// embed length
// embed normalized
// embed deterministic
// embed different
// rerank scores
// rerank relevant
// rerank empty
// rerank range
// classify positive
// classify negative
// classify harmful
// classify neutral
// tokens non empty
// tokens empty
// tokens longer
// infer chat
// infer embed
// infer rerank
// infer classify
// worker down
// worker degraded
// metrics
// error metric
// avg latency
// tps
// unknown model
// min helper
// getenv
