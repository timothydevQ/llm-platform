package quota_test

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/timothydevQ/llm-platform/services/control-plane/internal/quota"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	f, _ := os.CreateTemp("", "quota-*.db")
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	db, _ := sql.Open("sqlite", f.Name())
	db.Exec(`
	CREATE TABLE quotas (
		tenant_id TEXT PRIMARY KEY, tokens_per_minute INTEGER DEFAULT 100000,
		tokens_per_day INTEGER DEFAULT 5000000, budget_usd_per_day REAL DEFAULT 50,
		max_context_tokens INTEGER DEFAULT 8192, updated_at TEXT DEFAULT (datetime('now'))
	);
	CREATE TABLE quota_usage (
		tenant_id TEXT, window_key TEXT, window_type TEXT,
		tokens_used INTEGER DEFAULT 0, cost_usd REAL DEFAULT 0,
		updated_at TEXT DEFAULT (datetime('now')),
		PRIMARY KEY (tenant_id, window_key, window_type)
	);
	CREATE INDEX IF NOT EXISTS idx_quota_usage ON quota_usage(tenant_id, window_type, window_key DESC);
	`)
	return db
}

// ─── Config tests ─────────────────────────────────────────────────────────────

func TestDefaultConfig(t *testing.T) {
	cfg := quota.DefaultConfig("t1")
	if cfg.TenantID != "t1"           { t.Error("wrong tenant") }
	if cfg.TokensPerMinute <= 0        { t.Error("expected positive tpm") }
	if cfg.TokensPerDay <= 0           { t.Error("expected positive tpd") }
	if cfg.BudgetUSDPerDay <= 0        { t.Error("expected positive budget") }
	if cfg.MaxContextTokens <= 0       { t.Error("expected positive context limit") }
}

// ─── Enforcer tests ───────────────────────────────────────────────────────────

func TestEnforcer_AllowsUnderLimits(t *testing.T) {
	e := quota.NewEnforcer(openDB(t))
	e.UpsertConfig(&quota.Config{
		TenantID: "t1", TokensPerMinute: 100000, TokensPerDay: 5000000,
		BudgetUSDPerDay: 50, MaxContextTokens: 8192,
	})
	result := e.Check("t1", 100, 512)
	if !result.Allowed { t.Errorf("expected allowed, got: %s", result.Reason) }
}

func TestEnforcer_DeniesExcessiveContext(t *testing.T) {
	e := quota.NewEnforcer(openDB(t))
	e.UpsertConfig(&quota.Config{
		TenantID: "t1", TokensPerMinute: 100000, TokensPerDay: 5000000,
		BudgetUSDPerDay: 50, MaxContextTokens: 512,
	})
	result := e.Check("t1", 100, 1000) // context > max
	if result.Allowed { t.Error("expected denied for excess context") }
	if result.Reason != quota.DeniedContext.Reason { t.Errorf("wrong reason: %s", result.Reason) }
}

func TestEnforcer_DeniesExceedDayTokens(t *testing.T) {
	e := quota.NewEnforcer(openDB(t))
	e.UpsertConfig(&quota.Config{
		TenantID: "t1", TokensPerMinute: 1000000, TokensPerDay: 1000,
		BudgetUSDPerDay: 50, MaxContextTokens: 8192,
	})
	// Record usage that maxes out the day limit
	e.Record("t1", 1000, 0.01)
	result := e.Check("t1", 1, 100)
	if result.Allowed { t.Error("expected denied after day quota consumed") }
}

func TestEnforcer_DeniesExceedBudget(t *testing.T) {
	e := quota.NewEnforcer(openDB(t))
	e.UpsertConfig(&quota.Config{
		TenantID: "t1", TokensPerMinute: 1000000, TokensPerDay: 5000000,
		BudgetUSDPerDay: 0.00001, MaxContextTokens: 8192,
	})
	e.Record("t1", 100, 0.0001) // exceeds budget
	result := e.Check("t1", 1, 100)
	if result.Allowed { t.Error("expected denied after budget consumed") }
}

func TestEnforcer_AllowsUnknownTenantWithDefaults(t *testing.T) {
	e := quota.NewEnforcer(openDB(t))
	// No config in DB — should use defaults
	result := e.Check("unknown-tenant", 100, 512)
	if !result.Allowed { t.Errorf("expected default allow, got: %s", result.Reason) }
}

func TestEnforcer_RecordUpdatesUsage(t *testing.T) {
	e := quota.NewEnforcer(openDB(t))
	e.Record("t1", 500, 0.05)
	u := e.GetUsage("t1")
	if u.UsedToday < 500 { t.Errorf("expected at least 500 tokens used, got %d", u.UsedToday) }
}

func TestEnforcer_UpsertConfig_Stores(t *testing.T) {
	e := quota.NewEnforcer(openDB(t))
	cfg := &quota.Config{
		TenantID: "t2", TokensPerMinute: 50000, TokensPerDay: 2000000,
		BudgetUSDPerDay: 20, MaxContextTokens: 4096,
	}
	if err := e.UpsertConfig(cfg); err != nil { t.Fatalf("UpsertConfig: %v", err) }
	u := e.GetUsage("t2")
	if u.Config == nil { t.Fatal("expected config") }
	if u.Config.TokensPerMinute != 50000 { t.Errorf("wrong tpm: %d", u.Config.TokensPerMinute) }
}

func TestEnforcer_GetUsage_RemainingCalculation(t *testing.T) {
	e := quota.NewEnforcer(openDB(t))
	e.UpsertConfig(&quota.Config{
		TenantID: "t1", TokensPerMinute: 100000, TokensPerDay: 10000,
		BudgetUSDPerDay: 1.0, MaxContextTokens: 8192,
	})
	e.Record("t1", 3000, 0.3)
	u := e.GetUsage("t1")
	if u.TokensRemainingDay() > 10000 { t.Error("remaining should be less than limit") }
	if u.BudgetRemainingDay() > 1.0   { t.Error("budget remaining should be less than limit") }
}

// ─── Usage struct tests ───────────────────────────────────────────────────────

func TestUsage_TokensRemainingDay(t *testing.T) {
	u := &quota.Usage{Config: &quota.Config{TokensPerDay: 1000}, UsedToday: 600}
	if u.TokensRemainingDay() != 400 { t.Errorf("expected 400, got %d", u.TokensRemainingDay()) }
}

func TestUsage_TokensRemainingDay_Exceeded(t *testing.T) {
	u := &quota.Usage{Config: &quota.Config{TokensPerDay: 100}, UsedToday: 200}
	if u.TokensRemainingDay() != 0 { t.Errorf("expected 0 when exceeded, got %d", u.TokensRemainingDay()) }
}

func TestUsage_BudgetRemainingDay(t *testing.T) {
	u := &quota.Usage{Config: &quota.Config{BudgetUSDPerDay: 10.0}, SpentToday: 3.5}
	if u.BudgetRemainingDay() != 6.5 { t.Errorf("expected 6.5, got %f", u.BudgetRemainingDay()) }
}
// tw_6059_12636
