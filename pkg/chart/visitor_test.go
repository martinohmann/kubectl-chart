package chart

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

type testVisitor struct {
	sync.Mutex
	seenResources map[string]int
	seenHooks     map[string]int
}

func (v *testVisitor) Handle(config *Config, resources, hooks []runtime.Object, err error) error {
	if err != nil {
		return err
	}

	v.Lock()
	defer v.Unlock()

	if v.seenResources == nil {
		v.seenResources = make(map[string]int)
	}

	if v.seenHooks == nil {
		v.seenHooks = make(map[string]int)
	}

	v.seenResources[config.Name] = len(resources)
	v.seenHooks[config.Name] = len(hooks)

	return nil
}

func TestVisitor_Visit(t *testing.T) {
	opts := VisitorOptions{
		ChartDir:  "testdata/chart1",
		Namespace: "default",
	}

	v := NewVisitor(NewDefaultProcessor(), opts)
	tv := &testVisitor{}

	err := v.Visit(tv.Handle)

	require.NoError(t, err)

	assert.Equal(t, 1, tv.seenResources["chart1"])
	assert.Equal(t, 1, tv.seenHooks["chart1"])
}

func TestVisitor_VisitRecursive(t *testing.T) {
	opts := VisitorOptions{
		ChartDir:  "testdata",
		Namespace: "default",
		Recursive: true,
	}

	v := NewVisitor(NewDefaultProcessor(), opts)
	tv := &testVisitor{}

	err := v.Visit(tv.Handle)

	require.NoError(t, err)

	assert.Equal(t, 1, tv.seenResources["chart1"])
	assert.Equal(t, 1, tv.seenHooks["chart1"])

	assert.Equal(t, 1, tv.seenResources["chart2"])
	assert.Equal(t, 0, tv.seenHooks["chart2"])
}

func TestVisitor_VisitChartFilter(t *testing.T) {
	opts := VisitorOptions{
		ChartDir:    "testdata",
		Namespace:   "default",
		Recursive:   true,
		ChartFilter: []string{"chart2"},
	}

	v := NewVisitor(NewDefaultProcessor(), opts)
	tv := &testVisitor{}

	err := v.Visit(tv.Handle)

	require.NoError(t, err)

	assert.Equal(t, 0, tv.seenResources["chart1"])
	assert.Equal(t, 0, tv.seenHooks["chart1"])

	assert.Equal(t, 1, tv.seenResources["chart2"])
	assert.Equal(t, 0, tv.seenHooks["chart2"])
}
