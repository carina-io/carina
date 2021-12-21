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
package deviceManager

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/carina-io/carina/pkg/configuration"
	"github.com/carina-io/carina/pkg/devicemanager/bcache"
	"github.com/carina-io/carina/pkg/devicemanager/device"
	"github.com/carina-io/carina/pkg/devicemanager/lvmd"
	"github.com/carina-io/carina/pkg/devicemanager/troubleshoot"
	"github.com/carina-io/carina/pkg/devicemanager/types"
	"github.com/carina-io/carina/pkg/devicemanager/volume"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/exec"
	"github.com/carina-io/carina/utils/log"
	"github.com/carina-io/carina/utils/mutx"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeviceManager struct {
	client.Client
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
	// bcache
	Bcache bcache.Bcache
	// stop
	stopChan <-chan struct{}
	nodeName string
	// 本地设备一致性检查
	trouble *troubleshoot.Trouble
	// 配置变更即触发搜索本地磁盘逻辑
	configModifyChan chan struct{}
}

func NewDeviceManager(client client.Client,nodeName string, cache cache.Cache, stopChan <-chan struct{}) *DeviceManager {
	executor := &exec.CommandExecutor{}
	mutex := mutx.NewGlobalLocks()
	dm := DeviceManager{
		Client:           client,
		Executor:         executor,
		Mutex:            mutex,
		DiskManager:      &device.LocalDeviceImplement{Executor: executor},
		LvmManager:       &lvmd.Lvm2Implement{Executor: executor},
		VolumeManager:    &volume.LocalVolumeImplement{Mutex: mutex, Lv: &lvmd.Lvm2Implement{Executor: executor}, Bcache: &bcache.BcacheImplement{Executor: executor}, NoticeServerMap: make(map[string]chan struct{})},
		Bcache:           &bcache.BcacheImplement{Executor: executor},
		stopChan:         stopChan,
		nodeName:         nodeName,
		trouble:          &troubleshoot.Trouble{},
		configModifyChan: make(chan struct{}),
	}
	dm.trouble = troubleshoot.NewTroubleObject(dm.VolumeManager, cache, nodeName)
	// 注册监听配置变更
	dm.configModifyChan = make(chan struct{}, 1)
	configuration.RegisterListenerChan(dm.configModifyChan)

	return &dm
}
func (dm *DeviceManager) CheckNodeLables(nodelabekey string) (flag bool, err error) {
	//nodelabekey 为空，对所有节点有效
	if nodelabekey == "" {
		return true, nil
	}
	nodeList := &corev1.NodeList{}
	if err = dm.Client.List(context.Background(), nodeList); err != nil {
		return false, err
	}
	for _, n := range nodeList.Items {
		if _, ok := n.Annotations[nodelabekey]; ok && n.Name != dm.nodeName {
			return true, nil
		}
	}
	return false, nil
}

// 定时巡检磁盘，是否有新磁盘加入
func (dm *DeviceManager) AddAndRemoveDevice() {

	currentDiskSelector := configuration.DiskSelector()
	diskClass := configuration.NewDiskClass(currentDiskSelector)
	ActuallyVg, err := dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		log.Error("get current vg struct failed: " + err.Error())
		return
	}
	changeBefore := ActuallyVg
	log.Debug("ActuallyVg: ", ActuallyVg)
	newDisk, err := dm.DiscoverDisk()
	if err != nil {
		log.Error("find new device failed: " + err.Error())
		return
	}
	log.Debug("newDisk: ", newDisk)
	newPv, err := dm.DiscoverPv()
	if err != nil {
		log.Error("find new pv failed: " + err.Error())
		return
	}
	log.Debug("newPv: ", newPv)

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
	log.Debug("newDisk:", newDisk)
	// 需要新增的磁盘, 处理成容易比较的数据
	needAddPv := newDisk
	ActuallyVgMap := map[string][]string{}
	for _, v := range ActuallyVg {
		for _, pv := range v.PVS {
			ActuallyVgMap[v.VGName] = append(ActuallyVgMap[v.VGName], pv.PVName)
		}
	}
	log.Debug("ActuallyVgMap ", ActuallyVgMap)
	for vgName, pvs := range newDisk {
		if actuallyPv, ok := ActuallyVgMap[vgName]; ok {
			needAddPv[vgName] = utils.SliceSubSlice(pvs, actuallyPv)
		}
	}

	// 执行新增磁盘
	log.Debug("needAddPv ", needAddPv)

	for vg, pvs := range needAddPv {
		log.Infof("vg:%s ,pvs:%s ", vg, pvs)
		for _, pv := range pvs {
			if err := dm.VolumeManager.AddNewDiskToVg(pv, vg); err != nil {
				log.Errorf("add new disk failed vg: %s, disk: %s, error: %v", vg, pv, err)
			}
			//同步磁盘分区表
			if err := dm.LvmManager.PartProbe(); err != nil {
				log.Errorf("faild partprobe  error: %v", err)
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
	for _, v := range ActuallyVg {
		if _, ok := diskClass.DiskClassByName[v.VGName]; !ok {
			continue
		}
		diskSelector, err := regexp.Compile(strings.Join(diskClass.DiskClassByName[v.VGName].Re, "|"))
		if err != nil {
			log.Warnf("disk regex %s error %v ", strings.Join(diskClass.DiskClassByName[v.VGName].Re, "|"), err)
			return
		}
		log.Debug("diskSelector  ", diskSelector)
		for _, pv := range v.PVS {
			if strings.Contains(pv.PVName, "unknown") {
				_ = dm.LvmManager.RemoveUnknownDevice(pv.VGName)
				continue
			}

			//同一个vg里，如果正则不匹配就将磁盘移出vg
			if !diskSelector.MatchString(pv.PVName) {
				log.Infof("remove pv %s in vg %s", pv.PVName, v.VGName)
				if err := dm.VolumeManager.RemoveDiskInVg(pv.PVName, v.VGName); err != nil {
					log.Errorf("remove pv %s error %v", pv.PVName, err)
				}
				if err := dm.LvmManager.PartProbe(); err != nil {
					log.Errorf("faild partprobe  error: %v", err)
				}
			}

		}
	}

	changeAfter, err := dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		log.Error("get current vg struct failed: " + err.Error())
		return
	}
	log.Debug("new vgs ", changeAfter)
	if validateVg(changeBefore, changeAfter) {
		dm.VolumeManager.NoticeUpdateCapacity([]string{})
	}
}

// 查找是否有符合条件的块设备加入
func (dm *DeviceManager) DiscoverDisk() (map[string][]string, error) {
	blockClass := map[string][]string{}
	var name string
	dsList := configuration.DiskSelector()
	if len(dsList) == 0 {
		log.Info("cannot find new device")
		return blockClass, nil
	}
	diskClass := configuration.NewDiskClass(dsList)
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

	parentDisk := map[string]int8{}
	for _, d := range localDisk {
		parentDisk[d.ParentName] = 1
	}

	for _, ds := range diskClass.DiskClassByName {
		diskSelector, err := regexp.Compile(strings.Join(ds.Re, "|"))
		if err != nil {
			log.Warnf("disk regex %s error %v ", strings.Join(ds.Re, "|"), err)
			return blockClass, err
		}

		// 过滤出空块设备
		for _, d := range localDisk {
			if strings.Contains(d.Name, types.KEYWORD) {
				continue
			}
			// 如果是其他磁盘Parent直接跳过
			if _, ok := parentDisk[d.Name]; ok {
				continue
			}

			if d.Readonly || d.Size < 10<<30 || d.Filesystem != "" || d.MountPoint != "" {
				log.Infof("mismatched disk: %s filesystem:%s mountpoint:%s readonly:%t, size:%d", d.Name, d.Filesystem, d.MountPoint, d.Readonly, d.Size)
				continue
			}

			if strings.Contains(d.Name, "cache") {
				continue
			}

			// 过滤不支持的磁盘类型
			diskTypeCheck := true
			for _, t := range []string{types.LVMType, types.CryptType, types.MultiPath, "rom"} {
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
			name = ds.Name
			log.Infof("eligible %s device %s", ds.Name, d.Name)
			if !utils.ContainsString(blockClass[name], d.Name) {
				blockClass[name] = append(blockClass[name], d.Name)
			} else {
				//磁盘符合多个匹配，抛出异常不处理。
				log.Warnf("mutimatched disk:%s, regex:%s", d.Name, diskSelector.String())
				return map[string][]string{}, fmt.Errorf("mutimatched disk:%s, regex:%s", d.Name, diskSelector.String())
			}

		}

	}
	return blockClass, nil
}

// 支持发现Pv，由于某些异常情况，只创建成功了PV,并未创建成功VG
func (dm *DeviceManager) DiscoverPv() (map[string][]string, error) {
	resp := map[string][]string{}
	var name string
	dsList := configuration.DiskSelector()
	if len(dsList) == 0 {
		log.Info("disk selector cannot not be empty, skip pv scan")
		return resp, nil
	}
	diskClass := configuration.NewDiskClass(dsList)
	pvList, err := dm.VolumeManager.GetCurrentPvStruct()
	if err != nil {
		log.Errorf("get pv failed %s", err.Error())
		return nil, err
	}
	for _, ds := range diskClass.DiskClassByName {
		diskSelector, err := regexp.Compile(strings.Join(ds.Re, "|"))
		if err != nil {
			log.Warnf("disk regex %s error %v ", strings.Join(ds.Re, "|"), err)
			return resp, err
		}

		for _, pv := range pvList {
			//如果是属于同一个组,重新配置pv容量大小
			if pv.VGName == ds.Name {
				err := dm.LvmManager.PVResize(pv.PVName)
				if err != nil {
					log.Errorf("resize %s error", pv.PVName)
				}
			}
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
			name = ds.Name
			log.Infof("eligible %s pv %s", ds.Name, disk[0].Name)
			if !utils.ContainsString(resp[name], disk[0].Name) {
				resp[name] = append(resp[name], disk[0].Name)
			}

		}
	}
	return resp, nil
}

func (dm *DeviceManager) VolumeConsistencyCheck() {

	ticker1 := time.NewTicker(600 * time.Second)
	go func(t *time.Ticker) {
		defer ticker1.Stop()
		for {
			select {
			case <-t.C:
				log.Info("volume consistency check...")
				dm.trouble.CleanupOrphanVolume()
			case <-dm.stopChan:
				log.Info("stop volume consistency check...")
				return
			}
		}
	}(ticker1)
}

func (dm *DeviceManager) DeviceCheckTask() {
	log.Info("start device scan...")
	dm.VolumeManager.RefreshLvmCache()
	// 服务启动先检查一次
	dm.AddAndRemoveDevice()

	monitorInterval := configuration.DiskScanInterval()
	if monitorInterval == 0 {
		monitorInterval = 300
	}

	ticker1 := time.NewTicker(time.Duration(monitorInterval) * time.Second)
	go func(t *time.Ticker) {
		defer ticker1.Stop()
		for {
			select {
			case <-t.C:
				if configuration.DiskScanInterval() == 0 {
					ticker1.Reset(300 * time.Second)
					log.Info("skip disk discovery...")
					continue
				}

				if monitorInterval != configuration.DiskScanInterval() {
					monitorInterval = configuration.DiskScanInterval()
					ticker1.Reset(time.Duration(monitorInterval) * time.Second)
				}

				log.Infof("clock %d second device scan...", configuration.DiskScanInterval())
				dm.AddAndRemoveDevice()

			case <-dm.configModifyChan:
				log.Info("config modify trigger disk scan...")
				dm.AddAndRemoveDevice()
			case <-dm.stopChan:
				log.Info("stop device scan...")
				return
			}
		}
	}(ticker1)
}

func validateVg(src []types.VgGroup, dst []types.VgGroup) bool {
	if len(src) != len(dst) {
		return true
	}
	dstMap := map[string]uint64{}
	for _, d := range dst {
		dstMap[d.VGName] = d.VGSize
	}

	for _, s := range src {
		if v, ok := dstMap[s.VGName]; !ok {
			return true
		} else if s.VGSize != v {
			return true
		}
	}
	return false
}
