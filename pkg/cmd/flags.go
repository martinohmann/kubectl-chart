package cmd

import "github.com/spf13/cobra"

type ChartFlags struct {
	ChartDir    string
	ChartFilter []string
	Recursive   bool
	ValueFiles  []string
}

func (f *ChartFlags) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.ChartDir, "chart-dir", f.ChartDir, "Directory of the helm chart that should be rendered")
	cmd.Flags().StringSliceVar(&f.ChartFilter, "chart-filter", f.ChartFilter, "If set only render filtered charts")
	cmd.Flags().BoolVarP(&f.Recursive, "recursive", "R", f.Recursive, "If set all charts in --chart-dir will be recursively rendered")
	cmd.Flags().StringArrayVar(&f.ValueFiles, "value-file", f.ValueFiles, "File that should be merged onto the chart values before rendering")
}
