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
package utils

import (
	"sync/atomic"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	client.Client
	K8sClient   kubernetes.Interface
	updateCount int32
}

// GetClient -
func (r *FakeReconcileCSM) GetClient() client.Client {
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
