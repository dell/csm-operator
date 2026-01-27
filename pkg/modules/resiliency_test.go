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
	"fmt"
	"strings"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	drivers "github.com/dell/csm-operator/pkg/drivers"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResiliencyInjectDeployment(t *testing.T) {
	ctx := context.Background()
	tests := map[string]func(t *testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule){
		"success - greenfield injection": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerStore, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			return true, controllerYAML.Deployment, operatorConfig, customResource
		},
		"success - brownfield injection": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerStore, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			newDeployment, err := ResiliencyInjectDeployment(ctx, controllerYAML.Deployment, customResource, operatorConfig, string(csmv1.PowerStore), operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			return true, *newDeployment, operatorConfig, customResource
		},
		"fail - bad config path": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerStore, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			tmpOperatorConfig := operatorConfig
			tmpOperatorConfig.ConfigDirectory = "bad/path"
			return false, controllerYAML.Deployment, tmpOperatorConfig, customResource
		},
		"success - valid Powerscale driver name": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			newDeployment, err := ResiliencyInjectDeployment(ctx, controllerYAML.Deployment, customResource, operatorConfig, string(csmv1.PowerScale), operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			return true, *newDeployment, operatorConfig, customResource
		},
		"success - valid Powerflex driver name": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerFlex, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			newDeployment, err := ResiliencyInjectDeployment(ctx, controllerYAML.Deployment, customResource, operatorConfig, string(csmv1.PowerFlexName), operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			return true, *newDeployment, operatorConfig, customResource
		},
		"success - valid PowerMax driver name": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powermax_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerMax, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			newDeployment, err := ResiliencyInjectDeployment(ctx, controllerYAML.Deployment, customResource, operatorConfig, string(csmv1.PowerMax), operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			return true, *newDeployment, operatorConfig, customResource
		},
		"fail - bad PowerMax config path": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powermax_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerMax, operatorutils.VersionSpec{})
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
			newDeployment, err := ResiliencyInjectDeployment(ctx, dp, cr, opConfig, string(csmv1.PowerStore), operatorutils.VersionSpec{})
			if success {
				assert.NoError(t, err)
				if newDeployment == nil {
					panic(err)
				}
				err = checkApplyContainersResiliency(dp.Spec.Template.Spec.Containers, cr)
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

	tests := map[string]func(t *testing.T) (bool, rbacv1.ClusterRole, operatorutils.OperatorConfig, csmv1.ContainerStorageModule){
		"success - greenfield injection": func(*testing.T) (bool, rbacv1.ClusterRole, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerStore, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			return true, controllerYAML.Rbac.ClusterRole, operatorConfig, customResource
		},
		"fail - bad config path": func(*testing.T) (bool, rbacv1.ClusterRole, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			tmpOperatorConfig := operatorConfig
			tmpOperatorConfig.ConfigDirectory = "bad/path"

			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerStore, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			return false, controllerYAML.Rbac.ClusterRole, tmpOperatorConfig, customResource
		},
		"failure - getResiliencyModule error": func(*testing.T) (bool, rbacv1.ClusterRole, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			return false, rbacv1.ClusterRole{}, operatorConfig, csmv1.ContainerStorageModule{}
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, clusterRole, opConfig, cr := tc(t)
			newClusterRole, err := ResiliencyInjectClusterRole(ctx, clusterRole, cr, opConfig, "controller")
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

func TestResiliencyInjectRole(t *testing.T) {
	ctx := context.Background()
	c := &operatorutils.MockClient{}
	tests := map[string]func(t *testing.T) (bool, rbacv1.Role, operatorutils.OperatorConfig, csmv1.ContainerStorageModule, string, ctrlClient.Client){
		"success - greenfield injection": func(*testing.T) (bool, rbacv1.Role, operatorutils.OperatorConfig, csmv1.ContainerStorageModule, string, ctrlClient.Client) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			nodeYAML, err := drivers.GetNode(ctx, customResource, operatorConfig, csmv1.PowerStore, "node.yaml", c, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			return true, nodeYAML.Rbac.Role, operatorConfig, customResource, "node.yaml", c
		},
		"failure - getResiliencyModule error": func(*testing.T) (bool, rbacv1.Role, operatorutils.OperatorConfig, csmv1.ContainerStorageModule, string, ctrlClient.Client) {
			return false, rbacv1.Role{}, operatorConfig, csmv1.ContainerStorageModule{}, "node.yaml", c
		},
		"success - mode is controller": func(*testing.T) (bool, rbacv1.Role, operatorutils.OperatorConfig, csmv1.ContainerStorageModule, string, ctrlClient.Client) {
			return true, rbacv1.Role{}, operatorConfig, csmv1.ContainerStorageModule{}, "node.yaml", c
		},
		"failure - GetModuleDefaultVersion error": func(*testing.T) (bool, rbacv1.Role, operatorutils.OperatorConfig, csmv1.ContainerStorageModule, string, ctrlClient.Client) {
			customResource, _ := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			for i := range customResource.Spec.Modules {
				if customResource.Spec.Modules[i].Name == csmv1.Resiliency {
					customResource.Spec.Modules[i].ConfigVersion = ""
				}
			}
			// Set a driver version that likely doesn't have a default resiliency mapping
			customResource.Spec.Driver.ConfigVersion = "9.9.9-invalid"
			return false, rbacv1.Role{}, operatorConfig, customResource, "node", c
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, role, opConfig, cr, _, _ := tc(t)
			newRole, err := ResiliencyInjectRole(ctx, role, cr, opConfig, "node")
			if name == "success - mode is controller" {
				newRole, err = ResiliencyInjectRole(ctx, role, cr, opConfig, "controller")
			}

			if success {
				assert.NoError(t, err)
				assert.NotNil(t, newRole)
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
	tests := map[string]func(t *testing.T) (bool, applyv1.DaemonSetApplyConfiguration, operatorutils.OperatorConfig){
		"success - greenfield injection": func(*testing.T) (bool, applyv1.DaemonSetApplyConfiguration, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			nodeYAML, err := drivers.GetNode(ctx, customResource, operatorConfig, csmv1.PowerStore, "node.yaml", ctrlClientFake.NewClientBuilder().Build(), operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}

			return true, nodeYAML.DaemonSetApplyConfig, operatorConfig
		},
		"success - brownfield injection": func(*testing.T) (bool, applyv1.DaemonSetApplyConfiguration, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			nodeYAML, err := drivers.GetNode(ctx, customResource, operatorConfig, csmv1.PowerStore, "node.yaml", ctrlClientFake.NewClientBuilder().Build(), operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			newDaemonSet, err := ResiliencyInjectDaemonset(ctx, nodeYAML.DaemonSetApplyConfig, customResource, operatorConfig, string(csmv1.PowerStore), operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}

			return true, *newDaemonSet, operatorConfig
		},
		"fail - bad config path": func(*testing.T) (bool, applyv1.DaemonSetApplyConfiguration, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
			if err != nil {
				panic(err)
			}
			nodeYAML, err := drivers.GetNode(ctx, customResource, operatorConfig, csmv1.PowerStore, "node.yaml", ctrlClientFake.NewClientBuilder().Build(), operatorutils.VersionSpec{})
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
			newDaemonSet, err := ResiliencyInjectDaemonset(ctx, ds, customResource, opConfig, string(csmv1.PowerStore), operatorutils.VersionSpec{})
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

// checkApplyContainersResiliency - check container configuration for resiliency
func checkApplyContainersResiliency(containers []acorev1.ContainerApplyConfiguration, cr csmv1.ContainerStorageModule) error {
	resiliencyModule, err := getResiliencyModule(cr)
	if err != nil {
		return err
	}

	driverContainerName := "driver"
	ctx := context.Background()
	// fetch podmonAPIPort
	podmonAPIPort := getResiliencyEnv(resiliencyModule, cr.Spec.Driver.CSIDriverType)
	var container acorev1.ContainerApplyConfiguration
	// fetch podmonArrayConnectivityPollRate
	setResiliencyArgs(ctx, resiliencyModule, nodeMode, &container, operatorutils.VersionSpec{}, cr)
	podmonArrayConnectivityPollRate := getPollRateFromArgs(container.Args)

	for _, cnt := range containers {
		if *cnt.Name == operatorutils.ResiliencySideCarName {

			// check argument in resiliency sidecar(podmon)
			foundPodmonArrayConnectivityPollRate := false
			for _, arg := range cnt.Args {
				if fmt.Sprintf("--arrayConnectivityPollRate=%s", podmonArrayConnectivityPollRate) == arg {
					foundPodmonArrayConnectivityPollRate = true
				}
			}
			if !foundPodmonArrayConnectivityPollRate {
				return fmt.Errorf("missing the following argument %s", podmonArrayConnectivityPollRate)
			}

		} else if *cnt.Name == driverContainerName {
			// check envs in driver sidecar
			foundPodmonAPIPort := false
			foundPodmonArrayConnectivityPollRate := false
			for _, env := range cnt.Env {
				if *env.Name == XCSIPodmonAPIPort {
					foundPodmonAPIPort = true
					if *env.Value != podmonAPIPort {
						return fmt.Errorf("expected %s to have a value of: %s but got: %s", XCSIPodmonAPIPort, podmonAPIPort, *env.Value)
					}
				}
				if *env.Name == XCSIPodmonArrayConnectivityPollRate {
					foundPodmonArrayConnectivityPollRate = true
					if *env.Value != podmonArrayConnectivityPollRate {
						return fmt.Errorf("expected %s to have a value of: %s but got: %s", XCSIPodmonArrayConnectivityPollRate, podmonArrayConnectivityPollRate, *env.Value)
					}
				}
			}
			if !foundPodmonAPIPort {
				return fmt.Errorf("missing the following argument %s", podmonAPIPort)
			}
			if !foundPodmonArrayConnectivityPollRate {
				return fmt.Errorf("missing the following argument %s", podmonArrayConnectivityPollRate)
			}
		}
	}
	return nil
}

func TestResiliencyPrecheck_ClusterClientCreationError_NoOp(t *testing.T) {
	type fakeControllerRuntimeClientWrapper func(clusterConfigData []byte) (ctrlClient.Client, error)

	cr, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
	if err != nil {
		t.Fatal(err)
	}

	module := cr.Spec.Modules[0] // resiliency module
	sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

	// Simulate cluster client creation failure
	fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
		return nil, fmt.Errorf("synthetic cluster client creation failure")
	}

	// Save + restore wrappers
	oldNewControllerRuntimeClientWrapper := operatorutils.NewControllerRuntimeClientWrapper
	oldNewK8sClientWrapper := operatorutils.NewK8sClientWrapper
	defer func() {
		operatorutils.NewControllerRuntimeClientWrapper = oldNewControllerRuntimeClientWrapper
		operatorutils.NewK8sClientWrapper = oldNewK8sClientWrapper
	}()

	operatorutils.NewControllerRuntimeClientWrapper = fakeControllerRuntimeClient
	operatorutils.NewK8sClientWrapper = func(_ []byte) (*kubernetes.Clientset, error) { return nil, nil }

	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    sourceClient,
		K8sClient: fake.NewSimpleClientset(),
	}

	// NOTE: ResiliencyPrecheck logs and returns nil on wrapper errors in your implementation
	err = ResiliencyPrecheck(context.TODO(), operatorConfig, module, cr, &fakeReconcile)
	assert.NoError(t, err, "ResiliencyPrecheck is tolerant of wrapper errors")
}

func TestResiliencyPrecheck_K8sClientCreationError_NoOp(t *testing.T) {
	type fakeControllerRuntimeClientWrapper func(clusterConfigData []byte) (ctrlClient.Client, error)

	cr, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
	if err != nil {
		t.Fatal(err)
	}

	module := cr.Spec.Modules[0]
	sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

	// Cluster client creation succeeds
	fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
		return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
	}

	oldNewControllerRuntimeClientWrapper := operatorutils.NewControllerRuntimeClientWrapper
	oldNewK8sClientWrapper := operatorutils.NewK8sClientWrapper
	defer func() {
		operatorutils.NewControllerRuntimeClientWrapper = oldNewControllerRuntimeClientWrapper
		operatorutils.NewK8sClientWrapper = oldNewK8sClientWrapper
	}()

	operatorutils.NewControllerRuntimeClientWrapper = fakeControllerRuntimeClient
	// Simulate k8s client creation failure (implementation tolerates this)
	operatorutils.NewK8sClientWrapper = func(_ []byte) (*kubernetes.Clientset, error) {
		return nil, fmt.Errorf("synthetic k8s client creation failure")
	}

	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    sourceClient,
		K8sClient: fake.NewSimpleClientset(),
	}

	err = ResiliencyPrecheck(context.TODO(), operatorConfig, module, cr, &fakeReconcile)
	assert.NoError(t, err, "ResiliencyPrecheck is tolerant of k8s wrapper errors")
}

func TestResiliencyInjectDeployment_ArgsAndEnvConsistency(t *testing.T) {
	ctx := context.Background()

	cr, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
	if err != nil {
		t.Fatal(err)
	}

	ctrlYAML, err := drivers.GetController(ctx, cr, operatorConfig, csmv1.PowerStore, operatorutils.VersionSpec{})
	if err != nil {
		t.Fatal(err)
	}

	dp, err := ResiliencyInjectDeployment(ctx, ctrlYAML.Deployment, cr, operatorConfig, string(csmv1.PowerStore), operatorutils.VersionSpec{})
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, dp)

	// Use the existing helper to verify driver env + sidecar args match expected poll rate & API port
	if err := checkApplyContainersResiliency(dp.Spec.Template.Spec.Containers, cr); err != nil {
		t.Fatalf("resiliency container checks failed: %v", err)
	}

	// Explicit sidecar presence + safe arg prefix check (no slicing)
	const pollPrefix = "--arrayConnectivityPollRate="
	foundSidecar := false
	foundPollArg := false

	for _, c := range dp.Spec.Template.Spec.Containers {
		if c.Name != nil && *c.Name == operatorutils.ResiliencySideCarName {
			foundSidecar = true
			for _, a := range c.Args {
				if strings.HasPrefix(a, pollPrefix) {
					foundPollArg = true
					break
				}
			}
		}
	}

	assert.True(t, foundSidecar, "expected resiliency sidecar (podmon) to be injected")
	assert.True(t, foundPollArg, "expected argument with prefix --arrayConnectivityPollRate= in podmon sidecar")
}

func TestResiliencyInjectDaemonset_SidecarPresent(t *testing.T) {
	ctx := context.Background()
	cr, err := getCustomResource("./testdata/cr_powerstore_resiliency.yaml")
	if err != nil {
		t.Fatal(err)
	}

	nodeYAML, err := drivers.GetNode(ctx, cr, operatorConfig, csmv1.PowerStore, "node.yaml", ctrlClientFake.NewClientBuilder().Build(), operatorutils.VersionSpec{})
	if err != nil {
		t.Fatal(err)
	}

	ds, err := ResiliencyInjectDaemonset(ctx, nodeYAML.DaemonSetApplyConfig, cr, operatorConfig, string(csmv1.PowerStore), operatorutils.VersionSpec{})
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, ds)

	// Check the podmon sidecar exists
	foundSidecar := false
	for _, c := range ds.Spec.Template.Spec.Containers {
		if c.Name != nil && *c.Name == operatorutils.ResiliencySideCarName {
			foundSidecar = true
			break
		}
	}
	assert.True(t, foundSidecar, "expected resiliency sidecar to be present in daemonset")
}

// matched.Version is non-empty and matched.Images has an entry for the container name -> image is overridden from matched.
func TestModifyPodmon_UsesMatchedImageWhenVersionSet(t *testing.T) {
	name := "podmon"
	originalImage := "registry.example/podmon:old"
	matchedImage := "registry.example/podmon:from-matched"
	ctx := context.Background()

	matched := operatorutils.VersionSpec{
		Version: "v9.9.9", // non-empty to trigger the branch
		Images:  map[string]string{name: matchedImage},
	}

	// Component does NOT set Image/PullPolicy -> matched should be applied
	component := csmv1.ContainerTemplate{
		Image:           csmv1.ImageType(""),
		ImagePullPolicy: corev1.PullPolicy(""), // keep empty to avoid override
		Envs:            nil,
		Args:            nil,
	}

	// Build container apply configuration
	container := acorev1.Container().
		WithName(name).
		WithImage(originalImage)

	// Act
	modifyPodmon(ctx, component, container, matched, csmv1.ContainerStorageModule{})

	// Assert
	if container.Image == nil {
		t.Fatalf("container.Image should not be nil after modifyPodmon")
	}
	if *container.Image != matchedImage {
		t.Fatalf("expected image %q from matched.Images, got %q", matchedImage, *container.Image)
	}
}

// Case 2: matched.Version is empty -> skip matched override; image remains original (since component.Image is not set)
func TestModifyPodmon_SkipsMatchedWhenVersionEmpty(t *testing.T) {
	name := "podmon"
	originalImage := "registry.example/podmon:unchanged"
	matchedImage := "registry.example/podmon:should-NOT-apply"
	ctx := context.Background()

	matched := operatorutils.VersionSpec{
		Version: "", // empty -> branch skipped
		Images:  map[string]string{name: matchedImage},
	}

	component := csmv1.ContainerTemplate{
		Image:           csmv1.ImageType(""),
		ImagePullPolicy: corev1.PullPolicy(""),
		Envs:            nil,
		Args:            nil,
	}

	container := acorev1.Container().
		WithName(name).
		WithImage(originalImage)

	modifyPodmon(ctx, component, container, matched, csmv1.ContainerStorageModule{})

	if container.Image == nil {
		t.Fatalf("container.Image should not be nil after modifyPodmon")
	}
	if *container.Image != originalImage {
		t.Fatalf("expected image to remain %q when matched.Version is empty, got %q", originalImage, *container.Image)
	}
}

// Case 3: component overrides should take precedence over matched.Images (for both Image and ImagePullPolicy)
func TestModifyPodmon_ComponentOverridesImageAndPullPolicy(t *testing.T) {
	name := "podmon"
	originalImage := "registry.example/podmon:old"
	matchedImage := "registry.example/podmon:from-matched"
	componentImage := "registry.example/podmon:cr-override"
	ctx := context.Background()

	matched := operatorutils.VersionSpec{
		Version: "v1.16.0", // any non-empty to ensure branch executes
		Images:  map[string]string{name: matchedImage},
	}

	component := csmv1.ContainerTemplate{
		Image:           csmv1.ImageType(componentImage), // cast to csmv1.ImageType
		ImagePullPolicy: corev1.PullAlways,               // component policy override
		Envs:            nil,
		Args:            nil,
	}

	container := acorev1.Container().
		WithName(name).
		WithImage(originalImage).
		WithImagePullPolicy(corev1.PullIfNotPresent) // initial policy

	modifyPodmon(ctx, component, container, matched, csmv1.ContainerStorageModule{})

	// Image should be component override (not matched)
	if container.Image == nil {
		t.Fatalf("container.Image should not be nil after modifyPodmon")
	}
	if *container.Image != matchedImage {
		t.Fatalf("expected image %q from component override, got %q", matchedImage, *container.Image)
	}

	// Pull policy should be component override
	if container.ImagePullPolicy == nil {
		t.Fatalf("container.ImagePullPolicy should not be nil after modifyPodmon")
	}
	if string(*container.ImagePullPolicy) != string(corev1.PullAlways) {
		t.Fatalf("expected pull policy %q from component override, got %q", corev1.PullAlways, *container.ImagePullPolicy)
	}
}

func TestModifyPodmon_ReplacesEnvAndArgs(t *testing.T) {
	name := "podmon"
	matched := operatorutils.VersionSpec{} // not relevant for env/args
	ctx := context.Background()
	component := csmv1.ContainerTemplate{
		Envs: []corev1.EnvVar{
			{Name: "FOO", Value: "bar"},
		},
		Args: []string{"--enable", "--level=debug"},
	}

	// Build container WITHOUT an initial arg
	container := acorev1.Container().
		WithName(name).
		WithEnv(acorev1.EnvVar().WithName("FOO").WithValue("old"))
		// NOTE: intentionally NOT calling WithArgs("--old")

	// Act
	modifyPodmon(ctx, component, container, matched, csmv1.ContainerStorageModule{})

	// Assert env replacement
	found := false
	for _, e := range container.Env {
		if e.Name != nil && *e.Name == "FOO" && e.Value != nil && *e.Value == "bar" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected env FOO=bar to be present after replacement")
	}

	// Assert args replacement (now exact match)
	if len(container.Args) != 2 || container.Args[0] != "--enable" || container.Args[1] != "--level=debug" {
		t.Fatalf("expected args [--enable --level=debug], got %v", container.Args)
	}
}

func TestSetResiliencyArgs_SyntheticControllerMode_OverridesImageFromMatched(t *testing.T) {
	ctx := context.Background()

	// Minimal resiliency module with NO components → triggers synthetic branch.
	resiliency := csmv1.Module{
		Name:       csmv1.Resiliency,
		Enabled:    true,
		Components: nil, // key point: empty
	}

	// Construct a container that represents podmon in controller mode.
	// The override uses matched.Images[*container.Name], so ensure Name matches the key we set in matched.Images below.
	name := "podmon"
	image := "registry.example/podmon:template"
	container := &acorev1.ContainerApplyConfiguration{
		Name:  &name,
		Image: &image,
		// You can set Args/Env/PullPolicy if you want to extend coverage for modifyPodmon later.
	}

	// Matched spec with non-empty version and an image for container.Name → triggers modifyPodmon's matched path.
	matched := operatorutils.VersionSpec{
		Version: "v1.16.0",
		Images: map[string]string{
			name: "registry.example/podmon:override-controller",
		},
	}

	// Minimal CR; only needed for modifyPodmon fallbacks (not used since matched.Version != "")
	cr := csmv1.ContainerStorageModule{
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: csmv1.Driver{CSIDriverType: csmv1.PowerScaleName}, // driver type not used in modifyPodmon's matched path
		},
	}

	// Act: controller mode synthetic branch.
	setResiliencyArgs(ctx, resiliency, controllerMode, container, matched, cr)

	// Assert: container.Image must be overridden by matched.Images[*container.Name].
	if container.Image == nil {
		t.Fatalf("container.Image should not be nil after setResiliencyArgs")
	}
	got := *container.Image
	want := "registry.example/podmon:override-controller"
	if got != want {
		t.Errorf("unexpected image after synthetic controller override: got=%s want=%s", got, want)
	}
}

func TestSetResiliencyArgs_SyntheticNodeMode_OverridesImageFromMatched(t *testing.T) {
	ctx := context.Background()

	// Minimal resiliency module with NO components → triggers synthetic branch.
	resiliency := csmv1.Module{
		Name:       csmv1.Resiliency,
		Enabled:    true,
		Components: nil, // key point: empty
	}

	// Construct a container that represents podmon in node mode.
	// The override uses matched.Images[*container.Name], so ensure Name matches the key we set in matched.Images below.
	name := "podmon-node"
	image := "registry.example/podmon-node:template"
	container := &acorev1.ContainerApplyConfiguration{
		Name:  &name,
		Image: &image,
	}

	// Matched spec with non-empty version and an image for container.Name → triggers modifyPodmon's matched path.
	matched := operatorutils.VersionSpec{
		Version: "v1.16.0",
		Images: map[string]string{
			name: "registry.example/podmon-node:override",
		},
	}

	cr := csmv1.ContainerStorageModule{} // not used by matched path

	// Act: node mode synthetic branch.
	setResiliencyArgs(ctx, resiliency, nodeMode, container, matched, cr)

	// Assert: container.Image must be overridden by matched.Images[*container.Name].
	if container.Image == nil {
		t.Fatalf("container.Image should not be nil after setResiliencyArgs")
	}
	got := *container.Image
	want := "registry.example/podmon-node:override"
	if got != want {
		t.Errorf("unexpected image after synthetic node override: got=%s want=%s", got, want)
	}
}

func TestSetResiliencyArgs_SyntheticUnsupportedMode_NoChange(t *testing.T) {
	ctx := context.Background()

	// Minimal resiliency module with NO components → triggers synthetic branch logic, but the mode is unsupported → default: return (no-op).
	resiliency := csmv1.Module{
		Name:       csmv1.Resiliency,
		Enabled:    true,
		Components: nil,
	}

	name := "podmon"
	image := "registry.example/podmon:template"
	container := &acorev1.ContainerApplyConfiguration{
		Name:  &name,
		Image: &image,
	}

	// Even if matched is provided, since mode is unsupported the function will early-return without modifying the container.
	matched := operatorutils.VersionSpec{
		Version: "v1.16.0",
		Images: map[string]string{
			name: "registry.example/podmon:override-should-not-apply",
		},
	}

	cr := csmv1.ContainerStorageModule{}

	// Act: unsupported mode
	setResiliencyArgs(ctx, resiliency, "unsupported-mode", container, matched, cr)

	// Assert: no change expected
	if container.Image == nil {
		t.Fatalf("container.Image should not be nil")
	}
	got := *container.Image
	if got != image {
		t.Errorf("image unexpectedly changed for unsupported mode: got=%s want=%s", got, image)
	}
}
