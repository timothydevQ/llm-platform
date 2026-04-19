"""
backends — model inference backend package.

Public API:

    from backends import MockBackend, RouterBackend, get_backend

    # In tests (no model weights):
    backend = get_backend(use_real_models=False)

    # In production (loads sentence-transformers + transformers):
    backend = get_backend(use_real_models=True)
    backend.load()
"""
from .base import Backend
from .mock import MockBackend
from .router_backend import RouterBackend


def get_backend(*, use_real_models: bool = True) -> Backend:
    """
    Factory function returning the appropriate backend.

    Args:
        use_real_models: When True (default) returns RouterBackend backed by
            SentenceTransformersBackend + TransformersBackend.  When False
            returns MockBackend — suitable for unit tests and CI runs that
            cannot download model weights.

    The returned backend has NOT been loaded yet. Call backend.load() before
    passing it to ExecutorServicer.
    """
    if use_real_models:
        return RouterBackend(use_real_models=True)
    return MockBackend()


__all__ = ["Backend", "MockBackend", "RouterBackend", "get_backend"]
