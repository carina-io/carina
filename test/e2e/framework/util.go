package framework

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// Suite represents test suite.
type Suite string

const (

	// E2E represents a test suite for e2e.
	E2E Suite = "e2e"
	// NodeE2E represents a test suite for node e2e.
	NodeE2E Suite = "node e2e"

	// Poll how often to poll for conditions
	Poll = 2 * time.Second

	// DefaultTimeout time to wait for operations to complete
	DefaultTimeout = 90 * time.Second
)

// CurrentSuite represents current test suite.
var CurrentSuite Suite

// RunID unique identifier of the e2e run
var RunID = uuid.NewUUID()

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

func log(level string, format string, args ...interface{}) {
	fmt.Fprintf(ginkgo.GinkgoWriter, nowStamp()+": "+level+": "+format+"\n", args...)
}

// Logf logs to the INFO logs.
func Logf(format string, args ...interface{}) {
	log("INFO", format, args...)
}

// Failf logs to the INFO logs and fails the test.
func Failf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log("INFO", msg)
	ginkgo.Fail(nowStamp()+": "+msg, 1)
}

// CreateKubeNamespace creates a new namespace in the cluster
func CreateKubeNamespace(baseName string, c kubernetes.Interface) (string, error) {

	return createNamespace(baseName, nil, c)
}

func createNamespace(baseName string, labels map[string]string, c kubernetes.Interface) (string, error) {
	ts := time.Now().UnixNano()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("e2e-tests-%v-%v-", baseName, ts),
			Labels:       labels,
		},
	}

	// Be robust about making the namespace creation call.
	var got *corev1.Namespace
	var err error

	err = wait.Poll(Poll, DefaultTimeout, func() (bool, error) {
		got, err = c.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		if err != nil {
			Logf("Unexpected error while creating namespace: %v", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return "", err
	}
	return got.Name, nil
}

// DeleteKubeNamespace deletes a namespace and all the objects inside
func DeleteKubeNamespace(c kubernetes.Interface, namespace string) error {
	grace := int64(0)
	pb := metav1.DeletePropagationBackground
	return c.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{
		GracePeriodSeconds: &grace,
		PropagationPolicy:  &pb,
	})
}
