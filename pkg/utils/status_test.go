//  Copyright © 2021 - 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"reflect"
	"testing"
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetDeploymentStatus(t *testing.T) {

	ns := "default"
	licenceCred := getSecret(ns, "dls-license")
	ivLicense := getSecret(ns, "iv")

	sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(licenceCred).WithObjects(ivLicense).Build()

	fakeReconcile := FakeReconcileCSM{
		Client:    sourceClient,
		K8sClient: fake.NewSimpleClientset(),
	}
	type args struct {
		ctx      context.Context
		instance *csmv1.ContainerStorageModule
		r        ReconcileCSM
	}
	tests := []struct {
		name    string
		args    args
		want    csmv1.PodStatus
		wantErr bool
	}{
		{
			name: "Test get deployment status when instance name is empty",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("", "", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r:        &fakeReconcile,
			},
			want: csmv1.PodStatus{
				Available: "0",
				Desired:   "0",
				Failed:    "0",
			},
			wantErr: false,
		},
		{
			name: "Test get deployment status when instance name is authorization",
			args: args{
				ctx:      context.Background(),
				instance: createCSM(string(csmv1.Authorization), "", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r:        &fakeReconcile,
			},
			want: csmv1.PodStatus{
				Available: "0",
				Desired:   "0",
				Failed:    "0",
			},
			wantErr: false,
		},
		{
			name: "Test get deployment status when instance name is application-mobility",
			args: args{
				ctx:      context.Background(),
				instance: createCSM(string(csmv1.ApplicationMobility), "", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r:        &fakeReconcile,
			},
			want: csmv1.PodStatus{
				Available: "0",
				Desired:   "0",
				Failed:    "0",
			},
			wantErr: false,
		},
		{
			name: "Test get deployment status when instance name is controller is not found",
			args: args{
				ctx:      context.Background(),
				instance: createCSM(string(csmv1.PowerFlex), "", csmv1.PowerFlex, csmv1.Replication, false, nil),
				r:        &fakeReconcile,
			},
			want: csmv1.PodStatus{
				Available: "0",
				Desired:   "0",
				Failed:    "0",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDeploymentStatus(tt.args.ctx, tt.args.instance, tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDeploymentStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDeploymentStatus() = %v, want %v", got, tt.want)
			}
		})
	}

}

func TestWaitForNginxController(t *testing.T) {
	zero := int32(0)
	one := int32(1)

	name := "authorization-ingress-nginx-controller"
	ns := "authorization"

	tests := map[string]func() (*FakeReconcileCSM, csmv1.ContainerStorageModule, time.Duration, bool){
		"Test wait for nginx controller success": func() (*FakeReconcileCSM, csmv1.ContainerStorageModule, time.Duration, bool) {
			nginx := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
					Labels:    map[string]string{"app.kubernetes.io/name": "ingress-nginx"},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &one,
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: one,
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(nginx).Build()
			fakeReconcile := &FakeReconcileCSM{
				Client: sourceClient,
			}
			authorization := createCSM("authorization", "authorization", "", csmv1.AuthorizationServer, true, nil)
			wantErr := false

			return fakeReconcile, *authorization, 1 * time.Second, wantErr
		},
		"Test wait for nginx controller replicas not ready to ready": func() (*FakeReconcileCSM, csmv1.ContainerStorageModule, time.Duration, bool) {
			nginx := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
					Labels:    map[string]string{"app.kubernetes.io/name": "ingress-nginx"},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &one,
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: zero,
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(nginx).Build()
			fakeReconcile := &FakeReconcileCSM{
				Client: sourceClient,
			}
			authorization := createCSM("authorization", "authorization", "", csmv1.AuthorizationServer, true, nil)
			wantErr := false

			go func() {
				time.Sleep(1 * time.Second)
				nginx.Status.ReadyReplicas = one
				err := sourceClient.Status().Update(context.Background(), nginx)
				if err != nil {
					t.Errorf("failed to update nginx deployment: %v", err)
				}
			}()

			return fakeReconcile, *authorization, 3 * time.Second, wantErr
		},
		"Test wait for nginx controller times out": func() (*FakeReconcileCSM, csmv1.ContainerStorageModule, time.Duration, bool) {
			nginx := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
					Labels:    map[string]string{"app.kubernetes.io/name": "ingress-nginx"},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &one,
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: zero,
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(nginx).Build()
			fakeReconcile := &FakeReconcileCSM{
				Client: sourceClient,
			}
			authorization := createCSM("authorization", "authorization", "", csmv1.AuthorizationServer, true, nil)
			wantErr := true

			return fakeReconcile, *authorization, 1 * time.Second, wantErr
		},
		"Test nginx controller not found": func() (*FakeReconcileCSM, csmv1.ContainerStorageModule, time.Duration, bool) {
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			fakeReconcile := &FakeReconcileCSM{
				Client: sourceClient,
			}
			authorization := createCSM("authorization", "authorization", "", csmv1.AuthorizationServer, true, nil)
			wantErr := true

			return fakeReconcile, *authorization, 1 * time.Second, wantErr
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			fakeReconcile, authorization, duration, wantErr := test()
			err := WaitForNginxController(context.Background(), authorization, fakeReconcile, duration)
			if (err != nil) != wantErr {
				t.Errorf("WaitForNginxController() error = %v, wantErr %v", err, wantErr)
				return
			}
		})
	}

}

func getSecret(namespace, secretName string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"data": []byte(secretName),
		},
	}
}

func createCSM(name string, namespace string, driverType csmv1.DriverType, moduleType csmv1.ModuleType, moduleEnabled bool, components []csmv1.ContainerTemplate) *csmv1.ContainerStorageModule {
	return &csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
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
