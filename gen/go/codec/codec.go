// Package codec provides a protobuf binary codec for gRPC.
//
// This codec replaces the default gRPC protobuf codec with one that can
// marshal/unmarshal the hand-written message types in this module.
// Messages must implement ProtoMessage to participate in binary encoding.
//
// Why this exists instead of running protoc:
//   The proto definitions are in proto/**/*.proto. The Makefile target
//   `make proto` runs protoc and replaces this package with standard
//   protoc-gen-go output. Until then, this package provides binary-
//   compatible protobuf wire encoding using google.golang.org/protobuf/encoding/protowire.
//
// Wire format is standard protobuf binary — NOT JSON. Messages encoded here
// are readable by any protobuf library given the .proto schema.
package codec

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
	grpcencoding "google.golang.org/grpc/encoding"
)

func init() {
	grpcencoding.RegisterCodec(ProtoCodec{})
}

// ProtoMessage is implemented by every request/response type in gen/go.
// Replace this with proto.Message once protoc output is committed.
type ProtoMessage interface {
	MarshalProto() ([]byte, error)
	UnmarshalProto([]byte) error
}

// ProtoCodec serialises messages as protobuf binary using protowire.
// This is wire-compatible with standard protoc-generated code.
type ProtoCodec struct{}

func (ProtoCodec) Name() string { return "proto" }

func (ProtoCodec) Marshal(v any) ([]byte, error) {
	if m, ok := v.(ProtoMessage); ok {
		return m.MarshalProto()
	}
	return nil, fmt.Errorf("codec: %T does not implement ProtoMessage; run `make proto`", v)
}

func (ProtoCodec) Unmarshal(data []byte, v any) error {
	if m, ok := v.(ProtoMessage); ok {
		return m.UnmarshalProto(data)
	}
	return fmt.Errorf("codec: %T does not implement ProtoMessage; run `make proto`", v)
}

// ── Wire helpers (re-exported for message implementations) ────────────────────

// AppendTag appends a protobuf field tag.
func AppendTag(b []byte, fieldNum protowire.Number, typ protowire.Type) []byte {
	return protowire.AppendTag(b, fieldNum, typ)
}

// ConsumeTag reads a protobuf field tag.
func ConsumeTag(b []byte) (protowire.Number, protowire.Type, int) {
	return protowire.ConsumeTag(b)
}
// tw_6059_20995
// tw_6059_26517
// tw_6059_28024
// tw_6059_24928
