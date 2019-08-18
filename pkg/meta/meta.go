package meta

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// HasGroupKind returns true if obj has given groupKind.
func HasGroupKind(obj runtime.Object, groupKind schema.GroupKind) bool {
	gvk := obj.GetObjectKind().GroupVersionKind()

	return gvk.GroupKind() == groupKind
}

// DefaultNamespace will set namespace on the resource if it does not have a
// namespace set. Will return an error if namespace is empty or accessing the
// objects metadata fails for some reason.
func DefaultNamespace(obj runtime.Object, namespace string) error {
	if namespace == "" {
		return errors.New("default namespace cannot be empty")
	}

	accessor, _ := meta.Accessor(obj)

	if accessor.GetNamespace() != "" {
		return nil
	}

	accessor.SetNamespace(namespace)

	return nil
}

// GetObjectName returns the name from obj's metadata.
func GetObjectName(obj runtime.Object) string {
	accessor, _ := meta.Accessor(obj)

	return accessor.GetName()
}
