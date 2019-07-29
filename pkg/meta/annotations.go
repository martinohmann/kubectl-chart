package meta

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// AnnotationHookType contains the type of the hook. If this annotation is
	// set on a Job it will be treated as a hook and not show up as regular
	// resource anymore.
	AnnotationHookType = "kubectl-chart/hook-type"

	// AnnotationHookAllowFailure controls the behaviour in the event where the
	// hook fails due to timeouts or because the job failed. If set to "true",
	// these errors just will be logged and processing of other hooks and
	// resources continues. Other unhandled errors occuring during hook
	// execution (e.g. API-Server errors) will still bubble up the error
	// handling chain.
	AnnotationHookAllowFailure = "kubectl-chart/hook-allow-failure"

	// AnnotationHookNoWait controls the waiting behaviour. If set to "true",
	// it is not waited for the hook to complete. This cannot be used together
	// with AnnotationHookAllowFailure because the success of a hook is not
	// checked if we do not wait for it to finish.
	AnnotationHookNoWait = "kubectl-chart/hook-no-wait"

	// AnnotationHookWaitTimeout sets a custom wait timeout for a hook. If not
	// set, wait.DefaultWaitTimeout is used.
	AnnotationHookWaitTimeout = "kubectl-chart/hook-wait-timeout"

	// AnnotationDeletionPolicy can be set on resources to specify non-default
	// deletion behaviour. Currently this annotation is ignored on all
	// resources except for StatefulSets.
	AnnotationDeletionPolicy = "kubectl-chart/deletion-policy"
)

// HasAnnotation returns true if an annotation key exists and has given value.
// If value is omitted, only the existence of the annotation key is checked.
func HasAnnotation(obj runtime.Object, key string, value ...string) bool {
	metadata, _ := meta.Accessor(obj)

	val, found := metadata.GetAnnotations()[key]

	if found && len(value) > 0 {
		return value[0] == val
	}

	return found
}
