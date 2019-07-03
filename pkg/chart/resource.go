package chart

import (
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Resource struct {
	*unstructured.Unstructured
}

func NewResource(obj runtime.Object) *Resource {
	return &Resource{
		Unstructured: obj.(*unstructured.Unstructured),
	}
}

func (r *Resource) GetObject() runtime.Object {
	return r.Unstructured
}

// DefaultNamespace will set namespace on the resource if it does not have a
// namespace set. Will return an error if namespace is empty or accessing the
// objects metadata fails for some reason.
func (r *Resource) DefaultNamespace(namespace string) error {
	if namespace == "" {
		return errors.Errorf("default namespace cannot be empty")
	}

	accessor, err := meta.Accessor(r.Unstructured)
	if err != nil {
		return err
	}

	if accessor.GetNamespace() != "" {
		return nil
	}

	accessor.SetNamespace(namespace)

	return nil
}

func (r *Resource) DeletionPolicy() string {
	return r.nestedString("metadata", "annotations", AnnotationDeletionPolicy)
}

func (r *Resource) SetLabel(key, value string) error {
	return r.setNestedField(value, "metadata", "labels", key)
}

func (r *Resource) setNestedField(value interface{}, fields ...string) error {
	return unstructured.SetNestedField(r.Object, value, fields...)
}

func (r *Resource) nestedString(fields ...string) string {
	value, _, _ := unstructured.NestedString(r.Object, fields...)
	return value
}

type ResourceList []*Resource

func (l ResourceList) GetObjects() []runtime.Object {
	objs := make([]runtime.Object, len(l))
	for i := range l {
		objs[i] = l[i].GetObject()
	}

	return objs
}

// FindMatchingObject walks the resource list and returns the first object in
// the list matching obj if there is one. The second return value indicates
// whether the object was found or not.
func (l ResourceList) FindMatchingObject(obj runtime.Object) (runtime.Object, bool, error) {
	return resources.FindMatching(l.GetObjects(), obj)
}
