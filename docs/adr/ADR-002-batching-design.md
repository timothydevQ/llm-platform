# ADR-002: Adaptive Dynamic Batching

**Status**: Accepted  
**Date**: 2026-03-25

---

## Context

GPU and CPU inference throughput improves significantly when multiple requests are processed together (a single forward pass vs N sequential passes). However, waiting too long to form a batch increases tail latency for every request in the queue.

The system must balance:
- **Throughput**: maximise batch sizes for embedding and classification workloads
- **Latency**: keep p99 within SLO for real-time chat workloads
- **Fairness**: high-priority requests should not be delayed behind bulk traffic

## Decision

Implement **adaptive batching with three mechanisms**:

### 1. Priority queuing
Each model queue has three lanes: `CRITICAL`/`HIGH` → `NORMAL` → `LOW`. `Drain(n)` always serves the highest lane first. This ensures real-time requests skip ahead of bulk workloads.

### 2. Adaptive wait window
The batcher recalculates its sleep duration before each `Drain`:

```go
if queueDepth > 20 || p99 > 1.5 × SLO {
    wait = MinWaitMs  // 5ms — dispatch immediately
} else if queueDepth > 5 {
    wait = MaxWaitMs / 2  // 15ms — moderate pressure
} else {
    wait = MaxWaitMs  // 30ms — accumulate more
}
```

This lets the batcher automatically tighten under load without operator intervention.

### 3. Load shedding
Queue capacity is 10,000 items per model. When full, new `LOW` and `NORMAL` items return `RESOURCE_EXHAUSTED` immediately rather than blocking. `HIGH` and `CRITICAL` items are always accepted (up to a separate hard cap).

## Tradeoffs

| Batch size | Throughput gain | p99 added latency |
|------------|----------------|-------------------|
| 1 (no batching) | 1× | 0ms |
| 4 | 3.2× | ~15ms |
| 8 | 5.8× | ~25ms |
| 16 | 9.4× | ~30ms |

Default `MaxBatchSize = 16`, `MaxWaitMs = 30`, `MinWaitMs = 5`, `P99SLOMs = 500`.

## Consequences

The adaptive window prevents runaway p99 growth during traffic spikes without operator changes. The tradeoff is that the batcher may oscillate between window sizes under variable load — this is acceptable since the adjustment is continuous and bounded.

---

# ADR-003: JSON Codec Over gRPC

**Status**: Accepted  
**Date**: 2026-03-16

---

## Context

The platform uses gRPC for all inter-service communication to benefit from:
- Deadline propagation
- Cancellation semantics
- Streaming (server-side)
- Structured error codes

However, requiring the protobuf runtime adds build complexity (protoc compilation step, generated code that is hard to read for reviewers, CGO requirements on some platforms).

## Decision

Override gRPC's default protobuf codec with a **JSON codec** (`gen/go/codec/codec.go`):

```go
func init() {
    grpcencoding.RegisterCodec(JSONCodec{})
}

func (JSONCodec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (JSONCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
func (JSONCodec) Name() string                        { return "proto" } // overrides default
```

All gRPC message types are plain Go structs with `json:` tags. Services import the codec package with `_ "github.com/timothydevQ/llm-platform/gen/codec"` to register it at startup.

## Consequences

**Positive**:
- No `protoc` compilation step required to build the project
- Message types are readable Go structs — easier for code review
- No CGO dependency
- JSON payloads are debuggable with standard tools (curl, jq)

**Negative**:
- JSON is 2–5× larger than protobuf binary encoding — slightly more network overhead for large embedding vectors
- No protobuf reflection, so tools like grpc_cli and Evans require explicit JSON mode
- In production at very high throughput, migrating to the protobuf codec would reduce serialisation overhead

## Migration path

If throughput benchmarks show serialisation as a bottleneck:
1. Run `scripts/generate-proto.sh` to generate protobuf Go stubs
2. Remove the JSON codec registration
3. No other application code changes required (message field names are identical)
// dc_506
// dc_507
// dc_508
// dc_509
