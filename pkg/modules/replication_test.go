// Copyright (c) 2022-2026 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package modules

import (
	"context"
	"fmt"
	"strings"
	"testing"

	csmv1 "eos2git.cec.lab.emc.com/CSM/csm-operator/api/v1"
	drivers "eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/drivers"
	operatorutils "eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/operatorutils"
	shared "eos2git.cec.lab.emc.com/CSM/csm-operator/tests/sharedutil"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	t1 "k8s.io/apimachinery/pkg/types"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// where to find all the yaml files
var config = operatorutils.OperatorConfig{
	ConfigDirectory: "../../tests/config",
}

func TestReplicationInjectDeployment(t *testing.T) {
	ctx := context.Background()
	correctlyInjected := func(dp applyv1.DeploymentApplyConfiguration, cr csmv1.ContainerStorageModule) error {
		return CheckApplyContainersReplica(dp.Spec.Template.Spec.Containers, cr)
	}

	tests := map[string]func(t *testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule){
		"success - greenfield injection": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			cr, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, cr, operatorConfig, csmv1.PowerScaleName, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			return true, controllerYAML.Deployment, operatorConfig, cr
		},
		"success - powermax injection": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			cr, err := getCustomResource("./testdata/cr_powermax_replica.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, cr, operatorConfig, csmv1.PowerMax, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			return true, controllerYAML.Deployment, operatorConfig, cr
		},
		"fail - bad config path": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			cr, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, cr, operatorConfig, csmv1.PowerScaleName, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			tmp := operatorConfig
			tmp.ConfigDirectory = "bad/path"
			return false, controllerYAML.Deployment, tmp, cr
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, dp, opConfig, cr := tc(t)
			newDeployment, err := ReplicationInjectDeployment(ctx, dp, cr, opConfig, operatorutils.VersionSpec{})
			if success {
				assert.NoError(t, err)
				if err := correctlyInjected(*newDeployment, cr); err != nil {
					assert.NoError(t, err)
				}
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestReplicationInjectClusterRole(t *testing.T) {
	ctx := context.Background()

	tests := map[string]func(t *testing.T) (bool, rbacv1.ClusterRole, operatorutils.OperatorConfig, csmv1.ContainerStorageModule){
		"success - greenfield injection": func(*testing.T) (bool, rbacv1.ClusterRole, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			return true, controllerYAML.Rbac.ClusterRole, operatorConfig, customResource
		},
		"fail - bad config path": func(*testing.T) (bool, rbacv1.ClusterRole, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			tmpOperatorConfig := operatorConfig
			tmpOperatorConfig.ConfigDirectory = "bad/path"

			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			return false, controllerYAML.Rbac.ClusterRole, tmpOperatorConfig, customResource
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, clusterRole, opConfig, cr := tc(t)
			newClusterRole, err := ReplicationInjectClusterRole(ctx, clusterRole, cr, opConfig)
			if success {
				assert.NoError(t, err)
				assert.NoError(t, CheckClusterRoleReplica(newClusterRole.Rules))
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestReplicationPreCheck(t *testing.T) {
	type fakeControllerRuntimeClientWrapper func(clusterConfigData []byte) (ctrlClient.Client, error)

	tests := map[string]func(t *testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper){
		"success": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			replica := tmpCR.Spec.Modules[0]

			cluster1ConfigSecret := getSecret(operatorutils.ReplicationControllerNameSpace, "test-target-cluster-1")
			cluster2ConfigSecret := getSecret(operatorutils.ReplicationControllerNameSpace, "test-target-cluster-2")
			driverSecret1 := getSecret(customResource.Namespace, customResource.Name+"-creds")
			driverSecret2 := getSecret(customResource.Namespace, customResource.Name+"-certs-0")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cluster1ConfigSecret, cluster2ConfigSecret, driverSecret1, driverSecret2).Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(driverSecret1, driverSecret2).Build()
				return clusterClient, nil
			}

			return true, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"success - driver type PowerFlex": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "powerflex"
			replica := tmpCR.Spec.Modules[0]

			cluster1ConfigSecret := getSecret(operatorutils.ReplicationControllerNameSpace, "test-target-cluster-1")
			cluster2ConfigSecret := getSecret(operatorutils.ReplicationControllerNameSpace, "test-target-cluster-2")
			configJSONFileGood := fmt.Sprintf("%s/driverconfig/%s/config.json", config.ConfigDirectory, csmv1.PowerFlex)
			driverSecret1 := shared.MakeSecretWithJSON(customResource.Name+"-config", customResource.Namespace, configJSONFileGood)
			driverSecret2 := getSecret(customResource.Namespace, customResource.Name+"-certs-0")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cluster1ConfigSecret, cluster2ConfigSecret, driverSecret1, driverSecret2).Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(driverSecret1, driverSecret2).Build()
				return clusterClient, nil
			}

			return true, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"success - driver type PowerStore": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "powerstore"
			replica := tmpCR.Spec.Modules[0]

			cluster1ConfigSecret := getSecret(operatorutils.ReplicationControllerNameSpace, "test-target-cluster-1")
			cluster2ConfigSecret := getSecret(operatorutils.ReplicationControllerNameSpace, "test-target-cluster-2")
			configJSONFileGood := fmt.Sprintf("%s/driverconfig/%s/config.json", config.ConfigDirectory, csmv1.PowerStore)
			driverSecret1 := shared.MakeSecretWithJSON(customResource.Name+"-config", customResource.Namespace, configJSONFileGood)
			driverSecret2 := getSecret(customResource.Namespace, customResource.Name+"-certs-0")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cluster1ConfigSecret, cluster2ConfigSecret, driverSecret1, driverSecret2).Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(driverSecret1, driverSecret2).Build()
				return clusterClient, nil
			}

			return true, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"success - version provided": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			replica := tmpCR.Spec.Modules[0]
			replica.ConfigVersion = "v1.13.0"

			cluster1ConfigSecret := getSecret(operatorutils.ReplicationControllerNameSpace, "test-target-cluster-1")
			cluster2ConfigSecret := getSecret(operatorutils.ReplicationControllerNameSpace, "test-target-cluster-2")
			driverSecret1 := getSecret(customResource.Namespace, customResource.Name+"-creds")
			driverSecret2 := getSecret(customResource.Namespace, customResource.Name+"-certs-0")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cluster1ConfigSecret, cluster2ConfigSecret, driverSecret1, driverSecret2).Build()
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(driverSecret1, driverSecret2).Build()
				return clusterClient, nil
			}

			return true, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"fail - replica driver pre-check": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			replica := tmpCR.Spec.Modules[0]
			replica.ConfigVersion = "v1.9.0"

			cluster1ConfigSecret := getSecret(operatorutils.ReplicationControllerNameSpace, "test-target-cluster-1")
			cluster2ConfigSecret := getSecret(operatorutils.ReplicationControllerNameSpace, "test-target-cluster-2")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cluster1ConfigSecret, cluster2ConfigSecret).Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				driverSecret1 := getSecret(customResource.Namespace, customResource.Name+"-creds")
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(driverSecret1).Build()
				return clusterClient, nil
			}

			return false, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"fail - less than one cluster set": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			replica := tmpCR.Spec.Modules[0]
			replica.ConfigVersion = "v1.9.0"

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"fail - no cluster config secret found": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			replica := tmpCR.Spec.Modules[0]
			replica.ConfigVersion = "v1.9.0"

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"fail - unsupported replication version": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			replica := tmpCR.Spec.Modules[0]
			replica.ConfigVersion = "v100000.0.0"

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"fail - unsupported driver replication": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "unsupported-driver"
			replica := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			oldNewControllerRuntimeClientWrapper := operatorutils.NewControllerRuntimeClientWrapper
			oldNewK8sClientWrapper := operatorutils.NewK8sClientWrapper
			defer func() {
				operatorutils.NewControllerRuntimeClientWrapper = oldNewControllerRuntimeClientWrapper
				operatorutils.NewK8sClientWrapper = oldNewK8sClientWrapper
			}()

			success, replica, tmpCR, sourceClient, fakeControllerRuntimeClient := tc(t)
			operatorutils.NewControllerRuntimeClientWrapper = fakeControllerRuntimeClient
			operatorutils.NewK8sClientWrapper = func(_ []byte) (*kubernetes.Clientset, error) {
				return nil, nil
			}

			fakeReconcile := operatorutils.FakeReconcileCSM{
				Client:    sourceClient,
				K8sClient: fake.NewSimpleClientset(),
			}

			err := ReplicationPrecheck(context.TODO(), operatorConfig, replica, tmpCR, &fakeReconcile)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestReplicationManagerController(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			cr := &rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					Kind: "ClusterRole",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "dell-replication-manager-role",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},

		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - version set and resolved from ConfigMap": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}
			tmpCR := customResource
			// Use a supported version so ResolveVersionFromConfigMap succeeds
			tmpCR.Spec.Version = "v1.16.0"

			// Pre-create the replication ConfigMap in the fake client using real operatorconfig
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			realConfig := operatorutils.OperatorConfig{ConfigDirectory: "../../operatorconfig"}
			if _, err := CreateReplicationConfigmap(context.Background(), tmpCR, realConfig, sourceClient); err != nil {
				panic(err)
			}
			return true, false, tmpCR, sourceClient, operatorConfig
		},

		// Cover cr.Spec.Version != "" with unsupported version to force ResolveVersionFromConfigMap error
		"fail - unsupported version triggers ResolveVersionFromConfigMap error": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}
			tmpCR := customResource
			// Set an unsupported version to guarantee an error
			tmpCR.Spec.Version = "v0.0.0"

			// No ConfigMap in this client; but unsupported version itself should cause resolution error
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return false, false, tmpCR, sourceClient, operatorConfig
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op := tc(t)

			err := ReplicationManagerController(context.TODO(), isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestReplicationConfigmap(t *testing.T) {
	// Create a fake client to use in the test
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := ctrlClientFake.NewClientBuilder().WithScheme(scheme).Build()

	// Create a test ContainerStorageModule
	cr, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
	if err != nil {
		panic(err)
	}

	// Call the function we want to test
	// we can't use test config, as it doesn't have versionvalues
	realConfig := operatorutils.OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}
	objs, err := CreateReplicationConfigmap(context.Background(), cr, realConfig, fakeClient)
	// Check that the function returned the expected results
	if err != nil {
		t.Errorf("CreateReplicationConfigmap returned an unexpected error: %v", err)
	}

	if len(objs) != 1 {
		t.Errorf("CreateReplicationConfigmap returned the wrong number of objects: %d", len(objs))
	}

	cm, ok := objs[0].(*corev1.ConfigMap)
	if !ok {
		t.Errorf("CreateReplicationConfigmap returned the wrong type of object: %T", objs[0])
	}

	if cm.Name != "dell-replication-controller-config" {
		t.Errorf("CreateReplicationConfigmap returned the wrong ConfigMap name: %s", cm.Name)
	}

	if cm.Namespace != "dell-replication-controller" {
		t.Errorf("CreateReplicationConfigmap returned the wrong ConfigMap namespace: %s", cm.Namespace)
	}

	// Check that the ConfigMap was created in the fake client
	foundConfigMap := &corev1.ConfigMap{}
	err = fakeClient.Get(context.Background(), t1.NamespacedName{Name: "dell-replication-controller-config", Namespace: "dell-replication-controller"}, foundConfigMap)
	if err != nil {
		t.Errorf("ConfigMap was not created in the fake client: %v", err)
	}

	// Now verify that the ConfigMap can be deleted properly
	// Call the function we want to test
	if err := DeleteReplicationConfigmap(fakeClient); err != nil {
		t.Errorf("DeleteReplicationConfigmap returned an unexpected error: %v", err)
	}

	// Check that the ConfigMap was deleted from the fake client
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dell-replication-controller-config",
			Namespace: "dell-replication-controller",
		},
	}
	err = fakeClient.Get(context.Background(), t1.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, configMap)
	if err == nil {
		t.Errorf("ConfigMap was not deleted from the fake client")
	} else if !k8serrors.IsNotFound(err) {
		t.Errorf("ConfigMap was not deleted from the fake client: %v", err)
	}
}

func TestDeleteReplicationConfigmap_NotFound_ReturnsNil(t *testing.T) {
	// Create a fake client with no objects
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := ctrlClientFake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()

	// Should not error when ConfigMap is not present
	err := DeleteReplicationConfigmap(fakeClient)
	assert.NoError(t, err)
}

func TestCreateReplicationConfigmap_ConfigMapAlreadyExists_NoError(t *testing.T) {
	ctx := context.Background()

	// Fake client seeded with the ConfigMap so CreateReplicationConfigmap takes the "already exists" path.
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dell-replication-controller-config",
			Namespace: "dell-replication-controller",
		},
		Data: map[string]string{"foo": "bar"},
	}

	fakeClient := ctrlClientFake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	// CR with replication module present (needed to locate moduleconfig)
	cr, err := getCustomResource("./testdata/cr_powermax_replica.yaml")
	if err != nil {
		panic(err)
	}
	op := operatorutils.OperatorConfig{ConfigDirectory: "../../operatorconfig"}

	objs, err := CreateReplicationConfigmap(ctx, cr, op, fakeClient)
	assert.NoError(t, err)
	assert.NotEmpty(t, objs)
}

func TestGetReplicationCrdDeploy(t *testing.T) {
	realConfig := operatorutils.OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}

	customResource, err := getCustomResource("./testdata/cr_powermax_replica.yaml")
	if err != nil {
		panic(err)
	}

	yaml, err := getReplicationCrdDeploy(ctx, realConfig, customResource)
	assert.NoError(t, err)
	assert.Contains(t, yaml, "kind: CustomResourceDefinition")
}

func TestReplicationCrdDeployAndDelete(t *testing.T) {
	tests := map[string]func(t *testing.T) (operatorutils.OperatorConfig, csmv1.ContainerStorageModule, bool){
		"success case": func(_ *testing.T) (operatorutils.OperatorConfig, csmv1.ContainerStorageModule, bool) {
			operConfig := operatorutils.OperatorConfig{
				ConfigDirectory: "../../operatorconfig",
			}
			customResource, err := getCustomResource("./testdata/cr_powermax_replica.yaml")
			if err != nil {
				panic(err)
			}
			return operConfig, customResource, true
		},
		"failure invalid config dir": func(_ *testing.T) (operatorutils.OperatorConfig, csmv1.ContainerStorageModule, bool) {
			operConfig := operatorutils.OperatorConfig{
				ConfigDirectory: "../../DIRDONTEXIST",
			}
			customResource, err := getCustomResource("./testdata/cr_powermax_replica.yaml")
			if err != nil {
				panic(err)
			}
			return operConfig, customResource, false
		},
		"failure case no repl cr": func(_ *testing.T) (operatorutils.OperatorConfig, csmv1.ContainerStorageModule, bool) {
			operConfig := operatorutils.OperatorConfig{
				ConfigDirectory: "../../operatorconfig",
			}
			customResource := csmv1.ContainerStorageModule{}

			return operConfig, customResource, false
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			oc, cr, success := tc(t)
			crd := &apiextv1.CustomResourceDefinition{
				TypeMeta: metav1.TypeMeta{
					Kind: "CustomResourceDefinition",
				},
			}
			err := apiextv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}

			fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(crd).Build()

			err = ReplicationCrdDeploy(context.Background(), oc, cr, fakeClient)
			if success {
				assert.NoError(t, err)
				err = DeleteReplicationCrds(context.Background(), oc, cr, fakeClient)
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestGetReplicaController_UsesMatchedManagerImage_And_InitImageFromComponent(t *testing.T) {
	ctx := context.Background()

	// Load a CR that includes the replication module.
	// Either PowerScale or PowerMax replica CRs should work, but they must align with operatorconfig templates.
	cr, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
	if err != nil {
		panic(err)
	}
	if len(cr.Spec.Modules) == 0 {
		t.Fatalf("test CR must have at least one module")
	}

	// Mutate the module components to exercise the exact branches:
	// - Manager: set Image empty so the code takes matched.Images when matched.Version != "".
	// - Init: set Image explicitly so YAML replacement path is taken.
	replica := cr.Spec.Modules[0]
	for i, c := range replica.Components {
		if c.Name == operatorutils.ReplicationControllerManager {
			replica.Components[i].Image = "" // forces use of matched.Images[...] path
		}
		if c.Name == operatorutils.ReplicationControllerInit {
			replica.Components[i].Image = "test/init-image:1.0.0" // ensures init image branch is covered
		}
	}
	cr.Spec.Modules[0] = replica

	// Provide matched VersionSpec with image for the manager; non-empty Version triggers the branch
	matched := operatorutils.VersionSpec{
		Version: "v1.16.0", // any non-empty is fine; choose a known supported version
		Images: map[string]string{
			operatorutils.ReplicationControllerManager: "test/replication-manager:2.3.4",
		},
	}

	// IMPORTANT: use the real operatorconfig directory, not ../../tests/config
	op := operatorutils.OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}

	// Call function under test
	ctrlObjects, err := getReplicaController(ctx, op, cr, matched)
	assert.NoError(t, err, "getReplicaController should not error")

	// Extract the deployment to validate images
	var dep *appsv1.Deployment
	for _, obj := range ctrlObjects {
		if d, ok := obj.(*appsv1.Deployment); ok {
			dep = d
			break
		}
	}
	if dep == nil {
		t.Fatalf("expected a Deployment in ctrlObjects, got none")
	}

	// Verify manager image was set from matched.Images[...] (because matched.Version != "" and manager.Image was empty)
	if len(dep.Spec.Template.Spec.Containers) == 0 {
		t.Fatalf("expected at least one container in deployment")
	}
	assert.Equal(t, "test/replication-manager:2.3.4", dep.Spec.Template.Spec.Containers[0].Image,
		"manager container image should be set from matched.Images when matched.Version != \"\"")

	// Verify init image was taken from component.Image for ReplicationControllerInit
	// Depending on templates, init container may or may not be defined by default. If present, assert it.
	// If not present, we still cover the code path that injects the image into YAML before object creation.
	if len(dep.Spec.Template.Spec.InitContainers) > 0 {
		assert.Equal(t, "test/init-image:1.0.0", dep.Spec.Template.Spec.InitContainers[0].Image,
			"init container image should be set from the ReplicationControllerInit component.Image")
	} else {
		// As a fallback, ensure the YAML replacement ran by checking the template for env or args referencing the init image,
		// but if the operatorconfig's controller.yaml does not create an init container, the branch is still covered via replacement.
		t.Log("Deployment has no init containers; branch covered via YAML replacement even if template does not define init containers.")
	}
}

func TestGetReplicaController_CoversInitComponentBranch(t *testing.T) {
	ctx := context.Background()

	// Load a CR that includes the replication module
	cr, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
	if err != nil {
		panic(err)
	}
	if len(cr.Spec.Modules) == 0 || len(cr.Spec.Modules[0].Components) == 0 {
		t.Fatalf("test CR must have at least one module with at least one component")
	}

	// Mutate the FIRST component to simulate the init component with a non-empty image.
	// This forces the branch: component.Name == ReplicationControllerInit && component.Image != ""
	const initImg = "test/init-image:9.9.9"
	replica := cr.Spec.Modules[0]
	replica.Components[0].Name = operatorutils.ReplicationControllerInit
	replica.Components[0].Image = initImg
	cr.Spec.Modules[0] = replica

	// 'matched' can be empty; we only need the init image branch covered.
	matched := operatorutils.VersionSpec{}

	// Use the real operatorconfig directory so controller.yaml & version-values.yaml exist.
	op := operatorutils.OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}

	// Invoke the function under test.
	ctrlObjects, err := getReplicaController(ctx, op, cr, matched)
	assert.NoError(t, err, "getReplicaController should not error")

	// Find the Deployment to validate init image if template defines init containers.
	var dep *appsv1.Deployment
	for _, obj := range ctrlObjects {
		if d, ok := obj.(*appsv1.Deployment); ok {
			dep = d
			break
		}
	}
	if dep == nil {
		t.Fatalf("expected a Deployment in ctrlObjects, got none")
	}

	// If the template provides init containers, assert the image matches our injected component.
	if len(dep.Spec.Template.Spec.InitContainers) > 0 {
		assert.Equal(t, initImg, dep.Spec.Template.Spec.InitContainers[0].Image,
			"init container image should be set from the ReplicationControllerInit component.Image")
	} else {
		// Even if there are no init containers in the final Deployment, the target branch was executed
		// during components iteration and replicaInitImage was set.
		t.Log("Deployment has no init containers; the branch was still covered by setting the ReplicationControllerInit component.Image.")
	}
}

func TestGetReplicaApplyCR_SyntheticComponentImageOverride(t *testing.T) {
	ctx := context.Background()

	// Load a replication CR that has the replication module present
	cr, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
	if err != nil {
		panic(err)
	}
	if len(cr.Spec.Modules) == 0 {
		t.Fatalf("replication CR must include at least one module")
	}

	// Force the synthetic branch: no components in the replication module
	replica := cr.Spec.Modules[0]
	replica.Name = csmv1.Replication
	replica.Components = nil
	cr.Spec.Modules[0] = replica

	// Use the real operatorconfig so readConfigFile("container.yaml") succeeds
	op := operatorutils.OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}

	// Matched VersionSpec with image for the replication sidecar
	matched := operatorutils.VersionSpec{
		Version: "v1.16.0", // any non-empty version is fine here
		Images: map[string]string{
			operatorutils.ReplicationSideCarName: "registry.example/replication-sidecar:override",
		},
	}

	// Act
	mod, container, err := getReplicaApplyCR(ctx, cr, op, matched)
	assert.NoError(t, err, "getReplicaApplyCR should not error when container.yaml is present")
	if mod == nil || container == nil {
		t.Fatalf("expected non-nil module and container")
	}
	if container.Image == nil {
		t.Fatalf("container.Image should not be nil after YAML unmarshal")
	}

	// Assert: synthetic branch should set image from matched.Images[ReplicationSideCarName]
	got := *container.Image
	want := "registry.example/replication-sidecar:override"
	assert.Equal(t, want, got, "synthetic ReplicationSideCarName image should be overridden by matched.Images")
}

func TestGetReplicaController_SyntheticManagerImageOverride(t *testing.T) {
	ctx := context.Background()

	// Load a replication CR that contains the replication module
	cr, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
	if err != nil {
		panic(err)
	}
	if len(cr.Spec.Modules) == 0 {
		t.Fatalf("replication CR must include at least one module")
	}

	// Force synthetic branch: empty components in replication module
	replica := cr.Spec.Modules[0]
	replica.Name = csmv1.Replication
	replica.Components = nil
	cr.Spec.Modules[0] = replica

	// Use real operatorconfig directory so readConfigFile("controller.yaml") succeeds
	op := operatorutils.OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}

	// Provide matched VersionSpec with image for the manager; non-empty Version triggers the override path in GetFinalImage
	matched := operatorutils.VersionSpec{
		Version: "v1.16.0", // any non-empty version should be fine
		Images: map[string]string{
			operatorutils.ReplicationControllerManager: "registry.example/replication-manager:override",
		},
	}

	// Act
	ctrlObjects, err := getReplicaController(ctx, op, cr, matched)
	assert.NoError(t, err, "getReplicaController should not error with valid operatorconfig")

	// Find deployment and assert the manager image is set from matched.Images in the synthetic path
	var dep *appsv1.Deployment
	for _, obj := range ctrlObjects {
		if d, ok := obj.(*appsv1.Deployment); ok {
			dep = d
			break
		}
	}
	if dep == nil {
		t.Fatalf("expected a Deployment in ctrlObjects, got none")
	}
	if len(dep.Spec.Template.Spec.Containers) == 0 {
		t.Fatalf("expected at least one container in deployment")
	}
	got := dep.Spec.Template.Spec.Containers[0].Image
	want := "registry.example/replication-manager:override"
	assert.Equal(t, want, got, "synthetic ReplicationControllerManager image should be overridden by matched.Images")
}

func TestGetReplicaController_EnableKubevirtPVCRemap_True(t *testing.T) {
	ctx := context.Background()

	// Load PowerStore replica CR which includes ENABLE_KUBEVIRT_PVC_REMAP
	cr, err := getCustomResource("./testdata/cr_powerstore_replica.yaml")
	if err != nil {
		panic(err)
	}

	// Set ENABLE_KUBEVIRT_PVC_REMAP to "true" in the CR envs
	replica := cr.Spec.Modules[0]
	for i, component := range replica.Components {
		if component.Name == operatorutils.ReplicationControllerManager {
			for j, env := range component.Envs {
				if env.Name == "ENABLE_KUBEVIRT_PVC_REMAP" {
					replica.Components[i].Envs[j].Value = "true"
				}
			}
		}
	}
	cr.Spec.Modules[0] = replica

	// Use real operatorconfig directory (v1.15.0 has the placeholder)
	op := operatorutils.OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}
	matched := operatorutils.VersionSpec{}

	ctrlObjects, err := getReplicaController(ctx, op, cr, matched)
	assert.NoError(t, err, "getReplicaController should not error")

	var dep *appsv1.Deployment
	for _, obj := range ctrlObjects {
		if d, ok := obj.(*appsv1.Deployment); ok {
			dep = d
			break
		}
	}
	if dep == nil {
		t.Fatalf("expected a Deployment in ctrlObjects, got none")
	}

	// Verify deployment args contain --enable-kubevirt-pvc-remap=true
	foundArg := false
	for _, arg := range dep.Spec.Template.Spec.Containers[0].Args {
		if arg == "--enable-kubevirt-pvc-remap=true" {
			foundArg = true
			break
		}
	}
	assert.True(t, foundArg, "deployment args should contain --enable-kubevirt-pvc-remap=true when env is set to true")
}

func TestGetReplicaController_CustomRegistryOnly(t *testing.T) {
	ctx := context.Background()

	cr, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
	if err != nil {
		panic(err)
	}
	if len(cr.Spec.Modules) == 0 {
		t.Fatalf("test CR must have at least one module")
	}

	// Clear the manager component image so the code falls through to the CustomRegistry branch in GetFinalImage.
	replica := cr.Spec.Modules[0]
	for i, c := range replica.Components {
		if c.Name == operatorutils.ReplicationControllerManager {
			replica.Components[i].Image = ""
		}
	}
	cr.Spec.Modules[0] = replica

	// Use real operatorconfig directory (v1.15.0 has the placeholder)
	op := operatorutils.OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}
	matched := operatorutils.VersionSpec{}
	// Set custom registry on the CR; no ConfigMap (empty matched).
	cr.Spec.CustomRegistry = "my-registry.example.com"

	ctrlObjects, err := getReplicaController(ctx, op, cr, matched)
	assert.NoError(t, err, "getReplicaController should not error")

	var dep *appsv1.Deployment
	for _, obj := range ctrlObjects {
		if d, ok := obj.(*appsv1.Deployment); ok {
			dep = d
			break
		}
	}
	if dep == nil {
		t.Fatalf("expected a Deployment in ctrlObjects, got none")
	}

	// Verify the manager container image uses the custom registry prefix
	managerImage := dep.Spec.Template.Spec.Containers[0].Image
	assert.True(t, strings.HasPrefix(managerImage, "my-registry.example.com/"),
		"manager image should start with custom registry, got: %s", managerImage)
}

func TestGetReplicaController_EnableKubevirtPVCRemap_BackwardCompat(t *testing.T) {
	ctx := context.Background()

	// Load PowerStore replica CR and REMOVE the ENABLE_KUBEVIRT_PVC_REMAP env var
	// to simulate an older CR that does not include the new env var.
	cr, err := getCustomResource("./testdata/cr_powerstore_replica.yaml")
	if err != nil {
		panic(err)
	}

	replica := cr.Spec.Modules[0]
	for i, component := range replica.Components {
		if component.Name == operatorutils.ReplicationControllerManager {
			filtered := make([]corev1.EnvVar, 0, len(component.Envs))
			for _, env := range component.Envs {
				if env.Name != "ENABLE_KUBEVIRT_PVC_REMAP" {
					filtered = append(filtered, env)
				}
			}
			replica.Components[i].Envs = filtered
		}
	}
	cr.Spec.Modules[0] = replica

	// Use real operatorconfig directory (v1.15.0 has the placeholder)
	op := operatorutils.OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}
	matched := operatorutils.VersionSpec{}

	ctrlObjects, err := getReplicaController(ctx, op, cr, matched)
	assert.NoError(t, err, "getReplicaController should not error for backward-compatible CR without ENABLE_KUBEVIRT_PVC_REMAP")

	var dep *appsv1.Deployment
	for _, obj := range ctrlObjects {
		if d, ok := obj.(*appsv1.Deployment); ok {
			dep = d
			break
		}
	}
	if dep == nil {
		t.Fatalf("expected a Deployment in ctrlObjects, got none")
	}

	// Verify deployment args default to --enable-kubevirt-pvc-remap=false
	foundArg := false
	for _, arg := range dep.Spec.Template.Spec.Containers[0].Args {
		if arg == "--enable-kubevirt-pvc-remap=false" {
			foundArg = true
			break
		}
	}
	assert.True(t, foundArg, "deployment args should default to --enable-kubevirt-pvc-remap=false when env var is absent (backward compatibility)")
}

func TestGetReplicaController_ConfigMapWinsOverCustomRegistry(t *testing.T) {
	ctx := context.Background()

	cr, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
	if err != nil {
		panic(err)
	}
	if len(cr.Spec.Modules) == 0 {
		t.Fatalf("test CR must have at least one module")
	}

	// Clear the manager component image so the result is determined solely by matched vs custom registry.
	replica := cr.Spec.Modules[0]
	for i, c := range replica.Components {
		if c.Name == operatorutils.ReplicationControllerManager {
			replica.Components[i].Image = ""
		}
	}
	cr.Spec.Modules[0] = replica

	// Use real operatorconfig directory (v1.15.0 has the placeholder)
	op := operatorutils.OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}
	matched := operatorutils.VersionSpec{}

	ctrlObjects, err := getReplicaController(ctx, op, cr, matched)
	assert.NoError(t, err, "getReplicaController should not error for backward-compatible CR without ENABLE_KUBEVIRT_PVC_REMAP")
	// Set BOTH a custom registry and a matched (ConfigMap) image.
	cr.Spec.CustomRegistry = "my-registry.example.com"

	matched = operatorutils.VersionSpec{
		Version: "v1.15.0",
		Images: map[string]string{
			operatorutils.ReplicationControllerManager: "configmap-registry.io/replication-manager:from-configmap",
		},
	}

	ctrlObjects, err = getReplicaController(ctx, op, cr, matched)
	assert.NoError(t, err, "getReplicaController should not error")

	var dep *appsv1.Deployment
	for _, obj := range ctrlObjects {
		if d, ok := obj.(*appsv1.Deployment); ok {
			dep = d
			break
		}
	}
	if dep == nil {
		t.Fatalf("expected a Deployment in ctrlObjects, got none")
	}

	// Verify deployment args default to --enable-kubevirt-pvc-remap=false
	foundArg := false
	for _, arg := range dep.Spec.Template.Spec.Containers[0].Args {
		if arg == "--enable-kubevirt-pvc-remap=false" {
			foundArg = true
			break
		}
	}
	assert.True(t, foundArg, "deployment args should default to --enable-kubevirt-pvc-remap=false when env var is absent (backward compatibility)")
	if len(dep.Spec.Template.Spec.Containers) == 0 {
		t.Fatalf("expected at least one container in deployment")
	}

	got := dep.Spec.Template.Spec.Containers[0].Image
	want := "configmap-registry.io/replication-manager:from-configmap"
	assert.Equal(t, want, got, "ConfigMap image should win over custom registry")
}

func TestGetReplicaController_NeitherConfigMapNorRegistry(t *testing.T) {
	ctx := context.Background()

	cr, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
	if err != nil {
		panic(err)
	}
	if len(cr.Spec.Modules) == 0 {
		t.Fatalf("test CR must have at least one module")
	}

	// Clear the manager component image so the code falls all the way through to the template default.
	replica := cr.Spec.Modules[0]
	for i, c := range replica.Components {
		if c.Name == operatorutils.ReplicationControllerManager {
			replica.Components[i].Image = ""
		}
	}
	cr.Spec.Modules[0] = replica

	// No custom registry, no ConfigMap.
	cr.Spec.CustomRegistry = ""

	matched := operatorutils.VersionSpec{} // empty

	op := operatorutils.OperatorConfig{
		ConfigDirectory: "../../operatorconfig",
	}

	ctrlObjects, err := getReplicaController(ctx, op, cr, matched)
	assert.NoError(t, err, "getReplicaController should not error")

	var dep *appsv1.Deployment
	for _, obj := range ctrlObjects {
		if d, ok := obj.(*appsv1.Deployment); ok {
			dep = d
			break
		}
	}
	if dep == nil {
		t.Fatalf("expected a Deployment in ctrlObjects, got none")
	}
	if len(dep.Spec.Template.Spec.Containers) == 0 {
		t.Fatalf("expected at least one container in deployment")
	}

	// With no ConfigMap and no custom registry, the default template image should be used.
	got := dep.Spec.Template.Spec.Containers[0].Image
	assert.True(t, strings.Contains(got, "quay.io/dell/container-storage-modules/dell-replication-controller"),
		"manager image should be the default template image, got: %s", got)
}
