package hook

import (
	"testing"
	"time"

	"github.com/martinohmann/kubectl-chart/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name         string
		obj          runtime.Object
		expectedErr  string
		validateHook func(t *testing.T, h *Hook)
	}{
		{
			name:        "nil object causes error",
			expectedErr: "obj is of type <nil>, expected *unstructured.Unstructured",
		},
		{
			name: "a valid hook without timeout",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name":      "somehook",
						"namespace": "bar",
						"annotations": map[string]interface{}{
							meta.AnnotationHookType:   TypePostApply,
							meta.AnnotationHookNoWait: "true",
						},
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"restartPolicy": "Never",
							},
						},
					},
				},
			},
			validateHook: func(t *testing.T, h *Hook) {
				assert.Equal(t, time.Duration(0), h.WaitTimeout)
				assert.True(t, h.NoWait)
				assert.Equal(t, TypePostApply, h.Type)
			},
		},
		{
			name: "a valid hook with timeout",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name":      "somehook",
						"namespace": "bar",
						"annotations": map[string]interface{}{
							meta.AnnotationHookType:         TypePostApply,
							meta.AnnotationHookAllowFailure: "true",
							meta.AnnotationHookWaitTimeout:  "1h",
						},
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"restartPolicy": "Never",
							},
						},
					},
				},
			},
			validateHook: func(t *testing.T, h *Hook) {
				assert.Equal(t, time.Hour, h.WaitTimeout)
				assert.True(t, h.AllowFailure)
				assert.Equal(t, TypePostApply, h.Type)
			},
		},
		{
			name: "unsupported hook type",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "somehook",
						"namespace": "bar",
						"annotations": map[string]interface{}{
							meta.AnnotationHookType:         TypePostApply,
							meta.AnnotationHookAllowFailure: "true",
							meta.AnnotationHookWaitTimeout:  "1h",
						},
					},
					"data": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			expectedErr: `unsupported hook resource kind "ConfigMap", only "Job" is allowed`,
		},
		{
			name: "unsupported hook type",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name":      "somehook",
						"namespace": "bar",
						"annotations": map[string]interface{}{
							meta.AnnotationHookType:         "foo",
							meta.AnnotationHookAllowFailure: "true",
							meta.AnnotationHookWaitTimeout:  "1h",
						},
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"restartPolicy": "Never",
							},
						},
					},
				},
			},
			expectedErr: `unsupported hook type "foo", allowed values are: [post-apply post-delete pre-apply pre-delete]`,
		},
		{
			name: "conflicting annotations",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name":      "somehook",
						"namespace": "bar",
						"annotations": map[string]interface{}{
							meta.AnnotationHookType:         TypePreApply,
							meta.AnnotationHookAllowFailure: "true",
							meta.AnnotationHookNoWait:       "true",
						},
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"restartPolicy": "Never",
							},
						},
					},
				},
			},
			expectedErr: `annotations cannot be set at the same time: [kubectl-chart/hook-no-wait kubectl-chart/hook-allow-failure]`,
		},
		{
			name: "invalid wait timeout",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name":      "somehook",
						"namespace": "bar",
						"annotations": map[string]interface{}{
							meta.AnnotationHookType:         TypePreApply,
							meta.AnnotationHookAllowFailure: "true",
							meta.AnnotationHookWaitTimeout:  "foo",
						},
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"restartPolicy": "Never",
							},
						},
					},
				},
			},
			expectedErr: `malformed annotation "kubectl-chart/hook-wait-timeout": time: invalid duration foo`,
		},
		{
			name: "conflicting wait annotations",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name":      "somehook",
						"namespace": "bar",
						"annotations": map[string]interface{}{
							meta.AnnotationHookType:        TypePreApply,
							meta.AnnotationHookWaitTimeout: "5m",
							meta.AnnotationHookNoWait:      "true",
						},
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"restartPolicy": "Never",
							},
						},
					},
				},
			},
			expectedErr: `annotations cannot be set at the same time: [kubectl-chart/hook-no-wait kubectl-chart/hook-wait-timeout]`,
		},
		{
			name: "unsupported restartPolicy field value",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name":      "somehook",
						"namespace": "bar",
						"annotations": map[string]interface{}{
							meta.AnnotationHookType:   TypePreApply,
							meta.AnnotationHookNoWait: "true",
						},
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"restartPolicy": "Always",
							},
						},
					},
				},
			},
			expectedErr: `unsupported restartPolicy "Always" in the pod template, only "Never" is allowed`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h, err := Parse(test.obj)
			if test.expectedErr != "" {
				require.Error(t, err)
				assert.Equal(t, test.expectedErr, err.Error())
			} else {
				require.NoError(t, err)
			}

			if test.validateHook != nil {
				test.validateHook(t, h)
			}
		})
	}
}
