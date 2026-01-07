SHA := $(shell git rev-parse --short=8 HEAD)
GITVERSION := $(shell git describe --long --all)
BUILDDATE := $(shell date -Iseconds)
VERSION := $(or ${VERSION},$(shell git describe --tags --exact-match 2> /dev/null || git symbolic-ref -q --short HEAD || git rev-parse --short HEAD))

CGO_ENABLED := 1
LINKMODE := -extldflags '-static -s -w'

ifeq ($(CI),true)
  DOCKER_TTY_ARG=
else
  DOCKER_TTY_ARG=t
endif

.PHONY: all
all: test build

.PHONY: build
build:
	go build -tags netgo,osusergo \
		 -ldflags "$(LINKMODE) -X 'github.com/metal-stack/v.Version=$(VERSION)' \
								   -X 'github.com/metal-stack/v.Revision=$(GITVERSION)' \
								   -X 'github.com/metal-stack/v.GitSHA1=$(SHA)' \
								   -X 'github.com/metal-stack/v.BuildDate=$(BUILDDATE)'" \
	   -o bin/metal-robot github.com/metal-stack/metal-robot/cmd/metal-robot/...
	strip bin/metal-robot

.PHONY: test
test:
	go test ./... -coverprofile=coverage.out -covermode=atomic && go tool cover -func=coverage.out

.PHONY: test-integration
test-integration:
	# make sure you deploy the version you want to test before!
	go test -v -count=1 -tags=integration -timeout 600s -p 1 ./tests/e2e/...
