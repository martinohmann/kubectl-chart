package printers

import (
	"bytes"
	"io"
	"testing"

	"github.com/fatih/color"
	"github.com/martinohmann/kubectl-chart/pkg/recorders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"
)

func TestContextPrinter(t *testing.T) {
	// needed for making assertions about correct coloring
	noColor := color.NoColor
	defer func() { color.NoColor = noColor }()

	color.NoColor = false

	tests := []struct {
		name     string
		dryRun   bool
		color    bool
		printFn  func(t *testing.T, p ContextPrinter, w io.Writer)
		expected string
	}{
		{
			name: "simple",
			printFn: func(t *testing.T, p ContextPrinter, w io.Writer) {
				assert.NoError(t, p.PrintObj(getJob(), w))
			},
			expected: "job.batch/foo\n",
		},
		{
			name: "with operation",
			printFn: func(t *testing.T, p ContextPrinter, w io.Writer) {
				assert.NoError(t, p.WithOperation("created").PrintObj(getJob(), w))
			},
			expected: "job.batch/foo created\n",
		},
		{
			name: "with context",
			printFn: func(t *testing.T, p ContextPrinter, w io.Writer) {
				assert.NoError(t, p.WithContext("no-wait").PrintObj(getJob(), w))
			},
			expected: "job.batch/foo (no-wait)\n",
		},
		{
			name:   "dry run printer with context and operation",
			dryRun: true,
			printFn: func(t *testing.T, p ContextPrinter, w io.Writer) {
				p = p.WithContext("allow-failure", "timeout=10s").WithOperation("deleted")
				assert.NoError(t, p.PrintObj(getJob(), w))
			},
			expected: "job.batch/foo deleted (allow-failure,timeout=10s) (dry run)\n",
		},
		{
			name:  "operation with color",
			color: true,
			printFn: func(t *testing.T, p ContextPrinter, w io.Writer) {
				assert.NoError(t, p.WithOperation("created").PrintObj(getJob(), w))
			},
			expected: color.GreenString("job.batch/foo created\n"),
		},
		{
			name: "last operation takes precedence",
			printFn: func(t *testing.T, p ContextPrinter, w io.Writer) {
				p = p.WithOperation("created").WithOperation("deleted")
				assert.NoError(t, p.PrintObj(getJob(), w))
			},
			expected: "job.batch/foo deleted\n",
		},
		{
			name: "context does not accumulate",
			printFn: func(t *testing.T, p ContextPrinter, w io.Writer) {
				p = p.WithContext("bar=bar").WithContext("foo=bar", "baz")
				assert.NoError(t, p.PrintObj(getJob(), w))
			},
			expected: "job.batch/foo (foo=bar,baz)\n",
		},
		{
			name:  "unknown operations are not colored",
			color: true,
			printFn: func(t *testing.T, p ContextPrinter, w io.Writer) {
				assert.NoError(t, p.WithOperation("prettified").PrintObj(getJob(), w))
			},
			expected: "job.batch/foo prettified\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := NewContextPrinter(test.color, test.dryRun)
			buf := bytes.NewBuffer(nil)

			test.printFn(t, p, buf)

			assert.Equal(t, test.expected, buf.String())
		})
	}
}

func TestRecordingPrinter(t *testing.T) {
	obj := getJob()
	rec := recorders.NewOperationRecorder()

	buf := bytes.NewBuffer(nil)

	p := NewRecordingPrinter(rec, "created", printers.NewDiscardingPrinter())

	assert.NoError(t, p.PrintObj(obj, buf))

	recordedObjs := rec.RecordedObjects("created")

	require.Len(t, recordedObjs, 1)
	assert.Equal(t, obj, recordedObjs[0])
}

func getJob() *v1.Job {
	return &v1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
	}
}

func TestDiscardingContextPrinter(t *testing.T) {
	var buf bytes.Buffer

	p := NewDiscardingContextPrinter()

	p.WithContext("foo").WithOperation("bar").PrintObj(getJob(), &buf)

	assert.Empty(t, buf.String())
}
