package printers

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
	"github.com/martinohmann/kubectl-chart/pkg/recorders"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
)

// ResourcePrinter prints runtime objects.
type ResourcePrinter interface {
	// Print receives a runtime object, formats it and prints it to a writer.
	PrintObj(obj runtime.Object, w io.Writer) error
}

// ContextPrinter is a ResourcePrinter that can add context to printed object
// information.
type ContextPrinter interface {
	ResourcePrinter

	// WithOperation returns a new ContextPrinter for operation. The receiver
	// must not be mutated.
	WithOperation(operation string) ContextPrinter

	// WithContext returns a new ContextPrinter with the context values set.
	// The receiver must not be mutated.
	WithContext(context ...string) ContextPrinter
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

type colorFunc func(format string, a ...interface{}) string

var colorMap = map[string]colorFunc{
	"completed":  color.GreenString,
	"configured": color.YellowString,
	"created":    color.GreenString,
	"deleted":    color.RedString,
	"pruned":     color.RedString,
	"triggered":  color.CyanString,
}

type colorPrinter struct {
	delegate ResourcePrinter
	colorFn  colorFunc
}

// NewColorPrinter wraps delegate with a color printer for given operation.
func NewColorPrinter(delegate ResourcePrinter, operation string) ResourcePrinter {
	colorFn := colorMap[operation]
	if colorFn == nil {
		colorFn = fmt.Sprintf
	}

	return &colorPrinter{
		delegate: delegate,
		colorFn:  colorFn,
	}
}

// PrintObj implements ResourcePrinter.
func (p *colorPrinter) PrintObj(obj runtime.Object, w io.Writer) error {
	var buf bytes.Buffer

	err := p.delegate.PrintObj(obj, &buf)
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(p.colorFn(buf.String())))

	return err
}

type contextPrinter struct {
	ResourcePrinter
	operation string
	context   []string
	color     bool
	dryRun    bool
}

// NewContextPrinter creates a new ContextPrinter.
func NewContextPrinter(color, dryRun bool) ContextPrinter {
	return newContextPrinter("", nil, color, dryRun)
}

func newContextPrinter(operation string, context []string, color, dryRun bool) ContextPrinter {
	var p ResourcePrinter

	operationInfo := operation

	if len(context) > 0 {
		operationInfo = fmt.Sprintf("%s (%s)", operation, strings.Join(context, ","))
	}

	if dryRun {
		operationInfo = fmt.Sprintf("%s (dry run)", operationInfo)
	}

	p = &printers.NamePrinter{
		Operation: strings.TrimSpace(operationInfo),
	}

	if color {
		p = NewColorPrinter(p, operation)
	}

	return &contextPrinter{
		ResourcePrinter: p,
		operation:       operation,
		context:         context,
		color:           color,
		dryRun:          dryRun,
	}
}

// WithOperation implements WithOperation from the ContextPrinter interface.
func (p contextPrinter) WithOperation(operation string) ContextPrinter {
	return newContextPrinter(operation, p.context, p.color, p.dryRun)
}

// WithContext implements WithContext from the ContextPrinter interface.
func (p contextPrinter) WithContext(context ...string) ContextPrinter {
	return newContextPrinter(p.operation, context, p.color, p.dryRun)
}

type discardingContextPrinter struct {
	ResourcePrinter
}

// NewDiscardingContextPrinter creates a ContextPrinter that just discards
// everything.
func NewDiscardingContextPrinter() ContextPrinter {
	return &discardingContextPrinter{printers.NewDiscardingPrinter()}
}

// WithOperation implements WithOperation from the ContextPrinter interface.
func (p discardingContextPrinter) WithOperation(string) ContextPrinter {
	return p
}

// WithContext implements WithContext from the ContextPrinter interface.
func (p discardingContextPrinter) WithContext(...string) ContextPrinter {
	return p
}
