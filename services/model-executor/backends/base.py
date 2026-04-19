"""
backends/base.py — Abstract base class for model backends.

Concrete implementations:
  - MockBackend          (tests, no model files required)
  - SentenceTransformersBackend  (real embeddings via sentence-transformers)
  - CrossEncoderBackend  (real reranking via cross-encoder)
  - TransformersBackend  (real chat/classify/summarize via HuggingFace transformers)

All backends are thread-safe (models are loaded once at __init__ and inference
is stateless). The executor uses them from a ThreadPoolExecutor.
"""
from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Iterator, List


class Backend(ABC):
    """Abstract inference backend."""

    @abstractmethod
    def load(self) -> None:
        """Load model weights. Called once at startup."""

    @abstractmethod
    def model_ids(self) -> List[str]:
        """Return the list of model IDs this backend serves."""

    @abstractmethod
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
        """Run inference and return a result dict matching ExecuteResponse."""

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
        Yield individual string tokens for streaming inference.

        Default implementation: execute() then split the content on words.
        Override in streaming-capable backends (e.g. vLLM).
        """
        result  = self.execute(
            request_id=request_id, model_id=model_id, task_type=task_type,
            prompt=prompt, messages=messages, documents=[], query="",
            max_tokens=max_tokens,
        )
        content = result.get("content", "")
        for word in content.split():
            yield word + " "
