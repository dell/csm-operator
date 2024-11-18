//  Copyright © 2024 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	batchv1 "k8s.io/api/batch/v1"
	networking "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/stretchr/testify/assert"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	confv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	confcorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	confmetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
		io.Copy(&buf, reader)
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
	csmv1.AddToScheme(scheme)  // for CSM objects
	corev1.AddToScheme(scheme) // for namespaces
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(velerov1.AddToScheme(scheme))

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
	expectedResult = [][]byte{}
	result, err = SplitYaml([]byte(yamlString))
	assert.Nil(t, err)
	assert.Nil(t, result)

	// Test case: YAML with null byte
	yamlString = "\x00"
	result, err = SplitYaml([]byte(yamlString))
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

func TestGetBackupStorageLocation(t *testing.T) {
	ctx := context.Background()
	fakeClient := fullFakeClient()

	// Test case: BackupStorageLocation does not exist
	name := "test-backup-storage"
	namespace := "test-namespace"
	_, err := GetBackupStorageLocation(ctx, name, namespace, fakeClient)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	// Test case: BackupStorageLocation exists
	backupStorage := &velerov1.BackupStorageLocation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if err := fakeClient.Create(ctx, backupStorage); err != nil {
		t.Errorf("Failed to create BackupStorageLocation: %v", err)
	}
	backupStorage, err = GetBackupStorageLocation(ctx, name, namespace, fakeClient)
	if err != nil {
		t.Errorf("Failed to get BackupStorageLocation: %v", err)
	}
	if backupStorage.Name != name {
		t.Errorf("Expected name %s, got %s", name, backupStorage.Name)
	}
	if backupStorage.Namespace != namespace {
		t.Errorf("Expected namespace %s, got %s", namespace, backupStorage.Namespace)
	}
}

func TestUpdateSideCarApply(t *testing.T) {
	// Test case: update sidecar with matching name
	sc1env1 := "sidecar1-env1"
	sc1env2 := "sidecar1-env2"
	sc1env3 := "sidecar1-env3"
	oldenv1val := "old-env1-value"
	oldenv3val := "old-env3-value"
	newenv1val := "sidecar1-env1-value"
	newenv2val := "sidecar1-env2-value"
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
					Name:  sc1env2,
					Value: newenv2val,
				},
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
		Name:  &sc1env3,
		Value: &oldenv3val,
	},
	)

	UpdateSideCarApply(sideCars, container)

	expectedContainer := acorev1.Container().WithName("sidecar1").WithImage("sidecar1-image").WithImagePullPolicy("sidecar1-image-pull-policy").
		WithEnv(&acorev1.EnvVarApplyConfiguration{
			Name:  &sc1env1,
			Value: &newenv1val,
		}).
		/*WithEnv(&acorev1.EnvVarApplyConfiguration{ // IF we want to have new vars added in the Apply method, this will need to be uncommented.
			Name:  &sc1env2,
			Value: &newenv2val,
		}).*/
		WithEnv(&acorev1.EnvVarApplyConfiguration{
			Name:  &sc1env3,
			Value: &oldenv3val,
		},
		)

	assert.Equal(t, expectedContainer, container)

	// repeat the test with the other function that uses the child function
	// very minor code coverage gain, 0.1%
	UpdateInitContainerApply(sideCars, container)
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
			Sdc:                   "new-sdc-image",
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
				Common: csmv1.ContainerTemplate{
					ImagePullPolicy: corev1.PullPolicy("Always"),
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
kind: BackupStorageLocation
metadata:
  name: my-bsl
spec:
  provider: aws
  objectStorage:
    bucket: my-bucket
    region: us-east-1
---
apiVersion: v1
kind: VolumeSnapshotLocation
metadata:
  name: my-vsl
spec:
  provider: aws
  objectStorage:
    bucket: my-bucket
    region: us-east-1
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

	if len(ctrlObjects) != 25 {
		t.Errorf("Expected 25 objects, got %d", len(ctrlObjects))
	}

	for _, obj := range ctrlObjects {
		switch obj.(type) {
		case *corev1.Service:
			sv, ok := obj.(*corev1.Service)
			if !ok {
				t.Errorf("Expected Service object, got %T", obj)
			}
			if sv.Name != "my-service" {
				t.Errorf("Expected service name 'my-service', got %s", sv.Name)
			}
		case *rbacv1.ClusterRole:
			cr, ok := obj.(*rbacv1.ClusterRole)
			if !ok {
				t.Errorf("Expected ClusterRole object, got %T", obj)
			}
			if cr.Name != "my-cluster-role" {
				t.Errorf("Expected cluster role name 'my-cluster-role', got %s", cr.Name)
			}
		case *corev1.ConfigMap:
			cm, ok := obj.(*corev1.ConfigMap)
			if !ok {
				t.Errorf("Expected ConfigMap object, got %T", obj)
			}
			if cm.Name != "my-config" {
				t.Errorf("Expected config map name 'my-config', got %s", cm.Name)
			}
		case *appsv1.Deployment:
			dp, ok := obj.(*appsv1.Deployment)
			if !ok {
				t.Errorf("Expected Deployment object, got %T", obj)
			}
			if dp.Name != "my-deployment" {
				t.Errorf("Expected deployment name 'my-deployment', got %s", dp.Name)
			}
		case *apiextv1.CustomResourceDefinition:
			crd, ok := obj.(*apiextv1.CustomResourceDefinition)
			if !ok {
				t.Errorf("Expected CustomResourceDefinition object, got %T", obj)
			}
			if crd.Name != "my-crd" {
				t.Errorf("Expected custom resource definition name 'my-crd', got %s", crd.Name)
			}
		case *corev1.ServiceAccount:
			sa, ok := obj.(*corev1.ServiceAccount)
			if !ok {
				t.Errorf("Expected ServiceAccount object, got %T", obj)
			}
			if sa.Name != "my-sa" {
				t.Errorf("Expected service account name 'my-service-account', got %s", sa.Name)
			}
		case *rbacv1.ClusterRoleBinding:
			crb, ok := obj.(*rbacv1.ClusterRoleBinding)
			if !ok {
				t.Errorf("Expected ClusterRoleBinding object, got %T", obj)
			}
			if crb.Name != "my-crb" {
				t.Errorf("Expected cluster role binding name 'my-crb', got %s", crb.Name)
			}
		case *rbacv1.Role:
			role, ok := obj.(*rbacv1.Role)
			if !ok {
				t.Errorf("Expected Role object, got %T", obj)
			}
			if role.Name != "my-role" {
				t.Errorf("Expected role name 'my-role', got %s", role.Name)
			}
		case *rbacv1.RoleBinding:
			rb, ok := obj.(*rbacv1.RoleBinding)
			if !ok {
				t.Errorf("Expected RoleBinding object, got %T", obj)
			}
			if rb.Name != "my-role-binding" {
				t.Errorf("Expected role binding name 'my-role-binding', got %s", rb.Name)
			}
		case *corev1.PersistentVolumeClaim:
			pvc, ok := obj.(*corev1.PersistentVolumeClaim)
			if !ok {
				t.Errorf("Expected PersistentVolumeClaim object, got %T", obj)
			}
			if pvc.Name != "my-pvc" {
				t.Errorf("Expected persistent volume claim name 'my-pvc', got %s", pvc.Name)
			}
		case *batchv1.Job:
			job, ok := obj.(*batchv1.Job)
			if !ok {
				t.Errorf("Expected Job object, got %T", obj)
			}
			if job.Name != "my-job" {
				t.Errorf("Expected job name 'my-job', got %s", job.Name)
			}
		case *networking.Ingress:
			ing, ok := obj.(*networking.Ingress)
			if !ok {
				t.Errorf("Expected Ingress object, got %T", obj)
			}
			if ing.Name != "my-ingress" {
				t.Errorf("Expected ingress name 'my-ingress', got %s", ing.Name)
			}
		case *admissionregistration.ValidatingWebhookConfiguration:
			vwc, ok := obj.(*admissionregistration.ValidatingWebhookConfiguration)
			if !ok {
				t.Errorf("Expected ValidatingWebhookConfiguration object, got %T", obj)
			}
			if vwc.Name != "my-vwc" {
				t.Errorf("Expected validating webhook configuration name 'my-vwc', got %s", vwc.Name)
			}
		case *admissionregistration.MutatingWebhookConfiguration:
			mwc, ok := obj.(*admissionregistration.MutatingWebhookConfiguration)
			if !ok {
				t.Errorf("Expected MutatingWebhookConfiguration object, got %T", obj)
			}
			if mwc.Name != "my-mwc" {
				t.Errorf("Expected mutating webhook configuration name 'my-mwc', got %s", mwc.Name)
			}
		case *corev1.Secret:
			secret, ok := obj.(*corev1.Secret)
			if !ok {
				t.Errorf("Expected Secret object, got %T", obj)
			}
			if secret.Name != "my-secret" {
				t.Errorf("Expected secret name 'my-secret', got %s", secret.Name)
			}
		case *appsv1.DaemonSet:
			ds, ok := obj.(*appsv1.DaemonSet)
			if !ok {
				t.Errorf("Expected DaemonSet object, got %T", obj)
			}
			if ds.Name != "my-daemonset" {
				t.Errorf("Expected daemon set name 'my-daemonset', got %s", ds.Name)
			}
		case *velerov1.BackupStorageLocation:
			bsl, ok := obj.(*velerov1.BackupStorageLocation)
			if !ok {
				t.Errorf("Expected BackupStorageLocation object, got %T", obj)
			}
			if bsl.Name != "my-bsl" {
				t.Errorf("Expected backup storage location name 'my-bsl', got %s", bsl.Name)
			}
		case *velerov1.VolumeSnapshotLocation:
			vsl, ok := obj.(*velerov1.VolumeSnapshotLocation)
			if !ok {
				t.Errorf("Expected VolumeSnapshotLocation object, got %T", obj)
			}
			if vsl.Name != "my-vsl" {
				t.Errorf("Expected volume snapshot location name 'my-vsl', got %s", vsl.Name)
			}
		case *certmanagerv1.Issuer:
			issuer, ok := obj.(*certmanagerv1.Issuer)
			if !ok {
				t.Errorf("Expected Issuer object, got %T", obj)
			}
			if issuer.Name != "my-issuer" {
				t.Errorf("Expected issuer name 'my-issuer', got %s", issuer.Name)
			}
		case *certmanagerv1.Certificate:
			cert, ok := obj.(*certmanagerv1.Certificate)
			if !ok {
				t.Errorf("Expected Certificate object, got %T", obj)
			}
			if cert.Name != "my-certificate" {
				t.Errorf("Expected certificate name 'my-cert', got %s", cert.Name)
			}
		case *appsv1.StatefulSet:
			sts, ok := obj.(*appsv1.StatefulSet)
			if !ok {
				t.Errorf("Expected StatefulSet object, got %T", obj)
			}
			if sts.Name != "my-sts" {
				t.Errorf("Expected stateful set name 'my-sts', got %s", sts.Name)
			}
		case *storagev1.StorageClass:
			sc, ok := obj.(*storagev1.StorageClass)
			if !ok {
				t.Errorf("Expected StorageClass object, got %T", obj)
			}
			if sc.Name != "my-sc" {
				t.Errorf("Expected storage class name 'my-sc', got %s", sc.Name)
			}
		case *corev1.PersistentVolume:
			pv, ok := obj.(*corev1.PersistentVolume)
			if !ok {
				t.Errorf("Expected PersistentVolume object, got %T", obj)
			}
			if pv.Name != "my-pv" {
				t.Errorf("Expected persistent volume name 'my-pv', got %s", pv.Name)
			}
		case *corev1.Namespace:
			ns, ok := obj.(*corev1.Namespace)
			if !ok {
				t.Errorf("Expected Namespace object, got %T", obj)
			}
			if ns.Name != "my-ns" {
				t.Errorf("Expected namespace name 'my-ns', got %s", ns.Name)
			}
		case *networking.IngressClass:
			ic, ok := obj.(*networking.IngressClass)
			if !ok {
				t.Errorf("Expected IngressClass object, got %T", obj)
			}
			if ic.Name != "my-ic" {
				t.Errorf("Expected ingress class name 'my-ic', got %s", ic.Name)
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

	ctrlObjects, err = GetModuleComponentObj(invalidYamlString)
	assert.NotNil(t, err)
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
	// TODO: This object will need to be built precisely to match the input spec.
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
			Status: &confv1.DaemonSetStatusApplyConfiguration{
				// Status configuration
			},
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
				// ClusterRole configuration
			},
			ClusterRoleBinding: rbacv1.ClusterRoleBinding{
				// ClusterRoleBinding configuration
			},
		},
	}

	assert.Nil(t, err)
	// TODO: Proper comparison here once the
	// expected object has been ironed out
	assert.NotNil(t, ctrlObject)
	assert.NotNil(t, expected)
	// assert.Equal(t, ctrlObject, expected)

	// Test case: valid YAML - deployment
	// TODO: Reuse and edit the above input/expected outputs with modificatons for Deployment obj

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

	ctrlClient.Create(ctx, obj)
	err := DeleteObject(ctx, obj, ctrlClient)
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
	ctrlClient.Create(ctx, obj)
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
			driverConfig:     "v2.12.0",
			driverType:       csmv1.PowerScale,
			moduleType:       csmv1.Observability,
			path:             "../../operatorconfig",
			expectedVersion:  "v1.10.0",
			expectedErrorMsg: "",
		},
		/*
			{
				name:             "invalid driver",
				driverConfig:     "v2.12.0",
				driverType:       csmv1.UnknownDriver,
				moduleType:       csmv1.Observability,
				path:             "../operatorconfig",
				expectedVersion:  "",
				expectedErrorMsg: "unknown driver type: UnknownDriver",
			},*/
		{
			name:             "invalid version",
			driverConfig:     "v20.12.0",
			driverType:       csmv1.PowerScale,
			moduleType:       csmv1.Observability,
			path:             "../../operatorconfig",
			expectedVersion:  "",
			expectedErrorMsg: "does not exist in file ../../operatorconfig/moduleconfig/common/version-values.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := GetModuleDefaultVersion(tt.driverConfig, tt.driverType, tt.moduleType, tt.path)
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
			driverConfig:  "v2.12.0",
			expectedMajor: 2,
			expectedMinor: 12,
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
			driverConfig:  "2.12.0",
			expectedMajor: 2,
			expectedMinor: 12,
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

func TestGetClusterIDs(t *testing.T) {
	tests := []struct {
		name           string
		replica        csmv1.Module
		expectedOutput []string
	}{
		{
			name: "valid version",
			replica: csmv1.Module{
				Components: []csmv1.ContainerTemplate{
					{
						Name: ReplicationControllerManager,
						Envs: []corev1.EnvVar{
							{
								Name:  "TARGET_CLUSTERS_IDS",
								Value: "cluster1,cluster2",
							},
						},
					},
				},
			},
			expectedOutput: []string{"cluster1", "cluster2"},
		},
		{
			name: "invalid version",
			replica: csmv1.Module{
				Components: []csmv1.ContainerTemplate{
					{
						Name: ReplicationControllerManager,
						Envs: []corev1.EnvVar{
							{
								Name:  "TARGET_CLUSTERS_IDS",
								Value: "cluster1,cluster2,cluster3",
							},
						},
					},
				},
			},
			expectedOutput: []string{"cluster1", "cluster2", "cluster3"},
		},
		{
			name: "valid version - nil replica components",
			replica: csmv1.Module{
				Components: nil,
			},
			expectedOutput: []string{"self"},
		},
		{
			name: "valid version - empty cluster list",
			replica: csmv1.Module{
				Components: []csmv1.ContainerTemplate{
					{
						Name: ReplicationControllerManager,
						Envs: []corev1.EnvVar{
							{
								Name:  "TARGET_CLUSTERS_IDS",
								Value: "",
							},
						},
					},
				},
			},
			expectedOutput: []string{"self"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			output := getClusterIDs(ctx, tt.replica)

			assert.Equal(t, output, tt.expectedOutput)
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

// TODO: This test case isn't working yet. Needs some work,
// NewControllerRuntimeClient is rejecting the input.
/*
func TestGetClusterCtrlClient(t *testing.T) {
	// Create a fake context.Context
	ctx := context.Background()

	// Create a fake ctrlClient
	ctrlClient := fullFakeClient()

	// Create a fake clusterID
	clusterID := "test-cluster"

	// Create a fake secret
	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterID,
			Namespace: ReplicationControllerNameSpace,
		},
		Data: map[string][]byte{
			"data": []byte("test-data"),
		},
	}

	// Add the secret to the ctrlClient
	if err := ctrlClient.Create(ctx, &secret); err != nil {
		t.Fatalf("failed to create secret: %v", err)
	}

	// Call the function
	// TODO: This is erroring out, need to examine why
	clusterCtrlClient, err := getClusterCtrlClient(ctx, clusterID, ctrlClient)

	// Assert the expected result
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if clusterCtrlClient == nil {
		t.Error("expected clusterCtrlClient to be non-nil")
	}
}
*/

func TestGetNamespaces(t *testing.T) {
	// Create a fake context.Context
	ctx := context.Background()

	// CSM types must be registered with the scheme
	scheme := runtime.NewScheme()
	csmv1.AddToScheme(scheme)  // for CSM objects
	corev1.AddToScheme(scheme) // for namespaces

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
	namespaces, err := GetNamespaces(ctx, ctrlClient)

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

func TestIsAppMobilityComponentEnabled(t *testing.T) {
	// Test case: component is enabled
	instance := csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Modules: []csmv1.Module{
				{
					Name:    csmv1.ApplicationMobility,
					Enabled: true,
					Components: []csmv1.ContainerTemplate{
						{
							Name:    "application-mobility-controller-manager",
							Enabled: &[]bool{true}[0],
						},
					},
				},
			},
		},
	}
	expected := true
	result := IsAppMobilityComponentEnabled(context.Background(), instance, nil, csmv1.ApplicationMobility, "application-mobility-controller-manager")
	if result != expected {
		t.Errorf("Expected %v, but got %v", expected, result)
	}

	// Test case: component is disabled
	instance.Spec.Modules[0].Components[0].Enabled = &[]bool{false}[0]
	expected = false
	result = IsAppMobilityComponentEnabled(context.Background(), instance, nil, csmv1.ApplicationMobility, "application-mobility-controller-manager")
	if result != expected {
		t.Errorf("Expected %v, but got %v", expected, result)
	}

	// Test case: module is disabled
	instance.Spec.Modules[0].Enabled = false
	result = IsAppMobilityComponentEnabled(context.Background(), instance, nil, csmv1.ApplicationMobility, "application-mobility-controller-manager")
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

// TODO: Broken/unfinished. No reconciler object support.
/*
func TestGetDefaultClusters(t *testing.T) {
	// Create a fake context.Context
	ctx := context.Background()

	// CSM types must be registered with the scheme
	scheme := runtime.NewScheme()
	csmv1.AddToScheme(scheme)  // for CSM objects
	corev1.AddToScheme(scheme) // for namespaces

	// Create a fake ctrlClient
	ctrlClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()
	// Test case: replication module is enabled
	instance := csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Modules: []csmv1.Module{
				{
					Name:    csmv1.Replication,
					Enabled: true,
				},
			},
		},
	}

	fakeReconcile := FakeReconcileCSM{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
	}
	expectedReplicaEnabled := true
	expectedClusterClients := []ReplicaCluster{
		{
			ClusterCTRLClient: r.GetClient(),
			ClusterK8sClient:  r.GetK8sClient(),
			ClusterID:         DefaultSourceClusterID,
		},
	}

	replicaEnabled, clusterClients, err := GetDefaultClusters(ctx, instance, fakeReconcile)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}
	if replicaEnabled != expectedReplicaEnabled {
		t.Errorf("Expected %v, but got %v", expectedReplicaEnabled, replicaEnabled)
	}
	assert.Equal(t, clusterClients, expectedClusterClients)

	// Test case: replication module is disabled
	instance.Spec.Modules[0].Enabled = false
	expectedReplicaEnabled = false
	expectedClusterClients = []ReplicaCluster{}

	replicaEnabled, clusterClients, err = GetDefaultClusters(context.TODO(), instance, r)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}
	if replicaEnabled != expectedReplicaEnabled {
		t.Errorf("Expected %v, but got %v", expectedReplicaEnabled, replicaEnabled)
	}
	assert.Equal(t, clusterClients, expectedClusterClients)
}*/

func TestGetVolumeSnapshotLocation(t *testing.T) {
	// Test case: snapshot location exists
	ctx := context.Background()
	fakeClient := fullFakeClient()

	snapshotLocation := &velerov1.VolumeSnapshotLocation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-snapshot-location",
			Namespace: "test-namespace",
		},
	}
	if err := fakeClient.Create(ctx, snapshotLocation); err != nil {
		t.Errorf("Failed to create VolumeSnapshotLocation: %v", err)
	}

	snapshotLocation, err := GetVolumeSnapshotLocation(ctx, "test-snapshot-location", "test-namespace", fakeClient)
	if err != nil {
		t.Errorf("Failed to get VolumeSnapshotLocation: %v", err)
	}
	if snapshotLocation.Name != "test-snapshot-location" {
		t.Errorf("Expected name %s, got %s", "test-snapshot-location", snapshotLocation.Name)
	}
	if snapshotLocation.Namespace != "test-namespace" {
		t.Errorf("Expected namespace %s, got %s", "test-namespace", snapshotLocation.Namespace)
	}

	// Test case: snapshot location does not exist
	_, err = GetVolumeSnapshotLocation(ctx, "non-existent-snapshot-location", "test-namespace", fakeClient)
	if err == nil {
		t.Errorf("Expected error, but got nil")
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
	found, err = GetSecret(ctx, "test-secret", "test-namespace", ctrlClient)

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

	/*
		// Test case: error returned from getUpgradeInfo
		oldVersion = "1.0.0"
		newVersion = "2.0.0"
		operatorConfig.UpgradePaths = map[string]UpgradePaths{
			"driver": {
				MinUpgradePath: "2.0.0",
			},
		}
		expectedIsValid = false
		expectedErr := fmt.Errorf("getUpgradeInfo not successful")

		isValid, err = IsValidUpgrade(ctx, oldVersion, newVersion, csmComponentType, operatorConfig)
		if err == nil {
			t.Errorf("Expected error, but got nil")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected error %v, but got %v", expectedErr, err)
		}
		if isValid != expectedIsValid {
			t.Errorf("Expected %v, but got %v", expectedIsValid, isValid)
		}*/
}

// TODO: Work on this, it's failing with an unmarshal error. It's the last thing I was working on before EOD.
/*
func TestGetClusterCtrlClient(t *testing.T) {
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
	clusterCtrlClient, err := getClusterCtrlClient(ctx, clusterID, ctrlClient)

	// Assert the expected result
	assert.NoError(t, err)
	assert.NotNil(t, clusterCtrlClient)
}
*/

func TestGetClusterCtrlClient(t *testing.T) {
	// Create a fake context.Context
	ctx := context.Background()

	// CSM types must be registered with the scheme
	scheme := runtime.NewScheme()
	csmv1.AddToScheme(scheme)  // for CSM objects
	corev1.AddToScheme(scheme) // for namespaces

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
	csmv1.AddToScheme(scheme)  // for CSM objects
	corev1.AddToScheme(scheme) // for namespaces

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
