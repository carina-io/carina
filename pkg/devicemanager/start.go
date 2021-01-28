package deviceManager

import (
	"carina/pkg/devicemanager/types"
	"carina/utils/exec"
	"carina/utils/mutx"
)

type DeviceManager struct {

	// The implementation of executing a console command
	Executor exec.Executor
	// The local devices detected on the node
	Devices []*types.LocalDisk
	// 所有操作本地卷均需获取锁
	Mutex *mutx.GlobalLocks

	// LVM 操作

}

func run() {

	// 第一步： 初始化结构
	// 第二步： 从磁盘加载现有设备及lvm卷
	// 第三步： 启动定时磁盘检查服务
	// 第四步：

}

func FormatDevice() {

}
