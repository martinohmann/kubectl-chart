package chart

import (
	"testing"

	"github.com/martinohmann/kubectl-chart/pkg/deletions"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
)

func TestPVCPruner_Prune(t *testing.T) {
	deleter := deletions.NewFakeDeleter(nil)

	objs := []runtime.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "StatefulSet",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						AnnotationDeletionPolicy: DeletionPolicyDeletePVCs,
					},
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
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "StatefulSet",
				"metadata": map[string]interface{}{
					"name": "baz",
				},
			},
		},
	}

	body := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "PersistentVolumeClaim",
			"metadata": map[string]interface{}{
				"name": "qux",
			},
		},
	}

	pruner := &PVCPruner{
		BuilderFactory: func() *resource.Builder {
			return newDefaultBuilderWith(fakeClientWith("", t, map[string]string{
				"/persistentvolumeclaims?labelSelector=kubectl-chart%2Fowned-by-statefulset%3Dfoo": runtime.EncodeOrDie(unstructured.UnstructuredJSONScheme, body),
			}))
		},
		Deleter: deleter,
	}

	err := pruner.Prune(resources.NewVisitor(objs))

	require.NoError(t, err)
	require.Len(t, deleter.CalledWith, 1)

	infos, _ := deleter.CalledWith[0].Visitor.(*resource.Result).Infos()

	require.Len(t, infos, 1)

	pvc, ok := infos[0].Object.(*v1.PersistentVolumeClaim)

	require.True(t, ok)
	assert.Equal(t, "qux", pvc.GetName())
}
