/*
 *
 * Copyright © 2021-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

//  Copyright © 2023-2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package modules

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/dell/csm-operator/pkg/drivers"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReverseProxyPrecheck(t *testing.T) {
	type fakeControllerRuntimeClientWrapper func(clusterConfigData []byte) (ctrlClient.Client, error)

	tests := map[string]func(t *testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper){
		"success -  driver type Powermax": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}

			proxySecret := getSecret(customResource.Namespace, "csirevproxy-tls-secret")
			proxyConfigMap := getConfigMap(customResource.Namespace, "powermax-reverseproxy-config")

			tmpCR := customResource
			reverseProxy := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(proxySecret, proxyConfigMap).Build()
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(proxySecret, proxyConfigMap).Build()
				return clusterClient, nil
			}

			return true, reverseProxy, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"success - auth injected": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}

			proxySecret := getSecret(customResource.Namespace, "csirevproxy-tls-secret")
			proxyConfigMap := getConfigMap(customResource.Namespace, "powermax-reverseproxy-config")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "powermax"
			reverseProxy := tmpCR.Spec.Modules[0]
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(proxySecret, proxyConfigMap, karaviAuthconfig, proxyAuthzTokens).Build()
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(proxySecret, proxyConfigMap).Build()
				return clusterClient, nil
			}

			return true, reverseProxy, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"Fail - unsupported reverseProxy version": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}

			proxySecret := getSecret(customResource.Namespace, "csirevproxy-tls-secret")
			proxyConfigMap := getConfigMap(customResource.Namespace, "powermax-reverseproxy-config")

			tmpCR := customResource
			reverseProxy := tmpCR.Spec.Modules[0]
			reverseProxy.ConfigVersion = "v100000.0.0"

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(proxySecret, proxyConfigMap).Build()
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects(proxySecret, proxyConfigMap).Build(), nil
			}

			return false, reverseProxy, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"Fail - unsupported driver": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}

			proxySecret := getSecret(customResource.Namespace, "csirevproxy-tls-secret")
			proxyConfigMap := getConfigMap(customResource.Namespace, "powermax-reverseproxy-config")

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "unsupported-driver"
			reverseProxy := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(proxySecret, proxyConfigMap).Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, reverseProxy, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"Fail - no secret": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}

			proxyConfigMap := getConfigMap(customResource.Namespace, "powermax-reverseproxy-config")

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "powermax"
			reverseProxy := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(proxyConfigMap).Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, reverseProxy, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"Fail - no configmap": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}

			proxySecret := getSecret(customResource.Namespace, "csirevproxy-tls-secret")

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "powermax"
			reverseProxy := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(proxySecret).Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, reverseProxy, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"success - no components": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}

			proxySecret := getSecret(customResource.Namespace, "csirevproxy-tls-secret")
			proxyConfigMap := getConfigMap(customResource.Namespace, "powermax-reverseproxy-config")

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "powermax"
			reverseProxy := tmpCR.Spec.Modules[0]
			reverseProxy.Components = nil
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(proxySecret, proxyConfigMap).Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return true, reverseProxy, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"success - use reverse proxy secret": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy_use_secret.yaml")
			if err != nil {
				panic(err)
			}

			customResource.Spec.Driver.Common.Envs = append(customResource.Spec.Driver.Common.Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "true"})
			customResource.Spec.Modules[0].Components[0].Envs = append(customResource.Spec.Modules[0].Components[0].Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "true"})

			proxySecret := getSecret(customResource.Namespace, "csirevproxy-tls-secret")

			tmpCR := customResource
			reverseProxy := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(proxySecret).Build()
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(proxySecret).Build()
				return clusterClient, nil
			}

			return true, reverseProxy, tmpCR, sourceClient, fakeControllerRuntimeClient
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

			success, reverseProxy, tmpCR, sourceClient, fakeControllerRuntimeClient := tc(t)
			operatorutils.NewControllerRuntimeClientWrapper = fakeControllerRuntimeClient
			operatorutils.NewK8sClientWrapper = func(_ []byte) (*kubernetes.Clientset, error) {
				return nil, nil
			}

			fakeReconcile := operatorutils.FakeReconcileCSM{
				Client:    sourceClient,
				K8sClient: fake.NewSimpleClientset(),
			}

			err := ReverseProxyPrecheck(ctx, operatorConfig, reverseProxy, tmpCR, &fakeReconcile)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestReverseProxyServer(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}
			cm := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind: "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "csm-config-params",
				},
			}
			deployAsSidecar = false
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cm).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}
			deployAsSidecar = false
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - creating as Sidecar": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}
			deployAsSidecar = true
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - creating with minimal manifest": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}
			tmpCR.Spec.Modules[0].Components = nil
			deployAsSidecar = false
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - reverseproxy module not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR, err := getCustomResource("./testdata/cr_powermax_replica.yaml")
			if err != nil {
				panic(err)
			}
			deployAsSidecar = false
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return false, false, tmpCR, sourceClient, operatorConfig
		},
		"success - use reverse proxy secret": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR, err := getCustomResource("./testdata/cr_powermax_reverseproxy_use_secret.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR.Spec.Driver.Common.Envs = append(tmpCR.Spec.Driver.Common.Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "true"})
			tmpCR.Spec.Modules[0].Components[0].Envs = append(tmpCR.Spec.Modules[0].Components[0].Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "true"})

			deployAsSidecar = true
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - dynamically mount configMap": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR, err := getCustomResource("./testdata/cr_powermax_reverseproxy_use_secret.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR.Spec.Driver.Common.Envs = append(tmpCR.Spec.Driver.Common.Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "false"})

			tmpCR.Spec.Modules[0].Components[0].Envs = append(tmpCR.Spec.Modules[0].Components[0].Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "false"})

			deployAsSidecar = true
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - invalid csm version": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR, err := getCustomResource("./testdata/cr_powermax_reverseproxy_use_secret.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR.Spec.Driver.Common.Envs = append(tmpCR.Spec.Driver.Common.Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "false"})

			tmpCR.Spec.Modules[0].Components[0].Envs = append(tmpCR.Spec.Modules[0].Components[0].Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "false"})

			// Set config version to something invalid.
			tmpCR.Spec.Driver.ConfigVersion = "invalid"

			deployAsSidecar = true
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return false, false, tmpCR, sourceClient, operatorConfig
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op := tc(t)
			err := ReverseProxyServer(context.TODO(), isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestReverseProxyInjectDeployment(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule){
		"success - no deployAsSidecar": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerMax)
			if err != nil {
				panic(err)
			}
			return true, controllerYAML.Deployment, operatorConfig, customResource
		},
		"success - deployAsSidecar": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerMax)
			if err != nil {
				panic(err)
			}
			deployAsSidecar = true
			return true, controllerYAML.Deployment, operatorConfig, customResource
		},
		"success - dynamically mount secret": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy_use_secret.yaml")
			if err != nil {
				panic(err)
			}

			customResource.Spec.Driver.Common.Envs = append(customResource.Spec.Driver.Common.Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "true"})
			customResource.Spec.Modules[0].Components[0].Envs = append(customResource.Spec.Modules[0].Components[0].Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "true"})

			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerMax)
			if err != nil {
				panic(err)
			}
			deployAsSidecar = true

			return true, controllerYAML.Deployment, operatorConfig, customResource
		},
		"success - dynamically mount configMap": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy_use_secret.yaml")
			if err != nil {
				panic(err)
			}

			customResource.Spec.Driver.Common.Envs = append(customResource.Spec.Driver.Common.Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "false"})
			customResource.Spec.Modules[0].Components[0].Envs = append(customResource.Spec.Modules[0].Components[0].Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "false"})

			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerMax)
			if err != nil {
				panic(err)
			}
			deployAsSidecar = true

			return true, controllerYAML.Deployment, operatorConfig, customResource
		},
		"fail - invalid csm version": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy_use_secret.yaml")
			if err != nil {
				panic(err)
			}

			customResource.Spec.Driver.Common.Envs = append(customResource.Spec.Driver.Common.Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "false"})
			customResource.Spec.Modules[0].Components[0].Envs = append(customResource.Spec.Modules[0].Components[0].Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "false"})

			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerMax)
			if err != nil {
				panic(err)
			}

			customResource.Spec.Driver.ConfigVersion = "invalid"
			deployAsSidecar = true

			return false, controllerYAML.Deployment, operatorConfig, customResource
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, dp, op, cr := tc(t)
			_, err := ReverseProxyInjectDeployment(dp, cr, op)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestReverseProxyStartService(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - no service": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}
			deployAsSidecar = false
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - creates service": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}
			deployAsSidecar = true
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - deletes service": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}
			deployAsSidecar = true
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, true, tmpCR, sourceClient, operatorConfig
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, client, op := tc(t)
			err := ReverseProxyStartService(ctx, isDeleting, op, cr, client)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestAddReverseProxyServiceName(t *testing.T) {
	tests := map[string]func(t *testing.T) applyv1.DeploymentApplyConfiguration{
		"Add env var to driver container": func(*testing.T) applyv1.DeploymentApplyConfiguration {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerMax)
			if err != nil {
				panic(err)
			}
			deployAsSidecar = true
			return controllerYAML.Deployment
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dp := tc(t)
			AddReverseProxyServiceName(&dp)
			isEnvFound := false
			for i, cnt := range dp.Spec.Template.Spec.Containers {
				if *cnt.Name == "driver" {
					for _, env := range dp.Spec.Template.Spec.Containers[i].Env {
						if strings.EqualFold(*env.Name, CSIPmaxRevProxyServiceName) && strings.EqualFold(*env.Value, RevProxyServiceName) {
							isEnvFound = true
						}
					}
				}
			}
			if !isEnvFound {
				t.Errorf("Expected env vars: %v with value %v, but not found", CSIPmaxRevProxyServiceName, RevProxyServiceName)
			}
		})
	}
}

func TestIsReverseProxySidecar(t *testing.T) {
	type fakeControllerRuntimeClientWrapper func(clusterConfigData []byte) (ctrlClient.Client, error)

	tests := map[string]func(t *testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper){
		"Reverse proxy is configured as sidecar": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy_sidecar.yaml")
			if err != nil {
				panic(err)
			}

			proxySecret := getSecret(customResource.Namespace, "csirevproxy-tls-secret")
			proxyConfigMap := getConfigMap(customResource.Namespace, "powermax-reverseproxy-config")

			tmpCR := customResource
			reverseProxy := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(proxySecret, proxyConfigMap).Build()
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(proxySecret, proxyConfigMap).Build()
				return clusterClient, nil
			}

			return true, reverseProxy, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"Reverse proxy is not configured as sidecar": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}

			proxySecret := getSecret(customResource.Namespace, "csirevproxy-tls-secret")
			proxyConfigMap := getConfigMap(customResource.Namespace, "powermax-reverseproxy-config")

			tmpCR := customResource
			reverseProxy := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(proxySecret, proxyConfigMap).Build()
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(proxySecret, proxyConfigMap).Build()
				return clusterClient, nil
			}

			return false, reverseProxy, tmpCR, sourceClient, fakeControllerRuntimeClient
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

			isSideCar, reverseProxy, tmpCR, sourceClient, fakeControllerRuntimeClient := tc(t)
			operatorutils.NewControllerRuntimeClientWrapper = fakeControllerRuntimeClient
			operatorutils.NewK8sClientWrapper = func(_ []byte) (*kubernetes.Clientset, error) {
				return nil, nil
			}

			fakeReconcile := operatorutils.FakeReconcileCSM{
				Client:    sourceClient,
				K8sClient: fake.NewSimpleClientset(),
			}

			ReverseProxyPrecheck(ctx, operatorConfig, reverseProxy, tmpCR, &fakeReconcile)
			if isSideCar != IsReverseProxySidecar() {
				t.Errorf("Expected %v but got %v", isSideCar, IsReverseProxySidecar())
			}
		})
	}
}

func TestUpdatePowerMaxConfigMap(t *testing.T) {
	tests := map[string]func(t *testing.T) (*corev1.ConfigMap, csmv1.ContainerStorageModule, string){
		"success: add port to config params": func(*testing.T) (*corev1.ConfigMap, csmv1.ContainerStorageModule, string) {
			port := "2121"
			configData := ""
			cm := &corev1.ConfigMap{
				Data: map[string]string{
					drivers.ConfigParamsFile: configData,
				},
			}

			cr := csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{
									Name:  "X_CSI_REVPROXY_USE_SECRET",
									Value: "true",
								},
							},
						},
					},
					Modules: []csmv1.Module{
						{
							Name: csmv1.ReverseProxy,
							Components: []csmv1.ContainerTemplate{
								{
									Name: ReverseProxyServerComponent,
									Envs: []corev1.EnvVar{
										{
											Name:  "X_CSI_REVPROXY_PORT",
											Value: port,
										},
									},
								},
							},
						},
					},
				},
			}

			configData += fmt.Sprintf("\n%s: %s", "CSI_POWERMAX_REVERSE_PROXY_PORT", port)

			return cm, cr, configData
		},
		"success: not using secret": func(*testing.T) (*corev1.ConfigMap, csmv1.ContainerStorageModule, string) {
			// Arrange
			configData := ""
			cm := &corev1.ConfigMap{
				Data: map[string]string{
					drivers.ConfigParamsFile: configData,
				},
			}

			cr := csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{
									Name:  "X_CSI_REVPROXY_USE_SECRET",
									Value: "false",
								},
							},
						},
					},
				},
			}

			return cm, cr, configData
		},
		"success: minimal manifest with secret": func(*testing.T) (*corev1.ConfigMap, csmv1.ContainerStorageModule, string) {
			// Arrange
			configData := ""
			cm := &corev1.ConfigMap{
				Data: map[string]string{
					drivers.ConfigParamsFile: configData,
				},
			}

			cr := csmv1.ContainerStorageModule{
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: &csmv1.ContainerTemplate{
							Envs: []corev1.EnvVar{
								{
									Name:  "X_CSI_REVPROXY_USE_SECRET",
									Value: "true",
								},
							},
						},
					},
				},
			}

			// Due to minimal manifest, the default will be set.
			configData += fmt.Sprintf("\n%s: %s", "CSI_POWERMAX_REVERSE_PROXY_PORT", "2222")

			return cm, cr, configData
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cm, cr, expectedResponse := tc(t)

			UpdatePowerMaxConfigMap(cm, cr)
			assert.Equal(t, cm.Data[drivers.ConfigParamsFile], expectedResponse)
		})
	}
}

func TestResetDeployAsSidecar(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{
			name: "Test reset deploy as sidecar",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetDeployAsSidecar()
			if got := deployAsSidecar; got != tt.want {
				t.Errorf("ResetDeployAsSidecar() = %v, want %v", got, tt.want)
			}
		})
	}
}
