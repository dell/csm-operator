//  Copyright © 2021 - 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/utils"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// PowerStorePluginIdentifier -
	PowerStorePluginIdentifier = "powerstore"

	// PowerStoreConfigParamsVolumeMount -
	PowerStoreConfigParamsVolumeMount = "csi-powerstore-config-params"
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

	log.Debugw("preCheck", "secrets", len(secrets))
	return nil
}
