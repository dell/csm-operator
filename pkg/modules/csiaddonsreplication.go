//  Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	rbacv1 "k8s.io/api/rbac/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/yaml"
)

const (
	// CSIAddonsSideCarName is the name of the csi-addons sidecar container
	CSIAddonsSideCarName = "csi-addons"
)

// XCSIAddonsReplicationEnabled is the env var that controls feature registration
var XCSIAddonsReplicationEnabled = "X_CSI_CSIADDONS_REPLICATION_ENABLED"

// CSIAddonsReplicationSupportedDrivers is a map of drivers that support CSI Addons replication
var CSIAddonsReplicationSupportedDrivers = map[string]bool{
	string(csmv1.PowerMax):   true,
	string(csmv1.PowerStore): true,
}

// getCSIAddonsConfigPath returns the path to a csi-addons config file for the given filename
func getCSIAddonsConfigPath(ctx context.Context, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig, filename string) (string, error) {
	// Get driver version from csm-releases.yaml
	version, err := operatorutils.GetVersion(ctx, &cr, op)
	if err != nil {
		return "", fmt.Errorf("failed to get driver version: %v", err)
	}
	// Get csi-addons version based on driver version
	csiAddonsVersion, err := operatorutils.GetModuleDefaultVersion(version, cr.Spec.Driver.CSIDriverType, csmv1.CSIAddonsReplication, op.ConfigDirectory)
	if err != nil {
		return "", fmt.Errorf("failed to get csi-addons version for driver version %s: %v", version, err)
	}

	return filepath.Join(op.ConfigDirectory, "moduleconfig", "csi-addons-replication", csiAddonsVersion, filename), nil
}

func getCSIAddonsContainer(ctx context.Context, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig) (*acorev1.ContainerApplyConfiguration, error) {
	// Get the container config path
	configPath, err := getCSIAddonsConfigPath(ctx, cr, op, "container.yaml")
	if err != nil {
		return nil, err
	}
	buf, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read csi-addons container config: %v", err)
	}

	YamlString := operatorutils.ModifyCommonCR(string(buf), cr)

	var container acorev1.ContainerApplyConfiguration
	err = yaml.Unmarshal([]byte(YamlString), &container)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal csi-addons container config: %v", err)
	}

	// Apply image override if custom registry is specified
	if cr.Spec.CustomRegistry != "" {
		if container.Image != nil {
			resolvedImage := operatorutils.ResolveImage(ctx, *container.Image, cr)
			container.Image = &resolvedImage
		}
	}

	return &container, nil
}

// CSIAddonsReplicationInjectDeployment injects the csi-addons sidecar into the controller deployment
func CSIAddonsReplicationInjectDeployment(ctx context.Context, dp applyv1.DeploymentApplyConfiguration, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig) (*applyv1.DeploymentApplyConfiguration, error) {
	container, err := getCSIAddonsContainer(ctx, cr, op)
	if err != nil {
		return nil, err
	}
	dp.Spec.Template.Spec.Containers = append(dp.Spec.Template.Spec.Containers, *container)
	return &dp, nil
}

// CSIAddonsReplicationInjectClusterRole injects CSI Addons RBAC rules into the controller ClusterRole
func CSIAddonsReplicationInjectClusterRole(ctx context.Context, clusterRole rbacv1.ClusterRole, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig) (*rbacv1.ClusterRole, error) {
	// Get the rules config path
	configPath, err := getCSIAddonsConfigPath(ctx, cr, op, "rules.yaml")
	if err != nil {
		return nil, err
	}

	buf, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read csi-addons rules config: %v", err)
	}

	var rules []rbacv1.PolicyRule
	err = yaml.Unmarshal(buf, &rules)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal csi-addons rules config: %v", err)
	}

	clusterRole.Rules = append(clusterRole.Rules, rules...)
	return &clusterRole, nil
}

// CSIAddonsReplicationPrecheck runs precheck for CSI Addons Replication when enabled via ENV var
func CSIAddonsReplicationPrecheck(ctx context.Context, cr *csmv1.ContainerStorageModule) error {
	log := logger.GetLogger(ctx)

	if !CSIAddonsReplicationSupportedDrivers[string(cr.Spec.Driver.CSIDriverType)] {
		return fmt.Errorf("CSI Addons Replication does not support %s driver", string(cr.Spec.Driver.CSIDriverType))
	}

	// Check if replication module is also enabled (can't have both)
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Replication && m.Enabled {
			return fmt.Errorf("cannot enable both csm replication and csi-addons replication capabilities simultaneously")
		}
	}

	log.Infof("performed pre checks for: csi-addons-replication (enabled via environment variable)")
	return nil
}

// IsCSIAddonsReplicationEnabledViaEnv checks if CSI Addons Replication is enabled via environment variable
func IsCSIAddonsReplicationEnabledViaEnv(cr csmv1.ContainerStorageModule) bool {
	if cr.Spec.Driver.Common != nil {
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == XCSIAddonsReplicationEnabled && env.Value == "true" {
				return true
			}
		}
	}
	return false
}
