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
	utils "github.com/dell/csm-operator/pkg/utils"
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
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
		"happy path": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
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
		"fail - app mob config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
		"fail - bad yaml found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			tmpCR := customResource

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
		"fail - bad yaml found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
			certmanagerv1.AddToScheme(scheme.Scheme)
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},

		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
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
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
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
			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
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
			appMobility.ConfigVersion = "v1.0.0"
			ns := "default"
			licenceCred := getSecret(ns, "dls-license")
			ivLicense := getSecret(ns, "iv")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(licenceCred).WithObjects(ivLicense).Build()
			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
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
			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects(licenceCred).Build(), nil
			}

			return false, appMobility, tmpCR, sourceClient, fakeControllerRuntimeClient
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
			success, appMobility, tmpCR, sourceClient, fakeControllerRuntimeClient := tc(t)
			utils.NewControllerRuntimeClientWrapper = fakeControllerRuntimeClient
			utils.NewK8sClientWrapper = func(clusterConfigData []byte) (*kubernetes.Clientset, error) {
				return nil, nil
			}
			fakeReconcile := utils.FakeReconcileCSM{
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
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
			velerov1.AddToScheme(scheme.Scheme)
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},

		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			velerov1.AddToScheme(scheme.Scheme)
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
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
			oldNewControllerRuntimeClientWrapper := utils.NewControllerRuntimeClientWrapper
			oldNewK8sClientWrapper := utils.NewK8sClientWrapper
			defer func() {
				utils.NewControllerRuntimeClientWrapper = oldNewControllerRuntimeClientWrapper
				utils.NewK8sClientWrapper = oldNewK8sClientWrapper
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
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
	tests := map[string]func(t *testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
			apiextv1.AddToScheme(scheme.Scheme)
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()
			return true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			apiextv1.AddToScheme(scheme.Scheme)
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mobility module not found": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
	tests := map[string]func(t *testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
			apiextv1.AddToScheme(scheme.Scheme)
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()
			return true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			apiextv1.AddToScheme(scheme.Scheme)
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mobility module not found": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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

			velerov1.AddToScheme(scheme.Scheme)
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()
			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			velerov1.AddToScheme(scheme.Scheme)

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"fail - app mob deployment file bad yaml": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mob config file not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_application_mobility.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - app mobility module not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
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
			oldNewControllerRuntimeClientWrapper := utils.NewControllerRuntimeClientWrapper
			oldNewK8sClientWrapper := utils.NewK8sClientWrapper
			defer func() {
				utils.NewControllerRuntimeClientWrapper = oldNewControllerRuntimeClientWrapper
				utils.NewK8sClientWrapper = oldNewK8sClientWrapper
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
