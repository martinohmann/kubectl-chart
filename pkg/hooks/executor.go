package hooks

import "github.com/martinohmann/kubectl-chart/pkg/chart"

// Executor executes chart lifecycle hooks
type Executor interface {
	// ExecHooks takes a chart and executes all hooks of type hookType that are
	// defined in the chart. Depending on the configuration of the hooks it may
	// returns errors if hooks fail or not. Should return all other errors that
	// occur while hook execution.
	ExecHooks(c *chart.Chart, hookType string) error
}

// NoopExecutor does not execute any hooks.
type NoopExecutor struct{}

// ExecHooks implements Executor.
func (e *NoopExecutor) ExecHooks(*chart.Chart, string) error {
	return nil
}
