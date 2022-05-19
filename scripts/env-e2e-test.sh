# [Optional] ginko options for custom runs
export GINKGO_OPTS="-v"

# [Optional] Path to .kube cnifguration if it is not in the deafult loacaltion
export KUBECONFIG=""

# Must suply path to values file
export VALUES_FILE=""

# [Optional] namespace of operator if you deployed it to a namespace diffrent form the one below.
export OPERATOR_NAMESPACE="dell-csm-operator"
