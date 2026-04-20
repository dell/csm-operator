# Copyright © 2022-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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
export ARRAY_INFO_FILE=array-info.yaml
export GO111MODULE=on
export ACK_GINKGO_RC=true
export PROG="${0}"
export GINKGO_OPTS="--timeout 5h"
export E2E_VERBOSE=false
export CHECK_PREREQUISITES_ONLY=false

# Start with all modules false, they can be enabled by command line arguments
export AUTHORIZATION=false
export AUTHORIZATIONPROXYSERVER=false
export REPLICATION=false
export OBSERVABILITY=false
export RESILIENCY=false
export ZONING=false
export SFTP=false

export INSTALL_VAULT=false
export INSTALL_CONJUR=false
export CLEANUP_NS=true

export PROXY_HOST="csm-authorization.com"

# Namespaces are built dynamically based on selected platforms and modules.
# Namespace names use a configurable prefix (NS_PREFIX from array-info.yaml,
# default "e2e"). The operator namespace is the prefix itself; all others
# are ${NS_PREFIX}-<suffix>.
E2E_NAMESPACES=()

set -o errexit
set -o pipefail

PATH=$PATH:$(go env GOPATH)/bin

###############################################################################
# Function definitions
###############################################################################
function getArrayInfo() {
  local platforms=""
  [[ "${POWERFLEX:-}" == "true" ]]   && platforms="${platforms:+$platforms,}powerflex"
  [[ "${POWERSCALE:-}" == "true" ]]  && platforms="${platforms:+$platforms,}powerscale"
  [[ "${POWERMAX:-}" == "true" ]]    && platforms="${platforms:+$platforms,}powermax"
  [[ "${POWERSTORE:-}" == "true" ]]  && platforms="${platforms:+$platforms,}powerstore"
  [[ "${UNITY:-}" == "true" ]]       && platforms="${platforms:+$platforms,}unity"
  [[ "${COSI:-}" == "true" ]]        && platforms="${platforms:+$platforms,}cosi"

  # No specific platform selected (e.g. --sanity) -> load all
  if [[ -z "$platforms" ]]; then
    platforms="powerflex,powerscale,powermax,powerstore,unity"
  fi

  # Build active features from module flags.
  # "auth-common" is tied to auth (Redis/JWT credentials shared across platforms).
  # --no-modules disables all features, so only base platform sections load.
  local features=""
  if [[ "${NOMODULES:-}" != "true" ]]; then
    # Default: load auth and auth-common unless explicitly running --no-modules
    features="auth,auth-common"
    [[ "${ZONING:-}" == "true" ]]       && features="${features},zoning"
    [[ "${REPLICATION:-}" == "true" ]]  && features="${features},replication"
  fi
  # check for sftp feature for powerflex driver
  [[ "${POWERFLEX:-}" == "true" ]] && [[ "${SFTP:-}" == "true" ]] && features="${features},sftp"

  cd ./scripts/parse-array-info
  if ! output=$(go run main.go \
    -platforms "$platforms" \
    -features "$features" \
    -file "../../$ARRAY_INFO_FILE" 2>&1); then
    echo "Error: parse-array-info failed"
    echo "$output"
    exit 1
  fi

  eval "$output"
  cd ../..
}

function installSecretsStoreCSIDriver() {
  # Check for the actual CSIDriver registration, not just CRDs.
  # CRDs can survive a helm uninstall while the driver pods are gone.
  if kubectl get csidriver secrets-store.csi.k8s.io &>/dev/null; then
    echo "secrets-store-csi-driver is already running, skipping."
    return
  fi
  # Remove stale helm release if CRDs were deleted but release remains
  helm uninstall csi-secrets-store -n kube-system 2>/dev/null || true
  echo "Installing secrets-store-csi-driver..."
  helm repo add secrets-store-csi-driver https://kubernetes-sigs.github.io/secrets-store-csi-driver/charts
  helm install csi-secrets-store \
    secrets-store-csi-driver/secrets-store-csi-driver \
    --wait \
    --namespace kube-system \
    --set 'enableSecretRotation=true' \
    --set 'syncSecret.enabled=true' \
    --set 'tokenRequests[0].audience=conjur'
}

function vaultSetupAutomation() {
  echo "Removing any existing vault installation..."
  helm delete vault0 || true
  echo "Installing vault with all secrets for Authorization tests..."
  cd ./scripts/vault-automation
  go run main.go --kubeconfig "$KUBECONFIG" --name vault0 --env-config --secrets-store-csi-driver=true --csm-authorization-namespace "$E2E_NS_AUTH"
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
  # Install the Ginkgo v2 CLI matching the library version to ensure
  # verbose output (By() step annotations) is streamed correctly.
  if ! go install github.com/onsi/ginkgo/v2/ginkgo; then
    echo "Failed to install ginkgo v2 CLI"
    exit 1
  fi
}

function runTests() {
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

function resolveKubeconfig() {
  local default_kubeconfig
  default_kubeconfig="$HOME/.kube/config"

  if [ -n "${KUBECONFIG-}" ]; then
    export KUBECONFIG
  else
    export KUBECONFIG="$default_kubeconfig"
  fi
}

function getMasterNodeIP() {
  export CLUSTER_IP=$(grep server "$KUBECONFIG" | awk '{print $2}' | sed -E "s|https?://([^:/]+).*|\1|")
  if [ "$IS_OPENSHIFT" == "true" ]; then
    if which nslookup &> /dev/null; then
      export CLUSTER_IP=$(nslookup $CLUSTER_IP | awk '/^Address: / { print $2 }')
    else
      echo "nslookup not found, won't resolve cluster IP"
    fi
  fi
  echo "Cluster IP: $CLUSTER_IP"
}

function removeAuthCRDs() {
  local ns="$1"
  # Authorization CRDs have finalizers that block namespace deletion.
  # Remove the resources (and strip finalizers) before deleting the namespace.
  for crd in storage.csm-authorization.storage.dell.com csmrole csmtenant; do
    local items
    items=$(kubectl get "$crd" -n "$ns" -o name 2>/dev/null) || true
    for item in $items; do
      echo "  Removing finalizers from $item in $ns"
      kubectl patch "$item" -n "$ns" --type merge -p '{"metadata":{"finalizers":null}}' 2>/dev/null || true
      kubectl delete "$item" -n "$ns" --wait=false 2>/dev/null || true
    done
  done
  # Also delete any CSM resources to avoid other finalizer hangs
  kubectl delete csm --all -n "$ns" --wait=false 2>/dev/null || true
}

function setupNamespaces() {
  echo "Setting up clean e2e test namespaces..."
  for ns in "${E2E_NAMESPACES[@]}"; do
    if kubectl get namespace "$ns" &>/dev/null; then
      echo "Deleting existing namespace: $ns"
      removeAuthCRDs "$ns"
      kubectl delete namespace "$ns" --wait=true
    fi
    echo "Creating namespace: $ns"
    kubectl create namespace "$ns"
  done
  echo "All e2e test namespaces created successfully."
}

function cleanupSecretsStoreCRDs() {
  local crds
  crds=$(kubectl get crd -o name 2>/dev/null | grep secrets-store.csi.x-k8s.io || true)
  if [[ -n "$crds" ]]; then
    echo "Removing secrets-store-csi-driver CRDs..."
    echo "$crds" | xargs kubectl delete 2>/dev/null || true
  fi
}

function cleanupNamespaces() {
  echo "Cleaning up e2e test namespaces..."
  for ns in "${E2E_NAMESPACES[@]}"; do
    if kubectl get namespace "$ns" &>/dev/null; then
      echo "Deleting namespace: $ns"
      removeAuthCRDs "$ns"
      kubectl delete namespace "$ns" --wait=true --timeout=60s || true
    fi
  done
  # Clean up vault and secrets-store helm releases
  echo "Cleaning up vault and secrets-store installations..."
  helm uninstall vault0 -n default 2>/dev/null || true
  helm uninstall csi-secrets-store -n kube-system 2>/dev/null || true
  cleanupSecretsStoreCRDs
  echo "Namespace cleanup complete."
}

function onExit() {
  local exit_code=$?
  if [[ ${#E2E_NAMESPACES[@]} -eq 0 ]]; then
    exit $exit_code
  fi
  if [[ $CLEANUP_NS == "true" ]]; then
    cleanupNamespaces
  else
    echo "Skipping namespace cleanup (--no-cleanup-ns specified)."
  fi
  exit $exit_code
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
  echo "  -c                                           check for pre-requisites only"
  echo "  -v                                           enable verbose logging"
  echo "  --dellctl=<path to dellctl binary>           use to specify dellctl binary, if not in PATH"
  echo "  --kube-cfg=<path to kubeconfig file>         kubeconfig precedence: --kube-cfg, then KUBECONFIG env, then \$HOME/.kube/config"
  echo "  --scenarios=<path to custom scenarios file>  use to specify custom test scenarios file"
  echo "  --sanity                                     use to run e2e sanity suite"
  echo "  --auth                                       use to run e2e authorization suite"
  echo "  --replication                                use to run e2e replication suite"
  echo "  --obs                                        use to run e2e observability suite"
  echo "  --auth-proxy                                 use to run e2e auth-proxy suite"
  echo "  --resiliency                                 use to run e2e resiliency suite"
  echo "  --no-modules                                 use to run e2e suite without any modules"
  echo "  --cosi                                       use to run e2e cosi suite"
  echo "  --powerflex                                   use to run e2e powerflex suite"
  echo "  --powerscale                                  use to run e2e powerscale suite"
  echo "  --powerstore                                  use to run e2e powerstore suite"
  echo "  --unity                                      use to run e2e unity suite"
  echo "  --powermax                                    use to run e2e powermax suite"
  echo "  --zoning                                     use to run powerflex zoning tests (requires multiple storage systems)"
  echo "  --sftp                                       use to enable SFTP for PowerFlex tests"
  echo "  --minimal                                    use minimal testfiles scenarios"
  echo "  --install-vault                              force vault install (auto-installed when auth scenarios will run)"
  echo "  --install-conjur                             use to install authorization conjur instance with secrets for authorization tests"
  echo "  --add-tag=<scenario tag>                     use to specify scenarios to run by one of their tags"
  echo "  --no-cleanup-ns                              skip namespace deletion at the end of the test run"
  echo "  --continue-on-fail                           continue running scenarios after a failure (default: stop on first failure)"
  echo "  --junit-report=<path>                        write JUnit XML report to the given file path"
  echo
  echo "Examples:"
  echo "  Platform flags select drivers; module flags select features."
  echo "  When both are given, only scenarios matching a platform AND a module run."
  echo "  When no flags are given, all scenarios for all platforms and modules run."
  echo
  echo "  # All scenarios for all platforms and modules (no flags = run everything)"
  echo "  $PROG"
  echo
  echo "  # One platform, all modules (standalone + auth + obs + resiliency + replication)"
  echo "  $PROG --powerstore"
  echo
  echo "  # One platform, no modules (driver-only scenarios)"
  echo "  $PROG --powerstore --no-modules"
  echo
  echo "  # One platform, one module (only powerstore auth-proxy scenarios)"
  echo "  $PROG --powerstore --auth-proxy"
  echo
  echo "  # One module across all platforms (observability for every driver)"
  echo "  $PROG --obs"
  echo
  echo "  # Multiple platforms, one module"
  echo "  $PROG --powerstore --powermax --resiliency"
  echo
  echo "  # Sanity subset of one platform"
  echo "  $PROG --powermax --sanity"
  echo
  echo "  # Minimal scenarios — same filtering rules apply"
  echo "  $PROG --minimal --powerstore                  # all minimal powerstore scenarios"
  echo "  $PROG --minimal --powerstore --resiliency     # only minimal powerstore resiliency"
  echo "  $PROG --minimal --auth-proxy              # minimal auth-proxy for all platforms"
  echo

  exit 0
}

###############################################################################
# Parse command-line options
###############################################################################
while getopts ":hcv-:" optchar; do
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
    powerflex)
      export POWERFLEX=true ;;
    no-modules)
      export NOMODULES=true
      export AUTHORIZATION=false
      export AUTHORIZATIONPROXYSERVER=false
      export REPLICATION=false
      export OBSERVABILITY=false
      export RESILIENCY=false
      ;;
    cosi)
      export COSI=true ;;
    powerscale)
      export POWERSCALE=true ;;
    powerstore)
      export POWERSTORE=true ;;
    unity)
      export UNITY=true ;;
    powermax)
      export POWERMAX=true ;;
    zoning)
      export ZONING=true ;;
    sftp)
      export SFTP=true ;;
    kube-cfg)
      export KUBECONFIG="${!OPTIND}"
      OPTIND=$((OPTIND + 1))
      ;;
    kube-cfg=*)
      export KUBECONFIG=${OPTARG#*=}
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
    no-cleanup-ns)
      export CLEANUP_NS=false
      ;;
    continue-on-fail)
      export E2E_CONTINUE_ON_FAILURE=true
      ;;
    junit-report=*)
      export E2E_JUNIT_REPORT=${OPTARG#*=}
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
  c)
    export CHECK_PREREQUISITES_ONLY=true
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
trap onExit EXIT
if kubectl get crd | grep securitycontextconstraints.security.openshift.io &>/dev/null; then
  export IS_OPENSHIFT=true
else
  export IS_OPENSHIFT=false
fi
echo "IS_OPENSHIFT: $IS_OPENSHIFT"

resolveKubeconfig
getMasterNodeIP

getArrayInfo
checkForScenariosFile

# Compute namespace env vars from NS_PREFIX (set by array-info.yaml, default "e2e")
export NS_PREFIX="${NS_PREFIX:-e2e}"
export E2E_NS_OPERATOR="${NS_PREFIX}"
export E2E_NS_POWERFLEX="${NS_PREFIX}-powerflex"
export E2E_NS_POWERSCALE="${NS_PREFIX}-powerscale"
export E2E_NS_POWERMAX="${NS_PREFIX}-powermax"
export E2E_NS_POWERSTORE="${NS_PREFIX}-powerstore"
export E2E_NS_UNITY="${NS_PREFIX}-unity"
export E2E_NS_COSI="${NS_PREFIX}-cosi"
export E2E_NS_AUTH="${NS_PREFIX}-authorization"
export E2E_NS_PROXY="${NS_PREFIX}-proxy-ns"

# Build namespace list based on selected platforms and modules.
# When no platform or module flags are set, treat as "run everything".
ANY_FLAG_SET=false
for _v in POWERFLEX POWERSCALE POWERMAX POWERSTORE UNITY COSI \
          AUTHORIZATION AUTHORIZATIONPROXYSERVER REPLICATION OBSERVABILITY \
          RESILIENCY SANITY ZONING; do
  if [[ "${!_v:-}" == "true" ]]; then
    ANY_FLAG_SET=true
    break
  fi
done

E2E_NAMESPACES+=("$E2E_NS_OPERATOR")

if [[ "$ANY_FLAG_SET" == "false" ]]; then
  echo "No platform or module flags specified — running everything."
  E2E_NAMESPACES+=("$E2E_NS_POWERFLEX" "$E2E_NS_POWERSCALE" "$E2E_NS_POWERMAX" "$E2E_NS_POWERSTORE" "$E2E_NS_UNITY")
  if [[ "${NOMODULES:-}" != "true" ]]; then
    E2E_NAMESPACES+=("$E2E_NS_AUTH" "$E2E_NS_PROXY")
    AUTH_WILL_RUN=true
  else
    AUTH_WILL_RUN=false
  fi
else
  [[ "${POWERFLEX:-}" == "true" ]]              && E2E_NAMESPACES+=("$E2E_NS_POWERFLEX")
  [[ "${POWERSCALE:-}" == "true" ]]             && E2E_NAMESPACES+=("$E2E_NS_POWERSCALE")
  [[ "${POWERMAX:-}" == "true" ]]               && E2E_NAMESPACES+=("$E2E_NS_POWERMAX")
  [[ "${POWERSTORE:-}" == "true" ]]             && E2E_NAMESPACES+=("$E2E_NS_POWERSTORE")
  [[ "${UNITY:-}" == "true" ]]                  && E2E_NAMESPACES+=("$E2E_NS_UNITY")
  [[ "${COSI:-}" == "true" ]]                   && E2E_NAMESPACES+=("$E2E_NS_COSI")

  # Auth namespaces and vault are needed when auth scenarios will actually run:
  #   1. Auth flags explicitly set (--auth / --auth-proxy), OR
  #   2. No module flags at all (and not --no-modules): all modules run for the
  #      selected platform(s), which includes auth for powerflex/powerscale/powerstore/powermax.
  # If a non-auth module flag is set (e.g. --sanity, --obs), auth scenarios won't
  # match in ContainsTag so we skip the expensive vault/ingress setup.
  AUTH_WILL_RUN=false
  if [[ "${AUTHORIZATION:-}" == "true" || "${AUTHORIZATIONPROXYSERVER:-}" == "true" ]]; then
    AUTH_WILL_RUN=true
  elif [[ "${NOMODULES:-}" != "true" ]]; then
    # Check if any non-auth module flag is set.
    OTHER_MODULE_SET=false
    for flag in OBSERVABILITY REPLICATION RESILIENCY SANITY ZONING; do
      [[ "${!flag:-}" == "true" ]] && OTHER_MODULE_SET=true && break
    done
    if [[ "$OTHER_MODULE_SET" != "true" ]]; then
      # No module specified at all => all modules (including auth) run.
      # Platforms with auth scenarios: powerflex, powerscale, powerstore, powermax.
      if [[ "${POWERFLEX:-}" == "true" || "${POWERSCALE:-}" == "true" || "${POWERSTORE:-}" == "true" || "${POWERMAX:-}" == "true" ]]; then
        AUTH_WILL_RUN=true
      fi
    fi
  fi

  if [[ "$AUTH_WILL_RUN" == "true" ]]; then
    E2E_NAMESPACES+=("$E2E_NS_AUTH" "$E2E_NS_PROXY")
  fi
fi

setupNamespaces

# Install Gateway API and NGINX Gateway Fabric CRDs (prerequisite for auth v2.5.0+)
if [[ "$AUTH_WILL_RUN" == "true" ]] && [ "$IS_OPENSHIFT" != "true" ]; then
  echo "Installing Gateway API CRDs..."
  kubectl apply --server-side -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.5.1/standard-install.yaml
  echo "Installing NGINX Gateway Fabric CRDs..."
  kubectl apply --server-side -f https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/v2.4.2/deploy/crds.yaml
fi

# Install secrets-store-csi-driver CRDs if needed for authorization tests.
if [[ "$AUTH_WILL_RUN" == "true" || "$INSTALL_VAULT" == "true" || "$INSTALL_CONJUR" == "true" ]]; then
  installSecretsStoreCSIDriver
fi

# Auto-install vault when auth scenarios will run (unless explicitly skipped).
if [[ "$AUTH_WILL_RUN" == "true" || "$INSTALL_VAULT" == "true" ]]; then
  vaultSetupAutomation
fi
if [[ $INSTALL_CONJUR == "true" ]]; then
  conjurSetupAutomation
fi
if [[ "$AUTH_WILL_RUN" == "true" || "${AUTHORIZATIONPROXYSERVER:-}" == "true" ]]; then
  echo "Checking for dellctl - authorization tests will run"
  checkForDellctl

  echo "Authorization proxy host: $PROXY_HOST"
  export entryExists=$(cat /etc/hosts | grep $PROXY_HOST | wc -l)
  if [[ $entryExists != 1 ]]; then
      echo "Adding authorization host to /etc/hosts file"
      echo $CLUSTER_IP $PROXY_HOST | sudo tee -a /etc/hosts > /dev/null
  fi
fi

checkForGinkgo

# Ensure the baseline csm-images ConfigMap is present so that nightly image
# overrides are available for the latest version. This self-heals from a
# previous run that may have failed before its "Restore ConfigMap" cleanup step.
echo "Applying baseline csm-images ConfigMap..."
kubectl apply -f testfiles/authorization-templates/csm-images-baseline.yaml

if [[ $CHECK_PREREQUISITES_ONLY == "true" ]]; then
  echo "Skipping tests because check prerequisites only was requested."
  exit 0
fi

runTests
