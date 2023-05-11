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

var (
	// XCSIPodmonArrayConnectivityPollRate -
	XCSIPodmonArrayConnectivityPollRate = "X_CSI_PODMON_ARRAY_CONNECTIVITY_POLL_RATE"
	// XCSIPodmonAPIPort -
	XCSIPodmonAPIPort = "X_CSI_PODMON_API_PORT"
	// XCSIPodmonEnabled -
	XCSIPodmonEnabled = "X_CSI_PODMON_ENABLED"
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
}

// ResiliencyPrecheck - Resiliency module precheck for supported versions
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
	// roleFiles are under moduleConfig for node & controller mode
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

func getResiliencyEnv(resiliencyModule csmv1.Module, driverType csmv1.DriverType) string {
	for _, component := range resiliencyModule.Components {
		if component.Name == utils.PodmonNodeComponent {
			for _, env := range component.Envs {
				if env.Name == XCSIPodmonAPIPort {
					return env.Value
				}
			}
		}
	}
	return ""
}

func getResiliencyArgs(m csmv1.Module, mode string) []string {
	for _, component := range m.Components {
		if component.Name == utils.PodmonControllerComponent && mode == "controller" {
			return component.Args
		}
		if component.Name == utils.PodmonNodeComponent && mode == "node" {
			return component.Args
		}
	}
	return nil
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

	// read args from the respective components
	args := getResiliencyArgs(resiliencyModule, mode)

	var container acorev1.ContainerApplyConfiguration
	err = yaml.Unmarshal([]byte(YamlString), &container)
	if err != nil {
		return nil, nil, err
	}
	// set arguments
	container.Args = args
	return &resiliencyModule, &container, nil
}

// ResiliencyInjectDeployment - inject resiliency into deployment
func ResiliencyInjectDeployment(dp applyv1.DeploymentApplyConfiguration, cr csmv1.ContainerStorageModule, op utils.OperatorConfig, driverType string) (*applyv1.DeploymentApplyConfiguration, error) {
	resiliencyModule, containerPtr, err := getResiliencyApplyCR(cr, op, driverType, "controller")
	if err != nil {
		return nil, err
	}
	container := *containerPtr

	dp.Spec.Template.Spec.Containers = append(dp.Spec.Template.Spec.Containers, container)

	if driverType == string(csmv1.PowerScale) {
		driverType = string(csmv1.PowerScaleName)
	}
	// we need to set these ENV for PowerStore & PowerScale only
	if driverType == string(csmv1.PowerScaleName) || driverType == string(csmv1.PowerStore) {
		for i, cnt := range dp.Spec.Template.Spec.Containers {
			if *cnt.Name == "driver" {
				podmonAPIPort := getResiliencyEnv(*resiliencyModule, cr.Spec.Driver.CSIDriverType)
				podmonArrayConnectivityPollRate := getPollRateFromArgs(container.Args)
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
				break
			}
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
	utils.UpdateSideCarApply(resiliencyModule.Components, &container)
	// Get the controller arguments

	ds.Spec.Template.Spec.Containers = append(ds.Spec.Template.Spec.Containers, container)

	podmonAPIPort := getResiliencyEnv(*resiliencyModule, cr.Spec.Driver.CSIDriverType)
	enabled := "true"
	podmonArrayConnectivityPollRate := getPollRateFromArgs(container.Args)
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

// CheckApplyContainersResiliency - check container configuration for resiliency
func CheckApplyContainersResiliency(containers []acorev1.ContainerApplyConfiguration, cr csmv1.ContainerStorageModule) error {
	resiliencyModule, err := getResiliencyModule(cr)
	if err != nil {
		return err
	}

	driverString := "driver"

	// fetch podmonAPIPort
	podmonAPIPort := getResiliencyEnv(resiliencyModule, cr.Spec.Driver.CSIDriverType)

	// fetch podmonArrayConnectivityPollRate
	args := getResiliencyArgs(resiliencyModule, "node")
	podmonArrayConnectivityPollRate := getPollRateFromArgs(args)

	for _, cnt := range containers {
		if *cnt.Name == utils.ResiliecnySideCarName {

			// check argument in resiliency sidecar(podmon)
			foundPodmonArrayConnectivityPollRate := false
			for _, arg := range cnt.Args {
				if fmt.Sprintf("--arrayConnectivityPollRate=%s", podmonArrayConnectivityPollRate) == arg {
					foundPodmonArrayConnectivityPollRate = true
				}
			}
			if !foundPodmonArrayConnectivityPollRate {
				return fmt.Errorf("missing the following argument %s", podmonArrayConnectivityPollRate)
			}

		} else if *cnt.Name == driverString {
			// check envs in driver sidecar
			foundPodmonAPIPort := false
			foundPodmonArrayConnectivityPollRate := false
			for _, env := range cnt.Env {
				if *env.Name == XCSIPodmonAPIPort {
					foundPodmonAPIPort = true
					if *env.Value != podmonAPIPort {
						return fmt.Errorf("expected %s to have a value of: %s but got: %s", XCSIPodmonAPIPort, podmonAPIPort, *env.Value)
					}
				}
				if *env.Name == XCSIPodmonArrayConnectivityPollRate {
					foundPodmonArrayConnectivityPollRate = true
					if *env.Value != podmonArrayConnectivityPollRate {
						return fmt.Errorf("expected %s to have a value of: %s but got: %s", XCSIPodmonArrayConnectivityPollRate, podmonArrayConnectivityPollRate, *env.Value)
					}
				}
			}
			if !foundPodmonAPIPort {
				return fmt.Errorf("missing the following argument %s", podmonAPIPort)
			}
			if !foundPodmonArrayConnectivityPollRate {
				return fmt.Errorf("missing the following argument %s", podmonArrayConnectivityPollRate)
			}
		}
	}
	return nil
}