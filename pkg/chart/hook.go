package chart

import (
	"fmt"
	"strings"
	"time"

	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	PreApplyHook   = "pre-apply"
	PostApplyHook  = "post-apply"
	PreDeleteHook  = "pre-delete"
	PostDeleteHook = "post-delete"
)

var (
	DefaultHookWaitTimeout = 2 * time.Hour

	ValidHooks             = []string{PreApplyHook, PostApplyHook, PreDeleteHook, PostDeleteHook}
	ValidHookResourceKinds = []string{resources.KindJob}
)

type Hook struct {
	*Resource
}

func NewHook(obj runtime.Object) *Hook {
	return &Hook{
		Resource: NewResource(obj),
	}
}

func (h *Hook) RestartPolicy() string {
	return h.nestedString("spec", "template", "spec", "restartPolicy")
}

func (h *Hook) Type() string {
	return h.nestedString("metadata", "annotations", AnnotationHookType)
}

func (h *Hook) WaitTimeout() (time.Duration, error) {
	value, found, _ := unstructured.NestedString(h.Object, "metadata", "annotations", AnnotationHookWaitTimeout)
	if !found {
		return 0, nil
	}

	timeout, err := time.ParseDuration(value)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse annotation %q: %s", AnnotationHookWaitTimeout, timeout)
	}

	return timeout, nil
}

type HookMap map[string]HookList

func (m HookMap) GetObjects() []runtime.Object {
	objs := make([]runtime.Object, 0)
	for _, s := range m {
		objs = append(objs, s.GetObjects()...)
	}

	return objs
}

func (m HookMap) Type(hookType string) HookList {
	return m[hookType]
}

type HookList []*Hook

func (l HookList) GetObjects() []runtime.Object {
	objs := make([]runtime.Object, len(l))
	for i := range l {
		objs[i] = l[i].GetObject()
	}

	return objs
}

func (l HookList) EachItem(fn func(*Hook) error) error {
	for _, h := range l {
		err := fn(h)
		if err != nil {
			return err
		}
	}

	return nil
}

func IsValidHookType(typ string) bool {
	for _, t := range ValidHooks {
		if t == typ {
			return true
		}
	}

	return false
}

func IsValidHookResourceKind(kind string) bool {
	for _, k := range ValidHookResourceKinds {
		if k == kind {
			return true
		}
	}

	return false
}

// HasHookAnnotation returns true if obj is annotated to be a hook.
func HasHookAnnotation(obj runtime.Object) (bool, error) {
	_, found, err := resources.GetAnnotation(obj, AnnotationHookType)
	if err != nil {
		return false, err
	}

	return found, nil
}

type HookTypeError struct {
	Type       string
	Additional []string
}

func (e HookTypeError) Error() string {
	msg := fmt.Sprintf(
		"invalid hook type %q, allowed values are \"%s\"",
		e.Type,
		strings.Join(ValidHooks, `", "`),
	)

	if len(e.Additional) > 0 {
		msg = fmt.Sprintf("%s, \"%s\"", msg, strings.Join(e.Additional, `", "`))
	}

	return msg
}

type HookResourceKindError struct {
	Kind string
}

func (e HookResourceKindError) Error() string {
	return fmt.Sprintf(
		"invalid hook resource kind %q, allowed values are \"%s\"",
		e.Kind,
		strings.Join(ValidHookResourceKinds, `", "`),
	)
}

// ValidateHook returns an error if a hook has an unsupported type or resource
// kind or if other resource fields have unsupported values.
func ValidateHook(h *Hook) error {
	if !IsValidHookType(h.Type()) {
		return HookTypeError{Type: h.Type()}
	}

	if !IsValidHookResourceKind(h.GetKind()) {
		return HookResourceKindError{Kind: h.GetKind()}
	}

	if h.RestartPolicy() != "Never" {
		return errors.Errorf("invalid hook %q: restartPolicy of the pod template must be %q", h.GetName(), "Never")
	}

	return nil
}
