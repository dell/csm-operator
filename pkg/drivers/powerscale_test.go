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
	"os"
	"strings"
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
	powerScaleCSM               = csmForPowerScale()
	powerScaleInvalidCSMVersion = csmForPowerScaleInvalidVersion()
	powerScaleCSMEmptyEnv       = csmForPowerScaleWithEmptyEnv()
	powerScaleCSMBadSkipCert    = csmForPowerScaleBadSkipCert()
	powerScaleCSMBadCertCnt     = csmForPowerScaleBadCertCnt()
	powerScaleCSMBadVersion     = csmForPowerScaleBadVersion()
	objects                     = map[shared.StorageKey]runtime.Object{}
	powerScaleClient            = crclient.NewFakeClientNoInjector(objects)
	powerScaleSecret            = shared.MakeSecret("csm-creds", "driver-test", shared.ConfigVersion)

	powerScaleTests = []struct {
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
		{"happy path", powerScaleCSM, powerScaleClient, powerScaleSecret, ""},
		{"invalid value for skip cert validation", powerScaleCSMBadSkipCert, powerScaleClient, powerScaleSecret, "is an invalid value for X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION"},
		{"invalid value for cert secret cnt", powerScaleCSMBadCertCnt, powerScaleClient, powerScaleSecret, "is an invalid value for CERT_SECRET_COUNT"},
	}

	preCheckPowerScaleTest = []struct {
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
		{"missing secret", powerScaleCSM, powerScaleClient, powerScaleSecret, "failed to find secret"},
		{"bad version", powerScaleCSMBadVersion, powerScaleClient, powerScaleSecret, "not supported"},
		{"missing envs", powerScaleCSMEmptyEnv, powerScaleClient, powerScaleSecret, "failed to find secret"},
		{"invalid csm version", powerScaleInvalidCSMVersion, powerScaleClient, powerScaleSecret, "No custom resource configuration is available for CSM version v1.10.0"},
	}

	powerScaleCommonEnvTest = []struct {
		name       string
		yamlString string
		csm        csmv1.ContainerStorageModule
		ct         client.Client
		sec        *corev1.Secret
		fileType   string
		expected   string
	}{
		{
			name:       "update GOISILON_DEBUG value for Controller",
			yamlString: "<GOISILON_DEBUG>",
			csm:        goisilonDebug("true"),
			ct:         powerScaleClient,
			sec:        powerScaleSecret,
			fileType:   "Controller",
			expected:   "true",
		},
		{
			name:       "update GOISILON_DEBUG value for Node",
			yamlString: "<GOISILON_DEBUG>",
			csm:        goisilonDebug("true"),
			ct:         powerScaleClient,
			sec:        powerScaleSecret,
			fileType:   "Node",
			expected:   "true",
		},
		{
			name:       "update metrics enabled and default port for Controller",
			yamlString: "<X_CSI_METRICS_ENABLED> <X_CSI_METRICS_PORT>",
			csm:        csmForPowerScaleMetrics(),
			ct:         powerScaleClient,
			sec:        powerScaleSecret,
			fileType:   "Controller",
			expected:   "true 8443",
		},
		{
			name:       "metrics omitted renders disabled flag by default",
			yamlString: "<X_CSI_METRICS_ENABLED> <X_CSI_METRICS_PORT>",
			csm:        csmForPowerScale(),
			ct:         powerScaleClient,
			sec:        powerScaleSecret,
			fileType:   "Controller",
			expected:   "false 8443",
		},
		{
			name:       "metrics explicitly disabled renders disabled flag",
			yamlString: "<X_CSI_METRICS_ENABLED> <X_CSI_METRICS_PORT>",
			csm:        csmForPowerScaleMetricsDisabled(),
			ct:         powerScaleClient,
			sec:        powerScaleSecret,
			fileType:   "Node",
			expected:   "false 8443",
		},
		{
			name:       "update metrics tls paths for Node",
			yamlString: constants.CsiMetricsTLSCertFile + " " + constants.CsiMetricsTLSKeyFile,
			csm:        csmForPowerScaleMetricsTLS(),
			ct:         powerScaleClient,
			sec:        powerScaleSecret,
			fileType:   "Node",
			expected:   "/etc/metrics-tls/tls.crt /etc/metrics-tls/tls.key",
		},
		{
			name:       "structured metrics defaults are rendered when metrics enabled",
			yamlString: constants.CsiMetricsCollectionInterval + " " + constants.CsiMetricsCollectionCacheTTL + " " + constants.CsiMetricsArrayRateLimit + " " + constants.CsiMetricsArrayTimeout + " " + constants.CsiMetricsArrayCBThreshold + " " + constants.CsiMetricsArrayCBResetTimeout,
			csm:        csmForPowerScaleMetrics(),
			ct:         powerScaleClient,
			sec:        powerScaleSecret,
			fileType:   "Controller",
			expected:   "30s 25s 100 30s 3 30s",
		},
		{
			name:       "structured metrics partial config renders provided and default values",
			yamlString: constants.CsiMetricsCollectionInterval + " " + constants.CsiMetricsCollectionCacheTTL + " " + constants.CsiMetricsArrayRateLimit + " " + constants.CsiMetricsArrayTimeout + " " + constants.CsiMetricsArrayCBThreshold + " " + constants.CsiMetricsArrayCBResetTimeout,
			csm:        csmForPowerScaleMetricsPartialStructured(),
			ct:         powerScaleClient,
			sec:        powerScaleSecret,
			fileType:   "Node",
			expected:   "45s 25s 100 30s 3 30s",
		},
		{
			name:       "structured metrics explicit config renders expected values",
			yamlString: constants.CsiMetricsCollectionInterval + " " + constants.CsiMetricsCollectionCacheTTL + " " + constants.CsiMetricsArrayRateLimit + " " + constants.CsiMetricsArrayTimeout + " " + constants.CsiMetricsArrayCBThreshold + " " + constants.CsiMetricsArrayCBResetTimeout,
			csm:        csmForPowerScaleMetricsStructured(),
			ct:         powerScaleClient,
			sec:        powerScaleSecret,
			fileType:   "Controller",
			expected:   "1m 40s 250 45s 5 2m",
		},
		{
			name:       "invalid structured metrics values fall back to defaults",
			yamlString: constants.CsiMetricsCollectionInterval + " " + constants.CsiMetricsCollectionCacheTTL + " " + constants.CsiMetricsArrayRateLimit + " " + constants.CsiMetricsArrayTimeout + " " + constants.CsiMetricsArrayCBThreshold + " " + constants.CsiMetricsArrayCBResetTimeout,
			csm:        csmForPowerScaleMetricsStructuredInvalid(),
			ct:         powerScaleClient,
			sec:        powerScaleSecret,
			fileType:   "Controller",
			expected:   "30s 25s 100 30s 3 30s",
		},
		{
			name:       "leader election defaults are rendered when metrics enabled",
			yamlString: "<X_CSI_METRICS_LEADER_ELECTION_ENABLED> <X_CSI_METRICS_LEADER_ELECTION_LEASE_DURATION> <X_CSI_METRICS_LEADER_ELECTION_RENEW_DEADLINE> <X_CSI_METRICS_LEADER_ELECTION_RETRY_PERIOD>",
			csm:        csmForPowerScaleMetrics(),
			ct:         powerScaleClient,
			sec:        powerScaleSecret,
			fileType:   "Controller",
			expected:   "false 60s 40s 5s",
		},
		{
			name:       "leader election explicit values are rendered",
			yamlString: "<X_CSI_METRICS_LEADER_ELECTION_ENABLED> <X_CSI_METRICS_LEADER_ELECTION_LEASE_DURATION> <X_CSI_METRICS_LEADER_ELECTION_RENEW_DEADLINE> <X_CSI_METRICS_LEADER_ELECTION_RETRY_PERIOD>",
			csm:        csmForPowerScaleMetricsWithLeaderElection(),
			ct:         powerScaleClient,
			sec:        powerScaleSecret,
			fileType:   "Node",
			expected:   "true 60s 40s 10s",
		},
		{
			name:       "invalid leader election values fall back to defaults",
			yamlString: "<X_CSI_METRICS_LEADER_ELECTION_ENABLED> <X_CSI_METRICS_LEADER_ELECTION_LEASE_DURATION> <X_CSI_METRICS_LEADER_ELECTION_RENEW_DEADLINE> <X_CSI_METRICS_LEADER_ELECTION_RETRY_PERIOD>",
			csm:        csmForPowerScaleMetricsWithInvalidLeaderElection(),
			ct:         powerScaleClient,
			sec:        powerScaleSecret,
			fileType:   "Controller",
			expected:   "true 60s 40s 5s",
		},
		{
			name:       "leader election with nil enabled stays disabled",
			yamlString: "<X_CSI_METRICS_LEADER_ELECTION_ENABLED> <X_CSI_METRICS_LEADER_ELECTION_LEASE_DURATION> <X_CSI_METRICS_LEADER_ELECTION_RENEW_DEADLINE> <X_CSI_METRICS_LEADER_ELECTION_RETRY_PERIOD>",
			csm:        csmForPowerScaleMetricsWithLeaderElectionNilEnabled(),
			ct:         powerScaleClient,
			sec:        powerScaleSecret,
			fileType:   "Controller",
			expected:   "false 60s 40s 5s",
		},
	}
)

func TestGetApplyCertVolume(t *testing.T) {
	for _, tt := range powerScaleTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getApplyCertVolume(tt.csm)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

func TestMetricsLeaderElectionDurationDefaultsOrSanitized(t *testing.T) {
	defaultLease := "60s"
	defaultRenew := "40s"
	defaultRetry := "5s"

	tests := []struct {
		name          string
		lease         string
		renew         string
		retry         string
		expectedLease string
		expectedRenew string
		expectedRetry string
	}{
		{
			name:          "valid durations are preserved",
			lease:         "45s",
			renew:         "30s",
			retry:         "10s",
			expectedLease: "45s",
			expectedRenew: "30s",
			expectedRetry: "10s",
		},
		{
			name:          "empty values use defaults",
			lease:         "",
			renew:         "",
			retry:         "",
			expectedLease: defaultLease,
			expectedRenew: defaultRenew,
			expectedRetry: defaultRetry,
		},
		{
			name:          "invalid duration value falls back to default for that field only",
			lease:         "bad",
			renew:         "30s",
			retry:         "10s",
			expectedLease: defaultLease,
			expectedRenew: "30s",
			expectedRetry: "10s",
		},
		{
			name:          "non-positive durations fall back to defaults for those fields only",
			lease:         "0s",
			renew:         "-1s",
			retry:         "10s",
			expectedLease: defaultLease,
			expectedRenew: defaultRenew,
			expectedRetry: "10s",
		},
		{
			name:          "lease less than renew falls back to defaults",
			lease:         "40s",
			renew:         "40s",
			retry:         "5s",
			expectedLease: defaultLease,
			expectedRenew: defaultRenew,
			expectedRetry: defaultRetry,
		},
		{
			name:          "lease smaller than renew falls back to defaults",
			lease:         "40s",
			renew:         "60s",
			retry:         "5s",
			expectedLease: defaultLease,
			expectedRenew: defaultRenew,
			expectedRetry: defaultRetry,
		},
		{
			name:          "renew less than or equal retry falls back to defaults",
			lease:         "60s",
			renew:         "10s",
			retry:         "10s",
			expectedLease: defaultLease,
			expectedRenew: defaultRenew,
			expectedRetry: defaultRetry,
		},
		{
			name:          "retry greater than renew falls back to defaults",
			lease:         "60s",
			renew:         "30s",
			retry:         "35s",
			expectedLease: defaultLease,
			expectedRenew: defaultRenew,
			expectedRetry: defaultRetry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lease, renew, retry := metricsLeaderElectionDurationDefaultsOrSanitized(tt.lease, tt.renew, tt.retry, defaultLease, defaultRenew, defaultRetry)
			assert.Equal(t, tt.expectedLease, lease)
			assert.Equal(t, tt.expectedRenew, renew)
			assert.Equal(t, tt.expectedRetry, retry)
		})
	}
}

// TestMetricsLeaderElectionConstraintViolationsUseDefaults covers the specific
// constraint-violation scenarios that were failing in integration tests
// (scenarios 8.11, 8.17, 8.18, 8.24 from the test suite).  Each of these
// scenarios supplies syntactically-valid durations that violate the required
// relationship lease > renew > retry; the operator must substitute all three
// defaults rather than emitting the invalid combination.
func TestMetricsLeaderElectionConstraintViolationsUseDefaults(t *testing.T) {
	defaultLease := "60s"
	defaultRenew := "40s"
	defaultRetry := "5s"

	tests := []struct {
		name  string
		lease string
		renew string
		retry string
	}{
		// Scenario 8.11 / 8.17: lease <= renew (40s <= 60s)
		{name: "lease equal to renew (scenario 8.11)", lease: "40s", renew: "60s", retry: "5s"},
		{name: "lease less than renew (scenario 8.17)", lease: "40s", renew: "60s", retry: "5s"},
		// Scenario 8.18 / 8.24: renew <= retry (40s <= 40s)
		{name: "renew equal to retry (scenario 8.18 / 8.24)", lease: "60s", renew: "40s", retry: "40s"},
		// Additional edge case: all three equal
		{name: "all durations equal", lease: "40s", renew: "40s", retry: "40s"},
		// lease defaults to 60s, but renew=70s > 60s → constraint violation
		{name: "invalid lease defaults and still violates constraint", lease: "bad", renew: "70s", retry: "5s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lease, renew, retry := metricsLeaderElectionDurationDefaultsOrSanitized(
				tt.lease, tt.renew, tt.retry, defaultLease, defaultRenew, defaultRetry,
			)
			// On any constraint violation, the operator must return all three defaults.
			assert.Equal(t, defaultLease, lease, "lease must be default on constraint violation")
			assert.Equal(t, defaultRenew, renew, "renew must be default on constraint violation")
			assert.Equal(t, defaultRetry, retry, "retry must be default on constraint violation")
		})
	}
}

// TestModifyPowerScaleCR_LeaderElectionConstraintViolationUsesDefaults verifies
// that ModifyPowerScaleCR substitutes default leader-election values in the
// rendered YAML when the user-supplied values violate the required relationship.
// This ensures the constraint check propagates all the way to the YAML that is
// applied to Kubernetes, not just to the sanitisation helper.
func TestModifyPowerScaleCR_LeaderElectionConstraintViolationUsesDefaults(t *testing.T) {
	yamlTemplate := "<X_CSI_METRICS_LEADER_ELECTION_ENABLED> " +
		"<X_CSI_METRICS_LEADER_ELECTION_LEASE_DURATION> " +
		"<X_CSI_METRICS_LEADER_ELECTION_RENEW_DEADLINE> " +
		"<X_CSI_METRICS_LEADER_ELECTION_RETRY_PERIOD>"

	tests := []struct {
		name     string
		csm      csmv1.ContainerStorageModule
		fileType string
		// expected output uses the defaults (60s, 40s, 5s) for all three durations.
	}{
		{
			name:     "lease <= renew in Controller YAML uses defaults",
			csm:      csmForPowerScaleMetricsWithInvalidLeaderElection(),
			fileType: "Controller",
		},
		{
			name:     "lease <= renew in Node YAML uses defaults",
			csm:      csmForPowerScaleMetricsWithInvalidLeaderElection(),
			fileType: "Node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ModifyPowerScaleCR(yamlTemplate, tt.csm, tt.fileType)
			// leader-election is enabled in csmForPowerScaleMetricsWithInvalidLeaderElection
			assert.Equal(t, "true 60s 40s 5s", result,
				"constraint-violating leader election must be replaced with defaults in YAML")
		})
	}
}

// TestModifyPowerScaleCR_LeaderElectionEmptyLeaseDurationPreservesOtherValues verifies
// the scenario that failed in the operator-based integration test: when leaseDuration
// is omitted, the operator should default leaseDuration to 60s but preserve the
// user-supplied renewDeadline and retryPeriod values.
func TestModifyPowerScaleCR_LeaderElectionEmptyLeaseDurationPreservesOtherValues(t *testing.T) {
	yamlTemplate := "<X_CSI_METRICS_LEADER_ELECTION_ENABLED> " +
		"<X_CSI_METRICS_LEADER_ELECTION_LEASE_DURATION> " +
		"<X_CSI_METRICS_LEADER_ELECTION_RENEW_DEADLINE> " +
		"<X_CSI_METRICS_LEADER_ELECTION_RETRY_PERIOD>"

	result := ModifyPowerScaleCR(yamlTemplate, csmForPowerScaleMetricsWithEmptyLeaseDuration(), "Controller")

	assert.Equal(t, "true 60s 40s 10s", result,
		"empty leaseDuration must default to 60s without changing renewDeadline or retryPeriod")
}

func TestPrecheckPowerScaleTLSCert(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		csm         csmv1.ContainerStorageModule
		secrets     []*corev1.Secret
		expectedErr string
	}{
		{
			name: "metrics tls cert secret exists",
			csm:  csmForPowerScaleMetricsTLS(),
			secrets: []*corev1.Secret{
				shared.MakeSecret("csm-creds", "driver-test", shared.ConfigVersion),
				shared.MakeSecret("pscale-metrics-tls", "driver-test", shared.ConfigVersion),
			},
			expectedErr: "",
		},
		{
			name: "metrics tls cert secret missing",
			csm:  csmForPowerScaleMetricsTLS(),
			secrets: []*corev1.Secret{
				shared.MakeSecret("csm-creds", "driver-test", shared.ConfigVersion),
			},
			expectedErr: "failed to find secret pscale-metrics-tls",
		},
		{
			name: "metrics disabled with tls cert secret set skips check",
			csm: func() csmv1.ContainerStorageModule {
				cr := csmForPowerScale()
				cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{ //nolint:gosec
					Enabled:       false,
					TLSCertSecret: "pscale-metrics-tls",
				}
				return cr
			}(),
			secrets: []*corev1.Secret{
				shared.MakeSecret("csm-creds", "driver-test", shared.ConfigVersion),
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
			err := PrecheckPowerScale(ctx, &tt.csm, config, ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}

func TestPrecheckPowerScale(t *testing.T) {
	ctx := context.Background()
	for _, tt := range preCheckPowerScaleTest {
		t.Run(tt.name, func(t *testing.T) { // #nosec G601 - Run waits for the call to complete.
			// Use configForVersionChecks for invalid CSM version test
			cfg := config
			if tt.name == "invalid csm version" {
				cfg = configForVersionChecks
			}
			err := PrecheckPowerScale(ctx, &tt.csm, cfg, tt.ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}

	// grab the first secret

	for _, tt := range powerScaleTests {
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
			err := PrecheckPowerScale(ctx, &tt.csm, cfg, tt.ct)
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

func TestModifyPowerScaleCR(t *testing.T) {
	for _, tt := range powerScaleCommonEnvTest {
		t.Run(tt.name, func(t *testing.T) {
			result := ModifyPowerScaleCR(tt.yamlString, tt.csm, tt.fileType)
			if result != tt.expected {
				t.Errorf("expected %v, but got %v", tt.expected, result)
			}
		})
	}
}

func TestModifyPowerScaleCR_SidecarHTTPEndpoints(t *testing.T) {
	yamlTemplate := strings.Join([]string{
		PowerScaleResizerHTTPEndpointArg,
		PowerScaleAttacherHTTPEndpointArg,
		PowerScaleProvisionerHTTPEndpointArg,
		PowerScaleSnapshotterHTTPEndpointArg,
	}, "\n")

	resultWhenMetricsEnabled := ModifyPowerScaleCR(yamlTemplate, csmForPowerScaleMetrics(), "Controller")
	assert.Contains(t, resultWhenMetricsEnabled, "--http-endpoint=:8081")
	assert.Contains(t, resultWhenMetricsEnabled, "--http-endpoint=:8082")
	assert.Contains(t, resultWhenMetricsEnabled, "--http-endpoint=:8083")
	assert.Contains(t, resultWhenMetricsEnabled, "--http-endpoint=:8084")

	resultWhenMetricsDisabled := ModifyPowerScaleCR(yamlTemplate, csmForPowerScaleMetricsDisabled(), "Controller")
	assert.NotContains(t, resultWhenMetricsDisabled, "--http-endpoint=")
}

func TestModifyPowerScaleCR_HealthMonitorHTTPEndpointAlwaysPresent(t *testing.T) {
	yamlTemplate := "- \"--http-endpoint=:8080\"\n" + PowerScaleResizerHTTPEndpointArg

	result := ModifyPowerScaleCR(yamlTemplate, csmForPowerScaleMetricsDisabled(), "Controller")
	assert.Contains(t, result, "--http-endpoint=:8080")
	assert.NotContains(t, result, "--http-endpoint=:8082")
}

// makes a csm object with tolerations
func csmForPowerScale() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add log level to cover some code in GetConfigMap
	envVarLogLevel1 := corev1.EnvVar{Name: "CERT_SECRET_COUNT", Value: "0"}
	envVarLogLevel2 := corev1.EnvVar{Name: "X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION", Value: "false"}
	envVarLogLevel3 := corev1.EnvVar{Name: "GOISILON_DEBUG", Value: "false"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarLogLevel1, envVarLogLevel2, envVarLogLevel3}
	res.Spec.Driver.AuthSecret = "csm-creds"

	// Add pscale driver version
	res.Spec.Driver.ConfigVersion = shared.ConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerScale

	return res
}

func csmForPowerScaleWithEmptyEnv() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	res.Spec.Driver.Common.Envs = []corev1.EnvVar{}
	res.Spec.Driver.AuthSecret = "csm-creds"

	// Add pscale driver version
	res.Spec.Driver.ConfigVersion = shared.ConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerScale

	return res
}

// makes a csm object with tolerations
func csmForPowerScaleBadSkipCert() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add log level to cover some code in GetConfigMap
	envVarLogLevel1 := corev1.EnvVar{Name: "CERT_SECRET_COUNT", Value: "2"}
	envVarLogLevel2 := corev1.EnvVar{Name: "X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION", Value: "NotABool"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarLogLevel1, envVarLogLevel2}

	// Add pscale driver version
	res.Spec.Driver.ConfigVersion = shared.ConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerScale

	return res
}

// makes a csm object with tolerations
func csmForPowerScaleBadCertCnt() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add log level to cover some code in GetConfigMap
	envVarLogLevel1 := corev1.EnvVar{Name: "CERT_SECRET_COUNT", Value: "thisIsNotANumber"}
	envVarLogLevel2 := corev1.EnvVar{Name: "X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION", Value: "true"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarLogLevel1, envVarLogLevel2}

	// Add pscale driver version
	res.Spec.Driver.ConfigVersion = shared.ConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerScale

	return res
}

// makes a csm object with tolerations
func csmForPowerScaleBadVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add pscale driver version
	res.Spec.Driver.ConfigVersion = "v0"
	res.Spec.Driver.CSIDriverType = csmv1.PowerScale

	return res
}

// makes a csm object with tolerations
func csmForPowerScaleInvalidVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add pscale driver version
	res.Spec.Version = shared.InvalidCSMVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerScale

	return res
}

func goisilonDebug(debug string) csmv1.ContainerStorageModule {
	cr := csmForPowerScale()
	cr.Spec.Driver.Common.Envs = []corev1.EnvVar{
		{Name: "GOISILON_DEBUG", Value: debug},
	}

	return cr
}

func csmForPowerScaleMetrics() csmv1.ContainerStorageModule {
	cr := csmForPowerScale()
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{Enabled: true}
	return cr
}

func csmForPowerScaleMetricsDisabled() csmv1.ContainerStorageModule {
	cr := csmForPowerScale()
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{Enabled: false}
	return cr
}

func csmForPowerScaleMetricsTLS() csmv1.ContainerStorageModule {
	cr := csmForPowerScale()
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{ //nolint:gosec
		Enabled:       true,
		TLSCertSecret: "pscale-metrics-tls",
	}
	return cr
}

func csmForPowerScaleMetricsWithLeaderElection() csmv1.ContainerStorageModule {
	cr := csmForPowerScaleMetrics()
	leaderElectionEnabled := true
	cr.Spec.Driver.Metrics.LeaderElection = &csmv1.LeaderElectionConfig{
		Enabled:       &leaderElectionEnabled,
		LeaseDuration: "60s",
		RenewDeadline: "40s",
		RetryPeriod:   "10s",
	}
	return cr
}

func csmForPowerScaleMetricsWithInvalidLeaderElection() csmv1.ContainerStorageModule {
	cr := csmForPowerScaleMetrics()
	leaderElectionEnabled := true
	cr.Spec.Driver.Metrics.LeaderElection = &csmv1.LeaderElectionConfig{
		Enabled:       &leaderElectionEnabled,
		LeaseDuration: "40s",
		RenewDeadline: "60s",
		RetryPeriod:   "bad",
	}
	return cr
}

func csmForPowerScaleMetricsWithLeaderElectionNilEnabled() csmv1.ContainerStorageModule {
	cr := csmForPowerScaleMetrics()
	cr.Spec.Driver.Metrics.LeaderElection = &csmv1.LeaderElectionConfig{
		LeaseDuration: "60s",
		RenewDeadline: "40s",
		RetryPeriod:   "5s",
	}
	return cr
}

func csmForPowerScaleMetricsWithEmptyLeaseDuration() csmv1.ContainerStorageModule {
	cr := csmForPowerScaleMetrics()
	leaderElectionEnabled := true
	cr.Spec.Driver.Metrics.LeaderElection = &csmv1.LeaderElectionConfig{
		Enabled:       &leaderElectionEnabled,
		RenewDeadline: "40s",
		RetryPeriod:   "10s",
	}
	return cr
}

func csmForPowerScaleMetricsPartialStructured() csmv1.ContainerStorageModule {
	cr := csmForPowerScaleMetrics()
	cr.Spec.Driver.Metrics.Collection = &csmv1.MetricsCollectionConfig{
		Interval: "45s",
	}
	return cr
}

func csmForPowerScaleMetricsStructured() csmv1.ContainerStorageModule {
	cr := csmForPowerScaleMetrics()
	cr.Spec.Driver.Metrics.Collection = &csmv1.MetricsCollectionConfig{
		Interval: "1m",
		CacheTTL: "40s",
	}
	cr.Spec.Driver.Metrics.Array = &csmv1.MetricsArrayConfig{
		RateLimit: 250,
		Timeout:   "45s",
		CircuitBreaker: &csmv1.MetricsCircuitBreakerConfig{
			Threshold:    5,
			ResetTimeout: "2m",
		},
	}
	return cr
}

func csmForPowerScaleMetricsStructuredInvalid() csmv1.ContainerStorageModule {
	cr := csmForPowerScaleMetrics()
	cr.Spec.Driver.Metrics.Collection = &csmv1.MetricsCollectionConfig{
		Interval: "invalid",
		CacheTTL: "0s",
	}
	cr.Spec.Driver.Metrics.Array = &csmv1.MetricsArrayConfig{
		RateLimit: -1,
		Timeout:   "bad-timeout",
		CircuitBreaker: &csmv1.MetricsCircuitBreakerConfig{
			Threshold:    0,
			ResetTimeout: "-1s",
		},
	}
	return cr
}

// TestPowerScaleNodePodStartsWithoutTLSHD is a regression test that verifies
// the PowerScale node DaemonSet pod spec does NOT use a blocking hostPath mount
// for the tlshd binary. Previously, the pod spec required /usr/sbin/tlshd with
// type: File, which caused node pods to fail with ContainerCreating on clusters
// where tlshd was not installed, even when mTLS was not enabled. The fix mounts
// the parent directory /usr/sbin with type: DirectoryOrCreate instead, allowing
// pods to start regardless of whether tlshd is present. The driver detects tlshd
// at runtime and only requires it when NFSTransportSecurity is "mtls" or "tls".
func TestPowerScaleNodePodStartsWithoutTLSHD(t *testing.T) {
	ctx := context.Background()
	cr := csmForPowerScale()

	// Get the node DaemonSet YAML from the operator config
	nodeYAMLPath := "../../operatorconfig/driverconfig/powerscale/v2.18.0/node.yaml"
	nodeYAMLBytes, err := os.ReadFile(nodeYAMLPath)
	assert.NoError(t, err, "Failed to read node.yaml")

	nodeYAMLContent := string(nodeYAMLBytes)

	// REGRESSION CHECK 1: The hostPath path must be /usr/sbin, NOT /usr/sbin/tlshd
	assert.Contains(t, nodeYAMLContent, "path: /usr/sbin",
		"node.yaml must contain 'path: /usr/sbin'")
	assert.NotContains(t, nodeYAMLContent, "path: /usr/sbin/tlshd",
		"node.yaml must NOT contain 'path: /usr/sbin/tlshd' (would block pod startup)")

	// REGRESSION CHECK 2: The hostPath type must be DirectoryOrCreate, NOT File
	// Find the usr-sbin-tlshd volume section (in volumes, not volumeMounts) and verify its type
	lines := strings.Split(nodeYAMLContent, "\n")
	inTlshdVolume := false
	foundDirectoryOrCreate := false
	foundFileType := false
	for i, line := range lines {
		// Look for the volume definition (must have "hostPath:" in the next few lines)
		if strings.Contains(line, "- name: usr-sbin-tlshd") {
			// Check if this is in the volumes section (has hostPath)
			hasHostPath := false
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				if strings.Contains(lines[j], "hostPath:") {
					hasHostPath = true
					break
				}
			}
			if !hasHostPath {
				continue // This is volumeMounts, not volumes
			}

			inTlshdVolume = true
			// Check the next few lines for the type
			for j := i + 1; j < len(lines) && j < i+10; j++ {
				if strings.Contains(lines[j], "type: DirectoryOrCreate") {
					foundDirectoryOrCreate = true
					break
				}
				if strings.Contains(lines[j], "type: File") {
					foundFileType = true
					break
				}
				// Stop if we hit another volume definition
				if strings.Contains(lines[j], "- name:") {
					break
				}
			}
			break
		}
	}

	assert.True(t, inTlshdVolume, "usr-sbin-tlshd volume not found in node.yaml")
	assert.True(t, foundDirectoryOrCreate,
		"usr-sbin-tlshd volume must have 'type: DirectoryOrCreate'")
	assert.False(t, foundFileType,
		"usr-sbin-tlshd volume must NOT have 'type: File' (would block pod startup)")

	// REGRESSION CHECK 3: The mount path must be /host/usr/sbin, NOT /usr/sbin/tlshd
	assert.Contains(t, nodeYAMLContent, "mountPath: /host/usr/sbin",
		"node.yaml must contain 'mountPath: /host/usr/sbin'")
	assert.NotContains(t, nodeYAMLContent, "mountPath: /usr/sbin/tlshd",
		"node.yaml must NOT contain 'mountPath: /usr/sbin/tlshd'")

	helmNodeYAMLPath := "../../../helm-charts/charts/csi-isilon/templates/node.yaml"
	helmNodeYAMLBytes, err := os.ReadFile(helmNodeYAMLPath)
	assert.NoError(t, err, "Failed to read Helm node.yaml")
	helmNodeYAMLContent := string(helmNodeYAMLBytes)
	assert.Contains(t, helmNodeYAMLContent, "- name: usr-sbin-tlshd\n              mountPath: /host/usr/sbin\n              readOnly: true",
		"Helm driver must mount the host's /usr/sbin directory read-only")
	assert.Contains(t, helmNodeYAMLContent, "- name: usr-sbin-tlshd\n          hostPath:\n            path: /usr/sbin\n            type: DirectoryOrCreate",
		"Helm node.yaml must not require the tlshd binary to exist")
	assert.NotContains(t, helmNodeYAMLContent, "path: /usr/sbin/tlshd",
		"Helm node.yaml must NOT require /usr/sbin/tlshd to exist")

	t.Logf("✅ Regression test passed: node pod will start even without tlshd binary")
	_ = ctx
	_ = cr
}

func TestModifyPowerScaleCRStorageCapacity(t *testing.T) {
	tests := []struct {
		name             string
		cr               csmv1.ContainerStorageModule
		expectedCapacity string
	}{
		{
			name:             "CSIDriverSpec is nil: should use default false",
			cr:               csmForPowerScale(),
			expectedCapacity: "false",
		},
		{
			name: "CSIDriverSpec present with StorageCapacity true: should output true",
			cr: func() csmv1.ContainerStorageModule {
				res := csmForPowerScale()
				res.Spec.Driver.CSIDriverSpec = &csmv1.CSIDriverSpec{StorageCapacity: true}
				return res
			}(),
			expectedCapacity: "true",
		},
		{
			name: "CSIDriverSpec present with StorageCapacity false: should output false",
			cr: func() csmv1.ContainerStorageModule {
				res := csmForPowerScale()
				res.Spec.Driver.CSIDriverSpec = &csmv1.CSIDriverSpec{StorageCapacity: false}
				return res
			}(),
			expectedCapacity: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlString := CsiStorageCapacityEnabled
			result := ModifyPowerScaleCR(yamlString, tt.cr, "CSIDriverSpec")
			assert.Equal(t, tt.expectedCapacity, result)
		})
	}
}
