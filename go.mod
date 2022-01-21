module github.com/dell/csm-operator

go 1.17

require (
	github.com/go-logr/logr v1.2.2
	github.com/go-yaml/yaml v2.1.0+incompatible
	github.com/kubernetes-csi/external-snapshotter/client/v3 v3.0.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/stretchr/testify v1.7.0
	k8s.io/api v0.23.1
	k8s.io/apimachinery v0.23.1
	k8s.io/client-go v0.23.1
	sigs.k8s.io/controller-runtime v0.11.0
	sigs.k8s.io/yaml v1.3.0
)
