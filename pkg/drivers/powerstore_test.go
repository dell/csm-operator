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
	powerStoreCSM            = csmForPowerStore("csm")
	powerStoreCSMBadVersion  = csmForPowerStoreBadVersion()
	powerStoreClient         = crclient.NewFakeClientNoInjector(objects)
	configJSONFileGoodPStore = fmt.Sprintf("%s/driverconfig/%s/config.json", config.ConfigDirectory, csmv1.PowerStore)
	powerStoreSecret         = shared.MakeSecretWithJSON("csm-config", "driver-test", configJSONFileGoodPStore)
	//Uncomment once the secret validation is in
	//fakeSecretPstore         = shared.MakeSecret("fake-secret", "fake-ns", shared.PStoreConfigVersion)

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
	}

	//Uncomment once the secret validation is in

	// powerStorePrecheckTests = []struct {
	// 	// every single unit test name
	// 	name string
	// 	// csm object
	// 	csm csmv1.ContainerStorageModule
	// 	// client
	// 	ct client.Client
	// 	// secret
	// 	sec *corev1.Secret
	// 	// expected error
	// 	expectedErr string
	// }{
	// 	{"missing secret", powerStoreCSM, powerStoreClient, fakeSecretPstore, "failed to find secret"},
	// }
)

func TestPrecheckPowerStore(t *testing.T) {
	ctx := context.Background()
	//Uncomment once the secret validation is in
	// for _, tt := range powerStorePrecheckTests {
	// 	t.Run(tt.name, func(t *testing.T) {
	// 		err := PrecheckPowerStore(ctx, &tt.csm, config, tt.ct)
	// 		if tt.expectedErr == "" {
	// 			assert.Nil(t, err)
	// 		} else {
	// 			assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
	// 		}
	// 	})
	// }

	for _, tt := range powerStoreTests {
		tt.ct.Create(ctx, tt.sec)
		t.Run(tt.name, func(t *testing.T) {
			err := PrecheckPowerStore(ctx, &tt.csm, config, tt.ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
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

// makes a csm object
func csmForPowerStore(customCSMName string) csmv1.ContainerStorageModule {
	res := shared.MakeCSM(customCSMName, "driver-test", shared.PStoreConfigVersion)
	res.Spec.Driver.AuthSecret = "csm-config"

	// Add pstore driver version
	res.Spec.Driver.ConfigVersion = shared.PStoreConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore

	return res
}
