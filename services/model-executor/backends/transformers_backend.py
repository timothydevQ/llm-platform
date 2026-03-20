"""
backends/transformers_backend.py

Real text generation and classification using HuggingFace Transformers.

Models:
  Chat / Summarize: facebook/opt-125m  — 125M causal LM, CPU-capable (~500MB)
  Classify / Moderate: cross-encoder/nli-distilroberta-base — zero-shot NLI

Streaming: uses transformers.TextIteratorStreamer so tokens are yielded
incrementally from the model's decoding loop — not post-hoc word-splitting.
The streamer runs generation in a background thread; the main thread yields
chunks as they arrive, checking context.is_active() on each one.

Production upgrade path:
  Replace _gen_pipeline with a vLLM AsyncEngine call:
    from vllm import AsyncLLMEngine, SamplingParams
  The stream() method signature stays identical; only the internals change.
"""
from __future__ import annotations

import json
import logging
import os
import threading
import time
from queue import Empty, Queue
from typing import Iterator, List

from .base import Backend

log = logging.getLogger("executor.backends.transformers")

# ── Model config ───────────────────────────────────────────────────────────────
CHAT_MODEL_ID   = "gpt-small"
MEDIUM_MODEL_ID = "gpt-medium"
LARGE_MODEL_ID  = "gpt-large"

HF_CHAT_MODEL     = os.environ.get("CHAT_MODEL",     "facebook/opt-125m")
HF_CLASSIFY_MODEL = os.environ.get("CLASSIFY_MODEL", "cross-encoder/nli-distilroberta-base")

TASK_CHAT      = 1
TASK_SUMMARIZE = 2
TASK_CLASSIFY  = 5
TASK_MODERATE  = 6

SENTIMENT_LABELS = ["positive", "negative", "neutral"]
SAFETY_LABELS    = ["safe content", "harmful content", "spam"]


class TransformersBackend(Backend):
    """
    Real text-generation and classification backend.

    Thread-safe: HuggingFace pipelines release the GIL during C extension
    calls, so concurrent requests from a ThreadPoolExecutor work correctly.
    The TextIteratorStreamer for streaming is per-request (not shared state).
    """

    def __init__(self):
        self._gen_pipeline      = None
        self._classify_pipeline = None
        self._tokenizer         = None

    def load(self) -> None:
        from transformers import pipeline, AutoTokenizer

        log.info("Loading text-generation model %s", HF_CHAT_MODEL)
        t0 = time.time()
        self._gen_pipeline = pipeline(
            "text-generation",
            model=HF_CHAT_MODEL,
            device=-1,           # CPU; set device=0 for first GPU
            truncation=True,
            pad_token_id=50256,  # EOS token used as pad for open-end generation
        )
        # Load tokenizer separately for TextIteratorStreamer
        self._tokenizer = AutoTokenizer.from_pretrained(HF_CHAT_MODEL)
        # Warmup: JIT-compile kernels so first real request isn't slow
        self._gen_pipeline("Hello", max_new_tokens=5, do_sample=False)
        log.info("Text-generation ready in %.1fs", time.time() - t0)

        log.info("Loading zero-shot classification model %s", HF_CLASSIFY_MODEL)
        t1 = time.time()
        self._classify_pipeline = pipeline(
            "zero-shot-classification",
            model=HF_CLASSIFY_MODEL,
            device=-1,
        )
        self._classify_pipeline("warmup", candidate_labels=["a", "b"])
        log.info("Classification ready in %.1fs", time.time() - t1)

    def model_ids(self) -> List[str]:
        return [CHAT_MODEL_ID, MEDIUM_MODEL_ID, LARGE_MODEL_ID]

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
        if task_type in (TASK_CHAT, TASK_SUMMARIZE):
            return self._generate(request_id, model_id, prompt, messages, max_tokens)
        if task_type in (TASK_CLASSIFY, TASK_MODERATE):
            return self._classify(request_id, model_id, prompt, task_type)
        raise ValueError(
            f"TransformersBackend does not handle task_type={task_type}. "
            "Route embed/rerank to SentenceTransformersBackend."
        )

    # ── Text generation ───────────────────────────────────────────────────────

    def _generate(
        self,
        request_id: str,
        model_id:   str,
        prompt:     str,
        messages:   list,
        max_tokens: int,
    ) -> dict:
        if self._gen_pipeline is None:
            raise RuntimeError("text-generation pipeline not loaded — call load() first")

        text = _build_prompt(prompt, messages)
        cap  = min(max_tokens or 128, 512)

        start = time.time()
        output = self._gen_pipeline(
            text,
            max_new_tokens=cap,
            do_sample=False,   # greedy decoding — deterministic, no temperature
            truncation=True,
            return_full_text=False,  # return only the new tokens
        )
        generated = output[0]["generated_text"].strip()
        latency_s = time.time() - start

        tokens_in  = len(self._tokenizer.encode(text))          if self._tokenizer else _est(text)
        tokens_out = len(self._tokenizer.encode(generated))     if self._tokenizer else _est(generated)

        log.info(
            "generate request_id=%s model=%s tokens_in=%d tokens_out=%d latency_ms=%.1f",
            request_id, model_id, tokens_in, tokens_out, latency_s * 1000,
        )
        return {
            "request_id":    request_id,
            "model_id":      model_id,
            "content":       generated,
            "tokens_input":  tokens_in,
            "tokens_output": tokens_out,
            "latency_ms":    latency_s * 1000,
        }

    # ── Real token streaming via TextIteratorStreamer ─────────────────────────

    def stream(
        self, *,
        request_id: str,
        model_id:   str,
        task_type:  int,
        prompt:     str,
        messages:   list,
        max_tokens: int,
    ) -> Iterator[str]:
        """
        Yield tokens incrementally using transformers.TextIteratorStreamer.

        Architecture:
          1. Create a TextIteratorStreamer backed by the model's tokenizer.
          2. Launch model.generate() in a background thread — it writes decoded
             token strings to the streamer as each token is produced.
          3. This thread yields tokens from the streamer as they arrive.

        This is genuine incremental decoding, not post-hoc splitting.
        The caller (ExecutorServicer) checks context.is_active() on each
        yielded token so client disconnects abort generation immediately.
        """
        if self._gen_pipeline is None:
            raise RuntimeError("text-generation pipeline not loaded — call load() first")

        try:
            from transformers import TextIteratorStreamer
        except ImportError:
            # Older transformers versions — fall back to full generation
            log.warning("TextIteratorStreamer unavailable, falling back to batch generation")
            result = self._generate(request_id, model_id, prompt, messages, max_tokens)
            for token in result["content"].split():
                yield token + " "
            return

        text    = _build_prompt(prompt, messages)
        cap     = min(max_tokens or 128, 512)
        model   = self._gen_pipeline.model
        tokenizer = self._gen_pipeline.tokenizer

        inputs  = tokenizer(text, return_tensors="pt", truncation=True, max_length=1024)
        streamer = TextIteratorStreamer(
            tokenizer,
            skip_prompt=True,        # don't re-emit the input tokens
            skip_special_tokens=True,
        )

        generate_kwargs = dict(
            **inputs,
            max_new_tokens=cap,
            do_sample=False,
            streamer=streamer,
        )

        # Run generation in background thread so we can yield from main thread
        gen_thread = threading.Thread(target=model.generate, kwargs=generate_kwargs, daemon=True)
        gen_thread.start()

        try:
            for token_text in streamer:
                if token_text:
                    yield token_text
        finally:
            gen_thread.join(timeout=5)

    # ── Classification ────────────────────────────────────────────────────────

    def _classify(
        self,
        request_id: str,
        model_id:   str,
        text:       str,
        task_type:  int,
    ) -> dict:
        if self._classify_pipeline is None:
            raise RuntimeError("classification pipeline not loaded — call load() first")

        labels = SAFETY_LABELS if task_type == TASK_MODERATE else SENTIMENT_LABELS
        start  = time.time()

        result     = self._classify_pipeline(text, candidate_labels=labels)
        top_label  = result["labels"][0]
        top_score  = result["scores"][0]

        # Normalise safety label names
        label_map  = {"safe content": "safe", "harmful content": "harmful", "spam": "spam"}
        canonical  = label_map.get(top_label, top_label)

        content    = json.dumps({"label": canonical, "confidence": round(top_score, 4)})
        latency_s  = time.time() - start
        tokens_in  = len(self._tokenizer.encode(text)) if self._tokenizer else _est(text)

        log.debug(
            "classify request_id=%s label=%s confidence=%.4f latency_ms=%.1f",
            request_id, canonical, top_score, latency_s * 1000,
        )
        return {
            "request_id":    request_id,
            "model_id":      model_id,
            "content":       content,
            "tokens_input":  tokens_in,
            "tokens_output": _est(content),
            "latency_ms":    latency_s * 1000,
        }


# ── Helpers ────────────────────────────────────────────────────────────────────

def _build_prompt(prompt: str, messages: list) -> str:
    if messages:
        parts = []
        for m in messages:
            role    = m.get("role",    "user")    if isinstance(m, dict) else getattr(m, "role",    "user")
            content = m.get("content", "")        if isinstance(m, dict) else getattr(m, "content", "")
            parts.append(f"{role.capitalize()}: {content}")
        return "\n".join(parts) + "\nAssistant:"
    return prompt


def _est(text: str) -> int:
    return max(1, len(text) // 4)
// tw_6059_21056
// tw_6059_30308
// tw_6059_21099
// tw_6059_29932
// tw_6059_27986
// tw_6059_11608
// tw_6059_7153
// tw_6059_4336
// tw_6059_14852
// tw_6059_20964
// tw_6059_27787
// tw_6059_30482
// tw_6059_27622
// tw_6059_17074
// tw_6059_17481
// tw_6059_16907
// tw_6059_28585
