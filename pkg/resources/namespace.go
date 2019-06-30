package resources

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

// EnsureNamespaceSet will set defaultNamespace on all objs that do not have a
// namespace set. Will return an error if defaultNamespace is empty or
// accessing the objects metadata fails for some reason.
func EnsureNamespaceSet(defaultNamespace string, objs ...runtime.Object) error {
	if defaultNamespace == "" {
		return errors.Errorf("default namespace cannot be empty")
	}

	accessor := meta.NewAccessor()

	for _, obj := range objs {
		namespace, err := accessor.Namespace(obj)
		if err != nil {
			return err
		}

		if namespace != "" {
			continue
		}

		err = accessor.SetNamespace(obj, defaultNamespace)
		if err != nil {
			return err
		}
	}

	return nil
}
