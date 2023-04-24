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
	unityCSMBadConfig       = csmForUnityBadConfig()
	unityClient             = crclient.NewFakeClientNoInjector(objects)
	configJSONFileGoodUnity = fmt.Sprintf("%s/driverconfig/%s/config.json", config.ConfigDirectory, csmv1.Unity)
	unitySecret             = shared.MakeSecretWithJSON("csm-creds", "driver-test", configJSONFileGoodUnity)
	fakeSecretUnity         = shared.MakeSecret("fake-secret", "fake-ns", shared.UnityConfigVersion)

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
		{"bad version", unityCSMBadConfig, unityClient, unitySecret, "failed to find secret"},
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
	}
)

func TestPrecheckUnity(t *testing.T) {
	ctx := context.Background()
	for _, tt := range unityPrecheckTests {
		t.Run(tt.name, func(t *testing.T) {
			err := PrecheckUnity(ctx, &tt.csm, config, tt.ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else if err != nil {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}

	for _, tt := range unityTests {
		tt.ct.Create(ctx, tt.sec)
		t.Run(tt.name, func(t *testing.T) {
			err := PrecheckUnity(ctx, &tt.csm, config, tt.ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else if err != nil {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

// makes a csm object with a bad version
func csmForUnityBadVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.UnityConfigVersion)

	// Add pstore driver version
	res.Spec.Driver.ConfigVersion = shared.BadConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.Unity

	return res
}

// makes a csm object with a bad version
func csmForUnityBadConfig() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.UnityConfigVersion)

	// Add pstore driver version
	res.Spec.Driver.ConfigVersion = shared.UnityConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.Unity

	res.Spec.Driver.AuthSecret = "notARealSecret"

	return res
}

// makes a csm object
func csmForUnity(customCSMName string) csmv1.ContainerStorageModule {
	res := shared.MakeCSM(customCSMName, "driver-test", shared.UnityConfigVersion)
	res.Spec.Driver.AuthSecret = "csm-creds"

	// Add pstore driver version
	res.Spec.Driver.ConfigVersion = shared.UnityConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.Unity

	return res
}
