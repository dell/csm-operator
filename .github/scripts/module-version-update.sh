#!/bin/bash

# Copyright 2025 DELL Inc. or its subsidiaries.
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

# Reading actual release version from csm repository
obs_ver="$KARAVI_OBSERVABILITY"
auth_v2="$CSM_AUTHORIZATION_V2"
rep_ver="$CSM_REPLICATION"
res_ver="$KARAVI_RESILIENCY"
revproxy_ver="$CSIREVERSEPROXY"
csm_ver="$CSM_VERSION"
pscale_matrics="$CSM_METRICS_POWERSCALE"
pflex_matrics="$KARAVI_METRICS_POWERFLEX"
pmax_matrics="$CSM_METRICS_POWERMAX"
pstore_matrics="$CSM_METRICS_POWERSTORE"
otel_col="$OTEL_COLLECTOR"
pscale_driver_ver="$CSI_POWERSCALE"
pstore_driver_ver="$CSI_POWERSTORE"
pmax_driver_ver="$CSI_POWERMAX"
pflex_driver_ver="$CSI_VXFLEXOS"

dell_csi_replicator="$CSM_REPLICATION"
dell_replication_controller="$CSM_REPLICATION"

obs_ver="$(echo -e "${obs_ver}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
auth_v2="$(echo -e "${auth_v2}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
rep_ver="$(echo -e "${rep_ver}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
res_ver="$(echo -e "${res_ver}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
revproxy_ver="$(echo -e "${revproxy_ver}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
csm_ver="$(echo -e "${csm_ver}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
pscale_matrics="$(echo -e "${pscale_matrics}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
pflex_matrics="$(echo -e "${pflex_matrics}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
pmax_matrics="$(echo -e "${pmax_matrics}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
pstore_matrics="$(echo -e "${pstore_matrics}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
otel_col="$(echo -e "${otel_col}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
dell_csi_replicator="$(echo -e "${dell_csi_replicator}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
dell_replication_controller="$(echo -e "${dell_replication_controller}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"

pscale_driver_ver=${pscale_driver_ver//./}
pstore_driver_ver=${pstore_driver_ver//./}
pmax_driver_ver=${pmax_driver_ver//./}
pflex_driver_ver=${pflex_driver_ver//./}

pscale_driver_ver="$(echo -e "${pscale_driver_ver}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
pstore_driver_ver="$(echo -e "${pstore_driver_ver}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
pmax_driver_ver="$(echo -e "${pmax_driver_ver}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
pflex_driver_ver="$(echo -e "${pflex_driver_ver}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"

update_config_version_yq() {
   local f="$1"
   local module_name="$2"
   local new_version="$3"

   [[ -f "$f" ]] || { echo "skip (missing): $f"; return 0; }
   command -v yq >/dev/null 2>&1 || { echo "ERROR: yq (mikefarah v4) not found"; return 1; }

   # 1) Module exists?
   if ! MODULE_NAME="$module_name" yq -e '
      (.. | select(type == "!!map" and has("name") and .name == strenv(MODULE_NAME)))
   ' "$f" >/dev/null 2>&1; then
      return 0
   fi

   # 2) Module has configVersion?
   if ! MODULE_NAME="$module_name" yq -e '
      (.. | select(
         type == "!!map" and has("name") and .name == strenv(MODULE_NAME) and has("configVersion")
      ))
   ' "$f" >/dev/null 2>&1; then
      return 0
   fi

   # 3) Current value — emit only one match; no head in a pipeline
   local CURRENT_VER
   CURRENT_VER="$(
      MODULE_NAME="$module_name" \
      yq -r '
      # collect matches into an array, then pick element 0 (first)
      [
         (.. | select(
            type == "!!map"
            and has("name")
            and .name == strenv(MODULE_NAME)
            and has("configVersion")
         ).configVersion)
      ][0] // ""
      ' "$f"
   )"

   # 4) Skip if already desired
   [[ "$CURRENT_VER" == "$new_version" ]] && return 0

   # 5) Final guard: only update if assignable nodes exist
   if MODULE_NAME="$module_name" yq -e '
      (.. | select(
         type == "!!map" and has("name") and .name == strenv(MODULE_NAME) and has("configVersion")
      ))
   ' "$f" >/dev/null 2>&1; then
      MODULE_NAME="$module_name" NEW_VERSION="$new_version" \
      yq -i '
         (.. | select(
         type == "!!map"
         and has("name")
         and .name == strenv(MODULE_NAME)
         and has("configVersion")
         )) |= (.configVersion = strenv(NEW_VERSION))
      ' "$f" 2>/dev/null
   fi
}



semver_n_minus_one() {
   local ver="$1"             # e.g., "v2.4.0"
   local prefix=""
   local core="$ver"

   # Preserve leading 'v' if present
   if [[ "$core" =~ ^v(.*)$ ]]; then
      prefix="v"
      core="${BASH_REMATCH[1]}"
   fi

   # Parse X.Y.Z
   local major minor patch
   IFS='.' read -r major minor patch <<< "$core"

   # Basic validation
   if [[ -z "$major" || -z "$minor" || -z "$patch" || ! "$major" =~ ^[0-9]+$ || ! "$minor" =~ ^[0-9]+$ || ! "$patch" =~ ^[0-9]+$ ]]; then
      echo "ERROR: Not a valid semver: $ver" >&2
      printf "%s\n" "$ver"
      return 1
   fi

   if (( minor > 0 )); then
      minor=$((minor - 1))
   else
      # Policy decision: when MINOR == 0.
      # Option A (current): warn and keep original.
      # Option B: decrement MAJOR (if >0) and set MINOR to something (e.g., max), set PATCH=0.
      echo "WARNING: N-1 for $ver: minor == 0, keeping original (policy)." >&2
      printf "%s%s\n" "$prefix" "$major.$minor.$patch"
      return 0
   fi

   printf "%s%s\n" "$prefix" "$major.$minor.$patch"
}




update_observability_tag_only() {
   echo "<------------------ OBSERVABILITY -------------------->"
   set -Eeuo pipefail
   trap 'echo "❌ Error at line ${LINENO}: ${BASH_COMMAND}" >&2' ERR

   OBS_ROOT="$GITHUB_WORKSPACE/operatorconfig/moduleconfig/observability"

   update_metrics_tag_in_file() {
      local f="$1"
      [[ -f "$f" ]] || { echo "↷ Skip missing: $f"; return 0; }

      # csm-metrics images
      sed -i \
      -e "s|quay.io/dell/container-storage-modules/csm-metrics-powerflex.*|quay.io/dell/container-storage-modules/csm-metrics-powerflex:${pflex_matrics}|g" \
      -e "s|quay.io/dell/container-storage-modules/csm-metrics-powermax.*|quay.io/dell/container-storage-modules/csm-metrics-powermax:${pmax_matrics}|g" \
      -e "s|quay.io/dell/container-storage-modules/csm-metrics-powerscale.*|quay.io/dell/container-storage-modules/csm-metrics-powerscale:${pscale_matrics}|g" \
      -e "s|quay.io/dell/container-storage-modules/csm-metrics-powerstore.*|quay.io/dell/container-storage-modules/csm-metrics-powerstore:${pstore_matrics}|g" \
      "$f"

      # otel collector (support GHCR and Docker Hub forms)
      if [[ -n "${otel_col:-}" ]]; then
      sed -i \
         -e "s|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector.*|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector:${otel_col}|g" \
         -e "s|docker.io/otel/opentelemetry-collector.*|docker.io/otel/opentelemetry-collector:${otel_col}|g" \
         "$f"
      fi
   }
   
   retag_obs_version_dir() {
      local dir="$1"
      cd "$dir" || { echo "❌ Cannot cd to $dir"; return 1; }

      # Retag the three module files; skip if missing
      for f in karavi-metrics-powerflex.yaml karavi-metrics-powermax.yaml karavi-metrics-powerscale.yaml karavi-metrics-powerstore.yaml; do
      [[ -f "$f" ]] || { echo "↷ Skip missing: $dir/$f"; continue; }
      sed -i \
         -e "s|quay.io/dell/container-storage-modules/csm-metrics-powerflex.*|quay.io/dell/container-storage-modules/csm-metrics-powerflex:${pflex_matrics}|g" \
         -e "s|quay.io/dell/container-storage-modules/csm-metrics-powermax.*|quay.io/dell/container-storage-modules/csm-metrics-powermax:${pmax_matrics}|g" \
         -e "s|quay.io/dell/container-storage-modules/csm-metrics-powerscale.*|quay.io/dell/container-storage-modules/csm-metrics-powerscale:${pscale_matrics}|g" \
         -e "s|quay.io/dell/container-storage-modules/csm-metrics-powerstore.*|quay.io/dell/container-storage-modules/csm-metrics-powerstore:${pstore_matrics}|g" \
         "$f"
      done
   }

   
   # -----------------------------
   # Bundle CSV updates (modular, tag-only)
   # -----------------------------
   update_obs_bundle_manifest_images() {
      local csv="$GITHUB_WORKSPACE/bundle/manifests/dell-csm-operator.clusterserviceversion.yaml"
      [[ -f "$csv" ]] || { echo "↷ Skip missing: $csv"; return 0; }

      # csm-metrics images in - image:, "image":, and value:
      sed -i \
      -e "s|^\(\s*-\s*image:\s*quay\.io/dell/container-storage-modules/csm-metrics-powerflex\)\(:[^[:space:]]*\)\{0,1\}|\1:${pflex_matrics}|g" \
      -e "s|^\(\s*-\s*image:\s*quay\.io/dell/container-storage-modules/csm-metrics-powermax\)\(:[^[:space:]]*\)\{0,1\}|\1:${pmax_matrics}|g" \
      -e "s|^\(\s*-\s*image:\s*quay\.io/dell/container-storage-modules/csm-metrics-powerscale\)\(:[^[:space:]]*\)\{0,1\}|\1:${pscale_matrics}|g" \
      -e "s|^\(\s*-\s*image:\s*quay\.io/dell/container-storage-modules/csm-metrics-powerstore\)\(:[^[:space:]]*\)\{0,1\}|\1:${pstore_matrics}|g" \
      -e "s|\"image\":\s*\"quay\.io/dell/container-storage-modules/csm-metrics-powerflex\(\:[^\",]*\)\{0,1\}\"|\"image\": \"quay.io/dell/container-storage-modules/csm-metrics-powerflex:${pflex_matrics}\"|g" \
      -e "s|\"image\":\s*\"quay\.io/dell/container-storage-modules/csm-metrics-powermax\(\:[^\",]*\)\{0,1\}\"|\"image\": \"quay.io/dell/container-storage-modules/csm-metrics-powermax:${pmax_matrics}\"|g" \
      -e "s|\"image\":\s*\"quay\.io/dell/container-storage-modules/csm-metrics-powerscale\(\:[^\",]*\)\{0,1\}\"|\"image\": \"quay.io/dell/container-storage-modules/csm-metrics-powerscale:${pscale_matrics}\"|g" \
      -e "s|\"image\":\s*\"quay\.io/dell/container-storage-modules/csm-metrics-powerstore\(\:[^\",]*\)\{0,1\}\"|\"image\": \"quay.io/dell/container-storage-modules/csm-metrics-powerstore:${pstore_matrics}\"|g" \
      -e "s|^\(\s*value:\s*quay\.io/dell/container-storage-modules/csm-metrics-powerflex\)\(:[^[:space:]]*\)\{0,1\}|\1:${pflex_matrics}|g" \
      -e "s|^\(\s*value:\s*quay\.io/dell/container-storage-modules/csm-metrics-powermax\)\(:[^[:space:]]*\)\{0,1\}|\1:${pmax_matrics}|g" \
      -e "s|^\(\s*value:\s*quay\.io/dell/container-storage-modules/csm-metrics-powerscale\)\(:[^[:space:]]*\)\{0,1\}|\1:${pscale_matrics}|g" \
      -e "s|^\(\s*value:\s*quay\.io/dell/container-storage-modules/csm-metrics-powerstore\)\(:[^[:space:]]*\)\{0,1\}|\1:${pstore_matrics}|g" \
      "$csv"

      # otel collector (support GHCR and Docker forms)
      if [[ -n "${otel_col:-}" ]]; then
      sed -i \
         -e "s|^\(\s*-\s*image:\s*ghcr\.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector\)\(:[^[:space:]]*\)\{0,1\}|\1:${otel_col}|g" \
         -e "s|\"image\":\s*\"ghcr\.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector\(\:[^\",]*\)\{0,1\}\"|\"image\": \"ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector:${otel_col}\"|g" \
         -e "s|^\(\s*value:\s*ghcr\.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector\)\(:[^[:space:]]*\)\{0,1\}|\1:${otel_col}|g" \
         -e "s|^\(\s*-\s*image:\s*docker\.io/otel/opentelemetry-collector\)\(:[^[:space:]]*\)\{0,1\}|\1:${otel_col}|g" \
         -e "s|\"image\":\s*\"docker\.io/otel/opentelemetry-collector\(\:[^\",]*\)\{0,1\}\"|\"image\": \"docker.io/otel/opentelemetry-collector:${otel_col}\"|g" \
         -e "s|^\(\s*value:\s*docker\.io/otel/opentelemetry-collector\)\(:[^[:space:]]*\)\{0,1\}|\1:${otel_col}|g" \
         "$csv"
      fi
   }

   
   update_obs_bundle_manifest_config_version() {
   # Define the file you’re editing
      local input_file="$GITHUB_WORKSPACE/bundle/manifests/dell-csm-operator.clusterserviceversion.yaml"

      local search_string1="quay.io/dell/container-storage-modules/csm-metrics-"
      local search_string2="metrics-"
      local newver="${obs_ver}"
      local line_number=0
      local tmp_line=0

      LC_ALL=C
      while IFS= read -r line; do
         line_number=$((line_number + 1))

         # Match csm-metrics image lines but skip ones that include "value"
         if [[ "$line" == *"$search_string1"* && "$line" != *"value"* ]]; then
            IFS= read -r next_line || next_line=""
            if [[ "$next_line" == *"$search_string2"* ]]; then
            local line_number_tmp=$((line_number + 4 + tmp_line))
            tmp_line=$((tmp_line + 1))

            local data
            data="$(sed -n "${line_number_tmp}p" "$input_file" || true)"

            if [[ "$data" == *"configVersion"* ]]; then
               if [[ "$data" =~ ^[[:space:]]*\"configVersion\"[[:space:]]*: ]]; then
                  # JSON-like: preserve indentation and trailing comma
                  sed -E -i \
                  "${line_number_tmp}s/^([[:space:]]*)\"configVersion\"[[:space:]]*:[[:space:]]*\"[^\"]+\"([[:space:]]*,?)/\1\"configVersion\": \"${newver}\"\2/" \
                  "$input_file"
               else
                  # YAML-like: preserve indentation and trailing comments
                  sed -E -i \
                  "${line_number_tmp}s/^([[:space:]]*)configVersion:[[:space:]]*([^#]*)(#.*)?$/\1configVersion: ${newver}\3/" \
                  "$input_file"
               fi
            fi
            fi
         fi
      done < "$input_file"
   }



   # -----------------------------
   # Copy latest if needed, and ALWAYS retag inside obs_ver
   # -----------------------------
   mkdir -p "$OBS_ROOT"
   cd "$OBS_ROOT"

   if [[ -d "$obs_ver" ]]; then
      echo "ℹ️ Observability config dir exists: $OBS_ROOT/$obs_ver"
      # ✅ Retag even when the folder already exists
      retag_obs_version_dir "$OBS_ROOT/$obs_ver"
   else
      echo "ℹ️ Creating $OBS_ROOT/$obs_ver from latest"
      mapfile -t dirs < <(ls -d */ 2>/dev/null | sed 's|/$||' | sort -V)
      if (( ${#dirs[@]} == 0 )); then
      echo "❌ No existing observability directories to copy from."
      return 1
      fi
      dir_to_copy="${dirs[-1]}"
      dir_to_del="${dirs[0]}"

      cp -r "$dir_to_copy" "$obs_ver"
      echo "→ Copied: $dir_to_copy -> $obs_ver"

      # ✅ Immediately retag the freshly copied files (no nightly)
      retag_obs_version_dir "$OBS_ROOT/$obs_ver"

      # Optional: delete oldest dir to keep tree tidy
      if [[ "${DELETE_OLDEST_OBS_DIR:-false}" == "true" && "$dir_to_del" != "$dir_to_copy" ]]; then
      echo "→ Deleting oldest dir: $dir_to_del"
      rm -rf "$dir_to_del"
      else
      echo "ℹ️ Skipping deletion of oldest (set DELETE_OLDEST_OBS_DIR=true to enable)."
      fi
   fi

   # -----------------------------
   # Update all other locations (tags + configVersion)
   # -----------------------------
   update_metrics_tag_in_file "$GITHUB_WORKSPACE/config/manager/manager.yaml"
   update_metrics_tag_in_file "$GITHUB_WORKSPACE/config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml"
   update_metrics_tag_in_file "$GITHUB_WORKSPACE/deploy/operator.yaml"
   
   # Bundle/manifests CSV (images + configVersion)
   update_obs_bundle_manifest_images
   update_obs_bundle_manifest_config_version


   # Samples (configVersion + tags)
   samples_cfg_files=(
      "$GITHUB_WORKSPACE/config/samples/storage_v1_csm_powerflex.yaml"
      "$GITHUB_WORKSPACE/config/samples/storage_v1_csm_powermax.yaml"
      "$GITHUB_WORKSPACE/config/samples/storage_v1_csm_powerscale.yaml"
      "$GITHUB_WORKSPACE/config/samples/storage_v1_csm_powerstore.yaml"
   )
   for f in "${samples_cfg_files[@]}"; do
      update_config_version_yq "$f" "observability" "$obs_ver" 
      update_metrics_tag_in_file "$f"
   done

   # Detailed samples (driver-specific)
   driver_sample_files=(
      "$GITHUB_WORKSPACE/samples/$CSI_POWERMAX/storage_csm_powerflex_${pflex_driver_ver}.yaml"
      "$GITHUB_WORKSPACE/samples/$CSI_POWERMAX/storage_csm_powermax_${pmax_driver_ver}.yaml"
      "$GITHUB_WORKSPACE/samples/$CSI_POWERMAX/storage_csm_powerscale_${pscale_driver_ver}.yaml"
      "$GITHUB_WORKSPACE/samples/$CSI_POWERMAX/storage_csm_powerstore_${pstore_driver_ver}.yaml"
   )
   for f in "${driver_sample_files[@]}"; do
      update_config_version_yq "$f" "observability" "$obs_ver" 
      update_metrics_tag_in_file "$f"
   done

   # Testfiles
   shopt -s nullglob
   for f in "$GITHUB_WORKSPACE/tests/e2e/testfiles"/storage_csm*; do
      update_config_version_yq "$f" "observability" "$obs_ver" 
      update_metrics_tag_in_file "$f"
   done
   shopt -u nullglob

      shopt -s nullglob
   for f in "$GITHUB_WORKSPACE/tests/e2e/testfiles/minimal-testfiles"/storage_csm*; do
      update_config_version_yq "$f" "observability" "$obs_ver" 
      update_metrics_tag_in_file "$f"
   done
   shopt -u nullglob

   # pkg/modules/testdata
   for f in "$GITHUB_WORKSPACE/pkg/modules/testdata"/cr_*; do
      base="$(basename "$f")"
      case "$base" in
         cr_powerflex_observability_214.yaml|\
         cr_powerflex_observability_with_old_otel_image.yaml|\
         cr_powermax_observability_214.yaml|\
         cr_powerscale_observability_with_topology.yaml|\
         cr_powerscale_observability_214.yaml)
            # echo "↷ Skip explicitly: $base"
            continue
            ;;
      esac

      # echo "→ Updating $base"
      update_config_version_yq "$f" "observability" "$obs_ver" 
      update_metrics_tag_in_file "$f" 
   done

   echo "✅ Observability Module config -> ${obs_ver} updated successfully (tag-only, folder checked even when present)."
}

# It removes all nightly behavior and validates after updates.
update_resiliency_tag_only() {
   echo "<------------------ Resiliency -------------------->"
   set -Eeuo pipefail
   trap 'echo "❌ Error at line ${LINENO}: ${BASH_COMMAND}" >&2' ERR

   # Paths
   RES_ROOT="$GITHUB_WORKSPACE/operatorconfig/moduleconfig/resiliency"
   CSV_MANIFESTS_DIR="$GITHUB_WORKSPACE/bundle/manifests"
   CSV_FILE_NAME="dell-csm-operator.clusterserviceversion.yaml"
   CSV_FILE="$CSV_MANIFESTS_DIR/$CSV_FILE_NAME"
   CONFIG_MANAGER_DIR="$GITHUB_WORKSPACE/config/manager"
   CONFIG_MANAGER_FILE="$CONFIG_MANAGER_DIR/manager.yaml"
   CONFIG_BASES_DIR="$GITHUB_WORKSPACE/config/manifests/bases"
   CONFIG_BASES_CSV="$CONFIG_BASES_DIR/dell-csm-operator.clusterserviceversion.yaml"
   CONFIG_SAMPLES_DIR="$GITHUB_WORKSPACE/config/samples"
   DEPLOY_DIR="$GITHUB_WORKSPACE/deploy"
   DEPLOY_OPERATOR_FILE="$DEPLOY_DIR/operator.yaml"
   PKG_TESTDATA_DIR="$GITHUB_WORKSPACE/pkg/modules/testdata"
   SAMPLES_DIR="$GITHUB_WORKSPACE/samples/$CSI_POWERMAX"
   TESTFILES_DIR="$GITHUB_WORKSPACE/tests/e2e/testfiles"

   update_podmon_tag_in_file() {
      local f="$1"
      [[ -f "$f" ]] || { echo "↷ Skip missing: $f"; return 0; }

      # Ensure res_ver is set (fail early if missing)
      : "${res_ver:?res_ver is required (e.g., v1.15.0)}"

      # Normalize CRLF if file came from Windows (optional but helpful)
      if grep -q $'\r' "$f"; then
         sed -i 's/\r$//' "$f"
      fi

      # Replace any podmon occurrence and its optional tag with :$res_ver
      sed -E -i "s|(quay\.io/dell/container-storage-modules/podmon)(:[^[:space:]\",]+)?|\1:${res_ver}|g" "$f"
      }
         
      # Positional update of "configVersion" near podmon block
      update_podmon_bundle_manifest_config_version() {
      local input_file="$1"     # path to file to modify
      local offset="${2:-5}"    # default offset = 5, can be overridden
      local newver="${res_ver}" # uses env var res_ver, or set explicitly below

      # Validate inputs
      [[ -z "$input_file" ]] && { echo "✖ Missing input_file"; return 1; }
      [[ -f "$input_file" ]] || { echo "↷ Skip missing: $input_file"; return 0; }
      [[ -n "$newver" ]] || { echo "✖ Missing newver (res_ver). Set res_ver or pass as 3rd arg."; return 1; }

      local search_string1="quay.io/dell/container-storage-modules/podmon"
      local search_string2="imagePullPolicy"

      local line_number=0
      local tmp_line=0

      # Read file line-by-line and peek the next line
      while IFS= read -r line; do
         line_number=$((line_number + 1))

         if [[ "$line" == *"$search_string1"* ]]; then
            # Peek next line (safe if EOF)
            IFS= read -r next_line || true

            if [[ "$next_line" == *"$search_string2"* ]]; then
            # Compute target line number: start line + offset + tmp_line
            local line_number_tmp=$((line_number + offset + tmp_line))
            tmp_line=$((tmp_line + 1))

            # Get target line content
            local data
            data="$(sed -n "${line_number_tmp}p" "$input_file" || true)"

            if [[ "$data" == *"configVersion"* ]]; then
               # Preserve indentation from the existing line
               local indent
               indent="$(printf '%s\n' "$data" | sed -E 's/^([[:space:]]*).*/\1/')"

               # Replace the whole line using sed; maintain trailing comma
               sed -E -i "${line_number_tmp}s|^.*\$|${indent}\"configVersion\": \"${newver}\",|" "$input_file"
            else
               echo "ℹ Target line ${line_number_tmp} does not contain configVersion; skipped."
            fi
            fi
         fi
      done < "$input_file"
   }

   update_and_validate_files() {
      local files=("$@")
      for f in "${files[@]}"; do
         update_podmon_tag_in_file "$f"
      done
   }

   # -----------------------------
   # Ensure base dir and resolve target version dir
   # -----------------------------
   mkdir -p "$RES_ROOT"
   cd "$RES_ROOT"


   if [[ -d "$res_ver" ]]; then
      echo "ℹ️ Resiliency config dir exists: $RES_ROOT/$res_ver"
   else
      echo "ℹ️ Resiliency config dir does not exist: creating $RES_ROOT/$res_ver from latest"

      # Collect existing version dirs (strip trailing /, sort as versions)
      mapfile -t dirs < <(ls -d */ 2>/dev/null | sed 's|/$||' | sort -V)

      if (( ${#dirs[@]} == 0 )); then
         echo "❌ No existing resiliency directories to copy from."
         return 1
      fi

      # Latest and oldest
      dir_to_copy="${dirs[-1]}"          # equivalent of tail -1
      dir_to_del="${dirs[0]}"            # equivalent of head -1

      if [[ -d "$res_ver" ]]; then
         echo "⚠️ Target $res_ver already exists—skipping copy."
      else
         echo "→ Copying from: $dir_to_copy -> $res_ver"
         cp -r "$dir_to_copy" "$res_ver"

         echo "→ Deleting oldest dir: $dir_to_del"
         rm -rf "$dir_to_del"

      fi

      # Immediately retag the freshly copied resiliency config to the real tag (no nightly)
      echo "→ Retagging container-* under $RES_ROOT/$res_ver to podmon:$res_ver"
      (
         cd "$RES_ROOT/$res_ver" || { echo "❌ Cannot cd to $RES_ROOT/$res_ver"; return 1; }
         shopt -s nullglob
         for input_file in container-*; do
            # Handle YAML '- image:' lines
            sed -i -E "s|^(image:[[:space:]]*quay\.io/dell/container-storage-modules/podmon)(:[^[:space:]]+)?|\1:$res_ver|g" "$input_file"
         done
         shopt -u nullglob
      )

   fi

   # -----------------------------
   # Update resiliency module config (container-*) under target version
   # -----------------------------
   shopt -s nullglob
   res_config_files=( "$RES_ROOT/$res_ver"/container-* )
   shopt -u nullglob
   update_and_validate_files "${res_config_files[@]}"

   # -----------------------------
   # Update bundle/manifests CSV
   # Also positional configVersion update (offset 5 as in original)
   # -----------------------------
   update_and_validate_files "$CSV_FILE"
   update_podmon_bundle_manifest_config_version "$CSV_FILE" 5

   # -----------------------------
   # Update config/manager/manager.yaml
   # -----------------------------
   update_and_validate_files "$CONFIG_MANAGER_FILE"

   # -----------------------------
   # Update config/manifests/bases CSV
   # -----------------------------
   update_and_validate_files "$CONFIG_BASES_CSV"

   # ----------------------
   # Update config/samples 
   # ----------------------
   samples_cfg_files=(
      "$CONFIG_SAMPLES_DIR/storage_v1_csm_powerflex.yaml"
      "$CONFIG_SAMPLES_DIR/storage_v1_csm_powermax.yaml"
      "$CONFIG_SAMPLES_DIR/storage_v1_csm_powerscale.yaml"
      "$CONFIG_SAMPLES_DIR/storage_v1_csm_powerstore.yaml"
   )
   for f in "${samples_cfg_files[@]}"; do
      update_config_version_yq "$f" "resiliency" "$res_ver" 
      update_podmon_tag_in_file "$f"
   done
   # -----------------------------
   # Update deploy/operator.yaml
   # -----------------------------
   update_and_validate_files "$DEPLOY_OPERATOR_FILE"

   # -----------------------------
   # Update pkg/modules/testdata 
   # -----------------------------
   pkg_testdata_files=(
      "$PKG_TESTDATA_DIR/cr_powerflex_resiliency.yaml"
      "$PKG_TESTDATA_DIR/cr_powermax_resiliency.yaml"
      "$PKG_TESTDATA_DIR/cr_powerscale_resiliency.yaml"
      "$PKG_TESTDATA_DIR/cr_powerstore_resiliency.yaml"
   )
   for f in "${pkg_testdata_files[@]}"; do
      update_config_version_yq "$f" "resiliency" "$res_ver" 
      update_podmon_tag_in_file "$f"
   done

   # ------------------------------------------------------
   # Update samples/<CSI_POWERMAX> Also apply configVersion 
   # ------------------------------------------------------
   driver_sample_files=(
      "$SAMPLES_DIR/storage_csm_powerflex_${pflex_driver_ver}.yaml"
      "$SAMPLES_DIR/storage_csm_powermax_${pmax_driver_ver}.yaml"
      "$SAMPLES_DIR/storage_csm_powerscale_${pscale_driver_ver}.yaml"
      "$SAMPLES_DIR/storage_csm_powerstore_${pstore_driver_ver}.yaml"
   )
   for f in "${driver_sample_files[@]}"; do
      update_config_version_yq "$f" "resiliency" "$res_ver" 
      update_podmon_tag_in_file "$f"
   done

   # --------------------------
   # Update tests/e2e/testfiles 
   # --------------------------
   shopt -s nullglob
   e2e_files=( "$TESTFILES_DIR"/storage_csm* )
   shopt -u nullglob
   for f in "${e2e_files[@]}"; do
      update_config_version_yq "$f" "resiliency" "$res_ver" 
      update_podmon_tag_in_file "$f"
   done

   echo "✅ Resiliency Module config -> $res_ver updated successfully (tag-only, no nightly)."
}

update_replication_tag_only() {
   echo "<------------------ Replication -------------------->"
   set -Eeuo pipefail
   trap 'echo "❌ Error at line ${LINENO}: ${BASH_COMMAND}" >&2' ERR

   REPL_ROOT="$GITHUB_WORKSPACE/operatorconfig/moduleconfig/replication"
   BUNDLE_CSV="$GITHUB_WORKSPACE/bundle/manifests/dell-csm-operator.clusterserviceversion.yaml"

   retag_replication_module_dir() {
      local dir="$1"   # e.g., $REPL_ROOT/$rep_ver
      [[ -d "$dir" ]] || { echo "❌ Missing dir: $dir"; return 1; }

      # container.yaml → dell-csi-replicator
      if [[ -f "$dir/container.yaml" ]]; then
         sed -i \
         -e "s|quay.io/dell/container-storage-modules/dell-csi-replicator.*|quay.io/dell/container-storage-modules/dell-csi-replicator:${dell_csi_replicator}|g" \
         "$dir/container.yaml"
      else
         echo "↷ Skip missing: $dir/container.yaml"
      fi

      # controller.yaml → dell-replication-controller
      if [[ -f "$dir/controller.yaml" ]]; then
         sed -i \
         -e "s|quay.io/dell/container-storage-modules/dell-replication-controller.*|quay.io/dell/container-storage-modules/dell-replication-controller:${dell_replication_controller}|g" \
         "$dir/controller.yaml"
      else
         echo "↷ Skip missing: $dir/controller.yaml"
      fi
   }

   update_repl_tags_in_file() {
      local f="$1"
      local ver="${2:-$dell_csi_replicator}"
      [[ -f "$f" ]] || { echo "↷ Skip missing: $f"; return 0; }

      sed -i \
         -e "s|quay.io/dell/container-storage-modules/dell-csi-replicator.*|quay.io/dell/container-storage-modules/dell-csi-replicator:${ver}|g" \
         -e "s|quay.io/dell/container-storage-modules/dell-replication-controller.*|quay.io/dell/container-storage-modules/dell-replication-controller:${ver}|g" \
         "$f"
   }

# Positional update of "configVersion" near dell-replication-controller block
   update_replication_bundle_manifest_config_version() {
      local input_file="$1"     # path to file you want to modify
      local offset="${2:-4}"    # default offset = 4, can be overridden
      local newver="${rep_ver}" # uses env var rep_ver, or set explicitly below


      # Validate inputs
      [[ -z "$input_file" ]] && { echo "✖ Missing input_file"; return 1; }
      [[ -f "$input_file" ]] || { echo "↷ Skip missing: $input_file"; return 0; }
      [[ -n "$newver" ]] || { echo "✖ Missing newver (rep_ver). Set rep_ver or pass as 3rd arg."; return 1; }

      local search_string1="quay.io/dell/container-storage-modules/dell-replication-controller"
      local search_string2="dell-replication-controller-manager"

      local line_number=0
      local tmp_line=0

      # We read the file line-by-line and peek the next line when needed
      while IFS= read -r line; do
         line_number=$((line_number + 1))

         if [[ "$line" == *"$search_string1"* ]]; then
            # Read the next line (peek)
            IFS= read -r next_line || true

            if [[ "$next_line" == *"$search_string2"* ]]; then
            # Compute target line number at fixed offset (+ offset + tmp_line to handle multiple matches)
            local line_number_tmp=$((line_number + offset + tmp_line))
            tmp_line=$((tmp_line + 1))

            # Fetch target line content
            local data
            data="$(sed -n "${line_number_tmp}p" "$input_file" || true)"

            if [[ "$data" == *"configVersion"* ]]; then
               local indent
               indent="$(printf '%s\n' "$data" | sed -E 's/^([[:space:]]*).*/\1/')"
               sed -E -i "${line_number_tmp}s|^.*\$|${indent}\"configVersion\": \"${newver}\",|" "$input_file"
            else
               echo "ℹ Target line ${line_number_tmp} does not contain configVersion; skipped."
            fi
            fi
         fi
      done < "$input_file"
   }

   update_replication_bundle_manifest_images() {
      local csv="$BUNDLE_CSV"
      [[ -f "$csv" ]] || { echo "↷ Skip missing: $csv"; return 0; }

      # - image: / "image": / value: — for both replicator and controller
      sed -i \
         -e "s|^\(\s*-\s*image:\s*quay\.io/dell/container-storage-modules/dell-csi-replicator\)\(:[^[:space:]]*\)\{0,1\}|\1:${dell_csi_replicator}|g" \
         -e "s|\"image\":\s*\"quay\.io/dell/container-storage-modules/dell-csi-replicator\(\:[^\",]*\)\{0,1\}\"|\"image\": \"quay.io/dell/container-storage-modules/dell-csi-replicator:${dell_csi_replicator}\"|g" \
         -e "s|^\(\s*value:\s*quay\.io/dell/container-storage-modules/dell-csi-replicator\)\(:[^[:space:]]*\)\{0,1\}|\1:${dell_csi_replicator}|g" \
         -e "s|^\(\s*-\s*image:\s*quay\.io/dell/container-storage-modules/dell-replication-controller\)\(:[^[:space:]]*\)\{0,1\}|\1:${dell_replication_controller}|g" \
         -e "s|\"image\":\s*\"quay\.io/dell/container-storage-modules/dell-replication-controller\(\:[^\",]*\)\{0,1\}\"|\"image\": \"quay.io/dell/container-storage-modules/dell-replication-controller:${dell_replication_controller}\"|g" \
         -e "s|^\(\s*value:\s*quay\.io/dell/container-storage-modules/dell-replication-controller\)\(:[^[:space:]]*\)\{0,1\}|\1:${dell_replication_controller}|g" \
         "$csv"
   }


   mkdir -p "$REPL_ROOT"
   cd "$REPL_ROOT"

   if [[ -d "$rep_ver" ]]; then
      echo "ℹ️ Replication config dir exists: $REPL_ROOT/$rep_ver"
      retag_replication_module_dir "$REPL_ROOT/$rep_ver"
   else
      echo "ℹ️ Creating $REPL_ROOT/$rep_ver from latest"
      mapfile -t dirs < <(ls -d */ 2>/dev/null | sed 's|/$||' | sort -V)
      if (( ${#dirs[@]} == 0 )); then
         echo "❌ No existing replication directories to copy from."
         return 1
      fi

      dir_to_copy="${dirs[-1]}"
      dir_to_del="${dirs[0]}"

      cp -r "$dir_to_copy" "$rep_ver"
      echo "→ Copied: $dir_to_copy -> $rep_ver"

      echo "Deleted: $dir_to_del"
      rm -rf "$dir_to_del"

      retag_replication_module_dir "$REPL_ROOT/$rep_ver"

      # Optional tidy-up
      if [[ "${DELETE_OLDEST_REPL_DIR:-false}" == "true" && "$dir_to_del" != "$dir_to_copy" ]]; then
         echo "→ Deleting oldest dir: $dir_to_del"
         rm -rf "$dir_to_del"
      else
         echo "ℹ️ Skipping deletion of oldest (set DELETE_OLDEST_REPL_DIR=true to enable)."
      fi
   fi

   # -----------------------------
   # Update other locations
   # -----------------------------
   # Bundle CSV: images + configVersion
   update_replication_bundle_manifest_images
   # update_bundle_csv_config_version
   update_replication_bundle_manifest_config_version $BUNDLE_CSV

   # config/manager and bases CSV
   update_repl_tags_in_file "$GITHUB_WORKSPACE/config/manager/manager.yaml"
   update_repl_tags_in_file "$GITHUB_WORKSPACE/config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml"

   # deploy/operator.yaml
   update_repl_tags_in_file "$GITHUB_WORKSPACE/deploy/operator.yaml"

   # config/samples (configVersion + tags)
   samples_cfg_files=(
      "$GITHUB_WORKSPACE/config/samples/storage_v1_csm_powerflex.yaml"
      "$GITHUB_WORKSPACE/config/samples/storage_v1_csm_powermax.yaml"
      "$GITHUB_WORKSPACE/config/samples/storage_v1_csm_powerscale.yaml"
      "$GITHUB_WORKSPACE/config/samples/storage_v1_csm_powerstore.yaml"
   )
   for f in "${samples_cfg_files[@]}"; do
      update_config_version_yq "$f" "replication" "$rep_ver" 
      update_repl_tags_in_file "$f"
   done

   # pkg/modules/testdata
   for f in "$GITHUB_WORKSPACE/pkg/modules/testdata"/cr_*_replica.yaml; do
      update_repl_tags_in_file "$f"
   done

   # samples/$CSI_POWERMAX (driver-specific files)
   driver_sample_files=(
      "$GITHUB_WORKSPACE/samples/$CSI_POWERMAX/storage_csm_powerflex_${pflex_driver_ver}.yaml"
      "$GITHUB_WORKSPACE/samples/$CSI_POWERMAX/storage_csm_powermax_${pmax_driver_ver}.yaml"
      "$GITHUB_WORKSPACE/samples/$CSI_POWERMAX/storage_csm_powerscale_${pscale_driver_ver}.yaml"
      "$GITHUB_WORKSPACE/samples/$CSI_POWERMAX/storage_csm_powerstore_${pstore_driver_ver}.yaml"
   )
   for f in "${driver_sample_files[@]}"; do
      update_config_version_yq "$f" "replication" "$rep_ver" 
      update_repl_tags_in_file "$f"
   done

   local tf_dir="$GITHUB_WORKSPACE/tests/e2e/testfiles"
   if [[ -d "$tf_dir" ]]; then
   shopt -s nullglob
   # Compute N-1 version from auth_v2 (e.g., v2.4.0 -> v2.3.0)
   replication_minus_1="$(semver_n_minus_one "$rep_ver")"
   for f in "$tf_dir"/storage_csm_*; do
      base="$(basename "$f")"
      # Special-case file: set authorization to N-1
      if [[ "$base" == "storage_csm_powerflex_downgrade.yaml" ]]; then
         # Only touch if module exists & change is needed
         update_config_version_yq "$f" "replication" "$replication_minus_1"
         # If your images should also reflect N-1, update them:
         update_repl_tags_in_file "$f" "$replication_minus_1"
         continue
      fi
      # Default behavior for other files
      if grep -q "name: replication" "$f"; then
         update_config_version_yq "$f" "replication" "$rep_ver"
      fi
      # Keep using the regular image update for auth (current version)
      update_repl_tags_in_file "$f" "$rep_ver"
   done

   shopt -u nullglob
   fi

   echo "✅ Replication Module config -> ${rep_ver} updated successfully (tag-only, no nightly)."
}

update_reverseproxy_tag_only() {
   echo "<------------------ ReverseProxy -------------------->"
   set -Eeuo pipefail
   trap 'echo "❌ Error at line ${LINENO}: ${BASH_COMMAND}" >&2' ERR

   RP_ROOT="$GITHUB_WORKSPACE/operatorconfig/moduleconfig/csireverseproxy"
   BUNDLE_CSV="$GITHUB_WORKSPACE/bundle/manifests/dell-csm-operator.clusterserviceversion.yaml"

   # Retag module's container.yaml to the real tag (no nightly)
   retag_reverseproxy_module_dir() {
      local dir="$1"   # e.g., $RP_ROOT/$revproxy_ver
      [[ -d "$dir" ]] || { echo "❌ Missing dir: $dir"; return 1; }

      if [[ -f "$dir/container.yaml" ]]; then
      # If the file uses a plain image key, a precise rule helps:
      sed -i -E \
         -e "s|^([[:space:]]*image:[[:space:]]*quay\.io/dell/container-storage-modules/csipowermax-reverseproxy)(:[^[:space:]]+)?|\1:${revproxy_ver}|g" \
         "$dir/container.yaml"

      # Catch any other forms (value:, JSON "image":, generic occurrences)
      sed -i \
         -e "s|\"image\": \"quay.io/dell/container-storage-modules/csipowermax-reverseproxy.*|\"image\": \"quay.io/dell/container-storage-modules/csipowermax-reverseproxy:${revproxy_ver}\"|g" \
         -e "s|^\(\s*value:\s*quay\.io/dell/container-storage-modules/csipowermax-reverseproxy\)\(:[^[:space:]]*\)\{0,1\}|\1:${revproxy_ver}|g" \
         -e "s|quay\.io/dell/container-storage-modules/csipowermax-reverseproxy:[^\"'[:space:]]+|quay.io/dell/container-storage-modules/csipowermax-reverseproxy:${revproxy_ver}|g" \
         "$dir/container.yaml"
      else
      echo "↷ Skip missing: $dir/container.yaml"
      fi
   }

   
   update_revproxy_tags_in_file() {
      local f="$1"
      [[ -f "$f" ]] || { echo "↷ Skip missing: $f"; return 0; }
      [[ -n "$revproxy_ver" ]] || { echo "✖ Missing revproxy_ver"; return 1; }

      # One sed invocation, one expression; extended regex for ? and +
      sed -E -i \
         "s|(quay\.io/dell/container-storage-modules/csipowermax-reverseproxy)(:[^\"'[:space:]]+)?|\1:${revproxy_ver}|g" \
         "$f"
   }

   # Bundle/manifests CSV: update images & configVersion (pattern-based)
   update_reverseproxy_bundle_manifest_images() {
      local csv="$BUNDLE_CSV"
      [[ -f "$csv" ]] || { echo "↷ Skip missing: $csv"; return 0; }

      sed -i \
      -e "s|^\(\s*-\s*image:\s*quay\.io/dell/container-storage-modules/csipowermax-reverseproxy\)\(:[^[:space:]]*\)\{0,1\}|\1:${revproxy_ver}|g" \
      -e "s|\"image\":\s*\"quay\.io/dell/container-storage-modules/csipowermax-reverseproxy\(\:[^\",]*\)\{0,1\}\"|\"image\": \"quay.io/dell/container-storage-modules/csipowermax-reverseproxy:${revproxy_ver}\"|g" \
      -e "s|^\(\s*value:\s*quay\.io/dell/container-storage-modules/csipowermax-reverseproxy\)\(:[^[:space:]]*\)\{0,1\}|\1:${revproxy_ver}|g" \
      "$csv"
   }

   
# Positional update of "configVersion" near csipowermax-reverseproxy block
   update_reverseproxy_bundle_manifest_config_version() {
      local input_file="$1"        # path to file to modify
      local offset="${2:-4}"       # default offset = 4 (as in your snippet)
      local newver="${revproxy_ver}"  # uses env var revproxy_ver, or third arg

      # Validate inputs
      [[ -z "$input_file" ]] && { echo "✖ Missing input_file"; return 1; }
      [[ -f "$input_file" ]] || { echo "↷ Skip missing: $input_file"; return 0; }
      [[ -n "$newver" ]] || { echo "✖ Missing newver (revproxy_ver). Set revproxy_ver or pass as 3rd arg."; return 1; }

      local search_string1="quay.io/dell/container-storage-modules/csipowermax-reverseproxy"
      local search_string2="csipowermax-reverseproxy"

      local line_number=0
      local tmp_line=0

      # Read file line-by-line and peek the next line
      while IFS= read -r line; do
         line_number=$((line_number + 1))

         if [[ "$line" == *"$search_string1"* ]]; then
            # Peek next line; if EOF, next_line will be empty but won't break
            IFS= read -r next_line || true

            if [[ "$next_line" == *"$search_string2"* ]]; then
            # Compute target line number: start line + offset + tmp_line
            local line_number_tmp=$((line_number + offset + tmp_line))
            tmp_line=$((tmp_line + 1))

            # Get target line content
            local data
            data="$(sed -n "${line_number_tmp}p" "$input_file" || true)"

            if [[ "$data" == *"configVersion"* ]]; then
               # Preserve indentation from the existing line
               local indent
               indent="$(printf '%s\n' "$data" | sed -E 's/^([[:space:]]*).*/\1/')"

               # Replace the whole line; keep trailing comma
               sed -E -i "${line_number_tmp}s|^.*\$|${indent}\"configVersion\": \"${newver}\",|" "$input_file"
            else
               echo "ℹ Target line ${line_number_tmp} does not contain configVersion; skipped."
            fi
            fi
         fi
      done < "$input_file"
   }
   # -----------------------------
   # Copy latest if needed; ALWAYS retag inside revproxy_ver
   # -----------------------------
   mkdir -p "$RP_ROOT"
   cd "$RP_ROOT"

   if [[ -d "$revproxy_ver" ]]; then
      echo "ℹ️ Reverseproxy config dir exists: $RP_ROOT/$revproxy_ver"
      # ✅ Retag even when the folder already exists
      retag_reverseproxy_module_dir "$RP_ROOT/$revproxy_ver"
   else
      echo "ℹ️ Creating $RP_ROOT/$revproxy_ver from latest"
      mapfile -t dirs < <(ls -d */ 2>/dev/null | sed 's|/$||' | sort -V)
      if (( ${#dirs[@]} == 0 )); then
      echo "❌ No existing reverseproxy directories to copy from."
      return 1
      fi
      dir_to_copy="${dirs[-1]}"
      dir_to_del="${dirs[0]}"

      cp -r "$dir_to_copy" "$revproxy_ver"
      echo "→ Copied: $dir_to_copy -> $revproxy_ver"

      echo "→ Deleting oldest dir: $dir_to_del"
      rm -rf "$dir_to_del"

      # ✅ Immediately retag the freshly copied files (no nightly)
      retag_reverseproxy_module_dir "$RP_ROOT/$revproxy_ver"

   fi

   # Bundle CSV: images + configVersion
   update_reverseproxy_bundle_manifest_images
   update_reverseproxy_bundle_manifest_config_version $BUNDLE_CSV

   # config/manager
   update_revproxy_tags_in_file "$GITHUB_WORKSPACE/config/manager/manager.yaml"

   # bases CSV
   update_revproxy_tags_in_file "$GITHUB_WORKSPACE/config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml"

   # samples config (storage_v1_csm_powermax.yaml): configVersion + tags
   samples_cfg="$GITHUB_WORKSPACE/config/samples/storage_v1_csm_powermax.yaml"
   update_config_version_yq "$samples_cfg" "csireverseproxy" "$revproxy_ver" 
   update_revproxy_tags_in_file "$samples_cfg"

   # deploy/operator.yaml
   update_revproxy_tags_in_file "$GITHUB_WORKSPACE/deploy/operator.yaml"

   # pkg/modules/testdata (cr_powermax_*)
   shopt -s nullglob
   for f in "$GITHUB_WORKSPACE/pkg/modules/testdata"/cr_powermax_*; do
      update_config_version_yq "$f" "csireverseproxy" "$revproxy_ver" 
      update_revproxy_tags_in_file "$f"
   done
   shopt -u nullglob

   # detailed samples for the specific PowerMax driver version
   driver_sample="$GITHUB_WORKSPACE/samples/$CSI_POWERMAX/storage_csm_powermax_${pmax_driver_ver}.yaml"
   update_config_version_yq "$driver_sample" "csireverseproxy" "$revproxy_ver" 
   update_revproxy_tags_in_file "$driver_sample"     # tag-only (no nightly anywhere)

   # e2e testfiles (PowerMax)
   shopt -s nullglob
   for f in "$GITHUB_WORKSPACE/tests/e2e/testfiles"/storage_csm_powermax*; do
      update_config_version_yq "$f" "csireverseproxy" "$revproxy_ver" 
      update_revproxy_tags_in_file "$f"
   done
   shopt -u nullglob

      shopt -s nullglob
   for f in "$GITHUB_WORKSPACE/tests/e2e/testfiles/minimal-testfiles"/storage_csm_powermax*; do
      update_config_version_yq "$f" "csireverseproxy" "$revproxy_ver" 
      update_revproxy_tags_in_file "$f"
   done
   shopt -u nullglob

   echo "✅ Reverseproxy Module config -> ${revproxy_ver} updated successfully (tag-only, folder checked even when present)."
}

update_authorization_v2_tag_only() {

   echo "<------------------ Authorization V2 -------------------->"
   set -Eeuo pipefail
   trap 'echo "❌ Error at line ${LINENO}: ${BASH_COMMAND}" >&2' ERR

   # --- Inputs / defaults ---
   local AUTH_ROOT="$GITHUB_WORKSPACE/operatorconfig/moduleconfig/authorization"
   local BUNDLE_CSV="$GITHUB_WORKSPACE/bundle/manifests/dell-csm-operator.clusterserviceversion.yaml"
   local PRUNE_OFFSET="${PRUNE_OFFSET:-3}"              # delete n-3 by default
   local RECREATE_AUTH_V2="${RECREATE_AUTH_V2:-false}"  # ignored (we won't recreate if exists)

   list_dirs_sorted() {
      local root="${1:?root dir required}"
      mapfile -t _dirs < <(cd "$root" && ls -d */ 2>/dev/null | sed 's%/$%%' | sort -V)
      printf '%s\n' "${_dirs[@]}"
   }

   previous_of_target() {
      local root="${1:?root required}"
      local target="${2:?target version required}"

      mapfile -t dirs < <(list_dirs_sorted "$root")
      (( ${#dirs[@]} > 0 )) || { echo "❌ No version directories under $root" >&2; return 1; }

      local idx=-1 dir_to_copy=""
      if printf '%s\n' "${dirs[@]}" | grep -qx "${target}"; then
         for i in "${!dirs[@]}"; do
         [[ "${dirs[$i]}" == "${target}" ]] && { idx="$i"; break; }
         done
         (( idx > 0 )) || { echo "❌ '${target}' is earliest; no previous to copy from." >&2; return 1; }
         dir_to_copy="${dirs[$((idx - 1))]}"
      else
         mapfile -t dirs_plus < <(printf '%s\n' "${dirs[@]}" "${target}" | sort -V)
         for i in "${!dirs_plus[@]}"; do
         [[ "${dirs_plus[$i]}" == "${target}" ]] && { idx="$i"; break; }
         done
         (( idx > 0 )) || { echo "❌ '${target}' would be earliest; no previous to copy from." >&2; return 1; }
         dir_to_copy="${dirs_plus[$((idx - 1))]}"
      fi

      printf '%s\n' "${dir_to_copy}"
   }


   update_auth_images_in_file() {
      local f="$1"
      local ver="${2:-$auth_v2}"
      [[ -f "$f" ]] || { echo "↷ Skip missing: $f"; return 0; }

      # Portable in-place sed (GNU vs BSD/macOS)
      local SED_INPLACE=(-i)
      if ! sed --version >/dev/null 2>&1; then
         # BSD/macOS sed: -i requires a backup suffix; use empty
         SED_INPLACE=(-i '')
      fi

      sed "${SED_INPLACE[@]}" \
         -e 's#^\(\s*-\s*image:\s*quay\.io/dell/container-storage-modules/csm-authorization-sidecar\)\(:[^[:space:]]*\)\{0,1\}#\1:'"$ver"'#g' \
         -e 's#^\(\s*-\s*image:\s*quay\.io/dell/container-storage-modules/csm-authorization-proxy\)\(:[^[:space:]]*\)\{0,1\}#\1:'"$ver"'#g' \
         -e 's#^\(\s*-\s*image:\s*quay\.io/dell/container-storage-modules/csm-authorization-tenant\)\(:[^[:space:]]*\)\{0,1\}#\1:'"$ver"'#g' \
         -e 's#^\(\s*-\s*image:\s*quay\.io/dell/container-storage-modules/csm-authorization-role\)\(:[^[:space:]]*\)\{0,1\}#\1:'"$ver"'#g' \
         -e 's#^\(\s*-\s*image:\s*quay\.io/dell/container-storage-modules/csm-authorization-storage\)\(:[^[:space:]]*\)\{0,1\}#\1:'"$ver"'#g' \
         -e 's#^\(\s*-\s*image:\s*quay\.io/dell/container-storage-modules/csm-authorization-controller\)\(:[^[:space:]]*\)\{0,1\}#\1:'"$ver"'#g' \
         -e 's#"image":[[:space:]]*"quay\.io/dell/container-storage-modules/csm-authorization-sidecar\(:[^"]*\)\{0,1\}"#"image": "quay.io/dell/container-storage-modules/csm-authorization-sidecar:'"$ver"'"#g' \
         -e 's#"image":[[:space:]]*"quay\.io/dell/container-storage-modules/csm-authorization-proxy\(:[^"]*\)\{0,1\}"#"image": "quay.io/dell/container-storage-modules/csm-authorization-proxy:'"$ver"'"#g' \
         -e 's#"image":[[:space:]]*"quay\.io/dell/container-storage-modules/csm-authorization-tenant\(:[^"]*\)\{0,1\}"#"image": "quay.io/dell/container-storage-modules/csm-authorization-tenant:'"$ver"'"#g' \
         -e 's#"image":[[:space:]]*"quay\.io/dell/container-storage-modules/csm-authorization-role\(:[^"]*\)\{0,1\}"#"image": "quay.io/dell/container-storage-modules/csm-authorization-role:'"$ver"'"#g' \
         -e 's#"image":[[:space:]]*"quay\.io/dell/container-storage-modules/csm-authorization-storage\(:[^"]*\)\{0,1\}"#"image": "quay.io/dell/container-storage-modules/csm-authorization-storage:'"$ver"'"#g' \
         -e 's#"image":[[:space:]]*"quay\.io/dell/container-storage-modules/csm-authorization-controller\(:[^"]*\)\{0,1\}"#"image": "quay.io/dell/container-storage-modules/csm-authorization-controller:'"$ver"'"#g' \
         -e 's#^\(\s*value:\s*quay\.io/dell/container-storage-modules/csm-authorization-sidecar\)\(:[^[:space:]]*\)\{0,1\}#\1:'"$ver"'#g' \
         -e 's#^\(\s*value:\s*quay\.io/dell/container-storage-modules/csm-authorization-proxy\)\(:[^[:space:]]*\)\{0,1\}#\1:'"$ver"'#g' \
         -e 's#^\(\s*value:\s*quay\.io/dell/container-storage-modules/csm-authorization-tenant\)\(:[^[:space:]]*\)\{0,1\}#\1:'"$ver"'#g' \
         -e 's#^\(\s*value:\s*quay\.io/dell/container-storage-modules/csm-authorization-role\)\(:[^[:space:]]*\)\{0,1\}#\1:'"$ver"'#g' \
         -e 's#^\(\s*value:\s*quay\.io/dell/container-storage-modules/csm-authorization-storage\)\(:[^[:space:]]*\)\{0,1\}#\1:'"$ver"'#g' \
         -e 's#^\(\s*value:\s*quay\.io/dell/container-storage-modules/csm-authorization-controller\)\(:[^[:space:]]*\)\{0,1\}#\1:'"$ver"'#g' \
         -e 's#quay\.io/dell/container-storage-modules/csm-authorization-\(sidecar\|proxy\|tenant\|role\|storage\|controller\):[^"'\''[:space:]]\+#quay.io/dell/container-storage-modules/csm-authorization-\1:'"$ver"'#g' \
         "$f"
   }

   retag_auth_module_dir() {
      local dir="$1" # $AUTH_ROOT/$auth_v2
      [[ -d "$dir" ]] || { echo "❌ Missing dir: $dir"; return 1; }

      local f="$dir/container.yaml"
      [[ -f "$f" ]] || { echo "↷ Skip missing: $f"; return 0; }

      # Precise image key retag for sidecar (common in module container.yaml)
      sed -i -E "s#^([[:space:]]*image:[[:space:]]*quay\.io/dell/container-storage-modules/csm-authorization-sidecar)(:[^[:space:]]+)?#\1:${auth_v2}#g" "$f"

      # Mop up any other forms
      update_auth_images_in_file "$f"
   }

   update_auth_bundle_manifest_images() {
         local csv="$BUNDLE_CSV"
   [[ -f "$csv" ]] || { echo "↷ Skip missing: $csv"; return 0; }

   # 1) Images & values (your existing helper)
   update_auth_images_in_file "$csv"

   # 2) Service keys embedding image strings (unchanged logic, just consistent quoting)
   sed -i'' \
      -e 's|"authorizationController": "quay\.io/dell/container-storage-modules/csm-authorization-controller\(:[^",]*\)\{0,1\}"|"authorizationController": "quay.io/dell/container-storage-modules/csm-authorization-controller:'"${auth_v2}"'"|g' \
      -e 's|"proxyService": "quay\.io/dell/container-storage-modules/csm-authorization-proxy\(:[^",]*\)\{0,1\}"|"proxyService": "quay.io/dell/container-storage-modules/csm-authorization-proxy:'"${auth_v2}"'"|g' \
      -e 's|"roleService": "quay\.io/dell/container-storage-modules/csm-authorization-role\(:[^",]*\)\{0,1\}"|"roleService": "quay.io/dell/container-storage-modules/csm-authorization-role:'"${auth_v2}"'"|g' \
      -e 's|"storageService": "quay\.io/dell/container-storage-modules/csm-authorization-storage\(:[^",]*\)\{0,1\}"|"storageService": "quay.io/dell/container-storage-modules/csm-authorization-storage:'"${auth_v2}"'"|g' \
      -e 's|"tenantService": "quay\.io/dell/container-storage-modules/csm-authorization-tenant\(:[^",]*\)\{0,1\}"|"tenantService": "quay.io/dell/container-storage-modules/csm-authorization-tenant:'"${auth_v2}"'"|g' \
      "$csv"
   }

   delete_n_minus_offset_dir() {
      local root_dir="$1"
      local offset="${2:-3}"
      local protect_dir_1="${3:-}"
      local protect_dir_2="${4:-}"

      [[ -d "$root_dir" ]] || { echo "❌ Root dir not found: $root_dir"; return 1; }

      mapfile -t dirs < <(cd "$root_dir" && ls -d */ 2>/dev/null | sed 's%/$%%' | sort -V)
      if (( ${#dirs[@]} == 0 )); then
         echo "ℹ️ No version directories in $root_dir; nothing to delete."
         return 0
      fi

      local last_index=$(( ${#dirs[@]} - 1 ))
      local del_index=$(( last_index - offset ))
      if (( del_index < 0 )); then
         echo "ℹ️ Not enough directories to delete n-${offset} (have ${#dirs[@]}). Skipping."
         return 0
      fi

      local dir_to_delete="${dirs[$del_index]}"
      if { [[ -n "$protect_dir_1" && "$dir_to_delete" == "$protect_dir_1" ]]; } || \
         { [[ -n "$protect_dir_2" && "$dir_to_delete" == "$protect_dir_2" ]]; }; then
         echo "ℹ️ n-${offset} resolves to a protected directory (${dir_to_delete}). Skipping delete."
         return 0
      fi

      echo "→ Deleting n-${offset} directory: $root_dir/$dir_to_delete"
      rm -rf "$root_dir/$dir_to_delete"
   }

   # Update module-level "configVersion" inside any JSON `"modules": [...]` block
   # for modules named "authorization" or "authorization-proxy-server".
   # Works within alm-examples (JSON-in-string) as well as any other JSON blocks in the file.
   # Usage: update_auth_bundle_manifest_configversion <file> <new_version>
   update_auth_bundle_manifest_configversion() {
      local f="$1"
      local newver="$2"

      [[ -f "$f" ]] || { echo "↷ Skip missing: $f"; return 0; }
      [[ -n "$newver" ]] || { echo "✖ Missing new version"; return 1; }

      # We write to a temp and then move it back to avoid partial edits.
      local tmp="${f}.tmp.$$"

      awk -v NEWVER="$newver" '
         # State vars
         BEGIN {
            in_modules = 0;            # we are inside a "modules": [ ... ] array
            collecting = 0;            # collecting one module object {...}
            depth = 0;                 # brace depth for the current collected object
            buf = "";                  # buffer for current module object
         }

         # Helper: count braces on a line (we accept braces in strings; it is fine for our use)
         function count_braces(s,    nopen, nclose) {
            nopen = gsub(/\{/, "{", s);          # increments per "{"
            nclose = gsub(/\}/, "}", s);         # increments per "}"
            return nopen - nclose;               # net change
         }

         {
            line = $0

            # Detect entering/exiting a modules array in JSON
            if (!collecting) {
            # Enter modules when we see "modules": [
            if (line ~ /"[[:space:]]*modules[[:space:]]*":[[:space:]]*\[/) {
               in_modules = 1
            }
            # Leave modules when the array ends ("]") and we are not collecting an object
            if (in_modules && line ~ /^[[:space:]]*\][[:space:]]*,?[[:space:]]*$/) {
               in_modules = 0
            }
            }

            # If inside modules and we see start of an object, begin collecting
            if (in_modules && !collecting && line ~ /^[[:space:]]*\{[[:space:]]*$/) {
            collecting = 1
            buf = line "\n"
            depth = 1
            next
            }

            # If we are collecting a module object, accumulate and track depth
            if (collecting) {
            buf = buf line "\n"
            depth += count_braces(line)

            # Did we finish the module object? (depth returns to 0)
            if (depth == 0) {
               # Decide whether to patch: only if module name is authorization or authorization-proxy-server
               if (buf ~ /"[[:space:]]*name[[:space:]]*":[[:space:]]*"authorization-proxy-server"/ \
                  || buf ~ /"[[:space:]]*name[[:space:]]*":[[:space:]]*"authorization"/) {

                  # Replace the module-level configVersion (quoted JSON)
                  # Only the first occurrence at module level is changed; other modules untouched.
                  gsub(/"configVersion"[[:space:]]*:[[:space:]]*"[^"]+"/, "\"configVersion\": \"" NEWVER "\"", buf)

                  # Optional: you can also update proxy/role/storage/tenant/controller fields if needed here.
               }

               # Print the patched (or original) module object and reset
               printf "%s", buf
               collecting = 0
               buf = ""
               next
            }

            # Keep collecting until the object ends
            next
            }

            # Default: print the line verbatim
            print line
         }
      ' "$f" > "$tmp" && mv "$tmp" "$f"
   }

   # ---------------------------------------------------------------------------
   # Main flow
   # ---------------------------------------------------------------------------

   mkdir -p "$AUTH_ROOT"
   cd "$AUTH_ROOT" || { echo "❌ Cannot cd to $AUTH_ROOT"; return 1; }

   # Resolve previous version for auth_v2 robustly (works whether target exists or not)
   local dir_to_copy
   dir_to_copy="$(previous_of_target "$AUTH_ROOT" "$auth_v2")" || return 1
   echo "ℹ️ Previous version resolved for ${auth_v2}: ${dir_to_copy}"

   # Do NOT recreate if target exists; only copy+retag when missing
   if [[ -d "${auth_v2}" ]]; then
      echo "ℹ️ Target already exists: $AUTH_ROOT/${auth_v2} — skipping copy/retag."
   else
      [[ -d "${dir_to_copy}" ]] || { echo "❌ Source directory not found: $AUTH_ROOT/${dir_to_copy}"; return 1; }
      echo "→ Copying from previous: ${dir_to_copy} -> ${auth_v2}"
      cp -r "${dir_to_copy}" "${auth_v2}"
      # Retag only when we actually created the target
      retag_auth_module_dir "$AUTH_ROOT/${auth_v2}"
   fi

   # Bundle CSV (images + configVersion)
   update_auth_bundle_manifest_images
   update_auth_bundle_manifest_configversion "$BUNDLE_CSV" "$auth_v2"

   # config/manager and bases CSV
   update_auth_images_in_file "$GITHUB_WORKSPACE/config/manager/manager.yaml"
   update_auth_images_in_file "$GITHUB_WORKSPACE/config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml"

   # deploy/operator.yaml
   update_auth_images_in_file "$GITHUB_WORKSPACE/deploy/operator.yaml"

   # config/samples — authorization v2 sample
   local samples_auth_v2="$GITHUB_WORKSPACE/config/samples/storage_v1_csm_authorization_v2.yaml"
   update_config_version_yq "$samples_auth_v2" "authorization-proxy-server" "$auth_v2"   
   update_auth_images_in_file "$samples_auth_v2"

   for f in "$GITHUB_WORKSPACE/config/samples/storage_v1_csm_powermax.yaml" \
            "$GITHUB_WORKSPACE/config/samples/storage_v1_csm_powerscale.yaml" \
            "$GITHUB_WORKSPACE/config/samples/storage_v1_csm_powerflex.yaml" \
            "$GITHUB_WORKSPACE/config/samples/storage_v1_csm_powerstore.yaml"; do
      update_config_version_yq "$f" "authorization" "$auth_v2" 
      update_auth_images_in_file "$f"
   done

   shopt -s nullglob


   for f in "$GITHUB_WORKSPACE/pkg/modules/testdata"/cr_*; do
   base="$(basename "$f")"

   case "$base" in
      # Auth proxy explicit skips
      cr_auth_proxy_v2.2.0.yaml|\
      cr_auth_proxy_v230.yaml|\
      cr_powerflex_observability_214.yaml|\
      cr_powerscale_observability_214.yaml|\
      cr_powermax_observability_214.yaml|\
      cr_powerscale_observability_with_topology.yaml|\
      cr_powermax_observability_214.yaml|\
      cr_powerflex_observability_with_old_otel_image.yaml)
         continue
         ;;
   esac

   update_config_version_yq "$f" "authorization-proxy-server" "$auth_v2" 
   update_auth_images_in_file "$f"
   done

   shopt -u nullglob

   local auth_samples="$GITHUB_WORKSPACE/samples/authorization/csm_authorization_proxy_server_${auth_v2}.yaml"
   if [[ -f "$auth_samples" ]]; then
      echo "2733-------> Updating ${auth_samples}"
      sed -i -E "s#^configVersion:\s*v.*$#configVersion: ${auth_v2}#g" "$auth_samples"
      update_auth_images_in_file "$auth_samples"
   fi

   # samples — driver-specific files (powermax/powerscale/powerflex/powerstore)
   local driver_dir_powermax="$GITHUB_WORKSPACE/samples/${CSI_POWERMAX:-}"
   local alt_driver_dir="$GITHUB_WORKSPACE/samples/${pmax_driver_ver:-}"
   for f in "$driver_dir_powermax/storage_csm_powermax_${pmax_driver_ver:-}.yaml" \
            "$alt_driver_dir/storage_csm_powermax_${pmax_driver_ver:-}.yaml" \
            "$driver_dir_powermax/storage_csm_powerscale_${pscale_driver_ver:-}.yaml" \
            "$driver_dir_powermax/storage_csm_powerflex_${pflex_driver_ver:-}.yaml" \
            "$driver_dir_powermax/storage_csm_powerstore_${pstore_driver_ver:-}.yaml" \
            "$driver_dir_powermax/minimal-samples/powermax_${pscale_driver_ver:-}.yaml" \
            "$driver_dir_powermax/minimal-samples/powerflex_${pflex_driver_ver:-}.yaml" \
            "$driver_dir_powermax/minimal-samples/powerscale_${pmax_driver_ver:-}.yaml" \
            "$driver_dir_powermax/minimal-samples/powerstore_${pstore_driver_ver:-}.yaml"; do
      [[ -f "$f" ]] || { echo "↷ Skip missing: $f"; continue; }
      case "$f" in
         *powerflex*) update_config_version_yq "$f" "authorization" "$auth_v2"  ;;
         *)           update_config_version_yq "$f" "authorization" "$auth_v2"   ;;
      esac
      update_auth_images_in_file "$f"
   done

   # tests/e2e/testfiles/authorization-templates
   for f in "$GITHUB_WORKSPACE/tests/e2e/testfiles/authorization-templates"/storage_csm_authorization_v2_proxy_server_conjur.yaml \
            "$GITHUB_WORKSPACE/tests/e2e/testfiles/authorization-templates"/storage_csm_authorization_v2_proxy_server_default_redis.yaml\
            "$GITHUB_WORKSPACE/tests/e2e/testfiles/authorization-templates"/storage_csm_authorization_v2_proxy_server_secret.yaml \
            "$GITHUB_WORKSPACE/tests/e2e/testfiles/authorization-templates"/storage_csm_authorization_v2_proxy_server_vault.yaml \
            "$GITHUB_WORKSPACE/tests/e2e/testfiles/authorization-templates"/storage_csm_authorization_v2_proxy_server.yaml; do
      [[ -f "$f" ]] || { echo "↷ Skip missing: $f"; continue; }
      update_config_version_yq "$f" "authorization-proxy-server" "$auth_v2" 
      update_auth_images_in_file "$f"
   done

   # tests/e2e/testfiles (minimal-testfiles)
   local mt_dir="$GITHUB_WORKSPACE/tests/e2e/testfiles/minimal-testfiles"
   if [[ -d "$mt_dir" ]]; then
      shopt -s nullglob
      for f in "$mt_dir"/*; do
      # ---- Default logic for all other files ----
         update_config_version_yq "$f" "authorization" "$auth_v2" 
         update_auth_images_in_file "$f" 
      done
      shopt -u nullglob
   fi


   # tests/e2e/testfiles — only update files that have an authorization block
   local tf_dir="$GITHUB_WORKSPACE/tests/e2e/testfiles"
   if [[ -d "$tf_dir" ]]; then
   shopt -s nullglob
   # Compute N-1 version from auth_v2 (e.g., v2.4.0 -> v2.3.0)
   auth_v2_minus_1="$(semver_n_minus_one "$auth_v2")"
   for f in "$tf_dir"/storage_csm_*; do
      base="$(basename "$f")"
      # Special-case file: set authorization to N-1
      if [[ "$base" == "storage_csm_powerflex_auth_n_minus_1.yaml" ]]; then
         # Only touch if module exists & change is needed
         update_config_version_yq "$f" "authorization" "$auth_v2_minus_1"
         # If your images should also reflect N-1, update them:
         update_auth_images_in_file "$f" "$auth_v2_minus_1"
         continue
      fi
      # Default behavior for other files
      if grep -q "name: authorization" "$f"; then
         update_config_version_yq "$f" "authorization" "$auth_v2"
      fi
      # Keep using the regular image update for auth (current version)
      update_auth_images_in_file "$f" "$auth_v2"
   done

   shopt -u nullglob
   fi

   # --- Prune n-PRUNE_OFFSET, protecting source & target ---
   delete_n_minus_offset_dir "$AUTH_ROOT" "$PRUNE_OFFSET" "$dir_to_copy" "$auth_v2"

   echo "✅ Authorization v2 Module -> ${auth_v2} updated successfully (no recreate if exists"  
   echo "✅ Authorization v2 Module -> ${auth_v2} updated successfully (no recreate if exists; pruned n-${PRUNE_OFFSET} safely)."
}


update_version_values_inplace() {
   set -Eeuo pipefail
   trap 'echo "❌ Error at line ${LINENO}: ${BASH_COMMAND}" >&2' ERR

   # Per-driver target versions; if a variable is empty, that driver is skipped.
   CSI_POWERSCALE="${CSI_POWERSCALE:-}"
   CSI_VXFLEXOS="${CSI_VXFLEXOS:-}"     # powerflex section
   CSI_POWERSTORE="${CSI_POWERSTORE:-}"
   CSI_POWERMAX="${CSI_POWERMAX:-}"

   local vv="$GITHUB_WORKSPACE/operatorconfig/moduleconfig/common/version-values.yaml"
   [[ -f "$vv" ]] || { echo "❌ Missing $vv"; return 1; }

   # Work on a temp file, streaming and updating line-by-line
   local tmp; tmp="$(mktemp)"

   awk -v V_PSCALE="$CSI_POWERSCALE" \
         -v V_PFLEX="$CSI_VXFLEXOS" \
         -v V_PSTORE="$CSI_POWERSTORE" \
         -v V_PMAX="$CSI_POWERMAX" \
         -v AUTH="$auth_v2" \
         -v REP="$rep_ver" \
         -v OBS="$obs_ver" \
         -v RES="$res_ver" \
         -v REVPROXY="$revproxy_ver" '
      # Track current driver section and whether we are inside the target version block
      BEGIN { driver=""; in_ver=0 }

      # Detect driver headers precisely at column 0
      /^[[:space:]]*powerscale:[[:space:]]*$/ { driver="powerscale"; in_ver=0; print; next }
      /^[[:space:]]*powerflex:[[:space:]]*$/  { driver="powerflex";  in_ver=0; print; next }
      /^[[:space:]]*powerstore:[[:space:]]*$/ { driver="powerstore"; in_ver=0; print; next }
      /^[[:space:]]*powermax:[[:space:]]*$/   { driver="powermax";   in_ver=0; print; next }

      # Enter target version block for the current driver
      # Version header lines are at 2-space indent: "  vX.Y.Z:"
      {
         if (driver=="powerscale" && V_PSCALE!="" && $0 ~ ("^[[:space:]]{2}" V_PSCALE ":[[:space:]]*$")) { in_ver=1; print; next }
         if (driver=="powerflex"  && V_PFLEX!=""  && $0 ~ ("^[[:space:]]{2}" V_PFLEX  ":[[:space:]]*$")) { in_ver=1; print; next }
         if (driver=="powerstore" && V_PSTORE!="" && $0 ~ ("^[[:space:]]{2}" V_PSTORE ":[[:space:]]*$")) { in_ver=1; print; next }
         if (driver=="powermax"   && V_PMAX!=""   && $0 ~ ("^[[:space:]]{2}" V_PMAX   ":[[:space:]]*$")) { in_ver=1; print; next }
      }

      # Leaving a version block: the next version header (2 spaces) ends the current block
      in_ver==1 && /^[[:space:]]{2}v[0-9]+\.[0-9]+\.[0-9]+:[[:space:]]*$/ { in_ver=0; print; next }

      # Also leave if the next top-level header (column 0) starts
      in_ver==1 && /^[^[:space:]]/ { in_ver=0; print; next }

      # === Within the target version block: update only module lines for that driver ===
      in_ver==1 && driver=="powerscale" {
         if     ($0 ~ /^[[:space:]]{4}authorization:[[:space:]]*/) { print "    authorization: \"" AUTH "\""; next }
         else if($0 ~ /^[[:space:]]{4}replication:[[:space:]]*/)   { print "    replication: \""   REP  "\""; next }
         else if($0 ~ /^[[:space:]]{4}observability:[[:space:]]*/) { print "    observability: \"" OBS  "\""; next }
         else if($0 ~ /^[[:space:]]{4}resiliency:[[:space:]]*/)    { print "    resiliency: \""    RES  "\""; next }
         else { print; next }
      }

      in_ver==1 && driver=="powerflex" {
         if     ($0 ~ /^[[:space:]]{4}authorization:[[:space:]]*/) { print "    authorization: \"" AUTH "\""; next }
         else if($0 ~ /^[[:space:]]{4}replication:[[:space:]]*/)   { print "    replication: \""   REP  "\""; next }
         else if($0 ~ /^[[:space:]]{4}observability:[[:space:]]*/) { print "    observability: \"" OBS  "\""; next }
         else if($0 ~ /^[[:space:]]{4}resiliency:[[:space:]]*/)    { print "    resiliency: \""    RES  "\""; next }
         else { print; next }
      }

      in_ver==1 && driver=="powerstore" {
         if     ($0 ~ /^[[:space:]]{4}resiliency:[[:space:]]*/)    { print "    resiliency: \""    RES  "\""; next }
         else if($0 ~ /^[[:space:]]{4}authorization:[[:space:]]*/) { print "    authorization: \"" AUTH "\""; next }
         else if($0 ~ /^[[:space:]]{4}observability:[[:space:]]*/) { print "    observability: \"" OBS  "\""; next }
         else if($0 ~ /^[[:space:]]{4}replication:[[:space:]]*/)   { print "    replication: \""   REP  "\""; next }
         else { print; next }
      }

      in_ver==1 && driver=="powermax" {
         if     ($0 ~ /^[[:space:]]{4}csireverseproxy:[[:space:]]*/) { print "    csireverseproxy: \"" REVPROXY "\""; next }
         else if($0 ~ /^[[:space:]]{4}authorization:[[:space:]]*/)   { print "    authorization: \""   AUTH    "\""; next }
         else if($0 ~ /^[[:space:]]{4}replication:[[:space:]]*/)     { print "    replication: \""     REP     "\""; next }
         else if($0 ~ /^[[:space:]]{4}observability:[[:space:]]*/)   { print "    observability: \""   OBS     "\""; next }
         else if($0 ~ /^[[:space:]]{4}resiliency:[[:space:]]*/)      { print "    resiliency: \""      RES     "\""; next }
         else { print; next }
      }

      # Default: pass-through
      { print }
   ' "$vv" > "$tmp"

   mv "$tmp" "$vv"

   echo "✅ Updated values in ${vv} (in-place):"
   [[ -n "$CSI_POWERSCALE" ]] && echo "   - powerscale: ${CSI_POWERSCALE}"
   [[ -n "$CSI_VXFLEXOS"  ]] && echo "   - powerflex : ${CSI_VXFLEXOS}"
   [[ -n "$CSI_POWERSTORE" ]] && echo "   - powerstore: ${CSI_POWERSTORE}"
   [[ -n "$CSI_POWERMAX"   ]] && echo "   - powermax  : ${CSI_POWERMAX}"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
   update_observability_tag_only
   update_resiliency_tag_only
   update_replication_tag_only
   update_reverseproxy_tag_only
   update_authorization_v2_tag_only
   update_version_values_inplace
fi

echo "<<< ------- Module version update complete ------- >>>"
