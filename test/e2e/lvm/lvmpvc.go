package lvm

import (
	"time"

	"github.com/carina-io/carina/test/e2e/framework"
	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = framework.CrainaDescribe("LVM pvc e2e test", func() {
	f := framework.NewDefaultFramework("lvm-pvc")
	ginkgo.BeforeEach(func() {
		del := corev1.PersistentVolumeReclaimDelete
		waitForFirstConsumer := storagev1.VolumeBindingWaitForFirstConsumer
		t := true
		s := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: f.Namespace,
			},
			Provisioner:          "carina.storage.io",
			Parameters:           map[string]string{"csi.storage.k8s.io/fstype": "xfs", "carina.storage.io/disk-group-name": "carina-lvm-ssd"},
			ReclaimPolicy:        &del,
			MountOptions:         []string{},
			AllowVolumeExpansion: &t,
			VolumeBindingMode:    &waitForFirstConsumer,
			AllowedTopologies:    []corev1.TopologySelectorTerm{},
		}
		f.NewStorageClass(s)
	})
	ginkgo.AfterEach(func() {
		f.DeleteStorageClass(f.Namespace)
	})
	// 1. create LVM pvc
	ginkgo.It("should create LVM pvc", func() {
		persistentVolumeBlock := corev1.PersistentVolumeBlock
		lvmPvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "lvm-block-pvc-create",
				Namespace: f.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				VolumeMode:       &persistentVolumeBlock,
				StorageClassName: &f.Namespace,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("3Gi"),
					}},
			},
		}

		pvcResult := f.EnsurePvc(lvmPvc)
		framework.ExpectEqual(pvcResult.Status.Phase, corev1.ClaimPending)
	})

	// 2.LVM pvc resizing
	ginkgo.It("should update LVM pvc", func() {

		// 2.1 create a lvm pvc
		persistentVolumeBlock := corev1.PersistentVolumeBlock
		lvmPvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "lvm-block-pvc-update",
				Namespace: f.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				VolumeMode:       &persistentVolumeBlock,
				StorageClassName: &f.Namespace,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("3Gi"),
					}},
			},
		}

		pvcResult := f.EnsurePvc(lvmPvc)
		framework.ExpectEqual(pvcResult.Status.Phase, corev1.ClaimPending)

		// 2.2 create a deployment and bound lvm pvc
		var replicas int32 = 1
		podLabels := map[string]string{"web-server1": "web-server1"}
		lvmDeploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "lvm-deploy-deployment",
				Namespace: f.Namespace,
				Labels:    podLabels,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{MatchLabels: podLabels},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: podLabels,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "web-server1",
								Image: "docker.io/library/nginx:latest",
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "mypvc1",
										MountPath: "/var/lib/www/html",
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "mypvc1",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: pvcResult.Name,
										ReadOnly:  false,
									},
								},
							},
						},
					},
				},
			},
		}

		deployResult := f.EnsurDeployment(lvmDeploy)
		f.WaitForPod("web-server1=web-server1", 60*time.Second, false)
		framework.ExpectEqual(deployResult.Status.AvailableReplicas, replicas)
		// 2.3 pvc resizing
		pvcResult.Spec.Resources.Requests = corev1.ResourceList{
			corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("6Gi"),
		}
		f.UpdatePvc(pvcResult)
		// 2.4 check pvc resized
		pods := f.GetPods(deployResult.Namespace, "web-server1=web-server1")
		for _, pod := range pods.Items {
			stdout, _, _ := f.Kubectl("exec", "-it", pod.Name, "-n", pod.Name, "--", "df", "-h")
			framework.ExpectConsistOf(string(stdout), "6G")
		}
	})
	// 3.delete LVM pvc
	ginkgo.It("should delete LVM pvc", func() {
		persistentVolumeBlock := corev1.PersistentVolumeBlock
		lvmPvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "lvm-block-pvc-delete",
				Namespace: f.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				VolumeMode:       &persistentVolumeBlock,
				StorageClassName: &f.Namespace,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("3Gi"),
					}},
			},
		}

		pvcResult := f.EnsurePvc(lvmPvc)
		framework.ExpectEqual(pvcResult.Status.Phase, corev1.ClaimPending)
		f.DeletePvc(pvcResult.Name, pvcResult.Namespace)
	})

})
