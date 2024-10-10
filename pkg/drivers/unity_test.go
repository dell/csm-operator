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
	"github.com/dell/csm-operator/tests/shared"
	"github.com/dell/csm-operator/tests/shared/crclient"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	csmUnity                = csmForUnity("csm")
	unityCSMBadVersion      = csmForUnityBadVersion()
	unityCSMBadSkipCert     = csmForUnityBadSkipCert()
	unityCSMBadCertCnt      = csmForUnityBadCertCnt()
	unityClient             = crclient.NewFakeClientNoInjector(objects)
	configJSONFileGoodUnity = fmt.Sprintf("%s/driverconfig/%s/config.json", config.ConfigDirectory, csmv1.Unity)
	unitySecret             = shared.MakeSecretWithJSON("csm-creds", "driver-test", configJSONFileGoodUnity)
	fakeSecretUnity         = shared.MakeSecret("fake-secret", "fake-ns", shared.UnityConfigVersion)
	skipCertValidIsFalse    = csmForUnitySkipCertISFalse()

	unityTests = []struct {
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
		{"happy path", csmUnity, unityClient, unitySecret, ""},
		{"bad version", unityCSMBadVersion, unityClient, unitySecret, "not supported"},
		{"invalid value for skip cert validation", unityCSMBadSkipCert, unityClient, unitySecret, "is an invalid value for X_CSI_UNITY_SKIP_CERTIFICATE_VALIDATION"},
		{"invalid value for cert secret cnt", unityCSMBadCertCnt, unityClient, unitySecret, "is an invalid value for CERT_SECRET_COUNT"},
	}

	unityPrecheckTests = []struct {
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
		{"missing secret", csmUnity, unityClient, fakeSecretUnity, "failed to find secret"},
		{"should check for certs in driver namespace when X_CSI_UNITY_SKIP_CERTIFICATE_VALIDATION is false ", skipCertValidIsFalse, unityClient, fakeSecretUnity, "failed to find secret"},
	}
)

func TestPrecheckUnity(t *testing.T) {
	ctx := context.Background()
	for _, tt := range unityPrecheckTests {
		t.Run(tt.name, func(t *testing.T) { // #nosec G601 - Run waits for the call to complete.
			err := PrecheckUnity(ctx, &tt.csm, config, tt.ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else if err != nil {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}

	for _, tt := range unityTests {
		err := tt.ct.Create(ctx, tt.sec)
		if err != nil {
			assert.Nil(t, err)
		}

		t.Run(tt.name, func(t *testing.T) { // #nosec G601 - Run waits for the call to complete.
			err := PrecheckUnity(ctx, &tt.csm, config, tt.ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else if err != nil {
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
func csmForUnityBadVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.UnityConfigVersion)

	// Add unity driver version
	res.Spec.Driver.ConfigVersion = shared.BadConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.Unity

	return res
}

// makes a csm object
func csmForUnity(customCSMName string) csmv1.ContainerStorageModule {
	res := shared.MakeCSM(customCSMName, "driver-test", shared.UnityConfigVersion)

	// Add unity driver version
	res.Spec.Driver.ConfigVersion = shared.UnityConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.Unity
	envVar1 := corev1.EnvVar{Name: "X_CSI_UNITY_SKIP_CERTIFICATE_VALIDATION", Value: "true"}
	envVar2 := corev1.EnvVar{Name: "CERT_SECRET_COUNT", Value: "1"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVar1, envVar2}

	return res
}

// makes a csm object with bad cert skip validation input
func csmForUnityBadSkipCert() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.UnityConfigVersion)

	// Add log level to cover some code in GetConfigMap
	envVarLogLevel1 := corev1.EnvVar{Name: "CERT_SECRET_COUNT", Value: "2"}
	envVarLogLevel2 := corev1.EnvVar{Name: "X_CSI_UNITY_SKIP_CERTIFICATE_VALIDATION", Value: "NotABool"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarLogLevel1, envVarLogLevel2}

	// Add unity driver version
	res.Spec.Driver.ConfigVersion = shared.UnityConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.Unity

	return res
}

// makes a csm object with bad cert count input
func csmForUnityBadCertCnt() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.UnityConfigVersion)

	// Add log level to cover some code in GetConfigMap
	envVarLogLevel1 := corev1.EnvVar{Name: "CERT_SECRET_COUNT", Value: "thisIsNotANumber"}
	envVarLogLevel2 := corev1.EnvVar{Name: "X_CSI_UNITY_SKIP_CERTIFICATE_VALIDATION", Value: "true"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarLogLevel1, envVarLogLevel2}

	// Add unity driver version
	res.Spec.Driver.ConfigVersion = shared.UnityConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.Unity

	return res
}

// makes a csm object with bad cert skip validation input
func csmForUnitySkipCertISFalse() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.UnityConfigVersion)

	// Add log level to cover some code in GetConfigMap
	envVarLogLevel1 := corev1.EnvVar{Name: "CERT_SECRET_COUNT", Value: "2"}
	envVarLogLevel2 := corev1.EnvVar{Name: "X_CSI_UNITY_SKIP_CERTIFICATE_VALIDATION", Value: "False"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarLogLevel1, envVarLogLevel2}

	// Add unity driver version
	res.Spec.Driver.ConfigVersion = shared.UnityConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.Unity

	return res
}
