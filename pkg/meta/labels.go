package meta

import (
	"github.com/pkg/errors"
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

// AddLabel adds a label on an object. If the label already exists it will be
// overwritten.
func AddLabel(obj runtime.Object, key, value string) error {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return errors.Errorf("obj is of type %T, expected *unstructured.Unstructured", obj)
	}

	return unstructured.SetNestedField(u.Object, value, "metadata", "labels", key)
}
