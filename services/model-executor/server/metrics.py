"""
server/metrics.py

Prometheus metrics for the model executor. Exposes:
  - executor_requests_total (counter, by model/task/status)
  - executor_inference_duration_seconds (histogram, by model/task)
  - executor_tokens_input_total (counter, by model)
  - executor_tokens_output_total (counter, by model)
  - executor_active_requests (gauge)
  - executor_model_load_status (gauge, by model: 1=loaded, 0=not loaded)
"""
from __future__ import annotations

import threading
import time
from contextlib import contextmanager
from typing import Generator

try:
    from prometheus_client import (
        Counter, Histogram, Gauge, generate_latest, CONTENT_TYPE_LATEST,
        CollectorRegistry, REGISTRY,
    )
    _PROMETHEUS = True
except ImportError:
    _PROMETHEUS = False


class ExecutorMetrics:
    """
    Thread-safe Prometheus metrics wrapper.

    Falls back to simple in-memory counters when prometheus_client is not
    installed (e.g., in unit tests without the full dependency set).
    """

    def __init__(self, registry=None):
        self._lock           = threading.Lock()
        self._requests       = 0
        self._errors         = 0
        self._tokens_in      = 0
        self._tokens_out     = 0
        self._total_latency  = 0.0
        self._active         = 0

        if _PROMETHEUS:
            r = registry or REGISTRY
            self._prom_requests = Counter(
                "executor_requests_total",
                "Total inference requests",
                ["model_id", "task", "status"],
                registry=r,
            )
            self._prom_duration = Histogram(
                "executor_inference_duration_seconds",
                "Inference duration in seconds",
                ["model_id", "task"],
                buckets=[.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10],
                registry=r,
            )
            self._prom_tok_in  = Counter(
                "executor_tokens_input_total",
                "Total input tokens",
                ["model_id"],
                registry=r,
            )
            self._prom_tok_out = Counter(
                "executor_tokens_output_total",
                "Total output tokens",
                ["model_id"],
                registry=r,
            )
            self._prom_active  = Gauge(
                "executor_active_requests",
                "Currently in-flight requests",
                registry=r,
            )
        else:
            self._prom_requests = None
            self._prom_duration = None
            self._prom_tok_in   = None
            self._prom_tok_out  = None
            self._prom_active   = None

    def record(
        self, *,
        model_id:    str,
        task:        str,
        tokens_in:   int,
        tokens_out:  int,
        latency_s:   float,
        success:     bool,
    ) -> None:
        status = "ok" if success else "error"
        with self._lock:
            self._requests      += 1
            self._tokens_in     += tokens_in
            self._tokens_out    += tokens_out
            self._total_latency += latency_s
            if not success:
                self._errors += 1

        if _PROMETHEUS and self._prom_requests:
            self._prom_requests.labels(model_id=model_id, task=task, status=status).inc()
            self._prom_duration.labels(model_id=model_id, task=task).observe(latency_s)
            self._prom_tok_in.labels(model_id=model_id).inc(tokens_in)
            self._prom_tok_out.labels(model_id=model_id).inc(tokens_out)

    @contextmanager
    def track_active(self) -> Generator[None, None, None]:
        with self._lock:
            self._active += 1
        if _PROMETHEUS and self._prom_active:
            self._prom_active.inc()
        try:
            yield
        finally:
            with self._lock:
                self._active -= 1
            if _PROMETHEUS and self._prom_active:
                self._prom_active.dec()

    # ── Accessors ─────────────────────────────────────────────────────────────

    @property
    def requests(self) -> int:
        return self._requests

    @property
    def errors(self) -> int:
        return self._errors

    @property
    def tokens_in(self) -> int:
        return self._tokens_in

    @property
    def tokens_out(self) -> int:
        return self._tokens_out

    @property
    def avg_latency_ms(self) -> float:
        with self._lock:
            if self._requests == 0:
                return 0.0
            return (self._total_latency / self._requests) * 1000.0

    @property
    def tokens_per_second(self) -> float:
        with self._lock:
            if self._total_latency == 0:
                return 0.0
            return self._tokens_out / self._total_latency

    def prometheus_output(self) -> bytes:
        if _PROMETHEUS:
            return generate_latest()
        # Fallback: plain text
        lines = [
            f"executor_requests_total {self._requests}",
            f"executor_errors_total {self._errors}",
            f"executor_tokens_input_total {self._tokens_in}",
            f"executor_tokens_output_total {self._tokens_out}",
            f"executor_avg_latency_ms {self.avg_latency_ms:.2f}",
            f"executor_tokens_per_second {self.tokens_per_second:.2f}",
        ]
        return "\n".join(lines).encode()
// tw_6059_21076
// tw_6059_2279
// tw_6059_8687
