#!/bin/bash

# Copyright 2025-2026 DELL Inc. or its subsidiaries.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script updates CSI sidecar versions across CSM-operator files
set -ex

CONFIG_DIR="operatorconfig/driverconfig/common"

# Helper to fetch latest tag from registry.k8s.io for a given image name
get_latest_registry_tag() {
  local image=$1
  curl -sL "https://registry.k8s.io/v2/sig-storage/${image}/tags/list" \
    | jq -r '.tags | map(select(test("^v?[0-9]"))) | .[]' \
    | sort -V \
    | tail -n 1
}

# Can be used to bump the csi-metadata-retriever closer to a release
#
# Increment minor version: vX.Y[.Z] -> vX.(Y+1).0
# increment_minor() {
#   local tag="$1"

#   if [[ "$tag" =~ ^(v?)([0-9]+)\.([0-9]+)(\.([0-9]+))?$ ]]; then
#     local vprefix="${BASH_REMATCH[1]}"
#     local major="${BASH_REMATCH[2]}"
#     local minor="${BASH_REMATCH[3]}"
#     # local patch="${BASH_REMATCH[5]}"  # captured if present, but reset to 0 on minor bump
#     local new_minor=$((minor + 1))
#     echo "${vprefix}${major}.${new_minor}.0"
#   else
#     echo "Error: tag '$tag' is not a recognized semver (vX.Y[.Z])" >&2
#     return 1
#   fi
# }

# Fetch latest tags
latest_attacher_tag=$(get_latest_registry_tag "csi-attacher")
latest_provisioner_tag=$(get_latest_registry_tag "csi-provisioner")
latest_snapshotter_tag=$(get_latest_registry_tag "csi-snapshotter")
latest_registrar_tag=$(get_latest_registry_tag "csi-node-driver-registrar")
latest_resizer_tag=$(get_latest_registry_tag "csi-resizer")
latest_healthmonitor_tag=$(get_latest_registry_tag "csi-external-health-monitor-controller")

latest_sdc_quay_tag=$(curl -sL "https://quay.io/api/v1/repository/dell/storage/powerflex/sdc/tag/" \
  | jq -r '.tags[]?.name' \
  | grep -E '^[0-9]+\.[0-9]+(\.[0-9]+)?$' \
  | sort -V | tail -n 1)

latest_meta_tag=$(curl -sL "https://quay.io/api/v1/repository/dell/container-storage-modules/csi-metadata-retriever/tag/" \
  | jq -r '.tags[]?.name' \
  | grep -E '^v?[0-9]+\.[0-9]+(\.[0-9]+)?$' \
  | sort -V | tail -n 1)
# bumped_meta_tag=$(increment_minor "$latest_meta_tag")

echo "Latest tags fetched."

# Get top 3 latest k8s YAML files
top_k8s_files=$(find "$CONFIG_DIR" -maxdepth 1 -type f -name "k8s-*-values.yaml" \
  | sed -E 's|.*/k8s-([0-9]+)\.([0-9]+)-values\.yaml|\1.\2 &|' \
  | sort -Vr \
  | head -n 3 \
  | awk '{print $2}')

# Update all files with latest tag
for sidecar in attacher provisioner snapshotter registrar resizer external-health-monitor sdc metadata-retriever; do
  echo "Updating sidecar version for $sidecar"

  old_sidecar_ver=$(grep "$sidecar" "$CONFIG_DIR/default.yaml" | egrep 'registry.k8s.io|quay.io' | awk '{print $2}')
  old_sidecar_sub_string=$(echo "$old_sidecar_ver" | awk -F':' '{print $1}')

  if [[ -n "$old_sidecar_ver" ]]; then
      files_to_be_modified=$(grep -rl "$old_sidecar_ver")
  else
      echo "No old version found for $sidecar"
      continue
  fi

  for file in $files_to_be_modified; do
    case $sidecar in
      attacher) new_ver=$latest_attacher_tag ;;
      provisioner) new_ver=$latest_provisioner_tag ;;
      snapshotter) new_ver=$latest_snapshotter_tag ;;
      registrar) new_ver=$latest_registrar_tag ;;
      resizer) new_ver=$latest_resizer_tag ;;
      external-health-monitor) new_ver=$latest_healthmonitor_tag ;;
      sdc) new_ver=$latest_sdc_quay_tag ;;
      metadata-retriever) new_ver=$latest_meta_tag ;;
    esac

    if [[ -n "$old_sidecar_ver" ]]; then
      sed -i "s|${old_sidecar_ver}|${old_sidecar_sub_string}:${new_ver}|g" "$file"
      echo "Updated $sidecar from $old_sidecar_ver to ${old_sidecar_sub_string}:${new_ver}"
    else
      echo "No match found for $sidecar in $file"
    fi
  done
done

# Update operatorconfig/driverconfig/common directory
# Always include default.yaml
files_to_update=("$CONFIG_DIR/default.yaml")
for f in $top_k8s_files; do
  files_to_update+=("$f")
done

# Update selected files
for file in "${files_to_update[@]}"; do
  echo "Updating $file"

  sed -i -E "s|(registry.k8s.io/sig-storage/csi-attacher:)[^ ]+|\1${latest_attacher_tag}|" "$file"
  sed -i -E "s|(registry.k8s.io/sig-storage/csi-provisioner:)[^ ]+|\1${latest_provisioner_tag}|" "$file"
  sed -i -E "s|(registry.k8s.io/sig-storage/csi-snapshotter:)[^ ]+|\1${latest_snapshotter_tag}|" "$file"
  sed -i -E "s|(registry.k8s.io/sig-storage/csi-node-driver-registrar:)[^ ]+|\1${latest_registrar_tag}|" "$file"
  sed -i -E "s|(registry.k8s.io/sig-storage/csi-resizer:)[^ ]+|\1${latest_resizer_tag}|" "$file"
  sed -i -E "s|(registry.k8s.io/sig-storage/csi-external-health-monitor-controller:)[^ ]+|\1${latest_healthmonitor_tag}|" "$file"

  sed -i -E "s|(quay.io/dell/storage/powerflex/sdc:)[^ ]+|\1${latest_sdc_quay_tag}|" "$file"
  sed -i -E "s|(quay.io/dell/container-storage-modules/csi-metadata-retriever:)[^ ]+|\1${latest_meta_tag}|" "$file"

  echo "Updated $file"
done

echo "All files updated."
