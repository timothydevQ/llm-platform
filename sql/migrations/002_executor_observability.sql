-- Migration 002: Executor observability and model performance tracking
-- Adds tables for:
--   executor_heartbeats  - live executor node health (polled every 30s)
--   model_latency_stats  - rolling p50/p95/p99 per model per hour
--   quota_alerts         - audit trail of quota enforcement actions

PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

-- ── Executor heartbeat registry ────────────────────────────────────────────

-- Executors call POST /v1/executor/heartbeat every 30s.
-- The router polls this table to populate the scoring queue-depth signal
-- and to detect dead executors (last_seen > 90s ago).

CREATE TABLE IF NOT EXISTS executor_heartbeats (
    executor_id     TEXT    PRIMARY KEY,
    address         TEXT    NOT NULL,
    status          TEXT    NOT NULL DEFAULT 'healthy'
                            CHECK (status IN ('healthy','degraded','down')),
    load_factor     REAL    NOT NULL DEFAULT 0.0
                            CHECK (load_factor >= 0 AND load_factor <= 1),
    model_ids       TEXT    NOT NULL DEFAULT '[]', -- JSON array
    requests_served INTEGER NOT NULL DEFAULT 0,
    tokens_per_sec  REAL    NOT NULL DEFAULT 0.0,
    avg_latency_ms  REAL    NOT NULL DEFAULT 0.0,
    first_seen      TEXT    NOT NULL DEFAULT (datetime('now')),
    last_seen       TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_executor_last_seen
    ON executor_heartbeats(last_seen DESC);

-- View: executors considered alive (heartbeat within 90 seconds)
CREATE VIEW IF NOT EXISTS live_executors AS
    SELECT *
    FROM   executor_heartbeats
    WHERE  datetime(last_seen) >= datetime('now', '-90 seconds')
    AND    status != 'down';

-- ── Per-model latency statistics (hourly roll-up) ──────────────────────────

-- Populated by the router's background stats task every 5 minutes.
-- Used by:
--   - The scoring engine's health dimension (error_rate)
--   - The canary evaluator (p99 ratio check)
--   - The Grafana dashboard

CREATE TABLE IF NOT EXISTS model_latency_stats (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    model_id        TEXT    NOT NULL,
    window_start    TEXT    NOT NULL,  -- truncated to nearest hour: 2026-03-15T14:00
    request_count   INTEGER NOT NULL DEFAULT 0,
    error_count     INTEGER NOT NULL DEFAULT 0,
    p50_latency_ms  REAL    NOT NULL DEFAULT 0,
    p95_latency_ms  REAL    NOT NULL DEFAULT 0,
    p99_latency_ms  REAL    NOT NULL DEFAULT 0,
    avg_tokens_out  REAL    NOT NULL DEFAULT 0,
    total_cost_usd  REAL    NOT NULL DEFAULT 0,
    recorded_at     TEXT    NOT NULL DEFAULT (datetime('now')),
    UNIQUE (model_id, window_start)
);

CREATE INDEX IF NOT EXISTS idx_model_stats_model
    ON model_latency_stats(model_id, window_start DESC);

CREATE INDEX IF NOT EXISTS idx_model_stats_window
    ON model_latency_stats(window_start DESC);

-- ── Quota enforcement audit log ────────────────────────────────────────────

-- Every quota denial is written here. Gives operators a searchable record
-- of which tenants hit limits and when, without affecting the hot path.

CREATE TABLE IF NOT EXISTS quota_alerts (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id       TEXT    NOT NULL,
    denial_reason   TEXT    NOT NULL
                            CHECK (denial_reason IN (
                                'tokens_per_minute',
                                'tokens_per_day',
                                'budget_usd_per_day',
                                'context_length'
                            )),
    request_id      TEXT,
    estimated_tokens INTEGER,
    used_today      INTEGER,
    limit_today     INTEGER,
    created_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_quota_alerts_tenant
    ON quota_alerts(tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_quota_alerts_created
    ON quota_alerts(created_at DESC);

-- ── Executor load-shedding events ─────────────────────────────────────────

-- Written by the scheduler when it drops a request due to queue overflow.
-- Useful for capacity planning and SLO alerting.

CREATE TABLE IF NOT EXISTS load_shed_events (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    model_id     TEXT    NOT NULL,
    task_type    TEXT    NOT NULL,
    priority     TEXT    NOT NULL DEFAULT 'normal',
    queue_depth  INTEGER NOT NULL DEFAULT 0,
    tenant_id    TEXT,
    request_id   TEXT,
    created_at   TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_load_shed_model
    ON load_shed_events(model_id, created_at DESC);

-- ── Rolling-window performance view ───────────────────────────────────────

-- Convenience view used by the router's /v1/stats endpoint to populate
-- the per-model health signal without a complex join.

CREATE VIEW IF NOT EXISTS model_health_1h AS
    SELECT
        model_id,
        SUM(request_count)                              AS requests_1h,
        SUM(error_count)                                AS errors_1h,
        CASE WHEN SUM(request_count) > 0
             THEN CAST(SUM(error_count) AS REAL) / SUM(request_count)
             ELSE 0 END                                 AS error_rate_1h,
        MAX(p99_latency_ms)                             AS max_p99_1h,
        AVG(p99_latency_ms)                             AS avg_p99_1h,
        SUM(total_cost_usd)                             AS total_cost_1h
    FROM  model_latency_stats
    WHERE datetime(window_start) >= datetime('now', '-1 hour')
    GROUP BY model_id;
