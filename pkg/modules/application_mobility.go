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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	utils "github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/tests/shared/crclient"
)

const (
	// VeleroNamespace - namespace Velero is installed in
	VeleroNamespace = "<VELERO_NAMESPACE>"

	// AppMobReplicaCount - Number of replicas
	AppMobReplicaCount = "<APPLICATION_MOBILITY_REPLICA_COUNT>"
	// AppMobLicenseName - Name of license for app-mobility
	AppMobLicenseName = "<APPLICATION_MOBILITY_LICENSE_NAME>"
	// AppMobObjStoreSecretName - Secret name for object store
	AppMobObjStoreSecretName = "<APPLICATION_MOBILITY_OBJECT_STORE_SECRET_NAME>"
	// AppMobCtrlMgrComponent - component name in cr for app-mobility controller-manager
	AppMobCtrlMgrComponent = "application-mobility-controller-manager"
	// AppMobDeploymentManifest - filename of deployment manifest for app-mobility
	AppMobDeploymentManifest = "app-mobility-controller-manager.yaml"
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
	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return YamlString, err
	}

	deploymentPath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, AppMobDeploymentManifest)
	buf, err := os.ReadFile(filepath.Clean(deploymentPath))
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	//appMobNamespace := cr.Namespace

	for _, component := range appMob.Components {
		if component.Name == AppMobCtrlMgrComponent {
			YamlString = strings.ReplaceAll(YamlString, AppMobReplicaCount, component.ReplicaCount)
			YamlString = strings.ReplaceAll(YamlString, VeleroNamespace, component.VeleroNamespace)
			YamlString = strings.ReplaceAll(YamlString, AppMobObjStoreSecretName, component.ObjectStoreSecretName)
			YamlString = strings.ReplaceAll(YamlString, AppMobLicenseName, component.LicenseName)
		}
	}

	//YamlString = strings.ReplaceAll(YamlString, AppMobNamespace, appMobNamespace)

	return YamlString, nil
}

func AppMobilityDeployment(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {

	YamlString, err := getAppMobilityModuleDeployment(op, cr, csmv1.Module{})
	if err != nil {
		return err
	}
	deployObjects, err := utils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range deployObjects {
		if isDeleting {
			if err := utils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			} else {
				if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
