package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/timothydevQ/llm-platform/services/control-plane/internal/quota"
	"github.com/timothydevQ/llm-platform/services/control-plane/internal/registry"
	"github.com/timothydevQ/llm-platform/services/control-plane/internal/rollout"

	_ "modernc.org/sqlite"
)

type Server struct {
	reg      *registry.Registry
	rollouts *rollout.Manager
	quotas   *quota.Enforcer
}

func NewServer(db *sql.DB) *Server {
	return &Server{
		reg:      registry.New(db),
		rollouts: rollout.New(db),
		quotas:   quota.NewEnforcer(db),
	}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// ── Health ────────────────────────────────────────────────────────────────
	mux.HandleFunc("/healthz/live",  func(w http.ResponseWriter, _ *http.Request) { ok(w, map[string]string{"status":"alive"}) })
	mux.HandleFunc("/healthz/ready", func(w http.ResponseWriter, _ *http.Request) { ok(w, map[string]string{"status":"ready"}) })

	// ── Model registry ────────────────────────────────────────────────────────
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			models, err := s.reg.List(false)
			if err != nil { fail(w, 500, err.Error()); return }
			ok(w, models)
		case http.MethodPost:
			var m registry.Model
			if err := json.NewDecoder(r.Body).Decode(&m); err != nil { fail(w, 400, err.Error()); return }
			created, err := s.reg.Register(&m)
			if err != nil { fail(w, 400, err.Error()); return }
			ok(w, map[string]any{"model_id": m.ModelID, "created": created})
		default:
			http.Error(w, "method not allowed", 405)
		}
	})

	mux.HandleFunc("/v1/models/", func(w http.ResponseWriter, r *http.Request) {
		modelID := r.URL.Path[len("/v1/models/"):]
		switch r.Method {
		case http.MethodGet:
			m, err := s.reg.Get(modelID)
			if err != nil { fail(w, 500, err.Error()); return }
			if m == nil { fail(w, 404, "model not found"); return }
			ok(w, m)
		case http.MethodPatch:
			var body struct{ Enabled *bool `json:"enabled"` }
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil { fail(w, 400, err.Error()); return }
			if body.Enabled == nil { fail(w, 400, "enabled field required"); return }
			if err := s.reg.SetEnabled(modelID, *body.Enabled); err != nil { fail(w, 400, err.Error()); return }
			ok(w, map[string]string{"status":"updated"})
		default:
			http.Error(w, "method not allowed", 405)
		}
	})

	// ── Rollouts ──────────────────────────────────────────────────────────────
	mux.HandleFunc("/v1/rollouts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			cfgs, err := s.rollouts.List()
			if err != nil { fail(w, 500, err.Error()); return }
			ok(w, cfgs)
		case http.MethodPost:
			var cfg rollout.Config
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil { fail(w, 400, err.Error()); return }
			if err := s.rollouts.Upsert(&cfg); err != nil { fail(w, 400, err.Error()); return }
			ok(w, map[string]string{"rollout_id": cfg.RolloutID, "status":"configured"})
		default:
			http.Error(w, "method not allowed", 405)
		}
	})

	mux.HandleFunc("/v1/rollouts/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete { http.Error(w, "method not allowed", 405); return }
		rolloutID := r.URL.Path[len("/v1/rollouts/"):]
		var body struct{ Reason string `json:"reason"` }
		json.NewDecoder(r.Body).Decode(&body)
		if body.Reason == "" { body.Reason = "manual rollback" }
		if err := s.rollouts.Rollback(rolloutID, body.Reason); err != nil { fail(w, 400, err.Error()); return }
		ok(w, map[string]string{"status":"rolled_back"})
	})

	// ── Rollout weights (for the router to poll) ──────────────────────────────
	mux.HandleFunc("/v1/rollout-weights", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet { http.Error(w, "method not allowed", 405); return }
		ok(w, s.rollouts.RolloutWeights())
	})

	// ── Quotas ────────────────────────────────────────────────────────────────
	mux.HandleFunc("/v1/quotas", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }
		var cfg quota.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil { fail(w, 400, err.Error()); return }
		if err := s.quotas.UpsertConfig(&cfg); err != nil { fail(w, 500, err.Error()); return }
		ok(w, map[string]string{"status":"configured"})
	})

	mux.HandleFunc("/v1/quotas/", func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.URL.Path[len("/v1/quotas/"):]
		switch r.Method {
		case http.MethodGet:
			u := s.quotas.GetUsage(tenantID)
			ok(w, u)
		case http.MethodPost:
			// Check endpoint — used by gateway to pre-flight quota before routing
			var body struct {
				EstTokens    int64 `json:"estimated_tokens"`
				ContextTokens int32 `json:"context_tokens"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil { fail(w, 400, err.Error()); return }
			result := s.quotas.Check(tenantID, body.EstTokens, body.ContextTokens)
			if result.Allowed {
				ok(w, map[string]bool{"allowed": true})
			} else {
				ok(w, map[string]any{"allowed": false, "reason": result.Reason})
			}
		default:
			http.Error(w, "method not allowed", 405)
		}
	})

	// ── Metrics ───────────────────────────────────────────────────────────────
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		models, _ := s.reg.List(false)
		enabled := 0
		for _, m := range models {
			if m.Enabled { enabled++ }
		}
		rollouts, _ := s.rollouts.List()
		active := 0
		for _, r := range rollouts {
			if r.Enabled { active++ }
		}
		fmt.Fprintf(w, "control_plane_models_total %d\n", len(models))
		fmt.Fprintf(w, "control_plane_models_enabled %d\n", enabled)
		fmt.Fprintf(w, "control_plane_rollouts_active %d\n", active)
	})

	return mux
}

func main() {
	dbPath   := getenv("DB_PATH",   "/data/control-plane.db")
	httpPort := getenv("HTTP_PORT", "8083")

	db, err := openDB(dbPath)
	if err != nil { slog.Error("open db", "err", err); os.Exit(1) }
	defer db.Close()

	srv := NewServer(db)
	if err := seedDB(db); err != nil { slog.Warn("seed", "err", err) }

	httpSrv := &http.Server{
		Addr:         ":" + httpPort,
		Handler:      srv.routes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		slog.Info("Control Plane started", "port", httpPort)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	httpSrv.Shutdown(ctx)
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil { return nil, err }
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`
	PRAGMA journal_mode=WAL;
	PRAGMA foreign_keys=ON;
	CREATE TABLE IF NOT EXISTS models (
		model_id TEXT PRIMARY KEY, version TEXT DEFAULT 'v1', name TEXT, tier TEXT,
		cost_per_1k REAL DEFAULT 0.001, avg_latency_ms INTEGER DEFAULT 500,
		max_tokens INTEGER DEFAULT 4096, executor_addr TEXT DEFAULT '',
		enabled INTEGER DEFAULT 1, labels TEXT DEFAULT '{}',
		created_at TEXT DEFAULT (datetime('now')), updated_at TEXT DEFAULT (datetime('now'))
	);
	CREATE TABLE IF NOT EXISTS model_capabilities (
		model_id TEXT, task_type TEXT, PRIMARY KEY(model_id, task_type)
	);
	CREATE TABLE IF NOT EXISTS rollouts (
		rollout_id TEXT PRIMARY KEY, base_model_id TEXT, canary_model_id TEXT,
		canary_pct REAL DEFAULT 0, auto_rollback INTEGER DEFAULT 1,
		max_p99_ratio REAL DEFAULT 2.0, max_error_rate REAL DEFAULT 0.05,
		enabled INTEGER DEFAULT 0, rolled_back_at TEXT, rollback_reason TEXT,
		updated_at TEXT DEFAULT (datetime('now'))
	);
	CREATE TABLE IF NOT EXISTS rollout_metrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT, rollout_id TEXT, model_id TEXT,
		window_start TEXT, request_count INTEGER DEFAULT 0, error_count INTEGER DEFAULT 0,
		p99_latency_ms REAL DEFAULT 0, avg_tokens REAL DEFAULT 0, total_cost_usd REAL DEFAULT 0,
		recorded_at TEXT DEFAULT (datetime('now'))
	);
	CREATE TABLE IF NOT EXISTS quotas (
		tenant_id TEXT PRIMARY KEY, tokens_per_minute INTEGER DEFAULT 100000,
		tokens_per_day INTEGER DEFAULT 5000000, budget_usd_per_day REAL DEFAULT 50,
		max_context_tokens INTEGER DEFAULT 8192, updated_at TEXT DEFAULT (datetime('now'))
	);
	CREATE TABLE IF NOT EXISTS quota_usage (
		tenant_id TEXT, window_key TEXT, window_type TEXT,
		tokens_used INTEGER DEFAULT 0, cost_usd REAL DEFAULT 0,
		updated_at TEXT DEFAULT (datetime('now')),
		PRIMARY KEY (tenant_id, window_key, window_type)
	);
	`)
	return db, err
}

func seedDB(db *sql.DB) error {
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM models`).Scan(&n)
	if n > 0 { return nil }

	execAddr := getenv("EXECUTOR_ADDR", "model-executor:50051")
	models := []struct{ id, name, tier string; cost float64; lat, max int }{
		{"gpt-small",  "GPT Small",  "small",  0.0002, 200,  4096},
		{"gpt-medium", "GPT Medium", "medium", 0.002,  500,  8192},
		{"gpt-large",  "GPT Large",  "large",  0.02,   1200, 32768},
		{"embed-v2",   "Embed v2",   "small",  0.0001, 50,   8192},
		{"rerank-v1",  "Rerank v1",  "small",  0.0002, 100,  4096},
	}
	for _, m := range models {
		db.Exec(`INSERT OR IGNORE INTO models (model_id,name,tier,cost_per_1k,avg_latency_ms,max_tokens,executor_addr) VALUES(?,?,?,?,?,?,?)`,
			m.id, m.name, m.tier, m.cost, m.lat, m.max, execAddr)
	}
	caps := map[string][]string{
		"gpt-small": {"chat","summarize","classify","moderate"},
		"gpt-medium":{"chat","summarize","classify","moderate"},
		"gpt-large": {"chat","summarize"},
		"embed-v2":  {"embed"}, "rerank-v1": {"rerank"},
	}
	for mid, ts := range caps {
		for _, t := range ts { db.Exec(`INSERT OR IGNORE INTO model_capabilities VALUES(?,?)`, mid, t) }
	}
	return nil
}

func ok(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func getenv(k, fb string) string { if v := os.Getenv(k); v != "" { return v }; return fb }
// tw_6059_21221
