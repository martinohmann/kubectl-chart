package chart

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestProcessor_Process(t *testing.T) {
	config := &Config{
		Dir:       "testdata/testchart",
		Name:      "foobar",
		Namespace: "foo",
		Values:    map[string]interface{}{},
	}

	p := NewDefaultProcessor()

	resources, hooks, err := p.Process(config)

	require.NoError(t, err)

	expectedResources := []runtime.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Service",
				"metadata": map[string]interface{}{
					"name": "foobar-testchart",
					"labels": map[string]interface{}{
						"app.kubernetes.io/name":       "testchart",
						"helm.sh/chart":                "testchart-0.1.0",
						"app.kubernetes.io/instance":   "foobar",
						"app.kubernetes.io/managed-by": "Tiller",
						LabelName:                      "foobar",
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
						"app.kubernetes.io/name":     "testchart",
						"app.kubernetes.io/instance": "foobar",
					},
				},
			},
		},
	}

	expectedHooks := []runtime.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name": "foobar-testchart",
					"annotations": map[string]interface{}{
						AnnotationHook: PostApplyHook,
					},
					"labels": map[string]interface{}{
						"app.kubernetes.io/name":       "testchart",
						"helm.sh/chart":                "testchart-0.1.0",
						"app.kubernetes.io/instance":   "foobar",
						"app.kubernetes.io/managed-by": "Tiller",
						LabelName:                      "foobar",
					},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":            "testchart",
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
	}

	assert.Equal(t, expectedResources, resources)
	assert.Equal(t, expectedHooks, hooks)
}

func TestProcessor_ProcessInvalidHook(t *testing.T) {
	config := &Config{
		Dir:       "testdata/testchart",
		Name:      "foobar",
		Namespace: "foo",
		Values: map[string]interface{}{
			"hookType": "foo",
		},
	}

	p := NewDefaultProcessor()

	_, _, err := p.Process(config)

	require.Error(t, err)
	assert.Equal(t, `invalid hook type "foo", allowed values are "pre-apply", "post-apply", "pre-delete", "post-delete"`, err.Error())
}
