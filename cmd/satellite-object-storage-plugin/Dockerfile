FROM debian:stretch as s3fs-builder

# s3fs and some other dependencies
RUN apt-get update -y && apt-get upgrade -y && apt-get install -y \
libfuse2 gcc sqlite3 libsqlite3-dev \
psmisc procps libcurl3 xfsprogs curl unzip \
libfuse-dev fuse libssl-dev git autotools-dev automake libc6-dev build-essential libcurl4-openssl-dev libxml2-dev pkg-config && \
git clone https://github.com/s3fs-fuse/s3fs-fuse.git && cd s3fs-fuse && \
git checkout v1.89 && \
./autogen.sh && ./configure && make && make install && \
rm -rf /var/lib/apt/lists/*

FROM registry.access.redhat.com/ubi8/ubi as rclone-builder
RUN yum install wget git gcc -y

ENV ARCH=amd64
ENV GO_VERSION=1.18.3

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

RUN git clone https://github.com/rclone/rclone.git
RUN cd rclone && go build && ./rclone version && cp rclone /usr/local/bin/rclone

FROM registry.access.redhat.com/ubi8/ubi:latest
LABEL description="Satellite Object Storage Plugin"
RUN yum update -y && yum install fuse fuse-libs -y
COPY --from=s3fs-builder /usr/local/bin/s3fs /usr/bin/s3fs
COPY --from=rclone-builder /usr/local/bin/rclone /usr/bin/rclone
COPY ./bin/satellite-object-storage-plugin satellite-object-storage-plugin
ENTRYPOINT ["/satellite-object-storage-plugin"]
