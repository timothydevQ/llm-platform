# ADR-003: Python Executor with Pluggable Backends

**Status**: Accepted  
**Date**: 2026-03-27

---

## Decision

The model executor is written in Python (not Go) because:
1. The Python ML ecosystem (transformers, vLLM, TGI) has no comparable Go equivalent
2. The Go services handle the control plane, routing, and batching — Python handles the compute plane
3. This mirrors how real organisations structure their inference infrastructure

The backend is pluggable via a `MockBackend` (deterministic, no model required) / `TransformersBackend` / `vLLMBackend` interface.

## Consequences

This forces real engineering discipline: the Go and Python services must agree on a strict gRPC contract. The streaming cancellation path (`context.is_active()` check in Python) demonstrates real production awareness.
