package cmd

import (
	"bytes"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/diff"
	"github.com/martinohmann/kubectl-chart/pkg/yaml"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
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
		Use:   "diff",
		Short: "Diff resources from one or multiple helm charts",
		Long:  "Diffs resources of one or multiple helm charts against the live objects in the cluster.",
		Example: `  # Diff a single chart
  kubectl chart diff --chart-dir ~/charts/mychart

  # Diff multiple charts with custom diff context
  kubectl chart diff --chart-dir ~/charts --recursive --diff-context 20`,
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

	ChartFlags ChartFlags
	DiffFlags  DiffFlags

	OpenAPISchema  openapi.Resources
	DryRunVerifier *apply.DryRunVerifier
	BuilderFactory func() *resource.Builder
	DiffPrinter    diff.Printer
	Serializer     chart.Serializer
	Visitor        chart.Visitor

	Namespace string
}

func NewDiffOptions(streams genericclioptions.IOStreams) *DiffOptions {
	return &DiffOptions{
		IOStreams:  streams,
		DiffFlags:  NewDefaultDiffFlags(),
		Serializer: yaml.NewSerializer(),
	}
}

func (o *DiffOptions) Complete(f genericclioptions.RESTClientGetter) error {
	var err error

	o.DiffPrinter = o.DiffFlags.ToPrinter()

	o.BuilderFactory = func() *resource.Builder {
		return resource.NewBuilder(f)
	}

	discoveryClient, err := f.ToDiscoveryClient()
	if err != nil {
		return err
	}

	config, err := f.ToRESTConfig()
	if err != nil {
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	o.DryRunVerifier = &apply.DryRunVerifier{
		Finder:        cmdutil.NewCRDFinder(cmdutil.CRDFromDynamic(dynamicClient)),
		OpenAPIGetter: discoveryClient,
	}

	o.OpenAPISchema, err = openapi.NewOpenAPIGetter(discoveryClient).Get()
	if err != nil {
		return err
	}

	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.Visitor, err = o.ChartFlags.ToVisitor(o.Namespace)

	return err
}

func (o *DiffOptions) Run() error {
	return o.Visitor.Visit(func(c *chart.Chart, err error) error {
		if err != nil {
			return err
		}

		return o.Diff(c)
	})
}

// Diff performs a diff of a rendered chart and prints it.
func (o *DiffOptions) Diff(c *chart.Chart) error {
	err := o.diffRenderedResources(c)
	if err != nil {
		return err
	}

	return o.diffRemovedResources(c)
}

// Number of times we try to diff before giving-up
const maxRetries = 4

// diffRenderedResources retrieves information about all rendered resources and
// produces a diff of potential changes. The resources are merged with live
// object information to avoid showing diffs for generated fields.
func (o *DiffOptions) diffRenderedResources(c *chart.Chart) error {
	buf, err := o.Serializer.Encode(c.Resources.GetObjects())
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
		Stream(bytes.NewBuffer(buf), c.Config.Name).
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
func (o *DiffOptions) diffRemovedResources(c *chart.Chart) error {
	r := o.BuilderFactory().
		Unstructured().
		AllNamespaces(true).
		LabelSelectorParam(c.LabelSelector()).
		ResourceTypeOrNameArgs(true, "all").
		Flatten().
		Do()
	if err := r.Err(); err != nil {
		return err
	}

	return r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		infoObj := kdiff.InfoObject{Info: info}

		_, found, err := c.Resources.FindMatchingObject(infoObj.Live())
		if err != nil || found {
			return err
		}

		differ := diff.NewRemovalDiffer(infoObj.Name(), infoObj.Live())

		return differ.Print(o.DiffPrinter, o.Out)
	})
}
