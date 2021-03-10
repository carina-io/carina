package e2e

import (
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"time"
)

var pvc1 = `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-carina-pvc1
  namespace: carina
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 7Gi
  storageClassName: csi-carina-sc1
  volumeMode: Filesystem
`

var pvc2 = `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-carina-pvc2
  namespace: carina
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 7Gi
  storageClassName: csi-carina-sc2
  volumeMode: Filesystem
`

var pvc3 = `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-carina-pvc3
  namespace: carina
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 7Gi
  storageClassName: csi-carina-sc3
  volumeMode: Filesystem
`

var pvc4 = `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-carina-pvc4
  namespace: carina
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 7Gi
  storageClassName: csi-carina-sc4
  volumeMode: Filesystem
`

func createPvc() {
	stdout, stderr, err := kubectlWithInput([]byte(pvc1), "apply", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	By("Waiting for pvc pending")
	time.Sleep(5 * time.Second)
	Eventually(func() error {
		stdout, stderr, err = kubectl("get", "pvc", "csi-carina-pvc1", "-o", "json", "-n", NameSpace)
		if err != nil {
			return fmt.Errorf("failed to create PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
		}
		var pvc corev1.PersistentVolumeClaim
		err = json.Unmarshal(stdout, &pvc)
		if err != nil {
			return fmt.Errorf("unmarshal error: stdout=%s", stdout)
		}
		if pvc.Status.Phase != corev1.ClaimPending {
			return fmt.Errorf("pvc status error: %s, %s", "csi-carina-pvc1", pvc.Status.Phase)
		}
		return nil
	}).Should(Succeed())

	stdout, stderr, err = kubectlWithInput([]byte(pvc2), "apply", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	By("Waiting for pvc pending")
	time.Sleep(5 * time.Second)
	Eventually(func() error {
		stdout, stderr, err = kubectl("get", "pvc", "csi-carina-pvc1", "-o", "json", "-n", NameSpace)
		if err != nil {
			return fmt.Errorf("failed to create PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
		}
		var pvc corev1.PersistentVolumeClaim
		err = json.Unmarshal(stdout, &pvc)
		if err != nil {
			return fmt.Errorf("unmarshal error: stdout=%s", stdout)
		}
		if pvc.Status.Phase != corev1.ClaimPending {
			return fmt.Errorf("pvc status error: %s, %s", "csi-carina-pvc1", pvc.Status.Phase)
		}
		return nil
	}).Should(Succeed())

	stdout, stderr, err = kubectlWithInput([]byte(pvc3), "apply", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	By("Waiting for pvc pending")
	time.Sleep(5 * time.Second)
	Eventually(func() error {
		stdout, stderr, err = kubectl("get", "pvc", "csi-carina-pvc1", "-o", "json", "-n", NameSpace)
		if err != nil {
			return fmt.Errorf("failed to create PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
		}
		var pvc corev1.PersistentVolumeClaim
		err = json.Unmarshal(stdout, &pvc)
		if err != nil {
			return fmt.Errorf("unmarshal error: stdout=%s", stdout)
		}
		if pvc.Status.Phase != corev1.ClaimPending {
			return fmt.Errorf("pvc status error: %s, %s", "csi-carina-pvc1", pvc.Status.Phase)
		}
		return nil
	}).Should(Succeed())

	stdout, stderr, err = kubectlWithInput([]byte(pvc4), "apply", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	By("Waiting for pvc ready")
	time.Sleep(10 * time.Second)
	Eventually(func() error {
		stdout, stderr, err = kubectl("get", "pvc", "csi-carina-pvc1", "-o", "json", "-n", NameSpace)
		if err != nil {
			return fmt.Errorf("failed to create PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
		}
		var pvc corev1.PersistentVolumeClaim
		err = json.Unmarshal(stdout, &pvc)
		if err != nil {
			return fmt.Errorf("unmarshal error: stdout=%s", stdout)
		}
		if pvc.Status.Phase != corev1.ClaimBound {
			return fmt.Errorf("pvc status error: %s, %s", "csi-carina-pvc1", pvc.Status.Phase)
		}
		return nil
	}).Should(Succeed())
}

func deletePvc() {
	stdout, stderr, err := kubectlWithInput([]byte(pvc1), "delete", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = kubectlWithInput([]byte(pvc2), "delete", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = kubectlWithInput([]byte(pvc3), "delete", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = kubectlWithInput([]byte(pvc4), "delete", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

}
