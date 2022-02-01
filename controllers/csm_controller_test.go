package controllers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/test/shared"
	"github.com/dell/csm-operator/test/shared/clientgoclient"
	"github.com/dell/csm-operator/test/shared/crclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
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
	fmt.Println("Init test suite...")
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

	opts := zap.Options{
		Development: true,
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	configDir, err := filepath.Abs("../operatorconfig")
	assert.NoError(suite.T(), err)
	operatorConfig := utils.OperatorConfig{
		ConfigDirectory: configDir,
	}

	reconciler = &ContainerStorageModuleReconciler{
		Client:        suite.fakeClient,
		K8sClient:     suite.k8sClient,
		Scheme:        scheme.Scheme,
		Log:           ctrl.Log.WithName("controllers").WithName("unit-test"),
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

	fmt.Printf("reconcile response res=%#v\n", res)

	if expectedErr == "" {
		assert.NoError(suite.T(), err)
	}

	if err != nil {
		fmt.Printf("Error returned is: %s", err.Error())
		assert.True(suite.T(), strings.Contains(err.Error(), expectedErr))
	}

}

// helper method to create k8s objects
func (suite *CSMControllerTestSuite) makeFakeCSM(name, ns string) {
	configVersion := "v2.0.0"
	csm := shared.MakeCSM(name, ns, configVersion)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.PowerScale
	csm.Annotations[configVersionKey] = configVersion

	err := suite.fakeClient.Create(context.Background(), &csm)
	assert.Nil(suite.T(), err)
}

func (suite *CSMControllerTestSuite) ShouldFail(method string, obj runtime.Object) error {
	// Needs to implement based on need
	return nil
}
