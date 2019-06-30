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
		Use:  "render",
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
	ChartFlags *ChartFlags

	HookType string
	// Namespace string

	Serializer chart.Serializer
	Visitor    *chart.Visitor
}

func NewRenderOptions(streams genericclioptions.IOStreams) *RenderOptions {
	return &RenderOptions{
		IOStreams:  streams,
		ChartFlags: &ChartFlags{},
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
	return o.Visitor.Visit(func(config *chart.Config, resources, hooks []runtime.Object, err error) error {
		if err != nil {
			return err
		}

		objs, err := o.selectResources(resources, hooks)
		if err != nil {
			return err
		}

		buf, err := o.Serializer.Encode(objs)
		if err != nil {
			return err
		}

		fmt.Fprint(o.Out, string(buf))

		return nil
	})
}

// func (o *RenderOptions) Visit(fn func(config *chart.Config, resources, hooks []runtime.Object, err error) error) error {
// 	return o.Visitor.Visit(fn)
// }

func (o *RenderOptions) selectResources(resources, hooks []runtime.Object) ([]runtime.Object, error) {
	if o.HookType == "" {
		return resources, nil
	}

	if o.HookType == "all" {
		return hooks, nil
	}

	return chart.FilterHooks(o.HookType, hooks...)
}
