// Copyright (c) Dell Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package modules

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/constants"
	drivers "github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/logger"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	"github.com/dell/csm-operator/pkg/resources/deployment"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	confv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// Test seams (overridden in unit tests)
var (
	getObservabilityModuleFn = getObservabilityModule
	readConfigFileFn         = readConfigFile
	getVersionFn             = operatorutils.GetVersion
)

const (
	// ObservabilityOtelCollectorName - component otel-collector
	ObservabilityOtelCollectorName string = "otel-collector"

	// ObservabilityTopologyName - component topology
	ObservabilityTopologyName string = "topology"

	// ObservabilityCertManagerComponent cert-manager component name
	ObservabilityCertManagerComponent string = "cert-manager"

	// ObservabilityMetricsPowerScaleName - component metrics-powerscale
	ObservabilityMetricsPowerScaleName string = "metrics-powerscale"

	// ObservabilityMetricsPowerFlexName - component metrics-powerflex
	ObservabilityMetricsPowerFlexName string = "metrics-powerflex"

	// ObservabilityMetricsPowerMaxName - component metrics-powermax
	ObservabilityMetricsPowerMaxName string = "metrics-powermax"

	// ObservabilityMetricsPowerStoreName - component metrics-powerstore
	ObservabilityMetricsPowerStoreName string = "metrics-powerstore"

	// TopologyLogLevel -
	TopologyLogLevel string = "<TOPOLOGY_LOG_LEVEL>"

	// TopologyYamlFile -
	TopologyYamlFile string = "karavi-topology.yaml"

	// OtelCollectorAddress - Otel collector address
	OtelCollectorAddress string = "<COLLECTOR_ADDRESS>"

	// PowerScaleMaxConcurrentQueries - max concurrent queries
	PowerScaleMaxConcurrentQueries string = "<POWERSCALE_MAX_CONCURRENT_QUERIES>"

	// PowerscaleCapacityMetricsEnabled - enable/disable collection of capacity metrics
	PowerscaleCapacityMetricsEnabled string = "<POWERSCALE_CAPACITY_METRICS_ENABLED>"

	// PowerscalePerformanceMetricsEnabled - enable/disable collection of performance metrics
	PowerscalePerformanceMetricsEnabled string = "<POWERSCALE_PERFORMANCE_METRICS_ENABLED>"

	// PowerscaleTopologyMetricsEnabled - enable/disable collection of topology metrics
	PowerscaleTopologyMetricsEnabled string = "<POWERSCALE_TOPOLOGY_METRICS_ENABLED>"

	// PowerscaleTopologyMetricsPollFrequency - polling frequency to get topology metrics data
	PowerscaleTopologyMetricsPollFrequency string = "<POWERSCALE_TOPOLOGY_METRICS_POLL_FREQUENCY>"

	// PowerscaleClusterCapacityPollFrequency - polling frequency to get cluster capacity data
	PowerscaleClusterCapacityPollFrequency string = "<POWERSCALE_CLUSTER_CAPACITY_POLL_FREQUENCY>"

	// PowerscaleClusterPerformancePollFrequency - polling frequency to get cluster performance data
	PowerscaleClusterPerformancePollFrequency string = "<POWERSCALE_CLUSTER_PERFORMANCE_POLL_FREQUENCY>"

	// PowerscaleQuotaCapacityPollFrequency - polling frequency to get Quota capacity data
	PowerscaleQuotaCapacityPollFrequency string = "<POWERSCALE_QUOTA_CAPACITY_POLL_FREQUENCY>"

	// IsiclientInsecure - skip certificate validation
	IsiclientInsecure string = "<ISICLIENT_INSECURE>"

	// IsiclientAuthType - enables session-based/basic authentication
	IsiclientAuthType string = "<ISICLIENT_AUTH_TYPE>"

	// IsiclientVerbose - content of the OneFS REST API message
	IsiclientVerbose string = "<ISICLIENT_VERBOSE>"

	// PowerscaleLogLevel - the level for the PowerScale metrics
	PowerscaleLogLevel string = "<POWERSCALE_LOG_LEVEL>"

	// PowerscaleLogFormat - log format
	PowerscaleLogFormat string = "<POWERSCALE_LOG_FORMAT>"

	// PowerflexSdcMetricsEnabled - enable/disable collection of sdc metrics
	PowerflexSdcMetricsEnabled string = "<POWERFLEX_SDC_METRICS_ENABLED>"

	// PowerflexVolumeMetricsEnabled - enable/disable collection of volume metrics
	PowerflexVolumeMetricsEnabled string = "<POWERFLEX_VOLUME_METRICS_ENABLED>"

	// PowerflexStoragePoolMetricsEnabled - enable/disable collection of storage pool metrics
	PowerflexStoragePoolMetricsEnabled string = "<POWERFLEX_STORAGE_POOL_METRICS_ENABLED>"

	// PowerflexSdcIoPollFrequency - polling frequency to get sdc data
	PowerflexSdcIoPollFrequency string = "<POWERFLEX_SDC_IO_POLL_FREQUENCY>"

	// PowerflexVolumeIoPollFrequency - polling frequency to get volume data
	PowerflexVolumeIoPollFrequency string = "<POWERFLEX_VOLUME_IO_POLL_FREQUENCY>"

	// PowerflexStoragePoolPollFrequency - polling frequency to get storage pool data
	PowerflexStoragePoolPollFrequency string = "<POWERFLEX_STORAGE_POOL_POLL_FREQUENCY>"

	// PowerflexMaxConcurrentQueries - max concurrent queries
	PowerflexMaxConcurrentQueries string = "<POWERFLEX_MAX_CONCURRENT_QUERIES>"

	// PowerflexTopologyMetricsEnabled - enable/disable collection of topology metrics
	PowerflexTopologyMetricsEnabled string = "<POWERFLEX_TOPOLOGY_METRICS_ENABLED>"

	// PowerflexTopologyMetricsPollFrequency - polling frequency to get topology metrics data
	PowerflexTopologyMetricsPollFrequency string = "<POWERFLEX_TOPOLOGY_METRICS_POLL_FREQUENCY>"

	// PowerflexLogLevel - the level for the PowerFlex metrics
	PowerflexLogLevel string = "<POWERFLEX_LOG_LEVEL>"

	// PowerflexLogFormat - log format
	PowerflexLogFormat string = "<POWERFLEX_LOG_FORMAT>"

	// NginxProxyImage - Nginx proxy image name
	NginxProxyImage string = "<NGINX_PROXY_IMAGE>"

	// OtelCollectorImage - Otel collector image name
	OtelCollectorImage string = "<OTEL_COLLECTOR_IMAGE>"

	// PscaleObsYamlFile - PowerScale Observability yaml file
	PscaleObsYamlFile string = "karavi-metrics-powerscale.yaml"

	// OtelCollectorYamlFile - Otel Collector yaml file
	OtelCollectorYamlFile string = "karavi-otel-collector.yaml"

	// DriverDefaultReleaseName constant
	DriverDefaultReleaseName string = "<DriverDefaultReleaseName>"

	// PflexObsYamlFile - powerflex metrics yaml file
	PflexObsYamlFile string = "karavi-metrics-powerflex.yaml"

	// PmaxCapacityMetricsEnabled - enable/disable capacity metrics
	PmaxCapacityMetricsEnabled string = "<POWERMAX_CAPACITY_METRICS_ENABLED>"

	// PmaxCapacityPollFreq - polling frequency to get capacity metrics
	PmaxCapacityPollFreq string = "<POWERMAX_CAPACITY_POLL_FREQUENCY>"

	// PmaxPerformanceMetricsEnabled - enable/disable performance metrics
	PmaxPerformanceMetricsEnabled string = "<POWERMAX_PERFORMANCE_METRICS_ENABLED>"

	// PmaxPerformancePollFreq - polling frequency to get capacity metrics
	PmaxPerformancePollFreq string = "<POWERMAX_PERFORMANCE_POLL_FREQUENCY>"

	// PmaxConcurrentQueries - number of concurrent queries
	PmaxConcurrentQueries string = "<POWERMAX_MAX_CONCURRENT_QUERIES>"

	// PmaxTopologyMetricsEnabled - enable/disable collection of topology metrics
	PmaxTopologyMetricsEnabled string = "<POWERMAX_TOPOLOGY_METRICS_ENABLED>"

	// PmaxTopologyMetricsPollFrequency - polling frequency to get topology metrics data
	PmaxTopologyMetricsPollFrequency string = "<POWERMAX_TOPOLOGY_METRICS_POLL_FREQUENCY>"

	// PmaxLogLevel - the level for the Powermax metrics
	PmaxLogLevel string = "<POWERMAX_LOG_LEVEL>"

	// PmaxLogFormat - log format for Powermax metrics
	PmaxLogFormat string = "<POWERMAX_LOG_FORMAT>"

	// PMaxObsYamlFile - powermax metrics yaml file
	PMaxObsYamlFile string = "karavi-metrics-powermax.yaml"

	// PstoreObsYamlFile - powerstore metrics yaml file
	PstoreObsYamlFile string = "karavi-metrics-powerstore.yaml"

	// PstoreMaxConcurrentQueries - number of concurrent queries
	PstoreMaxConcurrentQueries string = "<POWERSTORE_MAX_CONCURRENT_QUERIES>"

	// PstoreVolumeEnabled - enable/disable volume metrics
	PstoreVolumeEnabled string = "<POWERSTORE_VOLUME_METRICS_ENABLED>"

	// PstoreVolumeIoPollFrequency - polling frequency to get volume IO metrics
	PstoreVolumeIoPollFrequency string = "<POWERSTORE_VOLUME_IO_POLL_FREQUENCY>"

	// PstoreSpacePollFrequency - polling frequency to get cluster capacity metrics data
	PstoreSpacePollFrequency string = "<POWERSTORE_SPACE_POLL_FREQUENCY>"

	// PstoreArrayPollFrequency - polling frequency to get array capacity metrics data
	PstoreArrayPollFrequency string = "<POWERSTORE_ARRAY_POLL_FREQUENCY>"

	// PstoreFileSystemPollFrequency - polling frequency to get file system capacity metrics data
	PstoreFileSystemPollFrequency string = "<POWERSTORE_FILE_SYSTEM_POLL_FREQUENCY>"

	// PstoreTopologyEnabled - enable/disable topology metrics
	PstoreTopologyEnabled string = "<POWERSTORE_TOPOLOGY_METRICS_ENABLED>"

	// PstoreTopologyPollFrequency - polling frequency to get topology capacity metrics data
	PstoreTopologyPollFrequency string = "<POWERSTORE_TOPOLOGY_POLL_FREQUENCY>"

	// PstoreLogLevel - the log level for the Powerstore metrics
	PstoreLogLevel string = "<POWERSTORE_LOG_LEVEL>"

	// PstoreLogFormat - the log format for the Powerstore metrics
	PstoreLogFormat string = "<POWERSTORE_LOG_FORMAT>"

	// PstoreApiCallTimeout - the array API call timeout
	PstoreAPITimeout string = "<X_CSI_POWERSTORE_API_TIMEOUT>"

	// ZipkinURI - Zipkin URI for Powerstore metrics
	ZipkinURI string = "<ZIPKIN_URI>"

	// ZipkinServiceName - Zipkin service name for Powerstore metrics
	ZipkinServiceName string = "<ZIPKIN_SERVICE_NAME>"

	// ZipkinProbability - Zipkin probability for Powerstore metrics
	ZipkinProbability string = "<ZIPKIN_PROBABILITY>"

	// SelfSignedCert - self-signed certificate file
	SelfSignedCert string = "selfsigned-cert.yaml"

	// CustomCert - custom certificate file
	CustomCert string = "custom-cert.yaml"

	// ObservabilityCertificate -- certificate for either topology or otel-collector in base64
	ObservabilityCertificate string = "<BASE64_CERTIFICATE>"

	// ObservabilityPrivateKey -- private key for either topology or otel-collector in base64
	ObservabilityPrivateKey string = "<BASE64_PRIVATE_KEY>"

	// ObservabilitySecretPrefix --  placeholder for either karavi-topology or otel-collector
	ObservabilitySecretPrefix string = "<OBSERVABILITY_SECRET_PREFIX>" // #nosec G101 -- false positive

	// CSMNameSpace - namespace CSM is found in. Needed for cases where pod namespace is not namespace of CSM
	CSMNameSpace string = "<CSM_NAMESPACE>"

	DefaultPowerScaleObsMetricsPort            int32  = 8443
	DefaultPowerScaleObsServiceMonitorInterval string = "30s"

	DefaultPowerStoreObsMetricsPort            int32  = 8443
	DefaultPowerStoreObsServiceMonitorInterval string = "30s"

	DefaultPowerMaxObsMetricsPort            int32  = 8443
	DefaultPowerMaxObsServiceMonitorInterval string = "30s"

	DefaultPowerFlexObsMetricsPort            int32  = 8443
	DefaultPowerFlexObsServiceMonitorInterval string = "30s"
)

type obsMetricsConfig struct {
	enabled                          bool
	port                             int32
	tlsCertSecret                    string
	serviceMonitorEnabled            bool
	serviceMonitorInterval           string
	serviceMonitorScrapeTimeout      string
	serviceMonitorInsecureSkipVerify bool
}

// getObsMetricsConfig builds an obsMetricsConfig from the module spec, using the
// provided defaults for port and service-monitor interval.
func getObsMetricsConfig(module csmv1.Module, defaultPort int32, defaultInterval string) obsMetricsConfig {
	config := obsMetricsConfig{
		enabled:                     false,
		port:                        defaultPort,
		serviceMonitorEnabled:       false,
		serviceMonitorInterval:      defaultInterval,
		serviceMonitorScrapeTimeout: "",
	}

	if module.Metrics != nil {
		config.enabled = module.Metrics.Enabled
		if module.Metrics.Port > 0 {
			config.port = module.Metrics.Port
		}
		config.tlsCertSecret = module.Metrics.TLSCertSecret
		if module.Metrics.ServiceMonitor != nil {
			config.serviceMonitorEnabled = module.Metrics.ServiceMonitor.Enabled
			if module.Metrics.ServiceMonitor.Interval != "" {
				config.serviceMonitorInterval = module.Metrics.ServiceMonitor.Interval
			}
			config.serviceMonitorScrapeTimeout = module.Metrics.ServiceMonitor.ScrapeTimeout
			config.serviceMonitorInsecureSkipVerify = module.Metrics.ServiceMonitor.InsecureSkipVerify
		}
		return config
	}

	return config
}

func updateObsService(service *corev1.Service, metricsConfig obsMetricsConfig) {
	filteredPorts := make([]corev1.ServicePort, 0, len(service.Spec.Ports))
	for _, port := range service.Spec.Ports {
		if port.Name != "obs-metrics" {
			filteredPorts = append(filteredPorts, port)
		}
	}
	if metricsConfig.enabled {
		filteredPorts = append(filteredPorts, corev1.ServicePort{
			Name:       "obs-metrics",
			Port:       metricsConfig.port,
			TargetPort: intstr.FromInt32(metricsConfig.port),
		})
	}
	service.Spec.Ports = filteredPorts
}

// updateObsDeployment applies metrics configuration to the named container within a Deployment.
func updateObsDeployment(deployment *appsv1.Deployment, metricsConfig obsMetricsConfig, containerName string) {
	metricsTLSVolName := "metrics-tls"
	metricsTLSMountPath := "/etc/metrics-tls"
	certEnvName := "X_CSI_METRICS_TLS_CERT_FILE"
	certFile := "/etc/metrics-tls/tls.crt"
	keyEnvName := "X_CSI_METRICS_TLS_KEY_FILE"
	keyFile := "/etc/metrics-tls/tls.key"

	filteredVolumes := make([]corev1.Volume, 0, len(deployment.Spec.Template.Spec.Volumes))
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.Name != metricsTLSVolName {
			filteredVolumes = append(filteredVolumes, volume)
		}
	}
	if metricsConfig.enabled && metricsConfig.tlsCertSecret != "" {
		filteredVolumes = append(filteredVolumes, corev1.Volume{
			Name: metricsTLSVolName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: metricsConfig.tlsCertSecret},
			},
		})
	}
	deployment.Spec.Template.Spec.Volumes = filteredVolumes

	for containerIndex := range deployment.Spec.Template.Spec.Containers {
		container := &deployment.Spec.Template.Spec.Containers[containerIndex]
		if container.Name != containerName {
			continue
		}

		filteredPorts := make([]corev1.ContainerPort, 0, len(container.Ports))
		for _, port := range container.Ports {
			if port.Name != "obs-metrics" {
				filteredPorts = append(filteredPorts, port)
			}
		}
		if metricsConfig.enabled {
			filteredPorts = append(filteredPorts, corev1.ContainerPort{
				Name:          "obs-metrics",
				ContainerPort: metricsConfig.port,
				Protocol:      corev1.ProtocolTCP,
			})
		}
		container.Ports = filteredPorts

		filteredMounts := make([]corev1.VolumeMount, 0, len(container.VolumeMounts))
		for _, mount := range container.VolumeMounts {
			if mount.Name != metricsTLSVolName {
				filteredMounts = append(filteredMounts, mount)
			}
		}
		const (
			metricsEnabledEnvName = "X_CSI_METRICS_ENABLED"
			metricsPortEnvName    = "X_CSI_METRICS_PORT"
		)
		filteredEnv := make([]corev1.EnvVar, 0, len(container.Env))
		for _, env := range container.Env {
			if env.Name != certEnvName && env.Name != keyEnvName && env.Name != metricsEnabledEnvName && env.Name != metricsPortEnvName {
				filteredEnv = append(filteredEnv, env)
			}
		}
		if metricsConfig.enabled {
			filteredEnv = append(filteredEnv,
				corev1.EnvVar{Name: metricsEnabledEnvName, Value: "true"},
				corev1.EnvVar{Name: metricsPortEnvName, Value: fmt.Sprintf("%d", metricsConfig.port)},
			)
		}
		if metricsConfig.enabled && metricsConfig.tlsCertSecret != "" {
			filteredMounts = append(filteredMounts, corev1.VolumeMount{
				Name:      metricsTLSVolName,
				MountPath: metricsTLSMountPath,
				ReadOnly:  true,
			})
			filteredEnv = append(
				filteredEnv,
				corev1.EnvVar{Name: certEnvName, Value: certFile},
				corev1.EnvVar{Name: keyEnvName, Value: keyFile},
			)
		}
		container.VolumeMounts = filteredMounts
		container.Env = filteredEnv
		return
	}
}

func updateObsServiceMonitor(obj crclient.Object, metricsConfig obsMetricsConfig) error {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}

	endpoints, found, err := unstructured.NestedSlice(u.Object, "spec", "endpoints")
	if err != nil {
		return err
	}
	if !found || len(endpoints) == 0 {
		endpoints = []interface{}{map[string]interface{}{}}
	}

	endpoint, ok := endpoints[0].(map[string]interface{})
	if !ok {
		endpoint = map[string]interface{}{}
	}
	endpoint["port"] = "obs-metrics"
	endpoint["interval"] = metricsConfig.serviceMonitorInterval
	if metricsConfig.serviceMonitorScrapeTimeout == "" {
		delete(endpoint, "scrapeTimeout")
	} else {
		endpoint["scrapeTimeout"] = metricsConfig.serviceMonitorScrapeTimeout
	}
	if !metricsConfig.enabled || metricsConfig.tlsCertSecret == "" {
		delete(endpoint, "scheme")
		delete(endpoint, "tlsConfig")
	} else {
		endpoint["scheme"] = "https"
		endpoint["tlsConfig"] = map[string]interface{}{
			"insecureSkipVerify": metricsConfig.serviceMonitorInsecureSkipVerify,
		}
	}
	endpoints[0] = endpoint

	return unstructured.SetNestedSlice(u.Object, endpoints, "spec", "endpoints")
}

// applyObsMetricsConfig applies the metrics configuration to the named Service and Deployment
// resource within the objects slice, and filters ServiceMonitor objects when metrics are disabled.
func applyObsMetricsConfig(objects []crclient.Object, metricsConfig obsMetricsConfig, resourceName string) ([]crclient.Object, error) {
	filteredObjects := make([]crclient.Object, 0, len(objects))
	for _, obj := range objects {
		switch typed := obj.(type) {
		case *corev1.Service:
			if typed.Name == resourceName {
				updateObsService(typed, metricsConfig)
			}
			filteredObjects = append(filteredObjects, obj)
		case *appsv1.Deployment:
			if typed.Name == resourceName {
				updateObsDeployment(typed, metricsConfig, resourceName)
			}
			filteredObjects = append(filteredObjects, obj)
		default:
			if obj.GetObjectKind().GroupVersionKind().Kind == "ServiceMonitor" {
				if !metricsConfig.enabled || !metricsConfig.serviceMonitorEnabled {
					continue
				}
				if err := updateObsServiceMonitor(obj, metricsConfig); err != nil {
					return nil, err
				}
			}
			filteredObjects = append(filteredObjects, obj)
		}
	}

	return filteredObjects, nil
}

// ComponentNameToSecretPrefix - map from component name to secret prefix
var ComponentNameToSecretPrefix = map[string]string{ObservabilityOtelCollectorName: "otel-collector", ObservabilityTopologyName: "karavi-topology", ObservabilityMetricsPowerStoreName: "karavi-metrics-powerstore"}

// ObservabilitySupportedDrivers is a map containing the CSI Drivers supported by CSM Replication. The key is driver name and the value is the driver plugin identifier
var ObservabilitySupportedDrivers = map[string]SupportedDriverParam{
	"powerscale": {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
	},
	"isilon": {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
	},
	"powerflex": {
		PluginIdentifier:              drivers.PowerFlexPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerFlexConfigParamsVolumeMount,
	},
	"vxflexos": {
		PluginIdentifier:              drivers.PowerFlexPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerFlexConfigParamsVolumeMount,
	},
	"powerstore": {
		PluginIdentifier:              drivers.PowerStorePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerStoreConfigParamsVolumeMount,
	},
	string(csmv1.PowerMax): {
		PluginIdentifier:              drivers.PowerMaxPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerMaxConfigParamsVolumeMount,
	},
}

var defaultVolumeConfigName = map[csmv1.DriverType]string{
	csmv1.PowerScaleName: "isilon-creds",
	csmv1.PowerScale:     "isilon-creds",
	csmv1.PowerFlexName:  "vxflexos-config",
	csmv1.PowerFlex:      "vxflexos-config",
	csmv1.PowerStore:     "powerstore-config",
}

// ObservabilityPrecheck  - runs precheck for CSM Otoolsabilitytools
func ObservabilityPrecheck(ctx context.Context, op operatorutils.OperatorConfig, obs csmv1.Module, cr csmv1.ContainerStorageModule, r operatorutils.ReconcileCSM) error {
	log := logger.GetLogger(ctx)

	if _, ok := ObservabilitySupportedDrivers[string(cr.Spec.Driver.CSIDriverType)]; !ok {
		return fmt.Errorf("CSM Operator does not suport Observability deployment for %s driver", cr.Spec.Driver.CSIDriverType)
	}

	// check if provided version is supported
	if obs.ConfigVersion != "" {
		err := checkVersion(string(csmv1.Observability), obs.ConfigVersion, op.ConfigDirectory)
		if err != nil {
			return err
		}
	}

	clusterClient := operatorutils.GetCluster(ctx, r)
	if obs.Metrics != nil && obs.Metrics.Enabled && obs.Metrics.TLSCertSecret != "" {
		err := clusterClient.ClusterCTRLClient.Get(
			ctx,
			types.NamespacedName{Name: obs.Metrics.TLSCertSecret, Namespace: cr.GetNamespace()},
			&corev1.Secret{},
		)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return fmt.Errorf("failed to find secret %s", obs.Metrics.TLSCertSecret)
			}
			return err
		}
	}

	log.Infof("\nperformed pre checks for: %s", obs.Name)
	return nil
}

// ObservabilityTopology - delete or update topology objectstools
func ObservabilityTopology(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client, matched operatorutils.VersionSpec) error {
	log := logger.GetLogger(ctx)

	configVersion, err := getVersionFn(ctx, &cr, op)
	if err != nil {
		return err
	}
	if strings.Contains(configVersion, "v2.13") || strings.Contains(configVersion, "v2.14") {
		topoObjects, err := getTopology(ctx, op, cr, matched)
		if err != nil {
			return err
		}

		for _, ctrlObj := range topoObjects {
			log.Infow("current topoObject is ", "ctrlObj", ctrlObj)
			if isDeleting {
				if err := operatorutils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
					return err
				}
			} else {
				if err := operatorutils.ApplyCTRLObject(ctx, ctrlObj, ctrlClient); err != nil {
					return err
				}
			}
		}
	} else {
		return fmt.Errorf("CSM Operator does not suport topology deployment from CSM 1.15 onwards")
	}

	return nil
}

func getTopology(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, matched operatorutils.VersionSpec) ([]crclient.Object, error) {
	obs, err := getObservabilityModuleFn(cr)
	if err != nil {
		return nil, err
	}

	buf, err := readConfigFileFn(ctx, obs, cr, op, TopologyYamlFile)
	if err != nil {
		return nil, err
	}
	YamlString := string(buf)

	logLevel := "info"
	topologyImage := ""

	for _, component := range obs.Components {
		if component.Name == ObservabilityTopologyName {
			topologyImage = operatorutils.GetFinalImage(ctx, cr, matched, component, YamlString)
			for _, env := range component.Envs {
				if strings.Contains(TopologyLogLevel, env.Name) {
					logLevel = strings.ToLower(strings.TrimSpace(env.Value))
				}
			}
		}
	}

	// Validate CR fields before YAML substitution to prevent injection attacks
	if err := operatorutils.ValidateKubernetesName(cr.Name); err != nil {
		return nil, fmt.Errorf("invalid CR name for YAML substitution: %w", err)
	}
	if err := operatorutils.ValidateKubernetesNamespace(cr.Namespace); err != nil {
		return nil, fmt.Errorf("invalid CR namespace for YAML substitution: %w", err)
	}
	if err := operatorutils.ValidateYAMLSubstitutionValue(logLevel, "TopologyLogLevel"); err != nil {
		return nil, fmt.Errorf("invalid log level for YAML substitution: %w", err)
	}

	YamlString = strings.ReplaceAll(YamlString, CSMName, cr.Name)
	YamlString = strings.ReplaceAll(YamlString, CSMNameSpace, cr.Namespace)
	YamlString = strings.ReplaceAll(YamlString, TopologyLogLevel, logLevel)

	topoObjects, err := operatorutils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return nil, err
	}
	operatorutils.SetContainerImage(topoObjects, "karavi-topology", "karavi-topology", topologyImage)

	return topoObjects, nil
}

// OtelCollector - delete or update otel collector objects
func OtelCollector(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client, matched operatorutils.VersionSpec) error {
	YamlString, err := getOtelCollector(ctx, op, cr, matched)
	if err != nil {
		return err
	}

	otelObjects, err := operatorutils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range otelObjects {
		if isDeleting {
			if err := operatorutils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := operatorutils.ApplyCTRLObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	return nil
}

// getOtelCollector - get otel collector yaml string
func getOtelCollector(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, matched operatorutils.VersionSpec) (string, error) {
	YamlString := ""

	obs, err := getObservabilityModule(cr)
	if err != nil {
		return YamlString, err
	}

	buf, err := readConfigFile(ctx, obs, cr, op, OtelCollectorYamlFile)
	if err != nil {
		return YamlString, err
	}
	YamlString = string(buf)

	nginxProxyImage := "quay.io/nginx/nginx-unprivileged:1.27"
	nginxProxyImageFromConfigMap := false
	if matched.Version != "" {
		if img := matched.Images["nginx-proxy"]; img != "" {
			nginxProxyImage = img
			nginxProxyImageFromConfigMap = true
		}
	}
	if !nginxProxyImageFromConfigMap && cr.Spec.CustomRegistry != "" {
		nginxProxyImage = operatorutils.ResolveImage(ctx, nginxProxyImage, cr)
	}
	otelCollectorImage := "ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector:0.160.0"
	otelCollectorImageFromConfigMap := false
	configVersion, err := operatorutils.GetVersion(ctx, &cr, op)
	if err != nil {
		return "", err
	}
	// Currently supported config versions by this operator(release candidate for CSM v2.14.0) are v2.11.0, v2.12.0, v2.13.0.
	// These config versions were already supported by the released operators. So use the same otel image for them.
	if configVersion == "v2.11.0" || configVersion == "v2.12.0" || configVersion == "v2.13.0" {
		otelCollectorImage = "otel/opentelemetry-collector:0.42.0"
	}

	for _, component := range obs.Components {
		if component.Name == ObservabilityOtelCollectorName {
			if matched.Version != "" {
				if img := matched.Images[component.Name]; img != "" {
					otelCollectorImage = img
					otelCollectorImageFromConfigMap = true
				}
			}
			if !otelCollectorImageFromConfigMap && cr.Spec.CustomRegistry != "" {
				otelCollectorImage = operatorutils.ResolveImage(ctx, otelCollectorImage, cr)
			} else if !otelCollectorImageFromConfigMap && component.Image != "" {
				otelCollectorImage = string(component.Image)
			}

			for _, env := range component.Envs {
				if strings.Contains(NginxProxyImage, env.Name) {
					nginxProxyImage = env.Value
					nginxProxyImageFromConfigMap = false
					if matched.Version != "" {
						if img := matched.Images["nginx-proxy"]; img != "" {
							nginxProxyImage = img
							nginxProxyImageFromConfigMap = true
						}
					}
					if !nginxProxyImageFromConfigMap && cr.Spec.CustomRegistry != "" {
						nginxProxyImage = operatorutils.ResolveImage(ctx, nginxProxyImage, cr)
					}
				}
			}
		}
	}

	// Validate CR fields before YAML substitution to prevent injection attacks
	if err := operatorutils.ValidateKubernetesName(cr.Name); err != nil {
		return "", fmt.Errorf("invalid CR name for YAML substitution: %w", err)
	}
	if err := operatorutils.ValidateKubernetesNamespace(cr.Namespace); err != nil {
		return "", fmt.Errorf("invalid CR namespace for YAML substitution: %w", err)
	}
	if err := operatorutils.ValidateImageString(otelCollectorImage, "OtelCollectorImage"); err != nil {
		return "", fmt.Errorf("invalid image for YAML substitution: %w", err)
	}
	if err := operatorutils.ValidateImageString(nginxProxyImage, "NginxProxyImage"); err != nil {
		return "", fmt.Errorf("invalid image for YAML substitution: %w", err)
	}

	YamlString = strings.ReplaceAll(YamlString, CSMName, cr.Name)
	YamlString = strings.ReplaceAll(YamlString, CSMNameSpace, cr.Namespace)
	YamlString = strings.ReplaceAll(YamlString, OtelCollectorImage, otelCollectorImage)
	YamlString = strings.ReplaceAll(YamlString, NginxProxyImage, nginxProxyImage)

	return YamlString, nil
}

// PowerScaleMetrics - delete or update powerscale metrics objects
func PowerScaleMetrics(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client, k8sClient kubernetes.Interface) error {
	log := logger.GetLogger(ctx)

	var matched operatorutils.VersionSpec
	if cr.Spec.Version != "" {
		var err error
		matched, err = operatorutils.ResolveVersionFromConfigMap(ctx, ctrlClient, &cr)
		if err != nil {
			log.Error(err, "Failed to get version from configmap")
			return err
		}
	}

	powerscaleMetricsObjects, err := getPowerScaleMetricsObjects(ctx, op, cr, matched)
	if err != nil {
		return err
	}

	// update secret volume and inject authorization to deployment
	var dpApply *confv1.DeploymentApplyConfiguration
	foundDp := false
	for i, obj := range powerscaleMetricsObjects {
		if deployment, ok := obj.(*appsv1.Deployment); ok {
			dpApply, err = parseObservabilityMetricsDeployment(ctx, deployment, op, cr, ctrlClient)
			if err != nil {
				return err
			}
			foundDp = true
			powerscaleMetricsObjects[i] = powerscaleMetricsObjects[len(powerscaleMetricsObjects)-1]
			powerscaleMetricsObjects = powerscaleMetricsObjects[:len(powerscaleMetricsObjects)-1]
			break
		}
	}
	if !foundDp {
		return fmt.Errorf("could not find deployment obj")
	}

	obs, err := getObservabilityModule(cr)
	if err != nil {
		return err
	}
	obsMetricsConfig := getObsMetricsConfig(obs, DefaultPowerScaleObsMetricsPort, DefaultPowerScaleObsServiceMonitorInterval)

	if !obsMetricsConfig.enabled || !obsMetricsConfig.serviceMonitorEnabled {
		serviceMonitor := &unstructured.Unstructured{}
		serviceMonitor.SetAPIVersion("monitoring.coreos.com/v1")
		serviceMonitor.SetKind("ServiceMonitor")
		serviceMonitor.SetName("karavi-metrics-powerscale-obs-monitor")
		serviceMonitor.SetNamespace(cr.Namespace)
		if err := operatorutils.DeleteObject(ctx, serviceMonitor, ctrlClient); err != nil {
			return err
		}
	}

	for _, ctrlObj := range powerscaleMetricsObjects {
		if isDeleting {
			if err := operatorutils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			// ServiceMonitor requires special handling to fetch resourceVersion for updates
			if ctrlObj.GetObjectKind().GroupVersionKind().Kind == "ServiceMonitor" {
				found := &unstructured.Unstructured{}
				found.SetGroupVersionKind(ctrlObj.GetObjectKind().GroupVersionKind())
				err = ctrlClient.Get(ctx, client.ObjectKey{Name: ctrlObj.GetName(), Namespace: ctrlObj.GetNamespace()}, found)
				if err == nil {
					// Copy resourceVersion from existing object for optimistic concurrency
					ctrlObj.SetResourceVersion(found.GetResourceVersion())
				}
			}
			if err := operatorutils.ApplyCTRLObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	// update Deployment
	if isDeleting {
		// Delete Deployment
		deploymentKey := client.ObjectKey{
			Namespace: *dpApply.Namespace,
			Name:      *dpApply.Name,
		}
		deploymentObj := &appsv1.Deployment{}
		if err = ctrlClient.Get(ctx, deploymentKey, deploymentObj); err == nil {
			if err = ctrlClient.Delete(ctx, deploymentObj); err != nil && !k8serrors.IsNotFound(err) {
				return fmt.Errorf("error deleting deployment: %v", err)
			}
		} else {
			log.Infow("error getting deployment", "deploymentKey", deploymentKey)
		}
	} else {
		// Create/Update Deployment
		if err = deployment.SyncDeployment(ctx, *dpApply, k8sClient, cr.Name); err != nil {
			return err
		}
	}

	return nil
}

// PowerStoreMetrics - delete or update powerstore metrics objects
func PowerStoreMetrics(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client, k8sClient kubernetes.Interface) error {
	log := logger.GetLogger(ctx)

	var matched operatorutils.VersionSpec
	if cr.Spec.Version != "" {
		var err error
		matched, err = operatorutils.ResolveVersionFromConfigMap(ctx, ctrlClient, &cr)
		if err != nil {
			log.Error(err, "Failed to get version from configmap")
			return err
		}
	}

	powerstoreMetricsObjects, err := getPowerStoreMetricsObjects(ctx, op, cr, matched)
	if err != nil {
		return err
	}

	// update deployment for powerstore metrics
	var dpApply *confv1.DeploymentApplyConfiguration
	foundDp := false
	for i, obj := range powerstoreMetricsObjects {
		if deployment, ok := obj.(*appsv1.Deployment); ok {
			dpApply, err = parseObservabilityMetricsDeployment(ctx, deployment, op, cr, ctrlClient)
			if err != nil {
				return err
			}
			foundDp = true
			powerstoreMetricsObjects[i] = powerstoreMetricsObjects[len(powerstoreMetricsObjects)-1]
			powerstoreMetricsObjects = powerstoreMetricsObjects[:len(powerstoreMetricsObjects)-1]
			break
		}
	}
	if !foundDp {
		return fmt.Errorf("could not find deployment obj")
	}

	obs, err := getObservabilityModule(cr)
	if err != nil {
		return err
	}
	obsMetricsConfig := getObsMetricsConfig(obs, DefaultPowerStoreObsMetricsPort, DefaultPowerStoreObsServiceMonitorInterval)

	if !obsMetricsConfig.enabled || !obsMetricsConfig.serviceMonitorEnabled {
		serviceMonitor := &unstructured.Unstructured{}
		serviceMonitor.SetAPIVersion("monitoring.coreos.com/v1")
		serviceMonitor.SetKind("ServiceMonitor")
		serviceMonitor.SetName("karavi-metrics-powerstore-obs-monitor")
		serviceMonitor.SetNamespace(cr.Namespace)
		if err := operatorutils.DeleteObject(ctx, serviceMonitor, ctrlClient); err != nil {
			return err
		}
	}

	for _, ctrlObj := range powerstoreMetricsObjects {
		if isDeleting {
			if err := operatorutils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			// ServiceMonitor requires special handling to fetch resourceVersion for updates
			if ctrlObj.GetObjectKind().GroupVersionKind().Kind == "ServiceMonitor" {
				found := &unstructured.Unstructured{}
				found.SetGroupVersionKind(ctrlObj.GetObjectKind().GroupVersionKind())
				err = ctrlClient.Get(ctx, client.ObjectKey{Name: ctrlObj.GetName(), Namespace: ctrlObj.GetNamespace()}, found)
				if err == nil {
					// Copy resourceVersion from existing object for optimistic concurrency
					ctrlObj.SetResourceVersion(found.GetResourceVersion())
				}
			}
			if err := operatorutils.ApplyCTRLObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	// update Deployment
	if isDeleting {
		// Delete Deployment
		deploymentKey := client.ObjectKey{
			Namespace: *dpApply.Namespace,
			Name:      *dpApply.Name,
		}
		deploymentObj := &appsv1.Deployment{}
		if err = ctrlClient.Get(ctx, deploymentKey, deploymentObj); err == nil {
			if err = ctrlClient.Delete(ctx, deploymentObj); err != nil && !k8serrors.IsNotFound(err) {
				return fmt.Errorf("error deleting deployment: %v", err)
			}
		} else {
			log.Infow("error getting deployment", "deploymentKey", deploymentKey)
		}
	} else {
		// Create/Update Deployment
		if err = deployment.SyncDeployment(ctx, *dpApply, k8sClient, cr.Name); err != nil {
			return err
		}
	}

	return nil
}

// getPowerStoreMetricsObjects - get powerstore metrics yaml string
func getPowerStoreMetricsObjects(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, matched operatorutils.VersionSpec) ([]crclient.Object, error) {
	obs, err := getObservabilityModule(cr)
	if err != nil {
		return nil, err
	}

	buf, err := readConfigFile(ctx, obs, cr, op, PstoreObsYamlFile)
	if err != nil {
		return nil, err
	}
	YamlString := string(buf)

	obsPstoreImage := ""
	maxConcurrentQueries := "10"
	volumeEnabled := "true"
	volumePollFrequency := "10"
	spacePollFrequency := "300"
	arrayPollFrequency := "300"
	fsPollFrequency := "20"
	topologyEnabled := "true"
	topologyPollFrequency := "30"
	apiTimeout := "120s"
	zipkinURI := ""
	zipkinServiceName := "metrics-powerstore"
	zipkinProbability := "0.0"
	logLevel := "info"
	logFormat := "json"
	otelCollectorAddress := "otel-collector:55680"

	for _, component := range obs.Components {
		if component.Name == ObservabilityMetricsPowerStoreName {
			obsPstoreImage = operatorutils.GetFinalImage(ctx, cr, matched, component, YamlString)
			for _, env := range component.Envs {
				if strings.Contains(PstoreMaxConcurrentQueries, env.Name) {
					maxConcurrentQueries = env.Value
				} else if strings.Contains(PstoreVolumeEnabled, env.Name) {
					volumeEnabled = env.Value
				} else if strings.Contains(PstoreVolumeIoPollFrequency, env.Name) {
					volumePollFrequency = env.Value
				} else if strings.Contains(PstoreSpacePollFrequency, env.Name) {
					spacePollFrequency = env.Value
				} else if strings.Contains(PstoreArrayPollFrequency, env.Name) {
					arrayPollFrequency = env.Value
				} else if strings.Contains(PstoreFileSystemPollFrequency, env.Name) {
					fsPollFrequency = env.Value
				} else if strings.Contains(PstoreTopologyEnabled, env.Name) {
					topologyEnabled = env.Value
				} else if strings.Contains(PstoreTopologyPollFrequency, env.Name) {
					topologyPollFrequency = env.Value
				} else if strings.Contains(PstoreAPITimeout, env.Name) {
					apiTimeout = env.Value
				} else if strings.Contains(ZipkinURI, env.Name) {
					zipkinURI = env.Value
				} else if strings.Contains(ZipkinServiceName, env.Name) {
					zipkinServiceName = env.Value
				} else if strings.Contains(ZipkinProbability, env.Name) {
					zipkinProbability = env.Value
				} else if strings.Contains(PstoreLogLevel, env.Name) {
					logLevel = strings.ToLower(strings.TrimSpace(env.Value))
				} else if strings.Contains(PstoreLogFormat, env.Name) {
					logFormat = strings.ToLower(strings.TrimSpace(env.Value))
				} else if strings.Contains(OtelCollectorAddress, env.Name) {
					otelCollectorAddress = env.Value
				}
			}
		}
	}

	// Validate CR fields and user-controlled values before YAML substitution
	if err := operatorutils.ValidateKubernetesName(cr.Name); err != nil {
		return nil, fmt.Errorf("invalid CR name for YAML substitution: %w", err)
	}
	if err := operatorutils.ValidateKubernetesNamespace(cr.Namespace); err != nil {
		return nil, fmt.Errorf("invalid CR namespace for YAML substitution: %w", err)
	}
	// Validate all environment-sourced values (including numeric/boolean values to prevent injection)
	userControlledValues := map[string]string{
		"logLevel":              logLevel,
		"logFormat":             logFormat,
		"otelCollectorAddress":  otelCollectorAddress,
		"maxConcurrentQueries":  maxConcurrentQueries,
		"volumeEnabled":         volumeEnabled,
		"volumePollFrequency":   volumePollFrequency,
		"spacePollFrequency":    spacePollFrequency,
		"arrayPollFrequency":    arrayPollFrequency,
		"fsPollFrequency":       fsPollFrequency,
		"topologyEnabled":       topologyEnabled,
		"topologyPollFrequency": topologyPollFrequency,
		"apiTimeout":            apiTimeout,
		"zipkinURI":             zipkinURI,
		"zipkinServiceName":     zipkinServiceName,
		"zipkinProbability":     zipkinProbability,
	}
	for fieldName, value := range userControlledValues {
		// Only validate non-empty values (some fields are optional)
		if value != "" {
			if err := operatorutils.ValidateYAMLSubstitutionValue(value, fieldName); err != nil {
				return nil, fmt.Errorf("invalid value for YAML substitution: %w", err)
			}
		}
	}

	YamlString = strings.ReplaceAll(YamlString, CSMName, cr.Name)
	YamlString = strings.ReplaceAll(YamlString, CSMNameSpace, cr.Namespace)
	YamlString = strings.ReplaceAll(YamlString, PstoreMaxConcurrentQueries, maxConcurrentQueries)
	YamlString = strings.ReplaceAll(YamlString, PstoreVolumeEnabled, volumeEnabled)
	YamlString = strings.ReplaceAll(YamlString, PstoreVolumeIoPollFrequency, volumePollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PstoreSpacePollFrequency, spacePollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PstoreArrayPollFrequency, arrayPollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PstoreFileSystemPollFrequency, fsPollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PstoreTopologyEnabled, topologyEnabled)
	YamlString = strings.ReplaceAll(YamlString, PstoreTopologyPollFrequency, topologyPollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PstoreAPITimeout, apiTimeout)
	YamlString = strings.ReplaceAll(YamlString, ZipkinURI, zipkinURI)
	YamlString = strings.ReplaceAll(YamlString, ZipkinServiceName, zipkinServiceName)
	YamlString = strings.ReplaceAll(YamlString, ZipkinProbability, zipkinProbability)
	YamlString = strings.ReplaceAll(YamlString, PstoreLogLevel, logLevel)
	YamlString = strings.ReplaceAll(YamlString, PstoreLogFormat, logFormat)
	YamlString = strings.ReplaceAll(YamlString, OtelCollectorAddress, otelCollectorAddress)
	YamlString = strings.ReplaceAll(YamlString, DriverDefaultReleaseName, cr.Name)

	obsMetricsConfig := getObsMetricsConfig(obs, DefaultPowerStoreObsMetricsPort, DefaultPowerStoreObsServiceMonitorInterval)
	obsMetricsEnabled := "false"
	if obsMetricsConfig.enabled {
		obsMetricsEnabled = "true"
	}
	obsMetricsPort := fmt.Sprintf("%d", obsMetricsConfig.port)
	YamlString = strings.ReplaceAll(YamlString, constants.CsiMetricsEnabled, obsMetricsEnabled)
	YamlString = strings.ReplaceAll(YamlString, constants.CsiMetricsPort, obsMetricsPort)

	metricsObjects, err := operatorutils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return nil, err
	}

	operatorutils.SetContainerImage(metricsObjects, "karavi-metrics-powerstore", "karavi-metrics-powerstore", obsPstoreImage)

	metricsObjects, err = applyObsMetricsConfig(metricsObjects, obsMetricsConfig, "karavi-metrics-powerstore")
	if err != nil {
		return nil, err
	}

	return metricsObjects, nil
}

// getPowerScaleMetricsObjects - get powerscale metrics yaml string
func getPowerScaleMetricsObjects(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, matched operatorutils.VersionSpec) ([]crclient.Object, error) {
	obs, err := getObservabilityModule(cr)
	if err != nil {
		return nil, err
	}

	buf, err := readConfigFile(ctx, obs, cr, op, PscaleObsYamlFile)
	if err != nil {
		return nil, err
	}
	YamlString := string(buf)

	logLevel := "info"
	otelCollectorAddress := "otel-collector:55680"
	pscaleImage := ""
	maxConcurrentQueries := "10"
	capacityEnabled := "true"
	performanceEnabled := "true"
	topologyEnabled := "true"
	topologyPollFrequency := "30"
	clusterCapacityPollFrequency := "30"
	clusterPerformancePollFrequency := "20"
	quotaCapacityPollFrequency := "30"
	clientInsecure := "true"
	clientAuthType := "1"
	clientVerbose := "0"
	logFormat := "json"
	obsMetricsConfig := getObsMetricsConfig(obs, DefaultPowerScaleObsMetricsPort, DefaultPowerScaleObsServiceMonitorInterval)
	obsMetricsEnabled := fmt.Sprintf("%t", obsMetricsConfig.enabled)
	obsMetricsPort := fmt.Sprintf("%d", obsMetricsConfig.port)

	for _, component := range obs.Components {
		if component.Name == ObservabilityMetricsPowerScaleName {
			pscaleImage = operatorutils.GetFinalImage(ctx, cr, matched, component, YamlString)
			for _, env := range component.Envs {
				if strings.Contains(PowerscaleLogLevel, env.Name) {
					logLevel = strings.ToLower(strings.TrimSpace(env.Value))
				} else if strings.Contains(PowerScaleMaxConcurrentQueries, env.Name) {
					maxConcurrentQueries = env.Value
				} else if strings.Contains(PowerscaleCapacityMetricsEnabled, env.Name) {
					capacityEnabled = env.Value
				} else if strings.Contains(PowerscalePerformanceMetricsEnabled, env.Name) {
					performanceEnabled = env.Value
				} else if strings.Contains(PowerscaleTopologyMetricsEnabled, env.Name) {
					topologyEnabled = env.Value
				} else if strings.Contains(PowerscaleTopologyMetricsPollFrequency, env.Name) {
					topologyPollFrequency = env.Value
				} else if strings.Contains(PowerscaleClusterCapacityPollFrequency, env.Name) {
					clusterCapacityPollFrequency = env.Value
				} else if strings.Contains(PowerscaleClusterPerformancePollFrequency, env.Name) {
					clusterPerformancePollFrequency = env.Value
				} else if strings.Contains(PowerscaleQuotaCapacityPollFrequency, env.Name) {
					quotaCapacityPollFrequency = env.Value
				} else if strings.Contains(IsiclientInsecure, env.Name) {
					clientInsecure = env.Value
				} else if strings.Contains(IsiclientAuthType, env.Name) {
					clientAuthType = env.Value
				} else if strings.Contains(IsiclientVerbose, env.Name) {
					clientVerbose = env.Value
				} else if strings.Contains(PowerscaleLogFormat, env.Name) {
					logFormat = strings.ToLower(strings.TrimSpace(env.Value))
				} else if strings.Contains(OtelCollectorAddress, env.Name) {
					otelCollectorAddress = env.Value
				}
			}
		}
	}

	// Validate CR fields and user-controlled values before YAML substitution
	if err := operatorutils.ValidateKubernetesName(cr.Name); err != nil {
		return nil, fmt.Errorf("invalid CR name for YAML substitution: %w", err)
	}
	if err := operatorutils.ValidateKubernetesNamespace(cr.Namespace); err != nil {
		return nil, fmt.Errorf("invalid CR namespace for YAML substitution: %w", err)
	}
	// Validate all environment-sourced values (including numeric/boolean values to prevent injection)
	userControlledValues := map[string]string{
		"logLevel":                        logLevel,
		"logFormat":                       logFormat,
		"otelCollectorAddress":            otelCollectorAddress,
		"maxConcurrentQueries":            maxConcurrentQueries,
		"capacityEnabled":                 capacityEnabled,
		"performanceEnabled":              performanceEnabled,
		"topologyEnabled":                 topologyEnabled,
		"topologyPollFrequency":           topologyPollFrequency,
		"clusterCapacityPollFrequency":    clusterCapacityPollFrequency,
		"clusterPerformancePollFrequency": clusterPerformancePollFrequency,
		"quotaCapacityPollFrequency":      quotaCapacityPollFrequency,
		"clientInsecure":                  clientInsecure,
		"clientAuthType":                  clientAuthType,
		"clientVerbose":                   clientVerbose,
	}
	for fieldName, value := range userControlledValues {
		// Only validate non-empty values (some fields are optional)
		if value != "" {
			if err := operatorutils.ValidateYAMLSubstitutionValue(value, fieldName); err != nil {
				return nil, fmt.Errorf("invalid value for YAML substitution: %w", err)
			}
		}
	}

	YamlString = strings.ReplaceAll(YamlString, CSMName, cr.Name)
	YamlString = strings.ReplaceAll(YamlString, CSMNameSpace, cr.Namespace)
	YamlString = strings.ReplaceAll(YamlString, PowerscaleLogLevel, logLevel)
	YamlString = strings.ReplaceAll(YamlString, PowerScaleMaxConcurrentQueries, maxConcurrentQueries)
	YamlString = strings.ReplaceAll(YamlString, PowerscaleCapacityMetricsEnabled, capacityEnabled)
	YamlString = strings.ReplaceAll(YamlString, PowerscalePerformanceMetricsEnabled, performanceEnabled)
	YamlString = strings.ReplaceAll(YamlString, PowerscaleTopologyMetricsEnabled, topologyEnabled)
	YamlString = strings.ReplaceAll(YamlString, PowerscaleTopologyMetricsPollFrequency, topologyPollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PowerscaleClusterCapacityPollFrequency, clusterCapacityPollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PowerscaleClusterPerformancePollFrequency, clusterPerformancePollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PowerscaleQuotaCapacityPollFrequency, quotaCapacityPollFrequency)
	YamlString = strings.ReplaceAll(YamlString, IsiclientInsecure, clientInsecure)
	YamlString = strings.ReplaceAll(YamlString, IsiclientAuthType, clientAuthType)
	YamlString = strings.ReplaceAll(YamlString, IsiclientVerbose, clientVerbose)
	YamlString = strings.ReplaceAll(YamlString, PowerscaleLogFormat, logFormat)
	YamlString = strings.ReplaceAll(YamlString, OtelCollectorAddress, otelCollectorAddress)
	YamlString = strings.ReplaceAll(YamlString, constants.CsiMetricsEnabled, obsMetricsEnabled)
	YamlString = strings.ReplaceAll(YamlString, constants.CsiMetricsPort, obsMetricsPort)
	YamlString = strings.ReplaceAll(YamlString, DriverDefaultReleaseName, cr.Name)

	metricsObjects, err := operatorutils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return nil, err
	}
	metricsObjects, err = applyObsMetricsConfig(metricsObjects, obsMetricsConfig, "karavi-metrics-powerscale")
	if err != nil {
		return nil, err
	}

	operatorutils.SetContainerImage(metricsObjects, "karavi-metrics-powerscale", "karavi-metrics-powerscale", pscaleImage)

	return metricsObjects, nil
}

// parseObservabilityMetricsDeployment - update secret volume and inject authorization to deployment
func parseObservabilityMetricsDeployment(ctx context.Context, deployment *appsv1.Deployment, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client) (*confv1.DeploymentApplyConfiguration, error) {
	// parse deployment to DeploymentApplyConfiguration
	dpBuf, err := yaml.Marshal(deployment)
	if err != nil {
		return nil, err
	}
	dpApply := &confv1.DeploymentApplyConfiguration{}
	err = yaml.Unmarshal(dpBuf, dpApply)
	if err != nil {
		return nil, err
	}

	// Update secret volume
	for i, v := range dpApply.Spec.Template.Spec.Volumes {
		if *v.Name == defaultVolumeConfigName[cr.GetDriverType()] && cr.Spec.Driver.AuthSecret != "" {
			dpApply.Spec.Template.Spec.Volumes[i].Secret.SecretName = &cr.Spec.Driver.AuthSecret
		}
	}

	// inject authorization to deployment
	if authorizationEnabled, _ := operatorutils.IsModuleEnabled(ctx, cr, csmv1.Authorization); authorizationEnabled {
		dpApply, err = AuthInjectDeployment(ctx, *dpApply, cr, op, ctrlClient)
		if err != nil {
			return nil, fmt.Errorf("injecting auth into Observability metrics deployment: %v", err)
		}
	}
	return dpApply, nil
}

// PowerFlexMetrics - delete or update powerflex metrics objects
func PowerFlexMetrics(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client, k8sClient kubernetes.Interface) error {
	log := logger.GetLogger(ctx)

	var matched operatorutils.VersionSpec
	if cr.Spec.Version != "" {
		var err error
		matched, err = operatorutils.ResolveVersionFromConfigMap(ctx, ctrlClient, &cr)
		if err != nil {
			log.Error(err, "Failed to get version from configmap")
			return err
		}
	}

	powerflexMetricsObjects, err := getPowerFlexMetricsObject(ctx, op, cr, matched)
	if err != nil {
		return err
	}

	// update secret volume and inject authorization to deployment
	var dpApply *confv1.DeploymentApplyConfiguration
	foundDp := false
	for i, obj := range powerflexMetricsObjects {
		if deployment, ok := obj.(*appsv1.Deployment); ok {
			dpApply, err = parseObservabilityMetricsDeployment(ctx, deployment, op, cr, ctrlClient)
			if err != nil {
				return err
			}
			foundDp = true
			powerflexMetricsObjects[i] = powerflexMetricsObjects[len(powerflexMetricsObjects)-1]
			powerflexMetricsObjects = powerflexMetricsObjects[:len(powerflexMetricsObjects)-1]
			break
		}
	}
	if !foundDp {
		return fmt.Errorf("could not find deployment obj")
	}

	for _, ctrlObj := range powerflexMetricsObjects {
		if isDeleting {
			if err := operatorutils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			// ServiceMonitor requires special handling to fetch resourceVersion for updates
			if ctrlObj.GetObjectKind().GroupVersionKind().Kind == "ServiceMonitor" {
				found := &unstructured.Unstructured{}
				found.SetGroupVersionKind(ctrlObj.GetObjectKind().GroupVersionKind())
				err = ctrlClient.Get(ctx, client.ObjectKey{Name: ctrlObj.GetName(), Namespace: ctrlObj.GetNamespace()}, found)
				if err == nil {
					// Copy resourceVersion from existing object for optimistic concurrency
					ctrlObj.SetResourceVersion(found.GetResourceVersion())
				}
			}
			if err := operatorutils.ApplyCTRLObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	// update Deployment
	if isDeleting {
		// Delete Deployment
		deploymentKey := client.ObjectKey{
			Namespace: *dpApply.Namespace,
			Name:      *dpApply.Name,
		}
		deploymentObj := &appsv1.Deployment{}
		if err = ctrlClient.Get(ctx, deploymentKey, deploymentObj); err == nil {
			if err = ctrlClient.Delete(ctx, deploymentObj); err != nil && !k8serrors.IsNotFound(err) {
				return fmt.Errorf("error deleting deployment: %v", err)
			}
		} else {
			log.Infow("error getting deployment", "deploymentKey", deploymentKey)
		}
	} else {
		// Create/Update Deployment
		if err = deployment.SyncDeployment(ctx, *dpApply, k8sClient, cr.Name); err != nil {
			return err
		}
	}

	return nil
}

// getPowerFlexMetricsObject - get powerflex metrics yaml string
func getPowerFlexMetricsObject(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, matched operatorutils.VersionSpec) ([]crclient.Object, error) {
	obs, err := getObservabilityModuleFn(cr)
	if err != nil {
		return nil, err
	}

	buf, err := readConfigFileFn(ctx, obs, cr, op, PflexObsYamlFile)
	if err != nil {
		return nil, err
	}
	YamlString := string(buf)

	obsMetricsConfig := getObsMetricsConfig(obs, DefaultPowerFlexObsMetricsPort, DefaultPowerFlexObsServiceMonitorInterval)
	obsMetricsEnabled := fmt.Sprintf("%t", obsMetricsConfig.enabled)
	obsMetricsPort := fmt.Sprintf("%d", obsMetricsConfig.port)

	otelCollectorAddress := "otel-collector:55680"
	pflexImage := ""
	maxConcurrentQueries := "10"
	sdcEnabled := "true"
	volumeEnabled := "true"
	storagePoolEnabled := "true"
	sdcPollFrequency := "10"
	volumePollFrequency := "10"
	storagePoolPollFrequency := "10"
	topologyEnabled := "true"
	topologyPollFrequency := "30"
	logFormat := "json"
	logLevel := "info"

	for _, component := range obs.Components {
		if component.Name == ObservabilityMetricsPowerFlexName {
			pflexImage = operatorutils.GetFinalImage(ctx, cr, matched, component, YamlString)
			for _, env := range component.Envs {
				if strings.Contains(PowerflexLogLevel, env.Name) {
					logLevel = strings.ToLower(strings.TrimSpace(env.Value))
				} else if strings.Contains(PowerflexMaxConcurrentQueries, env.Name) {
					maxConcurrentQueries = env.Value
				} else if strings.Contains(PowerflexSdcMetricsEnabled, env.Name) {
					sdcEnabled = env.Value
				} else if strings.Contains(PowerflexSdcIoPollFrequency, env.Name) {
					sdcPollFrequency = env.Value
				} else if strings.Contains(PowerflexTopologyMetricsEnabled, env.Name) {
					topologyEnabled = env.Value
				} else if strings.Contains(PowerflexTopologyMetricsPollFrequency, env.Name) {
					topologyPollFrequency = env.Value
				} else if strings.Contains(PowerflexVolumeMetricsEnabled, env.Name) {
					volumeEnabled = env.Value
				} else if strings.Contains(PowerflexVolumeIoPollFrequency, env.Name) {
					volumePollFrequency = env.Value
				} else if strings.Contains(PowerflexStoragePoolMetricsEnabled, env.Name) {
					storagePoolEnabled = env.Value
				} else if strings.Contains(PowerflexStoragePoolPollFrequency, env.Name) {
					storagePoolPollFrequency = env.Value
				} else if strings.Contains(PowerflexLogFormat, env.Name) {
					logFormat = strings.ToLower(strings.TrimSpace(env.Value))
				} else if strings.Contains(OtelCollectorAddress, env.Name) {
					otelCollectorAddress = env.Value
				}
			}
		}
	}

	// Validate CR fields and user-controlled values before YAML substitution
	if err := operatorutils.ValidateKubernetesName(cr.Name); err != nil {
		return nil, fmt.Errorf("invalid CR name for YAML substitution: %w", err)
	}
	if err := operatorutils.ValidateKubernetesNamespace(cr.Namespace); err != nil {
		return nil, fmt.Errorf("invalid CR namespace for YAML substitution: %w", err)
	}
	// Validate all environment-sourced values (including numeric/boolean values to prevent injection)
	userControlledValues := map[string]string{
		"logLevel":                 logLevel,
		"logFormat":                logFormat,
		"otelCollectorAddress":     otelCollectorAddress,
		"maxConcurrentQueries":     maxConcurrentQueries,
		"sdcEnabled":               sdcEnabled,
		"volumeEnabled":            volumeEnabled,
		"storagePoolEnabled":       storagePoolEnabled,
		"sdcPollFrequency":         sdcPollFrequency,
		"volumePollFrequency":      volumePollFrequency,
		"storagePoolPollFrequency": storagePoolPollFrequency,
		"topologyEnabled":          topologyEnabled,
		"topologyPollFrequency":    topologyPollFrequency,
	}
	for fieldName, value := range userControlledValues {
		// Only validate non-empty values (some fields are optional)
		if value != "" {
			if err := operatorutils.ValidateYAMLSubstitutionValue(value, fieldName); err != nil {
				return nil, fmt.Errorf("invalid value for YAML substitution: %w", err)
			}
		}
	}

	YamlString = strings.ReplaceAll(YamlString, CSMName, cr.Name)
	YamlString = strings.ReplaceAll(YamlString, CSMNameSpace, cr.Namespace)
	YamlString = strings.ReplaceAll(YamlString, PowerflexLogLevel, logLevel)
	YamlString = strings.ReplaceAll(YamlString, PowerflexMaxConcurrentQueries, maxConcurrentQueries)
	YamlString = strings.ReplaceAll(YamlString, PowerflexSdcMetricsEnabled, sdcEnabled)
	YamlString = strings.ReplaceAll(YamlString, PowerflexSdcIoPollFrequency, sdcPollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PowerflexVolumeMetricsEnabled, volumeEnabled)
	YamlString = strings.ReplaceAll(YamlString, PowerflexVolumeIoPollFrequency, volumePollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PowerflexStoragePoolMetricsEnabled, storagePoolEnabled)
	YamlString = strings.ReplaceAll(YamlString, PowerflexStoragePoolPollFrequency, storagePoolPollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PowerflexTopologyMetricsEnabled, topologyEnabled)
	YamlString = strings.ReplaceAll(YamlString, PowerflexTopologyMetricsPollFrequency, topologyPollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PowerflexLogFormat, logFormat)
	YamlString = strings.ReplaceAll(YamlString, OtelCollectorAddress, otelCollectorAddress)
	YamlString = strings.ReplaceAll(YamlString, constants.CsiMetricsEnabled, obsMetricsEnabled)
	YamlString = strings.ReplaceAll(YamlString, constants.CsiMetricsPort, obsMetricsPort)
	YamlString = strings.ReplaceAll(YamlString, DriverDefaultReleaseName, cr.Name)

	metricsObjects, err := operatorutils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return nil, err
	}
	metricsObjects, err = applyObsMetricsConfig(metricsObjects, obsMetricsConfig, "karavi-metrics-powerflex")
	if err != nil {
		return nil, err
	}

	operatorutils.SetContainerImage(metricsObjects, "karavi-metrics-powerflex", "karavi-metrics-powerflex", pflexImage)

	return metricsObjects, nil
}

// getObservabilityModule - get instance of observability module
func getObservabilityModule(cr csmv1.ContainerStorageModule) (csmv1.Module, error) {
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Observability {
			return m, nil
		}
	}
	return csmv1.Module{}, fmt.Errorf("could not find observability module")
}

// getIssuerCertServiceObs - gets cert manager issuer and certificate manifest for observability
func getIssuerCertServiceObs(ctx context.Context, op operatorutils.OperatorConfig, obs csmv1.Module, componentName string, cr csmv1.ContainerStorageModule) (string, error) {
	yamlString := ""
	certificate := ""
	privateKey := ""

	for _, component := range obs.Components {
		if component.Name == componentName {
			certificate = component.Certificate
			privateKey = component.PrivateKey
		}
	}

	// If we have at least one of the certificate or privateKey fields filled in, we assume the customer is trying to use a custom cert.
	// Otherwise, we give them the self-signed cert.
	if certificate != "" || privateKey != "" {
		if certificate != "" && privateKey != "" {
			buf, err := readConfigFile(ctx, obs, cr, op, CustomCert)
			if err != nil {
				return yamlString, err
			}

			yamlString = string(buf)
		} else {
			return yamlString, fmt.Errorf("observability install failed -- either cert or privatekey missing for %s custom cert", componentName)
		}
	} else {
		buf, err := readConfigFile(ctx, obs, cr, op, SelfSignedCert)
		if err != nil {
			return yamlString, err
		}

		yamlString = string(buf)
	}

	yamlString = strings.ReplaceAll(yamlString, ObservabilityCertificate, certificate)
	yamlString = strings.ReplaceAll(yamlString, ObservabilityPrivateKey, privateKey)
	yamlString = strings.ReplaceAll(yamlString, ObservabilitySecretPrefix, ComponentNameToSecretPrefix[componentName])
	yamlString = strings.ReplaceAll(yamlString, CSMNameSpace, cr.Namespace)

	return yamlString, nil
}

// isRetryableWebhookError checks if an error is related to cert-manager webhook connectivity
// or certificate verification issues. These errors are retryable with exponential backoff.
// Note: String matching is used because Kubernetes API errors don't provide typed errors
// for these specific webhook/certificate errors. The patterns are based on common
// cert-manager webhook error messages and are relatively stable.
func isRetryableWebhookError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Check for common retryable error patterns from cert-manager webhooks
	return strings.Contains(errStr, "tls: failed to verify certificate") ||
		strings.Contains(errStr, "x509: certificate signed by unknown authority") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "webhook") ||
		strings.Contains(errStr, "Internal error occurred: failed calling webhook")
}

// IssuerCertServiceObs - apply and delete the observability issuer and certificate service
func IssuerCertServiceObs(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	obs, err := getObservabilityModule(cr)
	if err != nil {
		return err
	}

	for _, component := range obs.Components {
		if (component.Name == ObservabilityOtelCollectorName && *component.Enabled) || (component.Name == ObservabilityTopologyName && *component.Enabled) || (component.Name == ObservabilityMetricsPowerStoreName && *component.Enabled) {
			yamlString, err := getIssuerCertServiceObs(ctx, op, obs, component.Name, cr)
			if err != nil {
				return err
			}

			// Retry logic with exponential backoff for webhook-related errors
			// Note: These values are hardcoded based on industry best practices:
			// - 5 retries provides sufficient attempts for transient webhook issues
			// - 2s base delay with exponential backoff results in ~62s total wait time
			// - This balances fast recovery with avoiding excessive retries
			// Consider making configurable via environment variables if tuning is needed for specific environments
			maxRetries := 5
			baseDelay := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt < maxRetries; attempt++ {
				err = applyOrDeleteObjects(ctx, ctrlClient, yamlString, isDeleting)
				if err == nil {
					break
				}

				lastErr = err

				// Only retry on webhook-related errors
				if !isRetryableWebhookError(err) {
					return err
				}

				// If this is the last attempt, don't wait
				if attempt == maxRetries-1 {
					break
				}

				// Exponential backoff
				delay := baseDelay * time.Duration(1<<uint(attempt))
				log := logger.GetLogger(ctx)
				log.Warnw("Webhook error encountered, retrying",
					"component", component.Name,
					"attempt", attempt+1,
					"maxRetries", maxRetries,
					"delay", delay,
					"error", err)

				select {
				case <-time.After(delay):
				case <-ctx.Done():
					if lastErr != nil {
						return fmt.Errorf("%w: %v", ctx.Err(), lastErr)
					}
					return ctx.Err()
				}
			}

			if lastErr != nil {
				return fmt.Errorf("failed to apply certificate resources for %s after %d retries: %w", component.Name, maxRetries, lastErr)
			}
		}
	}

	return nil
}

// PowerMaxMetrics - delete or update powermax metrics objects
func PowerMaxMetrics(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client, k8sClient kubernetes.Interface) error {
	log := logger.GetLogger(ctx)
	var matched operatorutils.VersionSpec
	if cr.Spec.Version != "" {
		var err error
		matched, err = operatorutils.ResolveVersionFromConfigMap(ctx, ctrlClient, &cr)
		if err != nil {
			log.Error(err, "Failed to get version from configmap")
			return err
		}
	}

	powerMaxMetricsObjects, err := getPowerMaxMetricsObject(ctx, op, cr, matched)
	if err != nil {
		return err
	}

	// update secret volume and inject authorization to deployment
	var dpApply *confv1.DeploymentApplyConfiguration
	foundDp := false
	for i, obj := range powerMaxMetricsObjects {
		if deployment, ok := obj.(*appsv1.Deployment); ok {
			dpApply, err = parseObservabilityMetricsDeployment(ctx, deployment, op, cr, ctrlClient)
			if err != nil {
				return err
			}
			foundDp = true
			powerMaxMetricsObjects[i] = powerMaxMetricsObjects[len(powerMaxMetricsObjects)-1]
			powerMaxMetricsObjects = powerMaxMetricsObjects[:len(powerMaxMetricsObjects)-1]
			break
		}
	}
	if !foundDp {
		return fmt.Errorf("could not find deployment obj")
	}

	version, err := operatorutils.GetVersion(ctx, &cr, op)
	if err != nil {
		return err
	}
	// Dynamic secret/configMap mounting is only supported in v2.14.0 and above
	secretSupported, err := operatorutils.MinVersionCheck(drivers.PowerMaxMountCredentialMinVersion, version)
	if err != nil {
		return err
	}

	useSecret := drivers.UseReverseProxySecret(&cr)
	if secretSupported && useSecret {
		// Append config map or mount cred secret.
		// We ensure that we pass through the DeploymentApplyConfiguration.
		_ = drivers.DynamicallyMountPowermaxContent(dpApply, cr)
	}

	if !useSecret {
		err := setPowerMaxMetricsConfigMap(dpApply, cr)
		if err != nil {
			return err
		}
	}

	obs, err := getObservabilityModule(cr)
	if err != nil {
		return err
	}
	pmaxObsMetricsConfig := getObsMetricsConfig(obs, DefaultPowerMaxObsMetricsPort, DefaultPowerMaxObsServiceMonitorInterval)

	if !pmaxObsMetricsConfig.enabled || !pmaxObsMetricsConfig.serviceMonitorEnabled {
		serviceMonitor := &unstructured.Unstructured{}
		serviceMonitor.SetAPIVersion("monitoring.coreos.com/v1")
		serviceMonitor.SetKind("ServiceMonitor")
		serviceMonitor.SetName("karavi-metrics-powermax-obs-monitor")
		serviceMonitor.SetNamespace(cr.Namespace)
		if err := operatorutils.DeleteObject(ctx, serviceMonitor, ctrlClient); err != nil {
			return err
		}
	}

	for _, ctrlObj := range powerMaxMetricsObjects {
		if isDeleting {
			if err := operatorutils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			// ServiceMonitor requires special handling to fetch resourceVersion for updates
			if ctrlObj.GetObjectKind().GroupVersionKind().Kind == "ServiceMonitor" {
				found := &unstructured.Unstructured{}
				found.SetGroupVersionKind(ctrlObj.GetObjectKind().GroupVersionKind())
				err = ctrlClient.Get(ctx, client.ObjectKey{Name: ctrlObj.GetName(), Namespace: ctrlObj.GetNamespace()}, found)
				if err == nil {
					// Copy resourceVersion from existing object for optimistic concurrency
					ctrlObj.SetResourceVersion(found.GetResourceVersion())
				}
			}
			if err := operatorutils.ApplyCTRLObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	// update Deployment
	if isDeleting {
		// Delete Deployment
		deploymentKey := client.ObjectKey{
			Namespace: *dpApply.Namespace,
			Name:      *dpApply.Name,
		}
		deploymentObj := &appsv1.Deployment{}
		if err = ctrlClient.Get(ctx, deploymentKey, deploymentObj); err == nil {
			if err = ctrlClient.Delete(ctx, deploymentObj); err != nil && !k8serrors.IsNotFound(err) {
				return fmt.Errorf("error deleting deployment: %v", err)
			}
		} else {
			log.Infow("error getting deployment", "deploymentKey", deploymentKey)
		}
	} else {
		// Create/Update Deployment
		if err = deployment.SyncDeployment(ctx, *dpApply, k8sClient, cr.Name); err != nil {
			return err
		}
	}

	return nil
}

func setPowerMaxMetricsConfigMap(dp *confv1.DeploymentApplyConfiguration, cr csmv1.ContainerStorageModule) error {
	obs, err := getObservabilityModule(cr)
	if err != nil {
		// Observability module not found
		return err
	}

	cm := "powermax-reverseproxy-config"
	// Get the config map name from the observability module
	for _, component := range obs.Components {
		if component.Name == ObservabilityMetricsPowerMaxName {
			for _, env := range component.Envs {
				if env.Name == "X_CSI_CONFIG_MAP_NAME" {
					cm = env.Value
					break
				}
			}
		}
	}

	optional := false
	vol := acorev1.VolumeApplyConfiguration{
		Name: &cm,
		VolumeSourceApplyConfiguration: acorev1.VolumeSourceApplyConfiguration{
			ConfigMap: &acorev1.ConfigMapVolumeSourceApplyConfiguration{
				LocalObjectReferenceApplyConfiguration: acorev1.LocalObjectReferenceApplyConfiguration{Name: &cm},
				Optional:                               &optional,
			},
		},
	}

	// Dynamically add the volume
	contains := slices.ContainsFunc(dp.Spec.Template.Spec.Volumes,
		func(v acorev1.VolumeApplyConfiguration) bool { return *v.Name == *vol.Name },
	)
	if !contains {
		dp.Spec.Template.Spec.Volumes = append(dp.Spec.Template.Spec.Volumes, vol)
	}

	mountPath := "/etc/reverseproxy"
	volumeMount := acorev1.VolumeMountApplyConfiguration{Name: &cm, MountPath: &mountPath}
	contains = slices.ContainsFunc(
		dp.Spec.Template.Spec.Containers[0].VolumeMounts,
		func(v acorev1.VolumeMountApplyConfiguration) bool {
			// Cast to pull out value instead of comparing addresses.
			return *v.Name == *volumeMount.Name
		},
	)

	if !contains {
		dp.Spec.Template.Spec.Containers[0].VolumeMounts = append(dp.Spec.Template.Spec.Containers[0].VolumeMounts, volumeMount)
	}

	return nil
}

// getPowerMaxMetricsObject - get powermax metrics yaml string
func getPowerMaxMetricsObject(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, matched operatorutils.VersionSpec) ([]crclient.Object, error) {
	obs, err := getObservabilityModule(cr)
	if err != nil {
		return nil, err
	}

	buf, err := readConfigFile(ctx, obs, cr, op, PMaxObsYamlFile)
	if err != nil {
		return nil, err
	}
	YamlString := string(buf)

	otelCollectorAddress := "otel-collector:55680"
	pmaxImage := ""
	maxConcurrentQueries := "10"
	capacityEnabled := "true"
	perfEnabled := "true"
	topologyEnabled := "true"
	topologyPollFrequency := "300"
	capacityPollFrequency := "3600"
	perfPollFrequency := "300"
	logFormat := "json"
	logLevel := "info"
	revproxyConfigMap := "powermax-reverseproxy-config"

	for _, component := range obs.Components {
		if component.Name == ObservabilityMetricsPowerMaxName {
			pmaxImage = operatorutils.GetFinalImage(ctx, cr, matched, component, YamlString)
			for _, env := range component.Envs {
				if strings.Contains(PmaxLogLevel, env.Name) {
					logLevel = strings.ToLower(strings.TrimSpace(env.Value))
				} else if strings.Contains(PmaxConcurrentQueries, env.Name) {
					maxConcurrentQueries = env.Value
				} else if strings.Contains(PmaxCapacityMetricsEnabled, env.Name) {
					capacityEnabled = env.Value
				} else if strings.Contains(PmaxCapacityPollFreq, env.Name) {
					capacityPollFrequency = env.Value
				} else if strings.Contains(PmaxPerformanceMetricsEnabled, env.Name) {
					perfEnabled = env.Value
				} else if strings.Contains(PmaxPerformancePollFreq, env.Name) {
					perfPollFrequency = env.Value
				} else if strings.Contains(PmaxTopologyMetricsEnabled, env.Name) {
					topologyEnabled = env.Value
				} else if strings.Contains(PmaxTopologyMetricsPollFrequency, env.Name) {
					topologyPollFrequency = env.Value
				} else if strings.Contains(ReverseProxyConfigMap, env.Name) {
					revproxyConfigMap = env.Value
				} else if strings.Contains(PmaxLogFormat, env.Name) {
					logFormat = strings.ToLower(strings.TrimSpace(env.Value))
				} else if strings.Contains(OtelCollectorAddress, env.Name) {
					otelCollectorAddress = env.Value
				}
			}
		}
	}

	// Validate CR fields and user-controlled values before YAML substitution
	if err := operatorutils.ValidateKubernetesName(cr.Name); err != nil {
		return nil, fmt.Errorf("invalid CR name for YAML substitution: %w", err)
	}
	if err := operatorutils.ValidateKubernetesNamespace(cr.Namespace); err != nil {
		return nil, fmt.Errorf("invalid CR namespace for YAML substitution: %w", err)
	}
	// Validate all environment-sourced values (including numeric/boolean values to prevent injection)
	userControlledValues := map[string]string{
		"logLevel":              logLevel,
		"logFormat":             logFormat,
		"otelCollectorAddress":  otelCollectorAddress,
		"maxConcurrentQueries":  maxConcurrentQueries,
		"capacityEnabled":       capacityEnabled,
		"perfEnabled":           perfEnabled,
		"topologyEnabled":       topologyEnabled,
		"topologyPollFrequency": topologyPollFrequency,
		"capacityPollFrequency": capacityPollFrequency,
		"perfPollFrequency":     perfPollFrequency,
		"revproxyConfigMap":     revproxyConfigMap,
	}
	for fieldName, value := range userControlledValues {
		// Only validate non-empty values (some fields are optional)
		if value != "" {
			if err := operatorutils.ValidateYAMLSubstitutionValue(value, fieldName); err != nil {
				return nil, fmt.Errorf("invalid value for YAML substitution: %w", err)
			}
		}
	}

	YamlString = strings.ReplaceAll(YamlString, CSMName, cr.Name)
	YamlString = strings.ReplaceAll(YamlString, CSMNameSpace, cr.Namespace)
	YamlString = strings.ReplaceAll(YamlString, PmaxLogLevel, logLevel)
	YamlString = strings.ReplaceAll(YamlString, PmaxLogFormat, logFormat)
	YamlString = strings.ReplaceAll(YamlString, PmaxConcurrentQueries, maxConcurrentQueries)
	YamlString = strings.ReplaceAll(YamlString, PmaxCapacityMetricsEnabled, capacityEnabled)
	YamlString = strings.ReplaceAll(YamlString, PmaxCapacityPollFreq, capacityPollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PmaxPerformanceMetricsEnabled, perfEnabled)
	YamlString = strings.ReplaceAll(YamlString, PmaxPerformancePollFreq, perfPollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PmaxTopologyMetricsEnabled, topologyEnabled)
	YamlString = strings.ReplaceAll(YamlString, PmaxTopologyMetricsPollFrequency, topologyPollFrequency)
	YamlString = strings.ReplaceAll(YamlString, OtelCollectorAddress, otelCollectorAddress)
	YamlString = strings.ReplaceAll(YamlString, ReverseProxyConfigMap, revproxyConfigMap)

	pmaxObsConfig := getObsMetricsConfig(obs, DefaultPowerMaxObsMetricsPort, DefaultPowerMaxObsServiceMonitorInterval)
	YamlString = strings.ReplaceAll(YamlString, constants.CsiMetricsEnabled, fmt.Sprintf("%t", pmaxObsConfig.enabled))
	YamlString = strings.ReplaceAll(YamlString, constants.CsiMetricsPort, fmt.Sprintf("%d", pmaxObsConfig.port))

	YamlString = strings.ReplaceAll(YamlString, DriverDefaultReleaseName, cr.Name)

	metricsObjects, err := operatorutils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return nil, err
	}
	metricsObjects, err = applyObsMetricsConfig(metricsObjects, pmaxObsConfig, "karavi-metrics-powermax")
	if err != nil {
		return nil, err
	}

	operatorutils.SetContainerImage(metricsObjects, "karavi-metrics-powermax", "karavi-metrics-powermax", pmaxImage)

	return metricsObjects, nil
}
