package chart

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/martinohmann/kubectl-chart/pkg/deletions"
	"github.com/martinohmann/kubectl-chart/pkg/printers"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/martinohmann/kubectl-chart/pkg/wait"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
)

const (
	PreApplyHook   = "pre-apply"
	PostApplyHook  = "post-apply"
	PreDeleteHook  = "pre-delete"
	PostDeleteHook = "post-delete"
)

var (
	DefaultHookWaitTimeout = 2 * time.Hour

	ValidHooks             = []string{PreApplyHook, PostApplyHook, PreDeleteHook, PostDeleteHook}
	ValidHookResourceKinds = []string{resources.KindJob}
)

type Hook struct {
	*Resource
}

func NewHook(obj runtime.Object) *Hook {
	return &Hook{
		Resource: NewResource(obj),
	}
}

func (h *Hook) RestartPolicy() string {
	return h.nestedString("spec", "template", "spec", "restartPolicy")
}

func (h *Hook) Type() string {
	return h.nestedString("metadata", "annotations", AnnotationHookType)
}

func (h *Hook) AllowFailure() bool {
	value := h.nestedString("metadata", "annotations", AnnotationHookAllowFailure)
	b, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}

	return b
}

func (h *Hook) WaitTimeout() (time.Duration, error) {
	value, found, _ := unstructured.NestedString(h.Object, "metadata", "annotations", AnnotationHookWaitTimeout)
	if !found {
		return 0, nil
	}

	timeout, err := time.ParseDuration(value)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse annotation %q: %s", AnnotationHookWaitTimeout, timeout)
	}

	return timeout, nil
}

type HookMap map[string]HookList

func (m HookMap) GetObjects() []runtime.Object {
	objs := make([]runtime.Object, 0)
	for _, s := range m {
		objs = append(objs, s.GetObjects()...)
	}

	return objs
}

func (m HookMap) Type(hookType string) HookList {
	return m[hookType]
}

type HookList []*Hook

func (l HookList) GetObjects() []runtime.Object {
	objs := make([]runtime.Object, len(l))
	for i := range l {
		objs[i] = l[i].GetObject()
	}

	return objs
}

func (l HookList) EachItem(fn func(*Hook) error) error {
	for _, h := range l {
		err := fn(h)
		if err != nil {
			return err
		}
	}

	return nil
}

func IsValidHookType(typ string) bool {
	for _, t := range ValidHooks {
		if t == typ {
			return true
		}
	}

	return false
}

func IsValidHookResourceKind(kind string) bool {
	for _, k := range ValidHookResourceKinds {
		if k == kind {
			return true
		}
	}

	return false
}

// HasHookAnnotation returns true if obj is annotated to be a hook.
func HasHookAnnotation(obj runtime.Object) (bool, error) {
	_, found, err := resources.GetAnnotation(obj, AnnotationHookType)
	if err != nil {
		return false, err
	}

	return found, nil
}

type HookTypeError struct {
	Type       string
	Additional []string
}

func (e HookTypeError) Error() string {
	msg := fmt.Sprintf(
		"invalid hook type %q, allowed values are \"%s\"",
		e.Type,
		strings.Join(ValidHooks, `", "`),
	)

	if len(e.Additional) > 0 {
		msg = fmt.Sprintf("%s, \"%s\"", msg, strings.Join(e.Additional, `", "`))
	}

	return msg
}

type HookResourceKindError struct {
	Kind string
}

func (e HookResourceKindError) Error() string {
	return fmt.Sprintf(
		"invalid hook resource kind %q, allowed values are \"%s\"",
		e.Kind,
		strings.Join(ValidHookResourceKinds, `", "`),
	)
}

// ValidateHook returns an error if a hook has an unsupported type or resource
// kind or if other resource fields have unsupported values.
func ValidateHook(h *Hook) error {
	if !IsValidHookType(h.Type()) {
		return HookTypeError{Type: h.Type()}
	}

	if !IsValidHookResourceKind(h.GetKind()) {
		return HookResourceKindError{Kind: h.GetKind()}
	}

	if h.RestartPolicy() != "Never" {
		return errors.Errorf("invalid hook %q: restartPolicy of the pod template must be %q", h.GetName(), "Never")
	}

	return nil
}

// HookExecutor executes chart lifecycle hooks.
type HookExecutor struct {
	genericclioptions.IOStreams
	DynamicClient  dynamic.Interface
	BuilderFactory func() *resource.Builder
	Mapper         meta.RESTMapper
	Deleter        deletions.Deleter
	Waiter         wait.Waiter
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

	infos := make([]*resource.Info, 0)
	resourceOptions := make(wait.ResourceOptions)

	err = hooks.EachItem(func(hook *Hook) error {
		e.PrintHook(hook)

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
			AllowFailure: hook.AllowFailure(),
			Timeout:      DefaultHookWaitTimeout,
		}

		timeout, err := hook.WaitTimeout()
		if err == nil && timeout > 0 {
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

func (e *HookExecutor) cleanupHooks(c *Chart, hookType string) error {
	result := e.BuilderFactory().
		Unstructured().
		ContinueOnError().
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
		Waiter:  e.Waiter,
		Visitor: result,
	})
}

func (e *HookExecutor) waitForCompletion(infos []*resource.Info, options wait.ResourceOptions) error {
	if len(infos) == 0 {
		return nil
	}

	err := e.Waiter.Wait(&wait.Request{
		ConditionFn: wait.NewCompletionConditionFunc(e.DynamicClient, e.ErrOut),
		Options: wait.Options{
			Timeout: DefaultHookWaitTimeout,
		},
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
func (e *HookExecutor) PrintHook(hook *Hook) error {
	operation := "triggered"

	if timeout, _ := hook.WaitTimeout(); timeout > 0 {
		operation = fmt.Sprintf("%s (timeout %s)", operation, timeout)
	}

	p := printers.NewNamePrinter(operation, e.DryRun)

	return p.PrintObj(hook, e.Out)
}
