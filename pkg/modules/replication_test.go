// Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	drivers "github.com/dell/csm-operator/pkg/drivers"
	utils "github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/tests/shared"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	t1 "k8s.io/apimachinery/pkg/types"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// where to find all the yaml files
var config = utils.OperatorConfig{
	ConfigDirectory: "../../tests/config",
}

func TestReplicationInjectDeployment(t *testing.T) {
	ctx := context.Background()
	correctlyInjected := func(dp applyv1.DeploymentApplyConfiguration, cr csmv1.ContainerStorageModule) error {
		return CheckApplyContainersReplica(dp.Spec.Template.Spec.Containers, cr)
	}

	tests := map[string]func(t *testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule){
		"success - greenfield injection": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName)
			if err != nil {
				panic(err)
			}
			return true, controllerYAML.Deployment, operatorConfig, customResource
		},
		"success - powermax injection": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powermax_replica.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerMax)
			if err != nil {
				panic(err)
			}
			return true, controllerYAML.Deployment, operatorConfig, customResource
		},
		"fail - bad config path": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName)
			if err != nil {
				panic(err)
			}
			tmpOperatorConfig := operatorConfig
			tmpOperatorConfig.ConfigDirectory = "bad/path"

			return false, controllerYAML.Deployment, tmpOperatorConfig, customResource
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, dp, opConfig, cr := tc(t)
			newDeployment, err := ReplicationInjectDeployment(dp, cr, opConfig)
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

	tests := map[string]func(t *testing.T) (bool, rbacv1.ClusterRole, utils.OperatorConfig, csmv1.ContainerStorageModule){
		"success - greenfield injection": func(*testing.T) (bool, rbacv1.ClusterRole, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName)
			if err != nil {
				panic(err)
			}
			return true, controllerYAML.Rbac.ClusterRole, operatorConfig, customResource
		},
		"fail - bad config path": func(*testing.T) (bool, rbacv1.ClusterRole, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			tmpOperatorConfig := operatorConfig
			tmpOperatorConfig.ConfigDirectory = "bad/path"

			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName)
			if err != nil {
				panic(err)
			}
			return false, controllerYAML.Rbac.ClusterRole, tmpOperatorConfig, customResource
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, clusterRole, opConfig, cr := tc(t)
			newClusterRole, err := ReplicationInjectClusterRole(clusterRole, cr, opConfig)
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

			cluster1ConfigSecret := getSecret(utils.ReplicationControllerNameSpace, "test-target-cluster-1")
			cluster2ConfigSecret := getSecret(utils.ReplicationControllerNameSpace, "test-target-cluster-2")
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

			cluster1ConfigSecret := getSecret(utils.ReplicationControllerNameSpace, "test-target-cluster-1")
			cluster2ConfigSecret := getSecret(utils.ReplicationControllerNameSpace, "test-target-cluster-2")
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

		"success - version provided": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			replica := tmpCR.Spec.Modules[0]
			replica.ConfigVersion = "v1.10.0"

			cluster1ConfigSecret := getSecret(utils.ReplicationControllerNameSpace, "test-target-cluster-1")
			cluster2ConfigSecret := getSecret(utils.ReplicationControllerNameSpace, "test-target-cluster-2")
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

			cluster1ConfigSecret := getSecret(utils.ReplicationControllerNameSpace, "test-target-cluster-1")
			cluster2ConfigSecret := getSecret(utils.ReplicationControllerNameSpace, "test-target-cluster-2")

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

			for i, component := range tmpCR.Spec.Modules[0].Components {
				if component.Name == utils.ReplicationControllerManager {
					for j, env := range component.Envs {
						if env.Name == "TARGET_CLUSTERS_IDS" {
							tmpCR.Spec.Modules[0].Components[i].Envs[j].Value = ""
						}
					}
				}
			}

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
			oldNewControllerRuntimeClientWrapper := utils.NewControllerRuntimeClientWrapper
			oldNewK8sClientWrapper := utils.NewK8sClientWrapper
			defer func() {
				utils.NewControllerRuntimeClientWrapper = oldNewControllerRuntimeClientWrapper
				utils.NewK8sClientWrapper = oldNewK8sClientWrapper
			}()

			success, replica, tmpCR, sourceClient, fakeControllerRuntimeClient := tc(t)
			utils.NewControllerRuntimeClientWrapper = fakeControllerRuntimeClient
			utils.NewK8sClientWrapper = func(_ []byte) (*kubernetes.Clientset, error) {
				return nil, nil
			}

			fakeReconcile := utils.FakeReconcileCSM{
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
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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

		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
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
	realConfig := utils.OperatorConfig{
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
