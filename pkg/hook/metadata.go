package hook

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
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
)

const (
	// LabelHookChartName is used to attach a label to each hook to be able to
	// keep track of them once they are deployed into a cluster. The label is
	// different from LabelChartName because hooks have a different lifecycle
	// than normal resources.
	LabelHookChartName = "kubectl-chart/hook-chart-name"

	// LabelHookType is set on chart hooks to be able to clean them up by type.
	LabelHookType = "kubectl-chart/hook-type"
)

// jobGK is the GroupKind that a hook resource must have.
var jobGK = schema.GroupKind{Group: "batch", Kind: "Job"}

// LabelSelector returns a selector which can be used to find hooks of given
// type for given chart in a cluster.
func LabelSelector(chartName, hookType string) string {
	return fmt.Sprintf("%s=%s,%s=%s", LabelHookChartName, chartName, LabelHookType, hookType)
}
