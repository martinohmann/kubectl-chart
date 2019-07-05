package wait

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic/fake"
)

type conditionTester struct {
	seenInfos int
}

func (c *conditionTester) IsConditionMet(info *resource.Info, w *Waiter, o Options, uidMap UIDMap) (runtime.Object, bool, error) {
	c.seenInfos++

	return info.Object, true, nil
}

func TestWaiter_Wait(t *testing.T) {
	streams := genericclioptions.NewTestIOStreamsDiscard()
	client := fake.NewSimpleDynamicClient(runtime.NewScheme())

	tester := &conditionTester{}

	w := NewDefaultWaiter(streams, client)

	infos := getTestInfos()

	req := &Request{
		ConditionFn: tester.IsConditionMet,
		Visitor:     resource.InfoListVisitor(infos),
		ResourceOptions: map[types.UID]Options{
			"abcd": Options{Timeout: 5 * time.Second},
		},
	}

	err := w.Wait(req)

	require.NoError(t, err)

	assert.Equal(t, 3, tester.seenInfos)
}

func getTestInfos() []*resource.Info {
	return []*resource.Info{
		&resource.Info{
			Name:      "foo",
			Namespace: "bar",
			Object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name":      "foo",
						"namespace": "bar",
					},
				},
			},
		},
		&resource.Info{
			Name:      "baz",
			Namespace: "baz",
			Object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name":      "baz",
						"namespace": "baz",
						"uid":       "abcd",
					},
				},
			},
		},
		&resource.Info{
			Name:      "bar",
			Namespace: "bar",
			Object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name":      "bar",
						"namespace": "bar",
					},
				},
			},
		},
	}
}
