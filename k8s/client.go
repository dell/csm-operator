package k8s

import (
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
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

// NewControllerRuntimeClient will return a new controller runtime client using config
func NewControllerRuntimeClient(data []byte) (ctrlClient.Client, error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(data)
	if err != nil {
		return nil, err
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	return ctrlClient.New(restConfig, ctrlClient.Options{Scheme: scheme})
}
