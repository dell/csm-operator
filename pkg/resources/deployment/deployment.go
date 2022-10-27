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

package deployment

import (
	"context"
	//"fmt"

	"github.com/dell/csm-operator/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	"k8s.io/client-go/kubernetes"

	//"reflect"
	"time"
)

// SleepTime - minimum time to sleep before checking the state of controller pod
var SleepTime = 10 * time.Second

// SyncDeployment - Syncs a Deployment for controller
func SyncDeployment(ctx context.Context, deployment appsv1.DeploymentApplyConfiguration, k8sClient kubernetes.Interface, csmName string) error {
	log := logger.GetLogger(ctx)

	log.Infow("Sync Deployment:", "name", *deployment.ObjectMetaApplyConfiguration.Name)

	deployments := k8sClient.AppsV1().Deployments(*deployment.ObjectMetaApplyConfiguration.Namespace)

	found, err := deployments.Get(ctx, *deployment.ObjectMetaApplyConfiguration.Name, metav1.GetOptions{})
	if err != nil {
		log.Errorw("get SyncDeployment error", "Error", err.Error())
	}
	opts := metav1.ApplyOptions{FieldManager: "application/apply-patch"}
	if found.Name == "" {
		log.Infow("No existing Deployment", "Name:", found.Name)

	} else {
		log.Infow("found deployment", "image", found.Spec.Template.Spec.Containers[0].Image)
	}

	deployment.Spec.Template.Labels["csm"] = csmName
	set, err := deployments.Apply(ctx, &deployment, opts)
	if err != nil {
		log.Errorw("Apply Deployment error", "set", err.Error())
		return err
	}
	log.Infow("deployment apply done", "name", set.Name)
	return nil
}
