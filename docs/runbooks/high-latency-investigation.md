# Runbook: High Inference Latency

## Severity
P1 if p99 > 2000ms | P2 if p99 > 500ms

## Steps

### 1. Check worker stats
curl http://localhost:8083/v1/stats

### 2. Check queue depth
curl http://localhost:8082/v1/stats | jq .queue_depth

### 3. Check cache hit rate
curl http://localhost:8084/v1/stats | jq .prompt_cache.metrics.hit_rate
<!-- steps -->
<!-- grafana -->
