#!/bin/bash
# yq is prerequisite for this script
# Simplified driver version update script with configuration-driven approach
# Usage: bash ./.github/scripts/driver-version-update-simplified.sh --driver_update_type "major" --csm_version "v1.11.0" --powerscale_version "2.16.0" --powermax_version "2.16.0" --powerflex_version "2.16.0" --powerstore_version "2.16.0" --unity_version "2.16.0"

cd "$GITHUB_WORKSPACE"

# Initialize variables with default values
driver_update_type=""
csm_version=""
powerscale_version=""
powermax_version=""
powerflex_version=""
powerstore_version=""
unity_version=""
cosi_version=""

# Set options for the getopt command
options=$(getopt -o "" -l "driver_update_type:,csm_version:,powerscale_version:,powermax_version:,powerflex_version:,powerstore_version:,unity_version:,cosi_version:" -- "$@")
if [ $? -ne 0 ]; then
    echo "Invalid arguments."
    exit 1
fi
eval set -- "$options"

# Read the named argument values
while [ $# -gt 0 ]; do
    case "$1" in
    --driver_update_type)
        driver_update_type="$2"
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

# Driver configuration mapping
declare -A DRIVER_CONFIGS
DRIVER_CONFIGS=(
    ["powerflex"]="csi-vxflexos:storage_csm_powerflex:7:driver:mdm-container:cr_powerflex_observability_custom_cert_missing_key cr_powerflex_observability_custom_cert cr_powerflex_observability cr_powerflex_replica cr_powerflex_resiliency:cr_powerflex_observability_custom_cert_missing_key cr_powerflex_observability_custom_cert cr_powerflex_observability:storage_csm_powerflex_auth_n_minus_1 storage_csm_powerflex_downgrade"
    ["powermax"]="csi-powermax:storage_csm_powermax:3:driver::cr_powermax_observability_use_secret cr_powermax_observability cr_powermax_replica cr_powermax_resiliency cr_powermax_reverseproxy_sidecar cr_powermax_reverseproxy_use_secret cr_powermax_reverseproxy::"
    ["powerscale"]="csi-isilon:storage_csm_powerscale:2:driver::cr_powerscale_auth_missing_skip_cert_env cr_powerscale_auth_validate_cert cr_powerscale_auth cr_powerscale_observability cr_powerscale_replica cr_powerscale_resiliency cr_powerscale_auth_driver_secret:cr_powerscale_auth_missing_skip_cert_env cr_powerscale_auth_validate_cert cr_powerscale_auth cr_powerscale_observability cr_powerscale_replica cr_powerscale_resiliency:storage_csm_powerscale_observability_val1"
    ["powerstore"]="csi-powerstore:storage_csm_powerstore:5:driver::cr_powerstore_resiliency cr_powerstore_auth cr_powerstore_observability cr_powerstore_replica:cr_powerstore_resiliency::"
    ["unity"]="csi-unity:storage_csm_unity:6:driver::::"
    ["cosi"]="cosi:storage_csm_cosi:8:objectstorage-provisioner::::"
)

# For Updating Version in bundle/manifests/dell-csm-operator.clusterserviceversion.yaml
UpdateVersion() {
    local driverImageName=$1
    local update_version=$2
    local input_file="bundle/manifests/dell-csm-operator.clusterserviceversion.yaml"
    
    # Update version for regular CSI drivers
    yq eval -i "(.spec.install.spec.deployments[].spec.template.spec.containers[] | select(.name == \"driver\") | .env[] | select(.name == \"RELATED_IMAGE_$driverImageName\").value) = \"quay.io/dell/container-storage-modules/$driverImageName:$update_version\"" "$input_file"
    
    # For COSI drivers, update the version in the CSI driver spec
    if [[ "$driverImageName" == "cosi" ]]; then
        yq eval -i "(.spec.customresourcedefinitions.owned[] | select(.name == \"containerstoragemodules.storage.dell.com\").spec.versions[].schema.openAPIV3Schema.properties.spec.properties.driver.properties.csiDriverSpec.properties.csiDriverType.enum[] | select(. == \"cosi\") | $(.)) | load(\"$input_file\" | .spec.customresourcedefinitions.owned[] | select(.name == \"containerstoragemodules.storage.dell.com\").spec.versions[].schema.openAPIV3Schema.properties.spec.properties.driver.properties.csiDriverSpec.properties.version) = \"$update_version\"" "$input_file"
    else
        # For regular CSI drivers, update the version in the deployment annotations
        yq eval -i "(.spec.install.spec.deployments[].spec.template.metadata.annotations.\"storage.dell.com/driver-$driverImageName-version\") = \"$update_version\"" "$input_file"
    fi
}

# For Updating Related Images in bundle/manifests/dell-csm-operator.clusterserviceversion.yaml
UpdateRelatedImages() {
    local driverImageName=$1
    local update_version=$2
    local previous_major_driver_version=$3
    local input_file="bundle/manifests/dell-csm-operator.clusterserviceversion.yaml"
    
    # Extract driver name from image name (e.g., "csi-powerstore" -> "powerstore")
    local driver_name="${driverImageName#csi-}"
    
    # Check if the driver sample files have spec.driver.image before updating related images
    local sample_file="config/samples/storage_v1_csm_${driver_name}.yaml"
    if [ -f "$sample_file" ]; then
        local driver_image_check=$(yq eval '.spec.driver.image' "$sample_file" 2>/dev/null)
        if [[ "$driver_image_check" == "null" || -z "$driver_image_check" ]]; then
            echo "⚠️  No spec.driver.image found in $sample_file, skipping related image update for $driver_name"
            return
        fi
    else
        echo "⚠️  Sample file $sample_file not found, skipping related image update for $driver_name"
        return
    fi
    
    # Check if the related image entry exists for this driver before updating
    local existing_image=$(yq eval "(.spec.relatedImages[] | select(.name == \"$driverImageName\").image)" "$input_file" 2>/dev/null)
    if [[ "$existing_image" != "null" && -n "$existing_image" ]]; then
        # Update related images in the CSV using yq only if entry exists
        yq eval -i "(.spec.relatedImages[] | select(.name == \"$driverImageName\").image) = \"quay.io/dell/container-storage-modules/$driverImageName:$update_version\"" "$input_file"
    else
        echo "⚠️  No related image entry found for $driverImageName in bundle CSV, skipping update"
    fi
}

# For Updating Related Images in config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml
UpdateBaseRelatedImages() {
    local driverImageName=$1
    local update_version=$2
    local previous_major_driver_version=$3
    local input_file="config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml"
    
    # Extract driver name from image name (e.g., "csi-powerstore" -> "powerstore")
    local driver_name="${driverImageName#csi-}"
    
    # Check if the driver sample files have spec.driver.image before updating related images
    local sample_file="config/samples/storage_v1_csm_${driver_name}.yaml"
    if [ -f "$sample_file" ]; then
        local driver_image_check=$(yq eval '.spec.driver.image' "$sample_file" 2>/dev/null)
        if [[ "$driver_image_check" == "null" || -z "$driver_image_check" ]]; then
            echo "⚠️  No spec.driver.image found in $sample_file, skipping related image update for $driver_name"
            return
        fi
    else
        echo "⚠️  Sample file $sample_file not found, skipping related image update for $driver_name"
        return
    fi
    
    # Check if the related image entry exists for this driver before updating
    local existing_image=$(yq eval "(.spec.relatedImages[] | select(.name == \"$driverImageName\").image)" "$input_file" 2>/dev/null)
    if [[ "$existing_image" != "null" && -n "$existing_image" ]]; then
        # Update related images in the base CSV using yq only if entry exists
        yq eval -i "(.spec.relatedImages[] | select(.name == \"$driverImageName\").image) = \"quay.io/dell/container-storage-modules/$driverImageName:$update_version\"" "$input_file"
    else
        echo "⚠️  No related image entry found for $driverImageName in base CSV, skipping update"
    fi
}

# For creating the latest driver sample file in samples folder
CreateLatestSampleFile() {
    local prefix=$1                     # e.g. "storage_csm_powerflex"
    local driver_sample_file_suffix=$2 # e.g. "2160"

    local latest_file=""
    local latest_version=""

    if [[ "$prefix" == "storage_csm_cosi" ]]; then
        local folder=samples/cosi/v*/
    else
        local folder=samples/v*/
    fi

    # Search for files inside versioned folders: samples/v*/[prefix]_v*.yaml
    for file in $(find $folder -type f -name "${prefix}_v*.yaml"); do
        local version_part=$(basename "$file" | grep -oE '[0-9]+')
        echo "Version_part: $version_part"
        if [[ $version_part -gt ${latest_version:-0} ]]; then
            latest_version=$version_part
            latest_file=$file
        fi
    done

    if [[ -z "$latest_file" ]]; then
        echo "❌ No latest sample file found in samples/v* for $prefix"
        exit 1
    fi

    # Extract major, minor from suffix: e.g. 2160 -> 2.16.0
    ExtractVersionFromSuffix "$driver_sample_file_suffix"

    if [[ "$prefix" == "storage_csm_cosi" ]]; then
        local versioned_folder="samples/cosi/v$major.$minor.0"
    else
        local versioned_folder="samples/v$major.$minor.0"
    fi
    mkdir -p "$versioned_folder"

    cp -v --update "$latest_file" "$versioned_folder/${prefix}_v${driver_sample_file_suffix}.yaml"
}

# Get minUpgradePath
GetMinUpgradePath() {
    local prefix=$1
    local search_paths
    if [[ "$prefix" == "storage_csm_cosi" ]]; then
        search_paths="samples/cosi/v*/"
    else
        search_paths="samples/v*/"
    fi
    local files=$(find $search_paths -type f -name "${prefix}_v*.yaml")

    if [ -z "$files" ]; then
        echo "0.0.0"
    else
        local oldest_file=$(echo "$files" | sort -V | head -1)
        local version_suffix=$(basename "$oldest_file" | grep -oE '_v[0-9]+' | grep -oE '[0-9]+')

        if [ -z "$version_suffix" ]; then
            echo "0.0.0"
        else
            ExtractVersionFromSuffix "$version_suffix"
            local min_upgrade_path="${major}.${minor}.${patch}"
            echo "$min_upgrade_path"
        fi
    fi
}

# Get latest(n-1) driver version where n is the version we are adding the support for in this release
GetLatestDriverVersion() {
    local prefix=$1
    local search_paths
    if [[ "$prefix" == "storage_csm_cosi" ]]; then
        search_paths="samples/cosi/v*/"
    else
        search_paths="samples/v*/"
    fi

    local files=$(find $search_paths -type f -name "${prefix}_v*.yaml")
    if [ -z "$files" ]; then
        echo "0.0.0"
        return
    fi

    local latest_file=$(echo "$files" | sort -V | tail -1)
    local version_suffix=$(basename "$latest_file" | sed -E "s/^${prefix}_v([0-9]+)\.yaml$/\1/")

    # Extract digits from version suffix safely (e.g., 2160 -> 2.16.0)
    ExtractVersionFromSuffix "$version_suffix"

    local latest_driver_version="${major}.${minor}.${patch}"
    echo "$latest_driver_version"
} 

GetSecondLatestDriverVersion() {
    local prefix=$1
    local files=$(find samples/v*/ -type f -name "${prefix}_v*.yaml")
    if [ -z "$files" ]; then
        echo "0.0.0"
        return
    fi

    # Extract semantic versions from filenames like v2140 → 2.14.0
    local versions=$(echo "$files" | sed -E "s|.*/${prefix}_v([0-9]{1})([0-9]{2})([0-9]{1})\.yaml|\1.\2.\3|" | sort -V)

    # Get unique minor versions (e.g., 2.14, 2.15)
    local minor_versions=$(echo "$versions" | awk -F. '{print $1"."$2}' | sort -V | uniq)

    # Get the second latest minor version
    local prev_minor=$(echo "$minor_versions" | tail -2 | head -1)

    # Filter versions matching that minor version and get highest patch
    local highest_patch=$(echo "$versions" | grep "^${prev_minor}\." | sort -V | tail -1)

    echo "$highest_patch"
}

# Get n-1 and n-2 versions from operatorconfig/driverconfig directory
GetDriverVersionsFromConfig() {
    local driver_name=$1
    local current_version=$2
    
    local config_dir="operatorconfig/driverconfig/$driver_name"
    if [ ! -d "$config_dir" ]; then
        echo "0.0.0 0.0.0"
        return
    fi
    
    # Get all version directories and sort them
    local all_versions=$(ls -d "$config_dir"/v* 2>/dev/null | sed 's|.*/||' | sort -V)
    
    if [ -z "$all_versions" ]; then
        echo "0.0.0 0.0.0"
        return
    fi
    
    # Parse current version to get major.minor
    local current_major=${current_version%%.*}
    local current_minor_tmp=${current_version#*.}
    local current_minor=${current_minor_tmp%%.*}
    
    # Group versions by minor version and get latest patch for each
    declare -A latest_patches
    while IFS= read -r version; do
        local version_clean=${version#v}
        local major=${version_clean%%.*}
        local minor_tmp=${version_clean#*.}
        local minor=${minor_tmp%%.*}
        local patch=${minor_tmp##*.}
        
        local minor_key="${major}.${minor}"
        
        # Keep only the latest patch for each minor version
        if [[ -z "${latest_patches[$minor_key]}" ]] || [[ "$version_clean" > "${latest_patches[$minor_key]}" ]]; then
            latest_patches[$minor_key]="$version_clean"
        fi
    done <<< "$all_versions"
    
    # Get sorted list of minor versions
    local minor_versions=$(printf '%s\n' "${!latest_patches[@]}" | sort -V)
    
    # Find current minor version in the list
    local current_minor_key="${current_major}.${current_minor}"
    local minor_array=($minor_versions)
    local current_index=-1
    
    for i in "${!minor_array[@]}"; do
        if [[ "${minor_array[$i]}" == "$current_minor_key" ]]; then
            current_index=$i
            break
        fi
    done
    
    local n_minus_1="0.0.0"
    local n_minus_2="0.0.0"
    
    # Get n-1 version (latest patch from previous minor version)
    if [[ $current_index -gt 0 ]]; then
        local prev_minor_key="${minor_array[$((current_index - 1))]}"
        n_minus_1="${latest_patches[$prev_minor_key]}"
    fi
    
    # Get n-2 version (latest patch from two minor versions back)
    if [[ $current_index -gt 1 ]]; then
        local prev_prev_minor_key="${minor_array[$((current_index - 2))]}"
        n_minus_2="${latest_patches[$prev_prev_minor_key]}"
    fi
    
    echo "$n_minus_1 $n_minus_2"
}

# Helper function to get just n-1 version
GetNMinusOneVersion() {
    local driver_name=$1
    local current_version=$2
    local versions=$(GetDriverVersionsFromConfig "$driver_name" "$current_version")
    echo "$versions" | awk '{print $1}'
}

# Helper function to get just n-2 version  
GetNMinusTwoVersion() {
    local driver_name=$1
    local current_version=$2
    local versions=$(GetDriverVersionsFromConfig "$driver_name" "$current_version")
    echo "$versions" | awk '{print $2}'
}

# For creating the latest minimal driver sample file in samples folder
CreateLatestMinimalSampleFile() {
    local prefix=$1
    local driver_sample_file_suffix=$2
    local destination_folder=$3  # e.g. samples/v2.16.0/minimal-samples

    # Get list of all minimal-samples folders
    local search_paths
    if [[ "$prefix" == "cosi" ]]; then
        search_paths="samples/cosi/v*/minimal-samples"
    else
        search_paths="samples/v*/minimal-samples"
    fi

    local all_folders=$(ls -d $search_paths 2>/dev/null | grep -vF "$destination_folder" | sort -Vr)

    if [ -z "$all_folders" ]; then
        echo "❌ No other minimal-sample folders found to copy from"
        exit 1
    fi

    local latest_folder=$(echo "$all_folders" | head -1)
    local latest_file=$(find "$latest_folder" -type f -name "${prefix}_v*.yaml" | sort -V | tail -1)

    if [ ! -f "$latest_file" ]; then
        echo "❌ No latest minimal sample found in $latest_folder for $prefix"
        exit 1
    fi

    mkdir -p "$destination_folder"
    cp -v --update "$latest_file" "$destination_folder/${prefix}_v${driver_sample_file_suffix}.yaml"
}

# Function to delete a file or directory if it exists
DeleteIfExists() {
    local path=$1
    if [ -f "$path" ]; then
        rm -f "$path"
    elif [ -d "$path" ]; then
        rm -rf "$path"
    fi
}

ExtractVersionFromSuffix() {
    local version_suffix=$1
    major=$((10#$(echo "$version_suffix" | cut -c1)))
    if [[ ${#version_suffix} -eq 4 ]]; then
        minor=$((10#$(echo "$version_suffix" | cut -c2-3)))
        patch=$((10#$(echo "$version_suffix" | cut -c4)))
    elif [[ ${#version_suffix} -eq 3 ]]; then
        minor=$((10#$(echo "$version_suffix" | cut -c2)))
        patch=$((10#$(echo "$version_suffix" | cut -c3)))
    else
        echo "Unexpected version suffix length: ${#version_suffix}" >&2
        exit 1
    fi
}

# Helper function to check if version field exists in YAML file
VersionFieldExists() {
    local file_path=$1
    local version_check=$(yq eval '.spec.version' "$file_path" 2>/dev/null)
    if [[ "$version_check" == "null" || -z "$version_check" ]]; then
        return 1  # version field doesn't exist
    else
        return 0  # version field exists
    fi
}

# Generic driver update function
UpdateDriverVersion() {
    local driver_name=$1
    local driver_version_update=$2
    local update_type=$3  # "major" or "patch"
    local csm_version_override=$4  # CSM version to use for .spec.version
    
    # Get n-1 and n-2 versions for proper version determination
    local version_info=$(GetDriverVersionsFromConfig "$driver_name" "$driver_version_update")
    local n_minus_1_version=$(echo "$version_info" | awk '{print $1}')
    local n_minus_2_version=$(echo "$version_info" | awk '{print $2}')
    
    echo "📋 Driver: $driver_name, Current: $driver_version_update, n-1: $n_minus_1_version, n-2: $n_minus_2_version"
    
    # Parse driver configuration
    local config_string="${DRIVER_CONFIGS[$driver_name]}"
    IFS=':' read -ra CONFIG_ARRAY <<< "$config_string"
    local image_name="${CONFIG_ARRAY[0]}"
    local sample_prefix="${CONFIG_ARRAY[1]}"
    local manager_env_index="${CONFIG_ARRAY[2]}"
    local driver_container_name="${CONFIG_ARRAY[3]}"
    local init_container_name="${CONFIG_ARRAY[4]}"
    local testdata_all="${CONFIG_ARRAY[5]}"
    local testdata_image="${CONFIG_ARRAY[6]}"
    local n_minus_1_tests="${CONFIG_ARRAY[7]}"
    
    # Extract version components
    local major_version=${driver_version_update%%.*}
    local minor_version_tmp=${driver_version_update#*.}
    local minor_version=${minor_version_tmp%%.*}
    local patch_version=${driver_version_update##*.}
    
    local driver_sample_file_suffix=$(echo "$driver_version_update" | tr -d '.' | tr -d '\n')
    
    # Determine if we should update version field
    local update_version_field=false
    local update_version
    local config_sample_file="config/samples/storage_v1_csm_${driver_name}.yaml"
    
    # Check if version field exists in the config sample file
    if [ -f "$config_sample_file" ] && VersionFieldExists "$config_sample_file"; then
        # Version field exists, update it with csm_version_override if provided, otherwise use driver version
        update_version_field=true
        if [ -n "$csm_version_override" ]; then
            update_version="$csm_version_override"
        else
            update_version="v$driver_version_update"
        fi
    else
        # Version field doesn't exist, don't update version field
        update_version_field=false
    fi
    
    # Determine image version
    local new_image_version="quay.io/dell/container-storage-modules/$image_name:v$driver_version_update"
    
    # Determine sample folder path
    local sample_version_folder
    if [[ "$driver_name" == "cosi" ]]; then
        sample_version_folder="samples/cosi/v$major_version.$minor_version.0"
    else
        sample_version_folder="samples/v$major_version.$minor_version.0"
    fi
    mkdir -p "$sample_version_folder/minimal-samples"
    
    if [ "$update_type" == "major" ]; then
        # Major update logic - use n-1 version as the previous major version
        local previous_major_driver_version="$n_minus_1_version"
        
        # Create sample and minimal sample
        CreateLatestSampleFile "$sample_prefix" "$driver_sample_file_suffix"
        CreateLatestMinimalSampleFile "$driver_name" "$driver_sample_file_suffix" "$sample_version_folder/minimal-samples"
        
        # Update samples
        if [ "$update_version_field" = true ]; then
            # Check each file before updating
            local sample_file="$sample_version_folder/${sample_prefix}_v${driver_sample_file_suffix}.yaml"
            if VersionFieldExists "$sample_file"; then
                yq -i '.spec.version = "'"$update_version"'"' "$sample_file"
            fi
            
            local minimal_sample_file="$sample_version_folder/minimal-samples/${driver_name}_v${driver_sample_file_suffix}.yaml"
            if VersionFieldExists "$minimal_sample_file"; then
                yq -i '.spec.version = "'"$update_version"'"' "$minimal_sample_file"
            fi
        fi
        yq -i '.spec.driver.common.image = "'"$new_image_version"'"' "$sample_version_folder/${sample_prefix}_v${driver_sample_file_suffix}.yaml"
        yq -i '.spec.driver.common.image = "'"$new_image_version"'"' "$sample_version_folder/minimal-samples/${driver_name}_v${driver_sample_file_suffix}.yaml"
        
        # Copy to config samples
        cp -v --update "$sample_version_folder/${sample_prefix}_v${driver_sample_file_suffix}.yaml" "config/samples/storage_v1_csm_${driver_name}.yaml"
        
        # Operator config updates
        cp -a --update "operatorconfig/driverconfig/$driver_name/v$previous_major_driver_version/." "operatorconfig/driverconfig/$driver_name/v$driver_version_update"
        
        # Update container images in operator configs
        if [[ "$driver_name" == "cosi" ]]; then
            yq eval -i 'with(select(.spec.template.spec.containers[0].name == "'"$driver_container_name"'"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' "operatorconfig/driverconfig/$driver_name/v$driver_version_update/controller.yaml"
        else
            yq eval -i 'with(select(.spec.template.spec.containers[5].name == "'"$driver_container_name"'"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' "operatorconfig/driverconfig/$driver_name/v$driver_version_update/controller.yaml"
            yq eval -i 'with(select(.spec.template.spec.containers[0].name == "'"$driver_container_name"'"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' "operatorconfig/driverconfig/$driver_name/v$driver_version_update/node.yaml"
            
            # Update init container if specified
            if [[ -n "$init_container_name" ]]; then
                yq eval -i 'with(select(.spec.template.spec.initContainers[0].name == "'"$init_container_name"'"); .spec.template.spec.initContainers[0].image = "'"$new_image_version"'")' "operatorconfig/driverconfig/$driver_name/v$driver_version_update/node.yaml"
            fi
        fi
        
        # Delete N-3 version folder
        local delete_minor_version=$((minor_version - 3))
        local driver_delete_version="$major_version.$delete_minor_version.0"
        DeleteIfExists "samples/v$driver_delete_version"
        DeleteIfExists "operatorconfig/driverconfig/$driver_name/v$driver_delete_version"
        
    else
        # Patch update logic
        local previous_patch_version=$((patch_version - 1))
        local previous_patch_driver_version="$major_version.$minor_version.$previous_patch_version"
        local previous_driver_sample_file_suffix=$(echo "$previous_patch_driver_version" | tr -d '.' | tr -d '\n')
        
        # Copy previous patch files
        cp -v --update "$sample_version_folder/${sample_prefix}_v${previous_driver_sample_file_suffix}.yaml" \
              "$sample_version_folder/${sample_prefix}_v${driver_sample_file_suffix}.yaml"
        cp -v --update "$sample_version_folder/minimal-samples/${driver_name}_v${previous_driver_sample_file_suffix}.yaml" \
              "$sample_version_folder/minimal-samples/${driver_name}_v${driver_sample_file_suffix}.yaml"
        
        # Update copied files
        if [ "$update_version_field" = true ]; then
            # Check each file before updating
            local sample_file="$sample_version_folder/${sample_prefix}_v${driver_sample_file_suffix}.yaml"
            if VersionFieldExists "$sample_file"; then
                yq -i '.spec.version = "'"$update_version"'"' "$sample_file"
            fi
            
            local minimal_sample_file="$sample_version_folder/minimal-samples/${driver_name}_v${driver_sample_file_suffix}.yaml"
            if VersionFieldExists "$minimal_sample_file"; then
                yq -i '.spec.version = "'"$update_version"'"' "$minimal_sample_file"
            fi
        fi
        yq -i '.spec.driver.common.image = "'"$new_image_version"'"' "$sample_version_folder/${sample_prefix}_v${driver_sample_file_suffix}.yaml"
        yq -i '.spec.driver.common.image = "'"$new_image_version"'"' "$sample_version_folder/minimal-samples/${driver_name}_v${driver_sample_file_suffix}.yaml"
        
        # Operator config update
        cp -a --update "operatorconfig/driverconfig/$driver_name/v$previous_patch_driver_version" \
              "operatorconfig/driverconfig/$driver_name/v$driver_version_update"
        DeleteIfExists "operatorconfig/driverconfig/$driver_name/v$previous_patch_driver_version"
        
        # Update container images
        if [[ "$driver_name" == "cosi" ]]; then
            yq eval -i 'with(select(.spec.template.spec.containers[0].name == "'"$driver_container_name"'"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' "operatorconfig/driverconfig/$driver_name/v$driver_version_update/controller.yaml"
        else
            yq eval -i 'with(select(.spec.template.spec.containers[5].name == "'"$driver_container_name"'"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' "operatorconfig/driverconfig/$driver_name/v$driver_version_update/controller.yaml"
            yq eval -i 'with(select(.spec.template.spec.containers[0].name == "'"$driver_container_name"'"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' "operatorconfig/driverconfig/$driver_name/v$driver_version_update/node.yaml"
            
            if [[ -n "$init_container_name" ]]; then
                yq eval -i 'with(select(.spec.template.spec.initContainers[0].name == "'"$init_container_name"'"); .spec.template.spec.initContainers[0].image = "'"$new_image_version"'")' "operatorconfig/driverconfig/$driver_name/v$driver_version_update/node.yaml"
            fi
        fi
    fi
    
    # Update upgrade path
    local min_upgrade_path=$(GetMinUpgradePath "$sample_prefix")
    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' "operatorconfig/driverconfig/$driver_name/v$driver_version_update/upgrade-path.yaml"
    
    # CSV updates
    UpdateVersion "$image_name" "$update_version"
    if [ "$update_type" == "major" ]; then
        local previous_major_driver_version=$(GetLatestDriverVersion "$sample_prefix")
        UpdateRelatedImages "$image_name" "$update_version" "$previous_major_driver_version"
        UpdateBaseRelatedImages "$image_name" "$update_version" "$previous_major_driver_version"
    else
        UpdateRelatedImages "$image_name" "$update_version"
        UpdateBaseRelatedImages "$image_name" "$update_version"
    fi
    
    # Update test data files
    if [[ -n "$testdata_all" ]]; then
        for i in $testdata_all; do
            if [ "$update_version_field" = true ]; then
                local testdata_file="pkg/modules/testdata/$i.yaml"
                if VersionFieldExists "$testdata_file"; then
                    yq -i '.spec.version = "'"$update_version"'"' "$testdata_file"
                fi
            fi
        done
    fi
    
    if [[ -n "$testdata_image" ]]; then
        for i in $testdata_image; do
            yq -i '.spec.driver.common.image = "'"$new_image_version"'"' "pkg/modules/testdata/$i.yaml"
        done
    fi
    
    # Test config updates
    if [ "$update_type" == "major" ]; then
        local previous_major_driver_version=$(GetLatestDriverVersion "$sample_prefix")
        cp -a --update "tests/config/driverconfig/$driver_name/v$previous_major_driver_version/." "tests/config/driverconfig/$driver_name/v$driver_version_update"
        local delete_minor_version=$((minor_version - 3))
        local driver_delete_version="$major_version.$delete_minor_version.0"
        DeleteIfExists "tests/config/driverconfig/$driver_name/v$driver_delete_version"
    else
        local previous_patch_version=$((patch_version - 1))
        local previous_patch_driver_version="$major_version.$minor_version.$previous_patch_version"
        cp -a --update "tests/config/driverconfig/$driver_name/v$previous_patch_driver_version" \
              "tests/config/driverconfig/$driver_name/v$driver_version_update"
        DeleteIfExists "tests/config/driverconfig/$driver_name/v$previous_patch_driver_version"
    fi
    
    # Update test config container images
    if [[ "$driver_name" == "cosi" ]]; then
        yq eval -i 'with(select(.spec.template.spec.containers[0].name == "'"$driver_container_name"'"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' "tests/config/driverconfig/$driver_name/v$driver_version_update/controller.yaml"
    else
        yq eval -i 'with(select(.spec.template.spec.containers[5].name == "'"$driver_container_name"'"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' "tests/config/driverconfig/$driver_name/v$driver_version_update/controller.yaml"
        yq eval -i 'with(select(.spec.template.spec.containers[0].name == "'"$driver_container_name"'"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' "tests/config/driverconfig/$driver_name/v$driver_version_update/node.yaml"
        
        if [[ -n "$init_container_name" ]]; then
            yq eval -i 'with(select(.spec.template.spec.initContainers[0].name == "'"$init_container_name"'"); .spec.template.spec.initContainers[0].image = "'"$new_image_version"'")' "tests/config/driverconfig/$driver_name/v$driver_version_update/node.yaml"
        fi
    fi
    
    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' "tests/config/driverconfig/$driver_name/v$driver_version_update/upgrade-path.yaml"
    
    # Update e2e testfiles
    if [ "$update_version_field" = true ]; then
        for f in $(find tests/e2e/testfiles -type f -name "${sample_prefix}*"); do
            if VersionFieldExists "$f"; then
                yq -i '.spec.version = "'"$update_version"'"' "$f"
            fi
        done
        for f in $(find tests/e2e/testfiles/minimal-testfiles -type f -name "${sample_prefix}*"); do
            if VersionFieldExists "$f"; then
                yq -i '.spec.version = "'"$update_version"'"' "$f"
            fi
        done
    fi
    
    # Update n-1 test files if specified
    if [[ -n "$n_minus_1_tests" ]]; then
        local second_previous_driver_version=$(GetSecondLatestDriverVersion "$sample_prefix")
        local second_previous_driver_config_version="v$second_previous_driver_version"
        local second_previous_driver_image_version="quay.io/dell/container-storage-modules/$image_name:v$second_previous_driver_version"
        
        for f in $n_minus_1_tests; do
            if [ "$update_version_field" = true ]; then
                local n_minus_1_file="tests/e2e/testfiles/$f.yaml"
                if VersionFieldExists "$n_minus_1_file"; then
                    yq -i '.spec.version = "'"$second_previous_driver_version"'"' "$n_minus_1_file"
                fi
            fi
            yq -i '.spec.driver.common.image = "'"$second_previous_driver_image_version"'"' "tests/e2e/testfiles/$f.yaml"
        done
    fi
    
    # Update manager environment
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env['"$manager_env_index"'].value = "'"$new_image_version"'")' config/manager/manager.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env['"$manager_env_index"'].value = "'"$new_image_version"'")' deploy/operator.yaml
}

# BadDriver update function
UpdateBadDriver() {
    local driver_version=$1
    
    if [ -z "$driver_version" -o "$driver_version" == " " ]; then
        return
    fi
    
    major_version=${driver_version%%.*}
    minor_version_tmp=${driver_version#*.}
    minor_version=${minor_version_tmp%%.*}
    patch_version=${driver_version##*.}
    previous_minor_version=$((minor_version - 1))
    previous_major_driver_version="$major_version.$previous_minor_version.$patch_version"
    cp -a --update "tests/config/driverconfig/badDriver/v$previous_major_driver_version/." "tests/config/driverconfig/badDriver/v$driver_version"
    delete_minor_version=$((minor_version - 3))
    driver_delete_version="$major_version.$delete_minor_version.$patch_version"
    DeleteIfExists "tests/config/driverconfig/badDriver/v$driver_delete_version"
}

#----------------------------Entry Point------------------------------------------

if [ "$driver_update_type" == "major" ]; then
    if [ ! -z "$powerflex_version" -a "$powerflex_version" != " " ]; then
        UpdateDriverVersion "powerflex" "$powerflex_version" "major" "$csm_version"
        UpdateBadDriver "$powerflex_version"
    fi
    if [ ! -z "$powermax_version" -a "$powermax_version" != " " ]; then
        UpdateDriverVersion "powermax" "$powermax_version" "major" "$csm_version"
        UpdateBadDriver "$powermax_version"
    fi
    if [ ! -z "$powerscale_version" -a "$powerscale_version" != " " ]; then
        UpdateDriverVersion "powerscale" "$powerscale_version" "major" "$csm_version"
        UpdateBadDriver "$powerscale_version"
    fi
    if [ ! -z "$powerstore_version" -a "$powerstore_version" != " " ]; then
        UpdateDriverVersion "powerstore" "$powerstore_version" "major" "$csm_version"
        UpdateBadDriver "$powerstore_version"
    fi
    if [ ! -z "$unity_version" -a "$unity_version" != " " ]; then
        UpdateDriverVersion "unity" "$unity_version" "major" "$csm_version"
        UpdateBadDriver "$unity_version"
    fi
    if [ ! -z "$cosi_version" -a "$cosi_version" != " " ]; then
        UpdateDriverVersion "cosi" "$cosi_version" "major" "$csm_version"
        UpdateBadDriver "$cosi_version"
    fi
elif [ "$driver_update_type" == "patch" ]; then
    [ ! -z "$powerflex_version" ] && UpdateDriverVersion "powerflex" "$powerflex_version" "patch" "$csm_version"
    [ ! -z "$powermax_version" ] && UpdateDriverVersion "powermax" "$powermax_version" "patch" "$csm_version"
    [ ! -z "$powerscale_version" ] && UpdateDriverVersion "powerscale" "$powerscale_version" "patch" "$csm_version"
    [ ! -z "$powerstore_version" ] && UpdateDriverVersion "powerstore" "$powerstore_version" "patch" "$csm_version"
    [ ! -z "$unity_version" ] && UpdateDriverVersion "unity" "$unity_version" "patch" "$csm_version"
    [ ! -z "$cosi_version" ] && UpdateDriverVersion "cosi" "$cosi_version" "patch" "$csm_version"
else
    echo "invalid driver_update_type"
    exit 1
fi
