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
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	metacv1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var defaultVolumeConfigName = map[csmv1.DriverType]string{
	csmv1.PowerScaleName: "isilon-configs",
}

const (
	ConfigParamsFile = "driver-config-params.yaml"
	// CSMNameSpace - namespace CSM is found in. Needed for cases where pod namespace is not namespace of CSM
	CSMNameSpace = "<CSM_NAMESPACE>"

	// CsiLogLevel - Defines the log level
	CsiLogLevel = "<CSI_LOG_LEVEL>"

	// CsiLogFormat - Defines the log format
	CsiLogFormat = "<CSI_LOG_FORMAT>"

	// OtelCollectorAddress - Defines the otel collector address
	OtelCollectorAddress = "<OTEL_COLLECTOR_ADDRESS>"

	// CsiLogFormat - Defines the log format for cosi
	CosiLogLevel = "COSI_LOG_LEVEL"

	// CsiLogFormat - Defines the log format for cosi
	CosiLogFormat = "COSI_LOG_FORMAT"
)

// GetController get controller yaml
func GetController(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, driverName csmv1.DriverType, matched operatorutils.VersionSpec) (*operatorutils.ControllerYAML, error) {
	log := logger.GetLogger(ctx)
	driverType := cr.Spec.Driver.CSIDriverType
	version, err := operatorutils.GetVersion(ctx, &cr, operatorConfig)
	if err != nil {
		return nil, err
	}
	controllerPath := fmt.Sprintf("%s/driverconfig/%s/%s/controller.yaml", operatorConfig.ConfigDirectory, driverName, version)
	log.Debugw("GetController", "controllerPath", controllerPath)
	buf, err := os.ReadFile(filepath.Clean(controllerPath))
	if err != nil {
		log.Errorw("GetController failed", "Error", err.Error())
		return nil, err
	}

	YamlString := operatorutils.ModifyCommonCR(string(buf), cr)
	if driverType == "powerstore" {
		YamlString = ModifyPowerstoreCR(YamlString, cr, "Controller")
	}
	log.Debugw("DriverSpec ", cr.Spec)
	switch driverType {
	case csmv1.Unity:
		YamlString = ModifyUnityCR(YamlString, cr, "Controller")
	case csmv1.PowerFlex:
		YamlString = ModifyPowerflexCR(YamlString, cr, "Controller")
	case csmv1.PowerMax:
		YamlString = ModifyPowermaxCR(YamlString, cr, "Controller")
	case csmv1.PowerScale:
		YamlString = ModifyPowerScaleCR(YamlString, cr, "Controller")
	case csmv1.Cosi:
		YamlString, err = ModifyCosiCR(YamlString, cr, "Controller")
		if err != nil {
			return nil, err
		}
	}

	driverYAML, err := operatorutils.GetDriverYaml(YamlString, "Deployment")
	if err != nil {
		log.Errorw("GetController get Deployment failed", "Error", err.Error())
		return nil, err
	}

	controllerYAML := driverYAML.(operatorutils.ControllerYAML)

	// if using a minimal manifest, replicas may not be present.
	if cr.Spec.Driver.Replicas != 0 {
		controllerYAML.Deployment.Spec.Replicas = &cr.Spec.Driver.Replicas
	}

	if cr.Spec.Driver.Controller != nil && len(cr.Spec.Driver.Controller.Tolerations) != 0 {
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

	if cr.Spec.Driver.Controller != nil && cr.Spec.Driver.Controller.NodeSelector != nil {
		controllerYAML.Deployment.Spec.Template.Spec.NodeSelector = cr.Spec.Driver.Controller.NodeSelector
	}

	// Checking if driver image is present in config map
	found := false
	var image string
	for k, v := range matched.Images {
		if k == string(cr.Spec.Driver.CSIDriverType) {
			image = v
			found = true
			break
		}
	}

	containers := controllerYAML.Deployment.Spec.Template.Spec.Containers
	newcontainers := make([]acorev1.ContainerApplyConfiguration, 0)
	for i, c := range containers {
		if c.Name != nil && (string(*c.Name) == "driver" || string(*c.Name) == "objectstorage-provisioner") {
			if found {
				c.Image = &image
			} else if cr.Spec.CustomRegistry != "" {
				resolvedImagePath := operatorutils.ResolveImage(ctx, string(*c.Image), cr)
				c.Image = &resolvedImagePath
			} else {
				// Check if Common is not nil before accessing Envs
				if cr.Spec.Driver.Common != nil {
					if cr.Spec.Driver.Common.Image != "" {
						image := string(cr.Spec.Driver.Common.Image)
						c.Image = &image
					}
				}
			}

			var commonEnvs, controllerEnvs []corev1.EnvVar
			if cr.Spec.Driver.Common != nil {
				commonEnvs = cr.Spec.Driver.Common.Envs
			}
			if cr.Spec.Driver.Controller != nil {
				controllerEnvs = cr.Spec.Driver.Controller.Envs
			}
			containers[i].Env = operatorutils.ReplaceAllApplyCustomEnvs(c.Env, commonEnvs, controllerEnvs)
			c.Env = containers[i].Env
		}

		removeContainer := false
		if string(*c.Name) == "csi-external-health-monitor-controller" || string(*c.Name) == "external-health-monitor" {
			removeContainer = true
		}
		for _, s := range cr.Spec.Driver.SideCars {
			if s.Name == *c.Name {
				if s.Enabled == nil {
					if string(*c.Name) == "csi-external-health-monitor-controller" || string(*c.Name) == "external-health-monitor" {
						removeContainer = true
						log.Infow("Container to be removed", "name", *c.Name)
						break
					}
					removeContainer = false
					log.Infow("Container to be enabled", "name", *c.Name)
					break
				} else if !*s.Enabled {
					removeContainer = true
					log.Infow("Container to be removed", "name", *c.Name)
				} else {
					removeContainer = false
					log.Infow("Container to be enabled", "name", *c.Name)
				}
				break
			}
		}
		if !removeContainer {
			operatorutils.ReplaceAllContainerImageApply(operatorConfig.K8sVersion, &c)
			operatorutils.UpdateSideCarApply(ctx, cr.Spec.Driver.SideCars, &c, cr, matched)
			if len(cr.Spec.Driver.SideCars) == 0 {
				if matched.Version != "" {
					if img := matched.Images[*containers[i].Name]; img != "" {
						*c.Image = img
					}
				} else if cr.Spec.CustomRegistry != "" {
					*c.Image = operatorutils.ResolveImage(ctx, string(*c.Image), cr)
				}
			}
			newcontainers = append(newcontainers, c)
		}

	}

	controllerYAML.Deployment.Spec.Template.Spec.Containers = newcontainers
	// Update volumes
	for i, v := range controllerYAML.Deployment.Spec.Template.Spec.Volumes {
		newV := new(acorev1.VolumeApplyConfiguration)
		if *v.Name == "certs" {
			if driverType == "isilon" || driverType == "powerflex" {
				newV, err = getApplyCertVolume(cr)
			}
			if driverType == "unity" {
				newV, err = getApplyCertVolumeUnity(cr)
			}
			if driverType == "powermax" {
				newV, err = getApplyCertVolumePowermax(cr)
			}
			if driverType == "powerstore" {
				newV, err = getApplyCertVolumePowerstore(cr)
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
	bOwnerDeletion := cr.Spec.Driver.ForceRemoveDriver != nil && !*cr.Spec.Driver.ForceRemoveDriver
	kind := cr.Kind
	v1 := "storage.dell.com/v1"
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
func GetNode(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, driverType csmv1.DriverType, filename string, ct client.Client, matched operatorutils.VersionSpec) (*operatorutils.NodeYAML, error) {
	log := logger.GetLogger(ctx)
	version, err := operatorutils.GetVersion(ctx, &cr, operatorConfig)
	if err != nil {
		return nil, err
	}
	configMapPath := fmt.Sprintf("%s/driverconfig/%s/%s/%s", operatorConfig.ConfigDirectory, driverType, version, filename)
	log.Debugw("GetNode", "configMapPath", configMapPath)
	buf, err := os.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		log.Errorw("GetNode failed", "Error", err.Error())
		return nil, err
	}

	YamlString := operatorutils.ModifyCommonCR(string(buf), cr)
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
	if cr.Spec.Driver.CSIDriverType == "isilon" {
		YamlString = ModifyPowerScaleCR(YamlString, cr, "Node")
	}

	driverYAML, err := operatorutils.GetDriverYaml(YamlString, "DaemonSet")
	if err != nil {
		log.Errorw("GetNode Daemonset failed", "Error", err.Error())
		return nil, err
	}

	nodeYaml := driverYAML.(operatorutils.NodeYAML)

	if cr.Spec.Driver.DNSPolicy != "" {
		dnspolicy := corev1.DNSPolicy(cr.Spec.Driver.DNSPolicy)
		nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.DNSPolicy = &dnspolicy
	}
	defaultDNSPolicy := corev1.DNSClusterFirstWithHostNet
	if cr.Spec.Driver.DNSPolicy == "" {
		nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.DNSPolicy = &defaultDNSPolicy
	}

	if cr.Spec.Driver.Node != nil && len(cr.Spec.Driver.Node.Tolerations) != 0 {
		tols := make([]acorev1.TolerationApplyConfiguration, 0)
		for _, t := range cr.Spec.Driver.Node.Tolerations {
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

	if cr.Spec.Driver.Node != nil && cr.Spec.Driver.Node.NodeSelector != nil {
		nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.NodeSelector = cr.Spec.Driver.Node.NodeSelector
	}

	// Checking if driver image is present in config map
	found := false
	var image string
	var mdmInitContainerImage string
	for k, v := range matched.Images {
		if k == string(cr.Spec.Driver.CSIDriverType) {
			image = v
			mdmInitContainerImage = v
			found = true
			break
		}
	}

	containers := nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Containers
	newcontainers := make([]acorev1.ContainerApplyConfiguration, 0)
	for i, c := range containers {
		if c.Name != nil && string(*c.Name) == "driver" {
			if found {
				c.Image = &image
			} else if cr.Spec.CustomRegistry != "" {
				resolvedImagePath := operatorutils.ResolveImage(ctx, string(*c.Image), cr)
				c.Image = &resolvedImagePath
			} else {
				if cr.Spec.Driver.Common != nil {
					// With minimal, this will override the node image if the driver image is overridden.
					if cr.Spec.Driver.Common.Image != "" {
						image := string(cr.Spec.Driver.Common.Image)
						c.Image = &image
					}
				}
			}

			var commonEnvs, nodeEnvs []corev1.EnvVar
			if cr.Spec.Driver.Common != nil {
				commonEnvs = cr.Spec.Driver.Common.Envs
			}
			if cr.Spec.Driver.Node != nil {
				nodeEnvs = cr.Spec.Driver.Node.Envs
			}
			containers[i].Env = operatorutils.ReplaceAllApplyCustomEnvs(c.Env, commonEnvs, nodeEnvs)
			c.Env = containers[i].Env
		}
		removeContainer := false
		if string(*c.Name) == "sdc-monitor" {
			removeContainer = true
		}
		for _, s := range cr.Spec.Driver.SideCars {
			if s.Name == *c.Name {
				if s.Enabled == nil {
					if string(*c.Name) == "sdc-monitor" {
						removeContainer = true
						log.Infow("Container to be removed", "name", *c.Name)
					} else {
						removeContainer = false
						log.Infow("Container to be enabled", "name", *c.Name)
					}
				} else if !*s.Enabled {
					removeContainer = true
					log.Infow("Container to be removed", "name", *c.Name)
				} else {
					removeContainer = false
					log.Infow("Container to be enabled", "name", *c.Name)
				}
				break
			}
		}
		if !removeContainer {
			operatorutils.ReplaceAllContainerImageApply(operatorConfig.K8sVersion, &containers[i])
			operatorutils.UpdateSideCarApply(ctx, cr.Spec.Driver.SideCars, &containers[i], cr, matched)
			if len(cr.Spec.Driver.SideCars) == 0 {
				if matched.Version != "" {
					if img := matched.Images[*containers[i].Name]; img != "" {
						*c.Image = img
					}
				} else if cr.Spec.CustomRegistry != "" {
					*c.Image = operatorutils.ResolveImage(ctx, string(*c.Image), cr)
				}
			}
			newcontainers = append(newcontainers, c)
		}
	}

	nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Containers = newcontainers

	var updatedCr csmv1.ContainerStorageModule
	if cr.Spec.Driver.CSIDriverType == "powerflex" {
		updatedCr, err = SetSDCinitContainers(ctx, cr, ct)
		if err != nil {
			log.Errorw("Failed to set SDC init container", "Error", err.Error())
			return nil, err
		}
	}

	initcontainers := make([]acorev1.ContainerApplyConfiguration, 0)
	sdcEnabled := true
	if updatedCr.Spec.Driver.Node != nil {
		for _, env := range updatedCr.Spec.Driver.Node.Envs {
			if env.Name == "X_CSI_SDC_ENABLED" && env.Value == "false" {
				sdcEnabled = false
			}
		}
	}
	for _, ic := range nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.InitContainers {
		if *ic.Name != "sdc" || sdcEnabled {
			initcontainers = append(initcontainers, ic)
		}
	}

	for i := range initcontainers {
		operatorutils.ReplaceAllContainerImageApply(operatorConfig.K8sVersion, &initcontainers[i])
		operatorutils.UpdateInitContainerApply(ctx, updatedCr.Spec.Driver.InitContainers, &initcontainers[i], cr, matched)
		// mdm-container is exclusive to powerflex driver deamonset, will use the driver image as an init container
		if *initcontainers[i].Name == "mdm-container" {
			// driver minimial manifest may not have common section
			if cr.Spec.Driver.Common != nil {
				if string(cr.Spec.Driver.Common.Image) != "" {
					image := string(cr.Spec.Driver.Common.Image)
					initcontainers[i].Image = &image
				}
			}
			if mdmInitContainerImage != "" {
				initcontainers[i].Image = &mdmInitContainerImage
				log.Debugw("MDM initcontainer image resolved from ConfigMap",
					"mdmInitContainerImage", mdmInitContainerImage,
				)
			} else if updatedCr.Spec.CustomRegistry != "" {
				resolvedImagePath := operatorutils.ResolveImage(ctx, string(*initcontainers[i].Image), cr)
				initcontainers[i].Image = &resolvedImagePath
				log.Debugw(fmt.Sprintf("custom registry resolved initcontianer MDM image: %s", *initcontainers[i].Image))
			}
		}
		if *initcontainers[i].Name == "sdc" {
			// Checking if sdc initcontainer image is present in config map
			found2 := false
			var sdcImage string
			for k, v := range matched.Images {
				if k == "sdc" {
					sdcImage = v
					found2 = true
					break
				}
			}
			if found2 {
				initcontainers[i].Image = &sdcImage
				log.Infow("SDC initcontainer image resolved from ConfigMap",
					"image", sdcImage,
				)
			} else if updatedCr.Spec.CustomRegistry != "" {
				resolvedImagePath := operatorutils.ResolveImage(ctx, string(*initcontainers[i].Image), cr)
				initcontainers[i].Image = &resolvedImagePath
				log.Debugw(fmt.Sprintf("custom registry resolved initcontianer sdc image: %s", *initcontainers[i].Image))
			}
		}
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
			if cr.Spec.Driver.CSIDriverType == "powerstore" {
				newV, err = getApplyCertVolumePowerstore(cr)
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
func GetUpgradeInfo(ctx context.Context, operatorConfig operatorutils.OperatorConfig, driverType csmv1.DriverType, oldVersion string) (string, error) {
	log := logger.GetLogger(ctx)
	upgradeInfoPath := fmt.Sprintf("%s/driverconfig/%s/%s/upgrade-path.yaml", operatorConfig.ConfigDirectory, driverType, oldVersion)
	log.Debugw("GetUpgradeInfo", "upgradeInfoPath", upgradeInfoPath)

	buf, err := os.ReadFile(filepath.Clean(upgradeInfoPath))
	if err != nil {
		log.Errorw("GetUpgradeInfo failed", "Error", err.Error())
		return "", err
	}
	YamlString := string(buf)

	var upgradePath operatorutils.UpgradePaths
	err = yaml.Unmarshal([]byte(YamlString), &upgradePath)
	if err != nil {
		log.Errorw("GetUpgradeInfo yaml marshall failed", "Error", err.Error())
		return "", err
	}

	// Example return value: "v2.2.0"
	return upgradePath.MinUpgradePath, nil
}

// GetConfigMap get configmap
func GetConfigMap(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, driverName csmv1.DriverType) (*corev1.ConfigMap, error) {
	log := logger.GetLogger(ctx)
	var podmanLogFormat string
	var podmanLogLevel string
	version, err := operatorutils.GetVersion(ctx, &cr, operatorConfig)
	if err != nil {
		return nil, err
	}
	configMapPath := fmt.Sprintf("%s/driverconfig/%s/%s/driver-config-params.yaml", operatorConfig.ConfigDirectory, driverName, version)
	log.Debugw("GetConfigMap", "configMapPath", configMapPath)

	buf, err := os.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		log.Errorw("GetConfigMap failed", "Error", err.Error())
		return nil, err
	}
	YamlString := operatorutils.ModifyCommonCR(string(buf), cr)

	var configMap corev1.ConfigMap
	cmValue := ""
	var configMapData map[string]string
	err = yaml.Unmarshal([]byte(YamlString), &configMap)
	if err != nil {
		log.Errorw("GetConfigMap yaml marshall failed", "Error", err.Error())
		return nil, err
	}

	if cr.Spec.Driver.Common != nil {
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "CSI_LOG_LEVEL" || env.Name == CosiLogLevel {
				cmValue += fmt.Sprintf("\n%s: %s", env.Name, env.Value)
				podmanLogLevel = env.Value
			}
			if env.Name == "CSI_LOG_FORMAT" || env.Name == CosiLogFormat {
				cmValue += fmt.Sprintf("\n%s: %s", env.Name, env.Value)
				podmanLogFormat = env.Value
			}
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

	if cr.Spec.Driver.CSIDriverType == "powerflex" {
		if cr.Spec.Driver.Common != nil {
			for _, env := range cr.Spec.Driver.Common.Envs {
				if env.Name == "INTERFACE_NAMES" {
					cmValue += fmt.Sprintf("\n%s: ", "interfaceNames")
					for _, v := range strings.Split(env.Value, ",") {
						cmValue += fmt.Sprintf("\n  %s ", v)
					}
				}
			}
		}
	}

	if cr.Spec.Driver.CSIDriverType == csmv1.PowerScale {
		if cr.Spec.Driver.Common != nil {
			for _, env := range cr.Spec.Driver.Common.Envs {
				if env.Name == "AZ_RECONCILE_INTERVAL" {
					cmValue += fmt.Sprintf("\n%s: %s", env.Name, env.Value)
				}
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
func GetCSIDriver(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, driverName csmv1.DriverType) (*storagev1.CSIDriver, error) {
	log := logger.GetLogger(ctx)
	version, err := operatorutils.GetVersion(ctx, &cr, operatorConfig)
	if err != nil {
		return nil, err
	}
	configMapPath := fmt.Sprintf("%s/driverconfig/%s/%s/csidriver.yaml", operatorConfig.ConfigDirectory, driverName, version)
	log.Debugw("GetCSIDriver", "configMapPath", configMapPath)
	buf, err := os.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		log.Errorw("GetCSIDriver failed", "Error", err.Error())
		return nil, err
	}

	var csidriver storagev1.CSIDriver

	YamlString := operatorutils.ModifyCommonCR(string(buf), cr)
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
	if cr.Spec.Driver.CSIDriverSpec != nil && cr.Spec.Driver.CSIDriverSpec.FSGroupPolicy != "" {
		fsGroupPolicy := storagev1.NoneFSGroupPolicy
		switch cr.Spec.Driver.CSIDriverSpec.FSGroupPolicy {
		case "ReadWriteOnceWithFSType":
			fsGroupPolicy = storagev1.ReadWriteOnceWithFSTypeFSGroupPolicy
		case "File":
			fsGroupPolicy = storagev1.FileFSGroupPolicy
		}
		csidriver.Spec.FSGroupPolicy = &fsGroupPolicy
		log.Debugw("GetCSIDriver", "fsGroupPolicy", fsGroupPolicy)
	}

	return &csidriver, nil
}

func IsCSMDREnabled(cr csmv1.ContainerStorageModule) string {
	enableCSMDR := "true"

	if cr.Spec.Driver.Common != nil {
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "X_CSM_DR_ENABLED" && env.Value != "" {
				enableCSMDR = env.Value
				break
			}
		}
	}

	return enableCSMDR
}
