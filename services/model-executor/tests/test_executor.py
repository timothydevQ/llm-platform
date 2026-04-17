"""
tests/test_executor.py — Test suite for the model executor service.
Run: pytest services/model-executor/tests/ -v
"""
import json
import math
import sys
import os
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from backends.mock import (
    MockBackend,
    TASK_CHAT, TASK_SUMMARIZE, TASK_EMBED,
    TASK_RERANK, TASK_CLASSIFY, TASK_MODERATE,
    _embedding, _rerank_scores, _classify, _estimate_tokens,
)
from server.main import ExecutorServicer, Metrics


# ── Helper ────────────────────────────────────────────────────────────────────

def fast_backend():
    b = MockBackend()
    # Patch model configs to use minimal latency for tests
    from backends import mock as m
    original = m._MODEL_CONFIGS.copy()
    for cfg in m._MODEL_CONFIGS.values():
        cfg["avg_latency_ms"] = 5
    return b, lambda: m._MODEL_CONFIGS.update(original)


# ── Embedding tests ───────────────────────────────────────────────────────────

class TestEmbedding(unittest.TestCase):

    def test_returns_non_empty(self):
        emb = _embedding("hello world", 8)
        self.assertGreater(len(emb), 0)

    def test_l2_normalised(self):
        emb  = _embedding("test text for normalisation", 8)
        norm = math.sqrt(sum(v * v for v in emb))
        self.assertAlmostEqual(norm, 1.0, places=5)

    def test_deterministic(self):
        self.assertEqual(_embedding("abc", 8), _embedding("abc", 8))

    def test_different_inputs_differ(self):
        self.assertNotEqual(_embedding("text one", 8), _embedding("text two!", 8))

    def test_empty_text_returns_zeros(self):
        emb = _embedding("", 8)
        self.assertEqual(len(emb), 8)
        self.assertTrue(all(v == 0.0 for v in emb))


# ── Rerank tests ──────────────────────────────────────────────────────────────

class TestRerank(unittest.TestCase):

    def test_returns_score_per_doc(self):
        scores = _rerank_scores("cat", ["cat food", "dog food", "fish"])
        self.assertEqual(len(scores), 3)

    def test_relevant_doc_higher_score(self):
        scores = _rerank_scores("machine learning", ["about ML methods", "cooking guide"])
        self.assertGreater(scores[0], scores[1])

    def test_empty_docs_empty_scores(self):
        self.assertEqual(_rerank_scores("q", []), [])

    def test_scores_between_zero_and_one(self):
        scores = _rerank_scores("hello world", ["hello there", "not relevant"])
        for s in scores:
            self.assertGreaterEqual(s, 0.0)
            self.assertLessEqual(s, 1.0)

    def test_empty_query_zero_scores(self):
        scores = _rerank_scores("", ["doc1", "doc2"])
        self.assertTrue(all(s == 0.0 for s in scores))


# ── Classify tests ────────────────────────────────────────────────────────────

class TestClassify(unittest.TestCase):

    def test_positive_label(self):
        r = json.loads(_classify("this is great and wonderful"))
        self.assertEqual(r["label"], "positive")

    def test_negative_label(self):
        r = json.loads(_classify("this is terrible and awful"))
        self.assertEqual(r["label"], "negative")

    def test_harmful_label(self):
        r = json.loads(_classify("hate and violence"))
        self.assertEqual(r["label"], "harmful")

    def test_neutral_label(self):
        r = json.loads(_classify("the weather today"))
        self.assertEqual(r["label"], "neutral")

    def test_has_confidence(self):
        r = json.loads(_classify("any text"))
        self.assertIn("confidence", r)
        self.assertGreater(r["confidence"], 0)


# ── Token estimation tests ────────────────────────────────────────────────────

class TestEstimateTokens(unittest.TestCase):

    def test_non_empty_positive(self):
        self.assertGreater(_estimate_tokens("hello"), 0)

    def test_empty_at_least_one(self):
        self.assertGreaterEqual(_estimate_tokens(""), 1)

    def test_longer_more_tokens(self):
        self.assertGreater(
            _estimate_tokens("this is a much longer text with many words"),
            _estimate_tokens("hi"),
        )


# ── MockBackend tests ─────────────────────────────────────────────────────────

class TestMockBackend(unittest.TestCase):

    def setUp(self):
        from backends import mock as m
        self._original = {k: v.copy() for k, v in m._MODEL_CONFIGS.items()}
        for cfg in m._MODEL_CONFIGS.values():
            cfg["avg_latency_ms"] = 5
        self.backend = MockBackend()

    def tearDown(self):
        from backends import mock as m
        m._MODEL_CONFIGS.update(self._original)

    def _run(self, task, **kw):
        return self.backend.run(
            model_id="gpt-small", task_type=task,
            prompt=kw.get("prompt",""), messages=kw.get("messages",[]),
            documents=kw.get("documents",[]), query=kw.get("query",""),
            max_tokens=512, request_id="test-req",
        )

    def test_model_ids(self):
        ids = self.backend.model_ids()
        self.assertIn("gpt-small", ids)
        self.assertIn("embed-v2", ids)

    def test_chat_returns_content(self):
        r = self._run(TASK_CHAT, prompt="hello")
        self.assertIn("content", r)
        self.assertGreater(len(r["content"]), 0)

    def test_summarize_returns_content(self):
        r = self._run(TASK_SUMMARIZE, prompt="long article")
        self.assertIn("content", r)

    def test_embed_returns_vector(self):
        r = self._run(TASK_EMBED, prompt="query")
        self.assertIn("embedding", r)
        self.assertGreater(len(r["embedding"]), 0)

    def test_embed_uses_query_fallback(self):
        r = self._run(TASK_EMBED, query="fallback query")
        self.assertIn("embedding", r)

    def test_rerank_returns_scores(self):
        r = self._run(TASK_RERANK, query="q", documents=["d1","d2","d3"])
        self.assertIn("scores", r)
        self.assertEqual(len(r["scores"]), 3)

    def test_classify_returns_json_label(self):
        r = self._run(TASK_CLASSIFY, prompt="great!")
        self.assertIn("content", r)
        label_obj = json.loads(r["content"])
        self.assertIn("label", label_obj)

    def test_moderate_returns_result(self):
        r = self._run(TASK_MODERATE, prompt="some text")
        self.assertIn("content", r)

    def test_has_latency_ms(self):
        r = self._run(TASK_CHAT, prompt="hi")
        self.assertIn("latency_ms", r)
        self.assertGreater(r["latency_ms"], 0)

    def test_has_tokens_output(self):
        r = self._run(TASK_CHAT, prompt="hello")
        self.assertIn("tokens_output", r)
        self.assertGreater(r["tokens_output"], 0)

    def test_unknown_model_uses_default(self):
        r = self.backend.run(
            model_id="unknown-model", task_type=TASK_CLASSIFY,
            prompt="test", messages=[], documents=[], query="",
            max_tokens=512, request_id="r",
        )
        self.assertIn("content", r)

    def test_stream_yields_tokens(self):
        tokens = list(self.backend.stream(
            model_id="gpt-small", task_type=TASK_CHAT,
            prompt="hello world", messages=[], max_tokens=64, request_id="r",
        ))
        self.assertGreater(len(tokens), 0)
        self.assertTrue(all(isinstance(t, str) for t in tokens))

    def test_request_id_in_result(self):
        r = self._run(TASK_CHAT, prompt="hi")
        self.assertEqual(r["request_id"], "test-req")

    def test_model_id_in_result(self):
        r = self._run(TASK_CHAT, prompt="hi")
        self.assertEqual(r["model_id"], "gpt-small")


# ── Metrics tests ─────────────────────────────────────────────────────────────

class TestMetrics(unittest.TestCase):

    def test_avg_latency_no_requests(self):
        m = Metrics()
        self.assertEqual(m.avg_latency_ms, 0.0)

    def test_tokens_per_second_no_data(self):
        m = Metrics()
        self.assertEqual(m.tokens_per_second, 0.0)

    def test_record_increments(self):
        m = Metrics()
        m.record(10, 20, 0.1)
        self.assertEqual(m.requests,   1)
        self.assertEqual(m.tokens_in,  10)
        self.assertEqual(m.tokens_out, 20)

    def test_avg_latency_calculated(self):
        m = Metrics()
        m.record(0, 0, 0.2)
        m.record(0, 0, 0.4)
        self.assertAlmostEqual(m.avg_latency_ms, 300.0, places=1)

    def test_error_counter(self):
        m = Metrics()
        m.record_error()
        m.record_error()
        self.assertEqual(m.errors, 2)


# ── ExecutorServicer tests ────────────────────────────────────────────────────

class MockContext:
    """Minimal mock for grpc.ServicerContext."""
    def __init__(self): self._active = True; self._code = None; self._details = ""
    def is_active(self): return self._active
    def set_code(self, c): self._code = c
    def set_details(self, d): self._details = d

class TestExecutorServicer(unittest.TestCase):

    def setUp(self):
        from backends import mock as m
        for cfg in m._MODEL_CONFIGS.values():
            cfg["avg_latency_ms"] = 5
        self.svc = ExecutorServicer(backend=MockBackend())

    def test_execute_chat(self):
        req = {"request_id":"r1","model_id":"gpt-small","task_type":TASK_CHAT,"prompt":"hi","messages":[],"documents":[],"query":"","max_tokens":64}
        resp = self.svc.Execute(req, MockContext())
        self.assertIn("content", resp)
        self.assertGreater(len(resp["content"]), 0)

    def test_execute_embed(self):
        req = {"request_id":"r2","model_id":"embed-v2","task_type":TASK_EMBED,"prompt":"search","messages":[],"documents":[],"query":"","max_tokens":64}
        resp = self.svc.Execute(req, MockContext())
        self.assertIn("embedding", resp)

    def test_execute_rerank(self):
        req = {"request_id":"r3","model_id":"rerank-v1","task_type":TASK_RERANK,"prompt":"","messages":[],"documents":["d1","d2"],"query":"q","max_tokens":64}
        resp = self.svc.Execute(req, MockContext())
        self.assertIn("scores", resp)
        self.assertEqual(len(resp["scores"]), 2)

    def test_execute_down_returns_empty(self):
        self.svc._status = "down"
        ctx = MockContext()
        resp = self.svc.Execute({"request_id":"r","model_id":"gpt-small"}, ctx)
        self.assertEqual(resp, {})
        import grpc as _grpc
        self.assertEqual(ctx._code, _grpc.StatusCode.UNAVAILABLE)

    def test_health_response(self):
        resp = self.svc.Health({}, MockContext())
        self.assertIn("executor_id", resp)
        self.assertIn("status", resp)
        self.assertEqual(resp["status"], "healthy")

    def test_set_status(self):
        resp = self.svc.SetStatus({"status":"degraded","load_factor":0.8}, MockContext())
        self.assertEqual(resp["status"], "degraded")
        self.assertEqual(self.svc._status, "degraded")

    def test_execute_stream_yields_chunks(self):
        req = {"request_id":"r4","model_id":"gpt-small","task_type":TASK_CHAT,"prompt":"hello world","messages":[],"max_tokens":64}
        ctx = MockContext()
        chunks = list(self.svc.ExecuteStream(req, ctx))
        self.assertGreater(len(chunks), 0)
        last = chunks[-1]
        self.assertTrue(last.get("done", False))

    def test_execute_increments_metrics(self):
        req = {"request_id":"r5","model_id":"gpt-small","task_type":TASK_CLASSIFY,"prompt":"test","messages":[],"documents":[],"query":"","max_tokens":64}
        before = self.svc._metrics.requests
        self.svc.Execute(req, MockContext())
        self.assertEqual(self.svc._metrics.requests, before + 1)


if __name__ == "__main__":
    unittest.main()
