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

package e2e

import (
	"encoding/json"
	"fmt"

	"github.com/carina-io/carina/utils/log"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	xfsPvcName     = "csi-carina-pvc1"
	ext4PvcName    = "csi-carina-pvc3"
	baseCapacity   = 7
	expandCapacity = 14
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

func testCreatePvc() {
	It("create pvc with xfs", func() {
		pvcName := "csi-carina-pvc1"
		stdout, stderr, err := kubectlWithInput([]byte(pvc1), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		By("Waiting for pvc pending")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", pvcName, "-o", "json", "-n", NameSpace)
			if err != nil {
				return fmt.Errorf("failed to create PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(stdout, &pvc)
			if err != nil {
				return fmt.Errorf("unmarshal error: stdout=%s", stdout)
			}
			if pvc.Status.Phase != corev1.ClaimPending {
				return fmt.Errorf("pvc status error: %s, %s", pvcName, pvc.Status.Phase)
			}
			return nil
		}).Should(Succeed())
	})

	It("create pvc without disk group", func() {
		pvcName := "csi-carina-pvc2"
		stdout, stderr, err := kubectlWithInput([]byte(pvc2), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		By("Waiting for pvc pending")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", pvcName, "-o", "json", "-n", NameSpace)
			if err != nil {
				return fmt.Errorf("failed to create PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(stdout, &pvc)
			if err != nil {
				return fmt.Errorf("unmarshal error: stdout=%s", stdout)
			}
			if pvc.Status.Phase != corev1.ClaimPending {
				return fmt.Errorf("pvc status error: %s, %s", pvcName, pvc.Status.Phase)
			}
			return nil
		}).Should(Succeed())
	})

	It("create pvc with ext4", func() {
		pvcName := "csi-carina-pvc3"
		stdout, stderr, err := kubectlWithInput([]byte(pvc3), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		By("Waiting for pvc pending")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", pvcName, "-o", "json", "-n", NameSpace)
			if err != nil {
				return fmt.Errorf("failed to create PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(stdout, &pvc)
			if err != nil {
				return fmt.Errorf("unmarshal error: stdout=%s", stdout)
			}
			if pvc.Status.Phase != corev1.ClaimPending {
				return fmt.Errorf("pvc status error: %s, %s", pvcName, pvc.Status.Phase)
			}
			return nil
		}).Should(Succeed())
	})

	It("create pvc with immediate", func() {
		pvcName := "csi-carina-pvc4"
		stdout, stderr, err := kubectlWithInput([]byte(pvc4), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		By("Waiting for pvc ready")
		nodeName, diskGroup := "", ""
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", pvcName, "-o", "json", "-n", NameSpace)
			if err != nil {
				return fmt.Errorf("failed to create PVC. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			var pvc corev1.PersistentVolumeClaim
			err = json.Unmarshal(stdout, &pvc)
			if err != nil {
				return fmt.Errorf("unmarshal error: stdout=%s", stdout)
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				log.Infof("pvc status error: %s, %s", pvcName, pvc.Status.Phase)
				return fmt.Errorf("pvc status: %s, %s", pvcName, pvc.Status.Phase)
			}

			By("get pv info")
			stdout, stderr, err = kubectl("get", "pv", pvc.Spec.VolumeName, "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get pv. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var pv corev1.PersistentVolume
			err = json.Unmarshal(stdout, &pv)
			if err != nil {
				return fmt.Errorf("unmarshal error: stdout=%s", stdout)
			}
			nodeName = pv.Spec.CSI.VolumeAttributes["carina.storage.io/node"]
			diskGroup = pv.Spec.CSI.VolumeAttributes["carina.storage.io/disk-group-name"]

			log.Info("pv check success")
			return nil
		}).Should(Succeed())

		Eventually(func() error {
			By("disk capacity check")
			stdout, stderr, err = kubectl("get", "node", nodeName, "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get node. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			var node corev1.Node
			err = json.Unmarshal(stdout, &node)
			if err != nil {
				log.Infof("failed to ummarshal node. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
				return fmt.Errorf("failed to ummarshal node. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			capacity := node.Status.Capacity.Name(corev1.ResourceName(fmt.Sprintf("carina.storage.io/%s", diskGroup)), resource.BinarySI).Value()
			allocatable := node.Status.Allocatable.Name(corev1.ResourceName(fmt.Sprintf("carina.storage.io/%s", diskGroup)), resource.BinarySI).Value()
			if capacity != allocatable+10+7 {
				log.Infof("failed to allocatable node. capacity: %d, allocatable: %d", capacity, allocatable)
				return fmt.Errorf("failed to allocatable node. capacity: %d, allocatable: %d", capacity, allocatable)
			}
			log.Infof("success to allocatable node. capacity: %d, allocatable: %d", capacity, allocatable)
			return nil
		}).Should(Succeed())
	})
}

func testDeletePvc() {
	It("delete pvc", func() {
		pvcName := "csi-carina-pvc1"
		stdout, stderr, err := kubectl("delete", "pvc", pvcName, "-n", NameSpace)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", pvcName, "-n", NameSpace)
			if err != nil {
				return err
			}
			return nil
		}).Should(HaveOccurred())

		pvcName = "csi-carina-pvc2"
		stdout, stderr, err = kubectl("delete", "pvc", pvcName, "-n", NameSpace)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", pvcName, "-n", NameSpace)
			if err != nil {
				return err
			}
			return nil
		}).Should(HaveOccurred())

		pvcName = "csi-carina-pvc3"
		stdout, stderr, err = kubectl("delete", "pvc", pvcName, "-n", NameSpace)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", pvcName, "-n", NameSpace)
			if err != nil {
				return err
			}
			return nil
		}).Should(HaveOccurred())

		pvcName = "csi-carina-pvc4"
		stdout, stderr, err = kubectl("delete", "pvc", pvcName, "-n", NameSpace)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", pvcName, "-n", NameSpace)
			if err != nil {
				return err
			}
			return nil
		}).Should(HaveOccurred())
	})
}
