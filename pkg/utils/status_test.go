package utils_test

import (
	"context"
	"testing"
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/pkg/utils/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_CalculateState(t *testing.T) {
	type checkFn func(*testing.T, bool, error)
	check := func(fns ...checkFn) []checkFn { return fns }

	hasNoError := func(t *testing.T, result bool, err error) {
		if err != nil {
			t.Fatalf("expected no error but found %v", err)
		}
	}

	checkExpectedOutput := func(expectedOutput interface{}) func(t *testing.T, result bool, err error) {
		return func(t *testing.T, result bool, err error) {
			assert.Equal(t, expectedOutput, result)
		}
	}

	// hasError := func(t *testing.T, result interface{}, err error) {
	// 	if err == nil {
	// 		t.Fatalf("expected error")
	// 	}
	// }

	tests := map[string]func(t *testing.T) (context.Context, *csmv1.ContainerStorageModule, utils.ReconcileCSM, *csmv1.ContainerStorageModuleStatus, []checkFn){

		"success": func(*testing.T) (context.Context, *csmv1.ContainerStorageModule, utils.ReconcileCSM, *csmv1.ContainerStorageModuleStatus, []checkFn) {

			reconciler := mocks.ReconcileCSM{}
			client := mocks.CRClient{}
			client.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
				arg := args.Get(2).(*appsv1.Deployment)
				replicas := int32(1)
				arg.Spec.Replicas = &replicas
				arg.Status.ReadyReplicas = 1
			}).Once()

			client.On("List", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
				arg := args.Get(1).(*v1.PodList)
				arg.Items = []v1.Pod{
					{
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
				}
			}).Once()

			client.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
				arg := args.Get(2).(*appsv1.DaemonSet)
				arg.Status.DesiredNumberScheduled = 1
				arg.Status.NumberReady = 1
				arg.Spec.MinReadySeconds = 0
			}).Once()

			client.On("List", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
				arg := args.Get(1).(*v1.PodList)
				arg.Items = []v1.Pod{
					{
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
				}
			}).Once()

			reconciler.On("GetClient").Return(&client)

			ctx := context.Background()
			instance := csmv1.ContainerStorageModule{}
			newStatus := csmv1.ContainerStorageModuleStatus{}

			return ctx, &instance, &reconciler, &newStatus, check(hasNoError, checkExpectedOutput(true))
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, instance, r, newStatus, checkFns := tc(t)
			result, err := utils.CalculateState(ctx, instance, r, newStatus)

			for _, checkFn := range checkFns {
				checkFn(t, result, err)
			}
		})
	}
}
