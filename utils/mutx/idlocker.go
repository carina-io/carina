/*
   Copyright @ 2021 bocloud <fushaosong@beyondcent.com>.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package mutx

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"sync"
)

// GlobalLocks All operations on the local disk need to be performed sequentially
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
