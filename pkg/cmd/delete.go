package cmd

import (
	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/deletions"
	"github.com/martinohmann/kubectl-chart/pkg/hook"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/martinohmann/kubectl-chart/pkg/resources/statefulset"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

func NewDeleteCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDeleteOptions(streams)

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete resources from one or multiple helm charts",
		Long: templates.LongDesc(`
			Deletes resources of one or multiple helm charts from a cluster.`),
		Example: templates.Examples(`
			# Delete resources of a single chart
			kubectl chart delete -f ~/charts/mychart

			# Delete resources of multiple charts
			kubectl chart delete -f ~/charts --recursive

			# Dry run resource deletion
			kubectl chart delete -f ~/charts/mychart --dry-run

			# Skip executing pre and post-delete hooks
			kubectl chart delete -f ~/charts/mychart --no-hooks`),
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
	cmd.Flags().BoolVar(&o.Prune, "prune", o.Prune, "If true, chart resources will be pruned by their chart label. This also removes resources not present in the chart anymore")

	return cmd
}

type DeleteOptions struct {
	genericclioptions.IOStreams

	ChartFlags ChartFlags
	HookFlags  HookFlags
	PrintFlags PrintFlags
	DryRun     bool
	Prune      bool

	DynamicClient  dynamic.Interface
	Mapper         meta.RESTMapper
	Visitor        chart.Visitor
	HookExecutor   *chart.HookExecutor
	Deleter        deletions.Deleter
	ResourceFinder *resources.Finder
	PVCPruner      *statefulset.PersistentVolumeClaimPruner

	Namespace string
}

func NewDeleteOptions(streams genericclioptions.IOStreams) *DeleteOptions {
	return &DeleteOptions{
		IOStreams: streams,
	}
}

func (o *DeleteOptions) Complete(f cmdutil.Factory) error {
	var err error

	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.DynamicClient, err = f.DynamicClient()
	if err != nil {
		return err
	}

	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}

	if o.Prune {
		discoveryClient, err := f.ToDiscoveryClient()
		if err != nil {
			return err
		}

		o.ResourceFinder = resources.NewFinder(discoveryClient, o.DynamicClient, o.Mapper)
	}

	p := o.PrintFlags.ToPrinter(o.DryRun)

	o.Deleter = deletions.NewDeleter(o.IOStreams, o.DynamicClient, p, o.DryRun)

	if !o.HookFlags.NoHooks {
		o.HookExecutor = chart.NewHookExecutor(
			o.IOStreams,
			o.DynamicClient,
			o.Mapper,
			p,
			o.DryRun,
		)
	}

	visitor, err := o.ChartFlags.ToVisitor(o.Namespace)
	if err != nil {
		return err
	}

	o.Visitor = chart.NewReverseVisitor(visitor)

	o.PVCPruner = statefulset.NewPersistentVolumeClaimPruner(
		o.DynamicClient,
		o.Deleter,
		o.Mapper,
	)

	return err
}

func (o *DeleteOptions) Run() error {
	return o.Visitor.Visit(func(c *chart.Chart, err error) error {
		if err != nil {
			return err
		}

		return o.DeleteChart(c)
	})
}

func (o *DeleteOptions) getResourceInfos(c *chart.Chart) ([]*resource.Info, error) {
	if o.Prune {
		return o.ResourceFinder.FindByLabelSelector(chart.LabelSelector(c))
	}

	return resources.ToInfoList(c.Resources, o.Mapper)
}

func (o *DeleteOptions) DeleteChart(c *chart.Chart) error {
	infos, err := o.getResourceInfos(c)
	if err != nil || len(infos) == 0 {
		return err
	}

	resources.SortInfosByKind(infos, resources.DeleteOrder)

	err = o.HookExecutor.ExecHooks(c, hook.TypePreDelete)
	if err != nil {
		return err
	}

	err = o.Deleter.Delete(resource.InfoListVisitor(infos))
	if err != nil {
		return err
	}

	err = o.HookExecutor.ExecHooks(c, hook.TypePostDelete)
	if err != nil {
		return err
	}

	deletedObjs := resources.ToObjectList(infos)

	return o.PVCPruner.PruneClaims(deletedObjs)
}
