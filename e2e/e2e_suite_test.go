package e2e

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
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
		By("only test .")
	})

	Context("pvc Immediate Create", func() {
		It("only test 2", func() {
			Expect("2").To(Equal("2"))
		})
	})

})
