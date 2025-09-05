#!/bin/bash
set -euo pipefail

CONFIG_DIR="operatorconfig/driverconfig/common"

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

latest_sdc_tag=$(curl -s "https://hub.docker.com/v2/repositories/dellemc/sdc/tags?page_size=100" \
  | jq -r '.results[].name' \
  | grep -E '^[0-9]+\.[0-9]+(\.[0-9]+)?$' \
  | sort -V | tail -n 1)

latest_sdc_quay_tag=$(curl -sL "https://quay.io/v2/dell/storage/powerflex/sdc/tags/list" \
  | jq -r '.tags[]?' \
  | grep -E '^[0-9]+\.[0-9]+(\.[0-9]+)?$' \
  | sort -V | tail -n 1)

latest_meta_tag=$(curl -sL "https://quay.io/v2/dell/container-storage-modules/csi-metadata-retriever/tags/list" \
  | jq -r '.tags[]?' \
  | grep -E '^v?[0-9]+\.[0-9]+(\.[0-9]+)?$' \
  | sort -V | tail -n 1)

echo "Latest tags fetched."

# Get top 3 latest k8s YAML files
top_k8s_files=$(find "$CONFIG_DIR" -maxdepth 1 -type f -name "k8s-*-values.yaml" \
  | sed -E 's|.*/k8s-([0-9]+)\.([0-9]+)-values\.yaml|\1.\2 &|' \
  | sort -Vr \
  | head -n 3 \
  | awk '{print $2}')

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

  sed -i -E "s|(dellemc/sdc:)[^ ]+|\1${latest_sdc_tag}|" "$file"
  sed -i -E "s|(quay.io/dell/storage/powerflex/sdc:)[^ ]+|\1${latest_sdc_quay_tag}|" "$file"
  sed -i -E "s|(quay.io/dell/container-storage-modules/csi-metadata-retriever:)[^ ]+|\1${latest_meta_tag}|" "$file"
  sed -i -E "s|(dellemc/csi-metadata-retriever:)[^ ]+|\1${latest_meta_tag}|" "$file"

  echo "? Updated $file"
done

echo "?? All selected files updated."
