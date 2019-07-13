package resources

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

// ToInfoList converts an unstructured.UnstructuredList to a resource info
// list. The mapper is used to obtain the REST mapping for each object.
func ToInfoList(objs *unstructured.UnstructuredList, mapper meta.RESTMapper) ([]*resource.Info, error) {
	infos := []*resource.Info{}

	for _, obj := range objs.Items {
		gvk := obj.GetObjectKind().GroupVersionKind()

		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return nil, err
		}

		metadata, err := meta.Accessor(&obj)
		if err != nil {
			return nil, err
		}

		info := &resource.Info{
			Mapping:         mapping,
			Namespace:       metadata.GetNamespace(),
			Name:            metadata.GetName(),
			Object:          &obj,
			ResourceVersion: metadata.GetResourceVersion(),
		}

		infos = append(infos, info)
	}

	return infos, nil
}
