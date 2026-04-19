#!/usr/bin/env bash
# download-models.sh — Pre-warm HuggingFace model cache.
#
# Run this once on a machine before starting the executor, or bake it into
# your Docker build with PRELOAD_MODELS=true.
#
# The models are cached at $HF_HOME (default: ~/.cache/huggingface).
# In Kubernetes, mount a PersistentVolumeClaim at that path so models
# survive pod restarts without re-downloading.
#
# Usage:
#   bash scripts/download-models.sh
#   HF_HOME=/mnt/model-cache bash scripts/download-models.sh

set -euo pipefail
cd "$(dirname "$0")/.."

: "${HF_HOME:=${HOME}/.cache/huggingface}"
export HF_HOME

echo "=== LLM Platform — model pre-download ==="
echo "Cache directory: ${HF_HOME}"
echo ""

python3 - << 'PYEOF'
import os, sys, time

def section(title):
    print(f"\n── {title} {'─' * (50 - len(title))}")

section("sentence-transformers/all-MiniLM-L6-v2  (embedding)")
t = time.time()
from sentence_transformers import SentenceTransformer
model = SentenceTransformer("sentence-transformers/all-MiniLM-L6-v2")
vec   = model.encode(["smoke test"], convert_to_numpy=True, normalize_embeddings=True)
print(f"   ✓ loaded in {time.time()-t:.1f}s  dim={vec.shape[1]}  norm={float((vec**2).sum()**0.5):.4f}")

section("cross-encoder/ms-marco-MiniLM-L-6-v2  (reranking)")
t = time.time()
from sentence_transformers import CrossEncoder
ce = CrossEncoder("cross-encoder/ms-marco-MiniLM-L-6-v2")
sc = ce.predict([("what is python", "Python is a programming language")])
print(f"   ✓ loaded in {time.time()-t:.1f}s  sample_score={float(sc[0]):.4f}")

section("facebook/opt-125m  (text generation)")
t = time.time()
from transformers import pipeline, AutoTokenizer
tok = AutoTokenizer.from_pretrained("facebook/opt-125m")
gen = pipeline("text-generation", model="facebook/opt-125m", device=-1, pad_token_id=50256)
out = gen("The capital of France is", max_new_tokens=10, do_sample=False, return_full_text=False)
print(f"   ✓ loaded in {time.time()-t:.1f}s  sample='{out[0]['generated_text'].strip()[:40]}'")

section("cross-encoder/nli-distilroberta-base  (classification)")
t = time.time()
clf = pipeline("zero-shot-classification", model="cross-encoder/nli-distilroberta-base", device=-1)
res = clf("This product is excellent", candidate_labels=["positive","negative","neutral"])
print(f"   ✓ loaded in {time.time()-t:.1f}s  top_label={res['labels'][0]}  score={res['scores'][0]:.4f}")

section("Summary")
print("   All models cached and verified.")
print(f"   Cache: {os.environ['HF_HOME']}")
PYEOF
