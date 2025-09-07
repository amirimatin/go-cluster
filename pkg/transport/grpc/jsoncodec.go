package grpc

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
)

// jsonCodec is a simple gRPC codec for JSON payloads, allowing us to avoid
// protobuf codegen for internal management calls.
type jsonCodec struct{}

func (jsonCodec) Marshal(v interface{}) ([]byte, error)   { return json.Marshal(v) }
func (jsonCodec) Unmarshal(b []byte, v interface{}) error { return json.Unmarshal(b, v) }
func (jsonCodec) Name() string                            { return "json" }

func init() {
	// Register once at package init.
	encoding.RegisterCodec(jsonCodec{})
}
