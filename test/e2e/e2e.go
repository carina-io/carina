package e2e

import (
	"os"
	"testing"

	"github.com/carina-io/carina/test/e2e/framework"
	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"k8s.io/component-base/logs"

	// tests to run
	_ "github.com/carina-io/carina/test/e2e/bcache"
	_ "github.com/carina-io/carina/test/e2e/lvm"
	_ "github.com/carina-io/carina/test/e2e/raw"
)

// RunE2ETests checks configuration parameters (specified through flags) and then runs
// E2E tests using the Ginkgo runner.
func RunE2ETests(t *testing.T) {
	logs.InitLogs()
	defer logs.FlushLogs()
	if os.Getenv("KUBECTL_PATH") != "" {
		framework.KubectlPath = os.Getenv("KUBECTL_PATH")
		framework.Logf("Using kubectl path '%s'", framework.KubectlPath)
	}

	framework.Logf("Starting e2e run %q on Ginkgo node %d", framework.RunID, config.GinkgoConfig.ParallelNode)

	ginkgo.RunSpecs(t, "carina e2e suite")
}
