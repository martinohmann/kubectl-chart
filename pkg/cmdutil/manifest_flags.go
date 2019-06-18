package cmdutil

import (
	"strings"

	"github.com/spf13/cobra"
)

type ManifestFlags struct {
	Charts       string
	ChartsDir    string
	ManifestsDir string
	SkipSave     bool
	SkipHooks    bool
	FullDiff     bool
	Force        bool
}

func (f *ManifestFlags) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.ManifestsDir, "manifests-dir", "./manifests", "Path to rendered manifests")
	cmd.Flags().StringVar(&f.ChartsDir, "charts-dir", "./charts", "Path to cluster charts")
	cmd.Flags().StringVar(&f.Charts, "charts", f.Charts, "Comma separated list of charts to filter")
	cmd.Flags().BoolVar(&f.SkipSave, "skip-save", f.SkipSave, "Skip saving rendered manifests")
	cmd.Flags().BoolVar(&f.SkipHooks, "skip-hooks", f.SkipHooks, "Skip manifest hooks")
}

func (f *ManifestFlags) ChartNames() []string {
	charts := strings.Split(f.Charts, ",")
	for i, chart := range charts {
		charts[i] = strings.TrimSpace(chart)
	}

	return charts
}
