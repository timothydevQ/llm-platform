# ─────────────────────────────────────────────────────────────────────────────
# llm-platform Makefile
#
# Targets
#   make proto       Regenerate protobuf stubs from proto/**/*.proto
#   make test-go     Run all Go tests with -race
#   make test-py     Run Python unit tests (no model weights required)
#   make test-int    Run Python integration tests (requires HF model weights)
#   make bench       Run reproducible latency benchmarks against a live executor
#   make lint        Run golangci-lint + ruff
#   make build       Build all Go service binaries
#   make docker      Build all Docker images
#   make migrate     Apply SQL migrations to DB_PATH
#   make clean       Remove generated binaries
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: proto test-go test-py test-int bench lint build docker migrate clean

# ── Paths ─────────────────────────────────────────────────────────────────────
PROTO_SRC   := proto
GEN_GO      := gen/go
GEN_PY      := services/model-executor/protos
SERVICES    := api-gateway router scheduler control-plane
EXECUTOR    := services/model-executor
DB_PATH     ?= /tmp/llm-platform-dev.db
BENCH_HOST  ?= localhost
BENCH_PORT  ?= 50051

# ── Proto generation ──────────────────────────────────────────────────────────
#
# Prerequisites (install once):
#   brew install protobuf
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
#   pip install grpcio-tools
#
# After running `make proto`, the generated files in gen/go/ and
# services/model-executor/protos/ replace the hand-written stubs.
# The hand-written stubs use the same protowire binary encoding and are
# wire-compatible with protoc output — `make proto` is a drop-in upgrade.

PROTOS := \
  $(PROTO_SRC)/inference/v1/inference.proto \
  $(PROTO_SRC)/execution/v1/execution.proto \
  $(PROTO_SRC)/routing/v1/routing.proto \
  $(PROTO_SRC)/scheduling/v1/scheduling.proto \
  $(PROTO_SRC)/platform/v1/platform.proto

proto:
	@echo "→ Checking prerequisites..."
	@command -v protoc            >/dev/null 2>&1 || (echo "ERROR: protoc not found. Install: brew install protobuf"; exit 1)
	@command -v protoc-gen-go     >/dev/null 2>&1 || (echo "ERROR: protoc-gen-go not found. Install: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; exit 1)
	@command -v protoc-gen-go-grpc>/dev/null 2>&1 || (echo "ERROR: protoc-gen-go-grpc not found. Install: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"; exit 1)
	@python3 -c "import grpc_tools" 2>/dev/null  || (echo "ERROR: grpcio-tools not found. Install: pip install grpcio-tools"; exit 1)

	@echo "→ Generating Go stubs..."
	@mkdir -p $(GEN_GO)
	@for proto in $(PROTOS); do \
	  echo "  protoc $$proto"; \
	  protoc \
	    --proto_path=. \
	    --go_out=$(GEN_GO) \
	    --go_opt=paths=source_relative \
	    --go-grpc_out=$(GEN_GO) \
	    --go-grpc_opt=paths=source_relative \
	    $$proto; \
	done

	@echo "→ Generating Python stubs..."
	@mkdir -p $(GEN_PY)
	@python3 -m grpc_tools.protoc \
	  --proto_path=. \
	  --python_out=$(GEN_PY) \
	  --grpc_python_out=$(GEN_PY) \
	  $(PROTO_SRC)/execution/v1/execution.proto \
	  $(PROTO_SRC)/inference/v1/inference.proto

	@echo "→ Patching Python import paths..."
	@# grpcio-tools emits absolute imports; fix to relative for package use
	@sed -i.bak 's/^import execution_v1_pb2/from . import execution_v1_pb2/g' $(GEN_PY)/*_pb2_grpc.py 2>/dev/null || true
	@find $(GEN_PY) -name "*.bak" -delete

	@echo "→ Removing hand-written codec stub (replaced by protoc output)..."
	@# The protoc-generated files implement proto.Message directly via protoimpl,
	@# so the hand-written ProtoCodec in gen/go/codec/ is no longer needed.
	@# gRPC will use its built-in protobuf codec automatically.
	@echo "   NOTE: Delete gen/go/codec/ and remove its import from service mains."
	@echo "   The ProtoCodec was only needed for the hand-written stubs."

	@echo "✓ Proto generation complete."
	@echo ""
	@echo "Next steps:"
	@echo "  1. Delete gen/go/codec/ (no longer needed with protoc output)"
	@echo "  2. Remove '_ \"github.com/timothydevQ/llm-platform/gen/codec\"' imports"
	@echo "  3. Run: make test-go"

# ── Go tests ──────────────────────────────────────────────────────────────────
test-go:
	@echo "→ Running Go tests (race detector enabled)..."
	@for svc in $(SERVICES); do \
	  echo "  Testing services/$$svc ..."; \
	  cd services/$$svc && go test -race -count=1 -timeout=60s ./... && cd ../..; \
	done
	@echo "✓ All Go tests passed."

# ── Python tests ──────────────────────────────────────────────────────────────
test-py:
	@echo "→ Running Python unit tests (MockBackend, no model weights)..."
	cd $(EXECUTOR) && \
	  USE_REAL_MODELS=false python3 -m pytest tests/test_executor.py -v --tb=short
	@echo "✓ Python unit tests passed."

test-int:
	@echo "→ Running Python integration tests (requires model weights)..."
	@echo "   Set HF_HOME to a directory with cached weights to avoid re-downloading."
	cd $(EXECUTOR) && \
	  USE_REAL_MODELS=true python3 -m pytest tests/test_integration.py -v --tb=short -s
	@echo "✓ Python integration tests passed."

# ── Benchmarks ────────────────────────────────────────────────────────────────
#
# Runs the reproducible latency benchmark against a live executor.
# Start the executor first: cd services/model-executor && python3 -m server.main
#
# Output format:
#   task       n    p50_ms   p95_ms   p99_ms   mean_ms  stddev   tokens/s
#   embed      200  31.2     48.7     67.3     33.1     9.2      —
#   ...
bench:
	@echo "→ Running benchmarks against executor at $(BENCH_HOST):$(BENCH_PORT)..."
	cd $(EXECUTOR) && python3 scripts/benchmark.py \
	  --host $(BENCH_HOST) \
	  --port $(BENCH_PORT) \
	  --warmup 10 \
	  --samples 200

# ── Lint ──────────────────────────────────────────────────────────────────────
lint:
	@echo "→ Running Go lint..."
	@command -v golangci-lint >/dev/null 2>&1 && \
	  for svc in $(SERVICES); do golangci-lint run ./services/$$svc/...; done || \
	  echo "  (golangci-lint not installed, skipping)"
	@echo "→ Running Python lint..."
	@command -v ruff >/dev/null 2>&1 && \
	  ruff check $(EXECUTOR) || echo "  (ruff not installed, skipping)"

# ── Build ─────────────────────────────────────────────────────────────────────
build:
	@echo "→ Building Go services..."
	@for svc in $(SERVICES); do \
	  echo "  Building $$svc..."; \
	  cd services/$$svc && CGO_ENABLED=1 go build -o bin/$$svc ./cmd/ && cd ../..; \
	done
	@echo "✓ All binaries built in services/<name>/bin/"

# ── Docker ────────────────────────────────────────────────────────────────────
docker:
	@echo "→ Building Docker images..."
	@for svc in $(SERVICES); do \
	  docker build -t llm-platform/$$svc:latest services/$$svc; \
	done
	docker build -t llm-platform/model-executor:latest services/model-executor
	@echo "✓ All images built."

# ── Migrations ────────────────────────────────────────────────────────────────
migrate:
	@echo "→ Applying SQL migrations to $(DB_PATH)..."
	DB_PATH=$(DB_PATH) bash scripts/run-migrations.sh
	@echo "✓ Migrations applied."

# ── Clean ─────────────────────────────────────────────────────────────────────
clean:
	@find services -name "bin" -type d -exec rm -rf {} + 2>/dev/null || true
	@find . -name "*.pyc" -delete
	@find . -name "__pycache__" -type d -exec rm -rf {} + 2>/dev/null || true
	@echo "✓ Cleaned."
