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
	"k8s.io/client-go/discovery"
	admissionregistrationv1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1beta1"
	internalv1alpha1 "k8s.io/client-go/kubernetes/typed/apiserverinternal/v1alpha1"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	appsv1beta1 "k8s.io/client-go/kubernetes/typed/apps/v1beta1"
	appsv1beta2 "k8s.io/client-go/kubernetes/typed/apps/v1beta2"
	authenticationv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	authenticationv1beta1 "k8s.io/client-go/kubernetes/typed/authentication/v1beta1"
	authorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	authorizationv1beta1 "k8s.io/client-go/kubernetes/typed/authorization/v1beta1"
	autoscalingv1 "k8s.io/client-go/kubernetes/typed/autoscaling/v1"
	autoscalingv2 "k8s.io/client-go/kubernetes/typed/autoscaling/v2"
	autoscalingv2beta1 "k8s.io/client-go/kubernetes/typed/autoscaling/v2beta1"
	autoscalingv2beta2 "k8s.io/client-go/kubernetes/typed/autoscaling/v2beta2"
	batchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	batchv1beta1 "k8s.io/client-go/kubernetes/typed/batch/v1beta1"
	certificatesv1 "k8s.io/client-go/kubernetes/typed/certificates/v1"
	certificatesv1beta1 "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	coordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	coordinationv1beta1 "k8s.io/client-go/kubernetes/typed/coordination/v1beta1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	discoveryv1 "k8s.io/client-go/kubernetes/typed/discovery/v1"
	discoveryv1beta1 "k8s.io/client-go/kubernetes/typed/discovery/v1beta1"
	eventsv1 "k8s.io/client-go/kubernetes/typed/events/v1"
	eventsv1beta1 "k8s.io/client-go/kubernetes/typed/events/v1beta1"
	extensionsv1beta1 "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	flowcontrolv1alpha1 "k8s.io/client-go/kubernetes/typed/flowcontrol/v1alpha1"
	flowcontrolv1beta1 "k8s.io/client-go/kubernetes/typed/flowcontrol/v1beta1"
	flowcontrolv1beta2 "k8s.io/client-go/kubernetes/typed/flowcontrol/v1beta2"
	networkingv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
	//networkalphav1 "k8s.io/client-go/kubernetes/typed/networking/v1alpha1"
	networkingv1beta1 "k8s.io/client-go/kubernetes/typed/networking/v1beta1"
	nodev1 "k8s.io/client-go/kubernetes/typed/node/v1"
	nodev1alpha1 "k8s.io/client-go/kubernetes/typed/node/v1alpha1"
	nodev1beta1 "k8s.io/client-go/kubernetes/typed/node/v1beta1"
	policyv1 "k8s.io/client-go/kubernetes/typed/policy/v1"
	policyv1beta1 "k8s.io/client-go/kubernetes/typed/policy/v1beta1"
	rbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	rbacv1alpha1 "k8s.io/client-go/kubernetes/typed/rbac/v1alpha1"
	rbacv1beta1 "k8s.io/client-go/kubernetes/typed/rbac/v1beta1"
	schedulingv1 "k8s.io/client-go/kubernetes/typed/scheduling/v1"
	schedulingv1alpha1 "k8s.io/client-go/kubernetes/typed/scheduling/v1alpha1"
	schedulingv1beta1 "k8s.io/client-go/kubernetes/typed/scheduling/v1beta1"
	storagev1 "k8s.io/client-go/kubernetes/typed/storage/v1"
	storagev1alpha1 "k8s.io/client-go/kubernetes/typed/storage/v1alpha1"
	storagev1beta1 "k8s.io/client-go/kubernetes/typed/storage/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// K8sClient implements client-go kubernetes interface
// Internally it's calling the FakeClient to apply/get/create/delete etc.
// as they need to shared the same map in memory
type K8sClient struct {
	FakeClient client.Client
}

// NewFakeClient returns a new K8sClient
func NewFakeClient(c client.Client) *K8sClient {
	return &K8sClient{
		FakeClient: c,
	}
}

// AppsV1 returns an fake AppsV1Interface implementation from the given client
func (c *K8sClient) AppsV1() appsv1.AppsV1Interface {
	return &FakeAppsV1{
		FakeClient: c.FakeClient,
	}
}

// AdmissionregistrationV1 retrieves the AdmissionregistrationV1Client
func (c *K8sClient) AdmissionregistrationV1() admissionregistrationv1.AdmissionregistrationV1Interface {
	panic("implement me")
}

// AdmissionregistrationV1beta1 retrieves the AdmissionregistrationV1beta1Client
func (c *K8sClient) AdmissionregistrationV1beta1() admissionregistrationv1beta1.AdmissionregistrationV1beta1Interface {
	panic("implement me")
}

// InternalV1alpha1 retrieves the InternalV1alpha1Client
func (c *K8sClient) InternalV1alpha1() internalv1alpha1.InternalV1alpha1Interface {
	panic("implement me")
}

// AppsV1beta1 retrieves the AppsV1beta1Client
func (c *K8sClient) AppsV1beta1() appsv1beta1.AppsV1beta1Interface {
	panic("implement me")
}

// AppsV1beta2 retrieves the AppsV1beta2Client
func (c *K8sClient) AppsV1beta2() appsv1beta2.AppsV1beta2Interface {
	panic("implement me")
}

// AuthenticationV1 retrieves the AuthenticationV1Client
func (c *K8sClient) AuthenticationV1() authenticationv1.AuthenticationV1Interface {
	panic("implement me")
}

// AuthenticationV1beta1 retrieves the AuthenticationV1beta1Client
func (c *K8sClient) AuthenticationV1beta1() authenticationv1beta1.AuthenticationV1beta1Interface {
	panic("implement me")
}

// AuthorizationV1 retrieves the AuthorizationV1Client
func (c *K8sClient) AuthorizationV1() authorizationv1.AuthorizationV1Interface {
	panic("implement me")
}

// AuthorizationV1beta1 retrieves the AuthorizationV1beta1Client
func (c *K8sClient) AuthorizationV1beta1() authorizationv1beta1.AuthorizationV1beta1Interface {
	panic("implement me")
}

// AutoscalingV1 retrieves the AutoscalingV1Client
func (c *K8sClient) AutoscalingV1() autoscalingv1.AutoscalingV1Interface {
	panic("implement me")
}

// AutoscalingV2 retrieves the AutoscalingV2Client
func (c *K8sClient) AutoscalingV2() autoscalingv2.AutoscalingV2Interface {
	panic("implement me")
}

// FlowcontrolV1beta2 retrieves the FlowcontrolV1beta2Client
func (c *K8sClient) FlowcontrolV1beta2() flowcontrolv1beta2.FlowcontrolV1beta2Interface {
	panic("implement me")
}

// AutoscalingV2beta1 retrieves the AutoscalingV2beta1Client
func (c *K8sClient) AutoscalingV2beta1() autoscalingv2beta1.AutoscalingV2beta1Interface {
	panic("implement me")
}

// AutoscalingV2beta2 retrieves the AutoscalingV2beta2Client
func (c *K8sClient) AutoscalingV2beta2() autoscalingv2beta2.AutoscalingV2beta2Interface {
	panic("implement me")
}

// BatchV1 retrieves the BatchV1Client
func (c *K8sClient) BatchV1() batchv1.BatchV1Interface {
	panic("implement me")
}

// BatchV1beta1 retrieves the BatchV1beta1Client
func (c *K8sClient) BatchV1beta1() batchv1beta1.BatchV1beta1Interface {
	panic("implement me")
}

// CertificatesV1 retrieves the CertificatesV1Client
func (c *K8sClient) CertificatesV1() certificatesv1.CertificatesV1Interface {
	panic("implement me")
}

// CertificatesV1beta1 retrieves the CertificatesV1beta1Client
func (c *K8sClient) CertificatesV1beta1() certificatesv1beta1.CertificatesV1beta1Interface {
	panic("implement me")
}

// CoordinationV1beta1 retrieves the CoordinationV1beta1Client
func (c *K8sClient) CoordinationV1beta1() coordinationv1beta1.CoordinationV1beta1Interface {
	panic("implement me")
}

// CoordinationV1 retrieves the CoordinationV1Client
func (c *K8sClient) CoordinationV1() coordinationv1.CoordinationV1Interface {
	panic("implement me")
}

// CoreV1 retrieves the CoreV1Client
func (c *K8sClient) CoreV1() corev1.CoreV1Interface {
	panic("implement me")
}

// DiscoveryV1 retrieves the DiscoveryV1Client
func (c *K8sClient) DiscoveryV1() discoveryv1.DiscoveryV1Interface {
	panic("implement me")
}

// DiscoveryV1beta1 retrieves the DiscoveryV1beta1Client
func (c *K8sClient) DiscoveryV1beta1() discoveryv1beta1.DiscoveryV1beta1Interface {
	panic("implement me")
}

// EventsV1 retrieves the EventsV1Client
func (c *K8sClient) EventsV1() eventsv1.EventsV1Interface {
	panic("implement me")
}

// EventsV1beta1 retrieves the EventsV1beta1Client
func (c *K8sClient) EventsV1beta1() eventsv1beta1.EventsV1beta1Interface {
	panic("implement me")
}

// ExtensionsV1beta1 retrieves the ExtensionsV1beta1Client
func (c *K8sClient) ExtensionsV1beta1() extensionsv1beta1.ExtensionsV1beta1Interface {
	panic("implement me")
}

// FlowcontrolV1alpha1 retrieves the FlowcontrolV1alpha1Client
func (c *K8sClient) FlowcontrolV1alpha1() flowcontrolv1alpha1.FlowcontrolV1alpha1Interface {
	panic("implement me")
}

// FlowcontrolV1beta1 retrieves the FlowcontrolV1beta1Client
func (c *K8sClient) FlowcontrolV1beta1() flowcontrolv1beta1.FlowcontrolV1beta1Interface {
	panic("implement me")
}

// NetworkingV1 retrieves the NetworkingV1Client
func (c *K8sClient) NetworkingV1() networkingv1.NetworkingV1Interface {
	panic("implement me")
}

// NetworkingV1beta1 retrieves the NetworkingV1beta1Client
func (c *K8sClient) NetworkingV1beta1() networkingv1beta1.NetworkingV1beta1Interface {
	panic("implement me")
}

// NodeV1 retrieves the NodeV1Client
func (c *K8sClient) NodeV1() nodev1.NodeV1Interface {
	panic("implement me")
}

// NodeV1alpha1 retrieves the NodeV1alpha1Client
func (c *K8sClient) NodeV1alpha1() nodev1alpha1.NodeV1alpha1Interface {
	panic("implement me")
}

// NodeV1beta1 retrieves the NodeV1beta1Client
func (c *K8sClient) NodeV1beta1() nodev1beta1.NodeV1beta1Interface {
	panic("implement me")
}

// PolicyV1 retrieves the PolicyV1Client
func (c *K8sClient) PolicyV1() policyv1.PolicyV1Interface {
	panic("implement me")
}

// PolicyV1beta1 retrieves the PolicyV1beta1Client
func (c *K8sClient) PolicyV1beta1() policyv1beta1.PolicyV1beta1Interface {
	panic("implement me")
}

// RbacV1 retrieves the RbacV1Client
func (c *K8sClient) RbacV1() rbacv1.RbacV1Interface {
	panic("implement me")
}

// RbacV1beta1 retrieves the RbacV1beta1Client
func (c *K8sClient) RbacV1beta1() rbacv1beta1.RbacV1beta1Interface {
	panic("implement me")
}

// RbacV1alpha1 retrieves the RbacV1alpha1Client
func (c *K8sClient) RbacV1alpha1() rbacv1alpha1.RbacV1alpha1Interface {
	panic("implement me")
}

// SchedulingV1alpha1 retrieves the SchedulingV1alpha1Client
func (c *K8sClient) SchedulingV1alpha1() schedulingv1alpha1.SchedulingV1alpha1Interface {
	panic("implement me")
}

// SchedulingV1beta1 retrieves the SchedulingV1beta1Client
func (c *K8sClient) SchedulingV1beta1() schedulingv1beta1.SchedulingV1beta1Interface {
	panic("implement me")
}

// SchedulingV1 retrieves the SchedulingV1Client
func (c *K8sClient) SchedulingV1() schedulingv1.SchedulingV1Interface {
	panic("implement me")
}

// StorageV1beta1 retrieves the StorageV1beta1Client
func (c *K8sClient) StorageV1beta1() storagev1beta1.StorageV1beta1Interface {
	panic("implement me")
}

// StorageV1 retrieves the StorageV1Client
func (c *K8sClient) StorageV1() storagev1.StorageV1Interface {
	panic("implement me")
}

// StorageV1alpha1 retrieves the StorageV1alpha1Client
func (c *K8sClient) StorageV1alpha1() storagev1alpha1.StorageV1alpha1Interface {
	panic("implement me")
}

// Discovery retrieves DiscoveryInterface
func (c *K8sClient) Discovery() discovery.DiscoveryInterface {
	panic("implement me")
}

// NetworkingV1alpha1 retrieves the NetworkingV1alpha1Client
//func (c *K8sClient) NetworkingV1alpha1() networkalphav1.NetworkingV1alpha1Interface {
	//panic("implement me")
//}
