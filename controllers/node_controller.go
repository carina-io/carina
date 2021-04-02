package controllers

import (
	"bocloud.com/cloudnative/carina/utils"
	"bocloud.com/cloudnative/carina/utils/log"
	"context"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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

	// your logic here
	node := &corev1.Node{}
	err := r.Client.Get(ctx, req.NamespacedName, node)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, err
	}

	if node.DeletionTimestamp == nil {
		return ctrl.Result{}, nil
	}

	if err := r.doFinalize(ctx, node); err != nil {
		log.Errorf("delete all pvc failed %s", err.Error())
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *NodeReconciler) doFinalize(ctx context.Context, node *corev1.Node) error {
	scs, err := r.targetStorageClasses(ctx)
	if err != nil {
		log.Errorf("unable to fetch StorageClass %s", err.Error())
		return err
	}

	var pvcs corev1.PersistentVolumeClaimList
	err = r.List(ctx, &pvcs, client.MatchingFields{utils.AnnSelectedNode: node.Name})
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

		err = r.Delete(ctx, &pvc)
		if err != nil {
			log.Error(err.Error(), " unable to delete PVC name ", pvc.Name, " namespace ", pvc.Namespace)
			return err
		}
		log.Info("deleted PVC name", pvc.Name, " namespace ", pvc.Namespace)
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

// SetupWithManager sets up Reconciler with Manager.
func (r *NodeReconciler) SetupW4ithManager(mgr ctrl.Manager) error {
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

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(pred).
		For(&corev1.Node{}).
		Complete(r)
}
