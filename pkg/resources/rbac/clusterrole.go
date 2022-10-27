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

	"github.com/dell/csm-operator/pkg/logger"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncClusterRole - Syncs a ClusterRole
func SyncClusterRole(ctx context.Context, clusterRole rbacv1.ClusterRole, client client.Client) error {
	log := logger.GetLogger(ctx)
	found := &rbacv1.ClusterRole{}
	err := client.Get(ctx, types.NamespacedName{Name: clusterRole.Name, Namespace: clusterRole.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new ClusterRole", "Name", clusterRole.Name)
		err = client.Create(ctx, &clusterRole)
		if err != nil {
			return err
		}
		// we need to return found object
		err := client.Get(ctx, types.NamespacedName{Name: clusterRole.Name, Namespace: clusterRole.Namespace}, found)
		if err != nil {
			return err
		}
	} else if err != nil {
		log.Info("Unknown error.", "Error", err.Error())
		return err
	} else {
		log.Info("Updating ClusterRole", "Name:", clusterRole.Name)
		err = client.Update(ctx, &clusterRole)
		if err != nil {
			return err
		}
	}

	return nil
}
