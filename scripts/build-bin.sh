#!/bin/bash
# ******************************************************************************
# * Licensed Materials - Property of IBM
# * IBM Cloud Kubernetes Service, 5737-D43
# * (C) Copyright IBM Corp. 2023 All Rights Reserved.
# * US Government Users Restricted Rights - Use, duplication or
# * disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
# ******************************************************************************

set -e
set -x
cd /go/src/github.com/IBM/ibm-object-csi-driver
CGO_ENABLED=0 GOOS=linux go build -mod=mod -a -ldflags "-X main.version=${git_commit_id} -extldflags \"-static\"" -o /go/bin/ibm-object-csi-driver ./cmd/$*
