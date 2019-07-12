package resources

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
)

// ToObjectList converts given info list to a slice of runtime objects.
func ToObjectList(infos []*resource.Info) []runtime.Object {
	objs := make([]runtime.Object, len(infos))

	for i, info := range infos {
		objs[i] = info.Object
	}

	return objs
}
