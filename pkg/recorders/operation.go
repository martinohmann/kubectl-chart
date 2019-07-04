package recorders

import (
	"sync"

	"github.com/martinohmann/kubectl-chart/pkg/resources"
	"k8s.io/apimachinery/pkg/runtime"
)

// Operation recorder records operations on objects.
type OperationRecorder interface {
	// Record records an object for given operation.
	Record(operation string, obj runtime.Object) error

	// Objects returns a resources.Visitor that visits all objects recorded for
	// given operation.
	Objects(operation string) resources.Visitor
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

// Objects implements OperationRecorder.
func (r *operationRecorder) Objects(operation string) resources.Visitor {
	r.Lock()
	defer r.Unlock()

	return resources.NewVisitor(r.operationMap[operation])
}
