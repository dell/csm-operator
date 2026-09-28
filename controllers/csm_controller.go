// Copyright © 2021-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dell/csm-operator/k8s"
	"github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/modules"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
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
	"go.uber.org/zap"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	t1 "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	sinformer "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ContainerStorageModuleReconciler reconciles a ContainerStorageModule object
type ContainerStorageModuleReconciler struct {
	// controller runtime client, responsible for create, delete, update, get etc.
	client.Client
	// k8s client, implements client-go/kubernetes interface, responsible for apply, which
	// client.Client does not provides
	K8sClient            kubernetes.Interface
	Scheme               *runtime.Scheme
	Log                  *zap.SugaredLogger
	Config               operatorutils.OperatorConfig
	updateCount          int32
	trcID                string
	EventRecorder        record.EventRecorder
	ContentWatchChannels map[string]chan struct{}
	ContentWatchLock     sync.Mutex
	Metrics              *OperatorMetrics // Prometheus metrics; nil disables recording
	// ClusterID is the unique identifier for this K8s cluster (kube-system namespace UID).
	ClusterID string
}

// DriverConfig  -
type DriverConfig struct {
	Driver     *storagev1.CSIDriver
	ConfigMap  *corev1.ConfigMap
	Node       *operatorutils.NodeYAML
	Controller *operatorutils.ControllerYAML
}

const (
	// MetadataPrefix - prefix for all labels & annotations
	MetadataPrefix = "storage.dell.com"

	// NodeYaml - yaml file name for node
	NodeYaml = "node.yaml"

	// CSMFinalizerName -
	CSMFinalizerName = "finalizer.dell.emc.com"

	// CSMVersion -
	CSMVersion = "v1.18.0"

	// RefreshEnvVar - environment variable name for watcher timed refreshes
	RefreshEnvVar = "REFRESH_INTERVAL_MINUTES"
)

var (
	dMutex                          sync.RWMutex
	configVersionKey                = fmt.Sprintf("%s/%s", MetadataPrefix, "CSMOperatorConfigVersion")
	previouslyAppliedCustomResource = fmt.Sprintf("%s/%s", MetadataPrefix, "PreviouslyAppliedConfiguration")
	preUpgradeSnapshotKey           = fmt.Sprintf("%s/%s", MetadataPrefix, "PreUpgradeSnapshot")
	pendingRechecks                 sync.Map // map[string]*time.Timer — deduplicates status rechecks per CSM
	lastReconciledState             sync.Map // map[string]CSMStateType — tracks last state the reconciler emitted an event for

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
// +kubebuilder:rbac:groups="replication.storage.openshift.io",resources=volumegroupreplications;volumegroupreplications/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="replication.storage.openshift.io",resources=volumegroupreplicationclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups="replication.storage.openshift.io",resources=volumegroupreplicationcontents;volumegroupreplicationcontents/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="replication.storage.openshift.io",resources=volumegroupreplications/finalizers;volumegroupreplicationcontents/finalizers,verbs=update
// +kubebuilder:rbac:groups="csiaddons.openshift.io",resources=csiaddonsnodes,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=pods;services;services/finalizers;endpoints;persistentvolumeclaims;events;configmaps;secrets;serviceaccounts;roles;ingresses,verbs=*
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;create;patch;update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=update;patch;get
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=create;update;get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;create;delete;patch;update
// +kubebuilder:rbac:groups="apps",resources=deployments;daemonsets;replicasets;statefulsets,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles;clusterrolebindings;replicasets;rolebindings,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles/finalizers,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=subjectaccessreviews,verbs=create
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;update;create;delete;patch
// +kubebuilder:rbac:groups="monitoring.coreos.com",resources=servicemonitors,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="monitoring.coreos.com",resources=podmonitors,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="monitoring.coreos.com",resources=prometheusrules,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="",resources=deployments/finalizers,resourceNames=dell-csm-operator-controller-manager,verbs=update
// +kubebuilder:rbac:groups="storage.k8s.io",resources=csidrivers,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="storage.k8s.io",resources=storageclasses,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="storage.k8s.io",resources=volumeattachments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="storage.k8s.io",resources=volumeattributesclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups="storage.k8s.io",resources=csinodes,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="csi.storage.k8s.io",resources=csinodeinfos,verbs=get;list;watch
// +kubebuilder:rbac:groups="snapshot.storage.k8s.io",resources=volumesnapshotclasses;volumesnapshotcontents,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="snapshot.storage.k8s.io",resources=volumesnapshotcontents/status,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups="snapshot.storage.k8s.io",resources=volumesnapshots,verbs=get;list;watch;update;patch;create;delete
// +kubebuilder:rbac:groups="snapshot.storage.k8s.io",resources=volumesnapshots/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="cdi.kubevirt.io",resources=datavolumes,verbs=get;update;delete
// +kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions,verbs=*
// +kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions/status,verbs=get;list;patch;watch
// +kubebuilder:rbac:groups="storage.k8s.io",resources=volumeattachments/status,verbs=patch
// +kubebuilder:rbac:groups="metrics.k8s.io",resources=pods,verbs=get;list
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="security.openshift.io",resources=securitycontextconstraints,resourceNames=privileged,verbs=use
// +kubebuilder:rbac:urls="/metrics",verbs=get
// +kubebuilder:rbac:groups="authentication.k8s.io",resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups="authorization.k8s.io",resources=subjectaccessreviews,verbs=create
// +kubebuilder:rbac:groups="cert-manager.io",resources=issuers;issuers/status,verbs=update;get;list;watch;patch
// +kubebuilder:rbac:groups="cert-manager.io",resources=clusterissuers;clusterissuers/status,verbs=update;get;list;watch;patch
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates;certificaterequests;clusterissuers;issuers,verbs=*
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates/finalizers;certificaterequests/finalizers;clusterissuers/finalizers;issuers/finalizers,verbs=update
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates/status;certificaterequests/status,verbs=update;patch
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates;certificaterequests;issuers,verbs=create;delete;deletecollection;patch;update
// +kubebuilder:rbac:groups="cert-manager.io",resources=signers,resourceNames=issuers.cert-manager.io/*;clusterissuers.cert-manager.io/*,verbs=approve
// +kubebuilder:rbac:groups="cert-manager.io",resources=*/*,verbs=*
// +kubebuilder:rbac:groups="",resources=secrets,resourceNames=cert-manager-webhook-ca,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="cert-manager.io",resources=configmaps,resourceNames=cert-manager-cainjector-leader-election;cert-manager-cainjector-leader-election-core,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,resourceNames=cert-manager-controller,verbs=get;update;patch
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
// +kubebuilder:rbac:groups="gateway.networking.k8s.io",resources=gateways,verbs=create;delete;get;list;update;watch
// +kubebuilder:rbac:groups="gateway.networking.k8s.io",resources=gateways/finalizers;httproutes/finalizers,verbs=update
// Gateway API controller RBAC - the operator must hold these permissions to create the nginx-gateway-fabric ClusterRole
// +kubebuilder:rbac:groups="autoscaling",resources=horizontalpodautoscalers,verbs=create;update;delete;get;list;watch
// +kubebuilder:rbac:groups="gateway.networking.k8s.io",resources=gatewayclasses,verbs=create;delete;get;list;update;watch
// +kubebuilder:rbac:groups="gateway.networking.k8s.io",resources=grpcroutes;backendtlspolicies;referencegrants;listenersets,verbs=get;list;watch
// +kubebuilder:rbac:groups="gateway.networking.k8s.io",resources=gateways/status;gatewayclasses/status;httproutes/status;grpcroutes/status;backendtlspolicies/status;listenersets/finalizers,verbs=update
// +kubebuilder:rbac:groups="gateway.nginx.org",resources=nginxgateways,verbs=create;delete;get;list;update;watch
// +kubebuilder:rbac:groups="gateway.nginx.org",resources=nginxproxies,verbs=create;delete;get;list;update;watch
// +kubebuilder:rbac:groups="gateway.nginx.org",resources=clientsettingspolicies;observabilitypolicies;upstreamsettingspolicies;authenticationfilters;proxysettingspolicies;ratelimitpolicies,verbs=list;watch
// +kubebuilder:rbac:groups="gateway.nginx.org",resources=nginxgateways/status;clientsettingspolicies/status;observabilitypolicies/status;upstreamsettingspolicies/status;authenticationfilters/status;proxysettingspolicies/status;ratelimitpolicies/status,verbs=update
// +kubebuilder:rbac:groups="route.openshift.io",resources=routes/custom-host,verbs=create
// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=validatingwebhookconfigurations;mutatingwebhookconfigurations,verbs=create;get;list;watch;update;delete;patch
// +kubebuilder:rbac:groups="apiregistration.k8s.io",resources=apiservices,verbs=get;list;watch;update;patch
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
// +kubebuilder:rbac:groups="dr.storage.dell.com",resources=volumejournals,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="dr.storage.dell.com",resources=volumejournals/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=objectstorage.k8s.io,resources=bucketaccessclasses,verbs=create;get;update;delete;list;watch
// +kubebuilder:rbac:groups=objectstorage.k8s.io,resources=bucketaccessclasses/status,verbs=create;get;update;delete;list;watch
// +kubebuilder:rbac:groups=objectstorage.k8s.io,resources=bucketaccesses,verbs=create;get;update;delete;list;watch
// +kubebuilder:rbac:groups=objectstorage.k8s.io,resources=bucketaccesses/status,verbs=create;get;update;delete;list;watch
// +kubebuilder:rbac:groups=objectstorage.k8s.io,resources=bucketclaims,verbs=create;get;update;delete;list;watch
// +kubebuilder:rbac:groups=objectstorage.k8s.io,resources=bucketclaims/status,verbs=create;get;update;delete;list;watch
// +kubebuilder:rbac:groups=objectstorage.k8s.io,resources=buckets,verbs=create;get;update;delete;list;watch
// +kubebuilder:rbac:groups=objectstorage.k8s.io,resources=buckets/status,verbs=create;get;update;delete;list;watch

// +kubebuilder:rbac:groups="groupsnapshot.storage.k8s.io",resources=volumegroupsnapshotclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups="groupsnapshot.storage.k8s.io",resources=volumegroupsnapshotcontents,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="groupsnapshot.storage.k8s.io",resources=volumegroupsnapshotcontents/status,verbs=update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ContainerStorageModule object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile

// csmDriverLabel extracts a short driver label from a CSIDriverType (e.g. "csi-powermax.dellemc.com" → "powermax").
func csmDriverLabel(driverType csmv1.DriverType) string {
	s := string(driverType)
	for _, known := range []string{"powermax", "powerflex", "powerscale", "powerstore", "unity"} {
		if strings.Contains(strings.ToLower(s), known) {
			return known
		}
	}
	if s != "" {
		return s
	}
	return "unknown"
}

// supportsPodMonitor returns true if the driver supports PodMonitor for node pod metrics.
// Currently supported: PowerScale, PowerStore, PowerMax, PowerFlex.
func supportsPodMonitor(driverType csmv1.DriverType) bool {
	return driverType == csmv1.PowerScale ||
		driverType == csmv1.PowerStore ||
		driverType == csmv1.PowerMax ||
		driverType == csmv1.PowerFlex
}

// Reconcile - main loop
func (r *ContainerStorageModuleReconciler) Reconcile(_ context.Context, req ctrl.Request) (ctrl.Result, error) {
	reconcileStart := time.Now()
	reconcileStatus := "success"
	reconcileDriver := "unknown"
	clusterID := r.ClusterID
	if clusterID == "" {
		clusterID = "unknown"
	}
	defer func() {
		if r.Metrics != nil {
			r.Metrics.RecordReconciliation(clusterID, reconcileDriver, "driver", reconcileStatus, time.Since(reconcileStart))
		}
	}()

	r.IncrUpdateCount()
	r.trcID = fmt.Sprintf("%d", r.GetUpdateCount())
	name := req.Name + "-" + r.trcID
	ctx, log := logger.GetNewContextWithLogger(name)
	unitTestRun := operatorutils.DetermineUnitTestRun(ctx)

	log.Info("################Starting Reconcile##############")
	defer operatorutils.LogEndReconcile()
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
		reconcileStatus = "failure"
		// Error reading the object - requeue the request.
		return reconcile.Result{}, nil
	}

	// Look up the last state the reconciler emitted a transition event for.
	// We use an in-memory map rather than the CR status because ContentWatch
	// handlers can update the CR status (e.g., to Failed) between reconcile
	// loops, making the stored state unreliable for transition detection.
	crKey := req.Namespace + "/" + req.Name
	var previousState csmv1.CSMStateType
	if v, ok := lastReconciledState.Load(crKey); ok {
		previousState = v.(csmv1.CSMStateType)
	}

	// Record metrics for this reconciliation
	if r.Metrics != nil {
		reconcileDriver = csmDriverLabel(csm.Spec.Driver.CSIDriverType)
		state := string(csm.GetCSMStatus().State)
		if state == "" {
			state = "unknown"
		}
		// Track this K8s cluster (1 per operator instance)
		r.Metrics.SetClustersManaged(clusterID, 1)
		// Track K8s API connectivity (always connected if reconcile runs)
		r.Metrics.SetClusterConnectivity(clusterID, true)
		// Track CR state
		r.Metrics.SetCRCount(clusterID, reconcileDriver, state, 1)
	}

	operatorConfig := &operatorutils.OperatorConfig{
		IsOpenShift:     r.Config.IsOpenShift,
		K8sVersion:      r.Config.K8sVersion,
		ConfigDirectory: r.Config.ConfigDirectory,
	}

	// Set default value for forceRemoveDriver to true if not specified by the user
	if csm.Spec.Driver.ForceRemoveDriver == nil {
		truebool := true
		csm.Spec.Driver.ForceRemoveDriver = &truebool
	}

	// Set default components if using miminal manifest (without components)
	err = operatorutils.LoadDefaultComponents(ctx, csm, *operatorConfig)
	if err != nil {
		reconcileStatus = "failure"
		return ctrl.Result{}, err
	}

	for i, m := range csm.Spec.Modules {
		if m.Name == csmv1.AuthorizationServer {
			authVersion, err := operatorutils.GetVersion(ctx, csm, *operatorConfig)
			if err != nil {
				reconcileStatus = "failure"
				return ctrl.Result{}, err
			}
			csm.Spec.Modules[i].ConfigVersion = authVersion
			break
		}
	}

	// perform prechecks
	err = r.PreChecks(ctx, csm, *operatorConfig)
	if err != nil {
		reconcileStatus = "failure"
		csm.GetCSMStatus().State = constants.InvalidConfig
		r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventUpdated, fmt.Sprintf("Failed Prechecks: %s", err))
		return operatorutils.HandleValidationError(ctx, csm, r, err)
	}

	// Auto-upgrade: when spec.Upgrade is "auto", automatically update
	// spec.Version to the latest supported version. This triggers the existing
	// upgrade detection, snapshot, and rollback infrastructure on the next reconcile.
	if csm.Spec.Upgrade == csmv1.UpgradePolicyAuto {
		autoUpgraded, autoUpgradeErr := r.handleAutoUpgrade(ctx, csm, *operatorConfig)
		if autoUpgradeErr != nil {
			log.Errorw("Auto-upgrade check failed", "error", autoUpgradeErr)
			r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventUpdated,
				fmt.Sprintf("Auto-upgrade check failed: %s", autoUpgradeErr))
		} else if autoUpgraded {
			// CR was updated with new version; requeue to let upgrade detection pick it up
			return reconcile.Result{Requeue: true}, nil
		}
	}

	// Save pre-upgrade snapshot if version is changing and no snapshot exists.
	// The snapshot must be persisted to the API server BEFORE setting status to Pending,
	// because the status change can trigger informer-driven handlers that check for the
	// snapshot to decide whether to cap status at Pending.
	upgradeDetected := operatorutils.IsUpgradeInProgress(ctx, csm, *operatorConfig)

	// Clear the succeeded stability period when an upgrade is detected.
	// This ensures the CR remains in Pending during the upgrade, rather than
	// immediately transitioning to Succeeded due to a stale stability entry
	// from the previous Succeeded state.
	if upgradeDetected {
		crKey = req.Namespace + "/" + req.Name
		operatorutils.ClearSucceededStabilityPeriod(crKey)
	}

	// Emit platform-specific warning if upgrade is in progress
	if upgradeDetected {
		emitPlatformWarning(ctx, csm, r.EventRecorder)
	}

	if csm.IsBeingDeleted() {
		log.Infow("Delete request", "csm", req.Namespace, "Name", req.Name)

		// Clear failure tracking to prevent memory leak
		crKey := req.Namespace + "/" + req.Name
		operatorutils.ClearFailureGracePeriod(crKey)

		// check for force cleanup
		if *csm.Spec.Driver.ForceRemoveDriver {
			// remove all resources deployed from CR by operator
			if err := r.removeDriver(ctx, *csm, *operatorConfig); err != nil {
				reconcileStatus = "failure"
				r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventDeleted, fmt.Sprintf("Failed to remove driver: %s", err))
				log.Errorw("remove driver", "error", err.Error())
				return ctrl.Result{}, fmt.Errorf("error when deleting driver: %v", err)
			}
		} else {
			// When forceRemoveDriver is false, remove ownerReferences from the
			// controller deployment so that Kubernetes garbage collection does
			// not delete it when the CSM CR is removed.
			if err := r.removeDeploymentOwnerRef(ctx, csm); err != nil {
				log.Infow("Could not remove ownerReference from deployment", "error", err.Error())
			}
		}

		// check for force cleanup on standalone module
		for _, m := range csm.Spec.Modules {
			if m.ForceRemoveModule {
				// remove all resources deployed from CR by operator
				if err := r.removeModule(ctx, *csm, *operatorConfig, r.Client); err != nil {
					reconcileStatus = "failure"
					r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventDeleted, fmt.Sprintf("Failed to remove module: %s", err))
					return ctrl.Result{}, fmt.Errorf("error when deleting module: %v", err)
				}
			}
		}
		if err := r.removeFinalizer(ctx, csm); err != nil {
			reconcileStatus = "failure"
			r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventDeleted, fmt.Sprintf("Failed to delete finalizer: %s", err))
			log.Errorw("remove driver finalizer", "error", err.Error())
			return ctrl.Result{}, fmt.Errorf("error when handling finalizer: %v", err)
		}

		// stop this CSM's informers
		r.ContentWatchLock.Lock()
		if stopCh, ok := r.ContentWatchChannels[csm.Name]; ok {
			close(stopCh)
			delete(r.ContentWatchChannels, csm.Name)
		}
		r.ContentWatchLock.Unlock()

		r.EventRecorder.Event(csm, corev1.EventTypeNormal, csmv1.EventDeleted, "Object finalizer is deleted")
		return ctrl.Result{}, nil
	}

	// Add finalizer
	if !csm.HasFinalizer(CSMFinalizerName) {
		log.Infow("HandleFinalizer", "name", CSMFinalizerName)
		if err := r.addFinalizer(ctx, csm); err != nil {
			reconcileStatus = "failure"
			r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventUpdated, fmt.Sprintf("Failed to add finalizer: %s", err))
			log.Errorw("HandleFinalizer", "error", err.Error())
			return ctrl.Result{}, fmt.Errorf("error when adding finalizer: %v", err)
		}
		r.EventRecorder.Event(csm, corev1.EventTypeNormal, csmv1.EventUpdated, "Object finalizer is added")
	}

	if upgradeDetected {
		snapshotSaved, err := savePreUpgradeSnapshot(ctx, csm)
		if err != nil {
			reconcileStatus = "failure"
			log.Errorw("Failed to save pre-upgrade snapshot", "error", err)
			r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventUpdated, fmt.Sprintf("Failed to save pre-upgrade snapshot: %s", err))
			return reconcile.Result{}, err
		}

		if snapshotSaved {
			latestCSM := &csmv1.ContainerStorageModule{}
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := r.GetClient().Get(ctx, t1.NamespacedName{
					Name:      csm.Name,
					Namespace: csm.Namespace,
				}, latestCSM)
				if err != nil {
					return err
				}
				// Copy the snapshot annotation from the local csm to latestCSM
				if annotations := csm.GetAnnotations(); annotations != nil {
					if snapshot, exists := annotations[preUpgradeSnapshotKey]; exists {
						if latestCSM.Annotations == nil {
							latestCSM.Annotations = make(map[string]string)
						}
						latestCSM.Annotations[preUpgradeSnapshotKey] = snapshot
					}
				}

				return r.GetClient().Update(ctx, latestCSM)
			})
			if err != nil {
				log.Error(err, "Failed to update CR with snapshot annotation")
				return reconcile.Result{}, err
			}

			// Update that Status due to having different callers.
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := r.GetClient().Get(ctx, t1.NamespacedName{
					Name:      csm.Name,
					Namespace: csm.Namespace,
				}, latestCSM)
				if err != nil {
					return err
				}

				latestCSM.Status.State = constants.Pending
				return r.GetClient().Status().Update(ctx, latestCSM)
			})
			if err != nil {
				log.Error(err, "Failed to update CR status")
				return reconcile.Result{}, err
			}

			csm = latestCSM
			csm.Status.State = constants.Pending
		}

		log.Infof("Upgrade detected for %s, status set to %s", csm.Name, csm.Status.State)
	}

	newStatus := csm.GetCSMStatus()

	// Update the driver
	syncErr := r.SyncCSM(ctx, *csm, *operatorConfig, r.Client)

	// Update status regardless of sync result to show actual state (Failed/Pending/Succeeded)
	statusRequeue, err := operatorutils.UpdateStatus(ctx, csm, r, newStatus, *operatorConfig)

	// Save the state that calculateState computed. newStatus is a pointer to
	// csm.Status, so any subsequent r.GetClient().Get(ctx, ..., csm) will
	// overwrite csm.Status and silently clobber newStatus.State with whatever
	// the API has at that instant (which may have been changed by a concurrent
	// informer handler). The reconciler must base its decisions (snapshot
	// clearing, Pending requeue, Failed rollback) on its OWN calculated state.
	calculatedState := newStatus.State
	lastReconciledState.Store(crKey, calculatedState)

	if err != nil && !unitTestRun {
		reconcileStatus = "failure"
		log.Error(err, "Failed to update CR status")
		// Don't return statusRequeue with error - controller-runtime ignores Requeue when error is non-nil
		// Framework will automatically requeue with exponential backoff
		return reconcile.Result{}, err
	}

	driverLabel := string(csm.GetDriverType())
	if syncErr == nil {
		// Persist the version annotation after successful sync
		var latestCSM *csmv1.ContainerStorageModule
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			latestCSM = &csmv1.ContainerStorageModule{}
			err := r.GetClient().Get(ctx, t1.NamespacedName{
				Name:      csm.Name,
				Namespace: csm.Namespace,
			}, latestCSM)
			if err != nil {
				return err
			}
			applyConfigVersionAnnotations(ctx, latestCSM, *operatorConfig)
			latestCSM.Status = *newStatus
			return r.GetClient().Update(ctx, latestCSM)
		})
		if err != nil {
			log.Error(err, "Failed to update CR with annotation after successful sync")
			return reconcile.Result{Requeue: true}, err
		}

		// Use the already-updated latestCSM object instead of re-fetching from cache.
		// The RetryOnConflict block updated latestCSM via the API server, so it has
		// the freshly persisted configVersionKey annotation.
		csm = latestCSM

		log.Infof("Retrieve CSM with the latest information, calculated state %s and annotation version %s", calculatedState, csm.Annotations[configVersionKey])

		// Clear the pre-upgrade snapshot when the upgrade has fully stabilized.
		// Clear when state is Succeeded and snapshot still exists. The snapshot
		// existence check is more reliable than checking version annotations which
		// can be stale. clearPreUpgradeSnapshot is a no-op when no snapshot exists.
		if calculatedState == constants.Succeeded {
			annotations := csm.GetAnnotations()
			_, snapshotExists := annotations[preUpgradeSnapshotKey]
			if snapshotExists {
				log.Infoln("Clearing the PreUpgradeSnapshot")
				if err := r.clearPreUpgradeSnapshot(ctx, csm); err != nil {
					log.Errorw("Failed to clear pre-upgrade snapshot, will retry", "error", err)
					return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
				}
			}
		}

		// start content watch for this CSM
		stop, err := r.ContentWatch(csm)
		if err != nil {
			reconcileStatus = "failure"
			log.Errorf("starting content watch for %s: %v", csm.Name, err)
			// Don't return Requeue with error - controller-runtime ignores Requeue when error is non-nil
			// Framework will automatically requeue with exponential backoff
			return reconcile.Result{}, err
		}

		r.ContentWatchLock.Lock()
		if stopCh, ok := r.ContentWatchChannels[csm.Name]; ok {
			close(stopCh)
		}
		r.ContentWatchChannels[csm.Name] = stop
		r.ContentWatchLock.Unlock()

		if r.Metrics != nil {
			r.Metrics.RecordReconciliation(clusterID, driverLabel, "csi", "success", time.Since(reconcileStart))
		}

		// Use the requeue decision from UpdateStatus based on the final calculated state
		if calculatedState == constants.Pending {
			// Only emit "in progress" on a state transition to Pending.
			// Subsequent requeued reconciles already have Pending stored in status.
			if previousState != constants.Pending {
				r.EventRecorder.Eventf(csm, corev1.EventTypeNormal, csmv1.EventUpdated, "install/update storage component: %s in progress", csm.Name)
			}
			return statusRequeue, nil
		}

		// When state is Failed (e.g., grace period elapsed while syncErr is nil),
		// attempt rollback if a pre-upgrade snapshot exists. If no snapshot exists,
		// rollback already completed on a prior reconcile; just requeue.
		if calculatedState == constants.Failed {
			// Emit a failure event on state transition, regardless of snapshot existence.
			if previousState != constants.Failed {
				r.EventRecorder.Eventf(csm, corev1.EventTypeWarning, csmv1.EventUpdated, "install/update storage component: %s failed", csm.Name)
			}
			annotations := csm.GetAnnotations()
			if _, hasSnapshot := annotations[preUpgradeSnapshotKey]; hasSnapshot {
				r.EventRecorder.Eventf(csm, corev1.EventTypeNormal, csmv1.EventUpdated,
					"Upgrade failed, starting automatic rollback for %s", csm.Name)
				rollbackErr := r.attemptRollback(ctx, csm, *operatorConfig)
				if rollbackErr != nil {
					log.Errorw("Rollback failed", "error", rollbackErr)
					r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventUpdated, fmt.Sprintf("Rollback failed: %s", rollbackErr))
				}
			}
			return statusRequeue, nil
		}

		r.EventRecorder.Eventf(csm, corev1.EventTypeNormal, csmv1.EventCompleted, "install/update storage component: %s completed OK", csm.Name)
		return statusRequeue, nil
	}

	// Attempt automatic rollback on upgrade failure if a pre-upgrade snapshot exists.
	// If no snapshot exists, rollback already completed on a prior reconcile.
	if calculatedState == constants.Failed {
		// Emit a failure event on state transition, regardless of snapshot existence.
		if previousState != constants.Failed {
			r.EventRecorder.Eventf(csm, corev1.EventTypeWarning, csmv1.EventUpdated, "install/update storage component: %s failed", csm.Name)
		}
		annotations := csm.GetAnnotations()
		if _, hasSnapshot := annotations[preUpgradeSnapshotKey]; hasSnapshot {
			r.EventRecorder.Eventf(csm, corev1.EventTypeNormal, csmv1.EventUpdated,
				"Upgrade failed, starting automatic rollback for %s", csm.Name)
			rollbackErr := r.attemptRollback(ctx, csm, *operatorConfig)
			if rollbackErr != nil {
				log.Errorw("Rollback failed", "error", rollbackErr)
				r.EventRecorder.Event(csm, corev1.EventTypeWarning, csmv1.EventUpdated, fmt.Sprintf("Rollback failed: %s", rollbackErr))
			}
		}
	}

	// Failed deployment — syncErr is guaranteed non-nil here (the syncErr==nil branch returns above)
	r.EventRecorder.Eventf(csm, corev1.EventTypeWarning, csmv1.EventUpdated, "Failed install: %s", syncErr.Error())

	reconcileStatus = "failure"

	if r.Metrics != nil {
		r.Metrics.RecordReconciliation(clusterID, driverLabel, "csi", "failure", time.Since(reconcileStart))
	}

	// Don't return Requeue with error - controller-runtime ignores Requeue when error is non-nil
	// Framework will automatically requeue with exponential backoff
	return reconcile.Result{}, syncErr
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
	log.Debugf("[handleDeploymentUpdate] deployment name: %s, desired: %d, numberReady: %d, available: %d, numberUnavailable: %d", d.Name, desired, ready, available, numberUnavailable)

	ns := d.Spec.Template.Labels[constants.CsmNamespaceLabel]

	if ns != "" {
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

		// Compute actual failed pods (pods in CrashLoopBackOff, ImagePullBackOff, etc.)
		// rather than using numberUnavailable, which is non-zero during normal pod startup.
		var labelSel map[string]string
		if d.Spec.Selector != nil {
			labelSel = d.Spec.Selector.MatchLabels
		}
		failedPods := operatorutils.ComputeDeploymentFailedPods(ctx, r.Client, d.Namespace, labelSel, numberUnavailable)

		// Pass deployment status from the event object directly to avoid stale informer cache reads
		deploymentStatus := csmv1.PodStatus{
			Available: strconv.Itoa(int(available)),
			Desired:   strconv.Itoa(int(desired)),
			Failed:    strconv.Itoa(int(failedPods)),
		}

		log.Infof("[handleDeploymentUpdate] deployment: %s, namespace: %s, name: %s", d.Name, d.Namespace, name)

		// During an active upgrade (PreUpgradeSnapshot exists), pods may
		// transiently appear ready. Cap status at Pending when snapshot exists
		// to prevent false positive Succeeded state during rolling updates.
		if newStatus.State == constants.Succeeded {
			annotations := csm.GetAnnotations()
			_, snapshotExists := annotations[preUpgradeSnapshotKey]
			if snapshotExists {
				log.Infoln("Setting status (deployment) to Pending due to PreUpgradeSnapshot")
				newStatus.State = constants.Pending
			}
		}

		result, _ := operatorutils.UpdateStatus(ctx, csm, r, newStatus, r.Config, deploymentStatus)
		// Only schedule a recheck when the stability period is specifically the
		// reason for Pending. Other Pending causes (pods not ready, failure grace
		// period) will be resolved by subsequent informer events.
		crKey := csm.GetNamespace() + "/" + csm.GetName()
		if result.RequeueAfter > 0 && operatorutils.IsStabilityPeriodPending(crKey) {
			r.scheduleStatusRecheck(namespacedName, result.RequeueAfter)
		}

		if newStatus.State == constants.Succeeded {
			r.EventRecorder.Eventf(csm, corev1.EventTypeNormal, csmv1.EventCompleted, "Deployment %s running OK", d.Name)
		}
	}
}

func (r *ContainerStorageModuleReconciler) handlePodsUpdate(_ interface{}, obj interface{}) {
	dMutex.Lock()
	defer dMutex.Unlock()

	p, _ := obj.(*corev1.Pod)
	name := p.GetLabels()[constants.CsmLabel]
	// if this pod is an obs. pod, namespace might not match csm namespace
	ns := p.GetLabels()[constants.CsmNamespaceLabel]

	if ns != "" {
		key := name + "-" + fmt.Sprintf("%d", r.GetUpdateCount())
		ctx, log := logger.GetNewContextWithLogger(key)

		if !p.ObjectMeta.DeletionTimestamp.IsZero() {
			log.Debugw("driver delete invoked", "stopping pod with name", p.Name)
			return
		}
		log.Debugw("pod modified for driver", "name", p.Name)

		namespacedName := t1.NamespacedName{
			Name:      name,
			Namespace: ns,
		}
		csm := new(csmv1.ContainerStorageModule)
		err := r.Client.Get(ctx, namespacedName, csm)
		if err != nil {
			r.Log.Errorw("daemonset get csm", "error", err.Error())
		}

		newStatus := csm.GetCSMStatus()

		log.Infof("[handlePodsUpdate] pods: %s, namespace: %s, name: %s", p.Name, p.Namespace, name)
		// During an active upgrade (PreUpgradeSnapshot exists), pods may
		// transiently appear ready. Cap status at Pending when snapshot exists
		// to prevent false positive Succeeded state during rolling updates.
		if newStatus.State == constants.Succeeded {
			annotations := csm.GetAnnotations()
			_, snapshotExists := annotations[preUpgradeSnapshotKey]
			if snapshotExists {
				log.Infoln("Setting status (pods) to Pending due to PreUpgradeSnapshot")
				newStatus.State = constants.Pending
			}
		}

		result, _ := operatorutils.UpdateStatus(ctx, csm, r, newStatus, r.Config)
		crKey := csm.GetNamespace() + "/" + csm.GetName()
		if result.RequeueAfter > 0 && operatorutils.IsStabilityPeriodPending(crKey) {
			r.scheduleStatusRecheck(namespacedName, result.RequeueAfter)
		}

		if newStatus.State == constants.Succeeded {
			r.EventRecorder.Eventf(csm, corev1.EventTypeNormal, csmv1.EventCompleted, "Pod %s running OK", p.Name)
		}
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

	log.Debugf("[handleDaemonsetUpdate] daemonset: %s, namespace: %s, name: %s, desired: %d, numberReady: %d, available: %d, numberUnavailable: %d",
		d.Name, d.Namespace, name, d.Status.DesiredNumberScheduled,
		d.Status.NumberReady, d.Status.NumberAvailable,
		d.Status.NumberUnavailable)

	ns := d.Spec.Template.Labels[constants.CsmNamespaceLabel]

	if ns != "" {
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

		newStatus := csm.GetCSMStatus()

		log.Infof("[handleDaemonsetUpdate] daemonset: %s, namespace: %s, name: %s", d.Name, d.Namespace, name)
		// During an active upgrade (PreUpgradeSnapshot exists), pods may
		// transiently appear ready. Cap status at Pending when snapshot exists
		// to prevent false positive Succeeded state during rolling updates.
		if newStatus.State == constants.Succeeded {
			annotations := csm.GetAnnotations()
			_, snapshotExists := annotations[preUpgradeSnapshotKey]
			if snapshotExists {
				log.Infoln("Setting status (daemonset) to Pending due to PreUpgradeSnapshot")
				newStatus.State = constants.Pending
			}
		}

		result, _ := operatorutils.UpdateStatus(ctx, csm, r, newStatus, r.Config)
		crKey := csm.GetNamespace() + "/" + csm.GetName()
		if result.RequeueAfter > 0 && operatorutils.IsStabilityPeriodPending(crKey) {
			r.scheduleStatusRecheck(namespacedName, result.RequeueAfter)
		}

		if newStatus.State == constants.Succeeded {
			r.EventRecorder.Eventf(csm, corev1.EventTypeNormal, csmv1.EventCompleted, "Daemonset %s running OK", d.Name)
		}
	}
}

// ContentWatch - watch updates on deployments, deamonsets, and pods
func (r *ContainerStorageModuleReconciler) ContentWatch(csm *csmv1.ContainerStorageModule) (chan struct{}, error) {
	_, log := logger.GetNewContextWithLogger("ContentWatch")
	refreshMinutes, err := operatorutils.GetEnvironmentVariable(RefreshEnvVar)
	if err != nil {
		log.Info("Refresh time environment variable not set, defaulting to 60 minutes")
		refreshMinutes = "60"
	}
	refreshMinutesInt, err := strconv.Atoi(refreshMinutes)
	if err != nil {
		log.Error("Refresh time environment variable not a valid number, defaulting to 60 minutes")
		refreshMinutesInt = 60
	} else {
		log.Info("Refresh time for ContentWatch set to ", refreshMinutesInt, " minutes")
	}
	refreshTime := time.Duration(refreshMinutesInt) * time.Minute
	sharedInformerFactory := sinformer.NewSharedInformerFactoryWithOptions(r.K8sClient, time.Duration(refreshTime))

	updateFn := func(oldObj interface{}, newObj interface{}) {
		r.informerUpdate(csm, oldObj, newObj, r.handleDaemonsetUpdate, r.handleDeploymentUpdate, r.handlePodsUpdate)
	}

	daemonsetInformer := sharedInformerFactory.Apps().V1().DaemonSets().Informer()
	_, err = daemonsetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: updateFn,
	})
	if err != nil {
		return nil, fmt.Errorf("ContentWatch failed adding event handler to daemonsetInformer: %v", err)
	}

	deploymentInformer := sharedInformerFactory.Apps().V1().Deployments().Informer()
	_, err = deploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: updateFn,
	})
	if err != nil {
		return nil, fmt.Errorf("ContentWatch failed adding event handler to deploymentInformer: %v", err)
	}

	podsInformer := sharedInformerFactory.Core().V1().Pods().Informer()
	_, err = podsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: updateFn,
	})
	if err != nil {
		return nil, fmt.Errorf("ContentWatch failed adding event handler to podsInformer: %v", err)
	}

	stopCh := make(chan struct{})
	sharedInformerFactory.Start(stopCh)

	// UT only: don't block on sync to avoid bookmark requirement
	if os.Getenv("UNIT_TEST") == "true" {
		return stopCh, nil
	}
	sharedInformerFactory.WaitForCacheSync(stopCh)

	return stopCh, nil
}

// scheduleStatusRecheck schedules a delayed status recheck for a CSM object.
// ContentWatch handlers cannot act on reconcile.Result (the return value is
// discarded), so when the succeeded stability period has not yet elapsed,
// this method schedules a time.AfterFunc that re-fetches the CSM and calls
// UpdateStatus again. If the follow-up still returns RequeueAfter and the
// stability period is still pending, another recheck is scheduled, creating
// a self-sustaining retry loop until the stability period elapses.
//
// This is a no-op if a timer is already active for this CSM, avoiding timer
// accumulation from rapid event sequences.
func (r *ContainerStorageModuleReconciler) scheduleStatusRecheck(namespacedName t1.NamespacedName, delay time.Duration) {
	key := namespacedName.String()

	// If a timer already exists for this CSM, let it handle the recheck.
	// This prevents timer accumulation from rapid deployment/pod events.
	if _, loaded := pendingRechecks.Load(key); loaded {
		return
	}

	timer := time.AfterFunc(delay, func() {
		pendingRechecks.Delete(key)

		recheckKey := namespacedName.Name + "-recheck"
		ctx, log := logger.GetNewContextWithLogger(recheckKey)

		dMutex.Lock()
		defer dMutex.Unlock()

		csm := new(csmv1.ContainerStorageModule)
		if err := r.Client.Get(ctx, namespacedName, csm); err != nil {
			log.Debugw("status recheck: CSM not found, skipping", "error", err)
			return
		}

		log.Infof("status recheck: recalculating state for %s", key)
		newStatus := csm.GetCSMStatus()
		result, _ := operatorutils.UpdateStatus(ctx, csm, r, newStatus, r.Config)

		// If still pending due to stability period, schedule another recheck
		crKey := csm.GetNamespace() + "/" + csm.GetName()
		if result.RequeueAfter > 0 && operatorutils.IsStabilityPeriodPending(crKey) {
			log.Infof("status recheck: stability period still pending, scheduling another recheck in %v", result.RequeueAfter)
			r.scheduleStatusRecheck(namespacedName, result.RequeueAfter)
		}
	})

	pendingRechecks.Store(key, timer)
}

func (r *ContainerStorageModuleReconciler) informerUpdate(csm *csmv1.ContainerStorageModule, oldObj interface{}, newObj interface{},
	handleDaemonsetUpdate func(oldObj interface{}, obj interface{}),
	handleDeploymentUpdate func(oldObj interface{}, obj interface{}),
	handlePodsUpdate func(oldObj interface{}, obj interface{}),
) {
	// extract csm labels from object
	// if labels are not present, don't specify a CSM, or CSM name does not match passed in CSM, do nothing
	// else, proceed to update CSM state
	var csmName, csmNamespace string
	var nameOk, namespaceOk bool
	var fn func(oldObj interface{}, newObj interface{})
	switch v := oldObj.(type) {
	case *appsv1.DaemonSet:
		csmName, nameOk = v.Spec.Template.Labels[constants.CsmLabel]
		csmNamespace, namespaceOk = v.Spec.Template.Labels[constants.CsmNamespaceLabel]
		fn = handleDaemonsetUpdate
	case *appsv1.Deployment:
		csmName, nameOk = v.Spec.Template.Labels[constants.CsmLabel]
		csmNamespace, namespaceOk = v.Spec.Template.Labels[constants.CsmNamespaceLabel]
		fn = handleDeploymentUpdate
	case *corev1.Pod:
		csmName, nameOk = v.GetLabels()[constants.CsmLabel]
		csmNamespace, namespaceOk = v.GetLabels()[constants.CsmNamespaceLabel]
		fn = handlePodsUpdate
	default:
		return
	}

	if (!nameOk || !namespaceOk) || (csmName == "" || csmNamespace == "") || (csmName != csm.Name) {
		return
	}

	fn(oldObj, newObj)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ContainerStorageModuleReconciler) SetupWithManager(mgr ctrl.Manager, limiter workqueue.TypedRateLimiter[reconcile.Request], maxReconcilers int) error {
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
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latestCSM := &csmv1.ContainerStorageModule{}
		if err := r.GetClient().Get(ctx, t1.NamespacedName{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		}, latestCSM); err != nil {
			return err
		}
		if !latestCSM.HasFinalizer(CSMFinalizerName) {
			return nil
		}
		latestCSM.SetFinalizers(nil)
		return r.Update(ctx, latestCSM)
	})
}

func (r *ContainerStorageModuleReconciler) addFinalizer(ctx context.Context, instance *csmv1.ContainerStorageModule) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latestCSM := &csmv1.ContainerStorageModule{}
		if err := r.GetClient().Get(ctx, t1.NamespacedName{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		}, latestCSM); err != nil {
			return err
		}
		latestCSM.SetFinalizers([]string{CSMFinalizerName})
		latestCSM.GetCSMStatus().State = constants.Creating
		return r.Update(ctx, latestCSM)
	})
}

func (r *ContainerStorageModuleReconciler) oldStandAloneModuleCleanup(ctx context.Context, newCR *csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, _ *DriverConfig) error {
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
			clusterClient := operatorutils.GetCluster(ctx, r)
			if err != nil {
				return err
			}
			log.Infow("Deleting Replication controller", "clusterID:", clusterClient.ClusterID)
			if err = modules.ReplicationManagerController(ctx, true, operatorConfig, *oldCR, clusterClient.ClusterCTRLClient); err != nil {
				return err
			}
			log.Infow("Deleting Replication CRDs", "clusterID:", clusterClient)
			if err = modules.DeleteReplicationCrds(ctx, operatorConfig, *oldCR, clusterClient.ClusterCTRLClient); err != nil {
				log.Warnf("Failed to delete replication CRDs: %v", err)
			}
		}
		// check if observability needs to be uninstalled
		oldObservabilityEnabled, oldObs := operatorutils.IsModuleEnabled(ctx, *oldCR, csmv1.Observability)
		newObservabilityEnabled, _ := operatorutils.IsModuleEnabled(ctx, *newCR, csmv1.Observability)
		// check if observability components need to be uninstalled
		components := []string{}
		if oldObservabilityEnabled && newObservabilityEnabled {
			for _, comp := range oldObs.Components {
				oldCompEnabled := operatorutils.IsModuleComponentEnabled(ctx, *oldCR, csmv1.Observability, comp.Name)
				newCompEnabled := operatorutils.IsModuleComponentEnabled(ctx, *newCR, csmv1.Observability, comp.Name)
				if oldCompEnabled && !newCompEnabled {
					components = append(components, comp.Name)
				}
			}
		}
		if (oldObservabilityEnabled && !newObservabilityEnabled) || len(components) > 0 {
			clusterClient := operatorutils.GetCluster(ctx, r)

			// remove module observability
			log.Infow("Deleting observability")
			if err = r.reconcileObservability(ctx, true, operatorConfig, *oldCR, components, clusterClient.ClusterCTRLClient, clusterClient.ClusterK8sClient, operatorutils.VersionSpec{}); err != nil {
				return err
			}

		}
	}

	return r.savePreviouslyAppliedConfig(ctx, newCR)
}

// savePreviouslyAppliedConfig persists the current CR spec as the
// previouslyAppliedCustomResource annotation. This annotation is read by
// savePreUpgradeSnapshot to build a rollback snapshot before upgrades.
func (r *ContainerStorageModuleReconciler) savePreviouslyAppliedConfig(ctx context.Context, cr *csmv1.ContainerStorageModule) error {
	copyCR := cr.DeepCopy()
	delete(copyCR.Annotations, previouslyAppliedCustomResource)
	delete(copyCR.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
	copyCR.ManagedFields = nil
	copyCR.Status = csmv1.ContainerStorageModuleStatus{}
	out, err := json.Marshal(copyCR)
	if err != nil {
		return fmt.Errorf("error marshalling CR to annotation: %v", err)
	}
	annotationValue := string(out)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latestCSM := &csmv1.ContainerStorageModule{}
		if err := r.GetClient().Get(ctx, t1.NamespacedName{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		}, latestCSM); err != nil {
			return err
		}
		if latestCSM.Annotations == nil {
			latestCSM.Annotations = make(map[string]string)
		}
		latestCSM.Annotations[previouslyAppliedCustomResource] = annotationValue
		return r.GetClient().Update(ctx, latestCSM)
	})
}

// metroSiteFailureHandlingEnabled returns the canonical Metro enablement value
// used by CRD provisioning and driver manifest generation.
func metroSiteFailureHandlingEnabled(cr csmv1.ContainerStorageModule) string {
	if cr.Spec.Driver.MetroSiteFailureHandling != nil {
		return strconv.FormatBool(cr.Spec.Driver.MetroSiteFailureHandling.Enabled)
	}
	return drivers.GetDriverCommonEnv(cr, "X_CSI_POWERMAX_METRO_SITE_FAILURE_HANDLING_ENABLED", "false")
}

// SyncCSM - Sync the current installation - this can lead to a create or update
func (r *ContainerStorageModuleReconciler) SyncCSM(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	// Install/update via configmap
	var matched operatorutils.VersionSpec
	if cr.Spec.Version != "" {
		var err error
		matched, err = operatorutils.ResolveVersionFromConfigMap(ctx, ctrlClient, &cr)
		if err != nil {
			log.Error(err, "Failed to get version from configmap")
			return err
		}
	}

	// Create/Update Authorization Proxy Server
	authorizationEnabled, _ := operatorutils.IsModuleEnabled(ctx, cr, csmv1.AuthorizationServer)
	if authorizationEnabled {
		log.Infow("Create/Update authorization")
		if err := r.reconcileAuthorizationCRDS(ctx, operatorConfig, cr, ctrlClient); err != nil {
			return fmt.Errorf("failed to deploy authorization proxy server: %v", err)
		}
		if err := r.reconcileAuthorization(ctx, false, operatorConfig, cr, ctrlClient, matched); err != nil {
			return fmt.Errorf("failed to deploy authorization proxy server: %v", err)
		}
		if err := r.savePreviouslyAppliedConfig(ctx, &cr); err != nil {
			return err
		}
		return nil
	}

	if !authorizationEnabled {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "proxy-server-metrics", Namespace: cr.Namespace},
		}
		if err := operatorutils.DeleteObject(ctx, svc, ctrlClient); err != nil {
			log.Warnw("Failed to delete authorization metrics Service (may not exist)", "error", err)
		}
		sm := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "monitoring.coreos.com/v1",
			"kind":       "ServiceMonitor",
			"metadata": map[string]interface{}{
				"name":      "proxy-server-metrics-monitor",
				"namespace": cr.Namespace,
			},
		}}
		if err := operatorutils.DeleteObject(ctx, sm, ctrlClient); err != nil {
			log.Warnw("Failed to delete authorization metrics ServiceMonitor (may not exist)", "error", err)
		}
	}

	// Create/Update Reverseproxy Server
	if reverseProxyEnabled, _ := operatorutils.IsModuleEnabled(ctx, cr, csmv1.ReverseProxy); reverseProxyEnabled && !modules.IsReverseProxySidecar() {
		log.Infow("Trying Create/Update reverseproxy...")
		if err := r.reconcileReverseProxyServer(ctx, false, operatorConfig, cr, ctrlClient); err != nil {
			return fmt.Errorf("failed to deploy reverseproxy proxy server: %v", err)
		}
	}

	// Install/update the Replication CRDs
	if replicationEnabled, _ := operatorutils.IsModuleEnabled(ctx, cr, csmv1.Replication); replicationEnabled {
		log.Infow("Create/Update Replication CRDs")
		if err := r.reconcileReplicationCRDS(ctx, operatorConfig, cr, ctrlClient); err != nil {
			return fmt.Errorf("failed to deploy replication CRDs: %v", err)
		}
	}

	// Get Driver resources
	driverConfig, err := getDriverConfig(ctx, cr, operatorConfig, ctrlClient, matched)
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

	if cr.GetDriverType() == csmv1.PowerMax {
		if !modules.IsReverseProxySidecar() {
			log.Infof("DeployAsSidar is false...csi-reverseproxy should be present as deployment\n")
			log.Infof("adding proxy service name...\n")
			modules.AddReverseProxyServiceName(&controller.Deployment)

			// Set the secret mount for powermax controller.
			// Note: No need to catch error since it only returns one if the interface casting fails which it shouldn't here.
			_ = drivers.DynamicallyMountPowermaxContent(&controller.Deployment, cr)
		} else {
			log.Info("Starting CSI ReverseProxy Service")
			if err := modules.ReverseProxyStartService(ctx, false, operatorConfig, cr, ctrlClient); err != nil {
				return fmt.Errorf("unable to reconcile reverse-proxy service: %v", err)
			}
			log.Info("Injecting CSI ReverseProxy into controller deployment")
			dp, err := modules.ReverseProxyInjectDeployment(ctx, controller.Deployment, cr, operatorConfig, matched)
			if err != nil {
				return fmt.Errorf("unable to inject ReverseProxy into deployment: %v", err)
			}

			controller.Deployment = *dp
		}

		// Set the secret mount for powermax node.
		// Note: No need to catch error since it only returns one if the interface casting fails which it shouldn't here.
		_ = drivers.DynamicallyMountPowermaxContent(&node.DaemonSetApplyConfig, cr)

		// Dynamically update the drivers config param.
		modules.UpdatePowerMaxConfigMap(configMap, cr)
	}

	isHarvester, err := k8s.IsHarvester()
	if err != nil {
		return fmt.Errorf("failed to detect harvester cluster: %v", err)
	}

	if cr.GetDriverType() == csmv1.PowerFlex {
		// if driver is powerflex and installing on openshift, we must remove the root host path, since it is read only
		// Mount is removed in CSM 1.17 but kept for backwards compatibility
		// Remove this code when CSM 1.16 is no longer supported
		if r.Config.IsOpenShift || isHarvester {
			_ = drivers.RemoveVolume(&node.DaemonSetApplyConfig, drivers.ScaleioBinPath)
		}
		if (cr.Spec.Driver.Node != nil) && cr.Spec.Driver.Node.Envs != nil {
			for _, env := range cr.Spec.Driver.Node.Envs {
				if env.Name == "X_CSI_SDC_SFTP_REPO_ENABLED" {
					if env.Value != "true" {
						_ = drivers.RemoveInitVolume(&node.DaemonSetApplyConfig, drivers.SftpKeys)
					}
					break
				}
			}
		} else {
			// if envs are not specified, we assume that sftp is disabled
			_ = drivers.RemoveInitVolume(&node.DaemonSetApplyConfig, drivers.SftpKeys)
		}
	}

	clusterClient := operatorutils.GetCluster(ctx, r)
	replicationEnabled, _ := operatorutils.IsModuleEnabled(ctx, cr, csmv1.Replication)

	for _, m := range cr.Spec.Modules {
		if m.Enabled {
			switch m.Name {
			case csmv1.Authorization:
				log.Info("Injecting CSM Authorization")
				dp, err := modules.AuthInjectDeployment(ctx, controller.Deployment, cr, operatorConfig, ctrlClient)
				if err != nil {
					return fmt.Errorf("injecting auth into deployment: %v", err)
				}
				controller.Deployment = *dp

				ds, err := modules.AuthInjectDaemonset(ctx, node.DaemonSetApplyConfig, cr, operatorConfig, ctrlClient)
				if err != nil {
					return fmt.Errorf("injecting auth into deamonset: %v", err)
				}

				node.DaemonSetApplyConfig = *ds
			case csmv1.Resiliency:
				log.Info("Injecting CSM Resiliency")

				// for controller-pod
				driverName := string(cr.Spec.Driver.CSIDriverType)
				dp, err := modules.ResiliencyInjectDeployment(ctx, controller.Deployment, cr, operatorConfig, driverName, matched)
				if err != nil {
					return fmt.Errorf("injecting resiliency into deployment: %v", err)
				}
				controller.Deployment = *dp

				// Injecting clusterroles
				clusterRole, err := modules.ResiliencyInjectClusterRole(ctx, controller.Rbac.ClusterRole, cr, operatorConfig, "controller")
				if err != nil {
					return fmt.Errorf("injecting resiliency into controller cluster role: %v", err)
				}

				controller.Rbac.ClusterRole = *clusterRole

				// Injecting roles
				role, err := modules.ResiliencyInjectRole(ctx, controller.Rbac.Role, cr, operatorConfig, "controller")
				if err != nil {
					return fmt.Errorf("injecting resiliency into controller role: %v", err)
				}

				controller.Rbac.Role = *role

				// for node-pod
				ds, err := modules.ResiliencyInjectDaemonset(ctx, node.DaemonSetApplyConfig, cr, operatorConfig, driverName, matched)
				if err != nil {
					return fmt.Errorf("injecting resiliency into daemonset: %v", err)
				}
				node.DaemonSetApplyConfig = *ds

				// Injecting clusterroles
				clusterRoleForNode, err := modules.ResiliencyInjectClusterRole(ctx, node.Rbac.ClusterRole, cr, operatorConfig, "node")
				if err != nil {
					return fmt.Errorf("injecting resiliency into node cluster role: %v", err)
				}

				node.Rbac.ClusterRole = *clusterRoleForNode

				// Injecting roles
				roleForNode, err := modules.ResiliencyInjectRole(ctx, node.Rbac.Role, cr, operatorConfig, "node")
				if err != nil {
					return fmt.Errorf("injecting resiliency into controller role: %v", err)
				}

				node.Rbac.Role = *roleForNode

			case csmv1.Replication:
				// This function adds replication sidecar to driver pods.
				log.Info("Injecting CSM Replication")
				dp, err := modules.ReplicationInjectDeployment(ctx, controller.Deployment, cr, operatorConfig, matched)
				if err != nil {
					return fmt.Errorf("injecting replication into deployment: %v", err)
				}
				controller.Deployment = *dp

				clusterRole, err := modules.ReplicationInjectClusterRole(ctx, controller.Rbac.ClusterRole, cr, operatorConfig)
				if err != nil {
					return fmt.Errorf("injecting replication into controller cluster role: %v", err)
				}

				controller.Rbac.ClusterRole = *clusterRole

			}
		}
	}

	// Check if CSI Addons should be injected via environment variable (without requiring module in CR spec)
	if modules.IsCSIAddonsReplicationEnabledViaEnv(cr) {
		log.Info("Injecting CSI Addons Replication (enabled via environment variable)")
		dp, err := modules.CSIAddonsReplicationInjectDeployment(ctx, controller.Deployment, cr, operatorConfig)
		if err != nil {
			return fmt.Errorf("injecting csi-addons replication into deployment: %v", err)
		}
		controller.Deployment = *dp

		clusterRole, err := modules.CSIAddonsReplicationInjectClusterRole(ctx, controller.Rbac.ClusterRole, cr, operatorConfig)
		if err != nil {
			return fmt.Errorf("injecting csi-addons replication into controller cluster role: %v", err)
		}

		controller.Rbac.ClusterRole = *clusterRole
	}

	log.Infof("Starting SYNC for %s cluster", clusterClient.ClusterID)
	if cr.GetDriverType() == csmv1.Cosi {
		if err = serviceaccount.SyncServiceAccount(ctx, controller.Rbac.ServiceAccount, clusterClient.ClusterCTRLClient); err != nil {
			return err
		}
		if err = rbac.SyncClusterRole(ctx, controller.Rbac.ClusterRole, clusterClient.ClusterCTRLClient); err != nil {
			return err
		}
		if err = rbac.SyncClusterRoleBindings(ctx, controller.Rbac.ClusterRoleBinding, clusterClient.ClusterCTRLClient); err != nil {
			return err
		}
		if err = configmap.SyncConfigMap(ctx, *configMap, clusterClient.ClusterCTRLClient); err != nil {
			return err
		}
		if err = deployment.SyncDeployment(ctx, controller.Deployment, clusterClient.ClusterK8sClient, cr.Name); err != nil {
			return err
		}
		return nil
	}

	// Create/Update ServiceAccount
	if err = serviceaccount.SyncServiceAccount(ctx, node.Rbac.ServiceAccount, clusterClient.ClusterCTRLClient); err != nil {
		return err
	}

	if err = serviceaccount.SyncServiceAccount(ctx, controller.Rbac.ServiceAccount, clusterClient.ClusterCTRLClient); err != nil {
		return err
	}

	// Create/Update ClusterRoles
	if err = rbac.SyncClusterRole(ctx, node.Rbac.ClusterRole, clusterClient.ClusterCTRLClient); err != nil {
		return err
	}

	if err = rbac.SyncClusterRole(ctx, controller.Rbac.ClusterRole, clusterClient.ClusterCTRLClient); err != nil {
		return err
	}

	// Create/Update ClusterRoleBinding
	if err = rbac.SyncClusterRoleBindings(ctx, node.Rbac.ClusterRoleBinding, clusterClient.ClusterCTRLClient); err != nil {
		return err
	}

	if err = rbac.SyncClusterRoleBindings(ctx, controller.Rbac.ClusterRoleBinding, clusterClient.ClusterCTRLClient); err != nil {
		return err
	}

	// Create/Update Roles
	if err = rbac.SyncRole(ctx, node.Rbac.Role, clusterClient.ClusterCTRLClient); err != nil {
		return err
	}

	if err = rbac.SyncRole(ctx, controller.Rbac.Role, clusterClient.ClusterCTRLClient); err != nil {
		return err
	}

	// Create/Update RoleBinding
	if err = rbac.SyncRoleBindings(ctx, node.Rbac.RoleBinding, clusterClient.ClusterCTRLClient); err != nil {
		return err
	}

	if err = rbac.SyncRoleBindings(ctx, controller.Rbac.RoleBinding, clusterClient.ClusterCTRLClient); err != nil {
		return err
	}

	// Create/Update CSIDriver
	if err = csidriver.SyncCSIDriver(ctx, *driver, clusterClient.ClusterCTRLClient); err != nil {
		return err
	}

	// Create/Update ConfigMap
	if err = configmap.SyncConfigMap(ctx, *configMap, clusterClient.ClusterCTRLClient); err != nil {
		return err
	}

	// Create/Update Deployment
	if err = deployment.SyncDeployment(ctx, controller.Deployment, clusterClient.ClusterK8sClient, cr.Name); err != nil {
		return err
	}

	enableCSMDR := drivers.GetDriverCommonEnv(cr, "X_CSM_DR_ENABLED", "true")
	log.Infow(fmt.Sprintf("Is CSM DR enabled: %s", enableCSMDR))
	// Create/Update CSM DR CRD for PowerStore
	if cr.GetDriverType() == csmv1.PowerStore && enableCSMDR == "true" {
		err = applyCSMDRCRD(ctx, cr, false, operatorConfig, clusterClient.ClusterCTRLClient)
		if err != nil {
			return err
		}
	}

	// Create/Update VolumeJournal CRD for PowerMax Metro site-failure handling.
	// The CRD must exist before the driver starts; without it every deferred
	// operation falls back to the in-memory journal and is lost on pod restart.
	enableMetro := metroSiteFailureHandlingEnabled(cr)
	log.Infow(fmt.Sprintf("Is PowerMax Metro site-failure handling enabled: %s", enableMetro))
	if cr.GetDriverType() == csmv1.PowerMax && enableMetro == "true" {
		if err = applyVolumeJournalCRD(ctx, cr, false, operatorConfig, clusterClient.ClusterCTRLClient); err != nil {
			return err
		}
	}

	// Create/Update DeamonSet, except for auth proxy
	if !authorizationEnabled {
		if err = daemonset.SyncDaemonset(ctx, node.DaemonSetApplyConfig, clusterClient.ClusterK8sClient, cr.Name); err != nil {
			return err
		}
	}

	if replicationEnabled {
		// This will also create the dell-replication-controller namespace.
		if err = modules.ReplicationManagerController(ctx, false, operatorConfig, cr, clusterClient.ClusterCTRLClient); err != nil {
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
	if observabilityEnabled, _ := operatorutils.IsModuleEnabled(ctx, cr, csmv1.Observability); observabilityEnabled {
		log.Infow("Create/Update observability")

		if err = r.reconcileObservability(ctx, false, operatorConfig, cr, nil, clusterClient.ClusterCTRLClient, clusterClient.ClusterK8sClient, matched); err != nil {
			return err
		}
	}

	// Sync metrics Service and ServiceMonitor resources for supported drivers.
	if operatorutils.SupportsDriverMetrics(cr.GetDriverType()) {
		if err = syncMetricsResources(ctx, false, cr, operatorConfig, clusterClient.ClusterCTRLClient); err != nil {
			return err
		}
	}

	// Sync module metrics resources (e.g., for resiliency podmon, replication)
	resiliencyInSpec := false
	replicationInSpec := false
	for _, module := range cr.Spec.Modules {
		if module.Name == csmv1.Resiliency {
			resiliencyInSpec = true
			if err := syncModuleMetricsResources(ctx, false, module, cr, clusterClient.ClusterCTRLClient); err != nil {
				log.Errorw("Failed to sync module metrics resources", "module", module.Name, "error", err)
				return err
			}
			if err := syncResiliencyPrometheusRule(ctx, false, module, cr, operatorConfig, clusterClient.ClusterCTRLClient); err != nil {
				log.Errorw("Failed to sync resiliency PrometheusRule", "error", err)
				return err
			}
		}
		if module.Name == csmv1.Replication {
			replicationInSpec = true
			if err := syncModuleMetricsResources(ctx, false, module, cr, clusterClient.ClusterCTRLClient); err != nil {
				log.Errorw("Failed to sync module metrics resources", "module", module.Name, "error", err)
				return err
			}
			if err := syncReplicationPrometheusRule(ctx, false, module, cr, operatorConfig, clusterClient.ClusterCTRLClient); err != nil {
				log.Errorw("Failed to sync replication PrometheusRule", "error", err)
				return err
			}
		}
	}

	// Clean up resiliency metrics resources if resiliency module was removed from spec
	if !resiliencyInSpec {
		svcName := cr.Name + "-resiliency-metrics"
		smName := cr.Name + "-resiliency-metrics"
		pmName := cr.Name + "-resiliency-metrics"

		// Delete Service
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svcName,
				Namespace: cr.Namespace,
			},
		}
		if err := operatorutils.DeleteObject(ctx, svc, clusterClient.ClusterCTRLClient); err != nil {
			log.Warnw("Failed to delete resiliency metrics Service (may not exist)", "name", svcName, "error", err)
		}

		// Delete ServiceMonitor
		sm := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "monitoring.coreos.com/v1",
				"kind":       "ServiceMonitor",
				"metadata": map[string]interface{}{
					"name":      smName,
					"namespace": cr.Namespace,
				},
			},
		}
		if err := operatorutils.DeleteObject(ctx, sm, clusterClient.ClusterCTRLClient); err != nil {
			log.Warnw("Failed to delete resiliency metrics ServiceMonitor (may not exist)", "name", smName, "error", err)
		}

		// Delete PodMonitor
		pm := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "monitoring.coreos.com/v1",
				"kind":       "PodMonitor",
				"metadata": map[string]interface{}{
					"name":      pmName,
					"namespace": cr.Namespace,
				},
			},
		}
		if err := operatorutils.DeleteObject(ctx, pm, clusterClient.ClusterCTRLClient); err != nil {
			log.Warnw("Failed to delete resiliency metrics PodMonitor (may not exist)", "name", pmName, "error", err)
		}

		// Delete PrometheusRule
		prName := resiliencyPrometheusRuleName(cr.Name)
		pr := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "monitoring.coreos.com/v1",
				"kind":       "PrometheusRule",
				"metadata": map[string]interface{}{
					"name":      prName,
					"namespace": cr.Namespace,
				},
			},
		}
		if err := operatorutils.DeleteObject(ctx, pr, clusterClient.ClusterCTRLClient); err != nil {
			log.Warnw("Failed to delete resiliency PrometheusRule (may not exist)", "name", prName, "error", err)
		}
	}

	// Clean up replication metrics resources if replication module was removed from spec
	if !replicationInSpec {
		svcName := cr.Name + "-replication-metrics"

		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svcName,
				Namespace: cr.Namespace,
			},
		}
		if err := operatorutils.DeleteObject(ctx, svc, clusterClient.ClusterCTRLClient); err != nil {
			log.Warnw("Failed to delete replication metrics Service (may not exist)", "name", svcName, "error", err)
		}

		sm := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "monitoring.coreos.com/v1",
				"kind":       "ServiceMonitor",
				"metadata": map[string]interface{}{
					"name":      svcName,
					"namespace": cr.Namespace,
				},
			},
		}
		if err := operatorutils.DeleteObject(ctx, sm, clusterClient.ClusterCTRLClient); err != nil {
			log.Warnw("Failed to delete replication metrics ServiceMonitor (may not exist)", "name", svcName, "error", err)
		}

		pm := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "monitoring.coreos.com/v1",
				"kind":       "PodMonitor",
				"metadata": map[string]interface{}{
					"name":      svcName,
					"namespace": cr.Namespace,
				},
			},
		}
		if err := operatorutils.DeleteObject(ctx, pm, clusterClient.ClusterCTRLClient); err != nil {
			log.Debugw("Replication metrics PodMonitor not present (may not have been created)", "name", svcName, "error", err)
		}

		if err := syncReplicationPrometheusRule(ctx, true, csmv1.Module{Name: csmv1.Replication}, cr, operatorConfig, clusterClient.ClusterCTRLClient); err != nil {
			log.Warnw("Failed to clean up replication PrometheusRule", "error", err)
		}
	}

	return nil
}

// reconcileObservability - Delete/Create/Update observability components
// isDeleting - true: Delete; false: Create/Update
func (r *ContainerStorageModuleReconciler) reconcileObservability(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, components []string, ctrlClient client.Client, k8sClient kubernetes.Interface, matched operatorutils.VersionSpec) error {
	log := logger.GetLogger(ctx)

	configVersion, err := operatorutils.GetVersion(ctx, &cr, op)
	if err != nil {
		return err
	}
	// if components is empty, reconcile all enabled components
	if len(components) == 0 {
		if enabled, obs := operatorutils.IsModuleEnabled(ctx, cr, csmv1.Observability); enabled {
			for _, comp := range obs.Components {
				if operatorutils.IsModuleComponentEnabled(ctx, cr, csmv1.Observability, comp.Name) {
					components = append(components, comp.Name)
				}
			}
		}
	}
	comp2reconFunc := map[string]func(context.Context, bool, operatorutils.OperatorConfig, csmv1.ContainerStorageModule, client.Client, operatorutils.VersionSpec) error{
		modules.ObservabilityOtelCollectorName:    modules.OtelCollector,
		modules.ObservabilityCertManagerComponent: modules.CommonCertManager,
	}
	// This will be deleted once we remove the old CSM versions which support topology
	if strings.Contains(configVersion, "v2.13") || strings.Contains(configVersion, "v2.14") {
		comp2reconFunc[modules.ObservabilityTopologyName] = modules.ObservabilityTopology
	}
	metricsComp2reconFunc := map[string]func(context.Context, bool, operatorutils.OperatorConfig, csmv1.ContainerStorageModule, client.Client, kubernetes.Interface) error{
		modules.ObservabilityMetricsPowerScaleName: modules.PowerScaleMetrics,
		modules.ObservabilityMetricsPowerFlexName:  modules.PowerFlexMetrics,
		modules.ObservabilityMetricsPowerMaxName:   modules.PowerMaxMetrics,
		modules.ObservabilityMetricsPowerStoreName: modules.PowerStoreMetrics,
	}

	for _, comp := range components {
		log.Infow(fmt.Sprintf("reconcile %s", comp))
		var err error
		switch comp {
		case modules.ObservabilityOtelCollectorName, modules.ObservabilityCertManagerComponent:
			err = comp2reconFunc[comp](ctx, isDeleting, op, cr, ctrlClient, matched)
		// This will be deleted once we remove the old CSM versions which support topology
		case modules.ObservabilityTopologyName:
			if strings.Contains(configVersion, "v2.13") || strings.Contains(configVersion, "v2.14") {
				err = comp2reconFunc[comp](ctx, isDeleting, op, cr, ctrlClient, matched)
			}
		case modules.ObservabilityMetricsPowerScaleName, modules.ObservabilityMetricsPowerFlexName, modules.ObservabilityMetricsPowerMaxName, modules.ObservabilityMetricsPowerStoreName:
			err = metricsComp2reconFunc[comp](ctx, isDeleting, op, cr, ctrlClient, k8sClient)
		default:
			err = fmt.Errorf("unsupported component type: %v", comp)
		}
		if err != nil {
			log.Errorf("failed to reconcile %s", comp)
			return err
		}
	}

	// We are doing this separately after creating other components because
	// the certificates rely on cert-manager being up.  When cert-manager is
	// enabled we must wait for the webhook deployment to become ready;
	// otherwise the API server rejects Certificate/Issuer creation because
	// the webhook (failurePolicy: Fail) is unreachable.
	if !isDeleting && operatorutils.IsModuleComponentEnabled(ctx, cr, csmv1.Observability, modules.ObservabilityCertManagerComponent) {
		webhookDep := &appsv1.Deployment{}
		depKey := t1.NamespacedName{Name: "cert-manager-webhook", Namespace: cr.Namespace}
		if err := ctrlClient.Get(ctx, depKey, webhookDep); err != nil {
			return fmt.Errorf("cert-manager-webhook deployment not found, will retry: %v", err)
		}
		if webhookDep.Status.ReadyReplicas < 1 {
			return fmt.Errorf("cert-manager-webhook is not ready yet (ready=%d), will retry", webhookDep.Status.ReadyReplicas)
		}

		// Also check that the webhook service has ready endpoints to ensure
		// the webhook is actually reachable, not just the deployment is ready
		// nolint:staticcheck // SA1019: corev1.Endpoints is deprecated but still used for webhook endpoint checks
		webhookEndpoints := &corev1.Endpoints{}
		endpointsKey := t1.NamespacedName{Name: "cert-manager-webhook", Namespace: cr.Namespace}
		if err := ctrlClient.Get(ctx, endpointsKey, webhookEndpoints); err != nil {
			return fmt.Errorf("cert-manager-webhook endpoints not found, will retry: %v", err)
		}
		readyAddresses := 0
		for _, subset := range webhookEndpoints.Subsets {
			readyAddresses += len(subset.Addresses)
		}
		if readyAddresses < 1 {
			return fmt.Errorf("cert-manager-webhook has no ready endpoints (ready=%d), will retry", readyAddresses)
		}

		// Add a small delay to ensure the webhook is fully initialized and ready
		// to handle requests. This helps avoid race conditions where the webhook
		// is running but not yet ready to validate Certificate/Issuer resources.
		log.Infow("cert-manager-webhook is ready, waiting for initialization")
		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if err := modules.IssuerCertServiceObs(ctx, isDeleting, op, cr, ctrlClient); err != nil {
		return fmt.Errorf("unable to deploy Certificate & Issuer for Observability: %v", err)
	}

	if err := syncObservabilityPrometheusRule(ctx, isDeleting, op, cr, ctrlClient); err != nil {
		return fmt.Errorf("unable to sync observability alert rules: %v", err)
	}

	return nil
}

// reconcileAuthorization - deploy authorization proxy server
func (r *ContainerStorageModuleReconciler) reconcileAuthorization(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client, matched operatorutils.VersionSpec) error {
	log := logger.GetLogger(ctx)

	if operatorutils.IsModuleComponentEnabled(ctx, cr, csmv1.AuthorizationServer, modules.AuthCertManagerComponent) {
		log.Infow("Reconcile authorization cert-manager")
		if err := modules.CommonCertManager(ctx, isDeleting, op, cr, ctrlClient, matched); err != nil {
			return fmt.Errorf("unable to reconcile cert-manager for authorization: %v", err)
		}
	}

	if operatorutils.IsModuleComponentEnabled(ctx, cr, csmv1.AuthorizationServer, modules.AuthProxyServerComponent) {
		log.Infow("Reconcile authorization proxy-server")
		if err := modules.AuthorizationServerDeployment(ctx, isDeleting, op, cr, ctrlClient, matched); err != nil {
			return fmt.Errorf("unable to reconcile authorization proxy server: %v", err)
		}

		for _, m := range cr.Spec.Modules {
			if m.Name == csmv1.AuthorizationServer {
				if err := syncModuleMetricsResources(ctx, isDeleting, m, cr, ctrlClient); err != nil {
					return fmt.Errorf("unable to sync authorization metrics resources: %v", err)
				}
				if err := syncAuthorizationPrometheusRule(ctx, isDeleting, m, cr, op, ctrlClient); err != nil {
					return fmt.Errorf("unable to sync authorization prometheus rule: %v", err)
				}
				break
			}
		}

		if err := modules.InstallPolicies(ctx, isDeleting, op, cr, ctrlClient); err != nil {
			return fmt.Errorf("unable to install policies: %v", err)
		}
	}

	// gatewayComponentEnabled: v2.5.0+ installs use component name "nginx-gateway-fabric".
	// nginxComponentEnabled: v2.4.0 and below use component name "nginx".
	// Both are checked for the gateway path to handle upgrade from v2.4.0 to v2.5.0
	// where the CR component name may still be "nginx".
	gatewayComponentEnabled := operatorutils.IsModuleComponentEnabled(ctx, cr, csmv1.AuthorizationServer, modules.AuthGatewayComponent)
	nginxComponentEnabled := operatorutils.IsModuleComponentEnabled(ctx, cr, csmv1.AuthorizationServer, modules.AuthNginxIngressComponent)

	// Check if authorization module version is v2.5.0 or later (Gateway API)
	var isV25OrLater bool
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.AuthorizationServer {
			var err error
			isV25OrLater, err = operatorutils.MinVersionCheck("v2.5.0", m.ConfigVersion)
			if err != nil {
				log.Errorw("error checking authorization version", "error", err)
			}
			break
		}
	}

	if r.Config.IsOpenShift {
		log.Infow("Using OpenShift default ingress controller")
		if nginxComponentEnabled || gatewayComponentEnabled {
			log.Warnw("openshift environment, skipping deployment of nginx/gateway ingress controller")
		}
	} else {
		if isV25OrLater && (gatewayComponentEnabled || nginxComponentEnabled) {
			log.Infow("Reconcile authorization Gateway API Controller")

			// When upgrading from v2.4.0 to v2.5.0, explicitly delete old NGINX Ingress Controller
			// before deploying Gateway API controller
			if !isDeleting {
				log.Infow("Cleaning up old NGINX Ingress Controller before Gateway API deployment")
				if err := modules.NginxIngressControllerCleanup(ctx, op, cr, ctrlClient); err != nil {
					log.Warnw("Failed to cleanup old NGINX Ingress Controller (may not exist)", "error", err)
				}
			}

			if err := modules.GatewayController(ctx, isDeleting, op, cr, ctrlClient); err != nil {
				return fmt.Errorf("unable to reconcile gateway API controller for authorization: %v", err)
			}
		} else if nginxComponentEnabled {
			log.Infow("Reconcile authorization NGINX Ingress Controller")
			if err := modules.NginxIngressController(ctx, isDeleting, op, cr, ctrlClient); err != nil {
				return fmt.Errorf("unable to reconcile nginx ingress controller for authorization: %v", err)
			}
		}
	}

	// Authorization Ingress rules
	if operatorutils.IsModuleComponentEnabled(ctx, cr, csmv1.AuthorizationServer, modules.AuthProxyServerComponent) {
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

// reconcileAuthorizationCRDS - reconcile Authorization CRDs
func (r *ContainerStorageModuleReconciler) reconcileAuthorizationCRDS(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	// Install Authorization CRDs
	if operatorutils.IsModuleComponentEnabled(ctx, cr, csmv1.AuthorizationServer, modules.AuthProxyServerComponent) {
		log.Infow("Reconcile Authorization CRDS")
		if err := modules.AuthCrdDeploy(ctx, op, cr, ctrlClient); err != nil {
			return fmt.Errorf("unable to reconcile Authorization CRDs: %v", err)
		}
	}

	return nil
}

func (r *ContainerStorageModuleReconciler) reconcileReplicationCRDS(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	if err := modules.ReplicationCrdDeploy(ctx, op, cr, ctrlClient); err != nil {
		return fmt.Errorf("unable to reconcile replication CRDs: %v", err)
	}
	return nil
}

func getDriverConfig(ctx context.Context,
	cr csmv1.ContainerStorageModule,
	operatorConfig operatorutils.OperatorConfig,
	ctrlClient client.Client,
	matched operatorutils.VersionSpec,
) (*DriverConfig, error) {
	var (
		err        error
		driver     *storagev1.CSIDriver
		configMap  *corev1.ConfigMap
		node       *operatorutils.NodeYAML
		controller *operatorutils.ControllerYAML
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
	if driverType != csmv1.Cosi {
		driver, err = drivers.GetCSIDriver(ctx, cr, operatorConfig, driverType)
		if err != nil {
			return nil, fmt.Errorf("getting %s CSIDriver: %v", driverType, err)
		}

		node, err = drivers.GetNode(ctx, cr, operatorConfig, driverType, NodeYaml, ctrlClient, matched)
		if err != nil {
			return nil, fmt.Errorf("getting %s node: %v", driverType, err)
		}
	}
	configMap, err = drivers.GetConfigMap(ctx, cr, operatorConfig, driverType)
	if err != nil {
		return nil, fmt.Errorf("getting %s configMap: %v", driverType, err)
	}

	controller, err = drivers.GetController(ctx, cr, operatorConfig, driverType, matched)
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

// reconcileReverseProxyServer - deploy reverse proxy server
func (r *ContainerStorageModuleReconciler) reconcileReverseProxyServer(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)
	log.Infow("Reconcile reverseproxy proxy")
	if err := modules.ReverseProxyServer(ctx, isDeleting, op, cr, ctrlClient); err != nil {
		return fmt.Errorf("unable to reconcile reverse-proxy server: %v", err)
	}
	return nil
}

func removeDriverFromCluster(ctx context.Context, cluster operatorutils.ClusterConfig, driverConfig *DriverConfig) error {
	log := logger.GetLogger(ctx)
	var err error

	log.Infow("Removing driver from", cluster.ClusterID)

	if driverConfig.Node != nil {
		if err = operatorutils.DeleteObject(ctx, &driverConfig.Node.Rbac.ServiceAccount, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete node service account", "Error", err.Error())
			return err
		}
		if err = operatorutils.DeleteObject(ctx, &driverConfig.Node.Rbac.ClusterRole, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete node cluster role", "Error", err.Error())
			return err
		}
		if err = operatorutils.DeleteObject(ctx, &driverConfig.Node.Rbac.ClusterRoleBinding, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete node cluster role binding", "Error", err.Error())
			return err
		}
		if err = operatorutils.DeleteObject(ctx, &driverConfig.Node.Rbac.Role, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete node role", "Error", err.Error())
			return err
		}
		if err = operatorutils.DeleteObject(ctx, &driverConfig.Node.Rbac.RoleBinding, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete node role binding", "Error", err.Error())
			return err
		}
	}

	if driverConfig.Controller != nil {
		if err = operatorutils.DeleteObject(ctx, &driverConfig.Controller.Rbac.ServiceAccount, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete controller service account", "Error", err.Error())
			return err
		}
		if err = operatorutils.DeleteObject(ctx, &driverConfig.Controller.Rbac.ClusterRole, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete controller cluster role", "Error", err.Error())
			return err
		}
		if err = operatorutils.DeleteObject(ctx, &driverConfig.Controller.Rbac.ClusterRoleBinding, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete controller cluster role binding", "Error", err.Error())
			return err
		}
		if err = operatorutils.DeleteObject(ctx, &driverConfig.Controller.Rbac.Role, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete controller cluster role", "Error", err.Error())
			return err
		}
		if err = operatorutils.DeleteObject(ctx, &driverConfig.Controller.Rbac.RoleBinding, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete controller role binding", "Error", err.Error())
			return err
		}
	}

	if driverConfig.ConfigMap != nil {
		if err = operatorutils.DeleteObject(ctx, driverConfig.ConfigMap, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete configmap", "Error", err.Error())
			return err
		}
	}

	if driverConfig.Driver != nil {
		if err = operatorutils.DeleteObject(ctx, driverConfig.Driver, cluster.ClusterCTRLClient); err != nil {
			log.Errorw("error delete csi driver", "Error", err.Error())
			return err
		}
	}

	if driverConfig.Node != nil {
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
	}

	if driverConfig.Controller != nil {
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

	}

	return nil
}

// removeDeploymentOwnerRef removes the ownerReference from the controller
// deployment so that Kubernetes garbage collection does not delete the
// deployment when the CSM CR is removed (forceRemoveDriver=false).
func (r *ContainerStorageModuleReconciler) removeDeploymentOwnerRef(ctx context.Context, csm *csmv1.ContainerStorageModule) error {
	log := logger.GetLogger(ctx)
	deployName := csm.GetControllerName()
	ns := csm.GetNamespace()

	deploy := &appsv1.Deployment{}
	err := r.GetClient().Get(ctx, t1.NamespacedName{Name: deployName, Namespace: ns}, deploy)
	if err != nil {
		return err
	}

	// Remove ownerReferences that point to this CSM CR
	filtered := make([]metav1.OwnerReference, 0, len(deploy.OwnerReferences))
	for _, ref := range deploy.OwnerReferences {
		if ref.UID != csm.GetUID() {
			filtered = append(filtered, ref)
		}
	}
	if len(filtered) == len(deploy.OwnerReferences) {
		return nil // nothing to remove
	}
	// Set to nil (not empty slice) so that PreChecks skips the ownerRef
	// validation on re-apply. An empty non-nil slice would cause PreChecks
	// to enter the check block but find no matching owner, returning an error.
	if len(filtered) == 0 {
		deploy.OwnerReferences = nil
	} else {
		deploy.OwnerReferences = filtered
	}
	log.Infow("Removing ownerReference from deployment to prevent GC", "deployment", deployName)
	return r.GetClient().Update(ctx, deploy)
}

func (r *ContainerStorageModuleReconciler) removeDriver(ctx context.Context, instance csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig) error {
	log := logger.GetLogger(ctx)

	// Get Driver resources
	driverConfig, err := getDriverConfig(ctx, instance, operatorConfig, r.Client, operatorutils.VersionSpec{})
	if err != nil {
		log.Error("error in getDriverConfig")
		return err
	}
	// driverConfig = nil means no driver specified in manifest
	if driverConfig == nil {
		return nil
	}

	clusterClient := operatorutils.GetCluster(ctx, r)
	if err = removeDriverFromCluster(ctx, clusterClient, driverConfig); err != nil {
		return err
	}
	replicationEnabled, _ := operatorutils.IsModuleEnabled(ctx, instance, csmv1.Replication)
	if replicationEnabled {
		log.Infow("Deleting Replication controller")
		if err = modules.ReplicationManagerController(ctx, true, operatorConfig, instance, clusterClient.ClusterCTRLClient); err != nil {
			return err
		}
		log.Infow("Deleting Replication configmap")
		if err = modules.DeleteReplicationConfigmap(clusterClient.ClusterCTRLClient); err != nil {
			return err
		}

		log.Infow("Deleting Replication CRDs")
		if err = modules.DeleteReplicationCrds(ctx, operatorConfig, instance, clusterClient.ClusterCTRLClient); err != nil {
			// failure here should not block  the deletion of the other components
			log.Warnf("unable to delete replication CRDs: %v", err)
		}
	}

	// remove module observability
	if observabilityEnabled, _ := operatorutils.IsModuleEnabled(ctx, instance, csmv1.Observability); observabilityEnabled {
		log.Infow("Deleting observability")
		if err = r.reconcileObservability(ctx, true, operatorConfig, instance, nil, clusterClient.ClusterCTRLClient, clusterClient.ClusterK8sClient, operatorutils.VersionSpec{}); err != nil {
			return err
		}
	}

	if instance.GetDriverType() == csmv1.PowerMax && modules.IsReverseProxySidecar() {
		log.Info("Removing CSI ReverseProxy Service")
		if err := modules.ReverseProxyStartService(ctx, true, operatorConfig, instance, clusterClient.ClusterCTRLClient); err != nil {
			return fmt.Errorf("unable to reconcile reverse-proxy service: %v", err)
		}
	}

	if instance.GetDriverType() == csmv1.PowerStore {
		// Version should not matter but CRD should be deleted no matter what.
		log.Infoln("Checking/removing the common CSM Disaster Recovery CRDs")

		if err := modules.PatchCSMDRCRDs(ctx, true, operatorConfig, clusterClient.ClusterCTRLClient); err != nil {
			return fmt.Errorf("unable to remove the common CSM Disaster Recovery CRDs: %v", err)
		}
	}

	// Do NOT delete the cluster-wide shared VolumeJournal CRD on per-instance removal.
	// The CRD is shared across drivers (PowerMax, PowerStore CSM-DR) and deleting it
	// would destroy all pending journals for all consumers. Prefer leaving it installed.
	// Ownership isolation is handled via labels (driver type, driver name, instance UID)
	// in the VolumeJournal CR instances, not by deleting the CRD itself.
	// CRD lifecycle management should be centralized and only performed when
	// no compatible consumers and no journal objects remain.

	if operatorutils.SupportsDriverMetrics(instance.GetDriverType()) {
		if err := syncMetricsResources(ctx, true, instance, operatorConfig, clusterClient.ClusterCTRLClient); err != nil {
			return err
		}
	}
	replicationModule := csmv1.Module{Name: csmv1.Replication}
	for _, module := range instance.Spec.Modules {
		if module.Name == csmv1.Replication {
			replicationModule = module
			break
		}
	}
	if err := syncReplicationPrometheusRule(ctx, true, replicationModule, instance, operatorConfig, clusterClient.ClusterCTRLClient); err != nil {
		return err
	}

	return nil
}

// syncMetricsResources creates or deletes the metrics Service, ServiceMonitor, PodMonitor, and PrometheusRule resources.
// When isDeleting is true or metrics are disabled the resources are removed; otherwise they are created/updated.
// operatorConfig is used to locate the per-driver YAML file that defines the PrometheusRule alert rules.
func syncMetricsResources(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	metrics := cr.Spec.Driver.Metrics
	metricsEnabled := metrics != nil && metrics.Enabled
	shouldDelete := isDeleting || !metricsEnabled

	// Default port: 8443 for PowerScale, PowerStore, and PowerMax; 9090 for PowerFlex and all other drivers.
	port := int32(9090)
	if cr.GetDriverType() != csmv1.PowerFlex && supportsPodMonitor(cr.GetDriverType()) {
		port = int32(8443)
	}
	if metrics != nil && metrics.Port != 0 {
		port = metrics.Port
	}

	bController := true
	bOwnerDeletion := cr.Spec.Driver.ForceRemoveDriver != nil && !*cr.Spec.Driver.ForceRemoveDriver
	ownerRef := metav1.OwnerReference{
		APIVersion:         "storage.dell.com/v1",
		Kind:               cr.Kind,
		Name:               cr.Name,
		UID:                cr.GetUID(),
		Controller:         &bController,
		BlockOwnerDeletion: &bOwnerDeletion,
	}

	// Use driver-specific label key for backward compatibility
	// PowerStore and PowerFlex use "name" label, other drivers use "app"
	controllerLabelKey := "app"
	controllerLabelValue := cr.Name + "-controller"
	if cr.GetDriverType() == csmv1.PowerStore || cr.GetDriverType() == csmv1.PowerFlex {
		controllerLabelKey = "name"
	}

	svcName := cr.Name + "-metrics"
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            svcName,
			Namespace:       cr.Namespace,
			Labels:          map[string]string{controllerLabelKey: controllerLabelValue},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{controllerLabelKey: controllerLabelValue},
			Ports: []corev1.ServicePort{
				{
					Name:       "metrics",
					Port:       port,
					TargetPort: intstr.FromInt32(port),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	svc.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Service"))

	if shouldDelete {
		if err := operatorutils.DeleteObject(ctx, svc, ctrlClient); err != nil {
			log.Warnw("Failed to delete metrics Service", "name", svcName, "error", err)
		}
	} else {
		if err := operatorutils.ApplyObject(ctx, svc, ctrlClient); err != nil {
			return fmt.Errorf("failed to sync metrics Service %s: %v", svcName, err)
		}
	}

	smName := cr.Name + "-metrics-monitor"

	smInterval := "30s"
	smScrapeTimeout := ""
	smInsecureSkipVerify := false
	serviceMonitorEnabled := !shouldDelete && metrics.ServiceMonitor != nil && metrics.ServiceMonitor.Enabled
	if serviceMonitorEnabled {
		if metrics.ServiceMonitor.Interval != "" {
			sanitized := operatorutils.PositiveDurationOrDefault(metrics.ServiceMonitor.Interval, smInterval)
			if sanitized != metrics.ServiceMonitor.Interval {
				log.Warnw("Invalid ServiceMonitor interval, using default", "provided", metrics.ServiceMonitor.Interval, "using", sanitized)
			}
			smInterval = sanitized
		}
		smScrapeTimeout = operatorutils.PositiveDurationOrEmpty(metrics.ServiceMonitor.ScrapeTimeout)
		if metrics.ServiceMonitor.ScrapeTimeout != "" && smScrapeTimeout == "" {
			log.Warnw("Invalid ServiceMonitor scrapeTimeout, ignoring", "provided", metrics.ServiceMonitor.ScrapeTimeout)
		}
		smInsecureSkipVerify = metrics.ServiceMonitor.InsecureSkipVerify
	}

	smEndpoint := map[string]interface{}{
		"port":     "metrics",
		"interval": smInterval,
		"path":     "/metrics",
	}
	if smScrapeTimeout != "" {
		smEndpoint["scrapeTimeout"] = smScrapeTimeout
	}
	if metrics != nil && metrics.TLSCertSecret != "" {
		smEndpoint["scheme"] = "https"
		smEndpoint["tlsConfig"] = map[string]interface{}{
			"insecureSkipVerify": smInsecureSkipVerify,
		}
	}

	sm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "monitoring.coreos.com/v1",
			"kind":       "ServiceMonitor",
			"metadata": map[string]interface{}{
				"name":      smName,
				"namespace": cr.Namespace,
				"labels":    map[string]interface{}{controllerLabelKey: cr.Name + "-controller"},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "storage.dell.com/v1",
						"kind":               cr.Kind,
						"name":               cr.Name,
						"uid":                string(cr.GetUID()),
						"controller":         true,
						"blockOwnerDeletion": bOwnerDeletion,
					},
				},
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						controllerLabelKey: cr.Name + "-controller",
					},
				},
				"endpoints": []interface{}{smEndpoint},
			},
		},
	}

	if !serviceMonitorEnabled {
		if err := operatorutils.DeleteObject(ctx, sm, ctrlClient); err != nil {
			// ServiceMonitor CRD may not be installed; log as warning rather than failing
			log.Warnw("Failed to delete ServiceMonitor (Prometheus Operator may not be installed)", "name", smName, "error", err)
		}
	} else {
		if err := operatorutils.ApplyObject(ctx, sm, ctrlClient); err != nil {
			return fmt.Errorf("failed to sync ServiceMonitor %s: %v", smName, err)
		}
	}

	// PodMonitor — targets node pods; reconciled for PowerScale, PowerStore, and PowerMax.
	pmName := cr.Name + "-node-metrics-monitor"
	pmInterval := "30s"
	pmScrapeTimeout := ""
	pmInsecureSkipVerify := false
	podMonitorEnabled := !shouldDelete &&
		supportsPodMonitor(cr.GetDriverType()) &&
		metrics.PodMonitor != nil && metrics.PodMonitor.Enabled
	if podMonitorEnabled {
		if metrics.PodMonitor.Interval != "" {
			sanitized := operatorutils.PositiveDurationOrDefault(metrics.PodMonitor.Interval, pmInterval)
			if sanitized != metrics.PodMonitor.Interval {
				log.Warnw("Invalid PodMonitor interval, using default", "provided", metrics.PodMonitor.Interval, "using", sanitized)
			}
			pmInterval = sanitized
		}
		pmScrapeTimeout = operatorutils.PositiveDurationOrEmpty(metrics.PodMonitor.ScrapeTimeout)
		if metrics.PodMonitor.ScrapeTimeout != "" && pmScrapeTimeout == "" {
			log.Warnw("Invalid PodMonitor scrapeTimeout, ignoring", "provided", metrics.PodMonitor.ScrapeTimeout)
		}
		pmInsecureSkipVerify = metrics.PodMonitor.InsecureSkipVerify
	}

	pmEndpoint := map[string]interface{}{
		"port":     "metrics",
		"interval": pmInterval,
		"path":     "/metrics",
	}
	if pmScrapeTimeout != "" {
		pmEndpoint["scrapeTimeout"] = pmScrapeTimeout
	}
	if metrics != nil && metrics.TLSCertSecret != "" {
		pmEndpoint["scheme"] = "https"
		pmEndpoint["tlsConfig"] = map[string]interface{}{
			"insecureSkipVerify": pmInsecureSkipVerify,
		}
	}

	pm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "monitoring.coreos.com/v1",
			"kind":       "PodMonitor",
			"metadata": map[string]interface{}{
				"name":      pmName,
				"namespace": cr.Namespace,
				"labels":    map[string]interface{}{"app": cr.Name + "-node"},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "storage.dell.com/v1",
						"kind":               cr.Kind,
						"name":               cr.Name,
						"uid":                string(cr.GetUID()),
						"controller":         true,
						"blockOwnerDeletion": bOwnerDeletion,
					},
				},
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": cr.Name + "-node",
					},
				},
				"podMetricsEndpoints": []interface{}{pmEndpoint},
			},
		},
	}

	if supportsPodMonitor(cr.GetDriverType()) {
		if !podMonitorEnabled {
			if err := operatorutils.DeleteObject(ctx, pm, ctrlClient); err != nil {
				log.Warnw("Failed to delete PodMonitor (Prometheus Operator may not be installed)", "name", pmName, "error", err)
			}
		} else {
			if err := operatorutils.ApplyObject(ctx, pm, ctrlClient); err != nil {
				return fmt.Errorf("failed to sync PodMonitor %s: %v", pmName, err)
			}
		}
	}

	prometheusRuleName := metricsPrometheusRuleName(cr.Name)
	prometheusRuleGroups, prometheusRuleEnabled := func() ([]interface{}, bool) {
		if shouldDelete || metrics.PrometheusRule == nil || !metrics.PrometheusRule.Enabled {
			return nil, false
		}
		groups, groupsResolved := resolvePrometheusRuleGroups(ctx, cr, operatorConfig)
		if !groupsResolved {
			return nil, false
		}
		// Inject namespace label into all alert rules across all groups for proper
		// AlertmanagerConfig namespace-scoped routing.
		injectNamespaceLabelIntoGroups(groups, cr.Namespace)
		return groups, true
	}()

	prometheusRule := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "monitoring.coreos.com/v1",
			"kind":       "PrometheusRule",
			"metadata": map[string]interface{}{
				"name":      prometheusRuleName,
				"namespace": cr.Namespace,
				"labels": map[string]interface{}{
					controllerLabelKey: cr.Name + "-controller",
					"namespace":        cr.Namespace,
				},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "storage.dell.com/v1",
						"kind":               cr.Kind,
						"name":               cr.Name,
						"uid":                string(cr.GetUID()),
						"controller":         true,
						"blockOwnerDeletion": bOwnerDeletion,
					},
				},
			},
			"spec": map[string]interface{}{
				"groups": prometheusRuleGroups,
			},
		},
	}

	if !prometheusRuleEnabled {
		if err := operatorutils.DeleteObject(ctx, prometheusRule, ctrlClient); err != nil {
			log.Warnw("Failed to delete PrometheusRule (Prometheus Operator may not be installed)", "name", prometheusRuleName, "error", err)
		}
	} else {
		if err := operatorutils.ApplyObject(ctx, prometheusRule, ctrlClient); err != nil {
			return fmt.Errorf("failed to sync PrometheusRule %s: %v", prometheusRuleName, err)
		}
	}

	return nil
}

// syncModuleMetricsResources creates Service, ServiceMonitor, and PodMonitor for module metrics.
// When isDeleting is true or metrics are disabled the resources are removed; otherwise they are created/updated.
func syncModuleMetricsResources(ctx context.Context, isDeleting bool, module csmv1.Module, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	// Only process resiliency, replication, and authorization module metrics
	if module.Name != csmv1.Resiliency && module.Name != csmv1.Replication && module.Name != csmv1.AuthorizationServer {
		return nil
	}

	svcName := cr.Name + "-resiliency-metrics"
	smName := cr.Name + "-resiliency-metrics"
	pmName := cr.Name + "-resiliency-metrics"

	metricsEnabled := module.Metrics != nil && module.Metrics.Enabled
	shouldDelete := isDeleting || !metricsEnabled

	port := int32(8444)
	controllerLabelValue := cr.Name + "-controller"
	serviceMonitorInterval := "30s"
	hasPodMonitor := true
	if module.Name == csmv1.Replication {
		svcName = cr.Name + "-replication-metrics"
		smName = cr.Name + "-replication-metrics"
		pmName = cr.Name + "-replication-metrics"
		port = 8445
	} else if module.Name == csmv1.AuthorizationServer {
		svcName = "proxy-server-metrics"
		smName = "proxy-server-metrics-monitor"
		pmName = ""
		port = modules.DefaultAuthProxyMetricsPort
		controllerLabelValue = "proxy-server"
		serviceMonitorInterval = modules.DefaultAuthProxyServiceMonitorInterval
		hasPodMonitor = false
	}
	if module.Metrics != nil && module.Metrics.Port != 0 {
		port = module.Metrics.Port
	}

	bController := true
	bOwnerDeletion := false
	ownerRef := metav1.OwnerReference{
		APIVersion:         "storage.dell.com/v1",
		Kind:               cr.Kind,
		Name:               cr.Name,
		UID:                cr.GetUID(),
		Controller:         &bController,
		BlockOwnerDeletion: &bOwnerDeletion,
	}

	// Use driver-specific label key for backward compatibility
	// PowerStore and PowerFlex use "name" label, other drivers use "app"
	controllerLabelKey := "app"
	if cr.GetDriverType() == csmv1.PowerStore || cr.GetDriverType() == csmv1.PowerFlex {
		controllerLabelKey = "name"
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            svcName,
			Namespace:       cr.Namespace,
			Labels:          map[string]string{controllerLabelKey: controllerLabelValue},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{controllerLabelKey: controllerLabelValue},
			Ports: []corev1.ServicePort{
				{
					Name:       "metrics",
					Port:       port,
					TargetPort: intstr.FromInt32(port),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	svc.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Service"))

	if shouldDelete {
		if err := operatorutils.DeleteObject(ctx, svc, ctrlClient); err != nil {
			log.Warnw("Failed to delete module metrics Service", "module", module.Name, "name", svcName, "error", err)
		}
	} else {
		if err := operatorutils.ApplyObject(ctx, svc, ctrlClient); err != nil {
			return fmt.Errorf("failed to sync module metrics Service %s: %v", svcName, err)
		}
		log.Infow("Synced module metrics Service", "module", module.Name, "name", svcName)
	}

	if shouldDelete {
		// Delete ServiceMonitor and PodMonitor
		sm := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "monitoring.coreos.com/v1",
				"kind":       "ServiceMonitor",
				"metadata": map[string]interface{}{
					"name":      smName,
					"namespace": cr.Namespace,
				},
			},
		}
		if err := operatorutils.DeleteObject(ctx, sm, ctrlClient); err != nil {
			log.Warnw("Failed to delete ServiceMonitor (Prometheus Operator may not be installed)", "name", smName, "error", err)
		}

		if hasPodMonitor {
			pm := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "monitoring.coreos.com/v1",
					"kind":       "PodMonitor",
					"metadata": map[string]interface{}{
						"name":      pmName,
						"namespace": cr.Namespace,
					},
				},
			}
			if err := operatorutils.DeleteObject(ctx, pm, ctrlClient); err != nil {
				log.Warnw("Failed to delete PodMonitor (Prometheus Operator may not be installed)", "name", pmName, "error", err)
			}
		}
		return nil
	}

	// Metrics enabled and not deleting, create/update ServiceMonitor
	if module.Metrics.ServiceMonitor != nil && module.Metrics.ServiceMonitor.Enabled {
		smInterval := serviceMonitorInterval
		smScrapeTimeout := ""
		if module.Metrics.ServiceMonitor.Interval != "" {
			sanitized := operatorutils.PositiveDurationOrDefault(module.Metrics.ServiceMonitor.Interval, smInterval)
			if sanitized != module.Metrics.ServiceMonitor.Interval {
				log.Warnw("Invalid ServiceMonitor interval, using default", "provided", module.Metrics.ServiceMonitor.Interval, "using", sanitized)
			}
			smInterval = sanitized
		}
		smScrapeTimeout = operatorutils.PositiveDurationOrEmpty(module.Metrics.ServiceMonitor.ScrapeTimeout)
		if module.Metrics.ServiceMonitor.ScrapeTimeout != "" && smScrapeTimeout == "" {
			log.Warnw("Invalid ServiceMonitor scrapeTimeout, ignoring", "provided", module.Metrics.ServiceMonitor.ScrapeTimeout)
		}

		smEndpoint := map[string]interface{}{
			"port":     "metrics",
			"interval": smInterval,
			"path":     "/metrics",
		}
		if smScrapeTimeout != "" {
			smEndpoint["scrapeTimeout"] = smScrapeTimeout
		}

		// Add TLS configuration when TLS cert secret is set
		if module.Metrics.TLSCertSecret != "" {
			smEndpoint["scheme"] = "https"
			smEndpoint["tlsConfig"] = map[string]interface{}{
				"insecureSkipVerify": module.Metrics.ServiceMonitor.InsecureSkipVerify,
			}
		}

		sm := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "monitoring.coreos.com/v1",
				"kind":       "ServiceMonitor",
				"metadata": map[string]interface{}{
					"name":      smName,
					"namespace": cr.Namespace,
					"labels": map[string]interface{}{
						controllerLabelKey: controllerLabelValue,
					},
					"ownerReferences": []interface{}{
						map[string]interface{}{
							"apiVersion":         "storage.dell.com/v1",
							"kind":               cr.Kind,
							"name":               cr.Name,
							"uid":                string(cr.GetUID()),
							"controller":         bController,
							"blockOwnerDeletion": bOwnerDeletion,
						},
					},
				},
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							controllerLabelKey: controllerLabelValue,
						},
					},
					"endpoints": []interface{}{smEndpoint},
				},
			},
		}
		if err := operatorutils.ApplyObject(ctx, sm, ctrlClient); err != nil {
			log.Warnw("Failed to sync ServiceMonitor (Prometheus Operator may not be installed)", "name", smName, "error", err)
			return fmt.Errorf("failed to sync ServiceMonitor %s: %v", smName, err)
		}
		log.Infow("Synced ServiceMonitor", "name", smName)
	}

	// Metrics enabled, create/update PodMonitor
	if hasPodMonitor && module.Metrics.PodMonitor != nil && module.Metrics.PodMonitor.Enabled {
		pmInterval := "30s"
		pmScrapeTimeout := ""
		if module.Metrics.PodMonitor.Interval != "" {
			sanitized := operatorutils.PositiveDurationOrDefault(module.Metrics.PodMonitor.Interval, pmInterval)
			if sanitized != module.Metrics.PodMonitor.Interval {
				log.Warnw("Invalid PodMonitor interval, using default", "provided", module.Metrics.PodMonitor.Interval, "using", sanitized)
			}
			pmInterval = sanitized
		}
		pmScrapeTimeout = operatorutils.PositiveDurationOrEmpty(module.Metrics.PodMonitor.ScrapeTimeout)
		if module.Metrics.PodMonitor.ScrapeTimeout != "" && pmScrapeTimeout == "" {
			log.Warnw("Invalid PodMonitor scrapeTimeout, ignoring", "provided", module.Metrics.PodMonitor.ScrapeTimeout)
		}

		pmEndpoint := map[string]interface{}{
			"port":     "metrics",
			"interval": pmInterval,
			"path":     "/metrics",
		}
		if pmScrapeTimeout != "" {
			pmEndpoint["scrapeTimeout"] = pmScrapeTimeout
		}

		// Add TLS configuration when TLS cert secret is set
		if module.Metrics.TLSCertSecret != "" {
			pmEndpoint["scheme"] = "https"
			pmEndpoint["tlsConfig"] = map[string]interface{}{
				"insecureSkipVerify": module.Metrics.PodMonitor.InsecureSkipVerify,
			}
		}

		// All drivers use "app" as the label key for node DaemonSet pods,
		// regardless of what they use for controller Deployment pods.
		pm := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "monitoring.coreos.com/v1",
				"kind":       "PodMonitor",
				"metadata": map[string]interface{}{
					"name":      pmName,
					"namespace": cr.Namespace,
					"labels":    map[string]interface{}{"app": cr.Name + "-node"},
					"ownerReferences": []interface{}{
						map[string]interface{}{
							"apiVersion":         "storage.dell.com/v1",
							"kind":               cr.Kind,
							"name":               cr.Name,
							"uid":                string(cr.GetUID()),
							"controller":         bController,
							"blockOwnerDeletion": bOwnerDeletion,
						},
					},
				},
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": cr.Name + "-node",
						},
					},
					"podMetricsEndpoints": []interface{}{pmEndpoint},
				},
			},
		}
		if err := operatorutils.ApplyObject(ctx, pm, ctrlClient); err != nil {
			log.Warnw("Failed to sync PodMonitor (Prometheus Operator may not be installed)", "name", pmName, "error", err)
			return fmt.Errorf("failed to sync PodMonitor %s: %v", pmName, err)
		}
		log.Infow("Synced PodMonitor", "name", pmName)
	}

	return nil
}

// syncResiliencyPrometheusRule creates or deletes a PrometheusRule for the Resiliency module.
// It is called separately from syncModuleMetricsResources so that operatorConfig is available
// to resolve the module version from csm-releases.yaml.
// When isDeleting is true or the module's PrometheusRule is disabled the resource is removed.
func syncResiliencyPrometheusRule(ctx context.Context, isDeleting bool, module csmv1.Module, cr csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	// Only process Resiliency module
	if module.Name != csmv1.Resiliency {
		return nil
	}

	prName := resiliencyPrometheusRuleName(cr.Name)
	metricsEnabled := module.Metrics != nil && module.Metrics.Enabled
	prEnabled := metricsEnabled && module.Metrics.PrometheusRule != nil && module.Metrics.PrometheusRule.Enabled
	shouldDelete := isDeleting || !prEnabled

	var prGroups []interface{}
	if !shouldDelete {
		// Resolve the driver config version, then look up the bundled resiliency module version.
		driverConfigVersion, err := operatorutils.GetVersion(ctx, &cr, operatorConfig)
		if err != nil {
			log.Warnw("Failed to resolve driver version for Resiliency PrometheusRule, skipping", "error", err)
			shouldDelete = true
		} else {
			moduleVersion, err := operatorutils.GetModuleDefaultVersion(driverConfigVersion, cr.GetDriverType(), csmv1.Resiliency, operatorConfig.ConfigDirectory)
			if err != nil {
				log.Warnw("Failed to resolve resiliency module version for PrometheusRule, skipping", "version", driverConfigVersion, "error", err)
				shouldDelete = true
			} else {
				groups, err := buildResiliencyAlertGroups(operatorConfig.ConfigDirectory, moduleVersion, cr.GetDriverType(), module.Metrics.PrometheusRule)
				if err != nil {
					log.Warnw("Failed to load Resiliency PrometheusRule YAML, skipping", "version", moduleVersion, "error", err)
					shouldDelete = true
				} else {
					// Inject namespace label for AlertmanagerConfig routing
					injectNamespaceLabelIntoGroups(groups, cr.Namespace)
					prGroups = groups
				}
			}
		}
	}

	// Use driver-specific label key for backward compatibility (PowerStore/PowerFlex use "name", others use "app")
	controllerLabelKey := "app"
	controllerLabelValue := cr.Name + "-controller"
	if cr.GetDriverType() == csmv1.PowerStore || cr.GetDriverType() == csmv1.PowerFlex {
		controllerLabelKey = "name"
	}

	bController := true
	bOwnerDeletion := cr.Spec.Driver.ForceRemoveDriver != nil && !*cr.Spec.Driver.ForceRemoveDriver
	pr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "monitoring.coreos.com/v1",
			"kind":       "PrometheusRule",
			"metadata": map[string]interface{}{
				"name":      prName,
				"namespace": cr.Namespace,
				"labels":    map[string]interface{}{controllerLabelKey: controllerLabelValue},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "storage.dell.com/v1",
						"kind":               cr.Kind,
						"name":               cr.Name,
						"uid":                string(cr.GetUID()),
						"controller":         bController,
						"blockOwnerDeletion": bOwnerDeletion,
					},
				},
			},
			"spec": map[string]interface{}{
				"groups": prGroups,
			},
		},
	}

	if shouldDelete {
		if err := operatorutils.DeleteObject(ctx, pr, ctrlClient); err != nil {
			log.Warnw("Failed to delete Resiliency PrometheusRule (Prometheus Operator may not be installed)", "name", prName, "error", err)
		}
		return nil
	}
	if err := operatorutils.ApplyObject(ctx, pr, ctrlClient); err != nil {
		return fmt.Errorf("failed to sync Resiliency PrometheusRule %s: %v", prName, err)
	}
	log.Infow("Synced Resiliency PrometheusRule", "name", prName)
	return nil
}

func syncReplicationPrometheusRule(ctx context.Context, isDeleting bool, module csmv1.Module, cr csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	if module.Name != csmv1.Replication {
		return nil
	}

	prName := replicationPrometheusRuleName(cr.Name)
	metricsEnabled := module.Metrics != nil && module.Metrics.Enabled
	prEnabled := module.Enabled && metricsEnabled && module.Metrics.PrometheusRule != nil && module.Metrics.PrometheusRule.Enabled
	shouldDelete := isDeleting || !prEnabled

	var prGroups []interface{}
	if !shouldDelete {
		groups, ok := resolveReplicationModuleAlertGroups(ctx, cr, operatorConfig)
		if !ok {
			shouldDelete = true
		} else {
			injectNamespaceLabelIntoGroups(groups, cr.Namespace)
			prGroups = groups
		}
	}

	controllerLabelKey := "app"
	if cr.GetDriverType() == csmv1.PowerStore || cr.GetDriverType() == csmv1.PowerFlex {
		controllerLabelKey = "name"
	}

	bController := true
	bOwnerDeletion := cr.Spec.Driver.ForceRemoveDriver != nil && !*cr.Spec.Driver.ForceRemoveDriver
	pr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "monitoring.coreos.com/v1",
			"kind":       "PrometheusRule",
			"metadata": map[string]interface{}{
				"name":      prName,
				"namespace": cr.Namespace,
				"labels":    map[string]interface{}{controllerLabelKey: cr.Name + "-controller"},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "storage.dell.com/v1",
						"kind":               cr.Kind,
						"name":               cr.Name,
						"uid":                string(cr.GetUID()),
						"controller":         bController,
						"blockOwnerDeletion": bOwnerDeletion,
					},
				},
			},
			"spec": map[string]interface{}{
				"groups": prGroups,
			},
		},
	}

	if shouldDelete {
		if err := operatorutils.DeleteObject(ctx, pr, ctrlClient); err != nil {
			log.Warnw("Failed to delete replication PrometheusRule (Prometheus Operator may not be installed)", "name", prName, "error", err)
		}
		return nil
	}
	if err := operatorutils.ApplyObject(ctx, pr, ctrlClient); err != nil {
		return fmt.Errorf("failed to sync replication PrometheusRule %s: %v", prName, err)
	}
	log.Infow("Synced replication PrometheusRule", "name", prName)
	return nil
}

// syncAuthorizationPrometheusRule creates or deletes the PrometheusRule for the CSM Authorization
// module based on module metrics and PrometheusRule configuration. It loads alert rules from the
// per-version module config and applies configurable thresholds before creating the resource.
func syncAuthorizationPrometheusRule(ctx context.Context, isDeleting bool, module csmv1.Module, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	metricsEnabled := module.Metrics != nil && module.Metrics.Enabled
	prometheusRuleEnabled := metricsEnabled && module.Metrics.PrometheusRule != nil && module.Metrics.PrometheusRule.Enabled
	shouldDelete := isDeleting || !metricsEnabled || !prometheusRuleEnabled

	promRuleName := authorizationPrometheusRuleName(cr.Name)

	// Use driver-specific label key for backward compatibility (PowerStore/PowerFlex use "name", others use "app").
	controllerLabelKey := "app"
	controllerLabelValue := "proxy-server"
	if cr.GetDriverType() == csmv1.PowerStore || cr.GetDriverType() == csmv1.PowerFlex {
		controllerLabelKey = "name"
	}

	bController := true
	bOwnerDeletion := false

	promRule := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "monitoring.coreos.com/v1",
			"kind":       "PrometheusRule",
			"metadata": map[string]interface{}{
				"name":      promRuleName,
				"namespace": cr.Namespace,
				"labels": map[string]interface{}{
					controllerLabelKey: controllerLabelValue,
				},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "storage.dell.com/v1",
						"kind":               cr.Kind,
						"name":               cr.Name,
						"uid":                string(cr.GetUID()),
						"controller":         bController,
						"blockOwnerDeletion": bOwnerDeletion,
					},
				},
			},
		},
	}

	if shouldDelete {
		log.Infow("Deleting authorization PrometheusRule", "name", promRuleName)
		if err := operatorutils.DeleteObject(ctx, promRule, ctrlClient); err != nil {
			log.Warnw("Failed to delete authorization PrometheusRule", "name", promRuleName, "error", err)
		}
		return nil
	}

	log.Infow("Syncing authorization PrometheusRule", "name", promRuleName)

	rules, err := buildAuthorizationAlertRules(ctx, op, module, cr)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Warnw("Failed to load authorization PrometheusRule YAML, skipping", "version", module.ConfigVersion, "error", err)
			if delErr := operatorutils.DeleteObject(ctx, promRule, ctrlClient); delErr != nil {
				log.Warnw("Failed to delete authorization PrometheusRule", "name", promRuleName, "error", delErr)
			}
			return nil
		}
		return fmt.Errorf("failed to build authorization PrometheusRule alert rules: %w", err)
	}

	groups := []interface{}{
		map[string]interface{}{
			"name":  "csm-authorization-alerts",
			"rules": rules,
		},
	}
	injectNamespaceLabelIntoGroups(groups, cr.Namespace)

	if err := unstructured.SetNestedMap(promRule.Object, map[string]interface{}{
		"groups": groups,
	}, "spec"); err != nil {
		return fmt.Errorf("failed to set PrometheusRule spec: %v", err)
	}

	if err := operatorutils.ApplyObject(ctx, promRule, ctrlClient); err != nil {
		return fmt.Errorf("failed to apply authorization PrometheusRule %s: %v", promRuleName, err)
	}

	return nil
}

// removeModule - remove standalone modules
func (r *ContainerStorageModuleReconciler) removeModule(ctx context.Context, instance csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	if authorizationEnabled, _ := operatorutils.IsModuleEnabled(ctx, instance, csmv1.AuthorizationServer); authorizationEnabled {
		log.Infow("Deleting Authorization Proxy Server")
		if err := r.reconcileAuthorization(ctx, true, operatorConfig, instance, ctrlClient, operatorutils.VersionSpec{}); err != nil {
			return err
		}
		log.Infow("Deleting Authorization CRDs")
		if err := modules.DeleteAuthCrds(ctx, operatorConfig, instance, ctrlClient); err != nil {
			// failure here should not block the deletion of the other components
			log.Warnf("unable to delete authorization CRDs: %v", err)
		}
	}
	if reverseproxyEnabled, _ := operatorutils.IsModuleEnabled(ctx, instance, csmv1.ReverseProxy); reverseproxyEnabled {
		// Check if standalone reverse proxy deployment exists before attempting deletion
		// This avoids relying on the package-level deployAsSidecar variable during deletion
		deployment := &appsv1.Deployment{}
		err := ctrlClient.Get(ctx, t1.NamespacedName{Name: "csipowermax-reverseproxy", Namespace: instance.GetNamespace()}, deployment)
		if err == nil {
			// Deployment exists, delete it (standalone deployment)
			log.Infow("Deleting standalone ReverseProxy deployment")
			if err := r.reconcileReverseProxyServer(ctx, true, operatorConfig, instance, ctrlClient); err != nil {
				return err
			}
		}
	}

	// Clean up resiliency module metrics resources
	if resiliencyEnabled, _ := operatorutils.IsModuleEnabled(ctx, instance, csmv1.Resiliency); resiliencyEnabled {
		log.Infow("Cleaning up resiliency module metrics resources")
		for _, module := range instance.Spec.Modules {
			if module.Name == csmv1.Resiliency {
				if err := syncModuleMetricsResources(ctx, true, module, instance, ctrlClient); err != nil {
					log.Warnw("Failed to clean up resiliency module metrics resources", "error", err)
				}
				if err := syncResiliencyPrometheusRule(ctx, true, module, instance, operatorConfig, ctrlClient); err != nil {
					log.Warnw("Failed to clean up resiliency PrometheusRule", "error", err)
				}
				break
			}
		}
	}

	// Clean up replication module metrics resources
	if replicationEnabled, _ := operatorutils.IsModuleEnabled(ctx, instance, csmv1.Replication); replicationEnabled {
		log.Infow("Cleaning up replication module metrics resources")
		for _, module := range instance.Spec.Modules {
			if module.Name == csmv1.Replication {
				if err := syncModuleMetricsResources(ctx, true, module, instance, ctrlClient); err != nil {
					log.Warnw("Failed to clean up replication module metrics resources", "error", err)
				}
				if err := syncReplicationPrometheusRule(ctx, true, module, instance, operatorConfig, ctrlClient); err != nil {
					log.Warnw("Failed to clean up replication PrometheusRule", "error", err)
				}
				break
			}
		}
	}

	return nil
}

// PreChecks - validate input values
func (r *ContainerStorageModuleReconciler) PreChecks(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig) error {
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
		// zoning initially applies only to pflex
		err = r.ZoneValidation(ctx, cr)
		if err != nil {
			return fmt.Errorf("error during zone validation: %v", err)
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
	case csmv1.Cosi:
		err := drivers.PrecheckCosi(ctx, cr, operatorConfig, r.GetClient())
		if err != nil {
			return fmt.Errorf("failed cosi validation: %v", err)
		}
	default:
		// Go to checkUpgrade if it is standalone module i.e. authorization proxy server
		if cr.HasModule(csmv1.AuthorizationServer) {
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

	// Check if valid custom registry is mentioned
	err = operatorutils.ValidateCustomRegistry(ctx, cr.Spec.CustomRegistry)
	if err != nil {
		return fmt.Errorf("failed custom registry validation: %v", err)
	}

	// Validate all user-supplied image overrides and customRegistry against
	// the operator-managed image allowlist. This prevents arbitrary image
	// injection through the configVersion path and custom registry abuse
	// through the spec.version path.
	allowlist, loadErr := operatorutils.LoadImageAllowlist(ctx, r.GetClient(), operatorutils.GetOperatorNamespace())
	if loadErr != nil {
		return fmt.Errorf("failed to load image allowlist: %v", loadErr)
	}
	if validateErr := operatorutils.ValidateImageOverrides(ctx, cr, allowlist); validateErr != nil {
		return fmt.Errorf("failed image allowlist validation: %v", validateErr)
	}

	// When spec.version is set, resolve images from the operator-managed
	// csm-images ConfigMap and validate those images against the same
	// allowlist. This ensures that operator-configured image mappings cannot
	// bypass the registry allowlist enforced on CR-supplied images.
	if cr.Spec.Version != "" {
		matched, err := operatorutils.ResolveVersionFromConfigMap(ctx, r.GetClient(), cr)
		if err != nil {
			return fmt.Errorf("failed to resolve version from csm-images ConfigMap: %v", err)
		}
		if err := operatorutils.ValidateVersionSpecImages(ctx, matched, allowlist); err != nil {
			return err
		}
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
			case csmv1.ReverseProxy:
				if err := modules.ReverseProxyPrecheck(ctx, operatorConfig, m, *cr, r); err != nil {
					return fmt.Errorf("failed reverseproxy validation: %v", err)
				}
			default:
				return fmt.Errorf("unsupported module type %s", m.Name)
			}
		}
	}

	// Check if CSI Addons is enabled via environment variable and run precheck
	if modules.IsCSIAddonsReplicationEnabledViaEnv(*cr) {
		if err := modules.CSIAddonsReplicationPrecheck(ctx, cr); err != nil {
			return fmt.Errorf("failed csi-addons replication validation: %v", err)
		}
	}

	return nil
}

// Check for upgrade/if upgrade is appropriate
func (r *ContainerStorageModuleReconciler) checkUpgrade(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig) (bool, error) {
	log := logger.GetLogger(ctx)
	newVersion := ""

	// If it is an upgrade/downgrade, check to see if we meet the minimum version
	// using operatorutils.IsValidUpgrade, which resolves minUpgradeFrom from
	// operatorconfig/common/csm-releases.yaml. If the upgrade path is not valid fail.
	// Existing version
	annotations := cr.GetAnnotations()
	oldVersion, configVersionExists := annotations[configVersionKey]
	// If annotation exists, we are doing an upgrade or modify
	if configVersionExists {
		if cr.HasModule(csmv1.AuthorizationServer) {
			// if spec.version is set in the CR (CSM v1.16.0 or later), use that
			if cr.Spec.Version != "" {
				ver, err := operatorutils.GetVersion(ctx, cr, operatorConfig)
				if err != nil {
					return false, err
				}
				newVersion = ver
			} else if cr.GetModule(csmv1.AuthorizationServer).ConfigVersion != "" {
				newVersion = cr.GetModule(csmv1.AuthorizationServer).ConfigVersion
			}

			if strings.HasPrefix(oldVersion, "v1.") && strings.HasPrefix(newVersion, "v2.") ||
				strings.HasPrefix(oldVersion, "v2.") && strings.HasPrefix(newVersion, "v1.") {
				log.Error("Cannot switch between Authorization v1 and v2")
				return false, nil
			}
			return operatorutils.IsValidUpgrade(ctx, oldVersion, newVersion, csmv1.Authorization, operatorConfig)
		}
		driverType := cr.Spec.Driver.CSIDriverType
		if driverType == csmv1.PowerScale {
			// use powerscale instead of isilon as the folder name is powerscale
			driverType = csmv1.PowerScaleName
		}
		newVersion, err := operatorutils.GetVersion(ctx, cr, operatorConfig)
		if err != nil {
			return false, err
		}
		return operatorutils.IsValidUpgrade(ctx, oldVersion, newVersion, driverType, operatorConfig)

	}
	log.Infow("proceeding with fresh driver install")
	return true, nil
}

// savePreUpgradeSnapshot saves the pre-upgrade CR spec as a JSON annotation before an upgrade is applied.
// It reads the old spec from the PreviouslyAppliedConfiguration annotation, since cr.Spec already
// contains the new (desired) version by the time the reconciler runs.
// Returns true if the annotation was set, false if it already exists (idempotent).
func savePreUpgradeSnapshot(ctx context.Context, cr *csmv1.ContainerStorageModule) (bool, error) {
	log := logger.GetLogger(ctx)

	annotations := cr.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Idempotent: skip if snapshot already exists
	if _, exists := annotations[preUpgradeSnapshotKey]; exists {
		log.Info("Pre-upgrade snapshot already exists, skipping")
		return false, nil
	}

	// Use the previously applied CR to get the old spec (before the user changed it)
	oldCrJSON, ok := annotations[previouslyAppliedCustomResource]
	if !ok || oldCrJSON == "" {
		log.Info("No previously applied configuration found, skipping snapshot")
		return false, nil
	}

	oldCR := new(csmv1.ContainerStorageModule)
	if err := json.Unmarshal([]byte(oldCrJSON), oldCR); err != nil {
		return false, fmt.Errorf("failed to deserialize previously applied configuration for snapshot: %w", err)
	}

	specJSON, err := json.Marshal(oldCR.Spec)
	if err != nil {
		return false, fmt.Errorf("failed to serialize old CR spec for snapshot: %w", err)
	}

	annotations[preUpgradeSnapshotKey] = string(specJSON)
	cr.SetAnnotations(annotations)
	log.Infof("Saved pre-upgrade snapshot for %s", cr.GetName())
	return true, nil
}

// clearPreUpgradeSnapshot removes the pre-upgrade snapshot annotation after a successful upgrade.
// It is safe to call when no snapshot exists (no-op). Uses retry-on-conflict for safe Update.
// If all retries are exhausted (e.g., concurrent Status().Update() from informer handlers
// keeps changing resourceVersion), the caller requeues the reconcile to retry later.
func (r *ContainerStorageModuleReconciler) clearPreUpgradeSnapshot(ctx context.Context, cr *csmv1.ContainerStorageModule) error {
	log := logger.GetLogger(ctx)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latestCSM := &csmv1.ContainerStorageModule{}
		if err := r.GetClient().Get(ctx, t1.NamespacedName{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		}, latestCSM); err != nil {
			return err
		}

		annotations := latestCSM.GetAnnotations()
		if annotations == nil {
			return nil
		}
		if _, exists := annotations[preUpgradeSnapshotKey]; !exists {
			return nil
		}

		delete(annotations, preUpgradeSnapshotKey)
		latestCSM.SetAnnotations(annotations)
		if err := r.GetClient().Update(ctx, latestCSM); err != nil {
			return err
		}

		log.Infof("Cleared pre-upgrade snapshot for %s", cr.GetName())
		return nil
	})
}

// resolveSnapshotVersion determines the config version from a snapshot spec.
func resolveSnapshotVersion(ctx context.Context, cr *csmv1.ContainerStorageModule, snapshotSpec csmv1.ContainerStorageModuleSpec, operatorConfig operatorutils.OperatorConfig) (string, error) {
	snapshotVersion := snapshotSpec.Driver.ConfigVersion
	if snapshotVersion == "" && snapshotSpec.Version != "" {
		tempCR := cr.DeepCopy()
		tempCR.Spec = snapshotSpec
		ver, err := operatorutils.GetVersion(ctx, tempCR, operatorConfig)
		if err != nil {
			return "", fmt.Errorf("rollback blocked: failed to resolve snapshot version: %w", err)
		}
		snapshotVersion = ver
	}
	return snapshotVersion, nil
}

// validateRollbackPath checks that the rollback target version is within N-2 of the current version.
func validateRollbackPath(ctx context.Context, cr *csmv1.ContainerStorageModule, currentVersion, snapshotVersion string, operatorConfig operatorutils.OperatorConfig) error {
	var isValid bool
	var err error

	if cr.HasModule(csmv1.AuthorizationServer) {
		// Authorization upgrade-path lives under moduleconfig/authorization/,
		// so validate with ModuleType (same as the normal upgrade path).
		isValid, err = operatorutils.IsValidUpgrade(ctx, currentVersion, snapshotVersion, csmv1.Authorization, operatorConfig)
	} else {
		driverType := cr.Spec.Driver.CSIDriverType
		if driverType == csmv1.PowerScale {
			driverType = csmv1.PowerScaleName
		}
		isValid, err = operatorutils.IsValidUpgrade(ctx, currentVersion, snapshotVersion, driverType, operatorConfig)
	}

	if err != nil || !isValid {
		return fmt.Errorf("rollback blocked: target version %s is outside supported N-2 range from %s: %w", snapshotVersion, currentVersion, err)
	}
	return nil
}

// attemptRollback reads the pre-upgrade snapshot and restores the CR spec.
// Returns true if rollback was initiated, false if rollback was blocked.
func (r *ContainerStorageModuleReconciler) attemptRollback(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig) error {
	log := logger.GetLogger(ctx)

	annotations := cr.GetAnnotations()
	snapshotJSON, exists := annotations[preUpgradeSnapshotKey]
	if !exists || snapshotJSON == "" {
		return fmt.Errorf("rollback blocked: no pre-upgrade snapshot available")
	}

	var snapshotSpec csmv1.ContainerStorageModuleSpec
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshotSpec); err != nil {
		return fmt.Errorf("rollback blocked: failed to deserialize snapshot: %w", err)
	}

	snapshotVersion, err := resolveSnapshotVersion(ctx, cr, snapshotSpec, operatorConfig)
	if err != nil {
		return err
	}
	if snapshotVersion == "" {
		return fmt.Errorf("rollback blocked: snapshot has empty version")
	}

	currentVersion, hasCurrentVersion := annotations[configVersionKey]
	if !hasCurrentVersion || currentVersion == "" {
		return fmt.Errorf("rollback blocked: no current version annotation found")
	}

	if err := validateRollbackPath(ctx, cr, currentVersion, snapshotVersion, operatorConfig); err != nil {
		return err
	}

	// Restore CR spec from snapshot using retry on conflict to handle stale resource version
	log.Infof("Initiating rollback from %s to %s for %s", currentVersion, snapshotVersion, cr.GetName())
	var rollbackErr error
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latestCSM := &csmv1.ContainerStorageModule{}
		err := r.GetClient().Get(ctx, t1.NamespacedName{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		}, latestCSM)
		if err != nil {
			return err
		}

		latestAnnotations := latestCSM.GetAnnotations()
		if latestAnnotations == nil {
			latestAnnotations = make(map[string]string)
		}

		// Verify snapshot still exists (idempotency check)
		if _, exists := latestAnnotations[preUpgradeSnapshotKey]; !exists {
			rollbackErr = fmt.Errorf("rollback blocked: snapshot no longer exists (may have been already processed)")
			return nil
		}

		latestCSM.Spec = snapshotSpec
		// Delete snapshot annotation so the status cap logic stops forcing Pending
		delete(latestAnnotations, preUpgradeSnapshotKey)
		// Update configVersionKey to the rollback target so the next reconcile
		// doesn't see a version mismatch and treat the rollback as a new upgrade
		latestAnnotations[configVersionKey] = snapshotVersion
		latestCSM.SetAnnotations(latestAnnotations)

		return r.GetClient().Update(ctx, latestCSM)
	})
	if rollbackErr != nil {
		return rollbackErr
	}
	if err != nil {
		return fmt.Errorf("rollback failed: unable to update CR: %w", err)
	}

	r.EventRecorder.Eventf(cr, corev1.EventTypeNormal, csmv1.EventUpdated,
		"Rollback initiated: restoring to version %s", snapshotVersion)

	log.Infof("Rollback initiated for %s, restored to version %s", cr.GetName(), snapshotVersion)
	return nil
}

// handleAutoUpgrade checks if a newer CSM version is available for the driver
// and updates the CR's spec.Version to trigger the upgrade flow. It only proceeds
// when the CR is in Succeeded state to avoid interfering with in-progress operations.
// Returns (true, nil) if the CR was updated, (false, nil) if no upgrade needed,
// or (false, err) on error.
func (r *ContainerStorageModuleReconciler) handleAutoUpgrade(ctx context.Context, csm *csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig) (bool, error) {
	log := logger.GetLogger(ctx)

	// Only auto-upgrade when the CR is in Succeeded state to avoid
	// interfering with in-progress upgrades, rollbacks, or pending operations.
	if csm.Status.State != constants.Succeeded {
		log.Debugw("Skipping auto-upgrade: CR is not in Succeeded state", "state", csm.Status.State)
		return false, nil
	}

	// Skip if an upgrade is already in progress (snapshot exists)
	annotations := csm.GetAnnotations()
	if annotations != nil {
		if _, hasSnapshot := annotations[preUpgradeSnapshotKey]; hasSnapshot {
			log.Debugw("Skipping auto-upgrade: pre-upgrade snapshot exists, upgrade/rollback in progress")
			return false, nil
		}
	}

	driverType := csm.Spec.Driver.CSIDriverType
	latestCSMVersion, err := operatorutils.GetLatestCSMVersion(ctx, driverType, operatorConfig)
	if err != nil {
		return false, fmt.Errorf("failed to determine latest CSM version for %s: %w", driverType, err)
	}

	currentVersion := csm.Spec.Version
	if currentVersion == "" {
		log.Infow("Auto-upgrade requires spec.version to be set; skipping auto-upgrade for configVersion-based CR")
		return false, nil
	}

	if currentVersion == latestCSMVersion {
		log.Debugw("Auto-upgrade: CR is already at the latest version", "version", currentVersion)
		return false, nil
	}

	// Validate the upgrade path before proceeding
	currentConfigVersion, err := operatorutils.GetVersion(ctx, csm, operatorConfig)
	if err != nil {
		return false, fmt.Errorf("failed to resolve current config version: %w", err)
	}

	// Temporarily set the version to latest to resolve target config version
	origVersion := csm.Spec.Version
	csm.Spec.Version = latestCSMVersion
	targetConfigVersion, err := operatorutils.GetVersion(ctx, csm, operatorConfig)
	csm.Spec.Version = origVersion // restore
	if err != nil {
		return false, fmt.Errorf("failed to resolve target config version for %s: %w", latestCSMVersion, err)
	}

	isValid, err := operatorutils.IsValidUpgrade(ctx, currentConfigVersion, targetConfigVersion, driverType, operatorConfig)
	if err != nil || !isValid {
		log.Infow("Auto-upgrade: upgrade path is not valid, skipping",
			"from", currentVersion, "to", latestCSMVersion, "error", err)
		r.EventRecorder.Eventf(csm, corev1.EventTypeWarning, csmv1.EventUpdated,
			"Auto-upgrade from %s to %s is not a valid upgrade path; skipping", currentVersion, latestCSMVersion)
		return false, nil
	}

	// Update the CR's spec.Version to trigger the upgrade flow
	log.Infow("Auto-upgrade: upgrading", "name", csm.Name, "from", currentVersion, "to", latestCSMVersion)
	r.EventRecorder.Eventf(csm, corev1.EventTypeNormal, csmv1.EventUpdated,
		"Auto-upgrade initiated: upgrading from %s to %s", currentVersion, latestCSMVersion)

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latestCSM := &csmv1.ContainerStorageModule{}
		err := r.GetClient().Get(ctx, t1.NamespacedName{
			Name:      csm.Name,
			Namespace: csm.Namespace,
		}, latestCSM)
		if err != nil {
			return err
		}

		latestCSM.Spec.Version = latestCSMVersion
		return r.GetClient().Update(ctx, latestCSM)
	})
	if err != nil {
		return false, fmt.Errorf("failed to update CR with new version %s: %w", latestCSMVersion, err)
	}

	return true, nil
}

// emitPlatformWarning emits a platform-specific warning event during upgrade.
func emitPlatformWarning(ctx context.Context, cr *csmv1.ContainerStorageModule, recorder record.EventRecorder) {
	log := logger.GetLogger(ctx)

	var message string
	switch cr.Spec.Driver.CSIDriverType {
	case csmv1.PowerMax:
		message = fmt.Sprintf("WARNING: Verify Unisphere version compatibility with CSM %s per the Dell CSM Support Matrix before proceeding.", cr.Spec.Version)
	case csmv1.PowerFlex:
		message = fmt.Sprintf("WARNING: Verify MDM version compatibility with CSM %s per the Dell CSM Support Matrix before proceeding. SDC versions on worker nodes will be validated by the operator.", cr.Spec.Version)
	case csmv1.PowerScale:
		message = fmt.Sprintf("WARNING: Verify OneFS version compatibility with CSM %s per the Dell CSM Support Matrix before proceeding.", cr.Spec.Version)
	case csmv1.PowerStore:
		message = fmt.Sprintf("WARNING: Verify storage array software compatibility with CSM %s per the Dell CSM Support Matrix before proceeding.", cr.Spec.Version)
	default:
		return
	}

	log.Info(message)
	recorder.Event(cr, corev1.EventTypeWarning, csmv1.EventUpdated, message)
}

// applyConfigVersionAnnotations - applies the config version annotation to the instance.
func applyConfigVersionAnnotations(ctx context.Context, instance *csmv1.ContainerStorageModule, op operatorutils.OperatorConfig) bool {
	log := logger.GetLogger(ctx)

	annotations := instance.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	var configVersion string
	var err error

	configVersion, err = operatorutils.GetVersion(ctx, instance, op)
	if err != nil {
		return false
	}

	annotations[CSMVersionKey] = CSMVersion
	if instance.Spec.Version != "" {
		annotations[CSMVersionKey] = instance.Spec.Version
	}

	if annotations[configVersionKey] != configVersion {
		annotations[configVersionKey] = configVersion
		log.Infof("Installing csm component [%s] with config Version [%s]. Updating Annotations with Config Version",
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

// GetConfig - Returns the operator config
func (r *ContainerStorageModuleReconciler) GetConfig() operatorutils.OperatorConfig {
	return r.Config
}

// ZoneValidation - If zones are configured performs validation and returns an error if the zone validation fails
func (r *ContainerStorageModuleReconciler) ZoneValidation(ctx context.Context, cr *csmv1.ContainerStorageModule) error {
	err := drivers.ValidateZones(ctx, cr, r.Client)
	if err != nil {
		return fmt.Errorf("zone validation failed with error: %v", err)
	}

	return err
}

// volumeJournalCRDManifest is the file name for the VolumeJournal CRD bundled
// inside each PowerMax driver version directory.
const (
	volumeJournalCRDManifest    = "volumejournal-crd.yaml"
	maxVolumeJournalCRDFileSize = 1 << 20
)

// applyVolumeJournalCRD installs or removes the VolumeJournal CRD that the
// PowerMax Metro site-failure feature relies on. The CRD YAML is read from the
// driver version directory (driverconfig/powermax/{version}/) so that the CRD
// schema stays in lock-step with the driver version deployed by the operator.
func applyVolumeJournalCRD(ctx context.Context, cr csmv1.ContainerStorageModule, isDeleting bool, op operatorutils.OperatorConfig, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	version, err := operatorutils.GetVersion(ctx, &cr, op)
	if err != nil {
		return err
	}

	crdPath := filepath.Join(op.ConfigDirectory, "driverconfig", "powermax", version, volumeJournalCRDManifest)
	file, err := os.Open(filepath.Clean(crdPath))
	if err != nil {
		return fmt.Errorf("failed to read VolumeJournal CRD from %s: %v", crdPath, err)
	}
	defer file.Close()

	buf, err := io.ReadAll(io.LimitReader(file, maxVolumeJournalCRDFileSize+1))
	if err != nil {
		return fmt.Errorf("failed to read VolumeJournal CRD from %s: %v", crdPath, err)
	}
	if len(buf) > maxVolumeJournalCRDFileSize {
		return fmt.Errorf("VolumeJournal CRD at %s exceeds the maximum allowed size of %d bytes", crdPath, maxVolumeJournalCRDFileSize)
	}

	crdObjects, err := operatorutils.GetModuleComponentObj(buf)
	if err != nil {
		return fmt.Errorf("failed to parse VolumeJournal CRD: %v", err)
	}

	for _, crdObj := range crdObjects {
		if isDeleting {
			log.Infoln("Deleting VolumeJournal CRD for PowerMax Metro")
			if err := operatorutils.DeleteObject(ctx, crdObj, ctrlClient); err != nil {
				return err
			}
		} else {
			log.Infoln("Applying VolumeJournal CRD for PowerMax Metro")
			if err := operatorutils.ApplyObject(ctx, crdObj, ctrlClient); err != nil {
				return err
			}
		}
	}
	return nil
}

func applyCSMDRCRD(ctx context.Context, cr csmv1.ContainerStorageModule, isDeleting bool, op operatorutils.OperatorConfig, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	version, err := operatorutils.GetVersion(ctx, &cr, op)
	if err != nil {
		return err
	}
	// CSM DR is only compatible starting with v2.16.0.
	isCompatible, err := operatorutils.MinVersionCheck(constants.DisasterRecoveryMinVersion, version)
	if err != nil {
		return fmt.Errorf("error checking version: %s", version)
	}

	if !isCompatible {
		log.Warnf("CSM Disaster Recovery (DR) is not compatible with version %s for %s", version, cr.Spec.Driver.CSIDriverType)

		// Delete CSM DR CRDs if we are downgrading.
		if err := modules.PatchCSMDRCRDs(ctx, true, op, ctrlClient); err != nil {
			return fmt.Errorf("unable to remove the common CSM DR Controller: %v", err)
		}
		return nil
	}

	log.Infoln("Applying the CSM Disaster Recovery (DR) CRDs")
	if err := modules.PatchCSMDRCRDs(ctx, isDeleting, op, ctrlClient); err != nil {
		return fmt.Errorf("unable to patch the common CSM Disaster Recovery (DR) Controller: %v", err)
	}

	return nil
}
