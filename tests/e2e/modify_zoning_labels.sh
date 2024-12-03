# Copyright © 2024 Dell Inc. or its subsidiaries. All Rights Reserved.
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
# ./modify_zoning_labels.sh add <zone>
# To remove a zone label:
# ./modify_zoning_labels.sh remove <label>
# To remove all zone labels:
# ./modify_zoning_labels.sh remove-all-zones

# get all worker node names in the cluster
get_worker_nodes() {
  kubectl get nodes -A | grep -v -E 'mast.r|control-plane'  | grep -v NAME | awk '{ print $1 }'
}

# add zone label to all worker nodes
add_zone_label() {
  local zone=$1
  for node in $(get_worker_nodes); do
    kubectl label nodes $node zone=$zone --overwrite
    echo "Added zone label '$zone' to $node"
  done
}

# remove zone label from worker nodes
remove_zone_label() {
  local label=$1
  for node in $(get_worker_nodes); do
    kubectl label nodes $node $label-
    echo "Removed label '$label' from $node"
  done
}

# remove all labels from worker nodes
remove_all_zone_labels() {
  for node in $(get_worker_nodes); do
    labels=$(kubectl get node $node -o jsonpath='{.metadata.labels}' | jq -r 'keys[]')
    for label in $labels; do
    // TODO: might have to adjust this based on the actual zone label name
    // this will remove all labels that start with "zone"
    if [[ $label == zone* ]]; then
        kubectl label nodes $node $label-
        echo "Removed label '$label' from $node"
    fi
    done
  done
}

if [ "$#" -lt 1 ]; then
  echo "Usage: $0 add <zone> | remove <label> | remove-all-zones"
  exit 1
fi

action=$1

case $action in
  add)
    if [ "$#" -ne 2 ]; then
      echo "Usage: $0 add <zone>"
      exit 1
    fi
    zone=$2
    add_zone_label $zone
    ;;
  remove)
    if [ "$#" -ne 2 ]; then
      echo "Usage: $0 remove <label>"
      exit 1
    fi
    label=$2
    remove_zone_label $label
    ;;
  remove-all-zones)
    remove_all_zone_labels
    ;;
  *)
    echo "Invalid action: $action"
    echo "Usage: $0 add <zone> | remove <label> | remove-all-zones"
    exit 1
    ;;
esac

exit 0
