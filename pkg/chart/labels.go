package chart

import (
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// LabelChartName is used to attach a label to each resource in a rendered chart
	// to be able to keep track of them once they are deployed into a cluster.
	LabelChartName = "kubectl-chart/chart-name"

	// LabelHookChartName is used to attach a label to each hook to be able to
	// keep track of them once they are deployed into a cluster. The label is
	// different from LabelChartName because hooks have a different lifecycle
	// than normal resources.
	LabelHookChartName = "kubectl-chart/hook-chart-name"

	// LabelHookType is set on chart hooks to be able to clean them up by type.
	LabelHookType = "kubectl-chart/hook-type"

	// LabelOwnedByStatefulSet is set on PersistentVolumeClaims to identify
	// them when a StatefulSet is deleted.
	LabelOwnedByStatefulSet = "kubectl-chart/owned-by-statefulset"
)

func LabelStatefulSets(objs []runtime.Object) error {
	for _, obj := range objs {
		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return errors.Errorf("illegal object type: %T", obj)
		}

		gvk := obj.GetObjectKind().GroupVersionKind()

		if gvk.GroupKind() != StatefulSetGK {
			continue
		}

		var statefulSet appsv1.StatefulSet
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &statefulSet)
		if err != nil {
			return err
		}

		spec := &statefulSet.Spec
		spec.Selector.MatchLabels[LabelOwnedByStatefulSet] = statefulSet.GetName()
		spec.Template.ObjectMeta.Labels[LabelOwnedByStatefulSet] = statefulSet.GetName()

		for i := range spec.VolumeClaimTemplates {
			spec.VolumeClaimTemplates[i].ObjectMeta.Labels[LabelOwnedByStatefulSet] = statefulSet.GetName()
		}

		u.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(&statefulSet)
		if err != nil {
			return err
		}
	}

	return nil
}
