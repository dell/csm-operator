//  Copyright © 2022-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//       http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package controllers

import (
	"context"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func getScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	err := csmv1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}
	scheme.AddKnownTypes(schema.GroupVersion{Group: "monitoring.coreos.com", Version: "v1"}, &unstructured.Unstructured{})
	return scheme
}

func int32Ptr(i int32) *int32 {
	return &i
}

// TestSyncAuthorizationPrometheusRule_Enabled_CreatesPrometheusRule verifies that
// when module metrics and PrometheusRule are enabled, the PrometheusRule is created
// with the correct alert rules.
func TestSyncAuthorizationPrometheusRule_Enabled_CreatesPrometheusRule(t *testing.T) {
	ctx := context.Background()
	namespace := "test"
	crName := "auth-promrule-test"

	cr := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "ContainerStorageModule",
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Version: "v1.18.0",
		},
	}

	module := csmv1.Module{
		Name:          csmv1.AuthorizationServer,
		ConfigVersion: "v2.6.0",
		Metrics: &csmv1.ModuleMetrics{
			Enabled: true,
			PrometheusRule: &csmv1.ModulePrometheusRuleConfig{
				Enabled:                     true,
				AuthFailureRateThreshold:    int32Ptr(15), // 15% -> 0.15
				AuthLatencyQuantile:         int32Ptr(90), // 90th percentile -> 0.90
				AuthLatencyThresholdSeconds: int32Ptr(3),  // 3 seconds
			},
		},
	}

	scheme := getScheme()
	ctrlClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	op := operatorutils.OperatorConfig{ConfigDirectory: "../operatorconfig"}

	err := syncAuthorizationPrometheusRule(ctx, false, module, cr, op, ctrlClient)
	require.NoError(t, err)

	promRuleName := authorizationPrometheusRuleName(crName)
	promRule := &unstructured.Unstructured{}
	promRule.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err = ctrlClient.Get(ctx, types.NamespacedName{Name: promRuleName, Namespace: namespace}, promRule)
	require.NoError(t, err, "PrometheusRule should be created")

	rules := flatRulesFromPrometheusRule(t, promRule)
	assert.Len(t, rules, 8, "should have 8 alert rules")
}

// TestSyncAuthorizationPrometheusRule_Disabled_DeletesPrometheusRule verifies that
// when PrometheusRule is disabled, the existing PrometheusRule is deleted.
func TestSyncAuthorizationPrometheusRule_Disabled_DeletesPrometheusRule(t *testing.T) {
	ctx := context.Background()
	namespace := "test"
	crName := "auth-promrule-delete"

	cr := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "ContainerStorageModule",
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Version: "v1.18.0",
		},
	}

	module := csmv1.Module{
		Name:          csmv1.AuthorizationServer,
		ConfigVersion: "v2.6.0",
		Metrics: &csmv1.ModuleMetrics{
			Enabled: true,
			PrometheusRule: &csmv1.ModulePrometheusRuleConfig{
				Enabled:                     true,
				AuthFailureRateThreshold:    int32Ptr(15), // 15% -> 0.15
				AuthLatencyQuantile:         int32Ptr(90), // 90th percentile -> 0.90
				AuthLatencyThresholdSeconds: int32Ptr(3),  // 3 seconds
			},
		},
	}

	scheme := getScheme()
	ctrlClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	op := operatorutils.OperatorConfig{ConfigDirectory: "../operatorconfig"}

	// First, create the PrometheusRule
	err := syncAuthorizationPrometheusRule(ctx, false, module, cr, op, ctrlClient)
	require.NoError(t, err)

	// Now disable PrometheusRule
	module.Metrics.PrometheusRule.Enabled = false
	err = syncAuthorizationPrometheusRule(ctx, false, module, cr, op, ctrlClient)
	require.NoError(t, err)

	promRuleName := authorizationPrometheusRuleName(crName)
	promRule := &unstructured.Unstructured{}
	promRule.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err = ctrlClient.Get(ctx, types.NamespacedName{Name: promRuleName, Namespace: namespace}, promRule)
	assert.Error(t, err, "PrometheusRule should be deleted")
}

// TestSyncAuthorizationPrometheusRule_MetricsDisabled_DeletesPrometheusRule verifies that
// when module metrics are disabled, the PrometheusRule is deleted.
func TestSyncAuthorizationPrometheusRule_MetricsDisabled_DeletesPrometheusRule(t *testing.T) {
	ctx := context.Background()
	namespace := "test"
	crName := "auth-promrule-metrics-off"

	cr := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "ContainerStorageModule",
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Version: "v1.18.0",
		},
	}

	module := csmv1.Module{
		Name:          csmv1.AuthorizationServer,
		ConfigVersion: "v2.6.0",
		Metrics: &csmv1.ModuleMetrics{
			Enabled: true,
			PrometheusRule: &csmv1.ModulePrometheusRuleConfig{
				Enabled:                     true,
				AuthFailureRateThreshold:    int32Ptr(15), // 15% -> 0.15
				AuthLatencyQuantile:         int32Ptr(90), // 90th percentile -> 0.90
				AuthLatencyThresholdSeconds: int32Ptr(3),  // 3 seconds
			},
		},
	}

	scheme := getScheme()
	ctrlClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	op := operatorutils.OperatorConfig{ConfigDirectory: "../operatorconfig"}

	// First, create the PrometheusRule
	err := syncAuthorizationPrometheusRule(ctx, false, module, cr, op, ctrlClient)
	require.NoError(t, err)

	// Now disable metrics
	module.Metrics.Enabled = false
	err = syncAuthorizationPrometheusRule(ctx, false, module, cr, op, ctrlClient)
	require.NoError(t, err)

	promRuleName := authorizationPrometheusRuleName(crName)
	promRule := &unstructured.Unstructured{}
	promRule.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err = ctrlClient.Get(ctx, types.NamespacedName{Name: promRuleName, Namespace: namespace}, promRule)
	assert.Error(t, err, "PrometheusRule should be deleted when metrics disabled")
}

// TestSyncAuthorizationPrometheusRule_OwnerReferences verifies that the
// PrometheusRule has the correct owner references set.
func TestSyncAuthorizationPrometheusRule_OwnerReferences(t *testing.T) {
	ctx := context.Background()
	namespace := "test"
	crName := "auth-promrule-owner"

	cr := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: namespace,
			UID:       "test-uid-123",
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "ContainerStorageModule",
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Version: "v1.18.0",
		},
	}

	module := csmv1.Module{
		Name:          csmv1.AuthorizationServer,
		ConfigVersion: "v2.6.0",
		Metrics: &csmv1.ModuleMetrics{
			Enabled: true,
			PrometheusRule: &csmv1.ModulePrometheusRuleConfig{
				Enabled:                     true,
				AuthFailureRateThreshold:    int32Ptr(15), // 15% -> 0.15
				AuthLatencyQuantile:         int32Ptr(90), // 90th percentile -> 0.90
				AuthLatencyThresholdSeconds: int32Ptr(3),  // 3 seconds
			},
		},
	}

	scheme := getScheme()
	ctrlClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	op := operatorutils.OperatorConfig{ConfigDirectory: "../operatorconfig"}

	err := syncAuthorizationPrometheusRule(ctx, false, module, cr, op, ctrlClient)
	require.NoError(t, err)

	promRuleName := authorizationPrometheusRuleName(crName)
	promRule := &unstructured.Unstructured{}
	promRule.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err = ctrlClient.Get(ctx, types.NamespacedName{Name: promRuleName, Namespace: namespace}, promRule)
	require.NoError(t, err)

	ownerRefs, found, err := unstructured.NestedSlice(promRule.Object, "metadata", "ownerReferences")
	require.NoError(t, err)
	require.True(t, found, "ownerReferences should be present")
	require.Len(t, ownerRefs, 1, "should have exactly one owner reference")

	ownerRef, ok := ownerRefs[0].(map[string]interface{})
	require.True(t, ok, "owner reference should be a map")
	assert.Equal(t, "storage.dell.com/v1", ownerRef["apiVersion"])
	assert.Equal(t, "ContainerStorageModule", ownerRef["kind"])
	assert.Equal(t, crName, ownerRef["name"])
	assert.Equal(t, "test-uid-123", ownerRef["uid"])
	assert.Equal(t, true, ownerRef["controller"])
	assert.Equal(t, false, ownerRef["blockOwnerDeletion"])
}
