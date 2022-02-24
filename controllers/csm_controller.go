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
	"sync/atomic"
	"time"

	k8sClient "github.com/dell/csm-operator/k8s"
	"github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/modules"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/resources/configmap"
	"github.com/dell/csm-operator/pkg/resources/csidriver"
	"github.com/dell/csm-operator/pkg/resources/daemonset"
	"github.com/dell/csm-operator/pkg/resources/deployment"
	"github.com/dell/csm-operator/pkg/resources/rbac"
	"github.com/dell/csm-operator/pkg/resources/serviceaccount"
	"github.com/dell/csm-operator/pkg/utils"
	"go.uber.org/zap"

	k8serror "k8s.io/apimachinery/pkg/api/errors"
	t1 "k8s.io/apimachinery/pkg/types"
	sinformer "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
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
	"sync"
)

var dMutex sync.RWMutex

// ContainerStorageModuleReconciler reconciles a ContainerStorageModule object
type ContainerStorageModuleReconciler struct {
	// controller runtime client, responsible for create, delete, update, get etc.
	client.Client
	// k8s client, implements client-go/kubernetes interface, responsible for apply, which
	// client.Client does not provides
	K8sClient     kubernetes.Interface
	Scheme        *runtime.Scheme
	Log           *zap.SugaredLogger
	Config        utils.OperatorConfig
	updateCount   int32
	trcID         string
	EventRecorder record.EventRecorder
}

// DriverConfig  -
type DriverConfig struct {
	Driver     *storagev1.CSIDriver
	ConfigMap  *corev1.ConfigMap
	Node       *utils.NodeYAML
	Controller *utils.ControllerYAML
}

// MetadataPrefix - prefix for all labels & annotations
const (
	// MetadataPrefix - prefix for all labels & annotations
	MetadataPrefix = "storage.dell.com"

	// NodeYaml - yaml file name for node
	NodeYaml = "node.yaml"

	// CSMFinalizerName -
	CSMFinalizerName = "finalizer.dell.emc.com"
)

var configVersionKey = fmt.Sprintf("%s/%s", MetadataPrefix, "CSIoperatorConfigVersion")

//+kubebuilder:rbac:groups=storage.dell.com,resources=containerstoragemodules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storage.dell.com,resources=containerstoragemodules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storage.dell.com,resources=containerstoragemodules/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods;services;services/finalizers;endpoints;persistentvolumeclaims;events;configmaps;secrets;serviceaccounts,verbs=*
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;create;patch;update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;create;delete;patch;update
// +kubebuilder:rbac:groups="apps",resources=deployments;daemonsets;replicasets;statefulsets,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles;clusterrolebindings;replicasets;rolebindings,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles/finalizers,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="monitoring.coreos.com",resources=servicemonitors,verbs=get;create
// +kubebuilder:rbac:groups="",resources=deployments/finalizers,resourceNames=dell-csm-operator-controller-manager,verbs=update
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
	r.IncrUpdateCount()
	r.trcID = fmt.Sprintf("%d", r.GetUpdateCount())
	name := req.Name + "-" + r.trcID
	ctx, log := logger.GetNewContextWithLogger(name)
	log.Info("################Starting Reconcile##############")
	csm := new(csmv1.ContainerStorageModule)

	log.Infow("reconcile for", "Namespace", req.Namespace, "Name", req.Name, "Attempt", r.GetUpdateCount())
	log.Debugw(fmt.Sprintf("Reconciling %s ", "csm"), "request", req.String())

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

	// Remove finalizer to allow users to delete CR
	operatorConfig := &utils.OperatorConfig{
		IsOpenShift:     r.Config.IsOpenShift,
		K8sVersion:      r.Config.K8sVersion,
		ConfigDirectory: r.Config.ConfigDirectory,
	}

	if csm.IsBeingDeleted() {
		r.Log.Info(fmt.Sprintf("HandleFinalizer for %v", req.NamespacedName))

		// check for force cleanup
		if csm.Spec.Driver.ForceRemoveDriver {
			// remove all resource deployed from CR by operator
			if err := r.removeDriver(ctx, *csm, *operatorConfig); err != nil {
				r.EventRecorder.Event(csm, corev1.EventTypeWarning, "Removing Driver", fmt.Sprintf("Failed to remove driver: %s", err))
				return ctrl.Result{}, fmt.Errorf("error when deleteing driver: %v", err)
			}

		}

		if err := r.removeFinalizer(csm); err != nil {
			r.EventRecorder.Event(csm, corev1.EventTypeWarning, "Deleting finalizer", fmt.Sprintf("Failed to delete finalizer: %s", err))
			return ctrl.Result{}, fmt.Errorf("error when handling finalizer: %v", err)
		}
		r.EventRecorder.Event(csm, corev1.EventTypeNormal, "Deleted", "Object finalizer is deleted")

		return ctrl.Result{}, nil
	}

	// Add finalizer
	if !csm.HasFinalizer(CSMFinalizerName) {
		r.Log.Info(fmt.Sprintf("AddFinalizer for %v", req.NamespacedName))
		if err := r.addFinalizer(csm); err != nil {
			r.EventRecorder.Event(csm, corev1.EventTypeWarning, "Adding finalizer", fmt.Sprintf("Failed to add finalizer: %s", err))
			return ctrl.Result{}, fmt.Errorf("error when adding finalizer: %v", err)
		}
		r.EventRecorder.Event(csm, corev1.EventTypeNormal, "Added", "Object finalizer is added")
		return ctrl.Result{}, nil
	}

	oldStatus := csm.GetCSMStatus()

	// Before doing anything else, check for config version and apply annotation if not set
	isUpdated, err := checkAndApplyConfigVersionAnnotations(csm, false)
	if err != nil {
		r.EventRecorder.Eventf(csm, corev1.EventTypeWarning, "Updated", "Failed add annotation during install: %s", err.Error())
		return utils.HandleValidationError(ctx, csm, r, err)
	}
	if isUpdated {
		err = r.GetClient().Update(ctx, csm)
		if err != nil {
			log.Error(err, "Failed to update CR with finalizer")
			return reconcile.Result{}, err
		}
	}

	// perfrom prechecks
	err = r.PreChecks(ctx, csm, *operatorConfig)
	if err != nil {
		r.EventRecorder.Event(csm, corev1.EventTypeWarning, "Running Pre-Check", fmt.Sprintf("Failed Prechecks: %s", err))
		return utils.HandleValidationError(ctx, csm, r, err)
	}

	// Set the driver status to updating

	newStatus := csm.GetCSMStatus()
	_, err = utils.HandleSuccess(ctx, csm, r, newStatus, oldStatus)
	if err != nil {
		log.Error(err, "Failed to update CR status")
	}
	// Update the driver
	r.EventRecorder.Eventf(csm, corev1.EventTypeNormal, "Updated", "Call install/update driver: %s", csm.Name)
	syncErr := r.SyncCSM(ctx, *csm, *operatorConfig)
	if syncErr == nil {
		return utils.LogBannerAndReturn(reconcile.Result{}, nil)
	}

	// Failed driver deployment
	r.EventRecorder.Eventf(csm, "Warning", "Updated", "Failed install: %s", syncErr.Error())

	return utils.LogBannerAndReturn(reconcile.Result{Requeue: true}, syncErr)
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
	dMutex.Lock()
	defer dMutex.Unlock()

	old, _ := oldObj.(*appsv1.Deployment)
	d, _ := obj.(*appsv1.Deployment)
	name := d.Spec.Template.Labels["csm"]
	key := name + "-" + fmt.Sprintf("%d", r.GetUpdateCount())
	ctx, log := logger.GetNewContextWithLogger(key)
	if name == "" {
		r.Log.Info("ignore deployment not labeled for csm", "name", d.Name)
		return
	}

	log.Debugw("deployment modified generation", fmt.Sprintf("%d", d.Generation), fmt.Sprintf("%d", old.Generation))

	desired := d.Status.Replicas
	available := d.Status.AvailableReplicas
	ready := d.Status.ReadyReplicas
	numberUnavailable := d.Status.UnavailableReplicas

	//Replicas:               2 desired | 2 updated | 2 total | 2 available | 0 unavailable

	log.Infow("deployment", "desired", desired)
	log.Infow("deployment", "numberReady", ready)
	log.Infow("deployment", "available", available)
	log.Infow("deployment", "numberUnavailable", numberUnavailable)

	ns := d.Namespace
	log.Debugw("deployment", "namespace", ns, "name", name)
	namespacedName := t1.NamespacedName{
		Name:      name,
		Namespace: ns,
	}

	csm := new(csmv1.ContainerStorageModule)
	err := r.Client.Get(ctx, namespacedName, csm)
	if err != nil {
		log.Error("deployment get csm", "error", err.Error())
	}
	newStatus := csm.GetCSMStatus()
	err = utils.UpdateStatus(ctx, csm, r, newStatus)
	if err != nil {
		log.Debugw("deployment status ", "pods", err.Error())
	}

}

func (r *ContainerStorageModuleReconciler) handlePodsUpdate(oldObj interface{}, obj interface{}) {
	dMutex.Lock()
	defer dMutex.Unlock()

	p, _ := obj.(*corev1.Pod)
	name := p.GetLabels()["csm"]
	if name == "" {
		return
	}
	key := name + "-" + fmt.Sprintf("%d", r.GetUpdateCount())
	ctx, log := logger.GetNewContextWithLogger(key)

	log.Infow("pod labeled for csm", "name", p.Name)

	// update it to trigger status watch
	ns := p.Namespace
	namespacedName := t1.NamespacedName{
		Name:      name,
		Namespace: ns,
	}
	csm := new(csmv1.ContainerStorageModule)
	err := r.Client.Get(ctx, namespacedName, csm)
	if err != nil {
		r.Log.Error("daemonset get csm", "error", err.Error())
	}
	log.Infow("csm prev status ", "state", csm.Status)
	newStatus := csm.GetCSMStatus()

	err = utils.UpdateStatus(ctx, csm, r, newStatus)
	state := csm.GetCSMStatus().NodeStatus.Failed
	stamp := fmt.Sprintf("at %d", time.Now().UnixNano())
	if state != "0" && err != nil {
		log.Infow("pod status ", "state", err.Error())
		r.EventRecorder.Eventf(csm, "Warning", "Updated", "%s Pod error details %s", stamp, err.Error())
	} else {
		r.EventRecorder.Eventf(csm, "Normal", "Complete", "%s Driver pods running OK", stamp)
	}

}

func (r *ContainerStorageModuleReconciler) handleDaemonsetUpdate(oldObj interface{}, obj interface{}) {
	dMutex.Lock()
	defer dMutex.Unlock()

	old, _ := oldObj.(*appsv1.DaemonSet)
	d, _ := obj.(*appsv1.DaemonSet)
	name := d.Spec.Template.Labels["csm"]
	if name == "" {
		r.Log.Debugw("ignore daemonset not labeled for csm", "name", d.Name)
		return
	}

	key := name + "-" + fmt.Sprintf("%d", r.GetUpdateCount())
	ctx, log := logger.GetNewContextWithLogger(key)

	log.Debugw("daemonset modified generation", fmt.Sprintf("%d", d.Generation), fmt.Sprintf("%d", old.Generation))

	desired := d.Status.DesiredNumberScheduled
	available := d.Status.NumberAvailable
	ready := d.Status.NumberReady
	numberUnavailable := d.Status.NumberUnavailable

	log.Infow("daemonset ", "name", d.Name, "namespace", d.Namespace)
	log.Infow("daemonset ", "desired", desired)
	log.Infow("daemonset ", "numberReady", ready)
	log.Infow("daemonset ", "available", available)
	log.Infow("daemonset ", "numberUnavailable", numberUnavailable)

	ns := d.Namespace
	r.Log.Debugw("daemonset ", "ns", ns, "name", name)
	namespacedName := t1.NamespacedName{
		Name:      name,
		Namespace: ns,
	}

	csm := new(csmv1.ContainerStorageModule)
	err := r.Client.Get(ctx, namespacedName, csm)
	if err != nil {
		r.Log.Error("daemonset get csm", "error", err.Error())
	}

	log.Infow("csm prev status ", "state", csm.Status)
	newStatus := csm.GetCSMStatus()
	err = utils.UpdateStatus(ctx, csm, r, newStatus)
	if err != nil {
		log.Debugw("daemonset status ", "pods", err.Error())
	}

}

// ContentWatch - watch updates on deployment and deamonset
func (r *ContainerStorageModuleReconciler) ContentWatch() error {

	clientset, err := k8sClient.GetClientSetWrapper()
	if err != nil {
		r.Log.Error(err, err.Error())
	}

	sharedInformerFactory := sinformer.NewSharedInformerFactory(clientset, time.Duration(time.Hour))
	daemonsetInformer := sharedInformerFactory.Apps().V1().DaemonSets().Informer()
	deploymentInformer := sharedInformerFactory.Apps().V1().Deployments().Informer()
	daemonsetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: r.handleDaemonsetUpdate,
	})
	deploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: r.handleDeploymentUpdate,
	})
	podsInformer := sharedInformerFactory.Core().V1().Pods().Informer()
	podsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: r.handlePodsUpdate,
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
		}).Complete(r)
}

func (r *ContainerStorageModuleReconciler) removeFinalizer(instance *csmv1.ContainerStorageModule) error {
	if !instance.HasFinalizer(CSMFinalizerName) {
		return nil
	}
	instance.SetFinalizers(nil)
	return r.Update(context.Background(), instance)
}

func (r *ContainerStorageModuleReconciler) addFinalizer(instance *csmv1.ContainerStorageModule) error {
	instance.SetFinalizers([]string{CSMFinalizerName})
	return r.Update(context.Background(), instance)
}

// SyncCSM - Sync the current installation - this can lead to a create or update
func (r *ContainerStorageModuleReconciler) SyncCSM(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) error {
	log := logger.GetLogger(ctx)

	// Get Driver resources
	driverConfig, err := r.getDriverConfig(ctx, cr, operatorConfig)
	if err != nil {
		return err
	}

	driver := driverConfig.Driver
	configMap := driverConfig.ConfigMap
	node := driverConfig.Node
	controller := driverConfig.Controller

	// Add module resources
	for _, m := range cr.Spec.Modules {
		if m.Enabled {
			switch m.Name {
			case csmv1.Authorization:
				log.Info("Injecting CSM Authorization")
				dp, err := modules.AuthInjectDeployment(controller.Deployment, cr, operatorConfig)
				if err != nil {
					return fmt.Errorf("injecting auth into deployment: %v", err)
				}
				controller.Deployment = *dp

				ds, err := modules.AuthInjectDaemonset(node.DaemonSetApplyConfig, cr, operatorConfig)
				if err != nil {
					return fmt.Errorf("injecting auth into deamonset: %v", err)
				}

				node.DaemonSetApplyConfig = *ds

			default:
				return fmt.Errorf("unsupported module type %s", m.Name)

			}

		}
	}

	// Create/Update ServiceAccount
	err = serviceaccount.SyncServiceAccount(ctx, &node.Rbac.ServiceAccount, r.Client)
	if err != nil {
		return err
	}
	err = serviceaccount.SyncServiceAccount(ctx, &controller.Rbac.ServiceAccount, r.Client)
	if err != nil {
		return err
	}

	// Create/Update ClusterRoles
	_, err = rbac.SyncClusterRole(ctx, &node.Rbac.ClusterRole, r.Client)
	if err != nil {
		return err
	}
	_, err = rbac.SyncClusterRole(ctx, &controller.Rbac.ClusterRole, r.Client)
	if err != nil {
		return err
	}

	// Create/Update ClusterRoleBinding
	err = rbac.SyncClusterRoleBindings(ctx, &node.Rbac.ClusterRoleBinding, r.Client)
	if err != nil {
		return err
	}
	err = rbac.SyncClusterRoleBindings(ctx, &controller.Rbac.ClusterRoleBinding, r.Client)
	if err != nil {
		return err
	}

	// Create/Update CSIDriver
	err = csidriver.SyncCSIDriver(ctx, driver, r.Client)
	if err != nil {
		return err
	}

	// Create/Update ConfigMap
	err = configmap.SyncConfigMap(ctx, configMap, r.Client)
	if err != nil {
		return err
	}

	// Create/Update Deployment
	err = deployment.SyncDeployment(ctx, &controller.Deployment, r.K8sClient, cr.Name)
	if err != nil {
		return err
	}

	// Create/Update DeamonSet

	err = daemonset.SyncDaemonset(ctx, &node.DaemonSetApplyConfig, r.K8sClient, cr.Name)
	if err != nil {
		return err
	}
	return nil

}

func (r *ContainerStorageModuleReconciler) getDriverConfig(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) (*DriverConfig, error) {
	var (
		err        error
		driver     *storagev1.CSIDriver
		configMap  *corev1.ConfigMap
		node       *utils.NodeYAML
		controller *utils.ControllerYAML
	)

	log := logger.GetLogger(ctx)

	// Get Driver resources
	log.Infof("Getting %s CSI Driver for Dell EMC", cr.Spec.Driver.CSIDriverType)
	driverType := cr.Spec.Driver.CSIDriverType

	if driverType == csmv1.PowerScale {
		// use powerscale instead of isilon as the folder name is powerscale
		driverType = csmv1.PowerScaleName
	}

	configMap, err = drivers.GetConfigMap(ctx, cr, operatorConfig, driverType)
	if err != nil {
		return nil, fmt.Errorf("getting %s configMap: %v", driverType, err)
	}

	driver, err = drivers.GetCSIDriver(ctx, cr, operatorConfig, driverType)
	if err != nil {
		return nil, fmt.Errorf("getting %s CSIDriver: %v", driverType, err)
	}

	node, err = drivers.GetNode(ctx, cr, operatorConfig, driverType, NodeYaml)
	if err != nil {
		return nil, fmt.Errorf("getting %s node: %v", driverType, err)
	}

	controller, err = drivers.GetController(ctx, cr, operatorConfig, driverType)
	if err != nil {
		return nil, fmt.Errorf("getting %s controller: %v", driverType, err)
	}

	return &DriverConfig{
		Driver:     driver,
		ConfigMap:  configMap,
		Node:       node,
		Controller: controller,
	}, nil

}

func (r *ContainerStorageModuleReconciler) removeDriver(ctx context.Context, instance csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) error {

	log := logger.GetLogger(ctx)

	deleteObj := func(obj client.Object) error {
		err := r.GetClient().Get(ctx, t1.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, obj)
		if err != nil && k8serror.IsNotFound(err) {
			return nil
		} else if err != nil {
			log.Info("Unknown error.", "Error", err.Error())
			return err
		} else {
			log.Info("Deleting object", "Name:", obj.GetName(), "Kind:", obj.GetObjectKind().GroupVersionKind().Kind)
			err = r.GetClient().Delete(ctx, obj)
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Get Driver resources
	driverConfig, err := r.getDriverConfig(ctx, instance, operatorConfig)
	if err != nil {
		return err
	}

	err = deleteObj(&driverConfig.Node.Rbac.ServiceAccount)
	if err != nil {
		return err
	}
	err = deleteObj(&driverConfig.Controller.Rbac.ServiceAccount)
	if err != nil {
		return err
	}

	err = deleteObj(&driverConfig.Node.Rbac.ClusterRole)
	if err != nil {
		return err
	}
	err = deleteObj(&driverConfig.Controller.Rbac.ClusterRole)
	if err != nil {
		return err
	}

	err = deleteObj(&driverConfig.Node.Rbac.ClusterRoleBinding)
	if err != nil {
		return err
	}
	err = deleteObj(&driverConfig.Controller.Rbac.ClusterRoleBinding)
	if err != nil {
		return err
	}

	err = deleteObj(driverConfig.ConfigMap)
	if err != nil {
		return err
	}

	err = deleteObj(driverConfig.Driver)
	if err != nil {
		return err
	}

	daemonsetKey := client.ObjectKey{
		Namespace: *driverConfig.Node.DaemonSetApplyConfig.Namespace,
		Name:      *driverConfig.Node.DaemonSetApplyConfig.Name,
	}
	daemonsetObj := &appsv1.DaemonSet{}
	if err = r.Get(ctx, daemonsetKey, daemonsetObj); err == nil {
		r.Delete(ctx, daemonsetObj)
	}

	deploymentKey := client.ObjectKey{
		Namespace: *driverConfig.Controller.Deployment.Namespace,
		Name:      *driverConfig.Controller.Deployment.Name,
	}
	deploymentObj := &appsv1.Deployment{}
	if err = r.Get(ctx, deploymentKey, deploymentObj); err == nil {
		r.Delete(ctx, deploymentObj)
	}

	return nil
}

// PreChecks - validate input values
func (r *ContainerStorageModuleReconciler) PreChecks(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) error {
	if cr.Spec.Driver.Common.Image == "" {
		return fmt.Errorf("driver image not specified in spec")
	}
	if cr.Spec.Driver.ConfigVersion == "" || cr.Spec.Driver.ConfigVersion != "v2.1.0" {
		return fmt.Errorf("driver version not specified in spec or driver version is not valid")
	}

	// add check for version

	// Check drivers
	switch cr.Spec.Driver.CSIDriverType {
	case csmv1.PowerScale:

		err := drivers.PrecheckPowerScale(ctx, cr, r)
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
				err := modules.AuthorizationPrecheck(ctx, cr.GetNamespace(), string(cr.Spec.Driver.CSIDriverType), operatorConfig, m, r.GetClient())
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

func checkAndApplyConfigVersionAnnotations(instance *csmv1.ContainerStorageModule, update bool) (bool, error) {
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
		fmt.Println(fmt.Sprintf("Installing CSI Driver %s with config Version %s. Updating Annotations with Config Version",
			instance.GetName(), instance.Spec.Driver.ConfigVersion))
	} else {
		if configVersion != instance.Spec.Driver.ConfigVersion {
			annotations[configVersionKey] = instance.Spec.Driver.ConfigVersion
			isUpdated = true
			instance.SetAnnotations(annotations)
			fmt.Println(fmt.Sprintf("Config Version changed from %s to %s. Updating Annotations",
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
