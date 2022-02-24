package controllers

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	opts = zap.Options{
		Development: true,
	}

	unittestLogger = zap.New(zap.UseFlagOptions(&opts)).WithName("controllers").WithName("unit-test")

	ctx = context.Background()

	createCMError  bool
	getCMError     bool
	updateCMError  bool
	createCSIError bool
	getCSIError    bool
	updateCSIError bool
	getCRError     bool
	updateCRError  bool
	createCRError  bool
	getCRBError    bool
	updateCRBError bool
	createCRBError bool
	createSAError  bool
	getSAError     bool
	updateSAError  bool
	csmName        = "csm"
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

	suite.makeFakeCSM(csmName, suite.namespace, true)
	suite.runFakeCSMManager(csmName, "", false)
	suite.deleteCSM(csmName)
	suite.runFakeCSMManager(csmName, "", true)
}

// test with a csm without a finalizer, reconcile should add it
func (suite *CSMControllerTestSuite) TestAddFinalizer() {
	suite.makeFakeCSM(csmName, suite.namespace, false)
	suite.runFakeCSMManager(csmName, "", true)
}

// test error injection. Client get should fail
func (suite *CSMControllerTestSuite) TestErrorInjection() {
	suite.reconcileWithErrorInjection(csmName, "")
}

// test csm not found. err should be nil
func (suite *CSMControllerTestSuite) TestCsmNotFound() {
	suite.runFakeCSMManager(csmName, "", true)
}

func (suite *CSMControllerTestSuite) TestIgnoreUpdatePredicate() {
	suite.createReconciler().ignoreUpdatePredicate()
}

// helper method to create and run reconciler
func TestCustom(t *testing.T) {
	testSuite := new(CSMControllerTestSuite)
	suite.Run(t, testSuite)
}

func (suite *CSMControllerTestSuite) createReconciler() (reconciler *ContainerStorageModuleReconciler) {

	configDir, err := filepath.Abs("../operatorconfig")
	assert.NoError(suite.T(), err)
	operatorConfig := utils.OperatorConfig{
		ConfigDirectory: configDir,
	}

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

func (suite *CSMControllerTestSuite) runFakeCSMManager(reqName, expectedErr string, reconcileDelete bool) {
	reconciler := suite.createReconciler()

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: suite.namespace,
			Name:      reqName,
		},
	}

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
	}
}

func (suite *CSMControllerTestSuite) reconcileWithErrorInjection(reqName, expectedErr string) {
	reconciler := suite.createReconciler()

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: suite.namespace,
			Name:      reqName,
		},
	}

	// invoke controller Reconcile to test. Typically k8s would call this when resource is changed
	res, err := reconciler.Reconcile(ctx, req)

	ctrl.Log.Info("reconcile response", "res is: ", res)

	if expectedErr == "" {
		assert.NoError(suite.T(), err)
	}

	if err != nil {
		ctrl.Log.Error(err, "Error returned")
		assert.True(suite.T(), strings.Contains(err.Error(), expectedErr))
	}

	getCMError = true
	res, err = reconciler.Reconcile(ctx, req)
	getCMError = false
	createCMError = true
	res, err = reconciler.Reconcile(ctx, req)
	createCMError = false
	updateCMError = true
	res, err = reconciler.Reconcile(ctx, req)
	updateCMError = false

	getCSIError = true
	res, err = reconciler.Reconcile(ctx, req)
	getCSIError = false
	createCSIError = true
	res, err = reconciler.Reconcile(ctx, req)
	createCSIError = false
	updateCSIError = true
	res, err = reconciler.Reconcile(ctx, req)
	updateCSIError = false

	getCRError = true
	res, err = reconciler.Reconcile(ctx, req)
	getCRError = false
	createCRError = true
	res, err = reconciler.Reconcile(ctx, req)
	createCRError = false
	updateCRError = true
	res, err = reconciler.Reconcile(ctx, req)
	updateCRError = false

	getCRBError = true
	res, err = reconciler.Reconcile(ctx, req)
	getCRBError = false
	createCRBError = true
	res, err = reconciler.Reconcile(ctx, req)
	createCRBError = false
	updateCRBError = true
	res, err = reconciler.Reconcile(ctx, req)
	updateCRBError = false

	getSAError = true
	res, err = reconciler.Reconcile(ctx, req)
	getSAError = false
	createSAError = true
	res, err = reconciler.Reconcile(ctx, req)
	createSAError = false
	updateSAError = true
	res, err = reconciler.Reconcile(ctx, req)
	updateSAError = false
}

func (suite *CSMControllerTestSuite) handleDaemonsetTest(r *ContainerStorageModuleReconciler, name string) {
	daemonset := &appsv1.DaemonSet{}
	err := suite.fakeClient.Get(ctx, client.ObjectKey{Namespace: suite.namespace, Name: name}, daemonset)
	assert.Nil(suite.T(), err)
	daemonset.Spec.Template.Labels = map[string]string{"csm": "csm"}

	r.handleDaemonsetUpdate(daemonset, daemonset)
}

func (suite *CSMControllerTestSuite) handleDeploymentTest(r *ContainerStorageModuleReconciler, name string) {
	deployement := &appsv1.Deployment{}
	err := suite.fakeClient.Get(ctx, client.ObjectKey{Namespace: suite.namespace, Name: name}, deployement)
	assert.Nil(suite.T(), err)
	deployement.Spec.Template.Labels = map[string]string{"csm": "csm"}

	r.handleDeploymentUpdate(deployement, deployement)
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

// helper method to create k8s objects
func (suite *CSMControllerTestSuite) makeFakeCSM(name, ns string, withFinalizer bool) {
	configVersion := shared.ConfigVersion

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

	csm := shared.MakeCSM(name, ns, configVersion)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.PowerScale
	if withFinalizer {
		csm.ObjectMeta.Finalizers = []string{CSMFinalizerName}
	}
	// remove driver when deleting csm
	csm.Spec.Driver.ForceRemoveDriver = true
	csm.Annotations[configVersionKey] = configVersion

	addModuleToCSM(&csm)

	err = suite.fakeClient.Create(ctx, &csm)
	assert.Nil(suite.T(), err)
}

func addModuleToCSM(csm *csmv1.ContainerStorageModule) {
	// add modules
	csm.Spec.Modules = []csmv1.Module{
		{
			Name:          "authorization",
			Enabled:       true,
			ConfigVersion: "v1.0.0",
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

func (suite *CSMControllerTestSuite) makeFakePod(name, ns string) {
	pod := shared.MakePod(name, ns)
	pod.Labels["csm"] = csmName
	err := suite.fakeClient.Create(ctx, &pod)
	assert.Nil(suite.T(), err)
}

func (suite *CSMControllerTestSuite) ShouldFail(method string, obj runtime.Object) error {
	// Needs to implement based on need
	switch v := obj.(type) {
	case *corev1.ConfigMap:
		cm := obj.(*corev1.ConfigMap)
		if method == "Create" && createCMError {
			fmt.Printf("[ShouldFail] force Configmap error for configmap named %+v\n", cm.Name)
			fmt.Printf("[ShouldFail] force Configmap error for obj of type %+v\n", v)
			return errors.New("unable to create ConfigMap")
		} else if method == "Update" && updateCMError {
			fmt.Printf("[ShouldFail] force update configmap error for obj of type %+v\n", v)
			return errors.New("unable to update ConfigMap")
		} else if method == "Get" && getCMError {
			fmt.Printf("[ShouldFail] force get configmap error for obj of type %+v\n", v)
			return errors.New("unable to get ConfigMap")
		}
	case *storagev1.CSIDriver:
		csi := obj.(*storagev1.CSIDriver)
		if method == "Create" && createCSIError {
			fmt.Printf("[ShouldFail] force Csidriver error for csidriver named %+v\n", csi.Name)
			fmt.Printf("[ShouldFail] force Csidriver error for obj of type %+v\n", v)
			return errors.New("unable to create Csidriver")
		} else if method == "Update" && updateCSIError {
			fmt.Printf("[ShouldFail] force update Csidriver error for obj of type %+v\n", v)
			return errors.New("unable to update Csidriver")
		} else if method == "Get" && getCSIError {
			fmt.Printf("[ShouldFail] force get Csidriver error for obj of type %+v\n", v)
			return errors.New("unable to get Csidriver")
		}
	case *rbacv1.ClusterRole:
		cr := obj.(*rbacv1.ClusterRole)
		if method == "Create" && createCRError {
			fmt.Printf("[ShouldFail] force ClusterRole error for ClusterRole named %+v\n", cr.Name)
			fmt.Printf("[ShouldFail] force ClusterRole error for obj of type %+v\n", v)
			return errors.New("unable to create ClusterRole")
		} else if method == "Update" && updateCRError {
			fmt.Printf("[ShouldFail] force update ClusterRole error for obj of type %+v\n", v)
			return errors.New("unable to update ClusterRole")
		} else if method == "Get" && getCRError {
			fmt.Printf("[ShouldFail] force get ClusterRole error for obj of type %+v\n", v)
			return errors.New("unable to get ClusterRole")
		}
	case *rbacv1.ClusterRoleBinding:
		crb := obj.(*rbacv1.ClusterRoleBinding)
		if method == "Create" && createCRBError {
			fmt.Printf("[ShouldFail] force ClusterRoleBinding error for ClusterRoleBinding named %+v\n", crb.Name)
			fmt.Printf("[ShouldFail] force ClusterRoleBinding error for obj of type %+v\n", v)
			return errors.New("unable to create ClusterRoleBinding")
		} else if method == "Update" && updateCRBError {
			fmt.Printf("[ShouldFail] force update ClusterRoleBinding error for obj of type %+v\n", v)
			return errors.New("unable to update ClusterRoleBinding")
		} else if method == "Get" && getCRBError {
			fmt.Printf("[ShouldFail] force get ClusterRoleBinding error for obj of type %+v\n", v)
			return errors.New("unable to get ClusterRoleBinding")
		}
	case *corev1.ServiceAccount:
		sa := obj.(*corev1.ServiceAccount)
		if method == "Create" && createSAError {
			fmt.Printf("[ShouldFail] force ServiceAccount error for ServiceAccount named %+v\n", sa.Name)
			fmt.Printf("[ShouldFail] force ServiceAccount error for obj of type %+v\n", v)
			return errors.New("unable to create ServiceAccount")
		} else if method == "Update" && updateSAError {
			fmt.Printf("[ShouldFail] force update ServiceAccount error for obj of type %+v\n", v)
			return errors.New("unable to update ServiceAccount")
		} else if method == "Get" && getSAError {
			fmt.Printf("[ShouldFail] force get ServiceAccount error for obj of type %+v\n", v)
			return errors.New("unable to get ServiceAccount")
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
