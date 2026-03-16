package scoring_test

import (
	"testing"

	"github.com/timothydevQ/llm-platform/services/router/internal/scoring"
)

// ─── Fixtures ─────────────────────────────────────────────────────────────────

func makeModels() []*scoring.ModelRecord {
	return []*scoring.ModelRecord{
		{ModelID: "gpt-small",  Version: "v1", Tier: "small",  Tasks: []string{"chat","summarize","classify"},
			CostPer1k: 0.0002, AvgLatencyMs: 200,  MaxTokens: 4096,  ExecutorAddr: "exec:50051", Enabled: true},
		{ModelID: "gpt-medium", Version: "v1", Tier: "medium", Tasks: []string{"chat","summarize"},
			CostPer1k: 0.002,  AvgLatencyMs: 500,  MaxTokens: 8192,  ExecutorAddr: "exec:50051", Enabled: true},
		{ModelID: "gpt-large",  Version: "v1", Tier: "large",  Tasks: []string{"chat","summarize"},
			CostPer1k: 0.02,   AvgLatencyMs: 1200, MaxTokens: 32768, ExecutorAddr: "exec:50051", Enabled: true},
		{ModelID: "embed-v2",   Version: "v1", Tier: "small",  Tasks: []string{"embed"},
			CostPer1k: 0.0001, AvgLatencyMs: 50,   MaxTokens: 8192,  ExecutorAddr: "exec:50051", Enabled: true},
		{ModelID: "rerank-v1",  Version: "v1", Tier: "small",  Tasks: []string{"rerank"},
			CostPer1k: 0.0002, AvgLatencyMs: 100,  MaxTokens: 4096,  ExecutorAddr: "exec:50051", Enabled: true},
	}
}

func newScorer() *scoring.Scorer {
	return scoring.NewScorer(scoring.NewHealthTracker(), nil)
}

// ─── Scorer tests ─────────────────────────────────────────────────────────────

func TestScorer_FiltersByTask(t *testing.T) {
	s := newScorer()
	req := &scoring.ScoringRequest{Task: "embed"}
	results := s.Score(req, makeModels())
	for _, r := range results {
		if r.ModelID != "embed-v2" {
			t.Errorf("embed task: unexpected model %s", r.ModelID)
		}
	}
}

func TestScorer_EmbedReturnsEmbedModel(t *testing.T) {
	s := newScorer()
	results := s.Score(&scoring.ScoringRequest{Task: "embed"}, makeModels())
	if len(results) == 0 { t.Fatal("expected at least one result") }
	if results[0].ModelID != "embed-v2" {
		t.Errorf("expected embed-v2 first, got %s", results[0].ModelID)
	}
}

func TestScorer_LowBudgetPrefersSmall(t *testing.T) {
	s := newScorer()
	req := &scoring.ScoringRequest{Task: "chat", CostBudget: "low", TenantMode: scoring.ModeCostOptimized}
	results := s.Score(req, makeModels())
	if len(results) == 0 { t.Fatal("expected results") }
	if results[0].ModelID != "gpt-small" {
		t.Errorf("expected gpt-small first for low budget, got %s", results[0].ModelID)
	}
}

func TestScorer_HighBudgetPrefersLarge(t *testing.T) {
	s := newScorer()
	req := &scoring.ScoringRequest{Task: "chat", CostBudget: "high", TenantMode: scoring.ModeCostOptimized}
	results := s.Score(req, makeModels())
	if len(results) == 0 { t.Fatal("expected results") }
	if results[0].ModelID != "gpt-large" {
		t.Errorf("expected gpt-large first for high budget, got %s", results[0].ModelID)
	}
}

func TestScorer_LatencyTargetFiltersSlowModels(t *testing.T) {
	s := newScorer()
	req := &scoring.ScoringRequest{Task: "chat", LatencyTargetMs: 300, TenantMode: scoring.ModeLatencyOptimized}
	results := s.Score(req, makeModels())
	for _, r := range results {
		if r.ModelID == "gpt-large" && r.LatencyScore > 0 {
			t.Error("gpt-large should score 0 for latency when target=300ms")
		}
	}
}

func TestScorer_DisabledModelFiltered(t *testing.T) {
	s := newScorer()
	models := makeModels()
	for _, m := range models {
		m.Enabled = false
	}
	results := s.Score(&scoring.ScoringRequest{Task: "chat"}, models)
	if len(results) != 0 {
		t.Errorf("expected 0 results when all disabled, got %d", len(results))
	}
}

func TestScorer_AllowedModelsFilter(t *testing.T) {
	s := newScorer()
	req := &scoring.ScoringRequest{
		Task:          "chat",
		AllowedModels: map[string]bool{"gpt-small": true},
	}
	results := s.Score(req, makeModels())
	if len(results) != 1 { t.Fatalf("expected 1, got %d", len(results)) }
	if results[0].ModelID != "gpt-small" {
		t.Errorf("expected gpt-small, got %s", results[0].ModelID)
	}
}

func TestScorer_RolloutWeightAffectsScore(t *testing.T) {
	s := newScorer()
	req := &scoring.ScoringRequest{
		Task:           "chat",
		RolloutWeights: map[string]float64{"gpt-large": 0.0},
	}
	results := s.Score(req, makeModels())
	for _, r := range results {
		if r.ModelID == "gpt-large" && r.TotalScore > 0 {
			t.Error("gpt-large should have 0 score with 0.0 rollout weight")
		}
	}
}

func TestScorer_SortedDescending(t *testing.T) {
	s := newScorer()
	results := s.Score(&scoring.ScoringRequest{Task: "chat"}, makeModels())
	for i := 1; i < len(results); i++ {
		if results[i].TotalScore > results[i-1].TotalScore {
			t.Errorf("results not sorted: [%d]=%f > [%d]=%f", i, results[i].TotalScore, i-1, results[i-1].TotalScore)
		}
	}
}

func TestScorer_EmptyAllowedMeansAll(t *testing.T) {
	s := newScorer()
	req := &scoring.ScoringRequest{Task: "chat", AllowedModels: map[string]bool{}}
	results := s.Score(req, makeModels())
	if len(results) == 0 {
		t.Error("empty allowed models map should allow all")
	}
}

// ─── Health tracker tests ─────────────────────────────────────────────────────

func TestHealthTracker_InitiallyZeroErrors(t *testing.T) {
	h := scoring.NewHealthTracker()
	if h.ErrorRate("any-model") != 0 {
		t.Error("expected 0 error rate initially")
	}
}

func TestHealthTracker_RecordFailuresIncreasesErrorRate(t *testing.T) {
	h := scoring.NewHealthTracker()
	for i := 0; i < 5; i++ { h.RecordSuccess("m1", 100) }
	for i := 0; i < 5; i++ { h.RecordFailure("m1") }
	rate := h.ErrorRate("m1")
	if rate <= 0 {
		t.Error("expected positive error rate after failures")
	}
}

func TestHealthTracker_HighErrorRateGivesLowScore(t *testing.T) {
	h := scoring.NewHealthTracker()
	for i := 0; i < 50; i++ { h.RecordFailure("m1") }
	rate := h.ErrorRate("m1")
	if rate < 0.4 {
		t.Errorf("expected high error rate, got %f", rate)
	}
}

func TestHealthTracker_P99LatencyTracked(t *testing.T) {
	h := scoring.NewHealthTracker()
	for i := 0; i < 100; i++ {
		h.RecordSuccess("m1", float64(i)*10)
	}
	p99 := h.P99Latency("m1")
	if p99 <= 0 {
		t.Error("expected positive p99 after recording successes")
	}
}

func TestHealthTracker_UnknownModelZeroP99(t *testing.T) {
	h := scoring.NewHealthTracker()
	if h.P99Latency("unknown") != 0 {
		t.Error("expected 0 p99 for unknown model")
	}
}

// ─── ModelRecord tests ────────────────────────────────────────────────────────

func TestModelRecord_SupportsTask(t *testing.T) {
	m := &scoring.ModelRecord{Tasks: []string{"chat", "embed"}}
	if !m.SupportsTask("chat")  { t.Error("expected chat supported") }
	if !m.SupportsTask("embed") { t.Error("expected embed supported") }
	if m.SupportsTask("rerank") { t.Error("expected rerank not supported") }
}

func TestScorer_NilAllowedMeansAll(t *testing.T) {
	s := newScorer()
	req := &scoring.ScoringRequest{Task: "chat", AllowedModels: nil}
	results := s.Score(req, makeModels())
	if len(results) == 0 { t.Error("nil allowed models should allow all") }
}
// tw_6059_799
// tw_6059_32096
// tw_6059_1211
// tw_6059_30334
// tw_6059_12972
// tw_6059_4387
