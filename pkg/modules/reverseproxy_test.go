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

package modules

import (
	"context"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
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
			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
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
			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
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

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
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

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
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

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, reverseProxy, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"Fail - no components": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
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

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, reverseProxy, tmpCR, sourceClient, fakeControllerRuntimeClient
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

			success, reverseProxy, tmpCR, sourceClient, fakeControllerRuntimeClient := tc(t)
			utils.NewControllerRuntimeClientWrapper = fakeControllerRuntimeClient
			utils.NewK8sClientWrapper = func(clusterConfigData []byte) (*kubernetes.Clientset, error) {
				return nil, nil
			}

			fakeReconcile := utils.FakeReconcileCSM{
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
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cm).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			tmpCR, err := getCustomResource("./testdata/cr_powermax_reverseproxy.yaml")
			if err != nil {
				panic(err)
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - authorization module not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			tmpCR, err := getCustomResource("./testdata/cr_powermax_replica.yaml")
			if err != nil {
				panic(err)
			}
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
