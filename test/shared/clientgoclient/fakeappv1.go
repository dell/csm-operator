package clientgoclient

import (
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FakeAppsV1 implements AppsV1Interface
type FakeAppsV1 struct {
	FakeClient client.Client
}

// DaemonSets takea a namespace and returns an DaemonSetInterface
func (c *FakeAppsV1) DaemonSets(namespace string) v1.DaemonSetInterface {
	return &FakeDaemonSets{
		FakeClient: c.FakeClient,
		Namespace:  namespace,
	}
}

// Deployments takea a namespace and returns an DeploymentInterface
func (c *FakeAppsV1) Deployments(namespace string) v1.DeploymentInterface {
	return &FakeDeployments{
		FakeClient: c.FakeClient,
		Namespace:  namespace,
	}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeAppsV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}

// ControllerRevisions takes a namespace and returns an ControllerRevisionInterface
func (c *FakeAppsV1) ControllerRevisions(namespace string) v1.ControllerRevisionInterface {
	panic("implement me")
}

// ReplicaSets takes a namespace and returns an ReplicaSetInterface
func (c *FakeAppsV1) ReplicaSets(namespace string) v1.ReplicaSetInterface {
	panic("implement me")
}

// StatefulSets takes a namespace and returns an StatefulSetInterface
func (c *FakeAppsV1) StatefulSets(namespace string) v1.StatefulSetInterface {
	panic("implement me")
}
