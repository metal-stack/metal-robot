BINARY := metal-robot
MAINMODULE := github.com/metal-stack/metal-robot/cmd/metal-robot
COMMONDIR := $(or ${COMMONDIR},../builder)
KUBECONFIG := $(or ${KUBECONFIG},.kubeconfig)


include $(COMMONDIR)/Makefile.inc

.PHONY: all
all::
	go mod tidy

release:: all;

.PHONY: local
local:
	docker build -f Dockerfile.telepresence -t telepresence-container .
	docker run \
	  --privileged --rm -it \
	  -v ${KUBECONFIG}:/.kubeconfig:ro \
	  --network host \
	  -e KUBECONFIG=/.kubeconfig \
	  telepresence-container telepresence --swap-deployment metal-robot --namespace metal-robot --run-shell
