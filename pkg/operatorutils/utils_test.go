//  Copyright © 2024 - 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package operatorutils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/assert"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	confv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	confcorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	confmetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

func captureOutput(f func()) string {
	reader, writer, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	stdout := os.Stdout
	stderr := os.Stderr
	defer func() {
		os.Stdout = stdout
		os.Stderr = stderr
	}()
	os.Stdout = writer
	os.Stderr = writer
	out := make(chan string)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		var buf bytes.Buffer
		wg.Done()
		_, _ = io.Copy(&buf, reader)
		out <- buf.String()
	}()
	wg.Wait()
	f()
	writer.Close()
	return <-out
}

// fullFakeClient is a helper function to create a fake client with all types pre-registered
func fullFakeClient() crclient.WithWatch {
	// CSM types must be registered with the scheme
	scheme := runtime.NewScheme()
	_ = csmv1.AddToScheme(scheme)  // for CSM objects
	_ = corev1.AddToScheme(scheme) // for namespaces
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	// Create a fake ctrlClient
	ctrlClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	return ctrlClient
}

// createCR is a helper function to create ContainerStorageModule object
func createCR(driverType csmv1.DriverType, moduleType csmv1.ModuleType, moduleEnabled bool, components []csmv1.ContainerTemplate) *csmv1.ContainerStorageModule {
	return &csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: csmv1.Driver{
				CSIDriverType: driverType,
			},
			Modules: []csmv1.Module{
				{
					Name:       moduleType,
					Enabled:    moduleEnabled,
					Components: components,
				},
			},
		},
	}
}

func TestSplitYaml(t *testing.T) {
	// Note: For all tests, the yaml converter puts maps into alphabetical order and has all 'tabs' as 4 spaces.
	// Whitespaces are *very particular* in unit test comparisons. Be aware of this.
	// If you feed in:
	//   containers:
	//     - name: my-container
	//       image: my-image
	//
	// you WILL get back:
	//
	//     containers:
	//         - image: my-image
	//           name: my-container
	//
	// Test case: Split a single YAML document
	yamlString := `apiVersion: v1
kind: Pod
metadata:
    name: my-pod
spec:
    containers:
        - image: my-image
          name: my-container
`

	expectedResult := [][]byte{
		[]byte(yamlString),
	}
	result, err := SplitYaml([]byte(yamlString))
	assert.Nil(t, err)
	assert.Equal(t, expectedResult, result)

	// Test case: Split multiple YAML documents
	yamlString = `---
apiVersion: v1
kind: Pod
metadata:
    name: my-pod
spec:
    containers:
        - name: my-container
          image: my-image
---
apiVersion: v1
kind: Service
metadata:
    name: my-service
spec:
    selector:
        app: my-app
    ports:
        - protocol: TCP
          port: 80
          targetPort: 9376
`
	expectedResult = [][]byte{
		[]byte(`apiVersion: v1
kind: Pod
metadata:
    name: my-pod
spec:
    containers:
        - image: my-image
          name: my-container
`),
		[]byte(`apiVersion: v1
kind: Service
metadata:
    name: my-service
spec:
    ports:
        - port: 80
          protocol: TCP
          targetPort: 9376
    selector:
        app: my-app
`),
	}
	result, err = SplitYaml([]byte(yamlString))
	assert.Nil(t, err)
	assert.Equal(t, expectedResult, result)

	// Test case: Empty YAML
	yamlString = ""
	result, err = SplitYaml([]byte(yamlString))
	assert.Nil(t, err)
	assert.Nil(t, result)

	// Test case: YAML with null byte
	yamlString = "\x00"
	_, err = SplitYaml([]byte(yamlString))
	assert.NotNil(t, err)
}

func TestHasModuleComponent(t *testing.T) {
	tests := []struct {
		name           string
		instance       csmv1.ContainerStorageModule
		mod            csmv1.ModuleType
		componentType  string
		expectedResult bool
	}{
		{
			name:           "Module does not exist",
			instance:       *createCR(csmv1.PowerFlex, csmv1.Replication, true, nil),
			mod:            csmv1.Observability,
			componentType:  "metrics-powerflex",
			expectedResult: false,
		},
		{
			name: "Module exist and component does not exist",
			instance: *createCR(csmv1.PowerFlex, csmv1.Observability, false, []csmv1.ContainerTemplate{
				{Name: "topology"},
			}),
			mod:            csmv1.Observability,
			componentType:  "metrics-powerflex",
			expectedResult: false,
		},
		{
			name: "Module exist and component exists",
			instance: *createCR(csmv1.PowerFlex, csmv1.Observability, false, []csmv1.ContainerTemplate{
				{Name: "metrics-powerflex"},
				{Name: "topology"},
			}),
			mod:            csmv1.Observability,
			componentType:  "metrics-powerflex",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasModuleComponent(tt.instance, tt.mod, tt.componentType)

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestIsModuleComponentEnabled(t *testing.T) {
	ctx := context.Background()

	// Test case: Module and component are enabled
	cr := csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Modules: []csmv1.Module{
				{
					Name:    csmv1.Observability,
					Enabled: true,
					Components: []csmv1.ContainerTemplate{
						{
							Name:    "topology",
							Enabled: &[]bool{true}[0],
						},
					},
				},
			},
		},
	}

	enabled := IsModuleComponentEnabled(ctx, cr, csmv1.Observability, "topology")
	if !enabled {
		t.Errorf("Expected true, got false")
	}

	// Test case: Module is disabled
	cr = csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Modules: []csmv1.Module{
				{
					Name:    csmv1.Observability,
					Enabled: false,
					Components: []csmv1.ContainerTemplate{
						{
							Name:    "topology",
							Enabled: &[]bool{true}[0],
						},
					},
				},
			},
		},
	}

	enabled = IsModuleComponentEnabled(ctx, cr, csmv1.Observability, "topology")
	if enabled {
		t.Errorf("Expected false, got true")
	}

	// Test case: Component is disabled
	cr = csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Modules: []csmv1.Module{
				{
					Name:    csmv1.Observability,
					Enabled: true,
					Components: []csmv1.ContainerTemplate{
						{
							Name:    "topology",
							Enabled: &[]bool{false}[0],
						},
					},
				},
			},
		},
	}

	enabled = IsModuleComponentEnabled(ctx, cr, csmv1.Observability, "topology")
	if enabled {
		t.Errorf("Expected false, got true")
	}

	// Test case: Component does not exist
	cr = csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Modules: []csmv1.Module{
				{
					Name:    csmv1.Observability,
					Enabled: true,
					Components: []csmv1.ContainerTemplate{
						{
							Name:    "otel-collector",
							Enabled: &[]bool{true}[0],
						},
					},
				},
			},
		},
	}

	enabled = IsModuleComponentEnabled(ctx, cr, csmv1.Observability, "topology")
	if enabled {
		t.Errorf("Expected false, got true")
	}
}

func TestIsModuleEnabled(t *testing.T) {
	ctx := context.Background()
	cr := csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Modules: []csmv1.Module{
				{
					Name:    csmv1.Observability,
					Enabled: true,
				},
			},
		},
	}

	// Test case: Module is enabled
	enabled, module := IsModuleEnabled(ctx, cr, csmv1.Observability)
	if !enabled {
		t.Errorf("Expected true, got false")
	}
	if module.Name != csmv1.Observability {
		t.Errorf("Expected module name %s, got %s", csmv1.Observability, module.Name)
	}

	// Test case: Module is disabled
	cr.Spec.Modules[0].Enabled = false
	enabled, module = IsModuleEnabled(ctx, cr, csmv1.Observability)
	if enabled {
		t.Errorf("Expected false, got true")
	}
	if module.Name != "" {
		t.Errorf("Expected module name %s, got %s", "", module.Name)
	}
}

func TestAddModuleComponent(t *testing.T) {
	type args struct {
		instance  *csmv1.ContainerStorageModule
		mod       csmv1.ModuleType
		component csmv1.ContainerTemplate
	}
	tests := []struct {
		name string
		args args
		want *csmv1.ContainerStorageModule
	}{
		{
			name: "Module does not exist",
			args: args{
				instance:  createCR(csmv1.PowerFlex, csmv1.Replication, false, nil),
				mod:       csmv1.Observability,
				component: csmv1.ContainerTemplate{Name: "topology"},
			},
			want: createCR(csmv1.PowerFlex, csmv1.Replication, false, nil),
		},
		{
			name: "Module exists and component is empty",
			args: args{
				instance:  createCR(csmv1.PowerFlex, csmv1.Observability, false, nil),
				mod:       csmv1.Observability,
				component: csmv1.ContainerTemplate{Name: "topology"},
			},
			want: createCR(csmv1.PowerFlex, csmv1.Observability, false, []csmv1.ContainerTemplate{
				{Name: "topology"},
			}),
		},
		{
			name: "Module exists and component is not empty",
			args: args{
				instance: createCR(csmv1.PowerFlex, csmv1.Observability, true, []csmv1.ContainerTemplate{
					{Name: "metrics-powerflex"},
				}),
				mod:       csmv1.Observability,
				component: csmv1.ContainerTemplate{Name: "topology"},
			},
			want: createCR(csmv1.PowerFlex, csmv1.Observability, true, []csmv1.ContainerTemplate{
				{Name: "metrics-powerflex"},
				{Name: "topology"},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddModuleComponent(tt.args.instance, tt.args.mod, tt.args.component)
			assert.Equal(t, tt.want, tt.args.instance)
		})
	}
}

func TestLoadDefaultComponents(t *testing.T) {
	invalidOp := OperatorConfig{
		ConfigDirectory: "invalid/path",
	}
	validOp := OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}
	enabled := new(bool)
	*enabled = true
	disabled := new(bool)
	*disabled = false

	type args struct {
		ctx context.Context
		cr  *csmv1.ContainerStorageModule
		op  OperatorConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *csmv1.ContainerStorageModule
		wantErr bool
	}{
		{
			name: "Observability module does not exist",
			args: args{
				ctx: context.Background(),
				cr:  createCR(csmv1.PowerFlex, csmv1.Replication, true, nil),
				op:  validOp,
			},
			want:    createCR(csmv1.PowerFlex, csmv1.Replication, true, nil),
			wantErr: false,
		},
		{
			name: "Default components not found",
			args: args{
				ctx: context.Background(),
				cr:  createCR(csmv1.PowerFlex, csmv1.Observability, true, nil),
				op:  invalidOp,
			},
			want:    createCR(csmv1.PowerFlex, csmv1.Observability, true, nil),
			wantErr: true,
		},
		{
			name: "Module disabled and components empty",
			args: args{
				ctx: context.Background(),
				cr:  createCR(csmv1.PowerScale, csmv1.Observability, false, nil),
				op:  validOp,
			},
			// if module is disabled, no components should be added
			want:    createCR(csmv1.PowerScale, csmv1.Observability, false, nil),
			wantErr: false,
		},
		{
			name: "Module disabled and topology component missing",
			args: args{
				ctx: context.Background(),
				cr: createCR(csmv1.PowerFlex, csmv1.Observability, false, []csmv1.ContainerTemplate{
					{Name: "otel-collector", Enabled: enabled},
					{Name: "metrics-powerflex", Enabled: enabled},
				}),
				op: validOp,
			},
			// if module is disabled, no components should be added
			want: createCR(csmv1.PowerFlex, csmv1.Observability, false, []csmv1.ContainerTemplate{
				{Name: "otel-collector", Enabled: enabled},
				{Name: "metrics-powerflex", Enabled: enabled},
			}),
			wantErr: false,
		},
		{
			name: "Module enabled and cert-manager component missing",
			args: args{
				ctx: context.Background(),
				cr: createCR(csmv1.PowerFlex, csmv1.Observability, true, []csmv1.ContainerTemplate{
					{Name: "topology", Enabled: enabled},
					{Name: "otel-collector", Enabled: enabled},
					{Name: "metrics-powerflex", Enabled: enabled},
				}),
				op: validOp,
			},
			want: createCR(csmv1.PowerFlex, csmv1.Observability, true, []csmv1.ContainerTemplate{
				{Name: "topology", Enabled: enabled},
				{Name: "otel-collector", Enabled: enabled},
				{Name: "metrics-powerflex", Enabled: enabled},
				{Name: "cert-manager", Enabled: disabled},
			}),
			wantErr: false,
		},
		{
			name: "Module disabled and all components exist",
			args: args{
				ctx: context.Background(),
				cr: createCR(csmv1.PowerFlex, csmv1.Observability, false, []csmv1.ContainerTemplate{
					{Name: "topology", Enabled: enabled},
					{Name: "otel-collector", Enabled: enabled},
					{Name: "cert-manager", Enabled: disabled},
					{Name: "metrics-powerflex", Enabled: enabled},
				}),
				op: validOp,
			},
			want: createCR(csmv1.PowerFlex, csmv1.Observability, false, []csmv1.ContainerTemplate{
				{Name: "topology", Enabled: enabled},
				{Name: "otel-collector", Enabled: enabled},
				{Name: "cert-manager", Enabled: disabled},
				{Name: "metrics-powerflex", Enabled: enabled},
			}),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := LoadDefaultComponents(tt.args.ctx, tt.args.cr, tt.args.op)

			if tt.wantErr {
				assert.Error(t, err)
			}
			assert.Equal(t, tt.want, tt.args.cr)
		})
	}
}

func TestSetContainerImage(t *testing.T) {
	type args struct {
		objects        []crclient.Object
		deploymentName string
		containerName  string
		image          string
		want           *corev1.Container
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Test setting image for a valid deployment and container",
			args: args{
				objects: []crclient.Object{
					&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-deployment",
						},
						Spec: appsv1.DeploymentSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "my-container",
											Image: "old-image",
										},
									},
								},
							},
						},
					},
				},
				deploymentName: "my-deployment",
				containerName:  "my-container",
				image:          "new-image",
				want: &corev1.Container{
					Name:  "my-container",
					Image: "new-image",
				},
			},
		},
		{
			name: "Test setting image for a non-existing deployment",
			args: args{
				objects: []crclient.Object{
					&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-deployment",
						},
						Spec: appsv1.DeploymentSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "my-container",
											Image: "old-image",
										},
									},
								},
							},
						},
					},
				},
				deploymentName: "non-existing-deployment",
				containerName:  "my-container",
				image:          "new-image",
				want:           nil,
			},
		},
		{
			name: "Test setting image for a non-existing container",
			args: args{
				objects: []crclient.Object{
					&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-deployment",
						},
						Spec: appsv1.DeploymentSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "my-container",
											Image: "old-image",
										},
									},
								},
							},
						},
					},
				},
				deploymentName: "my-deployment",
				containerName:  "non-existing-container",
				image:          "new-image",
				want:           nil,
			},
		},
		{
			name: "Test setting image for a deployment with no containers",
			args: args{
				objects: []crclient.Object{
					&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-deployment",
						},
						Spec: appsv1.DeploymentSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{},
								},
							},
						},
					},
				},
				deploymentName: "my-deployment",
				containerName:  "my-container",
				image:          "new-image",
				want:           nil,
			},
		},
		{
			name: "Test setting image for a deployment with no containers and empty objects slice",
			args: args{
				objects:        []crclient.Object{},
				deploymentName: "my-deployment",
				containerName:  "my-container",
				image:          "new-image",
				want:           nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetContainerImage(tt.args.objects, tt.args.deploymentName, tt.args.containerName, tt.args.image)

			// Find the deployment and container in the objects
			var container *corev1.Container
			for _, object := range tt.args.objects {
				if deployment, ok := object.(*appsv1.Deployment); ok && deployment.Name == tt.args.deploymentName {
					for _, c := range deployment.Spec.Template.Spec.Containers {
						if c.Name == tt.args.containerName {
							container = &c
							break
						}
					}
					break
				}
			}

			assert.Equal(t, tt.args.want, container)
		})
	}
}

func TestUpdateSideCarApply(t *testing.T) {
	// Test case: update sidecar with matching name
	ctx := context.Background()
	sc1env1 := "sidecar1-env1"
	oldenv1val := "old-env1-value"
	newenv1val := "sidecar1-env1-value"
	empty := "empty"
	emptyValue := ""
	sideCars := []csmv1.ContainerTemplate{
		{
			Name:            "sidecar1",
			Image:           "sidecar1-image",
			ImagePullPolicy: "sidecar1-image-pull-policy",
			Envs: []corev1.EnvVar{
				{
					Name:  sc1env1,
					Value: newenv1val,
				},
				{
					Name:  empty,
					Value: emptyValue,
				},
			},
			Args: []string{
				"--volume-name-prefix=csivol1",
			},
		},
		{
			Name:            "sidecar2",
			Image:           "sidecar2-image",
			ImagePullPolicy: "sidecar2-image-pull-policy",
			Envs: []corev1.EnvVar{
				{
					Name:  "sidecar2-env1",
					Value: "sidecar2-env1-value",
				},
				{
					Name:  "sidecar2-env2",
					Value: "sidecar2-env2-value",
				},
			},
			Args: []string{
				"--volume-name-prefix=csivol2",
			},
		},
	}

	container := acorev1.Container().
		WithName("sidecar1").
		WithImage("old-image").
		WithImagePullPolicy("old-image-pull-policy").
		WithEnv(&acorev1.EnvVarApplyConfiguration{
			Name:  &sc1env1,
			Value: &oldenv1val,
		},
		).WithEnv(&acorev1.EnvVarApplyConfiguration{
		Name:  &empty,
		Value: &emptyValue,
	},
	)

	UpdateSideCarApply(ctx, sideCars, container, csmv1.ContainerStorageModule{}, VersionSpec{})

	expectedContainer := acorev1.Container().WithName("sidecar1").WithImage("sidecar1-image").WithImagePullPolicy("sidecar1-image-pull-policy").
		WithEnv(&acorev1.EnvVarApplyConfiguration{
			Name:  &sc1env1,
			Value: &newenv1val,
		}).WithEnv(&acorev1.EnvVarApplyConfiguration{
		Name:  &empty,
		Value: &emptyValue,
	}).WithArgs("--volume-name-prefix=csivol1")

	assert.Equal(t, expectedContainer, container)

	// Use spec.version (with ConfigMap existing)
	UpdateSideCarApply(ctx, sideCars, container, csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Version: "v1.16.0",
		},
	}, VersionSpec{
		Version: "test-version",
		Images: map[string]string{
			"sidecar1": "configmap-sidecar1-image",
		},
	})

	expectedContainer2 := acorev1.Container().WithName("sidecar1").WithImage("configmap-sidecar1-image").WithImagePullPolicy("sidecar1-image-pull-policy").
		WithEnv(&acorev1.EnvVarApplyConfiguration{
			Name:  &sc1env1,
			Value: &newenv1val,
		}).WithEnv(&acorev1.EnvVarApplyConfiguration{
		Name:  &empty,
		Value: &emptyValue,
	}).WithArgs("--volume-name-prefix=csivol1")

	assert.Equal(t, expectedContainer2, container)

	// Use spec.version AND spec.customRegistry
	UpdateSideCarApply(ctx, sideCars, container, csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Version:        "v1.16.0",
			CustomRegistry: "test-custom-registry",
		},
	}, VersionSpec{})

	expectedContainer3 := acorev1.Container().WithName("sidecar1").WithImage("test-custom-registry/configmap-sidecar1-image").WithImagePullPolicy("sidecar1-image-pull-policy").
		WithEnv(&acorev1.EnvVarApplyConfiguration{
			Name:  &sc1env1,
			Value: &newenv1val,
		}).WithEnv(&acorev1.EnvVarApplyConfiguration{
		Name:  &empty,
		Value: &emptyValue,
	}).WithArgs("--volume-name-prefix=csivol1")

	assert.Equal(t, expectedContainer3, container)

	// Use spec.version with a sidecar not in the CR
	container2 := acorev1.Container().
		WithName("sidecarX").
		WithImage("old-image").
		WithImagePullPolicy("Always")

	// Matrix contains an entry for "sidecarX"
	matched := VersionSpec{
		Version: "vMatrix",
		Images: map[string]string{
			"sidecarX": "matrix-sidecarX-image",
		},
	}

	UpdateSideCarApply(ctx, sideCars, container2, csmv1.ContainerStorageModule{}, matched)

	expectedContainer4 := acorev1.Container().
		WithName("sidecarX").
		WithImage("matrix-sidecarX-image").
		WithImagePullPolicy("Always") // pull policy unchanged since no template matched

	assert.Equal(t, expectedContainer4, container2, "should apply matrix.Image even when no CR sidecar template matches")

	// Use spec.customRegistry with a sidecar not in the CR
	UpdateSideCarApply(ctx, sideCars, container2, csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			CustomRegistry: "test-custom-registry",
		},
	}, VersionSpec{})

	expectedContainer5 := acorev1.Container().
		WithName("sidecarX").
		WithImage("test-custom-registry/matrix-sidecarX-image").
		WithImagePullPolicy("Always") // pull policy unchanged since no template matched

	assert.Equal(t, expectedContainer5, container2, "should apply customRegistry even when no CR sidecar template matches")

	// repeat the test with the other function that uses the child function
	// very minor code coverage gain, 0.1%
	UpdateInitContainerApply(ctx, sideCars, container, csmv1.ContainerStorageModule{}, VersionSpec{})
	assert.Equal(t, expectedContainer, container)
}

func TestReplaceAllContainerImageApply(t *testing.T) {
	// Create a list of Images that will replace the image names in 'containers', below
	mockImages := K8sImagesConfig{
		K8sVersion: "test-k8s-version",
		// TODO: Why is Images an anonymous struct? Why is it not a known and defined struct?
		Images: struct {
			Attacher              string "json:\"attacher\" yaml:\"attacher\""
			Provisioner           string "json:\"provisioner\" yaml:\"provisioner\""
			Snapshotter           string "json:\"snapshotter\" yaml:\"snapshotter\""
			Registrar             string "json:\"registrar\" yaml:\"registrar\""
			Resizer               string "json:\"resizer\" yaml:\"resizer\""
			Externalhealthmonitor string "json:\"externalhealthmonitorcontroller\" yaml:\"externalhealthmonitorcontroller\""
			Sdc                   string "json:\"sdc\" yaml:\"sdc\""
			Sdcmonitor            string "json:\"sdcmonitor\" yaml:\"sdcmonitor\""
			Podmon                string "json:\"podmon\" yaml:\"podmon\""
			CSIRevProxy           string "json:\"csiReverseProxy\" yaml:\"csiReverseProxy\""
		}{
			Provisioner:           "new-provisioner-image",
			Attacher:              "new-attacher-image",
			Snapshotter:           "new-snapshotter-image",
			Registrar:             "new-registrar-image",
			Resizer:               "new-resizer-image",
			Externalhealthmonitor: "new-externalhealthmonitor-image",
			Sdc:                   "new-sdcmonitor-image",
			Sdcmonitor:            "new-sdcmonitor-image",
			Podmon:                "new-podmon-image",
		},
	}

	// config with container image names that will be replaced
	containers := []struct {
		Name    string
		Image   string
		NewName string
	}{
		{
			Name:    "provisioner",
			Image:   "old-provisioner-image",
			NewName: mockImages.Images.Provisioner,
		},
		{
			Name:    "attacher",
			Image:   "old-attacher-image",
			NewName: mockImages.Images.Attacher,
		},
		{
			Name:    "snapshotter",
			Image:   "old-snapshotter-image",
			NewName: mockImages.Images.Snapshotter,
		},
		{
			Name:    "registrar",
			Image:   "old-registrar-image",
			NewName: mockImages.Images.Registrar,
		},
		{
			Name:    "resizer",
			Image:   "old-resizer-image",
			NewName: mockImages.Images.Resizer,
		},
		{
			Name:    "external-health-monitor",
			Image:   "old-external-health-monitor-image",
			NewName: mockImages.Images.Externalhealthmonitor,
		},
		{
			Name:    "sdc",
			Image:   "old-sdc-image",
			NewName: mockImages.Images.Sdc,
		},
		{
			Name:    "sdc-monitor",
			Image:   "old-sdc-monitor-image",
			NewName: mockImages.Images.Sdcmonitor,
		},
		{
			Name:    "resiliency",
			Image:   "old-podmon-image",
			NewName: mockImages.Images.Podmon,
		},
	}

	for _, ctr := range containers {
		c := &acorev1.ContainerApplyConfiguration{
			Name:  &ctr.Name,
			Image: &ctr.Image,
		}

		// Call the function to test
		ReplaceAllContainerImageApply(mockImages, c)

		assert.Equal(t, ctr.NewName, *c.Image)
	}
}

func TestModifyCommonCR(t *testing.T) {
	// Test case 1: Modify the name and namespace
	cr := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: csmv1.Driver{
				Common: &csmv1.ContainerTemplate{
					ImagePullPolicy: corev1.PullPolicy("Always"),
					Envs: []corev1.EnvVar{
						{
							Name:  "KUBELET_CONFIG_DIR",
							Value: "test-value",
						},
					},
				},
			},
		},
	}
	yamlString := "name: " + DefaultReleaseName
	expected := "name: test-name"

	result := ModifyCommonCR(yamlString, cr)
	assert.Equal(t, expected, result)

	// Test case 2: Modify the image pull policy
	cr.Name = ""
	cr.Namespace = ""
	yamlString = "imagePullPolicy: " + DefaultImagePullPolicy
	expected = "imagePullPolicy: Always"

	result = ModifyCommonCR(yamlString, cr)
	assert.Equal(t, expected, result)

	// Test case 3: Modify both name, namespace, and image pull policy
	cr.Name = "test-name"
	cr.Namespace = "test-namespace"
	yamlString = "name: " + DefaultReleaseName + "\nimagePullPolicy: " + DefaultImagePullPolicy
	expected = "name: test-name\nimagePullPolicy: Always"

	result = ModifyCommonCR(yamlString, cr)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestReplaceAllArgs(t *testing.T) {
	// Test case: replace all args
	defaultArgs := []string{"arg1=value1", "arg2=value2", "arg3=value3"}
	crArgs := []string{"arg1=newValue1", "arg2=newValue2"}
	expected := []string{"arg1=newValue1", "arg2=newValue2", "arg3=value3"}

	result := ReplaceAllArgs(defaultArgs, crArgs)
	assert.Equal(t, expected, result)

	// Test case: replace some args
	defaultArgs = []string{"arg1=value1", "arg2=value2", "arg3=value3"}
	crArgs = []string{"arg1=newValue1"}
	expected = []string{"arg1=newValue1", "arg2=value2", "arg3=value3"}

	result = ReplaceAllArgs(defaultArgs, crArgs)
	assert.Equal(t, expected, result)

	// Test case: replace no args
	defaultArgs = []string{"arg1=value1", "arg2=value2", "arg3=value3"}
	crArgs = []string{}
	expected = []string{"arg1=value1", "arg2=value2", "arg3=value3"}

	result = ReplaceAllArgs(defaultArgs, crArgs)
	assert.Equal(t, expected, result)

	// Test case: merge args
	defaultArgs = []string{"arg1=value1", "arg2=value2", "arg3=value3"}
	crArgs = []string{"arg4=value1", "arg5=value2", "arg6=value3"}
	expected = []string{"arg1=value1", "arg2=value2", "arg3=value3", "arg4=value1", "arg5=value2", "arg6=value3"}

	result = ReplaceAllArgs(defaultArgs, crArgs)
	assert.Equal(t, expected, result)
}

// TODO: Cover more object types:
// ClusterRole, ClusterRoleBinding, ConfigMap, Deployment
func TestGetCTRLObject(t *testing.T) {
	// Test case: empty input
	ctrlBuf := []byte{}
	expected := []crclient.Object{}

	result, err := GetCTRLObject(ctrlBuf)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}

	assert.Equal(t, result, expected)

	// Test case: valid input
	ctrlBuf = []byte(`
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-sa
  namespace: default
`)

	expected = []crclient.Object{
		&corev1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ServiceAccount",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-sa",
				Namespace: "default",
			},
		},
	}

	result, err = GetCTRLObject(ctrlBuf)

	assert.Nil(t, err)
	assert.Equal(t, result, expected)

	ctrlBuf = []byte(`
apiVersion: v1
kind: Service
metadata:
  name: my-service
spec:
  selector:
    app: MyApp
  ports:
    - protocol: TCP
      port: 80
      targetPort: 9376
`)

	expected = []crclient.Object{
		&corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-service",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": "MyApp",
				},
				Ports: []corev1.ServicePort{
					{
						Protocol:   corev1.ProtocolTCP,
						Port:       80,
						TargetPort: intstr.FromInt(9376),
					},
				},
			},
		},
	}

	result, err = GetCTRLObject(ctrlBuf)

	assert.Nil(t, err)
	assert.Equal(t, result, expected)

	// Test case: invalid input
	ctrlBuf = []byte(`
apiVersion: v1
kind: Service
metadata:
  name: my-service
spec:
		`)

	expected = []crclient.Object{}

	result, err = GetCTRLObject(ctrlBuf)

	assert.NotNil(t, err)
	assert.Equal(t, result, expected)
}

func TestGetCTRLObjectClusterRole(t *testing.T) {
	// Test case: empty input
	ctrlBuf := []byte{}
	expected := []crclient.Object{}

	result, err := GetCTRLObject(ctrlBuf)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}

	assert.Equal(t, result, expected)
	// Test case: valid input
	ctrlBuf = []byte(`
apiVersion: v1
kind: ClusterRole
metadata:
  name: my-cluster-role
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "watch", "list"]
`)

	expected = []crclient.Object{
		&rbacv1.ClusterRole{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ClusterRole",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-cluster-role",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{"get", "watch", "list"},
				},
			},
		},
	}

	result, err = GetCTRLObject(ctrlBuf)

	assert.Nil(t, err)
	assert.Equal(t, result, expected)
}

func TestGetCTRLObjectClusterRoleBinding(t *testing.T) {
	// Test case: empty input
	ctrlBuf := []byte{}
	expected := []crclient.Object{}

	result, err := GetCTRLObject(ctrlBuf)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}

	assert.Equal(t, result, expected)
	// Test case: valid input
	ctrlBuf = []byte(`
apiVersion: v1
kind: ClusterRoleBinding
metadata:
  name: my-cluster-role-binding
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "watch", "list"]
`)
	expected = []crclient.Object{
		&rbacv1.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ClusterRoleBinding",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-cluster-role-binding",
			},
		},
	}

	result, err = GetCTRLObject(ctrlBuf)

	assert.Nil(t, err)
	assert.Equal(t, result, expected)
}

func TestGetCTRLObjectConfigMap(t *testing.T) {
	// Test case: empty input
	ctrlBuf := []byte{}
	expected := []crclient.Object{}

	result, err := GetCTRLObject(ctrlBuf)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}

	assert.Equal(t, result, expected)
	// Test case: valid input
	ctrlBuf = []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  key: value
`)
	expected = []crclient.Object{
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-config",
			},
			Data: map[string]string{
				"key": "value",
			},
		},
	}

	result, err = GetCTRLObject(ctrlBuf)

	assert.Nil(t, err)
	assert.Equal(t, result, expected)
}

func TestGetCTRLObjectDeployment(t *testing.T) {
	// Test case: empty input
	ctrlBuf := []byte{}
	expected := []crclient.Object{}

	result, err := GetCTRLObject(ctrlBuf)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}
	assert.Equal(t, result, expected)
	// Test case: valid input
	ctrlBuf = []byte(`
apiVersion: v1
kind: Deployment
metadata:
  name: my-deployment
spec:
  selector:
    matchLabels:
      app: MyApp
  template:
    metadata:
      labels:
        app: MyApp
    spec:
      containers:
        - name: myapp
          image: my-image
          ports:
            - containerPort: 8080
`)
	expected = []crclient.Object{
		&appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-deployment",
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "MyApp",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "MyApp",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "myapp",
								Image: "my-image",
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 8080,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result, err = GetCTRLObject(ctrlBuf)

	assert.Nil(t, err)
	assert.Equal(t, result, expected)
}

// TODO: Cover more object types:
// CustomResourceDefinition, ServiceAccount,
// ClusterRoleBinding, Role, RoleBinding,
// PersistentVolumeClaim, Job, IngressClass,
// Ingress, etc... see the associated method and check its coverage.
func TestGetModuleComponentObj(t *testing.T) {
	// Test case: Valid YAML
	yamlString := []byte(`
apiVersion: v1
kind: Service
metadata:
  name: my-service
spec:
  selector:
    app: MyApp
  ports:
    - protocol: TCP
      port: 80
      targetPort: 9376
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: my-cluster-role
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "watch", "list"]
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  key: value
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
spec:
  replicas: 3
  selector:
    matchLabels:
      app: MyApp
  template:
    metadata:
      labels:
        app: MyApp
    spec:
      containers:
        - name: my-container
          image: my-image
          ports:
            - containerPort: 8080
---
apiVersion: v1
kind: CustomResourceDefinition
metadata:
  name: my-crd
spec:
  group: my-group
  versions:
    - name: v1
      served: true
      storage: true
  scope: Namespaced
  names:
    plural: my-crds
    singular: my-cr
  preserveUnknownFields: false
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-sa
---
apiVersion: v1
kind: ClusterRoleBinding
metadata:
  name: my-crb
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "watch", "list"]
---
apiVersion: v1
kind: Role
metadata:
  name: my-role
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "watch", "list"]
---
apiVersion: v1
kind: RoleBinding
metadata:
  name: my-role-binding
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "watch", "list"]
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
---
apiVersion: batch/v1
kind: Job
metadata:
  name: my-job
spec:
  template:
    spec:
      containers:
        - name: my-container
          image: my-image
        - name: my-other-container
          image: my-other-image
        - name: my-third-container
          image: my-third-image
        - name: my-fourth-container
          image: my-fourth-image
        - name: my-fifth-container
          image: my-fifth-image
      restartPolicy: OnFailure
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  annotations:
    kubernetes.io/ingress.class: my-ingress-class
spec:
  rules:
    - host: my-host
      http:
        paths:
          - path: /
            backend:
              serviceName: my-service
              servicePort: 80
---
apiVersion: v1
kind: ValidatingWebhookConfiguration
metadata:
  name: my-vwc
webhooks:
  - name: my-vwh
    rules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE"]
        # TODO: Add support for "DELETE"
        # TODO: Add support for "CONNECT"
        # TODO: Add support for "PATCH"
        # TODO: Add support for "LIST"
        resources: ["pods"]
---
apiVersion: v1
kind: MutatingWebhookConfiguration
metadata:
  name: my-mwc
webhooks:
  - name: my-mwh
    rules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE"]
        # TODO: Add support for "DELETE"
        # TODO: Add support for "CONNECT"
        # TODO: Add support for "PATCH"
        # TODO: Add support for "LIST"
        resources: ["pods"]
---
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
type: Opaque
data:
  key: dmFsdWU=
---
apiVersion: v1
kind: DaemonSet
metadata:
  name: my-daemonset
spec:
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - name: my-container
          image: my-image
          ports:
            - containerPort: 8080
---
apiVersion: v1
kind: Issuer
metadata:
  name: my-issuer
spec:
  selfSigned: {}
---
apiVersion: v1
kind: Certificate
metadata:
  name: my-certificate
spec:
  secretName: my-secret
  issuerRef:
    name: my-issuer
  dnsNames:
    - my-dns-name
---
apiVersion: v1
kind: StatefulSet
metadata:
  name: my-sts
spec:
  serviceName: my-service
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - name: my-container
          image: my-image
          ports:
            - containerPort: 8080
---
apiVersion: v1
kind: StorageClass
metadata:
  name: my-sc
provisioner: my-provisioner
reclaimPolicy: Delete
volumeBindingMode: Immediate
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: my-pv
spec:
  storageClassName: my-storage-class
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  hostPath:
    path: /tmp/my-pv
---
apiVersion: v1
kind: Namespace
metadata:
  name: my-ns
---
apiVersion: v1
kind: IngressClass
metadata:
  name: my-ic
spec:
  controller: my-ingress-controller
---
`)
	ctrlObjects, err := GetModuleComponentObj(yamlString)
	if err != nil {
		t.Fatalf("Failed to get module component objects: %v", err)
	}

	if len(ctrlObjects) != 23 {
		t.Errorf("Expected 23 objects, got %d", len(ctrlObjects))
	}

	for _, obj := range ctrlObjects {
		switch v := obj.(type) {
		case *corev1.Service:
			if v.Name != "my-service" {
				t.Errorf("Expected service name 'my-service', got %s", v.Name)
			}
		case *rbacv1.ClusterRole:
			if v.Name != "my-cluster-role" {
				t.Errorf("Expected cluster role name 'my-cluster-role', got %s", v.Name)
			}
		case *corev1.ConfigMap:
			if v.Name != "my-config" {
				t.Errorf("Expected config map name 'my-config', got %s", v.Name)
			}
		case *appsv1.Deployment:
			if v.Name != "my-deployment" {
				t.Errorf("Expected deployment name 'my-deployment', got %s", v.Name)
			}
		case *apiextv1.CustomResourceDefinition:
			if v.Name != "my-crd" {
				t.Errorf("Expected custom resource definition name 'my-crd', got %s", v.Name)
			}
		case *corev1.ServiceAccount:
			if v.Name != "my-sa" {
				t.Errorf("Expected service account name 'my-service-account', got %s", v.Name)
			}
		case *rbacv1.ClusterRoleBinding:
			if v.Name != "my-crb" {
				t.Errorf("Expected cluster role binding name 'my-crb', got %s", v.Name)
			}
		case *rbacv1.Role:
			if v.Name != "my-role" {
				t.Errorf("Expected role name 'my-role', got %s", v.Name)
			}
		case *rbacv1.RoleBinding:
			if v.Name != "my-role-binding" {
				t.Errorf("Expected role binding name 'my-role-binding', got %s", v.Name)
			}
		case *corev1.PersistentVolumeClaim:
			if v.Name != "my-pvc" {
				t.Errorf("Expected persistent volume claim name 'my-pvc', got %s", v.Name)
			}
		case *batchv1.Job:
			if v.Name != "my-job" {
				t.Errorf("Expected job name 'my-job', got %s", v.Name)
			}
		case *networking.Ingress:
			if v.Name != "my-ingress" {
				t.Errorf("Expected ingress name 'my-ingress', got %s", v.Name)
			}
		case *admissionregistration.ValidatingWebhookConfiguration:
			if v.Name != "my-vwc" {
				t.Errorf("Expected validating webhook configuration name 'my-vwc', got %s", v.Name)
			}
		case *admissionregistration.MutatingWebhookConfiguration:
			if v.Name != "my-mwc" {
				t.Errorf("Expected mutating webhook configuration name 'my-mwc', got %s", v.Name)
			}
		case *corev1.Secret:
			if v.Name != "my-secret" {
				t.Errorf("Expected secret name 'my-secret', got %s", v.Name)
			}
		case *appsv1.DaemonSet:
			if v.Name != "my-daemonset" {
				t.Errorf("Expected daemon set name 'my-daemonset', got %s", v.Name)
			}
		case *certmanagerv1.Issuer:
			if v.Name != "my-issuer" {
				t.Errorf("Expected issuer name 'my-issuer', got %s", v.Name)
			}
		case *certmanagerv1.Certificate:
			if v.Name != "my-certificate" {
				t.Errorf("Expected certificate name 'my-cert', got %s", v.Name)
			}
		case *appsv1.StatefulSet:
			if v.Name != "my-sts" {
				t.Errorf("Expected stateful set name 'my-sts', got %s", v.Name)
			}
		case *storagev1.StorageClass:
			if v.Name != "my-sc" {
				t.Errorf("Expected storage class name 'my-sc', got %s", v.Name)
			}
		case *corev1.PersistentVolume:
			if v.Name != "my-pv" {
				t.Errorf("Expected persistent volume name 'my-pv', got %s", v.Name)
			}
		case *corev1.Namespace:
			if v.Name != "my-ns" {
				t.Errorf("Expected namespace name 'my-ns', got %s", v.Name)
			}
		case *networking.IngressClass:
			if v.Name != "my-ic" {
				t.Errorf("Expected ingress class name 'my-ic', got %s", v.Name)
			}
		default:
			t.Errorf("Unexpected object type: %T", obj)
		}
	}

	// Test case: Invalid YAML
	invalidYamlString := []byte(`
		apiVersion: v1
		kind: Service
		metadata:
			name: my-service
		spec:
			selector:
				app: MyApp
			ports:
				- protocol: TCP
					port: 80
					targetPort: 9376
		---
		apiVersion: rbac.authorization.k8s.io/v1
		kind: ClusterRole
		metadata:
			name: my-cluster-role
		rules:
		- apiGroups: [""]
			resources: ["pods"]
			verbs: ["get", "watch", "list"]
		---
		apiVersion: v1
		kind: ConfigMap
		metadata:
			name: my-config
		data:
			key: value
		---
		apiVersion: apps/v1
		kind: Deployment
		metadata:
			name: my-deployment
		spec:
			replicas: "invalid"
			selector:
				matchLabels:
					app: MyApp
			template:
				metadata:
					labels:
						app: MyApp
				spec:
					containers:
					- name: my-container
						image: my-image
						ports:
						- containerPort: 8080
					`)

	_, err = GetModuleComponentObj(invalidYamlString)
	assert.NotNil(t, err)
}

// This function is used to test the GetCTRLObject function
// when yamlUnmarshal returns an error
func TestGetCTRLObjectWithErrors(t *testing.T) {
	// Save the original function so we can revert after this test
	defaultYamlUnmarshal := yamlUnmarshal

	// Define the test case
	testCases := []struct {
		name string
		kind string
	}{
		{
			name: "TypeMeta yaml returns error",
			kind: "TypeMeta",
		},

		{
			name: "ClusterRole yaml returns error",
			kind: "ClusterRole",
		},
		{
			name: "ClusterRoleBinding yaml returns error",
			kind: "ClusterRoleBinding",
		},
		{
			name: "Service yaml returns error",
			kind: "Service",
		},
		{
			name: "ConfigMap yaml returns error",
			kind: "ConfigMap",
		},
		{
			name: "Deployment yaml returns error",
			kind: "Deployment",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			yamlStr := "apiVersion: v1\nkind: " + tt.kind + "\nmetadata:\n  name: test\ndata:\n  test-key: test-value\n"
			expectedErr := "mocking error return for " + tt.kind

			// Set the yamlUnmarshal function to return an error when the kind is tt.kind
			yamlUnmarshal = func(data []byte, v interface{}) error {
				vType := reflect.TypeOf(v)
				fmt.Printf("v kind is: %s \n", vType.Elem().String())

				if vType.Elem().String() == "v1."+tt.kind {
					return errors.New(expectedErr)
				}
				return yaml.Unmarshal(data, v)
			}

			// Call GetModuleComponentObj
			_, err := GetCTRLObject([]byte(yamlStr))

			if err == nil {
				t.Errorf("Expected an error, but got nil instead")
			}

			// Check if the error message contains the expected substring
			if !strings.Contains(err.Error(), expectedErr) {
				t.Errorf("Expected error message to contain %s, but got '%s' instead", expectedErr, err.Error())
			}
		})
	}
	// Revert back to the original function
	yamlUnmarshal = defaultYamlUnmarshal
}

// This function is used to test the GetModuleComponentObj function
// when yamlUnmarshal returns an error
func TestGetModuleComponentObjWithErrors(t *testing.T) {
	// Save the original function so we can revert after this test
	defaultYamlUnmarshal := yamlUnmarshal

	// Define the test case
	testCases := []struct {
		name string
		kind string
	}{
		{
			name: "TypeMeta yaml returns error",
			kind: "TypeMeta",
		},
		{
			name: "CustomResourceDefinition yaml returns error",
			kind: "CustomResourceDefinition",
		},
		{
			name: "ServiceAccount yaml returns error",
			kind: "ServiceAccount",
		},
		{
			name: "ClusterRole yaml returns error",
			kind: "ClusterRole",
		},
		{
			name: "ClusterRoleBinding yaml returns error",
			kind: "ClusterRoleBinding",
		},
		{
			name: "Role yaml returns error",
			kind: "Role",
		},
		{
			name: "RoleBinding yaml returns error",
			kind: "RoleBinding",
		},
		{
			name: "Service yaml returns error",
			kind: "Service",
		},
		{
			name: "PersistentVolumeClaim yaml returns error",
			kind: "PersistentVolumeClaim",
		},
		{
			name: "Job yaml returns error",
			kind: "Job",
		},
		{
			name: "IngressClass yaml returns error",
			kind: "IngressClass",
		},
		{
			name: "Ingress yaml returns error",
			kind: "Ingress",
		},
		{
			name: "ValidatingWebhookConfiguration yaml returns error",
			kind: "ValidatingWebhookConfiguration",
		},
		{
			name: "MutatingWebhookConfiguration yaml returns error",
			kind: "MutatingWebhookConfiguration",
		},
		{
			name: "ConfigMap yaml returns error",
			kind: "ConfigMap",
		},
		{
			name: "Secret yaml returns error",
			kind: "Secret",
		},
		{
			name: "Deployment yaml returns error",
			kind: "Deployment",
		},
		{
			name: "DaemonSet yaml returns error",
			kind: "DaemonSet",
		},
		{
			name: "Issuer yaml returns error",
			kind: "Issuer",
		},
		{
			name: "Certificate yaml returns error",
			kind: "Certificate",
		},
		{
			name: "StatefulSet yaml returns error",
			kind: "StatefulSet",
		},
		{
			name: "StorageClass yaml returns error",
			kind: "StorageClass",
		},
		{
			name: "PersistentVolume yaml returns error",
			kind: "PersistentVolume",
		},
		{
			name: "Namespace yaml returns error",
			kind: "Namespace",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			yamlStr := "apiVersion: v1\nkind: " + tt.kind + "\nmetadata:\n  name: test\ndata:\n  test-key: test-value\n"
			expectedErr := "mocking error return for " + tt.kind

			// Set the yamlUnmarshal function to return an error when the kind is tt.kind
			yamlUnmarshal = func(data []byte, v interface{}) error {
				vType := reflect.TypeOf(v)
				fmt.Printf("v kind is: %s \n", vType.Elem().String())

				if vType.Elem().String() == "v1."+tt.kind {
					return errors.New(expectedErr)
				}
				return yaml.Unmarshal(data, v)
			}

			// Call GetModuleComponentObj
			_, err := GetModuleComponentObj([]byte(yamlStr))

			if err == nil {
				t.Errorf("Expected an error, but got nil instead")
			}

			// Check if the error message contains the expected substring
			if !strings.Contains(err.Error(), expectedErr) {
				t.Errorf("Expected error message to contain %s, but got '%s' instead", expectedErr, err.Error())
			}
		})
	}
	// Revert back to the original function
	yamlUnmarshal = defaultYamlUnmarshal
}

// This function is used to test the GetDriverYaml function
// when yamlUnmarshal returns an error
func TestGetDriverYamlWithErrors(t *testing.T) {
	// Save the original function so we can revert after this test
	defaultYamlUnmarshal := yamlUnmarshal

	// Define the test case
	testCases := []struct {
		name string
		kind string
		set  string
	}{
		{
			name: "TypeMeta yaml returns error",
			kind: "TypeMeta",
			set:  "Deployment",
		},

		{
			name: "ClusterRole yaml returns error",
			kind: "ClusterRole",
			set:  "Deployment",
		},
		{
			name: "ClusterRoleBinding yaml returns error",
			kind: "ClusterRoleBinding",
			set:  "Deployment",
		},
		{
			name: "ServiceAccount yaml returns error",
			kind: "ServiceAccount",
			set:  "Deployment",
		},
		{
			name: "DeploymentApplyConfiguration yaml returns error",
			kind: "DeploymentApplyConfiguration",
			set:  "Deployment",
		},
		{
			name: "DaemonSetApplyConfiguration yaml returns error",
			kind: "DaemonSetApplyConfiguration",
			set:  "DaemonSet",
		},
		{
			name: "Role yaml returns error",
			kind: "Role",
			set:  "Deployment",
		},
		{
			name: "RoleBinding yaml returns error",
			kind: "RoleBinding",
			set:  "Deployment",
		},
		{
			name: "Invalid kind returns error",
			kind: "Service",
			set:  "Invalid",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			yamlStr := "apiVersion: v1\nkind: " + tt.kind + "\nmetadata:\n  name: test\ndata:\n  test-key: test-value\n"
			expectedErr := "mocking error return for " + tt.kind
			if tt.set == "Invalid" {
				expectedErr = "unsupported kind " + tt.set
			}

			// Set the yamlUnmarshal function to return an error when the kind is tt.kind
			yamlUnmarshal = func(data []byte, v interface{}) error {
				vType := reflect.TypeOf(v)
				fmt.Printf("v kind is: %s \n", vType.Elem().String())

				if vType.Elem().String() == "v1."+tt.kind {
					return errors.New(expectedErr)
				}
				return yaml.Unmarshal(data, v)
			}

			// Call GetModuleComponentObj
			_, err := GetDriverYaml(yamlStr, tt.set)

			if err == nil {
				t.Errorf("Expected an error, but got nil instead")
			}

			// Check if the error message contains the expected substring
			if !strings.Contains(err.Error(), expectedErr) {
				t.Errorf("Expected error message to contain %s, but got '%s' instead", expectedErr, err.Error())
			}
		})
	}
	// Revert back to the original function
	yamlUnmarshal = defaultYamlUnmarshal
}

func TestGetDriverYaml(t *testing.T) {
	// Test case: Valid YAML - DaemonSet
	// TODO: This string does not cover all elements necessary to build the rbac object.
	yamlString := `
apiVersion: v1
kind: Service
metadata:
  name: my-service
spec:
  selector:
    app: MyApp
  ports:
    - protocol: TCP
      port: 80
      targetPort: 9376
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-service
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: my-cluster-role
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "watch", "list"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: my-service
subjects:
  - kind: ServiceAccount
    name: my-service
roleRef:
  kind: ClusterRole
  name: my-cluster-role
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cluster-role
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "watch", "list"]
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: my-service
subjects:
  - kind: ServiceAccount
    name: my-service
roleRef:
  kind: Role
  name: cluster-role
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  key: value
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: my-daemonset
spec:
  replicas: 3
  selector:
    matchLabels:
      app: MyApp
  template:
    metadata:
      labels:
        app: MyApp
    spec:
      containers:
      - name: my-container
        image: my-image
        ports:
        - containerPort: 8080
    `

	ctrlObject, err := GetDriverYaml(yamlString, "DaemonSet")
	appsv1 := "apps/v1"
	daemonset := "DaemonSet"
	myDaemonset := "my-daemonset"
	myContainer := "my-container"
	myImage := "my-image"
	port8080 := int32(8080)

	expected := NodeYAML{
		DaemonSetApplyConfig: confv1.DaemonSetApplyConfiguration{
			TypeMetaApplyConfiguration: confmetav1.TypeMetaApplyConfiguration{
				APIVersion: &appsv1,
				Kind:       &daemonset,
			},
			ObjectMetaApplyConfiguration: &confmetav1.ObjectMetaApplyConfiguration{
				Name: &myDaemonset,
			},
			Spec: &confv1.DaemonSetSpecApplyConfiguration{
				// Spec configuration
				Selector: &confmetav1.LabelSelectorApplyConfiguration{
					MatchLabels: map[string]string{
						"app": "MyApp",
					},
				},
				Template: &confcorev1.PodTemplateSpecApplyConfiguration{
					ObjectMetaApplyConfiguration: &confmetav1.ObjectMetaApplyConfiguration{
						Labels: map[string]string{
							"app": "MyApp",
						},
					},
					// Template configuration
					Spec: &confcorev1.PodSpecApplyConfiguration{
						Containers: []confcorev1.ContainerApplyConfiguration{
							{
								// Container configuration
								Name:  &myContainer,
								Image: &myImage,
								Ports: []confcorev1.ContainerPortApplyConfiguration{
									{
										ContainerPort: &port8080,
									},
								},
							},
						},
					},
				},
			},
			Status: nil,
		},
		Rbac: RbacYAML{
			ServiceAccount: corev1.ServiceAccount{
				// ServiceAccount configuration
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-service",
				},
			},
			ClusterRole: rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRole",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster-role",
				},
				Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "watch", "list"}}},
			},
			ClusterRoleBinding: rbacv1.ClusterRoleBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRoleBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-service",
				},
				Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "my-service"}},
				RoleRef:  rbacv1.RoleRef{Kind: "ClusterRole", Name: "my-cluster-role", APIGroup: "rbac.authorization.k8s.io"},
			},
			Role: rbacv1.Role{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "Role",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-role",
				},
				Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "watch", "list"}}},
			},
			RoleBinding: rbacv1.RoleBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "RoleBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-service",
				},
				Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "my-service"}},
				RoleRef:  rbacv1.RoleRef{Kind: "Role", Name: "cluster-role", APIGroup: "rbac.authorization.k8s.io"},
			},
		},
	}

	nodeYaml := ctrlObject.(NodeYAML)
	assert.Nil(t, err)
	assert.Equal(t, expected, nodeYaml)

	yamlString = `
apiVersion: v1
kind: Service
metadata:
  name: my-service
spec:
  selector:
    app: MyApp
  ports:
    - protocol: TCP
      port: 80
      targetPort: 9376
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-service
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: my-cluster-role
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "watch", "list"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: my-service
subjects:
  - kind: ServiceAccount
    name: my-service
roleRef:
  kind: ClusterRole
  name: my-cluster-role
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: my-role
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "watch", "list"]
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: my-service
subjects:
  - kind: ServiceAccount
    name: my-service
roleRef:
  kind: Role
  name: my-role
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  key: value
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-controller
spec:
  selector:
    matchLabels:
      app: MyApp
  template:
    metadata:
      labels:
        app: MyApp
    spec:
      containers:
        - name: my-container
          image: my-image
          ports:
          - containerPort: 8080
    `

	deployment := "Deployment"
	myController := "my-controller"

	expectedController := ControllerYAML{
		Deployment: confv1.DeploymentApplyConfiguration{
			TypeMetaApplyConfiguration: confmetav1.TypeMetaApplyConfiguration{
				APIVersion: &appsv1,
				Kind:       &deployment,
			},
			ObjectMetaApplyConfiguration: &confmetav1.ObjectMetaApplyConfiguration{
				Name: &myController,
			},
			Spec: &confv1.DeploymentSpecApplyConfiguration{
				// Spec configuration
				Selector: &confmetav1.LabelSelectorApplyConfiguration{
					MatchLabels: map[string]string{
						"app": "MyApp",
					},
				},
				Template: &confcorev1.PodTemplateSpecApplyConfiguration{
					ObjectMetaApplyConfiguration: &confmetav1.ObjectMetaApplyConfiguration{
						Labels: map[string]string{
							"app": "MyApp",
						},
					},
					// Template configuration
					Spec: &confcorev1.PodSpecApplyConfiguration{
						Containers: []confcorev1.ContainerApplyConfiguration{
							{
								// Container configuration
								Name:  &myContainer,
								Image: &myImage,
								Ports: []confcorev1.ContainerPortApplyConfiguration{
									{
										ContainerPort: &port8080,
									},
								},
							},
						},
					},
				},
			},
			Status: nil,
		},
		Rbac: RbacYAML{
			ServiceAccount: corev1.ServiceAccount{
				// ServiceAccount configuration
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-service",
				},
			},
			ClusterRole: rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRole",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster-role",
				},
				Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "watch", "list"}}},
			},
			ClusterRoleBinding: rbacv1.ClusterRoleBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRoleBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-service",
				},
				Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "my-service"}},
				RoleRef:  rbacv1.RoleRef{Kind: "ClusterRole", Name: "my-cluster-role", APIGroup: "rbac.authorization.k8s.io"},
			},
			Role: rbacv1.Role{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "Role",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-role",
				},
				Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "watch", "list"}}},
			},
			RoleBinding: rbacv1.RoleBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "RoleBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-service",
				},
				Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "my-service"}},
				RoleRef:  rbacv1.RoleRef{Kind: "Role", Name: "my-role", APIGroup: "rbac.authorization.k8s.io"},
			},
		},
	}

	ctrlObject, err = GetDriverYaml(yamlString, "Deployment")
	assert.Nil(t, err)

	controllerYaml := ctrlObject.(ControllerYAML)
	assert.Nil(t, err)
	assert.Equal(t, expectedController, controllerYaml)

	// Test case: Invalid YAML
	invalidYamlString := `
		apiVersion: v1
		kind: Service
		metadata:
			name: my-service
		spec:
			selector:
				app: MyApp
			ports:
				- protocol: TCP
					port: 80
					targetPort: 9376
		---
		apiVersion: rbac.authorization.k8s.io/v1
		kind: ClusterRole
		metadata:
			name: my-cluster-role
		rules:
		- apiGroups: [""]
			resources: ["pods"]
			verbs: ["get", "watch", "list"]
		---
		apiVersion: v1
		kind: ConfigMap
		metadata:
			name: my-config
		data:
			key: value
		---
		apiVersion: apps/v1
		kind: Deployment
		metadata:
			name: my-deployment
		spec:
			replicas: "invalid"
			selector:
				matchLabels:
					app: MyApp
			template:
				metadata:
					labels:
						app: MyApp
				spec:
					containers:
					- name: my-container
						image: my-image
						ports:
						- containerPort: 8080
					`

	_, err = GetDriverYaml(invalidYamlString, "Deployment")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	// Test case: Invalid kind
	_, err = GetDriverYaml(`---`, "invalid kind")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestDeleteObject(t *testing.T) {
	// Test case: Delete object successfully
	ctx := context.Background()
	ctrlClient := fullFakeClient()

	obj := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pod",
			Namespace: "my-namespace",
		},
	}

	err := ctrlClient.Create(ctx, obj)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = DeleteObject(ctx, obj, ctrlClient)
	assert.Nil(t, err)

	// Test case: Object not found
	// just try to delete the same object that we know is no longer there (since it was just deleted)

	if err := DeleteObject(ctx, obj, ctrlClient); err != nil {
		t.Errorf("Failed to delete object: %v", err)
	}
}

func TestApplyCTRLObject(t *testing.T) {
	// Test case: Create a new object
	ctx := context.Background()
	ctrlClient := fullFakeClient()

	obj := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pod",
			Namespace: "my-namespace",
		},
	}

	err := ApplyCTRLObject(ctx, obj, ctrlClient)
	assert.Nil(t, err)

	if err := ApplyCTRLObject(ctx, obj, ctrlClient); err != nil {
		t.Errorf("Failed to apply object: %v", err)
	}
}

func TestApplyObject(t *testing.T) {
	// Test case: Create a new object
	ctx := context.Background()
	ctrlClient := fullFakeClient()

	obj := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pod",
			Namespace: "my-namespace",
		},
	}

	err := ApplyObject(ctx, obj, ctrlClient)
	assert.Nil(t, err)

	// Test case: Update an existing object
	obj.Labels = map[string]string{"key": "value"}

	err = ApplyObject(ctx, obj, ctrlClient)
	assert.Nil(t, err)

	// Test case: Error during object creation
	// TODO: Come up with some way to inject an error during creation
	// apply the same object as before doesn't trigger an error...
}

// TODO: This is where I left off. Come back tomorrow.
func TestLogEndReconcile(t *testing.T) {
	// Call the function
	output := captureOutput(func() { LogEndReconcile() })

	expectedOutput := "################End Reconcile##############\n"
	if output != expectedOutput {
		t.Errorf("Expected output %q, but got %q", expectedOutput, output)
	}
}

func TestGetModuleDefaultVersion(t *testing.T) {
	tests := []struct {
		name             string
		driverConfig     string
		driverType       csmv1.DriverType
		moduleType       csmv1.ModuleType
		path             string
		expectedVersion  string
		expectedErrorMsg string
	}{
		{
			name:             "valid version",
			driverConfig:     "v2.14.0",
			driverType:       csmv1.PowerScale,
			moduleType:       csmv1.Observability,
			path:             "../../operatorconfig",
			expectedVersion:  "v1.12.0",
			expectedErrorMsg: "",
		},
		{
			name:             "invalid version",
			driverConfig:     "v20.12.0",
			driverType:       csmv1.PowerScale,
			moduleType:       csmv1.Observability,
			path:             "../../operatorconfig",
			expectedVersion:  "",
			expectedErrorMsg: "does not exist in file ../../operatorconfig/moduleconfig/common/version-values.yaml",
		},
		{
			name:             "invalid module",
			driverConfig:     "v2.12.0",
			driverType:       csmv1.PowerScale,
			moduleType:       "invalid",
			path:             "../../operatorconfig",
			expectedVersion:  "",
			expectedErrorMsg: "does not exist in file ../../operatorconfig/moduleconfig/common/version-values.yaml",
		},
		{
			name:             "invalide driver",
			driverConfig:     "v2.12.0",
			driverType:       "invalid",
			moduleType:       csmv1.Observability,
			path:             "../../operatorconfig",
			expectedVersion:  "",
			expectedErrorMsg: "does not exist in file ../../operatorconfig/moduleconfig/common/version-values.yaml",
		},
		{
			name:             "GetModuleDefaultVersion when yamlUnmarshal returns an error",
			driverConfig:     "v2.13.0",
			driverType:       csmv1.PowerScale,
			moduleType:       csmv1.Observability,
			path:             "../../operatorconfig",
			expectedVersion:  "",
			expectedErrorMsg: "mock error from yamlUnmarshal",
		},
	}

	// Save the original function so we can revert after each test case
	defaultYamlUnmarshal := yamlUnmarshal

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// if test name contains yamlUnmarshal, we will use a different yamlUnmarshal function to mock an error
			if strings.Contains(tt.name, "yamlUnmarshal") {
				yamlUnmarshal = func(_ []byte, _ interface{}) error {
					return fmt.Errorf("mock error from yamlUnmarshal")
				}
			}
			version, err := GetModuleDefaultVersion(tt.driverConfig, tt.driverType, tt.moduleType, tt.path)
			// Revert to the original function
			yamlUnmarshal = defaultYamlUnmarshal
			if tt.expectedErrorMsg != "" {
				if err == nil {
					t.Errorf("expected error containing %q, but got nil", tt.expectedErrorMsg)
				} else if !strings.Contains(err.Error(), tt.expectedErrorMsg) {
					t.Errorf("expected error containing %q, but got %v", tt.expectedErrorMsg, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if version != tt.expectedVersion {
				t.Errorf("expected version %q, but got %q", tt.expectedVersion, version)
			}
		})
	}
}

func TestVersionParser(t *testing.T) {
	tests := []struct {
		name          string
		driverConfig  string
		expectedMajor int
		expectedMinor int
		expectedError string
	}{
		{
			name:          "valid version",
			driverConfig:  "v2.14.0",
			expectedMajor: 2,
			expectedMinor: 14,
			expectedError: "",
		},
		{
			name:          "invalid version",
			driverConfig:  "v2.12",
			expectedMajor: -1,
			expectedMinor: -1,
			expectedError: "not in correct version format",
		},
		{
			name:          "valid version alt format - no leading v",
			driverConfig:  "2.14.0",
			expectedMajor: 2,
			expectedMinor: 14,
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Println("test case: ", tt.name)
			majorVersion, minorVersion, err := versionParser(tt.driverConfig)
			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("expected error containing %q, but got nil", tt.expectedError)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error containing %q, but got %v", tt.expectedError, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if majorVersion != tt.expectedMajor {
				t.Errorf("expected major version %d, but got %d", tt.expectedMajor, majorVersion)
			}

			if minorVersion != tt.expectedMinor {
				t.Errorf("expected minor version %d, but got %d", tt.expectedMinor, minorVersion)
			}
		})
	}
}

func TestMinVersionCheck(t *testing.T) {
	tests := []struct {
		name           string
		minVersion     string
		version        string
		expectedResult bool
		expectedError  string
	}{
		{
			name:           "valid version",
			minVersion:     "v2.12.0",
			version:        "v2.12.1",
			expectedResult: true,
			expectedError:  "",
		},
		{
			name:           "invalid version",
			minVersion:     "v2.12.0",
			version:        "v2.11.0",
			expectedResult: false,
			expectedError:  "",
		},
		{
			name:           "invalid version format",
			minVersion:     "v2.12.0",
			version:        "v2.12",
			expectedResult: false,
			expectedError:  "not in correct version format",
		},
		{
			name:           "major greater than minimum major",
			minVersion:     "v2.12.0",
			version:        "v3.12.0",
			expectedResult: true,
			expectedError:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MinVersionCheck(tt.minVersion, tt.version)
			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("expected error containing %q, but got nil", tt.expectedError)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error containing %q, but got %v", tt.expectedError, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result != tt.expectedResult {
				t.Errorf("expected result %v, but got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestGetConfigData(t *testing.T) {
	// Create a fake context.Context
	ctx := context.Background()

	// Create a fake ctrlClient
	ctrlClient := fullFakeClient()

	// Create a fake clusterID
	clusterID := "test-cluster"

	// Create a fake secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterID,
			Namespace: ReplicationControllerNameSpace,
		},
		Data: map[string][]byte{
			"data": []byte("test-data"),
		},
	}

	// Add the secret to the ctrlClient
	if err := ctrlClient.Create(ctx, secret); err != nil {
		t.Fatalf("failed to create secret: %v", err)
	}

	// Call the function
	configData, err := getConfigData(ctx, clusterID, ctrlClient)

	// Assert the expected result
	assert.Nil(t, err)
	assert.Equal(t, configData, secret.Data["data"])

	// TODO: Add a test case for checking for a secret that isn't there
}

func TestGetCSMNamespaces(t *testing.T) {
	// Create a fake context.Context
	ctx := context.Background()

	// CSM types must be registered with the scheme
	scheme := runtime.NewScheme()
	_ = csmv1.AddToScheme(scheme)  // for CSM objects
	_ = corev1.AddToScheme(scheme) // for namespaces

	// Create a fake ctrlClient
	ctrlClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	// Create fake namespaces
	ns1 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}

	ns2 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ns-2",
		},
	}

	// Add the namespaces to the ctrlClient
	if err := ctrlClient.Create(ctx, ns1); err != nil {
		t.Fatalf("failed to create ns: %v", err)
	}
	if err := ctrlClient.Create(ctx, ns2); err != nil {
		t.Fatalf("failed to create ns: %v", err)
	}

	// Create fake CSM objects and add those
	csm1 := createCR(csmv1.PowerFlex, csmv1.Replication, true, nil)
	csm1.ObjectMeta = metav1.ObjectMeta{
		Name:      "test-csm-obj",
		Namespace: "test-namespace",
	}
	if err := ctrlClient.Create(ctx, csm1); err != nil {
		t.Fatalf("failed to create csm object: %v", err)
	}

	// Call the function
	namespaces, err := GetCSMNamespaces(ctx, ctrlClient)
	// Assert the expected result
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(namespaces) != 1 {
		t.Errorf("expected 1 namespaces, got %d", len(namespaces))
	}
	if namespaces[0] != "test-namespace" {
		t.Errorf("expected namespace %s, got %s", "test-namespace", namespaces[0])
	}
}

func TestContains(t *testing.T) {
	// Test case: slice contains the specified string
	slice := []string{"apple", "banana", "cherry"}
	str := "banana"
	expected := true
	result := Contains(slice, str)
	if result != expected {
		t.Errorf("Expected %v, but got %v", expected, result)
	}

	// Test case: slice does not contain the specified string
	slice = []string{"apple", "banana", "cherry"}
	str = "grape"
	expected = false
	result = Contains(slice, str)
	if result != expected {
		t.Errorf("Expected %v, but got %v", expected, result)
	}

	// Test case: empty slice
	slice = []string{}
	str = "apple"
	expected = false
	result = Contains(slice, str)
	if result != expected {
		t.Errorf("Expected %v, but got %v", expected, result)
	}
}

func TestIsResiliencyModuleEnabled(t *testing.T) {
	// Test case: resiliency module is enabled
	instance := csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Modules: []csmv1.Module{
				{
					Name:    csmv1.Resiliency,
					Enabled: true,
				},
			},
		},
	}

	expected := true
	result := IsResiliencyModuleEnabled(context.Background(), instance, nil)
	if result != expected {
		t.Errorf("Expected %v, but got %v", expected, result)
	}

	// Test case: resiliency module is disabled
	instance.Spec.Modules[0].Enabled = false
	expected = false
	result = IsResiliencyModuleEnabled(context.Background(), instance, nil)
	if result != expected {
		t.Errorf("Expected %v, but got %v", expected, result)
	}
}

func TestGetSecret(t *testing.T) {
	ctx := context.Background()
	ctrlClient := fullFakeClient()

	// Create a fake secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"data": []byte("test-data"),
		},
	}

	// Add the secret to the ctrlClient
	if err := ctrlClient.Create(ctx, secret); err != nil {
		t.Fatalf("failed to create secret: %v", err)
	}

	// Call the function
	found, err := GetSecret(ctx, "test-secret", "test-namespace", ctrlClient)

	// Assert the expected result
	assert.Nil(t, err)
	assert.Equal(t, found.Name, "test-secret")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if found.Name != "test-secret" {
		t.Errorf("expected name %s, got %s", "test-secret", found.Name)
	}

	// error case: secret doesn't exist
	if err := ctrlClient.Delete(ctx, secret); err != nil {
		t.Fatalf("failed to create secret: %v", err)
	}

	// Call the function
	_, err = GetSecret(ctx, "test-secret", "test-namespace", ctrlClient)

	// Assert the expected result
	assert.NotNil(t, err)
}

func TestDetermineUnitTestRun(t *testing.T) {
	// Test case: UNIT_TEST environment variable is not set
	ctx := context.Background()

	result := DetermineUnitTestRun(ctx)
	if result {
		t.Errorf("Expected false, but got %v", result)
	}

	// Test case: UNIT_TEST environment variable is set to "true"
	t.Setenv("UNIT_TEST", "true")
	result = DetermineUnitTestRun(ctx)
	if !result {
		t.Errorf("Expected true, but got %v", result)
	}

	// Test case: UNIT_TEST environment variable is set to "false"
	t.Setenv("UNIT_TEST", "false")
	result = DetermineUnitTestRun(ctx)
	if result {
		t.Errorf("Expected false, but got %v", result)
	}
}

func TestIsValidUpgrade(t *testing.T) {
	ctx := context.Background()

	csmComponentType := csmv1.Authorization
	operatorConfig := OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}

	// Test case: upgrade is valid
	oldVersion := "v1.11.0"
	newVersion := "v1.12.0"
	expectedIsValid := true

	isValid, err := IsValidUpgrade(ctx, oldVersion, newVersion, csmComponentType, operatorConfig)
	assert.Nil(t, err)
	assert.Equal(t, isValid, expectedIsValid)

	// Test case: downgrade is valid
	oldVersion = "v1.12.0"
	newVersion = "v1.11.0"
	expectedIsValid = true

	isValid, err = IsValidUpgrade(ctx, oldVersion, newVersion, csmComponentType, operatorConfig)
	assert.Nil(t, err)
	assert.Equal(t, isValid, expectedIsValid)

	// Test case: upgrade is not valid
	oldVersion = "v1.11.0"
	newVersion = "v1.99.0"
	expectedIsValid = false

	isValid, err = IsValidUpgrade(ctx, oldVersion, newVersion, csmComponentType, operatorConfig)
	assert.NotNil(t, err)
	assert.Equal(t, isValid, expectedIsValid)

	// Test case: downgrade is not valid
	oldVersion = "v1.11.0"
	newVersion = "v1.0.0"
	expectedIsValid = false

	isValid, err = IsValidUpgrade(ctx, oldVersion, newVersion, csmComponentType, operatorConfig)
	assert.NotNil(t, err)
	assert.Equal(t, isValid, expectedIsValid)

	// Test case: same version-- just a config update, no upgrade/downgrade
	oldVersion = "v1.11.0"
	newVersion = "v1.11.0"
	expectedIsValid = true

	isValid, err = IsValidUpgrade(ctx, oldVersion, newVersion, csmComponentType, operatorConfig)
	assert.Nil(t, err)
	assert.Equal(t, isValid, expectedIsValid)

	// Test case: upgrade is not valid 2
	oldVersion = "v1.12.0"
	newVersion = "v2.0.0"

	isValid, err = IsValidUpgrade(ctx, oldVersion, newVersion, csmComponentType, operatorConfig)
	assert.NotNil(t, err)
	assert.Equal(t, isValid, false)
}

func TestGetUpgradeInfo(t *testing.T) {
	ctx := context.Background()

	// Test case: corrupted upgrade path file
	oldVersion := "v2.2.0"

	// Create a malformed upgrade path file
	tempDir := t.TempDir()
	configDir := fmt.Sprintf("%s/moduleconfig/authorization/%s", tempDir, oldVersion)
	defer os.RemoveAll(configDir)

	err := os.MkdirAll(configDir, 0o700)
	assert.NoError(t, err)

	err = os.WriteFile(fmt.Sprintf("%s/upgrade-path.yaml", configDir),
		[]byte("not a real yaml file"), 0o600)
	assert.NoError(t, err)

	csmComponentType := csmv1.Authorization
	operatorConfig := OperatorConfig{
		ConfigDirectory: tempDir,
	}

	_, err = getUpgradeInfo(ctx, operatorConfig, csmComponentType, oldVersion)
	assert.NotNil(t, err)
}

func TestGetClusterCtrlClient(t *testing.T) {
	// Create a fake context.Context
	ctx := context.Background()

	// CSM types must be registered with the scheme
	scheme := runtime.NewScheme()
	_ = csmv1.AddToScheme(scheme)  // for CSM objects
	_ = corev1.AddToScheme(scheme) // for namespaces

	// Create a fake ctrlClient
	ctrlClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	// Create a fake clusterID
	clusterID := "test-cluster"

	// Create a fake secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterID,
			Namespace: ReplicationControllerNameSpace,
		},
		Data: map[string][]byte{
			"data": []byte("test-data"),
		},
	}

	// Add the secret to the ctrlClient
	if err := ctrlClient.Create(ctx, secret); err != nil {
		t.Fatalf("failed to create secret: %v", err)
	}

	// Call the function
	clusterCtrlClient, err := getClusterCtrlClient(ctx, clusterID, ctrlClient)

	// Assert the expected result
	assert.Error(t, err)
	assert.Nil(t, clusterCtrlClient)
}

func TestGetClusterK8SClient(t *testing.T) {
	// Create a fake context.Context
	ctx := context.Background()

	// CSM types must be registered with the scheme
	scheme := runtime.NewScheme()
	_ = csmv1.AddToScheme(scheme)  // for CSM objects
	_ = corev1.AddToScheme(scheme) // for namespaces

	// Create a fake ctrlClient
	ctrlClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	// Create a fake clusterID
	clusterID := "test-cluster"

	// Create a fake secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterID,
			Namespace: ReplicationControllerNameSpace,
		},
		Data: map[string][]byte{
			"data": []byte("test-data"),
		},
	}

	// Add the secret to the ctrlClient
	if err := ctrlClient.Create(ctx, secret); err != nil {
		t.Fatalf("failed to create secret: %v", err)
	}

	// Call the function
	clusterCtrlClient, err := getClusterK8SClient(ctx, clusterID, ctrlClient)

	// Assert the expected result
	assert.Error(t, err)
	assert.Nil(t, clusterCtrlClient)
}

func TestReplaceAllApplyCustomEnvs(t *testing.T) {
	test := "test"
	newValue := "new"

	tests := []struct {
		driverEnv   []acorev1.EnvVarApplyConfiguration
		commonEnv   []corev1.EnvVar
		nrEnv       []corev1.EnvVar
		expectedEnv []acorev1.EnvVarApplyConfiguration
	}{
		{
			driverEnv: []acorev1.EnvVarApplyConfiguration{
				{
					Name:  &test,
					Value: &test,
				},
				{
					Name:  &newValue,
					Value: &newValue,
					ValueFrom: &acorev1.EnvVarSourceApplyConfiguration{
						SecretKeyRef: &acorev1.SecretKeySelectorApplyConfiguration{
							LocalObjectReferenceApplyConfiguration: acorev1.LocalObjectReferenceApplyConfiguration{
								Name: &test,
							},
							Key:      &test,
							Optional: &[]bool{true}[0],
						},
					},
				},
			},
			commonEnv: []corev1.EnvVar{
				{
					Name:  "test",
					Value: "test",
				},
				{
					Name:  "empty",
					Value: "",
				},
			},
			nrEnv: []corev1.EnvVar{
				{
					Name:  "test",
					Value: "test",
				},
				{
					Name:  "empty",
					Value: "",
				},
			},
			expectedEnv: []acorev1.EnvVarApplyConfiguration{
				{
					Name:  &test,
					Value: &test,
				},
				{
					Name:  &newValue,
					Value: nil,
					ValueFrom: &acorev1.EnvVarSourceApplyConfiguration{
						SecretKeyRef: &acorev1.SecretKeySelectorApplyConfiguration{
							LocalObjectReferenceApplyConfiguration: acorev1.LocalObjectReferenceApplyConfiguration{
								Name: &test,
							},
							Key:      &test,
							Optional: &[]bool{true}[0],
						},
					},
				},
			},
		},
		{
			driverEnv: []acorev1.EnvVarApplyConfiguration{
				{
					Name:  &test,
					Value: &test,
				},
				{
					Name:  &newValue,
					Value: &newValue,
					ValueFrom: &acorev1.EnvVarSourceApplyConfiguration{
						FieldRef: &acorev1.ObjectFieldSelectorApplyConfiguration{
							FieldPath: &test,
						},
					},
				},
			},
			commonEnv: []corev1.EnvVar{
				{
					Name:  "test",
					Value: "test",
				},
				{
					Name:  "empty",
					Value: "",
				},
			},
			nrEnv: []corev1.EnvVar{
				{
					Name:  "test",
					Value: "test",
				},
				{
					Name:  "empty",
					Value: "",
				},
			},
			expectedEnv: []acorev1.EnvVarApplyConfiguration{
				{
					Name:  &test,
					Value: &test,
				},
				{
					Name:  &newValue,
					Value: nil,
					ValueFrom: &acorev1.EnvVarSourceApplyConfiguration{
						FieldRef: &acorev1.ObjectFieldSelectorApplyConfiguration{
							FieldPath: &test,
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		result := ReplaceAllApplyCustomEnvs(test.driverEnv, test.commonEnv, test.nrEnv)
		assert.Equal(t, test.expectedEnv, result)
	}
}

func Test_getUpgradeInfo(t *testing.T) {
	type args struct {
		ctx            context.Context
		operatorConfig OperatorConfig
		csmCompType    csmv1.ModuleType
		oldVersion     string
	}
	tests := []struct {
		name        string
		args        args
		want        string
		expectedErr string
	}{
		{
			name: "yamlUnmarshal returns error",
			args: args{
				ctx: context.Background(),
				operatorConfig: OperatorConfig{
					ConfigDirectory: "../../operatorconfig",
				},
				csmCompType: csmv1.Authorization,
				oldVersion:  "v2.2.0",
			},
			want:        "",
			expectedErr: "mock yamlUnmarshal error",
		},
	}

	// Save the original function so we can revert after this test
	defaultYamlUnmarshal := yamlUnmarshal
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// if test name contains yamlUnmarshal, we will use a different yamlUnmarshal function to mock an error
			if strings.Contains(tt.name, "yamlUnmarshal") {
				yamlUnmarshal = func(_ []byte, _ interface{}) error {
					return fmt.Errorf("mock yamlUnmarshal error")
				}
			}
			got, err := getUpgradeInfo(tt.args.ctx, tt.args.operatorConfig, tt.args.csmCompType, tt.args.oldVersion)
			// Revert to the original function
			yamlUnmarshal = defaultYamlUnmarshal
			if (err != nil) && err.Error() != tt.expectedErr {
				t.Errorf("getUpgradeInfo() returned error = %v, but expected error to be: %v", err, tt.expectedErr)
				return
			}
			if got != tt.want {
				t.Errorf("getUpgradeInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getDefaultComponents(t *testing.T) {
	type args struct {
		driverType csmv1.DriverType
		module     csmv1.ModuleType
		op         OperatorConfig
	}
	tests := []struct {
		name        string
		args        args
		want        []csmv1.ContainerTemplate
		expectedErr string
	}{
		{
			name: "yamlUnmarshal returns error",
			args: args{
				driverType: csmv1.PowerFlex,
				module:     csmv1.Observability,
				op: OperatorConfig{
					ConfigDirectory: "../../operatorconfig",
				},
			},
			want:        nil,
			expectedErr: "failed to unmarshal default-components.yaml for observability: mock yamlUnmarshal error",
		},
	}
	defaultYamlUnmarshal := yamlUnmarshal
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// if test name contains yamlUnmarshal, we will use a different yamlUnmarshal function to mock an error
			if strings.Contains(tt.name, "yamlUnmarshal") {
				yamlUnmarshal = func(_ []byte, _ interface{}) error {
					return fmt.Errorf("mock yamlUnmarshal error")
				}
			}
			got, err := getDefaultComponents(tt.args.driverType, tt.args.module, tt.args.op)
			// Revert to the original function
			yamlUnmarshal = defaultYamlUnmarshal
			if (err != nil) && err.Error() != tt.expectedErr {
				t.Errorf("getDefaultComponents() returned error = %v, but expected error to be: %v", err, tt.expectedErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDefaultComponents() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetEnvironmentVariable(t *testing.T) {
	// Test case: Environment variable is set
	os.Setenv("TEST_VAR", "test_value")
	value, err := GetEnvironmentVariable("TEST_VAR")
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}
	if value != "test_value" {
		t.Errorf("Expected value 'test_value', but got '%s'", value)
	}
	os.Unsetenv("TEST_VAR")

	// Test case: Environment variable is not set
	_, err = GetEnvironmentVariable("NON_EXISTENT_VAR")
	if err == nil {
		t.Errorf("Expected error, but got nil")
	}
}

func TestGetVersion(t *testing.T) {
	ctx := context.Background()
	newCSM := func(specVersion, configVersion string, platform csmv1.DriverType) *csmv1.ContainerStorageModule {
		return &csmv1.ContainerStorageModule{
			Spec: csmv1.ContainerStorageModuleSpec{
				Version: specVersion,
				Driver: csmv1.Driver{
					CSIDriverType: platform,
					ConfigVersion: configVersion,
					Common: &csmv1.ContainerTemplate{
						ImagePullPolicy: corev1.PullAlways,
					},
				},
			},
		}
	}

	// Helper for creating a CR with an AuthorizationServer module
	newCSMWithAuthModule := func(specVersion, driverConfigVersion, moduleConfigVersion string, platform csmv1.DriverType) *csmv1.ContainerStorageModule {
		return &csmv1.ContainerStorageModule{
			Spec: csmv1.ContainerStorageModuleSpec{
				Version: specVersion,
				Driver: csmv1.Driver{
					CSIDriverType: platform,
					ConfigVersion: driverConfigVersion,
					Common: &csmv1.ContainerTemplate{
						ImagePullPolicy: corev1.PullAlways,
					},
				},
				Modules: []csmv1.Module{
					{
						Name:          csmv1.AuthorizationServer,
						ConfigVersion: moduleConfigVersion,
					},
				},
			},
		}
	}

	// Helper for creating a CR with an AuthorizationServer module
	newCSMWithoutDriver := func(specVersion string) *csmv1.ContainerStorageModule {
		return &csmv1.ContainerStorageModule{
			Spec: csmv1.ContainerStorageModuleSpec{
				Version: specVersion,
				Modules: []csmv1.Module{
					{
						Name: csmv1.AuthorizationServer,
					},
				},
			},
		}
	}

	cases := []struct {
		name        string
		cr          *csmv1.ContainerStorageModule
		op          OperatorConfig
		want        string
		expectedErr string
	}{
		{
			name: "version_present",
			cr:   newCSM("v1.16.0", "", csmv1.PowerScale),
			op: OperatorConfig{
				ConfigDirectory: "../../operatorconfig",
			},
			want:        "v2.16.0",
			expectedErr: "",
		},
		{
			name: "config_version_present",
			cr:   newCSM("", "v2.16.0", csmv1.PowerStore),
			op: OperatorConfig{
				ConfigDirectory: "../../operatorconfig",
			},
			want:        "v2.16.0",
			expectedErr: "",
		},
		{
			name: "invalid_path",
			cr:   newCSM("v1.16.0", "", csmv1.PowerStore),
			op: OperatorConfig{
				ConfigDirectory: "invalid/path",
			},
			want:        "",
			expectedErr: "failed to read file invalid/path/common/csm-version-mapping.yaml",
		},
		{
			name: "invalid_platform",
			cr:   newCSM("v1.16.0", "", "invalid"),
			op: OperatorConfig{
				ConfigDirectory: "../../operatorconfig",
			},
			want:        "",
			expectedErr: "Unsupported platform invalid",
		},
		{
			name: "invalid_version",
			cr:   newCSM("v1.10.0", "", csmv1.PowerStore),
			op: OperatorConfig{
				ConfigDirectory: "../../operatorconfig",
			},
			want:        "",
			expectedErr: "No custom resource configuration is available for CSM version v1.10.0. Supported CSM versions are:",
		},
		{
			name: "config_version_from_module_when_driver_type_empty",
			cr:   newCSMWithAuthModule("", "", "v2.4.0", ""),
			op: OperatorConfig{
				ConfigDirectory: "../../operatorconfig",
			},
			want:        "v2.4.0",
			expectedErr: "",
		},
		{
			name: "config_version_from_module_when_driver_type_empty",
			cr:   newCSMWithoutDriver("v1.16.0"),
			op: OperatorConfig{
				ConfigDirectory: "../../operatorconfig",
			},
			want:        "v2.4.0",
			expectedErr: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := GetVersion(ctx, tc.cr, tc.op)
			if err != nil && !strings.Contains(err.Error(), tc.expectedErr) {
				t.Errorf("GetVersion() returned error = %v but expected to contain = %q", err, tc.expectedErr)
			}
			if err == nil && tc.expectedErr != "" {
				t.Errorf("GetVersion() expected error containing %q but got nil", tc.expectedErr)
			}
			if got != tc.want {
				t.Errorf("GetVersion() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Tests: ValidateConfigMap
func TestValidateConfigMap(t *testing.T) {
	cases := []struct {
		name        string
		version     VersionSpec
		expectedErr string
	}{
		{
			name: "valid_images_all_present",
			version: VersionSpec{
				Version: "v1.16.0",
				Images: map[string]string{
					"driver":  "repo/driver:1",
					"sidecar": "repo/sidecar:1",
				},
			},
			expectedErr: "",
		},
		{
			name: "invalid_empty_image_value",
			version: VersionSpec{
				Version: "v1.16.0",
				Images: map[string]string{
					"driver":  "repo/driver:1",
					"sidecar": "",
				},
			},
			expectedErr: `value for key "sidecar" is empty`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfigMap(tc.version)
			if tc.expectedErr == "" && err != nil {
				t.Fatalf("ValidateConfigMap() unexpected error: %v", err)
			}
			if tc.expectedErr != "" {
				if err == nil || err.Error() != tc.expectedErr {
					t.Fatalf("ValidateConfigMap() error = %v, want = %v", err, tc.expectedErr)
				}
			}
		})
	}
}

func newCSM(specVersion string) *csmv1.ContainerStorageModule {
	return &csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Version: specVersion,
			Driver: csmv1.Driver{
				Common: &csmv1.ContainerTemplate{
					ImagePullPolicy: corev1.PullAlways,
				},
			},
		},
	}
}

// Helper: setup scheme for fake client
func buildScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := csmv1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add csmv1 to scheme: %v", err)
	}
	return s
}

// Helper: build fake client with provided objects
func buildFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := buildScheme(t)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

// Helper: make a ConfigMap named CSMImages in a given namespace
func makeImagesConfigMap(namespace string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: CSMImages, Namespace: namespace},
		Data:       data,
	}
}

// Compose a versions.yaml string given slices of VersionSpec
func marshalVersionsYAML(t *testing.T, versions []VersionSpec) string {
	t.Helper()
	b, err := yaml.Marshal(versions)
	if err != nil {
		t.Fatalf("failed to marshal versions: %v", err)
	}
	return string(b)
}

// Tests UpdateUsingConfigMap
func TestUpdateUsingConfigMap(t *testing.T) {
	validVersions := []VersionSpec{
		{
			Version: "v1.16.0",
			Images: map[string]string{
				"driver":  "repo/driver:1",
				"sidecar": "repo/sidecar:1",
			},
		},
		{
			Version: "v2.16.0",
			Images: map[string]string{
				"driver":  "repo/driver:2",
				"sidecar": "repo/sidecar:2",
			},
		},
	}

	invalidImages := []VersionSpec{
		{
			Version: "v1.16.0",
			Images: map[string]string{
				"driver":  "repo/driver:1",
				"sidecar": "",
			},
		},
	}

	cases := []struct {
		name        string
		cr          *csmv1.ContainerStorageModule
		cm          corev1.ConfigMap
		want        VersionSpec
		expectedErr string
	}{
		{
			name: "empty_cm_data_returns_zero_spec",
			cr:   newCSM("v1.16.0"),
			cm:   corev1.ConfigMap{Data: map[string]string{}},
			want: VersionSpec{},
		},
		{
			name: "malformed_yaml_returns_error",
			cr:   newCSM("v1.16.0"),
			cm: corev1.ConfigMap{
				Data: map[string]string{
					"versions.yaml": "!! not a valid yaml !!",
				},
			},
			want:        VersionSpec{},
			expectedErr: "yaml: unmarshal errors", // substring check
		},
		{
			name: "version_not_found_in_yaml",
			cr:   newCSM("v9.99.9"),
			cm: corev1.ConfigMap{
				Data: map[string]string{
					"versions.yaml": marshalVersionsYAML(t, validVersions),
				},
			},
			want:        VersionSpec{},
			expectedErr: "version v9.99.9 not found in versions.yaml",
		},
		{
			name: "valid_match_returns_version_spec",
			cr:   newCSM("v2.16.0"),
			cm: corev1.ConfigMap{
				Data: map[string]string{
					"versions.yaml": marshalVersionsYAML(t, validVersions),
				},
			},
			want: VersionSpec{
				Version: "v2.16.0",
				Images: map[string]string{
					"driver":  "repo/driver:2",
					"sidecar": "repo/sidecar:2",
				},
			},
		},
		{
			name: "valid_match_but_empty_image_fails_validation",
			cr:   newCSM("v1.16.0"),
			cm: corev1.ConfigMap{
				Data: map[string]string{
					"versions.yaml": marshalVersionsYAML(t, invalidImages),
				},
			},
			want: VersionSpec{
				Version: "v1.16.0",
				Images: map[string]string{
					"driver":  "repo/driver:1",
					"sidecar": "",
				},
			},
			expectedErr: `value for key "sidecar" is empty`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := UpdateUsingConfigMap(tc.cr, tc.cm)
			if tc.expectedErr == "" && err != nil {
				t.Fatalf("UpdateUsingConfigMap() unexpected error: %v", err)
			}
			if tc.expectedErr != "" {
				if err == nil || !contains(err.Error(), tc.expectedErr) {
					t.Fatalf("UpdateUsingConfigMap() error = %v, want contains = %v", err, tc.expectedErr)
				}
			}
			if !versionSpecEqual(got, tc.want) {
				t.Fatalf("UpdateUsingConfigMap() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// Tests ResolveVersionFromConfigMap
func TestResolveVersionFromConfigMap(t *testing.T) {
	valid := []VersionSpec{
		{
			Version: "v1.16.0",
			Images: map[string]string{
				"driver":  "repo/driver:1",
				"sidecar": "repo/sidecar:1",
			},
		},
	}

	invalid := []VersionSpec{
		{
			Version: "v1.16.0",
			Images: map[string]string{
				"driver":  "repo/driver:1",
				"sidecar": "",
			},
		},
	}

	cases := []struct {
		name        string
		clientObjs  []client.Object
		cr          *csmv1.ContainerStorageModule
		want        VersionSpec
		expectedErr string
	}{
		{
			name:       "no_configmap_present_returns_zero_spec_and_no_error",
			clientObjs: []client.Object{},
			cr:         newCSM("v1.16.0"),
			want:       VersionSpec{},
		},
		{
			name: "valid_flow_returns_matched_version",
			clientObjs: []client.Object{
				makeImagesConfigMap("csm-ns", map[string]string{
					"versions.yaml": marshalVersionsYAML(t, valid),
				}),
			},
			cr: newCSM("v1.16.0"),
			want: VersionSpec{
				Version: "v1.16.0",
				Images: map[string]string{
					"driver":  "repo/driver:1",
					"sidecar": "repo/sidecar:1",
				},
			},
		},
		{
			name: "invalid_images_value_fails_validation",
			clientObjs: []client.Object{
				makeImagesConfigMap("csm-ns", map[string]string{
					"versions.yaml": marshalVersionsYAML(t, invalid),
				}),
			},
			cr: newCSM("v1.16.0"),
			want: VersionSpec{
				Version: "v1.16.0",
				Images: map[string]string{
					"driver":  "repo/driver:1",
					"sidecar": "",
				},
			},
			expectedErr: `value for key "sidecar" is empty`,
		},
		{
			name: "version_not_found_in_versions_yaml",
			clientObjs: []client.Object{
				makeImagesConfigMap("csm-ns", map[string]string{
					"versions.yaml": marshalVersionsYAML(t, []VersionSpec{
						{Version: "v2.0.0", Images: map[string]string{"driver": "x", "sidecar": "y"}},
					}),
				}),
			},
			cr:          newCSM("v1.16.0"),
			want:        VersionSpec{},
			expectedErr: "version v1.16.0 not found in versions.yaml",
		},
	}

	ctx := context.Background()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cl := buildFakeClient(t, tc.clientObjs...)
			got, err := ResolveVersionFromConfigMap(ctx, cl, tc.cr)

			if tc.expectedErr == "" && err != nil {
				t.Fatalf("ResolveVersionFromConfigMap() unexpected error: %v", err)
			}
			if tc.expectedErr != "" {
				if err == nil || !contains(err.Error(), tc.expectedErr) {
					t.Fatalf("ResolveVersionFromConfigMap() error = %v, want contains = %v", err, tc.expectedErr)
				}
			}
			if !versionSpecEqual(got, tc.want) {
				t.Fatalf("ResolveVersionFromConfigMap() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// Helpers: comparisons / contains
func contains(s, substr string) bool {
	return substr == "" || (s != "" && (len(substr) == 0 || (len(s) >= len(substr) && (indexOf(s, substr) >= 0))))
}

func indexOf(s, substr string) int {
	return index(s, substr)
}

// Straightforward rune-safe substring search for predictability in tests
func index(haystack, needle string) int {
	return len(fmt.Appendf(nil, "%s", haystack[:])) - len(fmt.Appendf(nil, "%s", haystack[len(needle):]))
}

// Safer comparison for VersionSpec maps
func versionSpecEqual(a, b VersionSpec) bool {
	if a.Version != b.Version {
		return false
	}
	if len(a.Images) != len(b.Images) {
		return false
	}
	for k, v := range a.Images {
		if b.Images[k] != v {
			return false
		}
	}
	return true
}

func TestValidateCustomRegistry(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		want     error
	}{
		{"Valid Standard", "quay.io", nil},
		{"Valid With Port", "localhost:5000", nil},
		{"Valid With Path", "docker.io/dell", nil},
		{"Valid IP", "127.0.0.1:5000", nil},
		{"Invalid structure", "://example", nil},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateCustomRegistry(ctx, tt.registry); got != tt.want {
				t.Errorf("ValidateCustomRegistry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetImageField(t *testing.T) {
	tests := []struct {
		name       string
		yamlString string
		want       string
	}{
		{
			name:       "single_line_no_split",
			yamlString: "quay.io/org/image:tag",
			want:       "quay.io/org/image:tag",
		},
		{
			name: "multi_line_image_present",
			yamlString: `
apiVersion: v1
kind: Pod
metadata:
  name: test
spec:
  containers:
    - name: test
      image: quay.io/org/image:tag
`,
			want: "quay.io/org/image:tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetImageField(tt.yamlString)
			if got != tt.want {
				t.Errorf("GetImageField() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetFinalImage(t *testing.T) {
	ctx := context.Background()
	componentName := "my-component"

	tests := []struct {
		name       string
		cr         csmv1.ContainerStorageModule
		matched    VersionSpec
		component  csmv1.ContainerTemplate
		yamlString string
		want       string
		useActual  bool
	}{
		{
			name: "matched version with component image present",
			matched: VersionSpec{
				Version: "v2.0.0",
				Images:  map[string]string{componentName: "repo/mapped-image:2.0.0"},
			},
			component: csmv1.ContainerTemplate{
				Name:  componentName,
				Image: "repo/default-image:latest",
			},
			cr:   csmv1.ContainerStorageModule{},
			want: "repo/mapped-image:2.0.0",
		},
		{
			name: "custom registry present uses ResolveImage(GetImageField(yaml), cr)",
			matched: VersionSpec{
				Version: "",
				Images:  map[string]string{},
			},
			component: csmv1.ContainerTemplate{
				Name:  componentName,
				Image: "",
			},
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					CustomRegistry: "my.registry.local",
				},
			},
			yamlString: "fake-yaml",
			useActual:  true,
		},
		{
			name: "component image present when no matched version and no custom registry",
			matched: VersionSpec{
				Version: "",
				Images:  map[string]string{},
			},
			component: csmv1.ContainerTemplate{
				Name:  componentName,
				Image: "repo/component-image:1.0",
			},
			cr:   csmv1.ContainerStorageModule{},
			want: "repo/component-image:1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.useActual {
				field := GetImageField(tt.yamlString)
				tt.want = ResolveImage(ctx, field, tt.cr)
			}

			got := GetFinalImage(ctx, tt.cr, tt.matched, tt.component, tt.yamlString)
			if got != tt.want {
				t.Errorf("GetFinalImage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveImage(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		originalImageFile string
		cr                csmv1.ContainerStorageModule
		want              string
	}{
		{
			name:              "custom registry empty => return original",
			originalImageFile: "repo/org/image:tag",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Version:        "v1.0.0",
					CustomRegistry: "",
				},
			},
			want: "repo/org/image:tag",
		},
		{
			name:              "retain=true, strip domain if present (dot domain)",
			originalImageFile: "quay.io/dell/container-storage-modules/plugin:1.2.3",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Version:                 "v1.2.3",
					CustomRegistry:          "my.registry.local",
					RetainImageRegistryPath: true,
				},
			},
			want: "my.registry.local/dell/container-storage-modules/plugin:1.2.3",
		},
		{
			name:              "retain=true, strip domain if present (localhost)",
			originalImageFile: "localhost/dell/csm/node:2.0.0",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Version:                 "v2.0.0",
					CustomRegistry:          "my.registry.local",
					RetainImageRegistryPath: true,
				},
			},
			want: "my.registry.local/dell/csm/node:2.0.0",
		},
		{
			name:              "retain=true, first segment not a domain => keep entire path",
			originalImageFile: "dell/container-storage-modules/controller:latest",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Version:                 "v3.0.0",
					CustomRegistry:          "internal.registry",
					RetainImageRegistryPath: true,
				},
			},
			want: "internal.registry/dell/container-storage-modules/controller:latest",
		},
		{
			name:              "retain=false, keep only final image name",
			originalImageFile: "quay.io/dell/csm/node:1.0.0",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Version:                 "v1.0.0",
					CustomRegistry:          "mirror.registry",
					RetainImageRegistryPath: false,
				},
			},
			want: "mirror.registry/node:1.0.0",
		},
		{
			name:              "retain=false, single segment (no slash) remains same segment",
			originalImageFile: "image:tag",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Version:                 "v1.0.0",
					CustomRegistry:          "mirror.registry",
					RetainImageRegistryPath: false,
				},
			},
			want: "mirror.registry/image:tag",
		},
		{
			name:              "inputs with whitespace are trimmed",
			originalImageFile: "   quay.io/org/image:tag   ",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Version:                 "   v1.0.0   ",
					CustomRegistry:          "   my.registry.local   ",
					RetainImageRegistryPath: false,
				},
			},
			want: "my.registry.local/image:tag",
		},
		{
			name:              "retain=true with path containing multiple segments and tag",
			originalImageFile: "registry.example.com/team/subteam/my-image:9.9",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Version:                 "v9.9.0",
					CustomRegistry:          "cr.mirror",
					RetainImageRegistryPath: true,
				},
			},
			want: "cr.mirror/team/subteam/my-image:9.9",
		},
		{
			name:              "retain=false with deeply nested path",
			originalImageFile: "registry.example.com/team/subteam/my-image:9.9",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Version:                 "v9.9.0",
					CustomRegistry:          "cr.mirror",
					RetainImageRegistryPath: false,
				},
			},
			want: "cr.mirror/my-image:9.9",
		},
		{
			name:              "retain=true with no slash (only image:tag) keeps same image in new registry",
			originalImageFile: "myimage:1.2.3",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Version:                 "v1.2.3",
					CustomRegistry:          "another.registry",
					RetainImageRegistryPath: true,
				},
			},
			want: "another.registry/myimage:1.2.3",
		},
		{
			name:              "retain=true, path with first segment containing dash but not domain",
			originalImageFile: "my-org/myrepo/image:tag",
			cr: csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Version:                 "v1.0.1",
					CustomRegistry:          "reg.example",
					RetainImageRegistryPath: true,
				},
			},
			want: "reg.example/my-org/myrepo/image:tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveImage(ctx, tt.originalImageFile, tt.cr)
			if got != tt.want {
				t.Errorf("ResolveImage() = %q, want %q", got, tt.want)
			}
		})
	}
}
