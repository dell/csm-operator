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

	csmv1 "github.com/dell/csm-operator/api/v1"
	shared "github.com/dell/csm-operator/tests/sharedutil"
	"github.com/dell/csm-operator/tests/sharedutil/crclient"
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
						AuthSecret: "powermax-config",
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
						AuthSecret: "powermax-config",
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
						AuthSecret: "powermax-config",
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
						AuthSecret: "powermax-config",
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
						AuthSecret: "powermax-config",
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
