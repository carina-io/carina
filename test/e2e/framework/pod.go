package framework

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func (f *Framework) GetPods(ns, labelSelector string) (pods *v1.PodList) {
	pods, err := f.KubeClientSet.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	assert.Nil(ginkgo.GinkgoT(), err, "getting pods")
	assert.NotNil(ginkgo.GinkgoT(), pods, "expected  pods but none returned")
	return pods
}

func (f *Framework) DeletePod(ns, name string) error {
	err := f.KubeClientSet.CoreV1().Pods(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
	assert.Nil(ginkgo.GinkgoT(), err, "deleting pod")
	return err
}

// WaitForPod waits for a specific Pod to be ready, using a label selector
func (f *Framework) WaitForPod(selector string, timeout time.Duration, shouldFail bool) {
	err := waitForPodsReady(f.KubeClientSet, timeout, 1, f.Namespace, metav1.ListOptions{
		LabelSelector: selector,
	})

	if shouldFail {
		assert.NotNil(ginkgo.GinkgoT(), err, "waiting for pods to be ready")
	} else {
		assert.Nil(ginkgo.GinkgoT(), err, "waiting for pods to be ready")
	}
}

// waitForPodsReady waits for a given amount of time until a group of Pods is running in the given namespace.
func waitForPodsReady(kubeClientSet kubernetes.Interface, timeout time.Duration, expectedReplicas int, namespace string, opts metav1.ListOptions) error {
	return wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		pl, err := kubeClientSet.CoreV1().Pods(namespace).List(context.TODO(), opts)
		if err != nil {
			return false, nil
		}

		r := 0
		for _, p := range pl.Items {
			if isRunning, _ := podRunningReady(&p); isRunning {
				r++
			}
		}

		if r == expectedReplicas {
			return true, nil
		}

		return false, nil
	})
}

// podRunningReady checks whether pod p's phase is running and it has a ready
// condition of status true.
func podRunningReady(p *v1.Pod) (bool, error) {
	// Check the phase is running.
	if p.Status.Phase != v1.PodRunning {
		return false, fmt.Errorf("want pod '%s' on '%s' to be '%v' but was '%v'",
			p.ObjectMeta.Name, p.Spec.NodeName, v1.PodRunning, p.Status.Phase)
	}
	// Check the ready condition is true.

	if !isPodReady(p) {
		return false, fmt.Errorf("pod '%s' on '%s' didn't have condition {%v %v}; conditions: %v",
			p.ObjectMeta.Name, p.Spec.NodeName, v1.PodReady, v1.ConditionTrue, p.Status.Conditions)
	}
	return true, nil
}

func isPodReady(p *v1.Pod) bool {
	for _, condition := range p.Status.Conditions {
		if condition.Type != v1.ContainersReady {
			continue
		}

		return condition.Status == v1.ConditionTrue
	}

	return false
}
