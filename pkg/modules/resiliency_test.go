// Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package modules

import (
	"context"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	drivers "github.com/dell/csm-operator/pkg/drivers"
	utils "github.com/dell/csm-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResiliencyInjectDeployment(t *testing.T) {
	ctx := context.Background()
	tests := map[string]func(t *testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule){
		"success - greenfield injection": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerStore)
			if err != nil {
				panic(err)
			}
			return true, controllerYAML.Deployment, operatorConfig, customResource
		},
		"success - brownfield injection": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerStore)
			if err != nil {
				panic(err)
			}
			newDeployment, err := ResiliencyInjectDeployment(controllerYAML.Deployment, customResource, operatorConfig, string(csmv1.PowerStore))
			if err != nil {
				panic(err)
			}
			return true, *newDeployment, operatorConfig, customResource
		},
		"fail - bad config path": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerStore)
			if err != nil {
				panic(err)
			}
			tmpOperatorConfig := operatorConfig
			tmpOperatorConfig.ConfigDirectory = "bad/path"
			return false, controllerYAML.Deployment, tmpOperatorConfig, customResource
		},
		"success - valid Powerscale driver name": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName)
			if err != nil {
				panic(err)
			}
			newDeployment, err := ResiliencyInjectDeployment(controllerYAML.Deployment, customResource, operatorConfig, string(csmv1.PowerScale))
			if err != nil {
				panic(err)
			}
			return true, *newDeployment, operatorConfig, customResource
		},
		"success - valid Powerflex driver name": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerFlex)
			if err != nil {
				panic(err)
			}
			newDeployment, err := ResiliencyInjectDeployment(controllerYAML.Deployment, customResource, operatorConfig, string(csmv1.PowerFlexName))
			if err != nil {
				panic(err)
			}
			return true, *newDeployment, operatorConfig, customResource
		},
		"success - valid PowerMax driver name": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powermax_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerMax)
			if err != nil {
				panic(err)
			}
			newDeployment, err := ResiliencyInjectDeployment(controllerYAML.Deployment, customResource, operatorConfig, string(csmv1.PowerMax))
			if err != nil {
				panic(err)
			}
			return true, *newDeployment, operatorConfig, customResource
		},
		"fail - bad PowerMax config path": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powermax_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerMax)
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
			newDeployment, err := ResiliencyInjectDeployment(dp, cr, opConfig, string(csmv1.PowerStore))
			if success {
				assert.NoError(t, err)
				if newDeployment == nil {
					panic(err)
				}
				err = CheckApplyContainersResiliency(dp.Spec.Template.Spec.Containers, cr)
				if err != nil {
					panic(err)
				}
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestResiliencyInjectClusterRole(t *testing.T) {
	ctx := context.Background()

	tests := map[string]func(t *testing.T) (bool, rbacv1.ClusterRole, utils.OperatorConfig, csmv1.ContainerStorageModule){
		"success - greenfield injection": func(*testing.T) (bool, rbacv1.ClusterRole, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerStore)
			if err != nil {
				panic(err)
			}
			return true, controllerYAML.Rbac.ClusterRole, operatorConfig, customResource
		},
		"fail - bad config path": func(*testing.T) (bool, rbacv1.ClusterRole, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			tmpOperatorConfig := operatorConfig
			tmpOperatorConfig.ConfigDirectory = "bad/path"

			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerStore)
			if err != nil {
				panic(err)
			}
			return false, controllerYAML.Rbac.ClusterRole, tmpOperatorConfig, customResource
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, clusterRole, opConfig, cr := tc(t)
			newClusterRole, err := ResiliencyInjectClusterRole(clusterRole, cr, opConfig, "controller")
			if success {
				assert.NoError(t, err)
				if newClusterRole == nil {
					panic(err)
				}
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestResiliencyPrecheck(t *testing.T) {
	type fakeControllerRuntimeClientWrapper func(clusterConfigData []byte) (ctrlClient.Client, error)

	tests := map[string]func(t *testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper){
		"valid - resiliency module version provided": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			replica := tmpCR.Spec.Modules[0]
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return true, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"fail - invalid resiliency version": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
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

		"fail - unsupported driver resiliency": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
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

		"success - supported driver resiliency": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = csmv1.PowerStore
			replica := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return true, replica, tmpCR, sourceClient, fakeControllerRuntimeClient
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

			err := ResiliencyPrecheck(context.TODO(), operatorConfig, replica, tmpCR, &fakeReconcile)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestResiliencyInjectDaemonset(t *testing.T) {
	ctx := context.Background()
	tests := map[string]func(t *testing.T) (bool, applyv1.DaemonSetApplyConfiguration, utils.OperatorConfig){
		"success - greenfield injection": func(*testing.T) (bool, applyv1.DaemonSetApplyConfiguration, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			nodeYAML, err := drivers.GetNode(ctx, customResource, operatorConfig, csmv1.PowerStore, "node.yaml")
			if err != nil {
				panic(err)
			}

			return true, nodeYAML.DaemonSetApplyConfig, operatorConfig
		},
		"success - brownfield injection": func(*testing.T) (bool, applyv1.DaemonSetApplyConfiguration, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			nodeYAML, err := drivers.GetNode(ctx, customResource, operatorConfig, csmv1.PowerStore, "node.yaml")
			if err != nil {
				panic(err)
			}
			newDaemonSet, err := ResiliencyInjectDaemonset(nodeYAML.DaemonSetApplyConfig, customResource, operatorConfig, string(csmv1.PowerStore))
			if err != nil {
				panic(err)
			}

			return true, *newDaemonSet, operatorConfig
		},
		"fail - bad config path": func(*testing.T) (bool, applyv1.DaemonSetApplyConfiguration, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			nodeYAML, err := drivers.GetNode(ctx, customResource, operatorConfig, csmv1.PowerStore, "node.yaml")
			if err != nil {
				panic(err)
			}
			tmpOperatorConfig := operatorConfig
			tmpOperatorConfig.ConfigDirectory = "bad/path"
			return false, nodeYAML.DaemonSetApplyConfig, tmpOperatorConfig
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, ds, opConfig := tc(t)
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			newDaemonSet, err := ResiliencyInjectDaemonset(ds, customResource, opConfig, string(csmv1.PowerStore))
			if success {
				assert.NoError(t, err)
				if newDaemonSet == nil {
					panic(err)
				}
			} else {
				assert.Error(t, err)
			}
		})
	}
}
