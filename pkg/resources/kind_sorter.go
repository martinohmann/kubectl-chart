package resources

import (
	"reflect"
	"sort"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
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

// SortByKind sorts a slice of runtime.Object in the given order.
func SortByKind(objs []runtime.Object, order Order) []runtime.Object {
	c := newKindComparator(order)

	sort.Slice(objs, func(i, j int) bool {
		return c.less(objs[i], objs[j])
	})

	return objs
}

// SortInfosByKind sorts a slice of *resource.Info in the given order.
func SortInfosByKind(infos []*resource.Info, order Order) []*resource.Info {
	c := newKindComparator(order)

	sort.Slice(infos, func(i, j int) bool {
		return c.less(infos[i].Object, infos[j].Object)
	})

	return infos
}

func newKindComparator(order Order) *kindComparator {
	o := make(map[string]int)

	for k, v := range order {
		o[v] = k
	}

	return &kindComparator{
		metadataAccessor: meta.NewAccessor(),
		sortForDeletion:  reflect.DeepEqual(order, DeleteOrder),
		order:            o,
	}
}

type kindComparator struct {
	order            map[string]int
	metadataAccessor meta.MetadataAccessor
	sortForDeletion  bool
}

func (s *kindComparator) less(a, b runtime.Object) bool {
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
