
IMAGE_REPOSITORY=registry.cn-hangzhou.aliyuncs.com
VERSION ?= latest
DATE=$(shell date '+%Y%m%d%H%M%S')
TAG = v0.9.0-25-g722a6ea-dirty
# Run tests
test:
	go test -v ./... -coverprofile cover.out

# Build the docker image
docker-build: test
	docker build . -t ${IMG}


# Push the docker image
release:
	docker buildx build -t $(IMAGE_REPOSITORY)/antmoveh/carina-scheduler:$(VERSION)-$(DATE) --platform=linux/arm64,linux/amd64 . --push

