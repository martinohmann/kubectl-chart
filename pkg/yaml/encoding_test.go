package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestEncoder_Encode(t *testing.T) {
	e := NewEncoder()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.decodeOnly {
				return
			}

			buf, err := e.Encode(tc.objs)

			require.NoError(t, err)
			assert.Equal(t, string(tc.raw), string(buf))
		})
	}
}

func TestEncoder_EncodeErrors(t *testing.T) {
	e := NewEncoder()

	for _, tc := range encodeErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := e.Encode(tc.objs)

			require.Error(t, err)
		})
	}
}

func TestDecoder_Decode(t *testing.T) {
	d := NewDecoder()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.encodeOnly {
				return
			}

			objs, err := d.Decode(tc.raw)

			require.NoError(t, err)
			assert.Equal(t, tc.objs, objs)
		})
	}
}

func TestDecoder_DecodeError(t *testing.T) {
	d := NewDecoder()

	for _, tc := range decodeErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := d.Decode(tc.raw)

			require.Error(t, err)
		})
	}
}

var testCases = []struct {
	name       string
	objs       []runtime.Object
	raw        []byte
	encodeOnly bool
	decodeOnly bool
}{
	{
		name: "empty",
		raw:  []byte{},
		objs: []runtime.Object{},
	},
	{
		name:       "empty resources",
		raw:        []byte("---\n---\n"),
		objs:       []runtime.Object{},
		decodeOnly: true,
	},
	{
		name: "one resource",
		raw: []byte(`---
apiVersion: v1
data:
  bar: baz
kind: ConfigMap
metadata:
  name: foo
`),
		objs: []runtime.Object{
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name": "foo",
					},
					"data": map[string]interface{}{
						"bar": "baz",
					},
				},
			},
		},
	},
	{
		name: "multiple resources",
		raw: []byte(`---
apiVersion: v1
data:
  bar: baz
kind: ConfigMap
metadata:
  name: foo
---
apiVersion: v1
data:
  baz: qux
kind: Secret
metadata:
  name: bar
`),
		objs: []runtime.Object{
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name": "foo",
					},
					"data": map[string]interface{}{
						"bar": "baz",
					},
				},
			},
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"name": "bar",
					},
					"data": map[string]interface{}{
						"baz": "qux",
					},
				},
			},
		},
	},
}

var decodeErrorCases = []struct {
	name string
	raw  []byte
}{
	{
		name: "malformed yaml",
		raw:  []byte(`}`),
	},
}

var encodeErrorCases = []struct {
	name string
	objs []runtime.Object
}{
	{
		name: "illegal object",
		objs: []runtime.Object{
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": func() {},
				},
			},
		},
	},
}
