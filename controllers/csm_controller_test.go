package controllers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/test/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type CSMControllerTestSuite struct {
	suite.Suite
	operatorClient client.Client
	namespace      string
}

// init every test
func (suite *CSMControllerTestSuite) SetupTest() {
	fmt.Println("Init test suite...")
	csmv1.AddToScheme(shared.Scheme)

	suite.operatorClient = fake.NewClientBuilder().Build()
	csmv1.AddToScheme(suite.operatorClient.Scheme())

	suite.namespace = "test"
	fmt.Println("Init done")
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
		Client: suite.operatorClient,
		Scheme: scheme.Scheme,
		Log:    ctrl.Log.WithName("controllers").WithName("unit-test"),
		Config: operatorConfig,
	}

	return reconciler
}

func (suite *CSMControllerTestSuite) runFakeCSMManager(reqName, expectedErr string) {
	reconciler := suite.createReconciler()

	mgr, _ := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             shared.Scheme,
		MetricsBindAddress: ":8080",
		Port:               9443,
		LeaderElection:     false,
	})

	expRateLimiter := workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 120*time.Second)
	reconciler.SetupWithManager(mgr, expRateLimiter, 1)

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
	csm := shared.MakeCSM(name, ns)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.PowerScale

	err := suite.operatorClient.Create(context.Background(), &csm)
	assert.Nil(suite.T(), err)
}
