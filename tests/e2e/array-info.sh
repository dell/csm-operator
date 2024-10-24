# Copyright © 2022-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# Must specify and manually create driver namespace
# USER MODIFICATION REQUIRED: must supply address of Authorization Proxy Server
# Since this e2e exposes the Proxy Server via NodePort, you can use a cluster node IP
export PROXY_HOST="csm-authorization.com"
export DELLCTL="/usr/local/bin/dellctl"
# The following are for creating PFlex secret/storage class
# do not include "https://" in the endpoint
export PFLEX_USER="username"
export PFLEX_PASS="password"
export PFLEX_SYSTEMID="systemID"
export PFLEX_ENDPOINT="10.1.1.1"
export PFLEX_MDM="10.0.0.1,10.0.0.2"
export PFLEX_NAS="none"
export PFLEX_AUTH_ENDPOINT="localhost:9401"
# The following are Authorization Proxy Server specific for powerflex:
export PFLEX_POOL="pool1"
export PFLEX_STORAGE="powerflex"
export PFLEX_VAULT_STORAGE_PATH="storage\/powerflex" # escape / with \
export PFLEX_QUOTA="10GB"
export PFLEX_ROLE="csmrole-powerflex"
export PFLEX_TENANT="csmtenant-powerflex"
export PFLEX_TENANT_PREFIX="tn1"
# The following are for creating PScale secret/storage class
# do not include "https://" in the endpoint
export PSCALE_CLUSTER="Isilon-System-Name"
export PSCALE_USER="username"
export PSCALE_PASS="password"
export PSCALE_ENDPOINT="1.1.1.1"
export PSCALE_PORT="8080"
export PSCALE_AUTH_ENDPOINT="localhost"
export PSCALE_AUTH_PORT="9400"
# The following are Authorization Proxy Server specific for powerscale:
export PSCALE_POOL_V1="ifs/data/csi"
export PSCALE_POOL_V2="\/ifs\/data\/csi" # escape / with \
export PSCALE_STORAGE="powerscale"
export PSCALE_VAULT_STORAGE_PATH="storage\/powerscale" # escape / with \
export PSCALE_QUOTA="0GB"
export PSCALE_ROLE="csmrole-powerscale"
export PSCALE_TENANT="csmtenant-powerscale"
export PSCALE_TENANT_PREFIX="tn1"
# The following are for creating Powermax secret/storage class
export PMAX_SYSTEMID="Pmax-System-Id"
export PMAX_ENDPOINT="10.0.0.1:8443"
export PMAX_AUTH_ENDPOINT="localhost"
export PMAX_USER="username"
export PMAX_PASS="password"
export PMAX_USER_ENCODED="username"
export PMAX_PASS_ENCODED="password"
export PMAX_SERVICE_LEVEL="Bronze"
# The following are Authorization Proxy Server specific for powermax:
export PMAX_POOL_V1="SRP_1"
export PMAX_POOL_V2="SRP_1"
export PMAX_STORAGE="powermax"
export PMAX_VAULT_STORAGE_PATH="storage\/powermax" # escape / with \
export PMAX_QUOTA="0GB"
export PMAX_ROLE="csmrole-powermax"
export PMAX_TENANT="csmtenant-powermax"
export PMAX_TENANT_PREFIX="tn1"
export PMAX_PORTGROUPS=""
export PMAX_PROTOCOL=""
export PMAX_ARRAYS="000000000000,000000000001"
# The following is PowerStore specific:
export PSTORE_USER="username"
export PSTORE_PASS="password"
export PSTORE_GLOBALID="myglobalpstoreid"
# ip only, do not include /api/rest at end
export PSTORE_ENDPOINT="1.1.1.1"
# The following is Unity specific:
export UNITY_USER="username"
export UNITY_PASS="password"
export UNITY_ARRAYID="myglobalunityid"
export UNITY_ENDPOINT="1.1.1.1"
export UNITY_NAS="mynas"
export UNITY_POOL="mypool"
# The following are for testing AM
export VOL_NS=ns1
export RES_NS=res-ns1
export AM_NS=test-vxflexos
export BACKEND_STORAGE_URL="10.0.0.4:32000"
export BUCKET_NAME="my-bucket"
export ALT_BUCKET_NAME="alt-bucket"
# Be sure to escape / with \
export AM_CONTROLLER_IMAGE="quay.io/dell/container-storage-modules/csm-application-mobility-controller:nightly"
export AM_PLUGIN_IMAGE="quay.io/dell/container-storage-modules/csm-application-mobility-velero-plugin:nightly"
