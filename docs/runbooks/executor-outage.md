# Runbook: Model Executor Outage

**Severity triggers**: executor_requests_total stops incrementing; gateway_errors rises > 5%

## Immediate steps

### 1. Check executor health
```bash
curl http://localhost:8085/healthz/ready
# Expected: {"status":"healthy"}
```

### 2. Check circuit breaker state
```bash
curl http://localhost:8081/v1/stats | jq .circuit_breakers
# Any model showing "open" means CB is active
```

### 3. Verify fallback routing is active
```bash
# Next inference should show fallback_used: true
curl -s -X POST http://localhost:8080/v1/classify \
  -H "Authorization: Bearer test-key-1234" \
  -d '{"prompt":"test"}' | jq .fallback_used
```

### 4. Check executor logs
```bash
docker compose logs model-executor --tail=50
# or in Kubernetes:
kubectl logs -n llm-platform -l app=model-executor --tail=50
```

### 5. Simulate degraded mode for testing
```bash
# Degrade (3x latency)
curl -X POST http://localhost:8085/v1/status \
  -d '{"status":"degraded","load_factor":0.8}'

# Full down
curl -X POST http://localhost:8085/v1/status \
  -d '{"status":"down"}'

# Restore
curl -X POST http://localhost:8085/v1/status \
  -d '{"status":"healthy","load_factor":0.0}'
```

### 6. CB recovery timeline
- CB opens after 3 failures
- After 20s, transitions to half-open
- 2 successful probes close the CB
- Monitor: `router_cb_blocked` should drop to 0

## Escalation
If CB does not close after 60s, check executor pod status and restart if needed.
// dc_517
// dc_518
