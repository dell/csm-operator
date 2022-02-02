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
	"sync"
)

var reqLogger logr.Logger
var dMutex sync.RWMutex

func getInt32(pointer *int32) int32 {
	if pointer == nil {
		return 0
	}
	return *pointer
}

func getDeploymentStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM) (int32, csmv1.PodStatus, error) {
	deployment := &appsv1.Deployment{}
	err := r.GetClient().Get(ctx, types.NamespacedName{Name: instance.GetControllerName(),
		Namespace: instance.GetNamespace()}, deployment)
	if err != nil {
		return 0, csmv1.PodStatus{}, err
	}
	replicas := getInt32(deployment.Spec.Replicas)
	readyPods := 0
	failedCount := 0
	reqLogger.Info("==============")
	reqLogger.Info("deployment", "status", deployment.Status)
	reqLogger.Info("==============")

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

		reqLogger.Info("==============")
		reqLogger.Info("deployment pod", "count", readyPods, "name", pod.Name, "status", pod.Status.Phase)
		reqLogger.Info("==============")

		if pod.Status.Phase == corev1.PodRunning {
			readyPods++
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.State.Running == nil {
					readyPods--
					failedCount++
					break
				}
			}
		} else {
			failedCount++
		}
	}

	return replicas, csmv1.PodStatus{
		Available: fmt.Sprintf("%d", readyPods),
		Desired:   fmt.Sprintf("%d", replicas),
		Failed:    fmt.Sprintf("%d", failedCount),
	}, nil
}

func getDaemonSetStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM) (int32, csmv1.PodStatus, error) {
	ds := &appsv1.DaemonSet{}
	err := r.GetClient().Get(ctx, types.NamespacedName{Name: instance.GetNodeName(),
		Namespace: instance.GetNamespace()}, ds)
	if err != nil {
		return 0, csmv1.PodStatus{}, err
	}
	faliedCount := 0
	podList := &v1.PodList{}
	opts := []client.ListOption{
		client.InNamespace(instance.GetNamespace()),
		client.MatchingLabels{"app": instance.GetNodeName()},
	}
	err = r.GetClient().List(ctx, podList, opts...)
	if err != nil {
		return ds.Status.DesiredNumberScheduled, csmv1.PodStatus{}, err
	}
	msg := "Pods ok"
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning {
			faliedCount++
			msg += "Pod not running " + pod.Name
		}
	}
	if faliedCount > 0 {
		err = errors.New(msg)
	}
	return ds.Status.DesiredNumberScheduled, csmv1.PodStatus{
		Available: fmt.Sprintf("%d", ds.Status.NumberAvailable),
		Desired:   fmt.Sprintf("%d", ds.Status.DesiredNumberScheduled),
		Failed:    fmt.Sprintf("%d", faliedCount),
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
	reqLogger.Info("controller", "replicas count", controllerReplicas)
	reqLogger.Info("controller", "controllerStatus.Available", controllerStatus.Available)

	reqLogger.Info("node pods", "expected", expected)
	reqLogger.Info("node pods", "nodeStatus.Available", nodeStatus.Available)

	if (fmt.Sprintf("%d", controllerReplicas) == controllerStatus.Available) && (fmt.Sprintf("%d", expected) == nodeStatus.Available) {
		running = true
		newStatus.State = constants.Succeeded
	}
	reqLogger.Info("csm", "calculate state", newStatus.State)
	var err error
	if controllerErr != nil {
		err = controllerErr
	}
	if daemonSetErr != nil {
		err = daemonSetErr
	}
	if daemonSetErr != nil && controllerErr != nil {
		err = fmt.Errorf("controllerError: %s, daemonseterror: %s", controllerErr.Error(), daemonSetErr.Error())
	}
	SetStatus(instance, newStatus, reqLogger)
	return running, err
}

// SetStatus of csm
func SetStatus(instance *csmv1.ContainerStorageModule, newStatus *csmv1.ContainerStorageModuleStatus, reqLogger logr.Logger) {
	instance.GetCSMStatus().State = newStatus.State
	reqLogger.Info("State", "Controller",
		newStatus.ControllerStatus, "Node", newStatus.NodeStatus)
	instance.GetCSMStatus().ControllerStatus = newStatus.ControllerStatus
	instance.GetCSMStatus().NodeStatus = newStatus.NodeStatus
}

// UpdateStatus of csm
func UpdateStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, reqLogger logr.Logger, newStatus *csmv1.ContainerStorageModuleStatus) error {
	dMutex.Lock()
	defer dMutex.Unlock()
	reqLogger.Info("current csm status", "status", instance.Status.State)
	reqLogger.Info("new status", "state", newStatus.State)

	SetStatus(instance, newStatus, reqLogger)

	if newStatus.State == constants.Succeeded {
		running, err := CalculateState(ctx, instance, r, newStatus)
		if err != nil {
			reqLogger.Info("Driver status ", "error", err.Error())
			newStatus.State = constants.Failed
		}
		reqLogger.Info("Attempting to update CR status", "running", running)
	}
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
	err := UpdateStatus(ctx, instance, r, reqLogger, newStatus)
	if err != nil {
		reqLogger.Error(err, "Failed to update CR status")
	}
	reqLogger.Error(validationError, fmt.Sprintf("*************Create/Update %s failed ********",
		instance.GetDriverType()))
	return LogBannerAndReturn(reconcile.Result{Requeue: false}, nil, reqLogger)
}

// HandleSuccess for csm
func HandleSuccess(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, log logr.Logger, newStatus, oldStatus *csmv1.ContainerStorageModuleStatus) (reconcile.Result, error) {
	reqLogger = log
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
