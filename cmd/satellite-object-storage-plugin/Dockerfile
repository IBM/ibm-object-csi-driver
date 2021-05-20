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

FROM registry.access.redhat.com/ubi8/ubi:latest
LABEL description="Satellite Object Storage Plugin"
RUN yum update -y && yum install fuse fuse-libs -y
COPY --from=s3fs-builder /usr/local/bin/s3fs /usr/bin/s3fs
COPY ./bin/satellite-object-storage-plugin satellite-object-storage-plugin
ENTRYPOINT ["/satellite-object-storage-plugin"]