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

package modules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	v1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"

	"sigs.k8s.io/yaml"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Constants to be used in reverse proxy config files
const (
	ReverseProxyServerComponent = "csipowermax-reverseproxy" // #nosec G101
	ReverseProxyDeployment      = "controller.yaml"
	ReverseProxySidecar         = "container.yaml"
	ReverseProxyService         = "service.yaml"
	ReverseProxyImage           = "<REVERSEPROXY_PROXY_SERVER_IMAGE>"
	ReverseProxyTLSSecret       = "<X_CSI_REVPROXY_TLS_SECRET>" // #nosec G101
	ReverseProxyConfigMap       = "<X_CSI_CONFIG_MAP_NAME>"
	ReverseProxyPort            = "<X_CSI_REVPROXY_PORT>"
)

// var used in deploying reverseproxy
var (
	deployAsSidecar            bool
	CSIPmaxRevProxyServiceName = "X_CSI_POWERMAX_PROXY_SERVICE_NAME"
	CSIPmaxRevProxyPort        = "X_CSI_POWERMAX_SIDECAR_PROXY_PORT"
	RevProxyDefaultPort        = "2222"
	RevProxyServiceName        = "csipowermax-reverseproxy"
	RevProxyConfigMapVolName   = "configmap-volume"
	RevProxyTLSSecretVolName   = "tls-secret"
)

// ReverseproxySupportedDrivers is a map containing the CSI Drivers supported by CSM Reverseproxy. The key is driver name and the value is the driver plugin identifier
var ReverseproxySupportedDrivers = map[string]SupportedDriverParam{
	string(csmv1.PowerMax): {
		PluginIdentifier:              drivers.PowerMaxPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerMaxConfigParamsVolumeMount,
	},
}

// ReverseProxyPrecheck  - runs precheck for CSM ReverseProxy
func ReverseProxyPrecheck(ctx context.Context, op utils.OperatorConfig, revproxy csmv1.Module, cr csmv1.ContainerStorageModule, r utils.ReconcileCSM) error {
	log := logger.GetLogger(ctx)

	if _, ok := ReverseproxySupportedDrivers[string(cr.Spec.Driver.CSIDriverType)]; !ok {
		return fmt.Errorf("CSM Reverseproxy does not support %s driver", string(cr.Spec.Driver.CSIDriverType))
	}

	// check if provided version is supported
	if revproxy.ConfigVersion != "" {
		err := checkVersion(string(csmv1.ReverseProxy), revproxy.ConfigVersion, op.ConfigDirectory)
		if err != nil {
			return err
		}
	}
	// Check for secrets
	proxyServerSecret := "csirevproxy-tls-secret" // #nosec G101
	proxyConfigMap := "powermax-reverseproxy-config"
	if len(revproxy.Components) < 1 {
		return fmt.Errorf("revproxy components can not be nil")
	}

	// we will populate default values for environments, if nothing is given (minimal yaml)
	if revproxy.Components[0].Envs == nil {
		revproxy.Components[0].Envs = append(revproxy.Components[0].Envs, corev1.EnvVar{
			Name:  "X_CSI_REVPROXY_TLS_SECRET",
			Value: "csirevproxy-tls-secret",
		})
		revproxy.Components[0].Envs = append(revproxy.Components[0].Envs, corev1.EnvVar{
			Name:  "X_CSI_REVPROXY_PORT",
			Value: "2222",
		})
		revproxy.Components[0].Envs = append(revproxy.Components[0].Envs, corev1.EnvVar{
			Name:  "X_CSI_CONFIG_MAP_NAME",
			Value: "powermax-reverseproxy-config",
		})
	}

	for _, env := range revproxy.Components[0].Envs {
		if env.Name == "X_CSI_REVPROXY_TLS_SECRET" {
			proxyServerSecret = env.Value
		}
		if env.Name == "X_CSI_CONFIG_MAP_NAME" {
			proxyConfigMap = env.Value
		}
		if env.Name == "DeployAsSidecar" {
			deployAsSidecar, _ = strconv.ParseBool(env.Value)
		}
	}

	err := r.GetClient().Get(ctx, types.NamespacedName{Name: proxyServerSecret, Namespace: cr.GetNamespace()}, &corev1.Secret{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to find secret %s", proxyServerSecret)
		}
	}

	err = r.GetClient().Get(ctx, types.NamespacedName{Name: proxyConfigMap, Namespace: cr.GetNamespace()}, &corev1.ConfigMap{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to find configmap %s", proxyConfigMap)
		}
	}
	log.Infof("\nperformed pre checks for: %s", revproxy.Name)
	return nil
}

// ReverseProxyServer - apply/delete deployment objects
func ReverseProxyServer(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	log := logger.GetLogger(ctx)
	log.Infof("Checking if DeployAsSidar is false...\n")
	if deployAsSidecar {
		log.Infof("DeployAsSidar is true...csi-reverseproxy will be installed as sidecar\n")
		log.Infof("exiting ReverseProxyServer...\n")
		return nil
	}
	YamlString, err := getReverseProxyDeployment(op, cr, csmv1.Module{})
	if err != nil {
		return err
	}
	deployObjects, err := utils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range deployObjects {
		log.Infof("Object: %v -----\n", ctrlObj)
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
	log.Info("Create/Update reverseproxy successful...")
	return nil
}

// ReverseProxyStartService starts reverseproxy service for node to connect to revserseproxy sidecar
func ReverseProxyStartService(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	log := logger.GetLogger(ctx)
	if !deployAsSidecar {
		log.Infof("DeployAsSidar is false...csi-reverseproxy service is part of deployement already\n")
		return nil
	}
	YamlString, err := getReverseProxyService(op, cr)
	if err != nil {
		return err
	}
	deployObjects, err := utils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range deployObjects {
		log.Infof("Object: %v -----\n", ctrlObj)
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
	log.Info("Create/Update reverseproxy serivce successful...")
	return nil
}

func getReverseProxyModule(cr csmv1.ContainerStorageModule) (csmv1.Module, error) {
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.ReverseProxy {
			return m, nil
		}
	}
	return csmv1.Module{}, fmt.Errorf("reverseproxy module not found")
}

// getReverseProxyService - gets the reverseproxy service manifest
func getReverseProxyService(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	yamlString := ""
	revProxy, err := getReverseProxyModule(cr)
	if err != nil {
		return yamlString, err
	}

	buf, err := readConfigFile(revProxy, cr, op, ReverseProxyService)
	if err != nil {
		return yamlString, err
	}

	yamlString = string(buf)
	var proxyPort string

	for _, component := range revProxy.Components {
		if component.Name == ReverseProxyServerComponent {
			for _, env := range component.Envs {
				if env.Name == "X_CSI_REVPROXY_PORT" {
					proxyPort = env.Value
				}
			}
		}
	}
	yamlString = strings.ReplaceAll(yamlString, utils.DefaultReleaseName, cr.Name)
	yamlString = strings.ReplaceAll(yamlString, ReverseProxyPort, proxyPort)
	yamlString = strings.ReplaceAll(yamlString, utils.DefaultReleaseNamespace, cr.Namespace)
	return yamlString, nil
}

// getReverseProxyDeployment - updates deployment manifest with reverseproxy CRD values
func getReverseProxyDeployment(op utils.OperatorConfig, cr csmv1.ContainerStorageModule, revProxy csmv1.Module) (string, error) {
	YamlString := ""
	revProxy, err := getReverseProxyModule(cr)
	if err != nil {
		return YamlString, err
	}

	deploymentPath := fmt.Sprintf("%s/moduleconfig/%s/%s/%s", op.ConfigDirectory, csmv1.ReverseProxy, revProxy.ConfigVersion, ReverseProxyDeployment)
	buf, err := os.ReadFile(filepath.Clean(deploymentPath))
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	proxyNamespace := cr.Namespace
	var proxyTLSSecret, proxyPort, proxyConfig string

	for _, component := range revProxy.Components {
		if component.Name == ReverseProxyServerComponent {
			YamlString = strings.ReplaceAll(YamlString, ReverseProxyImage, string(component.Image))
			for _, env := range component.Envs {
				if env.Name == "X_CSI_REVPROXY_TLS_SECRET" {
					proxyTLSSecret = env.Value
				}
				if env.Name == "X_CSI_REVPROXY_PORT" {
					proxyPort = env.Value
				}
				if env.Name == "X_CSI_CONFIG_MAP_NAME" {
					proxyConfig = env.Value
				}
			}
		}
	}

	YamlString = strings.ReplaceAll(YamlString, utils.DefaultReleaseNamespace, proxyNamespace)
	YamlString = strings.ReplaceAll(YamlString, ReverseProxyPort, proxyPort)
	YamlString = strings.ReplaceAll(YamlString, ReverseProxyTLSSecret, proxyTLSSecret)
	YamlString = strings.ReplaceAll(YamlString, ReverseProxyConfigMap, proxyConfig)
	return YamlString, nil
}

// ReverseProxyInjectDeployment injects reverseproxy container as sidecar into controller
func ReverseProxyInjectDeployment(dp v1.DeploymentApplyConfiguration, cr csmv1.ContainerStorageModule, op utils.OperatorConfig) (*v1.DeploymentApplyConfiguration, error) {
	log := logger.GetLogger(context.Background())

	if !deployAsSidecar {
		log.Infof("DeployAsSidar is false...csi-reverseproxy should be present as deployement\n")
		log.Infof("adding proxy service name...\n")
		for i, cnt := range dp.Spec.Template.Spec.Containers {
			if *cnt.Name == "driver" {
				dp.Spec.Template.Spec.Containers[i].Env = append(dp.Spec.Template.Spec.Containers[i].Env,
					acorev1.EnvVarApplyConfiguration{Name: &CSIPmaxRevProxyServiceName, Value: &RevProxyServiceName},
				)
				break
			}
		}
		return &dp, nil
	}

	revProxyModule, containerPtr, err := getRevproxyApplyCR(cr, op)
	if err != nil {
		return nil, err
	}
	container := *containerPtr
	// update the image
	for _, side := range revProxyModule.Components {
		if side.Name == ReverseProxyServerComponent {
			if side.Image != "" {
				*container.Image = string(side.Image)
			}
		}
	}
	dp.Spec.Template.Spec.Containers = append(dp.Spec.Template.Spec.Containers, container)
	// inject revProxy ENVs in driver environment
	revProxyPort := getRevProxyPort(*revProxyModule)
	for i, cnt := range dp.Spec.Template.Spec.Containers {
		if *cnt.Name == "driver" {
			dp.Spec.Template.Spec.Containers[i].Env = append(dp.Spec.Template.Spec.Containers[i].Env,
				acorev1.EnvVarApplyConfiguration{Name: &CSIPmaxRevProxyPort, Value: &revProxyPort},
			)
			break
		}
	}

	// inject revProxy volumes in driver volumes
	revProxyVolume := getRevProxyVolumeComp(*revProxyModule)
	dp.Spec.Template.Spec.Volumes = append(dp.Spec.Template.Spec.Volumes, revProxyVolume...)

	return &dp, nil
}

func getRevProxyPort(revProxyModule csmv1.Module) string {
	revProxyPort := RevProxyDefaultPort
	for _, component := range revProxyModule.Components {
		if component.Name == ReverseProxyServerComponent {
			for _, env := range component.Envs {
				if env.Name == "X_CSI_REVPROXY_PORT" {
					revProxyPort = env.Value
				}
			}
		}
	}
	return revProxyPort
}

func getRevProxyVolumeComp(revProxyModule csmv1.Module) []acorev1.VolumeApplyConfiguration {
	var revProxyConfigMap, revProxyTLSSecret string
	for _, component := range revProxyModule.Components {
		if component.Name == ReverseProxyServerComponent {
			for _, env := range component.Envs {
				if env.Name == "X_CSI_CONFIG_MAP_NAME" {
					revProxyConfigMap = env.Value
				}
				if env.Name == "X_CSI_REVPROXY_TLS_SECRET" {
					revProxyTLSSecret = env.Value
				}
			}
		}
	}
	optional := true
	revProxyVolumes := []acorev1.VolumeApplyConfiguration{
		{
			Name: &RevProxyConfigMapVolName,
			VolumeSourceApplyConfiguration: acorev1.VolumeSourceApplyConfiguration{
				ConfigMap: &acorev1.ConfigMapVolumeSourceApplyConfiguration{
					LocalObjectReferenceApplyConfiguration: acorev1.LocalObjectReferenceApplyConfiguration{Name: &revProxyConfigMap},
					Optional:                               &optional,
				},
			},
		},
		{
			Name: &RevProxyTLSSecretVolName,
			VolumeSourceApplyConfiguration: acorev1.VolumeSourceApplyConfiguration{
				Secret: &acorev1.SecretVolumeSourceApplyConfiguration{
					SecretName: &revProxyTLSSecret,
				},
			},
		},
	}

	return revProxyVolumes
}

// returns revproxy module and container
func getRevproxyApplyCR(cr csmv1.ContainerStorageModule, op utils.OperatorConfig) (*csmv1.Module, *acorev1.ContainerApplyConfiguration, error) {
	var err error
	revProxyModule, err := getReverseProxyModule(cr)
	if err != nil {
		return nil, nil, err
	}

	buf, err := readConfigFile(revProxyModule, cr, op, ReverseProxySidecar)
	if err != nil {
		return nil, nil, err
	}

	YamlString := utils.ModifyCommonCR(string(buf), cr)
	var container acorev1.ContainerApplyConfiguration
	err = yaml.Unmarshal([]byte(YamlString), &container)
	if err != nil {
		return nil, nil, err
	}
	return &revProxyModule, &container, nil
}
