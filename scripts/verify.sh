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

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# print header information
function header() {
	echo
	log separator
	echo "Environment configuration"
	echo "Kubernetes Version: ${kMajorVersion}.${kMinorVersion}"
	echo "Openshift: ${isOpenShift}"
	echo
}

# verify that the snap CRDs are installed
function verify_snap_crds() {
	# check for the snapshot CRDs.
    CRDS=("VolumeSnapshotClasses" "VolumeSnapshotContents" "VolumeSnapshots")
    for C in "${CRDS[@]}"; do
      log step "Checking for $C CRD"
      # Verify that snapshot related CRDs/CRs exist on the system.
      kubectl explain ${C} > /dev/null 2>&1
      if [ $? -ne 0 ]; then
        AddError "The CRD for ${C} is not Found. These need to be installed by the Kubernetes administrator"
        RESULT_SNAP_CRDS="Failed"
        log step_failure
      else
        log step_success
      fi
    done
}

function verify_snapshot_controller() {
  log step "Checking if snapshot controller is deployed"
  # check for the snapshot-controller. These are strongly suggested but not required
	kubectl get pods -A | grep snapshot-controller --quiet
	if [ $? -ne 0 ]; then
		AddWarning "The Snapshot Controller was not found on the system. These need to be installed by the Kubernetes administrator."
		RESULT_SNAP_CONTROLLER="Failed"
		log step_failure
	else
	  log step_success
	fi
}


# error, installation will not continue
function AddError() {
  for N in "$@"; do
    ERRORS+=("${N}")
  done
}

# warning, installation can continue
function AddWarning() {
  for N in "$@"; do
    WARNINGS+=("${N}")
  done
}

# Print a nice summary at the end
function summary() {
	# print all the WARNINGS
	if [ "${#WARNINGS[@]}" -ne 0 ]; then
		echo
		echo "Warnings:"
		for E in "${WARNINGS[@]}"; do
  			echo "- ${E}"
		done
		RC=$EXIT_WARNING
	fi

	# print all the ERRORS
	if [ "${#ERRORS[@]}" -ne 0 ]; then
		echo
		echo "Errors:"
		for E in "${ERRORS[@]}"; do
  			echo "- ${E}"
		done
		RC=$EXIT_ERROR
	fi
}

function check_for_kubectl() {
  log step "Checking for kubectl installation"
  out=$(command -v kubectl)
  if [ $? -eq 0 ]; then
    log step_success
  else
    log error "Couldn't find kubectl binary in path"
  fi
}
#
# main
#
# default values
RESULT_SNAP_CRDS="Passed"
RESULT_SNAP_CONTROLLER="Passed"

# exit codes
EXIT_SUCCESS=0
EXIT_WARNING=1
EXIT_ERROR=99

# arrays of messages
WARNINGS=()
ERRORS=()

# return code
RC=0

# Determine the kubernetes version
source $SCRIPTDIR/common.bash

header
log separator
check_for_kubectl
verify_snap_crds
verify_snapshot_controller
summary

if [ ${RESULT_SNAP_CRDS} == "Failed" ]; then
  echo "Some of the CRDs are not found on the system. These need to be installed by the Kubernetes administrator."
fi
echo
exit $RC
