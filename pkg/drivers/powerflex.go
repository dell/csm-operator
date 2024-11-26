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
	"regexp"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	// PowerFlexPluginIdentifier -
	PowerFlexPluginIdentifier = "powerflex"

	// PowerFlexConfigParamsVolumeMount -
	PowerFlexConfigParamsVolumeMount = "vxflexos-config-params"

	// CsiSdcEnabled - Flag to enable/disable SDC
	CsiSdcEnabled = "<X_CSI_SDC_ENABLED>"

	// CsiApproveSdcEnabled - Flag to enable/disable SDC approval
	CsiApproveSdcEnabled = "<X_CSI_APPROVE_SDC_ENABLED>"

	// CsiRenameSdcEnabled - Flag to enable/disable rename of SDC
	CsiRenameSdcEnabled = "<X_CSI_RENAME_SDC_ENABLED>"

	// CsiPrefixRenameSdc - String to rename SDC
	CsiPrefixRenameSdc = "<X_CSI_RENAME_SDC_PREFIX>"

	// CsiVxflexosMaxVolumesPerNode - Max volumes that the controller could schedule on a node
	CsiVxflexosMaxVolumesPerNode = "<X_CSI_MAX_VOLUMES_PER_NODE>"

	// CsiVxflexosQuotaEnabled - Flag to enable/disable setting of quota for NFS volumes
	CsiVxflexosQuotaEnabled = "<X_CSI_QUOTA_ENABLED>"

	// CsiPowerflexExternalAccess -  External Access flag
	CsiPowerflexExternalAccess = "<X_CSI_POWERFLEX_EXTERNAL_ACCESS>"

	// CsiDebug -  Debug flag
	CsiDebug = "<X_CSI_DEBUG>"
)

// PrecheckPowerFlex do input validation
func PrecheckPowerFlex(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, ct client.Client) error {
	log := logger.GetLogger(ctx)

	// Check if driver version is supported by doing a stat on a config file
	configFilePath := fmt.Sprintf("%s/driverconfig/%s/%s/upgrade-path.yaml", operatorConfig.ConfigDirectory, csmv1.PowerFlex, cr.Spec.Driver.ConfigVersion)
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Errorw("PreCheckPowerFlex failed in version check", "Error", err.Error())
		return fmt.Errorf("%s %s not supported", csmv1.PowerFlexName, cr.Spec.Driver.ConfigVersion)
	}

	mdmVar, err := GetMDMFromSecret(ctx, cr, ct)
	if err != nil {
		return err
	}

	sdcEnabled := true
	if cr.Spec.Driver.Node != nil {
		for _, env := range cr.Spec.Driver.Node.Envs {
			if env.Name == "X_CSI_SDC_ENABLED" && env.Value == "false" {
				sdcEnabled = false
			}
		}
	}
	newInitContainers := make([]csmv1.ContainerTemplate, 0)
	for _, initcontainer := range cr.Spec.Driver.InitContainers {
		if initcontainer.Name != "sdc" {
			newInitContainers = append(newInitContainers, initcontainer)
			continue
		}

		if sdcEnabled {
			for i := range initcontainer.Envs {
				if initcontainer.Envs[i].Name == "MDM" {
					initcontainer.Envs[i].Value = mdmVar
					break
				}
			}
			newInitContainers = append(newInitContainers, initcontainer)
		}
	}
	cr.Spec.Driver.InitContainers = newInitContainers

	for _, sidecar := range cr.Spec.Driver.SideCars {
		if sidecar.Name == "sdc-monitor" {
			for i := range sidecar.Envs {
				if sidecar.Envs[i].Name == "MDM" {
					sidecar.Envs[i].Value = mdmVar
					break
				}
			}
		}
	}

	return nil
}

// GetMDMFromSecret - Get MDM value from secret
func GetMDMFromSecret(ctx context.Context, cr *csmv1.ContainerStorageModule, ct client.Client) (string, error) {
	log := logger.GetLogger(ctx)
	secretName := cr.Name + "-config"
	credSecret, err := utils.GetSecret(ctx, secretName, cr.GetNamespace(), ct)
	if err != nil {
		return "", fmt.Errorf("reading secret [%s] error [%s]", secretName, err)
	}

	type StorageArrayConfig struct {
		Username                  string `json:"username"`
		Password                  string `json:"password"`
		SystemID                  string `json:"systemId"`
		Endpoint                  string `json:"endpoint"`
		SkipCertificateValidation bool   `json:"skipCertificateValidation,omitempty"`
		AllSystemNames            string `json:"allSystemNames"`
		IsDefault                 bool   `json:"isDefault,omitempty"`
		MDM                       string `json:"mdm"`
	}

	data := credSecret.Data
	configBytes := data["config"]
	mdmVal := ""
	mdmFin := ""
	ismdmip := false

	if string(configBytes) != "" {
		yamlConfig := make([]StorageArrayConfig, 0)
		configs, err := yaml.JSONToYAML(configBytes)
		if err != nil {
			return "", fmt.Errorf("unable to parse multi-array configuration[%v]", err)
		}
		// Not checking the return value here because any invalid yaml would already be detected by the JSONToYAML function above
		_ = yaml.Unmarshal(configs, &yamlConfig)

		var noOfDefaultArrays int
		tempMapToFindDuplicates := make(map[string]interface{}, 0)
		for i, config := range yamlConfig {
			if config.SystemID == "" {
				return "", fmt.Errorf("invalid value for SystemID at index [%d]", i)
			}
			if config.Username == "" {
				return "", fmt.Errorf("invalid value for Username at index [%d]", i)
			}
			if config.Password == "" {
				return "", fmt.Errorf("invalid value for Password at index [%d]", i)
			}
			if config.Endpoint == "" {
				return "", fmt.Errorf("invalid value for RestGateway at index [%d]", i)
			}
			if config.MDM != "" {
				mdmFin, ismdmip = ValidateIPAddress(config.MDM)
				if !ismdmip {
					return "", fmt.Errorf("Invalid MDM value. Ip address should be numeric and comma separated without space")
				}
				if i == 0 {
					mdmVal += mdmFin
				} else {
					mdmVal += "&" + mdmFin
				}
			}
			if config.AllSystemNames != "" {
				names := strings.Split(config.AllSystemNames, ",")
				log.Info("For systemID %s configured System Names found %#v ", config.SystemID, names)
			}

			if _, ok := tempMapToFindDuplicates[config.SystemID]; ok {
				return "", fmt.Errorf("Duplicate SystemID [%s] found in storageArrayList parameter", config.SystemID)
			}
			tempMapToFindDuplicates[config.SystemID] = nil

			if config.IsDefault {
				noOfDefaultArrays++
			}

			if noOfDefaultArrays > 1 {
				return "", fmt.Errorf("'isDefaultArray' parameter located in multiple places SystemID: %s. 'isDefaultArray' parameter should present only once in the storageArrayList", config.SystemID)
			}
		}
	} else {
		return "", fmt.Errorf("Arrays details are not provided in vxflexos-config secret")
	}
	return mdmVal, nil
}

// ValidateIPAddress validates that a proper set of IPs has been provided
func ValidateIPAddress(ipAdd string) (string, bool) {
	trimIP := strings.Split(ipAdd, ",")
	if len(trimIP) < 1 {
		return "", false
	}
	newIP := ""
	for i := range trimIP {
		trimIP[i] = strings.TrimSpace(trimIP[i])
		istrueip := IsIpv4Regex(trimIP[i])
		if istrueip {
			newIP = strings.Join(trimIP[:], ",")
		} else {
			return newIP, false
		}
	}
	return newIP, true
}

var ipRegex, _ = regexp.Compile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)

// IsIpv4Regex - Matches Ipaddress with regex and returns error if the Ip Address doesn't match regex
func IsIpv4Regex(ipAddress string) bool {
	return ipRegex.MatchString(ipAddress)
}

// ModifyPowerflexCR - Set environment variables provided in CR
func ModifyPowerflexCR(yamlString string, cr csmv1.ContainerStorageModule, fileType string) string {
	sdcEnabled := "true"
	approveSdcEnabled := ""
	renameSdcEnabled := ""
	renameSdcPrefix := ""
	maxVolumesPerNode := ""
	storageCapacity := "false"
	enableQuota := ""
	powerflexExternalAccess := ""
	healthMonitorController := "false"
	healthMonitorNode := "false"
	csiDebug := "true"

	// nolint:gosec
	switch fileType {
	case "Controller":
		if cr.Spec.Driver.Controller != nil {
			for _, env := range cr.Spec.Driver.Controller.Envs {
				if env.Name == "X_CSI_POWERFLEX_EXTERNAL_ACCESS" {
					powerflexExternalAccess = env.Value
				}
				if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
					healthMonitorController = env.Value
				}
				if env.Name == "X_CSI_DEBUG" {
					csiDebug = env.Value
				}
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorController)
		yamlString = strings.ReplaceAll(yamlString, CsiPowerflexExternalAccess, powerflexExternalAccess)
		yamlString = strings.ReplaceAll(yamlString, CsiDebug, csiDebug)

	case "Node":
		if cr.Spec.Driver.Node != nil {
			for _, env := range cr.Spec.Driver.Node.Envs {
				if env.Name == "X_CSI_SDC_ENABLED" {
					sdcEnabled = env.Value
				}
				if env.Name == "X_CSI_APPROVE_SDC_ENABLED" {
					approveSdcEnabled = env.Value
				}
				if env.Name == "X_CSI_RENAME_SDC_ENABLED" {
					renameSdcEnabled = env.Value
				}
				if env.Name == "X_CSI_RENAME_SDC_PREFIX" {
					renameSdcPrefix = env.Value
				}
				if env.Name == "X_CSI_MAX_VOLUMES_PER_NODE" {
					maxVolumesPerNode = env.Value
				}
				if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
					healthMonitorNode = env.Value
				}
				if env.Name == "X_CSI_DEBUG" {
					csiDebug = env.Value
				}
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiSdcEnabled, sdcEnabled)
		yamlString = strings.ReplaceAll(yamlString, CsiApproveSdcEnabled, approveSdcEnabled)
		yamlString = strings.ReplaceAll(yamlString, CsiRenameSdcEnabled, renameSdcEnabled)
		yamlString = strings.ReplaceAll(yamlString, CsiPrefixRenameSdc, renameSdcPrefix)
		yamlString = strings.ReplaceAll(yamlString, CsiVxflexosMaxVolumesPerNode, maxVolumesPerNode)
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorNode)
		yamlString = strings.ReplaceAll(yamlString, CsiDebug, csiDebug)

	case "CSIDriverSpec":
		if cr.Spec.Driver.CSIDriverSpec != nil && cr.Spec.Driver.CSIDriverSpec.StorageCapacity {
			storageCapacity = "true"
		}
		yamlString = strings.ReplaceAll(yamlString, CsiStorageCapacityEnabled, storageCapacity)
		yamlString = strings.ReplaceAll(yamlString, CsiVxflexosQuotaEnabled, enableQuota)
	}
	return yamlString
}
