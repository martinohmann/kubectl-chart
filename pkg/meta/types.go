package meta

// DeletionPolicy controls behaviour on resource deletion.
type DeletionPolicy string

// String implements fmt.Stringer.
func (p DeletionPolicy) String() string {
	return string(p)
}

const (
	// DeletionPolicyDeletePVCs can be specified in the
	// kubectl-chart/deletion-policy annotation on StatefulSets to make
	// kubectl-chart delete all PersistentVolumeClaims created from the
	// StatefulSet's VolumeClaimTemplates after the StatefulSet is deleted.
	DeletionPolicyDeletePVCs DeletionPolicy = "delete-pvcs"
)
