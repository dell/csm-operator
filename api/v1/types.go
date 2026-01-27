//  Copyright © 2021 - 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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

// ObservabilityComponentType - type representing the type of components inside observability module. e.g. - otel-collector
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
	ReverseProxyServer ModuleType = "csipowermax-reverseproxy" // #nosec G101

	// Topology - placeholder for constant topology
	Topology ObservabilityComponentType = "topology"

	// OtelCollector - placeholder for constant otel-collector
	OtelCollector ObservabilityComponentType = "otel-collector"

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

	// Cosi - placeholder for constant cosi
	Cosi DriverType = "cosi"

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
// +kubebuilder:validation:MaxProperties=10
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
	// +kubebuilder:validation:MaxItems=20
	Components []ContainerTemplate `json:"components,omitempty" yaml:"components,omitempty"`

	// ForceRemoveModule is the boolean flag used to remove authorization proxy server deployment when CR is deleted
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Force Remove Module"
	ForceRemoveModule bool `json:"forceRemoveModule,omitempty" yaml:"forceRemoveModule"`

	// InitContainer is the specification for Module InitContainer
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="InitContainer"
	InitContainer []ContainerTemplate `json:"initContainer,omitempty" yaml:"initContainer"`
}

// PodStatus - Represents PodStatus in a daemonset or deployment
type PodStatus struct {
	// Available is the number of available pods
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Available",xDescriptors="urn:alm:descriptor:text"
	Available string `json:"available,omitempty"`

	// Desired is the number of desired pods
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Desired",xDescriptors="urn:alm:descriptor:text"
	Desired string `json:"desired,omitempty"`

	// Failed is the number of failed pods
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Failed",xDescriptors="urn:alm:descriptor:text"
	Failed string `json:"failed,omitempty"`
}

// Driver of CSIDriver
// +k8s:openapi-gen=true
// +kubebuilder:validation:MaxProperties=20
type Driver struct {
	// CSIDriverType is the CSI Driver type for Dell Technologies - e.g, powermax, powerflex,...
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="CSI Driver Type"
	CSIDriverType DriverType `json:"csiDriverType" yaml:"csiDriverType"`

	// CSIDriverSpec is the specification for CSIDriver
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="CSI Driver Spec"
	CSIDriverSpec *CSIDriverSpec `json:"csiDriverSpec" yaml:"csiDriverSpec"`

	// ConfigVersion is the configuration version of the driver
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config Version"
	ConfigVersion string `json:"configVersion" yaml:"configVersion"`

	// Replicas is the count of controllers for Controller plugin
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Controller count"
	// +kubebuilder:default=2
	Replicas int32 `json:"replicas" yaml:"replicas"`

	// DNSPolicy is the dnsPolicy of the daemonset for Node plugin
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="DNSPolicy"
	DNSPolicy string `json:"dnsPolicy,omitempty" yaml:"dnsPolicy"`

	// Common is the common specification for both controller and node plugins
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Common specification"
	Common *ContainerTemplate `json:"common" yaml:"common"`

	// Controller is the specification for Controller plugin only
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Controller Specification"
	Controller *ContainerTemplate `json:"controller,omitempty" yaml:"controller"`

	// Node is the specification for Node plugin only
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Node specification"
	Node *ContainerTemplate `json:"node,omitempty" yaml:"node"`

	// SideCars is the specification for CSI sidecar containers
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="CSI SideCars specification"
	// +kubebuilder:validation:MaxItems=20
	SideCars []ContainerTemplate `json:"sideCars,omitempty" yaml:"sideCars"`

	// InitContainers is the specification for Driver InitContainers
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="InitContainers"
	InitContainers []ContainerTemplate `json:"initContainers,omitempty" yaml:"initContainers"`

	// SnapshotClass is the specification for Snapshot Classes
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Snapshot Classes"
	SnapshotClass []SnapshotClass `json:"snapshotClass,omitempty" yaml:"snapshotClass"`

	// AuthSecret is the name of the credentials secret for the driver
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Auth Secret"
	AuthSecret string `json:"authSecret,omitempty" yaml:"authSecret"`

	// TLSCertSecret is the name of the TLS Cert secret
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="TLSCert Secret"
	TLSCertSecret string `json:"tlsCertSecret,omitempty" yaml:"tlsCertSecret"`

	// ForceRemoveDriver is the boolean flag used to remove driver deployment when CR is deleted
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Force Remove Driver"
	ForceRemoveDriver *bool `json:"forceRemoveDriver,omitempty" yaml:"forceRemoveDriver"`
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
	// +kubebuilder:validation:MaxItems=30
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

	// ProxyServiceReplicas is the number of replicas for the proxy service deployment
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Proxy Service Replicas"
	ProxyServiceReplicas int `json:"proxyServiceReplicas,omitempty" yaml:"proxyServiceReplicas,omitempty"`

	// TenantService is the image tag for the Container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Tenant Service Container Image"
	TenantService string `json:"tenantService,omitempty" yaml:"tenantService,omitempty"`

	// TenantServiceReplicas is the number of replicas for the tenant service deployment
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Tenant Service Replicas"
	TenantServiceReplicas int `json:"tenantServiceReplicas,omitempty" yaml:"tenantServiceReplicas,omitempty"`

	// RoleService is the image tag for the Container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Role Service Container Image"
	RoleService string `json:"roleService,omitempty" yaml:"roleService,omitempty"`

	// RoleServiceReplicas is the number of replicas for the role service deployment
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Role Service Replicas"
	RoleServiceReplicas int `json:"roleServiceReplicas,omitempty" yaml:"roleServiceReplicas,omitempty"`

	// StorageService is the image tag for the Container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Storage Service Container Image"
	StorageService string `json:"storageService,omitempty" yaml:"storageService,omitempty"`

	// StorageServiceReplicas is the number of replicas for storage service deployment
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Storage Service Replicas"
	StorageServiceReplicas int `json:"storageServiceReplicas,omitempty" yaml:"storageServiceReplicas,omitempty"`

	// AuthorizationController is the image tag for the container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Controller Container Image"
	AuthorizationController string `json:"authorizationController,omitempty" yaml:"authorizationController,omitempty"`

	// AuthorizationControllerReplicas is the number of replicas for the authorization controller deployment
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Controller Replicas"
	AuthorizationControllerReplicas int `json:"authorizationControllerReplicas,omitempty" yaml:"authorizationControllerReplicas,omitempty"`

	// LeaderElection is boolean flag to enable leader election
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Leader Election"
	LeaderElection bool `json:"leaderElection,omitempty" yaml:"leaderElection,omitempty"`

	// OpenTelemetryCollectorAddress is the address of the OTLP receiving endpoint using gRPC
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="OpenTelemetry Collector Address of the OTLP endpoint using gRPC"
	OpenTelemetryCollectorAddress string `json:"openTelemetryCollectorAddress,omitempty" yaml:"openTelemetryCollectorAddress,omitempty"`

	// The interval which the reconcile of each controller is run
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Controller Reconcile Interval"
	ControllerReconcileInterval string `json:"controllerReconcileInterval,omitempty" yaml:"controllerReconcileInterval,omitempty"`

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

	// Hostname is the authorization proxy server hostname
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Proxy Server Hostname"
	Hostname string `json:"hostname,omitempty" yaml:"hostname,omitempty"`

	// ProxyServerIngress is the authorization proxy server ingress configuration
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Proxy Server ingress configuration"
	ProxyServerIngress []ProxyServerIngress `json:"proxyServerIngress,omitempty" yaml:"proxyServerIngress,omitempty"`

	// RedisStorageClass is the authorization proxy server redis storage class for persistence
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Proxy Server Redis storage class"
	RedisStorageClass string `json:"storageclass,omitempty" yaml:"storageclass,omitempty"`

	// Vaults are the vault configurations
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Vault Configurations"
	// Applicable till CSM v1.14
	Vaults []Vault `json:"vaultConfigurations,omitempty" yaml:"vaultConfigurations,omitempty"`

	// SecretProviderClasses is a collection of secret provider classes for retrieving secrets from external providers for storage system credentials
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Secret Provider Classes"
	// Applicable from CSM v1.15 onwards
	// Only one of SecretProviderClasses or Secrets must be specified (mutually exclusive)
	SecretProviderClasses *StorageSystemSecretProviderClasses `json:"secretProviderClasses,omitempty" yaml:"secretProviderClasses,omitempty"`

	// Secrets is a collection of kubernetes secrets for storage system credentials
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Secrets"
	// Applicable from CSM v1.15 onwards
	// Only one of SecretProviderClasses or Secrets must be specified (mutually exclusive)
	Secrets []string `json:"secrets,omitempty" yaml:"secrets,omitempty"`

	// skipCertificateValidation is the flag to skip certificate validation
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Skip Certificate Validation"
	SkipCertificateValidation bool `json:"skipCertificateValidation,omitempty" yaml:"skipCertificateValidation,omitempty"`

	// RedisName is the name of the redis statefulset
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Redis StatefulSet Name"
	RedisName string `json:"redisName,omitempty" yaml:"redisName,omitempty"`

	// RedisCommander is the name of the redis deployment
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Redis Deployment Name"
	RedisCommander string `json:"redisCommander,omitempty" yaml:"redisCommander,omitempty"`

	// RedisReplicas is the number of replicas for the redis deployment
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Redis Deployment Replicas"
	RedisReplicas int `json:"redisReplicas,omitempty" yaml:"redisReplicas,omitempty"`

	// Sentinel is the name of the sentinel statefulSet
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Sentinel StatefulSet Name"
	Sentinel string `json:"sentinel,omitempty" yaml:"sentinel,omitempty"`

	// RedisSecretProviderClass is the SecretProviderClass Object details for redis
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Redis SecretProviderClass details"
	// Applicable from CSM v1.15 onwards
	// +kubebuilder:validation:MaxItems=1
	RedisSecretProviderClass []RedisSecretProviderClass `json:"redisSecretProviderClass,omitempty" yaml:"redisSecretProviderClass,omitempty"`

	// ConfigSecretProviderClass is the SecretProviderClass Object details for config secret
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config SecretProviderClass details"
	// Applicable from CSM v1.15 onwards
	// +kubebuilder:validation:MaxItems=1
	ConfigSecretProviderClass []ConfigSecretProviderClass `json:"configSecretProviderClass,omitempty" yaml:"configSecretProviderClass,omitempty"`

	// Certificate is a certificate used for a certificate/private-key pair
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Certificate for certificate/private-key pair"
	Certificate string `json:"certificate,omitempty" yaml:"certificate,omitempty"`

	// PrivateKey is a private key used for a certificate/private-key pair
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Private key for certificate/private-key pair"
	PrivateKey string `json:"privateKey,omitempty" yaml:"privateKey,omitempty"`

	// CertificateAuthority is a certificate authority used to validate a certificate
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Certificate authority for validating a certificate"
	CertificateAuthority string `json:"certificateAuthority,omitempty" yaml:"certificateAuthority,omitempty"`
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

// ProxyServerIngress is the authorization ingress configuration struct
type ProxyServerIngress struct {
	// IngressClassName is the ingressClassName
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Proxy Server Ingress Class Name"
	IngressClassName string `json:"ingressClassName,omitempty" yaml:"ingressClassName,omitempty"`

	// Hosts is the hosts rules for the ingress
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Proxy Server Hosts"
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`

	// Annotations is an unstructured key value map that stores additional annotations for the ingress
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Proxy Server Annotations"
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

// RedisSecretProviderClass is the redis secret configuration for CSM Authorization
type RedisSecretProviderClass struct {
	// SecretProviderClassName is the name of the SecretProviderClass that holds the Redis secretObject
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Redis Secret Provider Class Name"
	SecretProviderClassName string `json:"secretProviderClassName,omitempty" yaml:"secretProviderClassName,omitempty"`

	// RedisSecretName is the name of the Kubernetes secret created by the CSI driver
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Redis Secret Name"
	RedisSecretName string `json:"redisSecretName,omitempty" yaml:"redisSecretName,omitempty"`

	// RedisUsernameKey is the key in the secret that holds the Redis username
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Redis Username Key"
	RedisUsernameKey string `json:"redisUsernameKey,omitempty" yaml:"redisUsernameKey,omitempty"`

	// RedisPasswordKey is the key in the secret that holds the Redis password
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Redis Password Key"
	RedisPasswordKey string `json:"redisPasswordKey,omitempty" yaml:"redisPasswordKey,omitempty"`

	// Conjur is the secret configuration with path to retrieve the Redis credentials from Conjur
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Conjur Secret Configuration"
	Conjur *ConjurCredentialPath `json:"conjur,omitempty" yaml:"conjur,omitempty"`
}

// ConfigSecretProviderClass is the config secret configuration for CSM Authorization
type ConfigSecretProviderClass struct {
	// SecretProviderClassName is the name of the SecretProviderClass that holds the config secretObject
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config Secret Provider Class Name"
	SecretProviderClassName string `json:"secretProviderClassName,omitempty" yaml:"secretProviderClassName,omitempty"`

	// ConfigSecretName is the name of the Kubernetes secret created by the CSI driver
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config Secret Name"
	ConfigSecretName string `json:"configSecretName,omitempty" yaml:"configSecretName,omitempty"`

	// Conjur is the secret configuration with path to retrieve the Config secret from Conjur
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Conjur Config Secret Configuration"
	Conjur *ConjurConfigPath `json:"conjur,omitempty" yaml:"conjur,omitempty"`
}

// StorageSystemSecretProviderClass is a collection of secret provider classes for retrieving secrets from external providers for storage system credentials
type StorageSystemSecretProviderClasses struct {
	// Vault is the list SecretProviderClass names provided by Vault
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Vault SecretProviderClass Names"
	Vaults []string `json:"vault,omitempty" yaml:"vault,omitempty"`

	// Conjur is the list SecretProviderClass names provided by Conjur
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Conjur SecretProviderClasses"
	Conjurs []ConjurSecretProviderClass `json:"conjur,omitempty" yaml:"conjur,omitempty"`
}

type ConjurSecretProviderClass struct {
	// Name is the name of the Conjur SecretProviderClass
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Conjur SecretProviderClass Name"
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Paths is the list of paths to the secrets in Conjur
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Conjur Credential Paths"
	Paths []ConjurCredentialPath `json:"paths,omitempty" yaml:"paths,omitempty"`
}

type ConjurCredentialPath struct {
	// UsernamePath is the path to the username in the secret
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Conjur Username Path"
	UsernamePath string `json:"usernamePath,omitempty" yaml:"usernamePath,omitempty"`

	// PasswordPath is the path to the password in the secret
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Conjur Password Path"
	PasswordPath string `json:"passwordPath,omitempty" yaml:"passwordPath,omitempty"`
}

type ConjurConfigPath struct {
	// SecretPath is the path to the config secret
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Conjur Secret Path"
	SecretPath string `json:"secretPath,omitempty" yaml:"secretPath,omitempty"`
}

// CSIDriverSpec struct
type CSIDriverSpec struct {
	FSGroupPolicy   string `json:"fSGroupPolicy,omitempty" yaml:"fSGroupPolicy,omitempty"`
	StorageCapacity bool   `json:"storageCapacity,omitempty" yaml:"storageCapacity"`
}

// Vault is the configuration for a vault instance struct
type Vault struct {
	// Identifier is the identifier for this vault
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Vault Identifier"
	Identifier string `json:"identifier,omitempty" yaml:"identifier,omitempty"`

	// Address is the address for this vault
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Vault Address"
	Address string `json:"address,omitempty" yaml:"address,omitempty"`

	// Role is the role for this vault
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Vault Role"
	Role string `json:"role,omitempty" yaml:"role,omitempty"`

	// SkipCertificateValidation validates the vault server certificate or not
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Vault Skip Certificate Validation"
	SkipCertificateValidation bool `json:"skipCertificateValidation,omitempty" yaml:"skipCertificateValidation,omitempty"`

	// ClientCertificate is the base64-encoded certificate for connecting to vault
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Vault CLient Certificate"
	ClientCertificate string `json:"clientCertificate,omitempty" yaml:"clientCertificate,omitempty"`

	// ClientKey validates is the base64-encoded certificate key for connecting to vault
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Vault CLient Certificate Key"
	ClientKey string `json:"clientKey,omitempty" yaml:"clientKey,omitempty"`

	// CertificateAuthority is the base64-encoded certificate authority for validaitng the vault certificate
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Vault Certificate Authority"
	CertificateAuthority string `json:"certificateAuthority,omitempty" yaml:"certificateAuthority,omitempty"`
}
