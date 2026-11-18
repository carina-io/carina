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
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

const podUid = "88888888-8888"

// useCgroupRoot points cgroupRoot at a temp dir, laid out for the systemd driver if systemd is set.
func useCgroupRoot(t *testing.T, systemd bool) string {
	root := t.TempDir()
	old := cgroupRoot
	cgroupRoot = root
	t.Cleanup(func() { cgroupRoot = old })
	if systemd {
		assert.NoError(t, os.Mkdir(path.Join(root, "kubepods.slice"), 0755))
	}
	return root
}

func TestGetPodCgroupPath(t *testing.T) {
	tests := []struct {
		qos     v1.PodQOSClass
		systemd bool
		want    string
	}{
		{v1.PodQOSGuaranteed, false, "kubepods/pod88888888-8888"},
		{v1.PodQOSBurstable, false, "kubepods/burstable/pod88888888-8888"},
		{v1.PodQOSBestEffort, false, "kubepods/besteffort/pod88888888-8888"},
		{v1.PodQOSGuaranteed, true, "kubepods.slice/kubepods-pod88888888_8888.slice"},
		{v1.PodQOSBurstable, true, "kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod88888888_8888.slice"},
		{v1.PodQOSBestEffort, true, "kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod88888888_8888.slice"},
	}
	for _, tt := range tests {
		root := useCgroupRoot(t, tt.systemd)
		assert.Equal(t, path.Join(root, tt.want), getPodCgroupPath(&PodBlkIO{PodUid: podUid, PodQos: tt.qos}), "qos=%s systemd=%v", tt.qos, tt.systemd)
	}
}

func TestGetCG2IOLimitStr(t *testing.T) {
	tests := []struct {
		limit IOLimit
		want  string
	}{
		{IOLimit{}, "8:16 rbps=max riops=max wbps=max wiops=max"},
		{IOLimit{Rbps: 1, Riops: 2, Wbps: 3, Wiops: 4}, "8:16 rbps=1 riops=2 wbps=3 wiops=4"},
		{IOLimit{Rbps: 10485760, Wiops: 100}, "8:16 rbps=10485760 riops=max wbps=max wiops=100"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, getCG2IOLimitStr("8:16", &tt.limit))
	}
}

func TestSetIOLimit(t *testing.T) {
	root := useCgroupRoot(t, false)
	podDir := path.Join(root, "kubepods/burstable/pod"+podUid)
	blkIO := &PodBlkIO{
		PodUid:      podUid,
		PodQos:      v1.PodQOSBurstable,
		DeviceIOSet: DeviceIOSet{"8:16": {Rbps: 1048576, Wiops: 100}},
	}

	err := SetIOLimit(blkIO)
	assert.ErrorContains(t, err, "cgroup path")

	assert.NoError(t, os.MkdirAll(podDir, 0755))
	err = SetIOLimit(blkIO)
	assert.ErrorContains(t, err, "io controller is not enabled in "+path.Join(root, "kubepods/burstable/cgroup.subtree_control"))

	ioMax := path.Join(podDir, "io.max")
	assert.NoError(t, os.WriteFile(ioMax, nil, 0600))
	assert.NoError(t, SetIOLimit(blkIO))
	got, _ := os.ReadFile(ioMax)
	assert.Equal(t, "8:16 rbps=1048576 riops=max wbps=max wiops=100", string(got))

	// Removing the annotations yields a zero IOLimit, which must reset the device to max.
	blkIO.DeviceIOSet["8:16"] = &IOLimit{}
	assert.NoError(t, SetIOLimit(blkIO))
	got, _ = os.ReadFile(ioMax)
	assert.Equal(t, "8:16 rbps=max riops=max wbps=max wiops=max", string(got))
}

func TestIsCgroup2UnifiedMode(t *testing.T) {
	root := useCgroupRoot(t, false)
	assert.False(t, IsCgroup2UnifiedMode())
	assert.NoError(t, os.WriteFile(path.Join(root, "cgroup.controllers"), []byte("io memory"), 0644))
	assert.True(t, IsCgroup2UnifiedMode())
}
