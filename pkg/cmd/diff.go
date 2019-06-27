package cmd

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/pkg/kubectl/cmd/apply"
	"k8s.io/kubernetes/pkg/kubectl/cmd/diff"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi"
	"k8s.io/utils/exec"
)

func NewDiffCmd(f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDiffOptions(streams)

	cmd := &cobra.Command{
		Use:  "diff",
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Run())
		},
	}

	o.ChartFlags.AddFlags(cmd)

	return cmd
}

type DiffOptions struct {
	genericclioptions.IOStreams
	*RenderOptions

	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.CachedDiscoveryInterface
	OpenAPISchema   openapi.Resources
	DryRunVerifier  *apply.DryRunVerifier
	BuilderFactory  func() *resource.Builder

	EnforceNamespace bool
}

func NewDiffOptions(streams genericclioptions.IOStreams) *DiffOptions {
	return &DiffOptions{
		IOStreams:     streams,
		RenderOptions: NewRenderOptions(streams),
	}
}

func (o *DiffOptions) Complete(f genericclioptions.RESTClientGetter) error {
	var err error

	o.BuilderFactory = func() *resource.Builder {
		return resource.NewBuilder(f)
	}

	o.DiscoveryClient, err = f.ToDiscoveryClient()
	if err != nil {
		return err
	}

	config, err := f.ToRESTConfig()
	if err != nil {
	}

	o.DynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	o.DryRunVerifier = &apply.DryRunVerifier{
		Finder:        cmdutil.NewCRDFinder(cmdutil.CRDFromDynamic(o.DynamicClient)),
		OpenAPIGetter: o.DiscoveryClient,
	}

	o.OpenAPISchema, err = openapi.NewOpenAPIGetter(o.DiscoveryClient).Get()
	if err != nil {
		return err
	}

	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	return nil
}

func (o *DiffOptions) createDiffer(filename string) *diff.DiffOptions {
	return &diff.DiffOptions{
		ServerSideApply: false,
		ForceConflicts:  false,
		Diff: &diff.DiffProgram{
			Exec:      exec.New(),
			IOStreams: o.IOStreams,
		},
		FilenameOptions: resource.FilenameOptions{
			Filenames: []string{filename},
		},
		DynamicClient:    o.DynamicClient,
		OpenAPISchema:    o.OpenAPISchema,
		DiscoveryClient:  o.DiscoveryClient,
		DryRunVerifier:   o.DryRunVerifier,
		CmdNamespace:     o.Namespace,
		EnforceNamespace: o.EnforceNamespace,
		Builder:          o.BuilderFactory(),
	}
}

func (o *DiffOptions) Run() error {
	diffResources := make([]runtime.Object, 0)

	err := o.Visit(func(config *chart.Config, resources, hooks []runtime.Object, err error) error {
		if err != nil {
			return err
		}

		fmt.Fprintf(o.Out, "%#v\n", resources)

		diffResources = append(diffResources, resources...)

		return nil
	})
	if err != nil {
		return err
	}

	buf, err := o.Serializer.Encode(diffResources)
	if err != nil {
		return err
	}

	// We need to use a tempfile here instead of a stream as
	// apply.ApplyOption requires that and we do not want to duplicate its
	// huge Run() method to override this.
	f, err := ioutil.TempFile("", "kubectl-chart")
	if err != nil {
		return err
	}

	defer f.Close()

	err = ioutil.WriteFile(f.Name(), buf, 0644)
	if err != nil {
		return err
	}

	defer os.Remove(f.Name())

	os.Setenv("KUBECTL_EXTERNAL_DIFF", "kubectl-chart-internaldiff")

	differ := o.createDiffer(f.Name())

	return differ.Run()
}
