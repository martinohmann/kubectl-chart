package hook

import (
	"fmt"
	"strings"
)

// UnsupportedTypeError denotes that a hook has a type that is not supported.
type UnsupportedTypeError struct {
	Type string
}

// Error implements the error interface.
func (e UnsupportedTypeError) Error() string {
	return fmt.Sprintf(
		"unsupported hook type %q, allowed values are \"%s\"",
		e.Type,
		strings.Join(SupportedTypes, `", "`),
	)
}

// UnsupportedKindError denotes that the hook object has a resource kind that
// is not supported to be used as a hook.
type UnsupportedKindError struct {
	Kind string
}

// Error implements the error interface.
func (e UnsupportedKindError) Error() string {
	return fmt.Sprintf(
		"unsupported hook resource kind %q, only %q is allowed",
		e.Kind,
		jobGK.Kind,
	)
}
