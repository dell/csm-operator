//  Copyright © 2023-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package modules

import (
	"context"
	"fmt"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	drivers "github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/logger"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/yaml"
)

var (
	// XCSIPodmonArrayConnectivityPollRate -
	XCSIPodmonArrayConnectivityPollRate = "X_CSI_PODMON_ARRAY_CONNECTIVITY_POLL_RATE"
	// XCSIPodmonAPIPort -
	XCSIPodmonAPIPort = "X_CSI_PODMON_API_PORT"
	// XCSIPodmonAPIToken -
	XCSIPodmonAPIToken = "X_CSI_PODMON_API_TOKEN" // #nosec G101
	// XCSIPodmonEnabled -
	XCSIPodmonEnabled = "X_CSI_PODMON_ENABLED"
	// DefaultModuleMetricsPort -
	DefaultModuleMetricsPort = "8444"
	// DefaultMetricsCollectionInterval -
	DefaultMetricsCollectionInterval = "30s"
	// ModuleMetricsEnabledPlaceholder -
	ModuleMetricsEnabledPlaceholder = "<X_CSI_METRICS_ENABLED>"
	// ModuleMetricsPortPlaceholder -
	ModuleMetricsPortPlaceholder = "<X_CSI_METRICS_PORT>"
	// ModuleMetricsCollectionIntervalPlaceholder -
	ModuleMetricsCollectionIntervalPlaceholder = "<X_CSI_METRICS_COLLECTION_INTERVAL>"
	// ModuleMetricsTLSCertFilePlaceholder -
	ModuleMetricsTLSCertFilePlaceholder = "<X_CSI_METRICS_TLS_CERT_FILE>"
	// ModuleMetricsTLSKeyFilePlaceholder -
	ModuleMetricsTLSKeyFilePlaceholder = "<X_CSI_METRICS_TLS_KEY_FILE>"
)

const (
	controllerMode = "controller"
	nodeMode       = "node"
)

// ResiliencySupportedDrivers is a map containing the CSI Drivers supported by CSM Resiliency. The key is driver name and the value is the driver plugin identifier
var ResiliencySupportedDrivers = map[string]SupportedDriverParam{
	string(csmv1.PowerStore): {
		PluginIdentifier:              drivers.PowerStorePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerStoreConfigParamsVolumeMount,
	},
	string(csmv1.PowerScaleName): {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
	},
	string(csmv1.PowerScale): {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
	},
	string(csmv1.PowerFlex): {
		PluginIdentifier:              drivers.PowerFlexPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerFlexConfigParamsVolumeMount,
	},
	string(csmv1.PowerFlexName): {
		PluginIdentifier:              drivers.PowerFlexPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerFlexConfigParamsVolumeMount,
	},
	string(csmv1.PowerMax): {
		PluginIdentifier:              drivers.PowerMaxPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerMaxConfigParamsVolumeMount,
	},
}

// ResiliencyPrecheck - Resiliency module precheck for supported versions
func ResiliencyPrecheck(ctx context.Context, op operatorutils.OperatorConfig, resiliency csmv1.Module, cr csmv1.ContainerStorageModule, _ operatorutils.ReconcileCSM) error {
	log := logger.GetLogger(ctx)

	if _, ok := ResiliencySupportedDrivers[string(cr.Spec.Driver.CSIDriverType)]; !ok {
		log.Errorf("CSM Operator does not suport Resiliency deployment for %s driver", cr.Spec.Driver.CSIDriverType)
		return fmt.Errorf("CSM Operator does not suport Resiliency deployment for %s driver", cr.Spec.Driver.CSIDriverType)
	}

	// check if provided version is supported
	if resiliency.ConfigVersion != "" {
		err := checkVersion(string(csmv1.Resiliency), resiliency.ConfigVersion, op.ConfigDirectory)
		if err != nil {
			log.Errorf("CSM Operator does not suport Resiliency deployment for this version combination %v", err)
			return err
		}
	}

	log.Infof("\nperformed pre checks for: %s", resiliency.Name)
	return nil
}

// ResiliencyInjectClusterRole - inject resiliency into clusterrole
func ResiliencyInjectClusterRole(ctx context.Context, clusterRole rbacv1.ClusterRole, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig, mode string) (*rbacv1.ClusterRole, error) {
	var err error
	roleFileName := mode + "-clusterroles.yaml"
	resiliencyModule, err := getResiliencyModule(cr)
	if err != nil {
		return nil, err
	}
	// roleFiles are under moduleConfig for node & controller mode
	buf, err := readConfigFile(ctx, resiliencyModule, cr, op, roleFileName)
	if err != nil {
		return nil, err
	}

	var rules []rbacv1.PolicyRule
	err = yaml.Unmarshal(buf, &rules)
	if err != nil {
		return nil, err
	}

	clusterRole.Rules = append(clusterRole.Rules, rules...)
	return &clusterRole, nil
}

// ResiliencyInjectRole - inject resiliency into role
func ResiliencyInjectRole(ctx context.Context, role rbacv1.Role, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig, mode string) (*rbacv1.Role, error) {
	// There are no roles for controller in Resliency
	if mode == "controller" {
		return &role, nil
	}

	var err error
	roleFileName := mode + "-roles.yaml"
	resiliencyModule, err := getResiliencyModule(cr)
	if err != nil {
		return nil, err
	}
	resiliencyVersion := resiliencyModule.ConfigVersion
	if resiliencyVersion == "" {
		version, err := operatorutils.GetVersion(ctx, &cr, op)
		if err != nil {
			return nil, err
		}
		resiliencyVersion, err = operatorutils.GetModuleDefaultVersion(version, cr.Spec.Driver.CSIDriverType, resiliencyModule.Name, op.ConfigDirectory)
		if err != nil {
			return nil, err
		}
	}
	isOldResiliencyVersion, err := operatorutils.MinVersionCheck(resiliencyVersion, "v1.12.0")
	if err != nil {
		return nil, err
	}
	if isOldResiliencyVersion {
		return &role, nil
	}
	// roleFiles are under moduleConfig for node & controller mode
	buf, err := readConfigFile(ctx, resiliencyModule, cr, op, roleFileName)
	if err != nil {
		return nil, err
	}

	var rules []rbacv1.PolicyRule
	err = yaml.Unmarshal(buf, &rules)
	if err != nil {
		return nil, err
	}

	role.Rules = append(role.Rules, rules...)
	return &role, nil
}

func getResiliencyModule(cr csmv1.ContainerStorageModule) (csmv1.Module, error) {
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Resiliency {
			return m, nil
		}
	}
	return csmv1.Module{}, fmt.Errorf("could not find resiliency module")
}

func getResiliencyEnv(resiliencyModule csmv1.Module, _ csmv1.DriverType) string {
	for _, component := range resiliencyModule.Components {
		if component.Name == operatorutils.PodmonNodeComponent {
			for _, env := range component.Envs {
				if env.Name == XCSIPodmonAPIPort {
					return env.Value
				}
			}
		}
	}
	return ""
}

func getResiliencyTokenEnv(resiliencyModule csmv1.Module, _ csmv1.DriverType) string {
	for _, component := range resiliencyModule.Components {
		if component.Name == operatorutils.PodmonNodeComponent {
			for _, env := range component.Envs {
				if env.Name == XCSIPodmonAPIToken {
					return env.Value
				}
			}
		}
	}
	return ""
}

// ModifyResiliencyCR performs string substitution on resiliency config templates
// for module metrics configuration
func ModifyResiliencyCR(yamlString string, module csmv1.Module) string {
	// Determine metrics values
	metricsEnabled := "false"
	metricsPort := DefaultModuleMetricsPort
	collectionInterval := DefaultMetricsCollectionInterval
	tlsCertFile := ""
	tlsKeyFile := ""

	if module.Metrics != nil {
		if module.Metrics.Enabled {
			metricsEnabled = "true"
		}
		if module.Metrics.Port != 0 {
			metricsPort = fmt.Sprintf("%d", module.Metrics.Port)
		}
		if module.Metrics.Collection != nil {
			if module.Metrics.Collection.Interval != "" {
				collectionInterval = module.Metrics.Collection.Interval
			}
		}
		if module.Metrics.TLSCertSecret != "" {
			tlsCertFile = "/etc/metrics-tls/tls.crt"
			tlsKeyFile = "/etc/metrics-tls/tls.key"
		}
	}

	// Substitute placeholders
	result := strings.ReplaceAll(yamlString, ModuleMetricsEnabledPlaceholder, metricsEnabled)
	result = strings.ReplaceAll(result, ModuleMetricsPortPlaceholder, metricsPort)
	result = strings.ReplaceAll(result, ModuleMetricsCollectionIntervalPlaceholder, collectionInterval)
	result = strings.ReplaceAll(result, ModuleMetricsTLSCertFilePlaceholder, tlsCertFile)
	result = strings.ReplaceAll(result, ModuleMetricsTLSKeyFilePlaceholder, tlsKeyFile)

	return result
}

// Apply resiliency module from the manifest file to the podmon sidecar
func modifyPodmon(ctx context.Context, component csmv1.ContainerTemplate, container *acorev1.ContainerApplyConfiguration, matched operatorutils.VersionSpec, cr csmv1.ContainerStorageModule, module csmv1.Module) {
	matchedImageApplied := false
	if matched.Version != "" {
		containerName := *container.Name
		if img := matched.Images[containerName]; img != "" {
			*container.Image = img
			matchedImageApplied = true
		}
	}
	if !matchedImageApplied && cr.Spec.CustomRegistry != "" {
		image := operatorutils.ResolveImage(ctx, *container.Image, cr)
		*container.Image = image
	} else if !matchedImageApplied && component.Image != "" {
		image := string(component.Image)
		if container.Image != nil {
			*container.Image = image
		}
		container.Image = &image
	}

	if component.ImagePullPolicy != "" {
		if container.ImagePullPolicy != nil {
			*container.ImagePullPolicy = component.ImagePullPolicy
		}
		container.ImagePullPolicy = &component.ImagePullPolicy
	}
	emptyEnv := make([]corev1.EnvVar, 0)
	container.Env = operatorutils.ReplaceAllApplyCustomEnvs(container.Env, emptyEnv, component.Envs)
	container.Args = operatorutils.ReplaceAllArgs(container.Args, component.Args)

	// Handle metrics port
	if module.Metrics != nil && module.Metrics.Enabled {
		for _, port := range component.Ports {
			if port.Name == "res-metrics" {
				if container.Ports == nil {
					container.Ports = []acorev1.ContainerPortApplyConfiguration{}
				}
				containerPort := port.ContainerPort
				container.Ports = append(container.Ports, acorev1.ContainerPortApplyConfiguration{
					ContainerPort: &containerPort,
					Name:          &port.Name,
					Protocol:      &port.Protocol,
				})
			}
		}
	}

	// Apply custom metrics port override to existing podmon ports as well.
	// This is applied even when module metrics are disabled so hostNetwork
	// podmon sidecars can avoid port collisions during e2e runs.
	if module.Metrics != nil && module.Metrics.Port != 0 {
		customPort := module.Metrics.Port
		for i := range container.Ports {
			if container.Ports[i].Name == nil {
				continue
			}
			if *container.Ports[i].Name == "res-metrics" || *container.Ports[i].Name == "metrics" {
				container.Ports[i].ContainerPort = &customPort
			}
		}
	}

	// Handle metrics TLS volume mount
	if len(component.VolumeMounts) > 0 {
		for _, mount := range component.VolumeMounts {
			if mount.Name == "resiliency-metrics-tls" {
				if container.VolumeMounts == nil {
					container.VolumeMounts = []acorev1.VolumeMountApplyConfiguration{}
				}
				readOnly := true
				container.VolumeMounts = append(container.VolumeMounts, acorev1.VolumeMountApplyConfiguration{
					Name:      &mount.Name,
					MountPath: &mount.MountPath,
					ReadOnly:  &readOnly,
				})
			}
		}
	}

	// Unconditionally mount TLS volume if metrics is enabled with TLS cert secret
	// This ensures the volume mount is added even if not in the component template
	if module.Metrics != nil && module.Metrics.Enabled && module.Metrics.TLSCertSecret != "" {
		metricsTLSVolName := "resiliency-metrics-tls"
		metricsTLSMountPath := "/etc/metrics-tls"
		readOnly := true
		dynamicallyMountVolume(container, acorev1.VolumeMountApplyConfiguration{
			Name:      &metricsTLSVolName,
			MountPath: &metricsTLSMountPath,
			ReadOnly:  &readOnly,
		})
	}
}

func setResiliencyArgs(ctx context.Context, m csmv1.Module, mode string, container *acorev1.ContainerApplyConfiguration, matched operatorutils.VersionSpec, cr csmv1.ContainerStorageModule) {
	// Note: Module metrics placeholders are now substituted via ModifyResiliencyCR()
	// before YAML unmarshaling, so no need to apply metrics env vars here

	// handle minimal manifest (no components listed) for override with configmap
	if len(m.Components) == 0 {
		var synthetic csmv1.ContainerTemplate
		switch mode {
		case controllerMode:
			synthetic = csmv1.ContainerTemplate{
				Name: operatorutils.PodmonControllerComponent,
			}
		case "node":
			synthetic = csmv1.ContainerTemplate{
				Name: operatorutils.PodmonNodeComponent,
			}
		default:
			return
		}
		modifyPodmon(ctx, synthetic, container, matched, cr, m)
	}
	for _, component := range m.Components {
		if component.Name == operatorutils.PodmonControllerComponent && mode == controllerMode {
			modifyPodmon(ctx, component, container, matched, cr, m)
		}
		if component.Name == operatorutils.PodmonNodeComponent && mode == "node" {
			modifyPodmon(ctx, component, container, matched, cr, m)
		}
	}
}

func getPollRateFromArgs(args []string) string {
	for _, arg := range args {
		if strings.Contains(arg, "arrayConnectivityPollRate") {
			sub := strings.Split(arg, "=")
			if len(sub) == 2 {
				return strings.Split(arg, "=")[1]
			}
		}
	}
	return ""
}

func getResiliencyApplyCR(ctx context.Context, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig, driverType, mode string, matched operatorutils.VersionSpec) (*csmv1.Module, *acorev1.ContainerApplyConfiguration, error) {
	resiliencyModule := csmv1.Module{}
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Resiliency {
			resiliencyModule = m
			break
		}
	}
	if driverType == string(csmv1.PowerScale) {
		driverType = string(csmv1.PowerScaleName)
	}
	if driverType == string(csmv1.PowerFlexName) {
		driverType = string(csmv1.PowerFlex)
	}
	fileToRead := "container-" + driverType + "-" + mode + ".yaml"
	buf, err := readConfigFile(ctx, resiliencyModule, cr, op, fileToRead)
	if err != nil {
		return nil, nil, err
	}

	// Substitute module metrics placeholders BEFORE ModifyCommonCR
	YamlString := ModifyResiliencyCR(string(buf), resiliencyModule)

	// Then apply common CR modifications
	YamlString = operatorutils.ModifyCommonCR(YamlString, cr)

	var container acorev1.ContainerApplyConfiguration
	err = yaml.Unmarshal([]byte(YamlString), &container)
	if err != nil {
		return nil, nil, err
	}

	// read args from the respective components
	setResiliencyArgs(ctx, resiliencyModule, mode, &container, matched, cr)
	return &resiliencyModule, &container, nil
}

// ResiliencyInjectDeployment - inject resiliency into deployment
func ResiliencyInjectDeployment(ctx context.Context, dp applyv1.DeploymentApplyConfiguration, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig, driverType string, matched operatorutils.VersionSpec) (*applyv1.DeploymentApplyConfiguration, error) {
	resiliencyModule, podmonPtr, err := getResiliencyApplyCR(ctx, cr, op, driverType, controllerMode, matched)
	if err != nil {
		return nil, err
	}
	podmon := *podmonPtr
	// prepend podmon container in controller-pod
	dp.Spec.Template.Spec.Containers = append([]acorev1.ContainerApplyConfiguration{podmon}, dp.Spec.Template.Spec.Containers...)

	// Add TLS secret volume if metrics is enabled with TLS cert secret
	if resiliencyModule.Metrics != nil && resiliencyModule.Metrics.Enabled && resiliencyModule.Metrics.TLSCertSecret != "" {
		metricsTLSVolName := "resiliency-metrics-tls"
		metricsTLSSecretName := resiliencyModule.Metrics.TLSCertSecret
		dynamicallyAddVolume(
			&dp.Spec.Template.Spec.Volumes,
			acorev1.VolumeApplyConfiguration{
				Name: &metricsTLSVolName,
				VolumeSourceApplyConfiguration: acorev1.VolumeSourceApplyConfiguration{
					Secret: &acorev1.SecretVolumeSourceApplyConfiguration{
						SecretName: &metricsTLSSecretName,
					},
				},
			},
		)
	}

	if driverType == string(csmv1.PowerScale) {
		driverType = string(csmv1.PowerScaleName)
	}
	// we need to set these ENV for PowerStore, PowerMax, PowerScale & PowerFlex only
	if driverType == string(csmv1.PowerScaleName) || driverType == string(csmv1.PowerStore) || driverType == string(csmv1.PowerMax) || driverType == string(csmv1.PowerFlex) {
		for i, cnt := range dp.Spec.Template.Spec.Containers {
			if *cnt.Name == "driver" {
				podmonAPIPort := getResiliencyEnv(*resiliencyModule, cr.Spec.Driver.CSIDriverType)
				podmonArrayConnectivityPollRate := getPollRateFromArgs(podmon.Args)
				enabled := "true"
				dp.Spec.Template.Spec.Containers[i].Env = append(dp.Spec.Template.Spec.Containers[i].Env,
					acorev1.EnvVarApplyConfiguration{Name: &XCSIPodmonEnabled, Value: &enabled},
				)
				if podmonArrayConnectivityPollRate != "" {
					dp.Spec.Template.Spec.Containers[i].Env = append(dp.Spec.Template.Spec.Containers[i].Env,
						acorev1.EnvVarApplyConfiguration{Name: &XCSIPodmonArrayConnectivityPollRate, Value: &podmonArrayConnectivityPollRate},
					)
				}
				if podmonAPIPort != "" {
					dp.Spec.Template.Spec.Containers[i].Env = append(dp.Spec.Template.Spec.Containers[i].Env,
						acorev1.EnvVarApplyConfiguration{Name: &XCSIPodmonAPIPort, Value: &podmonAPIPort},
					)
				}
				podmonAPIToken := getResiliencyTokenEnv(*resiliencyModule, cr.Spec.Driver.CSIDriverType)
				if podmonAPIToken != "" {
					dp.Spec.Template.Spec.Containers[i].Env = append(dp.Spec.Template.Spec.Containers[i].Env,
						acorev1.EnvVarApplyConfiguration{Name: &XCSIPodmonAPIToken, Value: &podmonAPIToken},
					)
				}
				break
			}
		}
	}
	return &dp, nil
}

// ResiliencyInjectDaemonset  - inject resiliency into daemonset
func ResiliencyInjectDaemonset(ctx context.Context, ds applyv1.DaemonSetApplyConfiguration, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig, driverType string, matched operatorutils.VersionSpec) (*applyv1.DaemonSetApplyConfiguration, error) {
	resiliencyModule, podmonPtr, err := getResiliencyApplyCR(ctx, cr, op, driverType, nodeMode, matched)
	if err != nil {
		return nil, err
	}

	podmon := *podmonPtr
	// prepend podmon container in node-pod
	ds.Spec.Template.Spec.Containers = append([]acorev1.ContainerApplyConfiguration{podmon}, ds.Spec.Template.Spec.Containers...)

	// Add TLS secret volume if metrics is enabled with TLS cert secret
	if resiliencyModule.Metrics != nil && resiliencyModule.Metrics.Enabled && resiliencyModule.Metrics.TLSCertSecret != "" {
		metricsTLSVolName := "resiliency-metrics-tls"
		metricsTLSSecretName := resiliencyModule.Metrics.TLSCertSecret
		dynamicallyAddVolume(
			&ds.Spec.Template.Spec.Volumes,
			acorev1.VolumeApplyConfiguration{
				Name: &metricsTLSVolName,
				VolumeSourceApplyConfiguration: acorev1.VolumeSourceApplyConfiguration{
					Secret: &acorev1.SecretVolumeSourceApplyConfiguration{
						SecretName: &metricsTLSSecretName,
					},
				},
			},
		)
	}

	podmonAPIPort := getResiliencyEnv(*resiliencyModule, cr.Spec.Driver.CSIDriverType)
	podmonAPIToken := getResiliencyTokenEnv(*resiliencyModule, cr.Spec.Driver.CSIDriverType)
	enabled := "true"
	podmonArrayConnectivityPollRate := getPollRateFromArgs(podmon.Args)
	for i, cnt := range ds.Spec.Template.Spec.Containers {
		if *cnt.Name == "driver" {
			ds.Spec.Template.Spec.Containers[i].Env = append(ds.Spec.Template.Spec.Containers[i].Env,
				acorev1.EnvVarApplyConfiguration{Name: &XCSIPodmonArrayConnectivityPollRate, Value: &podmonArrayConnectivityPollRate},
				acorev1.EnvVarApplyConfiguration{Name: &XCSIPodmonAPIPort, Value: &podmonAPIPort},
				acorev1.EnvVarApplyConfiguration{Name: &XCSIPodmonEnabled, Value: &enabled},
			)
			if podmonAPIToken != "" {
				ds.Spec.Template.Spec.Containers[i].Env = append(ds.Spec.Template.Spec.Containers[i].Env,
					acorev1.EnvVarApplyConfiguration{Name: &XCSIPodmonAPIToken, Value: &podmonAPIToken},
				)
			}

			break
		}
	}

	return &ds, nil
}

// dynamicallyMountVolume adds a volume mount to a container if it doesn't already exist
func dynamicallyMountVolume(ct *acorev1.ContainerApplyConfiguration, mount acorev1.VolumeMountApplyConfiguration) {
	contains := false
	if ct.VolumeMounts != nil {
		for _, v := range ct.VolumeMounts {
			if v.Name != nil && mount.Name != nil && *v.Name == *mount.Name {
				contains = true
				break
			}
		}
	}

	if !contains {
		if ct.VolumeMounts == nil {
			ct.VolumeMounts = []acorev1.VolumeMountApplyConfiguration{}
		}
		ct.VolumeMounts = append(ct.VolumeMounts, mount)
	}
}
