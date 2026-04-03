// Package quota enforces per-tenant token and spend budgets using
// sliding time windows stored in SQLite.
//
// Two windows are tracked per tenant:
//   - minute: tokens_per_minute limit — prevents burst abuse
//   - day:    tokens_per_day + budget_usd_per_day — cost control
package quota

import (
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ─── Config ───────────────────────────────────────────────────────────────────

type Config struct {
	TenantID         string  `json:"tenant_id"`
	TokensPerMinute  int64   `json:"tokens_per_minute"`
	TokensPerDay     int64   `json:"tokens_per_day"`
	BudgetUSDPerDay  float64 `json:"budget_usd_per_day"`
	MaxContextTokens int32   `json:"max_context_tokens"`
}

func DefaultConfig(tenantID string) *Config {
	return &Config{
		TenantID:         tenantID,
		TokensPerMinute:  100_000,
		TokensPerDay:     5_000_000,
		BudgetUSDPerDay:  50.0,
		MaxContextTokens: 8192,
	}
}

// ─── Usage snapshot ───────────────────────────────────────────────────────────

type Usage struct {
	Config      *Config `json:"config"`
	UsedToday   int64   `json:"used_today"`
	SpentToday  float64 `json:"spent_today"`
	UsedMinute  int64   `json:"used_minute"`
	WindowKey   string  `json:"window_key"`
}

func (u *Usage) TokensRemainingDay() int64 {
	rem := u.Config.TokensPerDay - u.UsedToday
	if rem < 0 { return 0 }
	return rem
}

func (u *Usage) BudgetRemainingDay() float64 {
	rem := u.Config.BudgetUSDPerDay - u.SpentToday
	if rem < 0 { return 0 }
	return rem
}

// ─── Enforcer ─────────────────────────────────────────────────────────────────

// CheckResult is the outcome of a quota check.
type CheckResult struct {
	Allowed bool
	Reason  string
}

var (
	Allowed           = CheckResult{Allowed: true}
	DeniedTokenDay    = CheckResult{Allowed: false, Reason: "daily token quota exceeded"}
	DeniedBudgetDay   = CheckResult{Allowed: false, Reason: "daily spend budget exceeded"}
	DeniedTokenMinute = CheckResult{Allowed: false, Reason: "per-minute token quota exceeded"}
	DeniedContext     = CheckResult{Allowed: false, Reason: "context length exceeds tenant maximum"}
)

// Enforcer checks and records quota usage.
type Enforcer struct {
	db     *sql.DB
	mu     sync.RWMutex
	cache  map[string]*configEntry  // in-memory config cache

	// Fast counters for minute window (reset every minute without DB round-trip)
	minuteMu      sync.Mutex
	minuteCounts  map[string]*minuteCounter
}

type configEntry struct {
	cfg       *Config
	fetchedAt time.Time
}

type minuteCounter struct {
	used   int64
	window string // "2006-01-02T15:04"
}

func NewEnforcer(db *sql.DB) *Enforcer {
	e := &Enforcer{
		db:           db,
		cache:        make(map[string]*configEntry),
		minuteCounts: make(map[string]*minuteCounter),
	}
	go e.flushMinuteLoop()
	return e
}

// Check verifies whether a request with estimatedTokens would exceed any quota.
// contextTokens is the size of the request context (for max_context_tokens check).
func (e *Enforcer) Check(tenantID string, estimatedTokens int64, contextTokens int32) CheckResult {
	cfg := e.loadConfig(tenantID)
	if cfg == nil { cfg = DefaultConfig(tenantID) }

	// Context length check (synchronous, no DB)
	if contextTokens > cfg.MaxContextTokens {
		return DeniedContext
	}

	// Minute window check (in-memory)
	if !e.checkMinute(tenantID, estimatedTokens, cfg) {
		return DeniedTokenMinute
	}

	// Day window check (DB)
	usage := e.dayUsage(tenantID)
	if usage.UsedToday+estimatedTokens > cfg.TokensPerDay {
		return DeniedTokenDay
	}
	// Cost check — estimate cost = tokens * cheapest model rate as floor
	estimatedCost := float64(estimatedTokens) / 1000.0 * 0.0001
	if usage.SpentToday+estimatedCost > cfg.BudgetUSDPerDay {
		return DeniedBudgetDay
	}
	return Allowed
}

// Record increments usage counters after a successful inference.
func (e *Enforcer) Record(tenantID string, tokensUsed int64, costUSD float64) {
	now := time.Now()
	dayKey := now.Format("2006-01-02")
	minuteKey := now.Format("2006-01-02T15:04")

	e.db.Exec(`
		INSERT INTO quota_usage (tenant_id, window_key, window_type, tokens_used, cost_usd, updated_at)
		VALUES (?,?,?,?,?,datetime('now'))
		ON CONFLICT(tenant_id, window_key, window_type) DO UPDATE SET
			tokens_used = tokens_used + excluded.tokens_used,
			cost_usd    = cost_usd    + excluded.cost_usd,
			updated_at  = datetime('now')`,
		tenantID, dayKey, "day", tokensUsed, costUSD)

	e.db.Exec(`
		INSERT INTO quota_usage (tenant_id, window_key, window_type, tokens_used, updated_at)
		VALUES (?,?,?,?,datetime('now'))
		ON CONFLICT(tenant_id, window_key, window_type) DO UPDATE SET
			tokens_used = tokens_used + excluded.tokens_used,
			updated_at  = datetime('now')`,
		tenantID, minuteKey, "minute", tokensUsed)

	// Update in-memory minute counter
	e.minuteMu.Lock()
	if mc, ok := e.minuteCounts[tenantID]; ok && mc.window == minuteKey {
		atomic.AddInt64(&mc.used, tokensUsed)
	}
	e.minuteMu.Unlock()
}

// GetUsage returns current usage for a tenant.
func (e *Enforcer) GetUsage(tenantID string) *Usage {
	cfg := e.loadConfig(tenantID)
	if cfg == nil { cfg = DefaultConfig(tenantID) }
	u := e.dayUsage(tenantID)
	u.Config = cfg
	return u
}

// UpsertConfig stores or updates a tenant's quota config.
func (e *Enforcer) UpsertConfig(cfg *Config) error {
	_, err := e.db.Exec(`
		INSERT OR REPLACE INTO quotas (tenant_id, tokens_per_minute, tokens_per_day, budget_usd_per_day, max_context_tokens, updated_at)
		VALUES (?,?,?,?,?,datetime('now'))`,
		cfg.TenantID, cfg.TokensPerMinute, cfg.TokensPerDay, cfg.BudgetUSDPerDay, cfg.MaxContextTokens)
	if err != nil { return err }
	e.mu.Lock()
	e.cache[cfg.TenantID] = &configEntry{cfg: cfg, fetchedAt: time.Now()}
	e.mu.Unlock()
	return nil
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (e *Enforcer) loadConfig(tenantID string) *Config {
	e.mu.RLock()
	entry, ok := e.cache[tenantID]
	e.mu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < 60*time.Second {
		return entry.cfg
	}

	row := e.db.QueryRow(`
		SELECT tokens_per_minute, tokens_per_day, budget_usd_per_day, max_context_tokens
		FROM quotas WHERE tenant_id=?`, tenantID)
	cfg := &Config{TenantID: tenantID}
	err := row.Scan(&cfg.TokensPerMinute, &cfg.TokensPerDay, &cfg.BudgetUSDPerDay, &cfg.MaxContextTokens)
	if err != nil {
		return nil // caller uses DefaultConfig
	}
	e.mu.Lock()
	e.cache[tenantID] = &configEntry{cfg: cfg, fetchedAt: time.Now()}
	e.mu.Unlock()
	return cfg
}

func (e *Enforcer) dayUsage(tenantID string) *Usage {
	dayKey := time.Now().Format("2006-01-02")
	row := e.db.QueryRow(`
		SELECT COALESCE(SUM(tokens_used),0), COALESCE(SUM(cost_usd),0)
		FROM quota_usage WHERE tenant_id=? AND window_type='day' AND window_key=?`,
		tenantID, dayKey)
	u := &Usage{WindowKey: dayKey}
	row.Scan(&u.UsedToday, &u.SpentToday)
	return u
}

func (e *Enforcer) checkMinute(tenantID string, estimatedTokens int64, cfg *Config) bool {
	now := time.Now()
	key := now.Format("2006-01-02T15:04")

	e.minuteMu.Lock()
	defer e.minuteMu.Unlock()

	mc, ok := e.minuteCounts[tenantID]
	if !ok || mc.window != key {
		e.minuteCounts[tenantID] = &minuteCounter{used: 0, window: key}
		mc = e.minuteCounts[tenantID]
	}
	if mc.used+estimatedTokens > cfg.TokensPerMinute {
		return false
	}
	return true
}

func (e *Enforcer) flushMinuteLoop() {
	for range time.NewTicker(time.Minute).C {
		key := time.Now().Format("2006-01-02T15:04")
		e.minuteMu.Lock()
		for tid, mc := range e.minuteCounts {
			if mc.window != key {
				delete(e.minuteCounts, tid)
			}
		}
		e.minuteMu.Unlock()
	}
}

var _ = fmt.Sprintf // keep import
// cq_278
// cq_279
// cq_280
// cq_281
// cq_282
// cq_283
// cq_284
// cq_285
// cq_286
// cq_287
// cq_288
// cq_289
// cq_290
