package main

import (
	"os"

	"github.com/martinohmann/kubectl-chart/pkg/cmd"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

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
	streams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	configFlags := genericclioptions.NewConfigFlags(true)

	configFlags.AddFlags(rootCmd.PersistentFlags())

	rootCmd.AddCommand(cmd.NewApplyCmd(configFlags, streams))
	rootCmd.AddCommand(cmd.NewDeleteCmd(configFlags, streams))
	rootCmd.AddCommand(cmd.NewRenderCmd(configFlags, streams))
	rootCmd.AddCommand(cmd.NewVersionCmd(streams))

	cmdutil.CheckErr(rootCmd.Execute())
}
