package cmd

import (
	"bytes"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/deletions"
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
		Use:  "delete",
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Run())
		},
	}

	o.ChartFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.DryRun, "dry-run", o.DryRun, "If true, only print the object that would be sent, without sending it. Warning: --dry-run cannot accurately output the result of merging the local manifest and the server-side data. Use --server-dry-run to get the merged result instead.")
	cmd.Flags().BoolVar(&o.Prune, "prune", o.Prune, "If true, all resources matching the chart selector will be pruned, even those previously removed from the chart.")

	return cmd
}

type DeleteOptions struct {
	genericclioptions.IOStreams

	DryRun     bool
	Prune      bool
	ChartFlags *ChartFlags

	DynamicClient  dynamic.Interface
	BuilderFactory func() *resource.Builder
	Mapper         meta.RESTMapper
	Serializer     chart.Serializer
	Visitor        *chart.Visitor
	HookExecutor   *chart.HookExecutor
	Deleter        deletions.Deleter
	Waiter         wait.Waiter

	Namespace        string
	EnforceNamespace bool
}

func NewDeleteOptions(streams genericclioptions.IOStreams) *DeleteOptions {
	return &DeleteOptions{
		IOStreams:  streams,
		ChartFlags: NewDefaultChartFlags(),
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

	o.Waiter = wait.NewDefaultWaiter(o.IOStreams)
	o.Deleter = deletions.NewDeleter(o.IOStreams, o.DynamicClient)

	o.HookExecutor = &chart.HookExecutor{
		IOStreams:      o.IOStreams,
		DryRun:         o.DryRun,
		DynamicClient:  o.DynamicClient,
		Mapper:         o.Mapper,
		BuilderFactory: o.BuilderFactory,
		Waiter:         o.Waiter,
		Deleter:        o.Deleter,
	}

	o.Visitor, err = o.ChartFlags.ToVisitor(o.Namespace)

	return err
}

func (o *DeleteOptions) Run() error {
	return o.Visitor.Visit(func(c *chart.Chart, err error) error {
		if err != nil {
			return err
		}

		builder := o.BuilderFactory().
			Unstructured().
			ContinueOnError().
			RequireObject(false)

		if o.Prune {
			builder = builder.
				AllNamespaces(true).
				ResourceTypeOrNameArgs(true, "all").
				LabelSelector(c.LabelSelector())
		} else {
			buf, err := o.Serializer.Encode(c.Resources.GetObjects())
			if err != nil {
				return err
			}

			builder = builder.
				NamespaceParam(o.Namespace).DefaultNamespace().
				Stream(bytes.NewBuffer(buf), c.Config.Name)
		}

		result := builder.
			Flatten().
			Do().
			IgnoreErrors(errors.IsNotFound)
		if err := result.Err(); err != nil {
			return err
		}

		err = o.HookExecutor.ExecHooks(c, chart.PreDeleteHook)
		if err != nil {
			return err
		}

		err = o.Deleter.Delete(&deletions.Request{
			DryRun:  o.DryRun,
			Waiter:  o.Waiter,
			Visitor: result,
		})
		if err != nil {
			return err
		}

		return o.HookExecutor.ExecHooks(c, chart.PostDeleteHook)
	})
}
