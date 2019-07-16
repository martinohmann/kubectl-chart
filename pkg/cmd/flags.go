package cmd

import (
	"path/filepath"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/diff"
	"github.com/martinohmann/kubectl-chart/pkg/printers"
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

func (f *ChartFlags) ToVisitor(namespace string) (chart.Visitor, error) {
	chartDir, err := filepath.Abs(f.ChartDir)
	if err != nil {
		return nil, err
	}

	options := chart.VisitorOptions{
		ChartDir:    chartDir,
		ChartFilter: f.ChartFilter,
		Recursive:   f.Recursive,
		ValueFiles:  f.ValueFiles,
		Namespace:   namespace,
	}

	return chart.NewVisitor(chart.NewDefaultProcessor(), options), nil
}

type DiffFlags struct {
	Context    int
	PrintFlags PrintFlags
}

func NewDefaultDiffFlags() DiffFlags {
	return DiffFlags{
		Context: 10,
	}
}

func (f *DiffFlags) AddFlags(cmd *cobra.Command) {
	f.PrintFlags.AddFlags(cmd)

	cmd.Flags().IntVar(&f.Context, "diff-context", f.Context, "Line context to display before and after each changed block")
}

func (f *DiffFlags) ToPrinter() diff.Printer {
	return diff.NewUnifiedPrinter(diff.Options{
		Color:   !f.PrintFlags.NoColor,
		Context: f.Context,
	})
}

type HookFlags struct {
	NoHooks bool
}

func (f *HookFlags) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.NoHooks, "no-hooks", f.NoHooks, "If set, no hooks will be executed")
}

type PrintFlags struct {
	NoColor bool
}

func (f *PrintFlags) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.NoColor, "no-color", f.NoColor, "If set, output will not be colored")
}

func (f *PrintFlags) ToPrinter(dryRun bool) printers.ContextPrinter {
	return printers.NewContextPrinter(!f.NoColor, dryRun)
}
