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
	"github.com/carina-io/carina/pkg/devicemanager/volume"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"

	carinav1beta1 "github.com/carina-io/carina/api/v1beta1"
)

// NodeStorageResourceReconciler reconciles a NodeStorageResource object
type NodeStorageResourceReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	nodeName string
	volume   volume.LocalVolume
}

//+kubebuilder:rbac:groups=carina.storage.io,resources=nodestorageresources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=carina.storage.io,resources=nodestorageresources/status,verbs=get;update;patch

func NewNodeStorageResourceReconciler(client client.Client, scheme *runtime.Scheme, nodeName string, volume volume.LocalVolume) *LogicVolumeReconciler {
	return &LogicVolumeReconciler{
		Client:   client,
		Scheme:   scheme,
		nodeName: nodeName,
		volume:   volume,
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
	_ = r.Log.WithValues("nodestorageresource", req.NamespacedName)

	nodeStorageResource := new(carinav1beta1.NodeStorageResource)
	err := r.Get(ctx, client.ObjectKey{Name: r.nodeName}, nodeStorageResource)
	if err != nil {
		if apierrs.IsNotFound(err) {
			err := r.createNodeStorageResource(ctx)
			if err != nil {
				log.Error(err, "unable to create NodeDevice ", r.nodeName)
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
		nsr.Status.SyncTime = time.Now()
		if err := r.Client.Status().Update(ctx, nsr); err != nil {
			log.Error(err, " failed to update nodeStorageResource status name ", nsr.Name)
		}
	}
	return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeStorageResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&carinav1beta1.NodeStorageResource{}).
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
			if pv != nil && pv.Spec.StorageClassName == utils.CSIPluginName {
				if pv.Spec.CSI.VolumeAttributes[utils.VolumeDeviceNode] == nodeName {
					return true
				}
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			pv := e.ObjectNew.(*corev1.PersistentVolume)
			if pv != nil && pv.Spec.StorageClassName == utils.CSIPluginName {
				if pv.Spec.CSI.VolumeAttributes[utils.VolumeDeviceNode] == nodeName {
					return true
				}
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			pv := e.Object.(*corev1.PersistentVolume)
			if pv != nil && pv.Spec.StorageClassName == utils.CSIPluginName {
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
			SyncTime: time.Now(),
		},
	}
	if err := r.Client.Create(ctx, NodeStorageResource); err != nil {
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
	if !reflect.DeepEqual(vgs, status.VgGroups) {
		status.VgGroups = vgs
		for _, v := range vgs {
			sizeGb := v.VGSize>>30 + 1
			freeGb := uint64(0)
			if v.VGFree > utils.DefaultReservedSpace {
				freeGb = (v.VGFree - utils.DefaultReservedSpace) >> 30
			}
			status.Capacity[v.VGName] = *resource.NewQuantity(int64(sizeGb), resource.BinarySI)
			status.Allocatable[v.VGName] = *resource.NewQuantity(int64(freeGb), resource.BinarySI)

		}
		return true
	}
	return false
}

// Determine whether the Disk needs to be updated
func (r *NodeStorageResourceReconciler) needUpdateDiskStatus(status *carinav1beta1.NodeStorageResourceStatus) bool {

	//disks := r.volume.getDisk()
	//if err != nil {
	//	return false
	//}
	//if !reflect.DeepEqual(disks, status.Disks) {
	//	status.Disks = disks
	//
	//	return true
	//}
	return false
}

// Determine whether the Raid needs to be updated
func (r *NodeStorageResourceReconciler) needUpdateRaidStatus(status *carinav1beta1.NodeStorageResourceStatus) bool {
	// TODO
	//raids, err := r.raids.GetRaids()
	//if err != nil {
	//	return false
	//}
	//if !reflect.DeepEqual(raids, status.RAIDs) {
	//	status.RAIDs = raids
	//
	//	return true
	//}
	return false
}
