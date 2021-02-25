module carina

go 1.13

require (
	github.com/fsnotify/fsnotify v1.4.9
	github.com/go-logr/logr v0.3.0
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.4.2
	github.com/labstack/echo/v4 v4.1.17
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.1
	go.uber.org/zap v1.15.0
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c
	google.golang.org/grpc v1.32.0
	google.golang.org/grpc/examples v0.0.0-20210223174733-dabedfb38b74 // indirect
	google.golang.org/protobuf v1.25.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/component-base v0.20.2
	k8s.io/klog/v2 v2.2.0
	k8s.io/kubernetes v1.19.8
	sigs.k8s.io/controller-runtime v0.8.2
)

replace (
	k8s.io/api => k8s.io/api v0.19.8
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.8
	k8s.io/apiserver => k8s.io/apiserver v0.19.8
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.8
	k8s.io/client-go => k8s.io/client-go v0.19.8
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.19.8
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.19.8
	k8s.io/code-generator => k8s.io/code-generator v0.19.8
	k8s.io/component-base => k8s.io/component-base v0.19.8
	k8s.io/cri-api => k8s.io/cri-api v0.19.8
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.19.8
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.8
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.19.8
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.19.8
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.19.8
	k8s.io/kubectl => k8s.io/kubectl v0.19.8
	k8s.io/kubelet => k8s.io/kubelet v0.19.8
	k8s.io/kubernetes => k8s.io/kubernetes v1.19.8
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.19.8
	k8s.io/metrics => k8s.io/metrics v0.19.8
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.19.8
)
