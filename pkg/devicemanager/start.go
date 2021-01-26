package deviceManager

import (
	"carina/pkg/devicemanager/types"
	"carina/utils/exec"
)

type Context struct {

	// The implementation of executing a console command
	Executor exec.Executor
	// The root configuration directory used by services
	ConfigDir string

	// The local devices detected on the node
	Devices []*types.LocalDisk
}

func run() {

	// 第一步： 初始化结构
	// 第二步： 从磁盘加载现有设备及lvm卷
	// 第三步： 启动定时磁盘检查服务
	// 第四步：

}

func FormatDevice() {

}
