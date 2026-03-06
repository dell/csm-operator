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

# This script updates the operator versions in the following files
#  1. Dockerfile
#  2. kustomization.yaml
#  3. controllers/csm_controller.go
#  4. operator.yaml
#  5. docker.mk
#  6. dell-csm-operator.clusterserviceversion.yaml
#  7. manager.yaml

# Usage: bash ./.github/scripts/operator-version-update.sh --operator_version "v1.11.0" --csm_version "v1.16.0"

cd "$GITHUB_WORKSPACE"

# Initialize variables with default values
operator_version=""
csm_version=""

# Set options for the getopt command
options=$(getopt -o "" -l "operator_version:,csm_version:" -- "$@")
if [ $? -ne 0 ]; then
    echo "Invalid arguments."
    exit 1
fi
eval set -- "$options"

# Read the named argument values
while [ $# -gt 0 ]; do
    case "$1" in
    --operator_version)
        operator_version="$2"
        shift
        ;;
    --csm_version)
        csm_version="$2"
        shift
        ;;
    --) shift ;;
    esac
    shift
done

op_version_wv="${operator_version:1}"
csm_ver_wv="${csm_version:1}"

echo "Latest version -->> $operator_version"
echo "CSM Version -->> $csm_version"

echo "Updating csm-operator"
sed -i "s/release=\"[^\"]*\"/release=\"${csm_ver_wv}\"/g" Dockerfile
sed -i "s/version=\"[^\"]*\"/version=\"${op_version_wv}\"/g" Dockerfile
echo "Dockerfile updated"

sed -i "s/newTag: .*/newTag: ${operator_version}/" config/install/kustomization.yaml
sed -i "s/newTag: .*/newTag: ${operator_version}/" config/manager/kustomization.yaml
echo "kustomization.yaml updated"

sed -i "s/CSMVersion: .*/CSMVersion: ${csm_version}/g" config/manager/manager.yaml
sed -i "s/dell-csm-operator:.*/dell-csm-operator:${operator_version}/g" config/manager/manager.yaml

sed -i "s/CSMVersion = .*/CSMVersion = \"${csm_version}\"/g" controllers/csm_controller.go
echo "csm_controller.go updated"

sed -i "s/dell-csm-operator:.*/dell-csm-operator:${operator_version}/g" deploy/olm/operator_community.yaml
sed -i "s/dell-csm-operator:.*/dell-csm-operator:${operator_version}/g" deploy/operator.yaml
sed -i "s/CSMVersion: .*/CSMVersion: ${csm_version}/g" deploy/operator.yaml
echo "operator.yaml updated"

sed -i "s/VERSION ?=.*/VERSION ?= ${operator_version}/g" images.mk
sed -i "s/BUNDLE_VERSION ?=.*/BUNDLE_VERSION ?= ${op_version_wv}/g" images.mk

file="dell-csm-operator.clusterserviceversion.yaml"
for i_dir in {'bundle/manifests/','config/manifests/bases/'}; do
    sed -i "s/dell-csm-operator:.*/dell-csm-operator:${operator_version}/g" "$i_dir/$file"
    sed -i "s/name: dell-csm-operator.v.*/name: dell-csm-operator.${operator_version}/g" "$i_dir/$file"
    sed -i "s/CSMVersion: .*/CSMVersion: ${csm_version}/g" "$i_dir/$file"
    sed -i "s/^[[:space:]][[:space:]]version: .*/  version: ${op_version_wv}/g" "$i_dir/$file"
done
echo "dell-csm-operator.clusterserviceversion.yaml updated"

echo "OPERATOR VERSION UPDATE COMPLETE"
