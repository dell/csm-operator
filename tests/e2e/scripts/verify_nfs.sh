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

verify_nfs_server() {
    failed=0
    echo "Assuming that 'node_credential' contains the nodes and their credentials. For more information, see tests/README.md."
    nfs_server_command="systemctl status nfs-mountd.service"

    for node in $(get_worker_nodes); do
        sshpass -f node_credential ssh root@$node $nfs_server_command > serverFileResponse.txt

        if grep -q "active (running)" serverFileResponse.txt; then
            echo "NFS Server is running on $node."
        else
            echo "NFS Server is not running on $node. Install it before running these tests."
            failed=1
        fi
    done

    rm serverFileResponse.txt

    echo "Finished verifying NFS server."
    exit $failed
}

check_sshpass
verify_nfs_server
