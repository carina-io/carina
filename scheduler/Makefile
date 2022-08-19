
IMAGE_REPOSITORY=registry.cn-hangzhou.aliyuncs.com/carina
VERSION ?= latest
DATE=$(shell date '+%Y%m%d%H%M%S')
ARCH ?= linux/arm64,linux/amd64

# Run tests
test:
	go test -v ./... -coverprofile cover.out

# Build the docker image
docker-build:
	go mod vendor
	docker build -t $(IMAGE_REPOSITORY)/carina-scheduler:$(VERSION) .
	rm -rf vendor
	docker push $(IMAGE_REPOSITORY)/carina-scheduler:$(VERSION)

# Push the docker image
release:
	go mod vendor
	docker buildx build -t $(IMAGE_REPOSITORY)/carina-scheduler:$(VERSION) --platform=$(ARCH) . --push
	rm -rf vendor

