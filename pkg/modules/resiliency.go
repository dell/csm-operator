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

func getResiliencyModule(cr csmv1.ContainerStorageModule) (csmv1.Module, error) {
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Resiliency {
			return m, nil

		}
	}
	return csmv1.Module{}, fmt.Errorf("could not find resiliency module")
}

func getResiliencyApplyCR(cr csmv1.ContainerStorageModule, op utils.OperatorConfig, driverType string) (*csmv1.Module, *acorev1.ContainerApplyConfiguration, error) {
	// var err error
	resiliencyModule := csmv1.Module{}
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Resiliency {
			resiliencyModule = m
			break
		}
	}
	fileToRead := "container-" + driverType + ".yaml"
	buf, err := readConfigFile(resiliencyModule, cr, op, fileToRead)
	if err != nil {
		return nil, nil, err
	}
	YamlString := utils.ModifyCommonCR(string(buf), cr)
	var container acorev1.ContainerApplyConfiguration
	err = yaml.Unmarshal([]byte(YamlString), &container)
	if err != nil {
		return nil, nil, err
	}
	return &resiliencyModule, &container, nil
}

// ResiliencyInjectDeployment - inject resiliency into deployment
func ResiliencyInjectDeployment(dp applyv1.DeploymentApplyConfiguration, cr csmv1.ContainerStorageModule, op utils.OperatorConfig, driverType string) (*applyv1.DeploymentApplyConfiguration, error) {
	resiliencyModule, containerPtr, err := getResiliencyApplyCR(cr, op, driverType)
	if err != nil {
		return nil, err
	}
	container := *containerPtr
	fmt.Printf("container specs are %+v", container)
	// Get the controller arguments
	var controllerArgs []string
	for _, arg := range containerPtr.Args {
		if strings.HasPrefix(arg, "--mode=controller") {
			controllerArgs = strings.Split(arg, " ")
			break
		}
	}
	containerPtr.Args = controllerArgs

	dp.Spec.Template.Spec.Containers = append(dp.Spec.Template.Spec.Containers, container)

	fmt.Printf("Resiliency module object... %+v \n", resiliencyModule)
	return &dp, nil
}

// ResiliencyInjectDaemonset  - inject resiliency into daemonset
func ResiliencyInjectDaemonset(ds applyv1.DaemonSetApplyConfiguration, cr csmv1.ContainerStorageModule, op utils.OperatorConfig, driverType string) (*applyv1.DaemonSetApplyConfiguration, error) {
	resiliencyModule, containerPtr, err := getResiliencyApplyCR(cr, op, driverType)
	if err != nil {
		return nil, err
	}

	container := *containerPtr
	fmt.Printf("daemon container specs are %+v", container)
	utils.UpdateSideCarApply(resiliencyModule.Components, &container)
	// Get the controller arguments
	var nodeArgs []string
	for _, arg := range containerPtr.Args {
		if strings.HasPrefix(arg, "--mode=node") {
			nodeArgs = strings.Split(arg, " ")
			break
		}
	}

	containerPtr.Args = nodeArgs

	ds.Spec.Template.Spec.Containers = append(ds.Spec.Template.Spec.Containers, container)

	return &ds, nil
}
