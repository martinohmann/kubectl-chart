package cmd

import (
	"fmt"

	"github.com/martinohmann/kubectl-chart/pkg/cmdutil"
	"github.com/martinohmann/kubectl-chart/pkg/template"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

func NewCmdApply(f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	flags := NewApplyFlags(f, streams)

	cmd := &cobra.Command{
		Use: "apply",
		Run: func(cmd *cobra.Command, args []string) {
			o, err := flags.ToOptions(args)
			cmdutil.CheckErr(err)
			cmdutil.CheckErr(o.Run())
		},
	}

	flags.AddFlags(cmd)

	return cmd
}

type ApplyFlags struct {
	RESTClientGetter genericclioptions.RESTClientGetter

	ManifestFlags   *cmdutil.ManifestFlags
	ValuesFileFlags *cmdutil.ValuesFileFlags
	DryRun          bool
	Force           bool

	genericclioptions.IOStreams
}

func NewApplyFlags(restClientGetter genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *ApplyFlags {
	return &ApplyFlags{
		RESTClientGetter: restClientGetter,
		ManifestFlags:    &cmdutil.ManifestFlags{},
		ValuesFileFlags:  &cmdutil.ValuesFileFlags{},
		IOStreams:        streams,
	}
}

func (f *ApplyFlags) AddFlags(cmd *cobra.Command) {
	f.ManifestFlags.AddFlags(cmd)
	f.ValuesFileFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&f.DryRun, "dry-run", f.DryRun, "Only print changes.")
	cmd.Flags().BoolVar(&f.Force, "force", f.Force, "Force apply of all resources, even unchanged.")
}

func (f *ApplyFlags) ToOptions(args []string) (*ApplyOptions, error) {
	clientConfig, err := f.RESTClientGetter.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	discoveryClient, err := f.RESTClientGetter.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	valueLoader, err := f.ValuesFileFlags.ToValueLoader()
	if err != nil {
		return nil, err
	}

	return &ApplyOptions{
		Builder:         resource.NewBuilder(f.RESTClientGetter),
		DynamicClient:   dynamicClient,
		DiscoveryClient: discoveryClient,
		ValueLoader:     valueLoader,
		IOStreams:       f.IOStreams,
		DryRun:          f.DryRun,
		Force:           f.Force,
		Charts:          f.ManifestFlags.ChartNames(),
		ChartsDir:       f.ManifestFlags.ChartsDir,
		ManifestsDir:    f.ManifestFlags.ManifestsDir,
	}, nil
}

type ApplyOptions struct {
	Builder         *resource.Builder
	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.CachedDiscoveryInterface
	ValueLoader     template.ValueLoader

	DryRun       bool
	Force        bool
	Charts       []string
	ChartsDir    string
	ManifestsDir string

	genericclioptions.IOStreams
}

func (o *ApplyOptions) Run() error {
	fmt.Fprintf(o.Out, "apply")

	values, err := o.ValueLoader.LoadValues()
	if err != nil {
		return err
	}

	_ = values

	return nil
}
