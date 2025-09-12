FROM golang:1.25.1

WORKDIR /go/src/github.com/IBM/ibm-object-csi-driver
ADD . /go/src/github.com/IBM/ibm-object-csi-driver

CMD ["./scripts/build-bin.sh"]
