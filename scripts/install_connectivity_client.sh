# Copyright © 2024 Dell Inc. or its subsidiaries. All Rights Reserved.
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

#!/bin/bash

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
ROOTDIR="$(dirname "$SCRIPTDIR")"
CMD=""

out=$(command -v oc)
if [ $? -eq 0 ]; then
    CMD=$out
else
    out=$(command -v kubectl)
    if [ $? -eq 0 ]; then
        CMD=$out
    fi
fi

if [ -z "$CMD" ]; then
    echo "Could not find oc or kubectl program in path, install failed."
    exit 1
fi

for ns in dell-csm karavi dell-connectivity-client; do
    $CMD get ns $ns &> /dev/null
    if [ $? -ne 0 ]; then
        echo "Creating namespace $ns"
        $CMD create ns $ns
        if [ $? -ne 0 ]; then
            echo "Failed to create namespace: $ns"
            echo "Failed to install the Dell Connectivity Client."
            exit 1
        fi
    fi
done

$CMD apply -f $ROOTDIR/samples/connectivity_client_v100.yaml

echo "Dell Connectivity Client installed."
