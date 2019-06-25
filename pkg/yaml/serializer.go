package yaml

import (
	"bytes"
	"io"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// Serializer converts between raw multi-resource yaml manifests and slices of
// runtime.Object.
type Serializer struct {
	encoder runtime.Encoder
	decoder runtime.Decoder
}

// NewSerializer creates a new *Serializer.
func NewSerializer() *Serializer {
	return &Serializer{
		encoder: json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil),
		decoder: unstructured.UnstructuredJSONScheme,
	}
}

// Decode decodes a multi-resource yaml into a slice of runtime.Object. The
// resulting objects are of type *unstructured.Unstructured.
func (s *Serializer) Decode(raw []byte) ([]runtime.Object, error) {
	d := yaml.NewYAMLOrJSONDecoder(bytes.NewBuffer(raw), 4096)

	objs := make([]runtime.Object, 0)

	for {
		ext := runtime.RawExtension{}
		if err := d.Decode(&ext); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		ext.Raw = bytes.TrimSpace(ext.Raw)
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}

		obj, _, err := s.decoder.Decode(ext.Raw, nil, nil)
		if err != nil {
			return nil, err
		}

		objs = append(objs, obj)
	}

	return objs, nil
}

// Encode encodes a slice of runtime.Object to a multi-resource yaml.
func (s *Serializer) Encode(objs []runtime.Object) ([]byte, error) {
	var buf bytes.Buffer

	for _, obj := range objs {
		_, err := buf.WriteString("---\n")
		if err != nil {
			return nil, errors.Wrap(err, "failed to write yaml separator")
		}

		err = s.encoder.Encode(obj, &buf)
		if err != nil {
			return nil, errors.Wrap(err, "failed to encode object to yaml")
		}
	}

	return buf.Bytes(), nil
}
