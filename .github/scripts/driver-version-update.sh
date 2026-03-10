#!/bin/bash

# Copyright © 2025 - 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Dell Technologies, Dell and other trademarks are trademarks of Dell Inc.
# or its subsidiaries. Other trademarks may be trademarks of their respective
# owners.



# Script updates the driver version to align with the CSM release version
# It will update:
# - bundle CSV configVersion and relatedImages
# - config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml
# - samples folder
#
# Usage:
# Note: include --previous_patch_version of the driver if increment_type is "patch"
#
# bash ./.github/scripts/driver-version-update.sh --increment_type "minor" --csm-version "1.17.0" --powerscale_version "2.17.0" --powermax_version "2.17.0" --powerflex_version "2.17.0" --powerstore_version "2.17.0" --unity_version "2.17.0" --cosi_version "1.1.0"

cd "$GITHUB_WORKSPACE"

increment_type=""
previous_patch_version=""
csm_version=""
powerscale_version=""
powermax_version=""
powerflex_version=""
powerstore_version=""
unity_version=""
cosi_version=""

GITHUB_WORKSPACE="${GITHUB_WORKSPACE:-$(pwd)}"
CSV="$GITHUB_WORKSPACE/bundle/manifests/dell-csm-operator.clusterserviceversion.yaml"
config_CSV="$GITHUB_WORKSPACE/config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml"
manager="$GITHUB_WORKSPACE/config/manager/manager.yaml"
deploy="$GITHUB_WORKSPACE/deploy/operator.yaml"

# Parse args (versions may include leading v)
options=$(getopt -o "" -l "increment_type:,previous_patch_version:,csm_version:,powerscale_version:,powermax_version:,powerflex_version:,powerstore_version:,unity_version:,cosi_version:" -- "$@")
if [ $? -ne 0 ]; then
    echo "Invalid arguments."
    exit 1
fi
eval set -- "$options"

# Read the named argument values
while [ $# -gt 0 ]; do
    case "$1" in
    --increment_type)
        increment_type="$2"
        shift
        ;;
    --previous_patch_version)
        previous_patch_version="$2"
        shift
        ;;
    --csm_version)
        csm_version="$2"
        shift
        ;;
    --powerscale_version)
        powerscale_version="$2"
        shift
        ;;
    --powermax_version)
        powermax_version="$2"
        shift
        ;;
    --powerflex_version)
        powerflex_version="$2"
        shift
        ;;
    --powerstore_version)
        powerstore_version="$2"
        shift
        ;;
    --unity_version)
        unity_version="$2"
        shift
        ;;
    --cosi_version)
        cosi_version="$2"
        shift
        ;;
    --) shift ;;
    esac
    shift
done

# ----------------
# Helper functions
# ----------------

# Trim whitespace and strip leading 'v' from a version string, returning plain numeric version.
# Supports:
#   - "123"  => 1.2.3
#   - "1204" => 1.20.4
#   - "1.2.3", "v1.2.3" => 1.2.3
Normalize_version() {
    local v="$1"
    v=$(printf '%s' "$v" | tr -d '\r\n' | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' -e 's/^v//')
    if [[ -z "$v" ]]; then
        echo "Version is empty"
        return
    fi

    # semver format
    if [[ $v =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
        printf '%s\n' "$v"
        return 0
    fi

    # 4 digits => M MM P
    if [[ $v =~ ^[0-9]{4}$ ]]; then
        major=${v:0:1}
        minor=${v:1:2}
        patch=${v:3:1}
        printf '%s.%s.%s\n' "$major" "$minor" "$patch"
        return 0
    fi

    # 3 digits => M M P
    if [[ $v =~ ^[0-9]{3}$ ]]; then
        major=${v:0:1}
        minor=${v:1:1}
        patch=${v:2:1}
        printf '%s.%s.%s\n' "$major" "$minor" "$patch"
        return 0
    fi

    # numeric format (accept "1_2_3" or "1-2-3")
    if [[ $v =~ ^([0-9]+)([0-9]{2})([0-9]{2})$ ]]; then
        printf '%s.%s.%s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[3]}"
        return 0
    fi

    echo "Invalid version format: '$v'" >&2
    exit 1
}

# Extract major/minor/patch from a version suffix (e.g., 2170 -> 2.17.0)
ParseSemver() {
    local version_suffix=$1
    local version=$(Normalize_version "$version_suffix")
    if [[ ! $version =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
        echo "Invalid normalized version '$version' (expected X.Y.Z digits-only)" >&2
        exit 1
    fi

    major=$((10#${BASH_REMATCH[1]}))
    minor=$((10#${BASH_REMATCH[2]}))
    patch=$((10#${BASH_REMATCH[3]}))

    new_version="${major}.${minor}.${patch}"
}

DeletePathsIfExist() {
    local pattern p
    for pattern in "$@"; do
        for p in $pattern; do
            [[ -e "$p" ]] || continue
            if [[ -f "$p" ]]; then
                echo "Deleting file $p"
                rm -f "$p"
            elif [[ -d "$p" ]]; then
                echo "Deleting directory $p"
                rm -rf "$p"
            fi
        done
    done
}

samples_root_for_prefix() {
    local prefix=$1
    if [[ "$prefix" == "storage_csm_cosi" || "$prefix" == "cosi" ]]; then
        echo "samples/cosi"
    else
        echo "samples"
    fi
}

# Get the current latest (n-1) driver version before adding support for the new version.
GetLatestDriverVersion() {
    local prefix=$1
    local search_paths=$(samples_root_for_prefix "$prefix")"/v*/"
    files=$(find $search_paths -type f -name "${prefix}_v*.yaml")
    if [ -z "$files" ]; then
        echo "No samples found for prefix: $prefix"
        return
    fi

    latest_file=$(echo "$files" | sort -V | tail -1)
    version_suffix=$(basename "$latest_file" | sed -E "s/^${prefix}_v([0-9]+)\.yaml$/\1/")

    # Extract digits from version suffix safely (e.g., 2160 -> 2.16.0)
    ParseSemver "$version_suffix"

    latest_driver_version="${major}.${minor}.${patch}"
    echo "Latest driver version: $latest_driver_version"
}

# Get the second latest (n-2) driver version before adding support for the new version.
GetSecondLatestDriverVersion() {
    local prefix=$1
    local files=$(find samples/v*/ -type f -name "${prefix}_v*.yaml")
    if [ -z "$files" ]; then
        echo "No samples found for prefix: $prefix"
        return
    fi

    # Extract semantic versions from filenames like v2140 → 2.14.0
    versions=$(echo "$files" | sed -E "s|.*/${prefix}_v([0-9]{1})([0-9]{2})([0-9]{1})\.yaml|\1.\2.\3|" | sort -V)
    # Get unique minor versions (e.g., 2.14, 2.15)
    minor_versions=$(echo "$versions" | awk -F. '{print $1"."$2}' | sort -V | uniq)
    # Get the second latest minor version
    prev_minor=$(echo "$minor_versions" | tail -2 | head -1)
    # Filter versions matching that minor version and get highest patch
    highest_patch=$(echo "$versions" | grep "^${prev_minor}\." | sort -V | tail -1)
    echo "N-2 driver version: $highest_patch"
}

# Determines minimal upgrade path
GetMinUpgradePath() {
    local prefix=$1
    local search_paths=$(samples_root_for_prefix "$prefix")"/v*/"
    files=$(find $search_paths -type f -name "${prefix}_v*.yaml")
    if [ -z "$files" ]; then
        echo "No minimum upgrade path found for prefix: $prefix"
        return
    fi

    oldest_file=$(echo "$files" | sort -V | head -1)
    version_suffix=$(basename "$oldest_file" | grep -oE '_v[0-9]+' | grep -oE '[0-9]+')
    ParseSemver "$version_suffix"
    echo "Minimum upgrade path: $new_version"
}

# ----------------
# Create functions
# ----------------

# Create the latest driver sample file under the correct versioned folder.
CreateLatestSampleFile() {
    local prefix=$1 version=$2 csm_version=$3

    samples_root=$(samples_root_for_prefix "$prefix")
    folder="$samples_root/v*/"
    versioned_folder="$samples_root/v$version"
    version_suffix=$(echo "$version" | tr -d '.' | tr -d '\n')

    local latest_file="" latest_version=""
    # Search for files inside versioned folders: samples/v*/[prefix]_v*.yaml
    for file in $(find $folder -type f -name "${prefix}_v*.yaml"); do
        version_part=$(basename "$file" | grep -oE '[0-9]+')
        if [[ $version_part -gt ${latest_version:-0} ]]; then
            latest_version=$version_part
            latest_file=$file
        fi
    done

    echo "Creating sample file: $versioned_folder/${prefix}_v${version_suffix}.yaml"
    mkdir -p "$versioned_folder"
    cp -v --update "$latest_file" "$versioned_folder/${prefix}_v${version_suffix}.yaml"

    UpdateVersionField "$versioned_folder/${prefix}_v${version_suffix}.yaml" "$version" "$csm_version" "$dd_repo"

    echo "Copying sample file to the config/samples directory"
    cp -v -a --update "$versioned_folder/${prefix}_v${version_suffix}.yaml" "$dd_config_sample_target"
}

# Create the latest minimal sample file in the samples directory
CreateLatestMinimalSampleFile() {
    local prefix=$1 version=$2 csm_version=$3

    samples_root=$(samples_root_for_prefix "$prefix")
    folder="$samples_root/v*/minimal-samples"
    versioned_folder="$samples_root/v$version/minimal-samples"
    version_suffix=$(echo "$version" | tr -d '.' | tr -d '\n')

    local latest_file="" latest_version=""
    # Search for files inside versioned folders: samples/v*/minimal-samples/[prefix]_v*.yaml
    for file in $(find $folder -type f -name "${prefix}_v*.yaml"); do
        version_part=$(basename "$file" | grep -oE '[0-9]+')
        if [[ $version_part -gt ${latest_version:-0} ]]; then
            latest_version=$version_part
            latest_file=$file
        fi
    done

    echo "Creating minimal sample file: $versioned_folder/${prefix}_v${version_suffix}.yaml"
    mkdir -p "$versioned_folder"
    cp -v --update "$latest_file" "$versioned_folder/${prefix}_v${version_suffix}.yaml"

    UpdateVersionField "$versioned_folder/${prefix}_v${version_suffix}.yaml" "$version" "$csm_version" "$dd_repo"
}

# Create the latest csm-images configmap in the samples directory
CreateConfigMap() {
    local version=$1 csm_version=$2 driver_key=$3
    cm="k8s_configmap.yaml"
    folder="samples/v*/"
    local latest_file="" latest_version=""
    if [[ "$driver_key" != "cosi" ]]; then
        versioned_folder="samples/v$version"
        for file in $(find $folder -type f -name "${cm}"); do
            version_part=$(basename "$file" | grep -oE '[0-9]+')
            if [[ $version_part -gt ${latest_version:-0} ]]; then
                latest_version=$version_part
                latest_file=$file
            fi
        done

        echo "Creating csm-images configmap file: $cm"
        mkdir -p "$versioned_folder"
        cp -v --update "$latest_file" "$versioned_folder/$cm"
    fi

    # update csm version
    CSM_VERSION="$csm_version" \
    yq eval -i '.data."versions.yaml" |= (from_yaml | (.[0].version = strenv(CSM_VERSION)) | to_yaml)' "$versioned_folder/$cm"

    # update image tags
    if [[ -n "$dd_configmap_image_key" ]]; then
        CSM_VERSION="$csm_version" IMAGE_KEY="$dd_configmap_image_key" IMAGE_VALUE="quay.io/dell/container-storage-modules/$dd_repo:v$version" \
        yq eval -i '.data."versions.yaml" |= (from_yaml | (.[] | select(.version == strenv(CSM_VERSION)) | .images[strenv(IMAGE_KEY)]) = strenv(IMAGE_VALUE) | to_yaml)' "$versioned_folder/$cm"
    fi
}

# ----------------
# Update functions
# ----------------

# Update configVersion, version, and related images in dell-csm-operator.clusterserviceversion.yaml
UpdateCSV() {
    local repo="$1" version="$2" csm_version="$3" file="$4"
    echo "Updating $repo in $file"
    local image="quay.io/dell/container-storage-modules/$repo:$version"

    # update version
    sed -i -E "s/^([[:space:]]*)\"version\":[[:space:]]*\"[^\"]*\"/\\1\"version\": \"$csm_version\"/" "$file"

    # update configVersion
    if [[ "$repo" != "cosi" ]]; then
        sed -i -E "s/^([[:space:]]*)\"configVersion\":[[:space:]]*\"[^\"]*\"/\\1\"configVersion\": \"$version\"/" "$file"
    else
        REPO="$repo" DRIVER_VERSION="$version" \
        perl -0777 -i -pe 'BEGIN{ $repo=$ENV{"REPO"}; $ver=$ENV{"DRIVER_VERSION"}; }
          s/("configVersion"\s*:\s*")([^"]*)("(?:(?!"configVersion").)*?"csiDriverType"\s*:\s*"\Q$repo\E")/${1}$ver$3/sg;
        ' "$file"
    fi

    # Update RELATED_IMAGE_* env vars under deployments (CSV install spec)
    local env_name="RELATED_IMAGE_${repo}"
    ENV_NAME="$env_name" ENV_VALUE="$image" \
    perl -0777 -i -pe 'BEGIN{ $n=$ENV{"ENV_NAME"}; $v=$ENV{"ENV_VALUE"}; }
      s/(\n[ \t]*-[ \t]*name:[ \t]*\Q$n\E[ \t]*\n[ \t]*value:[ \t]*)[^\n]*/${1}$v/gm;
    ' "$file"

    # Update spec.relatedImages
    yq eval -i "(.. | select(has(\"spec\") and (.spec | has(\"relatedImages\"))) | .spec.relatedImages[] | select(.name == \"$repo\") | .image) = \"$image\"" "$file"
}

# Update images in templates, like image field in containers
UpdateTemplates() {
    local file=$1 kind=$2 container_name=$3 new_image=$4

    echo "Updating container image in $file"
    CONTAINER_NAME="$container_name" IMAGE="$new_image" \
    yq eval -i '(
      .. |
      select(type == "!!map") |
      select(has("spec")) |
      .spec |
      select(has("template")) |
      .template |
      select(has("spec")) |
      .spec |
      select(has("'"$kind"'")) |
      ."'"$kind"'"[] |
      select(.name == strenv(CONTAINER_NAME)) |
      .image
    ) = strenv(IMAGE)' "$file"
}

# Update related images in templates, like RELATED_IMAGE_* env variables
UpdateRelatedImages() {
    local file=$1 env_name=$2 new_value=$3

    echo "Updating $env_name in $file"
    ENV_NAME="$env_name" ENV_VALUE="$new_value" \
    yq eval -i '(. | select(.kind == "Deployment") | .spec.template.spec.containers[]? | select(.name == "manager") | .env[]? | select(.name == strenv(ENV_NAME)) | .value) = strenv(ENV_VALUE)' "$file"
}

# Update versions in csm-version-mapping.yaml
UpdateCSMVersionMapping() {
    local file=$1 driver=$2 csm_version=$3 version=$4 csm_major=$5 delete_minor_version=$6
    echo "Updating $driver version in $file"
    DRIVER="$driver" CSM_VERSION="$csm_version" VERSION="$version" \
    yq eval -i '
      .[strenv(DRIVER)] = (
        { (strenv(CSM_VERSION)) : strenv(VERSION) | . style="double" }
        + ((.[strenv(DRIVER)] // {}) | del(.[strenv(CSM_VERSION)]))
      )
    ' "$file"

    # delete n-3 version
    if [[ -n "$delete_minor_version" ]]; then
        local prefix="v${csm_major}.${delete_minor_version}."
        local keys
        keys=$(yq eval -r ".[\"$driver\"] | keys | .[]" "$file")
        if [[ -n "$keys" ]]; then
            while IFS= read -r key; do
                if [[ -n "$key" && "$key" == ${prefix}* ]]; then
                    yq eval -i "del(.[\"$driver\"][\"$key\"])" "$file"
                fi
            done <<< "$keys"
        fi
    fi
}

# Update version, configVersion, and image field in the samples files if they exist
UpdateVersionField() {
    local file=$1 version=$2 csm_version=$3 repo=$4
    # update configVersion
    yq eval -i '(. as $doc | ($doc | select(.spec.driver.configVersion?) | .spec.driver.configVersion = "'"v$version"'") // $doc)' "$file"

    # update image
    local current_image image_prefix
    current_image=$(yq eval -r '.spec.driver.common.image // ""' "$file" 2>/dev/null || true)
    image_prefix="quay.io/dell/container-storage-modules/$repo:"
    if [[ -n "$current_image" && "$current_image" == "$image_prefix"* ]]; then
        yq eval -i '(. as $doc | ($doc | select(.spec.driver.common.image?) | .spec.driver.common.image = "'"quay.io/dell/container-storage-modules/$repo:v$version"'") // $doc)' "$file"
    fi

    # update version
    yq eval -i '(. as $doc | ($doc | select(.spec.version?) | .spec.version = "'"$csm_version"'") // $doc)' "$file"
}

UpdateDriver() {
    local driver_key=$1 version=$2 csm_version=$3 increment_type=$4 previous_patch_version=$5
    if [[ -z "$version" ]] || [[ -z "$csm_version" ]]; then
        echo "Skipping $driver_key: version not provided" >&2
        return 1
    fi

    if [[ "$increment_type" == "patch" ]]; then
        if [[ -z "$previous_patch_version" ]]; then
            echo "Skipping $driver_key: previous_patch_version not provided" >&2
            return 1
        fi
    fi

    if ! LoadDriverDescriptor "$driver_key"; then
        return 1
    fi

    echo "Processing $driver_key version: $version, csm version: $csm_version"

    normalized_csm_version=$(Normalize_version "$csm_version")
    semantic_csm_version="v$normalized_csm_version"
    # Parse csm version to get major/minor/patch for pruning
    ParseSemver "$normalized_csm_version"
    csm_major=$major
    csm_minor=$minor

    local csm_delete_minor_version
    csm_delete_minor_version=$((10#$csm_minor - 3))
    if (( csm_delete_minor_version < 0 )); then
        csm_delete_minor_version=""
    fi

    normalized_version=$(Normalize_version "$version")
    semantic_version="v$normalized_version"
    version_suffix=$(echo "$semantic_version" | tr -d '.' | tr -d '\n')
    image="$dd_image_repo:$semantic_version"

    # Parse driver version to get major/minor/patch for folder + pruning
    ParseSemver "$normalized_version"

    local previous_version n_version delete_minor_version driver_delete_version min_upgrade_path
    if [[ "$increment_type" == "patch" ]] && [[ -n "$previous_patch_version" ]]; then
        previous_version=$previous_patch_version
    else
        previous_version=$(Normalize_version "$(GetLatestDriverVersion "$dd_prefix" | tail -n 1 | awk '{print $NF}')")
    fi
    echo "Previous version: $previous_version"
    n_version="v$previous_version"
    delete_minor_version=$((minor - 3))
    if (( delete_minor_version < 0 )); then
        delete_minor_version=""
    fi

    UpdateCSV "$dd_repo" "$semantic_version" "$semantic_csm_version" "$CSV"
    UpdateCSV "$dd_repo" "$semantic_version" "$semantic_csm_version" "$config_CSV"
    UpdateRelatedImages "$manager" "$dd_manager_related_image_env" "$image"
    UpdateRelatedImages "$deploy" "$dd_manager_related_image_env" "$image"
    CreateLatestSampleFile "$dd_prefix" "$normalized_version" "$semantic_csm_version"
    CreateLatestMinimalSampleFile "$dd_minimal_prefix" "$normalized_version" "$semantic_csm_version"
    CreateConfigMap "$normalized_version" "$semantic_csm_version" "$driver_key"
    UpdateCSMVersionMapping "operatorconfig/common/csm-version-mapping.yaml" "$dd_driver_key" "$semantic_csm_version" "$semantic_version" "$csm_major" "$csm_delete_minor_version"

    echo "Copying template files to operatorconfig/driverconfig directory"
    cp -v -a --update "$dd_operator_dir/$n_version/." "$dd_operator_dir/$semantic_version"

    UpdateTemplates "$dd_operator_dir/$semantic_version/controller.yaml" "containers" "$dd_driver_container_name" "$image"
    UpdateTemplates "$dd_operator_dir/$semantic_version/node.yaml" "containers" "$dd_driver_container_name" "$image"
    if [[ -n "$dd_init_container_name" ]]; then
        UpdateTemplates "$dd_operator_dir/$semantic_version/node.yaml" "initContainers" "$dd_init_container_name" "$image"
    fi

    # Delete the n-3 support so that minimum upgrade path determines the minimum supported version
    DeletePathsIfExist "samples/v${major}.${delete_minor_version}.*/minimal-samples/${dd_minimal_prefix}_v*.yaml"
    DeletePathsIfExist "samples/v${major}.${delete_minor_version}.*/${dd_prefix}_v*.yaml"
    DeletePathsIfExist "$dd_operator_dir/v${major}.${delete_minor_version}."*

    min_upgrade_path=$(GetMinUpgradePath "$dd_prefix" | awk '{print $NF}')
    yq -i '.minUpgradePath = "v'"$min_upgrade_path"'"' "$dd_operator_dir/$semantic_version/upgrade-path.yaml"

    echo "Copying test files to tests/config directory"
    cp -v -a --update "$dd_tests_dir/$n_version/." "$dd_tests_dir/$semantic_version"
    DeletePathsIfExist "$dd_tests_dir/v${major}.${delete_minor_version}.*"

    UpdateTemplates "$dd_tests_dir/$semantic_version/controller.yaml" "containers" "$dd_driver_container_name" "$image"
    UpdateTemplates "$dd_tests_dir/$semantic_version/node.yaml" "containers" "$dd_driver_container_name" "$image"
    if [[ -n "$dd_init_container_name" ]]; then
        UpdateTemplates "$dd_tests_dir/$semantic_version/node.yaml" "initContainers" "$dd_init_container_name" "$image"
    fi
    yq -i '.minUpgradePath = "v'"$min_upgrade_path"'"' "$dd_tests_dir/$semantic_version/upgrade-path.yaml"

    if [[ -n "$dd_testdata_files" ]]; then
        for f in $dd_testdata_files; do
            UpdateVersionField "$f" "$version" "$csm_version" "$dd_repo"
        done
    fi

    if [[ -n "$dd_e2e_name_prefix" ]]; then
        for f in $(find tests/e2e/testfiles -type f -name "${dd_e2e_name_prefix}*"); do
            UpdateVersionField "$f" "$version" "$csm_version" "$dd_repo"
        done
        for f in $(find tests/e2e/testfiles/minimal-testfiles -type f -name "${dd_e2e_name_prefix}*"); do
            UpdateVersionField "$f" "$version" "$csm_version" "$dd_repo"
        done
    fi

    if [[ -n "$dd_downgrade_files" ]]; then
        local n_minus_1_version n_minus_1_image second_previous_version n_minus_2_version n_minus_2_image
        n_minus_1_version="$previous_version"

        second_previous_version=$(GetSecondLatestDriverVersion "$dd_prefix" | awk '{print $NF}')
        n_minus_2_version="$second_previous_version"

        for f in $dd_downgrade_files; do
            if [[ ! -f "$f" && -f "tests/e2e/testfiles/$f" ]]; then
                f="tests/e2e/testfiles/$f"
            fi
            if [[ ! -f "$f" ]]; then
                echo "Skipping downgrade file update: '$f' not found" >&2
                continue
            fi
            if [[ "$n_minus_1_version" == "$n_version" ]]; then
                UpdateVersionField "$f" "$n_minus_2_version" "$csm_version" "$dd_repo"
            else
                UpdateVersionField "$f" "$n_minus_1_version" "$csm_version" "$dd_repo"
            fi
        done
    fi
}

UpdateBadDriver() {
    local version=$1
    normalized_version=$(Normalize_version "$version")
    # Parse authoritative version to get major/minor/patch for folder + pruning
    ParseSemver "$normalized_version"

    previous_minor_version=$((minor - 1))
    previous_major_driver_version="$major.$previous_minor_version.$patch"

    cp -a --update tests/config/driverconfig/badDriver/v$previous_major_driver_version/. tests/config/driverconfig/badDriver/v$version
    delete_minor_version=$((minor - 3))
    DeletePathsIfExist tests/config/driverconfig/badDriver/v$major.$delete_minor_version.*
}

# ----------------
# Entry point
# ----------------

LoadDriverDescriptor() {
    local driver_key=$1

    case "$driver_key" in
    powerflex)
        dd_driver_key="powerflex"
        dd_prefix="storage_csm_powerflex"
        dd_minimal_prefix="powerflex"
        dd_repo="csi-vxflexos"
        dd_image_repo="quay.io/dell/container-storage-modules/csi-vxflexos"
        dd_operator_dir="operatorconfig/driverconfig/powerflex"
        dd_tests_dir="tests/config/driverconfig/powerflex"
        dd_config_sample_target="config/samples/storage_v1_csm_powerflex.yaml"
        dd_manager_related_image_env="RELATED_IMAGE_csi-vxflexos"
        dd_driver_container_name="driver"
        dd_init_container_name="mdm-container"
        dd_configmap_image_key="powerflex"
        dd_downgrade_files="storage_csm_powerflex_auth_n_minus_1.yaml storage_csm_powerflex_downgrade.yaml"
        dd_testdata_files="pkg/modules/testdata/cr_powerflex_observability_custom_cert_missing_key.yaml pkg/modules/testdata/cr_powerflex_observability_custom_cert.yaml pkg/modules/testdata/cr_powerflex_observability.yaml pkg/modules/testdata/cr_powerflex_replica.yaml pkg/modules/testdata/cr_powerflex_resiliency.yaml"
        dd_e2e_name_prefix="storage_csm_powerflex"
        ;;
    powermax)
        dd_driver_key="powermax"
        dd_prefix="storage_csm_powermax"
        dd_minimal_prefix="powermax"
        dd_repo="csi-powermax"
        dd_image_repo="quay.io/dell/container-storage-modules/csi-powermax"
        dd_operator_dir="operatorconfig/driverconfig/powermax"
        dd_tests_dir="tests/config/driverconfig/powermax"
        dd_config_sample_target="config/samples/storage_v1_csm_powermax.yaml"
        dd_manager_related_image_env="RELATED_IMAGE_csi-powermax"
        dd_driver_container_name="driver"
        dd_init_container_name=""
        dd_configmap_image_key="powermax"
        dd_downgrade_files=""
        dd_testdata_files="pkg/modules/testdata/cr_powermax_observability_use_secret.yaml pkg/modules/testdata/cr_powermax_observability.yaml pkg/modules/testdata/cr_powermax_replica.yaml pkg/modules/testdata/cr_powermax_resiliency.yaml pkg/modules/testdata/cr_powermax_reverseproxy_sidecar.yaml pkg/modules/testdata/cr_powermax_reverseproxy_use_secret.yaml pkg/modules/testdata/cr_powermax_reverseproxy.yaml"
        dd_e2e_name_prefix="storage_csm_powermax"
        ;;
    powerscale)
        dd_driver_key="powerscale"
        dd_prefix="storage_csm_powerscale"
        dd_minimal_prefix="powerscale"
        dd_repo="csi-isilon"
        dd_image_repo="quay.io/dell/container-storage-modules/csi-isilon"
        dd_operator_dir="operatorconfig/driverconfig/powerscale"
        dd_tests_dir="tests/config/driverconfig/powerscale"
        dd_config_sample_target="config/samples/storage_v1_csm_powerscale.yaml"
        dd_manager_related_image_env="RELATED_IMAGE_csi-isilon"
        dd_driver_container_name="driver"
        dd_init_container_name=""
        dd_configmap_image_key="isilon"
        dd_downgrade_files=""
        dd_testdata_files="pkg/modules/testdata/cr_powerscale_observability.yaml pkg/modules/testdata/cr_powerscale_replica.yaml pkg/modules/testdata/cr_powerscale_resiliency.yaml"
        dd_e2e_name_prefix="storage_csm_powerscale"
        ;;
    powerstore)
        dd_driver_key="powerstore"
        dd_prefix="storage_csm_powerstore"
        dd_minimal_prefix="powerstore"
        dd_repo="csi-powerstore"
        dd_image_repo="quay.io/dell/container-storage-modules/csi-powerstore"
        dd_operator_dir="operatorconfig/driverconfig/powerstore"
        dd_tests_dir="tests/config/driverconfig/powerstore"
        dd_config_sample_target="config/samples/storage_v1_csm_powerstore.yaml"
        dd_manager_related_image_env="RELATED_IMAGE_csi-powerstore"
        dd_driver_container_name="driver"
        dd_init_container_name=""
        dd_configmap_image_key="powerstore"
        dd_downgrade_files=""
        dd_testdata_files="pkg/modules/testdata/cr_powerstore_resiliency.yaml pkg/modules/testdata/cr_powerstore_observability.yaml pkg/modules/testdata/cr_powerstore_replica.yaml"
        dd_e2e_name_prefix="storage_csm_powerstore"
        ;;
    unity)
        dd_driver_key="unity"
        dd_prefix="storage_csm_unity"
        dd_minimal_prefix="unity"
        dd_repo="csi-unity"
        dd_image_repo="quay.io/dell/container-storage-modules/csi-unity"
        dd_operator_dir="operatorconfig/driverconfig/unity"
        dd_tests_dir="tests/config/driverconfig/unity"
        dd_config_sample_target="config/samples/storage_v1_csm_unity.yaml"
        dd_manager_related_image_env="RELATED_IMAGE_csi-unity"
        dd_driver_container_name="driver"
        dd_init_container_name=""
        dd_configmap_image_key="unity"
        dd_downgrade_files=""
        dd_testdata_files=""
        dd_e2e_name_prefix="storage_csm_unity"
        ;;
    cosi)
        dd_driver_key="cosi"
        dd_prefix="storage_csm_cosi"
        dd_minimal_prefix="cosi"
        dd_repo="cosi"
        dd_image_repo="quay.io/dell/container-storage-modules/cosi"
        dd_operator_dir="operatorconfig/driverconfig/cosi"
        dd_tests_dir="tests/config/driverconfig/cosi"
        dd_config_sample_target="config/samples/storage_v1_csm_cosi.yaml"
        dd_manager_related_image_env="RELATED_IMAGE_cosi"
        dd_driver_container_name="driver"
        dd_init_container_name=""
        dd_configmap_image_key="cosi"
        dd_downgrade_files=""
        dd_testdata_files=""
        dd_e2e_name_prefix="storage_csm_cosi"
        ;;
    *)
        echo "Unknown driver key: $driver_key" >&2
        return 1
        ;;
    esac
}

if [[ -n "$powerflex_version" ]]; then
    UpdateDriver powerflex $powerflex_version $csm_version $increment_type $previous_patch_version
fi

if [[ -n "$powermax_version" ]]; then
    UpdateDriver powermax $powermax_version $csm_version $increment_type $previous_patch_version
fi

if [[ -n "$powerscale_version" ]]; then
    UpdateDriver powerscale $powerscale_version $csm_version $increment_type $previous_patch_version
    UpdateBadDriver $powerscale_version
fi

if [[ -n "$powerstore_version" ]]; then
    UpdateDriver powerstore $powerstore_version $csm_version $increment_type $previous_patch_version
fi

if [[ -n "$unity_version" ]]; then
    UpdateDriver unity $unity_version $csm_version $increment_type $previous_patch_version
fi

if [[ -n "$cosi_version" ]]; then
    UpdateDriver cosi $cosi_version $csm_version $increment_type $previous_patch_version
fi
