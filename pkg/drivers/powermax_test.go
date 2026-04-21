//  Copyright © 2023-2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"errors"
	"fmt"
	"testing"

	csmv1 "eos2git.cec.lab.emc.com/CSM/csm-operator/api/v1"
	shared "eos2git.cec.lab.emc.com/CSM/csm-operator/tests/sharedutil"
	"eos2git.cec.lab.emc.com/CSM/csm-operator/tests/sharedutil/crclient"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	powerMaxCSM                   = csmForPowerMax("")
	powerMaxReverseProxySecret    = csmWithReverseProxySecret()
	powerMaxBadReverseProxySecret = csmWithBadReverseProxySecret()
	powerMaxCSMNoProxy            = csmForPowerMaxNOProxy()
	powerMaxCSMBadVersion         = csmForPowerMaxBadVersion()
	powerMaxInvalidCSMVersion     = csmForPowerMaxInvalidVersion()
	powermaxDefaultKubeletPath    = getDefaultKubeletPath()
	powerMaxClient                = crclient.NewFakeClientNoInjector(objects)
	powerMaxSecret                = shared.MakeSecret("csm-creds", "pmax-test", shared.PmaxConfigVersion)
	pMaxfakeSecret                = shared.MakeSecret("fake-creds", "fake-test", shared.PmaxConfigVersion)

	powerMaxTests = []struct {
		// every single unit test name
		name string
		// csm object
		csm csmv1.ContainerStorageModule
		// client
		ct  client.Client
		sec *corev1.Secret
		// expected error
		expectedErr string
	}{
		{"happy path", powerMaxCSM, powerMaxClient, powerMaxSecret, ""},
		{"success: use reverse proxy secret", powerMaxReverseProxySecret, powerMaxClient, powerMaxSecret, ""},
		{"invalid reverse proxy secret, use default", powerMaxBadReverseProxySecret, powerMaxClient, powerMaxSecret, ""},
		{"no proxy set defaults", powerMaxCSMNoProxy, powerMaxClient, powerMaxSecret, ""},
		{"missing secret", powerMaxCSM, powerMaxClient, pMaxfakeSecret, "failed to find secret"},
		{"bad version", powerMaxCSMBadVersion, powerMaxClient, powerMaxSecret, "not supported"},
		{"bad latest version", powermaxDefaultKubeletPath, powerMaxClient, powerMaxSecret, ""},
		{"invalid csm version", powerMaxInvalidCSMVersion, powerMaxClient, powerMaxSecret, "No custom resource configuration is available for CSM version v1.10.0"},
	}
)

func TestPrecheckPowerMax(t *testing.T) {
	ctx := context.Background()
	for _, tt := range powerMaxTests {
		err := tt.ct.Create(ctx, tt.sec)
		if err != nil {
			assert.Nil(t, err)
		}
		t.Run(tt.name, func(t *testing.T) { // #nosec G601 - Run waits for the call to complete.
			err := PrecheckPowerMax(ctx, &tt.csm, config, tt.ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				fmt.Printf("err: %+v\n", err)
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})

		// remove secret after each run
		err = tt.ct.Delete(ctx, tt.sec)
		if err != nil {
			assert.Nil(t, err)
		}
	}
}

func TestDynamicallyMountPowermaxContent(t *testing.T) {
	containerName := "driver"
	volumeName := "myVolume"
	tests := []struct {
		name          string
		configuration interface{}
		cr            csmv1.ContainerStorageModule
		expectedErr   error
	}{
		{
			name: "success: dynamically mount secret for deployment",
			configuration: &v1.DeploymentApplyConfiguration{
				Spec: &v1.DeploymentSpecApplyConfiguration{
					Template: &acorev1.PodTemplateSpecApplyConfiguration{
						Spec: &acorev1.PodSpecApplyConfiguration{
							Volumes: []acorev1.VolumeApplyConfiguration{
								{
									Name: &volumeName,
								},
							},
							Containers: []acorev1.ContainerApplyConfiguration{
								{
									Name: &containerName,
									VolumeMounts: []acorev1.VolumeMountApplyConfiguration{
										{
											Name: &volumeName,
										},
									},
								},
							},
						},
					},
				},
			},
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						AuthSecret: "powermax-config", // #nosec G101
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "true"}},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "success: dynamically mount config content for deployment",
			configuration: &v1.DeploymentApplyConfiguration{
				Spec: &v1.DeploymentSpecApplyConfiguration{
					Template: &acorev1.PodTemplateSpecApplyConfiguration{
						Spec: &acorev1.PodSpecApplyConfiguration{
							Volumes: []acorev1.VolumeApplyConfiguration{
								{
									Name: &volumeName,
								},
							},
							Containers: []acorev1.ContainerApplyConfiguration{
								{
									Name: &containerName,
									VolumeMounts: []acorev1.VolumeMountApplyConfiguration{
										{
											Name: &volumeName,
										},
									},
								},
							},
						},
					},
				},
			},
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						AuthSecret: "powermax-config", // #nosec G101
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "false"}},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "success: dynamically mount secret for daemonset",
			configuration: &v1.DaemonSetApplyConfiguration{
				Spec: &v1.DaemonSetSpecApplyConfiguration{
					Template: &acorev1.PodTemplateSpecApplyConfiguration{
						Spec: &acorev1.PodSpecApplyConfiguration{},
					},
				},
			},
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						AuthSecret: "powermax-config", // #nosec G101
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "true"}},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "success: empty envs",
			configuration: &v1.DeploymentApplyConfiguration{
				Spec: &v1.DeploymentSpecApplyConfiguration{
					Template: &acorev1.PodTemplateSpecApplyConfiguration{
						Spec: &acorev1.PodSpecApplyConfiguration{},
					},
				},
			},
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						AuthSecret: "powermax-config", // #nosec G101
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "error: invalid type passed through",
			configuration: &v1.ReplicaSetApplyConfiguration{
				Spec: &v1.ReplicaSetSpecApplyConfiguration{
					Template: &acorev1.PodTemplateSpecApplyConfiguration{
						Spec: &acorev1.PodSpecApplyConfiguration{},
					},
				},
			},
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						AuthSecret: "powermax-config", // #nosec G101
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "true"}},
						},
					},
				},
			},
			expectedErr: errors.New("invalid type passed through"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DynamicallyMountPowermaxContent(tt.configuration, tt.cr)
			if tt.expectedErr == nil {
				assert.Nil(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedErr.Error())
			}
		})
	}
}

// makes a csm object without proxy
func csmForPowerMaxNOProxy() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "pmax-test", shared.PmaxConfigVersion)

	// Add log level to cover some code in GetConfigMap
	envVarLogLevel1 := corev1.EnvVar{Name: "X_CSI_POWERMAX_PORTGROUPS", Value: "csi_pg"}
	envVarLogLevel2 := corev1.EnvVar{Name: "X_CSI_TRANSPORT_PROTOCOL", Value: "FC"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarLogLevel1, envVarLogLevel2}
	res.Spec.Driver.AuthSecret = "csm-creds"

	// Add pmax driver version
	res.Spec.Driver.ConfigVersion = shared.PmaxConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerMax

	return res
}

func csmWithReverseProxySecret() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "pmax-test", shared.PmaxConfigVersion)

	res.Spec.Driver.Common.Envs = []corev1.EnvVar{
		{Name: "X_CSI_POWERMAX_PORTGROUPS", Value: "csi_pg"},
		{Name: "X_CSI_TRANSPORT_PROTOCOL", Value: "FC"},
		{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "true"},
	}
	res.Spec.Driver.AuthSecret = "csm-creds"

	// Add pmax driver version
	res.Spec.Driver.ConfigVersion = shared.PmaxConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerMax

	revproxy := shared.MakeReverseProxyModule(shared.ConfigVersion)
	revproxy.Components[0].Envs = append(revproxy.Components[0].Envs, corev1.EnvVar{Name: CSIPowerMaxUseSecret, Value: "false"})
	res.Spec.Modules = append(res.Spec.Modules, revproxy)

	return res
}

func csmWithBadReverseProxySecret() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "pmax-test", shared.PmaxConfigVersion)

	res.Spec.Driver.Common.Envs = []corev1.EnvVar{
		{Name: "X_CSI_POWERMAX_PORTGROUPS", Value: "csi_pg"},
		{Name: "X_CSI_TRANSPORT_PROTOCOL", Value: "FC"},
		{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "invalid"},
	}
	res.Spec.Driver.AuthSecret = "csm-creds"

	// Add pmax driver version
	res.Spec.Driver.ConfigVersion = shared.PmaxConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerMax

	return res
}

// makes a csm object with tolerations
func csmForPowerMaxBadVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "pmax-test", shared.PmaxConfigVersion)

	// Add pmax driver version
	res.Spec.Driver.ConfigVersion = "v0"
	res.Spec.Driver.CSIDriverType = csmv1.PowerMax

	return res
}

// makes a csm object with tolerations
func csmForPowerMaxInvalidVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "pmax-test", shared.PmaxConfigVersion)

	// Add pmax driver version
	res.Spec.Version = shared.InvalidCSMVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerMax

	return res
}

func getDefaultKubeletPath() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "pmax-test", shared.PmaxConfigVersion)

	kubeEnv := corev1.EnvVar{Name: "KUBELET_CONFIG_DIR", Value: "/fake"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{kubeEnv}

	return res
}

func TestModifyPowermaxCRDynamicSGParameters(t *testing.T) {
	tests := []struct {
		name              string
		fileType          string
		cr                csmv1.ContainerStorageModule
		expectedDynamicSG string
	}{
		{
			name:              "Node: dynamic SG enabled with sync interval",
			fileType:          "Node",
			cr:                createCSMWithDynamicSGEnvs("true", "120"),
			expectedDynamicSG: "true",
		},
		{
			name:              "Node: dynamic SG disabled",
			fileType:          "Node",
			cr:                createCSMWithDynamicSGEnvs("false", "60"),
			expectedDynamicSG: "false",
		},
		{
			name:              "Controller: dynamic SG enabled",
			fileType:          "Controller",
			cr:                createCSMWithDynamicSGEnvs("true", "90"),
			expectedDynamicSG: "true",
		},
		{
			name:              "Controller: dynamic SG with default values",
			fileType:          "Controller",
			cr:                createCSMWithDynamicSGEnvs("false", ""),
			expectedDynamicSG: "false",
		},
		{
			name:              "Node: missing dynamic SG envs defaults",
			fileType:          "Node",
			cr:                shared.MakeCSM("csm", "pmax-test", shared.PmaxConfigVersion),
			expectedDynamicSG: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlString := CSIPmaxDynamicSGEnabled

			result := ModifyPowermaxCR(yamlString, tt.cr, tt.fileType)

			assert.Containsf(t, result, tt.expectedDynamicSG, "expected dynamic SG value %q in result", tt.expectedDynamicSG)
		})
	}
}

// Helper function to create CSM with dynamic SG environment variables
func createCSMWithDynamicSGEnvs(dynamicSGEnabled string, syncInterval string) csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "pmax-test", shared.PmaxConfigVersion)

	envs := []corev1.EnvVar{
		{Name: "X_CSI_DYNAMIC_SG_ENABLED", Value: dynamicSGEnabled},
	}

	if syncInterval != "" {
		envs = append(envs, corev1.EnvVar{Name: "X_CSI_SG_SYNC_INTERVAL", Value: syncInterval})
	}

	res.Spec.Driver.Common.Envs = envs
	res.Spec.Driver.AuthSecret = "csm-creds"
	res.Spec.Driver.ConfigVersion = shared.PmaxConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerMax

	return res
}

func TestModifyPowermaxCRFsckParameters(t *testing.T) {
	tests := []struct {
		name         string
		fileType     string
		cr           csmv1.ContainerStorageModule
		expectedFsck map[string]string
	}{
		{
			name:     "Node: fsck enabled and mode substituted from Common.Envs",
			fileType: "Node",
			cr:       createCSMWithFsckEnvs("true", "checkAndRepair"),
			expectedFsck: map[string]string{
				"X_CSI_FS_CHECK_ENABLED": "true",
				"X_CSI_FS_CHECK_MODE":    "checkAndRepair",
			},
		},
		{
			name:     "Node: fsck default values when Common.Envs has no fsck entries",
			fileType: "Node",
			cr:       shared.MakeCSM("csm", "pmax-test", shared.PmaxConfigVersion),
			expectedFsck: map[string]string{
				"X_CSI_FS_CHECK_ENABLED": "false",
				"X_CSI_FS_CHECK_MODE":    "checkOnly",
			},
		},
		{
			name:     "Node: fsck disabled with checkOnly mode",
			fileType: "Node",
			cr:       createCSMWithFsckEnvs("false", "checkOnly"),
			expectedFsck: map[string]string{
				"X_CSI_FS_CHECK_ENABLED": "false",
				"X_CSI_FS_CHECK_MODE":    "checkOnly",
			},
		},
		{
			name:     "Controller: fsck placeholders are not substituted",
			fileType: "Controller",
			cr:       createCSMWithFsckEnvs("true", "checkAndRepair"),
			expectedFsck: map[string]string{
				"X_CSI_FS_CHECK_ENABLED": "<X_CSI_FS_CHECK_ENABLED>",
				"X_CSI_FS_CHECK_MODE":    "<X_CSI_FS_CHECK_MODE>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlString := "X_CSI_FS_CHECK_ENABLED=<X_CSI_FS_CHECK_ENABLED> X_CSI_FS_CHECK_MODE=<X_CSI_FS_CHECK_MODE>"

			result := ModifyPowermaxCR(yamlString, tt.cr, tt.fileType)

			for key, expectedValue := range tt.expectedFsck {
				assert.Containsf(t, result, expectedValue, "expected %s value %q in result", key, expectedValue)
			}
		})
	}
}

func TestModifyPowermaxCRSpaceReclamationParameters(t *testing.T) {
	tests := []struct {
		name              string
		fileType          string
		cr                csmv1.ContainerStorageModule
		expectedSpaceRecl map[string]string
	}{
		{
			name:     "Node: space reclamation enabled with schedule substituted from Common.Envs",
			fileType: "Node",
			cr:       createCSMWithSpaceReclamationEnvs("true", "0 2 * * *", "5", "300"),
			expectedSpaceRecl: map[string]string{
				"X_CSI_SPACE_RECLAMATION_ENABLED":        "true",
				"X_CSI_SPACE_RECLAMATION_SCHEDULE":       "0 2 * * *",
				"X_CSI_SPACE_RECLAMATION_MAX_CONCURRENT": "5",
				"X_CSI_SPACE_RECLAMATION_TIMEOUT":        "300",
			},
		},
		{
			name:     "Node: space reclamation default values when Common.Envs has no entries",
			fileType: "Node",
			cr:       shared.MakeCSM("csm", "pmax-test", shared.PmaxConfigVersion),
			expectedSpaceRecl: map[string]string{
				"X_CSI_SPACE_RECLAMATION_ENABLED":        "false",
				"X_CSI_SPACE_RECLAMATION_SCHEDULE":       "",
				"X_CSI_SPACE_RECLAMATION_MAX_CONCURRENT": "",
				"X_CSI_SPACE_RECLAMATION_TIMEOUT":        "",
			},
		},
		{
			name:     "Node: space reclamation disabled with empty parameters",
			fileType: "Node",
			cr:       createCSMWithSpaceReclamationEnvs("false", "", "", ""),
			expectedSpaceRecl: map[string]string{
				"X_CSI_SPACE_RECLAMATION_ENABLED":        "false",
				"X_CSI_SPACE_RECLAMATION_SCHEDULE":       "",
				"X_CSI_SPACE_RECLAMATION_MAX_CONCURRENT": "",
				"X_CSI_SPACE_RECLAMATION_TIMEOUT":        "",
			},
		},
		{
			name:     "Controller: space reclamation placeholders are not substituted",
			fileType: "Controller",
			cr:       createCSMWithSpaceReclamationEnvs("true", "0 2 * * *", "5", "300"),
			expectedSpaceRecl: map[string]string{
				"X_CSI_SPACE_RECLAMATION_ENABLED":        "<X_CSI_SPACE_RECLAMATION_ENABLED>",
				"X_CSI_SPACE_RECLAMATION_SCHEDULE":       "<X_CSI_SPACE_RECLAMATION_SCHEDULE>",
				"X_CSI_SPACE_RECLAMATION_MAX_CONCURRENT": "<X_CSI_SPACE_RECLAMATION_MAX_CONCURRENT>",
				"X_CSI_SPACE_RECLAMATION_TIMEOUT":        "<X_CSI_SPACE_RECLAMATION_TIMEOUT>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlString := "X_CSI_SPACE_RECLAMATION_ENABLED=<X_CSI_SPACE_RECLAMATION_ENABLED> X_CSI_SPACE_RECLAMATION_SCHEDULE=<X_CSI_SPACE_RECLAMATION_SCHEDULE> X_CSI_SPACE_RECLAMATION_MAX_CONCURRENT=<X_CSI_SPACE_RECLAMATION_MAX_CONCURRENT> X_CSI_SPACE_RECLAMATION_TIMEOUT=<X_CSI_SPACE_RECLAMATION_TIMEOUT>"

			result := ModifyPowermaxCR(yamlString, tt.cr, tt.fileType)

			for key, expectedValue := range tt.expectedSpaceRecl {
				assert.Containsf(t, result, expectedValue, "expected %s value %q in result", key, expectedValue)
			}
		})
	}
}

// Helper function to create CSM with fsck environment variables
func createCSMWithFsckEnvs(fsckEnabled string, fsckMode string) csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "pmax-test", shared.PmaxConfigVersion)

	res.Spec.Driver.Common.Envs = []corev1.EnvVar{
		{Name: "X_CSI_FS_CHECK_ENABLED", Value: fsckEnabled},
		{Name: "X_CSI_FS_CHECK_MODE", Value: fsckMode},
	}
	res.Spec.Driver.AuthSecret = "csm-creds"
	res.Spec.Driver.ConfigVersion = shared.PmaxConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerMax

	return res
}

// Helper function to create CSM with SpaceReclamation environment variables
func createCSMWithSpaceReclamationEnvs(spaceReclamationEnabled string, spaceReclamationSchedule string, spaceReclamationMaxConcurrent string, spaceReclamationTimeOut string) csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "pmax-test", shared.PmaxConfigVersion)

	res.Spec.Driver.Common.Envs = []corev1.EnvVar{
		{Name: "X_CSI_SPACE_RECLAMATION_ENABLED", Value: spaceReclamationEnabled},
		{Name: "X_CSI_SPACE_RECLAMATION_SCHEDULE", Value: spaceReclamationSchedule},
		{Name: "X_CSI_SPACE_RECLAMATION_MAX_CONCURRENT", Value: spaceReclamationMaxConcurrent},
		{Name: "X_CSI_SPACE_RECLAMATION_TIMEOUT", Value: spaceReclamationTimeOut},
	}
	res.Spec.Driver.AuthSecret = "csm-creds"
	res.Spec.Driver.ConfigVersion = shared.PmaxConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerMax

	return res
}
