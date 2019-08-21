package resources

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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

// ToInfoList converts obj to a resource info list. The obj must be a list type
// runtime.Object or a slice of runtime.Object. The mapper is used to obtain
// the REST mapping for each object.
func ToInfoList(obj interface{}, mapper meta.RESTMapper) ([]*resource.Info, error) {
	objs, err := interfaceToObjectList(obj)
	if err != nil {
		return nil, err
	}

	infos := []*resource.Info{}

	for _, obj := range objs {
		gvk := obj.GetObjectKind().GroupVersionKind()

		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return nil, err
		}

		metadata, err := meta.Accessor(obj)
		if err != nil {
			return nil, err
		}

		info := &resource.Info{
			Mapping:         mapping,
			Namespace:       metadata.GetNamespace(),
			Name:            metadata.GetName(),
			Object:          obj,
			ResourceVersion: metadata.GetResourceVersion(),
		}

		infos = append(infos, info)
	}

	return infos, nil
}

func interfaceToObjectList(obj interface{}) ([]runtime.Object, error) {
	switch obj := obj.(type) {
	case []runtime.Object:
		return obj, nil
	case runtime.Object:
		return meta.ExtractList(obj)
	default:
		return nil, errors.Errorf("got %T, expected list type runtime.Object or []runtime.Object", obj)
	}
}
