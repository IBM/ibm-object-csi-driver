FROM golang:1.24.4

WORKDIR /go/src/github.com/IBM/ibm-object-csi-driver
ADD . /go/src/github.com/IBM/ibm-object-csi-driver

CMD ["./scripts/build-bin.sh"]
