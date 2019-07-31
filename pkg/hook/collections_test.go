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
	return &Hook{newUnstructured(name, hookType)}
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

	m.Add(newTestHook("foo", PreApply))
	m.Add(newTestHook("bar", PostApply))
	m.Add(newTestHook("baz", PostApply))

	assert.Len(t, m, 2)
	assert.Len(t, m[PreDelete], 0)
	assert.Len(t, m[PreApply], 1)
	assert.Len(t, m[PostApply], 2)

	assert.Equal(
		t,
		List{
			newTestHook("bar", PostApply),
			newTestHook("baz", PostApply),
		},
		m[PostApply],
	)

	assert.ElementsMatch(
		t,
		[]runtime.Object{
			newUnstructured("foo", PreApply),
			newUnstructured("bar", PostApply),
			newUnstructured("baz", PostApply),
		},
		m.All().ToObjectList(),
	)
}

func TestList_EachItem(t *testing.T) {
	l := List{
		newTestHook("bar", PostApply),
		newTestHook("baz", PostApply),
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
