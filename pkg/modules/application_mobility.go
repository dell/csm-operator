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
	"github.com/dell/csm-operator/pkg/logger"
	utils "github.com/dell/csm-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	//"github.com/dell/csm-operator/tests/shared/crclient"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (

	// AppMobDeploymentManifest - filename of deployment manifest for app-mobility
	AppMobDeploymentManifest = "app-mobility-controller-manager.yaml"
	// AppMobMetricService - filename of MetricService manifest for app-mobility
	AppMobMetricService = "app-mobility-controller-manager-metrics-service.yaml"
	// AppMobWebhookService - filename of Webhook manifest for app-mobility
	AppMobWebhookService = "app-mobility-webhook-service.yaml"
	// VeleroManifest -
	VeleroManifest = "velero-deployment.yaml"
	// AppMobCertManagerManifest -
	AppMobCertManagerManifest = "cert-manager.yaml"

	//ControllerImg - image for app-mobility-controller
	ControllerImg = "<CONTROLLER_IMAGE>"
	// AppMobNamespace - namespace Application Mobility is installed in
	AppMobNamespace = "<NAMESPACE>"
	// AppMobReplicaCount - Number of replicas
	AppMobReplicaCount = "<APPLICATION_MOBILITY_REPLICA_COUNT>"
	// AppMobLicenseName - Name of license for app-mobility
	AppMobLicenseName = "<APPLICATION_MOBILITY_LICENSE_NAME>"
	// AppMobObjStoreSecretName - Secret name for object store
	AppMobObjStoreSecretName = "<APPLICATION_MOBILITY_OBJECT_STORE_SECRET_NAME>"
	//BackupStorageLocation - name for BackupStorageLocation
	BackupStorageLocation = "<BACKUPSTORAGELOCATION_NAME>"
	// VeleroNamespace - namespace Velero is installed in
	VeleroNamespace = "<VELERO_NAMESPACE>"
	// ConfigProvider - configurations provider (csi/aws)
	ConfigProvider = "<CONFIG_PROVIDER>"
	// VeleroImg - Image for velero
	VeleroImg = "<VELERO_IMAGE>"
	// VeleroImgPullPolicy - image pull policy for velero
	VeleroImgPullPolicy = "<VELERO_IMAGE_PULLPOLICY>"
	// VeleroSecretName  -  Secret name for velero
	VeleroSecretName = "<VELERO_SECRET>" // #nosec G101
	//VeleroInitContainers = "<INIT_CONTAINERS>"

	// AppMobCtrlMgrComponent - component name in cr for app-mobility controller-manager
	AppMobCtrlMgrComponent = "application-mobility-controller-manager"
	// AppMobCertManagerComponent - cert-manager component
	AppMobCertManagerComponent = "cert-manager"
	// AppMobVeleroComponent - velero component
	AppMobVeleroComponent = "velero"
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
func getAppMobilityModuleDeployment(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {

	YamlString := ""
	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return YamlString, err
	}

	fmt.Printf("***** INSIDE APPLICATION DEPLOYMENT ******")
	deploymentPath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, AppMobDeploymentManifest)
	buf, err := os.ReadFile(filepath.Clean(deploymentPath))
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	controllerImage := ""
	licenseName := ""
	objectSecretName := ""

	for _, component := range appMob.Components {
		if component.Name == AppMobCtrlMgrComponent {
			if component.Image != "" {
				controllerImage = string(component.Image)
			}
			for _, env := range component.Envs {
				if strings.Contains(AppMobLicenseName, env.Name) {
					licenseName = env.Value
				}
				if strings.Contains(AppMobObjStoreSecretName, env.Name) {
					objectSecretName = env.Value
				}
			}
		}
	}

	YamlString = strings.ReplaceAll(YamlString, AppMobNamespace, cr.Namespace)
	YamlString = strings.ReplaceAll(YamlString, ControllerImg, controllerImage)
	YamlString = strings.ReplaceAll(YamlString, AppMobLicenseName, licenseName)
	YamlString = strings.ReplaceAll(YamlString, AppMobObjStoreSecretName, objectSecretName)

	return YamlString, nil
}

// AppMobilityDeployment - apply and delete controller manager deployment
func AppMobilityDeployment(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	YamlString, err := getAppMobilityModuleDeployment(op, cr)
	if err != nil {
		return err
	}
	fmt.Printf("**** NEED TO RUN DEPLOYMENT****")
	deployObjects, err := utils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range deployObjects {
		if isDeleting {
			if err := utils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			fmt.Printf("**** INSIDE APPLY OBJECT *****")
			if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	return nil
}

// getControllerManagerMetricService - updates metric manifest with app mobility CRD values
func getControllerManagerMetricService(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return YamlString, err
	}

	metricServicePath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, AppMobMetricService)
	buf, err := os.ReadFile(filepath.Clean(metricServicePath))
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	YamlString = strings.ReplaceAll(YamlString, AppMobNamespace, cr.Namespace)

	return YamlString, nil
}

// AppMobilityDeployment - apply and delete Controller manager metric service deployment
func controllerManagerMetricService(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	YamlString, err := getControllerManagerMetricService(op, cr)
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
			}
			if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	return nil
}

// getAppMobilityWebhookService - gets the app mobility webhook service manifest
func getAppMobilityWebhookService(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""
	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return YamlString, err
	}

	webhookServicePath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, AppMobWebhookService)
	buf, err := os.ReadFile(filepath.Clean(webhookServicePath))
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	YamlString = strings.ReplaceAll(YamlString, AppMobNamespace, cr.Namespace)

	return YamlString, nil
}

// AppMobilityWebhookService - apply/delete app mobility's webhook service
func AppMobilityWebhookService(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	YamlString, err := getAppMobilityWebhookService(op, cr)
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
			}
		} else {
			if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	return nil
}

// ApplicationMobilityPrecheck - runs precheck for CSM Application Mobility
func ApplicationMobilityPrecheck(ctx context.Context, op utils.OperatorConfig, appMob csmv1.Module, cr csmv1.ContainerStorageModule, r utils.ReconcileCSM) error {
	log := logger.GetLogger(ctx)

	fmt.Printf("**** GETTING INSIDE PRECHECK*****")
	// check if provided version is supported
	if appMob.ConfigVersion != "" {
		err := checkVersion(string(csmv1.ApplicationMobility), appMob.ConfigVersion, op.ConfigDirectory)
		if err != nil {
			return err
		}
	}

	// Check for secrets
	appMobilitySecrets := []string{"license"}
	for _, name := range appMobilitySecrets {
		found := &corev1.Secret{}
		err := r.GetClient().Get(ctx, types.NamespacedName{Name: name, Namespace: cr.GetNamespace()}, found)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return fmt.Errorf("failed to find secret %s", name)
			}
		}
	}

	log.Infof("performed pre-checks for %s", appMob.Name)
	return nil
}

// AppMobilityCertManager - Install/Delete cert-manager
func AppMobilityCertManager(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {

	YamlString, err := getAppMobCertManager(op, cr)
	if err != nil {
		return err
	}

	ctrlObjects, err := utils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range ctrlObjects {
		if isDeleting {
			if err := utils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}
	return nil
}

// getAppMobilityCertManager - gets the cert-manager manifest from common
func getAppMobCertManager(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	certManagerPath := fmt.Sprintf("%s/moduleconfig/common/%s", op.ConfigDirectory, AppMobCertManagerManifest)
	buf, err := os.ReadFile(filepath.Clean(certManagerPath))
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	appMobNamespace := cr.Namespace
	YamlString = strings.ReplaceAll(YamlString, AppMobNamespace, appMobNamespace)

	return YamlString, nil
}

// AppMobilityVelero - Install/Delete velero
func AppMobilityVelero(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {

	YamlString, err := getVelero(op, cr)
	if err != nil {
		return err
	}

	ctrlObjects, err := utils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range ctrlObjects {
		if isDeleting {
			if err := utils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}
	return nil
}

// getVelero - gets the velero-deployment manifest
func getVelero(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return YamlString, err
	}

	VeleroPath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, VeleroManifest)
	buf, err := os.ReadFile(filepath.Clean(VeleroPath))
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	backupStorageLocationName := ""
	veleroNS := ""
	provider := ""
	veleroImg := ""
	veleroImgPullPolicy := ""
	credName := ""
	for _, component := range appMob.Components {
		if component.Name == AppMobVeleroComponent {
			if component.Image != "" {
				veleroImg = string(component.Image)
			}
			if component.ImagePullPolicy != "" {
				veleroImgPullPolicy = string(component.ImagePullPolicy)
			}
			for _, env := range component.Envs {
				if strings.Contains(BackupStorageLocation, env.Name) {
					backupStorageLocationName = env.Value
				}
				if strings.Contains(VeleroNamespace, env.Name) {
					veleroNS = env.Value
				}
				if strings.Contains(ConfigProvider, env.Name) {
					provider = env.Value
				}
				if strings.Contains(VeleroSecretName, env.Name) {
					credName = env.Value
				}
			}
		}
	}

	YamlString = strings.ReplaceAll(YamlString, VeleroNamespace, veleroNS)
	YamlString = strings.ReplaceAll(YamlString, VeleroImg, veleroImg)
	YamlString = strings.ReplaceAll(YamlString, VeleroImgPullPolicy, veleroImgPullPolicy)
	YamlString = strings.ReplaceAll(YamlString, BackupStorageLocation, backupStorageLocationName)
	YamlString = strings.ReplaceAll(YamlString, ConfigProvider, provider)
	YamlString = strings.ReplaceAll(YamlString, VeleroSecretName, credName)
	return YamlString, nil
}

// IsAppMobilityComponentEnabled - check if Application Mobility componenets are enabled
/*
func IsVolumaeSnapshotEnabled(ctx context.Context, instance csmv1.ContainerStorageModule, r ReconcileCSM, mod csmv1.ModuleType, componentType string) bool {
	appMobilityEnabled, appmobility := IsModuleEnabled(ctx, instance, mod)
	if !appMobilityEnabled {
		return false
	}

	for _, c := range appmobility.Components {
		if c.Name == componentType && *c.Enabled {
			return true
		}
	}

	return false
}

// IsAppMobilityComponentEnabled - check if Application Mobility componenets are enabled
func IsCleanupCRDEnabled(ctx context.Context, instance csmv1.ContainerStorageModule, r ReconcileCSM, mod csmv1.ModuleType, componentType string) bool {
	appMobilityEnabled, appmobility := IsModuleEnabled(ctx, instance, mod)
	if !appMobilityEnabled {
		return false
	}

	for _, c := range appmobility.Components {
		if c.Name == componentType && *c.Enabled {
			return true
		}
	}

	return false
}

// IsAppMobilityComponentEnabled - check if Application Mobility componenets are enabled
func IsdeployResticEnabled(ctx context.Context, instance csmv1.ContainerStorageModule, r ReconcileCSM, mod csmv1.ModuleType, componentType string) bool {
	appMobilityEnabled, appmobility := IsModuleEnabled(ctx, instance, mod)
	if !appMobilityEnabled {
		return false
	}

	for _, c := range appmobility.Components {
		if c.Name == componentType && *c.Enabled {
			return true
		}
	}

	return false
}
*/
