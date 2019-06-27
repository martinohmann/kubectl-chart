package chart

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestFilterHooks(t *testing.T) {
	testObjs := []runtime.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name": "foo",
					"annotations": map[string]interface{}{
						AnnotationHook: PreApplyHook,
					},
				},
				"data": map[string]interface{}{
					"bar": "baz",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name": "baz",
					"annotations": map[string]interface{}{
						AnnotationHook: PostApplyHook,
					},
				},
				"data": map[string]interface{}{
					"bar": "baz",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name": "bar",
					"annotations": map[string]interface{}{
						AnnotationHook: PostApplyHook,
					},
				},
				"data": map[string]interface{}{
					"bar": "baz",
				},
			},
		},
	}

	cases := []struct {
		name                 string
		hookType             string
		expected             []runtime.Object
		expectedErrorMessage string
	}{
		{
			name:     "filter for hook type that is not present in input",
			hookType: PostDeleteHook,
			expected: []runtime.Object{},
		},
		{
			name:                 "filter for invalid hook type",
			hookType:             "some-invalid-hook-type",
			expectedErrorMessage: `invalid hook type "some-invalid-hook-type", allowed values are "pre-apply", "post-apply", "pre-delete", "post-delete"`,
		},
		{
			name:     "filter pre-apply hooks",
			hookType: PreApplyHook,
			expected: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name": "foo",
							"annotations": map[string]interface{}{
								AnnotationHook: PreApplyHook,
							},
						},
						"data": map[string]interface{}{
							"bar": "baz",
						},
					},
				},
			},
		},
		{
			name:     "filter post-apply hooks",
			hookType: PostApplyHook,
			expected: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name": "baz",
							"annotations": map[string]interface{}{
								AnnotationHook: PostApplyHook,
							},
						},
						"data": map[string]interface{}{
							"bar": "baz",
						},
					},
				},
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name": "bar",
							"annotations": map[string]interface{}{
								AnnotationHook: PostApplyHook,
							},
						},
						"data": map[string]interface{}{
							"bar": "baz",
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := FilterHooks(tc.hookType, testObjs...)
			if tc.expectedErrorMessage != "" {
				require.Error(t, err)
				assert.Equal(t, tc.expectedErrorMessage, err.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, res)
			}
		})
	}
}
