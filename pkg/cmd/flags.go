package cmd

import (
	"github.com/martinohmann/kubectl-chart/pkg/diff"
	"github.com/spf13/cobra"
)

type ChartFlags struct {
	ChartDir    string
	ChartFilter []string
	Recursive   bool
	ValueFiles  []string
}

func (f *ChartFlags) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.ChartDir, "chart-dir", f.ChartDir, "Directory of the helm chart that should be rendered. If not set the current directory is assumed")
	cmd.Flags().StringSliceVar(&f.ChartFilter, "chart-filter", f.ChartFilter, "If set only render filtered charts")
	cmd.Flags().BoolVarP(&f.Recursive, "recursive", "R", f.Recursive, "If set all charts in --chart-dir will be recursively rendered")
	cmd.Flags().StringArrayVar(&f.ValueFiles, "value-file", f.ValueFiles, "File that should be merged onto the chart values before rendering")
}

type DiffFlags struct {
	NoColor bool
	Context int
}

func NewDefaultDiffFlags() *DiffFlags {
	return &DiffFlags{
		Context: 10,
	}
}

func (f *DiffFlags) AddFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&f.Context, "diff-context", f.Context, "Line context to display before and after each changed block")
	cmd.Flags().BoolVar(&f.NoColor, "no-diff-color", f.NoColor, "Do not color diff output")
}

func (f *DiffFlags) ToPrinter() diff.Printer {
	return diff.NewPrinter(diff.Options{
		Color:   !f.NoColor,
		Context: f.Context,
	})
}
