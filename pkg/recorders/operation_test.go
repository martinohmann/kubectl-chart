package recorders

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestOperationRecorder(t *testing.T) {
	r := NewOperationRecorder()

	r.Record("created", newUnstructuredObj("foo"))
	r.Record("created", newUnstructuredObj("bar"))
	r.Record("deleted", newUnstructuredObj("baz"))

	assert.Equal(
		t,
		[]runtime.Object{
			newUnstructuredObj("foo"),
			newUnstructuredObj("bar"),
		},
		r.RecordedObjects("created"),
	)

	assert.Equal(
		t,
		[]runtime.Object{
			newUnstructuredObj("baz"),
		},
		r.RecordedObjects("deleted"),
	)

	assert.Nil(t, r.RecordedObjects("unchanged"))
}

func newUnstructuredObj(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": name,
			},
			"data": map[string]interface{}{},
		},
	}
}
