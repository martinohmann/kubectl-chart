package deletions

import (
	"fmt"
	"strings"
	"time"

	"github.com/martinohmann/kubectl-chart/pkg/wait"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/klog"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

// Request is a request for resource deletions.
type Request struct {
	// Visitor will be used to walk the resources that should be deleted.
	Visitor resource.Visitor

	// DryRun if enabled, deletion is only simulated and printed.
	DryRun bool

	// WaitForDeletion will enabled waiting until the resources are completely
	// deleted from the cluster.
	WaitForDeletion bool
}

// Deleter is a resource deleter.
type Deleter interface {
	// Delete takes a deletion request and performs it.
	Delete(r *Request) error
}

// deleter is a Deleter implementation.
type deleter struct {
	genericclioptions.IOStreams
	Waiter *wait.Waiter
}

// NewDeleter creates a new resource deleter.
func NewDeleter(streams genericclioptions.IOStreams, waiter *wait.Waiter) Deleter {
	return &deleter{
		IOStreams: streams,
		Waiter:    waiter,
	}
}

// Delete performs a deletion request. It will walk all resources in the
// visitor provided in the request and attempts to delete them. Optionally, it
// is waited for until the deletion of the resources is complete if
// WaitForDeletion is set in the request.
func (d *deleter) Delete(r *Request) error {
	found := 0

	deletedInfos := []*resource.Info{}
	uidMap := wait.UIDMap{}

	err := r.Visitor.Visit(func(info *resource.Info, err error) error {
		if errors.IsNotFound(err) {
			return nil
		}

		if err != nil {
			return err
		}

		deletedInfos = append(deletedInfos, info)
		found++

		if r.DryRun {
			if err = info.Get(); err != nil {
				return err
			}

			d.PrintObj(info, true)

			return nil
		}

		policy := metav1.DeletePropagationBackground

		response, err := d.deleteResource(info, &metav1.DeleteOptions{
			PropagationPolicy: &policy,
		})
		if err != nil {
			return err
		}

		d.PrintObj(info, false)

		resourceLocation := wait.ResourceLocation{
			GroupResource: info.Mapping.Resource.GroupResource(),
			Namespace:     info.Namespace,
			Name:          info.Name,
		}

		if status, ok := response.(*metav1.Status); ok && status.Details != nil {
			uidMap[resourceLocation] = status.Details.UID
			return nil
		}

		responseMetadata, err := meta.Accessor(response)
		if err != nil {
			// we don't have UID, but we didn't fail the delete, next best
			// thing is just skipping the UID
			klog.V(1).Info(err)
			return nil
		}

		uidMap[resourceLocation] = responseMetadata.GetUID()

		return nil
	})
	if err != nil || found == 0 {
		return err
	}

	if !r.WaitForDeletion || r.DryRun {
		return nil
	}

	err = d.Waiter.Wait(&wait.Request{
		ConditionFn: wait.IsDeleted,
		Options: wait.Options{
			Timeout: 24 * time.Hour,
		},
		Visitor: resource.InfoListVisitor(deletedInfos),
		UIDMap:  uidMap,
	})
	if errors.IsForbidden(err) || errors.IsMethodNotSupported(err) {
		// if we're forbidden from waiting, we shouldn't fail.
		// if the resource doesn't support a verb we need, we shouldn't fail.
		klog.V(1).Info(err)
		return nil
	}

	return err
}

func (d *deleter) deleteResource(info *resource.Info, deleteOptions *metav1.DeleteOptions) (runtime.Object, error) {
	response, err := resource.NewHelper(info.Client, info.Mapping).
		DeleteWithOptions(info.Namespace, info.Name, deleteOptions)

	if err != nil {
		return nil, cmdutil.AddSourceToErr("deleting", info.Source, err)
	}

	return response, nil
}

// PrintObj prints out the object that was deleted (or would be deleted if dry
// run is enabled).
func (d *deleter) PrintObj(info *resource.Info, dryRun bool) {
	operation := "deleted"
	groupKind := info.Mapping.GroupVersionKind
	kindString := fmt.Sprintf("%s.%s", strings.ToLower(groupKind.Kind), groupKind.Group)
	if len(groupKind.Group) == 0 {
		kindString = strings.ToLower(groupKind.Kind)
	}

	if dryRun {
		operation = fmt.Sprintf("%s (dry run)", operation)
	}

	fmt.Fprintf(d.Out, "%s/%s %s\n", kindString, info.Name, operation)
}
