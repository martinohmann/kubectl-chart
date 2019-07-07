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
	"k8s.io/client-go/dynamic"
	watchtools "k8s.io/client-go/tools/watch"
)

type DeletionWait struct {
	DynamicClient dynamic.Interface
	ErrOut        io.Writer

	// UID is a map of resource locations to UIDs which can help in identifying
	// objects while waiting.
	UIDMap UIDMap
}

func NewDeletedConditionFunc(client dynamic.Interface, errOut io.Writer, uidMap UIDMap) ConditionFunc {
	w := DeletionWait{
		DynamicClient: client,
		ErrOut:        errOut,
		UIDMap:        uidMap,
	}

	return w.ConditionFunc
}

// ConditionFunc waits for something to be deleted.
func (w DeletionWait) ConditionFunc(info *resource.Info, o Options) (runtime.Object, bool, error) {
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

		if uid, ok := w.UIDMap[resourceLocation]; ok {
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

		errWaitTimeout := waitTimeoutError(wait.ErrWaitTimeout, info)
		if endTime.Sub(time.Now()) < 0 {
			return obj, false, errWaitTimeout
		}

		ctx, cancel := watchtools.ContextWithOptionalTimeout(context.Background(), o.Timeout)

		watchEvent, err := watchtools.UntilWithoutRetry(ctx, objWatch, w.isDeleted)

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

func (w DeletionWait) isDeleted(event watch.Event) (bool, error) {
	switch event.Type {
	case watch.Error:
		// keep waiting in the event we see an error - we expect the watch to be closed by
		// the server if the error is unrecoverable.
		err := apierrors.FromObject(event.Object)
		fmt.Fprintf(w.ErrOut, "error: An error occurred while waiting for the object to be deleted: %v", err)
		return false, nil
	case watch.Deleted:
		return true, nil
	default:
		return false, nil
	}
}
