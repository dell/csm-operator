package clientgoClient

import (
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FakeAppsV1 struct {
	FakeClient client.Client
}

func (c *FakeAppsV1) DaemonSets(namespace string) v1.DaemonSetInterface {
	return &FakeDaemonSets{
		FakeClient: c.FakeClient,
		Namespace:      namespace,
	}
}

func (c *FakeAppsV1) Deployments(namespace string) v1.DeploymentInterface {
	return &FakeDeployments{
		FakeClient: c.FakeClient,
		Namespace:      namespace,
	}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeAppsV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}

func (c *FakeAppsV1) ControllerRevisions(namespace string) v1.ControllerRevisionInterface {
	panic("implement me")
}

func (c *FakeAppsV1) ReplicaSets(namespace string) v1.ReplicaSetInterface {
	panic("implement me")
}

func (c *FakeAppsV1) StatefulSets(namespace string) v1.StatefulSetInterface {
	panic("implement me")
}
