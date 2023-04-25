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
	utils "github.com/dell/csm-operator/pkg/utils"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DefaultPluginIdentifier - spring placeholder for driver plugin
	DefaultPluginIdentifier = "<DriverPluginIdentifier>"
	// DefaultDriverConfigParamsVolumeMount - string placeholder for Driver ConfigParamsVolumeMount
	DefaultDriverConfigParamsVolumeMount = "<DriverConfigParamsVolumeMount>"
	// CertManagerManifest -
	CertManagerManifest = "cert-manager.yaml"
)

// SupportedDriverParam -
type SupportedDriverParam struct {
	PluginIdentifier              string
	DriverConfigParamsVolumeMount string
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

func readConfigFile(module csmv1.Module, cr csmv1.ContainerStorageModule, op utils.OperatorConfig, filename string) ([]byte, error) {
	var err error
	moduleConfigVersion := module.ConfigVersion
	if moduleConfigVersion == "" {
		moduleConfigVersion, err = utils.GetModuleDefaultVersion(cr.Spec.Driver.ConfigVersion, cr.Spec.Driver.CSIDriverType, module.Name, op.ConfigDirectory)
		if err != nil {
			return nil, err
		}
	}

	configMapPath := fmt.Sprintf("%s/moduleconfig/%s/%s/%s", op.ConfigDirectory, module.Name, moduleConfigVersion, filename)
	return os.ReadFile(filepath.Clean(configMapPath))
}

// getCertManager - configure cert-manager with the specified namespace before installation
func getCertManager(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	certManagerPath := fmt.Sprintf("%s/moduleconfig/common/cert-manager/%s", op.ConfigDirectory, CertManagerManifest)
	buf, err := os.ReadFile(filepath.Clean(certManagerPath))
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	certNamespace := cr.Namespace
	YamlString = strings.ReplaceAll(YamlString, AuthNamespace, certNamespace)

	return YamlString, nil
}

// CommonCertManager - apply/delete cert-manager objects
func CommonCertManager(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	YamlString, err := getCertManager(op, cr)
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
