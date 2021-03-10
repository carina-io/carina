package e2e

import (
	//. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var sc1 = `
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-carina-sc1
provisioner: carina.storage.io
parameters:
  # file system
  csi.storage.k8s.io/fstype: xfs
  # disk group
  carina.storage.io/disk: carina-vg-hdd
reclaimPolicy: Delete
allowVolumeExpansion: true
# WaitForFirstConsumer表示被容器绑定调度后再创建pv
volumeBindingMode: WaitForFirstConsumer
mountOptions:
`

var sc2 = `
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-carina-sc2
provisioner: carina.storage.io
parameters:
  # file system
  csi.storage.k8s.io/fstype: xfs
reclaimPolicy: Delete
allowVolumeExpansion: true
# WaitForFirstConsumer表示被容器绑定调度后再创建pv
volumeBindingMode: WaitForFirstConsumer
mountOptions:
`

var sc3 = `
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-carina-sc3
provisioner: carina.storage.io
parameters:
reclaimPolicy: Delete
allowVolumeExpansion: true
# WaitForFirstConsumer表示被容器绑定调度后再创建pv
volumeBindingMode: WaitForFirstConsumer
mountOptions:
  - rw
`

var sc4 string = `
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-carina-sc4
provisioner: carina.storage.io
parameters:
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: Immediate
mountOptions:
`

func createStorageClass() {
	stdout, stderr, err := kubectlWithInput([]byte(sc1), "apply", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = kubectlWithInput([]byte(sc2), "apply", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = kubectlWithInput([]byte(sc3), "apply", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = kubectlWithInput([]byte(sc4), "apply", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
}

func deleteStorageClass() {
	stdout, stderr, err := kubectlWithInput([]byte(sc1), "delete", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = kubectlWithInput([]byte(sc2), "delete", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = kubectlWithInput([]byte(sc3), "delete", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = kubectlWithInput([]byte(sc4), "delete", "-f", "-")
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

}
