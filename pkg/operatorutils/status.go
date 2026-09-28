//  Copyright © 2021 - 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package operatorutils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/constants"
	"github.com/dell/csm-operator/pkg/logger"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	t1 "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var dMutex sync.RWMutex

const (
	MetadataPrefix        = "storage.dell.com"
	configVersionKey      = MetadataPrefix + "/CSMOperatorConfigVersion"
	preUpgradeSnapshotKey = MetadataPrefix + "/PreUpgradeSnapshot"
)

var checkModuleStatus = map[csmv1.ModuleType]func(context.Context, *csmv1.ContainerStorageModule, ReconcileCSM, *csmv1.ContainerStorageModuleStatus, OperatorConfig) (bool, error){
	csmv1.Observability:       observabilityStatusCheck,
	csmv1.AuthorizationServer: authProxyStatusCheck,
}

// NormalStartupReasons are container waiting reasons that indicate the pod is
// still starting up normally. Pods with these reasons are never counted as
// failed. Any other waiting reason is treated as a potential failure; the
// grace period (FailureGracePeriod) determines when the CR transitions to Failed.
var NormalStartupReasons = map[string]bool{
	"ContainerCreating": true,
	"PodInitializing":   true,
}

// FailureGracePeriod is the duration after first failure detection before the
// CR state transitions from Pending to Failed. This avoids marking a CR as
// Failed during transient issues like brief CrashLoopBackOff during rolling
// updates or slow image pulls.
// Configurable via the FAILURE_GRACE_PERIOD environment variable (e.g. "10m", "5m30s").
// Defaults to 10 minutes if not set or invalid.
var FailureGracePeriod = 10 * time.Minute

func init() {
	if val := os.Getenv("FAILURE_GRACE_PERIOD"); val != "" {
		if d, err := time.ParseDuration(val); err == nil && d > 0 {
			FailureGracePeriod = d
		}
	}
}

var (
	firstFailureObserved    = make(map[string]time.Time)
	firstFailureObservedMux sync.Mutex
)

// checkFailureGracePeriod returns true if the grace period has elapsed for the
// given CR key (namespace/name). On first call it records the current time.
func checkFailureGracePeriod(key string) bool {
	firstFailureObservedMux.Lock()
	defer firstFailureObservedMux.Unlock()
	if t, ok := firstFailureObserved[key]; ok {
		return time.Since(t) >= FailureGracePeriod
	}
	firstFailureObserved[key] = time.Now()
	return false
}

// ClearFailureGracePeriod removes the failure tracking for a CR when pods recover.
func ClearFailureGracePeriod(key string) {
	firstFailureObservedMux.Lock()
	defer firstFailureObservedMux.Unlock()
	delete(firstFailureObserved, key)
}

// SucceededStabilityPeriod is the duration after first Succeeded detection before
// the CR state transitions from Pending to Succeeded. This avoids marking a CR as
// Succeeded during transient issues like brief status blips during rolling updates.
var SucceededStabilityPeriod = 10 * time.Second

var (
	firstSucceededObserved    = make(map[string]time.Time)
	firstSucceededObservedMux sync.Mutex
)

// checkSucceededStabilityPeriod returns true if the stability period has elapsed for the
// given CR key (namespace/name). On first call it records the current time.
func checkSucceededStabilityPeriod(key string) bool {
	firstSucceededObservedMux.Lock()
	defer firstSucceededObservedMux.Unlock()
	if t, ok := firstSucceededObserved[key]; ok {
		return time.Since(t) >= SucceededStabilityPeriod
	}
	firstSucceededObserved[key] = time.Now()
	return false
}

// IsStabilityPeriodPending returns true if a stability period entry exists for the
// given CR key but has NOT yet elapsed. This is a read-only query with no side-effects
// (unlike checkSucceededStabilityPeriod which records a timestamp on first call).
// Returns false if no entry exists or if the period has already elapsed.
func IsStabilityPeriodPending(key string) bool {
	firstSucceededObservedMux.Lock()
	defer firstSucceededObservedMux.Unlock()
	t, ok := firstSucceededObserved[key]
	if !ok {
		return false
	}
	return time.Since(t) < SucceededStabilityPeriod
}

// ClearSucceededStabilityPeriod removes the Succeeded tracking for a CR when status
// changes away from Succeeded (e.g., pods become unhealthy).
func ClearSucceededStabilityPeriod(key string) {
	firstSucceededObservedMux.Lock()
	defer firstSucceededObservedMux.Unlock()
	delete(firstSucceededObserved, key)
}

// calculates deployment state of drivers only; module deployment status will be checked in checkModuleStatus
func getDeploymentStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM) (csmv1.PodStatus, error) {
	log := logger.GetLogger(ctx)
	deployment := &appsv1.Deployment{}
	var err error
	desired := int32(0)
	available := int32(0)
	ready := int32(0)
	numberUnavailable := int32(0)
	emptyStatus := csmv1.PodStatus{
		Available: "0",
		Desired:   "0",
		Failed:    "0",
	}

	clusterClient := GetCluster(ctx, r)
	if err != nil {
		return emptyStatus, err
	}

	log.Infof("getting deployment status for cluster: %s", clusterClient.ClusterID)

	if instance.GetName() == "" {
		log.Infof("Not a driver instance, will not check deploymentstatus")
		return emptyStatus, nil
	}

	if isAuthorizationProxyServer(instance) {
		log.Infof("Calculating status for authorization proxy server deployments and statefulsets")
		return getAuthProxyDeploymentStatus(ctx, instance, r)
	}

	err = clusterClient.ClusterCTRLClient.Get(ctx, t1.NamespacedName{
		Name:      instance.GetControllerName(),
		Namespace: instance.GetNamespace(),
	}, deployment)
	if err != nil {
		return emptyStatus, err
	}
	log.Infof("Calculating status for deployment: %s", deployment.Name)
	desired = deployment.Status.Replicas
	available = deployment.Status.AvailableReplicas
	ready = deployment.Status.ReadyReplicas
	numberUnavailable = deployment.Status.UnavailableReplicas

	log.Debugf("[deployment] desired: %d, numberReady: %d, available: %d, numberUnavailable: %d", desired, ready, available, numberUnavailable)

	// List pods to detect genuine failures vs pods that are simply still starting up.
	// UnavailableReplicas alone cannot distinguish the two cases.
	var labelSel map[string]string
	if deployment.Spec.Selector != nil {
		labelSel = deployment.Spec.Selector.MatchLabels
	}
	failedPods := ComputeDeploymentFailedPods(ctx, clusterClient.ClusterCTRLClient, instance.GetNamespace(), labelSel, numberUnavailable)

	return csmv1.PodStatus{
		Available: fmt.Sprintf("%d", available),
		Desired:   fmt.Sprintf("%d", desired),
		Failed:    fmt.Sprintf("%d", failedPods),
	}, err
}

// ComputeFailedPods lists pods matching the given selector and returns the count of pods
// in a genuine failure state (CrashLoopBackOff, ImagePullBackOff, etc.).
// It ignores pods that are simply starting up (ContainerCreating, PodInitializing,
// ErrImagePull). Only reasons in FailureWaitingReasons are counted as failures.
// fallback is returned when pod listing fails.
// Works for any workload type (Deployment, StatefulSet, DaemonSet).
func ComputeFailedPods(ctx context.Context, c client.Client, namespace string, selector map[string]string, fallback int32) int32 {
	podList := &corev1.PodList{}
	listErr := c.List(ctx, podList, []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(selector),
	}...)
	if listErr != nil {
		return fallback
	}
	failed := int32(0)
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodFailed {
			failed++
			continue
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && !NormalStartupReasons[cs.State.Waiting.Reason] {
				failed++
				break // count each pod only once even if multiple containers are failing
			}
		}
	}
	return failed
}

// ComputeDeploymentFailedPods is an alias for ComputeFailedPods for backward compatibility.
func ComputeDeploymentFailedPods(ctx context.Context, c client.Client, namespace string, selector map[string]string, fallback int32) int32 {
	return ComputeFailedPods(ctx, c, namespace, selector, fallback)
}

// ComputeStatefulSetFailedPods is an alias for ComputeFailedPods for backward compatibility.
func ComputeStatefulSetFailedPods(ctx context.Context, c client.Client, namespace string, selector map[string]string, fallback int32) int32 {
	return ComputeFailedPods(ctx, c, namespace, selector, fallback)
}

func getDaemonSetStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM) (int32, csmv1.PodStatus, error) {
	log := logger.GetLogger(ctx)

	var msg string

	totalAvialable := int32(0)
	totalDesired := int32(0)
	totalFailedCount := 0
	totalRunning := int32(0)

	clusterClient := GetCluster(ctx, r)
	totalRunning = 0
	log.Debugf("\ngetting daemonset status for cluster: %s", clusterClient.ClusterID)
	msg += fmt.Sprintf("error message for %s \n", clusterClient.ClusterID)

	ds := &appsv1.DaemonSet{}

	nodeName := instance.GetNodeName()
	namespace := instance.GetNamespace()
	log.Debugf("nodeName: %s, namespace: %s ", nodeName, namespace)
	err := clusterClient.ClusterCTRLClient.Get(ctx, t1.NamespacedName{
		Name:      nodeName,
		Namespace: namespace,
	}, ds)
	if err != nil {
		return 0, csmv1.PodStatus{}, err
	}
	failedCount := 0
	podList := &corev1.PodList{}
	label := instance.GetName() + "-node"
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{"app": label},
	}

	log.Debugf("Label is %s", label)
	err = clusterClient.ClusterCTRLClient.List(ctx, podList, opts...)
	if err != nil {
		return ds.Status.DesiredNumberScheduled, csmv1.PodStatus{}, err
	}

	errMap := make(map[string]string)
	for _, pod := range podList.Items {
		log.Debugf("daemonset pod %s : %s", pod.Name, pod.Status.Phase)

		// Check if init containers are still pending or running - pod is starting up, not failed
		initContainersRunning := false
		initContainerFailed := false
		for _, cs := range pod.Status.InitContainerStatuses {
			if cs.State.Terminated == nil {
				// Init container has not yet terminated (waiting or running)
				initContainersRunning = true
				break
			} else if cs.State.Terminated.ExitCode != 0 {
				// Init container failed (non-zero exit code)
				initContainerFailed = true
				log.Debugf("daemonset pod %s init container %s failed with exit code %d", pod.Name, cs.Name, cs.State.Terminated.ExitCode)
				errMap[fmt.Sprintf("InitContainerFailed:%s", cs.Name)] = fmt.Sprintf("exit code %d", cs.State.Terminated.ExitCode)
				break
			}
			// Init container succeeded (exit code 0), continue checking other init containers
		}

		if initContainerFailed {
			// Init container failed - count as failure regardless of other states
			failedCount++
			continue
		}

		if pod.Status.Phase == corev1.PodPending {
			podCounted := false
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.State.Waiting == nil {
					continue
				}
				reason := cs.State.Waiting.Reason

				// Normal startup reasons: pod is starting up, not a failure
				if NormalStartupReasons[reason] {
					log.Infof("daemonset pod container %s : %s (startup: %s)", pod.Name, pod.Status.Phase, reason)
					errMap[reason] = constants.PendingCreate
					continue
				}

				// If init containers are still running, treat as starting up regardless of reason
				if initContainersRunning {
					log.Infof("daemonset pod %s has init containers running, treating as starting up", pod.Name)
					errMap[reason] = constants.PendingCreate
					continue
				}

				// Any other waiting reason: count as failure (once per pod).
				// The grace period at the CR level determines when this becomes a real failure.
				if !podCounted {
					failedCount++
					podCounted = true
					log.Infow("daemonset pod container", "message", cs.State.Waiting.Message, constants.Reason, reason)
					shortMsg := strings.Replace(cs.State.Waiting.Message,
						constants.PodStatusRemoveString, "", 1)
					errMap[reason] = shortMsg
				}
			}
		}

		// Also check pods in Running phase but with init containers still running
		if pod.Status.Phase == corev1.PodRunning && initContainersRunning {
			log.Infof("daemonset pod %s is Running but init containers are still running", pod.Name)
			// Don't count as failure, pod is still initializing
			continue
		}
		// pod can be running even if not all containers are up
		podReadyCondition := corev1.ConditionFalse
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady {
				podReadyCondition = condition.Status
			}
		}

		if pod.Status.Phase == corev1.PodRunning && podReadyCondition == corev1.ConditionTrue {
			totalRunning++
		}
		if podReadyCondition != corev1.ConditionTrue {
			log.Infof("daemonset pod: %s is running, but is not ready", pod.Name)
		}
	}
	for k, v := range errMap {
		msg += k + "=" + v
	}

	log.Infof("[daemonset] status available pods %d, failedCount pods %d, desired pods %d", totalRunning, failedCount, ds.Status.DesiredNumberScheduled)

	totalAvialable += totalRunning
	totalDesired += ds.Status.DesiredNumberScheduled
	totalFailedCount += failedCount

	if totalFailedCount > 0 {
		err = errors.New(msg)
	}
	return totalDesired, csmv1.PodStatus{
		Available: fmt.Sprintf("%d", totalAvialable),
		Desired:   fmt.Sprintf("%d", totalDesired),
		Failed:    fmt.Sprintf("%d", totalFailedCount),
	}, err
}

func calculateState(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, newStatus *csmv1.ContainerStorageModuleStatus, op OperatorConfig, deploymentStatusOverride ...csmv1.PodStatus) (bool, error) {
	log := logger.GetLogger(ctx)
	running := true
	var err error
	nodeStatusGood := true

	// Default to Pending state during deployment
	newStatus.State = constants.Pending

	var controllerStatus csmv1.PodStatus
	if len(deploymentStatusOverride) > 0 {
		// Use the deployment status from the informer event to avoid stale cache reads
		controllerStatus = deploymentStatusOverride[0]
		log.Infof("using deployment status override: desired=%s, available=%s, failed=%s",
			controllerStatus.Desired, controllerStatus.Available, controllerStatus.Failed)
	} else {
		var controllerErr error
		controllerStatus, controllerErr = getDeploymentStatus(ctx, instance, r)
		if controllerErr != nil {
			log.Infof("error from getDeploymentStatus: %s", controllerErr.Error())
		}
	}

	// Auth proxy and Cosi driver have no daemonset. Putting this if/else in here and setting nodeStatusGood to true by
	// default is a little hacky but will be fixed when we refactor the status code in CSM 1.10 or 1.11
	log.Infof("instance.GetName() is %s", instance.GetName())
	if instance.GetName() != "" && !isAuthorizationProxyServer(instance) && instance.Spec.Driver.CSIDriverType != csmv1.Cosi {
		expected, nodeStatus, daemonSetErr := getDaemonSetStatus(ctx, instance, r)
		newStatus.NodeStatus = nodeStatus
		if daemonSetErr != nil {
			err = daemonSetErr
			log.Infof("calculate Daemonseterror msg [%s]", daemonSetErr.Error())
		}

		log.Infof("[daemonset] expected [%d], nodeStatus.Available [%s]", expected, nodeStatus.Available)
		nodeStatusGood = (fmt.Sprintf("%d", expected) == nodeStatus.Available)
	}

	newStatus.ControllerStatus = controllerStatus

	log.Infof("[deployment] controllerStatus.Desired [%s], controllerStatus.Available [%s]", controllerStatus.Desired, controllerStatus.Available)

	// Check if controller pods are ready
	// For drivers: also check daemonset (nodeStatusGood)
	// For auth proxy: only check controller pods (no daemonset)
	// Ensure controllerStatus is not empty before evaluating (Desired="0" means no deployments exist yet)
	controllerPodsReady := controllerStatus.Desired == controllerStatus.Available && controllerStatus.Desired != "0" && controllerStatus.Desired != ""
	allPodsReady := false
	if isAuthorizationProxyServer(instance) {
		// Auth proxy has no daemonset, only check controller pods
		allPodsReady = controllerPodsReady
	} else {
		// Drivers have both controller and daemonset pods
		allPodsReady = controllerPodsReady && nodeStatusGood
	}

	// Helper to check if any pods are in a failure state
	crKey := instance.GetNamespace() + "/" + instance.GetName()
	hasPodFailures := func() bool {
		return (controllerStatus.Failed != "" && controllerStatus.Failed != "0") ||
			(newStatus.NodeStatus.Failed != "" && newStatus.NodeStatus.Failed != "0")
	}

	if allPodsReady {
		// Check if PreUpgradeSnapshot exists to determine if upgrade is in progress.
		// When snapshot exists and stability period has NOT elapsed, keep state as Pending
		// to prevent false positive Succeeded state during rolling updates. Once the
		// stability period elapses, allow Succeeded state even with snapshot present,
		// so the reconcile loop can clear the snapshot.
		annotations := instance.GetAnnotations()
		_, snapshotExists := annotations[preUpgradeSnapshotKey]
		stabilityPeriodElapsed := checkSucceededStabilityPeriod(crKey)
		if snapshotExists && !stabilityPeriodElapsed {
			log.Info("PreUpgradeSnapshot exists and stability period not elapsed, keeping state as Pending")
			newStatus.State = constants.Pending
			running = false
		} else if stabilityPeriodElapsed {
			// Stability period has elapsed, safe to set Succeeded
			newStatus.State = constants.Succeeded
		} else {
			// Still within stability period, keep as Pending
			log.Infof("Pods are ready but within %v stability period, keeping state as Pending", SucceededStabilityPeriod)
			newStatus.State = constants.Pending
			running = false
		}

		moduleCheckPerformed := false
		for _, module := range instance.Spec.Modules {
			moduleStatus, exists := checkModuleStatus[module.Name]
			if exists && module.Enabled {
				moduleCheckPerformed = true
				moduleRunning, err := moduleStatus(ctx, instance, r, newStatus, op)
				if err != nil {
					log.Infof("status for module err msg [%s]", err.Error())
				}

				if !moduleRunning {
					running = false
					if hasPodFailures() {
						if checkFailureGracePeriod(crKey) {
							newStatus.State = constants.Failed
							ClearSucceededStabilityPeriod(crKey)
							log.Infof("grace period elapsed, marking CR as Failed")
						} else {
							newStatus.State = constants.Pending
							log.Infof("pod failures detected but within grace period, keeping CR as Pending")
						}
					} else {
						newStatus.State = constants.Pending
					}
					log.Infof("%s module not running", module.Name)
					break
				}
				log.Infof("%s module running", module.Name)
			}
		}
		// For module-only CRs (like auth proxy), if no module checks were performed,
		// keep state as Pending to ensure CR doesn't show Succeeded when no modules are configured
		// For driver CRs, this check is not applicable
		if !moduleCheckPerformed && isAuthorizationProxyServer(instance) {
			newStatus.State = constants.Pending
			running = false
			log.Infof("no enabled modules found for module-only CR, keeping state as Pending")
		}

		// Guard against rolling-update false positives: during a rolling update
		// the old pod stays Available (satisfying Desired==Available), but the
		// new pod may be in ImagePullBackOff or CrashLoopBackOff. The module
		// status check above only compares AvailableReplicas vs Spec.Replicas
		// and therefore passes. Catch this case here.
		if running && hasPodFailures() {
			running = false
			ClearSucceededStabilityPeriod(crKey)
			if checkFailureGracePeriod(crKey) {
				newStatus.State = constants.Failed
				log.Infof("pods ready but failures detected and grace period elapsed, marking CR as Failed")
			} else {
				newStatus.State = constants.Pending
				log.Infof("pods ready but failures detected within grace period, keeping CR as Pending")
			}
		}

		// Only clear the failure grace period when no pod failures remain.
		// Previously this was unconditional, which wiped failure tracking
		// during rolling updates where the old pod keeps Available==Desired
		// but the new pod is actually failing.
		if !hasPodFailures() {
			ClearFailureGracePeriod(crKey)
		}
	} else {
		log.Infof("deployment or daemonset did not have enough available pods")
		log.Infof("[deployment] desired [%s], available [%s], failed [%s]", controllerStatus.Desired, controllerStatus.Available, controllerStatus.Failed)
		log.Infof("[daemonset] healthy: [%v]", nodeStatusGood)
		running = false
		ClearSucceededStabilityPeriod(crKey)
		if hasPodFailures() {
			// Pods are failing — apply grace period before marking CR as Failed.
			// This avoids false-positive Failed status during transient issues
			// like brief CrashLoopBackOff during rolling updates.
			if checkFailureGracePeriod(crKey) {
				newStatus.State = constants.Failed
				ClearSucceededStabilityPeriod(crKey)
				log.Infof("grace period elapsed, marking CR as Failed")
			} else {
				newStatus.State = constants.Pending
				log.Infof("pod failures detected but within %v grace period, keeping CR as Pending", FailureGracePeriod)
			}
		} else {
			// No failures, pods are just starting up
			ClearFailureGracePeriod(crKey)
			newStatus.State = constants.Pending
		}
	}

	SetStatus(ctx, r, instance, newStatus)
	if isAuthorizationProxyServer(instance) && instance.Status.State == constants.Succeeded {
		copyCR := instance.DeepCopy()
		delete(copyCR.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
		copyCR.ManagedFields = nil
		copyCR.Status = csmv1.ContainerStorageModuleStatus{}
		out, err := json.Marshal(copyCR)
		if err != nil {
			log.Error(err, "error marshalling CR to annotation")
		}
		instance.Status.LastSuccessfulConfiguration = string(out)
	}
	return running, err
}

// SetStatus of csm
func SetStatus(ctx context.Context, _ ReconcileCSM, instance *csmv1.ContainerStorageModule, newStatus *csmv1.ContainerStorageModuleStatus) {
	log := logger.GetLogger(ctx)
	instance.GetCSMStatus().State = newStatus.State
	log.Infow("Driver State", "Controller",
		newStatus.ControllerStatus, "Node", newStatus.NodeStatus)
	instance.GetCSMStatus().ControllerStatus = newStatus.ControllerStatus
	instance.GetCSMStatus().NodeStatus = newStatus.NodeStatus
}

// UpdateStatus of csm
func UpdateStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, newStatus *csmv1.ContainerStorageModuleStatus, op OperatorConfig, deploymentStatusOverride ...csmv1.PodStatus) (reconcile.Result, error) {
	dMutex.Lock()
	defer dMutex.Unlock()

	log := logger.GetLogger(ctx)
	unitTestRun := DetermineUnitTestRun(ctx)

	log.Infow("update current csm status", "status", instance.Status.State)
	statusString := fmt.Sprintf("update new Status: (State - %s)",
		newStatus.State)
	log.Info(statusString)
	log.Infow("Update State", "Controller",
		newStatus.ControllerStatus, "Node", newStatus.NodeStatus)

	running, merr := calculateState(ctx, instance, r, newStatus, op, deploymentStatusOverride...)
	log.Info("calculateState returns ", "running: ", running)

	// Add last successful configuration into status if deployment is running
	// and controller has desired replicas to handle the last successful configuration change
	replicas := instance.Spec.Driver.Replicas
	if newStatus.ControllerStatus.Desired == strconv.Itoa(int(replicas)) && running {
		if lastAnnotations := instance.GetAnnotations(); lastAnnotations != nil {
			if lastApplied := lastAnnotations["storage.dell.com/PreviouslyAppliedConfiguration"]; lastApplied != "" {
				instance.Status.LastSuccessfulConfiguration = lastApplied
			}
		}
	}

	// NOTE: Do NOT clear SucceededStabilityPeriod here when running==true.
	// Clearing it would cause informer-driven handlers (handleDeploymentUpdate,
	// handlePodsUpdate) to restart the stability timer immediately, pushing the
	// state back to Pending in an infinite Succeeded→Pending oscillation.
	// The stability period is already properly cleared in calculateState when
	// pods become unhealthy (allPodsReady==false branch).

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		log := logger.GetLogger(ctx)

		csm := new(csmv1.ContainerStorageModule)
		err := r.GetClient().Get(ctx, t1.NamespacedName{
			Name:      instance.Name,
			Namespace: instance.GetNamespace(),
		}, csm)
		if err != nil {
			return err
		}

		log.Debugf("[instance] new controller Status - desired: %s, Available: %s, numberUnavailable: %s, State: %s",
			newStatus.ControllerStatus.Desired,
			newStatus.ControllerStatus.Available,
			newStatus.ControllerStatus.Failed,
			newStatus.State)

		if newStatus.State == csm.Status.State {
			log.Info("Deployment state unchanged, no need to update status")
			return nil
		}

		csm.Status = *newStatus
		err = r.GetClient().Status().Update(ctx, csm)
		return err
	})
	if err != nil {
		// May be conflict if max retries were hit, or may be something unrelated
		// like permissions or a network error
		log.Error(err, " Failed to update CR status")
		// Return error only - controller-runtime will requeue with exponential backoff
		return reconcile.Result{}, err
	}

	log.Info("Update done")
	log.Infow("UpdateStatus Driver state ", "newStatus.State", newStatus.State)

	// Determine requeue based on calculated state
	requeue := reconcile.Result{}
	if !running && !unitTestRun {
		if newStatus.State == constants.Pending {
			// Use RequeueAfter to avoid exponential backoff from AddRateLimited.
			// During rolling updates the reconciler requeues many times while
			// waiting for pods to become ready.  Requeue:true causes
			// queue.AddRateLimited which doubles the delay on every iteration
			// (5ms → 10ms → … → 1000s cap), eventually leaving the snapshot
			// annotation stuck for minutes.  RequeueAfter calls queue.Forget
			// (clearing the backoff counter) then queue.AddAfter, giving a
			// fixed, predictable polling interval.
			requeue = reconcile.Result{RequeueAfter: 5 * time.Second}
			log.Info("CSM state is Pending, will requeue after 5s")
		} else {
			requeue = reconcile.Result{Requeue: true}
			log.Info("CSM state is Failed, will requeue")
		}
	}

	// if CSM is not running, don't return error - just return requeue result
	// The controller will use the requeue result to continue monitoring
	if !running {
		return requeue, nil
	}

	return requeue, merr
}

// HandleValidationError for csm
func HandleValidationError(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM,
	validationError error,
) (reconcile.Result, error) {
	dMutex.Lock()
	defer dMutex.Unlock()
	log := logger.GetLogger(ctx)

	// Update the status to Failed with retry to handle concurrent status updates
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		csm := new(csmv1.ContainerStorageModule)
		if err := r.GetClient().Get(ctx, t1.NamespacedName{
			Name:      instance.Name,
			Namespace: instance.GetNamespace(),
		}, csm); err != nil {
			return err
		}
		csm.GetCSMStatus().State = constants.Failed
		return r.GetClient().Status().Update(ctx, csm)
	})
	if err != nil {
		log.Error(err, "Failed to update CR status HandleValidationError")
	}
	log.Error(validationError, fmt.Sprintf(" *************Create/Update %s failed ********",
		instance.GetDriverType()))
	return reconcile.Result{Requeue: false}, validationError
}

// IsUpgradeInProgress determines whether an upgrade is in progress by comparing
// the persisted configVersion annotation with the current version from the spec.
func IsUpgradeInProgress(ctx context.Context, cr *csmv1.ContainerStorageModule, op OperatorConfig) bool {
	annotations := cr.GetAnnotations()
	if annotations == nil {
		return false
	}

	oldVersion, exists := annotations[configVersionKey]
	if !exists {
		return false
	}

	newVersion, err := GetVersion(ctx, cr, op)
	if err != nil {
		return false
	}

	return oldVersion != newVersion
}

func getNginxControllerStatus(ctx context.Context, instance csmv1.ContainerStorageModule, r ReconcileCSM) wait.ConditionWithContextFunc {
	return func(context.Context) (bool, error) {
		deployment := &appsv1.Deployment{}
		labelKey := "app.kubernetes.io/name"
		label := "ingress-nginx"
		name := instance.GetNamespace() + "-ingress-nginx-controller"

		err := r.GetClient().Get(ctx, t1.NamespacedName{
			Name:      name,
			Namespace: instance.GetNamespace(),
		}, deployment)
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

	return wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, getNginxControllerStatus(ctx, instance, r))
}

func getGatewayControllerStatus(ctx context.Context, instance csmv1.ContainerStorageModule, r ReconcileCSM) wait.ConditionWithContextFunc {
	return func(context.Context) (bool, error) {
		deployment := &appsv1.Deployment{}
		name := instance.GetNamespace() + "-nginx-gateway-fabric"

		err := r.GetClient().Get(ctx, t1.NamespacedName{
			Name:      name,
			Namespace: instance.GetNamespace(),
		}, deployment)
		if err != nil {
			return false, err
		}

		if deployment.Spec.Replicas != nil && deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
			return true, nil
		}

		return false, nil
	}
}

// WaitForGatewayController - polls Gateway API controller deployment status
func WaitForGatewayController(ctx context.Context, instance csmv1.ContainerStorageModule, r ReconcileCSM, timeout time.Duration) error {
	log := logger.GetLogger(ctx)
	log.Infow("Polling status of Gateway API controller")

	return wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, getGatewayControllerStatus(ctx, instance, r))
}

// observabilityStatusCheck - calculate success state for observability module
func observabilityStatusCheck(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, _ *csmv1.ContainerStorageModuleStatus, op OperatorConfig) (bool, error) {
	log := logger.GetLogger(ctx)
	topologyEnabled := false
	otelEnabled := false
	certEnabled := false
	metricsEnabled := false

	driverName := instance.Spec.Driver.CSIDriverType

	// PowerScale DriverType should be changed from "isilon" to "powerscale"
	// this is a temporary fix until we can do that
	if driverName == csmv1.PowerScale {
		driverName = csmv1.PowerScaleName
	}

	for _, m := range instance.Spec.Modules {
		if m.Name == csmv1.Observability {
			for _, c := range m.Components {
				if c.Name == "topology" && *c.Enabled {
					topologyEnabled = true
				}
				if c.Name == "otel-collector" && *c.Enabled {
					otelEnabled = true
				}
				if c.Name == "cert-manager" && *c.Enabled {
					certEnabled = true
				}
				if c.Name == fmt.Sprintf("metrics-%s", driverName) && *c.Enabled {
					metricsEnabled = true
				}
			}
		}
	}

	namespace := instance.GetNamespace()
	configVersion, err := GetVersion(ctx, instance, op)
	if err != nil {
		return false, err
	}

	// Override namespace to "karavi" if config version is below v2.15
	if strings.Contains(configVersion, "v2.13") || strings.Contains(configVersion, "v2.14") {
		namespace = ObservabilityNamespace
	}

	opts := []client.ListOption{
		client.InNamespace(namespace),
	}
	deploymentList := &appsv1.DeploymentList{}
	err = r.GetClient().List(ctx, deploymentList, opts...)
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
				if !checkFn(&deployment) {
					log.Infof("%s component not running in observability deployment", deployment.Name)
					return false, nil
				}
			}
		case fmt.Sprintf("karavi-metrics-%s", driverName):
			if metricsEnabled {
				if !checkFn(&deployment) {
					log.Infof("%s component not running in observability deployment", deployment.Name)
					return false, nil
				}
			}
		case "karavi-topology":
			if topologyEnabled {
				if !checkFn(&deployment) {
					log.Infof("%s component not running in observability deployment", deployment.Name)
					return false, nil
				}
			}
		}
	}

	opts = []client.ListOption{
		client.InNamespace(namespace),
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
				if !checkFn(&deployment) {
					log.Infof("%s component not running in observability deployment", deployment.Name)
					return false, nil
				}
			}
		case "cert-manager-cainjector":
			if certEnabled {
				if !checkFn(&deployment) {
					log.Infof("%s component not running in observability deployment", deployment.Name)
					return false, nil
				}
			}
		case "cert-manager-webhook":
			if certEnabled {
				if !checkFn(&deployment) {
					log.Infof("%s component not running in observability deployment", deployment.Name)
					return false, nil
				}
			}
		}
	}

	return true, nil
}

// authProxyStatusCheck - calculate success state for auth proxy
func authProxyStatusCheck(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM, newStatus *csmv1.ContainerStorageModuleStatus, _ OperatorConfig) (bool, error) {
	log := logger.GetLogger(ctx)
	certEnabled := false
	nginxEnabled := false
	gatewayEnabled := false
	proxyServerEnabled := false
	redisCommanderName := ""
	redisName := ""
	sentinelName := ""

	// Check if authorization module version is v2.5.0 or later (Gateway API)
	var useGatewayAPI bool
	for _, m := range instance.Spec.Modules {
		if m.Name == csmv1.AuthorizationServer {
			var err error
			useGatewayAPI, err = MinVersionCheck("v2.5.0", m.ConfigVersion)
			if err != nil {
				log.Errorw("error checking authorization version", "error", err)
			}
			break
		}
	}

	for _, m := range instance.Spec.Modules {
		if m.Name == csmv1.AuthorizationServer {
			for _, c := range m.Components {
				if c.Name == "ingress-nginx" && *c.Enabled {
					nginxEnabled = true
				}
				// Check for either the new gateway component name (v2.5.0+) or the legacy
				// nginx name (upgrade compat: users migrating from v2.4.0 may still have
				// name: nginx in their CR; the version gate ensures gateway is used).
				if (c.Name == "nginx-gateway-fabric" || c.Name == "nginx") && *c.Enabled && useGatewayAPI {
					gatewayEnabled = true
				}
				if c.Name == "cert-manager" && *c.Enabled {
					certEnabled = true
				}
				if c.Name == "proxy-server" && *c.Enabled {
					proxyServerEnabled = true
				}
				if c.Name == "redis" {
					redisCommanderName = c.RedisCommander
					redisName = c.RedisName
					sentinelName = c.Sentinel
				}
			}
		}
	}

	authNamespace := instance.GetNamespace()
	isOpenShift := r.GetConfig().IsOpenShift

	opts := []client.ListOption{
		client.InNamespace(authNamespace),
	}
	deploymentList := &appsv1.DeploymentList{}
	err := r.GetClient().List(ctx, deploymentList, opts...)
	if err != nil {
		return false, err
	}

	// Always populate controllerStatus with accurate pod counts, even when deployments
	// are not yet ready. This ensures calculateState can distinguish Pending from Failed.
	podStatus, psErr := getAuthProxyDeploymentStatus(ctx, instance, r)
	if psErr != nil {
		log.Infof("error getting auth proxy deployment status: %v", psErr)
	} else if newStatus != nil {
		newStatus.ControllerStatus = podStatus
	}

	checkFn := func(deployment *appsv1.Deployment) bool {
		return deployment.Status.AvailableReplicas == *deployment.Spec.Replicas
	}

	// Track which required deployments have been found and checked
	foundDeployments := make(map[string]bool)
	requiredDeployments := []string{"role-service", "storage-service", "tenant-service", "authorization-controller"}
	if proxyServerEnabled {
		requiredDeployments = append(requiredDeployments, "proxy-server")
	}
	if certEnabled {
		requiredDeployments = append(requiredDeployments, "cert-manager", "cert-manager-cainjector", "cert-manager-webhook")
	}
	// On OpenShift the default router handles ingress; nginx/gateway deployments
	// are not created, so they must not be required for status to reach Succeeded.
	if gatewayEnabled && !isOpenShift {
		requiredDeployments = append(requiredDeployments, fmt.Sprintf("%s-nginx-gateway-fabric", authNamespace), fmt.Sprintf("%s-gateway-nginx", authNamespace))
	}
	if nginxEnabled && !isOpenShift {
		requiredDeployments = append(requiredDeployments, fmt.Sprintf("%s-ingress-nginx-controller", authNamespace))
	}
	if redisCommanderName != "" {
		requiredDeployments = append(requiredDeployments, redisCommanderName)
	}

	for _, deployment := range deploymentList.Items {
		deployment := deployment
		switch deployment.Name {
		case fmt.Sprintf("%s-ingress-nginx-controller", authNamespace):
			if nginxEnabled && !isOpenShift {
				foundDeployments[deployment.Name] = true
				if !checkFn(&deployment) {
					log.Infof("%s component not running in auth proxy deployment", deployment.Name)
					return false, nil
				}
			}
		case fmt.Sprintf("%s-nginx-gateway-controller", authNamespace):
			// Legacy name for older versions, keep for backward compatibility
			if gatewayEnabled && !isOpenShift {
				foundDeployments[deployment.Name] = true
				if !checkFn(&deployment) {
					log.Infof("%s component not running in auth proxy deployment", deployment.Name)
					return false, nil
				}
			}
		case fmt.Sprintf("%s-nginx-gateway-fabric", authNamespace), fmt.Sprintf("%s-gateway-nginx", authNamespace):
			// New gateway fabric names for v2.5.0+
			if gatewayEnabled && !isOpenShift {
				foundDeployments[deployment.Name] = true
				if !checkFn(&deployment) {
					log.Infof("%s component not running in auth proxy deployment", deployment.Name)
					return false, nil
				}
			}
		case "cert-manager", "cert-manager-cainjector", "cert-manager-webhook":
			if certEnabled {
				foundDeployments[deployment.Name] = true
				if !checkFn(&deployment) {
					log.Infof("%s component not running in auth proxy deployment", deployment.Name)
					return false, nil
				}
			}
		case "proxy-server":
			if proxyServerEnabled {
				foundDeployments[deployment.Name] = true
				if !checkFn(&deployment) {
					log.Infof("%s component not running in auth proxy deployment", deployment.Name)
					return false, nil
				}
			}
		case "role-service", "storage-service", "tenant-service", "authorization-controller":
			foundDeployments[deployment.Name] = true
			if !checkFn(&deployment) {
				log.Infof("%s component not running in auth proxy deployment", deployment.Name)
				return false, nil
			}
		default:
			if redisCommanderName != "" && deployment.Name == redisCommanderName {
				foundDeployments[deployment.Name] = true
				if !checkFn(&deployment) {
					log.Infof("%s component not running in auth proxy deployment", deployment.Name)
					return false, nil
				}
			}
		}
	}

	// Check that all required deployments exist
	for _, reqDep := range requiredDeployments {
		if !foundDeployments[reqDep] {
			log.Infof("required deployment %s not found in auth proxy namespace", reqDep)
			return false, nil
		}
	}

	// Check StatefulSets (Redis and Sentinel)
	checkStsFn := func(sts *appsv1.StatefulSet) bool {
		if sts.Spec.Replicas == nil {
			return sts.Status.ReadyReplicas == 1
		}
		return sts.Status.ReadyReplicas == *sts.Spec.Replicas
	}

	statefulSetList := &appsv1.StatefulSetList{}
	if err := r.GetClient().List(ctx, statefulSetList, opts...); err != nil {
		return false, err
	}

	// Track which required statefulsets have been found and checked
	foundStatefulSets := make(map[string]bool)
	requiredStatefulSets := []string{}
	if redisName != "" {
		requiredStatefulSets = append(requiredStatefulSets, redisName)
	}
	if sentinelName != "" {
		requiredStatefulSets = append(requiredStatefulSets, sentinelName)
	}

	for _, sts := range statefulSetList.Items {
		sts := sts
		if (redisName != "" && sts.Name == redisName) || (sentinelName != "" && sts.Name == sentinelName) {
			foundStatefulSets[sts.Name] = true
			if !checkStsFn(&sts) {
				log.Infof("%s component not running in auth proxy statefulset", sts.Name)
				return false, nil
			}
		}
	}

	// Check that all required statefulsets exist
	for _, reqSts := range requiredStatefulSets {
		if !foundStatefulSets[reqSts] {
			log.Infof("required statefulset %s not found in auth proxy namespace", reqSts)
			return false, nil
		}
	}

	log.Info("auth proxy deployment successful")

	return true, nil
}

func isAuthorizationProxyServer(cr *csmv1.ContainerStorageModule) bool {
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.AuthorizationServer {
			return true
		}
	}
	return false
}

func getAuthProxyDeploymentStatus(ctx context.Context, instance *csmv1.ContainerStorageModule, r ReconcileCSM) (csmv1.PodStatus, error) {
	log := logger.GetLogger(ctx)
	authNamespace := instance.GetNamespace()
	isOpenShift := r.GetConfig().IsOpenShift

	certEnabled := false
	nginxEnabled := false
	gatewayEnabled := false
	redisCommanderName := ""
	redisName := ""
	sentinelName := ""

	var useGatewayAPI bool
	for _, m := range instance.Spec.Modules {
		if m.Name == csmv1.AuthorizationServer {
			useGatewayAPI, _ = MinVersionCheck("v2.5.0", m.ConfigVersion)
			for _, c := range m.Components {
				if c.Name == "ingress-nginx" && *c.Enabled {
					nginxEnabled = true
				}
				if (c.Name == "nginx-gateway-fabric" || c.Name == "nginx") && *c.Enabled && useGatewayAPI {
					gatewayEnabled = true
				}
				if c.Name == "cert-manager" && *c.Enabled {
					certEnabled = true
				}
				if c.Name == "redis" {
					redisCommanderName = c.RedisCommander
					redisName = c.RedisName
					sentinelName = c.Sentinel
				}
			}
			break
		}
	}

	var desired, available, failed int32

	opts := []client.ListOption{
		client.InNamespace(authNamespace),
	}

	deploymentList := &appsv1.DeploymentList{}
	if err := r.GetClient().List(ctx, deploymentList, opts...); err != nil {
		return csmv1.PodStatus{Available: "0", Desired: "0", Failed: "0"}, err
	}
	log.Infof("Total deployments listed in namespace %s: %d", authNamespace, len(deploymentList.Items))

	for _, deployment := range deploymentList.Items {
		isAuthDep := false
		switch deployment.Name {
		case fmt.Sprintf("%s-ingress-nginx-controller", authNamespace):
			if nginxEnabled && !isOpenShift {
				isAuthDep = true
			}
		case fmt.Sprintf("%s-nginx-gateway-controller", authNamespace):
			// Legacy name for older versions, keep for backward compatibility
			if gatewayEnabled && !isOpenShift {
				isAuthDep = true
			}
		case fmt.Sprintf("%s-nginx-gateway-fabric", authNamespace), fmt.Sprintf("%s-gateway-nginx", authNamespace):
			// New gateway fabric names for v2.5.0+
			if gatewayEnabled && !isOpenShift {
				isAuthDep = true
			}
		case "cert-manager", "cert-manager-cainjector", "cert-manager-webhook":
			if certEnabled {
				isAuthDep = true
			}
		case "proxy-server", "role-service", "storage-service", "tenant-service", "authorization-controller":
			isAuthDep = true
		default:
			if redisCommanderName != "" && deployment.Name == redisCommanderName {
				isAuthDep = true
			}
		}

		if isAuthDep {
			if deployment.Spec.Replicas != nil {
				desired += *deployment.Spec.Replicas
			}
			readyReplicas := deployment.Status.ReadyReplicas
			available += readyReplicas
			log.Infof("Auth deployment counted: %s, Spec.Replicas=%d, ReadyReplicas=%d", deployment.Name, *deployment.Spec.Replicas, readyReplicas)
			var labelSel map[string]string
			if deployment.Spec.Selector != nil {
				labelSel = deployment.Spec.Selector.MatchLabels
			}
			failed += ComputeDeploymentFailedPods(ctx, r.GetClient(), authNamespace, labelSel, deployment.Status.UnavailableReplicas)
		}
	}

	// Use direct K8s client to get fresh data from API server instead of cached client
	// to avoid stale ReadyReplicas values
	statefulSets := r.GetK8sClient().AppsV1().StatefulSets(authNamespace)
	statefulSetList, err := statefulSets.List(ctx, metav1.ListOptions{})
	if err != nil {
		return csmv1.PodStatus{Available: "0", Desired: "0", Failed: "0"}, err
	}
	log.Infof("Total statefulsets listed in namespace %s: %d", authNamespace, len(statefulSetList.Items))

	for _, sts := range statefulSetList.Items {
		isAuthSts := false
		if (redisName != "" && sts.Name == redisName) || (sentinelName != "" && sts.Name == sentinelName) {
			isAuthSts = true
		}

		if isAuthSts {
			if sts.Spec.Replicas != nil {
				desired += *sts.Spec.Replicas
			}
			readyReplicas := sts.Status.ReadyReplicas
			available += readyReplicas
			log.Infof("Auth statefulset counted: %s, Spec.Replicas=%d, ReadyReplicas=%d", sts.Name, *sts.Spec.Replicas, readyReplicas)
			if sts.Spec.Replicas != nil {
				unavail := *sts.Spec.Replicas - sts.Status.ReadyReplicas
				var labelSel map[string]string
				if sts.Spec.Selector != nil {
					labelSel = sts.Spec.Selector.MatchLabels
				}
				failed += ComputeStatefulSetFailedPods(ctx, r.GetClient(), authNamespace, labelSel, unavail)
			}
		}
	}

	log.Infof("Auth proxy deployments status: desired=%d available=%d failed=%d", desired, available, failed)

	return csmv1.PodStatus{
		Available: fmt.Sprintf("%d", available),
		Desired:   fmt.Sprintf("%d", desired),
		Failed:    fmt.Sprintf("%d", failed),
	}, nil
}
