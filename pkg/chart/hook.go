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
	// AnnotationHookType contains the type of the hook. If this annotation is
	// set on a Job it will be treated as a hook and not show up as regular
	// resource anymore.
	AnnotationHookType = "kubectl-chart/hook-type"

	// AnnotationHookAllowFailure controls the behaviour in the event where the
	// hook fails due to timeouts or because the job failed. If set to "true",
	// these errors just will be logged and processing of other hooks and
	// resources continues. Other unhandled errors occuring during hook
	// execution (e.g. API-Server errors) will still bubble up the error
	// handling chain.
	AnnotationHookAllowFailure = "kubectl-chart/hook-allow-failure"

	// AnnotationHookNoWait controls the waiting behaviour. If set to "true",
	// it is not waited for the hook to complete. This cannot be used together
	// with AnnotationHookAllowFailure because the success of a hook is not
	// checked if we do not wait for it to finish.
	AnnotationHookNoWait = "kubectl-chart/hook-no-wait"

	// AnnotationHookWaitTimeout sets a custom wait timeout for a hook. If not
	// set, wait.DefaultWaitTimeout is used.
	AnnotationHookWaitTimeout = "kubectl-chart/hook-wait-timeout"

	PreApplyHook   = "pre-apply"
	PostApplyHook  = "post-apply"
	PreDeleteHook  = "pre-delete"
	PostDeleteHook = "post-delete"
)

var (
	// ValidHookTypes contains a list of supported hook types.
	ValidHookTypes = []string{PreApplyHook, PostApplyHook, PreDeleteHook, PostDeleteHook}

	jobGK  = schema.GroupKind{Group: "batch", Kind: "Job"}
	jobGVR = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
)

// Hook is a chart hook that gets executed before or after apply/delete
// depending on its type.
type Hook struct {
	*Resource
}

// NewHook creates a new hook from a runtime object.
func NewHook(obj runtime.Object) *Hook {
	return &Hook{
		Resource: NewResource(obj),
	}
}

// RestartPolicy gets the restartPolicy from the hook jobs pod spec.
func (h *Hook) RestartPolicy() string {
	return h.nestedString("spec", "template", "spec", "restartPolicy")
}

// Type returns the hook type, e.g. post-apply.
func (h *Hook) Type() string {
	return h.nestedString("metadata", "annotations", AnnotationHookType)
}

// AllowFailure indicates whether the hook is allowed to fail or not. If true,
// hook errors are only printed and execution continues. If this is true,
// NoWait() must not return true as well.
func (h *Hook) AllowFailure() bool {
	value := h.nestedString("metadata", "annotations", AnnotationHookAllowFailure)
	b, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}

	return b
}

// NoWait indicates whether it should be waited for the hook to complete or
// not. If true, AllowFailure() must not return true and it is also not
// possible to detect possible hook failures.
func (h *Hook) NoWait() bool {
	value := h.nestedString("metadata", "annotations", AnnotationHookNoWait)
	b, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}

	return b
}

// WaitTimeout returns the timeout for waiting for hook completion and an error
// if the annotation cannot be parsed.
func (h *Hook) WaitTimeout() (time.Duration, error) {
	value, found, _ := unstructured.NestedString(h.Object, "metadata", "annotations", AnnotationHookWaitTimeout)
	if !found {
		return 0, nil
	}

	timeout, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
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
	for _, t := range ValidHookTypes {
		if t == typ {
			return true
		}
	}

	return false
}

// HasHookAnnotation returns true if obj is annotated to be a hook.
func HasHookAnnotation(obj runtime.Object) (bool, error) {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return false, err
	}

	annotations := metadata.GetAnnotations()

	_, found := annotations[AnnotationHookType]

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
		strings.Join(ValidHookTypes, `", "`),
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
		"invalid hook resource kind %q, only %q is allowed",
		e.Kind,
		jobGK.Kind,
	)
}

// ValidateHook returns an error if a hook has an unsupported type or resource
// kind or if other resource fields have unsupported values.
func ValidateHook(h *Hook) error {
	if !IsValidHookType(h.Type()) {
		return HookTypeError{Type: h.Type()}
	}

	if h.GetKind() != jobGK.Kind {
		return HookResourceKindError{Kind: h.GetKind()}
	}

	if h.RestartPolicy() != "Never" {
		return errors.Errorf("invalid hook %q: restartPolicy of the pod template must be %q", h.GetName(), "Never")
	}

	if h.NoWait() && h.AllowFailure() {
		return errors.Errorf("invalid hook %q: %s and %s cannot be true at the same time", h.GetName(), AnnotationHookNoWait, AnnotationHookAllowFailure)
	}

	timeout, err := h.WaitTimeout()
	if err != nil {
		return errors.Wrapf(err, "invalid hook %q: malformed %s annotation", h.GetName(), AnnotationHookWaitTimeout)
	}

	if h.NoWait() && timeout > 0 {
		return errors.Errorf("invalid hook %q: %s and %s cannot be set at the same time", h.GetName(), AnnotationHookNoWait, AnnotationHookWaitTimeout)
	}

	return err
}

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

		mapping, err := e.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
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

		if hook.NoWait() {
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
			AllowFailure: hook.AllowFailure(),
			Timeout:      wait.DefaultWaitTimeout,
		}

		timeout, _ := hook.WaitTimeout()
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

func (e *HookExecutor) cleanupHooks(c *Chart, hookType string) error {
	objs, err := e.DynamicClient.
		Resource(jobGVR).
		Namespace(metav1.NamespaceAll).
		List(metav1.ListOptions{
			LabelSelector: c.HookLabelSelector(hookType),
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
func (e *HookExecutor) PrintHook(hook *Hook) error {
	options := make([]string, 0)

	if timeout, _ := hook.WaitTimeout(); timeout > 0 {
		options = append(options, fmt.Sprintf("timeout %s", timeout))
	}

	if hook.NoWait() {
		options = append(options, "no-wait")
	}

	if hook.AllowFailure() {
		options = append(options, "allow-failure")
	}

	return e.Printer.
		WithOperation("triggered").
		WithContext(options...).
		PrintObj(hook, e.Out)
}
