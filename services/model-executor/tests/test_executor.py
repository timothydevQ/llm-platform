"""
tests/test_executor.py — Test suite for the model executor.

Layer 1: Unit tests — no model weights (MockBackend + pure functions)
Layer 2: Servicer contract tests — ExecutorServicer with MockBackend
Layer 3: Backend interface tests — verify output shapes only

Run: pytest services/model-executor/tests/ -v
"""
import json, math, os, sys, threading, unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../../.."))

from services.model_executor.protos import execution_pb2 as pb2
from services.model_executor.backends.mock import MockBackend
from services.model_executor.server.executor import (
    ExecutorServicer, STATUS_DOWN, STATUS_DEGRADED
)
from services.model_executor.server.metrics import ExecutorMetrics


def _fast_servicer():
    b = MockBackend()
    return ExecutorServicer(b)

class _Ctx:
    def __init__(self, active=True):
        self._active = active; self.code = None; self.details = ""
    def is_active(self): return self._active
    def set_code(self, c): self.code = c
    def set_details(self, d): self.details = d


# ── pb2 message tests ─────────────────────────────────────────────────────────

class TestPb2Messages(unittest.TestCase):

    def test_execute_request_round_trip(self):
        req    = pb2.ExecuteRequest(request_id="r1", model_id="gpt-small",
                                    task_type=1, prompt="hi", max_tokens=128)
        parsed = pb2.ExecuteRequest.FromString(req.SerializeToString())
        self.assertEqual(parsed.request_id, "r1")
        self.assertEqual(parsed.task_type,   1)
        self.assertEqual(parsed.max_tokens,  128)

    def test_execute_request_defaults_empty(self):
        r = pb2.ExecuteRequest()
        self.assertEqual(r.messages,  [])
        self.assertEqual(r.documents, [])

    def test_execute_request_messages(self):
        m   = pb2.ChatMessage(role="user", content="hello")
        req = pb2.ExecuteRequest(request_id="r", model_id="m",
                                 task_type=1, messages=[m])
        d   = json.loads(req.SerializeToString())
        self.assertEqual(d["messages"][0]["role"],    "user")
        self.assertEqual(d["messages"][0]["content"], "hello")

    def test_execute_response_embedding(self):
        resp = pb2.ExecuteResponse(embedding=[0.1, 0.2, 0.3])
        self.assertAlmostEqual(resp.embedding[1], 0.2)

    def test_stream_chunk_done(self):
        c = pb2.StreamChunk(request_id="r", token="", done=True, tokens_out=10)
        self.assertTrue(c.done); self.assertEqual(c.tokens_out, 10)

    def test_task_type_names(self):
        self.assertEqual(pb2.TaskType.name(1), "chat")
        self.assertEqual(pb2.TaskType.name(3), "embed")
        self.assertEqual(pb2.TaskType.name(0), "unspecified")

    def test_health_response_to_dict(self):
        h = pb2.HealthResponse(status="healthy", model_ids=["a","b"])
        d = h.to_dict()
        self.assertEqual(d["status"], "healthy")
        self.assertEqual(len(d["model_ids"]), 2)


# ── MockBackend tests ─────────────────────────────────────────────────────────

class TestMockBackend(unittest.TestCase):

    def setUp(self):
        from services.model_executor.backends import mock as m
        self._orig = {k: v.copy() for k, v in m._MODEL_CONFIGS.items()}
        for cfg in m._MODEL_CONFIGS.values(): cfg["avg_latency_ms"] = 5
        self.b = MockBackend()

    def tearDown(self):
        from services.model_executor.backends import mock as m
        m._MODEL_CONFIGS.update(self._orig)

    def _run(self, task, **kw):
        return self.b.execute(
            request_id="t", model_id="gpt-small", task_type=task,
            prompt=kw.get("prompt",""), messages=kw.get("messages",[]),
            documents=kw.get("documents",[]), query=kw.get("query",""),
            max_tokens=64,
        )

    def test_model_ids(self):
        self.assertIn("gpt-small", self.b.model_ids())
        self.assertIn("embed-v2",  self.b.model_ids())

    def test_chat_content_non_empty(self):
        r = self._run(1, prompt="hi")
        self.assertGreater(len(r["content"]), 0)

    def test_embed_vector(self):
        r = self._run(3, prompt="query")
        self.assertGreater(len(r["embedding"]), 0)

    def test_embed_l2_normalised(self):
        r    = self._run(3, prompt="normalise me")
        norm = math.sqrt(sum(v*v for v in r["embedding"]))
        self.assertAlmostEqual(norm, 1.0, places=5)

    def test_rerank_scores_count(self):
        r = self._run(4, query="q", documents=["d1","d2","d3"])
        self.assertEqual(len(r["scores"]), 3)

    def test_rerank_relevant_higher(self):
        r = self._run(4, query="machine learning",
                      documents=["about ML methods", "cooking guide"])
        self.assertGreater(r["scores"][0], r["scores"][1])

    def test_classify_json_label(self):
        r   = self._run(5, prompt="great product")
        obj = json.loads(r["content"])
        self.assertIn("label", obj)
        self.assertIn("confidence", obj)

    def test_has_latency_ms(self):
        r = self._run(1, prompt="hi")
        self.assertGreater(r["latency_ms"], 0)

    def test_request_id_in_result(self):
        r = self.b.execute(request_id="MY-ID", model_id="gpt-small",
                           task_type=1, prompt="hi", messages=[],
                           documents=[], query="", max_tokens=64)
        self.assertEqual(r["request_id"], "MY-ID")

    def test_stream_yields_strings(self):
        tokens = list(self.b.stream(request_id="s", model_id="gpt-small",
                                    task_type=1, prompt="hello world",
                                    messages=[], max_tokens=64))
        self.assertGreater(len(tokens), 0)
        self.assertTrue(all(isinstance(t, str) for t in tokens))

    def test_unknown_model_uses_default(self):
        r = self.b.execute(request_id="r", model_id="UNKNOWN",
                           task_type=5, prompt="test", messages=[],
                           documents=[], query="", max_tokens=64)
        self.assertIn("content", r)


# ── Metrics tests ─────────────────────────────────────────────────────────────

class TestMetrics(unittest.TestCase):

    def _m(self): return ExecutorMetrics(registry=_new_reg())

    def test_zero_initially(self):
        m = self._m()
        self.assertEqual(m.avg_latency_ms,    0.0)
        self.assertEqual(m.tokens_per_second, 0.0)

    def test_record_increments(self):
        m = self._m()
        m.record(model_id="m", task="t", tokens_in=10, tokens_out=20,
                 latency_s=0.1, success=True)
        self.assertEqual(m.requests, 1); self.assertEqual(m.tokens_out, 20)

    def test_avg_latency(self):
        m = self._m()
        m.record(model_id="m", task="t", tokens_in=0, tokens_out=0,
                 latency_s=0.2, success=True)
        m.record(model_id="m", task="t", tokens_in=0, tokens_out=0,
                 latency_s=0.4, success=True)
        self.assertAlmostEqual(m.avg_latency_ms, 300.0, places=0)

    def test_error_counter(self):
        m = self._m()
        m.record(model_id="m", task="t", tokens_in=0, tokens_out=0,
                 latency_s=0.1, success=False)
        self.assertEqual(m.errors, 1)

    def test_prometheus_output(self):
        m = self._m(); out = m.prometheus_output()
        self.assertIsInstance(out, bytes); self.assertGreater(len(out), 0)


# ── ExecutorServicer tests ────────────────────────────────────────────────────

class TestExecutorServicer(unittest.TestCase):

    def setUp(self):
        from services.model_executor.backends import mock as m
        for cfg in m._MODEL_CONFIGS.values(): cfg["avg_latency_ms"] = 5
        self.svc = _fast_servicer()

    def _req(self, task, **kw):
        return pb2.ExecuteRequest(request_id="r1", model_id="gpt-small",
                                  task_type=task, max_tokens=64, **kw)

    def test_execute_chat(self):
        r = self.svc.Execute(self._req(1, prompt="hi"), _Ctx())
        self.assertGreater(len(r.content), 0)

    def test_execute_embed(self):
        r = self.svc.Execute(
            pb2.ExecuteRequest(request_id="r", model_id="embed-v2",
                               task_type=3, prompt="q", max_tokens=0), _Ctx())
        self.assertGreater(len(r.embedding), 0)

    def test_execute_rerank(self):
        r = self.svc.Execute(
            pb2.ExecuteRequest(request_id="r", model_id="rerank-v1", task_type=4,
                               query="q", documents=["d1","d2"], max_tokens=0), _Ctx())
        self.assertEqual(len(r.scores), 2)

    def test_execute_classify(self):
        r   = self.svc.Execute(self._req(5, prompt="great!"), _Ctx())
        obj = json.loads(r.content)
        self.assertIn("label", obj)

    def test_execute_down_503(self):
        import grpc as _g
        self.svc._status = STATUS_DOWN
        ctx = _Ctx()
        self.svc.Execute(self._req(1, prompt="hi"), ctx)
        self.assertEqual(ctx.code, _g.StatusCode.UNAVAILABLE)

    def test_execute_cancelled(self):
        import grpc as _g
        ctx = _Ctx(active=False)
        self.svc.Execute(self._req(1, prompt="hi"), ctx)
        self.assertEqual(ctx.code, _g.StatusCode.DEADLINE_EXCEEDED)

    def test_execute_increments_metrics(self):
        before = self.svc._metrics.requests
        self.svc.Execute(self._req(5, prompt="test"), _Ctx())
        self.assertGreater(self.svc._metrics.requests, before)

    def test_health_returns_healthy(self):
        r = self.svc.Health(pb2.HealthRequest(), _Ctx())
        self.assertEqual(r.status, "healthy")
        self.assertGreater(len(r.model_ids), 0)

    def test_set_status_degraded(self):
        r = self.svc.SetStatus(
            pb2.SetStatusRequest(status=STATUS_DEGRADED, load_factor=0.8), _Ctx())
        self.assertEqual(self.svc._status, STATUS_DEGRADED)
        self.assertEqual(r.status,         STATUS_DEGRADED)

    def test_stream_yields_done_sentinel(self):
        chunks = list(self.svc.ExecuteStream(self._req(1, prompt="hello"), _Ctx()))
        self.assertGreater(len(chunks), 0)
        self.assertTrue(chunks[-1].done)

    def test_stream_cancel_yields_nothing(self):
        chunks = list(self.svc.ExecuteStream(self._req(1, prompt="hi"), _Ctx(active=False)))
        self.assertEqual(len(chunks), 0)

    def test_concurrent_safe(self):
        errors = []
        def run():
            try: self.svc.Execute(self._req(5, prompt="c"), _Ctx())
            except Exception as e: errors.append(e)
        ts = [threading.Thread(target=run) for _ in range(10)]
        for t in ts: t.start()
        for t in ts: t.join()
        self.assertEqual(errors, [])


def _new_reg():
    try:
        from prometheus_client import CollectorRegistry
        return CollectorRegistry()
    except ImportError:
        return None


if __name__ == "__main__":
    unittest.main()
// tw_6059_24504
// tw_6059_9371
// tw_6059_28310
