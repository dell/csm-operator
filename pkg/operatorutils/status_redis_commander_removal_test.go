// ****
// Secure code generated utilizing Code Guardian.
// Applicable rules used: CG-K8S-001.2, CG-K8S-001.7
// ****

// Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operatorutils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// setupLegacyRedisCommanderAuthProxy creates a ready authorization proxy server
// whose CR still carries the legacy redisCommander field, plus a not-ready
// legacy Redis Commander Deployment that the reconciler is removing.
func setupLegacyRedisCommanderAuthProxy(t *testing.T, ns string) (FakeReconcileCSM, *appsv1.Deployment) {
	t.Helper()
	ctx := context.Background()
	ctrlClient := fullFakeClient()

	csm := authProxyCSMWithRedis(ns)
	assert.NoError(t, ctrlClient.Create(ctx, &csm))

	i32One := int32(1)
	for _, name := range []string{"proxy-server", "role-service", "storage-service", "tenant-service", "authorization-controller"} {
		dep := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 1, AvailableReplicas: 1},
			Spec:       appsv1.DeploymentSpec{Replicas: &i32One},
		}
		assert.NoError(t, ctrlClient.Create(ctx, &dep))
	}

	legacy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "rediscommander", Namespace: ns},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 0, AvailableReplicas: 0, UnavailableReplicas: 1},
		Spec:       appsv1.DeploymentSpec{Replicas: &i32One},
	}
	assert.NoError(t, ctrlClient.Create(ctx, legacy))

	k8sClient := fake.NewSimpleClientset()
	for _, name := range []string{"redis-csm", "sentinel"} {
		sts := appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Status:     appsv1.StatefulSetStatus{ReadyReplicas: 1, AvailableReplicas: 1},
			Spec:       appsv1.StatefulSetSpec{Replicas: &i32One},
		}
		assert.NoError(t, ctrlClient.Create(ctx, sts.DeepCopy()))
		_, err := k8sClient.AppsV1().StatefulSets(ns).Create(ctx, sts.DeepCopy(), metav1.CreateOptions{})
		assert.NoError(t, err)
	}

	return FakeReconcileCSM{Client: ctrlClient, K8sClient: k8sClient}, legacy
}

func TestAuthProxyStatusCheckIgnoresLegacyRedisCommander(t *testing.T) {
	fakeReconcile, _ := setupLegacyRedisCommanderAuthProxy(t, "test-legacy-rc-status")
	csm := authProxyCSMWithRedis("test-legacy-rc-status")

	status, err := authProxyStatusCheck(context.Background(), &csm, &fakeReconcile, nil, OperatorConfig{})

	assert.NoError(t, err)
	assert.True(t, status, "a legacy Redis Commander Deployment must not affect authorization readiness")
}

func TestGetAuthProxyDeploymentStatusExcludesLegacyRedisCommander(t *testing.T) {
	fakeReconcile, _ := setupLegacyRedisCommanderAuthProxy(t, "test-legacy-rc-count")
	csm := authProxyCSMWithRedis("test-legacy-rc-count")

	podStatus, err := getAuthProxyDeploymentStatus(context.Background(), &csm, &fakeReconcile)

	assert.NoError(t, err)
	assert.Equal(t, "7", podStatus.Desired, "5 service replicas + 1 redis + 1 sentinel; legacy Redis Commander excluded")
	assert.Equal(t, "7", podStatus.Available)
	assert.Equal(t, "0", podStatus.Failed)
}
