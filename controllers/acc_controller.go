//  Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package controllers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/resources/rbac"
	"github.com/dell/csm-operator/pkg/resources/serviceaccount"
	"github.com/dell/csm-operator/pkg/resources/statefulset"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/constants"
	"github.com/dell/csm-operator/pkg/logger"

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
)

const (
	// AccManifest - deployment resources for Apex Connectivity Client
	AccManifest string = "statefulset.yaml"

	// BrownfieldManifest - manifest for brownfield role/rolebinding creation
	BrownfieldManifest string = "brownfield-onboard.yaml"
)

// ApexConnectivityClientReconciler reconciles a ApexConnectivityClient object
type ApexConnectivityClientReconciler struct {
	// controller runtime client, responsible for create, delete, update, get etc.
	crclient.Client
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

// AccConfig - Apex Connectivity Client Config
type AccConfig struct {
	Controller *utils.StatefulControllerYAML
}

const (
	// AccMetadataPrefix - prefix for all labels & annotations
	AccMetadataPrefix = "storage.dell.com"

	// AccFinalizerName - the name of the finalizer
	AccFinalizerName = "finalizer.dell.com"

	// ApexName AccClient name
	ApexName = "ApexConnectivityClient"

	// AccVersion - the version of the connectivity client
	AccVersion = "v1.0.0"
)

var (
	accdMutex           sync.RWMutex
	accConfigVersionKey = fmt.Sprintf("%s/%s", AccMetadataPrefix, "ApexConnectivityClientConfigVersion")

	// AccVersionKey - the version of the connectivity client
	AccVersionKey = fmt.Sprintf("%s/%s", AccMetadataPrefix, "AccVersion")

	// AccStopWatch - watcher stop handle
	AccStopWatch = make(chan struct{})
)

//+kubebuilder:rbac:groups=storage.dell.com,resources=apexconnectivityclients,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storage.dell.com,resources=apexconnectivityclients/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storage.dell.com,resources=apexconnectivityclients/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=mobility.storage.dell.com,resources=backups,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ApexConnectivityClient object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *ApexConnectivityClientReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.IncrUpdateCount()
	r.trcID = fmt.Sprintf("%d", r.GetUpdateCount())
	name := req.Name + "-" + r.trcID
	ctx, log := logger.GetNewContextWithLogger(name)
	log.Info("################Starting Apex Connectivity Client Reconcile##############")
	acc := new(csmv1.ApexConnectivityClient)

	log.Infow("reconcile for", "Namespace", req.Namespace, "Name", req.Name, "Attempt", r.GetUpdateCount())

	// Fetch the ApexConnectivityClientReconciler instance
	err := r.Client.Get(ctx, req.NamespacedName, acc)
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

	op := &utils.OperatorConfig{
		IsOpenShift:     r.Config.IsOpenShift,
		K8sVersion:      r.Config.K8sVersion,
		ConfigDirectory: r.Config.ConfigDirectory,
	}
	crc := r.GetClient()

	// perform prechecks
	err = r.PreChecks(ctx, acc, *op)
	if err != nil {
		acc.GetApexConnectivityClientStatus().State = constants.InvalidConfig
		r.EventRecorder.Event(acc, corev1.EventTypeWarning, csmv1.EventUpdated, fmt.Sprintf("Failed Prechecks: %s", err))
		return utils.HandleAccValidationError(ctx, acc, r, err)
	}

	if acc.IsBeingDeleted() {
		log.Infow("Delete request", "acc", req.Namespace, "Name", req.Name)

		// check for force cleanup
		if acc.Spec.Client.ForceRemoveClient {
			// remove all resources deployed from CR by operator
			if err = DeployApexConnectivityClient(ctx, true, *op, *acc, crc); err != nil {
				r.EventRecorder.Event(acc, corev1.EventTypeWarning, csmv1.EventDeleted, fmt.Sprintf("Failed to remove client: %s", err))
				log.Errorw("remove client", "error", err.Error())
				return ctrl.Result{}, fmt.Errorf("error when deleting client: %v", err)
			}
		}

		if err = r.removeFinalizer(ctx, acc); err != nil {
			r.EventRecorder.Event(acc, corev1.EventTypeWarning, csmv1.EventDeleted, fmt.Sprintf("Failed to delete finalizer: %s", err))
			log.Errorw("Remove Apex Connectivity Client finalizer", "error", err.Error())
			return ctrl.Result{}, fmt.Errorf("error when handling finalizer: %v", err)
		}
		r.EventRecorder.Event(acc, corev1.EventTypeNormal, csmv1.EventDeleted, "Object finalizer is deleted")

		return ctrl.Result{}, nil
	}

	// Add finalizer
	if !acc.HasFinalizer(AccFinalizerName) {
		log.Infow("HandleFinalizer", "name", AccFinalizerName)
		if err = r.addFinalizer(ctx, acc); err != nil {
			r.EventRecorder.Event(acc, corev1.EventTypeWarning, csmv1.EventUpdated, fmt.Sprintf("Failed to add finalizer: %s", err))
			log.Errorw("HandleFinalizer", "error", err.Error())
			return ctrl.Result{}, fmt.Errorf("error when adding finalizer: %v", err)
		}
		r.EventRecorder.Event(acc, corev1.EventTypeNormal, csmv1.EventUpdated, "Object finalizer is added")
	}

	oldStatus := acc.GetApexConnectivityClientStatus()

	// Set the driver annotation
	isUpdated := applyAccConfigVersionAnnotations(ctx, acc)
	if isUpdated {
		err = r.GetClient().Update(ctx, acc)
		if err != nil {
			log.Error(err, "Failed to update CR with annotation")
			return reconcile.Result{}, err
		}
	}

	newStatus := acc.GetApexConnectivityClientStatus()
	requeue, err := utils.HandleAccSuccess(ctx, acc, r, newStatus, oldStatus)
	if err != nil {
		log.Error(err, "Failed to update CR status")
	}

	if err = DeployApexConnectivityClient(ctx, false, *op, *acc, crc); err == nil {
		syncErr := r.SyncACC(ctx, *acc, *op)
		if syncErr == nil && !requeue.Requeue {
			if err = utils.UpdateAccStatus(ctx, acc, r, newStatus); err != nil {
				log.Error(err, "Failed to update CR status")
				return utils.LogBannerAndReturn(reconcile.Result{Requeue: true}, err)
			}
			r.EventRecorder.Eventf(acc, corev1.EventTypeNormal, csmv1.EventCompleted, "install/update storage component: %s completed OK", acc.Name)
			return utils.LogBannerAndReturn(reconcile.Result{}, nil)
		}

		// syncErr can be nil, even if ACC state = failed
		if syncErr == nil {
			syncErr = errors.New("ACC state is failed")
		}
	}
	// Failed deployment
	r.EventRecorder.Eventf(acc, corev1.EventTypeWarning, csmv1.EventUpdated, "Failed install: %s", err.Error())

	return utils.LogBannerAndReturn(reconcile.Result{Requeue: true}, err)
}

func (r *ApexConnectivityClientReconciler) ignoreUpdatePredicate() predicate.Predicate {
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

// StatefulSetForApexConnectivityClient returns a apexConnectivityClient StatefulSet object
func (r *ApexConnectivityClientReconciler) handleStatefulSetUpdate(oldObj interface{}, obj interface{}) {
	accdMutex.Lock()
	defer accdMutex.Unlock()

	old, _ := oldObj.(*appsv1.StatefulSet)
	d, _ := obj.(*appsv1.StatefulSet)
	name := d.Spec.Template.Labels[constants.AccLabel]
	key := name + "-" + fmt.Sprintf("%d", r.GetUpdateCount())
	ctx, log := logger.GetNewContextWithLogger(key)
	if name == "" {
		return
	}

	log.Debugw("statefulSet modified generation", d.Generation, old.Generation)

	desired := d.Status.Replicas
	available := d.Status.AvailableReplicas
	ready := d.Status.ReadyReplicas

	log.Infow("statefulSet", "desired", desired)
	log.Infow("statefulSet", "numberReady", ready)
	log.Infow("statefulSet", "available", available)

	ns := d.Namespace
	log.Debugw("statefulSet", "namespace", ns, "name", name)
	namespacedName := t1.NamespacedName{
		Name:      name,
		Namespace: ns,
	}

	acc := new(csmv1.ApexConnectivityClient)
	err := r.Client.Get(ctx, namespacedName, acc)
	if err != nil {
		log.Error("statefulSet get acc", "error", err.Error())
	}

	newStatus := acc.GetApexConnectivityClientStatus()
	newStatus.ClientStatus.Available = strconv.Itoa(int(available))
	newStatus.ClientStatus.Desired = strconv.Itoa(int(desired))

	err = utils.UpdateAccStatus(ctx, acc, r, newStatus)
	if err != nil {
		log.Debugw("statefulSet status ", "pods", err.Error())
	} else {
		r.EventRecorder.Eventf(acc, corev1.EventTypeNormal, csmv1.EventCompleted, "Apex Connectivity Client running OK")
	}
}

func (r *ApexConnectivityClientReconciler) handlePodsUpdate(_ interface{}, obj interface{}) {
	accdMutex.Lock()
	defer accdMutex.Unlock()

	p, _ := obj.(*corev1.Pod)
	name := p.GetLabels()[constants.AccLabel]
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
	acc := new(csmv1.ApexConnectivityClient)
	err := r.Client.Get(ctx, namespacedName, acc)
	if err != nil {
		r.Log.Errorw("statefulset get acc", "error", err.Error())
	}
	log.Infow("acc prev status ", "state", acc.Status)
	newStatus := acc.GetApexConnectivityClientStatus()

	err = utils.UpdateAccStatus(ctx, acc, r, newStatus)
	state := acc.GetApexConnectivityClientStatus().State
	stamp := fmt.Sprintf("at %d", time.Now().UnixNano())
	if state != "0" && err != nil {
		log.Infow("pod status ", "state", err.Error())
		r.EventRecorder.Eventf(acc, corev1.EventTypeWarning, csmv1.EventUpdated, "%s Pod error details %s", stamp, err.Error())
	} else {
		r.EventRecorder.Eventf(acc, corev1.EventTypeNormal, csmv1.EventCompleted, "%s Apex Connectivity Client pods running OK", stamp)
	}
}

// ClientContentWatch - watch updates on deployment and statefulset
func (r *ApexConnectivityClientReconciler) ClientContentWatch() error {
	sharedInformerFactory := sinformer.NewSharedInformerFactory(r.K8sClient, time.Duration(time.Hour))

	statefulSetInformer := sharedInformerFactory.Apps().V1().StatefulSets().Informer()
	_, err := statefulSetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: r.handleStatefulSetUpdate,
	})
	if err != nil {
		return fmt.Errorf("ClientContentWatch failed adding event handler to statefulsetInformer: %v", err)
	}

	podsInformer := sharedInformerFactory.Core().V1().Pods().Informer()
	_, err = podsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: r.handlePodsUpdate,
	})
	if err != nil {
		return fmt.Errorf("ClientContentWatch failed adding event handler to podsInformer: %v", err)
	}

	sharedInformerFactory.Start(AccStopWatch)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApexConnectivityClientReconciler) SetupWithManager(mgr ctrl.Manager, limiter ratelimiter.RateLimiter, maxReconcilers int) error {
	go r.ClientContentWatch()
	return ctrl.NewControllerManagedBy(mgr).
		For(&csmv1.ApexConnectivityClient{}).
		WithEventFilter(r.ignoreUpdatePredicate()).
		WithOptions(controller.Options{
			RateLimiter:             limiter,
			MaxConcurrentReconciles: maxReconcilers,
		}).Complete(r)
}

func (r *ApexConnectivityClientReconciler) removeFinalizer(ctx context.Context, instance *csmv1.ApexConnectivityClient) error {
	if !instance.HasFinalizer(AccFinalizerName) {
		return nil
	}
	instance.SetFinalizers(nil)
	return r.Update(ctx, instance)
}

func (r *ApexConnectivityClientReconciler) addFinalizer(ctx context.Context, instance *csmv1.ApexConnectivityClient) error {
	instance.SetFinalizers([]string{AccFinalizerName})
	instance.GetApexConnectivityClientStatus().State = constants.Creating
	return r.Update(ctx, instance)
}

// PreChecks - validate input values
func (r *ApexConnectivityClientReconciler) PreChecks(ctx context.Context, cr *csmv1.ApexConnectivityClient, operatorConfig utils.OperatorConfig) error {
	log := logger.GetLogger(ctx)

	// Check if driver version is supported by doing a stat on a config file
	configFilePath := fmt.Sprintf("%s/clientconfig/%s/%s/upgrade-path.yaml", operatorConfig.ConfigDirectory, csmv1.DreadnoughtClient, cr.Spec.Client.ConfigVersion)
	log.Info(configFilePath)
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		return fmt.Errorf("%s %s not supported", csmv1.DreadnoughtClient, cr.Spec.Client.ConfigVersion)
	}
	return nil
}

func applyAccConfigVersionAnnotations(ctx context.Context, instance *csmv1.ApexConnectivityClient) bool {
	log := logger.GetLogger(ctx)

	// If client has not been initialized yet, we first annotate the client with the config version annotation
	annotations := instance.GetAnnotations()
	isUpdated := false
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[AccVersionKey] = AccVersion
	instance.SetAnnotations(annotations)

	if _, ok := annotations[accConfigVersionKey]; !ok {
		annotations[accConfigVersionKey] = instance.Spec.Client.ConfigVersion
		isUpdated = true
		instance.SetAnnotations(annotations)
		log.Infof("Installing storage component %s with config Version %s. Updating Annotations with Config Version",
			instance.GetName(), instance.Spec.Client.ConfigVersion)
	}
	return isUpdated
}

// DeployApexConnectivityClient - perform deployment
func DeployApexConnectivityClient(ctx context.Context, isDeleting bool, operatorConfig utils.OperatorConfig, cr csmv1.ApexConnectivityClient, ctrlClient crclient.Client) error {
	log := logger.GetLogger(ctx)

	YamlString := ""
	ModifiedYamlString := ""
	deploymentPath := fmt.Sprintf("%s/clientconfig/%s/%s/%s", operatorConfig.ConfigDirectory, csmv1.DreadnoughtClient, cr.Spec.Client.ConfigVersion, AccManifest)
	buf, err := os.ReadFile(filepath.Clean(deploymentPath))
	if err != nil {
		return err
	}

	YamlString = utils.ModifyCommonCRs(string(buf), cr)
	ModifiedYamlString = drivers.ModifyApexConnectivityClientCR(YamlString, cr)
	deployObjects, err := utils.GetModuleComponentObj([]byte(ModifiedYamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range deployObjects {
		if isDeleting {
			if err := utils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	if err = utils.CreateBrownfieldRbac(ctx, operatorConfig, cr, ctrlClient, isDeleting); err != nil {
		log.Error(err, "error creating role/rolebindings")
	}

	return nil
}

func getAccConfig(ctx context.Context, cr csmv1.ApexConnectivityClient, operatorConfig utils.OperatorConfig) (*AccConfig, error) {
	var (
		err        error
		controller *utils.StatefulControllerYAML
		log        = logger.GetLogger(ctx)
	)

	// if no ACC client is specified, return nil
	if cr.Spec.Client.CSMClientType == "" {
		log.Infof("No CSMClientType specified in manifest")
		return nil, nil
	}

	controller, err = drivers.GetAccController(ctx, cr, operatorConfig, cr.Spec.Client.CSMClientType)
	if err != nil {
		return nil, fmt.Errorf("getting Apex connectivity client controller: %v", err)
	}

	return &AccConfig{
		Controller: controller,
	}, nil
}

// SyncACC - sync apex connectivity client
func (r *ApexConnectivityClientReconciler) SyncACC(ctx context.Context, cr csmv1.ApexConnectivityClient, operatorConfig utils.OperatorConfig) error {
	log := logger.GetLogger(ctx)

	// get acc resource
	accconfig, err := getAccConfig(ctx, cr, operatorConfig)
	if err != nil {
		return err
	}

	if accconfig == nil {
		return nil
	}

	controller := accconfig.Controller

	_, clusterClients, err := utils.GetAccDefaultClusters(ctx, cr, r)
	if err != nil {
		return err
	}

	for _, cluster := range clusterClients {
		log.Infof("Starting SYNC for %s cluster", cluster.ClusterID)

		if err = serviceaccount.SyncServiceAccount(ctx, controller.Rbac.ServiceAccount, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		if err = rbac.SyncClusterRole(ctx, controller.Rbac.ClusterRole, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		// create update ClusterRoleBinding
		if err = rbac.SyncClusterRoleBindings(ctx, controller.Rbac.ClusterRoleBinding, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		// sync StatefulSet
		if err = statefulset.SyncStatefulSet(ctx, controller.StatefulSet, cluster.ClusterK8sClient, ApexName); err != nil {
			return err
		}
	}
	return nil
}

// GetClient - returns the split client
func (r *ApexConnectivityClientReconciler) GetClient() crclient.Client {
	return r.Client
}

// IncrUpdateCount - Increments the update count
func (r *ApexConnectivityClientReconciler) IncrUpdateCount() {
	atomic.AddInt32(&r.updateCount, 1)
}

// GetUpdateCount - Returns the current update count
func (r *ApexConnectivityClientReconciler) GetUpdateCount() int32 {
	return r.updateCount
}

// GetK8sClient - Returns the current update count
func (r *ApexConnectivityClientReconciler) GetK8sClient() kubernetes.Interface {
	return r.K8sClient
}
