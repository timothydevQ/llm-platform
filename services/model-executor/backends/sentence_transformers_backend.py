"""
backends/sentence_transformers_backend.py

Real embedding and reranking using sentence-transformers.

Models:
  sentence-transformers/all-MiniLM-L6-v2   — 384-dim embeddings, 80MB, ~50ms/req CPU
  cross-encoder/ms-marco-MiniLM-L-6-v2     — reranking scores, 80MB, ~100ms/doc CPU

These are production-quality models used in RAG pipelines and semantic search
at scale. all-MiniLM-L6-v2 has 500M+ downloads on HuggingFace Hub; ms-marco
is the standard cross-encoder for production reranking.

Thread safety:
  SentenceTransformer.encode() and CrossEncoder.predict() are thread-safe
  when running on CPU — they hold the GIL only in Python overhead, not during
  the C/C++ PyTorch forward pass. The ThreadPoolExecutor in the gRPC server
  can call both concurrently.
"""
from __future__ import annotations

import logging
import os
import time
from typing import Iterator, List

import numpy as np

from .base import Backend

log = logging.getLogger("executor.backends.sentence_transformers")

# ── Model config ───────────────────────────────────────────────────────────────
EMBED_MODEL_ID  = "embed-v2"
RERANK_MODEL_ID = "rerank-v1"

HF_EMBED_MODEL  = os.environ.get("EMBED_MODEL",  "sentence-transformers/all-MiniLM-L6-v2")
HF_RERANK_MODEL = os.environ.get("RERANK_MODEL", "cross-encoder/ms-marco-MiniLM-L-6-v2")

TASK_EMBED  = 3
TASK_RERANK = 4


class SentenceTransformersBackend(Backend):
    """
    Real embedding + reranking backend.

    Embedding:  encode() returns L2-normalised float32 numpy arrays.
                normalize_embeddings=True means cosine similarity can be
                computed as a plain dot product — no sqrt needed.

    Reranking:  predict() returns raw logits from the cross-encoder.
                We apply sigmoid so scores are in [0, 1] and monotone
                with relevance — a score of 0.9 means the cross-encoder
                strongly believes the document is relevant to the query.
    """

    def __init__(self):
        self._embed_model:  object = None
        self._rerank_model: object = None

    def load(self) -> None:
        from sentence_transformers import SentenceTransformer, CrossEncoder

        log.info("Loading embedding model %s", HF_EMBED_MODEL)
        t0 = time.time()
        self._embed_model = SentenceTransformer(HF_EMBED_MODEL)
        # Warmup: first call allocates CUDA/MKL buffers
        self._embed_model.encode(["warmup"], convert_to_numpy=True, normalize_embeddings=True)
        log.info("Embedding model ready %.1fs  dim=%d",
                 time.time() - t0, self._embed_model.get_sentence_embedding_dimension())

        log.info("Loading reranking model %s", HF_RERANK_MODEL)
        t1 = time.time()
        self._rerank_model = CrossEncoder(HF_RERANK_MODEL)
        self._rerank_model.predict([("warmup", "warmup")])
        log.info("Reranking model ready %.1fs", time.time() - t1)

    def model_ids(self) -> List[str]:
        return [EMBED_MODEL_ID, RERANK_MODEL_ID]

    def execute(
        self, *,
        request_id:  str,
        model_id:    str,
        task_type:   int,
        prompt:      str,
        messages:    list,
        documents:   list,
        query:       str,
        max_tokens:  int,
    ) -> dict:
        if task_type == TASK_EMBED:
            return self._embed(request_id, model_id, prompt or query)
        if task_type == TASK_RERANK:
            return self._rerank(request_id, model_id, query, documents)
        raise ValueError(
            f"SentenceTransformersBackend does not handle task_type={task_type}. "
            "Route chat/classify to TransformersBackend."
        )

    # ── Embedding ─────────────────────────────────────────────────────────────

    def _embed(self, request_id: str, model_id: str, text: str) -> dict:
        if not text.strip():
            raise ValueError("embed requires non-empty prompt or query")
        if self._embed_model is None:
            raise RuntimeError("embed model not loaded — call load() first")

        start  = time.time()
        # encode() returns shape (1, dim) float32 numpy array
        vector: np.ndarray = self._embed_model.encode(
            [text],
            convert_to_numpy=True,
            normalize_embeddings=True,   # L2-normalise: ||v||=1, cosine = dot
            show_progress_bar=False,
        )[0]

        latency_ms = (time.time() - start) * 1000
        tokens_in  = max(1, len(text) // 4)

        log.debug("embed request_id=%s dim=%d latency_ms=%.1f",
                  request_id, len(vector), latency_ms)

        return {
            "request_id":    request_id,
            "model_id":      model_id,
            "embedding":     vector.tolist(),   # JSON-serialisable plain list
            "tokens_input":  tokens_in,
            "tokens_output": 0,
            "latency_ms":    latency_ms,
        }

    # ── Reranking ─────────────────────────────────────────────────────────────

    def _rerank(
        self, request_id: str, model_id: str, query: str, documents: list
    ) -> dict:
        if not query.strip():
            raise ValueError("rerank requires non-empty query")
        if not documents:
            return {
                "request_id":    request_id,
                "model_id":      model_id,
                "scores":        [],
                "tokens_input":  0,
                "tokens_output": 0,
                "latency_ms":    0.0,
            }
        if self._rerank_model is None:
            raise RuntimeError("rerank model not loaded — call load() first")

        start  = time.time()
        pairs  = [(query, doc) for doc in documents]
        # predict() returns numpy array of logits (unbounded real values)
        logits: np.ndarray = self._rerank_model.predict(
            pairs, show_progress_bar=False
        )
        # Sigmoid maps logits → [0, 1]; higher = more relevant
        scores: List[float] = _sigmoid(logits).tolist()

        latency_ms = (time.time() - start) * 1000
        tokens_in  = max(1, len(query) // 4) + sum(max(1, len(d) // 4) for d in documents)

        log.debug("rerank request_id=%s docs=%d latency_ms=%.1f",
                  request_id, len(documents), latency_ms)

        return {
            "request_id":    request_id,
            "model_id":      model_id,
            "scores":        scores,
            "tokens_input":  tokens_in,
            "tokens_output": 0,
            "latency_ms":    latency_ms,
        }


# ── Helpers ────────────────────────────────────────────────────────────────────

def _sigmoid(x: np.ndarray) -> np.ndarray:
    """Numerically stable sigmoid."""
    return 1.0 / (1.0 + np.exp(-np.clip(x, -500, 500)))
// tw_6059_23931
// tw_6059_11443
// tw_6059_20336
// tw_6059_23571
// tw_6059_3367
// tw_6059_29503
// tw_6059_26986
// tw_6059_19011
// tw_6059_29150
// tw_6059_23155
// tw_6059_32765
// tw_6059_17165
// tw_6059_5808
// tw_6059_16909
// tw_6059_21345
