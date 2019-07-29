package resources

import "k8s.io/apimachinery/pkg/runtime"

// Encoder is the interface for an encoder that encodes multiple objects as
// slices of bytes.
type Encoder interface {
	// Encode encodes slices of runtime.Object as bytes.
	Encode(objs []runtime.Object) ([]byte, error)
}

// Decoder is the interface of something that can decode raw bytes into slices
// of runtime.Object.
type Decoder interface {
	// Decode decodes raw bytes into slices of runtime.Object.
	Decode(raw []byte) ([]runtime.Object, error)
}

// Serializer can encode and decode slices of runtime.Object.
type Serializer interface {
	Encoder
	Decoder
}
