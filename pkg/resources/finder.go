package resources

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
)

// DefaultSupportedVerbs is the list of verbs that is used when a *Finder is
// created via NewFinder.
var DefaultSupportedVerbs = metav1.Verbs{"get", "update", "list", "delete", "watch"}

// Finder is a resource finder.
type Finder struct {
	DiscoveryClient discovery.DiscoveryInterface
	DynamicClient   dynamic.Interface
	Mapper          meta.RESTMapper
	SupportedVerbs  metav1.Verbs
}

// NewFinder creates a new *Finder value.
func NewFinder(client discovery.DiscoveryInterface, dynamicClient dynamic.Interface, mapper meta.RESTMapper) *Finder {
	return &Finder{
		DiscoveryClient: client,
		DynamicClient:   dynamicClient,
		Mapper:          mapper,
		SupportedVerbs:  DefaultSupportedVerbs,
	}
}

// FindByLabelSelector finds all resources that match given label selector and
// returns the resource infos for them. It will only include resources that do
// at least support the verbs specified in f.SupportedVerbs.
func (f *Finder) FindByLabelSelector(selector string) ([]*resource.Info, error) {
	mappings, err := f.getMappings()
	if err != nil {
		return nil, err
	}

	infos := []*resource.Info{}

	for _, m := range mappings {
		objList, err := f.DynamicClient.
			Resource(m.Resource).
			Namespace(metav1.NamespaceAll).
			List(metav1.ListOptions{
				LabelSelector: selector,
			})
		if apierrors.IsNotFound(err) {
			continue
		}

		if apierrors.IsForbidden(err) || apierrors.IsMethodNotSupported(err) {
			// If we are not allowed to access a resource or for some reason it
			// does not support list, we should not fail.
			klog.V(1).Info(err)
			continue
		}

		if err != nil {
			return nil, err
		}

		for _, obj := range objList.Items {
			obj := obj // copy

			infos = append(infos, &resource.Info{
				Mapping:         m,
				Namespace:       obj.GetNamespace(),
				Name:            obj.GetName(),
				Object:          &obj,
				ResourceVersion: obj.GetResourceVersion(),
			})
		}
	}

	return infos, nil
}

func (f *Finder) getMappings() ([]*meta.RESTMapping, error) {
	_, lists, err := f.DiscoveryClient.ServerGroupsAndResources()
	if err != nil {
		return nil, err
	}

	mappings := make([]*meta.RESTMapping, 0)
	seenGKs := make(map[schema.GroupKind]bool)

	for _, list := range lists {
		gv, _ := schema.ParseGroupVersion(list.GroupVersion)

		for _, resource := range list.APIResources {
			if len(resource.Verbs) == 0 {
				continue
			}

			verbs := sets.NewString(resource.Verbs...)

			if len(f.SupportedVerbs) > 0 && !verbs.HasAll(f.SupportedVerbs...) {
				continue
			}

			group := resource.Group
			if group == "" {
				group = gv.Group
			}

			gk := schema.GroupKind{
				Group: group,
				Kind:  resource.Kind,
			}

			if seenGKs[gk] {
				continue
			}

			mapping, err := f.Mapper.RESTMapping(gk)
			if err != nil {
				return nil, err
			}

			mappings = append(mappings, mapping)
			seenGKs[gk] = true
		}
	}

	return mappings, nil
}
