# LLM Serving Platform

A **production-grade multi-model inference serving system** with intelligent routing, adaptive batching, per-tenant quota enforcement, and canary deployment mechanics.

> Built as a demonstration of senior-level platform engineering for real-time ML serving infrastructure. Inspired by the internal serving layers behind large-scale personalization and AI products.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     API Gateway  :8080  (Go)                                │
│  Auth · Admission · Rate Limit · SSE Streaming · Request Normalisation      │
└────────────────────────────┬────────────────────────────────────────────────┘
                             │ gRPC  /routing.v1.RouterService/Route
                             ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                       Router  :50052  (Go)                                  │
│  Multi-dimensional Scoring · Circuit Breakers · Canary Traffic Split        │
│  Tenant Policy · SQL Model Registry · Request Audit Log                     │
└────┬──────────────────┬──────────────────────────────┬───────────────────── ┘
     │                  │                              │
     │ gRPC             │ poll every 30s               │ gRPC
     │ Execute          │ model reload                 │ Execute (fallback)
     ▼                  ▼                              ▼
┌──────────────┐  ┌────────────────┐       ┌──────────────────────────────┐
│  Scheduler   │  │ Control Plane  │       │   Model Executor  :50051     │
│  :50053  Go  │  │  :8083  Go     │       │       (Python / grpcio)      │
│  Priority Q  │  │  Registry      │       │  chat · embed · rerank       │
│  Adaptive    │  │  Rollout/Canary│       │  classify · moderate         │
│  Batching    │  │  Quota Config  │       │  Streaming · Cancellation    │
│  Load Shed   │  │  Tenant CRUD   │       │  MockBackend → vLLM/TGI      │
└──────────────┘  └────────────────┘       └──────────────────────────────┘

Observability: Prometheus · Grafana · Jaeger
Delivery:      GitHub Actions · GHCR · Kubernetes / HPA
```

---

## What makes this elite, not just interesting

| Junior | Mid-Level | **This project** |
|--------|-----------|-----------------|
| "I called an LLM API" | "I deployed a model" | **Multi-dimensional scoring across 5 dimensions to route each request to the optimal model under cost, latency, health, and policy constraints** |
| Single model | Multiple models | **5-model registry with tier routing, canary traffic splitting, and per-model circuit breakers** |
| No batching | Fixed batching | **Adaptive batching window that tightens when p99 exceeds SLO and widens under low load** |
| No tenancy | Single tenant | **Per-tenant rate limiting, quota enforcement (minute + day windows), and routing policy** |
| No failure handling | Try/catch | **Circuit breakers per model, load shedding at queue saturation, auto-rollback on canary regression** |

---

## Table of Contents

- [Architecture](#architecture)
- [Services](#services)
- [Request Lifecycle](#request-lifecycle)
- [Multi-Dimensional Scoring](#multi-dimensional-scoring)
- [Adaptive Batching](#adaptive-batching)
- [Canary Deployments](#canary-deployments)
- [Quota Enforcement](#quota-enforcement)
- [Failure Semantics](#failure-semantics)
- [API Reference](#api-reference)
- [Getting Started](#getting-started)
- [Running Tests](#running-tests)
- [Observability](#observability)
- [SLOs and SLIs](#slos-and-slis)
- [Benchmarks](#benchmarks)
- [CI/CD Pipeline](#cicd-pipeline)
- [Design Decisions](#design-decisions)
- [Roadmap](#roadmap)

---

## Architecture

### Services

| Service | Port | Language | Responsibility |
|---------|------|----------|----------------|
| `api-gateway` | 8080 (HTTP) | Go | Auth, admission control, rate limiting, SSE streaming |
| `router` | 50052 (gRPC) + 8081 (HTTP admin) | Go | Cost/latency/health scoring, canary routing, circuit breakers |
| `scheduler` | 50053 (gRPC) + 8082 (HTTP admin) | Go | Priority queues, adaptive batching, load shedding |
| `control-plane` | 8083 (HTTP) | Go | Model registry, rollout config, tenant/quota management |
| `model-executor` | 50051 (gRPC) + 8085 (HTTP health) | Python | Actual inference — MockBackend today, vLLM/TGI in production |

### Proto Contracts

All internal communication uses **gRPC with a JSON codec** — plain Go structs travel over the wire without the protobuf runtime. This gives schema discipline and gRPC semantics (deadlines, cancellation, streaming, status codes) while keeping the generated code readable.

```
proto/inference/v1/inference.proto   → InferenceRequest, InferenceResponse, StreamChunk
proto/execution/v1/execution.proto  → ExecuteRequest, ExecuteResponse (executor contract)
proto/routing/v1/routing.proto      → RouteRequest, CandidateScore, RouteResponse
proto/scheduling/v1/scheduling.proto → ScheduleRequest, QueueStats
proto/platform/v1/platform.proto    → ModelDescriptor, RolloutConfig, Tenant, QuotaConfig
```

### SQL-Backed Platform State

All mutable platform state lives in SQLite (dev) / PostgreSQL (production). Nothing important is buried in YAML or environment variables.

| Table | Owned by | Purpose |
|-------|----------|---------|
| `models` + `model_capabilities` | control-plane | Model registry with tier and task capabilities |
| `routing_rules` | router | Cost/length-based routing rules evaluated by priority |
| `rollouts` + `rollout_metrics` | control-plane | Canary configs with auto-rollback evaluation windows |
| `tenants` | control-plane | Routing mode, rate limits, allowed model lists |
| `quotas` + `quota_usage` | control-plane | Per-tenant token and spend budgets with sliding windows |
| `api_keys` | api-gateway | Hashed keys → tenant mapping with enabled flag |
| `request_log` | router | Full audit trail with cost, latency, canary flag |
| `batch_log` | scheduler | Batch sizes, wait times, and flush reasons |

---

## Request Lifecycle

```
1.  Client → POST /v1/chat  (Bearer test-key-1234)

2.  api-gateway:
    ├── extract key from Authorization header
    ├── validate against api_keys table (SQLite, 60s cache)
    ├── Admit(): validate task, size, tokens, deadline
    ├── normalise max_tokens and deadline
    └── call /routing.v1.RouterService/Route (gRPC)

3.  router:
    ├── load tenant policy (routing_mode, rate_limit, allowed_models)
    ├── check per-tenant token bucket rate limit
    ├── load canary rollout weights from DB
    ├── build ScoringRequest with task, budget, latency_target, prompt_len
    ├── filter models by circuit breaker state
    ├── Score() → rank candidates across 5 dimensions
    ├── select primary (highest total_score)
    ├── dial executor via lazy gRPC pool
    ├── Execute() with 30s deadline
    ├── RecordSuccess()/RecordFailure() updating health tracker
    ├── async LogRequest() to request_log
    └── return InferenceResponse with trace_id, cost, model, latency

4.  model-executor (Python):
    ├── MockBackend.run() dispatches by task_type
    ├── simulate latency from model config
    ├── generate deterministic content/embedding/scores
    └── return ExecuteResponse with tokens_in, tokens_out, latency_ms

5.  api-gateway:
    ├── set X-Request-ID, X-Trace-ID, X-Response-Time-Ms headers
    └── return JSON response to client
```

For streaming: step 5 chunks the content word-by-word via Server-Sent Events with `Content-Type: text/event-stream`.

---

## Multi-Dimensional Scoring

The router scores every candidate model across five dimensions. The combined score determines which model handles the request.

```
total_score = (w_lat × latency_score)
            + (w_cost × cost_score)
            + (w_health × health_score)
            + (w_queue × queue_score)
            + (w_policy × policy_score)
            × rollout_weight
```

### Scoring dimensions

| Dimension | What it measures | How it scores |
|-----------|-----------------|---------------|
| **Latency** | Model avg latency vs `latency_target_ms` | small=1.0, medium=0.6, large=0.3; 0.0 if exceeds target |
| **Cost** | Cost per 1k tokens vs `cost_budget` | low-budget→small=1.0; high-budget→large=1.0; inverse of cost otherwise |
| **Health** | Rolling error rate over last 100 requests | `max(0, 1.0 - error_rate × 5)` → error_rate > 0.2 scores 0 |
| **Queue** | Current queue depth (from scheduler) | `max(0, 1.0 - depth/50)` → depth > 50 scores 0 |
| **Policy** | Prompt length vs model max context | 0.2 penalty if prompt > 80% of max context |

### Routing modes

| Mode | Latency | Cost | Health | Queue | Policy |
|------|---------|------|--------|-------|--------|
| `latency_optimized` | **0.50** | 0.10 | 0.25 | 0.10 | 0.05 |
| `cost_optimized` | 0.10 | **0.50** | 0.20 | 0.10 | 0.10 |
| `balanced` (default) | 0.25 | **0.25** | 0.25 | 0.15 | 0.10 |

Each tenant has a routing mode set in the `tenants` table. Premium tenants default to `latency_optimized`; economy tenants to `cost_optimized`.

### Circuit breakers

One circuit breaker per model. Thresholds: **3 failures → open**, 20s timeout, **2 successes in half-open → close**. When a CB opens, that model is excluded from the scoring candidate set. The next-best-scored model becomes the primary, and `fallback_used=true` appears in the response.

---

## Adaptive Batching

The scheduler accumulates requests in per-model, per-priority queues before dispatching them together to the executor. The batching window adapts to current load and p99 latency:

```
queue_depth < 5   → wait = MaxWaitMs (30ms)    — accumulate more
queue_depth 5-20  → wait = MaxWaitMs / 2 (15ms) — moderate pressure
queue_depth > 20  → wait = MinWaitMs (5ms)      — dispatch immediately
p99 > 1.5 × SLO  → wait = MinWaitMs             — tighten under latency pressure
```

**Priority lanes**: Each model queue has three lanes — `CRITICAL`, `HIGH → NORMAL → LOW`. `Drain(n)` always serves the highest lane first.

**Load shedding**: Queue capacity is 10,000 per model. When full, new `LOW` and `NORMAL` priority requests receive `RESOURCE_EXHAUSTED` and are not enqueued.

### Throughput vs latency tradeoff

| Batch size | Throughput | p99 added latency | Best for |
|------------|-----------|-------------------|---------|
| 1 (disabled) | 1× | 0ms | Ultra-low latency |
| 4 | 3.2× | ~15ms | Real-time chat |
| 8 | 5.8× | ~25ms | Balanced |
| **16** (default max) | **9.4×** | ~30ms | High-throughput embed/classify |

---

## Canary Deployments

```bash
# Route 10% of gpt-large traffic to gpt-medium (new version)
curl -X POST http://localhost:8083/v1/rollouts \
  -H "Content-Type: application/json" \
  -d '{
    "rollout_id":     "rollout-20260415",
    "base_model_id":  "gpt-large",
    "canary_model_id":"gpt-medium",
    "canary_pct":     0.1,
    "auto_rollback":  true,
    "max_p99_ratio":  2.0,
    "max_error_rate": 0.05,
    "enabled":        true
  }'
```

The **rollout evaluator** runs every 2 minutes and queries the `rollout_metrics` table for the last 10-minute window. If:
- `canary_error_rate > max_error_rate` **or**
- `canary_p99 / base_p99 > max_p99_ratio`

it automatically disables the rollout (`enabled=0`) and records the reason in `rollback_reason`. All traffic reverts to the base model.

**Manual rollback**:
```bash
curl -X DELETE http://localhost:8083/v1/rollouts/rollout-20260415 \
  -d '{"reason": "manual — elevated user complaints"}'
```

---

## Quota Enforcement

Two sliding windows per tenant:

| Window | Limit | Enforcement |
|--------|-------|-------------|
| **Minute** | `tokens_per_minute` | In-memory counter, reset each minute |
| **Day** | `tokens_per_day` + `budget_usd_per_day` | SQLite `quota_usage` table |

Check happens in the control-plane quota enforcer before routing. Hard limits:

```go
// Context length check (sync, no DB)
if contextTokens > cfg.MaxContextTokens → DENIED

// Minute window (in-memory counter)
if minuteCount + estimatedTokens > cfg.TokensPerMinute → DENIED

// Day window (SQLite)
if dayUsed + estimatedTokens > cfg.TokensPerDay → DENIED
if daySpent + estimatedCost > cfg.BudgetUSDPerDay → DENIED
```

Configure per-tenant quotas:
```bash
curl -X POST http://localhost:8083/v1/quotas \
  -d '{"tenant_id":"tenant-economy","tokens_per_minute":5000,"tokens_per_day":100000,"budget_usd_per_day":1.0}'
```

---

## Failure Semantics

### Scenario 1 — Executor goes down mid-stream

1. Router calls `Execute()` → gRPC returns `UNAVAILABLE`
2. `RecordFailure(modelID)` — CB failure count +1
3. After 3 failures, CB opens for 20s
4. Router's next request filters this model from candidates
5. Next-best-scored model (fallback) is selected
6. Response includes `"fallback_used": true`
7. After 20s, CB enters half-open; one probe request; 2 successes close it

**Test it:**
```bash
curl -X POST http://localhost:8085/v1/status -d '{"status":"down"}'
# Next inference calls will fallback automatically
curl -X POST http://localhost:8085/v1/status -d '{"status":"healthy"}'
# CB closes after 2 successful probes
```

### Scenario 2 — Traffic spike saturates scheduler

1. Queue depth exceeds 10,000 items for a model
2. New low/normal priority requests receive `RESOURCE_EXHAUSTED` (gRPC) → 429 (HTTP)
3. `HIGH` and `CRITICAL` priority requests continue to be served
4. Adaptive batcher shrinks wait window to `MinWaitMs (5ms)` — dispatches as fast as possible
5. Kubernetes HPA detects CPU > 60% on executor pods, scales up
6. Queue drains; normal batcher window resumes

### Scenario 3 — Canary model regresses

1. New model deployed as 10% canary
2. After 2 minutes, evaluator checks `rollout_metrics`
3. Canary p99 is 2.3× base p99 (threshold: 2.0×)
4. Auto-rollback fires: `enabled=0`, `rollback_reason` set
5. 100% traffic reverts to base model within one evaluation cycle
6. Alert fires on `control_plane_rollouts_active` dropping to 0

### Scenario 4 — Budget quota exceeded

1. Tenant has `budget_usd_per_day: 1.00`
2. Request comes in at $0.98 spent today
3. Quota check: `daySpent + estimatedCost (0.05) > budget (1.00)` → DENIED
4. Gateway receives `RESOURCE_EXHAUSTED` → returns 429 with `"reason": "daily spend budget exceeded"`
5. `gateway_admit_failed` metric increments; alert fires if rate sustained

---

## API Reference

### Authentication

All inference endpoints require an API key:
```
Authorization: Bearer <key>
X-API-Key: <key>  (alternative)
```

### Task endpoints

| Method | Path | Task |
|--------|------|------|
| POST | `/v1/chat` | Chat completion |
| POST | `/v1/summarize` | Text summarisation |
| POST | `/v1/embed` | Embedding vector |
| POST | `/v1/rerank` | Document reranking |
| POST | `/v1/classify` | Text classification |
| POST | `/v1/moderate` | Content moderation |

### Request fields

```json
{
  "prompt":            "string",
  "messages":          [{"role":"user","content":"..."}],
  "documents":         ["doc1","doc2"],
  "query":             "string",
  "max_tokens":        1024,
  "stream":            false,
  "priority":          1,
  "cost_budget":       "low|medium|high",
  "latency_target_ms": 300,
  "metadata":          {}
}
```

### Response fields

```json
{
  "request_id":    "a3f8b2c1",
  "trace_id":      "d4e9f0a2",
  "task_type":     1,
  "model_id":      "gpt-small",
  "model_tier":    1,
  "content":       "...",
  "embedding":     [0.12, -0.34, ...],
  "scores":        [0.92, 0.43, 0.11],
  "tokens_input":  42,
  "tokens_output": 128,
  "latency_ms":    213.5,
  "queue_wait_ms": 18.2,
  "cached":        false,
  "fallback_used": false,
  "is_canary":     false,
  "cost_usd":      0.0000256,
  "executor_id":   "executor-0"
}
```

### Streaming (SSE)

```bash
curl -N -X POST http://localhost:8080/v1/chat \
  -H "Authorization: Bearer test-key-1234" \
  -H "Content-Type: application/json" \
  -d '{"prompt":"Explain distributed systems","stream":true}'

# data: {"request_id":"a3f8b2c1","token":"Distributed ","done":false}
# data: {"request_id":"a3f8b2c1","token":"systems ","done":false}
# data: {"request_id":"a3f8b2c1","token":"are...","done":true,"tokens_out":47}
```

### Control Plane endpoints

```bash
# Register a model
POST   /v1/models              body: ModelDescriptor
GET    /v1/models              returns all models with enabled flag
GET    /v1/models/:id          single model
PATCH  /v1/models/:id          body: {"enabled": false}

# Canary rollouts
POST   /v1/rollouts            body: RolloutConfig
GET    /v1/rollouts            all rollouts
DELETE /v1/rollouts/:id        body: {"reason": "..."}
GET    /v1/rollout-weights     returns {model_id: weight} for router polling

# Quota management
POST   /v1/quotas              body: QuotaConfig
GET    /v1/quotas/:tenant_id   usage + config
POST   /v1/quotas/:tenant_id   check: body: {"estimated_tokens":100,"context_tokens":512}
```

---

## Getting Started

```bash
git clone https://github.com/timothydevQ/llm-platform.git
cd llm-platform

# Start all services + observability stack
docker compose up -d

# Verify all services healthy (wait ~15 seconds for startup)
for port in 8080 8081 8082 8083 8085; do
  printf "Port %d: " $port
  curl -sf http://localhost:$port/healthz/ready | python3 -m json.tool 2>/dev/null || echo "not ready"
done

# Send a test inference request
curl -s -X POST http://localhost:8080/v1/chat \
  -H "Authorization: Bearer test-key-1234" \
  -H "Content-Type: application/json" \
  -d '{"prompt":"What is adaptive batching?","cost_budget":"low"}' | jq .

# Send an embedding request
curl -s -X POST http://localhost:8080/v1/embed \
  -H "Authorization: Bearer test-key-1234" \
  -H "Content-Type: application/json" \
  -d '{"query":"semantic search for recommendations"}' | jq .

# View routing stats
curl -s http://localhost:8081/v1/stats | jq .

# View scheduler batch metrics
curl -s http://localhost:8082/v1/stats | jq .

# View model registry
curl -s http://localhost:8083/v1/models | jq .

# Dashboard
open http://localhost:3000  # Grafana (admin/admin)
open http://localhost:9090  # Prometheus
open http://localhost:16686 # Jaeger traces
```

---

## Running Tests

```bash
# Go services (with race detector)
for svc in api-gateway router scheduler control-plane; do
  echo "Testing $svc..."
  cd services/$svc
  go test -race -v ./... 2>&1 | tail -5
  cd ../..
done

# Python model-executor
cd services/model-executor
pip install -r requirements.txt
python -m pytest tests/ -v
cd ../..

# SQL migrations
for f in sql/migrations/*.sql; do
  sqlite3 /tmp/test-$(basename $f .sql).db < "$f"
  echo "✓ $f"
done
```

---

## Observability

| Tool | URL | What to look at |
|------|-----|-----------------|
| Grafana | localhost:3000 | Request rates, p50/p95/p99, fallback rate, batch sizes |
| Prometheus | localhost:9090 | Raw metrics, alert state |
| Jaeger | localhost:16686 | End-to-end trace per `trace_id` |

### Key metrics

| Metric | Description |
|--------|-------------|
| `gateway_requests_total` | All requests received by gateway |
| `gateway_auth_failed` | Auth failures (invalid/disabled keys) |
| `gateway_admit_failed` | Admission failures (quota, size, validation) |
| `gateway_routed` | Requests successfully routed |
| `router_routed` | Requests dispatched to executor |
| `router_fallbacks` | Requests served by fallback model |
| `router_cb_blocked` | Requests blocked by circuit breaker |
| `router_canary` | Requests routed to canary model |
| `scheduler_enqueued` | Items enqueued |
| `scheduler_dispatched` | Items dispatched to executor |
| `scheduler_load_shedded` | Items dropped (queue full) |
| `scheduler_avg_batch_size` | Rolling average batch size |
| `executor_requests_total` | Total inferences completed |
| `executor_tokens_per_second` | Token generation throughput |
| `executor_avg_latency_ms` | Average inference latency |
| `control_plane_models_enabled` | Enabled models in registry |
| `control_plane_rollouts_active` | Active canary rollouts |

---

## SLOs and SLIs

| Endpoint | p50 | p95 | p99 | Error rate |
|----------|-----|-----|-----|------------|
| `/v1/chat` (gpt-small) | 210ms | 380ms | 650ms | < 0.5% |
| `/v1/embed` | 55ms | 90ms | 140ms | < 0.1% |
| `/v1/classify` | 220ms | 390ms | 670ms | < 0.5% |
| `/v1/rerank` | 110ms | 185ms | 300ms | < 0.1% |
| All (warm cache) | 8ms | 18ms | 35ms | < 0.1% |

**SLO compliance target**: 99.5% of requests within p99 budget measured over any 5-minute window.

---

## Benchmarks

Full results: [docs/benchmarks/performance-results.md](docs/benchmarks/performance-results.md)

**Batching impact (embed, 50 concurrent users):**

| Batch size | req/s | p99 latency |
|------------|-------|-------------|
| 1 (disabled) | 180 | 65ms |
| 4 | 580 | 78ms |
| 8 | 1,020 | 92ms |
| **16 (default)** | **1,680** | **112ms** |

**Cost routing savings** (1,000 chat requests, mixed prompts):

| Strategy | Blended cost/1k | Savings vs always-large |
|----------|----------------|------------------------|
| Always gpt-large | $20.00 | — |
| Always gpt-small | $0.20 | 99% |
| **Balanced routing** | **$1.84** | **90.8%** |
| Latency-optimised | $3.20 | 84% |
| Cost-optimised | $0.48 | 97.6% |

---

## CI/CD Pipeline

```
push to main
├── test-go (matrix: api-gateway, router, scheduler, control-plane)
│   ├── go vet
│   ├── go test -race -coverprofile
│   └── codecov upload
├── test-python (model-executor)
│   ├── pip install requirements.txt
│   └── pytest tests/ -v
├── check-sql (all migrations against sqlite3)
├── security (Trivy CRITICAL/HIGH scan)
└── build (matrix: all 5 images → GHCR)
    └── deploy (update K8s image tags, commit, push → ArgoCD sync)
```

---

## Design Decisions

| Decision | ADR | Summary |
|----------|-----|---------|
| Multi-dimensional scoring | [ADR-001](docs/adr/ADR-001-routing-design.md) | 5 dimensions × mode-specific weights vs simple tier lookup |
| Adaptive batching window | [ADR-002](docs/adr/ADR-002-batching-design.md) | p99-driven tightening vs fixed 30ms window |
| JSON codec over gRPC | [ADR-003](docs/adr/ADR-003-executor-contract.md) | Plain Go structs vs proto runtime dependency |
| Per-model circuit breakers | inline | Per-model placement allows fallback routing; global CB prevents it |
| SQLite for dev, PostgreSQL for prod | inline | Same SQL schema; no migration code divergence |

---

## Roadmap

### Q3 2026 — Real model backends
- Replace `MockBackend` with `vLLMBackend` calling a local LLaMA-3 instance
- Add `TritonBackend` for non-LLM models (embed, classify)
- Real token counts from tokeniser library

### Q4 2026 — Advanced caching
- Semantic cache using embedding similarity (cosine > 0.98 = cache hit)
- Prompt prefix caching for shared system prompts
- Distributed cache via Redis for multi-node setups

### Q1 2027 — Production hardening
- GPU utilisation metrics from DCGM exporter
- GPU-aware HPA: scale on `DCGM_FI_DEV_GPU_UTIL` not CPU
- OTLP trace propagation end-to-end through all 5 services
- mTLS between internal services

### Q2 2027 — Platform maturity
- Self-service model onboarding API with capability testing
- A/B testing framework with statistical significance testing for canary evaluation
- Priority-aware spend enforcement: premium tenants continue when economy tenants are cut
// rd_528
// rd_529
// rd_530
// rd_531
// rd_532
// rd_533
// rd_534
// rd_535
// rd_536
// rd_537
// rd_538
// rd_539
// rd_540
// rd_541
// rd_542
// rd_543
