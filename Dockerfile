# Use an official Ubuntu image as the base image
FROM ubuntu:latest as s3fs-builder

# Set non-interactive mode to prevent prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install the required dependencies
RUN apt-get update
RUN apt-get -y install automake autotools-dev fuse g++ git libcurl4-openssl-dev libfuse-dev libssl-dev libxml2-dev make pkg-config

# Set the working directory in the container
WORKDIR /app

# Clone the S3FS repository (you may need to adjust the repository URL)
RUN git clone https://github.com/s3fs-fuse/s3fs-fuse.git .

# Build S3FS
RUN git checkout v1.91 && \
    ./autogen.sh && ./configure --prefix=/usr/local --with-openssl && make && make install
#RUN ./autogen.sh  && ./configure && make


# Clean up after the installation to reduce the image size
RUN apt-get clean && rm -rf /var/lib/apt/lists/* && rm -rf /app

#RUN apt-get update && apt-get install -y --no-install-recommends nfs-common && \
#   apt-get install -y udev && \		
#         apt-get install -y --no-install-recommends apt && \		
# 	apt-get install -y --no-install-recommends ca-certificates xfsprogs && \		
# 	apt-get upgrade -y && rm -rf /var/lib/apt/lists/*



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
LABEL description="IBM CSI Object Storage Plugin"
# Default values
ARG git_commit_id=unknown
ARG git_remote_url=unknown
ARG build_date=unknown
ARG jenkins_build_number=unknown
ARG REPO_SOURCE_URL=blank
ARG BUILD_URL=blank

# Add Labels to image to show build details
LABEL git-commit-id=${git_commit_id}
LABEL git-remote-url=${git_remote_url}
LABEL build-date=${build_date}
LABEL jenkins-build-number=${jenkins_build_number}
LABEL razee.io/source-url="${REPO_SOURCE_URL}"
LABEL razee.io/build-url="${BUILD_URL}"




RUN yum update -y && yum install fuse fuse-libs fuse3 fuse3-libs -y
COPY --from=s3fs-builder /usr/local/bin/s3fs /usr/bin/s3fs
COPY --from=rclone-builder /usr/local/bin/rclone /usr/bin/rclone
COPY ./bin/satellite-object-storage-plugin satellite-object-storage-plugin


RUN mkdir -p /home/ibm-csi-drivers/
ADD ibm-object-csi-driver /home/ibm-csi-drivers
RUN chmod +x /home/ibm-csi-drivers/ibm-object-csi-driver

ENTRYPOINT ["/home/ibm-csi-drivers/ibm-object-csi-driver"]
