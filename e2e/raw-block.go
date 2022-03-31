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
	"strconv"
	"strings"
	"time"

	"github.com/carina-io/carina/utils/log"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var rawPvc = `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: raw-block-pvc
  namespace: carina
spec:
  accessModes:
    - ReadWriteOnce
  volumeMode: Block
  resources:
    requests:
      storage: 13Gi
  storageClassName: csi-carina-raw
`

var rawPod = `
apiVersion: v1
kind: Pod
metadata:
  name: carina-block-pod
  namespace: carina
spec:
  containers:
    - name: centos
      securityContext:
        capabilities:
          add: ["SYS_RAWIO"]
      image: centos:latest
      command: ["/bin/sleep", "infinity"]
      volumeDevices:
        - name: data
          devicePath: /dev/xvda
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: raw-block-pvc
`

func rawBlockPod() {
	podName := "carina-block-pod"
	It("create block pod", func() {

		log.Info("create block pvc")
		stdout, stderr, err := kubectlWithInput([]byte(rawPvc), "-n", NameSpace, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		log.Info("Waiting for pod running")
		stdout, stderr, err = kubectlWithInput([]byte(rawPod), "-n", NameSpace, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		By("confirming that the pvc is bound")
		Eventually(func() error {
			stdout, stderr, err := kubectl("-n", NameSpace, "get", "pvc", "raw-block-pvc", "-o=template", "--template={{.status.phase}}")
			if err != nil {
				return fmt.Errorf("failed to get pvc. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			phase := strings.TrimSpace(string(stdout))
			if phase != "Bound" {
				return fmt.Errorf("pvc %s is not bind", "pvc-raw-device")
			}
			return nil
		}).Should(Succeed())

		By("confirming that the pod is running")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pods", podName, "-o", "json", "-n", NameSpace)
			if err != nil {
				log.Infof("get pod %s, error %v", podName, err)
				return err
			}
			var pod corev1.Pod
			err = json.Unmarshal(stdout, &pod)
			if err != nil {
				return fmt.Errorf("unmarshal error: stdout=%s", stdout)
			}

			if pod.Name == "" {
				log.Infof("not found pod %s", podName)
				return fmt.Errorf("not found pod %s", podName)
			}

			By("pod scheduler validate")
			Expect(pod.Spec.SchedulerName).Should(Equal("carina-scheduler"))

			if pod.Status.Phase != corev1.PodRunning {
				log.Infof("pod %s status %s", pod.Name, pod.Status.Phase)
				return fmt.Errorf("pod %s not running", pod.Name)
			}

			log.Infof("pod %s is running", pod.Name)

			By("exec pod ...")
			stdout, stderr, err = kubectl("exec", "-n", NameSpace, podName, "--", "blockdev", "--getsize64", "/dev/xvda")
			if err != nil {
				log.Infof("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
				return fmt.Errorf("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			By("check device capacity")
			s := strings.Replace(string(stdout), "\n", "", 1)
			blockCapacity, err := strconv.Atoi(s)
			if err != nil {
				log.Info(err.Error())
			}
			log.Infof("block device capacity %d", blockCapacity>>30)
			Expect(13 - blockCapacity>>30).Should(BeNumerically("<=", 1))

			return nil
		}, 5*time.Minute, 10*time.Second).Should(Succeed())
	})

	It("raw block expand", func() {
		log.Info("raw block expand")
		stdout, stderr, err := kubectl("patch", "pvc", "raw-block-pvc", "-n", NameSpace, "-p", `{"spec": {"resources": {"requests": {"storage": "26Gi"}}}}`)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			By("exec pod ...")
			stdout, stderr, err = kubectl("exec", "-n", NameSpace, podName, "--", "blockdev", "--getsize64", "/dev/xvda")
			if err != nil {
				log.Infof("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
				return fmt.Errorf("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			By("check device capacity")
			s := strings.Replace(string(stdout), "\n", "", 1)
			blockCapacity, err := strconv.Atoi(s)
			if err != nil {
				log.Info(err.Error())
			}
			log.Infof("block device capacity %d", blockCapacity>>30)

			if (26 - blockCapacity>>30) > 1 {
				return fmt.Errorf("device expand in progress")
			}

			return nil
		}, 5*time.Minute, 20*time.Second).Should(Succeed())
	})
}

func deleteBlockPod() {
	It("delete raw block pod", func() {
		podName := "carina-block-pod"
		stdout, stderr, err := kubectl("delete", "pod", podName, "-n", NameSpace)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		By("Waiting for pod delete")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pod", podName, "-n", NameSpace)
			if err != nil {
				log.Infof("delete pod %s success %v", podName, err)
				return err
			}
			return nil
		}).Should(HaveOccurred())

		pvcName := "raw-block-pvc"
		stdout, stderr, err = kubectl("delete", "pvc", pvcName, "-n", NameSpace)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		By("Waiting for pod delete")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pvc", pvcName, "-n", NameSpace)
			if err != nil {
				log.Infof("delete pvc %s success %v", pvcName, err)
				return err
			}
			return nil
		}).Should(HaveOccurred())
	})
}
