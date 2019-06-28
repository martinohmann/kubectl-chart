package main

import (
	"os"
	"path/filepath"

	"github.com/martinohmann/kubectl-chart/pkg/cmd"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

func main() {
	streams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	command := cmd.NewDifferCmd(filepath.Base(os.Args[0]), streams)

	cmdutil.CheckErr(command.Execute())
}
