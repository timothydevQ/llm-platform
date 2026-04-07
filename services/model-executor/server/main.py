"""
model-executor/server/main.py

gRPC executor service that runs model inference. The backend is pluggable
(mock for tests, transformers for local dev, vLLM for production).

Wire protocol: execution.v1.ExecutorService (JSON over gRPC via custom codec)

Run:
    python -m server.main
"""
from __future__ import annotations

import json
import logging
import os
import signal
import sys
import threading
import time
from concurrent import futures
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Iterator

import grpc
from grpc import ServicerContext

from backends.mock import MockBackend

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
log = logging.getLogger("executor")

# ── Task type constants (mirrors execution.v1 / inference.v1) ──────────────────
TASK_UNSPECIFIED = 0
TASK_CHAT        = 1
TASK_SUMMARIZE   = 2
TASK_EMBED       = 3
TASK_RERANK      = 4
TASK_CLASSIFY    = 5
TASK_MODERATE    = 6

EXECUTOR_ID = os.environ.get("EXECUTOR_ID", "executor-0")


# ── Response dataclasses ───────────────────────────────────────────────────────

class ExecuteResponse:
    __slots__ = ("request_id","model_id","content","embedding","scores",
                 "tokens_input","tokens_output","latency_ms")
    def __init__(self, **kw):
        for k, v in kw.items():
            setattr(self, k, v)
    def to_dict(self):
        return {s: getattr(self, s, None) for s in self.__slots__}


class StreamChunk:
    __slots__ = ("request_id","token","done","tokens_out")
    def __init__(self, **kw):
        for k, v in kw.items():
            setattr(self, k, v)
    def to_dict(self):
        return {s: getattr(self, s, None) for s in self.__slots__}


class HealthResponse:
    __slots__ = ("executor_id","status","model_ids","load_factor",
                 "requests_served","tokens_per_second","avg_latency_ms")
    def __init__(self, **kw):
        for k, v in kw.items():
            setattr(self, k, v)
    def to_dict(self):
        return {s: getattr(self, s, None) for s in self.__slots__}


# ── Metrics ────────────────────────────────────────────────────────────────────

class Metrics:
    def __init__(self):
        self._lock           = threading.Lock()
        self.requests        = 0
        self.tokens_in       = 0
        self.tokens_out      = 0
        self.errors          = 0
        self.total_latency_s = 0.0

    def record(self, tokens_in: int, tokens_out: int, latency_s: float):
        with self._lock:
            self.requests        += 1
            self.tokens_in       += tokens_in
            self.tokens_out      += tokens_out
            self.total_latency_s += latency_s

    def record_error(self):
        with self._lock:
            self.errors += 1

    @property
    def avg_latency_ms(self) -> float:
        with self._lock:
            if self.requests == 0:
                return 0.0
            return (self.total_latency_s / self.requests) * 1000

    @property
    def tokens_per_second(self) -> float:
        with self._lock:
            if self.total_latency_s == 0:
                return 0.0
            return self.tokens_out / self.total_latency_s


# ── Executor servicer ─────────────────────────────────────────────────────────

class ExecutorServicer:
    """
    gRPC servicer for execution.v1.ExecutorService.

    The JSON-over-gRPC codec encodes each request/response as a plain JSON
    dict, so we receive Python dicts (decoded by grpcio from the JSON bytes)
    rather than proto message objects.
    """

    def __init__(self, backend=None):
        self._backend  = backend or MockBackend()
        self._metrics  = Metrics()
        self._status   = "healthy"
        self._load     = 0.0
        self._lock     = threading.RLock()

    # ── Unary ─────────────────────────────────────────────────────────────────

    def Execute(self, request, context: ServicerContext):
        """Execute a single inference request."""
        req = _as_dict(request)
        if not _check_alive(self._status, context):
            return {}

        start = time.time()
        try:
            result = self._backend.run(
                model_id   = req.get("model_id", ""),
                task_type  = int(req.get("task_type", 0)),
                prompt     = req.get("prompt", ""),
                messages   = req.get("messages", []),
                documents  = req.get("documents", []),
                query      = req.get("query", ""),
                max_tokens = int(req.get("max_tokens", 1024)),
                request_id = req.get("request_id", ""),
            )
        except Exception as exc:
            self._metrics.record_error()
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(exc))
            log.error("Execute failed request_id=%s err=%s", req.get("request_id"), exc)
            return {}

        latency = time.time() - start
        self._metrics.record(
            result.get("tokens_input", 0),
            result.get("tokens_output", 0),
            latency,
        )
        result["latency_ms"] = latency * 1000
        log.info("Execute request_id=%s model=%s task=%s tokens_out=%d latency_ms=%.1f",
                 req.get("request_id"), req.get("model_id"),
                 req.get("task_type"), result.get("tokens_output", 0), latency * 1000)
        return result

    # ── Streaming ─────────────────────────────────────────────────────────────

    def ExecuteStream(self, request, context: ServicerContext):
        """Stream token chunks back to the caller."""
        req = _as_dict(request)
        if not _check_alive(self._status, context):
            return

        start      = time.time()
        request_id = req.get("request_id", "")

        try:
            gen = self._backend.stream(
                model_id   = req.get("model_id", ""),
                task_type  = int(req.get("task_type", 0)),
                prompt     = req.get("prompt", ""),
                messages   = req.get("messages", []),
                max_tokens = int(req.get("max_tokens", 1024)),
                request_id = request_id,
            )
            tokens_out = 0
            for chunk in gen:
                if context.is_active() is False:
                    log.info("stream cancelled request_id=%s", request_id)
                    return
                tokens_out += 1
                yield {
                    "request_id": request_id,
                    "token":      chunk,
                    "done":       False,
                    "tokens_out": tokens_out,
                }
                time.sleep(0.02)  # ~50 tokens/sec

            # Final sentinel
            yield {
                "request_id": request_id,
                "token":      "",
                "done":       True,
                "tokens_out": tokens_out,
            }
            self._metrics.record(0, tokens_out, time.time() - start)

        except Exception as exc:
            self._metrics.record_error()
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(exc))
            log.error("ExecuteStream failed request_id=%s err=%s", request_id, exc)

    # ── Health ────────────────────────────────────────────────────────────────

    def Health(self, request, context: ServicerContext):
        with self._lock:
            status = self._status
        return {
            "executor_id":       EXECUTOR_ID,
            "status":            status,
            "model_ids":         self._backend.model_ids(),
            "load_factor":       self._load,
            "requests_served":   self._metrics.requests,
            "tokens_per_second": round(self._metrics.tokens_per_second, 2),
            "avg_latency_ms":    round(self._metrics.avg_latency_ms, 2),
        }

    def SetStatus(self, request, context: ServicerContext):
        req = _as_dict(request)
        status     = req.get("status", "healthy")
        load_factor = float(req.get("load_factor", 0.0))
        with self._lock:
            self._status = status
            self._load   = load_factor
        log.info("Status changed status=%s load=%f", status, load_factor)
        return {"status": status}


# ── JSON codec helper ─────────────────────────────────────────────────────────

def _as_dict(obj) -> dict:
    """Convert a grpcio message to dict (it already is a dict with JSON codec)."""
    if isinstance(obj, dict):
        return obj
    # Fallback: try __dict__ for protobuf messages in production
    return getattr(obj, "DESCRIPTOR", None) and {} or (obj if isinstance(obj, dict) else {})


def _check_alive(status: str, context: ServicerContext) -> bool:
    if status == "down":
        context.set_code(grpc.StatusCode.UNAVAILABLE)
        context.set_details("executor is down")
        return False
    return True


# ── gRPC service registration ─────────────────────────────────────────────────

def _build_handler(servicer: ExecutorServicer) -> grpc.ServiceRpcHandlers:
    return grpc.method_service_handler(
        service="execution.v1.ExecutorService",
        method_handlers={
            "Execute": grpc.unary_unary_rpc_method_handler(
                servicer.Execute,
                request_deserializer=None,
                response_serializer=None,
            ),
            "ExecuteStream": grpc.unary_stream_rpc_method_handler(
                servicer.ExecuteStream,
                request_deserializer=None,
                response_serializer=None,
            ),
            "Health": grpc.unary_unary_rpc_method_handler(
                servicer.Health,
            ),
            "SetStatus": grpc.unary_unary_rpc_method_handler(
                servicer.SetStatus,
            ),
        },
    )


# ── HTTP health sidecar ───────────────────────────────────────────────────────

def _run_http(servicer: ExecutorServicer, port: int):
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *a): pass  # suppress access logs

        def do_GET(self):
            if self.path == "/healthz/live":
                self._json(200, {"status": "alive"})
            elif self.path == "/healthz/ready":
                with servicer._lock:
                    s = servicer._status
                code = 200 if s != "down" else 503
                self._json(code, {"status": s})
            elif self.path == "/v1/stats":
                self._json(200, {
                    "executor_id":       EXECUTOR_ID,
                    "status":            servicer._status,
                    "requests":          servicer._metrics.requests,
                    "tokens_out":        servicer._metrics.tokens_out,
                    "errors":            servicer._metrics.errors,
                    "avg_latency_ms":    round(servicer._metrics.avg_latency_ms, 2),
                    "tokens_per_second": round(servicer._metrics.tokens_per_second, 2),
                })
            elif self.path == "/metrics":
                body = (
                    f"executor_requests_total {servicer._metrics.requests}\n"
                    f"executor_tokens_out_total {servicer._metrics.tokens_out}\n"
                    f"executor_errors_total {servicer._metrics.errors}\n"
                    f"executor_avg_latency_ms {servicer._metrics.avg_latency_ms:.2f}\n"
                    f"executor_tokens_per_second {servicer._metrics.tokens_per_second:.2f}\n"
                )
                self.send_response(200)
                self.send_header("Content-Type", "text/plain")
                self.end_headers()
                self.wfile.write(body.encode())
            else:
                self.send_response(404); self.end_headers()

        def _json(self, code: int, data: dict):
            body = json.dumps(data).encode()
            self.send_response(code)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(body)

    HTTPServer(("", port), Handler).serve_forever()


# ── Main ──────────────────────────────────────────────────────────────────────

def serve():
    grpc_port   = int(os.environ.get("GRPC_PORT",   "50051"))
    http_port   = int(os.environ.get("HTTP_PORT",   "8085"))
    max_workers = int(os.environ.get("MAX_WORKERS", "10"))

    backend  = MockBackend()
    servicer = ExecutorServicer(backend=backend)
    handler  = _build_handler(servicer)

    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=max_workers),
        options=[
            ("grpc.max_send_message_length",    64 * 1024 * 1024),
            ("grpc.max_receive_message_length", 64 * 1024 * 1024),
        ],
    )
    server.add_generic_rpc_handlers([handler])
    server.add_insecure_port(f"[::]:{grpc_port}")
    server.start()
    log.info("Executor gRPC started port=%d executor_id=%s", grpc_port, EXECUTOR_ID)

    threading.Thread(target=_run_http, args=(servicer, http_port), daemon=True).start()
    log.info("Executor HTTP health started port=%d", http_port)

    def _shutdown(sig, frame):
        log.info("Shutting down executor...")
        server.stop(grace=10)
        sys.exit(0)

    signal.signal(signal.SIGTERM, _shutdown)
    signal.signal(signal.SIGINT, _shutdown)
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
// py_315
// py_316
// py_317
// py_318
// py_319
// py_320
// py_321
// py_322
// py_323
// py_324
// py_325
