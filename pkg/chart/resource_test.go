package chart

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestResource_DefaultNamespace(t *testing.T) {
	r := NewResource(&unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "SomeKind",
			"metadata": map[string]interface{}{
				"name": "bar",
			},
		},
	})

	err := r.DefaultNamespace("")

	require.Error(t, err)

	err = r.DefaultNamespace("bar")

	require.NoError(t, err)

	r.GetNamespace()

	assert.Equal(t, "bar", r.GetNamespace())

	err = r.DefaultNamespace("default")

	require.NoError(t, err)

	assert.Equal(t, "bar", r.GetNamespace())
}
