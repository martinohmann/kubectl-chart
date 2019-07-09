package chart

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newSortResource(kind, name string) *Resource {
	return &Resource{
		Unstructured: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       kind,
				"metadata": map[string]interface{}{
					"name": name,
				},
			},
		},
	}
}

var unsorted = []*Resource{
	newSortResource("Pod", "foo"),
	newSortResource("Prometheus", "bar"),
	newSortResource("ConfigMap", "foo"),
	newSortResource("Prometheus", "foo"),
	newSortResource("CustomResourceDefinition", "prometheus"),
	newSortResource("Pod", "bar"),
	newSortResource("Alertmanager", "baz"),
	newSortResource("Service", "baz"),
	newSortResource("Namespace", "kube-foo"),
}

func TestSortResources_ApplyOrder(t *testing.T) {
	expected := []*Resource{
		newSortResource("Namespace", "kube-foo"),
		newSortResource("ConfigMap", "foo"),
		newSortResource("CustomResourceDefinition", "prometheus"),
		newSortResource("Service", "baz"),
		newSortResource("Pod", "bar"),
		newSortResource("Pod", "foo"),
		newSortResource("Alertmanager", "baz"),
		newSortResource("Prometheus", "bar"),
		newSortResource("Prometheus", "foo"),
	}

	assert.Equal(t, expected, SortResources(unsorted, ApplyOrder))
}

func TestSortResources_DeleteOrder(t *testing.T) {
	expected := []*Resource{
		newSortResource("Alertmanager", "baz"),
		newSortResource("Prometheus", "bar"),
		newSortResource("Prometheus", "foo"),
		newSortResource("Service", "baz"),
		newSortResource("Pod", "bar"),
		newSortResource("Pod", "foo"),
		newSortResource("CustomResourceDefinition", "prometheus"),
		newSortResource("ConfigMap", "foo"),
		newSortResource("Namespace", "kube-foo"),
	}

	assert.Equal(t, expected, SortResources(unsorted, DeleteOrder))
}
