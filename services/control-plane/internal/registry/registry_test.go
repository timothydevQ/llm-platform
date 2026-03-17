package registry_test

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/timothydevQ/llm-platform/services/control-plane/internal/registry"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	f, _ := os.CreateTemp("", "registry-*.db")
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	db, _ := sql.Open("sqlite", f.Name())
	db.Exec(`
	CREATE TABLE models (
		model_id TEXT PRIMARY KEY, version TEXT DEFAULT 'v1', name TEXT,
		tier TEXT, cost_per_1k REAL DEFAULT 0.001, avg_latency_ms INTEGER DEFAULT 500,
		max_tokens INTEGER DEFAULT 4096, executor_addr TEXT DEFAULT '',
		enabled INTEGER DEFAULT 1, labels TEXT DEFAULT '{}',
		created_at TEXT DEFAULT (datetime('now')), updated_at TEXT DEFAULT (datetime('now'))
	);
	CREATE TABLE model_capabilities (
		model_id TEXT, task_type TEXT, PRIMARY KEY(model_id, task_type)
	);
	`)
	return db
}

func TestRegistry_Register(t *testing.T) {
	r := registry.New(openDB(t))
	m := &registry.Model{
		ModelID:      "gpt-test",
		Name:         "GPT Test",
		Tier:         "small",
		Capabilities: []string{"chat", "summarize"},
		Enabled:      true,
		ExecutorAddr: "exec:50051",
	}
	_, err := r.Register(m)
	if err != nil { t.Fatalf("Register: %v", err) }
}

func TestRegistry_Register_RequiresModelID(t *testing.T) {
	r := registry.New(openDB(t))
	_, err := r.Register(&registry.Model{Name: "bad", Tier: "small"})
	if err == nil { t.Error("expected error for missing model_id") }
}

func TestRegistry_Register_RequiresTier(t *testing.T) {
	r := registry.New(openDB(t))
	_, err := r.Register(&registry.Model{ModelID: "m1", Name: "m1"})
	if err == nil { t.Error("expected error for missing tier") }
}

func TestRegistry_Register_Idempotent(t *testing.T) {
	r := registry.New(openDB(t))
	m := &registry.Model{ModelID: "m1", Name: "M1", Tier: "small", ExecutorAddr: "x"}
	r.Register(m)
	_, err := r.Register(m)
	if err != nil { t.Fatalf("second Register: %v", err) }
}

func TestRegistry_List_Empty(t *testing.T) {
	r := registry.New(openDB(t))
	models, err := r.List(false)
	if err != nil { t.Fatalf("List: %v", err) }
	if len(models) != 0 { t.Errorf("expected empty, got %d", len(models)) }
}

func TestRegistry_List_AfterRegister(t *testing.T) {
	r := registry.New(openDB(t))
	r.Register(&registry.Model{ModelID: "m1", Name: "M1", Tier: "small", Enabled: true})
	r.Register(&registry.Model{ModelID: "m2", Name: "M2", Tier: "medium", Enabled: false})
	all, _ := r.List(false)
	if len(all) != 2 { t.Errorf("expected 2, got %d", len(all)) }
	enabled, _ := r.List(true)
	if len(enabled) != 1 { t.Errorf("expected 1 enabled, got %d", len(enabled)) }
}

func TestRegistry_Get_Existing(t *testing.T) {
	r := registry.New(openDB(t))
	r.Register(&registry.Model{ModelID: "m1", Name: "M1", Tier: "small", Capabilities: []string{"chat"}})
	m, err := r.Get("m1")
	if err != nil { t.Fatalf("Get: %v", err) }
	if m == nil { t.Fatal("expected non-nil model") }
	if m.ModelID != "m1" { t.Errorf("wrong model ID: %s", m.ModelID) }
}

func TestRegistry_Get_NonExistent(t *testing.T) {
	r := registry.New(openDB(t))
	m, err := r.Get("nonexistent")
	if err != nil { t.Fatalf("Get: %v", err) }
	if m != nil { t.Error("expected nil for non-existent model") }
}

func TestRegistry_Get_CapabilitiesPopulated(t *testing.T) {
	r := registry.New(openDB(t))
	r.Register(&registry.Model{ModelID: "m1", Name: "M1", Tier: "small", Capabilities: []string{"chat", "embed"}})
	m, _ := r.Get("m1")
	if len(m.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(m.Capabilities))
	}
}

func TestRegistry_SetEnabled_Disable(t *testing.T) {
	r := registry.New(openDB(t))
	r.Register(&registry.Model{ModelID: "m1", Name: "M1", Tier: "small", Enabled: true})
	if err := r.SetEnabled("m1", false); err != nil { t.Fatalf("SetEnabled: %v", err) }
	m, _ := r.Get("m1")
	if m.Enabled { t.Error("expected disabled") }
}

func TestRegistry_SetEnabled_NotFound(t *testing.T) {
	r := registry.New(openDB(t))
	err := r.SetEnabled("nonexistent", false)
	if err == nil { t.Error("expected error for missing model") }
}

func TestRegistry_UpdateLabels(t *testing.T) {
	r := registry.New(openDB(t))
	r.Register(&registry.Model{ModelID: "m1", Name: "M1", Tier: "small", Labels: map[string]string{"env": "prod"}})
	m, _ := r.Get("m1")
	if m.Labels["env"] != "prod" { t.Error("expected env=prod label") }
}
// tw_6059_8352
// tw_6059_30211
// tw_6059_19802
