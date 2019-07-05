package wait

import (
	"context"
	"fmt"
	"io"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/resource"
	watchtools "k8s.io/client-go/tools/watch"
)

// IsDeleted is a condition func for waiting for something to be deleted
func IsDeleted(info *resource.Info, w *Waiter, o Options, uidMap UIDMap) (runtime.Object, bool, error) {
	endTime := time.Now().Add(o.Timeout)

	for {
		if len(info.Name) == 0 {
			return info.Object, false, fmt.Errorf("resource name must be provided")
		}

		nameSelector := fields.OneTermEqualSelector("metadata.name", info.Name).String()

		// List with a name field selector to get the current resourceVersion
		// to watch from (not the object's resourceVersion)
		objList, err := w.DynamicClient.
			Resource(info.Mapping.Resource).
			Namespace(info.Namespace).
			List(metav1.ListOptions{FieldSelector: nameSelector})
		if apierrors.IsNotFound(err) {
			return info.Object, true, nil
		}

		if err != nil {
			return info.Object, false, err
		}

		if len(objList.Items) != 1 {
			return info.Object, true, nil
		}

		obj := &objList.Items[0]
		resourceLocation := ResourceLocation{
			GroupResource: info.Mapping.Resource.GroupResource(),
			Namespace:     obj.GetNamespace(),
			Name:          obj.GetName(),
		}

		if uid, ok := uidMap[resourceLocation]; ok {
			if obj.GetUID() != uid {
				return obj, true, nil
			}
		}

		watchOptions := metav1.ListOptions{
			FieldSelector:   nameSelector,
			ResourceVersion: objList.GetResourceVersion(),
		}

		objWatch, err := w.DynamicClient.
			Resource(info.Mapping.Resource).
			Namespace(info.Namespace).
			Watch(watchOptions)
		if err != nil {
			return obj, false, err
		}

		errWaitTimeout := waitTimeoutError(err, info)
		if endTime.Sub(time.Now()) < 0 {
			return obj, false, errWaitTimeout
		}

		ctx, cancel := watchtools.ContextWithOptionalTimeout(context.Background(), o.Timeout)

		watchEvent, err := watchtools.UntilWithoutRetry(ctx, objWatch, deletionWait{errOut: w.ErrOut}.isDeleted)

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

type deletionWait struct {
	errOut io.Writer
}

func (w deletionWait) isDeleted(event watch.Event) (bool, error) {
	switch event.Type {
	case watch.Error:
		// keep waiting in the event we see an error - we expect the watch to be closed by
		// the server if the error is unrecoverable.
		err := apierrors.FromObject(event.Object)
		fmt.Fprintf(w.errOut, "error: An error occurred while waiting for the object to be deleted: %v", err)
		return false, nil
	case watch.Deleted:
		return true, nil
	default:
		return false, nil
	}
}
