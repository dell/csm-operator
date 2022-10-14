# [Optional] ginko options for custom runs
export GINKGO_OPTS="-v"

# [Optional] Path to .kube configuration if it is not in the default location
# export KUBECONFIG=""

# Must supply path to values file if different from testfiles/values.yaml
export VALUES_FILE="testfiles/values.yaml"

# USER MODIFICATION REQUIRED: must supply path to your cert-csi binary
export CERT_CSI="/root/cert-csi"

# [Optional] uncomment any modules you want to test
# export AUTHORIZATION=true
# export REPLICATION=true
# export OBSERVABILITY=true

# [Optional] namespace of operator if you deployed it to a namespace diffrent form the one below.
# export OPERATOR_NAMESPACE="dell-csm-operator"
