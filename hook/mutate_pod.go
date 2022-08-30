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
package hook

import (
	"context"
	"encoding/json"
	"github.com/carina-io/carina"
	"github.com/carina-io/carina/getter"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/carina-io/carina/utils/log"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:webhookVersions=v1,path=/pod/mutate,mutating=true,failurePolicy=fail,matchPolicy=equivalent,groups="",resources=pods,verbs=create,versions=v1,path=/pod/mutate,mutating=true,sideEffects=none,name=pod-hook.carina.storage.io
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// podMutator mutates pods using PVC for Carina.
type podMutator struct {
	getter  *getter.RetryGetter
	decoder *admission.Decoder
}

// PodMutator creates a mutating webhook for Pods.
func PodMutator(mgr manager.Manager, dec *admission.Decoder) http.Handler {
	return &webhook.Admission{Handler: podMutator{getter.NewRetryGetter(mgr), dec}}
}

// Handle implements admission.Handler interface.
func (m podMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := m.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if len(pod.Spec.Containers) == 0 {
		return admission.Denied("pod has no containers")
	}

	// short cut
	if len(pod.Spec.Volumes) == 0 {
		return admission.Allowed("no volumes")
	}

	// Pods instantiated from templates may have empty name/namespace.
	// To lookup PVC in the same namespace, we set namespace obtained from req.
	if pod.Namespace == "" {
		log.Info("infer pod namespace from req namespace ", req.Namespace)
		pod.Namespace = req.Namespace
	}

	schedule, cSC, err := m.carinaSchedulePod(ctx, pod)

	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if schedule {
		pod.Spec.SchedulerName = carina.CarinaSchedule
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		if _, ok := pod.Annotations[carina.AllowPodMigrationIfNodeNotready]; !ok {
			for _, sc := range cSC {
				if _, ok = sc.Annotations[carina.AllowPodMigrationIfNodeNotready]; ok {
					pod.Annotations[carina.AllowPodMigrationIfNodeNotready] = sc.Annotations[carina.AllowPodMigrationIfNodeNotready]
					break
				}
			}
		}
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

func (m podMutator) carinaSchedulePod(ctx context.Context, pod *corev1.Pod) (bool, []storagev1.StorageClass, error) {
	cSC := []storagev1.StorageClass{}
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim == nil {
			// CSI volume type does not support direct reference from Pod
			// and may only be referenced in a Pod via a PersistentVolumeClaim
			// https://kubernetes.io/docs/concepts/storage/volumes/#csi
			continue
		}
		pvcName := vol.PersistentVolumeClaim.ClaimName
		name := types.NamespacedName{
			Namespace: pod.Namespace,
			Name:      pvcName,
		}

		var pvc corev1.PersistentVolumeClaim
		if err := m.getter.Get(ctx, name, &pvc); err != nil {
			if !apierrs.IsNotFound(err) {
				log.Error(err, "failed to get pvc pod", pod.Name, " namespace ", pod.Namespace, " pvc ", pvcName)
				return false, cSC, err
			}
			// Pods should be created even if their PVCs do not exist yet.
			continue
		}

		if pvc.Spec.StorageClassName == nil {
			// empty class name may appear when DefaultStorageClass admission plugin
			// is turned off, or there are no default StorageClass.
			// https://kubernetes.io/docs/concepts/storage/persistent-volumes/#class-1
			continue
		}
		var sc storagev1.StorageClass
		err := m.getter.Get(ctx, types.NamespacedName{Name: *pvc.Spec.StorageClassName}, &sc)
		if err != nil {
			log.Error(err, "failed to get sc ", *pvc.Spec.StorageClassName)
			continue
		}
		if sc.Provisioner != carina.CSIPluginName {
			continue
		}
		cSC = append(cSC, sc)
	}
	if len(cSC) > 0 {
		return true, cSC, nil
	}
	return false, cSC, nil
}
