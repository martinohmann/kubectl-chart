package chart

import (
	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	AnnotationHookType        = "kubectl-chart/hook-type"
	AnnotationHookWaitTimeout = "kubectl-chart/hook-wait-timeout"
	AnnotationDeletionPolicy  = "kubectl-chart/deletion-policy"

	DeletionPolicyDeletePVCs = "delete-pvcs"
)

func GetDeletionPolicy(obj runtime.Object) (string, error) {
	value, _, err := resources.GetAnnotation(obj, AnnotationDeletionPolicy)
	if err != nil {
		return "", err
	}

	return value, nil
}
