# ADR-001: Multi-Dimensional Model Scoring for Routing

**Status**: Accepted  
**Date**: 2026-03-18  
**Deciders**: Platform team

---

## Context

The platform needs to route each inference request to one of several available model backends. The naive approach — always picking the cheapest model or always the fastest — fails in production because:

- Latency requirements vary per request (real-time chat vs background summarisation)
- Cost budgets differ per tenant
- Model health changes dynamically (error rates spike during rollouts)
- Queue depth varies by model, making a "fast" model temporarily slow when overloaded

## Decision

Implement **multi-dimensional scoring** across five dimensions. Each candidate model receives a score in [0, 1] per dimension. A **mode-specific weight vector** determines the relative importance of each dimension, producing a single `total_score`.

```
total_score = Σ (w_i × score_i) × rollout_weight
```

### Dimensions

| Dimension | What it captures |
|-----------|-----------------|
| Latency | Does the model's average latency satisfy the request's `latency_target_ms`? |
| Cost | Does the model's cost align with the request's `cost_budget` hint? |
| Health | What is the rolling error rate over the last 100 requests for this model? |
| Queue | How deep is the current queue for this model in the scheduler? |
| Policy | Does the prompt length fit within the model's context window safely? |

### Mode weights

Three modes are available per tenant: `latency_optimized`, `cost_optimized`, `balanced`.

| Mode | Latency | Cost | Health | Queue | Policy |
|------|---------|------|--------|-------|--------|
| latency_optimized | 0.50 | 0.10 | 0.25 | 0.10 | 0.05 |
| cost_optimized | 0.10 | 0.50 | 0.20 | 0.10 | 0.10 |
| balanced | 0.25 | 0.25 | 0.25 | 0.15 | 0.10 |

### Rollout weights

When a canary rollout is active, the base model's effective weight is `(1 - canary_pct)` and the canary model's weight is `canary_pct`. A model with `rollout_weight = 0.0` scores 0 regardless of its individual dimension scores, effectively excluding it from routing.

## Alternatives Considered

1. **Rule-based tier routing** (e.g., `cost_budget=low → gpt-small`) — simple but ignores health and queue state. A healthy cheaper model could be overloaded while a healthy pricier model sits idle.

2. **Latency-only selection** (always pick the fastest model) — ignores cost, leading to unnecessary spend on large models for trivial tasks.

3. **Round-robin** — ignores all dimensions. Rejected.

## Consequences

**Positive**:
- A single routing code path handles all tenant configurations, task types, and canary states.
- Adding a new scoring dimension requires only adding a new method and updating the weight tuple.
- Circuit breaker placement remains in the router (not the scorer), keeping concerns separate.

**Negative**:
- Weight tuning requires empirical measurement. Initial weights are based on engineering judgment and will be adjusted after production data is available.
- The queue depth signal has up to 30s of staleness (model reload interval). Under sudden load spikes, the queue score may lag behind reality.

## Review after 30 days in production

Monitor `router_fallbacks` and `router_cb_blocked` metrics. If fallback rate exceeds 2%, revisit the health score penalty multiplier (currently `× 5`).
// tw_6059_5163
// tw_6059_16083
// tw_6059_31692
// tw_6059_5643
// tw_6059_28494
// tw_6059_5453
// tw_6059_29411
// tw_6059_25919
// tw_6059_12071
// tw_6059_13898
// tw_6059_5889
