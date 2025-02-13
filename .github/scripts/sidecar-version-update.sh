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

# This script updates the sidecar versions across CSM-operator files
attacher_ver="$1"
snapshotter_ver="$2"
provisioner_ver="$3"
registrar_ver="$4"
health_monitor_ver="$5"
metadata_retriever_ver="$6"
resizer_ver="$7"
sdc_ver="$8"

cd $GITHUB_WORKSPACE

for sidecar in {attacher,provisioner,snapshotter,registrar,resizer,external-health-monitor,sdc,metadata-retriever}
  do
    echo "Updating sidecar version for -->> $sidecar"
    old_sidecar_ver=$(cat operatorconfig/driverconfig/common/default.yaml | grep $sidecar | egrep 'registry.k8s.io|quay.io'  | awk '{print $2}')
    old_sidecar_sub_string=$(echo $old_sidecar_ver | awk -F':' '{print $1}')

    files_to_be_modified=$(grep -rl $old_sidecar_ver)

       for file in $files_to_be_modified
         do
            if [ $sidecar == 'attacher' ]; then
              sed -i "s|${old_sidecar_ver}|${old_sidecar_sub_string}:${attacher_ver}|g" $file
            elif [ $sidecar == 'provisioner' ]; then
              sed -i "s|${old_sidecar_ver}|${old_sidecar_sub_string}:${provisioner_ver}|g" $file
            elif [ $sidecar == 'snapshotter' ]; then
              sed -i "s|${old_sidecar_ver}|${old_sidecar_sub_string}:${snapshotter_ver}|g" $file
            elif [ $sidecar == 'registrar' ]; then
              sed -i "s|${old_sidecar_ver}|${old_sidecar_sub_string}:${registrar_ver}|g" $file
            elif [ $sidecar == 'resizer' ]; then
              sed -i "s|${old_sidecar_ver}|${old_sidecar_sub_string}:${resizer_ver}|g" $file
            elif [ $sidecar == 'external-health-monitor' ]; then
             sed -i "s|${old_sidecar_ver}|${old_sidecar_sub_string}:${health_monitor_ver}|g" $file
            elif [ $sidecar == 'sdc' ]; then
              sed -i "s|${old_sidecar_ver}|${old_sidecar_sub_string}:${sdc_ver}|g" $file
            elif [ $sidecar == 'metadata-retriever' ]; then
              sed -i "s|${old_sidecar_ver}|${old_sidecar_sub_string}:${metadata_retriever_ver}|g" $file
            fi
         done
         echo "Done updating sidecar version for -->> $sidecar"

  done
  echo "SIDECAR Version update complete"