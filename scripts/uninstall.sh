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