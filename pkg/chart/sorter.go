package chart

// Adapted from https://github.com/helm/helm/blob/master/pkg/tiller/kind_sorter.go

import (
	"reflect"
	"sort"

	"k8s.io/apimachinery/pkg/api/meta"
)

// Order is a slice of strings that defines the ordering of resources.
type Order []string

// ApplyOrder is the resource order for apply operations.
var ApplyOrder Order = []string{
	"Namespace",
	"ResourceQuota",
	"LimitRange",
	"PodSecurityPolicy",
	"PodDisruptionBudget",
	"Secret",
	"ConfigMap",
	"StorageClass",
	"PersistentVolume",
	"PersistentVolumeClaim",
	"ServiceAccount",
	"CustomResourceDefinition",
	"ClusterRole",
	"ClusterRoleBinding",
	"Role",
	"RoleBinding",
	"Service",
	"DaemonSet",
	"Pod",
	"ReplicationController",
	"ReplicaSet",
	"Deployment",
	"StatefulSet",
	"Job",
	"CronJob",
	"Ingress",
	"APIService",
}

// DeleteOrder is the resource order for delete operations.
var DeleteOrder Order = []string{
	"APIService",
	"Ingress",
	"Service",
	"CronJob",
	"Job",
	"StatefulSet",
	"Deployment",
	"ReplicaSet",
	"ReplicationController",
	"Pod",
	"DaemonSet",
	"RoleBinding",
	"Role",
	"ClusterRoleBinding",
	"ClusterRole",
	"CustomResourceDefinition",
	"ServiceAccount",
	"PersistentVolumeClaim",
	"PersistentVolume",
	"StorageClass",
	"ConfigMap",
	"Secret",
	"PodDisruptionBudget",
	"PodSecurityPolicy",
	"LimitRange",
	"ResourceQuota",
	"Namespace",
}

type resourceSorter struct {
	order            map[string]int
	resources        []*Resource
	metadataAccessor meta.MetadataAccessor
	sortForDeletion  bool
}

func newResourceSorter(resources []*Resource, order Order) *resourceSorter {
	o := make(map[string]int)

	for k, v := range order {
		o[v] = k
	}

	return &resourceSorter{
		resources:        resources,
		metadataAccessor: meta.NewAccessor(),
		sortForDeletion:  reflect.DeepEqual(order, DeleteOrder),
		order:            o,
	}
}

// Len implements Len from sort.Interface.
func (s *resourceSorter) Len() int {
	return len(s.resources)
}

// Swap implements Swap from sort.Interface.
func (s *resourceSorter) Swap(i, j int) {
	s.resources[i], s.resources[j] = s.resources[j], s.resources[i]
}

// Less implements Less from sort.Interface.
func (s *resourceSorter) Less(i, j int) bool {
	a, b := s.resources[i], s.resources[j]

	gvkA := a.GetObjectKind().GroupVersionKind()
	gvkB := b.GetObjectKind().GroupVersionKind()

	posA, aok := s.order[gvkA.Kind]
	posB, bok := s.order[gvkB.Kind]

	nameA, _ := s.metadataAccessor.Name(a)
	nameB, _ := s.metadataAccessor.Name(b)

	if !aok && !bok {
		if gvkA.Kind == gvkB.Kind {
			return nameA < nameB
		}

		return gvkA.Kind < gvkB.Kind
	}

	if !aok {
		return s.sortForDeletion
	}

	if !bok {
		return !s.sortForDeletion
	}

	if posA == posB {
		return nameA < nameB
	}

	return posA < posB
}

// SortResources sorts a slice of runtime.Object in the given order.
func SortResources(resources []*Resource, order Order) []*Resource {
	s := newResourceSorter(resources, order)

	sort.Sort(s)

	return resources
}
