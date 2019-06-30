package resources

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

// FindMatching walks haystack and returns the first object matching needle if
// there is one. The second return value indicates whether the object was found
// or not.
func FindMatching(haystack []runtime.Object, needle runtime.Object) (runtime.Object, bool, error) {
	for _, obj := range haystack {
		ok, err := Matches(obj, needle)
		if err != nil {
			return nil, false, err
		}

		if ok {
			return obj, true, nil
		}
	}

	return nil, false, nil
}

// Matches returns true as the first return value of a matches b. Two objects
// match if their kind, namespace and name are the same.
func Matches(a, b runtime.Object) (bool, error) {
	typeA, err := meta.TypeAccessor(a)
	if err != nil {
		return false, err
	}

	typeB, err := meta.TypeAccessor(b)
	if err != nil {
		return false, err
	}

	if typeA.GetKind() != typeB.GetKind() {
		return false, nil
	}

	metaA, err := meta.Accessor(a)
	if err != nil {
		return false, err
	}

	metaB, err := meta.Accessor(b)
	if err != nil {
		return false, err
	}

	if metaA.GetNamespace() != metaB.GetNamespace() {
		return false, nil
	}

	return metaA.GetName() == metaB.GetName(), nil
}
