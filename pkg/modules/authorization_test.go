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
	drivers "github.com/dell/csm-operator/pkg/drivers"
	utils "github.com/dell/csm-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAuthInjectDaemonset(t *testing.T) {
	ctx := context.Background()
	correctlyInjected := func(ds applyv1.DaemonSetApplyConfiguration, drivertype string) error {
		err := CheckAnnotationAuth(ds.Annotations)
		if err != nil {
			return err
		}
		err = CheckApplyVolumesAuth(ds.Spec.Template.Spec.Volumes)
		if err != nil {
			return err
		}

		err = CheckApplyContainersAuth(ds.Spec.Template.Spec.Containers, drivertype)
		if err != nil {
			return err
		}
		return nil
	}
	//*appsv1.DaemonSet
	tests := map[string]func(t *testing.T) (bool, applyv1.DaemonSetApplyConfiguration, utils.OperatorConfig){
		"success - greenfield injection": func(*testing.T) (bool, applyv1.DaemonSetApplyConfiguration, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}
			nodeYAML, err := drivers.GetNode(ctx, customResource, operatorConfig, csmv1.PowerScaleName, "node.yaml")
			if err != nil {
				panic(err)
			}

			return true, nodeYAML.DaemonSetApplyConfig, operatorConfig
		},
		"success - brownfiled injection": func(*testing.T) (bool, applyv1.DaemonSetApplyConfiguration, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}
			nodeYAML, err := drivers.GetNode(ctx, customResource, operatorConfig, csmv1.PowerScaleName, "node.yaml")
			if err != nil {
				panic(err)
			}
			newDaemonSet, err := AuthInjectDaemonset(nodeYAML.DaemonSetApplyConfig, customResource, operatorConfig)
			if err != nil {
				panic(err)
			}

			return true, *newDaemonSet, operatorConfig
		},
		"fail - bad config path": func(*testing.T) (bool, applyv1.DaemonSetApplyConfiguration, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}
			nodeYAML, err := drivers.GetNode(ctx, customResource, operatorConfig, csmv1.PowerScaleName, "node.yaml")
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
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}
			newDaemonSet, err := AuthInjectDaemonset(ds, customResource, opConfig)
			if success {
				assert.NoError(t, err)
				if err := correctlyInjected(*newDaemonSet, string(customResource.Spec.Driver.CSIDriverType)); err != nil {
					assert.NoError(t, err)
				}
			} else {
				assert.Error(t, err)
			}

		})
	}
}
func TestAuthInjectDeployment(t *testing.T) {
	ctx := context.Background()
	correctlyInjected := func(dp applyv1.DeploymentApplyConfiguration, drivertype string) error {
		err := CheckAnnotationAuth(dp.Annotations)
		if err != nil {
			return err
		}
		err = CheckApplyVolumesAuth(dp.Spec.Template.Spec.Volumes)
		if err != nil {
			return err
		}
		err = CheckApplyContainersAuth(dp.Spec.Template.Spec.Containers, drivertype)
		if err != nil {
			return err
		}
		return nil
	}

	tests := map[string]func(t *testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule){
		"success - greenfield injection": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName)
			if err != nil {
				panic(err)
			}
			return true, controllerYAML.Deployment, operatorConfig, customResource
		},
		"success - brownfield injection": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}
			tmpCR := customResource
			controllerYAML, err := drivers.GetController(ctx, tmpCR, operatorConfig, csmv1.PowerScaleName)
			if err != nil {
				panic(err)
			}
			newDeployment, err := AuthInjectDeployment(controllerYAML.Deployment, tmpCR, operatorConfig)
			if err != nil {
				panic(err)
			}

			return true, *newDeployment, operatorConfig, customResource
		},
		"fail - bad config path": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
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
			newDeployment, err := AuthInjectDeployment(dp, cr, opConfig)
			if success {
				assert.NoError(t, err)
				if err := correctlyInjected(*newDeployment, string(cr.Spec.Driver.CSIDriverType)); err != nil {
					assert.NoError(t, err)
				}
			} else {
				assert.Error(t, err)
			}

		})
	}

}
func TestAuthorizationPreCheck(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client){
		"success": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}
			namespace := customResource.Namespace
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]

			karaviAuthconfig := getSecret(namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(namespace, "proxy-authz-tokens")

			client := ctrlClientFake.NewClientBuilder().WithObjects(karaviAuthconfig, proxyAuthzTokens).Build()

			return true, auth, tmpCR, client
		},
		"success - version provided": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}
			namespace := customResource.Namespace
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			auth.ConfigVersion = "v1.4.0"

			karaviAuthconfig := getSecret(namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(namespace, "proxy-authz-tokens")

			client := ctrlClientFake.NewClientBuilder().WithObjects(karaviAuthconfig, proxyAuthzTokens).Build()

			return true, auth, tmpCR, client
		},
		"fail - SKIP_CERTIFICATE_VALIDATION is false but no cert": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}
			namespace := customResource.Namespace
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]

			// set skipCertificateValidation to false
			for i, env := range auth.Components[0].Envs {
				if env.Name == "SKIP_CERTIFICATE_VALIDATION" {
					auth.Components[0].Envs[i].Value = "false"
				}
			}

			karaviAuthconfig := getSecret(namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(namespace, "proxy-authz-tokens")
			client := ctrlClientFake.NewClientBuilder().WithObjects(karaviAuthconfig, proxyAuthzTokens).Build()

			return false, auth, tmpCR, client
		},
		"fail - invalid SKIP_CERTIFICATE_VALIDATION value": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			// set skipCertificateValidation to false
			for i, env := range auth.Components[0].Envs {
				if env.Name == "SKIP_CERTIFICATE_VALIDATION" {
					auth.Components[0].Envs[i].Value = "1234"
				}
			}

			client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, auth, tmpCR, client
		},
		"fail - empty proxy host": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]

			for i, env := range auth.Components[0].Envs {
				if env.Name == "PROXY_HOST" {
					auth.Components[0].Envs[i].Value = ""
				}
			}
			client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, auth, tmpCR, client
		},

		"fail - unsupported driver": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "unsupported-driver"
			auth := tmpCR.Spec.Modules[0]

			client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, auth, tmpCR, client
		},
		"fail - unsupported auth version": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			auth.ConfigVersion = "v100000.0.0"

			client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, auth, tmpCR, client
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, auth, tmpCR, client := tc(t)
			// err := ReplicationPrecheck(context.TODO(), operatorConfig, replica, tmpCR, sourceClient)
			err := AuthorizationPrecheck(context.TODO(), operatorConfig, auth, tmpCR, client)
			if success {
				assert.NoError(t, err)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestAuthorizationServerPreCheck(t *testing.T) {
	type fakeControllerRuntimeClientWrapper func(clusterConfigData []byte) (ctrlClient.Client, error)

	tests := map[string]func(t *testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper){

		"success": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]

			karaviConfig := getSecret(customResource.Namespace, "karavi-config-secret")
			karaviStorage := getSecret(customResource.Namespace, "karavi-storage-secret")
			karaviTLS := getSecret(customResource.Namespace, "karavi-auth-tls")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(karaviConfig, karaviStorage, karaviTLS).Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(karaviConfig, karaviStorage, karaviTLS).Build()
				return clusterClient, nil
			}

			return true, auth, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"success - version provided": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			auth.ConfigVersion = "v1.4.0"
			karaviConfig := getSecret(customResource.Namespace, "karavi-config-secret")
			karaviStorage := getSecret(customResource.Namespace, "karavi-storage-secret")
			karaviTLS := getSecret(customResource.Namespace, "karavi-auth-tls")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(karaviConfig, karaviStorage, karaviTLS).Build()
			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(karaviConfig, karaviStorage, karaviTLS).Build()
				return clusterClient, nil
			}

			return true, auth, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"fail - unsupported authorization version": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			auth.ConfigVersion = "v100000.0.0"

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, auth, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"fail - empty proxy host": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]

			for i, env := range auth.Components[0].Envs {
				if env.Name == "PROXY_HOST" {
					auth.Components[0].Envs[i].Value = ""
				}
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			fakeControllerRuntimeClient := func(clusterConfigData []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
				return clusterClient, nil
			}

			return false, auth, tmpCR, sourceClient, fakeControllerRuntimeClient
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

			success, auth, tmpCR, sourceClient, fakeControllerRuntimeClient := tc(t)
			utils.NewControllerRuntimeClientWrapper = fakeControllerRuntimeClient
			utils.NewK8sClientWrapper = func(clusterConfigData []byte) (*kubernetes.Clientset, error) {
				return nil, nil
			}

			fakeReconcile := utils.FakeReconcileCSM{
				Client:    sourceClient,
				K8sClient: fake.NewSimpleClientset(),
			}

			err := AuthorizationServerPrecheck(context.TODO(), operatorConfig, auth, tmpCR, &fakeReconcile)
			if success {
				assert.NoError(t, err)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestAuthorizationProxyServerDeployment(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			cr := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind: "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "csm-config-params",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			success, isDeleting, cr, sourceClient, op := tc(t)

			err := AuthorizationServer(context.TODO(), isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestAuthorizationIngressRules(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			i1 := &networking.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind: "Ingress",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "proxy-server",
				},
			}

			i2 := &networking.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind: "Ingress",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "tenant-service",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(i1, i2).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			success, isDeleting, cr, sourceClient, op := tc(t)

			err := AuthorizationIngress(context.TODO(), isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestAuthorizationPolicies(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			cr := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind: "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "common",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			success, isDeleting, cr, sourceClient, op := tc(t)

			err := InstallPolicies(context.TODO(), isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestAuthorizationNginxIngressController(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			cr := &networking.IngressClass{
				TypeMeta: metav1.TypeMeta{
					Kind: "IngressClass",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "nginx",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()
			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			success, isDeleting, cr, sourceClient, op := tc(t)

			err := NginxIngressController(context.TODO(), isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestAuthorizationCertManager(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			cr := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind: "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "authorization-cert-manager",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()
			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, utils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			success, isDeleting, cr, sourceClient, op := tc(t)

			err := AuthorizationCertManager(context.TODO(), isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)

			} else {
				assert.Error(t, err)
			}

		})
	}
}
