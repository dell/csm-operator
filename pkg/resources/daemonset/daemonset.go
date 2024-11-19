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

package daemonset

import (
	"context"

	"github.com/dell/csm-operator/pkg/logger"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	"k8s.io/client-go/kubernetes"
)

// SyncDaemonset - Syncs a daemonset object
func SyncDaemonset(ctx context.Context, daemonset appsv1.DaemonSetApplyConfiguration, k8sClient kubernetes.Interface, csmName string) error {
	log := logger.GetLogger(ctx)

	log.Infow("Sync DaemonSet:", "name", *daemonset.ObjectMetaApplyConfiguration.Name)

	// Get a config to talk to the apiserver
	daemonsets := k8sClient.AppsV1().DaemonSets(*daemonset.ObjectMetaApplyConfiguration.Namespace)

	found, err := daemonsets.Get(ctx, *daemonset.ObjectMetaApplyConfiguration.Name, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		log.Infow("No existing DaemonSet", "Name:", daemonset.Name)
	} else if err != nil {
		log.Errorw("Get SyncDaemonSet error", "Error", err.Error())
		return err
	} else {
		log.Infow("Found DaemonSet", "image", found.Spec.Template.Spec.Containers[0].Image)
	}

	opts := metav1.ApplyOptions{FieldManager: "application/apply-patch"}

	// ensure Spec and Template are initialized
	if daemonset.Spec.Template.Labels == nil {
		daemonset.Spec.Template.Labels = make(map[string]string)
	}
	daemonset.Spec.Template.Labels["csm"] = csmName

	_, err = daemonsets.Apply(ctx, &daemonset, opts)
	if err != nil {
		log.Errorw("Apply DaemonSet error", "set", err.Error())
		return err
	}
	return nil
}
