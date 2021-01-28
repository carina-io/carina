package deviceManager

import (
	"carina/pkg/devicemanager/device"
	"carina/pkg/devicemanager/lvmd"
	"carina/pkg/devicemanager/types"
	"carina/utils/exec"
	"carina/utils/mutx"
)

type DeviceManager struct {

	// The implementation of executing a console command
	Executor exec.Executor
	// 当前真实vg关系
	ActuallyVgGroup *types.VgGroup
	// 期望对vg关系
	DesiredVgGroup *types.VgGroup
	// 所有操作本地卷均需获取锁
	Mutex *mutx.GlobalLocks
	// 磁盘操作
	DiskManager device.LocalDevice
	// LVM 操作
	LvmManger lvmd.Lvm2
}

func Run() {

	// 第一步： 初始化结构
	// 第二步： 从磁盘加载现有设备及lvm卷
	// 第三步： 启动定时磁盘检查服务
	// 第四步：

}

func NewDeviceManager() *DeviceManager {
	executor := &exec.CommandExecutor{}
	mutex := &mutx.GlobalLocks{}
	dm := DeviceManager{
		Executor:        executor,
		ActuallyVgGroup: nil,
		DesiredVgGroup:  nil,
		Mutex:           mutex,
		DiskManager:     &device.LocalDeviceImplement{Executor: executor},
		LvmManger:       &lvmd.Lvm2Implement{Executor: executor},
	}
	return &dm
}

// 定时巡检磁盘，是否有新磁盘加入
func (dm *DeviceManager) DiscoverDisk() {

}

func (dm *DeviceManager) LvmHealthCheck() {

}
