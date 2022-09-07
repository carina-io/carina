package raw

import (
	"github.com/carina-io/carina/test/e2e/framework"
	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = framework.CrainaDescribe("Block Mode RAW pvc e2e test", func() {
	f := framework.NewDefaultFramework("block-raw-pvc")
	ginkgo.BeforeEach(func() {
		del := corev1.PersistentVolumeReclaimDelete
		waitForFirstConsumer := storagev1.VolumeBindingWaitForFirstConsumer
		t := true
		s := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: f.Namespace,
			},
			Provisioner:          "carina.storage.io",
			Parameters:           map[string]string{"csi.storage.k8s.io/fstype": "xfs", "carina.storage.io/disk-group-name": "carina-raw-loop", "carina.storage.io/exclusively-raw-disk": "true"},
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
	// 1. create Block RAW pvc
	ginkgo.It("should create RAW pvc", func() {
		persistentVolumeBlock := corev1.PersistentVolumeBlock
		rawPvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "block-raw-pvc-create",
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

		pvcResult := f.EnsurePvc(rawPvc)
		framework.ExpectEqual(pvcResult.Status.Phase, corev1.ClaimPending)
	})

	/*
		// 2.RAW pvc expand
		ginkgo.It("should expand RAW pvc", func() {

			// 2.1 create a raw pvc
			persistentVolumeBlock := corev1.PersistentVolumeBlock
			rawPvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "block-raw-pvc-expand",
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

			pvcResult := f.EnsurePvc(rawPvc)
			framework.ExpectEqual(pvcResult.Status.Phase, corev1.ClaimPending)

			// 2.2 create a deployment and bound raw pvc
			var replicas int32 = 1
			podLabels := map[string]string{"centos-block-raw": "centos-block-raw"}
			rawDeploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "block-raw-deploy-deployment",
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
									Name:            "centos",
									Image:           "centos:latest",
									ImagePullPolicy: corev1.PullIfNotPresent,
									SecurityContext: &corev1.SecurityContext{
										Capabilities: &corev1.Capabilities{
											Add: []corev1.Capability{"SYS_RAWIO"},
										},
									},
									Command: []string{"/bin/sleep", "infinity"},
									VolumeDevices: []corev1.VolumeDevice{
										{Name: "data", DevicePath: "/dev/xvda"},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "data",
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

			deployResult := f.EnsurDeployment(rawDeploy, "centos-block-raw=centos-block-raw")
			framework.ExpectEqual(deployResult.Status.AvailableReplicas, replicas)
			// 2.3 pvc expand
			f.PatchPvc(pvcResult.Namespace, pvcResult.Name, `{"spec": {"resources": {"requests": {"storage": "6Gi"}}}}`)
			// 2.4 check pvc expand  TODO failed to expand, https://github.com/carina-io/carina/issues/92
			pods := f.GetPods(deployResult.Namespace, "centos-block-raw=centos-block-raw")
			for _, pod := range pods.Items {
				gomega.Eventually(func() error {
					ginkgo.By("exec pod ...")
					stdout, stderr, err := f.Kubectl("exec", "-n", pod.Namespace, pod.Name, "--", "blockdev", "--getsize64", "/dev/xvda")
					if err != nil {
						framework.Logf("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
						return fmt.Errorf("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
					}

					ginkgo.By("check device capacity")
					s := strings.Replace(string(stdout), "\n", "", 1)
					blockCapacity, err := strconv.Atoi(s)
					if err != nil {
						framework.Logf(err.Error())
					}
					framework.Logf("block device capacity %d", blockCapacity>>30)

					if (6 - blockCapacity>>30) > 1 {
						return fmt.Errorf("device expand in progress")
					}

					return nil
				}, 8*time.Minute, 20*time.Second).Should(gomega.BeNil())

			}
		})

	*/

	// 3.delete RAW pvc
	ginkgo.It("should delete RAW pvc", func() {
		persistentVolumeBlock := corev1.PersistentVolumeBlock
		rawPvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "block-raw-pvc-delete",
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

		pvcResult := f.EnsurePvc(rawPvc)
		framework.ExpectEqual(pvcResult.Status.Phase, corev1.ClaimPending)
		f.DeletePvc(pvcResult.Name, pvcResult.Namespace)
	})

})
