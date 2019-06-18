package cmdutil

import (
	"path/filepath"
	"strings"

	"github.com/martinohmann/kubectl-chart/pkg/template"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type ValuesFileFlags struct {
	Filename string
}

func (f *ValuesFileFlags) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.Filename, "values-file", "values.yaml", "Path to chart values file")
}

func (f *ValuesFileFlags) ToValueLoader() (template.ValueLoader, error) {
	extension := filepath.Ext(strings.ToLower(f.Filename))

	switch extension {
	case ".yaml", ".yml":
		return template.NewYAMLValueLoader(f.Filename), nil
	default:
		return nil, errors.Errorf("unsupported values file extension %q", extension)
	}
}
