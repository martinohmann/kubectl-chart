package resources

import (
	"fmt"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	KindStatefulSet           = "StatefulSet"
	KindPersistentVolumeClaim = "PersistentVolumeClaim"
	KindPod                   = "Pod"
	KindJob                   = "Job"

	LabelOwnedByStatefulSet = "kubectl-chart/owned-by-statefulset"
)

func LabelStatefulSets(objs []runtime.Object) error {
	for _, obj := range objs {
		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return errors.Errorf("illegal object type: %T", obj)
		}

		if !IsOfKind(obj, KindStatefulSet) {
			continue
		}

		var statefulSet appsv1.StatefulSet
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &statefulSet)
		if err != nil {
			return err
		}

		spec := &statefulSet.Spec

		addStatefulSetLabel(&statefulSet, spec.Selector.MatchLabels)
		addStatefulSetLabel(&statefulSet, spec.Template.ObjectMeta.Labels)

		u.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(&statefulSet)
		if err != nil {
			return err
		}
	}

	return nil
}

func addStatefulSetLabel(statefulSet *appsv1.StatefulSet, labels map[string]string) {
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[LabelOwnedByStatefulSet] = statefulSet.GetName()
}

// PersistentVolumeClaimSelector returns a selector that can be used to query
// for PersistentVolumeClaims owned by a StatefulSet.
func PersistentVolumeClaimSelector(statefulSetName string) string {
	return fmt.Sprintf("%s=%s", LabelOwnedByStatefulSet, statefulSetName)
}
