package cmd

import (
	"fmt"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/yaml"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

func NewRenderCmd(f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRenderOptions(streams)

	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render resources from one or multiple helm charts",
		Long:  "Renders resources of one or multiple helm charts. This can be used to preview the manifests that are sent to the cluster.",
		Example: `  # Render a single chart
  kubectl chart render --chart-dir ~/charts/mychart

  # Render multiple charts
  kubectl chart render --chart-dir ~/charts --recursive

  # Render chart hooks
  kubectl chart render --chart-dir ~/charts/mychart --hook pre-apply

  # Render all chart hooks
  kubectl chart render --chart-dir ~/charts --recursive --hook all`,
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.ChartFlags.AddFlags(cmd)

	cmd.Flags().StringVar(&o.HookType, "hook", o.HookType, "If provided hooks with given type will be rendered")

	return cmd
}

type RenderOptions struct {
	genericclioptions.IOStreams

	ChartFlags ChartFlags
	HookType   string

	Serializer chart.Serializer
	Visitor    chart.Visitor
}

func NewRenderOptions(streams genericclioptions.IOStreams) *RenderOptions {
	return &RenderOptions{
		IOStreams:  streams,
		Serializer: yaml.NewSerializer(),
	}
}

func (o *RenderOptions) Validate() error {
	if o.HookType != "" && o.HookType != "all" && !chart.IsValidHookType(o.HookType) {
		return chart.HookTypeError{Type: o.HookType, Additional: []string{"all"}}
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

		buf, err := o.Serializer.Encode(objs)
		if err != nil {
			return err
		}

		fmt.Fprint(o.Out, string(buf))

		return nil
	})
}

func (o *RenderOptions) selectResources(c *chart.Chart) []runtime.Object {
	if o.HookType == "" {
		return c.Resources.GetObjects()
	}

	if o.HookType == "all" {
		return c.Hooks.GetObjects()
	}

	return c.Hooks.Type(o.HookType).GetObjects()
}
