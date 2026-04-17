package codec_test

import (
	"testing"

	_ "github.com/timothydevQ/llm-platform/gen/codec"
	grpcencoding "google.golang.org/grpc/encoding"
)

func TestCodecRegistered(t *testing.T) {
	c := grpcencoding.GetCodec("proto")
	if c == nil {
		t.Fatal("expected codec registered")
	}
}

func TestCodecName(t *testing.T) {
	c := grpcencoding.GetCodec("proto")
	if c.Name() != "proto" {
		t.Errorf("expected 'proto', got %q", c.Name())
	}
}

func TestCodecRoundTrip(t *testing.T) {
	type msg struct {
		ID  string  `json:"id"`
		Val float64 `json:"val"`
	}
	c := grpcencoding.GetCodec("proto")
	data, err := c.Marshal(&msg{ID: "abc", Val: 3.14})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got msg
	if err := c.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.ID != "abc" || got.Val != 3.14 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestCodecNilSlice(t *testing.T) {
	type msg struct {
		Vals []float64 `json:"vals"`
	}
	c := grpcencoding.GetCodec("proto")
	data, err := c.Marshal(&msg{Vals: []float64{1, 2, 3}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got msg
	c.Unmarshal(data, &got)
	if len(got.Vals) != 3 {
		t.Errorf("expected 3 vals, got %d", len(got.Vals))
	}
}
