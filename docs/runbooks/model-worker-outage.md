# Runbook: Model Worker Outage

## Immediate Steps

### 1. Check worker health
curl http://localhost:8083/healthz/ready

### 2. Check circuit breaker states
curl http://localhost:8081/v1/stats | jq .circuit_breakers

### 3. Verify fallback routing
curl -X POST http://localhost:8080/v1/chat \
  -H "Authorization: Bearer test-key-1234" \
  -H "Content-Type: application/json" \
  -d '{"prompt":"test"}'

Should see fallback_used: true

### 4. Restore worker
curl -X POST http://localhost:8083/v1/status \
  -d '{"status":"healthy"}'
<!-- steps -->
<!-- metrics -->
<!-- escalation -->
