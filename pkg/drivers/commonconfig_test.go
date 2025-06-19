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
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/tests/shared"
	"github.com/stretchr/testify/assert"
	storagev1 "k8s.io/api/storage/v1"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	csm                  = csmWithTolerations(csmv1.PowerScaleName, shared.ConfigVersion)
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
		{"config file is invalid", csm, badDriver, "bad.yaml", "unmarshal"},
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
			_, err := GetUpgradeInfo(ctx, config, tt.driverName, tt.csm.Spec.Driver.ConfigVersion)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

func TestGetController(t *testing.T) {
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetController(ctx, tt.csm, config, tt.driverName)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

func TestGetNode(t *testing.T) {
	ctx := context.Background()
	foundInitMdm := false
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := GetNode(ctx, tt.csm, config, tt.driverName, tt.filename, ctrlClientFake.NewClientBuilder().Build())
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

				assert.Equal(t, tt.csm.GetName(), *node.DaemonSetApplyConfig.OwnerReferences[0].Name)
				assert.Equal(t, tt.csm.Kind, *node.DaemonSetApplyConfig.OwnerReferences[0].Kind)
				assert.True(t, *node.DaemonSetApplyConfig.OwnerReferences[0].Controller)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}
