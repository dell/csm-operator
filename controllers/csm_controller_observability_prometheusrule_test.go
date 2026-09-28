// Copyright © 2022-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const testObservabilityConfigDir = "../operatorconfig"

var testObservabilityOpConfig = operatorutils.OperatorConfig{ConfigDirectory: testObservabilityConfigDir}

func TestLoadObservabilityPrometheusRuleAlerts_DefaultRules(t *testing.T) {
	rules, err := loadObservabilityPrometheusRuleAlerts(testObservabilityConfigDir, "v1.16.0")
	require.NoError(t, err)
	require.Len(t, rules, 4)

	expectedAlerts := []string{
		"CSMObservabilityMetricsCollectionFailure",
		"CSMObservabilityMetricsExportFailure",
		"CSMObservabilityArrayConnectivityLost",
		"CSMObservabilityScrapeEndpointUnavailable",
	}
	for _, alertName := range expectedAlerts {
		_, found := observabilityAlertRuleByName(t, rules, alertName)
		assert.True(t, found, "expected alert %s", alertName)
	}

	collection := mustObservabilityAlertRule(t, rules, "CSMObservabilityMetricsCollectionFailure")
	assert.Equal(t, `absent(dell_csm_obs_collection_rate) or dell_csm_obs_collection_rate == 0`, collection["expr"])
	assert.Equal(t, "3m", collection["for"])
	assert.Equal(t, "warning", observabilityNestedStringField(t, collection, "labels", "severity"))
	assert.Equal(t, "observability", observabilityNestedStringField(t, collection, "labels", "platform"))
	assert.Equal(t, "csm-module", observabilityNestedStringField(t, collection, "labels", "component"))

	exportFailure := mustObservabilityAlertRule(t, rules, "CSMObservabilityMetricsExportFailure")
	assert.Equal(t, `increase(dell_csm_obs_export_success_total{status=~"failure|error"}[5m]) > 0`, exportFailure["expr"])
	assert.Equal(t, "3m", exportFailure["for"])

	connectivity := mustObservabilityAlertRule(t, rules, "CSMObservabilityArrayConnectivityLost")
	assert.Equal(t, "dell_csm_obs_array_connectivity == 0", connectivity["expr"])
	assert.Equal(t, "2m", connectivity["for"])
	assert.Equal(t, "critical", observabilityNestedStringField(t, connectivity, "labels", "severity"))

	scrape := mustObservabilityAlertRule(t, rules, "CSMObservabilityScrapeEndpointUnavailable")
	assert.Equal(t, `absent(up{job=~"karavi-metrics-.*"}) or up{job=~"karavi-metrics-.*"} == 0`, scrape["expr"])
	assert.Equal(t, "2m", scrape["for"])
}

func TestSyncObservabilityPrometheusRule_CreatesAndDeletes(t *testing.T) {
	ctx := context.Background()
	cr := testObservabilityCR("obs-test", "test")
	cr.Spec.Modules[0].Metrics.PrometheusRule = &csmv1.ModulePrometheusRuleConfig{Enabled: true}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncObservabilityPrometheusRule(ctx, false, testObservabilityOpConfig, cr, fakeClient))

	promRule := fetchObservabilityPrometheusRule(ctx, t, fakeClient, cr.Namespace)
	rules := observabilityPrometheusRuleRules(t, promRule)
	require.Len(t, rules, 4)
	assert.Equal(t, `absent(dell_csm_obs_collection_rate) or dell_csm_obs_collection_rate == 0`, mustObservabilityAlertRule(t, rules, "CSMObservabilityMetricsCollectionFailure")["expr"])

	// Verify PrometheusRule has required labels for Prometheus discovery
	var err error
	labels, found, err := unstructured.NestedMap(promRule.Object, "metadata", "labels")
	require.NoError(t, err)
	require.True(t, found, "PrometheusRule should have labels")
	assert.Equal(t, "karavi-observability", labels["app.kubernetes.io/name"])
	assert.Equal(t, "obs-test", labels["app.kubernetes.io/instance"])
	assert.Equal(t, "csm-operator", labels["app.kubernetes.io/managed-by"])

	ownerRefs, found, err := unstructured.NestedSlice(promRule.Object, "metadata", "ownerReferences")
	require.NoError(t, err)
	require.True(t, found, "PrometheusRule should have ownerReferences")
	require.Len(t, ownerRefs, 1)
	ownerRef, ok := ownerRefs[0].(map[string]interface{})
	require.True(t, ok, "ownerReferences[0] should be a map")
	assert.Equal(t, "storage.dell.com/v1", ownerRef["apiVersion"])
	assert.Equal(t, "ContainerStorageModule", ownerRef["kind"])
	assert.Equal(t, "obs-test", ownerRef["name"])
	assert.Equal(t, "obs-test-uid", ownerRef["uid"])
	assert.Equal(t, true, ownerRef["controller"])
	assert.Equal(t, true, ownerRef["blockOwnerDeletion"])

	cr.Spec.Modules[0].Metrics.PrometheusRule.Enabled = false
	require.NoError(t, syncObservabilityPrometheusRule(ctx, false, testObservabilityOpConfig, cr, fakeClient))

	missing := &unstructured.Unstructured{}
	missing.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err = fakeClient.Get(ctx, types.NamespacedName{Name: observabilityPrometheusRuleName, Namespace: cr.Namespace}, missing)
	assert.True(t, k8sErrors.IsNotFound(err), "PrometheusRule should be deleted when prometheusRule.enabled is false")
}

func TestReconcileObservability_CreatesPrometheusRule(t *testing.T) {
	ctx := context.Background()
	cr := testObservabilityCR("obs-reconcile-test", "test")
	cr.Spec.Modules[0].Components = nil
	cr.Spec.Modules[0].Metrics.PrometheusRule = &csmv1.ModulePrometheusRuleConfig{Enabled: true}

	ctrlClient := fake.NewClientBuilder().Build()
	reconciler := &ContainerStorageModuleReconciler{}
	require.NoError(t, reconciler.reconcileObservability(ctx, false, testObservabilityOpConfig, cr, nil, ctrlClient, nil, operatorutils.VersionSpec{}))

	fetchObservabilityPrometheusRule(ctx, t, ctrlClient, cr.Namespace)
}

func TestSyncObservabilityPrometheusRule_PrometheusRuleNil(t *testing.T) {
	ctx := context.Background()
	cr := testObservabilityCR("obs-test", "test")
	cr.Spec.Modules[0].Metrics.PrometheusRule = nil

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncObservabilityPrometheusRule(ctx, false, testObservabilityOpConfig, cr, fakeClient))

	missing := &unstructured.Unstructured{}
	missing.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	var err error
	err = fakeClient.Get(ctx, types.NamespacedName{Name: observabilityPrometheusRuleName, Namespace: cr.Namespace}, missing)
	assert.True(t, k8sErrors.IsNotFound(err), "PrometheusRule should be deleted when prometheusRule is nil")
}

func TestSyncObservabilityPrometheusRule_LoadsModuleYAML(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "common"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "moduleconfig", "observability", "v1.16.0"), 0o755))

	releasesYAML := []byte("v1.18.0:\n  powerstore:\n    version: v2.18.0\n    modules:\n      observability: v1.16.0\n")
	ruleYAML := []byte("- alert: CSMObservabilityFromYAML\n  expr: vector(1)\n  for: \"0m\"\n  labels:\n    severity: warning\n    platform: observability\n    component: csm-module\n    alert_id: OBS-YAML\n  annotations:\n    summary: \"yaml rule\"\n    description: \"yaml rule\"\n")

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "common", "csm-releases.yaml"), releasesYAML, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "moduleconfig", "observability", "v1.16.0", "prometheusrule.yaml"), ruleYAML, 0o644))

	cr := testObservabilityCR("obs-yaml-test", "test")
	cr.Spec.Modules[0].Metrics.PrometheusRule = &csmv1.ModulePrometheusRuleConfig{Enabled: true}

	fakeClient := fake.NewClientBuilder().Build()
	opConfig := operatorutils.OperatorConfig{ConfigDirectory: tmpDir}
	require.NoError(t, syncObservabilityPrometheusRule(ctx, false, opConfig, cr, fakeClient))

	promRule := fetchObservabilityPrometheusRule(ctx, t, fakeClient, cr.Namespace)
	rules := observabilityPrometheusRuleRules(t, promRule)
	require.Len(t, rules, 1)
	assert.Equal(t, "CSMObservabilityFromYAML", mustObservabilityAlertRule(t, rules, "CSMObservabilityFromYAML")["alert"])
}

func testObservabilityCR(name, namespace string) csmv1.ContainerStorageModule {
	return csmv1.ContainerStorageModule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.dell.com/v1",
			Kind:       "ContainerStorageModule",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, UID: types.UID(name + "-uid")},
		Spec: csmv1.ContainerStorageModuleSpec{
			Version: "v1.18.0",
			Driver:  csmv1.Driver{CSIDriverType: csmv1.PowerStore},
			Modules: []csmv1.Module{{
				Name:       csmv1.Observability,
				Enabled:    true,
				Metrics:    &csmv1.ModuleMetrics{Enabled: true, PrometheusRule: &csmv1.ModulePrometheusRuleConfig{Enabled: true}},
				Components: []csmv1.ContainerTemplate{{Name: "metrics-powerstore", Enabled: ptrBool(true)}},
			}},
		},
	}
}

func ptrBool(v bool) *bool { return &v }

func fetchObservabilityPrometheusRule(ctx context.Context, t *testing.T, ctrlClient client.Client, namespace string) *unstructured.Unstructured {
	t.Helper()
	rule := &unstructured.Unstructured{}
	rule.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	require.NoError(t, ctrlClient.Get(ctx, types.NamespacedName{Name: observabilityPrometheusRuleName, Namespace: namespace}, rule))
	return rule
}

func observabilityPrometheusRuleRules(t *testing.T, promRule *unstructured.Unstructured) []interface{} {
	t.Helper()
	spec, found, err := unstructured.NestedMap(promRule.Object, "spec")
	require.NoError(t, err)
	require.True(t, found, "PrometheusRule spec should be present")
	groups, ok := spec["groups"].([]interface{})
	require.True(t, ok, "PrometheusRule groups should be a list")
	require.Len(t, groups, 1)
	group, ok := groups[0].(map[string]interface{})
	require.True(t, ok, "PrometheusRule group should be a map")
	rules, ok := group["rules"].([]interface{})
	require.True(t, ok, "PrometheusRule rules should be a list")
	return rules
}

func mustObservabilityAlertRule(t *testing.T, rules []interface{}, alertName string) map[string]interface{} {
	t.Helper()
	rule, found := observabilityAlertRuleByName(t, rules, alertName)
	require.True(t, found, "expected alert rule %s", alertName)
	return rule
}

func observabilityAlertRuleByName(t *testing.T, rules []interface{}, alertName string) (map[string]interface{}, bool) {
	t.Helper()
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]interface{})
		require.True(t, ok, "alert rule should be a map")
		if ruleMap["alert"] == alertName {
			return ruleMap, true
		}
	}
	return nil, false
}

func observabilityNestedStringField(t *testing.T, parent map[string]interface{}, container, key string) string {
	t.Helper()
	containerMap, ok := parent[container].(map[string]interface{})
	require.True(t, ok, "%s should be a map", container)
	value, ok := containerMap[key].(string)
	require.True(t, ok, "%s.%s should be a string", container, key)
	return value
}

func TestResolveObservabilityPrometheusRuleVersion_UsesConfigVersion(t *testing.T) {
	ctx := context.Background()
	cr := testObservabilityCR("obs-test", "test")
	cr.Spec.Modules[0].ConfigVersion = "v1.15.0"

	version, err := resolveObservabilityPrometheusRuleVersion(ctx, cr, testObservabilityOpConfig, cr.Spec.Modules[0])
	require.NoError(t, err)
	assert.Equal(t, "v1.15.0", version)
}

func TestResolveObservabilityPrometheusRuleVersion_FallsBackToDriverVersion(t *testing.T) {
	ctx := context.Background()
	cr := testObservabilityCR("obs-test", "test")
	cr.Spec.Modules[0].ConfigVersion = ""

	version, err := resolveObservabilityPrometheusRuleVersion(ctx, cr, testObservabilityOpConfig, cr.Spec.Modules[0])
	require.NoError(t, err)
	assert.Equal(t, "v1.16.0", version)
}

func TestResolveObservabilityPrometheusRuleAlertRules_EmptyRules(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "common"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "moduleconfig", "observability", "v1.16.0"), 0o755))

	releasesYAML := []byte("v1.18.0:\n  powerstore:\n    version: v2.18.0\n    modules:\n      observability: v1.16.0\n")
	emptyRuleYAML := []byte("")

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "common", "csm-releases.yaml"), releasesYAML, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "moduleconfig", "observability", "v1.16.0", "prometheusrule.yaml"), emptyRuleYAML, 0o644))

	cr := testObservabilityCR("obs-test", "test")
	opConfig := operatorutils.OperatorConfig{ConfigDirectory: tmpDir}

	_, err := resolveObservabilityPrometheusRuleAlertRules(ctx, cr, opConfig, cr.Spec.Modules[0])
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contains no alert rules")
}

func TestFindObservabilityModule_NotFound(t *testing.T) {
	cr := testObservabilityCR("obs-test", "test")
	cr.Spec.Modules[0].Name = csmv1.Authorization

	_, err := findObservabilityModule(cr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not find observability module")
}

func TestSyncObservabilityPrometheusRule_NoObservabilityModule(t *testing.T) {
	ctx := context.Background()
	cr := testObservabilityCR("obs-test", "test")
	cr.Spec.Modules[0].Name = csmv1.Authorization

	fakeClient := fake.NewClientBuilder().Build()
	err := syncObservabilityPrometheusRule(ctx, false, testObservabilityOpConfig, cr, fakeClient)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not find observability module")
}

func TestLoadObservabilityPrometheusRuleAlerts_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := loadObservabilityPrometheusRuleAlerts(tmpDir, "v1.16.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestResolveObservabilityPrometheusRuleVersion_InvalidReleasesFile(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "common"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "common", "csm-releases.yaml"), []byte("invalid yaml: ["), 0o644))

	cr := testObservabilityCR("obs-test", "test")
	cr.Spec.Modules[0].ConfigVersion = ""
	opConfig := operatorutils.OperatorConfig{ConfigDirectory: tmpDir}

	_, err := resolveObservabilityPrometheusRuleVersion(ctx, cr, opConfig, cr.Spec.Modules[0])
	require.Error(t, err)
}
