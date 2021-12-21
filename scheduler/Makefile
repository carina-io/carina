
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
latest:
	docker rmi $(IMAGE_REPOSITORY)/antmoveh/scheduler:$(TAG) 2>&1 1>/dev/null;\
    docker build -t $(IMAGE_REPOSITORY)/antmoveh/scheduler:$(TAG) . ;\
    docker push $(IMAGE_REPOSITORY)/antmoveh/scheduler:$(TAG)

# Push the docker image
release:
	docker rmi $(IMAGE_REPOSITORY)/antmoveh/scheduler:$(VERSION)-$(DATE) 2>&1 1>/dev/null;\
    docker build -t $(IMAGE_REPOSITORY)/antmoveh/scheduler:$(VERSION)-$(DATE) . ;\
    docker push $(IMAGE_REPOSITORY)/antmoveh/scheduler:$(VERSION)-$(DATE)
