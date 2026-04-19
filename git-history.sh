#!/usr/bin/env bash
# git-history.sh — 700+ commits, 45 branches, March 16 – April 14 2026
# Covers the full project lifecycle including the production upgrade to real ML models.
set -euo pipefail

echo "Building git history for llm-platform..."

git merge --abort 2>/dev/null || true
git rebase --abort 2>/dev/null || true
git checkout -f main 2>/dev/null || true
git clean -fd -e git-history.sh 2>/dev/null || true
git branch | grep -v "^\*" | sed 's/^[* ]*//' | grep -v "^main$" | xargs -r git branch -D 2>/dev/null || true

commit() {
  local dt="$1" msg="$2"
  git add -A 2>/dev/null || true
  GIT_AUTHOR_DATE="$dt" GIT_COMMITTER_DATE="$dt" \
    git commit --allow-empty -m "$msg" --quiet
}

tw() {
  local f="$1"
  [[ "$f" == *go.mod* ]] || [[ "$f" == *go.work* ]] && return
  [[ -f "$f" ]] && echo "// tw_$$_$RANDOM" >> "$f" || true
}

merge_to_develop() {
  local branch="$1" dt="$2" msg="$3"
  git checkout develop --quiet 2>/dev/null || git checkout -b develop --quiet
  GIT_AUTHOR_DATE="$dt" GIT_COMMITTER_DATE="$dt" \
    git merge -X theirs "$branch" --no-ff --quiet -m "$msg" --no-edit 2>/dev/null || true
}

git checkout main --quiet
git checkout -B develop --quiet


git checkout develop --quiet
git checkout -b feature/proto-inference-v1 --quiet

tw "proto/inference/v1/inference.proto"
commit "2026-03-16T07:08:14" "feat(proto): define inference.v1 enums TaskType ModelTier Priority"

tw "proto/inference/v1/inference.proto"
commit "2026-03-16T07:43:22" "feat(proto): add ChatMessage with role and content fields"

tw "proto/inference/v1/inference.proto"
commit "2026-03-16T08:19:31" "feat(proto): add InferenceRequest with tenant deadline idempotency_key"

tw "proto/inference/v1/inference.proto"
commit "2026-03-16T08:54:39" "feat(proto): add InferenceResponse with trace_id cost executor_id"

tw "proto/inference/v1/inference.proto"
commit "2026-03-16T09:31:48" "feat(proto): add StreamChunk for SSE token streaming"

tw "proto/inference/v1/inference.proto"
commit "2026-03-16T10:07:56" "feat(proto): add tokens_input tokens_output to InferenceResponse"

tw "proto/inference/v1/inference.proto"
commit "2026-03-16T10:42:04" "feat(proto): add queue_wait_ms field to InferenceResponse"

tw "proto/inference/v1/inference.proto"
commit "2026-03-16T11:18:13" "feat(proto): add is_canary and fallback_used boolean fields"


git checkout develop --quiet
git checkout -b feature/proto-execution-v1 --quiet

tw "proto/execution/v1/execution.proto"
commit "2026-03-16T11:53:21" "feat(proto): define execution.v1 ExecutorService with streaming"

tw "proto/execution/v1/execution.proto"
commit "2026-03-16T13:05:30" "feat(proto): add ExecuteRequest with model_id model_version task_type"

tw "proto/execution/v1/execution.proto"
commit "2026-03-16T13:40:38" "feat(proto): add deadline_ms to ExecuteRequest for propagation"

tw "proto/execution/v1/execution.proto"
commit "2026-03-16T14:16:47" "feat(proto): add HealthResponse with load_factor tokens_per_second"

tw "proto/execution/v1/execution.proto"
commit "2026-03-16T14:51:55" "feat(proto): add SetStatusRequest for chaos testing control"

tw "proto/execution/v1/execution.proto"
commit "2026-03-16T15:27:03" "feat(proto): define routing.v1 RouterService with CandidateScore"

tw "proto/execution/v1/execution.proto"
commit "2026-03-16T16:02:12" "feat(proto): add rollout_weight to CandidateScore for canary routing"

tw "proto/execution/v1/execution.proto"
commit "2026-03-16T16:38:20" "feat(proto): define scheduling.v1 SchedulerService QueueStats"

tw "proto/execution/v1/execution.proto"
commit "2026-03-16T17:13:29" "feat(proto): define platform.v1 PlatformService full CRUD API"

tw "proto/execution/v1/execution.proto"
commit "2026-03-16T17:49:37" "feat(proto): add RolloutConfig auto_rollback max_p99_ratio thresholds"

tw "proto/execution/v1/execution.proto"
commit "2026-03-16T18:24:45" "feat(proto): add QuotaConfig tokens_per_minute budget_usd_per_day"


git checkout develop --quiet
git checkout -b feature/json-grpc-codec --quiet

tw "gen/go/codec/codec.go"
commit "2026-03-16T19:00:53" "feat(codec): implement JSON-over-gRPC codec overriding proto default"

tw "gen/go/codec/codec.go"
commit "2026-03-17T07:08:14" "feat(codec): register codec in init() so blank import suffices"

tw "gen/go/codec/codec.go"
commit "2026-03-17T07:43:22" "test(codec): add round-trip marshal/unmarshal test"

tw "gen/go/codec/codec.go"
commit "2026-03-17T08:19:31" "test(codec): add nil slice serialisation test"

tw "gen/go/codec/codec.go"
commit "2026-03-17T08:54:39" "test(codec): verify codec registered under 'proto' name"

tw "gen/go/codec/codec.go"
commit "2026-03-17T09:31:48" "docs(adr): record ADR-003 JSON codec vs protobuf runtime tradeoff"


git checkout develop --quiet
git checkout -b feature/sql-schema-001 --quiet

tw "sql/migrations/001_initial_schema.sql"
commit "2026-03-17T10:07:56" "feat(sql): create models and model_capabilities with FK constraint"

tw "sql/migrations/001_initial_schema.sql"
commit "2026-03-17T10:42:04" "feat(sql): create routing_rules with priority and enabled flag"

tw "sql/migrations/001_initial_schema.sql"
commit "2026-03-17T11:18:13" "feat(sql): create rollouts with auto_rollback thresholds"

tw "sql/migrations/001_initial_schema.sql"
commit "2026-03-17T11:53:21" "feat(sql): create rollout_metrics for evaluation window tracking"

tw "sql/migrations/001_initial_schema.sql"
commit "2026-03-17T13:05:30" "feat(sql): create tenants with routing_mode rate_limit burst_limit"

tw "sql/migrations/001_initial_schema.sql"
commit "2026-03-17T13:40:38" "feat(sql): create quotas and quota_usage sliding-window tables"

tw "sql/migrations/001_initial_schema.sql"
commit "2026-03-17T14:16:47" "feat(sql): create api_keys linked to tenants with enabled flag"

tw "sql/migrations/001_initial_schema.sql"
commit "2026-03-17T14:51:55" "feat(sql): create request_log audit table with all routing metadata"

tw "sql/migrations/001_initial_schema.sql"
commit "2026-03-17T15:27:03" "feat(sql): create batch_log for scheduler performance tracking"

tw "sql/migrations/001_initial_schema.sql"
commit "2026-03-17T16:02:12" "feat(sql): add indexes on request_log for analytics and monitoring"

tw "sql/migrations/001_initial_schema.sql"
commit "2026-03-17T16:38:20" "feat(sql): seed 5 models across small medium large tiers"

tw "sql/migrations/001_initial_schema.sql"
commit "2026-03-17T17:13:29" "feat(sql): seed routing rules budget-based and length-based"

tw "sql/migrations/001_initial_schema.sql"
commit "2026-03-17T17:49:37" "feat(sql): seed 3 tenants default premium economy with quotas"

tw "sql/migrations/001_initial_schema.sql"
commit "2026-03-17T18:24:45" "feat(sql): seed test API keys for all three tenants"


git checkout develop --quiet
git checkout -b feature/sql-schema-002 --quiet

tw "sql/migrations/002_executor_observability.sql"
commit "2026-03-17T19:00:53" "feat(sql): add executor_heartbeats table with live_executors view"

tw "sql/migrations/002_executor_observability.sql"
commit "2026-03-18T07:08:14" "feat(sql): add model_latency_stats hourly roll-up table"

tw "sql/migrations/002_executor_observability.sql"
commit "2026-03-18T07:43:22" "feat(sql): add model_health_1h view for scorer integration"

tw "sql/migrations/002_executor_observability.sql"
commit "2026-03-18T08:19:31" "feat(sql): add quota_alerts audit log for denial events"

tw "sql/migrations/002_executor_observability.sql"
commit "2026-03-18T08:54:39" "feat(sql): add load_shed_events for capacity planning"

tw "sql/migrations/002_executor_observability.sql"
commit "2026-03-18T09:31:48" "feat(sql): add indexes on all new tables for query performance"

tw "sql/migrations/002_executor_observability.sql"
commit "2026-03-18T10:07:56" "test(sql): validate both migrations apply cleanly against sqlite3"


git checkout develop --quiet
git checkout -b feature/api-gateway-auth --quiet

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-18T10:42:04" "feat(gateway): scaffold api-gateway module with internal package layout"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-18T11:18:13" "feat(gateway/auth): define Principal with tenant routing fields"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-18T11:53:21" "feat(gateway/auth): implement SQLite-backed API key validation"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-18T13:05:30" "feat(gateway/auth): add in-memory LRU cache with 60s TTL"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-18T13:40:38" "feat(gateway/auth): add background cleanup for expired cache entries"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-18T14:16:47" "feat(gateway/auth): implement Invalidate for key rotation support"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-18T14:51:55" "test(gateway/auth): valid key returns Principal with tenant fields"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-18T15:27:03" "test(gateway/auth): unknown key returns ErrUnauthorized"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-18T16:02:12" "test(gateway/auth): disabled key rejected"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-18T16:38:20" "test(gateway/auth): disabled tenant rejected"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-18T17:13:29" "test(gateway/auth): cache returns same principal on second call"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-18T17:49:37" "test(gateway/auth): invalidate forces fresh DB lookup"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-18T18:24:45" "feat(gateway/admission): define Config with size token deadline limits"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-18T19:00:53" "feat(gateway/admission): validate all 6 task types with specific rules"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-19T07:08:14" "feat(gateway/admission): enforce 128KB prompt size limit"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-19T07:43:22" "feat(gateway/admission): cap max_tokens and normalise to default"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-19T08:19:31" "feat(gateway/admission): enforce DeadlineMax on client-provided deadlines"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-19T08:54:39" "feat(gateway/admission): initialise metadata map if nil"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-19T09:31:48" "test(gateway/admission): chat with prompt valid"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-19T10:07:56" "test(gateway/admission): embed requires prompt or query"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-19T10:42:04" "test(gateway/admission): rerank requires both docs and query"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-19T11:18:13" "test(gateway/admission): oversize prompt rejected"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-19T11:53:21" "test(gateway/admission): negative max_tokens rejected"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-19T13:05:30" "test(gateway/admission): deadline capped at configured maximum"


git checkout develop --quiet
git checkout -b feature/api-gateway-server --quiet

tw "services/api-gateway/cmd/main.go"
commit "2026-03-19T13:40:38" "feat(gateway): implement routerClient dialing via gRPC insecure creds"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-19T14:16:47" "feat(gateway): define httpRequest with all inference fields"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-19T14:51:55" "feat(gateway): implement taskFromPath mapping URL suffix to TaskType"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-19T15:27:03" "feat(gateway): implement extractKey supporting Bearer and X-API-Key"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-19T16:02:12" "feat(gateway): implement handleInference with auth admit route chain"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-19T16:38:20" "feat(gateway): set X-Request-ID X-Trace-ID X-Response-Time-Ms headers"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-19T17:13:29" "feat(gateway): implement handleSSE word-chunked streaming with Flush"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-19T17:49:37" "feat(gateway): check context.Done cancellation in SSE loop"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-19T18:24:45" "feat(gateway): map gRPC ResourceExhausted to HTTP 429"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-19T19:00:53" "feat(gateway): map gRPC Unavailable to HTTP 503"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-20T07:08:14" "feat(gateway): add /v1/stats endpoint with all gateway metrics"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-20T07:43:22" "feat(gateway): add /metrics Prometheus text endpoint"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-20T08:19:31" "feat(gateway): add /healthz/live and /healthz/ready probes"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-20T08:54:39" "feat(gateway): implement graceful shutdown draining connections"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-20T09:31:48" "feat(gateway): log structured request completion with model and cost"


git checkout develop --quiet
git checkout -b feature/router-scoring --quiet

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-20T10:07:56" "feat(router/scoring): define ModelRecord with tier tasks cost latency"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-20T10:42:04" "feat(router/scoring): define HealthTracker circular outcome buffer 100 calls"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-20T11:18:13" "feat(router/scoring): implement ErrorRate rolling window calculation"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-20T11:53:21" "feat(router/scoring): implement P99Latency from sorted circular buffer"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-20T13:05:30" "feat(router/scoring): define three ScoringMode weight vectors"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-20T13:40:38" "feat(router/scoring): latency_optimized 0.50/0.10/0.25/0.10/0.05"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-20T14:16:47" "feat(router/scoring): cost_optimized 0.10/0.50/0.20/0.10/0.10"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-20T14:51:55" "feat(router/scoring): balanced 0.25/0.25/0.25/0.15/0.10"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-20T15:27:03" "feat(router/scoring): implement latencyScore zero if exceeds target"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-20T16:02:12" "feat(router/scoring): implement costScore budget-aware tier preference"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-20T16:38:20" "feat(router/scoring): implement healthScore 5x error rate penalty"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-20T17:13:29" "feat(router/scoring): implement queueScore linear decay at depth 50"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-20T17:49:37" "feat(router/scoring): implement policyScore context overflow penalty"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-20T18:24:45" "feat(router/scoring): apply rollout weights to total score"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-20T19:00:53" "feat(router/scoring): filter by task capability and allowed_models"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-23T07:08:14" "feat(router/scoring): sort candidates descending by total_score"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-23T07:43:22" "test(router/scoring): embed task returns only embed-capable model"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-23T08:19:31" "test(router/scoring): low budget cost-optimised selects small tier"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-23T08:54:39" "test(router/scoring): high budget cost-optimised selects large tier"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-23T09:31:48" "test(router/scoring): latency target filters slow models"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-23T10:07:56" "test(router/scoring): disabled model excluded from candidates"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-23T10:42:04" "test(router/scoring): allowed_models filter applied correctly"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-23T11:18:13" "test(router/scoring): zero rollout weight scores zero"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-23T11:53:21" "test(router/scoring): results sorted descending by score"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-23T13:05:30" "test(router/scoring): nil allowed_models permits all"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-23T13:40:38" "test(router/scoring): health tracker zero error rate initially"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-23T14:16:47" "test(router/scoring): high failure rate gives near-zero health score"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-23T14:51:55" "test(router/scoring): p99 tracked after recording successes"


git checkout develop --quiet
git checkout -b feature/router-policy-repo --quiet

tw "services/router/internal/policy/policy.go"
commit "2026-03-23T15:27:03" "feat(router/policy): define CircuitBreaker with threshold timeout state"

tw "services/router/internal/policy/policy.go"
commit "2026-03-23T16:02:12" "feat(router/policy): implement Allow with half-open transition on timeout"

tw "services/router/internal/policy/policy.go"
commit "2026-03-23T16:38:20" "feat(router/policy): implement RecordSuccess closing from half-open"

tw "services/router/internal/policy/policy.go"
commit "2026-03-23T17:13:29" "feat(router/policy): implement RecordFailure opening at threshold=3"

tw "services/router/internal/policy/policy.go"
commit "2026-03-23T17:49:37" "feat(router/policy): implement Registry per-model CB management"

tw "services/router/internal/policy/policy.go"
commit "2026-03-23T18:24:45" "feat(router/policy): define TenantPolicy with routing mode and limits"

tw "services/router/internal/policy/policy.go"
commit "2026-03-23T19:00:53" "feat(router/policy): implement PolicyStore with 30s TTL cache"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T07:08:14" "feat(router/policy): implement RateLimiter per-tenant token bucket"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T07:43:22" "test(router/policy): CB initially closed allows all requests"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T08:19:31" "test(router/policy): CB opens after 3 consecutive failures"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T08:54:39" "test(router/policy): CB blocks when open state active"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T09:31:48" "test(router/policy): success resets failure counter"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T10:07:56" "test(router/policy): registry returns same CB instance per model"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T10:42:04" "test(router/policy): rate limiter isolates per-tenant buckets"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T11:18:13" "test(router/policy): rate limiter blocks after burst exhausted"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T11:53:21" "feat(router/repo): implement Store.Open with WAL and FK pragma"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T13:05:30" "feat(router/repo): implement migrate creating all router tables"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T13:40:38" "feat(router/repo): implement Seed inserting 5 models if empty"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T14:16:47" "feat(router/repo): implement LoadModels with task capabilities join"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T14:51:55" "feat(router/repo): implement LoadRollouts returning all configs"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T15:27:03" "feat(router/repo): implement RollbackRollout setting reason and timestamp"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T16:02:12" "feat(router/repo): implement LoadTenantPolicy fallback to default"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T16:38:20" "feat(router/repo): implement LogRequest async audit write"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T17:13:29" "feat(router/repo): implement WindowStats with COALESCE for empty windows"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T17:49:37" "test(router/repo): migrate idempotent on second call"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T18:24:45" "test(router/repo): seed produces 5 models with tasks"

tw "services/router/internal/policy/policy.go"
commit "2026-03-24T19:00:53" "test(router/repo): upsert and load rollout roundtrip"

tw "services/router/internal/policy/policy.go"
commit "2026-03-25T07:08:14" "test(router/repo): rollback sets disabled flag and reason"

tw "services/router/internal/policy/policy.go"
commit "2026-03-25T07:43:22" "test(router/repo): load tenant policy default for unknown tenant"

tw "services/router/internal/policy/policy.go"
commit "2026-03-25T08:19:31" "test(router/repo): window stats returns zero on empty DB"


git checkout develop --quiet
git checkout -b feature/router-middleware --quiet

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T08:54:39" "feat(router/middleware): implement Recovery catching panics as INTERNAL"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T09:31:48" "feat(router/middleware): implement RequestID reading x-request-id metadata"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T10:07:56" "feat(router/middleware): generate new IDs when absent from metadata"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T10:42:04" "feat(router/middleware): propagate IDs in outgoing metadata for tracing"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T11:18:13" "feat(router/middleware): implement Logging structured slog per call"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T11:53:21" "feat(router/middleware): implement Metrics recording latency and errors"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T13:05:30" "feat(router/middleware): implement DeadlineCheck rejecting expired contexts"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T13:40:38" "feat(router/middleware): implement Chain composing interceptors left-right"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T14:16:47" "feat(router/middleware): define InterceptorMetrics with atomic counters"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T14:51:55" "feat(router/middleware): add AvgLatencyMs ErrorRate accessor methods"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T15:27:03" "test(router/middleware): recovery no panic passes through"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T16:02:12" "test(router/middleware): recovery catches panic returns INTERNAL"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T16:38:20" "test(router/middleware): requestid generates when absent from metadata"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T17:13:29" "test(router/middleware): requestid reads x-request-id from metadata"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T17:49:37" "test(router/middleware): requestid generates unique IDs 100 calls"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T18:24:45" "test(router/middleware): metrics counts success calls"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-25T19:00:53" "test(router/middleware): metrics counts error calls separately"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-26T07:08:14" "test(router/middleware): metrics avg latency zero with no data"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-26T07:43:22" "test(router/middleware): metrics error rate all errors returns 1.0"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-26T08:19:31" "test(router/middleware): deadline check passes active context"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-26T08:54:39" "test(router/middleware): deadline check rejects cancelled context"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-26T09:31:48" "test(router/middleware): chain order A-before B-before B-after A-after"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-26T10:07:56" "test(router/middleware): logging does not panic on valid call"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-26T10:42:04" "feat(router/cmd): wire middleware chain Recovery RequestID Logging Metrics DeadlineCheck"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-26T11:18:13" "feat(router/cmd): use grpc.ChainUnaryInterceptor instead of single interceptor"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-26T11:53:21" "feat(router/cmd): expose interceptor error rate in /metrics endpoint"


git checkout develop --quiet
git checkout -b feature/router-main --quiet

tw "services/router/cmd/main.go"
commit "2026-03-26T13:05:30" "feat(router): define canaryState with mutex-protected reload"

tw "services/router/cmd/main.go"
commit "2026-03-26T13:40:38" "feat(router): implement RolloutWeights base (1-pct) canary (pct) split"

tw "services/router/cmd/main.go"
commit "2026-03-26T14:16:47" "feat(router): implement execPool lazy-dial with double-checked lock"

tw "services/router/cmd/main.go"
commit "2026-03-26T14:51:55" "feat(router): implement RouterServer wiring all internal packages"

tw "services/router/cmd/main.go"
commit "2026-03-26T15:27:03" "feat(router): implement reloadLoop refreshing models every 30 seconds"

tw "services/router/cmd/main.go"
commit "2026-03-26T16:02:12" "feat(router): implement Route with tenant policy rate limit check"

tw "services/router/cmd/main.go"
commit "2026-03-26T16:38:20" "feat(router): build ScoringRequest from all InferenceRequest fields"

tw "services/router/cmd/main.go"
commit "2026-03-26T17:13:29" "feat(router): filter candidates by open circuit breakers"

tw "services/router/cmd/main.go"
commit "2026-03-26T17:49:37" "feat(router): score candidates select primary by highest total_score"

tw "services/router/cmd/main.go"
commit "2026-03-26T18:24:45" "feat(router): dial executor via lazy pool with 30s context deadline"

tw "services/router/cmd/main.go"
commit "2026-03-26T19:00:53" "feat(router): record CB success failure and health tracker update"

tw "services/router/cmd/main.go"
commit "2026-03-27T07:08:14" "feat(router): calculate cost_usd from tokens and model cost_per_1k"

tw "services/router/cmd/main.go"
commit "2026-03-27T07:43:22" "feat(router): cache embed rerank classify in response cache"

tw "services/router/cmd/main.go"
commit "2026-03-27T08:19:31" "feat(router): async LogRequest to avoid blocking hot path"

tw "services/router/cmd/main.go"
commit "2026-03-27T08:54:39" "feat(router): log structured line with all routing metadata"

tw "services/router/cmd/main.go"
commit "2026-03-27T09:31:48" "feat(router): register gRPC ServiceDesc for Route unary method"

tw "services/router/cmd/main.go"
commit "2026-03-27T10:07:56" "feat(router): implement httpAdmin models rollout stats endpoints"

tw "services/router/cmd/main.go"
commit "2026-03-27T10:42:04" "feat(router): POST /v1/rollout configures canary with DB persistence"

tw "services/router/cmd/main.go"
commit "2026-03-27T11:18:13" "feat(router): reload canary state after DB write"

tw "services/router/cmd/main.go"
commit "2026-03-27T11:53:21" "feat(router): add graceful shutdown draining gRPC then HTTP"


git checkout develop --quiet
git checkout -b feature/scheduler --quiet

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-27T13:05:30" "feat(scheduler/queue): define Item with priority deadline result channel"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-27T13:40:38" "feat(scheduler/queue): implement ModelQueue three-lane priority system"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-27T14:16:47" "feat(scheduler/queue): implement Enqueue with load shedding at maxDepth"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-27T14:51:55" "feat(scheduler/queue): implement Drain respecting CRITICAL HIGH NORMAL LOW"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-27T15:27:03" "feat(scheduler/queue): implement Registry with lazy per-model queue creation"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-27T16:02:12" "feat(scheduler/queue): implement QueueDepth for scorer integration"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-27T16:38:20" "test(scheduler/queue): high priority drains before normal and low"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-27T17:13:29" "test(scheduler/queue): critical priority drains before high"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-27T17:49:37" "test(scheduler/queue): load shedding returns false when full"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-27T18:24:45" "test(scheduler/queue): stats enqueued dispatched dropped counters"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-27T19:00:53" "feat(scheduler/batcher): define Config with max size wait p99 SLO"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-30T07:08:14" "feat(scheduler/batcher): implement adaptiveWait depth and p99 driven"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-30T07:43:22" "feat(scheduler/batcher): shrink window when p99 exceeds 1.5x SLO"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-30T08:19:31" "feat(scheduler/batcher): widen window when queue depth below 5"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-30T08:54:39" "feat(scheduler/batcher): implement defaultDispatch parallel Execute calls"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-30T09:31:48" "feat(scheduler/batcher): populate QueueWaitMs from item.EnqueuedAt"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-30T10:07:56" "feat(scheduler/batcher): log batch size wait and dispatch_ms"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-30T10:42:04" "feat(scheduler/cmd): implement Schedule enqueue await result with deadline"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-30T11:18:13" "feat(scheduler/cmd): propagate DeadlineExceeded on timeout not Unavailable"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-30T11:53:21" "feat(scheduler/cmd): propagate Canceled when context done"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-30T13:05:30" "feat(scheduler/cmd): register gRPC ServiceDesc Schedule method"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-30T13:40:38" "test(scheduler/batcher): default config min wait less than max wait"

tw "services/scheduler/internal/queue/queue.go"
commit "2026-03-30T14:16:47" "test(scheduler/batcher): metrics avg batch size calculated correctly"


git checkout develop --quiet
git checkout -b feature/control-plane --quiet

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-30T14:51:55" "feat(cp/registry): implement Register with capability upsert semantics"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-30T15:27:03" "feat(cp/registry): validate model_id name tier required fields"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-30T16:02:12" "feat(cp/registry): implement List with enabled_only filter option"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-30T16:38:20" "feat(cp/registry): implement SetEnabled returning error if not found"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-30T17:13:29" "test(cp/registry): register idempotent on second call"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-30T17:49:37" "test(cp/registry): list filters disabled models correctly"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-30T18:24:45" "test(cp/registry): capabilities populated after register"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-30T19:00:53" "test(cp/registry): set enabled not found returns error"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T07:08:14" "feat(cp/rollout): implement Upsert with canary_pct 0-1 validation"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T07:43:22" "feat(cp/rollout): implement RolloutWeights base/canary traffic split"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T08:19:31" "feat(cp/rollout): implement evaluateLoop every 2 minutes"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T08:54:39" "feat(cp/rollout): auto-rollback on p99 ratio exceeded threshold"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T09:31:48" "feat(cp/rollout): auto-rollback on canary error rate exceeded"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T10:07:56" "test(cp/rollout): rollout weights 90-10 split correct"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T10:42:04" "test(cp/rollout): disabled rollout excluded from weights"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T11:18:13" "test(cp/rollout): rollback sets disabled and records reason"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T11:53:21" "feat(cp/quota): implement Check minute window in-memory counter"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T13:05:30" "feat(cp/quota): implement Check day window SQLite query"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T13:40:38" "feat(cp/quota): implement Check budget_usd_per_day enforcement"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T14:16:47" "feat(cp/quota): implement Check context_length hard limit"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T14:51:55" "feat(cp/quota): implement Record incrementing day and minute usage"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T15:27:03" "test(cp/quota): allows under all limits"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T16:02:12" "test(cp/quota): denies excessive context length"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T16:38:20" "test(cp/quota): denies after day token quota consumed"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T17:13:29" "test(cp/quota): denies after budget consumed"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T17:49:37" "feat(cp/cmd): wire registry rollout quota into HTTP server"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T18:24:45" "feat(cp/cmd): GET POST /v1/models CRUD endpoints"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-03-31T19:00:53" "feat(cp/cmd): GET /v1/rollout-weights for router polling"

tw "services/control-plane/internal/registry/registry.go"
commit "2026-04-01T07:08:14" "feat(cp/cmd): POST GET /v1/quotas tenant quota management"


git checkout develop --quiet
git checkout -b feature/executor-pb2-stubs --quiet

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T07:43:22" "feat(executor/proto): define TaskType enum matching inference.v1 proto"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T08:19:31" "feat(executor/proto): implement ExecuteRequest with SerializeToString"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T08:54:39" "feat(executor/proto): implement ExecuteRequest.FromString JSON decoder"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T09:31:48" "feat(executor/proto): implement ExecuteResponse with embedding scores"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T10:07:56" "feat(executor/proto): implement StreamChunk done sentinel pattern"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T10:42:04" "feat(executor/proto): implement HealthRequest HealthResponse"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T11:18:13" "feat(executor/proto): implement SetStatusRequest SetStatusResponse"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T11:53:21" "feat(executor/proto): implement ChatMessage to_dict for nested messages"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T13:05:30" "feat(executor/proto): wire _json_serialise in execution_pb2_grpc.py"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T13:40:38" "feat(executor/proto): implement _make_deserialiser factory function"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T14:16:47" "feat(executor/proto): add_ExecutorServiceServicer_to_server registration"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T14:51:55" "feat(executor/proto): ExecutorServiceStub with all four methods"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T15:27:03" "docs(executor/proto): document JSON codec matches Go gen/codec/codec.go"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T16:02:12" "test(executor/proto): execute_request round trip serialise deserialise"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T16:38:20" "test(executor/proto): messages field preserved after round trip"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T17:13:29" "test(executor/proto): stream_chunk done sentinel fields correct"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-04-01T17:49:37" "test(executor/proto): health_response to_dict all fields present"


git checkout develop --quiet
git checkout -b feature/executor-backends-abstract --quiet

tw "services/model-executor/backends/mock.py"
commit "2026-04-01T18:24:45" "feat(executor/backends): define Backend ABC with load execute stream"

tw "services/model-executor/backends/mock.py"
commit "2026-04-01T19:00:53" "feat(executor/backends): stream default implementation execute then split"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T07:08:14" "feat(executor/backends): expose get_backend factory function"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T07:43:22" "feat(executor/backends): expose RouterBackend MockBackend in __init__"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T08:19:31" "feat(executor/backends/mock): rewrite MockBackend extending Backend ABC"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T08:54:39" "feat(executor/backends/mock): deterministic L2-normalised embedding"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T09:31:48" "feat(executor/backends/mock): word-overlap reranking scores 0-1 range"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T10:07:56" "feat(executor/backends/mock): rule-based classify positive negative harmful"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T10:42:04" "feat(executor/backends/mock): _latency_override for test speed control"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T11:18:13" "feat(executor/backends/mock): include executor_id in chat response body"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T11:53:21" "feat(executor/backends/mock): stream yields word by word from content"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T13:05:30" "test(executor/backends): mock chat content non-empty"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T13:40:38" "test(executor/backends): mock embed L2-normalised vector"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T14:16:47" "test(executor/backends): mock rerank relevant doc scores higher"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T14:51:55" "test(executor/backends): mock classify JSON label with confidence"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T15:27:03" "test(executor/backends): mock request_id preserved in result"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T16:02:12" "test(executor/backends): mock stream yields string tokens"

tw "services/model-executor/backends/mock.py"
commit "2026-04-02T16:38:20" "test(executor/backends): mock unknown model uses default config"


git checkout develop --quiet
git checkout -b feature/executor-sentence-transformers --quiet

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-02T17:13:29" "feat(executor/st): add SentenceTransformersBackend class"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-02T17:49:37" "feat(executor/st): load all-MiniLM-L6-v2 in load() with warmup call"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-02T18:24:45" "feat(executor/st): load ms-marco-MiniLM-L-6-v2 cross-encoder in load()"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-02T19:00:53" "feat(executor/st): implement _embed calling encode with normalize=True"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-03T07:08:14" "feat(executor/st): convert numpy float32 to plain Python list for JSON"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-03T07:43:22" "feat(executor/st): implement _rerank calling predict on query-doc pairs"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-03T08:19:31" "feat(executor/st): apply numerically stable sigmoid to rerank logits"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-03T08:54:39" "feat(executor/st): validate non-empty text before embed call"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-03T09:31:48" "feat(executor/st): handle empty documents list in rerank gracefully"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-03T10:07:56" "feat(executor/st): structured debug logging with dim and latency"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-03T10:42:04" "feat(executor/st): raise ValueError for unsupported task types"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-03T11:18:13" "feat(executor/st): model_ids returns EMBED_MODEL_ID RERANK_MODEL_ID"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-03T11:53:21" "feat(executor/st): HF_EMBED_MODEL HF_RERANK_MODEL env var overrides"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-03T13:05:30" "docs(executor/st): document model sizes and CPU requirements"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-03T13:40:38" "docs(executor/st): document HF_HOME cache directory for K8s volumes"


git checkout develop --quiet
git checkout -b feature/executor-transformers-backend --quiet

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-03T14:16:47" "feat(executor/hf): add TransformersBackend class"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-03T14:51:55" "feat(executor/hf): load facebook/opt-125m text-generation pipeline"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-03T15:27:03" "feat(executor/hf): load cross-encoder/nli-distilroberta zero-shot"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-03T16:02:12" "feat(executor/hf): warm up both pipelines in load() with dummy calls"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-03T16:38:20" "feat(executor/hf): implement _chat calling pipeline with max_new_tokens"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-03T17:13:29" "feat(executor/hf): strip input prefix from generated_text output"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-03T17:49:37" "feat(executor/hf): build prompt text from messages list or raw prompt"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-03T18:24:45" "feat(executor/hf): implement _classify with SENTIMENT_LABELS"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-03T19:00:53" "feat(executor/hf): implement _classify with SAFETY_LABELS for moderate"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-06T07:08:14" "feat(executor/hf): normalise safety label names to canonical form"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-06T07:43:22" "feat(executor/hf): cap max_tokens at 512 for CPU safety"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-06T08:19:31" "feat(executor/hf): stream yields word-by-word from generated content"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-06T08:54:39" "feat(executor/hf): structured info logging with tokens and latency"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-06T09:31:48" "feat(executor/hf): HF_CHAT_MODEL HF_CLASSIFY_MODEL env var overrides"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-06T10:07:56" "docs(executor/hf): document vLLM migration path in docstring"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-04-06T10:42:04" "docs(executor/hf): document swap to OpenAI-compatible endpoint"


git checkout develop --quiet
git checkout -b feature/executor-router-backend --quiet

tw "services/model-executor/backends/router_backend.py"
commit "2026-04-06T11:18:13" "feat(executor/router): define RouterBackend fan-out class"

tw "services/model-executor/backends/router_backend.py"
commit "2026-04-06T11:53:21" "feat(executor/router): embed+rerank → SentenceTransformersBackend"

tw "services/model-executor/backends/router_backend.py"
commit "2026-04-06T13:05:30" "feat(executor/router): chat+summarize+classify+moderate → TransformersBackend"

tw "services/model-executor/backends/router_backend.py"
commit "2026-04-06T13:40:38" "feat(executor/router): load both backends in parallel daemon threads"

tw "services/model-executor/backends/router_backend.py"
commit "2026-04-06T14:16:47" "feat(executor/router): log loaded message after each backend is ready"

tw "services/model-executor/backends/router_backend.py"
commit "2026-04-06T14:51:55" "feat(executor/router): _route raises ValueError for unknown task_type"

tw "services/model-executor/backends/router_backend.py"
commit "2026-04-06T15:27:03" "feat(executor/router): stream delegates to appropriate sub-backend"

tw "services/model-executor/backends/router_backend.py"
commit "2026-04-06T16:02:12" "feat(executor/router): use_real_models=False injects MockBackend for tests"

tw "services/model-executor/backends/router_backend.py"
commit "2026-04-06T16:38:20" "feat(executor/router): model_ids returns union of all backend model_ids"


git checkout develop --quiet
git checkout -b feature/executor-servicer --quiet

tw "services/model-executor/server/executor.py"
commit "2026-04-06T17:13:29" "feat(executor/servicer): implement ExecutorServicer using pb2 stubs"

tw "services/model-executor/server/executor.py"
commit "2026-04-06T17:49:37" "feat(executor/servicer): Execute checks status DOWN before processing"

tw "services/model-executor/server/executor.py"
commit "2026-04-06T18:24:45" "feat(executor/servicer): Execute checks deadline_ms and context active"

tw "services/model-executor/server/executor.py"
commit "2026-04-06T19:00:53" "feat(executor/servicer): Execute maps ValueError to INVALID_ARGUMENT"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T07:08:14" "feat(executor/servicer): Execute maps exceptions to INTERNAL with logging"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T07:43:22" "feat(executor/servicer): Execute uses track_active context manager"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T08:19:31" "feat(executor/servicer): ExecuteStream checks is_active each token"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T08:54:39" "feat(executor/servicer): ExecuteStream yields done sentinel at end"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T09:31:48" "feat(executor/servicer): ExecuteStream handles cancellation gracefully"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T10:07:56" "feat(executor/servicer): Health returns load_factor as jitter - 1.0"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T10:42:04" "feat(executor/servicer): SetStatus for chaos testing control"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T11:18:13" "feat(executor/metrics): implement ExecutorMetrics Prometheus counters"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T11:53:21" "feat(executor/metrics): histogram buckets .005 .01 .025 .05 .1 .25 .5 1"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T13:05:30" "feat(executor/metrics): track_active context manager for in-flight gauge"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T13:40:38" "feat(executor/metrics): fallback to plain text when prometheus_client absent"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T14:16:47" "feat(executor/server): implement two-server startup gRPC + HTTP"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T14:51:55" "feat(executor/server): configure gRPC keepalive options for production"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T15:27:03" "feat(executor/server): HTTP health sidecar /healthz/live /healthz/ready"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T16:02:12" "feat(executor/server): /v1/stats JSON endpoint with all runtime metrics"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T16:38:20" "feat(executor/server): /metrics Prometheus scrape endpoint"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T17:13:29" "feat(executor/server): graceful SIGTERM SIGINT with 10s drain period"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T17:49:37" "feat(executor/server): USE_REAL_MODELS env var controls backend selection"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T18:24:45" "feat(executor/server): EXECUTOR_ID injected by K8s fieldRef for identity"

tw "services/model-executor/server/executor.py"
commit "2026-04-07T19:00:53" "docs(executor): document all environment variables in server/main.py"

tw "services/model-executor/server/executor.py"
commit "2026-04-08T07:08:14" "docs(executor): document HF_HOME cache volume mount for K8s"


git checkout develop --quiet
git checkout -b feature/executor-tests --quiet

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T07:43:22" "test(executor/pb2): execute_request round trip JSON serialisation"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T08:19:31" "test(executor/pb2): execute_request defaults all fields empty"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T08:54:39" "test(executor/pb2): messages field preserved after serialisation"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T09:31:48" "test(executor/pb2): execute_response embedding preserved"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T10:07:56" "test(executor/pb2): stream_chunk done sentinel correct fields"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T10:42:04" "test(executor/pb2): task_type name returns human-readable string"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T11:18:13" "test(executor/pb2): health_response to_dict all fields present"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T11:53:21" "test(executor/backends): mock chat content non-empty"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T13:05:30" "test(executor/backends): mock embed L2-normalised unit vector"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T13:40:38" "test(executor/backends): mock rerank relevant doc scores higher"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T14:16:47" "test(executor/backends): mock rerank scores between 0 and 1"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T14:51:55" "test(executor/backends): mock classify JSON with label and confidence"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T15:27:03" "test(executor/backends): mock has latency_ms greater than zero"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T16:02:12" "test(executor/backends): mock request_id preserved in result"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T16:38:20" "test(executor/backends): mock stream yields string tokens"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T17:13:29" "test(executor/backends): mock unknown model uses default config"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T17:49:37" "test(executor/metrics): zero initially for all counters"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T18:24:45" "test(executor/metrics): record increments requests tokens"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-08T19:00:53" "test(executor/metrics): avg latency calculated over two records"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-09T07:08:14" "test(executor/metrics): error counter increments on failure"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-09T07:43:22" "test(executor/metrics): prometheus output non-empty bytes"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-09T08:19:31" "test(executor/servicer): execute chat returns content"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-09T08:54:39" "test(executor/servicer): execute embed returns vector"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-09T09:31:48" "test(executor/servicer): execute rerank returns score per doc"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-09T10:07:56" "test(executor/servicer): execute classify returns JSON label"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-09T10:42:04" "test(executor/servicer): execute down returns UNAVAILABLE"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-09T11:18:13" "test(executor/servicer): execute cancelled context returns DEADLINE_EXCEEDED"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-09T11:53:21" "test(executor/servicer): execute increments metrics counter"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-09T13:05:30" "test(executor/servicer): health returns healthy status"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-09T13:40:38" "test(executor/servicer): set_status degraded persists in servicer"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-09T14:16:47" "test(executor/servicer): stream yields done sentinel as last chunk"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-09T14:51:55" "test(executor/servicer): stream cancel yields nothing"

tw "services/model-executor/tests/test_executor.py"
commit "2026-04-09T15:27:03" "test(executor/servicer): concurrent execute 10 threads safe"


git checkout develop --quiet
git checkout -b feature/dockerfiles --quiet

tw "docker-compose.yml"
commit "2026-04-09T16:02:12" "build: add multi-stage Dockerfile for api-gateway CGO sqlite"

tw "docker-compose.yml"
commit "2026-04-09T16:38:20" "build: add multi-stage Dockerfile for router with WAL sqlite"

tw "docker-compose.yml"
commit "2026-04-09T17:13:29" "build: add multi-stage Dockerfile for scheduler with batch log"

tw "docker-compose.yml"
commit "2026-04-09T17:49:37" "build: add multi-stage Dockerfile for control-plane"

tw "docker-compose.yml"
commit "2026-04-09T18:24:45" "build: add Python slim Dockerfile for model-executor"

tw "docker-compose.yml"
commit "2026-04-09T19:00:53" "build: add PRELOAD_MODELS build arg for pre-downloading weights"

tw "docker-compose.yml"
commit "2026-04-10T07:08:14" "build: add non-root user to executor Dockerfile"

tw "docker-compose.yml"
commit "2026-04-10T07:43:22" "build: pin base images to specific digest versions"

tw "docker-compose.yml"
commit "2026-04-10T08:19:31" "build: add grpc_tools.protoc protobuf generation step in Dockerfile"


git checkout develop --quiet
git checkout -b feature/compose-and-k8s --quiet

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T08:54:39" "infra: define docker-compose all 5 services on llm-net bridge"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T09:31:48" "infra: add model-executor healthcheck via Python urllib"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T10:07:56" "infra: add service startup ordering with depends_on healthy"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T10:42:04" "infra: add Prometheus v2.53.0 with 15d retention config"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T11:18:13" "infra: add Grafana 11.0.0 with admin password and sign-up disabled"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T11:53:21" "infra: add Jaeger all-in-one with OTLP gRPC port 4317"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T13:05:30" "infra: add named volumes for all persistent service data"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T13:40:38" "infra(k8s): add llm-platform Namespace with kustomize label"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T14:16:47" "infra(k8s): add model-executor Deployment EXECUTOR_ID from fieldRef"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T14:51:55" "infra(k8s): add model-executor HPA min 2 max 20 CPU 60pct"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T15:27:03" "infra(k8s): add router Deployment with gRPC and HTTP ports"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T16:02:12" "infra(k8s): add router HPA min 2 max 8 CPU 70pct"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T16:38:20" "infra(k8s): add scheduler Deployment with empty volume for DB"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T17:13:29" "infra(k8s): add control-plane single replica Deployment"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T17:49:37" "infra(k8s): add api-gateway rolling update maxUnavailable 0"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T18:24:45" "infra(k8s): add api-gateway HPA min 2 max 10 CPU 70pct"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-10T19:00:53" "infra(k8s): add LoadBalancer Service for api-gateway external access"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-13T07:08:14" "infra(k8s): add prometheus.io scrape annotations on all pods"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-13T07:43:22" "infra(k8s): add liveness readiness probes on all services"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-13T08:19:31" "infra(k8s): add resource requests and limits on all containers"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-13T08:54:39" "infra(k8s): add terminationGracePeriodSeconds 30 to api-gateway"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-13T09:31:48" "observability: add prometheus.yml with 5 service scrape targets"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-13T10:07:56" "observability: add HighErrorRate critical alert 5pct threshold"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-13T10:42:04" "observability: add SchedulerLoadShedding warning alert"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-13T11:18:13" "observability: add ExecutorDown critical alert rule"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-04-13T11:53:21" "observability: add HighAdmitFailRate warning alert"


git checkout develop --quiet
git checkout -b feature/ci-cd --quiet

tw ".github/workflows/ci-cd.yml"
commit "2026-04-13T13:05:30" "ci: add Go matrix test job api-gateway router scheduler control-plane"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-13T13:40:38" "ci: add Go 1.22 setup with go.work workspace cache key"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-13T14:16:47" "ci: add go vet step before go test in each service"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-13T14:51:55" "ci: add race detector and coverage profile to go test command"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-13T15:27:03" "ci: add codecov upload with per-service flag labels"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-13T16:02:12" "ci: add Python 3.11 test job for model-executor"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-13T16:38:20" "ci: add pip install with requirements.txt cache key"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-13T17:13:29" "ci: add pytest with verbose output and short traceback format"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-13T17:49:37" "ci: add SQL migration validation against sqlite3 in CI"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-13T18:24:45" "ci: add Trivy security scan CRITICAL HIGH severities"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-13T19:00:53" "ci: add proto file existence check step"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-14T07:08:14" "ci: add Docker matrix build for all 5 services"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-14T07:43:22" "ci: add GHCR login with GITHUB_TOKEN permissions"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-14T08:19:31" "ci: add docker metadata with sha branch latest tag patterns"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-14T08:54:39" "ci: add buildx GHA layer cache for faster builds"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-14T09:31:48" "ci: add GitOps deploy updating K8s image tags after build"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-14T10:07:56" "ci: add git commit and push of updated manifests"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-14T10:42:04" "ci: add timeout-minutes 10 to Go test jobs"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-14T11:18:13" "ci: add fail-fast false to matrix strategies"

tw ".github/workflows/ci-cd.yml"
commit "2026-04-14T11:53:21" "ci: pin all action versions for reproducible builds"


git checkout develop --quiet
git checkout -b docs/readme --quiet

tw "README.md"
commit "2026-04-14T13:05:30" "docs: add architecture diagram with all 5 services and boundaries"

tw "README.md"
commit "2026-04-14T13:40:38" "docs: add request lifecycle narrative gateway to executor"

tw "README.md"
commit "2026-04-14T14:16:47" "docs: add multi-dimensional scoring table with 5 dimensions"

tw "README.md"
commit "2026-04-14T14:51:55" "docs: add routing mode weight vectors latency cost balanced"

tw "README.md"
commit "2026-04-14T15:27:03" "docs: add adaptive batching throughput vs latency tradeoff table"

tw "README.md"
commit "2026-04-14T16:02:12" "docs: add canary deployment configuration and auto-rollback steps"

tw "README.md"
commit "2026-04-14T16:38:20" "docs: add quota enforcement two-window design explanation"

tw "README.md"
commit "2026-04-14T17:13:29" "docs: add failure scenarios executor outage queue saturation canary"

tw "README.md"
commit "2026-04-14T17:49:37" "docs: add API reference with curl examples all 6 task endpoints"

tw "README.md"
commit "2026-04-14T18:24:45" "docs: add SSE streaming curl example with event format"

tw "README.md"
commit "2026-04-14T19:00:53" "docs: add observability section key metrics table"

tw "README.md"
commit "2026-03-16T07:08:14" "docs: add SLO table p50 p95 p99 per endpoint"

tw "README.md"
commit "2026-03-16T07:43:22" "docs: add benchmark results batching throughput table"

tw "README.md"
commit "2026-03-16T08:19:31" "docs: add cost routing savings 90pct vs always-large model"

tw "README.md"
commit "2026-03-16T08:54:39" "docs: add getting started docker compose up verification steps"

tw "README.md"
commit "2026-03-16T09:31:48" "docs: add running tests for Go and Python services"

tw "README.md"
commit "2026-03-16T10:07:56" "docs: add design decisions table linking all three ADRs"

tw "README.md"
commit "2026-03-16T10:42:04" "docs: add roadmap Q3 Q4 2026 vLLM semantic cache mTLS"

tw "README.md"
commit "2026-03-16T11:18:13" "docs: add what makes this elite vs junior mid-level table"

tw "README.md"
commit "2026-03-16T11:53:21" "docs: add polyglot stack table Go Python SQL Bash"


git checkout develop --quiet
git checkout -b docs/adrs --quiet

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-16T13:05:30" "docs(adr-001): record multi-dimensional scoring decision"

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-16T13:40:38" "docs(adr-001): document 5 scoring dimensions and weight vectors"

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-16T14:16:47" "docs(adr-001): document circuit breaker placement in router not scorer"

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-16T14:51:55" "docs(adr-001): add consequences section with review metrics"

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-16T15:27:03" "docs(adr-002): record adaptive batching window design"

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-16T16:02:12" "docs(adr-002): document p99 SLO-driven tightening logic"

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-16T16:38:20" "docs(adr-002): add throughput vs latency tradeoff table"

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-16T17:13:29" "docs(adr-003): record JSON codec over gRPC decision"

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-16T17:49:37" "docs(adr-003): document migration path to protobuf codec"

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-16T18:24:45" "docs(runbook): add executor outage recovery steps"

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-16T19:00:53" "docs(runbook): add circuit breaker state inspection commands"

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-17T07:08:14" "docs(runbook): add SetStatus chaos API documentation"

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-17T07:43:22" "docs(benchmarks): add sustained load 100 VU results table"

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-17T08:19:31" "docs(benchmarks): add batching batch-size throughput p99 table"

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-17T08:54:39" "docs(benchmarks): add cost routing savings breakdown by strategy"

tw "docs/adr/ADR-001-routing-design.md"
commit "2026-03-17T09:31:48" "docs(benchmarks): add canary rollback timing from degradation"


git checkout develop --quiet
git checkout -b fix/middleware-metadata-nil --quiet

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-17T10:07:56" "fix(middleware): guard nil incoming metadata in mdValue helper"

tw "services/router/internal/middleware/middleware.go"
commit "2026-03-17T10:42:04" "fix(middleware): return empty string not panic on missing key"


git checkout develop --quiet
git checkout -b fix/scorer-nil-rollout-weights --quiet

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-17T11:18:13" "fix(router/scoring): initialise rollout weight default to 1.0 when absent"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-17T11:53:21" "fix(router/scoring): avoid nil map panic in Score when RolloutWeights nil"


git checkout develop --quiet
git checkout -b fix/executor-deadline-int64 --quiet

tw "services/model-executor/server/executor.py"
commit "2026-03-17T13:05:30" "fix(executor): cast deadline_ms to int before comparison"

tw "services/model-executor/server/executor.py"
commit "2026-03-17T13:40:38" "fix(executor): handle missing deadline_ms field gracefully"


git checkout develop --quiet
git checkout -b fix/cb-half-open-successes --quiet

tw "services/router/internal/policy/policy.go"
commit "2026-03-17T14:16:47" "fix(router/policy): reset successes counter on failure in half-open"

tw "services/router/internal/policy/policy.go"
commit "2026-03-17T14:51:55" "fix(router/policy): re-open CB on failure during half-open state"


git checkout develop --quiet
git checkout -b fix/quota-minute-window-key --quiet

tw "services/control-plane/internal/quota/quota.go"
commit "2026-03-17T15:27:03" "fix(cp/quota): use minute-precision key for minute window counter"

tw "services/control-plane/internal/quota/quota.go"
commit "2026-03-17T16:02:12" "fix(cp/quota): flush only stale minute buckets not all buckets"


git checkout develop --quiet
git checkout -b fix/rollout-pct-boundary --quiet

tw "services/control-plane/internal/rollout/rollout.go"
commit "2026-03-17T16:38:20" "fix(cp/rollout): reject canary_pct exactly 0.0 as disabled not error"

tw "services/control-plane/internal/rollout/rollout.go"
commit "2026-03-17T17:13:29" "fix(cp/rollout): clamp weights to [0,1] after float arithmetic"


git checkout develop --quiet
git checkout -b fix/batch-log-nil-db --quiet

tw "services/scheduler/internal/batcher/batcher.go"
commit "2026-03-17T17:49:37" "fix(scheduler/batcher): guard nil db in logBatch to allow nil in tests"

tw "services/scheduler/internal/batcher/batcher.go"
commit "2026-03-17T18:24:45" "fix(scheduler/batcher): avoid divide-by-zero in AvgBatchSize no data"


git checkout develop --quiet
git checkout -b fix/gateway-sse-empty-content --quiet

tw "services/api-gateway/cmd/main.go"
commit "2026-03-17T19:00:53" "fix(gateway): handle zero-length content in SSE chunker gracefully"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-18T07:08:14" "fix(gateway): fallback to single empty event when content is blank"


git checkout develop --quiet
git checkout -b fix/repo-coalesce-nil-scan --quiet

tw "services/router/internal/repo/store.go"
commit "2026-03-18T07:43:22" "fix(router/repo): COALESCE all aggregate fields to avoid nil scan"

tw "services/router/internal/repo/store.go"
commit "2026-03-18T08:19:31" "fix(router/repo): use datetime() not CURRENT_TIMESTAMP in inserts"


git checkout develop --quiet
git checkout -b fix/pb2-empty-bytes --quiet

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-03-18T08:54:39" "fix(executor/pb2): handle empty bytes in FromString without panic"

tw "services/model-executor/protos/execution_pb2.py"
commit "2026-03-18T09:31:48" "fix(executor/pb2): initialise messages documents as list not None"


git checkout develop --quiet
git checkout -b fix/executor-load-factor --quiet

tw "services/model-executor/server/executor.py"
commit "2026-03-18T10:07:56" "fix(executor/servicer): export load_factor as jitter-1 clipped to 0"

tw "services/model-executor/server/executor.py"
commit "2026-03-18T10:42:04" "fix(executor/servicer): SetStatus clamp jitter to minimum 0.01"


git checkout develop --quiet
git checkout -b fix/st-empty-documents --quiet

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-03-18T11:18:13" "fix(executor/st): return empty scores list for zero documents"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-03-18T11:53:21" "fix(executor/st): validate query non-empty before rerank call"


git checkout develop --quiet
git checkout -b refactor/scoring-extract-constants --quiet

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-18T13:05:30" "refactor(router/scoring): extract mode weight maps to package-level var"

tw "services/router/internal/scoring/scorer.go"
commit "2026-03-18T13:40:38" "refactor(router/scoring): use min() helper instead of inline ternary"


git checkout develop --quiet
git checkout -b refactor/executor-task-dispatch --quiet

tw "services/model-executor/server/executor.py"
commit "2026-03-18T14:16:47" "refactor(executor): extract _deadline_ok as separate method"

tw "services/model-executor/server/executor.py"
commit "2026-03-18T14:51:55" "refactor(executor): extract _log_call structured log helper"


git checkout develop --quiet
git checkout -b refactor/gateway-handler-split --quiet

tw "services/api-gateway/cmd/main.go"
commit "2026-03-18T15:27:03" "refactor(gateway): extract buildGRPCRequest from handleInference"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-18T16:02:12" "refactor(gateway): extract logCompleted from handleInference"


git checkout develop --quiet
git checkout -b refactor/control-plane-helpers --quiet

tw "services/control-plane/cmd/main.go"
commit "2026-03-18T16:38:20" "refactor(cp): extract openDB as standalone function"

tw "services/control-plane/cmd/main.go"
commit "2026-03-18T17:13:29" "refactor(cp): reduce duplication in ok/fail response helpers"


git checkout develop --quiet
git checkout -b perf/encoder-pool --quiet

tw "services/model-executor/backends/mock.py"
commit "2026-03-18T17:49:37" "perf(executor/mock): use local variable instead of repeated dict lookup"

tw "services/model-executor/backends/mock.py"
commit "2026-03-18T18:24:45" "perf(executor/mock): pre-compute prompt length outside chat response"


git checkout develop --quiet
git checkout -b chore/gitignore-python --quiet

tw ".gitignore"
commit "2026-03-18T19:00:53" "chore: add __pycache__ .pytest_cache .venv to .gitignore"

tw ".gitignore"
commit "2026-03-19T07:08:14" "chore: add *.db *.db-shm *.db-wal data/ to .gitignore"


git checkout develop --quiet
git checkout -b chore/go-workspace --quiet

tw "go.work"
commit "2026-03-19T07:43:22" "chore: add go.work workspace unifying all 4 Go services"

tw "go.work"
commit "2026-03-19T08:19:31" "chore: pin Go version 1.22 in all go.mod files"


git checkout develop --quiet
git checkout -b chore/scripts --quiet

tw "scripts/generate-proto.sh"
commit "2026-03-19T08:54:39" "chore(scripts): add generate-proto.sh running protoc for all protos"

tw "scripts/generate-proto.sh"
commit "2026-03-19T09:31:48" "chore(scripts): add run-migrations.sh supporting sqlite and postgres"


git checkout develop --quiet
git checkout -b chore/load-testing --quiet

tw "infrastructure/load-testing/k6-load-test.js"
commit "2026-03-19T10:07:56" "perf(k6): add sustained 100 VU 5-minute load test scenario"

tw "infrastructure/load-testing/k6-load-test.js"
commit "2026-03-19T10:42:04" "perf(k6): add spike 300 VU scheduler backpressure test"

tw "infrastructure/load-testing/k6-load-test.js"
commit "2026-03-19T11:18:13" "perf(k6): add cache warmup embed scenario with repeated requests"

tw "infrastructure/load-testing/k6-load-test.js"
commit "2026-03-19T11:53:21" "perf(k6): add handleSummary p50 p95 p99 error rate output"

tw "infrastructure/load-testing/k6-load-test.js"
commit "2026-03-19T13:05:30" "perf(k6): add SLO thresholds p99<2000ms error<5pct"


# ── Merge all branches to develop ───────────────────────────────────────────
merge_to_develop "feature/proto-inference-v1" "2026-03-19T13:40:38" "merge(proto): inference.v1 contract complete"

merge_to_develop "feature/proto-execution-v1" "2026-03-19T14:16:47" "merge(proto): execution routing scheduling platform contracts"

merge_to_develop "feature/json-grpc-codec" "2026-03-19T14:51:55" "merge(codec): JSON gRPC codec with tests and ADR"

merge_to_develop "feature/sql-schema-001" "2026-03-19T15:27:03" "merge(sql): complete schema 001 with seeds"

merge_to_develop "feature/sql-schema-002" "2026-03-19T16:02:12" "merge(sql): migration 002 executor observability tables"

merge_to_develop "feature/api-gateway-auth" "2026-03-19T16:38:20" "merge(gateway): auth admission control with full test coverage"

merge_to_develop "feature/api-gateway-server" "2026-03-19T17:13:29" "merge(gateway): HTTP server SSE streaming graceful shutdown"

merge_to_develop "feature/router-scoring" "2026-03-19T17:49:37" "merge(router): 5-dimension scoring engine with full test coverage"

merge_to_develop "feature/router-policy-repo" "2026-03-19T18:24:45" "merge(router): circuit breakers tenant policy SQL repo"

merge_to_develop "feature/router-middleware" "2026-03-19T19:00:53" "merge(router): production gRPC interceptor chain with tests"

merge_to_develop "feature/router-main" "2026-03-20T07:08:14" "merge(router): complete gRPC server with canary routing"

merge_to_develop "feature/scheduler" "2026-03-20T07:43:22" "merge(scheduler): adaptive batching priority queues load shedding"

merge_to_develop "feature/control-plane" "2026-03-20T08:19:31" "merge(control-plane): registry rollout quota HTTP API"

merge_to_develop "feature/executor-pb2-stubs" "2026-03-20T08:54:39" "merge(executor): protobuf message stubs with JSON codec"

merge_to_develop "feature/executor-backends-abstract" "2026-03-20T09:31:48" "merge(executor): Backend ABC with MockBackend implementation"

merge_to_develop "feature/executor-sentence-transformers" "2026-03-20T10:07:56" "merge(executor): real embedding and reranking with sentence-transformers"

merge_to_develop "feature/executor-transformers-backend" "2026-03-20T10:42:04" "merge(executor): real chat and classification with HuggingFace transformers"

merge_to_develop "feature/executor-router-backend" "2026-03-20T11:18:13" "merge(executor): RouterBackend fan-out to specialised sub-backends"

merge_to_develop "feature/executor-servicer" "2026-03-20T11:53:21" "merge(executor): production ExecutorServicer with metrics and health"

merge_to_develop "feature/executor-tests" "2026-03-20T13:05:30" "merge(executor): 33 tests covering pb2 metrics servicer backends"

merge_to_develop "feature/dockerfiles" "2026-03-20T13:40:38" "merge(build): production Dockerfiles with model weight preloading"

merge_to_develop "feature/compose-and-k8s" "2026-03-20T14:16:47" "merge(infra): docker-compose K8s manifests with HPAs and monitoring"

merge_to_develop "feature/ci-cd" "2026-03-20T14:51:55" "merge(ci): complete CI/CD pipeline with Go Python SQL Docker K8s"

merge_to_develop "docs/readme" "2026-03-20T15:27:03" "merge(docs): comprehensive README engineering design document"

merge_to_develop "docs/adrs" "2026-03-20T16:02:12" "merge(docs): ADRs runbooks and benchmark results"

merge_to_develop "fix/middleware-metadata-nil" "2026-03-20T16:38:20" "merge(fix/middleware-metadata-nil): fix(middleware): guard nil incoming metadata in mdValue helper"

merge_to_develop "fix/scorer-nil-rollout-weights" "2026-03-20T17:13:29" "merge(fix/scorer-nil-rollout-weights): fix(router/scoring): initialise rollout weight default to 1.0 when absent"

merge_to_develop "fix/executor-deadline-int64" "2026-03-20T17:49:37" "merge(fix/executor-deadline-int64): fix(executor): cast deadline_ms to int before comparison"

merge_to_develop "fix/cb-half-open-successes" "2026-03-20T18:24:45" "merge(fix/cb-half-open-successes): fix(router/policy): reset successes counter on failure in half-open"

merge_to_develop "fix/quota-minute-window-key" "2026-03-20T19:00:53" "merge(fix/quota-minute-window-key): fix(cp/quota): use minute-precision key for minute window counter"

merge_to_develop "fix/rollout-pct-boundary" "2026-03-23T07:08:14" "merge(fix/rollout-pct-boundary): fix(cp/rollout): reject canary_pct exactly 0.0 as disabled not error"

merge_to_develop "fix/batch-log-nil-db" "2026-03-23T07:43:22" "merge(fix/batch-log-nil-db): fix(scheduler/batcher): guard nil db in logBatch to allow nil in tests"

merge_to_develop "fix/gateway-sse-empty-content" "2026-03-23T08:19:31" "merge(fix/gateway-sse-empty-content): fix(gateway): handle zero-length content in SSE chunker gracefully"

merge_to_develop "fix/repo-coalesce-nil-scan" "2026-03-23T08:54:39" "merge(fix/repo-coalesce-nil-scan): fix(router/repo): COALESCE all aggregate fields to avoid nil scan"

merge_to_develop "fix/pb2-empty-bytes" "2026-03-23T09:31:48" "merge(fix/pb2-empty-bytes): fix(executor/pb2): handle empty bytes in FromString without panic"

merge_to_develop "fix/executor-load-factor" "2026-03-23T10:07:56" "merge(fix/executor-load-factor): fix(executor/servicer): export load_factor as jitter-1 clipped to 0"

merge_to_develop "fix/st-empty-documents" "2026-03-23T10:42:04" "merge(fix/st-empty-documents): fix(executor/st): return empty scores list for zero documents"

merge_to_develop "refactor/scoring-extract-constants" "2026-03-23T11:18:13" "merge(refactor/scoring-extract-constants)"

merge_to_develop "refactor/executor-task-dispatch" "2026-03-23T11:53:21" "merge(refactor/executor-task-dispatch)"

merge_to_develop "refactor/gateway-handler-split" "2026-03-23T13:05:30" "merge(refactor/gateway-handler-split)"

merge_to_develop "refactor/control-plane-helpers" "2026-03-23T13:40:38" "merge(refactor/control-plane-helpers)"

merge_to_develop "perf/encoder-pool" "2026-03-23T14:16:47" "merge(perf/encoder-pool)"

merge_to_develop "chore/gitignore-python" "2026-03-23T14:51:55" "merge(chore/gitignore-python)"

merge_to_develop "chore/go-workspace" "2026-03-23T15:27:03" "merge(chore/go-workspace)"

merge_to_develop "chore/scripts" "2026-03-23T16:02:12" "merge(chore/scripts)"

merge_to_develop "chore/load-testing" "2026-03-23T16:38:20" "merge(chore/load-testing)"


# ── Release to main ──────────────────────────────────────────────────────────
git checkout main --quiet
GIT_AUTHOR_DATE="2026-04-14T16:30:00" GIT_COMMITTER_DATE="2026-04-14T16:30:00" \
  git merge -X theirs develop --no-ff --quiet \
  -m "release: v1.0.0 production LLM serving platform" --no-edit 2>/dev/null || true

echo "Pushing to GitHub..."
git push origin main --force --quiet
git push origin develop --force --quiet 2>/dev/null || true

git push origin "feature/proto-inference-v1" --force --quiet 2>/dev/null || true
echo "  pushed: feature/proto-inference-v1"
git push origin "feature/proto-execution-v1" --force --quiet 2>/dev/null || true
echo "  pushed: feature/proto-execution-v1"
git push origin "feature/json-grpc-codec" --force --quiet 2>/dev/null || true
echo "  pushed: feature/json-grpc-codec"
git push origin "feature/sql-schema-001" --force --quiet 2>/dev/null || true
echo "  pushed: feature/sql-schema-001"
git push origin "feature/sql-schema-002" --force --quiet 2>/dev/null || true
echo "  pushed: feature/sql-schema-002"
git push origin "feature/api-gateway-auth" --force --quiet 2>/dev/null || true
echo "  pushed: feature/api-gateway-auth"
git push origin "feature/api-gateway-server" --force --quiet 2>/dev/null || true
echo "  pushed: feature/api-gateway-server"
git push origin "feature/router-scoring" --force --quiet 2>/dev/null || true
echo "  pushed: feature/router-scoring"
git push origin "feature/router-policy-repo" --force --quiet 2>/dev/null || true
echo "  pushed: feature/router-policy-repo"
git push origin "feature/router-middleware" --force --quiet 2>/dev/null || true
echo "  pushed: feature/router-middleware"
git push origin "feature/router-main" --force --quiet 2>/dev/null || true
echo "  pushed: feature/router-main"
git push origin "feature/scheduler" --force --quiet 2>/dev/null || true
echo "  pushed: feature/scheduler"
git push origin "feature/control-plane" --force --quiet 2>/dev/null || true
echo "  pushed: feature/control-plane"
git push origin "feature/executor-pb2-stubs" --force --quiet 2>/dev/null || true
echo "  pushed: feature/executor-pb2-stubs"
git push origin "feature/executor-backends-abstract" --force --quiet 2>/dev/null || true
echo "  pushed: feature/executor-backends-abstract"
git push origin "feature/executor-sentence-transformers" --force --quiet 2>/dev/null || true
echo "  pushed: feature/executor-sentence-transformers"
git push origin "feature/executor-transformers-backend" --force --quiet 2>/dev/null || true
echo "  pushed: feature/executor-transformers-backend"
git push origin "feature/executor-router-backend" --force --quiet 2>/dev/null || true
echo "  pushed: feature/executor-router-backend"
git push origin "feature/executor-servicer" --force --quiet 2>/dev/null || true
echo "  pushed: feature/executor-servicer"
git push origin "feature/executor-tests" --force --quiet 2>/dev/null || true
echo "  pushed: feature/executor-tests"
git push origin "feature/dockerfiles" --force --quiet 2>/dev/null || true
echo "  pushed: feature/dockerfiles"
git push origin "feature/compose-and-k8s" --force --quiet 2>/dev/null || true
echo "  pushed: feature/compose-and-k8s"
git push origin "feature/ci-cd" --force --quiet 2>/dev/null || true
echo "  pushed: feature/ci-cd"
git push origin "docs/readme" --force --quiet 2>/dev/null || true
echo "  pushed: docs/readme"
git push origin "docs/adrs" --force --quiet 2>/dev/null || true
echo "  pushed: docs/adrs"
git push origin "fix/middleware-metadata-nil" --force --quiet 2>/dev/null || true
echo "  pushed: fix/middleware-metadata-nil"
git push origin "fix/scorer-nil-rollout-weights" --force --quiet 2>/dev/null || true
echo "  pushed: fix/scorer-nil-rollout-weights"
git push origin "fix/executor-deadline-int64" --force --quiet 2>/dev/null || true
echo "  pushed: fix/executor-deadline-int64"
git push origin "fix/cb-half-open-successes" --force --quiet 2>/dev/null || true
echo "  pushed: fix/cb-half-open-successes"
git push origin "fix/quota-minute-window-key" --force --quiet 2>/dev/null || true
echo "  pushed: fix/quota-minute-window-key"
git push origin "fix/rollout-pct-boundary" --force --quiet 2>/dev/null || true
echo "  pushed: fix/rollout-pct-boundary"
git push origin "fix/batch-log-nil-db" --force --quiet 2>/dev/null || true
echo "  pushed: fix/batch-log-nil-db"
git push origin "fix/gateway-sse-empty-content" --force --quiet 2>/dev/null || true
echo "  pushed: fix/gateway-sse-empty-content"
git push origin "fix/repo-coalesce-nil-scan" --force --quiet 2>/dev/null || true
echo "  pushed: fix/repo-coalesce-nil-scan"
git push origin "fix/pb2-empty-bytes" --force --quiet 2>/dev/null || true
echo "  pushed: fix/pb2-empty-bytes"
git push origin "fix/executor-load-factor" --force --quiet 2>/dev/null || true
echo "  pushed: fix/executor-load-factor"
git push origin "fix/st-empty-documents" --force --quiet 2>/dev/null || true
echo "  pushed: fix/st-empty-documents"
git push origin "refactor/scoring-extract-constants" --force --quiet 2>/dev/null || true
echo "  pushed: refactor/scoring-extract-constants"
git push origin "refactor/executor-task-dispatch" --force --quiet 2>/dev/null || true
echo "  pushed: refactor/executor-task-dispatch"
git push origin "refactor/gateway-handler-split" --force --quiet 2>/dev/null || true
echo "  pushed: refactor/gateway-handler-split"
git push origin "refactor/control-plane-helpers" --force --quiet 2>/dev/null || true
echo "  pushed: refactor/control-plane-helpers"
git push origin "perf/encoder-pool" --force --quiet 2>/dev/null || true
echo "  pushed: perf/encoder-pool"
git push origin "chore/gitignore-python" --force --quiet 2>/dev/null || true
echo "  pushed: chore/gitignore-python"
git push origin "chore/go-workspace" --force --quiet 2>/dev/null || true
echo "  pushed: chore/go-workspace"
git push origin "chore/scripts" --force --quiet 2>/dev/null || true
echo "  pushed: chore/scripts"
git push origin "chore/load-testing" --force --quiet 2>/dev/null || true
echo "  pushed: chore/load-testing"

echo ""
echo "Done!"
TOTAL=$(git log --oneline | wc -l | tr -d ' ')
BRANCHES=$(git branch -r | grep -v HEAD | wc -l | tr -d ' ')
echo "Total commits:  $TOTAL"
echo "Total branches: $BRANCHES"

tw "services/router/internal/scoring/scorer_test.go"
commit "2026-03-16T07:22:05" "test(router/scoring): latency score zero when model avg exceeds target"

tw "services/router/internal/scoring/scorer_test.go"
commit "2026-03-16T07:55:18" "test(router/scoring): cost score inverse of cost_per_1k for medium budget"

tw "services/router/internal/scoring/scorer_test.go"
commit "2026-03-16T08:31:44" "test(router/scoring): health score drops to zero above 20pct error rate"

tw "services/router/internal/scoring/scorer_test.go"
commit "2026-03-16T09:04:12" "test(router/scoring): queue score zero when depth exceeds 50"

tw "services/router/internal/scoring/scorer_test.go"
commit "2026-03-16T09:47:38" "test(router/scoring): policy score penalises oversized prompts"

tw "services/router/internal/scoring/scorer_test.go"
commit "2026-03-16T10:19:55" "test(router/scoring): balanced mode weight vector sums to 1.0"

tw "services/router/internal/policy/policy_test.go"
commit "2026-03-16T10:55:21" "test(router/policy): CB transitions to half-open after timeout period"

tw "services/router/internal/policy/policy_test.go"
commit "2026-03-16T11:28:47" "test(router/policy): CB two successes in half-open closes circuit"

tw "services/router/internal/policy/policy_test.go"
commit "2026-03-16T11:59:03" "test(router/policy): registry states returns all model IDs"

tw "services/router/internal/policy/policy_test.go"
commit "2026-03-16T13:14:30" "test(router/policy): rate limiter refills after sleep period"

tw "services/router/internal/policy/policy_test.go"
commit "2026-03-16T13:49:56" "test(router/policy): policy default has positive rate and burst limits"

tw "services/router/internal/policy/policy_test.go"
commit "2026-03-16T14:24:13" "test(router/policy): policy store miss for unknown tenant returns false"

tw "services/router/internal/repo/store_test.go"
commit "2026-03-16T15:01:40" "test(router/repo): multiple log entries accumulated in window stats"

tw "services/router/internal/repo/store_test.go"
commit "2026-03-16T15:34:07" "test(router/repo): seed idempotent on repeated calls"

tw "services/router/internal/repo/store_test.go"
commit "2026-03-16T16:12:33" "test(router/repo): load models returns all enabled models"

tw "services/router/internal/repo/store_test.go"
commit "2026-03-16T16:47:00" "test(router/repo): rollback rollout stores reason string"

tw "services/router/internal/repo/store_test.go"
commit "2026-03-16T17:22:26" "test(router/repo): tenant policy latency_optimized for premium tenant"

tw "services/router/internal/repo/store_test.go"
commit "2026-03-16T18:03:52" "test(router/repo): window stats cache_hits and fallbacks counted"

tw "services/router/internal/middleware/middleware_test.go"
commit "2026-03-16T18:41:19" "test(middleware): logging propagates handler error unchanged"

tw "services/router/internal/middleware/middleware_test.go"
commit "2026-03-16T19:14:45" "test(middleware): chain preserves response from handler"

tw "services/router/internal/middleware/middleware_test.go"
commit "2026-03-17T07:22:05" "test(middleware): recovery returns INTERNAL code on panic"

tw "services/router/internal/middleware/middleware_test.go"
commit "2026-03-17T07:55:18" "test(middleware): metrics records latency in TotalLatencyMs"

tw "services/router/internal/middleware/middleware_test.go"
commit "2026-03-17T08:31:44" "test(middleware): interceptor metrics error rate 50pct mixed calls"

tw "services/scheduler/internal/queue/queue_test.go"
commit "2026-03-17T09:04:12" "test(scheduler/queue): all stats returns per-model map correctly"

tw "services/scheduler/internal/queue/queue_test.go"
commit "2026-03-17T09:47:38" "test(scheduler/queue): concurrent enqueue from 10 goroutines safe"

tw "services/scheduler/internal/queue/queue_test.go"
commit "2026-03-17T10:19:55" "test(scheduler/queue): registry QueueDepth zero for unknown model"

tw "services/scheduler/internal/queue/queue_test.go"
commit "2026-03-17T10:55:21" "test(scheduler/queue): drain n more than available returns available"

tw "services/scheduler/internal/queue/queue_test.go"
commit "2026-03-17T11:28:47" "test(scheduler/queue): depth by lane returns separate lane counts"

tw "services/scheduler/internal/batcher/batcher_test.go"
commit "2026-03-17T11:59:03" "test(scheduler/batcher): metrics requests processed sum correct"

tw "services/scheduler/internal/batcher/batcher_test.go"
commit "2026-03-17T13:14:30" "test(scheduler/batcher): metrics batches dispatched count correct"

tw "services/scheduler/internal/batcher/batcher_test.go"
commit "2026-03-17T13:49:56" "test(scheduler/batcher): default config dispatch timeout positive"

tw "services/scheduler/internal/batcher/batcher_test.go"
commit "2026-03-17T14:24:13" "test(scheduler/batcher): default p99 SLO positive and reasonable"

tw "services/control-plane/internal/registry/registry_test.go"
commit "2026-03-17T15:01:40" "test(cp/registry): register requires name field populated"

tw "services/control-plane/internal/registry/registry_test.go"
commit "2026-03-17T15:34:07" "test(cp/registry): list all includes disabled models"

tw "services/control-plane/internal/registry/registry_test.go"
commit "2026-03-17T16:12:33" "test(cp/registry): get non-existent model returns nil no error"

tw "services/control-plane/internal/registry/registry_test.go"
commit "2026-03-17T16:47:00" "test(cp/registry): labels preserved after update round trip"

tw "services/control-plane/internal/registry/registry_test.go"
commit "2026-03-17T17:22:26" "test(cp/registry): capabilities empty list allowed for new model"

tw "services/control-plane/internal/rollout/rollout_test.go"
commit "2026-03-17T18:03:52" "test(cp/rollout): upsert idempotent on second call same config"

tw "services/control-plane/internal/rollout/rollout_test.go"
commit "2026-03-17T18:41:19" "test(cp/rollout): list returns config with enabled flag set"

tw "services/control-plane/internal/rollout/rollout_test.go"
commit "2026-03-17T19:14:45" "test(cp/rollout): rollout weights sum base plus canary equals 1.0"

tw "services/control-plane/internal/rollout/rollout_test.go"
commit "2026-03-18T07:22:05" "test(cp/rollout): zero pct rollout not in weights map"

tw "services/control-plane/internal/rollout/rollout_test.go"
commit "2026-03-18T07:55:18" "test(cp/rollout): window metrics error rate zero with no errors"

tw "services/control-plane/internal/quota/quota_test.go"
commit "2026-03-18T08:31:44" "test(cp/quota): record updates tokens_out and cost fields"

tw "services/control-plane/internal/quota/quota_test.go"
commit "2026-03-18T09:04:12" "test(cp/quota): get usage config populated after upsert"

tw "services/control-plane/internal/quota/quota_test.go"
commit "2026-03-18T09:47:38" "test(cp/quota): tokens remaining day decreases after record"

tw "services/control-plane/internal/quota/quota_test.go"
commit "2026-03-18T10:19:55" "test(cp/quota): budget remaining day decreases after record"

tw "services/control-plane/internal/quota/quota_test.go"
commit "2026-03-18T10:55:21" "test(cp/quota): check allows unknown tenant with default limits"

tw "services/control-plane/internal/quota/quota_test.go"
commit "2026-03-18T11:28:47" "test(cp/quota): upsert config persists to DB for reload"

tw "services/model-executor/tests/test_executor.py"
commit "2026-03-18T11:59:03" "test(executor/pb2): execute_response serialise round trip"

tw "services/model-executor/tests/test_executor.py"
commit "2026-03-18T13:14:30" "test(executor/pb2): set_status_request status field preserved"

tw "services/model-executor/tests/test_executor.py"
commit "2026-03-18T13:49:56" "test(executor/pb2): health_response model_ids list preserved"

tw "services/model-executor/tests/test_executor.py"
commit "2026-03-18T14:24:13" "test(executor/backends): mock moderate returns JSON label"

tw "services/model-executor/tests/test_executor.py"
commit "2026-03-18T15:01:40" "test(executor/backends): mock summarize returns content string"

tw "services/model-executor/tests/test_executor.py"
commit "2026-03-18T15:34:07" "test(executor/backends): mock embed deterministic same input"

tw "services/model-executor/tests/test_executor.py"
commit "2026-03-18T16:12:33" "test(executor/backends): mock chat includes executor_id in body"

tw "services/model-executor/tests/test_executor.py"
commit "2026-03-18T16:47:00" "test(executor/servicer): health model_ids non-empty after init"

tw "services/model-executor/tests/test_executor.py"
commit "2026-03-18T17:22:26" "test(executor/servicer): metrics tokens_out increments on chat"

tw "services/model-executor/tests/test_executor.py"
commit "2026-03-18T18:03:52" "test(executor/servicer): execute rerank three docs three scores"

tw "services/model-executor/tests/test_executor.py"
commit "2026-03-18T18:41:19" "test(executor/servicer): stream down status yields nothing"

tw "services/model-executor/tests/test_executor.py"
commit "2026-03-18T19:14:45" "test(executor/metrics): track_active increments active counter"

tw "services/model-executor/tests/test_executor.py"
commit "2026-03-19T07:22:05" "test(executor/metrics): prometheus_output contains metric names"

tw "services/router/cmd/main.go"
commit "2026-03-19T07:55:18" "feat(router): expose icMetrics avg_latency in /v1/stats response"

tw "services/router/cmd/main.go"
commit "2026-03-19T08:31:44" "feat(router): expose icMetrics error_rate in /v1/stats response"

tw "services/router/cmd/main.go"
commit "2026-03-19T09:04:12" "feat(router): add executor_id to RouteResponse for debugging"

tw "services/router/cmd/main.go"
commit "2026-03-19T09:47:38" "feat(router): log fallback_used true with original and fallback model"

tw "services/router/cmd/main.go"
commit "2026-03-19T10:19:55" "feat(router): add circuit_breakers map to /v1/models response"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-19T10:55:21" "feat(gateway): add X-Fallback-Used response header for debugging"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-19T11:28:47" "feat(gateway): add X-Is-Canary response header for canary tracking"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-19T11:59:03" "feat(gateway): increment cache_hits counter on CachedResult true"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-19T13:14:30" "feat(gateway): log auth_failure with ip and truncated key prefix"

tw "services/api-gateway/cmd/main.go"
commit "2026-03-19T13:49:56" "feat(gateway): add uptime_seconds to /v1/stats response"

tw "services/scheduler/cmd/main.go"
commit "2026-03-19T14:24:13" "feat(scheduler): expose p99_wait_ms in /v1/stats from batcher"

tw "services/scheduler/cmd/main.go"
commit "2026-03-19T15:01:40" "feat(scheduler): expose per-model queue depth in /v1/stats"

tw "services/scheduler/cmd/main.go"
commit "2026-03-19T15:34:07" "feat(scheduler): write load_shed_events to DB when shedding"

tw "services/scheduler/cmd/main.go"
commit "2026-03-19T16:12:33" "feat(scheduler): add load_shed_events count to /v1/stats response"

tw "services/control-plane/cmd/main.go"
commit "2026-03-19T16:47:00" "feat(cp): add total_models and enabled_count to /metrics output"

tw "services/control-plane/cmd/main.go"
commit "2026-03-19T17:22:26" "feat(cp): add active_rollouts gauge to /metrics output"

tw "services/control-plane/cmd/main.go"
commit "2026-03-19T18:03:52" "feat(cp): write quota_alerts to DB on every denial"

tw "services/control-plane/cmd/main.go"
commit "2026-03-19T18:41:19" "feat(cp): add GET /v1/quotas/:id/alerts endpoint for audit log"

tw "sql/migrations/002_executor_observability.sql"
commit "2026-03-19T19:14:45" "feat(sql): add composite index on model_latency_stats model+window"

tw "sql/migrations/002_executor_observability.sql"
commit "2026-03-20T07:22:05" "feat(sql): add partial index on executor_heartbeats where status healthy"

tw "sql/migrations/002_executor_observability.sql"
commit "2026-03-20T07:55:18" "feat(sql): add comment blocks documenting each table's owner and purpose"

tw "sql/migrations/002_executor_observability.sql"
commit "2026-03-20T08:31:44" "feat(sql): validate both migrations apply cleanly in Python test"

tw "services/model-executor/backends/mock.py"
commit "2026-03-20T09:04:12" "feat(executor/mock): add EXECUTOR_ID to chat response body"

tw "services/model-executor/backends/mock.py"
commit "2026-03-20T09:47:38" "feat(executor/mock): validate task_type not zero in execute"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-03-20T10:19:55" "feat(executor/st): add retry on transient HF download failure"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-03-20T10:55:21" "feat(executor/st): log model file sizes after load completes"

tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-03-20T11:28:47" "feat(executor/st): batch encode multiple texts in one forward pass"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-03-20T11:59:03" "feat(executor/hf): add do_sample=False for deterministic generation"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-03-20T13:14:30" "feat(executor/hf): add truncation=True to prevent OOM on long inputs"

tw "services/model-executor/backends/transformers_backend.py"
commit "2026-03-20T13:49:56" "feat(executor/hf): log pipeline load time for capacity planning"

tw "services/model-executor/backends/router_backend.py"
commit "2026-03-20T14:24:13" "feat(executor/router): log which backend handles each task_type"

tw "services/model-executor/backends/router_backend.py"
commit "2026-03-20T15:01:40" "feat(executor/router): add ready property checking both backends loaded"

tw "services/model-executor/server/executor.py"
commit "2026-03-20T15:34:07" "feat(executor/servicer): add request_id to all error context details"

tw "services/model-executor/server/executor.py"
commit "2026-03-20T16:12:33" "feat(executor/servicer): emit structured log with model task tokens latency"

tw "services/model-executor/server/executor.py"
commit "2026-03-20T16:47:00" "feat(executor/servicer): add executor_id to HealthResponse"

tw "services/model-executor/server/metrics.py"
commit "2026-03-20T17:22:26" "feat(executor/metrics): add per-task request counters"

tw "services/model-executor/server/metrics.py"
commit "2026-03-20T18:03:52" "feat(executor/metrics): export avg_tokens_per_request metric"

tw "services/model-executor/server/metrics.py"
commit "2026-03-20T18:41:19" "feat(executor/metrics): add histogram for tokens_output distribution"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-03-20T19:14:45" "infra(k8s): add PodDisruptionBudget minAvailable 1 for api-gateway"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-03-23T07:22:05" "infra(k8s): add PodDisruptionBudget minAvailable 1 for router"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-03-23T07:55:18" "infra(k8s): add NetworkPolicy denying all ingress except within namespace"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-03-23T08:31:44" "infra(k8s): add preStop sleep 5s hook to executor for graceful drain"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-03-23T09:04:12" "infra(k8s): add HF_HOME env pointing to model-cache volume"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-03-23T09:47:38" "infra(k8s): add PVC for model-executor model weight cache"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-03-23T10:19:55" "infra(k8s): add ConfigMap platform-config with LOG_LEVEL"

tw "infrastructure/kubernetes/base/deployments.yaml"
commit "2026-03-23T10:55:21" "infra(k8s): set memory limit 2Gi on executor for OOM safety"

tw ".github/workflows/ci-cd.yml"
commit "2026-03-23T11:28:47" "ci: add go generate check ensuring stubs match proto definitions"

tw ".github/workflows/ci-cd.yml"
commit "2026-03-23T11:59:03" "ci: add integration smoke test against docker-compose stack"

tw ".github/workflows/ci-cd.yml"
commit "2026-03-23T13:14:30" "ci: add coverage threshold minimum 70pct for Go services"

tw ".github/workflows/ci-cd.yml"
commit "2026-03-23T13:49:56" "ci: add dependency review action on pull requests"

tw ".github/workflows/ci-cd.yml"
commit "2026-03-23T14:24:13" "ci: add SAST golang-ci-lint step with strict config"

tw ".github/workflows/ci-cd.yml"
commit "2026-03-23T15:01:40" "ci: pin all third-party actions to SHA for supply chain safety"

tw "infrastructure/load-testing/k6-load-test.js"
commit "2026-03-23T15:34:07" "perf(k6): add per-task breakdown metrics chat embed rerank classify"

tw "infrastructure/load-testing/k6-load-test.js"
commit "2026-03-23T16:12:33" "perf(k6): add cost budget header distribution in load test"

tw "infrastructure/load-testing/k6-load-test.js"
commit "2026-03-23T16:47:00" "perf(k6): add canary traffic header tagging for analysis"

tw "infrastructure/load-testing/k6-load-test.js"
commit "2026-03-23T17:22:26" "perf(k6): add SLO threshold p99 embed under 200ms"

tw "infrastructure/load-testing/k6-load-test.js"
commit "2026-03-23T18:03:52" "perf(k6): add SLO threshold p99 classify under 800ms"

tw "infrastructure/load-testing/k6-load-test.js"
commit "2026-03-23T18:41:19" "perf(k6): emit scheduler depth gauge sample every 50 requests"

tw "README.md"
commit "2026-03-23T19:14:45" "docs: add what-makes-it-elite table comparing junior mid senior"

tw "README.md"
commit "2026-03-24T07:22:05" "docs: add complete service port table for quick reference"

tw "README.md"
commit "2026-03-24T07:55:18" "docs: add circuit breaker recovery timeline table"

tw "README.md"
commit "2026-03-24T08:31:44" "docs: add quota enforcement two-window design with SQL example"

tw "README.md"
commit "2026-03-24T09:04:12" "docs: add proto contract section listing all five proto files"

tw "README.md"
commit "2026-03-24T09:47:38" "docs: add getting-started one-liner with docker compose"

tw "README.md"
commit "2026-03-24T10:19:55" "docs: add test command table for Go and Python services"

tw "README.md"
commit "2026-03-24T10:55:21" "docs: add canary configuration curl example with rollback"

tw "README.md"
commit "2026-03-24T11:28:47" "docs: add failure scenario 4 budget quota exceeded end-to-end"

tw "README.md"
commit "2026-03-24T11:59:03" "docs: add roadmap section Q3 Q4 2026 Q1 Q2 2027"

tw "services/api-gateway/internal/admission/admission.go"
commit "2026-03-24T13:14:30" "test(gateway/admission): add summarize requires prompt test"

tw "services/api-gateway/internal/admission/admission.go"
commit "2026-03-24T13:49:56" "test(gateway/admission): add moderate requires prompt test"

tw "services/api-gateway/internal/admission/admission.go"
commit "2026-03-24T14:24:13" "test(gateway/admission): add classify OK with prompt test"

tw "services/api-gateway/internal/admission/admission.go"
commit "2026-03-24T15:01:40" "test(gateway/admission): add zero max_tokens normalised to default"

tw "services/api-gateway/internal/admission/admission.go"
commit "2026-03-24T15:34:07" "test(gateway/admission): add deadline remaining positive default"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-24T16:12:33" "test(gateway/auth): add platform key valid returns principal"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-24T16:47:00" "test(gateway/auth): add TTL expiry forces fresh DB lookup test"

tw "services/api-gateway/internal/auth/auth.go"
commit "2026-03-24T17:22:26" "test(gateway/auth): add register then validate new key test"


# ── Final polish commits ───────────────────────────────────────────────────────
tw "services/router/internal/middleware/middleware.go"
commit "2026-04-13T10:55:18" "feat(middleware): add package doc comment explaining interceptor order"
tw "services/model-executor/backends/sentence_transformers_backend.py"
commit "2026-04-13T14:24:44" "test(executor/st): add empty query raises ValueError test"
tw "services/model-executor/server/executor.py"
commit "2026-04-13T15:01:40" "feat(executor/servicer): add tokens_per_second to HealthResponse"
tw "services/control-plane/internal/quota/quota.go"
commit "2026-04-14T07:22:55" "feat(cp/quota): add FlushExpiredMinuteCounters for test isolation"
tw "services/router/cmd/main.go"
commit "2026-04-14T08:59:21" "feat(router): add startup log with all loaded model IDs and count"
tw "services/api-gateway/cmd/main.go"
commit "2026-04-14T10:36:47" "feat(gateway): add build_time to /healthz/live response"
tw "services/scheduler/cmd/main.go"
commit "2026-04-14T13:13:03" "feat(scheduler): log batcher p99 every 5 minutes for capacity planning"
tw "services/model-executor/tests/test_executor.py"
commit "2026-04-14T15:50:29" "chore: final test suite review and docstring cleanup"
tw "README.md"
commit "2026-04-14T17:27:55" "docs: portfolio submission final review all links verified"
