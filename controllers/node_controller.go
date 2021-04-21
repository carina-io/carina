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
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=carina.storage.io,resources=logicvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=carina.storage.io,resources=logicvolumes/status,verbs=get;update;patch

// Reconcile finalize Node
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log.Infof("node %s reconcile manager...", req.Name)
	// your logic here
	utils.UntilMaxRetry(func() error {
		return r.resourceReconcile(ctx)

	}, 6, 120*time.Second)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up Reconciler with Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
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

func (r *NodeReconciler) getNeedRebuildVolume(ctx context.Context) ([]client.ObjectKey, error) {
	volumeObjectList := []client.ObjectKey{}

	log.Info("rebuild resources filter ...")
	lvList := new(carinav1.LogicVolumeList)
	err := r.List(ctx, lvList)
	if err != nil {
		return volumeObjectList, err
	}
	if len(lvList.Items) == 0 {
		return volumeObjectList, nil
	}

	// 获取所有Node
	nl := new(corev1.NodeList)
	err = r.List(ctx, nl)
	if err != nil {
		log.Errorf("unable to fetch node list %s", err.Error())
		return volumeObjectList, err
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
		if _, ok := nodeStatus[lv.Spec.NodeName]; !ok {
			volumeObjectList = append(volumeObjectList, client.ObjectKey{Namespace: lv.Spec.NameSpace, Name: lv.Spec.Pvc})
		}
		if nodeStatus[lv.Spec.NodeName] == 1 {
			volumeObjectList = append(volumeObjectList, client.ObjectKey{Namespace: lv.Spec.NameSpace, Name: lv.Spec.Pvc})
		}
	}

	return volumeObjectList, nil
}

func (r *NodeReconciler) rebuildVolume(ctx context.Context, volumeObjectList []client.ObjectKey) error {

	var pvc corev1.PersistentVolumeClaim
	for _, o := range volumeObjectList {
		err := r.Client.Get(ctx, o, &pvc)
		if err != nil {
			log.Errorf("unable to fetch PersistentVolumeClaim %s %s %s", o.Namespace, o.Name, err.Error())
		}

		newPvc := corev1.PersistentVolumeClaim{
			TypeMeta: metav1.TypeMeta{},
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

		log.Infof("rebuild pvc %s %s", o.Namespace, o.Name)
		err = r.Delete(ctx, &newPvc)
		if err != nil {
			log.Errorf("delete pvc %s %s error %s", o.Namespace, o.Name, err.Error())
		}
		data, err := newPvc.Marshal()
		log.Info(data)
		err = r.Create(ctx, &newPvc)
		if err != nil {
			data, err := newPvc.Marshal()
			if err != nil {
				log.Errorf("lose of pvc %s %s", pvc.Namespace, pvc.Name)
			}
			log.Errorf("create pvc failed %s", data)
		}
	}

	return nil
}
