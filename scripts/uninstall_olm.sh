#!/bin/bash
SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
ROOTDIR="$(dirname "$SCRIPTDIR")"
DEPLOYDIR="$ROOTDIR/deploy/olm"

# Constants
COMMUNITY_MANIFEST="operator_community.yaml"
MANIFEST_FILE="$DEPLOYDIR/$COMMUNITY_MANIFEST"

# Set the namespace
NS="test-csm-operator-olm"

# Get CSV name
CSV=`kubectl get csv -n $NS --no-headers -o custom-columns=":metadata.name" | grep dell-csm-operator 2>&1`

echo "Deleting the Operator Deployment"
echo
echo "*****"
kubectl delete -f $MANIFEST_FILE
kubectl delete csv $CSV -n $NS
echo