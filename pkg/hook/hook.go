package hook

import (
	"strconv"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// Supported types of hooks.
const (
	PreApply   = "pre-apply"
	PostApply  = "post-apply"
	PreDelete  = "pre-delete"
	PostDelete = "post-delete"
)

// SupportedTypes contains a list of supported hook types.
var SupportedTypes = []string{PreApply, PostApply, PreDelete, PostDelete}

// Hook gets executed before or after apply/delete depending on its type. It
// wraps an unstructured object to enhance it with some extra logic for
// validation and working with hook options.
type Hook struct {
	*unstructured.Unstructured
}

// New creates a new hook from a runtime object.
func New(obj runtime.Object) (*Hook, error) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, errors.Errorf("obj is of type %T, expected *unstructured.Unstructured", obj)
	}

	h := &Hook{u}

	err := Validate(h)
	if err != nil {
		return nil, err
	}

	return h, nil
}

func (h *Hook) nestedString(fields ...string) string {
	value, _, _ := unstructured.NestedString(h.Object, fields...)
	return value
}

// RestartPolicy gets the restartPolicy from the hook jobs pod spec.
func (h *Hook) RestartPolicy() string {
	return h.nestedString("spec", "template", "spec", "restartPolicy")
}

// Type returns the hook type, e.g. post-apply.
func (h *Hook) Type() string {
	return h.nestedString("metadata", "annotations", AnnotationHookType)
}

// AllowFailure indicates whether the hook is allowed to fail or not. If true,
// hook errors are only printed and execution continues. If this is true,
// NoWait() must not return true as well.
func (h *Hook) AllowFailure() bool {
	value := h.nestedString("metadata", "annotations", AnnotationHookAllowFailure)
	b, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}

	return b
}

// NoWait indicates whether it should be waited for the hook to complete or
// not. If true, AllowFailure() must not return true and it is also not
// possible to detect possible hook failures.
func (h *Hook) NoWait() bool {
	value := h.nestedString("metadata", "annotations", AnnotationHookNoWait)
	b, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}

	return b
}

// WaitTimeout returns the timeout for waiting for hook completion and an error
// if the annotation cannot be parsed.
func (h *Hook) WaitTimeout() (time.Duration, error) {
	value, found, _ := unstructured.NestedString(h.Object, "metadata", "annotations", AnnotationHookWaitTimeout)
	if !found {
		return 0, nil
	}

	timeout, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}

	return timeout, nil
}

// IsSupportedType returns true if typ is a supported hook type.
func IsSupportedType(typ string) bool {
	for _, t := range SupportedTypes {
		if t == typ {
			return true
		}
	}

	return false
}

// Validate returns an error if a hook has an unsupported type or resource kind
// or if other resource fields have unsupported values.
func Validate(h *Hook) error {
	if h.GetKind() != jobGK.Kind {
		return UnsupportedKindError{Kind: h.GetKind()}
	}

	if !IsSupportedType(h.Type()) {
		return UnsupportedTypeError{Type: h.Type()}
	}

	if h.RestartPolicy() != "Never" {
		return errors.Errorf("invalid hook %q: restartPolicy of the pod template must be %q", h.GetName(), "Never")
	}

	if h.NoWait() && h.AllowFailure() {
		return errors.Errorf("invalid hook %q: %s and %s cannot be true at the same time", h.GetName(), AnnotationHookNoWait, AnnotationHookAllowFailure)
	}

	timeout, err := h.WaitTimeout()
	if err != nil {
		return errors.Wrapf(err, "invalid hook %q: malformed %s annotation", h.GetName(), AnnotationHookWaitTimeout)
	}

	if h.NoWait() && timeout > 0 {
		return errors.Errorf("invalid hook %q: %s and %s cannot be set at the same time", h.GetName(), AnnotationHookNoWait, AnnotationHookWaitTimeout)
	}

	return err
}
