//  Copyright © 2024 - 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/assert"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestGetDeploymentStatus(t *testing.T) {
	ns := "default"
	licenseCred := getSecret(ns, "dls-license")
	ivLicense := getSecret(ns, "iv")

	err := csmv1.AddToScheme(scheme.Scheme)
	if err != nil {
		t.Fatal(err)
	}

	sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(licenseCred).WithObjects(ivLicense).Build()

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
		name      string
		args      args
		want      csmv1.PodStatus
		createObj client.Object
		wantErr   bool
	}{
		{
			name: "Test getDeploymentStatus when instance name is empty",
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
			createObj: nil,
			wantErr:   false,
		},
		{
			name: "Test getDeploymentStatus when instance is authorization proxy server",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("authorization", "", "", csmv1.AuthorizationServer, true, nil),
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
			name: "Test getDeploymentStatus when instance is authorization proxy server with non-default name",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("csm-authorization", "", "", csmv1.AuthorizationServer, true, nil),
				r:        &fakeReconcile,
			},
			want: csmv1.PodStatus{
				Available: "0",
				Desired:   "0",
				Failed:    "0",
			},
			createObj: nil,
			wantErr:   false,
		},
		{
			name: "Test getDeploymentStatus when instance name is application-mobility",
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
			createObj: nil,
			wantErr:   false,
		},
		{
			name: "Test getDeploymentStatus when instance name is controller is not found",
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
			createObj: nil,
			wantErr:   true,
		},
		{
			name: "Test getDeploymentStatus when instance is driver",
			args: args{
				ctx:      context.Background(),
				instance: createCSM(string(csmv1.PowerFlex), "", csmv1.PowerFlex, csmv1.Replication, false, nil),
				r:        &fakeReconcile,
			},
			want: csmv1.PodStatus{
				Available: "1",
				Desired:   "1",
				Failed:    "0",
			},
			createObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "powerflex-controller", Namespace: ""},
				Status: appsv1.DeploymentStatus{
					Replicas:            1,
					AvailableReplicas:   1,
					ReadyReplicas:       1,
					UnavailableReplicas: 0,
				},
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.args.instance.Name != "" && test.createObj != nil {
				err := test.args.r.GetClient().Create(context.Background(), test.createObj)
				assert.Nil(t, err)
			}

			got, err := getDeploymentStatus(test.args.ctx, test.args.instance, test.args.r)
			if (err != nil) != test.wantErr {
				t.Errorf("getDeploymentStatus() error = %v, wantErr %v", err, test.wantErr)
				return
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("getDeploymentStatus() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestGetDaemonSetStatus(t *testing.T) {
	type args struct {
		ctx      context.Context
		instance *csmv1.ContainerStorageModule
		r        ReconcileCSM
	}

	tests := []struct {
		name             string
		args             args
		wantTotalDesired int32
		wantStatus       csmv1.PodStatus
		wantErr          bool
	}{
		{
			name: "Test getDaemonSetStatus when GetCluster fails",
			args: args{
				ctx: context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, []csmv1.ContainerTemplate{
					{
						Name: "dell-replication-controller-manager",
						Envs: []corev1.EnvVar{{Name: "REPLICATION_CTRL_LOG_LEVEL", Value: "debug"}},
					},
				}),
				r: &FakeReconcileCSM{
					Client:    ctrlClientFake.NewClientBuilder().Build(),
					K8sClient: fake.NewSimpleClientset(),
				},
			},
			wantTotalDesired: 0,
			wantStatus: csmv1.PodStatus{
				Available: "0",
				Desired:   "0",
				Failed:    "0",
			},
			wantErr: true,
		},
		{
			name: "Test getDaemonSetStatus when namespace not found",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client:    ctrlClientFake.NewClientBuilder().Build(),
					K8sClient: fake.NewSimpleClientset(),
				},
			},
			wantTotalDesired: 0,
			wantStatus: csmv1.PodStatus{
				Available: "0",
				Desired:   "0",
				Failed:    "0",
			},
			wantErr: true,
		},
		{
			name: "Test getDaemonSetStatus when daemonset not found",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Namespace",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "powerflex",
						},
					}).Build(),
					K8sClient: fake.NewSimpleClientset(),
				},
			},
			wantTotalDesired: 0,
			wantStatus: csmv1.PodStatus{
				Available: "0",
				Desired:   "0",
				Failed:    "0",
			},
			wantErr: true,
		},
		{
			name: "Test getDaemonSetStatus with empty daemonset",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Namespace",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "powerflex",
						},
					}).WithObjects(&appsv1.DaemonSet{
						TypeMeta: metav1.TypeMeta{
							Kind:       "DaemonSet",
							APIVersion: "apps/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "powerflex-node",
							Namespace: "powerflex",
						},
					}).Build(),
					K8sClient: fake.NewSimpleClientset(),
				},
			},
			wantTotalDesired: 0,
			wantStatus: csmv1.PodStatus{
				Available: "0",
				Desired:   "0",
				Failed:    "0",
			},
			wantErr: false,
		},
		{
			name: "Test getDaemonSetStatus with one pending pod",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Namespace",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "powerflex",
						},
					}).WithObjects(&appsv1.DaemonSet{
						TypeMeta: metav1.TypeMeta{
							Kind:       "DaemonSet",
							APIVersion: "apps/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "powerflex-node",
							Namespace: "powerflex",
						},
						Status: appsv1.DaemonSetStatus{
							DesiredNumberScheduled: 1,
						},
					}).WithObjects(
						&corev1.Pod{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Pod",
								APIVersion: "v1",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      "powerflex-driver",
								Namespace: "powerflex",
								Labels: map[string]string{
									"app": "powerflex-node",
								},
							},
							Status: corev1.PodStatus{
								Phase: corev1.PodPending,
								Conditions: []corev1.PodCondition{
									{Type: corev1.PodReady, Status: corev1.ConditionTrue},
								},
							},
						}).Build(),
					K8sClient: fake.NewSimpleClientset(),
				},
			},
			wantTotalDesired: 1,
			wantStatus: csmv1.PodStatus{
				Available: "0",
				Desired:   "1",
				Failed:    "1",
			},
			wantErr: true,
		},
		{
			name: "Test getDaemonSetStatus with container state ImagePullBackoff",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Namespace",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "powerflex",
						},
					}).WithObjects(&appsv1.DaemonSet{
						TypeMeta: metav1.TypeMeta{
							Kind:       "DaemonSet",
							APIVersion: "apps/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "powerflex-node",
							Namespace: "powerflex",
						},
						Status: appsv1.DaemonSetStatus{
							DesiredNumberScheduled: 1,
						},
					}).WithObjects(
						&corev1.Pod{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Pod",
								APIVersion: "v1",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      "powerflex-driver",
								Namespace: "powerflex",
								Labels: map[string]string{
									"app": "powerflex-node",
								},
							},
							Status: corev1.PodStatus{
								Phase: corev1.PodPending,
								Conditions: []corev1.PodCondition{
									{Type: corev1.PodReady, Status: corev1.ConditionTrue},
								},
								ContainerStatuses: []corev1.ContainerStatus{
									{
										State: corev1.ContainerState{
											Waiting: &corev1.ContainerStateWaiting{
												Reason: "ImagePullBackOff",
											},
										},
									},
								},
							},
						}).Build(),
					K8sClient: fake.NewSimpleClientset(),
				},
			},
			wantTotalDesired: 1,
			wantStatus: csmv1.PodStatus{
				Available: "0",
				Desired:   "1",
				Failed:    "1",
			},
			wantErr: true,
		},
		{
			name: "Test getDaemonSetStatus with container state ContainerCreating",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Namespace",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "powerflex",
						},
					}).WithObjects(&appsv1.DaemonSet{
						TypeMeta: metav1.TypeMeta{
							Kind:       "DaemonSet",
							APIVersion: "apps/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "powerflex-node",
							Namespace: "powerflex",
						},
						Status: appsv1.DaemonSetStatus{
							DesiredNumberScheduled: 1,
						},
					}).WithObjects(
						&corev1.Pod{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Pod",
								APIVersion: "v1",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      "powerflex-driver",
								Namespace: "powerflex",
								Labels: map[string]string{
									"app": "powerflex-node",
								},
							},
							Status: corev1.PodStatus{
								Phase: corev1.PodPending,
								Conditions: []corev1.PodCondition{
									{Type: corev1.PodReady, Status: corev1.ConditionTrue},
								},
								ContainerStatuses: []corev1.ContainerStatus{
									{
										State: corev1.ContainerState{
											Waiting: &corev1.ContainerStateWaiting{
												Reason: "ContainerCreating",
											},
										},
									},
								},
							},
						}).Build(),
					K8sClient: fake.NewSimpleClientset(),
				},
			},
			wantTotalDesired: 1,
			wantStatus: csmv1.PodStatus{
				Available: "0",
				Desired:   "1",
				Failed:    "1",
			},
			wantErr: true,
		},
		{
			name: "Test getDaemonSetStatus with container state running",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Namespace",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "powerflex",
						},
					}).WithObjects(&appsv1.DaemonSet{
						TypeMeta: metav1.TypeMeta{
							Kind:       "DaemonSet",
							APIVersion: "apps/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "powerflex-node",
							Namespace: "powerflex",
						},
						Status: appsv1.DaemonSetStatus{
							DesiredNumberScheduled: 1,
						},
					}).WithObjects(
						&corev1.Pod{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Pod",
								APIVersion: "v1",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      "powerflex-driver",
								Namespace: "powerflex",
								Labels: map[string]string{
									"app": "powerflex-node",
								},
							},
							Status: corev1.PodStatus{
								Phase: corev1.PodRunning,
								Conditions: []corev1.PodCondition{
									{Type: corev1.PodReady, Status: corev1.ConditionTrue},
								},
								ContainerStatuses: []corev1.ContainerStatus{
									{
										State: corev1.ContainerState{
											Running: &corev1.ContainerStateRunning{
												StartedAt: metav1.Time{Time: time.Now()},
											},
										},
									},
								},
							},
						}).Build(),
					K8sClient: fake.NewSimpleClientset(),
				},
			},
			wantTotalDesired: 1,
			wantStatus: csmv1.PodStatus{
				Available: "1",
				Desired:   "1",
				Failed:    "0",
			},
			wantErr: false,
		},
		{
			name: "Test getDaemonSetStatus with pod running but container not running",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Namespace",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "powerflex",
						},
					}).WithObjects(&appsv1.DaemonSet{
						TypeMeta: metav1.TypeMeta{
							Kind:       "DaemonSet",
							APIVersion: "apps/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "powerflex-node",
							Namespace: "powerflex",
						},
						Status: appsv1.DaemonSetStatus{
							DesiredNumberScheduled: 1,
						},
					}).WithObjects(
						&corev1.Pod{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Pod",
								APIVersion: "v1",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      "powerflex-driver",
								Namespace: "powerflex",
								Labels: map[string]string{
									"app": "powerflex-node",
								},
							},
							Status: corev1.PodStatus{
								Phase: corev1.PodRunning,
								ContainerStatuses: []corev1.ContainerStatus{
									{
										State: corev1.ContainerState{
											Running: &corev1.ContainerStateRunning{
												StartedAt: metav1.Time{Time: time.Now()},
											},
										},
									},
								},
							},
						}).Build(),
					K8sClient: fake.NewSimpleClientset(),
				},
			},
			wantTotalDesired: 1,
			wantStatus: csmv1.PodStatus{
				Available: "0",
				Desired:   "1",
				Failed:    "0",
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			totalDesired, podStatus, err := getDaemonSetStatus(test.args.ctx, test.args.instance, test.args.r)
			if (err != nil) != test.wantErr {
				t.Errorf("getDaemonSetStatus() error = %v, wantErr %v", err, test.wantErr)
				return
			}

			if err == nil && totalDesired != test.wantTotalDesired {
				t.Errorf("getDaemonSetStatus() totalDesired = %v, wantTotalDesired %v", totalDesired, test.wantTotalDesired)
				return
			}

			if err == nil && !reflect.DeepEqual(podStatus, test.wantStatus) {
				t.Errorf("getDeploymentStatus() = %v, want %v", podStatus, test.wantStatus)
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

func TestAppMobStatusCheck(t *testing.T) {
	// Create a fake context.Context
	ctx := context.Background()
	ctrlClient := fullFakeClient()

	// Create a fake csm1 of csmv1.ContainerStorageModule
	csm1 := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
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
						{
							Name:    "cert-manager",
							Enabled: &[]bool{true}[0],
						},
						{
							Name:    "velero",
							Enabled: &[]bool{true}[0],
						},
					},
				},
			},
		},
	}

	// add the CSM object to the client
	err := ctrlClient.Create(ctx, &csm1)
	assert.NoError(t, err, "failed to create client object during test setup")

	i32One := int32(1)

	// add fake deployments and fake daemonsets to the client
	deployment1 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "application-mobility-controller-manager",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment2 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "application-mobility-velero",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment3 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager-webhook",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment4 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager-cainjector",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment5 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	err = ctrlClient.Create(ctx, &deployment1)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment2)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment3)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment4)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment5)
	assert.NoError(t, err, "failed to create client object during test setup")

	// create a fake running pod
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-agent",
			Namespace: "test-namespace",
			Labels:    map[string]string{"name": "node-agent"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "fake-container",
					Image: "fake-image",
				},
			},
		},
	}
	err = ctrlClient.Create(ctx, &pod)
	assert.NoError(t, err, "failed to create client object during test setup")

	// Create a fake instance of ReconcileCSM
	fakeReconcile := FakeReconcileCSM{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
	}

	// test 1: pods are running
	status, err := appMobStatusCheck(ctx, &csm1, &fakeReconcile, nil)
	assert.Nil(t, err)
	assert.Equal(t, true, status)

	// if !certEnabled && !veleroEnabled
	csm2 := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name-2",
			Namespace: "test-namespace",
		},
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
						{
							Name:    "cert-manager",
							Enabled: &[]bool{false}[0],
						},
						{
							Name:    "velero",
							Enabled: &[]bool{false}[0],
						},
					},
				},
			},
		},
	}
	err = ctrlClient.Create(ctx, &csm2)
	assert.NoError(t, err, "failed to create client object during test setup")
	status, err = appMobStatusCheck(ctx, &csm2, &fakeReconcile, nil)
	assert.Nil(t, err)
	assert.Equal(t, true, status)

	// Test 3: cert-manager is disabled, velero is enabled
	csm3 := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name-3",
			Namespace: "test-namespace",
		},
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
						{
							Name:    "cert-manager",
							Enabled: &[]bool{false}[0],
						},
						{
							Name:    "velero",
							Enabled: &[]bool{true}[0],
						},
					},
				},
			},
		},
	}
	err = ctrlClient.Create(ctx, &csm3)
	assert.NoError(t, err, "failed to create client object during test setup")
	status, err = appMobStatusCheck(ctx, &csm3, &fakeReconcile, nil)
	assert.Nil(t, err)
	assert.Equal(t, true, status)

	// Test 4: cert-manager is enabled, velero is disabled
	csm4 := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name-4",
			Namespace: "test-namespace",
		},
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
						{
							Name:    "cert-manager",
							Enabled: &[]bool{true}[0],
						},
						{
							Name:    "velero",
							Enabled: &[]bool{false}[0],
						},
					},
				},
			},
		},
	}
	err = ctrlClient.Create(ctx, &csm4)
	assert.NoError(t, err, "failed to create client object during test setup")
	status, err = appMobStatusCheck(ctx, &csm4, &fakeReconcile, nil)
	assert.Nil(t, err)
	assert.Equal(t, true, status)
}

func TestObservabilityStatusCheck(t *testing.T) {
	// Create a fake context.Context
	ctx := context.Background()
	ctrlClient := fullFakeClient()

	// Create a fake csm1 of csmv1.ContainerStorageModule
	csm1 := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: csmv1.Driver{
				CSIDriverType: "powerflex",
			},
			Modules: []csmv1.Module{
				{
					Name:    csmv1.Observability,
					Enabled: true,
					Components: []csmv1.ContainerTemplate{
						{
							Name:    "topology",
							Enabled: &[]bool{true}[0],
						},
						{
							Name:    "cert-manager",
							Enabled: &[]bool{true}[0],
						},
						{
							Name:    "otel-collector",
							Enabled: &[]bool{true}[0],
						},
						{
							Name:    "metrics-powerflex",
							Enabled: &[]bool{true}[0],
						},
					},
				},
			},
		},
	}

	// add the CSM object to the client
	err := ctrlClient.Create(ctx, &csm1)
	assert.NoError(t, err, "failed to create client object during test setup")
	i32One := int32(1)

	// add fake deployments to the client
	// first set of deployments: karavi
	deployment1 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment2 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "karavi-topology",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment3 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "karavi-metrics-powerflex",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	err = ctrlClient.Create(ctx, &deployment1)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment2)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment3)
	assert.NoError(t, err, "failed to create client object during test setup")

	// second set of deployments: cert manager
	// same namespace as CSM object
	deployment4 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment5 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager-cainjector",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment6 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager-webhook",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	err = ctrlClient.Create(ctx, &deployment4)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment5)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment6)
	assert.NoError(t, err, "failed to create client object during test setup")

	// Create a fake instance of ReconcileCSM
	fakeReconcile := FakeReconcileCSM{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
	}

	// test 1: pods are running
	status, err := observabilityStatusCheck(ctx, &csm1, &fakeReconcile, nil)
	assert.Nil(t, err)
	assert.Equal(t, true, status)
}

func TestObservabilityStatusCheckError(t *testing.T) {
	// Create a fake context.Context
	ctx := context.Background()
	ctrlClient := fullFakeClient()

	// Create a fake csm of csmv1.ContainerStorageModule
	csm := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name-powermax",
			Namespace: "test-namespace",
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: csmv1.Driver{
				CSIDriverType: "powermax",
			},
			Modules: []csmv1.Module{
				{
					Name:    csmv1.Observability,
					Enabled: true,
					Components: []csmv1.ContainerTemplate{
						{
							Name:    "topology",
							Enabled: &[]bool{true}[0],
						},
						{
							Name:    "cert-manager",
							Enabled: &[]bool{true}[0],
						},
						{
							Name:    "otel-collector",
							Enabled: &[]bool{true}[0],
						},
						{
							Name:    "metrics-powermax",
							Enabled: &[]bool{true}[0],
						},
					},
				},
			},
		},
	}

	// add the CSM object to the client
	err := ctrlClient.Create(ctx, &csm)
	assert.NoError(t, err, "failed to create client object during test setup")

	i32One := int32(1)

	otelDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	metricsPowerflexDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "karavi-metrics-powermax",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	topologyDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "karavi-topology",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	certManagerDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	certManagerCainjectorDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager-cainjector",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	certManagerWebhookDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager-webhook",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	err = ctrlClient.Create(ctx, &otelDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	// Create a fake instance of ReconcileCSM
	fakeReconcile := FakeReconcileCSM{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
	}

	_, err = observabilityStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &otelDeployment, 1)

	err = ctrlClient.Create(ctx, &metricsPowerflexDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = observabilityStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &metricsPowerflexDeployment, 1)

	err = ctrlClient.Create(ctx, &topologyDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = observabilityStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &topologyDeployment, 1)

	err = ctrlClient.Create(ctx, &certManagerDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = observabilityStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &certManagerDeployment, 1)

	err = ctrlClient.Create(ctx, &certManagerCainjectorDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = observabilityStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &certManagerCainjectorDeployment, 1)

	err = ctrlClient.Create(ctx, &certManagerWebhookDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = observabilityStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &certManagerWebhookDeployment, 1)

	// cleanup
	deleteDeployments(ctx, t, ctrlClient, &otelDeployment, &metricsPowerflexDeployment, &topologyDeployment, &certManagerDeployment, &certManagerCainjectorDeployment, &certManagerWebhookDeployment)
}

func recreateDeployment(ctx context.Context, t *testing.T, client client.WithWatch, deployment *appsv1.Deployment, readyReplicas int32) {
	err := client.Delete(ctx, deployment)
	assert.NoError(t, err, "failed to update client object during test setup")

	deployment.Status.ReadyReplicas = readyReplicas
	deployment.ResourceVersion = ""
	err = client.Create(ctx, deployment)
	assert.NoError(t, err, "failed to create client object during test setup")
}

func deleteDeployments(ctx context.Context, t *testing.T, client client.WithWatch, deployments ...*appsv1.Deployment) {
	for _, deployment := range deployments {
		err := client.Delete(ctx, deployment)
		assert.NoError(t, err, "failed to update client object during test setup")
	}
}

func TestAuthProxyStatusCheck(t *testing.T) {
	// Create a fake context.Context
	ctx := context.Background()
	ctrlClient := fullFakeClient()

	// Create a fake csm1 of csmv1.ContainerStorageModule
	csm1 := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: csmv1.Driver{
				CSIDriverType: "powerflex",
			},
			Modules: []csmv1.Module{
				{
					Name:    csmv1.AuthorizationServer,
					Enabled: true,
					Components: []csmv1.ContainerTemplate{
						{
							Name:    "ingress-nginx",
							Enabled: &[]bool{true}[0],
						},
						{
							Name:    "cert-manager",
							Enabled: &[]bool{true}[0],
						},
					},
				},
			},
		},
	}

	// add the CSM object to the client
	err := ctrlClient.Create(ctx, &csm1)
	assert.NoError(t, err, "failed to create client object during test setup")
	i32One := int32(1)

	// add fake deployments to the client
	// first set of deployments: karavi
	deployment1 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-namespace-ingress-nginx-controller",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment2 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment3 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager-cainjector",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment4 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager-webhook",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment5 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxy-server",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment6 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-commander",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment7 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-primary",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment8 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "role-service",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment9 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "storage-service",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment10 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-service",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	deployment11 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "authorization-controller",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	err = ctrlClient.Create(ctx, &deployment1)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment2)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment3)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment4)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment5)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment6)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment7)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment8)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment9)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment10)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &deployment11)
	assert.NoError(t, err, "failed to create client object during test setup")

	// Create a fake instance of ReconcileCSM
	fakeReconcile := FakeReconcileCSM{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
	}

	// test 1: pods are running
	status, err := authProxyStatusCheck(ctx, &csm1, &fakeReconcile, nil)
	assert.Nil(t, err)
	assert.Equal(t, true, status)
}

func TestAuthProxyStatusCheckError(t *testing.T) {
	// Create a fake context.Context
	ctx := context.Background()
	ctrlClient := fullFakeClient()

	// Create a fake csm1 of csmv1.ContainerStorageModule
	csm := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Modules: []csmv1.Module{
				{
					Name:    csmv1.AuthorizationServer,
					Enabled: true,
					Components: []csmv1.ContainerTemplate{
						{
							Name:    "ingress-nginx",
							Enabled: &[]bool{true}[0],
						},
						{
							Name:    "cert-manager",
							Enabled: &[]bool{true}[0],
						},
					},
				},
			},
		},
	}

	// add the CSM object to the client
	err := ctrlClient.Create(ctx, &csm)
	assert.NoError(t, err, "failed to create client object during test setup")
	i32One := int32(1)

	nginxDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-namespace-ingress-nginx-controller",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	certManagerDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	certManagerCainjectorDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager-cainjector",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	certManagerWebhookDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager-webhook",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	proxyServerDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxy-server",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	redisCommanderDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-commander",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	redisPrimaryDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-primary",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	roleServiceDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "role-service",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	storageServiceDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "storage-service",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	tenantServiceDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-service",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	authorizationControllerDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "authorization-controller",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}

	err = ctrlClient.Create(ctx, &nginxDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	// Create a fake instance of ReconcileCSM
	fakeReconcile := FakeReconcileCSM{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
	}

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &nginxDeployment, 1)

	err = ctrlClient.Create(ctx, &certManagerDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &certManagerDeployment, 1)

	err = ctrlClient.Create(ctx, &certManagerCainjectorDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &certManagerCainjectorDeployment, 1)

	err = ctrlClient.Create(ctx, &certManagerWebhookDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &certManagerWebhookDeployment, 1)

	err = ctrlClient.Create(ctx, &proxyServerDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &proxyServerDeployment, 1)

	err = ctrlClient.Create(ctx, &redisCommanderDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &redisCommanderDeployment, 1)

	err = ctrlClient.Create(ctx, &redisPrimaryDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &redisPrimaryDeployment, 1)

	err = ctrlClient.Create(ctx, &roleServiceDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &roleServiceDeployment, 1)

	err = ctrlClient.Create(ctx, &storageServiceDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &storageServiceDeployment, 1)

	err = ctrlClient.Create(ctx, &tenantServiceDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &tenantServiceDeployment, 1)

	err = ctrlClient.Create(ctx, &authorizationControllerDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &authorizationControllerDeployment, 1)

	deleteDeployments(ctx, t, ctrlClient, &nginxDeployment, &certManagerDeployment, &certManagerCainjectorDeployment, &certManagerWebhookDeployment, &proxyServerDeployment, &redisCommanderDeployment, &redisPrimaryDeployment, &roleServiceDeployment, &storageServiceDeployment, &tenantServiceDeployment)
}

func TestSetStatus(t *testing.T) {
	ctx := context.Background()
	instance := createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil)

	newStatus := &csmv1.ContainerStorageModuleStatus{
		State: constants.Succeeded,
		NodeStatus: csmv1.PodStatus{
			Available: "1",
			Failed:    "0",
			Desired:   "1",
		},
		ControllerStatus: csmv1.PodStatus{
			Available: "1",
			Failed:    "0",
			Desired:   "1",
		},
	}

	SetStatus(ctx, nil, instance, newStatus)

	assert.Equal(t, newStatus, instance.GetCSMStatus())
}

func TestHandleValidationError(t *testing.T) {
	type args struct {
		ctx             context.Context
		instance        *csmv1.ContainerStorageModule
		r               ReconcileCSM
		validationError error
	}

	tests := []struct {
		name           string
		args           args
		expectedResult reconcile.Result
		wantErr        bool
	}{
		{
			name: "Test HandleValidationError ",
			args: args{
				ctx:      context.Background(),
				instance: createCSMWithStatus("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil, csmv1.ContainerStorageModuleStatus{State: constants.Creating}),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Namespace",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "powerflex",
						},
					}).WithObjects(&appsv1.DaemonSet{
						TypeMeta: metav1.TypeMeta{
							Kind:       "DaemonSet",
							APIVersion: "apps/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "powerflex-node",
							Namespace: "powerflex",
						},
					}).Build(),
					K8sClient: fake.NewSimpleClientset(),
				},
				validationError: fmt.Errorf("validation error"),
			},
			expectedResult: reconcile.Result{Requeue: false},
			wantErr:        true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := HandleValidationError(test.args.ctx, test.args.instance, test.args.r, test.args.validationError)
			if (err != nil) != test.wantErr {
				t.Errorf("HandleValidationError() error = %v, wantErr %v", err, test.wantErr)
				return
			}
			assert.Equal(t, test.expectedResult, result)
			assert.Equal(t, constants.Failed, test.args.instance.GetCSMStatus().State)
		})
	}
}

func TestHandleSuccess(t *testing.T) {
	type args struct {
		ctx       context.Context
		instance  *csmv1.ContainerStorageModule
		r         ReconcileCSM
		oldStatus *csmv1.ContainerStorageModuleStatus
		newStatus *csmv1.ContainerStorageModuleStatus
	}

	tests := []struct {
		name string
		args args
		want reconcile.Result
	}{
		{
			name: "Test TestHandleSuccess with no change in status",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Namespace",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "powerflex",
						},
					}).WithObjects(&appsv1.DaemonSet{
						TypeMeta: metav1.TypeMeta{
							Kind:       "DaemonSet",
							APIVersion: "apps/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "powerflex-node",
							Namespace: "powerflex",
						},
						Status: appsv1.DaemonSetStatus{
							DesiredNumberScheduled: 1,
						},
					}).WithObjects(
						&corev1.Pod{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Pod",
								APIVersion: "v1",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      "powerflex-driver",
								Namespace: "powerflex",
								Labels: map[string]string{
									"app": "powerflex-node",
								},
							},
							Status: corev1.PodStatus{
								Phase: corev1.PodRunning,
								Conditions: []corev1.PodCondition{
									{Type: corev1.PodReady, Status: corev1.ConditionTrue},
								},
								ContainerStatuses: []corev1.ContainerStatus{
									{
										State: corev1.ContainerState{
											Running: &corev1.ContainerStateRunning{
												StartedAt: metav1.Time{Time: time.Now()},
											},
										},
									},
								},
							},
						}).Build(),
					K8sClient: fake.NewSimpleClientset(),
				},
				oldStatus: &csmv1.ContainerStorageModuleStatus{
					ControllerStatus: csmv1.PodStatus{
						Available: "1",
						Failed:    "0",
						Desired:   "1",
					},
					NodeStatus: csmv1.PodStatus{
						Available: "1",
						Failed:    "0",
						Desired:   "1",
					},
					State: constants.Succeeded,
				},
				newStatus: &csmv1.ContainerStorageModuleStatus{
					ControllerStatus: csmv1.PodStatus{
						Available: "1",
						Failed:    "0",
						Desired:   "1",
					},
					NodeStatus: csmv1.PodStatus{
						Available: "1",
						Failed:    "0",
						Desired:   "1",
					},
					State: constants.Succeeded,
				},
			},
			want: reconcile.Result{
				Requeue: false,
			},
		},
		{
			name: "Test TestHandleSuccess with change in status",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Namespace",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "powerflex",
						},
					}).WithObjects(&appsv1.DaemonSet{
						TypeMeta: metav1.TypeMeta{
							Kind:       "DaemonSet",
							APIVersion: "apps/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "powerflex-controller",
							Namespace: "powerflex",
						},
					}).Build(),
					K8sClient: fake.NewSimpleClientset(),
				},
				oldStatus: &csmv1.ContainerStorageModuleStatus{
					ControllerStatus: csmv1.PodStatus{
						Available: "0",
						Failed:    "0",
						Desired:   "1",
					},
					NodeStatus: csmv1.PodStatus{
						Available: "0",
						Failed:    "0",
						Desired:   "1",
					},
					State: constants.Succeeded,
				},
				newStatus: &csmv1.ContainerStorageModuleStatus{
					ControllerStatus: csmv1.PodStatus{
						Available: "1",
						Failed:    "0",
						Desired:   "1",
					},
					NodeStatus: csmv1.PodStatus{
						Available: "1",
						Failed:    "0",
						Desired:   "1",
					},
					State: constants.Succeeded,
				},
			},
			want: reconcile.Result{
				Requeue: true,
			},
		},
		{
			name: "Test TestHandleSuccess with change in status not successful",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Namespace",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "powerflex",
						},
					}).WithObjects(&appsv1.DaemonSet{
						TypeMeta: metav1.TypeMeta{
							Kind:       "DaemonSet",
							APIVersion: "apps/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "powerflex-controller",
							Namespace: "powerflex",
						},
					}).Build(),
					K8sClient: fake.NewSimpleClientset(),
				},
				oldStatus: &csmv1.ContainerStorageModuleStatus{
					ControllerStatus: csmv1.PodStatus{
						Available: "0",
						Failed:    "0",
						Desired:   "1",
					},
					NodeStatus: csmv1.PodStatus{
						Available: "0",
						Failed:    "0",
						Desired:   "1",
					},
					State: constants.Failed,
				},
				newStatus: &csmv1.ContainerStorageModuleStatus{
					ControllerStatus: csmv1.PodStatus{
						Available: "0",
						Failed:    "0",
						Desired:   "1",
					},
					NodeStatus: csmv1.PodStatus{
						Available: "0",
						Failed:    "0",
						Desired:   "1",
					},
					State: constants.Succeeded,
				},
			},
			want: reconcile.Result{
				Requeue: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requeue := HandleSuccess(test.args.ctx, test.args.instance, test.args.r, test.args.newStatus, test.args.oldStatus)
			assert.Equal(t, test.want, requeue)
		})
	}
}

// helpers
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

func createCSMWithStatus(name string, namespace string, driverType csmv1.DriverType, moduleType csmv1.ModuleType, moduleEnabled bool, components []csmv1.ContainerTemplate, status csmv1.ContainerStorageModuleStatus) *csmv1.ContainerStorageModule {
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
		Status: status,
	}
}

func TestUpdateStatus(t *testing.T) {
	ctx := context.TODO()

	// Define the initial ContainerStorageModule instance
	instance := &csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "default",
			UID:             "test-uid",
			ResourceVersion: "1",
		},
		Status: csmv1.ContainerStorageModuleStatus{
			State: "oldState",
			ControllerStatus: csmv1.PodStatus{
				Available: "1",
				Failed:    "0",
				Desired:   "1",
			},
			NodeStatus: csmv1.PodStatus{
				Available: "1",
				Failed:    "0",
				Desired:   "1",
			},
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-controller",
			Namespace: "default",
		},
	}

	daemonset := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node",
			Namespace: "default",
		},
	}

	// Define the new status for the update
	newStatus := &csmv1.ContainerStorageModuleStatus{
		State: constants.Succeeded,
		NodeStatus: csmv1.PodStatus{
			Available: "1",
			Failed:    "0",
			Desired:   "1",
		},
		ControllerStatus: csmv1.PodStatus{
			Available: "1",
			Failed:    "0",
			Desired:   "1",
		},
	}

	// Register the CRD with the scheme
	s := runtime.NewScheme()
	if err := csmv1.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add csmv1 scheme: %v", err)
	}
	err := corev1.AddToScheme(s)
	if err != nil {
		t.Fatal(err)
	}
	err = appsv1.AddToScheme(s)
	if err != nil {
		t.Fatal(err)
	}

	fakeClient := ctrlClientFake.NewClientBuilder().WithScheme(s).WithObjects(instance, deployment, daemonset).Build()

	// Ensure the instance exists in the fake client
	foundInstance := &csmv1.ContainerStorageModule{}
	err = fakeClient.Get(ctx, client.ObjectKey{Name: "test", Namespace: "default"}, foundInstance)
	if err != nil {
		t.Fatalf("Failed to get instance from fake client: %v", err)
	}

	// Mock the FakeReconcileCSM to simulate GetUpdateCount
	r := &FakeReconcileCSM{
		Client:    fakeClient,
		K8sClient: fake.NewSimpleClientset(),
	}

	// UpdateStatus function to be tested.
	err = UpdateStatus(ctx, instance, r, newStatus)

	assert.Error(t, err)
	assert.Equal(t, "containerstoragemodules.storage.dell.com \"test\" not found", err.Error())

	// Ensure the update count is incremented
	r.IncrUpdateCount()
	assert.Equal(t, int32(1), r.GetUpdateCount())
}

func TestUpdateStatusAuthorizationProxyServer(t *testing.T) {
	ctx := context.TODO()

	// Define the initial ContainerStorageModule instance
	instance := &csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "default",
			UID:             "test-uid",
			ResourceVersion: "1",
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Modules: []csmv1.Module{
				{
					Name:    csmv1.AuthorizationServer,
					Enabled: true,
				},
			},
		},
		Status: csmv1.ContainerStorageModuleStatus{
			State: "oldState",
			ControllerStatus: csmv1.PodStatus{
				Available: "1",
				Failed:    "0",
				Desired:   "1",
			},
			NodeStatus: csmv1.PodStatus{
				Available: "1",
				Failed:    "0",
				Desired:   "1",
			},
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-controller",
			Namespace: "default",
		},
	}

	daemonset := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node",
			Namespace: "default",
		},
	}

	// Define the new status for the update
	newStatus := &csmv1.ContainerStorageModuleStatus{
		State: constants.Succeeded,
		NodeStatus: csmv1.PodStatus{
			Available: "1",
			Failed:    "0",
			Desired:   "1",
		},
		ControllerStatus: csmv1.PodStatus{
			Available: "1",
			Failed:    "0",
			Desired:   "1",
		},
	}

	// Register the CRD with the scheme
	s := runtime.NewScheme()
	if err := csmv1.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add csmv1 scheme: %v", err)
	}
	err := corev1.AddToScheme(s)
	if err != nil {
		t.Fatal(err)
	}
	err = appsv1.AddToScheme(s)
	if err != nil {
		t.Fatal(err)
	}

	fakeClient := ctrlClientFake.NewClientBuilder().WithScheme(s).WithObjects(instance, deployment, daemonset).Build()

	// Ensure the instance exists in the fake client
	foundInstance := &csmv1.ContainerStorageModule{}
	err = fakeClient.Get(ctx, client.ObjectKey{Name: "test", Namespace: "default"}, foundInstance)
	if err != nil {
		t.Fatalf("Failed to get instance from fake client: %v", err)
	}

	// Mock the FakeReconcileCSM to simulate GetUpdateCount
	r := &FakeReconcileCSM{
		Client:    fakeClient,
		K8sClient: fake.NewSimpleClientset(),
	}

	// UpdateStatus function to be tested.
	err = UpdateStatus(ctx, instance, r, newStatus)

	assert.Error(t, err)
	assert.Equal(t, "containerstoragemodules.storage.dell.com \"test\" not found", err.Error())

	// Ensure the update count is incremented
	r.IncrUpdateCount()
	assert.Equal(t, int32(1), r.GetUpdateCount())
}
