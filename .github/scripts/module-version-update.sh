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

auth_v2_samples_format=${auth_v2//./}

input_csm_ver="$1"
update_flag="$2"
input_csm_ver="$(echo -e "${input_csm_ver}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
update_flag="$(echo -e "${update_flag}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"

# Step-1:- <<<< Updating observability module version >>>>
if [ -n $obs_ver ]; then
      cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/observability
      if [ -d $obs_ver ]; then
          if [[ "$update_flag" == "tag" ]]; then
             echo "Observability --> update flag received is --> tag"
             echo "Updating tags for Observability module"
             # Updating tags to latest observability module config
             cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/observability/$obs_ver
             sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerflex.*|quay.io/dell/container-storage-modules/csm-metrics-powerflex:${pflex_matrics}|g" karavi-metrics-powerflex.yaml
             sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powermax.*|quay.io/dell/container-storage-modules/csm-metrics-powermax:${pmax_matrics}|g" karavi-metrics-powermax.yaml
             sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerscale.*|quay.io/dell/container-storage-modules/csm-metrics-powerscale:${pscale_matrics}|g" karavi-metrics-powerscale.yaml

             cd $GITHUB_WORKSPACE/pkg/modules
             if [ -n "$otel_col" ]; then
             sed -i "s|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector.*|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector:${otel_col}\"|g" observability.go
             fi

             # Updating observability nightly images with the actual tags for the release
             cd $GITHUB_WORKSPACE/samples
             for input_file in {storage_csm_powerflex_${pflex_driver_ver}.yaml,storage_csm_powermax_${pmax_driver_ver}.yaml,storage_csm_powerscale_${pscale_driver_ver}.yaml,storage_csm_powerstore_${pstore_driver_ver}.yaml};
               do
               if [ -n "$otel_col" ]; then
                sed -i "s|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector.*|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector:${otel_col}|g" $input_file
               fi
               if [[ "$input_file" == "storage_v1_csm_powerscale.yaml" ]]; then
                sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerscale.*|quay.io/dell/container-storage-modules/csm-metrics-powerscale:${pscale_matrics}|g" $input_file
               fi
               if [[ "$input_file" == "storage_v1_csm_powermax.yaml" ]]; then
                  sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powermax.*|quay.io/dell/container-storage-modules/csm-metrics-powermax:${pmax_matrics}|g" $input_file
               fi
               if [[ "$input_file" == "storage_v1_csm_powerflex.yaml" ]]; then
                sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerflex.*|quay.io/dell/container-storage-modules/csm-metrics-powerflex:${pflex_matrics}|g" $input_file
              fi
             done
             echo "Latest release tags are updated to Observability module"
          else
          echo "Observability Module config directory --> $obs_ver already exists. Skipping Observability module version update"
          fi
      else
          echo "Observability Module config directory --> $obs_ver doesn't exists. Proceeding to update Observability module version"

          # observability moduleconfig update to latest
          cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/observability/
          dir_to_del=$(ls -d */ | sort -V | head -1)
          dir_to_copy=$(ls -d */ | sort -V | tail -1)
          cp -r $dir_to_copy $obs_ver
          rm -rf $dir_to_del

          cd $obs_ver
          sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerflex.*|quay.io/dell/container-storage-modules/csm-metrics-powerflex:nightly|g" karavi-metrics-powerflex.yaml
          sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powermax.*|quay.io/dell/container-storage-modules/csm-metrics-powermax:nightly|g" karavi-metrics-powermax.yaml
          sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerscale.*|quay.io/dell/container-storage-modules/csm-metrics-powerscale:nightly|g" karavi-metrics-powerscale.yaml

          cd $GITHUB_WORKSPACE/bundle/manifests
          input_file="dell-csm-operator.clusterserviceversion.yaml"

          search_string_4="  - image: quay.io/dell/container-storage-modules/csm-metrics-powerscale"
          search_string_5="\"image\": \"quay.io/dell/container-storage-modules/csm-metrics-powerscale"
          search_string_6="value: quay.io/dell/container-storage-modules/csm-metrics-powerscale"
          new_line_4="   - image: quay.io/dell/container-storage-modules/csm-metrics-powerscale:$pscale_matrics"
          new_line_5="                   \"image\": \"quay.io/dell/container-storage-modules/csm-metrics-powerscale:${pscale_matrics}\","
          new_line_6="                       value: quay.io/dell/container-storage-modules/csm-metrics-powerscale:$pscale_matrics"

          search_string_7="  - image: quay.io/dell/container-storage-modules/csm-metrics-powermax"
          search_string_8="\"image\": \"quay.io/dell/container-storage-modules/csm-metrics-powermax"
          search_string_9="value: quay.io/dell/container-storage-modules/csm-metrics-powermax"
          new_line_7="   - image: quay.io/dell/container-storage-modules/csm-metrics-powermax:$pmax_matrics"
          new_line_8="                   \"image\": \"quay.io/dell/container-storage-modules/csm-metrics-powermax:${pmax_matrics}\","
          new_line_9="                       value: quay.io/dell/container-storage-modules/csm-metrics-powermax:$pmax_matrics"

          search_string_10="  - image: quay.io/dell/container-storage-modules/csm-metrics-powerflex"
          search_string_11="\"image\": \"quay.io/dell/container-storage-modules/csm-metrics-powerflex"
          search_string_12="value: quay.io/dell/container-storage-modules/csm-metrics-powerflex"
          new_line_10="   - image: quay.io/dell/container-storage-modules/csm-metrics-powerflex:$pflex_matrics"
          new_line_11="                   \"image\": \"quay.io/dell/container-storage-modules/csm-metrics-powerflex:${pflex_matrics}\","
          new_line_12="                       value: quay.io/dell/container-storage-modules/csm-metrics-powerflex:$pflex_matrics"

          search_string_13="  - image: docker.io/otel/opentelemetry-collector"
          search_string_14="\"image\": \"docker.io/otel/opentelemetry-collector"
          search_string_15="value: docker.io/otel/opentelemetry-collector"
          new_line_13="   - image: docker.io/otel/opentelemetry-collector:$otel_col"
          new_line_14="                   \"image\": \"docker.io/otel/opentelemetry-collector:${otel_col}\","
          new_line_15="                       value: docker.io/otel/opentelemetry-collector:$otel_col"

          line_number=0
          while IFS= read -r line; do
             line_number=$((line_number + 1))
             if [[ "$line" == *"$search_string_4"* ]]; then
                 sed -i "$line_number c\ $new_line_4" "$input_file"
             fi
             if [[ "$line" == *"$search_string_5"* ]]; then
                 sed -i "$line_number c\ $new_line_5" "$input_file"
             fi
             if [[ "$line" == *"$search_string_6"* ]]; then
                 sed -i "$line_number c\ $new_line_6" "$input_file"
             fi
             if [[ "$line" == *"$search_string_7"* ]]; then
                 sed -i "$line_number c\ $new_line_7" "$input_file"
             fi
             if [[ "$line" == *"$search_string_8"* ]]; then
                 sed -i "$line_number c\ $new_line_8" "$input_file"
             fi
             if [[ "$line" == *"$search_string_9"* ]]; then
                 sed -i "$line_number c\ $new_line_9" "$input_file"
             fi
             if [[ "$line" == *"$search_string_10"* ]]; then
                 sed -i "$line_number c\ $new_line_10" "$input_file"
             fi
             if [[ "$line" == *"$search_string_11"* ]]; then
                 sed -i "$line_number c\ $new_line_11" "$input_file"
             fi
             if [[ "$line" == *"$search_string_12"* ]]; then
                 sed -i "$line_number c\ $new_line_12" "$input_file"
             fi
             if [[ "$line" == *"$search_string_13"* ]]; then
                 sed -i "$line_number c\ $new_line_13" "$input_file"
             fi
             if [[ "$line" == *"$search_string_14"* ]]; then
                 sed -i "$line_number c\ $new_line_14" "$input_file"
             fi
             if [[ "$line" == *"$search_string_15"* ]]; then
                 sed -i "$line_number c\ $new_line_15" "$input_file"
             fi
          done <"$input_file"

          search_string1="quay.io/dell/container-storage-modules/csm-metrics-"
          search_string2="metrics-"
          newver="$obs_ver"
          line_number=0
          tmp_line=0
          while IFS= read -r line
             do
               line_number=$((line_number+1))
               if [[ "$line" == *"$search_string1"* ]] && [[ "$line" != *"value"*  ]] ; then
                  IFS= read -r next_line
                    if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+4+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                           if [[ "$data" == *"configVersion"* ]]; then
                              sed -i "$line_number_tmp s/.*/                \"configVersion\": \"$newver\",/" "$input_file"
                           fi
                    fi
               fi
             done < "$input_file"

          cd $GITHUB_WORKSPACE/config/manager
          file_to_be_updated="manager.yaml"
           sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerscale.*|quay.io/dell/container-storage-modules/csm-metrics-powerscale:${pscale_matrics}|g" $file_to_be_updated
           sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powermax.*|quay.io/dell/container-storage-modules/csm-metrics-powermax:${pmax_matrics}|g" $file_to_be_updated
           sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerflex.*|quay.io/dell/container-storage-modules/csm-metrics-powerflex:${pflex_matrics}|g" $file_to_be_updated
           if [ -n "$otel_col" ]; then
              sed -i "s|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector.*|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector:${otel_col}|g" $file_to_be_updated
           fi

          cd $GITHUB_WORKSPACE/config/manifests/bases
          file_to_be_updated="dell-csm-operator.clusterserviceversion.yaml"
          sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerscale.*|quay.io/dell/container-storage-modules/csm-metrics-powerscale:${pscale_matrics}|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powermax.*|quay.io/dell/container-storage-modules/csm-metrics-powermax:${pmax_matrics}|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerflex.*|quay.io/dell/container-storage-modules/csm-metrics-powerflex:${pflex_matrics}|g" $file_to_be_updated
          if [ -n "$otel_col" ]; then
             sed -i "s|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector.*|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector:${otel_col}|g" $file_to_be_updated
          fi

          cd $GITHUB_WORKSPACE/config/samples
          for input_file in {storage_v1_csm_powerflex.yaml,storage_v1_csm_powermax.yaml,storage_v1_csm_powerscale.yaml};
          do
             search_string1="name: observability"
             search_string2="enabled"
             newver="$obs_ver"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+4+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
             done < "$input_file"

             if [ -n "$otel_col" ]; then
                sed -i "s|docker.io/otel/opentelemetry-collector.*|docker.io/otel/opentelemetry-collector:${otel_col}|g" $input_file
             fi
             if [[ "$input_file" == "storage_v1_csm_powerscale.yaml" ]]; then
                sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerscale.*|quay.io/dell/container-storage-modules/csm-metrics-powerscale:${pscale_matrics}|g" $input_file
             fi
             if [[ "$input_file" == "storage_v1_csm_powermax.yaml" ]]; then
                sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powermax.*|quay.io/dell/container-storage-modules/csm-metrics-powermax:${pmax_matrics}|g" $input_file
             fi
             if [[ "$input_file" == "storage_v1_csm_powerflex.yaml" ]]; then
                sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerflex.*|quay.io/dell/container-storage-modules/csm-metrics-powerflex:${pflex_matrics}|g" $input_file
             fi
          done

          cd $GITHUB_WORKSPACE/deploy
          file_to_be_updated="operator.yaml"
          sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerscale.*|quay.io/dell/container-storage-modules/csm-metrics-powerscale:${pscale_matrics}|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powermax.*|quay.io/dell/container-storage-modules/csm-metrics-powermax:${pmax_matrics}|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerflex.*|quay.io/dell/container-storage-modules/csm-metrics-powerflex:${pflex_matrics}|g" $file_to_be_updated
          if [ -n "$otel_col" ]; then
             sed -i "s|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector.*|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector:${otel_col}|g" $file_to_be_updated
          fi

          # Update detailed samples
          cd $GITHUB_WORKSPACE/samples
          for input_file in {storage_csm_powerflex_${pflex_driver_ver}.yaml,storage_csm_powermax_${pmax_driver_ver}.yaml,storage_csm_powerscale_${pscale_driver_ver}.yaml,storage_csm_powerstore_${pstore_driver_ver}.yaml};
          do
             search_string1="name: observability"
             search_string2="enabled"
             newver="$obs_ver"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+4+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
             done < "$input_file"

             if [ -n "$otel_col" ]; then
                sed -i "s|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector.*|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector:${otel_col}|g" $input_file
             fi
             if [[ "$input_file" == "storage_v1_csm_powerscale.yaml" ]]; then
                sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerscale.*|quay.io/dell/container-storage-modules/csm-metrics-powerscale:nightly|g" $input_file
             fi
             if [[ "$input_file" == "storage_v1_csm_powermax.yaml" ]]; then
                sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powermax.*|quay.io/dell/container-storage-modules/csm-metrics-powermax:nightly|g" $input_file
             fi
             if [[ "$input_file" == "storage_v1_csm_powerflex.yaml" ]]; then
                sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerflex.*|quay.io/dell/container-storage-modules/csm-metrics-powerflex:nightly|g" $input_file
             fi
          done

          # Update testfiles
          cd $GITHUB_WORKSPACE/tests/e2e/testfiles
          for input_file in storage_csm* ;
          do
             search_string1="name: observability"
             search_string2="enabled"
             newver="$obs_ver"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+3+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
             done < "$input_file"
             if [ -n "$otel_col" ]; then
                sed -i "s|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector.*|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector:${otel_col}|g" $input_file
             fi
             if [[ "$input_file" == "storage_v1_csm_powerscale.yaml" ]]; then
                sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerscale.*|quay.io/dell/container-storage-modules/csm-metrics-powerscale:nightly|g" $input_file
             fi
             if [[ "$input_file" == "storage_v1_csm_powermax.yaml" ]]; then
                sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powermax.*|quay.io/dell/container-storage-modules/csm-metrics-powermax:nightly|g" $input_file
             fi
             if [[ "$input_file" == "storage_v1_csm_powerflex.yaml" ]]; then
                sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerflex.*|quay.io/dell/container-storage-modules/csm-metrics-powerflex:nightly|g" $input_file
             fi
          done

          # Update pkg/modules/testdata
          cd $GITHUB_WORKSPACE/pkg/modules/testdata
          for input_file in cr_* ;
          do
             search_string1="name: observability"
             search_string2="enabled"
             newver="$obs_ver"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+3+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
             done < "$input_file"
             if [ -n "$otel_col" ]; then
                sed -i "s|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector.*|ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector:${otel_col}|g" $input_file
             fi
             if [[ "$input_file" == "storage_v1_csm_powerscale.yaml" ]]; then
                sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerscale.*|quay.io/dell/container-storage-modules/csm-metrics-powerscale:nightly|g" $input_file
             fi
             if [[ "$input_file" == "storage_v1_csm_powermax.yaml" ]]; then
                sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powermax.*|quay.io/dell/container-storage-modules/csm-metrics-powermax:nightly|g" $input_file
             fi
             if [[ "$input_file" == "storage_v1_csm_powerflex.yaml" ]]; then
                sed -i "s|quay.io/dell/container-storage-modules/csm-metrics-powerflex.*|quay.io/dell/container-storage-modules/csm-metrics-powerflex:nightly|g" $input_file
             fi
          done
          echo "Observability Module config --> $obs_ver updated successfully"
      fi
fi
# <<<< Observability module update complete >>>>

##################################################################################
# Step-2:- <<<< Updating Resiliency module versions >>>>

if [ -n "$res_ver" ]; then
      cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/resiliency
      if [ -d "$res_ver" ]; then
          if [[ "$update_flag" == "tag" ]]; then
             echo "Resiliency --> update flag received is --> tag"
             echo "Updating tags for Resiliency module"
             cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/resiliency/$res_ver
             for input_file in container-* ; do
             sed -i "s|quay.io/dell/container-storage-modules/podmon.*|quay.io/dell/container-storage-modules/podmon:$res_ver|g" $input_file
             done

             cd $GITHUB_WORKSPACE/samples
             for input_file in {storage_csm_powerflex_${pflex_driver_ver}.yaml,storage_csm_powermax_${pmax_driver_ver}.yaml,storage_csm_powerscale_${pscale_driver_ver}.yaml,storage_csm_powerstore_${pstore_driver_ver}.yaml}; do
             sed -i "s|quay.io/dell/container-storage-modules/podmon.*|quay.io/dell/container-storage-modules/podmon:$res_ver|g" $input_file
             done
             echo "Latest release tags are updated to Resiliency module"
          else
          echo "Resiliency Module config directory --> $res_ver already exists. Skipping Resiliency module version update"
          fi
      else
          echo "Resiliency Module config directory --> $res_ver doesn't exists. Proceeding to update Resiliency module version"

          # resiliency moduleconfig update to latest
          cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/resiliency/
          dir_to_del=$(ls -d */ | sort -V | head -1)
          dir_to_copy=$(ls -d */ | sort -V | tail -1)
          cp -r $dir_to_copy $res_ver
          rm -rf $dir_to_del

          # update podmon version to latest
          cd $res_ver
          for input_file in container-* ; do
          sed -i "s|quay.io/dell/container-storage-modules/podmon.*|quay.io/dell/container-storage-modules/podmon:nightly|g" $input_file
          done

          # update bundle/manifests
          cd $GITHUB_WORKSPACE/bundle/manifests
          input_file="dell-csm-operator.clusterserviceversion.yaml"
          search_string_1="  - image: quay.io/dell/container-storage-modules/podmon"
          search_string_2="\"image\": \"quay.io/dell/container-storage-modules/podmon"
          search_string_3="value: quay.io/dell/container-storage-modules/podmon"
          new_line_1="   - image: quay.io/dell/container-storage-modules/podmon:$res_ver"
          new_line_2="                   \"image\": \"quay.io/dell/container-storage-modules/podmon:${res_ver}\","
          new_line_3="                       value: quay.io/dell/container-storage-modules/podmon:$res_ver"
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

           search_string1="quay.io/dell/container-storage-modules/podmon"
           search_string2="imagePullPolicy"
           newver="$res_ver"
           line_number=0
           tmp_line=0
           while IFS= read -r line
              do
                line_number=$((line_number+1))
                if [[ "$line" == *"$search_string1"* ]] ; then
                   IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                         line_number_tmp=$((line_number+5+tmp_line))
                         tmp_line=$((tmp_line+1))
                         data=$(sed -n "${line_number_tmp}p" "$input_file")
                            if [[ "$data" == *"configVersion"* ]]; then
                               sed -i "$line_number_tmp s/.*/                \"configVersion\": \"$newver\",/" "$input_file"
                            fi
                     fi
                fi
              done < "$input_file"

           # update config/manager
          cd $GITHUB_WORKSPACE/config/manager
          file_to_be_updated="manager.yaml"
          sed -i "s|quay.io/dell/container-storage-modules/podmon.*|quay.io/dell/container-storage-modules/podmon:$res_ver|g" $file_to_be_updated

          # update config/manifests/bases
          cd $GITHUB_WORKSPACE/config/manifests/bases
          file_to_be_updated="dell-csm-operator.clusterserviceversion.yaml"
          sed -i "s|quay.io/dell/container-storage-modules/podmon.*|quay.io/dell/container-storage-modules/podmon:$res_ver|g" $file_to_be_updated

          # update config/samples
          cd $GITHUB_WORKSPACE/config/samples
          for input_file in {storage_v1_csm_powerflex.yaml,storage_v1_csm_powermax.yaml,storage_v1_csm_powerscale.yaml,storage_v1_csm_powerstore.yaml};
          do
             search_string1="name: resiliency"
             search_string2="enabled"
             newver="$res_ver"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+7+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          sed -i "s|quay.io/dell/container-storage-modules/podmon.*|quay.io/dell/container-storage-modules/podmon:$res_ver|g" $input_file
          done

          # update deploy
          cd $GITHUB_WORKSPACE/deploy
          sed -i "s|quay.io/dell/container-storage-modules/podmon.*|quay.io/dell/container-storage-modules/podmon:$res_ver|g" operator.yaml

          # update pkg/modules/testdata
          cd $GITHUB_WORKSPACE/pkg/modules/testdata
          for input_file in {cr_powerflex_resiliency.yaml,cr_powermax_resiliency.yaml,cr_powerscale_resiliency.yaml,cr_powerstore_resiliency.yaml};
          do
             search_string1="name: resiliency"
             search_string2="enabled"
             newver="$res_ver"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+7+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          sed -i "s|quay.io/dell/container-storage-modules/podmon.*|quay.io/dell/container-storage-modules/podmon:$res_ver|g" $input_file
          done

          # update samples
          cd $GITHUB_WORKSPACE/samples
          for input_file in {storage_csm_powerflex_${pflex_driver_ver}.yaml,storage_csm_powermax_${pmax_driver_ver}.yaml,storage_csm_powerscale_${pscale_driver_ver}.yaml,storage_csm_powerstore_${pstore_driver_ver}.yaml};
            do
             search_string1="name: resiliency"
             search_string2="enabled"
             newver="$res_ver"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+7+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          sed -i "s|quay.io/dell/container-storage-modules/podmon.*|quay.io/dell/container-storage-modules/podmon:nightly|g" $input_file
          done

          # update tests/e2e/testfiles
          cd $GITHUB_WORKSPACE/tests/e2e/testfiles
          for input_file in storage_csm* ;
          do
             search_string1="name: resiliency"
             search_string2="enabled"
             newver="$res_ver"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+7+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          sed -i "s|quay.io/dell/container-storage-modules/podmon.*|quay.io/dell/container-storage-modules/podmon:nightly|g" $input_file
          done
          echo "Resiliency Module config --> $res_ver updated successfully"
      fi
fi
# <<<< Resiliency module update complete >>>>

##################################################################################
# Step-3:- <<<< Updating Replication module versions >>>>
if [ -n "$rep_ver" ]; then
      cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/replication
      if [ -d "$rep_ver" ]; then
          if [[ "$update_flag" == "tag" ]]; then
             echo "Replication --> update flag received is --> tag"
             echo "Updating tags for Replication module"
             cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/replication/$rep_ver
             sed -i "s|quay.io/dell/container-storage-modules/dell-csi-replicator.*|quay.io/dell/container-storage-modules/dell-csi-replicator:$dell_csi_replicator|g" container.yaml
             sed -i "s|quay.io/dell/container-storage-modules/dell-replication-controller.*|quay.io/dell/container-storage-modules/dell-replication-controller:$dell_replication_controller|g" controller.yaml

             cd $GITHUB_WORKSPACE/samples
             for input_file in {storage_csm_powerflex_${pflex_driver_ver}.yaml,storage_csm_powermax_${pmax_driver_ver}.yaml,storage_csm_powerscale_${pscale_driver_ver}.yaml}; do
             sed -i "s|quay.io/dell/container-storage-modules/dell-csi-replicator.*|quay.io/dell/container-storage-modules/dell-csi-replicator:$dell_csi_replicator|g" $input_file
             sed -i "s|quay.io/dell/container-storage-modules/dell-replication-controller.*|quay.io/dell/container-storage-modules/dell-replication-controller:$dell_replication_controller|g" $input_file
             done

             echo "Latest release tags are updated to Replication module"
          else
          echo "Replication Module config directory --> $rep_ver already exists. Skipping Replication module version update"
          fi
      else
          echo "Replication Module config directory --> $rep_ver doesn't exists. Proceeding to update Replication module version"

          # replication moduleconfig update to latest
          cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/replication/
          dir_to_del=$(ls -d */ | sort -V | head -1)
          dir_to_copy=$(ls -d */ | sort -V | tail -1)
          cp -r $dir_to_copy $rep_ver
          rm -rf $dir_to_del

          # update replication controller and sidecar version to latest
          cd $rep_ver
          sed -i "s|quay.io/dell/container-storage-modules/dell-csi-replicator.*|quay.io/dell/container-storage-modules/dell-csi-replicator:nightly|g" container.yaml
          sed -i "s|quay.io/dell/container-storage-modules/dell-replication-controller.*|quay.io/dell/container-storage-modules/dell-replication-controller:nightly|g" controller.yaml

          # update bundle/manifests
          cd $GITHUB_WORKSPACE/bundle/manifests
          input_file="dell-csm-operator.clusterserviceversion.yaml"
          search_string_1="  - image: quay.io/dell/container-storage-modules/dell-csi-replicator"
          search_string_2="\"image\": \"quay.io/dell/container-storage-modules/dell-csi-replicator"
          search_string_3="value: quay.io/dell/container-storage-modules/dell-csi-replicator"
          new_line_1="   - image: quay.io/dell/container-storage-modules/dell-csi-replicator:$dell_csi_replicator"
          new_line_2="                   \"image\": \"quay.io/dell/container-storage-modules/dell-csi-replicator:${dell_csi_replicator}\","
          new_line_3="                       value: quay.io/dell/container-storage-modules/dell-csi-replicator:$dell_csi_replicator"

          search_string_4="  - image: quay.io/dell/container-storage-modules/dell-replication-controller"
          search_string_5="\"image\": \"quay.io/dell/container-storage-modules/dell-replication-controller"
          search_string_6="value: quay.io/dell/container-storage-modules/dell-replication-controller"
          new_line_4="   - image: quay.io/dell/container-storage-modules/dell-replication-controller:$dell_replication_controller"
          new_line_5="                   \"image\": \"quay.io/dell/container-storage-modules/dell-replication-controller:${dell_replication_controller}\","
          new_line_6="                       value: quay.io/dell/container-storage-modules/dell-replication-controller:$dell_replication_controller"

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
             if [[ "$line" == *"$search_string_4"* ]]; then
                 sed -i "$line_number c\ $new_line_4" "$input_file"
             fi
             if [[ "$line" == *"$search_string_5"* ]]; then
                 sed -i "$line_number c\ $new_line_5" "$input_file"
             fi
             if [[ "$line" == *"$search_string_6"* ]]; then
                 sed -i "$line_number c\ $new_line_6" "$input_file"
             fi
          done <"$input_file"

           search_string1="quay.io/dell/container-storage-modules/dell-replication-controller"
           search_string2="dell-replication-controller-manager"
           newver="$rep_ver"
           line_number=0
           tmp_line=0
           while IFS= read -r line
              do
                line_number=$((line_number+1))
                if [[ "$line" == *"$search_string1"* ]] ; then
                   IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                         line_number_tmp=$((line_number+4+tmp_line))
                         tmp_line=$((tmp_line+1))
                         data=$(sed -n "${line_number_tmp}p" "$input_file")
                            if [[ "$data" == *"configVersion"* ]]; then
                               sed -i "$line_number_tmp s/.*/                \"configVersion\": \"$newver\",/" "$input_file"
                            fi
                     fi
                fi
              done < "$input_file"

           # update config/manager
          cd $GITHUB_WORKSPACE/config/manager
          file_to_be_updated="manager.yaml"
          sed -i "s|quay.io/dell/container-storage-modules/dell-csi-replicator.*|quay.io/dell/container-storage-modules/dell-csi-replicator:$dell_csi_replicator|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/dell-replication-controller.*|quay.io/dell/container-storage-modules/dell-replication-controller:$dell_replication_controller|g" $file_to_be_updated

          # update config/manifests/bases
          cd $GITHUB_WORKSPACE/config/manifests/bases
          file_to_be_updated="dell-csm-operator.clusterserviceversion.yaml"
          sed -i "s|quay.io/dell/container-storage-modules/dell-csi-replicator.*|quay.io/dell/container-storage-modules/dell-csi-replicator:$dell_csi_replicator|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/dell-replication-controller.*|quay.io/dell/container-storage-modules/dell-replication-controller:$dell_replication_controller|g" $file_to_be_updated

          # update config/samples
          cd $GITHUB_WORKSPACE/config/samples
          for input_file in {storage_v1_csm_powerflex.yaml,storage_v1_csm_powermax.yaml,storage_v1_csm_powerscale.yaml,storage_v1_csm_powerstore.yaml};
          do
             search_string1="name: replication"
             search_string2="enabled"
             newver="$rep_ver"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+7+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          sed -i "s|quay.io/dell/container-storage-modules/dell-csi-replicator.*|quay.io/dell/container-storage-modules/dell-csi-replicator:$dell_csi_replicator|g" $input_file
          sed -i "s|quay.io/dell/container-storage-modules/dell-replication-controller.*|quay.io/dell/container-storage-modules/dell-replication-controller:$dell_replication_controller|g" $input_file
          done

          # update deploy
          cd $GITHUB_WORKSPACE/deploy
          file_to_be_updated="operator.yaml"
          sed -i "s|quay.io/dell/container-storage-modules/dell-csi-replicator.*|quay.io/dell/container-storage-modules/dell-csi-replicator:$dell_csi_replicator|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/dell-replication-controller.*|quay.io/dell/container-storage-modules/dell-replication-controller:$dell_replication_controller|g" $file_to_be_updated

          # update pkg/modules/testdata
          cd $GITHUB_WORKSPACE/pkg/modules/testdata
          for input_file in {cr_powerflex_replica.yaml,cr_powermax_replica.yaml,cr_powerscale_replica.yaml};
          do
          sed -i "s|quay.io/dell/container-storage-modules/dell-csi-replicator.*|quay.io/dell/container-storage-modules/dell-csi-replicator:$dell_csi_replicator|g" $input_file
          sed -i "s|quay.io/dell/container-storage-modules/dell-replication-controller.*|quay.io/dell/container-storage-modules/dell-replication-controller:$dell_replication_controller|g" $input_file
          done

          # update samples
          cd $GITHUB_WORKSPACE/samples
          for input_file in {storage_csm_powerflex_${pflex_driver_ver}.yaml,storage_csm_powermax_${pmax_driver_ver}.yaml,storage_csm_powerscale_${pscale_driver_ver}.yaml};
            do
             search_string1="name: replication"
             search_string2="enabled"
             newver="$rep_ver"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+7+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
           sed -i "s|quay.io/dell/container-storage-modules/dell-csi-replicator.*|quay.io/dell/container-storage-modules/dell-csi-replicator:nightly|g" $input_file
           sed -i "s|quay.io/dell/container-storage-modules/dell-replication-controller.*|quay.io/dell/container-storage-modules/dell-replication-controller:nightly|g" $input_file
          done

          # update tests/e2e/testfiles
          cd $GITHUB_WORKSPACE/tests/e2e/testfiles
          for input_file in storage_csm* ;
          do
             search_string1="name: replication"
             search_string2="enabled"
             newver="$rep_ver"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+7+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
           sed -i "s|quay.io/dell/container-storage-modules/dell-csi-replicator.*|quay.io/dell/container-storage-modules/dell-csi-replicator:nightly|g" $input_file
           sed -i "s|quay.io/dell/container-storage-modules/dell-replication-controller.*|quay.io/dell/container-storage-modules/dell-replication-controller:nightly|g" $input_file
          done

          echo "Replication Module config --> $rep_ver updated successfully"
      fi
fi
# <<<< Replication module update complete >>>>

##################################################################################
# Step-4:- <<<< Updating Reverseproxy module versions >>>>
if [ -n "$revproxy_ver" ]; then
      cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/csireverseproxy
      if [ -d "$revproxy_ver" ]; then
          if [[ "$update_flag" == "tag" ]]; then
             echo "Reverseproxy --> update flag received is --> tag"
             echo "Updating tags for Reverseproxy module"
             cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/csireverseproxy/$revproxy_ver
             sed -i "s|quay.io/dell/container-storage-modules/csipowermax-reverseproxy.*|quay.io/dell/container-storage-modules/csipowermax-reverseproxy:$revproxy_ver|g" container.yaml

             cd $GITHUB_WORKSPACE/samples
             sed -i "s|quay.io/dell/container-storage-modules/csipowermax-reverseproxy.*|quay.io/dell/container-storage-modules/csipowermax-reverseproxy:$revproxy_ver|g" storage_csm_powermax_${pmax_driver_ver}.yaml

             echo "Latest release tags are updated to Reverseproxy module"
          else
          echo "csireverseproxy Module config directory --> $revproxy_ver already exists. Skipping csireverseproxy module version update"
          fi
      else
          echo "csireverseproxy Module config directory --> $revproxy_ver doesn't exists. Proceeding to update csireverseproxy module version"

          # csireverseproxy moduleconfig update to latest
          cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/csireverseproxy/
          dir_to_del=$(ls -d */ | sort -V | head -1)
          dir_to_copy=$(ls -d */ | sort -V | tail -1)
          cp -r $dir_to_copy $revproxy_ver
          rm -rf $dir_to_del

          # update csireverseproxy version to latest
          cd $revproxy_ver
          sed -i "s|quay.io/dell/container-storage-modules/csipowermax-reverseproxy.*|quay.io/dell/container-storage-modules/csipowermax-reverseproxy:nightly" container.yaml

          # update bundle/manifests
          cd $GITHUB_WORKSPACE/bundle/manifests
          input_file="dell-csm-operator.clusterserviceversion.yaml"
          search_string_1="  - image: quay.io/dell/container-storage-modules/csipowermax-reverseproxy"
          search_string_2="\"image\": \"quay.io/dell/container-storage-modules/csipowermax-reverseproxy"
          search_string_3="value: quay.io/dell/container-storage-modules/csipowermax-reverseproxy"
          new_line_1="   - image: quay.io/dell/container-storage-modules/csipowermax-reverseproxy:$revproxy_ver"
          new_line_2="                   \"image\": \"quay.io/dell/container-storage-modules/csipowermax-reverseproxy:${revproxy_ver}\","
          new_line_3="                       value: quay.io/dell/container-storage-modules/csipowermax-reverseproxy:$revproxy_ver"
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

           search_string1="quay.io/dell/container-storage-modules/csipowermax-reverseproxy"
           search_string2="csipowermax-reverseproxy"
           newver="$revproxy_ver"
           line_number=0
           tmp_line=0
           while IFS= read -r line
              do
                line_number=$((line_number+1))
                if [[ "$line" == *"$search_string1"* ]] ; then
                   IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                         line_number_tmp=$((line_number+4+tmp_line))
                         tmp_line=$((tmp_line+1))
                         data=$(sed -n "${line_number_tmp}p" "$input_file")
                            if [[ "$data" == *"configVersion"* ]]; then
                               sed -i "$line_number_tmp s/.*/                \"configVersion\": \"$newver\",/" "$input_file"
                            fi
                     fi
                fi
              done < "$input_file"

           # update config/manager
          cd $GITHUB_WORKSPACE/config/manager
          file_to_be_updated="manager.yaml"
          sed -i "s|quay.io/dell/container-storage-modules/csipowermax-reverseproxy.*|quay.io/dell/container-storage-modules/csipowermax-reverseproxy:$revproxy_ver|g" $file_to_be_updated

          # update config/manifests/bases
          cd $GITHUB_WORKSPACE/config/manifests/bases
          file_to_be_updated="dell-csm-operator.clusterserviceversion.yaml"
          sed -i "s|quay.io/dell/container-storage-modules/csipowermax-reverseproxy.*|quay.io/dell/container-storage-modules/csipowermax-reverseproxy:$revproxy_ver|g" $file_to_be_updated

          # update config/samples
          cd $GITHUB_WORKSPACE/config/samples
          input_file=storage_v1_csm_powermax.yaml
          search_string1="name: csireverseproxy"
          search_string2="configVersion"
          newver="$revproxy_ver"
          line_number=0
          tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+1+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          sed -i "s|quay.io/dell/container-storage-modules/csipowermax-reverseproxy.*|quay.io/dell/container-storage-modules/csipowermax-reverseproxy:$revproxy_ver|g" $input_file

          # update deploy
          cd $GITHUB_WORKSPACE/deploy
          sed -i "s|quay.io/dell/container-storage-modules/csipowermax-reverseproxy.*|quay.io/dell/container-storage-modules/csipowermax-reverseproxy:$revproxy_ver|g" operator.yaml


          # update pkg/modules/testdata
          cd $GITHUB_WORKSPACE/pkg/modules/testdata
          for input_file in cr_powermax_* ;
          do
             search_string1='name: "csireverseproxy"'
             search_string2="enabled"
             newver="$revproxy_ver"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+3+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          sed -i "s|quay.io/dell/container-storage-modules/csipowermax-reverseproxy.*|quay.io/dell/container-storage-modules/csipowermax-reverseproxy:$revproxy_ver|g" $input_file
          done

          # update samples
          cd $GITHUB_WORKSPACE/samples
          input_file=storage_csm_powermax_${pmax_driver_ver}.yaml
          search_string1="name: csireverseproxy"
          search_string2="configVersion"
          newver="$revproxy_ver"
          line_number=0
          tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+1+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          sed -i "s|quay.io/dell/container-storage-modules/csipowermax-reverseproxy.*|quay.io/dell/container-storage-modules/csipowermax-reverseproxy:nightly|g" $input_file

          # update tests/e2e/testfiles
          cd $GITHUB_WORKSPACE/tests/e2e/testfiles
          for input_file in storage_csm_powermax* ;
          do
             search_string1="name: csireverseproxy"
             search_string2="configVersion"
             newver="$revproxy_ver"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+1+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          sed -i "s|quay.io/dell/container-storage-modules/csipowermax-reverseproxy.*|quay.io/dell/container-storage-modules/csipowermax-reverseproxy:nightly|g" $input_file
          done

          echo "Reverseproxy Module config --> $revproxy_ver updated successfully"
      fi
fi
# <<<< Reverseproxy module update complete >>>>

##################################################################################
# Step-5:- <<<< Updating Authorization module versions >>>>
if [ -n "$auth_v2" ]; then
      cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/authorization
      if [ -d "$auth_v2" ]; then
          if [[ "$update_flag" == "tag" ]]; then
             echo "Authorization --> update flag received is --> tag"
             echo "Updating tags for Authorization module"
             cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/authorization/$auth_v2
             sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-sidecar.*|quay.io/dell/container-storage-modules/csm-authorization-sidecar:$auth_v2|g" container.yaml

             cd $GITHUB_WORKSPACE/samples
             for input_file in {storage_csm_powermax_${pmax_driver_ver}.yaml,storage_csm_powerscale_${pscale_driver_ver}.yaml,storage_csm_powerflex_${pflex_driver_ver}.yaml}; do
             sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-sidecar.*|quay.io/dell/container-storage-modules/csm-authorization-sidecar:$auth_v2|g" $input_file
             done

             cd $GITHUB_WORKSPACE/samples/authorization
             # TODO: Not updating v1 as its going to be deprecated in csm-v1.15.0
             input_file=csm_authorization_proxy_server_$auth_v2_samples_format.yaml
             sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-proxy.*|quay.io/dell/container-storage-modules/csm-authorization-proxy:$auth_v2|g" $input_file
             sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-tenant.*|quay.io/dell/container-storage-modules/csm-authorization-tenant:$auth_v2|g" $input_file
             sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-role.*|quay.io/dell/container-storage-modules/csm-authorization-role:$auth_v2|g" $input_file
             sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-storage.*|quay.io/dell/container-storage-modules/csm-authorization-storage:$auth_v2|g" $input_file
             sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-controller.*|quay.io/dell/container-storage-modules/csm-authorization-controller:$auth_v2|g" $input_file

             echo "Latest release tags are updated to Authorization module"
          else
          echo "Authorization v2 Module config directory --> $auth_v2 already exists. Skipping Authorization v2 module version update"
          fi
      else
          echo "Authorization v2 Module config directory --> $auth_v2 doesn't exists. Proceeding to update Authorization v2 module version"

          # authorization v2 moduleconfig update to latest
          cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/authorization/
          dir_to_del=$(ls -d */ | sort -V | head -1)
          dir_to_copy=$(ls -d */ | sort -V | tail -1)
          cp -r $dir_to_copy $auth_v2
          rm -rf $dir_to_del

          # update authorization v2 version to latest
          cd $auth_v2
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-sidecar.*|quay.io/dell/container-storage-modules/csm-authorization-sidecar:nightly|g" container.yaml

          # update bundle/manifests
          cd $GITHUB_WORKSPACE/bundle/manifests
          input_file="dell-csm-operator.clusterserviceversion.yaml"
          search_string_1="  - image: quay.io/dell/container-storage-modules/csm-authorization-sidecar"
          search_string_2="\"image\": \"quay.io/dell/container-storage-modules/csm-authorization-sidecar"
          search_string_3="value: quay.io/dell/container-storage-modules/csm-authorization-sidecar"
          new_line_1="   - image: quay.io/dell/container-storage-modules/csm-authorization-sidecar:$auth_v2"
          new_line_2="                   \"image\": \"quay.io/dell/container-storage-modules/csm-authorization-sidecar:${auth_v2}\","
          new_line_3="                       value: quay.io/dell/container-storage-modules/csm-authorization-sidecar:$auth_v2"

          search_string_4="  - image: quay.io/dell/container-storage-modules/csm-authorization-proxy"
          search_string_5="value: quay.io/dell/container-storage-modules/csm-authorization-proxy"
          new_line_4="   - image: quay.io/dell/container-storage-modules/csm-authorization-proxy:$auth_v2"
          new_line_5="                       value: quay.io/dell/container-storage-modules/csm-authorization-proxy:$auth_v2"

          search_string_6="  - image: quay.io/dell/container-storage-modules/csm-authorization-tenant"
          search_string_7="value: quay.io/dell/container-storage-modules/csm-authorization-tenant"
          new_line_6="   - image: quay.io/dell/container-storage-modules/csm-authorization-tenant:$auth_v2"
          new_line_7="                       value: quay.io/dell/container-storage-modules/csm-authorization-tenant:$auth_v2"

          search_string_8="  - image: quay.io/dell/container-storage-modules/csm-authorization-role"
          search_string_9="value: quay.io/dell/container-storage-modules/csm-authorization-role"
          new_line_8="   - image: quay.io/dell/container-storage-modules/csm-authorization-role:$auth_v2"
          new_line_9="                       value: quay.io/dell/container-storage-modules/csm-authorization-role:$auth_v2"

          search_string_10="  - image: quay.io/dell/container-storage-modules/csm-authorization-storage"
          search_string_11="value: quay.io/dell/container-storage-modules/csm-authorization-storage"
          new_line_10="   - image: quay.io/dell/container-storage-modules/csm-authorization-storage:$auth_v2"
          new_line_11="                       value: quay.io/dell/container-storage-modules/csm-authorization-storage:$auth_v2"

          search_string_12="  - image: quay.io/dell/container-storage-modules/csm-authorization-controller"
          search_string_13="value: quay.io/dell/container-storage-modules/csm-authorization-controller"
          new_line_12="   - image: quay.io/dell/container-storage-modules/csm-authorization-controller:$auth_v2"
          new_line_13="                       value: quay.io/dell/container-storage-modules/csm-authorization-controller:$auth_v2"

          search_string_14="\"authorizationController\": \"quay.io/dell/container-storage-modules/csm-authorization-controller"
          new_line_14="                   \"authorizationController\": \"quay.io/dell/container-storage-modules/csm-authorization-controller:${auth_v2}\","

          search_string_15="\"proxyService\": \"quay.io/dell/container-storage-modules/csm-authorization-proxy"
          new_line_15="                   \"proxyService\": \"quay.io/dell/container-storage-modules/csm-authorization-proxy:${auth_v2}\","

          search_string_16="\"roleService\": \"quay.io/dell/container-storage-modules/csm-authorization-role"
          new_line_16="                    \"roleService\": \"quay.io/dell/container-storage-modules/csm-authorization-role:${auth_v2}\","

          search_string_16="\"storageService\": \"quay.io/dell/container-storage-modules/csm-authorization-storage"
          new_line_16="                   \"storageService\": \"quay.io/dell/container-storage-modules/csm-authorization-storage:${auth_v2}\","

          search_string_17="\"tenantService\": \"quay.io/dell/container-storage-modules/csm-authorization-tenant"
          new_line_17="                   \"tenantService\": \"quay.io/dell/container-storage-modules/csm-authorization-tenant:${auth_v2}\","


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
             if [[ "$line" == *"$search_string_4"* ]]; then
                 sed -i "$line_number c\ $new_line_4" "$input_file"
             fi
             if [[ "$line" == *"$search_string_5"* ]]; then
                 sed -i "$line_number c\ $new_line_5" "$input_file"
             fi
             if [[ "$line" == *"$search_string_6"* ]]; then
                 sed -i "$line_number c\ $new_line_6" "$input_file"
             fi
             if [[ "$line" == *"$search_string_7"* ]]; then
                 sed -i "$line_number c\ $new_line_7" "$input_file"
             fi
             if [[ "$line" == *"$search_string_8"* ]]; then
                 sed -i "$line_number c\ $new_line_8" "$input_file"
             fi
             if [[ "$line" == *"$search_string_9"* ]]; then
                 sed -i "$line_number c\ $new_line_9" "$input_file"
             fi
             if [[ "$line" == *"$search_string_10"* ]]; then
                 sed -i "$line_number c\ $new_line_10" "$input_file"
             fi
             if [[ "$line" == *"$search_string_11"* ]]; then
                 sed -i "$line_number c\ $new_line_11" "$input_file"
             fi
             if [[ "$line" == *"$search_string_12"* ]]; then
                 sed -i "$line_number c\ $new_line_12" "$input_file"
             fi
             if [[ "$line" == *"$search_string_13"* ]]; then
                 sed -i "$line_number c\ $new_line_13" "$input_file"
             fi
             if [[ "$line" == *"$search_string_14"* ]]; then
                 sed -i "$line_number c\ $new_line_14" "$input_file"
             fi
             if [[ "$line" == *"$search_string_15"* ]]; then
                 sed -i "$line_number c\ $new_line_15" "$input_file"
             fi
             if [[ "$line" == *"$search_string_16"* ]]; then
                 sed -i "$line_number c\ $new_line_16" "$input_file"
             fi
             if [[ "$line" == *"$search_string_17"* ]]; then
                 sed -i "$line_number c\ $new_line_17" "$input_file"
             fi
          done <"$input_file"

           search_string1="quay.io/dell/container-storage-modules/csm-authorization-sidecar"
           search_string2="karavi-authorization-proxy"
           newver="$auth_v2"
           line_number=0
           tmp_line=0
           while IFS= read -r line
              do
                line_number=$((line_number+1))
                if [[ "$line" == *"$search_string1"* ]] ; then
                   IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                         line_number_tmp=$((line_number+4+tmp_line))
                         tmp_line=$((tmp_line+1))
                         data=$(sed -n "${line_number_tmp}p" "$input_file")
                            if [[ "$data" == *"configVersion"* ]]; then
                               sed -i "$line_number_tmp s/.*/                \"configVersion\": \"$newver\",/" "$input_file"
                            fi
                     fi
                fi
              done < "$input_file"

           # update config/manager
          cd $GITHUB_WORKSPACE/config/manager
          file_to_be_updated="manager.yaml"
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-sidecar.*|quay.io/dell/container-storage-modules/csm-authorization-sidecar:$auth_v2|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-proxy.*|quay.io/dell/container-storage-modules/csm-authorization-proxy:$auth_v2|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-tenant.*|quay.io/dell/container-storage-modules/csm-authorization-tenant:$auth_v2|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-role.*|quay.io/dell/container-storage-modules/csm-authorization-role:$auth_v2|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-storage.*|quay.io/dell/container-storage-modules/csm-authorization-storage:$auth_v2|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-controller.*|quay.io/dell/container-storage-modules/csm-authorization-controller:$auth_v2|g" $file_to_be_updated

          # update config/manifests/bases
          cd $GITHUB_WORKSPACE/config/manifests/bases
          file_to_be_updated="dell-csm-operator.clusterserviceversion.yaml"
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-sidecar.*|quay.io/dell/container-storage-modules/csm-authorization-sidecar:$auth_v2|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-proxy.*|quay.io/dell/container-storage-modules/csm-authorization-proxy:$auth_v2|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-tenant.*|quay.io/dell/container-storage-modules/csm-authorization-tenant:$auth_v2|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-role.*|quay.io/dell/container-storage-modules/csm-authorization-role:$auth_v2|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-storage.*|quay.io/dell/container-storage-modules/csm-authorization-storage:$auth_v2|g" $file_to_be_updated
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-controller.*|quay.io/dell/container-storage-modules/csm-authorization-controller:$auth_v2|g" $file_to_be_updated

          # update config/samples
          cd $GITHUB_WORKSPACE/config/samples

          input_file=storage_v1_csm_authorization_v2.yaml
          search_string1="name: authorization-proxy-server"
          search_string2="enable"
          newver="$auth_v2"
          line_number=0
          tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+3+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-proxy.*|quay.io/dell/container-storage-modules/csm-authorization-proxy:$auth_v2|g" $input_file
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-tenant.*|quay.io/dell/container-storage-modules/csm-authorization-tenant:$auth_v2|g" $input_file
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-role.*|quay.io/dell/container-storage-modules/csm-authorization-role:$auth_v2|g" $input_file
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-storage.*|quay.io/dell/container-storage-modules/csm-authorization-storage:$auth_v2|g" $input_file
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-controller.*|quay.io/dell/container-storage-modules/csm-authorization-controller:$auth_v2|g" $input_file

          input_file=storage_v1_csm_powerflex.yaml
          search_string1="name: authorization"
          search_string2="enabled"
          newver="$auth_v2"
          line_number=0
          tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+5+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-sidecar.*|quay.io/dell/container-storage-modules/csm-authorization-sidecar:$auth_v2|g" $input_file

      for input_file in {storage_v1_csm_powermax.yaml,storage_v1_csm_powerscale.yaml}; do
          search_string1="name: authorization"
          search_string2="enable"
          newver="$auth_v2"
          line_number=0
          tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+4+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-sidecar.*|quay.io/dell/container-storage-modules/csm-authorization-sidecar:$auth_v2|g" $input_file
      done

          # update deploy
          cd $GITHUB_WORKSPACE/deploy
          input_file="operator.yaml"
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-proxy.*|quay.io/dell/container-storage-modules/csm-authorization-proxy:$auth_v2|g" $input_file
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-tenant.*|quay.io/dell/container-storage-modules/csm-authorization-tenant:$auth_v2|g" $input_file
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-role.*|quay.io/dell/container-storage-modules/csm-authorization-role:$auth_v2|g" $input_file
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-storage.*|quay.io/dell/container-storage-modules/csm-authorization-storage:$auth_v2|g" $input_file
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-controller.*|quay.io/dell/container-storage-modules/csm-authorization-controller:$auth_v2|g" $input_file

          # update pkg/modules/testdata
          cd $GITHUB_WORKSPACE/pkg/modules/testdata
          # TODO: Not updating auth-v1 as its going to be deprecated. Also, don't see auth-v2 latest updates here. Skipping... Update if required

          # update samples
          cd $GITHUB_WORKSPACE/samples
          for input_file in {storage_csm_powermax_${pmax_driver_ver}.yaml,storage_csm_powerscale_${pscale_driver_ver}.yaml};
            do
             search_string1="name: authorization"
             search_string2="enable"
             newver="$auth_v2"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+4+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-sidecar.*|quay.io/dell/container-storage-modules/csm-authorization-sidecar:nightly|g" $input_file
          done

             input_file=storage_csm_powerflex_${pflex_driver_ver}.yaml
             search_string1="name: authorization"
             search_string2="enabled"
             newver="$auth_v2"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+5+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-sidecar.*|quay.io/dell/container-storage-modules/csm-authorization-sidecar:nightly|g" $input_file

          cd $GITHUB_WORKSPACE/samples/minimal-samples
          for input_file in {powerflex_${pflex_driver_ver}.yaml,powermax_${pmax_driver_ver}.yaml,powerscale_${pscale_driver_ver}.yaml};
            do
             search_string1="name: authorization"
             search_string2="enable"
             newver="$auth_v2"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+4+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          done

          cd $GITHUB_WORKSPACE/samples/authorization
          # TODO: Not updating v1 as its going to be deprecated in csm-v1.15.0
          input_file=csm_authorization_proxy_server_$auth_v2_samples_format.yaml
          sed -i "s|configVersion: v.*|configVersion: $auth_v2|g" $input_file
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-proxy.*|quay.io/dell/container-storage-modules/csm-authorization-proxy:nightly|g" $input_file
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-tenant.*|quay.io/dell/container-storage-modules/csm-authorization-tenant:nightly|g" $input_file
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-role.*|quay.io/dell/container-storage-modules/csm-authorization-role:nightly|g" $input_file
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-storage.*|quay.io/dell/container-storage-modules/csm-authorization-storage:nightly|g" $input_file
          sed -i "s|quay.io/dell/container-storage-modules/csm-authorization-controller.*|quay.io/dell/container-storage-modules/csm-authorization-controller:nightly|g" $input_file

          # update tests/e2e/testfiles
          cd $GITHUB_WORKSPACE/tests/e2e/testfiles/authorization-templates
          for input_file in {storage_csm_authorization_v2_multiple_vaults.yaml,storage_csm_authorization_v2_proxy_server.yaml,storage_csm_authorization_v2_proxy_server_default_redis.yaml} ;
          do
             search_string1="name: authorization-proxy-server"
             search_string2="enable"
             newver="$auth_v2"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+3+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          done

          cd $GITHUB_WORKSPACE/tests/e2e/testfiles/minimal-testfiles
          # TODO: Not updating v1 as its going to be deprecated in csm-v1.15.0
          # update storage_csm_powerflex_auth.yaml
             input_file=storage_csm_powerflex_auth.yaml
             search_string1="name: authorization"
             search_string2="enable"
             newver="$auth_v2"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+3+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"

          # update storage_csm_powermax_reverseproxy_authorization_v2.yaml
             input_file=storage_csm_powermax_reverseproxy_authorization_v2.yaml
             search_string1="name: authorization"
             search_string2="enabled"
             newver="$auth_v2"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+2+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"

          # update storage_csm_powerscale_auth2.0.yaml
             input_file=storage_csm_powerscale_auth2.0.yaml
             search_string1="name: authorization"
             search_string2="enable"
             newver="$auth_v2"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+4+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"


          cd $GITHUB_WORKSPACE/tests/e2e/testfiles
          # TODO: Not updating v1 as its going to be deprecated in csm-v1.15.0
          for input_file in {storage_csm_powerflex_alt_vals_1.yaml,storage_csm_powerflex_alt_vals_2.yaml,storage_csm_powerflex_alt_vals_3.yaml,storage_csm_powerflex_alt_vals_4.yaml,storage_csm_powerflex_downgrade.yaml,storage_csm_powerflex_health_monitor.yaml,storage_csm_powerflex_no_sdc.yaml,storage_csm_powermax_authorization.yaml,storage_csm_powermax_observability_authorization.yaml,storage_csm_powermax_secret_auth_v2.yaml,storage_csm_powerscale.yaml,storage_csm_powerscale_alt_vals_1.yaml,storage_csm_powerscale_alt_vals_2.yaml,storage_csm_powerscale_alt_vals_3.yaml,storage_csm_powerscale_auth.yaml,storage_csm_powerscale_health_monitor.yaml,storage_csm_powerscale_observability.yaml,storage_csm_powerscale_observability_auth.yaml,storage_csm_powerscale_observability_top_custom_cert.yaml,storage_csm_powerscale_observability_val1.yaml,storage_csm_powerscale_observability_val2.yaml,storage_csm_powerscale_replica.yaml,storage_csm_powerflex_alt_vals_1.yaml,storage_csm_powerflex_alt_vals_2.yaml,storage_csm_powerflex_alt_vals_3.yaml,storage_csm_powerflex_alt_vals_4.yaml,storage_csm_powerflex_downgrade.yaml,storage_csm_powerflex_health_monitor.yaml,storage_csm_powerflex_no_sdc.yaml,storage_csm_powermax_authorization.yaml,storage_csm_powermax_observability_authorization.yaml,storage_csm_powermax_secret_auth_v2.yaml,storage_csm_powerscale.yaml,storage_csm_powerscale_alt_vals_1.yaml,storage_csm_powerscale_alt_vals_2.yaml,storage_csm_powerscale_alt_vals_3.yaml,storage_csm_powerscale_auth.yaml,storage_csm_powerscale_health_monitor.yaml,storage_csm_powerscale_observability.yaml,storage_csm_powerscale_observability_auth.yaml,storage_csm_powerscale_observability_top_custom_cert.yaml,storage_csm_powerscale_observability_val1.yaml,storage_csm_powerscale_observability_val2.yaml,storage_csm_powerscale_replica.yaml} ;
          do
             search_string1="name: authorization"
             search_string2="enable"
             newver="$auth_v2"
             line_number=0
             tmp_line=0
             while IFS= read -r line
                do
                  line_number=$((line_number+1))
                  if [[ "$line" == *"$search_string1"* ]] ; then
                     IFS= read -r next_line
                     if [[ "$next_line" == *"$search_string2"* ]]; then
                        line_number_tmp=$((line_number+3+tmp_line))
                        tmp_line=$((tmp_line+1))
                        data=$(sed -n "${line_number_tmp}p" "$input_file")
                        if [[ "$data" == *"configVersion"* ]]; then
                           sed -i "$line_number_tmp s/.*/      configVersion: $newver/" "$input_file"
                        fi
                     fi
                  fi
                done < "$input_file"
          done
          echo "Authorization v2 Module config --> $auth_v2 updated successfully"
      fi
fi
# <<<< Authorization module update complete >>>>

##################################################################################
if [[ "$update_flag" == "nightly" ]]; then
# Update all the latest module versions to version-values.yaml
cd $GITHUB_WORKSPACE/operatorconfig/moduleconfig/common
pscale_pflex_block=$(cat <<EOF
  $csm_ver:
    authorization: "$auth_v2"
    replication: "$rep_ver"
    observability: "$obs_ver"
    resiliency: "$res_ver"
EOF
)

pstore_block=$(cat <<EOF
  $csm_ver:
    resiliency: "$res_ver"
    authorization: "$auth_v2"
    observability: "$obs_ver"
EOF
)

pmax_block=$(cat <<EOF
  $csm_ver:
    csireverseproxy: "$revproxy_ver"
    authorization: "$auth_v2"
    replication: "$rep_ver"
    observability: "$obs_ver"
    resiliency: "$res_ver"
EOF
)

tmp_file="version-values.yaml.tmp"
while IFS= read -r line
do
    if [[ $line =~ powerflex: ]] || [[ $line =~ powerstore: ]]; then
        echo "$pscale_pflex_block" >> "$tmp_file"
        echo "$line" >> "$tmp_file"
    elif [[ $line =~ powermax: ]]; then
        echo "$pstore_block" >> "$tmp_file"
        echo "$line" >> "$tmp_file"
    else
        echo "$line" >> "$tmp_file"
    fi
done < "version-values.yaml"
sed -i ':a;/^\n*$/{$d;N;ba;}' "$tmp_file"
echo "$pmax_block" >> "$tmp_file"
mv "$tmp_file" "version-values.yaml"
fi

echo "<<< ------- Module version update complete ------- >>>"



