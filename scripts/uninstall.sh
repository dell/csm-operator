#!/bin/bash
SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
ROOTDIR="$(dirname "$SCRIPTDIR")"
DEPLOYDIR="$ROOTDIR/deploy"

# find the operator namespace from operator.yaml file
NS_STRING=$(cat ${DEPLOYDIR}/operator.yaml | grep "namespace:" | head -1)
if [ -z "${NS_STRING}" ]; then
  echo "Couldn't find any target namespace in ${DEPLOYDIR}/operator.yaml"
  exit 1
fi
# find the namespace from the filtered string
NAMESPACE=$(echo $NS_STRING | cut -d ' ' -f2)

echo "** Deleting the Operator Deployment **"
echo
echo kubectl delete -f $DEPLOYDIR/operator.yaml
kubectl delete -f $DEPLOYDIR/operator.yaml
echo