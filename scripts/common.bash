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

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

function log() {
  case $1 in
    separator)
      echo "******"
      ;;
    error)
      echo
      echo "*****************************************"
      printf "${RED}ERROR: $2\n"
      printf "${RED}Installation cannot continue${NC}\n"
      exit 1
      ;;
    warning)
      echo
      printf "${YELLOW}Warning: $2${NC}\n"
      ;;
    step)
      printf "%-75s %s" "$2"
      ;;
    step_success)
      printf "${GREEN}Success${NC}\n"
      ;;
    step_failure)
      printf "${RED}Failed${NC}\n"
      ;;
    step_warning)
      printf "${YELLOW}Warning${NC}\n"
      ;;
    Passed)
      printf "${GREEN}Success${NC}\n"
      ;;
    Failed)
      printf "${RED}Failed${NC}\n"
      ;;
    *)
      echo -n "Unknown"
      ;;
  esac
}

# waitOnRunning
# will wait, for a timeout period, for a number of pods to go into Running state within a namespace
# arguments:
#  $1: required: namespace to watch
#  $2: required: comma separated list of deployment type and name pairs
#      for example: "statefulset mystatefulset,daemonset mydaemonset"
#  $3: optional: timeout value, 300 seconds is the default.
waitOnRunning() {
  if [ -z "${2}" ]; then
    echo "No namespace and/or list of deployments was supplied. This field is required for waitOnRunning"
    return 1
  fi
  # namespace
  local NS="${1}"
  # pods
  IFS="," read -r -a PODS <<< "${2}"
  # timeout value passed in, or 300 seconds as a default
  local TIMEOUT="300"
  if [ -n "${3}" ]; then
    TIMEOUT="${3}"
  fi

  RUNNING=0
  for D in "${PODS[@]}"; do
    echo
    echo "Checking $D Waiting up to $TIMEOUT seconds to roll out."
    echo
    kubectl -n "${NS}" rollout status --timeout=${TIMEOUT}s ${D} 2>/dev/null
    if [ $? -ne 0 ]; then
      RUNNING=1
    fi
  done

  if [ $RUNNING -ne 0 ]; then
    return 1
  fi
  return 0
}

function check_or_create_namespace() {
  # Check if namespace exists
  log separator
  echo "Checking if namespace exists '$1'"
  kubectl get namespace $1 > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo "Namespace '$1' doesn't exist"
    echo "Creating namespace '$1'"
    kubectl create namespace $1 2>&1 >/dev/null
    if [ $? -ne 0 ]; then
      echo "Failed to create namespace: '$1'"
      echo "Exiting with failure"
      exit 1
    fi
  else
    echo "Namespace '$1' already exists"
  fi
  echo
}

# Get the kubernetes major and minor version numbers.
kMajorVersion=$(kubectl version -o="yaml" | grep -A8 'serverVersion:' | grep 'major'| egrep -o '[0-9]+')
kMinorVersion=$(kubectl version -o="yaml" | grep -A8 'serverVersion:' | grep 'minor'| egrep -o '[0-9]+')
kubectl get crd | grep securitycontextconstraints.security.openshift.io --quiet
if [ $? -ne 0 ]; then
  isOpenShift=false
else
  isOpenShift=true
fi

