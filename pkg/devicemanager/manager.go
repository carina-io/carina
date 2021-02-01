package deviceManager

import (
	"carina/pkg/configruation"
	"carina/pkg/devicemanager/device"
	"carina/pkg/devicemanager/lvmd"
	"carina/pkg/devicemanager/types"
	"carina/pkg/devicemanager/volume"
	"carina/utils"
	"carina/utils/exec"
	"carina/utils/log"
	"carina/utils/mutx"
	"regexp"
	"strings"
	"time"
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
	nodeName string
}

func Run() {

	// 第一步： 初始化结构
	// 第二步： 从磁盘加载现有设备及lvm卷
	// 第三步： 启动定时磁盘检查服务
	// 第四步：

}

func NewDeviceManager(nodeName string, stopChan <-chan struct{}) *DeviceManager {
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
		StopChan: stopChan,
		nodeName: nodeName,
	}
	return &dm
}

// 定时巡检磁盘，是否有新磁盘加入
func (dm *DeviceManager) AddAndRemoveDevice() {
	newDisk, err := dm.DiscoverDisk()
	if err != nil {
		log.Error("find new device failed: " + err.Error())
		return
	}
	ActuallyVg, err := dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		log.Error("get current vg struct failed: " + err.Error())
		return
	}
	// 需要新增的磁盘
	needAddPv := newDisk
	ActuallyVgMap := map[string][]string{}
	for _, v := range ActuallyVg {
		for _, pv := range v.PVS {
			ActuallyVgMap[v.VGName] = append(ActuallyVgMap[v.VGName], pv.PVName)
		}
	}

	for vgName, pvs := range newDisk {
		if actuallyPv, ok := ActuallyVgMap[vgName]; ok {
			needAddPv[vgName] = utils.SliceSubSlice(pvs, actuallyPv)
		}
	}
	// 执行新增磁盘
	for vg, pvs := range needAddPv {
		for _, pv := range pvs {
			if err := dm.VolumeManager.AddNewDiskToVg(pv, vg); err != nil {
				log.Errorf("add new disk failed vg: %s, disk: %s, error: %v", vg, pv, err)
			}
		}
	}
	// 移出磁盘
	ActuallyVg, err = dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		log.Error("get current vg struct failed: " + err.Error())
		return
	}

	diskSelector, err := regexp.Compile(strings.Join(configruation.DiskSelector(), "|"))
	if err != nil {
		log.Warnf("disk regex %s error %v ", strings.Join(configruation.DiskSelector(), "|"), err)
		return
	}

	for _, v := range ActuallyVg {
		for _, pv := range v.PVS {
			if !diskSelector.MatchString(pv.PVName) {
				if err := dm.VolumeManager.RemoveDiskInVg(pv.PVName, v.VGName); err != nil {
					log.Errorf("remove disk %s error %v", pv.PVName, err)
				}
			}
		}
	}
}

// 查找是否有符合条件对快设备加入
func (dm *DeviceManager) DiscoverDisk() (map[string][]string, error) {
	blockClass := map[string][]string{}
	// 列出所有本地磁盘
	localDisk, err := dm.DiskManager.ListDevicesDetail()
	if err != nil {
		log.Error("get local disk failed: " + err.Error())
		return blockClass, err
	}
	dsList := configruation.DiskSelector()
	if len(dsList) == 0 {
		log.Info("no set disk selector")
		return blockClass, nil
	}

	diskSelector, err := regexp.Compile(strings.Join(dsList, "|"))
	if err != nil {
		log.Warnf("disk regex %s error %v ", strings.Join(dsList, "|"), err)
		return blockClass, err
	}

	// 过滤出空对块设备
	for _, d := range localDisk {
		if d.Readonly || d.Size < 1>>31 || d.Filesystem == "" || d.MountPoint == "" || d.State != "running" {
			log.Info("mismatched disk: " + d.Name)
			continue
		}

		// 过滤不支持的磁盘类型
		diskTypeCheck := true
		for _, t := range []string{types.LVMType, types.PartType, types.CryptType, types.MultiPath, "rom"} {
			if strings.Contains(d.Type, t) {
				diskTypeCheck = false
				continue
			}
		}
		if !diskTypeCheck {
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
			blockClass[types.VGSSD] = append(blockClass[types.VGSSD], d.Name)
			log.Infof("find new ssd device %s", d.Name)
		} else if d.Rotational == "1" {
			blockClass[types.VGSSD] = append(blockClass[types.VGHDD], d.Name)
			log.Infof("find new hdd device %s", d.Name)
		} else {
			log.Infof("unsupported disk type name: %s, rota: %s", d.Name, d.Rotational)
		}
	}
	return blockClass, nil
}

func (dm *DeviceManager) LvmHealthCheck() {
	log.Info("init health check")

	ticker1 := time.NewTicker(60 * time.Second)
	go func(t *time.Ticker) {
		defer ticker1.Stop()
		for {
			select {
			case <-t.C:
				log.Info("exec lvm check: unrealized")
			case <-dm.StopChan:
				log.Info("stop lvm check")
				return
			}
		}
	}(ticker1)
}

func (dm *DeviceManager) DeviceCheckTask() {
	log.Info("init device check")

	ticker1 := time.NewTicker(60 * time.Second)
	go func(t *time.Ticker) {
		defer ticker1.Stop()
		for {
			select {
			case <-t.C:
				time.Sleep(time.Duration(configruation.DiskScanInterval()-int64(60)) * time.Second)
				log.Info("exec disk check task")
				dm.AddAndRemoveDevice()
			case <-dm.StopChan:
				log.Info("stop disk check task")
				return
			}
		}
	}(ticker1)
}
