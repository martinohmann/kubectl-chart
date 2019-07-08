package resources

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
)

// Visitor visits resources and calls fn for every resource that is
// encountered.
type Visitor interface {
	Visit(fn VisitorFunc) error
}

// VisitorFunc is the signature of a function that is called for resource that
// is encountered by the visitor.
type VisitorFunc func(obj runtime.Object, err error) error

// visitor is simple object visitor
type visitor struct {
	Objects []runtime.Object
}

// NewVisitor creates a new Visitor for objs.
func NewVisitor(objs []runtime.Object) Visitor {
	return &visitor{
		Objects: objs,
	}
}

// NewVisitorFromResourceInfoVisitor creates a new Visitor from a resource.Visitor
func NewVisitorFromResourceInfoVisitor(visitor resource.Visitor) (Visitor, error) {
	objs := make([]runtime.Object, 0)

	err := visitor.Visit(func(info *resource.Info, err error) error {
		if err != nil || info.Object == nil {
			return err
		}

		objs = append(objs, info.Object)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return NewVisitor(objs), nil
}

// Visit implements Visitor.
func (v *visitor) Visit(fn VisitorFunc) error {
	for _, obj := range v.Objects {
		if err := fn(obj, nil); err != nil {
			return err
		}
	}

	return nil
}

// StatefulSetVisitor visits all StatefulSets returned by the delegate.
type StatefulSetVisitor struct {
	Delegate Visitor
}

// NewStatefulSetVisitor creates a new visitor the only visits StatefulSets.
func NewStatefulSetVisitor(delegate Visitor) *StatefulSetVisitor {
	if v, ok := delegate.(*StatefulSetVisitor); ok {
		return v
	}

	return &StatefulSetVisitor{
		Delegate: delegate,
	}
}

// Visit implements Visitor.
func (v *StatefulSetVisitor) Visit(fn VisitorFunc) error {
	return v.Delegate.Visit(func(obj runtime.Object, err error) error {
		if err != nil {
			return err
		}

		gvk := obj.GetObjectKind().GroupVersionKind()

		if gvk.Kind != KindStatefulSet {
			return nil
		}

		return fn(obj, nil)
	})
}
