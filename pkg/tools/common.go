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

package tools

import (
	"context"
	"sync/atomic"

	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ReconcileCSM is the interface which extends each of the respective Reconcile interfaces
// for drivers
type ReconcileCSM interface {
	reconcile.Reconciler
	GetClient() crclient.Client
	GetK8sClient() kubernetes.Interface
	GetUpdateCount() int32
	IncrUpdateCount()
}

// FakeReconcileCSM -
type FakeReconcileCSM struct {
	reconcile.Reconciler
	crclient.Client
	K8sClient   kubernetes.Interface
	updateCount int32
}

// MockClient is a mock implementation of the client.Client interface for testing purposes.
type MockClient struct {
	mock.Mock
	crclient.Client
	GetFunc    func(ctx context.Context, key crclient.ObjectKey, obj crclient.Object, opts ...crclient.GetOption) error
	CreateFunc func(ctx context.Context, obj crclient.Object, opts ...crclient.CreateOption) error
	UpdateFunc func(ctx context.Context, obj crclient.Object, opts ...crclient.UpdateOption) error
}

// GetClient -
func (r *FakeReconcileCSM) GetClient() crclient.Client {
	return r.Client
}

// IncrUpdateCount - Increments the update count
func (r *FakeReconcileCSM) IncrUpdateCount() {
	atomic.AddInt32(&r.updateCount, 1)
}

// GetUpdateCount - Returns the current update count
func (r *FakeReconcileCSM) GetUpdateCount() int32 {
	return r.updateCount
}

// GetK8sClient - Returns the current update count
func (r *FakeReconcileCSM) GetK8sClient() kubernetes.Interface {
	return r.K8sClient
}

func (m *MockClient) Get(ctx context.Context, key crclient.ObjectKey, obj crclient.Object, opts ...crclient.GetOption) error {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, key, obj, opts...)
	}
	return nil
}

func (m *MockClient) Create(ctx context.Context, obj crclient.Object, opts ...crclient.CreateOption) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, obj, opts...)
	}
	return nil
}

func (m *MockClient) Update(ctx context.Context, obj crclient.Object, opts ...crclient.UpdateOption) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, obj, opts...)
	}
	return nil
}

func (m *MockClient) Delete(ctx context.Context, obj crclient.Object, opts ...crclient.DeleteOption) error {
	args := m.Called(ctx, obj, opts)
	if args.Get(0) == nil {
		return nil
	}
	return args.Error(0)
}
