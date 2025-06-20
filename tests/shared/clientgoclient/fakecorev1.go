//  Copyright © 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/testing"
)

type FakeCoreV1 struct {
	*testing.Fake
}

func (f *FakeCoreV1) RESTClient() rest.Interface {
	return nil
}

func (f *FakeCoreV1) ComponentStatuses() corev1.ComponentStatusInterface {
	panic("not implemented")
}

func (f *FakeCoreV1) ConfigMaps(_ string) corev1.ConfigMapInterface {
	panic("not implemented")
}

func (f *FakeCoreV1) Endpoints(_ string) corev1.EndpointsInterface {
	panic("not implemented")
}

func (f *FakeCoreV1) Events(_ string) corev1.EventInterface {
	panic("not implemented")
}

func (f *FakeCoreV1) LimitRanges(_ string) corev1.LimitRangeInterface {
	panic("not implemented")
}

func (f *FakeCoreV1) Namespaces() corev1.NamespaceInterface {
	panic("not implemented")
}

func (f *FakeCoreV1) Nodes() corev1.NodeInterface {
	panic("not implemented")
}

func (f *FakeCoreV1) PersistentVolumes() corev1.PersistentVolumeInterface {
	panic("not implemented")
}

func (f *FakeCoreV1) PersistentVolumeClaims(_ string) corev1.PersistentVolumeClaimInterface {
	panic("not implemented")
}

func (f *FakeCoreV1) Pods(_ string) corev1.PodInterface {
	return &FakePod{}
}

func (f *FakeCoreV1) PodTemplates(_ string) corev1.PodTemplateInterface {
	panic("not implemented")
}

func (f *FakeCoreV1) ReplicationControllers(_ string) corev1.ReplicationControllerInterface {
	panic("not implemented")
}

func (f *FakeCoreV1) ResourceQuotas(_ string) corev1.ResourceQuotaInterface {
	panic("not implemented")
}

func (f *FakeCoreV1) Secrets(_ string) corev1.SecretInterface {
	panic("not implemented")
}

func (f *FakeCoreV1) Services(_ string) corev1.ServiceInterface {
	panic("not implemented")
}

func (f *FakeCoreV1) ServiceAccounts(_ string) corev1.ServiceAccountInterface {
	panic("not implemented")
}
