BINARY := metal-robot
MAINMODULE := github.com/metal-stack/metal-robot/cmd/metal-robot
COMMONDIR := $(or ${COMMONDIR},../builder)
KUBECONFIG := $(or ${KUBECONFIG},.kubeconfig)

GITHUB_WEBHOOK_SECRET := $(or ${GITHUB_WEBHOOK_SECRET},something)
GITHUB_APP_ID := $(or ${GITHUB_APP_ID},72006)
GITHUB_APP_PRIVATE_KEY_PEM := $(or ${GITHUB_APP_PRIVATE_KEY_PEM},key.pem)

GITLAB_WEBHOOK_SECRET := $(or ${GITLAB_WEBHOOK_SECRET},something)

include $(COMMONDIR)/Makefile.inc

.PHONY: all
all::
	go mod tidy

release:: all;

.PHONY: start
start: all
	bin/metal-robot \
	  --bind-addr 0.0.0.0 \
	  --log-level debug \
	  --github-webhook-secret $(GITHUB_WEBHOOK_SECRET) \
	  --github-app-id $(GITHUB_APP_ID) \
	  --github-app-private-key-path $(GITHUB_APP_PRIVATE_KEY_PEM) \
	  --gitlab-webhook-secret $(GITLAB_WEBHOOK_SECRET)

.PHONY: local
local:
	docker build -f Dockerfile.telepresence -t telepresence-container .
	docker run \
	  --privileged --rm -it \
	  -v ${KUBECONFIG}:/.kubeconfig:ro \
	  --network host \
	  -e KUBECONFIG=/.kubeconfig \
	  telepresence-container telepresence --swap-deployment metal-robot --namespace metal-robot --run-shell
