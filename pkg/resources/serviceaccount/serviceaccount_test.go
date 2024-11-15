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

package serviceaccount

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSyncServiceAccountp(t *testing.T) {
	ctx := context.TODO()
	serviceAccount := corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-service-account",
		},
	}

	t.Run("Create new ServiceAccount", func(t *testing.T) {
		// fake client
		client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

		err := SyncServiceAccount(ctx, serviceAccount, client)
		assert.NoError(t, err)

		// check that the service account was created
		foundServiceAccount := &corev1.ServiceAccount{}
		err = client.Get(ctx, types.NamespacedName{Name: serviceAccount.Name, Namespace: serviceAccount.Namespace}, foundServiceAccount)
		assert.NoError(t, err)
	})

	t.Run("Handle error on getting ServiceAccount", func(t *testing.T) {
		client := &MockClient{
			GetFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return errors.New("get error")
			},
		}

		err := SyncServiceAccount(ctx, serviceAccount, client)
		assert.Error(t, err)
		assert.Equal(t, "get error", err.Error())
	})

	t.Run("Handle error on creating ServiceAccount", func(t *testing.T) {
		client := &MockClient{
			GetFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return apierrors.NewNotFound(corev1.Resource("serviceaccount"), key.Name)
			},
			CreateFunc: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
				return errors.New("create error")
			},
		}

		err := SyncServiceAccount(ctx, serviceAccount, client)
		assert.Error(t, err)
		assert.Equal(t, "creating serviceaccount: create error", err.Error())

	})

	t.Run("Handle existing ServiceAccount", func(t *testing.T) {
		client := &MockClient{
			GetFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return nil
			},
		}

		err := SyncServiceAccount(ctx, serviceAccount, client)
		assert.NoError(t, err)
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
