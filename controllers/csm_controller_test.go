package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/tests/shared"
	"github.com/dell/csm-operator/tests/shared/clientgoclient"
	"github.com/dell/csm-operator/tests/shared/crclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	opts = zap.Options{
		Development: true,
	}

	unittestLogger = zap.New(zap.UseFlagOptions(&opts)).WithName("controllers").WithName("unit-test")

	ctx = context.Background()

	createCMError    bool
	createCMErrorStr = "unable to create ConfigMap"

	getCMError    bool
	getCMErrorStr = "unable to get ConfigMap"

	updateCSMError    bool
	updateCSMErrorStr = "unable to get CSM"

	updateCMError    bool
	updateCMErrorStr = "unable to update ConfigMap"

	createCSIError    bool
	createCSIErrorStr = "unable to create Csidriver"

	getCSIError    bool
	getCSIErrorStr = "unable to get Csidriver"

	updateCSIError    bool
	updateCSIErrorStr = "unable to update Csidriver"

	getCRError    bool
	getCRErrorStr = "unable to get Clusterrole"

	updateCRError    bool
	updateCRErrorStr = "unable to update Clusterrole"

	createCRError    bool
	createCRErrorStr = "unable to create Clusterrole"

	getCRBError    bool
	getCRBErrorStr = "unable to get ClusterroleBinding"

	updateCRBError    bool
	updateCRBErrorStr = "unable to update Clusterroleinding"

	createCRBError    bool
	createCRBErrorStr = "unable to create ClusterroleBinding"

	createSAError    bool
	createSAErrorStr = "unable to create ServiceAccount"

	getSAError    bool
	getSAErrorStr = "unable to get ServiceAccount"

	updateSAError    bool
	updateSAErrorStr = "unable to update ServiceAccount"

	updateDSError    bool
	updateDSErrorStr = "unable to update Daemonset"

	deleteDSError    bool
	deleteDSErrorStr = "unable to delete Daemonset"

	deleteDeploymentError    bool
	deleteDeploymentErrorStr = "unable to delete Deployment"

	deleteSAError    bool
	deleteSAErrorStr = "unable to delete ServiceAccount"

	csmName = "csm"

	configVersion = shared.ConfigVersion

	req = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "test",
			Name:      csmName,
		},
	}

	operatorConfig = utils.OperatorConfig{
		ConfigDirectory: "../operatorconfig",
	}
)

// CSMContrllerTestSuite implements testify suite
// opeartorClient is the client for controller runtime
// k8sClient is the client for client go kubernetes, which
// is responsible for creating daemonset/deployment Interface and apply operations
// It also implements ErrorInjector interface so that we can force error
type CSMControllerTestSuite struct {
	suite.Suite
	fakeClient client.Client
	k8sClient  kubernetes.Interface
	namespace  string
}

// init every test
func (suite *CSMControllerTestSuite) SetupTest() {
	ctrl.SetLogger(unittestLogger)

	unittestLogger.Info("Init unit test...")

	csmv1.AddToScheme(scheme.Scheme)

	objects := map[shared.StorageKey]runtime.Object{}
	suite.fakeClient = crclient.NewFakeClient(objects, suite)
	suite.k8sClient = clientgoclient.NewFakeClient(suite.fakeClient)

	suite.namespace = "test"
}

// test a happy path scenerio with deletion
func (suite *CSMControllerTestSuite) TestReconcile() {
	suite.makeFakeCSM(csmName, suite.namespace, true, getReplicaModule())
	suite.runFakeCSMManager("", false)
	suite.deleteCSM(csmName)
	suite.runFakeCSMManager("", true)
}

// test error injection. Client get should fail
func (suite *CSMControllerTestSuite) TestErrorInjection() {
	// test csm not found. err should be nil
	suite.runFakeCSMManager("", true)
	// make a csm without finalizer
	suite.makeFakeCSM(csmName, suite.namespace, false, getAuthModule())
	suite.reconcileWithErrorInjection(csmName, "")
}

func (suite *CSMControllerTestSuite) TestCsmAnnotation() {

	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.PowerScale

	csm.ObjectMeta.Finalizers = []string{CSMFinalizerName}

	suite.fakeClient.Create(ctx, &csm)
	sec := shared.MakeSecret(csmName+"-creds", suite.namespace, configVersion)
	suite.fakeClient.Create(ctx, sec)

	reconciler := suite.createReconciler()
	updateCSMError = true
	_, err := reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	updateCSMError = false

}

func (suite *CSMControllerTestSuite) TestCsmFinalizerError() {

	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.ObjectMeta.Finalizers = []string{"foo"}
	suite.fakeClient.Create(ctx, &csm)
	reconciler := suite.createReconciler()
	updateCSMError = true
	_, err := reconciler.Reconcile(ctx, req)
	assert.NotNil(suite.T(), err)
	updateCSMError = false
}

// Test all edge cases in RevoveDriver
func (suite *CSMControllerTestSuite) TestRemoveDriver() {
	r := suite.createReconciler()
	csmWoType := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Driver.CSIDriverType = "powerscale"

	removeDriverTests := []struct {
		name          string
		csm           csmv1.ContainerStorageModule
		errorInjector *bool
		expectedErr   string
	}{
		{"getDriverConfig error", csmWoType, nil, "no such file or directory"},
		// can't find objects since they are not created. In this case error is nil
		{"delete obj not found", csm, nil, ""},
		{"get SA error", csm, &getSAError, getSAErrorStr},
		{"get CR error", csm, &getCRError, getCRErrorStr},
		{"get CRB error", csm, &getCRBError, getCRBErrorStr},
		{"get CM error", csm, &getCMError, getCMErrorStr},
		{"get Driver error", csm, &getCSIError, getCSIErrorStr},
		{"delete SA error", csm, &deleteSAError, deleteSAErrorStr},
		{"delete Daemonset error", csm, &deleteDSError, deleteDSErrorStr},
		{"delete Deployment error", csm, &deleteDeploymentError, deleteDeploymentErrorStr},
	}

	for _, tt := range removeDriverTests {
		suite.T().Run(tt.name, func(t *testing.T) {

			if tt.errorInjector != nil {
				// need to create all objs before running removeDriver to hit unknown error
				suite.makeFakeCSM(csmName, suite.namespace, true, getAuthModule())
				r.Reconcile(ctx, req)
				*tt.errorInjector = true
			}

			err := r.removeDriver(ctx, tt.csm, operatorConfig)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Error(t, err)
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}

			if tt.errorInjector != nil {
				*tt.errorInjector = false
				r.Client.(*crclient.Client).Clear()
			}
		})
	}

}

func (suite *CSMControllerTestSuite) TestCsmPreCheckVersionError() {

	// set bad version error
	configVersion = "v0"
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.PowerScale
	csm.Annotations[configVersionKey] = configVersion

	sec := shared.MakeSecret(csmName+"-creds", suite.namespace, configVersion)
	suite.fakeClient.Create(ctx, sec)

	csm.ObjectMeta.Finalizers = []string{CSMFinalizerName}
	suite.fakeClient.Create(ctx, &csm)
	reconciler := suite.createReconciler()

	_, err := reconciler.Reconcile(ctx, req)
	assert.NotNil(suite.T(), err)

	// set it back to good version for other tests
	suite.deleteCSM(csmName)
	reconciler = suite.createReconciler()
	_, err = reconciler.Reconcile(ctx, req)
	assert.Nil(suite.T(), err)
	configVersion = shared.ConfigVersion
}

func (suite *CSMControllerTestSuite) TestCsmPreCheckTypeError() {

	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Driver.CSIDriverType = csmv1.PowerFlex
	csm.Spec.Driver.Common.Image = "image"
	csm.Annotations[configVersionKey] = configVersion

	sec := shared.MakeSecret(csmName+"-creds", suite.namespace, configVersion)
	suite.fakeClient.Create(ctx, sec)

	csm.ObjectMeta.Finalizers = []string{CSMFinalizerName}
	suite.fakeClient.Create(ctx, &csm)
	reconciler := suite.createReconciler()

	configVersion = shared.ConfigVersion
	_, err := reconciler.Reconcile(ctx, req)
	assert.NotNil(suite.T(), err)
	// set it back to good version for other tests
	suite.deleteCSM(csmName)
	reconciler = suite.createReconciler()
	_, err = reconciler.Reconcile(ctx, req)
	assert.Nil(suite.T(), err)
	configVersion = shared.ConfigVersion
}

func (suite *CSMControllerTestSuite) TestIgnoreUpdatePredicate() {
	p := suite.createReconciler().ignoreUpdatePredicate()
	assert.NotNil(suite.T(), p)
	o := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "biz", Name: "baz"},
	}
	e := event.UpdateEvent{
		ObjectOld: o,
		ObjectNew: o,
	}
	r := p.Update(e)
	assert.NotNil(suite.T(), r)
	d := event.DeleteEvent{
		Object: o,
	}
	s := p.Delete(d)
	assert.NotNil(suite.T(), s)
}

// helper method to create and run reconciler
func TestCustom(t *testing.T) {
	testSuite := new(CSMControllerTestSuite)
	suite.Run(t, testSuite)
}

// test with a csm without a finalizer, reconcile should add it
func (suite *CSMControllerTestSuite) TestContentWatch() {
	suite.createReconciler().ContentWatch()
	expRateLimiter := workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 120*time.Second)
	suite.createReconciler().SetupWithManager(nil, expRateLimiter, 1)
	close(StopWatch)
	version, err := utils.GetModuleDefaultVersion("v2.2.0", "csi-isilon", csmv1.Authorization, "../operatorconfig")
	assert.NotNil(suite.T(), err)
	assert.NotNil(suite.T(), version)
}

func (suite *CSMControllerTestSuite) createReconciler() (reconciler *ContainerStorageModuleReconciler) {

	logType := logger.DevelopmentLogLevel
	logger.SetLoggerLevel(logType)
	_, log := logger.GetNewContextWithLogger("0")
	log.Infof("Version : %s", logType)

	reconciler = &ContainerStorageModuleReconciler{
		Client:        suite.fakeClient,
		K8sClient:     suite.k8sClient,
		Scheme:        scheme.Scheme,
		Log:           log,
		Config:        operatorConfig,
		EventRecorder: record.NewFakeRecorder(100),
	}

	return reconciler
}

func (suite *CSMControllerTestSuite) runFakeCSMManager(expectedErr string, reconcileDelete bool) {
	reconciler := suite.createReconciler()

	// invoke controller Reconcile to test. Typically k8s would call this when resource is changed
	res, err := reconciler.Reconcile(ctx, req)

	ctrl.Log.Info("reconcile response", "res is: ", res)

	if expectedErr == "" {
		assert.NoError(suite.T(), err)
	} else {
		assert.NotNil(suite.T(), err)
	}

	if err != nil {
		ctrl.Log.Error(err, "Error returned")
		assert.True(suite.T(), strings.Contains(err.Error(), expectedErr))
	}

	// after reconcile being run, we update deployment and daemonset
	// then call handleDeployment/DaemonsetUpdate explicitly because
	// in unit test listener does not get triggered
	// If delete, we shouldn't call these methods since reconcile
	// would return before this
	if !reconcileDelete {
		suite.handleDaemonsetTest(reconciler, "csm-node")
		suite.handleDeploymentTest(reconciler, "csm-controller")
		suite.handlePodTest(reconciler, "csm-pod")
		_, err = reconciler.Reconcile(ctx, req)
		if expectedErr == "" {
			assert.NoError(suite.T(), err)
		} else {
			assert.NotNil(suite.T(), err)
		}
	}
}

// call reconcile with different injection errors in k8s client
func (suite *CSMControllerTestSuite) reconcileWithErrorInjection(reqName, expectedErr string) {
	reconciler := suite.createReconciler()

	// create would fail
	createSAError = true
	_, err := reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), createSAErrorStr, "expected error containing %q, got %s", expectedErr, err)
	createSAError = false

	createCRError = true
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), createCRErrorStr, "expected error containing %q, got %s", expectedErr, err)
	createCRError = false

	createCRBError = true
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), createCRBErrorStr, "expected error containing %q, got %s", expectedErr, err)
	createCRBError = false

	createCSIError = true
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), createCSIErrorStr, "expected error containing %q, got %s", expectedErr, err)
	createCSIError = false

	createCMError = true
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), createCMErrorStr, "expected error containing %q, got %s", expectedErr, err)
	createCMError = false

	// create everything this time
	reconciler.Reconcile(ctx, req)

	getCSIError = true
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), getCSIErrorStr, "expected error containing %q, got %s", expectedErr, err)
	getCSIError = false

	getCMError = true
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), getCMErrorStr, "expected error containing %q, got %s", expectedErr, err)
	getCMError = false

	updateCMError = true
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), updateCMErrorStr, "expected error containing %q, got %s", expectedErr, err)
	updateCMError = false

	getCRBError = true
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), getCRBErrorStr, "expected error containing %q, got %s", expectedErr, err)
	getCRBError = false

	updateCRBError = true
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), updateCRBErrorStr, "expected error containing %q, got %s", expectedErr, err)
	updateCRBError = false

	getCRError = true
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), getCRErrorStr, "expected error containing %q, got %s", expectedErr, err)
	getCRError = false

	updateCRError = true
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), updateCRErrorStr, "expected error containing %q, got %s", expectedErr, err)
	updateCRError = false

	getSAError = true
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), getSAErrorStr, "expected error containing %q, got %s", expectedErr, err)
	getSAError = false

	updateSAError = true
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), updateSAErrorStr, "expected error containing %q, got %s", expectedErr, err)
	updateSAError = false

	updateDSError = true
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), updateDSErrorStr, "expected error containing %q, got %s", expectedErr, err)
	updateDSError = false

	deleteSAError = true
	suite.deleteCSM(csmName)
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), deleteSAErrorStr, "expected error containing %q, got %s", expectedErr, err)
	deleteSAError = false
}

func (suite *CSMControllerTestSuite) handleDaemonsetTest(r *ContainerStorageModuleReconciler, name string) {
	daemonset := &appsv1.DaemonSet{}
	err := suite.fakeClient.Get(ctx, client.ObjectKey{Namespace: suite.namespace, Name: name}, daemonset)
	assert.Nil(suite.T(), err)
	daemonset.Spec.Template.Labels = map[string]string{"csm": "csm"}

	r.handleDaemonsetUpdate(daemonset, daemonset)

	// Make Pod and set status
	pod := shared.MakePod(name, suite.namespace)
	pod.Labels["csm"] = csmName
	pod.Status.Phase = corev1.PodPending
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{
		{
			State: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{
					Reason: "test",
				},
			},
		},
	}
	err = suite.fakeClient.Create(ctx, &pod)
	assert.Nil(suite.T(), err)
	podList := &corev1.PodList{}
	err = suite.fakeClient.List(ctx, podList, nil)
	assert.Nil(suite.T(), err)
}

func (suite *CSMControllerTestSuite) handleDeploymentTest(r *ContainerStorageModuleReconciler, name string) {
	deployement := &appsv1.Deployment{}
	err := suite.fakeClient.Get(ctx, client.ObjectKey{Namespace: suite.namespace, Name: name}, deployement)
	assert.Nil(suite.T(), err)
	deployement.Spec.Template.Labels = map[string]string{"csm": "csm"}

	r.handleDeploymentUpdate(deployement, deployement)

	//Make Pod and set pod status
	pod := shared.MakePod(name, suite.namespace)
	pod.Labels["csm"] = csmName
	pod.Status.Phase = corev1.PodPending
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{
		{
			State: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{
					Reason: "test",
				},
			},
		},
	}
	err = suite.fakeClient.Create(ctx, &pod)
	assert.Nil(suite.T(), err)
	podList := &corev1.PodList{}
	err = suite.fakeClient.List(ctx, podList, nil)
	assert.Nil(suite.T(), err)
}

func (suite *CSMControllerTestSuite) handlePodTest(r *ContainerStorageModuleReconciler, name string) {
	suite.makeFakePod(name, suite.namespace)
	pod := &corev1.Pod{}

	err := suite.fakeClient.Get(ctx, client.ObjectKey{Namespace: suite.namespace, Name: name}, pod)
	assert.Nil(suite.T(), err)

	// since deployments/daemonsets dont create pod in non-k8s env, we have to explicitely create pod
	r.handlePodsUpdate(pod, pod)
}

// deleteCSM sets deletionTimeStamp on the csm object and deletes it
func (suite *CSMControllerTestSuite) deleteCSM(csmName string) {
	csm := &csmv1.ContainerStorageModule{}
	key := types.NamespacedName{Namespace: suite.namespace, Name: csmName}
	err := suite.fakeClient.Get(ctx, key, csm)
	assert.Nil(suite.T(), err)

	suite.fakeClient.(*crclient.Client).SetDeletionTimeStamp(ctx, csm)

	suite.fakeClient.Delete(ctx, csm)
}

func getReplicaModule() []csmv1.Module {
	return []csmv1.Module{
		{
			Name:          csmv1.Replication,
			Enabled:       true,
			ConfigVersion: "v1.2.0",
			Components: []csmv1.ContainerTemplate{
				{
					Name: utils.ReplicationSideCarName,
				},
				{
					Name: "dell-replication-controller-manager",
					Envs: []corev1.EnvVar{
						{
							Name:  "TARGET_CLUSTERS_IDS",
							Value: "skip-replication-cluster-check",
						},
					},
				},
			},
		},
	}
}

func getAuthModule() []csmv1.Module {
	return []csmv1.Module{
		{
			Name:          csmv1.Authorization,
			Enabled:       true,
			ConfigVersion: "v1.2.0",
			Components: []csmv1.ContainerTemplate{
				{
					Name: "karavi-authorization-proxy",
					Envs: []corev1.EnvVar{
						{
							Name:  "INSECURE",
							Value: "true",
						},
					},
				},
			},
		},
	}
}

func (suite *CSMControllerTestSuite) TestDeleteErrorReconcile() {
	suite.makeFakeCSM(csmName, suite.namespace, true, getAuthModule())
	suite.runFakeCSMManager("", false)

	updateCSMError = true
	suite.deleteCSM(csmName)
	reconciler := suite.createReconciler()
	_, err := reconciler.Reconcile(ctx, req)
	assert.NotNil(suite.T(), err)
	updateCSMError = false
}

// helper method to create k8s objects
func (suite *CSMControllerTestSuite) makeFakeCSM(name, ns string, withFinalizer bool, modules []csmv1.Module) {

	// make pre-requisite secrets
	sec := shared.MakeSecret(name+"-creds", ns, configVersion)
	err := suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// this secret required by authorization module
	sec = shared.MakeSecret("karavi-authorization-config", ns, configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// this secret required by authorization module
	sec = shared.MakeSecret("proxy-authz-tokens", ns, configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// replication secrets
	sec = shared.MakeSecret("skip-replication-cluster-check", utils.ReplicationControllerNameSpace, configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	csm := shared.MakeCSM(name, ns, configVersion)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.PowerScale
	truebool := true
	sideCarObjEnabledTrue := csmv1.ContainerTemplate{
		Name:            "provisioner",
		Enabled:         &truebool,
		Image:           "image2",
		ImagePullPolicy: "IfNotPresent",
		Args:            []string{"--volume-name-prefix=k8s"},
	}
	sideCarList := []csmv1.ContainerTemplate{sideCarObjEnabledTrue}
	csm.Spec.Driver.SideCars = sideCarList
	if withFinalizer {
		csm.ObjectMeta.Finalizers = []string{CSMFinalizerName}
	}
	// remove driver when deleting csm
	csm.Spec.Driver.ForceRemoveDriver = true
	csm.Annotations[configVersionKey] = configVersion

	csm.Spec.Modules = modules

	err = suite.fakeClient.Create(ctx, &csm)
	assert.Nil(suite.T(), err)
}

func (suite *CSMControllerTestSuite) makeFakePod(name, ns string) {
	pod := shared.MakePod(name, ns)
	pod.Labels["csm"] = csmName
	err := suite.fakeClient.Create(ctx, &pod)
	assert.Nil(suite.T(), err)
}

func (suite *CSMControllerTestSuite) ShouldFail(method string, obj runtime.Object) error {
	// Needs to implement based on need
	switch v := obj.(type) {
	case *csmv1.ContainerStorageModule:
		csm := obj.(*csmv1.ContainerStorageModule)
		if method == "Update" && updateCSMError {
			fmt.Printf("[ShouldFail] force Update csm error for obj of type %+v\n", csm)
			return errors.New(updateCSMErrorStr)
		}

	case *corev1.ConfigMap:
		cm := obj.(*corev1.ConfigMap)
		if method == "Create" && createCMError {
			fmt.Printf("[ShouldFail] force create Configmap error for configmap named %+v\n", cm.Name)
			return errors.New(createCMErrorStr)
		} else if method == "Update" && updateCMError {
			fmt.Printf("[ShouldFail] force Update Configmap error for configmap named %+v\n", cm.Name)
			return errors.New(updateCMErrorStr)
		} else if method == "Get" && getCMError {
			fmt.Printf("[ShouldFail] force Get Configmap error for configmap named %+v\n", cm.Name)
			fmt.Printf("[ShouldFail] force Get Configmap error for configmap named %+v\n", v)
			return errors.New(getCMErrorStr)
		}

	case *storagev1.CSIDriver:
		csi := obj.(*storagev1.CSIDriver)
		if method == "Create" && createCSIError {
			fmt.Printf("[ShouldFail] force Create Csidriver error for csidriver named %+v\n", csi.Name)
			return errors.New(createCSIErrorStr)
		} else if method == "Update" && updateCSIError {
			fmt.Printf("[ShouldFail] force Update Csidriver error for csidriver named %+v\n", csi.Name)
			return errors.New(updateCSIErrorStr)
		} else if method == "Get" && getCSIError {
			fmt.Printf("[ShouldFail] force Get Csidriver error for csidriver named %+v\n", csi.Name)
			return errors.New(getCSIErrorStr)
		}

	case *rbacv1.ClusterRole:
		cr := obj.(*rbacv1.ClusterRole)
		if method == "Create" && createCRError {
			fmt.Printf("[ShouldFail] force Create ClusterRole error for ClusterRole named %+v\n", cr.Name)
			return errors.New(createCRErrorStr)
		} else if method == "Update" && updateCRError {
			fmt.Printf("[ShouldFail] force Update ClusterRole error for ClusterRole named %+v\n", cr.Name)
			return errors.New(updateCRErrorStr)
		} else if method == "Get" && getCRError {
			fmt.Printf("[ShouldFail] force Get ClusterRole error for ClusterRole named %+v\n", cr.Name)
			return errors.New(getCRErrorStr)
		}

	case *rbacv1.ClusterRoleBinding:
		crb := obj.(*rbacv1.ClusterRoleBinding)
		if method == "Create" && createCRBError {
			fmt.Printf("[ShouldFail] force Create ClusterRoleBinding error for ClusterRoleBinding named %+v\n", crb.Name)
			return errors.New(createCRBErrorStr)
		} else if method == "Update" && updateCRBError {
			fmt.Printf("[ShouldFail] force Update ClusterRoleBinding error for ClusterRoleBinding named %+v\n", crb.Name)
			return errors.New(updateCRBErrorStr)
		} else if method == "Get" && getCRBError {
			fmt.Printf("[ShouldFail] force Get ClusterRoleBinding error for ClusterRoleBinding named %+v\n", crb.Name)
			return errors.New(getCRBErrorStr)
		}
	case *corev1.ServiceAccount:
		sa := obj.(*corev1.ServiceAccount)
		if method == "Create" && createSAError {
			fmt.Printf("[ShouldFail] force Create ServiceAccount error for ServiceAccount named %+v\n", sa.Name)
			return errors.New(createSAErrorStr)
		} else if method == "Update" && updateSAError {
			fmt.Printf("[ShouldFail] force Update ServiceAccount error for ServiceAccount named %+v\n", sa.Name)
			return errors.New(updateSAErrorStr)
		} else if method == "Get" && getSAError {
			fmt.Printf("[ShouldFail] force Get ServiceAccount error for ServiceAccount named %+v\n", sa.Name)
			return errors.New(getSAErrorStr)
		} else if method == "Delete" && deleteSAError {
			fmt.Printf("[ShouldFail] force Delete ServiceAccount error for ServiceAccount named %+v\n", sa.Name)
			return errors.New(deleteSAErrorStr)
		}

	case *appsv1.DaemonSet:
		ds := obj.(*appsv1.DaemonSet)
		if method == "Delete" && deleteDSError {
			fmt.Printf("[ShouldFail] force delete DaemonSet error for DaemonSet named %+v\n", ds.Name)
			return errors.New(deleteDSErrorStr)
		} else if method == "Update" && updateDSError {
			fmt.Printf("[ShouldFail] force update DaemonSet error for DaemonSet named %+v\n", ds.Name)
			return errors.New(updateDSErrorStr)
		}

	case *appsv1.Deployment:
		deployment := obj.(*appsv1.Deployment)
		if method == "Delete" && deleteDeploymentError {
			fmt.Printf("[ShouldFail] force Deployment error for Deployment named %+v\n", deployment.Name)
			return errors.New(deleteDeploymentErrorStr)
		}

	default:
	}
	return nil
}

// debugFakeObjects prints the runtime objects in the fake client
func (suite *CSMControllerTestSuite) debugFakeObjects() {
	objects := suite.fakeClient.(*crclient.Client).Objects
	for key, o := range objects {
		unittestLogger.Info("found fake object ", "name", key.Name)
		unittestLogger.Info("found fake object ", "object", fmt.Sprintf("%#v", o))
	}
}
