package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	storagev1 "k8s.io/api/storage/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func (f *Framework) NewStorageClass(s *storagev1.StorageClass) {

	err := createStorageClassWithRetries(f.KubeClientSet, s)
	assert.Nil(ginkgo.GinkgoT(), err, "creating storageClass")

	d, err := f.KubeClientSet.StorageV1().StorageClasses().Get(context.TODO(), s.Name, metav1.GetOptions{})
	assert.Nil(ginkgo.GinkgoT(), err, "getting deployment")
	assert.NotNil(ginkgo.GinkgoT(), d, "expected a deployment but none returned")

}

func createStorageClassWithRetries(c kubernetes.Interface, obj *storagev1.StorageClass) error {
	if obj == nil {
		return fmt.Errorf("object provided to create is empty")
	}
	createFunc := func() (bool, error) {
		_, err := c.StorageV1().StorageClasses().Create(context.TODO(), obj, metav1.CreateOptions{})
		if err == nil {
			return true, nil
		}
		if k8sErrors.IsAlreadyExists(err) {
			return true, nil
		}
		if isRetryableAPIError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to create object with non-retriable error: %v", err)
	}

	return retryWithExponentialBackOff(createFunc)
}

func (f *Framework) DeleteStorageClass(storageClassName string) {
	err := deleteStorageClassWithRetries(f.KubeClientSet, storageClassName)
	assert.Nil(ginkgo.GinkgoT(), err, "deleting storageClass")
}

func deleteStorageClassWithRetries(c kubernetes.Interface, storageClassName string) error {

	deleteFunc := func() (bool, error) {
		err := c.StorageV1().StorageClasses().Delete(context.TODO(), storageClassName, metav1.DeleteOptions{})
		if err == nil {
			return true, nil
		}

		if isRetryableAPIError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to delete object with non-retriable error: %v", err)
	}

	return retryWithExponentialBackOff(deleteFunc)
}
