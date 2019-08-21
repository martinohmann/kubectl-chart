package cmd

import (
	"bytes"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/diff"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/martinohmann/kubectl-chart/pkg/yaml"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
	"k8s.io/kubectl/pkg/cmd/apply"
	kdiff "k8s.io/kubectl/pkg/cmd/diff"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/openapi"
	"k8s.io/kubectl/pkg/util/templates"
)

// DryRunVerifier verifies if a given group-version-kind supports DryRun
// against the current server. Sending dryRun requests to apiserver that
// don't support it will result in objects being unwillingly persisted.
type DryRunVerifier interface {
	// HasSupport verifies if the given gvk supports DryRun. An error is
	// returned if it doesn't.
	HasSupport(gvk schema.GroupVersionKind) error
}

func NewDiffCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDiffOptions(streams)

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Diff resources from one or multiple helm charts",
		Long: templates.LongDesc(
			`Diffs resources of one or multiple helm charts against the live objects in the cluster.`),
		Example: templates.Examples(`
			# Diff a single chart
			kubectl chart diff -f ~/charts/mychart

			# Diff multiple charts with custom diff context and no coloring
			kubectl chart diff -f ~/charts --recursive --diff-context 20 --no-color`),
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Run())
		},
	}

	o.ChartFlags.AddFlags(cmd)
	o.DiffFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.Prune, "prune", o.Prune, "If true, chart resources not present anymore in the rendered chart manifest will be printed as deletions in the diff.")

	return cmd
}

type DiffOptions struct {
	genericclioptions.IOStreams
	cmdutil.Factory

	ChartFlags ChartFlags
	DiffFlags  DiffFlags
	Prune      bool

	OpenAPISchema  openapi.Resources
	DryRunVerifier DryRunVerifier
	DiffPrinter    diff.Printer
	Encoder        resources.Encoder
	Visitor        chart.Visitor

	Namespace string
}

func NewDiffOptions(streams genericclioptions.IOStreams) *DiffOptions {
	return &DiffOptions{
		IOStreams: streams,
		DiffFlags: NewDefaultDiffFlags(),
		Encoder:   yaml.NewEncoder(),
		Prune:     true,
	}
}

func (o *DiffOptions) Complete(f cmdutil.Factory) error {
	o.Factory = f
	o.DiffPrinter = o.DiffFlags.ToPrinter()

	discoveryClient, err := f.ToDiscoveryClient()
	if err != nil {
		return err
	}

	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return err
	}

	o.DryRunVerifier = newDryRunVerifier(dynamicClient, discoveryClient)

	o.OpenAPISchema, err = f.OpenAPISchema()
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
	buf, err := o.Encoder.Encode(c.Resources)
	if err != nil {
		return err
	}

	kdiffer, err := kdiff.NewDiffer("LIVE", "MERGED")
	if err != nil {
		return err
	}

	defer kdiffer.TearDown()

	kprinter := kdiff.Printer{}

	r := o.NewBuilder().
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
	if !o.Prune {
		return nil
	}

	r := o.NewBuilder().
		Unstructured().
		AllNamespaces(true).
		LabelSelectorParam(chart.LabelSelector(c)).
		ResourceTypeOrNameArgs(true, "all").
		Flatten().
		Do().
		IgnoreErrors(errors.IsNotFound)
	if err := r.Err(); err != nil {
		return err
	}

	return r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		obj := kdiff.InfoObject{Info: info}

		_, found := resources.FindMatchingObject(c.Resources, obj.Live())
		if found {
			// Objects still present in the chart do not need to be diffed
			// again as this already happened in diffRenderedResources.
			return nil
		}

		differ := diff.NewRemovalDiffer(obj.Name(), obj.Live())

		return differ.Print(o.DiffPrinter, o.Out)
	})
}

func newDryRunVerifier(dynamicClient dynamic.Interface, discoveryClient discovery.DiscoveryInterface) *apply.DryRunVerifier {
	return &apply.DryRunVerifier{
		Finder:        cmdutil.NewCRDFinder(cmdutil.CRDFromDynamic(dynamicClient)),
		OpenAPIGetter: discoveryClient,
	}
}
