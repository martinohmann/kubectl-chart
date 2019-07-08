package chart

import (
	"github.com/martinohmann/kubectl-chart/pkg/deletions"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/martinohmann/kubectl-chart/pkg/wait"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
)

type PVCPruner struct {
	BuilderFactory func() *resource.Builder
	Deleter        deletions.Deleter
	Waiter         wait.Waiter
}

func (p *PVCPruner) Prune(v resources.Visitor) error {
	statefulSetVisitor := resources.NewStatefulSetVisitor(v)

	return statefulSetVisitor.Visit(func(obj runtime.Object, err error) error {
		if err != nil {
			return err
		}

		policy, err := GetDeletionPolicy(obj)
		if err != nil || policy != DeletionPolicyDeletePVCs {
			return err
		}

		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return errors.Errorf("illegal object type: %T", obj)
		}

		result := p.BuilderFactory().
			Unstructured().
			ContinueOnError().
			NamespaceParam(u.GetNamespace()).DefaultNamespace().
			ResourceTypeOrNameArgs(true, resources.KindPersistentVolumeClaim).
			LabelSelector(PersistentVolumeClaimSelector(u.GetName())).
			Flatten().
			Do().
			IgnoreErrors(apierrors.IsNotFound)
		if err := result.Err(); err != nil {
			return err
		}

		return p.Deleter.Delete(&deletions.Request{
			Waiter:  p.Waiter,
			Visitor: result,
		})
	})
}
