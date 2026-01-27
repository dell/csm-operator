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
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	shared "github.com/dell/csm-operator/tests/sharedutil"
	"github.com/dell/csm-operator/tests/sharedutil/crclient"
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
	cosiCSM           = csmForCosi("", nil, nil)
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

func TestModifyCosiCRController(t *testing.T) {
	tests := []struct {
		name               string
		controllerYamlPath string
		cr                 csmv1.ContainerStorageModule
		checkFn            func(t *testing.T, manifest string)
	}{
		{
			"it modifies envs, tolerations, and node selectors",
			"testdata/cosi-controller.yaml",
			csmForCosi(csmv1.Cosi, map[string]string{
				"node-role.kubernetes.io/worker": "true",
			},
				[]corev1.Toleration{
					{Key: "node-role.kubernetes.io/worker", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
				},
				[]corev1.EnvVar{
					{Name: "COSI_LOG_LEVEL", Value: "info"},
					{Name: "COSI_LOG_FORMAT", Value: "text"},
					{Name: "OTEL_COLLECTOR_ADDRESS", Value: "test:1234"},
				}...),
			func(t *testing.T, manifest string) {
				assert.Contains(t, manifest, "--driver-config-params=/cosi-config-params/driver-config-params.yaml")
				assert.Contains(t, manifest, "--otel-endpoint=test:1234")
				assert.Contains(t, manifest, `node-role.kubernetes.io/worker: "true"`)
				assert.Contains(t, manifest, "key: node-role.kubernetes.io/worker")
				assert.Contains(t, manifest, "- effect: NoSchedule")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controllerYaml, err := os.ReadFile(tt.controllerYamlPath)
			if err != nil {
				t.Fatal(err)
			}
			manifest, err := ModifyCosiCR(string(controllerYaml), tt.cr, "Controller")
			assert.Nil(t, err)
			tt.checkFn(t, manifest)
		})
	}
}

func csmForCosi(driver csmv1.DriverType, selectors map[string]string, tolerations []corev1.Toleration, envs ...corev1.EnvVar) csmv1.ContainerStorageModule {
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
	envVars := []corev1.EnvVar{envVar1, envVar2, envVar3}
	if len(envs) > 0 {
		envVars = envs
	}
	res.Spec.Driver.Common.Envs = envVars

	res.Spec.Driver.Common.NodeSelector = selectors
	res.Spec.Driver.Common.Tolerations = tolerations

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
