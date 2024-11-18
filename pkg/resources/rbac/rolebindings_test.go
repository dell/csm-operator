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

package rbac

import (
	"context"
	"errors"
	"testing"

	common "github.com/dell/csm-operator/pkg/utils"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSyncClusterRoleBindings(t *testing.T) {
	type args struct {
		ctx    context.Context
		rb     rbacv1.ClusterRoleBinding
		client client.Client
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Test SyncClusterRoleBindings with known errror",
			args: args{
				ctx:    context.Background(),
				rb:     *MockClusterRoleBinding("", "test", "test"),
				client: ctrlClientFake.NewClientBuilder().Build(),
			},
			wantErr: true,
		},
		{
			name: "Test SyncClusterRoleBindings for unknown error",
			args: args{
				ctx: context.Background(),
				rb:  *MockClusterRoleBinding("test", "test", "test"),
				client: &common.MockClient{
					GetFunc: func(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return errors.New("unknown error")
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Test SyncClusterRoleBindings create scenario",
			args: args{
				ctx:    context.Background(),
				rb:     *MockClusterRoleBinding("test", "test", "test"),
				client: ctrlClientFake.NewClientBuilder().Build(),
			},
			wantErr: false,
		},
		{
			name: "Test SyncClusterRoleBindings update scenario",
			args: args{
				ctx: context.Background(),
				rb:  *MockClusterRoleBinding("test", "test", "test"),
				client: &common.MockClient{
					UpdateFunc: func(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
						return nil
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Test SyncClusterRoleBindings update error scenario",
			args: args{
				ctx: context.Background(),
				rb:  *MockClusterRoleBinding("test", "test", "test"),
				client: &common.MockClient{
					UpdateFunc: func(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
						return errors.New("update error")
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SyncClusterRoleBindings(tt.args.ctx, tt.args.rb, tt.args.client); (err != nil) != tt.wantErr {
				t.Errorf("SyncClusterRoleBindings() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func MockClusterRoleBinding(name, namespace, roleName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			Name: roleName,
			Kind: "ClusterRole",
		},
	}
}
