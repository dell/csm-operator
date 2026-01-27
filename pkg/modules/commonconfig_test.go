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
	"log"
	"os"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
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
			return true, true, tmpCR, sourceClient, operatorConfig, operatorutils.VersionSpec{}
		},
		"success - creating": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig, operatorutils.VersionSpec{}
		},
		"fail - wrong module name": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource
			badOperatorConfig.ConfigDirectory = "invalid-dir"

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()

			return false, false, tmpCR, sourceClient, badOperatorConfig, operatorutils.VersionSpec{}
		},
		"success - creating with custom registry": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy_custom_registry.yaml")
			if err != nil {
				panic(err)
			}

			tmpCR := customResource

			sourceClient := ctrlClientFake.NewClientBuilder().WithObjects().Build()
			return true, false, tmpCR, sourceClient, operatorConfig, operatorutils.VersionSpec{}
		},
		"success - creating with version overrides": func(*testing.T) (bool, bool, csmv1.ContainerStorageModule, ctrlClient.Client, operatorutils.OperatorConfig, operatorutils.VersionSpec) {
			customResource, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
			if err != nil {
				panic(err)
			}

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
	cr, err := getCustomResource("./testdata/cr_auth_proxy.yaml")
	if err != nil {
		t.Fatal(err)
	}

	cmCRD := &apiextv1.CustomResourceDefinition{
		TypeMeta:   metav1.TypeMeta{Kind: "CustomResourceDefinition"},
		ObjectMeta: metav1.ObjectMeta{Name: "certificates.cert-manager.io"},
	}
	err = apiextv1.AddToScheme(scheme.Scheme)
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
