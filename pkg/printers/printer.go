package printers

import (
	"io"

	"github.com/martinohmann/kubectl-chart/pkg/recorders"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/printers"
)

type RecordingPrinter struct {
	Recorder  recorders.OperationRecorder
	Operation string
	Printer   printers.ResourcePrinter
}

func NewRecordingPrinter(r recorders.OperationRecorder, operation string, p printers.ResourcePrinter) *RecordingPrinter {
	return &RecordingPrinter{
		Recorder:  r,
		Operation: operation,
		Printer:   p,
	}
}

func (p *RecordingPrinter) PrintObj(obj runtime.Object, w io.Writer) error {
	err := p.Recorder.Record(p.Operation, obj)
	if err != nil {
		return err
	}

	return p.Printer.PrintObj(obj, w)
}
