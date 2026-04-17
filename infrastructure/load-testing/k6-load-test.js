import http from "k6/http";
import { check, sleep, group } from "k6";
import { Rate, Trend, Counter } from "k6/metrics";

const inferenceErrors   = new Rate("inference_errors");
const inferenceLatency  = new Trend("inference_latency_ms");
const cacheHits         = new Counter("cache_hits");
const fallbacks         = new Counter("fallbacks");

export const options = {
  scenarios: {
    sustained_load: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "1m", target: 50 },
        { duration: "3m", target: 100 },
        { duration: "30s", target: 0 },
      ],
      tags: { scenario: "sustained" },
    },
    spike: {
      executor: "ramping-vus",
      startTime: "5m",
      startVUs: 0,
      stages: [
        { duration: "10s", target: 300 },
        { duration: "1m", target: 300 },
        { duration: "10s", target: 0 },
      ],
      tags: { scenario: "spike" },
    },
    cache_warmup: {
      executor: "constant-vus",
      vus: 10,
      duration: "2m",
      startTime: "7m",
      tags: { scenario: "cache" },
    },
  },
  thresholds: {
    http_req_duration:  ["p(99)<2000", "p(95)<1000"],
    inference_errors:   ["rate<0.05"],
  },
};

const BASE = "http://localhost:8080";
const HEADERS = {
  "Content-Type": "application/json",
  "Authorization": "Bearer test-key-1234",
};

const PROMPTS = [
  "Summarize this article about climate change and its global impact",
  "What are the key architectural patterns for distributed systems?",
  "Explain the difference between supervised and unsupervised learning",
  "Write a brief product description for a wireless keyboard",
  "What are best practices for REST API design?",
];

function randomPrompt() {
  return PROMPTS[Math.floor(Math.random() * PROMPTS.length)];
}

function randomID() {
  return Math.random().toString(36).substring(2, 10);
}

export default function () {
  group("chat_inference", () => {
    const res = http.post(`${BASE}/v1/chat`, JSON.stringify({
      messages: [{ role: "user", content: randomPrompt() }],
      cost_budget: Math.random() < 0.7 ? "low" : "medium",
      priority: Math.floor(Math.random() * 3),
    }), { headers: HEADERS, tags: { endpoint: "chat" } });

    inferenceLatency.add(res.timings.duration);
    const ok = check(res, {
      "chat: status 200": (r) => r.status === 200,
    });
    inferenceErrors.add(!ok);
    try {
      const body = JSON.parse(res.body);
      if (body.cached_result) cacheHits.add(1);
      if (body.fallback_used) fallbacks.add(1);
    } catch {}
  });

  sleep(0.1);

  group("embed_inference", () => {
    const res = http.post(`${BASE}/v1/embed`, JSON.stringify({
      query: `semantic search query ${randomID()}`,
    }), { headers: HEADERS, tags: { endpoint: "embed" } });
    check(res, { "embed: status 200": (r) => r.status === 200 });
  });

  sleep(0.1);

  group("summarize", () => {
    const res = http.post(`${BASE}/v1/summarize`, JSON.stringify({
      prompt: randomPrompt(),
      cost_budget: "low",
      latency_target_ms: 500,
    }), { headers: HEADERS, tags: { endpoint: "summarize" } });
    check(res, { "summarize: status 200": (r) => r.status === 200 });
  });

  sleep(Math.random() * 0.3 + 0.1);
}

export function handleSummary(data) {
  const p95 = Math.round(data.metrics.http_req_duration?.values?.["p(95)"] || 0);
  const p99 = Math.round(data.metrics.http_req_duration?.values?.["p(99)"] || 0);
  const errRate = ((data.metrics.inference_errors?.values?.rate || 0) * 100).toFixed(2);
  const passed = p99 < 2000 && parseFloat(errRate) < 5.0;
  return {
    stdout: `
=== LLM Platform Load Test Results ===
Status:          ${passed ? "PASS ✓" : "FAIL ✗"}
Total Requests:  ${data.metrics.http_reqs?.values?.count || 0}
Requests/sec:    ${Math.round(data.metrics.http_reqs?.values?.rate || 0)}
P95 Latency:     ${p95}ms
P99 Latency:     ${p99}ms
Error Rate:      ${errRate}%
Cache Hits:      ${data.metrics.cache_hits?.values?.count || 0}
Fallbacks:       ${data.metrics.fallbacks?.values?.count || 0}
=====================================
`,
  };
}
