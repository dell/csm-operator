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

CSV_FILE=$1
NS=$2
NAME=$3

# get rid of any \r characters from windows
sed -i 's/\r//g' $CSV_FILE

# get controller describe
controllerpod=`kubectl get pods -n $NS | grep -m 1 $NAME-controller | awk '{print $1}'`
kubectl describe pod $controllerpod -n $NS > controller-describe

# get node describe
nodepod=`kubectl get pods -n $NS | grep -m 1 $NAME-node | awk '{print $1}'`
kubectl describe pod $nodepod -n $NS > node-describe

# get csm describe
kubectl describe csm $NAME -n $NS > csm-describe

{
  read
  while IFS=, read -r paramName grepOptions paramValue k8sResource numOccurences
  do
    WC=`cat $k8sResource-describe | grep "$paramName" | grep $grepOptions "$paramValue" | wc -l`
    if [ $WC -ge $numOccurences ]; then
      echo "$numOccurences occurences of $paramName with value $paramValue found in $k8sResource"
    else
      echo "ERROR: $WC occurences of $paramName with value $paramValue found in $k8sResource, $numOccurences expected"
      exit 1
    fi
  done 
} < $CSV_FILE

rm -f controller-describe node-describe csm-describe

exit 0
