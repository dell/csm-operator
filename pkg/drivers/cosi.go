// Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package drivers

import (
	"context"
	"fmt"
	"os"
	"strings"

	csmv1 "eos2git.cec.lab.emc.com/CSM/csm-operator/api/v1"
	"eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/logger"
	operatorutils "eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/operatorutils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PrecheckCosi for input validation
func PrecheckCosi(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, ct client.Client) error {
	log := logger.GetLogger(ctx)
	secretName := cr.Name + "-config"

	// Check if driver version is supported by doing a stat on a config file
	configFilePath := fmt.Sprintf("%s/driverconfig/%s/%s/upgrade-path.yaml", operatorConfig.ConfigDirectory, csmv1.Cosi, cr.Spec.Driver.ConfigVersion)
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Errorw("PreCheckCOSI failed in version check", "Error", err.Error())
		return fmt.Errorf("%s %s not supported", csmv1.Cosi, cr.Spec.Driver.ConfigVersion)
	}

	log.Debugw("preCheck", "secret", secretName, "Namespace", cr.Namespace)
	_, err := operatorutils.GetSecret(ctx, secretName, cr.GetNamespace(), ct)
	if err != nil {
		return fmt.Errorf("reading secret [%s] error [%s]", secretName, err)
	}

	return nil
}

// ModifyCosiCR
func ModifyCosiCR(yamlString string, cr csmv1.ContainerStorageModule, fileType string) string {
	// TODO
	// nodeSelector and tolerations to be added dynamicly, if added in CSM CR
	log := logger.GetLogger(context.TODO())
	log.Info("Unimplemented")
	switch fileType {
	case "Controller":
		yamlString = strings.ReplaceAll(yamlString, CSMNameSpace, cr.Namespace)
	}
	return yamlString
}
