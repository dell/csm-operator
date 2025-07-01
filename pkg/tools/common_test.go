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

package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestIncrUpdateCount(t *testing.T) {
	// Create a new instance of FakeReconcileCSM
	r := &FakeReconcileCSM{}

	// Call the IncrUpdateCount function
	r.IncrUpdateCount()

	// Check if the updateCount is incremented
	if r.updateCount != 1 {
		t.Errorf("Expected updateCount to be 1, but got %d", r.updateCount)
	}
}

func TestGetUpdateCount(t *testing.T) {
	// Create a new instance of FakeReconcileCSM
	r := &FakeReconcileCSM{}

	// Call the IncrUpdateCount function
	result := r.GetUpdateCount()

	// Check if the updateCount is incremented
	if result != 0 {
		t.Errorf("Expected updateCount to be 0, but got %d", result)
	}
}

func TestMockClient_Get(t *testing.T) {
	mockClient := new(MockClient)
	mockClient.GetFunc = func(_ context.Context, _ crclient.ObjectKey, _ crclient.Object, _ ...crclient.GetOption) error {
		return nil
	}

	ctx := context.TODO()
	key := crclient.ObjectKey{Name: "test", Namespace: "default"}
	obj := &v1.Pod{}

	err := mockClient.Get(ctx, key, obj)
	assert.NoError(t, err)
}

func TestMockClient_Create(t *testing.T) {
	mockClient := new(MockClient)
	mockClient.CreateFunc = func(_ context.Context, _ crclient.Object, _ ...crclient.CreateOption) error {
		return nil
	}

	ctx := context.TODO()
	obj := &v1.Pod{}

	err := mockClient.Create(ctx, obj)
	assert.NoError(t, err)
}

func TestMockClient_Update(t *testing.T) {
	mockClient := new(MockClient)
	mockClient.UpdateFunc = func(_ context.Context, _ crclient.Object, _ ...crclient.UpdateOption) error {
		return nil
	}

	ctx := context.TODO()
	obj := &v1.Pod{}

	err := mockClient.Update(ctx, obj)
	assert.NoError(t, err)
}

func TestMockClient_Delete(t *testing.T) {
	mockClient := new(MockClient)
	mockClient.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ctx := context.TODO()
	obj := &v1.Pod{}

	err := mockClient.Delete(ctx, obj)
	assert.NoError(t, err)
	mockClient.AssertCalled(t, "Delete", ctx, obj, mock.Anything)
}
