/*
 Copyright © 2021 - 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
 
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/
package serviceaccount

import (
	"context"

	"github.com/dell/csm-operator/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncServiceAccount - Syncs a ServiceAccount
//func SyncServiceAccount(ctx context.Context, sa *corev1.ServiceAccount, client client.Client, csmName string, trcID string) error {
func SyncServiceAccount(ctx context.Context, sa corev1.ServiceAccount, client client.Client) error {
	log := logger.GetLogger(ctx)
	found := &corev1.ServiceAccount{}
	err := client.Get(ctx, types.NamespacedName{Name: sa.Name, Namespace: sa.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Infow("Creating a new ServiceAccount", "Namespace", sa.Namespace, "Name", sa.Name)
		err = client.Create(ctx, &sa)
		if err != nil {
			return err
		}

		return nil
	} else if err != nil {
		log.Errorw("Unknown error.", "Error", err.Error())
		return err
	} else {
		log.Infow("Updating ServiceAccount", "Name:", sa.Name)
		err = client.Update(ctx, &sa)
		if err != nil {
			return err
		}
	}
	return nil
}
