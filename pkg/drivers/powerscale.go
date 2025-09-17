//  Copyright © 2021 - 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	// +kubebuilder:scaffold:imports
)

const (
	// PowerScalePluginIdentifier -
	PowerScalePluginIdentifier = "powerscale"

	// PowerScaleConfigParamsVolumeMount -
	PowerScaleConfigParamsVolumeMount = "csi-isilon-config-params" // #nosec G101

	// PowerScaleConfigVolumeMount -
	PowerScaleConfigVolumeMount = "isilon-configs"

	// PowerScaleCSMNameSpace - namespace CSM is found in. Needed for cases where pod namespace is not namespace of CSM
	PowerScaleCSMNameSpace string = "<CSM_NAMESPACE>"

	// PowerScaleDebug - will be used to control the GOISILON_DEBUG variable
	PowerScaleDebug string = "<GOISILON_DEBUG>"

	// PowerScaleCsiVolPrefix - will be used to control the CSI_VOL_PREFIX variable
	PowerScaleCsiVolPrefix string = "<CSI_VOL_PREFIX>"
)

// PrecheckPowerScale do input validation
func PrecheckPowerScale(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, ct client.Client) error {
	log := logger.GetLogger(ctx)
	// Check for secret only
	config := cr.Name + "-creds"

	if cr.Spec.Driver.AuthSecret != "" {
		config = cr.Spec.Driver.AuthSecret
	}

	// Check if driver version is supported by doing a stat on a config file
	configFilePath := fmt.Sprintf("%s/driverconfig/%s/%s/upgrade-path.yaml", operatorConfig.ConfigDirectory, csmv1.PowerScaleName, cr.Spec.Driver.ConfigVersion)
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Errorw("PreCheckPowerScale failed in version check", "Error", err.Error())
		return fmt.Errorf("%s %s not supported", csmv1.PowerScaleName, cr.Spec.Driver.ConfigVersion)
	}

	// check if skip validation is enabled:
	skipCertValid := false
	certCount := 1
	if cr.Spec.Driver.Common != nil {
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION" {
				b, err := strconv.ParseBool(env.Value)
				if err != nil {
					return fmt.Errorf("%s is an invalid value for X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION: %v", env.Value, err)
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

	log.Debugw("preCheck", "skipCertValid", skipCertValid, "certCount", certCount, "secrets", len(secrets))

	if !skipCertValid {
		for i := 0; i < certCount; i++ {
			secrets = append(secrets, fmt.Sprintf("%s-certs-%d", cr.Name, i))
		}
	}

	for _, name := range secrets {
		found := &corev1.Secret{}
		err := ct.Get(ctx, types.NamespacedName{Name: name, Namespace: cr.GetNamespace()}, found)
		if err != nil {
			log.Error(err, "Failed query for secret ", name)
			if errors.IsNotFound(err) {
				return fmt.Errorf("failed to find secret %s", name)
			}
		}
	}

	return nil
}

func getApplyCertVolume(cr csmv1.ContainerStorageModule) (*acorev1.VolumeApplyConfiguration, error) {
	skipCertValid := false
	certCount := 1

	if cr.Spec.Driver.Common != nil {
		if len(cr.Spec.Driver.Common.Envs) == 0 ||
			(len(cr.Spec.Driver.Common.Envs) == 1 && cr.Spec.Driver.Common.Envs[0].Name != "CERT_SECRET_COUNT") {
			certCount = 0
		}

		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION" {
				b, err := strconv.ParseBool(env.Value)
				if err != nil {
					return nil, fmt.Errorf("%s is an invalid value for X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION: %v", env.Value, err)
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
		certCount = 0
		skipCertValid = true
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

// ModifyPowerScaleCR - It modifies the CR of powerscale/isilon (currently for Storage Capacity Tracking
func ModifyPowerScaleCR(yamlString string, cr csmv1.ContainerStorageModule, fileType string) string {
	// Parameters to initialise CR values
	storageCapacity := "false"
	healthMonitorNode := "false"
	healthMonitorController := "false"
	// GOISILON_DEBUG defaults to false
	debug := "false"
	// CSI_VOL_PREFIX defaults to csivol
	csiVolPrefix := "csivol"

	if cr.Spec.Driver.Common != nil {
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "GOISILON_DEBUG" {
				debug = env.Value
			}
		}
	}

	switch fileType {
	case "CSIDriverSpec":
		if cr.Spec.Driver.CSIDriverSpec != nil && cr.Spec.Driver.CSIDriverSpec.StorageCapacity {
			storageCapacity = "true"
		}
		yamlString = strings.ReplaceAll(yamlString, CsiStorageCapacityEnabled, storageCapacity)
	case "Controller":
		if cr.Spec.Driver.Controller != nil {
			for _, env := range cr.Spec.Driver.Controller.Envs {
				if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
					healthMonitorController = env.Value
				}
				if env.Name == "X_CSI_VOL_PREFIX" {
					csiVolPrefix = env.Value
				}
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorController)
		yamlString = strings.ReplaceAll(yamlString, PowerScaleCSMNameSpace, cr.Namespace)
		yamlString = strings.ReplaceAll(yamlString, PowerScaleDebug, debug)
		yamlString = strings.ReplaceAll(yamlString, PowerScaleCsiVolPrefix, csiVolPrefix) // applicable only for v2.14.0/controller.yaml
	case "Node":
		if cr.Spec.Driver.Node != nil {
			for _, env := range cr.Spec.Driver.Node.Envs {
				if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
					healthMonitorNode = env.Value
				}
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorNode)
		yamlString = strings.ReplaceAll(yamlString, PowerScaleCSMNameSpace, cr.Namespace)
		yamlString = strings.ReplaceAll(yamlString, PowerScaleDebug, debug)
	}
	return yamlString
}
