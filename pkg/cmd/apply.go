package cmd

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/deletions"
	"github.com/martinohmann/kubectl-chart/pkg/hook"
	"github.com/martinohmann/kubectl-chart/pkg/printers"
	"github.com/martinohmann/kubectl-chart/pkg/recorders"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/martinohmann/kubectl-chart/pkg/resources/statefulset"
	"github.com/martinohmann/kubectl-chart/pkg/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kprinters "k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubectl/pkg/cmd/apply"
	"k8s.io/kubectl/pkg/cmd/delete"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/openapi"
	"k8s.io/kubectl/pkg/validation"
)

var (
	// ErrIllegalDryRunFlagCombination is returned if mutual exclusive dry run
	// flags are set.
	ErrIllegalDryRunFlagCombination = errors.Errorf("--dry-run and --server-dry-run can't be used together")
)

func NewApplyCmd(f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewApplyOptions(streams)

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply resources from one or multiple helm charts",
		Long:  "Apply renders the resources of one or multiple helm charts and applies them to a cluster.",
		Example: `  # Render and apply a single chart
  kubectl chart apply -f ~/charts/mychart

  # Render and apply multiple charts with additional values merged
  kubectl chart apply -f ~/charts --recursive --values ~/some/additional/values.yaml

  # Dry run apply and print resource diffs
  kubectl chart apply -f ~/charts/mychart --diff --server-dry-run

  # Render and apply multiple charts with a chart filter
  kubectl chart apply -f ~/charts --recursive --chart-filter mychart

  # Skip executing pre and post-apply hooks
  kubectl chart apply -f ~/charts/mychart --no-hooks`,
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.ChartFlags.AddFlags(cmd)
	o.HookFlags.AddFlags(cmd)
	o.DiffFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.ServerDryRun, "server-dry-run", o.ServerDryRun, "If true, request will be sent to server with dry-run flag, which means the modifications won't be persisted. This is an alpha feature and flag.")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", o.DryRun, "If true, only print the object that would be sent, without sending it. Warning: --dry-run cannot accurately output the result of merging the local manifest and the server-side data. Use --server-dry-run to get the merged result instead.")
	cmd.Flags().BoolVar(&o.ShowDiff, "diff", o.ShowDiff, "If set, a diff for all resources will be displayed")
	cmd.Flags().BoolVar(&o.Prune, "prune", o.Prune, "If true, chart resources not present anymore in the rendered chart manifest will be pruned by their chart label.")

	return cmd
}

type ApplyOptions struct {
	genericclioptions.IOStreams

	ChartFlags   ChartFlags
	HookFlags    HookFlags
	DiffFlags    DiffFlags
	DiffOptions  *DiffOptions
	DryRun       bool
	ServerDryRun bool
	ShowDiff     bool
	Prune        bool

	Printer         printers.ContextPrinter
	Recorder        recorders.OperationRecorder
	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.DiscoveryInterface
	OpenAPISchema   openapi.Resources
	Mapper          meta.RESTMapper
	BuilderFactory  func() *resource.Builder
	Encoder         resources.Encoder
	Visitor         chart.Visitor
	HookExecutor    chart.HookExecutor
	Deleter         deletions.Deleter

	Namespace        string
	EnforceNamespace bool
}

func NewApplyOptions(streams genericclioptions.IOStreams) *ApplyOptions {
	return &ApplyOptions{
		IOStreams: streams,
		DiffFlags: NewDefaultDiffFlags(),
		Recorder:  recorders.NewOperationRecorder(),
		Encoder:   yaml.NewSerializer(),
		Prune:     true,
	}
}

func (o *ApplyOptions) Validate() error {
	if o.DryRun && o.ServerDryRun {
		return ErrIllegalDryRunFlagCombination
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

	if o.DynamicClient == nil {
		config, err := f.ToRESTConfig()
		if err != nil {
			return err
		}

		o.DynamicClient, err = dynamic.NewForConfig(config)
		if err != nil {
			return err
		}
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

	dryRun := o.DryRun || o.ServerDryRun

	o.Printer = o.DiffFlags.PrintFlags.ToPrinter(dryRun)

	o.Deleter = deletions.NewDeleter(
		o.IOStreams,
		o.DynamicClient,
		o.Printer.WithOperation("deleted"),
		dryRun,
	)

	if o.HookFlags.NoHooks {
		o.HookExecutor = &chart.NoopHookExecutor{}
	} else {
		o.HookExecutor = chart.NewHookExecutor(
			o.IOStreams,
			o.DynamicClient,
			o.Mapper,
			o.Printer,
			dryRun,
		)
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
		Encoder: o.Encoder,
		Visitor: o.Visitor,
	}

	return nil
}

func (o *ApplyOptions) Run() error {
	err := o.Visitor.Visit(func(c *chart.Chart, err error) error {
		if err != nil {
			return err
		}

		if len(c.Resources) == 0 {
			if !o.Prune {
				return nil
			}

			// try to prune deleted resources
			// TODO(mohmann): this needs refactoring
			deleteOptions := &DeleteOptions{
				DynamicClient:   o.DynamicClient,
				DiscoveryClient: o.DiscoveryClient,
				Mapper:          o.Mapper,
				DryRun:          o.DryRun || o.ServerDryRun,
				Deleter:         o.Deleter,
				HookExecutor:    o.HookExecutor,
				HookFlags:       HookFlags{NoHooks: true},
				Prune:           true,
			}

			return deleteOptions.DeleteChart(c)
		}

		if o.ShowDiff {
			err = o.DiffOptions.Diff(c)
			if err != nil {
				return err
			}
		}

		buf, err := o.Encoder.Encode(c.Resources)
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

		err = o.HookExecutor.ExecHooks(c, hook.PreApply)
		if err != nil {
			return err
		}

		applier := o.createApplier(c, f.Name())

		err = applier.Run()
		if err != nil {
			return err
		}

		return o.HookExecutor.ExecHooks(c, hook.PostApply)
	})
	if err != nil {
		return err
	}

	prunedObjs := o.Recorder.RecordedObjects("pruned")
	if len(prunedObjs) == 0 {
		return nil
	}

	pvcPruner := &statefulset.PersistentVolumeClaimPruner{
		Deleter:       o.Deleter,
		DynamicClient: o.DynamicClient,
		Mapper:        o.Mapper,
	}

	return pvcPruner.PruneClaims(prunedObjs)
}

func (o *ApplyOptions) createApplier(c *chart.Chart, filename string) *apply.ApplyOptions {
	return &apply.ApplyOptions{
		IOStreams:    o.IOStreams,
		DryRun:       o.DryRun,
		ServerDryRun: o.ServerDryRun,
		Overwrite:    true,
		OpenAPIPatch: true,
		Prune:        o.Prune,
		Selector:     chart.LabelSelector(c),
		DeleteOptions: &delete.DeleteOptions{
			Cascade:     true,
			GracePeriod: -1,
			Timeout:     time.Duration(0),
			FilenameOptions: resource.FilenameOptions{
				Filenames: []string{filename},
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
			p := o.Printer.WithOperation(operation)

			// Wrap the printer to keep track of the executed operations for
			// each object. We need that later on to perform additonal tasks.
			// Sadly, we have to do this for now to avoid duplicating most of
			// the logic of apply.ApplyOptions.
			return printers.NewRecordingPrinter(o.Recorder, operation, p), nil
		},
	}
}
