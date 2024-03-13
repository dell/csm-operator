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
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	// PowerFlexPluginIdentifier -
	PowerFlexPluginIdentifier = "powerflex"

	// PowerFlexConfigParamsVolumeMount -
	PowerFlexConfigParamsVolumeMount = "vxflexos-config-params"

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


	// SecretName is a placeholder for the pflex config secret name
	SecretName = "<SecretName>"
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
	var newmdm corev1.EnvVar
	for _, initcontainer := range cr.Spec.Driver.InitContainers {
		if initcontainer.Name == "sdc" {
			k := 0
			initenv := initcontainer.Envs
			for c, env := range initenv {
				if env.Name == "MDM" {
					env.Value = mdmVar
					newmdm = env
					k = c
					break
				}
			}
			initenv[k] = newmdm
			break
		}
	}

	for _, sidecar := range cr.Spec.Driver.SideCars {
		if sidecar.Name == "sdc-monitor" {
			sidenv := sidecar.Envs
			var updatenv corev1.EnvVar
			j := 0
			for c, env := range sidenv {
				if env.Name == "MDM" {
					env.Value = mdmVar
					updatenv = env
					j = c
					break
				}
			}
			sidenv[j] = updatenv
		}
	}

	return nil
}

// GetMDMFromSecret - Get MDM value from secret
func GetMDMFromSecret(ctx context.Context, cr *csmv1.ContainerStorageModule, ct client.Client) (string, error) {
	log := logger.GetLogger(ctx)
	secretName := cr.Name + "-config"
	if cr.Spec.Driver.AuthSecret != "" {
		secretName = cr.Spec.Driver.AuthSecret
	}
	
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
	approveSdcEnabled := ""
	renameSdcEnabled := ""
	renameSdcPrefix := ""
	maxVolumesPerNode := ""
	storageCapacity := "false"
	enableQuota := ""
	powerflexExternalAccess := ""
	healthMonitorController := ""
	healthMonitorNode := ""

	secretName := cr.Name + "-config"
	if cr.Spec.Driver.AuthSecret != "" {
		secretName = cr.Spec.Driver.AuthSecret
	}

	// nolint:gosec
	switch fileType {
	case "Controller":
		for _, env := range cr.Spec.Driver.Controller.Envs {
			if env.Name == "X_CSI_POWERFLEX_EXTERNAL_ACCESS" {
				powerflexExternalAccess = env.Value
			}
			if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
				healthMonitorController = env.Value
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorController)
		yamlString = strings.ReplaceAll(yamlString, CsiPowerflexExternalAccess, powerflexExternalAccess)
		yamlString = strings.ReplaceAll(yamlString, SecretName, secretName)

	case "Node":
		for _, env := range cr.Spec.Driver.Node.Envs {
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
			if env.Name == "X_CSI_QUOTA_ENABLED" {
				enableQuota = env.Value
			}
			if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
				healthMonitorNode = env.Value
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiApproveSdcEnabled, approveSdcEnabled)
		yamlString = strings.ReplaceAll(yamlString, CsiRenameSdcEnabled, renameSdcEnabled)
		yamlString = strings.ReplaceAll(yamlString, CsiPrefixRenameSdc, renameSdcPrefix)
		yamlString = strings.ReplaceAll(yamlString, CsiVxflexosMaxVolumesPerNode, maxVolumesPerNode)
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorNode)
		yamlString = strings.ReplaceAll(yamlString, SecretName, secretName)

	case "CSIDriverSpec":
		if cr.Spec.Driver.CSIDriverSpec.StorageCapacity {
			storageCapacity = "true"
		}
		yamlString = strings.ReplaceAll(yamlString, CsiStorageCapacityEnabled, storageCapacity)
		yamlString = strings.ReplaceAll(yamlString, CsiVxflexosQuotaEnabled, enableQuota)
	}
	return yamlString
}
