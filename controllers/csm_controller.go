/*

Copyright (c) 2021 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controllers

import (
	"context"
	"fmt"
	"math"
	"os"
	"sync/atomic"
	"time"

	"github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/modules"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/constants"
	"github.com/dell/csm-operator/pkg/resources/configmap"
	"github.com/dell/csm-operator/pkg/resources/csidriver"
	"github.com/dell/csm-operator/pkg/resources/daemonset"
	"github.com/dell/csm-operator/pkg/resources/deployment"
	"github.com/dell/csm-operator/pkg/resources/rbac"
	"github.com/dell/csm-operator/pkg/resources/serviceaccount"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/go-logr/logr"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ContainerStorageModuleReconciler reconciles a ContainerStorageModule object
type ContainerStorageModuleReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Log         logr.Logger
	Config      utils.OperatorConfig
	updateCount int32
}

// MetadataPrefix - prefix for all labels & annotations
const MetadataPrefix = "storage.dell.com"

var configVersionKey = fmt.Sprintf("%s/%s", MetadataPrefix, "CSIDriverConfigVersion")

//+kubebuilder:rbac:groups=storage.dell.com,resources=containerstoragemodules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storage.dell.com,resources=containerstoragemodules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storage.dell.com,resources=containerstoragemodules/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods;services;services/finalizers;endpoints;persistentvolumeclaims;events;configmaps;secrets;serviceaccounts,verbs=*
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;create;patch;update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;create;delete;patch;update
// +kubebuilder:rbac:groups="apps",resources=deployments;daemonsets;replicasets;statefulsets,verbs=get;list;watch;update;create;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles;clusterrolebindings;replicasets;rolebindings,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles/finalizers,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="monitoring.coreos.com",resources=servicemonitors,verbs=get;create
// +kubebuilder:rbac:groups="apps",resources=deployments/finalizers,resourceNames=dell-csi-operator-controller-manager,verbs=update
// +kubebuilder:rbac:groups="storage.k8s.io",resources=csidrivers,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="storage.k8s.io",resources=storageclasses,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="storage.k8s.io",resources=volumeattachments,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="storage.k8s.io",resources=csinodes,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="snapshot.storage.k8s.io",resources=volumesnapshotclasses;volumesnapshotcontents,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="snapshot.storage.k8s.io",resources=volumesnapshotcontents/status,verbs=update
// +kubebuilder:rbac:groups="snapshot.storage.k8s.io",resources=volumesnapshots;volumesnapshots/status,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions,verbs=create;list;watch;delete
// +kubebuilder:rbac:groups="storage.k8s.io",resources=volumeattachments/status,verbs=patch
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="security.openshift.io",resources=securitycontextconstraints,resourceNames=privileged,verbs=use

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ContainerStorageModule object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile

func (r *ContainerStorageModuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("csm", req.NamespacedName)
	// your logic here
	csm := new(csmv1.ContainerStorageModule)

	reqLogger := log.WithValues("Namespace", req.Namespace)
	reqLogger = reqLogger.WithValues("Name", req.Name)
	reqLogger = reqLogger.WithValues("Attempt", r.updateCount)
	reqLogger.Info(fmt.Sprintf("Reconciling %s ", "csm"), "request", req.String())

	retryInterval := constants.DefaultRetryInterval
	reqLogger.Info("################Starting Reconcile##############")
	r.IncrUpdateCount()

	// Fetch the ContainerStorageModuleReconciler instance
	err := r.Client.Get(ctx, req.NamespacedName, csm)
	if err != nil {
		if k8serror.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, nil
	}

	isCustomResourceMarkedForDeletion := csm.GetDeletionTimestamp() != nil
	if isCustomResourceMarkedForDeletion {
		return r.removeFinalizer(ctx, csm, reqLogger)
	}

	// Add finalizer
	csm.SetFinalizers([]string{"finalizer.dell.emc.com"})
	// Update CR
	err = r.Client.Update(ctx, csm)
	if err != nil {
		reqLogger.Error(err, "Failed to update CR with finalizer")
		return reconcile.Result{}, err
	}

	driverConfig := &utils.OperatorConfig{
		IsOpenShift:     r.Config.IsOpenShift,
		K8sVersion:      r.Config.K8sVersion,
		ConfigDirectory: r.Config.ConfigDirectory,
	}

	// Before doing anything else, check for config version and apply annotation if not set
	isUpdated, err := checkAndApplyConfigVersionAnnotations(*csm, log, false)
	if err != nil {
		return utils.HandleValidationError(ctx, csm, r, reqLogger, err)
	} else if isUpdated {
		_ = r.GetClient().Update(ctx, csm)
		return reconcile.Result{Requeue: true}, nil
	}

	status := csm.GetCSMStatus()
	// newStatus is the status object which is modified and finally used to update the Status
	// in case the instance or the status is updated
	newStatus := status.DeepCopy()
	// oldStatus is the previous status of the CR instance
	// This is used to compare if there is a need to update the status
	oldStatus := status.DeepCopy()
	oldState := oldStatus.State
	reqLogger.Info(fmt.Sprintf("CSM was previously in (%s) state", string(oldState)))

	// Check if the driver has changed
	expectedHash, actualHash, changed := utils.CSMHashChanged(csm)
	if changed {
		message := fmt.Sprintf("CSM spec has changed (%d vs %d)", actualHash, expectedHash)
		newStatus.ContainerStorageModuleHash = expectedHash
		reqLogger.Info(message)
	} else {
		reqLogger.Info("No changes detected in the driver spec")
	}

	// Check if force update was requested
	forceUpdate := csm.Spec.Driver.ForceUpdate
	checkStateOnly := false
	switch oldState {
	case constants.Running:
		fallthrough
	case constants.Succeeded:
		if changed {
			// If the driver hash has changed, we need to update the driver again
			newStatus.State = constants.Updating
			reqLogger.Info("Changed state to Updating as CSM spec changed")
		} else {
			// Just check the state of the driver and update status accordingly
			reqLogger.Info("Recalculating CSM state(only) as there is no change in driver spec")
			checkStateOnly = true
		}
	case constants.InvalidConfig:
		fallthrough
	case constants.Failed:
		// Check if force update was requested
		if forceUpdate {
			reqLogger.Info("Force update requested")
			newStatus.State = constants.Updating
		} else {
			if changed {
				// Do a reconcile as we detected a change
				newStatus.State = constants.Updating
			} else {
				reqLogger.Info(fmt.Sprintf("CR is in (%s) state. Reconcile request won't be requeued",
					newStatus.State))
				return utils.LogBannerAndReturn(reconcile.Result{}, nil, reqLogger)
			}
		}
	case constants.NoState:
		newStatus.State = constants.Updating
	case constants.Updating:
		reqLogger.Info("CSM already in Updating state")
	}

	// Always initialize the spec
	// TODO(Michael): maybe always intiailize spec)
	isUpdated = true

	// Check if CSM is in running state (only if the status was previously set to Succeeded or Running)
	if checkStateOnly {
		return utils.HandleSuccess(ctx, csm, r, reqLogger, newStatus, oldStatus)
	}
	// Remove the force update field if set
	// The assumption is that we will not have a spec with Running/Succeeded state
	// and the forceUpdate field set
	if forceUpdate {
		csm.Spec.Driver.ForceUpdate = false
		isUpdated = true
	}
	if changed {
		isUpdated = true
	}
	// Update the instance
	if isUpdated {
		updateInstanceError := r.updateInstance(ctx, csm, reqLogger, isUpdated)
		if updateInstanceError != nil {
			newStatus.LastUpdate.ErrorMessage = updateInstanceError.Error()
			return utils.LogBannerAndReturn(reconcile.Result{
				Requeue: true, RequeueAfter: retryInterval}, updateInstanceError, reqLogger)
		}
		// Also update the status as we calculate the hash every time
		newStatus.LastUpdate = utils.SetLastStatusUpdate(oldStatus, csmv1.Updating, "")
		updateStatusError := utils.UpdateStatus(ctx, csm, r, reqLogger, newStatus, oldStatus)
		if updateStatusError != nil {
			newStatus.LastUpdate.ErrorMessage = updateStatusError.Error()
			reqLogger.Info(fmt.Sprintf("\n################End Reconcile %s %s##############\n", csm.Spec.Driver.CSIDriverType, req))
			return utils.LogBannerAndReturn(reconcile.Result{Requeue: true, RequeueAfter: retryInterval}, updateStatusError, reqLogger)
		}
	}

	// perfrom prechecks
	err = r.PreChecks(ctx, csm, *driverConfig, reqLogger)
	if err != nil {
		return utils.HandleValidationError(ctx, csm, r, reqLogger, err)
	}

	// Set the driver status to updating
	newStatus.State = constants.Updating
	// Update the driver
	syncErr := r.SyncCSM(ctx, *csm, *driverConfig, reqLogger)
	if syncErr == nil {
		// Mark the driver state as succeeded
		newStatus.State = constants.Succeeded
		errorMsg := ""
		running, err := utils.CalculateState(ctx, csm, r, newStatus)
		if err != nil {
			errorMsg = err.Error()
		}
		if running {
			newStatus.State = constants.Running
		}
		newStatus.LastUpdate = utils.SetLastStatusUpdate(oldStatus,
			utils.GetOperatorConditionTypeFromState(newStatus.State), errorMsg)
		updateStatusError := utils.UpdateStatus(ctx, csm, r, reqLogger, newStatus, oldStatus)
		if updateStatusError != nil {
			return reconcile.Result{Requeue: true, RequeueAfter: retryInterval}, updateStatusError
		}
		if newStatus.State != constants.Running {
			return utils.LogBannerAndReturn(reconcile.Result{Requeue: true, RequeueAfter: retryInterval}, nil, reqLogger)
		}
		return utils.LogBannerAndReturn(reconcile.Result{}, nil, reqLogger)
	}

	// Failed to sync driver deployment
	// Look at the last condition
	_, _ = utils.CalculateState(ctx, csm, r, newStatus)
	newStatus.LastUpdate = utils.SetLastStatusUpdate(oldStatus, csmv1.Error, syncErr.Error())
	// Check the last condition
	if oldStatus.LastUpdate.Condition == csmv1.Error {
		reqLogger.Info(" Driver previously encountered an error")
		timeSinceLastConditionChange := metav1.Now().Sub(oldStatus.LastUpdate.Time.Time).Round(time.Second)
		reqLogger.Info(fmt.Sprintf("Time since last condition change :%v", timeSinceLastConditionChange))
		if timeSinceLastConditionChange >= constants.MaxRetryDuration {
			// Mark the driver as failed and update the condition
			newStatus.State = constants.Failed
			newStatus.LastUpdate = utils.SetLastStatusUpdate(oldStatus,
				utils.GetOperatorConditionTypeFromState(newStatus.State), syncErr.Error())
			// This will trigger a reconcile again
			_ = utils.UpdateStatus(ctx, csm, r, reqLogger, newStatus, oldStatus)
			return utils.LogBannerAndReturn(reconcile.Result{Requeue: false}, nil, reqLogger)
		}
		retryInterval = time.Duration(math.Min(float64(timeSinceLastConditionChange.Nanoseconds()*2),
			float64(constants.MaxRetryInterval.Nanoseconds())))
	} else {
		_ = utils.UpdateStatus(ctx, csm, r, reqLogger, newStatus, oldStatus)
	}
	reqLogger.Info(fmt.Sprintf("Retry Interval: %v", retryInterval))

	// Don't return an error here. Controller runtime will immediately requeue the request
	// Also the requeueAfter setting only is effective after an amount of time

	return utils.LogBannerAndReturn(reconcile.Result{Requeue: true, RequeueAfter: retryInterval}, nil, reqLogger)
}

func (r *ContainerStorageModuleReconciler) updateInstance(ctx context.Context, instance *csmv1.ContainerStorageModule, reqLogger logr.Logger, isUpdated bool) error {
	if isUpdated {
		reqLogger.Info("Attempting to update CR instance")
		err := r.GetClient().Update(ctx, instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update CR instance")
		} else {
			reqLogger.Info("Successfully updated CR instance")
		}
		return err
	}
	reqLogger.Info("No updates to instance at this point")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ContainerStorageModuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New("ContainerStorageModule", mgr, controller.Options{Reconciler: r})
	if err != nil {
		r.Log.Error(err, "Unable to setup ContainerStorageModule controller")
		os.Exit(1)
	}

	err = c.Watch(
		&source.Kind{Type: &csmv1.ContainerStorageModule{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		r.Log.Error(err, "Unable to watch ContainerStorageModule Driver")
		os.Exit(1)
	}

	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &csmv1.ContainerStorageModule{},
	})
	if err != nil {
		r.Log.Error(err, "Unable to watch Deployment")
		os.Exit(1)
	}
	err = c.Watch(&source.Kind{Type: &appsv1.DaemonSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &csmv1.ContainerStorageModule{},
	})
	if err != nil {
		r.Log.Error(err, "Unable to watch Daemonset")
		os.Exit(1)
	}
	return nil
}

func (r *ContainerStorageModuleReconciler) removeFinalizer(ctx context.Context, instance *csmv1.ContainerStorageModule, log logr.Logger) (reconcile.Result, error) {
	// Remove the finalizers
	instance.SetFinalizers(nil)
	// Update the object
	err := r.Client.Update(ctx, instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// SyncCSM - Sync the current installation - this can lead to a create or update
func (r *ContainerStorageModuleReconciler) SyncCSM(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig,
	reqLogger logr.Logger) error {

	var (
		err        error
		driver     *storagev1.CSIDriver
		configMap  *corev1.ConfigMap
		node       *utils.NodeYAML
		controller *utils.ControllerYAML
	)

	// Get Driver resources
	switch cr.Spec.Driver.CSIDriverType {
	case csmv1.PowerScale:
		reqLogger.Info("Getting Powerscale CSI Driver for Dell EMC")

		configMap, err = drivers.GetPowerScaleConfigMap(cr, operatorConfig)
		if err != nil {
			return fmt.Errorf("getting %s configMap: %v", csmv1.PowerScale, err)
		}

		driver, err = drivers.GetPowerScaleCSIDriver(cr, operatorConfig)
		if err != nil {
			return fmt.Errorf("getting %s configMap: %v", csmv1.PowerScale, err)
		}

		node, err = drivers.GetPowerScaleNode(cr, operatorConfig)
		if err != nil {
			return fmt.Errorf("getting %s node: %v", csmv1.PowerScale, err)
		}

		controller, err = drivers.GetPowerScaleController(cr, operatorConfig)
		if err != nil {
			return fmt.Errorf("getting %s controller: %v", csmv1.PowerScale, err)
		}

	default:
		return fmt.Errorf("unsupported driver type %s", cr.Spec.Driver.CSIDriverType)
	}

	// Add module resources
	for _, m := range cr.Spec.Modules {
		if m.Enabled {
			switch m.Name {
			case csmv1.Authorization:
				reqLogger.Info("Injecting CSM Authorization")
				dp, err := modules.InjectDeployment(controller.Deployment, cr, operatorConfig)
				if err != nil {
					return fmt.Errorf("injecting auth into deployment: %v", err)
				}
				controller.Deployment = *dp

				ds, err := modules.InjectDeamonset(node.DaemonSet, cr, operatorConfig)
				if err != nil {
					return fmt.Errorf("injecting auth into deamonset: %v", err)
				}

				node.DaemonSet = *ds

			default:
				return fmt.Errorf("unsupported module type %s", m.Name)

			}

		}
	}

	// Create/Update ServiceAccount
	err = serviceaccount.SyncServiceAccount(ctx, &node.Rbac.ServiceAccount, r.Client, reqLogger)
	if err != nil {
		return err
	}
	err = serviceaccount.SyncServiceAccount(ctx, &controller.Rbac.ServiceAccount, r.Client, reqLogger)
	if err != nil {
		return err
	}

	// Create/Update ClusterRoles
	_, err = rbac.SyncClusterRole(ctx, &node.Rbac.ClusterRole, r.Client, reqLogger)
	if err != nil {
		return err
	}
	_, err = rbac.SyncClusterRole(ctx, &controller.Rbac.ClusterRole, r.Client, reqLogger)
	if err != nil {
		return err
	}

	// Create/Update ClusterRoleBinding
	err = rbac.SyncClusterRoleBindings(ctx, &node.Rbac.ClusterRoleBinding, r.Client, reqLogger)
	if err != nil {
		return err
	}
	err = rbac.SyncClusterRoleBindings(ctx, &controller.Rbac.ClusterRoleBinding, r.Client, reqLogger)
	if err != nil {
		return err
	}

	// Create/Update CSIDriver
	err = csidriver.SyncCSIDriver(ctx, driver, r.Client, reqLogger)
	if err != nil {
		return err
	}

	// Create/Update ConfigMap
	err = configmap.SyncConfigMap(ctx, configMap, r.Client, reqLogger)
	if err != nil {
		return err
	}

	// Create/Update Deployment
	err = deployment.SyncDeployment(ctx, &controller.Deployment, r.Client, reqLogger)
	if err != nil {
		return err
	}

	// Create/Update DeamonSet
	err = daemonset.SyncDaemonset(ctx, &node.DaemonSet, r.Client, reqLogger)
	if err != nil {
		return err
	}

	return nil
}

func (r *ContainerStorageModuleReconciler) PreChecks(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig,
	reqLogger logr.Logger) error {
	if cr.Spec.Driver.Common.Image == "" {
		return fmt.Errorf("driver image not specified in spec")
	}
	if cr.Spec.Driver.ConfigVersion == "" {
		return fmt.Errorf("driver version not specified in spec")
	}

	// Check drivers
	switch cr.Spec.Driver.CSIDriverType {
	case csmv1.PowerScale:

		err := drivers.PrecheckPowerScale(ctx, cr, r, reqLogger)
		if err != nil {
			return fmt.Errorf("failed powerscale validation: %v", err)
		}

	default:
		return fmt.Errorf("unsupported driver type %s", cr.Spec.Driver.CSIDriverType)
	}

	// check modules
	for _, m := range cr.Spec.Modules {
		if m.Enabled {
			switch m.Name {
			case csmv1.Authorization:
				err := modules.AuthorizationPrecheck(ctx, cr, m, r, reqLogger)
				if err != nil {
					return fmt.Errorf("failed authorization validation: %v", err)
				}

			default:
				return fmt.Errorf("unsupported module type %s", m.Name)

			}

		}
	}

	return nil

}

func checkAndApplyConfigVersionAnnotations(instance csmv1.ContainerStorageModule, log logr.Logger, update bool) (bool, error) {
	if instance.Spec.Driver.ConfigVersion == "" {
		// fail immediately
		return false, fmt.Errorf("mandatory argument: ConfigVersion missing")
	}
	// If driver has not been initialized yet, we first annotate the driver with the config version annotation

	if instance.Status.ContainerStorageModuleHash == 0 || update {
		annotations := instance.GetAnnotations()
		isUpdated := false
		if annotations == nil {
			annotations = make(map[string]string)
			isUpdated = true
		}
		if configVersion, ok := annotations[configVersionKey]; !ok {
			annotations[configVersionKey] = instance.Spec.Driver.ConfigVersion
			isUpdated = true
			instance.SetAnnotations(annotations)
			log.Info(fmt.Sprintf("Installing CSI Driver %s with config Version %s. Updating Annotations with Config Version",
				instance.GetName(), instance.Spec.Driver.ConfigVersion))
		} else {
			if configVersion != instance.Spec.Driver.ConfigVersion {
				annotations[configVersionKey] = instance.Spec.Driver.ConfigVersion
				isUpdated = true
				instance.SetAnnotations(annotations)
				log.Info(fmt.Sprintf("Config Version changed from %s to %s. Updating Annotations",
					configVersion, instance.Spec.Driver.ConfigVersion))
			}
		}
		return isUpdated, nil
	}
	return false, nil
}

// GetClient - returns the split client
func (r *ContainerStorageModuleReconciler) GetClient() client.Client {
	return r.Client
}

// IncrUpdateCount - Increments the update count
func (r *ContainerStorageModuleReconciler) IncrUpdateCount() {
	atomic.AddInt32(&r.updateCount, 1)
}

// GetUpdateCount - Returns the current update count
func (r *ContainerStorageModuleReconciler) GetUpdateCount() int32 {
	return r.updateCount
}
