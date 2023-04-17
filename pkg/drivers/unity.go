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
	// UnityPluginIdentifier -
	UnityPluginIdentifier = "unity"

	// UnityConfigParamsVolumeMount -
	UnityConfigParamsVolumeMount = "csi-unity-config-params"

	// CsiUnityNodeNamePrefix - Node Name Prefix
	CsiUnityNodeNamePrefix = "<X_CSI_UNITY_NODENAME_PREFIX>"

	CsiLogLevel            = "<CSI_LOG_LEVEL>"
	AllowRWOMultipodAccess = "<X_CSI_UNITY_ALLOW_MULTI_POD_ACCESS>"

	MaxUnityVolumesPerNode   = "<MAX_UNITY_VOLUMES_PER_NODE>"
	SyncNodeInfoTimeInterval = "<X_CSI_UNITY_SYNC_NODEINFO_INTERVAL>"
	TenantName               = "<TENANT_NAME>"
)

// PrecheckUnity do input validation
func PrecheckUnity(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, ct client.Client) error {
	log := logger.GetLogger(ctx)
	// Check for secret only
	config := cr.Name + "-creds"

	if cr.Spec.Driver.AuthSecret != "" {
		config = cr.Spec.Driver.AuthSecret
	}

	// Check if driver version is supported by doing a stat on a config file
	configFilePath := fmt.Sprintf("%s/driverconfig/unity/%s/upgrade-path.yaml", operatorConfig.ConfigDirectory, cr.Spec.Driver.ConfigVersion)
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Errorw("PreCheckUnity failed in version check", "Error", err.Error())
		return fmt.Errorf("%s %s not supported", csmv1.Unity, cr.Spec.Driver.ConfigVersion)
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

// ModifyUnityCR - Configuring CR parameters
func ModifyUnityCR(yamlString string, cr csmv1.ContainerStorageModule, fileType string) string {
	// Parameters to initialise CR values
	nodePrefix := ""
	healthMonitorNode := ""
	healthMonitorController := ""

	switch fileType {
	case "Node":
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "X_CSI_UNITY_NODENAME_PREFIX" {
				nodePrefix = env.Value
			}
		}
		for _, env := range cr.Spec.Driver.Node.Envs {

			if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
				healthMonitorNode = env.Value
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiUnityNodeNamePrefix, nodePrefix)
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorNode)
	case "Controller":
		for _, env := range cr.Spec.Driver.Controller.Envs {

			if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
				healthMonitorController = env.Value
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorController)

	}
	return yamlString
}

// ModifyUnityConfigMap - Modify the Configmap parameters
func ModifyUnityConfigMap(ctx context.Context, cr csmv1.ContainerStorageModule) map[string]string {
	keyValue := ""
	var configMapData map[string]string
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
	configMapData = map[string]string{
		"driver-config-params.yaml": keyValue,
	}

	return configMapData

}
