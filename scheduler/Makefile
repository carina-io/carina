
IMAGE_REPOSITORY=registry.cn-hangzhou.aliyuncs.com
VERSION ?= latest
DATE=$(shell date '+%Y%m%d%H%M%S')

# Run tests
test:
	go test -v ./... -coverprofile cover.out

# Build the docker image
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
docker-push:
# 	docker push ${IMG}
	docker rmi docker.hub.com/carina/scheduler:latest 1>/dev/null 2>&1;\
    docker build -t docker.hub.com/carina/scheduler:latest . ;\
    docker push docker.hub.com/carina/scheduler:latest

# Push the docker image
release:
	docker rmi $(IMAGE_REPOSITORY)/antmoveh/scheduler:$(VERSION)-$(DATE) 2>&1 1>/dev/null;\
    docker build -t $(IMAGE_REPOSITORY)/antmoveh/scheduler:$(VERSION)-$(DATE) . ;\
    docker push $(IMAGE_REPOSITORY)/antmoveh/scheduler:$(VERSION)-$(DATE)
