Kind=v1.20.2

kversion ?=v1.20.2
# e2e settings
# Allow limiting the scope of the e2e tests. By default run everything
FOCUS ?= .*
E2E_SKIP ?= Bcache
# number of parallel test
E2E_NODES ?= 3
E2E_CHECK_LEAKS ?=

e2e: 
	@echo "e2e"
	E2E_NODES='$(E2E_NODES)' FOCUS='$(FOCUS)' E2E_SKIP='$(E2E_SKIP)' E2E_CHECK_LEAKS='$(E2E_CHECK_LEAKS)' ./e2e.sh

test:
	go clean -testcache
	go test -v .
kc:
	kind create cluster --config kind.yaml --image kindest/node:$(kversion) --name e2e
	kubectl get nodes

	for n in `docker ps --filter name=worker --format "{{.Names}}"`; \
	do \
		for i in `seq 1 2`; \
		do \
			docker exec $$n truncate --size=200G /tmp/disk$$i.device && \
			sleep 2 &&\
			docker exec $$n losetup -f /tmp/disk$$i.device --show; \
		done \
	done
kd:
	kind delete cluster --name e2e
	sudo losetup -D

install:
	cd deploycarina && ./deploy.sh install

uninstall:
	cd deploycarina && ./deploy.sh uninstall

