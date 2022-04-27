package clientgoclient

import (
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	rest "k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FakeCoreV1 struct {
	FakeClient client.Client
}

func (c *FakeCoreV1) ComponentStatuses() v1.ComponentStatusInterface {
	panic("implement me")
}

func (c *FakeCoreV1) ConfigMaps(namespace string) v1.ConfigMapInterface {
	return &FakeConfigMaps{
		FakeClient: c.FakeClient,
		Namespace:  namespace,
	}
}

func (c *FakeCoreV1) Endpoints(namespace string) v1.EndpointsInterface {
	panic("implement me")
}

func (c *FakeCoreV1) Events(namespace string) v1.EventInterface {
	panic("implement me")
}

func (c *FakeCoreV1) LimitRanges(namespace string) v1.LimitRangeInterface {
	panic("implement me")
}

func (c *FakeCoreV1) Namespaces() v1.NamespaceInterface {
	panic("implement me")
}

func (c *FakeCoreV1) Nodes() v1.NodeInterface {
	panic("implement me")
}

func (c *FakeCoreV1) PersistentVolumes() v1.PersistentVolumeInterface {
	panic("implement me")
}

func (c *FakeCoreV1) PersistentVolumeClaims(namespace string) v1.PersistentVolumeClaimInterface {
	panic("implement me")
}

func (c *FakeCoreV1) Pods(namespace string) v1.PodInterface {
	panic("implement me")
}

func (c *FakeCoreV1) PodTemplates(namespace string) v1.PodTemplateInterface {
	panic("implement me")
}

func (c *FakeCoreV1) ReplicationControllers(namespace string) v1.ReplicationControllerInterface {
	panic("implement me")
}

func (c *FakeCoreV1) ResourceQuotas(namespace string) v1.ResourceQuotaInterface {
	panic("implement me")
}

func (c *FakeCoreV1) Secrets(namespace string) v1.SecretInterface {
	panic("implement me")
}

func (c *FakeCoreV1) Services(namespace string) v1.ServiceInterface {
	panic("implement me")
}

func (c *FakeCoreV1) ServiceAccounts(namespace string) v1.ServiceAccountInterface {
	panic("implement me")
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeCoreV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
