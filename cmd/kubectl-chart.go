package main

import (
	"os"

	"github.com/martinohmann/kubectl-chart/pkg/cmd"
	"github.com/martinohmann/kubectl-chart/pkg/cmdutil"
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var (
	rootCmd = &cobra.Command{
		Use:           "kubectl-chart",
		Short:         "",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
)

func main() {
	streams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	configFlags := genericclioptions.NewConfigFlags(true)

	configFlags.AddFlags(rootCmd.PersistentFlags())

	rootCmd.AddCommand(cmd.NewCmdApply(configFlags, streams))
	rootCmd.AddCommand(cmd.NewCmdVersion(streams))

	cmdutil.CheckErr(rootCmd.Execute())
}
