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
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	drivers "github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/logger"
	utils "github.com/dell/csm-operator/pkg/utils"
	rbacv1 "k8s.io/api/rbac/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/yaml"
)

const (
	// DefaultPodmonArrayConnectivityPollRate -
	DefaultPodmonArrayConnectivityPollRate = "<PodmonArrayConnectivityPollRate>"
	// DefaultPodmonAPIPort -
	DefaultPodmonAPIPort = "<PodmonAPIPort>"
)

var (
	// XCSIPodmonArrayConnectivityPollRate -
	XCSIPodmonArrayConnectivityPollRate = "X_CSI_PODMON_ARRAY_CONNECTIVITY_POLL_RATE"
	// XCSIPodmonAPIPort -
	XCSIPodmonAPIPort = "X_CSI_PODMON_API_PORT"
	// XCSIPodmonEnabled -
	XCSIPodmonEnabled = "X_CSI_PODMON_ENABLED"
)

// ResiliencySupportedDrivers is a map containing the CSI Drivers supported by CMS Resiliency. The key is driver name and the value is the driver plugin identifier
var ResiliencySupportedDrivers = map[string]SupportedDriverParam{
	"powerstore": {
		PluginIdentifier:              drivers.PowerStorePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerStoreConfigParamsVolumeMount,
	},
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
}

// ResiliencyPrecheck - Resiliency
func ResiliencyPrecheck(ctx context.Context, op utils.OperatorConfig, resiliency csmv1.Module, cr csmv1.ContainerStorageModule, r utils.ReconcileCSM) error {
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
func ResiliencyInjectClusterRole(clusterRole rbacv1.ClusterRole, cr csmv1.ContainerStorageModule, op utils.OperatorConfig, mode string) (*rbacv1.ClusterRole, error) {
	var err error
	roleFileName := mode + "-roles.yaml"
	resiliencyModule, err := getResiliencyModule(cr)
	if err != nil {
		return nil, err
	}

	buf, err := readConfigFile(resiliencyModule, cr, op, roleFileName)
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

func getResiliencyModule(cr csmv1.ContainerStorageModule) (csmv1.Module, error) {
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Resiliency {
			return m, nil

		}
	}
	return csmv1.Module{}, fmt.Errorf("could not find resiliency module")
}

func getResiliencyEnv(resiliencyModule csmv1.Module, driverType csmv1.DriverType) (string, string) {
	podmonArrayConnectivityPollRate := DefaultPodmonArrayConnectivityPollRate
	podmonAPIPort := DefaultPodmonAPIPort

	for _, component := range resiliencyModule.Components {
		if component.Name == utils.ResiliecnySideCarName {
			for _, env := range component.Envs {
				if env.Name == XCSIPodmonArrayConnectivityPollRate {
					podmonArrayConnectivityPollRate = env.Value
				}
				if env.Name == XCSIPodmonAPIPort {
					podmonAPIPort = env.Value
				}
			}
		}
	}

	return podmonArrayConnectivityPollRate, podmonAPIPort
}

func getResiliencyApplyCR(cr csmv1.ContainerStorageModule, op utils.OperatorConfig, driverType, mode string) (*csmv1.Module, *acorev1.ContainerApplyConfiguration, error) {
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
	buf, err := readConfigFile(resiliencyModule, cr, op, fileToRead)
	if err != nil {
		return nil, nil, err
	}

	YamlString := utils.ModifyCommonCR(string(buf), cr)

	podmonArrayConnectivityPollRate, podmonAPIPort := getResiliencyEnv(resiliencyModule, cr.Spec.Driver.CSIDriverType)
	YamlString = strings.ReplaceAll(YamlString, DefaultPodmonArrayConnectivityPollRate, podmonArrayConnectivityPollRate)
	YamlString = strings.ReplaceAll(YamlString, DefaultPodmonAPIPort, podmonAPIPort)

	var container acorev1.ContainerApplyConfiguration
	err = yaml.Unmarshal([]byte(YamlString), &container)
	if err != nil {
		return nil, nil, err
	}
	return &resiliencyModule, &container, nil
}

// ResiliencyInjectDeployment - inject resiliency into deployment
func ResiliencyInjectDeployment(dp applyv1.DeploymentApplyConfiguration, cr csmv1.ContainerStorageModule, op utils.OperatorConfig, driverType string) (*applyv1.DeploymentApplyConfiguration, error) {
	resiliencyModule, containerPtr, err := getResiliencyApplyCR(cr, op, driverType, "controller")
	if err != nil {
		return nil, err
	}
	container := *containerPtr
	fmt.Printf("container specs are %+v", container)

	dp.Spec.Template.Spec.Containers = append(dp.Spec.Template.Spec.Containers, container)

	podmonArrayConnectivityPollRate, podmonAPIPort := getResiliencyEnv(*resiliencyModule, cr.Spec.Driver.CSIDriverType)
	enabled := "true"
	for i, cnt := range dp.Spec.Template.Spec.Containers {
		if *cnt.Name == "driver" {
			dp.Spec.Template.Spec.Containers[i].Env = append(dp.Spec.Template.Spec.Containers[i].Env,
				acorev1.EnvVarApplyConfiguration{Name: &XCSIPodmonArrayConnectivityPollRate, Value: &podmonArrayConnectivityPollRate},
				acorev1.EnvVarApplyConfiguration{Name: &XCSIPodmonAPIPort, Value: &podmonAPIPort},
				acorev1.EnvVarApplyConfiguration{Name: &XCSIPodmonEnabled, Value: &enabled},
			)
			break
		}
	}

	return &dp, nil
}

// ResiliencyInjectDaemonset  - inject resiliency into daemonset
func ResiliencyInjectDaemonset(ds applyv1.DaemonSetApplyConfiguration, cr csmv1.ContainerStorageModule, op utils.OperatorConfig, driverType string) (*applyv1.DaemonSetApplyConfiguration, error) {
	resiliencyModule, containerPtr, err := getResiliencyApplyCR(cr, op, driverType, "node")
	if err != nil {
		return nil, err
	}

	container := *containerPtr
	fmt.Printf("daemon container specs are %+v", container)
	utils.UpdateSideCarApply(resiliencyModule.Components, &container)
	// Get the controller arguments

	ds.Spec.Template.Spec.Containers = append(ds.Spec.Template.Spec.Containers, container)

	podmonArrayConnectivityPollRate, podmonAPIPort := getResiliencyEnv(*resiliencyModule, cr.Spec.Driver.CSIDriverType)
	enabled := "true"

	for i, cnt := range ds.Spec.Template.Spec.Containers {
		if *cnt.Name == "driver" {
			ds.Spec.Template.Spec.Containers[i].Env = append(ds.Spec.Template.Spec.Containers[i].Env,
				acorev1.EnvVarApplyConfiguration{Name: &XCSIPodmonArrayConnectivityPollRate, Value: &podmonArrayConnectivityPollRate},
				acorev1.EnvVarApplyConfiguration{Name: &XCSIPodmonAPIPort, Value: &podmonAPIPort},
				acorev1.EnvVarApplyConfiguration{Name: &XCSIPodmonEnabled, Value: &enabled},
			)
			break
		}
	}

	return &ds, nil
}
