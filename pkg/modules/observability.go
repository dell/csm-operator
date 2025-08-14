// Copyright (c) 2025 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package modules

import (
	"context"
	"fmt"
	"slices"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	csmv1 "github.com/dell/csm-operator/api/v1"
	drivers "github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/logger"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	"github.com/dell/csm-operator/pkg/resources/deployment"
	appsv1 "k8s.io/api/apps/v1"
	confv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
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
)

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

var defaultSecretsName = map[csmv1.DriverType]string{
	csmv1.PowerScale:     "<DriverDefaultReleaseName>-creds",
	csmv1.PowerScaleName: "<DriverDefaultReleaseName>-creds",
	csmv1.PowerFlex:      "<DriverDefaultReleaseName>-config",
	csmv1.PowerFlexName:  "<DriverDefaultReleaseName>-config",
	csmv1.PowerMax:       "<DriverDefaultReleaseName>-creds",
	csmv1.PowerStore:     "<DriverDefaultReleaseName>-config",
}

var defaultAuthSecretsName = []string{"karavi-authorization-config", "proxy-authz-tokens", "proxy-server-root-certificate"}

// ObservabilityPrecheck  - runs precheck for CSM Otoolsabilitytools
func ObservabilityPrecheck(ctx context.Context, op operatorutils.OperatorConfig, obs csmv1.Module, cr csmv1.ContainerStorageModule, _ operatorutils.ReconcileCSM) error {
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

	log.Infof("\nperformed pre checks for: %s", obs.Name)
	return nil
}

// ObservabilityTopology - delete or update topology objectstools
func ObservabilityTopology(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)
	configVersion := cr.Spec.Driver.ConfigVersion
	if strings.Contains(configVersion, "v2.13") || strings.Contains(configVersion, "v2.14") {
		topoObjects, err := getTopology(op, cr)
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

func getTopology(op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) ([]crclient.Object, error) {
	obs, err := getObservabilityModule(cr)
	if err != nil {
		return nil, err
	}

	buf, err := readConfigFile(obs, cr, op, TopologyYamlFile)
	if err != nil {
		return nil, err
	}
	YamlString := string(buf)

	logLevel := "INFO"
	topologyImage := ""

	for _, component := range obs.Components {
		if component.Name == ObservabilityTopologyName {
			if component.Image != "" {
				topologyImage = string(component.Image)
			}
			for _, env := range component.Envs {
				if strings.Contains(TopologyLogLevel, env.Name) {
					logLevel = env.Value
				}
			}
		}
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
func OtelCollector(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	YamlString, err := getOtelCollector(op, cr)
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
func getOtelCollector(op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	obs, err := getObservabilityModule(cr)
	if err != nil {
		return YamlString, err
	}

	buf, err := readConfigFile(obs, cr, op, OtelCollectorYamlFile)
	if err != nil {
		return YamlString, err
	}
	YamlString = string(buf)

	nginxProxyImage := "nginxinc/nginx-unprivileged:1.27"
	otelCollectorImage := "ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector:0.131.0"
	configVersion := cr.Spec.Driver.ConfigVersion
	// Currently supported config versions by this operator(release candidate for CSM v2.14.0) are v2.11.0, v2.12.0, v2.13.0.
	// These config versions were already supported by the released operators. So use the same otel image for them.
	if configVersion == "v2.11.0" || configVersion == "v2.12.0" || configVersion == "v2.13.0" {
		otelCollectorImage = "otel/opentelemetry-collector:0.42.0"
	}

	for _, component := range obs.Components {
		if component.Name == ObservabilityOtelCollectorName {
			if component.Image != "" {
				otelCollectorImage = string(component.Image)
			}
			for _, env := range component.Envs {
				if strings.Contains(NginxProxyImage, env.Name) {
					nginxProxyImage = env.Value
				}
			}
		}
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

	powerscaleMetricsObjects, err := getPowerScaleMetricsObjects(op, cr)
	if err != nil {
		return err
	}

	// update secret volume and inject authorization to deployment
	var dpApply *confv1.DeploymentApplyConfiguration
	foundDp := false
	for i, obj := range powerscaleMetricsObjects {
		if deployment, ok := obj.(*appsv1.Deployment); ok {
			dpApply, err = parseObservabilityMetricsDeployment(ctx, deployment, op, cr)
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

	for _, ctrlObj := range powerscaleMetricsObjects {
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

	powerstoreMetricsObjects, err := getPowerStoreMetricsObjects(op, cr)
	if err != nil {
		return err
	}

	// update deployment for powerstore metrics
	var dpApply *confv1.DeploymentApplyConfiguration
	foundDp := false
	for i, obj := range powerstoreMetricsObjects {
		if deployment, ok := obj.(*appsv1.Deployment); ok {
			dpApply, err = parseObservabilityMetricsDeployment(ctx, deployment, op, cr)
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

	for _, ctrlObj := range powerstoreMetricsObjects {
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
func getPowerStoreMetricsObjects(op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) ([]crclient.Object, error) {
	obs, err := getObservabilityModule(cr)
	if err != nil {
		return nil, err
	}

	buf, err := readConfigFile(obs, cr, op, PstoreObsYamlFile)
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
	zipkinURI := ""
	zipkinServiceName := "metrics-powerstore"
	zipkinProbability := "0.0"
	logLevel := "INFO"
	logFormat := "TEXT"
	otelCollectorAddress := "otel-collector:55680"

	for _, component := range obs.Components {
		if component.Name == ObservabilityMetricsPowerStoreName {
			if component.Image != "" {
				obsPstoreImage = string(component.Image)
			}
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
				} else if strings.Contains(ZipkinURI, env.Name) {
					zipkinURI = env.Value
				} else if strings.Contains(ZipkinServiceName, env.Name) {
					zipkinServiceName = env.Value
				} else if strings.Contains(ZipkinProbability, env.Name) {
					zipkinProbability = env.Value
				} else if strings.Contains(PstoreLogLevel, env.Name) {
					logLevel = env.Value
				} else if strings.Contains(PstoreLogFormat, env.Name) {
					logFormat = env.Value
				} else if strings.Contains(OtelCollectorAddress, env.Name) {
					otelCollectorAddress = env.Value
				}
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
	YamlString = strings.ReplaceAll(YamlString, ZipkinURI, zipkinURI)
	YamlString = strings.ReplaceAll(YamlString, ZipkinServiceName, zipkinServiceName)
	YamlString = strings.ReplaceAll(YamlString, ZipkinProbability, zipkinProbability)
	YamlString = strings.ReplaceAll(YamlString, PstoreLogLevel, logLevel)
	YamlString = strings.ReplaceAll(YamlString, PstoreLogFormat, logFormat)
	YamlString = strings.ReplaceAll(YamlString, OtelCollectorAddress, otelCollectorAddress)
	YamlString = strings.ReplaceAll(YamlString, DriverDefaultReleaseName, cr.Name)

	metricsObjects, err := operatorutils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return nil, err
	}
	operatorutils.SetContainerImage(metricsObjects, "karavi-metrics-powerstore", "karavi-metrics-powerstore", obsPstoreImage)

	return metricsObjects, nil
}

// getPowerScaleMetricsObjects - get powerscale metrics yaml string
func getPowerScaleMetricsObjects(op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) ([]crclient.Object, error) {
	obs, err := getObservabilityModule(cr)
	if err != nil {
		return nil, err
	}

	buf, err := readConfigFile(obs, cr, op, PscaleObsYamlFile)
	if err != nil {
		return nil, err
	}
	YamlString := string(buf)

	logLevel := "INFO"
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
	logFormat := "TEXT"

	for _, component := range obs.Components {
		if component.Name == ObservabilityMetricsPowerScaleName {
			if component.Image != "" {
				pscaleImage = string(component.Image)
			}
			for _, env := range component.Envs {
				if strings.Contains(PowerscaleLogLevel, env.Name) {
					logLevel = env.Value
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
					logFormat = env.Value
				} else if strings.Contains(OtelCollectorAddress, env.Name) {
					otelCollectorAddress = env.Value
				}
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
	YamlString = strings.ReplaceAll(YamlString, DriverDefaultReleaseName, cr.Name)

	metricsObjects, err := operatorutils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return nil, err
	}
	operatorutils.SetContainerImage(metricsObjects, "karavi-metrics-powerscale", "karavi-metrics-powerscale", pscaleImage)

	return metricsObjects, nil
}

// parseObservabilityMetricsDeployment - update secret volume and inject authorization to deployment
func parseObservabilityMetricsDeployment(ctx context.Context, deployment *appsv1.Deployment, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) (*confv1.DeploymentApplyConfiguration, error) {
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
		dpApply, err = AuthInjectDeployment(*dpApply, cr, op)
		if err != nil {
			return nil, fmt.Errorf("injecting auth into Observability metrics deployment: %v", err)
		}
	}
	return dpApply, nil
}

// PowerFlexMetrics - delete or update powerflex metrics objects
func PowerFlexMetrics(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client, k8sClient kubernetes.Interface) error {
	log := logger.GetLogger(ctx)

	powerflexMetricsObjects, err := getPowerFlexMetricsObject(op, cr)
	if err != nil {
		return err
	}

	// update secret volume and inject authorization to deployment
	var dpApply *confv1.DeploymentApplyConfiguration
	foundDp := false
	for i, obj := range powerflexMetricsObjects {
		if deployment, ok := obj.(*appsv1.Deployment); ok {
			dpApply, err = parseObservabilityMetricsDeployment(ctx, deployment, op, cr)
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
func getPowerFlexMetricsObject(op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) ([]crclient.Object, error) {
	obs, err := getObservabilityModule(cr)
	if err != nil {
		return nil, err
	}

	buf, err := readConfigFile(obs, cr, op, PflexObsYamlFile)
	if err != nil {
		return nil, err
	}
	YamlString := string(buf)

	otelCollectorAddress := "otel-collector:55680"
	pflexImage := ""
	maxConcurrentQueries := "10"
	sdcEnabled := "true"
	volumeEnabled := "true"
	storagePoolEnabled := "true"
	sdcPollFrequency := "10"
	volumePollFrequency := "10"
	storagePoolPollFrequency := "10"
	logFormat := "TEXT"
	logLevel := "INFO"

	for _, component := range obs.Components {
		if component.Name == ObservabilityMetricsPowerFlexName {
			if component.Image != "" {
				pflexImage = string(component.Image)
			}
			for _, env := range component.Envs {
				if strings.Contains(PowerflexLogLevel, env.Name) {
					logLevel = env.Value
				} else if strings.Contains(PowerflexMaxConcurrentQueries, env.Name) {
					maxConcurrentQueries = env.Value
				} else if strings.Contains(PowerflexSdcMetricsEnabled, env.Name) {
					sdcEnabled = env.Value
				} else if strings.Contains(PowerflexSdcIoPollFrequency, env.Name) {
					sdcPollFrequency = env.Value
				} else if strings.Contains(PowerflexVolumeMetricsEnabled, env.Name) {
					volumeEnabled = env.Value
				} else if strings.Contains(PowerflexVolumeIoPollFrequency, env.Name) {
					volumePollFrequency = env.Value
				} else if strings.Contains(PowerflexStoragePoolMetricsEnabled, env.Name) {
					storagePoolEnabled = env.Value
				} else if strings.Contains(PowerflexStoragePoolPollFrequency, env.Name) {
					storagePoolPollFrequency = env.Value
				} else if strings.Contains(PowerflexLogFormat, env.Name) {
					logFormat = env.Value
				} else if strings.Contains(OtelCollectorAddress, env.Name) {
					otelCollectorAddress = env.Value
				}
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
	YamlString = strings.ReplaceAll(YamlString, PowerflexLogFormat, logFormat)
	YamlString = strings.ReplaceAll(YamlString, OtelCollectorAddress, otelCollectorAddress)
	YamlString = strings.ReplaceAll(YamlString, DriverDefaultReleaseName, cr.Name)

	metricsObjects, err := operatorutils.GetModuleComponentObj([]byte(YamlString))
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
func getIssuerCertServiceObs(op operatorutils.OperatorConfig, obs csmv1.Module, componentName string, cr csmv1.ContainerStorageModule) (string, error) {
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
			buf, err := readConfigFile(obs, cr, op, CustomCert)
			if err != nil {
				return yamlString, err
			}

			yamlString = string(buf)
		} else {
			return yamlString, fmt.Errorf("observability install failed -- either cert or privatekey missing for %s custom cert", componentName)
		}
	} else {
		buf, err := readConfigFile(obs, cr, op, SelfSignedCert)
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

// IssuerCertServiceObs - apply and delete the observability issuer and certificate service
func IssuerCertServiceObs(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	obs, err := getObservabilityModule(cr)
	if err != nil {
		return err
	}

	for _, component := range obs.Components {
		if (component.Name == ObservabilityOtelCollectorName && *(component.Enabled)) || (component.Name == ObservabilityTopologyName && *(component.Enabled)) || (component.Name == ObservabilityMetricsPowerStoreName && *(component.Enabled)) {
			yamlString, err := getIssuerCertServiceObs(op, obs, component.Name, cr)
			if err != nil {
				return err
			}
			err = applyDeleteObjects(ctx, ctrlClient, yamlString, isDeleting)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// PowerMaxMetrics - delete or update powermax metrics objects
func PowerMaxMetrics(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client, k8sClient kubernetes.Interface) error {
	log := logger.GetLogger(ctx)

	powerMaxMetricsObjects, err := getPowerMaxMetricsObject(op, cr)
	if err != nil {
		return err
	}

	// update secret volume and inject authorization to deployment
	var dpApply *confv1.DeploymentApplyConfiguration
	foundDp := false
	for i, obj := range powerMaxMetricsObjects {
		if deployment, ok := obj.(*appsv1.Deployment); ok {
			dpApply, err = parseObservabilityMetricsDeployment(ctx, deployment, op, cr)
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

	// Dynamic secret/configMap mounting is only supported in v2.14.0 and above
	secretSupported, err := operatorutils.MinVersionCheck(drivers.PowerMaxMountCredentialMinVersion, cr.Spec.Driver.ConfigVersion)
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

	for _, ctrlObj := range powerMaxMetricsObjects {
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
		func(v acorev1.VolumeApplyConfiguration) bool { return *(v.Name) == *(vol.Name) },
	)
	if !contains {
		dp.Spec.Template.Spec.Volumes = append(dp.Spec.Template.Spec.Volumes, vol)
	}

	mountPath := "/etc/reverseproxy"
	volumeMount := acorev1.VolumeMountApplyConfiguration{Name: &cm, MountPath: &mountPath}
	contains = slices.ContainsFunc(dp.Spec.Template.Spec.Containers[0].VolumeMounts,
		func(v acorev1.VolumeMountApplyConfiguration) bool {
			// Cast to pull out value instead of comparing addresses.
			return *(v.Name) == *(volumeMount.Name)
		},
	)

	if !contains {
		dp.Spec.Template.Spec.Containers[0].VolumeMounts = append(dp.Spec.Template.Spec.Containers[0].VolumeMounts, volumeMount)
	}

	return nil
}

// getPowerMaxMetricsObject - get powermax metrics yaml string
func getPowerMaxMetricsObject(op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) ([]crclient.Object, error) {
	obs, err := getObservabilityModule(cr)
	if err != nil {
		return nil, err
	}

	buf, err := readConfigFile(obs, cr, op, PMaxObsYamlFile)
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
	topologyPollFrequency := "30"
	capacityPollFrequency := "10"
	perfPollFrequency := "10"
	logFormat := "TEXT"
	logLevel := "INFO"
	revproxyConfigMap := "powermax-reverseproxy-config"

	for _, component := range obs.Components {
		if component.Name == ObservabilityMetricsPowerMaxName {
			if component.Image != "" {
				pmaxImage = string(component.Image)
			}
			for _, env := range component.Envs {
				if strings.Contains(PmaxLogLevel, env.Name) {
					logLevel = env.Value
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
					logFormat = env.Value
				} else if strings.Contains(OtelCollectorAddress, env.Name) {
					otelCollectorAddress = env.Value
				}
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
	YamlString = strings.ReplaceAll(YamlString, DriverDefaultReleaseName, cr.Name)

	metricsObjects, err := operatorutils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return nil, err
	}
	operatorutils.SetContainerImage(metricsObjects, "karavi-metrics-powermax", "karavi-metrics-powermax", pmaxImage)

	return metricsObjects, nil
}
