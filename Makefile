## Dependency versions
CSI_VERSION=1.5.0
K8S_VERSION=1.21.5
KUBEBUILDER_VERSION = 3.2.1
KUSTOMIZE_VERSION= 3.8.9
PROTOC_VERSION=3.15.0
DATE=$(shell date '+%Y%m%d%H%M%S')
ARCH ?= linux/arm64,linux/amd64

# Image URL to use all building/pushing image targets
IMG ?= carina:raw
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

IMAGE_REPOSITORY=registry.cn-hangzhou.aliyuncs.com/carina
VERSION ?= latest


# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
test:
	#go test -v ./... -coverprofile cover.out
	go test -v ./utils
	go test -v ./utils/iolimit
	go test -v ./pkg/csidriver/driver
	go test -v ./runners

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./api/..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build:
	go mod vendor
	docker build -t $(IMAGE_REPOSITORY)/carina:$(VERSION) .
	rm -rf vendor
	docker push $(IMAGE_REPOSITORY)/carina:$(VERSION)

# Push the docker image
release:
	go mod vendor
	docker buildx build -t $(IMAGE_REPOSITORY)/carina:$(VERSION) --platform=$(ARCH) . --push
	rm -rf vendor

# find or download controller-gen
# download controller-gen if necessary
CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.9.0)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef