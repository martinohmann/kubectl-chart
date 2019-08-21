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
	tests := []struct {
		name        string
		obj         interface{}
		expected    []*resource.Info
		expectedErr string
	}{
		{
			name: "non-list-type object",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foo",
						"namespace": "bar",
					},
				},
			},
			expectedErr: "*unstructured.Unstructured is not a list: no Items field in this object",
		},
		{
			name:        "non-object",
			obj:         "foo",
			expectedErr: "got string, expected list type runtime.Object or []runtime.Object",
		},
		{
			name: "unstructured list",
			obj: &unstructured.UnstructuredList{
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
			},
			expected: []*resource.Info{
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
			},
		},
		{
			name: "slice of objects",
			obj: []runtime.Object{
				&unstructured.Unstructured{
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
			expected: []*resource.Info{
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
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme.Scheme)

			infos, err := ToInfoList(test.obj, mapper)
			if test.expectedErr != "" {
				require.Error(t, err)
				assert.Equal(t, test.expectedErr, err.Error())
			} else {
				require.NoError(t, err)

				require.Len(t, infos, len(test.expected))

				for i, expected := range test.expected {
					assert.Equal(t, infos[i].Object, expected.Object)
					assert.Equal(t, infos[i].Name, expected.Name)
					assert.Equal(t, infos[i].Namespace, expected.Namespace)
					assert.Equal(t, infos[i].Mapping.Resource, expected.Mapping.Resource)
					assert.Equal(t, infos[i].Mapping.GroupVersionKind, expected.Mapping.GroupVersionKind)
				}
			}
		})
	}
}
