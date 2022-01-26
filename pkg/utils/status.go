package utils

import (
	"context"
	"errors"
	"fmt"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/constants"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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
		//app=test-isilon-controller
		label := instance.GetNamespace() + "-controller"
		podList := &v1.PodList{}
		opts := []client.ListOption{
			client.InNamespace(instance.GetNamespace()),
			client.MatchingLabels{"app": label},
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
	failedCount := 0
	if len(ready) != len(available) {
		failedCount = len(stopped)
	}
	return replicas, csmv1.PodStatus{
		Available: "available=" + fmt.Sprintf("%d", readyCount),
		Desired:   "desired=" + fmt.Sprintf("%d", replicas),
		Failed:    "failed=" + fmt.Sprintf("%d", failedCount),
	}, nil
}

func getDaemonSetStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM) (int32, csmv1.PodStatus, error) {
	var available, starting, stopped []string
	node := &appsv1.DaemonSet{}
	err := r.GetClient().Get(ctx, types.NamespacedName{Name: instance.GetNodeName(),
		Namespace: instance.GetNamespace()}, node)
	if err != nil {
		return 0, csmv1.PodStatus{}, err
	}
	if node.Status.DesiredNumberScheduled == 0 || node.Status.NumberReady == 0 {
		stopped = append(stopped, instance.GetNodeName())
		err = errors.New("Pod stopped")
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
			if pod.Status.Phase == "Pending" {
				starting = append(starting, pod.Name)
				err = errors.New("Pod starting ")
			} else {
				available = append(available, pod.Name)
			}
		}
	}

	return node.Status.DesiredNumberScheduled, csmv1.PodStatus{
		Available: "available=" + fmt.Sprintf("%d", node.Status.NumberAvailable),
		Desired:   "desired=" + fmt.Sprintf("%d", node.Status.DesiredNumberScheduled),
		Failed:    "failed=" + fmt.Sprintf("%d", len(starting)),
	}, err
}

// CalculateState of pods
func CalculateState(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, newStatus *csmv1.ContainerStorageModuleStatus) (bool, error) {
	running := false
	controllerReplicas, controllerStatus, controllerErr := getDeploymentStatus(ctx, instance, r)
	newStatus.ControllerStatus = controllerStatus
	expected, nodeStatus, daemonSetErr := getDaemonSetStatus(ctx, instance, r)
	newStatus.NodeStatus = nodeStatus

	newStatus.State = constants.Failed
	if ((controllerReplicas != 0) && (string(controllerReplicas) == controllerStatus.Available)) && ((expected != 0) && (string(expected) == nodeStatus.Available)) {
		running = true
		newStatus.State = constants.Succeeded
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
	SetStatus(instance, newStatus, nil)
	return running, err
}

// SetStatus of csm
func SetStatus(instance *csmv1.ContainerStorageModule, newStatus *csmv1.ContainerStorageModuleStatus, reqLogger logr.Logger) {
	instance.GetCSMStatus().State = newStatus.State
	if reqLogger != nil {
		reqLogger.Info("State", "Controller",
			newStatus.ControllerStatus, "Node", newStatus.NodeStatus)
	}
	instance.GetCSMStatus().ControllerStatus = newStatus.ControllerStatus
	instance.GetCSMStatus().NodeStatus = newStatus.NodeStatus
}

// UpdateStatus of csm
func UpdateStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, reqLogger logr.Logger, newStatus *csmv1.ContainerStorageModuleStatus) error {
	statusString := fmt.Sprintf("Status: (State - %s)",
		newStatus.State)
	reqLogger.Info(statusString)
	reqLogger.Info("State", "Controller",
		newStatus.ControllerStatus, "Node", newStatus.NodeStatus)
	SetStatus(instance, newStatus, reqLogger)
	reqLogger.Info("Attempting to update CR status", "status", instance.Status.State)
	err := r.GetClient().Status().Update(ctx, instance)
	if err != nil {
		reqLogger.Error(err, "Failed to update CR status")
		return err
	}
	reqLogger.Info("updated CR status ok")
	return nil
}

// HandleValidationError for csm
func HandleValidationError(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, reqLogger logr.Logger,
	validationError error) (reconcile.Result, error) {
	reqLogger.Error(validationError, "Validation error")
	newStatus := instance.GetCSMStatus()
	// Update the status
	reqLogger.Info("Marking the driver status as InvalidConfig")
	newStatus.State = constants.Failed
	_ = UpdateStatus(ctx, instance, r, reqLogger, newStatus)
	reqLogger.Error(validationError, fmt.Sprintf("*************Create/Update %s failed ********",
		instance.GetDriverType()))
	return LogBannerAndReturn(reconcile.Result{Requeue: false}, nil, reqLogger)
}

// HandleSuccess for csm
func HandleSuccess(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, reqLogger logr.Logger, newStatus, oldStatus *csmv1.ContainerStorageModuleStatus) (reconcile.Result, error) {
	running, err := CalculateState(ctx, instance, r, newStatus)
	if err != nil {
		reqLogger.Info("Driver status ", "error", err.Error())
	}
	if running {
		newStatus.State = constants.Running
	}
	if err != nil {
		newStatus.State = constants.Failed
	}
	reqLogger.Info("Driver state ", "newStatus.State", newStatus.State)
	if newStatus.State == constants.Running {
		// If previously we were in running state
		if oldStatus.State == constants.Running {
			reqLogger.Info("Driver state didn't change from Running")
		}
		return LogBannerAndReturn(reconcile.Result{}, nil, reqLogger)
	}
	updateStatusError := UpdateStatus(ctx, instance, r, reqLogger, newStatus)
	if updateStatusError != nil {
		reqLogger.Error(updateStatusError, "failed to update the status")
		return LogBannerAndReturn(reconcile.Result{Requeue: true}, updateStatusError, reqLogger)
	}
	return LogBannerAndReturn(reconcile.Result{}, nil, reqLogger)
}
