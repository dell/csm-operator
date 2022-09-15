# [Optional] ginko options for custom runs
export GINKGO_OPTS="-v"

# [Optional] Path to .kube configuration if it is not in the default location
# export KUBECONFIG=""

# Must supply path to values file
export VALUES_FILE="testfiles/values.yaml"

# Must supply path to cert-csi binary
export CERT_CSI="/root/cert-csi"

# [Optional] set any modules you want to test to true
# export AUTHORIZATION=true
# export REPLICATION=true

# [Optional] namespace of operator if you deployed it to a namespace diffrent form the one below.
# export OPERATOR_NAMESPACE="dell-csm-operator"
