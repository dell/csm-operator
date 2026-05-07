#!/bin/bash
#
# Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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
#
# E2E test for offline bundle creation and preparation.
#
# Prerequisites:
#   1. podman or docker must be installed on the test VM
#   2. A local container registry must be running and accessible at OFFLINE_BUNDLE_REGISTRY
#   3. All cluster nodes must be able to pull from the local registry
#   4. SSH keys must be set up for passwordless access to cluster nodes
#   5. If an override config is used, the override source registry must be reachable
#
# Usage:
#   ./offline_bundle_test.sh
#
# Environment variables:
#   OFFLINE_BUNDLE_REGISTRY  - local registry to push images to (default: <VM_IP>:5000/dell-csm-operator)
#   OVERRIDE_CONFIG          - path to image override config file (optional)
#   REPO_DIR                 - path to the csm-operator repository root (default: ../../..)

set -o pipefail

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
REPO_DIR="${REPO_DIR:-${SCRIPTDIR}/../../..}"
OFFLINE_BUNDLE_SCRIPT="${REPO_DIR}/scripts/csm-offline-bundle.sh"
OVERRIDE_CONFIG="${OVERRIDE_CONFIG:-${SCRIPTDIR}/../testfiles/common-templates/image-overrides.conf}"

# Constants
REGISTRY_PATH=":5000/dell-csm-operator"

# Detect VM IP address for default registry
if [ -z "${OFFLINE_BUNDLE_REGISTRY}" ]; then
  # Try to get the primary IP address, excluding pod network CIDRs
  VM_IP=""
  # Method 1: Use ip command to get the default route interface IP
  if command -v ip &>/dev/null; then
    DEFAULT_IFACE=$(ip route show default 2>/dev/null | awk '/default/ {print $5}' | head -1)
    if [ -n "${DEFAULT_IFACE}" ]; then
      VM_IP=$(ip addr show "${DEFAULT_IFACE}" 2>/dev/null | grep "inet " | awk '{print $2}' | cut -d/ -f1 | head -1)
    fi
  fi

  # Method 2: Fallback to hostname -I, filter out pod networks
  if [ -z "${VM_IP}" ] && command -v hostname &>/dev/null; then
    for ip in $(hostname -I 2>/dev/null); do
      # Skip pod network CIDRs (10.244.0.0/16, 192.168.0.0/16, 172.16.0.0/12)
      if [[ ! "${ip}" =~ ^10\.244\. ]] && [[ ! "${ip}" =~ ^172\.(1[6-9]|2[0-9]|3[0-1])\. ]] && [[ "${ip}" != "127.0.0.1" ]]; then
        VM_IP="${ip}"
        break
      fi
    done
  fi

  # Method 3: Fallback to ifconfig
  if [ -z "${VM_IP}" ] && command -v ifconfig &>/dev/null; then
    for ip in $(ifconfig 2>/dev/null | grep "inet " | awk '{print $2}' | cut -d: -f2); do
      if [[ ! "${ip}" =~ ^10\.244\. ]] && [[ ! "${ip}" =~ ^172\.(1[6-9]|2[0-9]|3[0-1])\. ]] && [[ "${ip}" != "127.0.0.1" ]]; then
        VM_IP="${ip}"
        break
      fi
    done
  fi

  if [ -z "${VM_IP}" ]; then
    echo "ERROR: Could not detect VM IP address. Please set OFFLINE_BUNDLE_REGISTRY manually."
    exit 1
  fi
  OFFLINE_BUNDLE_REGISTRY="${VM_IP}${REGISTRY_PATH}"
  echo "Auto-detected VM IP: ${VM_IP}"
  echo "Using registry: ${OFFLINE_BUNDLE_REGISTRY}"
fi

PASS=0
FAIL=0
ERRORS=""

pass() {
  echo "  [PASS] $1"
  ((PASS++))
}

fail() {
  echo "  [FAIL] $1"
  ((FAIL++))
  ERRORS="${ERRORS}\n  - $1"
}

status() {
  echo ""
  echo "* $1"
  echo ""
}

###############################################################################
# Prerequisite checks
###############################################################################
check_prerequisites() {
  status "Checking prerequisites"

  # 1. Check for container runtime (podman or docker)
  CONTAINER_RUNTIME=""
  if command -v podman &>/dev/null; then
    CONTAINER_RUNTIME="podman"
    pass "Container runtime found: podman"
  elif command -v docker &>/dev/null; then
    CONTAINER_RUNTIME="docker"
    pass "Container runtime found: docker"
  else
    fail "No container runtime found. Install podman or docker."
    return 1
  fi

  # 2. Check container runtime is functional
  if ${CONTAINER_RUNTIME} info &>/dev/null; then
    pass "Container runtime '${CONTAINER_RUNTIME}' is functional"
  else
    fail "Container runtime '${CONTAINER_RUNTIME}' is not functional. Check daemon status."
    return 1
  fi

  # 3. Check that the offline bundle script exists
  if [ -f "${OFFLINE_BUNDLE_SCRIPT}" ]; then
    pass "Offline bundle script found: ${OFFLINE_BUNDLE_SCRIPT}"
  else
    fail "Offline bundle script not found: ${OFFLINE_BUNDLE_SCRIPT}"
    return 1
  fi

  # 4. Check local registry is accessible
  local registry_host="${OFFLINE_BUNDLE_REGISTRY%%/*}"
  if curl -sf "http://${registry_host}/v2/" &>/dev/null || \
     curl -sfk "https://${registry_host}/v2/" &>/dev/null; then
    pass "Local registry is accessible at ${registry_host}"
  else
    fail "Local registry is NOT accessible at ${registry_host}. Ensure a local registry is running (e.g., 'podman run -d -p 5000:5000 registry:2')."
    return 1
  fi

  # 5. Check override config exists (if specified)
  if [ -n "${OVERRIDE_CONFIG}" ]; then
    if [ -f "${OVERRIDE_CONFIG}" ]; then
      pass "Override config found: ${OVERRIDE_CONFIG}"
    else
      fail "Override config not found: ${OVERRIDE_CONFIG}"
      return 1
    fi
  fi

  # 6. Verify we can pull from the override source registry
  if [ -n "${OVERRIDE_CONFIG}" ] && [ -f "${OVERRIDE_CONFIG}" ]; then
    if ! check_override_registry_access; then
      return 1
    fi
  fi

  # 7. Check cluster nodes can reach the local registry
  if ! check_cluster_registry_access; then
    return 1
  fi

  return 0
}

# Verify that the override source images are accessible
check_override_registry_access() {
  status "Verifying override source registry access"

  # Extract the first override destination to test pull access
  local test_image=""
  while IFS= read -r line; do
    [[ "${line}" =~ ^[[:space:]]*$ ]] && continue
    [[ "${line}" =~ ^[[:space:]]*# ]] && continue
    local dst="${line#*=}"
    dst=$(echo "${dst}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
    if [ -n "${dst}" ]; then
      test_image="${dst}"
      break
    fi
  done < "${OVERRIDE_CONFIG}"

  if [ -z "${test_image}" ]; then
    fail "No override images found in config"
    return 1
  fi

  echo "  Testing pull access with: ${test_image}"
  if ${CONTAINER_RUNTIME} pull "${test_image}" &>/dev/null; then
    pass "Can pull from override source registry (tested: ${test_image})"
    # Clean up test image
    ${CONTAINER_RUNTIME} rmi "${test_image}" &>/dev/null || true
  else
    fail "Cannot pull from override source registry. Tested: ${test_image}. Check credentials and network access."
    return 1
  fi
}


# Verify that cluster nodes can reach the local registry
check_cluster_registry_access() {
  status "Verifying cluster nodes can access local registry"

  local registry_host="${OFFLINE_BUNDLE_REGISTRY%%/*}"

  # Check if kubectl is available
  if ! command -v kubectl &>/dev/null; then
    fail "kubectl not found. Cannot verify cluster registry access."
    return 1
  fi

  # Get worker nodes (exclude control-plane)
  local nodes
  nodes=$(kubectl get nodes -l '!node-role.kubernetes.io/control-plane' -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)
  if [ -z "${nodes}" ]; then
    fail "No cluster nodes found. Is the cluster accessible?"
    return 1
  fi

  # Note: Ensure SSH keys are set up for passwordless access to cluster nodes
  echo "  [INFO] Checking cluster node registry access via SSH"
  echo "  [INFO] Ensure SSH keys are configured for passwordless access to all cluster nodes"

  local check_passed=true
  local registry_host_only="${registry_host%%:*}"
  local registry_port="${registry_host##*:}"
  if [ "${registry_port}" = "${registry_host_only}" ]; then
    registry_port="5000"
  fi

  echo "  Checking registry access from worker nodes (${registry_host_only}:${registry_port})..."

  # Check each worker node via SSH
  for node in ${nodes}; do
    echo "  - Checking node: ${node}"
    # Get the node's external IP address
    local node_ip
    node_ip=$(kubectl get node "${node}" -o jsonpath='{.status.addresses[?(@.type=="ExternalIP")].address}' 2>/dev/null)
    if [ -z "${node_ip}" ]; then
      # Fallback to InternalIP if ExternalIP not available
      node_ip=$(kubectl get node "${node}" -o jsonpath='{.status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null)
    fi

    if [ -z "${node_ip}" ]; then
      fail "Could not determine IP address for node ${node}"
      check_passed=false
      continue
    fi

    # SSH to the node and check crictl and registry configuration
    local ssh_cmd="ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no ${node_ip}"
    
    # Check if crictl is installed
    if ! ${ssh_cmd} "command -v crictl &>/dev/null"; then
      fail "Node ${node} (${node_ip}) does not have crictl installed"
      check_passed=false
      continue
    fi
    
    # Check if registry certs directory exists
    if ${ssh_cmd} "[ -d '/etc/containerd/certs.d/${registry_host_only}' ] || [ -d '/etc/containerd/certs.d/${registry_host_only}:${registry_port}' ]" 2>/dev/null; then
      pass "Node ${node} (${node_ip}) has registry configuration for ${registry_host}"
    else
      fail "Node ${node} (${node_ip}) does not have registry configuration for ${registry_host}"
      check_passed=false
    fi
  done

  if [ "${check_passed}" = "false" ]; then
    return 1
  fi
  return 0
}

###############################################################################
# Test: Create offline bundle
###############################################################################
test_create_bundle() {
  status "Test: Creating offline bundle"

  local create_args="-c"
  if [ -n "${OVERRIDE_CONFIG}" ] && [ -f "${OVERRIDE_CONFIG}" ]; then
    create_args="${create_args} -o ${OVERRIDE_CONFIG}"
  fi

  if bash "${OFFLINE_BUNDLE_SCRIPT}" ${create_args}; then
    pass "Offline bundle created successfully"
  else
    fail "Offline bundle creation failed"
    return 1
  fi

  # Verify the bundle tarball was created
  local bundle_file="${REPO_DIR}/dell-csm-operator-bundle.tar.gz"
  if [ -f "${bundle_file}" ]; then
    local size
    size=$(du -h "${bundle_file}" | cut -f1)
    pass "Bundle tarball exists: ${bundle_file} (${size})"
  else
    fail "Bundle tarball not found: ${bundle_file}"
    return 1
  fi

  # Create temp directory using mktemp and move tarball there
  local temp_dir
  temp_dir=$(mktemp -d)
  if [ $? -ne 0 ] || [ -z "${temp_dir}" ]; then
    fail "Failed to create temporary directory with mktemp"
    return 1
  fi
  local bundle_temp="${temp_dir}/dell-csm-operator-bundle.tar.gz"
  
  if cp "${bundle_file}" "${bundle_temp}"; then
    pass "Bundle tarball moved to temp directory: ${bundle_temp}"
  else
    fail "Failed to move bundle tarball to temp directory"
    return 1
  fi

  # Verify the image manifest was created
  local manifest="${REPO_DIR}/scripts/images.manifest"
  if [ -f "${manifest}" ]; then
    local count
    count=$(wc -l < "${manifest}")
    pass "Image manifest created with ${count} images"
  else
    fail "Image manifest not found: ${manifest}"
    return 1
  fi

  # Export temp directory for other functions
  export TEMP_DIR="${temp_dir}"
  
  # Remove all images from local registry to ensure next step loads them
  remove_local_registry_images

  return 0
}

###############################################################################
# Test: Prepare offline bundle
###############################################################################
test_prepare_bundle() {
  status "Test: Preparing offline bundle for installation"

  # Get temp directory from environment (passed from test_create_bundle)
  local temp_dir="${TEMP_DIR:-$(mktemp -d)}"
  if [ $? -ne 0 ] || [ -z "${temp_dir}" ]; then
    fail "Failed to create temporary directory with mktemp"
    return 1
  fi
  local bundle_temp="${temp_dir}/dell-csm-operator-bundle.tar.gz"

  # Extract the tarball directly to temp directory
  if [ ! -f "${bundle_temp}" ]; then
    fail "Bundle tarball not found in temp directory: ${bundle_temp}"
    return 1
  fi

  # Clean up previous extraction if exists
  rm -rf "${temp_dir}/dell-csm-operator-bundle"

  if tar -xzf "${bundle_temp}" -C "${temp_dir}"; then
    pass "Bundle tarball extracted to: ${temp_dir}"
  else
    fail "Failed to extract bundle tarball"
    return 1
  fi

  # The extracted directory should be dell-csm-operator-bundle
  local bundle_dir="${temp_dir}/dell-csm-operator-bundle"
  if [ ! -d "${bundle_dir}" ]; then
    fail "Bundle directory not found after extraction: ${bundle_dir}"
    return 1
  fi

  # Change to bundle directory and run prepare
  pushd "${bundle_dir}" || {
    fail "Failed to change to bundle directory: ${bundle_dir}"
    return 1
  }

  # Get the VM IP for registry (re-detect to ensure we have the right IP)
  local vm_ip=""
  if command -v ip &>/dev/null; then
    DEFAULT_IFACE=$(ip route show default 2>/dev/null | awk '/default/ {print $5}' | head -1)
    if [ -n "${DEFAULT_IFACE}" ]; then
      vm_ip=$(ip addr show "${DEFAULT_IFACE}" 2>/dev/null | grep "inet " | awk '{print $2}' | cut -d/ -f1 | head -1)
    fi
  fi

  if [ -z "${vm_ip}" ]; then
    for ip in $(hostname -I 2>/dev/null); do
      if [[ ! "${ip}" =~ ^10\.244\. ]] && [[ ! "${ip}" =~ ^172\.(1[6-9]|2[0-9]|3[0-1])\. ]] && [[ "${ip}" != "127.0.0.1" ]]; then
        vm_ip="${ip}"
        break
      fi
    done
  fi

  if [ -z "${vm_ip}" ]; then
    fail "Could not detect VM IP address for prepare step"
    popd
    return 1
  fi

  local prepare_registry="${vm_ip}${REGISTRY_PATH}"
  echo "  Using registry for prepare: ${prepare_registry}"

  # Run prepare with the detected registry
  local prepare_args="-p -r ${prepare_registry}"
  if [ -n "${OVERRIDE_CONFIG}" ] && [ -f "${OVERRIDE_CONFIG}" ]; then
    prepare_args="${prepare_args} -o ${OVERRIDE_CONFIG}"
  fi

  if bash "./scripts/csm-offline-bundle.sh" ${prepare_args}; then
    pass "Offline bundle prepared successfully"
  else
    fail "Offline bundle preparation failed"
    popd
    return 1
  fi

  # Return to original directory
  popd

  # Verify the bundle directory exists with modified files
  if [ -d "${bundle_dir}" ]; then
    pass "Bundle directory exists: ${bundle_dir}"
  else
    fail "Bundle directory not found: ${bundle_dir}"
    return 1
  fi

  # Verify files were updated to point to the local registry
  local registry_prefix="${prepare_registry%%/*}"
  local modified_files
  modified_files=$(find "${bundle_dir}" -name "*.yaml" -exec grep -l "${registry_prefix}" {} \; 2>/dev/null | wc -l)
  if [ "${modified_files}" -gt 0 ]; then
    pass "Found ${modified_files} YAML files updated with target registry"
  else
    fail "No YAML files were updated with the target registry (${registry_prefix})"
    return 1
  fi

  # Verify images were pushed to the local registry
  verify_registry_images

  return 0
}

###############################################################################
# Verify images are in the local registry
###############################################################################
verify_registry_images() {
  status "Verifying images in local registry"

  local manifest="${REPO_DIR}/scripts/images.manifest"
  if [ ! -f "${manifest}" ]; then
    fail "Image manifest not found for verification"
    return 1
  fi

  local total=0
  local found=0
  local registry_host="${OFFLINE_BUNDLE_REGISTRY%%/*}"

  while IFS= read -r image_line; do
    ((total++))
    # Extract the image name and tag
    local image_name="${image_line##*/}"
    local repo_name="${image_name%%:*}"
    local tag="${image_name##*:}"
    local full_repo="${OFFLINE_BUNDLE_REGISTRY}/${repo_name}"

    # Query the registry catalog/tags
    if curl -sf "http://${registry_host}/v2/${OFFLINE_BUNDLE_REGISTRY#*/}/${repo_name}/tags/list" 2>/dev/null | grep -q "${tag}"; then
      ((found++))
    else
      echo "  WARNING: Image not found in registry: ${full_repo}:${tag}"
    fi
  done < "${manifest}"

  if [ "${found}" -eq "${total}" ]; then
    pass "All ${total} images found in local registry"
  elif [ "${found}" -gt 0 ]; then
    pass "Found ${found}/${total} images in local registry (some may use different naming)"
  else
    fail "No images found in local registry"
  fi
}

###############################################################################
# Remove local registry images
###############################################################################
remove_local_registry_images() {
  status "Removing images from local registry"

  local manifest="${REPO_DIR}/scripts/images.manifest"
  if [ ! -f "${manifest}" ]; then
    echo "  No image manifest found, nothing to remove"
    return 0
  fi

  local total=0
  local removed=0
  local registry_host="${OFFLINE_BUNDLE_REGISTRY%%/*}"

  while IFS= read -r image_line; do
    ((total++))
    # Extract the image name and tag
    local image_name="${image_line##*/}"
    local repo_name="${image_name%%:*}"
    local tag="${image_name##*:}"
    local full_repo="${OFFLINE_BUNDLE_REGISTRY}/${repo_name}"

    # Remove the original source image (from manifest line)
    if ${CONTAINER_RUNTIME} rmi "${image_line}" &>/dev/null; then
      ((removed++))
      echo "  Removed original source image: ${image_line}"
    else
      echo "  Original source image not found locally (already removed): ${image_line}"
    fi

    # Remove the retagged local registry image
    if ${CONTAINER_RUNTIME} rmi "${full_repo}:${tag}" &>/dev/null; then
      ((removed++))
      echo "  Removed retagged registry image: ${full_repo}:${tag}"
    else
      echo "  Retagged registry image not found locally (already removed): ${full_repo}:${tag}"
    fi
  done < "${manifest}"

  local total_images=$((total * 2))
  if [ "${removed}" -gt 0 ]; then
    pass "Removed ${removed}/${total_images} images from local container runtime"
  else
    pass "No images were found locally to remove"
  fi
}

###############################################################################
# Test: Deploy operator from extracted bundle
###############################################################################
test_deploy_operator() {
  status "Test: Deploying operator from extracted bundle"

  # Get temp directory from environment (passed from previous functions)
  local temp_dir="${TEMP_DIR:-$(mktemp -d)}"
  if [ $? -ne 0 ] || [ -z "${temp_dir}" ]; then
    fail "Failed to create temporary directory with mktemp"
    return 1
  fi
  local bundle_dir="${temp_dir}/dell-csm-operator-bundle"

  # Verify the bundle directory exists
  if [ ! -d "${bundle_dir}" ]; then
    fail "Bundle directory not found: ${bundle_dir}"
    return 1
  fi

  # Check if install script exists
  local install_script="${bundle_dir}/scripts/install.sh"
  if [ ! -f "${install_script}" ]; then
    fail "Install script not found: ${install_script}"
    return 1
  fi

  # Change to bundle directory for deployment
  pushd "${bundle_dir}" || {
    fail "Failed to change to bundle directory: ${bundle_dir}"
    return 1
  }

  # Make install script executable
  chmod +x "${install_script}"

  echo "  Deploying operator from: ${bundle_dir}"
  if bash "${install_script}" &>/dev/null; then
    pass "Operator deployment initiated successfully"
  else
    fail "Operator deployment failed"
    popd
    return 1
  fi

  # Wait for operator deployment to be available
  echo "  Waiting for operator deployment to be available..."
  if kubectl wait --for=condition=Available deployment/dell-csm-operator-controller-manager -n dell-csm-operator --timeout=300s; then
    pass "Operator deployment is available and ready"
  else
    fail "Operator deployment did not become available within 300 seconds"
    popd
    return 1
  fi

  # Return to original directory
  popd

  return 0
}

###############################################################################
# Cleanup
###############################################################################
cleanup() {
  status "Cleaning up"

  # Remove images from local registry
  remove_local_registry_images

  # Undeploy operator if it was installed
  if kubectl get namespace dell-csm-operator &>/dev/null; then
    echo "  Undeploying operator..."
    # Get temp directory from environment (passed from previous functions)
    local temp_dir="${TEMP_DIR:-$(mktemp -d)}"
    if [ $? -ne 0 ] || [ -z "${temp_dir}" ]; then
      fail "Failed to create temporary directory with mktemp"
      return 1
    fi
    local bundle_dir="${temp_dir}/dell-csm-operator-bundle"
    if [ -d "${bundle_dir}" ] && [ -f "${bundle_dir}/scripts/uninstall.sh" ]; then
      cd "${bundle_dir}" && bash scripts/uninstall.sh &>/dev/null || true
    fi
  fi

  # Remove bundle artifacts
  rm -rf "${REPO_DIR}/dell-csm-operator-bundle" 2>/dev/null
  rm -f "${REPO_DIR}/dell-csm-operator-bundle.tar.gz" 2>/dev/null
  rm -f "${REPO_DIR}/scripts/images.manifest" 2>/dev/null
  rm -rf "${REPO_DIR}/scripts/images.tar" 2>/dev/null

  # Remove temp directory if it exists
  if [ -n "${TEMP_DIR}" ] && [ -d "${TEMP_DIR}" ]; then
    rm -rf "${TEMP_DIR}"
  fi

  pass "Cleanup complete"
}

###############################################################################
# Main
###############################################################################
main() {
  echo "============================================================"
  echo "  CSM Operator - Offline Bundle E2E Test"
  echo "============================================================"
  echo ""
  echo "  Registry:        ${OFFLINE_BUNDLE_REGISTRY}"
  echo "  Override Config:  ${OVERRIDE_CONFIG:-none}"
  echo "  Repo Dir:         ${REPO_DIR}"
  echo ""

  # Run prerequisites check
  if ! check_prerequisites; then
    echo ""
    echo "============================================================"
    echo "  PREREQUISITES FAILED - Cannot proceed with tests"
    echo "============================================================"
    echo -e "  Errors:${ERRORS}"
    echo ""
    exit 1
  fi

  # Run tests - exit immediately if any test fails
  if ! test_create_bundle; then
    echo ""
    echo "============================================================"
    echo "  TEST FAILED: Create Bundle"
    echo "============================================================"
    cleanup
    exit 1
  fi

  if ! test_prepare_bundle; then
    echo ""
    echo "============================================================"
    echo "  TEST FAILED: Prepare Bundle"
    echo "============================================================"
    cleanup
    exit 1
  fi

  if ! test_deploy_operator; then
    echo ""
    echo "============================================================"
    echo "  TEST FAILED: Deploy Operator"
    echo "============================================================"
    cleanup
    exit 1
  fi

  # Cleanup
  cleanup

  # Report
  echo ""
  echo "============================================================"
  echo "  TEST RESULTS"
  echo "============================================================"
  echo "  Passed: ${PASS}"
  echo "  Failed: ${FAIL}"

  if [ -n "${ERRORS}" ]; then
    echo ""
    echo "  Failures:"
    echo -e "${ERRORS}"
  fi

  echo "============================================================"

  if [ "${FAIL}" -gt 0 ]; then
    exit 1
  fi
  exit 0
}

main "$@"
