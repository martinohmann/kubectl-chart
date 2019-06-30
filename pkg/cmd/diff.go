package cmd

import (
	"bytes"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/diff"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/kubectl/cmd/apply"
	kdiff "k8s.io/kubernetes/pkg/kubectl/cmd/diff"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
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
	o.DiffFlags.AddFlags(cmd)

	return cmd
}

type DiffOptions struct {
	genericclioptions.IOStreams
	*RenderOptions

	DiffFlags *DiffFlags

	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.CachedDiscoveryInterface
	OpenAPISchema   openapi.Resources
	DryRunVerifier  *apply.DryRunVerifier
	BuilderFactory  func() *resource.Builder
	DiffPrinter     diff.Printer

	EnforceNamespace bool
}

func NewDiffOptions(streams genericclioptions.IOStreams) *DiffOptions {
	return &DiffOptions{
		IOStreams:     streams,
		RenderOptions: NewRenderOptions(streams),
		DiffFlags:     NewDefaultDiffFlags(),
	}
}

func (o *DiffOptions) Complete(f genericclioptions.RESTClientGetter) error {
	var err error

	err = o.RenderOptions.Complete(f)
	if err != nil {
		return err
	}

	o.DiffPrinter = o.DiffFlags.ToPrinter()

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

func (o *DiffOptions) Run() error {
	return o.Visit(func(config *chart.Config, resources, hooks []runtime.Object, err error) error {
		if err != nil {
			return err
		}

		return o.Diff(config, resources, hooks)
	})
}

func (o *DiffOptions) Diff(config *chart.Config, resources, hooks []runtime.Object) error {
	err := o.diffRenderedResources(config, resources)
	if err != nil {
		return err
	}

	return o.diffRemovedResources(config, resources)
}

// Number of times we try to diff before giving-up
const maxRetries = 4

// diffRenderedResources retrieves information about all rendered resources and
// produces a diff of potential changes. The resources are merged with live
// object information to avoid showing diffs for generated fields.
func (o *DiffOptions) diffRenderedResources(config *chart.Config, objs []runtime.Object) error {
	buf, err := o.Serializer.Encode(objs)
	if err != nil {
		return err
	}

	kdiffer, err := kdiff.NewDiffer("LIVE", "MERGED")
	if err != nil {
		return err
	}

	defer kdiffer.TearDown()

	kprinter := kdiff.Printer{}

	r := o.BuilderFactory().
		Unstructured().
		NamespaceParam(o.Namespace).DefaultNamespace().
		Stream(bytes.NewBuffer(buf), "stream").
		Flatten().
		Do()
	if err := r.Err(); err != nil {
		return err
	}

	err = r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		if err := o.DryRunVerifier.HasSupport(info.Mapping.GroupVersionKind); err != nil {
			return err
		}

		local := info.Object.DeepCopyObject()

		for i := 1; i <= maxRetries; i++ {
			if err = info.Get(); err != nil {
				if !errors.IsNotFound(err) {
					return err
				}
				info.Object = nil
			}

			force := i == maxRetries
			if force {
				klog.Warningf(
					"Object (%v: %v) keeps changing, diffing without lock",
					info.Object.GetObjectKind().GroupVersionKind(),
					info.Name,
				)
			}

			obj := kdiff.InfoObject{
				LocalObj: local,
				Info:     info,
				Encoder:  scheme.DefaultJSONEncoder(),
				OpenAPI:  o.OpenAPISchema,
				Force:    force,
			}

			err = kdiffer.Diff(obj, kprinter)
			if !errors.IsConflict(err) {
				break
			}
		}

		return err
	})
	if err != nil {
		return err
	}

	differ := diff.NewPathDiffer(kdiffer.From.Dir.Name, kdiffer.To.Dir.Name)

	return differ.Print(o.DiffPrinter, o.Out)
}

// diffRemovedResources retrieves all resources matching the chart label from
// the cluster and compares them to the rendered resources from the helm chart.
// It will produce a deletion diff for resources that have been removed from
// the helm chart but which are still present in the cluster.
func (o *DiffOptions) diffRemovedResources(config *chart.Config, objs []runtime.Object) error {
	r := o.BuilderFactory().
		Unstructured().
		AllNamespaces(true).
		LabelSelectorParam(chart.LabelSelector(config.Name)).
		ResourceTypeOrNameArgs(true, "all").
		Flatten().
		Do()
	if err := r.Err(); err != nil {
		return err
	}

	infos := make([]kdiff.Object, 0)

	err := r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		infos = append(infos, kdiff.InfoObject{Info: info})

		return nil
	})
	if err != nil {
		return err
	}

	for _, info := range infos {
		_, found, err := resources.FindMatching(objs, info.Live())
		if err != nil {
			return err
		}

		if found {
			continue
		}

		differ := diff.NewRemovalDiffer(info.Name(), info.Live())

		err = differ.Print(o.DiffPrinter, o.Out)
		if err != nil {
			return err
		}
	}

	return err
}
