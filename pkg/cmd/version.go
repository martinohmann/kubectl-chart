package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/martinohmann/kubectl-chart/pkg/version"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

func NewVersionCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewVersionOptions(streams)

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Displays the version of kubectl-chart",
		Example: `  # Short version
  kubectl chart version --short

  # YAML Output format
  kubectl chart version --output yaml`,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().BoolVar(&o.Short, "short", false, "Display short version")
	cmd.Flags().StringVar(&o.Output, "output", o.Output, "Output format")

	return cmd
}

type VersionOptions struct {
	genericclioptions.IOStreams

	Short  bool
	Output string
}

func NewVersionOptions(streams genericclioptions.IOStreams) *VersionOptions {
	return &VersionOptions{
		IOStreams: streams,
	}
}

func (o *VersionOptions) Validate() error {
	if o.Output != "" && o.Output != "yaml" && o.Output != "json" {
		return errors.New("--output must be 'yaml' or 'json'")
	}

	return nil
}

func (o *VersionOptions) Run() error {
	v := version.Get()

	if o.Short {
		fmt.Fprintln(o.Out, v.GitVersion)
		return nil
	}

	switch o.Output {
	case "json":
		buf, err := json.Marshal(v)
		if err != nil {
			return err
		}

		fmt.Fprintln(o.Out, string(buf))
	case "yaml":
		buf, err := yaml.Marshal(v)
		if err != nil {
			return err
		}

		fmt.Fprintln(o.Out, string(buf))
	default:
		fmt.Fprintf(o.Out, "%#v\n", v)
	}

	return nil
}
