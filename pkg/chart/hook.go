package chart

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	AnnotationHook = "kubectl-chart.io/hook"

	PreApplyHook   = "pre-apply"
	PostApplyHook  = "post-apply"
	PreDeleteHook  = "pre-delete"
	PostDeleteHook = "post-delete"
)

var Hooks = []string{PreApplyHook, PostApplyHook, PreDeleteHook, PostDeleteHook}

func IsValidHookType(typ string) bool {
	for _, t := range Hooks {
		if t == typ {
			return true
		}
	}

	return false
}

func IsHook(obj runtime.Object) (bool, error) {
	value, found, err := getHookAnnotation(obj)
	if err != nil {
		return false, err
	}

	if !found {
		return false, nil
	}

	if IsValidHookType(value) {
		return true, nil
	}

	return false, HookTypeError{Type: value}
}

func FilterHooks(typ string, hooks ...runtime.Object) ([]runtime.Object, error) {
	if !IsValidHookType(typ) {
		return nil, HookTypeError{Type: typ}
	}

	filtered := make([]runtime.Object, 0)

	for _, obj := range hooks {
		value, found, err := getHookAnnotation(obj)
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
		strings.Join(Hooks, `", "`),
	)

	if len(e.Additional) > 0 {
		msg = fmt.Sprintf("%s, \"%s\"", msg, strings.Join(e.Additional, `", "`))
	}

	return msg
}

func getHookAnnotation(obj runtime.Object) (string, bool, error) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return "", false, errors.Errorf("illegal object type: %T", obj)
	}

	return unstructured.NestedString(u.Object, "metadata", "annotations", AnnotationHook)
}
