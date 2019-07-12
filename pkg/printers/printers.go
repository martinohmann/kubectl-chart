package printers

import (
	"fmt"
	"io"

	"github.com/martinohmann/kubectl-chart/pkg/recorders"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
)

// ResourcePrinter prints runtime objects.
type ResourcePrinter interface {
	// Print receives a runtime object, formats it and prints it to a writer.
	PrintObj(runtime.Object, io.Writer) error
}

// RecordingPrinter records all objects it prints.
type RecordingPrinter struct {
	Recorder  recorders.OperationRecorder
	Operation string
	Printer   ResourcePrinter
}

// NewRecordingPrinter creates a new *RecordingPrinter that records all objects
// using r and afterwards prints them with p. The returned printer must only
// print objects for the given operation.
func NewRecordingPrinter(r recorders.OperationRecorder, operation string, p ResourcePrinter) *RecordingPrinter {
	return &RecordingPrinter{
		Recorder:  r,
		Operation: operation,
		Printer:   p,
	}
}

// PrintObj implements ResourcePrinter.
func (p *RecordingPrinter) PrintObj(obj runtime.Object, w io.Writer) error {
	defer p.Recorder.Record(p.Operation, obj)

	return p.Printer.PrintObj(obj, w)
}

// NewNamePrinter creates a new name printer which annotates the printed names
// with "(dry run)" if dry run is enabled.
func NewNamePrinter(operation string, dryRun bool) *printers.NamePrinter {
	if dryRun {
		operation = fmt.Sprintf("%s (dry run)", operation)
	}

	return &printers.NamePrinter{Operation: operation}
}
