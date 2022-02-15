package controllers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/test/shared"
	"github.com/dell/csm-operator/test/shared/clientgoclient"
	"github.com/dell/csm-operator/test/shared/crclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
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

func (suite *CSMControllerTestSuite) TestReconcile() {
	suite.makeFakeCSM("csm", suite.namespace)
	suite.runFakeCSMManager("csm", suite.namespace)
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

func (suite *CSMControllerTestSuite) runFakeCSMManager(reqName, expectedErr string) {
	reconciler := suite.createReconciler()

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: suite.namespace,
			Name:      reqName,
		},
	}

	// invoke controller Reconcile to test. Typically k8s would call this when resource is changed
	res, err := reconciler.Reconcile(context.Background(), req)

	ctrl.Log.Info("reconcile response", "res is: ", res)

	if expectedErr == "" {
		assert.NoError(suite.T(), err)
	}

	if err != nil {
		ctrl.Log.Error(err, "Error returned")
		assert.True(suite.T(), strings.Contains(err.Error(), expectedErr))
	}

	// after reconcile being run, we update deployment and daemonset
	// then call handleDeployment/DaemonsetUpdate explicitely because
	// in unit test listener does not get triggered
	suite.handleDaemonsetTest(reconciler, "csm-node")
	suite.handleDeploymentTest(reconciler, "csm-controller")
}

func (suite *CSMControllerTestSuite) handleDaemonsetTest(r *ContainerStorageModuleReconciler, name string) {
	daemonset := &appsv1.DaemonSet{}
	err := suite.fakeClient.Get(context.Background(), client.ObjectKey{Namespace: suite.namespace, Name: name}, daemonset)
	assert.Nil(suite.T(), err)
	daemonset.Spec.Template.Labels = map[string]string{"csm": "csm"}

	r.handleDaemonsetUpdate(daemonset, daemonset)
}

func (suite *CSMControllerTestSuite) handleDeploymentTest(r *ContainerStorageModuleReconciler, name string) {
	deployement := &appsv1.Deployment{}
	err := suite.fakeClient.Get(context.Background(), client.ObjectKey{Namespace: suite.namespace, Name: name}, deployement)
	assert.Nil(suite.T(), err)
	deployement.Spec.Template.Labels = map[string]string{"csm": "csm"}

	r.handleDeploymentUpdate(deployement, deployement)
}

// helper method to create k8s objects
func (suite *CSMControllerTestSuite) makeFakeCSM(name, ns string) {
	configVersion := shared.ConfigVersion
	csm := shared.MakeCSM(name, ns, configVersion)
	sec := shared.MakeSecret(name, ns, configVersion)
	err := suite.fakeClient.Create(context.Background(), sec)
	assert.Nil(suite.T(), err)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.PowerScale
	csm.Annotations[configVersionKey] = configVersion

	err = suite.fakeClient.Create(context.Background(), &csm)
	assert.Nil(suite.T(), err)
}

func (suite *CSMControllerTestSuite) ShouldFail(method string, obj runtime.Object) error {
	// Needs to implement based on need
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
