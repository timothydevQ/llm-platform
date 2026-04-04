# Performance Benchmark Results

## Test Configuration
- 2 inference-gateway replicas, 2 model-router replicas, 3 worker replicas

## Sustained Load (100 VU, 5 minutes)
| Metric | Value |
|---|---|
| Requests/sec | 847 |
| P50 latency | 210ms |
| P95 latency | 480ms |
| P99 latency | 890ms |
| Error rate | 0.12% |

## With Cache Warmed
| Metric | Value |
|---|---|
| Requests/sec | 2,340 |
| P50 latency | 8ms |
| Cache hit rate | 71% |

## Cost Optimization
| Model | Traffic | Cost/1k reqs |
|---|---|---|
| gpt-small | 68% | $0.04 |
| gpt-medium | 24% | $0.40 |
| gpt-large | 8% | $4.00 |
| **Blended** | 100% | **$0.39** |
| vs always gpt-large | — | $4.00 |
| **Savings** | — | **90.2%** |
<!-- sustained -->
<!-- cache -->
<!-- cost -->
