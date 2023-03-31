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

package modules

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	utils "github.com/dell/csm-operator/pkg/utils"
	"sigs.k8s.io/yaml"
)

const (
        // Velero-related
        VeleroNamespace = "<VELERO_NAMESPACE>"

        // Application mobility-related
	// Number of replicas
        AppMobReplicaCount = "<APPLICATION_MOBILITY_REPLICA_COUNT>"
	// Name of license
	AppMobLicenseName = "<APPLICATION_MOBILITY_LICENSE_NAME>"
	// Secret name for object store
	AppMobObjStoreSecretName = "<APPLICATION_MOBILITY_OBJECT_STORE_SECRET_NAME>"
	// AppMobComponent - component name in cr for app-mobility controller-manager
	AppMobCtrlMgrComponent = "application-mobility-controller-manager"
)

// getAppMobilityModule - get instance of app mobility module
func getAppMobilityModule(cr csmv1.ContainerStorageModule) (csmv1.Module, error) {
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.ApplicationMobility {
			return m, nil
		}
	}
	return csmv1.Module{}, fmt.Errorf("Application Mobility module not found")
}

// getAppMobilityModuleDeployment - updates deployment manifest with app mobility CRD values
func getAppMobilityModuleDeployment(op utils.OperatorConfig, cr csmv1.ContainerStorageModule, appMob csmv1.Module) (string, error) {
	YamlString := ""
	auth, err := getAppMobilityModule(cr)
	if err != nil {
		return YamlString, err
	}

	deploymentPath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, AppMobDeploymentManifest)
	buf, err := os.ReadFile(filepath.Clean(deploymentPath))
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	appMobNamespace := cr.Namespace

	for _, component := range appMob.Components {
		if component.Name == AppMobCtrlMgrComponent {
			YamlString = strings.ReplaceAll(YamlString, AppMobReplicaCount, component.ReplicaCount)
			YamlString = strings.ReplaceAll(YamlString, AppMobImagePullPolicy, component.ImagePullPolicy)
			YamlString = strings.ReplaceAll(YamlString, AppMobControllerImage, component.Controller)
			YamlString = strings.ReplaceAll(YamlString, VeleroNamespace, component.VeleroNamespace)
			YamlString = strings.ReplaceAll(YamlString, AppMobObjStoreSecretName, component.ObjectStoreSecretName)
			YamlString = strings.ReplaceAll(YamlString, AppMobLicenseName, component.LicenseName)
		}
	}

	YamlString = strings.ReplaceAll(YamlString, AppMobNamespace, appMobNamespace)

	return YamlString, nil
}

