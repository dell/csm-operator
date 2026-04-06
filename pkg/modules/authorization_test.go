// Copyright (c) 2022-2025 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package modules

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"

	csmv1 "eos2git.cec.lab.emc.com/CSM/csm-operator/api/v1"
	drivers "eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/drivers"
	operatorutils "eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/operatorutils"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"

	shared "eos2git.cec.lab.emc.com/CSM/csm-operator/tests/sharedutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var (
	authCRName = "auth"
	authNS     = "auth-test"

	trueBool  = true
	falseBool = false
)

func TestCheckAnnotationAuth(t *testing.T) {
	t.Run("it handles an empty annotation", func(t *testing.T) {
		var empty map[string]string
		err := CheckAnnotationAuth(empty)
		if err == nil {
			t.Errorf("expected non-nil err, got %v", err)
		}
	})

	t.Run("it handles an incorrect auth annotation", func(t *testing.T) {
		want := "com.dell.karavi-authorization-proxy"
		invalid := map[string]string{
			"annotation": "test.proxy",
		}
		got := CheckAnnotationAuth(invalid)
		if got == nil {
			t.Errorf("got %v, expected annotation to be %s", got, want)
		}
	})

	t.Run("it handles an invalid annotation", func(t *testing.T) {
		got := map[string]string{
			"com.dell.karavi-authorization-proxy": "false",
		}
		err := CheckAnnotationAuth(got)
		if err == nil {
			t.Errorf("got %v, expected annotation to be true %s", got, err)
		}
	})
}

func TestCheckApplyVolumesAuth(t *testing.T) {
	t.Run("it handles an empty volume", func(t *testing.T) {
		got := []acorev1.VolumeApplyConfiguration{}
		customResource := csmv1.ContainerStorageModule{}
		authVersion := shared.AuthServerConfigVersion
		driverType := "powerscale"
		err := CheckApplyVolumesAuth(got, authVersion, driverType, customResource, ctrlClientFake.NewFakeClient())
		if err == nil {
			t.Errorf("got %v, expected to be missing isilon-config volume", got)
		}
	})

	t.Run("it handles karavi-authorization-config volume", func(t *testing.T) {
		got := []acorev1.VolumeApplyConfiguration{}
		customResource := csmv1.ContainerStorageModule{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "isilon",
			},
		}
		authVersion := shared.AuthServerConfigVersion
		driverType := "powerscale"
		karaviSecret := getSecret(customResource.Namespace, "karavi-authorization-config")
		client := ctrlClientFake.NewClientBuilder().WithObjects(karaviSecret).Build()
		err := CheckApplyVolumesAuth(got, authVersion, driverType, customResource, client)
		if err == nil {
			t.Errorf("got %v, expected to be missing karavi-authorization-config volume", got)
		}
	})

	t.Run("it handles karavi-authorization-config volume for versions v2.3.0 and below", func(t *testing.T) {
		got := []acorev1.VolumeApplyConfiguration{}
		customResource := csmv1.ContainerStorageModule{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "isilon",
			},
		}
		authVersion := "v2.3.0"
		driverType := "powerscale"
		karaviSecret := getSecret(customResource.Namespace, "karavi-authorization-config")
		client := ctrlClientFake.NewClientBuilder().WithObjects(karaviSecret).Build()
		err := CheckApplyVolumesAuth(got, authVersion, driverType, customResource, client)
		if err == nil {
			t.Errorf("got %v, expected to be missing karavi-authorization-config volume", got)
		}
	})

	t.Run("success - driver secret version and karavi secret exists", func(t *testing.T) {
		customResource := csmv1.ContainerStorageModule{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "isilon",
			},
		}
		authVersion := "v2.4.0"
		driverType := "powerscale"
		karaviSecret := getSecret(customResource.Namespace, "karavi-authorization-config")
		client := ctrlClientFake.NewClientBuilder().WithObjects(karaviSecret).Build()

		volName := KaraviAuthorizationConfigSecret
		vols := []acorev1.VolumeApplyConfiguration{
			*acorev1.Volume().WithName(volName),
		}
		err := CheckApplyVolumesAuth(vols, authVersion, driverType, customResource, client)
		assert.NoError(t, err)
	})

	t.Run("success - driver secret version and karavi secret missing", func(t *testing.T) {
		customResource := csmv1.ContainerStorageModule{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "isilon",
			},
		}
		authVersion := "v2.4.0"
		driverType := "powerscale"
		client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

		expected := AuthorizationSupportedDrivers[driverType].DriverConfigVolumeMount
		vols := []acorev1.VolumeApplyConfiguration{
			*acorev1.Volume().WithName(expected),
		}
		err := CheckApplyVolumesAuth(vols, authVersion, driverType, customResource, client)
		assert.NoError(t, err)
	})

	t.Run("fail - invalid auth config version", func(t *testing.T) {
		customResource := csmv1.ContainerStorageModule{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "isilon",
			},
		}
		authVersion := "not-a-version"
		driverType := "powerscale"
		client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

		volName := KaraviAuthorizationConfigSecret
		vols := []acorev1.VolumeApplyConfiguration{
			*acorev1.Volume().WithName(volName),
		}
		err := CheckApplyVolumesAuth(vols, authVersion, driverType, customResource, client)
		assert.Error(t, err)
	})
}

func TestCheckApplyContainersAuth(t *testing.T) {
	t.Run("it handles no config params volume mount", func(t *testing.T) {
		got := []acorev1.ContainerApplyConfiguration{
			*acorev1.Container().WithName("karavi-authorization-proxy"),
		}
		customResource := csmv1.ContainerStorageModule{}
		driver := "powerscale"
		authVersion := "v2.3.0"
		err := CheckApplyContainersAuth(got, driver, true, authVersion, customResource, ctrlClientFake.NewFakeClient())
		if err == nil {
			t.Errorf("got %v, expected csi-isilon-config-params to be injected", got)
		}
	})

	t.Run("fail - invalid auth config version", func(t *testing.T) {
		driver := "powerscale"
		vol1Name := "karavi-authorization-config"
		vol2Name := AuthorizationSupportedDrivers[driver].DriverConfigParamsVolumeMount
		envName := "PROXY_HOST"
		envVal := "proxy.example.local"

		got := []acorev1.ContainerApplyConfiguration{
			*acorev1.Container().WithName("karavi-authorization-proxy").
				WithVolumeMounts(&acorev1.VolumeMountApplyConfiguration{Name: &vol1Name},
					&acorev1.VolumeMountApplyConfiguration{Name: &vol2Name}).
				WithEnv(&acorev1.EnvVarApplyConfiguration{Name: &envName, Value: &envVal}),
		}

		customResource := csmv1.ContainerStorageModule{}
		authVersion := "not-a-version"
		err := CheckApplyContainersAuth(got, driver, true, authVersion, customResource, ctrlClientFake.NewFakeClient())
		assert.ErrorContains(t, err, "error checking version")
	})

	t.Run("it handles an empty container", func(t *testing.T) {
		got := []acorev1.ContainerApplyConfiguration{}
		customResource := csmv1.ContainerStorageModule{}
		driver := "powerscale"
		authVersion := shared.AuthServerConfigVersion
		err := CheckApplyContainersAuth(got, driver, true, authVersion, customResource, ctrlClientFake.NewFakeClient())
		if err == nil {
			t.Errorf("got %v, expected karavi-authorization-config to be injected", got)
		}
	})

	t.Run("it validates env malformed variable values", func(t *testing.T) {
		driver := "powerscale"
		vol1Name := "karavi-authorization-config"
		vol2Name := AuthorizationSupportedDrivers[driver].DriverConfigParamsVolumeMount
		envName := "INSECURE"
		envVal := "not a boolean value"
		got := []acorev1.ContainerApplyConfiguration{
			*acorev1.Container().WithName("karavi-authorization-proxy").
				WithVolumeMounts(&acorev1.VolumeMountApplyConfiguration{Name: &vol1Name},
					&acorev1.VolumeMountApplyConfiguration{Name: &vol2Name}).
				WithEnv(&acorev1.EnvVarApplyConfiguration{Name: &envName, Value: &envVal}),
		}
		customResource := csmv1.ContainerStorageModule{}
		authVersion := shared.AuthServerConfigVersion
		err := CheckApplyContainersAuth(got, driver, true, authVersion, customResource, ctrlClientFake.NewFakeClient())
		assert.Error(t, err)
	})

	t.Run("it validates SKIP_CERTIFICATE_VALIDATION malformed value", func(t *testing.T) {
		driver := "powerscale"
		vol1Name := "karavi-authorization-config"
		vol2Name := AuthorizationSupportedDrivers[driver].DriverConfigParamsVolumeMount
		envName := "SKIP_CERTIFICATE_VALIDATION"
		envVal := "not-a-bool"

		got := []acorev1.ContainerApplyConfiguration{
			*acorev1.Container().WithName("karavi-authorization-proxy").
				WithVolumeMounts(&acorev1.VolumeMountApplyConfiguration{Name: &vol1Name},
					&acorev1.VolumeMountApplyConfiguration{Name: &vol2Name}).
				WithEnv(&acorev1.EnvVarApplyConfiguration{Name: &envName, Value: &envVal}),
		}

		customResource := csmv1.ContainerStorageModule{}
		authVersion := "v2.3.0"
		err := CheckApplyContainersAuth(got, driver, true, authVersion, customResource, ctrlClientFake.NewFakeClient())
		assert.ErrorContains(t, err, "invalid value for SKIP_CERTIFICATE_VALIDATION")
	})

	t.Run("it validates conflicting cert configuration", func(t *testing.T) {
		driver := "powerscale"
		vol1Name := "karavi-authorization-config"
		vol2Name := AuthorizationSupportedDrivers[driver].DriverConfigParamsVolumeMount
		envName := "INSECURE"
		envVal := "false"

		got := []acorev1.ContainerApplyConfiguration{
			*acorev1.Container().WithName("karavi-authorization-proxy").
				WithVolumeMounts(&acorev1.VolumeMountApplyConfiguration{Name: &vol1Name},
					&acorev1.VolumeMountApplyConfiguration{Name: &vol2Name}).
				WithEnv(&acorev1.EnvVarApplyConfiguration{Name: &envName, Value: &envVal}),
		}
		customResource := csmv1.ContainerStorageModule{}
		authVersion := shared.AuthServerConfigVersion
		err := CheckApplyContainersAuth(got, driver, true, authVersion, customResource, ctrlClientFake.NewFakeClient())
		assert.Error(t, err)
	})

	t.Run("it validates cert env mismatch when skipCertificateValidation is true", func(t *testing.T) {
		driver := "powerscale"
		vol1Name := "karavi-authorization-config"
		vol2Name := AuthorizationSupportedDrivers[driver].DriverConfigParamsVolumeMount
		envName := "SKIP_CERTIFICATE_VALIDATION"
		envVal := "false"

		got := []acorev1.ContainerApplyConfiguration{
			*acorev1.Container().WithName("karavi-authorization-proxy").
				WithVolumeMounts(&acorev1.VolumeMountApplyConfiguration{Name: &vol1Name},
					&acorev1.VolumeMountApplyConfiguration{Name: &vol2Name}).
				WithEnv(&acorev1.EnvVarApplyConfiguration{Name: &envName, Value: &envVal}),
		}
		customResource := csmv1.ContainerStorageModule{}
		authVersion := "v2.3.0"
		err := CheckApplyContainersAuth(got, driver, true, authVersion, customResource, ctrlClientFake.NewFakeClient())
		assert.ErrorContains(t, err, "expected SKIP_CERTIFICATE_VALIDATION/INSECURE to be true")
	})

	t.Run("it validates cert env mismatch when skipCertificateValidation is false", func(t *testing.T) {
		driver := "powerscale"
		vol1Name := "karavi-authorization-config"
		vol2Name := AuthorizationSupportedDrivers[driver].DriverConfigParamsVolumeMount
		envName := "SKIP_CERTIFICATE_VALIDATION"
		envVal := "true"

		got := []acorev1.ContainerApplyConfiguration{
			*acorev1.Container().WithName("karavi-authorization-proxy").
				WithVolumeMounts(&acorev1.VolumeMountApplyConfiguration{Name: &vol1Name},
					&acorev1.VolumeMountApplyConfiguration{Name: &vol2Name}).
				WithEnv(&acorev1.EnvVarApplyConfiguration{Name: &envName, Value: &envVal}),
		}
		customResource := csmv1.ContainerStorageModule{}
		authVersion := "v2.3.0"
		err := CheckApplyContainersAuth(got, driver, false, authVersion, customResource, ctrlClientFake.NewFakeClient())
		assert.ErrorContains(t, err, "expected SKIP_CERTIFICATE_VALIDATION/INSECURE to be false")
	})

	t.Run("it validates empty proxy host value", func(t *testing.T) {
		driver := "powerscale"
		vol1Name := "karavi-authorization-config"
		vol2Name := AuthorizationSupportedDrivers[driver].DriverConfigParamsVolumeMount
		envName := "PROXY_HOST"
		envVal := ""

		got := []acorev1.ContainerApplyConfiguration{
			*acorev1.Container().WithName("karavi-authorization-proxy").
				WithVolumeMounts(&acorev1.VolumeMountApplyConfiguration{Name: &vol1Name},
					&acorev1.VolumeMountApplyConfiguration{Name: &vol2Name}).
				WithEnv(&acorev1.EnvVarApplyConfiguration{Name: &envName, Value: &envVal}),
		}
		customResource := csmv1.ContainerStorageModule{}
		authVersion := "v2.3.0"
		err := CheckApplyContainersAuth(got, driver, true, authVersion, customResource, ctrlClientFake.NewFakeClient())
		assert.Error(t, err)
	})

	t.Run("it handles no karavi authorization config volume mount when karavi secret exists", func(t *testing.T) {
		driver := "powerscale"
		vol1Name := AuthorizationSupportedDrivers[driver].DriverConfigParamsVolumeMount

		got := []acorev1.ContainerApplyConfiguration{
			*acorev1.Container().WithName("karavi-authorization-proxy").
				WithVolumeMounts(&acorev1.VolumeMountApplyConfiguration{Name: &vol1Name}),
		}
		customResource := CsmAuthorizationCR()
		authVersion := shared.AuthServerConfigVersion
		clientWithSecret := ctrlClientFake.NewClientBuilder().WithObjects(getSecret(customResource.Namespace, "karavi-authorization-config")).Build()
		err := CheckApplyContainersAuth(got, driver, true, authVersion, customResource, clientWithSecret)
		assert.ErrorContains(t, err, "missing the following volume mount karavi-authorization-config")
	})

	t.Run("it handles no driver secret volume mount when karavi secret doesn't exist", func(t *testing.T) {
		driver := "powerscale"
		vol1Name := AuthorizationSupportedDrivers[driver].DriverConfigParamsVolumeMount

		got := []acorev1.ContainerApplyConfiguration{
			*acorev1.Container().WithName("karavi-authorization-proxy").
				WithVolumeMounts(&acorev1.VolumeMountApplyConfiguration{Name: &vol1Name}),
		}
		customResource := CsmAuthorizationCR()
		authVersion := shared.AuthServerConfigVersion
		err := CheckApplyContainersAuth(got, driver, true, authVersion, customResource, ctrlClientFake.NewFakeClient())
		assert.ErrorContains(t, err, "missing the following volume mount isilon-configs")
	})
}

func TestAuthInjectDaemonset(t *testing.T) {
	ctx := context.Background()
	correctlyInjected := func(ds applyv1.DaemonSetApplyConfiguration, drivertype string, skipCertificateValidation bool, authVersion string, cr csmv1.ContainerStorageModule) error {
		err := CheckAnnotationAuth(ds.Annotations)
		if err != nil {
			return err
		}
		err = CheckApplyVolumesAuth(ds.Spec.Template.Spec.Volumes, authVersion, drivertype, cr, ctrlClientFake.NewFakeClient())
		if err != nil {
			return err
		}

		err = CheckApplyContainersAuth(ds.Spec.Template.Spec.Containers, drivertype, skipCertificateValidation, authVersion, cr, ctrlClientFake.NewFakeClient())
		if err != nil {
			return err
		}
		return nil
	}
	//*appsv1.DaemonSet
	tests := map[string]func(t *testing.T) (bool, bool, applyv1.DaemonSetApplyConfiguration, operatorutils.OperatorConfig, string){
		"success - greenfield injection": func(*testing.T) (bool, bool, applyv1.DaemonSetApplyConfiguration, operatorutils.OperatorConfig, string) {
			customResource := csmPowerScaleWithAuthCR()
			nodeYAML, err := drivers.GetNode(ctx, customResource, operatorConfig, csmv1.PowerScaleName, "node.yaml", ctrlClientFake.NewFakeClient(), operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			authVersion := shared.AuthServerConfigVersion

			return true, true, nodeYAML.DaemonSetApplyConfig, operatorConfig, authVersion
		},
		"success - brownfield injection": func(*testing.T) (bool, bool, applyv1.DaemonSetApplyConfiguration, operatorutils.OperatorConfig, string) {
			customResource := csmPowerScaleWithAuthCR()
			nodeYAML, err := drivers.GetNode(ctx, customResource, operatorConfig, csmv1.PowerScaleName, "node.yaml", ctrlClientFake.NewFakeClient(), operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			newDaemonSet, err := AuthInjectDaemonset(context.TODO(), nodeYAML.DaemonSetApplyConfig, customResource, operatorConfig, ctrlClientFake.NewFakeClient())
			if err != nil {
				panic(err)
			}
			authVersion := shared.AuthServerConfigVersion

			return true, true, *newDaemonSet, operatorConfig, authVersion
		},
		"success - validate certificate": func(*testing.T) (bool, bool, applyv1.DaemonSetApplyConfiguration, operatorutils.OperatorConfig, string) {
			customResource := csmPowerScaleWithAuthCR()
			// set skip certificate validation to false
			for i := range customResource.Spec.Modules[0].Components[0].Envs {
				if customResource.Spec.Modules[0].Components[0].Envs[i].Name == "SKIP_CERTIFICATE_VALIDATION" {
					customResource.Spec.Modules[0].Components[0].Envs[i].Value = "false"
				}
			}
			nodeYAML, err := drivers.GetNode(ctx, customResource, operatorConfig, csmv1.PowerScaleName, "node.yaml", ctrlClientFake.NewFakeClient(), operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			newDaemonSet, err := AuthInjectDaemonset(context.TODO(), nodeYAML.DaemonSetApplyConfig, customResource, operatorConfig, ctrlClientFake.NewFakeClient())
			if err != nil {
				panic(err)
			}
			authVersion := shared.AuthServerConfigVersion

			return true, false, *newDaemonSet, operatorConfig, authVersion
		},
		"fail - bad config path": func(*testing.T) (bool, bool, applyv1.DaemonSetApplyConfiguration, operatorutils.OperatorConfig, string) {
			customResource := csmPowerScaleWithAuthCR()
			nodeYAML, err := drivers.GetNode(ctx, customResource, operatorConfig, csmv1.PowerScaleName, "node.yaml", ctrlClientFake.NewFakeClient(), operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			tmpOperatorConfig := operatorConfig
			tmpOperatorConfig.ConfigDirectory = "bad/path"
			authVersion := customResource.Spec.Modules[0].ConfigVersion

			return false, false, nodeYAML.DaemonSetApplyConfig, tmpOperatorConfig, authVersion
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, skipCertificateValidation, ds, opConfig, authVersion := tc(t)
			customResource := csmPowerScaleWithAuthCR()
			newDaemonSet, err := AuthInjectDaemonset(context.TODO(), ds, customResource, opConfig, ctrlClientFake.NewFakeClient())
			if success {
				assert.NoError(t, err)
				if err := correctlyInjected(*newDaemonSet, string(customResource.Spec.Driver.CSIDriverType), skipCertificateValidation, authVersion, customResource); err != nil {
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
	correctlyInjected := func(dp applyv1.DeploymentApplyConfiguration, drivertype string, skipCertificateValidation bool, authVersion string, cr csmv1.ContainerStorageModule, ctrlClient ctrlClient.Client) error {
		err := CheckAnnotationAuth(dp.Annotations)
		if err != nil {
			return err
		}
		err = CheckApplyVolumesAuth(dp.Spec.Template.Spec.Volumes, authVersion, drivertype, cr, ctrlClient)
		if err != nil {
			return err
		}
		err = CheckApplyContainersAuth(dp.Spec.Template.Spec.Containers, drivertype, skipCertificateValidation, authVersion, cr, ctrlClient)
		if err != nil {
			return err
		}
		return nil
	}

	tests := map[string]func(t *testing.T) (bool, bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule, string, ctrlClient.Client){
		"success - greenfield injection": func(*testing.T) (bool, bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule, string, ctrlClient.Client) {
			customResource := csmPowerScaleWithAuthCR()
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			authVersion := shared.AuthServerConfigVersion
			return true, true, controllerYAML.Deployment, operatorConfig, customResource, authVersion, ctrlClientFake.NewFakeClient()
		},
		"success - greenfield injection missing skip certificate validation env": func(*testing.T) (bool, bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule, string, ctrlClient.Client) {
			customResource := csmPowerScaleWithAuthCR()
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			// Remove SKIP_CERTIFICATE_VALIDATION/INSECURE from the generated deployment to simulate a missing env.
			if controllerYAML.Deployment.Spec != nil &&
				controllerYAML.Deployment.Spec.Template != nil &&
				controllerYAML.Deployment.Spec.Template.Spec != nil &&
				len(controllerYAML.Deployment.Spec.Template.Spec.Containers) > 0 {
				envs := controllerYAML.Deployment.Spec.Template.Spec.Containers[0].Env
				filtered := []acorev1.EnvVarApplyConfiguration{}
				for _, e := range envs {
					if e.Name == nil {
						filtered = append(filtered, e)
						continue
					}
					if *e.Name == "SKIP_CERTIFICATE_VALIDATION" || *e.Name == "INSECURE" {
						continue
					}
					filtered = append(filtered, e)
				}
				controllerYAML.Deployment.Spec.Template.Spec.Containers[0].Env = filtered
			}
			authVersion := shared.AuthServerConfigVersion
			return true, true, controllerYAML.Deployment, operatorConfig, customResource, authVersion, ctrlClientFake.NewFakeClient()
		},
		"success - brownfield injection": func(*testing.T) (bool, bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule, string, ctrlClient.Client) {
			customResource := csmPowerScaleWithAuthCR()
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			ctrlClient := ctrlClientFake.NewFakeClient()
			newDeployment, err := AuthInjectDeployment(context.TODO(), controllerYAML.Deployment, customResource, operatorConfig, ctrlClient)
			if err != nil {
				panic(err)
			}
			authVersion := shared.AuthServerConfigVersion
			return true, true, *newDeployment, operatorConfig, customResource, authVersion, ctrlClient
		},
		"success - greenfield injection with driver secret": func(*testing.T) (bool, bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule, string, ctrlClient.Client) {
			customResource := csmPowerScaleWithAuthCR()
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			authVersion := shared.AuthServerConfigVersion
			return true, true, controllerYAML.Deployment, operatorConfig, customResource, authVersion, ctrlClientFake.NewFakeClient()
		},
		"success - validate certificate": func(*testing.T) (bool, bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule, string, ctrlClient.Client) {
			customResource := csmPowerScaleWithAuthCR()
			// set skip certificate validation to false
			for i := range customResource.Spec.Modules[0].Components[0].Envs {
				if customResource.Spec.Modules[0].Components[0].Envs[i].Name == "SKIP_CERTIFICATE_VALIDATION" {
					customResource.Spec.Modules[0].Components[0].Envs[i].Value = "false"
				}
			}
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			ctrlClient := ctrlClientFake.NewFakeClient()
			newDeployment, err := AuthInjectDeployment(context.TODO(), controllerYAML.Deployment, customResource, operatorConfig, ctrlClient)
			if err != nil {
				panic(err)
			}
			authVersion := shared.AuthServerConfigVersion
			return true, false, *newDeployment, operatorConfig, customResource, authVersion, ctrlClient
		},
		"fail - bad config path": func(*testing.T) (bool, bool, applyv1.DeploymentApplyConfiguration, operatorutils.OperatorConfig, csmv1.ContainerStorageModule, string, ctrlClient.Client) {
			customResource := csmPowerScaleWithAuthCR()
			controllerYAML, err := drivers.GetController(ctx, customResource, operatorConfig, csmv1.PowerScaleName, operatorutils.VersionSpec{})
			if err != nil {
				panic(err)
			}
			tmpOperatorConfig := operatorConfig
			tmpOperatorConfig.ConfigDirectory = "bad/path"
			authVersion := shared.AuthServerConfigVersion
			return false, true, controllerYAML.Deployment, tmpOperatorConfig, customResource, authVersion, ctrlClientFake.NewFakeClient()
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, skipCertificateValidation, dp, opConfig, cr, authVersion, client := tc(t)
			newDeployment, err := AuthInjectDeployment(context.TODO(), dp, cr, opConfig, client)
			if success {
				assert.NoError(t, err)
				if err := correctlyInjected(*newDeployment, string(cr.Spec.Driver.CSIDriverType), skipCertificateValidation, authVersion, cr, client); err != nil {
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
			customResource := csmPowerScaleWithAuthCR()
			namespace := customResource.Namespace
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			auth.ConfigVersion = shared.AuthServerConfigVersion

			proxyAuthzTokens := getSecret(namespace, "proxy-authz-tokens")

			client := ctrlClientFake.NewClientBuilder().WithObjects(proxyAuthzTokens).Build()

			return true, auth, tmpCR, client
		},
		"success - v2.3.0": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource := csmPowerScaleWithAuthCR()
			namespace := customResource.Namespace
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			auth.ConfigVersion = "v2.3.0"

			karaviAuthconfig := getSecret(namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(namespace, "proxy-authz-tokens")

			client := ctrlClientFake.NewClientBuilder().WithObjects(karaviAuthconfig, proxyAuthzTokens).Build()

			return true, auth, tmpCR, client
		},
		"fail - v2.3.0 has no karavi-authorization-config secret": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource := csmPowerScaleWithAuthCR()
			namespace := customResource.Namespace
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			auth.ConfigVersion = "v2.3.0"

			proxyAuthzTokens := getSecret(namespace, "proxy-authz-tokens")

			client := ctrlClientFake.NewClientBuilder().WithObjects(proxyAuthzTokens).Build()

			return false, auth, tmpCR, client
		},
		"fail - SKIP_CERTIFICATE_VALIDATION is false but no cert": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource := csmPowerScaleWithAuthCR()
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
			customResource := csmPowerScaleWithAuthCR()
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			// set skipCertificateValidation to an invalid value
			for i, env := range auth.Components[0].Envs {
				if env.Name == "SKIP_CERTIFICATE_VALIDATION" {
					auth.Components[0].Envs[i].Value = "1234"
				}
			}

			client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, auth, tmpCR, client
		},
		"fail - empty proxy host": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource := csmPowerScaleWithAuthCR()
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
			customResource := csmPowerScaleWithAuthCR()
			tmpCR := customResource
			tmpCR.Spec.Driver.CSIDriverType = "unsupported-driver"
			auth := tmpCR.Spec.Modules[0]

			client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, auth, tmpCR, client
		},
		"fail - unsupported auth version": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource := csmPowerScaleWithAuthCR()
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			auth.ConfigVersion = "v100000.0.0"

			client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, auth, tmpCR, client
		},
		"success - auto-add SKIP_CERTIFICATE_VALIDATION when missing": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource := csmPowerScaleWithAuthCR()
			namespace := customResource.Namespace
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]

			// Use v2.3.0 so karavi-authorization-config is still required (driver secret not used)
			auth.ConfigVersion = "v2.3.0"

			// Remove SKIP_CERTIFICATE_VALIDATION from the karavi-authorization-proxy component envs
			for i, comp := range auth.Components {
				if comp.Name == "karavi-authorization-proxy" {
					filtered := []corev1.EnvVar{}
					for _, e := range comp.Envs {
						if e.Name != "SKIP_CERTIFICATE_VALIDATION" {
							filtered = append(filtered, e)
						}
					}
					auth.Components[i].Envs = filtered
					break
				}
			}

			// Ensure PROXY_HOST is not empty (AuthorizationPrecheck requires it)
			for i, e := range auth.Components[0].Envs {
				if e.Name == "PROXY_HOST" && e.Value == "" {
					auth.Components[0].Envs[i].Value = "proxy.example.local"
				}
			}

			// Seed only the required secrets for v2.3.0:
			// - proxy-authz-tokens
			// - karavi-authorization-config
			// DO NOT seed proxy-server-root-certificate so success proves that SKIP_CERTIFICATE_VALIDATION=true was auto-added.
			karaviAuthconfig := getSecret(namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(namespace, "proxy-authz-tokens")
			client := ctrlClientFake.NewClientBuilder().WithObjects(karaviAuthconfig, proxyAuthzTokens).Build()

			// Expect success: auto-added SKIP_CERTIFICATE_VALIDATION=true -> no cert required
			return true, auth, tmpCR, client
		},
		"fail - invalid csm version": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource := csmPowerScaleWithAuthCR()
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			tmpCR.Spec.Version = shared.InvalidCSMVersion
			client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, auth, tmpCR, client
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, auth, tmpCR, client := tc(t)
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
			customResource := CsmAuthorizationCR()
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]

			karaviConfig := getSecret(customResource.Namespace, "karavi-config-secret")
			karaviTLS := getSecret(customResource.Namespace, "karavi-selfsigned-tls")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(karaviConfig, karaviTLS).Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(karaviConfig, karaviTLS).Build()
				return clusterClient, nil
			}

			return true, auth, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"success - version provided": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			tmpCR := CsmAuthorizationCR()
			auth := tmpCR.Spec.Modules[0]
			auth.ConfigVersion = shared.AuthServerConfigVersion
			karaviConfig := getSecret(tmpCR.Namespace, "karavi-config-secret")
			karaviStorage := getSecret(tmpCR.Namespace, "karavi-storage-secret")
			karaviTLS := getSecret(tmpCR.Namespace, "karavi-selfsigned-tls")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(karaviConfig, karaviStorage, karaviTLS).Build()
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(karaviConfig, karaviStorage, karaviTLS).Build()
				return clusterClient, nil
			}

			return true, auth, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"fail - unsupported authorization version": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			tmpCR := CsmAuthorizationCR()
			// AuthorizationServerPrecheck resolves version from the CR, so ensure the CR itself
			// contains an unsupported config version.
			tmpCR.Spec.Version = ""
			tmpCR.Spec.Driver.ConfigVersion = "v100000.0.0"
			tmpCR.Spec.Modules[0].ConfigVersion = "v100000.0.0"
			auth := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, auth, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"fail v2 - karavi-config-secret not found": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			tmpCR := CsmAuthorizationCR()
			// Force secret lookup path by clearing config SecretProviderClass configuration.
			for i := range tmpCR.Spec.Modules[0].Components {
				if tmpCR.Spec.Modules[0].Components[i].Name != AuthConfigSecretComponent {
					continue
				}
				tmpCR.Spec.Modules[0].Components[i].ConfigSecretProviderClass = nil
			}
			auth := tmpCR.Spec.Modules[0]

			karaviTLS := getSecret(tmpCR.Namespace, "karavi-selfsigned-tls")
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(karaviTLS).Build()

			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				clusterClient := ctrlClientFake.NewClientBuilder().WithObjects(karaviTLS).Build()
				return clusterClient, nil
			}

			return false, auth, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"fail - version empty after resolution": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			tmpCR := CsmAuthorizationCR()
			// AuthorizationServerPrecheck resolves version from the CR, so clear version on the CR.
			tmpCR.Spec.Version = ""
			tmpCR.Spec.Driver.ConfigVersion = ""
			tmpCR.Spec.Modules[0].ConfigVersion = ""
			auth := tmpCR.Spec.Modules[0]

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			// This will return false because we expect the check "if auth.ConfigVersion == """ to catch it
			return false, auth, tmpCR, sourceClient, fakeControllerRuntimeClient
		},
		"fail - invalid csm version": func(*testing.T) (bool, csmv1.Module, csmv1.ContainerStorageModule, ctrlClient.Client, fakeControllerRuntimeClientWrapper) {
			customResource := CsmAuthorizationCR()
			tmpCR := customResource
			auth := tmpCR.Spec.Modules[0]
			auth.ConfigVersion = ""
			tmpCR.Spec.Version = shared.InvalidCSMVersion

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			fakeControllerRuntimeClient := func(_ []byte) (ctrlClient.Client, error) {
				return ctrlClientFake.NewClientBuilder().WithObjects().Build(), nil
			}

			return false, auth, tmpCR, sourceClient, fakeControllerRuntimeClient
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

			success, auth, tmpCR, sourceClient, fakeControllerRuntimeClient := tc(t)
			operatorutils.NewControllerRuntimeClientWrapper = fakeControllerRuntimeClient
			operatorutils.NewK8sClientWrapper = func(_ []byte) (*kubernetes.Clientset, error) {
				return nil, nil
			}

			fakeReconcile := operatorutils.FakeReconcileCSM{
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

func TestAuthorizationServerDeployment(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec) {
			tmpCR := CsmAuthorizationCR()

			cm := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind: "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "csm-config-params",
				},
			}

			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cm).Build()

			return true, true, tmpCR, sourceClient, operatorConfig, operatorutils.VersionSpec{}
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec) {
			tmpCR := CsmAuthorizationCR()
			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig, operatorutils.VersionSpec{}
		},
		"success - use default redis kubernetes secret": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec) {
			customResource := CsmAuthorizationCR()

			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, customResource, sourceClient, operatorConfig, operatorutils.VersionSpec{}
		},
		"success - use redis secret provider class": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec) {
			customResource := CsmAuthorizationCR()
			for i := range customResource.Spec.Modules {
				tmp := customResource.Spec.Modules[i]

				if tmp.Name == csmv1.AuthorizationServer {
					tmp.Components = append(tmp.Components, csmv1.ContainerTemplate{
						Name: AuthRedisComponent,
						RedisSecretProviderClass: []csmv1.RedisSecretProviderClass{
							{
								SecretProviderClassName: "secret-provider-class-1",
								RedisSecretName:         "test-secret",
								RedisUsernameKey:        "key",
								RedisPasswordKey:        "key",
							},
						},
					})
					break
				}
			}

			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, customResource, sourceClient, operatorConfig, operatorutils.VersionSpec{}
		},
		"authorization module not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec) {
			customResource := CsmAuthorizationCR()
			tmpCR := customResource
			// Remove authorization
			for i := range tmpCR.Spec.Modules {
				if tmpCR.Spec.Modules[i].Name == csmv1.AuthorizationServer {
					tmpCR.Spec.Modules = nil
				}
			}
			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig, operatorutils.VersionSpec{}
		},
		"success - creating with custom registry": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec) {
			customResource := CsmAuthorizationCR()
			tmpCR := customResource
			// add custom registry
			tmpCR.Spec.CustomRegistry = "quay.io"
			tmpCR.Spec.RetainImageRegistryPath = trueBool

			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig, operatorutils.VersionSpec{}
		},
		"success - creating with config map": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec) {
			customResource := CsmAuthorizationCR()
			tmpCR := customResource
			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			matched := operatorutils.VersionSpec{
				Version: "v1.17.0",
				Images: map[string]string{
					"proxy-service":            "quay.io/dell/container-storage-modules/csm-authorization-proxy:v2.5.0",
					"tenant-service":           "quay.io/dell/container-storage-modules/csm-authorization-tenant:v2.5.0",
					"role-service":             "quay.io/dell/container-storage-modules/csm-authorization-role:v2.5.0",
					"storage-service":          "quay.io/dell/container-storage-modules/csm-authorization-storage:v2.5.0",
					"opa":                      "docker.io/openpolicyagent/opa:0.70.0",
					"opa-kube-mgmt":            "docker.io/openpolicyagent/kube-mgmt:9.2.1",
					"authorization-controller": "quay.io/dell/container-storage-modules/csm-authorization-controller:v2.5.0",
				},
			}

			return true, false, tmpCR, sourceClient, operatorConfig, matched
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op, matched := tc(t)

			err := AuthorizationServerDeployment(context.TODO(), isDeleting, op, cr, sourceClient, matched)
			if success {
				assert.NoError(t, err)
			} else {
				fmt.Println(err)
				assert.Error(t, err)
			}
		})
	}
}

func TestAuthorizationOpenTelemetry(t *testing.T) {
	cr := CsmAuthorizationCR()
	for i := range cr.Spec.Modules {
		if cr.Spec.Modules[i].Name != csmv1.AuthorizationServer {
			continue
		}
		// add otel address
		for j := range cr.Spec.Modules[i].Components {
			if cr.Spec.Modules[i].Components[j].Name == AuthProxyServerComponent {
				cr.Spec.Modules[i].Components[j].OpenTelemetryCollectorAddress = "otel-collector:8889"
			}
		}
	}
	err := certmanagerv1.AddToScheme(scheme.Scheme)
	if err != nil {
		panic(err)
	}
	sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

	err = AuthorizationServerDeployment(context.TODO(), false, operatorConfig, cr, sourceClient, operatorutils.VersionSpec{})
	if err != nil {
		t.Fatal(err)
	}

	storageService := &appsv1.Deployment{}
	err = sourceClient.Get(context.Background(), types.NamespacedName{Name: "storage-service", Namespace: cr.Namespace}, storageService)
	if err != nil {
		t.Fatal(err)
	}

	argFound := false
	for _, container := range storageService.Spec.Template.Spec.Containers {
		if container.Name == "storage-service" {
			for _, arg := range container.Args {
				if strings.Contains(arg, "--collector-address") {
					argFound = true
					if arg != "--collector-address=otel-collector:8889" {
						t.Fatalf("expected --collector-address=otel-collector:8889, got %s", arg)
					}
					break
				}
			}
		}
		if argFound {
			break
		}
	}

	if !argFound {
		t.Fatalf("expected --collector-address=otel-collector:8889, got none")
	}
}

func TestAuthorizationStorageServiceSecretProviderClass(t *testing.T) {
	type checkFn func(*testing.T, ctrlClient.Client, error)

	tests := map[string]func(t *testing.T) (bool, csmv1.ContainerStorageModule, csmv1.Module, ctrlClient.Client, checkFn){
		"mounts added for secret provider classes - vault": func(*testing.T) (bool, csmv1.ContainerStorageModule, csmv1.Module, ctrlClient.Client, checkFn) {
			secretProviderClasses := []string{"secret-provider-class-1", "secret-provider-class-2"}
			customResource := CsmAuthorizationCR()
			namespace := customResource.Namespace
			auth := customResource.Spec.Modules[0]
			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			checkFn := func(t *testing.T, client ctrlClient.Client, err error) {
				if err != nil {
					t.Fatal(err)
				}

				storageService := &appsv1.Deployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "storage-service", Namespace: namespace}, storageService)
				if err != nil {
					t.Fatal(err)
				}

				foundSecretProviderClassVolume := false
				foundSecretProviderClassVolumeMount := false
				for _, spc := range secretProviderClasses {
					for _, volume := range storageService.Spec.Template.Spec.Volumes {
						if volume.Name == fmt.Sprintf("secrets-store-inline-%s", spc) {
							foundSecretProviderClassVolume = true
							if volume.VolumeSource.CSI.VolumeAttributes["secretProviderClass"] != spc {
								t.Fatalf("expected volume.VolumeSource.CSI.VolumeAttributes[\"secretProviderClass\"] to be %s", spc)
							}
						}
					}

					if !foundSecretProviderClassVolume {
						t.Errorf("expected volume for secret provider class %s, wasn't found", fmt.Sprintf("secrets-store-inline-%s", spc))
					}

					for i, container := range storageService.Spec.Template.Spec.Containers {
						if container.Name == "storage-service" {
							for _, volumeMount := range storageService.Spec.Template.Spec.Containers[i].VolumeMounts {
								if volumeMount.Name == fmt.Sprintf("secrets-store-inline-%s", spc) {
									foundSecretProviderClassVolumeMount = true
									if volumeMount.MountPath != fmt.Sprintf("/etc/csm-authorization/%s", spc) {
										t.Fatalf("expected volumeMount.MountPath to be %s", spc)
									}
									if volumeMount.ReadOnly != true {
										t.Fatalf("expected volumeMount.ReadOnly to be true")
									}
								}
							}
							break
						}
					}

					if !foundSecretProviderClassVolumeMount {
						t.Errorf("expected volume mount for secret provider class %s, wasn't found", fmt.Sprintf("secrets-store-inline-%s", spc))
					}
				}
			}
			return false, customResource, auth, sourceClient, checkFn
		},

		"vault configurations ignored for v2.3.0 and above": func(*testing.T) (bool, csmv1.ContainerStorageModule, csmv1.Module, ctrlClient.Client, checkFn) {
			vaultIdentifiers := []string{"vault0"}
			vaultArgs := []string{"--vault=vault0,https://10.0.0.1:8400,csm-authorization,true", "--vault=vault1,https://10.0.0.2:8400,csm-authorization,true"}
			selfSignedVault0Issuer := "storage-service-selfsigned-vault0"
			selfSignedVault0Certificate := "storage-service-selfsigned-vault0"

			customResource := CsmAuthorizationCR()
			namespace := customResource.Namespace
			auth := customResource.Spec.Modules[0]
			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			checkFn := func(t *testing.T, client ctrlClient.Client, err error) {
				if err != nil {
					t.Fatal(err)
				}

				storageService := &appsv1.Deployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "storage-service", Namespace: namespace}, storageService)
				if err != nil {
					t.Fatal(err)
				}

				foundVaultClientVolume := false
				for _, id := range vaultIdentifiers {
					for _, volume := range storageService.Spec.Template.Spec.Volumes {
						if volume.Name == fmt.Sprintf("vault-client-certificate-%s", id) {
							foundVaultClientVolume = true
							break
						}
					}

					if foundVaultClientVolume {
						t.Errorf("expected no volume for %s, but found", fmt.Sprintf("vault-client-certificate-%s", id))
					}
				}

				foundVaultArgs := false
				for _, vaultArg := range vaultArgs {
					for _, c := range storageService.Spec.Template.Spec.Containers {
						if c.Name == "storage-service" {
							for _, arg := range c.Args {
								if arg == vaultArg {
									foundVaultArgs = true
									break
								}
							}
							break
						}
					}

					if foundVaultArgs {
						t.Errorf("expected arg %s to be not found", vaultArg)
					}
				}

				issuer := &certmanagerv1.Issuer{}
				err = client.Get(context.Background(), types.NamespacedName{Name: selfSignedVault0Issuer, Namespace: namespace}, issuer)
				if !apierrors.IsNotFound(err) {
					t.Errorf("expected not found error for issuer %s, but got %v", selfSignedVault0Issuer, err)
				}

				certificate := &certmanagerv1.Certificate{}
				err = client.Get(context.Background(), types.NamespacedName{Name: selfSignedVault0Certificate, Namespace: namespace}, certificate)
				if !apierrors.IsNotFound(err) {
					t.Errorf("expected not found error for certificate %s, but got %v", selfSignedVault0Certificate, err)
				}
			}
			return false, customResource, auth, sourceClient, checkFn
		},

		"mounts added for secret provider classes - conjur": func(*testing.T) (bool, csmv1.ContainerStorageModule, csmv1.Module, ctrlClient.Client, checkFn) {
			secretProviderClasses := []string{"secret-provider-class", "secret-provider-class-2"}
			annotations := "- secrets/usr: secrets/usr\n- secrets/pwd: secrets/pwd\n- secrets/usr2: secrets/usr2\n- secrets/pwd2: secrets/pwd2\n- secrets/usr3: secrets/usr3\n- secrets/pwd3: secrets/pwd3\n- secrets/redis-username: secrets/redis-username\n- secrets/redis-password: secrets/redis-password"

			cr := CsmAuthorizationCR()
			namespace := cr.Namespace
			auth := cr.Spec.Modules[0]
			for i := range cr.Spec.Modules {
				if cr.Spec.Modules[i].Name != csmv1.AuthorizationServer {
					continue
				}
				for j := range cr.Spec.Modules[i].Components {
					if cr.Spec.Modules[i].Components[j].Name == AuthStorageSystemCredentialsComponent {
						// clear vault SPCs
						cr.Spec.Modules[i].Components[j].SecretProviderClasses = nil
						// add conjur SPCs
						cr.Spec.Modules[i].Components[j].SecretProviderClasses = &csmv1.StorageSystemSecretProviderClasses{
							Conjurs: []csmv1.ConjurSecretProviderClass{
								{
									Name: "secret-provider-class",
									Paths: []csmv1.ConjurCredentialPath{
										{
											UsernamePath: "secrets/usr", // #nosec G101 - test file
											PasswordPath: "secrets/pwd", // #nosec G101 - test file
										},
										{
											UsernamePath: "secrets/usr2", // #nosec G101 - test file
											PasswordPath: "secrets/pwd2", // #nosec G101 - test file
										},
									},
								},
								{
									Name: "secret-provider-class-2",
									Paths: []csmv1.ConjurCredentialPath{
										{
											UsernamePath: "secrets/usr3", // #nosec G101 - test file
											PasswordPath: "secrets/pwd3", // #nosec G101 - test file
										},
									},
								},
							},
						}
					}
				}
			}

			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			checkFn := func(t *testing.T, client ctrlClient.Client, err error) {
				if err != nil {
					t.Fatal(err)
				}

				storageService := &appsv1.Deployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "storage-service", Namespace: namespace}, storageService)
				if err != nil {
					t.Fatal(err)
				}

				foundSecretProviderClassVolume := false
				foundSecretProviderClassVolumeMount := false
				for _, spc := range secretProviderClasses {
					for _, volume := range storageService.Spec.Template.Spec.Volumes {
						if volume.Name == fmt.Sprintf("secrets-store-inline-%s", spc) {
							foundSecretProviderClassVolume = true
							if volume.VolumeSource.CSI.VolumeAttributes["secretProviderClass"] != spc {
								t.Fatalf("expected volume.VolumeSource.CSI.VolumeAttributes[\"secretProviderClass\"] to be %s", spc)
							}
						}
					}

					if !foundSecretProviderClassVolume {
						t.Errorf("expected volume for secret provider class %s, wasn't found", fmt.Sprintf("secrets-store-inline-%s", spc))
					}

					for i, container := range storageService.Spec.Template.Spec.Containers {
						if container.Name == "storage-service" {
							for _, volumeMount := range storageService.Spec.Template.Spec.Containers[i].VolumeMounts {
								if volumeMount.Name == fmt.Sprintf("secrets-store-inline-%s", spc) {
									foundSecretProviderClassVolumeMount = true
									if volumeMount.MountPath != fmt.Sprintf("/etc/csm-authorization/%s", spc) {
										t.Fatalf("expected volumeMount.MountPath to be %s", spc)
									}
									if volumeMount.ReadOnly != true {
										t.Fatalf("expected volumeMount.ReadOnly to be true")
									}
								}
							}
							break
						}
					}

					if !foundSecretProviderClassVolumeMount {
						t.Errorf("expected volume mount for secret provider class %s, wasn't found", fmt.Sprintf("secrets-store-inline-%s", spc))
					}
				}

				if storageService.Spec.Template.Annotations["conjur.org/secrets"] != annotations {
					t.Errorf("expected annotations %s, got %s", annotations, storageService.Spec.Template.Annotations["conjur.org/secrets"])
				}
			}
			return false, cr, auth, sourceClient, checkFn
		},

		"annotations added for redis secret provider class - conjur": func(*testing.T) (bool, csmv1.ContainerStorageModule, csmv1.Module, ctrlClient.Client, checkFn) {
			secretProviderClasses := []string{"secret-provider-class", "secret-provider-class-2"}
			annotations := "- secrets/usr: secrets/usr\n- secrets/pwd: secrets/pwd\n- secrets/usr2: secrets/usr2\n- secrets/pwd2: secrets/pwd2\n- secrets/usr3: secrets/usr3\n- secrets/pwd3: secrets/pwd3\n- secrets/redis-username: secrets/redis-username\n- secrets/redis-password: secrets/redis-password"

			cr := CsmAuthorizationCR()
			namespace := cr.Namespace
			auth := cr.Spec.Modules[0]
			for i := range cr.Spec.Modules {
				if cr.Spec.Modules[i].Name != csmv1.AuthorizationServer {
					continue
				}
				for j := range cr.Spec.Modules[i].Components {
					if cr.Spec.Modules[i].Components[j].Name == AuthStorageSystemCredentialsComponent {
						// clear vault SPCs
						cr.Spec.Modules[i].Components[j].SecretProviderClasses = nil
						// add conjur SPCs
						cr.Spec.Modules[i].Components[j].SecretProviderClasses = &csmv1.StorageSystemSecretProviderClasses{
							Conjurs: []csmv1.ConjurSecretProviderClass{
								{
									Name: "secret-provider-class",
									Paths: []csmv1.ConjurCredentialPath{
										{
											UsernamePath: "secrets/usr", // #nosec G101 - test file
											PasswordPath: "secrets/pwd", // #nosec G101 - test file
										},
										{
											UsernamePath: "secrets/usr2", // #nosec G101 - test file
											PasswordPath: "secrets/pwd2", // #nosec G101 - test file
										},
									},
								},
								{
									Name: "secret-provider-class-2",
									Paths: []csmv1.ConjurCredentialPath{
										{
											UsernamePath: "secrets/usr3", // #nosec G101 - test file
											PasswordPath: "secrets/pwd3", // #nosec G101 - test file
										},
									},
								},
							},
						}
					}
				}
			}

			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			checkFn := func(t *testing.T, client ctrlClient.Client, err error) {
				if err != nil {
					t.Fatal(err)
				}

				storageService := &appsv1.Deployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "storage-service", Namespace: namespace}, storageService)
				if err != nil {
					t.Fatal(err)
				}

				foundSecretProviderClassVolume := false
				foundSecretProviderClassVolumeMount := false
				for _, spc := range secretProviderClasses {
					for _, volume := range storageService.Spec.Template.Spec.Volumes {
						if volume.Name == fmt.Sprintf("secrets-store-inline-%s", spc) {
							foundSecretProviderClassVolume = true
							if volume.VolumeSource.CSI.VolumeAttributes["secretProviderClass"] != spc {
								t.Fatalf("expected volume.VolumeSource.CSI.VolumeAttributes[\"secretProviderClass\"] to be %s", spc)
							}
						}
					}

					if !foundSecretProviderClassVolume {
						t.Errorf("expected volume for secret provider class %s, wasn't found", fmt.Sprintf("secrets-store-inline-%s", spc))
					}

					for i, container := range storageService.Spec.Template.Spec.Containers {
						if container.Name == "storage-service" {
							for _, volumeMount := range storageService.Spec.Template.Spec.Containers[i].VolumeMounts {
								if volumeMount.Name == fmt.Sprintf("secrets-store-inline-%s", spc) {
									foundSecretProviderClassVolumeMount = true
									if volumeMount.MountPath != fmt.Sprintf("/etc/csm-authorization/%s", spc) {
										t.Fatalf("expected volumeMount.MountPath to be %s", spc)
									}
									if volumeMount.ReadOnly != true {
										t.Fatalf("expected volumeMount.ReadOnly to be true")
									}
								}
							}
							break
						}
					}

					if !foundSecretProviderClassVolumeMount {
						t.Errorf("expected volume mount for secret provider class %s, wasn't found", fmt.Sprintf("secrets-store-inline-%s", spc))
					}
				}

				if storageService.Spec.Template.Annotations["conjur.org/secrets"] != annotations {
					t.Errorf("expected annotations %s, got %s", annotations, storageService.Spec.Template.Annotations["conjur.org/secrets"])
				}
			}
			return false, cr, auth, sourceClient, checkFn
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			isDeleting, cr, auth, sourceClient, checkFn := tc(t)

			err := authorizationStorageServiceV2(context.TODO(), isDeleting, cr, sourceClient, auth, operatorutils.VersionSpec{})
			checkFn(t, sourceClient, err)
		})
	}
}

func TestAuthorizationStorageServiceSecret(t *testing.T) {
	type checkFn func(*testing.T, ctrlClient.Client, error)

	tests := map[string]func(t *testing.T) (bool, csmv1.ContainerStorageModule, csmv1.Module, ctrlClient.Client, checkFn){
		"mounts added for kubernetes secrets": func(*testing.T) (bool, csmv1.ContainerStorageModule, csmv1.Module, ctrlClient.Client, checkFn) {
			secrets := []string{"secret-1", "secret-2"}

			cr := CsmAuthorizationCR()
			namespace := cr.Namespace
			auth := cr.Spec.Modules[0]
			for i := range cr.Spec.Modules {
				if cr.Spec.Modules[i].Name != csmv1.AuthorizationServer {
					continue
				}
				for j := range cr.Spec.Modules[i].Components {
					if cr.Spec.Modules[i].Components[j].Name == AuthStorageSystemCredentialsComponent {
						// clear vault SPCs
						cr.Spec.Modules[i].Components[j].SecretProviderClasses = nil
						// add secrets
						cr.Spec.Modules[i].Components[j].Secrets = secrets
					}
				}
			}
			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			checkFn := func(t *testing.T, client ctrlClient.Client, err error) {
				if err != nil {
					t.Fatal(err)
				}

				storageService := &appsv1.Deployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "storage-service", Namespace: namespace}, storageService)
				if err != nil {
					t.Fatal(err)
				}

				foundK8sSecretVolume := false
				foundK8sSecretVolumeMount := false
				for _, secret := range secrets {
					for _, volume := range storageService.Spec.Template.Spec.Volumes {
						if volume.Name == fmt.Sprintf("storage-system-secrets-%s", secret) {
							foundK8sSecretVolume = true
							if volume.VolumeSource.Secret.SecretName != secret {
								t.Fatalf("expected volume.VolumeSource.Secret.SecretName to be %s", secret)
							}
						}
					}

					if !foundK8sSecretVolume {
						t.Errorf("expected volume for kubernetes secret %s, wasn't found", fmt.Sprintf("storage-system-secrets-%s", secret))
					}

					for i, container := range storageService.Spec.Template.Spec.Containers {
						if container.Name == "storage-service" {
							for _, volumeMount := range storageService.Spec.Template.Spec.Containers[i].VolumeMounts {
								if volumeMount.Name == fmt.Sprintf("storage-system-secrets-%s", secret) {
									foundK8sSecretVolumeMount = true
									if volumeMount.MountPath != fmt.Sprintf("/etc/csm-authorization/%s", secret) {
										t.Fatalf("expected volumeMount.MountPath to be %s", secret)
									}
									if volumeMount.ReadOnly != true {
										t.Fatalf("expected volumeMount.ReadOnly to be true")
									}
								}
							}
							break
						}
					}

					if !foundK8sSecretVolumeMount {
						t.Errorf("expected volume mount for kubernetes secret %s, wasn't found", fmt.Sprintf("storage-system-secrets-%s", secret))
					}
				}
			}
			return false, cr, auth, sourceClient, checkFn
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			isDeleting, cr, auth, sourceClient, checkFn := tc(t)

			err := authorizationStorageServiceV2(context.TODO(), isDeleting, cr, sourceClient, auth, operatorutils.VersionSpec{})
			checkFn(t, sourceClient, err)
		})
	}
}

func TestAuthorizationStorageServiceSecretAndSecretProviderClass(t *testing.T) {
	type checkFn func(*testing.T, error)

	tests := map[string]func(t *testing.T) (bool, csmv1.ContainerStorageModule, csmv1.Module, ctrlClient.Client, checkFn){
		"not allowed to use secrets AND secret provider classes": func(*testing.T) (bool, csmv1.ContainerStorageModule, csmv1.Module, ctrlClient.Client, checkFn) {
			secrets := []string{"secret-1", "secret-2"}
			cr := CsmAuthorizationCR()
			auth := cr.Spec.Modules[0]
			for i := range cr.Spec.Modules {
				if cr.Spec.Modules[i].Name != csmv1.AuthorizationServer {
					continue
				}
				for j := range cr.Spec.Modules[i].Components {
					if cr.Spec.Modules[i].Components[j].Name == AuthStorageSystemCredentialsComponent {
						// add secrets
						cr.Spec.Modules[i].Components[j].Secrets = secrets
					}
				}
			}

			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			checkFn := func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.EqualError(t, err, "exactly one of SecretProviderClasses or Secrets must be specified in the CSM Authorization CR — not both, not neither")
			}
			return false, cr, auth, sourceClient, checkFn
		},
		"need exactly one of secrets or secret provider classes": func(*testing.T) (bool, csmv1.ContainerStorageModule, csmv1.Module, ctrlClient.Client, checkFn) {
			cr := CsmAuthorizationCR()
			auth := cr.Spec.Modules[0]
			for i := range cr.Spec.Modules {
				if cr.Spec.Modules[i].Name != csmv1.AuthorizationServer {
					continue
				}
				for j := range cr.Spec.Modules[i].Components {
					if cr.Spec.Modules[i].Components[j].Name == AuthStorageSystemCredentialsComponent {
						// clear vault SPCs
						cr.Spec.Modules[i].Components[j].SecretProviderClasses = nil
					}
				}
			}

			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			checkFn := func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.EqualError(t, err, "exactly one of SecretProviderClasses or Secrets must be specified in the CSM Authorization CR — not both, not neither")
			}
			return false, cr, auth, sourceClient, checkFn
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			isDeleting, cr, auth, sourceClient, checkFn := tc(t)

			err := authorizationStorageServiceV2(context.TODO(), isDeleting, cr, sourceClient, auth, operatorutils.VersionSpec{})
			checkFn(t, err)
		})
	}
}

func TestAuthorizationTenantServiceSecretProviderClass(t *testing.T) {
	type checkFn func(*testing.T, ctrlClient.Client, error)

	tests := map[string]func(t *testing.T) (bool, csmv1.ContainerStorageModule, csmv1.Module, ctrlClient.Client, checkFn){
		"mount and annotations added for config secret provider class - conjur": func(*testing.T) (bool, csmv1.ContainerStorageModule, csmv1.Module, ctrlClient.Client, checkFn) {
			secretProviderClasses := []string{"secret-provider-class"}
			annotations := "- secrets/redis-username: secrets/redis-username\n- secrets/redis-password: secrets/redis-password\n- secrets/config-object: secrets/config-object"

			cr := CsmAuthorizationCR()
			namespace := cr.Namespace
			auth := cr.Spec.Modules[0]
			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			checkFn := func(t *testing.T, client ctrlClient.Client, err error) {
				if err != nil {
					t.Fatal(err)
				}

				tenantService := &appsv1.Deployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "tenant-service", Namespace: namespace}, tenantService)
				if err != nil {
					t.Fatal(err)
				}

				foundSecretProviderClassVolume := false
				foundSecretProviderClassVolumeMount := false
				for _, spc := range secretProviderClasses {
					for _, volume := range tenantService.Spec.Template.Spec.Volumes {
						if volume.Name == fmt.Sprintf("secrets-store-inline-%s", spc) {
							foundSecretProviderClassVolume = true
							if volume.VolumeSource.CSI.VolumeAttributes["secretProviderClass"] != spc {
								t.Fatalf("expected volume.VolumeSource.CSI.VolumeAttributes[\"secretProviderClass\"] to be %s", spc)
							}
						}
					}

					if !foundSecretProviderClassVolume {
						t.Errorf("expected volume for secret provider class %s, wasn't found", fmt.Sprintf("secrets-store-inline-%s", spc))
					}

					for i, container := range tenantService.Spec.Template.Spec.Containers {
						if container.Name == "tenant-service" {
							for _, volumeMount := range tenantService.Spec.Template.Spec.Containers[i].VolumeMounts {
								if volumeMount.Name == fmt.Sprintf("secrets-store-inline-%s", spc) {
									foundSecretProviderClassVolumeMount = true
									if volumeMount.MountPath != fmt.Sprintf("/etc/csm-authorization/%s", spc) {
										t.Fatalf("expected volumeMount.MountPath to be %s", spc)
									}
									if volumeMount.ReadOnly != true {
										t.Fatalf("expected volumeMount.ReadOnly to be true")
									}
								}
							}
							break
						}
					}

					if !foundSecretProviderClassVolumeMount {
						t.Errorf("expected volume mount for secret provider class %s, wasn't found", fmt.Sprintf("secrets-store-inline-%s", spc))
					}
				}

				if tenantService.Spec.Template.Annotations["conjur.org/secrets"] != annotations {
					t.Errorf("expected annotations %s, got %s", annotations, tenantService.Spec.Template.Annotations["conjur.org/secrets"])
				}
			}
			return false, cr, auth, sourceClient, checkFn
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			isDeleting, cr, auth, sourceClient, checkFn := tc(t)

			err := applyDeleteAuthorizationTenantServiceV2(context.TODO(), isDeleting, cr, sourceClient, auth, operatorutils.VersionSpec{})
			checkFn(t, sourceClient, err)
		})
	}
}

func TestAuthorizationIngress(t *testing.T) {
	isOpenShift := true
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client) {
			tmpCR := CsmAuthorizationCR()
			tmpCR.Namespace = "authorization"

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

			return true, true, tmpCR, sourceClient
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client) {
			tmpCR := CsmAuthorizationCR()
			tmpCR.Namespace = "authorization"
			namespace := tmpCR.Namespace
			name := namespace + "-ingress-nginx-controller"

			dp := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind: "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app.kubernetes.io/name": "ingress-nginx"},
					},
				},
			}

			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind: "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(dp, pod).Build()

			return true, true, tmpCR, sourceClient
		},
		"success - creating with certs": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource := CsmAuthorizationCR()
			tmpCR := customResource
			namespace := tmpCR.Namespace
			name := namespace + "-ingress-nginx-controller"
			for i := range tmpCR.Spec.Modules {
				if tmpCR.Spec.Modules[i].Name != csmv1.AuthorizationServer {
					continue
				}
				for j := range tmpCR.Spec.Modules[i].Components {
					if tmpCR.Spec.Modules[i].Components[j].Name == AuthProxyServerComponent {
						tmpCR.Spec.Modules[i].Components[j].Certificate = "fake-cert"
						tmpCR.Spec.Modules[i].Components[j].PrivateKey = "fake-key"
					}
				}
			}

			dp := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind: "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app.kubernetes.io/name": "ingress-nginx"},
					},
				},
			}

			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind: "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(dp, pod).Build()

			return true, true, tmpCR, sourceClient
		},
		"success - creating with openshift and other annotations": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource := CsmAuthorizationCR()

			tmpCR := customResource
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, true, tmpCR, sourceClient
		},
		"authorization module not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client) {
			customResource := CsmAuthorizationCR()
			tmpCR := customResource
			// Remove authorization
			for i := range tmpCR.Spec.Modules {
				if tmpCR.Spec.Modules[i].Name == csmv1.AuthorizationServer {
					tmpCR.Spec.Modules = nil
				}
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient
		},
		"fail - NGINX ingress creation failure": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client) {
			tmpCR := CsmAuthorizationCR()
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			isOpenShift = false

			return false, false, tmpCR, sourceClient
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient := tc(t)
			// Reset isOpenShift to true by default unless test case sets it
			if name != "fail - NGINX ingress creation failure" {
				isOpenShift = true
			}

			// Register Gateway API scheme for tests
			_ = gatewayv1.AddToScheme(scheme.Scheme)

			fakeReconcile := operatorutils.FakeReconcileCSM{
				Client:    sourceClient,
				K8sClient: fake.NewSimpleClientset(),
			}
			err := AuthorizationIngress(context.TODO(), isDeleting, isOpenShift, cr, &fakeReconcile, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestInstallPolicies(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR := CsmAuthorizationCR()

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
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR := CsmAuthorizationCR()

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},

		"authorization module not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource := CsmAuthorizationCR()
			tmpCR := customResource
			// Remove authorization
			for i := range tmpCR.Spec.Modules {
				if tmpCR.Spec.Modules[i].Name == csmv1.AuthorizationServer {
					tmpCR.Spec.Modules = nil
				}
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
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

func TestNginxIngressControllerCleanup(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - cleanup existing nginx ingress controller": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR := CsmAuthorizationCR()
			// Use v2.5.0 for upgrade scenario tests
			for i := range tmpCR.Spec.Modules {
				if tmpCR.Spec.Modules[i].Name == csmv1.AuthorizationServer {
					tmpCR.Spec.Modules[i].ConfigVersion = "v2.5.0"
				}
			}

			// Create existing nginx ingress controller deployment that should be cleaned up
			deployment := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind: "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      tmpCR.Namespace + "-ingress-nginx-controller",
					Namespace: tmpCR.Namespace,
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(deployment).Build()
			return true, tmpCR, sourceClient, operatorConfig
		},
		"success - cleanup when nginx ingress controller doesn't exist": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR := CsmAuthorizationCR()
			// Use v2.5.0 for upgrade scenario tests
			for i := range tmpCR.Spec.Modules {
				if tmpCR.Spec.Modules[i].Name == csmv1.AuthorizationServer {
					tmpCR.Spec.Modules[i].ConfigVersion = "v2.5.0"
				}
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, tmpCR, sourceClient, operatorConfig
		},
		"authorization module not found": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource := CsmAuthorizationCR()
			tmpCR := customResource
			// Remove authorization
			for i := range tmpCR.Spec.Modules {
				if tmpCR.Spec.Modules[i].Name == csmv1.AuthorizationServer {
					tmpCR.Spec.Modules = nil
				}
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, tmpCR, sourceClient, operatorConfig // Cleanup succeeds even when module not found
		},
		"success - cleanup with file read error": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR := CsmAuthorizationCR()
			// Use v2.5.0 for upgrade scenario tests
			for i := range tmpCR.Spec.Modules {
				if tmpCR.Spec.Modules[i].Name == csmv1.AuthorizationServer {
					tmpCR.Spec.Modules[i].ConfigVersion = "v2.5.0"
				}
			}

			// Create operator config with invalid directory to trigger file read error
			invalidOp := operatorutils.OperatorConfig{
				ConfigDirectory: "/invalid/directory",
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, tmpCR, sourceClient, invalidOp
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			success, cr, sourceClient, op := test(t)
			err := NginxIngressControllerCleanup(context.TODO(), op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestNginxIngressController(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR := CsmAuthorizationCR()
			// Use v2.4.0 for nginx ingress controller tests (v2.5.0+ uses Gateway API)
			for i := range tmpCR.Spec.Modules {
				if tmpCR.Spec.Modules[i].Name == csmv1.AuthorizationServer {
					tmpCR.Spec.Modules[i].ConfigVersion = "v2.4.0"
				}
			}

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
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR := CsmAuthorizationCR()
			// Use v2.4.0 for nginx ingress controller tests (v2.5.0+ uses Gateway API)
			for i := range tmpCR.Spec.Modules {
				if tmpCR.Spec.Modules[i].Name == csmv1.AuthorizationServer {
					tmpCR.Spec.Modules[i].ConfigVersion = "v2.4.0"
				}
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},

		"authorization module not found": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource := CsmAuthorizationCR()
			tmpCR := customResource
			// Remove authorization
			for i := range tmpCR.Spec.Modules {
				if tmpCR.Spec.Modules[i].Name == csmv1.AuthorizationServer {
					tmpCR.Spec.Modules = nil
				}
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op := tc(t)

			// Register Gateway API scheme for tests
			_ = gatewayv1.AddToScheme(scheme.Scheme)

			err := NginxIngressController(context.TODO(), isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestAuthorizationCertificates(t *testing.T) {
	// Generate RSA key
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Error(err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(2048),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}

	// Create Certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Error(err)
	}

	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - using self-signed certificate": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR := CsmAuthorizationCR()
			tmpCR.Namespace = "authorization"
			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - using custom tls secret": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource := CsmAuthorizationCR()
			tmpCR := customResource
			for i := range tmpCR.Spec.Modules {
				if tmpCR.Spec.Modules[i].Name != csmv1.AuthorizationServer {
					continue
				}
				for j := range tmpCR.Spec.Modules[i].Components {
					if tmpCR.Spec.Modules[i].Components[j].Name == AuthProxyServerComponent {
						tmpCR.Spec.Modules[i].Components[j].Certificate = base64.StdEncoding.EncodeToString(certBytes)
						tmpCR.Spec.Modules[i].Components[j].PrivateKey = base64.StdEncoding.EncodeToString(x509.MarshalPKCS1PrivateKey(priv))
					}
				}
			}
			err := certmanagerv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},

		"fail - using partial custom cert": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource := CsmAuthorizationCR()
			tmpCR := customResource
			for i := range tmpCR.Spec.Modules {
				if tmpCR.Spec.Modules[i].Name != csmv1.AuthorizationServer {
					continue
				}
				for j := range tmpCR.Spec.Modules[i].Components {
					if tmpCR.Spec.Modules[i].Components[j].Name == AuthProxyServerComponent {
						tmpCR.Spec.Modules[i].Components[j].Certificate = "fake-cert"
					}
				}
			}
			err := certmanagerv1.AddToScheme(scheme.Scheme)
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

			err := InstallWithCerts(context.TODO(), isDeleting, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestAuthorizationCrdDeploy(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR := CsmAuthorizationCR()

			cr := &apiextv1.CustomResourceDefinition{
				TypeMeta: metav1.TypeMeta{
					Kind: "CustomResourceDefinition",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "csmroles.csm-authorization.storage.dell.com",
				},
			}
			err := apiextv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr).Build()
			return true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR := CsmAuthorizationCR()

			err := apiextv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, tmpCR, sourceClient, operatorConfig
		},
		"fail - auth deployment file bad yaml": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR := CsmAuthorizationCR()
			badOperatorConfig.ConfigDirectory = "./testdata/badYaml"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, tmpCR, sourceClient, badOperatorConfig
		},
		"fail - auth config file not found": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			tmpCR := CsmAuthorizationCR()
			badOperatorConfig.ConfigDirectory = "invalid-dir"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, tmpCR, sourceClient, badOperatorConfig
		},
		"authorization module not found": func(*testing.T) (bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource := CsmAuthorizationCR()
			tmpCR := customResource
			// Remove authorization
			for i := range tmpCR.Spec.Modules {
				if tmpCR.Spec.Modules[i].Name == csmv1.AuthorizationServer {
					tmpCR.Spec.Modules = nil
				}
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return false, tmpCR, sourceClient, operatorConfig
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, cr, sourceClient, op := tc(t)

			err := AuthCrdDeploy(ctx, op, cr, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestGetRedisChecksumFromSecretData(t *testing.T) {
	ctx := context.TODO()
	namespace := "default"
	secretName := "redis-secret"

	redisSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"password": []byte("test"),
			"username": []byte("test"),
		},
	}

	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(redisSecret).Build()
	cr := csmv1.ContainerStorageModule{}
	cr.Namespace = namespace

	checksum, err := getRedisChecksumFromSecretData(ctx, fakeClient, cr, secretName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checksum == "" {
		t.Fatal("expected checksum, got empty string")
	}
}

func TestMountSPCVolume(t *testing.T) {
	t.Run("it mounts redis volumes", func(t *testing.T) {
		spcName := "array-creds"
		expectedVolumeName := fmt.Sprintf("secrets-store-inline-%s", spcName)
		expectedMountPath := fmt.Sprintf("/etc/csm-authorization/%s", spcName)

		spec := &corev1.PodSpec{
			Volumes:    []corev1.Volume{},
			Containers: []corev1.Container{{}},
		}

		mountSPCVolume(spec, spcName)

		foundVolume := false
		for _, v := range spec.Volumes {
			if v.Name == expectedVolumeName {
				foundVolume = true
				if v.VolumeSource.CSI == nil || v.VolumeSource.CSI.Driver != "secrets-store.csi.k8s.io" {
					t.Errorf("unexpected CSI driver or nil CSI source")
				}
				if v.VolumeSource.CSI.VolumeAttributes["secretProviderClass"] != spcName {
					t.Errorf("expected secretProviderClass %s, got %s", spcName, v.VolumeSource.CSI.VolumeAttributes["secretProviderClass"])
				}
			}
		}
		if !foundVolume {
			t.Errorf("expected volume %s not found", expectedVolumeName)
		}

		foundMount := false
		for _, m := range spec.Containers[0].VolumeMounts {
			if m.Name == expectedVolumeName && m.MountPath == expectedMountPath && m.ReadOnly {
				foundMount = true
			}
		}
		if !foundMount {
			t.Errorf("expected volume mount %s at path %s not found", expectedVolumeName, expectedMountPath)
		}
	})

	t.Run("it doesn't duplicate volumes or mounts", func(t *testing.T) {
		spcName := "array-creds"
		volumeName := fmt.Sprintf("secrets-store-inline-%s", spcName)
		mountPath := fmt.Sprintf("/etc/csm-authorization/%s", spcName)

		readOnly := true
		spec := &corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: volumeName,
					VolumeSource: corev1.VolumeSource{
						CSI: &corev1.CSIVolumeSource{
							Driver:   "secrets-store.csi.k8s.io",
							ReadOnly: &readOnly,
							VolumeAttributes: map[string]string{
								"secretProviderClass": spcName,
							},
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      volumeName,
							MountPath: mountPath,
							ReadOnly:  true,
						},
					},
				},
			},
		}

		mountSPCVolume(spec, spcName)

		if len(spec.Volumes) != 1 {
			t.Errorf("expected 1 volume, got %d", len(spec.Volumes))
		}
		if len(spec.Containers[0].VolumeMounts) != 1 {
			t.Errorf("expected 1 volume mount, got %d", len(spec.Containers[0].VolumeMounts))
		}
	})
}

func TestUpdateConjurAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		conjurPaths []string
		want        map[string]string
	}{
		{
			name:        "empty annotations, add multiple paths",
			annotations: map[string]string{},
			conjurPaths: []string{"secrets/redis-username", "secrets/redis-password"},
			want: map[string]string{
				"conjur.org/secrets": "- secrets/redis-username: secrets/redis-username\n- secrets/redis-password: secrets/redis-password",
			},
		},
		{
			name:        "empty annotations, add single path",
			annotations: map[string]string{},
			conjurPaths: []string{"secrets/config-object"},
			want: map[string]string{ // #nosec G101
				"conjur.org/secrets": "- secrets/config-object: secrets/config-object",
			},
		},
		{
			name:        "empty annotations, no path",
			annotations: map[string]string{},
			conjurPaths: []string{},
			want:        map[string]string{},
		},
		{
			name:        "empty annotations, multiple path with empty value",
			annotations: map[string]string{},
			conjurPaths: []string{"secrets/config-object", ""},
			want:        map[string]string{},
		},
		{
			name: "non-empty annotations",
			annotations: map[string]string{
				"otherAnnotation": "otherValue",
			},
			conjurPaths: []string{"secrets/redis-username", "secrets/redis-password"},
			want: map[string]string{
				"conjur.org/secrets": "- secrets/redis-username: secrets/redis-username\n- secrets/redis-password: secrets/redis-password",
				"otherAnnotation":    "otherValue",
			},
		},
		{
			name: "update existing annotation",
			annotations: map[string]string{
				"conjur.org/secrets": "- secrets/system-username: secrets/system-username\n- secrets/system-password: secrets/system-password",
				"otherAnnotation":    "otherValue",
			},
			conjurPaths: []string{"secrets/redis-username", "secrets/redis-password"},
			want: map[string]string{
				"conjur.org/secrets": "- secrets/system-username: secrets/system-username\n- secrets/system-password: secrets/system-password\n- secrets/redis-username: secrets/redis-username\n- secrets/redis-password: secrets/redis-password",
				"otherAnnotation":    "otherValue",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateConjurAnnotations(tt.annotations, tt.conjurPaths...)
			if !reflect.DeepEqual(tt.annotations, tt.want) {
				t.Errorf("updateConjurAnnotations() = %v, want %v", tt.annotations, tt.want)
			}
		})
	}
}

func TestUpdateRedisGlobalVars(t *testing.T) {
	type args struct {
		component csmv1.ContainerTemplate
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "empty fields",
			args: args{
				component: csmv1.ContainerTemplate{
					RedisSecretProviderClass: []csmv1.RedisSecretProviderClass{},
				},
			},
			want: map[string]string{
				"redisSecretProviderClassName": "",
				"redisSecretName":              defaultRedisSecretName,
				"redisUsernameKey":             defaultRedisUsernameKey,
				"redisPasswordKey":             defaultRedisPasswordKey,
				"redisConjurUsernamePath":      "",
				"redisConjurPasswordPath":      "",
			},
		},
		{
			name: "class name but no secret name",
			args: args{
				component: csmv1.ContainerTemplate{
					RedisSecretProviderClass: []csmv1.RedisSecretProviderClass{
						{ // #nosec G101
							SecretProviderClassName: "ut-provider-class",
							RedisSecretName:         "",
							RedisUsernameKey:        "",
							RedisPasswordKey:        "",
						},
					},
				},
			},
			want: map[string]string{
				"redisSecretProviderClassName": "",
				"redisSecretName":              defaultRedisSecretName,
				"redisUsernameKey":             defaultRedisUsernameKey,
				"redisPasswordKey":             defaultRedisPasswordKey,
				"redisConjurUsernamePath":      "",
				"redisConjurPasswordPath":      "",
			},
		},
		{
			name: "all fields present",
			args: args{
				component: csmv1.ContainerTemplate{
					RedisSecretProviderClass: []csmv1.RedisSecretProviderClass{
						{ // #nosec G101
							SecretProviderClassName: "ut-provider-class",
							RedisSecretName:         "ut-secret-name",
							RedisUsernameKey:        "ut-username-key",
							RedisPasswordKey:        "ut-password-key",
							Conjur: &csmv1.ConjurCredentialPath{
								UsernamePath: "ut-username-path",
								PasswordPath: "ut-password-path",
							},
						},
					},
				},
			},
			want: map[string]string{ // #nosec G101
				"redisSecretProviderClassName": "ut-provider-class",
				"redisSecretName":              "ut-secret-name",
				"redisUsernameKey":             "ut-username-key",
				"redisPasswordKey":             "ut-password-key",
				"redisConjurUsernamePath":      "ut-username-path",
				"redisConjurPasswordPath":      "ut-password-path",
			}, // #nosec G101
		},
		{
			name: "conjur present but no values",
			args: args{
				component: csmv1.ContainerTemplate{
					RedisSecretProviderClass: []csmv1.RedisSecretProviderClass{
						{ // #nosec G101
							SecretProviderClassName: "",
							RedisSecretName:         "",
							RedisUsernameKey:        "",
							RedisPasswordKey:        "",
							Conjur: &csmv1.ConjurCredentialPath{
								UsernamePath: "",
								PasswordPath: "",
							},
						},
					},
				},
			},
			want: map[string]string{
				"redisSecretProviderClassName": "",
				"redisSecretName":              defaultRedisSecretName,
				"redisUsernameKey":             defaultRedisUsernameKey,
				"redisPasswordKey":             defaultRedisPasswordKey,
			},
		},
		{
			name: "conjur present with values",
			args: args{
				component: csmv1.ContainerTemplate{
					RedisSecretProviderClass: []csmv1.RedisSecretProviderClass{
						{ // #nosec G101
							SecretProviderClassName: "",
							RedisSecretName:         "",
							RedisUsernameKey:        "",
							RedisPasswordKey:        "",
							Conjur: &csmv1.ConjurCredentialPath{ // #nosec G101
								UsernamePath: "conjur.org/username",
								PasswordPath: "conjur.org/password",
							},
						},
					},
				},
			},
			want: map[string]string{ // #nosec G101
				"redisSecretProviderClassName": "",
				"redisSecretName":              defaultRedisSecretName,
				"redisUsernameKey":             defaultRedisUsernameKey,
				"redisPasswordKey":             defaultRedisPasswordKey,
				"redisConjurUsernamePath":      "conjur.org/username",
				"redisConjurPasswordPath":      "conjur.org/password",
			},
		},
		{
			name: "class name but no secret name",
			args: args{
				component: csmv1.ContainerTemplate{
					RedisSecretProviderClass: []csmv1.RedisSecretProviderClass{
						{ // #nosec G101
							SecretProviderClassName: "ut-provider-class",
							RedisSecretName:         "",
							RedisUsernameKey:        "ut-username-key",
							RedisPasswordKey:        "ut-password-key",
						},
					},
				},
			},
			want: map[string]string{
				"redisSecretProviderClassName": "",
				"redisSecretName":              defaultRedisSecretName,
				"redisUsernameKey":             defaultRedisUsernameKey,
				"redisPasswordKey":             defaultRedisPasswordKey,
				"redisConjurUsernamePath":      "",
				"redisConjurPasswordPath":      "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisSecretProviderClassName = ""
			redisSecretName = ""
			redisUsernameKey = ""
			redisPasswordKey = ""
			redisConjurUsernamePath = ""
			redisConjurPasswordPath = ""

			updateRedisGlobalVars(tt.args.component)
			assert.Equal(t, tt.want["redisSecretProviderClassName"], redisSecretProviderClassName)
			assert.Equal(t, tt.want["redisSecretName"], redisSecretName)
			assert.Equal(t, tt.want["redisUsernameKey"], redisUsernameKey)
			assert.Equal(t, tt.want["redisPasswordKey"], redisPasswordKey)
			assert.Equal(t, tt.want["redisConjurUsernamePath"], redisConjurUsernamePath)
			assert.Equal(t, tt.want["redisConjurPasswordPath"], redisConjurPasswordPath)
		})
	}
}

func TestUpdateConfigGlobalVars(t *testing.T) {
	type args struct {
		component csmv1.ContainerTemplate
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "empty fields",
			args: args{
				component: csmv1.ContainerTemplate{
					ConfigSecretProviderClass: []csmv1.ConfigSecretProviderClass{},
				},
			},
			want: map[string]string{ // #nosec G101
				"configSecretProviderClassName ": "",
				"configSecretName":               "karavi-config-secret",
				"configSecretPath ":              "",
			}, // #nosec G101
		},
		{
			name: "class name but no secret name",
			args: args{
				component: csmv1.ContainerTemplate{
					ConfigSecretProviderClass: []csmv1.ConfigSecretProviderClass{
						{ // #nosec G101
							SecretProviderClassName: "ut-provider-class",
							ConfigSecretName:        "",
						},
					},
				},
			},
			want: map[string]string{ // #nosec G101
				"configSecretProviderClassName ": "",
				"configSecretName":               "karavi-config-secret",
				"configSecretPath":               "",
			}, // #nosec G101
		},
		{
			name: "all fields present",
			args: args{
				component: csmv1.ContainerTemplate{
					ConfigSecretProviderClass: []csmv1.ConfigSecretProviderClass{
						{ // #nosec G101
							SecretProviderClassName: "ut-provider-class",
							ConfigSecretName:        "ut-secret-name",
							Conjur: &csmv1.ConjurConfigPath{
								SecretPath: "ut-secret-path",
							},
						},
					},
				},
			},
			want: map[string]string{ // #nosec G101
				"configSecretProviderClassName ": "ut-provider-class",
				"configSecretName":               "ut-secret-name",
				"configSecretPath":               "ut-secret-path",
			}, // #nosec G101
		},
		{
			name: "conjur present but no values",
			args: args{
				component: csmv1.ContainerTemplate{
					ConfigSecretProviderClass: []csmv1.ConfigSecretProviderClass{
						{ // #nosec G101
							SecretProviderClassName: "",
							ConfigSecretName:        "",
							Conjur: &csmv1.ConjurConfigPath{
								SecretPath: "",
							},
						},
					},
				},
			},
			want: map[string]string{ // #nosec G101
				"configSecretProviderClassName ": "",
				"configSecretName":               "karavi-config-secret",
				"configSecretPath":               "",
			}, // #nosec G101
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configSecretProviderClassName = ""
			configSecretName = ""
			configSecretPath = ""

			updateConfigGlobalVars(tt.args.component)
			assert.Equal(t, tt.want["configSecretProviderClassName "], configSecretProviderClassName)
			assert.Equal(t, tt.want["configSecretName"], configSecretName)
			assert.Equal(t, tt.want["configSecretPath"], configSecretPath)
		})
	}
}

func TestRemoveVaultFromStorageService(t *testing.T) {
	ctx := context.TODO()
	err := certmanagerv1.AddToScheme(scheme.Scheme)
	if err != nil {
		panic(err)
	}

	// Simulate an upgrade from v2.2.0 -> v2.3.0 where the old CR still has the deprecated
	// "vault" component present (backwards compatibility cleanup path).
	cr := CsmAuthorizationCR()
	cr.Name = "test-csm"
	cr.Namespace = "test-namespace"
	for i := range cr.Spec.Modules {
		if cr.Spec.Modules[i].Name != csmv1.AuthorizationServer {
			continue
		}
		cr.Spec.Modules[i].ConfigVersion = "v2.3.0"
		cr.Spec.Modules[i].Components = append(cr.Spec.Modules[i].Components, csmv1.ContainerTemplate{
			Name: AuthVaultComponent,
			Vaults: []csmv1.Vault{
				{
					Identifier:           "vault1",
					CertificateAuthority: base64.StdEncoding.EncodeToString([]byte("ca")),
					ClientCertificate:    base64.StdEncoding.EncodeToString([]byte("cert")),
					ClientKey:            base64.StdEncoding.EncodeToString([]byte("key")),
				},
				{
					Identifier: "vault2",
				},
			},
		})
	}

	caSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vault-certificate-authority-vault1",
			Namespace: cr.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"ca.crt": []byte("ca")},
	}
	clientSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vault-client-certificate-vault1",
			Namespace: cr.Namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{"tls.crt": []byte("cert"), "tls.key": []byte("key")},
	}
	issuer := &certmanagerv1.Issuer{
		TypeMeta: metav1.TypeMeta{APIVersion: "cert-manager.io/v1", Kind: "Issuer"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "storage-service-selfsigned-vault2",
			Namespace: cr.Namespace,
		},
	}
	certificate := &certmanagerv1.Certificate{
		TypeMeta: metav1.TypeMeta{APIVersion: "cert-manager.io/v1", Kind: "Certificate"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "storage-service-selfsigned-vault2",
			Namespace: cr.Namespace,
		},
	}

	// Create a test Deployment (represents an existing v2.2.0-era storage-service deployment)
	dp := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "storage-service",
			Namespace: "test-namespace",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "storage-service",
							Args: []string{"--vault=vault1,https://10.0.0.1:8400,csm-authorization,true", "--vault=vault2,https://10.0.0.2:8400,csm-authorization,true"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "vault-client-certificate-vault1",
									MountPath: "/etc/vault/vault1",
								},
								{
									Name:      "vault-client-certificate-vault2",
									MountPath: "/etc/vault/vault2",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "vault-client-certificate-vault1",
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									Sources: []corev1.VolumeProjection{
										{
											Secret: &corev1.SecretProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "vault-certificate-authority-vault1",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	ctrlClient := ctrlClientFake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(&dp, caSecret, clientSecret, issuer, certificate).Build()

	err = removeVaultFromStorageService(ctx, cr, ctrlClient, dp)
	if err != nil {
		t.Errorf("Expected nil error, but got %v", err)
	}

	err = ctrlClient.Get(ctx, client.ObjectKey{Name: caSecret.Name, Namespace: caSecret.Namespace}, &corev1.Secret{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected %s to be deleted, got %v", caSecret.Name, err)
	}
	err = ctrlClient.Get(ctx, client.ObjectKey{Name: clientSecret.Name, Namespace: clientSecret.Namespace}, &corev1.Secret{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected %s to be deleted, got %v", clientSecret.Name, err)
	}
	err = ctrlClient.Get(ctx, client.ObjectKey{Name: issuer.Name, Namespace: issuer.Namespace}, &certmanagerv1.Issuer{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected %s to be deleted, got %v", issuer.Name, err)
	}
	err = ctrlClient.Get(ctx, client.ObjectKey{Name: certificate.Name, Namespace: certificate.Namespace}, &certmanagerv1.Certificate{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected %s to be deleted, got %v", certificate.Name, err)
	}

	// Check if the vault certificates, args, and volume mounts are removed
	updatedDp := appsv1.Deployment{}
	err = ctrlClient.Get(ctx, client.ObjectKey{
		Namespace: dp.Namespace,
		Name:      dp.Name,
	}, &updatedDp)
	if err != nil {
		t.Errorf("Expected to get the updated deployment, but got %v", err)
	}

	// Check if the vault args are removed
	for _, container := range updatedDp.Spec.Template.Spec.Containers {
		if container.Name == "storage-service" {
			for _, arg := range container.Args {
				if strings.HasPrefix(arg, "--vault=") {
					t.Errorf("Expected vault args to be removed, but found %s", arg)
				}
			}
		}
	}

	// Check if the vault volume mounts are removed
	for _, container := range updatedDp.Spec.Template.Spec.Containers {
		if container.Name == "storage-service" {
			for _, volumeMount := range container.VolumeMounts {
				if strings.Contains(volumeMount.MountPath, "/etc/vault/") {
					t.Errorf("Expected vault volume mounts to be removed, but found %s", volumeMount.MountPath)
				}
			}
		}
	}

	// Check if the vault volumes are removed
	for _, volume := range updatedDp.Spec.Template.Spec.Volumes {
		if strings.Contains(volume.Name, "vault-client-certificate-") {
			t.Errorf("Expected vault volumes to be removed, but found %s", volume.Name)
		}
	}

	_ = ctrlClient.Delete(ctx, &dp)
}

func TestApplyDeleteVaultCertificates(t *testing.T) {
	err := certmanagerv1.AddToScheme(scheme.Scheme)
	if err != nil {
		panic(err)
	}

	tests := map[string]struct {
		expectErr bool
		buildCR   func() csmv1.ContainerStorageModule
	}{
		"invalid vault CA": {
			expectErr: true,
			buildCR: func() csmv1.ContainerStorageModule {
				cr := CsmAuthorizationCR()
				for i := range cr.Spec.Modules {
					if cr.Spec.Modules[i].Name != csmv1.AuthorizationServer {
						continue
					}
					cr.Spec.Modules[i].Components = append(cr.Spec.Modules[i].Components, csmv1.ContainerTemplate{
						Name: AuthVaultComponent,
						Vaults: []csmv1.Vault{{
							Identifier:           "vault-bad-ca",
							CertificateAuthority: "not-base64!!!",
						}},
					})
				}
				return cr
			},
		},
		"invalid vault client cert": {
			expectErr: true,
			buildCR: func() csmv1.ContainerStorageModule {
				cr := CsmAuthorizationCR()
				for i := range cr.Spec.Modules {
					if cr.Spec.Modules[i].Name != csmv1.AuthorizationServer {
						continue
					}
					cr.Spec.Modules[i].Components = append(cr.Spec.Modules[i].Components, csmv1.ContainerTemplate{
						Name: AuthVaultComponent,
						Vaults: []csmv1.Vault{{
							Identifier:        "vault-bad-cert",
							ClientCertificate: "not-base64!!!",
							ClientKey:         base64.StdEncoding.EncodeToString([]byte("key")),
						}},
					})
				}
				return cr
			},
		},
		"invalid vault client key": {
			expectErr: true,
			buildCR: func() csmv1.ContainerStorageModule {
				cr := CsmAuthorizationCR()
				for i := range cr.Spec.Modules {
					if cr.Spec.Modules[i].Name != csmv1.AuthorizationServer {
						continue
					}
					cr.Spec.Modules[i].Components = append(cr.Spec.Modules[i].Components, csmv1.ContainerTemplate{
						Name: AuthVaultComponent,
						Vaults: []csmv1.Vault{{
							Identifier:        "vault-bad-key",
							ClientCertificate: base64.StdEncoding.EncodeToString([]byte("cert")),
							ClientKey:         "not-base64!!!",
						}},
					})
				}
				return cr
			},
		},
		"success - valid vault certificate data": {
			expectErr: false,
			buildCR: func() csmv1.ContainerStorageModule {
				cr := CsmAuthorizationCR()
				for i := range cr.Spec.Modules {
					cr.Spec.Modules[i].Components = append(cr.Spec.Modules[i].Components, csmv1.ContainerTemplate{
						Name: AuthVaultComponent,
						Vaults: []csmv1.Vault{{
							Identifier:           "vault-good",
							CertificateAuthority: base64.StdEncoding.EncodeToString([]byte("ca")),
							ClientCertificate:    base64.StdEncoding.EncodeToString([]byte("cert")),
							ClientKey:            base64.StdEncoding.EncodeToString([]byte("key")),
						}},
					})
				}
				return cr
			},
		},
		"authorization module not found": {
			expectErr: true,
			buildCR: func() csmv1.ContainerStorageModule {
				cr := CsmAuthorizationCR()
				// Remove authorization
				cr.Spec.Modules = nil
				return cr
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cr := test.buildCR()
			ctrlClient := ctrlClientFake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects().Build()
			err := applyDeleteVaultCertificates(ctx, false, cr, ctrlClient)
			if test.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestGetAuthApplyCR(t *testing.T) {
	t.Run("configmap image override", func(t *testing.T) {
		ctx := context.Background()

		// Load a CR that has the Authorization module
		cr := csmPowerScaleWithAuthCR()
		// Ensure spec.version is set (so logs are informative; not strictly required for the seam)
		cr.Spec.Version = shared.CSMVersion

		// Fake client (no objects needed because we override the resolver)
		client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

		// Override resolver seam to return a matched version with image for the proxy key.
		orig := resolveVersionFromConfigMapAuth
		resolveVersionFromConfigMapAuth = func(_ context.Context, _ ctrlClient.Client, _ *csmv1.ContainerStorageModule) (operatorutils.VersionSpec, error) {
			return operatorutils.VersionSpec{
				Version: shared.CSMVersion,
				Images:  map[string]string{"karavi-authorization-proxy": "registry.example/proxy:from-configmap"},
			}, nil
		}
		defer func() { resolveVersionFromConfigMapAuth = orig }()

		// Act
		authModule, container, err := getAuthApplyCR(ctx, cr, operatorConfig, client)
		if err != nil {
			t.Fatalf("getAuthApplyCR returned error: %v", err)
		}
		if authModule == nil || container == nil {
			t.Fatalf("expected non-nil authModule and container")
		}

		// Assert: image should be overridden by matched.Images[proxyKey]
		if container.Image == nil {
			t.Fatalf("expected container.Image to be set")
		}
		if *container.Image != "registry.example/proxy:from-configmap" {
			t.Fatalf("expected image override to 'registry.example/proxy:from-configmap', got %q", *container.Image)
		}
	})

	t.Run("configmap no image for key", func(t *testing.T) {
		ctx := context.Background()

		// Load a CR that has the Authorization module
		cr := csmPowerScaleWithAuthCR()
		cr.Spec.Version = shared.CSMVersion

		client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

		// First, capture the template image by calling with matched.Images empty (no override).
		orig := resolveVersionFromConfigMapAuth
		resolveVersionFromConfigMapAuth = func(_ context.Context, _ ctrlClient.Client, _ *csmv1.ContainerStorageModule) (operatorutils.VersionSpec, error) {
			return operatorutils.VersionSpec{
				Version: shared.CSMVersion,
				Images:  map[string]string{}, // no image for the proxy key
			}, nil
		}
		defer func() { resolveVersionFromConfigMapAuth = orig }()

		authModule, container, err := getAuthApplyCR(ctx, cr, operatorConfig, client)
		if err != nil {
			t.Fatalf("getAuthApplyCR returned error: %v", err)
		}
		if authModule == nil || container == nil {
			t.Fatalf("expected non-nil authModule and container")
		}

		// Assert: since matched.Images lacks the proxy key, image must NOT be overridden.
		if container.Image == nil {
			t.Fatalf("expected container.Image to be set by template")
		}
		defaultImage := *container.Image
		if defaultImage == "" {
			t.Fatalf("expected non-empty template image")
		}

		// Now, re-run with a different resolver STILL not providing the proxy key, and ensure it stays unchanged
		resolveVersionFromConfigMapAuth = func(_ context.Context, _ ctrlClient.Client, _ *csmv1.ContainerStorageModule) (operatorutils.VersionSpec, error) {
			return operatorutils.VersionSpec{
				Version: shared.CSMVersion,
				Images:  map[string]string{"some-other-key": "registry.example/other:tag"}, // not the proxy key
			}, nil
		}

		_, container2, err := getAuthApplyCR(ctx, cr, operatorConfig, client)
		if err != nil {
			t.Fatalf("getAuthApplyCR returned error on second call: %v", err)
		}
		if container2.Image == nil {
			t.Fatalf("expected container.Image to remain set")
		}
		if *container2.Image != defaultImage {
			t.Fatalf("expected image to remain %q when no proxy key image provided, got %q", defaultImage, *container2.Image)
		}
	})

	t.Run("use custom registry", func(t *testing.T) {
		ctx := context.Background()

		cr := csmPowerScaleWithAuthCR()
		cr.Spec.Version = ""
		cr.Spec.CustomRegistry = "quay.io"
		cr.Spec.Modules[0].ConfigVersion = shared.AuthServerConfigVersion

		client := ctrlClientFake.NewClientBuilder().WithObjects().Build()
		authModule, container, err := getAuthApplyCR(ctx, cr, operatorConfig, client)
		if err != nil {
			t.Fatalf("getAuthApplyCR returned error: %v", err)
		}
		if authModule == nil || container == nil {
			t.Fatalf("expected non-nil authModule and container")
		}

		defaultImage := *container.Image
		if defaultImage == "" {
			t.Fatalf("expected non-empty image")
		}
		if !strings.Contains(defaultImage, cr.Spec.CustomRegistry) {
			t.Fatalf("image doesn't contain custom registry")
		}
	})

	t.Run("skip certificate validation not found", func(t *testing.T) {
		ctx := context.Background()

		cr := csmPowerScaleWithAuthCR()
		cr.Spec.Version = shared.CSMVersion
		if len(cr.Spec.Modules) == 0 {
			t.Fatalf("expected CR to have modules")
		}
		cr.Spec.Modules[0].ConfigVersion = shared.AuthServerConfigVersion
		cr.Spec.Modules[0].Components = []csmv1.ContainerTemplate{
			{
				Name: "karavi-authorization-proxy",
				Envs: []corev1.EnvVar{
					{
						Name:  "PROXY_HOST",
						Value: "testing-proxy-host",
					},
				},
			},
		}

		client := ctrlClientFake.NewClientBuilder().WithObjects().Build()
		authModule, _, err := getAuthApplyCR(ctx, cr, operatorConfig, client)
		if err != nil {
			t.Fatalf("getAuthApplyCR returned error: %v", err)
		}

		count := 0
		for _, env := range authModule.Components[0].Envs {
			if env.Name == "SKIP_CERTIFICATE_VALIDATION" {
				count++
				if env.Value != "true" {
					t.Fatalf("expected SKIP_CERTIFICATE_VALIDATION to be 'true', got %q", env.Value)
				}
			}
		}
		if count != 1 {
			t.Fatalf("expected exactly one SKIP_CERTIFICATE_VALIDATION env var to be present, got %d", count)
		}
	})
}

func TestGetDefaultAuthImage(t *testing.T) {
	tests := []struct {
		name         string
		componentImg string
		defaultImg   string
		expectedImg  string
	}{
		{
			name:         "both component image and default image specified returns component image",
			componentImg: "registry.example/component:1.0.0",
			defaultImg:   "registry.example/default:2.0.0",
			expectedImg:  "registry.example/component:1.0.0",
		},
		{
			name:         "no component image returns default image",
			componentImg: "",
			defaultImg:   "registry.example/default:2.0.0",
			expectedImg:  "registry.example/default:2.0.0",
		},
		{
			name:         "no images specified return empty string",
			componentImg: "",
			defaultImg:   "",
			expectedImg:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDefaultAuthImage(
				tt.componentImg,
				tt.defaultImg,
				operatorutils.VersionSpec{Version: "1.6.0", Images: map[string]string{}})
			if got != tt.expectedImg {
				t.Fatalf("unexpected image: got %q, want %q", got, tt.expectedImg)
			}
		})
	}
}

func CsmAuthorizationCR() csmv1.ContainerStorageModule {
	res := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      authCRName,
			Namespace: authNS,
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Version: shared.CSMVersion,
			Modules: []csmv1.Module{
				{
					Name:              csmv1.AuthorizationServer,
					Enabled:           true,
					ConfigVersion:     shared.AuthServerConfigVersion,
					ForceRemoveModule: true,
					Components: []csmv1.ContainerTemplate{
						{
							Name:    AuthNginxIngressComponent,
							Enabled: &trueBool,
						},
						{
							Name:    AuthCertManagerComponent,
							Enabled: &trueBool,
						},
						{
							Name:                            AuthProxyServerComponent,
							Enabled:                         &trueBool,
							ProxyServiceReplicas:            1,
							TenantServiceReplicas:           1,
							RoleServiceReplicas:             1,
							StorageServiceReplicas:          1,
							AuthorizationControllerReplicas: 1,
							LeaderElection:                  true,
							ControllerReconcileInterval:     "5m",
							Certificate:                     "",
							PrivateKey:                      "",
							Hostname:                        "csm-authorization.com",
							ProxyServerIngress: []csmv1.ProxyServerIngress{
								{
									IngressClassName: "nginx",
									Hosts:            []string{},
									Annotations:      map[string]string{},
								},
							},
							OpenTelemetryCollectorAddress: "",
						},
						{
							Name: AuthRedisComponent,
							RedisSecretProviderClass: []csmv1.RedisSecretProviderClass{
								{
									SecretProviderClassName: "redis-spc",
									RedisSecretName:         "redis-csm-secret",
									RedisUsernameKey:        "commander_user",
									RedisPasswordKey:        "password",
									Conjur: &csmv1.ConjurCredentialPath{
										UsernamePath: "secrets/redis-username", // #nosec G101 - test file
										PasswordPath: "secrets/redis-password", // #nosec G101 - test file
									},
								},
							},
							RedisName:      "redis-csm",
							RedisCommander: "rediscommander",
							Sentinel:       "sentinel",
							RedisReplicas:  5,
						},
						{
							Name: AuthConfigSecretComponent,
							ConfigSecretProviderClass: []csmv1.ConfigSecretProviderClass{
								{
									SecretProviderClassName: "secret-provider-class",
									ConfigSecretName:        "secret",
									Conjur: &csmv1.ConjurConfigPath{
										SecretPath: "secrets/config-object", // #nosec G101 - test file
									},
								},
							},
						},
						{
							Name: AuthStorageSystemCredentialsComponent,
							SecretProviderClasses: &csmv1.StorageSystemSecretProviderClasses{
								Vaults: []string{"secret-provider-class-1", "secret-provider-class-2"},
							},
						},
					},
				},
			},
		},
	}

	return res
}

func csmPowerScaleWithAuthCR() csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)
	res.Spec.Driver.CSIDriverSpec = &csmv1.CSIDriverSpec{}

	// Add image name
	res.Spec.Driver.Common.Image = "thisIsAnImage"

	// Add pscale driver version
	res.Spec.Driver.ConfigVersion = shared.ConfigVersion

	// Add pscale driver type
	res.Spec.Driver.CSIDriverType = csmv1.PowerScale

	// Add environment variable
	csiLogLevel := corev1.EnvVar{Name: "CSI_LOG_LEVEL", Value: "debug"}

	// Add node fields specific
	res.Spec.Driver.Node = &csmv1.ContainerTemplate{}
	if res.Spec.Driver.Node != nil {
		res.Spec.Driver.Node.NodeSelector = map[string]string{"thisIs": "NodeSelector"}
		res.Spec.Driver.Node.Envs = []corev1.EnvVar{csiLogLevel}
	}

	// Add controller fields specific
	res.Spec.Driver.Controller = &csmv1.ContainerTemplate{}
	if res.Spec.Driver.Controller != nil {
		res.Spec.Driver.Controller.NodeSelector = map[string]string{"thisIs": "NodeSelector"}
		res.Spec.Driver.Controller.Envs = []corev1.EnvVar{csiLogLevel}
	}

	res.Spec.Modules = []csmv1.Module{{
		Name:    csmv1.Authorization,
		Enabled: true,
		Components: []csmv1.ContainerTemplate{{
			Name:  "karavi-authorization-proxy",
			Image: "image",
			Envs: []corev1.EnvVar{
				{
					Name:  "PROXY_HOST",
					Value: "testing-proxy-host",
				},
				{
					Name:  "SKIP_CERTIFICATE_VALIDATION",
					Value: "true",
				},
			},
		}},
	}}
	return res
}

// TestGetGatewayController tests the getGatewayController function for Gateway API support
func TestGetGatewayController(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		cr          csmv1.ContainerStorageModule
		expectError bool
		description string
	}{
		{
			name:        "Valid Authorization Module with Gateway API",
			cr:          createAuthCRWithGatewayAPI(),
			expectError: false,
			description: "Should successfully generate Gateway API controller YAML",
		},
		{
			name:        "Missing Authorization Module",
			cr:          createCRWithoutAuthModule(),
			expectError: true,
			description: "Should fail when authorization module is not present",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := operatorutils.OperatorConfig{
				ConfigDirectory: "../../operatorconfig",
			}

			yamlString, err := getGatewayController(ctx, op, tt.cr)

			if tt.expectError {
				assert.Error(t, err, tt.description)
				assert.Empty(t, yamlString)
			} else {
				assert.NoError(t, err, tt.description)
				assert.NotEmpty(t, yamlString)
				// Verify namespace replacement occurred
				assert.Contains(t, yamlString, tt.cr.Namespace)
			}
		})
	}
}

// TestAuthorizationHTTPRoute tests the authorizationHTTPRoute function
func TestAuthorizationHTTPRoute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		cr          csmv1.ContainerStorageModule
		isDeleting  bool
		expectError bool
		description string
	}{
		{
			name:        "Delete HTTPRoute",
			cr:          createAuthCRWithGatewayAPI(),
			isDeleting:  true,
			expectError: false,
			description: "Should successfully delete HTTPRoute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Register Gateway API scheme
			s := scheme.Scheme
			_ = gatewayv1.AddToScheme(s)

			fakeClient := ctrlClientFake.NewClientBuilder().WithScheme(s).Build()

			r := &operatorutils.FakeReconcileCSM{
				Client:    fakeClient,
				K8sClient: fake.NewSimpleClientset(),
			}

			err := authorizationHTTPRoute(ctx, tt.isDeleting, tt.cr, r, fakeClient)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestAuthorizationIngressWithGatewayAPI tests the AuthorizationIngress function with Gateway API routing
func TestAuthorizationIngressWithGatewayAPI(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		cr          csmv1.ContainerStorageModule
		isDeleting  bool
		isOpenShift bool
		expectError bool
		description string
	}{
		{
			name:        "Gateway API Version - Delete HTTPRoute",
			cr:          createAuthCRWithGatewayAPI(),
			isDeleting:  true,
			isOpenShift: false,
			expectError: false,
			description: "Should successfully delete HTTPRoute for v2.5.0+ (no error when object doesn't exist)",
		},
		{
			name:        "Legacy Version - Should use Ingress",
			cr:          createAuthCRWithLegacyVersion(),
			isDeleting:  false,
			isOpenShift: false,
			expectError: true,
			description: "Should route to Ingress creation for v2.4.0 and below (fails without real resources)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Register Gateway API scheme
			s := scheme.Scheme
			_ = gatewayv1.AddToScheme(s)

			fakeClient := ctrlClientFake.NewClientBuilder().WithScheme(s).Build()

			r := &operatorutils.FakeReconcileCSM{
				Client:    fakeClient,
				K8sClient: fake.NewSimpleClientset(),
			}

			err := AuthorizationIngress(ctx, tt.isDeleting, tt.isOpenShift, tt.cr, r, fakeClient)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestCreateHTTPRoute tests HTTPRoute creation for Gateway API
func TestCreateHTTPRoute(t *testing.T) {
	tests := []struct {
		name        string
		cr          csmv1.ContainerStorageModule
		expectError bool
		description string
	}{
		{
			name:        "Valid HTTPRoute Creation",
			cr:          createAuthCRWithGatewayAPI(),
			expectError: false,
			description: "Should successfully create HTTPRoute object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route, err := createHTTPRoute(tt.cr)

			if tt.expectError {
				assert.Error(t, err, tt.description)
				assert.Nil(t, route)
			} else {
				assert.NoError(t, err, tt.description)
				assert.NotNil(t, route)
				assert.Equal(t, tt.cr.Namespace, route.Namespace)
				assert.Contains(t, route.Name, "proxy-server")
			}
		})
	}
}

// Helper functions for Gateway API tests

func createAuthCRWithGatewayAPI() csmv1.ContainerStorageModule {
	return csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "auth-gateway",
			Namespace: "authorization",
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: csmv1.Driver{
				CSIDriverType: csmv1.PowerStore,
				ConfigVersion: "v2.17.0",
			},
			Modules: []csmv1.Module{
				{
					Name:              csmv1.AuthorizationServer,
					Enabled:           true,
					ConfigVersion:     "v2.5.0",
					ForceRemoveModule: false,
					Components: []csmv1.ContainerTemplate{
						{
							Name:  "proxy-server",
							Image: "dellemc/csm-authorization-proxy:v2.5.0",
							Envs: []corev1.EnvVar{
								{Name: "PROXY_HOST", Value: "authorization-gateway-nginx.authorization.svc.cluster.local"},
							},
						},
					},
				},
			},
		},
	}
}

func createAuthCRWithLegacyVersion() csmv1.ContainerStorageModule {
	cr := createAuthCRWithGatewayAPI()
	cr.Spec.Modules[0].ConfigVersion = "v2.4.0"
	return cr
}

func createCRWithoutAuthModule() csmv1.ContainerStorageModule {
	return csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-auth",
			Namespace: "test",
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: csmv1.Driver{
				CSIDriverType: csmv1.PowerStore,
				ConfigVersion: "v2.17.0",
			},
			Modules: []csmv1.Module{},
		},
	}
}

func TestGatewayController(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		description string
		isDeleting  bool
		cr          csmv1.ContainerStorageModule
		expectError bool
	}{
		{
			description: "Gateway API controller creation",
			isDeleting:  false,
			cr:          createAuthCRWithGatewayAPI(),
			expectError: false,
		},
		{
			description: "Gateway API controller deletion",
			isDeleting:  true,
			cr:          createAuthCRWithGatewayAPI(),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			// Create a fake client with Gateway API scheme
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			require.NoError(t, rbacv1.AddToScheme(scheme))
			require.NoError(t, appsv1.AddToScheme(scheme))
			require.NoError(t, csmv1.AddToScheme(scheme))
			require.NoError(t, gatewayv1.AddToScheme(scheme))
			require.NoError(t, certmanagerv1.AddToScheme(scheme))

			fakeClient := ctrlClientFake.NewClientBuilder().WithScheme(scheme).Build()

			// Create operator config
			operatorConfig := operatorutils.OperatorConfig{
				ConfigDirectory: "../../operatorconfig",
			}

			// Test GatewayController function
			err := GatewayController(ctx, tt.isDeleting, operatorConfig, tt.cr, fakeClient)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}
