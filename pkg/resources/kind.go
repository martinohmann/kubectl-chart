package resources

import "k8s.io/apimachinery/pkg/runtime"

const (
	KindStatefulSet           = "StatefulSet"
	KindPersistentVolumeClaim = "PersistentVolumeClaim"
	KindPod                   = "Pod"
	KindJob                   = "Job"
)

// IsOfKind returns true if obj has kind.
func IsOfKind(obj runtime.Object, kind string) bool {
	gvk := obj.GetObjectKind().GroupVersionKind()

	return gvk.Kind == kind
}
