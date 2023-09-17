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

package runners

import (
	"context"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/carina-io/carina/pkg/configuration"
	deviceManager "github.com/carina-io/carina/pkg/devicemanager"
	"github.com/carina-io/carina/pkg/devicemanager/types"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
)

var _ manager.LeaderElectionRunnable = &deviceCheck{}

type deviceCheck struct {
	dm *deviceManager.DeviceManager
	// 配置变更即触发搜索本地磁盘逻辑
	configModifyChan chan struct{}
}

func NewDeviceCheck(dm *deviceManager.DeviceManager) manager.Runnable {
	dc := &deviceCheck{
		dm: dm,
	}
	// 注册监听配置变更
	dc.configModifyChan = make(chan struct{}, 1)
	configuration.RegisterListenerChan(dc.configModifyChan)
	return dc
}

func (dc *deviceCheck) Start(ctx context.Context) error {
	log.Info("Starting device scan...")
	dc.dm.VolumeManager.RefreshLvmCache()
	// 服务启动先检查一次
	dc.addAndRemoveDevice()

	monitorInterval := configuration.DiskScanInterval()
	if monitorInterval == 0 {
		monitorInterval = 300
	}

	ticker := time.NewTicker(time.Duration(monitorInterval) * time.Second)
	func(t *time.Ticker) {
		defer close(dc.configModifyChan)
		defer ticker.Stop()
		for {
			select {
			case <-t.C:
				if configuration.DiskScanInterval() == 0 {
					ticker.Reset(300 * time.Second)
					log.Info("skip disk discovery...")
					continue
				}

				if monitorInterval != configuration.DiskScanInterval() {
					monitorInterval = configuration.DiskScanInterval()
					ticker.Reset(time.Duration(monitorInterval) * time.Second)
				}

				log.Infof("clock %d second device scan...", configuration.DiskScanInterval())
				dc.addAndRemoveDevice()
				// here for raw storage update, reuse the scan ticker
				dc.dm.NoticeUpdateCapacity(deviceManager.Dummy, nil)
			case <-dc.configModifyChan:
				log.Info("config modify trigger disk scan...")
				dc.addAndRemoveDevice()
				go time.AfterFunc(10*time.Second, func() { dc.dm.NoticeUpdateCapacity(deviceManager.ConfigModify, nil) })
			case <-ctx.Done():
				log.Info("stop device scan...")
				return
			}
		}
	}(ticker)
	return nil
}

// addAndRemoveDevice 定时巡检磁盘，是否有新磁盘加入
func (dc *deviceCheck) addAndRemoveDevice() {
	diskClass := dc.dm.GetNodeDiskSelectGroup()
	actuallyVg, err := dc.dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		log.Error("get current vg struct failed: " + err.Error())
		return
	}
	changeBefore := actuallyVg
	log.Debug("ActuallyVg: ", actuallyVg)
	newDisk, err := dc.discoverDisk(diskClass)
	if err != nil {
		log.Error("find new device failed: " + err.Error())
		return
	}
	log.Debug("newDisk: ", newDisk)
	newPv, err := dc.discoverPv(diskClass)
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
			if err = dc.dm.VolumeManager.AddNewDiskToVg(pv, vg); err != nil {
				log.Errorf("add new disk failed vg: %s, disk: %s, error: %v", vg, pv, err)
			}
		}
	}

	time.Sleep(5 * time.Second)
	// 移出磁盘
	// 无法判断单独的PV属于carina管理范围，所以不支持单独对pv remove
	// 若是发生vgreduce成功，但是pvremove失败的情况，并不影响carina工作，也不影响磁盘再次使用
	actuallyVg, err = dc.dm.VolumeManager.GetCurrentVgStruct()
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
				_ = dc.dm.VolumeManager.GetLv().RemoveUnknownDevice(pv.VGName)
				continue
			}
			//同一个vg里，如果正则不匹配就将磁盘移出vg
			if !diskSelector.MatchString(pv.PVName) {
				log.Infof("try to remove pv %s from vg %s", pv.PVName, v.VGName)
				if err := dc.dm.VolumeManager.RemoveDiskInVg(pv.PVName, v.VGName); err != nil {
					log.Errorf("remove pv %s error %v", pv.PVName, err)
					continue
				}
				log.Infof("succeeded in removing pv %s from vg %s", pv.PVName, v.VGName)
			}

		}
	}

	changeAfter, err := dc.dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		log.Error("get current vg struct failed: " + err.Error())
		return
	}
	log.Debug("new vgs ", changeAfter)
	if !equality.Semantic.DeepEqual(changeBefore, changeAfter) {
		dc.dm.NoticeUpdateCapacity(deviceManager.LVMCheck, nil)
	}
}

// discoverDisk 查找是否有符合条件的块设备加入
func (dc *deviceCheck) discoverDisk(diskClass map[string]configuration.DiskSelectorItem) (map[string][]string, error) {
	blockClass := map[string][]string{}
	var name string
	// 列出所有本地磁盘
	localDisk, err := dc.dm.Partition.ListDevicesDetail("")
	if err != nil {
		log.Error("get local disk failed: " + err.Error())
		return blockClass, err
	}
	if len(localDisk) == 0 {
		log.Info("cannot find new device")
		return blockClass, nil
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
			if d.HavePartitions {
				continue
			}

			if strings.Contains(d.Name, "cache") {
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
			dused, err := dc.dm.Partition.GetDiskUsed(d.Name)
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
func (dc *deviceCheck) discoverPv(diskClass map[string]configuration.DiskSelectorItem) (map[string][]string, error) {
	resp := map[string][]string{}
	var name string
	pvList, err := dc.dm.VolumeManager.GetCurrentPvStruct()
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
				err := dc.dm.VolumeManager.GetLv().PVResize(pv.PVName)
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
			disk, err := dc.dm.Partition.ListDevicesDetailWithoutFilter(pv.PVName)
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

// NeedLeaderElection implements controller-runtime's manager.LeaderElectionRunnable.
func (dc *deviceCheck) NeedLeaderElection() bool {
	return false
}
