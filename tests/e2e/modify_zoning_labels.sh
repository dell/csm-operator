# Copyright © 2024-2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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

# This script is used as a command line argument in the e2e CustomTest section. It will
# add and remove zone labels for an e2e scenario.
#
# To add a zone label:
# ./modify_zoning_labels.sh add <zone1> <zone2> ...
# To remove a zone label:
# ./modify_zoning_labels.sh remove <zone-key>
# To remove all zone labels:
# ./modify_zoning_labels.sh remove-all-zones
# To validate the zone labels:
# ./modify_zoning_labels.sh validate-zoning

# get all worker node names in the cluster
get_worker_nodes() {
  kubectl get nodes -A | grep -v -E 'master|control-plane'  | grep -v NAME | awk '{ print $1 }'
}

# add zone label to all worker nodes
add_zone_label() {
  local zones=("$@")
  local index=0
  for node in $(get_worker_nodes); do
    local zone=${zones[$index]}
    if kubectl label nodes $node $zone; then
      echo "Added zone label '$zone' to $node"
    else
      echo "Failed to add zone label '$zone' to $node"
    fi

    index=$((index + 1))
    # reset the index if we reach the end of the zones array
    if [ $index -ge ${#zones[@]} ]; then
        index=0
    fi
  done
}

# remove a specific zone label from worker nodes
remove_zone_label() {
  local zone=$1
  nodes=$(kubectl get nodes -l $zone -o name)
  if [ -z "$nodes" ]; then
    echo "No nodes found with zone '$zone'"
    return
  fi

  for node in $(get_worker_nodes); do
    if kubectl label nodes $node $zone-; then
      echo "Removed label '$zone' from $node"
    else
      echo "Failed to remove label '$zone' from $node"
    fi
  done
}

# remove all labels from worker nodes starting with zone
remove_all_zone_labels() {
  for node in $(get_worker_nodes); do
    labels=$(kubectl get node $node -o jsonpath='{.metadata.labels}' | jq -r 'keys[]')

    for label in $labels; do
    # this will remove all labels that start with "zone"
    if [[ $label == zone* ]]; then
        if kubectl label nodes $node $label-; then
          echo "Removed label '$label' from $node"
        else
          echo "Failed to remove label '$label' from $node"
        fi
    fi
    done
  done
}

# validation part of script

# read the Kubernetes secret and extract zone information
read_secret() {
  secret_name=$1
  driverNamespace=$2
  secret_data=$(kubectl get secret $secret_name -n $driverNamespace -o jsonpath='{.data.config}')
  echo $secret_data | base64 --decode
}

# validating zoning is configured on the cluster - powermax
validate_zoning_powermax() {
  # read the secret and extract zone information
  secret_name="powermax-config"
  namespace="powermax"
  secret_content=$(read_secret $secret_name $namespace)

  # specify the zone and region label keys
  zone_label_key="zone.topology.kubernetes.io/zone"
  region_label_key="zone.topology.kubernetes.io/region"

  echo "Using label keys: $zone_label_key and $region_label_key"

  local zones=()
  while IFS= read -r line; do
      if [[ $line =~ $zone_label_key: ]]; then
          zone=$(echo "${line##* }" | tr -d '"')
          zones+=("$zone")
      fi
      if [[ $line =~ $region_label_key: ]]; then
          zone=$(echo "${line##* }" | tr -d '"')
          zones+=("$zone")
      fi
  done <<< "$(echo "$secret_content"  | grep 'zone')"

  echo "Configured zones in secret: ${zones[@]}"

  # list all pods in the driver namespace
  pods=$(kubectl get pods -n $namespace -o jsonpath='{.items[*].metadata.name}')

  # create a map of pods and the nodes they're running on
  declare -A pod_node_map
  for pod in $pods; do
    # check for running pods
    status=$(kubectl get pod $pod -n $namespace -o jsonpath='{.status.phase}')
    if [ "$status" == "Running" ]; then
      # get the node name
      node=$(kubectl get pod $pod -n $namespace -o jsonpath='{.spec.nodeName}')
      pod_node_map[$pod]=$node
    else
      echo "Pod $pod is not running. Status: $status"
      exit 1
    fi
  done

  # query the nodes to ensure they have zone labels
  for node in "${pod_node_map[@]}"; do
    echo "Checking node: $node"
    getLabel=$(kubectl get node $node -o jsonpath="{.metadata.labels}")
    zone_label=$(echo "$getLabel" | jq -r ".\"$zone_label_key\"")

    echo "Node $node zone label: $zone_label"

    match_found=false
    for zone in "${zones[@]}"; do
      echo "Comparing node zone label '$zone_label' with expected zone '$zone'"
      if [ "$zone" == "$zone_label" ]; then
        match_found=true
        echo "Node $node has a matching zone label: $zone_label"
        break
      fi
    done

    if [ "$match_found" == false ]; then
      echo "Node $node does not have a matching zone label: $zone_label"
      exit 1
    fi

    # repeat for secondary label
    getLabel=$(kubectl get node $node -o jsonpath="{.metadata.labels}")
    zone_label=$(echo "$getLabel" | jq -r ".\"$region_label_key\"")

    echo "Node $node zone label: $zone_label"

    match_found=false
    for zone in "${zones[@]}"; do
      echo "Comparing node zone label '$zone_label' with expected zone '$zone'"
      if [ "$zone" == "$zone_label" ]; then
        match_found=true
        echo "Node $node has a matching zone label: $zone_label"
        break
      fi
    done

    if [ "$match_found" == false ]; then
      echo "Node $node does not have a matching zone label: $zone_label"
      exit 1
    fi
  done

  echo "All zones on worker nodes match expected values from secret."
}

# validating zoning is configured on the cluster - powerflex
validate_zoning_powerflex() {
  # read the secret and extract zone information
  secret_name="test-vxflexos-config"
  namespace="test-vxflexos"
  secret_content=$(read_secret $secret_name $namespace)

  # parse the secret content to extract zones
  local zones=()
  while IFS= read -r line; do
    if [[ $line =~ name: ]]; then
      zone=$(echo "${line##* }" | tr -d '"')
      zones+=("$zone")
    fi
  done <<< "$(echo "$secret_content" | grep -A 1 'zone:' | grep 'name:')"
  echo "$secret_content" | grep -A 1 'zone:' | grep 'name:'

  echo "Configured zones in secret: ${zones[@]}"

  # list all pods in the driver namespace
  pods=$(kubectl get pods -n $namespace -o jsonpath='{.items[*].metadata.name}')

  # create a map of pods and the nodes they're running on
  declare -A pod_node_map
  for pod in $pods; do
    # check for running pods
    status=$(kubectl get pod $pod -n $namespace -o jsonpath='{.status.phase}')
    if [ "$status" == "Running" ]; then
      # get the node name
      node=$(kubectl get pod $pod -n $namespace -o jsonpath='{.spec.nodeName}')
      pod_node_map[$pod]=$node
    else
      echo "Pod $pod is not running. Status: $status"
      exit 1
    fi
  done

  # query the nodes to ensure they have zone labels
  for node in "${pod_node_map[@]}"; do
    echo "Checking node: $node"
    getLabel=$(kubectl get node $node -o jsonpath="{.metadata.labels}")
    zone_label=$(echo "$getLabel" | jq -r '.["zone.csi-vxflexos.dellemc.com"]')

    echo "Node $node zone label: $zone_label"

    match_found=false
    for zone in "${zones[@]}"; do
      echo "Comparing node zone label '$zone_label' with expected zone '$zone'"
      if [ "$zone" == "$zone_label" ]; then
        match_found=true
        echo "Node $node has a matching zone label: $zone_label"
        break
      fi
    done

    if [ "$match_found" == false ]; then
      echo "Node $node does not have a matching zone label: $zone_label"
      exit 1
    fi
  done
}

if [ "$#" -lt 1 ]; then
  echo "Usage: $0 add <zone1> <zone2> ... | remove <label> | remove-all-zones | validate-zoning"
  exit 1
fi

action=$1
shift

case $action in
  add)
    if [ "$#" -lt 1 ]; then
      echo "Usage: $0 add <zone1> <zone2> ..."
      exit 1
    fi
    add_zone_label "$@"
    ;;
  remove)
    if [ "$#" -ne 1 ]; then
      echo "Usage: $0 remove <zone>"
      exit 1
    fi
    zone=$1
    remove_zone_label $zone
    ;;
  remove-all-zones)
    remove_all_zone_labels
    ;;
  validate-zoning)
    if [ "$#" -ne 1 ]; then
      echo "Usage: $0 validate-zoning powermax|powerflex|powerscale|powerstore|unity"
      exit 1
    fi
    driver=$1
    shift
    case $driver in
      powermax)
        validate_zoning_powermax
        ;;
      powerflex)
        validate_zoning_powerflex
        ;;
      *)
        echo "Unsupported driver: $driver"
        ;;
    esac
    ;;
  *)
    echo "Invalid action: $action"
    echo "Usage: $0 add <zone1> <zone2> ... | remove <label> | remove-all-zones | validate-zoning"
    exit 1
    ;;
esac

exit 0