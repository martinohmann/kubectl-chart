package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/resource"
	watchtools "k8s.io/client-go/tools/watch"
	cmdwait "k8s.io/kubernetes/pkg/kubectl/cmd/wait"
)

// IsComplete is a cmdwait.ConditionFunc for waiting on a job to complete. It
// will also watch the failed status of the job and stops waiting with an error
// if the job failed.
func IsComplete(info *resource.Info, o *cmdwait.WaitOptions) (runtime.Object, bool, error) {
	endTime := time.Now().Add(o.Timeout)
	for {
		if len(info.Name) == 0 {
			return info.Object, false, fmt.Errorf("resource name must be provided")
		}

		nameSelector := fields.OneTermEqualSelector("metadata.name", info.Name).String()

		var gottenObj *unstructured.Unstructured
		// List with a name field selector to get the current resourceVersion to watch from (not the object's resourceVersion)
		gottenObjList, err := o.DynamicClient.Resource(info.Mapping.Resource).Namespace(info.Namespace).List(metav1.ListOptions{FieldSelector: nameSelector})

		resourceVersion := ""
		switch {
		case err != nil:
			return info.Object, false, err
		case len(gottenObjList.Items) != 1:
			resourceVersion = gottenObjList.GetResourceVersion()
		default:
			gottenObj = &gottenObjList.Items[0]
			complete, err := isComplete(gottenObj)
			if complete {
				return gottenObj, true, nil
			}
			if err != nil {
				return gottenObj, false, err
			}
			resourceVersion = gottenObjList.GetResourceVersion()
		}

		watchOptions := metav1.ListOptions{}
		watchOptions.FieldSelector = nameSelector
		watchOptions.ResourceVersion = resourceVersion
		objWatch, err := o.DynamicClient.Resource(info.Mapping.Resource).Namespace(info.Namespace).Watch(watchOptions)
		if err != nil {
			return gottenObj, false, err
		}

		timeout := endTime.Sub(time.Now())
		errWaitTimeoutWithName := extendErrWaitTimeout(wait.ErrWaitTimeout, info)
		if timeout < 0 {
			// we're out of time
			return gottenObj, false, errWaitTimeoutWithName
		}

		ctx, cancel := watchtools.ContextWithOptionalTimeout(context.Background(), o.Timeout)
		watchEvent, err := watchtools.UntilWithoutRetry(ctx, objWatch, completionWait{o.ErrOut}.isComplete)
		cancel()
		switch {
		case err == nil:
			return watchEvent.Object, true, nil
		case err == watchtools.ErrWatchClosed:
			continue
		case err == wait.ErrWaitTimeout:
			if watchEvent != nil {
				return watchEvent.Object, false, errWaitTimeoutWithName
			}
			return gottenObj, false, errWaitTimeoutWithName
		default:
			return gottenObj, false, err
		}
	}
}

func isComplete(obj *unstructured.Unstructured) (bool, error) {
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil {
		return false, err
	}

	if !found {
		return false, nil
	}

	statusComplete, ok := getConditionStatus(conditions, "complete")
	if ok {
		return statusComplete == "true", nil
	}

	statusFailed, ok := getConditionStatus(conditions, "failed")
	if ok && statusFailed == "true" {
		return false, &StatusFailedError{Name: obj.GetName(), GroupVersionKind: obj.GroupVersionKind()}
	}

	return false, nil
}

func getConditionStatus(conditions []interface{}, name string) (string, bool) {
	for _, conditionUncast := range conditions {
		condition := conditionUncast.(map[string]interface{})

		typ, found, err := unstructured.NestedString(condition, "type")
		if !found || err != nil || strings.ToLower(typ) != name {
			continue
		}

		status, found, err := unstructured.NestedString(condition, "status")
		if !found || err != nil {
			continue
		}

		return strings.ToLower(status), true
	}

	return "", false
}

type completionWait struct {
	errOut io.Writer
}

func (w completionWait) isComplete(event watch.Event) (bool, error) {
	if event.Type == watch.Error {
		// keep waiting in the event we see an error - we expect the watch to be closed by
		// the server
		err := apierrors.FromObject(event.Object)
		fmt.Fprintf(w.errOut, "error: An error occurred while waiting for the condition to be satisfied: %v", err)
		return false, nil
	}

	if event.Type == watch.Deleted {
		// this will chain back out, result in another get and an return false back up the chain
		return false, nil
	}

	obj := event.Object.(*unstructured.Unstructured)

	return isComplete(obj)
}

func extendErrWaitTimeout(err error, info *resource.Info) error {
	return fmt.Errorf("%s on %s/%s", err.Error(), info.Mapping.Resource.Resource, info.Name)
}

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
