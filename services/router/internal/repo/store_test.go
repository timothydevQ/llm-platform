package repo_test

import (
	"os"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"github.com/timothydevQ/llm-platform/services/router/internal/repo"
)

func openTestStore(t *testing.T) *repo.Store {
	t.Helper()
	f, err := os.CreateTemp("", "router-repo-*.db")
	if err != nil { t.Fatalf("temp file: %v", err) }
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	s, err := repo.Open(f.Name())
	if err != nil { t.Fatalf("Open: %v", err) }
	return s
}

func TestStore_Ping(t *testing.T) {
	s := openTestStore(t)
	if err := s.Ping(); err != nil { t.Fatalf("Ping: %v", err) }
}

func TestStore_Seed_Idempotent(t *testing.T) {
	s := openTestStore(t)
	if err := s.Seed("exec:50051"); err != nil { t.Fatalf("Seed: %v", err) }
	if err := s.Seed("exec:50051"); err != nil { t.Fatalf("Seed2: %v", err) }
}

func TestStore_LoadModels_AfterSeed(t *testing.T) {
	s := openTestStore(t)
	s.Seed("exec:50051")
	models, err := s.LoadModels()
	if err != nil { t.Fatalf("LoadModels: %v", err) }
	if len(models) == 0 { t.Error("expected models after seed") }
}

func TestStore_LoadModels_HasEmbedModel(t *testing.T) {
	s := openTestStore(t)
	s.Seed("exec:50051")
	models, _ := s.LoadModels()
	found := false
	for _, m := range models {
		if m.ModelID == "embed-v2" { found = true }
	}
	if !found { t.Error("embed-v2 not found") }
}

func TestStore_LoadModels_TasksPopulated(t *testing.T) {
	s := openTestStore(t)
	s.Seed("exec:50051")
	models, _ := s.LoadModels()
	for _, m := range models {
		if len(m.Tasks) == 0 { t.Errorf("model %s has no tasks", m.ModelID) }
	}
}

func TestStore_UpsertRollout_AndLoad(t *testing.T) {
	s := openTestStore(t)
	rc := &repo.RolloutConfig{
		RolloutID:     "r1",
		BaseModelID:   "gpt-large",
		CanaryModelID: "gpt-medium",
		CanaryPct:     0.1,
		AutoRollback:  true,
		MaxP99Ratio:   2.0,
		MaxErrorRate:  0.05,
		Enabled:       true,
	}
	if err := s.UpsertRollout(rc); err != nil { t.Fatalf("UpsertRollout: %v", err) }
	rollouts, err := s.LoadRollouts()
	if err != nil { t.Fatalf("LoadRollouts: %v", err) }
	if len(rollouts) != 1 { t.Fatalf("expected 1 rollout, got %d", len(rollouts)) }
	if rollouts[0].CanaryPct != 0.1 { t.Errorf("wrong pct: %f", rollouts[0].CanaryPct) }
}

func TestStore_RollbackRollout(t *testing.T) {
	s := openTestStore(t)
	s.UpsertRollout(&repo.RolloutConfig{RolloutID: "r2", BaseModelID: "gpt-large", CanaryModelID: "gpt-medium", Enabled: true})
	if err := s.RollbackRollout("r2", "p99 exceeded threshold"); err != nil { t.Fatalf("Rollback: %v", err) }
	rollouts, _ := s.LoadRollouts()
	if len(rollouts) == 0 { t.Fatal("expected rollout record") }
	if rollouts[0].Enabled { t.Error("expected disabled after rollback") }
}

func TestStore_LoadTenantPolicy_Default(t *testing.T) {
	s := openTestStore(t)
	p, err := s.LoadTenantPolicy("unknown-tenant")
	if err != nil { t.Fatalf("LoadTenantPolicy: %v", err) }
	if p == nil { t.Fatal("expected default policy") }
	if p.TenantID != "unknown-tenant" { t.Errorf("wrong tenant: %s", p.TenantID) }
}

func TestStore_LoadTenantPolicy_AfterSeed(t *testing.T) {
	s := openTestStore(t)
	s.Seed("exec:50051")
	p, err := s.LoadTenantPolicy("tenant-premium")
	if err != nil { t.Fatalf("LoadTenantPolicy: %v", err) }
	if p.RoutingMode != "latency_optimized" {
		t.Errorf("expected latency_optimized, got %s", p.RoutingMode)
	}
}

func TestStore_LogRequest_And_WindowStats(t *testing.T) {
	s := openTestStore(t)
	s.LogRequest(&repo.LogEntry{
		RequestID: "req-1", TaskType: "chat", ModelID: "gpt-small",
		TenantID: "t1", TokensInput: 10, TokensOutput: 20,
		LatencyMs: 200, CostUSD: 0.0001,
	})
	stats := s.WindowStats(time.Hour)
	if stats["requests"].(int64) != 1 {
		t.Errorf("expected 1 request, got %v", stats["requests"])
	}
}

func TestStore_WindowStats_Empty(t *testing.T) {
	s := openTestStore(t)
	stats := s.WindowStats(time.Hour)
	if stats["requests"].(int64) != 0 {
		t.Error("expected 0 on empty DB")
	}
}
// tw_6059_28462
// tw_6059_22042
// tw_6059_16013
// tw_6059_15185
// tw_6059_19085
