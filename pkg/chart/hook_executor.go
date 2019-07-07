package chart

import (
	"fmt"
	"strings"
	"time"

	"github.com/martinohmann/kubectl-chart/pkg/deletions"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/martinohmann/kubectl-chart/pkg/wait"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
)

// HookExecutor executes chart lifecycle hooks.
type HookExecutor struct {
	genericclioptions.IOStreams
	DynamicClient  dynamic.Interface
	BuilderFactory func() *resource.Builder
	Mapper         meta.RESTMapper
	Deleter        deletions.Deleter
	Waiter         *wait.Waiter
	DryRun         bool
}

// ExecHooks executes hooks of hookType from chart c. It will attempt to delete
// job hooks matching a label selector that are already deployed to the cluster
// before creating the hooks to prevent errors.
func (e *HookExecutor) ExecHooks(c *Chart, hookType string) error {
	hooks := c.Hooks.Type(hookType)

	if len(hooks) == 0 {
		return nil
	}

	// Make sure that there are no conflicting hooks present in the cluster.
	err := e.cleanupHooks(c, hookType)
	if err != nil {
		return err
	}

	hookInfos := make([]*resource.Info, 0)

	err = hooks.EachItem(func(hook *Hook) error {
		e.PrintHook(hook, "triggered")

		if e.DryRun {
			return nil
		}

		gvk := hook.GroupVersionKind()

		gk := schema.GroupKind{
			Group: gvk.Group,
			Kind:  gvk.Kind,
		}

		mapping, err := e.Mapper.RESTMapping(gk, gvk.Version)
		if err != nil {
			return err
		}

		res := e.DynamicClient.
			Resource(mapping.Resource).
			Namespace(hook.GetNamespace())

		obj, err := res.Create(hook.Unstructured, metav1.CreateOptions{})
		if err != nil {
			return err
		}

		info := &resource.Info{
			Mapping:         mapping,
			Namespace:       obj.GetNamespace(),
			Name:            obj.GetName(),
			Object:          obj,
			ResourceVersion: obj.GetResourceVersion(),
		}

		hookInfos = append(hookInfos, info)

		return nil
	})
	if err != nil {
		return err
	}

	return e.waitForCompletion(hookInfos)
}

func (e *HookExecutor) cleanupHooks(c *Chart, hookType string) error {
	result := e.BuilderFactory().
		Unstructured().
		ContinueOnError().
		RequireObject(false).
		AllNamespaces(true).
		ResourceTypeOrNameArgs(true, resources.KindJob).
		LabelSelector(c.HookLabelSelector(hookType)).
		Flatten().
		Do().
		IgnoreErrors(apierrors.IsNotFound)
	if err := result.Err(); err != nil {
		return err
	}

	return e.Deleter.Delete(&deletions.Request{
		DryRun:  e.DryRun,
		Waiter:  e.Waiter,
		Visitor: result,
	})
}

func (e *HookExecutor) waitForCompletion(infos []*resource.Info) error {
	err := e.Waiter.Wait(&wait.Request{
		ConditionFn: wait.IsComplete,
		Options: wait.Options{
			Timeout:      24 * time.Hour,
			AllowFailure: true,
		},
		Visitor: resource.InfoListVisitor(infos),
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
func (e *HookExecutor) PrintHook(hook *Hook, operation string) {
	groupKind := hook.GroupVersionKind()
	kindString := fmt.Sprintf("%s.%s", strings.ToLower(groupKind.Kind), groupKind.Group)
	if len(groupKind.Group) == 0 {
		kindString = strings.ToLower(groupKind.Kind)
	}

	if timeout, err := hook.WaitTimeout(); err != nil {
		// In case the timeout fails to parse, we just log the error and use
		// default
		klog.V(1).Info(err)
		return
	} else if timeout > 0 {
		operation = fmt.Sprintf("%s (timeout %s)", operation, timeout)
	}

	if e.DryRun {
		operation = fmt.Sprintf("%s (dry run)", operation)
	}

	fmt.Fprintf(e.Out, "hook %s/%s %s\n", kindString, hook.GetName(), operation)
}
