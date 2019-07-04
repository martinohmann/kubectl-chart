package printers

import (
	"io"

	"github.com/martinohmann/kubectl-chart/pkg/recorders"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/printers"
)

// RecordingPrinter records all objects it prints.
type RecordingPrinter struct {
	Recorder  recorders.OperationRecorder
	Operation string
	Printer   printers.ResourcePrinter
}

// NewRecordingPrinter creates a new *RecordingPrinter that records all objects
// using r and afterwards prints them with p. The returned printer must only
// print objects for the given operation.
func NewRecordingPrinter(r recorders.OperationRecorder, operation string, p printers.ResourcePrinter) *RecordingPrinter {
	return &RecordingPrinter{
		Recorder:  r,
		Operation: operation,
		Printer:   p,
	}
}

// PrintObj prints obj and writes the result to w.
func (p *RecordingPrinter) PrintObj(obj runtime.Object, w io.Writer) error {
	err := p.Recorder.Record(p.Operation, obj)
	if err != nil {
		return err
	}

	return p.Printer.PrintObj(obj, w)
}
