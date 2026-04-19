#!/usr/bin/env bash
# generate-proto.sh — regenerate all protobuf stubs
# Usage: bash scripts/generate-proto.sh
#
# Prerequisites:
#   brew install protobuf
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
#   pip install grpcio-tools

set -euo pipefail
cd "$(dirname "$0")/.."

GO_OUT="gen/go"
PY_OUT="services/model-executor/protos"

mkdir -p "$PY_OUT"

echo "Generating Go stubs..."
for proto in \
  proto/inference/v1/inference.proto \
  proto/execution/v1/execution.proto \
  proto/routing/v1/routing.proto \
  proto/scheduling/v1/scheduling.proto \
  proto/platform/v1/platform.proto; do

  echo "  → $proto"
  protoc \
    --proto_path=. \
    --go_out="$GO_OUT" \
    --go_opt=paths=source_relative \
    --go-grpc_out="$GO_OUT" \
    --go-grpc_opt=paths=source_relative \
    "$proto"
done

echo "Generating Python stubs..."
python -m grpc_tools.protoc \
  --proto_path=. \
  --python_out="$PY_OUT" \
  --grpc_python_out="$PY_OUT" \
  proto/execution/v1/execution.proto \
  proto/inference/v1/inference.proto

echo "✓ All stubs generated."
echo ""
echo "NOTE: The generated code in gen/go/ uses the JSON codec (gen/go/codec/)."
echo "In production, replace the hand-written stubs with protoc output and"
echo "add the google.golang.org/protobuf dependency."
