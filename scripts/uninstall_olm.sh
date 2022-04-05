#!/bin/bash
SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
ROOTDIR="$(dirname "$SCRIPTDIR")"
DEPLOYDIR="$ROOTDIR/deploy/olm"
source "$SCRIPTDIR"/common.bash


# Constants
COMMUNITY_MANIFEST="operator_community.yaml"
MANIFEST_FILE="$DEPLOYDIR/$COMMUNITY_MANIFEST"

# Set the namespace
NAMESPACE="test-csm-operator-olm"

# Get CSV name
CSV=`kubectl get csv -n $NAMESPACE --no-headers -o custom-columns=":metadata.name" | grep dell-csm-operator 2>&1`
log separator
echo "Deleting the Operator Deployment"
kubectl delete -f $MANIFEST_FILE
kubectl delete csv $CSV -n $NAMESPACE
echo