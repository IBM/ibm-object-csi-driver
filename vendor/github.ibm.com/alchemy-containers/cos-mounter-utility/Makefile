# ******************************************************************************
# * Licensed Materials - Property of IBM
# * IBM Cloud Container Service, 5737-D43
# * (C) Copyright IBM Corp. 2020 All Rights Reserved.
# * US Government Users Restricted Rights - Use, duplication or
# * disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
# ******************************************************************************

SHELL = /bin/bash
SERVER_EXE_NAME = s3fs-mounter-server
CLIENT_EXE_NAME = s3fs-mounter-client
PLUGIN_NAME = ibmcloud-object-storage-csi
GIT_COMMIT_SHA := "$(shell git rev-parse HEAD 2>/dev/null)"
GIT_REMOTE_URL := "$(shell git config --get remote.origin.url 2>/dev/null)"
BUILD_DATE := "$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")"
ARCH := $(shell docker version -f {{.Client.Arch}})
IBMCLOUD_S3FS_CONT_IMG := "armada-master/ibmcloud-cos-mounter"
IBMCLOUD_S3FS_TAG := "latest"

PKG_CODE_DIR := "$(GOPATH)/src/github.ibm.com/alchemy-containers/cos-mounter-utility"
PROXY_IMAGE_URL:= "registry.access.redhat.com"
ifdef ARTIFACTORY_JFRT_TOKEN
GOPROXY := https://na.artifactory.swg-devops.com/artifactory/api/go/wcp-alchemy-containers-team-go-virtual
GONOPROXY := github.ibm.com
PROXY_IMAGE_URL="docker-na.artifactory.swg-devops.com/wcp-alchemy-containers-team-access-redhat-docker-remote"
endif


.PHONY: deps
deps:
	go mod download


.PHONY: install-bt
install-bt:
	# Install 'bt' via git clone + installer script. Set BT_VERSION to install a custom version.
	@V=$${BT_VERSION:-master} C=~/.cache/bt/repo && if ! git -C "$$C" status >/dev/null; then mkdir -p "$$C" && rm -rf "$$C" && git clone https://github.ibm.com/alchemy-containers/go-build-tools.git "$$C"; fi && pushd "$$C" && git fetch && { git reset --hard "$$V" || git reset --hard origin/"$$V"; } && popd && "$$C"/scripts/install.sh "$$V"



.PHONY: buildServer
buildServer: deps
	CGO_ENABLED=0 GOOS=$(shell go env GOOS) GOARCH=$(shell go env GOARCH) go build -mod=mod -a -ldflags '-X main.vendorVersion='"${PLUGIN_NAME}-${GIT_COMMIT_SHA}"' -extldflags "-static"' -o bin/${SERVER_EXE_NAME} ./s3fs_server.go

	bt build image -goproxy \
		docker build --rm \
			--build-arg REPO_SOURCE_URL="${REPO_SOURCE_URL}" \
			--build-arg BUILD_URL="${BUILD_URL}" \
			--build-arg PROXY_IMAGE_URL=${PROXY_IMAGE_URL} \
			-t ${IBMCLOUD_S3FS_CONT_IMG}:${IBMCLOUD_S3FS_TAG} \
			-f deploy/Dockerfile.server .
