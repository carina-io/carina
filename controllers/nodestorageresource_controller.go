/*


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

package controllers

import (
	"context"
	"fmt"
	"github.com/anuvu/disko"
	"github.com/carina-io/carina/pkg/devicemanager/volume"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/carina-io/carina/api"
	carinav1beta1 "github.com/carina-io/carina/api/v1beta1"
	deviceManager "github.com/carina-io/carina/pkg/devicemanager"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	WorkersCount = 3
)

// NodeStorageResourceReconciler reconciles a NodeStorageResource object
type NodeStorageResourceReconciler struct {
	client.Client
	nodeName      string
	updateChannel chan volume.VolumeEvent
	stopChan      <-chan struct{}
	dm            *deviceManager.DeviceManager
}

func NewNodeStorageResourceReconciler(
	client client.Client,
	nodeName string,
	stopChan <-chan struct{},
	dm *deviceManager.DeviceManager,
) *NodeStorageResourceReconciler {
	return &NodeStorageResourceReconciler{
		Client:        client,
		nodeName:      nodeName,
		updateChannel: make(chan volume.VolumeEvent, 1000), // Buffer up to 1000 statuses
		stopChan:      stopChan,
		dm:            dm,
	}
}

func (r *NodeStorageResourceReconciler) reconcile(ve volume.VolumeEvent) {
	log.Infof("Try to update nodeStorageResource, trigger: %s, trigger at: %v", ve.Trigger, ve.TriggerAt.Format("2006-01-02 15:04:05.000000000"))

	nodeStorageResource := new(carinav1beta1.NodeStorageResource)
	ctx := context.Background()
	getErr := r.Get(ctx, client.ObjectKey{Name: r.nodeName}, nodeStorageResource)
	if getErr != nil {
		if apierrs.IsNotFound(getErr) {
			if err := r.createNodeStorageResource(ctx); err != nil {
				log.Error(err, "unable to create NodeStorageResource ", r.nodeName)
			} else {
				r.triggerReconcile()
			}
		}
	}

	nsr := nodeStorageResource.DeepCopy()

	lvmNeed := r.needUpdateLvmStatus(&nsr.Status)
	diskNeed := r.needUpdateDiskStatus(&nsr.Status)
	raidNeed := r.needUpdateRaidStatus(&nsr.Status)

	if lvmNeed || diskNeed || raidNeed {
		nsr.Status.SyncTime = metav1.Now()
		if err := r.Client.Status().Update(ctx, nsr); err != nil {
			log.Error(err, " failed to update nodeStorageResource status name ", nsr.Name)
		}
	}
}

// Run begins watching and syncing.
func (r *NodeStorageResourceReconciler) Run() {
	log.Infof("Starting nodeStorageResource reconciler")
	defer log.Infof("Shutting down nodeStorageResource reconciler")

	// register volume update notice chan
	r.dm.VolumeManager.RegisterNoticeChan(r.updateChannel)

	// for startup
	r.triggerReconcile()

	for {
		select {
		case event := <-r.updateChannel:
			r.reconcile(event)
		case <-r.stopChan:
			_ = r.deleteNodeStorageResource(context.TODO())
			log.Info("Delete nodestorageresource...")
			return
		}
	}
}

func (r *NodeStorageResourceReconciler) triggerReconcile() {
	r.updateChannel <- volume.VolumeEvent{Trigger: volume.Dummy, TriggerAt: time.Now()}
}

func (r *NodeStorageResourceReconciler) createNodeStorageResource(ctx context.Context) error {
	NodeStorageResource := &carinav1beta1.NodeStorageResource{
		TypeMeta: metav1.TypeMeta{
			Kind:       carinav1beta1.GroupVersion.Version,
			APIVersion: carinav1beta1.GroupVersion.Group,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: r.nodeName,
		},
		Spec: carinav1beta1.NodeStorageResourceSpec{
			NodeName: r.nodeName,
		},
		Status: carinav1beta1.NodeStorageResourceStatus{
			SyncTime: metav1.Now(),
		},
	}
	return r.Client.Create(ctx, NodeStorageResource)
}

func (r *NodeStorageResourceReconciler) deleteNodeStorageResource(ctx context.Context) error {
	NodeStorageResource := &carinav1beta1.NodeStorageResource{
		TypeMeta: metav1.TypeMeta{
			Kind:       carinav1beta1.GroupVersion.Version,
			APIVersion: carinav1beta1.GroupVersion.Group,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: r.nodeName,
		},
	}
	return r.Client.Delete(ctx, NodeStorageResource)
}

// Determine whether the LVM volume needs to be updated
func (r *NodeStorageResourceReconciler) needUpdateLvmStatus(status *carinav1beta1.NodeStorageResourceStatus) bool {
	vgs, err := r.dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		return false
	}

	if !equality.Semantic.DeepEqual(vgs, status.VgGroups) {
		status.VgGroups = vgs
		for _, v := range vgs {
			sizeGb := v.VGSize>>30 + 1
			freeGb := uint64(0)
			if v.VGFree > utils.DefaultReservedSpace {
				freeGb = (v.VGFree-utils.DefaultReservedSpace)>>30 + 1
			}
			if status.Capacity == nil {
				status.Capacity = make(map[string]resource.Quantity)
			}
			if status.Allocatable == nil {
				status.Allocatable = make(map[string]resource.Quantity)
			}
			status.Capacity[fmt.Sprintf("%s%s", utils.DeviceCapacityKeyPrefix, v.VGName)] = *resource.NewQuantity(int64(sizeGb), resource.BinarySI)
			status.Allocatable[fmt.Sprintf("%s%s", utils.DeviceCapacityKeyPrefix, v.VGName)] = *resource.NewQuantity(int64(freeGb), resource.BinarySI)
		}
		return true
	}
	return false
}

// Determine whether the Disk needs to be updated
func (r *NodeStorageResourceReconciler) needUpdateDiskStatus(status *carinav1beta1.NodeStorageResourceStatus) bool {

	diskSelectGroup := r.dm.GetNodeDiskSelectGroup()
	localDisk, err := r.dm.Partition.ListDevicesDetail("")
	if err != nil {
		log.Errorf("Scan  node disk resource error %s", err.Error())
		return false
	}
	blockClass := map[string][]string{}
	// If the disk has been added to a DiskSelectGroup group, add it to this DiskSelectGroup group
	hasMatchedDisk := map[string]int8{}
	for _, ds := range diskSelectGroup {
		if strings.ToLower(ds.Policy) == "lvm" {
			continue
		}
		diskSelector, err := regexp.Compile(strings.Join(ds.Re, "|"))
		if err != nil {
			log.Warnf("Disk regex %s error %v ", strings.Join(ds.Re, "|"), err)
			continue
		}
		for _, d := range localDisk {

			if !diskSelector.MatchString(d.Name) {
				log.Infof("Mismatched disk:%s, regex:%s", d.Name, diskSelector.String())
				continue
			}

			name := ds.Name
			if !utils.ContainsString(blockClass[name], d.Name) {
				if hasMatchedDisk[d.Name] == 1 {
					continue
				}

				blockClass[name] = append(blockClass[name], d.Name)
				hasMatchedDisk[d.Name] = 1
			}
		}
	}

	log.Infof("Get diskSelectGroup group info %s", blockClass)

	if len(blockClass) == 0 {
		return false
	}

	disks := []api.Disk{}
	groupDiskos := map[string][]disko.Disk{}

	for group, v := range blockClass {
		diskSet, err := r.dm.Partition.ScanAllDisk(v)
		if err != nil {
			log.Errorf("Scan node disk resource error %s", err.Error())
			return false
		}

		for _, disk := range diskSet {
			tmp := api.Disk{}
			utils.Fill(disk, &tmp)
			disks = append(disks, tmp)
			groupDiskos[group] = append(groupDiskos[group], disk)
		}
	}

	if !equality.Semantic.DeepEqual(disks, status.Disks) {
		log.Info("Update disks", disks)
		status.Disks = disks
		if status.Capacity == nil {
			status.Capacity = make(map[string]resource.Quantity)
		}
		if status.Allocatable == nil {
			status.Allocatable = make(map[string]resource.Quantity)
		}

		for group, diskos := range groupDiskos {
			for _, disk := range diskos {
				var avail uint64

				fs := disk.FreeSpaces()
				if len(fs) < 1 {
					log.Info("Disk:", disk.Path, " size:", disk.Size, " avail:", avail, " free:", fs)
					continue
				}

				sort.Slice(fs, func(a, b int) bool {
					return fs[a].Size() > fs[b].Size()
				})

				//剩余容量选择可用分区剩余空间最大容量
				avail = fs[0].Size()
				log.Info("Disk:", disk.Path, " size:", disk.Size, " avail:", avail, " free:", fs)
				status.Capacity[fmt.Sprintf("%s%s/%s", utils.DeviceCapacityKeyPrefix, group, disk.Name)] = *resource.NewQuantity(int64(disk.Size>>30+1), resource.BinarySI)
				status.Allocatable[fmt.Sprintf("%s%s/%s", utils.DeviceCapacityKeyPrefix, group, disk.Name)] = *resource.NewQuantity(int64(avail>>30+1), resource.BinarySI)
			}
		}
		return true
	}

	return false
}

// Determine whether the Raid needs to be updated
func (r *NodeStorageResourceReconciler) needUpdateRaidStatus(status *carinav1beta1.NodeStorageResourceStatus) bool {
	//TODO
	// raids, err := r.raids.GetRaids()
	// if err != nil {
	// 	return false
	// }
	// if !reflect.DeepEqual(raids, status.RAIDs) {
	// 	status.RAIDs = raids

	// 	return true
	// }
	return false
}
