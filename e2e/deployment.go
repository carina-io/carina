package e2e

import (
	"bocloud.com/cloudnative/carina/utils/log"
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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

func testDeployment1() {
	It("pod mount xfs filesystem", func() {
		stdout, stderr, err := kubectlWithInput([]byte(deployment1), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		By("Waiting for pod running")
		label := "app=web-server1"
		podName := ""
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

				By("pod webhook validate")
				Expect(pod.Spec.SchedulerName).Should(Equal("carina-scheduler"))

				if pod.Status.Phase != corev1.PodRunning {
					log.Infof("pod %s status %s", pod.Name, pod.Status.Phase)
					return fmt.Errorf("pod %s not running", pod.Name)
				}

				log.Infof("pod %s is running", pod.Name)

				By("exec pod ...")
				stdout, stderr, err = kubectl("exec", "-it", "-n", NameSpace, podName, "--", "sh", "df", "-T", "-i", "/var/lib/www/html")
				if err != nil {
					log.Infof("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
					return fmt.Errorf("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
				}

				log.Info(stdout)

			}

			return nil
		}, 5*time.Minute, 10*time.Second).Should(Succeed())
	})

}

func testDeployment3() {
	It("pod mount xfs filesystem", func() {
		stdout, stderr, err := kubectlWithInput([]byte(deployment3), "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		By("Waiting for pod running")
		label := "app=web-server3"
		podName := ""
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

				By("pod webhook validate")
				Expect(pod.Spec.SchedulerName).Should(Equal("carina-scheduler"))

				if pod.Status.Phase != corev1.PodRunning {
					log.Infof("pod %s status %s", pod.Name, pod.Status.Phase)
					return fmt.Errorf("pod %s not running", pod.Name)
				}

				log.Infof("pod %s is running", pod.Name)

				By("exec pod ...")
				stdout, stderr, err = kubectl("exec", "-it", "-n", NameSpace, podName, "--", "sh", "df", "-T", "-i", "/var/lib/www/html")
				if err != nil {
					log.Infof("failed to df. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
					return fmt.Errorf("failed to cat. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
				}

				log.Info(stdout)
			}

			return nil
		}, 5*time.Minute, 10*time.Second).Should(Succeed())
	})
}

func testDeleteDeployment() {
	It("delete mount filesystem pod", func() {
		deploymentName := "carina-deployment1"
		stdout, stderr, err := kubectl("delete", "deployment", deploymentName, "-n", NameSpace)
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		By("Waiting for pod delete")
		Eventually(func() error {
			stdout, stderr, err = kubectl("get", "deployment", deploymentName, "-n", NameSpace)
			if err != nil {
				log.Infof("get deployment %s error %v", deploymentName, err)
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
				log.Infof("get deployment %s error %v", deploymentName, err)
				return err
			}
			return nil
		}).Should(HaveOccurred())
	})
}
