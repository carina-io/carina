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

package controllers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/grpc/codes"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/carina-io/carina"
	carinav1 "github.com/carina-io/carina/api/v1"
	deviceManager "github.com/carina-io/carina/pkg/devicemanager"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
)

// LogicVolumeReconciler reconciles a LogicVolume object
type LogicVolumeReconciler struct {
	client.Client
	recorder record.EventRecorder
	dm       *deviceManager.DeviceManager
}

// +kubebuilder:rbac:groups=carina.storage.io,resources=logicvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=carina.storage.io,resources=logicvolumes/status,verbs=get;update;patch

func NewLogicVolumeReconciler(client client.Client, recorder record.EventRecorder, dm *deviceManager.DeviceManager) *LogicVolumeReconciler {
	return &LogicVolumeReconciler{
		Client:   client,
		recorder: recorder,
		dm:       dm,
	}
}

func (r *LogicVolumeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	// your logic here
	lv := new(carinav1.LogicVolume)
	if err := r.Get(ctx, req.NamespacedName, lv); err != nil {
		if !apierrs.IsNotFound(err) {
			log.Error(err, " unable to fetch LogicVolume")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if lv.ObjectMeta.DeletionTimestamp == nil {
		if lv.Status.VolumeID == "" {
			err := r.createLV(ctx, lv)
			if err != nil {
				log.Error(err, " failed to create LV name ", lv.Name)
			}
			return ctrl.Result{}, err
		}
		err := r.expandLV(ctx, lv)
		if err != nil {
			log.Error(err, " failed to expand LV name ", lv.Name)
		}
		return ctrl.Result{}, err
	}

	// finalization
	if !utils.ContainsString(lv.Finalizers, carina.LogicVolumeFinalizer) {
		// Our finalizer has finished, so the reconciler can do nothing.
		return ctrl.Result{}, nil
	}

	log.Info("Start finalizing LogicVolume name ", lv.Name)
	return ctrl.Result{}, r.removeLVIfExists(ctx, lv)
}

func (r *LogicVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&carinav1.LogicVolume{}).
		WithEventFilter(&logicVolumeFilter{r.dm.NodeName}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
		}).
		Complete(r)
}

// operation lvm
func (r *LogicVolumeReconciler) removeLVIfExists(ctx context.Context, lv *carinav1.LogicVolume) error {
	log.Info("Start to remove LV name ", lv.Name)

	// Finalizer's process ( RemoveLV then removeString ) is not atomic,
	// so checking existence of LV to ensure its idempotence
	var err error
	switch lv.Annotations[carina.VolumeManagerType] {
	case carina.LvmVolumeType:
		err = utils.UntilMaxRetry(func() error {
			return r.dm.VolumeManager.DeleteVolume(lv.Name, lv.Spec.DeviceGroup)
		}, 3, 1*time.Second)

	case carina.RawVolumeType:
		err = utils.UntilMaxRetry(func() error {
			return r.dm.Partition.DeletePartition(utils.PartitionName(lv.Name), lv.Spec.DeviceGroup)
		}, 3, 1*time.Second)
	default:
		log.Errorf("Delete LogicVolume: Create with no support volume type undefined %s", lv.Annotations[carina.VolumeManagerType])
		return nil
	}

	if err != nil {
		log.Error(err, " failed to remove LV name ", lv.Name, " uid ", lv.UID)
		return err
	}

	if err = r.syncNoticeUpdateCapacity(lv); err != nil {
		return err
	}

	lv2 := lv.DeepCopy()
	lv2.Finalizers = utils.SliceRemoveString(lv2.Finalizers, carina.LogicVolumeFinalizer)
	patch := client.MergeFrom(lv)
	if err = r.Patch(ctx, lv2, patch); err != nil {
		log.Error(err, " failed to remove finalizer name ", lv.Name)
		return err
	}
	log.Info("LV already removed name ", lv.Name, " uid ", lv.UID)
	return nil
}

func (r *LogicVolumeReconciler) createLV(ctx context.Context, lv *carinav1.LogicVolume) error {
	log.Info("Start to create LV name ", lv.Name)

	// When lv.Status.Code is not codes.OK (== 0), CreateLV has already failed.
	// LogicalVolume CRD will be deleted soon by the controller.
	if lv.Status.Code != codes.OK {
		return nil
	}
	reqBytes := lv.Spec.Size.Value()

	switch lv.Annotations[carina.VolumeManagerType] {
	case carina.LvmVolumeType:
		err := utils.UntilMaxRetry(func() error {
			return r.dm.VolumeManager.CreateVolume(lv.Name, lv.Spec.DeviceGroup, uint64(reqBytes), 1)
		}, 3, 1*time.Second)

		if err != nil {
			lv.Status.Code = codes.Internal
			if err.Error() == carina.ResourceExhausted {
				lv.Status.Code = codes.ResourceExhausted
			}
			lv.Status.Message = err.Error()
			lv.Status.Status = "Failed"
			r.recorder.Event(lv, corev1.EventTypeWarning, "CreateVolumeFailed", fmt.Sprintf("create volume failed node: %s, time: %s, error: %s", r.dm.NodeName, time.Now().Format("2006-01-02T15:04:05.000Z"), err.Error()))
		} else {
			lv.Status.VolumeID = carina.VolumePrefix + lv.Name
			lv.Status.CurrentSize = resource.NewQuantity(reqBytes, resource.BinarySI)
			lv.Status.Code = codes.OK
			lv.Status.Message = ""
			lv.Status.Status = "Success"

			lvInfo, _ := r.dm.VolumeManager.VolumeInfo(lv.Status.VolumeID, lv.Spec.DeviceGroup)
			if lvInfo != nil {
				lv.Status.DeviceMajor = lvInfo.LVKernelMajor
				lv.Status.DeviceMinor = lvInfo.LVKernelMinor
			}
			r.recorder.Event(lv, corev1.EventTypeNormal, "CreateVolumeSuccess", fmt.Sprintf("create volume success node: %s, time: %s", r.dm.NodeName, time.Now().Format("2006-01-02T15:04:05.000Z")))
		}

	case carina.RawVolumeType:
		if _, ok := lv.Annotations[carina.ExclusivityDisk]; ok {
			log.Info("Create lv using an exclusive disk")
		}
		err := utils.UntilMaxRetry(func() error {
			log.Info("name: ", utils.PartitionName(lv.Name), " group: ", lv.Spec.DeviceGroup, " size: ", uint64(reqBytes))
			return r.dm.Partition.CreatePartition(utils.PartitionName(lv.Name), lv.Spec.DeviceGroup, uint64(reqBytes))
		}, 3, 1*time.Second)

		if err != nil {
			if err.Error() == carina.ResourceExhausted {
				lv.Status.Code = codes.ResourceExhausted
			}
			lv.Status.Message = err.Error()
			lv.Status.Status = "Failed"
			r.recorder.Event(lv, corev1.EventTypeWarning, "CreateVolumeFailed", fmt.Sprintf("create volume failed node: %s, time: %s, error: %s", r.dm.NodeName, time.Now().Format("2006-01-02T15:04:05.000Z"), err.Error()))
		} else {
			lv.Status.VolumeID = carina.VolumePrefix + lv.Name
			lv.Status.CurrentSize = resource.NewQuantity(reqBytes, resource.BinarySI)
			lv.Status.Code = codes.OK
			lv.Status.Message = ""
			lv.Status.Status = "Success"

			diskInfo, err := r.dm.Partition.ScanDisk(lv.Spec.DeviceGroup)

			if err != nil {
				return fmt.Errorf("lv: %s,disk scan group: %s,err:%s", lv.Name, lv.Spec.DeviceGroup, err)
			}

			major, _ := strconv.ParseUint(diskInfo.UdevInfo.Properties["MAJOR"], 10, 32)
			lv.Status.DeviceMajor = uint32(major)
			minor, _ := strconv.ParseUint(diskInfo.UdevInfo.Properties["MINOR"], 10, 32)
			lv.Status.DeviceMajor = uint32(major)
			lv.Status.DeviceMinor = uint32(minor)
			r.recorder.Event(lv, corev1.EventTypeNormal, "CreateVolumeSuccess", fmt.Sprintf("create volume success node: %s, time: %s", r.dm.NodeName, time.Now().Format("2006-01-02T15:04:05.000Z")))
		}

	default:
		log.Errorf("Create LogicVolume: Create with no support volume type undefined %s", lv.Annotations[carina.VolumeManagerType])
		return nil
	}

	if err := r.syncNoticeUpdateCapacity(lv); err != nil {
		return err
	}

	if err := r.Status().Update(ctx, lv); err != nil {
		log.Error(err, " failed to update status name ", lv.Name, " uid ", lv.UID)
		return err
	}

	log.Info("Created new LV name ", lv.Name, " uid ", lv.UID, " status.volumeID ", lv.Status.VolumeID, " status.message ", lv.Status.Message)
	return nil
}

func (r *LogicVolumeReconciler) expandLV(ctx context.Context, lv *carinav1.LogicVolume) error {
	log.Info("Start to expand LV name ", lv.Name)

	// The reconciliation loop of LogicVolume may call expandLV before resizing is triggered.
	// So, lv.Status.CurrentSize could be nil here.
	if lv.Status.CurrentSize == nil {
		return nil
	}

	if lv.Spec.Size.Cmp(*lv.Status.CurrentSize) <= 0 {
		return nil
	}

	origBytes := (*lv.Status.CurrentSize).Value()
	reqBytes := lv.Spec.Size.Value()

	switch lv.Annotations[carina.VolumeManagerType] {
	case carina.LvmVolumeType:
		err := utils.UntilMaxRetry(func() error {
			return r.dm.VolumeManager.ResizeVolume(lv.Name, lv.Spec.DeviceGroup, uint64(reqBytes), 1)
		}, 3, 1*time.Second)
		if err != nil {
			if err.Error() == carina.ResourceExhausted {
				lv.Status.Code = codes.ResourceExhausted
			}
			lv.Status.Message = err.Error()
			lv.Status.Status = "Failed"
			r.recorder.Event(lv, corev1.EventTypeWarning, "ExpandVolumeFailed", fmt.Sprintf("expand volume failed node: %s, time: %s, error: %s", r.dm.NodeName, time.Now().Format("2006-01-02T15:04:05.000Z"), err.Error()))
		} else {
			lv.Status.CurrentSize = resource.NewQuantity(reqBytes, resource.BinarySI)
			lv.Status.Code = codes.OK
			lv.Status.Message = ""
			lv.Status.Status = "Success"
			r.recorder.Event(lv, corev1.EventTypeNormal, "ExpandVolumeSuccess", fmt.Sprintf("expand volume success node: %s, time: %s", r.dm.NodeName, time.Now().Format("2006-01-02T15:04:05.000Z")))
		}

	case carina.RawVolumeType:
		if _, ok := lv.Annotations[carina.ExclusivityDisk]; !ok {
			return fmt.Errorf("Extend lv: %s doesn't get  annotations: carina.storage.io/exclusively-raw-disk", lv.Name)
		}
		if lv.Annotations[carina.ExclusivityDisk] == "false" {
			return fmt.Errorf("Extend lv: %s doesn't using an exclusive disk", lv.Name)
		}
		err := utils.UntilMaxRetry(func() error {
			return r.dm.Partition.UpdatePartition(utils.PartitionName(lv.Name), lv.Spec.DeviceGroup, uint64(reqBytes))
		}, 3, 1*time.Second)
		if err != nil {
			if err.Error() == carina.ResourceExhausted {
				lv.Status.Code = codes.ResourceExhausted
			}
			lv.Status.Message = err.Error()
			lv.Status.Status = "Failed"
			r.recorder.Event(lv, corev1.EventTypeWarning, "ExpandVolumeFailed", fmt.Sprintf("expand volume failed node: %s, time: %s, error: %s", r.dm.NodeName, time.Now().Format("2006-01-02T15:04:05.000Z"), err.Error()))
		} else {
			lv.Status.CurrentSize = resource.NewQuantity(reqBytes, resource.BinarySI)
			lv.Status.Code = codes.OK
			lv.Status.Message = ""
			lv.Status.Status = "Success"
			r.recorder.Event(lv, corev1.EventTypeNormal, "ExpandVolumeSuccess", fmt.Sprintf("expand volume success node: %s, time: %s", r.dm.NodeName, time.Now().Format("2006-01-02T15:04:05.000Z")))
		}

	default:
		log.Errorf("Create LogicVolume: %s with no support volume type undefined %s", lv.Name, lv.Annotations[carina.VolumeManagerType])
		return nil
	}

	if err := r.syncNoticeUpdateCapacity(lv); err != nil {
		return err
	}

	if err := r.Status().Update(ctx, lv); err != nil {
		log.Error(err, " failed to update status name ", lv.Name, " uid ", lv.UID)
		return err
	}

	log.Info("Expanded LV name ", lv.Name, " uid ", lv.UID, " status.volumeID ", lv.Status.VolumeID,
		" original status.currentSize ", origBytes, " request spec.Size ", reqBytes, " status.message ", lv.Status.Message)
	return nil
}

// filter logicVolume
type logicVolumeFilter struct {
	nodeName string
}

func (f logicVolumeFilter) filter(lv *carinav1.LogicVolume) bool {
	if lv == nil {
		return false
	}
	if lv.Spec.NodeName == f.nodeName {
		return true
	}
	return false
}

func (f logicVolumeFilter) Create(e event.CreateEvent) bool {
	return f.filter(e.Object.(*carinav1.LogicVolume))
}

func (f logicVolumeFilter) Delete(e event.DeleteEvent) bool {
	return f.filter(e.Object.(*carinav1.LogicVolume))
}

func (f logicVolumeFilter) Update(e event.UpdateEvent) bool {
	newLogicVolume := e.ObjectNew.(*carinav1.LogicVolume)
	oldLogicVolume := e.ObjectOld.(*carinav1.LogicVolume)
	if newLogicVolume.ResourceVersion == oldLogicVolume.ResourceVersion {
		return false
	}
	return f.filter(newLogicVolume) || f.filter(oldLogicVolume)
}

func (f logicVolumeFilter) Generic(e event.GenericEvent) bool {
	return f.filter(e.Object.(*carinav1.LogicVolume))
}

func (r *LogicVolumeReconciler) syncNoticeUpdateCapacity(lv *carinav1.LogicVolume) error {
	done := make(chan struct{})
	r.dm.NoticeUpdateCapacity(deviceManager.LogicVolumeController, done)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Errorf("Update nsr capacity timeout(5s), LV name ", lv.Name, " uid ", lv.UID)
		return fmt.Errorf("update nsr capacity timeout(5s)")
	}
	return nil
}
