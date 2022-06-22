package e2e

import (
	"testing"

	"github.com/carina-io/carina/test/e2e/framework"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

func init() {
	testing.Init()
	framework.RegisterParseFlags()
}
func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	RunE2ETests(t)
}
