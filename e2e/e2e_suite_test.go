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
	SetDefaultEventuallyPollingInterval(3 * time.Second)
	SetDefaultEventuallyTimeout(time.Minute)
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

	//Context("first pvc create", testCreatePvc)
	//Context("mount xfs filesystem test", mountXfsFileSystem)
	//Context("mount ext4 filesystem test", mountExt4FileSystem)
	//Context("raw block pod", rawBlockPod)

	Context("create statefulSet pod", statefulSetCreate)

	By("cleanup all resources")
	Context("delete statefulSet pod", deleteStatefulSet)
	//Context("delete block pod", deleteBlockPod)
	//Context("delete all deployment", deleteAllDeployment)
	//Context("first pvc delete", testDeletePvc)
	Context("delete normal pod", deleteNormalDeployment)
})
