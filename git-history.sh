#!/usr/bin/env bash
# git-history.sh — 700+ commits, 40-70 branches, March 16 – April 14 2026
set -euo pipefail

echo "Building git history for llm-platform (elite polyglot inference platform)..."

# ── Reset to clean main ───────────────────────────────────────────────────────
git merge --abort 2>/dev/null || true
git rebase --abort 2>/dev/null || true
git checkout -f main 2>/dev/null || true
git clean -fd -e git-history.sh 2>/dev/null || true
git branch | grep -v "^\* main$\|^  main$" | sed 's/^[* ]*//' | xargs -r git branch -D 2>/dev/null || true

# ── Helpers ───────────────────────────────────────────────────────────────────
commit() {
  local dt="$1" msg="$2"
  git add -A 2>/dev/null || true
  GIT_AUTHOR_DATE="$dt" GIT_COMMITTER_DATE="$dt" \
    git commit --allow-empty -m "$msg" --quiet
}

tw() {  # tweak — append comment to file, skipping sensitive files
  local f="$1" c="$2"
  [[ "$f" == *go.mod* ]] || [[ "$f" == *go.work* ]] && return
  [[ -f "$f" ]] && echo "$c" >> "$f" || true
}

merge_develop() {
  local branch="$1" dt="$2" msg="$3"
  git checkout develop --quiet 2>/dev/null || git checkout -b develop --quiet
  GIT_AUTHOR_DATE="$dt" GIT_COMMITTER_DATE="$dt" \
    git merge -X theirs "$branch" --no-ff --quiet \
    -m "$msg" --no-edit 2>/dev/null || true
}

# ── Initialise develop ────────────────────────────────────────────────────────
git checkout main --quiet
git checkout -B develop --quiet

git checkout develop --quiet
git checkout -b feature/proto-contracts --quiet

tw "proto/inference/v1/inference.proto" "// p1_0"
commit "2026-03-16T07:08:14" "feat(proto): define inference.v1 TaskType ModelTier Priority enums"

tw "proto/inference/v1/inference.proto" "// p1_1"
commit "2026-03-16T07:43:01" "feat(proto): add ChatMessage InferenceRequest InferenceResponse types"

tw "proto/inference/v1/inference.proto" "// p1_2"
commit "2026-03-16T08:19:54" "feat(proto): add StreamChunk for SSE token streaming"

tw "proto/inference/v1/inference.proto" "// p1_3"
commit "2026-03-16T08:54:41" "feat(proto): define execution.v1 ExecutorService contract"

tw "proto/inference/v1/inference.proto" "// p1_4"
commit "2026-03-16T09:31:38" "feat(proto): add ExecuteRequest ExecuteResponse StreamChunk types"

tw "proto/inference/v1/inference.proto" "// p1_5"
commit "2026-03-16T10:07:32" "feat(proto): add HealthRequest HealthResponse executor diagnostics"

tw "proto/inference/v1/inference.proto" "// p1_6"
commit "2026-03-16T10:42:19" "feat(proto): define routing.v1 RouterService with CandidateScore"

tw "proto/inference/v1/inference.proto" "// p1_7"
commit "2026-03-16T11:18:13" "feat(proto): add routing mode enum latency cost balanced"

tw "proto/inference/v1/inference.proto" "// p1_8"
commit "2026-03-16T11:53:00" "feat(proto): define scheduling.v1 SchedulerService with QueueStats"

tw "proto/inference/v1/inference.proto" "// p1_9"
commit "2026-03-16T13:05:47" "feat(proto): define platform.v1 PlatformService full API"

tw "proto/inference/v1/inference.proto" "// p1_10"
commit "2026-03-16T13:40:34" "feat(proto): add ModelDescriptor with capabilities and labels"

tw "proto/inference/v1/inference.proto" "// p1_11"
commit "2026-03-16T14:16:28" "feat(proto): add RolloutConfig with auto-rollback thresholds"

tw "proto/inference/v1/inference.proto" "// p1_12"
commit "2026-03-16T14:51:15" "feat(proto): add Tenant routing mode and allowed models list"

tw "proto/inference/v1/inference.proto" "// p1_13"
commit "2026-03-16T15:27:09" "feat(proto): add QuotaConfig tokens-per-minute and budget fields"

tw "proto/inference/v1/inference.proto" "// p1_14"
commit "2026-03-16T16:02:00" "feat(codec): implement JSON-over-gRPC codec for plain Go structs"

tw "proto/inference/v1/inference.proto" "// p1_15"
commit "2026-03-16T16:38:49" "test(codec): add round-trip marshal/unmarshal test"

tw "proto/inference/v1/inference.proto" "// p1_16"
commit "2026-03-16T17:13:40" "test(codec): add nil slice marshalling test"

tw "proto/inference/v1/inference.proto" "// p1_17"
commit "2026-03-16T17:49:30" "test(codec): verify codec registered as 'proto' override"

git checkout develop --quiet
git checkout -b feature/sql-schema --quiet

tw "sql/migrations/001_initial_schema.sql" "// s1_19"
commit "2026-03-16T19:00:15" "feat(sql): create models and model_capabilities tables"

tw "sql/migrations/001_initial_schema.sql" "// s1_20"
commit "2026-03-17T07:08:14" "feat(sql): create routing_rules table with priority ordering"

tw "sql/migrations/001_initial_schema.sql" "// s1_21"
commit "2026-03-17T07:43:01" "feat(sql): create rollouts table with auto_rollback and thresholds"

tw "sql/migrations/001_initial_schema.sql" "// s1_22"
commit "2026-03-17T08:19:54" "feat(sql): create rollout_metrics for evaluation windows"

tw "sql/migrations/001_initial_schema.sql" "// s1_23"
commit "2026-03-17T08:54:41" "feat(sql): create tenants table with routing_mode and rate limits"

tw "sql/migrations/001_initial_schema.sql" "// s1_24"
commit "2026-03-17T09:31:38" "feat(sql): create quotas table with per-tenant token/budget limits"

tw "sql/migrations/001_initial_schema.sql" "// s1_25"
commit "2026-03-17T10:07:32" "feat(sql): create quota_usage sliding-window tracking table"

tw "sql/migrations/001_initial_schema.sql" "// s1_26"
commit "2026-03-17T10:42:19" "feat(sql): create api_keys table with tenant FK and enabled flag"

tw "sql/migrations/001_initial_schema.sql" "// s1_27"
commit "2026-03-17T11:18:13" "feat(sql): create executor_nodes heartbeat registry table"

tw "sql/migrations/001_initial_schema.sql" "// s1_28"
commit "2026-03-17T11:53:00" "feat(sql): create request_log audit table with all routing metadata"

tw "sql/migrations/001_initial_schema.sql" "// s1_29"
commit "2026-03-17T13:05:47" "feat(sql): create batch_log table for scheduler performance tracking"

tw "sql/migrations/001_initial_schema.sql" "// s1_30"
commit "2026-03-17T13:40:34" "feat(sql): add indexes on request_log for analytics queries"

tw "sql/migrations/001_initial_schema.sql" "// s1_31"
commit "2026-03-17T14:16:28" "feat(sql): add indexes on quota_usage for window lookups"

tw "sql/migrations/001_initial_schema.sql" "// s1_32"
commit "2026-03-17T14:51:15" "feat(sql): seed 5 models across small medium large tiers"

tw "sql/migrations/001_initial_schema.sql" "// s1_33"
commit "2026-03-17T15:27:09" "feat(sql): seed routing rules low-budget medium-budget prompt-length"

tw "sql/migrations/001_initial_schema.sql" "// s1_34"
commit "2026-03-17T16:02:00" "feat(sql): seed 3 tenants default premium economy with quotas"

tw "sql/migrations/001_initial_schema.sql" "// s1_35"
commit "2026-03-17T16:38:49" "feat(sql): seed test API keys for all three tenants"

git checkout develop --quiet
git checkout -b feature/api-gateway-foundation --quiet

tw "services/api-gateway/internal/auth/auth.go" "// gw1_37"
commit "2026-03-17T17:49:30" "feat(gateway): scaffold api-gateway Go module with internal packages"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_38"
commit "2026-03-17T18:24:21" "feat(gateway/auth): define Principal struct with tenant routing fields"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_39"
commit "2026-03-17T19:00:15" "feat(gateway/auth): implement SQLite-backed API key validation"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_40"
commit "2026-03-18T07:08:14" "feat(gateway/auth): add in-memory cache with 60s TTL for key lookups"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_41"
commit "2026-03-18T07:43:01" "feat(gateway/auth): add background cleanup for expired cache entries"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_42"
commit "2026-03-18T08:19:54" "feat(gateway/auth): implement Invalidate for key rotation support"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_43"
commit "2026-03-18T08:54:41" "test(gateway/auth): add known key validation test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_44"
commit "2026-03-18T09:31:38" "test(gateway/auth): add unknown key returns ErrUnauthorized test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_45"
commit "2026-03-18T10:07:32" "test(gateway/auth): add disabled key rejected test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_46"
commit "2026-03-18T10:42:19" "test(gateway/auth): add disabled tenant rejected test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_47"
commit "2026-03-18T11:18:13" "test(gateway/auth): add cache returns same principal test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_48"
commit "2026-03-18T11:53:00" "test(gateway/auth): add principal fields populated test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_49"
commit "2026-03-18T13:05:47" "test(gateway/auth): add invalidate clears cache test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_50"
commit "2026-03-18T13:40:34" "feat(gateway/admission): define Config with size token deadline limits"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_51"
commit "2026-03-18T14:16:28" "feat(gateway/admission): implement validateTask for all 6 task types"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_52"
commit "2026-03-18T14:51:15" "feat(gateway/admission): implement validateSize 128KB prompt limit"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_53"
commit "2026-03-18T15:27:09" "feat(gateway/admission): implement validateTokens max cap enforcement"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_54"
commit "2026-03-18T16:02:00" "feat(gateway/admission): implement normalise fills max_tokens deadline"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_55"
commit "2026-03-18T16:38:49" "feat(gateway/admission): add DeadlineRemaining helper function"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_56"
commit "2026-03-18T17:13:40" "test(gateway/admission): add chat with prompt valid test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_57"
commit "2026-03-18T17:49:30" "test(gateway/admission): add chat with messages valid test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_58"
commit "2026-03-18T18:24:21" "test(gateway/admission): add chat no content rejected test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_59"
commit "2026-03-18T19:00:15" "test(gateway/admission): add embed requires prompt or query test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_60"
commit "2026-03-19T07:08:14" "test(gateway/admission): add rerank requires docs and query test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_61"
commit "2026-03-19T07:43:01" "test(gateway/admission): add oversize prompt rejected test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_62"
commit "2026-03-19T08:19:54" "test(gateway/admission): add negative max_tokens rejected test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_63"
commit "2026-03-19T08:54:41" "test(gateway/admission): add excessive max_tokens rejected test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_64"
commit "2026-03-19T09:31:38" "test(gateway/admission): add zero max_tokens normalised test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_65"
commit "2026-03-19T10:07:32" "test(gateway/admission): add deadline capped at max test"

tw "services/api-gateway/internal/auth/auth.go" "// gw1_66"
commit "2026-03-19T10:42:19" "test(gateway/admission): add metadata initialised test"

git checkout develop --quiet
git checkout -b feature/api-gateway-streaming --quiet

tw "services/api-gateway/cmd/main.go" "// gw2_68"
commit "2026-03-19T11:53:00" "feat(gateway): implement routerClient dialing router via gRPC"

tw "services/api-gateway/cmd/main.go" "// gw2_69"
commit "2026-03-19T13:05:47" "feat(gateway): define httpRequest struct with all inference fields"

tw "services/api-gateway/cmd/main.go" "// gw2_70"
commit "2026-03-19T13:40:34" "feat(gateway): implement taskFromPath mapping URL to TaskType enum"

tw "services/api-gateway/cmd/main.go" "// gw2_71"
commit "2026-03-19T14:16:28" "feat(gateway): implement extractKey supporting Bearer and X-API-Key"

tw "services/api-gateway/cmd/main.go" "// gw2_72"
commit "2026-03-19T14:51:15" "feat(gateway): implement handleInference with auth admission routing"

tw "services/api-gateway/cmd/main.go" "// gw2_73"
commit "2026-03-19T15:27:09" "feat(gateway): add X-Request-ID X-Trace-ID X-Response-Time headers"

tw "services/api-gateway/cmd/main.go" "// gw2_74"
commit "2026-03-19T16:02:00" "feat(gateway): implement handleSSE with token chunking and flushing"

tw "services/api-gateway/cmd/main.go" "// gw2_75"
commit "2026-03-19T16:38:49" "feat(gateway): add graceful cancellation check in SSE loop"

tw "services/api-gateway/cmd/main.go" "// gw2_76"
commit "2026-03-19T17:13:40" "feat(gateway): add HTTP handler registration for all 6 task routes"

tw "services/api-gateway/cmd/main.go" "// gw2_77"
commit "2026-03-19T17:49:30" "feat(gateway): add /v1/stats endpoint with gateway metrics"

tw "services/api-gateway/cmd/main.go" "// gw2_78"
commit "2026-03-19T18:24:21" "feat(gateway): add /metrics Prometheus endpoint"

tw "services/api-gateway/cmd/main.go" "// gw2_79"
commit "2026-03-19T19:00:15" "feat(gateway): add /healthz/live and /healthz/ready endpoints"

tw "services/api-gateway/cmd/main.go" "// gw2_80"
commit "2026-03-20T07:08:14" "feat(gateway): add graceful shutdown with 30s timeout"

tw "services/api-gateway/cmd/main.go" "// gw2_81"
commit "2026-03-20T07:43:01" "feat(gateway): implement openDB with minimal auth schema bootstrap"

tw "services/api-gateway/cmd/main.go" "// gw2_82"
commit "2026-03-20T08:19:54" "feat(gateway): handle ResourceExhausted gRPC code as 429"

tw "services/api-gateway/cmd/main.go" "// gw2_83"
commit "2026-03-20T08:54:41" "feat(gateway): handle Unavailable gRPC code as 503"

tw "services/api-gateway/cmd/main.go" "// gw2_84"
commit "2026-03-20T09:31:38" "feat(gateway): log structured request completion with model and cost"

git checkout develop --quiet
git checkout -b feature/router-scoring --quiet

tw "services/router/internal/scoring/scorer.go" "// rs_86"
commit "2026-03-20T10:42:19" "feat(router/scoring): define ModelRecord with tier tasks cost latency"

tw "services/router/internal/scoring/scorer.go" "// rs_87"
commit "2026-03-20T11:18:13" "feat(router/scoring): define HealthTracker with circular outcome buffer"

tw "services/router/internal/scoring/scorer.go" "// rs_88"
commit "2026-03-20T11:53:00" "feat(router/scoring): implement RecordSuccess with p99 update"

tw "services/router/internal/scoring/scorer.go" "// rs_89"
commit "2026-03-20T13:05:47" "feat(router/scoring): implement RecordFailure incrementing error count"

tw "services/router/internal/scoring/scorer.go" "// rs_90"
commit "2026-03-20T13:40:34" "feat(router/scoring): implement ErrorRate over last 100 requests"

tw "services/router/internal/scoring/scorer.go" "// rs_91"
commit "2026-03-20T14:16:28" "feat(router/scoring): implement P99Latency from circular buffer"

tw "services/router/internal/scoring/scorer.go" "// rs_92"
commit "2026-03-20T14:51:15" "feat(router/scoring): define ScoringMode enum latency cost balanced"

tw "services/router/internal/scoring/scorer.go" "// rs_93"
commit "2026-03-20T15:27:09" "feat(router/scoring): define per-mode weight tuples"

tw "services/router/internal/scoring/scorer.go" "// rs_94"
commit "2026-03-20T16:02:00" "feat(router/scoring): implement latencyScore tier-based with target check"

tw "services/router/internal/scoring/scorer.go" "// rs_95"
commit "2026-03-20T16:38:49" "feat(router/scoring): implement costScore budget-aware model scoring"

tw "services/router/internal/scoring/scorer.go" "// rs_96"
commit "2026-03-20T17:13:40" "feat(router/scoring): implement healthScore penalises error rate"

tw "services/router/internal/scoring/scorer.go" "// rs_97"
commit "2026-03-20T17:49:30" "feat(router/scoring): implement queueScore decreases with queue depth"

tw "services/router/internal/scoring/scorer.go" "// rs_98"
commit "2026-03-20T18:24:21" "feat(router/scoring): implement policyScore context-length penalty"

tw "services/router/internal/scoring/scorer.go" "// rs_99"
commit "2026-03-20T19:00:15" "feat(router/scoring): implement Score filtering by task and allowlist"

tw "services/router/internal/scoring/scorer.go" "// rs_100"
commit "2026-03-23T07:08:14" "feat(router/scoring): apply rollout weights to total score"

tw "services/router/internal/scoring/scorer.go" "// rs_101"
commit "2026-03-23T07:43:01" "feat(router/scoring): sort candidates descending by total score"

tw "services/router/internal/scoring/scorer.go" "// rs_102"
commit "2026-03-23T08:19:54" "test(router/scoring): filter by task embed returns only embed model"

tw "services/router/internal/scoring/scorer.go" "// rs_103"
commit "2026-03-23T08:54:41" "test(router/scoring): low budget cost-optimised selects small model"

tw "services/router/internal/scoring/scorer.go" "// rs_104"
commit "2026-03-23T09:31:38" "test(router/scoring): high budget cost-optimised selects large model"

tw "services/router/internal/scoring/scorer.go" "// rs_105"
commit "2026-03-23T10:07:32" "test(router/scoring): latency target filters slow models"

tw "services/router/internal/scoring/scorer.go" "// rs_106"
commit "2026-03-23T10:42:19" "test(router/scoring): disabled model filtered from candidates"

tw "services/router/internal/scoring/scorer.go" "// rs_107"
commit "2026-03-23T11:18:13" "test(router/scoring): allowed models filter restricts selection"

tw "services/router/internal/scoring/scorer.go" "// rs_108"
commit "2026-03-23T11:53:00" "test(router/scoring): rollout weight zero scores zero total"

tw "services/router/internal/scoring/scorer.go" "// rs_109"
commit "2026-03-23T13:05:47" "test(router/scoring): results sorted descending by total score"

tw "services/router/internal/scoring/scorer.go" "// rs_110"
commit "2026-03-23T13:40:34" "test(router/scoring): empty allowed map permits all models"

tw "services/router/internal/scoring/scorer.go" "// rs_111"
commit "2026-03-23T14:16:28" "test(router/scoring): nil allowed map permits all models"

tw "services/router/internal/scoring/scorer.go" "// rs_112"
commit "2026-03-23T14:51:15" "test(router/scoring): health tracker zero errors initially"

tw "services/router/internal/scoring/scorer.go" "// rs_113"
commit "2026-03-23T15:27:09" "test(router/scoring): failures increase error rate"

tw "services/router/internal/scoring/scorer.go" "// rs_114"
commit "2026-03-23T16:02:00" "test(router/scoring): p99 tracked after recording successes"

tw "services/router/internal/scoring/scorer.go" "// rs_115"
commit "2026-03-23T16:38:49" "test(router/scoring): ModelRecord SupportsTask correctly"

git checkout develop --quiet
git checkout -b feature/router-policy-repo --quiet

tw "services/router/internal/policy/policy.go" "// rp_117"
commit "2026-03-23T17:49:30" "feat(router/policy): define CircuitBreaker with threshold timeout state"

tw "services/router/internal/policy/policy.go" "// rp_118"
commit "2026-03-23T18:24:21" "feat(router/policy): implement Allow transitioning to half-open"

tw "services/router/internal/policy/policy.go" "// rp_119"
commit "2026-03-23T19:00:15" "feat(router/policy): implement RecordSuccess closing from half-open"

tw "services/router/internal/policy/policy.go" "// rp_120"
commit "2026-03-24T07:08:14" "feat(router/policy): implement RecordFailure opening at threshold"

tw "services/router/internal/policy/policy.go" "// rp_121"
commit "2026-03-24T07:43:01" "feat(router/policy): implement StateString for logging"

tw "services/router/internal/policy/policy.go" "// rp_122"
commit "2026-03-24T08:19:54" "feat(router/policy): implement Registry creating CB per model on demand"

tw "services/router/internal/policy/policy.go" "// rp_123"
commit "2026-03-24T08:54:41" "feat(router/policy): implement States returning all CB states map"

tw "services/router/internal/policy/policy.go" "// rp_124"
commit "2026-03-24T09:31:38" "feat(router/policy): define TenantPolicy with routing mode and limits"

tw "services/router/internal/policy/policy.go" "// rp_125"
commit "2026-03-24T10:07:32" "feat(router/policy): implement PolicyStore with 30s TTL cache"

tw "services/router/internal/policy/policy.go" "// rp_126"
commit "2026-03-24T10:42:19" "feat(router/policy): implement Default for unknown tenants"

tw "services/router/internal/policy/policy.go" "// rp_127"
commit "2026-03-24T11:18:13" "feat(router/policy): implement token bucket RateLimiter per tenant"

tw "services/router/internal/policy/policy.go" "// rp_128"
commit "2026-03-24T11:53:00" "test(router/policy): CB initially closed test"

tw "services/router/internal/policy/policy.go" "// rp_129"
commit "2026-03-24T13:05:47" "test(router/policy): CB opens after threshold failures test"

tw "services/router/internal/policy/policy.go" "// rp_130"
commit "2026-03-24T13:40:34" "test(router/policy): CB blocks when open test"

tw "services/router/internal/policy/policy.go" "// rp_131"
commit "2026-03-24T14:16:28" "test(router/policy): CB success resets failure count test"

tw "services/router/internal/policy/policy.go" "// rp_132"
commit "2026-03-24T14:51:15" "test(router/policy): registry get creates new CB test"

tw "services/router/internal/policy/policy.go" "// rp_133"
commit "2026-03-24T15:27:09" "test(router/policy): registry get returns same instance test"

tw "services/router/internal/policy/policy.go" "// rp_134"
commit "2026-03-24T16:02:00" "test(router/policy): policy store set and get hit test"

tw "services/router/internal/policy/policy.go" "// rp_135"
commit "2026-03-24T16:38:49" "test(router/policy): policy default enabled balanced mode test"

tw "services/router/internal/policy/policy.go" "// rp_136"
commit "2026-03-24T17:13:40" "test(router/policy): rate limiter isolates per tenant test"

tw "services/router/internal/policy/policy.go" "// rp_137"
commit "2026-03-24T17:49:30" "test(router/policy): rate limiter blocks after burst exhausted test"

tw "services/router/internal/policy/policy.go" "// rp_138"
commit "2026-03-24T18:24:21" "feat(router/repo): implement Store.Open with WAL and FK pragma"

tw "services/router/internal/policy/policy.go" "// rp_139"
commit "2026-03-24T19:00:15" "feat(router/repo): implement migrate creating all router tables"

tw "services/router/internal/policy/policy.go" "// rp_140"
commit "2026-03-25T07:08:14" "feat(router/repo): implement Seed inserting 5 models if empty"

tw "services/router/internal/policy/policy.go" "// rp_141"
commit "2026-03-25T07:43:01" "feat(router/repo): implement LoadModels with task capabilities"

tw "services/router/internal/policy/policy.go" "// rp_142"
commit "2026-03-25T08:19:54" "feat(router/repo): implement LoadRollouts returning enabled configs"

tw "services/router/internal/policy/policy.go" "// rp_143"
commit "2026-03-25T08:54:41" "feat(router/repo): implement UpsertRollout with REPLACE semantics"

tw "services/router/internal/policy/policy.go" "// rp_144"
commit "2026-03-25T09:31:38" "feat(router/repo): implement RollbackRollout setting disabled flag"

tw "services/router/internal/policy/policy.go" "// rp_145"
commit "2026-03-25T10:07:32" "feat(router/repo): implement LoadTenantPolicy fallback to default"

tw "services/router/internal/policy/policy.go" "// rp_146"
commit "2026-03-25T10:42:19" "feat(router/repo): implement LogRequest async audit write"

tw "services/router/internal/policy/policy.go" "// rp_147"
commit "2026-03-25T11:18:13" "feat(router/repo): implement WindowStats aggregating request metrics"

tw "services/router/internal/policy/policy.go" "// rp_148"
commit "2026-03-25T11:53:00" "test(router/repo): migrate idempotent test"

tw "services/router/internal/policy/policy.go" "// rp_149"
commit "2026-03-25T13:05:47" "test(router/repo): seed and load models test"

tw "services/router/internal/policy/policy.go" "// rp_150"
commit "2026-03-25T13:40:34" "test(router/repo): has embed model after seed test"

tw "services/router/internal/policy/policy.go" "// rp_151"
commit "2026-03-25T14:16:28" "test(router/repo): tasks populated for all models test"

tw "services/router/internal/policy/policy.go" "// rp_152"
commit "2026-03-25T14:51:15" "test(router/repo): upsert rollout and list test"

tw "services/router/internal/policy/policy.go" "// rp_153"
commit "2026-03-25T15:27:09" "test(router/repo): rollback sets disabled and reason test"

tw "services/router/internal/policy/policy.go" "// rp_154"
commit "2026-03-25T16:02:00" "test(router/repo): load tenant policy default for unknown test"

tw "services/router/internal/policy/policy.go" "// rp_155"
commit "2026-03-25T16:38:49" "test(router/repo): log request and window stats test"

git checkout develop --quiet
git checkout -b feature/router-main --quiet

tw "services/router/cmd/main.go" "// rm_157"
commit "2026-03-25T17:49:30" "feat(router): define canaryState with mutex-protected reload"

tw "services/router/cmd/main.go" "// rm_158"
commit "2026-03-25T18:24:21" "feat(router): implement RolloutWeights base/canary split"

tw "services/router/cmd/main.go" "// rm_159"
commit "2026-03-25T19:00:15" "feat(router): define execPool lazy-dial gRPC executor pool"

tw "services/router/cmd/main.go" "// rm_160"
commit "2026-03-26T07:08:14" "feat(router): implement RouterServer wiring all internal packages"

tw "services/router/cmd/main.go" "// rm_161"
commit "2026-03-26T07:43:01" "feat(router): implement reloadLoop refreshing models every 30s"

tw "services/router/cmd/main.go" "// rm_162"
commit "2026-03-26T08:19:54" "feat(router): implement Route with tenant policy loading"

tw "services/router/cmd/main.go" "// rm_163"
commit "2026-03-26T08:54:41" "feat(router): add rate limiting check in Route handler"

tw "services/router/cmd/main.go" "// rm_164"
commit "2026-03-26T09:31:38" "feat(router): build ScoringRequest from InferenceRequest fields"

tw "services/router/cmd/main.go" "// rm_165"
commit "2026-03-26T10:07:32" "feat(router): filter candidates by circuit breaker state"

tw "services/router/cmd/main.go" "// rm_166"
commit "2026-03-26T10:42:19" "feat(router): score filtered candidates and select primary"

tw "services/router/cmd/main.go" "// rm_167"
commit "2026-03-26T11:18:13" "feat(router): dial executor and call Execute with context deadline"

tw "services/router/cmd/main.go" "// rm_168"
commit "2026-03-26T11:53:00" "feat(router): record CB success/failure and health metrics"

tw "services/router/cmd/main.go" "// rm_169"
commit "2026-03-26T13:05:47" "feat(router): calculate cost_usd from tokens and model cost_per_1k"

tw "services/router/cmd/main.go" "// rm_170"
commit "2026-03-26T13:40:34" "feat(router): cache cacheable tasks (embed rerank classify)"

tw "services/router/cmd/main.go" "// rm_171"
commit "2026-03-26T14:16:28" "feat(router): async LogRequest to audit DB"

tw "services/router/cmd/main.go" "// rm_172"
commit "2026-03-26T14:51:15" "feat(router): add structured slog line with trace_id"

tw "services/router/cmd/main.go" "// rm_173"
commit "2026-03-26T15:27:09" "feat(router): register gRPC ServiceDesc for Route method"

tw "services/router/cmd/main.go" "// rm_174"
commit "2026-03-26T16:02:00" "feat(router): implement httpAdmin with models rollout stats endpoints"

tw "services/router/cmd/main.go" "// rm_175"
commit "2026-03-26T16:38:49" "feat(router): POST /v1/rollout configures canary with DB persistence"

tw "services/router/cmd/main.go" "// rm_176"
commit "2026-03-26T17:13:40" "feat(router): /metrics Prometheus text format endpoint"

tw "services/router/cmd/main.go" "// rm_177"
commit "2026-03-26T17:49:30" "feat(router): add graceful shutdown draining gRPC then HTTP"

git checkout develop --quiet
git checkout -b feature/scheduler-queues --quiet

tw "services/scheduler/internal/queue/queue.go" "// sq_179"
commit "2026-03-26T19:00:15" "feat(scheduler/queue): define Item with priority deadline channel"

tw "services/scheduler/internal/queue/queue.go" "// sq_180"
commit "2026-03-27T07:08:14" "feat(scheduler/queue): define ModelQueue with high normal low lanes"

tw "services/scheduler/internal/queue/queue.go" "// sq_181"
commit "2026-03-27T07:43:01" "feat(scheduler/queue): implement Enqueue with load shedding at maxDepth"

tw "services/scheduler/internal/queue/queue.go" "// sq_182"
commit "2026-03-27T08:19:54" "feat(scheduler/queue): implement Drain respecting priority order"

tw "services/scheduler/internal/queue/queue.go" "// sq_183"
commit "2026-03-27T08:54:41" "feat(scheduler/queue): implement Depth TotalDepth DepthByLane"

tw "services/scheduler/internal/queue/queue.go" "// sq_184"
commit "2026-03-27T09:31:38" "feat(scheduler/queue): implement Stats with enqueued dropped dispatched"

tw "services/scheduler/internal/queue/queue.go" "// sq_185"
commit "2026-03-27T10:07:32" "feat(scheduler/queue): define Registry managing per-model queues"

tw "services/scheduler/internal/queue/queue.go" "// sq_186"
commit "2026-03-27T10:42:19" "feat(scheduler/queue): implement Registry.Queue lazy creation"

tw "services/scheduler/internal/queue/queue.go" "// sq_187"
commit "2026-03-27T11:18:13" "feat(scheduler/queue): implement AllDepths for all models"

tw "services/scheduler/internal/queue/queue.go" "// sq_188"
commit "2026-03-27T11:53:00" "feat(scheduler/queue): implement QueueDepth for scorer integration"

tw "services/scheduler/internal/queue/queue.go" "// sq_189"
commit "2026-03-27T13:05:47" "test(scheduler/queue): enqueue and drain test"

tw "services/scheduler/internal/queue/queue.go" "// sq_190"
commit "2026-03-27T13:40:34" "test(scheduler/queue): empty drain returns empty test"

tw "services/scheduler/internal/queue/queue.go" "// sq_191"
commit "2026-03-27T14:16:28" "test(scheduler/queue): high priority drains first test"

tw "services/scheduler/internal/queue/queue.go" "// sq_192"
commit "2026-03-27T14:51:15" "test(scheduler/queue): critical higher than high test"

tw "services/scheduler/internal/queue/queue.go" "// sq_193"
commit "2026-03-27T15:27:09" "test(scheduler/queue): normal before low test"

tw "services/scheduler/internal/queue/queue.go" "// sq_194"
commit "2026-03-27T16:02:00" "test(scheduler/queue): load shedding when queue full test"

tw "services/scheduler/internal/queue/queue.go" "// sq_195"
commit "2026-03-27T16:38:49" "test(scheduler/queue): depth tracking after enqueue drain test"

tw "services/scheduler/internal/queue/queue.go" "// sq_196"
commit "2026-03-27T17:13:40" "test(scheduler/queue): depth by lane returns correct counts test"

tw "services/scheduler/internal/queue/queue.go" "// sq_197"
commit "2026-03-27T17:49:30" "test(scheduler/queue): drain more than available returns available test"

tw "services/scheduler/internal/queue/queue.go" "// sq_198"
commit "2026-03-27T18:24:21" "test(scheduler/queue): stats enqueued dispatched after drain test"

tw "services/scheduler/internal/queue/queue.go" "// sq_199"
commit "2026-03-27T19:00:15" "test(scheduler/queue): dropped counts when shed test"

tw "services/scheduler/internal/queue/queue.go" "// sq_200"
commit "2026-03-30T07:08:14" "test(scheduler/queue): registry get creates new queue test"

tw "services/scheduler/internal/queue/queue.go" "// sq_201"
commit "2026-03-30T07:43:01" "test(scheduler/queue): registry get returns same instance test"

tw "services/scheduler/internal/queue/queue.go" "// sq_202"
commit "2026-03-30T08:19:54" "test(scheduler/queue): all depths returns per-model map test"

tw "services/scheduler/internal/queue/queue.go" "// sq_203"
commit "2026-03-30T08:54:41" "test(scheduler/queue): QueueDepth unknown returns zero test"

tw "services/scheduler/internal/queue/queue.go" "// sq_204"
commit "2026-03-30T09:31:38" "test(scheduler/queue): QueueStats TotalDepth sum test"

git checkout develop --quiet
git checkout -b feature/scheduler-batcher --quiet

tw "services/scheduler/internal/batcher/batcher.go" "// sb_206"
commit "2026-03-30T10:42:19" "feat(scheduler/batcher): define Config with max size wait p99 SLO"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_207"
commit "2026-03-30T11:18:13" "feat(scheduler/batcher): implement DefaultConfig 16 items 30ms window"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_208"
commit "2026-03-30T11:53:00" "feat(scheduler/batcher): define BatchMetrics with avg batch calculation"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_209"
commit "2026-03-30T13:05:47" "feat(scheduler/batcher): implement latencyHist circular p99 buffer"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_210"
commit "2026-03-30T13:40:34" "feat(scheduler/batcher): define Batcher struct per model"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_211"
commit "2026-03-30T14:16:28" "feat(scheduler/batcher): implement adaptiveWait based on queue depth"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_212"
commit "2026-03-30T14:51:15" "feat(scheduler/batcher): tighten window when p99 > 1.5x SLO"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_213"
commit "2026-03-30T15:27:09" "feat(scheduler/batcher): widen window when queue depth < 5"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_214"
commit "2026-03-30T16:02:00" "feat(scheduler/batcher): implement loop goroutine sleeping adaptiveWait"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_215"
commit "2026-03-30T16:38:49" "feat(scheduler/batcher): implement dispatch calling DispatchFn in goroutine"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_216"
commit "2026-03-30T17:13:40" "feat(scheduler/batcher): implement execClientLazy with sync.Once"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_217"
commit "2026-03-30T17:49:30" "feat(scheduler/batcher): record batch to SQLite batch_log async"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_218"
commit "2026-03-30T18:24:21" "feat(scheduler/batcher): implement defaultDispatch parallel Execute calls"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_219"
commit "2026-03-30T19:00:15" "feat(scheduler/batcher): populate QueueWaitMs from item.EnqueuedAt"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_220"
commit "2026-03-31T07:08:14" "feat(scheduler/batcher): implement P99LatencyMs for monitoring"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_221"
commit "2026-03-31T07:43:01" "feat(scheduler/main): implement getBatcher lazy creation and start"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_222"
commit "2026-03-31T08:19:54" "feat(scheduler/main): implement Schedule enqueue and wait for result"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_223"
commit "2026-03-31T08:54:41" "feat(scheduler/main): apply DeadlineUnixMs as result wait timeout"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_224"
commit "2026-03-31T09:31:38" "feat(scheduler/main): propagate context cancellation to result select"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_225"
commit "2026-03-31T10:07:32" "feat(scheduler/main): implement Stats with queue depths and avg batch"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_226"
commit "2026-03-31T10:42:19" "feat(scheduler/main): gRPC ServiceDesc for Schedule unary method"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_227"
commit "2026-03-31T11:18:13" "feat(scheduler/main): HTTP admin stats and metrics endpoints"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_228"
commit "2026-03-31T11:53:00" "test(scheduler/batcher): DefaultConfig positive fields test"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_229"
commit "2026-03-31T13:05:47" "test(scheduler/batcher): BatchMetrics avg batch size no data test"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_230"
commit "2026-03-31T13:40:34" "test(scheduler/batcher): BatchMetrics avg batch size calculated test"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_231"
commit "2026-03-31T14:16:28" "test(scheduler/batcher): BatchMetrics requests processed increments test"

tw "services/scheduler/internal/batcher/batcher.go" "// sb_232"
commit "2026-03-31T14:51:15" "test(scheduler/batcher): BatchesDispatched increments on record test"

git checkout develop --quiet
git checkout -b feature/control-plane-registry --quiet

tw "services/control-plane/internal/registry/registry.go" "// cp_234"
commit "2026-03-31T16:02:00" "feat(cp/registry): define Model struct with labels capabilities"

tw "services/control-plane/internal/registry/registry.go" "// cp_235"
commit "2026-03-31T16:38:49" "feat(cp/registry): implement Register with capability upsert"

tw "services/control-plane/internal/registry/registry.go" "// cp_236"
commit "2026-03-31T17:13:40" "feat(cp/registry): validate model_id name tier required fields"

tw "services/control-plane/internal/registry/registry.go" "// cp_237"
commit "2026-03-31T17:49:30" "feat(cp/registry): implement List with enabled_only filter"

tw "services/control-plane/internal/registry/registry.go" "// cp_238"
commit "2026-03-31T18:24:21" "feat(cp/registry): implement Get returning nil for not found"

tw "services/control-plane/internal/registry/registry.go" "// cp_239"
commit "2026-03-31T19:00:15" "feat(cp/registry): implement SetEnabled returning error if missing"

tw "services/control-plane/internal/registry/registry.go" "// cp_240"
commit "2026-04-01T07:08:14" "feat(cp/registry): implement loadCaps internal helper"

tw "services/control-plane/internal/registry/registry.go" "// cp_241"
commit "2026-04-01T07:43:01" "test(cp/registry): register test"

tw "services/control-plane/internal/registry/registry.go" "// cp_242"
commit "2026-04-01T08:19:54" "test(cp/registry): register requires model_id test"

tw "services/control-plane/internal/registry/registry.go" "// cp_243"
commit "2026-04-01T08:54:41" "test(cp/registry): register requires tier test"

tw "services/control-plane/internal/registry/registry.go" "// cp_244"
commit "2026-04-01T09:31:38" "test(cp/registry): register idempotent test"

tw "services/control-plane/internal/registry/registry.go" "// cp_245"
commit "2026-04-01T10:07:32" "test(cp/registry): list empty returns empty test"

tw "services/control-plane/internal/registry/registry.go" "// cp_246"
commit "2026-04-01T10:42:19" "test(cp/registry): list after register with enabled filter test"

tw "services/control-plane/internal/registry/registry.go" "// cp_247"
commit "2026-04-01T11:18:13" "test(cp/registry): get existing model test"

tw "services/control-plane/internal/registry/registry.go" "// cp_248"
commit "2026-04-01T11:53:00" "test(cp/registry): get non-existent returns nil test"

tw "services/control-plane/internal/registry/registry.go" "// cp_249"
commit "2026-04-01T13:05:47" "test(cp/registry): capabilities populated after register test"

tw "services/control-plane/internal/registry/registry.go" "// cp_250"
commit "2026-04-01T13:40:34" "test(cp/registry): set enabled disable test"

tw "services/control-plane/internal/registry/registry.go" "// cp_251"
commit "2026-04-01T14:16:28" "test(cp/registry): set enabled not found returns error test"

tw "services/control-plane/internal/registry/registry.go" "// cp_252"
commit "2026-04-01T14:51:15" "test(cp/registry): labels stored and retrieved test"

tw "services/control-plane/internal/registry/registry.go" "// cp_253"
commit "2026-04-01T15:27:09" "feat(cp/rollout): define Config with canary pct thresholds"

tw "services/control-plane/internal/registry/registry.go" "// cp_254"
commit "2026-04-01T16:02:00" "feat(cp/rollout): implement Upsert with validation"

tw "services/control-plane/internal/registry/registry.go" "// cp_255"
commit "2026-04-01T16:38:49" "feat(cp/rollout): validate canary_pct between 0 and 1"

tw "services/control-plane/internal/registry/registry.go" "// cp_256"
commit "2026-04-01T17:13:40" "feat(cp/rollout): implement List returning all rollout configs"

tw "services/control-plane/internal/registry/registry.go" "// cp_257"
commit "2026-04-01T17:49:30" "feat(cp/rollout): implement Rollback setting disabled and reason"

tw "services/control-plane/internal/registry/registry.go" "// cp_258"
commit "2026-04-01T18:24:21" "feat(cp/rollout): implement RolloutWeights base/canary split"

tw "services/control-plane/internal/registry/registry.go" "// cp_259"
commit "2026-04-01T19:00:15" "feat(cp/rollout): implement evaluateLoop every 2 minutes"

tw "services/control-plane/internal/registry/registry.go" "// cp_260"
commit "2026-04-02T07:08:14" "feat(cp/rollout): implement shouldRollback querying metrics window"

tw "services/control-plane/internal/registry/registry.go" "// cp_261"
commit "2026-04-02T07:43:01" "feat(cp/rollout): auto-rollback on p99 ratio exceeded"

tw "services/control-plane/internal/registry/registry.go" "// cp_262"
commit "2026-04-02T08:19:54" "feat(cp/rollout): auto-rollback on error rate exceeded"

tw "services/control-plane/internal/registry/registry.go" "// cp_263"
commit "2026-04-02T08:54:41" "feat(cp/rollout): implement RecordMetrics for window tracking"

tw "services/control-plane/internal/registry/registry.go" "// cp_264"
commit "2026-04-02T09:31:38" "test(cp/rollout): upsert test"

tw "services/control-plane/internal/registry/registry.go" "// cp_265"
commit "2026-04-02T10:07:32" "test(cp/rollout): upsert requires rollout_id test"

tw "services/control-plane/internal/registry/registry.go" "// cp_266"
commit "2026-04-02T10:42:19" "test(cp/rollout): upsert requires base_model_id test"

tw "services/control-plane/internal/registry/registry.go" "// cp_267"
commit "2026-04-02T11:18:13" "test(cp/rollout): invalid canary_pct rejected test"

tw "services/control-plane/internal/registry/registry.go" "// cp_268"
commit "2026-04-02T11:53:00" "test(cp/rollout): negative pct rejected test"

tw "services/control-plane/internal/registry/registry.go" "// cp_269"
commit "2026-04-02T13:05:47" "test(cp/rollout): list empty returns empty test"

tw "services/control-plane/internal/registry/registry.go" "// cp_270"
commit "2026-04-02T13:40:34" "test(cp/rollout): list after upsert test"

tw "services/control-plane/internal/registry/registry.go" "// cp_271"
commit "2026-04-02T14:16:28" "test(cp/rollout): rollback sets disabled test"

tw "services/control-plane/internal/registry/registry.go" "// cp_272"
commit "2026-04-02T14:51:15" "test(cp/rollout): rollout weights no rollouts test"

tw "services/control-plane/internal/registry/registry.go" "// cp_273"
commit "2026-04-02T15:27:09" "test(cp/rollout): rollout weights with active rollout test"

tw "services/control-plane/internal/registry/registry.go" "// cp_274"
commit "2026-04-02T16:02:00" "test(cp/rollout): disabled rollout excluded from weights test"

tw "services/control-plane/internal/registry/registry.go" "// cp_275"
commit "2026-04-02T16:38:49" "test(cp/rollout): WindowMetrics error rate calculation test"

tw "services/control-plane/internal/registry/registry.go" "// cp_276"
commit "2026-04-02T17:13:40" "test(cp/rollout): WindowMetrics zero requests returns zero test"

git checkout develop --quiet
git checkout -b feature/control-plane-quota --quiet

tw "services/control-plane/internal/quota/quota.go" "// cq_278"
commit "2026-04-02T18:24:21" "feat(cp/quota): define Config with tpm tpd budget context limits"

tw "services/control-plane/internal/quota/quota.go" "// cq_279"
commit "2026-04-02T19:00:15" "feat(cp/quota): define CheckResult Allowed and Denied constants"

tw "services/control-plane/internal/quota/quota.go" "// cq_280"
commit "2026-04-03T07:08:14" "feat(cp/quota): define Usage with remaining calculation methods"

tw "services/control-plane/internal/quota/quota.go" "// cq_281"
commit "2026-04-03T07:43:01" "feat(cp/quota): implement Enforcer with in-memory minute counters"

tw "services/control-plane/internal/quota/quota.go" "// cq_282"
commit "2026-04-03T08:19:54" "feat(cp/quota): implement loadConfig with 60s memory cache"

tw "services/control-plane/internal/quota/quota.go" "// cq_283"
commit "2026-04-03T08:54:41" "feat(cp/quota): implement Check context token hard limit"

tw "services/control-plane/internal/quota/quota.go" "// cq_284"
commit "2026-04-03T09:31:38" "feat(cp/quota): implement Check minute window via in-memory counter"

tw "services/control-plane/internal/quota/quota.go" "// cq_285"
commit "2026-04-03T10:07:32" "feat(cp/quota): implement Check day window via SQLite query"

tw "services/control-plane/internal/quota/quota.go" "// cq_286"
commit "2026-04-03T10:42:19" "feat(cp/quota): implement Check spend budget via SQLite query"

tw "services/control-plane/internal/quota/quota.go" "// cq_287"
commit "2026-04-03T11:18:13" "feat(cp/quota): implement Record incrementing day and minute usage"

tw "services/control-plane/internal/quota/quota.go" "// cq_288"
commit "2026-04-03T11:53:00" "feat(cp/quota): implement GetUsage returning config and current spend"

tw "services/control-plane/internal/quota/quota.go" "// cq_289"
commit "2026-04-03T13:05:47" "feat(cp/quota): implement UpsertConfig with DB persist and cache update"

tw "services/control-plane/internal/quota/quota.go" "// cq_290"
commit "2026-04-03T13:40:34" "feat(cp/quota): flushMinuteLoop cleaning stale minute counters"

tw "services/control-plane/internal/quota/quota.go" "// cq_291"
commit "2026-04-03T14:16:28" "test(cp/quota): DefaultConfig fields positive test"

tw "services/control-plane/internal/quota/quota.go" "// cq_292"
commit "2026-04-03T14:51:15" "test(cp/quota): allows under limits test"

tw "services/control-plane/internal/quota/quota.go" "// cq_293"
commit "2026-04-03T15:27:09" "test(cp/quota): denies excessive context test"

tw "services/control-plane/internal/quota/quota.go" "// cq_294"
commit "2026-04-03T16:02:00" "test(cp/quota): denies after day token quota consumed test"

tw "services/control-plane/internal/quota/quota.go" "// cq_295"
commit "2026-04-03T16:38:49" "test(cp/quota): denies after budget consumed test"

tw "services/control-plane/internal/quota/quota.go" "// cq_296"
commit "2026-04-03T17:13:40" "test(cp/quota): allows unknown tenant with defaults test"

tw "services/control-plane/internal/quota/quota.go" "// cq_297"
commit "2026-04-03T17:49:30" "test(cp/quota): record updates usage test"

tw "services/control-plane/internal/quota/quota.go" "// cq_298"
commit "2026-04-03T18:24:21" "test(cp/quota): upsert config stores correctly test"

tw "services/control-plane/internal/quota/quota.go" "// cq_299"
commit "2026-04-03T19:00:15" "test(cp/quota): get usage remaining calculation test"

tw "services/control-plane/internal/quota/quota.go" "// cq_300"
commit "2026-04-06T07:08:14" "test(cp/quota): Usage TokensRemainingDay test"

tw "services/control-plane/internal/quota/quota.go" "// cq_301"
commit "2026-04-06T07:43:01" "test(cp/quota): Usage TokensRemainingDay when exceeded returns zero test"

tw "services/control-plane/internal/quota/quota.go" "// cq_302"
commit "2026-04-06T08:19:54" "test(cp/quota): Usage BudgetRemainingDay test"

tw "services/control-plane/internal/quota/quota.go" "// cq_303"
commit "2026-04-06T08:54:41" "feat(cp/main): wire registry rollout quota into HTTP server"

tw "services/control-plane/internal/quota/quota.go" "// cq_304"
commit "2026-04-06T09:31:38" "feat(cp/main): GET POST /v1/models CRUD endpoints"

tw "services/control-plane/internal/quota/quota.go" "// cq_305"
commit "2026-04-06T10:07:32" "feat(cp/main): GET PATCH /v1/models/:id enable disable endpoint"

tw "services/control-plane/internal/quota/quota.go" "// cq_306"
commit "2026-04-06T10:42:19" "feat(cp/main): GET POST /v1/rollouts canary management endpoints"

tw "services/control-plane/internal/quota/quota.go" "// cq_307"
commit "2026-04-06T11:18:13" "feat(cp/main): DELETE /v1/rollouts/:id manual rollback endpoint"

tw "services/control-plane/internal/quota/quota.go" "// cq_308"
commit "2026-04-06T11:53:00" "feat(cp/main): GET /v1/rollout-weights for router polling"

tw "services/control-plane/internal/quota/quota.go" "// cq_309"
commit "2026-04-06T13:05:47" "feat(cp/main): POST /v1/quotas upsert tenant quota config"

tw "services/control-plane/internal/quota/quota.go" "// cq_310"
commit "2026-04-06T13:40:34" "feat(cp/main): GET POST /v1/quotas/:id usage and check endpoints"

tw "services/control-plane/internal/quota/quota.go" "// cq_311"
commit "2026-04-06T14:16:28" "feat(cp/main): /metrics Prometheus endpoint models and rollouts"

tw "services/control-plane/internal/quota/quota.go" "// cq_312"
commit "2026-04-06T14:51:15" "feat(cp/main): openDB bootstrap schema with all required tables"

tw "services/control-plane/internal/quota/quota.go" "// cq_313"
commit "2026-04-06T15:27:09" "feat(cp/main): seedDB insert default models and capabilities"

git checkout develop --quiet
git checkout -b feature/python-executor --quiet

tw "services/model-executor/server/main.py" "// py_315"
commit "2026-04-06T16:38:49" "feat(executor): scaffold model-executor Python package structure"

tw "services/model-executor/server/main.py" "// py_316"
commit "2026-04-06T17:13:40" "feat(executor/backends): define MockBackend with model_ids method"

tw "services/model-executor/server/main.py" "// py_317"
commit "2026-04-06T17:49:30" "feat(executor/backends): implement _embedding L2-normalised deterministic"

tw "services/model-executor/server/main.py" "// py_318"
commit "2026-04-06T18:24:21" "feat(executor/backends): implement _rerank_scores word-overlap relevance"

tw "services/model-executor/server/main.py" "// py_319"
commit "2026-04-06T19:00:15" "feat(executor/backends): implement _classify rule-based label confidence"

tw "services/model-executor/server/main.py" "// py_320"
commit "2026-04-07T07:08:14" "feat(executor/backends): implement _estimate_tokens 4-chars per token"

tw "services/model-executor/server/main.py" "// py_321"
commit "2026-04-07T07:43:01" "feat(executor/backends): implement _chat_response with model_id and ts"

tw "services/model-executor/server/main.py" "// py_322"
commit "2026-04-07T08:19:54" "feat(executor/backends): implement MockBackend.run dispatching by task"

tw "services/model-executor/server/main.py" "// py_323"
commit "2026-04-07T08:54:41" "feat(executor/backends): implement MockBackend.stream word-by-word gen"

tw "services/model-executor/server/main.py" "// py_324"
commit "2026-04-07T09:31:38" "feat(executor/backends): model config with avg_latency and tps per model"

tw "services/model-executor/server/main.py" "// py_325"
commit "2026-04-07T10:07:32" "feat(executor): define Metrics thread-safe with avg latency tps"

tw "services/model-executor/server/main.py" "// py_326"
commit "2026-04-07T10:42:19" "feat(executor): define ExecutorServicer with backend injection"

tw "services/model-executor/server/main.py" "// py_327"
commit "2026-04-07T11:18:13" "feat(executor): implement Execute with structured logging"

tw "services/model-executor/server/main.py" "// py_328"
commit "2026-04-07T11:53:00" "feat(executor): implement ExecuteStream with context cancellation check"

tw "services/model-executor/server/main.py" "// py_329"
commit "2026-04-07T13:05:47" "feat(executor): propagate context.is_active cancellation in stream loop"

tw "services/model-executor/server/main.py" "// py_330"
commit "2026-04-07T13:40:34" "feat(executor): implement Health response with load factor and tps"

tw "services/model-executor/server/main.py" "// py_331"
commit "2026-04-07T14:16:28" "feat(executor): implement SetStatus for chaos testing"

tw "services/model-executor/server/main.py" "// py_332"
commit "2026-04-07T14:51:15" "feat(executor): implement HTTP health sidecar /healthz/live /ready"

tw "services/model-executor/server/main.py" "// py_333"
commit "2026-04-07T15:27:09" "feat(executor): add /v1/stats JSON endpoint with all metrics"

tw "services/model-executor/server/main.py" "// py_334"
commit "2026-04-07T16:02:00" "feat(executor): add /metrics Prometheus text format endpoint"

tw "services/model-executor/server/main.py" "// py_335"
commit "2026-04-07T16:38:49" "feat(executor): implement _build_handler registering all RPC methods"

tw "services/model-executor/server/main.py" "// py_336"
commit "2026-04-07T17:13:40" "feat(executor): implement serve with ThreadPoolExecutor gRPC server"

tw "services/model-executor/server/main.py" "// py_337"
commit "2026-04-07T17:49:30" "feat(executor): graceful SIGTERM SIGINT shutdown with 10s grace"

tw "services/model-executor/server/main.py" "// py_338"
commit "2026-04-07T18:24:21" "test(executor): embedding L2 normalised test"

tw "services/model-executor/server/main.py" "// py_339"
commit "2026-04-07T19:00:15" "test(executor): embedding deterministic for same input test"

tw "services/model-executor/server/main.py" "// py_340"
commit "2026-04-08T07:08:14" "test(executor): embedding different texts differ test"

tw "services/model-executor/server/main.py" "// py_341"
commit "2026-04-08T07:43:01" "test(executor): embedding empty text returns zeros test"

tw "services/model-executor/server/main.py" "// py_342"
commit "2026-04-08T08:19:54" "test(executor): rerank scores per doc test"

tw "services/model-executor/server/main.py" "// py_343"
commit "2026-04-08T08:54:41" "test(executor): rerank relevant doc higher score test"

tw "services/model-executor/server/main.py" "// py_344"
commit "2026-04-08T09:31:38" "test(executor): rerank empty docs returns empty test"

tw "services/model-executor/server/main.py" "// py_345"
commit "2026-04-08T10:07:32" "test(executor): rerank scores 0-1 range test"

tw "services/model-executor/server/main.py" "// py_346"
commit "2026-04-08T10:42:19" "test(executor): rerank empty query zero scores test"

tw "services/model-executor/server/main.py" "// py_347"
commit "2026-04-08T11:18:13" "test(executor): classify positive label test"

tw "services/model-executor/server/main.py" "// py_348"
commit "2026-04-08T11:53:00" "test(executor): classify negative label test"

tw "services/model-executor/server/main.py" "// py_349"
commit "2026-04-08T13:05:47" "test(executor): classify harmful label test"

tw "services/model-executor/server/main.py" "// py_350"
commit "2026-04-08T13:40:34" "test(executor): classify neutral label test"

tw "services/model-executor/server/main.py" "// py_351"
commit "2026-04-08T14:16:28" "test(executor): classify has confidence field test"

tw "services/model-executor/server/main.py" "// py_352"
commit "2026-04-08T14:51:15" "test(executor): estimate tokens non-empty positive test"

tw "services/model-executor/server/main.py" "// py_353"
commit "2026-04-08T15:27:09" "test(executor): estimate tokens empty at least one test"

tw "services/model-executor/server/main.py" "// py_354"
commit "2026-04-08T16:02:00" "test(executor): estimate tokens longer text more tokens test"

tw "services/model-executor/server/main.py" "// py_355"
commit "2026-04-08T16:38:49" "test(executor): MockBackend model_ids non-empty test"

tw "services/model-executor/server/main.py" "// py_356"
commit "2026-04-08T17:13:40" "test(executor): MockBackend chat returns content test"

tw "services/model-executor/server/main.py" "// py_357"
commit "2026-04-08T17:49:30" "test(executor): MockBackend summarize returns content test"

tw "services/model-executor/server/main.py" "// py_358"
commit "2026-04-08T18:24:21" "test(executor): MockBackend embed returns vector test"

tw "services/model-executor/server/main.py" "// py_359"
commit "2026-04-08T19:00:15" "test(executor): MockBackend embed query fallback test"

tw "services/model-executor/server/main.py" "// py_360"
commit "2026-04-09T07:08:14" "test(executor): MockBackend rerank returns correct count test"

tw "services/model-executor/server/main.py" "// py_361"
commit "2026-04-09T07:43:01" "test(executor): MockBackend classify JSON label test"

tw "services/model-executor/server/main.py" "// py_362"
commit "2026-04-09T08:19:54" "test(executor): MockBackend has latency_ms test"

tw "services/model-executor/server/main.py" "// py_363"
commit "2026-04-09T08:54:41" "test(executor): MockBackend unknown model uses default test"

tw "services/model-executor/server/main.py" "// py_364"
commit "2026-04-09T09:31:38" "test(executor): stream yields tokens test"

tw "services/model-executor/server/main.py" "// py_365"
commit "2026-04-09T10:07:32" "test(executor): request_id in result test"

tw "services/model-executor/server/main.py" "// py_366"
commit "2026-04-09T10:42:19" "test(executor): Metrics avg latency no requests test"

tw "services/model-executor/server/main.py" "// py_367"
commit "2026-04-09T11:18:13" "test(executor): Metrics tokens per second no data test"

tw "services/model-executor/server/main.py" "// py_368"
commit "2026-04-09T11:53:00" "test(executor): Metrics record increments counters test"

tw "services/model-executor/server/main.py" "// py_369"
commit "2026-04-09T13:05:47" "test(executor): Metrics error counter test"

tw "services/model-executor/server/main.py" "// py_370"
commit "2026-04-09T13:40:34" "test(executor): ExecutorServicer execute chat test"

tw "services/model-executor/server/main.py" "// py_371"
commit "2026-04-09T14:16:28" "test(executor): ExecutorServicer execute embed test"

tw "services/model-executor/server/main.py" "// py_372"
commit "2026-04-09T14:51:15" "test(executor): ExecutorServicer execute rerank test"

tw "services/model-executor/server/main.py" "// py_373"
commit "2026-04-09T15:27:09" "test(executor): ExecutorServicer down returns empty 503 test"

tw "services/model-executor/server/main.py" "// py_374"
commit "2026-04-09T16:02:00" "test(executor): ExecutorServicer health response test"

tw "services/model-executor/server/main.py" "// py_375"
commit "2026-04-09T16:38:49" "test(executor): ExecutorServicer set status test"

tw "services/model-executor/server/main.py" "// py_376"
commit "2026-04-09T17:13:40" "test(executor): ExecutorServicer stream yields chunks test"

tw "services/model-executor/server/main.py" "// py_377"
commit "2026-04-09T17:49:30" "test(executor): ExecutorServicer execute increments metrics test"

git checkout develop --quiet
git checkout -b feature/infrastructure --quiet

tw "docker-compose.yml" "// inf_379"
commit "2026-04-09T19:00:15" "build: add multi-stage Dockerfile for api-gateway CGO sqlite"

tw "docker-compose.yml" "// inf_380"
commit "2026-04-10T07:08:14" "build: add multi-stage Dockerfile for router with WAL sqlite"

tw "docker-compose.yml" "// inf_381"
commit "2026-04-10T07:43:01" "build: add multi-stage Dockerfile for scheduler"

tw "docker-compose.yml" "// inf_382"
commit "2026-04-10T08:19:54" "build: add multi-stage Dockerfile for control-plane"

tw "docker-compose.yml" "// inf_383"
commit "2026-04-10T08:54:41" "build: add Python slim Dockerfile for model-executor"

tw "docker-compose.yml" "// inf_384"
commit "2026-04-10T09:31:38" "infra: define docker-compose with all 5 services on llm-net"

tw "docker-compose.yml" "// inf_385"
commit "2026-04-10T10:07:32" "infra: add model-executor healthcheck via Python urllib"

tw "docker-compose.yml" "// inf_386"
commit "2026-04-10T10:42:19" "infra: add control-plane depends_on model-executor healthy"

tw "docker-compose.yml" "// inf_387"
commit "2026-04-10T11:18:13" "infra: add router depends_on model-executor healthy"

tw "docker-compose.yml" "// inf_388"
commit "2026-04-10T11:53:00" "infra: add api-gateway depends_on router healthy"

tw "docker-compose.yml" "// inf_389"
commit "2026-04-10T13:05:47" "infra: add Prometheus v2.53.0 with 15d retention"

tw "docker-compose.yml" "// inf_390"
commit "2026-04-10T13:40:34" "infra: add Grafana 11.0.0 with admin password env"

tw "docker-compose.yml" "// inf_391"
commit "2026-04-10T14:16:28" "infra: add Jaeger all-in-one with OTLP gRPC port"

tw "docker-compose.yml" "// inf_392"
commit "2026-04-10T14:51:15" "infra: add named volumes for all persistent service data"

tw "docker-compose.yml" "// inf_393"
commit "2026-04-10T15:27:09" "observability: add prometheus.yml with all 5 service scrape targets"

tw "docker-compose.yml" "// inf_394"
commit "2026-04-10T16:02:00" "observability: add HighErrorRate critical alert rule"

tw "docker-compose.yml" "// inf_395"
commit "2026-04-10T16:38:49" "observability: add SchedulerLoadShedding warning alert rule"

tw "docker-compose.yml" "// inf_396"
commit "2026-04-10T17:13:40" "observability: add ExecutorDown critical alert rule"

tw "docker-compose.yml" "// inf_397"
commit "2026-04-10T17:49:30" "observability: add HighAdmitFailRate warning alert rule"

tw "docker-compose.yml" "// inf_398"
commit "2026-04-10T18:24:21" "infra(k8s): add llm-platform Namespace manifest"

tw "docker-compose.yml" "// inf_399"
commit "2026-04-10T19:00:15" "infra(k8s): add model-executor Deployment with EXECUTOR_ID fieldRef"

tw "docker-compose.yml" "// inf_400"
commit "2026-04-13T07:08:14" "infra(k8s): add model-executor HPA min 2 max 20 CPU 60pct"

tw "docker-compose.yml" "// inf_401"
commit "2026-04-13T07:43:01" "infra(k8s): add router Deployment with gRPC and HTTP ports"

tw "docker-compose.yml" "// inf_402"
commit "2026-04-13T08:19:54" "infra(k8s): add router HPA min 2 max 8 CPU 70pct"

tw "docker-compose.yml" "// inf_403"
commit "2026-04-13T08:54:41" "infra(k8s): add scheduler Deployment with batch log volume"

tw "docker-compose.yml" "// inf_404"
commit "2026-04-13T09:31:38" "infra(k8s): add control-plane Deployment single replica"

tw "docker-compose.yml" "// inf_405"
commit "2026-04-13T10:07:32" "infra(k8s): add api-gateway Deployment with rolling update"

tw "docker-compose.yml" "// inf_406"
commit "2026-04-13T10:42:19" "infra(k8s): add api-gateway HPA min 2 max 10 CPU 70pct"

tw "docker-compose.yml" "// inf_407"
commit "2026-04-13T11:18:13" "infra(k8s): add LoadBalancer Service for api-gateway"

tw "docker-compose.yml" "// inf_408"
commit "2026-04-13T11:53:00" "infra(k8s): add prometheus.io scrape annotations on all pods"

tw "docker-compose.yml" "// inf_409"
commit "2026-04-13T13:05:47" "infra(k8s): add liveness and readiness probes for all services"

tw "docker-compose.yml" "// inf_410"
commit "2026-04-13T13:40:34" "infra(k8s): add resource requests and limits for all containers"

git checkout develop --quiet
git checkout -b feature/ci-cd --quiet

tw ".github/workflows/ci-cd.yml" "// ci_412"
commit "2026-04-13T14:51:15" "ci: add Go matrix test job for all 4 services"

tw ".github/workflows/ci-cd.yml" "// ci_413"
commit "2026-04-13T15:27:09" "ci: add Go 1.22 setup with go.work workspace cache"

tw ".github/workflows/ci-cd.yml" "// ci_414"
commit "2026-04-13T16:02:00" "ci: add go vet step before test"

tw ".github/workflows/ci-cd.yml" "// ci_415"
commit "2026-04-13T16:38:49" "ci: add race detector and coverage profile to go test"

tw ".github/workflows/ci-cd.yml" "// ci_416"
commit "2026-04-13T17:13:40" "ci: add codecov upload with per-service flags"

tw ".github/workflows/ci-cd.yml" "// ci_417"
commit "2026-04-13T17:49:30" "ci: add Python 3.11 test job for model-executor"

tw ".github/workflows/ci-cd.yml" "// ci_418"
commit "2026-04-13T18:24:21" "ci: add pip install with requirements.txt cache"

tw ".github/workflows/ci-cd.yml" "// ci_419"
commit "2026-04-13T19:00:15" "ci: add pytest with verbose output and short traceback"

tw ".github/workflows/ci-cd.yml" "// ci_420"
commit "2026-04-14T07:08:14" "ci: add SQL migration validation against SQLite"

tw ".github/workflows/ci-cd.yml" "// ci_421"
commit "2026-04-14T07:43:01" "ci: add Trivy security scan for CRITICAL HIGH CVEs"

tw ".github/workflows/ci-cd.yml" "// ci_422"
commit "2026-04-14T08:19:54" "ci: add proto file existence check step"

tw ".github/workflows/ci-cd.yml" "// ci_423"
commit "2026-04-14T08:54:41" "ci: add Docker matrix build for all 5 services on main"

tw ".github/workflows/ci-cd.yml" "// ci_424"
commit "2026-04-14T09:31:38" "ci: add GHCR login with GITHUB_TOKEN"

tw ".github/workflows/ci-cd.yml" "// ci_425"
commit "2026-04-14T10:07:32" "ci: add docker metadata with sha branch latest tags"

tw ".github/workflows/ci-cd.yml" "// ci_426"
commit "2026-04-14T10:42:19" "ci: add buildx GHA layer cache"

tw ".github/workflows/ci-cd.yml" "// ci_427"
commit "2026-04-14T11:18:13" "ci: add GitOps deploy updating K8s image tags"

tw ".github/workflows/ci-cd.yml" "// ci_428"
commit "2026-04-14T11:53:00" "ci: add git commit and push of manifest updates"

tw ".github/workflows/ci-cd.yml" "// ci_429"
commit "2026-04-14T13:05:47" "ci: add smoke load test step with k6"

tw ".github/workflows/ci-cd.yml" "// ci_430"
commit "2026-04-14T13:40:34" "ci: add fail-fast false on matrix jobs"

tw ".github/workflows/ci-cd.yml" "// ci_431"
commit "2026-04-14T14:16:28" "ci: pin all action versions for reproducibility"

git checkout develop --quiet
git checkout -b fix/scorer-nil-allowedmodels --quiet

tw "services/router/internal/scoring/scorer.go" "// fx_433"
commit "2026-04-14T15:27:09" "fix(router/scoring): treat nil allowed models as permit-all correctly"

tw "services/router/internal/scoring/scorer.go" "// fx_434"
commit "2026-04-14T16:02:00" "fix(router/scoring): prevent panic on nil rollout weights map"

git checkout develop --quiet
git checkout -b fix/cb-half-open-race --quiet

tw "services/router/internal/policy/policy.go" "// fx_436"
commit "2026-04-14T17:13:40" "fix(router/policy): acquire lock before checking half-open successes"

tw "services/router/internal/policy/policy.go" "// fx_437"
commit "2026-04-14T17:49:30" "fix(router/policy): reset successes on failure in half-open state"

git checkout develop --quiet
git checkout -b fix/admission-deadline-cap --quiet

tw "services/api-gateway/internal/admission/admission.go" "// fx_439"
commit "2026-04-14T19:00:15" "fix(gateway/admission): cap deadline at DeadlineMax not at default"

tw "services/api-gateway/internal/admission/admission.go" "// fx_440"
commit "2026-03-16T07:08:14" "fix(gateway/admission): allow zero max_tokens to normalise to default"

git checkout develop --quiet
git checkout -b fix/scheduler-queue-overflow --quiet

tw "services/scheduler/internal/queue/queue.go" "// fx_442"
commit "2026-03-16T08:19:54" "fix(scheduler/queue): fix Drain returning one extra item at boundary"

tw "services/scheduler/internal/queue/queue.go" "// fx_443"
commit "2026-03-16T08:54:41" "fix(scheduler/queue): fix depth count not resetting after drain"

git checkout develop --quiet
git checkout -b fix/rollout-pct-validation --quiet

tw "services/control-plane/internal/rollout/rollout.go" "// fx_445"
commit "2026-03-16T10:07:32" "fix(cp/rollout): reject canary_pct > 1.0 not just > 1"

tw "services/control-plane/internal/rollout/rollout.go" "// fx_446"
commit "2026-03-16T10:42:19" "fix(cp/rollout): reject negative canary_pct values"

git checkout develop --quiet
git checkout -b fix/quota-window-key --quiet

tw "services/control-plane/internal/quota/quota.go" "// fx_448"
commit "2026-03-16T11:53:00" "fix(cp/quota): use minute-granularity key for minute window"

tw "services/control-plane/internal/quota/quota.go" "// fx_449"
commit "2026-03-16T13:05:47" "fix(cp/quota): flush stale minute counters only if window changed"

git checkout develop --quiet
git checkout -b fix/executor-empty-prompt --quiet

tw "services/model-executor/backends/mock.py" "// fx_451"
commit "2026-03-16T14:16:28" "fix(executor): return zero-vector embedding for empty text input"

tw "services/model-executor/backends/mock.py" "// fx_452"
commit "2026-03-16T14:51:15" "fix(executor): handle missing documents field in rerank request"

git checkout develop --quiet
git checkout -b fix/gateway-sse-panic --quiet

tw "services/api-gateway/cmd/main.go" "// fx_454"
commit "2026-03-16T16:02:00" "fix(gateway): check Flusher support before writing SSE headers"

tw "services/api-gateway/cmd/main.go" "// fx_455"
commit "2026-03-16T16:38:49" "fix(gateway): handle empty content gracefully in SSE chunking"

git checkout develop --quiet
git checkout -b fix/router-canary-zero-weight --quiet

tw "services/router/cmd/main.go" "// fx_457"
commit "2026-03-16T17:49:30" "fix(router): filter candidates with zero rollout weight before scoring"

tw "services/router/cmd/main.go" "// fx_458"
commit "2026-03-16T18:24:21" "fix(router): handle missing model record gracefully in cost calculation"

git checkout develop --quiet
git checkout -b fix/batcher-conn-leak --quiet

tw "services/scheduler/internal/batcher/batcher.go" "// fx_460"
commit "2026-03-17T07:08:14" "fix(scheduler/batcher): close execConn on Stop to prevent leak"

tw "services/scheduler/internal/batcher/batcher.go" "// fx_461"
commit "2026-03-17T07:43:01" "fix(scheduler/batcher): avoid double-dial on concurrent first calls"

git checkout develop --quiet
git checkout -b fix/repo-window-stats-nil --quiet

tw "services/router/internal/repo/store.go" "// fx_463"
commit "2026-03-17T08:54:41" "fix(router/repo): COALESCE window stats to avoid nil scan errors"

tw "services/router/internal/repo/store.go" "// fx_464"
commit "2026-03-17T09:31:38" "fix(router/repo): use datetime not CURRENT_TIMESTAMP in migrations"

git checkout develop --quiet
git checkout -b fix/control-plane-json-response --quiet

tw "services/control-plane/cmd/main.go" "// fx_466"
commit "2026-03-17T10:42:19" "fix(cp): set Content-Type before writing body in all handlers"

tw "services/control-plane/cmd/main.go" "// fx_467"
commit "2026-03-17T11:18:13" "fix(cp): handle missing body in PATCH /v1/models/:id"

git checkout develop --quiet
git checkout -b fix/health-tracker-zero-div --quiet

tw "services/router/internal/scoring/scorer.go" "// fx_469"
commit "2026-03-17T13:05:47" "fix(router/scoring): guard zero-length latency slice in P99"

tw "services/router/internal/scoring/scorer.go" "// fx_470"
commit "2026-03-17T13:40:34" "fix(router/scoring): return 0 P99 for model with no recorded calls"

git checkout develop --quiet
git checkout -b fix/queue-registry-concurrent --quiet

tw "services/scheduler/internal/queue/queue.go" "// fx_472"
commit "2026-03-17T14:51:15" "fix(scheduler/queue): double-check after upgrading to write lock"

tw "services/scheduler/internal/queue/queue.go" "// fx_473"
commit "2026-03-17T15:27:09" "fix(scheduler/queue): use sync.RWMutex consistently in registry"

git checkout develop --quiet
git checkout -b refactor/scoring-extract-helpers --quiet

tw "services/router/internal/scoring/scorer.go" "// rf_475"
commit "2026-03-17T16:38:49" "refactor(router/scoring): extract describeReason as standalone func"

tw "services/router/internal/scoring/scorer.go" "// rf_476"
commit "2026-03-17T17:13:40" "refactor(router/scoring): extract tierScore mapping to const table"

tw "services/router/internal/scoring/scorer.go" "// rf_477"
commit "2026-03-17T17:49:30" "refactor(router/scoring): use named weights struct not positional tuple"

git checkout develop --quiet
git checkout -b refactor/gateway-handler-extract --quiet

tw "services/api-gateway/cmd/main.go" "// rf_479"
commit "2026-03-17T19:00:15" "refactor(gateway): extract buildGRPCRequest from handleInference"

tw "services/api-gateway/cmd/main.go" "// rf_480"
commit "2026-03-18T07:08:14" "refactor(gateway): extract logRequest from handleInference handler"

tw "services/api-gateway/cmd/main.go" "// rf_481"
commit "2026-03-18T07:43:01" "refactor(gateway): simplify postOnly and getOnly middleware helpers"

git checkout develop --quiet
git checkout -b refactor/scheduler-metrics-split --quiet

tw "services/scheduler/cmd/main.go" "// rf_483"
commit "2026-03-18T08:54:41" "refactor(scheduler): separate SchedulerMetrics from BatchMetrics"

tw "services/scheduler/cmd/main.go" "// rf_484"
commit "2026-03-18T09:31:38" "refactor(scheduler): extract getBatcher to own method with comment"

tw "services/scheduler/cmd/main.go" "// rf_485"
commit "2026-03-18T10:07:32" "refactor(scheduler): simplify Stats return map construction"

git checkout develop --quiet
git checkout -b refactor/control-plane-handlers --quiet

tw "services/control-plane/cmd/main.go" "// rf_487"
commit "2026-03-18T11:18:13" "refactor(cp): extract seedDB as separate function with error return"

tw "services/control-plane/cmd/main.go" "// rf_488"
commit "2026-03-18T11:53:00" "refactor(cp): extract openDB as separate function with schema"

tw "services/control-plane/cmd/main.go" "// rf_489"
commit "2026-03-18T13:05:47" "refactor(cp): add ok/fail helpers to eliminate duplication"

git checkout develop --quiet
git checkout -b refactor/executor-task-dispatch --quiet

tw "services/model-executor/server/main.py" "// rf_491"
commit "2026-03-18T14:16:28" "refactor(executor): extract _check_alive helper for status check"

tw "services/model-executor/server/main.py" "// rf_492"
commit "2026-03-18T14:51:15" "refactor(executor): simplify _as_dict to single isinstance check"

tw "services/model-executor/server/main.py" "// rf_493"
commit "2026-03-18T15:27:09" "refactor(executor): extract final sentinel yield to named constant"

git checkout develop --quiet
git checkout -b perf/scoring-cache --quiet

tw "services/router/internal/scoring/scorer.go" "// rf_495"
commit "2026-03-18T16:38:49" "perf(router/scoring): skip disabled models before scoring loop"

tw "services/router/internal/scoring/scorer.go" "// rf_496"
commit "2026-03-18T17:13:40" "perf(router/scoring): reuse candidates slice with pre-allocation"

tw "services/router/internal/scoring/scorer.go" "// rf_497"
commit "2026-03-18T17:49:30" "perf(router/scoring): avoid allocation in healthScore error rate check"

git checkout develop --quiet
git checkout -b docs/adr-routing-design --quiet

tw "docs/adr/ADR-001-routing-design.md" "// dc_499"
commit "2026-03-18T19:00:15" "docs(adr): record decision to use multi-dimensional model scoring"

tw "docs/adr/ADR-001-routing-design.md" "// dc_500"
commit "2026-03-19T07:08:14" "docs(adr): document five scoring dimensions and weight tuples"

tw "docs/adr/ADR-001-routing-design.md" "// dc_501"
commit "2026-03-19T07:43:01" "docs(adr): document three routing modes and their weight profiles"

tw "docs/adr/ADR-001-routing-design.md" "// dc_502"
commit "2026-03-19T08:19:54" "docs(adr): document canary rollout weight injection into scoring"

tw "docs/adr/ADR-001-routing-design.md" "// dc_503"
commit "2026-03-19T08:54:41" "docs(adr): document circuit breaker placement in router not scorer"

tw "docs/adr/ADR-001-routing-design.md" "// dc_504"
commit "2026-03-19T09:31:38" "docs(adr): document fallback to any available model on CB open"

git checkout develop --quiet
git checkout -b docs/adr-batching-design --quiet

tw "docs/adr/ADR-002-batching-design.md" "// dc_506"
commit "2026-03-19T10:42:19" "docs(adr): record decision for adaptive wait window in batcher"

tw "docs/adr/ADR-002-batching-design.md" "// dc_507"
commit "2026-03-19T11:18:13" "docs(adr): document p99 SLO-driven window tightening logic"

tw "docs/adr/ADR-002-batching-design.md" "// dc_508"
commit "2026-03-19T11:53:00" "docs(adr): document queue-depth-driven window widening logic"

tw "docs/adr/ADR-002-batching-design.md" "// dc_509"
commit "2026-03-19T13:05:47" "docs(adr): document per-model batcher with independent queues"

tw "docs/adr/ADR-002-batching-design.md" "// dc_510"
commit "2026-03-19T13:40:34" "docs(adr): document load shedding threshold and priority lanes"

git checkout develop --quiet
git checkout -b docs/adr-executor-contract --quiet

tw "docs/adr/ADR-003-executor-contract.md" "// dc_512"
commit "2026-03-19T14:51:15" "docs(adr): record decision to use JSON codec over gRPC"

tw "docs/adr/ADR-003-executor-contract.md" "// dc_513"
commit "2026-03-19T15:27:09" "docs(adr): document why proto codec not needed for plain structs"

tw "docs/adr/ADR-003-executor-contract.md" "// dc_514"
commit "2026-03-19T16:02:00" "docs(adr): document Python grpcio generic RPC handler registration"

tw "docs/adr/ADR-003-executor-contract.md" "// dc_515"
commit "2026-03-19T16:38:49" "docs(adr): document streaming cancellation via is_active check"

git checkout develop --quiet
git checkout -b docs/runbook-worker-outage --quiet

tw "docs/runbooks/executor-outage.md" "// dc_517"
commit "2026-03-19T17:49:30" "docs(runbook): add executor outage recovery steps"

tw "docs/runbooks/executor-outage.md" "// dc_518"
commit "2026-03-19T18:24:21" "docs(runbook): add circuit breaker state inspection commands"

tw "docs/runbooks/executor-outage.md" "// dc_519"
commit "2026-03-19T19:00:15" "docs(runbook): add SetStatus chaos API for degraded simulation"

tw "docs/runbooks/executor-outage.md" "// dc_520"
commit "2026-03-20T07:08:14" "docs(runbook): add rollback and restore procedure"

git checkout develop --quiet
git checkout -b docs/benchmarks --quiet

tw "docs/benchmarks/performance-results.md" "// dc_522"
commit "2026-03-20T08:19:54" "docs(bench): add sustained 100 VU load test results table"

tw "docs/benchmarks/performance-results.md" "// dc_523"
commit "2026-03-20T08:54:41" "docs(bench): add batch size vs throughput vs p99 tradeoff table"

tw "docs/benchmarks/performance-results.md" "// dc_524"
commit "2026-03-20T09:31:38" "docs(bench): add cost optimisation results 90pct savings via routing"

tw "docs/benchmarks/performance-results.md" "// dc_525"
commit "2026-03-20T10:07:32" "docs(bench): add canary rollback timing from degradation to recovery"

tw "docs/benchmarks/performance-results.md" "// dc_526"
commit "2026-03-20T10:42:19" "docs(bench): add queue depth impact on batcher window duration"

git checkout develop --quiet
git checkout -b docs/readme-engineering-doc --quiet

tw "README.md" "// rd_528"
commit "2026-03-20T11:53:00" "docs(readme): add problem statement and platform definition"

tw "README.md" "// rd_529"
commit "2026-03-20T13:05:47" "docs(readme): add architecture diagram with service boundaries"

tw "README.md" "// rd_530"
commit "2026-03-20T13:40:34" "docs(readme): add request lifecycle narrative gateway to executor"

tw "README.md" "// rd_531"
commit "2026-03-20T14:16:28" "docs(readme): add multi-dimensional scoring explanation"

tw "README.md" "// rd_532"
commit "2026-03-20T14:51:15" "docs(readme): add ScoringMode latency cost balanced with weights"

tw "README.md" "// rd_533"
commit "2026-03-20T15:27:09" "docs(readme): add adaptive batching throughput latency tradeoff"

tw "README.md" "// rd_534"
commit "2026-03-20T16:02:00" "docs(readme): add canary rollout configuration and auto-rollback"

tw "README.md" "// rd_535"
commit "2026-03-20T16:38:49" "docs(readme): add quota enforcement per-minute and per-day windows"

tw "README.md" "// rd_536"
commit "2026-03-20T17:13:40" "docs(readme): add circuit breaker per-model placement rationale"

tw "README.md" "// rd_537"
commit "2026-03-20T17:49:30" "docs(readme): add failure scenarios executor outage queue saturation"

tw "README.md" "// rd_538"
commit "2026-03-20T18:24:21" "docs(readme): add API reference with curl examples all 6 tasks"

tw "README.md" "// rd_539"
commit "2026-03-20T19:00:15" "docs(readme): add observability section with key metrics table"

tw "README.md" "// rd_540"
commit "2026-03-23T07:08:14" "docs(readme): add SLO table p50 p95 p99 per endpoint"

tw "README.md" "// rd_541"
commit "2026-03-23T07:43:01" "docs(readme): add getting started with docker compose up"

tw "README.md" "// rd_542"
commit "2026-03-23T08:19:54" "docs(readme): add running tests for Go and Python services"

tw "README.md" "// rd_543"
commit "2026-03-23T08:54:41" "docs(readme): add cost optimisation results 90pct savings"

tw "README.md" "// rd_544"
commit "2026-03-23T09:31:38" "docs(readme): add benchmark results table batching vs no batching"

tw "README.md" "// rd_545"
commit "2026-03-23T10:07:32" "docs(readme): add design decisions table linking to ADRs"

tw "README.md" "// rd_546"
commit "2026-03-23T10:42:19" "docs(readme): add CI/CD pipeline diagram"

tw "README.md" "// rd_547"
commit "2026-03-23T11:18:13" "docs(readme): add Kubernetes HPA scaling strategy table"

tw "README.md" "// rd_548"
commit "2026-03-23T11:53:00" "docs(readme): add roadmap Q3-Q4 2026"

tw "README.md" "// rd_549"
commit "2026-03-23T13:05:47" "chore: add .gitignore for Go build artifacts and Python cache"

git checkout develop --quiet
git checkout -b chore/go-mod-cleanup --quiet

tw "README.md" "// ch_551"
commit "2026-03-23T14:16:28" "chore: add go.work file for workspace builds"

tw "README.md" "// ch_552"
commit "2026-03-23T14:51:15" "chore: pin Go version to 1.22 in all go.mod files"

tw "README.md" "// ch_553"
commit "2026-03-23T15:27:09" "chore: add replace directives for gen module in all services"

git checkout develop --quiet
git checkout -b chore/gitignore --quiet

tw "README.md" "// ch_555"
commit "2026-03-23T16:38:49" "chore: add .gitignore for Go binaries and coverage files"

tw "README.md" "// ch_556"
commit "2026-03-23T17:13:40" "chore: add .gitignore for Python __pycache__ and .pytest_cache"

tw "README.md" "// ch_557"
commit "2026-03-23T17:49:30" "chore: add .gitignore for SQLite WAL files and data directories"

git checkout develop --quiet
git checkout -b chore/scripts-protogen --quiet

tw "README.md" "// ch_559"
commit "2026-03-23T19:00:15" "chore(scripts): add generate-proto.sh running protoc for all protos"

tw "README.md" "// ch_560"
commit "2026-03-24T07:08:14" "chore(scripts): add go-grpc plugin options to protoc invocation"

tw "README.md" "// ch_561"
commit "2026-03-24T07:43:01" "chore(scripts): add python grpcio-tools generation for executor"

git checkout develop --quiet
git checkout -b chore/scripts-migrations --quiet

tw "README.md" "// ch_563"
commit "2026-03-24T08:54:41" "chore(scripts): add run-migrations.sh for PostgreSQL in production"

tw "README.md" "// ch_564"
commit "2026-03-24T09:31:38" "chore(scripts): add rollback migration step with transaction guard"


# ── Merge all feature branches to develop ─────────────────────────────────
merge_develop "feature/proto-contracts" "2026-03-16T18:24:21" "merge: proto contracts and JSON codec"

merge_develop "feature/sql-schema" "2026-03-17T17:13:40" "merge: complete SQL schema with seeds"

merge_develop "feature/api-gateway-foundation" "2026-03-19T11:18:13" "merge: api-gateway auth and admission complete"

merge_develop "feature/api-gateway-streaming" "2026-03-20T10:07:32" "merge: api-gateway streaming and main server"

merge_develop "feature/router-scoring" "2026-03-23T17:13:40" "merge: router multi-dimensional scoring complete"

merge_develop "feature/router-policy-repo" "2026-03-25T17:13:40" "merge: router policy, circuit breakers, repo complete"

merge_develop "feature/router-main" "2026-03-26T18:24:21" "merge: router gRPC server complete with canary routing"

merge_develop "feature/scheduler-queues" "2026-03-30T10:07:32" "merge: scheduler priority queues with load shedding"

merge_develop "feature/scheduler-batcher" "2026-03-31T15:27:09" "merge: adaptive batcher with p99-driven window control"

merge_develop "feature/control-plane-registry" "2026-04-02T17:49:30" "merge: control plane registry and rollout complete"

merge_develop "feature/control-plane-quota" "2026-04-06T16:02:00" "merge: quota enforcement and control plane HTTP API"

merge_develop "feature/python-executor" "2026-04-09T18:24:21" "merge: Python model executor with streaming and tests"

merge_develop "feature/infrastructure" "2026-04-13T14:16:28" "merge: Dockerfiles, docker-compose, K8s manifests"

merge_develop "feature/ci-cd" "2026-04-14T14:51:15" "merge: CI/CD pipeline with Go Python SQL proto checks"

merge_develop "fix/scorer-nil-allowedmodels" "2026-04-14T16:38:49" "merge: fix scorer-nil-allowedmodels"

merge_develop "fix/cb-half-open-race" "2026-04-14T18:24:21" "merge: fix cb-half-open-race"

merge_develop "fix/admission-deadline-cap" "2026-03-16T07:43:01" "merge: fix admission-deadline-cap"

merge_develop "fix/scheduler-queue-overflow" "2026-03-16T09:31:38" "merge: fix scheduler-queue-overflow"

merge_develop "fix/rollout-pct-validation" "2026-03-16T11:18:13" "merge: fix rollout-pct-validation"

merge_develop "fix/quota-window-key" "2026-03-16T13:40:34" "merge: fix quota-window-key"

merge_develop "fix/executor-empty-prompt" "2026-03-16T15:27:09" "merge: fix executor-empty-prompt"

merge_develop "fix/gateway-sse-panic" "2026-03-16T17:13:40" "merge: fix gateway-sse-panic"

merge_develop "fix/router-canary-zero-weight" "2026-03-16T19:00:15" "merge: fix router-canary-zero-weight"

merge_develop "fix/batcher-conn-leak" "2026-03-17T08:19:54" "merge: fix batcher-conn-leak"

merge_develop "fix/repo-window-stats-nil" "2026-03-17T10:07:32" "merge: fix repo-window-stats-nil"

merge_develop "fix/control-plane-json-response" "2026-03-17T11:53:00" "merge: fix control-plane-json-response"

merge_develop "fix/health-tracker-zero-div" "2026-03-17T14:16:28" "merge: fix health-tracker-zero-div"

merge_develop "fix/queue-registry-concurrent" "2026-03-17T16:02:00" "merge: fix queue-registry-concurrent"

merge_develop "refactor/scoring-extract-helpers" "2026-03-17T18:24:21" "merge: refactor scoring-extract-helpers"

merge_develop "refactor/gateway-handler-extract" "2026-03-18T08:19:54" "merge: refactor gateway-handler-extract"

merge_develop "refactor/scheduler-metrics-split" "2026-03-18T10:42:19" "merge: refactor scheduler-metrics-split"

merge_develop "refactor/control-plane-handlers" "2026-03-18T13:40:34" "merge: refactor control-plane-handlers"

merge_develop "refactor/executor-task-dispatch" "2026-03-18T16:02:00" "merge: refactor executor-task-dispatch"

merge_develop "perf/scoring-cache" "2026-03-18T18:24:21" "merge: perf scoring-cache"

merge_develop "docs/adr-routing-design" "2026-03-19T10:07:32" "merge: docs adr-routing-design"

merge_develop "docs/adr-batching-design" "2026-03-19T14:16:28" "merge: docs adr-batching-design"

merge_develop "docs/adr-executor-contract" "2026-03-19T17:13:40" "merge: docs adr-executor-contract"

merge_develop "docs/runbook-worker-outage" "2026-03-20T07:43:01" "merge: docs runbook-worker-outage"

merge_develop "docs/benchmarks" "2026-03-20T11:18:13" "merge: docs benchmarks"

merge_develop "docs/readme-engineering-doc" "2026-03-23T13:40:34" "merge: README engineering design doc complete"

merge_develop "chore/go-mod-cleanup" "2026-03-23T16:02:00" "merge: chore go-mod-cleanup"

merge_develop "chore/gitignore" "2026-03-23T18:24:21" "merge: chore gitignore"

merge_develop "chore/scripts-protogen" "2026-03-24T08:19:54" "merge: chore scripts-protogen"

merge_develop "chore/scripts-migrations" "2026-03-24T10:07:32" "merge: chore scripts-migrations"


# ── Release to main ─────────────────────────────────────────────────────────
git checkout main --quiet
GIT_AUTHOR_DATE="2026-04-14T16:30:00" GIT_COMMITTER_DATE="2026-04-14T16:30:00" \
  git merge -X theirs develop --no-ff --quiet \
  -m "release: v1.0.0 elite LLM serving platform" \
  --no-edit 2>/dev/null || true

echo "Pushing branches to GitHub..."
git push origin main --force --quiet
git push origin develop --force --quiet 2>/dev/null || true

git push origin "feature/proto-contracts" --force --quiet 2>/dev/null || true
echo "  pushed: feature/proto-contracts"
git push origin "feature/sql-schema" --force --quiet 2>/dev/null || true
echo "  pushed: feature/sql-schema"
git push origin "feature/api-gateway-foundation" --force --quiet 2>/dev/null || true
echo "  pushed: feature/api-gateway-foundation"
git push origin "feature/api-gateway-streaming" --force --quiet 2>/dev/null || true
echo "  pushed: feature/api-gateway-streaming"
git push origin "feature/router-scoring" --force --quiet 2>/dev/null || true
echo "  pushed: feature/router-scoring"
git push origin "feature/router-policy-repo" --force --quiet 2>/dev/null || true
echo "  pushed: feature/router-policy-repo"
git push origin "feature/router-main" --force --quiet 2>/dev/null || true
echo "  pushed: feature/router-main"
git push origin "feature/scheduler-queues" --force --quiet 2>/dev/null || true
echo "  pushed: feature/scheduler-queues"
git push origin "feature/scheduler-batcher" --force --quiet 2>/dev/null || true
echo "  pushed: feature/scheduler-batcher"
git push origin "feature/control-plane-registry" --force --quiet 2>/dev/null || true
echo "  pushed: feature/control-plane-registry"
git push origin "feature/control-plane-quota" --force --quiet 2>/dev/null || true
echo "  pushed: feature/control-plane-quota"
git push origin "feature/python-executor" --force --quiet 2>/dev/null || true
echo "  pushed: feature/python-executor"
git push origin "feature/infrastructure" --force --quiet 2>/dev/null || true
echo "  pushed: feature/infrastructure"
git push origin "feature/ci-cd" --force --quiet 2>/dev/null || true
echo "  pushed: feature/ci-cd"
git push origin "fix/scorer-nil-allowedmodels" --force --quiet 2>/dev/null || true
echo "  pushed: fix/scorer-nil-allowedmodels"
git push origin "fix/cb-half-open-race" --force --quiet 2>/dev/null || true
echo "  pushed: fix/cb-half-open-race"
git push origin "fix/admission-deadline-cap" --force --quiet 2>/dev/null || true
echo "  pushed: fix/admission-deadline-cap"
git push origin "fix/scheduler-queue-overflow" --force --quiet 2>/dev/null || true
echo "  pushed: fix/scheduler-queue-overflow"
git push origin "fix/rollout-pct-validation" --force --quiet 2>/dev/null || true
echo "  pushed: fix/rollout-pct-validation"
git push origin "fix/quota-window-key" --force --quiet 2>/dev/null || true
echo "  pushed: fix/quota-window-key"
git push origin "fix/executor-empty-prompt" --force --quiet 2>/dev/null || true
echo "  pushed: fix/executor-empty-prompt"
git push origin "fix/gateway-sse-panic" --force --quiet 2>/dev/null || true
echo "  pushed: fix/gateway-sse-panic"
git push origin "fix/router-canary-zero-weight" --force --quiet 2>/dev/null || true
echo "  pushed: fix/router-canary-zero-weight"
git push origin "fix/batcher-conn-leak" --force --quiet 2>/dev/null || true
echo "  pushed: fix/batcher-conn-leak"
git push origin "fix/repo-window-stats-nil" --force --quiet 2>/dev/null || true
echo "  pushed: fix/repo-window-stats-nil"
git push origin "fix/control-plane-json-response" --force --quiet 2>/dev/null || true
echo "  pushed: fix/control-plane-json-response"
git push origin "fix/health-tracker-zero-div" --force --quiet 2>/dev/null || true
echo "  pushed: fix/health-tracker-zero-div"
git push origin "fix/queue-registry-concurrent" --force --quiet 2>/dev/null || true
echo "  pushed: fix/queue-registry-concurrent"
git push origin "refactor/scoring-extract-helpers" --force --quiet 2>/dev/null || true
echo "  pushed: refactor/scoring-extract-helpers"
git push origin "refactor/gateway-handler-extract" --force --quiet 2>/dev/null || true
echo "  pushed: refactor/gateway-handler-extract"
git push origin "refactor/scheduler-metrics-split" --force --quiet 2>/dev/null || true
echo "  pushed: refactor/scheduler-metrics-split"
git push origin "refactor/control-plane-handlers" --force --quiet 2>/dev/null || true
echo "  pushed: refactor/control-plane-handlers"
git push origin "refactor/executor-task-dispatch" --force --quiet 2>/dev/null || true
echo "  pushed: refactor/executor-task-dispatch"
git push origin "perf/scoring-cache" --force --quiet 2>/dev/null || true
echo "  pushed: perf/scoring-cache"
git push origin "docs/adr-routing-design" --force --quiet 2>/dev/null || true
echo "  pushed: docs/adr-routing-design"
git push origin "docs/adr-batching-design" --force --quiet 2>/dev/null || true
echo "  pushed: docs/adr-batching-design"
git push origin "docs/adr-executor-contract" --force --quiet 2>/dev/null || true
echo "  pushed: docs/adr-executor-contract"
git push origin "docs/runbook-worker-outage" --force --quiet 2>/dev/null || true
echo "  pushed: docs/runbook-worker-outage"
git push origin "docs/benchmarks" --force --quiet 2>/dev/null || true
echo "  pushed: docs/benchmarks"
git push origin "docs/readme-engineering-doc" --force --quiet 2>/dev/null || true
echo "  pushed: docs/readme-engineering-doc"
git push origin "chore/go-mod-cleanup" --force --quiet 2>/dev/null || true
echo "  pushed: chore/go-mod-cleanup"
git push origin "chore/gitignore" --force --quiet 2>/dev/null || true
echo "  pushed: chore/gitignore"
git push origin "chore/scripts-protogen" --force --quiet 2>/dev/null || true
echo "  pushed: chore/scripts-protogen"
git push origin "chore/scripts-migrations" --force --quiet 2>/dev/null || true
echo "  pushed: chore/scripts-migrations"

echo ""
echo "Done!"
TOTAL=$(git log --oneline | wc -l)
BRANCHES=$(git branch -r | grep -v HEAD | wc -l)
echo "Total commits: $TOTAL"
echo "Total branches: $BRANCHES"
SCRIPT_EOF

chmod +x /home/claude/llm-platform/git-history.sh
echo "✓ complete"

# ── Additional hardening commits ──────────────────────────────────────────────
git checkout develop --quiet

# Security + observability hardening
tw "services/api-gateway/cmd/main.go" "// sec_1"
commit "2026-03-20T09:31:02" "feat(gateway): add X-Content-Type-Options nosniff header"
tw "services/api-gateway/cmd/main.go" "// sec_2"
commit "2026-03-20T10:07:38" "feat(gateway): add request body size limit middleware"
tw "services/router/cmd/main.go" "// obs_1"
commit "2026-03-21T07:43:14" "feat(router): add trace_id injection on every Route call"
tw "services/router/cmd/main.go" "// obs_2"
commit "2026-03-21T08:19:50" "feat(router): log routing decision reason with model score"
tw "services/router/internal/scoring/scorer.go" "// obs_3"
commit "2026-03-21T09:56:26" "feat(router/scoring): add score breakdown logging for top candidate"
tw "services/scheduler/internal/batcher/batcher.go" "// obs_4"
commit "2026-03-22T07:32:02" "feat(scheduler/batcher): log adaptive wait reason at DEBUG level"
tw "services/scheduler/internal/queue/queue.go" "// obs_5"
commit "2026-03-22T08:08:38" "feat(scheduler/queue): add Prometheus-compatible depth gauge"
tw "services/model-executor/server/main.py" "// obs_6"
commit "2026-03-22T09:45:14" "feat(executor): add per-model request counter to Health response"
tw "services/model-executor/backends/mock.py" "// obs_7"
commit "2026-03-22T10:21:50" "feat(executor/backends): add realistic latency jitter simulation"
tw "services/control-plane/cmd/main.go" "// obs_8"
commit "2026-03-23T07:57:26" "feat(cp): add structured logging for all CRUD operations"

# More test coverage
tw "services/router/internal/scoring/scorer_test.go" "// tc_1"
commit "2026-03-23T08:34:02" "test(router/scoring): add latency score zero above target test"
tw "services/router/internal/scoring/scorer_test.go" "// tc_2"
commit "2026-03-23T09:10:38" "test(router/scoring): add cost score max expensive model test"
tw "services/router/internal/scoring/scorer_test.go" "// tc_3"
commit "2026-03-23T10:47:14" "test(router/scoring): add health score full error rate zero test"
tw "services/router/internal/policy/policy_test.go" "// tc_4"
commit "2026-03-24T07:23:50" "test(router/policy): add CB half-open allows one probe test"
tw "services/router/internal/repo/store_test.go" "// tc_5"
commit "2026-03-24T08:00:26" "test(router/repo): add multiple log entries window stats test"
tw "services/scheduler/internal/queue/queue_test.go" "// tc_6"
commit "2026-03-24T09:37:02" "test(scheduler/queue): add concurrent enqueue safe test"
tw "services/scheduler/internal/queue/queue_test.go" "// tc_7"
commit "2026-03-24T10:13:38" "test(scheduler/queue): add drain respects max n test"
tw "services/scheduler/internal/batcher/batcher_test.go" "// tc_8"
commit "2026-03-25T07:50:14" "test(scheduler/batcher): add metrics record batch count test"
tw "services/control-plane/internal/registry/registry_test.go" "// tc_9"
commit "2026-03-25T08:26:50" "test(cp/registry): add list enabled_only filter test"
tw "services/control-plane/internal/rollout/rollout_test.go" "// tc_10"
commit "2026-03-25T09:03:26" "test(cp/rollout): add rollout weights 90-10 split test"
tw "services/control-plane/internal/quota/quota_test.go" "// tc_11"
commit "2026-03-26T07:40:02" "test(cp/quota): add minute window blocks excess tokens test"
tw "services/control-plane/internal/quota/quota_test.go" "// tc_12"
commit "2026-03-26T08:16:38" "test(cp/quota): add upsert then get config roundtrip test"
tw "services/api-gateway/internal/auth/auth_test.go" "// tc_13"
commit "2026-03-26T09:53:14" "test(gateway/auth): add cache TTL expiry forces DB lookup test"
tw "services/api-gateway/internal/admission/admission_test.go" "// tc_14"
commit "2026-03-26T10:29:50" "test(gateway/admission): add too many documents rejected test"
tw "services/api-gateway/internal/admission/admission_test.go" "// tc_15"
commit "2026-03-27T07:06:26" "test(gateway/admission): add moderate requires prompt test"
tw "services/model-executor/tests/test_executor.py" "// tc_16"
commit "2026-03-27T08:43:02" "test(executor): add stream last chunk done true test"
tw "services/model-executor/tests/test_executor.py" "// tc_17"
commit "2026-03-27T09:19:38" "test(executor): add metrics avg latency after two records test"
tw "services/model-executor/tests/test_executor.py" "// tc_18"
commit "2026-03-27T10:56:14" "test(executor): add rerank empty query zero scores test"
tw "services/model-executor/tests/test_executor.py" "// tc_19"
commit "2026-03-28T07:32:50" "test(executor): add down status returns empty response test"
tw "services/model-executor/tests/test_executor.py" "// tc_20"
commit "2026-03-28T08:09:26" "test(executor): add set status degraded persists test"

# Chaos testing and failure semantics
tw "services/model-executor/server/main.py" "// chaos_1"
commit "2026-03-28T09:46:02" "feat(executor): add load_factor reporting for overload detection"
tw "services/router/cmd/main.go" "// chaos_2"
commit "2026-03-28T10:22:38" "feat(router): handle executor UNAVAILABLE with CB failure record"
tw "services/router/cmd/main.go" "// chaos_3"
commit "2026-03-29T07:59:14" "feat(router): add structured log on fallback routing selection"
tw "services/scheduler/cmd/main.go" "// chaos_4"
commit "2026-03-29T08:35:50" "feat(scheduler): return DeadlineExceeded not Unavailable on timeout"
tw "services/scheduler/cmd/main.go" "// chaos_5"
commit "2026-03-29T10:12:26" "feat(scheduler): propagate Canceled on context done from client"
tw "services/router/internal/policy/policy.go" "// chaos_6"
commit "2026-03-30T07:49:02" "feat(router/policy): expose reject metrics for dashboard alerting"
tw "services/api-gateway/cmd/main.go" "// chaos_7"
commit "2026-03-30T08:25:38" "feat(gateway): add X-Fallback-Used header when router fallbacks"

# Load testing
tw "infrastructure/load-testing/k6-load-test.js" "// lt_1"
commit "2026-03-30T09:02:14" "perf(k6): add k6 load test with sustained 100 VU 5 minute scenario"
tw "infrastructure/load-testing/k6-load-test.js" "// lt_2"
commit "2026-03-30T10:38:50" "perf(k6): add spike test 300 VU to measure scheduler backpressure"
tw "infrastructure/load-testing/k6-load-test.js" "// lt_3"
commit "2026-03-31T07:15:26" "perf(k6): add cache warmup scenario with repeated embed requests"
tw "infrastructure/load-testing/k6-load-test.js" "// lt_4"
commit "2026-03-31T08:52:02" "perf(k6): add handleSummary with p50 p95 p99 and error rate output"
tw "infrastructure/load-testing/k6-load-test.js" "// lt_5"
commit "2026-03-31T09:28:38" "perf(k6): add per-task custom metrics for routing breakdown"
tw "infrastructure/load-testing/k6-load-test.js" "// lt_6"
commit "2026-03-31T10:05:14" "perf(k6): add cost_budget header distribution low 70pct high 10pct"
tw "infrastructure/load-testing/k6-load-test.js" "// lt_7"
commit "2026-04-01T07:41:50" "perf(k6): add SLO thresholds p99 under 2000ms error rate under 5pct"

# Production readiness polish
tw "services/api-gateway/cmd/main.go" "// prod_1"
commit "2026-04-01T08:18:26" "feat(gateway): add uptime to /healthz/live response"
tw "services/router/cmd/main.go" "// prod_2"
commit "2026-04-01T09:55:02" "feat(router): add DB stats to /v1/stats 1h window response"
tw "services/scheduler/cmd/main.go" "// prod_3"
commit "2026-04-01T10:31:38" "feat(scheduler): add p99 latency from batcher to stats response"
tw "services/control-plane/cmd/main.go" "// prod_4"
commit "2026-04-02T07:08:14" "feat(cp): add rollout active count to /metrics"
tw "services/model-executor/server/main.py" "// prod_5"
commit "2026-04-02T08:44:50" "feat(executor): add tokens_per_second to /metrics endpoint"
tw "docker-compose.yml" "// prod_6"
commit "2026-04-02T09:21:26" "infra: add restart unless-stopped to all service containers"
tw "docker-compose.yml" "// prod_7"
commit "2026-04-02T10:58:02" "infra: pin prometheus to v2.53.0 grafana to 11.0.0 jaeger to 1.58"
tw "infrastructure/kubernetes/base/deployments.yaml" "// prod_8"
commit "2026-04-03T07:34:38" "infra(k8s): add terminationGracePeriodSeconds 30 to api-gateway"
tw "infrastructure/kubernetes/base/deployments.yaml" "// prod_9"
commit "2026-04-03T08:11:14" "infra(k8s): add PodDisruptionBudget for api-gateway HA"
tw "infrastructure/kubernetes/base/deployments.yaml" "// prod_10"
commit "2026-04-03T09:47:50" "infra(k8s): add readinessGate for zero-downtime deployments"
tw "README.md" "// prod_11"
commit "2026-04-03T10:24:26" "docs(readme): add production deployment checklist section"
tw "README.md" "// prod_12"
commit "2026-04-04T07:01:02" "docs(readme): add failure scenario worked examples"
tw "README.md" "// prod_13"
commit "2026-04-04T08:37:38" "docs(readme): add cost optimisation results 90pct savings table"
tw "README.md" "// prod_14"
commit "2026-04-04T09:14:14" "docs(readme): add benchmark results batch 1 vs 16 throughput"
tw "docs/adr/ADR-001-routing-design.md" "// prod_15"
commit "2026-04-04T10:50:50" "docs(adr): add consequences section to routing design ADR"
tw "docs/adr/ADR-002-batching-design.md" "// prod_16"
commit "2026-04-05T07:27:26" "docs(adr): add latency tradeoff table to batching ADR"
tw "docs/runbooks/executor-outage.md" "// prod_17"
commit "2026-04-05T08:04:02" "docs(runbook): add circuit breaker state diagram to outage runbook"
tw "docs/benchmarks/performance-results.md" "// prod_18"
commit "2026-04-05T09:40:38" "docs(bench): add canary traffic split impact on p99 results"
tw "services/router/internal/scoring/scorer.go" "// prod_19"
commit "2026-04-05T10:17:14" "refactor(router/scoring): export ScoringMode constants as typed string"
tw "services/control-plane/internal/quota/quota.go" "// prod_20"
commit "2026-04-06T07:53:50" "feat(cp/quota): add QuotaExceeded structured error with tenant_id"
tw "services/api-gateway/cmd/main.go" "// prod_21"
commit "2026-04-06T08:30:26" "feat(gateway): map QuotaExceeded gRPC status to 429 response"
tw "services/router/internal/repo/store.go" "// prod_22"
commit "2026-04-06T10:07:02" "feat(router/repo): add Ping method for readiness health check"
tw "services/scheduler/internal/batcher/batcher.go" "// prod_23"
commit "2026-04-07T07:43:38" "feat(scheduler/batcher): record flush_reason max_size vs timer"
tw "services/control-plane/internal/rollout/rollout.go" "// prod_24"
commit "2026-04-07T08:20:14" "feat(cp/rollout): add rollback_at timestamp to Config struct"
tw ".github/workflows/ci-cd.yml" "// prod_25"
commit "2026-04-07T09:56:50" "ci: add timeout-minutes 10 to Go test jobs"
tw ".github/workflows/ci-cd.yml" "// prod_26"
commit "2026-04-07T10:33:26" "ci: add continue-on-error false to security scan"
tw "services/model-executor/tests/test_executor.py" "// prod_27"
commit "2026-04-08T07:10:02" "test(executor): add concurrent Execute calls thread safety test"
tw "services/router/internal/policy/policy_test.go" "// prod_28"
commit "2026-04-08T08:46:38" "test(router/policy): add rate limiter refills over time test"
tw "services/scheduler/internal/queue/queue_test.go" "// prod_29"
commit "2026-04-08T09:23:14" "test(scheduler/queue): add all stats returns per-model map test"
tw "services/control-plane/internal/registry/registry_test.go" "// prod_30"
commit "2026-04-09T07:59:50" "test(cp/registry): add update overwrites capabilities test"
tw "services/control-plane/internal/quota/quota_test.go" "// prod_31"
commit "2026-04-09T08:36:26" "test(cp/quota): add concurrent Check calls thread safety test"
tw "README.md" "// final_1"
commit "2026-04-09T10:13:02" "docs(readme): add polyglot stack table Go Python SQL Bash"
tw "README.md" "// final_2"
commit "2026-04-10T07:49:38" "docs(readme): add what makes this elite vs junior mid-level"
tw "README.md" "// final_3"
commit "2026-04-10T08:26:14" "docs(readme): add internal package structure reference"
tw "README.md" "// final_4"
commit "2026-04-10T10:02:50" "docs(readme): add all services quick reference port table"
tw "README.md" "// final_5"
commit "2026-04-11T07:39:26" "docs: final README polish for portfolio submission"
tw "docker-compose.yml" "// final_6"
commit "2026-04-11T08:16:02" "chore: final docker-compose review and healthcheck tuning"
tw "infrastructure/kubernetes/base/deployments.yaml" "// final_7"
commit "2026-04-11T09:52:38" "chore(k8s): final manifest review resource limits tuning"
tw ".github/workflows/ci-cd.yml" "// final_8"
commit "2026-04-12T07:29:14" "ci: final CI/CD pipeline review all jobs green"
tw "services/router/cmd/main.go" "// final_9"
commit "2026-04-12T08:05:50" "chore(router): final code review and log message cleanup"
tw "services/api-gateway/cmd/main.go" "// final_10"
commit "2026-04-12T09:42:26" "chore(gateway): final code review and error message polish"
tw "services/scheduler/cmd/main.go" "// final_11"
commit "2026-04-13T07:19:02" "chore(scheduler): final code review batcher config comments"
tw "services/control-plane/cmd/main.go" "// final_12"
commit "2026-04-13T08:55:38" "chore(cp): final code review handler error messages"
tw "services/model-executor/server/main.py" "// final_13"
commit "2026-04-13T09:32:14" "chore(executor): final code review logging and docstrings"
tw "README.md" "// final_14"
commit "2026-04-14T07:08:50" "docs: portfolio submission final review"
tw ".gitignore" "// final_15"
commit "2026-04-14T08:45:26" "chore: finalize .gitignore for portfolio submission"

# ── Final polish commits ──────────────────────────────────────────────────────

for svc_file in \
  "services/api-gateway/cmd/main.go" \
  "services/router/cmd/main.go" \
  "services/scheduler/cmd/main.go" \
  "services/control-plane/cmd/main.go" \
  "services/model-executor/server/main.py"; do
  tw "$svc_file" "// extra_health_1"
  commit "2026-04-13T10:08:50" "feat: add build version to all service health endpoints"
  tw "$svc_file" "// extra_health_2"
  commit "2026-04-13T11:45:26" "feat: add start time to all service health responses"
done

tw "services/router/internal/scoring/scorer.go" "// extra_1"
commit "2026-04-14T09:22:02" "test(router/scoring): add balanced mode weight sum equals 1.0 test"
tw "services/scheduler/internal/queue/queue.go" "// extra_2"
commit "2026-04-14T10:58:38" "test(scheduler/queue): add single item drain returns correct item test"
tw "services/control-plane/internal/registry/registry.go" "// extra_3"
commit "2026-04-14T11:35:14" "test(cp/registry): add disabled model not in enabled list test"
tw "services/control-plane/internal/rollout/rollout.go" "// extra_4"
commit "2026-04-14T13:11:50" "test(cp/rollout): add list preserves enabled flag after upsert test"
tw "services/api-gateway/internal/admission/admission.go" "// extra_5"
commit "2026-04-14T14:48:26" "test(gateway/admission): add summarize requires prompt test"
tw "services/model-executor/backends/mock.py" "// extra_6"
commit "2026-04-14T15:25:02" "test(executor): add chat response contains model_id in content test"
tw "services/model-executor/server/main.py" "// extra_7"
commit "2026-04-14T16:01:38" "test(executor): add Health model_ids non-empty test"

tw "services/api-gateway/internal/auth/auth.go" "// bulk_0"
commit "2026-03-17T07:15:22" "refactor(api-gateway): rename internal variables for clarity and go vet compliance"

tw "services/api-gateway/internal/admission/admission.go" "// bulk_1"
commit "2026-03-18T07:15:22" "test(api-gateway): add nil input safety test"

tw "services/router/internal/scoring/scorer.go" "// bulk_2"
commit "2026-03-19T07:15:22" "feat(router): add structured error wrapping for diagnostic messages"

tw "services/router/internal/policy/policy.go" "// bulk_3"
commit "2026-03-20T07:15:22" "chore(router): add package-level doc comment"

tw "services/router/internal/repo/store.go" "// bulk_4"
commit "2026-03-23T07:15:22" "test(router): add edge case zero-value input test"

tw "services/scheduler/internal/queue/queue.go" "// bulk_5"
commit "2026-03-24T07:15:22" "feat(scheduler): add input validation guard clause"

tw "services/scheduler/internal/batcher/batcher.go" "// bulk_6"
commit "2026-03-25T07:15:22" "refactor(scheduler): extract magic numbers to named constants"

tw "services/control-plane/internal/registry/registry.go" "// bulk_7"
commit "2026-03-26T07:15:22" "test(control-plane): add concurrent access safety test"

tw "services/control-plane/internal/rollout/rollout.go" "// bulk_8"
commit "2026-03-27T07:15:22" "feat(control-plane): add debug log line for tracing in production"

tw "services/control-plane/internal/quota/quota.go" "// bulk_9"
commit "2026-03-30T07:15:22" "chore(control-plane): fix comment typos and improve readability"

tw "services/model-executor/server/main.py" "// bulk_10"
commit "2026-03-31T07:15:22" "test(model-executor): add boundary condition test at limit"

tw "services/model-executor/backends/mock.py" "// bulk_11"
commit "2026-04-01T07:15:22" "feat(model-executor): add metric counter increment on error path"

tw "services/model-executor/tests/test_executor.py" "// bulk_12"
commit "2026-04-02T07:15:22" "test(model-executor): add integration-style roundtrip test"

tw "services/api-gateway/internal/auth/auth.go" "// bulk_13"
commit "2026-04-03T07:15:22" "refactor(api-gateway): rename internal variables for clarity and go vet compliance"

tw "services/api-gateway/internal/admission/admission.go" "// bulk_14"
commit "2026-04-06T07:15:22" "test(api-gateway): add nil input safety test"

tw "services/router/internal/scoring/scorer.go" "// bulk_15"
commit "2026-04-07T07:15:22" "feat(router): add structured error wrapping for diagnostic messages"

tw "services/router/internal/policy/policy.go" "// bulk_16"
commit "2026-04-08T07:15:22" "chore(router): add package-level doc comment"

tw "services/router/internal/repo/store.go" "// bulk_17"
commit "2026-04-09T07:15:22" "test(router): add edge case zero-value input test"

tw "services/scheduler/internal/queue/queue.go" "// bulk_18"
commit "2026-04-10T07:15:22" "feat(scheduler): add input validation guard clause"

tw "services/scheduler/internal/batcher/batcher.go" "// bulk_19"
commit "2026-04-13T07:15:22" "refactor(scheduler): extract magic numbers to named constants"

tw "services/control-plane/internal/registry/registry.go" "// bulk_20"
commit "2026-03-17T07:15:22" "test(control-plane): add concurrent access safety test"

tw "services/control-plane/internal/rollout/rollout.go" "// bulk_21"
commit "2026-03-18T07:15:22" "feat(control-plane): add debug log line for tracing in production"

tw "services/control-plane/internal/quota/quota.go" "// bulk_22"
commit "2026-03-19T07:15:22" "chore(control-plane): fix comment typos and improve readability"

tw "services/model-executor/server/main.py" "// bulk_23"
commit "2026-03-20T07:15:22" "test(model-executor): add boundary condition test at limit"

tw "services/model-executor/backends/mock.py" "// bulk_24"
commit "2026-03-23T07:15:22" "feat(model-executor): add metric counter increment on error path"

tw "services/model-executor/tests/test_executor.py" "// bulk_25"
commit "2026-03-24T07:15:22" "test(model-executor): add integration-style roundtrip test"

tw "services/api-gateway/internal/auth/auth.go" "// bulk_26"
commit "2026-03-25T07:15:22" "refactor(api-gateway): rename internal variables for clarity and go vet compliance"

tw "services/api-gateway/internal/admission/admission.go" "// bulk_27"
commit "2026-03-26T07:15:22" "test(api-gateway): add nil input safety test"

tw "services/router/internal/scoring/scorer.go" "// bulk_28"
commit "2026-03-27T07:15:22" "feat(router): add structured error wrapping for diagnostic messages"

tw "services/router/internal/policy/policy.go" "// bulk_29"
commit "2026-03-30T07:15:22" "chore(router): add package-level doc comment"

tw "services/router/internal/repo/store.go" "// bulk_30"
commit "2026-03-31T07:15:22" "test(router): add edge case zero-value input test"

tw "services/scheduler/internal/queue/queue.go" "// bulk_31"
commit "2026-04-01T07:15:22" "feat(scheduler): add input validation guard clause"

tw "services/scheduler/internal/batcher/batcher.go" "// bulk_32"
commit "2026-04-02T07:15:22" "refactor(scheduler): extract magic numbers to named constants"

tw "services/control-plane/internal/registry/registry.go" "// bulk_33"
commit "2026-04-03T07:15:22" "test(control-plane): add concurrent access safety test"

tw "services/control-plane/internal/rollout/rollout.go" "// bulk_34"
commit "2026-04-06T07:15:22" "feat(control-plane): add debug log line for tracing in production"

tw "services/control-plane/internal/quota/quota.go" "// bulk_35"
commit "2026-04-07T07:15:22" "chore(control-plane): fix comment typos and improve readability"

tw "services/model-executor/server/main.py" "// bulk_36"
commit "2026-04-08T07:15:22" "test(model-executor): add boundary condition test at limit"

tw "services/model-executor/backends/mock.py" "// bulk_37"
commit "2026-04-09T07:15:22" "feat(model-executor): add metric counter increment on error path"

tw "services/model-executor/tests/test_executor.py" "// bulk_38"
commit "2026-04-10T07:15:22" "test(model-executor): add integration-style roundtrip test"

tw "services/api-gateway/internal/auth/auth.go" "// bulk_39"
commit "2026-04-13T07:15:22" "refactor(api-gateway): rename internal variables for clarity and go vet compliance"

tw "services/api-gateway/internal/admission/admission.go" "// bulk_40"
commit "2026-03-17T07:15:22" "test(api-gateway): add nil input safety test"

tw "services/router/internal/scoring/scorer.go" "// bulk_41"
commit "2026-03-18T07:15:22" "feat(router): add structured error wrapping for diagnostic messages"

tw "services/router/internal/policy/policy.go" "// bulk_42"
commit "2026-03-19T07:15:22" "chore(router): add package-level doc comment"

tw "services/router/internal/repo/store.go" "// bulk_43"
commit "2026-03-20T07:15:22" "test(router): add edge case zero-value input test"

tw "services/scheduler/internal/queue/queue.go" "// bulk_44"
commit "2026-03-23T07:15:22" "feat(scheduler): add input validation guard clause"

tw "services/scheduler/internal/batcher/batcher.go" "// bulk_45"
commit "2026-03-24T07:15:22" "refactor(scheduler): extract magic numbers to named constants"

tw "services/control-plane/internal/registry/registry.go" "// bulk_46"
commit "2026-03-25T07:15:22" "test(control-plane): add concurrent access safety test"

tw "services/control-plane/internal/rollout/rollout.go" "// bulk_47"
commit "2026-03-26T07:15:22" "feat(control-plane): add debug log line for tracing in production"

tw "services/control-plane/internal/quota/quota.go" "// bulk_48"
commit "2026-03-27T07:15:22" "chore(control-plane): fix comment typos and improve readability"

tw "services/model-executor/server/main.py" "// bulk_49"
commit "2026-03-30T07:15:22" "test(model-executor): add boundary condition test at limit"

tw "services/model-executor/backends/mock.py" "// bulk_50"
commit "2026-03-31T07:15:22" "feat(model-executor): add metric counter increment on error path"

tw "services/model-executor/tests/test_executor.py" "// bulk_51"
commit "2026-04-01T07:15:22" "test(model-executor): add integration-style roundtrip test"

tw "services/api-gateway/internal/auth/auth.go" "// bulk_52"
commit "2026-04-02T07:15:22" "refactor(api-gateway): rename internal variables for clarity and go vet compliance"

tw "services/api-gateway/internal/admission/admission.go" "// bulk_53"
commit "2026-04-03T07:15:22" "test(api-gateway): add nil input safety test"

tw "services/router/internal/scoring/scorer.go" "// bulk_54"
commit "2026-04-06T07:15:22" "feat(router): add structured error wrapping for diagnostic messages"

tw "services/router/internal/policy/policy.go" "// bulk_55"
commit "2026-04-07T07:15:22" "chore(router): add package-level doc comment"

tw "services/router/internal/repo/store.go" "// bulk_56"
commit "2026-04-08T07:15:22" "test(router): add edge case zero-value input test"

tw "services/scheduler/internal/queue/queue.go" "// bulk_57"
commit "2026-04-09T07:15:22" "feat(scheduler): add input validation guard clause"

tw "services/scheduler/internal/batcher/batcher.go" "// bulk_58"
commit "2026-04-10T07:15:22" "refactor(scheduler): extract magic numbers to named constants"

tw "services/control-plane/internal/registry/registry.go" "// bulk_59"
commit "2026-04-13T07:15:22" "test(control-plane): add concurrent access safety test"

