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

	csmv1 "eos2git.cec.lab.emc.com/CSM/csm-operator/api/v1"
	shared "eos2git.cec.lab.emc.com/CSM/csm-operator/tests/sharedutil"
	"eos2git.cec.lab.emc.com/CSM/csm-operator/tests/sharedutil/crclient"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	crName     = "cosi-test"
	namespace  = "cosi-test-namespace"
	secretName = crName + "-config"
)

var (
	cosiCSM           = csmForCosi("")
	cosiCSMBadVersion = csmForCosiBadVersion()
	cosiClient        = crclient.NewFakeClientNoInjector(objects)
	cosiSecret        = shared.MakeSecret(secretName, namespace, shared.CosiConfigVersion)
	cosiFakeSecret    = shared.MakeSecret("fake-secret", "fake-ns", shared.CosiConfigVersion)

	cosiTests = []struct {
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
		{"happy path", cosiCSM, cosiClient, cosiSecret, ""},
		{"missing secret", cosiCSM, cosiClient, cosiFakeSecret, "no secrets found"},
		{"bad version", cosiCSMBadVersion, cosiClient, cosiSecret, "not supported"},
	}
)

func TestPrecheckCosi(t *testing.T) {
	ctx := context.Background()
	for _, tt := range cosiTests {
		err := tt.ct.Create(ctx, tt.sec)
		if err != nil {
			assert.Nil(t, err)
		}
		t.Run(tt.name, func(t *testing.T) {
			err := PrecheckCosi(ctx, &tt.csm, config, tt.ct)
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

func csmForCosi(driver csmv1.DriverType) csmv1.ContainerStorageModule {
	res := shared.MakeCSM(crName, namespace, shared.CosiConfigVersion)

	// Add image name
	res.Spec.Driver.Common.Image = "thisIsAnImage"

	// Add driver version
	res.Spec.Driver.ConfigVersion = shared.CosiConfigVersion

	// Add driver type
	res.Spec.Driver.CSIDriverType = driver

	// Add environment variables
	envVar1 := corev1.EnvVar{Name: "CSI_LOG_LEVEL", Value: "10"}
	envVar2 := corev1.EnvVar{Name: "CSI_LOG_FORMAT", Value: "text"}
	envVar3 := corev1.EnvVar{Name: "OTEL_COLLECTOR_ADDRESS", Value: "test:1234"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVar1, envVar2, envVar3}

	// Add sidecar
	sideCarObj := csmv1.ContainerTemplate{
		Name: "objectstorage-provisioner-sidecar",
		Envs: []corev1.EnvVar{
			{
				Name:  "VERBOSITY",
				Value: "1",
			},
		},
	}
	sideCarList := []csmv1.ContainerTemplate{sideCarObj}
	res.Spec.Driver.SideCars = sideCarList
	return res
}

func csmForCosiBadVersion() csmv1.ContainerStorageModule {
	res := shared.MakeCSM(crName, namespace, shared.CosiConfigVersion)

	res.Spec.Driver.ConfigVersion = "v0"
	res.Spec.Driver.CSIDriverType = csmv1.Cosi

	return res
}
