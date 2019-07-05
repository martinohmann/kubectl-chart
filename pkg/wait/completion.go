package wait

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
)

var (
	jobGVK = schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}
)

// IsComplete is a cmdwait.ConditionFunc for waiting on a job to complete. It
// will also watch the failed status of the job and stops waiting with an error
// if the job failed.
func IsComplete(info *resource.Info, w *Waiter, o Options, uidMap UIDMap) (runtime.Object, bool, error) {
	if info.Mapping.GroupVersionKind != jobGVK {
		return info.Object, false, &WaitSkippedError{Name: info.Name, GroupVersionKind: info.Mapping.GroupVersionKind}
	}

	endTime := time.Now().Add(o.Timeout)

	for {
		if len(info.Name) == 0 {
			return info.Object, false, fmt.Errorf("resource name must be provided")
		}

		nameSelector := fields.OneTermEqualSelector("metadata.name", info.Name).String()

		var obj *unstructured.Unstructured
		// List with a name field selector to get the current resourceVersion
		// to watch from (not the object's resourceVersion)
		objList, err := w.DynamicClient.
			Resource(info.Mapping.Resource).
			Namespace(info.Namespace).
			List(metav1.ListOptions{FieldSelector: nameSelector})

		var resourceVersion string

		switch {
		case err != nil:
			return info.Object, false, err
		case len(objList.Items) != 1:
			resourceVersion = objList.GetResourceVersion()
		default:
			obj = &objList.Items[0]
			complete, err := isComplete(obj)
			if complete {
				return obj, true, nil
			}
			if err != nil {
				return obj, false, err
			}
			resourceVersion = objList.GetResourceVersion()
		}

		watchOptions := metav1.ListOptions{
			FieldSelector:   nameSelector,
			ResourceVersion: resourceVersion,
		}

		objWatch, err := w.DynamicClient.
			Resource(info.Mapping.Resource).
			Namespace(info.Namespace).
			Watch(watchOptions)
		if err != nil {
			return obj, false, err
		}

		errWaitTimeout := waitTimeoutError(wait.ErrWaitTimeout, info)
		if endTime.Sub(time.Now()) < 0 {
			return obj, false, errWaitTimeout
		}

		ctx, cancel := watchtools.ContextWithOptionalTimeout(context.Background(), o.Timeout)

		watchEvent, err := watchtools.UntilWithoutRetry(ctx, objWatch, completionWait{w.ErrOut}.isComplete)

		cancel()

		switch {
		case err == nil:
			return watchEvent.Object, true, nil
		case err == watchtools.ErrWatchClosed:
			continue
		case err == wait.ErrWaitTimeout:
			if watchEvent != nil {
				return watchEvent.Object, false, errWaitTimeout
			}

			return obj, false, errWaitTimeout
		default:
			return obj, false, err
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
		err = &StatusFailedError{Name: obj.GetName(), GroupVersionKind: obj.GroupVersionKind()}
	}

	return false, err
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
