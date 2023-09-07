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
	// AppMobCrds - name of app-mobility crd manifest yaml
	AppMobCrds = "app-mobility-crds.yaml"
	// VeleroManifest - filename of Velero manifest for app-mobility
	VeleroManifest = "velero-deployment.yaml"
	// AppMobCertManagerManifest - filename of Cert-manager manifest for app-mobility
	AppMobCertManagerManifest = "cert-manager.yaml"
	// ControllerImagePullPolicy - default image pull policy in yamls
	ControllerImagePullPolicy = "<CONTROLLER_IMAGE_PULLPOLICY>"
	//UseVolSnapshotManifest - filename of use volume snapshot manifest for app-mobility
	UseVolSnapshotManifest = "velero-volumesnapshotlocation.yaml"
	// CleanupCrdManifest - filename of Cleanup Crds manifest for app-mobility
	CleanupCrdManifest = "cleanupcrds.yaml"
	// VeleroCrdManifest - filename of Velero crds manisfest for Velero feature
	VeleroCrdManifest = "velero-crds.yaml"
	// VeleroAccessManifest - filename where velero access with its contents
	VeleroAccessManifest = "velero-secret.yaml"
	// ResticCrdManifest - filename of restic manifest for app-mobility
	ResticCrdManifest = "restic.yaml"
	// CertManagerIssuerCertManifest - filename of the issuer and cert for app-mobility
	CertManagerIssuerCertManifest = "certificate.yaml"
	//NodeAgentCrdManifest - filename of node-agent manifest for app-mobility
	NodeAgentCrdManifest = "node-agent.yaml"

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
	//BackupStorageLocation - name for Backup Storage Location
	BackupStorageLocation = "<BACKUPSTORAGELOCATION_NAME>"
	//VeleroBucketName - name for the used velero bucket
	VeleroBucketName = "<BUCKET_NAME>"
	//VolSnapshotlocation - name for Volume Snapshot location
	VolSnapshotlocation = "<VOL_SNAPSHOT_LOCATION_NAME>"
	//BackupStorageURL - cloud url for backup storage location
	BackupStorageURL = "<BACKUP_STORAGE_URL>"

	// VeleroNamespace - namespace Velero is installed in
	VeleroNamespace = "<VELERO_NAMESPACE>"
	// ConfigProvider - configurations provider (csi/aws)
	ConfigProvider = "<CONFIGURATION_PROVIDER>"
	// VeleroImage - Image for velero
	VeleroImage = "<VELERO_IMAGE>"
	// VeleroImagePullPolicy - image pull policy for velero
	VeleroImagePullPolicy = "<VELERO_IMAGE_PULLPOLICY>"
	// VeleroAccess  -  Secret name for velero
	VeleroAccess = "<VELERO_ACCESS>"
	//AWSInitContainerName - Name of init container for velero - aws
	AWSInitContainerName = "<AWS_INIT_CONTAINER_NAME>"
	//AWSInitContainerImage - Image of init container for velero -aws
	AWSInitContainerImage = "<AWS_INIT_CONTAINER_IMAGE>"
	//DELLInitContainerName - Name of init container for velero - dell
	DELLInitContainerName = "<DELL_INIT_CONTAINER_NAME>"
	//DELLInitContainerImage - Image of init container for velero - dell
	DELLInitContainerImage = "<DELL_INIT_CONTAINER_IMAGE>"
	//AccessContents - contents of the object store secret
	AccessContents = "<CRED_CONTENTS>"
	//AKeyID - contains the aws access key id
	AKeyID = "<KEY_ID>"
	//AKey - contains the aws access key
	AKey = "<KEY>"

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

// getVeleroCrdDeploy - applies and deploy VeleroCrd manifest
func getVeleroCrdDeploy(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	yamlString := ""

	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return yamlString, err
	}

	veleroCrdPath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, VeleroCrdManifest)
	buf, err := os.ReadFile(filepath.Clean(veleroCrdPath))
	if err != nil {
		return yamlString, err
	}

	yamlString = string(buf)

	return yamlString, nil
}

// VeleroCrdDeploy - apply and delete Velero crds deployment
func VeleroCrdDeploy(ctx context.Context, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {

	yamlString, err := getVeleroCrdDeploy(op, cr)
	if err != nil {
		return err
	}

	ctrlObjects, err := utils.GetModuleComponentObj([]byte(yamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range ctrlObjects {
		if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
			return err
		}
	}

	return nil
}

// getAppMobCrdDeploy - apply and deploy app mobility crd manifest
func getAppMobCrdDeploy(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	yamlString := ""

	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return yamlString, err
	}

	appMobCrdPath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, AppMobCrds)
	buf, err := os.ReadFile(filepath.Clean(appMobCrdPath))
	if err != nil {
		return yamlString, err
	}

	yamlString = string(buf)

	yamlString = strings.ReplaceAll(yamlString, AppMobNamespace, cr.Namespace)

	return yamlString, nil
}

// AppMobCrdDeploy - apply and delete Velero crds deployment
func AppMobCrdDeploy(ctx context.Context, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {

	yamlString, err := getAppMobCrdDeploy(op, cr)
	if err != nil {
		return err
	}

	ctrlObjects, err := utils.GetModuleComponentObj([]byte(yamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range ctrlObjects {
		if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
			return err
		}
	}

	return nil
}

// getAppMobilityModuleDeployment - updates deployment manifest with app mobility CRD values
func getAppMobilityModuleDeployment(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {

	yamlString := ""
	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return yamlString, err
	}

	deploymentPath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, AppMobDeploymentManifest)
	buf, err := os.ReadFile(filepath.Clean(deploymentPath))
	if err != nil {
		return yamlString, err
	}

	yamlString = string(buf)
	controllerImage := ""
	controllerImagePullPolicy := ""
	licenseName := ""
	replicaCount := ""
	objectSecretName := ""

	for _, component := range appMob.Components {
		if component.Name == AppMobCtrlMgrComponent {
			controllerImage = string(component.Image)
			controllerImagePullPolicy = string(component.ImagePullPolicy)
			for _, env := range component.Envs {
				if strings.Contains(AppMobLicenseName, env.Name) {
					licenseName = env.Value
				}
				if strings.Contains(AppMobReplicaCount, env.Name) {
					replicaCount = env.Value
				}
			}
		}
		if component.Name == AppMobVeleroComponent {
			for _, env := range component.Envs {
				if strings.Contains(AppMobObjStoreSecretName, env.Name) {
					objectSecretName = env.Value
				}
			}
		}
		for _, cred := range component.ComponentCred {
			if cred.Enabled {
				yamlString = strings.ReplaceAll(yamlString, AppMobObjStoreSecretName, cred.Name)
			} else {
				yamlString = strings.ReplaceAll(yamlString, AppMobObjStoreSecretName, objectSecretName)
			}
		}
	}

	yamlString = strings.ReplaceAll(yamlString, AppMobNamespace, cr.Namespace)
	yamlString = strings.ReplaceAll(yamlString, ControllerImg, controllerImage)
	yamlString = strings.ReplaceAll(yamlString, ControllerImagePullPolicy, controllerImagePullPolicy)
	yamlString = strings.ReplaceAll(yamlString, AppMobLicenseName, licenseName)
	yamlString = strings.ReplaceAll(yamlString, AppMobReplicaCount, replicaCount)

	return yamlString, nil
}

// AppMobilityDeployment - apply and delete controller manager deployment
func AppMobilityDeployment(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {

	yamlString, err := getAppMobilityModuleDeployment(op, cr)
	if err != nil {
		return err
	}

	er := applyDeleteObjects(ctx, ctrlClient, yamlString, isDeleting)
	if er != nil {
		return er
	}

	return nil
}

// getControllerManagerMetricService - updates metric manifest with app mobility CRD values
func getControllerManagerMetricService(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	yamlString := ""

	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return yamlString, err
	}

	metricServicePath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, AppMobMetricService)
	buf, err := os.ReadFile(filepath.Clean(metricServicePath))
	if err != nil {
		return yamlString, err
	}

	yamlString = string(buf)
	yamlString = strings.ReplaceAll(yamlString, AppMobNamespace, cr.Namespace)

	return yamlString, nil
}

// ControllerManagerMetricService - apply and delete Controller manager metric service deployment
func ControllerManagerMetricService(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	yamlString, err := getControllerManagerMetricService(op, cr)
	if err != nil {
		return err
	}

	er := applyDeleteObjects(ctx, ctrlClient, yamlString, isDeleting)
	if er != nil {
		return er
	}

	return nil
}

// getAppMobilityWebhookService - gets the app mobility webhook service manifest
func getAppMobilityWebhookService(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	yamlString := ""
	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return yamlString, err
	}

	webhookServicePath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, AppMobWebhookService)
	buf, err := os.ReadFile(filepath.Clean(webhookServicePath))
	if err != nil {
		return yamlString, err
	}

	yamlString = string(buf)
	yamlString = strings.ReplaceAll(yamlString, AppMobNamespace, cr.Namespace)

	return yamlString, nil
}

// AppMobilityWebhookService - apply/delete app mobility's webhook service
func AppMobilityWebhookService(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	yamlString, err := getAppMobilityWebhookService(op, cr)
	if err != nil {
		return err
	}

	er := applyDeleteObjects(ctx, ctrlClient, yamlString, isDeleting)
	if er != nil {
		return er
	}

	return nil
}

// getIssuerCertService - gets the app mobility cert manager's issuer and certificate manifest
func getIssuerCertService(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	yamlString := ""
	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return yamlString, err
	}

	issuerCertServicePath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, CertManagerIssuerCertManifest)
	buf, err := os.ReadFile(filepath.Clean(issuerCertServicePath))
	if err != nil {
		return yamlString, err
	}

	yamlString = string(buf)
	yamlString = strings.ReplaceAll(yamlString, AppMobNamespace, cr.Namespace)

	return yamlString, nil
}

// IssuerCertService() - apply and delete the app mobility issuer and certificate service
func IssuerCertService(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	yamlString, err := getIssuerCertService(op, cr)
	if err != nil {
		return err
	}

	er := applyDeleteObjects(ctx, ctrlClient, yamlString, isDeleting)
	if er != nil {
		return er
	}

	return nil
}

// ApplicationMobilityPrecheck - runs precheck for CSM Application Mobility
func ApplicationMobilityPrecheck(ctx context.Context, op utils.OperatorConfig, appMob csmv1.Module, cr csmv1.ContainerStorageModule, r utils.ReconcileCSM) error {
	log := logger.GetLogger(ctx)

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

	yamlString, err := getCertManager(op, cr)
	if err != nil {
		return err
	}

	er := applyDeleteObjects(ctx, ctrlClient, yamlString, isDeleting)
	if er != nil {
		return er
	}

	return nil
}

// CreateVeleroAccess - Install/Delete velero-secret yaml from operator config
func CreateVeleroAccess(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {

	yamlString, err := getCreateVeleroAccess(op, cr)
	if err != nil {
		return err
	}

	er := applyDeleteObjects(ctx, ctrlClient, yamlString, isDeleting)
	if er != nil {
		return er
	}

	return nil
}

// getCreateVeleroAccess - gets the velero-secret manifest from operatorconfig
func getCreateVeleroAccess(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {

	yamlString := ""

	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return yamlString, err
	}

	veleroAccessPath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, VeleroAccessManifest)
	buf, err := os.ReadFile(filepath.Clean(veleroAccessPath))
	if err != nil {
		return yamlString, err
	}

	yamlString = string(buf)
	veleroNS := ""
	credName := ""
	accessID := ""
	access := ""

	for _, component := range appMob.Components {
		if component.Name == AppMobVeleroComponent {
			for _, env := range component.Envs {
				if strings.Contains(VeleroNamespace, env.Name) {
					veleroNS = env.Value
				}
			}
			for _, cred := range component.ComponentCred {
				if cred.Enabled {
					credName = string(cred.Name)
					accessID = string(cred.SecretContents.AccessKeyID)
					access = string(cred.SecretContents.AccessKey)

				}
			}
		}
	}

	yamlString = strings.ReplaceAll(yamlString, VeleroNamespace, veleroNS)
	yamlString = strings.ReplaceAll(yamlString, VeleroAccess, credName)
	yamlString = strings.ReplaceAll(yamlString, AKeyID, accessID)
	yamlString = strings.ReplaceAll(yamlString, AKey, access)

	return yamlString, nil
}

// AppMobilityVelero - Install/Delete velero along with its features - use volume snapshot location and cleanup crds
func AppMobilityVelero(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {

	var useSnap bool
	var cleanUp bool
	var nodeAgent bool
	credName := ""
	veleroNS := ""

	yamlString, err := getVelero(op, cr)
	if err != nil {
		return err
	}

	er := applyDeleteObjects(ctx, ctrlClient, yamlString, isDeleting)
	if er != nil {
		return er
	}

	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.ApplicationMobility {
			for _, c := range m.Components {
				if c.Name == AppMobVeleroComponent {
					if c.UseSnapshot {
						useSnap = true
					}
					if c.CleanUpCRDs {
						cleanUp = true
					}
					if c.DeployNodeAgent {
						nodeAgent = true
					}
					for _, env := range c.Envs {
						if strings.Contains(AppMobObjStoreSecretName, env.Name) {
							credName = env.Value
						}
						if strings.Contains(VeleroNamespace, env.Name) {
							veleroNS = env.Value
						}
					}
					for _, cred := range c.ComponentCred {
						if cred.Enabled {
							credName = string(cred.Name)
						}
					}
				}
				for _, env := range c.Envs {
					if strings.Contains(AppMobObjStoreSecretName, env.Name) {
						credName = env.Value
					}
				}
				for _, cred := range c.ComponentCred {
					if cred.Enabled {
						credName = string(cred.Name)
					}
				}
			}
		}
	}

	foundCred, err := utils.GetSecret(ctx, credName, veleroNS, ctrlClient)
	if foundCred == nil {
		err := CreateVeleroAccess(ctx, isDeleting, op, cr, ctrlClient)
		if err != nil {
			return fmt.Errorf("unable to deploy velero-secret for Application Mobility: %v", err)
		}
	}

	if useSnap {
		yamlString2, err := getUseVolumeSnapshot(op, cr)
		if err != nil {
			return err
		}

		er := applyDeleteObjects(ctx, ctrlClient, yamlString2, isDeleting)
		if er != nil {
			return er
		}
	}
	if cleanUp {
		yamlString3, err := getCleanupcrds(op, cr)
		if err != nil {
			return err
		}

		er := applyDeleteObjects(ctx, ctrlClient, yamlString3, isDeleting)
		if er != nil {
			return er
		}

	}
	if nodeAgent {
		yamlString4, err := getNodeAgent(op, cr)
		if err != nil {
			return err
		}

		er := applyDeleteObjects(ctx, ctrlClient, yamlString4, isDeleting)
		if er != nil {
			return er
		}
	}
	return nil
}

// getVelero - gets the velero-deployment manifest
func getVelero(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	yamlString := ""

	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return yamlString, err
	}

	veleroPath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, VeleroManifest)
	buf, err := os.ReadFile(filepath.Clean(veleroPath))
	if err != nil {
		return yamlString, err
	}

	yamlString = string(buf)
	backupStorageLocationName := ""
	bucketName := ""
	veleroNS := ""
	provider := ""
	veleroImg := ""
	veleroImgPullPolicy := ""
	veleroAWSInitContainerName := ""
	veleroAWSInitContainerImage := ""
	veleroDELLInitContainerName := ""
	veleroDELLInitContainerImage := ""
	backupURL := ""
	objectSecretName := ""

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
				if strings.Contains(VeleroBucketName, env.Name) {
					bucketName = env.Value
				}
				if strings.Contains(VeleroNamespace, env.Name) {
					veleroNS = env.Value
				}
				if strings.Contains(ConfigProvider, env.Name) {
					provider = env.Value
				}
				if strings.Contains(BackupStorageURL, env.Name) {
					backupURL = env.Value
				}
				if strings.Contains(AppMobObjStoreSecretName, env.Name) {
					objectSecretName = env.Value
				}

			}
			for _, cred := range component.ComponentCred {
				if cred.Enabled {
					yamlString = strings.ReplaceAll(yamlString, AppMobObjStoreSecretName, cred.Name)
				} else {
					yamlString = strings.ReplaceAll(yamlString, AppMobObjStoreSecretName, objectSecretName)
				}
			}
		}
	}
	for _, m := range cr.Spec.Modules {
		for _, icontainer := range m.InitContainer {
			if icontainer.Name == "velero-plugin-for-aws" {
				veleroAWSInitContainerName = icontainer.Name
				veleroAWSInitContainerImage = string(icontainer.Image)
			}
			if icontainer.Name == "dell-custom-velero-plugin" {
				veleroDELLInitContainerName = icontainer.Name
				veleroDELLInitContainerImage = string(icontainer.Image)
			}
		}
	}

	yamlString = strings.ReplaceAll(yamlString, VeleroNamespace, veleroNS)
	yamlString = strings.ReplaceAll(yamlString, VeleroImage, veleroImg)
	yamlString = strings.ReplaceAll(yamlString, VeleroImagePullPolicy, veleroImgPullPolicy)
	yamlString = strings.ReplaceAll(yamlString, AWSInitContainerName, veleroAWSInitContainerName)
	yamlString = strings.ReplaceAll(yamlString, AWSInitContainerImage, veleroAWSInitContainerImage)
	yamlString = strings.ReplaceAll(yamlString, DELLInitContainerName, veleroDELLInitContainerName)
	yamlString = strings.ReplaceAll(yamlString, DELLInitContainerImage, veleroDELLInitContainerImage)
	yamlString = strings.ReplaceAll(yamlString, BackupStorageLocation, backupStorageLocationName)
	yamlString = strings.ReplaceAll(yamlString, VeleroBucketName, bucketName)
	yamlString = strings.ReplaceAll(yamlString, BackupStorageURL, backupURL)
	yamlString = strings.ReplaceAll(yamlString, ConfigProvider, provider)

	return yamlString, nil
}

// getUseVolumeSnapshot - gets the velero - volume snapshot location manifest
func getUseVolumeSnapshot(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	yamlString := ""

	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return yamlString, err
	}

	volSnapshotPath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, UseVolSnapshotManifest)
	buf, err := os.ReadFile(filepath.Clean(volSnapshotPath))
	if err != nil {
		return yamlString, err
	}

	yamlString = string(buf)
	volSnapshotLocationName := ""
	veleroNS := ""
	provider := ""
	backupURL := ""
	for _, component := range appMob.Components {
		if component.Name == AppMobVeleroComponent {
			for _, env := range component.Envs {
				if strings.Contains(VolSnapshotlocation, env.Name) {
					volSnapshotLocationName = env.Value
				}
				if strings.Contains(VeleroNamespace, env.Name) {
					veleroNS = env.Value
				}
				if strings.Contains(ConfigProvider, env.Name) {
					provider = env.Value
				}
				if strings.Contains(BackupStorageURL, env.Name) {
					backupURL = env.Value
				}
			}
		}
	}

	yamlString = strings.ReplaceAll(yamlString, VeleroNamespace, veleroNS)
	yamlString = strings.ReplaceAll(yamlString, VolSnapshotlocation, volSnapshotLocationName)
	yamlString = strings.ReplaceAll(yamlString, ConfigProvider, provider)
	yamlString = strings.ReplaceAll(yamlString, BackupStorageURL, backupURL)

	return yamlString, nil
}

// getCLeanupcrds - gets the clean-up crd manifests
func getCleanupcrds(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	yamlString := ""

	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return yamlString, err
	}

	cleanupCrdsPath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, CleanupCrdManifest)
	buf, err := os.ReadFile(filepath.Clean(cleanupCrdsPath))
	if err != nil {
		return yamlString, err
	}

	yamlString = string(buf)
	veleroNS := ""
	veleroImgPullPolicy := ""
	for _, component := range appMob.Components {
		if component.Name == AppMobVeleroComponent {
			if component.ImagePullPolicy != "" {
				veleroImgPullPolicy = string(component.ImagePullPolicy)
			}
			for _, env := range component.Envs {
				if strings.Contains(VeleroNamespace, env.Name) {
					veleroNS = env.Value
				}
			}
		}
	}

	yamlString = strings.ReplaceAll(yamlString, VeleroNamespace, veleroNS)
	yamlString = strings.ReplaceAll(yamlString, VeleroImagePullPolicy, veleroImgPullPolicy)
	return yamlString, nil
}

// getNodeAgent - gets ndoe-agent services manifests
func getNodeAgent(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	yamlString := ""

	appMob, err := getAppMobilityModule(cr)
	if err != nil {
		return yamlString, err
	}
	cleanupCrdsPath := fmt.Sprintf("%s/moduleconfig/application-mobility/%s/%s", op.ConfigDirectory, appMob.ConfigVersion, NodeAgentCrdManifest)
	buf, err := os.ReadFile(filepath.Clean(cleanupCrdsPath))
	if err != nil {
		return yamlString, err
	}

	yamlString = string(buf)
	veleroNS := ""
	veleroImgPullPolicy := ""
	veleroImg := ""
	objectSecretName := ""

	for _, component := range appMob.Components {
		if component.Name == AppMobVeleroComponent {
			if component.Image != "" {
				veleroImg = string(component.Image)
			}
			if component.ImagePullPolicy != "" {
				veleroImgPullPolicy = string(component.ImagePullPolicy)
			}
			for _, env := range component.Envs {
				if strings.Contains(VeleroNamespace, env.Name) {
					veleroNS = env.Value
				}
				if strings.Contains(AppMobObjStoreSecretName, env.Name) {
					objectSecretName = env.Value
				}

			}
			for _, cred := range component.ComponentCred {
				if cred.Enabled {
					yamlString = strings.ReplaceAll(yamlString, AppMobObjStoreSecretName, cred.Name)
				} else {
					yamlString = strings.ReplaceAll(yamlString, AppMobObjStoreSecretName, objectSecretName)
				}
			}
		}
	}

	yamlString = strings.ReplaceAll(yamlString, VeleroImage, veleroImg)
	yamlString = strings.ReplaceAll(yamlString, VeleroNamespace, veleroNS)
	yamlString = strings.ReplaceAll(yamlString, VeleroImagePullPolicy, veleroImgPullPolicy)
	return yamlString, nil
}

// applyDeleteObjects - Applies/Deletes the object based on boolean value
func applyDeleteObjects(ctx context.Context, ctrlClient crclient.Client, yamlString string, isDeleting bool) error {

	ctrlObjects, err := utils.GetModuleComponentObj([]byte(yamlString))
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
