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

package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestModulePrometheusRuleConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      ModulePrometheusRuleConfig
		expectError bool
	}{
		{
			name: "valid enabled true",
			config: ModulePrometheusRuleConfig{
				Enabled: true,
			},
			expectError: false,
		},
		{
			name: "valid enabled false",
			config: ModulePrometheusRuleConfig{
				Enabled: false,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			// Test YAML serialization
			yamlBytes, err := yaml.Marshal(tt.config)
			require.NoError(t, err)

			// Test YAML deserialization
			var decoded ModulePrometheusRuleConfig
			err = yaml.Unmarshal(yamlBytes, &decoded)
			require.NoError(t, err)

			// Verify enabled field is preserved when false
			assert.Equal(t, tt.config.Enabled, decoded.Enabled)
		})
	}
}

func TestModuleMetricsWithPrometheusRule(t *testing.T) {
	metrics := &ModuleMetrics{
		Enabled: true,
		PrometheusRule: &ModulePrometheusRuleConfig{
			Enabled: true,
		},
	}

	// Test JSON serialization
	jsonBytes, err := json.Marshal(metrics)
	require.NoError(t, err)

	// Test JSON deserialization
	var decoded ModuleMetrics
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)
	assert.NotNil(t, decoded.PrometheusRule)
	assert.True(t, decoded.PrometheusRule.Enabled)
}

func TestModuleMetricsWithPrometheusRuleDisabled(t *testing.T) {
	metrics := &ModuleMetrics{
		Enabled: true,
		PrometheusRule: &ModulePrometheusRuleConfig{
			Enabled: false,
		},
	}

	// Test JSON serialization
	jsonBytes, err := json.Marshal(metrics)
	require.NoError(t, err)

	// Test JSON deserialization
	var decoded ModuleMetrics
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)
	assert.NotNil(t, decoded.PrometheusRule)
	assert.False(t, decoded.PrometheusRule.Enabled)
}

func TestModuleMetricsWithNilPrometheusRule(t *testing.T) {
	metrics := &ModuleMetrics{
		Enabled:        true,
		PrometheusRule: nil,
	}

	// Test JSON serialization
	jsonBytes, err := json.Marshal(metrics)
	require.NoError(t, err)

	// Test JSON deserialization
	var decoded ModuleMetrics
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)
	assert.Nil(t, decoded.PrometheusRule)
}
