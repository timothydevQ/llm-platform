// Package registry manages the model registry with create/read/update/disable
// operations backed by SQLite.
package registry

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ─── Model ────────────────────────────────────────────────────────────────────

type Model struct {
	ModelID      string            `json:"model_id"`
	Version      string            `json:"version"`
	Name         string            `json:"name"`
	Tier         string            `json:"tier"`
	Capabilities []string          `json:"capabilities"`
	CostPer1k    float64           `json:"cost_per_1k"`
	AvgLatencyMs int               `json:"avg_latency_ms"`
	MaxTokens    int               `json:"max_tokens"`
	ExecutorAddr string            `json:"executor_addr"`
	Enabled      bool              `json:"enabled"`
	Labels       map[string]string `json:"labels"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// ─── Registry ─────────────────────────────────────────────────────────────────

type Registry struct{ db *sql.DB }

func New(db *sql.DB) *Registry { return &Registry{db: db} }

func (r *Registry) Register(m *Model) (bool, error) {
	if m.ModelID == "" || m.Name == "" || m.Tier == "" {
		return false, fmt.Errorf("model_id, name, and tier are required")
	}
	if m.Version == "" { m.Version = "v1" }

	labelsJSON, _ := json.Marshal(m.Labels)

	res, err := r.db.Exec(`
		INSERT INTO models (model_id, version, name, tier, cost_per_1k, avg_latency_ms, max_tokens, executor_addr, enabled, labels)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(model_id) DO UPDATE SET
			version=excluded.version, name=excluded.name, tier=excluded.tier,
			cost_per_1k=excluded.cost_per_1k, avg_latency_ms=excluded.avg_latency_ms,
			max_tokens=excluded.max_tokens, executor_addr=excluded.executor_addr,
			enabled=excluded.enabled, labels=excluded.labels, updated_at=datetime('now')`,
		m.ModelID, m.Version, m.Name, m.Tier,
		m.CostPer1k, m.AvgLatencyMs, m.MaxTokens, m.ExecutorAddr,
		b2i(m.Enabled), string(labelsJSON))
	if err != nil { return false, err }

	// Upsert capabilities
	r.db.Exec(`DELETE FROM model_capabilities WHERE model_id=?`, m.ModelID)
	for _, cap := range m.Capabilities {
		r.db.Exec(`INSERT OR IGNORE INTO model_capabilities VALUES (?,?)`, m.ModelID, cap)
	}

	rows, _ := res.RowsAffected()
	created := rows > 0
	return created, nil
}

func (r *Registry) List(enabledOnly bool) ([]*Model, error) {
	q := `SELECT model_id, version, name, tier, cost_per_1k, avg_latency_ms, max_tokens, executor_addr, enabled, labels, created_at, updated_at FROM models`
	if enabledOnly {
		q += ` WHERE enabled=1`
	}
	q += ` ORDER BY model_id`

	rows, err := r.db.Query(q)
	if err != nil { return nil, err }
	defer rows.Close()

	var out []*Model
	for rows.Next() {
		m := &Model{}
		var enabled int
		var labelsJSON, createdStr, updatedStr string
		err := rows.Scan(&m.ModelID, &m.Version, &m.Name, &m.Tier,
			&m.CostPer1k, &m.AvgLatencyMs, &m.MaxTokens, &m.ExecutorAddr,
			&enabled, &labelsJSON, &createdStr, &updatedStr)
		if err != nil { return nil, err }
		m.Enabled = enabled == 1
		json.Unmarshal([]byte(labelsJSON), &m.Labels)
		m.Capabilities = r.loadCaps(m.ModelID)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Registry) Get(modelID string) (*Model, error) {
	row := r.db.QueryRow(`
		SELECT model_id, version, name, tier, cost_per_1k, avg_latency_ms, max_tokens, executor_addr, enabled, labels
		FROM models WHERE model_id=?`, modelID)
	m := &Model{}
	var enabled int
	var labelsJSON string
	err := row.Scan(&m.ModelID, &m.Version, &m.Name, &m.Tier,
		&m.CostPer1k, &m.AvgLatencyMs, &m.MaxTokens, &m.ExecutorAddr, &enabled, &labelsJSON)
	if err == sql.ErrNoRows { return nil, nil }
	if err != nil { return nil, err }
	m.Enabled = enabled == 1
	json.Unmarshal([]byte(labelsJSON), &m.Labels)
	m.Capabilities = r.loadCaps(m.ModelID)
	return m, nil
}

func (r *Registry) SetEnabled(modelID string, enabled bool) error {
	res, err := r.db.Exec(`UPDATE models SET enabled=?,updated_at=datetime('now') WHERE model_id=?`, b2i(enabled), modelID)
	if err != nil { return err }
	n, _ := res.RowsAffected()
	if n == 0 { return fmt.Errorf("model %s not found", modelID) }
	return nil
}

func (r *Registry) loadCaps(modelID string) []string {
	rows, _ := r.db.Query(`SELECT task_type FROM model_capabilities WHERE model_id=? ORDER BY task_type`, modelID)
	defer rows.Close()
	var caps []string
	for rows.Next() {
		var c string
		rows.Scan(&c)
		caps = append(caps, c)
	}
	return caps
}

func b2i(b bool) int {
	if b { return 1 }
	return 0
}
// tw_6059_23985
