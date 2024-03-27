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
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/carina-io/carina/pkg/configuration"
	"github.com/carina-io/carina/pkg/devicemanager/bcache"
	"github.com/carina-io/carina/pkg/devicemanager/lvmd"
	"github.com/carina-io/carina/pkg/devicemanager/partition"
	"github.com/carina-io/carina/pkg/devicemanager/volume"
	"github.com/carina-io/carina/utils/exec"
	"github.com/carina-io/carina/utils/log"
	"github.com/carina-io/carina/utils/mutx"
)

type Trigger string

const (
	Dummy                 Trigger = "dummy"
	ConfigModify          Trigger = "configModify"
	LVMCheck              Trigger = "lvmCheck"
	CleanupOrphan         Trigger = "cleanupOrphan"
	LogicVolumeController Trigger = "logicVolumeController"
)

type VolumeEvent struct {
	Trigger   Trigger
	TriggerAt time.Time
	Done      chan struct{}
}

type DeviceManager struct {
	Cache cache.Cache
	// Volume 操作
	VolumeManager volume.LocalVolume
	//磁盘以及分区操作
	Partition     partition.LocalPartition
	NodeName      string
	noticeUpdates []chan *VolumeEvent
}

func NewDeviceManager(nodeName string, cache cache.Cache) *DeviceManager {
	executor := &exec.CommandExecutor{}
	mutex := mutx.NewGlobalLocks()

	dm := DeviceManager{
		Cache:         cache,
		VolumeManager: &volume.LocalVolumeImplement{Mutex: mutex, Lv: &lvmd.Lvm2Implement{Executor: executor}, Bcache: &bcache.BcacheImplement{Executor: executor}},
		Partition:     &partition.LocalPartitionImplement{Mutex: mutex, CacheParttionNum: make(map[string]uint), Executor: executor},
		NodeName:      nodeName,
		noticeUpdates: []chan *VolumeEvent{},
	}
	return &dm
}

func (dm *DeviceManager) GetNodeDiskSelectGroup() map[string]configuration.DiskSelectorItem {
	diskClass := map[string]configuration.DiskSelectorItem{}
	currentDiskSelector := configuration.DiskSelector()
	node := &corev1.Node{}
	err := dm.Cache.Get(context.Background(), client.ObjectKey{Name: dm.NodeName}, node)
	if err != nil {
		log.Errorf("get node %s error %s", dm.NodeName, err.Error())
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

func (dm *DeviceManager) NoticeUpdateCapacity(trigger Trigger, done chan struct{}) {
	for _, notice := range dm.noticeUpdates {
		select {
		case notice <- &VolumeEvent{Trigger: trigger, TriggerAt: time.Now(), Done: done}:
		case <-time.After(10 * time.Second):
			log.Debug("Notice channel is full, send all update channel timeout(10s).")
		}
	}
}

func (dm *DeviceManager) RegisterNoticeChan(notice chan *VolumeEvent) {
	dm.noticeUpdates = append(dm.noticeUpdates, notice)
}
