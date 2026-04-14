// Copyright (c) 2022-2026 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"log"
	"os"
	"strings"
	"testing"

	csmv1 "eos2git.cec.lab.emc.com/CSM/csm-operator/api/v1"
	operatorutils "eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/operatorutils"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrlClientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

var (
	operatorConfig    operatorutils.OperatorConfig
	badOperatorConfig operatorutils.OperatorConfig
)

func TestMain(m *testing.M) {
	status := 0

	operatorConfig = operatorutils.OperatorConfig{}
	operatorConfig.ConfigDirectory = "../../operatorconfig"

	err := apiextv1.AddToScheme(scheme.Scheme)
	if err != nil {
		panic(err)
	}

	if st := m.Run(); st > status {
		status = st
	}

	os.Exit(status)
}

func getCustomResource(path string) (csmv1.ContainerStorageModule, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("failed to read testdata: %v", err)
	}
	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(b, &customResource)
	if err != nil {
		return customResource, fmt.Errorf("failed to unmarshal CSM Custom resource: %v", err)
	}

	return customResource, nil
}

func getSecret(namespace, secretName string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"data": []byte(secretName),
		},
	}
}

func getConfigMap(namespace, configmapName string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"data": configmapName,
		},
	}
}

func TestCommonCertManager(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec){
		"success - deleting": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec) {
			customResource := CsmAuthorizationCR()
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
			return true, true, tmpCR, sourceClient, operatorConfig, operatorutils.VersionSpec{}
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec) {
			customResource := CsmAuthorizationCR()
			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig, operatorutils.VersionSpec{}
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
			badOperatorConfig.ConfigDirectory = "invalid-dir"

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig, operatorutils.VersionSpec{}
		},
		"success - creating with custom registry": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec) {
			customResource := CsmAuthorizationCR()
			tmpCR := customResource
			// add custom registry
			tmpCR.Spec.CustomRegistry = "quay.io"
			tmpCR.Spec.RetainImageRegistryPath = trueBool

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig, operatorutils.VersionSpec{}
		},
		"success - creating with version overrides": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec) {
			customResource := CsmAuthorizationCR()
			tmpCR := customResource
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			matched := operatorutils.VersionSpec{
				Version: "v1.14.0",
				Images: map[string]string{
					"cert-manager-cainjector": "quay.io/jetstack/cert-manager-cainjector:v1.6.1",
					"cert-manager-controller": "quay.io/jetstack/cert-manager-controller:v1.6.1",
					"cert-manager-webhook":    "quay.io/jetstack/cert-manager-webhook:v1.6.1",
				},
			}
			return true, false, tmpCR, sourceClient, operatorConfig, matched
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, cr, sourceClient, op, matched := tc(t)

			err := CommonCertManager(context.TODO(), isDeleting, op, cr, sourceClient, matched)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestPatchCSMDRCRDs(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, bool, ctrlClient.Client, operatorutils.OperatorConfig){
		"success - deleting": func(*testing.T) (bool, bool, ctrlClient.Client, operatorutils.OperatorConfig) {
			crd := &apiextv1.CustomResourceDefinition{
				TypeMeta: metav1.TypeMeta{
					Kind: "CustomResourceDefinition",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "volumejournals.dr.storage.dell.com",
				},
			}

			err := apiextv1.AddToScheme(scheme.Scheme)
			if err != nil {
				panic(err)
			}

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects(crd).Build()
			return true, true, sourceClient, operatorConfig
		},
		"success - creating": func(*testing.T) (bool, bool, ctrlClient.Client, operatorutils.OperatorConfig) {
			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, sourceClient, operatorConfig
		},
		"fail - invalid directory": func(*testing.T) (bool, bool, ctrlClient.Client, operatorutils.OperatorConfig) {
			badOperatorConfig.ConfigDirectory = "invalid-dir"

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, sourceClient, badOperatorConfig
		},
		"fail - unable to apply crd": func(*testing.T) (bool, bool, ctrlClient.Client, operatorutils.OperatorConfig) {
			cluster := operatorutils.ClusterConfig{
				ClusterCTRLClient: customClient{
					Client: fake.NewClientBuilder().Build(),
				},
			}

			return false, false, cluster.ClusterCTRLClient, operatorConfig
		},
		"fail - unable to delete crd": func(*testing.T) (bool, bool, ctrlClient.Client, operatorutils.OperatorConfig) {
			cluster := operatorutils.ClusterConfig{
				ClusterCTRLClient: customClient{
					Client: fake.NewClientBuilder().Build(),
				},
			}

			return false, true, cluster.ClusterCTRLClient, operatorConfig
		},
	}

	ctx := context.TODO()
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, isDeleting, sourceClient, op := tc(t)

			err := PatchCSMDRCRDs(ctx, isDeleting, op, sourceClient)
			if success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// customClient is our custom client that we will pass to removeDriverFromCluster
// this lets us control what Delete/Get/ etc returns from within removeDriverFromCluster
type customClient struct {
	client.Client
}

// Delete method is modified to return an error whenever its called
// this lets us control when to return an error from applyDeleteObjects
func (c customClient) Delete(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
	return fmt.Errorf("failed to delete: %s", obj.GetName())
}

// Get method is modified to always return no error
// This is so we can test out errors when an object exists but cannot be deleted
func (c customClient) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return nil
}

func TestApplyDeleteObjects(t *testing.T) {
	ctx := context.TODO()

	cluster := operatorutils.ClusterConfig{
		ClusterID: "test",
		ClusterCTRLClient: customClient{
			Client: fake.NewClientBuilder().Build(),
		},
	}

	normalYamlString := `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-configmap
  namespace: default
data:
  key: value
  `

	tests := []struct {
		name        string
		yamlString  string
		expectedErr string
		isDeleting  bool
	}{
		{
			name:        "fails to delete",
			yamlString:  normalYamlString,
			expectedErr: "failed to delete",
			isDeleting:  true,
		},
		{
			name:        "fails to apply",
			yamlString:  normalYamlString,
			expectedErr: "not found",
			isDeleting:  false,
		},
		{
			name:        "invalid yaml passed",
			yamlString:  "1",
			expectedErr: "cannot unmarshal number",
			isDeleting:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := applyDeleteObjects(ctx, cluster.ClusterCTRLClient, tt.yamlString, tt.isDeleting)
			if tt.expectedErr == "" {
				if err != nil {
					t.Errorf("removeDriverFromCluster() returned error = %v, but no error was expected", err)
				}
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

// Asserts CRDs are kept on uninstall (CommonCertManager delete path keeps CRDs).
func TestCommonCertManager_CRDsArePreservedOnDelete(t *testing.T) {
	ctx := context.TODO()
	cr := CsmAuthorizationCR()
	cmCRD := &apiextv1.CustomResourceDefinition{
		TypeMeta:   metav1.TypeMeta{Kind: "CustomResourceDefinition"},
		ObjectMeta: metav1.ObjectMeta{Name: "certificates.cert-manager.io"},
	}
	err := apiextv1.AddToScheme(scheme.Scheme)
	if err != nil {
		t.Fatal(err)
	}
	src := ctrlClientFake.NewClientBuilder().WithObjects(cmCRD).Build()

	err = CommonCertManager(ctx, true, operatorConfig, cr, src, operatorutils.VersionSpec{})
	assert.NoError(t, err)

	got := &apiextv1.CustomResourceDefinition{}
	err = src.Get(ctx, client.ObjectKey{Name: "certificates.cert-manager.io"}, got)
	assert.NoError(t, err, "CRD must remain present after uninstall")
}

// Success apply path in applyDeleteObjects.
func TestApplyDeleteObjects_SuccessApply(t *testing.T) {
	ctx := context.TODO()
	cli := fake.NewClientBuilder().Build()

	yml := `apiVersion: v1
kind: ConfigMap
metadata:
  name: ut-config
  namespace: default
data:
  k: v
`

	err := applyDeleteObjects(ctx, cli, yml, false)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Name: "ut-config", Namespace: "default"}, cm)
	assert.NoError(t, err)
	assert.Equal(t, "v", cm.Data["k"])
}

// Success delete path in applyDeleteObjects.
func TestApplyDeleteObjects_SuccessDelete(t *testing.T) {
	ctx := context.TODO()
	cli := fake.NewClientBuilder().Build()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "ut-del", Namespace: "default"},
		Data:       map[string]string{"k": "v"},
	}
	assert.NoError(t, cli.Create(ctx, cm))

	yml := `apiVersion: v1
kind: ConfigMap
metadata:
  name: ut-del
  namespace: default
data:
  k: v
`

	err := applyDeleteObjects(ctx, cli, yml, true)
	assert.NoError(t, err)

	got := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Name: "ut-del", Namespace: "default"}, got)
	assert.Error(t, err)
}

// Multi-document YAML path in applyDeleteObjects (apply & delete).
func TestApplyDeleteObjects_MultiDocYAML(t *testing.T) {
	ctx := context.TODO()
	cli := fake.NewClientBuilder().Build()

	yml := `apiVersion: v1
kind: ConfigMap
metadata:
  name: ut-cm-1
  namespace: default
data:
  k1: v1
---
apiVersion: v1
kind: Secret
metadata:
  name: ut-secret-1
  namespace: default
type: Opaque
stringData:
  s1: v1
`

	err := applyDeleteObjects(ctx, cli, yml, false)
	assert.NoError(t, err)

	assert.NoError(t, cli.Get(ctx, client.ObjectKey{Name: "ut-cm-1", Namespace: "default"}, &corev1.ConfigMap{}))
	assert.NoError(t, cli.Get(ctx, client.ObjectKey{Name: "ut-secret-1", Namespace: "default"}, &corev1.Secret{}))

	err = applyDeleteObjects(ctx, cli, yml, true)
	assert.NoError(t, err)

	assert.Error(t, cli.Get(ctx, client.ObjectKey{Name: "ut-cm-1", Namespace: "default"}, &corev1.ConfigMap{}))
	assert.Error(t, cli.Get(ctx, client.ObjectKey{Name: "ut-secret-1", Namespace: "default"}, &corev1.Secret{}))
}

// TestGetCertManager_SparseConfigMap verifies that a ConfigMap defining
// only a subset of cert-manager image keys correctly falls through to
// defaults for the missing keys (no unreplaced placeholders).
func TestGetCertManager_SparseConfigMap(t *testing.T) {
	ctx := context.TODO()
	cr := CsmAuthorizationCR()

	// Only the controller key is provided; cainjector and webhook are absent.
	matched := operatorutils.VersionSpec{
		Version: "v1.14.0",
		Images: map[string]string{
			"cert-manager-controller": "registry.example/cert-controller:override",
		},
	}

	yaml, err := getCertManager(ctx, operatorConfig, cr, matched)
	assert.NoError(t, err)

	// The overridden key should use the ConfigMap value.
	assert.Contains(t, yaml, "registry.example/cert-controller:override",
		"cert-manager-controller should be overridden by ConfigMap")

	// The missing keys must NOT remain as unreplaced placeholders.
	assert.NotContains(t, yaml, CertManagerCaInjector,
		"cainjector placeholder should be replaced with default, not left raw")
	assert.NotContains(t, yaml, CertManagerWebhook,
		"webhook placeholder should be replaced with default, not left raw")

	// The missing keys should fall through to their defaults.
	assert.Contains(t, yaml, CertManagerCaInjectorImage,
		"cainjector should resolve to default image")
	assert.Contains(t, yaml, CertManagerWebhookImage,
		"webhook should resolve to default image")
}

// TestGetCertManager_SparseConfigMapWithCustomRegistry verifies that
// missing ConfigMap keys fall through to Custom Registry (not defaults).
func TestGetCertManager_SparseConfigMapWithCustomRegistry(t *testing.T) {
	ctx := context.TODO()
	cr := CsmAuthorizationCR()
	cr.Spec.CustomRegistry = "my-registry.example.com"

	// Only the webhook key is provided.
	matched := operatorutils.VersionSpec{
		Version: "v1.14.0",
		Images: map[string]string{
			"cert-manager-webhook": "registry.example/cert-webhook:override",
		},
	}

	yaml, err := getCertManager(ctx, operatorConfig, cr, matched)
	assert.NoError(t, err)

	// The overridden key should use the ConfigMap value.
	assert.Contains(t, yaml, "registry.example/cert-webhook:override",
		"cert-manager-webhook should be overridden by ConfigMap")

	// The missing keys must be resolved via Custom Registry, not left as placeholders.
	assert.NotContains(t, yaml, CertManagerCaInjector,
		"cainjector placeholder should be replaced")
	assert.NotContains(t, yaml, CertManagerController,
		"controller placeholder should be replaced")

	// The missing keys should use Custom Registry (contain the registry prefix).
	assert.Contains(t, yaml, "my-registry.example.com/",
		"missing keys should be resolved via custom registry")
}

// TestGetCertManager_AllKeysFromConfigMap verifies that when all keys
// are present in the ConfigMap, all three images are overridden.
func TestGetCertManager_AllKeysFromConfigMap(t *testing.T) {
	ctx := context.TODO()
	cr := CsmAuthorizationCR()

	matched := operatorutils.VersionSpec{
		Version: "v1.14.0",
		Images: map[string]string{
			"cert-manager-cainjector": "registry.example/cainjector:cm",
			"cert-manager-controller": "registry.example/controller:cm",
			"cert-manager-webhook":    "registry.example/webhook:cm",
		},
	}

	yaml, err := getCertManager(ctx, operatorConfig, cr, matched)
	assert.NoError(t, err)

	assert.Contains(t, yaml, "registry.example/cainjector:cm")
	assert.Contains(t, yaml, "registry.example/controller:cm")
	assert.Contains(t, yaml, "registry.example/webhook:cm")

	// No placeholders should remain.
	assert.NotContains(t, yaml, CertManagerCaInjector)
	assert.NotContains(t, yaml, CertManagerController)
	assert.NotContains(t, yaml, CertManagerWebhook)
}

func TestGetCertManager_CustomRegistryOnly(t *testing.T) {
	ctx := context.TODO()
	cr := CsmAuthorizationCR()
	cr.Spec.CustomRegistry = "my-registry.example.com"

	matched := operatorutils.VersionSpec{} // empty - no ConfigMap

	yaml, err := getCertManager(ctx, operatorConfig, cr, matched)
	assert.NoError(t, err)

	// All three images should use the custom registry.
	assert.NotContains(t, yaml, CertManagerCaInjector, "cainjector placeholder should be replaced")
	assert.NotContains(t, yaml, CertManagerController, "controller placeholder should be replaced")
	assert.NotContains(t, yaml, CertManagerWebhook, "webhook placeholder should be replaced")

	// All should contain the custom registry prefix.
	// Count occurrences of the custom registry prefix.
	count := strings.Count(yaml, "my-registry.example.com/")
	assert.GreaterOrEqual(t, count, 3, "expected at least 3 images with custom registry prefix")
}

func TestGetCertManager_NeitherConfigMapNorRegistry(t *testing.T) {
	ctx := context.TODO()
	cr := CsmAuthorizationCR()

	matched := operatorutils.VersionSpec{} // empty

	yaml, err := getCertManager(ctx, operatorConfig, cr, matched)
	assert.NoError(t, err)

	// Default images should be in the output.
	assert.Contains(t, yaml, CertManagerCaInjectorImage, "cainjector should use default image")
	assert.Contains(t, yaml, CertManagerControllerImage, "controller should use default image")
	assert.Contains(t, yaml, CertManagerWebhookImage, "webhook should use default image")

	// No placeholders should remain.
	assert.NotContains(t, yaml, CertManagerCaInjector, "cainjector placeholder should be replaced")
	assert.NotContains(t, yaml, CertManagerController, "controller placeholder should be replaced")
	assert.NotContains(t, yaml, CertManagerWebhook, "webhook placeholder should be replaced")
}

func TestGetCertManager_AllKeysFromConfigMapWithCustomRegistry(t *testing.T) {
	ctx := context.TODO()
	cr := CsmAuthorizationCR()
	cr.Spec.CustomRegistry = "my-registry.example.com"

	matched := operatorutils.VersionSpec{
		Version: "v1.14.0",
		Images: map[string]string{
			"cert-manager-cainjector": "configmap-registry.example/cainjector:cm",
			"cert-manager-controller": "configmap-registry.example/controller:cm",
			"cert-manager-webhook":    "configmap-registry.example/webhook:cm",
		},
	}

	yaml, err := getCertManager(ctx, operatorConfig, cr, matched)
	assert.NoError(t, err)

	// All three should come from ConfigMap, NOT custom registry.
	assert.Contains(t, yaml, "configmap-registry.example/cainjector:cm")
	assert.Contains(t, yaml, "configmap-registry.example/controller:cm")
	assert.Contains(t, yaml, "configmap-registry.example/webhook:cm")

	// Custom registry prefix should NOT appear.
	assert.NotContains(t, yaml, "my-registry.example.com/", "custom registry should not override ConfigMap images")
}

func TestGetCertManager_EmptyVersionFallsThrough(t *testing.T) {
	ctx := context.TODO()
	cr := CsmAuthorizationCR()

	matched := operatorutils.VersionSpec{
		Version: "", // empty version -> ConfigMap check skipped
		Images: map[string]string{
			"cert-manager-cainjector": "should-not-apply:v1",
		},
	}

	yaml, err := getCertManager(ctx, operatorConfig, cr, matched)
	assert.NoError(t, err)

	// The image should NOT be from matched.Images since Version is empty.
	assert.NotContains(t, yaml, "should-not-apply:v1",
		"empty version should skip ConfigMap images")

	// Should fall through to defaults.
	assert.Contains(t, yaml, CertManagerCaInjectorImage)
}
