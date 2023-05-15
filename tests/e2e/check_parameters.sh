# Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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

# This script takes in a csv file of parameter names and their expected values
# for a driver deployment and checks to make sure that the values are as expected.

if [ "$#" -ne 3 ]; then
  echo "Incorrect number of parameters provided to $0. Usage:"
  echo "$0 <csv file> <deployment namespace> <deployment name>"
  exit 1
fi

# get controller describe
controllerpod=`kubectl get pods -n $2 | grep -m 1 controller | awk '{print $1}'`
kubectl describe pod $controllerpod -n $2 > controller-describe

# get node describe
nodepod=`kubectl get pods -n $2 | grep -m 1 node | awk '{print $1}'`
kubectl describe pod $nodepod -n $2 > node-describe

# get csm describe
kubectl describe csm $3 -n $2 > csm-describe

{
  read
  while IFS=, read -r paramName grepOptions paramValue k8sResource
  do
    cat $k8sResource-describe | grep "$paramName" | grep -q $grepOptions "$paramValue"
    RET=$?
    if [ "$RET" == "0" ]; then
      echo "$paramName with value $paramValue found in $k8sResource"
    else
      echo "ERROR: $paramName with value $paramValue NOT found in $k8sResource"
      exit 1
    fi
  done 
} < $1

rm -f controller-describe node-describe csm-describe

exit 0
