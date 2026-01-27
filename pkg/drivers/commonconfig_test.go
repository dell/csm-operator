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

package drivers

import (
	"context"
	"strings"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	shared "github.com/dell/csm-operator/tests/sharedutil"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	csm                  = csmWithTolerations(csmv1.PowerScaleName, shared.ConfigVersion, "")
	csm1                 = csmWithTolerations(csmv1.PowerScaleName, "", shared.InvalidCSMVersion)
	pFlexCSM             = csmForPowerFlex(pflexCSMName)
	pStoreCSM            = csmWithPowerstore(csmv1.PowerStore, shared.PStoreConfigVersion)
	pScaleCSM            = csmWithPowerScale(csmv1.PowerScale, shared.PScaleConfigVersion)
	unityCSM             = csmWithUnity(csmv1.Unity, shared.UnityConfigVersion, false)
	unityCSMCertProvided = csmWithUnity(csmv1.Unity, shared.UnityConfigVersion, true)
	unityCSMInvalidValue = csmWithUnityInvalidValue(csmv1.Unity, shared.UnityConfigVersion)
	pmaxCSM              = csmWithPowermax(csmv1.PowerMax, shared.PmaxConfigVersion)

	fakeDriver csmv1.DriverType = "fakeDriver"
	badDriver  csmv1.DriverType = "badDriver"

	tests = []struct {
		// every single unit test name
		name string
		// csm object
		csm csmv1.ContainerStorageModule
		// driver name
		driverName csmv1.DriverType
		// yaml file name to read
		filename string
		// expected error
		expectedErr string
	}{
		{"pscale happy path", csm, csmv1.PowerScaleName, "node.yaml", ""},
		{"powerscale happy path", pScaleCSM, csmv1.PowerScaleName, "node.yaml", ""},
		{"pflex happy path", pFlexCSM, csmv1.PowerFlex, "node.yaml", ""},
		{"pflex no-sdc path", csmForPowerFlex("no-sdc"), csmv1.PowerFlex, "node.yaml", ""},
		{"pflex with no common section", csmForPowerFlex("no-common-section"), csmv1.PowerFlex, "node.yaml", ""},
		{"pstore happy path", pStoreCSM, csmv1.PowerStore, "node.yaml", ""},
		{"unity happy path", unityCSM, csmv1.Unity, "node.yaml", ""},
		{"unity happy path when secrets with certificates provided", unityCSMCertProvided, csmv1.Unity, "node.yaml", ""},
		{"unity common is nil", unityCSMInvalidValue, csmv1.Unity, "node.yaml", ""},
		{"file does not exist", csm, fakeDriver, "NonExist.yaml", "no such file or directory"},
		{"pmax happy path", pmaxCSM, csmv1.PowerMax, "node.yaml", ""},
		{"pmax common env without node section", csmForPowerMax("common-env-override-no-node"), csmv1.PowerMax, "node.yaml", ""},
		{"config file is invalid", csm, badDriver, "bad.yaml", "unmarshal"},
		{"config file is invalid", csm1, badDriver, "bad.yaml", "No custom resource configuration is available for CSM version v1.10.0"},
	}
)

func TestGetCsiDriver(t *testing.T) {
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csiDriver, err := GetCSIDriver(ctx, tt.csm, config, tt.driverName)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
				if tt.csm.Spec.Driver.CSIDriverSpec != nil {
					switch tt.csm.Spec.Driver.CSIDriverSpec.FSGroupPolicy {
					case "":
						assert.Equal(t, storagev1.ReadWriteOnceWithFSTypeFSGroupPolicy, *csiDriver.Spec.FSGroupPolicy)
					case "ReadWriteOnceWithFSType":
						assert.Equal(t, storagev1.ReadWriteOnceWithFSTypeFSGroupPolicy, *csiDriver.Spec.FSGroupPolicy)
					case "File":
						assert.Equal(t, storagev1.FileFSGroupPolicy, *csiDriver.Spec.FSGroupPolicy)
					default:
						assert.Equal(t, storagev1.NoneFSGroupPolicy, *csiDriver.Spec.FSGroupPolicy)
					}
				}
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

func TestGetConfigMap(t *testing.T) {
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetConfigMap(ctx, tt.csm, config, tt.driverName)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

func TestGetUpgradeInfo(t *testing.T) {
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.csm.Spec.Driver.ConfigVersion != "" {
				_, err := GetUpgradeInfo(ctx, config, tt.driverName, tt.csm.Spec.Driver.ConfigVersion)
				if tt.expectedErr == "" {
					assert.Nil(t, err)
				} else {
					assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
				}
			}
		})
	}
}

func TestGetController(t *testing.T) {
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetController(ctx, tt.csm, config, tt.driverName, operatorutils.VersionSpec{})
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

func TestGetControllerCOSI(t *testing.T) {
	csm := csmForCosi(csmv1.Cosi, map[string]string{
		"node-role.kubernetes.io/worker": "true",
	},
		[]corev1.Toleration{
			{Key: "node-role.kubernetes.io/worker", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
		},
		[]corev1.EnvVar{
			{Name: "COSI_LOG_LEVEL", Value: "info"},
			{Name: "COSI_LOG_FORMAT", Value: "text"},
			{Name: "OTEL_COLLECTOR_ADDRESS", Value: "test:1234"},
		}...)
	_, err := GetController(context.Background(), csm, config, csmv1.Cosi, operatorutils.VersionSpec{})
	assert.Nil(t, err)
}

func TestGetNode(t *testing.T) {
	ctx := context.Background()
	foundInitMdm := false

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := GetNode(ctx, tt.csm, config, tt.driverName, tt.filename, ctrlClientFake.NewClientBuilder().Build(), operatorutils.VersionSpec{})
			if tt.expectedErr == "" {
				assert.Nil(t, err)
				initcontainers := node.DaemonSetApplyConfig.Spec.Template.Spec.InitContainers
				for i := range initcontainers {
					if *initcontainers[i].Name == "mdm-container" {
						foundInitMdm = true
						// if min manifest test case, there will be no common section
						if tt.name != "pflex with no common section" {
							assert.Equal(t, string(tt.csm.Spec.Driver.Common.Image), *initcontainers[i].Image)
						}
					}
				}
				// if driver is powerflex, then check that mdm-container is present
				if tt.driverName == "powerflex" {
					assert.Equal(t, true, foundInitMdm)
				}

				if tt.name == "pmax common env without node section" {
					// expect driver container to have overridden env vars defined under the CSM driver.common section
					foundEnv := false
					for _, c := range node.DaemonSetApplyConfig.Spec.Template.Spec.Containers {
						if *c.Name == "driver" {
							for _, e := range c.Env {
								if e.Name != nil && e.Value != nil && *e.Name == "X_CSI_K8S_CLUSTER_PREFIX" && *e.Value == "UNIT-TEST" {
									foundEnv = true
								}
							}
						}
					}
					assert.Equal(t, true, foundEnv)
				}

			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

func TestIsCSMDREnabled(t *testing.T) {
	tests := []struct {
		name        string
		cr          csmv1.ContainerStorageModule
		expected    string
		expectedErr string
	}{
		{
			name: "X_CSM_DR_ENABLED is true",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{
									Name:  "X_CSM_DR_ENABLED",
									Value: "true",
								},
							},
						},
					},
				},
			},
			expected: "true",
		},
		{
			name: "X_CSM_DR_ENABLED is false",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{
									Name:  "X_CSM_DR_ENABLED",
									Value: "false",
								},
							},
						},
					},
				},
			},
			expected: "false",
		},
		{
			name: "X_CSM_DR_ENABLED is not set",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{},
						},
					},
				},
			},
			expected: "true",
		},
		{
			name: "X_CSM_DR_ENABLED is empty",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{
									Name:  "X_CSM_DR_ENABLED",
									Value: "",
								},
							},
						},
					},
				},
			},
			expected: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCSMDREnabled(tt.cr)
			if tt.expectedErr == "" {
				if result != tt.expected {
					t.Errorf("Expected %s, but got %s", tt.expected, result)
				}
			} else {
				if !strings.Contains(result, tt.expectedErr) {
					t.Errorf("Expected error containing %q, but got %s", tt.expectedErr, result)
				}
			}
		})
	}
}

func TestGetNode_SDCImageFromConfigMap(t *testing.T) {
	ctx := context.Background()

	// Use an existing happy-path PowerFlex CR (ensures SDC init container is present and enabled)
	cr := csmForPowerFlex(pflexCSMName)

	// Build a VersionSpec with an explicit SDC image in the matched images map.
	matched := operatorutils.VersionSpec{
		Images: map[string]string{
			// driver image mapping may be used elsewhere, keep it reasonable
			string(csmv1.PowerFlex): "dellemc/csi-powerflex:vtest",
			// This is the key path under test
			"sdc": "dellemc/sdc:test-tag",
		},
	}

	node, err := GetNode(
		ctx,
		cr,
		config,
		csmv1.PowerFlex,
		"node.yaml",
		ctrlClientFake.NewClientBuilder().Build(),
		matched,
	)
	assert.Nil(t, err)

	// Find the sdc init container and assert the image was picked from matched.Images["sdc"]
	foundSDC := false
	for _, ic := range node.DaemonSetApplyConfig.Spec.Template.Spec.InitContainers {
		if ic.Name != nil && *ic.Name == "sdc" {
			foundSDC = true
			if ic.Image == nil {
				t.Fatalf("sdc init container image is nil")
			}
			assert.Equal(t, "dellemc/sdc:test-tag", *ic.Image, "sdc image should be set from matched.Images")
			break
		}
	}
	assert.True(t, foundSDC, "expected to find sdc init container in PowerFlex node DaemonSet")
}

func TestGetNode_SDCImageFromCustomRegistry(t *testing.T) {
	ctx := context.Background()

	// Start from a PowerFlex CR and set a custom registry.
	cr := csmForPowerFlex(pflexCSMName)
	cr.Spec.CustomRegistry = "registry.company.io/prod"

	// No sdc entry in matched.Images -> forces the custom registry fallback branch.
	matched := operatorutils.VersionSpec{
		Images: map[string]string{
			// Keep other images minimal/realistic. No "sdc" key on purpose.
			string(csmv1.PowerFlex): "dellemc/csi-powerflex:vtest",
		},
	}

	node, err := GetNode(
		ctx,
		cr,
		config,
		csmv1.PowerFlex,
		"node.yaml",
		ctrlClientFake.NewClientBuilder().Build(),
		matched,
	)
	assert.Nil(t, err)

	// Validate sdc init container image came from operatorutils.ResolveImage(...) (i.e., custom registry applied)
	foundSDC := false
	for _, ic := range node.DaemonSetApplyConfig.Spec.Template.Spec.InitContainers {
		if ic.Name != nil && *ic.Name == "sdc" {
			foundSDC = true
			if ic.Image == nil {
				t.Fatalf("sdc init container image is nil")
			}
			// We don't assert exact suffix because ResolveImage composes path from template + registry.
			// Prefix check is robust and sufficient to prove the fallback path executed.
			assert.True(t, strings.HasPrefix(*ic.Image, "registry.company.io/prod/"),
				"expected sdc image to be resolved using custom registry, got %q", *ic.Image)
			break
		}
	}
	assert.True(t, foundSDC, "expected to find sdc init container in PowerFlex node DaemonSet")
}
