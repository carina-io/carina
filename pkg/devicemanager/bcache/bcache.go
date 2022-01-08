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

package bcache

import (
	"fmt"
	"github.com/carina-io/carina/pkg/devicemanager/types"
	"github.com/carina-io/carina/utils/exec"
)

type BcacheImplement struct {
	Executor exec.Executor
}

func (bi *BcacheImplement) CreateBcache(dev, cacheDev string, block, bucket string) error {
	_ = bi.Executor.ExecuteCommand("wipefs", "-af", dev)
	_ = bi.Executor.ExecuteCommand("wipefs", "-af", cacheDev)

	if block != "" && bucket != "" {
		return bi.Executor.ExecuteCommand("make-bcache", "--block", block, "--bucket", bucket, "-B", dev, "-C", cacheDev, "--wipe-bcache")
	}

	return bi.Executor.ExecuteCommand("make-bcache", "-B", dev, "-C", cacheDev, "--wipe-bcache")
}

func (bi *BcacheImplement) RemoveBcache(bcacheInfo *types.BcacheDeviceInfo) error {

	var err error
	var cmd string

	// remove cache device
	cmd = fmt.Sprintf("echo %s > /sys/block/%s/bcache/detach", bcacheInfo.CsetUuid, bcacheInfo.Name)
	err = bi.Executor.ExecuteCommand("/bin/sh", "-c", cmd)
	if err != nil {
		return err
	}

	// unregister cache device
	cmd = fmt.Sprintf("echo 1 > /sys/fs/bcache/%s/unregister", bcacheInfo.CsetUuid)
	_ = bi.Executor.ExecuteCommand("/bin/sh", "-c", cmd)

	// umount /dev/
	_ = bi.Executor.ExecuteCommand("umount", fmt.Sprintf("/dev/%s", bcacheInfo.Name))

	// stop backend device
	cmd = fmt.Sprintf("echo 1 > /sys/block/%s/bcache/stop", bcacheInfo.Name)
	err = bi.Executor.ExecuteCommand("/bin/sh", "-c", cmd)
	if err != nil {
		return err
	}

	return nil
}

// GetDeviceBcache lsblk --pairs --noheadings --output KNAME,MAJ:MIN /dev/hdd/pvc-test-v1
func (bi *BcacheImplement) GetDeviceBcache(dev string) (*types.BcacheDeviceInfo, error) {
	deviceInfo, err := bi.Executor.ExecuteCommandWithOutput("lsblk", "--pairs", "--noheadings", "--output", "KNAME,MAJ:MIN", dev)
	if err != nil {
		return nil, err
	}
	return parseDevice(deviceInfo), nil
}

func (bi *BcacheImplement) RegisterDevice(dev ...string) error {
	for _, d := range dev {
		err := bi.Executor.ExecuteCommand("bcache-register", d)
		if err != nil {
			return err
		}
	}
	return nil
}

func (bi *BcacheImplement) ShowDevice(dev string) (*types.BcacheDeviceInfo, error) {
	bcacheInfo, err := bi.Executor.ExecuteCommandWithOutput("bcache-super-show", "-f", dev)
	if err != nil {
		return nil, err
	}
	return parseBcache(bcacheInfo), nil
}

func (bi *BcacheImplement) SetCacheMode(bcache string, cachePolicy string) error {
	cmd := fmt.Sprintf("echo %s > /sys/block/%s/bcache/cache_mode", cachePolicy, bcache)
	return bi.Executor.ExecuteCommand("/bin/sh", "-c", cmd)
}
