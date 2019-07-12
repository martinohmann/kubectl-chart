package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
)

func TestToObjectList(t *testing.T) {
	given := []*resource.Info{
		{
			Object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name": "foo",
					},
				},
			},
		}, {
			Object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "bar",
					},
				},
			},
		},
	}

	expected := []runtime.Object{given[0].Object, given[1].Object}

	assert.Equal(t, expected, ToObjectList(given))
}
