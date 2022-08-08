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

package iolimit

import (
	libcontainercgroups "github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"testing"
)

func TestGetPodBlkIOCgroupPath(t *testing.T) {
	podBlkIO := &PodBlkIO{
		PodUid:      "88888888-8888",
		PodQos:      v1.PodQOSBestEffort,
		DeviceIOSet: map[string]*IOLimit{},
	}
	a := assert.New(t)
	blkIOPath := getPodBlkIOCgroupPath(podBlkIO)
	if libcontainercgroups.IsCgroup2UnifiedMode() {
		if getCgroupDriverType() == SystemdDriver {
			a.Equal("/sys/fs/cgroup/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod88888888_8888.slice", blkIOPath)
		} else {
			a.Equal("/sys/fs/cgroup/kubepods/besteffort/pod88888888-8888", blkIOPath)
		}
	} else {
		if getCgroupDriverType() == SystemdDriver {
			a.Equal("/sys/fs/cgroup/blkio/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod88888888_8888.slice", blkIOPath)
		} else {
			a.Equal("/sys/fs/cgroup/blkio/kubepods/besteffort/pod88888888-8888", blkIOPath)
		}
	}

	podBlkIO.PodQos = v1.PodQOSGuaranteed
	blkIOPath = getPodBlkIOCgroupPath(podBlkIO)
	if libcontainercgroups.IsCgroup2UnifiedMode() {
		if getCgroupDriverType() == SystemdDriver {
			a.Equal("/sys/fs/cgroup/kubepods.slice/kubepods-pod88888888_8888.slice", blkIOPath)
		} else {
			a.Equal("/sys/fs/cgroup/kubepods/pod88888888-8888", blkIOPath)
		}
	} else {
		if getCgroupDriverType() == SystemdDriver {
			a.Equal("/sys/fs/cgroup/blkio/kubepods.slice/kubepods-pod88888888_8888.slice", blkIOPath)
		} else {
			a.Equal("/sys/fs/cgroup/blkio/kubepods/pod88888888-8888", blkIOPath)
		}
	}

	podBlkIO.PodQos = v1.PodQOSBurstable
	blkIOPath = getPodBlkIOCgroupPath(podBlkIO)
	if libcontainercgroups.IsCgroup2UnifiedMode() {
		if getCgroupDriverType() == SystemdDriver {
			a.Equal("/sys/fs/cgroup/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod88888888_8888.slice", blkIOPath)
		} else {
			a.Equal("/sys/fs/cgroup/kubepods/burstable/pod88888888-8888", blkIOPath)
		}
	} else {
		if getCgroupDriverType() == SystemdDriver {
			a.Equal("/sys/fs/cgroup/blkio/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod88888888_8888.slice", blkIOPath)
		} else {
			a.Equal("/sys/fs/cgroup/blkio/kubepods/burstable/pod88888888-8888", blkIOPath)
		}
	}
}
