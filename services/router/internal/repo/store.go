// Package repo provides database access for the router's model registry,
// rollout configs, and tenant policies.
package repo

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/timothydevQ/llm-platform/services/router/internal/scoring"
	"github.com/timothydevQ/llm-platform/services/router/internal/policy"
)

// Store wraps the SQLite database used by the router.
type Store struct {
	db *sql.DB
	mu sync.RWMutex // guards cached state
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	return s, s.migrate()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
	PRAGMA journal_mode=WAL;
	PRAGMA foreign_keys=ON;

	CREATE TABLE IF NOT EXISTS models (
		model_id        TEXT PRIMARY KEY,
		version         TEXT NOT NULL DEFAULT 'v1',
		name            TEXT NOT NULL,
		tier            TEXT NOT NULL,
		cost_per_1k     REAL NOT NULL DEFAULT 0.001,
		avg_latency_ms  INTEGER NOT NULL DEFAULT 500,
		max_tokens      INTEGER NOT NULL DEFAULT 4096,
		executor_addr   TEXT NOT NULL,
		enabled         INTEGER NOT NULL DEFAULT 1
	);
	CREATE TABLE IF NOT EXISTS model_capabilities (
		model_id  TEXT NOT NULL,
		task_type TEXT NOT NULL,
		PRIMARY KEY (model_id, task_type)
	);
	CREATE TABLE IF NOT EXISTS rollouts (
		rollout_id      TEXT PRIMARY KEY,
		base_model_id   TEXT NOT NULL,
		canary_model_id TEXT NOT NULL,
		canary_pct      REAL NOT NULL DEFAULT 0.0,
		auto_rollback   INTEGER NOT NULL DEFAULT 1,
		max_p99_ratio   REAL NOT NULL DEFAULT 2.0,
		max_error_rate  REAL NOT NULL DEFAULT 0.05,
		enabled         INTEGER NOT NULL DEFAULT 0,
		rolled_back_at  TEXT,
		rollback_reason TEXT,
		updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
	);
	CREATE TABLE IF NOT EXISTS tenants (
		tenant_id       TEXT PRIMARY KEY,
		name            TEXT NOT NULL,
		routing_mode    TEXT NOT NULL DEFAULT 'balanced',
		allowed_models  TEXT NOT NULL DEFAULT '[]',
		rate_limit_rps  INTEGER NOT NULL DEFAULT 50,
		burst_limit     INTEGER NOT NULL DEFAULT 100,
		enabled         INTEGER NOT NULL DEFAULT 1
	);
	CREATE TABLE IF NOT EXISTS request_log (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		request_id    TEXT NOT NULL,
		trace_id      TEXT,
		tenant_id     TEXT,
		task_type     TEXT NOT NULL,
		model_id      TEXT,
		tokens_input  INTEGER,
		tokens_output INTEGER,
		latency_ms    REAL,
		queue_wait_ms REAL,
		cost_usd      REAL,
		cached        INTEGER NOT NULL DEFAULT 0,
		fallback_used INTEGER NOT NULL DEFAULT 0,
		is_canary     INTEGER NOT NULL DEFAULT 0,
		routing_mode  TEXT,
		created_at    TEXT NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_req_created ON request_log(created_at DESC);
	`)
	return err
}

// Seed inserts default models if the table is empty.
func (s *Store) Seed(executorAddr string) error {
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM models`).Scan(&n)
	if n > 0 {
		return nil
	}

	models := []struct{ id, name, tier, addr string; cost float64; lat, max int }{
		{"gpt-small",  "GPT Small",  "small",  executorAddr, 0.0002, 200,  4096},
		{"gpt-medium", "GPT Medium", "medium", executorAddr, 0.002,  500,  8192},
		{"gpt-large",  "GPT Large",  "large",  executorAddr, 0.02,   1200, 32768},
		{"embed-v2",   "Embed v2",   "small",  executorAddr, 0.0001, 50,   8192},
		{"rerank-v1",  "Rerank v1",  "small",  executorAddr, 0.0002, 100,  4096},
	}
	for _, m := range models {
		if _, err := s.db.Exec(
			`INSERT OR IGNORE INTO models (model_id, name, tier, cost_per_1k, avg_latency_ms, max_tokens, executor_addr) VALUES (?,?,?,?,?,?,?)`,
			m.id, m.name, m.tier, m.cost, m.lat, m.max, m.addr,
		); err != nil {
			return err
		}
	}

	caps := map[string][]string{
		"gpt-small":  {"chat", "summarize", "classify", "moderate"},
		"gpt-medium": {"chat", "summarize", "classify", "moderate"},
		"gpt-large":  {"chat", "summarize"},
		"embed-v2":   {"embed"},
		"rerank-v1":  {"rerank"},
	}
	for mid, tasks := range caps {
		for _, t := range tasks {
			s.db.Exec(`INSERT OR IGNORE INTO model_capabilities VALUES (?,?)`, mid, t)
		}
	}

	// Seed default tenants
	s.db.Exec(`INSERT OR IGNORE INTO tenants (tenant_id, name, routing_mode, rate_limit_rps, burst_limit) VALUES
		('tenant-default', 'Default', 'balanced', 50, 100),
		('tenant-premium', 'Premium', 'latency_optimized', 200, 500),
		('tenant-economy', 'Economy', 'cost_optimized', 20, 40)`)

	return nil
}

// LoadModels returns all enabled models with their task capabilities.
func (s *Store) LoadModels() ([]*scoring.ModelRecord, error) {
	rows, err := s.db.Query(`
		SELECT model_id, version, tier, cost_per_1k, avg_latency_ms, max_tokens, executor_addr
		FROM models WHERE enabled=1 ORDER BY model_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*scoring.ModelRecord
	for rows.Next() {
		m := &scoring.ModelRecord{Enabled: true}
		var tier string
		err := rows.Scan(&m.ModelID, &m.Version, &tier, &m.CostPer1k, &m.AvgLatencyMs, &m.MaxTokens, &m.ExecutorAddr)
		if err != nil {
			return nil, err
		}
		m.Tier = tier
		m.Tasks = s.loadTasks(m.ModelID)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) loadTasks(modelID string) []string {
	rows, _ := s.db.Query(`SELECT task_type FROM model_capabilities WHERE model_id=?`, modelID)
	defer rows.Close()
	var tasks []string
	for rows.Next() {
		var t string
		rows.Scan(&t)
		tasks = append(tasks, t)
	}
	return tasks
}

// RolloutConfig is the live canary configuration.
type RolloutConfig struct {
	RolloutID      string
	BaseModelID    string
	CanaryModelID  string
	CanaryPct      float64
	AutoRollback   bool
	MaxP99Ratio    float64
	MaxErrorRate   float64
	Enabled        bool
}

// LoadRollouts returns all enabled rollout configurations.
func (s *Store) LoadRollouts() ([]*RolloutConfig, error) {
	rows, err := s.db.Query(`
		SELECT rollout_id, base_model_id, canary_model_id, canary_pct,
		       auto_rollback, max_p99_ratio, max_error_rate, enabled
		FROM rollouts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*RolloutConfig
	for rows.Next() {
		r := &RolloutConfig{}
		var ar int
		rows.Scan(&r.RolloutID, &r.BaseModelID, &r.CanaryModelID, &r.CanaryPct,
			&ar, &r.MaxP99Ratio, &r.MaxErrorRate, &r.Enabled)
		r.AutoRollback = ar == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) UpsertRollout(r *RolloutConfig) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO rollouts
			(rollout_id, base_model_id, canary_model_id, canary_pct, auto_rollback, max_p99_ratio, max_error_rate, enabled, updated_at)
		VALUES (?,?,?,?,?,?,?,?,datetime('now'))`,
		r.RolloutID, r.BaseModelID, r.CanaryModelID, r.CanaryPct,
		b2i(r.AutoRollback), r.MaxP99Ratio, r.MaxErrorRate, b2i(r.Enabled))
	return err
}

func (s *Store) RollbackRollout(rolloutID, reason string) error {
	_, err := s.db.Exec(`
		UPDATE rollouts SET enabled=0, rolled_back_at=datetime('now'), rollback_reason=?, updated_at=datetime('now')
		WHERE rollout_id=?`, reason, rolloutID)
	return err
}

// LoadTenantPolicy loads a tenant's routing policy.
func (s *Store) LoadTenantPolicy(tenantID string) (*policy.TenantPolicy, error) {
	row := s.db.QueryRow(`
		SELECT tenant_id, routing_mode, allowed_models, rate_limit_rps, burst_limit, enabled
		FROM tenants WHERE tenant_id=?`, tenantID)
	p := &policy.TenantPolicy{}
	var allowedJSON string
	var enabled int
	err := row.Scan(&p.TenantID, &p.RoutingMode, &allowedJSON, &p.RateLimit, &p.BurstLimit, &enabled)
	if err == sql.ErrNoRows {
		return policy.Default(tenantID), nil
	}
	if err != nil {
		return nil, err
	}
	p.Enabled = enabled == 1
	var allowed []string
	if err := json.Unmarshal([]byte(allowedJSON), &allowed); err == nil && len(allowed) > 0 {
		p.AllowedModels = make(map[string]bool)
		for _, m := range allowed {
			p.AllowedModels[m] = true
		}
	}
	return p, nil
}

// LogEntry is a row in the request audit log.
type LogEntry struct {
	RequestID    string
	TraceID      string
	TenantID     string
	TaskType     string
	ModelID      string
	TokensInput  int32
	TokensOutput int32
	LatencyMs    float64
	QueueWaitMs  float64
	CostUSD      float64
	Cached       bool
	FallbackUsed bool
	IsCanary     bool
	RoutingMode  string
}

func (s *Store) LogRequest(e *LogEntry) {
	s.db.Exec(`
		INSERT INTO request_log
			(request_id,trace_id,tenant_id,task_type,model_id,tokens_input,tokens_output,
			 latency_ms,queue_wait_ms,cost_usd,cached,fallback_used,is_canary,routing_mode)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.RequestID, e.TraceID, e.TenantID, e.TaskType, e.ModelID,
		e.TokensInput, e.TokensOutput, e.LatencyMs, e.QueueWaitMs, e.CostUSD,
		b2i(e.Cached), b2i(e.FallbackUsed), b2i(e.IsCanary), e.RoutingMode)
}

// WindowStats returns aggregated metrics over a time window.
func (s *Store) WindowStats(d time.Duration) map[string]any {
	cutoff := time.Now().Add(-d).Format("2006-01-02 15:04:05")
	row := s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(AVG(latency_ms),0), COALESCE(SUM(cost_usd),0),
		       COALESCE(SUM(tokens_output),0), COALESCE(SUM(cached),0), COALESCE(SUM(fallback_used),0)
		FROM request_log WHERE created_at>=?`, cutoff)
	var cnt, cached, fallback int64
	var avgLat, totalCost float64
	var totalTok int64
	row.Scan(&cnt, &avgLat, &totalCost, &totalTok, &cached, &fallback)
	return map[string]any{
		"requests":       cnt,
		"avg_latency_ms": round(avgLat, 2),
		"total_cost_usd": round(totalCost, 8),
		"tokens_out":     totalTok,
		"cache_hits":     cached,
		"fallbacks":      fallback,
		"window_sec":     d.Seconds(),
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func b2i(b bool) int { if b { return 1 }; return 0 }

func round(v float64, places int) float64 {
	p := 1.0
	for i := 0; i < places; i++ { p *= 10 }
	return float64(int(v*p+0.5)) / p
}

// Verify ensures Store can be opened and schema applied.
func (s *Store) Ping() error {
	return s.db.Ping()
}

var _ = fmt.Sprintf // keep fmt imported
// fx_463
// fx_464
