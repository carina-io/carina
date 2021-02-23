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
	// 所有操作本地卷均需获取锁
	Mutex *mutx.GlobalLocks
	// 磁盘操作
	DiskManager device.LocalDevice
	// LVM 操作
	LvmManager lvmd.Lvm2
	// Volume 操作
	VolumeManager volume.LocalVolume
	// stop
	stopChan <-chan struct{}
	nodeName string
	// 磁盘选择器
	diskSelector []string
}

func NewDeviceManager(nodeName string, stopChan <-chan struct{}) *DeviceManager {
	executor := &exec.CommandExecutor{}
	mutex := mutx.NewGlobalLocks()
	dm := DeviceManager{
		Executor:    executor,
		Mutex:       mutex,
		DiskManager: &device.LocalDeviceImplement{Executor: executor},
		LvmManager:  &lvmd.Lvm2Implement{Executor: executor},
		VolumeManager: &volume.LocalVolumeImplement{
			Mutex:           mutex,
			Lv:              &lvmd.Lvm2Implement{Executor: executor},
			NoticeServerMap: make(map[string]chan struct{}),
		},
		stopChan: stopChan,
		nodeName: nodeName,
	}
	return &dm
}

// 定时巡检磁盘，是否有新磁盘加入
func (dm *DeviceManager) AddAndRemoveDevice() {
	// 判断配置是否更改，若是没有更改没必要扫描磁盘
	noErrorFlag := true
	currentDiskSelector := configruation.DiskSelector()
	if utils.SliceEqualSlice(dm.diskSelector, currentDiskSelector) {
		log.Info("no change disk selector")
		return
	}

	newDisk, err := dm.DiscoverDisk()
	if err != nil {
		log.Error("find new device failed: " + err.Error())
		return
	}
	newPv, err := dm.DiscoverPv()
	if err != nil {
		log.Error("find new pv failed: " + err.Error())
		return
	}

	// 合并新增设备
	for key, value := range newDisk {
		if v, ok := newPv[key]; ok {
			newDisk[key] = utils.SliceMergeSlice(value, v)
		}
	}
	for key, value := range newPv {
		if _, ok := newDisk[key]; !ok {
			newDisk[key] = value
		}
	}

	ActuallyVg, err := dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		log.Error("get current vg struct failed: " + err.Error())
		return
	}
	// 需要新增的磁盘, 处理成容易比较的数据
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
				noErrorFlag = false
			}
		}
	}
	time.Sleep(5 * time.Second)
	// 移出磁盘
	// 无法判断单独的PV属于carina管理范围，所以不支持单独对pv remove
	// 若是发生vgreduce成功，但是pvremove失败的情况，并不影响carina工作，也不影响磁盘再次使用
	ActuallyVg, err = dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		log.Error("get current vg struct failed: " + err.Error())
		return
	}

	diskSelector, err := regexp.Compile(strings.Join(currentDiskSelector, "|"))
	if err != nil {
		log.Warnf("disk regex %s error %v ", strings.Join(configruation.DiskSelector(), "|"), err)
		return
	}

	for _, v := range ActuallyVg {
		for _, pv := range v.PVS {
			if !diskSelector.MatchString(pv.PVName) {
				if err := dm.VolumeManager.RemoveDiskInVg(pv.PVName, v.VGName); err != nil {
					log.Errorf("remove disk %s error %v", pv.PVName, err)
					noErrorFlag = false
				}
			}
		}
	}
	if noErrorFlag {
		dm.diskSelector = currentDiskSelector
	}
}

// 查找是否有符合条件的块设备加入
func (dm *DeviceManager) DiscoverDisk() (map[string][]string, error) {
	blockClass := map[string][]string{}
	// 列出所有本地磁盘
	localDisk, err := dm.DiskManager.ListDevicesDetail("")
	if err != nil {
		log.Error("get local disk failed: " + err.Error())
		return blockClass, err
	}
	if len(localDisk) == 0 {
		log.Info("cannot find new device")
		return blockClass, nil
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

	parentDisk := map[string]int8{}
	for _, d := range localDisk {
		parentDisk[d.ParentName] = 1
	}
	// 过滤出空块设备
	for _, d := range localDisk {
		if strings.Contains(d.Name, types.KEYWORD) {
			continue
		}
		// 如果是其他磁盘对Parent直接跳过
		if _, ok := parentDisk[d.Name]; ok {
			continue
		}

		if d.Readonly || d.Size < 10<<30 || d.Filesystem != "" || d.MountPoint != "" || d.State == "running" {
			log.Infof("mismatched disk: %s filesystem:%s mountpoint:%s state:%s, readonly:%t, size:%d", d.Name, d.Filesystem, d.MountPoint, d.State, d.Readonly, d.Size)
			continue
		}

		// 过滤不支持的磁盘类型
		diskTypeCheck := true
		for _, t := range []string{types.LVMType, types.PartType, types.CryptType, types.MultiPath, "rom"} {
			if strings.Contains(d.Type, t) {
				diskTypeCheck = false
				break
			}
		}
		if !diskTypeCheck {
			log.Infof("mismatched disk:%s, disktype:%s", d.Name, d.Type)
			continue
		}

		if !diskSelector.MatchString(d.Name) {
			log.Infof("mismatched disk:%s, regex:%s", d.Name, diskSelector.String())
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
			log.Infof("eligible ssd device %s", d.Name)
		} else if d.Rotational == "1" {
			blockClass[types.VGHDD] = append(blockClass[types.VGHDD], d.Name)
			log.Infof("eligible hdd device %s", d.Name)
		} else {
			log.Infof("unsupported disk type name: %s, rota: %s", d.Name, d.Rotational)
		}
	}

	return blockClass, nil
}

// 支持发现Pv，由于某些异常情况，只创建成功了PV,并未创建成功VG
func (dm *DeviceManager) DiscoverPv() (map[string][]string, error) {
	resp := map[string][]string{}
	pvList, err := dm.VolumeManager.GetCurrentPvStruct()
	if err != nil {
		log.Errorf("get pv failed %s", err.Error())
		return nil, err
	}
	dsList := configruation.DiskSelector()
	if len(dsList) == 0 {
		log.Info("no set disk selector")
		return resp, nil
	}
	diskSelector, err := regexp.Compile(strings.Join(dsList, "|"))
	if err != nil {
		log.Warnf("disk regex %s error %v ", strings.Join(dsList, "|"), err)
		return resp, err
	}
	for _, pv := range pvList {
		if pv.VGName != "" {
			continue
		}
		if !diskSelector.MatchString(pv.PVName) {
			log.Infof("mismatched pv:%s, regex:%s", pv.PVName, diskSelector.String())
			continue
		}
		disk, err := dm.DiskManager.ListDevicesDetail(pv.PVName)
		if err != nil {
			log.Errorf("get device failed %s", err.Error())
			continue
		}
		if len(disk) != 1 {
			log.Error("get disk count not equal 1")
			continue
		}
		if disk[0].Rotational == "0" {
			resp[types.VGSSD] = append(resp[types.VGSSD], disk[0].Name)
			log.Infof("eligible ssd pv %s", disk[0].Name)
		} else if disk[0].Rotational == "1" {
			resp[types.VGHDD] = append(resp[types.VGHDD], disk[0].Name)
			log.Infof("eligible hdd pv %s", disk[0].Name)
		} else {
			log.Infof("unsupported disk type name: %s, rota: %s", disk[0].Name, disk[0].Rotational)
		}
	}
	return resp, nil
}

func (dm *DeviceManager) LvmHealthCheck() {

	ticker1 := time.NewTicker(60 * time.Second)
	go func(t *time.Ticker) {
		defer ticker1.Stop()
		for {
			select {
			case <-t.C:
				log.Info("volume health check...")
				//dm.VolumeManager.HealthCheck()
			case <-dm.stopChan:
				log.Info("stop volume health check...")
				return
			}
		}
	}(ticker1)
}

func (dm *DeviceManager) DeviceCheckTask() {
	log.Info("start device monitor...")
	dm.VolumeManager.RefreshLvmCache()
	// 服务启动先检查一次
	dm.AddAndRemoveDevice()

	ticker1 := time.NewTicker(120 * time.Second)
	go func(t *time.Ticker) {
		defer ticker1.Stop()
		for {
			select {
			case <-t.C:
				time.Sleep(time.Duration(configruation.DiskScanInterval()-int64(120)) * time.Second)
				log.Info("device monitor...")
				dm.AddAndRemoveDevice()
			case <-dm.stopChan:
				log.Info("stop device monitor...")
				return
			}
		}
	}(ticker1)
}
