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
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	k8sClient "github.com/dell/csm-operator/k8s"
	"github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/modules"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/resources/configmap"
	"github.com/dell/csm-operator/pkg/resources/csidriver"
	"github.com/dell/csm-operator/pkg/resources/daemonset"
	"github.com/dell/csm-operator/pkg/resources/deployment"
	"github.com/dell/csm-operator/pkg/resources/rbac"
	"github.com/dell/csm-operator/pkg/resources/serviceaccount"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/go-logr/logr"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	t1 "k8s.io/apimachinery/pkg/types"
	sinformer "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ContainerStorageModuleReconciler reconciles a ContainerStorageModule object
type ContainerStorageModuleReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
	Config        utils.OperatorConfig
	updateCount   int32
	EventRecorder record.EventRecorder
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
// +kubebuilder:rbac:groups="csi.storage.k8s.io",resources=csinodeinfos,verbs=get;list;watch
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

// Reconcile - main loop
func (r *ContainerStorageModuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("csm", req.NamespacedName)
	// your logic here
	csm := new(csmv1.ContainerStorageModule)

	reqLogger := log.WithValues("Namespace", req.Namespace)
	reqLogger = reqLogger.WithValues("Name", req.Name)
	reqLogger = reqLogger.WithValues("Attempt", r.updateCount)
	reqLogger.Info(fmt.Sprintf("Reconciling %s ", "csm"), "request", req.String())

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
	err = r.Client.Get(ctx, req.NamespacedName, csm)
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
	isUpdated, err := checkAndApplyConfigVersionAnnotations(csm, log, false)
	if err != nil {
		return utils.HandleValidationError(ctx, csm, r, reqLogger, err)
	}
	if isUpdated {
		_ = r.GetClient().Update(ctx, csm)
		return reconcile.Result{Requeue: true}, nil
	}

	// perfrom prechecks
	err = r.PreChecks(ctx, csm, *driverConfig, reqLogger)
	if err != nil {
		return utils.HandleValidationError(ctx, csm, r, reqLogger, err)
	}
	r.EventRecorder.Eventf(csm, "Normal", "Updated", "PreChecks ok: %s", csm.Name)

	// Set the driver status to updating

	oldStatus := csm.GetCSMStatus()
	newStatus := oldStatus
	utils.HandleSuccess(ctx, csm, r, reqLogger, newStatus, oldStatus)

	// Update the driver
	r.EventRecorder.Eventf(csm, "Normal", "Updated", "Call install/update driver: %s", csm.Name)
	syncErr := r.SyncCSM(ctx, *csm, *driverConfig, reqLogger)
	_, _ = utils.CalculateState(ctx, csm, r, newStatus)
	if syncErr == nil {
		// driver state is succeeded
		r.EventRecorder.Eventf(csm, "Normal", "Updated", "Driver install OK: reconcile count=%d name=%s", r.updateCount, csm.Name)
		return utils.LogBannerAndReturn(reconcile.Result{}, err, reqLogger)
	}

	// Failed to sync driver deployment
	r.EventRecorder.Eventf(csm, "Warning", "Updated", "Failed  install: %s", syncErr.Error())

	return utils.LogBannerAndReturn(reconcile.Result{Requeue: false}, syncErr, reqLogger)
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

func (r *ContainerStorageModuleReconciler) ignoreUpdatePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates to status in which case metadata.Generation does not change
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Evaluates to false if the object has been confirmed deleted.
			return !e.DeleteStateUnknown
		},
	}
}

func (r *ContainerStorageModuleReconciler) handleDeploymentUpdate(oldObj interface{}, obj interface{}) {

	old, _ := oldObj.(*appsv1.Deployment)
	d, _ := obj.(*appsv1.Deployment)
	name := d.Spec.Template.Labels["csm"]
	if name == "" {
		r.Log.Info("deployment not found", "name", d.Name)
		return
	}

	r.Log.Info("deployment modified generation", fmt.Sprintf("%d", d.Generation), fmt.Sprintf("%d", old.Generation))
	desired := d.Status.UpdatedReplicas
	available := d.Status.AvailableReplicas
	ready := d.Status.ReadyReplicas

	r.Log.Info("deployment csm", "desired", desired)
	r.Log.Info("deployment csm", "numberReady", ready)
	r.Log.Info("deployment csm", "available", available)

	ns := d.Namespace
	r.Log.Info("deployment", "namespace", ns, "name", name)
	namespacedName := t1.NamespacedName{
		Name:      name,
		Namespace: ns,
	}

	csm := new(csmv1.ContainerStorageModule)
	ctx := context.Background()
	err := r.Client.Get(ctx, namespacedName, csm)
	if err != nil {
		r.Log.Info("deployment get csm", "error", err.Error())
	}

	// get status and update csm

	r.Log.Info("csm prev status", "state", csm.Status)

	state, err := utils.CalculateState(ctx, csm, r, csm.GetCSMStatus())
	if err != nil {
		r.Log.Info("Failed to update Deployment status", "error", err.Error())
	}
	r.Log.Info("deployment status", "state", state)

	if !state {
		errorMsg := errors.New("deployment in error")
		if err != nil {
			errorMsg = err
		}
		err := utils.UpdateStatus(ctx, csm, r, r.Log, csm.GetCSMStatus())
		if err != nil {
			r.Log.Info("Failed to update Deployment status", "error", err.Error())
		}
		r.Log.Info("deployment in err", "err", errorMsg)

		controllerstart := len(csm.Status.ControllerStatus.Starting)
		controllerstop := len(csm.Status.ControllerStatus.Stopped)
		r.EventRecorder.Eventf(csm, "Warning", "Updated", "Deployment status check csm, controller pod count starting:%d, stopped:%d", controllerstart, controllerstop)

		r.EventRecorder.Eventf(csm, "Warning", "Updated", "Deployment status check Error ,controller pod desired:%d, available:%d", desired, available)
	} else {
		r.Log.Info("csm status", "curent state", csm.Status.State)
		err := utils.UpdateStatus(ctx, csm, r, r.Log, csm.GetCSMStatus())
		if err != nil {
			r.Log.Info("Failed to update Deployment status", "error", err.Error())
		}
		r.EventRecorder.Eventf(csm, "Normal", "Updated", "Deployment status check OK : %s desired pods %d, ready pods %d", d.Name, desired, ready)
	}
	return
}

func (r *ContainerStorageModuleReconciler) handleDaemonsetUpdate(oldObj interface{}, obj interface{}) {
	old, _ := oldObj.(*appsv1.DaemonSet)
	d, _ := obj.(*appsv1.DaemonSet)
	name := d.Spec.Template.Labels["csm"]
	if name == "" {
		r.Log.Info("daemonset not found", "name", d.Name)
		return
	}

	r.Log.Info("daemonset modified generation", fmt.Sprintf("%d", d.Generation), fmt.Sprintf("%d", old.Generation))
	desired := d.Status.DesiredNumberScheduled
	available := d.Status.NumberAvailable
	ready := d.Status.NumberReady

	r.Log.Info("daemonset csm", "desired", desired)
	r.Log.Info("daemonset csm", "numberReady", ready)
	r.Log.Info("daemonset csm", "available", available)

	ns := d.Namespace
	r.Log.Info("daemonset ", "ns", ns, "name", name)
	namespacedName := t1.NamespacedName{
		Name:      name,
		Namespace: ns,
	}

	csm := new(csmv1.ContainerStorageModule)
	ctx := context.Background()
	err := r.Client.Get(ctx, namespacedName, csm)
	if err != nil {
		r.Log.Info("daemonset get csm", "error", err.Error())
	}
	// get status and update csm

	r.Log.Info("csm prev status ", "state", csm.Status)

	state, err := utils.CalculateState(ctx, csm, r, csm.GetCSMStatus())
	if err != nil {
		r.Log.Info("Failed to update Deployment status", "error", err.Error())
	}
	r.Log.Info("daemonset status", "state", state)

	if !state {
		errorMsg := errors.New("daemonset in error")
		if err != nil {
			errorMsg = err
		}
		err := utils.UpdateStatus(ctx, csm, r, r.Log, csm.GetCSMStatus())
		if err != nil {
			r.Log.Info("Failed to update Daemonset status", "error", err.Error())
		}
		r.Log.Info("daemonset in err", "err", errorMsg)

		nodestart := len(csm.Status.NodeStatus.Starting)
		nodestop := len(csm.Status.NodeStatus.Stopped)
		r.EventRecorder.Eventf(csm, "Warning", "Updated", "Daemonset status check csm, node pod count starting:%d, stopped:%d", nodestart, nodestop)

		r.EventRecorder.Eventf(csm, "Warning", "Updated", "Daemonset status check Error ,node pod desired:%d, available:%d", desired, available)
	} else {
		r.Log.Info("csm status", "curent state", csm.Status.State)
		err := utils.UpdateStatus(ctx, csm, r, r.Log, csm.GetCSMStatus())
		if err != nil {
			r.Log.Info("Failed to update Daemonset status", "error", err.Error())
		}
		r.EventRecorder.Eventf(csm, "Normal", "Updated", "Daemonset status check OK : %s desired pods %d, ready pods %d", d.Name, desired, ready)
	}
	return
}

// ContentWatch -watch
func (r *ContainerStorageModuleReconciler) ContentWatch() error {

	clientset, err := k8sClient.GetClientSetWrapper()
	if err != nil {
		r.Log.Info(err.Error(), "setup snapWatch", "test mode")
	}

	sharedInformerFactory := sinformer.NewSharedInformerFactory(clientset, time.Duration(time.Hour))

	contentInformer := sharedInformerFactory.Apps().V1().DaemonSets().Informer()
	contentdeploymentInformer := sharedInformerFactory.Apps().V1().Deployments().Informer()
	contentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: r.handleDaemonsetUpdate,
	})
	contentdeploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: r.handleDeploymentUpdate,
	})

	stop := make(chan struct{})
	sharedInformerFactory.Start(stop)

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ContainerStorageModuleReconciler) SetupWithManager(mgr ctrl.Manager, limiter ratelimiter.RateLimiter, maxReconcilers int) error {

	go r.ContentWatch()

	return ctrl.NewControllerManagedBy(mgr).
		For(&csmv1.ContainerStorageModule{}).
		WithEventFilter(r.ignoreUpdatePredicate()).
		WithOptions(controller.Options{
			RateLimiter:             limiter,
			MaxConcurrentReconciles: maxReconcilers,
		}).
		Complete(r)
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
				dp, err := modules.AuthInjectDeployment(controller.Deployment, cr, operatorConfig)
				if err != nil {
					return fmt.Errorf("injecting auth into deployment: %v", err)
				}
				controller.Deployment = *dp

				ds, err := modules.AuthInjectDaemonset(node.DaemonSet, cr, operatorConfig)
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
	err = deployment.SyncDeployment(ctx, &controller.Deployment, r.Client, reqLogger, cr.Name)
	if err != nil {
		return err
	}

	// Create/Update DeamonSet

	err = daemonset.SyncDaemonset(ctx, &node.DaemonSet, r.Client, reqLogger, cr.Name)
	if err != nil {
		return err
	}
	return nil
}

// PreChecks - validate input values
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
				err := modules.AuthorizationPrecheck(ctx, cr.GetNamespace(), string(cr.Spec.Driver.CSIDriverType), operatorConfig, m, r.GetClient(), reqLogger)
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

func checkAndApplyConfigVersionAnnotations(instance *csmv1.ContainerStorageModule, log logr.Logger, update bool) (bool, error) {
	if instance.Spec.Driver.ConfigVersion == "" {
		// fail immediately
		return false, fmt.Errorf("mandatory argument: ConfigVersion missing")
	}
	// If driver has not been initialized yet, we first annotate the driver with the config version annotation

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
