// Package rollout manages canary deployments with automatic rollback.
//
// A rollout sends configurable traffic percentage to a canary model version.
// A background evaluator periodically compares canary vs base metrics, and
// triggers rollback if the canary's p99 latency ratio or error rate exceed
// configured thresholds.
package rollout

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ─── Rollout config ───────────────────────────────────────────────────────────

type Config struct {
	RolloutID     string  `json:"rollout_id"`
	BaseModelID   string  `json:"base_model_id"`
	CanaryModelID string  `json:"canary_model_id"`
	CanaryPct     float64 `json:"canary_pct"`
	AutoRollback  bool    `json:"auto_rollback"`
	MaxP99Ratio   float64 `json:"max_p99_ratio"`   // canary_p99 / base_p99 > this → rollback
	MaxErrorRate  float64 `json:"max_error_rate"`  // canary error rate > this → rollback
	Enabled       bool    `json:"enabled"`
}

// ─── Window metrics (aggregated per eval window) ───────────────────────────────

type WindowMetrics struct {
	ModelID    string
	WindowKey  string  // "2026-03-15T14:00"
	Requests   int64
	Errors     int64
	P99LatMs   float64
	AvgTokens  float64
	TotalCost  float64
}

func (m *WindowMetrics) ErrorRate() float64 {
	if m.Requests == 0 { return 0 }
	return float64(m.Errors) / float64(m.Requests)
}

// ─── Manager ──────────────────────────────────────────────────────────────────

type Manager struct {
	db       *sql.DB
	mu       sync.RWMutex
	configs  map[string]*Config
}

func New(db *sql.DB) *Manager {
	m := &Manager{db: db, configs: make(map[string]*Config)}
	go m.evaluateLoop()
	return m
}

func (m *Manager) Upsert(cfg *Config) error {
	if cfg.RolloutID == ""  { return fmt.Errorf("rollout_id required") }
	if cfg.BaseModelID == "" { return fmt.Errorf("base_model_id required") }
	if cfg.CanaryModelID == "" { return fmt.Errorf("canary_model_id required") }
	if cfg.CanaryPct < 0 || cfg.CanaryPct > 1 {
		return fmt.Errorf("canary_pct must be between 0 and 1")
	}

	_, err := m.db.Exec(`
		INSERT OR REPLACE INTO rollouts
			(rollout_id, base_model_id, canary_model_id, canary_pct, auto_rollback,
			 max_p99_ratio, max_error_rate, enabled, updated_at)
		VALUES (?,?,?,?,?,?,?,?,datetime('now'))`,
		cfg.RolloutID, cfg.BaseModelID, cfg.CanaryModelID, cfg.CanaryPct,
		b2i(cfg.AutoRollback), cfg.MaxP99Ratio, cfg.MaxErrorRate, b2i(cfg.Enabled))
	if err != nil { return err }

	m.mu.Lock()
	m.configs[cfg.RolloutID] = cfg
	m.mu.Unlock()

	slog.Info("rollout upserted",
		"rollout_id", cfg.RolloutID,
		"base", cfg.BaseModelID,
		"canary", cfg.CanaryModelID,
		"pct", cfg.CanaryPct)
	return nil
}

func (m *Manager) List() ([]*Config, error) {
	rows, err := m.db.Query(`
		SELECT rollout_id, base_model_id, canary_model_id, canary_pct,
		       auto_rollback, max_p99_ratio, max_error_rate, enabled
		FROM rollouts ORDER BY rollout_id`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []*Config
	for rows.Next() {
		c := &Config{}
		var ar, en int
		rows.Scan(&c.RolloutID, &c.BaseModelID, &c.CanaryModelID, &c.CanaryPct,
			&ar, &c.MaxP99Ratio, &c.MaxErrorRate, &en)
		c.AutoRollback = ar == 1
		c.Enabled = en == 1
		out = append(out, c)
	}
	return out, rows.Err()
}

func (m *Manager) Rollback(rolloutID, reason string) error {
	_, err := m.db.Exec(`
		UPDATE rollouts SET enabled=0, rolled_back_at=datetime('now'), rollback_reason=?, updated_at=datetime('now')
		WHERE rollout_id=?`, reason, rolloutID)
	if err != nil { return err }

	m.mu.Lock()
	if cfg, ok := m.configs[rolloutID]; ok {
		cfg.Enabled = false
	}
	m.mu.Unlock()

	slog.Warn("rollout rolled back", "rollout_id", rolloutID, "reason", reason)
	return err
}

// RecordMetrics records a request outcome for rollout evaluation.
func (m *Manager) RecordMetrics(wm *WindowMetrics) {
	m.db.Exec(`
		INSERT INTO rollout_metrics
			(rollout_id, model_id, window_start, request_count, error_count, p99_latency_ms, avg_tokens, total_cost_usd)
		VALUES
			((SELECT rollout_id FROM rollouts WHERE (base_model_id=? OR canary_model_id=?) AND enabled=1 LIMIT 1),
			 ?,?,?,?,?,?,?)`,
		wm.ModelID, wm.ModelID,
		wm.ModelID, wm.WindowKey, wm.Requests, wm.Errors,
		wm.P99LatMs, wm.AvgTokens, wm.TotalCost)
}

// RolloutWeights returns a map of model_id → traffic weight for active rollouts.
// base model gets (1 - canary_pct), canary gets canary_pct.
func (m *Manager) RolloutWeights() map[string]float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	weights := make(map[string]float64)
	for _, cfg := range m.configs {
		if !cfg.Enabled { continue }
		weights[cfg.BaseModelID] = 1.0 - cfg.CanaryPct
		weights[cfg.CanaryModelID] = cfg.CanaryPct
	}
	return weights
}

// evaluateLoop periodically checks canary health and triggers auto-rollback.
func (m *Manager) evaluateLoop() {
	for range time.NewTicker(2 * time.Minute).C {
		m.evaluateAll()
	}
}

func (m *Manager) evaluateAll() {
	configs, err := m.List()
	if err != nil { return }

	for _, cfg := range configs {
		if !cfg.Enabled || !cfg.AutoRollback { continue }
		if m.shouldRollback(cfg) {
			reason := fmt.Sprintf("auto-rollback: canary metrics exceeded thresholds (pct=%.0f%%)", cfg.CanaryPct*100)
			m.Rollback(cfg.RolloutID, reason)
		}
	}
}

func (m *Manager) shouldRollback(cfg *Config) bool {
	// Query last 10 minutes of metrics for both models
	window := time.Now().Add(-10 * time.Minute).Format("2006-01-02T15:04")
	row := m.db.QueryRow(`
		SELECT COALESCE(AVG(p99_latency_ms),0), COALESCE(SUM(error_count)*1.0/NULLIF(SUM(request_count),0),0)
		FROM rollout_metrics WHERE model_id=? AND window_start >= ?`,
		cfg.CanaryModelID, window)
	var canaryP99, canaryErr float64
	row.Scan(&canaryP99, &canaryErr)

	row = m.db.QueryRow(`
		SELECT COALESCE(AVG(p99_latency_ms),0), COALESCE(SUM(error_count)*1.0/NULLIF(SUM(request_count),0),0)
		FROM rollout_metrics WHERE model_id=? AND window_start >= ?`,
		cfg.BaseModelID, window)
	var baseP99, _ float64
	row.Scan(&baseP99, nil)

	if canaryErr > cfg.MaxErrorRate {
		slog.Warn("canary error rate exceeded", "rollout", cfg.RolloutID, "rate", canaryErr)
		return true
	}
	if baseP99 > 0 && canaryP99/baseP99 > cfg.MaxP99Ratio {
		slog.Warn("canary p99 ratio exceeded", "rollout", cfg.RolloutID, "ratio", canaryP99/baseP99)
		return true
	}
	return false
}

func b2i(b bool) int {
	if b { return 1 }
	return 0
}
// tw_6059_30247
