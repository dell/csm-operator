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

package drivers

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// PowerStorePluginIdentifier -
	PowerStorePluginIdentifier = "powerstore"

	// PowerStoreConfigParamsVolumeMount -
	PowerStoreConfigParamsVolumeMount = "powerstore-config-params"

	// CsiPowerstoreNodeNamePrefix - Node Name Prefix
	CsiPowerstoreNodeNamePrefix = "<X_CSI_POWERSTORE_NODE_NAME_PREFIX>"

	// CsiPowerstoreMaxVolumesPerNode - Maximum Volumes Per Node
	CsiPowerstoreMaxVolumesPerNode = "<X_CSI_POWERSTORE_MAX_VOLUMES_PER_NODE>"

	// CsiFcPortFilterFilePath - Fc Port Filter File Path
	CsiFcPortFilterFilePath = "<X_CSI_FC_PORTS_FILTER_FILE_PATH>"

	// CsiNfsAcls - variable setting the permissions on NFS mount directory
	CsiNfsAcls = "<X_CSI_NFS_ACLS>"

	// CsiHealthMonitorEnabled - health monitor flag
	CsiHealthMonitorEnabled = "<X_CSI_HEALTH_MONITOR_ENABLED>"

	// CsiPowerstoreEnableChap -  CHAP flag
	CsiPowerstoreEnableChap = "<X_CSI_POWERSTORE_ENABLE_CHAP>"

	// CsiPowerstoreExternalAccess -  External Access flag
	CsiPowerstoreExternalAccess = "<X_CSI_POWERSTORE_EXTERNAL_ACCESS>"
	// CsiStorageCapacityEnabled - Storage capacity flag
	CsiStorageCapacityEnabled = "false"

	// PowerStoreCSMNameSpace - namespace CSM is found in. Needed for cases where pod namespace is not namespace of CSM
	PowerStoreCSMNameSpace string = "<CSM_NAMESPACE>"

	// PowerStoreDebug - will be used to control the GOPOWERSTORE_DEBUG variable
	PowerStoreDebug string = "<GOPOWERSTORE_DEBUG>"

	// PowerStoreNfsClientPort - NFS Client Port
	PowerStoreNfsClientPort = "<X_CSI_NFS_CLIENT_PORT>"

	// PowerStoreNfsClientPort - NFS Server Port
	PowerStoreNfsServerPort = "<X_CSI_NFS_SERVER_PORT>"

	// PowerStoreNfsExportDirectory - NFS Export Directory
	PowerStoreNfsExportDirectory = "<X_CSI_NFS_EXPORT_DIRECTORY>"

	// CSMAuthEnabled - CSI Volume name Prefix
	CSMAuthEnabled string = "<X_CSM_AUTH_ENABLED>"

	// PowerStoreAPITimeout - Powerstore REST API Timeout
	PowerStoreAPITimeout = "<X_CSI_POWERSTORE_API_TIMEOUT>"

	// PodmonArrayConnectivityTimeout - Podmon Array Connectivity Timeout
	PodmonArrayConnectivityTimeout = "<X_CSI_PODMON_ARRAY_CONNECTIVITY_TIMEOUT>"
)

// PrecheckPowerStore do input validation
func PrecheckPowerStore(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, ct client.Client) error {
	log := logger.GetLogger(ctx)
	// Check for secret only
	config := cr.Name + "-config"

	if cr.Spec.Driver.AuthSecret != "" {
		config = cr.Spec.Driver.AuthSecret
	}

	// Check if driver version is supported by doing a stat on a config file
	configFilePath := fmt.Sprintf("%s/driverconfig/powerstore/%s/upgrade-path.yaml", operatorConfig.ConfigDirectory, cr.Spec.Driver.ConfigVersion)
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Errorw("PreCheckPowerStore failed in version check", "Error", err.Error())
		return fmt.Errorf("%s %s not supported", csmv1.PowerStore, cr.Spec.Driver.ConfigVersion)
	}

	// Default values
	skipCertValid := true
	certCount := 1

	// Check environment variables from the CR spec
	if cr.Spec.Driver.Common != nil {
		for _, env := range cr.Spec.Driver.Common.Envs {
			switch env.Name {
			case "X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION":
				certTempCheck, err := strconv.ParseBool(env.Value)
				if err != nil {
					return fmt.Errorf("invalid value for X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION: %s (%v)", env.Value, err)
				}
				skipCertValid = certTempCheck

			case "CERT_SECRET_COUNT":
				certTempCount, err := strconv.ParseInt(env.Value, 0, 8)
				if err != nil {
					return fmt.Errorf("invalid value for CERT_SECRET_COUNT: %s (%v)", env.Value, err)
				}
				certCount = int(certTempCount)
			}
		}
	}

	secrets := []string{config}
	log.Debugw("preCheck", "secrets", len(secrets), "certCount", certCount, "Namespace", cr.Namespace)
	if !skipCertValid {
		for i := 0; i < certCount; i++ {
			secrets = append(secrets, fmt.Sprintf("%s-certs-%d", cr.Name, i))
		}
	}

	for _, name := range secrets {
		found := &corev1.Secret{}
		err := ct.Get(ctx, types.NamespacedName{Name: name, Namespace: cr.GetNamespace()}, found)
		if err != nil {
			log.Error(err, " Failed query for secret ", name, "Namespace", cr.Namespace)
			if errors.IsNotFound(err) {
				return fmt.Errorf("failed to find secret %s", name)
			}
		}
	}

	return nil
}

// ModifyPowerstoreCR -
func ModifyPowerstoreCR(yamlString string, cr csmv1.ContainerStorageModule, fileType string) string {
	// Parameters to initialise CR values
	nodePrefix := ""
	fcPortFilter := ""
	nfsAcls := ""
	healthMonitorController := ""
	chap := ""
	healthMonitorNode := ""
	powerstoreExternalAccess := ""
	storageCapacity := "false"
	maxVolumesPerNode := ""
	nfsClientPort := "2050"
	nfsServerPort := "2049"
	nfsExportDirectory := "/var/lib/dell/nfs"
	powerstoreAPITimeout := "120s"
	podmonArrayConnectivityTimeout := "10s"
	debug := "false"
	authEnabled := "false"

	if cr.Spec.Driver.Common != nil {
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "GOPOWERSTORE_DEBUG" {
				debug = env.Value
			}
			if env.Name == "X_CSI_NFS_CLIENT_PORT" && env.Value != "" {
				nfsClientPort = env.Value
			}
			if env.Name == "X_CSI_NFS_SERVER_PORT" && env.Value != "" {
				nfsServerPort = env.Value
			}
			if env.Name == "X_CSI_NFS_EXPORT_DIRECTORY" && env.Value != "" {
				nfsExportDirectory = env.Value
			}
			if env.Name == "X_CSI_POWERSTORE_API_TIMEOUT" && env.Value != "" {
				powerstoreAPITimeout = env.Value
			}
			if env.Name == "X_CSI_PODMON_ARRAY_CONNECTIVITY_TIMEOUT" && env.Value != "" {
				podmonArrayConnectivityTimeout = env.Value
			}
		}
	}

	switch fileType {
	case "Node":
		if cr.Spec.Driver.Common != nil {
			for _, env := range cr.Spec.Driver.Common.Envs {
				if env.Name == "X_CSI_POWERSTORE_NODE_NAME_PREFIX" {
					nodePrefix = env.Value
				}
				if env.Name == "X_CSI_FC_PORTS_FILTER_FILE_PATH" {
					fcPortFilter = env.Value
				}
			}
		}
		if cr.Spec.Driver.Node != nil {
			for _, env := range cr.Spec.Driver.Node.Envs {
				if env.Name == "X_CSI_POWERSTORE_ENABLE_CHAP" {
					chap = env.Value
				}
				if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
					healthMonitorNode = env.Value
				}
				if env.Name == "X_CSI_POWERSTORE_MAX_VOLUMES_PER_NODE" {
					maxVolumesPerNode = env.Value
				}
			}
			//	Set the env. whether authorization is enabled or not in the node to trim the tenant prefix in the driver
			for i, mod := range cr.Spec.Modules {
				if mod.Name == csmv1.Authorization {
					cr.Spec.Driver.Node.Envs = append(cr.Spec.Driver.Node.Envs, corev1.EnvVar{
						Name:  "X_CSM_AUTH_ENABLED",
						Value: strconv.FormatBool(cr.Spec.Modules[i].Enabled),
					})
					authEnabled = strconv.FormatBool(cr.Spec.Modules[i].Enabled)
					break
				}
			}
		}

		yamlString = strings.ReplaceAll(yamlString, CsiPowerstoreNodeNamePrefix, nodePrefix)
		yamlString = strings.ReplaceAll(yamlString, CsiFcPortFilterFilePath, fcPortFilter)
		yamlString = strings.ReplaceAll(yamlString, CsiPowerstoreEnableChap, chap)
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorNode)
		yamlString = strings.ReplaceAll(yamlString, CsiPowerstoreMaxVolumesPerNode, maxVolumesPerNode)
		yamlString = strings.ReplaceAll(yamlString, PowerStoreCSMNameSpace, cr.Namespace)
		yamlString = strings.ReplaceAll(yamlString, PowerStoreDebug, debug)
		yamlString = strings.ReplaceAll(yamlString, PowerStoreNfsClientPort, nfsClientPort)
		yamlString = strings.ReplaceAll(yamlString, PowerStoreNfsServerPort, nfsServerPort)
		yamlString = strings.ReplaceAll(yamlString, PowerStoreNfsExportDirectory, nfsExportDirectory)
		yamlString = strings.ReplaceAll(yamlString, CSMAuthEnabled, authEnabled)
		yamlString = strings.ReplaceAll(yamlString, PowerStoreAPITimeout, powerstoreAPITimeout)
		yamlString = strings.ReplaceAll(yamlString, PodmonArrayConnectivityTimeout, podmonArrayConnectivityTimeout)
	case "Controller":
		if cr.Spec.Driver.Controller != nil {
			for _, env := range cr.Spec.Driver.Controller.Envs {
				if env.Name == "X_CSI_NFS_ACLS" {
					nfsAcls = env.Value
				}
				if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
					healthMonitorController = env.Value
				}
				if env.Name == "X_CSI_POWERSTORE_EXTERNAL_ACCESS" {
					powerstoreExternalAccess = env.Value
				}
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiNfsAcls, nfsAcls)
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorController)
		yamlString = strings.ReplaceAll(yamlString, CsiPowerstoreExternalAccess, powerstoreExternalAccess)
		yamlString = strings.ReplaceAll(yamlString, PowerStoreCSMNameSpace, cr.Namespace)
		yamlString = strings.ReplaceAll(yamlString, PowerStoreDebug, debug)
		yamlString = strings.ReplaceAll(yamlString, PowerStoreNfsClientPort, nfsClientPort)
		yamlString = strings.ReplaceAll(yamlString, PowerStoreNfsServerPort, nfsServerPort)
		yamlString = strings.ReplaceAll(yamlString, PowerStoreNfsExportDirectory, nfsExportDirectory)
		yamlString = strings.ReplaceAll(yamlString, PowerStoreAPITimeout, powerstoreAPITimeout)
		yamlString = strings.ReplaceAll(yamlString, PodmonArrayConnectivityTimeout, podmonArrayConnectivityTimeout)
	case "CSIDriverSpec":
		if cr.Spec.Driver.CSIDriverSpec != nil && cr.Spec.Driver.CSIDriverSpec.StorageCapacity {
			storageCapacity = "true"
		}
		yamlString = strings.ReplaceAll(yamlString, CsiStorageCapacityEnabled, storageCapacity)
	}
	return yamlString
}

func getApplyCertVolumePowerstore(cr csmv1.ContainerStorageModule) (*acorev1.VolumeApplyConfiguration, error) {
	skipCertValid := true
	certCount := 1

	if cr.Spec.Driver.Common != nil {
		if len(cr.Spec.Driver.Common.Envs) == 0 ||
			(len(cr.Spec.Driver.Common.Envs) == 1 && cr.Spec.Driver.Common.Envs[0].Name != "CERT_SECRET_COUNT") {
			certCount = 0
		}
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION" {
				b, err := strconv.ParseBool(env.Value)
				if err != nil {
					return nil, fmt.Errorf("%s is an invalid value for X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION: %v", env.Value, err)
				}
				skipCertValid = b
			}
			if env.Name == "CERT_SECRET_COUNT" {
				d, err := strconv.ParseInt(env.Value, 0, 8)
				if err != nil {
					return nil, fmt.Errorf("%s is an invalid value for CERT_SECRET_COUNT: %v", env.Value, err)
				}
				certCount = int(d)
			}
		}
	} else {
		skipCertValid = true
		certCount = 0
	}

	name := "certs"
	volume := acorev1.VolumeApplyConfiguration{
		Name: &name,
		VolumeSourceApplyConfiguration: acorev1.VolumeSourceApplyConfiguration{
			Projected: &acorev1.ProjectedVolumeSourceApplyConfiguration{
				Sources: []acorev1.VolumeProjectionApplyConfiguration{},
			},
		},
	}

	if !skipCertValid {
		for i := 0; i < certCount; i++ {
			localname := fmt.Sprintf("%s-certs-%d", cr.Name, i)
			value := fmt.Sprintf("cert-%d", i)
			source := acorev1.SecretProjectionApplyConfiguration{
				LocalObjectReferenceApplyConfiguration: acorev1.LocalObjectReferenceApplyConfiguration{Name: &localname},
				Items: []acorev1.KeyToPathApplyConfiguration{
					{
						Key:  &value,
						Path: &value,
					},
				},
			}
			volume.VolumeSourceApplyConfiguration.Projected.Sources = append(volume.VolumeSourceApplyConfiguration.Projected.Sources, acorev1.VolumeProjectionApplyConfiguration{Secret: &source})

		}
	}

	return &volume, nil
}
