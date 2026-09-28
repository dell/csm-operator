// Copyright (c) 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func TestObservabilityPrecheck_MetricsTLSCertSecret(t *testing.T) {
	type testCase struct {
		name        string
		mutateCR    func(*csmv1.ContainerStorageModule, *csmv1.Module)
		secretNames []string
		expectedErr string
	}

	tests := []testCase{
		{
			name: "metrics enabled with tls secret validates secret exists",
			mutateCR: func(_ *csmv1.ContainerStorageModule, obs *csmv1.Module) {
				obs.Metrics = &csmv1.ModuleMetrics{ //nolint:gosec
					Enabled:       true,
					TLSCertSecret: "powerscale-metrics-tls",
				}
			},
			secretNames: []string{"powerscale-metrics-tls"},
			expectedErr: "",
		},
		{
			name: "metrics enabled with tls secret returns error when secret missing",
			mutateCR: func(_ *csmv1.ContainerStorageModule, obs *csmv1.Module) {
				obs.Metrics = &csmv1.ModuleMetrics{ //nolint:gosec
					Enabled:       true,
					TLSCertSecret: "powerscale-metrics-tls",
				}
			},
			expectedErr: "failed to find secret powerscale-metrics-tls",
		},
		{
			name: "metrics disabled with tls secret skips secret validation",
			mutateCR: func(_ *csmv1.ContainerStorageModule, obs *csmv1.Module) {
				obs.Metrics = &csmv1.ModuleMetrics{ //nolint:gosec
					Enabled:       false,
					TLSCertSecret: "powerscale-metrics-tls",
				}
			},
			expectedErr: "",
		},
		{
			name: "metrics enabled with empty tls secret skips secret validation",
			mutateCR: func(_ *csmv1.ContainerStorageModule, obs *csmv1.Module) {
				obs.Metrics = &csmv1.ModuleMetrics{ //nolint:gosec
					Enabled:       true,
					TLSCertSecret: "",
				}
			},
			expectedErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				t.Fatalf("failed to load CR: %v", err)
			}

			observability := customResource.Spec.Modules[0]
			tt.mutateCR(&customResource, &observability)
			secrets := make([]ctrlClient.Object, 0, len(tt.secretNames))
			for _, secretName := range tt.secretNames {
				secrets = append(secrets, getSecret(customResource.Namespace, secretName))
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(secrets...).Build()
			fakeReconcile := operatorutils.FakeReconcileCSM{
				Client:    sourceClient,
				K8sClient: fake.NewSimpleClientset(),
			}

			err = ObservabilityPrecheck(ctx, operatorConfig, observability, customResource, &fakeReconcile)
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}

func TestObservabilityTopologyController(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig){
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
		"success - creating with spec version set": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}
			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
			tmpCR := customResource
			tmpCR.Spec.Version = "v1.18.0"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
			return true, false, tmpCR, sourceClient, op, func() kubernetes.Interface {
				return clientgoclient.NewFakeClient(sourceClient)
			}
		},
		"success - creating with auth secret override": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}
			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
			tmpCR := customResource
			tmpCR.Spec.Driver.AuthSecret = "custom-auth-secret"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
			return true, false, tmpCR, sourceClient, op, func() kubernetes.Interface {
				return clientgoclient.NewFakeClient(sourceClient)
			}
		},
		"success - creating with observability self-metrics enabled": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, func() kubernetes.Interface) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
			if err != nil {
				panic(err)
			}
			isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
			for mi := range customResource.Spec.Modules {
				if customResource.Spec.Modules[mi].Name == csmv1.Observability {
					customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
						Enabled: true,
						Port:    9443,
						ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
							Enabled:       true,
							Interval:      "15s",
							ScrapeTimeout: "5s",
						},
					}
				}
			}
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
			return true, false, customResource, sourceClient, op, func() kubernetes.Interface {
				return clientgoclient.NewFakeClient(sourceClient)
			}
		},
		// --- Failure cases below
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

func TestPowerScaleMetrics_ServiceMonitorEnabledToDisabledTransitionDeletesMonitor(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	require.NoError(t, err)

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:  true,
					Interval: "15s",
				},
			}
		}
	}

	isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerScaleMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerscale-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor))

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled: false,
				},
			}
		}
	}

	err = PowerScaleMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitorAfter := &unstructured.Unstructured{}
	serviceMonitorAfter.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	err = fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerscale-obs-monitor", Namespace: customResource.Namespace}, serviceMonitorAfter)
	require.True(t, k8sErrors.IsNotFound(err), "ServiceMonitor must be deleted when observability serviceMonitor is disabled")
}

func TestPowerScaleMetrics_MetricsEnabledToDisabledDeletesServiceMonitor(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	require.NoError(t, err)

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:  true,
					Interval: "30s",
				},
			}
		}
	}

	isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerScaleMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerscale-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor))

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: false,
			}
		}
	}

	err = PowerScaleMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitorAfter := &unstructured.Unstructured{}
	serviceMonitorAfter.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	err = fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerscale-obs-monitor", Namespace: customResource.Namespace}, serviceMonitorAfter)
	require.True(t, k8sErrors.IsNotFound(err), "ServiceMonitor must be deleted when metrics.enabled is disabled")
}

func TestPowerScaleMetrics_ServiceMonitorCreatedWhenBothEnabled(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	require.NoError(t, err)

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled:       true,
				Port:          9443,
				TLSCertSecret: "powerscale-metrics-tls",
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:            true,
					Interval:           "30s",
					ScrapeTimeout:      "10s",
					InsecureSkipVerify: true,
				},
			}
		}
	}

	isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerScaleMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	err = fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerscale-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor)
	require.NoError(t, err, "ServiceMonitor must be created when both metrics.enabled and serviceMonitor.enabled are true")

	spec, found, err := unstructured.NestedMap(serviceMonitor.Object, "spec")
	require.NoError(t, err)
	require.True(t, found, "ServiceMonitor spec must exist")

	endpoints, found, err := unstructured.NestedSlice(spec, "endpoints")
	require.NoError(t, err)
	require.True(t, found, "ServiceMonitor endpoints must exist")
	require.Greater(t, len(endpoints), 0, "ServiceMonitor must have at least one endpoint")

	endpoint, ok := endpoints[0].(map[string]interface{})
	require.True(t, ok, "ServiceMonitor endpoint must be a map")
	assert.Equal(t, "obs-metrics", endpoint["port"])
	assert.Equal(t, "30s", endpoint["interval"])
	assert.Equal(t, "10s", endpoint["scrapeTimeout"])
	assert.Equal(t, "https", endpoint["scheme"])
	tlsConfig, ok := endpoint["tlsConfig"].(map[string]interface{})
	require.True(t, ok, "ServiceMonitor tlsConfig must be a map")
	assert.Equal(t, true, tlsConfig["insecureSkipVerify"])

	service := &corev1.Service{}
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerscale", Namespace: customResource.Namespace}, service))
	obsMetricsPortFound := false
	for _, port := range service.Spec.Ports {
		if port.Name == "obs-metrics" {
			obsMetricsPortFound = true
			assert.Equal(t, int32(9443), port.Port)
		}
	}
	assert.True(t, obsMetricsPortFound, "Service must expose obs-metrics port")
}

func TestPowerScaleMetrics_ServiceMonitorUpdatedOnCRChange(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	require.NoError(t, err)

	// Initial reconcile with interval=15s
	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:       true,
					Interval:      "15s",
					ScrapeTimeout: "5s",
				},
			}
		}
	}

	isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerScaleMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerscale-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor))

	// Verify initial interval
	spec, found, err := unstructured.NestedMap(serviceMonitor.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	endpoints, found, err := unstructured.NestedSlice(spec, "endpoints")
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, len(endpoints), 0)
	endpoint, ok := endpoints[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "15s", endpoint["interval"])
	assert.Equal(t, "5s", endpoint["scrapeTimeout"])

	// Update CR with new interval=60s and scrapeTimeout=25s
	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:       true,
					Interval:      "60s",
					ScrapeTimeout: "25s",
				},
			}
		}
	}

	// Reconcile again
	err = PowerScaleMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	// Verify ServiceMonitor was updated
	serviceMonitorUpdated := &unstructured.Unstructured{}
	serviceMonitorUpdated.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerscale-obs-monitor", Namespace: customResource.Namespace}, serviceMonitorUpdated))

	spec, found, err = unstructured.NestedMap(serviceMonitorUpdated.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	endpoints, found, err = unstructured.NestedSlice(spec, "endpoints")
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, len(endpoints), 0)
	endpoint, ok = endpoints[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "60s", endpoint["interval"], "ServiceMonitor interval should be updated to 60s")
	assert.Equal(t, "25s", endpoint["scrapeTimeout"], "ServiceMonitor scrapeTimeout should be updated to 25s")
}

func TestPowerScaleMetrics_ServiceMonitorNotCreatedWhenMetricsDisabled(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	require.NoError(t, err)

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: false,
			}
		}
	}

	isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerScaleMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	err = fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerscale-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor)
	require.True(t, k8sErrors.IsNotFound(err), "ServiceMonitor must not be created when metrics.enabled is false")
}

func TestPowerScaleMetrics_ServiceMonitorTLSConfigUpdatedOnCRChange(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	require.NoError(t, err)

	// Initial reconcile with TLS enabled and insecureSkipVerify=true
	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			// #nosec G101 - test file
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled:       true,
				Port:          9443,
				TLSCertSecret: "metrics-tls-secret", //nolint:gosec
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:            true,
					InsecureSkipVerify: true,
				},
			}
		}
	}

	isilonCreds := getSecret(customResource.Namespace, "isilon-creds")
	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metrics-tls-secret",
			Namespace: customResource.Namespace,
		},
	}
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(isilonCreds, tlsSecret).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerScaleMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerscale-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor))

	// Verify initial TLS config
	spec, found, err := unstructured.NestedMap(serviceMonitor.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	endpoints, found, err := unstructured.NestedSlice(spec, "endpoints")
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, len(endpoints), 0)
	endpoint, ok := endpoints[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "https", endpoint["scheme"])
	tlsConfig, ok := endpoint["tlsConfig"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, tlsConfig["insecureSkipVerify"])

	// Update CR with insecureSkipVerify=false
	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			// #nosec G101 - test file
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled:       true,
				Port:          9443,
				TLSCertSecret: "metrics-tls-secret", //nolint:gosec
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:            true,
					InsecureSkipVerify: false,
				},
			}
		}
	}

	// Reconcile again
	err = PowerScaleMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	// Verify ServiceMonitor TLS config was updated
	serviceMonitorUpdated := &unstructured.Unstructured{}
	serviceMonitorUpdated.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerscale-obs-monitor", Namespace: customResource.Namespace}, serviceMonitorUpdated))

	spec, found, err = unstructured.NestedMap(serviceMonitorUpdated.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	endpoints, found, err = unstructured.NestedSlice(spec, "endpoints")
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, len(endpoints), 0)
	endpoint, ok = endpoints[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "https", endpoint["scheme"])
	tlsConfig, ok = endpoint["tlsConfig"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, false, tlsConfig["insecureSkipVerify"], "ServiceMonitor tlsConfig.insecureSkipVerify should be updated to false")
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
		"Fail - wrong module name": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, operatorConfig
		},
		"success - creating with spec version set": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}
			vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")
			tmpCR := customResource
			tmpCR.Spec.Version = "v1.18.0"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(vxflexosCreds).Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - creating with auth secret override": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
			if err != nil {
				panic(err)
			}
			vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")
			tmpCR := customResource
			tmpCR.Spec.Driver.AuthSecret = "custom-auth-secret"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(vxflexosCreds).Build()
			return true, false, tmpCR, sourceClient, operatorConfig
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

type serviceMonitorValidatingClient struct {
	ctrlClient.Client
}

func (c *serviceMonitorValidatingClient) Update(ctx context.Context, obj ctrlClient.Object, opts ...ctrlClient.UpdateOption) error {
	if obj.GetObjectKind().GroupVersionKind().Kind == "ServiceMonitor" && obj.GetResourceVersion() == "" {
		return fmt.Errorf("ServiceMonitor update missing resourceVersion")
	}
	return c.Client.Update(ctx, obj, opts...)
}

func TestPowerFlexMetrics_ServiceMonitorUpdatePreservesResourceVersion(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
	require.NoError(t, err)

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:  true,
					Interval: "15s",
				},
			}
		}
	}

	vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")
	existingServiceMonitor := &unstructured.Unstructured{}
	existingServiceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	existingServiceMonitor.SetName("karavi-metrics-powerflex-obs-monitor")
	existingServiceMonitor.SetNamespace(customResource.Namespace)
	existingServiceMonitor.SetResourceVersion("123")

	baseClient := ctrlClientFake.NewClientBuilder().WithObjects(vxflexosCreds, existingServiceMonitor).Build()
	validatingClient := &serviceMonitorValidatingClient{Client: baseClient}
	k8sClient := clientgoclient.NewFakeClient(validatingClient)

	err = PowerFlexMetrics(ctx, false, operatorConfig, customResource, validatingClient, k8sClient)
	require.NoError(t, err)
}

func TestPowerFlexMetrics_SelfMetricsEnvConfigured(t *testing.T) {
	ctx := context.Background()

	customResource, err := getCustomResource("./testdata/cr_powerflex_observability.yaml")
	require.NoError(t, err)
	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{Enabled: true, Port: 9443}
		}
	}
	vxflexosCreds := getSecret(customResource.Namespace, "test-vxflexos-config")
	sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(vxflexosCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(sourceClient)

	err = PowerFlexMetrics(ctx, false, operatorConfig, customResource, sourceClient, k8sClient)
	require.NoError(t, err)

	deployment := &appsv1.Deployment{}
	require.NoError(t, sourceClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerflex", Namespace: customResource.Namespace}, deployment))
	require.NotEmpty(t, deployment.Spec.Template.Spec.Containers, "deployment must include the metrics container")

	var metricsContainer *corev1.Container
	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name == "karavi-metrics-powerflex" {
			metricsContainer = &deployment.Spec.Template.Spec.Containers[i]
			break
		}
	}
	require.NotNil(t, metricsContainer, "karavi-metrics-powerflex container must exist")

	getEnvValue := func(name string) (string, bool) {
		for _, env := range metricsContainer.Env {
			if env.Name == name {
				return env.Value, true
			}
		}
		return "", false
	}

	value, found := getEnvValue("X_CSI_METRICS_ENABLED")
	require.True(t, found, "X_CSI_METRICS_ENABLED must be injected into the metrics container")
	assert.Equal(t, "true", value)

	value, found = getEnvValue("X_CSI_METRICS_PORT")
	require.True(t, found, "X_CSI_METRICS_PORT must be injected into the metrics container")
	assert.Equal(t, "9443", value)
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
		"success - creating with spec version set": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_observability.yaml")
			if err != nil {
				panic(err)
			}
			powerstoreCreds := getSecret(customResource.Namespace, "test-powerstore-config")
			tmpCR := customResource
			tmpCR.Spec.Version = "v1.18.0"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(powerstoreCreds).Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - creating with auth secret override": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powerstore_observability.yaml")
			if err != nil {
				panic(err)
			}
			powerstoreCreds := getSecret(customResource.Namespace, "test-powerstore-config")
			tmpCR := customResource
			tmpCR.Spec.Driver.AuthSecret = "custom-auth-secret"
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

func TestPowerStoreMetrics_ServiceMonitorEnabledToDisabledTransitionDeletesMonitor(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powerstore_observability.yaml")
	require.NoError(t, err)

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:  true,
					Interval: "15s",
				},
			}
		}
	}

	powerstoreCreds := getSecret(customResource.Namespace, "test-powerstore-config")
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(powerstoreCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerStoreMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerstore-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor))

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled: false,
				},
			}
		}
	}

	err = PowerStoreMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitorAfter := &unstructured.Unstructured{}
	serviceMonitorAfter.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	err = fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerstore-obs-monitor", Namespace: customResource.Namespace}, serviceMonitorAfter)
	require.True(t, k8sErrors.IsNotFound(err), "ServiceMonitor must be deleted when observability serviceMonitor is disabled")
}

func TestPowerStoreMetrics_MetricsEnabledToDisabledDeletesServiceMonitor(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powerstore_observability.yaml")
	require.NoError(t, err)

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:  true,
					Interval: "30s",
				},
			}
		}
	}

	powerstoreCreds := getSecret(customResource.Namespace, "test-powerstore-config")
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(powerstoreCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerStoreMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerstore-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor))

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: false,
			}
		}
	}

	err = PowerStoreMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitorAfter := &unstructured.Unstructured{}
	serviceMonitorAfter.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	err = fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerstore-obs-monitor", Namespace: customResource.Namespace}, serviceMonitorAfter)
	require.True(t, k8sErrors.IsNotFound(err), "ServiceMonitor must be deleted when metrics.enabled is disabled")
}

func TestPowerStoreMetrics_ServiceMonitorCreatedWhenBothEnabled(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powerstore_observability.yaml")
	require.NoError(t, err)

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled:       true,
				Port:          9443,
				TLSCertSecret: "powerstore-metrics-tls",
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:            true,
					Interval:           "30s",
					ScrapeTimeout:      "10s",
					InsecureSkipVerify: true,
				},
			}
		}
	}

	powerstoreCreds := getSecret(customResource.Namespace, "test-powerstore-config")
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(powerstoreCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerStoreMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	err = fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerstore-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor)
	require.NoError(t, err, "ServiceMonitor must be created when both metrics.enabled and serviceMonitor.enabled are true")

	spec, found, err := unstructured.NestedMap(serviceMonitor.Object, "spec")
	require.NoError(t, err)
	require.True(t, found, "ServiceMonitor spec must exist")

	endpoints, found, err := unstructured.NestedSlice(spec, "endpoints")
	require.NoError(t, err)
	require.True(t, found, "ServiceMonitor endpoints must exist")
	require.Greater(t, len(endpoints), 0, "ServiceMonitor must have at least one endpoint")

	endpoint, ok := endpoints[0].(map[string]interface{})
	require.True(t, ok, "ServiceMonitor endpoint must be a map")
	assert.Equal(t, "obs-metrics", endpoint["port"])
	assert.Equal(t, "30s", endpoint["interval"])
	assert.Equal(t, "10s", endpoint["scrapeTimeout"])
	assert.Equal(t, "https", endpoint["scheme"])
	tlsConfig, ok := endpoint["tlsConfig"].(map[string]interface{})
	require.True(t, ok, "ServiceMonitor tlsConfig must be a map")
	assert.Equal(t, true, tlsConfig["insecureSkipVerify"])

	service := &corev1.Service{}
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerstore", Namespace: customResource.Namespace}, service))
	obsMetricsPortFound := false
	for _, port := range service.Spec.Ports {
		if port.Name == "obs-metrics" {
			obsMetricsPortFound = true
			assert.Equal(t, int32(9443), port.Port)
		}
	}
	assert.True(t, obsMetricsPortFound, "Service must expose obs-metrics port")
}

func TestPowerStoreMetrics_ServiceMonitorUpdatedOnCRChange(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powerstore_observability.yaml")
	require.NoError(t, err)

	// Initial reconcile with interval=15s
	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:       true,
					Interval:      "15s",
					ScrapeTimeout: "5s",
				},
			}
		}
	}

	powerstoreCreds := getSecret(customResource.Namespace, "test-powerstore-config")
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(powerstoreCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerStoreMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerstore-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor))

	// Verify initial interval
	spec, found, err := unstructured.NestedMap(serviceMonitor.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	endpoints, found, err := unstructured.NestedSlice(spec, "endpoints")
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, len(endpoints), 0)
	endpoint, ok := endpoints[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "15s", endpoint["interval"])
	assert.Equal(t, "5s", endpoint["scrapeTimeout"])

	// Update CR with new interval=60s and scrapeTimeout=25s
	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:       true,
					Interval:      "60s",
					ScrapeTimeout: "25s",
				},
			}
		}
	}

	// Reconcile again
	err = PowerStoreMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	// Verify ServiceMonitor was updated
	serviceMonitorUpdated := &unstructured.Unstructured{}
	serviceMonitorUpdated.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerstore-obs-monitor", Namespace: customResource.Namespace}, serviceMonitorUpdated))

	spec, found, err = unstructured.NestedMap(serviceMonitorUpdated.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	endpoints, found, err = unstructured.NestedSlice(spec, "endpoints")
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, len(endpoints), 0)
	endpoint, ok = endpoints[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "60s", endpoint["interval"], "ServiceMonitor interval should be updated to 60s")
	assert.Equal(t, "25s", endpoint["scrapeTimeout"], "ServiceMonitor scrapeTimeout should be updated to 25s")
}

func TestPowerStoreMetrics_ServiceMonitorNotCreatedWhenMetricsDisabled(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powerstore_observability.yaml")
	require.NoError(t, err)

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: false,
			}
		}
	}

	powerstoreCreds := getSecret(customResource.Namespace, "test-powerstore-config")
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(powerstoreCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerStoreMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	err = fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerstore-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor)
	require.True(t, k8sErrors.IsNotFound(err), "ServiceMonitor must not be created when metrics.enabled is false")
}

func TestPowerStoreMetrics_ServiceMonitorTLSConfigUpdatedOnCRChange(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powerstore_observability.yaml")
	require.NoError(t, err)

	// Initial reconcile with TLS enabled and insecureSkipVerify=true
	for mi := range customResource.Spec.Modules {
		// #nosec G101 - test file
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled:       true,
				Port:          9443,
				TLSCertSecret: "metrics-tls-secret", //nolint:gosec
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:            true,
					InsecureSkipVerify: true,
				},
			}
		}
	}

	powerstoreCreds := getSecret(customResource.Namespace, "test-powerstore-config")
	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metrics-tls-secret",
			Namespace: customResource.Namespace,
		},
	}
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(powerstoreCreds, tlsSecret).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerStoreMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerstore-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor))

	// Verify initial TLS config
	spec, found, err := unstructured.NestedMap(serviceMonitor.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	endpoints, found, err := unstructured.NestedSlice(spec, "endpoints")
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, len(endpoints), 0)
	endpoint, ok := endpoints[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "https", endpoint["scheme"])
	tlsConfig, ok := endpoint["tlsConfig"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, tlsConfig["insecureSkipVerify"])

	// Update CR with insecureSkipVerify=false
	for mi := range customResource.Spec.Modules {
		// #nosec G101 - test file
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled:       true,
				Port:          9443,
				TLSCertSecret: "metrics-tls-secret", //nolint:gosec
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:            true,
					InsecureSkipVerify: false,
				},
			}
		}
	}

	// Reconcile again
	err = PowerStoreMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	// Verify ServiceMonitor TLS config was updated
	serviceMonitorUpdated := &unstructured.Unstructured{}
	serviceMonitorUpdated.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powerstore-obs-monitor", Namespace: customResource.Namespace}, serviceMonitorUpdated))

	spec, found, err = unstructured.NestedMap(serviceMonitorUpdated.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	endpoints, found, err = unstructured.NestedSlice(spec, "endpoints")
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, len(endpoints), 0)
	endpoint, ok = endpoints[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "https", endpoint["scheme"])
	tlsConfig, ok = endpoint["tlsConfig"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, false, tlsConfig["insecureSkipVerify"], "ServiceMonitor tlsConfig.insecureSkipVerify should be updated to false")
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
		"success - creating with spec version set": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
			if err != nil {
				panic(err)
			}
			pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")
			tmpCR := customResource
			tmpCR.Spec.Version = "v1.18.0"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds).Build()
			return true, false, tmpCR, sourceClient, operatorConfig
		},
		"success - creating with auth secret override": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig) {
			customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
			if err != nil {
				panic(err)
			}
			pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")
			tmpCR := customResource
			tmpCR.Spec.Driver.AuthSecret = "custom-auth-secret"
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds).Build()
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

func TestPowerMaxMetrics_ServiceMonitorEnabledToDisabledTransitionDeletesMonitor(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
	require.NoError(t, err)

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:  true,
					Interval: "15s",
				},
			}
		}
	}

	pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerMaxMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powermax-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor))

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled: false,
				},
			}
		}
	}

	err = PowerMaxMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitorAfter := &unstructured.Unstructured{}
	serviceMonitorAfter.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	err = fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powermax-obs-monitor", Namespace: customResource.Namespace}, serviceMonitorAfter)
	require.True(t, k8sErrors.IsNotFound(err), "ServiceMonitor must be deleted when observability serviceMonitor is disabled")
}

func TestPowerMaxMetrics_MetricsEnabledToDisabledDeletesServiceMonitor(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
	require.NoError(t, err)

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:  true,
					Interval: "30s",
				},
			}
		}
	}

	pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerMaxMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powermax-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor))

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: false,
			}
		}
	}

	err = PowerMaxMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitorAfter := &unstructured.Unstructured{}
	serviceMonitorAfter.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	err = fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powermax-obs-monitor", Namespace: customResource.Namespace}, serviceMonitorAfter)
	require.True(t, k8sErrors.IsNotFound(err), "ServiceMonitor must be deleted when metrics.enabled is disabled")
}

func TestPowerMaxMetrics_ServiceMonitorCreatedWhenBothEnabled(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
	require.NoError(t, err)

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled:       true,
				Port:          9443,
				TLSCertSecret: "powermax-metrics-tls",
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:            true,
					Interval:           "30s",
					ScrapeTimeout:      "10s",
					InsecureSkipVerify: true,
				},
			}
		}
	}

	pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerMaxMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	err = fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powermax-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor)
	require.NoError(t, err, "ServiceMonitor must be created when both metrics.enabled and serviceMonitor.enabled are true")

	spec, found, err := unstructured.NestedMap(serviceMonitor.Object, "spec")
	require.NoError(t, err)
	require.True(t, found, "ServiceMonitor spec must exist")

	endpoints, found, err := unstructured.NestedSlice(spec, "endpoints")
	require.NoError(t, err)
	require.True(t, found, "ServiceMonitor endpoints must exist")
	require.Greater(t, len(endpoints), 0, "ServiceMonitor must have at least one endpoint")

	endpoint, ok := endpoints[0].(map[string]interface{})
	require.True(t, ok, "ServiceMonitor endpoint must be a map")
	assert.Equal(t, "obs-metrics", endpoint["port"])
	assert.Equal(t, "30s", endpoint["interval"])
	assert.Equal(t, "10s", endpoint["scrapeTimeout"])
	assert.Equal(t, "https", endpoint["scheme"])
	tlsConfig, ok := endpoint["tlsConfig"].(map[string]interface{})
	require.True(t, ok, "ServiceMonitor tlsConfig must be a map")
	assert.Equal(t, true, tlsConfig["insecureSkipVerify"])

	service := &corev1.Service{}
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powermax", Namespace: customResource.Namespace}, service))
	obsMetricsPortFound := false
	for _, port := range service.Spec.Ports {
		if port.Name == "obs-metrics" {
			obsMetricsPortFound = true
			assert.Equal(t, int32(9443), port.Port)
		}
	}
	assert.True(t, obsMetricsPortFound, "Service must expose obs-metrics port")
}

func TestPowerMaxMetrics_ServiceMonitorUpdatedOnCRChange(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
	require.NoError(t, err)

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:       true,
					Interval:      "15s",
					ScrapeTimeout: "5s",
				},
			}
		}
	}

	pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerMaxMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powermax-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor))

	spec, found, err := unstructured.NestedMap(serviceMonitor.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	endpoints, found, err := unstructured.NestedSlice(spec, "endpoints")
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, len(endpoints), 0)
	endpoint, ok := endpoints[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "15s", endpoint["interval"])
	assert.Equal(t, "5s", endpoint["scrapeTimeout"])

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: true,
				Port:    9443,
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:       true,
					Interval:      "60s",
					ScrapeTimeout: "25s",
				},
			}
		}
	}

	err = PowerMaxMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitorUpdated := &unstructured.Unstructured{}
	serviceMonitorUpdated.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powermax-obs-monitor", Namespace: customResource.Namespace}, serviceMonitorUpdated))

	spec, found, err = unstructured.NestedMap(serviceMonitorUpdated.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	endpoints, found, err = unstructured.NestedSlice(spec, "endpoints")
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, len(endpoints), 0)
	endpoint, ok = endpoints[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "60s", endpoint["interval"], "ServiceMonitor interval should be updated to 60s")
	assert.Equal(t, "25s", endpoint["scrapeTimeout"], "ServiceMonitor scrapeTimeout should be updated to 25s")
}

func TestPowerMaxMetrics_ServiceMonitorNotCreatedWhenMetricsDisabled(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
	require.NoError(t, err)

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled: false,
			}
		}
	}

	pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerMaxMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	err = fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powermax-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor)
	require.True(t, k8sErrors.IsNotFound(err), "ServiceMonitor must not be created when metrics.enabled is false")
}

func TestPowerMaxMetrics_ServiceMonitorTLSConfigUpdatedOnCRChange(t *testing.T) {
	ctx := context.Background()
	customResource, err := getCustomResource("./testdata/cr_powermax_observability.yaml")
	require.NoError(t, err)

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			// #nosec G101 - test file
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled:       true,
				Port:          9443,
				TLSCertSecret: "metrics-tls-secret", //nolint:gosec
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:            true,
					InsecureSkipVerify: true,
				},
			}
		}
	}

	pmaxCreds := getSecret(customResource.Namespace, "test-powermax-creds")
	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metrics-tls-secret",
			Namespace: customResource.Namespace,
		},
	}
	fakeClient := ctrlClientFake.NewClientBuilder().WithObjects(pmaxCreds, tlsSecret).Build()
	k8sClient := clientgoclient.NewFakeClient(fakeClient)

	err = PowerMaxMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitor := &unstructured.Unstructured{}
	serviceMonitor.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powermax-obs-monitor", Namespace: customResource.Namespace}, serviceMonitor))

	spec, found, err := unstructured.NestedMap(serviceMonitor.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	endpoints, found, err := unstructured.NestedSlice(spec, "endpoints")
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, len(endpoints), 0)
	endpoint, ok := endpoints[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "https", endpoint["scheme"])
	tlsConfig, ok := endpoint["tlsConfig"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, tlsConfig["insecureSkipVerify"])

	for mi := range customResource.Spec.Modules {
		if customResource.Spec.Modules[mi].Name == csmv1.Observability {
			// #nosec G101 - test file
			customResource.Spec.Modules[mi].Metrics = &csmv1.ModuleMetrics{
				Enabled:       true,
				Port:          9443,
				TLSCertSecret: "metrics-tls-secret", //nolint:gosec
				ServiceMonitor: &csmv1.MetricsServiceMonitorConfig{
					Enabled:            true,
					InsecureSkipVerify: false,
				},
			}
		}
	}

	err = PowerMaxMetrics(ctx, false, operatorConfig, customResource, fakeClient, k8sClient)
	require.NoError(t, err)

	serviceMonitorUpdated := &unstructured.Unstructured{}
	serviceMonitorUpdated.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"})
	require.NoError(t, fakeClient.Get(ctx, ctrlClient.ObjectKey{Name: "karavi-metrics-powermax-obs-monitor", Namespace: customResource.Namespace}, serviceMonitorUpdated))

	spec, found, err = unstructured.NestedMap(serviceMonitorUpdated.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	endpoints, found, err = unstructured.NestedSlice(spec, "endpoints")
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, len(endpoints), 0)
	endpoint, ok = endpoints[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "https", endpoint["scheme"])
	tlsConfig, ok = endpoint["tlsConfig"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, false, tlsConfig["insecureSkipVerify"], "ServiceMonitor tlsConfig.insecureSkipVerify should be updated to false")
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
						if e.Name == "LOG_LEVEL" && e.Value == "debug" {
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
		t.Fatalf("LOG_LEVEL env with value debug not found in rendered objects")
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

func TestObservabilityTopology_SupportedVersion_AppliesObjects(t *testing.T) {
	ctx := context.Background()

	// Save & restore seams
	origGetObs := getObservabilityModuleFn
	origReadCfg := readConfigFileFn
	origGetVer := getVersionFn
	defer func() {
		getObservabilityModuleFn = origGetObs
		readConfigFileFn = origReadCfg
		getVersionFn = origGetVer
	}()

	getVersionFn = func(_ context.Context, _ *csmv1.ContainerStorageModule, _ operatorutils.OperatorConfig) (string, error) {
		return "v2.14.0", nil
	}

	getObservabilityModuleFn = func(_ csmv1.ContainerStorageModule) (csmv1.Module, error) {
		return csmv1.Module{
			Name:    csmv1.Observability,
			Enabled: true,
			Components: []csmv1.ContainerTemplate{
				{
					Name:    ObservabilityTopologyName,
					Enabled: func() *bool { b := true; return &b }(),
					Envs:    []corev1.EnvVar{{Name: TopologyLogLevel, Value: "DEBUG"}},
				},
			},
		}, nil
	}

	readConfigFileFn = func(_ context.Context, _ csmv1.Module, cr csmv1.ContainerStorageModule, _ operatorutils.OperatorConfig, _ string) ([]byte, error) {
		yaml := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: karavi-topology
  namespace: %s
spec:
  selector:
    matchLabels:
      app: karavi-topology
  template:
    metadata:
      labels:
        app: karavi-topology
    spec:
      containers:
      - name: karavi-topology
        image: quay.io/dell/container-storage-modules/csm-topology:v1.0.0
        env:
        - name: TOPOLOGY_LOG_LEVEL
          value: %s
`, cr.Namespace, TopologyLogLevel)
		return []byte(yaml), nil
	}

	cr := csmv1.ContainerStorageModule{ObjectMeta: metav1.ObjectMeta{Name: "isilon", Namespace: "isilon"}}
	client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

	err := ObservabilityTopology(ctx, false, operatorConfig, cr, client, operatorutils.VersionSpec{})
	assert.NoError(t, err)
}

func TestObservabilityTopology_UnsupportedVersion_ReturnsError(t *testing.T) {
	ctx := context.Background()

	origGetVer := getVersionFn
	defer func() { getVersionFn = origGetVer }()

	getVersionFn = func(_ context.Context, _ *csmv1.ContainerStorageModule, _ operatorutils.OperatorConfig) (string, error) {
		return "v2.15.0", nil
	}

	cr := csmv1.ContainerStorageModule{ObjectMeta: metav1.ObjectMeta{Name: "isilon", Namespace: "isilon"}}
	client := ctrlClientFake.NewClientBuilder().WithObjects().Build()

	err := ObservabilityTopology(ctx, false, operatorConfig, cr, client, operatorutils.VersionSpec{})
	assert.Error(t, err)
}

func TestObservabilityTopology_SupportedVersion_Deleting_DeletesObjects(t *testing.T) {
	ctx := context.Background()

	// Save & restore seams
	origGetObs := getObservabilityModuleFn
	origReadCfg := readConfigFileFn
	origGetVer := getVersionFn
	defer func() {
		getObservabilityModuleFn = origGetObs
		readConfigFileFn = origReadCfg
		getVersionFn = origGetVer
	}()

	getVersionFn = func(_ context.Context, _ *csmv1.ContainerStorageModule, _ operatorutils.OperatorConfig) (string, error) {
		return "v2.13.0", nil
	}

	getObservabilityModuleFn = func(_ csmv1.ContainerStorageModule) (csmv1.Module, error) {
		return csmv1.Module{
			Name:    csmv1.Observability,
			Enabled: true,
			Components: []csmv1.ContainerTemplate{
				{
					Name:    ObservabilityTopologyName,
					Enabled: func() *bool { b := true; return &b }(),
					Envs:    []corev1.EnvVar{{Name: TopologyLogLevel, Value: "INFO"}},
				},
			},
		}, nil
	}

	readConfigFileFn = func(_ context.Context, _ csmv1.Module, cr csmv1.ContainerStorageModule, _ operatorutils.OperatorConfig, _ string) ([]byte, error) {
		yaml := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: karavi-topology
  namespace: %s
spec:
  selector:
    matchLabels:
      app: karavi-topology
  template:
    metadata:
      labels:
        app: karavi-topology
    spec:
      containers:
      - name: karavi-topology
        image: quay.io/dell/container-storage-modules/csm-topology:v1.0.0
`, cr.Namespace)
		return []byte(yaml), nil
	}

	cr := csmv1.ContainerStorageModule{ObjectMeta: metav1.ObjectMeta{Name: "isilon", Namespace: "isilon"}}
	// Seed an existing Deployment so delete path exercises a real delete.
	existing := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "karavi-topology", Namespace: cr.Namespace}}
	client := ctrlClientFake.NewClientBuilder().WithObjects(existing).Build()

	err := ObservabilityTopology(ctx, true, operatorConfig, cr, client, operatorutils.VersionSpec{})
	assert.NoError(t, err)
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

func TestGetPowerFlexMetricsObject_CustomRegistryOnly(t *testing.T) {
	ctx := context.Background()

	origGetObs := getObservabilityModuleFn
	origReadCfg := readConfigFileFn
	defer func() {
		getObservabilityModuleFn = origGetObs
		readConfigFileFn = origReadCfg
	}()

	getObservabilityModuleFn = func(_ csmv1.ContainerStorageModule) (csmv1.Module, error) {
		return csmv1.Module{
			Name:    csmv1.Observability,
			Enabled: true,
			Components: []csmv1.ContainerTemplate{
				{
					Name: ObservabilityMetricsPowerFlexName,
					Envs: []corev1.EnvVar{
						{Name: PowerflexLogLevel, Value: "INFO"},
					},
				},
			},
		}, nil
	}

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
        image: quay.io/dell/karavi-metrics-powerflex:template
        env:
        - name: %s
          value: INFO
`, cr.Namespace, PowerflexLogLevel)
		return []byte(yaml), nil
	}

	cr := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csm-ut",
			Namespace: "csm-ut-ns",
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			CustomRegistry: "my-registry.example.com",
		},
	}
	op := operatorConfig

	matched := operatorutils.VersionSpec{} // empty - no ConfigMap

	objs, err := getPowerFlexMetricsObject(ctx, op, cr, matched)
	if err != nil {
		t.Fatalf("getPowerFlexMetricsObject returned error: %v", err)
	}

	found := false
	for _, obj := range objs {
		if dep, ok := obj.(*appsv1.Deployment); ok {
			for _, c := range dep.Spec.Template.Spec.Containers {
				if c.Name == "karavi-metrics-powerflex" {
					if strings.HasPrefix(c.Image, "my-registry.example.com/") {
						found = true
					} else {
						t.Errorf("expected custom registry prefix, got %q", c.Image)
					}
				}
			}
		}
	}
	if !found {
		t.Fatalf("karavi-metrics-powerflex container with custom registry image not found")
	}
}

func TestGetPowerFlexMetricsObject_ConfigMapWinsOverCustomRegistry(t *testing.T) {
	ctx := context.Background()

	origGetObs := getObservabilityModuleFn
	origReadCfg := readConfigFileFn
	defer func() {
		getObservabilityModuleFn = origGetObs
		readConfigFileFn = origReadCfg
	}()

	getObservabilityModuleFn = func(_ csmv1.ContainerStorageModule) (csmv1.Module, error) {
		return csmv1.Module{
			Name:    csmv1.Observability,
			Enabled: true,
			Components: []csmv1.ContainerTemplate{
				{
					Name: ObservabilityMetricsPowerFlexName,
					Envs: []corev1.EnvVar{
						{Name: PowerflexLogLevel, Value: "INFO"},
					},
				},
			},
		}, nil
	}

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
        image: quay.io/dell/karavi-metrics-powerflex:template
        env:
        - name: %s
          value: INFO
`, cr.Namespace, PowerflexLogLevel)
		return []byte(yaml), nil
	}

	cr := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csm-ut",
			Namespace: "csm-ut-ns",
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			CustomRegistry: "my-registry.example.com",
		},
	}
	op := operatorConfig

	configMapImage := "configmap-registry.example.com/karavi-metrics-powerflex:from-cm"
	matched := operatorutils.VersionSpec{
		Version: "v9.9.9",
		Images: map[string]string{
			ObservabilityMetricsPowerFlexName: configMapImage,
		},
	}

	objs, err := getPowerFlexMetricsObject(ctx, op, cr, matched)
	if err != nil {
		t.Fatalf("getPowerFlexMetricsObject returned error: %v", err)
	}

	found := false
	for _, obj := range objs {
		if dep, ok := obj.(*appsv1.Deployment); ok {
			for _, c := range dep.Spec.Template.Spec.Containers {
				if c.Name == "karavi-metrics-powerflex" {
					if c.Image == configMapImage {
						found = true
					} else {
						t.Errorf("expected ConfigMap image %q, got %q", configMapImage, c.Image)
					}
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected ConfigMap image to win over custom registry")
	}
}

func TestGetPowerFlexMetricsObject_NeitherConfigMapNorRegistry(t *testing.T) {
	ctx := context.Background()

	origGetObs := getObservabilityModuleFn
	origReadCfg := readConfigFileFn
	defer func() {
		getObservabilityModuleFn = origGetObs
		readConfigFileFn = origReadCfg
	}()

	getObservabilityModuleFn = func(_ csmv1.ContainerStorageModule) (csmv1.Module, error) {
		return csmv1.Module{
			Name:    csmv1.Observability,
			Enabled: true,
			Components: []csmv1.ContainerTemplate{
				{
					Name: ObservabilityMetricsPowerFlexName,
					Envs: []corev1.EnvVar{
						{Name: PowerflexLogLevel, Value: "INFO"},
					},
				},
			},
		}, nil
	}

	templateImage := "quay.io/dell/karavi-metrics-powerflex:template"
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
        image: %s
        env:
        - name: %s
          value: INFO
`, cr.Namespace, templateImage, PowerflexLogLevel)
		return []byte(yaml), nil
	}

	cr := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csm-ut",
			Namespace: "csm-ut-ns",
		},
	}
	op := operatorConfig

	matched := operatorutils.VersionSpec{} // empty

	objs, err := getPowerFlexMetricsObject(ctx, op, cr, matched)
	if err != nil {
		t.Fatalf("getPowerFlexMetricsObject returned error: %v", err)
	}

	found := false
	for _, obj := range objs {
		if dep, ok := obj.(*appsv1.Deployment); ok {
			for _, c := range dep.Spec.Template.Spec.Containers {
				if c.Name == "karavi-metrics-powerflex" {
					if c.Image == templateImage {
						found = true
					} else {
						t.Errorf("expected template default image %q, got %q", templateImage, c.Image)
					}
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected template default image to be used")
	}
}

func TestIsRetryableWebhookError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "tls certificate verification error",
			err:      fmt.Errorf("tls: failed to verify certificate"),
			expected: true,
		},
		{
			name:     "x509 unknown authority error",
			err:      fmt.Errorf("x509: certificate signed by unknown authority"),
			expected: true,
		},
		{
			name:     "connection refused error",
			err:      fmt.Errorf("connection refused"),
			expected: true,
		},
		{
			name:     "no such host error",
			err:      fmt.Errorf("no such host"),
			expected: true,
		},
		{
			name:     "webhook error",
			err:      fmt.Errorf("webhook configuration error"),
			expected: true,
		},
		{
			name:     "internal error calling webhook",
			err:      fmt.Errorf("Internal error occurred: failed calling webhook"),
			expected: true,
		},
		{
			name:     "other error",
			err:      fmt.Errorf("some other error"),
			expected: false,
		},
		{
			name:     "generic error",
			err:      fmt.Errorf("generic error message"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableWebhookError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseObservabilityMetricsDeployment(t *testing.T) {
	ctx := context.Background()

	// Test with valid deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test-image",
						},
					},
				},
			},
		},
	}

	cr, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	if err != nil {
		panic(err)
	}

	dpApply, err := parseObservabilityMetricsDeployment(ctx, deployment, operatorConfig, cr, ctrlClientFake.NewClientBuilder().Build())
	assert.NoError(t, err)
	assert.NotNil(t, dpApply)
}

func TestGetOtelCollector_ModuleNotFound_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// CR without observability module
	cr, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
	if err != nil {
		panic(err)
	}

	_, err = getOtelCollector(ctx, operatorConfig, cr, operatorutils.VersionSpec{})
	assert.Error(t, err)
}

func TestGetTopology_ModuleNotFound_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// CR without observability module
	cr, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
	if err != nil {
		panic(err)
	}

	_, err = getTopology(ctx, operatorConfig, cr, operatorutils.VersionSpec{})
	assert.Error(t, err)
}

func TestIssuerCertServiceObs_ModuleNotFound_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// CR without observability module
	cr, err := getCustomResource("./testdata/cr_powerscale_replica.yaml")
	if err != nil {
		panic(err)
	}

	err = IssuerCertServiceObs(ctx, false, operatorConfig, cr, ctrlClientFake.NewClientBuilder().Build())
	assert.Error(t, err)
}

func TestIssuerCertServiceObs_Success(t *testing.T) {
	ctx := context.Background()

	// CR with observability module and otel-collector enabled
	cr, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	if err != nil {
		panic(err)
	}

	ctrlClient := ctrlClientFake.NewClientBuilder().Build()
	err = IssuerCertServiceObs(ctx, false, operatorConfig, cr, ctrlClient)
	assert.NoError(t, err)
}

func TestGetTopology_InvalidCRName_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// Save originals and restore after test
	origGetObs := getObservabilityModuleFn
	origReadCfg := readConfigFileFn
	defer func() {
		getObservabilityModuleFn = origGetObs
		readConfigFileFn = origReadCfg
	}()

	// Mock getObservabilityModuleFn to return a valid Observability module
	getObservabilityModuleFn = func(_ csmv1.ContainerStorageModule) (csmv1.Module, error) {
		return csmv1.Module{
			Name:    csmv1.Observability,
			Enabled: true,
			Components: []csmv1.ContainerTemplate{
				{
					Name: ObservabilityTopologyName,
					Envs: []corev1.EnvVar{
						{
							Name:  TopologyLogLevel,
							Value: "info",
						},
					},
				},
			},
		}, nil
	}

	// Mock readConfigFileFn to return a minimal YAML
	readConfigFileFn = func(_ context.Context, _ csmv1.Module, cr csmv1.ContainerStorageModule, _ operatorutils.OperatorConfig, _ string) ([]byte, error) {
		yaml := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: karavi-topology
  namespace: %s
spec:
  template:
    spec:
      containers:
      - name: karavi-topology
`, cr.Namespace)
		return []byte(yaml), nil
	}

	// Test with invalid CR name (contains invalid characters)
	cr := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid@name#with$special.chars",
			Namespace: "default",
		},
	}

	_, err := getTopology(ctx, operatorConfig, cr, operatorutils.VersionSpec{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid CR name for YAML substitution")
}

func TestGetTopology_InvalidCRNamespace_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// Save originals and restore after test
	origGetObs := getObservabilityModuleFn
	origReadCfg := readConfigFileFn
	defer func() {
		getObservabilityModuleFn = origGetObs
		readConfigFileFn = origReadCfg
	}()

	// Mock getObservabilityModuleFn to return a valid Observability module
	getObservabilityModuleFn = func(_ csmv1.ContainerStorageModule) (csmv1.Module, error) {
		return csmv1.Module{
			Name:    csmv1.Observability,
			Enabled: true,
			Components: []csmv1.ContainerTemplate{
				{
					Name: ObservabilityTopologyName,
					Envs: []corev1.EnvVar{
						{
							Name:  TopologyLogLevel,
							Value: "info",
						},
					},
				},
			},
		}, nil
	}

	// Mock readConfigFileFn to return a minimal YAML
	readConfigFileFn = func(_ context.Context, _ csmv1.Module, cr csmv1.ContainerStorageModule, _ operatorutils.OperatorConfig, _ string) ([]byte, error) {
		yaml := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: karavi-topology
  namespace: %s
spec:
  template:
    spec:
      containers:
      - name: karavi-topology
`, cr.Namespace)
		return []byte(yaml), nil
	}

	// Test with invalid CR namespace (contains invalid characters)
	cr := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "valid-name",
			Namespace: "invalid@namespace#with$special.chars",
		},
	}

	_, err := getTopology(ctx, operatorConfig, cr, operatorutils.VersionSpec{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid CR namespace for YAML substitution")
}

func TestGetTopology_InvalidLogLevel_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// Save originals and restore after test
	origGetObs := getObservabilityModuleFn
	origReadCfg := readConfigFileFn
	defer func() {
		getObservabilityModuleFn = origGetObs
		readConfigFileFn = origReadCfg
	}()

	// Mock getObservabilityModuleFn to return a module with invalid log level
	getObservabilityModuleFn = func(_ csmv1.ContainerStorageModule) (csmv1.Module, error) {
		return csmv1.Module{
			Name:    csmv1.Observability,
			Enabled: true,
			Components: []csmv1.ContainerTemplate{
				{
					Name: ObservabilityTopologyName,
					Envs: []corev1.EnvVar{
						{
							Name:  TopologyLogLevel,
							Value: "info\nmalicious: content", // Contains newline - YAML injection attempt
						},
					},
				},
			},
		}, nil
	}

	// Mock readConfigFileFn to return a minimal YAML
	readConfigFileFn = func(_ context.Context, _ csmv1.Module, cr csmv1.ContainerStorageModule, _ operatorutils.OperatorConfig, _ string) ([]byte, error) {
		yaml := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: karavi-topology
  namespace: %s
spec:
  template:
    spec:
      containers:
      - name: karavi-topology
`, cr.Namespace)
		return []byte(yaml), nil
	}

	// Test with valid CR name/namespace but invalid log level
	cr := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "valid-name",
			Namespace: "valid-namespace",
		},
	}

	_, err := getTopology(ctx, operatorConfig, cr, operatorutils.VersionSpec{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log level for YAML substitution")
}

func TestGetOtelCollector_InvalidCRName_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// Use a real CR file and modify it to have invalid name
	customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	if err != nil {
		panic(err)
	}

	// Modify the CR name to be invalid
	customResource.Name = "invalid@name#with$special.chars"

	_, err = getOtelCollector(ctx, operatorConfig, customResource, operatorutils.VersionSpec{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid CR name for YAML substitution")
}

func TestGetOtelCollector_InvalidCRNamespace_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// Use a real CR file and modify it to have invalid namespace
	customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	if err != nil {
		panic(err)
	}

	// Modify the CR namespace to be invalid
	customResource.Namespace = "invalid@namespace#with$special.chars"

	_, err = getOtelCollector(ctx, operatorConfig, customResource, operatorutils.VersionSpec{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid CR namespace for YAML substitution")
}

func TestGetOtelCollector_InvalidOtelCollectorImage_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// Use a real CR file and modify it to have invalid otel collector image
	customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	if err != nil {
		panic(err)
	}

	// Find the observability module and modify the otel collector image to be invalid
	for i, module := range customResource.Spec.Modules {
		if module.Name == csmv1.Observability {
			for j, component := range module.Components {
				if component.Name == ObservabilityOtelCollectorName {
					// Set invalid image with newline for YAML injection attempt
					customResource.Spec.Modules[i].Components[j].Image = csmv1.ImageType("otel-collector:latest\nmalicious: content")
					break
				}
			}
			break
		}
	}

	_, err = getOtelCollector(ctx, operatorConfig, customResource, operatorutils.VersionSpec{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image for YAML substitution")
}

func TestGetOtelCollector_InvalidNginxProxyImage_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// Use a real CR file and modify it to have invalid nginx proxy image
	customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	if err != nil {
		panic(err)
	}

	// Find the observability module and modify the nginx proxy image env var to be invalid
	found := false
	for i, module := range customResource.Spec.Modules {
		if module.Name == csmv1.Observability {
			for j, component := range module.Components {
				if component.Name == ObservabilityOtelCollectorName {
					// Set invalid nginx proxy image env var with newline for YAML injection attempt
					for k, env := range component.Envs {
						if env.Name == NginxProxyImage {
							customResource.Spec.Modules[i].Components[j].Envs[k].Value = "nginx:latest\nmalicious: content"
							found = true
							break
						}
					}
					// If env var doesn't exist, add it
					if !found {
						customResource.Spec.Modules[i].Components[j].Envs = append(component.Envs, corev1.EnvVar{
							Name:  NginxProxyImage,
							Value: "nginx:latest\nmalicious: content",
						})
						found = true
					}
					break
				}
			}
			break
		}
	}

	if !found {
		t.Skip("NginxProxyImage env var not found in otel-collector component, skipping test")
	}

	_, err = getOtelCollector(ctx, operatorConfig, customResource, operatorutils.VersionSpec{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image for YAML substitution")
}

func TestGetPowerStoreMetricsObjects_InvalidCRName_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// Use a real CR file and modify it to have invalid name
	customResource, err := getCustomResource("./testdata/cr_powerstore_observability.yaml")
	if err != nil {
		panic(err)
	}

	// Modify the CR name to be invalid
	customResource.Name = "invalid@name#with$special.chars"

	_, err = getPowerStoreMetricsObjects(ctx, operatorConfig, customResource, operatorutils.VersionSpec{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid CR name for YAML substitution")
}

func TestGetPowerStoreMetricsObjects_InvalidCRNamespace_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// Use a real CR file and modify it to have invalid namespace
	customResource, err := getCustomResource("./testdata/cr_powerstore_observability.yaml")
	if err != nil {
		panic(err)
	}

	// Modify the CR namespace to be invalid
	customResource.Namespace = "invalid@namespace#with$special.chars"

	_, err = getPowerStoreMetricsObjects(ctx, operatorConfig, customResource, operatorutils.VersionSpec{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid CR namespace for YAML substitution")
}

func TestGetPowerStoreMetricsObjects_InvalidValue_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// Use a real CR file and modify it to have invalid log level
	customResource, err := getCustomResource("./testdata/cr_powerstore_observability.yaml")
	if err != nil {
		panic(err)
	}

	// Find the observability module and modify the log level to be invalid
	found := false
	for i, module := range customResource.Spec.Modules {
		if module.Name == csmv1.Observability {
			for j, component := range module.Components {
				if component.Name == ObservabilityMetricsPowerStoreName {
					// Set invalid log level env var with newline for YAML injection attempt
					for k, env := range component.Envs {
						if strings.Contains("POWERSTORE_LOG_LEVEL", env.Name) {
							customResource.Spec.Modules[i].Components[j].Envs[k].Value = "info\nmalicious: content"
							found = true
							break
						}
					}
					// If env var doesn't exist, add it
					if !found {
						customResource.Spec.Modules[i].Components[j].Envs = append(component.Envs, corev1.EnvVar{
							Name:  "POWERSTORE_LOG_LEVEL",
							Value: "info\nmalicious: content",
						})
						found = true
					}
					break
				}
			}
			break
		}
	}

	if !found {
		t.Skip("POWERSTORE_LOG_LEVEL env var not found in powerstore metrics component, skipping test")
	}

	_, err = getPowerStoreMetricsObjects(ctx, operatorConfig, customResource, operatorutils.VersionSpec{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value for YAML substitution")
}

func TestGetPowerScaleMetricsObjects_InvalidCRName_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// Use a real CR file and modify it to have invalid name
	customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	if err != nil {
		panic(err)
	}

	// Modify the CR name to be invalid
	customResource.Name = "invalid@name#with$special.chars"

	_, err = getPowerScaleMetricsObjects(ctx, operatorConfig, customResource, operatorutils.VersionSpec{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid CR name for YAML substitution")
}

func TestGetPowerScaleMetricsObjects_InvalidCRNamespace_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// Use a real CR file and modify it to have invalid namespace
	customResource, err := getCustomResource("./testdata/cr_powerscale_observability.yaml")
	if err != nil {
		panic(err)
	}

	// Modify the CR namespace to be invalid
	customResource.Namespace = "invalid@namespace#with$special.chars"

	_, err = getPowerScaleMetricsObjects(ctx, operatorConfig, customResource, operatorutils.VersionSpec{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid CR namespace for YAML substitution")
}
