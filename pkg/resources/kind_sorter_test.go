package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type nameKind struct {
	name, kind string
}

var (
	unsorted = []nameKind{
		{"foo", "Pod"},
		{"bar", "Prometheus"},
		{"foo", "ConfigMap"},
		{"foo", "Prometheus"},
		{"prometheus", "CustomResourceDefinition"},
		{"bar", "Pod"},
		{"baz", "Alertmanager"},
		{"baz", "Service"},
		{"kube-foo", "Namespace"},
	}
	sortedForApply = []nameKind{
		{"kube-foo", "Namespace"},
		{"foo", "ConfigMap"},
		{"prometheus", "CustomResourceDefinition"},
		{"baz", "Service"},
		{"bar", "Pod"},
		{"foo", "Pod"},
		{"baz", "Alertmanager"},
		{"bar", "Prometheus"},
		{"foo", "Prometheus"},
	}
	sortedForDelete = []nameKind{
		{"baz", "Alertmanager"},
		{"bar", "Prometheus"},
		{"foo", "Prometheus"},
		{"baz", "Service"},
		{"bar", "Pod"},
		{"foo", "Pod"},
		{"prometheus", "CustomResourceDefinition"},
		{"foo", "ConfigMap"},
		{"kube-foo", "Namespace"},
	}
)

func TestSortByKind_ApplyOrder(t *testing.T) {
	given := nameKindsToObjectList(unsorted)
	expected := nameKindsToObjectList(sortedForApply)

	assert.Equal(t, expected, SortByKind(given, ApplyOrder))
}

func TestSortByKind_DeleteOrder(t *testing.T) {
	given := nameKindsToObjectList(unsorted)
	expected := nameKindsToObjectList(sortedForDelete)

	assert.Equal(t, expected, SortByKind(given, DeleteOrder))
}

func nameKindsToObjectList(nameKinds []nameKind) []runtime.Object {
	objs := make([]runtime.Object, len(nameKinds))

	for i, nk := range nameKinds {
		objs[i] = newUnstructured(nk.kind, nk.name)
	}

	return objs
}

func newUnstructured(kind, name string) runtime.Object {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}
}
