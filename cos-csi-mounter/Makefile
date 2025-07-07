NAME := cos-csi-mounter
APP_VERSION := 0.5.5
BUILD_DIR := $(NAME)-$(APP_VERSION)
BIN_DIR := bin

DEBIAN_CONTROL := $(BUILD_DIR)/DEBIAN/control
MAINTAINER := "IKS Storage"
DEB_ARCH := all
DESCRIPTION := "IBM cos-csi-mounter service"

INSTALL_TAR_FILE := "$(NAME)-$(APP_VERSION).deb.tar.gz"
CHECKSUM_FILE := "$(INSTALL_TAR_FILE).sha256"

test:
	go test -v -timeout 1800s -coverprofile=cover.out ./...
	go tool cover -html=cover.out -o=cover.html

ut-coverage: test
	@./scripts/coverage.sh
	rm cover.html cover.out

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod mod -o ${BIN_DIR}/cos-csi-mounter-server -ldflags "-s -w" -a ./server

deb-build:
deb-build: build-linux
	rm -rf $(BUILD_DIR)
	mkdir -p $(BUILD_DIR)/DEBIAN
	mkdir -p ${BUILD_DIR}/etc/systemd/system/ ${BUILD_DIR}/usr/local/bin/ $(BUILD_DIR)/etc/ibmcloud

	cp install/cos-csi-mounter.service $(BUILD_DIR)/etc/systemd/system/
	cp install/share.conf $(BUILD_DIR)/etc/ibmcloud/
	cp ${BIN_DIR}/cos-csi-mounter-server $(BUILD_DIR)/usr/local/bin/
	cp install/postinst.sh $(BUILD_DIR)/DEBIAN/postinst
	cp install/prerm.sh $(BUILD_DIR)/DEBIAN/prerm

	echo "Package: $(BUILD_DIR)" > $(DEBIAN_CONTROL)
	echo "Version: $(APP_VERSION)" >> $(DEBIAN_CONTROL)
	echo "Maintainer: $(MAINTAINER)" >> $(DEBIAN_CONTROL)
	echo "Architecture: $(DEB_ARCH)" >> $(DEBIAN_CONTROL)
	echo "Description: $(DESCRIPTION)" >> $(DEBIAN_CONTROL)

	dpkg-deb --build $(BUILD_DIR)
	rm -rf $(BUILD_DIR)


tar-package:
tar-package:
	mkdir packages
	cp ${NAME}-* packages/
	cd packages && tar -czvf ../$(INSTALL_TAR_FILE) *
	sha256sum $(INSTALL_TAR_FILE) > $(CHECKSUM_FILE)
	@printf "Production - Install package created ok: $(INSTALL_TAR_FILE)\n"

clean:
	rm -rf ${BIN_DIR}
	rm *.deb
	rm -rf packages

packages:
packages: deb-build tar-package clean