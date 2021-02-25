package controllers

import (
	"carina/pkg/configruation"
	"carina/utils"
	"carina/utils/log"
	"context"
	"encoding/json"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"
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
		return ctrl.Result{}, nil
	}

	if pv.Spec.CSI.Driver != utils.CSIPluginName {
		return ctrl.Result{}, nil
	}

	err = r.updateNodeConfigMap(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up Reconciler with Manager.
func (r *PersistentVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {

	ticker1 := time.NewTicker(60 * time.Second)
	go func(t *time.Ticker) {
		defer ticker1.Stop()
		after := time.After(200 * time.Second)
		for {
			select {
			case <-t.C:
				err := r.updateNodeConfigMap(context.Background())
				if err != nil {
					log.Errorf("update node storage config map failed %s", err.Error())
				}
			case <-after:
				log.Info("stop node storage config map update...")
				return
			}
		}
	}(ticker1)

	pred := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		UpdateFunc:  func(event.UpdateEvent) bool { return true },
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

func (r *PersistentVolumeReconciler) updateNodeConfigMap(ctx context.Context) error {
	nl := new(corev1.NodeList)
	err := r.List(ctx, nl)
	if err != nil {
		return err
	}

	nodeDevice := []map[string]string{}
	for _, node := range nl.Items {
		tmp := map[string]string{}
		for key, v := range node.Status.Capacity {
			if strings.HasPrefix(string(key), utils.DeviceCapacityKeyPrefix) {
				tmp["capacity."+string(key)] = v.String()
			}
		}
		for key, v := range node.Status.Allocatable {
			if strings.HasPrefix(string(key), utils.DeviceCapacityKeyPrefix) {
				tmp["allocatable."+string(key)] = v.String()
			}
		}
		if len(tmp) > 0 {
			tmp["nodeName"] = node.Name
			nodeDevice = append(nodeDevice, tmp)
		}
	}
	byteJson, err := json.Marshal(nodeDevice)
	if err != nil {
		log.Errorf("carina-node-storage json marshal failed %s", err.Error())
		return err
	}

	cm := &corev1.ConfigMap{}
	err = r.Get(ctx, client.ObjectKey{Namespace: configruation.RuntimeNamespace(), Name: "carina-node-storage"}, cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			c := corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "carina-node-storage",
					Namespace: configruation.RuntimeNamespace(),
				},
				Data: map[string]string{"node": string(byteJson)},
			}
			err = r.Create(ctx, &c)
			if err != nil {
				log.Errorf("update config map carina-vg failed %s", err.Error())
				return err
			}
			return nil
		}
		return err
	}

	cm.Data = map[string]string{"node": string(byteJson)}
	err = r.Update(ctx, cm)
	if err != nil {
		log.Errorf("update config map carina-vg failed %s", err.Error())
		return err
	}
	return nil
}
