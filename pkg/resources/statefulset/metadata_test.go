package statefulset

import (
	"testing"

	"github.com/martinohmann/kubectl-chart/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestAddOwnerLabels(t *testing.T) {
	tests := []struct {
		name        string
		obj         runtime.Object
		expected    runtime.Object
		expectedErr string
	}{
		{
			name: "add labels if not present",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foobar",
						"namespace": "foo",
					},
					"spec": map[string]interface{}{
						"serviceName": "foobar",
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"foo": "bar",
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"foo": "bar",
								},
							},
						},
						"volumeClaimTemplates": []interface{}{
							map[string]interface{}{
								"metadata": map[string]interface{}{
									"name": "baz",
								},
							},
						},
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foobar",
						"namespace": "foo",
					},
					"spec": map[string]interface{}{
						"serviceName": "foobar",
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"foo":                        "bar",
								meta.LabelOwnedByStatefulSet: "foobar",
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"foo":                        "bar",
									meta.LabelOwnedByStatefulSet: "foobar",
								},
							},
						},
						"volumeClaimTemplates": []interface{}{
							map[string]interface{}{
								"metadata": map[string]interface{}{
									"name": "baz",
									"labels": map[string]interface{}{
										meta.LabelOwnedByStatefulSet: "foobar",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "update labels if present",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foobar",
						"namespace": "foo",
					},
					"spec": map[string]interface{}{
						"serviceName": "foobar",
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"foo":                        "bar",
								meta.LabelOwnedByStatefulSet: "bar",
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"foo":                        "bar",
									meta.LabelOwnedByStatefulSet: "bar",
								},
							},
						},
						"volumeClaimTemplates": []interface{}{
							map[string]interface{}{
								"metadata": map[string]interface{}{
									"name": "baz",
								},
							},
						},
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foobar",
						"namespace": "foo",
					},
					"spec": map[string]interface{}{
						"serviceName": "foobar",
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"foo":                        "bar",
								meta.LabelOwnedByStatefulSet: "foobar",
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"foo":                        "bar",
									meta.LabelOwnedByStatefulSet: "foobar",
								},
							},
						},
						"volumeClaimTemplates": []interface{}{
							map[string]interface{}{
								"metadata": map[string]interface{}{
									"name": "baz",
									"labels": map[string]interface{}{
										meta.LabelOwnedByStatefulSet: "foobar",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "missing spec.volumeClaimTemplates should not cause error",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foobar",
						"namespace": "foo",
					},
					"spec": map[string]interface{}{
						"serviceName": "foobar",
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"foo": "bar",
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foobar",
						"namespace": "foo",
					},
					"spec": map[string]interface{}{
						"serviceName": "foobar",
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"foo":                        "bar",
								meta.LabelOwnedByStatefulSet: "foobar",
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"foo":                        "bar",
									meta.LabelOwnedByStatefulSet: "foobar",
								},
							},
						},
					},
				},
			},
		},
		{
			name:        "malformed matchLabels should cause an error",
			expectedErr: `while setting labels for StatefulSet "foobar": .spec.selector.matchLabels accessor error: foo is of the type string, expected map[string]interface{}`,
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foobar",
						"namespace": "foo",
					},
					"spec": map[string]interface{}{
						"serviceName": "foobar",
						"selector": map[string]interface{}{
							"matchLabels": "foo",
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
		},
		{
			name:        "malformed pod template labels should cause an error",
			expectedErr: `while setting labels for StatefulSet "foobar": .spec.template.metadata.labels accessor error: foo is of the type string, expected map[string]interface{}`,
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foobar",
						"namespace": "foo",
					},
					"spec": map[string]interface{}{
						"serviceName": "foobar",
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"foo": "bar",
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": "foo",
							},
						},
					},
				},
			},
		},
		{
			name:        "malformed volumeClaimTemplates should cause an error",
			expectedErr: `while setting labels for StatefulSet "foobar": .spec.volumeClaimTemplates is of type map[string]interface {}, expected []interface{}`,
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foobar",
						"namespace": "foo",
					},
					"spec": map[string]interface{}{
						"serviceName": "foobar",
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"foo": "bar",
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"foo": "bar",
								},
							},
						},
						"volumeClaimTemplates": map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "baz",
							},
						},
					},
				},
			},
		},
		{
			name:        "malformed volumeClaimTemplate items should cause an error",
			expectedErr: `while setting labels for StatefulSet "foobar": .spec.volumeClaimTemplates[0] is of type string, expected map[string]interface{}`,
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foobar",
						"namespace": "foo",
					},
					"spec": map[string]interface{}{
						"serviceName": "foobar",
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"foo": "bar",
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"foo": "bar",
								},
							},
						},
						"volumeClaimTemplates": []interface{}{
							"foo",
						},
					},
				},
			},
		},
		{
			name:        "malformed volumeClaimTemplate labels should cause an error",
			expectedErr: `while setting labels for StatefulSet "foobar": .metadata.labels accessor error: foo is of the type string, expected map[string]interface{}`,
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foobar",
						"namespace": "foo",
					},
					"spec": map[string]interface{}{
						"serviceName": "foobar",
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"foo": "bar",
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"foo": "bar",
								},
							},
						},
						"volumeClaimTemplates": []interface{}{
							map[string]interface{}{
								"metadata": map[string]interface{}{
									"labels": "foo",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "empty object will receive labels",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foobar",
						"namespace": "foo",
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "foobar",
						"namespace": "foo",
					},
					"spec": map[string]interface{}{
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								meta.LabelOwnedByStatefulSet: "foobar",
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									meta.LabelOwnedByStatefulSet: "foobar",
								},
							},
						},
					},
				},
			},
		},
		{
			name:        "required unstructured objects",
			expectedErr: "obj is of type *v1.StatefulSet, expected *unstructured.Unstructured",
			obj:         &appsv1.StatefulSet{},
		},
		{
			name:        "wrong GroupKind",
			expectedErr: `obj "foobar" is of GroupKind "Job.batch", expected "StatefulSet.apps"`,
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name":      "foobar",
						"namespace": "foo",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := AddOwnerLabels(test.obj)
			if test.expectedErr != "" {
				require.Error(t, err)
				assert.Equal(t, test.expectedErr, err.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expected, test.obj)
			}
		})
	}
}
