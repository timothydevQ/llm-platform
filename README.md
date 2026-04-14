# llm-platform

A multi-model LLM serving platform: intelligent routing, adaptive batching,
per-tenant quota enforcement, canary deployments.

---

## What is real today vs what is production-targeted

| Component | Status | Notes |
|---|---|---|
| **Embedding** (`/v1/embed`) | **Real** | `all-MiniLM-L6-v2` → 384-dim L2-normalised vectors via `sentence-transformers`. `tests/test_integration.py` verifies dimension and normalisation. |
| **Reranking** (`/v1/rerank`) | **Real** | `ms-marco-MiniLM-L-6-v2` cross-encoder. Logits → sigmoid scores. Integration test verifies relevant docs rank higher. |
| **Text generation** (`/v1/chat`) | **Real model, limited quality** | `facebook/opt-125m` (125M params). Generates coherent text. Not a production LLM — swap for vLLM + Llama-3-8B; the `Backend` interface is identical. |
| **Token streaming** | **Real** | `transformers.TextIteratorStreamer` — tokens emitted from the decode loop, not post-hoc word-splitting. Cancellation via `context.is_active()` check per token. |
| **Token counting** | **Real** | `AutoTokenizer.encode()` — actual BPE counts, not `len/4` estimates. |
| **Classification/moderation** | **Real** | `nli-distilroberta-base` zero-shot classification. |
| **gRPC wire format** | **Real protobuf binary** | Go: `google.golang.org/protobuf/encoding/protowire`. Python: `google.protobuf` descriptor pool. Not JSON. Run `make proto` to replace with `protoc-gen-go` output; wire bytes are identical. |
| **Proto contracts** | **Real schemas, hand-generated stubs** | `.proto` files in `proto/` are authoritative. Stubs match field numbers exactly. `make proto` generates conventional `protoc` output. |
| **gRPC interceptors** | **Real** | `Recovery`, `RequestID`, `Logging`, `Metrics`, `DeadlineCheck` via `grpc.ChainUnaryInterceptor`. |
| **Multi-dim routing** | **Real logic** | 5-dimension × 3-mode weight scoring. Unit tests verify all scoring paths. Not tested under production traffic volumes. |
| **Adaptive batching** | **Real logic** | p99-driven window tightening, three priority lanes, load shedding. Algorithm correct; throughput numbers are from single-node CPU tests. |
| **SQLite state** | **Real** | Model registry, rollout config, quota windows, request audit log. Two migrations with views (`live_executors`, `model_health_1h`). |
| **Quota enforcement** | **Real** | In-memory minute counter + SQLite day window. Denials logged to `quota_alerts`. |
| **Canary rollout** | **Real mechanics** | SQL-backed traffic split, auto-rollback evaluation loop. Rollback triggers on mock metrics in dev; uses real `model_latency_stats` in production. |
| **Kubernetes manifests** | **Production-targeted** | Correct HPA, PDB, probes, resource limits. Not deployed to a real cluster. |
| **Benchmark numbers** | **Real, single-node CPU** | MacBook M3 Pro, CPU-only. Run `make bench` to reproduce. Numbers for GPU / production scale would differ significantly. |

---

## Architecture

```
Client  ──HTTP/JSON──▶  api-gateway :8080  (Go)
                          auth · admission · SSE streaming
                          │
                          │  gRPC / protowire binary
                          ▼
                        router :50052  (Go)
                          5-dim scoring · circuit breakers · canary split
                          │
                          │  gRPC / protowire binary
                          ▼
                        model-executor :50051  (Python)
                          embed/rerank   → sentence-transformers
                          chat/classify  → transformers (TextIteratorStreamer)
                          │
                    ┌─────┴──────┐
                    ▼            ▼
          scheduler :50053   control-plane :8083
          priority queues    model registry
          adaptive batch     rollout config
          load shedding      quota enforcement
```

### Wire format

All internal gRPC calls use **protobuf binary**:

- **Go** — `google.golang.org/protobuf/encoding/protowire`, field numbers match `.proto` exactly
- **Python** — `google.protobuf` descriptor pool, real `SerializeToString` / `FromString`
- Run `make proto` for conventional protoc output; wire format is identical

---

## Quick start

```bash
# Download model weights once (~800MB)
bash scripts/download-models.sh

# Start all services
docker compose up -d

# Verify real embedding (should print 384)
curl -s -X POST http://localhost:8080/v1/embed \
  -H "Authorization: Bearer test-key-1234" \
  -d '{"query":"test"}' \
  | python3 -c "import sys,json; print(len(json.load(sys.stdin)['embedding']))"

# Real cross-encoder reranking
curl -s -X POST http://localhost:8080/v1/rerank \
  -H "Authorization: Bearer test-key-1234" \
  -d '{"query":"transformer architecture",
       "documents":["Self-attention models sequence relationships.",
                    "A transformer steps up AC voltage."]}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['scores'])"
# → [0.87, 0.09]

# Real token streaming
curl -N -X POST http://localhost:8080/v1/chat \
  -H "Authorization: Bearer test-key-1234" \
  -d '{"prompt":"Explain attention mechanisms","stream":true}'
# data: {"token":"The ","done":false}
# data: {"token":"attention ","done":false}
# ...

# No model weights (MockBackend for CI)
USE_REAL_MODELS=false docker compose up model-executor
```

---

## Running tests

```bash
make test-go          # Go tests, race detector, all 4 services
make test-py          # Python unit tests, MockBackend, no weights needed
make test-int         # Python integration tests, real model weights required
```

Integration tests skip automatically if `sentence-transformers` is not installed:
```
SKIP: Real model weights unavailable — set USE_REAL_MODELS=true and install sentence-transformers
```

---

## Benchmarks

**Environment:** MacBook M3 Pro, CPU-only, `facebook/opt-125m` + `all-MiniLM-L6-v2` + `ms-marco-MiniLM-L-6-v2`

```
task              n     p50_ms   p95_ms   p99_ms   mean_ms  stddev  tok/s
──────────────────────────────────────────────────────────────────────────
embed            200      31.4     48.9     67.2      33.6     9.1     —
rerank/5docs     200      88.3    134.1    189.4      92.7    21.3     —
classify         200      95.1    142.8    198.3      99.4    23.6     —
chat/20tok       100    1193.2   1847.6   2412.1    1241.8   198.4   41.2
chat/50tok       100    2847.3   3921.0   4510.2    2934.1   312.7   39.8
stream/20tok      50    first_token=298ms, total=1231ms, tok/s=38.4
```

**How to reproduce:**
```bash
# 1. Start executor
cd services/model-executor
USE_REAL_MODELS=true python3 -m server.main &

# 2. Run benchmark
make bench
# or directly:
python3 scripts/benchmark.py --samples 200 --warmup 10

# Quick version (no chat):
python3 scripts/benchmark.py --no-chat --no-stream --samples 100
```

Latency is wall-clock gRPC round-trip. p50/p95/p99 are exact percentiles, not interpolated. A GPU node with vLLM would produce ~10× lower chat latency and ~100× higher tokens/second.

---

## Routing

Each request is scored across five dimensions:

```
total_score = (w_lat × latency_score)
            + (w_cost × cost_score)
            + (w_health × health_score)
            + (w_queue × queue_score)
            + (w_policy × policy_score)
            × rollout_weight
```

| Dimension | What it measures |
|---|---|
| latency | Model avg latency vs request `latency_target_ms` |
| cost | `cost_per_1k` vs request `cost_budget` (low/medium/high) |
| health | Rolling error rate last 100 calls (`HealthTracker`) |
| queue | Current queue depth in scheduler |
| policy | Prompt length vs model context window |

Mode weight vectors:

| Mode | latency | cost | health | queue | policy |
|------|---------|------|--------|-------|--------|
| `latency_optimized` | **0.50** | 0.10 | 0.25 | 0.10 | 0.05 |
| `cost_optimized` | 0.10 | **0.50** | 0.20 | 0.10 | 0.10 |
| `balanced` | 0.25 | 0.25 | 0.25 | 0.15 | 0.10 |

---

## API reference

**Auth:** `Authorization: Bearer <key>` or `X-API-Key: <key>`  
**Test keys:** `test-key-1234` (balanced), `platform-key-5678` (latency-optimized)

| Method | Path | Task |
|--------|------|------|
| POST | `/v1/embed` | 384-dim embedding |
| POST | `/v1/rerank` | Cross-encoder relevance scores |
| POST | `/v1/chat` | Text generation |
| POST | `/v1/classify` | Sentiment classification |
| POST | `/v1/summarize` | Summarization |
| POST | `/v1/moderate` | Safety classification |

Request body:
```json
{
  "prompt": "...",
  "query": "...",
  "documents": ["doc1", "doc2"],
  "messages": [{"role": "user", "content": "..."}],
  "max_tokens": 128,
  "stream": false,
  "cost_budget": "low",
  "latency_target_ms": 500
}
```

---

## Design decisions

| Decision | ADR | Why |
|---|---|---|
| Hand-written protowire stubs | [ADR-003](docs/adr/ADR-003-executor-contract.md) | No protoc in CI; protowire binary is wire-identical to protoc output. `make proto` is a drop-in upgrade. |
| `facebook/opt-125m` | — | Smallest model runnable on CPU without quantisation. Swap for vLLM + Llama-3; `Backend` ABC stays identical. |
| `TextIteratorStreamer` | — | Real incremental decoding. `context.is_active()` check per token enables prompt cancellation. |
| SQLite everywhere | — | Same SQL schema as PostgreSQL. No code divergence between dev and prod. |
| Per-model circuit breakers | [ADR-001](docs/adr/ADR-001-routing-design.md) | A global CB prevents fallback routing. Per-model CBs allow routing around degraded models. |
| p99-driven batch window | [ADR-002](docs/adr/ADR-002-batching-design.md) | Fixed windows cause runaway tail latency under uneven load. |

---

## Upgrading to a production LLM

The executor's `Backend` ABC is stable. Replace `TransformersBackend._generate()`:

```python
# Current: facebook/opt-125m on CPU
output = self._gen_pipeline(text, max_new_tokens=cap, ...)

# Upgrade A: larger HF model on GPU
HF_CHAT_MODEL = "meta-llama/Meta-Llama-3-8B-Instruct"  # set CUDA_VISIBLE_DEVICES

# Upgrade B: vLLM (much higher throughput, same interface)
from vllm import LLM, SamplingParams
llm = LLM(model="meta-llama/Meta-Llama-3-8B-Instruct")
outputs = llm.generate([text], SamplingParams(max_tokens=cap))
generated = outputs[0].outputs[0].text

# For real streaming with vLLM, use AsyncLLMEngine:
from vllm import AsyncLLMEngine, AsyncEngineArgs, SamplingParams
engine = AsyncLLMEngine.from_engine_args(AsyncEngineArgs(model="..."))
async for output in engine.generate(text, SamplingParams(...), request_id=request_id):
    yield output.outputs[0].text
```

Nothing else changes — the gRPC interface, routing, batching, and quota logic are all model-agnostic.
// tw_6059_13103
// tw_6059_21752
// tw_6059_17658
// tw_6059_6886
// tw_6059_21226
