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
export CSIADDONS=false
export ZONING=false
export SFTP=false

export INSTALL_VAULT=false
export INSTALL_CONJUR=false
export CLEANUP_NS=true

export PROXY_HOST="csm-authorization.com"

# mTLS configuration for PowerScale tests
# Set this to the SmartConnect zone FQDN of your PowerScale array
# If not set, mTLS tests will skip (tlshd daemon check will fail)
export POWERSCALE_MTLS_FQDN=""

# When enabled, mTLS prerequisite checks use the local host instead of SSHing
# to every Kubernetes node. Use only when the E2E tests run on the worker node.
export LOCAL_NODE_PREREQS=false

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
    # Check if zoning is configured in array-info.yaml for PowerFlex
    if [[ "${POWERFLEX:-}" == "true" ]] && grep -q "POWERFLEX_ZONING_USER.*[^\" ]" "$ARRAY_INFO_FILE" 2>/dev/null; then
      features="${features},zoning"
    elif [[ "${ZONING:-}" == "true" ]]; then
      # Fallback to --zoning flag for backward compatibility
      features="${features},zoning"
    fi
    # Check if replication is configured in array-info.yaml
    if grep -q "REPLICATION.*true\|replication.*true" "$ARRAY_INFO_FILE" 2>/dev/null; then
      features="${features},replication"
    elif [[ "${REPLICATION:-}" == "true" ]]; then
      # Fallback to --replication flag for backward compatibility
      features="${features},replication"
    fi
  fi
  # Include metro feature for powerstore driver if configured in array-info.yaml
  if [[ "${POWERSTORE:-}" == "true" ]]; then
    if grep -q "^powerstore-metro:" "$ARRAY_INFO_FILE" 2>/dev/null; then
      features="${features},metro"
    fi
  fi
  # Include sftp feature for powerflex driver if configured in array-info.yaml
  # Check if powerflex-sftp section exists in array-info.yaml
  if [[ "${POWERFLEX:-}" == "true" ]]; then
    # Read the array-info.yaml to check if SFTP section exists
    # If the section exists, add sftp to features - parse-array-info will only export non-empty values
    if grep -q "^powerflex-sftp:" "$ARRAY_INFO_FILE" 2>/dev/null; then
      features="${features},sftp"
    fi
  fi

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
    --timeout 10m \
    --namespace kube-system \
    --set 'enableSecretRotation=true' \
    --set 'syncSecret.enabled=true' \
    --set 'tokenRequests[0].audience=conjur'
}

function vaultSetupAutomation() {
  echo "Removing any existing vault installation..."
  helm delete vault0 || true
  echo "Installing vault with all secrets for Authorization tests..."
  go run ./scripts/vault-automation/main.go --kubeconfig "$KUBECONFIG" --name vault0 --env-config --secrets-store-csi-driver=true --csm-authorization-namespace "$E2E_NS_AUTH"
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

function installImageAllowlist() {
  # Install or update the csm-image-allowlist ConfigMap to allow
  # images from the internal artifactory registry used in csm-images ConfigMap.
  echo "Installing or updating csm-image-allowlist ConfigMap..."
  local ALLOWLIST_PATH="testfiles/common-templates/cm-allowed-images.yaml"
  if [[ -f "$ALLOWLIST_PATH" ]]; then
    kubectl apply -f "$ALLOWLIST_PATH"
  else
    echo "Warning: cm-allowed-images.yaml not found at $ALLOWLIST_PATH, skipping"
  fi
}

function installPrometheusCRDs() {
  # Idempotently install the Prometheus Operator CRDs needed for
  # ServiceMonitor, PodMonitor, and PrometheusRule objects. These are required
  # by any scenario that enables driver/module metrics (resiliency, replication,
  # observability, authorization-proxy-server) or driver alerting (prometheusRule).
  if kubectl get crd servicemonitors.monitoring.coreos.com &>/dev/null && \
     kubectl get crd podmonitors.monitoring.coreos.com &>/dev/null && \
     kubectl get crd prometheusrules.monitoring.coreos.com &>/dev/null; then
    echo "Prometheus Operator CRDs already present"
    return
  fi

  local PROMETHEUS_OPERATOR_VERSION="v0.93.0"
  echo "Installing Prometheus Operator CRDs (${PROMETHEUS_OPERATOR_VERSION})..."

  kubectl apply -f \
    "https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/${PROMETHEUS_OPERATOR_VERSION}/example/prometheus-operator-crd/monitoring.coreos.com_servicemonitors.yaml"
  kubectl apply -f \
    "https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/${PROMETHEUS_OPERATOR_VERSION}/example/prometheus-operator-crd/monitoring.coreos.com_podmonitors.yaml"
  kubectl apply -f \
    "https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/${PROMETHEUS_OPERATOR_VERSION}/example/prometheus-operator-crd/monitoring.coreos.com_prometheusrules.yaml"

  kubectl wait --for condition=Established crd/servicemonitors.monitoring.coreos.com --timeout=120s
  kubectl wait --for condition=Established crd/podmonitors.monitoring.coreos.com --timeout=120s
  kubectl wait --for condition=Established crd/prometheusrules.monitoring.coreos.com --timeout=120s
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
  if ! (go get github.com/onsi/ginkgo/v2 && go mod vendor); then
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

function ensureTLSHD() {
  if [[ "$LOCAL_NODE_PREREQS" == "true" ]]; then
    echo "Checking local tlshd availability (SSH node checks disabled)..."
    if ! command -v tlshd >/dev/null 2>&1; then
      echo "tlshd is not installed on the local host"
      return 1
    fi
    if command -v systemctl >/dev/null 2>&1 && ! systemctl is-active --quiet tlshd.service; then
      echo "tlshd.service is not active on the local host"
      return 1
    fi
    echo "Local tlshd is installed and active"
    return 0
  fi

  echo "Checking tlshd availability on cluster nodes..."
  
  # Get all node IPs from the cluster
  local nodes=$(kubectl get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}')
  
  if [ -z "$nodes" ]; then
    echo "No nodes found in cluster"
    return 1
  fi
  
  for node_ip in $nodes; do
    echo "Installing tlshd on node $node_ip..."
    
    # Check if tlshd is already installed
    if ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 root@$node_ip "which tlshd" 2>/dev/null; then
      echo "tlshd already installed on $node_ip, skipping"
      continue
    fi
    
    # Install ktls-utils package (contains tlshd)
    ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 root@$node_ip << 'EOF'
      set -e
      # Detect OS and install tlshd
      if [ -f /etc/redhat-release ]; then
        # RHEL/CentOS/Rocky
        if command -v dnf &> /dev/null; then
          dnf install -y ktls-utils || yum install -y ktls-utils
        else
          yum install -y ktls-utils
        fi
      elif [ -f /etc/debian_version ]; then
        # Debian/Ubuntu
        apt-get update
        apt-get install -y ktls-utils
      else
        echo "Unsupported OS, cannot install tlshd"
        exit 1
      fi
      
      # Start tlshd service
      systemctl enable tlshd || true
      systemctl start tlshd || true
      
      echo "tlshd installed and started on $(hostname)"
EOF
    
    if [ $? -eq 0 ]; then
      echo "Successfully installed tlshd on $node_ip"
    else
      echo "Failed to install tlshd on $node_ip"
      return 1
    fi
  done
  
  echo "tlshd verification completed on all nodes"
  return 0
}

function getPowerScaleOneFSVersion() {
  local response version

  export POWERSCALE_ONEFS_VERSION=""
  if ! command -v curl &>/dev/null; then
    echo "curl is unavailable; mTLS scenarios will be skipped because OneFS version cannot be checked"
    return 0
  fi

  response=$(curl --silent --show-error --fail --insecure \
    --user "${POWERSCALE_USER}:${POWERSCALE_PASS}" \
    "https://${POWERSCALE_ENDPOINT}:${POWERSCALE_PORT}/platform/3/cluster/config" 2>/dev/null) || {
    echo "Unable to query OneFS version; mTLS scenarios will be skipped"
    return 0
  }

  version=$(printf '%s' "$response" | sed -n 's/.*"release"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
  if [[ -z "$version" ]]; then
    echo "OneFS version was not returned by the API; mTLS scenarios will be skipped"
    return 0
  fi

  export POWERSCALE_ONEFS_VERSION="$version"
  echo "Detected PowerScale OneFS version: $POWERSCALE_ONEFS_VERSION"
}

function oneFSVersionSupportsMTLS() {
  local major minor
  major="${POWERSCALE_ONEFS_VERSION%%.*}"
  minor="${POWERSCALE_ONEFS_VERSION#*.}"
  minor="${minor%%.*}"

  [[ "$major" =~ ^[0-9]+$ && "$minor" =~ ^[0-9]+$ ]] || return 1
  (( major > 9 || (major == 9 && minor >= 16) ))
}

function configureMTLSEnvironment() {
  echo "Configuring mTLS environment..."
  
  # Check if POWERSCALE_MTLS_FQDN is set
  if [ -z "$POWERSCALE_MTLS_FQDN" ]; then
    echo "POWERSCALE_MTLS_FQDN not set, using array endpoint from array-info.yaml"
    
    # Try to get the array endpoint from array-info.yaml
    if [ -f "$ARRAY_INFO_FILE" ]; then
      local endpoint=$(grep -E "POWERSCALE_ENDPOINT|POWERSCALE_ARRAY_ENDPOINT" "$ARRAY_INFO_FILE" | head -1 | cut -d':' -f2 | xargs)
      if [ -n "$endpoint" ]; then
        # Extract IP from endpoint (remove protocol and port)
        local fqdn=$(echo "$endpoint" | sed -E 's|https?://([^:/]+).*|\1|')
        export POWERSCALE_MTLS_FQDN="$fqdn"
        echo "Using POWERSCALE_MTLS_FQDN=$POWERSCALE_MTLS_FQDN from array-info.yaml"
      else
        echo "Could not find POWERSCALE_ENDPOINT in array-info.yaml"
        return 1
      fi
    else
      echo "array-info.yaml not found, cannot auto-configure POWERSCALE_MTLS_FQDN"
      return 1
    fi
  else
    echo "Using provided POWERSCALE_MTLS_FQDN=$POWERSCALE_MTLS_FQDN"
  fi
  
  # Add FQDN to /etc/hosts if it's an IP address (for testing purposes)
  if [[ "$POWERSCALE_MTLS_FQDN" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "POWERSCALE_MTLS_FQDN is an IP address, adding to /etc/hosts as powerscale.local"
    if ! grep -q "powerscale.local" /etc/hosts; then
      echo "$POWERSCALE_MTLS_FQDN powerscale.local" | sudo tee -a /etc/hosts > /dev/null
      export POWERSCALE_MTLS_FQDN="powerscale.local"
      echo "Updated POWERSCALE_MTLS_FQDN to $POWERSCALE_MTLS_FQDN"
    fi
  fi
  
  return 0
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
  # Deduplicate the namespace list to avoid double-create errors.
  local -A seen=()
  local unique_ns=()
  for ns in "${E2E_NAMESPACES[@]}"; do
    if [[ -z "${seen[$ns]:-}" ]]; then
      seen[$ns]=1
      unique_ns+=("$ns")
    fi
  done
  for ns in "${unique_ns[@]}"; do
    if kubectl get namespace "$ns" &>/dev/null; then
      echo "Deleting existing namespace: $ns"
      removeAuthCRDs "$ns"
      kubectl delete namespace "$ns" --wait=true --timeout=120s || true
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
  # Delete namespaces FIRST and wait for pods to terminate. Pods with CSI
  # volumes (e.g. Secrets Store) need the CSI driver running to unmount.
  # If we uninstall the CSI driver before pods terminate, they get stuck in
  # Terminating because volume unmount can never complete.
  for ns in "${E2E_NAMESPACES[@]}"; do
    if kubectl get namespace "$ns" &>/dev/null; then
      echo "Deleting namespace: $ns"
      removeAuthCRDs "$ns"
      kubectl delete namespace "$ns" --wait=true --timeout=120s || true
    fi
  done
  # Now that all pods using CSI volumes are gone, safe to uninstall drivers
  echo "Cleaning up vault, conjur, and secrets-store installations..."
  helm uninstall vault0 conjur conjur-csi-provider -n default 2>/dev/null || true
  helm uninstall csi-secrets-store -n kube-system 2>/dev/null || true
  cleanupSecretsStoreCRDs
  
  # Clean up /etc/hosts entry for authorization proxy
  if [[ -n "$PROXY_HOST" && -n "$CLUSTER_IP" ]]; then
    echo "Removing authorization host from /etc/hosts file"
    sudo sed -i "/$PROXY_HOST/d" /etc/hosts 2>/dev/null || true
  fi

  # Clean up /etc/hosts entry for OpenShift router service
  if [[ "$ROUTER_HOST_ADDED" == "true" ]]; then
    echo "Removing OpenShift router service from /etc/hosts file"
    sudo sed -i "/router-internal-default.openshift-ingress.svc.cluster.local/d" /etc/hosts 2>/dev/null || true
  fi
  
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
  echo "  --csiaddons                                  use to run e2e CSI Addons replication suite"
  echo "  --no-modules                                 use to run e2e suite without any modules"
  echo "  --cosi                                       use to run e2e cosi suite"
  echo "  --powerflex                                   use to run e2e powerflex suite"
  echo "  --powerscale                                  use to run e2e powerscale suite"
  echo "  --powerstore                                  use to run e2e powerstore suite"
  echo "  --unity                                      use to run e2e unity suite"
  echo "  --powermax                                    use to run e2e powermax suite"
  echo "  --zoning                                     use to run powerflex zoning tests (requires multiple storage systems)"
  echo "  --minimal                                    use minimal testfiles scenarios"
  echo "  --install-vault                              force vault install (auto-installed when auth scenarios will run)"
  echo "  --install-conjur                             use to install authorization conjur instance with secrets for authorization tests"
  echo "  --add-tag=<scenario tag>                     use to specify scenarios to run by one of their tags"
  echo "  --no-cleanup-ns                              skip namespace deletion at the end of the test run"
  echo "  --continue-on-fail                           continue running scenarios after a failure (default: stop on first failure)"
  echo "  --offline-bundle                              use to run offline bundle create and prepare tests (requires local registry)"
  echo "  --junit-report=<path>                        write JUnit XML report to the given file path"
  echo "  --mtls-fqdn=<fqdn>                           set POWERSCALE_MTLS_FQDN for mTLS testing (auto-detected from array-info.yaml if not set)"
  echo "  --local-node-prereqs                         check tlshd locally without SSHing to Kubernetes nodes"
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
  echo "  # Offline bundle tests (requires local registry at localhost:5000)"
  echo "  $PROG --offline-bundle"
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
    csiaddons)
      export CSIADDONS=true ;;
    powerflex)
      export POWERFLEX=true ;;
    no-modules)
      export NOMODULES=true
      export AUTHORIZATION=false
      export AUTHORIZATIONPROXYSERVER=false
      export REPLICATION=false
      export OBSERVABILITY=false
      export RESILIENCY=false
      export CSIADDONS=false
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
    offline-bundle)
      export OFFLINE_BUNDLE=true ;;
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
    mtls-fqdn=*)
      export POWERSCALE_MTLS_FQDN=${OPTARG#*=}
      ;;
    local-node-prereqs)
      export LOCAL_NODE_PREREQS=true
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
          RESILIENCY CSIADDONS SANITY ZONING OFFLINE_BUNDLE; do
  if [[ "${!_v:-}" == "true" ]]; then
    ANY_FLAG_SET=true
    break
  fi
done

# Skip operator namespace for offline-bundle only tests
if [[ "${OFFLINE_BUNDLE:-}" != "true" ]]; then
  E2E_NAMESPACES+=("$E2E_NS_OPERATOR")
fi

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

  # Sanity suite spans all platforms (including Unity and COSI). When no
  # platform flag is given alongside --sanity, ensure all platform namespaces
  # are created so every sanity-tagged scenario has a namespace to deploy into.
  if [[ "${SANITY:-}" == "true" ]]; then
    ANY_PLATFORM_SET=false
    for _p in POWERFLEX POWERSCALE POWERMAX POWERSTORE UNITY COSI; do
      [[ "${!_p:-}" == "true" ]] && ANY_PLATFORM_SET=true && break
    done
    if [[ "$ANY_PLATFORM_SET" == "false" ]]; then
      E2E_NAMESPACES+=("$E2E_NS_POWERFLEX" "$E2E_NS_POWERSCALE" "$E2E_NS_POWERMAX" "$E2E_NS_POWERSTORE" "$E2E_NS_UNITY" "$E2E_NS_COSI")
    fi
  fi

  # Auth namespaces and vault are needed when auth scenarios will actually run:
  #   1. Auth flags explicitly set (--auth / --auth-proxy), OR
  #   2. --sanity includes auth scenarios (driver+auth and auth-proxy sanity tests), OR
  #   3. No module flags at all (and not --no-modules): all modules run for the
  #      selected platform(s), which includes auth for powerflex/powerscale/powerstore/powermax.
  # If a non-auth, non-sanity module flag is set (e.g. --obs), auth scenarios won't
  # match in ContainsTag so we skip the expensive vault/ingress setup.
  AUTH_WILL_RUN=false
  if [[ "${AUTHORIZATION:-}" == "true" || "${AUTHORIZATIONPROXYSERVER:-}" == "true" ]]; then
    AUTH_WILL_RUN=true
  elif [[ "${SANITY:-}" == "true" ]]; then
    # Sanity suite includes auth scenarios (driver+auth combos and auth-proxy
    # standalone tests), so auth infrastructure must be set up.
    AUTH_WILL_RUN=true
  elif [[ "${NOMODULES:-}" != "true" ]]; then
    # Check if any non-auth module flag is set.
    OTHER_MODULE_SET=false
    for flag in OBSERVABILITY REPLICATION RESILIENCY ZONING; do
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
    # Auth scenarios include driver+auth combos (e.g. "Install PowerFlex Driver
    # (With Authorization V2)") that deploy into driver namespaces. When no
    # platform flag is given, ContainsTag matches all platforms, so we must
    # ensure the driver namespaces exist for every platform that has auth
    # scenarios. If specific platforms were requested, their namespaces are
    # already added above.
    ANY_PLATFORM_SET=false
    for _p in POWERFLEX POWERSCALE POWERMAX POWERSTORE UNITY COSI; do
      [[ "${!_p:-}" == "true" ]] && ANY_PLATFORM_SET=true && break
    done
    if [[ "$ANY_PLATFORM_SET" == "false" ]]; then
      E2E_NAMESPACES+=("$E2E_NS_POWERFLEX" "$E2E_NS_POWERSCALE" "$E2E_NS_POWERSTORE" "$E2E_NS_POWERMAX")
    fi
  fi
fi

setupNamespaces

# Install image allowlist ConfigMap to allow internal artifactory registry images
installImageAllowlist

# Install Prometheus Operator CRDs required for metrics scenarios.
installPrometheusCRDs

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

  # For OpenShift, also add router-internal-default service to /etc/hosts
  if [[ "$IS_OPENSHIFT" == "true" ]]; then
    ROUTER_HOST="router-internal-default.openshift-ingress.svc.cluster.local"
    # Use the current host's IP (bastion) instead of service ClusterIP
    # since tests run from bastion and need DNS resolution locally
    ROUTER_IP=$(hostname -I | awk '{print $1}')
    if [[ -n "$ROUTER_IP" ]]; then
      echo "Adding OpenShift router service to /etc/hosts file"
      echo "$ROUTER_IP $ROUTER_HOST" | sudo tee -a /etc/hosts > /dev/null
      export ROUTER_HOST_ADDED=true
    fi
  fi
fi

checkForGinkgo

# Handle mTLS environment setup when the selected scenarios include mTLS.
if grep -qi "mtls" "$E2E_SCENARIOS_FILE"; then
  echo "mTLS scenarios are available, checking PowerScale environment..."

  # mTLS requires OneFS 9.16.0 or later. Query the array before installing
  # tlshd; older arrays run their non-mTLS scenarios normally.
  getPowerScaleOneFSVersion
  if oneFSVersionSupportsMTLS; then
    # ensureTLSHD skips nodes where tlshd is already installed.
    ensureTLSHD
  else
    echo "OneFS ${POWERSCALE_ONEFS_VERSION:-unknown} does not support mTLS; mTLS scenarios will be skipped"
  fi

  # Configure mTLS environment variables
  if [[ -n "$POWERSCALE_MTLS_FQDN" || -f "$ARRAY_INFO_FILE" ]]; then
    configureMTLSEnvironment
  fi
fi

# Ensure the baseline csm-images ConfigMap is present so that nightly image
# overrides are available for the latest version. This self-heals from a
# previous run that may have failed before its "Restore ConfigMap" cleanup step.
echo "Applying baseline csm-images ConfigMap..."
kubectl apply -f testfiles/common-templates/csm-images-baseline.yaml

if [[ $CHECK_PREREQUISITES_ONLY == "true" ]]; then
  echo "Skipping tests because check prerequisites only was requested."
  exit 0
fi

runTests
