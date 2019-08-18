package hook

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Supported types of hooks.
const (
	TypePostApply  = "post-apply"
	TypePostDelete = "post-delete"
	TypePreApply   = "pre-apply"
	TypePreDelete  = "pre-delete"
)

// SupportedTypes contains all supported hook types.
var SupportedTypes = sets.NewString(TypePostApply, TypePostDelete, TypePreApply, TypePreDelete)

// Hook gets executed before or after apply/delete depending on its type..
type Hook struct {
	*unstructured.Unstructured

	// Type contains the hook type, e.g. post-apply.
	Type string

	// AllowFailure indicates whether the hook is allowed to fail or not. If true,
	// hook errors are only printed and execution continues. If this is true,
	// NoWait must not be true as well.
	AllowFailure bool

	// NoWait indicates whether it should be waited for the hook to complete or
	// not. If true, AllowFailure must not be true and it is also not possible
	// to detect possible hook failures.
	NoWait bool

	// WaitTimeout sets a custom hook wait timeout. If zero, a default wait
	// timeout will be used. Must be zero if NoWait is set to true.
	WaitTimeout time.Duration
}
