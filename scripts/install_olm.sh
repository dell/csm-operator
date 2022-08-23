#!/bin/bash
# This script does the following:
# 1. Create a CatalogSource containing index for various Operator versions
# 2. Create an OperatorGroup
# 3. Create a subscription for the Operator

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
ROOTDIR="$(dirname "$SCRIPTDIR")"
DEPLOYDIR="$ROOTDIR/deploy/olm"
VERIFYSCRIPT="verify.sh"
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
  echo "Installing Dell Container Storage Modules Operator in an OLM environment"
  echo
}

function check_for_kubectl() {
  log separator
  echo "Checking for kubectl installation"
  out=$(command -v kubectl)
  if [ $? -ne 0 ]; then
    log error "Couldn't find kubectl binary in path" $unableToFindKubectlErrorMsg
  fi
  echo "kubectl exists"
  echo
}

function check_for_olm_components() {
  log separator
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
  log step_success
  echo
}

function check_existing_installation() {
  log separator
  echo "Checking for existing installation of Dell Container Storage Modules Operator"
  # get namespace from YAML file for deployment
  NS_STRING=$(cat ${MANIFEST_FILE} | grep "namespace:" | head -1)
  if [ -z "${NS_STRING}" ]; then
    echo "Couldn't find any target namespace in ${MANIFEST_FILE}"
    exit 1
  fi
  # find the namespace from the filtered string
  NAMESPACE=$(echo $NS_STRING | cut -d ' ' -f2)

  kubectl get catalogsource "$CATALOGSOURCE" -n $NAMESPACE > /dev/null 2>&1
  if [ $? -eq 0 ]; then
    log error "A CatalogSource with name $CATALOGSOURCE already exists in namespace $NAMESPACE. $uninstallComponentErrorMsg "
  fi
  kubectl get operatorgroup "$OPERATORGROUP" -n $NAMESPACE > /dev/null 2>&1
  if [ $? -eq 0 ]; then
    log error "An OperatorGroup with name $OPERATORGROUP already exists in namespace $NAMESPACE. $uninstallComponentErrorMsg"
  fi
  kubectl get Subscription "$SUBSCRIPTION" -n "$NAMESPACE" > /dev/null 2>&1
  if [ $? -eq 0 ]; then
    log error "A Subscription with name $SUBSCRIPTION already exists in namespace $NAMESPACE. $uninstallComponentErrorMsg"
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

function install_operator() {
  log separator
  echo "Installing Operator"
  kubectl apply -f $MANIFEST_FILE
  echo
}

source "$SCRIPTDIR"/common.bash
header
check_existing_installation
verify_prerequisites
check_for_olm_components
check_or_create_namespace $NAMESPACE
install_operator
log separator
echo "The installation will take some time to complete"
echo "If the installation is successful, a CSV with the status 'Succeeded' and a deployment dell-csm-operator-controller-manager pod with the status 'Running' should be created in the namespace $NAMESPACE"
