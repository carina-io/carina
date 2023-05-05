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
	"fmt"
	"github.com/anuvu/disko/linux"
	"github.com/carina-io/carina"
	carinav1 "github.com/carina-io/carina/api/v1"
	deviceManager "github.com/carina-io/carina/pkg/devicemanager"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"strconv"
	"strings"
	"time"
)

var _ manager.LeaderElectionRunnable = &troubleShoot{}

type troubleShoot struct {
	dm *deviceManager.DeviceManager
}

const logPrefix = "Clean orphan volume:"

func NewTroubleShoot(dm *deviceManager.DeviceManager) manager.Runnable {
	err := dm.Cache.IndexField(context.Background(), &carinav1.LogicVolume{}, "nodeName", func(object client.Object) []string {
		return []string{object.(*carinav1.LogicVolume).Spec.NodeName}
	})

	if err != nil {
		log.Errorf("index node with logicVolume error %s", err.Error())
	}

	return &troubleShoot{
		dm: dm,
	}
}

func (t *troubleShoot) Start(ctx context.Context) error {
	log.Info("Starting troubleshoot...")
	ticker := time.NewTicker(600 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			log.Info("Volume consistency check...")
			t.cleanupOrphanVolume()
			t.cleanupOrphanPartition()
		case <-ctx.Done():
			log.Info("Stop volume consistency check...")
			return nil
		}
	}
}

func (t *troubleShoot) cleanupOrphanVolume() {
	//t.volumeManager.HealthCheck()

	// step.1 获取所有本地volume
	log.Infof("%s get all local logic volume", logPrefix)
	volumeList, err := t.dm.VolumeManager.VolumeList("", "")
	if err != nil {
		log.Errorf("% get all local volume failed %s", logPrefix, err.Error())
	}

	// step.2 检查卷状态是否正常
	log.Infof("%s check volume status", logPrefix)
	for _, lv := range volumeList {
		if lv.LVActive != "active" {
			log.Warnf("%s logic volume %s current status %s", logPrefix, lv.LVName, lv.LVActive)
		}
	}

	// step.3 获取集群中logicVolume对象
	log.Infof("%s get all logicVolume in cluster", logPrefix)
	lvList := &carinav1.LogicVolumeList{}
	err = t.dm.Cache.List(context.Background(), lvList, client.MatchingFields{"nodeName": t.dm.NodeName})
	if err != nil {
		log.Errorf("%s list logic volume error %s", logPrefix, err.Error())
		return
	}

	// step.4 对比本地volume与logicVolume是否一致， 集群中没有的便删除本地的
	log.Infof("%s cleanup orphan volume", logPrefix)
	mapLvList := make(map[string]bool)
	for _, v := range lvList.Items {
		//skip raw logicVolume
		if v.Annotations[carina.VolumeManagerType] == carina.RawVolumeType {
			continue
		}
		mapLvList[fmt.Sprintf("%s%s", carina.VolumePrefix, v.Name)] = true
	}

	var deleteVolume bool
	for _, v := range volumeList {
		if _, ok := mapLvList[v.LVName]; !ok && strings.HasPrefix(v.LVName, carina.VolumePrefix) { // filter thin volume
			// version upgrade causes lv.status to be empty. set the remedy here
			pv := new(v1.PersistentVolume)
			err = t.dm.Cache.Get(context.Background(), types.NamespacedName{Name: v.LVName[7:]}, pv)
			if err != nil {
				log.Warnf("get persistent volume %s %s", v.LVName[7:], err.Error())
			}
			if pv != nil && len(pv.Name) != 0 {
				major, _ := strconv.ParseUint(pv.Spec.CSI.VolumeAttributes[carina.VolumeDeviceMajor], 10, 32)
				minor, _ := strconv.ParseUint(pv.Spec.CSI.VolumeAttributes[carina.VolumeDeviceMinor], 10, 32)
				deviceGroup := pv.Spec.CSI.VolumeAttributes[carina.DeviceDiskKey]
				if len(deviceGroup) == 0 {
					deviceGroup = pv.Spec.CSI.VolumeAttributes[carina.DeviceCapacityKeyPrefix+"disk-type"]
				}
				newLv := &carinav1.LogicVolume{
					TypeMeta: metav1.TypeMeta{
						Kind:       "LogicVolume",
						APIVersion: "carina.storage.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: pv.Name,
						Annotations: map[string]string{
							carina.VolumeManagerType: carina.LvmVolumeType,
						},
						Finalizers: []string{carina.LogicVolumeFinalizer},
					},
					Spec: carinav1.LogicVolumeSpec{
						NodeName:    pv.Spec.CSI.VolumeAttributes[carina.VolumeDeviceNode],
						DeviceGroup: deviceGroup,
						Size:        *pv.Spec.Capacity.Storage(),
						NameSpace:   pv.Spec.CSI.VolumeAttributes["csi.storage.k8s.io/pvc/namespace"],
						Pvc:         pv.Spec.CSI.VolumeAttributes["csi.storage.k8s.io/pvc/name"],
					},
					Status: carinav1.LogicVolumeStatus{
						VolumeID:    pv.Spec.CSI.VolumeHandle,
						Code:        0,
						Message:     "",
						CurrentSize: pv.Spec.Capacity.Storage(),
						Status:      "Success",
						DeviceMajor: uint32(major),
						DeviceMinor: uint32(minor),
					},
				}
				log.Infof("repair lv %s", newLv.Name)
				err = t.dm.Client.Create(context.Background(), newLv)
				if err != nil {
					log.Errorf("create logic volume failed %s %s", newLv.Name, err.Error())
				}
				continue
			}

			log.Infof("%s remove volume %s %s", logPrefix, v.VGName, v.LVName)
			err := t.dm.VolumeManager.DeleteVolume(v.LVName, v.VGName)
			if err != nil {
				log.Errorf("%s delete volume vg %s lv %s error %s", logPrefix, v.VGName, v.LVName, err.Error())
			} else {
				deleteVolume = true
			}
		}
	}

	if deleteVolume {
		t.dm.NoticeUpdateCapacity(deviceManager.CleanupOrphan, nil)
	}

	log.Infof("%s volume check finished.", logPrefix)
}

// 清理裸盘分区和logicVolume的对应关系
func (t *troubleShoot) cleanupOrphanPartition() {
	// step.1 获取所有本地 磁盘分区，一个lv其实就是对应一个分区
	log.Infof("%s get all local partition", "CleanupOrphanPartition")

	disklist, err := t.dm.Partition.ListDevicesDetail("")
	if err != nil {
		log.Errorf("fail get all local parttions failed %s", err.Error())
	}

	//TODU step.2 检查磁盘逻辑坏道，物理坏道隔离

	// step.3 获取集群中logicVolume对象
	log.Infof("%s get all logicVolume in cluster", logPrefix)
	lvList := &carinav1.LogicVolumeList{}
	err = t.dm.Cache.List(context.Background(), lvList, client.MatchingFields{"nodeName": t.dm.NodeName})
	if err != nil {
		log.Errorf("%s list logic volume error %s", logPrefix, err.Error())
		return
	}

	// step.4 对比本地分区与logicVolume是否一致， 集群中没有的便删除本地磁盘分区
	log.Infof("%s cleanup orphan parttions", logPrefix)
	mapLvList := map[string]bool{}
	for _, v := range lvList.Items {
		//skip lvm logicVolume
		if v.Annotations[carina.VolumeManagerType] == carina.LvmVolumeType {
			continue
		}

		mapLvList[utils.PartitionName(v.Name)] = true
	}
	log.Infof("MapLvList:%v", mapLvList)
	var deletePartion bool
	for _, d := range disklist {
		if d.Type == "part" {
			continue
		}
		disk, err := linux.System().ScanDisk(d.Name)
		if err != nil {
			log.Errorf("%s get disk info error %s", logPrefix, err.Error())
			return
		}
		if len(disk.Partitions) < 1 {
			continue
		}
		for _, p := range disk.Partitions {
			if !strings.HasPrefix(p.Name, carina.CarinaPrefix) {
				log.Infof("Skip parttions %s", p.Name)
				continue
			}
			log.Infof("Check parttions %s %d %d", p.Name, p.Start, p.Last)
			if _, ok := mapLvList[p.Name]; !ok {
				log.Warnf("Remove parttions %s %d %d", p.Name, p.Start, p.Last)
				if err := t.dm.Partition.DeletePartitionByPartNumber(disk, p.Number); err != nil {
					log.Errorf("Delete parttions in disk name: %s  number: %d error: %s", disk.Name, p.Number, err.Error())
				} else {
					deletePartion = true
				}
			}
		}
	}
	if deletePartion {
		t.dm.NoticeUpdateCapacity(deviceManager.CleanupOrphan, nil)
	}
	log.Infof("%s volume check finished.", logPrefix)
}

func (t *troubleShoot) NeedLeaderElection() bool {
	return false
}
