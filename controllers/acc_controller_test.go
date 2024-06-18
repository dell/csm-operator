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
	"strings"
	"testing"
	"time"

	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/workqueue"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	statefulsetpkg "github.com/dell/csm-operator/pkg/resources/statefulset"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/tests/shared"
	"github.com/dell/csm-operator/tests/shared/clientgoclient"
	"github.com/dell/csm-operator/tests/shared/crclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	configv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	corev12 "k8s.io/client-go/applyconfigurations/core/v1"
	v1 "k8s.io/client-go/applyconfigurations/meta/v1"
)

var (
	accOpts = zap.Options{
		Development: true,
	}

	accUnittestLogger = zap.New(zap.UseFlagOptions(&accOpts)).WithName("controllers").WithName("unit-test")

	accCtx = context.Background()

	createAccCMError    bool
	createAccCMErrorStr = "unable to create ConfigMap"

	getAccCMError    bool
	getAccCMErrorStr = "unable to get ConfigMap"

	updateAccCSMError    bool
	updateAccCSMErrorStr = "unable to get ACC"

	updateAccCMError    bool
	updateAccCMErrorStr = "unable to update ConfigMap"

	createAccError    bool
	createAccErrorStr = "unable to create Apex Connectivity Client"

	getAccError    bool
	getAccErrorStr = "unable to get Apex Connectivity Client"

	updateAccError    bool
	updateAccErrorStr = "unable to update Apex Connectivity Client"

	getAccCRError    bool
	getAccCRErrorStr = "unable to get Clusterrole"

	updateAccCRError    bool
	updateAccCRErrorStr = "unable to update Clusterrole"

	createAccCRError    bool
	createAccCRErrorStr = "unable to create Clusterrole"

	getAccCRBError    bool
	getAccCRBErrorStr = "unable to get ClusterroleBinding"

	updateAccCRBError    bool
	updateAccCRBErrorStr = "unable to update Clusterroleinding"

	createAccCRBError    bool
	createAccCRBErrorStr = "unable to create ClusterroleBinding"

	createAccSAError    bool
	createAccSAErrorStr = "unable to create ServiceAccount"

	getAccSAError    bool
	getAccSAErrorStr = "unable to get ServiceAccount"

	updateAccSAError    bool
	updateAccSAErrorStr = "unable to update ServiceAccount"

	deleteStatefulSetError    bool
	deleteStatefulSetErrorStr = "unable to delete Deployment"

	deleteAccSAError    bool
	deleteAccSAErrorStr = "unable to delete ServiceAccount"

	accName = "acc"

	accConfigVersion = "v1.0.0"

	accContainerName = "connectivity-client-docker-k8s"

	accReq = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "test",
			Name:      accName,
		},
	}

	accOperatorConfig = utils.OperatorConfig{
		ConfigDirectory: "../operatorconfig",
	}
	badAccOperatorConfig = utils.OperatorConfig{
		ConfigDirectory: "../in-valid-path",
	}
)

// AccContrllerTestSuite implements testify suite
// operatorClient is the client for controller runtime
// k8sClient is the client for client go kubernetes, which
// is responsible for creating daemonset/deployment Interface and apply operations
// It also implements ErrorInjector interface so that we can force error
type AccControllerTestSuite struct {
	suite.Suite
	fakeClient client.Client
	k8sClient  kubernetes.Interface
	namespace  string
}

// init every test
func (suite *AccControllerTestSuite) SetupTest() {
	ctrl.SetLogger(accUnittestLogger)

	accUnittestLogger.Info("Init Apex Connectivity Client unit test...")

	csmv1.AddToScheme(scheme.Scheme)

	objects := map[shared.StorageKey]runtime.Object{}
	suite.fakeClient = crclient.NewFakeClient(objects, suite)
	suite.k8sClient = clientgoclient.NewFakeClient(suite.fakeClient)

	suite.namespace = "test"
}

// test a happy path scenerio with deletion
func (suite *AccControllerTestSuite) TestReconcileAcc() {
	suite.makeFakeAcc(accName, suite.namespace, true)
	suite.runFakeAccManager("", false)
	suite.deleteAcc(accName)
	suite.runFakeAccManager("", true)
}

func (suite *AccControllerTestSuite) TestReconcileAccError() {
	suite.makeFakeAcc(accName, suite.namespace, true)
	suite.runFakeAccManagerError("", false)
	suite.deleteAcc(accName)
}

// test error injection. Client get should fail
func (suite *AccControllerTestSuite) TestErrorInjection() {
	// test csm not found. err should be nil
	suite.runFakeAccManager("", true)
	// make a csm without finalizer
	suite.makeFakeAcc(csmName, suite.namespace, false)
	suite.reconcileAccWithErrorInjection(accName, "")
}

func (suite *AccControllerTestSuite) TestAccConnectivityClient() {
	csm := shared.MakeAcc(accName, suite.namespace, accConfigVersion)
	csm.Spec.Client.CSMClientType = csmv1.DreadnoughtClient
	csm.Spec.Client.Common.Image = "image"
}

func (suite *AccControllerTestSuite) TestAccConnectivityClientConnectionTarget() {
	csm := shared.MakeAcc(accName, suite.namespace, accConfigVersion)
	csm.Spec.Client.CSMClientType = csmv1.DreadnoughtClient
	csm.Spec.Client.Common.Image = "image"
	csm.Spec.Client.ConnectionTarget = "dev-svc.example.com"

	csm.ObjectMeta.Finalizers = []string{AccFinalizerName}

	suite.fakeClient.Create(accCtx, &csm)
	reconciler := suite.createAccReconciler()
	_, err := reconciler.Reconcile(accCtx, accReq)
	assert.Nil(suite.T(), err)
}

func (suite *AccControllerTestSuite) TestAccConnectivityClientCaCert() {
	csm := shared.MakeAcc(accName, suite.namespace, accConfigVersion)
	csm.Spec.Client.CSMClientType = csmv1.DreadnoughtClient
	csm.Spec.Client.Common.Image = "image"
	csm.Spec.Client.UsePrivateCaCerts = true

	csm.ObjectMeta.Finalizers = []string{AccFinalizerName}

	suite.fakeClient.Create(accCtx, &csm)
	reconciler := suite.createAccReconciler()
	_, err := reconciler.Reconcile(accCtx, accReq)
	assert.Nil(suite.T(), err)
}

func (suite *AccControllerTestSuite) TestAccConnectivityClientDcmImage() {
	csm := shared.MakeAcc(accName, suite.namespace, accConfigVersion)
	csm.Spec.Client.CSMClientType = csmv1.DreadnoughtClient
	csm.Spec.Client.Common.Name = accContainerName
	csm.Spec.Client.Common.Image = "image"

	csm.ObjectMeta.Finalizers = []string{AccFinalizerName}

	suite.fakeClient.Create(accCtx, &csm)
	reconciler := suite.createAccReconciler()
	_, err := reconciler.Reconcile(accCtx, accReq)
	assert.Nil(suite.T(), err)
}

func (suite *AccControllerTestSuite) TestAccConnectivityClientNoReconciler() {
	csm := shared.MakeAcc(accName, suite.namespace, accConfigVersion)
	csm.Spec.Client.CSMClientType = csmv1.DreadnoughtClient
	csm.Spec.Client.Common.Name = accContainerName
	csm.Spec.Client.Common.Image = "image"

	csm.ObjectMeta.Finalizers = []string{AccFinalizerName}

	suite.fakeClient.Create(accCtx, &csm)
	reconciler := suite.createAccReconciler()

	// Trigger error by using fake namespace
	oldNamespace := accReq.Namespace
	accReq.Namespace = "nonexistantnamespace"
	_, err := reconciler.Reconcile(accCtx, accReq)
	assert.Nil(suite.T(), err)

	// Restore namespace
	suite.deleteAcc(accName)
	accReq.Namespace = oldNamespace
}

func (suite *AccControllerTestSuite) TestAccConnectivityClientAnnotation() {
	csm := shared.MakeAcc(accName, suite.namespace, accConfigVersion)
	csm.Spec.Client.CSMClientType = csmv1.DreadnoughtClient
	csm.Spec.Client.Common.Image = "image"

	csm.ObjectMeta.Finalizers = []string{AccFinalizerName}

	suite.fakeClient.Create(accCtx, &csm)
	sec := shared.MakeSecret(accName+"-creds", suite.namespace, accConfigVersion)
	suite.fakeClient.Create(accCtx, sec)

	reconciler := suite.createAccReconciler()
	updateAccError = true
	_, err := reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	updateAccError = false
}

func (suite *AccControllerTestSuite) TestCsmFinalizerError() {
	csm := shared.MakeAcc(accName, suite.namespace, accConfigVersion)
	csm.ObjectMeta.Finalizers = []string{"foo"}
	suite.fakeClient.Create(accCtx, &csm)
	sec := shared.MakeSecret(accName+"-creds", suite.namespace, accConfigVersion)
	suite.fakeClient.Create(accCtx, sec)

	reconciler := suite.createAccReconciler()
	updateAccError = true
	_, err := reconciler.Reconcile(accCtx, accReq)
	assert.NotNil(suite.T(), err)
	updateAccError = false
}

func (suite *AccControllerTestSuite) TestCsmPreCheckVersionError() {
	// set bad version error
	accConfigVersion = "v0"
	csm := shared.MakeAcc(accName, suite.namespace, accConfigVersion)
	csm.Spec.Client.Common.Image = "image"
	csm.Annotations[configVersionKey] = accConfigVersion

	sec := shared.MakeSecret(accName+"-creds", suite.namespace, accConfigVersion)
	suite.fakeClient.Create(accCtx, sec)

	csm.ObjectMeta.Finalizers = []string{AccFinalizerName}
	suite.fakeClient.Create(accCtx, &csm)
	reconciler := suite.createAccReconciler()

	_, err := reconciler.Reconcile(accCtx, accReq)
	assert.NotNil(suite.T(), err)

	// set it back to good version for other tests
	suite.deleteAcc(accName)
	reconciler = suite.createAccReconciler()
	_, err = reconciler.Reconcile(accCtx, accReq)
	assert.NotNil(suite.T(), err)
	accConfigVersion = shared.AccConfigVersion
}

func (suite *AccControllerTestSuite) TestPreCheckAccError() {
	csm := shared.MakeAcc(accName, suite.namespace, accConfigVersion)
	csm.Spec.Client.CSMClientType = csmv1.DreadnoughtClient
	csm.Spec.Client.Common.Image = "image"
	csm.Annotations[configVersionKey] = accConfigVersion

	csm.ObjectMeta.Finalizers = []string{AccFinalizerName}
	suite.fakeClient.Create(accCtx, &csm)
	reconciler := suite.createAccReconciler()

	badOperatorConfig := utils.OperatorConfig{
		ConfigDirectory: "../in-valid-path",
	}

	err := reconciler.PreChecks(accCtx, &csm, badOperatorConfig)
	assert.NotNil(suite.T(), err)
}

func (suite *AccControllerTestSuite) TestPreCheckAccUnsupportedVersion() {
	acc := shared.MakeAcc(accName, suite.namespace, accConfigVersion)
	acc.Spec.Client.CSMClientType = csmv1.DreadnoughtClient
	acc.Spec.Client.Common.Image = "image"
	acc.Annotations[configVersionKey] = configVersion

	sec := shared.MakeSecret(accName+"-creds", suite.namespace, shared.AccConfigVersion)
	suite.fakeClient.Create(accCtx, sec)

	acc.ObjectMeta.Finalizers = []string{AccFinalizerName}
	suite.fakeClient.Create(accCtx, &acc)
	reconciler := suite.createAccReconciler()

	acc.Spec.Client.ConfigVersion = "v0"
	err := reconciler.PreChecks(accCtx, &acc, accOperatorConfig)
	assert.NotNil(suite.T(), err)
}

func (suite *AccControllerTestSuite) TestIgnoreUpdatePredicate() {
	p := suite.createAccReconciler().ignoreUpdatePredicate()
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

func (suite *AccControllerTestSuite) TestDeleteError() {
	suite.makeFakeAcc(accName, suite.namespace, true)
	suite.runFakeAccManager("", false)
	suite.deleteAcc(accName)

	deleteStatefulSetError = true
	suite.runFakeAccManager(deleteStatefulSetErrorStr, true)
	deleteStatefulSetError = false
}

// helper method to create and run reconciler
func TestCustomAcc(t *testing.T) {
	testSuite := new(AccControllerTestSuite)
	suite.Run(t, testSuite)
}

// test with a csm without a finalizer, reconcile should add it
func (suite *AccControllerTestSuite) TestClientContentWatch() {
	suite.createAccReconciler().ClientContentWatch()
	expRateLimiter := workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 120*time.Second)
	suite.createAccReconciler().SetupWithManager(nil, expRateLimiter, 1)
	close(AccStopWatch)
}

func (suite *AccControllerTestSuite) createAccReconciler() (reconciler *ApexConnectivityClientReconciler) {
	logType := logger.DevelopmentLogLevel
	logger.SetLoggerLevel(logType)
	_, log := logger.GetNewContextWithLogger("0")
	log.Infof("Version : %s", logType)

	reconciler = &ApexConnectivityClientReconciler{
		Client:        suite.fakeClient,
		K8sClient:     suite.k8sClient,
		Scheme:        scheme.Scheme,
		Log:           log,
		Config:        accOperatorConfig,
		EventRecorder: record.NewFakeRecorder(100),
	}

	return reconciler
}

func (suite *AccControllerTestSuite) runFakeAccManager(expectedErr string, reconcileDelete bool) {
	reconciler := suite.createAccReconciler()

	// invoke controller Reconcile to test. Typically k8s would call this when resource is changed
	res, err := reconciler.Reconcile(accCtx, accReq)

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
		suite.handleStatefulSetUpdateTest(reconciler, "dell-connectivity-client")
		suite.handleAccPodTest(reconciler, "acc-pod")
		_, err = reconciler.Reconcile(accCtx, accReq)
		if expectedErr == "" {
			assert.NoError(suite.T(), err)
		} else {
			assert.NotNil(suite.T(), err)
		}
	}
}

func (suite *AccControllerTestSuite) runFakeAccManagerError(expectedErr string, reconcileDelete bool) {
	reconciler := suite.createAccReconciler()

	// invoke controller Reconcile to test. Typically k8s would call this when resource is changed
	res, err := reconciler.Reconcile(accCtx, accReq)

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
		suite.handleStatefulSetUpdateTestFake(reconciler, "dell-connectivity-client")
		suite.handleAccPodTest(reconciler, "")
		_, err = reconciler.Reconcile(accCtx, accReq)
		assert.Nil(suite.T(), err)
	}
}

// call reconcile with different injection errors in k8s client
func (suite *AccControllerTestSuite) reconcileAccWithErrorInjection(_, expectedErr string) {
	reconciler := suite.createAccReconciler()

	// create would fail
	createAccSAError = true
	_, err := reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), createAccSAErrorStr, "expected error containing %q, got %s", expectedErr, err)
	createAccSAError = false

	createAccCRError = true
	_, err = reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), createAccCRErrorStr, "expected error containing %q, got %s", expectedErr, err)
	createAccCRError = false

	createAccCRBError = true
	_, err = reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), createAccCRBErrorStr, "expected error containing %q, got %s", expectedErr, err)
	createAccCRBError = false

	createAccError = true
	_, err = reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), createAccErrorStr, "expected error containing %q, got %s", expectedErr, err)
	createAccError = false

	createAccCMError = true
	_, err = reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), createAccCMErrorStr, "expected error containing %q, got %s", expectedErr, err)
	createAccCMError = false

	// create everything this time
	reconciler.Reconcile(accCtx, accReq)

	getAccError = true
	_, err = reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), getAccErrorStr, "expected error containing %q, got %s", expectedErr, err)
	getAccError = false

	getAccCMError = true
	_, err = reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), getAccCMErrorStr, "expected error containing %q, got %s", expectedErr, err)
	getAccCMError = false

	updateAccCMError = true
	_, err = reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), updateAccCMErrorStr, "expected error containing %q, got %s", expectedErr, err)
	updateAccCMError = false

	getCRBError = true
	_, err = reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), getAccCRBErrorStr, "expected error containing %q, got %s", expectedErr, err)
	getAccCRBError = false

	updateAccCRBError = true
	_, err = reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), updateAccCRBErrorStr, "expected error containing %q, got %s", expectedErr, err)
	updateAccCRBError = false

	getAccCRError = true
	_, err = reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), getAccCRErrorStr, "expected error containing %q, got %s", expectedErr, err)
	getAccCRError = false

	updateAccCRError = true
	_, err = reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), updateAccCRErrorStr, "expected error containing %q, got %s", expectedErr, err)
	updateAccCRError = false

	getAccSAError = true
	_, err = reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), getAccSAErrorStr, "expected error containing %q, got %s", expectedErr, err)
	getAccSAError = false

	updateAccSAError = true
	_, err = reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), updateAccSAErrorStr, "expected error containing %q, got %s", expectedErr, err)
	updateAccSAError = false

	deleteAccSAError = true
	suite.deleteAcc(accName)
	_, err = reconciler.Reconcile(accCtx, accReq)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), deleteAccSAErrorStr, "expected error containing %q, got %s", expectedErr, err)
	deleteAccSAError = false

}

func (suite *AccControllerTestSuite) handleStatefulSetUpdateTest(r *ApexConnectivityClientReconciler, name string) {
	statefulSet := &appsv1.StatefulSet{}
	err := suite.fakeClient.Get(accCtx, client.ObjectKey{Namespace: suite.namespace, Name: name}, statefulSet)
	assert.Nil(suite.T(), err)
	statefulSet.Spec.Template.Labels = map[string]string{"acc": "acc"}

	r.handleStatefulSetUpdate(statefulSet, statefulSet)

	// Make Pod and set pod status
	pod := shared.MakePod(name, suite.namespace)
	pod.Labels["acc"] = accName
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
	err = suite.fakeClient.Create(accCtx, &pod)
	assert.Nil(suite.T(), err)
	podList := &corev1.PodList{}
	err = suite.fakeClient.List(accCtx, podList, nil)
	assert.Nil(suite.T(), err)
}

func (suite *AccControllerTestSuite) handleStatefulSetUpdateTestFake(r *ApexConnectivityClientReconciler, name string) {
	statefulSet := &appsv1.StatefulSet{}
	err := suite.fakeClient.Get(accCtx, client.ObjectKey{Namespace: suite.namespace, Name: name}, statefulSet)
	assert.Error(suite.T(), err)
	statefulSet.Spec.Template.Labels = map[string]string{"acc": "acc"}

	r.handleStatefulSetUpdate(statefulSet, statefulSet)

	// Make Pod and set pod status
	pod := shared.MakePod(name, suite.namespace)
	pod.Labels["acc"] = accName
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
	err = suite.fakeClient.Create(accCtx, &pod)
	assert.Nil(suite.T(), err)
	podList := &corev1.PodList{}
	err = suite.fakeClient.List(accCtx, podList, nil)
	assert.Nil(suite.T(), err)
}

func (suite *AccControllerTestSuite) handleAccPodTest(r *ApexConnectivityClientReconciler, name string) {
	suite.makeAccFakePod(name, suite.namespace)
	pod := &corev1.Pod{}

	err := suite.fakeClient.Get(accCtx, client.ObjectKey{Namespace: suite.namespace, Name: name}, pod)
	assert.Nil(suite.T(), err)

	// since deployments/daemonsets dont create pod in non-k8s env, we have to explicitely create pod
	r.handlePodsUpdate(pod, pod)
}

// deleteAcc sets deletionTimeStamp on the csm object and deletes it
func (suite *AccControllerTestSuite) deleteAcc(accName string) {
	csm := &csmv1.ApexConnectivityClient{}
	key := types.NamespacedName{Namespace: suite.namespace, Name: accName}
	err := suite.fakeClient.Get(accCtx, key, csm)
	assert.Nil(suite.T(), err)

	suite.fakeClient.(*crclient.Client).SetDeletionTimeStamp(accCtx, csm)

	suite.fakeClient.Delete(accCtx, csm)
}

func (suite *AccControllerTestSuite) TestAccDeleteErrorReconcile() {
	suite.makeFakeAcc(accName, suite.namespace, true)
	suite.runFakeAccManager("", false)

	updateAccCSMError = true
	suite.deleteAcc(accName)
	reconciler := suite.createAccReconciler()
	_, err := reconciler.Reconcile(accCtx, accReq)
	fmt.Println(err)
	updateAccCSMError = false
}

// helper method to create k8s objects
func (suite *AccControllerTestSuite) makeFakeAcc(name, ns string, withFinalizer bool) {
	// make pre-requisite secrets
	sec := shared.MakeSecret(name+"-creds", ns, accConfigVersion)
	err := suite.fakeClient.Create(accCtx, sec)
	assert.Nil(suite.T(), err)

	csm := shared.MakeAcc(name, ns, accConfigVersion)
	csm.Spec.Client.CSMClientType = csmv1.DreadnoughtClient
	csm.Spec.Client.Common.Image = "image"
	sideCarObj1 := csmv1.ContainerTemplate{
		Name:            "kubernetes-proxy",
		Image:           "image2",
		ImagePullPolicy: "IfNotPresent",
	}
	sideCarObj2 := csmv1.ContainerTemplate{
		Name:            "cert-persister",
		Image:           "image3",
		ImagePullPolicy: "IfNotPresent",
	}
	sideCarList := []csmv1.ContainerTemplate{sideCarObj1, sideCarObj2}
	csm.Spec.Client.SideCars = sideCarList

	initContainerObj := csmv1.ContainerTemplate{
		Name:            "connectivity-client-init",
		Image:           "image4",
		ImagePullPolicy: "IfNotPresent",
	}
	initContainerList := []csmv1.ContainerTemplate{initContainerObj}
	csm.Spec.Client.InitContainers = initContainerList

	if withFinalizer {
		csm.ObjectMeta.Finalizers = []string{AccFinalizerName}
	}
	// remove driver when deleting csm
	csm.Spec.Client.ForceRemoveClient = true
	csm.Annotations[configVersionKey] = accConfigVersion

	err = suite.fakeClient.Create(accCtx, &csm)
	assert.Nil(suite.T(), err)
}

func (suite *AccControllerTestSuite) makeAccFakePod(name, ns string) {
	pod := shared.MakePod(name, ns)
	pod.Labels["acc"] = accName
	err := suite.fakeClient.Create(accCtx, &pod)
	assert.Nil(suite.T(), err)
}

func (suite *AccControllerTestSuite) ShouldFail(method string, obj runtime.Object) error {
	// Needs to implement based on need
	switch v := obj.(type) {
	case *csmv1.ApexConnectivityClient:
		csm := obj.(*csmv1.ApexConnectivityClient)
		if method == "Update" && updateAccError {
			fmt.Printf("[ShouldFail] force Updatecsm error for obj of type %+v\n", csm)
			return errors.New(updateAccCSMErrorStr)
		}

	case *corev1.ConfigMap:
		cm := obj.(*corev1.ConfigMap)
		if method == "Create" && createAccCMError {
			fmt.Printf("[ShouldFail] force create Configmap error for configmap named %+v\n", cm.Name)
			return errors.New(createAccCMErrorStr)
		} else if method == "Update" && updateAccCMError {
			fmt.Printf("[ShouldFail] force Update Configmap error for configmap named %+v\n", cm.Name)
			return errors.New(updateAccCMErrorStr)
		} else if method == "Get" && getAccCMError {
			fmt.Printf("[ShouldFail] force Get Configmap error for configmap named %+v\n", cm.Name)
			fmt.Printf("[ShouldFail] force Get Configmap error for configmap named %+v\n", v)
			return errors.New(getAccCMErrorStr)
		}

	case *rbacv1.ClusterRole:
		cr := obj.(*rbacv1.ClusterRole)
		if method == "Create" && createAccCRError {
			fmt.Printf("[ShouldFail] force Create ClusterRole error for ClusterRole named %+v\n", cr.Name)
			return errors.New(createAccCRErrorStr)
		} else if method == "Update" && updateAccCRError {
			fmt.Printf("[ShouldFail] force Update ClusterRole error for ClusterRole named %+v\n", cr.Name)
			return errors.New(updateAccCRErrorStr)
		} else if method == "Get" && getAccCRError {
			fmt.Printf("[ShouldFail] force Get ClusterRole error for ClusterRole named %+v\n", cr.Name)
			return errors.New(getAccCRErrorStr)
		}

	case *rbacv1.ClusterRoleBinding:
		crb := obj.(*rbacv1.ClusterRoleBinding)
		if method == "Create" && createAccCRBError {
			fmt.Printf("[ShouldFail] force Create ClusterRoleBinding error for ClusterRoleBinding named %+v\n", crb.Name)
			return errors.New(createAccCRBErrorStr)
		} else if method == "Update" && updateAccCRBError {
			fmt.Printf("[ShouldFail] force Update ClusterRoleBinding error for ClusterRoleBinding named %+v\n", crb.Name)
			return errors.New(updateAccCRBErrorStr)
		} else if method == "Get" && getAccCRBError {
			fmt.Printf("[ShouldFail] force Get ClusterRoleBinding error for ClusterRoleBinding named %+v\n", crb.Name)
			return errors.New(getAccCRBErrorStr)
		}
	case *corev1.ServiceAccount:
		sa := obj.(*corev1.ServiceAccount)
		if method == "Create" && createAccSAError {
			fmt.Printf("[ShouldFail] force Create ServiceAccount error for ServiceAccount named %+v\n", sa.Name)
			return errors.New(createAccSAErrorStr)
		} else if method == "Update" && updateAccSAError {
			fmt.Printf("[ShouldFail] force Update ServiceAccount error for ServiceAccount named %+v\n", sa.Name)
			return errors.New(updateAccSAErrorStr)
		} else if method == "Get" && getAccSAError {
			fmt.Printf("[ShouldFail] force Get ServiceAccount error for ServiceAccount named %+v\n", sa.Name)
			return errors.New(getAccSAErrorStr)
		} else if method == "Delete" && deleteAccSAError {
			fmt.Printf("[ShouldFail] force Delete ServiceAccount error for ServiceAccount named %+v\n", sa.Name)
			return errors.New(deleteAccSAErrorStr)
		}
	case *appsv1.StatefulSet:
		ss := obj.(*appsv1.StatefulSet)
		if method == "Delete" && deleteStatefulSetError {
			fmt.Printf("[ShouldFail] force StatefulSet error for StatefulSet named %+v\n", ss.Name)
			return errors.New(deleteStatefulSetErrorStr)
		}

	default:
	}
	return nil
}

// debugFakeObjects prints the runtime objects in the fake client
func (suite *AccControllerTestSuite) debugAccFakeObjects() {
	objects := suite.fakeClient.(*crclient.Client).Objects
	for key, o := range objects {
		accUnittestLogger.Info("found fake object ", "name", key.Name)
		accUnittestLogger.Info("found fake object ", "object", fmt.Sprintf("%#v", o))
	}
}

func TestSyncStatefulSet(t *testing.T) {
	labels := make(map[string]string, 1)
	labels["*-8-acc"] = "/*-acc"
	statefulset := configv1.StatefulSetApplyConfiguration{
		ObjectMetaApplyConfiguration: &v1.ObjectMetaApplyConfiguration{Name: &[]string{"acc"}[0], Namespace: &[]string{"default"}[0]},
		Spec: &configv1.StatefulSetSpecApplyConfiguration{Template: &corev12.PodTemplateSpecApplyConfiguration{
			ObjectMetaApplyConfiguration: &v1.ObjectMetaApplyConfiguration{Labels: labels},
		}},
	}
	k8sClient := fake.NewSimpleClientset()
	accName = "acc"
	containers := make([]corev1.Container, 0)
	containers = append(containers, corev1.Container{Name: "fake-container", Image: "fake-image"})
	create, err := k8sClient.AppsV1().StatefulSets("default").Create(context.Background(), &appsv1.StatefulSet{
		ObjectMeta: apiv1.ObjectMeta{
			Name:      accName,
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: apiv1.ObjectMeta{},
				Spec:       corev1.PodSpec{Containers: containers},
			},
		},
	}, apiv1.CreateOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, create)
	k8sClient.PrependReactor("patch", "statefulsets", func(_ clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("fake error")
	})
	err = statefulsetpkg.SyncStatefulSet(context.Background(), statefulset, k8sClient, csmName)
	assert.Error(t, err)
}

// Test all edge cases in SyncCSM
func (suite *AccControllerTestSuite) TestSyncACC() {
	r := suite.createAccReconciler()
	acc := shared.MakeAcc(accName, suite.namespace, accConfigVersion)
	accBadType := shared.MakeAcc(accName, suite.namespace, accConfigVersion)
	accBadType.Spec.Client.CSMClientType = "wrongclient"

	syncACCTests := []struct {
		name        string
		acc         csmv1.ApexConnectivityClient
		op          utils.OperatorConfig
		expectedErr string
	}{
		{"getClientConfig bad op config", acc, badAccOperatorConfig, ""},
		{"getClientConfig error", accBadType, badAccOperatorConfig, "no such file or directory"},
	}

	for _, tt := range syncACCTests {
		suite.T().Run(tt.name, func(t *testing.T) {
			err := r.SyncACC(ctx, tt.acc, tt.op)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Error(t, err)
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}
