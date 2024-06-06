//  Copyright © 2022 - 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"strings"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	csmv1 "github.com/dell/csm-operator/api/v1"
	v1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/tests/shared"
	"github.com/dell/csm-operator/tests/shared/clientgoclient"
	"github.com/dell/csm-operator/tests/shared/crclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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

	updateDSError    bool
	updateDSErrorStr = "unable to update Daemonset"

	deleteDSError    bool
	deleteDSErrorStr = "unable to delete Daemonset"

	deleteDeploymentError    bool
	deleteDeploymentErrorStr = "unable to delete Deployment"

	deleteSAError    bool
	deleteSAErrorStr = "unable to delete ServiceAccount"

	csmName = "csm"

	configVersion            = shared.ConfigVersion
	oldConfigVersion         = shared.OldConfigVersion
	upgradeConfigVersion     = shared.UpgradeConfigVersion
	jumpUpgradeConfigVersion = shared.JumpUpgradeConfigVersion
	invalidConfigVersion     = shared.BadConfigVersion

	req = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "test",
			Name:      csmName,
		},
	}

	operatorConfig = utils.OperatorConfig{
		ConfigDirectory: "../operatorconfig",
	}

	badOperatorConfig = utils.OperatorConfig{
		ConfigDirectory: "../in-valid-path",
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
	velerov1.AddToScheme(scheme.Scheme)
	apiextv1.AddToScheme(scheme.Scheme)

	apiextv1.AddToScheme(scheme.Scheme)
	certmanagerv1.AddToScheme(scheme.Scheme)

	objects := map[shared.StorageKey]runtime.Object{}
	suite.fakeClient = crclient.NewFakeClient(objects, suite)
	suite.k8sClient = clientgoclient.NewFakeClient(suite.fakeClient)

	suite.namespace = "test"

	os.Setenv("UNIT_TEST", "true")
}

// test a happy path scenerio with deletion
func (suite *CSMControllerTestSuite) TestReconcile() {
	suite.makeFakeCSM(csmName, suite.namespace, true, append(getReplicaModule(), getObservabilityModule()...))
	suite.runFakeCSMManager("", false)
	suite.deleteCSM(csmName)
	suite.runFakeCSMManager("", true)
}

func (suite *CSMControllerTestSuite) TestAuthorizationServerReconcile() {
	suite.makeFakeAuthServerCSM(csmName, suite.namespace, getAuthProxyServer())
	suite.runFakeAuthCSMManager("timed out waiting for the condition", false, false)
	suite.deleteCSM(csmName)
	suite.runFakeAuthCSMManager("", true, false)
}

func (suite *CSMControllerTestSuite) TestAuthorizationServerReconcileOCP() {
	suite.makeFakeAuthServerCSMOCP(csmName, suite.namespace, getAuthProxyServerOCP())
	suite.runFakeAuthCSMManager("", false, true)
	suite.deleteCSM(csmName)
	suite.runFakeAuthCSMManager("", true, true)
}

func (suite *CSMControllerTestSuite) TestAppMobReconcile() {
	suite.makeFakeAppMobCSM(csmName, suite.namespace, getAppMob())
	suite.runFakeAuthCSMManager("", false, false)
	suite.deleteCSM(csmName)
	suite.runFakeAuthCSMManager("", true, false)
}

func (suite *CSMControllerTestSuite) TestResiliencyReconcile() {
	suite.makeFakeResiliencyCSM(csmName, suite.namespace, true, append(getResiliencyModule(), getResiliencyModule()...), string(v1.PowerStore))
	suite.runFakeCSMManager("", false)
	suite.deleteCSM(csmName)
	suite.runFakeCSMManager("", true)
}

func (suite *CSMControllerTestSuite) TestResiliencyReconcileError() {
	suite.makeFakeResiliencyCSM(csmName, suite.namespace, false, append(getResiliencyModule(), getResiliencyModule()...), "unsupported-driver")
	reconciler := suite.createReconciler()
	res, err := reconciler.Reconcile(ctx, req)
	ctrl.Log.Info("reconcile response", "res is: ", res)
	if err != nil {
		assert.Error(suite.T(), err)
	}
}

func (suite *CSMControllerTestSuite) TestReverseProxyReconcile() {
	suite.makeFakeRevProxyCSM(csmName, suite.namespace, true, getReverseProxyModule(), string(v1.PowerMax))
	suite.runFakeCSMManager("", false)
	suite.deleteCSM(csmName)
	suite.runFakeCSMManager("", true)
}

func (suite *CSMControllerTestSuite) TestReverseProxyPreCheckError() {
	suite.makeFakeRevProxyCSM(csmName, suite.namespace, false, getReverseProxyModule(), "badVersion")
	reconciler := suite.createReconciler()
	res, err := reconciler.Reconcile(ctx, req)
	ctrl.Log.Info("reconcile response", "res is: ", res)
	if err != nil {
		assert.Error(suite.T(), err)
	}
}

func (suite *CSMControllerTestSuite) TestReconcileReverseProxyError() {
	csm := shared.MakeCSM(csmName, suite.namespace, shared.PmaxConfigVersion)
	csm.Spec.Modules = getReverseProxyModule()
	reconciler := suite.createReconciler()
	err := reconciler.reconcileReverseProxy(ctx, false, badOperatorConfig, csm, suite.fakeClient)
	assert.NotNil(suite.T(), err)
}

func (suite *CSMControllerTestSuite) TestPowermaxReconcileError() {
	suite.makeFakeRevProxyCSM(csmName, suite.namespace, false, getReverseProxyModule(), "badDriver")
	reconciler := suite.createReconciler()
	res, err := reconciler.Reconcile(ctx, req)
	ctrl.Log.Info("reconcile response", "res is: ", res)
	if err != nil {
		assert.Error(suite.T(), err)
	}
}

// test error injection. Client get should fail
func (suite *CSMControllerTestSuite) TestErrorInjection() {
	// test csm not found. err should be nil
	suite.runFakeCSMManager("", true)
	// make a csm without finalizer
	suite.makeFakeCSM(csmName, suite.namespace, false, getAuthModule())
	suite.reconcileWithErrorInjection(csmName, "")
}

func (suite *CSMControllerTestSuite) TestPowerScaleAnnotation() {
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

func (suite *CSMControllerTestSuite) TestPowerFlexAnnotation() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.PowerFlex

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

func (suite *CSMControllerTestSuite) TestPowerStoreAnnotation() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.PowerStore

	csm.ObjectMeta.Finalizers = []string{CSMFinalizerName}

	suite.fakeClient.Create(ctx, &csm)
	sec := shared.MakeSecret(csmName+"-config", suite.namespace, configVersion)
	suite.fakeClient.Create(ctx, sec)

	reconciler := suite.createReconciler()
	updateCSMError = true
	_, err := reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	updateCSMError = false
}

func (suite *CSMControllerTestSuite) TestUnityAnnotation() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.Unity

	csm.ObjectMeta.Finalizers = []string{CSMFinalizerName}

	suite.fakeClient.Create(ctx, &csm)
	sec := shared.MakeSecret(csmName+"-config", suite.namespace, configVersion)
	suite.fakeClient.Create(ctx, sec)

	reconciler := suite.createReconciler()
	updateCSMError = true
	_, err := reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	updateCSMError = false
}

func (suite *CSMControllerTestSuite) TestPowermaxAnnotation() {
	csm := shared.MakeCSM(csmName, suite.namespace, shared.PmaxConfigVersion)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.PowerMax

	csm.ObjectMeta.Finalizers = []string{CSMFinalizerName}

	suite.fakeClient.Create(ctx, &csm)
	sec := shared.MakeSecret(csmName+"-creds", suite.namespace, shared.PmaxConfigVersion)
	suite.fakeClient.Create(ctx, sec)

	reconciler := suite.createReconciler()
	updateCSMError = true
	_, err := reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	updateCSMError = false
}

func (suite *CSMControllerTestSuite) TestCsmUpgrade() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.PowerScale

	csm.ObjectMeta.Finalizers = []string{CSMFinalizerName}

	suite.fakeClient.Create(ctx, &csm)
	sec := shared.MakeSecret(csmName+"-creds", suite.namespace, configVersion)
	suite.fakeClient.Create(ctx, sec)

	annotations := csm.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if _, ok := annotations[configVersionKey]; !ok {
		annotations[configVersionKey] = upgradeConfigVersion
		csm.SetAnnotations(annotations)
	}

	reconciler := suite.createReconciler()
	_, err := reconciler.Reconcile(ctx, req)
	assert.Nil(suite.T(), err)
}

func (suite *CSMControllerTestSuite) TestCsmUpgradeVersionTooOld() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.PowerScale

	csm.ObjectMeta.Finalizers = []string{CSMFinalizerName}

	suite.fakeClient.Create(ctx, &csm)
	sec := shared.MakeSecret(csmName+"-creds", suite.namespace, configVersion)
	suite.fakeClient.Create(ctx, sec)

	annotations := csm.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if _, ok := annotations[configVersionKey]; !ok {
		annotations[configVersionKey] = oldConfigVersion
		csm.SetAnnotations(annotations)
	}

	reconciler := suite.createReconciler()
	_, err := reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
}

func (suite *CSMControllerTestSuite) TestCsmUpgradeSkipVersion() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.PowerScale

	csm.ObjectMeta.Finalizers = []string{CSMFinalizerName}

	suite.fakeClient.Create(ctx, &csm)
	sec := shared.MakeSecret(csmName+"-creds", suite.namespace, configVersion)
	suite.fakeClient.Create(ctx, sec)

	annotations := csm.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if _, ok := annotations[configVersionKey]; !ok {
		annotations[configVersionKey] = jumpUpgradeConfigVersion
		csm.SetAnnotations(annotations)
	}

	reconciler := suite.createReconciler()
	_, err := reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
}

func (suite *CSMControllerTestSuite) TestCsmUpgradePathInvalid() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.PowerScale

	csm.ObjectMeta.Finalizers = []string{CSMFinalizerName}

	suite.fakeClient.Create(ctx, &csm)
	sec := shared.MakeSecret(csmName+"-creds", suite.namespace, configVersion)
	suite.fakeClient.Create(ctx, sec)

	annotations := csm.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if _, ok := annotations[configVersionKey]; !ok {
		annotations[configVersionKey] = invalidConfigVersion
		csm.SetAnnotations(annotations)
	}

	reconciler := suite.createReconciler()
	_, err := reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
}

func (suite *CSMControllerTestSuite) TestCsmFinalizerError() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.ObjectMeta.Finalizers = []string{"foo"}
	csm.Spec.Driver.CSIDriverType = csmv1.PowerScale
	suite.fakeClient.Create(ctx, &csm)
	sec := shared.MakeSecret(csmName+"-creds", suite.namespace, configVersion)
	suite.fakeClient.Create(ctx, sec)

	reconciler := suite.createReconciler()
	updateCSMError = true
	_, err := reconciler.Reconcile(ctx, req)
	assert.NotNil(suite.T(), err)
	updateCSMError = false
}

// Test all edge cases in RemoveDriver
func (suite *CSMControllerTestSuite) TestRemoveDriver() {
	r := suite.createReconciler()
	csmBadType := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csmBadType.Spec.Driver.CSIDriverType = "wrongdriver"
	csmWoType := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Driver.CSIDriverType = "powerscale"

	removeDriverTests := []struct {
		name          string
		csm           csmv1.ContainerStorageModule
		errorInjector *bool
		expectedErr   string
	}{
		{"getDriverConfig error", csmBadType, nil, "no such file or directory"},
		// don't return error if there's no driver- could be a valid case like Auth server or App Mobility
		{"getDriverConfig no driver", csmWoType, nil, ""},
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
				suite.makeFakeCSM(csmName, suite.namespace, true, append(getAuthModule(), getObservabilityModule()...))
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

// Test all edge cases in SyncCSM
func (suite *CSMControllerTestSuite) TestSyncCSM() {
	r := suite.createReconciler()
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csmBadType := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csmBadType.Spec.Driver.CSIDriverType = "wrongdriver"
	authProxyServerCSM := shared.MakeCSM(csmName, suite.namespace, configVersion)
	authProxyServerCSM.Spec.Modules = getAuthProxyServer()
	appMobCSM := shared.MakeCSM(csmName, suite.namespace, configVersion)
	appMobCSM.Spec.Modules = getAppMob()
	reverseProxyServerCSM := shared.MakeCSM(csmName, suite.namespace, configVersion)
	reverseProxyServerCSM.Spec.Modules = getReverseProxyModule()

	syncCSMTests := []struct {
		name        string
		csm         csmv1.ContainerStorageModule
		op          utils.OperatorConfig
		expectedErr string
	}{
		{"auth proxy server bad op conf", authProxyServerCSM, badOperatorConfig, "failed to deploy authorization proxy server"},
		{"app mobility happy path", appMobCSM, operatorConfig, ""},
		{"app mobility bad op conf", appMobCSM, badOperatorConfig, "failed to deploy application mobility"},
		{"reverse proxy server bad op conf", reverseProxyServerCSM, badOperatorConfig, "failed to deploy reverseproxy proxy server"},
		{"getDriverConfig bad op config", csm, badOperatorConfig, ""},
		{"getDriverConfig error", csmBadType, badOperatorConfig, "no such file or directory"},
	}

	for _, tt := range syncCSMTests {
		suite.T().Run(tt.name, func(t *testing.T) {
			err := r.SyncCSM(ctx, tt.csm, tt.op, r.Client)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Error(t, err)
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

func (suite *CSMControllerTestSuite) TestRemoveModule() {
	r := suite.createReconciler()
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Modules = getAuthProxyServer()
	csmBadVersionAuthProxy := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csmBadVersionAuthProxy.Spec.Modules = getAuthProxyServer()
	csmBadVersionAuthProxy.Spec.Modules[0].ConfigVersion = shared.BadConfigVersion
	csmBadVersionRevProxy := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csmBadVersionRevProxy.Spec.Modules = getReverseProxyModule()
	csmBadVersionRevProxy.Spec.Modules[0].ConfigVersion = shared.BadConfigVersion

	removeModuleTests := []struct {
		name          string
		csm           csmv1.ContainerStorageModule
		errorInjector *bool
		expectedErr   string
	}{
		{"remove module - success", csm, nil, ""},
		{"remove module bad version error", csmBadVersionAuthProxy, nil, "unable to reconcile"},
		{"remove module bad version error", csmBadVersionRevProxy, nil, "unable to reconcile"},
	}

	for _, tt := range removeModuleTests {
		suite.T().Run(csmName, func(t *testing.T) {
			if tt.errorInjector != nil {
				suite.makeFakeCSM(csmName, suite.namespace, false, getAuthProxyServer())
				r.Reconcile(ctx, req)
				*tt.errorInjector = true
			}
			err := r.removeModule(ctx, tt.csm, operatorConfig, r.Client)
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

func (suite *CSMControllerTestSuite) TestOldStandAloneModuleCleanup() {
	tests := map[string]func(t *testing.T) (csm *csmv1.ContainerStorageModule, errorInjector *bool, expectedErr string){
		"Success - Enable all modules": func(*testing.T) (*csmv1.ContainerStorageModule, *bool, string) {
			suite.makeFakeCSM(csmName, suite.namespace, false, append(append(getReplicaModule(), getObservabilityModule()...), getAppMob()...))
			csm := &csmv1.ContainerStorageModule{}
			key := types.NamespacedName{Namespace: suite.namespace, Name: csmName}
			err := suite.fakeClient.Get(ctx, key, csm)
			assert.Nil(suite.T(), err)
			csm.Spec.Modules = append(append(getReplicaModule(), getObservabilityModule()...), getAppMob()...)
			return csm, &[]bool{false}[0], ""
		},
		"Success - Disable all modules": func(*testing.T) (*csmv1.ContainerStorageModule, *bool, string) {
			suite.makeFakeCSM(csmName, suite.namespace, false, append(append(getReplicaModule(), getObservabilityModule()...), getAppMob()...))

			csm := &csmv1.ContainerStorageModule{}
			key := types.NamespacedName{Namespace: suite.namespace, Name: csmName}
			err := suite.fakeClient.Get(ctx, key, csm)
			assert.Nil(suite.T(), err)
			replica := getReplicaModule()
			replica[0].Enabled = false
			appMob := getAppMob()
			appMob[0].Enabled = false
			obs := getObservabilityModule()
			obs[0].Enabled = false
			csm.Spec.Modules = append(append(replica, obs...), appMob...)
			return csm, &[]bool{false}[0], ""
		},
		"Success - Disable Components": func(*testing.T) (*csmv1.ContainerStorageModule, *bool, string) {
			suite.makeFakeCSM(csmName, suite.namespace, false, append(append(getReplicaModule(), getObservabilityModule()...), getAppMob()...))

			csm := &csmv1.ContainerStorageModule{}
			key := types.NamespacedName{Namespace: suite.namespace, Name: csmName}
			err := suite.fakeClient.Get(ctx, key, csm)
			assert.Nil(suite.T(), err)
			obs := getObservabilityModule()
			obs[0].Components[0].Enabled = &[]bool{false}[0]
			appMob := getAppMob()
			appMob[0].Components[0].Enabled = &[]bool{false}[0]
			csm.Spec.Modules = append(append(getReplicaModule(), getObservabilityModule()...), getAppMob()...)
			return csm, &[]bool{false}[0], ""
		},
	}

	r := suite.createReconciler()
	for name, tc := range tests {
		suite.T().Run(name, func(t *testing.T) {
			csm, errorInjector, expectedErr := tc(t)
			if errorInjector != nil {
				*errorInjector = true
			}
			driverConfig, _ := getDriverConfig(ctx, *csm, operatorConfig)
			err := r.oldStandAloneModuleCleanup(ctx, csm, operatorConfig, driverConfig)

			if expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Error(t, err)
				assert.Containsf(t, err.Error(), expectedErr, "expected error containing %q, got %s", expectedErr, err)
			}

			if errorInjector != nil {
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
	assert.NotNil(suite.T(), err)
	configVersion = shared.ConfigVersion
}

func (suite *CSMControllerTestSuite) TestCsmPreCheckTypeError() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Driver.CSIDriverType = csmv1.PowerStore
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
	assert.NotNil(suite.T(), err)
	configVersion = shared.ConfigVersion
}

func (suite *CSMControllerTestSuite) TestCsmPreCheckModuleError() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Driver.CSIDriverType = csmv1.PowerScale
	csm.Spec.Driver.Common.Image = "image"
	csm.Annotations[configVersionKey] = configVersion

	sec := shared.MakeSecret(csmName+"-creds", suite.namespace, configVersion)
	suite.fakeClient.Create(ctx, sec)

	csm.ObjectMeta.Finalizers = []string{CSMFinalizerName}
	suite.fakeClient.Create(ctx, &csm)
	reconciler := suite.createReconciler()

	badOperatorConfig := utils.OperatorConfig{
		ConfigDirectory: "../in-valid-path",
	}

	// error in Authorization
	csm.Spec.Modules = getAuthModule()
	err := reconciler.PreChecks(ctx, &csm, badOperatorConfig)
	assert.NotNil(suite.T(), err)

	// error in Authorization Proxy Server
	csm.Spec.Modules = getAuthProxyServer()
	err = reconciler.PreChecks(ctx, &csm, badOperatorConfig)
	assert.NotNil(suite.T(), err)

	// error in App-Mobility
	csm.Spec.Modules = getAppMob()
	err = reconciler.PreChecks(ctx, &csm, badOperatorConfig)
	assert.NotNil(suite.T(), err)

	// error in Replication
	csm.Spec.Modules = getReplicaModule()
	err = reconciler.PreChecks(ctx, &csm, badOperatorConfig)
	assert.NotNil(suite.T(), err)

	// error in Resiliency
	csm.Spec.Modules = getResiliencyModule()
	err = reconciler.PreChecks(ctx, &csm, badOperatorConfig)
	assert.NotNil(suite.T(), err)

	// error in Observability
	csm.Spec.Modules = getObservabilityModule()
	err = reconciler.PreChecks(ctx, &csm, badOperatorConfig)
	assert.NotNil(suite.T(), err)

	// error unsupported module
	csm.Spec.Modules = []csmv1.Module{
		{
			Name:    "Unsupported module",
			Enabled: true,
		},
	}
	err = reconciler.PreChecks(ctx, &csm, badOperatorConfig)
	assert.NotNil(suite.T(), err)
}

func (suite *CSMControllerTestSuite) TestCsmPreCheckModuleUnsupportedVersion() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Driver.CSIDriverType = csmv1.PowerScale
	csm.Spec.Driver.Common.Image = "image"
	csm.Annotations[configVersionKey] = configVersion

	sec := shared.MakeSecret(csmName+"-creds", suite.namespace, configVersion)
	suite.fakeClient.Create(ctx, sec)

	csm.ObjectMeta.Finalizers = []string{CSMFinalizerName}
	suite.fakeClient.Create(ctx, &csm)
	reconciler := suite.createReconciler()

	// error in Authorization
	csm.Spec.Modules = getAuthModule()
	csm.Spec.Modules[0].ConfigVersion = "1.0.0"
	err := reconciler.PreChecks(ctx, &csm, operatorConfig)
	assert.NotNil(suite.T(), err)

	// error in Authorization Proxy Server
	csm.Spec.Modules = getAuthProxyServer()
	csm.Spec.Modules[0].ConfigVersion = "1.0.0"
	err = reconciler.PreChecks(ctx, &csm, operatorConfig)
	assert.NotNil(suite.T(), err)

	// error in App Mobility
	csm.Spec.Modules = getAppMob()
	csm.Spec.Modules[0].ConfigVersion = "8.0.0"
	err = reconciler.PreChecks(ctx, &csm, operatorConfig)
	assert.NotNil(suite.T(), err)

	// error in Replication
	csm.Spec.Modules = getReplicaModule()
	csm.Spec.Modules[0].ConfigVersion = "1.0.0"
	err = reconciler.PreChecks(ctx, &csm, operatorConfig)
	assert.NotNil(suite.T(), err)

	// error in Resiliency
	csm.Spec.Modules = getResiliencyModule()
	csm.Spec.Modules[0].ConfigVersion = "1.0.0"
	err = reconciler.PreChecks(ctx, &csm, operatorConfig)
	assert.NotNil(suite.T(), err)

	// error in Observability
	csm.Spec.Modules = getObservabilityModule()
	csm.Spec.Modules[0].ConfigVersion = "1.0.0"
	err = reconciler.PreChecks(ctx, &csm, operatorConfig)
	assert.NotNil(suite.T(), err)

	// error unsupported module
	csm.Spec.Modules = []csmv1.Module{
		{
			Name:    "Unsupported module",
			Enabled: true,
		},
	}
	err = reconciler.PreChecks(ctx, &csm, operatorConfig)
	assert.NotNil(suite.T(), err)
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
	version, err := utils.GetModuleDefaultVersion("v2.4.0", "csi-isilon", csmv1.Authorization, "../operatorconfig")
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

	// invoke controller Reconcile to test. Typically, k8s would call this when resource is changed
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

func (suite *CSMControllerTestSuite) runFakeAuthCSMManager(expectedErr string, reconcileDelete, isOpenShift bool) {
	reconciler := suite.createReconciler()
	if isOpenShift {
		reconciler.Config.IsOpenShift = true
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

	if !reconcileDelete {
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
func (suite *CSMControllerTestSuite) reconcileWithErrorInjection(_, expectedErr string) {
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

	// test CSM object with failed state leads to requeue
	os.Setenv("UNIT_TEST", "false")
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), "CSM state is failed", "expected error containing %q, got %s", expectedErr, err)
	os.Setenv("UNIT_TEST", "true")

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

	// test CSM object with failed state, cannot update CSM object
	os.Setenv("UNIT_TEST", "false")
	updateCSMError = true
	_, err = reconciler.Reconcile(ctx, req)
	assert.Error(suite.T(), err)
	assert.Containsf(suite.T(), err.Error(), updateCSMErrorStr, "expected error containing %q, got %s", expectedErr, err)
	updateCSMError = false
	os.Setenv("UNIT_TEST", "true")

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
	deployment := &appsv1.Deployment{}
	err := suite.fakeClient.Get(ctx, client.ObjectKey{Namespace: suite.namespace, Name: name}, deployment)
	assert.Nil(suite.T(), err)
	deployment.Spec.Template.Labels = map[string]string{"csm": "csm"}

	r.handleDeploymentUpdate(deployment, deployment)

	// Make Pod and set pod status
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

func getObservabilityModule() []csmv1.Module {
	return []csmv1.Module{
		{
			Name:          csmv1.Observability,
			Enabled:       true,
			ConfigVersion: "v1.8.0",
			Components: []csmv1.ContainerTemplate{
				{
					Name:    "topology",
					Enabled: &[]bool{true}[0],
					Envs: []corev1.EnvVar{
						{
							Name:  "TOPOLOGY_LOG_LEVEL",
							Value: "INFO",
						},
					},
				},
				{
					Name:    "otel-collector",
					Enabled: &[]bool{true}[0],
					Envs: []corev1.EnvVar{
						{
							Name:  "NGINX_PROXY_IMAGE",
							Value: "nginxinc/nginx-unprivileged:1.20",
						},
					},
				},
				{
					Name:    "metrics-powerscale",
					Enabled: &[]bool{true}[0],
					Envs: []corev1.EnvVar{
						{
							Name:  "POWERSCALE_MAX_CONCURRENT_QUERIES",
							Value: "10",
						},
					},
				},
				{
					Name:    "metrics-powerflex",
					Enabled: &[]bool{true}[0],
					Envs: []corev1.EnvVar{
						{
							Name:  "POWERFLEX_MAX_CONCURRENT_QUERIES",
							Value: "10",
						},
					},
				},
			},
		},
	}
}

func getReplicaModule() []csmv1.Module {
	return []csmv1.Module{
		{
			Name:          csmv1.Replication,
			Enabled:       true,
			ConfigVersion: "v1.6.0",
			Components: []csmv1.ContainerTemplate{
				{
					Name: utils.ReplicationSideCarName,
				},
				{
					Name: utils.ReplicationControllerManager,
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

func getResiliencyModule() []csmv1.Module {
	return []csmv1.Module{
		{
			Name:          csmv1.Resiliency,
			Enabled:       true,
			ConfigVersion: "v1.9.0",
			Components: []csmv1.ContainerTemplate{
				{
					Name: utils.ResiliencySideCarName,
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
			ConfigVersion: "v2.0.0-alpha",
			Components: []csmv1.ContainerTemplate{
				{
					Name: "karavi-authorization-proxy",
					Envs: []corev1.EnvVar{
						{
							Name:  "SKIP_CERTIFICATE_VALIDATION",
							Value: "true",
						},
					},
				},
			},
		},
	}
}

func getAuthProxyServer() []csmv1.Module {
	return []csmv1.Module{
		{
			Name:              csmv1.AuthorizationServer,
			Enabled:           true,
			ConfigVersion:     "v2.0.0-alpha",
			ForceRemoveModule: true,
			Components: []csmv1.ContainerTemplate{
				{
					Name:     "proxy-server",
					Enabled:  &[]bool{true}[0],
					Hostname: "csm-auth.com",
					ProxyServerIngress: []csmv1.ProxyServerIngress{
						{
							IngressClassName: "nginx",
							Hosts:            []string{"additional-host.com"},
							Annotations:      map[string]string{"test": "test"},
						},
					},
				},
				{
					Name:    "cert-manager",
					Enabled: &[]bool{true}[0],
				},
				{
					Name:    "nginx",
					Enabled: &[]bool{true}[0],
				},
				{
					Name:              "redis",
					RedisStorageClass: "test-storage",
				},
			},
		},
	}
}

func getAuthProxyServerOCP() []csmv1.Module {
	return []csmv1.Module{
		{
			Name:              csmv1.AuthorizationServer,
			Enabled:           true,
			ConfigVersion:     "v2.0.0-alpha",
			ForceRemoveModule: true,
			Components: []csmv1.ContainerTemplate{
				{
					Name:     "proxy-server",
					Enabled:  &[]bool{true}[0],
					Hostname: "csm-auth.com",
					ProxyServerIngress: []csmv1.ProxyServerIngress{
						{
							IngressClassName: "nginx",
							Hosts:            []string{"additional-host.com"},
							Annotations:      map[string]string{"test": "test"},
						},
					},
				},
				{
					Name:    "cert-manager",
					Enabled: &[]bool{true}[0],
				},
				{
					Name:    "nginx",
					Enabled: &[]bool{false}[0],
				},
				{
					Name:              "redis",
					RedisStorageClass: "test-storage",
				},
			},
		},
	}
}

func getAppMob() []csmv1.Module {
	return []csmv1.Module{
		{
			Name:          csmv1.ApplicationMobility,
			Enabled:       true,
			ConfigVersion: "v1.0.2",
			Components: []csmv1.ContainerTemplate{
				{
					Name:    "application-mobility-controller-manager",
					Enabled: &[]bool{true}[0],
					Envs: []corev1.EnvVar{
						{
							Name:  "APPLICATION_MOBILITY_REPLICA_COUNT",
							Value: "1",
						},
					},
				},
				{
					Name:    "cert-manager",
					Enabled: &[]bool{true}[0],
				},
				{
					Name:    "velero",
					Enabled: &[]bool{true}[0],
					Envs: []corev1.EnvVar{
						{
							Name:  "BACKUPSTORAGELOCATION_NAME",
							Value: "default",
						},
						{
							Name:  "CONFIG_PROVIDER",
							Value: "aws",
						},
						{
							Name:  "BUCKET_NAME",
							Value: "velero-bucket",
						},
						{
							Name:  "VOL_SNAPSHOT_LOCATION_NAME",
							Value: "default",
						},
						{
							Name:  "BACKUP_STORAGE_URL",
							Value: "localhost:8000",
						},
					},
				},
			},
			ForceRemoveModule: true,
		},
	}
}

func getReverseProxyModule() []csmv1.Module {
	return []csmv1.Module{
		{
			Name:          csmv1.ReverseProxy,
			Enabled:       true,
			ConfigVersion: "v2.9.0",
			Components: []csmv1.ContainerTemplate{
				{
					Name:    string(csmv1.ReverseProxyServer),
					Enabled: &[]bool{true}[0],
					Envs: []corev1.EnvVar{
						{
							Name:  "X_CSI_REVPROXY_TLS_SECRET",
							Value: "csirevproxy-tls-secret",
						},
						{
							Name:  "X_CSI_REVPROXY_PORT",
							Value: "2222",
						},
						{
							Name:  "X_CSI_CONFIG_MAP_NAME",
							Value: "powermax-reverseproxy-config",
						},
					},
				},
			},
			ForceRemoveModule: true,
		},
	}
}

func (suite *CSMControllerTestSuite) TestDeleteErrorReconcile() {
	suite.makeFakeCSM(csmName, suite.namespace, true, append(getAuthModule(), getObservabilityModule()...))
	suite.runFakeCSMManager("", false)

	updateCSMError = true
	suite.deleteCSM(csmName)
	reconciler := suite.createReconciler()
	_, err := reconciler.Reconcile(ctx, req)
	assert.NotNil(suite.T(), err)
	updateCSMError = false
}

func (suite *CSMControllerTestSuite) TestReconcileObservabilityError() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Modules = getObservabilityModule()
	reconciler := suite.createReconciler()
	badOperatorConfig := utils.OperatorConfig{
		ConfigDirectory: "../in-valid-path",
	}
	err := reconciler.reconcileObservability(ctx, false, badOperatorConfig, csm, nil, suite.fakeClient, suite.k8sClient)
	assert.NotNil(suite.T(), err)

	for i := range csm.Spec.Modules[0].Components {
		fmt.Printf("Component name: %s\n", csm.Spec.Modules[0].Components[i].Name)
		csm.Spec.Modules[0].Components[i].Enabled = &[]bool{false}[0]
		err = reconciler.reconcileObservability(ctx, false, badOperatorConfig, csm, nil, suite.fakeClient, suite.k8sClient)
		if i < len(csm.Spec.Modules[0].Components)-1 {
			assert.NotNil(suite.T(), err)
		} else {
			assert.Nil(suite.T(), err)
		}
	}

	// Restore the status
	for i := range csm.Spec.Modules[0].Components {
		csm.Spec.Modules[0].Components[i].Enabled = &[]bool{true}[0]
	}
}

func (suite *CSMControllerTestSuite) TestReconcileObservabilityErrorBadComponent() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Modules = getObservabilityModule()
	reconciler := suite.createReconciler()

	badComponent := []csmv1.ContainerTemplate{
		{
			Name:    "fake-news",
			Enabled: &[]bool{true}[0],
			Envs: []corev1.EnvVar{
				{
					Name:  "TOPOLOGY_LOG_LEVEL",
					Value: "INFO",
				},
			},
		},
	}

	goodModules := csm.Spec.Modules[0].Components
	csm.Spec.Modules[0].Components = append(badComponent, csm.Spec.Modules[0].Components...)

	err := reconciler.reconcileObservability(ctx, false, operatorConfig, csm, nil, suite.fakeClient, suite.k8sClient)
	assert.NotNil(suite.T(), err)

	csm.Spec.Modules[0].Components = goodModules
}

func (suite *CSMControllerTestSuite) TestReconcileObservabilityErrorBadCert() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Modules = getObservabilityModule()
	reconciler := suite.createReconciler()

	goodModules := csm.Spec.Modules[0].Components
	for index, component := range csm.Spec.Modules[0].Components {
		if component.Name == "topology" {
			csm.Spec.Modules[0].Components[index].Certificate = "bad-cert"
		}
		if component.Name == "metrics-powerscale" {
			csm.Spec.Modules[0].Components[index].Enabled = &[]bool{false}[0]
		}
		if component.Name == "metrics-powerflex" {
			csm.Spec.Modules[0].Components[index].Enabled = &[]bool{false}[0]
		}
	}

	fmt.Printf("[TestReconcileObservabilityErrorBadCert] module components: %+v\n", csm.Spec.Modules[0].Components)

	err := reconciler.reconcileObservability(ctx, false, operatorConfig, csm, nil, suite.fakeClient, suite.k8sClient)
	assert.NotNil(suite.T(), err)

	csm.Spec.Modules[0].Components = goodModules
}

func (suite *CSMControllerTestSuite) TestReconcileAuthorization() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Modules = getAuthProxyServer()
	reconciler := suite.createReconciler()
	badOperatorConfig := utils.OperatorConfig{
		ConfigDirectory: "../in-valid-path",
	}
	err := reconciler.reconcileAuthorization(ctx, false, badOperatorConfig, csm, suite.fakeClient)
	assert.NotNil(suite.T(), err)

	err = reconciler.reconcileAuthorizationCRDS(ctx, badOperatorConfig, csm, suite.fakeClient)
	assert.NotNil(suite.T(), err)

	csm.Spec.Modules[0].Components[0].Enabled = &[]bool{false}[0]
	err = reconciler.reconcileAuthorization(ctx, false, badOperatorConfig, csm, suite.fakeClient)
	assert.NotNil(suite.T(), err)

	csm.Spec.Modules[0].Components[1].Enabled = &[]bool{false}[0]
	err = reconciler.reconcileAuthorization(ctx, false, badOperatorConfig, csm, suite.fakeClient)
	assert.Error(suite.T(), err)

	csm.Spec.Modules[0].Components[2].Enabled = &[]bool{false}[0]
	err = reconciler.reconcileAuthorization(ctx, false, badOperatorConfig, csm, suite.fakeClient)
	assert.Nil(suite.T(), err)

	csm.Spec.Modules[0].Components[3].Enabled = &[]bool{false}[0]
	err = reconciler.reconcileAuthorization(ctx, false, badOperatorConfig, csm, suite.fakeClient)
	assert.Nil(suite.T(), err)

	// Restore the status
	for _, c := range csm.Spec.Modules[0].Components {
		c.Enabled = &[]bool{false}[0]
	}
}

func (suite *CSMControllerTestSuite) TestReconcileAuthorizationBadCert() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Modules = getAuthProxyServer()
	reconciler := suite.createReconciler()

	goodModules := csm.Spec.Modules[0].Components
	for index, component := range csm.Spec.Modules[0].Components {
		if component.Name == string(csmv1.AuthorizationServer) {
			csm.Spec.Modules[0].Components[index].Certificate = "bad-cert"
		}
	}

	fmt.Printf("[TestReconcileAuthorizationBadCert] module components: %+v\n", csm.Spec.Modules[0].Components)

	err := reconciler.reconcileAuthorization(ctx, false, operatorConfig, csm, suite.fakeClient)
	assert.NotNil(suite.T(), err)

	csm.Spec.Modules[0].Components = goodModules
}

func (suite *CSMControllerTestSuite) TestReconcileAppMob() {
	csm := shared.MakeCSM(csmName, suite.namespace, configVersion)
	csm.Spec.Modules = getAppMob()
	reconciler := suite.createReconciler()
	badOperatorConfig := utils.OperatorConfig{
		ConfigDirectory: "../in-valid-path",
	}
	goodOperatorConfig := utils.OperatorConfig{
		ConfigDirectory: "../operatorconfig",
	}
	err := reconciler.reconcileAppMobility(ctx, false, badOperatorConfig, csm, suite.fakeClient)
	assert.NotNil(suite.T(), err)

	er := reconciler.reconcileAppMobilityCRDS(ctx, badOperatorConfig, csm, suite.fakeClient)
	assert.NotNil(suite.T(), er)

	csm.Spec.Modules[0].Components[0].Enabled = &[]bool{false}[0]
	err = reconciler.reconcileAppMobility(ctx, false, badOperatorConfig, csm, suite.fakeClient)
	assert.NotNil(suite.T(), err)

	csm.Spec.Modules[0].Components[1].Enabled = &[]bool{false}[0]
	err = reconciler.reconcileAppMobility(ctx, false, badOperatorConfig, csm, suite.fakeClient)
	assert.Error(suite.T(), err)

	csm.Spec.Modules[0].Components[2].Enabled = &[]bool{false}[0]
	err = reconciler.reconcileAppMobility(ctx, false, goodOperatorConfig, csm, suite.fakeClient)
	assert.Nil(suite.T(), err)

	// Restore the status
	for _, c := range csm.Spec.Modules[0].Components {
		c.Enabled = &[]bool{false}[0]
	}
}

// helper method to create k8s objects
func (suite *CSMControllerTestSuite) makeFakeCSM(name, ns string, withFinalizer bool, modules []csmv1.Module) {
	// make pre-requisite secrets
	sec := shared.MakeSecret(name+"-creds", ns, configVersion)
	err := suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// this secret is required by authorization module
	sec = shared.MakeSecret("karavi-authorization-config", ns, configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// this secret is required by authorization module
	sec = shared.MakeSecret("proxy-authz-tokens", ns, configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// this secret is required by authorization module
	sec = shared.MakeSecret("karavi-config-secret", ns, configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// this secret is required by authorization module
	sec = shared.MakeSecret("proxy-storage-secret", ns, configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// replication secrets
	sec = shared.MakeSecret("skip-replication-cluster-check", utils.ReplicationControllerNameSpace, configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// this secret required by reverseproxy module
	sec = shared.MakeSecret("csirevproxy-tls-secret", ns, configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// this configmap is required by reverseproxy module
	cm := shared.MakeConfigMap("csirevproxy-tls-secret", ns, configVersion)
	err = suite.fakeClient.Create(ctx, cm)
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
	out, _ := json.Marshal(&csm)
	csm.Annotations[previouslyAppliedCustomResource] = string(out)

	err = suite.fakeClient.Create(ctx, &csm)
	assert.Nil(suite.T(), err)
}

func (suite *CSMControllerTestSuite) makeFakeResiliencyCSM(name, ns string, withFinalizer bool, modules []csmv1.Module, driverType string) {
	sec := shared.MakeSecret(name+"-config", ns, configVersion)
	err := suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	csm := shared.MakeCSM(name, ns, configVersion)
	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = v1.DriverType(driverType)

	truebool := true
	sideCarObjEnabledTrue := csmv1.ContainerTemplate{
		Name:            "podmon",
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
	out, _ := json.Marshal(&csm)
	csm.Annotations[previouslyAppliedCustomResource] = string(out)

	err = suite.fakeClient.Create(ctx, &csm)
	assert.Nil(suite.T(), err)
}

// helper method to create k8s objects
func (suite *CSMControllerTestSuite) makeFakeAppMobCSM(name, ns string, _ []csmv1.Module) {
	// this secret required by application-mobility module
	sec := shared.MakeSecret("cloud-creds", ns, configVersion)
	err := suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// this secret required by application-mobility module
	sec = shared.MakeSecret("dls-license", "default", configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// this secret required by application-mobility module
	sec = shared.MakeSecret("iv", "default", configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// this secret required by application-mobility module
	sec = shared.MakeSecret("velero-restic-credentials", ns, configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// this secret required by application-mobility module
	sec = shared.MakeSecret("cert-manager-webhook-ca", ns, configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	csm := shared.MakeModuleCSM(name, ns, configVersion)

	csm.Spec.Modules = getAppMob()
	csm.Spec.Modules[0].ForceRemoveModule = true

	err = suite.fakeClient.Create(ctx, &csm)
	assert.Nil(suite.T(), err)
}

func (suite *CSMControllerTestSuite) makeFakeAuthServerCSM(name, ns string, _ []csmv1.Module) {
	// this secret is required by authorization module
	sec := shared.MakeSecret("karavi-config-secret", ns, configVersion)
	err := suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// this secret is required by authorization module
	sec = shared.MakeSecret("karavi-storage-secret", ns, configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	csm := shared.MakeModuleCSM(name, ns, configVersion)

	csm.Spec.Modules = getAuthProxyServer()
	csm.Spec.Modules[0].ForceRemoveModule = true
	csm.Annotations[configVersionKey] = configVersion

	err = suite.fakeClient.Create(ctx, &csm)
	assert.Nil(suite.T(), err)
}

func (suite *CSMControllerTestSuite) makeFakeAuthServerCSMOCP(name, ns string, _ []csmv1.Module) {
	// this secret is required by authorization module
	sec := shared.MakeSecret("karavi-config-secret", ns, configVersion)
	err := suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	// this secret is required by authorization module
	sec = shared.MakeSecret("karavi-storage-secret", ns, configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)

	csm := shared.MakeModuleCSM(name, ns, configVersion)

	csm.Spec.Modules = getAuthProxyServerOCP()
	csm.Spec.Modules[0].ForceRemoveModule = true
	csm.Annotations[configVersionKey] = configVersion

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

func (suite *CSMControllerTestSuite) makeFakeRevProxyCSM(name string, ns string, withFinalizer bool, modules []v1.Module, driverType string) {
	// Create secrets and config map for Reconcile
	sec := shared.MakeSecret("csirevproxy-tls-secret", ns, configVersion)
	err := suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)
	sec = shared.MakeSecret("powermax-creds", ns, configVersion)
	err = suite.fakeClient.Create(ctx, sec)
	assert.Nil(suite.T(), err)
	cm := shared.MakeConfigMap(driverType+"-reverseproxy-config", ns, configVersion)
	err = suite.fakeClient.Create(ctx, cm)
	assert.Nil(suite.T(), err)

	csm := shared.MakeCSM(name, ns, shared.PmaxConfigVersion)

	csm.Spec.Driver.Common.Image = "image"
	csm.Spec.Driver.CSIDriverType = csmv1.PowerMax
	csm.Spec.Driver.AuthSecret = "powermax-creds"
	if driverType == "badVersion" {
		modules[0].ConfigVersion = "v2.4.0"
	}
	if driverType == "badDriver" {
		csm.Spec.Driver.ConfigVersion = "v2.4.0"
	}
	trueBool := true
	sideCarObjEnabledTrue := csmv1.ContainerTemplate{
		Name:            string(csmv1.ReverseProxyServer),
		Enabled:         &trueBool,
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
	csm.Spec.Modules = modules
	out, _ := json.Marshal(&csm)
	csm.Annotations[previouslyAppliedCustomResource] = string(out)

	err = suite.fakeClient.Create(ctx, &csm)
	assert.Nil(suite.T(), err)
}
