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
  carina.storage.io/disk-group-name: ssd
reclaimPolicy: Delete
allowVolumeExpansion: true
# WaitForFirstConsumer表示被容器绑定调度后再创建pv
volumeBindingMode: WaitForFirstConsumer
mountOptions:
  - rw
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
`

var sc4 = `
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
