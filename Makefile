## Dependency versions
CSI_VERSION=1.3.0
K8S_VERSION=1.20.4
KUBEBUILDER_VERSION = 2.3.1
KUSTOMIZE_VERSION= 3.8.9
PROTOC_VERSION=3.15.0
DATE=$(shell date '+%Y%m%d%H%M%S')

# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

IMAGE_REPOSITORY=registry.cn-hangzhou.aliyuncs.com
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
	go test -v ./pkg/csidriver/driver
	go test -v ./pkg/devicemanager

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

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

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
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
latest:
	go build -ldflags="-X main.gitCommitID=`git rev-parse HEAD`" -gcflags '-N -l' -o bin/carina-node github.com/bocloud/carina/cmd/carina-node ;\
	go build -ldflags="-X main.gitCommitID=`git rev-parse HEAD`" -gcflags '-N -l' -o bin/carina-controller github.com/bocloud/carina/cmd/carina-controller ;\
	docker rmi $(IMAGE_REPOSITORY)/antmoveh/carina:latest 1>/dev/null 2>&1;\
	docker build -f Dockerfile.local -t $(IMAGE_REPOSITORY)/antmoveh/carina:latest . ;\
	docker push $(IMAGE_REPOSITORY)/antmoveh/carina:latest

# Push the docker image
release:
	docker rmi $(IMAGE_REPOSITORY)/antmoveh/carina:$(VERSION)-$(DATE) 2>&1 1>/dev/null;\
    docker build -t $(IMAGE_REPOSITORY)/antmoveh/carina:$(VERSION)-$(DATE) . ;\
    docker push $(IMAGE_REPOSITORY)/antmoveh/carina:$(VERSION)-$(DATE)

# Push the docker image
local:
	go build -ldflags="-X main.gitCommitID=`git rev-parse HEAD`" -gcflags '-N -l' -o bin/carina-node github.com/bocloud/carina/cmd/carina-node ;\
	go build -ldflags="-X main.gitCommitID=`git rev-parse HEAD`" -gcflags '-N -l' -o bin/carina-controller github.com/bocloud/carina/cmd/carina-controller ;\
	docker rmi 192.168.56.101:5000/carina:latest 1>/dev/null 2>&1;\
	docker build -f Dockerfile.local -t 192.168.56.101:5000/carina:latest . ;\
	docker push 192.168.56.101:5000/carina:latest


# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
