package statefulset

import (
	"fmt"

	"github.com/martinohmann/kubectl-chart/pkg/meta"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	statefulSetGK           = schema.GroupKind{Group: "apps", Kind: "StatefulSet"}
	persistentVolumeClaimGK = schema.GroupKind{Kind: "PersistentVolumeClaim"}
)

// AddOwnerLabels adds the kubectl-chart/owned-by-statefulset label in the
// following locations:
//   - spec.selector.matchLabels
//   - spec.template.metadata.labels
//   - spec.volumeClaimTemplates.metadata.labels
//
// Kubernetes versions before 1.15 use the the matchLabels to set PVCs labels,
// while as of 1.15 labels from the volumeClaimTemplate's metadata are also
// supported (and take precedence). So we just set them all to be sure that we
// have a way to identify PVCs created from the volumeClaimTemplates later.
func AddOwnerLabels(obj runtime.Object) error {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return errors.Errorf("obj is of type %T, expected *unstructured.Unstructured", obj)
	}

	name := u.GetName()

	if !meta.HasGroupKind(obj, statefulSetGK) {
		return errors.Errorf("obj %q is of GroupKind %q, expected %q", name, u.GroupVersionKind().GroupKind(), statefulSetGK)
	}

	err := setNestedStringMapKey(u.Object, meta.LabelOwnedByStatefulSet, name, "spec", "selector", "matchLabels")
	if err != nil {
		return errors.Wrapf(err, "while setting labels for StatefulSet %q", name)
	}

	err = setNestedStringMapKey(u.Object, meta.LabelOwnedByStatefulSet, name, "spec", "template", "metadata", "labels")
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

		err = setNestedStringMapKey(vct, meta.LabelOwnedByStatefulSet, name, "metadata", "labels")
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
	return fmt.Sprintf("%s=%s", meta.LabelOwnedByStatefulSet, statefulSetName)
}
