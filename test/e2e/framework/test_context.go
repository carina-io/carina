package framework

import (
	"flag"

	"github.com/onsi/ginkgo/config"
)

// TestContextType describes the client context to use in communications with the Kubernetes API.
type TestContextType struct {
	KubeHost string
	//KubeConfig  string
	KubeContext string
}

// TestContext is the global client context for tests.
var TestContext TestContextType

// registerCommonFlags registers flags common to all e2e test suites.
func registerCommonFlags() {
	config.GinkgoConfig.EmitSpecProgress = true

	flag.StringVar(&TestContext.KubeHost, "kubernetes-host", "http://127.0.0.1:8080", "The kubernetes host, or apiserver, to connect to")
	//flag.StringVar(&TestContext.KubeConfig, "kubernetes-config", os.Getenv(clientcmd.RecommendedConfigPathEnvVar), "Path to config containing embedded authinfo for kubernetes. Default value is from environment variable "+clientcmd.RecommendedConfigPathEnvVar)
	flag.StringVar(&TestContext.KubeContext, "kubernetes-context", "", "config context to use for kubernetes. If unset, will use value from 'current-context'")
}

// RegisterParseFlags registers and parses flags for the test binary.
func RegisterParseFlags() {
	registerCommonFlags()
	flag.Parse()
}
