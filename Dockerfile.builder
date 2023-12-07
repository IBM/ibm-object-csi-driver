FROM golang:1.20.10

WORKDIR /go/src/github.com/IBM/satellite-object-storage-plugin
ADD . /go/src/github.com/IBM/satellite-object-storage-plugin

CMD ["./scripts/build-bin.sh"]
