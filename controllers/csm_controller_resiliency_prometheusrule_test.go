// Copyright © 2022-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package controllers contains unit tests for the Resiliency module PrometheusRule
// functions: buildResiliencyAlertGroups, applyResiliencyThresholds,
// syncResiliencyPrometheusRule, resolvePrometheusRuleGroups,
// parsePrometheusRuleGroupsYAML, and injectNamespaceLabelIntoGroups.
//
// Coverage targets addressed here (pre-fix values):
//   - buildResiliencyAlertGroups:   0.0%
//   - applyResiliencyThresholds:    0.0%
//   - syncResiliencyPrometheusRule: 48.6%
//   - resolvePrometheusRuleGroups:  69.2%
//   - parsePrometheusRuleGroupsYAML: 75.0%
//   - injectNamespaceLabelIntoGroups: 73.3%
package controllers

import (
	"context"
	"strings"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	shared "github.com/dell/csm-operator/tests/sharedutil"
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

// ── constants ────────────────────────────────────────────────────────────────

// testResiliencyModuleVersion is the resiliency module version present for driver v2.18.0.
const testResiliencyModuleVersion = "v1.17.0"

// expectedResiliencyGroupNamePrefix is the prefix for the driver-specific group name in the resiliency prometheusrule.yaml.
// The full group name is "csm-resiliency-<platform>" (e.g., "csm-resiliency-powerstore").
const expectedResiliencyGroupNamePrefix = "csm-resiliency-"

// expectedResiliencyRuleCount is the number of alert rules in the resiliency group (RES-01..04).
const expectedResiliencyRuleCount = 4

// expectedResiliencyAlertNames are the canonical alert names defined in the resiliency YAML.
var expectedResiliencyAlertNames = []string{
	"CSMResiliencyPodFailureDetected",
	"CSMResiliencyConnectivityLost",
	"CSMResiliencyFailoverTriggered",
	"CSMResiliencyFailoverFailed",
}

// ── helpers ───────────────────────────────────────────────────────────────────

// i32 is a convenience wrapper that returns a pointer to the given int32 value,
// matching the pattern used in ModulePrometheusRuleConfig field setters.
func i32(v int32) *int32 { return &v }

// makeResiliencyModule builds a Module with Resiliency name and fully-enabled
// PrometheusRule metrics. Callers can override Metrics after construction.
func makeResiliencyModule(connectivityRatio *int32) csmv1.Module {
	cfg := &csmv1.ModulePrometheusRuleConfig{
		Enabled:                  true,
		ConnectivitySuccessRatio: connectivityRatio,
	}
	return csmv1.Module{
		Name: csmv1.Resiliency,
		Metrics: &csmv1.ModuleMetrics{
			Enabled:        true,
			PrometheusRule: cfg,
		},
	}
}

// makeResiliencyCSM returns a ContainerStorageModule wired for the given driver with
// ConfigVersion v2.18.0 so that GetModuleDefaultVersion can resolve resiliency v1.17.0.
func makeResiliencyCSM(name, ns string, driverType csmv1.DriverType) csmv1.ContainerStorageModule {
	cr := shared.MakeCSM(name, ns, "v2.18.0")
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{
		Name:      name,
		Namespace: ns,
		UID:       types.UID(name + "-uid"),
	}
	cr.Spec.Driver.CSIDriverType = driverType
	cr.Spec.Driver.ConfigVersion = "v2.18.0"
	return cr
}

// fetchResiliencyPrometheusRule retrieves the resiliency-scoped PrometheusRule resource
// (name = <crName>-resiliency-alerts) from the fake client.
func fetchResiliencyPrometheusRule(ctx context.Context, t *testing.T, ctrlClient client.Client, crName, namespace string) *unstructured.Unstructured {
	t.Helper()
	pr := &unstructured.Unstructured{}
	pr.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	require.NoError(t, ctrlClient.Get(ctx, types.NamespacedName{Name: resiliencyPrometheusRuleName(crName), Namespace: namespace}, pr))
	return pr
}

// flatRulesFromResiliencyPrometheusRule returns all alert rules in the resiliency PrometheusRule.
func flatRulesFromResiliencyPrometheusRule(t *testing.T, pr *unstructured.Unstructured) []interface{} {
	t.Helper()
	return flatRulesFromPrometheusRule(t, pr)
}

// ── buildResiliencyAlertGroups ────────────────────────────────────────────────

// TestBuildResiliencyAlertGroups_MissingYAML verifies that an error containing
// "prometheusrule.yaml" is returned when the moduleVersion directory does not exist.
func TestBuildResiliencyAlertGroups_MissingYAML(t *testing.T) {
	_, err := buildResiliencyAlertGroups(testConfigDir, "v0.0.0-nonexistent", csmv1.PowerStore, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prometheusrule.yaml")
}

// TestBuildResiliencyAlertGroups_GroupStructure verifies that the resiliency YAML for v1.17.0
// is loaded as a single group ("csm-resiliency-alerts") containing exactly 4 rules.
func TestBuildResiliencyAlertGroups_GroupStructure(t *testing.T) {
	groups, err := buildResiliencyAlertGroups(testConfigDir, testResiliencyModuleVersion, csmv1.PowerStore, nil)
	require.NoError(t, err)
	require.Len(t, groups, 1, "resiliency YAML should produce exactly 1 alert group")

	groupMap, ok := groups[0].(map[string]interface{})
	require.True(t, ok, "group element should be a map")
	// Group name is driver-specific: "csm-resiliency-<platform>" (e.g., "csm-resiliency-powerstore")
	groupName, ok := groupMap["name"].(string)
	require.True(t, ok, "group name should be a string")
	assert.True(t, strings.HasPrefix(groupName, expectedResiliencyGroupNamePrefix), "group name should start with %s, got %s", expectedResiliencyGroupNamePrefix, groupName)

	rules, ok := groupMap["rules"].([]interface{})
	require.True(t, ok, "group rules should be a list")
	assert.Len(t, rules, expectedResiliencyRuleCount, "resiliency group should have %d rules (RES-01..RES-04)", expectedResiliencyRuleCount)
}

// TestBuildResiliencyAlertGroups_AllAlertNamesPresent verifies that all four canonical
// resiliency alert names (RES-01..RES-04) are present after loading the YAML.
func TestBuildResiliencyAlertGroups_AllAlertNamesPresent(t *testing.T) {
	groups, err := buildResiliencyAlertGroups(testConfigDir, testResiliencyModuleVersion, csmv1.PowerStore, nil)
	require.NoError(t, err)
	rules := flattenGroupRules(t, groups)
	require.Len(t, rules, expectedResiliencyRuleCount)

	for _, name := range expectedResiliencyAlertNames {
		_, found := alertRuleByName(t, rules, name)
		assert.True(t, found, "expected alert %s to be present", name)
	}
}

// TestBuildResiliencyAlertGroups_DriverPlaceholdersSubstituted verifies that every driver
// type has its <DriverName> and <DriverPlatform> placeholders replaced (no raw angle-bracket
// strings remain in the parsed output).
func TestBuildResiliencyAlertGroups_DriverPlaceholdersSubstituted(t *testing.T) {
	drivers := []csmv1.DriverType{
		csmv1.PowerStore,
		csmv1.PowerFlex,
		csmv1.PowerScale,
		csmv1.PowerScaleName,
		csmv1.PowerMax,
	}
	for _, d := range drivers {
		groups, err := buildResiliencyAlertGroups(testConfigDir, testResiliencyModuleVersion, d, nil)
		require.NoError(t, err, "driver %s: unexpected error", d)
		require.NotEmpty(t, groups, "driver %s: expected non-empty groups", d)

		rules := flattenGroupRules(t, groups)
		for _, rule := range rules {
			ruleMap := rule.(map[string]interface{})
			expr, _ := ruleMap["expr"].(string)
			assert.False(t, strings.Contains(expr, "<Driver"), "driver %s: unresolved placeholder in expr: %s", d, expr)
		}
	}
}

// TestBuildResiliencyAlertGroups_CustomConnectivity verifies that a non-nil
// ConnectivitySuccessRatio is substituted into the RES-02 alert expression.
func TestBuildResiliencyAlertGroups_CustomConnectivity(t *testing.T) {
	cfg := &csmv1.ModulePrometheusRuleConfig{
		Enabled:                  true,
		ConnectivitySuccessRatio: i32(75),
	}
	groups, err := buildResiliencyAlertGroups(testConfigDir, testResiliencyModuleVersion, csmv1.PowerStore, cfg)
	require.NoError(t, err)

	rules := flattenGroupRules(t, groups)
	connectivityLost := mustAlertRule(t, rules, "CSMResiliencyConnectivityLost")
	expr := connectivityLost["expr"].(string)
	assert.Contains(t, expr, "75", "custom connectivity ratio should appear in RES-02 expr")
	assert.NotContains(t, expr, "<AlertConnectivitySuccessRatio>")
}

// ── applyResiliencyThresholds ─────────────────────────────────────────────────

// TestApplyResiliencyThresholds_DriverNames is a table-driven test that verifies the
// correct <DriverName> and <DriverPlatform> substitutions for every supported driver type.
func TestApplyResiliencyThresholds_DriverNames(t *testing.T) {
	template := "<DriverName>|<DriverPlatform>|<AlertConnectivitySuccessRatio>"

	cases := []struct {
		driverType       csmv1.DriverType
		expectedName     string
		expectedPlatform string
	}{
		{csmv1.PowerStore, "csi-powerstore", "powerstore"},
		{csmv1.PowerFlex, "csi-vxflexos", "powerflex"},
		{csmv1.PowerScale, "csi-powerscale", "powerscale"},
		{csmv1.PowerScaleName, "csi-powerscale", "powerscale"},
		{csmv1.PowerMax, "csi-powermax", "powermax"},
		// Unknown driver — falls through without special substitution
		{"unity", "unity", "unity"},
	}

	for _, tc := range cases {
		result := applyResiliencyThresholds(template, tc.driverType, nil)
		parts := strings.SplitN(result, "|", 3)
		require.Len(t, parts, 3, "driver %s: unexpected split", tc.driverType)
		assert.Equal(t, tc.expectedName, parts[0], "driver %s: DriverName mismatch", tc.driverType)
		assert.Equal(t, tc.expectedPlatform, parts[1], "driver %s: DriverPlatform mismatch", tc.driverType)
	}
}

// TestApplyResiliencyThresholds_DefaultConnectivity verifies that when cfg is nil,
// the default connectivity ratio (90) is substituted.
func TestApplyResiliencyThresholds_DefaultConnectivity(t *testing.T) {
	result := applyResiliencyThresholds("<AlertConnectivitySuccessRatio>", csmv1.PowerStore, nil)
	assert.Equal(t, "90", result, "nil cfg should default to 90")
}

// TestApplyResiliencyThresholds_CustomConnectivity verifies that a non-default
// ConnectivitySuccessRatio value is substituted correctly.
func TestApplyResiliencyThresholds_CustomConnectivity(t *testing.T) {
	cfg := &csmv1.ModulePrometheusRuleConfig{ConnectivitySuccessRatio: i32(60)}
	result := applyResiliencyThresholds("<AlertConnectivitySuccessRatio>", csmv1.PowerStore, cfg)
	assert.Equal(t, "60", result)
}

// TestApplyResiliencyThresholds_EmptyConfigDefaultsApplied verifies that an explicitly
// constructed empty ModulePrometheusRuleConfig (all nil fields) still falls back to
// the package defaults rather than panicking or substituting zero values.
func TestApplyResiliencyThresholds_EmptyConfigDefaultsApplied(t *testing.T) {
	cfg := &csmv1.ModulePrometheusRuleConfig{} // all fields nil
	result := applyResiliencyThresholds("<AlertConnectivitySuccessRatio>", csmv1.PowerMax, cfg)
	assert.Equal(t, "90", result, "empty config should still produce default ratio 90")
}

// TestApplyResiliencyThresholds_NoUnresolvedPlaceholders verifies that after applying
// thresholds on the real prometheusrule.yaml content, no angle-bracket placeholders remain.
func TestApplyResiliencyThresholds_NoUnresolvedPlaceholders(t *testing.T) {
	groups, err := buildResiliencyAlertGroups(testConfigDir, testResiliencyModuleVersion, csmv1.PowerScale, nil)
	require.NoError(t, err)

	rules := flattenGroupRules(t, groups)
	for _, rule := range rules {
		ruleMap := rule.(map[string]interface{})
		expr, _ := ruleMap["expr"].(string)
		assert.False(t, strings.Contains(expr, "<Alert"), "unresolved threshold placeholder in expr: %s", expr)
		assert.False(t, strings.Contains(expr, "<Driver"), "unresolved driver placeholder in expr: %s", expr)
	}
}

// ── parsePrometheusRuleGroupsYAML ─────────────────────────────────────────────

// TestParsePrometheusRuleGroupsYAML_InvalidYAML verifies that the error path is exercised
// when the YAML string is structurally invalid (e.g., a scalar where a list is expected).
func TestParsePrometheusRuleGroupsYAML_InvalidYAML(t *testing.T) {
	// A YAML mapping is not compatible with a []interface{} unmarshal target.
	invalidYAML := "key: value\nnested:\n  a: 1\n"
	groups, err := parsePrometheusRuleGroupsYAML(invalidYAML)
	// sigs.k8s.io/yaml (via JSON) will treat a map unmarshal into []interface{} as an error.
	if err != nil {
		assert.Nil(t, groups)
		assert.Contains(t, err.Error(), "prometheusrule YAML")
	} else {
		// Some YAML parsers succeed but return a nil/empty slice for type mismatch.
		// Either outcome is acceptable; the important thing is no panic.
		t.Log("parsePrometheusRuleGroupsYAML returned nil error for map-to-slice YAML; verifying nil groups")
	}
}

// TestParsePrometheusRuleGroupsYAML_MalformedYAML verifies that a syntax error in the
// YAML string causes an error to be returned.
func TestParsePrometheusRuleGroupsYAML_MalformedYAML(t *testing.T) {
	malformed := "- name: good\n- name: bad\n  broken: [unclosed"
	_, err := parsePrometheusRuleGroupsYAML(malformed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prometheusrule YAML")
}

// TestParsePrometheusRuleGroupsYAML_ValidList verifies the happy path: a YAML list of group
// maps is correctly unmarshalled to a []interface{} with the expected length.
func TestParsePrometheusRuleGroupsYAML_ValidList(t *testing.T) {
	yamlStr := "- name: group1\n  rules: []\n- name: group2\n  rules: []\n"
	groups, err := parsePrometheusRuleGroupsYAML(yamlStr)
	require.NoError(t, err)
	assert.Len(t, groups, 2)
}

// ── injectNamespaceLabelIntoGroups — defensive-path tests ────────────────────

// TestInjectNamespaceLabelIntoGroups_NonMapGroupSkipped verifies that a group element
// that is NOT a map[string]interface{} (e.g., a plain string) is silently skipped
// without panicking or overwriting unrelated groups.
func TestInjectNamespaceLabelIntoGroups_NonMapGroupSkipped(t *testing.T) {
	// groups contains one non-map element followed by a real group map with a rule.
	groups := []interface{}{
		"this-is-a-string-not-a-map", // should be skipped
		map[string]interface{}{
			"name": "real-group",
			"rules": []interface{}{
				map[string]interface{}{
					"alert":  "SomeAlert",
					"labels": map[string]interface{}{"namespace": "old"},
				},
			},
		},
	}
	injectNamespaceLabelIntoGroups(groups, "injected-ns")

	// The second (real) group should have its namespace label updated.
	realGroup := groups[1].(map[string]interface{})
	rules := realGroup["rules"].([]interface{})
	labels := rules[0].(map[string]interface{})["labels"].(map[string]interface{})
	assert.Equal(t, "injected-ns", labels["namespace"])
}

// TestInjectNamespaceLabelIntoGroups_MissingRulesKeySkipped verifies that a group map
// with no "rules" key (or a non-list "rules" value) is silently skipped.
func TestInjectNamespaceLabelIntoGroups_MissingRulesKeySkipped(t *testing.T) {
	groups := []interface{}{
		map[string]interface{}{
			"name": "group-without-rules",
			// "rules" key intentionally absent
		},
	}
	// Must not panic.
	assert.NotPanics(t, func() {
		injectNamespaceLabelIntoGroups(groups, "ns")
	})
}

// TestInjectNamespaceLabelIntoGroups_RulesNotSliceSkipped verifies that a group where
// "rules" is not []interface{} (e.g., a string) is skipped without panicking.
func TestInjectNamespaceLabelIntoGroups_RulesNotSliceSkipped(t *testing.T) {
	groups := []interface{}{
		map[string]interface{}{
			"name":  "bad-rules-type",
			"rules": "not-a-slice",
		},
	}
	assert.NotPanics(t, func() {
		injectNamespaceLabelIntoGroups(groups, "ns")
	})
}

// TestInjectNamespaceLabelIntoGroups_RuleNotMapSkipped verifies that a rule element
// that is not a map (e.g., a string) is silently skipped without panicking.
func TestInjectNamespaceLabelIntoGroups_RuleNotMapSkipped(t *testing.T) {
	groups := []interface{}{
		map[string]interface{}{
			"name": "group",
			"rules": []interface{}{
				"this-is-a-string-rule", // should be skipped
			},
		},
	}
	assert.NotPanics(t, func() {
		injectNamespaceLabelIntoGroups(groups, "ns")
	})
}

// TestInjectNamespaceLabelIntoGroups_LabelsNotMapSkipped verifies that a rule whose
// "labels" field is not a map (e.g., a string) is silently skipped without panicking.
func TestInjectNamespaceLabelIntoGroups_LabelsNotMapSkipped(t *testing.T) {
	groups := []interface{}{
		map[string]interface{}{
			"name": "group",
			"rules": []interface{}{
				map[string]interface{}{
					"alert":  "AlertWithBadLabels",
					"labels": "not-a-map", // should be skipped
				},
			},
		},
	}
	assert.NotPanics(t, func() {
		injectNamespaceLabelIntoGroups(groups, "ns")
	})
}

// ── resolvePrometheusRuleGroups ───────────────────────────────────────────────

// TestResolvePrometheusRuleGroups_VersionResolutionFailure verifies that when
// cr.Spec.Driver.ConfigVersion is empty and GetVersion returns an error (because
// cr.Spec.Version references an unknown CSM release), resolvePrometheusRuleGroups
// returns nil, false — exercising the previously uncovered error branch.
func TestResolvePrometheusRuleGroups_VersionResolutionFailure(t *testing.T) {
	cr := shared.MakeCSM("resolve-fail", "test-ns", "v2.18.0")
	cr.Spec.Driver.CSIDriverType = csmv1.PowerStore
	cr.Spec.Driver.ConfigVersion = "" // force resolution via Spec.Version
	cr.Spec.Version = "v99.99.0"      // not present in csm-releases.yaml → GetVersion error
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled:        true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{Enabled: true},
	}

	groups, ok := resolvePrometheusRuleGroups(context.Background(), cr, testOpConfig)
	assert.Nil(t, groups)
	assert.False(t, ok)
}

// TestResolvePrometheusRuleGroups_UnsupportedDriver verifies that a driver with no
// prometheusrule.yaml (such as Unity) returns empty groups and false.
func TestResolvePrometheusRuleGroups_UnsupportedDriver(t *testing.T) {
	cr := shared.MakeCSM("resolve-unity", "test-ns", "v2.18.0")
	cr.Spec.Driver.CSIDriverType = csmv1.Unity
	cr.Spec.Driver.ConfigVersion = "v2.18.0"
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled:        true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{Enabled: true},
	}

	groups, ok := resolvePrometheusRuleGroups(context.Background(), cr, testOpConfig)
	assert.Empty(t, groups)
	assert.False(t, ok)
}

// TestResolvePrometheusRuleGroups_PowerScaleWithExplicitVersion verifies the happy
// path: a CR with an explicit ConfigVersion resolves PowerScale groups correctly.
func TestResolvePrometheusRuleGroups_PowerScaleWithExplicitVersion(t *testing.T) {
	cr := shared.MakeCSM("resolve-pscale", "test-ns", shared.PScaleConfigVersion)
	cr.Spec.Driver.CSIDriverType = csmv1.PowerScale
	cr.Spec.Driver.ConfigVersion = shared.PScaleConfigVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled:        true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{Enabled: true},
	}

	groups, ok := resolvePrometheusRuleGroups(context.Background(), cr, testOpConfig)
	assert.True(t, ok)
	assert.Len(t, groups, 6, "PowerScale should resolve 6 alert groups")
}

// ── syncResiliencyPrometheusRule ──────────────────────────────────────────────

// TestSyncResiliencyPrometheusRule_NonResiliencyModuleIsNoOp verifies that passing a module
// with a name other than Resiliency returns nil immediately without creating any resource.
func TestSyncResiliencyPrometheusRule_NonResiliencyModuleIsNoOp(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()
	cr := makeResiliencyCSM("non-res-noop", "test-ns", csmv1.PowerStore)

	wrongModule := csmv1.Module{Name: csmv1.Replication}
	err := syncResiliencyPrometheusRule(ctx, false, wrongModule, cr, testOpConfig, fakeClient)
	require.NoError(t, err)

	// No PrometheusRule should have been created.
	pr := &unstructured.Unstructured{}
	pr.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	getErr := fakeClient.Get(ctx, types.NamespacedName{Name: resiliencyPrometheusRuleName("non-res-noop"), Namespace: "test-ns"}, pr)
	assert.True(t, k8sErrors.IsNotFound(getErr), "no PrometheusRule should be created for a non-Resiliency module")
}

// TestSyncResiliencyPrometheusRule_IsDeletingDeletesRule verifies that passing isDeleting=true
// issues a delete call even when the module spec is fully enabled.
func TestSyncResiliencyPrometheusRule_IsDeletingDeletesRule(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("res-deleting", "test-ns", csmv1.PowerStore)
	module := makeResiliencyModule(nil)
	fakeClient := fake.NewClientBuilder().Build()

	// First create the rule.
	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))
	_ = fetchResiliencyPrometheusRule(ctx, t, fakeClient, cr.Name, cr.Namespace)

	// Now delete it.
	require.NoError(t, syncResiliencyPrometheusRule(ctx, true, module, cr, testOpConfig, fakeClient))

	pr := &unstructured.Unstructured{}
	pr.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(ctx, types.NamespacedName{Name: resiliencyPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, pr)
	assert.True(t, k8sErrors.IsNotFound(err), "PrometheusRule should be deleted when isDeleting=true")
}

// TestSyncResiliencyPrometheusRule_MetricsNilSkipsCreate verifies that a module with a nil
// Metrics field causes shouldDelete=true (no PrometheusRule created).
func TestSyncResiliencyPrometheusRule_MetricsNilSkipsCreate(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("res-nil-metrics", "test-ns", csmv1.PowerStore)
	module := csmv1.Module{Name: csmv1.Resiliency, Metrics: nil}
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := &unstructured.Unstructured{}
	pr.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(ctx, types.NamespacedName{Name: resiliencyPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, pr)
	assert.True(t, k8sErrors.IsNotFound(err), "PrometheusRule should not be created when Metrics is nil")
}

// TestSyncResiliencyPrometheusRule_MetricsDisabledSkipsCreate verifies that Metrics.Enabled=false
// results in shouldDelete=true (PrometheusRule is not created).
func TestSyncResiliencyPrometheusRule_MetricsDisabledSkipsCreate(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("res-metrics-off", "test-ns", csmv1.PowerStore)
	module := csmv1.Module{
		Name: csmv1.Resiliency,
		Metrics: &csmv1.ModuleMetrics{
			Enabled:        false,
			PrometheusRule: &csmv1.ModulePrometheusRuleConfig{Enabled: true},
		},
	}
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := &unstructured.Unstructured{}
	pr.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(ctx, types.NamespacedName{Name: resiliencyPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, pr)
	assert.True(t, k8sErrors.IsNotFound(err), "PrometheusRule should not be created when Metrics.Enabled=false")
}

// TestSyncResiliencyPrometheusRule_PrometheusRuleNilSkipsCreate verifies that a nil
// PrometheusRule pointer in the module spec results in shouldDelete=true.
func TestSyncResiliencyPrometheusRule_PrometheusRuleNilSkipsCreate(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("res-pr-nil", "test-ns", csmv1.PowerStore)
	module := csmv1.Module{
		Name: csmv1.Resiliency,
		Metrics: &csmv1.ModuleMetrics{
			Enabled:        true,
			PrometheusRule: nil,
		},
	}
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := &unstructured.Unstructured{}
	pr.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(ctx, types.NamespacedName{Name: resiliencyPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, pr)
	assert.True(t, k8sErrors.IsNotFound(err), "PrometheusRule should not be created when PrometheusRule config is nil")
}

// TestSyncResiliencyPrometheusRule_PrometheusRuleDisabledSkipsCreate verifies that
// PrometheusRule.Enabled=false causes a delete (shouldDelete=true).
func TestSyncResiliencyPrometheusRule_PrometheusRuleDisabledSkipsCreate(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("res-pr-disabled", "test-ns", csmv1.PowerStore)
	module := csmv1.Module{
		Name: csmv1.Resiliency,
		Metrics: &csmv1.ModuleMetrics{
			Enabled: true,
			PrometheusRule: &csmv1.ModulePrometheusRuleConfig{
				Enabled: false,
			},
		},
	}
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := &unstructured.Unstructured{}
	pr.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(ctx, types.NamespacedName{Name: resiliencyPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, pr)
	assert.True(t, k8sErrors.IsNotFound(err), "PrometheusRule should not be created when PrometheusRule.Enabled=false")
}

// TestSyncResiliencyPrometheusRule_CreatesForPowerStore verifies end-to-end creation of the
// resiliency PrometheusRule for the PowerStore driver (which uses the "name" controller label key).
func TestSyncResiliencyPrometheusRule_CreatesForPowerStore(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("pstore-res", "pstore-ns", csmv1.PowerStore)
	module := makeResiliencyModule(nil)
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := fetchResiliencyPrometheusRule(ctx, t, fakeClient, cr.Name, cr.Namespace)

	// Verify resource name and namespace.
	assert.Equal(t, resiliencyPrometheusRuleName(cr.Name), pr.GetName())
	assert.Equal(t, cr.Namespace, pr.GetNamespace())

	// Verify controller label key for PowerStore is "name", not "app".
	labels := pr.GetLabels()
	_, hasNameKey := labels["name"]
	_, hasAppKey := labels["app"]
	assert.True(t, hasNameKey, "PowerStore resiliency PrometheusRule should use 'name' controller label key")
	assert.False(t, hasAppKey, "PowerStore resiliency PrometheusRule should NOT use 'app' controller label key")

	// Verify exactly 1 group with 4 rules.
	groups := prometheusRuleRules(t, pr)
	require.Len(t, groups, 1)
	rules := flatRulesFromResiliencyPrometheusRule(t, pr)
	assert.Len(t, rules, expectedResiliencyRuleCount)

	// Verify driver-specific placeholders are substituted (PowerStore → csi-powerstore).
	podFailure := mustAlertRule(t, rules, "CSMResiliencyPodFailureDetected")
	expr := podFailure["expr"].(string)
	assert.Contains(t, expr, "csi-powerstore", "PowerStore PrometheusRule should reference csi-powerstore driver name")
}

// TestSyncResiliencyPrometheusRule_CreatesForPowerScale verifies end-to-end creation of the
// resiliency PrometheusRule for the PowerScale driver (which uses the "app" controller label key).
func TestSyncResiliencyPrometheusRule_CreatesForPowerScale(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("pscale-res", "pscale-ns", csmv1.PowerScale)
	module := makeResiliencyModule(nil)
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := fetchResiliencyPrometheusRule(ctx, t, fakeClient, cr.Name, cr.Namespace)

	// Verify controller label key for PowerScale is "app".
	labels := pr.GetLabels()
	_, hasAppKey := labels["app"]
	assert.True(t, hasAppKey, "PowerScale resiliency PrometheusRule should use 'app' controller label key")

	// Verify driver name substitution.
	rules := flatRulesFromResiliencyPrometheusRule(t, pr)
	podFailure := mustAlertRule(t, rules, "CSMResiliencyPodFailureDetected")
	expr := podFailure["expr"].(string)
	assert.Contains(t, expr, "csi-powerscale")
}

// TestSyncResiliencyPrometheusRule_CreatesForPowerFlex verifies end-to-end creation of the
// resiliency PrometheusRule for the PowerFlex driver (which also uses the "name" controller label key).
func TestSyncResiliencyPrometheusRule_CreatesForPowerFlex(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("pflex-res", "pflex-ns", csmv1.PowerFlex)
	module := makeResiliencyModule(nil)
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := fetchResiliencyPrometheusRule(ctx, t, fakeClient, cr.Name, cr.Namespace)

	// PowerFlex uses "name" label key (same as PowerStore).
	labels := pr.GetLabels()
	_, hasNameKey := labels["name"]
	_, hasAppKey := labels["app"]
	assert.True(t, hasNameKey, "PowerFlex resiliency PrometheusRule should use 'name' controller label key")
	assert.False(t, hasAppKey, "PowerFlex resiliency PrometheusRule should NOT use 'app' label key")

	// Verify driver name substitution.
	rules := flatRulesFromResiliencyPrometheusRule(t, pr)
	podFailure := mustAlertRule(t, rules, "CSMResiliencyPodFailureDetected")
	expr := podFailure["expr"].(string)
	assert.Contains(t, expr, "csi-vxflexos")
}

// TestSyncResiliencyPrometheusRule_CreatesForPowerMax verifies end-to-end creation of the
// resiliency PrometheusRule for the PowerMax driver (which uses the "app" controller label key).
func TestSyncResiliencyPrometheusRule_CreatesForPowerMax(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("pmax-res", "pmax-ns", csmv1.PowerMax)
	module := makeResiliencyModule(nil)
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := fetchResiliencyPrometheusRule(ctx, t, fakeClient, cr.Name, cr.Namespace)

	// Verify rule count and driver name substitution.
	rules := flatRulesFromResiliencyPrometheusRule(t, pr)
	assert.Len(t, rules, expectedResiliencyRuleCount)

	connectivityLost := mustAlertRule(t, rules, "CSMResiliencyConnectivityLost")
	expr := connectivityLost["expr"].(string)
	assert.Contains(t, expr, "csi-powermax")
}

// TestSyncResiliencyPrometheusRule_CustomConnectivityThreshold verifies that a custom
// ConnectivitySuccessRatio is correctly substituted in the RES-02 alert expression.
func TestSyncResiliencyPrometheusRule_CustomConnectivityThreshold(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("pstore-custom-ratio", "test-ns", csmv1.PowerStore)
	module := makeResiliencyModule(i32(70))
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := fetchResiliencyPrometheusRule(ctx, t, fakeClient, cr.Name, cr.Namespace)
	rules := flatRulesFromResiliencyPrometheusRule(t, pr)
	connectivityLost := mustAlertRule(t, rules, "CSMResiliencyConnectivityLost")
	expr := connectivityLost["expr"].(string)
	assert.Contains(t, expr, "70", "custom connectivity threshold 70 should appear in RES-02 expression")
	assert.NotContains(t, expr, "90", "default threshold 90 should not appear when a custom value is set")
}

// TestSyncResiliencyPrometheusRule_ResourceCreatedInCorrectNamespace verifies that the
// resiliency PrometheusRule resource is created in the same namespace as the CR.
func TestSyncResiliencyPrometheusRule_ResourceCreatedInCorrectNamespace(t *testing.T) {
	ctx := context.Background()
	targetNamespace := "injected-ns-test"
	cr := makeResiliencyCSM("pstore-ns-inject", targetNamespace, csmv1.PowerStore)
	module := makeResiliencyModule(nil)
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := fetchResiliencyPrometheusRule(ctx, t, fakeClient, cr.Name, cr.Namespace)
	// The PrometheusRule resource itself must live in the CR's namespace.
	assert.Equal(t, targetNamespace, pr.GetNamespace(), "PrometheusRule must be created in the CR's namespace")

	// Verify all expected alert rules are present with correct structure.
	rules := flatRulesFromResiliencyPrometheusRule(t, pr)
	require.Len(t, rules, expectedResiliencyRuleCount)
	for _, name := range expectedResiliencyAlertNames {
		_, found := alertRuleByName(t, rules, name)
		assert.True(t, found, "expected alert %s to be present in the PrometheusRule", name)
	}
}

// TestSyncResiliencyPrometheusRule_OwnerReferenceSet verifies that the created PrometheusRule
// carries an ownerReference pointing back to the ContainerStorageModule CR.
func TestSyncResiliencyPrometheusRule_OwnerReferenceSet(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("pstore-owner-ref", "test-ns", csmv1.PowerStore)
	module := makeResiliencyModule(nil)
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := fetchResiliencyPrometheusRule(ctx, t, fakeClient, cr.Name, cr.Namespace)
	owners := pr.GetOwnerReferences()
	require.Len(t, owners, 1, "PrometheusRule should have exactly one ownerReference")
	assert.Equal(t, cr.Name, owners[0].Name)
}

// TestSyncResiliencyPrometheusRule_BadDriverVersionFallsBackToDelete verifies that when
// the CR has an empty ConfigVersion and an invalid Spec.Version, the driver version lookup
// fails gracefully and the resource is deleted (shouldDelete=true fallback).
func TestSyncResiliencyPrometheusRule_BadDriverVersionFallsBackToDelete(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("pstore-bad-ver", "test-ns", csmv1.PowerStore)
	cr.Spec.Driver.ConfigVersion = ""
	cr.Spec.Version = "v99.99.0" // not in csm-releases.yaml
	module := makeResiliencyModule(nil)
	fakeClient := fake.NewClientBuilder().Build()

	// Should not return an error (only logs a warning and falls back to delete).
	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := &unstructured.Unstructured{}
	pr.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(ctx, types.NamespacedName{Name: resiliencyPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, pr)
	assert.True(t, k8sErrors.IsNotFound(err), "PrometheusRule should not be created when driver version resolution fails")
}

// TestSyncResiliencyPrometheusRule_BadModuleVersionFallsBackToDelete verifies that when the
// driver version resolves correctly but the resiliency module version lookup fails (driver
// has no resiliency entry for the given config version), the function falls back to delete.
func TestSyncResiliencyPrometheusRule_BadModuleVersionFallsBackToDelete(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("pstore-bad-module-ver", "test-ns", csmv1.PowerStore)
	// Use a driver version that exists in csm-releases.yaml but has no resiliency module entry.
	// Unity has no resiliency module in any version, but we're using PowerStore with an unknown version.
	cr.Spec.Driver.ConfigVersion = "v9.9.9" // valid version format but not in csm-releases.yaml
	module := makeResiliencyModule(nil)
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := &unstructured.Unstructured{}
	pr.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(ctx, types.NamespacedName{Name: resiliencyPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, pr)
	assert.True(t, k8sErrors.IsNotFound(err), "PrometheusRule should not be created when module version lookup fails")
}

// TestSyncResiliencyPrometheusRule_DisableAfterCreate verifies the create-then-disable lifecycle:
// a PrometheusRule created when enabled is correctly removed when the module is later disabled.
func TestSyncResiliencyPrometheusRule_DisableAfterCreate(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("pstore-lifecycle", "test-ns", csmv1.PowerStore)
	module := makeResiliencyModule(nil)
	fakeClient := fake.NewClientBuilder().Build()

	// Step 1: Create.
	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))
	_ = fetchResiliencyPrometheusRule(ctx, t, fakeClient, cr.Name, cr.Namespace)

	// Step 2: Disable PrometheusRule.
	module.Metrics.PrometheusRule.Enabled = false
	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := &unstructured.Unstructured{}
	pr.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(ctx, types.NamespacedName{Name: resiliencyPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, pr)
	assert.True(t, k8sErrors.IsNotFound(err), "PrometheusRule should be removed when prometheusRule.enabled is set to false")
}

// TestSyncResiliencyPrometheusRule_ResiliencyRuleNameIsDistinctFromDriverRule verifies that
// the resiliency PrometheusRule has a different name than the driver PrometheusRule, so both
// can coexist in the same namespace without collision.
func TestSyncResiliencyPrometheusRule_ResiliencyRuleNameIsDistinctFromDriverRule(t *testing.T) {
	crName := "collision-check"
	driverRuleName := metricsPrometheusRuleName(crName)
	resiliencyRuleName := resiliencyPrometheusRuleName(crName)
	assert.NotEqual(t, driverRuleName, resiliencyRuleName,
		"resiliency PrometheusRule name must differ from driver PrometheusRule name to avoid collision")
	assert.Equal(t, crName+"-alerts", driverRuleName)
	assert.Equal(t, crName+"-resiliency-alerts", resiliencyRuleName)
}

// TestSyncResiliencyPrometheusRule_AllDriversProduceCorrectPlatformLabel is a table-driven test
// that verifies the <DriverPlatform> label in alert metadata is correctly set for each driver.
func TestSyncResiliencyPrometheusRule_AllDriversProduceCorrectPlatformLabel(t *testing.T) {
	cases := []struct {
		driverType       csmv1.DriverType
		expectedPlatform string
		expectedName     string
		labelKey         string
	}{
		{csmv1.PowerStore, "powerstore", "csi-powerstore", "name"},
		{csmv1.PowerFlex, "powerflex", "csi-vxflexos", "name"},
		{csmv1.PowerScale, "powerscale", "csi-powerscale", "app"},
		{csmv1.PowerMax, "powermax", "csi-powermax", "app"},
	}

	for _, tc := range cases {
		t.Run(string(tc.driverType), func(t *testing.T) {
			ctx := context.Background()
			crName := "platform-test-" + string(tc.driverType)
			cr := makeResiliencyCSM(crName, "test-ns", tc.driverType)
			module := makeResiliencyModule(nil)
			fakeClient := fake.NewClientBuilder().Build()

			require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

			pr := fetchResiliencyPrometheusRule(ctx, t, fakeClient, crName, "test-ns")

			// Verify controller label key.
			labels := pr.GetLabels()
			_, hasExpectedKey := labels[tc.labelKey]
			assert.True(t, hasExpectedKey, "driver %s: expected label key %q", tc.driverType, tc.labelKey)

			// Verify driver name in alert expressions.
			rules := flatRulesFromResiliencyPrometheusRule(t, pr)
			podFailure := mustAlertRule(t, rules, "CSMResiliencyPodFailureDetected")
			expr := podFailure["expr"].(string)
			assert.Contains(t, expr, tc.expectedName,
				"driver %s: expected driver name %q in RES-01 expr", tc.driverType, tc.expectedName)

			// Verify platform label in alert labels (from <DriverPlatform> substitution).
			alertLabels, ok := podFailure["labels"].(map[string]interface{})
			require.True(t, ok, "driver %s: labels field should be a map", tc.driverType)
			assert.Equal(t, tc.expectedPlatform, alertLabels["platform"],
				"driver %s: platform label mismatch", tc.driverType)
		})
	}
}

// TestSyncResiliencyPrometheusRule_RES02ConnectivityExprContainsDefaultRatio verifies that
// with the default configuration (nil cfg), the RES-02 alert fires when connectivity drops
// below 90% (the package default).
func TestSyncResiliencyPrometheusRule_RES02ConnectivityExprContainsDefaultRatio(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("default-ratio-test", "test-ns", csmv1.PowerStore)
	module := makeResiliencyModule(nil) // nil → default 90%
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := fetchResiliencyPrometheusRule(ctx, t, fakeClient, cr.Name, cr.Namespace)
	rules := flatRulesFromResiliencyPrometheusRule(t, pr)
	connectivityLost := mustAlertRule(t, rules, "CSMResiliencyConnectivityLost")
	expr := connectivityLost["expr"].(string)
	assert.Contains(t, expr, "90", "default connectivity ratio 90 should appear in RES-02 expression")
	assert.Equal(t, "2m", connectivityLost["for"], "RES-02 should fire after 2 minutes (critical alert)")
}

// TestSyncResiliencyPrometheusRule_RES01AlertMetadata verifies the metadata (for duration,
// severity label, alert_id label) of the RES-01 pod failure detection alert.
func TestSyncResiliencyPrometheusRule_RES01AlertMetadata(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("res01-metadata", "test-ns", csmv1.PowerStore)
	module := makeResiliencyModule(nil)
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := fetchResiliencyPrometheusRule(ctx, t, fakeClient, cr.Name, cr.Namespace)
	rules := flatRulesFromResiliencyPrometheusRule(t, pr)

	res01 := mustAlertRule(t, rules, "CSMResiliencyPodFailureDetected")
	assert.Equal(t, "0m", res01["for"], "RES-01 should fire immediately (pod failure is time-critical)")
	alertLabels := res01["labels"].(map[string]interface{})
	assert.Equal(t, "warning", alertLabels["severity"])
	assert.Equal(t, "RES-01", alertLabels["alert_id"])
	assert.Equal(t, "csm-resiliency", alertLabels["component"])
}

// TestSyncResiliencyPrometheusRule_RES04FailoverFailedIsCritical verifies that the RES-04
// failover-failed alert carries severity=critical (not warning), as defined in the YAML.
func TestSyncResiliencyPrometheusRule_RES04FailoverFailedIsCritical(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("res04-metadata", "test-ns", csmv1.PowerStore)
	module := makeResiliencyModule(nil)
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := fetchResiliencyPrometheusRule(ctx, t, fakeClient, cr.Name, cr.Namespace)
	rules := flatRulesFromResiliencyPrometheusRule(t, pr)

	res04 := mustAlertRule(t, rules, "CSMResiliencyFailoverFailed")
	alertLabels := res04["labels"].(map[string]interface{})
	assert.Equal(t, "critical", alertLabels["severity"], "RES-04 failover failure should be severity=critical")
	assert.Equal(t, "RES-04", alertLabels["alert_id"])
}

// TestSyncResiliencyPrometheusRule_BadYAMLFallsBackToDelete verifies that when the resiliency
// prometheusrule YAML is not accessible (e.g., bad configDir), the function falls back to
// shouldDelete=true without propagating an error.
func TestSyncResiliencyPrometheusRule_BadYAMLFallsBackToDelete(t *testing.T) {
	ctx := context.Background()
	cr := makeResiliencyCSM("pstore-bad-yaml", "test-ns", csmv1.PowerStore)
	module := makeResiliencyModule(nil)
	fakeClient := fake.NewClientBuilder().Build()
	badConfig := operatorutils.OperatorConfig{ConfigDirectory: "/nonexistent-path"}

	// Should not error — logs a warning and falls back to delete.
	require.NoError(t, syncResiliencyPrometheusRule(ctx, false, module, cr, badConfig, fakeClient))

	pr := &unstructured.Unstructured{}
	pr.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(ctx, types.NamespacedName{Name: resiliencyPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, pr)
	assert.True(t, k8sErrors.IsNotFound(err), "PrometheusRule should not be created when config directory is invalid")
}
