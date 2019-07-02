package resources

import "k8s.io/apimachinery/pkg/runtime"

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

// Visit implements Visitor.
func (v *visitor) Visit(fn VisitorFunc) error {
	for _, obj := range v.Objects {
		if err := fn(obj, nil); err != nil {
			return err
		}
	}

	return nil
}
