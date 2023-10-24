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
	"context"
	"errors"
	"fmt"
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/constants"
	"github.com/dell/csm-operator/pkg/logger"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"sync"

	t1 "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var dMutex sync.RWMutex

func getInt32(pointer *int32) int32 {
	if pointer == nil {
		return 0
	}
	return *pointer
}

// TODO: Currently commented this block of code as the API used to get the latest deployment status is not working as expected
// TODO: Can be uncommented once this issues gets sorted out
/* func getDeploymentStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM) (int32, csmv1.PodStatus, error) {
	deployment := &appsv1.Deployment{}
	log := logger.GetLogger(ctx)

	var err error
	var msg string
	totalReplicas := int32(0)
	totalReadyPods := 0
	totalFailedCount := 0

	_, clusterClients, err := GetDefaultClusters(ctx, *instance, r)
	if err != nil {
		return int32(totalReplicas), csmv1.PodStatus{}, err
	}

	for _, cluster := range clusterClients {
		log.Infof("deployment status for cluster: %s", cluster.ClusterID)
		msg += fmt.Sprintf("error message for %s \n", cluster.ClusterID)

		err = cluster.ClusterCTRLClient.Get(ctx, t1.NamespacedName{Name: instance.GetControllerName(),
			Namespace: instance.GetNamespace()}, deployment)
		if err != nil {
			return 0, csmv1.PodStatus{}, err
		}
		replicas := getInt32(deployment.Spec.Replicas)
		readyPods := 0
		failedCount := 0

		//powerflex and powerscale use different label names for the controller name:
		//app=isilon-controller
		//name=vxflexos-controller
		//name=powerstore-controller
		driver := instance.GetDriverType()
		log.Infof("driver type: %s", driver)
		controllerLabelName := "app"
		if (driver == "powerflex") || (driver == "powerstore") {
			controllerLabelName = "name"
		}
		label := instance.GetName() + "-controller"
		opts := []client.ListOption{
			client.InNamespace(instance.GetNamespace()),
			client.MatchingLabels{controllerLabelName: label},
		}

		podList := &corev1.PodList{}
		err = cluster.ClusterCTRLClient.List(ctx, podList, opts...)
		if err != nil {
			return deployment.Status.ReadyReplicas, csmv1.PodStatus{}, err
		}

		for _, pod := range podList.Items {

			log.Infof("deployment pod count %d name %s status %s", readyPods, pod.Name, pod.Status.Phase)
			if pod.Status.Phase == corev1.PodRunning {
				readyPods++
			} else if pod.Status.Phase == corev1.PodPending {
				failedCount++
				errMap := make(map[string]string)
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.State.Waiting != nil && cs.State.Waiting.Reason != constants.ContainerCreating {
						log.Infow("container", "message", cs.State.Waiting.Message, constants.Reason, cs.State.Waiting.Reason)
						shortMsg := strings.Replace(cs.State.Waiting.Message,
							constants.PodStatusRemoveString, "", 1)
						errMap[cs.State.Waiting.Reason] = shortMsg
					}
					if cs.State.Waiting != nil && cs.State.Waiting.Reason == constants.ContainerCreating {
						errMap[cs.State.Waiting.Reason] = constants.PendingCreate
					}
				}
				for k, v := range errMap {
					msg += k + "=" + v
				}
			}
		}

		totalReplicas += replicas
		totalReadyPods += readyPods
		totalFailedCount += failedCount
	}

	if totalFailedCount > 0 {
		err = errors.New(msg)
	}

	log.Infof("Deployment totalReplicas count %d totalReadyPods %d totalFailedCount %d", totalReplicas, totalReadyPods, totalFailedCount)

	return totalReplicas, csmv1.PodStatus{
		Available: fmt.Sprintf("%d", totalReadyPods),
		Desired:   fmt.Sprintf("%d", totalReplicas),
		Failed:    fmt.Sprintf("%d", totalFailedCount),
	}, err
} */

func getAccStatefulSetStatus(ctx context.Context, instance *csmv1.ApexConnectivityClient, r ReconcileCSM) (int32, csmv1.PodStatus, error) {
	statefulSet := &appsv1.StatefulSet{}
	log := logger.GetLogger(ctx)

	var err error
	var msg string
	totalReplicas := int32(0)
	totalReadyPods := 0
	totalFailedCount := 0

	_, clusterClients, err := GetAccDefaultClusters(ctx, *instance, r)
	if err != nil {
		return int32(totalReplicas), csmv1.PodStatus{}, err
	}

	for _, cluster := range clusterClients {
		log.Infof("statefulSet status for cluster: %s", cluster.ClusterID)
		msg += fmt.Sprintf("error message for %s \n", cluster.ClusterID)

		err = cluster.ClusterCTRLClient.Get(ctx, t1.NamespacedName{Name: instance.GetApexConnectivityClientName(),
			Namespace: instance.GetNamespace()}, statefulSet)
		if err != nil {
			return 0, csmv1.PodStatus{}, err
		}
		replicas := getInt32(statefulSet.Spec.Replicas)
		readyPods := 0
		failedCount := 0

		label := instance.GetNamespace() + "-controller"
		opts := []client.ListOption{
			client.InNamespace(instance.GetNamespace()),
			client.MatchingLabels{"app": label},
		}

		podList := &corev1.PodList{}
		err = cluster.ClusterCTRLClient.List(ctx, podList, opts...)
		if err != nil {
			return statefulSet.Status.ReadyReplicas, csmv1.PodStatus{}, err
		}

		for _, pod := range podList.Items {

			log.Infof("statefulSet pod count %d name %s status %s", readyPods, pod.Name, pod.Status.Phase)
			if pod.Status.Phase == corev1.PodRunning {
				readyPods++
			} else if pod.Status.Phase == corev1.PodPending {
				failedCount++
				errMap := make(map[string]string)
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.State.Waiting != nil && cs.State.Waiting.Reason != constants.ContainerCreating {
						log.Infow("container", "message", cs.State.Waiting.Message, constants.Reason, cs.State.Waiting.Reason)
						shortMsg := strings.Replace(cs.State.Waiting.Message,
							constants.PodStatusRemoveString, "", 1)
						errMap[cs.State.Waiting.Reason] = shortMsg
					}
					if cs.State.Waiting != nil && cs.State.Waiting.Reason == constants.ContainerCreating {
						errMap[cs.State.Waiting.Reason] = constants.PendingCreate
					}
				}
				for k, v := range errMap {
					msg += k + "=" + v
				}
			}
		}

		totalReplicas += replicas
		totalReadyPods += readyPods
		totalFailedCount += failedCount
	}

	if totalFailedCount > 0 {
		err = errors.New(msg)
	}

	return totalReplicas, csmv1.PodStatus{
		Available: fmt.Sprintf("%d", totalReadyPods),
		Desired:   fmt.Sprintf("%d", totalReplicas),
		Failed:    fmt.Sprintf("%d", totalFailedCount),
	}, err
}

func getDaemonSetStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM) (int32, csmv1.PodStatus, error) {
	log := logger.GetLogger(ctx)

	var msg string

	totalAvialable := int32(0)
	totalDesired := int32(0)
	totalFailedCount := 0

	_, clusterClients, err := GetDefaultClusters(ctx, *instance, r)
	if err != nil {
		return 0, csmv1.PodStatus{}, err
	}

	for _, cluster := range clusterClients {
		log.Infof("\ndaemonset status for cluster: %s", cluster.ClusterID)
		msg += fmt.Sprintf("error message for %s \n", cluster.ClusterID)

		ds := &appsv1.DaemonSet{}
		err := cluster.ClusterCTRLClient.Get(ctx, t1.NamespacedName{Name: instance.GetNodeName(),
			Namespace: instance.GetNamespace()}, ds)
		if err != nil {
			return 0, csmv1.PodStatus{}, err
		}

		failedCount := 0
		podList := &corev1.PodList{}
		label := instance.GetName() + "-node"
		opts := []client.ListOption{
			client.InNamespace(instance.GetNamespace()),
			client.MatchingLabels{"app": label},
		}

		err = cluster.ClusterCTRLClient.List(ctx, podList, opts...)
		if err != nil {
			return ds.Status.DesiredNumberScheduled, csmv1.PodStatus{}, err
		}

		errMap := make(map[string]string)
		for _, pod := range podList.Items {
			log.Infof("daemonset pod %s : %s", pod.Name, pod.Status.Phase)
			if pod.Status.Phase == corev1.PodPending {
				failedCount++
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.State.Waiting != nil && cs.State.Waiting.Reason != constants.ContainerCreating {
						//message: Back-off pulling image "dellec/csi-isilon:xxxx"
						//reason: ImagePullBackOff
						log.Infow("daemonset pod container", "message", cs.State.Waiting.Message, constants.Reason, cs.State.Waiting.Reason)
						shortMsg := strings.Replace(cs.State.Waiting.Message,
							constants.PodStatusRemoveString, "", 1)
						errMap[cs.State.Waiting.Reason] = shortMsg
					}
					if cs.State.Waiting != nil && cs.State.Waiting.Reason == constants.ContainerCreating {
						log.Infof("daemonset pod container %s : %s", pod.Name, pod.Status.Phase)
						errMap[cs.State.Waiting.Reason] = constants.PendingCreate
					}
				}
			}
		}
		for k, v := range errMap {
			msg += k + "=" + v
		}

		log.Infof("daemonset status available pods %d", ds.Status.NumberAvailable)
		log.Infof("daemonset status failedCount pods %d", failedCount)
		log.Infof("daemonset status desired pods %d", ds.Status.DesiredNumberScheduled)

		totalAvialable += ds.Status.NumberAvailable
		totalDesired += ds.Status.DesiredNumberScheduled
		totalFailedCount += failedCount

	}

	if totalFailedCount > 0 {
		err = errors.New(msg)
	}
	return totalDesired, csmv1.PodStatus{
		Available: fmt.Sprintf("%d", totalAvialable),
		Desired:   fmt.Sprintf("%d", totalDesired),
		Failed:    fmt.Sprintf("%d", totalFailedCount),
	}, err
}

func calculateState(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, newStatus *csmv1.ContainerStorageModuleStatus) (bool, error) {
	log := logger.GetLogger(ctx)
	running := false
	// TODO: Currently commented this block of code as the API used to get the latest deployment status is not working as expected
	// TODO: Can be uncommented once this issues gets sorted out
	/* controllerReplicas, controllerStatus, controllerErr := getDeploymentStatus(ctx, instance, r)
	expected, nodeStatus, daemonSetErr := getDaemonSetStatus(ctx, instance, r)
	newStatus.ControllerStatus = controllerStatus
	newStatus.NodeStatus = nodeStatus */
	expected, nodeStatus, daemonSetErr := getDaemonSetStatus(ctx, instance, r)
	newStatus.NodeStatus = nodeStatus
	controllerReplicas := newStatus.ControllerStatus.Desired
	controllerStatus := newStatus.ControllerStatus

	newStatus.State = constants.Failed
	log.Infof("deployment controllerReplicas [%s]", controllerReplicas)
	log.Infof("deployment controllerStatus.Available [%s]", controllerStatus.Available)

	log.Infof("daemonset expected [%d]", expected)
	log.Infof("daemonset nodeStatus.Available [%s]", nodeStatus.Available)

	if (controllerReplicas == controllerStatus.Available) && (fmt.Sprintf("%d", expected) == nodeStatus.Available) {
		running = true
		newStatus.State = constants.Succeeded
	}
	log.Infof("calculate overall state [%s]", newStatus.State)
	var err error = nil
	// TODO: Uncomment this when the controller runtime API gets fixed
	/*
		if controllerErr != nil {
			err = controllerErr
		}
		if daemonSetErr != nil {
			err = daemonSetErr
		}
		if daemonSetErr != nil && controllerErr != nil {
			err = fmt.Errorf("ControllerError: %s, Daemonseterror: %s", controllerErr.Error(), daemonSetErr.Error())
			log.Infof("calculate overall error msg [%s]", err.Error())
		} */

	if daemonSetErr != nil {
		err = daemonSetErr
		log.Infof("calculate Daemonseterror msg [%s]", daemonSetErr.Error())
	}
	SetStatus(ctx, r, instance, newStatus)
	return running, err
}

func calculateAccState(ctx context.Context, instance *csmv1.ApexConnectivityClient, r ReconcileCSM, newStatus *csmv1.ApexConnectivityClientStatus) (bool, error) {
	log := logger.GetLogger(ctx)
	running := false
	controllerReplicas, clientStatus, controllerErr := getAccStatefulSetStatus(ctx, instance, r)
	newStatus.ClientStatus = clientStatus

	newStatus.State = constants.Failed
	log.Infof("statefulSet controllerReplicas [%d]", controllerReplicas)
	log.Infof("statefulSet clientStatus.Available [%s]", clientStatus.Available)

	if fmt.Sprintf("%d", controllerReplicas) == clientStatus.Available {
		running = true
		newStatus.State = constants.Succeeded
	}
	log.Infof("calculate overall state [%s]", newStatus.State)
	var err error
	if controllerErr != nil {
		err = controllerErr
	}
	SetAccStatus(ctx, r, instance, newStatus)
	return running, err
}

// SetStatus of csm
func SetStatus(ctx context.Context, r ReconcileCSM, instance *csmv1.ContainerStorageModule, newStatus *csmv1.ContainerStorageModuleStatus) {

	log := logger.GetLogger(ctx)
	instance.GetCSMStatus().State = newStatus.State
	log.Infow("Driver State", "Controller",
		newStatus.ControllerStatus, "Node", newStatus.NodeStatus)
	instance.GetCSMStatus().ControllerStatus = newStatus.ControllerStatus
	instance.GetCSMStatus().NodeStatus = newStatus.NodeStatus
}

func SetAccStatus(ctx context.Context, r ReconcileCSM, instance *csmv1.ApexConnectivityClient, newStatus *csmv1.ApexConnectivityClientStatus) {

	log := logger.GetLogger(ctx)
	instance.GetApexConnectivityClientStatus().State = newStatus.State
	log.Infow("Apex Client State", "Client",
		newStatus.ClientStatus)
	instance.GetApexConnectivityClientStatus().ClientStatus = newStatus.ClientStatus
}

// UpdateStatus of csm
func UpdateStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, newStatus *csmv1.ContainerStorageModuleStatus) error {
	dMutex.Lock()
	defer dMutex.Unlock()

	log := logger.GetLogger(ctx)
	log.Infow("update current csm status", "status", instance.Status.State)
	statusString := fmt.Sprintf("update new Status: (State - %s)",
		newStatus.State)
	log.Info(statusString)
	log.Infow("Update State", "Controller",
		newStatus.ControllerStatus, "Node", newStatus.NodeStatus)

	_, merr := calculateState(ctx, instance, r, newStatus)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		log := logger.GetLogger(ctx)

		csm := new(csmv1.ContainerStorageModule)
		err := r.GetClient().Get(ctx, t1.NamespacedName{Name: instance.Name,
			Namespace: instance.GetNamespace()}, csm)
		if err != nil {
			return err
		}

		log.Infow("instance - new controller Status", "desired", instance.Status.ControllerStatus.Desired)
		log.Infow("instance - new controller Status", "Available", instance.Status.ControllerStatus.Available)
		log.Infow("instance - new controller Status", "numberUnavailable", instance.Status.ControllerStatus.Failed)
		log.Infow("instance - new controller Status", "State", instance.Status.State)

		csm.Status = instance.Status
		err = r.GetClient().Status().Update(ctx, csm)
		return err
	})
	if err != nil {
		// May be conflict if max retries were hit, or may be something unrelated
		// like permissions or a network error
		log.Error(err, " Failed to update CR status")
		return err
	}
	if err != nil {
		log.Error(err, " Failed to update CR status")
		return err
	}
	log.Info("Update done")
	return merr
}

// UpdateStatus of csm
func UpdateAccStatus(ctx context.Context, instance *csmv1.ApexConnectivityClient, r ReconcileCSM, newStatus *csmv1.ApexConnectivityClientStatus) error {
	dMutex.Lock()
	defer dMutex.Unlock()

	log := logger.GetLogger(ctx)
	log.Infow("update current csm status", "status", instance.Status.State)
	statusString := fmt.Sprintf("update new Status: (State - %s)",
		newStatus.State)
	log.Info(statusString)
	log.Infow("Update State", "Client",
		newStatus.ClientStatus)

	_, merr := calculateAccState(ctx, instance, r, newStatus)
	csm := new(csmv1.ApexConnectivityClient)
	err := r.GetClient().Get(ctx, t1.NamespacedName{Name: instance.Name,
		Namespace: instance.GetNamespace()}, csm)
	if err != nil {
		return err
	}
	csm.Status = instance.Status
	err = r.GetClient().Status().Update(ctx, csm)
	if err != nil {
		log.Error(err, " Failed to update CR status")
		return err
	}
	log.Info("Update done")
	return merr
}

// HandleValidationError for csm
func HandleValidationError(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM,
	validationError error) (reconcile.Result, error) {
	dMutex.Lock()
	defer dMutex.Unlock()
	log := logger.GetLogger(ctx)

	newStatus := instance.GetCSMStatus()
	// Update the status
	newStatus.State = constants.Failed
	err := r.GetClient().Status().Update(ctx, instance)
	if err != nil {
		log.Error(err, "Failed to update CR status HandleValidationError")
	}
	log.Error(validationError, fmt.Sprintf(" *************Create/Update %s failed ********",
		instance.GetDriverType()))
	return LogBannerAndReturn(reconcile.Result{Requeue: false}, validationError)
}

// HandleValidationError for csm
func HandleAccValidationError(ctx context.Context, instance *csmv1.ApexConnectivityClient, r ReconcileCSM,
	validationError error) (reconcile.Result, error) {
	dMutex.Lock()
	defer dMutex.Unlock()
	log := logger.GetLogger(ctx)

	newStatus := instance.GetApexConnectivityClientStatus()
	// Update the status
	newStatus.State = constants.Failed
	err := r.GetClient().Status().Update(ctx, instance)
	if err != nil {
		log.Error(err, "Failed to update CR status HandleValidationError")
	}
	log.Error(validationError, fmt.Sprintf(" *************Create/Update %s failed ********",
		instance.GetClientType()))
	return LogBannerAndReturn(reconcile.Result{Requeue: false}, validationError)
}

// HandleSuccess for csm
func HandleSuccess(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, newStatus, oldStatus *csmv1.ContainerStorageModuleStatus) (reconcile.Result, error) {
	dMutex.Lock()
	defer dMutex.Unlock()

	log := logger.GetLogger(ctx)

	running, err := calculateState(ctx, instance, r, newStatus)
	if err != nil {
		log.Error("HandleSuccess Driver status ", "error", err.Error())
		newStatus.State = constants.Failed
	}
	if running {
		newStatus.State = constants.Running
	}
	log.Infow("HandleSuccess Driver state ", "newStatus.State", newStatus.State)
	if newStatus.State == constants.Running {
		// If previously we were in running state
		if oldStatus.State == constants.Running {
			log.Info("HandleSuccess Driver state didn't change from Running")
		}
		return reconcile.Result{}, nil
	}
	return LogBannerAndReturn(reconcile.Result{}, nil)
}

// HandleSuccess for csm
func HandleAccSuccess(ctx context.Context, instance *csmv1.ApexConnectivityClient, r ReconcileCSM, newStatus, oldStatus *csmv1.ApexConnectivityClientStatus) (reconcile.Result, error) {
	dMutex.Lock()
	defer dMutex.Unlock()

	log := logger.GetLogger(ctx)

	running, err := calculateAccState(ctx, instance, r, newStatus)
	if err != nil {
		log.Error("HandleSuccess ApexConnectivityClient status ", "error", err.Error())
		newStatus.State = constants.Failed
	}
	if running {
		newStatus.State = constants.Running
	}
	log.Infow("HandleSuccess Apex Client state ", "newStatus.State", newStatus.State)
	if newStatus.State == constants.Running {
		// If previously we were in running state
		if oldStatus.State == constants.Running {
			log.Info("HandleSuccess Apex Client state didn't change from Running")
		}
		return reconcile.Result{}, nil
	}
	return LogBannerAndReturn(reconcile.Result{}, nil)
}

// GetNginxControllerStatus - gets deployment status of the NGINX ingress controller
func GetNginxControllerStatus(ctx context.Context, instance csmv1.ContainerStorageModule, r ReconcileCSM) wait.ConditionFunc {
	return func() (bool, error) {
		deployment := &appsv1.Deployment{}
		labelKey := "app.kubernetes.io/name"
		label := "ingress-nginx"
		name := instance.GetNamespace() + "-ingress-nginx-controller"

		err := r.GetClient().Get(ctx, t1.NamespacedName{
			Name:      name,
			Namespace: instance.GetNamespace()}, deployment)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return false, err
			}
			return false, err
		}

		opts := []client.ListOption{
			client.InNamespace(instance.GetNamespace()),
			client.MatchingLabels{labelKey: label},
		}

		deploymentList := &appsv1.DeploymentList{}
		err = r.GetClient().List(ctx, deploymentList, opts...)
		if err != nil {
			return false, err
		}

		for _, deployment := range deploymentList.Items {
			if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
				return true, nil
			}
		}

		return false, err
	}
}

// WaitForNginxController - polls deployment status
func WaitForNginxController(ctx context.Context, instance csmv1.ContainerStorageModule, r ReconcileCSM, timeout time.Duration) error {
	log := logger.GetLogger(ctx)
	log.Infow("Polling status of NGINX ingress controller")

	return wait.PollImmediate(time.Second, timeout, GetNginxControllerStatus(ctx, instance, r))
}
