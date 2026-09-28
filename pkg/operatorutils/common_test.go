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

package operatorutils

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

func TestPositiveDurationOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue string
		expected     string
	}{
		{
			name:         "valid positive duration returned as-is",
			input:        "45s",
			defaultValue: "30s",
			expected:     "45s",
		},
		{
			name:         "empty string returns default",
			input:        "",
			defaultValue: "30s",
			expected:     "30s",
		},
		{
			name:         "non-duration string returns default",
			input:        "not-a-duration",
			defaultValue: "30s",
			expected:     "30s",
		},
		{
			name:         "negative duration returns default",
			input:        "-10s",
			defaultValue: "30s",
			expected:     "30s",
		},
		{
			name:         "zero duration returns default",
			input:        "0s",
			defaultValue: "30s",
			expected:     "30s",
		},
		{
			name:         "whitespace-padded valid duration is accepted",
			input:        "  60s  ",
			defaultValue: "30s",
			expected:     "60s",
		},
		{
			name:         "valid minutes duration returned as-is",
			input:        "5m",
			defaultValue: "30s",
			expected:     "5m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PositiveDurationOrDefault(tt.input, tt.defaultValue)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestPositiveDurationOrEmpty(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid positive duration returned as-is",
			input:    "15s",
			expected: "15s",
		},
		{
			name:     "empty string returns empty",
			input:    "",
			expected: "",
		},
		{
			name:     "invalid string returns empty",
			input:    "not-a-timeout",
			expected: "",
		},
		{
			name:     "negative duration returns empty",
			input:    "-5s",
			expected: "",
		},
		{
			name:     "zero duration returns empty",
			input:    "0s",
			expected: "",
		},
		{
			name:     "whitespace-padded valid duration is accepted",
			input:    "  30s  ",
			expected: "30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PositiveDurationOrEmpty(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
