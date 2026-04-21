BINARY := metal-robot
MAINMODULE := github.com/metal-stack/metal-robot/cmd/metal-robot
KUBECONFIG := $(or ${KUBECONFIG},.kubeconfig)

.PHONY: all
all::
	go mod tidy

release:: all;

.PHONY: start
start: all
	bin/metal-robot \
	  --bind-addr 0.0.0.0 \
	  --log-level debug \

.PHONY: build
build:
	go build \
		-ldflags "$(LINKMODE) -X 'github.com/metal-stack/v.Version=$(VERSION)' \
					   -X 'github.com/metal-stack/v.Revision=$(GITVERSION)' \
					   -X 'github.com/metal-stack/v.GitSHA1=$(SHA)' \
					   -X 'github.com/metal-stack/v.BuildDate=$(BUILDDATE)'" \
		-o bin/$(BINARY) $(MAINMODULE)
	strip bin/$(BINARY)

.PHONY: test
test:
	go test ./... -coverprofile=coverage.out -covermode=atomic && go tool cover -func=coverage.out

.PHONY: swap
swap:
	docker build -f Dockerfile.telepresence -t telepresence-container .
	docker run \
	  --privileged --rm -it \
	  -v ${KUBECONFIG}:/.kubeconfig:ro \
	  --network host \
	  -e KUBECONFIG=/.kubeconfig \
	  telepresence-container telepresence --swap-deployment metal-robot --namespace metal-robot --run-shell
