CMDS=ibm-object-csi-driver
EXE_DRIVER_NAME=ibm-object-csi-driver

REGISTRY=quay.io/ibm-object-csi-driver

export LINT_VERSION="1.57.2"

COLOR_YELLOW=\033[0;33m
COLOR_RESET=\033[0m
GOFILES=$(shell find . -type f -name '*.go' -not -path "./vendor/*")


all: build


.PHONY: build-% clean

REV=$(shell git describe --long --tags --match='v*' --dirty 2>/dev/null || git rev-list -n1 HEAD)
GIT_REMOTE_URL="$(shell git config --get remote.origin.url 2>/dev/null)"
BUILD_DATE="$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")"
BUILD_NUMBER?=unknown
TAG ?= dev
ARCH ?= amd64
ALL_ARCH ?= amd64 ppc64le

CORE_IMAGE_NAME ?= $(EXE_DRIVER_NAME)
CORE_DRIVER_IMG ?= $(REGISTRY)/$(CORE_IMAGE_NAME)

.PHONY: test
test: 
	go test -v -race ./cmd/... ./pkg/...

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

.PHONY: driver
driver: deps buildimage

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -ldflags '-X main.version=$(REV) -extldflags "-static"' -o ${GOPATH}/bin/${EXE_DRIVER_NAME} ./cmd/$*


.PHONY: buildimage
buildimage: build-binary
	docker build	\
        --build-arg RHSM_PASS=$(RHSM_PASS) \
        --build-arg RHSM_USER=$(RHSM_USER) \
	--build-arg git_commit_id=${REV} \
        --build-arg git_remote_url=${GIT_REMOTE_URL} \
        --build-arg build_date=${BUILD_DATE} \
        --build-arg jenkins_build_number=${BUILD_NUMBER} \
        --build-arg REPO_SOURCE_URL=${REPO_SOURCE_URL} \
        --build-arg BUILD_URL=${BUILD_URL} \
	-t $(CORE_DRIVER_IMG):$(ARCH)-$(TAG) -f Dockerfile .

.PHONY: build-binary
build-binary:
	docker build --build-arg TAG=$(REV) --build-arg OS=linux --build-arg ARCH=$(ARCH) -t csi-driver-builder --pull -f Dockerfile.builder .
	docker run --env GHE_TOKEN=${GHE_TOKEN} csi-driver-builder
	docker cp `docker ps -q -n=1`:/go/bin/${EXE_DRIVER_NAME} ./${EXE_DRIVER_NAME}
