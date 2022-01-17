package utils_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kubernetes-csi/external-snapshotter/client/v3/clientset/versioned/scheme"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/constants"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/pkg/utils/mocks"
	"github.com/stretchr/testify/assert"
)

func Test_HandleSuccess(t *testing.T) {
	type checkFn func(*testing.T, reconcile.Result, error)
	check := func(fns ...checkFn) []checkFn { return fns }

	hasNoError := func(t *testing.T, result reconcile.Result, err error) {
		if err != nil {
			t.Fatalf("expected no error but found %v", err)
		}
	}

	checkExpectedOutput := func(expectedOutput reconcile.Result) func(t *testing.T, result reconcile.Result, err error) {
		return func(t *testing.T, result reconcile.Result, err error) {
			assert.Equal(t, expectedOutput, result)
		}
	}

	// hasError := func(t *testing.T, result interface{}, err error) {
	// 	if err == nil {
	// 		t.Fatalf("expected error")
	// 	}
	// }

	tests := map[string]func(t *testing.T) (context.Context, *csmv1.ContainerStorageModule, utils.ReconcileCSM, logr.Logger, *csmv1.ContainerStorageModuleStatus, *csmv1.ContainerStorageModuleStatus, []checkFn){

		"success all in running state": func(*testing.T) (context.Context, *csmv1.ContainerStorageModule, utils.ReconcileCSM, logr.Logger, *csmv1.ContainerStorageModuleStatus, *csmv1.ContainerStorageModuleStatus, []checkFn) {

			reconciler := mocks.ReconcileCSM{}

			clientBuilder := fake.NewClientBuilder()

			s := scheme.Scheme
			appsv1.SchemeBuilder.AddToScheme(s)
			csmv1.SchemeBuilder.AddToScheme(s)
			v1.SchemeBuilder.AddToScheme(s)
			clientBuilder.WithScheme(s)

			replicas := int32(1)
			clientBuilder.WithObjects(
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-csm-controller",
						Namespace: "csm-namespace",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
					},
					Status: appsv1.DeploymentStatus{
						ReadyReplicas: 1,
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-csm-controller",
						Namespace: "csm-namespace",
						Labels:    map[string]string{"app": "test-csm-controller"},
					},
					Status: v1.PodStatus{
						Phase: corev1.PodRunning,
						ContainerStatuses: []v1.ContainerStatus{
							{
								State: v1.ContainerState{
									Running: &v1.ContainerStateRunning{},
								},
							},
						},
					},
				},
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-csm-node",
						Namespace: "csm-namespace",
					},
					Spec: appsv1.DaemonSetSpec{
						MinReadySeconds: 0,
					},
					Status: appsv1.DaemonSetStatus{
						DesiredNumberScheduled: 1,
						NumberReady:            1,
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-csm-node",
						Labels:    map[string]string{"app": "test-csm-node"},
						Namespace: "csm-namespace",
					},
					Status: v1.PodStatus{
						Conditions: []v1.PodCondition{
							{
								Type:   v1.PodReady,
								Status: v1.ConditionTrue,
								LastTransitionTime: metav1.Time{
									Time: time.Now(),
								},
							},
						},
						Phase: corev1.PodRunning,
						ContainerStatuses: []v1.ContainerStatus{
							{
								State: v1.ContainerState{
									Running: &v1.ContainerStateRunning{},
								},
							},
						},
					},
				},
				&csmv1.ContainerStorageModule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-csm",
						Namespace: "csm-namespace",
					},
					Status: csmv1.ContainerStorageModuleStatus{
						ControllerStatus: csmv1.PodStatus{
							Available: []string{"test-csm-controller"},
						},
						NodeStatus: csmv1.PodStatus{
							Available: []string{"test-csm-node"},
						},
						State: csmv1.CSMStateType(csmv1.Running),
					},
				},
			)

			fakeClient := clientBuilder.Build()

			reconciler.On("GetClient").Return(fakeClient)

			ctx := context.Background()
			instance := csmv1.ContainerStorageModule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-csm",
					Namespace: "csm-namespace",
				},
			}
			newStatus := csmv1.ContainerStorageModuleStatus{}
			oldStatus := csmv1.ContainerStorageModuleStatus{
				ControllerStatus: csmv1.PodStatus{
					Available: []string{"test-csm-controller"},
				},
				NodeStatus: csmv1.PodStatus{
					Available: []string{"test-csm-node"},
				},
				LastUpdate: csmv1.LastUpdate{
					Condition: csmv1.Running,
				},
				State: constants.Running,
			}

			log := zap.New()

			return ctx, &instance, &reconciler, log, &newStatus, &oldStatus, check(hasNoError, checkExpectedOutput(reconcile.Result{Requeue: false, RequeueAfter: 0}))
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, instance, r, reqLogger, newStatus, oldStatus, checkFns := tc(t)

			result, err := utils.HandleSuccess(ctx, instance, r, reqLogger, newStatus, oldStatus)

			for _, checkFn := range checkFns {
				checkFn(t, result, err)
			}
		})
	}
}
