// Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	powerStoreCSM               = csmForPowerStore("csm")
	powerStoreCSMBadVersion     = csmForPowerStoreBadVersion()
	powerStoreInvalidCSMVersion = csmForPowerStoreInvalidVersion()
	powerStoreCSMBadCertCnt     = csmForPowerStoreBadCertCnt()
	powerStoreCSMEmptyEnv       = csmForPowerStoreWithEmptyEnv()
	powerStoreCSMBadSkipCert    = csmForPowerStoreBadSkipCert()
	powerStoreSkipCertFalse     = csmForPowerStoreSkipCertFalse()
	powerStoreClient            = crclient.NewFakeClientNoInjector(objects)
	configJSONFileGoodPStore    = fmt.Sprintf("%s/driverconfig/%s/config.json", config.ConfigDirectory, csmv1.PowerStore)
	powerStoreSecret            = shared.MakeSecretWithJSON("csm-config", "driver-test", configJSONFileGoodPStore)
	fakeSecretPstore            = shared.MakeSecret("fake-secret", "fake-ns", shared.PStoreConfigVersion)

	powerStoreTests = []struct {
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
		{"happy path", powerStoreCSM, powerStoreClient, powerStoreSecret, ""},
		{"bad version", powerStoreCSMBadVersion, powerStoreClient, powerStoreSecret, "not supported"},
		{"invalid csm version", powerStoreInvalidCSMVersion, powerStoreClient, powerStoreSecret, "No custom resource configuration is available for CSM version v1.10.0"},
	}

	powerStoreCertsVolumeTests = []struct {
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
		{"invalid value for skip cert validation", powerStoreCSMBadSkipCert, powerStoreClient, powerStoreSecret, "is an invalid value for X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION"},
		{"invalid value for cert secret cnt", powerStoreCSMBadCertCnt, powerStoreClient, powerStoreSecret, "is an invalid value for CERT_SECRET_COUNT"},
		{"skip cert false", powerStoreSkipCertFalse, powerStoreClient, powerStoreSecret, ""},
		{"common is nil", powerStoreCSMCommonNil, powerStoreClient, powerStoreSecret, ""},
		{"common env is empty", powerStoreCSMCommonEnvEmpty, powerStoreClient, powerStoreSecret, ""},
	}

	powerStorePrecheckTests = []struct {
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
		{"missing secret", powerStoreCSM, powerStoreClient, fakeSecretPstore, "failed to find secret"},
		{"bad version", powerStoreCSMBadVersion, powerStoreClient, fakeSecretPstore, "not supported"},
		{"missing envs", powerStoreCSMEmptyEnv, powerStoreClient, fakeSecretPstore, "failed to find secret"},
		{"invalid skip cert validation", csmForPowerStoreBadSkipCert(), powerStoreClient, fakeSecretPstore, "invalid value for X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION"},
		{"invalid cert secret count", csmForPowerStoreBadCertCnt(), powerStoreClient, fakeSecretPstore, "invalid value for CERT_SECRET_COUNT"},
		{"valid skip cert validation", csmForPowerStoreGoodSkipCert(), powerStoreClient, fakeSecretPstore, "failed to find secret csm-creds"},
	}

	powerStoreCommonEnvTest = []struct {
		name       string
		yamlString string
		csm        csmv1.ContainerStorageModule
		ct         client.Client
		sec        *corev1.Secret
		fileType   string
		expected   string
	}{
		{
			name:       "update GOPOWERSTORE_DEBUG value for Controller",
			yamlString: "<GOPOWERSTORE_DEBUG>",
			csm:        gopowerstoreDebug("true"),
			ct:         powerStoreClient,
			sec:        powerStoreSecret,
			fileType:   "Controller",
			expected:   "true",
		},
		{
			name:     "when auth module is enabled",
			csm:      enableAuthModule(),
			ct:       powerStoreClient,
			sec:      powerStoreSecret,
			fileType: "Node",
		},
		{
			name:     "when auth module is disabled",
			csm:      disableAuthModule(),
			ct:       powerStoreClient,
			sec:      powerStoreSecret,
			fileType: "Node",
		},
		{
			name:     "when node object is nil",
			csm:      getNilNodeObject(),
			ct:       powerStoreClient,
			sec:      powerStoreSecret,
			fileType: "Node",
		},
		{
			name:     "when node object is not nil and Env. is nil",
			csm:      getNilEnvObject(),
			ct:       powerStoreClient,
			sec:      powerStoreSecret,
			fileType: "Node",
		},
		{
			name:     "when auth module env. is set to true",
			csm:      setAuthModuleEnv("true"),
			ct:       powerStoreClient,
			sec:      powerStoreSecret,
			fileType: "Node",
		},
		{
			name:     "update existing auth module env. to false",
			csm:      setAuthModuleEnv("false"),
			ct:       powerStoreClient,
			sec:      powerStoreSecret,
			fileType: "Node",
		},
		{
			name:       "update GOPOWERSTORE_DEBUG value for Node",
			yamlString: "<GOPOWERSTORE_DEBUG>",
			csm:        gopowerstoreDebug("true"),
			ct:         powerStoreClient,
			sec:        powerStoreSecret,
			fileType:   "Node",
			expected:   "true",
		},
		{
			name: "update Powerstore API and Podmon connectivity timeout for Node",
			yamlString: `
			- name: X_CSI_POWERSTORE_API_TIMEOUT
		      value: "<X_CSI_POWERSTORE_API_TIMEOUT>"
		    - name: X_CSI_PODMON_ARRAY_CONNECTIVITY_TIMEOUT
		      value: "<X_CSI_PODMON_ARRAY_CONNECTIVITY_TIMEOUT>"`,
			csm:      csmForPowerStore("csm"),
			ct:       powerStoreClient,
			sec:      powerStoreSecret,
			fileType: "Node",
			expected: `
			- name: X_CSI_POWERSTORE_API_TIMEOUT
		      value: "120s"
		    - name: X_CSI_PODMON_ARRAY_CONNECTIVITY_TIMEOUT
		      value: "10s"`,
		},
		{
			name: "update Powerstore API and Podmon connectivity timeout for Controller",
			yamlString: `
			- name: X_CSI_POWERSTORE_API_TIMEOUT
		      value: "<X_CSI_POWERSTORE_API_TIMEOUT>"
		    - name: X_CSI_PODMON_ARRAY_CONNECTIVITY_TIMEOUT
		      value: "<X_CSI_PODMON_ARRAY_CONNECTIVITY_TIMEOUT>"`,
			csm:      csmForPowerStore("csm"),
			ct:       powerStoreClient,
			sec:      powerStoreSecret,
			fileType: "Controller",
			expected: `
			- name: X_CSI_POWERSTORE_API_TIMEOUT
		      value: "120s"
		    - name: X_CSI_PODMON_ARRAY_CONNECTIVITY_TIMEOUT
		      value: "10s"`,
		},
	}
)

func csmForPowerStoreGoodSkipCert() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Valid environment variables
	envCertCount := corev1.EnvVar{
		Name:  "CERT_SECRET_COUNT",
		Value: "1", // Ensures the loop runs at least once
	}
	envSkipCertValidation := corev1.EnvVar{
		Name:  "X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION",
		Value: "true", // Valid boolean string
	}

	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envCertCount, envSkipCertValidation}
	res.Spec.Driver.AuthSecret = "csm-creds"
	res.Spec.Driver.ConfigVersion = shared.ConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore

	return res
}

func csmForPowerStoreMultipleCertSecrets() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.PStoreConfigVersion)
	// Add log level to cover some code in GetConfigMap
	envVarLogLevel1 := corev1.EnvVar{Name: "CERT_SECRET_COUNT", Value: "2"}
	envVarLogLevel2 := corev1.EnvVar{Name: "X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION", Value: "false"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarLogLevel1, envVarLogLevel2}
	// Add powerstore driver version
	res.Spec.Driver.ConfigVersion = shared.PStoreConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore
	return res
}

func csmForPowerStoreWithEmptyEnv() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	res.Spec.Driver.Common.Envs = []corev1.EnvVar{}
	res.Spec.Driver.AuthSecret = "csm-creds"

	// Add pscale driver version
	res.Spec.Driver.ConfigVersion = shared.ConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerScale

	return res
}

var powerStoreCSMCommonEnvEmpty = csmv1.ContainerStorageModule{
	ObjectMeta: metav1.ObjectMeta{
		Name: "test-csm",
	},
	Spec: csmv1.ContainerStorageModuleSpec{
		Driver: csmv1.Driver{
			Common: &csmv1.ContainerTemplate{
				Envs: []corev1.EnvVar{
					{},
				},
			},
		},
	},
}

var powerStoreCSMSkipCertInvalid = csmv1.ContainerStorageModule{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "powerstore",
		Namespace: "default",
	},
	Spec: csmv1.ContainerStorageModuleSpec{
		Driver: csmv1.Driver{
			ConfigVersion: "v1",
			Common: &csmv1.ContainerTemplate{
				Envs: []corev1.EnvVar{
					{
						Name:  "X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION",
						Value: "notabool",
					},
				},
			},
		},
	},
}

func csmForPowerStoreSkipCertFalse() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add log level to cover some code in GetConfigMap
	envVarLogLevel1 := corev1.EnvVar{Name: "CERT_SECRET_COUNT", Value: "2"}
	envVarLogLevel2 := corev1.EnvVar{Name: "X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION", Value: "false"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarLogLevel1, envVarLogLevel2}

	// Add powerstore driver version
	res.Spec.Driver.ConfigVersion = shared.ConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore

	return res
}

func csmForPowerStoreSkipCertTrue() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add valid env vars
	envVarCertCount := corev1.EnvVar{Name: "CERT_SECRET_COUNT", Value: "2"}
	envVarSkipCert := corev1.EnvVar{Name: "X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION", Value: "true"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarCertCount, envVarSkipCert}

	// Set PowerStore driver type and config version
	res.Spec.Driver.ConfigVersion = shared.ConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore

	return res
}

var powerStoreCSMCommonNil = csmv1.ContainerStorageModule{
	ObjectMeta: metav1.ObjectMeta{
		Name: "test-csm",
	},
	Spec: csmv1.ContainerStorageModuleSpec{
		Driver: csmv1.Driver{
			Common: nil,
		},
	},
}

func TestPrecheckPowerStore(t *testing.T) {
	ctx := context.Background()
	for _, tt := range powerStorePrecheckTests {
		t.Run(tt.name, func(t *testing.T) { // #nosec G601 - Run waits for the call to complete.
			err := PrecheckPowerStore(ctx, &tt.csm, config, tt.ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}

	for _, tt := range powerStoreTests {
		err := tt.ct.Create(ctx, tt.sec)
		if err != nil {
			assert.Nil(t, err)
		}
		t.Run(tt.name, func(t *testing.T) { // #nosec G601 - Run waits for the call to complete.
			err := PrecheckPowerStore(ctx, &tt.csm, config, tt.ct)
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

func TestModifyPowerstoreCR(t *testing.T) {
	for _, tt := range powerStoreCommonEnvTest {
		t.Run(tt.name, func(t *testing.T) {
			result := ModifyPowerstoreCR(tt.yamlString, tt.csm, tt.fileType)
			if result != tt.expected {
				t.Errorf("expected %v, but got %v", tt.expected, result)
			}
		})
	}
}

// makes a csm object with a bad version
func csmForPowerStoreBadVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.PStoreConfigVersion)

	// Add pstore driver version
	res.Spec.Driver.ConfigVersion = shared.BadConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore

	return res
}

// makes a csm object with a invalid csm version
func csmForPowerStoreInvalidVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.PStoreConfigVersion)

	// Add pstore driver version
	res.Spec.Version = shared.InvalidCSMVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore

	return res
}

// makes a csm object
func csmForPowerStore(customCSMName string) csmv1.ContainerStorageModule {
	res := shared.MakeCSM(customCSMName, "driver-test", shared.PStoreConfigVersion)
	res.Spec.Driver.AuthSecret = "csm-config"

	// Add pstore driver version
	res.Spec.Driver.ConfigVersion = shared.PStoreConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore

	return res
}

func gopowerstoreDebug(debug string) csmv1.ContainerStorageModule {
	cr := csmForPowerStore("csm")
	cr.Spec.Driver.Common.Envs = []corev1.EnvVar{
		{Name: "GOPOWERSTORE_DEBUG", Value: debug},
	}

	return cr
}

func enableAuthModule() csmv1.ContainerStorageModule {
	cr := csmForPowerStore("csm")
	cr.Spec.Modules = []csmv1.Module{
		{
			Name:    csmv1.Authorization,
			Enabled: true,
		},
	}
	cr.Spec.Driver.Node.Envs = append(cr.Spec.Driver.Node.Envs, corev1.EnvVar{Name: "X_CSM_AUTH_ENABLED", Value: "true"})
	return cr
}

func disableAuthModule() csmv1.ContainerStorageModule {
	cr := csmForPowerStore("csm")
	cr.Spec.Modules = []csmv1.Module{
		{
			Name:    csmv1.Authorization,
			Enabled: false,
		},
	}
	return cr
}

func getNilNodeObject() csmv1.ContainerStorageModule {
	cr := csmForPowerStore("csm")
	cr.Spec.Driver.Node = nil
	return cr
}

func getNilEnvObject() csmv1.ContainerStorageModule {
	cr := csmForPowerStore("csm")
	cr.Spec.Driver.Node.Envs = nil
	return cr
}

func setAuthModuleEnv(value string) csmv1.ContainerStorageModule {
	cr := csmForPowerStore("csm")
	cr.Spec.Driver.Node.Envs = append(cr.Spec.Driver.Node.Envs, corev1.EnvVar{Name: "X_CSM_AUTH_ENABLED", Value: value})
	return cr
}

func csmForPowerStoreBadSkipCert() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add log level to cover some code in GetConfigMap
	envVarLogLevel1 := corev1.EnvVar{Name: "CERT_SECRET_COUNT", Value: "2"}
	envVarLogLevel2 := corev1.EnvVar{Name: "X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION", Value: "NotABool"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarLogLevel1, envVarLogLevel2}

	// Add pscale driver version
	res.Spec.Driver.ConfigVersion = shared.ConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore

	return res
}

// makes a csm object with tolerations
func csmForPowerStoreBadCertCnt() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add log level to cover some code in GetConfigMap
	envVarLogLevel1 := corev1.EnvVar{Name: "CERT_SECRET_COUNT", Value: "thisIsNotANumber"}
	envVarLogLevel2 := corev1.EnvVar{Name: "X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION", Value: "true"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarLogLevel1, envVarLogLevel2}

	// Add pscale driver version
	res.Spec.Driver.ConfigVersion = shared.ConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore

	return res
}

func TestGetApplyCertVolumePowerStore(t *testing.T) {
	for _, tt := range powerStoreCertsVolumeTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getApplyCertVolumePowerstore(tt.csm)
			t.Logf("Expected error: %q, Actual error: %v", tt.expectedErr, err)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}
