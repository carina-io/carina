package controllers

import (
	carinav1 "bocloud.com/cloudnative/carina/api/v1"
	"bocloud.com/cloudnative/carina/utils"
	"bocloud.com/cloudnative/carina/utils/log"
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"
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

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(pred).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewItemFastSlowRateLimiter(10*time.Second, 60*time.Second, 5),
		}).
		For(&corev1.Node{}).
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

	// 获取所有Node
	nl := new(corev1.NodeList)
	err = r.List(ctx, nl)
	if err != nil {
		log.Errorf("unable to fetch node list %s", err.Error())
		return volumeObjectMap, err
	}
	nodeStatus := map[string]uint8{}
	for _, n := range nl.Items {
		if n.DeletionTimestamp != nil || n.Status.Phase == corev1.NodeTerminated {
			nodeStatus[n.Name] = 1
			continue
		}
		nodeStatus[n.Name] = 0
	}

	for _, lv := range lvList.Items {
		if _, ok := r.cacheNoDeleteLv[lv.Name]; ok {
			continue
		}
		if v, ok := nodeStatus[lv.Spec.NodeName]; ok && v == 0 {
			continue
		}

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
