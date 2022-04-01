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

	"github.com/carina-io/carina/pkg/devicemanager/partition"
	"github.com/carina-io/carina/pkg/devicemanager/volume"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
	"google.golang.org/grpc/codes"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	carinav1 "github.com/carina-io/carina/api/v1"
)

// LogicVolumeReconciler reconciles a LogicVolume object
type LogicVolumeReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	nodeName  string
	volume    volume.LocalVolume
	partition partition.LocalPartition
}

// +kubebuilder:rbac:groups=carina.storage.io,resources=logicvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=carina.storage.io,resources=logicvolumes/status,verbs=get;update;patch

func NewLogicVolumeReconciler(client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder, nodeName string, volume volume.LocalVolume, partition partition.LocalPartition) *LogicVolumeReconciler {
	return &LogicVolumeReconciler{
		Client:    client,
		Scheme:    scheme,
		Recorder:  recorder,
		nodeName:  nodeName,
		volume:    volume,
		partition: partition,
	}
}

func (r *LogicVolumeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	// your logic here
	lv := new(carinav1.LogicVolume)
	if err := r.Client.Get(ctx, req.NamespacedName, lv); err != nil {
		if !apierrs.IsNotFound(err) {
			log.Error(err, "unable to fetch LogicVolume")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if lv.Spec.NodeName != r.nodeName {
		log.Info("unfiltered logic value nodeName ", lv.Spec.NodeName)
		return ctrl.Result{}, nil
	}

	if lv.ObjectMeta.DeletionTimestamp == nil {
		if !utils.ContainsString(lv.Finalizers, utils.LogicVolumeFinalizer) {
			lv2 := lv.DeepCopy()
			lv2.Finalizers = append(lv2.Finalizers, utils.LogicVolumeFinalizer)
			patch := client.MergeFrom(lv)
			if err := r.Patch(ctx, lv2, patch); err != nil {
				log.Error(err, " failed to add finalizer name ", lv.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}

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
	if !utils.ContainsString(lv.Finalizers, utils.LogicVolumeFinalizer) {
		// Our finalizer has finished, so the reconciler can do nothing.
		return ctrl.Result{}, nil
	}

	log.Info("start finalizing LogicVolume name ", lv.Name)
	err := r.removeLVIfExists(ctx, lv)
	if err != nil {
		return ctrl.Result{}, err
	}

	lv2 := lv.DeepCopy()
	lv2.Finalizers = utils.SliceRemoveString(lv2.Finalizers, utils.LogicVolumeFinalizer)
	patch := client.MergeFrom(lv)
	if err := r.Patch(ctx, lv2, patch); err != nil {
		log.Error(err, " failed to remove finalizer name ", lv.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *LogicVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&carinav1.LogicVolume{}).
		WithEventFilter(&logicVolumeFilter{r.nodeName}).
		Complete(r)
}

// operation lvm
func (r *LogicVolumeReconciler) removeLVIfExists(ctx context.Context, lv *carinav1.LogicVolume) error {
	// Finalizer's process ( RemoveLV then removeString ) is not atomic,
	// so checking existence of LV to ensure its idempotence

	switch lv.Annotations[utils.VolumeManagerType] {
	case utils.LvmVolumeType:
		err := utils.UntilMaxRetry(func() error {
			return r.volume.DeleteVolume(lv.Name, lv.Spec.DeviceGroup)
		}, 10, 12*time.Second)
		if err != nil {
			log.Error(err, " failed to remove LV name ", lv.Name, " uid ", lv.Spec.DeviceGroup)
		}
	case utils.RawVolumeType:
		err := utils.UntilMaxRetry(func() error {
			return r.partition.DeletePartition(utils.PartitionName(lv.Name), lv.Spec.DeviceGroup)
		}, 10, 12*time.Second)
		if err != nil {
			log.Error(err, " failed to remove Parttition name ", lv.Name, " uid ", lv.Spec.DeviceGroup)
		}
	default:
		log.Errorf("Delete LogicVolume: Create with no support volume type undefined")
		return fmt.Errorf("Create with no support type ")
	}

	r.volume.NoticeUpdateCapacity([]string{lv.Spec.DeviceGroup})
	log.Info("LV already removed name ", lv.Name, " uid ", lv.UID)
	return nil
}

func (r *LogicVolumeReconciler) createLV(ctx context.Context, lv *carinav1.LogicVolume) error {
	// When lv.Status.Code is not codes.OK (== 0), CreateLV has already failed.
	// LogicalVolume CRD will be deleted soon by the controller.

	if lv.Status.Code != codes.OK {
		return nil
	}
	reqBytes := lv.Spec.Size.Value()

	switch lv.Annotations[utils.VolumeManagerType] {
	case utils.LvmVolumeType:
		err := utils.UntilMaxRetry(func() error {
			return r.volume.CreateVolume(lv.Name, lv.Spec.DeviceGroup, uint64(reqBytes), 1)
		}, 5, 12*time.Second)

		if err != nil {
			lv.Status.Code = codes.Internal
			lv.Status.Message = err.Error()
			lv.Status.Status = "Failed"
			r.Recorder.Event(lv, corev1.EventTypeWarning, "CreateVolumeFailed", fmt.Sprintf("create volume failed node: %s, time: %s, error: %s", r.nodeName, time.Now().Format("2006-01-02T15:04:05.000Z"), err.Error()))
			//update status failed
			if err2 := r.Status().Update(ctx, lv); err2 != nil {
				// err2 is logged but not returned because err is more important
				log.Error(err2, " failed to update status name ", lv.Name, " uid ", lv.UID)
			}
		} else {
			lv.Status.VolumeID = "volume-" + lv.Name
			lv.Status.CurrentSize = resource.NewQuantity(reqBytes, resource.BinarySI)
			lv.Status.Code = codes.OK
			lv.Status.Message = ""
			lv.Status.Status = "Success"

			lvInfo, _ := r.volume.VolumeInfo(lv.Status.VolumeID, lv.Spec.DeviceGroup)
			if lvInfo != nil {
				lv.Status.DeviceMajor = lvInfo.LVKernelMajor
				lv.Status.DeviceMinor = lvInfo.LVKernelMinor
			}
			r.Recorder.Event(lv, corev1.EventTypeNormal, "CreateVolumeSuccess", fmt.Sprintf("create volume success node: %s, time: %s", r.nodeName, time.Now().Format("2006-01-02T15:04:05.000Z")))
		}

	case utils.RawVolumeType:
		if _, ok := lv.Annotations[utils.ExclusivityDisk]; !ok {
			log.Info("Create lv using an exclusive disk")
		}
		err := utils.UntilMaxRetry(func() error {
			log.Info("name: ", utils.PartitionName(lv.Name), " group: ", lv.Spec.DeviceGroup, " size: ", uint64(reqBytes))
			return r.partition.CreatePartition(utils.PartitionName(lv.Name), lv.Spec.DeviceGroup, uint64(reqBytes))
		}, 5, 12*time.Second)

		if err != nil {
			lv.Status.Code = codes.Internal
			lv.Status.Message = err.Error()
			lv.Status.Status = "Failed"
			r.Recorder.Event(lv, corev1.EventTypeWarning, "CreateVolumeFailed", fmt.Sprintf("create volume failed node: %s, time: %s, error: %s", r.nodeName, time.Now().Format("2006-01-02T15:04:05.000Z"), err.Error()))
			//update status failed
			if err2 := r.Status().Update(ctx, lv); err2 != nil {
				// err2 is logged but not returned because err is more important
				log.Error(err2, " failed to update status name ", lv.Name, " uid ", lv.UID)
			}
		} else {
			lv.Status.VolumeID = "volume-" + lv.Name
			lv.Status.CurrentSize = resource.NewQuantity(reqBytes, resource.BinarySI)
			lv.Status.Code = codes.OK
			lv.Status.Message = ""
			lv.Status.Status = "Success"

			diskInfo, err := r.partition.ScanDisk(lv.Spec.DeviceGroup)

			if err != nil {
				return fmt.Errorf("lv: %s,disk scan group: %s,err:%s", lv.Name, lv.Spec.DeviceGroup, err)
			}

			major, _ := strconv.ParseUint(diskInfo.UdevInfo.Properties["MAJOR"], 10, 32)
			lv.Status.DeviceMajor = uint32(major)
			minor, _ := strconv.ParseUint(diskInfo.UdevInfo.Properties["MINOR"], 10, 32)
			lv.Status.DeviceMajor = uint32(major)
			lv.Status.DeviceMinor = uint32(minor)

			r.Recorder.Event(lv, corev1.EventTypeNormal, "CreateVolumeSuccess", fmt.Sprintf("create volume success node: %s, time: %s", r.nodeName, time.Now().Format("2006-01-02T15:04:05.000Z")))
		}

	default:
		log.Errorf("Create LogicVolume: Create with no support volume type undefined")
		return fmt.Errorf("Create with no support type ")
	}
	//update status success
	if err := r.Status().Update(ctx, lv); err != nil {
		log.Error(err, " failed to update status name ", lv.Name, " uid ", lv.UID)
		return err
	}

	r.volume.NoticeUpdateCapacity([]string{lv.Spec.DeviceGroup})
	log.Info("created new LV name ", lv.Name, " uid ", lv.UID, " status.volumeID ", lv.Status.VolumeID)
	return nil
}

func (r *LogicVolumeReconciler) expandLV(ctx context.Context, lv *carinav1.LogicVolume) error {
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

	switch lv.Annotations[utils.VolumeManagerType] {
	case utils.LvmVolumeType:
		if _, ok := lv.Annotations[utils.ExclusivityDisk]; !ok {
			return fmt.Errorf("Extend lv %s doesn't using an exclusive disk", lv.Name)
		}
		err := utils.UntilMaxRetry(func() error {
			return r.volume.ResizeVolume(lv.Name, lv.Spec.DeviceGroup, uint64(reqBytes), 1)
		}, 10, 12*time.Second)
		if err != nil {
			lv.Status.Code = codes.Internal
			lv.Status.Message = err.Error()
			lv.Status.Status = "Failed"
			r.Recorder.Event(lv, corev1.EventTypeWarning, "ExpandVolumeFailed", fmt.Sprintf("expand volume failed node: %s, time: %s, error: %s", r.nodeName, time.Now().Format("2006-01-02T15:04:05.000Z"), err.Error()))
			//update status failed
			if err2 := r.Status().Update(ctx, lv); err2 != nil {
				// err2 is logged but not returned because err is more important
				log.Error(err2, " failed to update status name ", lv.Name, " uid ", lv.UID)
			}
			return err
		} else {
			lv.Status.CurrentSize = resource.NewQuantity(reqBytes, resource.BinarySI)
			lv.Status.Code = codes.OK
			lv.Status.Message = ""
			lv.Status.Status = "Success"
			r.Recorder.Event(lv, corev1.EventTypeNormal, "ExpandVolumeSuccess", fmt.Sprintf("expand volume success node: %s, time: %s", r.nodeName, time.Now().Format("2006-01-02T15:04:05.000Z")))
		}

	case utils.RawVolumeType:
		if _, ok := lv.Annotations[utils.ExclusivityDisk]; !ok {
			return fmt.Errorf("Extend lv: %s doesn't using an exclusive disk", lv.Name)
		}
		err := utils.UntilMaxRetry(func() error {
			return r.partition.UpdatePartition(utils.PartitionName(lv.Name), lv.Spec.DeviceGroup, uint64(reqBytes))
		}, 10, 12*time.Second)
		if err != nil {
			lv.Status.Code = codes.Internal
			lv.Status.Message = err.Error()
			lv.Status.Status = "Failed"
			r.Recorder.Event(lv, corev1.EventTypeWarning, "ExpandVolumeFailed", fmt.Sprintf("expand volume failed node: %s, time: %s, error: %s", r.nodeName, time.Now().Format("2006-01-02T15:04:05.000Z"), err.Error()))
			//update status failed
			if err2 := r.Status().Update(ctx, lv); err2 != nil {
				// err2 is logged but not returned because err is more important
				log.Error(err2, " failed to update status name ", lv.Name, " uid ", lv.UID)
			}
			return err
		} else {
			lv.Status.CurrentSize = resource.NewQuantity(reqBytes, resource.BinarySI)
			lv.Status.Code = codes.OK
			lv.Status.Message = ""
			lv.Status.Status = "Success"
			r.Recorder.Event(lv, corev1.EventTypeNormal, "ExpandVolumeSuccess", fmt.Sprintf("expand volume success node: %s, time: %s", r.nodeName, time.Now().Format("2006-01-02T15:04:05.000Z")))
		}

	default:
		log.Errorf("Create LogicVolume: %s with no support volume type undefined", lv.Name)
		return fmt.Errorf("lv %s Create with no support type ", lv.Name)
	}

	//update status success
	if err := r.Status().Update(ctx, lv); err != nil {
		log.Error(err, " failed to update status name ", lv.Name, " uid ", lv.UID)
		return err
	}

	r.volume.NoticeUpdateCapacity([]string{lv.Spec.DeviceGroup})
	log.Info("expanded LV name ", lv.Name, " uid ", lv.UID, " status.volumeID ", lv.Status.VolumeID,
		" original status.currentSize ", origBytes, " status.currentSize ", reqBytes)
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
	return f.filter(e.ObjectNew.(*carinav1.LogicVolume))
}

func (f logicVolumeFilter) Generic(e event.GenericEvent) bool {
	return f.filter(e.Object.(*carinav1.LogicVolume))
}
