# ADR-004: Per-Model Circuit Breakers

## Status
Accepted

## Decision
One circuit breaker per model in the model router. 3 failures → open, 2 successes → close, 20s timeout.

## Rationale
A global CB would prevent fallback routing. Per-model CBs allow routing to a different model when one is unhealthy.
