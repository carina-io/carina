package framework

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func (f *Framework) GetDeployment(namespace string, name string) *appsv1.Deployment {

	pvc, err := f.KubeClientSet.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	assert.Nil(ginkgo.GinkgoT(), err, "getting deployment")
	assert.NotNil(ginkgo.GinkgoT(), pvc, "expected a deployment but none returned")
	return pvc
}

func (f *Framework) EnsurDeployment(deploy *appsv1.Deployment, selector string) *appsv1.Deployment {
	err := createDeploymentWithRetries(f.KubeClientSet, deploy.Namespace, deploy)
	assert.Nil(ginkgo.GinkgoT(), err, "creating deployment")
	f.WaitForPod(selector, 360*time.Second, false)
	deployResult := f.GetDeployment(deploy.Namespace, deploy.Name)
	return deployResult
}

func createDeploymentWithRetries(c kubernetes.Interface, namespace string, obj *appsv1.Deployment) error {
	if obj == nil {
		return fmt.Errorf("object provided to create is empty")
	}
	createFunc := func() (bool, error) {
		_, err := c.AppsV1().Deployments(namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
		if err == nil {
			return true, nil
		}
		if k8sErrors.IsAlreadyExists(err) {
			return false, err
		}
		if isRetryableAPIError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to create object with non-retriable error: %v", err)
	}

	return retryWithExponentialBackOff(createFunc)
}
