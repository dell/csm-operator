/*

Copyright (c) 2021 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ContainerStorageModuleSpec defines the desired state of ContainerStorageModule
type ContainerStorageModuleSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Driver is a CSI Drivers for Dell EMC
	Driver Driver `json:"driver,omitempty" yaml:"driver,omitempty"`

	// ContainerStorageModuleModules is list of ContainerStorageModule Modules you want to deploy
	Modules []ContainerStorageModuleModule `json:"modules,omitempty" yaml:"modules,omitempty"`
}

// ContainerStorageModuleStatus defines the observed state of ContainerStorageModule
type ContainerStorageModuleStatus struct {
	// ControllerStatus is the status of Controller pods
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="ControllerStatus",xDescriptors="urn:alm:descriptor:com.tectonic.ui:podStatuses"
	ControllerStatus PodStatus `json:"controllerStatus,omitempty"`

	// NodeStatus is the status of Controller pods
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="NodeStatus",xDescriptors="urn:alm:descriptor:com.tectonic.ui:podStatuses"
	NodeStatus PodStatus `json:"nodeStatus,omitempty"`

	// ContainerStorageModuleHash is a hash of the driver specification
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="ContainerStorageModuleHash",xDescriptors="urn:alm:descriptor:text"
	ContainerStorageModuleHash uint64 `json:"csmHash,omitempty" yaml:"csmHash"`

	// State is the state of the driver installation
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="State",xDescriptors="urn:alm:descriptor:text"
	State ContainerStorageModuleStateType `json:"state,omitempty" yaml:"state"`

	// LastUpdate is the last updated state of the driver
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="LastUpdate"
	LastUpdate LastUpdate `json:"lastUpdate,omitempty" yaml:"lastUpdate"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ContainerStorageModule is the Schema for the csms API
type ContainerStorageModule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContainerStorageModuleSpec   `json:"spec,omitempty"`
	Status ContainerStorageModuleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ContainerStorageModuleList contains a list of ContainerStorageModule
type ContainerStorageModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContainerStorageModule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ContainerStorageModule{}, &ContainerStorageModuleList{})
}
