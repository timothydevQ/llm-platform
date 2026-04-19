"""
server/main.py — Model executor entrypoint.

Run as: python -m server.main  (from the model-executor/ directory)

The package uses relative imports throughout. Running as a module ensures
Python resolves them correctly. The Dockerfile CMD is:
  CMD ["python", "-m", "server.main"]

Environment variables
─────────────────────
GRPC_PORT        gRPC listen port           (default: 50051)
HTTP_PORT        HTTP health/metrics port   (default: 8085)
MAX_WORKERS      ThreadPoolExecutor size    (default: 10)
EXECUTOR_ID      Pod identity from K8s fieldRef  (default: executor-0)
USE_REAL_MODELS  "true" loads sentence-transformers + transformers weights
                 "false" uses MockBackend — for CI without model files
EMBED_MODEL      HF repo ID  (default: sentence-transformers/all-MiniLM-L6-v2)
RERANK_MODEL     HF repo ID  (default: cross-encoder/ms-marco-MiniLM-L-6-v2)
CHAT_MODEL       HF repo ID  (default: facebook/opt-125m)
HF_HOME          Weight cache dir. Mount a PVC here in Kubernetes to avoid
                 re-downloading on every pod restart.
"""
from __future__ import annotations

import json
import logging
import os
import signal
import sys
import threading
from concurrent import futures
from http.server import BaseHTTPRequestHandler, HTTPServer

import grpc

from ..protos.execution_pb2_grpc import add_ExecutorServiceServicer_to_server
from ..backends.router_backend import RouterBackend
from ..backends.mock import MockBackend
from .executor import ExecutorServicer, STATUS_DOWN, EXECUTOR_ID

logging.basicConfig(
    level=os.environ.get("LOG_LEVEL", "INFO"),
    format="%(asctime)s %(levelname)-8s %(name)-30s %(message)s",
)
log = logging.getLogger("executor.main")

# ── gRPC server options ────────────────────────────────────────────────────────
_GRPC_OPTIONS = [
    ("grpc.max_send_message_length",         64 * 1024 * 1024),   # 64 MB
    ("grpc.max_receive_message_length",       64 * 1024 * 1024),
    # Keepalive prevents idle connections being torn down by cloud LBs
    ("grpc.keepalive_time_ms",               30_000),
    ("grpc.keepalive_timeout_ms",            10_000),
    ("grpc.keepalive_permit_without_calls",  True),
    ("grpc.http2.max_pings_without_data",    0),
]


# ── HTTP health + metrics sidecar ─────────────────────────────────────────────

class _HealthHandler(BaseHTTPRequestHandler):
    """Minimal HTTP handler for K8s probes and Prometheus scraping."""

    servicer: ExecutorServicer = None  # set at startup

    def log_message(self, *a):  # suppress per-request access logs
        pass

    def do_GET(self):
        if self.path == "/healthz/live":
            self._j(200, {
                "status":      "alive",
                "executor_id": EXECUTOR_ID,
            })

        elif self.path == "/healthz/ready":
            with self.servicer._lock:
                status = self.servicer._status
            code = 200 if status != STATUS_DOWN else 503
            self._j(code, {"status": status, "executor_id": EXECUTOR_ID})

        elif self.path == "/v1/stats":
            m = self.servicer._metrics
            with self.servicer._lock:
                status = self.servicer._status
            self._j(200, {
                "executor_id":        EXECUTOR_ID,
                "status":             status,
                "model_ids":          self.servicer._backend.model_ids(),
                "requests_served":    m.requests,
                "errors":             m.errors,
                "tokens_in":          m.tokens_in,
                "tokens_out":         m.tokens_out,
                "avg_latency_ms":     round(m.avg_latency_ms, 2),
                "tokens_per_second":  round(m.tokens_per_second, 2),
            })

        elif self.path == "/metrics":
            body = self.servicer._metrics.prometheus_output()
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
            self.end_headers()
            self.wfile.write(body)

        else:
            self.send_response(404)
            self.end_headers()

    def _j(self, code: int, data: dict) -> None:
        body = json.dumps(data).encode()
        self.send_response(code)
        self.send_header("Content-Type",   "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


# ── Main ───────────────────────────────────────────────────────────────────────

def serve() -> None:
    grpc_port   = int(os.environ.get("GRPC_PORT",      "50051"))
    http_port   = int(os.environ.get("HTTP_PORT",      "8085"))
    max_workers = int(os.environ.get("MAX_WORKERS",    "10"))
    use_real    = os.environ.get("USE_REAL_MODELS", "true").lower() == "true"

    log.info(
        "Executor starting  id=%s  grpc=%d  http=%d  workers=%d  real_models=%s",
        EXECUTOR_ID, grpc_port, http_port, max_workers, use_real,
    )

    # ── Load backends ─────────────────────────────────────────────────────────
    if use_real:
        backend = RouterBackend(use_real_models=True)
    else:
        log.warning("USE_REAL_MODELS=false — using MockBackend (no model weights loaded)")
        backend = MockBackend()

    log.info("Loading model weights — this may take 20-60s on first run …")
    backend.load()
    log.info("Backends ready  model_ids=%s", backend.model_ids())

    # ── gRPC server ───────────────────────────────────────────────────────────
    servicer = ExecutorServicer(backend)
    pool     = futures.ThreadPoolExecutor(max_workers=max_workers)
    srv      = grpc.server(pool, options=_GRPC_OPTIONS)
    add_ExecutorServiceServicer_to_server(servicer, srv)
    srv.add_insecure_port(f"[::]:{grpc_port}")
    srv.start()
    log.info("gRPC server listening  port=%d", grpc_port)

    # ── HTTP health sidecar ───────────────────────────────────────────────────
    _HealthHandler.servicer = servicer
    http_thread = threading.Thread(
        target=lambda: HTTPServer(("", http_port), _HealthHandler).serve_forever(),
        daemon=True,
        name="http-health",
    )
    http_thread.start()
    log.info("HTTP health server listening  port=%d", http_port)

    # ── Graceful shutdown ─────────────────────────────────────────────────────
    def _shutdown(sig, _frame):
        log.info("Signal %s received — draining in-flight RPCs (10s grace) …", sig)
        srv.stop(grace=10)
        log.info("Shutdown complete")
        sys.exit(0)

    signal.signal(signal.SIGTERM, _shutdown)
    signal.signal(signal.SIGINT,  _shutdown)

    log.info("Executor ready  executor_id=%s  models=%s", EXECUTOR_ID, backend.model_ids())
    srv.wait_for_termination()


if __name__ == "__main__":
    serve()
