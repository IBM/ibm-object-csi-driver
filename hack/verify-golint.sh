#!/bin/bash

#/******************************************************************************
 #Copyright 2023 IBM Corp.
 # Licensed under the Apache License, Version 2.0 (the "License");
 # you may not use this file except in compliance with the License.
 # You may obtain a copy of the License at
 #
 #     http://www.apache.org/licenses/LICENSE-2.0
 #
 # Unless required by applicable law or agreed to in writing, software
 # distributed under the License is distributed on an "AS IS" BASIS,
 # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 # See the License for the specific language governing permissions and
 # limitations under the License.
# *****************************************************************************/

set -euo pipefail

if [[ -z "$(command -v golangci-lint)" ]]; then
  echo "Cannot find golangci-lint. Installing golangci-lint..."
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)"/bin v2.0.2
  PATH=$PATH:$(go env GOPATH)/bin
  export PATH=$PATH
fi

echo "Verifying golint"

golangci-lint run --timeout=10m -v

echo "Congratulations! Lint check completed for all Go source files."
