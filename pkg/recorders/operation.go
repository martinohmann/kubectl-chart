package recorders

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
)

// Operation recorder records operations on objects.
type OperationRecorder interface {
	// Record records an object for given operation.
	Record(operation string, obj runtime.Object) error

	// RecordedObjects returns a slice of objects recorded for given operation.
	RecordedObjects(operation string) []runtime.Object
}

type operationRecorder struct {
	sync.Mutex
	operationMap map[string][]runtime.Object
}

// NewOperationRecorder creates a generic OperationRecorder.
func NewOperationRecorder() OperationRecorder {
	return &operationRecorder{
		operationMap: make(map[string][]runtime.Object),
	}
}

// Record implements OperationRecorder.
func (r *operationRecorder) Record(operation string, obj runtime.Object) error {
	r.Lock()
	defer r.Unlock()

	if r.operationMap[operation] == nil {
		r.operationMap[operation] = make([]runtime.Object, 0, 1)
	}

	r.operationMap[operation] = append(r.operationMap[operation], obj)

	return nil
}

// RecordedObjects implements OperationRecorder.
func (r *operationRecorder) RecordedObjects(operation string) []runtime.Object {
	r.Lock()
	defer r.Unlock()

	return r.operationMap[operation]
}
