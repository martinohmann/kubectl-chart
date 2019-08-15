package cmd

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/imdario/mergo"
	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/helm/pkg/chartutil"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func NewDumpValuesCmd(f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDumpValuesOptions(streams)

	cmd := &cobra.Command{
		Use:   "dump-values",
		Short: "Dump merged values for a chart",
		Long:  "This command dumps the merged values for the provided charts how they would be available in templates. This is useful for debugging.",
		Example: `  # Dump values for a single chart
  kubectl chart dump-values -f ~/charts/mychart

  # Dump values for multiple charts with additional values merged
  kubectl chart dump-values -f ~/charts --recursive --values ~/some/additional/values.yaml

  # Dump values for multiple charts with filter
  kubectl chart dump-values -f ~/charts --recursive --chart-filter mychart`,
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Run())
		},
	}

	o.AddFlags(cmd)

	return cmd
}

type DumpValuesOptions struct {
	genericclioptions.IOStreams
	ChartFlags
}

func NewDumpValuesOptions(streams genericclioptions.IOStreams) *DumpValuesOptions {
	return &DumpValuesOptions{
		IOStreams: streams,
	}
}

func (o *DumpValuesOptions) Complete(f genericclioptions.RESTClientGetter) error {
	var err error

	o.ChartDir, err = filepath.Abs(o.ChartDir)
	if err != nil {
		return err
	}

	return nil
}
func (o *DumpValuesOptions) Run() error {
	additionalValues, err := chart.LoadValues(o.ValueFiles...)
	if err != nil {
		return err
	}

	if !o.ChartFlags.Recursive {
		return o.Dump(filepath.Base(o.ChartDir), o.ChartDir, additionalValues)
	}

	infos, err := ioutil.ReadDir(o.ChartDir)
	if err != nil {
		return err
	}

	for _, info := range infos {
		if !info.IsDir() || !chart.Include(o.ChartFilter, info.Name()) {
			continue
		}

		chartDir := filepath.Join(o.ChartDir, info.Name())

		err = o.Dump(info.Name(), chartDir, additionalValues)
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *DumpValuesOptions) Dump(chartName, chartDir string, additionalValues map[interface{}]interface{}) error {
	ok, err := chartutil.IsChartDir(chartDir)
	if !ok {
		return err
	}

	values, err := chart.LoadValues(filepath.Join(chartDir, "values.yaml"))
	if err != nil {
		return err
	}

	chartValues, err := chart.ValuesForChart(chartName, additionalValues)
	if err != nil {
		return err
	}

	err = mergo.Merge(&values, chartValues, mergo.WithOverride)
	if err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "---\n# Merged values for chart: %s\n---\n", chartName)

	return yaml.NewEncoder(o.Out).Encode(values)
}
