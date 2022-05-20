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

	"github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/modules"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/constants"
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

const (
	// MetadataPrefix - prefix for all labels & annotations
	MetadataPrefix = "storage.dell.com"

	// NodeYaml - yaml file name for node
	NodeYaml = "node.yaml"

	// CSMFinalizerName -
	CSMFinalizerName = "finalizer.dell.emc.com"
)

var (
	dMutex           sync.RWMutex
	configVersionKey = fmt.Sprintf("%s/%s", MetadataPrefix, "CSIoperatorConfigVersion")

	// StopWatch - watcher stop handle
	StopWatch = make(chan struct{})
)

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

	operatorConfig := &utils.OperatorConfig{
		IsOpenShift:     r.Config.IsOpenShift,
		K8sVersion:      r.Config.K8sVersion,
		ConfigDirectory: r.Config.ConfigDirectory,
	}

	if csm.IsBeingDeleted() {
		log.Infow("Delete request", "csm", req.Namespace, "Name", req.Name)

		// check for force cleanup
		if csm.Spec.Driver.ForceRemoveDriver {
			// remove all resource deployed from CR by operator
			if err := r.removeDriver(ctx, *csm, *operatorConfig); err != nil {
				r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventDeleted, fmt.Sprintf("Failed to remove driver: %s", err))
				log.Errorw("remove driver", "error", err.Error())
				return ctrl.Result{}, fmt.Errorf("error when deleteing driver: %v", err)
			}
		}

		if err := r.removeFinalizer(ctx, csm); err != nil {
			r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventDeleted, fmt.Sprintf("Failed to delete finalizer: %s", err))
			log.Errorw("remove driver finalizer", "error", err.Error())
			return ctrl.Result{}, fmt.Errorf("error when handling finalizer: %v", err)
		}
		r.EventRecorder.Event(csm, corev1.EventTypeNormal, csmv1.EventDeleted, "Object finalizer is deleted")

		return ctrl.Result{}, nil
	}

	// Add finalizer
	if !csm.HasFinalizer(CSMFinalizerName) {
		log.Infow("HandleFinalizer", "name", CSMFinalizerName)
		if err := r.addFinalizer(ctx, csm); err != nil {
			r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventUpdated, fmt.Sprintf("Failed to add finalizer: %s", err))
			log.Errorw("HandleFinalizer", "error", err.Error())
			return ctrl.Result{}, fmt.Errorf("error when adding finalizer: %v", err)
		}
		r.EventRecorder.Event(csm, corev1.EventTypeNormal, csmv1.EventUpdated, "Object finalizer is added")
	}

	oldStatus := csm.GetCSMStatus()

	// perform prechecks
	err = r.PreChecks(ctx, csm, *operatorConfig)
	if err != nil {
		csm.GetCSMStatus().State = constants.InvalidConfig
		r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventUpdated, fmt.Sprintf("Failed Prechecks: %s", err))
		return utils.HandleValidationError(ctx, csm, r, err)
	}

	// Set the driver annotation
	isUpdated := applyConfigVersionAnnotations(ctx, csm)
	if isUpdated {
		err = r.GetClient().Update(ctx, csm)
		if err != nil {
			log.Error(err, "Failed to update CR with annotation")
			return reconcile.Result{}, err
		}
	}

	newStatus := csm.GetCSMStatus()
	_, err = utils.HandleSuccess(ctx, csm, r, newStatus, oldStatus)
	if err != nil {
		log.Error(err, "Failed to update CR status")
	}
	// Update the driver
	syncErr := r.SyncCSM(ctx, *csm, *operatorConfig)
	if syncErr == nil {
		r.EventRecorder.Eventf(csm, corev1.EventTypeNormal, csmv1.EventCompleted, "install/update driver: %s completed OK", csm.Name)
		return utils.LogBannerAndReturn(reconcile.Result{}, nil)
	}

	// Failed driver deployment
	r.EventRecorder.Eventf(csm, corev1.EventTypeWarning, csmv1.EventUpdated, "Failed install: %s", syncErr.Error())

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
	name := d.Spec.Template.Labels[constants.CsmLabel]
	key := name + "-" + fmt.Sprintf("%d", r.GetUpdateCount())
	ctx, log := logger.GetNewContextWithLogger(key)
	if name == "" {
		return
	}

	log.Debugw("deployment modified generation", d.Generation, old.Generation)

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
	} else {
		r.EventRecorder.Eventf(csm, corev1.EventTypeNormal, csmv1.EventCompleted, "Driver deployment running OK")
	}

}

func (r *ContainerStorageModuleReconciler) handlePodsUpdate(oldObj interface{}, obj interface{}) {
	dMutex.Lock()
	defer dMutex.Unlock()

	p, _ := obj.(*corev1.Pod)
	name := p.GetLabels()[constants.CsmLabel]
	ns := p.Namespace
	if name == "" {
		return
	}
	key := name + "-" + fmt.Sprintf("%d", r.GetUpdateCount())
	ctx, log := logger.GetNewContextWithLogger(key)

	if !p.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Debugw("driver delete invoked", "stopping pod with name", p.Name)
		return
	}
	log.Infow("pod modified for driver", "name", p.Name)

	namespacedName := t1.NamespacedName{
		Name:      name,
		Namespace: ns,
	}
	csm := new(csmv1.ContainerStorageModule)
	err := r.Client.Get(ctx, namespacedName, csm)
	if err != nil {
		r.Log.Errorw("daemonset get csm", "error", err.Error())
	}
	log.Infow("csm prev status ", "state", csm.Status)
	newStatus := csm.GetCSMStatus()

	err = utils.UpdateStatus(ctx, csm, r, newStatus)
	state := csm.GetCSMStatus().State
	stamp := fmt.Sprintf("at %d", time.Now().UnixNano())
	if state != "0" && err != nil {
		log.Infow("pod status ", "state", err.Error())
		r.EventRecorder.Eventf(csm, corev1.EventTypeWarning, csmv1.EventUpdated, "%s Pod error details %s", stamp, err.Error())
	} else {
		r.EventRecorder.Eventf(csm, corev1.EventTypeNormal, csmv1.EventCompleted, "%s Driver pods running OK", stamp)
	}

}

func (r *ContainerStorageModuleReconciler) handleDaemonsetUpdate(oldObj interface{}, obj interface{}) {
	dMutex.Lock()
	defer dMutex.Unlock()

	old, _ := oldObj.(*appsv1.DaemonSet)
	d, _ := obj.(*appsv1.DaemonSet)
	name := d.Spec.Template.Labels[constants.CsmLabel]
	if name == "" {
		return
	}

	key := name + "-" + fmt.Sprintf("%d", r.GetUpdateCount())
	ctx, log := logger.GetNewContextWithLogger(key)

	log.Debugw("daemonset modified generation", "new", d.Generation, "old", old.Generation)

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
	} else {
		r.EventRecorder.Eventf(csm, corev1.EventTypeNormal, csmv1.EventCompleted, "Driver daemonset running OK")
	}

}

// ContentWatch - watch updates on deployment and deamonset
func (r *ContainerStorageModuleReconciler) ContentWatch() error {

	sharedInformerFactory := sinformer.NewSharedInformerFactory(r.K8sClient, time.Duration(time.Hour))

	daemonsetInformer := sharedInformerFactory.Apps().V1().DaemonSets().Informer()
	daemonsetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: r.handleDaemonsetUpdate,
	})

	deploymentInformer := sharedInformerFactory.Apps().V1().Deployments().Informer()
	deploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: r.handleDeploymentUpdate,
	})

	podsInformer := sharedInformerFactory.Core().V1().Pods().Informer()
	podsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: r.handlePodsUpdate,
	})

	sharedInformerFactory.Start(StopWatch)
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

func (r *ContainerStorageModuleReconciler) removeFinalizer(ctx context.Context, instance *csmv1.ContainerStorageModule) error {
	if !instance.HasFinalizer(CSMFinalizerName) {
		return nil
	}
	instance.SetFinalizers(nil)
	return r.Update(ctx, instance)
}

func (r *ContainerStorageModuleReconciler) addFinalizer(ctx context.Context, instance *csmv1.ContainerStorageModule) error {
	instance.SetFinalizers([]string{CSMFinalizerName})
	instance.GetCSMStatus().State = constants.Creating
	return r.Update(ctx, instance)
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

	replicationEnabled, clusterClients, err := utils.GetDefaultClusters(ctx, cr, r)
	if err != nil {
		return err
	}

	for _, m := range cr.Spec.Modules {
		if m.Enabled {
			switch m.Name {
			case csmv1.Authorization:
				if replicationEnabled { /* TODO(Michael): for now, Replication deployment is mutually exclusive with other module */
					return fmt.Errorf("cannot deploy replication with %s", m.Name)
				}
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
			case csmv1.Replication:
				log.Info("Injecting CSM Replication")
				dp, err := modules.ReplicationInjectDeployment(controller.Deployment, cr, operatorConfig)
				if err != nil {
					return fmt.Errorf("injecting replication into deployment: %v", err)
				}
				controller.Deployment = *dp

				clusterRole, err := modules.ReplicationInjectClusterRole(controller.Rbac.ClusterRole, cr, operatorConfig)
				if err != nil {
					return fmt.Errorf("injecting replication into controller cluster role: %v", err)
				}

				controller.Rbac.ClusterRole = *clusterRole

				err = modules.ReplicationInstallManagerController(ctx, operatorConfig, cr)
				if err != nil {
					return fmt.Errorf("failed top deploy replication controller: %v", err)
				}

			default:
				return fmt.Errorf("unsupported module type %s", m.Name)
			}

		}
	}

	for _, cluster := range clusterClients {
		// Create/Update ServiceAccount
		if err = serviceaccount.SyncServiceAccount(ctx, &node.Rbac.ServiceAccount, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		if err = serviceaccount.SyncServiceAccount(ctx, &controller.Rbac.ServiceAccount, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		// Create/Update ClusterRoles
		if err = rbac.SyncClusterRole(ctx, &node.Rbac.ClusterRole, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		if err = rbac.SyncClusterRole(ctx, &controller.Rbac.ClusterRole, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		// Create/Update ClusterRoleBinding
		if err = rbac.SyncClusterRoleBindings(ctx, &node.Rbac.ClusterRoleBinding, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		if err = rbac.SyncClusterRoleBindings(ctx, &controller.Rbac.ClusterRoleBinding, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		// Create/Update CSIDriver
		if err = csidriver.SyncCSIDriver(ctx, driver, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		// Create/Update ConfigMap
		if err = configmap.SyncConfigMap(ctx, configMap, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		// Create/Update Deployment
		if err = deployment.SyncDeployment(ctx, &controller.Deployment, cluster.ClusterK8sClient, cr.Name); err != nil {
			return err
		}

		// Create/Update DeamonSet
		if err = daemonset.SyncDaemonset(ctx, &node.DaemonSetApplyConfig, cluster.ClusterK8sClient, cr.Name); err != nil {
			return err
		}

	}

	return nil
}

func (r *ContainerStorageModuleReconciler) getDriverConfig(ctx context.Context,
	cr csmv1.ContainerStorageModule,
	operatorConfig utils.OperatorConfig) (*DriverConfig, error) {
	var (
		err        error
		driver     *storagev1.CSIDriver
		configMap  *corev1.ConfigMap
		node       *utils.NodeYAML
		controller *utils.ControllerYAML
		log        = logger.GetLogger(ctx)
	)

	// Get Driver resources
	log.Infof("Getting %s CSI Driver for Dell Technologies", cr.Spec.Driver.CSIDriverType)
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

	// Get Driver resources
	driverConfig, err := r.getDriverConfig(ctx, instance, operatorConfig)
	if err != nil {
		log.Error("error in getDriverConfig")
		return err
	}

	replicationEnabled, clusterClients, err := utils.GetDefaultClusters(ctx, instance, r)
	if err != nil {
		return err
	}
	for _, cluster := range clusterClients {
		if err = utils.DeleteObject(ctx, &driverConfig.Node.Rbac.ServiceAccount, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete node service account", "Error", err.Error())
			return err
		}

		if err = utils.DeleteObject(ctx, &driverConfig.Controller.Rbac.ServiceAccount, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete controller service account", "Error", err.Error())
			return err
		}

		if err = utils.DeleteObject(ctx, &driverConfig.Node.Rbac.ClusterRole, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete node cluster role", "Error", err.Error())
			return err
		}

		if err = utils.DeleteObject(ctx, &driverConfig.Controller.Rbac.ClusterRole, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete controller cluster role", "Error", err.Error())
			return err
		}

		if err = utils.DeleteObject(ctx, &driverConfig.Node.Rbac.ClusterRoleBinding, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete node cluster role binding", "Error", err.Error())
			return err
		}

		if err = utils.DeleteObject(ctx, &driverConfig.Controller.Rbac.ClusterRoleBinding, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete controller cluster role binding", "Error", err.Error())
			return err
		}

		if err = utils.DeleteObject(ctx, driverConfig.ConfigMap, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete configmap", "Error", err.Error())
			return err
		}

		if err = utils.DeleteObject(ctx, driverConfig.Driver, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete csi driver", "Error", err.Error())
			return err
		}

		daemonsetKey := client.ObjectKey{
			Namespace: *driverConfig.Node.DaemonSetApplyConfig.Namespace,
			Name:      *driverConfig.Node.DaemonSetApplyConfig.Name,
		}

		daemonsetObj := &appsv1.DaemonSet{}
		err = cluster.ClusterCTRLClient.Get(ctx, daemonsetKey, daemonsetObj)
		if err == nil {
			if err = cluster.ClusterCTRLClient.Delete(ctx, daemonsetObj); err != nil && !k8serror.IsNotFound(err) {
				log.Errorw("error delete daemonset", "Error", err.Error())
				return err
			}
		} else {
			log.Infow("error getting daemonset", "daemonsetKey", daemonsetKey)
		}

		deploymentKey := client.ObjectKey{
			Namespace: *driverConfig.Controller.Deployment.Namespace,
			Name:      *driverConfig.Controller.Deployment.Name,
		}

		deploymentObj := &appsv1.Deployment{}
		if err = cluster.ClusterCTRLClient.Get(ctx, deploymentKey, deploymentObj); err == nil {
			if err = cluster.ClusterCTRLClient.Delete(ctx, deploymentObj); err != nil && !k8serror.IsNotFound(err) {
				log.Errorw("error delete deployment", "Error", err.Error())
				return err
			}
		} else {
			log.Infow("error getting deployment", "deploymentKey", deploymentKey)
		}

		if replicationEnabled {
			log.Infow("Deleting Replication controller")
			if err = modules.ReplicationUninstallManagerController(ctx, operatorConfig, instance, cluster.ClusterCTRLClient); err != nil {
				log.Errorw("error deleting replication controller", err.Error())
				return err
			}
		}

	}

	return nil
}

// PreChecks - validate input values
func (r *ContainerStorageModuleReconciler) PreChecks(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) error {
	// Check drivers
	switch cr.Spec.Driver.CSIDriverType {
	case csmv1.PowerScale:
		err := drivers.PrecheckPowerScale(ctx, cr, operatorConfig, r.GetClient())
		if err != nil {
			return fmt.Errorf("failed powerscale validation: %v", err)
		}

	default:
		return fmt.Errorf("unsupported driver type %s", cr.Spec.Driver.CSIDriverType)
	}

	upgradeValid, err := checkUpgrade(ctx, cr, operatorConfig)
	if err != nil {
		return fmt.Errorf("failed upgrade check: %v", err)
	} else if !upgradeValid {
		return fmt.Errorf("failed upgrade check because upgrade is not valid")
	}

	// check modules
	for _, m := range cr.Spec.Modules {
		if m.Enabled {
			switch m.Name {
			case csmv1.Authorization:
				err := modules.AuthorizationPrecheck(ctx, operatorConfig, m, *cr, r.GetClient())
				if err != nil {
					return fmt.Errorf("failed authorization validation: %v", err)
				}

			case csmv1.Replication:
				err := modules.ReplicationPrecheck(ctx, operatorConfig, m, *cr, r)
				if err != nil {
					return fmt.Errorf("failed replication validation: %v", err)
				}

			default:
				return fmt.Errorf("unsupported module type %s", m.Name)

			}

		}
	}
	return nil
}

// Check for upgrade/if upgrade is appropriate
func checkUpgrade(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) (bool, error) {
	log := logger.GetLogger(ctx)
	driverType := cr.Spec.Driver.CSIDriverType
	if driverType == csmv1.PowerScale {
		// use powerscale instead of isilon as the folder name is powerscale
		driverType = csmv1.PowerScaleName
	}
	// If it is an upgrade/downgrade, check to see if we meet the minimum version using GetUpgradeInfo, which returns the minimum version required
	// for the desired upgrade. If the upgrade path is not valid fail
	// Existing version
	annotations := cr.GetAnnotations()
	oldVersion, configVersionExists := annotations[configVersionKey]

	// If annotation exists, we are doing an upgrade or modify
	if configVersionExists {
		// if versions are equal, it is a modify
		if oldVersion == cr.Spec.Driver.ConfigVersion {
			log.Infow("proceeding with modification of driver install")
			return true, nil
			//if not equal, it is an upgrade/downgrade
		} else {
			// get minimum required version for upgrade
			minUpgradePath, err := drivers.GetUpgradeInfo(ctx, operatorConfig, driverType, oldVersion)
			if err != nil {
				log.Infow("GetUpgradeInfo not successful")
				return false, err
			}
			//
			installValid, err := utils.MinVersionCheck(minUpgradePath, cr.Spec.Driver.ConfigVersion)
			if err != nil {
				return false, err
			} else if installValid {
				log.Infow("proceeding with valid driver upgrade from version %s to version %s", oldVersion, cr.Spec.Driver.ConfigVersion)
				return installValid, nil
			} else {
				log.Infow("not proceeding with invalid driver upgrade")
				return installValid, fmt.Errorf("failed upgrade check: upgrade from version %s to %s not valid", oldVersion, cr.Spec.Driver.ConfigVersion)
			}
		}
	} else {
		log.Infow("proceeding with fresh driver install")
		return true, nil
	}
}

// TODO: refactor this
func applyConfigVersionAnnotations(ctx context.Context, instance *csmv1.ContainerStorageModule) bool {

	log := logger.GetLogger(ctx)

	// If driver has not been initialized yet, we first annotate the driver with the config version annotation

	annotations := instance.GetAnnotations()
	isUpdated := false
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if _, ok := annotations[configVersionKey]; !ok {
		annotations[configVersionKey] = instance.Spec.Driver.ConfigVersion
		isUpdated = true
		instance.SetAnnotations(annotations)
		log.Infof("Installing CSI Driver %s with config Version %s. Updating Annotations with Config Version",
			instance.GetName(), instance.Spec.Driver.ConfigVersion)
	}
	return isUpdated
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

// GetK8sClient - Returns the current update count
func (r *ContainerStorageModuleReconciler) GetK8sClient() kubernetes.Interface {
	return r.K8sClient
}
