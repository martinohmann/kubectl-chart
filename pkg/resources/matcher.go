package resources

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

// FindMatching walks haystack and returns the first object matching needle if
// there is one. The second return value indicates whether the object was found
// or not.
func FindMatchingObject(haystack []runtime.Object, needle runtime.Object) (runtime.Object, bool) {
	for _, obj := range haystack {
		if matches(obj, needle) {
			return obj, true
		}
	}

	return nil, false
}

// matches returns true as the first return value of a matches b. Two objects
// match if their kind, namespace and name are the same.
func matches(a, b runtime.Object) bool {
	typeA, _ := meta.TypeAccessor(a)
	typeB, _ := meta.TypeAccessor(b)

	if typeA.GetKind() != typeB.GetKind() {
		return false
	}

	metaA, _ := meta.Accessor(a)
	metaB, _ := meta.Accessor(b)

	if metaA.GetNamespace() != metaB.GetNamespace() {
		return false
	}

	return metaA.GetName() == metaB.GetName()
}
