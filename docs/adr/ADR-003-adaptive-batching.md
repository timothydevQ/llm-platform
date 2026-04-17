# ADR-003: Adaptive Dynamic Batching

## Status
Accepted

## Decision
Accumulate requests for up to 30ms or max batch size 16 before dispatching. Immediate dispatch when queue reaches MaxBatchSize.

## Consequences
- p50 latency +15ms (batching window)
- Throughput increases 4-8x
- Load shedding at 10k queue depth
