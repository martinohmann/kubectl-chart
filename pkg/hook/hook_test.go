package hook

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNew(t *testing.T) {
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
							AnnotationHookType:   PostApply,
							AnnotationHookNoWait: "true",
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
				timeout, err := h.WaitTimeout()

				assert.NoError(t, err)
				assert.Equal(t, time.Duration(0), timeout)
				assert.True(t, h.NoWait())
				assert.Equal(t, PostApply, h.Type())
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
							AnnotationHookType:         PostApply,
							AnnotationHookAllowFailure: "true",
							AnnotationHookWaitTimeout:  "1h",
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
				timeout, err := h.WaitTimeout()

				assert.NoError(t, err)
				assert.Equal(t, time.Hour, timeout)
				assert.True(t, h.AllowFailure())
				assert.Equal(t, PostApply, h.Type())
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
							AnnotationHookType:         PostApply,
							AnnotationHookAllowFailure: "true",
							AnnotationHookWaitTimeout:  "1h",
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
							AnnotationHookType:         "foo",
							AnnotationHookAllowFailure: "true",
							AnnotationHookWaitTimeout:  "1h",
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
			expectedErr: `unsupported hook type "foo", allowed values are "pre-apply", "post-apply", "pre-delete", "post-delete"`,
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
							AnnotationHookType:         PreApply,
							AnnotationHookAllowFailure: "true",
							AnnotationHookNoWait:       "true",
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
			expectedErr: `invalid hook "somehook": kubectl-chart/hook-no-wait and kubectl-chart/hook-allow-failure cannot be true at the same time`,
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
							AnnotationHookType:         PreApply,
							AnnotationHookAllowFailure: "true",
							AnnotationHookWaitTimeout:  "foo",
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
			expectedErr: `invalid hook "somehook": malformed kubectl-chart/hook-wait-timeout annotation: time: invalid duration foo`,
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
							AnnotationHookType:        PreApply,
							AnnotationHookWaitTimeout: "5m",
							AnnotationHookNoWait:      "true",
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
			expectedErr: `invalid hook "somehook": kubectl-chart/hook-no-wait and kubectl-chart/hook-wait-timeout cannot be set at the same time`,
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
							AnnotationHookType:   PreApply,
							AnnotationHookNoWait: "true",
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
			expectedErr: `invalid hook "somehook": restartPolicy of the pod template must be "Never"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h, err := New(test.obj)
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
