FROM registry.access.redhat.com/ubi8/ubi
RUN yum install wget git gcc -y

ENV ARCH=amd64
ENV GO_VERSION=1.14.10

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
RUN cd rclone && go build && ./rclone version