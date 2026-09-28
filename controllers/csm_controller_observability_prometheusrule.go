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
	"fmt"
	"os"
	"path/filepath"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/operatorutils"
	versionpkg "github.com/dell/csm-operator/pkg/version"
	metav1unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"
)

const (
	observabilityPrometheusRuleName  = "karavi-observability-alerts"
	observabilityPrometheusRuleGroup = "csm-observability-alerts"
)

func syncObservabilityPrometheusRule(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	log := logger.GetLogger(ctx)

	obs, err := findObservabilityModule(cr)
	if err != nil {
		return err
	}

	shouldDelete := isDeleting || obs.Metrics == nil || !obs.Metrics.Enabled || obs.Metrics.PrometheusRule == nil || !obs.Metrics.PrometheusRule.Enabled
	rule := buildObservabilityPrometheusRule(cr, nil)

	if shouldDelete {
		if err := operatorutils.DeleteObject(ctx, rule, ctrlClient); err != nil {
			log.Warnw("Failed to delete observability PrometheusRule (may not exist)", "name", rule.GetName(), "error", err)
		}
		return nil
	}

	rules, err := resolveObservabilityPrometheusRuleAlertRules(ctx, cr, op, obs)
	if err != nil {
		return fmt.Errorf("failed to resolve observability PrometheusRule rules: %w", err)
	}

	rule = buildObservabilityPrometheusRule(cr, rules)
	if err := operatorutils.ApplyObject(ctx, rule, ctrlClient); err != nil {
		return fmt.Errorf("failed to sync observability PrometheusRule %s: %w", rule.GetName(), err)
	}
	log.Infow("Synced observability PrometheusRule", "name", rule.GetName())
	return nil
}

func findObservabilityModule(cr csmv1.ContainerStorageModule) (csmv1.Module, error) {
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Observability {
			return m, nil
		}
	}
	return csmv1.Module{}, fmt.Errorf("could not find observability module")
}

func resolveObservabilityPrometheusRuleAlertRules(ctx context.Context, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig, obs csmv1.Module) ([]interface{}, error) {
	version, err := resolveObservabilityPrometheusRuleVersion(ctx, cr, op, obs)
	if err != nil {
		return nil, err
	}

	rules, err := loadObservabilityPrometheusRuleAlerts(op.ConfigDirectory, version)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("observability prometheusrule.yaml at version %s contains no alert rules", version)
	}
	return rules, nil
}

func resolveObservabilityPrometheusRuleVersion(ctx context.Context, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig, obs csmv1.Module) (string, error) {
	if obs.ConfigVersion != "" {
		return obs.ConfigVersion, nil
	}

	driverConfigVersion, err := operatorutils.GetVersion(ctx, &cr, op)
	if err != nil {
		return "", err
	}

	releasesPath := filepath.Join(op.ConfigDirectory, "common", "csm-releases.yaml")
	info, err := versionpkg.Load(releasesPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", releasesPath, err)
	}

	driverType := cr.GetDriverType()
	if driverType == csmv1.PowerScale {
		driverType = csmv1.PowerScaleName
	}

	moduleVersion := info.ModuleVersion(string(driverType), driverConfigVersion, string(csmv1.Observability))
	if moduleVersion == "" {
		return "", fmt.Errorf("no observability config version is available for %s config version %s", driverType, driverConfigVersion)
	}
	return moduleVersion, nil
}

// parseObservabilityPrometheusRulesYAML unmarshals a YAML list of alert rule maps into []interface{}.
// The sigs.k8s.io/yaml package is used (consistent with the rest of the operator) so that
// nested maps are decoded as map[string]interface{} (JSON semantics).
func parseObservabilityPrometheusRulesYAML(yamlStr string) ([]interface{}, error) {
	var rules []interface{}
	if err := sigsyaml.Unmarshal([]byte(yamlStr), &rules); err != nil {
		return nil, fmt.Errorf("parsing observability prometheusrule YAML: %w", err)
	}
	return rules, nil
}

func loadObservabilityPrometheusRuleAlerts(configDir, version string) ([]interface{}, error) {
	path := filepath.Join(configDir, "moduleconfig", "observability", version, "prometheusrule.yaml")
	buf, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("reading prometheusrule.yaml for observability/%s: %w", version, err)
	}
	return parseObservabilityPrometheusRulesYAML(string(buf))
}

func buildObservabilityPrometheusRule(cr csmv1.ContainerStorageModule, rules []interface{}) *metav1unstructured.Unstructured {
	rule := &metav1unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "monitoring.coreos.com/v1",
		"kind":       "PrometheusRule",
		"metadata": map[string]interface{}{
			"name":      observabilityPrometheusRuleName,
			"namespace": cr.Namespace,
			"labels": map[string]interface{}{
				"app.kubernetes.io/name":       "karavi-observability",
				"app.kubernetes.io/instance":   cr.Name,
				"app.kubernetes.io/managed-by": "csm-operator",
			},
			"ownerReferences": []interface{}{
				map[string]interface{}{
					"apiVersion":         "storage.dell.com/v1",
					"kind":               cr.Kind,
					"name":               cr.Name,
					"uid":                string(cr.GetUID()),
					"controller":         true,
					"blockOwnerDeletion": true,
				},
			},
		},
		"spec": map[string]interface{}{
			"groups": []interface{}{
				map[string]interface{}{
					"name":     observabilityPrometheusRuleGroup,
					"interval": "30s",
					"rules":    rules,
				},
			},
		},
	}}
	rule.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"})
	return rule
}
