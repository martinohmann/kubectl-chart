package chart

import (
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	// LabelOwnedByStatefulSet is set on PersistentVolumeClaims to identify
	// them when a StatefulSet is deleted.
	LabelOwnedByStatefulSet = "kubectl-chart/owned-by-statefulset"
)

var (
	jobGVR = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}

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

// defaultNamespace will set namespace on the resource if it does not have a
// namespace set. Will return an error if namespace is empty or accessing the
// objects metadata fails for some reason.
func defaultNamespace(obj runtime.Object, namespace string) error {
	if namespace == "" {
		return errors.Errorf("default namespace cannot be empty")
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	if accessor.GetNamespace() != "" {
		return nil
	}

	accessor.SetNamespace(namespace)

	return nil
}

func setLabel(obj runtime.Object, key, value string) error {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return errors.Errorf("obj is of type %T, expected *unstructured.Unstructured", obj)
	}

	return unstructured.SetNestedField(u.Object, value, "metadata", "labels", key)
}

func hasAnnotation(obj runtime.Object, key string) bool {
	metadata, _ := meta.Accessor(obj)

	_, found := metadata.GetAnnotations()[key]

	return found
}
