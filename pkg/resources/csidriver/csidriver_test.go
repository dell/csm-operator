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

package csidriver

import (
	"context"
	"errors"
	"testing"

	common "github.com/dell/csm-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSyncCSIDriver(t *testing.T) {
	ctx := context.TODO()
	csiDriver := storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-csidriver",
		},
	}

	t.Run("Create new CSIDriver", func(t *testing.T) {
		// fake client
		client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

		err := SyncCSIDriver(ctx, csiDriver, client)
		assert.NoError(t, err)

		// check that the csidriver was created
		foundCSIDriver := &storagev1.CSIDriver{}
		err = client.Get(ctx, types.NamespacedName{Name: csiDriver.Name}, foundCSIDriver)
		assert.NoError(t, err)
		assert.Equal(t, csiDriver.Name, foundCSIDriver.Name)
	})

	t.Run("Update existing CSIDriver", func(t *testing.T) {
		// fake client
		client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

		updatedCSIDriver := csiDriver.DeepCopy()
		updatedCSIDriver.Annotations = map[string]string{"key": "test-annotation"}

		err := SyncCSIDriver(ctx, *updatedCSIDriver, client)
		assert.NoError(t, err)

		// check that the csidriver was updated
		foundCSIDriver := &storagev1.CSIDriver{}
		err = client.Get(ctx, types.NamespacedName{Name: csiDriver.Name}, foundCSIDriver)
		assert.NoError(t, err)
		assert.Equal(t, updatedCSIDriver.Annotations, foundCSIDriver.Annotations)
	})

	t.Run("Handle error on getting CSIDriver", func(t *testing.T) {
		client := &common.MockClient{
			GetFunc: func(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return errors.New("get error")
			},
		}

		err := SyncCSIDriver(ctx, csiDriver, client)
		assert.Error(t, err)
		assert.Equal(t, "get error", err.Error())
	})

	t.Run("Handle error on creating CSIDriver", func(t *testing.T) {
		client := &common.MockClient{
			GetFunc: func(_ context.Context, key client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return apierrors.NewNotFound(storagev1.Resource("csidriver"), key.Name)
			},
			CreateFunc: func(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
				return errors.New("create error")
			},
		}

		err := SyncCSIDriver(ctx, csiDriver, client)
		assert.Error(t, err)
		assert.Equal(t, "creating csidriver object: create error", err.Error())
	})

	t.Run("Handle error on updating CSIDriver", func(t *testing.T) {
		client := &common.MockClient{
			GetFunc: func(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return nil
			},
			UpdateFunc: func(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
				return errors.New("update error")
			},
		}

		err := SyncCSIDriver(ctx, csiDriver, client)
		assert.Error(t, err)
		assert.Equal(t, "updating csidriver object: update error", err.Error())
	})
}
