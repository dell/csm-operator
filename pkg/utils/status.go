package utils

import (
	"context"
	"errors"
	"fmt"
	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/constants"
	"github.com/dell/csm-operator/pkg/logger"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	t1 "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	"sync"
)

var dMutex sync.RWMutex

func getInt32(pointer *int32) int32 {
	if pointer == nil {
		return 0
	}
	return *pointer
}

func getDeploymentStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM) (int32, csmv1.PodStatus, error) {
	deployment := &appsv1.Deployment{}
	log := logger.GetLogger(ctx)

	err := r.GetClient().Get(ctx, t1.NamespacedName{Name: instance.GetControllerName(),
		Namespace: instance.GetNamespace()}, deployment)
	if err != nil {
		return 0, csmv1.PodStatus{}, err
	}
	replicas := getInt32(deployment.Spec.Replicas)
	readyPods := 0
	failedCount := 0

	//app=test-isilon-controller
	label := instance.GetNamespace() + "-controller"
	opts := []client.ListOption{
		client.InNamespace(instance.GetNamespace()),
		client.MatchingLabels{"app": label},
	}

	podList := &v1.PodList{}
	err = r.GetClient().List(ctx, podList, opts...)
	if err != nil {
		return deployment.Status.ReadyReplicas, csmv1.PodStatus{}, err
	}

	var msg string
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
	if failedCount > 0 {
		err = errors.New(msg)
	}

	return replicas, csmv1.PodStatus{
		Available: fmt.Sprintf("%d", readyPods),
		Desired:   fmt.Sprintf("%d", replicas),
		Failed:    fmt.Sprintf("%d", failedCount),
	}, err
}

func getDaemonSetStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM) (int32, csmv1.PodStatus, error) {
	log := logger.GetLogger(ctx)

	ds := &appsv1.DaemonSet{}
	err := r.GetClient().Get(ctx, t1.NamespacedName{Name: instance.GetNodeName(),
		Namespace: instance.GetNamespace()}, ds)
	if err != nil {
		return 0, csmv1.PodStatus{}, err
	}

	failedCount := 0
	podList := &v1.PodList{}
	label := instance.GetNamespace() + "-node"
	opts := []client.ListOption{
		client.InNamespace(instance.GetNamespace()),
		client.MatchingLabels{"app": label},
	}

	err = r.GetClient().List(ctx, podList, opts...)
	if err != nil {
		return ds.Status.DesiredNumberScheduled, csmv1.PodStatus{}, err
	}

	var msg string
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
	if failedCount > 0 {
		err = errors.New(msg)
	}
	return ds.Status.DesiredNumberScheduled, csmv1.PodStatus{
		Available: fmt.Sprintf("%d", ds.Status.NumberAvailable),
		Desired:   fmt.Sprintf("%d", ds.Status.DesiredNumberScheduled),
		Failed:    fmt.Sprintf("%d", failedCount),
	}, err
}

// CalculateState of pods
func CalculateState(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, newStatus *csmv1.ContainerStorageModuleStatus) (bool, error) {
	log := logger.GetLogger(ctx)
	running := false
	controllerReplicas, controllerStatus, controllerErr := getDeploymentStatus(ctx, instance, r)
	newStatus.ControllerStatus = controllerStatus
	expected, nodeStatus, daemonSetErr := getDaemonSetStatus(ctx, instance, r)
	newStatus.NodeStatus = nodeStatus

	newStatus.State = constants.Failed
	log.Infof("deployment controllerReplicas [%d]", controllerReplicas)
	log.Infof("deployment controllerStatus.Available [%s]", controllerStatus.Available)

	log.Infof("daemonset expected [%d]", expected)
	log.Infof("daemonset nodeStatus.Available [%s]", nodeStatus.Available)

	if (fmt.Sprintf("%d", controllerReplicas) == controllerStatus.Available) && (fmt.Sprintf("%d", expected) == nodeStatus.Available) {
		running = true
		newStatus.State = constants.Succeeded
	}
	log.Infof("calculate overall state [%s]", newStatus.State)
	var err error
	if controllerErr != nil {
		err = controllerErr
	}
	if daemonSetErr != nil {
		err = daemonSetErr
	}
	if daemonSetErr != nil && controllerErr != nil {
		err = fmt.Errorf("ControllerError: %s, Daemonseterror: %s", controllerErr.Error(), daemonSetErr.Error())
		log.Infof("calculate overall error msg [%s]", err.Error())
	}
	SetStatus(ctx, r, instance, newStatus)
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

	_, merr := CalculateState(ctx, instance, r, newStatus)
	csm := new(csmv1.ContainerStorageModule)
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
	log := logger.GetLogger(ctx)

	log.Error(validationError, "Validation error")
	newStatus := instance.GetCSMStatus()
	// Update the status
	log.Info("Marking the driver status as InvalidConfig")
	newStatus.State = constants.Failed
	err := UpdateStatus(ctx, instance, r, newStatus)
	if err != nil {
		log.Error(err, "Failed to update CR status HandleValidationError")
	}
	log.Error(validationError, fmt.Sprintf("*************Create/Update %s failed ********",
		instance.GetDriverType()))
	return LogBannerAndReturn(reconcile.Result{Requeue: false}, nil)
}

// HandleSuccess for csm
func HandleSuccess(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, newStatus, oldStatus *csmv1.ContainerStorageModuleStatus) (reconcile.Result, error) {
	dMutex.Lock()
	defer dMutex.Unlock()

	log := logger.GetLogger(ctx)

	running, err := CalculateState(ctx, instance, r, newStatus)
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
