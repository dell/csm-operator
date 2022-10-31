# Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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
DEPLOYDIR="$ROOTDIR/deploy"
source "$SCRIPTDIR"/common.bash

# find the operator namespace from operator.yaml file
NS_STRING=$(cat ${DEPLOYDIR}/operator.yaml | grep "namespace:" | head -1)
if [ -z "${NS_STRING}" ]; then
  echo "Couldn't find any target namespace in ${DEPLOYDIR}/operator.yaml"
  exit 1
fi
# find the namespace from the filtered string
NAMESPACE=$(echo $NS_STRING | cut -d ' ' -f2)
log separator
echo "Deleting the Operator Deployment"
echo
log separator
kubectl delete -f $DEPLOYDIR/operator.yaml
echo