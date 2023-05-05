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

package runners

import (
	"context"
	"fmt"
	"github.com/anuvu/disko"
	"github.com/carina-io/carina"
	"github.com/carina-io/carina/getter"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sort"
	"strings"
	"time"

	"github.com/carina-io/carina/api"
	carinav1beta1 "github.com/carina-io/carina/api/v1beta1"
	deviceManager "github.com/carina-io/carina/pkg/devicemanager"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ manager.LeaderElectionRunnable = &nodeStorageResourceReconciler{}

// NodeStorageResourceReconciler reconciles a NodeStorageResource object
type nodeStorageResourceReconciler struct {
	client.Client
	updateChannel chan *deviceManager.VolumeEvent
	dm            *deviceManager.DeviceManager
	getter        *getter.RetryGetter
}

//+kubebuilder:rbac:groups=carina.storage.io,resources=nodestorageresources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=carina.storage.io,resources=nodestorageresources/status,verbs=get;update;patch

func NewNodeStorageResourceReconciler(
	mgr manager.Manager,
	dm *deviceManager.DeviceManager,
) manager.Runnable {
	return &nodeStorageResourceReconciler{
		Client:        mgr.GetClient(),
		updateChannel: make(chan *deviceManager.VolumeEvent, 500), // Buffer up to 500 statuses
		dm:            dm,
		getter:        getter.NewRetryGetter(mgr),
	}
}

func (r *nodeStorageResourceReconciler) reconcile(ve *deviceManager.VolumeEvent) {
	log.Infof("Try to update nodeStorageResource, trigger: %s, trigger at: %v", ve.Trigger, ve.TriggerAt.Format("2006-01-02 15:04:05.000000000"))

	nodeStorageResource := new(carinav1beta1.NodeStorageResource)
	ctx := context.Background()
	getErr := r.getter.Get(ctx, client.ObjectKey{Name: r.dm.NodeName}, nodeStorageResource)
	if getErr != nil {
		if apierrs.IsNotFound(getErr) {
			if err := r.createNodeStorageResource(ctx); err != nil {
				log.Error(err, " unable to create NodeStorageResource ", r.dm.NodeName)
			} else {
				r.triggerReconcile()
			}
		}
		return
	}

	nsr := nodeStorageResource.DeepCopy()
	nsr.Status.SyncTime = metav1.Time{}

	newStatus := r.generateStatus()

	if !equality.Semantic.DeepEqual(nsr.Status, newStatus) {
		log.Infof("Need to update nodeStorageResource status")
		nsr.Status = newStatus
		nsr.Status.SyncTime = metav1.Now()
		if err := r.Client.Status().Update(ctx, nsr); err != nil {
			log.Error(err, " failed to update nodeStorageResource status name ", nsr.Name)
			return
		}
	}
	// sync notice
	if ve.Done != nil {
		close(ve.Done)
	}
}

func (r *nodeStorageResourceReconciler) Start(ctx context.Context) error {
	log.Infof("Starting nodeStorageResource reconciler")
	defer log.Infof("Shutting down nodeStorageResource reconciler")
	defer close(r.updateChannel)

	// register volume update notice chan
	r.dm.RegisterNoticeChan(r.updateChannel)

	go r.triggerReconcile()

	for {
		select {
		case event := <-r.updateChannel:
			r.reconcile(event)
		case <-ctx.Done():
			_ = r.deleteNodeStorageResource(context.TODO())
			log.Info("Delete nodestorageresource...")
			return nil
		}
	}
}

func (r *nodeStorageResourceReconciler) triggerReconcile() {
	r.updateChannel <- &deviceManager.VolumeEvent{Trigger: deviceManager.Dummy, TriggerAt: time.Now()}
}

func (r *nodeStorageResourceReconciler) createNodeStorageResource(ctx context.Context) error {
	node := new(v1.Node)
	if err := r.Get(ctx, client.ObjectKey{Name: r.dm.NodeName}, node); err != nil {
		return err
	}

	NodeStorageResource := &carinav1beta1.NodeStorageResource{
		TypeMeta: metav1.TypeMeta{
			Kind:       carinav1beta1.GroupVersion.Version,
			APIVersion: carinav1beta1.GroupVersion.Group,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: r.dm.NodeName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v1.SchemeGroupVersion.WithKind("Node").Version,
					Kind:       v1.SchemeGroupVersion.WithKind("Node").Kind,
					Name:       r.dm.NodeName,
					UID:        node.UID,
				},
			},
		},
		Spec: carinav1beta1.NodeStorageResourceSpec{
			NodeName: r.dm.NodeName,
		},
		Status: carinav1beta1.NodeStorageResourceStatus{
			SyncTime: metav1.Now(),
		},
	}
	return r.Client.Create(ctx, NodeStorageResource)
}

func (r *nodeStorageResourceReconciler) deleteNodeStorageResource(ctx context.Context) error {
	NodeStorageResource := &carinav1beta1.NodeStorageResource{
		TypeMeta: metav1.TypeMeta{
			Kind:       carinav1beta1.GroupVersion.Version,
			APIVersion: carinav1beta1.GroupVersion.Group,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: r.dm.NodeName,
		},
	}
	return r.Client.Delete(ctx, NodeStorageResource)
}

func (r *nodeStorageResourceReconciler) generateStatus() carinav1beta1.NodeStorageResourceStatus {
	status := carinav1beta1.NodeStorageResourceStatus{
		Capacity:    make(map[string]resource.Quantity),
		Allocatable: make(map[string]resource.Quantity),
		SyncTime:    metav1.Time{},
	}

	r.generateLvmStatus(&status)
	r.generateDiskStatus(&status)
	r.generateRaidStatus(&status)

	return status
}

func (r *nodeStorageResourceReconciler) generateLvmStatus(status *carinav1beta1.NodeStorageResourceStatus) {
	diskSelectGroup := r.dm.GetNodeDiskSelectGroup()
	vgs, err := r.dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		log.Errorf("Get current vg struct error %s", err.Error())
		return
	}
	for _, vg := range vgs {
		if _, ok := diskSelectGroup[vg.VGName]; !ok {
			continue
		}
		status.VgGroups = append(status.VgGroups, vg)
	}

	for _, v := range status.VgGroups {
		sizeGb := v.VGSize>>30 + 1
		freeGb := uint64(0)
		if v.VGFree > carina.DefaultReservedSpace {
			freeGb = (v.VGFree-carina.DefaultReservedSpace)>>30 + 1
		}
		status.Capacity[fmt.Sprintf("%s%s", carina.DeviceCapacityKeyPrefix, v.VGName)] = *resource.NewQuantity(int64(sizeGb), resource.BinarySI)
		status.Allocatable[fmt.Sprintf("%s%s", carina.DeviceCapacityKeyPrefix, v.VGName)] = *resource.NewQuantity(int64(freeGb), resource.BinarySI)
	}

}

func (r *nodeStorageResourceReconciler) generateDiskStatus(status *carinav1beta1.NodeStorageResourceStatus) {
	diskSelectGroup := r.dm.GetNodeDiskSelectGroup()
	localDisk, err := r.dm.Partition.ListDevicesDetail("")
	if err != nil {
		log.Errorf("Scan  node disk resource error %s", err.Error())
		return
	}
	blockClass := map[string][]string{}
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

			if d.Type == "part" {
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
		return
	}

	groupDiskos := map[string][]disko.Disk{}

	for group, v := range blockClass {
		diskSet, err := r.dm.Partition.ScanAllDisk(v)
		if err != nil {
			log.Errorf("Scan node disk resource error %s", err.Error())
			return
		}

		for _, disk := range diskSet {
			tmp := api.Disk{}
			utils.Fill(disk, &tmp)
			status.Disks = append(status.Disks, tmp)
			groupDiskos[group] = append(groupDiskos[group], disk)
		}
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
			status.Capacity[fmt.Sprintf("%s%s/%s", carina.DeviceCapacityKeyPrefix, group, disk.Name)] = *resource.NewQuantity(int64(disk.Size>>30), resource.BinarySI)
			status.Allocatable[fmt.Sprintf("%s%s/%s", carina.DeviceCapacityKeyPrefix, group, disk.Name)] = *resource.NewQuantity(int64(avail>>30), resource.BinarySI)
		}
	}
}

func (r *nodeStorageResourceReconciler) generateRaidStatus(status *carinav1beta1.NodeStorageResourceStatus) {
	//TODO
}

// NeedLeaderElection implements controller-runtime's manager.LeaderElectionRunnable.
func (r *nodeStorageResourceReconciler) NeedLeaderElection() bool {
	return false
}
