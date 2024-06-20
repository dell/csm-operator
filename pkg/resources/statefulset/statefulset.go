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

package statefulset

import (
	"context"
	"time"

	//"fmt"

	"github.com/dell/csm-operator/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	"k8s.io/client-go/kubernetes"
	//"reflect"
)

// SleepTime - minimum time to sleep before checking the state of controller pod
var SleepTime = 10 * time.Second

// SyncStatefulSet - Syncs a StatefulSet for controller
func SyncStatefulSet(ctx context.Context, StatefulSet appsv1.StatefulSetApplyConfiguration, k8sClient kubernetes.Interface, accName string) error {
	log := logger.GetLogger(ctx)

	log.Infow("Sync StatefulSet:", "name", *StatefulSet.ObjectMetaApplyConfiguration.Name)

	StatefulSets := k8sClient.AppsV1().StatefulSets(*StatefulSet.ObjectMetaApplyConfiguration.Namespace)

	found, err := StatefulSets.Get(ctx, *StatefulSet.ObjectMetaApplyConfiguration.Name, metav1.GetOptions{})
	if err != nil {
		log.Errorw("get SyncStatefulSet error", "Error", err.Error())
	}
	opts := metav1.ApplyOptions{Force: true, FieldManager: "application/apply-patch"}
	if found == nil || found.Name == "" {
		log.Infow("No existing StatefulSet", "Name:", StatefulSet.Name)
	} else {
		log.Infow("found StatefulSet", "image", found.Spec.Template.Spec.Containers[0].Image)
	}

	StatefulSet.Spec.Template.Labels["app.kubernetes.io/instance"] = accName

	set, err := StatefulSets.Apply(ctx, &StatefulSet, opts)
	if err != nil {
		log.Errorw("Apply StatefulSet error", "set", err.Error())
		return err
	}
	log.Infow("StatefulSet apply done", "name", set.Name)
	return nil
}
