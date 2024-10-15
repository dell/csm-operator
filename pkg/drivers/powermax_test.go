//  Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//       http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

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
	powerMaxCSM                = csmForPowerMax()
	powerMaxCSMNoProxy         = csmForPowerMaxNOProxy()
	powerMaxCSMBadVersion      = csmForPowerMaxBadVersion()
	powermaxDefaultKubeletPath = getDefaultKubeletPath()
	powerMaxClient             = crclient.NewFakeClientNoInjector(objects)
	powerMaxSecret             = shared.MakeSecret("csm-creds", "pmax-test", shared.PmaxConfigVersion)
	pMaxfakeSecret             = shared.MakeSecret("fake-creds", "fake-test", shared.PmaxConfigVersion)

	powerMaxTests = []struct {
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
		{"happy path", powerMaxCSM, powerMaxClient, powerMaxSecret, ""},
		{"no proxy set defaults", powerMaxCSMNoProxy, powerMaxClient, powerMaxSecret, ""},
	}

	preCheckpowerMaxTest = []struct {
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
		{"missing secret", powerMaxCSM, powerMaxClient, pMaxfakeSecret, "failed to find secret"},
		{"bad version", powerMaxCSMBadVersion, powerMaxClient, powerMaxSecret, "not supported"},
		{"bad latest version", powermaxDefaultKubeletPath, powerMaxClient, powerMaxSecret, ""},
	}
)

func TestPrecheckPowerMax(t *testing.T) {
	ctx := context.Background()
	for _, tt := range preCheckpowerMaxTest {
		t.Run(tt.name, func(t *testing.T) { // #nosec G601 - Run waits for the call to complete.
			err := PrecheckPowerMax(ctx, &tt.csm, config, tt.ct)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}

	for _, tt := range powerMaxTests {
		err := tt.ct.Create(ctx, tt.sec)
		if err != nil {
			assert.Nil(t, err)
		}
		t.Run(tt.name, func(t *testing.T) { // #nosec G601 - Run waits for the call to complete.
			err := PrecheckPowerMax(ctx, &tt.csm, config, tt.ct)
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

// makes a csm object without proxy
func csmForPowerMaxNOProxy() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "pmax-test", shared.PmaxConfigVersion)

	// Add log level to cover some code in GetConfigMap
	envVarLogLevel1 := corev1.EnvVar{Name: "X_CSI_POWERMAX_PORTGROUPS", Value: "csi_pg"}
	envVarLogLevel2 := corev1.EnvVar{Name: "X_CSI_TRANSPORT_PROTOCOL", Value: "FC"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarLogLevel1, envVarLogLevel2}
	res.Spec.Driver.AuthSecret = "csm-creds"

	// Add pmax driver version
	res.Spec.Driver.ConfigVersion = shared.PmaxConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerMax

	return res
}

// makes a csm object with proxy
func csmForPowerMax() csmv1.ContainerStorageModule {
	res := csmForPowerMaxNOProxy()
	revproxy := shared.MakeReverseProxyModule(shared.ConfigVersion)
	res.Spec.Modules = append(res.Spec.Modules, revproxy)
	return res
}

// makes a csm object with tolerations
func csmForPowerMaxBadVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "pmax-test", shared.PmaxConfigVersion)

	// Add pmax driver version
	res.Spec.Driver.ConfigVersion = "v0"
	res.Spec.Driver.CSIDriverType = csmv1.PowerMax

	return res
}

func getDefaultKubeletPath() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "pmax-test", shared.PmaxConfigVersion)

	kubeEnv := corev1.EnvVar{Name: "KUBELET_CONFIG_DIR", Value: "/fake"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{kubeEnv}

	return res
}
