// Copyright © 2023-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"github.com/dell/csm-operator/pkg/constants"
	shared "github.com/dell/csm-operator/tests/sharedutil"
	"github.com/dell/csm-operator/tests/sharedutil/crclient"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	powerStoreCSM               = csmForPowerStore()
	powerStoreInvalidCSMVersion = csmForPowerStoreInvalidVersion()
	powerStoreCSMEmptyEnv       = csmForPowerStoreWithEmptyEnv()
	powerStoreCSMBadSkipCert    = csmForPowerStoreBadSkipCert()
	powerStoreCSMBadCertCnt     = csmForPowerStoreBadCertCnt()
	powerStoreCSMBadVersion     = csmForPowerStoreBadVersion()
	powerStoreObjects           = map[shared.StorageKey]runtime.Object{}
	powerStoreClient            = crclient.NewFakeClientNoInjector(powerStoreObjects)
	powerStoreSecret            = shared.MakeSecret("powerstore-config", "driver-test", shared.ConfigVersion)

	powerStoreTests = []struct {
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
		{"happy path", powerStoreCSM, powerStoreClient, powerStoreSecret, ""},
		{"invalid value for skip cert validation", powerStoreCSMBadSkipCert, powerStoreClient, powerStoreSecret, "invalid value for X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION"},
		{"invalid value for cert secret cnt", powerStoreCSMBadCertCnt, powerStoreClient, powerStoreSecret, "invalid value for CERT_SECRET_COUNT"},
	}

	preCheckPowerStoreTest = []struct {
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
		{"missing secret", powerStoreCSM, powerStoreClient, powerStoreSecret, "failed to find secret"},
		{"bad version", powerStoreCSMBadVersion, powerStoreClient, powerStoreSecret, "not supported"},
		{"missing envs", powerStoreCSMEmptyEnv, powerStoreClient, powerStoreSecret, "failed to find secret"},
		{"invalid csm version", powerStoreInvalidCSMVersion, powerStoreClient, powerStoreSecret, "No custom resource configuration is available for CSM version v1.10.0"},
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
			name:       "update metrics enabled and default port for Controller",
			yamlString: "<X_CSI_METRICS_ENABLED> <X_CSI_METRICS_PORT>",
			csm:        csmForPowerStoreMetrics(),
			ct:         powerStoreClient,
			sec:        powerStoreSecret,
			fileType:   "Controller",
			expected:   "true 8443",
		},
		{
			name:       "update metrics tls paths for Node",
			yamlString: constants.CsiMetricsTLSCertFile + " " + constants.CsiMetricsTLSKeyFile,
			csm:        csmForPowerStoreMetricsTLS(),
			ct:         powerStoreClient,
			sec:        powerStoreSecret,
			fileType:   "Node",
			expected:   "/etc/metrics-tls/tls.crt /etc/metrics-tls/tls.key",
		},
		{
			name:       "CSIDriverSpec with storage capacity enabled",
			yamlString: "<X_CSI_STORAGE_CAPACITY_ENABLED>",
			csm:        csmForPowerStoreWithStorageCapacity(),
			ct:         powerStoreClient,
			sec:        powerStoreSecret,
			fileType:   "CSIDriverSpec",
			expected:   "true",
		},
		{
			name:       "CSIDriverSpec with storage capacity disabled",
			yamlString: "<X_CSI_STORAGE_CAPACITY_ENABLED>",
			csm:        csmForPowerStoreWithStorageCapacityDisabled(),
			ct:         powerStoreClient,
			sec:        powerStoreSecret,
			fileType:   "CSIDriverSpec",
			expected:   "false",
		},
		{
			name:       "CSIDriverSpec nil: should use default false",
			yamlString: "<X_CSI_STORAGE_CAPACITY_ENABLED>",
			csm:        csmForPowerStore(),
			ct:         powerStoreClient,
			sec:        powerStoreSecret,
			fileType:   "CSIDriverSpec",
			expected:   "false",
		},
		{
			name:       "metrics with custom port for Controller",
			yamlString: "<X_CSI_METRICS_PORT>",
			csm:        csmForPowerStoreMetricsWithPort(),
			ct:         powerStoreClient,
			sec:        powerStoreSecret,
			fileType:   "Controller",
			expected:   "9090",
		},
		{
			name:       "metrics disabled for Node",
			yamlString: "<X_CSI_METRICS_ENABLED> <X_CSI_METRICS_PORT>",
			csm:        csmForPowerStore(),
			ct:         powerStoreClient,
			sec:        powerStoreSecret,
			fileType:   "Node",
			expected:   "false 8443",
		},
		{
			name:       "metrics with leader election enabled for Controller",
			yamlString: constants.CsiMetricsLeaderElectionEnabled,
			csm:        csmForPowerStoreMetricsWithLeaderElection(),
			ct:         powerStoreClient,
			sec:        powerStoreSecret,
			fileType:   "Controller",
			expected:   "true",
		},
		{
			name:       "nfs auto-select enabled for Node",
			yamlString: CsiPowerstoreNfsAutoSelect,
			csm:        csmForPowerStoreNfsAutoSelect("true"),
			ct:         powerStoreClient,
			sec:        powerStoreSecret,
			fileType:   "Node",
			expected:   "true",
		},
		{
			name:       "nfs auto-select enabled for Controller",
			yamlString: CsiPowerstoreNfsAutoSelect,
			csm:        csmForPowerStoreNfsAutoSelect("true"),
			ct:         powerStoreClient,
			sec:        powerStoreSecret,
			fileType:   "Controller",
			expected:   "true",
		},
		{
			name:       "nfs auto-select default false when not set",
			yamlString: CsiPowerstoreNfsAutoSelect,
			csm:        csmForPowerStore(),
			ct:         powerStoreClient,
			sec:        powerStoreSecret,
			fileType:   "Node",
			expected:   "false",
		},
	}
)

func TestGetApplyCertVolumePowerstore(t *testing.T) {
	for _, tt := range powerStoreTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getApplyCertVolumePowerstore(tt.csm)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

func TestPrecheckPowerStoreTLSCert(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		csm         csmv1.ContainerStorageModule
		secrets     []*corev1.Secret
		expectedErr string
	}{
		{
			name: "metrics tls cert secret exists",
			csm:  csmForPowerStoreMetricsTLS(),
			secrets: []*corev1.Secret{
				shared.MakeSecret("powerstore-config", "driver-test", shared.ConfigVersion),
				shared.MakeSecret("powerstore-metrics-tls", "driver-test", shared.ConfigVersion),
			},
			expectedErr: "",
		},
		{
			name: "metrics tls cert secret missing",
			csm:  csmForPowerStoreMetricsTLS(),
			secrets: []*corev1.Secret{
				shared.MakeSecret("powerstore-config", "driver-test", shared.ConfigVersion),
			},
			expectedErr: "failed to find metrics TLS secret powerstore-metrics-tls",
		},
		{
			name: "metrics disabled with tls cert secret set skips check",
			csm: func() csmv1.ContainerStorageModule {
				cr := csmForPowerStore()
				cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
					Enabled:       false,
					TLSCertSecret: "powerstore-metrics-tls",
				}
				return cr
			}(),
			secrets: []*corev1.Secret{
				shared.MakeSecret("powerstore-config", "driver-test", shared.ConfigVersion),
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
			err := PrecheckPowerStore(ctx, &tt.csm, config, ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}

func TestPrecheckPowerStore(t *testing.T) {
	ctx := context.Background()
	for _, tt := range preCheckPowerStoreTest {
		t.Run(tt.name, func(t *testing.T) { // #nosec G601 - Run waits for the call to complete.
			// Use configForVersionChecks for invalid CSM version test
			cfg := config
			if tt.name == "invalid csm version" {
				cfg = configForVersionChecks
			}
			err := PrecheckPowerStore(ctx, &tt.csm, cfg, tt.ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}

	// grab the first secret
	for _, tt := range powerStoreTests {
		// create secret for each run
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
			err := PrecheckPowerStore(ctx, &tt.csm, cfg, tt.ct)
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

// makes a csm object with tolerations
func csmForPowerStore() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add log level to cover some code in GetConfigMap
	envVarLogLevel1 := corev1.EnvVar{Name: "CERT_SECRET_COUNT", Value: "0"}
	envVarLogLevel2 := corev1.EnvVar{Name: "X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION", Value: "false"}
	envVarLogLevel3 := corev1.EnvVar{Name: "GOPOWERSTORE_DEBUG", Value: "false"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarLogLevel1, envVarLogLevel2, envVarLogLevel3}
	res.Spec.Driver.AuthSecret = "powerstore-config"

	// Add powerstore driver version
	res.Spec.Driver.ConfigVersion = shared.ConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore

	return res
}

func csmForPowerStoreWithEmptyEnv() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	res.Spec.Driver.Common.Envs = []corev1.EnvVar{}
	res.Spec.Driver.AuthSecret = "powerstore-config"

	// Add powerstore driver version
	res.Spec.Driver.ConfigVersion = shared.ConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore

	return res
}

// makes a csm object with tolerations
func csmForPowerStoreBadSkipCert() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add log level to cover some code in GetConfigMap
	envVarLogLevel1 := corev1.EnvVar{Name: "CERT_SECRET_COUNT", Value: "2"}
	envVarLogLevel2 := corev1.EnvVar{Name: "X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION", Value: "NotABool"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarLogLevel1, envVarLogLevel2}

	// Add powerstore driver version
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

	// Add powerstore driver version
	res.Spec.Driver.ConfigVersion = shared.ConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore

	return res
}

// makes a csm object with tolerations
func csmForPowerStoreBadVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add powerstore driver version
	res.Spec.Driver.ConfigVersion = "v0"
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore

	return res
}

// makes a csm object with tolerations
func csmForPowerStoreInvalidVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add powerstore driver version
	res.Spec.Version = shared.InvalidCSMVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore

	return res
}

func gopowerstoreDebug(debug string) csmv1.ContainerStorageModule {
	cr := csmForPowerStore()
	cr.Spec.Driver.Common.Envs = []corev1.EnvVar{
		{Name: "GOPOWERSTORE_DEBUG", Value: debug},
	}

	return cr
}

func csmForPowerStoreMetrics() csmv1.ContainerStorageModule {
	cr := csmForPowerStore()
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{Enabled: true}
	return cr
}

func csmForPowerStoreMetricsTLS() csmv1.ContainerStorageModule {
	cr := csmForPowerStore()
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled:       true,
		TLSCertSecret: "powerstore-metrics-tls",
	}
	return cr
}

func csmForPowerStoreWithStorageCapacity() csmv1.ContainerStorageModule {
	cr := csmForPowerStore()
	cr.Spec.Driver.CSIDriverSpec = &csmv1.CSIDriverSpec{
		StorageCapacity: true,
	}
	return cr
}

func csmForPowerStoreWithStorageCapacityDisabled() csmv1.ContainerStorageModule {
	cr := csmForPowerStore()
	cr.Spec.Driver.CSIDriverSpec = &csmv1.CSIDriverSpec{
		StorageCapacity: false,
	}
	return cr
}

func csmForPowerStoreMetricsWithPort() csmv1.ContainerStorageModule {
	cr := csmForPowerStore()
	customPort := int32(9090)
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled: true,
		Port:    customPort,
	}
	return cr
}

func csmForPowerStoreMetricsWithLeaderElection() csmv1.ContainerStorageModule {
	cr := csmForPowerStore()
	leaderElectionEnabled := true
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled: true,
		LeaderElection: &csmv1.LeaderElectionConfig{
			Enabled: &leaderElectionEnabled,
		},
	}
	return cr
}

func csmForPowerStoreNfsAutoSelect(value string) csmv1.ContainerStorageModule {
	cr := csmForPowerStore()
	nfsAutoSelectEnv := corev1.EnvVar{Name: "X_CSI_POWERSTORE_NFS_AUTO_SELECT", Value: value}
	if cr.Spec.Driver.Node == nil {
		cr.Spec.Driver.Node = &csmv1.ContainerTemplate{}
	}
	cr.Spec.Driver.Node.Envs = append(cr.Spec.Driver.Node.Envs, nfsAutoSelectEnv)
	if cr.Spec.Driver.Controller == nil {
		cr.Spec.Driver.Controller = &csmv1.ContainerTemplate{}
	}
	cr.Spec.Driver.Controller.Envs = append(cr.Spec.Driver.Controller.Envs, nfsAutoSelectEnv)
	return cr
}
