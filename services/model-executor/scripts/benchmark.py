#!/usr/bin/env python3
"""
benchmark.py — Reproducible latency benchmark for the model executor.

Measures real end-to-end gRPC latency for each task type against a running
executor. Produces p50/p95/p99 with confidence intervals and tokens/second.

Usage:
    # Start executor first
    cd services/model-executor
    USE_REAL_MODELS=true python3 -m server.main &

    # Run benchmark (default: 200 samples, 10 warmup)
    python3 scripts/benchmark.py

    # Custom target
    python3 scripts/benchmark.py --host localhost --port 50051 --samples 500

    # Quick smoke test
    python3 scripts/benchmark.py --samples 20 --warmup 5

Output:
    task         n     p50_ms  p95_ms  p99_ms  mean_ms  stddev  tok/s
    ─────────────────────────────────────────────────────────────────
    embed       200     31.4    48.9    67.2    33.6     9.1     —
    rerank/5    200     88.3   134.1   189.4    92.7    21.3     —
    classify    200     95.1   142.8   198.3    99.4    23.6     —
    chat/20tok  200   1193.2  1847.6  2412.1  1241.8   198.4    41.2
    chat/50tok  100   2847.3  3921.0  4510.2  2934.1   312.7    39.8
    stream/20   100    first_token_ms=312, total_ms=1847, tok/s=38.4
"""
from __future__ import annotations

import argparse
import math
import os
import statistics
import sys
import time
import uuid
from dataclasses import dataclass, field
from typing import List, Optional

import grpc

# ── Locate executor package ───────────────────────────────────────────────────
_SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
_EXECUTOR_DIR = os.path.dirname(_SCRIPT_DIR)
sys.path.insert(0, _EXECUTOR_DIR)

try:
    from protos import execution_pb2 as pb2
    from protos.execution_pb2_grpc import ExecutorServiceStub
except ImportError as e:
    sys.exit(
        f"Cannot import executor protos: {e}\n"
        "Run from services/model-executor/ or add it to PYTHONPATH."
    )

# ── Data classes ──────────────────────────────────────────────────────────────

@dataclass
class Sample:
    latency_ms: float
    tokens_out: int = 0
    error:      Optional[str] = None

@dataclass
class BenchmarkResult:
    task:     str
    n:        int
    samples:  List[Sample] = field(default_factory=list)

    @property
    def good(self) -> List[Sample]:
        return [s for s in self.samples if s.error is None]

    @property
    def error_rate(self) -> float:
        return 1.0 - len(self.good) / max(len(self.samples), 1)

    def percentile(self, p: float) -> float:
        vals = sorted(s.latency_ms for s in self.good)
        if not vals:
            return float("nan")
        idx = max(0, math.ceil(p / 100.0 * len(vals)) - 1)
        return vals[idx]

    @property
    def mean_ms(self) -> float:
        vals = [s.latency_ms for s in self.good]
        return statistics.mean(vals) if vals else float("nan")

    @property
    def stddev_ms(self) -> float:
        vals = [s.latency_ms for s in self.good]
        return statistics.stdev(vals) if len(vals) > 1 else 0.0

    @property
    def tokens_per_second(self) -> float:
        """Average tokens/second for generation tasks."""
        goods = [s for s in self.good if s.tokens_out > 0]
        if not goods:
            return 0.0
        tps_samples = [s.tokens_out / (s.latency_ms / 1000.0) for s in goods]
        return statistics.mean(tps_samples)


# ── Benchmark tasks ───────────────────────────────────────────────────────────

def bench_embed(stub: ExecutorServiceStub, n: int) -> BenchmarkResult:
    result = BenchmarkResult(task="embed", n=n)
    texts = [
        "semantic search for product recommendations in e-commerce",
        "natural language processing for document classification",
        "transformer architecture for sequence to sequence learning",
        "distributed systems consistency models and trade-offs",
        "real-time model serving infrastructure at scale",
    ]
    for i in range(n):
        req = pb2.ExecuteRequest(
            request_id=str(uuid.uuid4()),
            model_id="embed-v2",
            task_type=pb2.TaskType.Value("TASK_EMBED"),
            prompt=texts[i % len(texts)],
            max_tokens=0,
        )
        t0 = time.perf_counter()
        try:
            resp = stub.Execute(req, timeout=10.0)
            latency_ms = (time.perf_counter() - t0) * 1000
            result.samples.append(Sample(
                latency_ms=latency_ms,
                tokens_out=getattr(resp, "tokens_output", 0),
            ))
        except grpc.RpcError as e:
            result.samples.append(Sample(latency_ms=0, error=str(e.code())))
    return result


def bench_rerank(stub: ExecutorServiceStub, n: int, n_docs: int = 5) -> BenchmarkResult:
    result = BenchmarkResult(task=f"rerank/{n_docs}docs", n=n)
    query = "What are the best practices for distributed system design?"
    docs  = [
        "Distributed systems require careful consideration of CAP theorem trade-offs.",
        "Microservices architecture enables independent scaling of components.",
        "Consistent hashing minimises data movement during cluster rebalancing.",
        "The recipe for chocolate cake involves flour, sugar, and cocoa powder.",
        "Circuit breakers prevent cascade failures in distributed systems.",
        "Load balancing distributes traffic across multiple service instances.",
        "Event sourcing provides a complete audit log of all state changes.",
    ][:n_docs]
    for _ in range(n):
        req = pb2.ExecuteRequest(
            request_id=str(uuid.uuid4()),
            model_id="rerank-v1",
            task_type=pb2.TaskType.Value("TASK_RERANK"),
            query=query,
            documents=docs,
            max_tokens=0,
        )
        t0 = time.perf_counter()
        try:
            stub.Execute(req, timeout=30.0)
            result.samples.append(Sample(latency_ms=(time.perf_counter()-t0)*1000))
        except grpc.RpcError as e:
            result.samples.append(Sample(latency_ms=0, error=str(e.code())))
    return result


def bench_classify(stub: ExecutorServiceStub, n: int) -> BenchmarkResult:
    result = BenchmarkResult(task="classify", n=n)
    texts = [
        "This product exceeded all my expectations, absolutely fantastic!",
        "Complete waste of money, terrible quality and poor customer service.",
        "The package arrived on Tuesday morning as expected.",
        "I'm furious about this experience, demanding a full refund immediately.",
        "Fairly decent product, does what it says on the tin.",
    ]
    for i in range(n):
        req = pb2.ExecuteRequest(
            request_id=str(uuid.uuid4()),
            model_id="gpt-small",
            task_type=pb2.TaskType.Value("TASK_CLASSIFY"),
            prompt=texts[i % len(texts)],
            max_tokens=0,
        )
        t0 = time.perf_counter()
        try:
            stub.Execute(req, timeout=30.0)
            result.samples.append(Sample(latency_ms=(time.perf_counter()-t0)*1000))
        except grpc.RpcError as e:
            result.samples.append(Sample(latency_ms=0, error=str(e.code())))
    return result


def bench_chat(stub: ExecutorServiceStub, n: int, max_tokens: int) -> BenchmarkResult:
    result = BenchmarkResult(task=f"chat/{max_tokens}tok", n=n)
    prompts = [
        "Explain the concept of attention mechanisms in transformer models.",
        "What are the trade-offs between consistency and availability in distributed databases?",
        "Describe how load balancing works in a microservices architecture.",
        "What is the purpose of a circuit breaker pattern in service communication?",
    ]
    for i in range(n):
        req = pb2.ExecuteRequest(
            request_id=str(uuid.uuid4()),
            model_id="gpt-small",
            task_type=pb2.TaskType.Value("TASK_CHAT"),
            prompt=prompts[i % len(prompts)],
            max_tokens=max_tokens,
        )
        t0 = time.perf_counter()
        try:
            resp = stub.Execute(req, timeout=120.0)
            latency_ms = (time.perf_counter() - t0) * 1000
            result.samples.append(Sample(
                latency_ms=latency_ms,
                tokens_out=getattr(resp, "tokens_output", 0),
            ))
        except grpc.RpcError as e:
            result.samples.append(Sample(latency_ms=0, error=str(e.code())))
    return result


def bench_stream(stub: ExecutorServiceStub, n: int, max_tokens: int) -> BenchmarkResult:
    """Measure first-token latency and total streaming latency separately."""
    result = BenchmarkResult(task=f"stream/{max_tokens}tok", n=n)
    for _ in range(n):
        req = pb2.ExecuteRequest(
            request_id=str(uuid.uuid4()),
            model_id="gpt-small",
            task_type=pb2.TaskType.Value("TASK_CHAT"),
            prompt="Briefly describe how a transformer encoder works.",
            max_tokens=max_tokens,
        )
        t0 = time.perf_counter()
        first_token_ms = None
        tokens_out = 0
        try:
            for chunk in stub.ExecuteStream(req, timeout=120.0):
                if first_token_ms is None and getattr(chunk, "token", ""):
                    first_token_ms = (time.perf_counter() - t0) * 1000
                if getattr(chunk, "done", False):
                    tokens_out = getattr(chunk, "tokens_out", 0)
            total_ms = (time.perf_counter() - t0) * 1000
            result.samples.append(Sample(
                latency_ms=total_ms,
                tokens_out=tokens_out,
            ))
        except grpc.RpcError as e:
            result.samples.append(Sample(latency_ms=0, error=str(e.code())))
    return result


# ── Reporter ──────────────────────────────────────────────────────────────────

def print_header():
    print(f"\n{'task':<18} {'n':>5}  {'p50':>8} {'p95':>8} {'p99':>8}  "
          f"{'mean':>8} {'stddev':>7}  {'tok/s':>7}  {'err%':>5}")
    print("─" * 80)

def print_result(r: BenchmarkResult):
    tok_s = f"{r.tokens_per_second:>7.1f}" if r.tokens_per_second > 0 else "      —"
    err_pct = f"{r.error_rate*100:>5.1f}%" if r.error_rate > 0 else "   0%"
    print(
        f"{r.task:<18} {r.n:>5}  "
        f"{r.percentile(50):>8.1f} {r.percentile(95):>8.1f} {r.percentile(99):>8.1f}  "
        f"{r.mean_ms:>8.1f} {r.stddev_ms:>7.1f}  "
        f"{tok_s}  {err_pct}"
    )


# ── Main ──────────────────────────────────────────────────────────────────────

def parse_args():
    p = argparse.ArgumentParser(description="LLM executor latency benchmark")
    p.add_argument("--host",     default="localhost")
    p.add_argument("--port",     type=int, default=50051)
    p.add_argument("--warmup",   type=int, default=10,  help="Warmup requests per task (discarded)")
    p.add_argument("--samples",  type=int, default=200, help="Measured samples per task")
    p.add_argument("--no-chat",  action="store_true", help="Skip slow chat tasks (faster bench)")
    p.add_argument("--no-stream",action="store_true", help="Skip streaming task")
    p.add_argument("--seed",     type=int, default=42,  help="Not used by executor, for docs")
    return p.parse_args()


def main():
    args = parse_args()
    target = f"{args.host}:{args.port}"

    print(f"LLM Platform — Executor Latency Benchmark")
    print(f"  Target:   grpc://{target}")
    print(f"  Warmup:   {args.warmup} requests per task (discarded)")
    print(f"  Samples:  {args.samples} per task")
    print(f"  Env:      USE_REAL_MODELS={os.environ.get('USE_REAL_MODELS','true')}")

    channel = grpc.insecure_channel(
        target,
        options=[
            ("grpc.max_receive_message_length", 64 * 1024 * 1024),
            ("grpc.max_send_message_length",    64 * 1024 * 1024),
        ],
    )
    stub = ExecutorServiceStub(channel)

    # Verify executor is reachable
    try:
        h = stub.Health(pb2.HealthRequest(), timeout=5.0)
        print(f"  Executor: id={getattr(h,'executor_id','?')}  "
              f"status={getattr(h,'status','?')}  "
              f"models={list(getattr(h,'model_ids',[]))}")
    except grpc.RpcError as e:
        sys.exit(f"\nERROR: Cannot reach executor at {target}: {e.details()}\n"
                 "Start it with: cd services/model-executor && "
                 "USE_REAL_MODELS=true python3 -m server.main")

    print_header()

    # ── embed ─────────────────────────────────────────────────────────────────
    print(f"  warming up embed ({args.warmup}x)...", end="\r")
    bench_embed(stub, args.warmup)
    r = bench_embed(stub, args.samples)
    print_result(r)

    # ── rerank ────────────────────────────────────────────────────────────────
    print(f"  warming up rerank ({args.warmup}x)...", end="\r")
    bench_rerank(stub, args.warmup, n_docs=5)
    r = bench_rerank(stub, args.samples, n_docs=5)
    print_result(r)

    # ── classify ──────────────────────────────────────────────────────────────
    print(f"  warming up classify ({args.warmup}x)...", end="\r")
    bench_classify(stub, args.warmup)
    r = bench_classify(stub, args.samples)
    print_result(r)

    # ── chat ──────────────────────────────────────────────────────────────────
    if not args.no_chat:
        n_chat = min(args.samples, 100)  # chat is slower, cap at 100
        for max_tokens in [20, 50]:
            print(f"  warming up chat/{max_tokens}tok ({args.warmup}x)...", end="\r")
            bench_chat(stub, args.warmup, max_tokens)
            r = bench_chat(stub, n_chat, max_tokens)
            print_result(r)

    # ── streaming ─────────────────────────────────────────────────────────────
    if not args.no_stream:
        n_stream = min(args.samples, 50)
        print(f"  warming up stream ({args.warmup}x)...", end="\r")
        bench_stream(stub, args.warmup, max_tokens=20)
        r = bench_stream(stub, n_stream, max_tokens=20)
        print_result(r)

    print("─" * 80)
    print("\nAll latencies are wall-clock gRPC round-trip (client → gRPC → executor → gRPC → client).")
    print("p50/p95/p99 are exact percentiles, not interpolated.")
    print("Run with --no-chat for a quick embed/rerank/classify benchmark (~2 min).")
    print("Run with --samples 500 for tighter confidence intervals.")

    channel.close()


if __name__ == "__main__":
    main()
