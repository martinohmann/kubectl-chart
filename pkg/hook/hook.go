package hook

import (
	"fmt"
	"strconv"
	"time"

	"github.com/martinohmann/kubectl-chart/pkg/meta"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// jobGK is the GroupKind that a hook resource must have.
var jobGK = schema.GroupKind{Group: "batch", Kind: "Job"}

// LabelSelector returns a selector which can be used to find hooks of given
// type for given chart in a cluster.
func LabelSelector(chartName, hookType string) string {
	return fmt.Sprintf("%s=%s,%s=%s", meta.LabelHookChartName, chartName, meta.LabelHookType, hookType)
}

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
		return nil, errors.Wrapf(err, "invalid hook %q", h.GetName())
	}

	return h, nil
}

func (h *Hook) nestedString(fields ...string) string {
	value, _, _ := unstructured.NestedString(h.Object, fields...)
	return value
}

// restartPolicy gets the restartPolicy from the hook jobs pod spec.
func (h *Hook) restartPolicy() corev1.RestartPolicy {
	return corev1.RestartPolicy(h.nestedString("spec", "template", "spec", "restartPolicy"))
}

// Type returns the hook type, e.g. post-apply.
func (h *Hook) Type() string {
	return h.nestedString("metadata", "annotations", meta.AnnotationHookType)
}

// AllowFailure indicates whether the hook is allowed to fail or not. If true,
// hook errors are only printed and execution continues. If this is true,
// NoWait() must not return true as well.
func (h *Hook) AllowFailure() bool {
	value := h.nestedString("metadata", "annotations", meta.AnnotationHookAllowFailure)
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
	value := h.nestedString("metadata", "annotations", meta.AnnotationHookNoWait)
	b, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}

	return b
}

// WaitTimeout returns the timeout for waiting for hook completion and an error
// if the annotation cannot be parsed.
func (h *Hook) WaitTimeout() (time.Duration, error) {
	value, found, _ := unstructured.NestedString(h.Object, "metadata", "annotations", meta.AnnotationHookWaitTimeout)
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
		return NewUnsupportedKindError(h.GetKind())
	}

	if !IsSupportedType(h.Type()) {
		return NewUnsupportedTypeError(h.Type())
	}

	if h.restartPolicy() != corev1.RestartPolicyNever {
		return NewUnsupportedRestartPolicyError(h.restartPolicy())
	}

	if h.NoWait() && h.AllowFailure() {
		return NewIllegalAnnotationCombinationError(meta.AnnotationHookNoWait, meta.AnnotationHookAllowFailure)
	}

	timeout, err := h.WaitTimeout()
	if err != nil {
		return errors.Wrapf(err, "malformed annotation %q", meta.AnnotationHookWaitTimeout)
	}

	if h.NoWait() && timeout > 0 {
		return NewIllegalAnnotationCombinationError(meta.AnnotationHookNoWait, meta.AnnotationHookWaitTimeout)
	}

	return err
}
