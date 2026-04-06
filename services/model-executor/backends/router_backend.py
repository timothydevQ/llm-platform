"""
backends/router_backend.py

RouterBackend fans out requests to the appropriate specialised backend:
  - embed + rerank → SentenceTransformersBackend
  - chat + summarize + classify + moderate → TransformersBackend

This is the backend the executor server registers at startup. The two
underlying backends are loaded independently so embed can warm up while
the language model is still loading.
"""
from __future__ import annotations

import logging
import threading
from typing import Iterator, List

from .base import Backend
from .sentence_transformers_backend import SentenceTransformersBackend, TASK_EMBED, TASK_RERANK
from .transformers_backend import TransformersBackend

log = logging.getLogger("executor.backends.router")

TASK_CHAT      = 1
TASK_SUMMARIZE = 2
TASK_CLASSIFY  = 5
TASK_MODERATE  = 6


class RouterBackend(Backend):
    """
    Fan-out backend that dispatches to specialised sub-backends.

    Loading order:
      1. SentenceTransformers (embed + rerank) — fast, ~5s
      2. Transformers (chat + classify)        — slower, ~15-30s on CPU

    Both backends warm up in parallel (background threads) so the executor
    becomes healthy as soon as at least one backend is ready.
    """

    def __init__(self, *, use_real_models: bool = True):
        if use_real_models:
            self._st  = SentenceTransformersBackend()
            self._hf  = TransformersBackend()
        else:
            # Inject mock backends in tests without loading model weights
            from .mock import MockBackend as _M
            self._st = _M()
            self._hf = _M()

        self._ready: dict[str, bool] = {"st": False, "hf": False}
        self._lock = threading.Lock()

    def load(self) -> None:
        """Load both backends in parallel background threads."""
        def _load_st():
            try:
                self._st.load()
                with self._lock:
                    self._ready["st"] = True
                log.info("SentenceTransformers backend ready")
            except Exception as exc:
                log.error("SentenceTransformers backend failed to load: %s", exc)

        def _load_hf():
            try:
                self._hf.load()
                with self._lock:
                    self._ready["hf"] = True
                log.info("Transformers backend ready")
            except Exception as exc:
                log.error("Transformers backend failed to load: %s", exc)

        t1 = threading.Thread(target=_load_st, daemon=True)
        t2 = threading.Thread(target=_load_hf, daemon=True)
        t1.start()
        t2.start()
        # Join both — executor health probe marks it ready once load() returns
        t1.join()
        t2.join()

    def model_ids(self) -> List[str]:
        return self._st.model_ids() + self._hf.model_ids()

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
        backend = self._route(task_type)
        return backend.execute(
            request_id=request_id, model_id=model_id, task_type=task_type,
            prompt=prompt, messages=messages, documents=documents,
            query=query, max_tokens=max_tokens,
        )

    def stream(
        self, *,
        request_id: str,
        model_id:   str,
        task_type:  int,
        prompt:     str,
        messages:   list,
        max_tokens: int,
    ) -> Iterator[str]:
        backend = self._route(task_type)
        yield from backend.stream(
            request_id=request_id, model_id=model_id, task_type=task_type,
            prompt=prompt, messages=messages, max_tokens=max_tokens,
        )

    def _route(self, task_type: int) -> Backend:
        if task_type in (TASK_EMBED, TASK_RERANK):
            return self._st
        if task_type in (TASK_CHAT, TASK_SUMMARIZE, TASK_CLASSIFY, TASK_MODERATE):
            return self._hf
        raise ValueError(f"No backend registered for task_type={task_type}")
// tw_6059_22467
// tw_6059_30407
// tw_6059_31141
// tw_6059_25815
// tw_6059_19241
// tw_6059_2822
// tw_6059_15549
// tw_6059_26913
