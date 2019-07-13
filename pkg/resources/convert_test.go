package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestToObjectList(t *testing.T) {
	given := []*resource.Info{
		{
			Object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name": "foo",
					},
				},
			},
		}, {
			Object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "bar",
					},
				},
			},
		},
	}

	expected := []runtime.Object{given[0].Object, given[1].Object}

	assert.Equal(t, expected, ToObjectList(given))
}

func TestToInfoList(t *testing.T) {
	given := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{
			{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foo",
						"namespace": "bar",
					},
				},
			},
		},
	}

	expected := []*resource.Info{
		{
			Object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foo",
						"namespace": "bar",
					},
				},
			},
			Name:      "foo",
			Namespace: "bar",
			Mapping: &meta.RESTMapping{
				Resource: schema.GroupVersionResource{
					Group:    "apps",
					Version:  "v1",
					Resource: "statefulsets",
				},
				GroupVersionKind: schema.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "StatefulSet",
				},
			},
		},
	}

	mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme.Scheme)

	infos, err := ToInfoList(given, mapper)

	require.NoError(t, err)
	require.Len(t, infos, len(expected))
	assert.Equal(t, infos[0].Object, expected[0].Object)
	assert.Equal(t, infos[0].Name, expected[0].Name)
	assert.Equal(t, infos[0].Namespace, expected[0].Namespace)
	assert.Equal(t, infos[0].Mapping.Resource, expected[0].Mapping.Resource)
	assert.Equal(t, infos[0].Mapping.GroupVersionKind, expected[0].Mapping.GroupVersionKind)

}
