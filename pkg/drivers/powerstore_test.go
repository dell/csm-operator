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
	powerStoreCSM            = csmForPowerStore("csm")
	powerStoreCSMBadVersion  = csmForPowerStoreBadVersion()
	powerStoreClient         = crclient.NewFakeClientNoInjector(objects)
	configJSONFileGoodPStore = fmt.Sprintf("%s/driverconfig/%s/config.json", config.ConfigDirectory, csmv1.PowerStore)
	powerStoreSecret         = shared.MakeSecretWithJSON("csm-config", "driver-test", configJSONFileGoodPStore)
	fakeSecretPstore         = shared.MakeSecret("fake-secret", "fake-ns", shared.PStoreConfigVersion)

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
			name:       "update GOPOWERSTORE_DEBUG value for Node",
			yamlString: "<GOPOWERSTORE_DEBUG>",
			csm:        gopowerstoreDebug("true"),
			ct:         powerStoreClient,
			sec:        powerStoreSecret,
			fileType:   "Node",
			expected:   "true",
		},
		{
			name: "update HBNFS values for Node",
			yamlString: `
			- name: X_CSI_NFS_EXPORT_DIRECTORY
		      value: "<X_CSI_NFS_EXPORT_DIRECTORY>"
		    - name: X_CSI_NFS_CLIENT_PORT
		      value: "<X_CSI_NFS_CLIENT_PORT>"
		    - name: X_CSI_NFS_SERVER_PORT
		      value: "<X_CSI_NFS_SERVER_PORT>"`,
			csm:      csmForPowerStoreWithHBNFS("csm"),
			ct:       powerStoreClient,
			sec:      powerStoreSecret,
			fileType: "Node",
			expected: `
			- name: X_CSI_NFS_EXPORT_DIRECTORY
		      value: "/var/lib/dell/myNfsExport"
		    - name: X_CSI_NFS_CLIENT_PORT
		      value: "2220"
		    - name: X_CSI_NFS_SERVER_PORT
		      value: "2221"`,
		},
		{
			name: "update HBNFS values for Controller",
			yamlString: `
			- name: X_CSI_NFS_EXPORT_DIRECTORY
              value: "<X_CSI_NFS_EXPORT_DIRECTORY>"
            - name: X_CSI_NFS_CLIENT_PORT
              value: "<X_CSI_NFS_CLIENT_PORT>"
            - name: X_CSI_NFS_SERVER_PORT
              value: "<X_CSI_NFS_SERVER_PORT>"`,
			csm:      csmForPowerStoreWithHBNFS("csm"),
			ct:       powerStoreClient,
			sec:      powerStoreSecret,
			fileType: "Controller",
			expected: `
		    - name: X_CSI_NFS_EXPORT_DIRECTORY
              value: "/var/lib/dell/myNfsExport"
			- name: X_CSI_NFS_CLIENT_PORT
		      value: "2220"
		    - name: X_CSI_NFS_SERVER_PORT
		      value: "2221"`,
		},
		{
			name: "minimal minifest - update HBNFS values for Node",
			yamlString: `
			- name: X_CSI_NFS_EXPORT_DIRECTORY
              value: "<X_CSI_NFS_EXPORT_DIRECTORY>"
            - name: X_CSI_NFS_CLIENT_PORT
              value: "<X_CSI_NFS_CLIENT_PORT>"
            - name: X_CSI_NFS_SERVER_PORT
              value: "<X_CSI_NFS_SERVER_PORT>"`,
			csm:      csmForPowerStore("csm"),
			ct:       powerStoreClient,
			sec:      powerStoreSecret,
			fileType: "Node",
			expected: `
			- name: X_CSI_NFS_EXPORT_DIRECTORY
              value: "/var/lib/dell/nfs"
            - name: X_CSI_NFS_CLIENT_PORT
              value: "2050"
            - name: X_CSI_NFS_SERVER_PORT
              value: "2049"`,
		},
	}
)

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

// makes a csm object
func csmForPowerStore(customCSMName string) csmv1.ContainerStorageModule {
	res := shared.MakeCSM(customCSMName, "driver-test", shared.PStoreConfigVersion)
	res.Spec.Driver.AuthSecret = "csm-config"

	// Add pstore driver version
	res.Spec.Driver.ConfigVersion = shared.PStoreConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerStore

	return res
}

func csmForPowerStoreWithHBNFS(customCSMName string) csmv1.ContainerStorageModule {
	cr := csmForPowerStore(customCSMName)

	cr.Spec.Driver.Common.Envs = []corev1.EnvVar{
		{Name: "X_CSI_NFS_CLIENT_PORT", Value: "2220"},
		{Name: "X_CSI_NFS_SERVER_PORT", Value: "2221"},
		{Name: "X_CSI_NFS_EXPORT_DIRECTORY", Value: "/var/lib/dell/myNfsExport"},
	}

	return cr
}

func gopowerstoreDebug(debug string) csmv1.ContainerStorageModule {
	cr := csmForPowerStore("csm")
	cr.Spec.Driver.Common.Envs = []corev1.EnvVar{
		{Name: "GOPOWERSTORE_DEBUG", Value: debug},
	}

	return cr
}
