package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/martinohmann/kubectl-chart/pkg/chart"
	"github.com/martinohmann/kubectl-chart/pkg/wait"
	"github.com/martinohmann/kubectl-chart/pkg/yaml"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
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
	cmd.Flags().BoolVar(&o.Prune, "prune", o.Prune, "If true, all resources matching the chart selector will be pruned, even those previously removed from the chart.")

	return cmd
}

type DeleteOptions struct {
	genericclioptions.IOStreams

	DryRun     bool
	Prune      bool
	ChartFlags *ChartFlags

	DynamicClient  dynamic.Interface
	BuilderFactory func() *resource.Builder
	Serializer     chart.Serializer
	Visitor        *chart.Visitor
	HookExecutor   *HookExecutor

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

	o.HookExecutor = &HookExecutor{
		IOStreams: o.IOStreams,
		DryRun:    o.DryRun,
	}

	o.Visitor, err = o.ChartFlags.ToVisitor(o.Namespace)

	return err
}

func (o *DeleteOptions) Run() error {
	return o.Visitor.Visit(func(c *chart.Chart, err error) error {
		if err != nil {
			return err
		}

		builder := o.BuilderFactory().
			Unstructured().
			ContinueOnError().
			RequireObject(false)

		if o.Prune {
			builder = builder.AllNamespaces(true).
				ResourceTypeOrNameArgs(true, "all").
				LabelSelector(c.LabelSelector())
		} else {
			buf, err := o.Serializer.Encode(c.Resources.GetObjects())
			if err != nil {
				return err
			}

			builder = builder.
				NamespaceParam(o.Namespace).DefaultNamespace().
				Stream(bytes.NewBuffer(buf), c.Config.Name)
		}

		resourceDeleter := &ResourceDeleter{
			IOStreams:       o.IOStreams,
			DynamicClient:   o.DynamicClient,
			DryRun:          o.DryRun,
			WaitForDeletion: true,
			Builder:         builder,
			Waiter:          wait.NewDefaultWaiter(o.IOStreams, o.DynamicClient),
		}

		err = o.HookExecutor.ExecHooks(c, chart.PreDeleteHook)
		if err != nil {
			return err
		}

		err = resourceDeleter.Delete()
		if err != nil {
			return err
		}

		return o.HookExecutor.ExecHooks(c, chart.PostDeleteHook)
	})
}

// ResourceDeleter carries out resource deletions based on the result returned
// by the builder.
type ResourceDeleter struct {
	genericclioptions.IOStreams
	Builder         *resource.Builder
	DynamicClient   dynamic.Interface
	DryRun          bool
	WaitForDeletion bool
	Waiter          *wait.Waiter
}

// Delete deletes all resources matching the infos in the result returned by
// the builder. If the DryRun is set to true, deletion operations will be only
// printed without actually performing them. If the WaitForDeletion field is
// set to true, the deleter will wait until the resources are deleted from the
// cluster.
func (d *ResourceDeleter) Delete() error {
	result := d.Builder.Flatten().Do()
	if err := result.Err(); err != nil {
		return err
	}

	result = result.IgnoreErrors(errors.IsNotFound)

	found := 0

	deletedInfos := []*resource.Info{}
	uidMap := wait.UIDMap{}

	err := result.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		deletedInfos = append(deletedInfos, info)
		found++

		if d.DryRun {
			if err = info.Get(); err != nil {
				return err
			}

			d.PrintObj(info)

			return nil
		}

		policy := metav1.DeletePropagationBackground

		response, err := d.deleteResource(info, &metav1.DeleteOptions{
			PropagationPolicy: &policy,
		})
		if err != nil {
			return err
		}

		resourceLocation := wait.ResourceLocation{
			GroupResource: info.Mapping.Resource.GroupResource(),
			Namespace:     info.Namespace,
			Name:          info.Name,
		}

		if status, ok := response.(*metav1.Status); ok && status.Details != nil {
			uidMap[resourceLocation] = status.Details.UID
			return nil
		}

		responseMetadata, err := meta.Accessor(response)
		if err != nil {
			// we don't have UID, but we didn't fail the delete, next best
			// thing is just skipping the UID
			klog.V(1).Info(err)
			return nil
		}

		uidMap[resourceLocation] = responseMetadata.GetUID()

		return nil
	})
	if err != nil || found == 0 {
		return err
	}

	if !d.WaitForDeletion || d.DryRun {
		return nil
	}

	req := &wait.Request{
		ConditionFn: wait.IsDeleted,
		Options: wait.Options{
			Timeout: 24 * time.Hour,
		},
		Visitor: resource.InfoListVisitor(deletedInfos),
		UIDMap:  uidMap,
	}

	err = d.Waiter.Wait(req)
	if errors.IsForbidden(err) || errors.IsMethodNotSupported(err) {
		// if we're forbidden from waiting, we shouldn't fail.
		// if the resource doesn't support a verb we need, we shouldn't fail.
		klog.V(1).Info(err)
		return nil
	}

	return err
}

func (d *ResourceDeleter) deleteResource(info *resource.Info, deleteOptions *metav1.DeleteOptions) (runtime.Object, error) {
	response, err := resource.NewHelper(info.Client, info.Mapping).
		DeleteWithOptions(info.Namespace, info.Name, deleteOptions)

	if err != nil {
		return nil, cmdutil.AddSourceToErr("deleting", info.Source, err)
	}

	d.PrintObj(info)

	return response, nil
}

// PrintObj prints out the object that was deleted (or would be deleted if dry
// run is enabled).
func (d *ResourceDeleter) PrintObj(info *resource.Info) {
	operation := "deleted"
	groupKind := info.Mapping.GroupVersionKind
	kindString := fmt.Sprintf("%s.%s", strings.ToLower(groupKind.Kind), groupKind.Group)
	if len(groupKind.Group) == 0 {
		kindString = strings.ToLower(groupKind.Kind)
	}

	if d.DryRun {
		operation = fmt.Sprintf("%s (dry run)", operation)
	}

	fmt.Fprintf(d.Out, "%s/%s %s\n", kindString, info.Name, operation)
}
