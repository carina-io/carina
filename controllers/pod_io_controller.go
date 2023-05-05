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
	"github.com/carina-io/carina"
	"github.com/carina-io/carina/pkg/devicemanager/partition"
	"github.com/carina-io/carina/utils/iolimit"
	"k8s.io/kubectl/pkg/util/qos"
	"strconv"
	"sync"
	"time"

	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

const (
	// KubernetesCustomized pod annotation BlkIOThrottleReadBPS
	KubernetesCustomized = "carina.storage.io"
)

// PodReconciler reconciles a Node object
type PodIOReconciler struct {
	client.Client
	nodeName  string
	ioCache   sync.Map
	partition partition.LocalPartition
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;delete

func NewPodIOReconciler(
	client client.Client,
	nodeName string,
	partition partition.LocalPartition,
) *PodIOReconciler {
	return &PodIOReconciler{
		Client:    client,
		nodeName:  nodeName,
		ioCache:   sync.Map{},
		partition: partition,
	}
}

// Reconcile finalize Node
func (r *PodIOReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	pod := &corev1.Pod{}
	if err := r.Get(ctx, req.NamespacedName, pod); err != nil {
		log.Error(err, " unable to fetch pod")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if pod.DeletionTimestamp != nil {
		r.ioCache.Delete(pod.UID)
		return ctrl.Result{}, nil
	}

	log.Debug("Try to update pod's cgroup blkio, pod namespace: " + pod.Namespace + ", pod name:" + pod.Name)

	if err := r.handleSinglePodCGroupConfig(ctx, pod); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up Reconciler with Manager.
func (r *PodIOReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	err := mgr.GetFieldIndexer().IndexField(ctx, &corev1.Pod{}, "combinedIndex", func(object client.Object) []string {
		return []string{object.(*corev1.Pod).Spec.NodeName}
	})
	if err != nil {
		return err
	}

	err = mgr.GetFieldIndexer().IndexField(ctx, &corev1.PersistentVolume{}, "pvIndex", func(object client.Object) []string {
		pv := object.(*corev1.PersistentVolume)
		if pv == nil {
			return nil
		}
		if pv.Spec.CSI == nil {
			return nil
		}
		if pv.Spec.CSI.Driver != carina.CSIPluginName {
			return nil
		}
		if pv.Status.Phase != corev1.VolumeBound {
			return nil
		}
		if pv.Spec.ClaimRef == nil {
			return nil
		}
		return []string{fmt.Sprintf("%s-%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)}
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(podFilter{r.nodeName}).
		WithOptions(controller.Options{
			RateLimiter:             workqueue.NewItemFastSlowRateLimiter(10*time.Second, 60*time.Second, 5),
			MaxConcurrentReconciles: 5,
		}).
		For(&corev1.Pod{}).
		Complete(r)
}

func (r *PodIOReconciler) handleSinglePodCGroupConfig(ctx context.Context, pod *corev1.Pod) error {
	newPodIOLimit := r.getPodIOLimit(pod)
	oldPodIOLimit, ok := r.ioCache.Load(pod.UID)
	if ok && newPodIOLimit.Equal(oldPodIOLimit.(*iolimit.IOLimit)) {
		log.Debug("Pod's io throttles hasn't changed, ignore it, namespace: " + pod.Namespace + ", name: " + pod.Name)
		return nil
	}

	log.Infof("Need to update pod's cgroup blkio, namespace: %s, name: %s", pod.Namespace, pod.Name)
	if err := iolimit.SetIOLimit(r.getPodBlkIO(ctx, pod)); err != nil {
		return err
	}
	r.ioCache.Store(pod.UID, newPodIOLimit)
	return nil
}

func (r *PodIOReconciler) getPodBlkIO(ctx context.Context, pod *corev1.Pod) *iolimit.PodBlkIO {
	if pod == nil {
		return &iolimit.PodBlkIO{}
	}
	deviceIOSet := iolimit.DeviceIOSet{}
	iolt := r.getPodIOLimit(pod)
	for _, volume := range pod.Spec.Volumes {
		if volume.VolumeSource.PersistentVolumeClaim == nil {
			continue
		}
		pvList := &corev1.PersistentVolumeList{}
		err := r.Client.List(ctx, pvList, client.MatchingFields{"pvIndex": fmt.Sprintf("%s-%s", pod.GetNamespace(), volume.VolumeSource.PersistentVolumeClaim.ClaimName)})
		if err != nil {
			log.Errorf("Failed to get pv %s, error: %s", volume.VolumeSource.PersistentVolumeClaim.ClaimName, err.Error())
			continue
		}

		if len(pvList.Items) != 1 {
			log.Errorf("Get pv count %d not equal one", len(pvList.Items))
			continue
		}
		pvInfo := pvList.Items[0]
		deviceMajor := pvInfo.Spec.CSI.VolumeAttributes[carina.VolumeDeviceMajor]
		deviceMinor := pvInfo.Spec.CSI.VolumeAttributes[carina.VolumeDeviceMinor]
		if deviceMajor == "" || deviceMinor == "" {
			continue
		}
		deviceNo := fmt.Sprintf("%s:%s", deviceMajor, deviceMinor)
		if device, err := r.partition.GetDevice(deviceNo); device == nil || err != nil {
			log.Errorf("Can't find device %s, ignore it, err: %v", deviceNo, err)
			continue
		}

		deviceIOSet[deviceNo] = iolt
	}
	return &iolimit.PodBlkIO{
		PodUid:      string(pod.UID),
		PodQos:      qos.GetPodQOS(pod),
		DeviceIOSet: deviceIOSet,
	}
}

func (r *PodIOReconciler) getPodIOLimit(pod *corev1.Pod) *iolimit.IOLimit {
	if pod == nil {
		return &iolimit.IOLimit{}
	}
	iolt := &iolimit.IOLimit{}
	for _, throttle := range iolimit.GetSupportedIOThrottles() {
		var throttleVal uint64
		var err error
		newValue, ok := pod.Annotations[fmt.Sprintf("%s/%s", KubernetesCustomized, throttle)]
		if ok {
			if throttleVal, err = strconv.ParseUint(newValue, 10, 64); err != nil {
				log.Warnf("Failed to parse %s， will use default value 0", newValue)
			}
		}
		switch throttle {
		case iolimit.BlkIOThrottleReadBPS:
			iolt.Rbps = throttleVal
		case iolimit.BlkIOThrottleReadIOPS:
			iolt.Riops = throttleVal
		case iolimit.BlkIOThrottleWriteBPS:
			iolt.Wbps = throttleVal
		case iolimit.BlkIOThrottleWriteIOPS:
			iolt.Wiops = throttleVal
		default:
			log.Warnf("Unsupported throttle type %s", throttle)
		}
	}
	return iolt
}

// filter carina pod
type podFilter struct {
	nodeName string
}

func (p podFilter) filter(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	if pod.Spec.NodeName != p.nodeName {
		return false
	}
	if utils.IsStaticPod(pod) {
		return false
	}
	if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodSucceeded {
		return false
	}
	var ioThrottleExist bool
	for _, ioThrottle := range iolimit.GetSupportedIOThrottles() {
		if _, ok := pod.Annotations[fmt.Sprintf("%s/%s", carina.CSIPluginName, ioThrottle)]; ok {
			ioThrottleExist = true
			break
		}
	}
	if !ioThrottleExist {
		return false
	}

	return true
}

func (p podFilter) Create(e event.CreateEvent) bool {
	return p.filter(e.Object.(*corev1.Pod))
}

func (p podFilter) Delete(e event.DeleteEvent) bool {
	return p.filter(e.Object.(*corev1.Pod))
}

func (p podFilter) Update(e event.UpdateEvent) bool {
	newPod := e.ObjectNew.(*corev1.Pod)
	oldPod := e.ObjectOld.(*corev1.Pod)
	if newPod.ResourceVersion == oldPod.ResourceVersion {
		return false
	}
	return p.filter(newPod) || p.filter(oldPod)
}

func (p podFilter) Generic(e event.GenericEvent) bool {
	return false
}
