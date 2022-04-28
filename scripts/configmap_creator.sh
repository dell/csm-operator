#!/bin/bash

source ./common.sh

NS="dell-csm-operator"
DRIVER="powerscale"
DRIVERVERSION="v2.2.0"
# All below variables not recommended for user modification
DELETE_EXIST_CFGMAP=0
TGZFOLDER=""
TEMPDIR="/tmp/csm-oper-cfg-map/"
PROG="${0}"
CFGMAP_NAMES_FILE="cm_names.txt"
SCRIPTDIR=`pwd`
cfgmaps_created=""

function cleanup() {
  # Get rid of temp directory
  rm -rf $TEMPDIR
  cd $SCRIPTDIR
}

function cleanup_created_cfgmaps() {
  if [ "$cfgmaps_created" != "" ]; then
    kubectl delete configmap $cfgmaps_created -n $NS
  fi
}

function read_configmap_names() {
  # make sure there is no existing list
  rm -rf $CFGMAP_NAMES_FILE
  log section "Read configmap names from file $DRIVER-$DRIVERVERSION.yaml"
  cat $DRIVER-$DRIVERVERSION.yaml | while read line
  do
    echo $line | grep listOfConfigMapNames
      if [ "$?" == "0" ]; then
        echo "reading configmaps list"
          while read line
          do
            echo $line | grep "-"
            if [ "$?" == "0" ]; then
              echo $line | awk ' {print $2}' >> $CFGMAP_NAMES_FILE
            else
              log step "Done reading configmap names"
              break
            fi
          done
        fi
  done
}

function usage() {
  echo
  echo "Help for $PROG"
  echo
  echo "This script uses config files from the github.com/dell/csm-operator-config repo to install configmaps for a specified driver"
  echo "To select the driver and version, edit the DRIVER and DRIVERVERSION variables to reflect the drivername and version"
  echo
  echo "Usage: $PROG options..."
  echo "Options:"
  echo "  Optional"
  echo "  --force-configmap-delete                 Force deletion of existing configmaps"
  echo "  --use-local-tgz=<tgz-file>               Use local tgz in given folder"
  echo "  -h                                       Help"
  echo

  exit 0
}

while getopts ":h-:" optchar; do
  case "${optchar}" in
  -)
    case "${OPTARG}" in
    force-configmap-delete)
      DELETE_EXIST_CFGMAP=1
      ;;
    use-local-tgz)
      TGZFOLDER="${!OPTIND}"
      OPTIND=$((OPTIND + 1))
      ;;
    use-local-tgz=*)
      TGZFOLDER=${OPTARG#*=}
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

# make sure kubectl is available
kubectl --help >&/dev/null || {
  log error "kubectl required for installation... exiting"
  log Failed
  exit 1
}

# if we are using a local tgz, get the absolute path
if [ "$TGZFOLDER" != "" ]; then
  if [ -f $TGZFOLDER ]; then
    # Make tgz name absolute path. If tgz name already is absolute path, this will change nothing
    TGZFOLDERABS=`readlink -f $TGZFOLDER`
    log step "using tar file located at $TGZFOLDERABS"
  else
    log error "the provided tgz, $TGZFOLDER, was not found"
    cleanup
    log Failed
    exit 4
  fi
fi

# Make temporary directory, move into it
mkdir $TEMPDIR
cd $TEMPDIR


if [ "$TGZFOLDER" == "" ]; then
  log section "Downloading configmap yamls"

  # Download config tgz -- you will be required to put your personal access token in here as long as the csm-operator-config repo is private
  read -s -p 'Please enter GitHub token: ' github_token
  wget --header "Authorization: token ${github_token}" https://raw.githubusercontent.com/dell/csm-operator-config/main/$DRIVER/$DRIVER-$DRIVERVERSION/downloads/$DRIVER-$DRIVERVERSION.tgz
  wget_ret=$?
  if [ "$wget_ret" != "0" ]; then
    log error "wget of config files failed with return code $wget_ret, exiting"
    cleanup
    log Failed
    exit 2
  fi

  log section "Untar config files"
  tar -xzvf $DRIVER-$DRIVERVERSION.tgz
else
  log section "Untar config files"
  tar -xzvf $TGZFOLDERABS
fi

# read configmap names from yaml file
read_configmap_names

log section "Create config maps"
#cat $CFGMAP_NAMES_FILE | while read cfgmap
while read cfgmap
do
  log step "> Creating configmap $cfgmap"

  # check for existing configmap
  kubectl get configmap -n $NS | grep -q $cfgmap

  # delete if user wants to replace; otherwise, skip this iteration
  if [ "$?" == "0" ]; then
    if [ $DELETE_EXIST_CFGMAP -eq 1 ]; then
      kubectl delete configmap $cfgmap -n $NS
    else
      log step "Not replacing pre-existing configmap $cfgmap"
      continue
    fi
  fi

  # create new configmap
  kubectl create cm $cfgmap --from-file=$cfgmap -n $NS

  # check the configmap exists
  kubectl get configmap -n $NS | grep -q $cfgmap
  if [ "$?" == "0" ]; then
    cfgmaps_created="$cfgmaps_created$cfgmap "
    log step "configmap $cfgmap successfully created"
  else
    log error "configmap $cfgmap not successfully created"
    # clean up and exit
    cleanup_created_cfgmaps
    cleanup
    log Failed
    exit 3
  fi
done <<<`cat $CFGMAP_NAMES_FILE`
log section "Finishing configmap creation"

if [ "$cfgmaps_created" == "" ]; then
  log step "No new configmaps created"
else
  # remove trailing space then insert commas
  cfgmaps_created=${cfgmaps_created%?}
  log step "Following configmaps created: ${cfgmaps_created//[ ]/, }"
fi

log Passed
cleanup

