package wait

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
)

// Options are wait options for a single resource.
type Options struct {
	// Timeout is the timeout after which to stop waiting and return an error
	// if the waiting condition was not met yet.
	Timeout time.Duration

	// AllowFailure indicates if an error during waiting for this resource is
	// acceptable. In this case the error will just be logged.
	AllowFailure bool
}

// ResourceOptions defines custom wait options for resource UIDs.
type ResourceOptions map[types.UID]Options

// Request is a request to wait for multiple resources.
type Request struct {
	// Options are options for waiting on resource conditions to meet.
	Options Options
	// ResourceOptions is a map with Resource UIDs as keys. This allows
	// fine-grained configuration of per-resource wait options. This is
	// optional.
	ResourceOptions ResourceOptions

	// ConditionFn is used to determine when waiting should be stopped.
	ConditionFn ConditionFunc

	// Visitor will be used to walk the resources that should be waited on.
	Visitor resource.Visitor

	// UID is a map of resource locations to UIDs which can help in identifying
	// objects while waiting.
	UIDMap UIDMap
}

// OptionsFor returns Options for a resource info. If the resource has a UID
// and the request has custom options for it, those will be returned. The
// default request options will be returned otherwise.
func (r *Request) OptionsFor(info *resource.Info) Options {
	accessor, err := meta.Accessor(info.Object)
	if err == nil {
		uid := accessor.GetUID()

		options, ok := r.ResourceOptions[uid]
		if ok {
			return options
		}
	}

	return r.Options
}

// ResourceLocation holds the location of a resource.
type ResourceLocation struct {
	GroupResource schema.GroupResource
	Namespace     string
	Name          string
}

// UIDMap maps ResourceLocation with UID.
type UIDMap map[ResourceLocation]types.UID

// ConditionFunc is called for every resource that is waited for. It should
// check if the waiting condition is met or not, and if errors occured while
// waiting.
type ConditionFunc func(info *resource.Info, w *Waiter, o Options, uidMap UIDMap) (runtime.Object, bool, error)

// Waiter is a generic waiter.
type Waiter struct {
	genericclioptions.IOStreams

	Printer       printers.ResourcePrinter
	DynamicClient dynamic.Interface
}

// NewDefaultWaiter creates a new *Waiter which uses a printers.Nameprinter to
// print wait status.
func NewDefaultWaiter(streams genericclioptions.IOStreams, client dynamic.Interface) *Waiter {
	return NewWaiter(streams, &printers.NamePrinter{}, client)
}

// NewWaiter creates a new *Waiter value.
func NewWaiter(streams genericclioptions.IOStreams, p printers.ResourcePrinter, client dynamic.Interface) *Waiter {
	return &Waiter{
		IOStreams:     streams,
		Printer:       p,
		DynamicClient: client,
	}
}

// Waiter waits for all resources using the provided options. If no condition
// func is defined in the options the default condition to wait for is resource
// deletion.
func (w *Waiter) Wait(r *Request) error {
	err := r.Visitor.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		options := r.OptionsFor(info)

		obj, success, err := r.ConditionFn(info, w, options, r.UIDMap)
		if success {
			w.Printer.PrintObj(obj, w.Out)
			return nil
		}

		skipErr, ok := err.(*WaitSkippedError)
		if ok && skipErr != nil {
			fmt.Fprintln(w.ErrOut, skipErr.Error())
			return nil
		}

		statusError, ok := err.(*StatusFailedError)
		if ok && statusError != nil && options.AllowFailure {
			fmt.Fprintln(w.ErrOut, statusError.Error())
			return nil
		}

		if err == nil {
			return errors.Errorf("%v unsatisified for unknown reason", obj)
		}

		return err
	})

	return err
}
