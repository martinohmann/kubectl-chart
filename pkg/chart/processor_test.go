package chart

import (
	"testing"

	"github.com/martinohmann/kubectl-chart/pkg/hook"
	"github.com/martinohmann/kubectl-chart/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestProcessor_Process(t *testing.T) {
	config := &Config{
		Dir:       "testdata/valid-charts/chart1",
		Name:      "foobar",
		Namespace: "foo",
		Values:    map[interface{}]interface{}{},
	}

	p := NewDefaultProcessor()

	c, err := p.Process(config)

	require.NoError(t, err)

	expectedResources := []runtime.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Service",
				"metadata": map[string]interface{}{
					"name":      "foobar-chart1",
					"namespace": "foo",
					"labels": map[string]interface{}{
						"app.kubernetes.io/name":       "chart1",
						"helm.sh/chart":                "chart1-0.1.0",
						"app.kubernetes.io/instance":   "foobar",
						"app.kubernetes.io/managed-by": "Tiller",
						meta.LabelChartName:            "foobar",
					},
				},
				"spec": map[string]interface{}{
					"type": "ClusterIP",
					"ports": []interface{}{
						map[string]interface{}{
							"port":       int64(80),
							"targetPort": "http",
							"protocol":   "TCP",
							"name":       "http",
						},
					},
					"selector": map[string]interface{}{
						"app.kubernetes.io/name":     "chart1",
						"app.kubernetes.io/instance": "foobar",
					},
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "StatefulSet",
				"metadata": map[string]interface{}{
					"name":      "foobar-chart1",
					"namespace": "foo",
					"labels": map[string]interface{}{
						meta.LabelChartName: "foobar",
					},
				},
				"spec": map[string]interface{}{
					"serviceName": "foobar-chart1",
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"foo":                        "bar",
							meta.LabelOwnedByStatefulSet: "foobar-chart1",
						},
					},
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"foo":                        "bar",
								meta.LabelOwnedByStatefulSet: "foobar-chart1",
							},
						},
					},
					"volumeClaimTemplates": []interface{}{
						map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "baz",
								"labels": map[string]interface{}{
									meta.LabelOwnedByStatefulSet: "foobar-chart1",
								},
							},
						},
					},
				},
			},
		},
	}

	expectedHooks := hook.Map{
		hook.PostApply: hook.List{
			&hook.Hook{
				Unstructured: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "batch/v1",
						"kind":       "Job",
						"metadata": map[string]interface{}{
							"name":      "foobar-chart1",
							"namespace": "bar",
							"annotations": map[string]interface{}{
								meta.AnnotationHookType: hook.PostApply,
							},
							"labels": map[string]interface{}{
								"app.kubernetes.io/name":       "chart1",
								"helm.sh/chart":                "chart1-0.1.0",
								"app.kubernetes.io/instance":   "foobar",
								"app.kubernetes.io/managed-by": "Tiller",
								meta.LabelHookChartName:        "foobar",
								meta.LabelHookType:             hook.PostApply,
							},
						},
						"spec": map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"restartPolicy": "Never",
									"containers": []interface{}{
										map[string]interface{}{
											"name":            "chart1",
											"image":           "nginx:stable",
											"imagePullPolicy": "IfNotPresent",
											"ports": []interface{}{
												map[string]interface{}{
													"containerPort": int64(80),
													"protocol":      "TCP",
													"name":          "http",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	assert.Equal(t, expectedResources, c.Resources)
	assert.Equal(t, expectedHooks, c.Hooks)
}

func TestProcessor_ProcessUnsupportedHook(t *testing.T) {
	config := &Config{
		Dir:       "testdata/valid-charts/chart1",
		Name:      "foobar",
		Namespace: "foo",
		Values: map[interface{}]interface{}{
			"hookType": "foo",
		},
	}

	p := NewDefaultProcessor()

	_, err := p.Process(config)

	require.Error(t, err)
	assert.Equal(t, `while parsing template "chart1/templates/hook.yaml": unsupported hook type "foo", allowed values are "pre-apply", "post-apply", "pre-delete", "post-delete"`, err.Error())
}
