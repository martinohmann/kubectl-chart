package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/apimachinery/pkg/api/meta"
)

func TestEnsureNamespaceSet(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "SomeKind",
			"metadata": map[string]interface{}{
				"name": "bar",
			},
		},
	}

	err := EnsureNamespaceSet("", obj)

	require.Error(t, err)

	err = EnsureNamespaceSet("bar", obj)

	require.NoError(t, err)

	accessor, _ := meta.Accessor(obj)

	assert.Equal(t, "bar", accessor.GetNamespace())

	err = EnsureNamespaceSet("default", obj)

	require.NoError(t, err)

	assert.Equal(t, "bar", accessor.GetNamespace())
}
