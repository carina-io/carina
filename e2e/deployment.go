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
	"strconv"
	"strings"
	"time"
)

var deployment1 = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: carina-deployment1
  namespace: carina
  labels:
    app: web-server1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web-server1
  template:
    metadata:
      labels:
        app: web-server1
    spec:
      containers:
        - name: web-server1
          image: docker.io/library/nginx:latest
          volumeMounts:
            - name: mypvc1
              mountPath: /var/lib/www/html
      volumes:
        - name: mypvc1
          persistentVolumeClaim:
            claimName: csi-carina-pvc1
            readOnly: false
`

var deployment3 = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: carina-deployment3
  namespace: carina
  labels:
    app: web-server3
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web-server3
  template:
    metadata:
      labels:
        app: web-server3
    spec:
      containers:
        - name: web-server3
          image: docker.io/library/nginx:latest
          volumeMounts:
            - name: mypvc3
              mountPath: /var/lib/www/html
      volumes:
        - name: mypvc3
          persistentVolumeClaim:
            claimName: csi-carina-pvc3
            readOnly: false
`

func mountXfsFileSystem() {
	podName := ""
	label := "app=web-server1"
	It("pod mount xfs filesystem", func() {
		log.Info("Waiting for pod running")
		stdout, stderr, err := kubectlWithInput([]byte(deployment1), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pods", "-l", label, "-o", "json", "-n", NameSpace)
			if err != nil {
				log.Infof("get pod label %s, error %v", label, err)
				return err
			}
			var pods corev1.PodList
			err = json.Unmarshal(stdout, &pods)
			if err != nil {
				return fmt.Errorf("unmarshal error: stdout=%s", stdout)
			}

			for _, pod := range pods.Items {
				if pod.Name == "" {
					log.Infof("not found pod label %s", label)
					return fmt.Errorf("not found pod label %s", label)
				}

				By("pod scheduler validate")
				Expect(pod.Spec.SchedulerName).Should(Equal("carina-scheduler"))

				if pod.Status.Phase != corev1.PodRunning {
					log.Infof("pod %s status %s", pod.Name, pod.Status.Phase)
					return fmt.Errorf("pod %s not running", pod.Name)
				}

				log.Infof("pod %s is running", pod.Name)

				By("exec pod ...")
				stdout, stderr, err = kubectl("exec", "-n", NameSpace, pod.Name, "--", "df", "-h", "-T", "/var/lib/www/html")
				if err != nil {
					log.Infof("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
					return fmt.Errorf("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
				}

				By("check mount device capacity")
				mountFileInfo := string(stdout)
				log.Info(mountFileInfo)
				Expect(mountFileInfo).To(ContainSubstring("xfs"))
				mountFileList := strings.Split(mountFileInfo, " ")
				fileCapacity := 0
				for _, m := range mountFileList {
					if strings.HasSuffix(m, "G") {
						m1 := strings.Replace(m, "G", "", 1)
						fileCapacity, _ = strconv.Atoi(strings.Split(m1, ".")[0])
						break
					}
				}
				log.Infof("xfs file capacity %d", fileCapacity)
				Expect(baseCapacity - fileCapacity).Should(BeNumerically("<=", 1))

				podName = pod.Name
			}
			return nil
		}, 5*time.Minute, 10*time.Second).Should(Succeed())
	})

	It("xfs filesystem expand", func() {
		log.Info("xfs filesystem expand")
		stdout, stderr, err := kubectl("patch", "pvc", xfsPvcName, "-n", NameSpace, "-p", `{"spec": {"resources": {"requests": {"storage": "14Gi"}}}}`)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			By("exec pod ...")
			stdout, stderr, err = kubectl("exec", "-n", NameSpace, podName, "--", "df", "-h", "-T", "/var/lib/www/html")
			if err != nil {
				log.Infof("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
				return fmt.Errorf("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			By("check mount device capacity")
			mountFileInfo := string(stdout)
			log.Info(mountFileInfo)
			Expect(mountFileInfo).To(ContainSubstring("xfs"))
			mountFileList := strings.Split(mountFileInfo, " ")
			fileCapacity := 0
			for _, m := range mountFileList {
				if strings.HasSuffix(m, "G") {
					m1 := strings.Replace(m, "G", "", 1)
					fileCapacity, _ = strconv.Atoi(strings.Split(m1, ".")[0])
					break
				}
			}
			log.Infof("xfs file capacity %d", fileCapacity)

			if (expandCapacity - fileCapacity) > 1 {
				return fmt.Errorf("xfs filesystem expand in progress")
			}

			return nil
		}, 5*time.Minute, 20*time.Second).Should(Succeed())
	})

	It("xfs filesystem pod restart", func() {
		log.Info("xfs filesystem pod restart")
		stdout, stderr, err := kubectl("delete", "pod", podName, "-n", NameSpace)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pods", "-l", label, "-o", "json", "-n", NameSpace)
			if err != nil {
				log.Infof("get pod label %s, error %v", label, err)
				return err
			}
			var pods corev1.PodList
			err = json.Unmarshal(stdout, &pods)
			if err != nil {
				return fmt.Errorf("unmarshal error: stdout=%s", stdout)
			}

			if len(pods.Items) == 0 {
				log.Info("pods not create")
				return fmt.Errorf("pods not create")
			}

			for _, pod := range pods.Items {
				if pod.Name == "" {
					log.Infof("not found pod label %s", label)
					return fmt.Errorf("not found pod label %s", label)
				}

				By("pod scheduler validate")
				Expect(pod.Spec.SchedulerName).Should(Equal("carina-scheduler"))

				if pod.Status.Phase != corev1.PodRunning {
					log.Infof("pod %s status %s", pod.Name, pod.Status.Phase)
					return fmt.Errorf("pod %s not running", pod.Name)
				}

				log.Infof("pod %s is running", pod.Name)

				By("exec pod ...")
				stdout, stderr, err = kubectl("exec", "-n", NameSpace, pod.Name, "--", "df", "-h", "-T", "/var/lib/www/html")
				if err != nil {
					log.Infof("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
					return fmt.Errorf("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
				}

				By("check mount device capacity")
				mountFileInfo := string(stdout)
				log.Info(mountFileInfo)
				Expect(mountFileInfo).To(ContainSubstring("xfs"))
				mountFileList := strings.Split(mountFileInfo, " ")
				fileCapacity := 0
				for _, m := range mountFileList {
					if strings.HasSuffix(m, "G") {
						m1 := strings.Replace(m, "G", "", 1)
						fileCapacity, _ = strconv.Atoi(strings.Split(m1, ".")[0])
						break
					}
				}
				log.Infof("xfs file capacity %d", fileCapacity)
				Expect(expandCapacity - fileCapacity).Should(BeNumerically("<=", 1))
			}
			return nil
		}, 5*time.Minute, 10*time.Second).Should(Succeed())
	})

}

func mountExt4FileSystem() {
	podName := ""
	label := "app=web-server3"
	It("pod mount ext4 filesystem", func() {
		log.Info("Waiting for pod running")
		By("pod mount ext4 filesystem")
		stdout, stderr, err := kubectlWithInput([]byte(deployment3), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pods", "-l", label, "-o", "json", "-n", NameSpace)
			if err != nil {
				log.Infof("get pod label %s, error %v", label, err)
				return err
			}
			var pods corev1.PodList
			err = json.Unmarshal(stdout, &pods)
			if err != nil {
				return fmt.Errorf("unmarshal error: stdout=%s", stdout)
			}

			if len(pods.Items) == 0 {
				log.Info("pods not create")
				return fmt.Errorf("pods not create")
			}

			for _, pod := range pods.Items {
				if pod.Name == "" {
					log.Infof("not found pod label %s", label)
					return fmt.Errorf("not found pod label %s", label)
				}

				By("pod scheduler validate")
				Expect(pod.Spec.SchedulerName).Should(Equal("carina-scheduler"))

				if pod.Status.Phase != corev1.PodRunning {
					log.Infof("pod %s status %s", pod.Name, pod.Status.Phase)
					return fmt.Errorf("pod %s not running", pod.Name)
				}

				log.Infof("pod %s is running", pod.Name)

				By("exec pod ...")
				stdout, stderr, err = kubectl("exec", "-n", NameSpace, pod.Name, "--", "df", "-h", "-T", "/var/lib/www/html")
				if err != nil {
					log.Infof("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
					return fmt.Errorf("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
				}

				By("check mount device capacity")
				mountFileInfo := string(stdout)
				log.Info(mountFileInfo)
				Expect(mountFileInfo).To(ContainSubstring("ext4"))
				mountFileList := strings.Split(mountFileInfo, " ")
				fileCapacity := 0
				for _, m := range mountFileList {
					if strings.HasSuffix(m, "G") {
						m1 := strings.Replace(m, "G", "", 1)
						fileCapacity, _ = strconv.Atoi(strings.Split(m1, ".")[0])
						break
					}
				}
				log.Infof("ext4 file capacity %d", fileCapacity)
				Expect(baseCapacity - fileCapacity).Should(BeNumerically("<=", 1))

				podName = pod.Name
			}
			return nil
		}, 5*time.Minute, 10*time.Second).Should(Succeed())

		log.Info("ext4 filesystem expand")
		By("ext4 filesystem expand")

		//stdout, stderr, err = kubectlWithInput([]byte(strings.Replace(pvc3, "7", "14", 1)), "apply", "-f", "-")
		stdout, stderr, err = kubectl("patch", "pvc", ext4PvcName, "-n", NameSpace, "-p", `{"spec": {"resources": {"requests": {"storage": "14Gi"}}}}`)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			By("exec pod ...")
			stdout, stderr, err = kubectl("exec", "-n", NameSpace, podName, "--", "df", "-h", "-T", "/var/lib/www/html")
			if err != nil {
				log.Infof("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
				return fmt.Errorf("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}

			By("check mount device capacity")
			mountFileInfo := string(stdout)
			log.Info(mountFileInfo)
			Expect(mountFileInfo).To(ContainSubstring("ext4"))
			mountFileList := strings.Split(mountFileInfo, " ")
			fileCapacity := 0
			for _, m := range mountFileList {
				if strings.HasSuffix(m, "G") {
					m1 := strings.Replace(m, "G", "", 1)
					fileCapacity, _ = strconv.Atoi(strings.Split(m1, ".")[0])
					break
				}
			}
			log.Infof("ext4 file capacity %d", fileCapacity)

			if (expandCapacity - fileCapacity) > 1 {
				return fmt.Errorf("ext4 filesystem expand in progress")
			}

			return nil
		}, 5*time.Minute, 20*time.Second).Should(Succeed())

		log.Info("ext4 filesystem pod restart")
		By("ext4 filesystem pod restart")
		stdout, stderr, err = kubectl("delete", "pod", podName, "-n", NameSpace)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "pods", "-l", label, "-o", "json", "-n", NameSpace)
			if err != nil {
				log.Infof("get pod label %s, error %v", label, err)
				return err
			}
			var pods corev1.PodList
			err = json.Unmarshal(stdout, &pods)
			if err != nil {
				return fmt.Errorf("unmarshal error: stdout=%s", stdout)
			}

			for _, pod := range pods.Items {
				if pod.Name == "" {
					log.Infof("not found pod label %s", label)
					return fmt.Errorf("not found pod label %s", label)
				}

				By("pod scheduler validate")
				Expect(pod.Spec.SchedulerName).Should(Equal("carina-scheduler"))

				if pod.Status.Phase != corev1.PodRunning {
					log.Infof("pod %s status %s", pod.Name, pod.Status.Phase)
					return fmt.Errorf("pod %s not running", pod.Name)
				}

				log.Infof("pod %s is running", pod.Name)

				By("exec pod ...")
				stdout, stderr, err = kubectl("exec", "-n", NameSpace, pod.Name, "--", "df", "-h", "-T", "/var/lib/www/html")
				if err != nil {
					log.Infof("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
					return fmt.Errorf("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
				}

				By("check mount device capacity")
				mountFileInfo := string(stdout)
				log.Info(mountFileInfo)
				Expect(mountFileInfo).To(ContainSubstring("ext4"))
				mountFileList := strings.Split(mountFileInfo, " ")
				fileCapacity := 0
				for _, m := range mountFileList {
					if strings.HasSuffix(m, "G") {
						m1 := strings.Replace(m, "G", "", 1)
						fileCapacity, _ = strconv.Atoi(strings.Split(m1, ".")[0])
						break
					}
				}
				log.Infof("ext4 file capacity %d", fileCapacity)
				Expect(expandCapacity - fileCapacity).Should(BeNumerically("<=", 1))
			}
			return nil
		}, 5*time.Minute, 10*time.Second).Should(Succeed())
	})

}

func deleteAllDeployment() {
	It("delete mount filesystem pod", func() {
		deploymentName := "carina-deployment1"
		stdout, stderr, err := kubectl("delete", "deployment", deploymentName, "-n", NameSpace)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		By("Waiting for pod delete")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "deployment", deploymentName, "-n", NameSpace)
			if err != nil {
				log.Infof("delete deployment %s success %v", deploymentName, err)
				return err
			}
			return nil
		}).Should(HaveOccurred())

		deploymentName = "carina-deployment3"
		stdout, stderr, err = kubectl("delete", "deployment", deploymentName, "-n", NameSpace)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		By("Waiting for pod delete")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "deployment", deploymentName, "-n", NameSpace)
			if err != nil {
				log.Infof("delete deployment %s success %v", deploymentName, err)
				return err
			}
			return nil
		}).Should(HaveOccurred())
	})
}
