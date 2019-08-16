package yaml

import (
	"bytes"
	"io"

	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var (
	_ resources.Encoder = Encoder{}
	_ resources.Decoder = Decoder{}
)

// Encoder is a wrapper for a runtime.Encoder which properly encodes a slice of
// runtime.Object into a multi-YAML document.
type Encoder struct {
	runtime.Encoder
}

// NewEncoder creates a new *Encoder value.
func NewEncoder() Encoder {
	return Encoder{json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)}
}

// Encode encodes a slice of runtime.Object to a multi-resource yaml.
func (e Encoder) Encode(objs []runtime.Object) ([]byte, error) {
	var buf bytes.Buffer

	for _, obj := range objs {
		_, err := buf.WriteString("---\n")
		if err != nil {
			return nil, errors.Wrap(err, "failed to write yaml separator")
		}

		err = e.Encoder.Encode(obj, &buf)
		if err != nil {
			return nil, errors.Wrap(err, "failed to encode object to yaml")
		}
	}

	return buf.Bytes(), nil
}

// Decoder is a wrapper for a runtime.Decoder which properly decodes multi-YAML
// documents into a slice of runtime.Object.
type Decoder struct {
	runtime.Decoder
}

// NewDecoder creates a new *Decoder value.
func NewDecoder() Decoder {
	return Decoder{unstructured.UnstructuredJSONScheme}
}

// Decode decodes a multi-resource yaml into a slice of runtime.Object. The
// resulting objects are of type *unstructured.Unstructured.
func (d Decoder) Decode(raw []byte) ([]runtime.Object, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewBuffer(raw), 4096)

	objs := make([]runtime.Object, 0)

	for {
		ext := runtime.RawExtension{}
		if err := decoder.Decode(&ext); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		ext.Raw = bytes.TrimSpace(ext.Raw)
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}

		obj, _, err := d.Decoder.Decode(ext.Raw, nil, nil)
		if err != nil {
			return nil, err
		}

		objs = append(objs, obj)
	}

	return objs, nil
}
