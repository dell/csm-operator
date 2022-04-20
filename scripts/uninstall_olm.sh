#!/bin/bash
SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
ROOTDIR="$(dirname "$SCRIPTDIR")"
DEPLOYDIR="$ROOTDIR/deploy/olm"
source "$SCRIPTDIR"/common.bash


# Constants
COMMUNITY_MANIFEST="operator_community.yaml"
MANIFEST_FILE="$DEPLOYDIR/$COMMUNITY_MANIFEST"

# find the operator namespace from operator.yaml file
NS_STRING=$(cat ${MANIFEST_FILE} | grep "namespace:" | head -1)
if [ -z "${NS_STRING}" ]; then
  echo "Couldn't find any target namespace in ${MANIFEST_FILE}"
  exit 1
fi
# find the namespace from the filtered string
NAMESPACE=$(echo $NS_STRING | cut -d ' ' -f2)

# Get CSV name
CSV=`kubectl get csv -n $NAMESPACE --no-headers -o custom-columns=":metadata.name" | grep dell-csm-operator 2>&1`
log separator
echo "Deleting the Operator Deployment"
echo
log separator
kubectl delete -f $MANIFEST_FILE
kubectl delete csv $CSV -n $NAMESPACE
echo