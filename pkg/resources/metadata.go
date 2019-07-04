package resources

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func GetAnnotation(obj runtime.Object, name string) (string, bool, error) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return "", false, errors.Errorf("illegal object type: %T", obj)
	}

	return unstructured.NestedString(u.Object, "metadata", "annotations", name)
}
