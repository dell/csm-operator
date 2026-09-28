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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/modules"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	sigsyaml "sigs.k8s.io/yaml"
)

// Default threshold values used when the MetricsPrometheusRuleConfig field is nil or unset.
// These match the kubebuilder defaults in api/v1/types.go.
const (
	// PowerScale defaults.
	defaultQuotaWarningThreshold     int32 = 80
	defaultQuotaCriticalThreshold    int32 = 90
	defaultNodepoolWarningThreshold  int32 = 80
	defaultNodepoolCriticalThreshold int32 = 90
	defaultAPIErrorRateThreshold     int32 = 10
	defaultAPIErrorRateWindow              = "5m"
	defaultCPUWarningThreshold       int32 = 80
	defaultMemoryWarningThreshold    int64 = 2147483648
	defaultNFSLatencyWarningSeconds  int32 = 10
	defaultCrashLoopRestartThreshold int32 = 3
	defaultCrashLoopWindow                 = "15m"

	// PowerFlex defaults.
	// CPU and memory defaults are declared separately from the PowerScale ones above so the
	// two drivers can diverge without silently changing each other's alerting behaviour,
	// even though the values currently coincide.
	defaultPFlexCPUWarningThreshold             int32 = 80
	defaultPFlexMemoryWarningThreshold          int64 = 2147483648
	defaultPFlexRestartCountThreshold           int32 = 3
	defaultPFlexPoolCapacityWarningPercent      int32 = 80
	defaultPFlexPoolCapacityCriticalPercent     int32 = 90
	defaultPFlexThinRatioWarningThreshold             = "0.8"
	defaultPFlexDataReductionDegradedThresh           = "1.5"
	defaultPFlexRCGLagWarningSeconds            int64 = 300
	defaultPFlexRCGBandwidthWarningKBps         int64 = 10240
	defaultPFlexRCGLatencyWarningSeconds              = "5.0"
	defaultPFlexVolumeOperationFailureThreshold int32 = 3

	// PowerStore defaults.
	defaultApplianceWarningThreshold  int32 = 80
	defaultApplianceCriticalThreshold int32 = 90

	// Resiliency defaults.
	defaultConnectivitySuccessRatio int32 = 90

	// PowerMax and PowerStore shared defaults.
	defaultVolumeOperationFailureThreshold int32 = 3

	// PowerMax-specific defaults.
	defaultStorageGroupCapacityWarning  int32 = 80
	defaultStorageGroupCapacityCritical int32 = 90
	// defaultSRPSnapshotCapacityWarning: 80% matches the Helm chart default and ER specification.
	defaultSRPSnapshotCapacityWarning int32 = 80

	// CSM Replication module defaults (shared across all drivers; SRDF-specific ones are PowerMax-only).
	defaultRPOThresholdSeconds int32 = 300      // 5 minutes — REP-01, REP-02
	defaultSRDFMinBandwidth    int64 = 10485760 // 10 MB/s  — REP-07
	defaultSRDFLagSeconds      int32 = 300      // 5 minutes — REP-12
)

// resolvePrometheusRuleGroups resolves the config version (from spec.driver.configVersion or
// spec.version via the version registry) and loads the rule groups from the per-version YAML file.
// It returns an empty slice when the driver has no prometheusrule.yaml (not an error — caller
// treats an empty return as "no PrometheusRule to create").
// Errors are logged as warnings; the boolean return indicates whether any groups were loaded.
func resolvePrometheusRuleGroups(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig) ([]interface{}, bool) {
	log := logger.GetLogger(ctx)

	configVersion := cr.Spec.Driver.ConfigVersion
	if configVersion == "" {
		resolved, err := operatorutils.GetVersion(ctx, &cr, operatorConfig)
		if err != nil {
			log.Warnw("Failed to resolve config version for PrometheusRule, skipping", "driver", cr.GetDriverType(), "error", err)
			return nil, false
		}
		configVersion = resolved
	}

	groups, err := buildDriverAlertGroups(operatorConfig.ConfigDirectory, cr.GetDriverType(), configVersion, cr.Spec.Driver.Metrics.PrometheusRule)
	if err != nil {
		log.Warnw("Failed to load PrometheusRule YAML, skipping", "driver", cr.GetDriverType(), "version", configVersion, "error", err)
		return nil, false
	}
	return groups, len(groups) > 0
}

// normalizeDriverType collapses type aliases to their canonical form so that
// buildDriverAlertGroups only needs a single case per driver.
// For example, csmv1.PowerFlexName ("vxflexos") → csmv1.PowerFlex ("powerflex").
func normalizeDriverType(driverType csmv1.DriverType) csmv1.DriverType {
	switch driverType {
	case csmv1.PowerFlexName:
		return csmv1.PowerFlex
	case csmv1.PowerScaleName:
		return csmv1.PowerScale
	default:
		return driverType
	}
}

// buildDriverAlertGroups loads the PrometheusRule rule groups for the given driver from the
// per-version YAML file at:
//
//	<configDir>/driverconfig/<driverFolderName>/<version>/prometheusrule.yaml
//
// The YAML file is structured as a list of PrometheusRule groups (PrometheusRule.spec.groups),
// each containing a name, optional interval, and a list of alert rules.
// Threshold placeholders in the YAML (e.g. <AlertQuotaWarningThreshold>) are substituted
// with values from cfg before parsing.  The namespace label is NOT injected here — callers
// (i.e. the controller) invoke injectNamespaceLabelIntoGroups after loading so that every
// alert rule carries the correct namespace for AlertmanagerConfig namespace-scoped routing.
// Drivers that do not yet have a prometheusrule.yaml return an empty slice without error —
// callers treat an empty slice as "no PrometheusRule".
//
// Adding alert rules for a new driver requires only:
//  1. Creating operatorconfig/driverconfig/<driver>/<version>/prometheusrule.yaml
//     using the group YAML structure (list of groups, each with name, interval, and rules).
//  2. Adding a case to the switch below (and an applyXxxThresholds helper if thresholds differ).
//
// No other Go code changes are needed.
func buildDriverAlertGroups(configDir string, driverType csmv1.DriverType, version string, cfg *csmv1.MetricsPrometheusRuleConfig) ([]interface{}, error) {
	switch normalizeDriverType(driverType) {
	case csmv1.PowerScale:
		return loadPrometheusRuleGroups(configDir, string(csmv1.PowerScaleName), version, applyPowerScaleThresholds, cfg)
	case csmv1.PowerFlex:
		return loadPrometheusRuleGroups(configDir, string(csmv1.PowerFlex), version, applyPowerFlexThresholds, cfg)
	case csmv1.PowerStore:
		return loadPrometheusRuleGroups(configDir, string(csmv1.PowerStore), version, applyPowerStoreThresholds, cfg)
	case csmv1.PowerMax:
		return loadPrometheusRuleGroups(configDir, string(csmv1.PowerMax), version, applyPowerMaxThresholds, cfg)
	default:
		return []interface{}{}, nil
	}
}

// loadPrometheusRuleGroups reads <configDir>/driverconfig/<folderName>/<version>/prometheusrule.yaml,
// applies threshold substitutions via applyThresholds, and returns the parsed groups slice.
func loadPrometheusRuleGroups(
	configDir, folderName, version string,
	applyThresholds func(string, *csmv1.MetricsPrometheusRuleConfig) string,
	cfg *csmv1.MetricsPrometheusRuleConfig,
) ([]interface{}, error) {
	path := filepath.Join(configDir, "driverconfig", folderName, version, "prometheusrule.yaml")
	buf, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("reading prometheusrule.yaml for %s/%s: %w", folderName, version, err)
	}
	yamlStr := applyThresholds(string(buf), cfg)
	return parsePrometheusRuleGroupsYAML(yamlStr)
}

// parsePrometheusRuleGroupsYAML unmarshals a YAML list of PrometheusRule group maps into []interface{}.
// Each element in the list represents one PrometheusRule group (with "name", optional "interval",
// and "rules" fields).  The sigs.k8s.io/yaml package is used (consistent with the rest of the
// operator) so that nested maps are decoded as map[string]interface{} (JSON semantics).
// All driver prometheusrule.yaml files (PowerScale, PowerStore, etc.) use this bare-list format.
func parsePrometheusRuleGroupsYAML(yamlStr string) ([]interface{}, error) {
	var groups []interface{}
	if err := sigsyaml.Unmarshal([]byte(yamlStr), &groups); err != nil {
		return nil, fmt.Errorf("parsing prometheusrule YAML: %w", err)
	}
	return groups, nil
}

// applyPowerScaleThresholds replaces <Alert*> placeholders in the YAML string with
// the resolved threshold values from cfg (falling back to defaults when nil).
func applyPowerScaleThresholds(yamlStr string, cfg *csmv1.MetricsPrometheusRuleConfig) string {
	safe := nilSafePrometheusRule(cfg)

	r := strings.NewReplacer(
		"<AlertVolumeOperationFailureThreshold>", fmt.Sprintf("%d", int32Val(safe.VolumeOperationFailureThreshold, defaultVolumeOperationFailureThreshold)),
		"<AlertQuotaWarningThreshold>", fmt.Sprintf("%d", int32Val(safe.QuotaWarningThreshold, defaultQuotaWarningThreshold)),
		"<AlertQuotaCriticalThreshold>", fmt.Sprintf("%d", int32Val(safe.QuotaCriticalThreshold, defaultQuotaCriticalThreshold)),
		"<AlertNodepoolWarningThreshold>", fmt.Sprintf("%d", int32Val(safe.NodepoolWarningThreshold, defaultNodepoolWarningThreshold)),
		"<AlertNodepoolCriticalThreshold>", fmt.Sprintf("%d", int32Val(safe.NodepoolCriticalThreshold, defaultNodepoolCriticalThreshold)),
		"<AlertAPIErrorRateThreshold>", fmt.Sprintf("%d", int32Val(safe.APIErrorRateThreshold, defaultAPIErrorRateThreshold)),
		"<AlertAPIErrorRateWindow>", stringVal(safe.APIErrorRateWindow, defaultAPIErrorRateWindow),
		"<AlertCPUWarningThreshold>", fmt.Sprintf("%d", int32Val(safe.CPUWarningThreshold, defaultCPUWarningThreshold)),
		"<AlertMemoryWarningThreshold>", fmt.Sprintf("%d", int64Val(safe.MemoryWarningThreshold, defaultMemoryWarningThreshold)),
		"<AlertNFSv3LatencyWarningSeconds>", fmt.Sprintf("%d", int32Val(safe.NFSv3LatencyWarningSeconds, defaultNFSLatencyWarningSeconds)),
		"<AlertNFSv4LatencyWarningSeconds>", fmt.Sprintf("%d", int32Val(safe.NFSv4LatencyWarningSeconds, defaultNFSLatencyWarningSeconds)),
		"<AlertDriverCrashLoopRestartThreshold>", fmt.Sprintf("%d", int32Val(safe.DriverCrashLoopRestartThreshold, defaultCrashLoopRestartThreshold)),
		"<AlertDriverCrashLoopWindow>", stringVal(safe.DriverCrashLoopWindow, defaultCrashLoopWindow),
	)
	return r.Replace(yamlStr)
}

// applyPowerFlexThresholds replaces <Alert*> placeholders in the YAML string with
// the resolved threshold values from cfg (falling back to defaults when nil).
func applyPowerFlexThresholds(yamlStr string, cfg *csmv1.MetricsPrometheusRuleConfig) string {
	safe := nilSafePrometheusRule(cfg)

	r := strings.NewReplacer(
		"<AlertVolumeOperationFailureThreshold>", fmt.Sprintf("%d", int32Val(safe.VolumeOperationFailureThreshold, defaultPFlexVolumeOperationFailureThreshold)),
		"<AlertRestartCountThreshold>", fmt.Sprintf("%d", int32Val(safe.RestartCountThreshold, defaultPFlexRestartCountThreshold)),
		"<AlertCPUWarningThreshold>", fmt.Sprintf("%d", int32Val(safe.CPUWarningThreshold, defaultPFlexCPUWarningThreshold)),
		"<AlertMemoryWarningThreshold>", fmt.Sprintf("%d", int64Val(safe.MemoryWarningThreshold, defaultPFlexMemoryWarningThreshold)),
		"<AlertPoolCapacityWarningPercent>", fmt.Sprintf("%d", int32Val(safe.PoolCapacityWarningPercent, defaultPFlexPoolCapacityWarningPercent)),
		"<AlertPoolCapacityCriticalPercent>", fmt.Sprintf("%d", int32Val(safe.PoolCapacityCriticalPercent, defaultPFlexPoolCapacityCriticalPercent)),
		"<AlertThinRatioWarningThreshold>", stringVal(safe.ThinRatioWarningThreshold, defaultPFlexThinRatioWarningThreshold),
		"<AlertDataReductionDegradedThreshold>", stringVal(safe.DataReductionDegradedThreshold, defaultPFlexDataReductionDegradedThresh),
		"<AlertRCGLagWarningSeconds>", fmt.Sprintf("%d", int64Val(safe.RCGLagWarningSeconds, defaultPFlexRCGLagWarningSeconds)),
		"<AlertRCGBandwidthWarningKBps>", fmt.Sprintf("%d", int64Val(safe.RCGBandwidthWarningKBps, defaultPFlexRCGBandwidthWarningKBps)),
		"<AlertRCGLatencyWarningSeconds>", stringVal(safe.RCGLatencyWarningSeconds, defaultPFlexRCGLatencyWarningSeconds),
	)
	return r.Replace(yamlStr)
}

// applyPowerStoreThresholds replaces <Alert*> placeholders in the YAML string with
// the resolved threshold values from cfg (falling back to defaults when nil).
func applyPowerStoreThresholds(yamlStr string, cfg *csmv1.MetricsPrometheusRuleConfig) string {
	safe := nilSafePrometheusRule(cfg)

	r := strings.NewReplacer(
		"<AlertApplianceWarningThreshold>", fmt.Sprintf("%d", int32Val(safe.ApplianceWarningThreshold, defaultApplianceWarningThreshold)),
		"<AlertApplianceCriticalThreshold>", fmt.Sprintf("%d", int32Val(safe.ApplianceCriticalThreshold, defaultApplianceCriticalThreshold)),
		"<AlertAPIErrorRateThreshold>", fmt.Sprintf("%d", int32Val(safe.APIErrorRateThreshold, defaultAPIErrorRateThreshold)),
		"<AlertAPIErrorRateWindow>", stringVal(safe.APIErrorRateWindow, defaultAPIErrorRateWindow),
		"<AlertCPUWarningThreshold>", fmt.Sprintf("%d", int32Val(safe.CPUWarningThreshold, defaultCPUWarningThreshold)),
		"<AlertMemoryWarningThreshold>", fmt.Sprintf("%d", int64Val(safe.MemoryWarningThreshold, defaultMemoryWarningThreshold)),
		"<AlertDriverCrashLoopRestartThreshold>", fmt.Sprintf("%d", int32Val(safe.DriverCrashLoopRestartThreshold, defaultCrashLoopRestartThreshold)),
		"<AlertDriverCrashLoopWindow>", stringVal(safe.DriverCrashLoopWindow, defaultCrashLoopWindow),
		"<AlertVolumeOperationFailureThreshold>", fmt.Sprintf("%d", int32Val(safe.VolumeOperationFailureThreshold, defaultVolumeOperationFailureThreshold)),
	)
	return r.Replace(yamlStr)
}

// applyPowerMaxThresholds replaces <Alert*> placeholders in the PowerMax YAML string with
// the resolved threshold values from cfg (falling back to defaults when nil).
// The namespace label is NOT set here — it is injected uniformly for all alert rules by
// injectNamespaceLabelIntoGroups after loading, consistent with the PowerScale pattern.
// This ensures that every alert (including absent()- and sum by()-based rules that carry
// no metric namespace label) reaches the correct AlertmanagerConfig namespace-scoped route.
func applyPowerMaxThresholds(yamlStr string, cfg *csmv1.MetricsPrometheusRuleConfig) string {
	safe := nilSafePrometheusRule(cfg)

	r := strings.NewReplacer(
		"<AlertVolumeOperationFailureThreshold>", fmt.Sprintf("%d", int32Val(safe.VolumeOperationFailureThreshold, defaultVolumeOperationFailureThreshold)),
		"<AlertCPUWarningThreshold>", fmt.Sprintf("%d", int32Val(safe.CPUWarningThreshold, defaultCPUWarningThreshold)),
		"<AlertMemoryWarningThreshold>", fmt.Sprintf("%d", int64Val(safe.MemoryWarningThreshold, defaultMemoryWarningThreshold)),
		"<AlertAPIErrorRateThreshold>", fmt.Sprintf("%d", int32Val(safe.APIErrorRateThreshold, defaultAPIErrorRateThreshold)),
		"<AlertAPIErrorRateWindow>", stringVal(safe.APIErrorRateWindow, defaultAPIErrorRateWindow),
		"<AlertStorageGroupCapacityWarning>", fmt.Sprintf("%d", int32Val(safe.StorageGroupCapacityWarning, defaultStorageGroupCapacityWarning)),
		"<AlertStorageGroupCapacityCritical>", fmt.Sprintf("%d", int32Val(safe.StorageGroupCapacityCritical, defaultStorageGroupCapacityCritical)),
		"<AlertSRPSnapshotCapacityWarning>", fmt.Sprintf("%d", int32Val(safe.SRPSnapshotCapacityWarning, defaultSRPSnapshotCapacityWarning)),
	)
	return r.Replace(yamlStr)
}

// int64Val returns *p if non-nil, otherwise def.
func int64Val(p *int64, def int64) int64 {
	if p != nil {
		return *p
	}
	return def
}

// int32Val returns *p if non-nil, otherwise def.
func int32Val(p *int32, def int32) int32 {
	if p != nil {
		return *p
	}
	return def
}

// stringVal returns *p if non-nil and non-empty, otherwise def.
func stringVal(p *string, def string) string {
	if p != nil && *p != "" {
		return *p
	}
	return def
}

// nilSafePrometheusRule returns cfg if non-nil, otherwise an empty config (all fields nil/zero).
func nilSafePrometheusRule(cfg *csmv1.MetricsPrometheusRuleConfig) *csmv1.MetricsPrometheusRuleConfig {
	if cfg != nil {
		return cfg
	}
	return &csmv1.MetricsPrometheusRuleConfig{}
}

// metricsPrometheusRuleName returns the name of the PrometheusRule resource for a given CR name.
func metricsPrometheusRuleName(crName string) string {
	return crName + "-alerts"
}

// injectNamespaceLabelIntoGroups adds a "namespace" label to every alert rule inside each group
// and narrows any broad namespace regex filters in PromQL expressions to the specific namespace.
// This is required for AlertmanagerConfig namespace-scoped routing: Alertmanager uses the
// namespace label to match alerts to the correct AlertmanagerConfig in multi-tenant clusters.
// The groups slice is the []interface{} parsed from prometheusrule.yaml — each element is a
// map[string]interface{} with "name", optional "interval", and "rules" ([]interface{}) fields.
func injectNamespaceLabelIntoGroups(groups []interface{}, namespace string) {
	for _, g := range groups {
		groupMap, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		rules, ok := groupMap["rules"].([]interface{})
		if !ok {
			continue
		}
		for _, rule := range rules {
			ruleMap, ok := rule.(map[string]interface{})
			if !ok {
				continue
			}
			labels, ok := ruleMap["labels"].(map[string]interface{})
			if !ok {
				continue
			}
			labels["namespace"] = namespace
			// Narrow broad namespace regex in PromQL expressions to the specific namespace
			// so that alerts are scoped to their own driver namespace in multi-tenant clusters.
			if expr, ok := ruleMap["expr"].(string); ok {
				ruleMap["expr"] = strings.ReplaceAll(expr, `namespace=~".+"`, fmt.Sprintf(`namespace="%s"`, namespace))
			}
		}
	}
}

// ── Resiliency Module PrometheusRule (spec.modules[resiliency].metrics.prometheusRule) ──────────

// resiliencyPrometheusRuleName returns the name of the PrometheusRule resource for the resiliency module.
// Named separately from the driver PrometheusRule to coexist without collision.
func resiliencyPrometheusRuleName(crName string) string {
	return crName + "-resiliency-alerts"
}

// buildResiliencyAlertGroups loads the resiliency module PrometheusRule groups from:
//
//	<configDir>/moduleconfig/resiliency/<moduleVersion>/prometheusrule.yaml
//
// The YAML is structured as a list of PrometheusRule groups. Placeholders
// (<DriverName>, <DriverPlatform>, <AlertConnectivitySuccessRatio>) are substituted
// at load time using the driver type and the module-level PrometheusRule config.
func buildResiliencyAlertGroups(configDir, moduleVersion string, driverType csmv1.DriverType, cfg *csmv1.ModulePrometheusRuleConfig) ([]interface{}, error) {
	path := filepath.Join(configDir, "moduleconfig", "resiliency", moduleVersion, "prometheusrule.yaml")
	buf, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("reading resiliency prometheusrule.yaml for version %s: %w", moduleVersion, err)
	}
	yamlStr := applyResiliencyThresholds(string(buf), driverType, cfg)
	return parsePrometheusRuleGroupsYAML(yamlStr)
}

// applyResiliencyThresholds substitutes driver-specific and threshold placeholders
// in the resiliency prometheusrule.yaml template.
func applyResiliencyThresholds(yamlStr string, driverType csmv1.DriverType, cfg *csmv1.ModulePrometheusRuleConfig) string {
	safe := nilSafeModulePrometheusRule(cfg)

	driverName := string(driverType)
	// Normalise: PowerScaleName ("isilon") is the folder name but the metric label uses "csi-powerscale"
	switch driverType {
	case csmv1.PowerScale, csmv1.PowerScaleName:
		driverName = "csi-powerscale"
	case csmv1.PowerStore:
		driverName = "csi-powerstore"
	case csmv1.PowerFlex:
		driverName = "csi-vxflexos"
	case csmv1.PowerMax:
		driverName = "csi-powermax"
	}

	// Platform label is the human-readable storage platform name used in alert labels.
	driverPlatform := string(driverType)
	switch driverType {
	case csmv1.PowerScale, csmv1.PowerScaleName:
		driverPlatform = "powerscale"
	case csmv1.PowerStore:
		driverPlatform = "powerstore"
	case csmv1.PowerFlex:
		driverPlatform = "powerflex"
	case csmv1.PowerMax:
		driverPlatform = "powermax"
	}

	connectivityRatio := int32Val(safe.ConnectivitySuccessRatio, defaultConnectivitySuccessRatio)

	r := strings.NewReplacer(
		"<DriverName>", driverName,
		"<DriverPlatform>", driverPlatform,
		"<AlertConnectivitySuccessRatio>", fmt.Sprintf("%d", connectivityRatio),
	)
	return r.Replace(yamlStr)
}

// nilSafeModulePrometheusRule returns cfg if non-nil, otherwise an empty config (all fields nil/zero).
func nilSafeModulePrometheusRule(cfg *csmv1.ModulePrometheusRuleConfig) *csmv1.ModulePrometheusRuleConfig {
	if cfg != nil {
		return cfg
	}
	return &csmv1.ModulePrometheusRuleConfig{}
}

// replicationPrometheusRuleName returns the name of the replication module PrometheusRule resource
// for a given CR name. It is separate from the driver PrometheusRule (metricsPrometheusRuleName)
// so that the replication rules — which are driver-agnostic (dell_csm_repl_*) — are clearly
// distinguishable from the driver-specific rules.
func replicationPrometheusRuleName(crName string) string {
	return crName + "-replication-alerts"
}

// applyReplicationThresholds replaces <Alert*> placeholders in the CSM Replication module YAML
// with the resolved threshold values and deployment namespace.
func applyReplicationThresholds(yamlStr string, cfg *csmv1.ModulePrometheusRuleConfig, namespace string) string {
	safe := nilSafeModulePrometheusRule(cfg)

	r := strings.NewReplacer(
		"<AlertRPOThresholdSeconds>", fmt.Sprintf("%d", int32Val(safe.RPOThresholdSeconds, defaultRPOThresholdSeconds)),
		"<AlertSRDFMinBandwidth>", fmt.Sprintf("%d", int64Val(safe.SRDFMinBandwidth, defaultSRDFMinBandwidth)),
		"<AlertSRDFLagSeconds>", fmt.Sprintf("%d", int32Val(safe.SRDFLagSeconds, defaultSRDFLagSeconds)),
		"<AlertNamespace>", namespace,
	)
	return r.Replace(yamlStr)
}

// Default authorization threshold values used when the ModulePrometheusRuleConfig field is nil or unset.
// Using integer values to avoid floating-point precision issues.
// Failure rate and quantile are expressed as integers (e.g., 10 for 10%, 95 for 95th percentile)
// and will be divided by 100 when substituted into Prometheus expressions.
const (
	defaultAuthFailureRateThreshold    int32 = 10
	defaultAuthLatencyQuantile         int32 = 95
	defaultAuthLatencyThresholdSeconds int32 = 2
)

// buildAuthorizationAlertRules loads the CSM Authorization PrometheusRule alert rules from the
// resolved module config directory, applies threshold substitutions, and returns the parsed
// rules slice. The module's Metrics.PrometheusRule configuration provides the threshold values.
func buildAuthorizationAlertRules(ctx context.Context, op operatorutils.OperatorConfig, module csmv1.Module, cr csmv1.ContainerStorageModule) ([]interface{}, error) {
	raw, err := modules.ReadAuthorizationModuleConfigFile(ctx, module, cr, op, "prometheusrule.yaml")
	if err != nil {
		return nil, fmt.Errorf("reading authorization prometheusrule.yaml: %w", err)
	}

	var promRuleCfg *csmv1.ModulePrometheusRuleConfig
	if module.Metrics != nil {
		promRuleCfg = module.Metrics.PrometheusRule
	}
	yamlStr := applyAuthorizationThresholds(string(raw), nilSafeModulePrometheusRule(promRuleCfg))
	return parsePrometheusRuleGroupsYAML(yamlStr)
}

// applyAuthorizationThresholds replaces <Alert*> placeholders in the authorization
// prometheusrule.yaml with values from cfg, falling back to defaults.
// Uses integer values to avoid floating-point precision issues.
// Failure rate and quantile are divided by 100 to convert from percentage to decimal.
func applyAuthorizationThresholds(yamlStr string, cfg *csmv1.ModulePrometheusRuleConfig) string {
	failureRate := int32Val(cfg.AuthFailureRateThreshold, defaultAuthFailureRateThreshold)
	latencyQuantile := int32Val(cfg.AuthLatencyQuantile, defaultAuthLatencyQuantile)
	latencySeconds := int32Val(cfg.AuthLatencyThresholdSeconds, defaultAuthLatencyThresholdSeconds)

	// Convert percentage-based values to decimals for Prometheus expressions
	failureRateDecimal := float64(failureRate) / 100.0
	latencyQuantileDecimal := float64(latencyQuantile) / 100.0

	r := strings.NewReplacer(
		"<AlertAuthFailureRateThreshold>", fmt.Sprintf("%.2f", failureRateDecimal),
		"<AlertAuthLatencyQuantile>", fmt.Sprintf("%.2f", latencyQuantileDecimal),
		"<AlertAuthLatencyThresholdSeconds>", fmt.Sprintf("%d", latencySeconds),
	)
	return r.Replace(yamlStr)
}

// loadModulePrometheusRuleGroups reads <configDir>/moduleconfig/<moduleName>/<version>/prometheusrule.yaml,
// applies threshold substitutions via applyThresholds, and returns the parsed groups slice.
// It follows the same pattern as loadPrometheusRuleGroups but targets the moduleconfig directory.
func loadModulePrometheusRuleGroups(
	configDir, moduleName, version string,
	applyThresholds func(string, *csmv1.ModulePrometheusRuleConfig) string,
	cfg *csmv1.ModulePrometheusRuleConfig,
) ([]interface{}, error) {
	path := filepath.Join(configDir, "moduleconfig", moduleName, version, "prometheusrule.yaml")
	buf, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("reading prometheusrule.yaml for module %s/%s: %w", moduleName, version, err)
	}
	yamlStr := applyThresholds(string(buf), cfg)
	return parsePrometheusRuleGroupsYAML(yamlStr)
}

// buildReplicationModuleAlertGroups loads the CSM Replication module PrometheusRule groups
// from <configDir>/moduleconfig/replication/<moduleVersion>/prometheusrule.yaml.
// Threshold placeholders are substituted from cfg (with defaults when nil).
// The YAML defines the group structure (name, interval, rules) — no group wrapping by the caller.
func buildReplicationModuleAlertGroups(configDir, moduleVersion, namespace string, cfg *csmv1.ModulePrometheusRuleConfig) ([]interface{}, error) {
	applyThresholds := func(yamlStr string, cfg *csmv1.ModulePrometheusRuleConfig) string {
		return applyReplicationThresholds(yamlStr, cfg, namespace)
	}
	return loadModulePrometheusRuleGroups(configDir, "replication", moduleVersion, applyThresholds, cfg)
}

// resolveReplicationModuleAlertGroups resolves the replication module config version and loads
// the alert rule groups. It returns nil, false when the replication module is not found in the CR,
// the version cannot be determined, or the YAML cannot be loaded.
//
// Version resolution order:
//  1. m.ConfigVersion (explicit field in the module spec)
//  2. operatorutils.GetModuleDefaultVersion — derives from driver config version + csm-releases.yaml
func resolveReplicationModuleAlertGroups(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig) ([]interface{}, bool) {
	log := logger.GetLogger(ctx)

	moduleVersion, ok := resolveReplicationModuleVersion(ctx, cr, operatorConfig)
	if !ok {
		return nil, false
	}

	var prometheusRuleConfig *csmv1.ModulePrometheusRuleConfig
	for _, module := range cr.Spec.Modules {
		if module.Name == csmv1.Replication && module.Metrics != nil {
			prometheusRuleConfig = module.Metrics.PrometheusRule
			break
		}
	}

	groups, err := buildReplicationModuleAlertGroups(operatorConfig.ConfigDirectory, moduleVersion, cr.Namespace, prometheusRuleConfig)
	if err != nil {
		log.Warnw("Failed to load replication PrometheusRule YAML, skipping", "version", moduleVersion, "error", err)
		return nil, false
	}
	return groups, len(groups) > 0
}

// resolveReplicationModuleVersion finds the config version to use for the replication module.
// It returns the version string and true when resolved successfully, or "", false on failure.
func resolveReplicationModuleVersion(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig) (string, bool) {
	log := logger.GetLogger(ctx)

	for _, m := range cr.Spec.Modules {
		if m.Name != csmv1.Replication {
			continue
		}

		// Use the explicitly-set configVersion when available.
		if m.ConfigVersion != "" {
			return m.ConfigVersion, true
		}

		// Derive from the driver config version via csm-releases.yaml.
		driverConfigVersion := cr.Spec.Driver.ConfigVersion
		if driverConfigVersion == "" {
			var err error
			driverConfigVersion, err = operatorutils.GetVersion(ctx, &cr, operatorConfig)
			if err != nil {
				log.Warnw("Failed to resolve driver config version for replication PrometheusRule", "error", err)
				return "", false
			}
		}

		moduleVersion, err := operatorutils.GetModuleDefaultVersion(driverConfigVersion, cr.GetDriverType(), csmv1.Replication, operatorConfig.ConfigDirectory)
		if err != nil || moduleVersion == "" {
			log.Warnw("Failed to resolve replication module version for PrometheusRule",
				"driverConfigVersion", driverConfigVersion, "error", err)
			return "", false
		}

		return moduleVersion, true
	}

	// Replication module not in the CR.
	return "", false
}

// authorizationPrometheusRuleName returns the name of the PrometheusRule resource for authorization alerts.
func authorizationPrometheusRuleName(crName string) string {
	return crName + "-csm-authorization-alerts"
}
