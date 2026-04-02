"""
backends/mock.py — Deterministic mock backend.

Used by:
  - Unit tests (no model weights required, fast CI)
  - Local dev when USE_REAL_MODELS=false
  - Benchmarking the serving infrastructure without ML bottleneck

Produces:
  - Deterministic, L2-normalised embedding vectors (hash-based)
  - Word-overlap reranking scores
  - Rule-based sentiment/safety classification labels
  - Plausible chat responses with model_id and request_id in body

All outputs are deterministic for the same input — important for
cache hit-rate tests and reproducibility.
"""
from __future__ import annotations

import json
import math
import os
import time
from typing import Iterator, List

from .base import Backend

# ── Task type constants (mirror proto/inference/v1/inference.proto) ───────────
TASK_CHAT      = 1
TASK_SUMMARIZE = 2
TASK_EMBED     = 3
TASK_RERANK    = 4
TASK_CLASSIFY  = 5
TASK_MODERATE  = 6

EXECUTOR_ID = os.environ.get("EXECUTOR_ID", "executor-0")

# ── Per-model config ──────────────────────────────────────────────────────────
# avg_latency_ms is overridden to 5ms in tests; kept realistic for local dev.
_MODEL_CONFIGS: dict[str, dict] = {
    "gpt-small":  {"embed_dim": 384,  "avg_latency_ms": 200,  "tps": 50.0},
    "gpt-medium": {"embed_dim": 768,  "avg_latency_ms": 500,  "tps": 30.0},
    "gpt-large":  {"embed_dim": 1536, "avg_latency_ms": 1200, "tps": 15.0},
    "embed-v2":   {"embed_dim": 384,  "avg_latency_ms": 50,   "tps": 200.0},
    "rerank-v1":  {"embed_dim": 384,  "avg_latency_ms": 100,  "tps": 100.0},
}
_DEFAULT_CONFIG = _MODEL_CONFIGS["gpt-small"]


# ── Pure helpers ──────────────────────────────────────────────────────────────

def _deterministic_embedding(text: str, dim: int) -> List[float]:
    """Return a deterministic, L2-normalised float vector."""
    if not text:
        return [0.0] * min(dim, 8)
    h = 0
    for ch in text:
        h = (h * 31 + ord(ch)) & 0xFFFFFFFF
    raw = [math.sin(float(h + i)) * math.cos(float(h * i + 1)) for i in range(min(dim, 16))]
    norm = math.sqrt(sum(v * v for v in raw)) or 1.0
    return [v / norm for v in raw]


def _word_overlap_scores(query: str, docs: List[str]) -> List[float]:
    """Score documents by word-overlap with the query."""
    q_words = set(query.lower().split())
    scores = []
    for doc in docs:
        if not q_words:
            scores.append(0.0)
        else:
            hits = sum(1 for w in q_words if w in doc.lower())
            scores.append(hits / len(q_words))
    return scores


def _rule_classify(text: str) -> str:
    lower = text.lower()
    if any(w in lower for w in ("hate", "violence", "harm", "threat", "kill", "attack")):
        return json.dumps({"label": "harmful",  "confidence": 0.92})
    if any(w in lower for w in ("great", "awesome", "excellent", "wonderful", "amazing", "love")):
        return json.dumps({"label": "positive", "confidence": 0.87})
    if any(w in lower for w in ("bad", "terrible", "horrible", "awful", "worst", "hate")):
        return json.dumps({"label": "negative", "confidence": 0.85})
    return json.dumps({"label": "neutral", "confidence": 0.75})


def _estimate_tokens(text: str) -> int:
    """~4 characters per token (GPT-2 approximation)."""
    return max(1, len(text) // 4)


def _chat_response(model_id: str, prompt: str, messages: list) -> str:
    last = prompt
    if messages:
        for m in reversed(messages):
            c = m.get("content","") if isinstance(m, dict) else getattr(m,"content","")
            if c:
                last = c
                break
    preview = last[:60] + "..." if len(last) > 60 else last
    ts = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    return f'[{model_id}@{EXECUTOR_ID}] Response to: "{preview}" — {ts}'


# ── Backend ───────────────────────────────────────────────────────────────────

class MockBackend(Backend):
    """
    Deterministic mock that simulates inference without model weights.

    Thread-safe: all methods are stateless after __init__.
    The _latency_override attribute (set by tests) overrides the per-model
    avg_latency_ms so the test suite runs in milliseconds, not seconds.
    """

    def __init__(self):
        self._latency_override: float | None = None

    def load(self) -> None:
        """No-op — mock has no weights to load."""

    def model_ids(self) -> List[str]:
        return list(_MODEL_CONFIGS.keys())

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
        cfg   = _MODEL_CONFIGS.get(model_id, _DEFAULT_CONFIG)
        lat_s = (self._latency_override or cfg["avg_latency_ms"] / 1000.0)
        time.sleep(lat_s)
        start = time.time()

        result: dict = {"request_id": request_id, "model_id": model_id}

        if task_type in (TASK_CHAT, TASK_SUMMARIZE):
            content = _chat_response(model_id, prompt, messages)
            result.update({
                "content":       content,
                "tokens_input":  _estimate_tokens(prompt),
                "tokens_output": _estimate_tokens(content),
            })
        elif task_type == TASK_EMBED:
            text = prompt or query
            result.update({
                "embedding":     _deterministic_embedding(text, cfg["embed_dim"]),
                "tokens_input":  _estimate_tokens(text),
                "tokens_output": 0,
            })
        elif task_type == TASK_RERANK:
            result.update({
                "scores":        _word_overlap_scores(query, documents),
                "tokens_input":  _estimate_tokens(query) + sum(_estimate_tokens(d) for d in documents),
                "tokens_output": 0,
            })
        elif task_type in (TASK_CLASSIFY, TASK_MODERATE):
            content = _rule_classify(prompt)
            result.update({
                "content":       content,
                "tokens_input":  _estimate_tokens(prompt),
                "tokens_output": _estimate_tokens(content),
            })
        else:
            result.update({"content": f"unsupported task {task_type}",
                           "tokens_input": 1, "tokens_output": 1})

        result["latency_ms"] = (time.time() - start + lat_s) * 1000
        return result

    def stream(
        self, *,
        request_id: str,
        model_id:   str,
        task_type:  int,
        prompt:     str,
        messages:   list,
        max_tokens: int,
    ) -> Iterator[str]:
        """Yield words one at a time to simulate token streaming."""
        result  = self.execute(
            request_id=request_id, model_id=model_id, task_type=task_type,
            prompt=prompt, messages=messages, documents=[], query="",
            max_tokens=max_tokens,
        )
        content = result.get("content", "")
        for word in content.split():
            yield word + " "
// tw_6059_15030
// tw_6059_15816
// tw_6059_19104
// tw_6059_13855
// tw_6059_10175
// tw_6059_4490
// tw_6059_19392
// tw_6059_23746
// tw_6059_32599
// tw_6059_7752
// tw_6059_30581
// tw_6059_20554
// tw_6059_18373
// tw_6059_6002
// tw_6059_21111
// tw_6059_10495
// tw_6059_2600
