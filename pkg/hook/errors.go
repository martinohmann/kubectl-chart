package hook

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// UnsupportedTypeError denotes that a hook has a type that is not supported.
type UnsupportedTypeError struct {
	Type string
}

// NewUnsupportedTypeError creates a new UnsupportedTypeError for t.
func NewUnsupportedTypeError(t string) UnsupportedTypeError {
	return UnsupportedTypeError{
		Type: t,
	}
}

// Error implements the error interface.
func (e UnsupportedTypeError) Error() string {
	return fmt.Sprintf(
		"unsupported hook type %q, allowed values are: %v",
		e.Type,
		SupportedTypes,
	)
}

// UnsupportedKindError denotes that the hook object has a resource kind that
// is not supported to be used as a hook.
type UnsupportedKindError struct {
	Kind string
}

// NewUnsupportedKindError creates a new UnsupportedKindError for kind.
func NewUnsupportedKindError(kind string) UnsupportedKindError {
	return UnsupportedKindError{
		Kind: kind,
	}
}

// Error implements the error interface.
func (e UnsupportedKindError) Error() string {
	return fmt.Sprintf(
		"unsupported hook resource kind %q, only %q is allowed",
		e.Kind,
		jobGK.Kind,
	)
}

// UnsupportedRestartPolicyError denotes that the hook object has an unsupported restart
// policy set in the pod template.
type UnsupportedRestartPolicyError struct {
	RestartPolicy corev1.RestartPolicy
}

// NewUnsupportedRestartPolicyError creates a new UnsupportedRestartPolicyError for rp.
func NewUnsupportedRestartPolicyError(rp corev1.RestartPolicy) UnsupportedRestartPolicyError {
	return UnsupportedRestartPolicyError{
		RestartPolicy: rp,
	}
}

// Error implements the error interface.
func (e UnsupportedRestartPolicyError) Error() string {
	return fmt.Sprintf(
		"unsupported restartPolicy %q in the pod template, only %q is allowed",
		e.RestartPolicy,
		corev1.RestartPolicyNever,
	)
}

// IllegalAnnotationCombinationError denotes that conflicting annotations are
// used on a hook object.
type IllegalAnnotationCombinationError struct {
	Annotations []string
}

// NewIllegalAnnotationCombinationError creates a new
// IllegalAnnotationCombinationError for annotations.
func NewIllegalAnnotationCombinationError(annotations ...string) IllegalAnnotationCombinationError {
	return IllegalAnnotationCombinationError{
		Annotations: annotations,
	}
}

// Error implements the error interface.
func (e IllegalAnnotationCombinationError) Error() string {
	return fmt.Sprintf("annotations cannot be set at the same time: %v", e.Annotations)
}
