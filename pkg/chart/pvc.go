package chart

import (
	"fmt"

	"github.com/martinohmann/kubectl-chart/pkg/deletions"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
)

const (
	// AnnotationDeletionPolicy can be set on resources to specify non-default
	// deletion behaviour. Currently this annotation is ignored on all
	// resources except for StatefulSets.
	AnnotationDeletionPolicy = "kubectl-chart/deletion-policy"

	// DeletionPolicyDeletePVCs can be specified in the
	// kubectl-chart/deletion-policy annotation on StatefulSets to make
	// kubectl-chart delete all PersistentVolumeClaims created from the
	// StatefulSet's VolumeClaimTemplates after the StatefulSet is deleted.
	DeletionPolicyDeletePVCs = "delete-pvcs"
)

var (
	// StatefulSetGK is the GroupKind of StatefulSets
	StatefulSetGK = schema.GroupKind{Group: "apps", Kind: "StatefulSet"}

	// PersistentVolumeClaimGK is the GroupKind of PersistentVolumeClaims
	PersistentVolumeClaimGK = schema.GroupKind{Kind: "PersistentVolumeClaim"}
)

// PersistentVolumeClaimPruner prunes PersistentVolumeClaims of deleted
// StatefulSets.
type PersistentVolumeClaimPruner struct {
	Deleter       deletions.Deleter
	DynamicClient dynamic.Interface
	Mapper        meta.RESTMapper
}

// NewPersistentVolumeClaimPruner creates a new PersistentVolumeClaimPruner value.
func NewPersistentVolumeClaimPruner(client dynamic.Interface, deleter deletions.Deleter, mapper meta.RESTMapper) *PersistentVolumeClaimPruner {
	return &PersistentVolumeClaimPruner{
		Deleter:       deleter,
		DynamicClient: client,
		Mapper:        mapper,
	}
}

// Prune searches the slice of runtime objects for StatefulSets that have a
// deletion policy that requests the deletion of all PersistentVolumeClaims
// associated with the StatefulSet once it is deleted and prunes them. It is
// required that the object slice only contains objects of type
// *unstructured.Unstructured.
func (p *PersistentVolumeClaimPruner) PruneClaims(objs []runtime.Object) error {
	mapping, err := p.Mapper.RESTMapping(PersistentVolumeClaimGK)
	if err != nil {
		return err
	}

	for _, obj := range objs {
		gvk := obj.GetObjectKind().GroupVersionKind()

		if gvk.GroupKind() != StatefulSetGK {
			continue
		}

		err := p.pruneClaims(obj, mapping)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *PersistentVolumeClaimPruner) pruneClaims(obj runtime.Object, mapping *meta.RESTMapping) error {
	metadata, _ := meta.Accessor(obj)

	annotations := metadata.GetAnnotations()
	if annotations[AnnotationDeletionPolicy] != DeletionPolicyDeletePVCs {
		return nil
	}

	objs, err := p.DynamicClient.
		Resource(mapping.Resource).
		Namespace(metav1.NamespaceAll).
		List(metav1.ListOptions{
			LabelSelector: PersistentVolumeClaimSelector(metadata.GetName()),
		})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	infos, err := resources.ToInfoList(objs, p.Mapper)
	if err != nil {
		return err
	}

	return p.Deleter.Delete(resource.InfoListVisitor(infos))
}

// PersistentVolumeClaimSelector returns a selector that can be used to query
// for PersistentVolumeClaims owned by a StatefulSet.
func PersistentVolumeClaimSelector(statefulSetName string) string {
	return fmt.Sprintf("%s=%s", LabelOwnedByStatefulSet, statefulSetName)
}
