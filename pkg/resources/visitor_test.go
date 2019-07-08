package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestStatefulSetVisitor_Visit(t *testing.T) {
	objs := []runtime.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "StatefulSet",
				"metadata": map[string]interface{}{
					"name": "foo",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name": "bar",
				},
			},
		},
	}

	v := NewStatefulSetVisitor(NewVisitor(objs))

	collectedObjs := make([]runtime.Object, 0)

	err := v.Visit(func(obj runtime.Object, err error) error {
		if err != nil {
			return err
		}

		collectedObjs = append(collectedObjs, obj)

		return nil
	})

	require.NoError(t, err)

	assert.Len(t, collectedObjs, 1)
	assert.Equal(t, "StatefulSet", collectedObjs[0].GetObjectKind().GroupVersionKind().Kind)
}
