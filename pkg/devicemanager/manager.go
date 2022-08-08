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
	"github.com/carina-io/carina/pkg/devicemanager/bcache"
	"k8s.io/apimachinery/pkg/api/equality"
	"regexp"
	"strings"
	"time"

	"github.com/carina-io/carina/pkg/configuration"
	"github.com/carina-io/carina/pkg/devicemanager/lvmd"
	"github.com/carina-io/carina/pkg/devicemanager/partition"
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
	Cache cache.Cache
	// The implementation of executing a console command
	Executor exec.Executor
	// 所有操作本地卷均需获取锁
	Mutex *mutx.GlobalLocks
	// Volume 操作
	VolumeManager volume.LocalVolume
	//磁盘以及分区操作
	Partition partition.LocalPartition
	// stop
	stopChan <-chan struct{}
	nodeName string
	// 本地设备一致性检查
	trouble *troubleshoot.Trouble
	// 配置变更即触发搜索本地磁盘逻辑
	configModifyChan chan struct{}
}

func NewDeviceManager(nodeName string, cache cache.Cache, stopChan <-chan struct{}) *DeviceManager {
	executor := &exec.CommandExecutor{}
	mutex := mutx.NewGlobalLocks()

	dm := DeviceManager{
		Cache:            cache,
		Executor:         executor,
		Mutex:            mutex,
		VolumeManager:    &volume.LocalVolumeImplement{Mutex: mutex, Lv: &lvmd.Lvm2Implement{Executor: executor}, Bcache: &bcache.BcacheImplement{Executor: executor}, NoticeUpdate: make(chan *volume.VolumeEvent)},
		Partition:        &partition.LocalPartitionImplement{Mutex: mutex, CacheParttionNum: make(map[string]uint), Executor: executor},
		stopChan:         stopChan,
		nodeName:         nodeName,
		configModifyChan: make(chan struct{}),
	}
	dm.trouble = troubleshoot.NewTroubleObject(dm.VolumeManager, dm.Partition, cache, nodeName)
	// 注册监听配置变更
	dm.configModifyChan = make(chan struct{}, 1)
	configuration.RegisterListenerChan(dm.configModifyChan)
	return &dm
}

func (dm *DeviceManager) GetNodeDiskSelectGroup() map[string]configuration.DiskSelectorItem {
	diskClass := map[string]configuration.DiskSelectorItem{}
	currentDiskSelector := configuration.DiskSelector()
	node := &corev1.Node{}
	err := dm.Cache.Get(context.Background(), client.ObjectKey{Name: dm.nodeName}, node)
	if err != nil {
		log.Errorf("get node %s error %s", dm.nodeName, err.Error())
		return nil
	}

	for _, v := range currentDiskSelector {
		if v.NodeLabel == "" {
			diskClass[v.Name] = v
		}
		if _, ok := node.Labels[v.NodeLabel]; ok {
			diskClass[v.Name] = v
		}
	}
	return diskClass
}

// addAndRemoveDevice 定时巡检磁盘，是否有新磁盘加入
func (dm *DeviceManager) addAndRemoveDevice() {
	diskClass := dm.GetNodeDiskSelectGroup()
	actuallyVg, err := dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		log.Error("get current vg struct failed: " + err.Error())
		return
	}
	changeBefore := actuallyVg
	log.Debug("ActuallyVg: ", actuallyVg)
	newDisk, err := dm.discoverDisk(diskClass)
	if err != nil {
		log.Error("find new device failed: " + err.Error())
		return
	}
	log.Debug("newDisk: ", newDisk)
	newPv, err := dm.discoverPv(diskClass)
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
	actuallyVgMap := map[string][]string{}
	for _, v := range actuallyVg {
		for _, pv := range v.PVS {
			actuallyVgMap[v.VGName] = append(actuallyVgMap[v.VGName], pv.PVName)
		}
	}
	log.Debug("ActuallyVgMap ", actuallyVgMap)

	// 执行新增磁盘
	for vg, pvs := range newDisk {
		log.Infof("vg:%s, pvs:%s ", vg, pvs)
		for _, pv := range pvs {
			//过滤已经在磁盘组的磁盘
			if v, ok := actuallyVgMap[vg]; ok && utils.ContainsString(v, pv) {
				continue
			}
			if err := dm.VolumeManager.AddNewDiskToVg(pv, vg); err != nil {
				log.Errorf("add new disk failed vg: %s, disk: %s, error: %v", vg, pv, err)
			}
			//同步磁盘分区表
			if err := dm.VolumeManager.GetLv().PartProbe(); err != nil {
				log.Errorf("failed partprobe  error: %v", err)
			}
		}
	}

	time.Sleep(5 * time.Second)
	// 移出磁盘
	// 无法判断单独的PV属于carina管理范围，所以不支持单独对pv remove
	// 若是发生vgreduce成功，但是pvremove失败的情况，并不影响carina工作，也不影响磁盘再次使用
	actuallyVg, err = dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		log.Error("get current vg struct failed: " + err.Error())
		return
	}

	for _, v := range actuallyVg {
		if _, ok := diskClass[v.VGName]; !ok {
			continue
		}

		diskSelector, err := regexp.Compile(strings.Join(diskClass[v.VGName].Re, "|"))
		if err != nil {
			log.Warnf("disk regex %s error %v ", strings.Join(diskClass[v.VGName].Re, "|"), err)
			return
		}
		log.Debug("diskSelector  ", diskSelector)
		for _, pv := range v.PVS {
			if strings.Contains(pv.PVName, "unknown") {
				_ = dm.VolumeManager.GetLv().RemoveUnknownDevice(pv.VGName)
				continue
			}
			//同一个vg里，如果正则不匹配就将磁盘移出vg
			if !diskSelector.MatchString(pv.PVName) {
				log.Infof("remove pv %s in vg %s", pv.PVName, v.VGName)
				if err := dm.VolumeManager.RemoveDiskInVg(pv.PVName, v.VGName); err != nil {
					log.Errorf("remove pv %s error %v", pv.PVName, err)
				}
				if err := dm.VolumeManager.GetLv().PartProbe(); err != nil {
					log.Errorf("failed partprobe  error: %v", err)
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
	if !equality.Semantic.DeepEqual(changeBefore, changeAfter) {
		dm.VolumeManager.NoticeUpdateCapacity(volume.LVMCheck, nil)
	}
}

// discoverDisk 查找是否有符合条件的块设备加入
func (dm *DeviceManager) discoverDisk(diskClass map[string]configuration.DiskSelectorItem) (map[string][]string, error) {
	blockClass := map[string][]string{}
	var name string
	// 列出所有本地磁盘
	localDisk, err := dm.Partition.ListDevicesDetailWithoutFilter("")
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
	// If the disk has been added to a VG group, add it to this vg group
	hasMatchedDisk := map[string]int8{}

	for _, ds := range diskClass {
		if strings.ToLower(ds.Policy) == "raw" {
			// 目前不支持raw磁盘模式
			continue
		}
		diskSelector, err := regexp.Compile(strings.Join(ds.Re, "|"))
		if err != nil {
			log.Warnf("disk regex %s error %v ", strings.Join(ds.Re, "|"), err)
			continue
		}
		// 过滤出空块设备
		for _, d := range localDisk {
			// 如果是其他磁盘Parent直接跳过
			if _, ok := parentDisk[d.Name]; ok {
				continue
			}

			if strings.Contains(d.Name, "cache") {
				continue
			}

			if d.Filesystem == types.Lvm2FsType {
				continue
			}

			// 过滤不支持的磁盘类型
			diskTypeCheck := true
			for _, t := range []string{types.LVMType, types.CryptType, types.MultiPath, types.RomType} {
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
			dused, err := dm.Partition.GetDiskUsed(d.Name)
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
				if hasMatchedDisk[d.Name] == 1 {
					continue
				}
				blockClass[name] = append(blockClass[name], d.Name)
				hasMatchedDisk[d.Name] = 1
			}
		}
	}
	return blockClass, nil
}

// discoverPv 支持发现Pv，由于某些异常情况，只创建成功了PV,并未创建成功VG
func (dm *DeviceManager) discoverPv(diskClass map[string]configuration.DiskSelectorItem) (map[string][]string, error) {
	resp := map[string][]string{}
	var name string
	pvList, err := dm.VolumeManager.GetCurrentPvStruct()
	if err != nil {
		log.Errorf("get pv failed %s", err.Error())
		return nil, err
	}
	for _, ds := range diskClass {
		if strings.ToLower(ds.Policy) == "raw" {
			continue
		}
		diskSelector, err := regexp.Compile(strings.Join(ds.Re, "|"))
		if err != nil {
			log.Warnf("disk regex %s error %v ", strings.Join(ds.Re, "|"), err)
			return resp, err
		}

		for _, pv := range pvList {
			// 如果是属于同一个组,重新配置pv容量大小
			if pv.VGName == ds.Name {
				err := dm.VolumeManager.GetLv().PVResize(pv.PVName)
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
			disk, err := dm.Partition.ListDevicesDetailWithoutFilter(pv.PVName)
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
				dm.trouble.CleanupOrphanPartition()
			case <-dm.stopChan:
				log.Info("stop volume consistency check...")
				return
			}
		}
	}(ticker1)
}

func (dm *DeviceManager) DeviceCheckTask() {
	dm.Cache.WaitForCacheSync(context.Background())
	log.Info("start device scan...")
	dm.VolumeManager.RefreshLvmCache()
	// 服务启动先检查一次
	dm.addAndRemoveDevice()

	monitorInterval := configuration.DiskScanInterval()
	if monitorInterval == 0 {
		monitorInterval = 300
	}

	ticker1 := time.NewTicker(time.Duration(monitorInterval) * time.Second)
	func(t *time.Ticker) {
		defer close(dm.configModifyChan)
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
				dm.addAndRemoveDevice()
				// here for raw storage update, reuse the scan ticker
				dm.VolumeManager.NoticeUpdateCapacity(volume.Dummy, nil)
			case <-dm.configModifyChan:
				log.Info("config modify trigger disk scan...")
				dm.addAndRemoveDevice()
				go time.AfterFunc(10*time.Second, func() { dm.VolumeManager.NoticeUpdateCapacity(volume.ConfigModify, nil) })
			case <-dm.stopChan:
				log.Info("stop device scan...")
				return
			}
		}
	}(ticker1)
}
