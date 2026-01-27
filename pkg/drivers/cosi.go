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
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
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
func ModifyCosiCR(yamlString string, cr csmv1.ContainerStorageModule, fileType string) (string, error) {
	// default values for the envs
	otelCollectorAddr := ""

	// gather the env values from the CR
	if cr.Spec.Driver.Common != nil {
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "OTEL_COLLECTOR_ADDRESS" {
				otelCollectorAddr = env.Value
			}
		}
	}

	// replace the placeholders with actual values
	switch fileType {
	case "Controller":
		yamlString = strings.ReplaceAll(yamlString, CSMNameSpace, cr.Namespace)
		yamlString = strings.ReplaceAll(yamlString, OtelCollectorAddress, otelCollectorAddr)
	}

	// if nodeSelector or tolerations are provided, we need to modify the deployment object
	if cr.Spec.Driver.Common != nil && (len(cr.Spec.Driver.Common.NodeSelector) > 0 || len(cr.Spec.Driver.Common.Tolerations) > 0) {
		objects, err := operatorutils.GetCTRLObject([]byte(yamlString))
		if err != nil {
			return "", fmt.Errorf("parsing controller objects: %v", err)
		}

		var dp *appsv1.Deployment
		for _, obj := range objects {
			if obj, ok := obj.(*appsv1.Deployment); ok {
				dp = obj
				break
			}
		}

		if dp == nil {
			return "", fmt.Errorf("failed to find cosi controller deployment object")
		}

		dp.Spec.Template.Spec.NodeSelector = cr.Spec.Driver.Common.NodeSelector
		dp.Spec.Template.Spec.Tolerations = cr.Spec.Driver.Common.Tolerations

		// Marshal each object back to YAML and join with "---"
		var buf bytes.Buffer
		for i, obj := range objects {
			b, err := yaml.Marshal(obj)
			if err != nil {
				return "", fmt.Errorf("marshalling object %d: %w", i, err)
			}
			if i > 0 {
				buf.WriteString("---\n")
			}
			buf.Write(b)
		}
		return buf.String(), nil
	}
	return yamlString, nil
}
