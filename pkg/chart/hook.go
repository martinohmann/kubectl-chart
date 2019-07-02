package chart

import (
	"fmt"
	"strings"
	"time"

	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	AnnotationHook            = "kubectl-chart/hook"
	AnnotationHookWaitTimeout = "kubectl-chart/hook-wait-timeout"

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

func IsHook(obj runtime.Object) (bool, error) {
	value, found, err := resources.GetAnnotation(obj, AnnotationHook)
	if err != nil {
		return false, err
	}

	if !found {
		return false, nil
	}

	if !IsValidHookType(value) {
		return false, HookTypeError{Type: value}
	}

	gvk := obj.GetObjectKind().GroupVersionKind()

	if !IsValidHookResourceKind(gvk.Kind) {
		return false, HookResourceKindError{Kind: gvk.Kind}
	}

	return true, nil
}

func FilterHooks(typ string, hooks ...runtime.Object) ([]runtime.Object, error) {
	if !IsValidHookType(typ) {
		return nil, HookTypeError{Type: typ}
	}

	filtered := make([]runtime.Object, 0)

	for _, obj := range hooks {
		value, found, err := resources.GetAnnotation(obj, AnnotationHook)
		if err != nil {
			return nil, err
		}

		if found && value == typ {
			filtered = append(filtered, obj)
		}
	}

	return filtered, nil
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

type Hook struct {
	Object      runtime.Object
	Type        string
	WaitTimeout time.Duration
}

func ParseHook(obj runtime.Object) (*Hook, error) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, errors.Errorf("illegal object type: %T", obj)
	}

	if u.GetKind() != resources.KindJob {
		return nil, HookResourceKindError{Kind: u.GetKind()}
	}

	var job batchv1.Job
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &job)
	if err != nil {
		return nil, err
	}

	restartPolicy := job.Spec.Template.Spec.RestartPolicy

	if restartPolicy != corev1.RestartPolicyNever {
		return nil, errors.Errorf("invalid hook %q: spec.template.spec.restartPolicy must be %s", job.GetName(), corev1.RestartPolicyNever)
	}

	annotations := job.ObjectMeta.Annotations
	if annotations == nil {
		return nil, errors.Errorf("invalid hook %q: no annotations set", job.GetName())
	}

	hook := &Hook{
		Object: obj,
		Type:   annotations[AnnotationHook],
	}

	wt, ok := annotations[AnnotationHookWaitTimeout]
	if ok {
		hook.WaitTimeout, err = time.ParseDuration(wt)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse annotation %q: %s", AnnotationHookWaitTimeout, wt)
		}
	}

	return hook, nil
}

func HooksToObjects(hooks ...*Hook) []runtime.Object {
	objs := make([]runtime.Object, len(hooks))
	for i := range hooks {
		objs[i] = hooks[i].Object
	}

	return objs
}
