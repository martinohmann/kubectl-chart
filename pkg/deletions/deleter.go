package deletions

import (
	"fmt"

	"github.com/martinohmann/kubectl-chart/pkg/printers"
	"github.com/martinohmann/kubectl-chart/pkg/wait"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
)

// Deleter is a resource deleter.
type Deleter interface {
	// Delete walks all resources in the visitor and attempts to delete them.
	// Optionally, it waits for until the deletion of the resources is complete
	// if the Deleter supports waiting.
	Delete(v resource.Visitor) error
}

// deleter is a Deleter implementation.
type deleter struct {
	genericclioptions.IOStreams
	DynamicClient dynamic.Interface
	Printer       printers.ResourcePrinter
	Waiter        wait.Waiter

	// DryRun if enabled, deletion is only simulated and printed.
	DryRun bool
}

// NewDeleter creates a new resource deleter.
func NewDeleter(streams genericclioptions.IOStreams, client dynamic.Interface, printer printers.ContextPrinter, dryRun bool) Deleter {
	return &deleter{
		IOStreams:     streams,
		DynamicClient: client,
		DryRun:        dryRun,
		Waiter:        wait.NewDefaultWaiter(streams),
		Printer:       printer.WithOperation("deleted"),
	}
}

// Delete implements Deleter.
func (d *deleter) Delete(v resource.Visitor) error {
	found := 0

	deletedInfos := []*resource.Info{}
	uidMap := wait.UIDMap{}

	err := v.Visit(func(info *resource.Info, err error) error {
		if errors.IsNotFound(err) {
			fmt.Fprintln(d.ErrOut, err)
		} else if err != nil {
			return err
		}

		deletedInfos = append(deletedInfos, info)
		found++

		if d.DryRun {
			_, err := d.getResource(info)
			if err != nil {
				return err
			}

			return d.Printer.PrintObj(info.Object, d.Out)
		}

		err = d.deleteResource(info)
		if err != nil {
			return err
		}

		d.Printer.PrintObj(info.Object, d.Out)

		resourceLocation := wait.ResourceLocation{
			GroupResource: info.Mapping.Resource.GroupResource(),
			Namespace:     info.Namespace,
			Name:          info.Name,
		}

		metadata, err := meta.Accessor(info.Object)
		if err != nil {
			// we don't have UID, but we didn't fail the delete, next best
			// thing is just skipping the UID
			klog.V(1).Info(err)
			return nil
		}

		uidMap[resourceLocation] = metadata.GetUID()

		return nil
	})
	if (err != nil && !errors.IsNotFound(err)) || found == 0 {
		return err
	}

	if d.Waiter == nil || d.DryRun {
		return nil
	}

	err = d.Waiter.Wait(&wait.Request{
		ConditionFn: wait.NewDeletedConditionFunc(d.DynamicClient, d.ErrOut, uidMap),
		Visitor:     resource.InfoListVisitor(deletedInfos),
	})
	if errors.IsForbidden(err) || errors.IsMethodNotSupported(err) {
		// if we're forbidden from waiting, we shouldn't fail.
		// if the resource doesn't support a verb we need, we shouldn't fail.
		klog.V(1).Info(err)
		return nil
	}

	return err
}

func (d *deleter) getResource(info *resource.Info) (*unstructured.Unstructured, error) {
	return d.DynamicClient.
		Resource(info.Mapping.Resource).
		Namespace(info.Namespace).
		Get(info.Name, metav1.GetOptions{})
}

func (d *deleter) deleteResource(info *resource.Info) error {
	policy := metav1.DeletePropagationBackground

	return d.DynamicClient.
		Resource(info.Mapping.Resource).
		Namespace(info.Namespace).
		Delete(info.Name, &metav1.DeleteOptions{
			PropagationPolicy: &policy,
		})
}
