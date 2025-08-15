# Copyright © 2022-2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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
export ARRAY_INFO_FILE=array-info.env
export GO111MODULE=on
export ACK_GINKGO_RC=true
export PROG="${0}"
export GINKGO_OPTS="--timeout 5h"
export E2E_VERBOSE=false

# Start with all modules false, they can be enabled by command line arguments
export AUTHORIZATION=false
export AUTHORIZATIONPROXYSERVER=false
export REPLICATION=false
export OBSERVABILITY=false
export RESILIENCY=false
export ZONING=false
export SHAREDNFS=false

export INSTALL_VAULT=false
export INSTALL_CONJUR=false

export PROXY_HOST="csm-authorization.com"

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

function vaultSetupAutomation() {
  echo "Removing any existing vault installation..."
  helm delete vault0 || true
  echo "Installing vault with all secrets for Authorization tests..."
  cd ./scripts/vault-automation
  go run main.go --kubeconfig ~/.kube/config --name vault0 --env-config --secrets-store-csi-driver=true
  cd ../..
}

function conjurSetupAutomation() {
  echo "Removing any existing conjur installation..."
  helm delete conjur || true
  helm delete conjur-csi-provider || true
  echo "Installing conjur with all secrets for Authorization tests..."
  cd ./scripts/conjur-automation
  ./conjur.sh --control-node $CLUSTER_IP --env-config
  mv -f conjur-spc.yaml ../../testfiles/authorization-templates/storage_csm_authorization_secret_provider_class_conjur.yaml
  cd ../..
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

function checkForDellctl() {
  if [ -v DELLCTL ]; then
    # Check if the file exists and is not the same as the destination
    if [ "$DELLCTL" != "/usr/local/bin/dellctl" ]; then
      stat "$DELLCTL" >&/dev/null || {
        echo "error: $DELLCTL is not a valid path for dellctl - exiting"
        exit 1
      }
      cp "$DELLCTL" /usr/local/bin/
    fi
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

  # Uncomment for authorization proxy server
  #cp $DELLCTL /usr/local/bin/

  PATH=$PATH:$(go env GOPATH)/bin

  OPTS=()

  if [ -z "${GINKGO_OPTS-}" ]; then
      OPTS=(-v)
  else
      read -ra OPTS <<<"-v $GINKGO_OPTS"
  fi

  pwd
  ginkgo -mod=mod "${OPTS[@]}"

  # Uncomment for authorization proxy server
  # rm -f /usr/local/bin/dellctl

  # Checking for test status
  TEST_PASS=$?
  if [[ $TEST_PASS -ne 0 ]]; then
    exit 1
  fi
}

function getMasterNodeIP() {
  export CLUSTER_IP=$(grep server ~/.kube/config | awk '{print $2}' | sed -E "s|https?://([^:/]+).*|\1|")
  if [[ $IS_OPENSHIFT == "true" ]]; then
    export CLUSTER_IP=$(nslookup $CLUSTER_IP | awk '/^Address: / { print $2 }')
  fi
  echo "Cluster IP: $CLUSTER_IP"
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
  echo "  -v                                           enable verbose logging"
  echo "  --dellctl=<path to dellctl binary>           use to specify dellctl binary, if not in PATH"
  echo "  --kube-cfg=<path to kubeconfig file>         use to specify non-default kubeconfig file"
  echo "  --scenarios=<path to custom scenarios file>  use to specify custom test scenarios file"
  echo "  --sanity                                     use to run e2e sanity suite"
  echo "  --auth                                       use to run e2e authorization suite"
  echo "  --replication                                use to run e2e replication suite"
  echo "  --obs                                        use to run e2e observability suite"
  echo "  --auth-proxy                                 use to run e2e auth-proxy suite"
  echo "  --resiliency                                 use to run e2e resiliency suite"
  echo "  --no-modules                                 use to run e2e suite without any modules"
  echo "  --pflex                                      use to run e2e powerflex suite"
  echo "  --pscale                                     use to run e2e powerscale suite"
  echo "  --pstore                                     use to run e2e powerstore suite"
  echo "  --unity                                      use to run e2e unity suite"
  echo "  --pmax                                       use to run e2e powermax suite"
  echo "  --zoning                                     use to run powerflex zoning tests (requires multiple storage systems)"
  echo "  --minimal                                    use minimal testfiles scenarios"
  echo "  --sharednfs                                  use to run e2e sharednfs suite (pre-requisite, the nodes need to have nfs-server setup)"
  echo "  --install-vault                              use to install authorization vault instance with secrets for authorization tests"
  echo "  --install-conjur                             use to install authorization conjur instance with secrets for authorization tests"
  echo "  --add-tag=<scenario tag>                     use to specify scenarios to run by one of their tags"
  echo

  exit 0
}

###############################################################################
# Parse command-line options
###############################################################################
while getopts ":hv-:" optchar; do
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
    pflex)
      export POWERFLEX=true ;;
    no-modules)
      export NOMODULES=true
      export AUTHORIZATION=false
      export AUTHORIZATIONPROXYSERVER=false
      export REPLICATION=false
      export OBSERVABILITY=false
      export RESILIENCY=false
      ;;
    pscale)
      export POWERSCALE=true ;;
    pstore)
      export POWERSTORE=true ;;
    unity)
      export UNITY=true ;;
    pmax)
      export POWERMAX=true ;;
    zoning)
      export ZONING=true ;;
    kube-cfg)
      KUBECONFIG="${!OPTIND}"
      OPTIND=$((OPTIND + 1))
      ;;
    kube-cfg=*)
      KUBECONFIG=${OPTARG#*=}
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
    install-vault)
      export INSTALL_VAULT=true
      ;;
    install-conjur)
      export INSTALL_CONJUR=true
      ;;
    add-tag=*)
      export ADD_SCENARIO_TAG=${OPTARG#*=}
      ;;
    minimal)
      export E2E_SCENARIOS_FILE=testfiles/minimal-testfiles/scenarios.yaml
      ;;
    sharednfs)
      export SHAREDNFS=true ;;
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
  v)
    E2E_VERBOSE=true
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
if kubectl get crd | grep securitycontextconstraints.security.openshift.io &>/dev/null; then
  export IS_OPENSHIFT=true
else
  export IS_OPENSHIFT=false
fi
echo "IS_OPENSHIFT: $IS_OPENSHIFT"

getMasterNodeIP

getArrayInfo
checkForScenariosFile
if [[ $INSTALL_VAULT == "true" ]]; then
  vaultSetupAutomation
fi
if [[ $INSTALL_CONJUR == "true" ]]; then
  conjurSetupAutomation
fi
if [[ $AUTHORIZATIONPROXYSERVER == "true" ]]; then
  echo "Checking for dellctl - AUTHORIZATIONPROXYSERVER"
  checkForDellctl

  echo "Authorization proxy host: $PROXY_HOST"
  export entryExists=$(cat /etc/hosts | grep $PROXY_HOST | wc -l)
  if [[ $entryExists != 1 ]]; then
      echo "Adding authorization host to /etc/hosts file"
      echo $CLUSTER_IP $PROXY_HOST >> /etc/hosts
  fi
fi

checkForGinkgo
# runTests
