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
	"math/rand"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestE2e(t *testing.T) {

	rand.Seed(time.Now().UnixNano())

	RegisterFailHandler(Fail)
	SetDefaultEventuallyPollingInterval(5 * time.Second)
	SetDefaultEventuallyTimeout(2 * time.Minute)
	RunSpecs(t, "E2e Suite")
}

var _ = BeforeSuite(func() {
	createNamespace(NameSpace)
	createStorageClass()

})

var _ = AfterSuite(func() {
	deleteNamespace(NameSpace)
	deleteStorageClass()
})

var _ = Describe("Carina", func() {
	BeforeEach(func() {
		By("start carina e2e test.")
	})

	AfterEach(func() {
		By("end carina e2e test.")

	})

	Context("init", func() {
		It("empty", func() {
			By("empty context")
		})
	})

	Context("create normal pod", normalDeployment)
	Context("pvc create", testCreatePvc)
	Context("mount xfs filesystem test", mountXfsFileSystem)
	Context("mount ext4 filesystem test", mountExt4FileSystem)
	Context("raw block pod", rawBlockPod)
	Context("create statefulSet pod", statefulSetCreate)
	Context("create topostatefulSet pod", topoStatefulSetCreate)

	By("cleanup all resources")
	Context("delete all deployment", deleteAllDeployment)
	Context("delete block pod", deleteBlockPod)
	Context("delete statefulSet pod", deleteStatefulSet)
	Context("delete topostatefulSet pod", deletetopoStatefulSet)
	Context("all pvc delete", testDeletePvc)
	Context("delete normal pod", deleteNormalDeployment)
})
