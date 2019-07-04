package diff

import "io"

// Options control the behaviour and output of the Differ.
type Options struct {
	// Context is the number of unchanged lines before and after each changed
	// block that should also be printed to give some more context on the
	// change.
	Context int

	// Color enables colored diff output.
	Color bool
}

// Subject holds the contents and filenames to compare. FromFile and ToFile are
// just used in the diff header to provide some context on what is being
// diffed.
type Subject struct {
	A, B             string
	FromFile, ToFile string
}

// DefaultOptions are the default options that are used if none are provided.
var DefaultOptions = Options{Context: 10, Color: true}

// Differ is the interface for a Differ.
type Differ interface {
	// Print uses p to print a diff to w.
	Print(p Printer, w io.Writer) error
}

// Printer is the interface for a diff printer.
type Printer interface {
	// Print prints a diff for subject s to w.
	Print(s Subject, w io.Writer) error
}
