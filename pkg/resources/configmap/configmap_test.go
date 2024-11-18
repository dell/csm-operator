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

package configmap

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

func TestSyncConfigMap(t *testing.T) {
	ctx := context.TODO()
	configMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	t.Run("Create new ConfigMap", func(t *testing.T) {
		// fake client
		client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

		err := SyncConfigMap(ctx, configMap, client)
		assert.NoError(t, err)

		// check that the configmap was created
		foundConfigMap := &corev1.ConfigMap{}
		err = client.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, foundConfigMap)
		assert.NoError(t, err)
		assert.Equal(t, configMap.Data, foundConfigMap.Data)

		// Check that the ConfigMap has the correct data
		if foundConfigMap.Data["key"] != configMap.Data["key"] {
			t.Errorf("ConfigMap has incorrect data: expected %s, got %s", configMap.Data["key"], foundConfigMap.Data["key"])
		}
	})

	t.Run("Update existing ConfigMap", func(t *testing.T) {
		// fake client
		client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(&configMap).Build()

		updatedConfigMap := configMap.DeepCopy()
		updatedConfigMap.Data["key"] = "new-value"

		err := SyncConfigMap(ctx, *updatedConfigMap, client)
		assert.NoError(t, err)

		// check that the configmap was updated
		foundConfigMap := &corev1.ConfigMap{}
		err = client.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, foundConfigMap)
		assert.NoError(t, err)
		assert.Equal(t, updatedConfigMap.Data, foundConfigMap.Data)

		// Check that the ConfigMap has the correct data
		if foundConfigMap.Data["key"] != updatedConfigMap.Data["key"] {
			t.Errorf("ConfigMap has incorrect data: expected %s, got %s", updatedConfigMap.Data["key"], foundConfigMap.Data["key"])
		}
	})

	t.Run("Handle error on getting ConfigMap", func(t *testing.T) {
		client := &MockClient{
			GetFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return errors.New("get error")
			},
		}

		err := SyncConfigMap(ctx, configMap, client)
		assert.Error(t, err)
		assert.Equal(t, "get error", err.Error())
	})

	t.Run("Handle error on creating ConfigMap", func(t *testing.T) {
		client := &MockClient{
			GetFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return apierrors.NewNotFound(corev1.Resource("configmap"), key.Name)
			},
			CreateFunc: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
				return errors.New("create error")
			},
		}

		err := SyncConfigMap(ctx, configMap, client)
		assert.Error(t, err)
		assert.Equal(t, "creating configmap: create error", err.Error())

	})

	t.Run("Handle error on updating ConfigMap", func(t *testing.T) {
		client := &MockClient{
			GetFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return nil
			},

			UpdateFunc: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
				return errors.New("update error")
			},
		}

		err := SyncConfigMap(ctx, configMap, client)
		assert.Error(t, err)
		assert.Equal(t, "updating configmap: update error", err.Error())
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
