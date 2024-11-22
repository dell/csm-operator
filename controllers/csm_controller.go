//  Copyright © 2021 - 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/modules"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csmv1 "github.com/dell/csm-operator/api/v1"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// CSMVersion -
	CSMVersion = "v1.12.0"
)

var (
	dMutex                          sync.RWMutex
	configVersionKey                = fmt.Sprintf("%s/%s", MetadataPrefix, "CSMOperatorConfigVersion")
	previouslyAppliedCustomResource = fmt.Sprintf("%s/%s", MetadataPrefix, "PreviouslyAppliedConfiguration")

	// CSMVersionKey -
	CSMVersionKey = fmt.Sprintf("%s/%s", MetadataPrefix, "CSMVersion")

	// StopWatch - watcher stop handle
	StopWatch = make(chan struct{})
)

// +kubebuilder:rbac:groups=storage.dell.com,resources=containerstoragemodules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.dell.com,resources=containerstoragemodules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=storage.dell.com,resources=containerstoragemodules/finalizers,verbs=update
// +kubebuilder:rbac:groups="replication.storage.dell.com",resources=dellcsireplicationgroups,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="replication.storage.dell.com",resources=dellcsireplicationgroups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods;services;services/finalizers;endpoints;persistentvolumeclaims;events;configmaps;secrets;serviceaccounts;roles;ingresses,verbs=*
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;create;patch;update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=update;patch;get
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=create;get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;create;delete;patch;update
// +kubebuilder:rbac:groups="apps",resources=deployments;daemonsets;replicasets;statefulsets,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles;clusterrolebindings;replicasets;rolebindings,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles/finalizers,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=subjectaccessreviews,verbs=create
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="*",resources=*,resourceNames=application-mobility-velero-server,verbs=*
// +kubebuilder:rbac:groups="monitoring.coreos.com",resources=servicemonitors,verbs=get;create
// +kubebuilder:rbac:groups="",resources=deployments/finalizers,resourceNames=dell-csm-operator-controller-manager,verbs=update
// +kubebuilder:rbac:groups="storage.k8s.io",resources=csidrivers,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="storage.k8s.io",resources=storageclasses,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="storage.k8s.io",resources=volumeattachments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="storage.k8s.io",resources=csinodes,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="csi.storage.k8s.io",resources=csinodeinfos,verbs=get;list;watch
// +kubebuilder:rbac:groups="snapshot.storage.k8s.io",resources=volumesnapshotclasses;volumesnapshotcontents,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="snapshot.storage.k8s.io",resources=volumesnapshotcontents/status,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups="snapshot.storage.k8s.io",resources=volumesnapshots,verbs=get;list;watch;update;patch;create;delete
// +kubebuilder:rbac:groups="snapshot.storage.k8s.io",resources=volumesnapshots/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="volumegroup.storage.dell.com",resources=dellcsivolumegroupsnapshots;dellcsivolumegroupsnapshots/status,verbs=create;list;watch;delete;update;get;patch
// +kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions,verbs=*
// +kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions/status,verbs=get;list;patch;watch
// +kubebuilder:rbac:groups="storage.k8s.io",resources=volumeattachments/status,verbs=patch
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="security.openshift.io",resources=securitycontextconstraints,resourceNames=privileged,verbs=use
// +kubebuilder:rbac:urls="/metrics",verbs=get
// +kubebuilder:rbac:groups="authentication.k8s.io",resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups="authorization.k8s.io",resources=subjectaccessreviews,verbs=create
// +kubebuilder:rbac:groups="cert-manager.io",resources=issuers;issuers/status,verbs=update;get;list;watch;patch
// +kubebuilder:rbac:groups="cert-manager.io",resources=clusterissuers;clusterissuers/status,verbs=update;get;list;watch;patch
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates;certificaterequests;clusterissuers;issuers,verbs=*
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates/finalizers;certificaterequests/finalizers,verbs=update
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates/status;certificaterequests/status,verbs=update;patch
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates;certificaterequests;issuers,verbs=create;delete;deletecollection;patch;update
// +kubebuilder:rbac:groups="cert-manager.io",resources=signers,resourceNames=issuers.cert-manager.io/*;clusterissuers.cert-manager.io/*,verbs=approve
// +kubebuilder:rbac:groups="cert-manager.io",resources=*/*,verbs=*
// +kubebuilder:rbac:groups="",resources=secrets,resourceNames=cert-manager-webhook-ca,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="cert-manager.io",resources=configmaps,resourceNames=cert-manager-cainjector-leader-election;cert-manager-cainjector-leader-election-core,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,resourceNames=cert-manager-controller,verbs=get;update;patch
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=backups,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=backups/finalizers,verbs=update
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=backups/status,verbs=get;patch;update
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=clusterconfigs,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=clusterconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=clusterconfigs/status,verbs=get;patch;update
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=podvolumebackups,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=podvolumebackups/finalizers,verbs=update
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=podvolumebackups/status,verbs=get;patch;update
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=podvolumerestores,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=podvolumerestores/finalizers,verbs=update
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=podvolumerestores/status,verbs=get;patch;update
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=restores,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=restores/finalizers,verbs=update
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=restores/status,verbs=get;patch;update
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=schedules,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="mobility.storage.dell.com",resources=schedules/status,verbs=get;patch;update
// +kubebuilder:rbac:groups="velero.io",resources=backups,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="velero.io",resources=backups/finalizers,verbs=update
// +kubebuilder:rbac:groups="velero.io",resources=backups/status,verbs=get;list;patch;update
// +kubebuilder:rbac:groups="velero.io",resources=backupstoragelocations,verbs=get;list;patch;update;watch
// +kubebuilder:rbac:groups="velero.io",resources=deletebackuprequests,verbs=create;delete;get;list;watch
// +kubebuilder:rbac:groups="velero.io",resources=podvolumebackups,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="velero.io",resources=podvolumebackups/finalizers,verbs=update
// +kubebuilder:rbac:groups="velero.io",resources=podvolumebackups/status,verbs=create;get;list;patch;update
// +kubebuilder:rbac:groups="velero.io",resources=podvolumerestores,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="velero.io",resources=backuprepositories,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="velero.io",resources=restores,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,resourceNames=cert-manager-cainjector-leader-election;cert-manager-cainjector-leader-election-core,verbs=get;update;patch
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,resourceNames=cert-manager-controller,verbs=get;update;patch
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs=create;patch
// +kubebuilder:rbac:groups="acme.cert-manager.io",resources=orders,verbs=create;delete;get;list;watch
// +kubebuilder:rbac:groups="acme.cert-manager.io",resources=orders;orders/status,verbs=update;patch
// +kubebuilder:rbac:groups="acme.cert-manager.io",resources=orders;challenges,verbs=get;list;watch;create;delete;deletecollection;patch;update
// +kubebuilder:rbac:groups="acme.cert-manager.io",resources=clusterissuers;issuers,verbs=get;list;watch
// +kubebuilder:rbac:groups="acme.cert-manager.io",resources=challenges,verbs=create;delete
// +kubebuilder:rbac:groups="acme.cert-manager.io",resources=orders/finalizers,verbs=update
// +kubebuilder:rbac:groups="acme.cert-manager.io",resources=challenges;challenges/status,verbs=update;get;list;watch;patch
// +kubebuilder:rbac:groups="acme.cert-manager.io",resources=challenges/finalizers,verbs=update
// +kubebuilder:rbac:groups="acme.cert-manager.io",resources=*/*,verbs=*
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses,verbs=*
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses/finalizers,verbs=update
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingressclasses,verbs=create;get;list;watch;update;delete
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses/status,verbs=update;get;list;watch
// +kubebuilder:rbac:groups="gateway.networking.k8s.io",resources=httproutes,verbs=get;list;watch;create;delete;update
// +kubebuilder:rbac:groups="gateway.networking.k8s.io",resources=httproutes;gateways,verbs=get;list;watch
// +kubebuilder:rbac:groups="gateway.networking.k8s.io",resources=gateways/finalizers;httproutes/finalizers,verbs=update
// +kubebuilder:rbac:groups="route.openshift.io",resources=routes/custom-host,verbs=create
// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=validatingwebhookconfigurations;mutatingwebhookconfigurations,verbs=create;get;list;watch;update;delete;patch
// +kubebuilder:rbac:groups="apiregistration.k8s.io",resources=apiservices,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="apiregistration.k8s.io",resources=customresourcedefinitions,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="auditregistration.k8s.io",resources=auditsinks,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="",resources=configmaps,resourceNames=ingress-controller-leader,verbs=get;update
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,resourceNames=ingress-controller-leader,verbs=get;update;patch
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs=create;list;watch;patch
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,resourceNames=cert-manager-cainjector-leader-election;cert-manager-cainjector-leader-election-core,verbs=get;update;patch
// +kubebuilder:rbac:groups="discovery.k8s.io",resources=endpointslices,verbs=list;watch;get
// +kubebuilder:rbac:groups="certificates.k8s.io",resources=certificatesigningrequests,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="certificates.k8s.io",resources=certificatesigningrequests/status,verbs=update;patch
// +kubebuilder:rbac:groups="certificates.k8s.io",resources=signers,resourceNames=issuers.cert-manager.io/*;clusterissuers.cert-manager.io/*,verbs=sign
// +kubebuilder:rbac:groups="",resources=configmaps,resourceNames=cert-manager-cainjector-leader-election;cert-manager-cainjector-leader-election-core;cert-manager-controller,verbs=get;update;patch
// +kubebuilder:rbac:groups="batch",resources=jobs,verbs=list;watch;create;update;delete
// +kubebuilder:rbac:groups="storage.k8s.io",resources=csistoragecapacities,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="csm-authorization.storage.dell.com",resources=storages;csmtenants;csmroles,verbs=get;list
// +kubebuilder:rbac:groups="csm-authorization.storage.dell.com",resources=csmroles,verbs=watch;create;update;patch;delete
// +kubebuilder:rbac:groups="csm-authorization.storage.dell.com",resources=csmroles/finalizers,verbs=update
// +kubebuilder:rbac:groups="csm-authorization.storage.dell.com",resources=csmroles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="csm-authorization.storage.dell.com",resources=csmtenants,verbs=watch;create;update;patch;delete
// +kubebuilder:rbac:groups="csm-authorization.storage.dell.com",resources=csmtenants/finalizers,verbs=update
// +kubebuilder:rbac:groups="csm-authorization.storage.dell.com",resources=csmtenants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="csm-authorization.storage.dell.com",resources=storages,verbs=watch;create;update;patch;delete
// +kubebuilder:rbac:groups="csm-authorization.storage.dell.com",resources=storages/finalizers,verbs=update
// +kubebuilder:rbac:groups="csm-authorization.storage.dell.com",resources=storages/status,verbs=get;update;patch

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
func (r *ContainerStorageModuleReconciler) Reconcile(_ context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.IncrUpdateCount()
	r.trcID = fmt.Sprintf("%d", r.GetUpdateCount())
	name := req.Name + "-" + r.trcID
	ctx, log := logger.GetNewContextWithLogger(name)
	unitTestRun := utils.DetermineUnitTestRun(ctx)

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

	// Set default components if using miminal manifest (without components)
	err = utils.LoadDefaultComponents(ctx, csm, *operatorConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	// perform prechecks
	err = r.PreChecks(ctx, csm, *operatorConfig)
	if err != nil {
		csm.GetCSMStatus().State = constants.InvalidConfig
		r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventUpdated, fmt.Sprintf("Failed Prechecks: %s", err))
		return utils.HandleValidationError(ctx, csm, r, err)
	}

	// Set default replica count if not specified
	if csm.Spec.Driver.Replicas == 0 {
		csm.Spec.Driver.Replicas = 2
	}

	if csm.IsBeingDeleted() {
		log.Infow("Delete request", "csm", req.Namespace, "Name", req.Name)

		// check for force cleanup
		if csm.Spec.Driver.ForceRemoveDriver {
			// remove all resources deployed from CR by operator
			if err := r.removeDriver(ctx, *csm, *operatorConfig); err != nil {
				r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventDeleted, fmt.Sprintf("Failed to remove driver: %s", err))
				log.Errorw("remove driver", "error", err.Error())
				return ctrl.Result{}, fmt.Errorf("error when deleting driver: %v", err)
			}
		}

		// check for force cleanup on standalone module
		for _, m := range csm.Spec.Modules {
			if m.ForceRemoveModule {
				// remove all resources deployed from CR by operator
				if err := r.removeModule(ctx, *csm, *operatorConfig, r.Client); err != nil {
					r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventDeleted, fmt.Sprintf("Failed to remove module: %s", err))
					return ctrl.Result{}, fmt.Errorf("error when deleting module: %v", err)
				}
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
	requeue := utils.HandleSuccess(ctx, csm, r, newStatus, oldStatus)

	// Update the driver
	syncErr := r.SyncCSM(ctx, *csm, *operatorConfig, r.Client)
	if syncErr == nil && !requeue.Requeue {
		err = utils.UpdateStatus(ctx, csm, r, newStatus)
		if err != nil && !unitTestRun {
			log.Error(err, "Failed to update CR status")
			utils.LogEndReconcile()
			return reconcile.Result{Requeue: true}, err
		}
		r.EventRecorder.Eventf(csm, corev1.EventTypeNormal, csmv1.EventCompleted, "install/update storage component: %s completed OK", csm.Name)
		utils.LogEndReconcile()
		return reconcile.Result{}, nil
	}

	// syncErr can be nil, even if CSM state = failed
	if syncErr == nil {
		syncErr = errors.New("CSM state is failed")
	}

	// Failed deployment
	r.EventRecorder.Eventf(csm, corev1.EventTypeWarning, csmv1.EventUpdated, "Failed install: %s", syncErr.Error())

	utils.LogEndReconcile()
	return reconcile.Result{Requeue: true}, syncErr
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

	log.Debugw("deployment modified generation", d.Name, d.Generation, old.Generation)

	desired := d.Status.Replicas
	available := d.Status.AvailableReplicas
	ready := d.Status.ReadyReplicas
	numberUnavailable := d.Status.UnavailableReplicas

	// Replicas:               2 desired | 2 updated | 2 total | 2 available | 0 unavailable

	log.Infow("deployment", "deployment name", d.Name, "desired", desired)
	log.Infow("deployment", "deployment name", d.Name, "numberReady", ready)
	log.Infow("deployment", "deployment name", d.Name, "available", available)
	log.Infow("deployment", "deployment name", d.Name, "numberUnavailable", numberUnavailable)

	ns := d.Spec.Template.Labels[constants.CsmNamespaceLabel]
	if ns == "" {
		ns = d.Namespace
	}
	log.Debugw("csm being modified in handledeployment", "namespace", ns, "name", name)
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

	// Updating controller status manually as controller runtime API is not updating csm object with latest data
	// TODO: Can remove this once the controller runtime repo has a fix for updating the object passed
	newStatus.ControllerStatus.Available = strconv.Itoa(int(available))
	newStatus.ControllerStatus.Desired = strconv.Itoa(int(desired))
	newStatus.ControllerStatus.Failed = strconv.Itoa(int(numberUnavailable))

	err = utils.UpdateStatus(ctx, csm, r, newStatus)
	if err != nil {
		log.Debugw("deployment status ", "pods", err.Error())
	} else {
		r.EventRecorder.Eventf(csm, corev1.EventTypeNormal, csmv1.EventCompleted, "Driver deployment running OK")
	}
}

func (r *ContainerStorageModuleReconciler) handlePodsUpdate(_ interface{}, obj interface{}) {
	dMutex.Lock()
	defer dMutex.Unlock()

	p, _ := obj.(*corev1.Pod)
	name := p.GetLabels()[constants.CsmLabel]
	// if this pod is an obs. pod, namespace might not match csm namespace
	ns := p.GetLabels()[constants.CsmNamespaceLabel]
	if ns == "" {
		ns = p.Namespace
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

	ns := d.Spec.Template.Labels[constants.CsmNamespaceLabel]
	if ns == "" {
		ns = d.Namespace
	}
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
	_, err := daemonsetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: r.handleDaemonsetUpdate,
	})
	if err != nil {
		return fmt.Errorf("ContentWatch failed adding event handler to daemonsetInformer: %v", err)
	}

	deploymentInformer := sharedInformerFactory.Apps().V1().Deployments().Informer()
	_, err = deploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: r.handleDeploymentUpdate,
	})
	if err != nil {
		return fmt.Errorf("ContentWatch failed adding event handler to deploymentInformer: %v", err)
	}

	podsInformer := sharedInformerFactory.Core().V1().Pods().Informer()
	_, err = podsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: r.handlePodsUpdate,
	})
	if err != nil {
		return fmt.Errorf("ContentWatch failed adding event handler to podsInformer: %v", err)
	}

	sharedInformerFactory.Start(StopWatch)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ContainerStorageModuleReconciler) SetupWithManager(mgr ctrl.Manager, limiter ratelimiter.RateLimiter, maxReconcilers int) error {
	go func() {
		err := r.ContentWatch()
		if err != nil {
			fmt.Println("ContentWatch failed", err)
			os.Exit(1)
		}
	}()

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

func (r *ContainerStorageModuleReconciler) oldStandAloneModuleCleanup(ctx context.Context, newCR *csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, driverConfig *DriverConfig) error {
	log := logger.GetLogger(ctx)
	log.Info("Checking if standalone modules need clean up")

	replicaEnabled := func(cr *csmv1.ContainerStorageModule) bool {
		for _, m := range cr.Spec.Modules {
			if m.Name == csmv1.Replication {
				return m.Enabled
			}
		}
		return false
	}

	var err error

	if oldCrJSON, ok := newCR.Annotations[previouslyAppliedCustomResource]; ok && oldCrJSON != "" {
		oldCR := new(csmv1.ContainerStorageModule)
		err = json.Unmarshal([]byte(oldCrJSON), oldCR)
		if err != nil {
			return fmt.Errorf("error unmarshalling old annotation: %v", err)
		}

		// Check if replica needs to be uninstalled
		if replicaEnabled(oldCR) && !replicaEnabled(newCR) {
			_, clusterClients, err := utils.GetDefaultClusters(ctx, *oldCR, r)
			if err != nil {
				return err
			}
			for _, cluster := range clusterClients {
				log.Infow("Deleting Replication controller", "clusterID:", cluster.ClusterID)
				if err = modules.ReplicationManagerController(ctx, true, operatorConfig, *oldCR, cluster.ClusterCTRLClient); err != nil {
					return err
				}

				// also uninstalled drivers in target clusters
				if cluster.ClusterID != utils.DefaultSourceClusterID {
					if err = removeDriverReplicaCluster(ctx, cluster, driverConfig); err != nil {
						return err
					}
				}
			}
		}
		// check if observability needs to be uninstalled
		oldObservabilityEnabled, oldObs := utils.IsModuleEnabled(ctx, *oldCR, csmv1.Observability)
		newObservabilityEnabled, _ := utils.IsModuleEnabled(ctx, *newCR, csmv1.Observability)
		// check if observability components need to be uninstalled
		components := []string{}
		if oldObservabilityEnabled && newObservabilityEnabled {
			for _, comp := range oldObs.Components {
				oldCompEnabled := utils.IsModuleComponentEnabled(ctx, *oldCR, csmv1.Observability, comp.Name)
				newCompEnabled := utils.IsModuleComponentEnabled(ctx, *newCR, csmv1.Observability, comp.Name)
				if oldCompEnabled && !newCompEnabled {
					components = append(components, comp.Name)
				}
			}
		}
		if (oldObservabilityEnabled && !newObservabilityEnabled) || len(components) > 0 {
			_, clusterClients, err := utils.GetDefaultClusters(ctx, *oldCR, r)
			if err != nil {
				return err
			}
			for _, cluster := range clusterClients {
				// remove module observability
				log.Infow("Deleting observability")
				if err = r.reconcileObservability(ctx, true, operatorConfig, *oldCR, components, cluster.ClusterCTRLClient, cluster.ClusterK8sClient); err != nil {
					return err
				}
			}
		}

		// check if application mobility needs to be uninstalled
		oldApplicationmobilityEnabled, _ := utils.IsModuleEnabled(ctx, *oldCR, csmv1.ApplicationMobility)
		newApplicationmobilityEnabled, _ := utils.IsModuleEnabled(ctx, *newCR, csmv1.ApplicationMobility)

		if oldApplicationmobilityEnabled && !newApplicationmobilityEnabled {
			_, clusterClients, err := utils.GetDefaultClusters(ctx, *oldCR, r)
			if err != nil {
				return err
			}

			for _, cluster := range clusterClients {
				log.Infow("Deleting application mobility")
				if err := r.reconcileAppMobility(ctx, true, operatorConfig, *oldCR, cluster.ClusterCTRLClient); err != nil {
					return err
				}
			}
		}
	}

	copyCR := newCR.DeepCopy()
	delete(copyCR.Annotations, previouslyAppliedCustomResource)
	delete(copyCR.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
	copyCR.ManagedFields = nil
	copyCR.Status = csmv1.ContainerStorageModuleStatus{}
	out, err := json.Marshal(copyCR)
	if err != nil {
		return fmt.Errorf("error marshalling CR to annotation: %v", err)
	}
	newCR.Annotations[previouslyAppliedCustomResource] = string(out)

	return r.GetClient().Update(ctx, newCR)
}

// SyncCSM - Sync the current installation - this can lead to a create or update
func (r *ContainerStorageModuleReconciler) SyncCSM(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	// Create/Update Authorization Proxy Server
	authorizationEnabled, _ := utils.IsModuleEnabled(ctx, cr, csmv1.AuthorizationServer)
	if authorizationEnabled {
		log.Infow("Create/Update authorization")
		if err := r.reconcileAuthorizationCRDS(ctx, operatorConfig, cr, ctrlClient); err != nil {
			return fmt.Errorf("failed to deploy authorization proxy server: %v", err)
		}
		if err := r.reconcileAuthorization(ctx, false, operatorConfig, cr, ctrlClient); err != nil {
			return fmt.Errorf("failed to deploy authorization proxy server: %v", err)
		}
		return nil
	}

	if appmobilityEnabled, _ := utils.IsModuleEnabled(ctx, cr, csmv1.ApplicationMobility); appmobilityEnabled {
		log.Infow("Create/Update application mobility")
		if err := r.reconcileAppMobilityCRDS(ctx, operatorConfig, cr, ctrlClient); err != nil {
			return fmt.Errorf("failed to deploy application mobility: %v", err)
		}
		if err := r.reconcileAppMobility(ctx, false, operatorConfig, cr, ctrlClient); err != nil {
			return fmt.Errorf("failed to deploy application mobility: %v", err)
		}
	}

	// Create/Update Reverseproxy Server
	if reverseProxyEnabled, _ := utils.IsModuleEnabled(ctx, cr, csmv1.ReverseProxy); reverseProxyEnabled {
		log.Infow("Trying Create/Update reverseproxy...")
		if err := r.reconcileReverseProxy(ctx, false, operatorConfig, cr, ctrlClient); err != nil {
			return fmt.Errorf("failed to deploy reverseproxy proxy server: %v", err)
		}
	}

	// Get Driver resources
	driverConfig, err := getDriverConfig(ctx, cr, operatorConfig)
	if err != nil {
		return err
	}

	// driverConfig = nil means no driver specified in manifest
	if driverConfig == nil {
		return nil
	}

	err = r.oldStandAloneModuleCleanup(ctx, &cr, operatorConfig, driverConfig)
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
			case csmv1.Resiliency:
				log.Info("Injecting CSM Resiliency")

				// for controller-pod
				driverName := string(cr.Spec.Driver.CSIDriverType)
				dp, err := modules.ResiliencyInjectDeployment(controller.Deployment, cr, operatorConfig, driverName)
				if err != nil {
					return fmt.Errorf("injecting resiliency into deployment: %v", err)
				}
				controller.Deployment = *dp

				clusterRole, err := modules.ResiliencyInjectClusterRole(controller.Rbac.ClusterRole, cr, operatorConfig, "controller")
				if err != nil {
					return fmt.Errorf("injecting resiliency into controller cluster role: %v", err)
				}

				controller.Rbac.ClusterRole = *clusterRole

				// for node-pod
				ds, err := modules.ResiliencyInjectDaemonset(node.DaemonSetApplyConfig, cr, operatorConfig, driverName)
				if err != nil {
					return fmt.Errorf("injecting resiliency into daemonset: %v", err)
				}
				node.DaemonSetApplyConfig = *ds

				clusterRoleForNode, err := modules.ResiliencyInjectClusterRole(node.Rbac.ClusterRole, cr, operatorConfig, "node")
				if err != nil {
					return fmt.Errorf("injecting resiliency into node cluster role: %v", err)
				}

				node.Rbac.ClusterRole = *clusterRoleForNode
			case csmv1.Replication:
				// This function adds replication sidecar to driver pods.
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
			case csmv1.ReverseProxy:
				log.Info("Injecting CSI ReverseProxy")
				dp, err := modules.ReverseProxyInjectDeployment(controller.Deployment, cr, operatorConfig)
				if err != nil {
					return fmt.Errorf("injecting replication into deployment: %v", err)
				}
				controller.Deployment = *dp
			}
		}
	}

	for _, cluster := range clusterClients {
		log.Infof("Starting SYNC for %s cluster", cluster.ClusterID)
		// Create/Update ServiceAccount
		if err = serviceaccount.SyncServiceAccount(ctx, node.Rbac.ServiceAccount, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		if err = serviceaccount.SyncServiceAccount(ctx, controller.Rbac.ServiceAccount, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		// Create/Update ClusterRoles
		if err = rbac.SyncClusterRole(ctx, node.Rbac.ClusterRole, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		if err = rbac.SyncClusterRole(ctx, controller.Rbac.ClusterRole, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		// Create/Update ClusterRoleBinding
		if err = rbac.SyncClusterRoleBindings(ctx, node.Rbac.ClusterRoleBinding, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		if err = rbac.SyncClusterRoleBindings(ctx, controller.Rbac.ClusterRoleBinding, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		// Create/Update CSIDriver
		if err = csidriver.SyncCSIDriver(ctx, *driver, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		// Create/Update ConfigMap
		if err = configmap.SyncConfigMap(ctx, *configMap, cluster.ClusterCTRLClient); err != nil {
			return err
		}

		// Create/Update Deployment
		if err = deployment.SyncDeployment(ctx, controller.Deployment, cluster.ClusterK8sClient, cr.Name); err != nil {
			return err
		}

		// Create/Update DeamonSet, except for auth proxy
		if !authorizationEnabled {
			if err = daemonset.SyncDaemonset(ctx, node.DaemonSetApplyConfig, cluster.ClusterK8sClient, cr.Name); err != nil {
				return err
			}
		}

		if replicationEnabled {
			// This will also create the dell-replication-controller namespace.
			if err = modules.ReplicationManagerController(ctx, false, operatorConfig, cr, cluster.ClusterCTRLClient); err != nil {
				return fmt.Errorf("failed to deploy replication controller: %v", err)
			}

			// Create ConfigMap if it does not already exist.
			// ConfigMap requires namespace to be created.
			_, err = modules.CreateReplicationConfigmap(ctx, cr, operatorConfig, ctrlClient)
			if err != nil {
				return fmt.Errorf("injecting replication into replication configmap: %v", err)
			}
		}

		// if Observability is enabled, create or update obs components: topology, metrics of PowerScale and PowerFlex
		if observabilityEnabled, _ := utils.IsModuleEnabled(ctx, cr, csmv1.Observability); observabilityEnabled {
			log.Infow("Create/Update observability")

			if err = r.reconcileObservability(ctx, false, operatorConfig, cr, nil, cluster.ClusterCTRLClient, cluster.ClusterK8sClient); err != nil {
				return err
			}
		}

	}

	return nil
}

// reconcileObservability - Delete/Create/Update observability components
// isDeleting - true: Delete; false: Create/Update
func (r *ContainerStorageModuleReconciler) reconcileObservability(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, components []string, ctrlClient client.Client, k8sClient kubernetes.Interface) error {
	log := logger.GetLogger(ctx)

	// if components is empty, reconcile all enabled components
	if len(components) == 0 {
		if enabled, obs := utils.IsModuleEnabled(ctx, cr, csmv1.Observability); enabled {
			for _, comp := range obs.Components {
				if utils.IsModuleComponentEnabled(ctx, cr, csmv1.Observability, comp.Name) {
					components = append(components, comp.Name)
				}
			}
		}
	}
	comp2reconFunc := map[string]func(context.Context, bool, utils.OperatorConfig, csmv1.ContainerStorageModule, client.Client) error{
		modules.ObservabilityTopologyName:         modules.ObservabilityTopology,
		modules.ObservabilityOtelCollectorName:    modules.OtelCollector,
		modules.ObservabilityCertManagerComponent: modules.CommonCertManager,
	}
	metricsComp2reconFunc := map[string]func(context.Context, bool, utils.OperatorConfig, csmv1.ContainerStorageModule, client.Client, kubernetes.Interface) error{
		modules.ObservabilityMetricsPowerScaleName: modules.PowerScaleMetrics,
		modules.ObservabilityMetricsPowerFlexName:  modules.PowerFlexMetrics,
		modules.ObservabilityMetricsPowerMaxName:   modules.PowerMaxMetrics,
	}

	for _, comp := range components {
		log.Infow(fmt.Sprintf("reconcile %s", comp))
		var err error
		switch comp {
		case modules.ObservabilityTopologyName, modules.ObservabilityOtelCollectorName, modules.ObservabilityCertManagerComponent:
			err = comp2reconFunc[comp](ctx, isDeleting, op, cr, ctrlClient)
		case modules.ObservabilityMetricsPowerScaleName, modules.ObservabilityMetricsPowerFlexName, modules.ObservabilityMetricsPowerMaxName:
			err = metricsComp2reconFunc[comp](ctx, isDeleting, op, cr, ctrlClient, k8sClient)
		default:
			err = fmt.Errorf("unsupported component type: %v", comp)
		}
		if err != nil {
			log.Errorf("failed to reconcile %s", comp)
			return err
		}
	}

	// We are doing this separately after creating other components because the certificates rely on cert-manager being up
	if err := modules.IssuerCertServiceObs(ctx, isDeleting, op, cr, ctrlClient); err != nil {
		return fmt.Errorf("unable to deploy Certificate & Issuer for Observability: %v", err)
	}

	return nil
}

// reconcileAuthorization - deploy authorization proxy server
func (r *ContainerStorageModuleReconciler) reconcileAuthorization(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	if utils.IsModuleComponentEnabled(ctx, cr, csmv1.AuthorizationServer, modules.AuthCertManagerComponent) {
		log.Infow("Reconcile authorization cert-manager")
		if err := modules.CommonCertManager(ctx, isDeleting, op, cr, ctrlClient); err != nil {
			return fmt.Errorf("unable to reconcile cert-manager for authorization: %v", err)
		}
	}

	if utils.IsModuleComponentEnabled(ctx, cr, csmv1.AuthorizationServer, modules.AuthProxyServerComponent) {
		log.Infow("Reconcile authorization proxy-server")
		if err := modules.AuthorizationServerDeployment(ctx, isDeleting, op, cr, ctrlClient); err != nil {
			return fmt.Errorf("unable to reconcile authorization proxy server: %v", err)
		}

		if err := modules.InstallPolicies(ctx, isDeleting, op, cr, ctrlClient); err != nil {
			return fmt.Errorf("unable to install policies: %v", err)
		}
	}

	if r.Config.IsOpenShift {
		log.Infow("Using OpenShift default ingress controller")
		if utils.IsModuleComponentEnabled(ctx, cr, csmv1.AuthorizationServer, modules.AuthNginxIngressComponent) {
			log.Warnw("openshift environment, skipping deployment of nginx ingress controller")
		}
	} else {
		if utils.IsModuleComponentEnabled(ctx, cr, csmv1.AuthorizationServer, modules.AuthNginxIngressComponent) {
			log.Infow("Reconcile authorization NGINX Ingress Controller")
			if err := modules.NginxIngressController(ctx, isDeleting, op, cr, ctrlClient); err != nil {
				return fmt.Errorf("unable to reconcile nginx ingress controller for authorization: %v", err)
			}
		}
	}

	// Authorization Ingress rules
	if utils.IsModuleComponentEnabled(ctx, cr, csmv1.AuthorizationServer, modules.AuthProxyServerComponent) {
		log.Infow("Reconcile authorization Ingresses")
		if err := modules.AuthorizationIngress(ctx, isDeleting, r.Config.IsOpenShift, cr, r, ctrlClient); err != nil {
			return fmt.Errorf("unable to reconcile authorization ingress rules: %v", err)
		}
	}

	log.Infow("Reconcile authorization certificates")
	if err := modules.InstallWithCerts(ctx, isDeleting, op, cr, ctrlClient); err != nil {
		return fmt.Errorf("unable to install certificates for Authorization: %v", err)
	}

	return nil
}

func (r *ContainerStorageModuleReconciler) reconcileAppMobilityCRDS(ctx context.Context, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	// AppMobility installs Application Mobility CRDS
	if utils.IsAppMobilityComponentEnabled(ctx, cr, r, csmv1.ApplicationMobility, modules.AppMobCtrlMgrComponent) {
		log.Infow("Reconcile Application Mobility CRDS")
		if err := modules.AppMobCrdDeploy(ctx, op, cr, ctrlClient); err != nil {
			return fmt.Errorf("unable to reconcile Application Mobility CRDs: %v", err)
		}
		if err := modules.VeleroCrdDeploy(ctx, op, cr, ctrlClient); err != nil {
			return fmt.Errorf("unable to reconcile Velero CRDS : %v", err)
		}
	}

	return nil
}

// reconcileAuthorizationCRDS - reconcile Authorization CRDs
func (r *ContainerStorageModuleReconciler) reconcileAuthorizationCRDS(ctx context.Context, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	// Install Authorization CRDs
	if utils.IsModuleComponentEnabled(ctx, cr, csmv1.AuthorizationServer, modules.AuthProxyServerComponent) {
		log.Infow("Reconcile Authorization CRDS")
		if err := modules.AuthCrdDeploy(ctx, op, cr, ctrlClient); err != nil {
			return fmt.Errorf("unable to reconcile Authorization CRDs: %v", err)
		}
	}

	return nil
}

// reconcileAppMobility - deploy Application Mobility
func (r *ContainerStorageModuleReconciler) reconcileAppMobility(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	// AppMobility installs Application Mobility Controller Manager
	if utils.IsAppMobilityComponentEnabled(ctx, cr, r, csmv1.ApplicationMobility, modules.AppMobCtrlMgrComponent) {
		log.Infow("Reconcile Application Mobility Controller Manager")
		if err := modules.AppMobilityWebhookService(ctx, isDeleting, op, cr, ctrlClient); err != nil {
			return fmt.Errorf("unable to deploy WebhookService for Application Mobility: %v", err)
		}
		if err := modules.ControllerManagerMetricService(ctx, isDeleting, op, cr, ctrlClient); err != nil {
			return fmt.Errorf("unable to deploy MetricService for Application Mobility: %v", err)
		}
		if utils.IsAppMobilityComponentEnabled(ctx, cr, r, csmv1.ApplicationMobility, modules.AppMobCertManagerComponent) {
			if err := modules.CommonCertManager(ctx, isDeleting, op, cr, ctrlClient); err != nil {
				return fmt.Errorf("unable to reconcile cert-manager for Application Mobility: %v", err)
			}
		}
		if err := modules.IssuerCertService(ctx, isDeleting, op, cr, ctrlClient); err != nil {
			return fmt.Errorf("unable to deploy Certificate & Issuer for Application Mobility: %v", err)
		}
		if err := modules.AppMobilityDeployment(ctx, isDeleting, op, cr, ctrlClient); err != nil {
			return fmt.Errorf("unable to reconcile Application Mobility controller Manager: %v", err)
		}
	}

	// Appmobility installs velero
	if utils.IsAppMobilityComponentEnabled(ctx, cr, r, csmv1.ApplicationMobility, modules.AppMobVeleroComponent) {
		log.Infow("Reconcile application mobility velero")
		if err := modules.AppMobilityVelero(ctx, isDeleting, op, cr, ctrlClient); err != nil {
			return fmt.Errorf("unable to reconcile velero for Application Mobility: %v", err)
		}
		if err := modules.UseBackupStorageLoc(ctx, isDeleting, op, cr, ctrlClient); err != nil {
			return fmt.Errorf("unable to apply backupstorage location for Application Mobility: %v", err)
		}
	}

	return nil
}

func getDriverConfig(ctx context.Context,
	cr csmv1.ContainerStorageModule,
	operatorConfig utils.OperatorConfig,
) (*DriverConfig, error) {
	var (
		err        error
		driver     *storagev1.CSIDriver
		configMap  *corev1.ConfigMap
		node       *utils.NodeYAML
		controller *utils.ControllerYAML
		log        = logger.GetLogger(ctx)
	)

	// if no driver is specified, return nil
	if cr.Spec.Driver.CSIDriverType == "" {
		log.Infof("No driver specified in manifest")
		return nil, nil
	}

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

// reconcileReverseProxy - deploy reverse proxy server
func (r *ContainerStorageModuleReconciler) reconcileReverseProxy(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)
	log.Infow("Reconcile reverseproxy proxy")
	if err := modules.ReverseProxyServer(ctx, isDeleting, op, cr, ctrlClient); err != nil {
		return fmt.Errorf("unable to reconcile reverse-proxy server: %v", err)
	}
	if err := modules.ReverseProxyStartService(ctx, isDeleting, op, cr, ctrlClient); err != nil {
		return fmt.Errorf("unable to reconcile reverse-proxy service: %v", err)
	}
	return nil
}

func removeDriverReplicaCluster(ctx context.Context, cluster utils.ReplicaCluster, driverConfig *DriverConfig) error {
	log := logger.GetLogger(ctx)
	var err error

	log.Infow("removing driver from", cluster.ClusterID)

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

	return nil
}

func (r *ContainerStorageModuleReconciler) removeDriver(ctx context.Context, instance csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) error {
	log := logger.GetLogger(ctx)

	// Get Driver resources
	driverConfig, err := getDriverConfig(ctx, instance, operatorConfig)
	if err != nil {
		log.Error("error in getDriverConfig")
		return err
	}
	// driverConfig = nil means no driver specified in manifest
	if driverConfig == nil {
		return nil
	}

	replicationEnabled, clusterClients, err := utils.GetDefaultClusters(ctx, instance, r)
	if err != nil {
		return err
	}
	for _, cluster := range clusterClients {
		if err = removeDriverReplicaCluster(ctx, cluster, driverConfig); err != nil {
			return err
		}
		if replicationEnabled {
			log.Infow("Deleting Replication controller")
			if err = modules.ReplicationManagerController(ctx, true, operatorConfig, instance, cluster.ClusterCTRLClient); err != nil {
				return err
			}
			log.Infow("Deleting Replication configmap")
			if err = modules.DeleteReplicationConfigmap(cluster.ClusterCTRLClient); err != nil {
				return err
			}
		}

		// remove module observability
		if observabilityEnabled, _ := utils.IsModuleEnabled(ctx, instance, csmv1.Observability); observabilityEnabled {
			log.Infow("Deleting observability")
			if err = r.reconcileObservability(ctx, true, operatorConfig, instance, nil, cluster.ClusterCTRLClient, cluster.ClusterK8sClient); err != nil {
				return err
			}
		}

	}

	return nil
}

// removeModule - remove standalone modules
func (r *ContainerStorageModuleReconciler) removeModule(ctx context.Context, instance csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	if authorizationEnabled, _ := utils.IsModuleEnabled(ctx, instance, csmv1.AuthorizationServer); authorizationEnabled {
		log.Infow("Deleting Authorization Proxy Server")
		if err := r.reconcileAuthorization(ctx, true, operatorConfig, instance, ctrlClient); err != nil {
			return err
		}
	}

	if appMobilityEnabled, _ := utils.IsModuleEnabled(ctx, instance, csmv1.ApplicationMobility); appMobilityEnabled {
		log.Infow("Deleting Application Mobility")
		if err := r.reconcileAppMobility(ctx, true, operatorConfig, instance, ctrlClient); err != nil {
			return err
		}
	}
	if reverseproxyEnabled, _ := utils.IsModuleEnabled(ctx, instance, csmv1.ReverseProxy); reverseproxyEnabled {
		log.Infow("Deleting ReverseProxy")
		if err := r.reconcileReverseProxy(ctx, true, operatorConfig, instance, ctrlClient); err != nil {
			return err
		}
	}

	return nil
}

// PreChecks - validate input values
func (r *ContainerStorageModuleReconciler) PreChecks(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) error {
	log := logger.GetLogger(ctx)
	// Check drivers
	switch cr.Spec.Driver.CSIDriverType {
	case csmv1.PowerScale:
		err := drivers.PrecheckPowerScale(ctx, cr, operatorConfig, r.GetClient())
		if err != nil {
			return fmt.Errorf("failed powerscale validation: %v", err)
		}
	case csmv1.PowerFlex:
		err := drivers.PrecheckPowerFlex(ctx, cr, operatorConfig, r.GetClient())
		if err != nil {
			return fmt.Errorf("failed powerflex validation: %v", err)
		}
	case csmv1.PowerStore:
		err := drivers.PrecheckPowerStore(ctx, cr, operatorConfig, r.GetClient())
		if err != nil {
			return fmt.Errorf("failed powerstore validation: %v", err)
		}

	case csmv1.Unity:
		err := drivers.PrecheckUnity(ctx, cr, operatorConfig, r.GetClient())
		if err != nil {
			return fmt.Errorf("failed unity validation: %v", err)
		}
	case csmv1.PowerMax:
		err := drivers.PrecheckPowerMax(ctx, cr, operatorConfig, r.GetClient())
		if err != nil {
			return fmt.Errorf("failed powermax validation: %v", err)
		}
	default:
		// Go to checkUpgrade if it is standalone module i.e. app mobility or authorizatio proxy server
		if cr.HasModule(csmv1.ApplicationMobility) || cr.HasModule(csmv1.AuthorizationServer) {
			break
		}

		return fmt.Errorf("unsupported driver type %s", cr.Spec.Driver.CSIDriverType)
	}

	upgradeValid, err := r.checkUpgrade(ctx, cr, operatorConfig)
	if err != nil {
		return fmt.Errorf("failed upgrade check: %v", err)
	} else if !upgradeValid {
		log.Infof("upgrade is not valid")
		return nil
	}

	// check for owner reference
	deployments := r.K8sClient.AppsV1().Deployments(cr.Namespace)
	driver, err := deployments.Get(ctx, cr.Name+"-controller", metav1.GetOptions{})
	if err != nil {
		log.Infow("Driver not installed yet")
	} else {
		if driver.GetOwnerReferences() != nil {
			found := false
			cred := driver.GetOwnerReferences()
			for _, m := range cred {
				if m.Name == cr.Name {
					log.Infow("Owner reference is found and matches")
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("required Owner reference not found. Please re-install driver ")
			}
		}
	}

	// check modules
	log.Infow("Starting prechecks for modules")
	for _, m := range cr.Spec.Modules {
		if m.Enabled {
			switch m.Name {
			case csmv1.Authorization:
				if err := modules.AuthorizationPrecheck(ctx, operatorConfig, m, *cr, r.GetClient()); err != nil {
					return fmt.Errorf("failed authorization validation: %v", err)
				}

			case csmv1.AuthorizationServer:
				if err := modules.AuthorizationServerPrecheck(ctx, operatorConfig, m, *cr, r); err != nil {
					return fmt.Errorf("failed authorization proxy server validation: %v", err)
				}

			case csmv1.Replication:
				if err := modules.ReplicationPrecheck(ctx, operatorConfig, m, *cr, r); err != nil {
					return fmt.Errorf("failed replication validation: %v", err)
				}

			case csmv1.Resiliency:
				if err := modules.ResiliencyPrecheck(ctx, operatorConfig, m, *cr, r); err != nil {
					return fmt.Errorf("failed resiliency validation: %v", err)
				}

			case csmv1.Observability:
				// observability precheck
				if err := modules.ObservabilityPrecheck(ctx, operatorConfig, m, *cr, r); err != nil {
					return fmt.Errorf("failed observability validation: %v", err)
				}
			case csmv1.ApplicationMobility:
				// ApplicationMobility precheck
				if err := modules.ApplicationMobilityPrecheck(ctx, operatorConfig, m, *cr, r); err != nil {
					return fmt.Errorf("failed Appmobility validation: %v", err)
				}
			case csmv1.ReverseProxy:
				if err := modules.ReverseProxyPrecheck(ctx, operatorConfig, m, *cr, r); err != nil {
					return fmt.Errorf("failed reverseproxy validation: %v", err)
				}
			default:
				return fmt.Errorf("unsupported module type %s", m.Name)
			}
		}
	}

	return nil
}

// Check for upgrade/if upgrade is appropriate
func (r *ContainerStorageModuleReconciler) checkUpgrade(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) (bool, error) {
	log := logger.GetLogger(ctx)

	// If it is an upgrade/downgrade, check to see if we meet the minimum version using GetUpgradeInfo, which returns the minimum version required
	// for the desired upgrade. If the upgrade path is not valid fail
	// Existing version
	annotations := cr.GetAnnotations()
	oldVersion, configVersionExists := annotations[configVersionKey]
	// If annotation exists, we are doing an upgrade or modify
	if configVersionExists {
		if cr.HasModule(csmv1.AuthorizationServer) {
			newVersion := cr.GetModule(csmv1.AuthorizationServer).ConfigVersion
			if newVersion == "v2.0.0-alpha" {
				return false, nil
			}
			return utils.IsValidUpgrade(ctx, oldVersion, newVersion, csmv1.Authorization, operatorConfig)
		}
		if cr.HasModule(csmv1.ApplicationMobility) {
			newVersion := cr.GetModule(csmv1.ApplicationMobility).ConfigVersion
			modules.ApplicationMobilityOldVersion = oldVersion
			return utils.IsValidUpgrade(ctx, oldVersion, newVersion, csmv1.ApplicationMobility, operatorConfig)
		}
		driverType := cr.Spec.Driver.CSIDriverType
		if driverType == csmv1.PowerScale {
			// use powerscale instead of isilon as the folder name is powerscale
			driverType = csmv1.PowerScaleName
		}
		newVersion := cr.Spec.Driver.ConfigVersion
		return utils.IsValidUpgrade(ctx, oldVersion, newVersion, driverType, operatorConfig)

	}
	log.Infow("proceeding with fresh driver install")
	return true, nil
}

// applyConfigVersionAnnotations - applies the config version annotation to the instance.
func applyConfigVersionAnnotations(ctx context.Context, instance *csmv1.ContainerStorageModule) bool {
	log := logger.GetLogger(ctx)

	annotations := instance.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[CSMVersionKey] = CSMVersion

	var configVersion string
	if instance.HasModule(csmv1.AuthorizationServer) {
		configVersion = instance.GetModule(csmv1.AuthorizationServer).ConfigVersion
	} else if instance.HasModule(csmv1.ApplicationMobility) {
		configVersion = instance.GetModule(csmv1.ApplicationMobility).ConfigVersion
	} else {
		configVersion = instance.Spec.Driver.ConfigVersion
	}

	if annotations[configVersionKey] != configVersion {
		annotations[configVersionKey] = configVersion
		log.Infof("Installing csm component %s with config Version %s. Updating Annotations with Config Version",
			instance.GetName(), configVersion)
		instance.SetAnnotations(annotations)
		return true
	}

	return false
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
