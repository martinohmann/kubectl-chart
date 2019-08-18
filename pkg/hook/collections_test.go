package hook

import (
	"errors"
	"testing"

	"github.com/martinohmann/kubectl-chart/pkg/meta"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func newTestHook(name, hookType string) *Hook {
	return MustParse(newUnstructured(name, hookType))
}

func newUnstructured(name, hookType string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "batch/v1",
			"kind":       "Job",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": "bar",
				"annotations": map[string]interface{}{
					meta.AnnotationHookType: hookType,
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
	}
}

func TestMap(t *testing.T) {
	m := Map{}

	m.Add(newTestHook("foo", TypePreApply))
	m.Add(newTestHook("bar", TypePostApply))
	m.Add(newTestHook("baz", TypePostApply))

	assert.Len(t, m, 2)
	assert.Len(t, m[TypePreDelete], 0)
	assert.Len(t, m[TypePreApply], 1)
	assert.Len(t, m[TypePostApply], 2)

	assert.Equal(
		t,
		List{
			newTestHook("bar", TypePostApply),
			newTestHook("baz", TypePostApply),
		},
		m[TypePostApply],
	)

	assert.ElementsMatch(
		t,
		[]runtime.Object{
			newUnstructured("foo", TypePreApply),
			newUnstructured("bar", TypePostApply),
			newUnstructured("baz", TypePostApply),
		},
		m.All().ToObjectList(),
	)
}

func TestList_EachItem(t *testing.T) {
	l := List{
		newTestHook("bar", TypePostApply),
		newTestHook("baz", TypePostApply),
	}

	names := []string{}

	err := l.EachItem(func(h *Hook) error {
		names = append(names, h.GetName())

		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, []string{"bar", "baz"}, names)

	err = l.EachItem(func(h *Hook) error {
		return errors.New("whoops")
	})

	assert.Error(t, err)
	assert.Equal(t, "whoops", err.Error())
}
