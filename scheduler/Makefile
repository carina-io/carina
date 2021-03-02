

# Build the docker image
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
docker-push:
# 	docker push ${IMG}
	docker rmi docker.hub.com/carina/scheduler:latest 1>/dev/null 2>&1;\
    docker build -t docker.hub.com/carina/scheduler:latest . ;\
    docker push docker.hub.com/carina/scheduler:latest


