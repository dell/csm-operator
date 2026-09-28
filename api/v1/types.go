// Copyright © 2021-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

// UpgradePolicy - type representing the upgrade policy for the CSM CR
type UpgradePolicy string

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
	// CSIAddonsReplication - placeholder for constant
	CSIAddonsReplication = "csi-addons-replication"

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

	// UpgradePolicyAuto enables automatic upgrades to the latest supported version
	UpgradePolicyAuto UpgradePolicy = "auto"
	// UpgradePolicyManual requires manual version changes for upgrades (default)
	UpgradePolicyManual UpgradePolicy = "manual"
)

// Module defines the desired state of a ContainerStorageModule
// +kubebuilder:validation:MaxProperties=11
type Module struct {
	// Name is name of ContainerStorageModule modules
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Name"
	Name ModuleType `json:"name" yaml:"name"`

	// Enabled is used to indicate whether or not to deploy a module
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enabled"
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ConfigVersion is the configuration version of the module
	// Deprecated: Use spec.version instead. This field will be removed in a future release.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config Version"
	// +kubebuilder:validation:Optional
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
	// +kubebuilder:validation:MaxItems=20
	InitContainer []ContainerTemplate `json:"initContainer,omitempty" yaml:"initContainer"`

	// Metrics configures module-level metrics (e.g., for resiliency podmon)
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Module Metrics Configuration"
	Metrics *ModuleMetrics `json:"metrics,omitempty" yaml:"metrics,omitempty"`
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
	// Deprecated: Use spec.version instead. This field will be removed in a future release.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config Version"
	// +kubebuilder:validation:Optional
	ConfigVersion string `json:"configVersion,omitempty" yaml:"configVersion,omitempty"`

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
	// +kubebuilder:validation:MaxItems=20
	InitContainers []ContainerTemplate `json:"initContainers,omitempty" yaml:"initContainers"`

	// SnapshotClass is the specification for Snapshot Classes
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Snapshot Classes"
	SnapshotClass []SnapshotClass `json:"snapshotClass,omitempty" yaml:"snapshotClass"`

	// AuthSecret is the name of the credentials secret for the driver
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Auth Secret"
	AuthSecret string `json:"authSecret,omitempty" yaml:"authSecret"` //gosec:disable G117

	// TLSCertSecret is the name of the TLS Cert secret
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="TLSCert Secret"
	TLSCertSecret string `json:"tlsCertSecret,omitempty" yaml:"tlsCertSecret"`

	// ForceRemoveDriver is the boolean flag used to remove driver deployment when CR is deleted
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Force Remove Driver"
	ForceRemoveDriver *bool `json:"forceRemoveDriver,omitempty" yaml:"forceRemoveDriver"`

	// Metrics is the configuration for the driver metrics endpoint and monitoring
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Driver Metrics Configuration"
	Metrics *DriverMetrics `json:"metrics,omitempty" yaml:"metrics,omitempty"`

	// MetroSiteFailureHandling configures PowerMax Metro site-failure handling
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metro Site Failure Handling Configuration"
	MetroSiteFailureHandling *MetroSiteFailureHandlingConfig `json:"metroSiteFailureHandling,omitempty" yaml:"metroSiteFailureHandling,omitempty"`
}

// DriverMetrics defines the metrics endpoint and monitoring configuration for a driver
type DriverMetrics struct {
	// Enabled enables the shared Prometheus metrics HTTP endpoint on the controller
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics Enabled"
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// Port is the port on which the metrics endpoint is exposed
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics Port"
	Port int32 `json:"port,omitempty" yaml:"port,omitempty"`

	// TLSCertSecret is the name of a Kubernetes TLS Secret used to serve the metrics endpoint over HTTPS
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics TLS Cert Secret"
	TLSCertSecret string `json:"tlsCertSecret,omitempty" yaml:"tlsCertSecret,omitempty"`

	// LeaderElection configures leader election for array-level metrics collection
	// Only the leader controller will collect array-level metrics to avoid duplicate API calls
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics Leader Election Configuration"
	LeaderElection *LeaderElectionConfig `json:"leaderElection,omitempty" yaml:"leaderElection,omitempty"`

	// GatewayMonitoring configures PowerFlex Gateway health monitoring
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Gateway Monitoring Configuration"
	GatewayMonitoring *GatewayMonitoringConfig `json:"gatewayMonitoring,omitempty" yaml:"gatewayMonitoring,omitempty"`

	// Collection configures driver metrics collection cadence and cache behavior
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics Collection Configuration"
	Collection *MetricsCollectionConfig `json:"collection,omitempty" yaml:"collection,omitempty"`

	// Array configures array-facing metrics collection controls
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics Array Configuration"
	Array *MetricsArrayConfig `json:"array,omitempty" yaml:"array,omitempty"`

	// ServiceMonitor configures optional Prometheus Operator ServiceMonitor creation
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ServiceMonitor Configuration"
	ServiceMonitor *MetricsServiceMonitorConfig `json:"serviceMonitor,omitempty" yaml:"serviceMonitor,omitempty"`

	// PodMonitor configures optional Prometheus Operator PodMonitor creation for node pods
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="PodMonitor Configuration"
	PodMonitor *MetricsPodMonitorConfig `json:"podMonitor,omitempty" yaml:"podMonitor,omitempty"`

	// PrometheusRule configures optional Prometheus Operator alert rule creation for supported drivers
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="PrometheusRule Configuration"
	// +optional
	PrometheusRule *MetricsPrometheusRuleConfig `json:"prometheusRule,omitempty" yaml:"prometheusRule,omitempty"`
}

// LeaderElectionConfig configures leader election for array-level metrics collection
type LeaderElectionConfig struct {
	// Enabled enables leader election for array-level metrics collection
	// Only the leader controller will collect array-level metrics to avoid duplicate API calls
	// When nil, defaults to false
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Leader Election Enabled"
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// LeaseDuration is the duration that non-leader candidates will wait to acquire the lease
	// Default value: 60s
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Leader Election Lease Duration"
	LeaseDuration string `json:"leaseDuration,omitempty" yaml:"leaseDuration,omitempty"`

	// RenewDeadline is the duration that the acting leader will retry refreshing leadership before giving up
	// Default value: 40s
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Leader Election Renew Deadline"
	RenewDeadline string `json:"renewDeadline,omitempty" yaml:"renewDeadline,omitempty"`

	// RetryPeriod is the duration the LeaderElector clients should wait between tries of actions
	// Default value: 5s
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Leader Election Retry Period"
	RetryPeriod string `json:"retryPeriod,omitempty" yaml:"retryPeriod,omitempty"`
}

// GatewayMonitoringConfig configures PowerFlex Gateway health monitoring
type GatewayMonitoringConfig struct {
	// Enabled enables Gateway health monitoring on the controller
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Gateway Monitoring Enabled"
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// LeaderElectionEnabled enables leader election so only one controller actively monitors gateways
	// When nil, defaults to true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Gateway Monitoring Leader Election Enabled"
	LeaderElectionEnabled *bool `json:"leaderElectionEnabled,omitempty" yaml:"leaderElectionEnabled,omitempty"`

	// PollInterval is how frequently the leader controller probes each gateway endpoint
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Gateway Monitoring Poll Interval"
	// +kubebuilder:validation:XValidation:rule="self == '' || size(self) > 0",message="pollInterval must be a non-empty string"
	PollInterval string `json:"pollInterval,omitempty" yaml:"pollInterval,omitempty"`
}

// MetricsCollectionConfig configures collection interval and cache TTL for driver metrics.
type MetricsCollectionConfig struct {
	// Interval is how frequently metrics are collected from the array
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics Collection Interval"
	// +kubebuilder:validation:XValidation:rule="self == '' || size(self) > 0",message="interval must be a non-empty string"
	Interval string `json:"interval,omitempty" yaml:"interval,omitempty"`

	// CacheTTL is how long collected metric responses are cached
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics Collection Cache TTL"
	// +kubebuilder:validation:XValidation:rule="self == '' || size(self) > 0",message="cacheTTL must be a non-empty string"
	CacheTTL string `json:"cacheTTL,omitempty" yaml:"cacheTTL,omitempty"`
}

// MetricsArrayConfig configures array request controls for driver metrics collection.
type MetricsArrayConfig struct {
	// RateLimit sets the max allowed array requests per collection window
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics Array Rate Limit"
	// +kubebuilder:validation:Minimum=1
	RateLimit int32 `json:"rateLimit,omitempty" yaml:"rateLimit,omitempty"`

	// Timeout sets the timeout for metrics array requests
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics Array Timeout"
	// +kubebuilder:validation:XValidation:rule="self == '' || size(self) > 0",message="timeout must be a non-empty string"
	Timeout string `json:"timeout,omitempty" yaml:"timeout,omitempty"`

	// CircuitBreaker configures metrics collection circuit breaker controls
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics Circuit Breaker Configuration"
	CircuitBreaker *MetricsCircuitBreakerConfig `json:"circuitBreaker,omitempty" yaml:"circuitBreaker,omitempty"`
}

// MetricsCircuitBreakerConfig configures threshold and reset timeout for the metrics circuit breaker.
type MetricsCircuitBreakerConfig struct {
	// Threshold opens the circuit after this many consecutive failures
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics Circuit Breaker Threshold"
	// +kubebuilder:validation:Minimum=1
	Threshold int32 `json:"threshold,omitempty" yaml:"threshold,omitempty"`

	// ResetTimeout is how long to wait before retrying after opening the circuit
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics Circuit Breaker Reset Timeout"
	// +kubebuilder:validation:XValidation:rule="self == '' || size(self) > 0",message="resetTimeout must be a non-empty string"
	ResetTimeout string `json:"resetTimeout,omitempty" yaml:"resetTimeout,omitempty"`
}

// MetricsServiceMonitorConfig configures Prometheus Operator ServiceMonitor creation
type MetricsServiceMonitorConfig struct {
	// Enabled creates a ServiceMonitor CR for Prometheus Operator integration
	// Requires the Prometheus Operator CRDs to be installed in the cluster
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ServiceMonitor Enabled"
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// Interval is the Prometheus scrape interval for the metrics endpoint
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ServiceMonitor Scrape Interval"
	// +kubebuilder:validation:XValidation:rule="self == '' || size(self) > 0",message="interval must be a non-empty string"
	Interval string `json:"interval,omitempty" yaml:"interval,omitempty"`

	// ScrapeTimeout is the Prometheus scrape timeout for the metrics endpoint
	// Default value: "" (unset; Prometheus uses its global scrape_timeout)
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ServiceMonitor Scrape Timeout"
	// +kubebuilder:validation:XValidation:rule="self == '' || size(self) > 0",message="scrapeTimeout must be a non-empty string"
	ScrapeTimeout string `json:"scrapeTimeout,omitempty" yaml:"scrapeTimeout,omitempty"`

	// InsecureSkipVerify skips TLS certificate verification when Prometheus scrapes the metrics endpoint
	// Only applies when tlsCertSecret is set
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ServiceMonitor Insecure Skip Verify"
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
}

// MetricsPodMonitorConfig configures Prometheus Operator PodMonitor creation for node pods
type MetricsPodMonitorConfig struct {
	// Enabled creates a PodMonitor CR for Prometheus Operator integration targeting node pods
	// Requires the Prometheus Operator CRDs to be installed in the cluster
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="PodMonitor Enabled"
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// Interval is the Prometheus scrape interval for the node metrics endpoint
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="PodMonitor Scrape Interval"
	// +kubebuilder:validation:XValidation:rule="self == '' || size(self) > 0",message="interval must be a non-empty string"
	Interval string `json:"interval,omitempty" yaml:"interval,omitempty"`

	// ScrapeTimeout is the Prometheus scrape timeout for the node metrics endpoint
	// Default value: "" (unset; Prometheus uses its global scrape_timeout)
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="PodMonitor Scrape Timeout"
	// +kubebuilder:validation:XValidation:rule="self == '' || size(self) > 0",message="scrapeTimeout must be a non-empty string"
	ScrapeTimeout string `json:"scrapeTimeout,omitempty" yaml:"scrapeTimeout,omitempty"`

	// InsecureSkipVerify skips TLS certificate verification when Prometheus scrapes the node metrics endpoint
	// Only applies when tlsCertSecret is set
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="PodMonitor Insecure Skip Verify"
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
}

// MetricsPrometheusRuleConfig configures Prometheus Operator alert rule creation for supported drivers.
type MetricsPrometheusRuleConfig struct {
	// Enabled creates a PrometheusRule CR for Prometheus Operator integration.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="PrometheusRule Enabled"
	// +kubebuilder:default=true
	Enabled bool `json:"enabled" yaml:"enabled"`

	// QuotaWarningThreshold is the whole-percent quota utilization threshold that triggers a warning alert for drivers exposing quota metrics.
	// PowerScale uses this field.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Quota Warning Threshold"
	// +kubebuilder:default=80
	QuotaWarningThreshold *int32 `json:"quotaWarningThreshold,omitempty" yaml:"quotaWarningThreshold,omitempty"`

	// QuotaCriticalThreshold is the whole-percent quota utilization threshold that triggers a critical alert for drivers exposing quota metrics.
	// PowerScale uses this field.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Quota Critical Threshold"
	// +kubebuilder:default=90
	QuotaCriticalThreshold *int32 `json:"quotaCriticalThreshold,omitempty" yaml:"quotaCriticalThreshold,omitempty"`

	// NodepoolWarningThreshold is the whole-percent node pool utilization threshold that triggers a warning alert for drivers exposing node pool metrics.
	// PowerScale uses this field.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Node Pool Warning Threshold"
	// +kubebuilder:default=80
	NodepoolWarningThreshold *int32 `json:"nodepoolWarningThreshold,omitempty" yaml:"nodepoolWarningThreshold,omitempty"`

	// NodepoolCriticalThreshold is the whole-percent node pool utilization threshold that triggers a critical alert for drivers exposing node pool metrics.
	// PowerScale uses this field.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Node Pool Critical Threshold"
	// +kubebuilder:default=90
	NodepoolCriticalThreshold *int32 `json:"nodepoolCriticalThreshold,omitempty" yaml:"nodepoolCriticalThreshold,omitempty"`

	// ApplianceWarningThreshold is the whole-percent appliance utilization threshold that triggers a warning alert for drivers exposing appliance metrics.
	// PowerStore uses this field.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Appliance Warning Threshold"
	// +kubebuilder:default=80
	ApplianceWarningThreshold *int32 `json:"applianceWarningThreshold,omitempty" yaml:"applianceWarningThreshold,omitempty"`

	// ApplianceCriticalThreshold is the whole-percent appliance utilization threshold that triggers a critical alert for drivers exposing appliance metrics.
	// PowerStore uses this field.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Appliance Critical Threshold"
	// +kubebuilder:default=90
	ApplianceCriticalThreshold *int32 `json:"applianceCriticalThreshold,omitempty" yaml:"applianceCriticalThreshold,omitempty"`

	// APIErrorRateThreshold is the whole-percent API error threshold that triggers an alert for drivers exposing request success and failure metrics.
	// PowerScale uses this field.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="API Error Rate Threshold"
	// +kubebuilder:default=10
	APIErrorRateThreshold *int32 `json:"apiErrorRateThreshold,omitempty" yaml:"apiErrorRateThreshold,omitempty"`

	// APIErrorRateWindow is the lookback window used when evaluating the API error ratio alert expression.
	// PowerScale uses this field.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="API Error Rate Window"
	// +kubebuilder:default="5m"
	APIErrorRateWindow *string `json:"apiErrorRateWindow,omitempty" yaml:"apiErrorRateWindow,omitempty"`

	// CPUWarningThreshold is the whole-percent CPU utilization threshold that triggers a warning alert for driver pods.
	// PowerScale uses this field.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="CPU Warning Threshold"
	// +kubebuilder:default=80
	CPUWarningThreshold *int32 `json:"cpuWarningThreshold,omitempty" yaml:"cpuWarningThreshold,omitempty"`

	// MemoryWarningThreshold is the driver memory usage threshold in bytes that triggers a warning alert.
	// PowerScale uses this field.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Memory Warning Threshold"
	// +kubebuilder:default=2147483648
	MemoryWarningThreshold *int64 `json:"memoryWarningThreshold,omitempty" yaml:"memoryWarningThreshold,omitempty"`

	// NFSv3LatencyWarningSeconds is the NFSv3 p99 latency threshold in seconds that triggers a warning alert.
	// PowerScale uses this field.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="NFSv3 Latency Warning Seconds"
	// +kubebuilder:default=10
	NFSv3LatencyWarningSeconds *int32 `json:"nfsV3LatencyWarningSeconds,omitempty" yaml:"nfsV3LatencyWarningSeconds,omitempty"`

	// NFSv4LatencyWarningSeconds is the NFSv4 p99 latency threshold in seconds that triggers a warning alert.
	// PowerScale uses this field.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="NFSv4 Latency Warning Seconds"
	// +kubebuilder:default=10
	NFSv4LatencyWarningSeconds *int32 `json:"nfsV4LatencyWarningSeconds,omitempty" yaml:"nfsV4LatencyWarningSeconds,omitempty"`

	// DriverCrashLoopRestartThreshold is the restart count threshold used to detect crash looping within the configured window.
	// PowerScale uses this field.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Driver CrashLoop Restart Threshold"
	// +kubebuilder:default=3
	DriverCrashLoopRestartThreshold *int32 `json:"driverCrashLoopRestartThreshold,omitempty" yaml:"driverCrashLoopRestartThreshold,omitempty"`

	// DriverCrashLoopWindow is the lookback window used when evaluating crash loop restart deltas.
	// PowerScale uses this field.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Driver CrashLoop Window"
	// +kubebuilder:default="15m"
	DriverCrashLoopWindow *string `json:"driverCrashLoopWindow,omitempty" yaml:"driverCrashLoopWindow,omitempty"`

	// RestartCountThreshold is the pod restart count within the 15-minute window that triggers a crash-loop alert.
	// PowerFlex uses this field (PF-10). Default: 3.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Restart Count Threshold"
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	RestartCountThreshold *int32 `json:"restartCountThreshold,omitempty" yaml:"restartCountThreshold,omitempty"`

	// PoolCapacityWarningPercent is the storage pool utilization percentage that triggers a warning alert.
	// PowerFlex uses this field (PF-13). Default: 80.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Pool Capacity Warning Percent"
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=80
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=99
	PoolCapacityWarningPercent *int32 `json:"poolCapacityWarningPercent,omitempty" yaml:"poolCapacityWarningPercent,omitempty"`

	// PoolCapacityCriticalPercent is the storage pool utilization percentage that triggers a critical alert.
	// PowerFlex uses this field (PF-14). Default: 90.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Pool Capacity Critical Percent"
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=90
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	PoolCapacityCriticalPercent *int32 `json:"poolCapacityCriticalPercent,omitempty" yaml:"poolCapacityCriticalPercent,omitempty"`

	// ThinRatioWarningThreshold is the thin-provisioning over-commitment ratio threshold that triggers a warning alert.
	// PowerFlex uses this field (PF-15). Stored as a decimal string to avoid controller-gen float64 rejection. Default: "0.8".
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Thin Ratio Warning Threshold"
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="0.8"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?$`
	ThinRatioWarningThreshold *string `json:"thinRatioWarningThreshold,omitempty" yaml:"thinRatioWarningThreshold,omitempty"`

	// DataReductionDegradedThreshold is the data-reduction ratio below which a degradation alert fires.
	// PowerFlex uses this field (PF-16). Stored as a decimal string to avoid controller-gen float64 rejection. Default: "1.5".
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Data Reduction Degraded Threshold"
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="1.5"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?$`
	DataReductionDegradedThreshold *string `json:"dataReductionDegradedThreshold,omitempty" yaml:"dataReductionDegradedThreshold,omitempty"`

	// RCGLagWarningSeconds is the persistent RCG replication lag in seconds that triggers a warning alert.
	// PowerFlex uses this field (PF-20). Default: 300.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="RCG Lag Warning Seconds"
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=300
	// +kubebuilder:validation:Minimum=1
	RCGLagWarningSeconds *int64 `json:"rcgLagWarningSeconds,omitempty" yaml:"rcgLagWarningSeconds,omitempty"`

	// RCGBandwidthWarningKBps is the RCG transmit bandwidth in KB/s below which a degradation alert fires.
	// PowerFlex uses this field (PF-21). Default: 10240.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="RCG Bandwidth Warning KBps"
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=10240
	// +kubebuilder:validation:Minimum=1
	RCGBandwidthWarningKBps *int64 `json:"rcgBandwidthWarningKBps,omitempty" yaml:"rcgBandwidthWarningKBps,omitempty"`

	// RCGLatencyWarningSeconds is the RCG transmit latency in seconds that triggers a warning alert.
	// PowerFlex uses this field (PF-22). Stored as a decimal string to avoid controller-gen float64 rejection. Default: "5.0".
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="RCG Latency Warning Seconds"
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="5.0"
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?$`
	RCGLatencyWarningSeconds *string `json:"rcgLatencyWarningSeconds,omitempty" yaml:"rcgLatencyWarningSeconds,omitempty"`

	// VolumeOperationFailureThreshold is the minimum number of operation failures within a 5-minute
	// window that triggers a volume-operation alert. All eight volume-operation alerts use this threshold.
	// PowerFlex uses this field (PF-01..08); PowerMax uses this field (PM-01..08); PowerStore uses this field. Default: 3.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Volume Operation Failure Threshold"
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	VolumeOperationFailureThreshold *int32 `json:"volumeOperationFailureThreshold,omitempty" yaml:"volumeOperationFailureThreshold,omitempty"`

	// StorageGroupCapacityWarning is the whole-percent Storage Group utilization threshold that
	// triggers a warning alert. PowerMax uses this field (PM-13).
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Storage Group Capacity Warning Threshold"
	// +kubebuilder:default=80
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=99
	StorageGroupCapacityWarning *int32 `json:"storageGroupCapacityWarning,omitempty" yaml:"storageGroupCapacityWarning,omitempty"`

	// StorageGroupCapacityCritical is the whole-percent Storage Group utilization threshold that
	// triggers a critical alert. PowerMax uses this field (PM-14).
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Storage Group Capacity Critical Threshold"
	// +kubebuilder:default=90
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	StorageGroupCapacityCritical *int32 `json:"storageGroupCapacityCritical,omitempty" yaml:"storageGroupCapacityCritical,omitempty"`

	// SRPSnapshotCapacityWarning is the whole-percent SRP snapshot capacity threshold that triggers
	// a warning alert. PowerMax uses this field (PM-15). SRP-level monitoring is the permanent
	// solution (per-SG snapshot metric is not required per product management decision).
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="SRP Snapshot Capacity Warning Threshold"
	// +kubebuilder:default=80
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	SRPSnapshotCapacityWarning *int32 `json:"srpSnapshotCapacityWarning,omitempty" yaml:"srpSnapshotCapacityWarning,omitempty"`
}

// ModulePrometheusRuleConfig configures Prometheus Operator alert rule creation for supported modules.
type ModulePrometheusRuleConfig struct {
	// Enabled creates a PrometheusRule CR for Prometheus Operator integration.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Module PrometheusRule Enabled"
	// +kubebuilder:default=true
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ConnectivitySuccessRatio is the minimum percentage of successful array connectivity checks
	// below which the RES-02 CSMResiliencyConnectivityLost alert fires.
	// This field is only read when set under spec.modules[name=resiliency].metrics.prometheusRule.
	// Setting it under spec.driver.metrics.prometheusRule has no effect on resiliency alerts.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Connectivity Success Ratio"
	// +kubebuilder:default=90
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	ConnectivitySuccessRatio *int32 `json:"connectivitySuccessRatio,omitempty" yaml:"connectivitySuccessRatio,omitempty"`

	// RPOThresholdSeconds is the replication lag threshold in seconds used by the
	// CSMReplicationLagExceeded (REP-01) and CSMReplicationRPOViolation (REP-02) alerts.
	// Applies when the CSM Replication module is enabled. Default: 300 (5 minutes).
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="RPO Threshold Seconds"
	// +kubebuilder:default=300
	// +kubebuilder:validation:Minimum=30
	// +kubebuilder:validation:Maximum=86400
	RPOThresholdSeconds *int32 `json:"rpoThresholdSeconds,omitempty" yaml:"rpoThresholdSeconds,omitempty"`

	// SRDFMinBandwidth is the minimum acceptable SRDF replication bandwidth in bytes per second.
	// Used by the PowerMaxSRDFBandwidthDegraded (REP-07) alert.
	// Applies when the CSM Replication module is enabled with PowerMax SRDF.
	// Default: 10485760 (10 MB/s).
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="SRDF Minimum Bandwidth"
	// +kubebuilder:default=10485760
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1073741824
	SRDFMinBandwidth *int64 `json:"srdfMinBandwidth,omitempty" yaml:"srdfMinBandwidth,omitempty"`

	// SRDFLagSeconds is the SRDF replication lag threshold in seconds for the
	// PowerMaxSRDFLagExceeded (REP-12) alert.
	// Applies when the CSM Replication module is enabled with PowerMax SRDF.
	// Default: 300 (5 minutes).
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="SRDF Lag Seconds"
	// +kubebuilder:default=300
	// +kubebuilder:validation:Minimum=30
	// +kubebuilder:validation:Maximum=86400
	SRDFLagSeconds *int32 `json:"srdfLagSeconds,omitempty" yaml:"srdfLagSeconds,omitempty"`

	// AuthFailureRateThreshold is the percentage threshold (0-100) for the high CSM Authorization failure-rate alert (P4.AUTH.2).
	// The value is divided by 100 when substituted into Prometheus expressions.
	// This field is only read when set under spec.modules[name=authorization].metrics.prometheusRule.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Failure Rate Threshold"
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	AuthFailureRateThreshold *int32 `json:"authFailureRateThreshold,omitempty" yaml:"authFailureRateThreshold,omitempty"`

	// AuthLatencyQuantile is the quantile percentage (0-100) used for the high CSM Authorization latency alert (P4.AUTH.7).
	// The value is divided by 100 when substituted into Prometheus expressions (e.g., 95 for 95th percentile).
	// This field is only read when set under spec.modules[name=authorization].metrics.prometheusRule.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Latency Quantile"
	// +kubebuilder:default=95
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	AuthLatencyQuantile *int32 `json:"authLatencyQuantile,omitempty" yaml:"authLatencyQuantile,omitempty"`

	// AuthLatencyThresholdSeconds is the latency threshold in seconds for the high CSM Authorization latency alert (P4.AUTH.7).
	// This field is only read when set under spec.modules[name=authorization].metrics.prometheusRule.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Latency Threshold Seconds"
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=1
	AuthLatencyThresholdSeconds *int32 `json:"authLatencyThresholdSeconds,omitempty" yaml:"authLatencyThresholdSeconds,omitempty"`
}

// ModuleMetrics configures module-level metrics (e.g., for resiliency podmon)
type ModuleMetrics struct {
	// Enabled enables the module metrics endpoint
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Module Metrics Enabled"
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// Port is the port on which the metrics endpoint is exposed
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Module Metrics Port"
	Port int32 `json:"port,omitempty" yaml:"port,omitempty"`

	// TLSCertSecret is the name of a Kubernetes TLS Secret (tls.crt / tls.key) used to serve metrics over HTTPS
	// When set the operator mounts the secret and configures HTTPS automatically
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Module Metrics TLS Cert Secret"
	TLSCertSecret string `json:"tlsCertSecret,omitempty" yaml:"tlsCertSecret,omitempty"`

	// Collection configures metrics collection cadence and cache behavior
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Module Metrics Collection Config"
	Collection *MetricsCollectionConfig `json:"collection,omitempty" yaml:"collection,omitempty"`

	// ServiceMonitor configures optional Prometheus Operator ServiceMonitor creation
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Module ServiceMonitor Configuration"
	ServiceMonitor *MetricsServiceMonitorConfig `json:"serviceMonitor,omitempty" yaml:"serviceMonitor,omitempty"`

	// PodMonitor configures optional Prometheus Operator PodMonitor creation for node pods
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Module PodMonitor Configuration"
	PodMonitor *MetricsPodMonitorConfig `json:"podMonitor,omitempty" yaml:"podMonitor,omitempty"`

	// PrometheusRule configures Prometheus Operator alert rule creation for supported modules.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Module PrometheusRule Configuration"
	PrometheusRule *ModulePrometheusRuleConfig `json:"prometheusRule,omitempty" yaml:"prometheusRule,omitempty"`
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
	// +kubebuilder:validation:MaxItems=50
	Envs []corev1.EnvVar `json:"envs,omitempty" yaml:"envs"`

	// Ports is the list of ports for the container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Container Ports"
	Ports []corev1.ContainerPort `json:"ports,omitempty" yaml:"ports"`

	// VolumeMounts is the list of volume mounts for the container
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Container Volume Mounts"
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty" yaml:"volumeMounts"`

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

	// Gateway is the gateway configuration for the authorization proxy-server (v2.5.0+)
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Proxy Server Gateway configuration"
	Gateway *ProxyServerGateway `json:"gateway,omitempty" yaml:"gateway,omitempty"`

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

	// RedisUsername is the username for the redis instance
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Redis Username"
	RedisUsername string `json:"redisUsername,omitempty" yaml:"redisUsername,omitempty"`

	// RedisPassword is the password for the redis instance
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Redis Password"
	RedisPassword string `json:"redisPassword,omitempty" yaml:"redisPassword,omitempty"`

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
	PrivateKey string `json:"privateKey,omitempty" yaml:"privateKey,omitempty"` //gosec:disable G117

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

// ProxyServerGateway is the gateway configuration for the authorization proxy-server (v2.5.0+)
type ProxyServerGateway struct {
	// GatewayClassName is the name of the GatewayClass for the proxy-server Gateway resource
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Proxy Server Gateway Class Name"
	GatewayClassName string `json:"gatewayClassName,omitempty" yaml:"gatewayClassName,omitempty"`

	// Hosts is the additional hostnames for the proxy-server HTTPRoute
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Proxy Server Gateway Hosts"
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`

	// Annotations is an unstructured key value map that stores additional annotations for the proxy-server HTTPRoute
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authorization Proxy Server Gateway Annotations"
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
	ClientKey string `json:"clientKey,omitempty" yaml:"clientKey,omitempty"` //gosec:disable G117

	// CertificateAuthority is the base64-encoded certificate authority for validaitng the vault certificate
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Vault Certificate Authority"
	CertificateAuthority string `json:"certificateAuthority,omitempty" yaml:"certificateAuthority,omitempty"`
}

// MetroSiteFailureHandlingConfig configures PowerMax Metro site-failure handling
type MetroSiteFailureHandlingConfig struct {
	// Enabled enables Metro site-failure handling for PowerMax
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metro Site Failure Handling Enabled"
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// StateCheckTimeoutSeconds is the context timeout for CheckMetroState RDF group queries
	// Must be 5–120.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metro State Check Timeout Seconds"
	// +kubebuilder:default=15
	// +kubebuilder:validation:Minimum=5
	// +kubebuilder:validation:Maximum=120
	StateCheckTimeoutSeconds *int32 `json:"stateCheckTimeoutSeconds,omitempty" yaml:"stateCheckTimeoutSeconds,omitempty"`

	// QueueWarningThreshold is the number of pending deferred operations that triggers a Kubernetes warning event
	// Must be 10–10000.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metro Queue Warning Threshold"
	// +kubebuilder:default=75
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:validation:Maximum=10000
	QueueWarningThreshold *int32 `json:"queueWarningThreshold,omitempty" yaml:"queueWarningThreshold,omitempty"`

	// QueueHardLimit is the maximum number of deferred operations. New deferrals are rejected when this limit is reached
	// Must be 50–10000.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metro Queue Hard Limit"
	// +kubebuilder:default=100
	// +kubebuilder:validation:Minimum=50
	// +kubebuilder:validation:Maximum=10000
	QueueHardLimit *int32 `json:"queueHardLimit,omitempty" yaml:"queueHardLimit,omitempty"`

	// ReconciliationBackoffSeconds is the base backoff (seconds) for retry on reconciliation failures
	// Actual delay = base * 2^retry, capped at 5 minutes. Must be 1–60.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metro Reconciliation Backoff Seconds"
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=60
	ReconciliationBackoffSeconds *int32 `json:"reconciliationBackoffSeconds,omitempty" yaml:"reconciliationBackoffSeconds,omitempty"`
}
