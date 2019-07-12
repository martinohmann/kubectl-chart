package printers

import (
	"bytes"
	"testing"

	"github.com/martinohmann/kubectl-chart/pkg/recorders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"
)

func TestNamePrinter(t *testing.T) {
	obj := getJob()

	buf := bytes.NewBuffer(nil)

	assert.NoError(t, NewNamePrinter("created", false).PrintObj(obj, buf))
	assert.NoError(t, NewNamePrinter("deleted", true).PrintObj(obj, buf))

	expected := `job.batch/foo created
job.batch/foo deleted (dry run)
`

	assert.Equal(t, expected, buf.String())
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
