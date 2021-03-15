Kind=v1.20.2

kversion ?=v1.20.2


all:
	@echo "e2e"
test:
	go clean -testcache
	go test -v .
kc:
	kind create cluster --config kind.yaml --image kindest/node:$(kversion) --name e2e
	kind get kubeconfig --name="e2e" > /root/.kube/config
	kubectl get nodes

	for n in `docker ps --filter name=worker --format "{{.Names}}"`; \
	do \
		for i in `seq 1 5`; \
		do \
			docker exec $$n truncate --size=200G /tmp/disk$$i.device && \
			sleep 1 &&\
			docker exec $$n losetup -f /tmp/disk$$i.device --show; \
		done \
	done
	
kd:
	kind delete cluster --name e2e
	for name in `lsblk --paths --output NAME |grep loop`; \
	do \
		losetup -d $$name; \
	done
