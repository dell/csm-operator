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
	"os"
	"path/filepath"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	shared "github.com/dell/csm-operator/tests/sharedutil"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
)

// makeCSIAddonsCR returns a minimal CR for the given driver type using spec.Version from shared
// constants, which includes csi-addons-replication in the version mapping.
// Optional modules can be provided to configure the CR with specific module settings.
func makeCSIAddonsCR(driverType csmv1.DriverType, modules ...csmv1.Module) csmv1.ContainerStorageModule {
	return csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Version: shared.CSMVersion,
			Driver: csmv1.Driver{
				CSIDriverType: driverType,
				Common: &csmv1.ContainerTemplate{
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
			},
			Modules: modules,
		},
	}
}

// makeCSIAddonsDeploymentConfig returns a DeploymentApplyConfiguration with the
// Spec → Template → Spec chain initialized so container append does not panic.
func makeCSIAddonsDeploymentConfig() applyv1.DeploymentApplyConfiguration {
	podSpec := acorev1.PodSpecApplyConfiguration{}
	template := acorev1.PodTemplateSpecApplyConfiguration{Spec: &podSpec}
	spec := applyv1.DeploymentSpecApplyConfiguration{Template: &template}
	return applyv1.DeploymentApplyConfiguration{Spec: &spec}
}

// ---------------------------------------------------------------------------
// TestCSIAddonsReplicationPrecheck
// ---------------------------------------------------------------------------

func TestCSIAddonsReplicationPrecheck(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		cr        csmv1.ContainerStorageModule
		expectErr string
	}{
		{
			name:      "success - powermax driver with no conflicting module",
			cr:        makeCSIAddonsCR(csmv1.PowerMax),
			expectErr: "",
		},
		{
			name:      "success - powerstore driver with no conflicting module",
			cr:        makeCSIAddonsCR(csmv1.PowerStore),
			expectErr: "",
		},
		{
			name: "fail - unsupported driver type",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Version: shared.CSMVersion,
					Driver: csmv1.Driver{
						CSIDriverType: csmv1.PowerScale,
					},
				},
			},
			expectErr: "CSI Addons Replication does not support",
		},
		{
			name: "fail - csm replication module also enabled for powermax",
			cr: makeCSIAddonsCR(csmv1.PowerMax, csmv1.Module{
				Name:    csmv1.Replication,
				Enabled: true,
			}),
			expectErr: "cannot enable both csm replication and csi-addons replication capabilities simultaneously",
		},
		{
			name: "fail - csm replication module also enabled for powerstore",
			cr: makeCSIAddonsCR(csmv1.PowerStore, csmv1.Module{
				Name:    csmv1.Replication,
				Enabled: true,
			}),
			expectErr: "cannot enable both csm replication and csi-addons replication capabilities simultaneously",
		},
		{
			name: "success - csm replication module present but disabled",
			cr: makeCSIAddonsCR(csmv1.PowerMax, csmv1.Module{
				Name:    csmv1.Replication,
				Enabled: false,
			}),
			expectErr: "",
		},
		{
			name: "success - powerstore with csm replication module present but disabled",
			cr: makeCSIAddonsCR(csmv1.PowerStore, csmv1.Module{
				Name:    csmv1.Replication,
				Enabled: false,
			}),
			expectErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CSIAddonsReplicationPrecheck(ctx, &tt.cr)
			if tt.expectErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.expectErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestIsCSIAddonsReplicationEnabledViaEnv
// ---------------------------------------------------------------------------

func TestIsCSIAddonsReplicationEnabledViaEnv(t *testing.T) {
	tests := []struct {
		name     string
		cr       csmv1.ContainerStorageModule
		expected bool
	}{
		{
			name: "true - env var present and set to true",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{Name: XCSIAddonsReplicationEnabled, Value: "true"},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "false - env var present but value is not true",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{Name: XCSIAddonsReplicationEnabled, Value: "false"},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "false - env var not present in common envs",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{Name: "OTHER_ENV", Value: "true"},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "false - common is nil",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: nil,
					},
				},
			},
			expected: false,
		},
		{
			name: "false - common envs is empty",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{},
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCSIAddonsReplicationEnabledViaEnv(tt.cr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// TestCSIAddonsReplicationInjectDeployment
// ---------------------------------------------------------------------------

func TestCSIAddonsReplicationInjectDeployment(t *testing.T) {
	ctx := context.Background()
	powerMaxCR := makeCSIAddonsCR(csmv1.PowerMax)
	powerStoreCR := makeCSIAddonsCR(csmv1.PowerStore)

	tests := []struct {
		name      string
		cr        csmv1.ContainerStorageModule
		op        operatorutils.OperatorConfig
		expectErr bool
	}{
		{
			name:      "success - valid powermax cr and config",
			cr:        powerMaxCR,
			op:        operatorConfig,
			expectErr: false,
		},
		{
			name:      "success - valid powerstore cr and config",
			cr:        powerStoreCR,
			op:        operatorConfig,
			expectErr: false,
		},
		{
			name: "fail - bad config directory causes GetModuleDefaultVersion error",
			cr:   powerMaxCR,
			op: operatorutils.OperatorConfig{
				ConfigDirectory: "bad/path",
			},
			expectErr: true,
		},
		{
			name: "fail - GetVersion error when Spec.Version set and config missing",
			cr: func() csmv1.ContainerStorageModule {
				tmp := makeCSIAddonsCR(csmv1.PowerMax)
				tmp.Spec.Version = "v1.18.0"
				return tmp
			}(),
			op: operatorutils.OperatorConfig{
				ConfigDirectory: "bad/path",
			},
			expectErr: true,
		},
		{
			name: "success - custom registry resolves image override for powermax",
			cr: func() csmv1.ContainerStorageModule {
				tmp := makeCSIAddonsCR(csmv1.PowerMax)
				tmp.Spec.CustomRegistry = "my.registry.io"
				return tmp
			}(),
			op:        operatorConfig,
			expectErr: false,
		},
		{
			name: "success - custom registry resolves image override for powerstore",
			cr: func() csmv1.ContainerStorageModule {
				tmp := makeCSIAddonsCR(csmv1.PowerStore)
				tmp.Spec.CustomRegistry = "my.registry.io"
				return tmp
			}(),
			op:        operatorConfig,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dp := makeCSIAddonsDeploymentConfig()
			result, err := CSIAddonsReplicationInjectDeployment(ctx, dp, tt.cr, tt.op)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				found := false
				for _, c := range result.Spec.Template.Spec.Containers {
					if c.Name != nil && *c.Name == CSIAddonsSideCarName {
						found = true
						break
					}
				}
				assert.True(t, found, "csi-addons container should be injected into deployment")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestCSIAddonsReplicationInjectClusterRole
// ---------------------------------------------------------------------------

func TestCSIAddonsReplicationInjectClusterRole(t *testing.T) {
	ctx := context.Background()
	baseCR := makeCSIAddonsCR(csmv1.PowerMax)
	powerStoreCR := makeCSIAddonsCR(csmv1.PowerStore)

	tests := []struct {
		name        string
		cr          csmv1.ContainerStorageModule
		op          operatorutils.OperatorConfig
		expectErr   bool
		expectRules bool
	}{
		{
			name:        "success - valid powermax cr and config",
			cr:          baseCR,
			op:          operatorConfig,
			expectErr:   false,
			expectRules: true,
		},
		{
			name:        "success - valid powerstore cr and config",
			cr:          powerStoreCR,
			op:          operatorConfig,
			expectErr:   false,
			expectRules: true,
		},
		{
			name: "fail - bad config directory causes GetModuleDefaultVersion error",
			cr:   baseCR,
			op: operatorutils.OperatorConfig{
				ConfigDirectory: "bad/path",
			},
			expectErr:   true,
			expectRules: false,
		},
		{
			name: "fail - GetVersion error when Spec.Version set and config missing",
			cr: func() csmv1.ContainerStorageModule {
				tmp := makeCSIAddonsCR(csmv1.PowerMax)
				tmp.Spec.Version = "v1.17.0"
				return tmp
			}(),
			op: operatorutils.OperatorConfig{
				ConfigDirectory: "bad/path",
			},
			expectErr:   true,
			expectRules: false,
		},
		{
			name: "fail - invalid rules yaml causes unmarshal error",
			cr:   baseCR,
			op: func() operatorutils.OperatorConfig {
				tmpDir, _ := os.MkdirTemp("", "csiaddons-test-*")
				_ = os.MkdirAll(filepath.Join(tmpDir, "common"), 0o755)
				_ = os.MkdirAll(filepath.Join(tmpDir, "moduleconfig", "csi-addons-replication", "v1.0.0"), 0o755)
				csmReleases := shared.CSMVersion + ":\n  powermax:\n    version: v2.18.0\n    modules:\n      csi-addons-replication: v1.0.0\n"
				_ = os.WriteFile(filepath.Join(tmpDir, "common", "csm-releases.yaml"), []byte(csmReleases), 0o644)
				_ = os.WriteFile(filepath.Join(tmpDir, "moduleconfig", "csi-addons-replication", "v1.0.0", "rules.yaml"), []byte("invalid: yaml: ]: bad\n"), 0o644)
				return operatorutils.OperatorConfig{ConfigDirectory: tmpDir}
			}(),
			expectErr:   true,
			expectRules: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusterRole := rbacv1.ClusterRole{}
			result, err := CSIAddonsReplicationInjectClusterRole(ctx, clusterRole, tt.cr, tt.op)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.expectRules {
					assert.Greater(t, len(result.Rules), 0, "expected RBAC rules to be injected")
				}
			}
		})
	}
}
