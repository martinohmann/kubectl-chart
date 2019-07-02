package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/printers"
	"github.com/martinohmann/kubectl-chart/pkg/recorders"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/martinohmann/kubectl-chart/pkg/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.ChartFlags.AddFlags(cmd)
	o.DiffFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.ServerDryRun, "server-dry-run", o.ServerDryRun, "If true, request will be sent to server with dry-run flag, which means the modifications won't be persisted. This is an alpha feature and flag.")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", o.DryRun, "If true, only print the object that would be sent, without sending it. Warning: --dry-run cannot accurately output the result of merging the local manifest and the server-side data. Use --server-dry-run to get the merged result instead.")
	cmd.Flags().BoolVar(&o.ShowDiff, "diff", o.ShowDiff, "If set, a diff for all resources will be displayed")

	return cmd
}

type ApplyOptions struct {
	genericclioptions.IOStreams

	DryRun       bool
	ServerDryRun bool
	Recorder     recorders.OperationRecorder
	ShowDiff     bool
	ChartFlags   *ChartFlags
	DiffFlags    *DiffFlags
	DiffOptions  *DiffOptions

	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.CachedDiscoveryInterface
	OpenAPISchema   openapi.Resources
	Mapper          meta.RESTMapper
	BuilderFactory  func() *resource.Builder
	Serializer      chart.Serializer
	Visitor         *chart.Visitor

	Namespace        string
	EnforceNamespace bool
}

func NewApplyOptions(streams genericclioptions.IOStreams) *ApplyOptions {
	return &ApplyOptions{
		IOStreams:  streams,
		ChartFlags: NewDefaultChartFlags(),
		DiffFlags:  NewDefaultDiffFlags(),
		Recorder:   recorders.NewOperationRecorder(),
		Serializer: yaml.NewSerializer(),
	}
}

func (o *ApplyOptions) Validate() error {
	if o.DryRun && o.ServerDryRun {
		return errors.Errorf("--dry-run and --server-dry-run can't be used together")
	}

	return nil
}

func (o *ApplyOptions) Complete(f genericclioptions.RESTClientGetter) error {
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
		return err
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

	o.Visitor, err = o.ChartFlags.ToVisitor(o.Namespace)
	if err != nil {
		return err
	}

	if !o.ShowDiff {
		return nil
	}

	o.DiffOptions = &DiffOptions{
		IOStreams:      o.IOStreams,
		OpenAPISchema:  o.OpenAPISchema,
		BuilderFactory: o.BuilderFactory,
		Namespace:      o.Namespace,
		DiffPrinter:    o.DiffFlags.ToPrinter(),
		DryRunVerifier: &apply.DryRunVerifier{
			Finder:        cmdutil.NewCRDFinder(cmdutil.CRDFromDynamic(o.DynamicClient)),
			OpenAPIGetter: o.DiscoveryClient,
		},
		Serializer: o.Serializer,
		Visitor:    o.Visitor,
	}

	return nil
}

func (o *ApplyOptions) Run() error {
	err := o.Visitor.Visit(func(config *chart.Config, resources, hooks []runtime.Object, err error) error {
		if err != nil {
			return err
		}

		if o.ShowDiff {
			err = o.DiffOptions.Diff(config, resources, hooks)
			if err != nil {
				return err
			}
		}

		buf, err := o.Serializer.Encode(resources)
		if err != nil {
			return err
		}

		// We need to use a tempfile here instead of a stream as
		// apply.ApplyOption requires that and we do not want to duplicate its
		// huge Run() method to override this.
		f, err := ioutil.TempFile("", config.Name)
		if err != nil {
			return err
		}

		defer f.Close()

		err = ioutil.WriteFile(f.Name(), buf, 0644)
		if err != nil {
			return err
		}

		defer os.Remove(f.Name())

		applier := o.createApplier(config.Name, f.Name())

		return applier.Run()
	})
	if err != nil {
		return err
	}

	return o.Recorder.Objects("pruned").Visit(func(obj runtime.Object, err error) error {
		if err != nil {
			return err
		}

		if !resources.IsOfKind(obj, resources.KindStatefulSet) {
			return nil
		}

		policy, err := resources.GetDeletionPolicy(obj)
		if err != nil || policy != resources.DeletionPolicyDeletePVCs {
			return err
		}

		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return errors.Errorf("illegal object type: %T", obj)
		}

		resourceDeleter := &ResourceDeleter{
			IOStreams:       o.IOStreams,
			DynamicClient:   o.DynamicClient,
			DryRun:          o.DryRun || o.ServerDryRun,
			WaitForDeletion: true,
			Builder: o.BuilderFactory().
				Unstructured().
				ContinueOnError().
				NamespaceParam(u.GetNamespace()).DefaultNamespace().
				ResourceTypeOrNameArgs(true, resources.KindPersistentVolumeClaim).
				LabelSelector(resources.PersistentVolumeClaimSelector(u.GetName())),
		}

		return resourceDeleter.Delete()
	})
}

func (o *ApplyOptions) createApplier(chartName, filename string) *apply.ApplyOptions {
	return &apply.ApplyOptions{
		IOStreams:    o.IOStreams,
		DryRun:       o.DryRun,
		ServerDryRun: o.ServerDryRun,
		Overwrite:    true,
		OpenAPIPatch: true,
		Prune:        true,
		Selector:     chart.LabelSelector(chartName),
		DeleteOptions: &delete.DeleteOptions{
			Cascade:         true,
			GracePeriod:     -1,
			ForceDeletion:   false,
			Timeout:         time.Duration(0),
			WaitForDeletion: false,
			FilenameOptions: resource.FilenameOptions{
				Filenames: []string{filename},
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
		ToPrinter: func(operation string) (kprinters.ResourcePrinter, error) {
			printOperation := operation
			if o.DryRun {
				printOperation = fmt.Sprintf("%s (dry run)", operation)
			}
			if o.ServerDryRun {
				printOperation = fmt.Sprintf("%s (server dry run)", operation)
			}

			p := &kprinters.NamePrinter{Operation: printOperation}

			// Wrap the printer to keep track of the executed operations for
			// each object. We need that later on to perform additonal tasks.
			// Sadly, we have to do this for now to avoid duplicating most of
			// the logic of apply.ApplyOptions.
			return printers.NewRecordingPrinter(o.Recorder, operation, p), nil
		},
	}
}
