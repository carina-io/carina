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

var statefulset1 = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: carina-stateful
  namespace: carina
spec:
  serviceName: "mysql-service"
  replicas: 2
  selector:
    matchLabels:
      app: mysql
  template:
    metadata:
      labels:
        app: mysql
    spec:
      terminationGracePeriodSeconds: 10
      containers:
        - name: mysqlpod
          image: mysql:5.7
          env:
            - name: MYSQL_ROOT_PASSWORD
              value: "123456"
          ports:
            - containerPort: 80
              name: my-port
          volumeMounts:
            - name: db
              mountPath: /var/lib/mysql
  volumeClaimTemplates:
    - metadata:
        name: db
      spec:
        accessModes: [ "ReadWriteOnce" ]
        storageClassName: csi-carina-sc1
        resources:
          requests:
            storage: 3Gi
`

func statefulSetCreate() {
	label := "app=mysql"
	It("statefulSet pod auto mount", func() {
		log.Info("Waiting for pod running")
		stdout, stderr, err := kubectlWithInput([]byte(statefulset1), "apply", "-f", "-")
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
				stdout, stderr, err = kubectl("exec", "-n", NameSpace, pod.Name, "--", "df", "-h", "-T", "/var/lib/mysql")
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
				Expect(3 - fileCapacity).Should(BeNumerically("<=", 1))
			}
			return nil
		}, 5*time.Minute, 10*time.Second).Should(Succeed())
	})

}

func deleteStatefulSet() {
	It("delete StatefulSet", func() {
		statefulSet := "carina-stateful"
		stdout, stderr, err := kubectl("delete", "StatefulSet", statefulSet, "-n", NameSpace)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		By("Waiting for pod delete")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "StatefulSet", statefulSet, "-n", NameSpace)
			if err != nil {
				log.Infof("delete StatefulSet %s success %v", statefulSet, err)
				return err
			}
			return nil
		}).Should(HaveOccurred())

	})
}
