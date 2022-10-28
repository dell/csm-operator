// Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"strconv"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	drivers "github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/resources/deployment"
	utils "github.com/dell/csm-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	confv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	// ObservabilityOtelCollectorName - component otel-collector
	ObservabilityOtelCollectorName string = "otel-collector"

	// ObservabilityTopologyName - component topology
	ObservabilityTopologyName string = "topology"

	// ObservabilityMetricsPowerScaleName - component metrics-powerscale
	ObservabilityMetricsPowerScaleName string = "metrics-powerscale"

	// ObservabilityMetricsPowerFlexName - component metrics-powerflex
	ObservabilityMetricsPowerFlexName string = "metrics-powerflex"

	// TopologyLogLevel -
	TopologyLogLevel string = "<TOPOLOGY_LOG_LEVEL>"

	// TopologyImage -
	TopologyImage string = "<TOPOLOGY_IMAGE>"

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

	// PowerScaleImage - PowerScale image name
	PowerScaleImage string = "<POWERSCALE_OBS_IMAGE>"

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
)

// ObservabilitySupportedDrivers is a map containing the CSI Drivers supported by CMS Replication. The key is driver name and the value is the driver plugin identifier
var ObservabilitySupportedDrivers = map[string]SupportedDriverParam{
	"powerscale": {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
	},
	"isilon": {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
	},
}

var defaultVolumeConfigName = map[csmv1.DriverType]string{
	csmv1.PowerScaleName: "isilon-creds",
	csmv1.PowerScale:     "isilon-creds",
}

// ObservabilityPrecheck  - runs precheck for CSM Observability
func ObservabilityPrecheck(ctx context.Context, op utils.OperatorConfig, obs csmv1.Module, cr csmv1.ContainerStorageModule, r utils.ReconcileCSM) error {
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

	// Pre-check For PowerScale secrets
	// Check for secret only
	secretName := cr.Name + "-creds"

	if cr.Spec.Driver.AuthSecret != "" {
		secretName = cr.Spec.Driver.AuthSecret
	}

	found := &corev1.Secret{}
	err := r.GetClient().Get(ctx, types.NamespacedName{Name: secretName,
		Namespace: utils.ObservabilityNamespace}, found)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to find secret %s and certificate validation is requested", secretName)
		}
		log.Error(err, "Failed to query for secret. Warning - the controller pod may not start")
	}

	//precheck for CSM Observability's Authorization
	if authorizationEnabled, _ := utils.IsModuleEnabled(ctx, cr, csmv1.Authorization); authorizationEnabled {
		if err := ObservabilityAuthPrecheck(ctx, cr, r.GetClient()); err != nil {
			return fmt.Errorf("failed authorization validation for Observability: %v", err)
		}
	}

	log.Infof("\nperformed pre checks for: %s", obs.Name)
	return nil
}

// ObservabilityAuthPrecheck  - runs precheck for CSM Observability's Authorization
func ObservabilityAuthPrecheck(ctx context.Context, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	auth := csmv1.Module{}
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Authorization {
			auth = m
			break
		}
	}

	secrets := []string{"karavi-authorization-config", "proxy-authz-tokens"}

	// Check for secrets
	for _, env := range auth.Components[0].Envs {
		if env.Name == "SKIP_CERTIFICATE_VALIDATION" {
			skipCertValid, err := strconv.ParseBool(env.Value)
			if err != nil {
				return fmt.Errorf("%s is an invalid value for SKIP_CERTIFICATE_VALIDATION: %v", env.Value, err)
			}

			if !skipCertValid {
				secrets = append(secrets, "proxy-server-root-certificate")
			}
			break
		}
	}

	for _, name := range secrets {
		found := &corev1.Secret{}
		err := ctrlClient.Get(ctx, types.NamespacedName{Name: name,
			Namespace: utils.ObservabilityNamespace}, found)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return fmt.Errorf("failed to find secret %s and certificate validation is requested", name)
			}
		}
	}

	return nil
}

// ObservabilityTopology - delete or update topology objects
func ObservabilityTopology(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	YamlString, err := getTopology(op, cr)
	if err != nil {
		return err
	}

	topoObjects, err := utils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range topoObjects {
		if isDeleting {
			if err := utils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	return nil
}

// getTopology - get topology yaml string
func getTopology(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	obs, err := getObservabilityModule(cr)
	if err != nil {
		return YamlString, err
	}

	buf, err := readConfigFile(obs, cr, op, TopologyYamlFile)
	if err != nil {
		return YamlString, err
	}
	YamlString = string(buf)

	logLevel := "INFO"
	topologyImage := "dellemc/csm-topology:v1.3.0"

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

	YamlString = strings.ReplaceAll(YamlString, TopologyLogLevel, logLevel)
	YamlString = strings.ReplaceAll(YamlString, TopologyImage, topologyImage)
	return YamlString, nil
}

// OtelCollector - delete or update otel collector objects
func OtelCollector(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	YamlString, err := getOtelCollector(op, cr)
	if err != nil {
		return err
	}

	otelObjects, err := utils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range otelObjects {
		if isDeleting {
			if err := utils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	return nil
}

// getOtelCollector - get otel collector yaml string
func getOtelCollector(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
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

	nginxProxyImage := "nginxinc/nginx-unprivileged:1.20"
	otelCollectorImage := "otel/opentelemetry-collector:0.42.0"

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

	YamlString = strings.ReplaceAll(YamlString, OtelCollectorImage, otelCollectorImage)
	YamlString = strings.ReplaceAll(YamlString, NginxProxyImage, nginxProxyImage)
	return YamlString, nil
}

// PowerScaleMetrics - delete or update powerscale metrics objects
func PowerScaleMetrics(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client, k8sClient kubernetes.Interface) error {
	log := logger.GetLogger(ctx)

	ObjectsYamlString, err := getPowerScaleMetricsObjects(op, cr)
	if err != nil {
		return err
	}

	powerscaleMetricsObjects, err := utils.GetModuleComponentObj([]byte(ObjectsYamlString))
	if err != nil {
		return err
	}

	//update secret volume and inject authorization to deployment
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
			if err := utils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
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
				return fmt.Errorf("error delete deployment: %v", err)
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

// getPowerScaleMetricsObjects - get powerscale metrics yaml string
func getPowerScaleMetricsObjects(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	obs, err := getObservabilityModule(cr)
	if err != nil {
		return YamlString, err
	}

	buf, err := readConfigFile(obs, cr, op, PscaleObsYamlFile)
	if err != nil {
		return YamlString, err
	}
	YamlString = string(buf)

	logLevel := "INFO"
	otelCollectorAddress := "otel-collector:55680"
	pscaleImage := "dellemc/dellemc/csm-metrics-powerscale:v1.0.0"
	maxConcurrentQueries := "10"
	capacityEnabled := "true"
	performanceEnabled := "true"
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

	YamlString = strings.ReplaceAll(YamlString, PowerScaleImage, pscaleImage)
	YamlString = strings.ReplaceAll(YamlString, PowerscaleLogLevel, logLevel)
	YamlString = strings.ReplaceAll(YamlString, PowerScaleMaxConcurrentQueries, maxConcurrentQueries)
	YamlString = strings.ReplaceAll(YamlString, PowerscaleCapacityMetricsEnabled, capacityEnabled)
	YamlString = strings.ReplaceAll(YamlString, PowerscalePerformanceMetricsEnabled, performanceEnabled)
	YamlString = strings.ReplaceAll(YamlString, PowerscaleClusterCapacityPollFrequency, clusterCapacityPollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PowerscaleClusterPerformancePollFrequency, clusterPerformancePollFrequency)
	YamlString = strings.ReplaceAll(YamlString, PowerscaleQuotaCapacityPollFrequency, quotaCapacityPollFrequency)
	YamlString = strings.ReplaceAll(YamlString, IsiclientInsecure, clientInsecure)
	YamlString = strings.ReplaceAll(YamlString, IsiclientAuthType, clientAuthType)
	YamlString = strings.ReplaceAll(YamlString, IsiclientVerbose, clientVerbose)
	YamlString = strings.ReplaceAll(YamlString, PowerscaleLogFormat, logFormat)
	YamlString = strings.ReplaceAll(YamlString, OtelCollectorAddress, otelCollectorAddress)
	YamlString = strings.ReplaceAll(YamlString, DriverDefaultReleaseName, cr.Name)

	return YamlString, nil
}

// parseObservabilityMetricsDeployment - update secret volume and inject authorization to deployment
func parseObservabilityMetricsDeployment(ctx context.Context, deployment *appsv1.Deployment, op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (*confv1.DeploymentApplyConfiguration, error) {
	//parse deployment to DeploymentApplyConfiguration
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

	//inject authorization to deployment
	if authorizationEnabled, _ := utils.IsModuleEnabled(ctx, cr, csmv1.Authorization); authorizationEnabled {
		dpApply, err = AuthInjectDeployment(*dpApply, cr, op)
		if err != nil {
			return nil, fmt.Errorf("injecting auth into Observability metrics deployment: %v", err)
		}
	}

	return dpApply, nil
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
