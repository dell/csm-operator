//  Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package drivers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	metacv1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"sigs.k8s.io/yaml"
)

var defaultVolumeConfigName = map[csmv1.DriverType]string{
	csmv1.PowerScaleName: "isilon-configs",
}

// GetController get controller yaml
func GetController(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, driverName csmv1.DriverType) (*utils.ControllerYAML, error) {
	log := logger.GetLogger(ctx)
	configMapPath := fmt.Sprintf("%s/driverconfig/%s/%s/controller.yaml", operatorConfig.ConfigDirectory, driverName, cr.Spec.Driver.ConfigVersion)
	log.Debugw("GetController", "configMapPath", configMapPath)
	buf, err := os.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		log.Errorw("GetController failed", "Error", err.Error())
		return nil, err
	}

	YamlString := utils.ModifyCommonCR(string(buf), cr)
	if cr.Spec.Driver.CSIDriverType == "powerstore" {
		YamlString = ModifyPowerstoreCR(YamlString, cr, "Controller")
	}
	log.Debugw("DriverSpec ", cr.Spec)
	if cr.Spec.Driver.CSIDriverType == "unity" {
		YamlString = ModifyUnityCR(YamlString, cr, "Controller")
	}
	if cr.Spec.Driver.CSIDriverType == "powerflex" {
		YamlString = ModifyPowerflexCR(YamlString, cr, "Controller")
	}
	if cr.Spec.Driver.CSIDriverType == "powermax" {
		YamlString = ModifyPowermaxCR(YamlString, cr, "Controller")
	}

	driverYAML, err := utils.GetDriverYaml(YamlString, "Deployment")
	if err != nil {
		log.Errorw("GetController get Deployment failed", "Error", err.Error())
		return nil, err
	}

	controllerYAML := driverYAML.(utils.ControllerYAML)
	controllerYAML.Deployment.Spec.Replicas = &cr.Spec.Driver.Replicas

	if len(cr.Spec.Driver.Controller.Tolerations) != 0 {
		tols := make([]acorev1.TolerationApplyConfiguration, 0)
		for _, t := range cr.Spec.Driver.Controller.Tolerations {
			log.Debugw("Adding toleration", "t", t)
			toleration := acorev1.Toleration()
			toleration.WithKey(t.Key)
			toleration.WithOperator(t.Operator)
			toleration.WithValue(t.Value)
			toleration.WithEffect(t.Effect)
			if t.TolerationSeconds != nil {
				toleration.WithTolerationSeconds(*t.TolerationSeconds)
			}
			tols = append(tols, *toleration)
		}

		controllerYAML.Deployment.Spec.Template.Spec.Tolerations = tols
	}

	if cr.Spec.Driver.Controller.NodeSelector != nil {
		controllerYAML.Deployment.Spec.Template.Spec.NodeSelector = cr.Spec.Driver.Controller.NodeSelector
	}

	containers := controllerYAML.Deployment.Spec.Template.Spec.Containers
	newcontainers := make([]acorev1.ContainerApplyConfiguration, 0)
	for i, c := range containers {
		if string(*c.Name) == "driver" {
			containers[i].Env = utils.ReplaceAllApplyCustomEnvs(c.Env, cr.Spec.Driver.Common.Envs, cr.Spec.Driver.Controller.Envs)
			c.Env = containers[i].Env
			if string(cr.Spec.Driver.Common.Image) != "" {
				image := string(cr.Spec.Driver.Common.Image)
				c.Image = &image
			}
		}

		removeContainer := false
		for _, s := range cr.Spec.Driver.SideCars {
			if s.Name == *c.Name {
				if s.Enabled == nil {
					log.Infow("Container to be enabled", "name", *c.Name)
					break
				} else if !*s.Enabled {
					removeContainer = true
					log.Infow("Container to be removed", "name", *c.Name)
				} else {
					log.Infow("Container to be enabled", "name", *c.Name)
				}
				break
			}
		}
		if !removeContainer {
			utils.ReplaceAllContainerImageApply(operatorConfig.K8sVersion, &containers[i])
			utils.UpdateSideCarApply(cr.Spec.Driver.SideCars, &containers[i])
			newcontainers = append(newcontainers, c)
		}

	}

	controllerYAML.Deployment.Spec.Template.Spec.Containers = newcontainers
	// Update volumes
	for i, v := range controllerYAML.Deployment.Spec.Template.Spec.Volumes {
		newV := new(acorev1.VolumeApplyConfiguration)
		if *v.Name == "certs" {
			if cr.Spec.Driver.CSIDriverType == "isilon" || cr.Spec.Driver.CSIDriverType == "powerflex" {
				newV, err = getApplyCertVolume(cr)
			}
			if cr.Spec.Driver.CSIDriverType == "unity" {
				newV, err = getApplyCertVolumeUnity(cr)
			}
			if cr.Spec.Driver.CSIDriverType == "powermax" {
				newV, err = getApplyCertVolumePowermax(cr)
			}
			if err != nil {
				log.Errorw("GetController spec template volumes", "Error", err.Error())
				return nil, err
			}
			controllerYAML.Deployment.Spec.Template.Spec.Volumes[i] = *newV
		}
		if *v.Name == defaultVolumeConfigName[driverName] && cr.Spec.Driver.AuthSecret != "" {
			controllerYAML.Deployment.Spec.Template.Spec.Volumes[i].Secret.SecretName = &cr.Spec.Driver.AuthSecret
		}

	}

	crUID := cr.GetUID()
	bController := true
	bOwnerDeletion := !cr.Spec.Driver.ForceRemoveDriver
	kind := cr.Kind
	v1 := "apps/v1"
	controllerYAML.Deployment.OwnerReferences = []metacv1.OwnerReferenceApplyConfiguration{
		{
			APIVersion:         &v1,
			Controller:         &bController,
			BlockOwnerDeletion: &bOwnerDeletion,
			Kind:               &kind,
			Name:               &cr.Name,
			UID:                &crUID,
		},
	}

	return &controllerYAML, nil
}

// GetNode get node yaml
func GetNode(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, driverType csmv1.DriverType, filename string) (*utils.NodeYAML, error) {
	log := logger.GetLogger(ctx)
	configMapPath := fmt.Sprintf("%s/driverconfig/%s/%s/%s", operatorConfig.ConfigDirectory, driverType, cr.Spec.Driver.ConfigVersion, filename)
	log.Debugw("GetNode", "configMapPath", configMapPath)
	buf, err := os.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		log.Errorw("GetNode failed", "Error", err.Error())
		return nil, err
	}

	YamlString := utils.ModifyCommonCR(string(buf), cr)
	if cr.Spec.Driver.CSIDriverType == "powerstore" {
		YamlString = ModifyPowerstoreCR(YamlString, cr, "Node")
	}
	if cr.Spec.Driver.CSIDriverType == "powerflex" {
		YamlString = ModifyPowerflexCR(YamlString, cr, "Node")
	}
	if cr.Spec.Driver.CSIDriverType == "unity" {
		YamlString = ModifyUnityCR(YamlString, cr, "Node")
	}
	if cr.Spec.Driver.CSIDriverType == "powermax" {
		YamlString = ModifyPowermaxCR(YamlString, cr, "Node")
	}

	driverYAML, err := utils.GetDriverYaml(YamlString, "DaemonSet")
	if err != nil {
		log.Errorw("GetNode Daemonset failed", "Error", err.Error())
		return nil, err
	}

	nodeYaml := driverYAML.(utils.NodeYAML)

	if cr.Spec.Driver.DNSPolicy != "" {
		dnspolicy := corev1.DNSPolicy(cr.Spec.Driver.DNSPolicy)
		nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.DNSPolicy = &dnspolicy
	}

	if len(cr.Spec.Driver.Node.Tolerations) != 0 {
		tols := make([]acorev1.TolerationApplyConfiguration, 0)
		for _, t := range cr.Spec.Driver.Node.Tolerations {
			fmt.Printf("[BRUH] toleration t: %+v\n", t)
			toleration := acorev1.Toleration()
			toleration.WithKey(t.Key)
			toleration.WithOperator(t.Operator)
			toleration.WithValue(t.Value)
			toleration.WithEffect(t.Effect)
			if t.TolerationSeconds != nil {
				toleration.WithTolerationSeconds(*t.TolerationSeconds)
			}
			tols = append(tols, *toleration)
		}

		nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Tolerations = tols
	}

	if cr.Spec.Driver.Node.NodeSelector != nil {
		nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.NodeSelector = cr.Spec.Driver.Node.NodeSelector
	}

	containers := nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Containers
	newcontainers := make([]acorev1.ContainerApplyConfiguration, 0)
	for i, c := range containers {
		if string(*c.Name) == "driver" {
			containers[i].Env = utils.ReplaceAllApplyCustomEnvs(c.Env, cr.Spec.Driver.Common.Envs, cr.Spec.Driver.Node.Envs)
			c.Env = containers[i].Env
			if string(cr.Spec.Driver.Common.Image) != "" {
				image := string(cr.Spec.Driver.Common.Image)
				c.Image = &image
			}
		}
		removeContainer := false
		for _, s := range cr.Spec.Driver.SideCars {
			if s.Name == *c.Name {
				if s.Enabled == nil {
					log.Infow("Container to be enabled", "name", *c.Name)
					break
				} else if !*s.Enabled {
					removeContainer = true
					log.Infow("Container to be removed", "name", *c.Name)
				} else {
					log.Infow("Container to be enabled", "name", *c.Name)
				}
				break
			}
		}
		if !removeContainer {
			utils.ReplaceAllContainerImageApply(operatorConfig.K8sVersion, &containers[i])
			utils.UpdateSideCarApply(cr.Spec.Driver.SideCars, &containers[i])
			newcontainers = append(newcontainers, c)
		}
	}

	nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Containers = newcontainers

	initcontainers := nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.InitContainers
	for i := range initcontainers {
		utils.ReplaceAllContainerImageApply(operatorConfig.K8sVersion, &initcontainers[i])
		utils.UpdateinitContainerApply(cr.Spec.Driver.InitContainers, &initcontainers[i])
	}

	nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.InitContainers = initcontainers

	// Update volumes
	for i, v := range nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Volumes {
		newV := new(acorev1.VolumeApplyConfiguration)
		if *v.Name == "certs" {
			if cr.Spec.Driver.CSIDriverType == "isilon" || cr.Spec.Driver.CSIDriverType == "powerflex" {
				newV, err = getApplyCertVolume(cr)
			}
			if cr.Spec.Driver.CSIDriverType == "unity" {
				newV, err = getApplyCertVolumeUnity(cr)
			}
			if cr.Spec.Driver.CSIDriverType == "powermax" {
				newV, err = getApplyCertVolumePowermax(cr)
			}
			if err != nil {
				log.Errorw("GetNode apply cert Volume failed", "Error", err.Error())
				return nil, err
			}
			nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Volumes[i] = *newV
		}
		if *v.Name == defaultVolumeConfigName[driverType] && cr.Spec.Driver.AuthSecret != "" {
			nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Volumes[i].Secret.SecretName = &cr.Spec.Driver.AuthSecret
		}

	}

	return &nodeYaml, nil
}

// GetUpgradeInfo -
func GetUpgradeInfo(ctx context.Context, operatorConfig utils.OperatorConfig, driverType csmv1.DriverType, oldVersion string) (string, error) {
	log := logger.GetLogger(ctx)
	upgradeInfoPath := fmt.Sprintf("%s/driverconfig/%s/%s/upgrade-path.yaml", operatorConfig.ConfigDirectory, driverType, oldVersion)
	log.Debugw("GetUpgradeInfo", "upgradeInfoPath", upgradeInfoPath)

	buf, err := os.ReadFile(filepath.Clean(upgradeInfoPath))
	if err != nil {
		log.Errorw("GetUpgradeInfo failed", "Error", err.Error())
		return "", err
	}
	YamlString := string(buf)

	var upgradePath utils.UpgradePaths
	err = yaml.Unmarshal([]byte(YamlString), &upgradePath)
	if err != nil {
		log.Errorw("GetUpgradeInfo yaml marshall failed", "Error", err.Error())
		return "", err
	}

	// Example return value: "v2.2.0"
	return upgradePath.MinUpgradePath, nil
}

// GetConfigMap get configmap
func GetConfigMap(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, driverName csmv1.DriverType) (*corev1.ConfigMap, error) {
	log := logger.GetLogger(ctx)
	var podmanLogFormat string
	var podmanLogLevel string
	configMapPath := fmt.Sprintf("%s/driverconfig/%s/%s/driver-config-params.yaml", operatorConfig.ConfigDirectory, driverName, cr.Spec.Driver.ConfigVersion)
	log.Debugw("GetConfigMap", "configMapPath", configMapPath)

	buf, err := os.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		log.Errorw("GetConfigMap failed", "Error", err.Error())
		return nil, err
	}
	YamlString := utils.ModifyCommonCR(string(buf), cr)

	var configMap corev1.ConfigMap
	cmValue := ""
	var configMapData map[string]string
	err = yaml.Unmarshal([]byte(YamlString), &configMap)
	if err != nil {
		log.Errorw("GetConfigMap yaml marshall failed", "Error", err.Error())
		return nil, err
	}

	for _, env := range cr.Spec.Driver.Common.Envs {
		if env.Name == "CSI_LOG_LEVEL" {
			cmValue += fmt.Sprintf("\n%s: %s", env.Name, env.Value)
			podmanLogLevel = env.Value
		}
		if env.Name == "CSI_LOG_FORMAT" {
			cmValue += fmt.Sprintf("\n%s: %s", env.Name, env.Value)
			podmanLogFormat = env.Value
		}
	}

	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Resiliency {
			if m.Enabled {
				cmValue += fmt.Sprintf("\n%s: %s", "PODMON_CONTROLLER_LOG_LEVEL", podmanLogLevel)
				cmValue += fmt.Sprintf("\n%s: %s", "PODMON_CONTROLLER_LOG_FORMAT", podmanLogFormat)
				cmValue += fmt.Sprintf("\n%s: %s", "PODMON_NODE_LOG_LEVEL", podmanLogLevel)
				cmValue += fmt.Sprintf("\n%s: %s", "PODMON_NODE_LOG_FORMAT", podmanLogFormat)
			}
		}
	}

	configMapData = map[string]string{
		"driver-config-params.yaml": cmValue,
	}
	configMap.Data = configMapData

	if cr.Spec.Driver.CSIDriverType == "unity" {
		configMap.Data = ModifyUnityConfigMap(ctx, cr)
	}
	return &configMap, nil
}

// GetCSIDriver get driver
func GetCSIDriver(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, driverName csmv1.DriverType) (*storagev1.CSIDriver, error) {
	log := logger.GetLogger(ctx)
	configMapPath := fmt.Sprintf("%s/driverconfig/%s/%s/csidriver.yaml", operatorConfig.ConfigDirectory, driverName, cr.Spec.Driver.ConfigVersion)
	log.Debugw("GetCSIDriver", "configMapPath", configMapPath)
	buf, err := os.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		log.Errorw("GetCSIDriver failed", "Error", err.Error())
		return nil, err
	}

	var csidriver storagev1.CSIDriver

	YamlString := utils.ModifyCommonCR(string(buf), cr)
	switch cr.Spec.Driver.CSIDriverType {
	case "powerstore":
		YamlString = ModifyPowerstoreCR(YamlString, cr, "CSIDriverSpec")
	case "isilon":
		YamlString = ModifyPowerScaleCR(YamlString, cr, "CSIDriverSpec")
	case "powermax":
		YamlString = ModifyPowermaxCR(YamlString, cr, "CSIDriverSpec")
	case "powerflex":
		YamlString = ModifyPowerflexCR(YamlString, cr, "CSIDriverSpec")
	case "unity":
		YamlString = ModifyUnityCR(YamlString, cr, "CSIDriverSpec")
	}
	err = yaml.Unmarshal([]byte(YamlString), &csidriver)
	if err != nil {
		log.Errorw("GetCSIDriver yaml marshall failed", "Error", err.Error())
		return nil, err
	}
	// overriding default FSGroupPolicy if this was provided in manifest
	if cr.Spec.Driver.CSIDriverSpec.FSGroupPolicy != "" {
		fsGroupPolicy := storagev1.NoneFSGroupPolicy
		if cr.Spec.Driver.CSIDriverSpec.FSGroupPolicy == "ReadWriteOnceWithFSType" {
			fsGroupPolicy = storagev1.ReadWriteOnceWithFSTypeFSGroupPolicy
		} else if cr.Spec.Driver.CSIDriverSpec.FSGroupPolicy == "File" {
			fsGroupPolicy = storagev1.FileFSGroupPolicy
		}
		csidriver.Spec.FSGroupPolicy = &fsGroupPolicy
		log.Debugw("GetCSIDriver", "fsGroupPolicy", fsGroupPolicy)
	}

	return &csidriver, nil
}
