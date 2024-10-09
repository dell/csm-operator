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
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/tests/shared"
	"github.com/dell/csm-operator/tests/shared/crclient"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	powerFlexCSM              = csmForPowerFlex(pflexCSMName)
	powerFlexCSMBadVersion    = csmForPowerFlexBadVersion()
	powerFlexClient           = crclient.NewFakeClientNoInjector(objects)
	configJSONFileGood        = fmt.Sprintf("%s/driverconfig/%s/config.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBadUser     = fmt.Sprintf("%s/driverconfig/%s/config-empty-username.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBadPW       = fmt.Sprintf("%s/driverconfig/%s/config-empty-password.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBadSysID    = fmt.Sprintf("%s/driverconfig/%s/config-empty-sysid.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBadEndPoint = fmt.Sprintf("%s/driverconfig/%s/config-empty-endpoint.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileTwoDefaults = fmt.Sprintf("%s/driverconfig/%s/config-two-defaults.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBadMDM      = fmt.Sprintf("%s/driverconfig/%s/config-invalid-mdm.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileDuplSysID   = fmt.Sprintf("%s/driverconfig/%s/config-duplicate-sysid.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileEmpty       = fmt.Sprintf("%s/driverconfig/%s/config-empty.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBad         = fmt.Sprintf("%s/driverconfig/%s/config-bad.json", config.ConfigDirectory, csmv1.PowerFlex)
	powerFlexSecret           = shared.MakeSecretWithJSON(pflexCredsName, pFlexNS, configJSONFileGood)
	fakeSecret                = shared.MakeSecret("fake-secret", "fake-ns", shared.PFlexConfigVersion)

	powerFlexTests = []struct {
		// every single unit test name
		name string
		// csm object
		csm csmv1.ContainerStorageModule
		// client
		ct client.Client
		// secret
		sec *corev1.Secret
		// expected error
		expectedErr string
	}{
		{"missing secret", powerFlexCSM, powerFlexClient, fakeSecret, "no secrets found"},
		{"happy path", powerFlexCSM, powerFlexClient, powerFlexSecret, ""},
		{"happy path without sdc", csmForPowerFlex("no-sdc"), powerFlexClient, shared.MakeSecretWithJSON("no-sdc-config", pFlexNS, configJSONFileGood), ""},
		{"bad version", powerFlexCSMBadVersion, powerFlexClient, powerFlexSecret, "not supported"},
		{"bad username", csmForPowerFlex("bad-user"), powerFlexClient, shared.MakeSecretWithJSON("bad-user-config", pFlexNS, configJSONFileBadUser), "invalid value for Username"},
		{"bad password", csmForPowerFlex("bad-pw"), powerFlexClient, shared.MakeSecretWithJSON("bad-pw-config", pFlexNS, configJSONFileBadPW), "invalid value for Password"},
		{"bad system ID", csmForPowerFlex("bad-sysid"), powerFlexClient, shared.MakeSecretWithJSON("bad-sysid-config", pFlexNS, configJSONFileBadSysID), "invalid value for SystemID"},
		{"bad endpoint", csmForPowerFlex("bad-endpt"), powerFlexClient, shared.MakeSecretWithJSON("bad-endpt-config", pFlexNS, configJSONFileBadEndPoint), "invalid value for RestGateway"},
		{"two default systems", csmForPowerFlex("two-def-sys"), powerFlexClient, shared.MakeSecretWithJSON("two-def-sys-config", pFlexNS, configJSONFileTwoDefaults), "multiple places"},
		{"bad mdm ip", csmForPowerFlex("bad-mdm"), powerFlexClient, shared.MakeSecretWithJSON("bad-mdm-config", pFlexNS, configJSONFileBadMDM), "Invalid MDM value"},
		{"duplicate system id", csmForPowerFlex("dupl-sysid"), powerFlexClient, shared.MakeSecretWithJSON("dupl-sysid-config", pFlexNS, configJSONFileDuplSysID), "Duplicate SystemID"},
		{"empty config", csmForPowerFlex("empty"), powerFlexClient, shared.MakeSecretWithJSON("empty-config", pFlexNS, configJSONFileEmpty), "Arrays details are not provided"},
		{"bad config", csmForPowerFlex("bad"), powerFlexClient, shared.MakeSecretWithJSON("bad-config", pFlexNS, configJSONFileBad), "unable to parse"},
	}

	modifyPowerflexCRTests = []struct {
		name       string
		yamlString string
		cr         csmv1.ContainerStorageModule
		fileType   string
		expected   string
	}{
		{
			name:       "Controller case with values",
			yamlString: "CSI_HEALTH_MONITOR_ENABLED=OLD CSI_POWERFLEX_EXTERNAL_ACCESS=OLD CSI_DEBUG=OLD",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Controller: csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{Name: "X_CSI_POWERFLEX_EXTERNAL_ACCESS", Value: "NEW_POWERFLEX_ACCESS"},
								{Name: "X_CSI_HEALTH_MONITOR_ENABLED", Value: "NEW_HEALTH_MONITOR"},
								{Name: "X_CSI_DEBUG", Value: "NEW_DEBUG"},
							},
						},
					},
				},
			},
			fileType: "Controller",
			expected: "CSI_HEALTH_MONITOR_ENABLED=OLD CSI_POWERFLEX_EXTERNAL_ACCESS=OLD CSI_DEBUG=OLD",
		},
		{
			name:       "Node case with values",
			yamlString: "CSI_SDC_ENABLED=NEW_SDC_ENABLED CSI_APPROVE_SDC_ENABLED=NEW_APPROVE_SDC CSI_RENAME_SDC_ENABLED=NEW_RENAME_SDC CSI_PREFIX_RENAME_SDC=NEW_RENAME_PREFIX CSI_VXFLEXOS_MAX_VOLUMES_PER_NODE=NEW_MAX_VOLUMES CSI_HEALTH_MONITOR_ENABLED=NEW_HEALTH_MONITOR_NODE CSI_DEBUG=NEW_DEBUG",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Node: csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{Name: "X_CSI_SDC_ENABLED", Value: "NEW_SDC_ENABLED"},
								{Name: "X_CSI_APPROVE_SDC_ENABLED", Value: "NEW_APPROVE_SDC"},
								{Name: "X_CSI_RENAME_SDC_ENABLED", Value: "NEW_RENAME_SDC"},
								{Name: "X_CSI_PREFIX_RENAME_SDC", Value: "NEW_RENAME_PREFIX"},
								{Name: "X_CSI_VXFLEXOS_MAX_VOLUMES_PER_NODE", Value: "NEW_MAX_VOLUMES"},
								{Name: "X_CSI_HEALTH_MONITOR_ENABLED", Value: "NEW_HEALTH_MONITOR_NODE"},
								{Name: "X_CSI_DEBUG", Value: "NEW_DEBUG"},
							},
						},
					},
				},
			},
			fileType: "Node",
			expected: "CSI_SDC_ENABLED=NEW_SDC_ENABLED CSI_APPROVE_SDC_ENABLED=NEW_APPROVE_SDC CSI_RENAME_SDC_ENABLED=NEW_RENAME_SDC CSI_PREFIX_RENAME_SDC=NEW_RENAME_PREFIX CSI_VXFLEXOS_MAX_VOLUMES_PER_NODE=NEW_MAX_VOLUMES CSI_HEALTH_MONITOR_ENABLED=NEW_HEALTH_MONITOR_NODE CSI_DEBUG=NEW_DEBUG",
		},
		{
			name:       "CSIDriverSpec case with storage capacity",
			yamlString: "CSI_STORAGE_CAPACITY_ENABLED=OLD CSI_VXFLEXOS_QUOTA_ENABLED=OLD",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						CSIDriverSpec: csmv1.CSIDriverSpec{
							StorageCapacity: true,
						},
					},
				},
			},
			fileType: "CSIDriverSpec",
			expected: "CSI_STORAGE_CAPACITY_ENABLED=OLD CSI_VXFLEXOS_QUOTA_ENABLED=OLD",
		},
		{
			name:       "CSIDriverSpec case without storage capacity",
			yamlString: "CSI_STORAGE_CAPACITY_ENABLED=OLD CSI_VXFLEXOS_QUOTA_ENABLED=OLD",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						CSIDriverSpec: csmv1.CSIDriverSpec{
							StorageCapacity: false,
						},
					},
				},
			},
			fileType: "CSIDriverSpec",
			expected: "CSI_STORAGE_CAPACITY_ENABLED=OLD CSI_VXFLEXOS_QUOTA_ENABLED=OLD",
		},
	}
)

func TestPowerFlexGo(t *testing.T) {
	ctx := context.Background()
	for _, tt := range powerFlexTests {
		err := tt.ct.Create(ctx, tt.sec)
		if err != nil {
			assert.Nil(t, err)
		}
		t.Run(tt.name, func(t *testing.T) { // #nosec G601 - Run waits for the call to complete.
			err := PrecheckPowerFlex(ctx, &tt.csm, config, tt.ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
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

// makes a csm object with a bad version
func csmForPowerFlexBadVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM(pflexCSMName, pFlexNS, shared.PFlexConfigVersion)

	// Add pflex driver version
	res.Spec.Driver.ConfigVersion = shared.BadConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerFlex

	return res
}

func TestModifyPowerflexCR(t *testing.T) {
	for _, tt := range modifyPowerflexCRTests {
		t.Run(tt.name, func(t *testing.T) {
			result := ModifyPowerflexCR(tt.yamlString, tt.cr, tt.fileType)
			assert.Equal(t, tt.expected, result)
		})
	}
}
