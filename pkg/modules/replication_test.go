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
	"os"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	drivers "github.com/dell/csm-operator/pkg/drivers"
	utils "github.com/dell/csm-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"

	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

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
			return false, controllerYAML.Rbac.ClusterRole, operatorConfig, customResource
		},
		/*"fail - bad config path": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
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
		},*/
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, clusterRole, opConfig, cr := tc(t)
			newClusterRole, err := ReplicationInjectClusterRole(clusterRole, cr, opConfig)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
			fmt.Println(newClusterRole)

		})
	}
}

func TestReplicationPreCheck(t *testing.T) {
	type fakeControllerRuntimeClientWrapper func(clusterConfigData []byte) (ctrlClient.Client, error)

	tests := map[string]func(t *testing.T) (bool, bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper){
		"success": func(*testing.T) (bool, bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			replica := tmpCR.Spec.Modules[0]

			cluster1ConfigSecret := getSecret(ReplicationControllerNameSpace, "test-cluster-1")
			cluster2ConfigSecret := getSecret(ReplicationControllerNameSpace, "test-cluster-2")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cluster1ConfigSecret, cluster2ConfigSecret).Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				driverSecret1 := getSecret(customResource.Namespace, customResource.Name+"-creds")
				driverSecret2 := getSecret(customResource.Namespace, customResource.Name+"-certs-0")
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(driverSecret1, driverSecret2).Build()
				return clusterClient, nil
			}

			return true, true, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"success - version provided": func(*testing.T) (bool, bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			replica := tmpCR.Spec.Modules[0]
			replica.ConfigVersion = "v1.2.0"

			cluster1ConfigSecret := getSecret(ReplicationControllerNameSpace, "test-cluster-1")
			cluster2ConfigSecret := getSecret(ReplicationControllerNameSpace, "test-cluster-2")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cluster1ConfigSecret, cluster2ConfigSecret).Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				driverSecret1 := getSecret(customResource.Namespace, customResource.Name+"-creds")
				driverSecret2 := getSecret(customResource.Namespace, customResource.Name+"-certs-0")
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(driverSecret1, driverSecret2).Build()
				return clusterClient, nil
			}

			return true, true, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"fail - replica driver pre-check": func(*testing.T) (bool, bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			replica := tmpCR.Spec.Modules[0]
			replica.ConfigVersion = "v1.2.0"

			cluster1ConfigSecret := getSecret(ReplicationControllerNameSpace, "test-cluster-1")
			cluster2ConfigSecret := getSecret(ReplicationControllerNameSpace, "test-cluster-2")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cluster1ConfigSecret, cluster2ConfigSecret).Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				driverSecret1 := getSecret(customResource.Namespace, customResource.Name+"-creds")
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(driverSecret1).Build()
				return clusterClient, nil
			}

			return false, true, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"fail - less tahn one cluster set": func(*testing.T) (bool, bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			replica := tmpCR.Spec.Modules[0]
			replica.ConfigVersion = "v1.2.0"

			for i, component := range replica.Components {
				if component.Name == "dell-replication-controller" {
					for j, env := range component.Envs {
						if env.Name == "CLUSTERS_IDS" {
							replica.Components[i].Envs[j].Value = "test-cluster-1"
						}
					}
				}
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, true, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"fail - no cluster config secret found": func(*testing.T) (bool, bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			replica := tmpCR.Spec.Modules[0]
			replica.ConfigVersion = "v1.2.0"

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, true, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"fail - repctl not installed": func(*testing.T) (bool, bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			replica := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, false, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"fail - unsupported replication version": func(*testing.T) (bool, bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			replica := tmpCR.Spec.Modules[0]
			replica.ConfigVersion = "v100000.0.0"

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, true, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"fail - unsupported driver replication": func(*testing.T) (bool, bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "unsupported-driver"
			replica := tmpCR.Spec.Modules[0]
			replica.ConfigVersion = "v100000.0.0"

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, true, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			oldNewControllerRuntimeClientWrapper := NewControllerRuntimeClientWrapper
			defer func() {
				NewControllerRuntimeClientWrapper = oldNewControllerRuntimeClientWrapper
			}()

			success, setREPCTL, replica, tmpCR, sourceClient, fakeControllerRuntimeClient := tc(t)
			NewControllerRuntimeClientWrapper = fakeControllerRuntimeClient

			if setREPCTL {
				os.Setenv("REPCTL_BINARY", "echo")
				defer os.Unsetenv("REPCTL_BINARY")
			}

			err := ReplicationPrecheck(context.TODO(), operatorConfig, replica, tmpCR, sourceClient)
			if success {
				assert.NoError(t, err)

			} else {
				assert.Error(t, err)
			}

		})
	}
}
