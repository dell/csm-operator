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

	csmv1 "github.com/dell/csm-operator/api/v1"
	utils "github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/tests/shared/clientgoclient"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestObservabilityPrecheck(t *testing.T) {
	type fakeControllerRuntimeClientWrapper func(clusterConfigData []byte) (ctrlClient.Client, error)

	tests := map[string]func(t *testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper){
		"success": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			isilonCreds := getSecret("karavi", "test-isilon-creds")

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

			isilonCreds := getSecret("karavi", "test-isilon-creds")

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

		"success - version provided": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			isilonCreds := getSecret("karavi", "test-isilon-creds")

			tmpCR := customResource
			observability := tmpCR.Spec.Modules[0]
			observability.ConfigVersion = "v1.3.0"

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

			isilonCreds := getSecret("karavi", "test-isilon-creds")
			karaviAuthconfig := getSecret("karavi", "karavi-authorization-config")
			proxyAuthzTokens := getSecret("karavi", "proxy-authz-tokens")

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

			isilonCreds := getSecret("karavi", "test-isilon-creds")

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

			isilonCreds := getSecret("karavi", "test-isilon-creds")

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "unsupported-driver"
			observability := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, observability, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"Fail - isilon secrets not provided": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "powerscale"
			observability := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, observability, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"Fail - auth secrets not provided": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			isilonCreds := getSecret("karavi", "test-isilon-creds")

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "powerscale"
			observability := tmpCR.Spec.Modules[0]
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, observability, tmpCR, sourceClient, fakeControllerRuntimeClient
		},

		"Fail - SKIP_CERTIFICATE_VALIDATION is false but no cert": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			isilonCreds := getSecret("karavi", "test-isilon-creds")
			karaviAuthconfig := getSecret("karavi", "karavi-authorization-config")
			proxyAuthzTokens := getSecret("karavi", "proxy-authz-tokens")

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "powerscale"
			observability := tmpCR.Spec.Modules[0]
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

			// set skipCertificateValidation to false
			for i, env := range auth.Components[0].Envs {
				if env.Name == "SKIP_CERTIFICATE_VALIDATION" {
					auth.Components[0].Envs[i].Value = "false"
				}
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds, karaviAuthconfig, proxyAuthzTokens).Build()

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

			err := ObservabilityPrecheck(context.TODO(), operatorConfig, observability, tmpCR, &fakeReconcile)
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

			err := ObservabilityTopology(context.TODO(), isDeleting, op, cr, sourceClient)
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

			tmpCR := customResource

			cr := &rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					Kind: "ClusterRole",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "karavi-metrics-powerscale-controller",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - deleting with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

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
		"success - creating with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true

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
			k8sClient := clientgoclient.NewFakeClient(sourceClient)
			err := PowerScaleMetrics(context.TODO(), isDeleting, op, cr, sourceClient, k8sClient)
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

			err := OtelCollector(context.TODO(), isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)

			} else {
				assert.Error(t, err)
			}

		})
	}
}
