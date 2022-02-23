#!/bin/bash
# This script does the following:
# 1. Create a CatalogSource containing index for various Operator versions
# 2. Create an OperatorGroup
# 3. Create a subscription for the Operator

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
ROOTDIR="$(dirname "$SCRIPTDIR")"
DEPLOYDIR="$ROOTDIR/deploy/olm"

# Constants
CATALOGSOURCE="dell-csm-catalogsource"
OPERATORGROUP="dell-csm-operatorgroup"
SUBSCRIPTION="dell-csm-subscription"
COMMUNITY_MANIFEST="operator_community.yaml"

MANIFEST_FILE="$DEPLOYDIR/$COMMUNITY_MANIFEST"

unableToFindKubectlErrorMsg="Install kubectl before running this script"
uninstallComponentErrorMsg="Remove all existing installations before running this script"
installOLMErrorMsg="Install all OLM components correctly before running this script"


catsrccrd="catalogsources.operators.coreos.com"
csvcrd="clusterserviceversions.operators.coreos.com"
ipcrd="installplans.operators.coreos.com"
opgroupcrd="operatorgroups.operators.coreos.com"
subcrd="subscriptions.operators.coreos.com"

# print header information
function header() {
  echo "Installing CSM Operator in an OLM environment"
  echo
}

function check_for_kubectl() {
  echo "*****"
  echo "Checking for kubectl installation"
  out=$(command -v kubectl)
  if [ $? -ne 0 ]; then
    log error "Couldn't find kubectl binary in path" $unableToFindKubectlErrorMsg
  fi
  echo "kubectl exists"
  echo
}

function check_for_olm_components() {
  echo "*****"
  echo "Checking for OLM installation"
  kubectl get crd | grep $catsrccrd --quiet
  if [ $? -ne 0 ]; then
    log error "Couldn't find $catsrccrd. $installOLMErrorMsg"
  fi
  kubectl get crd | grep $csvcrd --quiet
  if [ $? -ne 0 ]; then
    log error "Couldn't find csvcrd. $installOLMErrorMsg"
  fi
  kubectl get crd | grep $ipcrd --quiet
  if [ $? -ne 0 ]; then
    log error "Couldn't find $ipcrd. $installOLMErrorMsg"
  fi
  kubectl get crd | grep $opgroupcrd --quiet
  if [ $? -ne 0 ]; then
    log error "Couldn't find $opgroupcrd. $installOLMErrorMsg"
  fi
  kubectl get crd | grep $subcrd --quiet
  if [ $? -ne 0 ]; then
    log error "Couldn't find $subcrd. $installOLMErrorMsg"
  fi
  echo "OLM exists"
  echo
}

function check_existing_installation() {
  echo "*****"
  echo "Checking for existing installation of Dell CSM Operator"
  kubectl get catalogsource "$CATALOGSOURCE" -n $NS > /dev/null 2>&1
  if [ $? -eq 0 ]; then
    log error "A CatalogSource with name $CATALOGSOURCE already exists in namespace $NS. $uninstallComponentErrorMsg "
  fi
  kubectl get operatorgroup "$OPERATORGROUP" -n $NS > /dev/null 2>&1
  if [ $? -eq 0 ]; then
    log error "An OperatorGroup with name $OPERATORGROUP already exists in namespace $NS. $uninstallComponentErrorMsg"
  fi
  kubectl get Subscription "$SUBSCRIPTION" -n "$NS" > /dev/null 2>&1
  if [ $? -eq 0 ]; then
    log error "A Subscription with name $SUBSCRIPTION already exists in namespace $NS. $uninstallComponentErrorMsg"
  fi
  echo
}

function set_namespace() {
  NS="test-csm-operator-olm"
  echo "*****"
  echo "CSM Operator will be installed in namespace: $NS"
}

function check_or_create_namespace() {
  # Check if namespace exists
  kubectl get namespace $NS > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo "Namespace $NS doesn't exist"
    echo "Creating namespace $NS"
    echo "kubectl create namespace $NS"
    kubectl create namespace $NS 2>&1 >/dev/null
    if [ $? -ne 0 ]; then
      echo "Failed to create namespace: $NS"
      echo "Exiting with failure"
      exit 1
    fi
  else
    echo "Namespace $NS already exists"
  fi
  echo
}

function install_operator() {
  echo "*****"
  echo "Installing Operator"
  kubectl apply -f $MANIFEST_FILE
  echo
}

source "$SCRIPTDIR"/common.bash

header
check_for_kubectl
check_for_olm_components
set_namespace
check_or_create_namespace
check_existing_installation
install_operator

echo "*****"
echo "The installation will take some time to complete"
echo "If the installation is successful, a CSV with the status 'Succeeded' should be created in the namespace $NS"
