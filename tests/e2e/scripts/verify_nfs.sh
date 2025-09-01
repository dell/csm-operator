#!/bin/bash

# Copyright © 2020-2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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
#

# Copyright © 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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

check_sshpass() {
    if ! command -v sshpass &> /dev/null; then
        echo "sshpass is not installed, install before running this script"
        exit 1
    fi
}

# get all worker node names and IPs in the cluster
get_worker_nodes() {
  kubectl get nodes -A -o wide | grep -v -E 'master|control-plane' | awk 'NR>1 { print $6 }'
}

check_service() {
    failed=0
    expected_status=$1
    service=$2

    echo "Checking service status: $service"

    for node in $(get_worker_nodes); do
        sshpass -f node_credential ssh root@$node "systemctl status $service" > serverFileResponse.txt

        if ! grep -q "$expected_status" serverFileResponse.txt; then
            echo "Service $service is not running on $node. Install it before running these tests."
            failed=1
        fi
    done

    rm serverFileResponse.txt

    return $failed
}

verify_nfs_server() {
    echo "Assuming that 'node_credential' contains the nodes and their credentials. For more information, see tests/README.md."

    nfs_mountd_service="nfs-mountd.service"
    nfs_mountd_response="active (running)"
    check_service "$nfs_mountd_response" "$nfs_mountd_service"
    ret=$?
    if [ $ret -eq 1 ]; then
        echo "NFS Server is not running on all nodes. Install it before running these tests."
        exit 1
    fi


    nfs_server_service="nfs-server.service"
    nfs_server_response="active"
    check_service "$nfs_server_response" "$nfs_server_service"
    ret=$?
    if [ $ret -eq 1 ]; then
        echo "NFS Server is not running on all nodes. Install it before running these tests."
        exit 1
    fi

    echo "NFS Server is running on all nodes."
}

check_sshpass
verify_nfs_server
