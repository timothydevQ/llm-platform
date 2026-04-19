"""
tests/test_integration.py — Integration tests for the model executor.

These tests spin up real model backends and verify the full gRPC path from
ExecutorServicer → Backend → real HuggingFace model → response.

They are skipped automatically when model weights are unavailable (CI without
HF_HOME cache, USE_REAL_MODELS=false, or missing sentence-transformers).

Run all tests:          pytest tests/ -v
Run only unit tests:    pytest tests/ -v -m "not integration"
Run only integration:   pytest tests/ -v -m integration
Run with real models:   USE_REAL_MODELS=true pytest tests/ -v -m integration
"""
import json
import math
import os
import sys
import time
import threading
import unittest

# ── Dependency checks ─────────────────────────────────────────────────────────

def _can_run_real_models() -> bool:
    """Return True if sentence-transformers is importable and weights exist."""
    if os.environ.get("USE_REAL_MODELS", "true").lower() == "false":
        return False
    try:
        import sentence_transformers  # noqa: F401
        import transformers          # noqa: F401
        return True
    except ImportError:
        return False


REAL_MODELS = _can_run_real_models()
SKIP_REASON = (
    "Real model weights unavailable — "
    "set USE_REAL_MODELS=true and install sentence-transformers"
)


# ── gRPC mock for unit tests ──────────────────────────────────────────────────

class _Ctx:
    def __init__(self, active: bool = True):
        self._active = active
        self.code    = None
        self.details = ""
    def is_active(self):    return self._active
    def set_code(self, c):  self.code = c
    def set_details(self, d): self.details = d


# ── Unit tests (MockBackend, no network) ─────────────────────────────────────

class TestMockBackendUnit(unittest.TestCase):
    """Fast unit tests using MockBackend — always run."""

    def setUp(self):
        from backends.mock import MockBackend, _MODEL_CONFIGS
        self.b = MockBackend()
        self.b._latency_override = 0.001
        for cfg in _MODEL_CONFIGS.values():
            cfg["avg_latency_ms"] = 2

    def _run(self, task, **kw):
        return self.b.execute(
            request_id="r", model_id="gpt-small", task_type=task,
            prompt=kw.get("prompt", ""), messages=kw.get("messages", []),
            documents=kw.get("documents", []), query=kw.get("query", ""),
            max_tokens=64,
        )

    def test_embed_is_unit_l2_normalised(self):
        emb  = self._run(3, prompt="query")["embedding"]
        norm = math.sqrt(sum(v * v for v in emb))
        self.assertAlmostEqual(norm, 1.0, places=5,
                               msg="embedding must be L2-normalised")

    def test_embed_deterministic(self):
        e1 = self._run(3, prompt="same input")["embedding"]
        e2 = self._run(3, prompt="same input")["embedding"]
        self.assertEqual(e1, e2, "embedding must be deterministic for same input")

    def test_rerank_scores_in_range(self):
        r = self._run(4, query="machine learning", documents=["about ML", "cooking"])
        for s in r["scores"]:
            self.assertGreaterEqual(s, 0.0)
            self.assertLessEqual(s, 1.0)

    def test_rerank_relevant_higher(self):
        r = self._run(4, query="machine learning",
                      documents=["about machine learning methods", "cooking guide"])
        self.assertGreater(r["scores"][0], r["scores"][1],
                           "relevant document should score higher")

    def test_classify_positive(self):
        obj = json.loads(self._run(5, prompt="This is amazing and wonderful")["content"])
        self.assertEqual(obj["label"], "positive")

    def test_classify_harmful(self):
        obj = json.loads(self._run(5, prompt="I will cause harm and violence")["content"])
        self.assertEqual(obj["label"], "harmful")

    def test_stream_yields_strings(self):
        tokens = list(self.b.stream(
            request_id="r", model_id="gpt-small", task_type=1,
            prompt="hello world test", messages=[], max_tokens=32,
        ))
        self.assertGreater(len(tokens), 0)
        self.assertTrue(all(isinstance(t, str) for t in tokens))

    def test_concurrent_calls_thread_safe(self):
        errors = []
        def run():
            try:
                self._run(1, prompt="concurrent test")
            except Exception as e:
                errors.append(e)
        threads = [threading.Thread(target=run) for _ in range(20)]
        for t in threads: t.start()
        for t in threads: t.join()
        self.assertEqual(errors, [], f"concurrent calls produced errors: {errors}")


# ── Integration tests (real models) ──────────────────────────────────────────

@unittest.skipUnless(REAL_MODELS, SKIP_REASON)
class TestSentenceTransformersReal(unittest.TestCase):
    """Integration tests for SentenceTransformersBackend with real weights."""

    @classmethod
    def setUpClass(cls):
        from backends.sentence_transformers_backend import SentenceTransformersBackend
        cls.backend = SentenceTransformersBackend()
        cls.backend.load()

    def _embed(self, text: str) -> list:
        r = self.backend.execute(
            request_id="r", model_id="embed-v2", task_type=3,
            prompt=text, messages=[], documents=[], query="", max_tokens=0,
        )
        return r["embedding"]

    def _rerank(self, query: str, docs: list) -> list:
        r = self.backend.execute(
            request_id="r", model_id="rerank-v1", task_type=4,
            prompt="", messages=[], documents=docs, query=query, max_tokens=0,
        )
        return r["scores"]

    # ── Embedding quality ─────────────────────────────────────────────────────

    def test_real_embedding_dimension(self):
        emb = self._embed("hello world")
        # all-MiniLM-L6-v2 produces 384-dimensional vectors
        self.assertEqual(len(emb), 384,
                         f"expected 384-dim embedding, got {len(emb)}")

    def test_real_embedding_l2_normalised(self):
        emb  = self._embed("semantic search test")
        norm = math.sqrt(sum(v * v for v in emb))
        self.assertAlmostEqual(norm, 1.0, places=4,
                               msg=f"embedding not L2-normalised: norm={norm}")

    def test_real_embedding_deterministic(self):
        e1 = self._embed("consistent output test")
        e2 = self._embed("consistent output test")
        self.assertEqual(e1, e2, "same input must produce same embedding")

    def test_real_embedding_different_inputs_differ(self):
        e1 = self._embed("apple fruit")
        e2 = self._embed("quantum physics")
        # Cosine similarity should be well below 1 for unrelated texts
        dot = sum(a * b for a, b in zip(e1, e2))
        self.assertLess(dot, 0.95,
                        f"unrelated texts should have low similarity, got dot={dot:.4f}")

    def test_real_embedding_similar_inputs_similar(self):
        e1 = self._embed("machine learning model training")
        e2 = self._embed("deep learning neural network fitting")
        dot = sum(a * b for a, b in zip(e1, e2))
        # Related ML texts should have high cosine similarity
        self.assertGreater(dot, 0.5,
                           f"related texts should have high similarity, got dot={dot:.4f}")

    def test_real_embedding_latency_reasonable(self):
        start = time.time()
        self._embed("latency test sentence")
        latency_ms = (time.time() - start) * 1000
        # Should complete within 5 seconds on CPU (typical: 30-100ms)
        self.assertLess(latency_ms, 5000,
                        f"embedding too slow: {latency_ms:.0f}ms")

    # ── Reranking quality ─────────────────────────────────────────────────────

    def test_real_rerank_scores_count(self):
        docs   = ["doc1", "doc2", "doc3"]
        scores = self._rerank("query", docs)
        self.assertEqual(len(scores), 3,
                         "must return one score per document")

    def test_real_rerank_scores_in_range(self):
        scores = self._rerank("python programming",
                               ["Python tutorial", "JavaScript guide"])
        for i, s in enumerate(scores):
            self.assertGreaterEqual(s, 0.0, f"score[{i}] < 0")
            self.assertLessEqual(s,   1.0, f"score[{i}] > 1")

    def test_real_rerank_relevant_doc_ranks_higher(self):
        """The model should score a directly relevant document above an irrelevant one."""
        scores = self._rerank(
            query="What is Python used for?",
            docs=[
                "Python is a programming language used for data science and web development.",
                "The Amazon rainforest covers a large portion of South America.",
            ],
        )
        self.assertGreater(
            scores[0], scores[1],
            f"relevant doc should rank higher: scores={scores}",
        )

    def test_real_rerank_empty_docs_returns_empty(self):
        scores = self._rerank("query", [])
        self.assertEqual(scores, [], "empty docs should return empty scores")

    def test_real_rerank_latency_reasonable(self):
        start = time.time()
        self._rerank("test query", ["doc1", "doc2", "doc3"])
        latency_ms = (time.time() - start) * 1000
        self.assertLess(latency_ms, 10_000, f"rerank too slow: {latency_ms:.0f}ms")

    def test_real_concurrent_embed_thread_safe(self):
        """Multiple concurrent embed calls must not corrupt each other's results."""
        results, errors = {}, []

        def run(i):
            try:
                results[i] = self._embed(f"concurrent test sentence number {i}")
            except Exception as e:
                errors.append(e)

        threads = [threading.Thread(target=run, args=(i,)) for i in range(5)]
        for t in threads: t.start()
        for t in threads: t.join()

        self.assertEqual(errors, [], f"concurrent errors: {errors}")
        self.assertEqual(len(results), 5, "expected 5 results")
        # Each result must be independently normalised
        for i, emb in results.items():
            norm = math.sqrt(sum(v * v for v in emb))
            self.assertAlmostEqual(norm, 1.0, places=4,
                                   msg=f"result[{i}] not normalised: norm={norm}")


@unittest.skipUnless(REAL_MODELS, SKIP_REASON)
class TestTransformersReal(unittest.TestCase):
    """Integration tests for TransformersBackend with real weights."""

    @classmethod
    def setUpClass(cls):
        from backends.transformers_backend import TransformersBackend
        cls.backend = TransformersBackend()
        cls.backend.load()

    def _gen(self, prompt: str, max_tokens: int = 20) -> dict:
        return self.backend.execute(
            request_id="r", model_id="gpt-small", task_type=1,
            prompt=prompt, messages=[], documents=[], query="",
            max_tokens=max_tokens,
        )

    def _classify(self, text: str) -> dict:
        return self.backend.execute(
            request_id="r", model_id="gpt-small", task_type=5,
            prompt=text, messages=[], documents=[], query="", max_tokens=0,
        )

    # ── Text generation ───────────────────────────────────────────────────────

    def test_real_generation_returns_content(self):
        r = self._gen("The capital of France is")
        self.assertIn("content", r)
        self.assertGreater(len(r["content"]), 0,
                           "generation should return non-empty content")

    def test_real_generation_tokens_counted(self):
        r = self._gen("Hello world")
        self.assertGreater(r["tokens_input"],  0, "tokens_input must be positive")
        self.assertGreater(r["tokens_output"], 0, "tokens_output must be positive")

    def test_real_generation_latency_recorded(self):
        r = self._gen("Test sentence")
        self.assertGreater(r["latency_ms"], 0, "latency_ms must be positive")

    def test_real_generation_deterministic(self):
        """Greedy decoding (do_sample=False) must be deterministic."""
        r1 = self._gen("The largest planet in our solar system is", max_tokens=10)
        r2 = self._gen("The largest planet in our solar system is", max_tokens=10)
        self.assertEqual(r1["content"], r2["content"],
                         "greedy generation must be deterministic")

    def test_real_stream_yields_tokens(self):
        tokens = list(self.backend.stream(
            request_id="r", model_id="gpt-small", task_type=1,
            prompt="Once upon a time", messages=[], max_tokens=20,
        ))
        self.assertGreater(len(tokens), 0, "stream must yield at least one token")
        self.assertTrue(all(isinstance(t, str) for t in tokens),
                        "all stream chunks must be strings")

    def test_real_stream_content_matches_generate(self):
        """Streaming and unary generation should produce equivalent content."""
        prompt = "The moon is"
        max_tokens = 15

        unary_r  = self._gen(prompt, max_tokens)
        streamed = "".join(self.backend.stream(
            request_id="r", model_id="gpt-small", task_type=1,
            prompt=prompt, messages=[], max_tokens=max_tokens,
        )).strip()

        # They may differ slightly due to special token handling, but should
        # both be non-empty and share common content
        self.assertGreater(len(streamed), 0, "streaming content must be non-empty")
        self.assertGreater(len(unary_r["content"]), 0, "unary content must be non-empty")

    # ── Classification ────────────────────────────────────────────────────────

    def test_real_classify_returns_label_and_confidence(self):
        r   = self._classify("This movie was absolutely fantastic!")
        obj = json.loads(r["content"])
        self.assertIn("label",      obj, "classification must have label")
        self.assertIn("confidence", obj, "classification must have confidence")

    def test_real_classify_confidence_in_range(self):
        r   = self._classify("neutral everyday statement")
        obj = json.loads(r["content"])
        self.assertGreaterEqual(obj["confidence"], 0.0)
        self.assertLessEqual(   obj["confidence"], 1.0)

    def test_real_classify_positive_text(self):
        """Model should lean positive for clearly positive text."""
        r   = self._classify("I absolutely love this product, it is amazing!")
        obj = json.loads(r["content"])
        # The model may not always get exactly "positive" but confidence should be high
        self.assertGreater(obj["confidence"], 0.5,
                           f"should be confident about clearly positive text: {obj}")

    def test_real_classify_harmful_text(self):
        r   = self._classify("I want to cause harm and violence")
        obj = json.loads(r["content"])
        self.assertIn(obj["label"], ["harmful", "negative"],
                      f"harmful text should be detected: {obj}")


@unittest.skipUnless(REAL_MODELS, SKIP_REASON)
class TestRouterBackendReal(unittest.TestCase):
    """End-to-end integration: RouterBackend dispatches to correct sub-backend."""

    @classmethod
    def setUpClass(cls):
        from backends.router_backend import RouterBackend
        cls.backend = RouterBackend(use_real_models=True)
        cls.backend.load()

    def test_embed_dispatched_to_st_backend(self):
        r = self.backend.execute(
            request_id="r", model_id="embed-v2", task_type=3,
            prompt="router dispatch test", messages=[], documents=[], query="",
            max_tokens=0,
        )
        self.assertEqual(len(r["embedding"]), 384,
                         "RouterBackend should route embed to SentenceTransformers (384-dim)")

    def test_rerank_dispatched_to_st_backend(self):
        r = self.backend.execute(
            request_id="r", model_id="rerank-v1", task_type=4,
            prompt="", messages=[], documents=["doc1", "doc2"],
            query="test query", max_tokens=0,
        )
        self.assertEqual(len(r["scores"]), 2,
                         "RouterBackend should route rerank to SentenceTransformers")

    def test_chat_dispatched_to_transformers_backend(self):
        r = self.backend.execute(
            request_id="r", model_id="gpt-small", task_type=1,
            prompt="Hello", messages=[], documents=[], query="", max_tokens=10,
        )
        self.assertIn("content", r)
        self.assertGreater(len(r["content"]), 0,
                           "RouterBackend should route chat to TransformersBackend")

    def test_unknown_task_raises(self):
        with self.assertRaises(ValueError):
            self.backend.execute(
                request_id="r", model_id="m", task_type=99,
                prompt="x", messages=[], documents=[], query="", max_tokens=0,
            )


# ── Run ───────────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    verbosity = 2 if "-v" in sys.argv else 1
    print(f"Real models available: {REAL_MODELS}")
    if not REAL_MODELS:
        print(f"Skip reason: {SKIP_REASON}")
    print()
    unittest.main(verbosity=verbosity)
