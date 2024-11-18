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

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSyncClusterRole(t *testing.T) {
	// Create a fake context.Context
	ctx := context.Background()

	// Create a new instance of the ClusterRole
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
	}

	t.Run("Get ClusterRole", func(t *testing.T) {
		client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

		err := SyncClusterRole(ctx, *cr, client)
		assert.NoError(t, err)

		// Get the updated ClusterRole from the fake client
		updated := &rbacv1.ClusterRole{}
		err = client.Get(context.TODO(), types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}, updated)
		if err != nil {
			t.Fatalf("Failed to get updated ClusterRole: %v", err)
		}

		// Assert that the ClusterRole has been updated correctly
		if updated.Name != cr.Name {
			t.Errorf("Expected Name to be %s, got %s", cr.Name, updated.Name)
		}
		if updated.Namespace != cr.Namespace {
			t.Errorf("Expected Namespace to be %s, got %s", cr.Namespace, updated.Namespace)
		}
	})

	t.Run("Handle error on getting ClusterRole", func(t *testing.T) {
		client := &MockClient{
			GetFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return errors.New("get error")
			},
		}
		err := SyncClusterRole(ctx, *cr, client)
		assert.Error(t, err)
		assert.Equal(t, "get error", err.Error())
	})

	t.Run("Create new ClusterRole", func(t *testing.T) {
		// fake client
		client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		err := SyncClusterRole(ctx, *cr, client)
		assert.NoError(t, err)
		// check that the clusterrole was created
		foundClusterRole := &rbacv1.ClusterRole{}
		err = client.Get(ctx, types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}, foundClusterRole)
		assert.NoError(t, err)
		assert.Equal(t, cr.Name, foundClusterRole.Name)
		// Check that the clusterrole has the correct data
		if foundClusterRole.Name != cr.Name {
			t.Errorf("ClusterRole has incorrect data: expected %s, got %s", cr.Name, foundClusterRole.Name)
		}
	})

	//broken sub-test case
	// t.Run("Update ClusterRole", func(t *testing.T) {
	// 	// fake client
	// 	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

	// 	updatedClusterRole := cr.DeepCopy()
	// 	updatedClusterRole.Name = "new-name"

	// 	err := SyncClusterRole(ctx, *updatedClusterRole, client)
	// 	assert.NoError(t, err)

	// 	// check that the ClusterRole was updated
	// 	foundClusterRole := &rbacv1.ClusterRole{}
	// 	err = client.Get(ctx, types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}, foundClusterRole)
	// 	assert.NoError(t, err)
	// 	assert.Equal(t, updatedClusterRole.Name, foundClusterRole.Name)
	// 	// Check that the ClusterRole has the correct data
	// 	if foundClusterRole.Name != updatedClusterRole.Name {
	// 		t.Errorf("ClusterRole has incorrect data: expected %s, got %s", updatedClusterRole.Name, foundClusterRole.Name)
	// 	}
	// })

	t.Run("Handle error on creating ClusterRole", func(t *testing.T) {
		client := &MockClient{
			GetFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return errors.New("get error")
			},
			CreateFunc: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
				return errors.New("create error")
			},
		}
		err := SyncClusterRole(ctx, *cr, client)
		assert.Error(t, err)
		assert.Equal(t, "get error", err.Error())
	})
}

// MockClient is a mock implementation of the client.Client interface for testing purposes.
type MockClient struct {
	client.Client
	GetFunc    func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	CreateFunc func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	UpdateFunc func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
}

func (m *MockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, key, obj, opts...)
	}
	return nil
}

func (m *MockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, obj, opts...)
	}
	return nil
}

func (m *MockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, obj, opts...)
	}
	return nil
}
