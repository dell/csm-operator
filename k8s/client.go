package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/discovery"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// GetClientSetWrapper -
var GetClientSetWrapper = func() (kubernetes.Interface, error) {
	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}
// GetKubeAPIServerVersion returns version of the k8s/openshift cluster
func GetKubeAPIServerVersion() (*version.Info, error) {
	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	// Create the discoveryClient
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	sv, err := discoveryClient.ServerVersion()
	if err != nil {
		return nil, err
	}
	return sv, nil
}

// IsOpenShift - Returns a boolean which indicates if we are running in an OpenShift cluster
func IsOpenShift() (bool, error) {
	k8sClientSet, err := GetClientSetWrapper()
	if err != nil {
		return false, err
	}

	serverGroups, _, err := k8sClientSet.Discovery().ServerGroupsAndResources()
	if err != nil {
		return false, err
	}
	openshiftAPIGroup := "security.openshift.io"
	for i := 0; i < len(serverGroups); i++ {
		if serverGroups[i].Name == openshiftAPIGroup {
			return true, nil
		}
	}
	return false, nil
}
