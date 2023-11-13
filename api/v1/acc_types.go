//  Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package v1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ApexConnectivityClientSpec defines the desired state of ApexConnectivityClient
type ApexConnectivityClientSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Client is a Apex Connectivity Client for Dell Technologies
	Client Client `json:"client,omitempty" yaml:"client,omitempty"`
}

// ApexConnectivityClientStatus defines the observed state of ApexConnectivityClient
type ApexConnectivityClientStatus struct {
	// ClientStatus is the status of Client pods
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="ClientStatus",xDescriptors="urn:alm:descriptor:com.tectonic.ui:podStatuses"
	ClientStatus PodStatus `json:"clientStatus,omitempty"`

	// State is the state of the client installation
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="State",xDescriptors="urn:alm:descriptor:text"
	State CSMStateType `json:"state,omitempty" yaml:"state"`
}

// +kubebuilder:validation:Optional
// +kubebuilder:resource:scope=Namespaced,shortName={"acc"}
// +kubebuilder:printcolumn:name="CreationTime",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="CSMClientType",type=string,JSONPath=`.spec.client.csmClientType`,description="Type of Client"
// +kubebuilder:printcolumn:name="ConfigVersion",type=string,JSONPath=`.spec.client.configVersion`,description="Version of Apex client"
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`,description="State of Installation"
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ApexConnectivityClient is the Schema for the ApexConnectivityClient API
type ApexConnectivityClient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApexConnectivityClientSpec   `json:"spec,omitempty"`
	Status ApexConnectivityClientStatus `json:"status,omitempty"`
}

// ApexConnectivityClientList contains a list of ApexConnectivityClient
// +kubebuilder:object:root=true
type ApexConnectivityClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApexConnectivityClient `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApexConnectivityClient{}, &ApexConnectivityClientList{})
}

// GetApexConnectivityClientStatus - Returns a pointer to the client instance
func (cr *ApexConnectivityClient) GetApexConnectivityClientStatus() *ApexConnectivityClientStatus {
	return &cr.Status
}

// GetApexConnectivityClientName - Returns the Client
func (cr *ApexConnectivityClient) GetApexConnectivityClientName() string {
	return fmt.Sprintf("%s", cr.Name)
}

// GetApexConnectivityClientSpec - Returns a pointer to the GetApexConnectivityClientSpec instance
func (cr *ApexConnectivityClient) GetApexConnectivityClientSpec() *ApexConnectivityClientSpec {
	return &cr.Spec
}

// GetClientType - Returns the client type
func (cr *ApexConnectivityClient) GetClientType() ClientType {
	return cr.Spec.Client.CSMClientType
}

// IsBeingDeleted  - Returns  true if a deletion timestamp is set
func (cr *ApexConnectivityClient) IsBeingDeleted() bool {
	return !cr.ObjectMeta.DeletionTimestamp.IsZero()
}

// HasFinalizer returns true if the item has the specified finalizer
func (cr *ApexConnectivityClient) HasFinalizer(finalizerName string) bool {
	for _, item := range cr.ObjectMeta.Finalizers {
		if item == finalizerName {
			return true
		}
	}
	return false
}
