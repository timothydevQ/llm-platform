"""
server/executor.py

ExecutorServicer — the gRPC service implementation for execution.v1.ExecutorService.

Key production properties:
  - Uses proper protobuf stubs (protos/execution_pb2_grpc.py)
  - Tracks Prometheus metrics per model/task
  - Propagates gRPC deadlines (deadline_ms field in ExecuteRequest)
  - Supports server-streaming with per-token context cancellation check
  - Thread-safe status + load_factor control for chaos testing
  - Structured logging on every call with request_id and trace context
"""
from __future__ import annotations

import json
import logging
import os
import threading
import time
from typing import Iterator

import grpc

from ..protos import execution_pb2 as pb2
from ..protos.execution_pb2_grpc import ExecutorServiceServicer
from ..backends.base import Backend
from .metrics import ExecutorMetrics

log = logging.getLogger("executor.server")

EXECUTOR_ID = os.environ.get("EXECUTOR_ID", "executor-0")

# Status constants
STATUS_HEALTHY  = "healthy"
STATUS_DEGRADED = "degraded"   # still serves, with elevated latency_jitter
STATUS_DOWN     = "down"       # rejects all requests


class ExecutorServicer(ExecutorServiceServicer):
    """
    Production gRPC servicer for execution.v1.ExecutorService.

    Usage:
        backend  = RouterBackend()
        backend.load()
        servicer = ExecutorServicer(backend)
        add_ExecutorServiceServicer_to_server(servicer, grpc_server)
    """

    def __init__(self, backend: Backend):
        self._backend = backend
        self._metrics = ExecutorMetrics()
        self._status  = STATUS_HEALTHY
        self._jitter  = 1.0   # latency multiplier; >1 = degraded
        self._lock    = threading.RLock()

    # ── Unary inference ───────────────────────────────────────────────────────

    def Execute(self, request: pb2.ExecuteRequest, context: grpc.ServicerContext) -> pb2.ExecuteResponse:
        """Single inference call. Respects gRPC deadline and request.deadline_ms."""
        with self._lock:
            status = self._status
            jitter = self._jitter

        if status == STATUS_DOWN:
            context.set_code(grpc.StatusCode.UNAVAILABLE)
            context.set_details(f"executor {EXECUTOR_ID} is down")
            return pb2.ExecuteResponse()

        # Check deadline (both gRPC deadline and request-level deadline_ms)
        if not self._deadline_ok(request, context):
            return pb2.ExecuteResponse()

        model_id  = request.model_id
        task_type = request.task_type
        task_name = pb2.TaskType.name(task_type)
        start     = time.time()

        with self._metrics.track_active():
            try:
                result = self._backend.execute(
                    request_id = request.request_id,
                    model_id   = model_id,
                    task_type  = task_type,
                    prompt     = request.prompt,
                    messages   = list(request.messages),
                    documents  = list(request.documents),
                    query      = request.query,
                    max_tokens = request.max_tokens or 512,
                )
            except ValueError as exc:
                # Bad request — wrong task for this backend
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details(str(exc))
                self._metrics.record(
                    model_id=model_id, task=task_name,
                    tokens_in=0, tokens_out=0,
                    latency_s=time.time() - start, success=False,
                )
                return pb2.ExecuteResponse()
            except Exception as exc:
                context.set_code(grpc.StatusCode.INTERNAL)
                context.set_details(f"inference failed: {exc}")
                log.error(
                    "Execute failed request_id=%s model=%s task=%s err=%s",
                    request.request_id, model_id, task_name, exc, exc_info=True,
                )
                self._metrics.record(
                    model_id=model_id, task=task_name,
                    tokens_in=0, tokens_out=0,
                    latency_s=time.time() - start, success=False,
                )
                return pb2.ExecuteResponse()

        latency_s = time.time() - start
        self._metrics.record(
            model_id   = model_id,
            task       = task_name,
            tokens_in  = result.get("tokens_input", 0),
            tokens_out = result.get("tokens_output", 0),
            latency_s  = latency_s,
            success    = True,
        )

        log.info(
            "Execute ok request_id=%s model=%s task=%s "
            "tokens_in=%d tokens_out=%d latency_ms=%.1f",
            request.request_id, model_id, task_name,
            result.get("tokens_input", 0), result.get("tokens_output", 0),
            latency_s * 1000,
        )

        return pb2.ExecuteResponse(
            request_id    = request.request_id,
            model_id      = model_id,
            content       = result.get("content", ""),
            embedding     = result.get("embedding", []),
            scores        = result.get("scores", []),
            tokens_input  = result.get("tokens_input", 0),
            tokens_output = result.get("tokens_output", 0),
            latency_ms    = latency_s * 1000,
        )

    # ── Streaming inference ───────────────────────────────────────────────────

    def ExecuteStream(
        self,
        request:  pb2.ExecuteRequest,
        context:  grpc.ServicerContext,
    ) -> Iterator[pb2.StreamChunk]:
        """
        Server-streaming inference — yields one StreamChunk per token.

        Cancellation: checks context.is_active() before each yield so that
        client disconnects abort generation immediately, freeing compute.
        """
        with self._lock:
            if self._status == STATUS_DOWN:
                context.set_code(grpc.StatusCode.UNAVAILABLE)
                context.set_details(f"executor {EXECUTOR_ID} is down")
                return

        request_id = request.request_id
        model_id   = request.model_id
        task_type  = request.task_type
        start      = time.time()
        tokens_out = 0

        try:
            gen = self._backend.stream(
                request_id = request_id,
                model_id   = model_id,
                task_type  = task_type,
                prompt     = request.prompt,
                messages   = list(request.messages),
                max_tokens = request.max_tokens or 512,
            )
            for token_str in gen:
                if not context.is_active():
                    log.info("stream cancelled request_id=%s tokens_out=%d", request_id, tokens_out)
                    return

                tokens_out += 1
                yield pb2.StreamChunk(
                    request_id = request_id,
                    token      = token_str,
                    done       = False,
                    tokens_out = tokens_out,
                )

            # Final sentinel — lets client know generation is complete
            yield pb2.StreamChunk(
                request_id = request_id,
                token      = "",
                done       = True,
                tokens_out = tokens_out,
            )

        except Exception as exc:
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(exc))
            log.error("ExecuteStream failed request_id=%s err=%s", request_id, exc, exc_info=True)
            return

        latency_s = time.time() - start
        self._metrics.record(
            model_id=model_id, task=pb2.TaskType.name(task_type),
            tokens_in=0, tokens_out=tokens_out,
            latency_s=latency_s, success=True,
        )

    # ── Health and control ────────────────────────────────────────────────────

    def Health(self, request: pb2.HealthRequest, context: grpc.ServicerContext) -> pb2.HealthResponse:
        with self._lock:
            status = self._status
        return pb2.HealthResponse(
            executor_id       = EXECUTOR_ID,
            status            = status,
            model_ids         = self._backend.model_ids(),
            load_factor       = self._jitter - 1.0,  # 0=normal, >0=degraded
            requests_served   = self._metrics.requests,
            tokens_per_second = round(self._metrics.tokens_per_second, 2),
            avg_latency_ms    = round(self._metrics.avg_latency_ms, 2),
        )

    def SetStatus(self, request: pb2.SetStatusRequest, context: grpc.ServicerContext) -> pb2.SetStatusResponse:
        """Chaos testing endpoint. Sets status and optional latency multiplier."""
        status = request.status or STATUS_HEALTHY
        jitter = max(0.01, float(request.load_factor) + 1.0) if request.load_factor > 0 else 1.0

        with self._lock:
            self._status = status
            self._jitter = jitter

        log.info("Status changed status=%s jitter=%.2f executor=%s", status, jitter, EXECUTOR_ID)
        return pb2.SetStatusResponse(status=status)

    # ── Internal ──────────────────────────────────────────────────────────────

    def _deadline_ok(self, request: pb2.ExecuteRequest, context: grpc.ServicerContext) -> bool:
        """
        Check both the gRPC deadline (propagated by Go services) and the
        explicit deadline_ms field for belt-and-suspenders safety.
        """
        if not context.is_active():
            context.set_code(grpc.StatusCode.DEADLINE_EXCEEDED)
            context.set_details("client cancelled before execution")
            return False

        if request.deadline_ms > 0:
            remaining_ms = request.deadline_ms - int(time.time() * 1000)
            if remaining_ms <= 0:
                context.set_code(grpc.StatusCode.DEADLINE_EXCEEDED)
                context.set_details(f"request deadline already passed ({-remaining_ms}ms ago)")
                return False

        return True
// tw_6059_9587
