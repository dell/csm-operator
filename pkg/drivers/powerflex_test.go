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
	shared "github.com/dell/csm-operator/tests/sharedutil"
	"github.com/dell/csm-operator/tests/sharedutil/crclient"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	powerFlexCSM               = csmForPowerFlex(pflexCSMName)
	powerFlexCSMBadVersion     = csmForPowerFlexBadVersion()
	powerFlexInvalidCSMVersion = csmForPowerFlexInvalidVersion()
	powerFlexClient            = crclient.NewFakeClientNoInjector(objects)
	configJSONFileGood         = fmt.Sprintf("%s/driverconfig/%s/config.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBadUser      = fmt.Sprintf("%s/driverconfig/%s/config-empty-username.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBadPW        = fmt.Sprintf("%s/driverconfig/%s/config-empty-password.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBadSysID     = fmt.Sprintf("%s/driverconfig/%s/config-empty-sysid.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBadEndPoint  = fmt.Sprintf("%s/driverconfig/%s/config-empty-endpoint.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileTwoDefaults  = fmt.Sprintf("%s/driverconfig/%s/config-two-defaults.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBadMDM       = fmt.Sprintf("%s/driverconfig/%s/config-invalid-mdm.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileDuplSysID    = fmt.Sprintf("%s/driverconfig/%s/config-duplicate-sysid.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileEmpty        = fmt.Sprintf("%s/driverconfig/%s/config-empty.json", config.ConfigDirectory, csmv1.PowerFlex)
	configJSONFileBad          = fmt.Sprintf("%s/driverconfig/%s/config-bad.json", config.ConfigDirectory, csmv1.PowerFlex)
	powerFlexSecret            = shared.MakeSecretWithJSON(pflexCredsName, pFlexNS, configJSONFileGood)
	fakeSecret                 = shared.MakeSecret("fake-secret", "fake-ns", shared.PFlexConfigVersion)

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
		{"happy path with initContainers but no MDM", csmForPowerFlex("no-mdm"), powerFlexClient, shared.MakeSecretWithJSON("no-mdm-config", pFlexNS, configJSONFileGood), ""},
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
		{"Auth and Replication enabled with valid prefix", csmForPowerFlex("auth-repl-valid-prefix"), powerFlexClient, shared.MakeSecretWithJSON("auth-repl-valid-prefix-config", pFlexNS, configJSONFileGood), ""},
		{"Auth and Replication enabled with invalid prefix", csmForPowerFlex("auth-repl-invalid-prefix"), powerFlexClient, shared.MakeSecretWithJSON("auth-repl-invalid-prefix-config", pFlexNS, configJSONFileGood), "volume name prefix"},
		{"invalid csm version", powerFlexInvalidCSMVersion, powerFlexClient, powerFlexSecret, "No custom resource configuration is available for CSM version v1.10.0"},
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
						Controller: &csmv1.ContainerTemplate{
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
			yamlString: "CSI_SDC_ENABLED=NEW_SDC_ENABLED CSI_APPROVE_SDC_ENABLED=NEW_APPROVE_SDC CSI_RENAME_SDC_ENABLED=NEW_RENAME_SDC CSI_PREFIX_RENAME_SDC=NEW_RENAME_PREFIX CSI_VXFLEXOS_MAX_VOLUMES_PER_NODE=NEW_MAX_VOLUMES CSI_HEALTH_MONITOR_ENABLED=NEW_HEALTH_MONITOR_NODE CSI_DEBUG=NEW_DEBUG CSI_FS_CHECK_ENABLED=<X_CSI_FS_CHECK_ENABLED> CSI_FS_CHECK_MODE=<X_CSI_FS_CHECK_MODE>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Node: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{Name: "X_CSI_SDC_ENABLED", Value: "NEW_SDC_ENABLED"},
								{Name: "X_CSI_APPROVE_SDC_ENABLED", Value: "NEW_APPROVE_SDC"},
								{Name: "X_CSI_RENAME_SDC_ENABLED", Value: "NEW_RENAME_SDC"},
								{Name: "X_CSI_PREFIX_RENAME_SDC", Value: "NEW_RENAME_PREFIX"},
								{Name: "X_CSI_VXFLEXOS_MAX_VOLUMES_PER_NODE", Value: "NEW_MAX_VOLUMES"},
								{Name: "X_CSI_HEALTH_MONITOR_ENABLED", Value: "NEW_HEALTH_MONITOR_NODE"},
								{Name: "X_CSI_DEBUG", Value: "NEW_DEBUG"},
								{Name: "X_CSI_RENAME_SDC_PREFIX", Value: "P1"},
								{Name: "X_CSI_SDC_SFTP_REPO_ENABLED", Value: "true"},
								{Name: "REPO_ADDRESS", Value: "sftp://0.0.0.0"},
								{Name: "REPO_USER", Value: "sftpuser"},
								{Name: "X_CSI_MAX_VOLUMES_PER_NODE", Value: "100"},
								{Name: "X_CSI_FS_CHECK_ENABLED", Value: "false"},
								{Name: "X_CSI_FS_CHECK_MODE", Value: "checkOnly"},
							},
						},
					},
				},
			},
			fileType: "Node",
			expected: "CSI_SDC_ENABLED=NEW_SDC_ENABLED CSI_APPROVE_SDC_ENABLED=NEW_APPROVE_SDC CSI_RENAME_SDC_ENABLED=NEW_RENAME_SDC CSI_PREFIX_RENAME_SDC=NEW_RENAME_PREFIX CSI_VXFLEXOS_MAX_VOLUMES_PER_NODE=NEW_MAX_VOLUMES CSI_HEALTH_MONITOR_ENABLED=NEW_HEALTH_MONITOR_NODE CSI_DEBUG=NEW_DEBUG CSI_FS_CHECK_ENABLED=false CSI_FS_CHECK_MODE=checkOnly",
		},
		{
			name:       "Node case with values - fscheck",
			yamlString: "CSI_FS_CHECK_ENABLED=<X_CSI_FS_CHECK_ENABLED> CSI_FS_CHECK_MODE=<X_CSI_FS_CHECK_MODE>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{Name: "X_CSI_FS_CHECK_ENABLED", Value: "true"},
								{Name: "X_CSI_FS_CHECK_MODE", Value: "checkOnly"},
							},
						},
					},
				},
			},
			fileType: "Node",
			expected: "CSI_FS_CHECK_ENABLED=true CSI_FS_CHECK_MODE=checkOnly",
		},
		{
			name:       "Node case with values - space reclamation",
			yamlString: "CSI_SPACE_RECLAMATION_ENABLED=<X_CSI_SPACE_RECLAMATION_ENABLED> CSI_SPACE_RECLAMATION_SCHEDULE=<X_CSI_SPACE_RECLAMATION_SCHEDULE> CSI_SPACE_RECLAMATION_MAX_CONCURRENT=<X_CSI_SPACE_RECLAMATION_MAX_CONCURRENT> CSI_SPACE_RECLAMATION_TIMEOUT=<X_CSI_SPACE_RECLAMATION_TIMEOUT>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{Name: "X_CSI_SPACE_RECLAMATION_ENABLED", Value: "true"},
								{Name: "X_CSI_SPACE_RECLAMATION_SCHEDULE", Value: "0 2 * * 0"},
								{Name: "X_CSI_SPACE_RECLAMATION_MAX_CONCURRENT", Value: "1"},
								{Name: "X_CSI_SPACE_RECLAMATION_TIMEOUT", Value: "1h"},
							},
						},
					},
				},
			},
			fileType: "Node",
			expected: "CSI_SPACE_RECLAMATION_ENABLED=true CSI_SPACE_RECLAMATION_SCHEDULE=0 2 * * 0 CSI_SPACE_RECLAMATION_MAX_CONCURRENT=1 CSI_SPACE_RECLAMATION_TIMEOUT=1h",
		},
		{
			name:       "CSIDriverSpec case with storage capacity",
			yamlString: "CSI_STORAGE_CAPACITY_ENABLED=OLD CSI_VXFLEXOS_QUOTA_ENABLED=OLD",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						CSIDriverSpec: &csmv1.CSIDriverSpec{
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
						CSIDriverSpec: &csmv1.CSIDriverSpec{
							StorageCapacity: false,
						},
					},
				},
			},
			fileType: "CSIDriverSpec",
			expected: "CSI_STORAGE_CAPACITY_ENABLED=OLD CSI_VXFLEXOS_QUOTA_ENABLED=OLD",
		},
		{
			name:       "update GOSCALEIO_SHOWHTTP value for Controller",
			yamlString: "<GOSCALEIO_SHOWHTTP>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{Name: "GOSCALEIO_SHOWHTTP", Value: "true"},
							},
						},
					},
				},
			},
			fileType: "Controller",
			expected: "true",
		},
		{
			name:       "update GOSCALEIO_SHOWHTTP value for Node",
			yamlString: "<GOSCALEIO_SHOWHTTP>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{Name: "GOSCALEIO_SHOWHTTP", Value: "true"},
							},
						},
					},
				},
			},
			fileType: "Node",
			expected: "true",
		},
		{
			name:       "update GOSCALEIO_DEBUG value for Controller",
			yamlString: "<GOSCALEIO_DEBUG>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{Name: "GOSCALEIO_DEBUG", Value: "false"},
							},
						},
					},
				},
			},
			fileType: "Controller",
			expected: "false",
		},
		{
			name:       "update GOSCALEIO_DEBUG value for Node",
			yamlString: "<GOSCALEIO_DEBUG>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{Name: "GOSCALEIO_DEBUG", Value: "false"},
							},
						},
					},
				},
			},
			fileType: "Node",
			expected: "false",
		},
		{
			name:       "update common X_CSI_PROBE_TIMEOUT value in CR",
			yamlString: "X_CSI_PROBE_TIMEOUT=<X_CSI_PROBE_TIMEOUT>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{Name: "X_CSI_PROBE_TIMEOUT", Value: "5s"},
							},
						},
					},
				},
			},
			fileType: "Controller",
			expected: "X_CSI_PROBE_TIMEOUT=5s",
		},
		{
			name:       "update common X_CSI_AUTH_TYPE value in CR",
			yamlString: "X_CSI_AUTH_TYPE=<X_CSI_AUTH_TYPE>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{Name: "X_CSI_AUTH_TYPE", Value: "OIDC"},
							},
						},
					},
				},
			},
			fileType: "Controller",
			expected: "X_CSI_AUTH_TYPE=OIDC",
		},
		{
			name:       "metrics nil - all placeholders get defaults for Controller",
			yamlString: "<X_CSI_METRICS_ENABLED> <X_CSI_METRICS_PORT> <X_CSI_GATEWAY_MONITORING_ENABLED> <X_CSI_GATEWAY_MONITORING_LEADER_ELECTION_ENABLED> <X_CSI_GATEWAY_MONITORING_POLL_INTERVAL>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{},
				},
			},
			fileType: "Controller",
			expected: "false 9090 false true 30s",
		},
		{
			name:       "metrics enabled for Controller",
			yamlString: "<X_CSI_METRICS_ENABLED>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Metrics: &csmv1.DriverMetrics{Enabled: true},
					},
				},
			},
			fileType: "Controller",
			expected: "true",
		},
		{
			name:       "metrics custom port for Controller",
			yamlString: "<X_CSI_METRICS_PORT>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Metrics: &csmv1.DriverMetrics{Port: 8080},
					},
				},
			},
			fileType: "Controller",
			expected: "8080",
		},
		{
			name:       "gateway monitoring enabled when metrics is also enabled",
			yamlString: "<X_CSI_METRICS_ENABLED> <X_CSI_GATEWAY_MONITORING_ENABLED>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Metrics: &csmv1.DriverMetrics{
							Enabled: true,
							GatewayMonitoring: &csmv1.GatewayMonitoringConfig{
								Enabled: true,
							},
						},
					},
				},
			},
			fileType: "Controller",
			expected: "true true",
		},
		{
			name:       "gateway monitoring disabled when metrics is disabled",
			yamlString: "<X_CSI_GATEWAY_MONITORING_ENABLED>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Metrics: &csmv1.DriverMetrics{
							Enabled: false,
							GatewayMonitoring: &csmv1.GatewayMonitoringConfig{
								Enabled: true,
							},
						},
					},
				},
			},
			fileType: "Controller",
			expected: "false",
		},
		{
			name:       "gateway monitoring leader election disabled",
			yamlString: "<X_CSI_GATEWAY_MONITORING_LEADER_ELECTION_ENABLED>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Metrics: &csmv1.DriverMetrics{
							GatewayMonitoring: &csmv1.GatewayMonitoringConfig{
								LeaderElectionEnabled: &falseBool,
							},
						},
					},
				},
			},
			fileType: "Controller",
			expected: "false",
		},
		{
			name:       "gateway monitoring custom poll interval",
			yamlString: "<X_CSI_GATEWAY_MONITORING_POLL_INTERVAL>",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Metrics: &csmv1.DriverMetrics{
							GatewayMonitoring: &csmv1.GatewayMonitoringConfig{
								PollInterval: "60s",
							},
						},
					},
				},
			},
			fileType: "Controller",
			expected: "60s",
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
			// Use configForVersionChecks for invalid CSM version test
			cfg := config
			if tt.name == "invalid csm version" {
				cfg = configForVersionChecks
			}
			err := PrecheckPowerFlex(ctx, &tt.csm, cfg, tt.ct)
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

// makes a csm object with a bad version
func csmForPowerFlexBadVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM(pflexCSMName, pFlexNS, shared.PFlexConfigVersion)

	// Add pflex driver version
	res.Spec.Driver.ConfigVersion = shared.BadConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerFlex

	return res
}

// makes a csm object with a bad version
func csmForPowerFlexInvalidVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM(pflexCSMName, pFlexNS, shared.PFlexConfigVersion)

	// Add pflex driver version
	res.Spec.Version = shared.InvalidCSMVersion
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

func TestExtractZonesFromSecret(t *testing.T) {
	emptyConfigData := ``
	invalidConfigData := `
- username: "admin"
	-
`
	dataWithNoSystemID := `
- username: "admin"
  password: "password"
  endpoint: "https://127.0.0.2"
  skipCertificateValidation: true
  mdm: "10.0.0.3,10.0.0.4"
`
	dataWithZone := `
- username: "admin"
  password: "password"
  systemID: "2b11bb111111bb1b"
  endpoint: "https://127.0.0.2"
  skipCertificateValidation: true
  mdm: "10.0.0.3,10.0.0.4"
  zone:
    name: "US-EAST"
    labelKey: "zone.csi-vxflexos.dellemc.com"
`
	zoneDataWithMultiArray := `
- username: "admin"
  password: "password"
  systemID: "2b11bb111111bb1b"
  endpoint: "https://127.0.0.2"
  skipCertificateValidation: true
  mdm: "10.0.0.3,10.0.0.4"
  zone:
    name: "ZONE-1"
    labelKey: "zone.csi-vxflexos.dellemc.com"
- username: "admin"
  password: "password"
  systemID: "1a99aa999999aa9a"
  endpoint: "https://127.0.0.1"
  skipCertificateValidation: true
  mdm: "10.0.0.5,10.0.0.6"
  zone:
    name: "ZONE-2"
    labelKey: "zone.csi-vxflexos.dellemc.com"
`
	zoneDataWithMultiArraySomeZone := `
- username: "admin"
  password: "password"
  systemID: "2b11bb111111bb1b"
  endpoint: "https://127.0.0.2"
  skipCertificateValidation: true
  mdm: "10.0.0.3,10.0.0.4"
- username: "admin"
  password: "password"
  systemID: "1a99aa999999aa9a"
  endpoint: "https://127.0.0.1"
  skipCertificateValidation: true
  mdm: "10.0.0.5,10.0.0.6"
  zone:
    name: "ZONE-2"
    labelKey: "zone.csi-vxflexos.dellemc.com"
`
	zoneDataWithMultiArraySomeZone2 := `
- username: "admin"
  password: "password"
  systemID: "2b11bb111111bb1b"
  endpoint: "https://127.0.0.2"
  skipCertificateValidation: true
  mdm: "10.0.0.3,10.0.0.4"
- username: "admin"
  password: "password"
  systemID: "1a99aa999999aa9a"
  endpoint: "https://127.0.0.1"
  skipCertificateValidation: true
  mdm: "10.0.0.5,10.0.0.6"
- username: "admin"
  password: "password"
  systemID: "1a99aa999999aa9a"
  endpoint: "https://127.0.0.1"
  skipCertificateValidation: true
  mdm: "10.0.0.5,10.0.0.6"
  zone:
    name: "ZONE-2"
    labelKey: "zone.csi-vxflexos.dellemc.com"
- username: "admin"
  password: "password"
  systemID: "1a99aa999999aa9a"
  endpoint: "https://127.0.0.1"
  skipCertificateValidation: true
  mdm: "10.0.0.5,10.0.0.6"
`
	dataWithoutZone := `
- username: "admin"
  password: "password"
  systemID: "2b11bb111111bb1b"
  endpoint: "https://127.0.0.2"
  skipCertificateValidation: true
  mdm: "10.0.0.3,10.0.0.4"
`
	zoneDataWithMultiArrayPartialZone1 := `
- username: "admin"
  password: "password"
  systemID: "2b11bb111111bb1b"
  endpoint: "https://127.0.0.2"
  skipCertificateValidation: true
  mdm: "10.0.0.3,10.0.0.4"
  zone:
    name: "ZONE-1"
    labelKey: "zone.csi-vxflexos.dellemc.com"
- username: "admin"
  password: "password"
  systemID: "1a99aa999999aa9a"
  endpoint: "https://127.0.0.1"
  skipCertificateValidation: true
  mdm: "10.0.0.5,10.0.0.6"
  zone:
    name: ""
    labelKey: "zone.csi-vxflexos.dellemc.com"
`
	zoneDataWithMultiArrayPartialZone2 := `
- username: "admin"
  password: "password"
  systemID: "2b11bb111111bb1b"
  endpoint: "https://127.0.0.2"
  skipCertificateValidation: true
  mdm: "10.0.0.3,10.0.0.4"
  zone:
    name: "ZONE-1"
    labelKey: "zone.csi-vxflexos.dellemc.com"
- username: "admin"
  password: "password"
  systemID: "1a99aa999999aa9a"
  endpoint: "https://127.0.0.1"
  skipCertificateValidation: true
  mdm: "10.0.0.5,10.0.0.6"
  zone:
    name: "myname"
`
	zoneDataWithMultiArrayPartialZone3 := `
- username: "admin"
  password: "password"
  systemID: "2b11bb111111bb1b"
  endpoint: "https://127.0.0.2"
  skipCertificateValidation: true
  mdm: "10.0.0.3,10.0.0.4"
  zone:
    name: "ZONE-1"
    labelKey: ""
- username: "admin"
  password: "password"
  systemID: "1a99aa999999aa9a"
  endpoint: "https://127.0.0.1"
  skipCertificateValidation: true
  mdm: "10.0.0.5,10.0.0.6"
  zone:
    name: "myname"
    labelKey: "zone.csi-vxflexos.dellemc.com"
`

	ctx := context.Background()
	tests := map[string]func() (client.WithWatch, string, bool){
		"success with zone": func() (client.WithWatch, string, bool) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vxflexos-config",
					Namespace: "vxflexos",
				},
				Data: map[string][]byte{
					"config": []byte(dataWithZone),
				},
			}

			client := fake.NewClientBuilder().WithObjects(secret).Build()
			return client, "vxflexos-config", false
		},
		"success with zone and multi array": func() (client.WithWatch, string, bool) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vxflexos-config",
					Namespace: "vxflexos",
				},
				Data: map[string][]byte{
					"config": []byte(zoneDataWithMultiArray),
				},
			}

			client := fake.NewClientBuilder().WithObjects(secret).Build()
			return client, "vxflexos-config", false
		},
		"fail multi array but only some zone": func() (client.WithWatch, string, bool) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vxflexos-config",
					Namespace: "vxflexos",
				},
				Data: map[string][]byte{
					"config": []byte(zoneDataWithMultiArraySomeZone),
				},
			}
			client := fake.NewClientBuilder().WithObjects(secret).Build()
			return client, "vxflexos-config", true
		},
		"fail multi array but only some zone test two": func() (client.WithWatch, string, bool) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vxflexos-config",
					Namespace: "vxflexos",
				},
				Data: map[string][]byte{
					"config": []byte(zoneDataWithMultiArraySomeZone2),
				},
			}
			client := fake.NewClientBuilder().WithObjects(secret).Build()
			return client, "vxflexos-config", true
		},
		"success no zone": func() (client.WithWatch, string, bool) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vxflexos-config",
					Namespace: "vxflexos",
				},
				Data: map[string][]byte{
					"config": []byte(dataWithoutZone),
				},
			}

			client := fake.NewClientBuilder().WithObjects(secret).Build()
			return client, "vxflexos-config", false
		},
		"error getting secret": func() (client.WithWatch, string, bool) {
			client := fake.NewClientBuilder().Build()
			return client, "vxflexos-not-found", true
		},
		"error parsing empty secret": func() (client.WithWatch, string, bool) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vxflexos-config",
					Namespace: "vxflexos",
				},
				Data: map[string][]byte{
					"config": []byte(emptyConfigData),
				},
			}

			client := fake.NewClientBuilder().WithObjects(secret).Build()
			return client, "vxflexos-config", true
		},
		"error with no system id": func() (client.WithWatch, string, bool) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vxflexos-config",
					Namespace: "vxflexos",
				},
				Data: map[string][]byte{
					"config": []byte(dataWithNoSystemID),
				},
			}

			client := fake.NewClientBuilder().WithObjects(secret).Build()
			return client, "vxflexos-config", true
		},

		"error unmarshaling config": func() (client.WithWatch, string, bool) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vxflexos-config",
					Namespace: "vxflexos",
				},
				Data: map[string][]byte{
					"config": []byte(invalidConfigData),
				},
			}

			client := fake.NewClientBuilder().WithObjects(secret).Build()
			return client, "vxflexos-config", true
		},
		"Fail Partial Zone Config 1": func() (client.WithWatch, string, bool) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vxflexos-config",
					Namespace: "vxflexos",
				},
				Data: map[string][]byte{
					"config": []byte(zoneDataWithMultiArrayPartialZone1),
				},
			}

			client := fake.NewClientBuilder().WithObjects(secret).Build()
			return client, "vxflexos-config", true
		},
		"Fail Partial Zone Config 2": func() (client.WithWatch, string, bool) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vxflexos-config",
					Namespace: "vxflexos",
				},
				Data: map[string][]byte{
					"config": []byte(zoneDataWithMultiArrayPartialZone2),
				},
			}

			client := fake.NewClientBuilder().WithObjects(secret).Build()
			return client, "vxflexos-config", true
		},
		"Fail Partial Zone Config 3": func() (client.WithWatch, string, bool) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vxflexos-config",
					Namespace: "vxflexos",
				},
				Data: map[string][]byte{
					"config": []byte(zoneDataWithMultiArrayPartialZone3),
				},
			}

			client := fake.NewClientBuilder().WithObjects(secret).Build()
			return client, "vxflexos-config", true
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			client, secret, wantErr := tc()
			err := ValidateZonesInSecret(ctx, client, "vxflexos", secret)
			if wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestRemoveVolume(t *testing.T) {
	volumeName := ScaleioBinPath
	differentVolumeName := "different-volume-name"
	containerName := "driver"
	tests := []struct {
		name          string
		configuration *v1.DaemonSetApplyConfiguration
		wantErr       bool
	}{
		{
			name: "Remove volume and mount",
			configuration: &v1.DaemonSetApplyConfiguration{
				Spec: &v1.DaemonSetSpecApplyConfiguration{
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
			wantErr: false,
		},
		{
			name: "RemoveVolume called with a daemonset that doesn't include the volume",
			configuration: &v1.DaemonSetApplyConfiguration{
				Spec: &v1.DaemonSetSpecApplyConfiguration{
					Template: &acorev1.PodTemplateSpecApplyConfiguration{
						Spec: &acorev1.PodSpecApplyConfiguration{
							Volumes: []acorev1.VolumeApplyConfiguration{
								{
									Name: &differentVolumeName,
								},
							},
							Containers: []acorev1.ContainerApplyConfiguration{
								{
									Name: &containerName,
									VolumeMounts: []acorev1.VolumeMountApplyConfiguration{
										{
											Name: &differentVolumeName,
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:          "RemoveVolume called with nil daemonset",
			configuration: nil,
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RemoveVolume(tt.configuration, volumeName); (err != nil) != tt.wantErr {
				t.Errorf("RemoveVolume() error = %v, wantErr %v", err, tt.wantErr)
			}
			// check that the volume and volume mount were removed
			if !tt.wantErr {
				for i := range tt.configuration.Spec.Template.Spec.Volumes {
					assert.NotEqual(t, *tt.configuration.Spec.Template.Spec.Volumes[i].Name, volumeName)
				}
				for c := range tt.configuration.Spec.Template.Spec.Containers {
					for i := range tt.configuration.Spec.Template.Spec.Containers[c].VolumeMounts {
						assert.NotEqual(t, *tt.configuration.Spec.Template.Spec.Containers[c].VolumeMounts[i].Name, volumeName)
					}
				}
			}
		})
	}
}

func TestPrecheckPowerFlexTLSCert(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		csm         csmv1.ContainerStorageModule
		secrets     []*corev1.Secret
		expectedErr string
	}{
		{
			name: "metrics tls cert secret exists",
			csm: func() csmv1.ContainerStorageModule {
				cr := csmForPowerFlex("pflex-tls-test")
				cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{ // #nosec G101
					Enabled:       true,
					TLSCertSecret: "pflex-tls-cert",
				}
				return cr
			}(),
			secrets: []*corev1.Secret{
				shared.MakeSecretWithJSON("pflex-tls-test-config", pFlexNS, configJSONFileGood),
				shared.MakeSecret("pflex-tls-cert", pFlexNS, shared.PFlexConfigVersion),
			},
			expectedErr: "",
		},
		{
			name: "metrics tls cert secret missing",
			csm: func() csmv1.ContainerStorageModule {
				cr := csmForPowerFlex("pflex-tls-test")
				cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{ // #nosec G101
					Enabled:       true,
					TLSCertSecret: "missing-tls-secret",
				}
				return cr
			}(),
			secrets: []*corev1.Secret{
				shared.MakeSecretWithJSON("pflex-tls-test-config", pFlexNS, configJSONFileGood),
			},
			expectedErr: "failed to find secret missing-tls-secret",
		},
		{
			name: "metrics disabled with tls cert secret set skips check",
			csm: func() csmv1.ContainerStorageModule {
				cr := csmForPowerFlex("pflex-tls-test")
				cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{ // #nosec G101
					Enabled:       false,
					TLSCertSecret: "missing-tls-secret",
				}
				return cr
			}(),
			secrets: []*corev1.Secret{
				shared.MakeSecretWithJSON("pflex-tls-test-config", pFlexNS, configJSONFileGood),
			},
			expectedErr: "",
		},
		{
			name: "metrics nil skips tls cert check",
			csm:  csmForPowerFlex("pflex-tls-test"),
			secrets: []*corev1.Secret{
				shared.MakeSecretWithJSON("pflex-tls-test-config", pFlexNS, configJSONFileGood),
			},
			expectedErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := make([]client.Object, 0, len(tt.secrets))
			for _, s := range tt.secrets {
				objs = append(objs, s)
			}
			ct := fake.NewClientBuilder().WithObjects(objs...).Build()
			err := PrecheckPowerFlex(ctx, &tt.csm, config, ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}

func TestRemoveInitVolume(t *testing.T) {
	volumeName := SftpKeys
	differentVolumeName := "different-volume-name"
	containerName := "driver"
	tests := []struct {
		name          string
		configuration *v1.DaemonSetApplyConfiguration
		wantErr       bool
	}{
		{
			name: "Remove init volume and mount",
			configuration: &v1.DaemonSetApplyConfiguration{
				Spec: &v1.DaemonSetSpecApplyConfiguration{
					Template: &acorev1.PodTemplateSpecApplyConfiguration{
						Spec: &acorev1.PodSpecApplyConfiguration{
							Volumes: []acorev1.VolumeApplyConfiguration{
								{
									Name: &volumeName,
								},
							},
							InitContainers: []acorev1.ContainerApplyConfiguration{
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
			wantErr: false,
		},
		{
			name: "RemoveVolume called with a daemonset that doesn't include the volume",
			configuration: &v1.DaemonSetApplyConfiguration{
				Spec: &v1.DaemonSetSpecApplyConfiguration{
					Template: &acorev1.PodTemplateSpecApplyConfiguration{
						Spec: &acorev1.PodSpecApplyConfiguration{
							Volumes: []acorev1.VolumeApplyConfiguration{
								{
									Name: &differentVolumeName,
								},
							},
							InitContainers: []acorev1.ContainerApplyConfiguration{
								{
									Name: &containerName,
									VolumeMounts: []acorev1.VolumeMountApplyConfiguration{
										{
											Name: &differentVolumeName,
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:          "RemoveVolume called with nil daemonset",
			configuration: nil,
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RemoveInitVolume(tt.configuration, volumeName); (err != nil) != tt.wantErr {
				t.Errorf("RemoveVolume() error = %v, wantErr %v", err, tt.wantErr)
			}
			// check that the volume and volume mount were removed
			if !tt.wantErr {
				for i := range tt.configuration.Spec.Template.Spec.Volumes {
					assert.NotEqual(t, *tt.configuration.Spec.Template.Spec.Volumes[i].Name, volumeName)
				}
				for c := range tt.configuration.Spec.Template.Spec.Containers {
					for i := range tt.configuration.Spec.Template.Spec.Containers[c].VolumeMounts {
						assert.NotEqual(t, *tt.configuration.Spec.Template.Spec.Containers[c].VolumeMounts[i].Name, volumeName)
					}
				}
			}
		})
	}
}
