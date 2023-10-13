# Copyright © 2022-2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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

# [Optional] ginko options for custom runs
export GINKGO_OPTS="-v"

# [Optional] Path to .kube configuration if it is not in the default location
# export KUBECONFIG=""

# Must supply path to values file if different from testfiles/values.yaml
export VALUES_FILE="testfiles/test-AM.yaml"

# USER MODIFICATION REQUIRED: must supply path to your cert-csi binary
export CERT_CSI="/root/cert-csi"

# [Optional] uncomment any modules you want to test
# export AUTHORIZATION=true
# export REPLICATION=true
# export OBSERVABILITY=true
# export AUTHORIZATIONPROXYSERVER=true
# export RESILIENCY=true
# export APPMOBILITY=true

# [Optional] namespace of operator if you deployed it to a namespace diffrent form the one below.
# export OPERATOR_NAMESPACE="dell-csm-operator"

# USER MODIFICATION REQUIRED: must supply path to your karavictl binary
# export KARAVICTL="/root/karavictl"

# The following are Authorization Proxy Server specific:
# Must supply storage array details
# Storage type examples - powerscale, powerflex, powermax
# export STORAGE_TYPE="powerscale"
# export END_POINT="1.1.1.1:8080"
# export SYSTEM_ID="xxxxxx"
# export STORAGE_USER="xxxx"
# export STORAGE_PASSWORD="xxxxx"
# export STORAGE_POOL="pool"
# Must specify and manually create driver namespace
# export DRIVER_NAMESPACE="namespace"

# The following are for creating PFlex secret/storage class
# do not include "https://" in the endpoint
export PFLEX_USER=""
export PFLEX_PASS=""
export PFLEX_SYSTEMID=""
export PFLEX_ENDPOINT=""
export PFLEX_MDM=""
export PFLEX_AUTH_ENDPOINT=""  
export PFLEX_POOL=""

# The following are for creating PScale secret/storage class
# do not include "https://" in the endpoint
export PSCALE_CLUSTER=""
export PSCALE_USER=""
export PSCALE_PASS=""
export PSCALE_ENDPOINT=""
export PSCALE_AUTH_ENDPOINT=""
export PSCALE_AUTH_PORT=""

# The following are for testing AM
export VOL_NS=wordpress
export RES_NS=res-wordpress
export AM_NS=test-vxflexos
export BACKEND_STORAGE_URL=""
export ACCESS_KEY_ID=""
export ACCESS_KEY=""
# Be sure to escape and / with \
export AM_CONTROLLER_IMAGE=""
export AM_PLUGIN_IMAGE=""
