"""
backends/mock.py

Deterministic mock backend that simulates inference without loading any models.
Produces realistic latency distributions, L2-normalised embeddings, reranking
scores, and JSON classification results.

In production, swap this for a TransformersBackend or vLLMBackend that calls
the actual model runtime.
"""
from __future__ import annotations

import json
import math
import os
import time
from typing import Iterator, List

# ── Task constants (mirrors proto) ────────────────────────────────────────────

TASK_CHAT      = 1
TASK_SUMMARIZE = 2
TASK_EMBED     = 3
TASK_RERANK    = 4
TASK_CLASSIFY  = 5
TASK_MODERATE  = 6

# ── Model configuration ───────────────────────────────────────────────────────

_MODEL_CONFIGS = {
    "gpt-small":  {"embed_dim": 768,  "avg_latency_ms": 200,  "tps": 50.0},
    "gpt-medium": {"embed_dim": 1024, "avg_latency_ms": 500,  "tps": 30.0},
    "gpt-large":  {"embed_dim": 1536, "avg_latency_ms": 1200, "tps": 15.0},
    "embed-v2":   {"embed_dim": 1536, "avg_latency_ms": 50,   "tps": 200.0},
    "rerank-v1":  {"embed_dim": 768,  "avg_latency_ms": 100,  "tps": 100.0},
}

_DEFAULT_CONFIG = _MODEL_CONFIGS["gpt-small"]
_EXECUTOR_ID    = os.environ.get("EXECUTOR_ID", "executor-0")


# ── Helpers ───────────────────────────────────────────────────────────────────

def _embedding(text: str, dim: int) -> List[float]:
    """Deterministic, L2-normalised embedding based on text hash."""
    if not text:
        return [0.0] * min(dim, 8)
    h = 0
    for ch in text:
        h = (h * 31 + ord(ch)) & 0xFFFFFFFF
    vec = [math.sin(float(h + i)) * math.cos(float(h * i + 1)) for i in range(min(dim, 16))]
    norm = math.sqrt(sum(v * v for v in vec)) or 1.0
    return [v / norm for v in vec]


def _rerank_scores(query: str, docs: List[str]) -> List[float]:
    """Simple word-overlap relevance scoring."""
    q_words = set(query.lower().split())
    scores = []
    for doc in docs:
        if not q_words:
            scores.append(0.0)
            continue
        d_lower = doc.lower()
        matches = sum(1 for w in q_words if w in d_lower)
        scores.append(matches / len(q_words))
    return scores


def _classify(text: str) -> str:
    lower = text.lower()
    if any(w in lower for w in ("hate", "violence", "harm", "threat", "kill")):
        return json.dumps({"label": "harmful",  "confidence": 0.92})
    if any(w in lower for w in ("great", "awesome", "excellent", "wonderful", "amazing")):
        return json.dumps({"label": "positive", "confidence": 0.87})
    if any(w in lower for w in ("bad", "terrible", "horrible", "awful", "worst")):
        return json.dumps({"label": "negative", "confidence": 0.85})
    return json.dumps({"label": "neutral", "confidence": 0.75})


def _estimate_tokens(text: str) -> int:
    return max(1, len(text) // 4)


def _chat_response(model_id: str, prompt: str, messages: list) -> str:
    last = prompt
    if messages:
        for m in reversed(messages):
            if isinstance(m, dict) and m.get("content"):
                last = m["content"]; break
    preview = last[:60] + "..." if len(last) > 60 else last
    ts = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    return f'[{model_id}] Response to: "{preview}" — generated at {ts} by {_EXECUTOR_ID}'


# ── Backend ───────────────────────────────────────────────────────────────────

class MockBackend:
    """
    Deterministic mock inference backend.

    Simulates latency from model config, produces plausible output for all
    supported task types. Thread-safe (no mutable shared state after init).
    """

    def model_ids(self) -> List[str]:
        return list(_MODEL_CONFIGS.keys())

    def run(
        self, *,
        model_id: str,
        task_type: int,
        prompt: str,
        messages: list,
        documents: list,
        query: str,
        max_tokens: int,
        request_id: str,
    ) -> dict:
        cfg   = _MODEL_CONFIGS.get(model_id, _DEFAULT_CONFIG)
        start = time.time()

        # Simulate inference latency
        time.sleep(cfg["avg_latency_ms"] / 1000.0)

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
            emb  = _embedding(text, cfg["embed_dim"])
            result.update({
                "embedding":     emb,
                "tokens_input":  _estimate_tokens(text),
                "tokens_output": 0,
            })

        elif task_type == TASK_RERANK:
            scores = _rerank_scores(query, documents)
            result.update({
                "scores":        scores,
                "tokens_input":  _estimate_tokens(query) + sum(_estimate_tokens(d) for d in documents),
                "tokens_output": 0,
            })

        elif task_type in (TASK_CLASSIFY, TASK_MODERATE):
            content = _classify(prompt)
            result.update({
                "content":       content,
                "tokens_input":  _estimate_tokens(prompt),
                "tokens_output": _estimate_tokens(content),
            })

        else:
            result.update({"content": f"unsupported task {task_type}", "tokens_input": 1, "tokens_output": 1})

        result["latency_ms"] = (time.time() - start) * 1000
        return result

    def stream(
        self, *,
        model_id: str,
        task_type: int,
        prompt: str,
        messages: list,
        max_tokens: int,
        request_id: str,
    ) -> Iterator[str]:
        """Yield individual tokens for streaming inference."""
        # Build the full response first, then stream word by word
        result = self.run(
            model_id=model_id, task_type=task_type,
            prompt=prompt, messages=messages,
            documents=[], query="", max_tokens=max_tokens,
            request_id=request_id,
        )
        content = result.get("content", "")
        for word in content.split():
            yield word + " "
