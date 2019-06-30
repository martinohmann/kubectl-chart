package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestFindMatching(t *testing.T) {
	cases := []struct {
		name     string
		haystack []runtime.Object
		needle   runtime.Object
		found    bool
	}{
		{
			name:  "object matches",
			found: true,
			haystack: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "SomeKind",
						"metadata": map[string]interface{}{
							"name":      "foo",
							"namespace": "bar",
						},
					},
				},
			},
			needle: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "SomeKind",
					"metadata": map[string]interface{}{
						"name":      "foo",
						"namespace": "bar",
					},
				},
			},
		},
		{
			name: "name mismatch",
			haystack: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "SomeKind",
						"metadata": map[string]interface{}{
							"name":      "bar",
							"namespace": "bar",
						},
					},
				},
			},
			needle: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "SomeKind",
					"metadata": map[string]interface{}{
						"name":      "foo",
						"namespace": "bar",
					},
				},
			},
		},
		{
			name: "kind mismatch",
			haystack: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "SomeOtherKind",
						"metadata": map[string]interface{}{
							"name":      "foo",
							"namespace": "bar",
						},
					},
				},
			},
			needle: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "SomeKind",
					"metadata": map[string]interface{}{
						"name":      "foo",
						"namespace": "bar",
					},
				},
			},
		},
		{
			name: "namespace mismatch",
			haystack: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "SomeKind",
						"metadata": map[string]interface{}{
							"name":      "foo",
							"namespace": "default",
						},
					},
				},
			},
			needle: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "SomeKind",
					"metadata": map[string]interface{}{
						"name":      "foo",
						"namespace": "bar",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, found, err := FindMatching(tc.haystack, tc.needle)

			require.NoError(t, err)

			assert.Equal(t, tc.found, found)
		})
	}
}
