package cmd

import (
	"bytes"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/deletions"
	"github.com/martinohmann/kubectl-chart/pkg/hooks"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/martinohmann/kubectl-chart/pkg/wait"
	"github.com/martinohmann/kubectl-chart/pkg/yaml"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

func NewDeleteCmd(f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDeleteOptions(streams)

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete resources from one or multiple helm charts",
		Long:  "Deletes resources of one or multiple helm charts from a cluster.",
		Example: `  # Delete resources of a single chart
  kubectl chart delete --chart-dir ~/charts/mychart

  # Delete resources of multiple charts
  kubectl chart delete --chart-dir ~/charts --recursive

  # Dry run resource deletion
  kubectl chart delete --chart-dir ~/charts/mychart --dry-run

  # Skip executing pre and post-delete hooks
  kubectl chart delete --chart-dir ~/charts/mychart --no-hooks`,
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Run())
		},
	}

	o.ChartFlags.AddFlags(cmd)
	o.HookFlags.AddFlags(cmd)
	o.PrintFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.DryRun, "dry-run", o.DryRun, "If true, only print the object that would be sent, without sending it. Warning: --dry-run cannot accurately output the result of merging the local manifest and the server-side data. Use --server-dry-run to get the merged result instead.")

	return cmd
}

type DeleteOptions struct {
	genericclioptions.IOStreams

	ChartFlags ChartFlags
	HookFlags  HookFlags
	PrintFlags PrintFlags
	DryRun     bool

	DynamicClient  dynamic.Interface
	BuilderFactory func() *resource.Builder
	Mapper         meta.RESTMapper
	Serializer     chart.Serializer
	Visitor        chart.Visitor
	HookExecutor   hooks.Executor
	Deleter        deletions.Deleter

	Namespace        string
	EnforceNamespace bool
}

func NewDeleteOptions(streams genericclioptions.IOStreams) *DeleteOptions {
	return &DeleteOptions{
		IOStreams:  streams,
		Serializer: yaml.NewSerializer(),
	}
}

func (o *DeleteOptions) Complete(f genericclioptions.RESTClientGetter) error {
	var err error

	o.BuilderFactory = func() *resource.Builder {
		return resource.NewBuilder(f)
	}

	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	config, err := f.ToRESTConfig()
	if err != nil {
	}

	o.DynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}

	p := o.PrintFlags.ToPrinter(o.DryRun)

	o.Deleter = deletions.NewDeleter(o.IOStreams, o.DynamicClient, p, o.DryRun)

	if o.HookFlags.NoHooks {
		o.HookExecutor = &hooks.NoopExecutor{}
	} else {
		o.HookExecutor = &chart.HookExecutor{
			IOStreams:     o.IOStreams,
			DryRun:        o.DryRun,
			DynamicClient: o.DynamicClient,
			Mapper:        o.Mapper,
			Waiter:        wait.NewDefaultWaiter(o.IOStreams),
			Deleter:       o.Deleter,
			Printer:       p,
		}
	}

	visitor, err := o.ChartFlags.ToVisitor(o.Namespace)
	if err != nil {
		return err
	}

	o.Visitor = chart.NewReverseVisitor(visitor)

	return err
}

func (o *DeleteOptions) Run() error {
	return o.Visitor.Visit(func(c *chart.Chart, err error) error {
		if err != nil {
			return err
		}

		chart.SortResources(c.Resources, chart.DeleteOrder)

		buf, err := o.Serializer.Encode(c.Resources.GetObjects())
		if err != nil {
			return err
		}

		result := o.BuilderFactory().
			Unstructured().
			ContinueOnError().
			NamespaceParam(o.Namespace).DefaultNamespace().
			Stream(bytes.NewBuffer(buf), c.Config.Name).
			Flatten().
			Do().
			IgnoreErrors(errors.IsNotFound)
		if err := result.Err(); err != nil {
			return err
		}

		infos, err := result.Infos()
		if err != nil {
			return err
		}

		if len(infos) == 0 {
			return nil
		}

		err = o.HookExecutor.ExecHooks(c, chart.PreDeleteHook)
		if err != nil {
			return err
		}

		err = o.Deleter.Delete(resource.InfoListVisitor(infos))
		if err != nil {
			return err
		}

		err = o.HookExecutor.ExecHooks(c, chart.PostDeleteHook)
		if err != nil {
			return err
		}

		deletedObjs := resources.ToObjectList(infos)
		if len(deletedObjs) == 0 {
			return nil
		}

		pvcPruner := &chart.PersistentVolumeClaimPruner{
			Deleter:       o.Deleter,
			DynamicClient: o.DynamicClient,
			Mapper:        o.Mapper,
		}

		return pvcPruner.PruneClaims(deletedObjs)
	})
}
