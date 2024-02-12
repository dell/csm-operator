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

var checkModuleStatus = map[csmv1.ModuleType]func(context.Context, *csmv1.ContainerStorageModule, ReconcileCSM, *csmv1.ContainerStorageModuleStatus) (bool, error){
	csmv1.Observability:       observabilityStatusCheck,
	csmv1.ApplicationMobility: appMobStatusCheck,
}

func getInt32(pointer *int32) int32 {
	if pointer == nil {
		return 0
	}
	return *pointer
}

func getDeploymentStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM) (int32, csmv1.PodStatus, error) {
	log := logger.GetLogger(ctx)
	var msg string
	deployment := &appsv1.Deployment{}
	var err error
	desired := int32(0)
	available := int32(0)
	ready := int32(0)
	numberUnavailable := int32(0)
	totalReplicas := int32(0)

	_, clusterClients, err := GetDefaultClusters(ctx, *instance, r)
	if err != nil {
		return int32(totalReplicas), csmv1.PodStatus{}, err
	}

	for _, cluster := range clusterClients {
		log.Infof("deployment status for cluster: %s", cluster.ClusterID)
		msg += fmt.Sprintf("error message for %s \n", cluster.ClusterID)

		log.Infof("DEBUG: instance being looked at is: %v", instance)
		log.Infof("DEBUG: instance name being looked at is: %v", instance.Name)
		log.Infof("DEBUG: instance spec being looked at is: %v", instance.GetContainerStorageModuleSpec())

		err = cluster.ClusterCTRLClient.Get(ctx, t1.NamespacedName{Name: instance.GetControllerName(),
			Namespace: instance.GetNamespace()}, deployment)
		if err != nil {
			return 0, csmv1.PodStatus{}, err
		}
		log.Infof("Calculating status for deployment: %s", deployment.Name)
		desired = deployment.Status.Replicas
		available = deployment.Status.AvailableReplicas
		ready = deployment.Status.ReadyReplicas
		numberUnavailable = deployment.Status.UnavailableReplicas

		log.Infow("deployment", "desired", desired)
		log.Infow("deployment", "numberReady", ready)
		log.Infow("deployment", "available", available)
		log.Infow("deployment", "numberUnavailable", numberUnavailable)
	}

	return ready, csmv1.PodStatus{
		Available: fmt.Sprintf("%d", available),
		Desired:   fmt.Sprintf("%d", desired),
		Failed:    fmt.Sprintf("%d", numberUnavailable),
	}, err

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

		nodeName := instance.GetNodeName()

		// Application-mobility has a different node name than the drivers
		if instance.GetName() == "application-mobility" {
			log.Infof("Changing nodeName for application-mobility")
			nodeName = "application-mobility-node-agent"
		}

		log.Infof("nodeName is %s", nodeName)
		err := cluster.ClusterCTRLClient.Get(ctx, t1.NamespacedName{Name: nodeName,
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

		//if instance is AM, need to search for different named daemonset
		if instance.GetName() == "application-mobility" {
			log.Infof("Changing labels for application-mobility")
			label = "application-mobility-node-agent"
			opts = []client.ListOption{
				client.InNamespace(instance.GetNamespace()),
				client.MatchingLabels{"name": label},
			}

		}

		log.Infof("Label is %s", label)
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
	running := true
	var err error = nil
	// TODO: Currently commented this block of code as the API used to get the latest deployment status is not working as expected
	// TODO: Can be uncommented once this issues gets sorted out
	controllerReplicas, controllerStatus, controllerErr := getDeploymentStatus(ctx, instance, r)
	if controllerErr != nil {
		log.Infof("eror from getDeploymentStatus: %s", controllerErr.Error())
	}

	newStatus.ControllerStatus = controllerStatus
	expected, nodeStatus, daemonSetErr := getDaemonSetStatus(ctx, instance, r)
	newStatus.NodeStatus = nodeStatus

	newStatus.State = constants.Succeeded
	log.Infof("deployment controllerReplicas [%s]", controllerReplicas)
	log.Infof("deployment controllerStatus.Available [%s]", controllerStatus.Available)

	log.Infof("daemonset expected [%d]", expected)
	log.Infof("daemonset nodeStatus.Available [%s]", nodeStatus.Available)

	if (fmt.Sprintf("%d", controllerReplicas) == controllerStatus.Available) && (fmt.Sprintf("%d", expected) == nodeStatus.Available) {

		for _, module := range instance.Spec.Modules {
			moduleStatus, exists := checkModuleStatus[module.Name]
			if exists && module.Enabled {
				moduleRunning, err := moduleStatus(ctx, instance, r, newStatus)
				if err != nil {
					log.Infof("status for module err msg [%s]", err.Error())
				}

				if !moduleRunning {
					running = false
					newStatus.State = constants.Failed
					log.Infof("%s module not running", module)
					break
				}
			}
		}
	} else {
		running = false
		newStatus.State = constants.Failed
	}

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

// SetAccStatus of csm
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

// UpdateAccStatus of csm
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

// HandleAccValidationError for csm
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

// HandleAccSuccess for csm
func HandleAccSuccess(ctx context.Context, instance *csmv1.ApexConnectivityClient, r ReconcileCSM, newStatus, oldStatus *csmv1.ApexConnectivityClientStatus) (reconcile.Result, error) {
	dMutex.Lock()
	defer dMutex.Unlock()

	log := logger.GetLogger(ctx)

	running, err := calculateAccState(ctx, instance, r, newStatus)
	if err != nil {
		log.Error("HandleSuccess ApexConnectivityClient status ", "error: ", err.Error())
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

// statusForAppMob - calculate success state for application-mobility module
func appMobStatusCheck(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, _ *csmv1.ContainerStorageModuleStatus) (bool, error) {
	log := logger.GetLogger(ctx)
	veleroEnabled := false
	certEnabled := false
	var certManagerRunning bool
	var certManagerCainInjectorRunning bool
	var certManagerWebhookRunning bool
	appMobRunning := false
	veleroRunning := false
	var daemonRunning bool
	var readyPods int
	var notreadyPods int

	for _, m := range instance.Spec.Modules {
		if m.Name == csmv1.ApplicationMobility {
			for _, c := range m.Components {
				if c.Name == "velero" {
					if *c.Enabled {
						veleroEnabled = true
					}
				}
				if c.Name == "cert-manager" {
					if *c.Enabled {
						certEnabled = true
					}
				}

			}
		}
	}

	namespace := instance.GetNamespace()
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}

	deploymentList := &appsv1.DeploymentList{}
	err := r.GetClient().List(ctx, deploymentList, opts...)
	if err != nil {
		return false, err
	}

	checkFn := func(deployment *appsv1.Deployment) bool {
		return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
	}

	for _, deployment := range deploymentList.Items {
		deployment := deployment
		switch deployment.Name {
		case "cert-manager":
			if certEnabled {
				certManagerRunning = checkFn(&deployment)
			}
		case "cert-manager-cainjector":
			if certEnabled {
				certManagerCainInjectorRunning = checkFn(&deployment)
			}
		case "cert-manager-webhook":
			if certEnabled {
				certManagerWebhookRunning = checkFn(&deployment)
			}
		case "application-mobility-controller-manager":
			appMobRunning = checkFn(&deployment)
		case "application-mobility-velero":
			if veleroEnabled {
				veleroRunning = checkFn(&deployment)
			}
		}

	}

	label := "application-mobility-node-agent"
	opts = []client.ListOption{
		client.InNamespace(instance.GetNamespace()),
		client.MatchingLabels{"name": label},
	}

	podList := &corev1.PodList{}
	err = r.GetClient().List(ctx, podList, opts...)
	if err != nil {
		return false, err
	}

	log.Info("podList: %+v\n", podList)

	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			readyPods++
		} else {
			notreadyPods++
		}
	}

	if notreadyPods > 0 {
		daemonRunning = false
	} else {
		daemonRunning = true
	}

	if certEnabled && veleroEnabled {
		return appMobRunning && certManagerRunning && certManagerCainInjectorRunning && certManagerWebhookRunning && veleroRunning && daemonRunning, nil
	}

	if !certEnabled && !veleroEnabled {
		return appMobRunning && daemonRunning, nil
	}

	if !certEnabled && veleroEnabled {
		return appMobRunning && daemonRunning && veleroRunning, nil
	}

	if certEnabled && !veleroEnabled {
		return appMobRunning && certManagerCainInjectorRunning && certManagerRunning && certManagerWebhookRunning && daemonRunning, nil
	}

	return false, nil
}

// observabilityStatusCheck - calculate success state for observability module
func observabilityStatusCheck(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, _ *csmv1.ContainerStorageModuleStatus) (bool, error) {

	topologyEnabled := false
	otelEnabled := false
	certEnabled := false
	metricsEnabled := false
	certManagerRunning := false
	certManagerCainInjectorRunning := false
	certManagerWebhookRunning := false
	otelRunning := false
	metricsRunning := false
	topologyRunning := false

	for _, m := range instance.Spec.Modules {
		if m.Name == csmv1.Observability {
			for _, c := range m.Components {
				if c.Name == "topology" {
					if *c.Enabled {
						topologyEnabled = true
					}
				}
				if c.Name == "otel-collector" {
					if *c.Enabled {
						otelEnabled = true
					}
				}
				if c.Name == "cert-manager" {
					if *c.Enabled {
						certEnabled = true
					}
				}
				if c.Name == fmt.Sprintf("metrics-%s", instance.Spec.Driver.CSIDriverType) {
					if *c.Enabled {
						metricsEnabled = true
					}
				}
			}
		}
	}

	namespace := "karavi"
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}
	deploymentList := &appsv1.DeploymentList{}
	err := r.GetClient().List(ctx, deploymentList, opts...)
	if err != nil {
		return false, err
	}

	checkFn := func(deployment *appsv1.Deployment) bool {
		return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
	}

	for _, deployment := range deploymentList.Items {
		deployment := deployment
		switch deployment.Name {
		case "otel-collector":
			if otelEnabled {
				otelRunning = checkFn(&deployment)
			}
		case fmt.Sprintf("%s-metrics-%s", namespace, instance.Spec.Driver.CSIDriverType):
			if metricsEnabled {
				metricsRunning = checkFn(&deployment)
			}
		case fmt.Sprintf("%s-topology", namespace):
			if topologyEnabled {
				topologyRunning = checkFn(&deployment)
			}
		}
	}

	namespaceCert := instance.GetNamespace()
	opts = []client.ListOption{
		client.InNamespace(namespaceCert),
	}

	deploymentCertList := &appsv1.DeploymentList{}
	err = r.GetClient().List(ctx, deploymentCertList, opts...)
	if err != nil {
		return false, err
	}

	for _, deployment := range deploymentCertList.Items {
		deployment := deployment
		switch deployment.Name {
		case "cert-manager":
			if certEnabled {
				certManagerRunning = checkFn(&deployment)
			}
		case "cert-manager-cainjector":
			if certEnabled {
				certManagerCainInjectorRunning = checkFn(&deployment)
			}
		case "cert-manager-webhook":
			if certEnabled {
				certManagerWebhookRunning = checkFn(&deployment)
			}
		}
	}

	if certEnabled && otelEnabled && metricsEnabled && topologyEnabled {
		return certManagerRunning && certManagerCainInjectorRunning && certManagerWebhookRunning && otelRunning && metricsRunning && topologyRunning, nil
	}

	if !certEnabled && otelEnabled && metricsEnabled && topologyEnabled {
		return otelRunning && metricsRunning && topologyRunning, nil
	}

	if certEnabled && otelEnabled && metricsEnabled && !topologyEnabled {
		return certManagerRunning && certManagerCainInjectorRunning && certManagerWebhookRunning && otelRunning && metricsRunning, nil
	}

	if !certEnabled && otelEnabled && metricsEnabled && !topologyEnabled {
		return otelRunning && metricsRunning, nil
	}

	if certEnabled && metricsEnabled && !topologyEnabled && !otelEnabled {
		return certManagerRunning && certManagerCainInjectorRunning && certManagerWebhookRunning && metricsRunning, nil
	}

	if !certEnabled && metricsEnabled && !topologyEnabled && !otelEnabled {
		return metricsRunning, nil
	}

	return false, nil
}
