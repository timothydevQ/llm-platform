-- Migration 001: LLM Platform core schema
-- Compatible with PostgreSQL 15+ and SQLite 3.37+

PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

-- ── Model registry ─────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS models (
    model_id        TEXT    PRIMARY KEY,
    version         TEXT    NOT NULL DEFAULT 'v1',
    name            TEXT    NOT NULL,
    tier            TEXT    NOT NULL CHECK (tier IN ('small','medium','large')),
    cost_per_1k     REAL    NOT NULL DEFAULT 0.001,
    avg_latency_ms  INTEGER NOT NULL DEFAULT 500,
    max_tokens      INTEGER NOT NULL DEFAULT 4096,
    executor_addr   TEXT    NOT NULL,
    enabled         INTEGER NOT NULL DEFAULT 1,
    labels          TEXT    NOT NULL DEFAULT '{}',  -- JSON
    created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS model_capabilities (
    model_id  TEXT NOT NULL REFERENCES models(model_id) ON DELETE CASCADE,
    task_type TEXT NOT NULL CHECK (task_type IN ('chat','summarize','embed','rerank','classify','moderate')),
    PRIMARY KEY (model_id, task_type)
);

-- ── Routing policy ─────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS routing_rules (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_name      TEXT    UNIQUE NOT NULL,
    task_type      TEXT,               -- NULL = any
    cost_budget    TEXT,               -- 'low'|'medium'|'high'|NULL
    min_prompt_len INTEGER,
    max_prompt_len INTEGER,
    target_tier    TEXT    NOT NULL CHECK (target_tier IN ('small','medium','large')),
    priority       INTEGER NOT NULL DEFAULT 0,
    enabled        INTEGER NOT NULL DEFAULT 1,
    created_at     TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- ── Rollout / canary ────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS rollouts (
    rollout_id       TEXT    PRIMARY KEY,
    base_model_id    TEXT    NOT NULL,
    canary_model_id  TEXT    NOT NULL,
    canary_pct       REAL    NOT NULL DEFAULT 0.0 CHECK (canary_pct BETWEEN 0 AND 1),
    auto_rollback    INTEGER NOT NULL DEFAULT 1,
    max_p99_ratio    REAL    NOT NULL DEFAULT 2.0,
    max_error_rate   REAL    NOT NULL DEFAULT 0.05,
    enabled          INTEGER NOT NULL DEFAULT 0,
    rolled_back_at   TEXT,
    rollback_reason  TEXT,
    created_at       TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- ── Rollout evaluation windows (for auto-rollback) ─────────────────────────

CREATE TABLE IF NOT EXISTS rollout_metrics (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    rollout_id       TEXT    NOT NULL REFERENCES rollouts(rollout_id) ON DELETE CASCADE,
    model_id         TEXT    NOT NULL,
    window_start     TEXT    NOT NULL,
    request_count    INTEGER NOT NULL DEFAULT 0,
    error_count      INTEGER NOT NULL DEFAULT 0,
    p99_latency_ms   REAL    NOT NULL DEFAULT 0,
    avg_tokens       REAL    NOT NULL DEFAULT 0,
    total_cost_usd   REAL    NOT NULL DEFAULT 0,
    recorded_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- ── Tenants ─────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS tenants (
    tenant_id       TEXT    PRIMARY KEY,
    name            TEXT    NOT NULL,
    routing_mode    TEXT    NOT NULL DEFAULT 'balanced' CHECK (routing_mode IN ('latency_optimized','cost_optimized','balanced')),
    allowed_models  TEXT    NOT NULL DEFAULT '[]', -- JSON array, empty=all
    rate_limit_rps  INTEGER NOT NULL DEFAULT 50,
    burst_limit     INTEGER NOT NULL DEFAULT 100,
    enabled         INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- ── Quotas ──────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS quotas (
    tenant_id          TEXT    PRIMARY KEY REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    tokens_per_minute  INTEGER NOT NULL DEFAULT 100000,
    tokens_per_day     INTEGER NOT NULL DEFAULT 5000000,
    budget_usd_per_day REAL    NOT NULL DEFAULT 50.0,
    max_context_tokens INTEGER NOT NULL DEFAULT 8192,
    updated_at         TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- ── Quota usage tracking ────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS quota_usage (
    tenant_id    TEXT    NOT NULL,
    window_key   TEXT    NOT NULL, -- e.g. "2026-03-15" or "2026-03-15T14:00"
    window_type  TEXT    NOT NULL CHECK (window_type IN ('minute','day')),
    tokens_used  INTEGER NOT NULL DEFAULT 0,
    cost_usd     REAL    NOT NULL DEFAULT 0,
    updated_at   TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (tenant_id, window_key, window_type)
);

CREATE INDEX IF NOT EXISTS idx_quota_usage_tenant ON quota_usage(tenant_id, window_type, window_key DESC);

-- ── API keys ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS api_keys (
    key_id      TEXT    PRIMARY KEY,
    key_hash    TEXT    UNIQUE NOT NULL,
    tenant_id   TEXT    NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    name        TEXT,
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    last_used   TEXT
);

CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);

-- ── Executor registry ───────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS executor_nodes (
    executor_id     TEXT    PRIMARY KEY,
    address         TEXT    NOT NULL,
    status          TEXT    NOT NULL DEFAULT 'unknown',
    load_factor     REAL    NOT NULL DEFAULT 0.0,
    last_heartbeat  TEXT    NOT NULL DEFAULT (datetime('now')),
    registered_at   TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- ── Request audit log ───────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS request_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id      TEXT    NOT NULL,
    trace_id        TEXT,
    tenant_id       TEXT,
    task_type       TEXT    NOT NULL,
    model_id        TEXT,
    model_version   TEXT,
    executor_id     TEXT,
    tokens_input    INTEGER,
    tokens_output   INTEGER,
    latency_ms      REAL,
    queue_wait_ms   REAL,
    cost_usd        REAL,
    cached          INTEGER NOT NULL DEFAULT 0,
    fallback_used   INTEGER NOT NULL DEFAULT 0,
    is_canary       INTEGER NOT NULL DEFAULT 0,
    routing_mode    TEXT,
    error_code      TEXT,
    created_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_req_log_created   ON request_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_req_log_tenant    ON request_log(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_req_log_model     ON request_log(model_id, created_at DESC);

-- ── Scheduler batch log ─────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS batch_log (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    batch_id     TEXT    NOT NULL,
    model_id     TEXT    NOT NULL,
    task_type    TEXT    NOT NULL,
    batch_size   INTEGER NOT NULL,
    wait_ms      REAL    NOT NULL DEFAULT 0,
    dispatch_ms  REAL    NOT NULL DEFAULT 0,
    flush_reason TEXT,   -- 'max_size'|'max_wait'|'deadline'
    created_at   TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_batch_log_model ON batch_log(model_id, created_at DESC);
// tw_6059_16610
// tw_6059_26820
// tw_6059_25368
// tw_6059_9511
// tw_6059_9002
// tw_6059_23229
// tw_6059_1691
// tw_6059_7682
