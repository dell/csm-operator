#!/bin/bash

# Copyright 2026 DELL Inc. or its subsidiaries.
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

## Overview
# $1 should be the OCP version for the new samples
# This script does the following:
# - Grabs the example files from samples/(latest-version) and copies them to samples/ocp/(OCP_VERSION)
# - Goes out to https://github.com/redhat-openshift-ecosystem/certified-operators/blob/main/operators/dell-csm-operator-certified/
# and grabs the latest image versions of the OCP images from the dell-csm-operator-certified.clusterserviceversion.yaml
# - Then updates the latest example files in sample/ocp/(latest-ocp-version) with the related image for `registry.connect.redhat.com`

OCP_VERSION="$1"
DRIVER_VERSION="${2:-latest}"
COSI_VERSION="${3:-latest}"
SAMPLE_DIR="samples"
SAMPLE_DIR_COSI="samples/cosi"
URL="https://raw.githubusercontent.com/redhat-openshift-ecosystem/certified-operators/main/operators/dell-csm-operator-certified/$OCP_VERSION/manifests/dell-csm-operator-certified.clusterserviceversion.yaml"
declare -A image_map

##
# do_post_fix_image_map_for_naming_convention
# A few of the related images do not follow the naming convention, for these we have to fix in post.
##
#### TODO: Hopefully we can remove this func in the future once we fix the naming convention.
do_post_fix_image_map_for_naming_convention() {
    declare -A postfix_map
    postfix_map["externalhealthmonitorcontroller"]="external-health-monitor"
    postfix_map["podmon-node"]="podmon"
    postfix_map["metadataretriever"]="csi-metadata-retriever"
    postfix_map["csi-powerstore"]="powerstore"
    postfix_map["csi-vxflexos"]="powerflex"
    postfix_map["csi-isilon"]="isilon"
    postfix_map["csi-powermax"]="powermax"
    postfix_map["nginx"]="nginx-proxy"
    postfix_map["csm-authorization-proxy"]="proxy-service"
    postfix_map["csm-authorization-tenant"]="tenant-service"
    postfix_map["csm-authorization-role"]="role-service"
    postfix_map["csm-authorization-storage"]="storage-service"
    postfix_map["csm-authorization-controller"]="authorization-controller"
    postfix_map["redis-commander"]="commander"
    postfix_map["kube-mgmt"]="opa-kube-mgmt"
    postfix_map["objectstorage-provisioner-sidecar"]="objectstorage-sidecar"

    # Update the key values in the image_map to what they should be called
    for key in "${!postfix_map[@]}"; do
        original_map_value=${image_map[$key]}
        image_map["$key"]="$postfix_map["$key"]"
        image_map["${postfix_map[$key]}"]=${image_map["$key"]}
        image_map["${postfix_map[$key]}"]=$original_map_value
    done
}

##
# get_latest_images_from_certified_yaml
# Grabs the official certified ocp images from the `$OCP_VERSION/manifests/dell-csm-operator-certified.clusterserviceversion.yaml`
# Then creates the OCP image map based on the `.spec.related_images` in the .yaml
##
get_latest_images_from_certified_yaml() {

    # Download the YAML file
    if curl -s -f -o temp.yaml "$URL"; then
        echo "OCP Version: $OCP_VERSION was found"
    else
        echo "OCP Version: $OCP_VERSION was not found, please double check $URL"
        exit 1
    fi

    # Fill in the image_map with the values from `dell-csm-operator-certified` `spec.relatedImages`
    while IFS= read -r line; do
    if [[ $line =~ "image:" ]]; then
        image=$(echo "$line" | sed -E 's/image: (.*)/\1/')
    elif [[ $line =~ "name:" ]]; then
        name=$(echo "$line" | sed -E 's/name: (.*)/\1/')
        image_map[$name]=$image
        # Add in the sdc-monitor image since sdc and sdc-monitor use same image
        # Only sdc is referenced in the certified images
        if [[ $name == "sdc" ]]; then
            image_map["sdc-monitor"]=$image
        fi
    fi
    done < <(yq e '.spec.relatedImages[]' temp.yaml)

    # Remove the temporary YAML file
    rm temp.yaml
}

##
# escaped_string
# Takes in a string and modifies it with escape characters
# $1 String that needs to be escaped
##
escape_string() {
    string="$1"
    escaped_string=${string//\//\\/}
    echo "$escaped_string"
}

##
# grab_leading_spaces
# Grabs the amount of empty spaces before a string
# $1 value which will be inspected for how many leading spaces it has
##
grab_leading_spaces() {
    string="$1"
    leading_spaces=$(echo "$string" | sed -r 's/[^ ].*//')
    leading_spaces_minus_one=$(echo "$leading_spaces" | sed 's/.$//')
    echo "$leading_spaces_minus_one"
}

##
# grab_sample_files_from_version
# Grabs the latest sample file for non ocp deployements.
# Then prepares them in the ocp directory
##
grab_sample_files_from_version() {
    # Defaults to latest
    if [[ "$DRIVER_VERSION" == "latest" ]]; then 
        # Get the latest samples in the samples directory
        local latest_version_samples=$(ls $SAMPLE_DIR | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | sort -V | tail -1)
    else
        local latest_version_samples=$DRIVER_VERSION
    fi

    mkdir -p $SAMPLE_DIR/ocp/$OCP_VERSION
    
    # Check if DRIVER_VERSION is a non-patch version (like 2.15.1)
    if [[ "$DRIVER_VERSION" != "latest" ]] && [[ "$DRIVER_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        # Convert version to vXXXX format (e.g., 2.15.1 -> v2151)
        local version_pattern="v${DRIVER_VERSION//./}"
        # Convert patch version to base version (e.g., 2.15.1 -> 2.15.0)
        local base_version="${DRIVER_VERSION%.*}.0"
        # Only grab files that have the version pattern in their name
        cp $SAMPLE_DIR/v$base_version/*$version_pattern.yaml $SAMPLE_DIR/ocp/$OCP_VERSION -v || true
        # Copy the configmap
        cp $SAMPLE_DIR/v$base_version/k8s_configmap.yaml $SAMPLE_DIR/ocp/$OCP_VERSION/k8s_configmap.yaml -v || true
    else
        # Use || true to ignore the recursive error.
        # This is since we dont want any of the directories just the files in the latest sample
        cp $SAMPLE_DIR/v$latest_version_samples/* $SAMPLE_DIR/ocp/$OCP_VERSION -v || true
    fi

}

##
# grab_sample_files_from_version_cosi
# Grabs the latest sample file for cosi ocp deployements.
# Then prepares them in the ocp directory
##
grab_sample_files_from_version_cosi() {

    # Defaults to latest
    if [[ "$COSI_VERSION" == "latest" ]]; then 
        # Get the latest samples in the samples directory
        local latest_version_samples_cosi=$(ls $SAMPLE_DIR_COSI | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | sort -V | tail -1)
    else
        local latest_version_samples_cosi=$COSI_VERSION
    fi

    # Check if COSI_VERSION is a non-patch version (like 2.15.1)
    if [[ "$COSI_VERSION" != "latest" ]] && [[ "$COSI_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        # Convert version to vXXXX format (e.g., 2.15.1 -> v2151)
        local cosi_version_pattern="v${COSI_VERSION//./}"
        # Only grab files that have the version pattern in their name
        cp $SAMPLE_DIR_COSI/v$latest_version_samples_cosi/*$cosi_version_pattern.yaml $SAMPLE_DIR/ocp/$OCP_VERSION -v || true
    else
        # Use || true to ignore the recursive error.
        # This is since we dont want any of the directories just the files in the latest sample
        cp $SAMPLE_DIR_COSI/v$latest_version_samples_cosi/* $SAMPLE_DIR/ocp/$OCP_VERSION -v || true
    fi
}

##
# do_replacemnt takes in a key and file and replaces value with correlating value in image map
# $1 value which lives in old sample file which should be replaced
# $2 file where the key which needs to be replaced lives
# $3 prefix_key key related to the value in the file ie `image:` of `image: some-container-image:vX.X.X`
##
do_replacement() {
    value="$1"
    file="$2"
    prefix_key="$3"

    # Grab the `image: image_name` for the particular key in the non updated sample file
    file_val=$(cat $file | grep -m 1 -E "*image:.*$value:")
    # Validate that we dont grep an empty value
    if [[ -n "$file_val" ]]; then
        # Get the number of leading spaces before `image:` in the sample file
        leading_spaces=$(grab_leading_spaces "$file_val")
        # Add escape characters for both the file_var and the image_map var
        file_val=$(escape_string "$file_val")
        ocp_image_val=$(escape_string "${image_map[$value]}")
        # Do a replace for the `image: file_var` with `image: ocp_image_val`
        sed -i "s/$file_val/$leading_spaces $prefix_key $ocp_image_val/g" "$file" > /dev/null 
        echo "Updated $file replacing $file_val with $ocp_image_val"
    fi
}

##
# update_each_sample_file loops through all the files in the new ocp directory.
# Then preforms the replacement in each respective file.
##
update_each_sample_file() {
    # For each file and replace the image: with the corresponding ocp image
    for file in $SAMPLE_DIR/ocp/$OCP_VERSION/*; do
        for key in "${!image_map[@]}"; do
            do_replacement $key $file "image:"

            # if the image is prefixed by `- image:` instead of `image:` make the change for those images also.
            val=$(cat $file | grep -m 1 -E "*- image:.*$key:")
            if [ "$?" -eq 0 ]; then
                echo "There is still another place where $key needs to be updated"
                do_replacement $key $file "- image:"
            fi
        done
    done
}

##
# update_ocp_configmap
# Creates the ocp_configmap.yaml from the latest k8s_configmap.yaml
# - Extracts only the first (latest) version block
# - Replaces image values with OCP images from image_map
# - Adjusts metadata/data indentation to 1-space
##
update_ocp_configmap() {
    local latest_version_samples=$(ls $SAMPLE_DIR | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | sort -V | tail -1)
    local k8s_configmap="$SAMPLE_DIR/ocp/$OCP_VERSION/k8s_configmap.yaml"
    local ocp_configmap="$SAMPLE_DIR/ocp/$OCP_VERSION/ocp_configmap.yaml"

    if [ ! -f "$k8s_configmap" ]; then
        echo "Error: $k8s_configmap not found"
        exit 1
    fi

    # Extract only the first version block from versions.yaml
    # Find the start of the first version block and the start of the second version block
    local first_version_start=$(grep -n "^\s*- version:" "$k8s_configmap" | head -1 | cut -d: -f1)
    local second_version_start=$(grep -n "^\s*- version:" "$k8s_configmap" | sed -n '2p' | cut -d: -f1)
    
    if [ -n "$second_version_start" ]; then
        # Extract content from start to just before the second version
        local end_line=$((second_version_start - 1))
        head -n "$end_line" "$k8s_configmap" > "$ocp_configmap"
    else
        # If there's only one version, just copy the file
        cp "$k8s_configmap" "$ocp_configmap"
    fi

    # Clean up the k8s configmap after the transfer is done for OCP configMap
    rm $k8s_configmap

    # Replace each image value with the OCP image from image_map
    for key in "${!image_map[@]}"; do
        local ocp_image="${image_map[$key]}"
        # Match lines like '        <key>: <any-image-value>' inside the versions.yaml block
        # Use a sed pattern that matches the key at the start (after spaces) followed by ': ' and replaces the image value
        local escaped_ocp_image
        escaped_ocp_image=$(escape_string "$ocp_image")
        sed -i "s|^\([[:space:]]*${key}:[[:space:]]*\).*|\1${escaped_ocp_image}|" "$ocp_configmap" 
        echo "Updating configmap $escaped_ocp_image "
    done

    echo "Created $ocp_configmap from $k8s_configmap with OCP images (only latest version)"
}

#################### Main ####################
get_latest_images_from_certified_yaml
do_post_fix_image_map_for_naming_convention
grab_sample_files_from_version
grab_sample_files_from_version_cosi
update_ocp_configmap
update_each_sample_file
