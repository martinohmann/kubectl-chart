package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/printers"
	"github.com/martinohmann/kubectl-chart/pkg/recorders"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/martinohmann/kubectl-chart/pkg/wait"
	"github.com/martinohmann/kubectl-chart/pkg/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kprinters "k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/kubectl/cmd/apply"
	"k8s.io/kubernetes/pkg/kubectl/cmd/delete"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi"
	"k8s.io/kubernetes/pkg/kubectl/validation"
)

func NewApplyCmd(f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewApplyOptions(streams)

	cmd := &cobra.Command{
		Use:  "apply",
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.ChartFlags.AddFlags(cmd)
	o.DiffFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.ServerDryRun, "server-dry-run", o.ServerDryRun, "If true, request will be sent to server with dry-run flag, which means the modifications won't be persisted. This is an alpha feature and flag.")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", o.DryRun, "If true, only print the object that would be sent, without sending it. Warning: --dry-run cannot accurately output the result of merging the local manifest and the server-side data. Use --server-dry-run to get the merged result instead.")
	cmd.Flags().BoolVar(&o.ShowDiff, "diff", o.ShowDiff, "If set, a diff for all resources will be displayed")

	return cmd
}

type ApplyOptions struct {
	genericclioptions.IOStreams

	DryRun       bool
	ServerDryRun bool
	Recorder     recorders.OperationRecorder
	ShowDiff     bool
	ChartFlags   *ChartFlags
	DiffFlags    *DiffFlags
	DiffOptions  *DiffOptions

	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.CachedDiscoveryInterface
	OpenAPISchema   openapi.Resources
	Mapper          meta.RESTMapper
	BuilderFactory  func() *resource.Builder
	Serializer      chart.Serializer
	Visitor         *chart.Visitor
	HookExecutor    *HookExecutor
	Waiter          *wait.Waiter

	Namespace        string
	EnforceNamespace bool
}

func NewApplyOptions(streams genericclioptions.IOStreams) *ApplyOptions {
	return &ApplyOptions{
		IOStreams:  streams,
		ChartFlags: NewDefaultChartFlags(),
		DiffFlags:  NewDefaultDiffFlags(),
		Recorder:   recorders.NewOperationRecorder(),
		Serializer: yaml.NewSerializer(),
	}
}

func (o *ApplyOptions) Validate() error {
	if o.DryRun && o.ServerDryRun {
		return errors.Errorf("--dry-run and --server-dry-run can't be used together")
	}

	return nil
}

func (o *ApplyOptions) Complete(f genericclioptions.RESTClientGetter) error {
	var err error

	o.BuilderFactory = func() *resource.Builder {
		return resource.NewBuilder(f)
	}

	o.DiscoveryClient, err = f.ToDiscoveryClient()
	if err != nil {
		return err
	}

	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	o.DynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	o.OpenAPISchema, err = openapi.NewOpenAPIGetter(o.DiscoveryClient).Get()
	if err != nil {
		return err
	}

	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}

	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.Visitor, err = o.ChartFlags.ToVisitor(o.Namespace)
	if err != nil {
		return err
	}

	o.Waiter = wait.NewDefaultWaiter(o.IOStreams, o.DynamicClient)

	o.HookExecutor = &HookExecutor{
		IOStreams:      o.IOStreams,
		DryRun:         o.DryRun || o.ServerDryRun,
		DynamicClient:  o.DynamicClient,
		Mapper:         o.Mapper,
		BuilderFactory: o.BuilderFactory,
		Waiter:         o.Waiter,
	}

	if !o.ShowDiff {
		return nil
	}

	o.DiffOptions = &DiffOptions{
		IOStreams:      o.IOStreams,
		OpenAPISchema:  o.OpenAPISchema,
		BuilderFactory: o.BuilderFactory,
		Namespace:      o.Namespace,
		DiffPrinter:    o.DiffFlags.ToPrinter(),
		DryRunVerifier: &apply.DryRunVerifier{
			Finder:        cmdutil.NewCRDFinder(cmdutil.CRDFromDynamic(o.DynamicClient)),
			OpenAPIGetter: o.DiscoveryClient,
		},
		Serializer: o.Serializer,
		Visitor:    o.Visitor,
	}

	return nil
}

func (o *ApplyOptions) Run() error {
	err := o.Visitor.Visit(func(c *chart.Chart, err error) error {
		if err != nil {
			return err
		}

		if o.ShowDiff {
			err = o.DiffOptions.Diff(c)
			if err != nil {
				return err
			}
		}

		buf, err := o.Serializer.Encode(c.Resources.GetObjects())
		if err != nil {
			return err
		}

		// We need to use a tempfile here instead of a stream as
		// apply.ApplyOption requires that and we do not want to duplicate its
		// huge Run() method to override this.
		f, err := ioutil.TempFile("", c.Config.Name)
		if err != nil {
			return err
		}

		defer f.Close()

		err = ioutil.WriteFile(f.Name(), buf, 0644)
		if err != nil {
			return err
		}

		defer os.Remove(f.Name())

		err = o.HookExecutor.ExecHooks(c, chart.PreApplyHook)
		if err != nil {
			return err
		}

		applier := o.createApplier(c, f.Name())

		err = applier.Run()
		if err != nil {
			return err
		}

		return o.HookExecutor.ExecHooks(c, chart.PostApplyHook)
	})
	if err != nil {
		return err
	}

	return o.Recorder.Objects("pruned").Visit(func(obj runtime.Object, err error) error {
		if err != nil {
			return err
		}

		if !resources.IsOfKind(obj, resources.KindStatefulSet) {
			return nil
		}

		policy, err := chart.GetDeletionPolicy(obj)
		if err != nil || policy != chart.DeletionPolicyDeletePVCs {
			return err
		}

		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return errors.Errorf("illegal object type: %T", obj)
		}

		resourceDeleter := &ResourceDeleter{
			IOStreams:       o.IOStreams,
			DynamicClient:   o.DynamicClient,
			DryRun:          o.DryRun || o.ServerDryRun,
			WaitForDeletion: true,
			Waiter:          o.Waiter,
			Builder: o.BuilderFactory().
				Unstructured().
				ContinueOnError().
				NamespaceParam(u.GetNamespace()).DefaultNamespace().
				ResourceTypeOrNameArgs(true, resources.KindPersistentVolumeClaim).
				LabelSelector(chart.PersistentVolumeClaimSelector(u.GetName())),
		}

		return resourceDeleter.Delete()
	})
}

func (o *ApplyOptions) createApplier(c *chart.Chart, filename string) *apply.ApplyOptions {
	return &apply.ApplyOptions{
		IOStreams:    o.IOStreams,
		DryRun:       o.DryRun,
		ServerDryRun: o.ServerDryRun,
		Overwrite:    true,
		OpenAPIPatch: true,
		Prune:        true,
		Selector:     c.LabelSelector(),
		DeleteOptions: &delete.DeleteOptions{
			Cascade:         true,
			GracePeriod:     -1,
			ForceDeletion:   false,
			Timeout:         time.Duration(0),
			WaitForDeletion: false,
			FilenameOptions: resource.FilenameOptions{
				Filenames: []string{filename},
				Recursive: false,
			},
		},
		PrintFlags:       genericclioptions.NewPrintFlags(""),
		Recorder:         genericclioptions.NoopRecorder{},
		Validator:        validation.NullSchema{},
		Builder:          o.BuilderFactory(),
		DiscoveryClient:  o.DiscoveryClient,
		DynamicClient:    o.DynamicClient,
		OpenAPISchema:    o.OpenAPISchema,
		Mapper:           o.Mapper,
		Namespace:        o.Namespace,
		EnforceNamespace: o.EnforceNamespace,
		ToPrinter: func(operation string) (kprinters.ResourcePrinter, error) {
			printOperation := operation
			if o.DryRun {
				printOperation = fmt.Sprintf("%s (dry run)", operation)
			}
			if o.ServerDryRun {
				printOperation = fmt.Sprintf("%s (server dry run)", operation)
			}

			p := &kprinters.NamePrinter{Operation: printOperation}

			// Wrap the printer to keep track of the executed operations for
			// each object. We need that later on to perform additonal tasks.
			// Sadly, we have to do this for now to avoid duplicating most of
			// the logic of apply.ApplyOptions.
			return printers.NewRecordingPrinter(o.Recorder, operation, p), nil
		},
	}
}

// HookExecutor executes chart lifecycle hooks.
type HookExecutor struct {
	genericclioptions.IOStreams
	DynamicClient  dynamic.Interface
	BuilderFactory func() *resource.Builder
	Mapper         meta.RESTMapper
	Waiter         *wait.Waiter
	DryRun         bool
}

// ExecHooks executes hooks of hookType from chart c. It will attempt to delete
// job hooks matching a label selector that are already deployed to the cluster
// before creating the hooks to prevent errors.
func (e *HookExecutor) ExecHooks(c *chart.Chart, hookType string) error {
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

	err = hooks.EachItem(func(hook *chart.Hook) error {
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

func (e *HookExecutor) cleanupHooks(c *chart.Chart, hookType string) error {
	builder := e.BuilderFactory().
		Unstructured().
		ContinueOnError().
		RequireObject(false).
		AllNamespaces(true).
		ResourceTypeOrNameArgs(true, resources.KindJob).
		LabelSelector(c.HookLabelSelector(hookType))

	resourceDeleter := &ResourceDeleter{
		IOStreams:       e.IOStreams,
		DynamicClient:   e.DynamicClient,
		DryRun:          e.DryRun,
		WaitForDeletion: true,
		Waiter:          e.Waiter,
		Builder:         builder,
	}

	return resourceDeleter.Delete()
}

func (e *HookExecutor) waitForCompletion(infos []*resource.Info) error {
	req := &wait.Request{
		ConditionFn: wait.IsComplete,
		Options: wait.Options{
			Timeout:      24 * time.Hour,
			AllowFailure: true,
		},
		Visitor: resource.InfoListVisitor(infos),
	}

	err := e.Waiter.Wait(req)
	if apierrors.IsForbidden(err) || apierrors.IsMethodNotSupported(err) {
		// if we're forbidden from waiting, we shouldn't fail.
		// if the resource doesn't support a verb we need, we shouldn't fail.
		klog.V(1).Info(err)
		return nil
	}

	return err
}

// PrintHook prints a hooks.
func (e *HookExecutor) PrintHook(hook *chart.Hook, operation string) {
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
