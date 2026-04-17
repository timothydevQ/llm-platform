// Package codec registers a JSON codec that overrides gRPC's default protobuf
// codec, allowing plain Go structs to be used as gRPC message types without
// the protobuf runtime or protoc compilation.
//
// Import with a blank identifier in each service's main package:
//
//	import _ "github.com/timothydevQ/llm-platform/gen/codec"
package codec

import (
	"encoding/json"

	grpcencoding "google.golang.org/grpc/encoding"
)

func init() {
	grpcencoding.RegisterCodec(JSONCodec{})
}

// JSONCodec serialises gRPC messages as JSON instead of protobuf.
type JSONCodec struct{}

func (JSONCodec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (JSONCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
func (JSONCodec) Name() string                        { return "proto" } // overrides default codec
