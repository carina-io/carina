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

package runners

import (
	"context"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type readinessCheck struct {
	check    func() error
	interval time.Duration

	mu    sync.RWMutex
	ready bool
	err   error
}

// Checker is the interface to check plugin readiness.
type Checker interface {
	manager.Runnable
	Ready() (bool, error)
}

var _ manager.LeaderElectionRunnable = &readinessCheck{}

// NewChecker creates controller-runtime's manager.Runnable to run
// health check function periodically at given interval.
func NewChecker(check func() error, interval time.Duration) Checker {
	return &readinessCheck{check: check, interval: interval}
}

func (c *readinessCheck) setError(e error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// plugin becomes ready at the first non-nil check.
	if e == nil {
		c.ready = true
	}
	c.err = e
}

// Start implements controller-runtime's manager.Runnable.
func (c *readinessCheck) Start(ctx context.Context) error {
	c.setError(c.check())

	tick := time.NewTicker(c.interval)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			c.setError(c.check())
		case <-ctx.Done():
			return nil
		}
	}
}

// NeedLeaderElection implements controller-runtime's manager.LeaderElectionRunnable.
func (c *readinessCheck) NeedLeaderElection() bool {
	return false
}

// Ready can be passed to driver.NewIdentityService.
func (c *readinessCheck) Ready() (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ready, c.err
}
