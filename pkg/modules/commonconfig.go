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

package modules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DefaultPluginIdentifier - spring placeholder for driver plugin
	DefaultPluginIdentifier = "<DriverPluginIdentifier>"
	// DefaultDriverConfigParamsVolumeMount - string placeholder for Driver ConfigParamsVolumeMount
	DefaultDriverConfigParamsVolumeMount = "<DriverConfigParamsVolumeMount>"
	// DefaultDriverConfigVolumeMount - string placeholder for Driver ConfigVolumeMount
	DefaultDriverConfigVolumeMount = "<DriverConfigVolumeMount>"
	// CertManagerManifest -
	CertManagerManifest = "cert-manager.yaml"
	// CertManagerCRDsManifest -
	CertManagerCRDsManifest = "cert-manager-crds.yaml"
	// CommonNamespace -
	CommonNamespace = "<NAMESPACE>"
	// CSMName - name
	CSMName = "<NAME>"
	// ComConfigCSMNameSpace - namespace CSM is found in. Needed for cases where pod namespace is not namespace of CSM
	ComConfigCSMNameSpace string = "<CSM_NAMESPACE>"

	// CSMDRCRDsManifest - file name for dr crds
	CSMDRCRDsManifest = "dr-crds.yaml"

	// CertManagerCaInjector - placeholder for cert-manager ca injector
	CertManagerCaInjector = "<CERT_MANAGER_CAINJECTOR_IMAGE>"

	// CertManagerController - placeholder for cert-manager controller
	CertManagerController = "<CERT_MANAGER_CONTROLLER_IMAGE>"

	// CertManagerWebhook - placeholder for cert-manager webhook
	CertManagerWebhook = "<CERT_MANAGER_WEBHOOK_IMAGE>"

	// CertManagerCaInjectorImage - image for cert-manager ca injector
	CertManagerCaInjectorImage = "quay.io/jetstack/cert-manager-cainjector:v1.11.0"

	// CertManagerControllerImage - image for cert-manager controller
	CertManagerControllerImage = "quay.io/jetstack/cert-manager-controller:v1.11.0"

	// CertManagerWebhookImage - image for cert-manager webhook
	CertManagerWebhookImage = "quay.io/jetstack/cert-manager-webhook:v1.11.0"
)

// SupportedDriverParam -
type SupportedDriverParam struct {
	PluginIdentifier              string
	DriverConfigParamsVolumeMount string
	DriverConfigVolumeMount       string
}

func checkVersion(moduleType, givenVersion, configPath string) error {
	files, err := os.ReadDir(fmt.Sprintf("%s/moduleconfig/%s/", configPath, moduleType))
	if err != nil {
		return err
	}
	found := false
	supportedVersions := ""
	for _, file := range files {
		supportedVersions += (file.Name() + ",")
		if file.Name() == givenVersion {
			found = true
		}
	}
	if !found {
		return fmt.Errorf(
			"CSM %s does not have %s version. The following are supported versions: %s",
			moduleType, givenVersion, supportedVersions[:len(supportedVersions)-1],
		)
	}
	return nil
}

func readConfigFile(ctx context.Context, module csmv1.Module, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig, filename string) ([]byte, error) {
	moduleConfigVersion := module.ConfigVersion
	if moduleConfigVersion == "" {
		version, err := operatorutils.GetVersion(ctx, &cr, op)
		if err != nil {
			return nil, err
		}
		// Spec.Version is introduced in 1.16.0 (Auth 2.4)
		// and ConfigVersion needs to populated to support N-2 case
		authAtLeast22, err := operatorutils.MinVersionCheck("v2.2.0", version)
		if err != nil {
			return nil, err
		}
		if authAtLeast22 && module.Name == csmv1.AuthorizationServer {
			moduleConfigVersion = version
			module.ConfigVersion = version
		} else {
			moduleConfigVersion, err = operatorutils.GetModuleDefaultVersion(version, cr.Spec.Driver.CSIDriverType, module.Name, op.ConfigDirectory)
			if err != nil {
				return nil, err
			}
		}
	}

	if module.Name == csmv1.AuthorizationServer {
		configPath := fmt.Sprintf("%s/moduleconfig/%s/%s/%s", op.ConfigDirectory, csmv1.Authorization, moduleConfigVersion, filename)
		return os.ReadFile(filepath.Clean(configPath))
	}

	configMapPath := fmt.Sprintf("%s/moduleconfig/%s/%s/%s", op.ConfigDirectory, module.Name, moduleConfigVersion, filename)
	return os.ReadFile(filepath.Clean(configMapPath))
}

// getCertManager - configure cert-manager with the specified namespace before installation
func getCertManager(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, matched operatorutils.VersionSpec) (string, error) {
	YamlString := ""
	certManagerPath := fmt.Sprintf("%s/moduleconfig/common/cert-manager/%s", op.ConfigDirectory, CertManagerManifest)
	buf, err := os.ReadFile(filepath.Clean(certManagerPath))
	if err != nil {
		return YamlString, err
	}
	YamlString = string(buf)

	if matched.Version != "" {
		placeholders := map[string]string{
			"cert-manager-cainjector": CertManagerCaInjector,
			"cert-manager-controller": CertManagerController,
			"cert-manager-webhook":    CertManagerWebhook,
		}

		for key, placeholder := range placeholders {
			if img := matched.Images[key]; img != "" {
				YamlString = strings.ReplaceAll(YamlString, placeholder, img)
			}
		}

	} else if cr.Spec.CustomRegistry != "" {
		YamlString = strings.ReplaceAll(YamlString, CertManagerCaInjector, operatorutils.ResolveImage(ctx, CertManagerCaInjectorImage, cr))
		YamlString = strings.ReplaceAll(YamlString, CertManagerController, operatorutils.ResolveImage(ctx, CertManagerControllerImage, cr))
		YamlString = strings.ReplaceAll(YamlString, CertManagerWebhook, operatorutils.ResolveImage(ctx, CertManagerWebhookImage, cr))
	} else {
		YamlString = strings.ReplaceAll(YamlString, CertManagerCaInjector, CertManagerCaInjectorImage)
		YamlString = strings.ReplaceAll(YamlString, CertManagerController, CertManagerControllerImage)
		YamlString = strings.ReplaceAll(YamlString, CertManagerWebhook, CertManagerWebhookImage)
	}

	certNamespace := cr.Namespace
	YamlString = strings.ReplaceAll(YamlString, CommonNamespace, certNamespace)
	YamlString = strings.ReplaceAll(YamlString, CSMName, cr.Name)
	YamlString = strings.ReplaceAll(YamlString, ComConfigCSMNameSpace, cr.Namespace)
	return YamlString, nil
}

func getCertManagerCRDs(op operatorutils.OperatorConfig) (string, error) {
	YamlString := ""

	certManagerPath := fmt.Sprintf("%s/moduleconfig/common/cert-manager/%s", op.ConfigDirectory, CertManagerCRDsManifest)
	buf, err := os.ReadFile(filepath.Clean(certManagerPath))
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	return YamlString, nil
}

// CommonCertManager - apply/delete cert-manager objects
func CommonCertManager(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client, matched operatorutils.VersionSpec) error {
	YamlString, err := getCertManager(ctx, op, cr, matched)
	if err != nil {
		return err
	}
	crdYamlString, err := getCertManagerCRDs(op)
	if err != nil {
		return err
	}

	ctrlObjects, err := operatorutils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	crdObjects, err := operatorutils.GetModuleComponentObj([]byte(crdYamlString))
	if err != nil {
		return err
	}

	// keep cert-manager CRDs in place, even if cert-manager is uninstalled
	for _, crdObj := range crdObjects {
		if !isDeleting {
			if err := operatorutils.ApplyObject(ctx, crdObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	for _, ctrlObj := range ctrlObjects {
		if isDeleting {
			if err := operatorutils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := operatorutils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	return nil
}

func getCSMDRCRDs(op operatorutils.OperatorConfig) (string, error) {
	YamlString := ""

	certManagerPath := fmt.Sprintf("%s/moduleconfig/common/disaster-recovery/%s", op.ConfigDirectory, CSMDRCRDsManifest)
	buf, err := os.ReadFile(filepath.Clean(certManagerPath))
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	return YamlString, nil
}

func PatchCSMDRCRDs(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, ctrlClient crclient.Client) error {
	crdYamlString, err := getCSMDRCRDs(op)
	if err != nil {
		return err
	}

	crdObjects, err := operatorutils.GetModuleComponentObj([]byte(crdYamlString))
	if err != nil {
		return err
	}

	for _, crdObj := range crdObjects {
		if isDeleting {
			if err := operatorutils.DeleteObject(ctx, crdObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := operatorutils.ApplyObject(ctx, crdObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	return nil
}

func applyDeleteObjects(ctx context.Context, ctrlClient crclient.Client, yamlString string, isDeleting bool) error {
	ctrlObjects, err := operatorutils.GetModuleComponentObj([]byte(yamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range ctrlObjects {
		if isDeleting {
			if err := operatorutils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := operatorutils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	return nil
}
