package e2e

import (
	"fmt"
	"os"
	//. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func createNamespace(ns string) {
	stdout, stderr, err := kubectl("create", "namespace", ns)
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	Eventually(func() error {
		return waitCreatingDefaultSA(ns)
	}).Should(Succeed())
	fmt.Fprintln(os.Stderr, "created namespace: "+ns)
}

func waitCreatingDefaultSA(ns string) error {
	stdout, stderr, err := kubectl("get", "sa", "-n", ns, "default")
	if err != nil {
		return fmt.Errorf("default sa is not found. stdout=%s, stderr=%s, err=%v", stdout, stderr, err)
	}
	return nil
}

func deleteNamespace(ns string) {
	stdout, stderr, err := kubectl("delete", "ns", ns)
	Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
}
