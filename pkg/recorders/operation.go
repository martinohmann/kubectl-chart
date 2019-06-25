package recorders

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
)

type OperationRecorder interface {
	Record(operation string, obj runtime.Object) error
	GetRecordedObjects() map[string][]runtime.Object
}

type operationRecorder struct {
	sync.Mutex
	operationMap map[string][]runtime.Object
}

func NewOperationRecorder() OperationRecorder {
	return &operationRecorder{
		operationMap: make(map[string][]runtime.Object),
	}
}

func (r *operationRecorder) Record(operation string, obj runtime.Object) error {
	r.Lock()
	defer r.Unlock()

	if r.operationMap[operation] == nil {
		r.operationMap[operation] = make([]runtime.Object, 0, 1)
	}

	r.operationMap[operation] = append(r.operationMap[operation], obj)

	return nil
}

func (r *operationRecorder) GetRecordedObjects() map[string][]runtime.Object {
	r.Lock()
	defer r.Unlock()

	return r.operationMap
}
