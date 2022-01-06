
IMAGE_REPOSITORY=registry.cn-hangzhou.aliyuncs.com
VERSION ?= latest
DATE=$(shell date '+%Y%m%d%H%M%S')
ARCH ?= linux/arm,linux/arm64,linux/amd64

# Run tests
test:
	go test -v ./... -coverprofile cover.out

# Build the docker image
docker-build: test
	go mod vendor
	docker build . -t ${IMG}
	rm -rf vendor


# Push the docker image
release:
	go mod vendor
	docker buildx build -t $(IMAGE_REPOSITORY)/antmoveh/carina-scheduler:$(VERSION)-$(DATE) --platform=$(ARCH) . --push
	rm -rf vendor

