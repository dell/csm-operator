// Copyright (c) 2022 - 2023 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"github.com/stretchr/testify/assert"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	appsv1 "k8s.io/api/apps/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAppMobilityModuleDeployment(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			cm := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind: "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "application-mobility-controller-manager",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cm).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"happy path": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mobility module not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			tmpCR := customResource

			return false, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op := tc(t)

			err := AppMobilityDeployment(context.TODO(), isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestAppMobilityWebhookService(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			cr := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind: "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "application-mobility-webhook-service",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()
			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"fail - bad yaml found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			tmpCR := customResource

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op := tc(t)

			err := AppMobilityWebhookService(context.TODO(), isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestControllerManagerMetricService(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			cr := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind: "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "application-mobility-controller-manager-metrics-service",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()
			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"fail - bad yaml found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op := tc(t)

			err := ControllerManagerMetricService(context.TODO(), isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestApplicationMobilityIssuerCertService(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
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
			err = certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},

		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
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
		"fail - app mobility module not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			tmpCR := customResource

			return false, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
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

			err := IssuerCertService(ctx, isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestApplicationMobilityPrecheck(t *testing.T) {
	type fakeControllerRuntimeClientWrapper func(clusterConfigData []byte) (ctrlClient.Client, error)

	tests := map[string]func(t *testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper){
		"success": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			ns := "default"
			licenceCred := getSecret(ns, "dls-license")
			ivLicense := getSecret(ns, "iv")
			tmpCR := customResource
			appMobility := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(licenceCred).WithObjects(ivLicense).Build()
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(licenceCred).WithObjects(ivLicense).Build()
				return clusterClient, nil
			}

			return true, appMobility, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"Fail - unsupported app-mobility version": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			appMobility := tmpCR.Spec.Modules[0]
			appMobility.ConfigVersion = "v10.10.10"
			ns := "default"
			licenceCred := getSecret(ns, "dls-license")
			ivLicense := getSecret(ns, "iv")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(licenceCred).WithObjects(ivLicense).Build()
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects(licenceCred).WithObjects(ivLicense).Build(), nil
			}

			return false, appMobility, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"Success - working app-mobility version": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			appMobility := tmpCR.Spec.Modules[0]
			appMobility.ConfigVersion = "v1.2.0"
			ns := "default"
			licenceCred := getSecret(ns, "dls-license")
			ivLicense := getSecret(ns, "iv")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(licenceCred).WithObjects(ivLicense).Build()
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects(licenceCred).WithObjects(ivLicense).Build(), nil
			}

			return true, appMobility, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"failed to find secret": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			appMobility := tmpCR.Spec.Modules[0]
			ns := "default"
			licenceCred := getSecret(ns, "dls-licenses")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(licenceCred).Build()
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects(licenceCred).Build(), nil
			}

			return false, appMobility, tmpCR, sourceClient, fakeControllerRuntimeClient
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
			success, appMobility, tmpCR, sourceClient, fakeControllerRuntimeClient := tc(t)
			operatorutils.NewControllerRuntimeClientWrapper = fakeControllerRuntimeClient
			operatorutils.NewK8sClientWrapper = func(_ []byte) (*kubernetes.Clientset, error) {
				return nil, nil
			}
			fakeReconcile := operatorutils.FakeReconcileCSM{
				Client:    sourceClient,
				K8sClient: fake.NewSimpleClientset(),
			}
			err := ApplicationMobilityPrecheck(context.TODO(), operatorConfig, appMobility, tmpCR, &fakeReconcile)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestAppMobilityVelero(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			cr := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind: "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "application-mobility-velero",
				},
			}
			err = velerov1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},

		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			err = velerov1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},

		"success - upgrading with old node agent with valid old version": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			err = velerov1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			ApplicationMobilityOldVersion = "v1.0.3"

			return true, false, tmpCR, sourceClient, operatorConfig
		},

		"success - upgrade should finish if failed to remove old node agent Daemonset": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			err = velerov1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			ApplicationMobilityOldVersion = "old-version"

			return true, false, tmpCR, sourceClient, operatorConfig
		},

		"fail - app mobility module not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			tmpCR := customResource

			return false, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
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

			err := AppMobilityVelero(ctx, isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestAppMobilityCertManager(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			cr := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind: "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "application-mobility-cert-manager-webhook",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()
			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
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
			success, isDeleting, cr, sourceClient, op := tc(t)

			err := AppMobilityCertManager(ctx, isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestVeleroCrdDeploy(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			cr := &apiextv1.CustomResourceDefinition{
				TypeMeta: metav1.TypeMeta{
					Kind: "CustomResourceDefinition",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "backuprepositories.velero.io",
				},
			}
			err = apiextv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()
			return true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			err = apiextv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mobility module not found": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			tmpCR := customResource

			return false, tmpCR, sourceClient, operatorConfig
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, cr, sourceClient, op := tc(t)

			err := VeleroCrdDeploy(ctx, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestAppMobCrdDeploy(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			cr := &apiextv1.CustomResourceDefinition{
				TypeMeta: metav1.TypeMeta{
					Kind: "CustomResourceDefinition",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "clusterconfigs.mobility.storage.dell.com",
				},
			}
			err = apiextv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()
			return true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			err = apiextv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mobility module not found": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			tmpCR := customResource

			return false, tmpCR, sourceClient, operatorConfig
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, cr, sourceClient, op := tc(t)

			err := AppMobCrdDeploy(ctx, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestBackupStorageLoc(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			cr := &velerov1.BackupStorageLocation{
				TypeMeta: metav1.TypeMeta{
					Kind: "BackupStorageLocation",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
			}

			err = velerov1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()
			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			err = velerov1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mobility module not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			tmpCR := customResource

			return false, false, tmpCR, sourceClient, operatorConfig
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

			err := UseBackupStorageLoc(ctx, isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// currently will test getBackupStorageLoc and getUseVolumeSnapshot, as these methods are expected to use a custom region
// or use the default value "region" if custom region is not defined in CR
func TestRegionSubstitutions(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"region default value": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, tmpCR, sourceClient, operatorConfig
		},
		"region custom value": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility_custom_region.yaml")
			if err != nil {
				panic(err)
			}
			tmpCR := customResource
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return false, tmpCR, sourceClient, operatorConfig
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
			defaultRegion, cr, sourceClient, op := tc(t)

			bslName, bslYaml, err := getBackupStorageLoc(ctx, op, cr, sourceClient)
			vslName, vslYaml, errVsl := getUseVolumeSnapshot(ctx, op, cr, sourceClient)

			assert.NoError(t, err)
			assert.NoError(t, errVsl)

			if defaultRegion {
				// fields we set in testdata/cr_application_mobility.yaml
				assert.Equal(t, bslName, "default")
				assert.Equal(t, vslName, "default")
				// region is not set in cr_application_mobility.yaml, so default value should be used
				assert.Contains(t, bslYaml, "region: region")
				assert.Contains(t, vslYaml, "region: region")
			} else {
				// fields we set in testdata/cr_application_mobility_custom_region.yaml
				assert.Equal(t, bslName, "my-new-location")
				assert.Equal(t, vslName, "my-new-location")
				assert.Contains(t, bslYaml, "region: custom")
				assert.Contains(t, vslYaml, "region: custom")

			}
		})
	}
}
