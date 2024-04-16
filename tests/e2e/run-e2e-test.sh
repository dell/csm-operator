# Copyright © 2022-2023 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source ./env-e2e-test.sh

if [ -z "${VALUES_FILE-}" ]; then
    echo "!!!! FATAL: VALUES_FILE was not provided"
    exit 1
fi
export E2E_VALUES_FILE=$VALUES_FILE

export GO111MODULE=on
export ACK_GINKGO_RC=true

if ! (go mod vendor && go get github.com/onsi/ginkgo/v2); then
    echo "go mod vendor or go get ginkgo error"
    exit 1
fi

# copy cert-csi binary into local folder
cp $CERT_CSI .

# Uncomment for authorization proxy server
#cp $KARAVICTL /usr/local/bin/

# Copy certificates for Observability tests
#cp ../../samples/observability/selfsigned-cert.yaml ./testfiles/observability-cert.yaml

PATH=$PATH:$(go env GOPATH)/bin

OPTS=()

if [ -z "${GINKGO_OPTS-}" ]; then
    OPTS=(-v)
else
    read -ra OPTS <<<"-v $GINKGO_OPTS"
fi

pwd
ginkgo -mod=mod "${OPTS[@]}"

rm -f cert-csi

# Uncomment for authorization proxy server
#rm -f /usr/local/bin/karavictl

# Checking for test status
TEST_PASS=$?
if [[ $TEST_PASS -ne 0 ]]; then
    exit 1
fi
