CMDS=satellite-object-storage-plugin

REGISTRY_NAME=quay.io/satellite-object-storage-plugin

#export GO111MODULE=$(GO111MODULE_FLAG)

export LINT_VERSION="1.45.2"

COLOR_YELLOW=\033[0;33m
COLOR_RESET=\033[0m
GOFILES=$(shell find . -type f -name '*.go' -not -path "./vendor/*")

IMAGE_TAGS=canary

all: build


.PHONY: build-% build container-% container clean

REV=$(shell git describe --long --tags --match='v*' --dirty 2>/dev/null || git rev-list -n1 HEAD)


IMAGE_NAME=$(REGISTRY_NAME)/$*

ARCH := $(if $(GOARCH),$(GOARCH),$(shell go env GOARCH))

.PHONY: test
test: 
	go test -v -race ./cmd/... ./pkg/...

buildlocal:
	mkdir -p bin
	CGO_ENABLED=0 go build -a -ldflags '-X main.version=$(REV) -extldflags "-static"' -o ./bin/$* ./cmd/satellite-object-storage-plugin/$*

build-%:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-X main.version=$(REV) -extldflags "-static"' -o ./bin/$* ./cmd/$*
	
	if [ "$$ARCH" = "amd64" ]; then \
		CGO_ENABLED=0 GOOS=windows go build -a -ldflags '-X main.version=$(REV) -extldflags "-static"' -o ./bin/$*.exe ./cmd/$* ; \
		CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le go build -a -ldflags '-X main.version=$(REV) -extldflags "-static"' -o ./bin/$*-ppc64le ./cmd/$* ; \
	fi

container-%: build-%
	docker build --build-arg RHSM_PASS=$(RHSM_PASS) --build-arg RHSM_USER=$(RHSM_USER) -t $*:latest -f $(shell if [ -e ./cmd/$*/Dockerfile ]; then echo ./cmd/$*/Dockerfile; else echo Dockerfile; fi) --label revision=$(REV) .
build: $(CMDS:%=build-%)
container: $(CMDS:%=container-%)

.PHONY: deps
deps:
	echo "Installing dependencies ..."
	@if ! which golangci-lint >/dev/null || [[ "$$(golangci-lint --version)" != *${LINT_VERSION}* ]]; then \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@v${LINT_VERSION}; \
	fi


.PHONY: fmt
fmt: lint
	gofmt -l ${GOFILES}

.PHONY: coverage
coverage:
	go tool cover -html=cover.out -o=cover.html

clean:
	-rm -rf bin
test-sanity:
	go test -timeout 160s ./tests/sanity/sanity_test.go

.PHONY: lint
lint:
	hack/verify-golint.sh
