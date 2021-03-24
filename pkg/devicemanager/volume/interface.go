package volume

import (
	"bocloud.com/cloudnative/carina/pkg/devicemanager/types"
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
}
