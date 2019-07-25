package hook

import "k8s.io/apimachinery/pkg/runtime"

// Map is a map of hook type to lists of hooks.
type Map map[string]List

// ToObjectList converts m to a slice of runtime.Object.
func (m Map) ToObjectList() []runtime.Object {
	objs := make([]runtime.Object, 0)
	for _, s := range m {
		objs = append(objs, s.ToObjectList()...)
	}

	return objs
}

// Add adds h to the map and puts it into the bucket determined by its type.
func (m Map) Add(h *Hook) Map {
	hookType := h.Type()

	if m[hookType] == nil {
		m[hookType] = List{h}
	} else {
		m[hookType] = append(m[hookType], h)
	}

	return m
}

// List is a slice of *Hook.
type List []*Hook

// ToObjectList converts l to a slice of runtime.Object.
func (l List) ToObjectList() []runtime.Object {
	objs := make([]runtime.Object, len(l))
	for i := range l {
		objs[i] = l[i].Unstructured
	}

	return objs
}

// EachItem walks the list and calls fn for each item. If fn returns an error
// for any hook, walking the list is stopped and the error is returned.
func (l List) EachItem(fn func(*Hook) error) error {
	for _, h := range l {
		if err := fn(h); err != nil {
			return err
		}
	}

	return nil
}
