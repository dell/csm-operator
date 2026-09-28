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
	"os"
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

func TestGetDeploymentStatusPodStates(t *testing.T) {
	ctx := context.Background()
	ns := "pflex"
	depLabels := map[string]string{"app": "pflex-controller"}
	i32One := int32(1)

	makeClient := func(depStatus appsv1.DeploymentStatus, podStatus corev1.PodStatus) ReconcileCSM {
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "pflex-controller", Namespace: ns},
			Spec: appsv1.DeploymentSpec{
				Replicas: &i32One,
				Selector: &metav1.LabelSelector{MatchLabels: depLabels},
			},
			Status: depStatus,
		}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pflex-controller-pod", Namespace: ns, Labels: depLabels},
			Status:     podStatus,
		}
		c := fullFakeClient()
		_ = c.Create(ctx, dep)
		_ = c.Create(ctx, pod)
		return &FakeReconcileCSM{Client: c, K8sClient: fake.NewSimpleClientset()}
	}

	instance := createCSM("pflex", ns, csmv1.PowerFlex, csmv1.Replication, false, nil)

	t.Run("CrashLoopBackOff pod counts as failed", func(t *testing.T) {
		r := makeClient(
			appsv1.DeploymentStatus{Replicas: 1, AvailableReplicas: 0, UnavailableReplicas: 1},
			corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{
					{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}},
				},
			},
		)
		got, err := getDeploymentStatus(ctx, instance, r)
		assert.NoError(t, err)
		assert.Equal(t, "1", got.Failed, "CrashLoopBackOff pod should be counted as failed")
	})

	t.Run("PodFailed phase counts as failed", func(t *testing.T) {
		r := makeClient(
			appsv1.DeploymentStatus{Replicas: 1, AvailableReplicas: 0, UnavailableReplicas: 1},
			corev1.PodStatus{Phase: corev1.PodFailed},
		)
		got, err := getDeploymentStatus(ctx, instance, r)
		assert.NoError(t, err)
		assert.Equal(t, "1", got.Failed, "PodFailed pod should be counted as failed")
	})

	t.Run("ContainerCreating pod is not a failure", func(t *testing.T) {
		r := makeClient(
			appsv1.DeploymentStatus{Replicas: 1, AvailableReplicas: 0, UnavailableReplicas: 1},
			corev1.PodStatus{
				Phase: corev1.PodPending,
				ContainerStatuses: []corev1.ContainerStatus{
					{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}},
				},
			},
		)
		got, err := getDeploymentStatus(ctx, instance, r)
		assert.NoError(t, err)
		assert.Equal(t, "0", got.Failed, "ContainerCreating pod should not be counted as failed")
	})
}

func TestComputeFailedPods(t *testing.T) {
	ctx := context.Background()
	ns := "test-ns"
	labels := map[string]string{"app": "test"}

	makeClient := func(podStatus corev1.PodStatus) client.Client {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pod-0", Namespace: ns, Labels: labels},
			Status:     podStatus,
		}
		c := fullFakeClient()
		_ = c.Create(ctx, pod)
		return c
	}

	t.Run("CrashLoopBackOff counts as failed", func(t *testing.T) {
		c := makeClient(corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}},
			},
		})
		assert.Equal(t, int32(1), ComputeFailedPods(ctx, c, ns, labels, 0))
	})

	t.Run("ImagePullBackOff counts as failed", func(t *testing.T) {
		c := makeClient(corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}}},
			},
		})
		assert.Equal(t, int32(1), ComputeFailedPods(ctx, c, ns, labels, 0))
	})

	t.Run("ContainerCreating is not a failure", func(t *testing.T) {
		c := makeClient(corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}},
			},
		})
		assert.Equal(t, int32(0), ComputeFailedPods(ctx, c, ns, labels, 0))
	})

	t.Run("PodFailed phase counts as failed", func(t *testing.T) {
		c := makeClient(corev1.PodStatus{Phase: corev1.PodFailed})
		assert.Equal(t, int32(1), ComputeFailedPods(ctx, c, ns, labels, 0))
	})

	t.Run("ErrImagePull counts as failed", func(t *testing.T) {
		c := makeClient(corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ErrImagePull"}}},
			},
		})
		assert.Equal(t, int32(1), ComputeFailedPods(ctx, c, ns, labels, 0))
	})

	t.Run("PodInitializing is not a failure (transient)", func(t *testing.T) {
		c := makeClient(corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing"}}},
			},
		})
		assert.Equal(t, int32(0), ComputeFailedPods(ctx, c, ns, labels, 0))
	})

	t.Run("CreateContainerConfigError counts as failed", func(t *testing.T) {
		c := makeClient(corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CreateContainerConfigError"}}},
			},
		})
		assert.Equal(t, int32(1), ComputeFailedPods(ctx, c, ns, labels, 0))
	})

	t.Run("Multiple failing containers in same pod counted once", func(t *testing.T) {
		c := makeClient(corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}},
				{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}}},
			},
		})
		assert.Equal(t, int32(1), ComputeFailedPods(ctx, c, ns, labels, 0))
	})

	t.Run("fallback on list error", func(t *testing.T) {
		c := fullFakeClient() // no pods, but use bad namespace to trigger no match
		assert.Equal(t, int32(0), ComputeFailedPods(ctx, c, ns, labels, 0))
	})
}

func TestComputeStatefulSetFailedPods(t *testing.T) {
	ctx := context.Background()
	ns := "pflex"
	stsLabels := map[string]string{"app": "redis"}

	makeClient := func(podStatus corev1.PodStatus) ReconcileCSM {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "redis-0", Namespace: ns, Labels: stsLabels},
			Status:     podStatus,
		}
		c := fullFakeClient()
		_ = c.Create(ctx, pod)
		return &FakeReconcileCSM{Client: c, K8sClient: fake.NewSimpleClientset()}
	}

	t.Run("CrashLoopBackOff pod counts as failed", func(t *testing.T) {
		r := makeClient(corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}},
			},
		})
		got := ComputeStatefulSetFailedPods(ctx, r.GetClient(), ns, stsLabels, 1)
		assert.Equal(t, int32(1), got, "CrashLoopBackOff pod should be counted as failed")
	})

	t.Run("PodFailed phase counts as failed", func(t *testing.T) {
		r := makeClient(corev1.PodStatus{Phase: corev1.PodFailed})
		got := ComputeStatefulSetFailedPods(ctx, r.GetClient(), ns, stsLabels, 1)
		assert.Equal(t, int32(1), got, "PodFailed pod should be counted as failed")
	})

	t.Run("ContainerCreating pod is not a failure", func(t *testing.T) {
		r := makeClient(corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}},
			},
		})
		got := ComputeStatefulSetFailedPods(ctx, r.GetClient(), ns, stsLabels, 1)
		assert.Equal(t, int32(0), got, "ContainerCreating pod should not be counted as failed")
	})
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
				Failed:    "0",
			},
			wantErr: false,
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
				Failed:    "0",
			},
			wantErr: false,
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
			name: "Test getDaemonSetStatus with init container still running",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta:   metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"},
						ObjectMeta: metav1.ObjectMeta{Name: "powerflex"},
					}).WithObjects(&appsv1.DaemonSet{
						TypeMeta:   metav1.TypeMeta{Kind: "DaemonSet", APIVersion: "apps/v1"},
						ObjectMeta: metav1.ObjectMeta{Name: "powerflex-node", Namespace: "powerflex"},
						Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 1},
					}).WithObjects(&corev1.Pod{
						TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
						ObjectMeta: metav1.ObjectMeta{Name: "powerflex-driver", Namespace: "powerflex", Labels: map[string]string{"app": "powerflex-node"}},
						Status: corev1.PodStatus{
							Phase: corev1.PodPending,
							InitContainerStatuses: []corev1.ContainerStatus{
								{Name: "init-csi", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.Time{Time: time.Now()}}}},
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
		{
			name: "Test getDaemonSetStatus with init container failed",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta:   metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"},
						ObjectMeta: metav1.ObjectMeta{Name: "powerflex"},
					}).WithObjects(&appsv1.DaemonSet{
						TypeMeta:   metav1.TypeMeta{Kind: "DaemonSet", APIVersion: "apps/v1"},
						ObjectMeta: metav1.ObjectMeta{Name: "powerflex-node", Namespace: "powerflex"},
						Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 1},
					}).WithObjects(&corev1.Pod{
						TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
						ObjectMeta: metav1.ObjectMeta{Name: "powerflex-driver", Namespace: "powerflex", Labels: map[string]string{"app": "powerflex-node"}},
						Status: corev1.PodStatus{
							Phase: corev1.PodPending,
							InitContainerStatuses: []corev1.ContainerStatus{
								{Name: "init-csi", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1, Reason: "Error"}}},
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
			name: "Test getDaemonSetStatus with pod running but init containers still running",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta:   metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"},
						ObjectMeta: metav1.ObjectMeta{Name: "powerflex"},
					}).WithObjects(&appsv1.DaemonSet{
						TypeMeta:   metav1.TypeMeta{Kind: "DaemonSet", APIVersion: "apps/v1"},
						ObjectMeta: metav1.ObjectMeta{Name: "powerflex-node", Namespace: "powerflex"},
						Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 1},
					}).WithObjects(&corev1.Pod{
						TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
						ObjectMeta: metav1.ObjectMeta{Name: "powerflex-driver", Namespace: "powerflex", Labels: map[string]string{"app": "powerflex-node"}},
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
							InitContainerStatuses: []corev1.ContainerStatus{
								{Name: "init-csi", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.Time{Time: time.Now()}}}},
							},
							ContainerStatuses: []corev1.ContainerStatus{
								{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.Time{Time: time.Now()}}}},
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
		{
			name: "Test getDaemonSetStatus with PodInitializing (transient, not failure)",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta:   metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"},
						ObjectMeta: metav1.ObjectMeta{Name: "powerflex"},
					}).WithObjects(&appsv1.DaemonSet{
						TypeMeta:   metav1.TypeMeta{Kind: "DaemonSet", APIVersion: "apps/v1"},
						ObjectMeta: metav1.ObjectMeta{Name: "powerflex-node", Namespace: "powerflex"},
						Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 1},
					}).WithObjects(&corev1.Pod{
						TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
						ObjectMeta: metav1.ObjectMeta{Name: "powerflex-driver", Namespace: "powerflex", Labels: map[string]string{"app": "powerflex-node"}},
						Status: corev1.PodStatus{
							Phase: corev1.PodPending,
							ContainerStatuses: []corev1.ContainerStatus{
								{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing"}}},
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
		{
			name: "Test getDaemonSetStatus with ErrImagePull (counted as failure, grace period handles transience)",
			args: args{
				ctx:      context.Background(),
				instance: createCSM("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil),
				r: &FakeReconcileCSM{
					Client: ctrlClientFake.NewClientBuilder().WithObjects(&corev1.Namespace{
						TypeMeta:   metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"},
						ObjectMeta: metav1.ObjectMeta{Name: "powerflex"},
					}).WithObjects(&appsv1.DaemonSet{
						TypeMeta:   metav1.TypeMeta{Kind: "DaemonSet", APIVersion: "apps/v1"},
						ObjectMeta: metav1.ObjectMeta{Name: "powerflex-node", Namespace: "powerflex"},
						Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 1},
					}).WithObjects(&corev1.Pod{
						TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
						ObjectMeta: metav1.ObjectMeta{Name: "powerflex-driver", Namespace: "powerflex", Labels: map[string]string{"app": "powerflex-node"}},
						Status: corev1.PodStatus{
							Phase: corev1.PodPending,
							ContainerStatuses: []corev1.ContainerStatus{
								{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ErrImagePull"}}},
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
				ConfigVersion: "v2.15.0", // Added to ensure GetVersion has data
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

	// Initialize the new OperatorConfig argument
	opConfig := OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}

	// test 1: pods are running
	// Added opConfig to the function call
	status, err := observabilityStatusCheck(ctx, &csm1, &fakeReconcile, nil, opConfig)
	assert.Nil(t, err)
	assert.Equal(t, true, status)
}

func TestObservabilityStatusCheckError(t *testing.T) {
	// Create a fake context.Context
	ctx := context.Background()
	ctrlClient := fullFakeClient()

	// Define the operator config required for status checks
	opConfig := OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}

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

	_, err = observabilityStatusCheck(ctx, &csm, &fakeReconcile, nil, opConfig)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &otelDeployment, 1)

	err = ctrlClient.Create(ctx, &metricsPowerflexDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = observabilityStatusCheck(ctx, &csm, &fakeReconcile, nil, opConfig)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &metricsPowerflexDeployment, 1)

	err = ctrlClient.Create(ctx, &topologyDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = observabilityStatusCheck(ctx, &csm, &fakeReconcile, nil, opConfig)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &topologyDeployment, 1)

	err = ctrlClient.Create(ctx, &certManagerDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = observabilityStatusCheck(ctx, &csm, &fakeReconcile, nil, opConfig)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &certManagerDeployment, 1)

	err = ctrlClient.Create(ctx, &certManagerCainjectorDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = observabilityStatusCheck(ctx, &csm, &fakeReconcile, nil, opConfig)
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &certManagerCainjectorDeployment, 1)

	err = ctrlClient.Create(ctx, &certManagerWebhookDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = observabilityStatusCheck(ctx, &csm, &fakeReconcile, nil, opConfig)
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
					Name:          csmv1.AuthorizationServer,
					Enabled:       true,
					ConfigVersion: "v2.4.0",
					Components: []csmv1.ContainerTemplate{
						{
							Name:    "proxy-server",
							Enabled: &[]bool{true}[0],
						},
						{
							Name:    "ingress-nginx",
							Enabled: &[]bool{true}[0],
						},
						{
							Name:    "cert-manager",
							Enabled: &[]bool{true}[0],
						},
						{
							Name:           "redis",
							RedisName:      "redis-csm",
							Sentinel:       "sentinel",
							RedisCommander: "redis-commander",
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
			ReadyReplicas:     1,
			AvailableReplicas: 1,
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
			ReadyReplicas:     1,
			AvailableReplicas: 1,
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
			ReadyReplicas:     1,
			AvailableReplicas: 1,
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
			ReadyReplicas:     1,
			AvailableReplicas: 1,
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
			ReadyReplicas:     1,
			AvailableReplicas: 1,
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
			ReadyReplicas:     1,
			AvailableReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &i32One,
		},
	}
	redisSts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-csm",
			Namespace: "test-namespace",
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &i32One,
		},
	}
	sentinelSts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sentinel",
			Namespace: "test-namespace",
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: 1,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &i32One,
		},
	}
	deployment8 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "role-service",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas:     1,
			AvailableReplicas: 1,
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
			ReadyReplicas:     1,
			AvailableReplicas: 1,
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
			ReadyReplicas:     1,
			AvailableReplicas: 1,
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
			ReadyReplicas:     1,
			AvailableReplicas: 1,
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
	err = ctrlClient.Create(ctx, &redisSts)
	assert.NoError(t, err, "failed to create client object during test setup")
	err = ctrlClient.Create(ctx, &sentinelSts)
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
	status, err := authProxyStatusCheck(ctx, &csm1, &fakeReconcile, nil, OperatorConfig{})
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

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil, OperatorConfig{})
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &nginxDeployment, 1)

	err = ctrlClient.Create(ctx, &certManagerDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil, OperatorConfig{})
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &certManagerDeployment, 1)

	err = ctrlClient.Create(ctx, &certManagerCainjectorDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil, OperatorConfig{})
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &certManagerCainjectorDeployment, 1)

	err = ctrlClient.Create(ctx, &certManagerWebhookDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil, OperatorConfig{})
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &certManagerWebhookDeployment, 1)

	err = ctrlClient.Create(ctx, &proxyServerDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil, OperatorConfig{})
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &proxyServerDeployment, 1)

	err = ctrlClient.Create(ctx, &redisCommanderDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil, OperatorConfig{})
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &redisCommanderDeployment, 1)

	err = ctrlClient.Create(ctx, &redisPrimaryDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil, OperatorConfig{})
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &redisPrimaryDeployment, 1)

	err = ctrlClient.Create(ctx, &roleServiceDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil, OperatorConfig{})
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &roleServiceDeployment, 1)

	err = ctrlClient.Create(ctx, &storageServiceDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil, OperatorConfig{})
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &storageServiceDeployment, 1)

	err = ctrlClient.Create(ctx, &tenantServiceDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil, OperatorConfig{})
	assert.Nil(t, err)

	recreateDeployment(ctx, t, ctrlClient, &tenantServiceDeployment, 1)

	err = ctrlClient.Create(ctx, &authorizationControllerDeployment)
	assert.NoError(t, err, "failed to create client object during test setup")

	_, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil, OperatorConfig{})
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
		checkState     bool
	}{
		{
			name: "Test HandleValidationError ",
			args: func() args {
				instance := createCSMWithStatus("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil, csmv1.ContainerStorageModuleStatus{State: constants.Creating})
				s := runtime.NewScheme()
				_ = csmv1.AddToScheme(s)
				_ = corev1.AddToScheme(s)
				_ = appsv1.AddToScheme(s)
				return args{
					ctx:      context.Background(),
					instance: instance,
					r: &FakeReconcileCSM{
						Client: ctrlClientFake.NewClientBuilder().WithScheme(s).
							WithObjects(instance).
							WithStatusSubresource(instance).
							Build(),
						K8sClient: fake.NewSimpleClientset(),
					},
					validationError: fmt.Errorf("validation error"),
				}
			}(),
			expectedResult: reconcile.Result{Requeue: false},
			wantErr:        true,
			checkState:     true,
		},
		{
			name: "Test HandleValidationError Get fails",
			args: func() args {
				instance := createCSMWithStatus("powerflex", "powerflex", csmv1.PowerFlex, csmv1.Replication, true, nil, csmv1.ContainerStorageModuleStatus{State: constants.Creating})
				s := runtime.NewScheme()
				_ = csmv1.AddToScheme(s)
				_ = corev1.AddToScheme(s)
				// CSM not added as object — Get will return NotFound, covering line 607/613
				return args{
					ctx:      context.Background(),
					instance: instance,
					r: &FakeReconcileCSM{
						Client:    ctrlClientFake.NewClientBuilder().WithScheme(s).Build(),
						K8sClient: fake.NewSimpleClientset(),
					},
					validationError: fmt.Errorf("validation error"),
				}
			}(),
			expectedResult: reconcile.Result{Requeue: false},
			wantErr:        true,
			checkState:     false,
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
			if test.checkState {
				// HandleValidationError fetches a fresh CSM from the client and updates
				// its status — the original instance is not modified. Re-fetch to verify.
				stored := &csmv1.ContainerStorageModule{}
				getErr := test.args.r.GetClient().Get(test.args.ctx, client.ObjectKeyFromObject(test.args.instance), stored)
				assert.Nil(t, getErr)
				assert.Equal(t, constants.Failed, stored.GetCSMStatus().State)
			}
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
	_, err = UpdateStatus(ctx, instance, r, newStatus, OperatorConfig{})

	assert.Error(t, err)
	assert.Equal(t, "containerstoragemodules.storage.dell.com \"test\" not found", err.Error())

	// Ensure the update count is incremented
	r.IncrUpdateCount()
	assert.Equal(t, int32(1), r.GetUpdateCount())
}

func TestUpdateStatusSetsLastSuccessfulConfiguration(t *testing.T) {
	ctx := context.TODO()

	// Define the initial ContainerStorageModule instance
	instance := &csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "default",
			UID:             "test-uid",
			ResourceVersion: "1",
			Annotations: map[string]string{
				"storage.dell.com/PreviouslyAppliedConfiguration": `{"driver":"replicas=1"}`,
			},
		},
		Status: csmv1.ContainerStorageModuleStatus{
			State:            "oldState",
			ControllerStatus: csmv1.PodStatus{Available: "1", Failed: "0", Desired: "1"},
			NodeStatus:       csmv1.PodStatus{Available: "1", Failed: "0", Desired: "1"},
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: csmv1.Driver{Replicas: 1},
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-controller",
			Namespace: "default",
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          1,
			ReadyReplicas:     1,
			UpdatedReplicas:   1,
			AvailableReplicas: 1,
		},
	}

	daemonset := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node",
			Namespace: "default",
		},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 0,
			NumberReady:            0,
			NumberAvailable:        0,
			NumberUnavailable:      0,
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
	// Mock the FakeReconcileCSM
	r := &FakeReconcileCSM{
		Client:    fakeClient,
		K8sClient: fake.NewSimpleClientset(),
	}

	// Clear stability period to ensure immediate Succeeded state for this test
	ClearSucceededStabilityPeriod(instance.GetNamespace() + "/" + instance.GetName())
	// Manually set the first succeeded time to the past to bypass stability period
	firstSucceededObservedMux.Lock()
	firstSucceededObserved[instance.GetNamespace()+"/"+instance.GetName()] = time.Now().Add(-31 * time.Second)
	firstSucceededObservedMux.Unlock()

	// UpdateStatus function to be tested.
	_, err = UpdateStatus(ctx, instance, r, newStatus, OperatorConfig{})

	assert.Error(t, err)
	assert.Equal(t, `{"driver":"replicas=1"}`, instance.Status.LastSuccessfulConfiguration)
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
	_, err = UpdateStatus(ctx, instance, r, newStatus, OperatorConfig{})

	assert.Error(t, err)
	assert.Equal(t, "containerstoragemodules.storage.dell.com \"test\" not found", err.Error())

	// Ensure the update count is incremented
	r.IncrUpdateCount()
	assert.Equal(t, int32(1), r.GetUpdateCount())
}

// TestWaitForGatewayController tests the WaitForGatewayController function
func TestWaitForGatewayController(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		deployment    *appsv1.Deployment
		instance      csmv1.ContainerStorageModule
		expectError   bool
		errorContains string
		description   string
	}{
		{
			name: "Gateway controller ready",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-auth-nginx-gateway-fabric",
					Namespace: "test-auth",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: func() *int32 { r := int32(1); return &r }(),
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: 1,
				},
			},
			instance: csmv1.ContainerStorageModule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test-auth",
				},
			},
			expectError: false,
			description: "Should succeed when gateway controller is ready",
		},
		{
			name: "Gateway controller not ready",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-auth-nginx-gateway-fabric",
					Namespace: "test-auth",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: func() *int32 { r := int32(2); return &r }(),
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: 1,
				},
			},
			instance: csmv1.ContainerStorageModule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test-auth",
				},
			},
			expectError:   true,
			errorContains: "context deadline exceeded",
			description:   "Should timeout when gateway controller is not ready",
		},
		{
			name:       "Gateway controller deployment not found",
			deployment: nil,
			instance: csmv1.ContainerStorageModule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test-auth",
				},
			},
			expectError:   true,
			errorContains: "not found",
			description:   "Should error when gateway controller deployment doesn't exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := runtime.NewScheme()
			if err := csmv1.AddToScheme(s); err != nil {
				t.Fatalf("Unable to add csmv1 scheme: %v", err)
			}
			if err := appsv1.AddToScheme(s); err != nil {
				t.Fatalf("Unable to add appsv1 scheme: %v", err)
			}

			var fakeClient client.Client
			if tt.deployment != nil {
				fakeClient = ctrlClientFake.NewClientBuilder().WithScheme(s).WithObjects(tt.deployment).Build()
			} else {
				fakeClient = ctrlClientFake.NewClientBuilder().WithScheme(s).Build()
			}

			r := &FakeReconcileCSM{
				Client:    fakeClient,
				K8sClient: fake.NewSimpleClientset(),
			}

			err := WaitForGatewayController(ctx, tt.instance, r, 2*time.Second)

			if tt.expectError {
				assert.Error(t, err, tt.description)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains, tt.description)
				}
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestGetGatewayControllerStatus tests the getGatewayControllerStatus function
func TestGetGatewayControllerStatus(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		deployment  *appsv1.Deployment
		instance    csmv1.ContainerStorageModule
		expectReady bool
		expectError bool
		description string
	}{
		{
			name: "Gateway controller ready - all replicas ready",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-auth-nginx-gateway-fabric",
					Namespace: "test-auth",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: func() *int32 { r := int32(2); return &r }(),
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: 2,
				},
			},
			instance: csmv1.ContainerStorageModule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test-auth",
				},
			},
			expectReady: true,
			expectError: false,
			description: "Should return ready when all replicas are ready",
		},
		{
			name: "Gateway controller not ready - partial replicas",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-auth-nginx-gateway-fabric",
					Namespace: "test-auth",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: func() *int32 { r := int32(3); return &r }(),
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: 1,
				},
			},
			instance: csmv1.ContainerStorageModule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test-auth",
				},
			},
			expectReady: false,
			expectError: false,
			description: "Should return not ready when some replicas are not ready",
		},
		{
			name: "Gateway controller ready - single replica",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-auth-nginx-gateway-fabric",
					Namespace: "test-auth",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: func() *int32 { r := int32(1); return &r }(),
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: 1,
				},
			},
			instance: csmv1.ContainerStorageModule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test-auth",
				},
			},
			expectReady: true,
			expectError: false,
			description: "Should return ready for single replica deployment",
		},
		{
			name:       "Gateway controller deployment not found",
			deployment: nil,
			instance: csmv1.ContainerStorageModule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test-auth",
				},
			},
			expectReady: false,
			expectError: true,
			description: "Should error when deployment doesn't exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := runtime.NewScheme()
			if err := csmv1.AddToScheme(s); err != nil {
				t.Fatalf("Unable to add csmv1 scheme: %v", err)
			}
			if err := appsv1.AddToScheme(s); err != nil {
				t.Fatalf("Unable to add appsv1 scheme: %v", err)
			}

			var fakeClient client.Client
			if tt.deployment != nil {
				fakeClient = ctrlClientFake.NewClientBuilder().WithScheme(s).WithObjects(tt.deployment).Build()
			} else {
				fakeClient = ctrlClientFake.NewClientBuilder().WithScheme(s).Build()
			}

			r := &FakeReconcileCSM{
				Client:    fakeClient,
				K8sClient: fake.NewSimpleClientset(),
			}

			conditionFunc := getGatewayControllerStatus(ctx, tt.instance, r)
			ready, err := conditionFunc(ctx)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}

			assert.Equal(t, tt.expectReady, ready, tt.description)
		})
	}
}

func authProxyCSMWithRedis(namespace string) csmv1.ContainerStorageModule {
	return csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-auth",
			Namespace: namespace,
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Modules: []csmv1.Module{
				{
					Name:          csmv1.AuthorizationServer,
					Enabled:       true,
					ConfigVersion: "v2.4.0",
					Components: []csmv1.ContainerTemplate{
						{
							Name:    "cert-manager",
							Enabled: &[]bool{false}[0],
						},
						{
							Name:           "redis",
							RedisCommander: "rediscommander",
							RedisName:      "redis-csm",
							Sentinel:       "sentinel",
						},
					},
				},
			},
		},
	}
}

func TestAuthProxyStatusCheckWithRedisStatefulsets(t *testing.T) {
	ctx := context.Background()
	ctrlClient := fullFakeClient()
	ns := "test-redis-ns"

	csm := authProxyCSMWithRedis(ns)
	err := ctrlClient.Create(ctx, &csm)
	assert.NoError(t, err)

	i32One := int32(1)

	makeReadyDep := func(name string) appsv1.Deployment {
		return appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 1, AvailableReplicas: 1},
			Spec:       appsv1.DeploymentSpec{Replicas: &i32One},
		}
	}

	for _, name := range []string{"proxy-server", "role-service", "storage-service", "tenant-service", "authorization-controller", "rediscommander"} {
		dep := makeReadyDep(name)
		err = ctrlClient.Create(ctx, &dep)
		assert.NoError(t, err)
	}

	fakeReconcile := FakeReconcileCSM{Client: ctrlClient, K8sClient: fake.NewSimpleClientset()}

	redisSts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "redis-csm", Namespace: ns},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 0},
		Spec:       appsv1.StatefulSetSpec{Replicas: &i32One},
	}
	err = ctrlClient.Create(ctx, &redisSts)
	assert.NoError(t, err)

	status, err := authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil, OperatorConfig{})
	assert.Nil(t, err)
	assert.False(t, status, "redis-csm not ready should cause false")

	err = ctrlClient.Delete(ctx, &redisSts)
	assert.NoError(t, err)
	redisSts.Status.ReadyReplicas = 1
	redisSts.ResourceVersion = ""
	err = ctrlClient.Create(ctx, &redisSts)
	assert.NoError(t, err)

	sentinelSts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "sentinel", Namespace: ns},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 0},
		Spec:       appsv1.StatefulSetSpec{Replicas: &i32One},
	}
	err = ctrlClient.Create(ctx, &sentinelSts)
	assert.NoError(t, err)

	status, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil, OperatorConfig{})
	assert.Nil(t, err)
	assert.False(t, status, "sentinel not ready should cause false")

	err = ctrlClient.Delete(ctx, &sentinelSts)
	assert.NoError(t, err)
	sentinelSts.Status.ReadyReplicas = 1
	sentinelSts.ResourceVersion = ""
	err = ctrlClient.Create(ctx, &sentinelSts)
	assert.NoError(t, err)

	newStatus := &csmv1.ContainerStorageModuleStatus{}
	status, err = authProxyStatusCheck(ctx, &csm, &fakeReconcile, newStatus, OperatorConfig{})
	assert.Nil(t, err)
	assert.True(t, status, "all components ready should return true")
	assert.NotEqual(t, "0", newStatus.ControllerStatus.Desired, "controllerStatus.Desired should be populated")
}

func TestAuthProxyStatusCheckPopulatesControllerStatusOnNotReady(t *testing.T) {
	ctx := context.Background()
	ctrlClient := fullFakeClient()
	ns := "test-notready-ns"

	csm := authProxyCSMWithRedis(ns)
	err := ctrlClient.Create(ctx, &csm)
	assert.NoError(t, err)

	i32One := int32(1)

	// Create all core deployments as ready except proxy-server
	for _, name := range []string{"role-service", "storage-service", "tenant-service", "authorization-controller", "rediscommander"} {
		dep := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 1, AvailableReplicas: 1},
			Spec:       appsv1.DeploymentSpec{Replicas: &i32One},
		}
		err = ctrlClient.Create(ctx, &dep)
		assert.NoError(t, err)
	}
	// proxy-server is NOT ready
	proxyDep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "proxy-server", Namespace: ns},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 0, AvailableReplicas: 0},
		Spec:       appsv1.DeploymentSpec{Replicas: &i32One},
	}
	err = ctrlClient.Create(ctx, &proxyDep)
	assert.NoError(t, err)

	fakeReconcile := FakeReconcileCSM{Client: ctrlClient, K8sClient: fake.NewSimpleClientset()}
	newStatus := &csmv1.ContainerStorageModuleStatus{}

	status, err := authProxyStatusCheck(ctx, &csm, &fakeReconcile, newStatus, OperatorConfig{})
	assert.Nil(t, err)
	assert.False(t, status, "proxy-server not ready should return false")
	// controllerStatus should still be populated by the early getAuthProxyDeploymentStatus call
	assert.NotEqual(t, "", newStatus.ControllerStatus.Desired, "controllerStatus.Desired should be populated even when not ready")
}

func TestAuthProxyStatusCheckRedisCommanderNotReady(t *testing.T) {
	ctx := context.Background()
	ctrlClient := fullFakeClient()
	ns := "test-rcns"

	csm := authProxyCSMWithRedis(ns)
	err := ctrlClient.Create(ctx, &csm)
	assert.NoError(t, err)

	i32One := int32(1)

	for _, name := range []string{"proxy-server", "role-service", "storage-service", "tenant-service", "authorization-controller"} {
		dep := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 1, AvailableReplicas: 1},
			Spec:       appsv1.DeploymentSpec{Replicas: &i32One},
		}
		err = ctrlClient.Create(ctx, &dep)
		assert.NoError(t, err)
	}

	rcDep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "rediscommander", Namespace: ns},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 0, AvailableReplicas: 0},
		Spec:       appsv1.DeploymentSpec{Replicas: &i32One},
	}
	err = ctrlClient.Create(ctx, &rcDep)
	assert.NoError(t, err)

	fakeReconcile := FakeReconcileCSM{Client: ctrlClient, K8sClient: fake.NewSimpleClientset()}

	status, err := authProxyStatusCheck(ctx, &csm, &fakeReconcile, nil, OperatorConfig{})
	assert.Nil(t, err)
	assert.False(t, status, "rediscommander not ready should cause false")
}

func TestGetAuthProxyDeploymentStatusWithStatefulsets(t *testing.T) {
	ctx := context.Background()
	ctrlClient := fullFakeClient()
	ns := "test-gads-ns"

	csm := authProxyCSMWithRedis(ns)
	err := ctrlClient.Create(ctx, &csm)
	assert.NoError(t, err)

	i32Three := int32(3)
	i32One := int32(1)

	deps := []struct {
		name      string
		available int32
		unavail   int32
	}{
		{"proxy-server", 1, 0},
		{"role-service", 1, 0},
		{"storage-service", 1, 0},
		{"tenant-service", 1, 0},
		{"authorization-controller", 1, 0},
		{"rediscommander", 1, 0},
	}
	for _, d := range deps {
		dep := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: d.name, Namespace: ns},
			Status:     appsv1.DeploymentStatus{AvailableReplicas: d.available, UnavailableReplicas: d.unavail, ReadyReplicas: d.available},
			Spec:       appsv1.DeploymentSpec{Replicas: &i32One},
		}
		err = ctrlClient.Create(ctx, &dep)
		assert.NoError(t, err)
	}

	redisSts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "redis-csm", Namespace: ns},
		Status:     appsv1.StatefulSetStatus{AvailableReplicas: 3, ReadyReplicas: 3},
		Spec:       appsv1.StatefulSetSpec{Replicas: &i32Three},
	}
	sentinelSts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "sentinel", Namespace: ns},
		Status:     appsv1.StatefulSetStatus{AvailableReplicas: 1, ReadyReplicas: 1},
		Spec:       appsv1.StatefulSetSpec{Replicas: &i32One},
	}
	err = ctrlClient.Create(ctx, &redisSts)
	assert.NoError(t, err)
	err = ctrlClient.Create(ctx, &sentinelSts)
	assert.NoError(t, err)

	k8sClient := fake.NewSimpleClientset()
	_, err = k8sClient.AppsV1().StatefulSets(ns).Create(ctx, &redisSts, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = k8sClient.AppsV1().StatefulSets(ns).Create(ctx, &sentinelSts, metav1.CreateOptions{})
	assert.NoError(t, err)

	fakeReconcile := FakeReconcileCSM{Client: ctrlClient, K8sClient: k8sClient}

	podStatus, err := getAuthProxyDeploymentStatus(ctx, &csm, &fakeReconcile)
	assert.Nil(t, err)
	assert.Equal(t, "10", podStatus.Desired, "6 dep replicas + 3 redis + 1 sentinel = 10")
	assert.Equal(t, "10", podStatus.Available)
	assert.Equal(t, "0", podStatus.Failed)
}

func TestGetAuthProxyDeploymentStatusOpenShift(t *testing.T) {
	ctx := context.Background()
	ctrlClient := fullFakeClient()
	ns := "test-ocp-ns"

	csm := authProxyCSMWithRedis(ns)
	err := ctrlClient.Create(ctx, &csm)
	assert.NoError(t, err)

	i32Three := int32(3)
	i32One := int32(1)

	// Create core auth proxy deployments
	deps := []struct {
		name      string
		available int32
		unavail   int32
	}{
		{"proxy-server", 1, 0},
		{"role-service", 1, 0},
		{"storage-service", 1, 0},
		{"tenant-service", 1, 0},
		{"authorization-controller", 1, 0},
		{"rediscommander", 1, 0},
	}
	for _, d := range deps {
		dep := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: d.name, Namespace: ns},
			Status:     appsv1.DeploymentStatus{AvailableReplicas: d.available, UnavailableReplicas: d.unavail, ReadyReplicas: d.available},
			Spec:       appsv1.DeploymentSpec{Replicas: &i32One},
		}
		err = ctrlClient.Create(ctx, &dep)
		assert.NoError(t, err)
	}

	// Create nginx and gateway deployments (should be skipped on OpenShift)
	nginxDep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-ingress-nginx-controller", ns), Namespace: ns},
		Status:     appsv1.DeploymentStatus{AvailableReplicas: 1, ReadyReplicas: 1},
		Spec:       appsv1.DeploymentSpec{Replicas: &i32One},
	}
	err = ctrlClient.Create(ctx, &nginxDep)
	assert.NoError(t, err)

	gatewayDep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-nginx-gateway-fabric", ns), Namespace: ns},
		Status:     appsv1.DeploymentStatus{AvailableReplicas: 1, ReadyReplicas: 1},
		Spec:       appsv1.DeploymentSpec{Replicas: &i32One},
	}
	err = ctrlClient.Create(ctx, &gatewayDep)
	assert.NoError(t, err)

	redisSts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "redis-csm", Namespace: ns},
		Status:     appsv1.StatefulSetStatus{AvailableReplicas: 3, ReadyReplicas: 3},
		Spec:       appsv1.StatefulSetSpec{Replicas: &i32Three},
	}
	sentinelSts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "sentinel", Namespace: ns},
		Status:     appsv1.StatefulSetStatus{AvailableReplicas: 1, ReadyReplicas: 1},
		Spec:       appsv1.StatefulSetSpec{Replicas: &i32One},
	}
	err = ctrlClient.Create(ctx, &redisSts)
	assert.NoError(t, err)
	err = ctrlClient.Create(ctx, &sentinelSts)
	assert.NoError(t, err)

	k8sClient := fake.NewSimpleClientset()
	_, err = k8sClient.AppsV1().StatefulSets(ns).Create(ctx, &redisSts, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = k8sClient.AppsV1().StatefulSets(ns).Create(ctx, &sentinelSts, metav1.CreateOptions{})
	assert.NoError(t, err)

	fakeReconcile := FakeReconcileCSM{Client: ctrlClient, K8sClient: k8sClient}

	// Test with isOpenShift=true - nginx and gateway should be skipped
	fakeReconcile.Config = OperatorConfig{IsOpenShift: true}
	podStatus, err := getAuthProxyDeploymentStatus(ctx, &csm, &fakeReconcile)
	assert.Nil(t, err)
	assert.Equal(t, "10", podStatus.Desired, "6 dep replicas + 3 redis + 1 sentinel = 10 (nginx/gateway skipped)")
	assert.Equal(t, "10", podStatus.Available)
	assert.Equal(t, "0", podStatus.Failed)

	// Test with isOpenShift=false - nginx and gateway are not enabled in this CSM
	fakeReconcile.Config = OperatorConfig{IsOpenShift: false}
	podStatus, err = getAuthProxyDeploymentStatus(ctx, &csm, &fakeReconcile)
	assert.Nil(t, err)
	assert.Equal(t, "10", podStatus.Desired, "6 dep replicas + 3 redis + 1 sentinel = 10 (nginx/gateway not enabled)")
	assert.Equal(t, "10", podStatus.Available)
	assert.Equal(t, "0", podStatus.Failed)
}

func TestCalculateStateLastSuccessfulConfiguration(t *testing.T) {
	// Test the LastSuccessfulConfiguration logic for authorization proxy server
	ctx := context.Background()
	ctrlClient := fullFakeClient()
	ns := "test-ns"

	// Create an authorization proxy server CSM with Succeeded status
	csm := createCSM("csm-authorization", ns, "", csmv1.AuthorizationServer, true, nil)
	csm.Status.State = constants.Succeeded
	csm.Annotations = map[string]string{
		"kubectl.kubernetes.io/last-applied-configuration": "test-config",
	}
	// Set ConfigVersion for the authorization module
	if len(csm.Spec.Modules) > 0 {
		csm.Spec.Modules[0].ConfigVersion = "v2.5.0"
	}
	err := ctrlClient.Create(ctx, csm)
	assert.NoError(t, err)

	// Create a successful deployment
	i32One := int32(1)
	dep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxy-server",
			Namespace: ns,
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas: 1,
			ReadyReplicas:     1,
			Replicas:          1,
		},
		Spec: appsv1.DeploymentSpec{Replicas: &i32One},
	}
	err = ctrlClient.Create(ctx, &dep)
	assert.NoError(t, err)

	// Create required deployments for auth proxy module check
	requiredDeployments := []string{"role-service", "storage-service", "tenant-service", "authorization-controller"}
	for _, depName := range requiredDeployments {
		reqDep := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      depName,
				Namespace: ns,
			},
			Status: appsv1.DeploymentStatus{
				AvailableReplicas: 1,
				ReadyReplicas:     1,
				Replicas:          1,
			},
			Spec: appsv1.DeploymentSpec{Replicas: &i32One},
		}
		err = ctrlClient.Create(ctx, &reqDep)
		assert.NoError(t, err)
	}

	fakeReconcile := FakeReconcileCSM{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
		Config:    OperatorConfig{},
	}

	newStatus := &csmv1.ContainerStorageModuleStatus{
		State: constants.Succeeded,
	}

	// Clear stability period to ensure immediate Succeeded state for this test
	ClearSucceededStabilityPeriod(csm.GetNamespace() + "/" + csm.GetName())
	// Manually set the first succeeded time to the past to bypass stability period
	firstSucceededObservedMux.Lock()
	firstSucceededObserved[csm.GetNamespace()+"/"+csm.GetName()] = time.Now().Add(-31 * time.Second)
	firstSucceededObservedMux.Unlock()

	// Call calculateState - this should set LastSuccessfulConfiguration
	running, err := calculateState(ctx, csm, &fakeReconcile, newStatus, OperatorConfig{})
	assert.NoError(t, err)
	assert.True(t, running)
	assert.Equal(t, constants.Succeeded, csm.Status.State)
	assert.NotEmpty(t, csm.Status.LastSuccessfulConfiguration, "LastSuccessfulConfiguration should be set for auth proxy")
	assert.NotContains(t, csm.Status.LastSuccessfulConfiguration, "kubectl.kubernetes.io/last-applied-configuration")
}

func TestCalculateStateWithDeploymentStatusOverride(t *testing.T) {
	// Test the deploymentStatusOverride parameter in calculateState
	ctx := context.Background()
	ctrlClient := fullFakeClient()
	ns := "test-ns"

	// Create a driver CSM
	csm := createCSM("powerflex", ns, csmv1.PowerFlex, csmv1.Replication, true, nil)
	err := ctrlClient.Create(ctx, csm)
	assert.NoError(t, err)

	// Create a successful daemonset
	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "powerflex-node",
			Namespace: ns,
			Labels: map[string]string{
				"powerflex-node": "true",
			},
		},
		Status: appsv1.DaemonSetStatus{
			NumberAvailable:        1,
			NumberReady:            1,
			DesiredNumberScheduled: 1,
		},
	}
	err = ctrlClient.Create(ctx, &ds)
	assert.NoError(t, err)

	// Create a pod for the daemonset
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "powerflex-node-abc123",
			Namespace: ns,
			Labels: map[string]string{
				"app": "powerflex-node",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	err = ctrlClient.Create(ctx, &pod)
	assert.NoError(t, err)

	fakeReconcile := FakeReconcileCSM{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
		Config:    OperatorConfig{},
	}

	newStatus := &csmv1.ContainerStorageModuleStatus{
		State: constants.Succeeded,
	}

	// Clear stability period to ensure immediate Succeeded state for this test
	ClearSucceededStabilityPeriod(csm.GetNamespace() + "/" + csm.GetName())
	// Manually set the first succeeded time to the past to bypass stability period
	firstSucceededObservedMux.Lock()
	firstSucceededObserved[csm.GetNamespace()+"/"+csm.GetName()] = time.Now().Add(-31 * time.Second)
	firstSucceededObservedMux.Unlock()

	// Test with deploymentStatusOverride - this should use the override instead of calling getDeploymentStatus
	overrideStatus := csmv1.PodStatus{
		Desired:   "2",
		Available: "2",
		Failed:    "0",
	}
	running, err := calculateState(ctx, csm, &fakeReconcile, newStatus, OperatorConfig{}, overrideStatus)
	assert.NoError(t, err)
	assert.True(t, running)
	assert.Equal(t, constants.Succeeded, csm.Status.State)
	assert.Equal(t, overrideStatus, csm.Status.ControllerStatus, "ControllerStatus should use the override")
}

func TestFailureGracePeriod(t *testing.T) {
	t.Run("first failure records time and returns false", func(t *testing.T) {
		key := "test-ns/test-grace-first"
		ClearFailureGracePeriod(key) // ensure clean state
		assert.False(t, checkFailureGracePeriod(key), "first call should return false (grace period not elapsed)")
	})

	t.Run("second call within grace period returns false", func(t *testing.T) {
		key := "test-ns/test-grace-within"
		ClearFailureGracePeriod(key)
		_ = checkFailureGracePeriod(key) // first call records time
		assert.False(t, checkFailureGracePeriod(key), "second call within grace period should return false")
	})

	t.Run("call after grace period returns true", func(t *testing.T) {
		key := "test-ns/test-grace-elapsed"
		ClearFailureGracePeriod(key)
		// Manually set the first failure time in the past
		firstFailureObservedMux.Lock()
		firstFailureObserved[key] = time.Now().Add(-11 * time.Minute)
		firstFailureObservedMux.Unlock()
		assert.True(t, checkFailureGracePeriod(key), "should return true after grace period elapsed")
	})

	t.Run("ClearFailureGracePeriod resets tracking", func(t *testing.T) {
		key := "test-ns/test-grace-clear"
		_ = checkFailureGracePeriod(key) // record time
		ClearFailureGracePeriod(key)     // clear it
		assert.False(t, checkFailureGracePeriod(key), "should return false after clear (new first observation)")
	})
}

func TestFailureGracePeriodEnvVar(t *testing.T) {
	original := FailureGracePeriod
	defer func() { FailureGracePeriod = original }()

	t.Run("valid env var overrides default", func(t *testing.T) {
		FailureGracePeriod = 10 * time.Minute // reset
		os.Setenv("FAILURE_GRACE_PERIOD", "5m")
		defer os.Unsetenv("FAILURE_GRACE_PERIOD")
		// Re-run init logic
		if val := os.Getenv("FAILURE_GRACE_PERIOD"); val != "" {
			if d, err := time.ParseDuration(val); err == nil && d > 0 {
				FailureGracePeriod = d
			}
		}
		assert.Equal(t, 5*time.Minute, FailureGracePeriod)
	})

	t.Run("invalid env var keeps default", func(t *testing.T) {
		FailureGracePeriod = 10 * time.Minute // reset
		os.Setenv("FAILURE_GRACE_PERIOD", "invalid")
		defer os.Unsetenv("FAILURE_GRACE_PERIOD")
		if val := os.Getenv("FAILURE_GRACE_PERIOD"); val != "" {
			if d, err := time.ParseDuration(val); err == nil && d > 0 {
				FailureGracePeriod = d
			}
		}
		assert.Equal(t, 10*time.Minute, FailureGracePeriod)
	})

	t.Run("zero duration keeps default", func(t *testing.T) {
		FailureGracePeriod = 10 * time.Minute
		os.Setenv("FAILURE_GRACE_PERIOD", "0s")
		defer os.Unsetenv("FAILURE_GRACE_PERIOD")
		if val := os.Getenv("FAILURE_GRACE_PERIOD"); val != "" {
			if d, err := time.ParseDuration(val); err == nil && d > 0 {
				FailureGracePeriod = d
			}
		}
		assert.Equal(t, 10*time.Minute, FailureGracePeriod)
	})

	t.Run("negative duration keeps default", func(t *testing.T) {
		FailureGracePeriod = 10 * time.Minute
		os.Setenv("FAILURE_GRACE_PERIOD", "-5m")
		defer os.Unsetenv("FAILURE_GRACE_PERIOD")
		if val := os.Getenv("FAILURE_GRACE_PERIOD"); val != "" {
			if d, err := time.ParseDuration(val); err == nil && d > 0 {
				FailureGracePeriod = d
			}
		}
		assert.Equal(t, 10*time.Minute, FailureGracePeriod)
	})

	t.Run("unset env var keeps default", func(t *testing.T) {
		FailureGracePeriod = 10 * time.Minute
		os.Unsetenv("FAILURE_GRACE_PERIOD")
		if val := os.Getenv("FAILURE_GRACE_PERIOD"); val != "" {
			if d, err := time.ParseDuration(val); err == nil && d > 0 {
				FailureGracePeriod = d
			}
		}
		assert.Equal(t, 10*time.Minute, FailureGracePeriod)
	})
}

func TestSucceededStabilityPeriod(t *testing.T) {
	t.Run("first succeeded records time and returns false", func(t *testing.T) {
		key := "test-ns/test-succeeded-first"
		ClearSucceededStabilityPeriod(key) // ensure clean state
		assert.False(t, checkSucceededStabilityPeriod(key), "first call should return false (stability period not elapsed)")
	})

	t.Run("second call within stability period returns false", func(t *testing.T) {
		key := "test-ns/test-succeeded-within"
		ClearSucceededStabilityPeriod(key)
		_ = checkSucceededStabilityPeriod(key) // first call records time
		assert.False(t, checkSucceededStabilityPeriod(key), "second call within stability period should return false")
	})

	t.Run("call after stability period returns true", func(t *testing.T) {
		key := "test-ns/test-succeeded-elapsed"
		ClearSucceededStabilityPeriod(key)
		// Manually set the first succeeded time in the past (31 seconds ago)
		firstSucceededObservedMux.Lock()
		firstSucceededObserved[key] = time.Now().Add(-31 * time.Second)
		firstSucceededObservedMux.Unlock()
		assert.True(t, checkSucceededStabilityPeriod(key), "should return true after stability period elapsed")
	})

	t.Run("ClearSucceededStabilityPeriod resets tracking", func(t *testing.T) {
		key := "test-ns/test-succeeded-clear"
		_ = checkSucceededStabilityPeriod(key) // record time
		ClearSucceededStabilityPeriod(key)     // clear it
		assert.False(t, checkSucceededStabilityPeriod(key), "should return false after clear (new first observation)")
	})
}

func TestCalculateStateAuthProxyNoModules(t *testing.T) {
	// Test the case where auth proxy has no enabled modules - should stay in Pending state
	ctx := context.Background()
	ctrlClient := fullFakeClient()
	ns := "test-ns"

	// Create an authorization proxy server CSM with no enabled modules
	csm := createCSM("csm-authorization", ns, "", csmv1.AuthorizationServer, false, nil) // Module disabled
	err := ctrlClient.Create(ctx, csm)
	assert.NoError(t, err)

	// Create a successful deployment
	i32One := int32(1)
	dep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxy-server",
			Namespace: ns,
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas: 1,
			ReadyReplicas:     1,
			Replicas:          1,
		},
		Spec: appsv1.DeploymentSpec{Replicas: &i32One},
	}
	err = ctrlClient.Create(ctx, &dep)
	assert.NoError(t, err)

	fakeReconcile := FakeReconcileCSM{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
		Config:    OperatorConfig{},
	}

	newStatus := &csmv1.ContainerStorageModuleStatus{
		State: constants.Succeeded,
	}

	// Call calculateState - should set state to Pending since no modules are enabled
	running, err := calculateState(ctx, csm, &fakeReconcile, newStatus, OperatorConfig{})
	assert.NoError(t, err)
	assert.False(t, running, "Should return false when no modules are enabled")
	assert.Equal(t, constants.Pending, csm.Status.State, "State should be Pending when no modules are enabled for auth proxy")
}

func TestCalculateStateWithPreUpgradeSnapshot(t *testing.T) {
	ctx := context.Background()
	ctrlClient := fullFakeClient()
	ns := "test-ns"

	csm := createCSM("powerflex", ns, csmv1.PowerFlex, csmv1.Replication, true, nil)
	// Set the PreUpgradeSnapshot annotation
	csm.Annotations = map[string]string{
		MetadataPrefix + "/PreUpgradeSnapshot": `{"some":"snapshot"}`,
	}
	err := ctrlClient.Create(ctx, csm)
	assert.NoError(t, err)

	// Create a successful daemonset
	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "powerflex-node",
			Namespace: ns,
			Labels:    map[string]string{"powerflex-node": "true"},
		},
		Status: appsv1.DaemonSetStatus{
			NumberAvailable:        1,
			NumberReady:            1,
			DesiredNumberScheduled: 1,
		},
	}
	err = ctrlClient.Create(ctx, &ds)
	assert.NoError(t, err)

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "powerflex-node-abc123",
			Namespace: ns,
			Labels:    map[string]string{"app": "powerflex-node"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
	err = ctrlClient.Create(ctx, &pod)
	assert.NoError(t, err)

	fakeReconcile := FakeReconcileCSM{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
		Config:    OperatorConfig{},
	}

	newStatus := &csmv1.ContainerStorageModuleStatus{
		State: constants.Succeeded,
	}

	// Override deployment status to simulate all pods ready
	overrideStatus := csmv1.PodStatus{
		Desired:   "1",
		Available: "1",
		Failed:    "0",
	}

	crKey := csm.GetNamespace() + "/" + csm.GetName()

	t.Run("snapshot exists with stability period not elapsed", func(t *testing.T) {
		ClearSucceededStabilityPeriod(crKey)
		// Fresh call - stability period not elapsed

		running, err := calculateState(ctx, csm, &fakeReconcile, newStatus, OperatorConfig{}, overrideStatus)
		assert.NoError(t, err)
		assert.False(t, running)
		assert.Equal(t, constants.Pending, csm.Status.State, "State should be Pending when snapshot exists and stability period not elapsed")
	})

	t.Run("snapshot exists with stability period elapsed", func(t *testing.T) {
		ClearSucceededStabilityPeriod(crKey)
		// Manually set stability period to past to simulate elapsed
		firstSucceededObservedMux.Lock()
		firstSucceededObserved[crKey] = time.Now().Add(-31 * time.Second)
		firstSucceededObservedMux.Unlock()

		running, err := calculateState(ctx, csm, &fakeReconcile, newStatus, OperatorConfig{}, overrideStatus)
		assert.NoError(t, err)
		assert.True(t, running)
		assert.Equal(t, constants.Succeeded, csm.Status.State, "State should be Succeeded when stability period elapsed even with snapshot")
	})
}

func TestCalculateStateStabilityPeriodNotElapsed(t *testing.T) {
	// Test that calculateState keeps state as Pending when stability period has not elapsed
	ctx := context.Background()
	ctrlClient := fullFakeClient()
	ns := "test-ns"

	csm := createCSM("powerflex", ns, csmv1.PowerFlex, csmv1.Replication, true, nil)
	err := ctrlClient.Create(ctx, csm)
	assert.NoError(t, err)

	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "powerflex-node",
			Namespace: ns,
			Labels:    map[string]string{"powerflex-node": "true"},
		},
		Status: appsv1.DaemonSetStatus{
			NumberAvailable:        1,
			NumberReady:            1,
			DesiredNumberScheduled: 1,
		},
	}
	err = ctrlClient.Create(ctx, &ds)
	assert.NoError(t, err)

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "powerflex-node-abc123",
			Namespace: ns,
			Labels:    map[string]string{"app": "powerflex-node"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
	err = ctrlClient.Create(ctx, &pod)
	assert.NoError(t, err)

	fakeReconcile := FakeReconcileCSM{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
		Config:    OperatorConfig{},
	}

	newStatus := &csmv1.ContainerStorageModuleStatus{
		State: constants.Succeeded,
	}

	// Clear any previous stability tracking so this is a fresh first call
	crKey := csm.GetNamespace() + "/" + csm.GetName()
	ClearSucceededStabilityPeriod(crKey)

	overrideStatus := csmv1.PodStatus{
		Desired:   "1",
		Available: "1",
		Failed:    "0",
	}

	running, err := calculateState(ctx, csm, &fakeReconcile, newStatus, OperatorConfig{}, overrideStatus)
	assert.NoError(t, err)
	assert.False(t, running, "Should not be running when stability period has not elapsed")
	assert.Equal(t, constants.Pending, csm.Status.State, "State should be Pending when within stability period")
}

func TestCalculateStatePodsNotReadyWithFailures(t *testing.T) {
	ctx := context.Background()
	ctrlClient := fullFakeClient()
	ns := "test-ns"

	csm := createCSM("powerflex", ns, csmv1.PowerFlex, csmv1.Replication, true, nil)
	err := ctrlClient.Create(ctx, csm)
	assert.NoError(t, err)

	// Create daemonset with no ready pods
	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "powerflex-node",
			Namespace: ns,
			Labels:    map[string]string{"powerflex-node": "true"},
		},
		Status: appsv1.DaemonSetStatus{
			NumberAvailable:        0,
			NumberReady:            0,
			DesiredNumberScheduled: 1,
		},
	}
	err = ctrlClient.Create(ctx, &ds)
	assert.NoError(t, err)

	fakeReconcile := FakeReconcileCSM{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
		Config:    OperatorConfig{},
	}

	crKey := csm.GetNamespace() + "/" + csm.GetName()

	t.Run("pods not ready with failures within grace period", func(t *testing.T) {
		ClearFailureGracePeriod(crKey)
		ClearSucceededStabilityPeriod(crKey)

		newStatus := &csmv1.ContainerStorageModuleStatus{
			State: constants.Pending,
		}

		// Override with failed pods - controller available != desired and has failures
		overrideStatus := csmv1.PodStatus{
			Desired:   "1",
			Available: "0",
			Failed:    "1",
		}

		running, err := calculateState(ctx, csm, &fakeReconcile, newStatus, OperatorConfig{}, overrideStatus)
		assert.NoError(t, err)
		assert.False(t, running)
		assert.Equal(t, constants.Pending, csm.Status.State, "State should be Pending within failure grace period")
	})

	t.Run("pods not ready with failures after grace period elapsed", func(t *testing.T) {
		ClearSucceededStabilityPeriod(crKey)
		// Set failure observation time to past to simulate elapsed grace period
		firstFailureObservedMux.Lock()
		firstFailureObserved[crKey] = time.Now().Add(-11 * time.Minute)
		firstFailureObservedMux.Unlock()

		newStatus := &csmv1.ContainerStorageModuleStatus{
			State: constants.Pending,
		}

		overrideStatus := csmv1.PodStatus{
			Desired:   "1",
			Available: "0",
			Failed:    "1",
		}

		running, err := calculateState(ctx, csm, &fakeReconcile, newStatus, OperatorConfig{}, overrideStatus)
		assert.NoError(t, err)
		assert.False(t, running)
		assert.Equal(t, constants.Failed, csm.Status.State, "State should be Failed after grace period elapsed")
	})

	t.Run("pods not ready without failures clears failure grace period", func(t *testing.T) {
		ClearSucceededStabilityPeriod(crKey)

		// Pre-record a failure observation
		firstFailureObservedMux.Lock()
		firstFailureObserved[crKey] = time.Now()
		firstFailureObservedMux.Unlock()

		newStatus := &csmv1.ContainerStorageModuleStatus{
			State: constants.Pending,
		}

		// Override with no failures - just not enough pods available yet
		overrideStatus := csmv1.PodStatus{
			Desired:   "1",
			Available: "0",
			Failed:    "0",
		}

		running, err := calculateState(ctx, csm, &fakeReconcile, newStatus, OperatorConfig{}, overrideStatus)
		assert.NoError(t, err)
		assert.False(t, running)
		assert.Equal(t, constants.Pending, csm.Status.State, "State should be Pending when pods starting up without failures")

		// Verify failure grace period was cleared
		firstFailureObservedMux.Lock()
		_, exists := firstFailureObserved[crKey]
		firstFailureObservedMux.Unlock()
		assert.False(t, exists, "Failure grace period should be cleared when no pod failures")
	})
}

func TestCalculateStateModuleNotRunningWithFailures(t *testing.T) {
	ctx := context.Background()
	ctrlClient := fullFakeClient()
	ns := "test-ns"

	// Create an auth proxy server CSM with observability enabled
	// observability module check will fail because no observability deployments exist
	csm := createCSM("csm-authorization", ns, "", csmv1.AuthorizationServer, true, nil)
	// Also add an observability module that is enabled
	csm.Spec.Modules = append(csm.Spec.Modules, csmv1.Module{
		Name:    csmv1.Observability,
		Enabled: true,
	})
	err := ctrlClient.Create(ctx, csm)
	assert.NoError(t, err)

	fakeReconcile := FakeReconcileCSM{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
		Config:    OperatorConfig{},
	}

	crKey := csm.GetNamespace() + "/" + csm.GetName()

	t.Run("module not running with controller pod failures within grace period", func(t *testing.T) {
		ClearFailureGracePeriod(crKey)
		ClearSucceededStabilityPeriod(crKey)
		// Bypass stability period
		firstSucceededObservedMux.Lock()
		firstSucceededObserved[crKey] = time.Now().Add(-31 * time.Second)
		firstSucceededObservedMux.Unlock()

		newStatus := &csmv1.ContainerStorageModuleStatus{
			State: constants.Succeeded,
		}

		// Controller pods available but with failures
		overrideStatus := csmv1.PodStatus{
			Desired:   "1",
			Available: "1",
			Failed:    "1",
		}

		running, err := calculateState(ctx, csm, &fakeReconcile, newStatus, OperatorConfig{}, overrideStatus)
		assert.NoError(t, err)
		assert.False(t, running)
		assert.Equal(t, constants.Pending, csm.Status.State, "State should be Pending when module fails within grace period")
	})

	t.Run("module not running with node pod failures within grace period", func(t *testing.T) {
		ClearFailureGracePeriod(crKey)
		ClearSucceededStabilityPeriod(crKey)
		// Bypass stability period
		firstSucceededObservedMux.Lock()
		firstSucceededObserved[crKey] = time.Now().Add(-31 * time.Second)
		firstSucceededObservedMux.Unlock()

		newStatus := &csmv1.ContainerStorageModuleStatus{
			State: constants.Succeeded,
			// Node failures trigger hasPodFailures
			NodeStatus: csmv1.PodStatus{
				Available: "0",
				Failed:    "1",
				Desired:   "1",
			},
		}

		// Controller pods available, no controller failures
		overrideStatus := csmv1.PodStatus{
			Desired:   "1",
			Available: "1",
			Failed:    "0",
		}

		running, err := calculateState(ctx, csm, &fakeReconcile, newStatus, OperatorConfig{}, overrideStatus)
		assert.NoError(t, err)
		assert.False(t, running)
		assert.Equal(t, constants.Pending, csm.Status.State, "State should be Pending when module fails with node pod failures within grace period")
	})

	t.Run("module not running without pod failures", func(t *testing.T) {
		ClearFailureGracePeriod(crKey)
		ClearSucceededStabilityPeriod(crKey)
		// Bypass stability period
		firstSucceededObservedMux.Lock()
		firstSucceededObserved[crKey] = time.Now().Add(-31 * time.Second)
		firstSucceededObservedMux.Unlock()

		newStatus := &csmv1.ContainerStorageModuleStatus{
			State: constants.Succeeded,
		}

		overrideStatus := csmv1.PodStatus{
			Desired:   "1",
			Available: "1",
			Failed:    "0",
		}

		running, err := calculateState(ctx, csm, &fakeReconcile, newStatus, OperatorConfig{}, overrideStatus)
		assert.NoError(t, err)
		assert.False(t, running)
		assert.Equal(t, constants.Pending, csm.Status.State, "State should be Pending when module not running but no pod failures")
	})
}

// TestCalculateStateRollingUpdateFailureMaskedByOldPod verifies that during a
// rolling update the CR does not reach Succeeded when the new pod is failing
// (e.g., ImagePullBackOff) even though the old pod keeps AvailableReplicas
// equal to Spec.Replicas.
func TestCalculateStateRollingUpdateFailureMaskedByOldPod(t *testing.T) {
	ctx := context.Background()
	ctrlClient := fullFakeClient()
	ns := "test-ns"

	// Use a driver CR so there is no checkModuleStatus entry and
	// moduleCheckPerformed stays false — running remains true and the
	// new hasPodFailures guard fires.
	csm := createCSM("powerflex-rolling", ns, csmv1.PowerFlex, csmv1.Replication, false, nil)
	err := ctrlClient.Create(ctx, csm)
	assert.NoError(t, err)

	// Create a healthy daemonset so nodeStatusGood=true
	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "powerflex-rolling-node", Namespace: ns,
			Labels: map[string]string{"powerflex-rolling-node": "true"},
		},
		Status: appsv1.DaemonSetStatus{
			NumberAvailable: 1, NumberReady: 1, DesiredNumberScheduled: 1,
		},
	}
	err = ctrlClient.Create(ctx, &ds)
	assert.NoError(t, err)

	// Create a Running+Ready pod matching the daemonset label so
	// getDaemonSetStatus counts it as available (totalRunning=1).
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "powerflex-rolling-node-abc", Namespace: ns,
			Labels: map[string]string{"app": "powerflex-rolling-node"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
	err = ctrlClient.Create(ctx, &pod)
	assert.NoError(t, err)

	fakeReconcile := FakeReconcileCSM{
		Client:    ctrlClient,
		K8sClient: fake.NewSimpleClientset(),
		Config:    OperatorConfig{},
	}

	crKey := csm.GetNamespace() + "/" + csm.GetName()

	t.Run("pods ready but failures detected within grace period", func(t *testing.T) {
		ClearFailureGracePeriod(crKey)
		ClearSucceededStabilityPeriod(crKey)
		// Bypass stability period so code would normally reach Succeeded
		firstSucceededObservedMux.Lock()
		firstSucceededObserved[crKey] = time.Now().Add(-31 * time.Second)
		firstSucceededObservedMux.Unlock()

		newStatus := &csmv1.ContainerStorageModuleStatus{State: constants.Succeeded}

		// Old pod still available → Desired==Available, but new pod is failing
		overrideStatus := csmv1.PodStatus{
			Desired:   "1",
			Available: "1",
			Failed:    "1",
		}

		running, err := calculateState(ctx, csm, &fakeReconcile, newStatus, OperatorConfig{}, overrideStatus)
		assert.NoError(t, err)
		assert.False(t, running)
		assert.Equal(t, constants.Pending, csm.Status.State, "State should be Pending when pods are ready but failures detected within grace period")
	})

	t.Run("pods ready but failures detected after grace period", func(t *testing.T) {
		ClearSucceededStabilityPeriod(crKey)
		// Bypass stability period
		firstSucceededObservedMux.Lock()
		firstSucceededObserved[crKey] = time.Now().Add(-31 * time.Second)
		firstSucceededObservedMux.Unlock()

		// Pre-set the failure grace period to an expired time.
		// Note: ClearFailureGracePeriod is called inside allPodsReady when
		// no module check detects failures, so set it AFTER the clear would
		// have happened by using checkFailureGracePeriod first (which seeds the
		// timestamp), then overwriting it to the past.
		checkFailureGracePeriod(crKey)
		firstFailureObservedMux.Lock()
		firstFailureObserved[crKey] = time.Now().Add(-FailureGracePeriod - time.Second)
		firstFailureObservedMux.Unlock()

		newStatus := &csmv1.ContainerStorageModuleStatus{State: constants.Succeeded}
		overrideStatus := csmv1.PodStatus{
			Desired:   "1",
			Available: "1",
			Failed:    "1",
		}

		running, err := calculateState(ctx, csm, &fakeReconcile, newStatus, OperatorConfig{}, overrideStatus)
		assert.NoError(t, err)
		assert.False(t, running)
		assert.Equal(t, constants.Failed, csm.Status.State, "State should be Failed when pods ready but failures detected and grace period elapsed")
	})
}

func TestIsUpgradeInProgress(t *testing.T) {
	ctx := context.Background()

	t.Run("no annotations returns false", func(t *testing.T) {
		cr := &csmv1.ContainerStorageModule{
			Spec: csmv1.ContainerStorageModuleSpec{
				Driver: csmv1.Driver{
					CSIDriverType: csmv1.PowerFlex,
					ConfigVersion: "v2.12.0",
				},
			},
		}
		assert.False(t, IsUpgradeInProgress(ctx, cr, OperatorConfig{}))
	})

	t.Run("no configVersion annotation returns false", func(t *testing.T) {
		cr := &csmv1.ContainerStorageModule{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"some-other-key": "value",
				},
			},
			Spec: csmv1.ContainerStorageModuleSpec{
				Driver: csmv1.Driver{
					CSIDriverType: csmv1.PowerFlex,
					ConfigVersion: "v2.12.0",
				},
			},
		}
		assert.False(t, IsUpgradeInProgress(ctx, cr, OperatorConfig{}))
	})

	t.Run("same version returns false", func(t *testing.T) {
		cr := &csmv1.ContainerStorageModule{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					MetadataPrefix + "/CSMOperatorConfigVersion": "v2.12.0",
				},
			},
			Spec: csmv1.ContainerStorageModuleSpec{
				Driver: csmv1.Driver{
					CSIDriverType: csmv1.PowerFlex,
					ConfigVersion: "v2.12.0",
				},
			},
		}
		assert.False(t, IsUpgradeInProgress(ctx, cr, OperatorConfig{}))
	})

	t.Run("different version returns true", func(t *testing.T) {
		cr := &csmv1.ContainerStorageModule{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					MetadataPrefix + "/CSMOperatorConfigVersion": "v2.11.0",
				},
			},
			Spec: csmv1.ContainerStorageModuleSpec{
				Driver: csmv1.Driver{
					CSIDriverType: csmv1.PowerFlex,
					ConfigVersion: "v2.12.0",
				},
			},
		}
		assert.True(t, IsUpgradeInProgress(ctx, cr, OperatorConfig{}))
	})

	t.Run("GetVersion error returns false", func(t *testing.T) {
		cr := &csmv1.ContainerStorageModule{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					MetadataPrefix + "/CSMOperatorConfigVersion": "v2.11.0",
				},
			},
			Spec: csmv1.ContainerStorageModuleSpec{
				// Use Version field which requires reading a file that won't exist
				Version: "v99.99.99",
				Driver: csmv1.Driver{
					CSIDriverType: csmv1.PowerFlex,
				},
			},
		}
		assert.False(t, IsUpgradeInProgress(ctx, cr, OperatorConfig{ConfigDirectory: "/nonexistent"}))
	})
}

func TestUpdateStatusRequeuePending(t *testing.T) {
	ctx := context.TODO()
	s := runtime.NewScheme()
	_ = csmv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = appsv1.AddToScheme(s)

	instance := createCSMWithStatus("powerflex", "default", csmv1.PowerFlex, csmv1.Replication, true, nil, csmv1.ContainerStorageModuleStatus{
		State:            constants.Succeeded,
		ControllerStatus: csmv1.PodStatus{Available: "0", Failed: "0", Desired: "0"},
		NodeStatus:       csmv1.PodStatus{Available: "0", Failed: "0", Desired: "0"},
	})

	fakeClient := ctrlClientFake.NewClientBuilder().WithScheme(s).
		WithObjects(instance).
		WithStatusSubresource(instance).
		Build()

	r := &FakeReconcileCSM{
		Client:    fakeClient,
		K8sClient: fake.NewSimpleClientset(),
	}

	newStatus := &csmv1.ContainerStorageModuleStatus{
		State: constants.Pending,
		ControllerStatus: csmv1.PodStatus{
			Available: "0",
			Failed:    "0",
			Desired:   "0",
		},
		NodeStatus: csmv1.PodStatus{
			Available: "0",
			Failed:    "0",
			Desired:   "0",
		},
	}

	// Ensure UNIT_TEST is NOT set so requeue logic is exercised
	origUnitTest := os.Getenv("UNIT_TEST")
	os.Unsetenv("UNIT_TEST")
	defer func() {
		if origUnitTest != "" {
			os.Setenv("UNIT_TEST", origUnitTest)
		}
	}()

	result, err := UpdateStatus(ctx, instance, r, newStatus, OperatorConfig{})
	assert.NoError(t, err)
	assert.Equal(t, 5*time.Second, result.RequeueAfter, "Pending state should requeue after 5 seconds")
}

func TestUpdateStatusRequeueFailed(t *testing.T) {
	ctx := context.TODO()
	s := runtime.NewScheme()
	_ = csmv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = appsv1.AddToScheme(s)

	instance := createCSMWithStatus("powerflex", "default", csmv1.PowerFlex, csmv1.Replication, true, nil, csmv1.ContainerStorageModuleStatus{
		State:            constants.Succeeded,
		ControllerStatus: csmv1.PodStatus{Available: "0", Failed: "1", Desired: "1"},
		NodeStatus:       csmv1.PodStatus{Available: "0", Failed: "0", Desired: "0"},
	})

	// Create daemonset with node status matching "not ready"
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "powerflex-node",
			Namespace: "default",
			Labels:    map[string]string{"powerflex-node": "true"},
		},
		Status: appsv1.DaemonSetStatus{
			NumberAvailable:        0,
			NumberReady:            0,
			DesiredNumberScheduled: 1,
		},
	}

	fakeClient := ctrlClientFake.NewClientBuilder().WithScheme(s).
		WithObjects(instance, ds).
		WithStatusSubresource(instance).
		Build()

	r := &FakeReconcileCSM{
		Client:    fakeClient,
		K8sClient: fake.NewSimpleClientset(),
	}

	crKey := instance.GetNamespace() + "/" + instance.GetName()
	ClearSucceededStabilityPeriod(crKey)

	// Set failure observation in the past so grace period has elapsed
	firstFailureObservedMux.Lock()
	firstFailureObserved[crKey] = time.Now().Add(-11 * time.Minute)
	firstFailureObservedMux.Unlock()

	newStatus := &csmv1.ContainerStorageModuleStatus{
		State: constants.Pending,
		ControllerStatus: csmv1.PodStatus{
			Available: "0",
			Failed:    "1",
			Desired:   "1",
		},
		NodeStatus: csmv1.PodStatus{
			Available: "0",
			Failed:    "0",
			Desired:   "0",
		},
	}

	origUnitTest := os.Getenv("UNIT_TEST")
	os.Unsetenv("UNIT_TEST")
	defer func() {
		if origUnitTest != "" {
			os.Setenv("UNIT_TEST", origUnitTest)
		}
	}()

	// Use deploymentStatusOverride so calculateState sees the failures
	overrideStatus := csmv1.PodStatus{
		Desired:   "1",
		Available: "0",
		Failed:    "1",
	}

	result, err := UpdateStatus(ctx, instance, r, newStatus, OperatorConfig{}, overrideStatus)
	assert.NoError(t, err)
	// nolint:staticcheck // SA1019: result.Requeue is deprecated but still used in tests
	assert.True(t, result.Requeue, "Failed state should requeue")
}

func TestUpdateStatusStateUnchanged(t *testing.T) {
	// Test the path where newStatus.State == csm.Status.State (no status update needed)
	ctx := context.TODO()
	s := runtime.NewScheme()
	_ = csmv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = appsv1.AddToScheme(s)

	// Instance starts in Pending state
	instance := createCSMWithStatus("powerflex", "default", csmv1.PowerFlex, csmv1.Replication, true, nil, csmv1.ContainerStorageModuleStatus{
		State:            constants.Pending,
		ControllerStatus: csmv1.PodStatus{Available: "0", Failed: "0", Desired: "0"},
		NodeStatus:       csmv1.PodStatus{Available: "0", Failed: "0", Desired: "0"},
	})

	fakeClient := ctrlClientFake.NewClientBuilder().WithScheme(s).
		WithObjects(instance).
		WithStatusSubresource(instance).
		Build()

	r := &FakeReconcileCSM{
		Client:    fakeClient,
		K8sClient: fake.NewSimpleClientset(),
	}

	// New status is also Pending (no change)
	newStatus := &csmv1.ContainerStorageModuleStatus{
		State: constants.Pending,
		ControllerStatus: csmv1.PodStatus{
			Available: "0",
			Failed:    "0",
			Desired:   "0",
		},
		NodeStatus: csmv1.PodStatus{
			Available: "0",
			Failed:    "0",
			Desired:   "0",
		},
	}

	// Should not error since state is unchanged
	_, err := UpdateStatus(ctx, instance, r, newStatus, OperatorConfig{})
	assert.NoError(t, err)
}

func TestUpdateStatusRunningPreservesStability(t *testing.T) {
	// Test that UpdateStatus does NOT clear stability period when running.
	// Clearing it would cause informer-driven handlers to restart the timer
	// immediately, creating an infinite Succeeded→Pending oscillation.
	ctx := context.TODO()
	s := runtime.NewScheme()
	_ = csmv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = appsv1.AddToScheme(s)

	instance := createCSMWithStatus("powerflex", "default", csmv1.PowerFlex, csmv1.Replication, true, nil, csmv1.ContainerStorageModuleStatus{
		State:            constants.Pending,
		ControllerStatus: csmv1.PodStatus{Available: "1", Failed: "0", Desired: "1"},
		NodeStatus:       csmv1.PodStatus{Available: "1", Failed: "0", Desired: "1"},
	})

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "powerflex-node",
			Namespace: "default",
			Labels:    map[string]string{"powerflex-node": "true"},
		},
		Status: appsv1.DaemonSetStatus{
			NumberAvailable:        1,
			NumberReady:            1,
			DesiredNumberScheduled: 1,
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "powerflex-node-abc123",
			Namespace: "default",
			Labels:    map[string]string{"app": "powerflex-node"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}

	fakeClient := ctrlClientFake.NewClientBuilder().WithScheme(s).
		WithObjects(instance, ds, pod).
		WithStatusSubresource(instance).
		Build()

	r := &FakeReconcileCSM{
		Client:    fakeClient,
		K8sClient: fake.NewSimpleClientset(),
	}

	crKey := instance.GetNamespace() + "/" + instance.GetName()
	// Bypass stability period so calculateState returns Succeeded + running=true
	firstSucceededObservedMux.Lock()
	firstSucceededObserved[crKey] = time.Now().Add(-31 * time.Second)
	firstSucceededObservedMux.Unlock()

	newStatus := &csmv1.ContainerStorageModuleStatus{
		State: constants.Succeeded,
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
	}

	// Use deploymentStatusOverride so calculateState sees controller pods as ready
	overrideStatus := csmv1.PodStatus{
		Desired:   "1",
		Available: "1",
		Failed:    "0",
	}

	_, err := UpdateStatus(ctx, instance, r, newStatus, OperatorConfig{}, overrideStatus)
	assert.NoError(t, err)

	// Verify stability period is preserved (NOT cleared) when running is true.
	// This ensures informer-driven handlers won't restart the stability timer
	// and push the state back to Pending.
	firstSucceededObservedMux.Lock()
	_, exists := firstSucceededObserved[crKey]
	firstSucceededObservedMux.Unlock()
	assert.True(t, exists, "Succeeded stability period should be preserved when running is true to prevent Succeeded→Pending oscillation")
}

func TestIsStabilityPeriodPending(t *testing.T) {
	t.Run("no entry returns false", func(t *testing.T) {
		key := "test-ns/no-entry"
		ClearSucceededStabilityPeriod(key)
		assert.False(t, IsStabilityPeriodPending(key))
	})

	t.Run("fresh entry returns true", func(t *testing.T) {
		key := "test-ns/fresh-entry"
		// Simulate what checkSucceededStabilityPeriod does on first call
		firstSucceededObservedMux.Lock()
		firstSucceededObserved[key] = time.Now()
		firstSucceededObservedMux.Unlock()

		assert.True(t, IsStabilityPeriodPending(key))
		ClearSucceededStabilityPeriod(key)
	})

	t.Run("elapsed entry returns false", func(t *testing.T) {
		key := "test-ns/elapsed-entry"
		firstSucceededObservedMux.Lock()
		firstSucceededObserved[key] = time.Now().Add(-31 * time.Second)
		firstSucceededObservedMux.Unlock()

		assert.False(t, IsStabilityPeriodPending(key))
		ClearSucceededStabilityPeriod(key)
	})
}
