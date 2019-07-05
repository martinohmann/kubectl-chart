package wait

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
)

// StatusFailedError is used when a job transitioned into status failed. This
// is usually an error that might be acceptable and can be handled.
type StatusFailedError struct {
	Name             string
	GroupVersionKind schema.GroupVersionKind
}

// Error implements error.
func (e StatusFailedError) Error() string {
	return fmt.Sprintf("%s %q is in status failed", e.GroupVersionKind.String(), e.Name)
}

type WaitSkippedError struct {
	Name             string
	GroupVersionKind schema.GroupVersionKind
}

// Error implements error.
func (e WaitSkippedError) Error() string {
	return fmt.Sprintf("skipped waiting for %s %q", e.GroupVersionKind.String(), e.Name)
}

type WaitTimeoutError struct {
	Err      error
	Resource string
	Name     string
}

func (e WaitTimeoutError) Error() string {
	return fmt.Sprintf("%s on %s/%s", e.Err.Error(), e.Resource, e.Name)
}

func waitTimeoutError(err error, info *resource.Info) error {
	return &WaitTimeoutError{err, info.Mapping.Resource.Resource, info.Name}
}
