package cmd

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/yaml"
	"github.com/pkg/errors"
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

	cmd.Flags().StringVar(&o.ChartDir, "chart-dir", o.ChartDir, "Directory of the helm chart that should be rendered")
	cmd.Flags().StringVar(&o.HookType, "hook", o.HookType, "If provided hooks with given type will be rendered")
	cmd.Flags().BoolVarP(&o.Recursive, "recursive", "R", o.Recursive, "If set all charts in --chart-dir will be recursively rendered")
	cmd.Flags().StringArrayVar(&o.ValueFiles, "value-file", o.ValueFiles, "File that should be merged onto the chart values before rendering")

	return cmd
}

type RenderOptions struct {
	genericclioptions.IOStreams

	ChartDir   string
	Recursive  bool
	HookType   string
	ValueFiles []string
	Namespace  string

	chartProcessor *chart.Processor
	serializer     chart.Serializer
}

func NewRenderOptions(streams genericclioptions.IOStreams) *RenderOptions {
	return &RenderOptions{
		IOStreams:      streams,
		chartProcessor: chart.NewDefaultProcessor(),
		serializer:     yaml.NewSerializer(),
	}
}

func (o *RenderOptions) Validate() error {
	if o.HookType != "" && o.HookType != "all" && !chart.IsValidHookType(o.HookType) {
		return chart.HookTypeError{Type: o.HookType, Additional: []string{"all"}}
	}

	return nil
}

func (o *RenderOptions) Complete(f genericclioptions.RESTClientGetter) error {
	var err error

	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	return nil
}

func (o *RenderOptions) Run() error {
	values, err := chart.LoadValues(o.ValueFiles...)
	if err != nil {
		return err
	}

	configs, err := o.buildChartConfigs(values)
	if err != nil {
		return err
	}

	for _, config := range configs {
		resources, hooks, err := o.chartProcessor.Process(config)
		if err != nil {
			return errors.Wrapf(err, "while processing chart %q", config.Name)
		}

		objs, err := o.filterRenderResources(resources, hooks)
		if err != nil {
			return err
		}

		buf, err := o.serializer.Encode(objs)
		if err != nil {
			return nil
		}

		fmt.Fprint(o.Out, string(buf))
	}

	return err
}

func (o *RenderOptions) filterRenderResources(resources, hooks []runtime.Object) ([]runtime.Object, error) {
	if o.HookType == "" {
		return resources, nil
	}

	if o.HookType == "all" {
		return hooks, nil
	}

	return chart.FilterHooks(o.HookType, hooks...)
}

func (o *RenderOptions) buildChartConfigs(values map[string]interface{}) ([]*chart.Config, error) {
	configs := make([]*chart.Config, 0)

	if o.Recursive {
		infos, err := ioutil.ReadDir(o.ChartDir)
		if err != nil {
			return nil, err
		}

		for _, info := range infos {
			if !info.IsDir() {
				continue
			}

			configs = append(configs, &chart.Config{
				Dir:       filepath.Join(o.ChartDir, info.Name()),
				Name:      info.Name(),
				Namespace: o.Namespace,
				Values:    values,
			})
		}
	} else {
		configs = append(configs, &chart.Config{
			Dir:       o.ChartDir,
			Name:      filepath.Base(o.ChartDir),
			Namespace: o.Namespace,
			Values:    values,
		})
	}

	return configs, nil
}
