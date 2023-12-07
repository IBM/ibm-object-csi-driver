FROM registry.access.redhat.com/ubi8/ubi-minimal:8.6-941 as s3fs-builder

ARG   RHSM_PASS=blank
ARG   RHSM_USER=blank

ENV   RHSM_PASS "${RHSM_PASS}"
ENV   RHSM_USER "${RHSM_USER}"

ADD   register-sys.sh /usr/bin/
RUN   microdnf update --setopt=tsflags=nodocs && \
      microdnf install -y --nodocs hostname subscription-manager
RUN   hostname; chmod 755 /usr/bin/register-sys.sh &&  /usr/bin/register-sys.sh
RUN   microdnf update --setopt=tsflags=nodocs && \
      microdnf install -y --nodocs iputils nfs-utils rpcbind xfsprogs udev nc e2fsprogs e4fsprogs && \
      microdnf clean all -y
RUN   microdnf update --setopt=tsflags=nodocs && \
      microdnf install -y gcc libstdc++-devel \
      gcc-c++ fuse curl-devel \
      libxml2-devel openssl-devel mailcap \
      git automake make
RUN   microdnf -y install fuse-devel
RUN   rm /usr/bin/register-sys.sh && subscription-manager unregister && subscription-manager clean

RUN git clone https://github.com/s3fs-fuse/s3fs-fuse.git && cd s3fs-fuse && \
    git checkout v1.93 && \
    ./autogen.sh && ./configure --prefix=/usr/local --with-openssl && make && make install && \
    rm -rf /var/lib/apt/lists/*

FROM registry.access.redhat.com/ubi8/ubi as rclone-builder
RUN yum install wget git gcc -y

ENV ARCH=amd64
ENV GO_VERSION=1.19

RUN echo $ARCH $GO_VERSION

RUN wget -q https://dl.google.com/go/go$GO_VERSION.linux-$ARCH.tar.gz && \
  tar -xf go$GO_VERSION.linux-$ARCH.tar.gz && \
  rm go$GO_VERSION.linux-$ARCH.tar.gz && \
  mv go /usr/local

ENV GOROOT /usr/local/go
ENV GOPATH /go
ENV PATH=$GOPATH/bin:$GOROOT/bin:$PATH
ENV GOARCH $ARCH
ENV GO111MODULE=on

RUN git clone https://github.com/rclone/rclone.git && \
      cd rclone && git checkout tags/v1.64.0 && \
      go build && ./rclone version && \
      cp rclone /usr/local/bin/rclone

FROM registry.access.redhat.com/ubi8/ubi:latest

# Default values
ARG git_commit_id=unknown
ARG git_remote_url=unknown
ARG build_date=unknown

LABEL description="IBM CSI Object Storage Plugin"
LABEL build-date=${build_date}
LABEL git-commit-id=${git_commit_id}
RUN yum update -y && yum install fuse fuse-libs fuse3 fuse3-libs -y
COPY --from=s3fs-builder /usr/local/bin/s3fs /usr/bin/s3fs
COPY --from=rclone-builder /usr/local/bin/rclone /usr/bin/rclone
COPY ibm-object-csi-driver ibm-object-csi-driver
ENTRYPOINT ["/ibm-object-csi-driver"]
