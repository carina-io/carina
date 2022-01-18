
IMAGE_REPOSITORY=registry.cn-hangzhou.aliyuncs.com/antmoveh
VERSION ?= latest
DATE=$(shell date '+%Y%m%d%H%M%S')
ARCH ?= linux/arm64,linux/amd64
# Image URL to use all building/pushing image targets
IMG ?= carina-scheduler:latest

# Run tests
test:
	go test -v ./... -coverprofile cover.out

# Build the docker image
docker-build:
	go mod vendor
	docker build -t $(IMG) .
	rm -rf vendor
	docker push $(IMG)

# Push the docker image
release:
	go mod vendor
	docker buildx build -t $(IMAGE_REPOSITORY)/carina-scheduler:$(VERSION) --platform=$(ARCH) . --push
	rm -rf vendor

