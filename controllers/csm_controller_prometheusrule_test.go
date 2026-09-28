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

package controllers

import (
	"context"
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
	sigsyaml "sigs.k8s.io/yaml"
)

// testConfigDir is the relative path to operatorconfig from the controllers package.
const testConfigDir = "../operatorconfig"

// testPowerScaleVersion is the PowerScale driver version whose prometheusrule.yaml is under test.
const testPowerScaleVersion = "v2.18.0"

// testPowerFlexVersion is the PowerFlex driver version whose prometheusrule.yaml is under test.
const testPowerFlexVersion = "v2.18.0"

// testPowerStoreVersion is the PowerStore driver version whose prometheusrule.yaml is under test.
const testPowerStoreVersion = "v2.18.0"

// testPowerMaxVersion is the PowerMax driver version whose prometheusrule.yaml is under test.
const testPowerMaxVersion = "v2.18.0"

// testOpConfig points syncMetricsResources at the real operatorconfig directory so that
// the driver prometheusrule.yaml files can be loaded during tests.
var testOpConfig = operatorutils.OperatorConfig{ConfigDirectory: testConfigDir}

// expectedPowerScaleGroupNames lists the six named groups in the PowerScale prometheusrule.yaml.
var expectedPowerScaleGroupNames = []string{
	"powerscale-csi-volume-operations",
	"powerscale-csi-driver-health",
	"powerscale-csi-quota",
	"powerscale-csi-nodepool",
	"powerscale-csi-access-control",
	"powerscale-csi-nfs-performance",
}

// expectedPowerFlexGroupNames lists the four named groups in the PowerFlex prometheusrule.yaml.
var expectedPowerFlexGroupNames = []string{
	"powerflex-csi-volume-operations",
	"powerflex-csi-driver-health",
	"powerflex-csi-storage-capacity",
	"powerflex-csi-rcg-replication",
}

// expectedPowerStoreGroupNames lists the group names defined in the PowerStore prometheusrule.yaml.
var expectedPowerStoreGroupNames = []string{
	"powerstore-csi-volume-operations",
	"powerstore-csi-driver-health",
	"powerstore-csi-appliance-capacity",
}

// expectedPowerMaxGroupNames lists the three named groups in the PowerMax prometheusrule.yaml.
var expectedPowerMaxGroupNames = []string{
	"powermax-csi-volume-operations",
	"powermax-csi-driver-health",
	"powermax-csi-capacity",
}

// TestBuildDriverAlertGroups_Dispatches verifies the dispatch switch:
// PowerScale (both type aliases) returns 6 groups (22 rules total);
// PowerFlex (both type aliases) returns 4 groups (26 rules total);
// PowerStore returns 3 groups (18 rules total);
// PowerMax returns 3 groups (15 rules total); unsupported drivers return empty.
func TestBuildDriverAlertGroups_Dispatches(t *testing.T) {
	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerScale, testPowerScaleVersion, nil)
	require.NoError(t, err)
	rules := flattenGroupRules(t, groups)
	require.Len(t, rules, 22)

	groupsAlt, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerScaleName, testPowerScaleVersion, nil)
	require.NoError(t, err)
	rulesAlt := flattenGroupRules(t, groupsAlt)
	require.Len(t, rulesAlt, 22)

	pstoreGroups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerStore, testPowerStoreVersion, nil)
	require.NoError(t, err)
	pstoreRules := flattenGroupRules(t, pstoreGroups)
	require.Len(t, pstoreRules, 18)

	pmaxGroups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerMax, testPowerMaxVersion, nil)
	require.NoError(t, err)
	require.Len(t, pmaxGroups, 3, "PowerMax should have 3 named groups")
	pmaxRules := flattenGroupRules(t, pmaxGroups)
	require.Len(t, pmaxRules, 15, "PowerMax should have 15 rules total across all groups")

	flexGroups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerFlex, testPowerFlexVersion, nil)
	require.NoError(t, err)
	require.Len(t, flexGroups, 4, "PowerFlex should have 4 named groups")

	flexGroupsAlt, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerFlexName, testPowerFlexVersion, nil)
	require.NoError(t, err)
	require.Len(t, flexGroupsAlt, 4)

	unityGroups, err := buildDriverAlertGroups(testConfigDir, csmv1.Unity, testPowerScaleVersion, nil)
	require.NoError(t, err)
	assert.Empty(t, unityGroups)
}

// TestBuildDriverAlertGroups_MissingYAML verifies that a meaningful error is returned when
// the prometheusrule.yaml file does not exist for the given driver/version.
func TestBuildDriverAlertGroups_MissingYAML(t *testing.T) {
	_, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerScale, "v0.0.0-nonexistent", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prometheusrule.yaml")
}

// TestBuildPowerScaleAlertGroups_GroupStructure verifies that the YAML file is loaded correctly
// with defaults, producing the expected 6 groups with correct names and interval settings.
func TestBuildPowerScaleAlertGroups_GroupStructure(t *testing.T) {
	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerScale, testPowerScaleVersion, nil)
	require.NoError(t, err)
	require.Len(t, groups, 6)

	for i, expectedName := range expectedPowerScaleGroupNames {
		groupMap, ok := groups[i].(map[string]interface{})
		require.True(t, ok, "group %d should be a map", i)
		assert.Equal(t, expectedName, groupMap["name"], "group %d name mismatch", i)
		assert.Equal(t, "30s", groupMap["interval"], "group %d should have interval:30s", i)
	}

	// Verify total rule count across all groups is 22.
	allRules := flattenGroupRules(t, groups)
	assert.Len(t, allRules, 22, "total rule count across all groups should be 22")
}

// TestBuildPowerScaleAlertGroups_DefaultRules verifies that the YAML file is loaded correctly
// with defaults, producing the expected 22 rules with correct structure and PromQL expressions.
func TestBuildPowerScaleAlertGroups_DefaultRules(t *testing.T) {
	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerScale, testPowerScaleVersion, nil)
	require.NoError(t, err)
	rules := flattenGroupRules(t, groups)
	require.Len(t, rules, 22)

	expectedAlerts := []string{
		"PowerScaleCreateVolumeFailure",
		"PowerScaleDeleteVolumeFailure",
		"PowerScaleControllerPublishFailure",
		"PowerScaleControllerUnpublishFailure",
		"PowerScaleNodeStageFailure",
		"PowerScaleNodeUnstageFailure",
		"PowerScaleNodePublishFailure",
		"PowerScaleNodeUnpublishFailure",
		"PowerScaleDriverUnavailable",
		"PowerScaleDriverCrashLooping",
		"PowerScaleDriverHighCPU",
		"PowerScaleDriverHighMemory",
		"PowerScaleMetricsStale",
		"PowerScaleQuotaWarning",
		"PowerScaleQuotaCritical",
		"PowerScaleQuotaHardBreach",
		"PowerScaleNodePoolWarning",
		"PowerScaleNodePoolCritical",
		"PowerScaleNFSAuthFailureRate",
		"PowerScaleAPIErrorRate",
		"PowerScaleNFSv3HighLatency",
		"PowerScaleNFSv4HighLatency",
	}
	for _, alertName := range expectedAlerts {
		_, found := alertRuleByName(t, rules, alertName)
		assert.True(t, found, "expected alert %s", alertName)
	}

	createVolume := mustAlertRule(t, rules, "PowerScaleCreateVolumeFailure")
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="CreateVolume"}[5m]) > 3`, createVolume["expr"])
	assert.Equal(t, "1m", createVolume["for"])
	assert.Equal(t, "warning", nestedStringField(t, createVolume, "labels", "severity"))
	assert.Equal(t, "powerscale", nestedStringField(t, createVolume, "labels", "platform"))
	assert.Equal(t, "csi-driver", nestedStringField(t, createVolume, "labels", "component"))
	// descriptions include printf-formatted $value (failure count) and threshold
	assert.Contains(t, nestedStringField(t, createVolume, "annotations", "description"), `printf "%.0f"`)
	assert.Contains(t, nestedStringField(t, createVolume, "annotations", "description"), "{{ $labels.error_code }}")
	assert.Contains(t, nestedStringField(t, createVolume, "annotations", "description"), "3 failures")
	// G-2: runbook_url placeholder is present
	assert.Equal(t, "", nestedStringField(t, createVolume, "annotations", "runbook_url"))
	// G-18: per-operation specific summary
	assert.Equal(t, "PowerScale CSI CreateVolume operation failures detected", nestedStringField(t, createVolume, "annotations", "summary"))

	crashLoop := mustAlertRule(t, rules, "PowerScaleDriverCrashLooping")
	// G-6: uses changes() not delta()
	assert.Equal(t, "changes(dell_csi_driver_restart_total[15m]) > 3", crashLoop["expr"])
	assert.Equal(t, "0m", crashLoop["for"])
	// G-7 (crash loop specific): description includes $value
	assert.Contains(t, nestedStringField(t, crashLoop, "annotations", "description"), "{{ $value }}")

	driverUnavailable := mustAlertRule(t, rules, "PowerScaleDriverUnavailable")
	// G-4: correct count-based expression (not absent())
	assert.Equal(t, `(count(dell_csi_driver_uptime_seconds{cluster_name=~".+",instance_type="controller"}) or on() vector(0)) == 0 or (count(dell_csi_driver_uptime_seconds{cluster_name=~".+",instance_type="node"}) or on() vector(0)) == 0`, driverUnavailable["expr"])
	// G-5: description does not reference $labels.cluster_name (would be empty with count expr)
	desc := nestedStringField(t, driverUnavailable, "annotations", "description")
	assert.Equal(t, "PowerScale CSI driver has zero controller instances or zero node instances reporting metrics for 5 minutes.", desc)

	cpu := mustAlertRule(t, rules, "PowerScaleDriverHighCPU")
	assert.Equal(t, "dell_csi_driver_cpu_usage_percent > 80", cpu["expr"])
	assert.Equal(t, "5m", cpu["for"])
	// G-8: description includes $value and threshold
	assert.Contains(t, nestedStringField(t, cpu, "annotations", "description"), "{{ $value }}")
	assert.Contains(t, nestedStringField(t, cpu, "annotations", "description"), "80%")

	memory := mustAlertRule(t, rules, "PowerScaleDriverHighMemory")
	// G-8: description includes $value and threshold
	assert.Contains(t, nestedStringField(t, memory, "annotations", "description"), "{{ $value }}")
	assert.Contains(t, nestedStringField(t, memory, "annotations", "description"), "2147483648")

	// G-9: MetricsStale description explains cache semantics
	metricsStale := mustAlertRule(t, rules, "PowerScaleMetricsStale")
	assert.Contains(t, nestedStringField(t, metricsStale, "annotations", "description"), "cached data")

	quotaWarning := mustAlertRule(t, rules, "PowerScaleQuotaWarning")
	assert.Equal(t, "dell_powerscale_quota_utilization_ratio * 100 > 80", quotaWarning["expr"])
	assert.Equal(t, "{{ $labels.access_zone }}", nestedStringField(t, quotaWarning, "labels", "access_zone"))
	// G-10: uses volume_id not quota_id
	assert.Equal(t, "Quota warning for volume {{ $labels.volume_id }} in access zone {{ $labels.access_zone }}", nestedStringField(t, quotaWarning, "annotations", "summary"))
	// G-11: formatted float in description
	assert.Contains(t, nestedStringField(t, quotaWarning, "annotations", "description"), `printf "%.1f"`)
	// G-12: threshold in description
	assert.Contains(t, nestedStringField(t, quotaWarning, "annotations", "description"), "80%")

	quotaCritical := mustAlertRule(t, rules, "PowerScaleQuotaCritical")
	// G-10: uses volume_id not quota_id
	assert.Equal(t, "Quota critical for volume {{ $labels.volume_id }} in access zone {{ $labels.access_zone }}", nestedStringField(t, quotaCritical, "annotations", "summary"))

	// G-13: QuotaHardBreach description includes breach count and remediation pointer
	quotaHardBreach := mustAlertRule(t, rules, "PowerScaleQuotaHardBreach")
	hardBreachDesc := nestedStringField(t, quotaHardBreach, "annotations", "description")
	assert.Contains(t, hardBreachDesc, "dell_powerscale_quota_utilization_ratio")
	assert.Contains(t, hardBreachDesc, `printf "%.0f"`)

	// G-14: NodePool descriptions include $value and threshold
	nodePoolWarning := mustAlertRule(t, rules, "PowerScaleNodePoolWarning")
	assert.Contains(t, nestedStringField(t, nodePoolWarning, "annotations", "description"), "{{ $value }}")
	assert.Contains(t, nestedStringField(t, nodePoolWarning, "annotations", "description"), "80%")

	nodePoolCritical := mustAlertRule(t, rules, "PowerScaleNodePoolCritical")
	assert.Contains(t, nestedStringField(t, nodePoolCritical, "annotations", "description"), "{{ $value }}")
	assert.Contains(t, nestedStringField(t, nodePoolCritical, "annotations", "description"), "90%")

	// G-15: NFSAuthFailureRate description includes $value
	nfsAuth := mustAlertRule(t, rules, "PowerScaleNFSAuthFailureRate")
	assert.Contains(t, nestedStringField(t, nfsAuth, "annotations", "description"), "{{ $value }}")

	apiErrorRate := mustAlertRule(t, rules, "PowerScaleAPIErrorRate")
	assert.Equal(t, `(rate(dell_powerscale_api_calls_total{status="failure"}[5m]) / ignoring(status) sum without(status) (rate(dell_powerscale_api_calls_total[5m]))) * 100 > 10`, apiErrorRate["expr"])
	// G-16: description includes $value, window, and threshold
	apiDesc := nestedStringField(t, apiErrorRate, "annotations", "description")
	assert.Contains(t, apiDesc, "{{ $value }}")
	assert.Contains(t, apiDesc, "5m")
	assert.Contains(t, apiDesc, "10%")

	nfsV3 := mustAlertRule(t, rules, "PowerScaleNFSv3HighLatency")
	assert.Equal(t, "histogram_quantile(0.99, rate(dell_powerscale_nfs_v3_latency_seconds_bucket[5m])) > 10", nfsV3["expr"])
	nfsV3Desc := nestedStringField(t, nfsV3, "annotations", "description")
	// G-17: uses humanizeDuration
	assert.Contains(t, nfsV3Desc, "humanizeDuration")
	// G-17: threshold in description
	assert.Contains(t, nfsV3Desc, "10s")
	assert.Contains(t, nfsV3Desc, "latency is bucket-approximated from the OneFS optime API; minimum resolution is 100ms")

	nfsV4 := mustAlertRule(t, rules, "PowerScaleNFSv4HighLatency")
	nfsV4Desc := nestedStringField(t, nfsV4, "annotations", "description")
	assert.Contains(t, nfsV4Desc, "humanizeDuration")
	assert.Contains(t, nfsV4Desc, "10s")
}

// TestBuildPowerScaleAlertGroups_UsesCustomThresholds verifies that threshold placeholders in
// the YAML are correctly replaced with values from the MetricsPrometheusRuleConfig.
func TestBuildPowerScaleAlertGroups_UsesCustomThresholds(t *testing.T) {
	volumeOpThreshold := int32(5)
	quotaWarningThreshold := int32(75)
	quotaCriticalThreshold := int32(95)
	nodepoolWarningThreshold := int32(81)
	nodepoolCriticalThreshold := int32(96)
	apiErrorRateThreshold := int32(15)
	apiErrorRateWindow := "10m"
	cpuWarningThreshold := int32(85)
	memoryWarningThreshold := int64(4096)
	nfsV3LatencyWarningSeconds := int32(7)
	nfsV4LatencyWarningSeconds := int32(9)
	driverCrashLoopRestartThreshold := int32(4)
	driverCrashLoopWindow := "20m"

	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerScale, testPowerScaleVersion, &csmv1.MetricsPrometheusRuleConfig{
		VolumeOperationFailureThreshold: &volumeOpThreshold,
		QuotaWarningThreshold:           &quotaWarningThreshold,
		QuotaCriticalThreshold:          &quotaCriticalThreshold,
		NodepoolWarningThreshold:        &nodepoolWarningThreshold,
		NodepoolCriticalThreshold:       &nodepoolCriticalThreshold,
		APIErrorRateThreshold:           &apiErrorRateThreshold,
		APIErrorRateWindow:              &apiErrorRateWindow,
		CPUWarningThreshold:             &cpuWarningThreshold,
		MemoryWarningThreshold:          &memoryWarningThreshold,
		NFSv3LatencyWarningSeconds:      &nfsV3LatencyWarningSeconds,
		NFSv4LatencyWarningSeconds:      &nfsV4LatencyWarningSeconds,
		DriverCrashLoopRestartThreshold: &driverCrashLoopRestartThreshold,
		DriverCrashLoopWindow:           &driverCrashLoopWindow,
	})
	require.NoError(t, err)
	require.Len(t, groups, 6)

	rules := flattenGroupRules(t, groups)
	require.Len(t, rules, 22)

	// Volume operation alerts use the custom threshold (PS-01 to PS-08)
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="CreateVolume"}[5m]) > 5`, mustAlertRule(t, rules, "PowerScaleCreateVolumeFailure")["expr"])
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="NodeStageVolume"}[5m]) > 5`, mustAlertRule(t, rules, "PowerScaleNodeStageFailure")["expr"])
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="NodeUnpublishVolume"}[5m]) > 5`, mustAlertRule(t, rules, "PowerScaleNodeUnpublishFailure")["expr"])
	assert.Equal(t, "changes(dell_csi_driver_restart_total[20m]) > 4", mustAlertRule(t, rules, "PowerScaleDriverCrashLooping")["expr"])
	assert.Equal(t, "dell_csi_driver_cpu_usage_percent > 85", mustAlertRule(t, rules, "PowerScaleDriverHighCPU")["expr"])
	assert.Equal(t, "dell_csi_driver_memory_usage_bytes > 4096", mustAlertRule(t, rules, "PowerScaleDriverHighMemory")["expr"])
	assert.Equal(t, "dell_powerscale_quota_utilization_ratio * 100 > 75", mustAlertRule(t, rules, "PowerScaleQuotaWarning")["expr"])
	assert.Equal(t, "dell_powerscale_quota_utilization_ratio * 100 > 95", mustAlertRule(t, rules, "PowerScaleQuotaCritical")["expr"])
	assert.Equal(t, "dell_powerscale_nodepool_utilization_percent > 81", mustAlertRule(t, rules, "PowerScaleNodePoolWarning")["expr"])
	assert.Equal(t, "dell_powerscale_nodepool_utilization_percent > 96", mustAlertRule(t, rules, "PowerScaleNodePoolCritical")["expr"])
	assert.Equal(t, `(rate(dell_powerscale_api_calls_total{status="failure"}[10m]) / ignoring(status) sum without(status) (rate(dell_powerscale_api_calls_total[10m]))) * 100 > 15`, mustAlertRule(t, rules, "PowerScaleAPIErrorRate")["expr"])
	assert.Equal(t, "histogram_quantile(0.99, rate(dell_powerscale_nfs_v3_latency_seconds_bucket[5m])) > 7", mustAlertRule(t, rules, "PowerScaleNFSv3HighLatency")["expr"])
	assert.Equal(t, "histogram_quantile(0.99, rate(dell_powerscale_nfs_v4_latency_seconds_bucket[5m])) > 9", mustAlertRule(t, rules, "PowerScaleNFSv4HighLatency")["expr"])
}

// TestBuildPowerFlexAlertGroups_GroupStructure verifies that the PowerFlex YAML is loaded
// correctly, producing the expected 4 groups with correct names.
func TestBuildPowerFlexAlertGroups_GroupStructure(t *testing.T) {
	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerFlex, testPowerFlexVersion, nil)
	require.NoError(t, err)
	require.Len(t, groups, 4)

	for i, expectedName := range expectedPowerFlexGroupNames {
		groupMap, ok := groups[i].(map[string]interface{})
		require.True(t, ok, "group %d should be a map", i)
		assert.Equal(t, expectedName, groupMap["name"], "group %d name mismatch", i)
		assert.Equal(t, "30s", groupMap["interval"], "group %d should have interval:30s", i)
	}

	allRules := flattenGroupRules(t, groups)
	assert.Len(t, allRules, 26, "total rule count across all groups should be 26")
}

// TestBuildPowerFlexAlertGroups_DefaultRules verifies that the YAML file is loaded correctly
// with defaults, producing the expected 26 rules with correct structure and PromQL expressions.
func TestBuildPowerFlexAlertGroups_DefaultRules(t *testing.T) {
	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerFlex, testPowerFlexVersion, nil)
	require.NoError(t, err)
	require.Len(t, groups, 4)

	rules := flattenGroupRules(t, groups)
	require.Len(t, rules, 26)

	expectedAlerts := []string{
		"PowerFlexCreateVolumeFailure",
		"PowerFlexDeleteVolumeFailure",
		"PowerFlexControllerPublishFailure",
		"PowerFlexControllerUnpublishFailure",
		"PowerFlexNodeStageFailure",
		"PowerFlexNodeUnstageFailure",
		"PowerFlexNodePublishFailure",
		"PowerFlexNodeUnpublishFailure",
		"PowerFlexDriverUnavailable",
		"PowerFlexDriverCrashLooping",
		"PowerFlexDriverHighCPU",
		"PowerFlexDriverHighMemory",
		"PowerFlexPoolCapacityWarning",
		"PowerFlexPoolCapacityCritical",
		"PowerFlexThinProvisioningHighRatio",
		"PowerFlexDataReductionDegraded",
		"PowerFlexArrayConnectivityLost",
		"PowerFlexRCGStateNotConsistent",
		"PowerFlexRCGInactive",
		"PowerFlexRCGLagExceeded",
		"PowerFlexRCGBandwidthDegraded",
		"PowerFlexRCGLatencyHigh",
		"PowerFlexRCGFrozen",
		"PowerFlexRCGPaused",
		"PowerFlexRCGFailoverActive",
		"PowerFlexRCGRPOViolation",
	}
	for _, alertName := range expectedAlerts {
		_, found := alertRuleByName(t, rules, alertName)
		assert.True(t, found, "expected alert %s", alertName)
	}

	// Verify volume operation failure rules use default threshold (3) and are namespace-scoped
	createVolume := mustAlertRule(t, rules, "PowerFlexCreateVolumeFailure")
	assert.Equal(t, `increase(dell_csi_operation_failure_total{namespace=~".+",operation="CreateVolume"}[5m]) > 3`, createVolume["expr"])
	assert.Equal(t, "0m", createVolume["for"])
	assert.Equal(t, "warning", nestedStringField(t, createVolume, "labels", "severity"))
	assert.Equal(t, "powerflex", nestedStringField(t, createVolume, "labels", "platform"))
	assert.Equal(t, "csi-driver", nestedStringField(t, createVolume, "labels", "component"))

	unavailable := mustAlertRule(t, rules, "PowerFlexDriverUnavailable")
	assert.Equal(t, `(count(dell_csi_driver_uptime_seconds{driver="csi-vxflexos", pod=~".+-controller-.+"}) or on() vector(0)) == 0 or (count(dell_csi_driver_uptime_seconds{driver="csi-vxflexos", pod=~".+-node-.+"}) or on() vector(0)) == 0`, unavailable["expr"])
	assert.Equal(t, "5m", unavailable["for"])
	assert.Equal(t, "critical", nestedStringField(t, unavailable, "labels", "severity"))

	connectivity := mustAlertRule(t, rules, "PowerFlexArrayConnectivityLost")
	assert.Equal(t, "dell_powerflex_array_healthy == 0", connectivity["expr"])
	assert.Equal(t, "critical", nestedStringField(t, connectivity, "labels", "severity"))

	rcgState := mustAlertRule(t, rules, "PowerFlexRCGStateNotConsistent")
	assert.Equal(t, `dell_powerflex_rcg_state{state!="Consistent"} == 1`, rcgState["expr"])
	assert.Equal(t, "critical", nestedStringField(t, rcgState, "labels", "severity"))

	// Verify default threshold substitution
	crashLoop := mustAlertRule(t, rules, "PowerFlexDriverCrashLooping")
	assert.Equal(t, `changes(dell_csi_driver_restart_total{driver="csi-vxflexos"}[15m]) > 3`, crashLoop["expr"])

	cpu := mustAlertRule(t, rules, "PowerFlexDriverHighCPU")
	assert.Equal(t, `dell_csi_driver_cpu_usage_percent{driver="csi-vxflexos"} > 80`, cpu["expr"])

	memory := mustAlertRule(t, rules, "PowerFlexDriverHighMemory")
	assert.Equal(t, `dell_csi_driver_memory_usage_bytes{driver="csi-vxflexos"} > 2147483648`, memory["expr"])

	poolWarn := mustAlertRule(t, rules, "PowerFlexPoolCapacityWarning")
	assert.Equal(t, "dell_powerflex_storage_pool_utilization_ratio * 100 > 80", poolWarn["expr"])

	poolCrit := mustAlertRule(t, rules, "PowerFlexPoolCapacityCritical")
	assert.Equal(t, "dell_powerflex_storage_pool_utilization_ratio * 100 > 90", poolCrit["expr"])

	thinRatio := mustAlertRule(t, rules, "PowerFlexThinProvisioningHighRatio")
	assert.Equal(t, "dell_powerflex_storage_pool_thin_ratio > 0.8", thinRatio["expr"])

	dataReduction := mustAlertRule(t, rules, "PowerFlexDataReductionDegraded")
	assert.Equal(t, "dell_powerflex_storage_pool_data_reduction_ratio < 1.5", dataReduction["expr"])

	rcgLag := mustAlertRule(t, rules, "PowerFlexRCGLagExceeded")
	assert.Equal(t, "dell_powerflex_rcg_lag_persistent_seconds > 300", rcgLag["expr"])

	rcgBw := mustAlertRule(t, rules, "PowerFlexRCGBandwidthDegraded")
	assert.Contains(t, rcgBw["expr"], "dell_powerflex_rcg_transmit_bandwidth_kbps < 10240")
	assert.Contains(t, rcgBw["expr"].(string), `dell_powerflex_rcg_activity_state{direction="local"} == 1`)
	assert.Contains(t, rcgBw["expr"].(string), "dell_powerflex_rcg_freeze_state == 0")
	assert.Contains(t, rcgBw["expr"].(string), "dell_powerflex_rcg_pause_mode == 0")

	rcgLatency := mustAlertRule(t, rules, "PowerFlexRCGLatencyHigh")
	assert.Equal(t, "dell_powerflex_rcg_transmit_latency_seconds > 5.0", rcgLatency["expr"])

	rcgRPO := mustAlertRule(t, rules, "PowerFlexRCGRPOViolation")
	assert.Contains(t, rcgRPO["expr"], "dell_powerflex_rcg_lag_persistent_seconds")
	assert.Contains(t, rcgRPO["expr"].(string), "dell_powerflex_rcg_rpo_seconds")
	assert.Contains(t, rcgRPO["expr"].(string), "dell_powerflex_rcg_rpo_seconds > 0")
	assert.Equal(t, "critical", nestedStringField(t, rcgRPO, "labels", "severity"))
}

// TestBuildPowerFlexAlertGroups_UsesCustomThresholds verifies that threshold placeholders in
// the YAML are correctly replaced with values from the MetricsPrometheusRuleConfig.
func TestBuildPowerFlexAlertGroups_UsesCustomThresholds(t *testing.T) {
	restartCount := int32(5)
	cpuThreshold := int32(90)
	memThreshold := int64(4294967296)
	poolWarnPercent := int32(75)
	poolCritPercent := int32(95)
	thinRatioThreshold := "1.2"
	dataReductionThreshold := "2.0"
	rcgLagSecs := int64(600)
	rcgBwKBps := int64(20480)
	rcgLatencySecs := "10.0"
	volOpFailThreshold := int32(3)

	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerFlex, testPowerFlexVersion, &csmv1.MetricsPrometheusRuleConfig{
		RestartCountThreshold:           &restartCount,
		CPUWarningThreshold:             &cpuThreshold,
		MemoryWarningThreshold:          &memThreshold,
		PoolCapacityWarningPercent:      &poolWarnPercent,
		PoolCapacityCriticalPercent:     &poolCritPercent,
		ThinRatioWarningThreshold:       &thinRatioThreshold,
		DataReductionDegradedThreshold:  &dataReductionThreshold,
		RCGLagWarningSeconds:            &rcgLagSecs,
		RCGBandwidthWarningKBps:         &rcgBwKBps,
		RCGLatencyWarningSeconds:        &rcgLatencySecs,
		VolumeOperationFailureThreshold: &volOpFailThreshold,
	})
	require.NoError(t, err)
	require.Len(t, groups, 4)

	rules := flattenGroupRules(t, groups)
	require.Len(t, rules, 26)

	// Verify VolumeOperationFailureThreshold is applied to all 8 volume operation alerts.
	for _, alertName := range []string{
		"PowerFlexCreateVolumeFailure",
		"PowerFlexDeleteVolumeFailure",
		"PowerFlexControllerPublishFailure",
		"PowerFlexControllerUnpublishFailure",
		"PowerFlexNodeStageFailure",
		"PowerFlexNodeUnstageFailure",
		"PowerFlexNodePublishFailure",
		"PowerFlexNodeUnpublishFailure",
	} {
		rule := mustAlertRule(t, rules, alertName)
		assert.Contains(t, rule["expr"], "> 3", "expected custom threshold in %s expr", alertName)
	}

	assert.Equal(t, `changes(dell_csi_driver_restart_total{driver="csi-vxflexos"}[15m]) > 5`,
		mustAlertRule(t, rules, "PowerFlexDriverCrashLooping")["expr"])
	assert.Equal(t, `dell_csi_driver_cpu_usage_percent{driver="csi-vxflexos"} > 90`,
		mustAlertRule(t, rules, "PowerFlexDriverHighCPU")["expr"])
	assert.Equal(t, `dell_csi_driver_memory_usage_bytes{driver="csi-vxflexos"} > 4294967296`,
		mustAlertRule(t, rules, "PowerFlexDriverHighMemory")["expr"])
	assert.Equal(t, "dell_powerflex_storage_pool_utilization_ratio * 100 > 75",
		mustAlertRule(t, rules, "PowerFlexPoolCapacityWarning")["expr"])
	assert.Equal(t, "dell_powerflex_storage_pool_utilization_ratio * 100 > 95",
		mustAlertRule(t, rules, "PowerFlexPoolCapacityCritical")["expr"])
	assert.Equal(t, "dell_powerflex_storage_pool_thin_ratio > 1.2",
		mustAlertRule(t, rules, "PowerFlexThinProvisioningHighRatio")["expr"])
	assert.Equal(t, "dell_powerflex_storage_pool_data_reduction_ratio < 2.0",
		mustAlertRule(t, rules, "PowerFlexDataReductionDegraded")["expr"])
	assert.Equal(t, "dell_powerflex_rcg_lag_persistent_seconds > 600",
		mustAlertRule(t, rules, "PowerFlexRCGLagExceeded")["expr"])
	bwExpr := mustAlertRule(t, rules, "PowerFlexRCGBandwidthDegraded")["expr"]
	assert.Contains(t, bwExpr, "dell_powerflex_rcg_transmit_bandwidth_kbps < 20480")
	assert.Contains(t, bwExpr.(string), "dell_powerflex_rcg_freeze_state == 0")
	assert.Contains(t, bwExpr.(string), "dell_powerflex_rcg_pause_mode == 0")
	assert.Equal(t, "dell_powerflex_rcg_transmit_latency_seconds > 10.0",
		mustAlertRule(t, rules, "PowerFlexRCGLatencyHigh")["expr"])
}

// TestBuildPowerStoreAlertGroups_GroupStructure validates that the PowerStore prometheusrule.yaml
// produces the expected group structure with correct names and total rule count.
func TestBuildPowerStoreAlertGroups_GroupStructure(t *testing.T) {
	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerStore, testPowerStoreVersion, nil)
	require.NoError(t, err)
	require.Len(t, groups, len(expectedPowerStoreGroupNames), "expected %d groups", len(expectedPowerStoreGroupNames))

	for i, expectedName := range expectedPowerStoreGroupNames {
		groupMap, ok := groups[i].(map[string]interface{})
		require.True(t, ok, "group %d should be a map", i)
		assert.Equal(t, expectedName, groupMap["name"], "group %d name mismatch", i)
		assert.Equal(t, "30s", groupMap["interval"], "group %d should have interval 30s", i)
	}

	rules := flattenGroupRules(t, groups)
	require.Len(t, rules, 18, "total rule count across all groups")
}

// TestBuildPowerStoreAlertGroups_DefaultRules verifies that the YAML file is loaded correctly
// with defaults, producing the expected 18 rules with correct structure and PromQL expressions.
func TestBuildPowerStoreAlertGroups_DefaultRules(t *testing.T) {
	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerStore, testPowerStoreVersion, nil)
	require.NoError(t, err)
	rules := flattenGroupRules(t, groups)
	require.Len(t, rules, 18)

	expectedAlerts := []string{
		"PowerStoreCreateVolumeFailure",
		"PowerStoreDeleteVolumeFailure",
		"PowerStoreControllerPublishVolumeFailure",
		"PowerStoreControllerUnpublishVolumeFailure",
		"PowerStoreNodeStageVolumeFailure",
		"PowerStoreNodeUnstageVolumeFailure",
		"PowerStoreNodePublishVolumeFailure",
		"PowerStoreNodeUnpublishVolumeFailure",
		"PowerStoreDriverPodUnavailable",
		"PowerStoreDriverCrashLooping",
		"PowerStoreDriverHighCPUUsage",
		"PowerStoreDriverHighMemoryUsage",
		"PowerStoreHighAPIErrorRate",
		"PowerStoreMetricsStale",
		"PowerStoreAuthenticationFailure",
		"PowerStoreApplianceCapacityWarning",
		"PowerStoreApplianceCapacityCritical",
		"PowerStoreThinProvisioningOvercommitment",
	}
	for _, alertName := range expectedAlerts {
		_, found := alertRuleByName(t, rules, alertName)
		assert.True(t, found, "expected alert %s", alertName)
	}

	createVolume := mustAlertRule(t, rules, "PowerStoreCreateVolumeFailure")
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="CreateVolume"}[5m]) > 3`, createVolume["expr"])
	assert.Equal(t, "0m", createVolume["for"])
	assert.Equal(t, "warning", nestedStringField(t, createVolume, "labels", "severity"))
	assert.Equal(t, "powerstore", nestedStringField(t, createVolume, "labels", "platform"))
	assert.Equal(t, "csi-driver", nestedStringField(t, createVolume, "labels", "component"))
	assert.Contains(t, nestedStringField(t, createVolume, "annotations", "description"), "{{ $labels.error_code }}")

	crashLoop := mustAlertRule(t, rules, "PowerStoreDriverCrashLooping")
	assert.Equal(t, "changes(dell_csi_driver_restart_total[15m]) > 3", crashLoop["expr"])
	assert.Equal(t, "0m", crashLoop["for"])

	cpu := mustAlertRule(t, rules, "PowerStoreDriverHighCPUUsage")
	assert.Equal(t, "dell_csi_driver_cpu_usage_percent > 80", cpu["expr"])
	assert.Equal(t, "5m", cpu["for"])

	applianceWarning := mustAlertRule(t, rules, "PowerStoreApplianceCapacityWarning")
	assert.Equal(t, "dell_powerstore_appliance_utilization_ratio * 100 > 80", applianceWarning["expr"])
	assert.Contains(t, nestedStringField(t, applianceWarning, "annotations", "summary"), "{{ $labels.appliance_name }}")

	applianceCritical := mustAlertRule(t, rules, "PowerStoreApplianceCapacityCritical")
	assert.Equal(t, "dell_powerstore_appliance_utilization_ratio * 100 > 90", applianceCritical["expr"])

	apiErrorRate := mustAlertRule(t, rules, "PowerStoreHighAPIErrorRate")
	apiExpr, ok := apiErrorRate["expr"].(string)
	require.True(t, ok, "expr should be a string")
	assert.Contains(t, apiExpr, `sum by (global_id, endpoint, method) (rate(dell_csi_api_call_total{status="failure"}[5m]))`)
	assert.Contains(t, apiExpr, `sum by (global_id, endpoint, method) (rate(dell_csi_api_call_total[5m]))`)
	assert.Contains(t, apiExpr, "* 100 > 10")
	assert.Contains(t, apiExpr, "and")
	assert.Contains(t, apiExpr, "> 0")

	metricsStale := mustAlertRule(t, rules, "PowerStoreMetricsStale")
	assert.Equal(t, "dell_powerstore_metrics_stale == 1", metricsStale["expr"])
	assert.Equal(t, "warning", nestedStringField(t, metricsStale, "labels", "severity"))

	authFailure := mustAlertRule(t, rules, "PowerStoreAuthenticationFailure")
	assert.Equal(t, `increase(dell_csi_operation_failure_total{error_code="auth_failure"}[5m]) > 0`, authFailure["expr"])
	assert.Equal(t, "critical", nestedStringField(t, authFailure, "labels", "severity"))
}

// TestBuildPowerStoreAlertGroups_UsesCustomThresholds verifies that threshold placeholders in
// the YAML are correctly replaced with values from the MetricsPrometheusRuleConfig.
func TestBuildPowerStoreAlertGroups_UsesCustomThresholds(t *testing.T) {
	applianceWarningThreshold := int32(75)
	applianceCriticalThreshold := int32(95)
	apiErrorRateThreshold := int32(15)
	apiErrorRateWindow := "10m"
	cpuWarningThreshold := int32(85)
	memoryWarningThreshold := int64(4096)
	driverCrashLoopRestartThreshold := int32(4)
	driverCrashLoopWindow := "20m"

	volumeOperationFailureThreshold := int32(3)

	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerStore, testPowerStoreVersion, &csmv1.MetricsPrometheusRuleConfig{
		ApplianceWarningThreshold:       &applianceWarningThreshold,
		ApplianceCriticalThreshold:      &applianceCriticalThreshold,
		APIErrorRateThreshold:           &apiErrorRateThreshold,
		APIErrorRateWindow:              &apiErrorRateWindow,
		CPUWarningThreshold:             &cpuWarningThreshold,
		MemoryWarningThreshold:          &memoryWarningThreshold,
		DriverCrashLoopRestartThreshold: &driverCrashLoopRestartThreshold,
		DriverCrashLoopWindow:           &driverCrashLoopWindow,
		VolumeOperationFailureThreshold: &volumeOperationFailureThreshold,
	})
	require.NoError(t, err)
	rules := flattenGroupRules(t, groups)
	require.Len(t, rules, 18)

	assert.Equal(t, "changes(dell_csi_driver_restart_total[20m]) > 4", mustAlertRule(t, rules, "PowerStoreDriverCrashLooping")["expr"])
	assert.Equal(t, "dell_csi_driver_cpu_usage_percent > 85", mustAlertRule(t, rules, "PowerStoreDriverHighCPUUsage")["expr"])
	assert.Equal(t, "dell_csi_driver_memory_usage_bytes > 4096", mustAlertRule(t, rules, "PowerStoreDriverHighMemoryUsage")["expr"])
	assert.Equal(t, "dell_powerstore_appliance_utilization_ratio * 100 > 75", mustAlertRule(t, rules, "PowerStoreApplianceCapacityWarning")["expr"])
	assert.Equal(t, "dell_powerstore_appliance_utilization_ratio * 100 > 95", mustAlertRule(t, rules, "PowerStoreApplianceCapacityCritical")["expr"])
	apiExpr, ok := mustAlertRule(t, rules, "PowerStoreHighAPIErrorRate")["expr"].(string)
	require.True(t, ok, "expr should be a string")
	assert.Contains(t, apiExpr, `sum by (global_id, endpoint, method) (rate(dell_csi_api_call_total{status="failure"}[10m]))`)
	assert.Contains(t, apiExpr, `sum by (global_id, endpoint, method) (rate(dell_csi_api_call_total[10m]))`)
	assert.Contains(t, apiExpr, "* 100 > 15")

	// Verify volume operation failure threshold is applied to all 8 volume op alerts
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="CreateVolume"}[5m]) > 3`, mustAlertRule(t, rules, "PowerStoreCreateVolumeFailure")["expr"])
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="DeleteVolume"}[5m]) > 3`, mustAlertRule(t, rules, "PowerStoreDeleteVolumeFailure")["expr"])
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="ControllerPublishVolume"}[5m]) > 3`, mustAlertRule(t, rules, "PowerStoreControllerPublishVolumeFailure")["expr"])
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="ControllerUnpublishVolume"}[5m]) > 3`, mustAlertRule(t, rules, "PowerStoreControllerUnpublishVolumeFailure")["expr"])
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="NodeStageVolume"}[5m]) > 3`, mustAlertRule(t, rules, "PowerStoreNodeStageVolumeFailure")["expr"])
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="NodeUnstageVolume"}[5m]) > 3`, mustAlertRule(t, rules, "PowerStoreNodeUnstageVolumeFailure")["expr"])
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="NodePublishVolume"}[5m]) > 3`, mustAlertRule(t, rules, "PowerStoreNodePublishVolumeFailure")["expr"])
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="NodeUnpublishVolume"}[5m]) > 3`, mustAlertRule(t, rules, "PowerStoreNodeUnpublishVolumeFailure")["expr"])
}

// TestInjectNamespaceLabelIntoGroups verifies that namespace is injected into all rules
// and that broad namespace regex patterns in PromQL expressions are narrowed.
func TestInjectNamespaceLabelIntoGroups(t *testing.T) {
	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerStore, testPowerStoreVersion, nil)
	require.NoError(t, err)

	injectNamespaceLabelIntoGroups(groups, "prod-storage")

	rules := flattenGroupRules(t, groups)
	require.Len(t, rules, 18)
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]interface{})
		require.True(t, ok)
		alertName := ruleMap["alert"]
		assert.Equal(t, "prod-storage", nestedStringField(t, ruleMap, "labels", "namespace"),
			"namespace label should be injected for alert %s", alertName)
	}

	// Verify PST-09 DriverPodUnavailable expr has namespace narrowed from regex to specific value
	podUnavailable := mustAlertRule(t, rules, "PowerStoreDriverPodUnavailable")
	expr, ok := podUnavailable["expr"].(string)
	require.True(t, ok)
	assert.Contains(t, expr, `namespace="prod-storage"`, "namespace regex should be replaced with specific namespace")
	assert.NotContains(t, expr, `namespace=~".+"`, "broad namespace regex should not remain")
}

// TestInjectNamespaceLabelIntoGroups_PowerScale verifies that namespace labels are injected into
// every PowerScale alert rule across all groups.
func TestInjectNamespaceLabelIntoGroups_PowerScale(t *testing.T) {
	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerScale, testPowerScaleVersion, nil)
	require.NoError(t, err)

	injectNamespaceLabelIntoGroups(groups, "test-namespace")

	rules := flattenGroupRules(t, groups)
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]interface{})
		require.True(t, ok)
		alertName, _ := ruleMap["alert"].(string)
		labels, ok := ruleMap["labels"].(map[string]interface{})
		require.True(t, ok, "rule %s should have labels", alertName)
		assert.Equal(t, "test-namespace", labels["namespace"], "rule %s should have namespace label", alertName)
	}
}

// TestInjectNamespaceLabelIntoGroups_PowerFlex verifies that namespace labels are injected into
// every PowerFlex alert rule and that PF-01..08 namespace=~".+" selectors are narrowed.
func TestInjectNamespaceLabelIntoGroups_PowerFlex(t *testing.T) {
	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerFlex, testPowerFlexVersion, nil)
	require.NoError(t, err)

	injectNamespaceLabelIntoGroups(groups, "vxflexos")

	rules := flattenGroupRules(t, groups)
	require.Len(t, rules, 26, "total rule count across all groups should be 26")
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]interface{})
		require.True(t, ok)
		alertName := ruleMap["alert"]
		assert.Equal(t, "vxflexos", nestedStringField(t, ruleMap, "labels", "namespace"),
			"namespace label should be injected for alert %s", alertName)
	}

	// Verify PF-01 CreateVolumeFailure expr has namespace narrowed from regex to specific value
	createVolume := mustAlertRule(t, rules, "PowerFlexCreateVolumeFailure")
	expr, ok := createVolume["expr"].(string)
	require.True(t, ok)
	assert.Contains(t, expr, `namespace="vxflexos"`, "PF-01 namespace selector should be narrowed")
	assert.NotContains(t, expr, `namespace=~".+"`, "broad namespace regex should not remain after injection")

	// Verify PF-25 FailoverActive has severity: info (not warning)
	failoverActive := mustAlertRule(t, rules, "PowerFlexRCGFailoverActive")
	assert.Equal(t, "info", nestedStringField(t, failoverActive, "labels", "severity"),
		"PF-25 PowerFlexRCGFailoverActive severity must be info per ER requirement")
}

// TestInjectNamespaceLabelIntoGroups_EmptyNamespace verifies graceful handling of empty namespace.
func TestInjectNamespaceLabelIntoGroups_EmptyNamespace(t *testing.T) {
	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerStore, testPowerStoreVersion, nil)
	require.NoError(t, err)

	injectNamespaceLabelIntoGroups(groups, "")

	rules := flattenGroupRules(t, groups)
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "", nestedStringField(t, ruleMap, "labels", "namespace"),
			"namespace label should be empty string for alert %s", ruleMap["alert"])
	}
}

func TestInjectNamespaceLabelIntoGroups_IgnoresMalformedEntries(t *testing.T) {
	groups := []interface{}{
		"not-a-map",
		map[string]interface{}{"rules": "not-a-slice"},
		map[string]interface{}{"rules": []interface{}{
			"not-a-rule-map",
			map[string]interface{}{"labels": "not-a-map"},
			map[string]interface{}{"labels": map[string]interface{}{"existing": "value"}},
		}},
		map[string]interface{}{"rules": []interface{}{
			map[string]interface{}{"labels": map[string]interface{}{"existing": "value"}},
		}},
	}

	injectNamespaceLabelIntoGroups(groups, "test-namespace")

	validGroup := groups[3].(map[string]interface{})
	rules := validGroup["rules"].([]interface{})
	rule := rules[0].(map[string]interface{})
	labels := rule["labels"].(map[string]interface{})
	assert.Equal(t, "test-namespace", labels["namespace"])
	assert.Equal(t, "value", labels["existing"])
}

// TestSyncMetricsResources_PowerFlex_CreatesPrometheusRule verifies that syncMetricsResources
// creates a PrometheusRule with 4 groups (25 PowerFlex alert rules) when PrometheusRule is enabled.
func TestSyncMetricsResources_PowerFlex_CreatesPrometheusRule(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("vxflexos-metrics-test", "test", shared.PFlexConfigVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "vxflexos-metrics-test", Namespace: "test", UID: types.UID("pflex-promrule-create")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerFlex
	cr.Spec.Driver.ConfigVersion = shared.PFlexConfigVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled: true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{
			Enabled: true,
		},
	}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))

	promRule := fetchPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace)
	groups := prometheusRuleRules(t, promRule)
	require.Len(t, groups, 4, "PrometheusRule should have 4 named groups")

	rules := flatRulesFromPrometheusRule(t, promRule)
	require.Len(t, rules, 26)

	poolWarn := mustAlertRule(t, rules, "PowerFlexPoolCapacityWarning")
	assert.Equal(t, "dell_powerflex_storage_pool_utilization_ratio * 100 > 80", poolWarn["expr"])
	assert.Equal(t, "warning", nestedStringField(t, poolWarn, "labels", "severity"))

	unavailable := mustAlertRule(t, rules, "PowerFlexDriverUnavailable")
	assert.Equal(t, "critical", nestedStringField(t, unavailable, "labels", "severity"))

	// Namespace label is injected into all rules
	for _, rule := range rules {
		ruleMap := rule.(map[string]interface{})
		alertName, _ := ruleMap["alert"].(string)
		assert.Equal(t, cr.Namespace, nestedStringField(t, ruleMap, "labels", "namespace"), "namespace label missing on %s", alertName)
	}
}

// TestSyncMetricsResources_PowerFlex_UsesCustomThresholds verifies that custom threshold
// values are correctly propagated into the created PrometheusRule.
func TestSyncMetricsResources_PowerFlex_UsesCustomThresholds(t *testing.T) {
	testCtx := context.Background()
	poolWarnPercent := int32(85)
	rcgLagSecs := int64(120)

	cr := shared.MakeCSM("vxflexos-metrics-custom", "test", shared.PFlexConfigVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "vxflexos-metrics-custom", Namespace: "test", UID: types.UID("pflex-promrule-custom")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerFlex
	cr.Spec.Driver.ConfigVersion = shared.PFlexConfigVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled: true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{
			Enabled:                    true,
			PoolCapacityWarningPercent: &poolWarnPercent,
			RCGLagWarningSeconds:       &rcgLagSecs,
		},
	}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))

	promRule := fetchPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace)
	rules := flatRulesFromPrometheusRule(t, promRule)
	assert.Equal(t, "dell_powerflex_storage_pool_utilization_ratio * 100 > 85",
		mustAlertRule(t, rules, "PowerFlexPoolCapacityWarning")["expr"])
	assert.Equal(t, "dell_powerflex_rcg_lag_persistent_seconds > 120",
		mustAlertRule(t, rules, "PowerFlexRCGLagExceeded")["expr"])
}

// TestSyncMetricsResources_PowerFlex_DeletesPrometheusRuleWhenDisabled verifies that
// the PrometheusRule is deleted when PrometheusRule.Enabled is set to false.
func TestSyncMetricsResources_PowerFlex_DeletesPrometheusRuleWhenDisabled(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("vxflexos-metrics-del", "test", shared.PFlexConfigVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "vxflexos-metrics-del", Namespace: "test", UID: types.UID("pflex-promrule-delete")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerFlex
	cr.Spec.Driver.ConfigVersion = shared.PFlexConfigVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled: true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{
			Enabled: true,
		},
	}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))
	_ = fetchPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace)

	cr.Spec.Driver.Metrics.PrometheusRule.Enabled = false
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))

	promRule := &unstructured.Unstructured{}
	promRule.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(testCtx, types.NamespacedName{Name: metricsPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, promRule)
	require.True(t, k8sErrors.IsNotFound(err), "PrometheusRule should be deleted when disabled")
}

func TestSyncMetricsResources_CreatesPrometheusRule(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pscale-metrics-test", "test", shared.PScaleConfigVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "pscale-metrics-test", Namespace: "test", UID: types.UID("pscale-promrule-create")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerScale
	cr.Spec.Driver.ConfigVersion = shared.PScaleConfigVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled: true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{
			Enabled: true,
		},
	}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))

	promRule := fetchPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace)
	groups := prometheusRuleRules(t, promRule)
	require.Len(t, groups, 6, "PrometheusRule should have 6 named groups")

	rules := flatRulesFromPrometheusRule(t, promRule)
	require.Len(t, rules, 22)

	// Volume operation alerts use default threshold (3) with for:1m
	createVolume := mustAlertRule(t, rules, "PowerScaleCreateVolumeFailure")
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="CreateVolume"}[5m]) > 3`, createVolume["expr"])
	assert.Equal(t, "1m", createVolume["for"])

	quotaWarning := mustAlertRule(t, rules, "PowerScaleQuotaWarning")
	assert.Equal(t, "dell_powerscale_quota_utilization_ratio * 100 > 80", quotaWarning["expr"])
	assert.Equal(t, "Quota warning for volume {{ $labels.volume_id }} in access zone {{ $labels.access_zone }}", nestedStringField(t, quotaWarning, "annotations", "summary"))
	assert.Equal(t, "Quota critical for volume {{ $labels.volume_id }} in access zone {{ $labels.access_zone }}", nestedStringField(t, mustAlertRule(t, rules, "PowerScaleQuotaCritical"), "annotations", "summary"))
	// Verify namespace label was injected
	assert.Equal(t, "test", nestedStringField(t, quotaWarning, "labels", "namespace"))

	// Verify namespace label is injected into all rules
	for _, rule := range rules {
		ruleMap := rule.(map[string]interface{})
		alertName, _ := ruleMap["alert"].(string)
		assert.Equal(t, cr.Namespace, nestedStringField(t, ruleMap, "labels", "namespace"), "namespace label missing on %s", alertName)
	}
}

// TestSyncMetricsResources_PowerScaleGroupStructureIsPreserved verifies that the PrometheusRule
// created by the operator preserves the multi-group structure from the PowerScale YAML file.
func TestSyncMetricsResources_PowerScaleGroupStructureIsPreserved(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pscale-groups-test", "test", shared.PScaleConfigVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "pscale-groups-test", Namespace: "test", UID: types.UID("pscale-groups")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerScale
	cr.Spec.Driver.ConfigVersion = shared.PScaleConfigVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled:        true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{Enabled: true},
	}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))

	promRule := fetchPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace)
	groups := prometheusRuleRules(t, promRule)
	require.Len(t, groups, 6)

	for i, expectedName := range expectedPowerScaleGroupNames {
		groupMap, ok := groups[i].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, expectedName, groupMap["name"], "group %d name mismatch", i)
		assert.Equal(t, "30s", groupMap["interval"], "group %d interval mismatch", i)
	}
}

// TestSyncMetricsResources_ResolvesConfigVersionFromSpecVersion verifies that the PrometheusRule
// path works when spec.version is set and spec.driver.configVersion is intentionally left empty.
func TestSyncMetricsResources_ResolvesConfigVersionFromSpecVersion(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pscale-metrics-spec-version-test", "test", "")
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "pscale-metrics-spec-version-test", Namespace: "test", UID: types.UID("pscale-promrule-spec-version")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerScale
	cr.Spec.Driver.ConfigVersion = ""
	cr.Spec.Version = shared.CSMVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled: true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{
			Enabled: true,
		},
	}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))

	promRule := fetchPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace)
	rules := flatRulesFromPrometheusRule(t, promRule)
	require.Len(t, rules, 22)
	assert.Equal(t, "dell_powerscale_quota_utilization_ratio * 100 > 80", mustAlertRule(t, rules, "PowerScaleQuotaWarning")["expr"])
}

func TestSyncMetricsResources_PrometheusRuleUsesCustomThresholds(t *testing.T) {
	testCtx := context.Background()
	quotaWarningThreshold := int32(75)

	cr := shared.MakeCSM("pscale-metrics-test", "test", shared.PScaleConfigVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "pscale-metrics-test", Namespace: "test", UID: types.UID("pscale-promrule-custom")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerScale
	cr.Spec.Driver.ConfigVersion = shared.PScaleConfigVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled: true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{
			Enabled:                  true,
			QuotaWarningThreshold:    &quotaWarningThreshold,
			NodepoolWarningThreshold: &quotaWarningThreshold,
		},
	}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))

	promRule := fetchPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace)
	rules := flatRulesFromPrometheusRule(t, promRule)
	assert.Equal(t, "dell_powerscale_quota_utilization_ratio * 100 > 75", mustAlertRule(t, rules, "PowerScaleQuotaWarning")["expr"])
	assert.Equal(t, "dell_powerscale_nodepool_utilization_percent > 75", mustAlertRule(t, rules, "PowerScaleNodePoolWarning")["expr"])
}

func TestSyncMetricsResources_DeletesPrometheusRuleWhenDisabled(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pscale-metrics-test", "test", shared.PScaleConfigVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "pscale-metrics-test", Namespace: "test", UID: types.UID("pscale-promrule-delete")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerScale
	cr.Spec.Driver.ConfigVersion = shared.PScaleConfigVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled: true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{
			Enabled: true,
		},
	}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))
	_ = fetchPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace)

	cr.Spec.Driver.Metrics.PrometheusRule.Enabled = false
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))

	promRule := &unstructured.Unstructured{}
	promRule.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(testCtx, types.NamespacedName{Name: metricsPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, promRule)
	require.True(t, k8sErrors.IsNotFound(err), "PrometheusRule should be deleted when disabled")
}

// TestSyncMetricsResources_GroupStructureIsPreserved verifies that the operator preserves
// the multi-group YAML structure when creating the PrometheusRule resource.
func TestSyncMetricsResources_GroupStructureIsPreserved(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pstore-group-test", "test-ns", shared.PStoreConfigVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "pstore-group-test", Namespace: "test-ns", UID: types.UID("pstore-group-structure")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerStore
	cr.Spec.Driver.ConfigVersion = shared.PStoreConfigVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled: true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{
			Enabled: true,
		},
	}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))

	promRule := fetchPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace)
	groups := prometheusRuleRules(t, promRule)
	require.Len(t, groups, len(expectedPowerStoreGroupNames), "expected %d groups in PrometheusRule", len(expectedPowerStoreGroupNames))

	for i, expectedName := range expectedPowerStoreGroupNames {
		groupMap, ok := groups[i].(map[string]interface{})
		require.True(t, ok, "group %d should be a map", i)
		assert.Equal(t, expectedName, groupMap["name"], "group %d name mismatch", i)
	}

	// Verify namespace label was injected into all rules
	rules := flatRulesFromPrometheusRule(t, promRule)
	require.Len(t, rules, 18)
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "test-ns", nestedStringField(t, ruleMap, "labels", "namespace"),
			"namespace label should be injected for alert %s", ruleMap["alert"])
	}

	// Verify PST-09 DriverPodUnavailable expr has namespace narrowed
	podUnavailable := mustAlertRule(t, rules, "PowerStoreDriverPodUnavailable")
	expr, ok := podUnavailable["expr"].(string)
	require.True(t, ok)
	assert.Contains(t, expr, `namespace="test-ns"`)
	assert.NotContains(t, expr, `namespace=~".+"`)
}

// ──────────────────────────────────────────────────────────────────────────────
// PowerMax driver PrometheusRule tests
// ──────────────────────────────────────────────────────────────────────────────

// TestBuildDriverAlertGroups_PowerMaxMissingYAML verifies error for a nonexistent PowerMax version.
func TestBuildDriverAlertGroups_PowerMaxMissingYAML(t *testing.T) {
	_, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerMax, "v0.0.0-nonexistent", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prometheusrule.yaml")
}

// TestBuildPowerMaxAlertGroups_GroupStructure verifies the PowerMax prometheusrule.yaml is loaded
// with the correct 3-group structure and expected group names and intervals.
func TestBuildPowerMaxAlertGroups_GroupStructure(t *testing.T) {
	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerMax, testPowerMaxVersion, nil)
	require.NoError(t, err)
	require.Len(t, groups, 3, "PowerMax should have 3 named groups")

	for i, expectedName := range expectedPowerMaxGroupNames {
		groupMap, ok := groups[i].(map[string]interface{})
		require.True(t, ok, "group %d should be a map", i)
		assert.Equal(t, expectedName, groupMap["name"], "group %d name mismatch", i)
		assert.Equal(t, "30s", groupMap["interval"], "group %d should have interval:30s", i)
	}

	// Verify total rule count across all groups is 15.
	allRules := flattenGroupRules(t, groups)
	assert.Len(t, allRules, 15, "total rule count across all groups should be 15")
}

// TestBuildPowerMaxAlertGroups_DefaultRules verifies the PowerMax prometheusrule.yaml is loaded
// correctly with default thresholds, producing the expected 15 alert rules with correct structure.
func TestBuildPowerMaxAlertGroups_DefaultRules(t *testing.T) {
	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerMax, testPowerMaxVersion, nil)
	require.NoError(t, err)
	require.Len(t, groups, 3)

	rules := flattenGroupRules(t, groups)
	require.Len(t, rules, 15)

	expectedAlerts := []string{
		"PowerMaxCreateVolumeFailure",
		"PowerMaxDeleteVolumeFailure",
		"PowerMaxControllerPublishFailure",
		"PowerMaxControllerUnpublishFailure",
		"PowerMaxNodeStageFailure",
		"PowerMaxNodeUnstageFailure",
		"PowerMaxNodePublishFailure",
		"PowerMaxNodeUnpublishFailure",
		"PowerMaxDriverPodUnavailable",
		"PowerMaxDriverHighCPU",
		"PowerMaxDriverHighMemory",
		"PowerMaxAPIErrorRateHigh",
		"PowerMaxStorageGroupCapacityWarning",
		"PowerMaxStorageGroupCapacityCritical",
		"PowerMaxSRPSnapshotCapacityWarning",
	}
	for _, alertName := range expectedAlerts {
		_, found := alertRuleByName(t, rules, alertName)
		assert.True(t, found, "expected alert %s to be present", alertName)
	}

	// Verify PM-01 structure
	createVolume := mustAlertRule(t, rules, "PowerMaxCreateVolumeFailure")
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="CreateVolume"}[5m]) > 3`, createVolume["expr"])
	assert.Equal(t, "1m", createVolume["for"])
	assert.Equal(t, "warning", nestedStringField(t, createVolume, "labels", "severity"))
	assert.Equal(t, "powermax", nestedStringField(t, createVolume, "labels", "platform"))
	assert.Equal(t, "csi-driver", nestedStringField(t, createVolume, "labels", "component"))
	assert.Equal(t, "PM-01", nestedStringField(t, createVolume, "labels", "alert_id"))

	// Verify PM-05 uses NodeStageVolume (not CreateSnapshot — bug fix verification)
	nodeStage := mustAlertRule(t, rules, "PowerMaxNodeStageFailure")
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="NodeStageVolume"}[5m]) > 3`, nodeStage["expr"])
	assert.Equal(t, "PM-05", nestedStringField(t, nodeStage, "labels", "alert_id"))

	// Verify PM-06 uses NodeUnstageVolume (not DeleteSnapshot — bug fix verification)
	nodeUnstage := mustAlertRule(t, rules, "PowerMaxNodeUnstageFailure")
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="NodeUnstageVolume"}[5m]) > 3`, nodeUnstage["expr"])
	assert.Equal(t, "PM-06", nestedStringField(t, nodeUnstage, "labels", "alert_id"))

	// Verify PM-09: absent() carries no metric labels; namespace is injected by
	// injectNamespaceLabelIntoGroups (called by the controller) — not set in the YAML.
	podUnavailable := mustAlertRule(t, rules, "PowerMaxDriverPodUnavailable")
	assert.Equal(t, `absent(dell_csi_driver_uptime_seconds{instance_type="controller"})`, podUnavailable["expr"])
	assert.Equal(t, "5m", podUnavailable["for"])
	assert.Equal(t, "critical", nestedStringField(t, podUnavailable, "labels", "severity"))
	assert.Equal(t, "PM-09", nestedStringField(t, podUnavailable, "labels", "alert_id"))

	// Verify PM-10 CPU threshold default
	cpu := mustAlertRule(t, rules, "PowerMaxDriverHighCPU")
	assert.Equal(t, "dell_csi_driver_cpu_usage_percent > 80", cpu["expr"])
	assert.Equal(t, "5m", cpu["for"])

	// Verify PM-11 memory threshold default (2 GiB)
	memory := mustAlertRule(t, rules, "PowerMaxDriverHighMemory")
	assert.Equal(t, "dell_csi_driver_memory_usage_bytes > 2147483648", memory["expr"])

	// Verify PM-13 storage group capacity warning default
	sgWarning := mustAlertRule(t, rules, "PowerMaxStorageGroupCapacityWarning")
	assert.Equal(t, "dell_csi_storagegroup_utilization_ratio * 100 > 80", sgWarning["expr"])
	assert.Equal(t, "10m", sgWarning["for"])
	assert.Equal(t, "warning", nestedStringField(t, sgWarning, "labels", "severity"))

	// Verify PM-14 storage group capacity critical default
	sgCritical := mustAlertRule(t, rules, "PowerMaxStorageGroupCapacityCritical")
	assert.Equal(t, "dell_csi_storagegroup_utilization_ratio * 100 > 90", sgCritical["expr"])
	assert.Equal(t, "1m", sgCritical["for"])
	assert.Equal(t, "critical", nestedStringField(t, sgCritical, "labels", "severity"))

	// Verify PM-15: sum by(array_id, srp_id) drops namespace; namespace is injected by
	// injectNamespaceLabelIntoGroups (called by the controller) — not set in the YAML.
	srpSnapshot := mustAlertRule(t, rules, "PowerMaxSRPSnapshotCapacityWarning")
	assert.Contains(t, srpSnapshot["expr"], "> 80")
	assert.Equal(t, "10m", srpSnapshot["for"])
	assert.Equal(t, "PM-15", nestedStringField(t, srpSnapshot, "labels", "alert_id"))
}

// TestBuildPowerMaxAlertGroups_UsesCustomThresholds verifies that threshold placeholders in
// the PowerMax YAML are correctly replaced with values from the MetricsPrometheusRuleConfig.
func TestBuildPowerMaxAlertGroups_UsesCustomThresholds(t *testing.T) {
	volumeOpThreshold := int32(5)
	cpuThreshold := int32(90)
	memoryThreshold := int64(4294967296) // 4 GiB
	apiErrorThreshold := int32(15)
	apiErrorWindow := "10m"
	sgWarningThreshold := int32(75)
	sgCriticalThreshold := int32(95)
	srpSnapshotThreshold := int32(70)

	groups, err := buildDriverAlertGroups(testConfigDir, csmv1.PowerMax, testPowerMaxVersion, &csmv1.MetricsPrometheusRuleConfig{
		VolumeOperationFailureThreshold: &volumeOpThreshold,
		CPUWarningThreshold:             &cpuThreshold,
		MemoryWarningThreshold:          &memoryThreshold,
		APIErrorRateThreshold:           &apiErrorThreshold,
		APIErrorRateWindow:              &apiErrorWindow,
		StorageGroupCapacityWarning:     &sgWarningThreshold,
		StorageGroupCapacityCritical:    &sgCriticalThreshold,
		SRPSnapshotCapacityWarning:      &srpSnapshotThreshold,
	})
	require.NoError(t, err)
	require.Len(t, groups, 3)

	rules := flattenGroupRules(t, groups)
	require.Len(t, rules, 15)

	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="CreateVolume"}[5m]) > 5`, mustAlertRule(t, rules, "PowerMaxCreateVolumeFailure")["expr"])
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="NodeStageVolume"}[5m]) > 5`, mustAlertRule(t, rules, "PowerMaxNodeStageFailure")["expr"])
	assert.Equal(t, "dell_csi_driver_cpu_usage_percent > 90", mustAlertRule(t, rules, "PowerMaxDriverHighCPU")["expr"])
	assert.Equal(t, "dell_csi_driver_memory_usage_bytes > 4294967296", mustAlertRule(t, rules, "PowerMaxDriverHighMemory")["expr"])
	assert.Equal(t, "dell_csi_storagegroup_utilization_ratio * 100 > 75", mustAlertRule(t, rules, "PowerMaxStorageGroupCapacityWarning")["expr"])
	assert.Equal(t, "dell_csi_storagegroup_utilization_ratio * 100 > 95", mustAlertRule(t, rules, "PowerMaxStorageGroupCapacityCritical")["expr"])
	assert.Contains(t, mustAlertRule(t, rules, "PowerMaxSRPSnapshotCapacityWarning")["expr"], "> 70")
	// API error rate check: window and threshold
	apiExpr := mustAlertRule(t, rules, "PowerMaxAPIErrorRateHigh")["expr"].(string)
	assert.Contains(t, apiExpr, "[10m]")
	assert.Contains(t, apiExpr, "> 15")
}

// TestSyncMetricsResources_PowerMax_CreatesPrometheusRule verifies end-to-end PrometheusRule
// creation for the PowerMax driver via syncMetricsResources.
func TestSyncMetricsResources_PowerMax_CreatesPrometheusRule(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pmax-metrics-test", "test-ns", testPowerMaxVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "pmax-metrics-test", Namespace: "test-ns", UID: types.UID("pmax-promrule-create")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerMax
	cr.Spec.Driver.ConfigVersion = testPowerMaxVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled: true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{
			Enabled: true,
		},
	}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))

	promRule := fetchPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace)

	// Verify group structure is preserved
	groups := prometheusRuleRules(t, promRule)
	require.Len(t, groups, 3, "PowerMax PrometheusRule should have 3 named groups")
	for i, expectedName := range expectedPowerMaxGroupNames {
		groupMap, ok := groups[i].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, expectedName, groupMap["name"], "group %d name mismatch", i)
	}

	rules := flatRulesFromPrometheusRule(t, promRule)
	require.Len(t, rules, 15)

	// Verify a few key alerts
	assert.Equal(t, `increase(dell_csi_operation_failure_total{operation="CreateVolume"}[5m]) > 3`, mustAlertRule(t, rules, "PowerMaxCreateVolumeFailure")["expr"])
	assert.Equal(t, "dell_csi_storagegroup_utilization_ratio * 100 > 80", mustAlertRule(t, rules, "PowerMaxStorageGroupCapacityWarning")["expr"])
	assert.Equal(t, `absent(dell_csi_driver_uptime_seconds{instance_type="controller"})`, mustAlertRule(t, rules, "PowerMaxDriverPodUnavailable")["expr"])

	// PM-09: absent() carries no metric labels; namespace must be injected by injectNamespaceLabelIntoGroups.
	// Verify the controller correctly injects the CR namespace into this alert.
	assert.Equal(t, "test-ns", nestedStringField(t, mustAlertRule(t, rules, "PowerMaxDriverPodUnavailable"), "labels", "namespace"))
	// PM-15: sum by(array_id, srp_id) drops namespace; namespace must be injected by injectNamespaceLabelIntoGroups.
	assert.Equal(t, "test-ns", nestedStringField(t, mustAlertRule(t, rules, "PowerMaxSRPSnapshotCapacityWarning"), "labels", "namespace"))

	// Verify namespace is injected into ALL PowerMax alerts (not just PM-09 and PM-15)
	for _, rule := range rules {
		ruleMap := rule.(map[string]interface{})
		alertName, _ := ruleMap["alert"].(string)
		assert.Equal(t, cr.Namespace, nestedStringField(t, ruleMap, "labels", "namespace"), "namespace label missing on %s", alertName)
	}
}

// TestSyncMetricsResources_PowerMax_DeletesPrometheusRuleWhenDisabled verifies that disabling
// prometheusRule.enabled removes the PrometheusRule for the PowerMax driver.
func TestSyncMetricsResources_PowerMax_DeletesPrometheusRuleWhenDisabled(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pmax-metrics-test", "test-ns", testPowerMaxVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "pmax-metrics-test", Namespace: "test-ns", UID: types.UID("pmax-promrule-delete")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerMax
	cr.Spec.Driver.ConfigVersion = testPowerMaxVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled: true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{
			Enabled: true,
		},
	}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))
	_ = fetchPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace)

	cr.Spec.Driver.Metrics.PrometheusRule.Enabled = false
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))

	promRule := &unstructured.Unstructured{}
	promRule.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(testCtx, types.NamespacedName{Name: metricsPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, promRule)
	require.True(t, k8sErrors.IsNotFound(err), "PrometheusRule should be deleted when disabled")
}

// mustAlertRule finds a rule by alert name, failing the test if not found.
func mustAlertRule(t *testing.T, rules []interface{}, alertName string) map[string]interface{} {
	t.Helper()
	rule, found := alertRuleByName(t, rules, alertName)
	require.True(t, found, "expected alert rule %s", alertName)
	return rule
}

// alertRuleByName searches the rules slice for a rule with the given alert name.
func alertRuleByName(t *testing.T, rules []interface{}, alertName string) (map[string]interface{}, bool) {
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

// nestedStringField extracts a string from parent[container][key], failing the test if any cast fails.
func nestedStringField(t *testing.T, parent map[string]interface{}, container, key string) string {
	t.Helper()
	containerMap, ok := parent[container].(map[string]interface{})
	require.True(t, ok, "%s should be a map", container)
	value, ok := containerMap[key].(string)
	require.True(t, ok, "%s.%s should be a string", container, key)
	return value
}

func fetchPrometheusRule(testCtx context.Context, t *testing.T, ctrlClient client.Client, crName, namespace string) *unstructured.Unstructured {
	t.Helper()
	promRule := &unstructured.Unstructured{}
	promRule.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	require.NoError(t, ctrlClient.Get(testCtx, types.NamespacedName{Name: metricsPrometheusRuleName(crName), Namespace: namespace}, promRule))
	return promRule
}

const (
	testAuthorizationVersion       = "v2.6.0"
	testAuthorizationLegacyVersion = "v2.5.0"
)

func makeAuthorizationBuildModule() csmv1.Module {
	return csmv1.Module{
		Name:          csmv1.AuthorizationServer,
		ConfigVersion: testAuthorizationVersion,
	}
}

func makeAuthorizationSyncModule(version string) csmv1.Module {
	return csmv1.Module{
		Name:          csmv1.AuthorizationServer,
		ConfigVersion: version,
		Metrics: &csmv1.ModuleMetrics{
			Enabled: true,
			PrometheusRule: &csmv1.ModulePrometheusRuleConfig{
				Enabled: true,
			},
		},
	}
}

func fetchAuthorizationPrometheusRule(ctx context.Context, t *testing.T, ctrlClient client.Client, crName, namespace string) *unstructured.Unstructured {
	t.Helper()
	pr := &unstructured.Unstructured{}
	pr.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	require.NoError(t, ctrlClient.Get(ctx, types.NamespacedName{Name: authorizationPrometheusRuleName(crName), Namespace: namespace}, pr))
	return pr
}

// TestBuildAuthorizationAlertRules_LoadsDefaults verifies that the authorization
// prometheusrule.yaml is loaded and default thresholds are substituted correctly.
func TestBuildAuthorizationAlertRules_LoadsDefaults(t *testing.T) {
	ctx := context.Background()
	rules, err := buildAuthorizationAlertRules(ctx, operatorutils.OperatorConfig{ConfigDirectory: testConfigDir}, makeAuthorizationBuildModule(), csmv1.ContainerStorageModule{})
	require.NoError(t, err)
	require.Len(t, rules, 8)

	expectedAlerts := []string{
		"AuthorizationServiceDown",
		"HighAuthorizationFailureRate",
		"TokenValidationFailure",
		"UnauthorizedAccessAttempts",
		"CredentialShieldingBreach",
		"QuotaAuthorizationFailure",
		"HighAuthorizationLatency",
		"AuthorizationServiceRecovery",
	}
	for _, alertName := range expectedAlerts {
		_, found := alertRuleByName(t, rules, alertName)
		assert.True(t, found, "expected alert %s", alertName)
	}

	serviceDown := mustAlertRule(t, rules, "AuthorizationServiceDown")
	assert.Equal(t, "dell_csm_auth_up == 0 or absent(dell_csm_auth_up)", serviceDown["expr"])
	assert.Equal(t, "5m", serviceDown["for"])
	assert.Equal(t, "critical", nestedStringField(t, serviceDown, "labels", "severity"))
	assert.Equal(t, "csm-authorization", nestedStringField(t, serviceDown, "labels", "platform"))
	assert.Equal(t, "authorization", nestedStringField(t, serviceDown, "labels", "component"))

	failureRate := mustAlertRule(t, rules, "HighAuthorizationFailureRate")
	assert.Equal(t, `sum by (storage_type, tenant) (rate(dell_csm_auth_request_total{status="failure"}[5m])) / sum by (storage_type, tenant) (rate(dell_csm_auth_request_total[5m])) > 0.10`, failureRate["expr"])

	latency := mustAlertRule(t, rules, "HighAuthorizationLatency")
	assert.Equal(t, `histogram_quantile(0.95, rate(dell_csm_auth_request_duration_seconds_bucket[5m])) > 2`, latency["expr"])
	assert.Contains(t, nestedStringField(t, latency, "annotations", "description"), "0.95 quantile")

	recovery := mustAlertRule(t, rules, "AuthorizationServiceRecovery")
	assert.Equal(t, "info", nestedStringField(t, recovery, "labels", "severity"))
	assert.Equal(t, "changes(dell_csm_auth_up[1m]) > 0 and dell_csm_auth_up == 1", recovery["expr"])
	assert.Equal(t, "0m", recovery["for"])
}

// TestBuildAuthorizationAlertRules_UsesCustomThresholds verifies that threshold placeholders
// are replaced with values from the MetricsPrometheusRuleConfig.
func TestBuildAuthorizationAlertRules_UsesCustomThresholds(t *testing.T) {
	ctx := context.Background()
	failureRate := int32(20) // 20% -> 0.20

	module := makeAuthorizationBuildModule()
	module.Metrics = &csmv1.ModuleMetrics{
		Enabled: true,
		PrometheusRule: &csmv1.ModulePrometheusRuleConfig{
			AuthFailureRateThreshold: &failureRate,
		},
	}

	rules, err := buildAuthorizationAlertRules(ctx, operatorutils.OperatorConfig{ConfigDirectory: testConfigDir}, module, csmv1.ContainerStorageModule{})
	require.NoError(t, err)
	require.Len(t, rules, 8)

	failureRateRule := mustAlertRule(t, rules, "HighAuthorizationFailureRate")
	assert.Equal(t, `sum by (storage_type, tenant) (rate(dell_csm_auth_request_total{status="failure"}[5m])) / sum by (storage_type, tenant) (rate(dell_csm_auth_request_total[5m])) > 0.20`, failureRateRule["expr"])

	latencyRule := mustAlertRule(t, rules, "HighAuthorizationLatency")
	assert.Equal(t, `histogram_quantile(0.95, rate(dell_csm_auth_request_duration_seconds_bucket[5m])) > 2`, latencyRule["expr"])
}

// TestSyncAuthorizationPrometheusRule_CreatesNamespaceLabels verifies that the controller injects
// the CR namespace into every rule label for namespace-scoped Alertmanager routing.
func TestSyncAuthorizationPrometheusRule_CreatesNamespaceLabels(t *testing.T) {
	ctx := context.Background()
	cr := shared.MakeCSM("auth-sync", "test-ns", testAuthorizationVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "auth-sync", Namespace: "test-ns", UID: types.UID("auth-sync-uid")}
	module := makeAuthorizationSyncModule(testAuthorizationVersion)
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncAuthorizationPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := fetchAuthorizationPrometheusRule(ctx, t, fakeClient, cr.Name, cr.Namespace)
	labels := pr.GetLabels()
	assert.Equal(t, "proxy-server", labels["app"])
	assert.NotContains(t, labels, "release")

	rules := flatRulesFromPrometheusRule(t, pr)
	require.Len(t, rules, 8)

	for _, rule := range rules {
		ruleMap := rule.(map[string]interface{})
		alertName, _ := ruleMap["alert"].(string)
		assert.Equal(t, cr.Namespace, nestedStringField(t, ruleMap, "labels", "namespace"), "namespace label missing on %s", alertName)
	}
}

// TestSyncAuthorizationPrometheusRule_UsesDriverSpecificLabelKey verifies that
// the authorization PrometheusRule follows the driver-specific label key used by
// other operator-managed resources.
func TestSyncAuthorizationPrometheusRule_UsesDriverSpecificLabelKey(t *testing.T) {
	ctx := context.Background()
	cr := shared.MakeCSM("auth-sync-pflex", "test-ns", shared.PFlexConfigVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "auth-sync-pflex", Namespace: "test-ns", UID: types.UID("auth-sync-pflex-uid")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerFlex
	module := makeAuthorizationSyncModule(testAuthorizationVersion)
	fakeClient := fake.NewClientBuilder().Build()

	require.NoError(t, syncAuthorizationPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := fetchAuthorizationPrometheusRule(ctx, t, fakeClient, cr.Name, cr.Namespace)
	labels := pr.GetLabels()
	assert.Equal(t, "proxy-server", labels["name"])
	assert.NotContains(t, labels, "release")
}

// TestSyncAuthorizationPrometheusRule_MissingYAMLDeletesRule verifies that older auth versions
// without prometheusrule.yaml do not break reconcile and remove any stale PrometheusRule.
func TestSyncAuthorizationPrometheusRule_MissingYAMLDeletesRule(t *testing.T) {
	ctx := context.Background()
	cr := shared.MakeCSM("auth-old", "test-ns", testAuthorizationVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "auth-old", Namespace: "test-ns", UID: types.UID("auth-old-uid")}
	module := makeAuthorizationSyncModule(testAuthorizationLegacyVersion)
	fakeClient := fake.NewClientBuilder().Build()

	stale := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "monitoring.coreos.com/v1",
		"kind":       "PrometheusRule",
		"metadata": map[string]interface{}{
			"name":      authorizationPrometheusRuleName(cr.Name),
			"namespace": cr.Namespace,
		},
		"spec": map[string]interface{}{
			"groups": []interface{}{},
		},
	}}
	require.NoError(t, fakeClient.Create(ctx, stale))

	require.NoError(t, syncAuthorizationPrometheusRule(ctx, false, module, cr, testOpConfig, fakeClient))

	pr := &unstructured.Unstructured{}
	pr.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(ctx, types.NamespacedName{Name: authorizationPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, pr)
	assert.True(t, k8sErrors.IsNotFound(err), "PrometheusRule should be deleted when the version does not ship one")
}

// TestBuildAuthorizationAlertRules_MissingYAML verifies that a meaningful error is returned
// when the prometheusrule.yaml file does not exist for the given version.
func TestBuildAuthorizationAlertRules_MissingYAML(t *testing.T) {
	ctx := context.Background()
	module := csmv1.Module{Name: csmv1.AuthorizationServer, ConfigVersion: "v0.0.0-nonexistent"}
	_, err := buildAuthorizationAlertRules(ctx, operatorutils.OperatorConfig{ConfigDirectory: testConfigDir}, module, csmv1.ContainerStorageModule{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prometheusrule.yaml")
}

func prometheusRuleRules(t *testing.T, promRule *unstructured.Unstructured) []interface{} {
	t.Helper()
	spec, found, err := unstructured.NestedMap(promRule.Object, "spec")
	require.NoError(t, err)
	require.True(t, found, "PrometheusRule spec should be present")
	groups, ok := spec["groups"].([]interface{})
	require.True(t, ok, "PrometheusRule groups should be a list")
	return groups
}

// flatRulesFromPrometheusRule returns all alert rules from all groups in a PrometheusRule.
func flatRulesFromPrometheusRule(t *testing.T, promRule *unstructured.Unstructured) []interface{} {
	t.Helper()
	groups := prometheusRuleRules(t, promRule)
	return flattenGroupRules(t, groups)
}

// flattenGroupRules collects all alert rules from all groups into a single flat slice.
func flattenGroupRules(t *testing.T, groups []interface{}) []interface{} {
	t.Helper()
	var allRules []interface{}
	for i, g := range groups {
		groupMap, ok := g.(map[string]interface{})
		require.True(t, ok, "group %d should be a map", i)
		rules, ok := groupMap["rules"].([]interface{})
		require.True(t, ok, "group %d rules should be a list", i)
		allRules = append(allRules, rules...)
	}
	return allRules
}

// ──────────────────────────────────────────────────────────────────────────────
// CSM Replication module PrometheusRule tests
// ──────────────────────────────────────────────────────────────────────────────

// testReplicationModuleVersion is the replication module version under test.
const testReplicationModuleVersion = "v1.16.0"

// TestReplicationPrometheusRuleName verifies the naming convention for the
// replication module PrometheusRule resource ({cr.Name}-replication-alerts).
func TestReplicationPrometheusRuleName(t *testing.T) {
	assert.Equal(t, "powermax-replication-alerts", replicationPrometheusRuleName("powermax"))
	assert.Equal(t, "my-driver-replication-alerts", replicationPrometheusRuleName("my-driver"))
}

// TestBuildReplicationModuleAlertGroups_DefaultRules verifies the replication
// prometheusrule.yaml is loaded with default thresholds, producing 1 group named
// "csm-replication-alerts" containing 14 rules (2 record rules + 12 alert rules).
func TestBuildReplicationModuleAlertGroups_DefaultRules(t *testing.T) {
	groups, err := buildReplicationModuleAlertGroups(testConfigDir, testReplicationModuleVersion, "test-namespace", nil)
	require.NoError(t, err)
	require.Len(t, groups, 1, "expected 1 group: csm-replication-alerts")
	groupMap, ok := groups[0].(map[string]interface{})
	require.True(t, ok, "group should be a map")
	assert.Equal(t, "csm-replication-alerts", groupMap["name"])
	assert.Equal(t, "30s", groupMap["interval"])
	rules := flattenGroupRules(t, groups)
	require.Len(t, rules, 14, "expected 2 record rules + 12 alert rules")

	// Verify all alert rules are present
	expectedAlerts := []string{
		"CSMReplicationLagExceeded",             // REP-01
		"CSMReplicationRPOViolation",            // REP-02
		"CSMReplicationLinkDown",                // REP-03
		"CSMReplicationFailure",                 // REP-04
		"CSMReplicationControllerDown",          // REP-05
		"CSMReplicationMetricsStale",            // REP-06
		"PowerMaxSRDFBandwidthDegraded",         // REP-07
		"CSMReplicationSRDFFailoverTriggered",   // REP-08
		"PowerMaxSRDFStateNotSynchronized",      // REP-09
		"PowerMaxSRDFStatePersistentlyDegraded", // REP-10
		"PowerMaxSRDFLinkDown",                  // REP-11
		"PowerMaxSRDFLagExceeded",               // REP-12
	}
	for _, alertName := range expectedAlerts {
		_, found := alertRuleByName(t, rules, alertName)
		assert.True(t, found, "expected alert %s to be present", alertName)
	}

	// Verify record rules are present and namespace-scoped
	for _, recordName := range []string{
		"dell_powermax_srdf_group_state_current",
		"dell_powermax_srdf_group_state_previous",
	} {
		rule, found := recordRuleByName(t, rules, recordName)
		assert.True(t, found, "expected record rule %s to be present", recordName)
		assert.Contains(t, rule["expr"], `dell_powermax_srdf_group_state{namespace="test-namespace"}`)
	}

	// REP-01: lag exceeded warning (default 300s, 2m for:)
	rep01 := mustAlertRule(t, rules, "CSMReplicationLagExceeded")
	assert.Contains(t, rep01["expr"], `dell_csm_repl_lag_seconds{namespace="test-namespace"} > 300`)
	assert.Equal(t, "2m", rep01["for"])
	assert.Equal(t, "warning", nestedStringField(t, rep01, "labels", "severity"))
	assert.Equal(t, "REP-01", nestedStringField(t, rep01, "labels", "alert_id"))

	// REP-02: RPO violation critical (same expr, 15m for:)
	rep02 := mustAlertRule(t, rules, "CSMReplicationRPOViolation")
	assert.Contains(t, rep02["expr"], `dell_csm_repl_lag_seconds{namespace="test-namespace"} > 300`)
	assert.Equal(t, "15m", rep02["for"])
	assert.Equal(t, "critical", nestedStringField(t, rep02, "labels", "severity"))
	assert.Equal(t, "REP-02", nestedStringField(t, rep02, "labels", "alert_id"))

	// REP-05: controller health check
	rep05 := mustAlertRule(t, rules, "CSMReplicationControllerDown")
	assert.Contains(t, rep05["expr"], `dell_csm_repl_controller_health{namespace="test-namespace"} == 0`)
	assert.Equal(t, "5m", rep05["for"])
	assert.Equal(t, "critical", nestedStringField(t, rep05, "labels", "severity"))

	// REP-06: stale flag, missing metrics, and incomplete collection detection
	rep06 := mustAlertRule(t, rules, "CSMReplicationMetricsStale")
	assert.Contains(t, rep06["expr"], `dell_csm_repl_metrics_stale{namespace="test-namespace"} == 1`)
	assert.Contains(t, rep06["expr"], `absent(dell_csm_repl_last_collection_timestamp_seconds{namespace="test-namespace"})`)
	assert.Contains(t, rep06["expr"], `time() - max by (namespace, driver) (dell_csm_repl_last_collection_timestamp_seconds{namespace="test-namespace"})`)
	assert.NotContains(t, rep06["expr"], "timestamp(")
	assert.Equal(t, "5m", rep06["for"])
	assert.Equal(t, "warning", nestedStringField(t, rep06, "labels", "severity"))

	// REP-07: SRDF bandwidth (default 10 MB/s)
	rep07 := mustAlertRule(t, rules, "PowerMaxSRDFBandwidthDegraded")
	assert.Contains(t, rep07["expr"], `dell_powermax_srdf_bandwidth_bytes_per_sec{namespace="test-namespace"} < 10485760`)
	assert.Equal(t, "10m", rep07["for"])
	assert.Equal(t, "warning", nestedStringField(t, rep07, "labels", "severity"))
	assert.Equal(t, "REP-07", nestedStringField(t, rep07, "labels", "alert_id"))

	rep10 := mustAlertRule(t, rules, "PowerMaxSRDFStatePersistentlyDegraded")
	assert.Contains(t, rep10["expr"], `max by (namespace, driver, rg_name, rdf_group, mode) (dell_powermax_srdf_group_state{namespace="test-namespace"})`)

	// REP-11: SRDF link down (critical, 2m)
	rep11 := mustAlertRule(t, rules, "PowerMaxSRDFLinkDown")
	assert.Contains(t, rep11["expr"], `min by (namespace, driver, rdf_group, mode) (dell_powermax_srdf_link_status{namespace="test-namespace"})`)
	assert.Equal(t, "2m", rep11["for"])
	assert.Equal(t, "critical", nestedStringField(t, rep11, "labels", "severity"))
	assert.Equal(t, "REP-11", nestedStringField(t, rep11, "labels", "alert_id"))

	// REP-12: SRDF lag (default 300s, 5m for:, warning)
	rep12 := mustAlertRule(t, rules, "PowerMaxSRDFLagExceeded")
	assert.Contains(t, rep12["expr"], `dell_powermax_srdf_lag_seconds{namespace="test-namespace"} > 300`)
	assert.Equal(t, "5m", rep12["for"])
	assert.Equal(t, "warning", nestedStringField(t, rep12, "labels", "severity"))
	assert.Equal(t, "REP-12", nestedStringField(t, rep12, "labels", "alert_id"))
}

// TestBuildReplicationModuleAlertGroups_MissingYAML verifies that a meaningful error
// is returned when prometheusrule.yaml does not exist for the given module version.
func TestBuildReplicationModuleAlertGroups_MissingYAML(t *testing.T) {
	_, err := buildReplicationModuleAlertGroups(testConfigDir, "v0.0.0-nonexistent", "test-namespace", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prometheusrule.yaml")
}

// TestBuildReplicationModuleAlertGroups_UsesCustomThresholds verifies that threshold
// placeholders in the replication YAML are replaced with values from the config.
func TestBuildReplicationModuleAlertGroups_UsesCustomThresholds(t *testing.T) {
	rpoThreshold := int32(120)
	srdfMinBandwidth := int64(5242880) // 5 MB/s
	srdfLagSeconds := int32(60)

	groups, err := buildReplicationModuleAlertGroups(testConfigDir, testReplicationModuleVersion, "test-namespace", &csmv1.ModulePrometheusRuleConfig{
		RPOThresholdSeconds: &rpoThreshold,
		SRDFMinBandwidth:    &srdfMinBandwidth,
		SRDFLagSeconds:      &srdfLagSeconds,
	})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	rules := flattenGroupRules(t, groups)
	require.Len(t, rules, 14)

	// REP-01 and REP-02 share the same RPO threshold
	assert.Contains(t, mustAlertRule(t, rules, "CSMReplicationLagExceeded")["expr"], `dell_csm_repl_lag_seconds{namespace="test-namespace"} > 120`)
	assert.Contains(t, mustAlertRule(t, rules, "CSMReplicationRPOViolation")["expr"], `dell_csm_repl_lag_seconds{namespace="test-namespace"} > 120`)

	// REP-07: custom bandwidth threshold
	assert.Contains(t, mustAlertRule(t, rules, "PowerMaxSRDFBandwidthDegraded")["expr"], "< 5242880")

	// REP-12: custom SRDF lag threshold
	assert.Contains(t, mustAlertRule(t, rules, "PowerMaxSRDFLagExceeded")["expr"], "> 60")
}

// TestApplyReplicationThresholds_ReplacesAllPlaceholders verifies that all
// placeholder tokens are substituted correctly using default values when cfg is nil.
func TestApplyReplicationThresholds_ReplacesAllPlaceholders(t *testing.T) {
	input := "<AlertRPOThresholdSeconds> <AlertSRDFMinBandwidth> <AlertSRDFLagSeconds> <AlertNamespace>"
	result := applyReplicationThresholds(input, nil, "test-namespace")
	assert.Equal(t, "300 10485760 300 test-namespace", result)
}

// TestApplyReplicationThresholds_UsesCustomValues verifies that custom values override defaults.
func TestApplyReplicationThresholds_UsesCustomValues(t *testing.T) {
	rpo := int32(60)
	bw := int64(1048576) // 1 MB/s
	lag := int32(30)
	input := "<AlertRPOThresholdSeconds> <AlertSRDFMinBandwidth> <AlertSRDFLagSeconds> <AlertNamespace>"
	result := applyReplicationThresholds(input, &csmv1.ModulePrometheusRuleConfig{
		RPOThresholdSeconds: &rpo,
		SRDFMinBandwidth:    &bw,
		SRDFLagSeconds:      &lag,
	}, "custom-namespace")
	assert.Equal(t, "60 1048576 30 custom-namespace", result)
}

func TestSyncReplicationPrometheusRule_UsesModulePrometheusRuleConfig(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pmax-repl-module-config", "test-ns", testPowerMaxVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "pmax-repl-module-config", Namespace: "test-ns", UID: types.UID("pmax-repl-module-config")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerMax
	cr.Spec.Driver.ConfigVersion = testPowerMaxVersion
	cr.Spec.Driver.Metrics = nil

	var replicationModule csmv1.Module
	require.NoError(t, sigsyaml.Unmarshal([]byte(`
name: replication
enabled: true
configVersion: v1.16.0
metrics:
  enabled: true
  prometheusRule:
    enabled: true
    rpoThresholdSeconds: 60
    srdfMinBandwidth: 2097152
    srdfLagSeconds: 30
`), &replicationModule))
	cr.Spec.Modules = []csmv1.Module{replicationModule}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncReplicationPrometheusRule(testCtx, false, replicationModule, cr, testOpConfig, fakeClient))

	replRules := replicationPrometheusRuleRules(t, fetchReplicationPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace))
	assert.Contains(t, mustAlertRule(t, replRules, "CSMReplicationLagExceeded")["expr"], "> 60")
	assert.Contains(t, mustAlertRule(t, replRules, "CSMReplicationMetricsStale")["expr"], `absent(dell_csm_repl_last_collection_timestamp_seconds{namespace="test-ns"})`)
	assert.Contains(t, mustAlertRule(t, replRules, "PowerMaxSRDFLagExceeded")["expr"], "> 30")
	assert.Contains(t, mustAlertRule(t, replRules, "PowerMaxSRDFBandwidthDegraded")["expr"], "< 2097152")
}

// TestSyncReplicationPrometheusRule_PowerMax_CreatesReplicationPrometheusRule
// verifies that enabling the replication module with prometheusRule creates a separate
// {cr.Name}-replication-alerts PrometheusRule with 14 rules in the csm-replication-alerts group.
func TestSyncReplicationPrometheusRule_PowerMax_CreatesReplicationPrometheusRule(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pmax-repl-test", "test-ns", testPowerMaxVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "pmax-repl-test", Namespace: "test-ns", UID: types.UID("pmax-repl-promrule-create")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerMax
	cr.Spec.Driver.ConfigVersion = testPowerMaxVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled: true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{
			Enabled: true,
		},
	}
	cr.Spec.Modules = []csmv1.Module{
		{
			Name:          csmv1.Replication,
			Enabled:       true,
			ConfigVersion: testReplicationModuleVersion,
			Metrics: &csmv1.ModuleMetrics{
				Enabled:        true,
				PrometheusRule: &csmv1.ModulePrometheusRuleConfig{Enabled: true},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))
	require.NoError(t, syncReplicationPrometheusRule(testCtx, false, cr.Spec.Modules[0], cr, testOpConfig, fakeClient))

	// Driver PrometheusRule must still be created with 15 rules
	driverRules := flatRulesFromPrometheusRule(t, fetchPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace))
	require.Len(t, driverRules, 15)

	// Replication PrometheusRule must be created with 14 rules
	replRules := replicationPrometheusRuleRules(t, fetchReplicationPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace))
	require.Len(t, replRules, 14)

	// Spot-check key alert thresholds in the replication rule
	assert.Contains(t, mustAlertRule(t, replRules, "CSMReplicationLagExceeded")["expr"], "> 300")
	assert.Contains(t, mustAlertRule(t, replRules, "PowerMaxSRDFLagExceeded")["expr"], "> 300")
	assert.Contains(t, mustAlertRule(t, replRules, "PowerMaxSRDFBandwidthDegraded")["expr"], "< 10485760")

	// Namespace label must be injected into all replication alert rules (consistent with driver pattern).
	for _, rule := range replRules {
		ruleMap, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}
		alertName, _ := ruleMap["alert"].(string)
		if alertName == "" {
			continue // skip record rules
		}
		assert.Equal(t, cr.Namespace, nestedStringField(t, ruleMap, "labels", "namespace"),
			"namespace label missing on replication alert %s", alertName)
	}
}

// TestSyncReplicationPrometheusRule_PowerMax_UsesCustomThresholds verifies that
// custom replication thresholds are applied to the replication PrometheusRule.
func TestSyncReplicationPrometheusRule_PowerMax_UsesCustomThresholds(t *testing.T) {
	testCtx := context.Background()
	rpoThreshold := int32(60)
	srdfLagSeconds := int32(30)
	srdfMinBandwidth := int64(2097152) // 2 MB/s

	cr := shared.MakeCSM("pmax-repl-custom", "test-ns", testPowerMaxVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "pmax-repl-custom", Namespace: "test-ns", UID: types.UID("pmax-repl-promrule-custom")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerMax
	cr.Spec.Driver.ConfigVersion = testPowerMaxVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled:        true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{Enabled: true},
	}
	cr.Spec.Modules = []csmv1.Module{
		{
			Name:          csmv1.Replication,
			Enabled:       true,
			ConfigVersion: testReplicationModuleVersion,
			Metrics: &csmv1.ModuleMetrics{
				Enabled: true,
				PrometheusRule: &csmv1.ModulePrometheusRuleConfig{
					Enabled:             true,
					RPOThresholdSeconds: &rpoThreshold,
					SRDFLagSeconds:      &srdfLagSeconds,
					SRDFMinBandwidth:    &srdfMinBandwidth,
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncReplicationPrometheusRule(testCtx, false, cr.Spec.Modules[0], cr, testOpConfig, fakeClient))

	replRules := replicationPrometheusRuleRules(t, fetchReplicationPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace))
	require.Len(t, replRules, 14)

	assert.Contains(t, mustAlertRule(t, replRules, "CSMReplicationLagExceeded")["expr"], "> 60")
	assert.Contains(t, mustAlertRule(t, replRules, "CSMReplicationRPOViolation")["expr"], "> 60")
	assert.Contains(t, mustAlertRule(t, replRules, "PowerMaxSRDFLagExceeded")["expr"], "> 30")
	assert.Contains(t, mustAlertRule(t, replRules, "PowerMaxSRDFBandwidthDegraded")["expr"], "< 2097152")
}

// TestSyncReplicationPrometheusRule_PowerMax_NoRuleWithoutReplicationModule verifies
// that no replication PrometheusRule is created when the replication module is absent from the CR.
func TestSyncReplicationPrometheusRule_PowerMax_NoRuleWithoutReplicationModule(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pmax-norepl", "test-ns", testPowerMaxVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "pmax-norepl", Namespace: "test-ns", UID: types.UID("pmax-norepl-promrule")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerMax
	cr.Spec.Driver.ConfigVersion = testPowerMaxVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled: true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{
			Enabled: true,
		},
	}
	// No replication module in Spec.Modules

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncMetricsResources(testCtx, false, cr, testOpConfig, fakeClient))
	replicationModule := csmv1.Module{
		Name:    csmv1.Replication,
		Enabled: true,
		Metrics: &csmv1.ModuleMetrics{
			Enabled:        true,
			PrometheusRule: &csmv1.ModulePrometheusRuleConfig{Enabled: true},
		},
	}
	require.NoError(t, syncReplicationPrometheusRule(testCtx, false, replicationModule, cr, testOpConfig, fakeClient))

	// Driver PrometheusRule should exist
	_ = fetchPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace)

	// Replication PrometheusRule must NOT exist
	replRule := &unstructured.Unstructured{}
	replRule.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(testCtx, types.NamespacedName{Name: replicationPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, replRule)
	require.True(t, k8sErrors.IsNotFound(err), "replication PrometheusRule must not be created when module is absent")
}

// TestSyncReplicationPrometheusRule_PowerMax_DeletesReplicationPrometheusRuleWhenModuleDisabled
// verifies that disabling the replication module removes the replication PrometheusRule.
func TestSyncReplicationPrometheusRule_PowerMax_DeletesReplicationPrometheusRuleWhenModuleDisabled(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pmax-repl-disable", "test-ns", testPowerMaxVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "pmax-repl-disable", Namespace: "test-ns", UID: types.UID("pmax-repl-promrule-disable")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerMax
	cr.Spec.Driver.ConfigVersion = testPowerMaxVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{
		Enabled: true,
		PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{
			Enabled: true,
		},
	}
	cr.Spec.Modules = []csmv1.Module{
		{
			Name:          csmv1.Replication,
			Enabled:       true,
			ConfigVersion: testReplicationModuleVersion,
			Metrics: &csmv1.ModuleMetrics{
				Enabled:        true,
				PrometheusRule: &csmv1.ModulePrometheusRuleConfig{Enabled: true},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().Build()
	require.NoError(t, syncReplicationPrometheusRule(testCtx, false, cr.Spec.Modules[0], cr, testOpConfig, fakeClient))
	// Verify replication rule exists
	_ = fetchReplicationPrometheusRule(testCtx, t, fakeClient, cr.Name, cr.Namespace)

	// Disable the replication module
	cr.Spec.Modules[0].Enabled = false
	require.NoError(t, syncReplicationPrometheusRule(testCtx, false, cr.Spec.Modules[0], cr, testOpConfig, fakeClient))

	// Replication PrometheusRule must be deleted
	replRule := &unstructured.Unstructured{}
	replRule.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(testCtx, types.NamespacedName{Name: replicationPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, replRule)
	require.True(t, k8sErrors.IsNotFound(err), "replication PrometheusRule should be deleted when module is disabled")
}

func TestRemoveModule_ReplicationDeletesPrometheusRule(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pmax-repl-remove", "test-ns", testPowerMaxVersion)
	cr.Kind = "ContainerStorageModule"
	cr.ObjectMeta = metav1.ObjectMeta{Name: "pmax-repl-remove", Namespace: "test-ns", UID: types.UID("pmax-repl-remove")}
	cr.Spec.Driver.CSIDriverType = csmv1.PowerMax
	cr.Spec.Driver.ConfigVersion = testPowerMaxVersion
	cr.Spec.Modules = []csmv1.Module{
		{
			Name:          csmv1.Replication,
			Enabled:       true,
			ConfigVersion: testReplicationModuleVersion,
			Metrics: &csmv1.ModuleMetrics{
				Enabled:        true,
				PrometheusRule: &csmv1.ModulePrometheusRuleConfig{Enabled: true},
			},
		},
	}

	prometheusRule := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "monitoring.coreos.com/v1",
			"kind":       "PrometheusRule",
			"metadata": map[string]interface{}{
				"name":      replicationPrometheusRuleName(cr.Name),
				"namespace": cr.Namespace,
			},
		},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(prometheusRule).Build()
	reconciler := &ContainerStorageModuleReconciler{}

	require.NoError(t, reconciler.removeModule(testCtx, cr, testOpConfig, fakeClient))

	actual := &unstructured.Unstructured{}
	actual.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	err := fakeClient.Get(testCtx, types.NamespacedName{Name: replicationPrometheusRuleName(cr.Name), Namespace: cr.Namespace}, actual)
	assert.True(t, k8sErrors.IsNotFound(err), "replication PrometheusRule should be deleted during module cleanup")
}

// ──────────────────────────────────────────────────────────────────────────────
// Replication test helper functions
// ──────────────────────────────────────────────────────────────────────────────

// fetchReplicationPrometheusRule fetches the {crName}-replication-alerts PrometheusRule.
func fetchReplicationPrometheusRule(testCtx context.Context, t *testing.T, ctrlClient client.Client, crName, namespace string) *unstructured.Unstructured {
	t.Helper()
	replRule := &unstructured.Unstructured{}
	replRule.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	require.NoError(t, ctrlClient.Get(testCtx, types.NamespacedName{Name: replicationPrometheusRuleName(crName), Namespace: namespace}, replRule))
	return replRule
}

// replicationPrometheusRuleRules extracts the rules from the replication PrometheusRule's
// single "csm-replication-alerts" group, asserting the group name is correct.
func replicationPrometheusRuleRules(t *testing.T, promRule *unstructured.Unstructured) []interface{} {
	t.Helper()
	spec, found, err := unstructured.NestedMap(promRule.Object, "spec")
	require.NoError(t, err)
	require.True(t, found, "replication PrometheusRule spec should be present")
	groups, ok := spec["groups"].([]interface{})
	require.True(t, ok, "replication PrometheusRule groups should be a list")
	require.Len(t, groups, 1, "replication PrometheusRule should have exactly 1 group")
	group, ok := groups[0].(map[string]interface{})
	require.True(t, ok, "replication PrometheusRule group should be a map")
	assert.Equal(t, "csm-replication-alerts", group["name"], "group name should be csm-replication-alerts")
	rules, ok := group["rules"].([]interface{})
	require.True(t, ok, "replication PrometheusRule rules should be a list")
	return rules
}

// recordRuleByName searches the rules slice for a record rule with the given name.
// Record rules have a "record" key instead of "alert".
func recordRuleByName(t *testing.T, rules []interface{}, recordName string) (map[string]interface{}, bool) {
	t.Helper()
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]interface{})
		require.True(t, ok, "rule should be a map")
		if ruleMap["record"] == recordName {
			return ruleMap, true
		}
	}
	return nil, false
}

// ──────────────────────────────────────────────────────────────────────────────
// Branch-coverage tests for resolve* and parse* functions
// ──────────────────────────────────────────────────────────────────────────────

// TestResolvePrometheusRuleGroups_BuildError verifies the warning+false return when
// the prometheusrule.yaml cannot be read (bad config directory).
func TestResolvePrometheusRuleGroups_BuildError(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pscale-test", "test", shared.PScaleConfigVersion)
	cr.Spec.Driver.CSIDriverType = csmv1.PowerScale
	cr.Spec.Driver.ConfigVersion = shared.PScaleConfigVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{}}

	badConfig := operatorutils.OperatorConfig{ConfigDirectory: "/nonexistent/path"}
	groups, ok := resolvePrometheusRuleGroups(testCtx, cr, badConfig)
	assert.Nil(t, groups)
	assert.False(t, ok)
}

// TestResolvePrometheusRuleGroups_GetVersionError verifies the warning+false return when
// spec.driver.configVersion is empty and spec.version cannot be resolved.
func TestResolvePrometheusRuleGroups_GetVersionError(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pscale-test", "test", "")
	cr.Spec.Driver.CSIDriverType = csmv1.PowerScale
	cr.Spec.Driver.ConfigVersion = ""
	cr.Spec.Version = shared.InvalidCSMVersion // Cannot be resolved
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{}}

	groups, ok := resolvePrometheusRuleGroups(testCtx, cr, testOpConfig)
	assert.Nil(t, groups)
	assert.False(t, ok)
}

// TestResolveReplicationModuleVersion_NoModuleInCR verifies "", false when the replication
// module is not present in the CR's Spec.Modules.
func TestResolveReplicationModuleVersion_NoModuleInCR(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pmax-test", "test-ns", testPowerMaxVersion)
	cr.Spec.Driver.CSIDriverType = csmv1.PowerMax
	cr.Spec.Driver.ConfigVersion = testPowerMaxVersion
	// No replication module in Spec.Modules

	version, ok := resolveReplicationModuleVersion(testCtx, cr, testOpConfig)
	assert.Empty(t, version)
	assert.False(t, ok)
}

// TestResolveReplicationModuleVersion_DeriveFromDriverVersion verifies that when
// m.ConfigVersion is empty, the replication module version is correctly derived from
// the driver config version via csm-releases.yaml.
func TestResolveReplicationModuleVersion_DeriveFromDriverVersion(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pmax-test", "test-ns", testPowerMaxVersion)
	cr.Spec.Driver.CSIDriverType = csmv1.PowerMax
	cr.Spec.Driver.ConfigVersion = testPowerMaxVersion
	cr.Spec.Modules = []csmv1.Module{
		{
			Name:    csmv1.Replication,
			Enabled: true,
			// ConfigVersion intentionally left empty — should be derived from csm-releases.yaml
		},
	}

	version, ok := resolveReplicationModuleVersion(testCtx, cr, testOpConfig)
	require.True(t, ok)
	assert.Equal(t, testReplicationModuleVersion, version)
}

// TestResolveReplicationModuleVersion_GetVersionError verifies "", false when both
// m.ConfigVersion and driver.ConfigVersion are empty and spec.version cannot be resolved.
func TestResolveReplicationModuleVersion_GetVersionError(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pmax-test", "test-ns", "")
	cr.Spec.Driver.CSIDriverType = csmv1.PowerMax
	cr.Spec.Driver.ConfigVersion = ""
	cr.Spec.Version = shared.InvalidCSMVersion // Cannot be resolved by GetVersion
	cr.Spec.Modules = []csmv1.Module{
		{
			Name:    csmv1.Replication,
			Enabled: true,
			// ConfigVersion left empty → triggers GetVersion which will fail
		},
	}

	version, ok := resolveReplicationModuleVersion(testCtx, cr, testOpConfig)
	assert.Empty(t, version)
	assert.False(t, ok)
}

// TestResolveReplicationModuleVersion_GetModuleDefaultVersionError verifies "", false when
// the driver config version is valid but has no replication module mapping in csm-releases.yaml.
func TestResolveReplicationModuleVersion_GetModuleDefaultVersionError(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pmax-test", "test-ns", "")
	cr.Spec.Driver.CSIDriverType = csmv1.PowerMax
	cr.Spec.Driver.ConfigVersion = "v0.0.0-nonexistent" // Not in csm-releases.yaml
	cr.Spec.Modules = []csmv1.Module{
		{
			Name:    csmv1.Replication,
			Enabled: true,
		},
	}

	version, ok := resolveReplicationModuleVersion(testCtx, cr, testOpConfig)
	assert.Empty(t, version)
	assert.False(t, ok)
}

// TestResolveReplicationModuleAlertGroups_NoModule verifies nil, false when the replication
// module is not in the CR's Spec.Modules.
func TestResolveReplicationModuleAlertGroups_NoModule(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pmax-test", "test-ns", testPowerMaxVersion)
	cr.Spec.Driver.CSIDriverType = csmv1.PowerMax
	cr.Spec.Driver.ConfigVersion = testPowerMaxVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{}}

	groups, ok := resolveReplicationModuleAlertGroups(testCtx, cr, testOpConfig)
	assert.Nil(t, groups)
	assert.False(t, ok)
}

// TestResolveReplicationModuleAlertGroups_BadConfigDir verifies nil, false when the
// config directory does not contain the module YAML.
func TestResolveReplicationModuleAlertGroups_BadConfigDir(t *testing.T) {
	testCtx := context.Background()
	cr := shared.MakeCSM("pmax-test", "test-ns", testPowerMaxVersion)
	cr.Spec.Driver.CSIDriverType = csmv1.PowerMax
	cr.Spec.Driver.ConfigVersion = testPowerMaxVersion
	cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{PrometheusRule: &csmv1.MetricsPrometheusRuleConfig{}}
	cr.Spec.Modules = []csmv1.Module{
		{
			Name:          csmv1.Replication,
			Enabled:       true,
			ConfigVersion: testReplicationModuleVersion,
		},
	}

	badConfig := operatorutils.OperatorConfig{ConfigDirectory: "/nonexistent/path"}
	groups, ok := resolveReplicationModuleAlertGroups(testCtx, cr, badConfig)
	assert.Nil(t, groups)
	assert.False(t, ok)
}
