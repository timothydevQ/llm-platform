# ADR-001: Cost-Aware Model Routing

## Status
Accepted

## Decision
Route requests to the cheapest model that can satisfy the task requirements, scaling up only when necessary.

Routing hierarchy: `cost_budget: low` or latency target < 300ms → small; `cost_budget: high` → large; prompt > 2000 chars → large; prompt 500-2000 → medium; default → small.

## Consequences
- gpt-small handles ~70% of traffic at $0.0002/1k tokens
- ~85-90% cost reduction vs always using gpt-large
