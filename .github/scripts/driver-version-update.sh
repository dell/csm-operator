#!/bin/bash

# Usage for major nightly update: bash ./.github/scripts/driver-version-update.sh --driver_update_type "major" --release_type "nightly" --powerscale_version "2.15.0" --powermax_version "2.15.0" --powerflex_version "2.15.0" --powerstore_version "2.15.0" --unity_version "2.15.0"
# Usage for major tag update: bash ./.github/scripts/driver-version-update.sh --driver_update_type "major" --release_type "tag" --powerscale_version "2.15.0" --powermax_version "2.15.0" --powerflex_version "2.15.0" --powerstore_version "2.15.0" --unity_version "2.15.0"
# Usage for patch update: bash ./.github/scripts/driver-version-update.sh --driver_update_type "patch" --release_type "nightly" --powerscale_version "2.14.1" --powermax_version "2.14.1" --powerflex_version "2.14.1" --powerstore_version "2.14.1" --unity_version "2.14.1"

# Initialize variables with default values
driver_update_type=""
release_type=""
powerscale_version=""
powermax_version=""
powerflex_version=""
powerstore_version=""
unity_version=""

# Set options for the getopt command
options=$(getopt -o "" -l "driver_update_type:,release_type:,powerscale_version:,powermax_version:,powerflex_version:,powerstore_version:,unity_version:" -- "$@")
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
    --release_type)
        release_type="$2"
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
    --) shift ;;
    esac
    shift
done

# For Updating Config Version in bundle/manifests/dell-csm-operator.clusterserviceversion.yaml
UpdateConfigVersion() {
    driverImageName=$1
    update_config_version=$2
    input_file="bundle/manifests/dell-csm-operator.clusterserviceversion.yaml"
    search_string1="quay.io/dell/container-storage-modules/$driverImageName:v"
    nightly_search_string1="quay.io/dell/container-storage-modules/$driverImageName:nightly"
    search_string2="imagePullPolicy"
    line_number=0
    while IFS= read -r line; do
        line_number=$((line_number + 1))
        if [[ "$line" == *"$search_string1"* ]]; then
            IFS= read -r next_line
            if [[ "$next_line" == *"$search_string2"* ]]; then
                line_number=$((line_number + 3))
                sed -i "$line_number s/.*/              \"configVersion\": \""$update_config_version"\",/" "$input_file"
                break
            fi
        fi
    done <"$input_file"
    while IFS= read -r line; do
        line_number=$((line_number + 1))
        if [[ "$line" == *"$nightly_search_string1"* ]]; then
            IFS= read -r next_line
            if [[ "$next_line" == *"$search_string2"* ]]; then
                line_number=$((line_number + 3))
                sed -i "$line_number s/.*/              \"configVersion\": \""$update_config_version"\",/" "$input_file"
                break
            fi
        fi
    done <"$input_file"
}

# For Updating Related Images in bundle/manifests/dell-csm-operator.clusterserviceversion.yaml
UpdateRelatedImages() {
    driverImageName=$1
    update_version=$2
    input_file="bundle/manifests/dell-csm-operator.clusterserviceversion.yaml"
    nightly_search_string_1=" - image: quay.io/dell/container-storage-modules/$driverImageName:nightly"
    nightly_search_string_2="                  value: quay.io/dell/container-storage-modules/$driverImageName:nightly"
    nightly_search_string_3="                \"image\": \"quay.io/dell/container-storage-modules/$driverImageName:nightly"
    new_line_1="   - image: quay.io/dell/container-storage-modules/$driverImageName:$update_version"
    new_line_2="                       value: quay.io/dell/container-storage-modules/$driverImageName:$update_version"
    new_line_3="               \"image\": \"quay.io/dell/container-storage-modules/$driverImageName:$update_version\","
    line_number=0
    while IFS= read -r line; do
        line_number=$((line_number + 1))
        if [[ "$line" == *"$nightly_search_string_1"* ]]; then
            sed -i "$line_number c\ $new_line_1" "$input_file"
        fi
        if [[ "$line" == *"$nightly_search_string_2"* ]]; then
            sed -i "$line_number c\ $new_line_2" "$input_file"
        fi
        if [[ "$line" == *"$nightly_search_string_3"* ]]; then
            sed -i "$line_number c\ $new_line_3" "$input_file"
        fi
    done <"$input_file"
}

# For Updating nightly Related Images in bundle/manifests/dell-csm-operator.clusterserviceversion.yaml
UpdateNightlyRelatedImages() {
    driverImageName=$1
    input_file="bundle/manifests/dell-csm-operator.clusterserviceversion.yaml"
    search_string_1=" - image: quay.io/dell/container-storage-modules/$driverImageName:v"
    search_string_2="                  value: quay.io/dell/container-storage-modules/$driverImageName:v"
    search_string_3="                \"image\": \"quay.io/dell/container-storage-modules/$driverImageName:v"
    new_line_1="   - image: quay.io/dell/container-storage-modules/$driverImageName:nightly"
    new_line_2="                       value: quay.io/dell/container-storage-modules/$driverImageName:nightly"
    new_line_3="               \"image\": \"quay.io/dell/container-storage-modules/$driverImageName:nightly\","
    line_number=0
    while IFS= read -r line; do
        line_number=$((line_number + 1))
        if [[ "$line" == *"$search_string_1"* ]]; then
            sed -i "$line_number c\ $new_line_1" "$input_file"
        fi
        if [[ "$line" == *"$search_string_2"* ]]; then
            sed -i "$line_number c\ $new_line_2" "$input_file"
        fi
        if [[ "$line" == *"$search_string_3"* ]]; then
            sed -i "$line_number c\ $new_line_3" "$input_file"
        fi
    done <"$input_file"
}

# For Updating Related Images in config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml
UpdateBaseRelatedImages() {
    driverImageName=$1
    update_version=$2
    input_file="config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml"
    nightly_search_string_1="  - image: quay.io/dell/container-storage-modules/$driverImageName:nightly"
    new_line_1="   - image: quay.io/dell/container-storage-modules/$driverImageName:$update_version"
    line_number=0
    while IFS= read -r line; do
        line_number=$((line_number + 1))
        if [[ "$line" == *"$nightly_search_string_1"* ]]; then
            sed -i "$line_number c\ $new_line_1" "$input_file"
        fi
    done <"$input_file"
}

# For Updating nightly Related Images in config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml
UpdateNightlyBaseRelatedImages() {
    driverImageName=$1
    input_file="config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml"
    search_string_1="  - image: quay.io/dell/container-storage-modules/$driverImageName:v"
    new_line_1="   - image: quay.io/dell/container-storage-modules/$driverImageName:nightly"
    line_number=0
    while IFS= read -r line; do
        line_number=$((line_number + 1))
        if [[ "$line" == *"$search_string_1"* ]]; then
            sed -i "$line_number c\ $new_line_1" "$input_file"
        fi
    done <"$input_file"
}
# For creating the latest driver sample file in samples folder
CreateLatestSampleFile() {
    prefix=$1
    driver_sample_file_suffix=$driver_sample_file_suffix
    files=($(ls "samples" | grep "^$prefix"))
    largest_numerical_value=0
    latest_file_name=""
    # Iterate over the files and find the file with the largest numerical value in its name
    for file in "${files[@]}"; do
        numerical_value=$(echo "$file" | grep -oE '[0-9]+' | tail -1)
        if [[ $numerical_value -gt $largest_numerical_value ]]; then
            largest_numerical_value=$numerical_value
            latest_file_name="$file"
        fi
    done
    cp -v samples/$latest_file_name samples/${prefix}_v${driver_sample_file_suffix}.yaml
}

# Get minUpgradePath
GetMinUpgradePath() {
    prefix=$1
    files=($(ls "samples" | grep "^$prefix"))
    smallest_numerical_value=100000000
    # Iterate over the files and find the smallest numerical value in its name
    for file in "${files[@]}"; do
        numerical_value=$(echo "$file" | grep -oE '[0-9]+' | tail -1)
        if [[ $numerical_value -lt $smallest_numerical_value ]]; then
            smallest_numerical_value=$numerical_value
        fi
    done
    min_upgrade_path="${smallest_numerical_value:0:1}.${smallest_numerical_value:1:2}.${smallest_numerical_value:3:1}"
    echo "$min_upgrade_path"
}

# Get latest(n-1) driver version where n is the version we are adding the support for in this release
GetLatestDriverVersion() {
    prefix=$1
    files=($(ls "samples" | grep "^$prefix"))
    largest_numerical_value=0
    # Iterate over the files and find the smallest numerical value in its name
    for file in "${files[@]}"; do
        numerical_value=$(echo "$file" | grep -oE '[0-9]+' | tail -1)
        if [[ $numerical_value -gt $largest_numerical_value ]]; then
            largest_numerical_value=$numerical_value
        fi
    done
    latest_driver_version="${largest_numerical_value:0:1}.${largest_numerical_value:1:2}.${largest_numerical_value:3:1}"
    echo "$latest_driver_version"
}

# For creating the latest minimal driver sample file in samples folder
CreateLatestMinimalSampleFile() {
    prefix=$1
    driver_sample_file_suffix=$driver_sample_file_suffix
    files=($(ls "samples/minimal-samples" | grep "^$prefix"))
    largest_numerical_value=0
    latest_file_name=""
    # Iterate over the files and find the file with the largest numerical value in its name
    for file in "${files[@]}"; do
        numerical_value=$(echo "$file" | grep -oE '[0-9]+' | tail -1)
        if [[ $numerical_value -gt $largest_numerical_value ]]; then
            largest_numerical_value=$numerical_value
            latest_file_name="$file"
        fi
    done
    cp -v samples/minimal-samples/$latest_file_name samples/minimal-samples/${prefix}_v${driver_sample_file_suffix}.yaml
}

# For Updating Powerflex Driver Major Version
UpdateMajorPowerflexDriver() {
    driver_version_update=$1
    release_type=$2
    # Extract the values of major_version, minor_version, and patch_version from the input string
    major_version=${driver_version_update%%.*}
    minor_version=${driver_version_update#*.}
    minor_version=${minor_version%%.*}
    patch_version=${driver_version_update##*.}

    previous_major_driver_version=$(GetLatestDriverVersion "storage_csm_powerflex")
    driver_sample_file_suffix=$(echo "$driver_version_update" | tr -d '.' | tr -d '\n')
    CreateLatestSampleFile "storage_csm_powerflex" $driver_sample_file_suffix
    CreateLatestMinimalSampleFile "powerflex" $driver_sample_file_suffix

    update_config_version="v$driver_version_update"

    # Replace the config version in the file
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' samples/storage_csm_powerflex_v$driver_sample_file_suffix.yaml
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' samples/minimal-samples/powerflex_v$driver_sample_file_suffix.yaml

    # Specify the new image versions
    if [ "$release_type" == "nightly" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-vxflexos:nightly"
    elif [ "$release_type" == "tag" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-vxflexos:v$driver_version_update"
    fi

    # Replace the image version in the file
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' samples/storage_csm_powerflex_v$driver_sample_file_suffix.yaml
    cp -v samples/storage_csm_powerflex_v$driver_sample_file_suffix.yaml config/samples/storage_v1_csm_powerflex.yaml

    cp -a operatorconfig/driverconfig/powerflex/v$previous_major_driver_version/. operatorconfig/driverconfig/powerflex/v$driver_version_update
    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' operatorconfig/driverconfig/powerflex/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' operatorconfig/driverconfig/powerflex/v$driver_version_update/node.yaml
    yq eval -i 'with(select(.spec.template.spec.initContainers[0].name == "mdm-container"); .spec.template.spec.initContainers[0].image = "'"$new_image_version"'")' operatorconfig/driverconfig/powerflex/v$driver_version_update/node.yaml

    delete_minor_version=$((minor_version - 3))
    driver_delete_version="$major_version.$delete_minor_version.$patch_version"
    driver_delete_version_sample_file_suffix=$(echo "$driver_delete_version" | tr -d '.' | tr -d '\n')
    rm samples/storage_csm_powerflex_v$driver_delete_version_sample_file_suffix.yaml
    rm samples/minimal-samples/powerflex_v$driver_delete_version_sample_file_suffix.yaml
    rm -r operatorconfig/driverconfig/powerflex/v$driver_delete_version

    min_upgrade_path=$(GetMinUpgradePath "storage_csm_powerflex")
    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' operatorconfig/driverconfig/powerflex/v$driver_version_update/upgrade-path.yaml

    # Update config version in bundle/manifests/dell-csm-operator.clusterserviceversion.yaml
    UpdateConfigVersion csi-vxflexos $update_config_version

    # Update driver version in bundle/manifests/dell-csm-operator.clusterserviceversion.yaml and config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml
    if [ "$release_type" == "nightly" ]; then
        UpdateNightlyRelatedImages csi-vxflexos
        UpdateNightlyBaseRelatedImages csi-vxflexos
    elif [ "$release_type" == "tag" ]; then
        UpdateRelatedImages csi-vxflexos $update_config_version
        UpdateBaseRelatedImages csi-vxflexos $update_config_version
    fi

    declare -a configArr=(
        "cr_powerflex_observability_custom_cert_missing_key"
        "cr_powerflex_observability_custom_cert"
        "cr_powerflex_observability"
        "cr_powerflex_replica"
        "cr_powerflex_resiliency"
    )
    for i in "${configArr[@]}"; do
        yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' pkg/modules/testdata/$i.yaml
    done

    declare -a imageArr=(
        "cr_powerflex_observability_custom_cert_missing_key"
        "cr_powerflex_observability_custom_cert"
        "cr_powerflex_observability"
    )
    for i in "${imageArr[@]}"; do
        yq -i '.spec.driver.common.image = "'"$new_image_version"'"' pkg/modules/testdata/$i.yaml
    done

    cp -a tests/config/driverconfig/powerflex/v$previous_major_driver_version/. tests/config/driverconfig/powerflex/v$driver_version_update
    rm -r tests/config/driverconfig/powerflex/v$driver_delete_version

    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' tests/config/driverconfig/powerflex/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' tests/config/driverconfig/powerflex/v$driver_version_update/node.yaml
    yq eval -i 'with(select(.spec.template.spec.initContainers[0].name == "mdm-container"); .spec.template.spec.initContainers[0].image = "'"$new_image_version"'")' tests/config/driverconfig/powerflex/v$driver_version_update/node.yaml

    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' tests/config/driverconfig/powerflex/v$driver_version_update/upgrade-path.yaml

    # Update config version in testfiles
    testfiles="tests/e2e/testfiles"
    prefix="storage_csm_powerflex"
    configArr=($(find "$testfiles" -type f -name "${prefix}*"))
    for i in "${configArr[@]}"; do
        yq eval -i '(.spec.driver.configVersion) |= "'"$update_config_version"'"' $i
    done

    previous_driver_config_version="v$previous_major_driver_version"
    previous_driver_image_version="quay.io/dell/container-storage-modules/csi-vxflexos:v$previous_major_driver_version"

    # Update config version to n-1 in testfiles
    declare -a configArr=(
        "storage_csm_powerflex_auth_n_minus_1"
        "storage_csm_powerflex_downgrade"
    )
    for i in "${configArr[@]}"; do
        yq eval -i '(.spec.driver.configVersion) |= "'"$previous_driver_config_version"'"' tests/e2e/testfiles/$i.yaml
    done

    # Update image version to n-1 in testfiles
    declare -a imageArr=(
        "storage_csm_powerflex_auth_n_minus_1"
        "storage_csm_powerflex_downgrade"
    )
    for i in "${imageArr[@]}"; do
        yq -i '.spec.driver.common.image = "'"$previous_driver_image_version"'"' tests/e2e/testfiles/$i.yaml
    done

    # Update config version in minimal testfiles
    testfiles="tests/e2e/testfiles/minimal-testfiles"
    configArr=($(find "$testfiles" -type f -name "${prefix}*"))
    for i in "${configArr[@]}"; do
        yq eval -i '(.spec.driver.configVersion) |= "'"$update_config_version"'"' $i
    done

    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[6].value = "'"$new_image_version"'")' config/manager/manager.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[6].value = "'"$new_image_version"'")' deploy/operator.yaml

    find . -type f \( -name "*.yaml" -o -name "*.yml" \) -exec sed -i 's/" # /"  # /g' {} +
}

# For Updating Powerflex Driver Patch Version

UpdatePatchPowerflexDriver() {
    driver_version_update=$1
    release_type=$2

    major_version=${driver_version_update%%.*}
    minor_version_tmp=${driver_version_update#*.}
    minor_version=${minor_version_tmp%%.*}
    patch_version=${driver_version_update##*.}

    previous_patch_version=$((patch_version - 1))
    previous_patch_driver_version="$major_version.$minor_version.$previous_patch_version"

    driver_sample_file_suffix=$(echo "$driver_version_update" | tr -d '.' | tr -d '\n')
    previous_driver_sample_file_suffix=$(echo "$previous_patch_driver_version" | tr -d '.' | tr -d '\n')

    sample_version_folder="samples/v$major_version.$minor_version.0"

    # Ensure the directory exists
    mkdir -p "$sample_version_folder/minimal-samples"

    # Copy latest patch file to create new patch version
    cp -v "$sample_version_folder/storage_csm_powerflex_v$previous_driver_sample_file_suffix.yaml" \
          "$sample_version_folder/storage_csm_powerflex_v$driver_sample_file_suffix.yaml"
    cp -v "$sample_version_folder/minimal-samples/powerflex_v$previous_driver_sample_file_suffix.yaml" \
          "$sample_version_folder/minimal-samples/powerflex_v$driver_sample_file_suffix.yaml"

    update_config_version="v$driver_version_update"
    if [ "$release_type" == "nightly" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-vxflexos:nightly"
    elif [ "$release_type" == "tag" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-vxflexos:v$driver_version_update"
    fi

    # Update new sample file with configVersion and image
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' "$sample_version_folder/storage_csm_powerflex_v$driver_sample_file_suffix.yaml"
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' "$sample_version_folder/storage_csm_powerflex_v$driver_sample_file_suffix.yaml"

    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' "$sample_version_folder/minimal-samples/powerflex_v$driver_sample_file_suffix.yaml"
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' "$sample_version_folder/minimal-samples/powerflex_v$driver_sample_file_suffix.yaml"

    # Remove old patch version sample files
    rm -v "$sample_version_folder/storage_csm_powerflex_v$previous_driver_sample_file_suffix.yaml"
    rm -v "$sample_version_folder/minimal-samples/powerflex_v$previous_driver_sample_file_suffix.yaml"

    # Update operator driver config
    cp -a operatorconfig/driverconfig/powerflex/v$previous_patch_driver_version \
          operatorconfig/driverconfig/powerflex/v$driver_version_update

    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' \
        operatorconfig/driverconfig/powerflex/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' \
        operatorconfig/driverconfig/powerflex/v$driver_version_update/node.yaml
    yq eval -i 'with(select(.spec.template.spec.initContainers[0].name == "mdm-container"); .spec.template.spec.initContainers[0].image = "'"$new_image_version"'")' \
        operatorconfig/driverconfig/powerflex/v$driver_version_update/node.yaml

    rm -r operatorconfig/driverconfig/powerflex/v$previous_patch_driver_version

    min_upgrade_path=$(GetMinUpgradePath "$sample_version_folder")
    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' operatorconfig/driverconfig/powerflex/v$driver_version_update/upgrade-path.yaml

    # Update related images in CSV
    UpdateConfigVersion csi-vxflexos $update_config_version
    if [ "$release_type" == "nightly" ]; then
        UpdateNightlyRelatedImages csi-vxflexos
        UpdateNightlyBaseRelatedImages csi-vxflexos
    else
        UpdateRelatedImages csi-vxflexos $update_config_version
        UpdateBaseRelatedImages csi-vxflexos $update_config_version
    fi

    # Update test data files
    for i in \
        cr_powerflex_observability_custom_cert_missing_key \
        cr_powerflex_observability_custom_cert \
        cr_powerflex_observability \
        cr_powerflex_replica \
        cr_powerflex_resiliency
    do
        yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' pkg/modules/testdata/$i.yaml
    done

    for i in \
        cr_powerflex_observability_custom_cert_missing_key \
        cr_powerflex_observability_custom_cert \
        cr_powerflex_observability
    do
        yq -i '.spec.driver.common.image = "'"$new_image_version"'"' pkg/modules/testdata/$i.yaml
    done

    # Test config updates
    cp -a tests/config/driverconfig/powerflex/v$previous_patch_driver_version \
          tests/config/driverconfig/powerflex/v$driver_version_update
    rm -r tests/config/driverconfig/powerflex/v$previous_patch_driver_version

    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' \
        tests/config/driverconfig/powerflex/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' \
        tests/config/driverconfig/powerflex/v$driver_version_update/node.yaml
    yq eval -i 'with(select(.spec.template.spec.initContainers[0].name == "mdm-container"); .spec.template.spec.initContainers[0].image = "'"$new_image_version"'")' \
        tests/config/driverconfig/powerflex/v$driver_version_update/node.yaml

    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' tests/config/driverconfig/powerflex/v$driver_version_update/upgrade-path.yaml

    # Update e2e test sample versions
    testfiles="tests/e2e/testfiles"
    for f in $(find "$testfiles" -type f -name "storage_csm_powerflex*"); do
        yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' "$f"
    done
    testfiles="tests/e2e/testfiles/minimal-testfiles"
    for f in $(find "$testfiles" -type f -name "storage_csm_powerflex*"); do
        yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' "$f"
    done

    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[6].value = "'"$new_image_version"'")' config/manager/manager.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[6].value = "'"$new_image_version"'")' deploy/operator.yaml

    # Fix formatting (optional)
    find . -type f \( -name "*.yaml" -o -name "*.yml" \) -exec sed -i 's/" # /"  # /g' {} +
}

# For Updating Powermax Driver Major Version
UpdateMajorPowermaxDriver() {
    driver_version_update=$1
    release_type=$2
    # Extract the values of major_version, minor_version, and patch_version from the input string
    major_version=${driver_version_update%%.*}
    minor_version=${driver_version_update#*.}
    minor_version=${minor_version%%.*}
    patch_version=${driver_version_update##*.}

    previous_major_driver_version=$(GetLatestDriverVersion "storage_csm_powermax")

    driver_sample_file_suffix=$(echo "$driver_version_update" | tr -d '.' | tr -d '\n')
    CreateLatestSampleFile "storage_csm_powermax" $driver_sample_file_suffix
    CreateLatestMinimalSampleFile "powermax" $driver_sample_file_suffix

    update_config_version="v$driver_version_update"

    # Replace the config version in the file
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' samples/storage_csm_powermax_v$driver_sample_file_suffix.yaml
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' samples/minimal-samples/powermax_v$driver_sample_file_suffix.yaml

    # Specify the new image versions
    if [ "$release_type" == "nightly" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-powermax:nightly"
    elif [ "$release_type" == "tag" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-powermax:v$driver_version_update"
    fi

    # Replace the image version in the file
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' samples/storage_csm_powermax_v$driver_sample_file_suffix.yaml
    cp -v samples/storage_csm_powermax_v$driver_sample_file_suffix.yaml config/samples/storage_v1_csm_powermax.yaml

    cp -a operatorconfig/driverconfig/powermax/v$previous_major_driver_version/. operatorconfig/driverconfig/powermax/v$driver_version_update
    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' operatorconfig/driverconfig/powermax/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' operatorconfig/driverconfig/powermax/v$driver_version_update/node.yaml

    delete_minor_version=$((minor_version - 3))
    driver_delete_version="$major_version.$delete_minor_version.$patch_version"
    driver_delete_version_sample_file_suffix=$(echo "$driver_delete_version" | tr -d '.' | tr -d '\n')
    rm samples/storage_csm_powermax_v$driver_delete_version_sample_file_suffix.yaml
    rm samples/minimal-samples/powermax_v$driver_delete_version_sample_file_suffix.yaml
    rm -r operatorconfig/driverconfig/powermax/v$driver_delete_version

    min_upgrade_path=$(GetMinUpgradePath "storage_csm_powermax")
    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' operatorconfig/driverconfig/powermax/v$driver_version_update/upgrade-path.yaml

    # Update config version in bundle/manifests/dell-csm-operator.clusterserviceversion.yaml
    UpdateConfigVersion csi-powermax $update_config_version

    # Update driver version in bundle/manifests/dell-csm-operator.clusterserviceversion.yaml and config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml
    if [ "$release_type" == "nightly" ]; then
        UpdateNightlyRelatedImages csi-powermax
        UpdateNightlyBaseRelatedImages csi-powermax
    elif [ "$release_type" == "tag" ]; then
        UpdateRelatedImages csi-powermax $update_config_version
        UpdateBaseRelatedImages csi-powermax $update_config_version
    fi

    declare -a configArr=(
        "cr_powermax_observability_use_secret"
        "cr_powermax_observability"
        "cr_powermax_replica"
        "cr_powermax_resiliency"
        "cr_powermax_reverseproxy_sidecar"
        "cr_powermax_reverseproxy_use_secret"
        "cr_powermax_reverseproxy"
    )
    for i in "${configArr[@]}"; do
        yq -i e '.spec.driver.configVersion = "'"$update_config_version"'"' pkg/modules/testdata/$i.yaml
        yq -i e '.spec.driver.common.image = "'"$new_image_version"'"' pkg/modules/testdata/$i.yaml
    done

    cp -a tests/config/driverconfig/powermax/v$previous_major_driver_version/. tests/config/driverconfig/powermax/v$driver_version_update
    rm -r tests/config/driverconfig/powermax/v$driver_delete_version

    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' tests/config/driverconfig/powermax/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' tests/config/driverconfig/powermax/v$driver_version_update/node.yaml

    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' tests/config/driverconfig/powermax/v$driver_version_update/upgrade-path.yaml

    # Update config version in testfiles
    testfiles="tests/e2e/testfiles"
    prefix="storage_csm_powermax"
    configArr=($(find "$testfiles" -type f -name "${prefix}*"))
    for i in "${configArr[@]}"; do
        yq eval -i '(.spec.driver.configVersion) |= "'"$update_config_version"'"' $i
    done

    # Update config version in minimal testfiles
    testfiles="tests/e2e/testfiles/minimal-testfiles"
    configArr=($(find "$testfiles" -type f -name "${prefix}*"))
    for i in "${configArr[@]}"; do
        yq eval -i '(.spec.driver.configVersion) |= "'"$update_config_version"'"' $i
    done

    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[2].value = "'"$new_image_version"'")' config/manager/manager.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[2].value = "'"$new_image_version"'")' deploy/operator.yaml
}

# For Updating Powermax Driver Patch Version
UpdatePatchPowermaxDriver() {
    driver_version_update=$1
    release_type=$2

    # Extract version components
    major_version=${driver_version_update%%.*}
    minor_tmp=${driver_version_update#*.}
    minor_version=${minor_tmp%%.*}
    patch_version=${driver_version_update##*.}

    previous_patch_version=$((patch_version - 1))
    previous_patch_driver_version="$major_version.$minor_version.$previous_patch_version"

    driver_sample_file_suffix=$(echo "$driver_version_update" | tr -d '.' | tr -d '\n')
    previous_driver_sample_file_suffix=$(echo "$previous_patch_driver_version" | tr -d '.' | tr -d '\n')

    sample_version_folder="samples/v$major_version.$minor_version.0"
    mkdir -p "$sample_version_folder/minimal-samples"

    cp -v "$sample_version_folder/storage_csm_powermax_v$previous_driver_sample_file_suffix.yaml" \
          "$sample_version_folder/storage_csm_powermax_v$driver_sample_file_suffix.yaml"
    cp -v "$sample_version_folder/minimal-samples/powermax_v$previous_driver_sample_file_suffix.yaml" \
          "$sample_version_folder/minimal-samples/powermax_v$driver_sample_file_suffix.yaml"

    update_config_version="v$driver_version_update"
    if [ "$release_type" == "nightly" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-powermax:nightly"
    else
        new_image_version="quay.io/dell/container-storage-modules/csi-powermax:v$driver_version_update"
    fi

    # Update image + config version in sample files
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' \
        "$sample_version_folder/storage_csm_powermax_v$driver_sample_file_suffix.yaml"
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' \
        "$sample_version_folder/storage_csm_powermax_v$driver_sample_file_suffix.yaml"

    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' \
        "$sample_version_folder/minimal-samples/powermax_v$driver_sample_file_suffix.yaml"
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' \
        "$sample_version_folder/minimal-samples/powermax_v$driver_sample_file_suffix.yaml"

    # Remove old patch sample files
    rm -v "$sample_version_folder/storage_csm_powermax_v$previous_driver_sample_file_suffix.yaml"
    rm -v "$sample_version_folder/minimal-samples/powermax_v$previous_driver_sample_file_suffix.yaml"

    # Operator config updates
    cp -a operatorconfig/driverconfig/powermax/v$previous_patch_driver_version \
          operatorconfig/driverconfig/powermax/v$driver_version_update
    rm -r operatorconfig/driverconfig/powermax/v$previous_patch_driver_version

    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' \
        operatorconfig/driverconfig/powermax/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' \
        operatorconfig/driverconfig/powermax/v$driver_version_update/node.yaml

    min_upgrade_path=$(GetMinUpgradePath "$sample_version_folder")
    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' \
        operatorconfig/driverconfig/powermax/v$driver_version_update/upgrade-path.yaml

    UpdateConfigVersion csi-powermax $update_config_version
    if [ "$release_type" == "nightly" ]; then
        UpdateNightlyRelatedImages csi-powermax
        UpdateNightlyBaseRelatedImages csi-powermax
    else
        UpdateRelatedImages csi-powermax $update_config_version
        UpdateBaseRelatedImages csi-powermax $update_config_version
    fi

    # Update testdata YAMLs
    declare -a configArr=(
        "cr_powermax_observability_use_secret"
        "cr_powermax_observability"
        "cr_powermax_replica"
        "cr_powermax_resiliency"
        "cr_powermax_reverseproxy_sidecar"
        "cr_powermax_reverseproxy_use_secret"
        "cr_powermax_reverseproxy"
    )
    for i in "${configArr[@]}"; do
        yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' pkg/modules/testdata/$i.yaml
        yq -i '.spec.driver.common.image = "'"$new_image_version"'"' pkg/modules/testdata/$i.yaml
    done

    cp -a tests/config/driverconfig/powermax/v$previous_patch_driver_version \
          tests/config/driverconfig/powermax/v$driver_version_update
    rm -r tests/config/driverconfig/powermax/v$previous_patch_driver_version

    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' \
        tests/config/driverconfig/powermax/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' \
        tests/config/driverconfig/powermax/v$driver_version_update/node.yaml

    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' \
        tests/config/driverconfig/powermax/v$driver_version_update/upgrade-path.yaml

    # Update config version in testfiles
    for i in $(find tests/e2e/testfiles -type f -name "storage_csm_powermax*"); do
        yq eval -i '.spec.driver.configVersion = "'"$update_config_version"'"' "$i"
    done
    for i in $(find tests/e2e/testfiles/minimal-testfiles -type f -name "storage_csm_powermax*"); do
        yq eval -i '.spec.driver.configVersion = "'"$update_config_version"'"' "$i"
    done

    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[2].value = "'"$new_image_version"'")' config/manager/manager.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[2].value = "'"$new_image_version"'")' deploy/operator.yaml
}

# For Updating Powerscale Driver Major Version
UpdateMajorPowerscaleDriver() {
    driver_version_update=$1
    release_type=$2
    # Extract the values of major_version, minor_version, and patch_version from the input string
    major_version=${driver_version_update%%.*}
    minor_version=${driver_version_update#*.}
    minor_version=${minor_version%%.*}
    patch_version=${driver_version_update##*.}

    previous_major_driver_version=$(GetLatestDriverVersion "storage_csm_powerscale")

    driver_sample_file_suffix=$(echo "$driver_version_update" | tr -d '.' | tr -d '\n')
    CreateLatestSampleFile "storage_csm_powerscale" $driver_sample_file_suffix
    CreateLatestMinimalSampleFile "powerscale" $driver_sample_file_suffix

    update_config_version="v$driver_version_update"

    # Replace the config version in the file
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' samples/storage_csm_powerscale_v$driver_sample_file_suffix.yaml
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' samples/minimal-samples/powerscale_v$driver_sample_file_suffix.yaml

    # Specify the new image versions
    if [ "$release_type" == "nightly" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-isilon:nightly"
    elif [ "$release_type" == "tag" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-isilon:v$driver_version_update"
    fi

    # Replace the image version in the file
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' samples/storage_csm_powerscale_v$driver_sample_file_suffix.yaml
    cp -v samples/storage_csm_powerscale_v$driver_sample_file_suffix.yaml config/samples/storage_v1_csm_powerscale.yaml

    cp -a operatorconfig/driverconfig/powerscale/v$previous_major_driver_version/. operatorconfig/driverconfig/powerscale/v$driver_version_update
    yq eval -i 'with(select(.spec.template.spec.containers[6].name == "driver"); .spec.template.spec.containers[6].image = "'"$new_image_version"'")' operatorconfig/driverconfig/powerscale/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' operatorconfig/driverconfig/powerscale/v$driver_version_update/node.yaml

    delete_minor_version=$((minor_version - 3))
    driver_delete_version="$major_version.$delete_minor_version.$patch_version"
    driver_delete_version_sample_file_suffix=$(echo "$driver_delete_version" | tr -d '.' | tr -d '\n')
    rm samples/storage_csm_powerscale_v$driver_delete_version_sample_file_suffix.yaml
    rm samples/minimal-samples/powerscale_v$driver_delete_version_sample_file_suffix.yaml
    rm -r operatorconfig/driverconfig/powerscale/v$driver_delete_version

    min_upgrade_path=$(GetMinUpgradePath "storage_csm_powerscale")
    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' operatorconfig/driverconfig/powerscale/v$driver_version_update/upgrade-path.yaml

    # Update config version in bundle/manifests/dell-csm-operator.clusterserviceversion.yaml
    UpdateConfigVersion csi-isilon $update_config_version

    # Update driver version in bundle/manifests/dell-csm-operator.clusterserviceversion.yaml and config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml
    if [ "$release_type" == "nightly" ]; then
        UpdateNightlyRelatedImages csi-isilon
        UpdateNightlyBaseRelatedImages csi-isilon
    elif [ "$release_type" == "tag" ]; then
        UpdateRelatedImages csi-isilon $update_config_version
        UpdateBaseRelatedImages csi-isilon $update_config_version
    fi

    declare -a configArr=(
        "cr_powerscale_auth_missing_skip_cert_env"
        "cr_powerscale_auth_validate_cert"
        "cr_powerscale_auth"
        "cr_powerscale_observability"
        "cr_powerscale_replica"
        "cr_powerscale_resiliency"
    )
    for i in "${configArr[@]}"; do
        yq -i e '.spec.driver.configVersion = "'"$update_config_version"'"' pkg/modules/testdata/$i.yaml
        yq -i e '.spec.driver.common.image = "'"$new_image_version"'"' pkg/modules/testdata/$i.yaml
    done

    cp -a tests/config/driverconfig/powerscale/v$previous_major_driver_version/. tests/config/driverconfig/powerscale/v$driver_version_update
    rm -r tests/config/driverconfig/powerscale/v$driver_delete_version

    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' tests/config/driverconfig/powerscale/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' tests/config/driverconfig/powerscale/v$driver_version_update/node.yaml

    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' tests/config/driverconfig/powerscale/v$driver_version_update/upgrade-path.yaml

    # Update config version in testfiles
    testfiles="tests/e2e/testfiles"
    prefix="storage_csm_powerscale"
    configArr=($(find "$testfiles" -type f -name "${prefix}*"))
    for i in "${configArr[@]}"; do
        yq eval -i '(.spec.driver.configVersion) |= "'"$update_config_version"'"' $i
    done

    previous_driver_config_version="v$previous_major_driver_version"
    previous_driver_image_version="quay.io/dell/container-storage-modules/csi-isilon:v$previous_major_driver_version"

    # Update config version to n-1 in testfiles
    declare -a configArr=(
        "storage_csm_powerscale_observability_val1"
    )
    for i in "${configArr[@]}"; do
        yq eval -i '(.spec.driver.configVersion) |= "'"$previous_driver_config_version"'"' tests/e2e/testfiles/$i.yaml
    done

    # Update image version to n-1 in testfiles
    declare -a imageArr=(
        "storage_csm_powerscale_observability_val1"
    )
    for i in "${imageArr[@]}"; do
        yq -i '.spec.driver.common.image = "'"$previous_driver_image_version"'"' tests/e2e/testfiles/$i.yaml
    done

    # Update config version in minimal testfiles
    testfiles="tests/e2e/testfiles/minimal-testfiles"
    configArr=($(find "$testfiles" -type f -name "${prefix}*"))
    for i in "${configArr[@]}"; do
        yq eval -i '(.spec.driver.configVersion) |= "'"$update_config_version"'"' $i
    done

    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[1].value = "'"$new_image_version"'")' config/manager/manager.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[1].value = "'"$new_image_version"'")' deploy/operator.yaml
}

# For Updating Powerscale Driver Patch Version
UpdatePatchPowerscaleDriver() {
    driver_version_update=$1
    release_type=$2

    # Parse version components
    major_version=${driver_version_update%%.*}
    minor_tmp=${driver_version_update#*.}
    minor_version=${minor_tmp%%.*}
    patch_version=${driver_version_update##*.}

    previous_patch_version=$((patch_version - 1))
    previous_patch_driver_version="$major_version.$minor_version.$previous_patch_version"

    driver_sample_file_suffix=$(echo "$driver_version_update" | tr -d '.' | tr -d '\n')
    previous_driver_sample_file_suffix=$(echo "$previous_patch_driver_version" | tr -d '.' | tr -d '\n')

    sample_version_folder="samples/v$major_version.$minor_version.0"
    mkdir -p "$sample_version_folder/minimal-samples"

    # Copy previous patch as new patch
    cp -v "$sample_version_folder/storage_csm_powerscale_v$previous_driver_sample_file_suffix.yaml" \
          "$sample_version_folder/storage_csm_powerscale_v$driver_sample_file_suffix.yaml"
    cp -v "$sample_version_folder/minimal-samples/powerscale_v$previous_driver_sample_file_suffix.yaml" \
          "$sample_version_folder/minimal-samples/powerscale_v$driver_sample_file_suffix.yaml"

    update_config_version="v$driver_version_update"
    if [ "$release_type" == "nightly" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-isilon:nightly"
    else
        new_image_version="quay.io/dell/container-storage-modules/csi-isilon:v$driver_version_update"
    fi

    # Patch values in copied files
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' \
        "$sample_version_folder/storage_csm_powerscale_v$driver_sample_file_suffix.yaml"
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' \
        "$sample_version_folder/storage_csm_powerscale_v$driver_sample_file_suffix.yaml"

    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' \
        "$sample_version_folder/minimal-samples/powerscale_v$driver_sample_file_suffix.yaml"
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' \
        "$sample_version_folder/minimal-samples/powerscale_v$driver_sample_file_suffix.yaml"

    # Remove old patch sample
    rm -v "$sample_version_folder/storage_csm_powerscale_v$previous_driver_sample_file_suffix.yaml"
    rm -v "$sample_version_folder/minimal-samples/powerscale_v$previous_driver_sample_file_suffix.yaml"

    # Operator config update
    cp -a operatorconfig/driverconfig/powerscale/v$previous_patch_driver_version \
          operatorconfig/driverconfig/powerscale/v$driver_version_update
    rm -r operatorconfig/driverconfig/powerscale/v$previous_patch_driver_version

    yq eval -i 'with(select(.spec.template.spec.containers[6].name == "driver"); .spec.template.spec.containers[6].image = "'"$new_image_version"'")' \
        operatorconfig/driverconfig/powerscale/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' \
        operatorconfig/driverconfig/powerscale/v$driver_version_update/node.yaml

    min_upgrade_path=$(GetMinUpgradePath "$sample_version_folder")
    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' \
        operatorconfig/driverconfig/powerscale/v$driver_version_update/upgrade-path.yaml

    # CSV updates
    UpdateConfigVersion csi-isilon $update_config_version
    if [ "$release_type" == "nightly" ]; then
        UpdateNightlyRelatedImages csi-isilon
        UpdateNightlyBaseRelatedImages csi-isilon
    else
        UpdateRelatedImages csi-isilon $update_config_version
        UpdateBaseRelatedImages csi-isilon $update_config_version
    fi

    # Testdata files
    for i in \
        cr_powerscale_auth_missing_skip_cert_env \
        cr_powerscale_auth_validate_cert \
        cr_powerscale_auth \
        cr_powerscale_observability \
        cr_powerscale_replica \
        cr_powerscale_resiliency
    do
        yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' pkg/modules/testdata/$i.yaml
        yq -i '.spec.driver.common.image = "'"$new_image_version"'"' pkg/modules/testdata/$i.yaml
    done

    # Tests/config
    cp -a tests/config/driverconfig/powerscale/v$previous_patch_driver_version \
          tests/config/driverconfig/powerscale/v$driver_version_update
    rm -r tests/config/driverconfig/powerscale/v$previous_patch_driver_version

    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' \
        tests/config/driverconfig/powerscale/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' \
        tests/config/driverconfig/powerscale/v$driver_version_update/node.yaml

    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' \
        tests/config/driverconfig/powerscale/v$driver_version_update/upgrade-path.yaml

    # E2E TestFiles
    for f in $(find tests/e2e/testfiles -type f -name "storage_csm_powerscale*"); do
        yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' "$f"
    done
    for f in $(find tests/e2e/testfiles/minimal-testfiles -type f -name "storage_csm_powerscale*"); do
        yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' "$f"
    done

    # Manager image updates
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[1].value = "'"$new_image_version"'")' config/manager/manager.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[1].value = "'"$new_image_version"'")' deploy/operator.yaml
}

# For Updating Powerstore Driver Major Version
UpdateMajorPowerstoreDriver() {
    driver_version_update=$1
    release_type=$2
    # Extract the values of major_version, minor_version, and patch_version from the input string
    major_version=${driver_version_update%%.*}
    minor_version=${driver_version_update#*.}
    minor_version=${minor_version%%.*}
    patch_version=${driver_version_update##*.}

    previous_major_driver_version=$(GetLatestDriverVersion "storage_csm_powerstore")

    driver_sample_file_suffix=$(echo "$driver_version_update" | tr -d '.' | tr -d '\n')
    CreateLatestSampleFile "storage_csm_powerstore" $driver_sample_file_suffix
    CreateLatestMinimalSampleFile "powerstore" $driver_sample_file_suffix

    update_config_version="v$driver_version_update"

    # Replace the config version in the file
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' samples/storage_csm_powerstore_v$driver_sample_file_suffix.yaml
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' samples/minimal-samples/powerstore_v$driver_sample_file_suffix.yaml

    # Specify the new image versions
    if [ "$release_type" == "nightly" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-powerstore:nightly"
    elif [ "$release_type" == "tag" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-powerstore:v$driver_version_update"
    fi

    # Replace the image version in the file
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' samples/storage_csm_powerstore_v$driver_sample_file_suffix.yaml
    cp -v samples/storage_csm_powerstore_v$driver_sample_file_suffix.yaml config/samples/storage_v1_csm_powerstore.yaml

    cp -a operatorconfig/driverconfig/powerstore/v$previous_major_driver_version/. operatorconfig/driverconfig/powerstore/v$driver_version_update
    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' operatorconfig/driverconfig/powerstore/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' operatorconfig/driverconfig/powerstore/v$driver_version_update/node.yaml

    delete_minor_version=$((minor_version - 3))
    driver_delete_version="$major_version.$delete_minor_version.$patch_version"
    driver_delete_version_sample_file_suffix=$(echo "$driver_delete_version" | tr -d '.' | tr -d '\n')
    rm samples/storage_csm_powerstore_v$driver_delete_version_sample_file_suffix.yaml
    rm samples/minimal-samples/powerstore_v$driver_delete_version_sample_file_suffix.yaml
    rm -r operatorconfig/driverconfig/powerstore/v$driver_delete_version

    min_upgrade_path=$(GetMinUpgradePath "storage_csm_powerscale")
    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' operatorconfig/driverconfig/powerscale/v$driver_version_update/upgrade-path.yaml

    # Update config version in bundle/manifests/dell-csm-operator.clusterserviceversion.yaml
    UpdateConfigVersion csi-powerstore $update_config_version

    # Update driver version in bundle/manifests/dell-csm-operator.clusterserviceversion.yaml and config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml
    if [ "$release_type" == "nightly" ]; then
        UpdateNightlyRelatedImages csi-powerstore
        UpdateNightlyBaseRelatedImages csi-powerstore
    elif [ "$release_type" == "tag" ]; then
        UpdateRelatedImages csi-powerstore $update_config_version
        UpdateBaseRelatedImages csi-powerstore $update_config_version
    fi

    yq -i e '.spec.driver.common.image = "'"$new_image_version"'"' pkg/modules/testdata/cr_powerstore_resiliency.yaml
    yq -i e '.spec.driver.configVersion = "'"$update_config_version"'"' pkg/modules/testdata/cr_powerstore_resiliency.yaml

    cp -a tests/config/driverconfig/powerstore/v$previous_major_driver_version/. tests/config/driverconfig/powerstore/v$driver_version_update
    rm -r tests/config/driverconfig/powerstore/v$driver_delete_version

    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' tests/config/driverconfig/powerstore/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' tests/config/driverconfig/powerstore/v$driver_version_update/node.yaml

    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' tests/config/driverconfig/powerstore/v$driver_version_update/upgrade-path.yaml

    # Update config version in testfiles
    testfiles="tests/e2e/testfiles"
    prefix="storage_csm_powerstore"
    configArr=($(find "$testfiles" -type f -name "${prefix}*"))
    for i in "${configArr[@]}"; do
        yq eval -i '(.spec.driver.configVersion) |= "'"$update_config_version"'"' $i
    done

    # Update config version in minimal testfiles
    testfiles="tests/e2e/testfiles/minimal-testfiles"
    configArr=($(find "$testfiles" -type f -name "${prefix}*"))
    for i in "${configArr[@]}"; do
        yq eval -i '(.spec.driver.configVersion) |= "'"$update_config_version"'"' $i
    done

    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[4].value = "'"$new_image_version"'")' config/manager/manager.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[4].value = "'"$new_image_version"'")' deploy/operator.yaml
}

# For Updating Powerstore Driver Patch Version
UpdatePatchPowerstoreDriver() {
    driver_version_update=$1
    release_type=$2

    # Extract version components
    major_version=${driver_version_update%%.*}
    minor_tmp=${driver_version_update#*.}
    minor_version=${minor_tmp%%.*}
    patch_version=${driver_version_update##*.}

    previous_patch_version=$((patch_version - 1))
    previous_patch_driver_version="$major_version.$minor_version.$previous_patch_version"

    driver_sample_file_suffix=$(echo "$driver_version_update" | tr -d '.' | tr -d '\n')
    previous_driver_sample_file_suffix=$(echo "$previous_patch_driver_version" | tr -d '.' | tr -d '\n')

    sample_version_folder="samples/v$major_version.$minor_version.0"
    mkdir -p "$sample_version_folder/minimal-samples"

    # Copy previous patch as new patch
    cp -v "$sample_version_folder/storage_csm_powerstore_v$previous_driver_sample_file_suffix.yaml" \
          "$sample_version_folder/storage_csm_powerstore_v$driver_sample_file_suffix.yaml"
    cp -v "$sample_version_folder/minimal-samples/powerstore_v$previous_driver_sample_file_suffix.yaml" \
          "$sample_version_folder/minimal-samples/powerstore_v$driver_sample_file_suffix.yaml"

    update_config_version="v$driver_version_update"
    if [ "$release_type" == "nightly" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-powerstore:nightly"
    else
        new_image_version="quay.io/dell/container-storage-modules/csi-powerstore:v$driver_version_update"
    fi

    # Update configVersion and image in new sample
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' \
        "$sample_version_folder/storage_csm_powerstore_v$driver_sample_file_suffix.yaml"
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' \
        "$sample_version_folder/storage_csm_powerstore_v$driver_sample_file_suffix.yaml"

    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' \
        "$sample_version_folder/minimal-samples/powerstore_v$driver_sample_file_suffix.yaml"
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' \
        "$sample_version_folder/minimal-samples/powerstore_v$driver_sample_file_suffix.yaml"

    # Delete old patch
    rm -v "$sample_version_folder/storage_csm_powerstore_v$previous_driver_sample_file_suffix.yaml"
    rm -v "$sample_version_folder/minimal-samples/powerstore_v$previous_driver_sample_file_suffix.yaml"

    # Operator config patch
    cp -a operatorconfig/driverconfig/powerstore/v$previous_patch_driver_version \
          operatorconfig/driverconfig/powerstore/v$driver_version_update
    rm -r operatorconfig/driverconfig/powerstore/v$previous_patch_driver_version

    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' \
        operatorconfig/driverconfig/powerstore/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' \
        operatorconfig/driverconfig/powerstore/v$driver_version_update/node.yaml

    min_upgrade_path=$(GetMinUpgradePath "$sample_version_folder")
    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' \
        operatorconfig/driverconfig/powerstore/v$driver_version_update/upgrade-path.yaml

    # CSV and image reference updates
    UpdateConfigVersion csi-powerstore $update_config_version
    if [ "$release_type" == "nightly" ]; then
        UpdateNightlyRelatedImages csi-powerstore
        UpdateNightlyBaseRelatedImages csi-powerstore
    else
        UpdateRelatedImages csi-powerstore $update_config_version
        UpdateBaseRelatedImages csi-powerstore $update_config_version
    fi

    # Testdata patching
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' pkg/modules/testdata/cr_powerstore_resiliency.yaml
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' pkg/modules/testdata/cr_powerstore_resiliency.yaml

    # Test driver config
    cp -a tests/config/driverconfig/powerstore/v$previous_patch_driver_version \
          tests/config/driverconfig/powerstore/v$driver_version_update
    rm -r tests/config/driverconfig/powerstore/v$previous_patch_driver_version

    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' \
        tests/config/driverconfig/powerstore/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' \
        tests/config/driverconfig/powerstore/v$driver_version_update/node.yaml

    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' \
        tests/config/driverconfig/powerstore/v$driver_version_update/upgrade-path.yaml

    # e2e test patching
    for f in $(find tests/e2e/testfiles -type f -name "storage_csm_powerstore*"); do
        yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' "$f"
    done
    for f in $(find tests/e2e/testfiles/minimal-testfiles -type f -name "storage_csm_powerstore*"); do
        yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' "$f"
    done

    # Manager env image patch
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[4].value = "'"$new_image_version"'")' config/manager/manager.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[4].value = "'"$new_image_version"'")' deploy/operator.yaml
}

# For Updating Unity Driver Major Version
UpdateMajorUnityDriver() {
    driver_version_update=$1
    release_type=$2
    # Extract the values of major_version, minor_version, and patch_version from the input string
    major_version=${driver_version_update%%.*}
    minor_version=${driver_version_update#*.}
    minor_version=${minor_version%%.*}
    patch_version=${driver_version_update##*.}

    previous_major_driver_version=$(GetLatestDriverVersion "storage_csm_unity")

    driver_sample_file_suffix=$(echo "$driver_version_update" | tr -d '.' | tr -d '\n')
    CreateLatestSampleFile "storage_csm_unity" $driver_sample_file_suffix
    CreateLatestMinimalSampleFile "unity" $driver_sample_file_suffix

    update_config_version="v$driver_version_update"

    # Replace the config version in the file
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' samples/storage_csm_unity_v$driver_sample_file_suffix.yaml
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' samples/minimal-samples/unity_v$driver_sample_file_suffix.yaml

    # Specify the new image versions
    if [ "$release_type" == "nightly" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-unity:nightly"
    elif [ "$release_type" == "tag" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-unity:v$driver_version_update"
    fi

    # Replace the image version in the file
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' samples/storage_csm_unity_v$driver_sample_file_suffix.yaml
    cp -v samples/storage_csm_unity_v$driver_sample_file_suffix.yaml config/samples/storage_v1_csm_unity.yaml

    cp -a operatorconfig/driverconfig/unity/v$previous_major_driver_version/. operatorconfig/driverconfig/unity/v$driver_version_update
    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' operatorconfig/driverconfig/unity/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' operatorconfig/driverconfig/unity/v$driver_version_update/node.yaml

    delete_minor_version=$((minor_version - 3))
    driver_delete_version="$major_version.$delete_minor_version.$patch_version"
    driver_delete_version_sample_file_suffix=$(echo "$driver_delete_version" | tr -d '.' | tr -d '\n')
    rm samples/storage_csm_unity_v$driver_delete_version_sample_file_suffix.yaml
    rm samples/minimal-samples/unity_v$driver_delete_version_sample_file_suffix.yaml
    rm -r operatorconfig/driverconfig/unity/v$driver_delete_version

    min_upgrade_path=$(GetMinUpgradePath "storage_csm_unity")
    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' operatorconfig/driverconfig/unity/v$driver_version_update/upgrade-path.yaml

    # Update config version in bundle/manifests/dell-csm-operator.clusterserviceversion.yaml
    UpdateConfigVersion csi-unity $update_config_version

    # Update driver version in bundle/manifests/dell-csm-operator.clusterserviceversion.yaml and config/manifests/bases/dell-csm-operator.clusterserviceversion.yaml
    if [ "$release_type" == "nightly" ]; then
        UpdateNightlyRelatedImages csi-unity
        UpdateNightlyBaseRelatedImages csi-unity
    elif [ "$release_type" == "tag" ]; then
        UpdateRelatedImages csi-unity $update_config_version
        UpdateBaseRelatedImages csi-unity $update_config_version
    fi

    cp -a tests/config/driverconfig/unity/v$previous_major_driver_version/. tests/config/driverconfig/unity/v$driver_version_update
    rm -r tests/config/driverconfig/unity/v$driver_delete_version

    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' tests/config/driverconfig/unity/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' tests/config/driverconfig/unity/v$driver_version_update/node.yaml

    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' tests/config/driverconfig/unity/v$driver_version_update/upgrade-path.yaml

    # Update config version in testfiles
    testfiles="tests/e2e/testfiles"
    prefix="storage_csm_unity"
    configArr=($(find "$testfiles" -type f -name "${prefix}*"))
    for i in "${configArr[@]}"; do
        yq eval -i '(.spec.driver.configVersion) |= "'"$update_config_version"'"' $i
    done

    # Update config version in minimal testfiles
    testfiles="tests/e2e/testfiles/minimal-testfiles"
    configArr=($(find "$testfiles" -type f -name "${prefix}*"))
    for i in "${configArr[@]}"; do
        yq eval -i '(.spec.driver.configVersion) |= "'"$update_config_version"'"' $i
    done

    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[5].value = "'"$new_image_version"'")' config/manager/manager.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[5].value = "'"$new_image_version"'")' deploy/operator.yaml
}

# For Updating Unity Driver Patch Version
UpdatePatchUnityDriver() {
    driver_version_update=$1
    release_type=$2

    # Extract version components
    major_version=${driver_version_update%%.*}
    minor_tmp=${driver_version_update#*.}
    minor_version=${minor_tmp%%.*}
    patch_version=${driver_version_update##*.}

    previous_patch_version=$((patch_version - 1))
    previous_patch_driver_version="$major_version.$minor_version.$previous_patch_version"

    driver_sample_file_suffix=$(echo "$driver_version_update" | tr -d '.' | tr -d '\n')
    previous_driver_sample_file_suffix=$(echo "$previous_patch_driver_version" | tr -d '.' | tr -d '\n')

    sample_version_folder="samples/v$major_version.$minor_version.0"
    mkdir -p "$sample_version_folder/minimal-samples"

    # Copy previous patch as base
    cp -v "$sample_version_folder/storage_csm_unity_v$previous_driver_sample_file_suffix.yaml" \
          "$sample_version_folder/storage_csm_unity_v$driver_sample_file_suffix.yaml"
    cp -v "$sample_version_folder/minimal-samples/unity_v$previous_driver_sample_file_suffix.yaml" \
          "$sample_version_folder/minimal-samples/unity_v$driver_sample_file_suffix.yaml"

    update_config_version="v$driver_version_update"
    if [ "$release_type" == "nightly" ]; then
        new_image_version="quay.io/dell/container-storage-modules/csi-unity:nightly"
    else
        new_image_version="quay.io/dell/container-storage-modules/csi-unity:v$driver_version_update"
    fi

    # Update configVersion and image
    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' \
        "$sample_version_folder/storage_csm_unity_v$driver_sample_file_suffix.yaml"
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' \
        "$sample_version_folder/storage_csm_unity_v$driver_sample_file_suffix.yaml"

    yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' \
        "$sample_version_folder/minimal-samples/unity_v$driver_sample_file_suffix.yaml"
    yq -i '.spec.driver.common.image = "'"$new_image_version"'"' \
        "$sample_version_folder/minimal-samples/unity_v$driver_sample_file_suffix.yaml"

    # Delete old patch files
    rm -v "$sample_version_folder/storage_csm_unity_v$previous_driver_sample_file_suffix.yaml"
    rm -v "$sample_version_folder/minimal-samples/unity_v$previous_driver_sample_file_suffix.yaml"

    # Operator config patch update
    cp -a operatorconfig/driverconfig/unity/v$previous_patch_driver_version \
          operatorconfig/driverconfig/unity/v$driver_version_update
    rm -r operatorconfig/driverconfig/unity/v$previous_patch_driver_version

    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' \
        operatorconfig/driverconfig/unity/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' \
        operatorconfig/driverconfig/unity/v$driver_version_update/node.yaml

    min_upgrade_path=$(GetMinUpgradePath "$sample_version_folder")
    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' \
        operatorconfig/driverconfig/unity/v$driver_version_update/upgrade-path.yaml

    # CSV and base manifest updates
    UpdateConfigVersion csi-unity $update_config_version
    if [ "$release_type" == "nightly" ]; then
        UpdateNightlyRelatedImages csi-unity
        UpdateNightlyBaseRelatedImages csi-unity
    else
        UpdateRelatedImages csi-unity $update_config_version
        UpdateBaseRelatedImages csi-unity $update_config_version
    fi

    # Test driver config update
    cp -a tests/config/driverconfig/unity/v$previous_patch_driver_version \
          tests/config/driverconfig/unity/v$driver_version_update
    rm -r tests/config/driverconfig/unity/v$previous_patch_driver_version

    yq eval -i 'with(select(.spec.template.spec.containers[5].name == "driver"); .spec.template.spec.containers[5].image = "'"$new_image_version"'")' \
        tests/config/driverconfig/unity/v$driver_version_update/controller.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "driver"); .spec.template.spec.containers[0].image = "'"$new_image_version"'")' \
        tests/config/driverconfig/unity/v$driver_version_update/node.yaml

    yq -i '.minUpgradePath = "'"v$min_upgrade_path"'"' \
        tests/config/driverconfig/unity/v$driver_version_update/upgrade-path.yaml

    # Update e2e test sample versions
    for f in $(find tests/e2e/testfiles -type f -name "storage_csm_unity*"); do
        yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' "$f"
    done
    for f in $(find tests/e2e/testfiles/minimal-testfiles -type f -name "storage_csm_unity*"); do
        yq -i '.spec.driver.configVersion = "'"$update_config_version"'"' "$f"
    done

    # Patch manager.yaml and operator.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[5].value = "'"$new_image_version"'")' config/manager/manager.yaml
    yq eval -i 'with(select(.spec.template.spec.containers[0].name == "manager"); .spec.template.spec.containers[0].env[5].value = "'"$new_image_version"'")' deploy/operator.yaml
} 

UpdateBadDriver() {
    driver_version_update=$1
    # Extract the values of major_version, minor_version, and patch_version from the input string
    major_version=${driver_version_update%%.*}
    minor_version=${driver_version_update#*.}
    minor_version=${minor_version%%.*}
    patch_version=${driver_version_update##*.}

    previous_minor_version=$((minor_version - 1))
    previous_major_driver_version="$major_version.$previous_minor_version.$patch_version"

    cp -a tests/config/driverconfig/badDriver/v$previous_major_driver_version/. tests/config/driverconfig/badDriver/v$driver_version_update
    delete_minor_version=$((minor_version - 3))
    driver_delete_version="$major_version.$delete_minor_version.$patch_version"
    rm -r tests/config/driverconfig/badDriver/v$driver_delete_version
}

if [ "$driver_update_type" == "major" ]; then
    if [ ! -z "$powerscale_version" -a "$powerscale_version" != " " ]; then
        UpdateMajorPowerflexDriver $powerflex_version $release_type
        UpdateMajorPowermaxDriver $powermax_version $release_type
        UpdateMajorPowerscaleDriver $powerscale_version $release_type
        UpdateMajorPowerstoreDriver $powerstore_version $release_type
        UpdateMajorUnityDriver $unity_version $release_type
        UpdateBadDriver $powerscale_version
    else
        echo "invalid powerscale_version"
        exit 1
    fi
elif [ "$driver_update_type" == "patch" ]; then
    if [ ! -z "$powerflex_version" -a "$powerflex_version" != " " ]; then
        UpdatePatchPowerflexDriver $powerflex_version $release_type
    fi
    if [ ! -z "$powermax_version" -a "$powermax_version" != " " ]; then
        UpdatePatchPowermaxDriver $powermax_version $release_type
    fi
    if [ ! -z "$powerscale_version" -a "$powerscale_version" != " " ]; then
        UpdatePatchPowerscaleDriver $powerscale_version $release_type
    fi
    if [ ! -z "$powerstore_version" -a "$powerstore_version" != " " ]; then
        UpdatePatchPowerstoreDriver $powerstore_version $release_type
    fi
    if [ ! -z "$unity_version" -a "$unity_version" != " " ]; then
        UpdatePatchUnityDriver $unity_version $release_type
    fi
else
    echo "invalid driver_update_type"
    exit 1
fi
