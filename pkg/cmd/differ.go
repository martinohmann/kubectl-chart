package cmd

import (
	"fmt"
	"os"

	"github.com/martinohmann/kubectl-chart/pkg/filediff"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewDifferCmd(name string, streams genericclioptions.IOStreams) *cobra.Command {
	return &cobra.Command{
		Use:    fmt.Sprintf("%s [from] [to]", name),
		Hidden: true,
		Args:   cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			differ := filediff.NewDiffer(args[0], args[1])

			n, err := differ.WriteTo(streams.Out)
			if err != nil {
				fmt.Fprintln(streams.ErrOut, err)
				os.Exit(2)
			}

			if n > 0 {
				// Exit with status 1 to indicate that there are changes
				os.Exit(1)
			}
		},
	}
}
