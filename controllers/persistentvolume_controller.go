package controllers

import (
	carinav1 "carina/api/v1"
	"carina/utils"
	"carina/utils/log"
	"context"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"
)

// PersistentVolumeClaimReconciler reconciles a PersistentVolumeClaim object
type PersistentVolumeReconciler struct {
	client.Client
	APIReader client.Reader
	Log       logr.Logger
}

// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;delete

// Reconcile finalize PVC
func (r *PersistentVolumeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("persistentvolume", req.NamespacedName)
	// your logic here
	pv := &corev1.PersistentVolume{}
	err := r.Get(ctx, req.NamespacedName, pv)
	if err != nil {
		log.Errorf("get pv info failed %s", req.Name)
		return ctrl.Result{}, err
	}

	if pv.Spec.CSI.Driver != utils.CSIPluginName {
		return ctrl.Result{}, nil
	}

	if pv.Spec.NodeAffinity == nil {
		lv := &carinav1.LogicVolume{}
		err = r.Get(ctx, client.ObjectKey{Namespace: utils.LogicVolumeNamespace, Name: pv.Name}, lv)
		if err != nil {
			log.Errorf("get lv failed %s %s", pv.Name, err.Error())
			return ctrl.Result{}, err
		}

		pv.Spec.NodeAffinity = &corev1.VolumeNodeAffinity{
			Required: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/hostname",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{lv.Spec.NodeName},
							},
						},
					},
				},
			},
		}
	}

	if err := r.Update(ctx, pv); err != nil {
		log.Errorf("failed to update pv %s", pv.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up Reconciler with Manager.
func (r *PersistentVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(pred).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewItemFastSlowRateLimiter(10*time.Second, 60*time.Second, 5),
		}).
		For(&corev1.PersistentVolume{}).
		Complete(r)
}
