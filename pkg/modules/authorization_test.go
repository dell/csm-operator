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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	drivers "github.com/dell/csm-operator/pkg/drivers"
	utils "github.com/dell/csm-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	"sigs.k8s.io/yaml"

	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	operatorConfig utils.OperatorConfig
)

func TestMain(m *testing.M) {
	status := 0

	operatorConfig = utils.OperatorConfig{}
	operatorConfig.ConfigDirectory = "../../operatorconfig"

	if st := m.Run(); st > status {
		status = st
	}

	os.Exit(status)
}

func getCustomResource() (csmv1.ContainerStorageModule, error) {
	b, err := ioutil.ReadFile("./testdata/cr_powerscale_auth.yaml")
	if err != nil {
		log.Fatalf("failed to read testdata: %v", err)

	}
	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(b, &customResource)
	if err != nil {
		return customResource, fmt.Errorf("failed to read unmarshal CSM Custom resource: %v", err)
	}

	return customResource, nil

}

func TestAuthInjectDaemonset(t *testing.T) {
	ctx := context.Background()
	correctlyInjected := func(ds applyv1.DaemonSetApplyConfiguration, drivertype string) error {
		err := CheckAnnotation(ds.Annotations)
		if err != nil {
			return err
		}
		err = CheckApplyVolumes(ds.Spec.Template.Spec.Volumes)
		if err != nil {
			return err
		}

		err = CheckApplyContainers(ds.Spec.Template.Spec.Containers, drivertype)
		if err != nil {
			return err
		}
		return nil
	}
	//*appsv1.DaemonSet
	tests := map[string]func(t *testing.T) (bool, applyv1.DaemonSetApplyConfiguration, utils.OperatorConfig){
		"success - greenfield injection": func(*testing.T) (bool, applyv1.DaemonSetApplyConfiguration, utils.OperatorConfig) {
			customResource, err := getCustomResource()
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
			customResource, err := getCustomResource()
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
			customResource, err := getCustomResource()
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
			customResource, err := getCustomResource()
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
		err := CheckAnnotation(dp.Annotations)
		if err != nil {
			return err
		}
		err = CheckApplyVolumes(dp.Spec.Template.Spec.Volumes)
		if err != nil {
			return err
		}
		err = CheckApplyContainers(dp.Spec.Template.Spec.Containers, drivertype)
		if err != nil {
			return err
		}
		return nil
	}

	tests := map[string]func(t *testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule){
		"success - greenfield injection": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource()
			if err != nil {
				panic(err)
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName)
			if err != nil {
				panic(err)
			}
			return true, controllerYAML.Deployment, operatorConfig, customResource
		},
		"success - brownfiled injection": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
			customResource, err := getCustomResource()
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
			customResource, err := getCustomResource()
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
	getSecret := func(namespace, secretName string) *corev1.Secret {
		return &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        secretName,
				Namespace:   namespace,
				Annotations: map[string]string{},
			},
		}
	}
	tests := map[string]func(t *testing.T) (bool, string, string, csmv1.Module, ctrlClient.Client){
		"success": func(*testing.T) (bool, string, string, csmv1.Module, ctrlClient.Client) {
			customResource, err := getCustomResource()
			if err != nil {
				panic(err)
			}
			namespace := customResource.Namespace
			drivertype := string(customResource.Spec.Driver.CSIDriverType)
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]

			karaviAuthconfig := getSecret(namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(namespace, "proxy-authz-tokens")

			client := ctrlClientFake.NewClientBuilder().WithObjects(karaviAuthconfig, proxyAuthzTokens).Build()

			return true, namespace, drivertype, auth, client
		},
		"success - version provided": func(*testing.T) (bool, string, string, csmv1.Module, ctrlClient.Client) {
			customResource, err := getCustomResource()
			if err != nil {
				panic(err)
			}
			namespace := customResource.Namespace
			drivertype := string(customResource.Spec.Driver.CSIDriverType)
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			auth.ConfigVersion = "v1.0.0"

			karaviAuthconfig := getSecret(namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(namespace, "proxy-authz-tokens")

			client := ctrlClientFake.NewClientBuilder().WithObjects(karaviAuthconfig, proxyAuthzTokens).Build()

			return true, namespace, drivertype, auth, client
		},
		"fail - INSECURE is false but no cert": func(*testing.T) (bool, string, string, csmv1.Module, ctrlClient.Client) {
			customResource, err := getCustomResource()
			if err != nil {
				panic(err)
			}
			namespace := customResource.Namespace
			drivertype := string(customResource.Spec.Driver.CSIDriverType)
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]

			// set insecure to false
			for i, env := range auth.Components[0].Envs {
				if env.Name == "INSECURE" {
					auth.Components[0].Envs[i].Value = "false"
				}
			}

			karaviAuthconfig := getSecret(namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(namespace, "proxy-authz-tokens")
			client := ctrlClientFake.NewClientBuilder().WithObjects(karaviAuthconfig, proxyAuthzTokens).Build()

			return false, namespace, drivertype, auth, client
		},
		"fail - invalid INSECURE value": func(*testing.T) (bool, string, string, csmv1.Module, ctrlClient.Client) {
			customResource, err := getCustomResource()
			if err != nil {
				panic(err)
			}
			namespace := customResource.Namespace
			drivertype := string(customResource.Spec.Driver.CSIDriverType)
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			// set insecure to false
			for i, env := range auth.Components[0].Envs {
				if env.Name == "INSECURE" {
					auth.Components[0].Envs[i].Value = "1234"
				}
			}

			client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, namespace, drivertype, auth, client
		},
		"fail - empty proxy host": func(*testing.T) (bool, string, string, csmv1.Module, ctrlClient.Client) {
			customResource, err := getCustomResource()
			if err != nil {
				panic(err)
			}
			namespace := customResource.Namespace
			drivertype := string(customResource.Spec.Driver.CSIDriverType)
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]

			for i, env := range auth.Components[0].Envs {
				if env.Name == "PROXY_HOST" {
					auth.Components[0].Envs[i].Value = ""
				}
			}
			client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, namespace, drivertype, auth, client
		},

		"fail - unsupported driver": func(*testing.T) (bool, string, string, csmv1.Module, ctrlClient.Client) {
			customResource, err := getCustomResource()
			if err != nil {
				panic(err)
			}
			namespace := customResource.Namespace
			drivertype := "unsupported-driver"
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]

			client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, namespace, drivertype, auth, client
		},
		"fail - unsupported auth version": func(*testing.T) (bool, string, string, csmv1.Module, ctrlClient.Client) {
			customResource, err := getCustomResource()
			if err != nil {
				panic(err)
			}
			namespace := customResource.Namespace
			drivertype := string(customResource.Spec.Driver.CSIDriverType)
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			auth.ConfigVersion = "v100000.0.0"

			client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, namespace, drivertype, auth, client
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, namespace, drivertype, authModule, client := tc(t)
			err := AuthorizationPrecheck(context.TODO(), namespace, drivertype, operatorConfig, authModule, client)
			if success {
				assert.NoError(t, err)

			} else {
				assert.Error(t, err)
			}

		})
	}
}
