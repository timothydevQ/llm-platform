# Postmortem: Cache Eviction Spike — March 10, 2024

## Summary
Batch job filled embed cache (50k entries) in 4 minutes causing evictions. P99 latency spiked from 50ms to 1200ms for embedding requests for 14 minutes.

## Root Cause
Embed cache sized for online traffic. Batch workloads require a separate non-competing cache.

## Action Items
- Separate online and batch cache namespaces (done)
- Increase embed cache to 200k entries (done)
- Add batch job rate limiting to prevent cache thrashing (in progress)
<!-- timeline -->
<!-- actions -->
<!-- prevention -->
