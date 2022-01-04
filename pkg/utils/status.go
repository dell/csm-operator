package utils

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/constants"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func getInt32(pointer *int32) int32 {
	if pointer == nil {
		return 0
	}
	return *pointer
}

func getDeploymentStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM) (int32, csmv1.PodStatus, error) {
	var available, ready, starting, stopped []string
	deployment := &appsv1.Deployment{}
	err := r.GetClient().Get(ctx, types.NamespacedName{Name: instance.GetControllerName(),
		Namespace: instance.GetNamespace()}, deployment)
	if err != nil {
		return 0, csmv1.PodStatus{}, err
	}
	replicas := getInt32(deployment.Spec.Replicas)
	readyCount := deployment.Status.ReadyReplicas
	if replicas == 0 || readyCount == 0 {
		stopped = append(stopped, instance.GetControllerName())
	} else {
		podList := &v1.PodList{}
		opts := []client.ListOption{
			client.InNamespace(instance.GetNamespace()),
			client.MatchingLabels{"app": instance.GetNodeName()},
		}
		err = r.GetClient().List(ctx, podList, opts...)
		if err != nil {
			return replicas, csmv1.PodStatus{}, err
		}
		for _, pod := range podList.Items {
			if pod.Status.Phase == corev1.PodRunning {
				running := true
				for _, containerStatus := range pod.Status.ContainerStatuses {
					if containerStatus.State.Running == nil {
						running = false
						break
					}
				}
				if running {
					available = append(available, pod.Name)
				} else {
					ready = append(ready, pod.Name)
				}
			} else if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodUnknown || pod.Status.Phase == corev1.PodRunning {
				starting = append(starting, pod.Name)
			} else if pod.Status.Phase == corev1.PodFailed {
				stopped = append(stopped, pod.Name)
			}
		}
	}
	return replicas, csmv1.PodStatus{
		Available: available,
		Stopped:   stopped,
		Starting:  starting,
		Ready:     ready,
	}, nil
}

func getDaemonSetStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM) (int32, csmv1.PodStatus, error) {
	var available, ready, starting, stopped []string
	node := &appsv1.DaemonSet{}
	err := r.GetClient().Get(ctx, types.NamespacedName{Name: instance.GetNodeName(),
		Namespace: instance.GetNamespace()}, node)
	if err != nil {
		return 0, csmv1.PodStatus{}, err
	}
	if node.Status.DesiredNumberScheduled == 0 || node.Status.NumberReady == 0 {
		stopped = append(stopped, instance.GetNodeName())
	} else {
		podList := &v1.PodList{}
		opts := []client.ListOption{
			client.InNamespace(instance.GetNamespace()),
			client.MatchingLabels{"app": instance.GetNodeName()},
		}
		err = r.GetClient().List(ctx, podList, opts...)
		if err != nil {
			return node.Status.DesiredNumberScheduled, csmv1.PodStatus{}, err
		}
		for _, pod := range podList.Items {
			if podutil.IsPodAvailable(&pod, node.Spec.MinReadySeconds, metav1.Now()) {
				available = append(available, pod.Name)
			} else if podutil.IsPodReady(&pod) {
				ready = append(ready, pod.Name)
			} else {
				starting = append(starting, pod.Name)
			}
		}
	}
	return node.Status.DesiredNumberScheduled, csmv1.PodStatus{
		Available: available,
		Stopped:   stopped,
		Starting:  starting,
		Ready:     ready,
	}, nil
}

// CalculateState
func CalculateState(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, newStatus *csmv1.ContainerStorageModuleStatus) (bool, error) {
	running := false
	controllerReplicas, controllerStatus, controllerErr := getDeploymentStatus(ctx, instance, r)
	newStatus.ControllerStatus = controllerStatus
	expected, nodeStatus, daemonSetErr := getDaemonSetStatus(ctx, instance, r)
	newStatus.NodeStatus = nodeStatus
	if ((controllerReplicas != 0) && (controllerReplicas == int32(len(controllerStatus.Available)))) && ((expected != 0) && (expected == int32(len(nodeStatus.Available)))) {
		// Even if there is an error message, it is okay to overwrite that as all the pods are in running state
		running = true
	}
	var err error
	if controllerErr != nil {
		if daemonSetErr != nil {
			err = fmt.Errorf("controllerError: %s, daemonseterror: %s", controllerErr.Error(), daemonSetErr.Error())
		} else {
			err = controllerErr
		}
	} else {
		if daemonSetErr != nil {
			err = daemonSetErr
		} else {
			err = nil
		}
	}
	return running, err
}

// SetStatus
func SetStatus(instance *csmv1.ContainerStorageModule, newStatus *csmv1.ContainerStorageModuleStatus) {
	instance.GetCSMStatus().State = newStatus.State
	instance.GetCSMStatus().LastUpdate.ErrorMessage = newStatus.LastUpdate.ErrorMessage
	instance.GetCSMStatus().LastUpdate.Condition = newStatus.LastUpdate.Condition
	instance.GetCSMStatus().LastUpdate.Time = newStatus.LastUpdate.Time
	instance.GetCSMStatus().ControllerStatus = newStatus.ControllerStatus
	instance.GetCSMStatus().NodeStatus = newStatus.NodeStatus
	instance.GetCSMStatus().ContainerStorageModuleHash = newStatus.ContainerStorageModuleHash
}

// SetLastStatusUpdate
func SetLastStatusUpdate(status *csmv1.ContainerStorageModuleStatus, conditionType csmv1.CSMOperatorConditionType, errorMsg string) csmv1.LastUpdate {
	// If the condition has not changed, then don't update the time
	if status.LastUpdate.Condition == conditionType && status.LastUpdate.ErrorMessage == errorMsg {
		return csmv1.LastUpdate{
			Condition:    conditionType,
			ErrorMessage: errorMsg,
			Time:         status.LastUpdate.Time,
		}
	}
	return csmv1.LastUpdate{
		Condition:    conditionType,
		ErrorMessage: errorMsg,
		Time:         metav1.Now(),
	}
}

// UpdateStatus
func UpdateStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, reqLogger logr.Logger, newStatus, oldStatus *csmv1.ContainerStorageModuleStatus) error {
	//running := calculateState(ctx, instance, r, newStatus)
	if !reflect.DeepEqual(oldStatus, newStatus) {
		statusString := fmt.Sprintf("Status: (State - %s, Error Message - %s, Driver Hash - %d)",
			newStatus.State, newStatus.LastUpdate.ErrorMessage, newStatus.ContainerStorageModuleHash)
		reqLogger.Info(statusString)
		reqLogger.Info("State", "Controller",
			newStatus.ControllerStatus, "Node", newStatus.NodeStatus)
		SetStatus(instance, newStatus)
		reqLogger.Info("Attempting to update CR status")
		err := r.GetClient().Status().Update(ctx, instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update CR status")
			return err
		}
		reqLogger.Info("Successfully updated CR status")
	} else {
		reqLogger.Info("No change to status. No updates will be applied to CR status")
	}
	return nil
}

// HandleValidationError
func HandleValidationError(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, reqLogger logr.Logger,
	validationError error) (reconcile.Result, error) {
	reqLogger.Error(validationError, "Validation error")
	status := instance.GetCSMStatus()
	oldStatus := status.DeepCopy()
	newStatus := status.DeepCopy()
	// Update the status
	reqLogger.Info("Marking the driver status as InvalidConfig")
	_, _ = CalculateState(ctx, instance, r, newStatus)
	newStatus.LastUpdate = SetLastStatusUpdate(oldStatus, csmv1.InvalidConfig, validationError.Error())
	newStatus.State = constants.InvalidConfig
	_ = UpdateStatus(ctx, instance, r, reqLogger, newStatus, oldStatus)
	reqLogger.Error(validationError, fmt.Sprintf("*************Create/Update %s failed ********",
		instance.GetDriverType()))
	return LogBannerAndReturn(reconcile.Result{Requeue: false}, nil, reqLogger)
}

// GetOperatorConditionTypeFromState - Returns operator condition type
func GetOperatorConditionTypeFromState(state csmv1.CSMStateType) csmv1.CSMOperatorConditionType {
	switch state {
	case constants.Succeeded:
		return csmv1.Succeeded
	case constants.Running:
		return csmv1.Running
	case constants.InvalidConfig:
		return csmv1.InvalidConfig
	case constants.Updating:
		return csmv1.Updating
	case constants.Failed:
		return csmv1.Failed
	}
	return csmv1.Error
}

// HandleSuccess
func HandleSuccess(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, reqLogger logr.Logger, newStatus, oldStatus *csmv1.ContainerStorageModuleStatus) (reconcile.Result, error) {
	errorMsg := ""
	running, err := CalculateState(ctx, instance, r, newStatus)
	if err != nil {
		errorMsg = err.Error()
	}
	if running {
		newStatus.State = constants.Running
	} else if err != nil {
		newStatus.State = constants.Updating
	} else {
		newStatus.State = constants.Succeeded
	}
	newStatus.LastUpdate = SetLastStatusUpdate(oldStatus,
		GetOperatorConditionTypeFromState(newStatus.State), errorMsg)
	retryInterval := constants.DefaultRetryInterval
	requeue := true
	if newStatus.State == constants.Running {
		// If previously we were in running state
		if oldStatus.State == constants.Running {
			requeue = false
			reqLogger.Info("Driver state didn't change from Running")
		}
	} else if newStatus.State == constants.Succeeded {
		if oldStatus.State == constants.Running {
			// We went back to succeeded from running
			reqLogger.Info("Driver migrated from Running state to Succeeded state")
		} else if oldStatus.State == constants.Succeeded {
			timeSinceLastConditionChange := metav1.Now().Sub(oldStatus.LastUpdate.Time.Time).Round(time.Millisecond)
			reqLogger.Info(fmt.Sprintf("Time since last condition change: %v", timeSinceLastConditionChange))
			if timeSinceLastConditionChange >= constants.MaxRetryDuration {
				// Don't requeue again
				requeue = false
				reqLogger.Info("Time elapsed since last condition change is more than max limit. Not going to reconcile")
			} else {
				// set to the default retry interval at minimum
				retryInterval = time.Duration(math.Max(float64(timeSinceLastConditionChange.Nanoseconds()*2),
					float64(constants.DefaultRetryInterval)))
				// Maximum set to MaxRetryInterval
				retryInterval = time.Duration(math.Min(float64(retryInterval), float64(constants.MaxRetryInterval)))
			}
		}
	} else {
		requeue = true
	}
	updateStatusError := UpdateStatus(ctx, instance, r, reqLogger, newStatus, oldStatus)
	if updateStatusError != nil {
		reqLogger.Error(updateStatusError, "failed to update the status")
		// Don't return error as controller runtime will immediately requeue the request
		return LogBannerAndReturn(reconcile.Result{Requeue: true, RequeueAfter: retryInterval}, nil, reqLogger)
	}
	if requeue {
		reqLogger.Info(fmt.Sprintf("Requeue interval: %v", retryInterval))
		return LogBannerAndReturn(reconcile.Result{Requeue: true, RequeueAfter: retryInterval}, nil, reqLogger)
	}
	return LogBannerAndReturn(reconcile.Result{}, nil, reqLogger)
}
