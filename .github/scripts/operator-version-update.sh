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

op_version="$CSM_OPERATOR"
op_ver_fmt="  version: ${op_version:1}"
op_version_v="${op_version:1}"
csm_ver_v="$CSM_VERSION"
csm_ver_wv="${csm_ver_v:1}"

update_flag="$2"
update_flag="$(echo -e "${update_flag}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"

echo "Latest version -->> $op_version"
echo "CSM Version -->> $csm_ver_v"

if [[ "$update_flag" == "nightly" ]]; then
   echo "Updating csm-operator with nightly flag"
        cd $GITHUB_WORKSPACE
        sed -i "s/release=\"[^\"]*\"/release=\"${csm_ver_wv}\"/g" Dockerfile
        sed -i "s/version=\"[^\"]*\"/version=\"${op_version_v}\"/g" Dockerfile
        echo "Dockerfile updated"

        k_file="kustomization.yaml"
        for k_dir in {'config/install/','config/manager/'}; do
        cd "$GITHUB_WORKSPACE/$k_dir"
        sed -i "s/newTag: .*/newTag: ${op_version}/" $k_file
        done
        echo "kustomization.yaml updated"

        cd $GITHUB_WORKSPACE
        sed -i "s/CSMVersion: .*/CSMVersion: ${csm_ver_v}/g" config/manager/manager.yaml
        sed -i "s/dell-csm-operator:.*/dell-csm-operator:${op_version}/g" config/manager/manager.yaml

        cd "$GITHUB_WORKSPACE"
        sed -i "s/CSMVersion = .*/CSMVersion = \"${csm_ver_v}\"/g" controllers/csm_controller.go
        echo "csm_controller.go updated"

        sed -i "s/dell-csm-operator:.*/dell-csm-operator:${op_version}/g" deploy/olm/operator_community.yaml
        sed -i "s/dell-csm-operator:.*/dell-csm-operator:nightly/g" deploy/operator.yaml
        sed -i "s/CSMVersion: .*/CSMVersion: ${csm_ver_v}/g" deploy/operator.yaml
        echo "operator.yaml updated"

        d_file="docker.mk"
        sed -i "s/VERSION ?=.*/VERSION ?= ${op_version}/g" $d_file
        sed -i "s/BUNDLE_VERSION ?=.*/BUNDLE_VERSION ?= ${op_version_v}/g" $d_file
        sed -i "s/example.com\/operator-catalog:.*/example.com\/operator-catalog:${op_version})./g" $d_file
        echo "docker.mk updated"

        file="dell-csm-operator.clusterserviceversion.yaml"
        for i_dir in {'bundle/manifests/','config/manifests/bases/'}; do
            cd "$GITHUB_WORKSPACE/$i_dir"
            sed -i "s/dell-csm-operator:.*/dell-csm-operator:${op_version}/g" $file
            sed -i "s/name: dell-csm-operator.v.*/name: dell-csm-operator.${op_version}/g" $file
            sed -i "s/^[[:space:]][[:space:]]version: .*/  version: ${op_version_v}/g" $file
            awk -v var="$op_ver_fmt" '/skips/{ n=NR+2 } NR==n{$0=var }1' $file > $file.tmp
            mv -f $file.tmp $file
            rm -f $file.tmp
        done
        echo "dell-csm-operator.clusterserviceversion.yaml updated"
        echo "OPERATOR VERSION UPDATE COMPLETE"
fi
if [[ "$update_flag" == "tag" ]]; then
   echo "Updating csm-operator with latest tag"
        cd "$GITHUB_WORKSPACE"
        sed -i "s/dell-csm-operator:.*/dell-csm-operator:${op_version}/g" deploy/operator.yaml
fi
