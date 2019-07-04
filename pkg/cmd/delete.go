package cmd

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/yaml"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/pkg/kubectl/cmd/delete"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

func NewDeleteCmd(f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDeleteOptions(streams)

	cmd := &cobra.Command{
		Use:  "delete",
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Run())
		},
	}

	o.ChartFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.DryRun, "dry-run", o.DryRun, "If true, only print the object that would be sent, without sending it. Warning: --dry-run cannot accurately output the result of merging the local manifest and the server-side data. Use --server-dry-run to get the merged result instead.")

	return cmd
}

type DeleteOptions struct {
	genericclioptions.IOStreams

	DryRun     bool
	ChartFlags *ChartFlags

	DynamicClient  dynamic.Interface
	Mapper         meta.RESTMapper
	BuilderFactory func() *resource.Builder
	Serializer     chart.Serializer
	Visitor        *chart.Visitor

	Namespace        string
	EnforceNamespace bool
}

func NewDeleteOptions(streams genericclioptions.IOStreams) *DeleteOptions {
	return &DeleteOptions{
		IOStreams:  streams,
		ChartFlags: NewDefaultChartFlags(),
		Serializer: yaml.NewSerializer(),
	}
}

func (o *DeleteOptions) Complete(f genericclioptions.RESTClientGetter) error {
	var err error

	o.BuilderFactory = func() *resource.Builder {
		return resource.NewBuilder(f)
	}

	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
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

	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}

	o.Visitor, err = o.ChartFlags.ToVisitor(o.Namespace)

	return err
}

func (o *DeleteOptions) createDeleter(chartName string, stream io.Reader) (*delete.DeleteOptions, error) {
	d := &delete.DeleteOptions{
		IOStreams:       o.IOStreams,
		IgnoreNotFound:  true,
		Cascade:         true,
		WaitForDeletion: true,
		GracePeriod:     -1,
		Mapper:          o.Mapper,
		DynamicClient:   o.DynamicClient,
	}

	r := o.BuilderFactory().
		Unstructured().
		ContinueOnError().
		NamespaceParam(o.Namespace).DefaultNamespace().
		Stream(stream, chartName).
		RequireObject(false).
		Do()

	if err := r.Err(); err != nil {
		return nil, err
	}

	d.Result = r

	return d, nil
}

func (o *DeleteOptions) Run() error {
	return o.Visitor.Visit(func(config *chart.Config, resources, hooks []runtime.Object, err error) error {
		if err != nil {
			return err
		}

		buf, err := o.Serializer.Encode(resources)
		if err != nil {
			return err
		}

		deleter, err := o.createDeleter(config.Name, bytes.NewBuffer(buf))
		if err != nil {
			return err
		}

		if !o.DryRun {
			return deleter.RunDelete()
		}

		return deleter.Result.Visit(func(info *resource.Info, err error) error {
			if err != nil {
				return err
			}

			err = info.Get()
			if errors.IsNotFound(err) {
				return nil
			} else if err != nil {
				return err
			}

			o.PrintObj(info)

			return nil
		})
	})
}

func (o *DeleteOptions) PrintObj(info *resource.Info) {
	operation := "deleted"
	groupKind := info.Mapping.GroupVersionKind
	kindString := fmt.Sprintf("%s.%s", strings.ToLower(groupKind.Kind), groupKind.Group)
	if len(groupKind.Group) == 0 {
		kindString = strings.ToLower(groupKind.Kind)
	}

	if o.DryRun {
		operation = fmt.Sprintf("%s (dry run)", operation)
	}

	fmt.Fprintf(o.Out, "%s \"%s\" %s\n", kindString, info.Name, operation)
}
