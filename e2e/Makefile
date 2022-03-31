Kind=v1.20.2

kversion ?=v1.20.2


all: kc
	@echo "e2e"
	E2ETEST=1 /root/go/ginkgo --failFast -v .
test:
	go clean -testcache
	go test -v .
kc:
	kind create cluster --config kind.yaml --image kindest/node:$(kversion) --name e2e
	kubectl get nodes

	for n in `docker ps --filter name=worker --format "{{.Names}}"`; \
	do \
		for i in `seq 1 5`; \
		do \
			docker exec $$n truncate --size=200G /tmp/disk$$i.device && \
			sleep 2 &&\
			docker exec $$n losetup -f /tmp/disk$$i.device --show; \
		done \
	done
	
kd:
	kind delete cluster --name e2e
	losetup -D
