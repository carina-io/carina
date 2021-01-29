package mutx

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"sync"
)

// All operations on the local disk need to be performed sequentially
type GlobalLocks struct {
	locks sets.String
	mux   sync.Mutex
}

// NewGlobalLocks returns new GlobalLocks.
func NewGlobalLocks() *GlobalLocks {
	return &GlobalLocks{
		locks: sets.NewString(),
	}
}

// TryAcquire tries to acquire the lock for operating on Id and returns true if successful.
// If another operation is already using Id, returns false.
func (gl *GlobalLocks) TryAcquire(Id string) bool {
	gl.mux.Lock()
	defer gl.mux.Unlock()
	if gl.locks.Has(Id) {
		return false
	}
	gl.locks.Insert(Id)
	return true
}

// Release deletes the lock on Id.
func (gl *GlobalLocks) Release(Id string) {
	gl.mux.Lock()
	defer gl.mux.Unlock()
	gl.locks.Delete(Id)
}
