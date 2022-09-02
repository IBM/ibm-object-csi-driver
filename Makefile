CMDS=satellite-object-storage-plugin

REGISTRY_NAME=quay.io/satellite-object-storage-plugin
IMAGE_TAGS=canary

all: build

.PHONY: build-% build container-% container clean

REV=$(shell git describe --long --tags --match='v*' --dirty 2>/dev/null || git rev-list -n1 HEAD)


IMAGE_NAME=$(REGISTRY_NAME)/$*

ARCH := $(if $(GOARCH),$(GOARCH),$(shell go env GOARCH))


build-%:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-X main.version=$(REV) -extldflags "-static"' -o ./bin/$* ./cmd/$*
	if [ "$$ARCH" = "amd64" ]; then \
		CGO_ENABLED=0 GOOS=windows go build -a -ldflags '-X main.version=$(REV) -extldflags "-static"' -o ./bin/$*.exe ./cmd/$* ; \
		CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le go build -a -ldflags '-X main.version=$(REV) -extldflags "-static"' -o ./bin/$*-ppc64le ./cmd/$* ; \
	fi

container-%: build-%
	docker build -t $*:latest -f $(shell if [ -e ./cmd/$*/Dockerfile ]; then echo ./cmd/$*/Dockerfile; else echo Dockerfile; fi) --label revision=$(REV) .

build: $(CMDS:%=build-%)
container: $(CMDS:%=container-%)

clean:
	-rm -rf bin