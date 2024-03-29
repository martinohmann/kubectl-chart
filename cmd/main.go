package main

import (
	"flag"
	"os"

	"github.com/martinohmann/kubectl-chart/pkg/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

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

func init() {
	klog.InitFlags(flag.CommandLine)
	flag.Set("logtostderr", "true")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
}

func main() {
	streams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	configFlags := genericclioptions.NewConfigFlags(true)

	configFlags.AddFlags(rootCmd.PersistentFlags())

	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	f := cmdutil.NewFactory(configFlags)

	rootCmd.AddCommand(cmd.NewApplyCmd(f, streams))
	rootCmd.AddCommand(cmd.NewDeleteCmd(f, streams))
	rootCmd.AddCommand(cmd.NewRenderCmd(f, streams))
	rootCmd.AddCommand(cmd.NewDiffCmd(f, streams))
	rootCmd.AddCommand(cmd.NewDumpValuesCmd(streams))
	rootCmd.AddCommand(cmd.NewVersionCmd(streams))

	cmdutil.CheckErr(rootCmd.Execute())
}
