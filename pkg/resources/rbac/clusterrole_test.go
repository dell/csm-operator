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

package rbac

import (
	"context"
	"errors"
	"testing"

	common "github.com/dell/csm-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSyncClusterRole(t *testing.T) {
	ctx := context.TODO()
	clusterRole := rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-cluster-role",
		},
	}

	t.Run("Create new ClusterRole", func(t *testing.T) {
		// fake client
		client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

		err := SyncClusterRole(ctx, clusterRole, client)
		assert.NoError(t, err)

		// check that the cluster role was created
		foundClusterRole := &rbacv1.ClusterRole{}
		err = client.Get(ctx, types.NamespacedName{Name: clusterRole.Name, Namespace: clusterRole.Namespace}, foundClusterRole)
		assert.NoError(t, err)
	})

	t.Run("Handle error on getting clusterRole", func(t *testing.T) {
		client := &common.MockClient{
			GetFunc: func(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return errors.New("get error")
			},
		}

		err := SyncClusterRole(ctx, clusterRole, client)
		assert.Error(t, err)
		assert.Equal(t, "get error", err.Error())
	})

	t.Run("Handle error on creating clusterRole", func(t *testing.T) {
		client := &common.MockClient{
			GetFunc: func(_ context.Context, key client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return apierrors.NewNotFound(rbacv1.Resource("clusterrole"), key.Name)
			},
			CreateFunc: func(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
				return errors.New("create error")
			},
		}

		err := SyncClusterRole(ctx, clusterRole, client)
		assert.Error(t, err)
		assert.Equal(t, "create error", err.Error())
	})

	t.Run("Handle error on getting created clusterRole", func(t *testing.T) {
		client := &common.MockClient{
			CreateFunc: func(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
				return nil
			},
			GetFunc: func(_ context.Context, key client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return apierrors.NewNotFound(rbacv1.Resource("clusterrole"), key.Name)
			},
		}

		err := SyncClusterRole(ctx, clusterRole, client)
		assert.Error(t, err)
		assert.Equal(t, "clusterrole.rbac.authorization.k8s.io \"my-cluster-role\" not found", err.Error())
	})

	t.Run("Handle existing clusterRole", func(t *testing.T) {
		client := &common.MockClient{
			GetFunc: func(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return nil
			},
		}

		err := SyncClusterRole(ctx, clusterRole, client)
		assert.NoError(t, err)
	})

	t.Run("Handle existing clusterRole update error", func(t *testing.T) {
		client := &common.MockClient{
			GetFunc: func(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return nil
			},
			UpdateFunc: func(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
				return errors.New("update error")
			},
		}

		err := SyncClusterRole(ctx, clusterRole, client)
		assert.Error(t, err)
		assert.Equal(t, "update error", err.Error())
	})
}
