package deviceManager

import (
	"carina/pkg/configruation"
	"carina/pkg/devicemanager/device"
	"carina/pkg/devicemanager/lvmd"
	"carina/pkg/devicemanager/types"
	"carina/pkg/devicemanager/volume"
	"carina/utils/exec"
	"carina/utils/log"
	"carina/utils/mutx"
	"regexp"
	"strings"
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
	LvmManager lvmd.Lvm2
	// Volume 操作
	VolumeManager volume.LocalVolume
	// stop
	StopChan <-chan struct{}
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
		LvmManager:      &lvmd.Lvm2Implement{Executor: executor},
		VolumeManager: &volume.LocalVolumeImplement{
			Mutex: mutex,
			Lv:    &lvmd.Lvm2Implement{Executor: executor},
		},
	}
	return &dm
}

// 定时巡检磁盘，是否有新磁盘加入
func (dm *DeviceManager) FindAndAddNewDeviceToVG() {
	newDisk, err := dm.DiscoverDisk()
	if err != nil {
		log.Error("find new device failed: "+ err.Error())
		return
	}
	vg, err := dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		log.Error("get current vg struct failed: " + err.Error())
		return
	}

	for _, v := range vg {
		v.VGName
		if ds, ok := newDisk[v.VGName]; ok {

		}
	}



}

// 查找是否有符合条件对快设备加入
func (dm *DeviceManager) DiscoverDisk() (map[string]string, error) {
	blockClass := map[string]string{}
	// 列出所有本地磁盘
	localDisk, err := dm.DiskManager.ListDevicesDetail()
	if err != nil {
		log.Error("get local disk failed: " + err.Error())
		return blockClass, err
	}

	diskSelector, err := regexp.Compile(strings.Join(configruation.GlobalConfig.DiskSelector, "|"))
	if err != nil {
		log.Warnf("设置磁盘正则表达式错误: " + strings.Join(configruation.GlobalConfig.DiskSelector, "|"))
		return blockClass, err
	}

	// 过滤出空对块设备
	for _, d := range localDisk {
		if d.Readonly || d.Size < 1>>31 || d.Filesystem == "" || d.MountPoint == "" || d.State != "running" {
			log.Info("mismatched disk: " + d.Name)
			continue
		}
		if strings.Contains(d.Type, "lvm") || strings.Contains(d.Type, "part") || strings.Contains(d.Type, "rom") {
			log.Info("mismatched disk: " + d.Name)
			continue
		}

		if !diskSelector.MatchString(d.Name) {
			log.Info("mismatched disk: " + d.Name)
			continue
		}

		// 判断设备是否已经存在数据
		dused, err := dm.DiskManager.GetDiskUsed(d.Name)
		if err != nil {
			log.Warnf("get disk %s used failed %v", d.Name, err)
			continue
		}
		if dused > 0 {
			log.Warnf("block device don't empty " + d.Name)
			continue
		}

		if d.Rotational == "0" {
			blockClass[types.VGSSD] = d.Name
			log.Infof("find new ssd device %s", d.Name)
		} else if d.Rotational == "1" {
			blockClass[types.VGHDD] = d.Name
			log.Infof("find new hdd device %s", d.Name)
		} else {
			log.Infof("unsupported disk type name: %s, rota: %s", d.Name, d.Rotational)
		}
	}
	return blockClass, nil
}

func (dm *DeviceManager) LvmHealthCheck() {

}
