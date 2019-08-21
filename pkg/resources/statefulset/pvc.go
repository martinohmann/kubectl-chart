package statefulset

import (
	"github.com/martinohmann/kubectl-chart/pkg/deletions"
	"github.com/martinohmann/kubectl-chart/pkg/meta"
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kmeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
)

// PersistentVolumeClaimPruner prunes PersistentVolumeClaims of deleted
// StatefulSets.
type PersistentVolumeClaimPruner struct {
	Deleter       deletions.Deleter
	DynamicClient dynamic.Interface
	Mapper        kmeta.RESTMapper
}

// NewPersistentVolumeClaimPruner creates a new PersistentVolumeClaimPruner value.
func NewPersistentVolumeClaimPruner(client dynamic.Interface, deleter deletions.Deleter, mapper kmeta.RESTMapper) *PersistentVolumeClaimPruner {
	return &PersistentVolumeClaimPruner{
		Deleter:       deleter,
		DynamicClient: client,
		Mapper:        mapper,
	}
}

// PruneClaims searches the slice of runtime objects for StatefulSets that have
// a deletion policy that requests the deletion of all PersistentVolumeClaims
// associated with the StatefulSet once it is deleted and prunes them. It is
// required that the object slice only contains objects of type
// *unstructured.Unstructured.
func (p *PersistentVolumeClaimPruner) PruneClaims(objs []runtime.Object) error {
	if len(objs) == 0 {
		return nil
	}

	mapping, err := p.Mapper.RESTMapping(persistentVolumeClaimGK)
	if err != nil {
		return err
	}

	for _, obj := range objs {
		if !meta.HasGroupKind(obj, statefulSetGK) {
			continue
		}

		err := p.pruneClaims(obj, mapping)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *PersistentVolumeClaimPruner) pruneClaims(obj runtime.Object, mapping *kmeta.RESTMapping) error {
	if !meta.HasAnnotation(obj, meta.AnnotationDeletionPolicy, meta.DeletionPolicyDeletePVCs.String()) {
		return nil
	}

	metadata, _ := kmeta.Accessor(obj)

	objs, err := p.DynamicClient.
		Resource(mapping.Resource).
		Namespace(metav1.NamespaceAll).
		List(metav1.ListOptions{
			LabelSelector: persistentVolumeClaimSelector(metadata.GetName()),
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
