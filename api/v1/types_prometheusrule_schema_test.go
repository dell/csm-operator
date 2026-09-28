// Copyright © 2021-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package v1

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sigsyaml "sigs.k8s.io/yaml"
)

func loadCRDv1Schema(t *testing.T) map[string]interface{} {
	t.Helper()
	crdPath := filepath.Join("..", "..", "config", "crd", "bases", "storage.dell.com_containerstoragemodules.yaml")
	data, err := os.ReadFile(crdPath)
	require.NoError(t, err)

	var crd map[string]interface{}
	require.NoError(t, sigsyaml.Unmarshal(data, &crd))

	versions, ok := crd["spec"].(map[string]interface{})["versions"].([]interface{})
	require.True(t, ok)
	require.NotEmpty(t, versions)

	var v1Schema map[string]interface{}
	for _, version := range versions {
		versionMap, ok := version.(map[string]interface{})
		require.True(t, ok)
		if versionMap["name"] == "v1" {
			v1Schema, ok = versionMap["schema"].(map[string]interface{})
			require.True(t, ok)
			break
		}
	}
	require.NotNil(t, v1Schema, "v1 schema must exist in the CRD")

	openAPI, ok := v1Schema["openAPIV3Schema"].(map[string]interface{})
	require.True(t, ok)
	properties, ok := openAPI["properties"].(map[string]interface{})
	require.True(t, ok)
	specProperties, ok := properties["spec"].(map[string]interface{})["properties"].(map[string]interface{})
	require.True(t, ok)
	return specProperties
}

func TestContainerStorageModuleCRDIncludesDriverMetricsPrometheusRuleSchema(t *testing.T) {
	specProperties := loadCRDv1Schema(t)
	driverProperties, ok := specProperties["driver"].(map[string]interface{})["properties"].(map[string]interface{})
	require.True(t, ok)
	metricsProperties, ok := driverProperties["metrics"].(map[string]interface{})["properties"].(map[string]interface{})
	require.True(t, ok)

	prometheusRule, ok := metricsProperties["prometheusRule"].(map[string]interface{})
	require.True(t, ok, "driver.metrics.prometheusRule schema must exist")

	prometheusRuleProperties, ok := prometheusRule["properties"].(map[string]interface{})
	require.True(t, ok)

	testCases := map[string]struct {
		fieldType interface{}
		defValue  interface{}
	}{
		"enabled":                         {fieldType: "boolean", defValue: true},
		"quotaWarningThreshold":           {fieldType: "integer", defValue: float64(80)},
		"quotaCriticalThreshold":          {fieldType: "integer", defValue: float64(90)},
		"nodepoolWarningThreshold":        {fieldType: "integer", defValue: float64(80)},
		"nodepoolCriticalThreshold":       {fieldType: "integer", defValue: float64(90)},
		"apiErrorRateThreshold":           {fieldType: "integer", defValue: float64(10)},
		"apiErrorRateWindow":              {fieldType: "string", defValue: "5m"},
		"cpuWarningThreshold":             {fieldType: "integer", defValue: float64(80)},
		"memoryWarningThreshold":          {fieldType: "integer", defValue: float64(2147483648)},
		"nfsV3LatencyWarningSeconds":      {fieldType: "integer", defValue: float64(10)},
		"nfsV4LatencyWarningSeconds":      {fieldType: "integer", defValue: float64(10)},
		"driverCrashLoopRestartThreshold": {fieldType: "integer", defValue: float64(3)},
		"driverCrashLoopWindow":           {fieldType: "string", defValue: "15m"},
		"restartCountThreshold":           {fieldType: "integer", defValue: float64(3)},
		"poolCapacityWarningPercent":      {fieldType: "integer", defValue: float64(80)},
		"poolCapacityCriticalPercent":     {fieldType: "integer", defValue: float64(90)},
		"thinRatioWarningThreshold":       {fieldType: "string", defValue: "0.8"},
		"dataReductionDegradedThreshold":  {fieldType: "string", defValue: "1.5"},
		"rcgLagWarningSeconds":            {fieldType: "integer", defValue: float64(300)},
		"rcgBandwidthWarningKBps":         {fieldType: "integer", defValue: float64(10240)},
		"rcgLatencyWarningSeconds":        {fieldType: "string", defValue: "5.0"},
		"applianceWarningThreshold":       {fieldType: "integer", defValue: float64(80)},
		"applianceCriticalThreshold":      {fieldType: "integer", defValue: float64(90)},
		"volumeOperationFailureThreshold": {fieldType: "integer", defValue: float64(3)},
		"storageGroupCapacityWarning":     {fieldType: "integer", defValue: float64(80)},
		"storageGroupCapacityCritical":    {fieldType: "integer", defValue: float64(90)},
		"srpSnapshotCapacityWarning":      {fieldType: "integer", defValue: float64(80)},
	}

	_, exists := prometheusRuleProperties["connectivitySuccessRatio"]
	assert.False(t, exists, "driver metrics prometheusRule must not expose resiliency connectivitySuccessRatio")

	for fieldName, tc := range testCases {
		fieldSchema, ok := prometheusRuleProperties[fieldName].(map[string]interface{})
		require.Truef(t, ok, "%s schema must exist", fieldName)
		assert.Equal(t, tc.fieldType, fieldSchema["type"], "%s type mismatch", fieldName)
		assert.EqualValues(t, tc.defValue, fieldSchema["default"], "%s default mismatch", fieldName)
	}
	for _, fieldName := range []string{"connectivitySuccessRatio", "rpoThresholdSeconds", "srdfMinBandwidth", "srdfLagSeconds"} {
		assert.NotContains(t, prometheusRuleProperties, fieldName)
	}
	volumeThreshold := prometheusRuleProperties["volumeOperationFailureThreshold"].(map[string]interface{})
	assert.EqualValues(t, 1, volumeThreshold["minimum"])
}

func TestContainerStorageModuleCRDIncludesModuleMetricsPrometheusRuleSchema(t *testing.T) {
	specProperties := loadCRDv1Schema(t)
	modules, ok := specProperties["modules"].(map[string]interface{})
	require.True(t, ok)
	items, ok := modules["items"].(map[string]interface{})
	require.True(t, ok)
	moduleProperties, ok := items["properties"].(map[string]interface{})
	require.True(t, ok)
	metrics, ok := moduleProperties["metrics"].(map[string]interface{})
	require.True(t, ok)
	metricsProperties, ok := metrics["properties"].(map[string]interface{})
	require.True(t, ok)
	prometheusRule, ok := metricsProperties["prometheusRule"].(map[string]interface{})
	require.True(t, ok, "modules[].metrics.prometheusRule schema must exist")
	prometheusRuleProperties, ok := prometheusRule["properties"].(map[string]interface{})
	require.True(t, ok)

	testCases := map[string]struct {
		fieldType interface{}
		defValue  interface{}
		minimum   interface{}
		maximum   interface{}
	}{
		// enabled field
		"enabled": {fieldType: "boolean", defValue: true},
		// Authorization-specific fields present in module schema
		"authFailureRateThreshold":    {fieldType: "integer", defValue: float64(10)},
		"authLatencyQuantile":         {fieldType: "integer", defValue: float64(95)},
		"authLatencyThresholdSeconds": {fieldType: "integer", defValue: float64(2)},
		"connectivitySuccessRatio":    {fieldType: "integer", defValue: float64(90)},
		"rpoThresholdSeconds":         {fieldType: "integer", defValue: float64(300), minimum: float64(30), maximum: float64(86400)},
		"srdfMinBandwidth":            {fieldType: "integer", defValue: float64(10485760), minimum: float64(1), maximum: float64(1073741824)},
		"srdfLagSeconds":              {fieldType: "integer", defValue: float64(300), minimum: float64(30), maximum: float64(86400)},
	}

	for fieldName, tc := range testCases {
		fieldSchema, ok := prometheusRuleProperties[fieldName].(map[string]interface{})
		require.Truef(t, ok, "%s schema must exist", fieldName)
		assert.Equal(t, tc.fieldType, fieldSchema["type"], "%s type mismatch", fieldName)
		assert.EqualValues(t, tc.defValue, fieldSchema["default"], "%s default mismatch", fieldName)
		if tc.minimum != nil {
			assert.EqualValues(t, tc.minimum, fieldSchema["minimum"], "%s minimum mismatch", fieldName)
			assert.EqualValues(t, tc.maximum, fieldSchema["maximum"], "%s maximum mismatch", fieldName)
		}
	}
}

func TestPrometheusRuleConfigTypesSeparateDriverAndModuleFields(t *testing.T) {
	driverConfig := reflect.TypeOf(MetricsPrometheusRuleConfig{})
	moduleMetrics := reflect.TypeOf(ModuleMetrics{})
	moduleConfig := reflect.TypeOf(ModulePrometheusRuleConfig{})

	for _, fieldName := range []string{"ConnectivitySuccessRatio", "RPOThresholdSeconds", "SRDFMinBandwidth", "SRDFLagSeconds"} {
		_, found := driverConfig.FieldByName(fieldName)
		assert.False(t, found)
	}

	prometheusRuleField, hasModulePrometheusRule := moduleMetrics.FieldByName("PrometheusRule")
	require.True(t, hasModulePrometheusRule)
	assert.Equal(t, reflect.TypeOf((*ModulePrometheusRuleConfig)(nil)), prometheusRuleField.Type)

	for _, fieldName := range []string{"ConnectivitySuccessRatio", "RPOThresholdSeconds", "SRDFMinBandwidth", "SRDFLagSeconds"} {
		_, found := moduleConfig.FieldByName(fieldName)
		assert.True(t, found)
	}
}
