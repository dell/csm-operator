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

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	drivers "github.com/dell/csm-operator/pkg/drivers"
	utils "github.com/dell/csm-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"

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
		"success - brownfiled injection": func(*testing.T) (bool, applyv1.DeploymentApplyConfiguration, utils.OperatorConfig, csmv1.ContainerStorageModule) {
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
			auth.ConfigVersion = "v1.2.0"

			karaviAuthconfig := getSecret(namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(namespace, "proxy-authz-tokens")

			client := ctrlClientFake.NewClientBuilder().WithObjects(karaviAuthconfig, proxyAuthzTokens).Build()

			return true, auth, tmpCR, client
		},
		"fail - INSECURE is false but no cert": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}
			namespace := customResource.Namespace
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

			return false, auth, tmpCR, client
		},
		"fail - invalid INSECURE value": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_auth.yaml")
			if err != nil {
				panic(err)
			}
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			// set insecure to false
			for i, env := range auth.Components[0].Envs {
				if env.Name == "INSECURE" {
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
