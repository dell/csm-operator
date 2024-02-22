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
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	csmv1 "github.com/dell/csm-operator/api/v1"
	utils "github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/tests/shared"
	"github.com/dell/csm-operator/tests/shared/clientgoclient"
	"github.com/dell/csm-operator/tests/shared/crclient"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var ctx = context.Background()

func TestObservabilityPrecheck(t *testing.T) {
	type fakeControllerRuntimeClientWrapper func(clusterConfigData []byte) (ctrlClient.Client, error)

	tests := map[string]func(t *testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper){
		"success": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")

			tmpCR := customResource
			observability := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
				return clusterClient, nil
			}

			return true, observability, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"success - driver type PowerScale": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "powerscale"
			observability := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
				return clusterClient, nil
			}

			return true, observability, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"success - driver type PowerFlex": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}

			vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "powerflex"
			observability := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(vxflexosCreds).Build()
			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(vxflexosCreds).Build()
				return clusterClient, nil
			}

			return true, observability, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"success - driver type Powermax": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
			if err != nil {
				panic(err)
			}

			pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "powermax"
			observability := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds).Build()
			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds).Build()
				return clusterClient, nil
			}

			return true, observability, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"success - version provided": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")

			tmpCR := customResource
			observability := tmpCR.Spec.Modules[0]
			observability.ConfigVersion = "v1.6.0"

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
				return clusterClient, nil
			}

			return true, observability, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"success - auth injected": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "powerscale"
			observability := tmpCR.Spec.Modules[0]
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds, karaviAuthconfig, proxyAuthzTokens).Build()
			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
				return clusterClient, nil
			}

			return true, observability, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"Fail - unsupported observability version": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")

			tmpCR := customResource
			observability := tmpCR.Spec.Modules[0]
			observability.ConfigVersion = "v100000.0.0"

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build(), nil
			}

			return false, observability, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"Fail - unsupported driver": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "unsupported-driver"
			observability := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, observability, tmpCR, sourceClient, fakeControllerRuntimeClient
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

			success, observability, tmpCR, sourceClient, fakeControllerRuntimeClient := tc(t)
			utils.NewControllerRuntimeClientWrapper = fakeControllerRuntimeClient
			utils.NewK8sClientWrapper = func(clusterConfigData []byte) (*kubernetes.Clientset, error) {
				return nil, nil
			}

			fakeReconcile := utils.FakeReconcileCSM{
				Client:    sourceClient,
				K8sClient: fake.NewSimpleClientset(),
			}

			err := ObservabilityPrecheck(ctx, operatorConfig, observability, tmpCR, &fakeReconcile)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestObservabilityTopologyController(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			cr := &rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					Kind: "ClusterRole",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "karavi-observability-topology-controller",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},

		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},

		"Fail - observability module not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op := tc(t)

			err := ObservabilityTopology(ctx, isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestPowerScaleMetrics(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}
			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")

			tmpCR := customResource

			cr := &rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					Kind: "ClusterRole",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "karavi-metrics-powerscale-controller",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr, isilonCreds).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - deleting with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			cr := &rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					Kind: "ClusterRole",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "karavi-metrics-powerscale-controller",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr, isilonCreds, karaviAuthconfig, proxyAuthzTokens).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - deleting with auth after one cycle": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}
			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds, karaviAuthconfig, proxyAuthzTokens).Build()
			k8sClient := clientgoclient.NewFakeClient(sourceClient)

			// pre-run to generate objects
			err = PowerScaleMetrics(ctx, false, operatorConfig, tmpCR, sourceClient, k8sClient)
			if err != nil {
				panic(err)
			}

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}
			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - creating with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}
			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds, karaviAuthconfig, proxyAuthzTokens).Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - update objects": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			objects := map[shared.StorageKey]runtime.Object{}
			fakeClient := crclient.NewFakeClientNoInjector(objects)
			fakeClient.Create(ctx, isilonCreds)
			fakeClient.Create(ctx, karaviAuthconfig)
			fakeClient.Create(ctx, proxyAuthzTokens)
			k8sClient := clientgoclient.NewFakeClient(fakeClient)
			// pre-run to generate objects
			err = PowerScaleMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			return true, false, tmpCR, fakeClient, operatorConfig
		},
		"success - copy secrets when secrets already existed": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}
			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
			isilonKaraviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			isilonProxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")
			karaviIsilonCreds := getSecret("karavi", "isilon-creds")
			karaviAuthconfig := getSecret("karavi", "isilon-karavi-authorization-config")
			proxyAuthzTokens := getSecret("karavi", "isilon-proxy-authz-tokens")
			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds, isilonKaraviAuthconfig, isilonProxyAuthzTokens, karaviIsilonCreds, karaviAuthconfig, proxyAuthzTokens).Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"Fail - no secrets in isilon namespace": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},
		"Fail - wrong module name": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},
		"Fail - skipCertificateValidation is false but no cert": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true
			// set skipCertificateValidation to false
			for i, env := range auth.Components[0].Envs {
				if env.Name == "SKIP_CERTIFICATE_VALIDATION" {
					auth.Components[0].Envs[i].Value = "false"
				}
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds, karaviAuthconfig, proxyAuthzTokens).Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op := tc(t)
			k8sClient := clientgoclient.NewFakeClient(sourceClient)
			err := PowerScaleMetrics(ctx, isDeleting, op, cr, sourceClient, k8sClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestOtelCollector(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			cr := &rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					Kind: "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "otel-collector-config",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},

		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"Fail - wrong module name": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op := tc(t)

			err := OtelCollector(ctx, isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestPowerFlexMetrics(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}
			vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")

			tmpCR := customResource

			cr := &rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					Kind: "ClusterRole",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "karavi-metrics-powerflex-controller",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr, vxflexosCreds).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - deleting with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}

			vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			cr := &rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					Kind: "ClusterRole",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "karavi-metrics-powerscale-controller",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr, vxflexosCreds, karaviAuthconfig, proxyAuthzTokens).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - deleting with auth after one cycle": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}
			vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(vxflexosCreds, karaviAuthconfig, proxyAuthzTokens).Build()
			k8sClient := clientgoclient.NewFakeClient(sourceClient)

			// pre-run to generate objects
			err = PowerFlexMetrics(ctx, false, operatorConfig, tmpCR, sourceClient, k8sClient)
			if err != nil {
				panic(err)
			}

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}
			vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(vxflexosCreds).Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - creating with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}
			vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(vxflexosCreds, karaviAuthconfig, proxyAuthzTokens).Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - update objects": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}

			vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			objects := map[shared.StorageKey]runtime.Object{}
			fakeClient := crclient.NewFakeClientNoInjector(objects)
			fakeClient.Create(ctx, vxflexosCreds)
			fakeClient.Create(ctx, karaviAuthconfig)
			fakeClient.Create(ctx, proxyAuthzTokens)
			k8sClient := clientgoclient.NewFakeClient(fakeClient)
			// pre-run to generate objects
			err = PowerFlexMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			return true, false, tmpCR, fakeClient, operatorConfig
		},
		"success - copy secrets when secrets already existed": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}
			vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")
			vxflexosAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			vxflexosProxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")
			karaviVxflexosCreds := getSecret("karavi", "test-vxflexos-config")
			karaviAuthconfig := getSecret("karavi", "powerflex-karavi-authorization-config")
			proxyAuthzTokens := getSecret("karavi", "powerflex-proxy-authz-tokens")
			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(vxflexosCreds, karaviAuthconfig, proxyAuthzTokens, karaviVxflexosCreds, vxflexosAuthconfig, vxflexosProxyAuthzTokens).Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"Fail - wrong module name": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},
		"Fail - no secrets in test-vxflexos namespace": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op := tc(t)
			k8sClient := clientgoclient.NewFakeClient(sourceClient)
			err := PowerFlexMetrics(ctx, isDeleting, op, cr, sourceClient, k8sClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestPowerMaxMetrics(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
			if err != nil {
				panic(err)
			}
			pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")

			tmpCR := customResource

			cr := &rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					Kind: "ClusterRole",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "karavi-metrics-powermax-controller",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr, pmaxCreds).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - deleting with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
			if err != nil {
				panic(err)
			}

			pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			cr := &rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					Kind: "ClusterRole",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "karavi-metrics-powermax-controller",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr, pmaxCreds, karaviAuthconfig, proxyAuthzTokens).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - deleting with auth after one cycle": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
			if err != nil {
				panic(err)
			}
			pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds, karaviAuthconfig, proxyAuthzTokens).Build()
			k8sClient := clientgoclient.NewFakeClient(sourceClient)

			// pre-run to generate objects
			err = PowerMaxMetrics(ctx, false, operatorConfig, tmpCR, sourceClient, k8sClient)
			if err != nil {
				panic(err)
			}

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
			if err != nil {
				panic(err)
			}
			pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds).Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - creating with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
			if err != nil {
				panic(err)
			}
			pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds, karaviAuthconfig, proxyAuthzTokens).Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - update objects": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
			if err != nil {
				panic(err)
			}

			pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			objects := map[shared.StorageKey]runtime.Object{}
			fakeClient := crclient.NewFakeClientNoInjector(objects)
			fakeClient.Create(ctx, pmaxCreds)
			fakeClient.Create(ctx, karaviAuthconfig)
			fakeClient.Create(ctx, proxyAuthzTokens)
			k8sClient := clientgoclient.NewFakeClient(fakeClient)
			// pre-run to generate objects
			err = PowerMaxMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			return true, false, tmpCR, fakeClient, operatorConfig
		},
		"Fail - no secrets in test-powermax namespace": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},
		"Fail - wrong module name": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},
		"Fail - skipCertificateValidation is false but no cert": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true
			// set skipCertificateValidation to false
			for i, env := range auth.Components[0].Envs {
				if env.Name == "SKIP_CERTIFICATE_VALIDATION" {
					auth.Components[0].Envs[i].Value = "false"
				}
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds, karaviAuthconfig, proxyAuthzTokens).Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op := tc(t)
			k8sClient := clientgoclient.NewFakeClient(sourceClient)
			err := PowerMaxMetrics(ctx, isDeleting, op, cr, sourceClient, k8sClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestObservabilityCertIssuer(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			cr := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind: "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "application-mobility-certificate",
				},
			}
			certmanagerv1.AddToScheme(scheme.Scheme)
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},

		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			certmanagerv1.AddToScheme(scheme.Scheme)
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mobility module not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			tmpCR := customResource

			return false, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
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
			success, isDeleting, cr, sourceClient, op := tc(t)

			err := IssuerCertService(ctx, isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
