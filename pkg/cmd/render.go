package cmd

import (
	"fmt"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/hook"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/martinohmann/kubectl-chart/pkg/yaml"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

func NewRenderCmd(f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRenderOptions(streams)

	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render resources from one or multiple helm charts",
		Long: templates.LongDesc(`
			Renders resources of one or multiple helm charts.
			This can be used to preview the manifests that are sent to the cluster.`),
		Example: templates.Examples(`
			# Render a single chart
			kubectl chart render -f ~/charts/mychart

			# Render multiple charts
			kubectl chart render -f ~/charts --recursive

			# Render chart hooks
			kubectl chart render -f ~/charts/mychart --hook-type pre-apply

			# Render all chart hooks
			kubectl chart render -f ~/charts --recursive --hook-type all`),
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.ChartFlags.AddFlags(cmd)

	cmd.Flags().StringVar(&o.HookType, "hook-type", o.HookType, "If provided hooks with given type will be rendered. Specify 'all' to render all hooks.")

	return cmd
}

type RenderOptions struct {
	genericclioptions.IOStreams

	ChartFlags ChartFlags
	HookType   string

	Encoder resources.Encoder
	Visitor chart.Visitor
}

func NewRenderOptions(streams genericclioptions.IOStreams) *RenderOptions {
	return &RenderOptions{
		IOStreams: streams,
		Encoder:   yaml.NewEncoder(),
	}
}

func (o *RenderOptions) Validate() error {
	if o.HookType != "" && o.HookType != "all" && !hook.IsSupportedType(o.HookType) {
		return hook.NewUnsupportedTypeError(o.HookType)
	}

	return nil
}

func (o *RenderOptions) Complete(f genericclioptions.RESTClientGetter) error {
	namespace, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.Visitor, err = o.ChartFlags.ToVisitor(namespace)

	return err
}

func (o *RenderOptions) Run() error {
	return o.Visitor.Visit(func(c *chart.Chart, err error) error {
		if err != nil {
			return err
		}

		objs := o.selectResources(c)

		buf, err := o.Encoder.Encode(objs)
		if err != nil {
			return err
		}

		fmt.Fprint(o.Out, string(buf))

		return nil
	})
}

func (o *RenderOptions) selectResources(c *chart.Chart) []runtime.Object {
	if o.HookType == "" {
		return c.Resources
	}

	if o.HookType == "all" {
		return c.Hooks.All().ToObjectList()
	}

	return c.Hooks[o.HookType].ToObjectList()
}
