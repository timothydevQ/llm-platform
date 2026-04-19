# ─────────────────────────────────────────────────────────────────────────────
# model-executor Dockerfile
#
# Two build modes:
#
#   docker build .
#     Installs dependencies. Models download on first request and are cached
#     at /root/.cache/huggingface (mount a volume here in production).
#
#   docker build --build-arg PRELOAD_MODELS=true .
#     Pre-bakes model weights into the image layer (~800MB larger).
#     Useful when you want deterministic pod start times with no network I/O.
#
# Run:
#   docker run -p 50051:50051 -p 8085:8085 model-executor
#   docker run -e USE_REAL_MODELS=false model-executor   # MockBackend for CI
# ─────────────────────────────────────────────────────────────────────────────

FROM python:3.11-slim AS base

RUN apt-get update && apt-get install -y --no-install-recommends \
        gcc g++ \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app/model-executor

# Install Python deps first (cached unless requirements.txt changes)
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

# ── Optional: pre-download model weights at build time ────────────────────────
ARG PRELOAD_MODELS=false
RUN if [ "$PRELOAD_MODELS" = "true" ]; then \
      python -c " \
import os; \
os.environ.setdefault('HF_HOME', '/root/.cache/huggingface'); \
from sentence_transformers import SentenceTransformer, CrossEncoder; \
from transformers import pipeline, AutoTokenizer; \
print('Downloading all-MiniLM-L6-v2 …'); \
SentenceTransformer('sentence-transformers/all-MiniLM-L6-v2'); \
print('Downloading ms-marco cross-encoder …'); \
CrossEncoder('cross-encoder/ms-marco-MiniLM-L-6-v2'); \
print('Downloading facebook/opt-125m …'); \
AutoTokenizer.from_pretrained('facebook/opt-125m'); \
pipeline('text-generation', model='facebook/opt-125m', device=-1); \
print('All models cached.') \
"; fi

# ── Application code ──────────────────────────────────────────────────────────
COPY . .

# Create package __init__.py so `python -m server.main` resolves imports
RUN touch __init__.py

ENV GRPC_PORT=50051 \
    HTTP_PORT=8085 \
    MAX_WORKERS=10 \
    EXECUTOR_ID=executor-0 \
    USE_REAL_MODELS=true \
    PYTHONUNBUFFERED=1 \
    PYTHONFAULTHANDLER=1 \
    HF_HOME=/root/.cache/huggingface

EXPOSE 50051 8085

# Run as non-root for security
RUN addgroup --system executor \
 && adduser --system --ingroup executor --no-create-home executor
USER executor

# Run as a Python module so relative imports (..protos, ..backends) resolve
WORKDIR /app
CMD ["python", "-m", "model-executor.server.main"]
