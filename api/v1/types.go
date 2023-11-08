//  Copyright © 2021 - 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	corev1 "k8s.io/api/core/v1"
)

// CSMStateType - type representing the state of the ContainerStorageModule (in status)
type CSMStateType string

// CSMOperatorConditionType  defines the type of the last status update
type CSMOperatorConditionType string

// ImageType - represents type of image
type ImageType string

// DriverType - type representing the type of the driver. e.g. - powermax, unity
type DriverType string

// ModuleType - type representing the type of the modules. e.g. - authorization, podmon
type ModuleType string

// ObservabilityComponentType - type representing the type of components inside observability module. e.g. - topology
type ObservabilityComponentType string

// ClientType - the type of the client
type ClientType string

const (
	// Replication - placeholder for replication constant
	Replication ModuleType = "replication"

	// Resiliency - placeholder for resiliency constant
	Resiliency ModuleType = "resiliency"

	// Observability - placeholder for constant observability
	Observability ModuleType = "observability"

	// PodMon - placeholder for constant podmon
	PodMon ModuleType = "podmon"

	// VgSnapShotter - placeholder for constant vgsnapshotter
	VgSnapShotter ModuleType = "vgsnapshotter"

	// Authorization - placeholder for constant authorization
	Authorization ModuleType = "authorization"

	// AuthorizationServer - placeholder for constant authorization proxy server
	AuthorizationServer ModuleType = "authorization-proxy-server"

	// ReverseProxy - placeholder for constant csireverseproxy
	ReverseProxy ModuleType = "csireverseproxy"

	// ReverseProxyServer - placeholder for constant csipowermax-reverseproxy
	ReverseProxyServer ModuleType = "csipowermax-reverseproxy"

	// Topology - placeholder for constant topology
	Topology ObservabilityComponentType = "topology"

	// PowerFlex - placeholder for constant powerflex
	PowerFlex DriverType = "powerflex"

	// PowerFlexName - placeholder for constant powerflex
	PowerFlexName DriverType = "vxflexos"

	// PowerMax - placeholder for constant powermax
	PowerMax DriverType = "powermax"

	// PowerScale - placeholder for constant isilon
	PowerScale DriverType = "isilon"

	// PowerScaleName - placeholder for constant PowerScale
	PowerScaleName DriverType = "powerscale"

	// Unity - placeholder for constant unity
	Unity DriverType = "unity"

	// PowerStore - placeholder for constant powerstore
	PowerStore DriverType = "powerstore"

	// DreadnoughtClient - placeholder for the APEX Connectivity Client
	DreadnoughtClient ClientType = "apexconnectivityclient"

	// Provisioner - placeholder for constant
	Provisioner = "provisioner"
	// Attacher - placeholder for constant
	Attacher = "attacher"
	// Snapshotter - placeholder for constant
	Snapshotter = "snapshotter"
	// Registrar - placeholder for constant
	Registrar = "registrar"
	// Resizer - placeholder for constant
	Resizer = "resizer"
	// Sdcmonitor - placeholder for constant
	Sdcmonitor = "sdc-monitor"
	// Externalhealthmonitor - placeholder for constant
	Externalhealthmonitor = "external-health-monitor"
	// Sdc - placeholder for constant
	Sdc = "sdc"

	// EventDeleted - Deleted in event recorder
	EventDeleted = "Deleted"
	// EventUpdated - Updated in event recorder
	EventUpdated = "Updated"
	// EventCompleted - Completed in event recorder
	EventCompleted = "Completed"

	// Succeeded - constant
	Succeeded CSMOperatorConditionType = "Succeeded"
	// InvalidConfig - constant
	InvalidConfig CSMOperatorConditionType = "InvalidConfig"
	// Running - Constant
	Running CSMOperatorConditionType = "Running"
	// Error - Constant
	Error CSMOperatorConditionType = "Error"
	// Updating - Constant
	Updating CSMOperatorConditionType = "Updating"
	// Failed - Constant
	Failed CSMOperatorConditionType = "Failed"
)

// Module defines the desired state of a ContainerStorageModule
type Module struct {
	// Name is name of ContainerStorageModule modules
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Name"
	Name ModuleType `json:"name" yaml:"name"`

	// Enabled is used to indicate whether or not to deploy a module
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enabled"
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ConfigVersion is the configuration version of the module
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config Version"
	ConfigVersion string `json:"configVersion,omitempty" yaml:"configVersion,omitempty"`

	// Components is the specification for CSM components containers
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ContainerStorageModule components specification"
	Components []ContainerTemplate `json:"components,omitempty" yaml:"components,omitempty"`

	// ForceRemoveModule is the boolean flag used to remove authorization proxy server deployment when CR is deleted
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Force Remove Module"
	ForceRemoveModule bool `json:"forceRemoveModule,omitempty" yaml:"forceRemoveModule"`
}

// PodStatus - Represents PodStatus in a daemonset or deployment
type PodStatus struct {
	Available string `json:"available,omitempty"`
	Desired   string `json:"desired,omitempty"`
	Failed    string `json:"failed,omitempty"`
}

// Driver of CSIDriver
// +k8s:openapi-gen=true
type Driver struct {
	// CSIDriverType is the CSI Driver type for Dell Technologies - e.g, powermax, powerflex,...
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="CSI Driver Type"
	CSIDriverType DriverType `json:"csiDriverType" yaml:"csiDriverType"`

	// CSIDriverSpec is the specification for CSIDriver
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="CSI Driver Spec"
	CSIDriverSpec CSIDriverSpec `json:"csiDriverSpec" yaml:"csiDriverSpec"`

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

	// ForceRemoveDriver is the boolean flag used to remove driver deployment when CR is deleted
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Force Remove Driver"
	ForceRemoveDriver bool `json:"forceRemoveDriver,omitempty" yaml:"forceRemoveDriver"`
}

// Client - APEX Connectivity Client deployment info
// +k8s:openapi-gen=true
type Client struct {
	// ClientType is the Client type for Dell Technologies - e.g, ApexConnectivityClient
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Client Type"
	CSMClientType ClientType `json:"csmClientType" yaml:"csmClientType"`

	// ConfigVersion is the configuration version of the client
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config Version"
	ConfigVersion string `json:"configVersion" yaml:"configVersion"`

	// Common is the common specification for both controller and node plugins
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Common specification"
	Common ContainerTemplate `json:"common" yaml:"common"`

	// SideCars is the specification for CSI sidecar containers
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="CSI SideCars specification"
	SideCars []ContainerTemplate `json:"sideCars,omitempty" yaml:"sideCars"`

	// InitContainers is the specification for Driver InitContainers
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="InitContainers"
	InitContainers []ContainerTemplate `json:"initContainers,omitempty" yaml:"initContainers"`

	// ForceRemoveClient is the boolean flag used to remove client deployment when CR is deleted
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Force Remove Client"
	ForceRemoveClient bool `json:"forceRemoveClient,omitempty" yaml:"forceRemoveClient"`

	// ConnectionTarget is the target that the client connects to in the Dell datacenter
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Connection Target"
	ConnectionTarget string `json:"connectionTarget,omitempty" yaml:"connectionTarget"`

	// UsePrivateCaCerts is used to specify private CA signed certs
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Use Private CA Certs"
	UsePrivateCaCerts bool `json:"usePrivateCaCerts,omitempty" yaml:"usePrivateCaCerts"`
}

// ContainerTemplate template
type ContainerTemplate struct {
	// Name is the name of Container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Container Name"
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Enabled is used to indicate wether or not to deploy a module
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enabled"
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// Image is the image tag for the Container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Container Image"
	Image ImageType `json:"image,omitempty" yaml:"image,omitempty"`

	// ImagePullPolicy is the image pull policy for the image
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Container Image Pull Policy",xDescriptors="urn:alm:descriptor:com.tectonic.ui:imagePullPolicy"
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`

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

	// ProxyService is the image tag for the Container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Proxy Service Container Image"
	ProxyService string `json:"proxyService,omitempty" yaml:"proxyService,omitempty"`

	// TenantService is the image tag for the Container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Tenant Service Container Image"
	TenantService string `json:"tenantService,omitempty" yaml:"tenantService,omitempty"`

	// RoleService is the image tag for the Container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Role Service Container Image"
	RoleService string `json:"roleService,omitempty" yaml:"roleService,omitempty"`

	// StorageService is the image tag for the Container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Storage Service Container Image"
	StorageService string `json:"storageService,omitempty" yaml:"storageService,omitempty"`

	// Redis is the image tag for the Container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Redis Container Image"
	Redis string `json:"redis,omitempty" yaml:"redis,omitempty"`

	// Commander is the image tag for the Container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Commander Container Image"
	Commander string `json:"commander,omitempty" yaml:"commander,omitempty"`

	// Opa is the image tag for the Container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Opa Container Image"
	Opa string `json:"opa,omitempty" yaml:"opa,omitempty"`

	// OpaKubeMgmt is the image tag for the Container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Opa Kube Management Container Image"
	OpaKubeMgmt string `json:"opaKubeMgmt,omitempty" yaml:"opaKubeMgmt,omitempty"`
}

// SnapshotClass struct
type SnapshotClass struct {
	// Name is the name of the Snapshot Class
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Snapshot Class Name"
	Name string `json:"name" yaml:"name"`

	// Parameters is a map of driver specific parameters for snapshot class
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Snapshot Class Parameters"
	Parameters map[string]string `json:"parameters,omitempty" yaml:"parameters"`
}

// CSIDriverSpec struct
type CSIDriverSpec struct {
	FSGroupPolicy   string `json:"fSGroupPolicy,omitempty" yaml:"fSGroupPolicy,omitempty"`
	StorageCapacity bool   `json:"storageCapacity,omitempty" yaml:"storageCapacity"`
}
