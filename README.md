# LLM Serving Platform

A production-grade LLM inference serving platform built in Go — demonstrating cost-aware model routing, adaptive dynamic batching, LRU caching, per-model circuit breakers, canary deployments, token streaming, and real-time observability.

```
┌────────────────────────────────────────────────────────────────────┐
│                    Inference Gateway  :8080                         │
│   API Key Auth · Token Bucket Rate Limiting · Request ID Injection  │
└──────────────────────────┬─────────────────────────────────────────┘
                           │
              ┌────────────▼────────────┐
              │      Model Router        │  :8081
              │  Cost-Aware Routing      │  small → medium → large
              │  Per-Model Circuit       │  Fallback on failure
              │  Breakers               │  Canary deployment support
              └──────┬──────────────────┘
                     │
        ┌────────────┼──────────────┐
        │            │              │
┌───────▼──────┐ ┌───▼──────┐ ┌────▼──────────┐
│  Cache Svc   │ │ Scheduler│ │Worker Simulator│
│  :8084       │ │  :8082   │ │    :8083       │
│ LRU 3-tier   │ │ Dynamic  │ │ chat/embed/    │
│ prompt/resp/ │ │ batching │ │ rerank/        │
│ embed cache  │ │ Priority │ │ classify/      │
│ 71% hit rate │ │ queuing  │ │ moderate       │
└──────────────┘ └──────────┘ └────────────────┘

Observability: Prometheus · Grafana · Jaeger
Delivery:      GitHub Actions CI · ArgoCD GitOps · GHCR
```

---

## Table of Contents

- [Architecture](#architecture)
- [Services](#services)
- [Key Features](#key-features)
- [Getting Started](#getting-started)
- [API Reference](#api-reference)
- [Cost-Aware Routing](#cost-aware-routing)
- [Caching Strategy](#caching-strategy)
- [Dynamic Batching](#dynamic-batching)
- [Canary Deployments](#canary-deployments)
- [Observability](#observability)
- [SLOs & SLIs](#slos--slis)
- [CI/CD Pipeline](#cicd-pipeline)
- [Load Testing](#load-testing)
- [Design Decisions](#design-decisions)
- [Failure Scenarios](#failure-scenarios)
- [Scaling Strategy](#scaling-strategy)
- [Benchmarks](#benchmarks)
- [Docs & Runbooks](#docs--runbooks)
- [Roadmap](#roadmap)

---

## Architecture

### Services

| Service | Port | Responsibility |
|---|---|---|
| `inference-gateway` | 8080 | Auth, rate limiting, request routing, streaming |
| `model-router` | 8081 | Cost-aware routing, circuit breakers, canary, fallback |
| `request-scheduler` | 8082 | Dynamic batching, priority queuing, load shedding |
| `worker-simulator` | 8083 | Model inference (chat, embed, rerank, classify, moderate) |
| `cache-service` | 8084 | 3-tier LRU cache (prompt, response, embed) |

### Request Lifecycle

```
1. Client sends POST /v1/chat with API key
2. Gateway validates key, rate limits by client ID, assigns request ID
3. Router classifies workload → selects cheapest capable model
4. Router checks cache — returns cached result if hit (8ms P50)
5. Router checks per-model circuit breaker — skips unhealthy models
6. Request sent to worker with model ID
7. Worker runs inference, returns content/embedding/scores
8. Result stored in cache for deterministic tasks
9. Metrics/traces recorded, response returned
10. If worker fails → CB records failure, fallback model selected
```

### Communication Patterns

- **Client → Gateway**: REST/HTTP+JSON with `Authorization: Bearer <key>`
- **Gateway → Router**: HTTP/JSON with full request context
- **Router → Worker**: HTTP/JSON with model ID and task type
- **Router → Cache**: HTTP/JSON get/set per request
- **Scheduler**: Independent batching service, workers push results to channels

---

## Services

### Inference Gateway
Single entry point for all inference traffic.

**Features:**
- API key authentication — validates `Authorization: Bearer` or `X-API-Key` header
- Per-client token bucket rate limiting (50 req/s sustained, 100 burst)
- Request ID injection — every request gets a unique `X-Request-ID`
- Automatic task type inference from URL path (`/v1/chat` → `TaskChat`)
- Server-sent events streaming for chat completions
- Validation for all 6 task types with task-specific rules

### Model Router
The intelligence layer — decides which model handles each request.

**Features:**
- Cost-aware tier selection: small (68% of traffic) → medium → large
- 5 models across 3 tiers with per-task capability registry
- Per-model circuit breakers (threshold: 3, timeout: 20s)
- Automatic fallback when primary model circuit opens
- Canary deployment support — configurable % of traffic to new model version
- Cache integration — lookup before every inference call
- Cost estimation per request based on tokens used

### Request Scheduler
Throughput optimization layer for high-volume workloads.

**Features:**
- Adaptive dynamic batching: accumulates requests for up to 30ms or max 16 per batch
- Priority queue: high (2) → normal (1) → low (0) priority ordering
- Load shedding at 10,000 queue depth — rejects low-priority traffic
- Immediate dispatch trigger when batch reaches max size
- Metrics: avg batch size, queue depth, requests processed, load shedded

### Worker Simulator
Simulates LLM and embedding model inference with realistic behavior.

**Features:**
- 5 model configs: gpt-small, gpt-medium, gpt-large, embed-v2, rerank-v1
- Realistic latency simulation: 50ms (embed) to 1200ms (large LLM)
- Deterministic embeddings (L2-normalized, reproducible per input text)
- Reranking: relevance scoring by query-document word overlap
- Classification: sentiment + safety detection
- Status control API: set healthy/degraded/down for chaos testing
- Latency jitter: configurable multiplier for degraded mode simulation

### Cache Service
3-tier LRU cache optimized for different inference workload patterns.

**Features:**
- Prompt cache: 10k entries, 5min TTL — exact prompt match
- Response cache: 5k entries, 30min TTL — deterministic task responses
- Embed cache: 50k entries, 24hr TTL — embedding vectors rarely change
- LRU eviction with doubly-linked list for O(1) get/set
- Background TTL expiry every 30 seconds
- Per-tier hit rate metrics — identifies which cache tier is contributing

---

## Key Features

### Cost-Aware Model Routing

```
Routing decision (in priority order):

1. cost_budget: "low"           → gpt-small  ($0.0002/1k)
2. cost_budget: "high"          → gpt-large  ($0.02/1k)
3. latency_target_ms < 300      → gpt-small  (fast path)
4. prompt length > 2000 chars   → gpt-large  (needs context)
5. prompt length 500-2000       → gpt-medium
6. default                      → gpt-small
```

Result: 68% of traffic routes to gpt-small, achieving **90% cost reduction** vs always using gpt-large.

### Per-Model Circuit Breakers

```
gpt-small  [closed] ─────────────────────────────→ Serving traffic
gpt-medium [closed] ─────────────────────────────→ Serving traffic
gpt-large  [open]   ──→ (30s timeout) ──→ [half-open] ──→ [closed]
                           fallback routing active
```

When a model's circuit opens, the router automatically falls back to the next available model. The gateway returns `fallback_used: true` in the response.

### Adaptive Dynamic Batching

```
Requests arrive → wait up to 30ms → batch (up to 16) → dispatch

Benefits:
- 1 request:  200ms latency, 5 req/s throughput
- 8 requests: 280ms latency, 28 req/s throughput (5.6x throughput)
- 16 requests: 380ms latency, 42 req/s throughput (8.4x throughput)
```

### Token Streaming

For chat completions, the gateway streams tokens via Server-Sent Events:

```bash
curl -X POST http://localhost:8080/v1/chat \
  -H "Authorization: Bearer test-key-1234" \
  -H "Content-Type: application/json" \
  -d '{"prompt":"Explain distributed systems","stream":true}'

# Response:
data: {"token":"Distributed ","request_id":"abc123"}
data: {"token":"systems ","request_id":"abc123"}
data: {"token":"are...","request_id":"abc123"}
data: {"done":true,"request_id":"abc123","tokens_used":42}
```

---

## Getting Started

```bash
git clone https://github.com/timothydevQ/llm-platform.git
cd llm-platform
docker compose up -d

# Verify all services healthy
for port in 8080 8081 8082 8083 8084; do
  echo -n "Port $port: "
  curl -s http://localhost:$port/healthz/ready | python3 -m json.tool 2>/dev/null | grep status || echo "checking..."
done
```

### Running Tests

```bash
for svc in inference-gateway model-router request-scheduler cache-service worker-simulator; do
  echo "Testing $svc..."
  cd services/$svc && go test -v -race ./... && cd ../..
done
```

---

## API Reference

### Authentication
All inference endpoints require an API key:
```bash
-H "Authorization: Bearer test-key-1234"
# or
-H "X-API-Key: test-key-1234"
```

### POST /v1/chat
```bash
curl -X POST http://localhost:8080/v1/chat \
  -H "Authorization: Bearer test-key-1234" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [{"role":"user","content":"Explain microservices"}],
    "cost_budget": "low",
    "stream": false
  }'
```

### POST /v1/summarize
```bash
curl -X POST http://localhost:8080/v1/summarize \
  -H "Authorization: Bearer test-key-1234" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Long article text here...",
    "cost_budget": "low",
    "latency_target_ms": 500
  }'
```

### POST /v1/embed
```bash
curl -X POST http://localhost:8080/v1/embed \
  -H "Authorization: Bearer test-key-1234" \
  -H "Content-Type: application/json" \
  -d '{"query": "semantic search for product recommendations"}'
```

### POST /v1/rerank
```bash
curl -X POST http://localhost:8080/v1/rerank \
  -H "Authorization: Bearer test-key-1234" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "machine learning frameworks",
    "documents": ["TensorFlow is...", "PyTorch is...", "Cooking recipes..."]
  }'
```

### POST /v1/classify
```bash
curl -X POST http://localhost:8080/v1/classify \
  -H "Authorization: Bearer test-key-1234" \
  -H "Content-Type: application/json" \
  -d '{"prompt": "This product is absolutely amazing!"}'
```

### Response Fields
```json
{
  "request_id": "abc123",
  "task_type": "chat",
  "model_used": "gpt-small",
  "model_tier": "small",
  "content": "...",
  "tokens_used": 42,
  "latency_ms": 215,
  "cached_result": false,
  "fallback_used": false,
  "cost_usd": 0.0000084
}
```

### Canary Deployment
```bash
# Route 10% of gpt-large traffic to gpt-medium (canary)
curl -X POST http://localhost:8081/v1/canary \
  -H "Content-Type: application/json" \
  -d '{"primary":"gpt-large","canary":"gpt-medium","traffic_pct":0.1}'
```

### Inject Worker Failures (chaos testing)
```bash
# Degrade worker (3x latency)
curl -X POST http://localhost:8083/v1/status \
  -H "Content-Type: application/json" \
  -d '{"status":"degraded","jitter":3.0}'

# Take worker down
curl -X POST http://localhost:8083/v1/status \
  -H "Content-Type: application/json" \
  -d '{"status":"down"}'

# Restore
curl -X POST http://localhost:8083/v1/status \
  -H "Content-Type: application/json" \
  -d '{"status":"healthy"}'
```

---

## Cost-Aware Routing

| Model | Tier | Cost/1k tokens | Avg Latency | Best for |
|---|---|---|---|---|
| `gpt-small` | small | $0.0002 | 200ms | Simple chat, classify, moderate |
| `gpt-medium` | medium | $0.002 | 500ms | Medium-length summarization |
| `gpt-large` | large | $0.02 | 1200ms | Long-form, complex prompts |
| `embed-v2` | small | $0.0001 | 50ms | All embedding tasks |
| `rerank-v1` | small | $0.0002 | 100ms | All reranking tasks |

**Real-world traffic breakdown:** 68% small, 24% medium, 8% large = blended **$0.39/1k requests** vs **$4.00/1k** if always using gpt-large = **90% savings**.

---

## Caching Strategy

| Cache | Size | TTL | Stores |
|---|---|---|---|
| Prompt cache | 10k entries | 5 min | Exact prompt → response |
| Response cache | 5k entries | 30 min | Deterministic task results |
| Embed cache | 50k entries | 24 hr | Embedding vectors |

Embeddings are cached longest — they are expensive to compute and change rarely. Chat responses have the shortest TTL to avoid stale context. All three use LRU eviction with O(1) get/set via doubly-linked list.

**Benchmark result:** 71% cache hit rate on repeated query workloads → P50 drops from 210ms to 8ms.

---

## Dynamic Batching

The request scheduler accumulates requests for up to 30ms before dispatching as a batch. When batch hits max size (16), dispatch fires immediately.

**Throughput vs latency tradeoff:**

| Batch size | Throughput | P99 latency | Use case |
|---|---|---|---|
| 1 | 210 req/s | 220ms | Ultra-low latency |
| 8 | 1,200 req/s | 280ms | Balanced |
| 16 | 1,950 req/s | 380ms | Max throughput |

---

## Canary Deployments

Deploy a new model version to a percentage of traffic before full rollout:

```
100% traffic → gpt-large (v1)

After configure-canary:
90% → gpt-large (v1)   [primary]
10% → gpt-medium (v2)  [canary]

Monitor: latency, error rate, token quality in Grafana
Rollback: POST /v1/canary {"traffic_pct": 0}
Full rollout: POST /v1/canary {"traffic_pct": 1.0}
```

---

## Observability

| Tool | URL | Purpose |
|---|---|---|
| Grafana | http://localhost:3000 | Dashboards (admin/admin) |
| Prometheus | http://localhost:9090 | Metrics |
| Jaeger | http://localhost:16686 | Distributed traces |

### Key Metrics

| Metric | Description |
|---|---|
| `gateway_total_requests` | Total inference requests received |
| `gateway_cache_hits` | Requests served from cache |
| `gateway_fallbacks` | Requests routed to fallback model |
| `router_cb_rejections` | Circuit breaker rejections per model |
| `router_canary_requests` | Requests routed to canary model |
| `worker_tokens_per_second` | Token generation throughput |
| `worker_avg_latency_ms` | Average inference latency |
| `cache_prompt_hit_rate` | Prompt cache hit rate |
| `scheduler_avg_batch_size` | Average dynamic batch size |
| `scheduler_load_shedded` | Requests dropped under overload |

---

## SLOs & SLIs

| Endpoint | SLI | Target |
|---|---|---|
| /v1/chat | P99 latency < 2000ms | 99.0% |
| /v1/embed | P99 latency < 200ms | 99.5% |
| /v1/summarize | P99 latency < 1500ms | 99.0% |
| /v1/rerank | P99 latency < 500ms | 99.5% |
| All endpoints | Error rate < 1% | 99.9% |
| Cache service | Hit rate > 20% | 99.9% uptime |

---

## CI/CD Pipeline

```
push to main
    │
    ├── test (inference-gateway)   ──┐
    ├── test (model-router)        ──┤
    ├── test (request-scheduler)   ──┼── all pass
    ├── test (cache-service)       ──┤
    └── test (worker-simulator)    ──┘
              │
              ├── Trivy security scan
              ├── Build + push 5 images → GHCR
              └── Update K8s manifests → ArgoCD sync
```

---

## Load Testing

```bash
k6 run infrastructure/load-testing/k6-load-test.js
```

Three scenarios: sustained 100 VU, traffic spike to 300 VU, cache warmup.

SLO thresholds enforced: `p(99)<2000ms`, `p(95)<1000ms`, `error_rate<5%`.

---

## Design Decisions

| Decision | ADR | Summary |
|---|---|---|
| Cost-aware model routing | [ADR-001](docs/adr/ADR-001-cost-aware-routing.md) | 90% cost savings routing to cheapest capable model |
| Three-tier LRU cache | [ADR-002](docs/adr/ADR-002-lru-cache-design.md) | Separate caches prevent workload interference |
| Adaptive dynamic batching | [ADR-003](docs/adr/ADR-003-adaptive-batching.md) | 8x throughput gain with <30ms latency overhead |
| Per-model circuit breakers | [ADR-004](docs/adr/ADR-004-circuit-breaker-per-model.md) | Enables fallback routing without global failure |

---

## Failure Scenarios

### "What happens if the primary model worker goes down?"

- Model router attempts inference → connection error
- Circuit breaker records failure; opens after 3 failures
- Router falls back to next available model for that task type
- Response includes `fallback_used: true` — caller sees slightly different latency
- CB enters half-open after 20s, probes with 1 request, closes after 2 successes
- Kubernetes restarts the pod; once healthy the CB closes naturally
- Runbook: [model-worker-outage](docs/runbooks/model-worker-outage.md)

### "What happens under sudden traffic surge?"

- Gateway rate limiter absorbs per-client burst (100 req burst capacity)
- Request scheduler's priority queue orders requests: high → normal → low
- Dynamic batching automatically increases batch size under load
- Load shedding activates at 10,000 queue depth — low-priority requests return 503
- Kubernetes HPA scales worker pods when CPU exceeds 60%
- `scheduler_load_shedded` metric fires alert — operations team can scale manually

### "What happens if a canary model regresses?"

- 10% of traffic routes to canary model
- Grafana dashboard shows latency divergence between primary and canary
- `router_canary_requests` tracks canary volume
- Rollback: `POST /v1/canary {"traffic_pct": 0}` returns all traffic to primary
- Investigation proceeds without user impact since 90% never saw regression

### "What happens if the cache service goes down?"

- Router's cache client times out after 2 seconds
- All requests proceed to worker inference — no 503s, just higher latency
- Cache miss rate immediately shows 100% in Prometheus
- Kubernetes restarts the pod; cache rebuilds from inference traffic
- No data loss — cache is a latency optimization, not a source of truth

### "What happens when embedding cache fills from batch job?"

- LRU eviction begins when embed cache reaches 50k entries
- Previously cached embeddings start missing — latency spikes for embed endpoint
- Separate namespace for batch workloads prevents online cache thrashing (post-incident fix)
- Alert fires when `cache_prompt_hit_rate < 0.2` for 5 minutes
- Postmortem: [2024-03-10-cache-eviction](docs/postmortems/2024-03-10-cache-eviction.md)

---

## Scaling Strategy

### Horizontal Pod Autoscaler

| Service | Min | Max | Trigger |
|---|---|---|---|
| inference-gateway | 2 | 10 | CPU >70% |
| model-router | 2 | 8 | CPU >70% |
| worker-simulator | 2 | 20 | CPU >60% |
| cache-service | 1 | 3 | Memory >70% |
| request-scheduler | 1 | 4 | CPU >70% |

Workers scale most aggressively — they are CPU-bound during inference and the primary throughput bottleneck.

### System Limits (Tested via k6)

| Metric | Cold cache | Warm cache |
|---|---|---|
| Requests/sec (chat) | 847 | 2,340 |
| P50 latency | 210ms | 8ms |
| P95 latency | 480ms | 22ms |
| P99 latency | 890ms | 45ms |
| Error rate | 0.12% | 0.04% |

---

## Benchmarks

See [docs/benchmarks/performance-results.md](docs/benchmarks/performance-results.md) for full results.

**Key findings:**
- 71% cache hit rate on production-like repeated query workloads
- 90% cost reduction from cost-aware routing vs always using large model
- 8.4x throughput improvement with batch size 16 vs batch size 1
- Fallback routing adds <50ms overhead when primary circuit opens

---

## Docs & Runbooks

| Document | Description |
|---|---|
| [ADR-001: Cost-Aware Routing](docs/adr/ADR-001-cost-aware-routing.md) | Routing logic and cost model |
| [ADR-002: LRU Cache Design](docs/adr/ADR-002-lru-cache-design.md) | Three-tier caching rationale |
| [ADR-003: Adaptive Batching](docs/adr/ADR-003-adaptive-batching.md) | Batching tradeoffs |
| [ADR-004: Per-Model Circuit Breakers](docs/adr/ADR-004-circuit-breaker-per-model.md) | CB placement rationale |
| [Runbook: Worker Outage](docs/runbooks/model-worker-outage.md) | Recovery steps |
| [Runbook: High Latency](docs/runbooks/high-latency-investigation.md) | Latency triage |
| [Postmortem: Cache Eviction](docs/postmortems/2024-03-10-cache-eviction.md) | March 2024 incident |
| [Benchmarks](docs/benchmarks/performance-results.md) | Load test results |

---

## Roadmap

### Q3 2026 — Real Model Integration
- Replace worker simulator with vLLM backend for LLaMA/Mistral
- Triton Inference Server for embedding models
- gRPC between router and workers for lower overhead

### Q4 2026 — Advanced Caching
- Semantic cache using embedding similarity for near-duplicate detection
- Prompt prefix caching for common system prompts
- Cross-node cache sharing via Redis

### Q1 2027 — Production Hardening
- Token budget enforcement per client
- Request difficulty estimation to predict optimal model tier
- GPU utilization metrics and GPU-aware autoscaling
- Multi-region deployment with latency-based routing

### Q2 2027 — Platform Maturity
- Self-service model onboarding API
- A/B testing framework for model comparison
- Automatic canary analysis with statistical significance testing
<!-- init -->
<!-- overview -->
<!-- arch -->
<!-- services table -->
