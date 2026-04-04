# ADR-002: Three-Tier LRU Cache

## Status
Accepted

## Decision
Three independent LRU caches with different TTLs: prompt cache (10k, 5min), response cache (5k, 30min), embed cache (50k, 24hr).

## Rationale
Embeddings are expensive to compute but change rarely. Separation prevents chat cache pressure from evicting embeddings.
<!-- decision -->
