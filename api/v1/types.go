/*

Copyright (c) 2021 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CSMStateType - type representing the state of the CSM (in status)
type CSMStateType string

// CSMOperatorConditionType  defines the type of the last status update
type CSMOperatorConditionType string

// ImageType - represents type of image
type ImageType string

// DriverType - type representing the type of the driver. e.g. - powermax, unity
type DriverType string

// ModuleType - type representing the type of the modules. e.g. - authorization, podmon
type ModuleType string

const (
	// Replication - placeholder for replication constant
	Replication ModuleType = "replication"

	// Observability - placeholder for constant observability
	Observability ModuleType = "observability"

	// PodMon - placeholder for constant podmon
	PodMon ModuleType = "podmon"

	// VgSnapShotter - placeholder for constant vgsnapshotter
	VgSnapShotter ModuleType = "vgsnapshotter"

	// Authorization - placeholder for constant authorization
	Authorization ModuleType = "authorization"

	// ReverseProxy - placeholder for constant csireverseproxy
	ReverseProxy ModuleType = "csireverseproxy"

	// PowerFlex - placeholder for constant powerflex
	PowerFlex DriverType = "powerflex"

	// PowerMax - placeholder for constant powermax
	PowerMax DriverType = "powermax"

	// PowerScale - placeholder for constant PowerScale
	PowerScale DriverType = "powerscale"

	// Unity - placeholder for constant unity
	Unity DriverType = "unity"

	// PowerStore - placeholder for constant powerstore
	PowerStore DriverType = "powerstore"
)

// CSMModule defines the desired state of a CSMModules
type CSMModule struct {
	// Name is name of CSM modules
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Name"
	Name ModuleType `json:"name" yaml:"name"`

	// Enabled is used to indicate wether or not to deploy a module
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enabled"
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ConfigVersion is the configuration version of the driver
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config Version"
	ConfigVersion string `json:"configVersion" yaml:"configVersion"`

	// Components is the specification for SM components containers
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="CSM components specification"
	Components []ContainerTemplate `json:"components,omitempty" yaml:"components,omitempty"`

	// ReverseProxyConfig is the configuration for reverse proxy
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Reverse Proxy Configuration"
	RevProxy RevProxyConfig `json:"revProxyConfig,omitempty" yaml:"revProxyConfig,omitempty"`
}

// PodStatus - Represents a list of PodStatus
type PodStatus struct {
	Available []string `json:"available,omitempty"`
	Ready     []string `json:"ready,omitempty"`
	Starting  []string `json:"starting,omitempty"`
	Stopped   []string `json:"stopped,omitempty"`
}

// LastUpdate - Stores the last update condition for the CSM status
type LastUpdate struct {

	// CSMOperatorCondition is the last known condition of the Custom Resource
	CSMOperatorCondition CSMOperatorConditionType `json:"CSMOperatorCondition,omitempty"`

	// Time is the time stamp for the last condition update
	Time metav1.Time `json:"time,omitempty" yaml:"time"`

	// ErrorMessage is the last error message associated with the condition
	ErrorMessage string `json:"errorMessage,omitempty" yaml:"errorMessage"`
}

// Driver of CSIDriver
// +k8s:openapi-gen=true
type Driver struct {
	// CSIDriverType is the CSI Driver type for Dell EMC - e.g, powermax, powerflex,...
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="CSI Driver Type"
	CSIDriverType DriverType `json:"csiDriverType" yaml:"csiDriverType"`

	// K8sVersion is the k8s varsion
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="k8s Version"
	K8sVersion string `json:"k8sVersion" yaml:"k8sVersion"`

	// VolumeNamePrefix is a string prepended to each volume created by the CSI driver
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Volume Name Prefix"
	VolumeNamePrefix string `json:"k8sVersion" yaml:"k8sVersion"`

	// IsOpenShift a boolean to check if the deployment is intended for openshift
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Is OpenShift"
	IsOpenShift bool `json:"isOpenShift" yaml:"isOpenShift"`

	// ConfigVersion is the configuration version of the driver
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config Version"
	ConfigVersion string `json:"configVersion" yaml:"configVersion"`

	// Replicas is the count of controllers for Controller plugin
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Controller count"
	Replicas int32 `json:"replicas" yaml:"replicas"`

	// DNSPolicy is the dnsPolicy of the daemonset for Node plugin
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="DNSPolicy"
	DNSPolicy string `json:"dnsPolicy,omitempty" yaml:"dnsPolicy"`

	// Common is the common specification for both controller and node plugins
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Common specification"
	Common ContainerTemplate `json:"common" yaml:"common"`

	// Controller is the specification for Controller plugin only
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Controller Specification"
	Controller ContainerTemplate `json:"controller,omitempty" yaml:"controller"`

	// Node is the specification for Node plugin only
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Node specification"
	Node ContainerTemplate `json:"node,omitempty" yaml:"node"`

	// SideCars is the specification for CSI sidecar containers
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="CSI SideCars specification"
	SideCars []ContainerTemplate `json:"sideCars,omitempty" yaml:"sideCars"`

	// InitContainers is the specification for Driver InitContainers
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="InitContainers"
	InitContainers []ContainerTemplate `json:"initContainers,omitempty" yaml:"initContainers"`

	// SnapshotClass is the specification for Snapshot Classes
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Snapshot Classes"
	SnapshotClass []SnapshotClass `json:"snapshotClass,omitempty" yaml:"snapshotClass"`

	// ForceUpdate is the boolean flag used to force an update of the driver instance
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Force update"
	ForceUpdate bool `json:"forceUpdate,omitempty" yaml:"forceUpdate"`

	// AuthSecret is the name of the credentials secret for the driver
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Auth Secret"
	AuthSecret string `json:"authSecret,omitempty" yaml:"authSecret"`

	// TLSCertSecret is the name of the TLS Cert secret
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="TLSCert Secret"
	TLSCertSecret string `json:"tlsCertSecret,omitempty" yaml:"tlsCertSecret"`
}

type ContainerTemplate struct {

	// Name is the name of Container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Container Name"
	Name ImageType `json:"name,omitempty" yaml:"name,omitempty"`

	// Enabled is used to indicate wether or not to deploy a module
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enabled"
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Image is the image tag for the Container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Container Image"
	Image string `json:"image,omitempty" yaml:"image,omitempty"`

	// ImagePullPolicy is the image pull policy for the image
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Container Image Pull Policy",xDescriptors="urn:alm:descriptor:com.tectonic.ui:imagePullPolicy"
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Args is the set of arguments for the container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Container Arguments"
	Args []string `json:"args,omitempty" yaml:"args"`

	// Envs is the set of environment variables for the container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Container Environment vars"
	Envs []corev1.EnvVar `json:"envs,omitempty" yaml:"envs"`

	// Tolerations is the list of tolerations for the driver pods
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Tolerations"
	Tolerations []corev1.Toleration `json:"tolerations,omitempty" yaml:"tolerations"`

	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="NodeSelector"
	NodeSelector map[string]string `json:"nodeSelector,omitempty" yaml:"nodeSelector"`
}

type SnapshotClass struct {
	// Name is the name of the Snapshot Class
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Snapshot Class Name"
	Name string `json:"name" yaml:"name"`

	// Parameters is a map of driver specific parameters for snapshot class
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Snapshot Class Parameters"
	Parameters map[string]string `json:"parameters,omitempty" yaml:"parameters"`
}
