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
	// UnityPluginIdentifier - Unity Plugin Identifier
	UnityPluginIdentifier = "unity"

	// UnityConfigParamsVolumeMount - Volume mount
	UnityConfigParamsVolumeMount = "csi-unity-config-params"

	// CsiLogLevel - Defines the log level
	CsiLogLevel = "<CSI_LOG_LEVEL>"

	// AllowRWOMultipodAccess - Defines if multiple pod should have access to a volume
	AllowRWOMultipodAccess = "<X_CSI_UNITY_ALLOW_MULTI_POD_ACCESS>"

	// MaxUnityVolumesPerNode - Max volumes per node
	MaxUnityVolumesPerNode = "<MAX_UNITY_VOLUMES_PER_NODE>"

	// SyncNodeInfoTimeInterval - Interval to sync node info
	SyncNodeInfoTimeInterval = "<X_CSI_UNITY_SYNC_NODEINFO_INTERVAL>"

	// TenantName - Name of the tenant
	TenantName = "<TENANT_NAME>"

	// AllowedNetworks - list of networks that can be used for NFS traffic
	AllowedNetworks = "<X_CSI_ALLOWED_NETWORKS>"

	// UnityCSMNameSpace - namespace CSM is found in. Needed for cases where pod namespace is not namespace of CSM
	UnityCSMNameSpace string = "<CSM_NAMESPACE>"

	// UnityDebug - will be used to control the GOISILON_DEBUG variable
	UnityDebug string = "<GOUNITY_DEBUG>"

	// UnityHTTP - will be used to control the GOUNITY_SHOWHTTP variable
	UnityHTTP string = "<GOUNITY_SHOWHTTP>"
)

// PrecheckUnity do input validation
func PrecheckUnity(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, ct client.Client) error {
	log := logger.GetLogger(ctx)
	// Check for secret only
	config := cr.Name + "-creds"

	// Check if driver version is supported by doing a stat on a config file
	configFilePath := fmt.Sprintf("%s/driverconfig/unity/%s/upgrade-path.yaml", operatorConfig.ConfigDirectory, cr.Spec.Driver.ConfigVersion)
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Errorw("PreCheckUnity failed in version check", "Error", err.Error(), "Namespace", cr.Namespace)
		return fmt.Errorf("%s %s not supported", csmv1.Unity, cr.Spec.Driver.ConfigVersion)
	}

	skipCertValid := true
	certCount := 1
	if cr.Spec.Driver.Common != nil {
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "X_CSI_UNITY_SKIP_CERTIFICATE_VALIDATION" {
				b, err := strconv.ParseBool(env.Value)
				if err != nil {
					return fmt.Errorf("%s is an invalid value for X_CSI_UNITY_SKIP_CERTIFICATE_VALIDATION: %v", env.Value, err)
				}
				skipCertValid = b
			}
			if env.Name == "CERT_SECRET_COUNT" {
				d, err := strconv.ParseInt(env.Value, 0, 8)
				if err != nil {
					return fmt.Errorf("%s is an invalid value for CERT_SECRET_COUNT: %v", env.Value, err)
				}
				certCount = int(d)
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

// ModifyUnityCR - Configuring CR parameters
func ModifyUnityCR(yamlString string, cr csmv1.ContainerStorageModule, fileType string) string {
	// Parameters to initialise CR values
	healthMonitorNode := "false"
	healthMonitorController := "false"
	storageCapacity := "false"
	allowedNetworks := ""
	// GOUNITY_DEBUG defaults to false
	debug := "false"
	// GOUNITY_SHOWHTTP defaults to false
	showHTTP := "false"

	if cr.Spec.Driver.Common != nil {
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "GOUNITY_DEBUG" {
				debug = env.Value
			}
			if env.Name == "GOUNITY_SHOWHTTP" {
				showHTTP = env.Value
			}
		}
	}

	switch fileType {
	case "Node":
		if cr.Spec.Driver.Node != nil {
			for _, env := range cr.Spec.Driver.Node.Envs {
				if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
					healthMonitorNode = env.Value
				}
				if env.Name == "X_CSI_ALLOWED_NETWORKS" {
					allowedNetworks = env.Value
				}
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorNode)
		yamlString = strings.ReplaceAll(yamlString, AllowedNetworks, allowedNetworks)
		yamlString = strings.ReplaceAll(yamlString, UnityCSMNameSpace, cr.Namespace)
		yamlString = strings.ReplaceAll(yamlString, UnityDebug, debug)
		yamlString = strings.ReplaceAll(yamlString, UnityHTTP, showHTTP)
	case "Controller":
		if cr.Spec.Driver.Controller != nil {
			for _, env := range cr.Spec.Driver.Controller.Envs {
				if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
					healthMonitorController = env.Value
				}
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorController)
		yamlString = strings.ReplaceAll(yamlString, UnityCSMNameSpace, cr.Namespace)
		yamlString = strings.ReplaceAll(yamlString, UnityDebug, debug)
		yamlString = strings.ReplaceAll(yamlString, UnityHTTP, showHTTP)
	case "CSIDriverSpec":
		if cr.Spec.Driver.CSIDriverSpec != nil && cr.Spec.Driver.CSIDriverSpec.StorageCapacity {
			storageCapacity = "true"
		}
		yamlString = strings.ReplaceAll(yamlString, CsiStorageCapacityEnabled, storageCapacity)

	}
	return yamlString
}

// ModifyUnityConfigMap - Modify the Configmap parameters
func ModifyUnityConfigMap(_ context.Context, cr csmv1.ContainerStorageModule) map[string]string {
	keyValue := ""
	var configMapData map[string]string
	if cr.Spec.Driver.Common != nil {
		for _, env := range cr.Spec.Driver.Common.Envs {

			if env.Name == "X_CSI_UNITY_ALLOW_MULTI_POD_ACCESS" {
				keyValue += fmt.Sprintf("\n %s: %s", "ALLOW_MULTI_POD_ACCESS", env.Value)
			}
			if env.Name == "MAX_UNITY_VOLUMES_PER_NODE" {
				keyValue += fmt.Sprintf("\n %s: %s", env.Name, env.Value)
			}
			if env.Name == "X_CSI_UNITY_SYNC_NODEINFO_INTERVAL" {
				keyValue += fmt.Sprintf("\n %s: %s", "SYNC_NODE_INFO_TIME_INTERVAL", env.Value)
			}
			if env.Name == "TENANT_NAME" {
				keyValue += fmt.Sprintf("\n %s: %s", env.Name, env.Value)
			}
			if env.Name == "CSI_LOG_LEVEL" {
				keyValue += fmt.Sprintf("\n %s: %s", env.Name, env.Value)
			}
		}
	}
	configMapData = map[string]string{
		"driver-config-params.yaml": keyValue,
	}

	return configMapData
}

func getApplyCertVolumeUnity(cr csmv1.ContainerStorageModule) (*acorev1.VolumeApplyConfiguration, error) {
	skipCertValid := true
	certCount := 1
	if cr.Spec.Driver.Common != nil {
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "X_CSI_UNITY_SKIP_CERTIFICATE_VALIDATION" {
				b, err := strconv.ParseBool(env.Value)
				if err != nil {
					return nil, fmt.Errorf("%s is an invalid value for X_CSI_UNITY_SKIP_CERTIFICATE_VALIDATION: %v", env.Value, err)
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
