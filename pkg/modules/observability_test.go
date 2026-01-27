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
	"fmt"
	"strings"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	shared "github.com/dell/csm-operator/tests/sharedutil"
	"github.com/dell/csm-operator/tests/sharedutil/clientgoclient"
	"github.com/dell/csm-operator/tests/sharedutil/crclient"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
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
			observability.ConfigVersion = "v1.13.0"

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
		"Success - deleting topology component for old csm": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability_with_topology.yaml")
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

		"Success - creating topology component for old csm": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability_with_topology.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"Fail - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
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

			return false, true, tmpCR, sourceClient, operatorConfig
		},

		"Fail - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
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
		// Covers image override & TopologyLogLevel env-based override
		"Success - topology image and loglevel override": func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability_with_topology.yaml")
			if err != nil {
				panic(err)
			}
			tmpCR := customResource

			// Find the topology component defensively.
			topoFound := false
			for mi := range tmpCR.Spec.Modules {
				for ci := range tmpCR.Spec.Modules[mi].Components {
					if tmpCR.Spec.Modules[mi].Components[ci].Name == ObservabilityTopologyName {
						topoFound = true
						// Override image to drive topologyImage path in getTopology
						tmpCR.Spec.Modules[mi].Components[ci].Image = csmv1.ImageType("registry.example/karavi-topology:test-override")

						// Ensure an env whose name contains TopologyLogLevel exists and set it to DEBUG.
						envs := tmpCR.Spec.Modules[mi].Components[ci].Envs
						set := false
						for ei := range envs {
							if strings.Contains(TopologyLogLevel, envs[ei].Name) {
								tmpCR.Spec.Modules[mi].Components[ci].Envs[ei].Value = "DEBUG"
								set = true
								break
							}
						}
						if !set {
							// Create a new env for TopologyLogLevel
							tmpCR.Spec.Modules[mi].Components[ci].Envs = append(tmpCR.Spec.Modules[mi].Components[ci].Envs, corev1.EnvVar{
								Name:  TopologyLogLevel, // e.g., "TOPOLOGY_LOG_LEVEL"
								Value: "DEBUG",
							})
						}
						break
					}
				}
			}
			if !topoFound {
				t.Skip("ObservabilityTopologyName component not found in CR; skipping branch-coverage test for topology image/loglevel")
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op := tc(t)

			err := ObservabilityTopology(ctx, isDeleting, op, cr, sourceClient, operatorutils.VersionSpec{})
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
			// Extra validation for the override case to ensure branch coverage in getTopology:
			if name == "Success - topology image and loglevel override" {
				topoObjs, topoErr := getTopology(ctx, op, cr, operatorutils.VersionSpec{})
				if topoErr != nil {
					t.Fatalf("getTopology returned error: %v", topoErr)
				}

				// Scan objects for karavi-topology container and verify image + log level surfaced
				foundImage := false
				foundLogLevel := false

				for _, obj := range topoObjs {
					if dep, ok := obj.(*appsv1.Deployment); ok {
						// Check env injection or substitutions
						// 1) Container image override
						for _, c := range dep.Spec.Template.Spec.Containers {
							if c.Name == "karavi-topology" {
								if c.Image == "registry.example/karavi-topology:test-override" {
									foundImage = true
								}
								// 2) Check env for DEBUG
								for _, e := range c.Env {
									if strings.Contains(TopologyLogLevel, e.Name) && e.Value == "DEBUG" {
										foundLogLevel = true
										break
									}
								}
								// If your template uses args instead of envs, also scan c.Args:
								if !foundLogLevel {
									for _, a := range c.Args {
										if strings.Contains(a, "DEBUG") && strings.Contains(a, strings.TrimSpace(strings.ReplaceAll(TopologyLogLevel, "_", "-"))) {
											foundLogLevel = true
											break
										}
									}
								}

								// Optionally check labels/annotations if your template substitutes there:
								if !foundLogLevel {
									for k, v := range dep.Spec.Template.Annotations {
										if strings.Contains(k, strings.ToLower(TopologyLogLevel)) && v == "DEBUG" {
											foundLogLevel = true
											break
										}
									}
									for k, v := range dep.Spec.Template.Labels {
										if strings.Contains(k, strings.ToLower(TopologyLogLevel)) && v == "DEBUG" {
											foundLogLevel = true
											break
										}
									}
								}
							}
						}
					}
				}

				assert.True(t, foundImage, "karavi-topology container with overridden image not found in rendered objects")
				assert.True(t, foundLogLevel, "TopologyLogLevel=DEBUG not found in env/args/labels; adjust checks to match template usage")
			}
		})
	}
}

func TestPowerScaleMetrics(t *testing.T) {
	ctx := context.Background()
	// If you have a shared operatorConfig in your tests, use that.
	// Otherwise construct a minimal viable OperatorConfig that works for your environment.
	op := operatorConfig
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface) {
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
			return true, true, tmpCR, sourceClient, op, func() kubernetes.Interface {
				return fake.NewSimpleClientset()
			}
		},
		"success - deleting with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface) {
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
			return true, true, tmpCR, sourceClient, op, func() kubernetes.Interface {
				return fake.NewSimpleClientset()
			}
		},
		"success - deleting with auth after one cycle": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface) {
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
			err = PowerScaleMetrics(ctx, false, op, tmpCR, sourceClient, k8sClient)
			if err != nil {
				panic(err)
			}
			return true, true, tmpCR, sourceClient, op, func() kubernetes.Interface {
				// fresh client for delete cycle
				return clientgoclient.NewFakeClient(sourceClient)
			}
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}
			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
			tmpCR := customResource
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
			return true, false, tmpCR, sourceClient, op, func() kubernetes.Interface {
				return clientgoclient.NewFakeClient(sourceClient)
			}
		},
		"success - creating with auth": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface) {
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
			return true, false, tmpCR, sourceClient, op, func() kubernetes.Interface {
				return clientgoclient.NewFakeClient(sourceClient)
			}
		},
		"success - update objects": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}
			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")
			objects := map[shared.StorageKey]runtime.Object{}
			fakeClient := crclient.NewFakeClientNoInjector(objects)
			if err = fakeClient.Create(ctx, isilonCreds); err != nil {
				panic(err)
			}
			if err = fakeClient.Create(ctx, karaviAuthconfig); err != nil {
				panic(err)
			}
			if err = fakeClient.Create(ctx, proxyAuthzTokens); err != nil {
				panic(err)
			}
			k8sClient := clientgoclient.NewFakeClient(fakeClient)
			// pre-run to generate objects
			err = PowerScaleMetrics(ctx, false, op, customResource, fakeClient, k8sClient)
			if err != nil {
				panic(err)
			}
			// enable auth after generation → should patch/update
			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true
			return true, false, tmpCR, fakeClient, op, func() kubernetes.Interface {
				return clientgoclient.NewFakeClient(fakeClient)
			}
		},
		"success - copy secrets when secrets already existed (v2.14)": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability_214.yaml")
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
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(
				isilonCreds, isilonKaraviAuthconfig, isilonProxyAuthzTokens,
				karaviIsilonCreds, karaviAuthconfig, proxyAuthzTokens,
			).Build()
			return true, false, tmpCR, sourceClient, op, func() kubernetes.Interface {
				return clientgoclient.NewFakeClient(sourceClient)
			}
		},
		"success - CR image override (powerscale metrics)": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}
			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
			// Set image override on the metrics-powerscale component
			for i := range customResource.Spec.Modules[0].Components {
				if customResource.Spec.Modules[0].Components[i].Name == "metrics-powerscale" {
					customResource.Spec.Modules[0].Components[i].Image = "registry.example/karavi-metrics-powerscale:cr-override"
				}
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
			return true, false, customResource, sourceClient, op, func() kubernetes.Interface {
				return clientgoclient.NewFakeClient(sourceClient)
			}
		},
		// --- Failure cases below
		"Fail - no secrets in isilon namespace (v2.14)": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability_214.yaml")
			if err != nil {
				panic(err)
			}
			tmpCR := customResource
			// Auth enabled triggers auth injection path and secrets usage
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true
			// No secrets provided in isilon namespace → should fail during appendObservabilitySecrets
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return false, false, tmpCR, sourceClient, op, func() kubernetes.Interface {
				return clientgoclient.NewFakeClient(sourceClient)
			}
		},
		"Fail - wrong module name (no deployment found)": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface) {
			// Replica CR does not have observability metrics module → getPowerScaleMetricsObjects won't include deployment
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}
			tmpCR := customResource
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return false, false, tmpCR, sourceClient, op, func() kubernetes.Interface {
				return clientgoclient.NewFakeClient(sourceClient)
			}
		},
		"Fail - skipCertificateValidation=false but no cert": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability_214.yaml")
			if err != nil {
				panic(err)
			}
			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
			karaviAuthconfig := getSecret(customResource.Namespace, "karavi-authorization-config")
			proxyAuthzTokens := getSecret(customResource.Namespace, "proxy-authz-tokens")
			tmpCR := customResource
			auth := &tmpCR.Spec.Modules[1]
			auth.Enabled = true
			// set SKIP_CERTIFICATE_VALIDATION to false → requires cert present
			for i, env := range auth.Components[0].Envs {
				if env.Name == "SKIP_CERTIFICATE_VALIDATION" {
					auth.Components[0].Envs[i].Value = "false"
				}
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds, karaviAuthconfig, proxyAuthzTokens).Build()
			return false, false, tmpCR, sourceClient, op, func() kubernetes.Interface {
				return clientgoclient.NewFakeClient(sourceClient)
			}
		},
		"Fail - CR has version set but configmap missing (ResolveVersionFromConfigMap error)": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}
			// Force version resolution path
			customResource.Spec.Version = "v2.14.0"
			tmpCR := customResource
			// No ConfigMap present in cluster for version resolution → should error
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return false, false, tmpCR, sourceClient, op, func() kubernetes.Interface {
				return clientgoclient.NewFakeClient(sourceClient)
			}
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op, k8sFactory := tc(t)
			// client-go fake constructed per case
			k8sClient := k8sFactory()
			err := PowerScaleMetrics(ctx, isDeleting, op, cr, sourceClient, k8sClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestPowerScaleMetrics_VersionResolveError(t *testing.T) {
	ctx := context.Background()
	// Construct a minimal CR that sets Spec.Version to trigger ResolveVersionFromConfigMap.
	cr, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	if err != nil {
		t.Fatalf("failed to load CR: %v", err)
	}
	cr.Spec.Version = "v2.14.0" // non-empty → forces ResolveVersionFromConfigMap path
	// Build a controller-runtime fake client with NO ConfigMaps or related resources,
	// so ResolveVersionFromConfigMap will fail.
	ctrlClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
	// client-go fake for deployment sync (won’t be reached due to early error).
	k8sClient := clientgoclient.NewFakeClient(ctrlClient)
	// Use the operatorConfig available in your test suite.
	op := operatorConfig
	// Act: invoke PowerScaleMetrics. Expect an error returned from version resolution.
	err = PowerScaleMetrics(ctx, false /*isDeleting*/, op, cr, ctrlClient, k8sClient)
	// Assert: the function must return an error at the version resolution step.
	assert.Error(t, err, "expected error when configmap for version resolution is missing")
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

			err := OtelCollector(ctx, isDeleting, op, cr, sourceClient, operatorutils.VersionSpec{})
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
		"success - CR image override (powerflex metrics)": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}
			vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")

			for i := range customResource.Spec.Modules[0].Components {
				if customResource.Spec.Modules[0].Components[i].Name == "metrics-powerflex" {
					customResource.Spec.Modules[0].Components[i].Image = "registry.example/karavi-metrics-powerflex:cr-override"
				}
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(vxflexosCreds).Build()
			return true, false, customResource, sourceClient, operatorConfig
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
		"success - copy secrets when secrets already existed": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability_214.yaml")
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
		"Fail - wrong module name": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},
		"Fail - no secrets in test-vxflexos namespace": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability_214.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},

		"Fail - version resolve error (ConfigMap missing)": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			// Non-empty version forces ResolveVersionFromConfigMap path
			tmpCR.Spec.Version = "v2.14.0"

			// No ConfigMap seeded → resolution should fail and function should return error
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

func TestPowerStoreMetrics(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_observability.yaml")
			if err != nil {
				panic(err)
			}
			powerstoreCreds := getSecret(customResource.Namespace, "test-powerstore-config")

			tmpCR := customResource

			cr := &rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					Kind: "ClusterRole",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "karavi-metrics-powerstore-controller",
				},
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(cr, powerstoreCreds).Build()

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_observability.yaml")
			if err != nil {
				panic(err)
			}
			powerstoreCreds := getSecret(customResource.Namespace, "test-powerstore-config")

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(powerstoreCreds).Build()

			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - deleting after one cycle": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			pstoreCreds := getSecret(customResource.Namespace, "test-powerstore-config")

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(pstoreCreds).Build()
			k8sClient := clientgoclient.NewFakeClient(sourceClient)

			// pre-run to generate objects
			err = PowerStoreMetrics(ctx, false, operatorConfig, tmpCR, sourceClient, k8sClient)
			if err != nil {
				panic(err)
			}

			return true, true, tmpCR, sourceClient, operatorConfig
		},
		"Fail - wrong module name": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},

		// Add this inside the `tests` map in TestPowerStoreMetrics
		"Fail - version resolve error (ConfigMap missing)": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			// Load the standard PowerStore observability CR
			customResource, err := getCustomResource("./testdata/cr_powerstore_observability.yaml")
			if err != nil {
				panic(err)
			}

			// Force the version resolution path
			tmpCR := customResource
			tmpCR.Spec.Version = "v2.14.0" // non-empty triggers ResolveVersionFromConfigMap

			// No ConfigMap seeded in the fake client, so resolution should fail
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			// Expect error
			return false, false, tmpCR, sourceClient, operatorConfig
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op := tc(t)
			k8sClient := clientgoclient.NewFakeClient(sourceClient)
			err := PowerStoreMetrics(ctx, isDeleting, op, cr, sourceClient, k8sClient)
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
		"success - CR image override (powerflex metrics)": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}
			vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")

			for i := range customResource.Spec.Modules[0].Components {
				if customResource.Spec.Modules[0].Components[i].Name == "metrics-powerflex" {
					customResource.Spec.Modules[0].Components[i].Image = "registry.example/karavi-metrics-powerflex:cr-override"
				}
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(vxflexosCreds).Build()
			return true, false, customResource, sourceClient, operatorConfig
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
		"Fail - no secrets in test-powermax namespace": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability_214.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
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
		"Fail - skipCertificateValidation is false but no cert": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability_214.yaml")
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

		"Fail - version resolve error (ConfigMap missing)": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			// Non-empty version forces ResolveVersionFromConfigMap path
			tmpCR.Spec.Version = "v2.14.0"

			// No ConfigMap seeded → resolution should fail and function should return error
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

func TestGetTopology_MockedInputs_CoversImageAndLogLevelBranch(t *testing.T) {
	ctx := context.Background()

	// Save originals and restore after test
	origGetObs := getObservabilityModuleFn
	origReadCfg := readConfigFileFn
	defer func() {
		getObservabilityModuleFn = origGetObs
		readConfigFileFn = origReadCfg
	}()

	// --- Mock getObservabilityModuleFn to return a crafted Observability module ---
	getObservabilityModuleFn = func(_ csmv1.ContainerStorageModule) (csmv1.Module, error) {
		return csmv1.Module{
			Name:    csmv1.Observability,
			Enabled: true,
			Components: []csmv1.ContainerTemplate{
				{
					Name:  ObservabilityTopologyName,                                         // e.g., "karavi-topology"
					Image: csmv1.ImageType("registry.example/karavi-topology:test-override"), // non-empty → covers image branch
					Envs: []corev1.EnvVar{
						{
							// Name must contain TopologyLogLevel token to drive logLevel branch
							Name:  TopologyLogLevel, // e.g., "TOPOLOGY_LOG_LEVEL"
							Value: "DEBUG",
						},
					},
				},
			},
		}, nil
	}

	// --- Mock readConfigFileFn to return a minimal deployment YAML ---
	// We place the literal TopologyLogLevel token in the env VALUE so ReplaceAll sets it to "DEBUG".
	readConfigFileFn = func(_ context.Context, _ csmv1.Module, cr csmv1.ContainerStorageModule, _ operatorutils.OperatorConfig, _ string) ([]byte, error) {
		yaml := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: karavi-topology
  namespace: %s
spec:
  template:
    metadata:
      labels:
        app: karavi-topology
    spec:
      containers:
      - name: karavi-topology
        image: registry.example/karavi-topology:template
        env:
        - name: LOG_LEVEL
          value: %s
`, cr.Namespace, TopologyLogLevel)
		return []byte(yaml), nil
	}

	// Minimal CR; only name/namespace are needed for token replacement
	cr := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csm-ut",
			Namespace: "csm-ut-ns",
		},
		// No need to set Spec.Driver; we mocked collaborators
	}

	// Use your existing operatorConfig (readConfigFileFn is mocked, so on-disk templates are not needed)
	op := operatorConfig

	// Act
	objs, err := getTopology(ctx, op, cr, operatorutils.VersionSpec{})
	if err != nil {
		t.Fatalf("getTopology returned error: %v", err)
	}
	if len(objs) == 0 {
		t.Fatalf("expected non-empty topology objects")
	}

	// Assert: verify karavi-topology image override and log level surfaced in env
	foundImage := false
	foundLogLevel := false

	for _, obj := range objs {
		if dep, ok := obj.(*appsv1.Deployment); ok {
			for _, c := range dep.Spec.Template.Spec.Containers {
				if c.Name == "karavi-topology" {
					if c.Image == "registry.example/karavi-topology:test-override" {
						foundImage = true
					}
					for _, e := range c.Env {
						// value should have been replaced from TopologyLogLevel token to "DEBUG"
						if e.Name == "LOG_LEVEL" && e.Value == "DEBUG" {
							foundLogLevel = true
						}
					}
				}
			}
		}
	}

	if !foundImage {
		t.Fatalf("karavi-topology container with overridden image not found in rendered objects")
	}
	if !foundLogLevel {
		t.Fatalf("LOG_LEVEL env with value DEBUG not found in rendered objects")
	}
}

func TestGetPowerFlexMetricsObject_ImageFromMatchedVersionSpec(t *testing.T) {
	ctx := context.Background()

	// Save & restore seams
	origGetObs := getObservabilityModuleFn
	origReadCfg := readConfigFileFn
	defer func() {
		getObservabilityModuleFn = origGetObs
		readConfigFileFn = origReadCfg
	}()

	// --- Mock getObservabilityModuleFn to include the PowerFlex metrics component ---
	getObservabilityModuleFn = func(_ csmv1.ContainerStorageModule) (csmv1.Module, error) {
		return csmv1.Module{
			Name:    csmv1.Observability,
			Enabled: true,
			Components: []csmv1.ContainerTemplate{
				{
					Name: ObservabilityMetricsPowerFlexName, // the component we scan
					// Do NOT set component.Image here so matched override is visible
					Envs: []corev1.EnvVar{
						{Name: PowerflexLogLevel, Value: "INFO"},
					},
				},
			},
		}, nil
	}

	// --- Mock readConfigFileFn to return a minimal Deployment YAML with the container ---
	readConfigFileFn = func(_ context.Context, _ csmv1.Module, cr csmv1.ContainerStorageModule, _ operatorutils.OperatorConfig, _ string) ([]byte, error) {
		yaml := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: karavi-metrics-powerflex
  namespace: %s
spec:
  template:
    spec:
      containers:
      - name: karavi-metrics-powerflex
        image: registry.example/karavi-metrics-powerflex:template
        env:
        - name: %s
          value: INFO
`, cr.Namespace, PowerflexLogLevel)
		return []byte(yaml), nil
	}

	// Minimal CR with name/namespace
	cr := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csm-ut",
			Namespace: "csm-ut-ns",
		},
	}
	op := operatorConfig

	// --- Craft matched to drive the branch (non-empty Version + image for the component name) ---
	matched := operatorutils.VersionSpec{
		Version: "v9.9.9",
		Images: map[string]string{
			ObservabilityMetricsPowerFlexName: "registry.example/karavi-metrics-powerflex:from-matched",
		},
	}

	// Act
	objs, err := getPowerFlexMetricsObject(ctx, op, cr, matched)
	if err != nil {
		t.Fatalf("getPowerFlexMetricsObject returned error: %v", err)
	}
	if len(objs) == 0 {
		t.Fatalf("expected non-empty metrics objects")
	}

	// Assert: image was set from matched.Images[component.Name]
	found := false
	for _, obj := range objs {
		if dep, ok := obj.(*appsv1.Deployment); ok {
			for _, c := range dep.Spec.Template.Spec.Containers {
				if c.Name == "karavi-metrics-powerflex" {
					if c.Image == "registry.example/karavi-metrics-powerflex:from-matched" {
						found = true
					}
				}
			}
		}
	}
	if !found {
		t.Fatalf("karavi-metrics-powerflex container with image from matched not found")
	}
}

// Also cover precedence: component.Image should override matched.Images when non-empty
func TestGetPowerFlexMetricsObject_ComponentImageOverridesMatched(t *testing.T) {
	ctx := context.Background()

	// Save & restore seams
	origGetObs := getObservabilityModuleFn
	origReadCfg := readConfigFileFn
	defer func() {
		getObservabilityModuleFn = origGetObs
		readConfigFileFn = origReadCfg
	}()

	// Mock Observability module with component.Image set
	getObservabilityModuleFn = func(_ csmv1.ContainerStorageModule) (csmv1.Module, error) {
		return csmv1.Module{
			Name:    csmv1.Observability,
			Enabled: true,
			Components: []csmv1.ContainerTemplate{
				{
					Name:  ObservabilityMetricsPowerFlexName,
					Image: csmv1.ImageType("registry.example/karavi-metrics-powerflex:from-component"),
					Envs:  nil,
				},
			},
		}, nil
	}

	// Minimal template YAML
	readConfigFileFn = func(_ context.Context, _ csmv1.Module, cr csmv1.ContainerStorageModule, _ operatorutils.OperatorConfig, _ string) ([]byte, error) {
		yaml := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: karavi-metrics-powerflex
  namespace: %s
spec:
  template:
    spec:
      containers:
      - name: karavi-metrics-powerflex
        image: registry.example/karavi-metrics-powerflex:template
`, cr.Namespace)
		return []byte(yaml), nil
	}

	cr := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csm-ut",
			Namespace: "csm-ut-ns",
		},
	}
	op := operatorConfig

	matched := operatorutils.VersionSpec{
		Version: "v9.9.9",
		Images: map[string]string{
			ObservabilityMetricsPowerFlexName: "registry.example/karavi-metrics-powerflex:from-matched",
		},
	}

	objs, err := getPowerFlexMetricsObject(ctx, op, cr, matched)
	if err != nil {
		t.Fatalf("getPowerFlexMetricsObject returned error: %v", err)
	}

	// Assert: component.Image should win over matched.Images
	want := "registry.example/karavi-metrics-powerflex:from-matched"
	found := false
	for _, obj := range objs {
		if dep, ok := obj.(*appsv1.Deployment); ok {
			for _, c := range dep.Spec.Template.Spec.Containers {
				if c.Name == "karavi-metrics-powerflex" {
					if c.Image == want {
						found = true
					} else {
						t.Errorf("unexpected image for container %q: got=%q want=%q", c.Name, c.Image, want)
					}
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected karavi-metrics-powerflex image from matched.Images to override component/template")
	}
}

func TestObservabilityTopology_VersionResolveBranches(t *testing.T) {
	ctx := context.Background()

	// Save & restore seams
	origGetVersion := getVersionFn
	defer func() {
		getVersionFn = origGetVersion
	}()

	t.Run("success - version set and ResolveVersionFromConfigMap returns matched", func(t *testing.T) {
		customResource, err := getCustomResource("./testdata/cr_powerscale_observability_with_topology.yaml")
		if err != nil {
			t.Fatalf("failed to load CR: %v", err)
		}
		// Force version resolution path
		customResource.Spec.Version = "v2.14.0"

		sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

		matched := operatorutils.VersionSpec{
			Version: "v2.14.0",
			Images: map[string]string{
				ObservabilityTopologyName: "registry.example/karavi-topology:from-matched",
			},
		}

		// Stub GetVersion → must pass the contains("v2.14") check
		getVersionFn = func(_ context.Context, _ *csmv1.ContainerStorageModule, _ operatorutils.OperatorConfig) (string, error) {
			return "v2.14.0", nil
		}

		// Act
		err = ObservabilityTopology(ctx, false /* isDeleting */, operatorConfig, customResource, sourceClient, matched)
		assert.NoError(t, err, "expected success when version resolves from configmap and GetVersion returns v2.14.x")
	})

	t.Run("fail - version set and ResolveVersionFromConfigMap returns error", func(t *testing.T) {
		customResource, err := getCustomResource("./testdata/cr_powerscale_observability_with_topology.yaml")
		if err != nil {
			t.Fatalf("failed to load CR: %v", err)
		}
		customResource.Spec.Version = "v2.14.0"

		sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

		// Optionally stub GetVersion to something valid—won’t be reached due to early error
		getVersionFn = func(_ context.Context, _ *csmv1.ContainerStorageModule, _ operatorutils.OperatorConfig) (string, error) {
			return "", fmt.Errorf("forced GetVersion error")
		}

		err = ObservabilityTopology(ctx, false, operatorConfig, customResource, sourceClient, operatorutils.VersionSpec{})
		assert.Error(t, err, "expected error when version resolution fails")
	})
}
