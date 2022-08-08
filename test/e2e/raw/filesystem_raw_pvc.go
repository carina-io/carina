package raw

import (
	"fmt"
	"strings"
	"time"

	"github.com/carina-io/carina/test/e2e/framework"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = framework.CrainaDescribe("Filesystem Mode RAW pvc e2e test", func() {
	f := framework.NewDefaultFramework("filesystem-raw-pvc")
	ginkgo.BeforeEach(func() {
		del := corev1.PersistentVolumeReclaimDelete
		waitForFirstConsumer := storagev1.VolumeBindingWaitForFirstConsumer
		t := true
		s := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: f.Namespace,
			},
			Provisioner:          "carina.storage.io",
			Parameters:           map[string]string{"csi.storage.k8s.io/fstype": "xfs", "carina.storage.io/disk-group-name": "carina-raw-ssd", "carina.storage.io/exclusively-raw-disk": "true"},
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
	// 1. create Filesystem RAW pvc
	ginkgo.It("should create RAW pvc", func() {
		persistentVolumeFilesystem := corev1.PersistentVolumeFilesystem
		rawPvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "filesystem-raw-pvc-create",
				Namespace: f.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				VolumeMode:       &persistentVolumeFilesystem,
				StorageClassName: &f.Namespace,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("3Gi"),
					}},
			},
		}

		pvcResult := f.EnsurePvc(rawPvc)
		framework.ExpectEqual(pvcResult.Status.Phase, corev1.ClaimPending)
	})

	// 2.RAW pvc expand
	ginkgo.It("should expand RAW pvc", func() {

		// 2.1 create a Filesystem raw pvc
		persistentVolumeFilesystem := corev1.PersistentVolumeFilesystem
		rawPvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "filesystem-raw-pvc-expand",
				Namespace: f.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				VolumeMode:       &persistentVolumeFilesystem,
				StorageClassName: &f.Namespace,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("3Gi"),
					}},
			},
		}

		pvcResult := f.EnsurePvc(rawPvc)
		framework.ExpectEqual(pvcResult.Status.Phase, corev1.ClaimPending)

		// 2.2 create a deployment and bound raw pvc
		var replicas int32 = 1
		podLabels := map[string]string{"web-server1-filesystem-raw": "web-server1-filesystem-raw"}
		rawDeploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "filesystem-raw-deploy-deployment",
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
								Name:            "web-server1",
								Image:           "docker.io/library/nginx:1.23.1",
								ImagePullPolicy: corev1.PullIfNotPresent,
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

		deployResult := f.EnsurDeployment(rawDeploy, "web-server1-filesystem-raw=web-server1-filesystem-raw")

		framework.ExpectEqual(deployResult.Status.AvailableReplicas, replicas)
		// 2.3 pvc expand

		f.PatchPvc(pvcResult.Namespace, pvcResult.Name, `{"spec": {"resources": {"requests": {"storage": "5Gi"}}}}`)
		// 2.4 check pvc expand
		pods := f.GetPods(deployResult.Namespace, "web-server1-filesystem-raw=web-server1-filesystem-raw")
		for _, pod := range pods.Items {
			gomega.Eventually(func() error {
				ginkgo.By("exec pod ...")
				stdout, stderr, err := f.Kubectl("exec", "-it", pod.Name, "-n", pod.Namespace, "--", "df", "-h")
				framework.Logf("stdout: %s, stderr:,%s, err: %v", string(stdout), string(stderr), err)
				cs := strings.Contains(string(stdout), "4.7G")
				if !cs {
					return fmt.Errorf("pvc expand in progress")
				}
				return nil
			}, 8*time.Minute, 60*time.Second).Should(gomega.BeNil())

		}
	})
	// 3.delete RAW pvc
	ginkgo.It("should delete RAW pvc", func() {
		persistentVolumeFilesystem := corev1.PersistentVolumeFilesystem
		rawPvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "filesystem-raw-pvc-delete",
				Namespace: f.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				VolumeMode:       &persistentVolumeFilesystem,
				StorageClassName: &f.Namespace,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("3Gi"),
					}},
			},
		}

		pvcResult := f.EnsurePvc(rawPvc)
		framework.ExpectEqual(pvcResult.Status.Phase, corev1.ClaimPending)
		f.DeletePvc(pvcResult.Name, pvcResult.Namespace)
	})

})
