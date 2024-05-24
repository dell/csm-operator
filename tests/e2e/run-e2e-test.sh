# Copyright © 2022-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
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

###############################################################################
# Set environment variables and options
###############################################################################
export E2E_SCENARIOS_FILE=testfiles/scenarios.yaml
export ARRAY_INFO_FILE=array-info.sh
export GO111MODULE=on
export ACK_GINKGO_RC=true
export PROG="${0}"

set -o errexit
set -o nounset
set -o pipefail

PATH=$PATH:$(go env GOPATH)/bin

###############################################################################
# Function definitions
###############################################################################
function getArrayInfo() {
  source ./$ARRAY_INFO_FILE
}

function checkForScenariosFile() {
  if [ -v SCENARIOS ]; then
    export E2E_SCENARIOS_FILE=$SCENARIOS
  fi

  stat $E2E_SCENARIOS_FILE >&/dev/null || {
    echo "error: $E2E_SCENARIOS_FILE is not a valid scenario file - exiting"
    exit 1
  }
}

function checkForCertCsi() {
  if [ -v CERT_CSI ]; then
    stat $CERT_CSI >&/dev/null || {
      echo "error: $CERT_CSI is not a valid cert-csi binary - exiting"
      exit 1
    }
    cp $CERT_CSI /usr/local/bin/
  fi

  cert-csi --help >&/dev/null || {
    echo "error: cert-csi required but not available - exiting"
    exit 1
  }
}

function checkForKaravictl() {
  if [ -v KARAVICTL ]; then
    stat $KARAVICTL >&/dev/null || {
      echo "$KARAVICTL is not a valid karavictl binary - exiting"
      exit 1
    }
    cp $KARAVICTL /usr/local/bin/
  fi

  karavictl --help >&/dev/null || {
    echo "error:karavictl required but not available - exiting"
    exit 1
  }
}

function checkForDellctl() {
  if [ -v DELLCTL ]; then
    stat $DELLCTL >&/dev/null || {
      echo "error: $DELLCTL is not a valid path for dellctl - exiting"
      exit 1
    }
    cp $DELLCTL /usr/local/bin/
  fi

  dellctl --help >&/dev/null || {
    echo "error: dellctl required but not available - exiting"
    exit 1
  }
}

function checkForGinkgo() {
  if ! (go mod vendor && go get github.com/onsi/ginkgo/v2); then
    echo "go mod vendor or go get ginkgo error"
    exit 1
  fi
}

function runTests() {
  ginkgo -mod=mod -v

  # Checking for test status
  TEST_PASS=$?
  if [[ $TEST_PASS -ne 0 ]]; then
    exit 1
  fi
}

function usage() {
  echo
  echo "Help for $PROG"
  echo
  echo "This script runs the E2E tests for the csm-operator. You can specify different test suites with flags such as '--sanity' or '--powerflex'. Please see readme for more information"
  echo
  echo "Usage: $PROG options..."
  echo "Options:"
  echo "  Optional"
  echo "  -h                                           print out helptext"
  echo "  --cert-csi=<path to cert-csi binary>         use to specify cert-csi binary, if not in PATH"
  echo "  --karavictl=<path to karavictl binary>       use to specify karavictl binary, if not in PATH"
  echo "  --dellctl=<path to dellctl binary>           use to specify dellctl binary, if not in PATH"
  echo "  --kube-cfg=<path to kubeconfig file>         use to specify non-default kubeconfig file"
  echo "  --scenarios=<path to custom scenarios file>  use to specify custom test scenarios file"
  echo "  --sanity                                     use to run e2e sanity suite"
  echo "  --auth                                       use to run e2e authorization suite"
  echo "  --replication                                use to run e2e replication suite"
  echo "  --obs                                        use to run e2e observability suite"
  echo "  --auth-proxy                                 use to run e2e auth-proxy suite"
  echo "  --resiliency                                 use to run e2e resiliency suite"
  echo "  --app-mobility                               use to run e2e application-mobility suite"
  echo "  --pflex                                      use to run e2e powerflex suite"
  echo "  --pscale                                     use to run e2e powerscale suite"
  echo "  --pstore                                     use to run e2e powerstore suite"
  echo "  --unity                                      use to run e2e unity suite"
  echo "  --pmax                                       use to run e2e powermax suite"
  echo

  exit 0
}

###############################################################################
# Parse command-line options
###############################################################################
while getopts ":h-:" optchar; do
  case "${optchar}" in
  -)
    case "${OPTARG}" in
    sanity)
      export SANITY=true ;;
    auth)
      export AUTHORIZATION=true ;;
    replication)
      export REPLICATION=true ;;
    obs)
      export OBSERVABILITY=true ;;
    auth-proxy)
      export AUTHORIZATIONPROXYSERVER=true ;;
    resiliency)
      export RESILIENCY=true ;;
    app-mobility)
      export APPLICATIONMOBILITY=true ;;
    pflex)
      export POWERFLEX=true ;;
    pscale)
      export POWERSCALE=true ;;
    pstore)
      export POWERSTORE=true ;;
    unity)
      export UNITY=true ;;
    pmax)
      export POWERMAX=true ;;
    cert-csi)
      CERT_CSI="${!OPTIND}"
      OPTIND=$((OPTIND + 1))
      ;;
    cert-csi=*)
      CERT_CSI=${OPTARG#*=}
      ;;
    kube-cfg)
      KUBECONFIG="${!OPTIND}"
      OPTIND=$((OPTIND + 1))
      ;;
    kube-cfg=*)
      KUBECONFIG=${OPTARG#*=}
      ;;
    karavictl)
      KARAVICTL="${!OPTIND}"
      OPTIND=$((OPTIND + 1))
      ;;
    karavictl=*)
      KARAVICTL=${OPTARG#*=}
      ;;
    dellctl)
      DELLCTL="${!OPTIND}"
      OPTIND=$((OPTIND + 1))
      ;;
    dellctl=*)
      DELLCTL=${OPTARG#*=}
      ;;
    scenarios)
      SCENARIOS="${!OPTIND}"
      OPTIND=$((OPTIND + 1))
      ;;
    scenarios=*)
      SCENARIOS=${OPTARG#*=}
      ;;
    *)
      echo "Unknown option -${OPTARG}"
      echo "For help, run $PROG -h"
      exit 1
      ;;
    esac
    ;;
  h)
    usage
    ;;
  *)
    echo "Unknown option -${OPTARG}"
    echo "For help, run $PROG -h"
    exit 1
    ;;
  esac
done

###############################################################################
# Check pre-requisites and run tests
###############################################################################
getArrayInfo
checkForScenariosFile
checkForCertCsi
checkForKaravictl
if [ -v APPLICATIONMOBILITY ]; then
  checkForDellctl
fi
checkForGinkgo
runTests

