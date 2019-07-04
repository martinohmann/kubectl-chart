package diff

import (
	"io"

	"github.com/martinohmann/go-difflib/difflib"
)

const (
	subjectCreated = "<created>"
	subjectRemoved = "<removed>"
)

// UnifiedPrinter printes unified diffs.
type UnifiedPrinter struct {
	Options Options
}

// NewPrinter creates a new unified diff printer with diff options.
func NewUnifiedPrinter(o Options) UnifiedPrinter {
	return UnifiedPrinter{
		Options: o,
	}
}

// Print implements Printer.
func (p UnifiedPrinter) Print(s Subject, w io.Writer) error {
	if s.FromFile == "" {
		s.FromFile = subjectCreated
	}

	if s.ToFile == "" {
		s.ToFile = subjectRemoved
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(s.A),
		B:        difflib.SplitLines(s.B),
		FromFile: s.FromFile,
		ToFile:   s.ToFile,
		Context:  p.Options.Context,
		Color:    p.Options.Color,
	}

	return difflib.WriteUnifiedDiff(w, diff)
}
