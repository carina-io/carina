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

package volume

import (
	"github.com/carina-io/carina/api"
	"github.com/carina-io/carina/pkg/devicemanager/lvmd"
	"github.com/carina-io/carina/pkg/devicemanager/types"
)

// LocalVolume 本接口负责对外提供方法
// 处理业务逻辑并调用lvm接口
type LocalVolume interface {
	CreateVolume(lvName, vgName string, size, ratio uint64) error
	DeleteVolume(lvName, vgName string) error
	ResizeVolume(lvName, vgName string, size, ratio uint64) error
	VolumeList(lvName, vgName string) ([]types.LvInfo, error)
	VolumeInfo(lvName, vgName string) (*types.LvInfo, error)

	// GetCurrentVgStruct 额外的方法
	GetCurrentVgStruct() ([]api.VgGroup, error)
	GetCurrentPvStruct() ([]api.PVInfo, error)
	AddNewDiskToVg(disk, vgName string) error
	RemoveDiskInVg(disk, vgName string) error

	HealthCheck()
	RefreshLvmCache()

	// CreateBcache bcache
	CreateBcache(dev, cacheDev string, block, bucket string, cacheMode string) (*types.BcacheDeviceInfo, error)
	DeleteBcache(dev, cacheDev string) error
	BcacheDeviceInfo(dev string) (*types.BcacheDeviceInfo, error)

	GetLv() lvmd.Lvm2
}
