package chart

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testVisitor struct {
	sync.Mutex
	seenResources map[string]int
	seenHooks     map[string]int
}

func (v *testVisitor) Handle(c *Chart, err error) error {
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

	v.seenResources[c.Config.Name] = len(c.Resources.GetObjects())
	v.seenHooks[c.Config.Name] = len(c.Hooks.GetObjects())

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

	assert.Equal(t, 2, tv.seenResources["chart1"])
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

	assert.Equal(t, 2, tv.seenResources["chart1"])
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

func TestReverseVisitor_Visit(t *testing.T) {
	opts := VisitorOptions{
		ChartDir:  "testdata",
		Namespace: "default",
		Recursive: true,
	}

	v := NewReverseVisitor(NewVisitor(NewDefaultProcessor(), opts))

	seenCharts := make([]string, 0)

	err := v.Visit(func(c *Chart, err error) error {
		require.NoError(t, err)

		seenCharts = append(seenCharts, c.Config.Name)

		return nil
	})

	require.NoError(t, err)

	assert.Equal(t, []string{"chart2", "chart1"}, seenCharts)
}
