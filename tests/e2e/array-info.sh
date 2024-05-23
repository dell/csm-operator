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

# USER MODIFICATION REQUIRED: must supply address of Authorization Proxy Server
# Since this e2e exposes the Proxy Server via NodePort, you can use a cluster node IP
export PROXY_HOST="csm-authorization.com"

# The following are Authorization Proxy Server specific:
# Must supply storage array details
# Storage type examples - powerscale, powerflex, powermax
export STORAGE_TYPE="powerscale"
export END_POINT="1.1.1.1:8080"
export SYSTEM_ID="Isilon-System-Name"
export STORAGE_USER="username"
export STORAGE_PASSWORD="password"
export STORAGE_POOL="/ifs/data/csi"
# Must specify and manually create driver namespace
export DRIVER_NAMESPACE="isilon"

# The following are for creating PFlex secret/storage class
# do not include "https://" in the endpoint
export PFLEX_USER="username"
export PFLEX_PASS="password"
export PFLEX_SYSTEMID="00990099ddcc"
export PFLEX_ENDPOINT="10.1.1.1"
export PFLEX_MDM="10.0.0.1,10.0.0.2"
export PFLEX_AUTH_ENDPOINT="localhost:9401"  
export PFLEX_POOL="pool1"

# The following are for creating PScale secret/storage class
# do not include "https://" in the endpoint
export PSCALE_CLUSTER="Isilon-System-Name"
export PSCALE_USER="username"
export PSCALE_PASS="password"
export PSCALE_ENDPOINT="1.1.1.1"
export PSCALE_AUTH_ENDPOINT="localhost"
export PSCALE_AUTH_PORT="9400"

# The following are for testing AM
export VOL_NS=wordpress
export RES_NS=res-wordpress
export AM_NS=test-vxflexos
export BACKEND_STORAGE_URL="10.0.0.4:32000"
export BUCKET_NAME="my-bucket"
export ALT_BUCKET_NAME="alt-bucket" 
# Be sure to escape / with \
export AM_CONTROLLER_IMAGE="dellemc/csm-application-mobility-controller:nightly"
export AM_PLUGIN_IMAGE="dellemc/csm-application-mobility-velero-plugin:nightly"
