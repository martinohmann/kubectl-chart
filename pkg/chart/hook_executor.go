package chart

import (
	"fmt"

	"github.com/martinohmann/kubectl-chart/pkg/deletions"
	"github.com/martinohmann/kubectl-chart/pkg/hook"
	"github.com/martinohmann/kubectl-chart/pkg/printers"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/martinohmann/kubectl-chart/pkg/wait"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
)

// HookExecutor executes chart lifecycle hooks.
type HookExecutor struct {
	genericclioptions.IOStreams
	DynamicClient dynamic.Interface
	Mapper        meta.RESTMapper
	Deleter       deletions.Deleter
	Waiter        wait.Waiter
	Printer       printers.ContextPrinter
	DryRun        bool
}

// ExecHooks executes hooks of hookType from chart c. It will attempt to delete
// job hooks matching a label selector that are already deployed to the cluster
// before creating the hooks to prevent errors.
func (e *HookExecutor) ExecHooks(c *Chart, hookType string) error {
	hooks := c.Hooks[hookType]

	if len(hooks) == 0 {
		return nil
	}

	// Make sure that there are no conflicting hooks present in the cluster.
	err := e.cleanupHooks(c.Config.Name, hookType)
	if err != nil {
		return err
	}

	infos := make([]*resource.Info, 0)
	resourceOptions := make(wait.ResourceOptions)

	err = hooks.EachItem(func(h *hook.Hook) error {
		e.PrintHook(h)

		if e.DryRun {
			return nil
		}

		gvk := h.GroupVersionKind()

		mapping, err := e.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return err
		}

		obj, err := e.DynamicClient.
			Resource(mapping.Resource).
			Namespace(h.GetNamespace()).
			Create(h.Unstructured, metav1.CreateOptions{})
		if err != nil {
			return err
		}

		if h.NoWait() {
			return nil
		}

		info := &resource.Info{
			Mapping:         mapping,
			Namespace:       obj.GetNamespace(),
			Name:            obj.GetName(),
			Object:          obj,
			ResourceVersion: obj.GetResourceVersion(),
		}

		infos = append(infos, info)

		metadata, err := meta.Accessor(obj)
		if err != nil {
			klog.V(1).Info(err)
			return nil
		}

		uid := metadata.GetUID()
		if uid == "" {
			return nil
		}

		options := wait.Options{
			AllowFailure: h.AllowFailure(),
			Timeout:      wait.DefaultWaitTimeout,
		}

		timeout, _ := h.WaitTimeout()
		if timeout > 0 {
			options.Timeout = timeout
		}

		resourceOptions[uid] = options

		return nil
	})
	if err != nil {
		return err
	}

	return e.waitForCompletion(infos, resourceOptions)
}

func (e *HookExecutor) cleanupHooks(chartName, hookType string) error {
	objs, err := e.DynamicClient.
		Resource(jobGVR).
		Namespace(metav1.NamespaceAll).
		List(metav1.ListOptions{
			LabelSelector: hook.LabelSelector(chartName, hookType),
		})
	if apierrors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return err
	}

	infos, err := resources.ToInfoList(objs, e.Mapper)
	if err != nil {
		return err
	}

	return e.Deleter.Delete(resource.InfoListVisitor(infos))
}

func (e *HookExecutor) waitForCompletion(infos []*resource.Info, options wait.ResourceOptions) error {
	if len(infos) == 0 {
		return nil
	}

	err := e.Waiter.Wait(&wait.Request{
		ConditionFn:     wait.NewCompletionConditionFunc(e.DynamicClient, e.ErrOut),
		ResourceOptions: options,
		Visitor:         resource.InfoListVisitor(infos),
	})
	if apierrors.IsForbidden(err) || apierrors.IsMethodNotSupported(err) {
		// if we're forbidden from waiting, we shouldn't fail.
		// if the resource doesn't support a verb we need, we shouldn't fail.
		klog.V(1).Info(err)
		return nil
	}

	return err
}

// PrintHook prints a hooks.
func (e *HookExecutor) PrintHook(h *hook.Hook) error {
	options := make([]string, 0)

	if timeout, _ := h.WaitTimeout(); timeout > 0 {
		options = append(options, fmt.Sprintf("timeout %s", timeout))
	}

	if h.NoWait() {
		options = append(options, "no-wait")
	}

	if h.AllowFailure() {
		options = append(options, "allow-failure")
	}

	return e.Printer.
		WithOperation("triggered").
		WithContext(options...).
		PrintObj(h, e.Out)
}
