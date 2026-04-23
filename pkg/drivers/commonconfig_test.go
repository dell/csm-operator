//  Copyright © 2022-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

	csmv1 "eos2git.cec.lab.emc.com/CSM/csm-operator/api/v1"
	operatorutils "eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/operatorutils"
	shared "eos2git.cec.lab.emc.com/CSM/csm-operator/tests/sharedutil"
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
			// Use configForVersionChecks for invalid CSM version test
			cfg := config
			if tt.csm.Spec.Version == shared.InvalidCSMVersion {
				cfg = configForVersionChecks
			}
			csiDriver, err := GetCSIDriver(ctx, tt.csm, cfg, tt.driverName)
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
			// Use configForVersionChecks for invalid CSM version test
			cfg := config
			if tt.csm.Spec.Version == shared.InvalidCSMVersion {
				cfg = configForVersionChecks
			}
			_, err := GetConfigMap(ctx, tt.csm, cfg, tt.driverName)
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
				// Use configForVersionChecks for invalid CSM version test
				cfg := config
				if tt.csm.Spec.Version == shared.InvalidCSMVersion {
					cfg = configForVersionChecks
				}
				_, err := GetUpgradeInfo(ctx, cfg, tt.driverName, tt.csm.Spec.Driver.ConfigVersion)
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
			// Use configForVersionChecks for invalid CSM version test
			cfg := config
			if tt.csm.Spec.Version == shared.InvalidCSMVersion {
				cfg = configForVersionChecks
			}
			_, err := GetController(ctx, tt.csm, cfg, tt.driverName, operatorutils.VersionSpec{})
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
			// Use configForVersionChecks for invalid CSM version test
			cfg := config
			if tt.csm.Spec.Version == shared.InvalidCSMVersion {
				cfg = configForVersionChecks
			}
			node, err := GetNode(ctx, tt.csm, cfg, tt.driverName, tt.filename, ctrlClientFake.NewClientBuilder().Build(), operatorutils.VersionSpec{})
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
			result := GetDriverCommonEnv(tt.cr, "X_CSM_DR_ENABLED", "true")
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

func TestGetCSMDRBindPort(t *testing.T) {
	tests := []struct {
		name        string
		cr          csmv1.ContainerStorageModule
		expected    string
		expectedErr string
	}{
		{
			name: "X_CSM_DR_BIND_PORT is set to custom port",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{
									Name:  "X_CSM_DR_BIND_PORT",
									Value: "9000",
								},
							},
						},
					},
				},
			},
			expected: "9000",
		},
		{
			name: "X_CSM_DR_BIND_PORT is set to port without colon",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{
									Name:  "X_CSM_DR_BIND_PORT",
									Value: "8080",
								},
							},
						},
					},
				},
			},
			expected: "8080",
		},
		{
			name: "X_CSM_DR_BIND_PORT is not set",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{},
						},
					},
				},
			},
			expected: "8082",
		},
		{
			name: "X_CSM_DR_BIND_PORT is empty",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{
									Name:  "X_CSM_DR_BIND_PORT",
									Value: "",
								},
							},
						},
					},
				},
			},
			expected: "8082",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetDriverCommonEnv(tt.cr, "X_CSM_DR_BIND_PORT", "8082")
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

func TestGetNode_SDCConfigMapWinsOverCustomRegistry(t *testing.T) {
	ctx := context.Background()

	cr := csmForPowerFlex(pflexCSMName)
	cr.Spec.CustomRegistry = "my-registry.example.com"

	configMapSDCImage := "configmap-registry.example/sdc:from-cm"
	matched := operatorutils.VersionSpec{
		Version: "v1.0",
		Images: map[string]string{
			string(csmv1.PowerFlex): "dellemc/csi-powerflex:vtest",
			"sdc":                   configMapSDCImage,
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

	foundSDC := false
	for _, ic := range node.DaemonSetApplyConfig.Spec.Template.Spec.InitContainers {
		if ic.Name != nil && *ic.Name == "sdc" {
			foundSDC = true
			if ic.Image == nil {
				t.Fatalf("sdc init container image is nil")
			}
			assert.Equal(t, configMapSDCImage, *ic.Image,
				"ConfigMap SDC image should win over custom registry")
			break
		}
	}
	assert.True(t, foundSDC, "expected to find sdc init container")
}

func TestGetNode_SDCNeitherConfigMapNorRegistry(t *testing.T) {
	ctx := context.Background()

	cr := csmForPowerFlex(pflexCSMName)
	// No custom registry set

	matched := operatorutils.VersionSpec{
		Images: map[string]string{
			// No "sdc" key - empty matched
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

	foundSDC := false
	for _, ic := range node.DaemonSetApplyConfig.Spec.Template.Spec.InitContainers {
		if ic.Name != nil && *ic.Name == "sdc" {
			foundSDC = true
			if ic.Image == nil {
				t.Fatalf("sdc init container image is nil")
			}
			// Should be the template default (not empty, not custom registry)
			assert.NotEmpty(t, *ic.Image, "SDC image should not be empty")
			assert.NotContains(t, *ic.Image, "my-registry", "should not contain custom registry prefix")
			break
		}
	}
	assert.True(t, foundSDC, "expected to find sdc init container")
}

func TestGetController_DriverImageFromConfigMap(t *testing.T) {
	ctx := context.Background()

	cr := csmForPowerFlex(pflexCSMName)

	configMapDriverImage := "configmap-registry.example/csi-powerflex:from-cm"
	matched := operatorutils.VersionSpec{
		Version: "v1.0",
		Images: map[string]string{
			string(csmv1.PowerFlex): configMapDriverImage,
		},
	}

	controller, err := GetController(
		ctx,
		cr,
		config,
		csmv1.PowerFlex,
		matched,
	)
	assert.Nil(t, err)

	foundDriver := false
	for _, c := range controller.Deployment.Spec.Template.Spec.Containers {
		if c.Name != nil && *c.Name == "driver" {
			foundDriver = true
			if c.Image == nil {
				t.Fatalf("driver container image is nil")
			}
			assert.Equal(t, configMapDriverImage, *c.Image,
				"driver image should be set from ConfigMap")
			break
		}
	}
	assert.True(t, foundDriver, "expected to find driver container in controller deployment")
}

func TestGetController_DriverImageConfigMapWinsOverCustomRegistry(t *testing.T) {
	ctx := context.Background()

	cr := csmForPowerFlex(pflexCSMName)
	cr.Spec.CustomRegistry = "my-registry.example.com"

	configMapDriverImage := "configmap-registry.example/csi-powerflex:from-cm"
	matched := operatorutils.VersionSpec{
		Version: "v1.0",
		Images: map[string]string{
			string(csmv1.PowerFlex): configMapDriverImage,
		},
	}

	controller, err := GetController(
		ctx,
		cr,
		config,
		csmv1.PowerFlex,
		matched,
	)
	assert.Nil(t, err)

	foundDriver := false
	for _, c := range controller.Deployment.Spec.Template.Spec.Containers {
		if c.Name != nil && *c.Name == "driver" {
			foundDriver = true
			if c.Image == nil {
				t.Fatalf("driver container image is nil")
			}
			assert.Equal(t, configMapDriverImage, *c.Image,
				"ConfigMap driver image should win over custom registry")
			break
		}
	}
	assert.True(t, foundDriver, "expected to find driver container in controller deployment")
}

func TestGetController_DriverImageCustomRegistryOnly(t *testing.T) {
	ctx := context.Background()

	cr := csmForPowerFlex(pflexCSMName)
	cr.Spec.CustomRegistry = "my-registry.example.com"

	matched := operatorutils.VersionSpec{} // no ConfigMap

	controller, err := GetController(
		ctx,
		cr,
		config,
		csmv1.PowerFlex,
		matched,
	)
	assert.Nil(t, err)

	foundDriver := false
	for _, c := range controller.Deployment.Spec.Template.Spec.Containers {
		if c.Name != nil && *c.Name == "driver" {
			foundDriver = true
			if c.Image == nil {
				t.Fatalf("driver container image is nil")
			}
			assert.True(t, strings.HasPrefix(*c.Image, "my-registry.example.com/"),
				"driver image should use custom registry, got %q", *c.Image)
			break
		}
	}
	assert.True(t, foundDriver, "expected to find driver container in controller deployment")
}

func TestGetController_DriverImageCustomRegistryRetainPath(t *testing.T) {
	ctx := context.Background()

	cr := csmForPowerFlex(pflexCSMName)
	cr.Spec.CustomRegistry = "my-registry.example.com"
	cr.Spec.RetainImageRegistryPath = true

	matched := operatorutils.VersionSpec{} // no ConfigMap

	controller, err := GetController(
		ctx,
		cr,
		config,
		csmv1.PowerFlex,
		matched,
	)
	assert.Nil(t, err)

	foundDriver := false
	for _, c := range controller.Deployment.Spec.Template.Spec.Containers {
		if c.Name != nil && *c.Name == "driver" {
			foundDriver = true
			if c.Image == nil {
				t.Fatalf("driver container image is nil")
			}
			// retainImageRegistryPath=true should keep the org/repo path
			// template default: quay.io/dell/container-storage-modules/csi-vxflexos:v2.17.0
			// expected: my-registry.example.com/dell/container-storage-modules/csi-vxflexos:v2.17.0
			assert.True(t, strings.HasPrefix(*c.Image, "my-registry.example.com/dell/container-storage-modules/"),
				"driver image should retain repository path, got %q", *c.Image)
			assert.True(t, strings.Contains(*c.Image, "csi-vxflexos:"),
				"driver image should contain the image name, got %q", *c.Image)
			break
		}
	}
	assert.True(t, foundDriver, "expected to find driver container in controller deployment")
}

func TestGetController_SidecarImageCustomRegistryRetainPath(t *testing.T) {
	ctx := context.Background()

	cr := csmForPowerFlex(pflexCSMName)
	cr.Spec.CustomRegistry = "my-registry.example.com"
	cr.Spec.RetainImageRegistryPath = true

	// Need K8sVersion populated so ReplaceAllContainerImageApply sets real sidecar images
	configWithK8s := operatorutils.OperatorConfig{
		ConfigDirectory: config.ConfigDirectory,
		K8sVersion: operatorutils.K8sImagesConfig{
			Images: struct {
				Attacher              string `json:"attacher" yaml:"attacher"`
				Provisioner           string `json:"provisioner" yaml:"provisioner"`
				Snapshotter           string `json:"snapshotter" yaml:"snapshotter"`
				Registrar             string `json:"registrar" yaml:"registrar"`
				Resizer               string `json:"resizer" yaml:"resizer"`
				Externalhealthmonitor string `json:"externalhealthmonitorcontroller" yaml:"externalhealthmonitorcontroller"`
				Sdc                   string `json:"sdc" yaml:"sdc"`
				Sdcmonitor            string `json:"sdcmonitor" yaml:"sdcmonitor"`
				Podmon                string `json:"podmon" yaml:"podmon"`
				CSIRevProxy           string `json:"csiReverseProxy" yaml:"csiReverseProxy"`
			}{
				Provisioner: "registry.k8s.io/sig-storage/csi-provisioner:v6.1.0",
				Attacher:    "registry.k8s.io/sig-storage/csi-attacher:v4.10.0",
				Snapshotter: "registry.k8s.io/sig-storage/csi-snapshotter:v8.4.0",
				Resizer:     "registry.k8s.io/sig-storage/csi-resizer:v2.0.0",
				Registrar:   "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.15.0",
			},
		},
	}

	matched := operatorutils.VersionSpec{} // no ConfigMap

	controller, err := GetController(
		ctx,
		cr,
		configWithK8s,
		csmv1.PowerFlex,
		matched,
	)
	assert.Nil(t, err)

	foundProvisioner := false
	for _, c := range controller.Deployment.Spec.Template.Spec.Containers {
		if c.Name != nil && *c.Name == "provisioner" {
			foundProvisioner = true
			if c.Image == nil {
				t.Fatalf("provisioner container image is nil")
			}
			// retainImageRegistryPath=true should keep the sig-storage path
			// template default: registry.k8s.io/sig-storage/csi-provisioner:v6.1.0
			// expected: my-registry.example.com/sig-storage/csi-provisioner:v6.1.0
			assert.True(t, strings.HasPrefix(*c.Image, "my-registry.example.com/sig-storage/"),
				"provisioner image should retain repository path, got %q", *c.Image)
			assert.True(t, strings.Contains(*c.Image, "csi-provisioner:"),
				"provisioner image should contain the image name, got %q", *c.Image)
			break
		}
	}
	assert.True(t, foundProvisioner, "expected to find provisioner container in controller deployment")
}

func TestGetNode_DriverImageCustomRegistryRetainPath(t *testing.T) {
	ctx := context.Background()

	cr := csmForPowerFlex(pflexCSMName)
	cr.Spec.CustomRegistry = "my-registry.example.com"
	cr.Spec.RetainImageRegistryPath = true

	matched := operatorutils.VersionSpec{} // no ConfigMap

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

	foundDriver := false
	for _, c := range node.DaemonSetApplyConfig.Spec.Template.Spec.Containers {
		if c.Name != nil && *c.Name == "driver" {
			foundDriver = true
			if c.Image == nil {
				t.Fatalf("driver container image is nil")
			}
			// retainImageRegistryPath=true should keep the org/repo path
			// template default: quay.io/dell/container-storage-modules/csi-vxflexos:v2.17.0
			// expected: my-registry.example.com/dell/container-storage-modules/csi-vxflexos:v2.17.0
			assert.True(t, strings.HasPrefix(*c.Image, "my-registry.example.com/dell/container-storage-modules/"),
				"node driver image should retain repository path, got %q", *c.Image)
			assert.True(t, strings.Contains(*c.Image, "csi-vxflexos:"),
				"node driver image should contain the image name, got %q", *c.Image)
			break
		}
	}
	assert.True(t, foundDriver, "expected to find driver container in node daemonset")
}

func TestGetNode_SDCImageCustomRegistryRetainPath(t *testing.T) {
	ctx := context.Background()

	cr := csmForPowerFlex(pflexCSMName)
	cr.Spec.CustomRegistry = "my-registry.example.com"
	cr.Spec.RetainImageRegistryPath = true

	// Need K8sVersion populated so ReplaceAllContainerImageApply sets real init container images
	configWithK8s := operatorutils.OperatorConfig{
		ConfigDirectory: config.ConfigDirectory,
		K8sVersion: operatorutils.K8sImagesConfig{
			Images: struct {
				Attacher              string `json:"attacher" yaml:"attacher"`
				Provisioner           string `json:"provisioner" yaml:"provisioner"`
				Snapshotter           string `json:"snapshotter" yaml:"snapshotter"`
				Registrar             string `json:"registrar" yaml:"registrar"`
				Resizer               string `json:"resizer" yaml:"resizer"`
				Externalhealthmonitor string `json:"externalhealthmonitorcontroller" yaml:"externalhealthmonitorcontroller"`
				Sdc                   string `json:"sdc" yaml:"sdc"`
				Sdcmonitor            string `json:"sdcmonitor" yaml:"sdcmonitor"`
				Podmon                string `json:"podmon" yaml:"podmon"`
				CSIRevProxy           string `json:"csiReverseProxy" yaml:"csiReverseProxy"`
			}{
				Provisioner: "registry.k8s.io/sig-storage/csi-provisioner:v6.1.0",
				Registrar:   "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.15.0",
				Sdcmonitor:  "quay.io/dell/storage/powerflex/sdc:5.0",
			},
		},
	}

	matched := operatorutils.VersionSpec{} // no ConfigMap

	node, err := GetNode(
		ctx,
		cr,
		configWithK8s,
		csmv1.PowerFlex,
		"node.yaml",
		ctrlClientFake.NewClientBuilder().Build(),
		matched,
	)
	assert.Nil(t, err)

	foundSDC := false
	for _, ic := range node.DaemonSetApplyConfig.Spec.Template.Spec.InitContainers {
		if ic.Name != nil && *ic.Name == "sdc" {
			foundSDC = true
			if ic.Image == nil {
				t.Fatalf("sdc init container image is nil")
			}
			// retainImageRegistryPath=true should keep the org path
			// template default: quay.io/dell/storage/powerflex/sdc:5.0
			// expected: my-registry.example.com/dell/storage/powerflex/sdc:5.0
			assert.True(t, strings.HasPrefix(*ic.Image, "my-registry.example.com/dell/storage/"),
				"sdc image should retain repository path, got %q", *ic.Image)
			assert.True(t, strings.Contains(*ic.Image, "sdc:"),
				"sdc image should contain the image name, got %q", *ic.Image)
			break
		}
	}
	assert.True(t, foundSDC, "expected to find sdc init container in node daemonset")
}

func TestSubstituteEnvVar(t *testing.T) {
	tests := []struct {
		name       string
		yamlString string
		varName    string
		value      string
		expected   string
	}{
		{
			name:       "Single-variable text substitution",
			yamlString: "TEST_PARAM_A=<TEST_PARAM_A>",
			varName:    "TEST_PARAM_A",
			value:      "false",
			expected:   "TEST_PARAM_A=false",
		},
		{
			name:       "Mutli-variable text substitution",
			yamlString: "TEST_PARAM_A=<TEST_PARAM_A> TEST_PARAM_B='<TEST_PARAM_B>'",
			varName:    "TEST_PARAM_B",
			value:      "true",
			expected:   "TEST_PARAM_A=<TEST_PARAM_A> TEST_PARAM_B='true'",
		},
		{
			name:       "No-variable text substitution",
			yamlString: "TEST_PARAM_A=false TEST_PARAM_B=true",
			varName:    "TEST_PARAM_B",
			value:      "false",
			expected:   "TEST_PARAM_A=false TEST_PARAM_B=true",
		},
		{
			name:       "Empty yaml string",
			yamlString: "",
			varName:    "TEST_PARAM_A",
			value:      "val",
			expected:   "",
		},
		{
			name:       "Empty variable name",
			yamlString: "TEST_PARAM_A=<TEST_PARAM_A>",
			varName:    "",
			value:      "val",
			expected:   "TEST_PARAM_A=<TEST_PARAM_A>",
		},
		{
			name:       "Empty value",
			yamlString: "TEST_PARAM_A=<TEST_PARAM_A>",
			varName:    "TEST_PARAM_A",
			value:      "",
			expected:   "TEST_PARAM_A=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SubstituteEnvVar(tt.yamlString, tt.varName, tt.value)
			assert.Equal(t, tt.expected, result, "SubstituteEnvVar should correctly substitute placeholders")
		})
	}
}
