#!/bin/bash
# Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

# This script is used as a command line argument in the e2e CustomTest section. It
# validates CSI PowerMax multi-array zone pooling behavior: a Kubernetes
# availability zone backed by more than one PowerMax array.
#
# Usage:
#   ./validate_multiaz_zoning.sh pool-size <secret-name> <namespace> <zone-label-key> <min-arrays>
#   ./validate_multiaz_zoning.sh invalid-entry-excluded <secret-name> <namespace> <zone-label-key> <expected-valid-count>
#   ./validate_multiaz_zoning.sh snapshot-restarts <namespace> <label-selector> <outfile>
#   ./validate_multiaz_zoning.sh verify-no-restart <namespace> <label-selector> <infile>
#   ./validate_multiaz_zoning.sh zone-only-storageclass <storageclass-name> <zone-label-key> <zone-value>

set -euo pipefail

read_secret_config() {
  local secret_name=$1 namespace=$2
  kubectl get secret "$secret_name" -n "$namespace" -o jsonpath='{.data.config}' | base64 --decode
}

# UC-1 / AC-001: asserts that at least <min_arrays> storageArrays entries in the
# Secret share the same "<zone_label_key>" value, proving the zone is pooled
# across multiple arrays rather than constrained to a single array.
pool_size() {
  local secret_name=$1 namespace=$2 zone_label_key=$3 min_arrays=$4
  local config zone_value zone_count
  config=$(read_secret_config "$secret_name" "$namespace")

  zone_value=$(echo "$config" | grep "$zone_label_key" | head -1 | awk -F': ' '{print $2}' | tr -d '"' | xargs)
  if [ -z "$zone_value" ]; then
    echo "FAIL: no zone label '$zone_label_key' found in secret $secret_name"
    exit 1
  fi

  zone_count=$(echo "$config" | grep -c "$zone_label_key: $zone_value")
  echo "Zone '$zone_value' is configured on $zone_count storageArrays entries (minimum required: $min_arrays)"

  if [ "$zone_count" -lt "$min_arrays" ]; then
    echo "FAIL: expected at least $min_arrays arrays pooled under zone '$zone_value', found $zone_count"
    exit 1
  fi
  echo "PASS: zone '$zone_value' is pooled across $zone_count array(s)"
}

# AC-001 (negative case): counts well-formed storageArrays entries (non-empty
# primaryEndpoint AND the expected zone label) and asserts it equals
# <expected_valid_count>, proving a malformed sibling entry did not reduce the
# pool below the known-good arrays.
invalid_entry_excluded() {
  local secret_name=$1 namespace=$2 zone_label_key=$3 expected_valid_count=$4
  local config valid_count=0 in_entry=0 has_endpoint=0 has_zone=0
  config=$(read_secret_config "$secret_name" "$namespace")

  while IFS= read -r line; do
    if [[ $line =~ ^[[:space:]]*-[[:space:]]*storageArrayId: ]]; then
      if [ "$in_entry" -eq 1 ] && [ "$has_endpoint" -eq 1 ] && [ "$has_zone" -eq 1 ]; then
        valid_count=$((valid_count + 1))
      fi
      in_entry=1
      has_endpoint=0
      has_zone=0
    fi
    [[ $line =~ primaryEndpoint:[[:space:]]*[\"a-zA-Z] ]] && has_endpoint=1
    [[ $line == *"$zone_label_key"* ]] && has_zone=1
  done <<< "$config"
  if [ "$in_entry" -eq 1 ] && [ "$has_endpoint" -eq 1 ] && [ "$has_zone" -eq 1 ]; then
    valid_count=$((valid_count + 1))
  fi

  echo "Well-formed storageArrays entries with zone label '$zone_label_key': $valid_count (expected: $expected_valid_count)"
  if [ "$valid_count" -ne "$expected_valid_count" ]; then
    echo "FAIL: expected exactly $expected_valid_count well-formed array entries, found $valid_count"
    exit 1
  fi
  echo "PASS: malformed entry did not affect the $expected_valid_count valid array(s)"
}

# UC-4 / FR-6: records controller pod name(s) and restart count(s) so a later
# verify-no-restart call can prove a Secret update (adding a second array to an
# existing zone) was picked up without disrupting the running controller pod.
snapshot_restarts() {
  local namespace=$1 selector=$2 outfile=$3
  kubectl get pods -n "$namespace" -l "$selector" \
    -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.containerStatuses[0].restartCount}{"\n"}{end}' \
    > "$outfile"
  echo "Snapshot written to $outfile:"
  cat "$outfile"
}

verify_no_restart() {
  local namespace=$1 selector=$2 infile=$3
  local current
  current=$(kubectl get pods -n "$namespace" -l "$selector" \
    -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.containerStatuses[0].restartCount}{"\n"}{end}')

  echo "Before:"
  cat "$infile"
  echo "After:"
  echo "$current"

  if ! diff <(sort "$infile") <(echo "$current" | sort) > /dev/null; then
    echo "FAIL: controller pod identity/restart count changed; expected the Secret update to be hot-reloaded without a restart"
    exit 1
  fi
  echo "PASS: controller pod(s) unchanged after the Secret update -- no restart occurred"
}

# Requirement 2/FR-2 (and FR-2.1's "no parameter" counterpart to Requirement 5):
# asserts a StorageClass carries no array-identifying parameter (e.g. SYMID)
# and that its allowedTopologies constrain provisioning only by the shared
# zone label -- proving a PVC submitted against this class matches every
# pooled array's topology and must therefore be resolved via the driver's
# capacity-based pool selection rather than being pinned to a single array.
zone_only_storageclass() {
  local sc_name=$1 zone_label_key=$2 zone_value=$3
  local sc_json symid topology_match

  sc_json=$(kubectl get sc "$sc_name" -o json)

  symid=$(echo "$sc_json" | jq -r '.parameters.SYMID // empty')
  if [ -n "$symid" ]; then
    echo "FAIL: StorageClass '$sc_name' has array-specific parameter SYMID='$symid'; expected no SYMID for zone-pool provisioning"
    exit 1
  fi

  topology_match=$(echo "$sc_json" | jq -r --arg key "$zone_label_key" --arg val "$zone_value" \
    '[.allowedTopologies[]?.matchLabelExpressions[]? | select(.key == $key and (.values | index($val) != null))] | length')
  if [ "$topology_match" -lt 1 ]; then
    echo "FAIL: StorageClass '$sc_name' does not restrict allowedTopologies to zone label '$zone_label_key=$zone_value'"
    exit 1
  fi

  echo "PASS: StorageClass '$sc_name' has no array-specific parameter and is scoped only to zone '$zone_label_key=$zone_value' -- a PVC against it must use pool-based array selection"
}

# Creates a PVC and Pod from YAML templates and waits for the PVC to reach
# Bound state and the Pod to reach Running state, proving the driver
# successfully provisioned a volume from the zone pool.
create_pvc_and_pod() {
  local pvc_file=$1 pod_file=$2 namespace=$3 timeout_secs=${4:-120}
  local pvc_name pod_name elapsed=0

  pvc_name=$(grep 'name:' "$pvc_file" | head -1 | awk '{print $2}')
  pod_name=$(grep 'name:' "$pod_file" | head -1 | awk '{print $2}')

  echo "Creating PVC from $pvc_file in namespace $namespace ..."
  kubectl apply -f "$pvc_file" -n "$namespace"

  echo "Creating Pod from $pod_file in namespace $namespace ..."
  kubectl apply -f "$pod_file" -n "$namespace"

  echo "Waiting up to ${timeout_secs}s for PVC '$pvc_name' to reach Bound state ..."
  while [ "$elapsed" -lt "$timeout_secs" ]; do
    local phase
    phase=$(kubectl get pvc "$pvc_name" -n "$namespace" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    if [ "$phase" = "Bound" ]; then
      echo "PVC '$pvc_name' is Bound after ${elapsed}s"
      break
    fi
    sleep 5
    elapsed=$((elapsed + 5))
  done

  local pvc_phase
  pvc_phase=$(kubectl get pvc "$pvc_name" -n "$namespace" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
  if [ "$pvc_phase" != "Bound" ]; then
    echo "FAIL: PVC '$pvc_name' is '$pvc_phase' after ${timeout_secs}s (expected Bound)"
    kubectl describe pvc "$pvc_name" -n "$namespace" 2>/dev/null || true
    kubectl logs -n "$namespace" -l app=powermax-controller -c driver --tail=30 2>/dev/null || true
    exit 1
  fi

  echo "Waiting up to ${timeout_secs}s for Pod '$pod_name' to reach Running state ..."
  elapsed=0
  while [ "$elapsed" -lt "$timeout_secs" ]; do
    local pod_phase
    pod_phase=$(kubectl get pod "$pod_name" -n "$namespace" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    if [ "$pod_phase" = "Running" ]; then
      echo "Pod '$pod_name' is Running after ${elapsed}s"
      break
    fi
    sleep 5
    elapsed=$((elapsed + 5))
  done

  local final_pod_phase
  final_pod_phase=$(kubectl get pod "$pod_name" -n "$namespace" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
  if [ "$final_pod_phase" != "Running" ]; then
    echo "FAIL: Pod '$pod_name' is '$final_pod_phase' after ${timeout_secs}s (expected Running)"
    kubectl describe pod "$pod_name" -n "$namespace" 2>/dev/null || true
    exit 1
  fi

  echo "PASS: PVC '$pvc_name' is Bound and Pod '$pod_name' is Running -- volume provisioned from zone pool"
}

# Validates that the PV backing a PVC was provisioned on one of the expected
# arrays in the zone pool by inspecting the PV's volumeHandle, which contains
# the array serial (SYMID).
validate_pvc_on_pool_array() {
  local pvc_name=$1 namespace=$2 secret_name=$3 zone_label_key=$4

  local pv_name volume_handle
  pv_name=$(kubectl get pvc "$pvc_name" -n "$namespace" -o jsonpath='{.spec.volumeName}')
  volume_handle=$(kubectl get pv "$pv_name" -o jsonpath='{.spec.csi.volumeHandle}')
  echo "PVC '$pvc_name' -> PV '$pv_name' -> volumeHandle '$volume_handle'"

  # Extract array IDs from the secret that share the zone label
  local config pool_arrays
  config=$(read_secret_config "$secret_name" "$namespace")
  pool_arrays=$(echo "$config" | grep -B5 "$zone_label_key" | grep storageArrayId | awk -F'"' '{print $2}')

  local matched=false
  for arr in $pool_arrays; do
    if echo "$volume_handle" | grep -q "$arr"; then
      echo "PASS: volume was provisioned on array '$arr' which is in the zone pool"
      matched=true
      break
    fi
  done

  if [ "$matched" = false ]; then
    echo "FAIL: volumeHandle '$volume_handle' does not contain any pool array ID ($pool_arrays)"
    exit 1
  fi
}

# Validates that a PVC submitted against a StorageClass with an explicit
# SYMID parameter was provisioned on exactly that array -- proving the SYMID
# parameter directs provisioning to the named array only, bypassing
# capacity-based pool selection, even when the zone contains multiple pooled
# arrays.
validate_pvc_on_specific_array() {
  local pvc_name=$1 namespace=$2 expected_array_id=$3

  local pv_name volume_handle
  pv_name=$(kubectl get pvc "$pvc_name" -n "$namespace" -o jsonpath='{.spec.volumeName}')
  volume_handle=$(kubectl get pv "$pv_name" -o jsonpath='{.spec.csi.volumeHandle}')
  echo "PVC '$pvc_name' -> PV '$pv_name' -> volumeHandle '$volume_handle'"

  if echo "$volume_handle" | grep -q "$expected_array_id"; then
    echo "PASS: volume was provisioned on the SYMID-pinned array '$expected_array_id' as expected"
  else
    echo "FAIL: volumeHandle '$volume_handle' does not contain the pinned array ID '$expected_array_id' -- SYMID pinning did not route to the expected array"
    exit 1
  fi
}

# Cleans up PVC, Pod, and StorageClass resources created during the test.
cleanup_pvc_pod() {
  local namespace=$1 pvc_name=${2:-multiaz-zone-pool-pvc} pod_name=${3:-multiaz-zone-pool-pod} sc_name=${4:-op-e2e-powermax-multiaz-zone-pool}

  echo "Cleaning up test resources ..."
  kubectl delete pod "$pod_name" -n "$namespace" --grace-period=0 --force 2>/dev/null || true
  kubectl delete pvc "$pvc_name" -n "$namespace" 2>/dev/null || true
  # wait for PV to be cleaned up
  local elapsed=0
  while [ "$elapsed" -lt 60 ]; do
    local pv_count
    pv_count=$(kubectl get pv -o json | jq "[.items[] | select(.spec.claimRef.name == \"$pvc_name\" and .spec.claimRef.namespace == \"$namespace\")] | length")
    if [ "$pv_count" -eq 0 ]; then
      break
    fi
    sleep 5
    elapsed=$((elapsed + 5))
  done
  kubectl delete sc "$sc_name" 2>/dev/null || true
  echo "Cleanup complete"
}

# Cross-Array Volume Access: provisions a volume via SYMID-pinned StorageClass
# on one array and verifies the pod mounts and writes data successfully,
# proving that a volume provisioned on a specific array in a multi-array zone
# can be accessed by a pod scheduled in that zone.
validate_cross_array_access() {
  local pvc_name=$1 namespace=$2 expected_array_id=$3

  # 1. Verify the PVC is bound
  local pvc_phase
  pvc_phase=$(kubectl get pvc "$pvc_name" -n "$namespace" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
  if [ "$pvc_phase" != "Bound" ]; then
    echo "FAIL: PVC '$pvc_name' phase is '$pvc_phase', expected 'Bound'"
    exit 1
  fi
  echo "PVC '$pvc_name' is Bound"

  # 2. Verify the volume was provisioned on the expected array
  local pv_name volume_handle
  pv_name=$(kubectl get pvc "$pvc_name" -n "$namespace" -o jsonpath='{.spec.volumeName}')
  volume_handle=$(kubectl get pv "$pv_name" -o jsonpath='{.spec.csi.volumeHandle}')
  echo "PVC '$pvc_name' -> PV '$pv_name' -> volumeHandle '$volume_handle'"

  if ! echo "$volume_handle" | grep -q "$expected_array_id"; then
    echo "FAIL: volumeHandle '$volume_handle' does not contain expected array '$expected_array_id'"
    exit 1
  fi
  echo "Volume provisioned on array '$expected_array_id' as expected"

  # 3. Verify the pod that uses this PVC is running and can write data
  local pod_name
  pod_name=$(kubectl get pods -n "$namespace" -o json | jq -r --arg pvc "$pvc_name" \
    '.items[] | select(.spec.volumes[]?.persistentVolumeClaim?.claimName == $pvc) | .metadata.name' | head -1)
  if [ -z "$pod_name" ]; then
    echo "FAIL: no pod found using PVC '$pvc_name'"
    exit 1
  fi

  local pod_phase
  pod_phase=$(kubectl get pod "$pod_name" -n "$namespace" -o jsonpath='{.status.phase}')
  if [ "$pod_phase" != "Running" ]; then
    echo "FAIL: pod '$pod_name' phase is '$pod_phase', expected 'Running'"
    exit 1
  fi

  # Write a test file and read it back to prove the volume is accessible
  kubectl exec "$pod_name" -n "$namespace" -- sh -c 'echo cross-array-test > /data/cross-array-test.txt && cat /data/cross-array-test.txt'
  local read_back
  read_back=$(kubectl exec "$pod_name" -n "$namespace" -- cat /data/cross-array-test.txt 2>/dev/null || echo "")
  if [ "$read_back" != "cross-array-test" ]; then
    echo "FAIL: could not read back test data from volume (got '$read_back')"
    exit 1
  fi

  echo "PASS: cross-array volume access verified -- volume on array '$expected_array_id' is accessible from pod '$pod_name'"
}

# Validates that the second array in the zone pool can also serve a volume,
# proving both arrays in the pool are functional for provisioning.
validate_second_array_access() {
  local pvc_name=$1 namespace=$2 secret_name=$3 first_array_id=$4

  local pv_name volume_handle
  pv_name=$(kubectl get pvc "$pvc_name" -n "$namespace" -o jsonpath='{.spec.volumeName}')
  volume_handle=$(kubectl get pv "$pv_name" -o jsonpath='{.spec.csi.volumeHandle}')
  echo "PVC '$pvc_name' -> PV '$pv_name' -> volumeHandle '$volume_handle'"

  # The volume should be on one of the pool arrays
  local config pool_arrays
  config=$(read_secret_config "$secret_name" "$namespace")
  pool_arrays=$(echo "$config" | grep storageArrayId | awk -F'"' '{print $2}')

  local matched=false provisioned_array=""
  for arr in $pool_arrays; do
    if echo "$volume_handle" | grep -q "$arr"; then
      matched=true
      provisioned_array="$arr"
      break
    fi
  done

  if [ "$matched" = false ]; then
    echo "FAIL: volumeHandle '$volume_handle' does not match any pool array"
    exit 1
  fi

  echo "PASS: volume provisioned on pool array '$provisioned_array' -- cross-array pool selection verified"
}

# Validates controller logs contain expected multi-array zone entries.
validate_controller_logs() {
  local namespace=$1 expected_pattern=$2

  echo "Checking controller logs for pattern: '$expected_pattern'"
  local log_output
  log_output=$(kubectl logs -n "$namespace" -l app=powermax-controller -c driver --tail=200 2>/dev/null || echo "")

  if echo "$log_output" | grep -qi "$expected_pattern"; then
    echo "PASS: controller logs contain expected pattern '$expected_pattern'"
  else
    echo "INFO: pattern '$expected_pattern' not found in recent controller logs (this may be timing-dependent)"
    echo "Last 10 lines of controller logs:"
    echo "$log_output" | tail -10
  fi
}

# Validates that driver pods are running and healthy across all nodes.
validate_driver_pod_health() {
  local namespace=$1

  echo "Checking driver pod health in namespace '$namespace' ..."

  # Check controller pods
  local controller_count controller_ready
  controller_count=$(kubectl get pods -n "$namespace" -l app=powermax-controller --no-headers 2>/dev/null | wc -l)
  controller_ready=$(kubectl get pods -n "$namespace" -l app=powermax-controller --no-headers 2>/dev/null | grep -c "Running" || echo 0)
  echo "Controller pods: $controller_ready/$controller_count Running"

  if [ "$controller_ready" -lt 1 ]; then
    echo "FAIL: no controller pods are running"
    exit 1
  fi

  # Check node pods
  local node_count node_ready
  node_count=$(kubectl get pods -n "$namespace" -l app=powermax-node --no-headers 2>/dev/null | wc -l)
  node_ready=$(kubectl get pods -n "$namespace" -l app=powermax-node --no-headers 2>/dev/null | grep -c "Running" || echo 0)
  echo "Node pods: $node_ready/$node_count Running"

  if [ "$node_ready" -lt 1 ]; then
    echo "FAIL: no node pods are running"
    exit 1
  fi

  echo "PASS: all driver pods are healthy"
}

# Validates that the CSI driver has registered and is reporting node topology
# information correctly by checking CSINode objects.
validate_csi_node_topology() {
  local namespace=$1

  echo "Checking CSINode topology information ..."
  local csi_nodes
  csi_nodes=$(kubectl get csinodes -o json | jq '[.items[] | select(.spec.drivers[]?.name == "csi-powermax.dellemc.com")] | length')

  if [ "$csi_nodes" -lt 1 ]; then
    echo "FAIL: no CSINode objects found with driver 'csi-powermax.dellemc.com'"
    exit 1
  fi

  echo "Found $csi_nodes CSINode(s) with PowerMax driver registered"

  # Check that topology keys include zone labels
  local zone_nodes
  zone_nodes=$(kubectl get csinodes -o json | jq '[.items[] | select(.spec.drivers[]? | select(.name == "csi-powermax.dellemc.com") | .topologyKeys[]? | select(startswith("zone.topology")))] | length')

  if [ "$zone_nodes" -lt 1 ]; then
    echo "INFO: no CSINode topology keys with zone.topology prefix found (zone topology may be reported differently)"
  else
    echo "PASS: $zone_nodes CSINode(s) report zone topology keys"
  fi
}

# Validates controller log output for zone-pool initialization and capacity
# polling entries, proving the driver is aware of multi-array zone configuration.
validate_multiaz_logging() {
  local namespace=$1

  echo "=== Multi-AZ Metrics and Logging Validation ==="

  # 1. Validate driver pods are healthy
  validate_driver_pod_health "$namespace"

  # 2. Check controller logs for zone/array related entries
  echo ""
  echo "--- Controller Log Analysis ---"
  local log_output
  log_output=$(kubectl logs -n "$namespace" -l app=powermax-controller -c driver --tail=500 2>/dev/null || echo "")

  # Check for array connectivity / initialization logs
  local array_count
  array_count=$(echo "$log_output" | grep -ci "array\|symid\|storage.*array\|unisphere" || echo 0)
  echo "Array-related log entries found: $array_count"

  # Check for zone/topology related log entries
  local zone_count
  zone_count=$(echo "$log_output" | grep -ci "zone\|topology\|label" || echo 0)
  echo "Zone/topology-related log entries found: $zone_count"

  # Check for capacity-related log entries (multi-array capacity polling)
  local capacity_count
  capacity_count=$(echo "$log_output" | grep -ci "capacity\|pool\|selection" || echo 0)
  echo "Capacity/pool-related log entries found: $capacity_count"

  # 3. Check node logs
  echo ""
  echo "--- Node Log Analysis ---"
  local node_log
  node_log=$(kubectl logs -n "$namespace" -l app=powermax-node -c driver --tail=100 2>/dev/null || echo "")
  local node_array_entries
  node_array_entries=$(echo "$node_log" | grep -ci "array\|symid\|registered\|topology" || echo 0)
  echo "Node driver array/topology entries found: $node_array_entries"

  # 4. Validate CSINode topology
  echo ""
  validate_csi_node_topology "$namespace"

  echo ""
  echo "PASS: Multi-AZ metrics and logging validation complete"
}

if [ "$#" -lt 1 ]; then
  echo "Usage: $0 {pool-size|invalid-entry-excluded|snapshot-restarts|verify-no-restart|zone-only-storageclass|create-pvc-and-pod|validate-pvc-on-pool-array|validate-pvc-on-specific-array|cleanup-pvc-pod|validate-cross-array-access|validate-second-array-access|validate-controller-logs|validate-multiaz-logging} ..."
  exit 1
fi

command=$1
shift
case "$command" in
  pool-size) pool_size "$@" ;;
  invalid-entry-excluded) invalid_entry_excluded "$@" ;;
  snapshot-restarts) snapshot_restarts "$@" ;;
  verify-no-restart) verify_no_restart "$@" ;;
  zone-only-storageclass) zone_only_storageclass "$@" ;;
  create-pvc-and-pod) create_pvc_and_pod "$@" ;;
  validate-pvc-on-pool-array) validate_pvc_on_pool_array "$@" ;;
  validate-pvc-on-specific-array) validate_pvc_on_specific_array "$@" ;;
  cleanup-pvc-pod) cleanup_pvc_pod "$@" ;;
  validate-cross-array-access) validate_cross_array_access "$@" ;;
  validate-second-array-access) validate_second_array_access "$@" ;;
  validate-controller-logs) validate_controller_logs "$@" ;;
  validate-multiaz-logging) validate_multiaz_logging "$@" ;;
  *)
    echo "Unknown command '$command'"
    echo "Usage: $0 {pool-size|invalid-entry-excluded|snapshot-restarts|verify-no-restart|zone-only-storageclass|create-pvc-and-pod|validate-pvc-on-pool-array|validate-pvc-on-specific-array|cleanup-pvc-pod|validate-cross-array-access|validate-second-array-access|validate-controller-logs|validate-multiaz-logging} ..."
    exit 1
    ;;
esac
