package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/printers"
	"github.com/martinohmann/kubectl-chart/pkg/recorders"
	"github.com/martinohmann/kubectl-chart/pkg/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kprinters "k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/pkg/kubectl/cmd/apply"
	"k8s.io/kubernetes/pkg/kubectl/cmd/delete"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi"
	"k8s.io/kubernetes/pkg/kubectl/validation"
)

func NewApplyCmd(f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewApplyOptions(streams)

	cmd := &cobra.Command{
		Use:  "apply",
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Run())
		},
	}

	o.ChartFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.ServerDryRun, "server-dry-run", o.ServerDryRun, "If true, request will be sent to server with dry-run flag, which means the modifications won't be persisted. This is an alpha feature and flag.")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", o.DryRun, "If true, only print the object that would be sent, without sending it. Warning: --dry-run cannot accurately output the result of merging the local manifest and the server-side data. Use --server-dry-run to get the merged result instead.")

	return cmd
}

type ApplyOptions struct {
	genericclioptions.IOStreams
	*ChartFlags

	DryRun       bool
	ServerDryRun bool

	RenderOptions *RenderOptions
	Recorder      recorders.OperationRecorder
	Serializer    chart.Serializer

	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.CachedDiscoveryInterface
	OpenAPISchema   openapi.Resources
	Mapper          meta.RESTMapper
	BuilderFactory  func() *resource.Builder

	Namespace        string
	EnforceNamespace bool
}

func NewApplyOptions(streams genericclioptions.IOStreams) *ApplyOptions {
	return &ApplyOptions{
		IOStreams:     streams,
		ChartFlags:    &ChartFlags{},
		RenderOptions: NewRenderOptions(streams),
		Recorder:      recorders.NewOperationRecorder(),
		Serializer:    yaml.NewSerializer(),
	}
}

func (o *ApplyOptions) Complete(f genericclioptions.RESTClientGetter) error {
	if o.DryRun && o.ServerDryRun {
		return errors.Errorf("--dry-run and --server-dry-run can't be used together")
	}

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

	o.OpenAPISchema, err = openapi.NewOpenAPIGetter(o.DiscoveryClient).Get()
	if err != nil {
		return err
	}

	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}

	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.RenderOptions.ChartFlags = o.ChartFlags
	o.RenderOptions.Namespace = o.Namespace

	return nil
}

func (o *ApplyOptions) Run() error {
	err := o.RenderOptions.Visit(func(config *chart.Config, resources, hooks []runtime.Object, err error) error {
		if err != nil {
			return err
		}

		buf, err := o.Serializer.Encode(resources)
		if err != nil {
			return err
		}

		f, err := ioutil.TempFile("", config.Name)
		if err != nil {
			return err
		}

		defer os.Remove(f.Name())

		err = ioutil.WriteFile(f.Name(), buf, 0644)
		if err != nil {
			return err
		}

		applier := o.createApplier(&applyConfig{
			ChartName: config.Name,
			Filename:  f.Name(),
		})

		return applier.Run()
	})

	objs := o.Recorder.GetRecordedObjects()

	fmt.Fprintf(o.Out, "%#v\n", objs)

	return err
}

type applyConfig struct {
	ChartName string
	Filename  string
}

func (o *ApplyOptions) createApplier(config *applyConfig) *apply.ApplyOptions {
	return &apply.ApplyOptions{
		IOStreams:    o.IOStreams,
		DryRun:       o.DryRun,
		ServerDryRun: o.ServerDryRun,
		Overwrite:    true,
		OpenAPIPatch: true,
		Prune:        true,
		Selector:     fmt.Sprintf("%s=%s", chart.LabelName, config.ChartName),
		DeleteOptions: &delete.DeleteOptions{
			Cascade:         true,
			GracePeriod:     -1,
			ForceDeletion:   false,
			Timeout:         time.Duration(0),
			WaitForDeletion: false,
			FilenameOptions: resource.FilenameOptions{
				Filenames: []string{config.Filename},
				Recursive: false,
			},
		},
		PrintFlags:       genericclioptions.NewPrintFlags(""),
		Recorder:         genericclioptions.NoopRecorder{},
		Validator:        validation.NullSchema{},
		Builder:          o.BuilderFactory(),
		DiscoveryClient:  o.DiscoveryClient,
		DynamicClient:    o.DynamicClient,
		OpenAPISchema:    o.OpenAPISchema,
		Mapper:           o.Mapper,
		Namespace:        o.Namespace,
		EnforceNamespace: o.EnforceNamespace,
		// allow for a success message operation to be specified at print time
		ToPrinter: func(operation string) (kprinters.ResourcePrinter, error) {
			printOperation := operation
			if o.DryRun {
				printOperation = fmt.Sprintf("%s (dry run)", operation)
			}
			if o.ServerDryRun {
				printOperation = fmt.Sprintf("%s (server dry run)", operation)
			}

			p := &kprinters.NamePrinter{Operation: printOperation}

			return printers.NewRecordingPrinter(o.Recorder, operation, p), nil
		},
	}
}
