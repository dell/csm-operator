//  Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//       http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

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
func (c *FakeAppsV1) ControllerRevisions(_ string) v1.ControllerRevisionInterface {
	panic("implement me")
}

// ReplicaSets takes a namespace and returns an ReplicaSetInterface
func (c *FakeAppsV1) ReplicaSets(_ string) v1.ReplicaSetInterface {
	panic("implement me")
}

// StatefulSets takes a namespace and returns an StatefulSetInterface
func (c *FakeAppsV1) StatefulSets(_ string) v1.StatefulSetInterface {
	panic("implement me")
}
