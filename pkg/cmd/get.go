package cmd

import (
	"fmt"
	"strings"

	"github.com/martinohmann/kubectl-chart/internal/kubernetes/pkg/kubectl/cmd/get"
	"github.com/martinohmann/kubectl-chart/pkg/meta"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func NewGetCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewGetOptions(f)

	cmd := get.NewCmdGet("kubectl", f, streams)

	cmd.Example = strings.Replace(cmd.Example, "kubectl", "kubectl chart", -1)
	cmd.PreRun = o.PrepareCommand

	cmd.Flags().MarkHidden("selector")
	cmd.Flags().MarkHidden("all-namespaces")
	cmd.Flags().Set("all-namespaces", "true")

	cmd.Flags().StringArrayVarP(&o.ChartNames, "chart-name", "c", o.ChartNames, "Name of the chart to display resources for. If empty, resources for all known charts are displayed.")

	return cmd
}

type GetOptions struct {
	genericclioptions.RESTClientGetter

	ChartNames []string
}

func NewGetOptions(f genericclioptions.RESTClientGetter) *GetOptions {
	return &GetOptions{
		RESTClientGetter: f,
	}
}

func (o *GetOptions) PrepareCommand(cmd *cobra.Command, _ []string) {
	o.setupSelector(cmd)
	o.setupNamespace(cmd)
}

func (o *GetOptions) setupSelector(cmd *cobra.Command) {
	selector := meta.LabelChartName

	if len(o.ChartNames) > 0 {
		selector = fmt.Sprintf("%s in (%s)", meta.LabelChartName, strings.Join(o.ChartNames, ","))
	}

	cmd.Flags().Set("selector", selector)
}

func (o *GetOptions) setupNamespace(cmd *cobra.Command) {
	namespace, enforceNamespace, _ := o.ToRawKubeConfigLoader().Namespace()
	if namespace != "" && enforceNamespace {
		cmd.Flags().Set("all-namespaces", "false")
	}
}
