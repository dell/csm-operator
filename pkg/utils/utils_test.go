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
	"context"
	"testing"

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
	scheme := runtime.NewScheme()

	// Register the necessary types with the scheme
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(velerov1.AddToScheme(scheme))

	// Set the scheme as the default scheme
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(velerov1.AddToScheme(scheme))
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()
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
    `)

	ctrlObjects, err := GetModuleComponentObj(yamlString)
	if err != nil {
		t.Fatalf("Failed to get module component objects: %v", err)
	}

	if len(ctrlObjects) != 4 {
		t.Errorf("Expected 4 objects, got %d", len(ctrlObjects))
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
