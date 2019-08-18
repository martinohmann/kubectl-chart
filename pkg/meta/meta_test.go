package meta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func newUnstructured(gvk schema.GroupVersionKind, annotations map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": gvk.GroupVersion().String(),
			"kind":       gvk.Kind,
			"metadata": map[string]interface{}{
				"name":        "foo",
				"annotations": annotations,
			},
		},
	}
}

func TestHasAnnotation(t *testing.T) {
	obj := newUnstructured(schema.GroupVersionKind{}, map[string]interface{}{"foo": "bar"})

	assert.False(t, HasAnnotation(obj, "bar"))
	assert.True(t, HasAnnotation(obj, "foo"))
	assert.False(t, HasAnnotation(obj, "foo", "baz"))
	assert.True(t, HasAnnotation(obj, "foo", "bar"))
}

func TestAddLabel(t *testing.T) {
	obj := newUnstructured(schema.GroupVersionKind{}, nil)

	assert.NoError(t, AddLabel(obj, "foo", "bar"))

	assert.Equal(t, map[string]string{"foo": "bar"}, obj.GetLabels())
}

func TestAddLabelError(t *testing.T) {
	pod := &corev1.Pod{}

	err := AddLabel(pod, "foo", "bar")

	assert.Error(t, err)
	assert.Equal(t, "obj is of type *v1.Pod, expected *unstructured.Unstructured", err.Error())
}

func TestHasGroupKind(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}

	obj := newUnstructured(gvk, nil)

	assert.True(t, HasGroupKind(obj, gvk.GroupKind()))
	assert.False(t, HasGroupKind(obj, schema.GroupKind{Group: "apps", Kind: "Deployment"}))
}

func TestDefaultNamespace(t *testing.T) {
	obj := newUnstructured(schema.GroupVersionKind{}, nil)

	err := DefaultNamespace(obj, "")

	assert.Error(t, err)
	assert.Equal(t, "default namespace cannot be empty", err.Error())

	assert.NoError(t, DefaultNamespace(obj, "foo"))

	assert.Equal(t, "foo", obj.GetNamespace())

	assert.NoError(t, DefaultNamespace(obj, "bar"))

	assert.Equal(t, "foo", obj.GetNamespace())
}

func TestGetObjectHame(t *testing.T) {
	obj := newUnstructured(schema.GroupVersionKind{}, nil)

	assert.Equal(t, "foo", GetObjectName(obj))
}
