import http from "k6/http";
import { check, sleep, group } from "k6";
import { Rate, Trend, Counter, Gauge } from "k6/metrics";

// ── Custom metrics ────────────────────────────────────────────────────────────
const inferenceErrors     = new Rate("inference_errors");
const inferenceLatency    = new Trend("inference_latency_ms");
const routerFallbacks     = new Counter("router_fallbacks");
const cacheHits           = new Counter("cache_hits");
const queueDepth          = new Gauge("scheduler_queue_depth");
const costPerRequest      = new Trend("cost_per_request_usd");

// ── Scenarios ─────────────────────────────────────────────────────────────────
export const options = {
  scenarios: {
    // Scenario 1: Sustained mixed load
    sustained: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "1m",  target: 50  },
        { duration: "3m",  target: 100 },
        { duration: "30s", target: 0   },
      ],
      tags: { scenario: "sustained" },
    },
    // Scenario 2: Spike — tests scheduler backpressure
    spike: {
      executor: "ramping-vus",
      startTime: "5m",
      startVUs: 0,
      stages: [
        { duration: "10s", target: 300 },
        { duration: "1m",  target: 300 },
        { duration: "10s", target: 0   },
      ],
      tags: { scenario: "spike" },
    },
    // Scenario 3: Embed cache warmup
    cache_warmup: {
      executor: "constant-vus",
      vus: 20,
      duration: "2m",
      startTime: "7m",
      tags: { scenario: "cache_warmup" },
    },
  },

  // SLO thresholds — CI will fail if these are breached
  thresholds: {
    http_req_duration:  ["p(99)<2000", "p(95)<1000", "p(50)<500"],
    inference_errors:   ["rate<0.05"],
    "http_req_duration{endpoint:embed}":     ["p(99)<200"],
    "http_req_duration{endpoint:classify}":  ["p(99)<800"],
    "http_req_duration{endpoint:chat}":      ["p(99)<2000"],
  },
};

// ── Config ────────────────────────────────────────────────────────────────────
const BASE    = __ENV.TARGET || "http://localhost:8080";
const API_KEY = __ENV.API_KEY || "test-key-1234";
const HEADERS = {
  "Content-Type":  "application/json",
  "Authorization": `Bearer ${API_KEY}`,
};

const PROMPTS = [
  "Summarize this article about machine learning infrastructure at scale",
  "What are the key differences between synchronous and asynchronous inference?",
  "Explain the tradeoffs between batching and latency in model serving systems",
  "Write a brief product description for an AI-powered recommendation engine",
  "What design patterns are most effective for distributed systems observability?",
];

const COST_BUDGETS = ["low", "low", "low", "medium", "high"]; // 60% low, 20% medium, 20% high

function rand(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

function randomId() {
  return Math.random().toString(36).substring(2, 10);
}

// ── Request helpers ───────────────────────────────────────────────────────────

function chat(costBudget) {
  const res = http.post(`${BASE}/v1/chat`, JSON.stringify({
    messages:    [{ role: "user", content: rand(PROMPTS) }],
    cost_budget: costBudget,
    priority:    Math.random() < 0.1 ? 2 : 1,  // 10% high priority
  }), { headers: HEADERS, tags: { endpoint: "chat" } });

  inferenceLatency.add(res.timings.duration);
  const ok = check(res, { "chat: status 200": r => r.status === 200 });
  inferenceErrors.add(!ok);

  if (res.status === 200) {
    const body = parseBody(res.body);
    if (body.fallback_used) routerFallbacks.add(1);
    if (body.cached)        cacheHits.add(1);
    if (body.cost_usd)      costPerRequest.add(body.cost_usd);
  }
}

function embed() {
  const res = http.post(`${BASE}/v1/embed`, JSON.stringify({
    query: `semantic search ${randomId()} products recommendations`,
  }), { headers: HEADERS, tags: { endpoint: "embed" } });

  check(res, { "embed: status 200": r => r.status === 200 });
  inferenceErrors.add(res.status !== 200);
}

function classify() {
  const texts = [
    "This product is absolutely amazing!",
    "The service was terrible, very disappointed.",
    "Normal day, nothing special happened.",
    "I hate everything about this experience.",
  ];
  const res = http.post(`${BASE}/v1/classify`, JSON.stringify({
    prompt: rand(texts),
    cost_budget: "low",
  }), { headers: HEADERS, tags: { endpoint: "classify" } });

  check(res, { "classify: status 200": r => r.status === 200 });
  inferenceErrors.add(res.status !== 200);
}

function checkSchedulerDepth() {
  const res = http.get(`${BASE.replace(":8080", ":8082")}/v1/stats`);
  if (res.status === 200) {
    const body = parseBody(res.body);
    const depths = body.queue_depths || {};
    const maxDepth = Math.max(...Object.values(depths).map(Number), 0);
    queueDepth.add(maxDepth);
  }
}

function parseBody(raw) {
  try { return JSON.parse(raw); } catch { return {}; }
}

// ── Main VU function ──────────────────────────────────────────────────────────

export default function () {
  const budget = rand(COST_BUDGETS);

  group("chat_inference", () => { chat(budget); });
  sleep(0.1);

  group("embed_inference", () => { embed(); });
  sleep(0.1);

  if (Math.random() < 0.3) {
    group("classify_inference", () => { classify(); });
    sleep(0.05);
  }

  // Sample scheduler depth 5% of the time (low overhead)
  if (Math.random() < 0.05) {
    checkSchedulerDepth();
  }

  sleep(Math.random() * 0.3 + 0.1);
}

// ── Summary ───────────────────────────────────────────────────────────────────

export function handleSummary(data) {
  const dur   = data.metrics.http_req_duration?.values;
  const p50   = Math.round(dur?.["p(50)"] || 0);
  const p95   = Math.round(dur?.["p(95)"] || 0);
  const p99   = Math.round(dur?.["p(99)"] || 0);
  const total = data.metrics.http_reqs?.values?.count || 0;
  const rps   = Math.round(data.metrics.http_reqs?.values?.rate || 0);
  const errPct = ((data.metrics.inference_errors?.values?.rate || 0) * 100).toFixed(2);
  const fb    = data.metrics.router_fallbacks?.values?.count || 0;
  const hits  = data.metrics.cache_hits?.values?.count || 0;
  const sloOk = p99 < 2000 && parseFloat(errPct) < 5.0;

  return {
    stdout: `
╔══════════════════════════════════════════════════╗
║         LLM Platform Load Test Results           ║
╠══════════════════════════════════════════════════╣
║  Status:          ${sloOk ? "✓ PASS" : "✗ FAIL"}                        ║
║  Total Requests:  ${String(total).padEnd(10)}                ║
║  Requests/sec:    ${String(rps).padEnd(10)}                ║
║  P50 Latency:     ${String(p50 + "ms").padEnd(10)}                ║
║  P95 Latency:     ${String(p95 + "ms").padEnd(10)}                ║
║  P99 Latency:     ${String(p99 + "ms").padEnd(10)}                ║
║  Error Rate:      ${String(errPct + "%").padEnd(10)}                ║
║  Router Fallbacks:${String(fb).padEnd(10)}                ║
║  Cache Hits:      ${String(hits).padEnd(10)}                ║
╚══════════════════════════════════════════════════╝
`,
  };
}
// tw_6059_29692
// tw_6059_852
// tw_6059_23889
// tw_6059_5237
// tw_6059_1386
// tw_6059_2434
// tw_6059_22857
// tw_6059_13487
// tw_6059_8589
