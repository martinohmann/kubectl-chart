package chart

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestProcessor_Process(t *testing.T) {
	config := &Config{
		Dir:       "testdata/chart1",
		Name:      "foobar",
		Namespace: "foo",
		Values:    map[interface{}]interface{}{},
	}

	p := NewDefaultProcessor()

	c, err := p.Process(config)

	require.NoError(t, err)

	expectedResources := ResourceList{
		&Resource{
			Unstructured: &unstructured.Unstructured{
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
							LabelChartName:                 "foobar",
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
		},
	}

	expectedHooks := HookMap{
		PostApplyHook: HookList{
			&Hook{
				Resource: &Resource{
					Unstructured: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "batch/v1",
							"kind":       "Job",
							"metadata": map[string]interface{}{
								"name":      "foobar-chart1",
								"namespace": "bar",
								"annotations": map[string]interface{}{
									AnnotationHookType: PostApplyHook,
								},
								"labels": map[string]interface{}{
									"app.kubernetes.io/name":       "chart1",
									"helm.sh/chart":                "chart1-0.1.0",
									"app.kubernetes.io/instance":   "foobar",
									"app.kubernetes.io/managed-by": "Tiller",
									LabelHookChartName:             "foobar",
									LabelHookType:                  PostApplyHook,
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
		},
	}

	assert.Equal(t, expectedResources, c.Resources)
	assert.Equal(t, expectedHooks, c.Hooks)
}

func TestProcessor_ProcessInvalidHook(t *testing.T) {
	config := &Config{
		Dir:       "testdata/chart1",
		Name:      "foobar",
		Namespace: "foo",
		Values: map[interface{}]interface{}{
			"hookType": "foo",
		},
	}

	p := NewDefaultProcessor()

	_, err := p.Process(config)

	require.Error(t, err)
	assert.Equal(t, `invalid hook type "foo", allowed values are "pre-apply", "post-apply", "pre-delete", "post-delete"`, err.Error())
}
