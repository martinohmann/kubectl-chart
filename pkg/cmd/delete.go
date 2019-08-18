package cmd

import (
	"bytes"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/deletions"
	"github.com/martinohmann/kubectl-chart/pkg/hook"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/martinohmann/kubectl-chart/pkg/resources/statefulset"
	"github.com/martinohmann/kubectl-chart/pkg/yaml"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

func NewDeleteCmd(f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
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
	DynamicClientGetter

	ChartFlags ChartFlags
	HookFlags  HookFlags
	PrintFlags PrintFlags
	DryRun     bool
	Prune      bool

	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.DiscoveryInterface
	BuilderFactory  func() *resource.Builder
	Mapper          meta.RESTMapper
	Encoder         resources.Encoder
	Visitor         chart.Visitor
	HookExecutor    *chart.HookExecutor
	Deleter         deletions.Deleter
	ResourceFinder  *resources.Finder

	Namespace        string
	EnforceNamespace bool
}

func NewDeleteOptions(streams genericclioptions.IOStreams) *DeleteOptions {
	return &DeleteOptions{
		IOStreams: streams,
		Encoder:   yaml.NewEncoder(),
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

	o.DynamicClient, err = o.DynamicClientGetter.Get(f)
	if err != nil {
		return err
	}

	o.DiscoveryClient, err = f.ToDiscoveryClient()
	if err != nil {
		return err
	}

	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}

	if o.Prune {
		o.ResourceFinder = resources.NewFinder(o.DiscoveryClient, o.DynamicClient, o.Mapper)
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

func (o *DeleteOptions) DeleteChart(c *chart.Chart) error {
	var err error
	var infos []*resource.Info

	if o.Prune {
		infos, err = o.ResourceFinder.FindByLabelSelector(chart.LabelSelector(c))
	} else {
		var buf []byte

		buf, err = o.Encoder.Encode(c.Resources)
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
		if err = result.Err(); err != nil {
			return err
		}

		infos, err = result.Infos()
	}

	if err != nil {
		return err
	}

	if len(infos) == 0 {
		return nil
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
	if len(deletedObjs) == 0 {
		return nil
	}

	pvcPruner := &statefulset.PersistentVolumeClaimPruner{
		Deleter:       o.Deleter,
		DynamicClient: o.DynamicClient,
		Mapper:        o.Mapper,
	}

	return pvcPruner.PruneClaims(deletedObjs)
}
