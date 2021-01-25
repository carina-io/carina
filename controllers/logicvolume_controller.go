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
	"carina/pkg/device/lvmd"
	"carina/pkg/device/types"
	"carina/utils"
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/go-logr/logr"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	carinav1 "carina/api/v1"
)

// LogicVolumeReconciler reconciles a LogicVolume object
type LogicVolumeReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	nodeName  string
	vgService lvmd.VGService
	lvService lvmd.LVService
}

// +kubebuilder:rbac:groups=carina.storage.io,resources=logicvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=carina.storage.io,resources=logicvolumes/status,verbs=get;update;patch

func NewLogicVolumeReconciler(client client.Client, log logr.Logger, nodeName string) *LogicVolumeReconciler {
	return &LogicVolumeReconciler{
		Client:   client,
		Log:      log,
		Scheme:   nil,
		nodeName: nodeName,
	}
}

func (r *LogicVolumeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("logicvolume", req.NamespacedName)

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
		log.Info("unfiltered logic value", "nodeName", lv.Spec.NodeName)
		return ctrl.Result{}, nil
	}

	if lv.ObjectMeta.DeletionTimestamp == nil {
		if !containsString(lv.Finalizers, utils.LogicVolumeFinalizer) {
			lv2 := lv.DeepCopy()
			lv2.Finalizers = append(lv2.Finalizers, utils.LogicVolumeFinalizer)
			patch := client.MergeFrom(lv)
			if err := r.Patch(ctx, lv2, patch); err != nil {
				log.Error(err, "failed to add finalizer", "name", lv.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}

		if lv.Status.VolumeID == "" {
			err := r.createLV(ctx, log, lv)
			if err != nil {
				log.Error(err, "failed to create LV", "name", lv.Name)
			}
			return ctrl.Result{}, err
		}
		err := r.expandLV(ctx, log, lv)
		if err != nil {
			log.Error(err, "failed to expand LV", "name", lv.Name)
		}
		return ctrl.Result{}, err
	}

	// finalization
	if !containsString(lv.Finalizers, utils.LogicVolumeFinalizer) {
		// Our finalizer has finished, so the reconciler can do nothing.
		return ctrl.Result{}, nil
	}

	log.Info("start finalizing LogicVolume", "name", lv.Name)
	err := r.removeLVIfExists(ctx, log, lv)
	if err != nil {
		return ctrl.Result{}, err
	}

	lv2 := lv.DeepCopy()
	lv2.Finalizers = removeString(lv2.Finalizers, utils.LogicVolumeFinalizer)
	patch := client.MergeFrom(lv)
	if err := r.Patch(ctx, lv2, patch); err != nil {
		log.Error(err, "failed to remove finalizer", "name", lv.Name)
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
func (r *LogicVolumeReconciler) removeLVIfExists(ctx context.Context, log logr.Logger, lv *carinav1.LogicVolume) error {
	// Finalizer's process ( RemoveLV then removeString ) is not atomic,
	// so checking existence of LV to ensure its idempotence
	respList, err := r.vgService.GetLVList(ctx, types.GetLVListRequest{DeviceClassName: lv.Spec.DeviceClassName})
	if err != nil {
		log.Error(err, "failed to list LV")
		return err
	}

	for _, v := range respList.Volumes {
		if v.Name != string(lv.UID) {
			continue
		}
		err := r.lvService.RemoveLV(ctx, types.RemoveLVRequest{Name: string(lv.UID), DeviceClassName: lv.Spec.DeviceClassName})
		if err != nil {
			log.Error(err, "failed to remove LV", "name", lv.Name, "uid", lv.UID)
			return err
		}
		log.Info("removed LV", "name", lv.Name, "uid", lv.UID)
		return nil
	}
	log.Info("LV already removed", "name", lv.Name, "uid", lv.UID)
	return nil
}

func (r *LogicVolumeReconciler) volumeExists(ctx context.Context, log logr.Logger, lv *carinav1.LogicVolume) (bool, error) {
	respList, err := r.vgService.GetLVList(ctx, types.GetLVListRequest{DeviceClassName: lv.Spec.DeviceClassName})
	if err != nil {
		log.Error(err, "failed to get list of LV")
		return false, err
	}

	for _, v := range respList.Volumes {
		if v.Name != string(lv.UID) {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (r *LogicVolumeReconciler) createLV(ctx context.Context, log logr.Logger, lv *carinav1.LogicVolume) error {
	// When lv.Status.Code is not codes.OK (== 0), CreateLV has already failed.
	// LogicalVolume CRD will be deleted soon by the controller.
	if lv.Status.Code != codes.OK {
		return nil
	}

	reqBytes := lv.Spec.Size.Value()

	err := func() error {
		// In case the controller crashed just after LVM LV creation, LV may already exist.
		found, err := r.volumeExists(ctx, log, lv)
		if err != nil {
			lv.Status.Code = codes.Internal
			lv.Status.Message = "failed to check volume existence"
			return err
		}
		if found {
			log.Info("set volumeID to existing LogicalVolume", "name", lv.Name, "uid", lv.UID, "status.volumeID", lv.Status.VolumeID)
			lv.Status.VolumeID = string(lv.UID)
			lv.Status.Code = codes.OK
			lv.Status.Message = ""
			return nil
		}

		resp, err := r.lvService.CreateLV(ctx, types.CreateLVRequest{
			Name:            string(lv.UID),
			SizeGB:          uint64(reqBytes >> 30),
			Tags:            nil,
			DeviceClassName: lv.Spec.DeviceClassName,
		})
		if err != nil {
			code, message := extractFromError(err)
			log.Error(err, message)
			lv.Status.Code = code
			lv.Status.Message = message
			return err
		}

		lv.Status.VolumeID = resp.Volume.Name
		lv.Status.CurrentSize = resource.NewQuantity(reqBytes, resource.BinarySI)
		lv.Status.Code = codes.OK
		lv.Status.Message = ""
		return nil
	}()

	if err != nil {
		if err2 := r.Status().Update(ctx, lv); err2 != nil {
			// err2 is logged but not returned because err is more important
			log.Error(err2, "failed to update status", "name", lv.Name, "uid", lv.UID)
		}
		return err
	}

	if err := r.Status().Update(ctx, lv); err != nil {
		log.Error(err, "failed to update status", "name", lv.Name, "uid", lv.UID)
		return err
	}

	log.Info("created new LV", "name", lv.Name, "uid", lv.UID, "status.volumeID", lv.Status.VolumeID)
	return nil
}

func (r *LogicVolumeReconciler) expandLV(ctx context.Context, log logr.Logger, lv *carinav1.LogicVolume) error {
	// lv.Status.CurrentSize is added in v0.4.0 and filled by topolvm-controller when resizing is triggered.
	// The reconciliation loop of LogicalVolume may call expandLV before resizing is triggered.
	// So, lv.Status.CurrentSize could be nil here.
	if lv.Status.CurrentSize == nil {
		return nil
	}

	if lv.Spec.Size.Cmp(*lv.Status.CurrentSize) <= 0 {
		return nil
	}

	origBytes := (*lv.Status.CurrentSize).Value()
	reqBytes := lv.Spec.Size.Value()

	err := func() error {
		err := r.lvService.ResizeLV(ctx, types.ResizeLVRequest{Name: string(lv.UID), SizeGB: uint64(reqBytes >> 30), DeviceClassName: lv.Spec.DeviceClassName})
		if err != nil {
			code, message := extractFromError(err)
			log.Error(err, message)
			lv.Status.Code = code
			lv.Status.Message = message
			return err
		}

		lv.Status.CurrentSize = resource.NewQuantity(reqBytes, resource.BinarySI)
		lv.Status.Code = codes.OK
		lv.Status.Message = ""
		return nil
	}()

	if err != nil {
		if err2 := r.Status().Update(ctx, lv); err2 != nil {
			// err2 is logged but not returned because err is more important
			log.Error(err2, "failed to update status", "name", lv.Name, "uid", lv.UID)
		}
		return err
	}

	if err := r.Status().Update(ctx, lv); err != nil {
		log.Error(err, "failed to update status", "name", lv.Name, "uid", lv.UID)
		return err
	}

	log.Info("expanded LV", "name", lv.Name, "uid", lv.UID, "status.volumeID", lv.Status.VolumeID,
		"original status.currentSize", origBytes, "status.currentSize", reqBytes)
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

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func extractFromError(err error) (codes.Code, string) {
	s, ok := status.FromError(err)
	if !ok {
		return codes.Internal, err.Error()
	}
	return s.Code(), s.Message()
}
