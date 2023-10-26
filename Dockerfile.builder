FROM golang:1.19.6

WORKDIR /go/src/github.com/IBM/satellite-object-storage-plugin
ADD . /go/src/github.com/IBM/satellite-object-storage-plugin

ARG TAG
ARG OS
ARG ARCH

CMD ["./scripts/build-bin.sh"]
