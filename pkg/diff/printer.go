package diff

import (
	"io"

	"github.com/martinohmann/go-difflib/difflib"
)

const (
	subjectCreated = "<created>"
	subjectRemoved = "<removed>"
)

// Printer is a diff printer.
type Printer struct {
	Options Options
}

// NewPrinter creates a new diff printer with diff options.
func NewPrinter(o Options) Printer {
	return Printer{
		Options: o,
	}
}

// Subject holds the contents and filenames to compare. FromFile and ToFile are
// just used in the diff header to provide some context on what is being
// diffed.
type Subject struct {
	A, B             string
	FromFile, ToFile string
}

// Print takes a Subject s and prints a diff of it to an io.Writer.
func (p Printer) Print(s Subject, w io.Writer) error {
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
