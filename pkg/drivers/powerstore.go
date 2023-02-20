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
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// PowerStorePluginIdentifier -
	PowerStorePluginIdentifier = "powerstore"

	// PowerStoreConfigParamsVolumeMount -
	PowerStoreConfigParamsVolumeMount = "csi-powerstore-config-params"

	// CsiPowerstoreNodeNamePrefix - Node Name Prefix
	CsiPowerstoreNodeNamePrefix = "<X_CSI_POWERSTORE_NODE_NAME_PREFIX>"

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
)

// PrecheckPowerStore do input validation
func PrecheckPowerStore(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, ct client.Client) error {
	log := logger.GetLogger(ctx)
	// Check for secret only
	config := cr.Name + "-config"

	// Check if driver version is supported by doing a stat on a config file
	configFilePath := fmt.Sprintf("%s/driverconfig/powerstore/%s/upgrade-path.yaml", operatorConfig.ConfigDirectory, cr.Spec.Driver.ConfigVersion)
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Errorw("PreCheckPowerStore failed in version check", "Error", err.Error())
		return fmt.Errorf("%s %s not supported", csmv1.PowerStore, cr.Spec.Driver.ConfigVersion)
	}
	secrets := []string{config}

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

	log.Debugw("preCheck", "secrets", len(secrets))
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

	switch fileType {
	case "Node":
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "X_CSI_POWERSTORE_NODE_NAME_PREFIX" {
				nodePrefix = env.Value
			}
			if env.Name == "X_CSI_FC_PORTS_FILTER_FILE_PATH" {
				fcPortFilter = env.Value
			}
		}
		for _, env := range cr.Spec.Driver.Node.Envs {
			if env.Name == "X_CSI_POWERSTORE_ENABLE_CHAP" {
				chap = env.Value
			}
			if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
				healthMonitorNode = env.Value
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiPowerstoreNodeNamePrefix, nodePrefix)
		yamlString = strings.ReplaceAll(yamlString, CsiFcPortFilterFilePath, fcPortFilter)
		yamlString = strings.ReplaceAll(yamlString, CsiPowerstoreEnableChap, chap)
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorNode)
	case "Controller":
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
		yamlString = strings.ReplaceAll(yamlString, CsiNfsAcls, nfsAcls)
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorController)
		yamlString = strings.ReplaceAll(yamlString, CsiPowerstoreExternalAccess, powerstoreExternalAccess)
	case "CSIDriverSpec":
		if cr.Spec.Driver.CSIDriverSpec.StorageCapacity {
			storageCapacity = "true"
		}
		yamlString = strings.ReplaceAll(yamlString, CsiStorageCapacityEnabled, storageCapacity)
	}
	return yamlString
}
