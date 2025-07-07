// Copyright (c) 2025 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	"github.com/dell/csm-operator/tests/shared"
	"github.com/dell/csm-operator/tests/shared/clientgoclient"
	"github.com/dell/csm-operator/tests/shared/crclient"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	confv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
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
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
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
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
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
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
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
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
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
			observability.ConfigVersion = "v1.10.0"

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
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
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
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

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
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

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, observability, tmpCR, sourceClient, fakeControllerRuntimeClient
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

			success, observability, tmpCR, sourceClient, fakeControllerRuntimeClient := tc(t)
			operatorutils.NewControllerRuntimeClientWrapper = fakeControllerRuntimeClient
			operatorutils.NewK8sClientWrapper = func(_ []byte) (*kubernetes.Clientset, error) {
				return nil, nil
			}

			fakeReconcile := operatorutils.FakeReconcileCSM{
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
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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

		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},

		"Fail - observability module not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
		"success - deleting with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
		"success - deleting with auth after one cycle": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}
			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - creating with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
		"success - update objects": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			objects := map[shared.StorageKey]runtime.Object{}
			fakeClient := crclient.NewFakeClientNoInjector(objects)
			err = fakeClient.Create(ctx, isilonCreds)
			if err != nil {
				panic(err)
			}
			err = fakeClient.Create(ctx, karaviAuthconfig)
			if err != nil {
				panic(err)
			}
			err = fakeClient.Create(ctx, proxyAuthzTokens)
			if err != nil {
				panic(err)
			}
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
		"Fail - wrong module name": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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

		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},

		"success - with older otel image": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability_with_old_otel_image.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},

		"Fail - wrong module name": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
		"success - deleting with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
		"success - deleting with auth after one cycle": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}
			vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(vxflexosCreds).Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - creating with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
		"success - update objects": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}

			vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			objects := map[shared.StorageKey]runtime.Object{}
			fakeClient := crclient.NewFakeClientNoInjector(objects)
			err = fakeClient.Create(ctx, vxflexosCreds)
			if err != nil {
				panic(err)
			}
			err = fakeClient.Create(ctx, karaviAuthconfig)
			if err != nil {
				panic(err)
			}
			err = fakeClient.Create(ctx, proxyAuthzTokens)
			if err != nil {
				panic(err)
			}
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
		"Fail - wrong module name": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
		"success - deleting with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
		"success - deleting with auth after one cycle": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
			if err != nil {
				panic(err)
			}
			pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds).Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - creating with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
		"success - update objects": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
			if err != nil {
				panic(err)
			}

			pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")

			objects := map[shared.StorageKey]runtime.Object{}
			fakeClient := crclient.NewFakeClientNoInjector(objects)
			err = fakeClient.Create(ctx, pmaxCreds)
			if err != nil {
				panic(err)
			}
			err = fakeClient.Create(ctx, karaviAuthconfig)
			if err != nil {
				panic(err)
			}
			err = fakeClient.Create(ctx, proxyAuthzTokens)
			if err != nil {
				panic(err)
			}
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
		"success - dynamically mount secret (2.14.0+)": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability_use_secret.yaml")
			if err != nil {
				panic(err)
			}
			pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")

			customResource.Spec.Driver.Common.Envs = append(customResource.Spec.Driver.Common.Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "true"})

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds).Build()

			return true, false, customResource, sourceClient, operatorConfig
		},
		"success - dynamically mount configMap (2.14.0+)": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability_use_secret.yaml")
			if err != nil {
				panic(err)
			}
			pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")

			customResource.Spec.Driver.Common.Envs = append(customResource.Spec.Driver.Common.Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "false"})

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds).Build()

			return true, false, customResource, sourceClient, operatorConfig
		},
		"Fail - invalid config version": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability_use_secret.yaml")
			if err != nil {
				panic(err)
			}
			pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")

			customResource.Spec.Driver.Common.Envs = append(customResource.Spec.Driver.Common.Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "false"})

			customResource.Spec.Driver.ConfigVersion = "invalid-version"

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds).Build()

			return false, false, customResource, sourceClient, operatorConfig
		},
		"Fail - wrong module name": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_replica.yaml")
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
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - creating with self-signed cert": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			err = certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - creating with custom cert": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability_custom_cert.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			err = certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - creating with partial custom cert": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability_custom_cert_missing_key.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			err = certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - observability module not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			tmpCR := customResource

			return false, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - observability deployment file bad yaml": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - observability config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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
			oldNewControllerRuntimeClientWrapper := operatorutils.NewControllerRuntimeClientWrapper
			oldNewK8sClientWrapper := operatorutils.NewK8sClientWrapper
			defer func() {
				operatorutils.NewControllerRuntimeClientWrapper = oldNewControllerRuntimeClientWrapper
				operatorutils.NewK8sClientWrapper = oldNewK8sClientWrapper
			}()
			success, isDeleting, cr, sourceClient, op := tc(t)

			err := IssuerCertServiceObs(ctx, isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestSetPowerMaxMetricsConfigMap(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, *confv1.DeploymentApplyConfiguration, csmv1.ContainerStorageModule){
		"success - dynamically mount configMap": func(*testing.T) (bool, *confv1.DeploymentApplyConfiguration, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability_use_secret.yaml")
			if err != nil {
				panic(err)
			}

			customResource.Spec.Driver.Common.Envs = append(customResource.Spec.Driver.Common.Envs,
				corev1.EnvVar{Name: "X_CSI_REVPROXY_USE_SECRET", Value: "false"})

			mountName := "powermax-reverseproxy-config"
			mountPath := "/etc/reverseproxy"
			dp := &confv1.DeploymentApplyConfiguration{
				Spec: &confv1.DeploymentSpecApplyConfiguration{
					Template: &acorev1.PodTemplateSpecApplyConfiguration{
						Spec: &acorev1.PodSpecApplyConfiguration{
							Containers: []acorev1.ContainerApplyConfiguration{
								{
									VolumeMounts: []acorev1.VolumeMountApplyConfiguration{
										{
											Name:      &mountName,
											MountPath: &mountPath,
										},
									},
								},
							},
						},
					},
				},
			}

			return true, dp, customResource
		},
		"Fail - wrong module name": func(*testing.T) (bool, *confv1.DeploymentApplyConfiguration, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powermax_replica.yaml")
			if err != nil {
				panic(err)
			}

			return false, nil, customResource
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, dp, cr := tc(t)
			err := setPowerMaxMetricsConfigMap(dp, cr)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
