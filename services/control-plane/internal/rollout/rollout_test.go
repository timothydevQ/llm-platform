package rollout_test

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/timothydevQ/llm-platform/services/control-plane/internal/rollout"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	f, _ := os.CreateTemp("", "rollout-*.db")
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	db, _ := sql.Open("sqlite", f.Name())
	db.Exec(`
	CREATE TABLE rollouts (
		rollout_id TEXT PRIMARY KEY, base_model_id TEXT, canary_model_id TEXT,
		canary_pct REAL DEFAULT 0, auto_rollback INTEGER DEFAULT 1,
		max_p99_ratio REAL DEFAULT 2.0, max_error_rate REAL DEFAULT 0.05,
		enabled INTEGER DEFAULT 0, rolled_back_at TEXT, rollback_reason TEXT,
		updated_at TEXT DEFAULT (datetime('now'))
	);
	CREATE TABLE rollout_metrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT, rollout_id TEXT, model_id TEXT,
		window_start TEXT, request_count INTEGER DEFAULT 0, error_count INTEGER DEFAULT 0,
		p99_latency_ms REAL DEFAULT 0, avg_tokens REAL DEFAULT 0, total_cost_usd REAL DEFAULT 0,
		recorded_at TEXT DEFAULT (datetime('now'))
	);
	`)
	return db
}

func TestManager_Upsert(t *testing.T) {
	m := rollout.New(openDB(t))
	err := m.Upsert(&rollout.Config{
		RolloutID:     "r1",
		BaseModelID:   "gpt-large",
		CanaryModelID: "gpt-medium",
		CanaryPct:     0.1,
		AutoRollback:  true,
		MaxP99Ratio:   2.0,
		MaxErrorRate:  0.05,
		Enabled:       true,
	})
	if err != nil { t.Fatalf("Upsert: %v", err) }
}

func TestManager_Upsert_RequiresRolloutID(t *testing.T) {
	m := rollout.New(openDB(t))
	err := m.Upsert(&rollout.Config{BaseModelID: "gpt-large", CanaryModelID: "gpt-medium"})
	if err == nil { t.Error("expected error for missing rollout_id") }
}

func TestManager_Upsert_RequiresBaseModel(t *testing.T) {
	m := rollout.New(openDB(t))
	err := m.Upsert(&rollout.Config{RolloutID: "r1", CanaryModelID: "gpt-medium"})
	if err == nil { t.Error("expected error for missing base_model_id") }
}

func TestManager_Upsert_InvalidCanaryPct(t *testing.T) {
	m := rollout.New(openDB(t))
	err := m.Upsert(&rollout.Config{RolloutID: "r1", BaseModelID: "a", CanaryModelID: "b", CanaryPct: 1.5})
	if err == nil { t.Error("expected error for canary_pct > 1") }
}

func TestManager_Upsert_NegativePct(t *testing.T) {
	m := rollout.New(openDB(t))
	err := m.Upsert(&rollout.Config{RolloutID: "r1", BaseModelID: "a", CanaryModelID: "b", CanaryPct: -0.1})
	if err == nil { t.Error("expected error for negative canary_pct") }
}

func TestManager_List_Empty(t *testing.T) {
	m := rollout.New(openDB(t))
	cfgs, err := m.List()
	if err != nil { t.Fatalf("List: %v", err) }
	if len(cfgs) != 0 { t.Errorf("expected 0, got %d", len(cfgs)) }
}

func TestManager_List_AfterUpsert(t *testing.T) {
	m := rollout.New(openDB(t))
	m.Upsert(&rollout.Config{RolloutID: "r1", BaseModelID: "a", CanaryModelID: "b", Enabled: true})
	cfgs, _ := m.List()
	if len(cfgs) != 1 { t.Errorf("expected 1, got %d", len(cfgs)) }
	if !cfgs[0].Enabled { t.Error("expected enabled") }
}

func TestManager_Rollback(t *testing.T) {
	m := rollout.New(openDB(t))
	m.Upsert(&rollout.Config{RolloutID: "r1", BaseModelID: "a", CanaryModelID: "b", Enabled: true})
	if err := m.Rollback("r1", "p99 exceeded"); err != nil { t.Fatalf("Rollback: %v", err) }
	cfgs, _ := m.List()
	if len(cfgs) != 1 { t.Fatalf("expected 1 config") }
	if cfgs[0].Enabled { t.Error("expected disabled after rollback") }
}

func TestManager_RolloutWeights_NoRollouts(t *testing.T) {
	m := rollout.New(openDB(t))
	weights := m.RolloutWeights()
	if len(weights) != 0 { t.Error("expected empty weights with no rollouts") }
}

func TestManager_RolloutWeights_WithRollout(t *testing.T) {
	m := rollout.New(openDB(t))
	m.Upsert(&rollout.Config{
		RolloutID:     "r1",
		BaseModelID:   "gpt-large",
		CanaryModelID: "gpt-medium",
		CanaryPct:     0.1,
		Enabled:       true,
	})
	weights := m.RolloutWeights()
	if weights["gpt-large"] != 0.9  { t.Errorf("base weight: %f", weights["gpt-large"]) }
	if weights["gpt-medium"] != 0.1 { t.Errorf("canary weight: %f", weights["gpt-medium"]) }
}

func TestManager_RolloutWeights_DisabledRollout(t *testing.T) {
	m := rollout.New(openDB(t))
	m.Upsert(&rollout.Config{
		RolloutID: "r1", BaseModelID: "a", CanaryModelID: "b", CanaryPct: 0.1, Enabled: false,
	})
	weights := m.RolloutWeights()
	if len(weights) != 0 { t.Error("disabled rollout should not contribute weights") }
}

func TestWindowMetrics_ErrorRate(t *testing.T) {
	wm := &rollout.WindowMetrics{Requests: 100, Errors: 5}
	if wm.ErrorRate() != 0.05 { t.Errorf("expected 0.05, got %f", wm.ErrorRate()) }
}

func TestWindowMetrics_ErrorRate_ZeroRequests(t *testing.T) {
	wm := &rollout.WindowMetrics{Requests: 0, Errors: 0}
	if wm.ErrorRate() != 0 { t.Error("expected 0 with no requests") }
}
// tw_6059_27152
