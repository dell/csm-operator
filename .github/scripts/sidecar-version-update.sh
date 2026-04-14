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
# It only updates files that match the CSM version to avoid modifying released configs
set -ex

CONFIG_DIR="operatorconfig/driverconfig/common"
MAPPING_FILE="operatorconfig/common/csm-version-mapping.yaml"

# Get parameters from workflow
CSM_VERSION="${1:-}"
CSI_METADATA_RETRIEVER_VERSION="${2:-}"

if [[ -z "$CSM_VERSION" ]]; then
  echo "Error: CSM_VERSION is required"
  exit 1
fi

# Helper to fetch latest tag from registry.k8s.io for a given image name
get_latest_registry_tag() {
  local image=$1
  curl -sL "https://registry.k8s.io/v2/sig-storage/${image}/tags/list" \
    | jq -r '.tags | map(select(test("^v?[0-9]"))) | .[]' \
    | sort -V \
    | tail -n 1
}

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

# Use provided csi-metadata-retriever version or fetch from registry
if [[ -n "$CSI_METADATA_RETRIEVER_VERSION" ]]; then
  latest_meta_tag="$CSI_METADATA_RETRIEVER_VERSION"
else
  latest_meta_tag=$(curl -sL "https://quay.io/api/v1/repository/dell/container-storage-modules/csi-metadata-retriever/tag/" \
    | jq -r '.tags[]?.name' \
    | grep -E '^v?[0-9]+\.[0-9]+(\.[0-9]+)?$' \
    | sort -V | tail -n 1)
fi

echo "Latest tags fetched."
echo "CSM Version: $CSM_VERSION"

# Read the csm-version-mapping to get driver versions for this CSM version
if [[ ! -f "$MAPPING_FILE" ]]; then
  echo "Error: Mapping file not found: $MAPPING_FILE"
  exit 1
fi

# Extract driver versions for the current CSM version
declare -A driver_versions
for driver in powerflex powermax powerscale powerstore unity cosi; do
  driver_version=$(grep -A 100 "^${driver}:" "$MAPPING_FILE" | grep "${CSM_VERSION}:" | sed -E 's/.*"(v[0-9]+\.[0-9]+\.[0-9]+)".*/\1/' | head -1)
  if [[ -n "$driver_version" ]]; then
    driver_versions[$driver]="$driver_version"
    echo "Driver $driver: $driver_version"
  fi
done

# Find ALL files with sidecar references
echo "Finding all files with sidecar references..."
all_files=$(find . -name "*.yaml" -o -name "*.yml" 2>/dev/null | xargs grep -l "registry.k8s.io/sig-storage\|quay.io/dell/container-storage-modules" 2>/dev/null | sort -u)

# Get top 3 latest k8s YAML files from common config
all_k8s_files=$(find "$CONFIG_DIR" -maxdepth 1 -type f -name "k8s-*-values.yaml" \
  | sed -E 's|.*/k8s-([0-9]+)\.([0-9]+)-values\.yaml|\1.\2 &|' \
  | sort -Vr \
  | awk '{print $2}')
top_k8s_files=$(echo "$all_k8s_files" | head -n 3)

# Filter files to only include those that match the CSM version's driver versions
files_to_update=()

# Always include default.yaml
files_to_update+=("$CONFIG_DIR/default.yaml")
files_to_update+=("testdata/default.yaml")

# Always include top 3 k8s YAML files
for f in $top_k8s_files; do
  files_to_update+=("$f")
done

# Process all other files
for file in $all_files; do
  # Skip files in .git, vendor, and other non-relevant directories
  if [[ "$file" =~ \.git/ ]] || [[ "$file" =~ /vendor/ ]]; then
    continue
  fi

  should_update=0

  # Check if file matches any of the driver versions for this CSM version
  for driver in "${!driver_versions[@]}"; do
    driver_version="${driver_versions[$driver]}"

    # Check if file path contains the driver version
    if [[ "$file" =~ $driver_version ]]; then
      should_update=1
      break
    fi

    # Check if file is a sample or test file that references the driver version
    if [[ "$file" =~ samples/ ]] || [[ "$file" =~ tests/ ]]; then
      # For sample/test files, check if they contain the driver version or csm version in their content
      if grep -q "$driver_version" "$file" 2>/dev/null || grep -q "$CSM_VERSION" "$file" 2>/dev/null; then
        should_update=1
        break
      fi
    fi

    # Check if file is a config/manager file (operator deployment)
    if [[ "$file" =~ config/manager/ ]] || [[ "$file" =~ config/manifests/ ]] || [[ "$file" =~ deploy/ ]]; then
      should_update=1
      break
    fi

    # Check if file is a bundle/catalog file (operator deployment)
    if [[ "$file" =~ bundle/ ]] || [[ "$file" =~ catalog/ ]]; then
      should_update=1
      break
    fi
  done

  if [[ $should_update -eq 1 ]]; then
    files_to_update+=("$file")
  fi
done

echo "Candidates for update: ${#files_to_update[@]} files"

# Define sidecar patterns and their corresponding versions
declare -A sidecar_patterns=(
  ["registry.k8s.io/sig-storage/csi-attacher"]="${latest_attacher_tag}"
  ["registry.k8s.io/sig-storage/csi-provisioner"]="${latest_provisioner_tag}"
  ["registry.k8s.io/sig-storage/csi-snapshotter"]="${latest_snapshotter_tag}"
  ["registry.k8s.io/sig-storage/csi-node-driver-registrar"]="${latest_registrar_tag}"
  ["registry.k8s.io/sig-storage/csi-resizer"]="${latest_resizer_tag}"
  ["registry.k8s.io/sig-storage/csi-external-health-monitor-controller"]="${latest_healthmonitor_tag}"
  ["quay.io/dell/storage/powerflex/sdc"]="${latest_sdc_quay_tag}"
  ["quay.io/dell/container-storage-modules/csi-metadata-retriever"]="${latest_meta_tag}"
)

# Helper function to update sidecar versions in a file, scoped to a specific CSM version
# This is used for files with multiple version blocks (e.g., configmaps)
update_file_for_csm_version() {
  local file=$1
  local csm_ver=$2
  local file_changed=0
  local in_version_block=0
  local current_version=""
  local output=""

  while IFS= read -r line; do
    # Check if this line starts a version block (e.g., "- version: v1.17.0")
    if [[ "$line" =~ ^[[:space:]]*-[[:space:]]*version:[[:space:]]*(.+)$ ]]; then
      current_version="${BASH_REMATCH[1]}"
      in_version_block=1
      output+="$line"$'\n'
      continue
    fi

    # Check if we're exiting a version block (next top-level item)
    if [[ "$in_version_block" == 1 ]] && [[ "$line" =~ ^[[:space:]]*- ]] && [[ ! "$line" =~ version: ]]; then
      in_version_block=0
      current_version=""
    fi

    # If we're in a version block that matches our CSM version, update sidecar references
    if [[ "$in_version_block" == 1 ]] && [[ "$current_version" == "$csm_ver" ]]; then
      local updated_line="$line"
      for pattern in "${!sidecar_patterns[@]}"; do
        new_version="${sidecar_patterns[$pattern]}"
        updated_line=$(echo "$updated_line" | sed -E "s|(${pattern}:)[^ \"]+|\1${new_version}|g")
      done

      if [[ "$updated_line" != "$line" ]]; then
        file_changed=1
      fi
      output+="$updated_line"$'\n'
    else
      # Not in matching version block, keep line as-is
      output+="$line"$'\n'
    fi
  done < "$file"

  # If file was modified, write the output back to the file
  if [[ $file_changed -eq 1 ]]; then
    echo -n "$output" > "$file"
    return 0
  else
    return 1
  fi
}

# Update all files with latest tags and track which ones actually changed
updated_files=()

for file in "${files_to_update[@]}"; do
  # Check if file contains version blocks (like csm-images.yaml)
  if grep -q "^[[:space:]]*-[[:space:]]*version:" "$file" 2>/dev/null; then
    # File has version blocks, use scoped update function
    if update_file_for_csm_version "$file" "$CSM_VERSION"; then
      updated_files+=("$file")
      echo "Updated: $file (scoped to CSM version $CSM_VERSION)"
    fi
  else
    # File doesn't have version blocks, update all sidecar references
    file_changed=0

    # Check if file would be modified by testing each sidecar pattern
    for pattern in "${!sidecar_patterns[@]}"; do
      new_version="${sidecar_patterns[$pattern]}"
      if sed -E "s|(${pattern}:)[^ \"]+|\1${new_version}|g" "$file" | grep -q "${new_version}"; then
        file_changed=1
        break
      fi
    done

    # Only apply changes if file would be modified
    if [[ $file_changed -eq 1 ]]; then
      for pattern in "${!sidecar_patterns[@]}"; do
        new_version="${sidecar_patterns[$pattern]}"
        sed -i -E "s|(${pattern}:)[^ \"]+|\1${new_version}|g" "$file"
      done

      updated_files+=("$file")
      echo "Updated: $file"
    fi
  fi
done

echo ""
echo "Summary: ${#updated_files[@]} files were updated"
