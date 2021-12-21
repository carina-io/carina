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
	"time"

	carinav1 "github.com/carina-io/carina/api/v1"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type nodeStatusType string

const (
	Abnormal nodeStatusType = "abnormal"
	Normal   nodeStatusType = "normal"
)

// NodeReconciler reconciles a Node object
type NodeReconciler struct {
	client.Client
	// stop
	StopChan <-chan struct{}
	// cacheLV
	cacheNoDeleteLv map[string]uint8
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=carina.storage.io,resources=logicvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=carina.storage.io,resources=logicvolumes/status,verbs=get;update;patch

// Reconcile finalize Node
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log.Infof("node %s reconcile manager...", req.Name)
	// your logic here

	go r.resourceReconcile(ctx)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up Reconciler with Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.cacheNoDeleteLv = make(map[string]uint8)

	ctx := context.Background()
	ticker1 := time.NewTicker(600 * time.Second)
	go func(t *time.Ticker) {
		defer ticker1.Stop()
		for {
			select {
			case <-t.C:
				_ = r.resourceReconcile(ctx)
			case <-r.StopChan:
				log.Info("stop node reconcile...")
				return
			}
		}
	}(ticker1)

	pred := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		UpdateFunc:  func(event.UpdateEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	podFilter := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(pred).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewItemFastSlowRateLimiter(10*time.Second, 60*time.Second, 5),
		}).
		For(&corev1.Node{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{}).WithEventFilter(podFilter).
		Complete(r)
}

func (r *NodeReconciler) resourceReconcile(ctx context.Context) error {
	log.Infof("logic volume resource reconcile ...")
	o, err := r.getNeedRebuildVolume(ctx)
	if err != nil {
		log.Errorf("get need rebuild volume error %s", err.Error())
		return err
	}

	err = r.rebuildVolume(ctx, o)
	if err != nil {
		log.Errorf("rebuild volume error %s", err.Error())
		return err
	}
	return nil
}

func (r *NodeReconciler) getNeedRebuildVolume(ctx context.Context) (map[string]client.ObjectKey, error) {

	volumeObjectMap := map[string]client.ObjectKey{}

	lvList := new(carinav1.LogicVolumeList)
	err := r.List(ctx, lvList)
	if err != nil {
		return volumeObjectMap, err
	}
	if len(lvList.Items) == 0 {
		return volumeObjectMap, nil
	}

	nodeStatus, err := r.nodeStatusList(ctx)
	if err != nil {
		return volumeObjectMap, err
	}

	pvMap, err := r.pvMap(ctx)
	if err != nil {
		log.Errorf("unable to fetch pv list %s", err.Error())
		return volumeObjectMap, err
	}

	for _, lv := range lvList.Items {
		// bcache logicvolume not be remove
		if len(lv.OwnerReferences) > 0 {
			continue
		}
		// 删除没有对应pv的logic volume
		_, ok := pvMap[lv.Name]
		if lv.Status.Status != "" && !ok {
			if lv.Finalizers != nil && utils.ContainsString(lv.Finalizers, utils.LogicVolumeFinalizer) {
				log.Infof("remove logic volume %s", lv.Name)
				if err = r.Delete(ctx, &lv); err != nil {
					log.Errorf(" failed to remove logic volume %s", err.Error())
					return volumeObjectMap, err
				}
				lv2 := lv.DeepCopy()
				lv2.Finalizers = utils.SliceRemoveString(lv2.Finalizers, utils.LogicVolumeFinalizer)
				patch := client.MergeFrom(&lv)
				if err := r.Patch(ctx, lv2, patch); err != nil {
					log.Error(err, " failed to remove finalizer name ", lv.Name)
					return volumeObjectMap, err
				}
			}
			continue
		}
		err := r.clearPod(ctx, lv.Spec.NodeName)
		if err != nil {
			log.Errorf("unable to clear pod in not ready node:%s  err:%s", lv.Spec.NodeName, err.Error())
			return volumeObjectMap, err
		}
		// 重建逻辑

		if _, ok := r.cacheNoDeleteLv[lv.Name]; ok {
			continue
		}
		if v, ok := nodeStatus[lv.Spec.NodeName]; ok && v == "normal" {
			continue
		}

		log.Info("Namespace: ", lv.Spec.NameSpace, " Name: ", lv.Spec.Pvc, " Status: ", lv.Status.Status)
		volumeObjectMap[lv.Name] = client.ObjectKey{Namespace: lv.Spec.NameSpace, Name: lv.Spec.Pvc}
		if lv.Finalizers != nil && utils.ContainsString(lv.Finalizers, utils.LogicVolumeFinalizer) {
			lv2 := lv.DeepCopy()
			lv2.Finalizers = utils.SliceRemoveString(lv2.Finalizers, utils.LogicVolumeFinalizer)
			patch := client.MergeFrom(&lv)
			if err := r.Patch(ctx, lv2, patch); err != nil {
				log.Error(err, " failed to remove finalizer name ", lv.Name)
				return volumeObjectMap, err
			}
		}
	}
	return volumeObjectMap, nil
}

func (r *NodeReconciler) rebuildVolume(ctx context.Context, volumeObjectMap map[string]client.ObjectKey) error {

	var pvc corev1.PersistentVolumeClaim
	for key, o := range volumeObjectMap {
		err := r.Client.Get(ctx, o, &pvc)
		if err != nil {
			r.cacheNoDeleteLv[key] = 0
			log.Warnf("unable to fetch PersistentVolumeClaim %s %s %s", o.Namespace, o.Name, err.Error())
			continue
		}

		newPvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.Name,
				Namespace: o.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      pvc.Spec.AccessModes,
				Selector:         pvc.Spec.Selector,
				Resources:        pvc.Spec.Resources,
				StorageClassName: pvc.Spec.StorageClassName,
				VolumeMode:       pvc.Spec.VolumeMode,
				DataSource:       pvc.Spec.DataSource,
			},
			Status: corev1.PersistentVolumeClaimStatus{},
		}

		log.Infof("rebuild pvc namespace: %s name: %s", o.Namespace, o.Name)
		err = r.Delete(ctx, &newPvc)
		if err != nil {
			log.Errorf("delete pvc %s %s error %s", o.Namespace, o.Name, err.Error())
		}

		err = utils.UntilMaxRetry(func() error {
			return r.Create(ctx, &newPvc)
		}, 12, 10*time.Second)
		if err != nil {
			log.Warnf("create pvc failed namespace: %s, name %s, storageClass %s, volumeMode %s, resources: %d, dataSource: %s",
				newPvc.Namespace, newPvc.Name, *(newPvc.Spec.StorageClassName), *(newPvc.Spec.VolumeMode),
				newPvc.Spec.Resources.Requests.Storage().Value(),
			)
			log.Errorf("retry ten times create pvc error %s, please check", err.Error())
			return err
		}
	}
	return nil
}

func (r *NodeReconciler) pvMap(ctx context.Context) (map[string]uint8, error) {
	result := map[string]uint8{}
	pvList := new(corev1.PersistentVolumeList)
	err := r.List(ctx, pvList)
	if err != nil {
		return result, err
	}
	for _, pv := range pvList.Items {
		result[pv.Name] = 0
	}
	return result, nil
}

//when pvc is use by pod,delete pvc will not success,so you need to kill pod force,unattach volume
func (r *NodeReconciler) clearPod(ctx context.Context, nodeName string) error {
	podMap, err := r.podMap(ctx)
	if err != nil {
		log.Errorf("unable to fetch pod list %s", err.Error())
		return err
	}
	log.Infof("podmap list: %s,node: %s", podMap, nodeName)
	if len(podMap) == 0 {
		return nil
	}
	if poddelete, ok := podMap[nodeName]; ok {
		for _, p := range poddelete {
			pod := &corev1.Pod{}
			vo := &storagev1.VolumeAttachment{}
			err = r.Get(ctx, p, pod)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			err = r.killPod(ctx, pod)
			if err != nil {
				return err
			}
			err = r.Get(ctx, p, vo)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			err = r.forceDetach(ctx, vo)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

//when node id  delete and notready will be mark abnormal
func (r *NodeReconciler) nodeStatusList(ctx context.Context) (map[string]nodeStatusType, error) {
	nodeList := map[string]nodeStatusType{}
	nl := new(corev1.NodeList)
	err := r.List(ctx, nl)
	if err != nil {
		log.Errorf("unable to fetch node list %s", err.Error())
		return nodeList, err
	}

	for _, n := range nl.Items {
		//when node is delete, clear pods
		if n.DeletionTimestamp != nil || n.Status.Phase == corev1.NodeTerminated {
			nodeList[n.Name] = Abnormal
		}
		//when node is nodeready, clear pods
		for _, s := range n.Status.Conditions {
			if s.Type == corev1.NodeReady && s.Status != corev1.ConditionTrue {
				nodeList[n.Name] = Abnormal
			}
		}
		nodeList[n.Name] = Normal
	}
	return nodeList, nil

}

//get polist from  notready nodes
func (r *NodeReconciler) podMap(ctx context.Context) (map[string][]client.ObjectKey, error) {
	result := map[string][]client.ObjectKey{}
	podList := new(corev1.PodList)
	err := r.List(ctx, podList)
	if err != nil {
		return result, err
	}
	nodeStatus, err := r.nodeStatusList(ctx)
	if err != nil {
		return result, err
	}

	for _, pod := range podList.Items {
		//skip ready node
		if _, ok := nodeStatus[pod.Spec.NodeName]; !ok {
			continue
		}
		// check annotation carina.io/rebuild-node-notready: true
		if _, ok := pod.Annotations["carina.io/rebuild-node-notready"]; !ok || pod.Annotations["carina.io/rebuild-node-notready"] == "false" {
			continue
		}
		//select carina-csi build pod in notready node
		var podInAbnormalNode bool = false
		for _, vol := range pod.Spec.Volumes {
			if vol.PersistentVolumeClaim == nil {
				continue
			}
			pvcName := vol.PersistentVolumeClaim.ClaimName
			pvc := &corev1.PersistentVolumeClaim{}
			err = r.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: pvcName}, pvc)
			if err != nil {
				log.Errorf("get pvc err in namespace: %s name: %s err:%s ", pod.Namespace, pvcName, err.Error())
				continue
			}
			if pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName == utils.CSIPluginName {
				continue
			}
			if _, ok := nodeStatus[pvc.Annotations["volume.kubernetes.io/selected-node"]]; !ok {
				continue
			}
			podInAbnormalNode = true

		}
		if podInAbnormalNode {
			result[pod.Spec.NodeName] = append(result[pod.Spec.NodeName], client.ObjectKey{Namespace: pod.Namespace, Name: pod.Name})
		}

	}
	return result, nil
}

//kill pod force
func (r *NodeReconciler) killPod(ctx context.Context, pod *corev1.Pod) error {
	noGracePeriod := int64(0)
	err := r.Delete(ctx, pod, &client.DeleteOptions{GracePeriodSeconds: &noGracePeriod})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	log.Infof("delete pod namespace: %s name: %s", pod.Namespace, pod.Name)
	return nil
}

// Delete a VolumeAttachment.
func (r *NodeReconciler) forceDetach(ctx context.Context, va *storagev1.VolumeAttachment) error {
	noGracePeriod := int64(0)
	err := r.Delete(ctx, va, &client.DeleteOptions{GracePeriodSeconds: &noGracePeriod})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	log.Infof("delete VolumeAttachment name: %s", va.Name)
	return nil
}
