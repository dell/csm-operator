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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
			yamlString: "CSI_SDC_ENABLED=NEW_SDC_ENABLED CSI_APPROVE_SDC_ENABLED=NEW_APPROVE_SDC CSI_RENAME_SDC_ENABLED=NEW_RENAME_SDC CSI_PREFIX_RENAME_SDC=NEW_RENAME_PREFIX CSI_VXFLEXOS_MAX_VOLUMES_PER_NODE=NEW_MAX_VOLUMES CSI_HEALTH_MONITOR_ENABLED=NEW_HEALTH_MONITOR_NODE CSI_DEBUG=NEW_DEBUG",
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

func TestRemoveInitVolume(t *testing.T) {
	volumeName := ScaleioBinPath
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
