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
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/anuvu/disko"
	"github.com/carina-io/carina/api"
	deviceManager "github.com/carina-io/carina/pkg/devicemanager"
	"github.com/carina-io/carina/pkg/devicemanager/partition"
	"github.com/carina-io/carina/pkg/devicemanager/types"
	"github.com/carina-io/carina/pkg/devicemanager/volume"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	carinav1beta1 "github.com/carina-io/carina/api/v1beta1"
)

// NodeStorageResourceReconciler reconciles a NodeStorageResource object
type NodeStorageResourceReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	nodeName string
	// stop
	StopChan  <-chan struct{}
	volume    volume.LocalVolume
	partition partition.LocalPartition
	dm        *deviceManager.DeviceManager
}

//+kubebuilder:rbac:groups=carina.storage.io,resources=nodestorageresources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=carina.storage.io,resources=nodestorageresources/status,verbs=get;update;patch

func NewNodeStorageResourceReconciler(
	client client.Client,
	scheme *runtime.Scheme, nodeName string,
	volume volume.LocalVolume,
	stopChan <-chan struct{},
	partition partition.LocalPartition,
	dm *deviceManager.DeviceManager,
) *NodeStorageResourceReconciler {

	return &NodeStorageResourceReconciler{
		Client:    client,
		Scheme:    scheme,
		nodeName:  nodeName,
		volume:    volume,
		StopChan:  stopChan,
		partition: partition,
		dm:        dm,
	}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NodeStorageResource object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.6.4/pkg/reconcile
func (r *NodeStorageResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()

	nodeStorageResource := new(carinav1beta1.NodeStorageResource)
	err := r.Get(ctx, client.ObjectKey{Name: r.nodeName}, nodeStorageResource)
	if err != nil {
		if apierrs.IsNotFound(err) {
			err := r.createNodeStorageResource(ctx)
			if err != nil {
				log.Error(err, "unable to create NodeStorageResource ", r.nodeName)
			}
			return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, err
		}
		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, nil
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
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeStorageResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {

	ticker1 := time.NewTicker(600 * time.Second)
	go func(t *time.Ticker) {
		defer ticker1.Stop()
		for {
			select {
			case <-t.C:
				_ = r.ensureNodeStorageResourceExist()
			case <-r.StopChan:
				_ = r.deleteNodeStorageResource(context.TODO())
				log.Info("delete nodestorageresource...")
				return
			}
		}
	}(ticker1)
	go time.AfterFunc(15*time.Second, r.triggerReconcile)

	nodePredicateFn := builder.WithPredicates(
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				o := e.Object.(*carinav1beta1.NodeStorageResource)
				if o != nil {
					if o.Spec.NodeName == r.nodeName {
						return true
					}
					return false
				}
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				o := e.Object.(*carinav1beta1.NodeStorageResource)
				if o != nil {
					if o.Spec.NodeName == r.nodeName {
						return true
					}
					return false
				}
				return false
			},
			UpdateFunc:  func(event.UpdateEvent) bool { return false },
			GenericFunc: func(event.GenericEvent) bool { return false },
		})

	return ctrl.NewControllerManagedBy(mgr).
		For(&carinav1beta1.NodeStorageResource{}, nodePredicateFn).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewItemFastSlowRateLimiter(10*time.Second, 60*time.Second, 5),
		}).
		Watches(&source.Kind{Type: &corev1.PersistentVolume{}}, &handler.EnqueueRequestForObject{}, pvPredicateFn(r.nodeName)).
		Complete(r)
}

func pvPredicateFn(nodeName string) builder.Predicates {
	return builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			pv := e.Object.(*corev1.PersistentVolume)
			if pv != nil && pv.Spec.CSI != nil && pv.Spec.CSI.Driver == utils.CSIPluginName {
				if pv.Spec.CSI.VolumeAttributes[utils.VolumeDeviceNode] == nodeName {
					return true
				}
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			pv := e.ObjectNew.(*corev1.PersistentVolume)
			if pv != nil && pv.Spec.CSI != nil && pv.Spec.CSI.Driver == utils.CSIPluginName {
				if pv.Spec.CSI.VolumeAttributes[utils.VolumeDeviceNode] == nodeName {
					return true
				}
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			pv := e.Object.(*corev1.PersistentVolume)
			if pv != nil && pv.Spec.CSI != nil && pv.Spec.CSI.Driver == utils.CSIPluginName {
				if pv.Spec.CSI.VolumeAttributes[utils.VolumeDeviceNode] == nodeName {
					return true
				}
			}
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	})
}

func (r *NodeStorageResourceReconciler) triggerReconcile() {
	log.Info("trigger reconcile logic")
	ctx := context.Background()
	err := r.deleteNodeStorageResource(ctx)
	if err != nil {
		log.Warnf("delete node resource error %s", err.Error())
	}
	err = r.createNodeStorageResource(ctx)
	if err != nil {
		log.Warnf("create node resource error %s", err.Error())
	}
}

func (r *NodeStorageResourceReconciler) ensureNodeStorageResourceExist() error {
	ctx := context.Background()
	nodeStorageResource := new(carinav1beta1.NodeStorageResource)
	err := r.Get(ctx, client.ObjectKey{Name: r.nodeName}, nodeStorageResource)
	if err != nil {
		if apierrs.IsNotFound(err) {
			err := r.createNodeStorageResource(ctx)
			if err != nil {
				log.Error(err, "unable to create NodeStorageResource ", r.nodeName)
			}
		}
	}
	return nil
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
	if err := r.Client.Create(ctx, NodeStorageResource); err != nil {
		return err
	}
	return nil
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
	if err := r.Client.Delete(ctx, NodeStorageResource); err != nil {
		return err
	}
	return nil
}

// Determine whether the LVM volume needs to be updated
func (r *NodeStorageResourceReconciler) needUpdateLvmStatus(status *carinav1beta1.NodeStorageResourceStatus) bool {
	vgs, err := r.volume.GetCurrentVgStruct()
	if err != nil {
		return false
	}
	if !equality.Semantic.DeepEqual(vgs, status.VgGroups) {
		status.VgGroups = vgs
		for _, v := range vgs {
			sizeGb := v.VGSize>>30 + 1
			freeGb := uint64(0)
			if v.VGFree > utils.DefaultReservedSpace {
				freeGb = (v.VGFree - utils.DefaultReservedSpace) >> 30
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
	localDisk, err := r.dm.DiskManager.ListDevicesDetail("")
	if err != nil {
		log.Errorf("scan  node disk resource error %s", err.Error())
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
			log.Warnf("disk regex %s error %v ", strings.Join(ds.Re, "|"), err)
			continue
		}
		// 过滤出空块设备
		for _, d := range localDisk {
			if d.Type == "part" || d.ParentName != "" {
				continue
			}
			if strings.Contains(d.Name, types.KEYWORD) {
				continue
			}

			if d.Readonly || d.Size < 10<<30 || d.Filesystem != "" || d.MountPoint != "" {
				//log.Infof("mismatched disk: %s filesystem:%s mountpoint:%s readonly:%t, size:%d", d.Name, d.Filesystem, d.MountPoint, d.Readonly, d.Size)
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
				//log.Infof("mismatched disk:%s, disktype:%s", d.Name, d.Type)
				continue
			}

			if !diskSelector.MatchString(d.Name) {
				//log.Infof("mismatched disk:%s, regex:%s", d.Name, diskSelector.String())
				continue
			}

			name := ds.Name
			//log.Infof("eligible %s device %s", ds.Name, d.Name)
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
	//disksMap := make(map[string][]api.Disk)
	for _, v := range blockClass {
		diskSet, err := r.partition.ScanAllDisk(v)
		if err != nil {
			log.Errorf("scan  node disk resource error %s", err.Error())
			return false
		}

		for _, disk := range diskSet {
			tmp := api.Disk{}
			utils.Fill(disk, &tmp)
			disks = append(disks, tmp)
			//disksMap = append(disksMap, map[group]tmp{})
		}

	}

	if !equality.Semantic.DeepEqual(disks, status.Disks) {

		status.Disks = disks
		if status.Capacity == nil {
			status.Capacity = make(map[string]resource.Quantity)
		}
		if status.Allocatable == nil {
			status.Allocatable = make(map[string]resource.Quantity)
		}

		for _, disk := range disks {
			var diskGroup string
			for group, v := range blockClass {
				if utils.ContainsString(v, disk.Path) {
					diskGroup = group
				}
			}
			var avail uint64
			tmp := disko.Disk{}
			utils.Fill(disk, &tmp)
			fs := tmp.FreeSpaces()
			sort.Slice(fs, func(a, b int) bool {
				return fs[a].Size() > fs[b].Size()
			})
			//剩余容量选择可用分区剩余空间最大容量
			avail = fs[0].Size()

			status.Capacity[fmt.Sprintf("%s%s/%s", utils.DeviceCapacityKeyPrefix, diskGroup, disk.Name)] = *resource.NewQuantity(int64(disk.Size), resource.BinarySI)
			status.Allocatable[fmt.Sprintf("%s%s/%s", utils.DeviceCapacityKeyPrefix, diskGroup, disk.Name)] = *resource.NewQuantity(int64(avail), resource.BinarySI)
		}

		//fmt.Println(status)

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
