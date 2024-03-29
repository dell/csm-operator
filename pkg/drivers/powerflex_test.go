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
)

func TestPowerFlexGo(t *testing.T) {
	ctx := context.Background()
	for _, tt := range powerFlexTests {
		tt.ct.Create(ctx, tt.sec)
		t.Run(tt.name, func(t *testing.T) { // #nosec G601 - Run waits for the call to complete.
			err := PrecheckPowerFlex(ctx, &tt.csm, config, tt.ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
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
