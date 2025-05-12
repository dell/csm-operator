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
	"reflect"
	"regexp"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/client-go/applyconfigurations/apps/v1"
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

	// PowerFlexCSMNameSpace - namespace CSM is found in. Needed for cases where pod namespace is not namespace of CSM
	PowerFlexCSMNameSpace string = "<CSM_NAMESPACE>"

	// ScaleioBinPath - name of volume that is mounted by the CSI plugin when not running on OCP
	ScaleioBinPath = "scaleio-path-bin"

	// SftpKeys - name of volume that is mounted for sftp
	SftpKeys = "sftp-keys"

	// PowerFlexDebug - will be used to control the GOSCALEIO_DEBUG variable
	PowerFlexDebug string = "<GOSCALEIO_DEBUG>"

	// PowerFlexShowHTTP - will be used to control the GOSCALEIO_SHOWHTTP variable
	PowerFlexShowHTTP string = "<GOSCALEIO_SHOWHTTP>"

	// PowerFlexShowHTTP - will be used to control the GOSCALEIO_SHOWHTTP variable
	PowerFlexSftpRepoAddress string = "<X_CSI_SFTP_REPO_ADDRESS>"

	// PowerFlexShowHTTP - will be used to control the GOSCALEIO_SHOWHTTP variable
	PowerFlexSftpRepoUser string = "<X_CSI_SFTP_REPO_USER>"

	// PowerFlexSdcRepoEnabled - will be used to control the GOSCALEIO_SHOWHTTP variable
	PowerFlexSdcRepoEnabled string = "<SDC_SFTP_REPO_ENABLED>"
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

	// Check if MDM is set in the secret
	_, err := GetMDMFromSecret(ctx, cr, ct)
	if err != nil {
		return err
	}

	return nil
}

func SetSDCinitContainers(ctx context.Context, cr csmv1.ContainerStorageModule, ct client.Client) (csmv1.ContainerStorageModule, error) {
	mdmVar, _ := GetMDMFromSecret(ctx, &cr, ct)

	// Check if SDC is enabled
	sdcEnabled := true
	if cr.Spec.Driver.Node != nil {
		for _, env := range cr.Spec.Driver.Node.Envs {
			if env.Name == "X_CSI_SDC_ENABLED" && env.Value == "false" {
				sdcEnabled = false
				break
			}
		}
	}

	// Update init containers
	var newInitContainers []csmv1.ContainerTemplate
	for _, initcontainer := range cr.Spec.Driver.InitContainers {
		if initcontainer.Name == "sdc" && sdcEnabled {
			// Ensure MDM env variable is set
			mdmUpdated := false
			for i, env := range initcontainer.Envs {
				if env.Name == "MDM" {
					initcontainer.Envs[i].Value = mdmVar
					mdmUpdated = true
					break
				}
			}
			// If MDM not found, update it from secret
			if !mdmUpdated {
				initcontainer.Envs = append(initcontainer.Envs, corev1.EnvVar{
					Name:  "MDM",
					Value: mdmVar,
				})
			}
		}
		newInitContainers = append(newInitContainers, initcontainer)
	}

	// If there is no init containers and SDC is enabled, add a sdc init container
	if len(newInitContainers) == 0 && sdcEnabled {
		newInitContainers = append(newInitContainers, csmv1.ContainerTemplate{
			Name: "sdc",
			Envs: []corev1.EnvVar{{Name: "MDM", Value: mdmVar}},
		})
	}
	cr.Spec.Driver.InitContainers = newInitContainers

	// Update sidecar containers
	for i := range cr.Spec.Driver.SideCars {
		if cr.Spec.Driver.SideCars[i].Name == "sdc-monitor" {
			// Ensure MDM env variable is set
			mdmUpdated := false
			for j, env := range cr.Spec.Driver.SideCars[i].Envs {
				if env.Name == "MDM" {
					cr.Spec.Driver.SideCars[i].Envs[j].Value = mdmVar
					mdmUpdated = true
					break
				}
			}
			// If MDM not found, update it from secret
			if !mdmUpdated {
				cr.Spec.Driver.SideCars[i].Envs = append(cr.Spec.Driver.SideCars[i].Envs, corev1.EnvVar{
					Name:  "MDM",
					Value: mdmVar,
				})
			}
		}
	}

	// If no sidecars are present, add a new "sdc-monitor" sidecar with MDM
	if len(cr.Spec.Driver.SideCars) == 0 {
		cr.Spec.Driver.SideCars = []csmv1.ContainerTemplate{
			{
				Name: "sdc-monitor",
				Envs: []corev1.EnvVar{{Name: "MDM", Value: mdmVar}},
			},
		}
	}

	return cr, nil
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
	debug := "false"
	showHTTP := "false"
	sftpRepoAddress := "sftp://0.0.0.0"
	sftpRepoUser := ""
	sftpEnabled := ""

	if cr.Spec.Driver.Common != nil {
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "GOSCALEIO_DEBUG" {
				debug = env.Value
			}
			if env.Name == "GOSCALEIO_SHOWHTTP" {
				showHTTP = env.Value
			}
		}
	}

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
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorController)
		yamlString = strings.ReplaceAll(yamlString, CsiPowerflexExternalAccess, powerflexExternalAccess)
		yamlString = strings.ReplaceAll(yamlString, PowerFlexCSMNameSpace, cr.Namespace)
		yamlString = strings.ReplaceAll(yamlString, PowerFlexDebug, debug)
		yamlString = strings.ReplaceAll(yamlString, PowerFlexShowHTTP, showHTTP)

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
				if env.Name == "REPO_ADDRESS" {
					sftpRepoAddress = env.Value
				}
				if env.Name == "REPO_USER" {
					sftpRepoUser = env.Value
				}
				if env.Name == "SDC_SFTP_REPO_ENABLED" {
					sftpEnabled = env.Value
				}
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiSdcEnabled, sdcEnabled)
		yamlString = strings.ReplaceAll(yamlString, CsiApproveSdcEnabled, approveSdcEnabled)
		yamlString = strings.ReplaceAll(yamlString, CsiRenameSdcEnabled, renameSdcEnabled)
		yamlString = strings.ReplaceAll(yamlString, CsiPrefixRenameSdc, renameSdcPrefix)
		yamlString = strings.ReplaceAll(yamlString, CsiVxflexosMaxVolumesPerNode, maxVolumesPerNode)
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorNode)
		yamlString = strings.ReplaceAll(yamlString, PowerFlexCSMNameSpace, cr.Namespace)
		yamlString = strings.ReplaceAll(yamlString, PowerFlexDebug, debug)
		yamlString = strings.ReplaceAll(yamlString, PowerFlexShowHTTP, showHTTP)
		yamlString = strings.ReplaceAll(yamlString, PowerFlexSftpRepoAddress, sftpRepoAddress)
		yamlString = strings.ReplaceAll(yamlString, PowerFlexSftpRepoUser, sftpRepoUser)
		yamlString = strings.ReplaceAll(yamlString, PowerFlexSdcRepoEnabled, sftpEnabled)

	case "CSIDriverSpec":
		if cr.Spec.Driver.CSIDriverSpec != nil && cr.Spec.Driver.CSIDriverSpec.StorageCapacity {
			storageCapacity = "true"
		}
		yamlString = strings.ReplaceAll(yamlString, CsiStorageCapacityEnabled, storageCapacity)
		yamlString = strings.ReplaceAll(yamlString, CsiVxflexosQuotaEnabled, enableQuota)
	}
	return yamlString
}

// ValidateZones - zone validation for topology aware clusters
func ValidateZones(ctx context.Context, cr *csmv1.ContainerStorageModule, ct client.Client) error {
	secretName := cr.Name + "-config"
	err := ValidateZonesInSecret(ctx, ct, cr.Namespace, secretName)
	return err
}

// ValidateZonesInSecret - inspects incoming secret for zone validity
func ValidateZonesInSecret(ctx context.Context, kube client.Client, namespace string, secret string) error {
	log := logger.GetLogger(ctx)

	arraySecret, err := utils.GetSecret(ctx, secret, namespace, kube)
	if err != nil {
		return fmt.Errorf("reading secret [%s] error %v", secret, err)
	}

	type Zone struct {
		Name     string `json:"name,omitempty"`
		LabelKey string `json:"labelKey,omitempty"`
	}

	type StorageArrayConfig struct {
		SystemID string `json:"systemID"`
		Zone     Zone   `json:"zone,omitempty"`
	}

	data := arraySecret.Data
	configBytes := data["config"]

	if string(configBytes) != "" {
		yamlConfig := make([]StorageArrayConfig, 0)
		configs, err := yaml.JSONToYAML(configBytes)
		if err != nil {
			return fmt.Errorf("malformed json in array secret - unable to parse multi-array configuration %v", err)
		}
		err = yaml.Unmarshal(configs, &yamlConfig)
		if err != nil {
			return fmt.Errorf("unable to unmarshal array secret %v", err)
		}

		var labelKey string
		var numArrays, numArraysWithZone int
		numArrays = len(yamlConfig)
		for _, configParam := range yamlConfig {
			if configParam.SystemID == "" {
				return fmt.Errorf("invalid value for SystemID")
			}
			if reflect.DeepEqual(configParam.Zone, Zone{}) {
				log.Infof("Zone is not specified for SystemID: %s", configParam.SystemID)
			} else {
				log.Infof("Zone is specified for SystemID: %s", configParam.SystemID)
				if configParam.Zone.LabelKey == "" {
					return fmt.Errorf("zone LabelKey is empty or not specified for SystemID: %s",
						configParam.SystemID)
				}

				if labelKey == "" {
					labelKey = configParam.Zone.LabelKey
				} else {
					if labelKey != configParam.Zone.LabelKey {
						return fmt.Errorf("labelKey is not consistent across all arrays in secret")
					}
				}

				if configParam.Zone.Name == "" {
					return fmt.Errorf("zone name is empty or not specified for SystemID: %s",
						configParam.SystemID)
				}
				numArraysWithZone++
			}
		}

		log.Infof("found %d arrays zoning on %d", numArrays, numArraysWithZone)
		if numArraysWithZone > 0 && numArrays != numArraysWithZone {
			return fmt.Errorf("not all arrays have zoning configured. Check the array info secret, zone key should be the same for all arrays")
		} else if numArraysWithZone == 0 {
			log.Info("Zoning information not found in the array secret. Continue with topology-unaware driver installation mode")
		}
	} else {
		return fmt.Errorf("array details are not provided in secret")
	}

	return nil
}

func RemoveVolume(configuration *v1.DaemonSetApplyConfiguration, volumeName string) error {
	if configuration == nil {
		return fmt.Errorf("RemoveVolume called with a nil daemonset")
	}
	podTemplate := configuration.Spec.Template
	for i, vol := range podTemplate.Spec.Volumes {
		if vol.Name != nil && *vol.Name == volumeName {
			podTemplate.Spec.Volumes = append(podTemplate.Spec.Volumes[0:i], podTemplate.Spec.Volumes[i+1:]...)
			break
		}
	}
	for c := range podTemplate.Spec.Containers {
		for i, volMount := range podTemplate.Spec.Containers[c].VolumeMounts {
			if volMount.Name != nil && *volMount.Name == volumeName {
				podTemplate.Spec.Containers[c].VolumeMounts = append(podTemplate.Spec.Containers[c].VolumeMounts[0:i], podTemplate.Spec.Containers[c].VolumeMounts[i+1:]...)
				return nil
			}
		}
	}
	return nil
}

func RemoveInitVolume(configuration *v1.DaemonSetApplyConfiguration, volumeName string) error {
	if configuration == nil {
		return fmt.Errorf("RemoveVolume called with a nil daemonset")
	}
	podTemplate := configuration.Spec.Template
	for i, vol := range podTemplate.Spec.Volumes {
		if vol.Name != nil && *vol.Name == volumeName {
			podTemplate.Spec.Volumes = append(podTemplate.Spec.Volumes[0:i], podTemplate.Spec.Volumes[i+1:]...)
			break
		}
	}
	for c := range podTemplate.Spec.InitContainers {
		for i, volMount := range podTemplate.Spec.InitContainers[c].VolumeMounts {
			if volMount.Name != nil && *volMount.Name == volumeName {
				podTemplate.Spec.InitContainers[c].VolumeMounts = append(podTemplate.Spec.InitContainers[c].VolumeMounts[0:i], podTemplate.Spec.InitContainers[c].VolumeMounts[i+1:]...)
				return nil
			}
		}
	}
	return nil
}
