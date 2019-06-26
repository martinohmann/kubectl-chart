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

	o.ChartFlags.AddFlags(cmd)

	cmd.Flags().StringVar(&o.HookType, "hook", o.HookType, "If provided hooks with given type will be rendered")

	return cmd
}

type RenderOptions struct {
	genericclioptions.IOStreams
	*ChartFlags

	HookType  string
	Namespace string

	chartProcessor *chart.Processor
	Serializer     chart.Serializer
}

func NewRenderOptions(streams genericclioptions.IOStreams) *RenderOptions {
	return &RenderOptions{
		IOStreams:      streams,
		ChartFlags:     &ChartFlags{},
		chartProcessor: chart.NewDefaultProcessor(),
		Serializer:     yaml.NewSerializer(),
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
	return o.Visit(func(config *chart.Config, resources, hooks []runtime.Object, err error) error {
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

func (o *RenderOptions) Visit(fn func(config *chart.Config, resources, hooks []runtime.Object, err error) error) error {
	values, err := chart.LoadValues(o.ValueFiles...)
	if err != nil {
		return err
	}

	configs, err := o.buildChartConfigs(values)
	if err != nil {
		return err
	}

	for _, config := range configs {
		if len(o.ChartFilter) > 0 && !contains(o.ChartFilter, config.Name) {
			continue
		}

		resources, hooks, err := o.chartProcessor.Process(config)
		if err != nil {
			return errors.Wrapf(err, "while processing chart %q", config.Name)
		}

		if err != nil {
			if fnErr := fn(config, resources, hooks, err); fnErr != nil {
				return fnErr
			}
			continue
		}

		if err := fn(config, resources, hooks, nil); err != nil {
			return err
		}
	}

	return err
}

func (o *RenderOptions) selectResources(resources, hooks []runtime.Object) ([]runtime.Object, error) {
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

func contains(s []string, v string) bool {
	for _, u := range s {
		if u == v {
			return true
		}
	}

	return false
}
