package hook

import (
	"strconv"
	"time"

	"github.com/martinohmann/kubectl-chart/pkg/meta"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// jobGK is the GroupKind that a hook resource must have.
var jobGK = schema.GroupKind{Group: "batch", Kind: "Job"}

// MustParse wraps Parse() and panics if parsing fails.
func MustParse(obj runtime.Object) *Hook {
	h, err := Parse(obj)
	if err != nil {
		panic(err)
	}

	return h
}

// Parse parses a runtime.Object into a *Hook. Returns errors if parsing fails.
func Parse(obj runtime.Object) (*Hook, error) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, errors.Errorf("obj is of type %T, expected *unstructured.Unstructured", obj)
	}

	if u.GetKind() != jobGK.Kind {
		return nil, NewUnsupportedKindError(u.GetKind())
	}

	annotations := u.GetAnnotations()

	hookType := annotations[meta.AnnotationHookType]
	if !SupportedTypes.Has(hookType) {
		return nil, NewUnsupportedTypeError(hookType)
	}

	allowFailure := parseBool(annotations[meta.AnnotationHookAllowFailure])
	noWait := parseBool(annotations[meta.AnnotationHookNoWait])

	if noWait && allowFailure {
		return nil, NewIllegalAnnotationCombinationError(meta.AnnotationHookNoWait, meta.AnnotationHookAllowFailure)
	}

	waitTimeout, err := parseDuration(annotations[meta.AnnotationHookWaitTimeout])
	if err != nil {
		return nil, errors.Wrapf(err, "malformed annotation %q", meta.AnnotationHookWaitTimeout)
	}

	if noWait && waitTimeout > 0 {
		return nil, NewIllegalAnnotationCombinationError(meta.AnnotationHookNoWait, meta.AnnotationHookWaitTimeout)
	}

	restartPolicy := parseRestartPolicy(u)
	if restartPolicy != corev1.RestartPolicyNever {
		return nil, NewUnsupportedRestartPolicyError(restartPolicy)
	}

	h := &Hook{
		Unstructured: u,
		Type:         hookType,
		AllowFailure: allowFailure,
		NoWait:       noWait,
		WaitTimeout:  waitTimeout,
	}

	return h, nil
}

func parseRestartPolicy(obj *unstructured.Unstructured) corev1.RestartPolicy {
	value, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "restartPolicy")

	return corev1.RestartPolicy(value)
}

func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}

	return time.ParseDuration(s)
}

func parseBool(s string) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false
	}

	return b
}
