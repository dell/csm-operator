#!/bin/bash

VERIFYSCRIPT="verify.sh"
SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
PROG="${0}"
ROOTDIR="$(dirname "$SCRIPTDIR")"
DEPLOYDIR="$ROOTDIR/deploy"

#
# usage will print command execution help and then exit
function usage() {
  echo
  echo "Help for $PROG"
  echo
  echo "Usage: $PROG options..."
  echo "Options:"
  echo "  Optional"
  echo "  --upgrade                                Perform an upgrade of the Operator, default is false"
  echo "  -h                                       Help"
  echo

  exit 0
}

# warning, with an option for users to continue
function warning() {
  echo "*****************************************"
  echo "WARNING:"
  for N in "$@"; do
    echo $N
  done
  echo
  if [ "${ASSUMEYES}" == "true" ]; then
    echo "Continuing as '-Y' argument was supplied"
    return
  fi
  read -n 1 -p "Press 'y' to continue or any other key to exit: " CONT
  echo
  if [ "${CONT}" != "Y" -a "${CONT}" != "y" ]; then
    echo "quitting at user request"
    exit 2
  fi
}

# print header information
function header() {
  if [ "$MODE" == "upgrade" ]; then
    echo "Upgrading Dell Container Storage Modules Operator"
  else
    echo "Installing Dell Container Storage Modules Operator"
  fi

  echo
}

# verify pre-requisites
function verify_prerequisites() {
  if [ ! -f "${SCRIPTDIR}/${VERIFYSCRIPT}" ]; then
    log error "Unable to locate ${VERIFYSCRIPT} script in ${SCRIPTDIR}"
  fi
  bash "${SCRIPTDIR}/${VERIFYSCRIPT}"
  case $? in
  0) ;;
  
  1)
    warning "Pre-requisites validation failed but installation can continue. " \
      "This may affect driver/module installation."
    ;;
  *)
    log error "Pre-requisites validation failed."
    ;;
  esac
}

function check_existing_installation() {
  log separator
  echo "Checking for existing installation of Dell Container Storage Modules Operator"
  # get namespace from YAML file for deployment
  NS_STRING=$(cat ${DEPLOYDIR}/operator.yaml | grep "namespace:" | head -1)
  if [ -z "${NS_STRING}" ]; then
    echo "Couldn't find any target namespace in ${DEPLOYDIR}/operator.yaml"
    exit 1
  fi
  # find the namespace from the filtered string
  NAMESPACE=$(echo $NS_STRING | cut -d ' ' -f2)
  
  # check for existing installations in the namespace
  log step "Checking for existing installation"
  # check for operator in dell-csm-operator namespace
  kubectl get pods -n ${NAMESPACE} | grep "dell-csm-operator" --quiet
  if [ $? -eq 0 ]; then
    operator_in_namespace=true
  fi

  if [ "$MODE" == "upgrade" ]; then
  	if  [ "$operator_in_namespace" = true ]; then
       log step_success
       echo "Found existing installation of log error in '$NAMESPACE' namespace"
       echo "Attempting to upgrade the Operator as --upgrade option was specified"
    else
       log step_failure
       log error "Operator is not found in '$NAMESPACE' namespace to upgrade.Install the operator without the upgrade option."
    fi
  else
  	if [ "$operator_in_namespace" = true ]; then
       log step_failure
       log warning "Found existing installation of dell-csm-operator in '$NAMESPACE' namespace"
       log error "Remove the existing installation using uninstall.sh script, or use the --upgrade option to upgrade the Operator"
       exit 1
    else
       log step_success
    fi
  fi
  echo
}

function install_or_update_crd() {
  log separator
  if [ "$MODE" == "upgrade" ]; then
    log step "Update CRD"
  else
    log step "Install/Update CRD"
  fi
  kubectl apply -f ${DEPLOYDIR}/crds/storage.dell.com_containerstoragemodules.yaml 2>&1 >/dev/null
  if [ $? -ne 0 ]; then
    log error "Failed to install/update CRD"
  fi
  log step_success
  echo
}

function create_operator_deployment() {
  log separator
  if [ "$MODE" == "upgrade" ]; then
    log step "Upgrade Operator"
  else
    log step "Install Operator"
  fi
  kubectl apply -f ${DEPLOYDIR}/operator.yaml 2>&1 >/dev/null
  if [ $? -ne 0 ]; then
    log error "Failed to deploy operator"
  fi
  log step_success
  echo
}

function install_operator() {
  log separator
  if [ "$MODE" == "upgrade" ]; then
    log separator
    echo "Upgrading Operator"
  else
    log separator
    echo "Installing Operator"
  fi
  install_or_update_crd
  create_operator_deployment
  echo
}

function check_progress() {
  # find out the deployment name
  # wait for the deployment to finish, use the default timeout
  waitOnRunning "${NAMESPACE}" "deployment dell-csm-operator-controller-manager"
  if [ $? -eq 1 ]; then
    warning "Timed out waiting for installation of the operator to complete." \
      "This does not indicate a fatal error, pods may take a while to start." \
      "Progress can be checked by running \"kubectl get pods -n dell-csm-operator\"."
  fi
}

# Print a nice summary at the end
function summary() {
  echo
  log separator
  echo "Installation complete"
  echo
}

#
# main
#
ASSUMEYES="false"

while getopts ":h-:" optchar; do
  case "${optchar}" in
  -)
    case "${OPTARG}" in
    upgrade)
      MODE="upgrade"
      ;;
    *)
      echo "Unknown option -${OPTARG}"
      echo "For help, run $PROG -h"
      exit 1
      ;;
    esac
    ;;

  h)
    usage
    ;;
  *)
    echo "Unknown option -${OPTARG}"
    echo "For help, run $PROG -h"
    exit 1
    ;;
  esac
done

source "$SCRIPTDIR"/common.bash
header
log separator
check_existing_installation
verify_prerequisites
check_or_create_namespace $NAMESPACE
install_operator
check_progress
summary
