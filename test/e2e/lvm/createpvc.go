package lvm

import (
	"github.com/carina-io/carina/test/e2e/framework"
	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = framework.CrainaDescribe("create LVM pvc", func() {
	f := framework.NewDefaultFramework("lvm-pvc")
	storageClassName := "csi-carina-lvm"
	ginkgo.BeforeEach(func() {
		del := corev1.PersistentVolumeReclaimDelete
		waitForFirstConsumer := storagev1.VolumeBindingWaitForFirstConsumer
		t := true
		s := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: storageClassName,
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

	// 1. create LVM pvc
	ginkgo.It("should create LVM pvc", func() {
		persistentVolumeBlock := corev1.PersistentVolumeBlock
		lvmPvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "lvm-block-pvc",
				Namespace: f.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				VolumeMode:       &persistentVolumeBlock,
				StorageClassName: &storageClassName,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("13Gi"),
					}},
			},
		}

		pvcResult := f.EnsurePvc(lvmPvc)
		framework.ExpectEqual(pvcResult.Status.Phase, corev1.ClaimPending)
	})

	// TODO 2.LVM pvc resizing

	// TODO 3.delete LVM pvc
	ginkgo.AfterEach(func() {
		f.DeleteStorageClass(storageClassName)
	})
})
