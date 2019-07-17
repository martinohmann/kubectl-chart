package chart

import (
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// AnnotationHookType contains the type of the hook. If this annotation is
	// set on a Job it will be treated as a hook and not show up as regular
	// resource anymore.
	AnnotationHookType = "kubectl-chart/hook-type"

	// AnnotationHookAllowFailure controls the behaviour in the event where the
	// hook fails due to timeouts or because the job failed. If set to "true",
	// these errors just will be logged and processing of other hooks and
	// resources continues. Other unhandled errors occuring during hook
	// execution (e.g. API-Server errors) will still bubble up the error
	// handling chain.
	AnnotationHookAllowFailure = "kubectl-chart/hook-allow-failure"

	// AnnotationHookNoWait controls the waiting behaviour. If set to "true",
	// it is not waited for the hook to complete. This cannot be used together
	// with AnnotationHookAllowFailure because the success of a hook is not
	// checked if we do not wait for it to finish.
	AnnotationHookNoWait = "kubectl-chart/hook-no-wait"

	// AnnotationHookWaitTimeout sets a custom wait timeout for a hook. If not
	// set, wait.DefaultWaitTimeout is used.
	AnnotationHookWaitTimeout = "kubectl-chart/hook-wait-timeout"
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

var (
	jobGVR = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}

	jobGK                   = schema.GroupKind{Group: "batch", Kind: "Job"}
	statefulSetGK           = schema.GroupKind{Group: "apps", Kind: "StatefulSet"}
	persistentVolumeClaimGK = schema.GroupKind{Kind: "PersistentVolumeClaim"}
)

// labelStatefulSet adds the kubectl-chart/owned-by-statefulset label in the
// following locations:
//   - spec.selector.matchLabels
//   - spec.template.metadata.labels
//   - spec.volumeClaimTemplates.metadata.labels
//
// Kubernetes versions before 1.15 use the the matchLabels to set PVCs labels,
// while as of 1.15 labels from the volumeClaimTemplate's metadata are also
// supported (and take precedence). So we just set them all to be sure that we
// have a way to identify PVCs created from the volumeClaimTemplates later.
func labelStatefulSet(obj runtime.Object) error {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return errors.Errorf("obj is of type %T, expected *unstructured.Unstructured", obj)
	}

	name := u.GetName()

	err := setNestedStringMapKey(u.Object, LabelOwnedByStatefulSet, name, "spec", "selector", "matchLabels")
	if err != nil {
		return errors.Wrapf(err, "while setting labels for StatefulSet %q", name)
	}

	err = setNestedStringMapKey(u.Object, LabelOwnedByStatefulSet, name, "spec", "template", "metadata", "labels")
	if err != nil {
		return errors.Wrapf(err, "while setting labels for StatefulSet %q", name)
	}

	val, found, err := unstructured.NestedFieldNoCopy(u.Object, "spec", "volumeClaimTemplates")
	if err != nil || !found {
		return err
	}

	vcts, ok := val.([]interface{})
	if !ok {
		return errors.Errorf("while setting labels for StatefulSet %q: .spec.volumeClaimTemplates is of type %T, expected []interface{}", name, val)
	}

	for i, item := range vcts {
		vct, ok := item.(map[string]interface{})
		if !ok {
			return errors.Errorf("while setting labels for StatefulSet %q: .spec.volumeClaimTemplates[%d] is of type %T, expected map[string]interface{}", name, i, item)
		}

		err = setNestedStringMapKey(vct, LabelOwnedByStatefulSet, name, "metadata", "labels")
		if err != nil {
			return errors.Wrapf(err, "while setting labels for StatefulSet %q", name)
		}
	}

	return nil
}

// setNestedStringMapKey sets the key in the nested map identified by fields to
// given string value. If the map does not exist it will be created.
func setNestedStringMapKey(obj map[string]interface{}, key, value string, fields ...string) error {
	m, found, err := unstructured.NestedStringMap(obj, fields...)
	if err != nil {
		return err
	}

	if !found {
		m = make(map[string]string)
	}

	m[key] = value

	return unstructured.SetNestedStringMap(obj, m, fields...)
}

// persistentVolumeClaimSelector returns a selector that can be used to query
// for PersistentVolumeClaims owned by a StatefulSet.
func persistentVolumeClaimSelector(statefulSetName string) string {
	return fmt.Sprintf("%s=%s", LabelOwnedByStatefulSet, statefulSetName)
}
