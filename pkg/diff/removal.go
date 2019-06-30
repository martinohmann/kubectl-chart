package diff

import (
	"io"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// RemovalDiffer creates a diff for an object removal.
type RemovalDiffer struct {
	Object runtime.Object
	Name   string
}

// NewRemovalDiffer creates a new *RemovalDiffer for given obj. The name
// parameter is used to display a useful name identifying the object in the
// diff header.
func NewRemovalDiffer(name string, obj runtime.Object) *RemovalDiffer {
	return &RemovalDiffer{
		Object: obj,
		Name:   name,
	}
}

// Print implements Differ. It writes an object removal diff to w using p.
func (d *RemovalDiffer) Print(p Printer, w io.Writer) error {
	buf, err := yaml.Marshal(d.Object)
	if err != nil {
		return err
	}

	s := Subject{
		A:        string(buf),
		FromFile: d.Name,
	}

	return p.Print(s, w)
}
