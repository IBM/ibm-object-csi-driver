NAME := cos-csi-mounter
APP_VERSION := 1.0.1
BUILD_DIR := $(NAME)-$(APP_VERSION)
BIN_DIR := bin

DEBIAN_CONTROL := $(BUILD_DIR)/DEBIAN/control
MAINTAINER := "IKS Storage"
DEB_ARCH := all
DESCRIPTION := "IBM cos-csi-mounter service"

INSTALL_TAR_FILE_DEB := "$(NAME)-$(APP_VERSION).deb.tar.gz"
INSTALL_TAR_FILE_RPM := "$(NAME)-$(APP_VERSION).rpm.tar.gz"
CHECKSUM_FILE_DEB := "$(INSTALL_TAR_FILE_DEB).sha256"
CHECKSUM_FILE_RPM := "$(INSTALL_TAR_FILE_RPM).sha256"

LICENSE := "IBM"
RPM_ARCH := x86_64
RPM_RELEASE_NUM := 1
REDHAT_SPEC := $(BUILD_DIR)/red-hat.spec

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

rpm-build:
rpm-build: build-linux
	rm -rf $(BUILD_DIR)
	mkdir -p $(BUILD_DIR)/
	mkdir -p $(BUILD_DIR)/rpm/SOURCES/usr/local/bin $(BUILD_DIR)/rpm/SOURCES/etc/systemd/system $(BUILD_DIR)/rpm/SOURCES/etc/ibmcloud
	cp install/cos-csi-mounter.service $(BUILD_DIR)/rpm/SOURCES/etc/systemd/system
	cp install/share.conf $(BUILD_DIR)/rpm/SOURCES/etc/ibmcloud
	cp ${BIN_DIR}/cos-csi-mounter-server $(BUILD_DIR)/rpm/SOURCES/usr/local/bin

	echo "Name: $(NAME)" > $(REDHAT_SPEC)
	echo "Version: $(APP_VERSION)" >> $(REDHAT_SPEC)
	echo "Release: $(RPM_RELEASE_NUM)" >> $(REDHAT_SPEC)
	echo "Summary: $(DESCRIPTION)" >> $(REDHAT_SPEC)
	echo "License: $(LICENSE)" >>  $(REDHAT_SPEC)
	echo "BuildArch: $(RPM_ARCH)" >>  $(REDHAT_SPEC)
	echo "Requires: mount.ibmshare = 0.1.1" >> $(REDHAT_SPEC)
	echo "%define _rpmfilename $(NAME)-$(APP_VERSION).rpm" >> $(REDHAT_SPEC)
	echo "%build" >> $(REDHAT_SPEC)

	echo "%install" >> $(REDHAT_SPEC)
	echo "rm -rf %{buildroot}" >> $(REDHAT_SPEC)
	echo "mkdir -p %{buildroot}/etc/systemd/system/" >> $(REDHAT_SPEC)
	echo "mkdir -p %{buildroot}/usr/local/bin/" >> $(REDHAT_SPEC)
	echo "mkdir -p %{buildroot}/etc/ibmcloud/" >> $(REDHAT_SPEC)
	echo "cp -r %{_sourcedir}/* %{buildroot}/" >> $(REDHAT_SPEC)

	echo "%description" >> $(REDHAT_SPEC)
	echo "This package provides the IBM Mount Share helper container service."

	echo "%files" >> $(REDHAT_SPEC)
	echo "/etc/systemd/system/cos-csi-mounter.service" >> $(REDHAT_SPEC)
	echo "/usr/local/bin/cos-csi-mounter-server" >> $(REDHAT_SPEC)
	echo "/etc/ibmcloud/share.conf" >> $(REDHAT_SPEC)

	echo "%post" >> $(REDHAT_SPEC)
	echo "systemctl enable cos-csi-mounter.service" >> $(REDHAT_SPEC)
	echo "systemctl start cos-csi-mounter.service" >> $(REDHAT_SPEC)

	echo "%preun" >> $(REDHAT_SPEC)
	echo "systemctl stop cos-csi-mounter.service || true" >> $(REDHAT_SPEC)
	echo "systemctl disable cos-csi-mounter.service || true" >> $(REDHAT_SPEC)

	rpmbuild -ba --build-in-place --define "_topdir $(PWD)/$(BUILD_DIR)/rpm" $(REDHAT_SPEC)
	cp  $(BUILD_DIR)/rpm/RPMS/* ./
	rm -rf $(BUILD_DIR)

tar-package:
tar-package:
	mkdir packages
	cp ${NAME}-* packages/
	cd packages && \
		tar -czvf ../$(INSTALL_TAR_FILE_DEB) *.deb && \
		tar -czvf ../$(INSTALL_TAR_FILE_RPM) *.rpm
	sha256sum $(INSTALL_TAR_FILE_DEB) > $(CHECKSUM_FILE_DEB)
	sha256sum $(INSTALL_TAR_FILE_RPM) > $(CHECKSUM_FILE_RPM)
	@printf "Production - Install package created ok: $(INSTALL_TAR_FILE_DEB) and $(INSTALL_TAR_FILE_RPM)\n"

clean:
	rm -rf ${BIN_DIR}
	rm *.deb *.rpm
	rm -rf packages

packages:
packages: deb-build rpm-build tar-package clean