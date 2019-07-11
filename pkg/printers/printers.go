package printers

import (
	"bytes"
	"fmt"
	"io"

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

type OperationPrinter interface {
	ResourcePrinter
	WithOperation(operation string) OperationPrinter
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

type colorFunc func(format string, a ...interface{}) string

var (
	colorMap = map[string]colorFunc{
		"created":    color.GreenString,
		"deleted":    color.RedString,
		"pruned":     color.RedString,
		"triggered":  color.CyanString,
		"configured": color.YellowString,
	}

	operationPrefix = map[string]string{
		"created":    "+",
		"deleted":    "-",
		"pruned":     "-",
		"triggered":  "^",
		"configured": "~",
	}
)

type colorPrinter struct {
	delegate ResourcePrinter
	colorFn  colorFunc
	prefix   string
}

// NewColorPrinter wraps delegate with a color printer for given operation.
func NewColorPrinter(delegate ResourcePrinter, operation string) ResourcePrinter {
	colorFn, ok := colorMap[operation]
	if !ok {
		colorFn = fmt.Sprintf
	}

	prefix, ok := operationPrefix[operation]
	if !ok {
		prefix = " "
	}

	return &colorPrinter{
		delegate: delegate,
		colorFn:  colorFn,
		prefix:   prefix,
	}
}

// PrintObj implements ResourcePrinter
func (p *colorPrinter) PrintObj(obj runtime.Object, w io.Writer) error {
	var buf bytes.Buffer

	err := p.delegate.PrintObj(obj, &buf)
	if err != nil {
		return err
	}

	colored := p.colorFn("%s %s", p.prefix, buf.String())

	_, err = w.Write([]byte(colored))

	return err
}

var _ OperationPrinter = &operationPrinter{}
var _ OperationPrinter = &discardingOperationPrinter{}

type operationPrinter struct {
	ResourcePrinter
	operation string
	color     bool
	dryRun    bool
}

func NewOperationPrinter(color, dryRun bool) OperationPrinter {
	return newOperationPrinter("", color, dryRun)
}

func newOperationPrinter(operation string, color, dryRun bool) OperationPrinter {
	var p ResourcePrinter

	p = NewNamePrinter(operation, dryRun)

	if color {
		p = NewColorPrinter(p, operation)
	}

	return &operationPrinter{
		ResourcePrinter: p,
		operation:       operation,
		color:           color,
		dryRun:          dryRun,
	}
}

func (p *operationPrinter) WithOperation(operation string) OperationPrinter {
	return newOperationPrinter(operation, p.color, p.dryRun)
}

type discardingOperationPrinter struct{}

func NewDiscardingOperationPrinter() OperationPrinter {
	return &discardingOperationPrinter{}
}

func (p *discardingOperationPrinter) WithOperation(string) OperationPrinter {
	return p
}

func (p *discardingOperationPrinter) PrintObj(runtime.Object, io.Writer) error {
	return nil
}
