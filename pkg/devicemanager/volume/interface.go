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
	"github.com/aws/aws-sdk-go/service/accessanalyzer"
	"github.com/bocloud/carina/pkg/devicemanager/types"
)

const (
	THIN     = "thin-"
	SNAP     = "snap-"
	LVVolume = "volume-"
)

// 本接口负责对外提供方法
// 处理业务逻辑并调用lvm接口
type LocalVolume interface {
	CreateVolume(lvName, vgName string, size, ratio uint64) error
	DeleteVolume(lvName, vgName string) error
	ResizeVolume(lvName, vgName string, size, ratio uint64) error
	VolumeList(lvName, vgName string) ([]types.LvInfo, error)
	VolumeInfo(lvName, vgName string) (*types.LvInfo, error)

	CreateSnapshot(snapName, lvName, vgName string) error
	DeleteSnapshot(snapName, vgName string) error
	RestoreSnapshot(snapName, vgName string) error
	SnapshotList(lvName, vgName string) ([]types.LvInfo, error)

	CloneVolume(lvName, vgName, newLvName string) error

	// 额外的方法
	GetCurrentVgStruct() ([]types.VgGroup, error)
	GetCurrentPvStruct() ([]types.PVInfo, error)
	AddNewDiskToVg(disk, vgName string) error
	RemoveDiskInVg(disk, vgName string) error

	HealthCheck()
	RefreshLvmCache()
	// For Device Plugin
	NoticeUpdateCapacity(vgName []string)
	// 注册通知服务，因为多个vg组，每个组需要不同的channel
	RegisterNoticeServer(vgName string, notice chan struct{})

	// bcache
	CreateBcache(dev, cacheDev string) (string, error)
	RemoveBcache(dev, cacheDev string) error
	BcacheDeviceInfo(dev string) (*types.BcacheDeviceInfo, error)
}
