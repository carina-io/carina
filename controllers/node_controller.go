package controllers

import (
	"bocloud.com/cloudnative/carina/utils"
	"bocloud.com/cloudnative/carina/utils/log"
	"context"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"
)

// NodeReconciler reconciles a Node object
type NodeReconciler struct {
	client.Client
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="storage.k8s.io",resources=storageclasses,verbs=get;list;watch

// Reconcile finalize Node
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log.Infof("node %s is deleted.", req.Name)
	// your logic here
	utils.UntilMaxRetry(func() error {
		return r.ResourceReconcile(ctx)

	}, 6, 120*time.Second)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up Reconciler with Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	err := mgr.GetFieldIndexer().IndexField(ctx, &corev1.PersistentVolumeClaim{}, utils.AnnSelectedNode, func(o client.Object) []string {
		return []string{o.(*corev1.PersistentVolumeClaim).Annotations[utils.AnnSelectedNode]}
	})
	if err != nil {
		return err
	}
	pred := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	go utils.UntilMaxRetry(func() error {
		time.Sleep(300 * time.Second)
		return r.ResourceReconcile(ctx)

	}, 2, 120*time.Second)

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(pred).
		For(&corev1.Node{}).
		Complete(r)
}

// 过滤所有node, 当pvc所选节点不存在时，将会被删除
func (r *NodeReconciler) ResourceReconcile(ctx context.Context) error {

	log.Info("filter pvc to delete ...")
	// 获取所有Node
	nl := new(corev1.NodeList)
	err := r.List(ctx, nl)
	if err != nil {
		log.Errorf("unable to fetch node list %s", err.Error())
		return err
	}

	nodeList := []string{}
	for _, n := range nl.Items {
		if n.DeletionTimestamp != nil || n.Status.Phase == corev1.NodeTerminated {
			continue
		}
		nodeList = append(nodeList, n.Name)
	}

	scs, err := r.targetStorageClasses(ctx)
	if err != nil {
		log.Errorf("unable to fetch StorageClass %s", err.Error())
		return err
	}

	var pvcs corev1.PersistentVolumeClaimList
	err = r.List(ctx, &pvcs)
	if err != nil {
		log.Errorf("unable to fetch PersistentVolumeClaimList %s", err.Error())
		return err
	}

	for _, pvc := range pvcs.Items {
		if pvc.Spec.StorageClassName == nil {
			continue
		}
		if !scs[*pvc.Spec.StorageClassName] {
			continue
		}

		if utils.ContainsString(nodeList, pvc.Annotations[utils.AnnSelectedNode]) {
			continue
		}
		if pvc.DeletionTimestamp != nil {
			continue
		}

		log.Infof("delete pvc %s namespace %s", pvc.Name, pvc.Namespace)
		err = r.Delete(ctx, &pvc)
		if err != nil {
			log.Error(err.Error(), " unable to delete PVC name ", pvc.Name, " namespace ", pvc.Namespace)
			return err
		}
	}

	return nil
}

func (r *NodeReconciler) targetStorageClasses(ctx context.Context) (map[string]bool, error) {
	var scl storagev1.StorageClassList
	if err := r.List(ctx, &scl); err != nil {
		return nil, err
	}

	targets := make(map[string]bool)
	for _, sc := range scl.Items {
		if sc.Provisioner != utils.CSIPluginName {
			continue
		}
		targets[sc.Name] = true
	}
	return targets, nil
}
