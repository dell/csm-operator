// Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	csmv1 "eos2git.cec.lab.emc.com/CSM/csm-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// TestfileSpec describes a single testfile to generate from a sample.
type TestfileSpec struct {
	// OutputFilename is the name of the generated file (e.g., "storage_csm_powerflex.yaml").
	OutputFilename string

	// SamplePath is the path to the source sample relative to samplesBaseDir
	// (e.g., "v2.17.0/storage_csm_powerflex_v2170.yaml").
	SamplePath string

	// Namespace overrides metadata.namespace (e.g., "${E2E_NS_POWERFLEX}").
	Namespace string

	// Name overrides metadata.name. Empty string = keep the sample's name.
	Name string

	// VersionOverride, if non-empty, forces spec.version to this value and
	// clears driver.configVersion + all explicit images. Used when the source
	// sample uses configVersion but the test needs spec.version.
	VersionOverride string

	// EnableModules lists module names to set enabled=true
	// (e.g., ["authorization", "observability"]).
	EnableModules []string

	// EnableComponents maps module name -> list of component names to enable.
	// Components not listed remain at their sample default.
	EnableComponents map[string][]string

	// DriverEnvOverrides overrides env vars in spec.driver.common.envs.
	// Map of env-name -> value.
	DriverEnvOverrides map[string]string

	// ComponentEnvOverrides overrides env vars in module component envs.
	// Map of "moduleName/componentName" -> map of env-name -> value.
	ComponentEnvOverrides map[string]map[string]string

	// RemoveDriverEnvs lists env var names to remove from spec.driver.common.envs.
	// Use this when the sample sets a value but the original testfile omitted it.
	RemoveDriverEnvs []string

	// EnableHealthMonitor, if true, enables the csi-external-health-monitor-controller
	// sidecar and sets X_CSI_HEALTH_MONITOR_ENABLED=true in controller and node envs.
	EnableHealthMonitor bool
}

// On-demand generation state: directories and tracking of generated files.
var (
	genOutputDir   string
	genSamplesDir  string
	genMu          sync.Mutex
	generatedFiles = map[string]bool{}
	specMap        map[string]TestfileSpec // lazily built
)

// InitTestfileGeneration stores the output and samples directories for
// on-demand generation. Call this once in BeforeSuite instead of
// GenerateTestfilesFromSamples.
func InitTestfileGeneration(outputDir, samplesBaseDir string) {
	genOutputDir = outputDir
	genSamplesDir = samplesBaseDir
}

// EnsureTestfileGenerated generates a single test file from its sample if
// the given path corresponds to a registered TestfileSpec and it has not
// already been generated. Safe for concurrent use.
func EnsureTestfileGenerated(filePath string) error {
	basename := filepath.Base(filePath)

	genMu.Lock()
	if generatedFiles[basename] {
		genMu.Unlock()
		return nil
	}
	genMu.Unlock()

	m := testfileSpecMap()
	spec, ok := m[basename]
	if !ok {
		return nil // not a generated file — static file, nothing to do
	}

	if err := generateOne(spec, genOutputDir, genSamplesDir); err != nil {
		return fmt.Errorf("generate %s: %w", basename, err)
	}

	genMu.Lock()
	generatedFiles[basename] = true
	genMu.Unlock()
	return nil
}

// GenerateTestfilesFromSamples reads sample CRs from samplesBaseDir, applies
// per-spec transformations, and writes the results to outputDir.
// Kept for backward compatibility; prefer InitTestfileGeneration +
// EnsureTestfileGenerated for on-demand generation.
func GenerateTestfilesFromSamples(outputDir, samplesBaseDir string) error {
	specs := testfileSpecs()
	for _, s := range specs {
		if err := generateOne(s, outputDir, samplesBaseDir); err != nil {
			return fmt.Errorf("generate %s: %w", s.OutputFilename, err)
		}
	}
	return nil
}

// CleanupGeneratedTestfiles removes generated test files. When on-demand
// generation is used (InitTestfileGeneration), only files that were actually
// generated are removed. When the legacy outputDir overload is used, all
// registered files are removed.
func CleanupGeneratedTestfiles(outputDir string) {
	genMu.Lock()
	defer genMu.Unlock()

	if len(generatedFiles) > 0 {
		dir := genOutputDir
		if dir == "" {
			dir = outputDir
		}
		for filename := range generatedFiles {
			os.Remove(filepath.Join(dir, filename))
		}
		generatedFiles = map[string]bool{}
		return
	}

	// Legacy path: clean all registered specs
	for _, s := range testfileSpecs() {
		os.Remove(filepath.Join(outputDir, s.OutputFilename))
	}
}

// testfileSpecMap returns a map of OutputFilename -> TestfileSpec for fast
// lookup. The map is built once and cached.
func testfileSpecMap() map[string]TestfileSpec {
	if specMap != nil {
		return specMap
	}
	specs := testfileSpecs()
	m := make(map[string]TestfileSpec, len(specs))
	for _, s := range specs {
		m[s.OutputFilename] = s
	}
	specMap = m
	return m
}

// generateOne reads a sample, applies spec transformations, and writes the output YAML.
func generateOne(spec TestfileSpec, outputDir, samplesBaseDir string) error {
	cr, err := readSample(filepath.Join(samplesBaseDir, spec.SamplePath))
	if err != nil {
		return err
	}

	if spec.VersionOverride != "" {
		convertToSpecVersion(&cr, spec.VersionOverride)
	}

	applyOverrides(&cr, spec)

	// Ensure TypeMeta survives round-trip
	cr.APIVersion = "storage.dell.com/v1"
	cr.Kind = "ContainerStorageModule"

	data, err := yaml.Marshal(cr)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	outPath := filepath.Join(outputDir, spec.OutputFilename)
	if err := os.WriteFile(outPath, data, 0o644); err != nil { // #nosec G306
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}

// readSample reads and unmarshals a sample YAML into csmv1.ContainerStorageModule.
func readSample(path string) (csmv1.ContainerStorageModule, error) {
	var cr csmv1.ContainerStorageModule
	raw, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return cr, fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(raw, &cr); err != nil {
		return cr, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	return cr, nil
}

// convertToSpecVersion clears driver.configVersion, sets spec.version,
// strips all explicit image fields, and removes NGINX_PROXY_IMAGE envs
// from module components (forbidden by CEL when spec.version is set).
// Used for v2.15.0 samples.
func convertToSpecVersion(cr *csmv1.ContainerStorageModule, version string) {
	cr.Spec.Version = version
	cr.Spec.Driver.ConfigVersion = ""

	// Strip driver common image
	if cr.Spec.Driver.Common != nil {
		cr.Spec.Driver.Common.Image = ""
	}

	// Strip sidecar images
	for i := range cr.Spec.Driver.SideCars {
		cr.Spec.Driver.SideCars[i].Image = ""
	}

	// Strip initContainer images
	for i := range cr.Spec.Driver.InitContainers {
		cr.Spec.Driver.InitContainers[i].Image = ""
	}

	// Strip module configVersion, component images, and NGINX_PROXY_IMAGE envs
	for i := range cr.Spec.Modules {
		cr.Spec.Modules[i].ConfigVersion = ""
		for j := range cr.Spec.Modules[i].Components {
			cr.Spec.Modules[i].Components[j].Image = ""
			// Remove NGINX_PROXY_IMAGE env
			filtered := make([]corev1.EnvVar, 0, len(cr.Spec.Modules[i].Components[j].Envs))
			for _, env := range cr.Spec.Modules[i].Components[j].Envs {
				if env.Name != "NGINX_PROXY_IMAGE" {
					filtered = append(filtered, env)
				}
			}
			cr.Spec.Modules[i].Components[j].Envs = filtered
		}
		for j := range cr.Spec.Modules[i].InitContainer {
			cr.Spec.Modules[i].InitContainer[j].Image = ""
		}
	}
}

// applyOverrides modifies namespace, name, modules, and env vars per the spec.
func applyOverrides(cr *csmv1.ContainerStorageModule, spec TestfileSpec) {
	if spec.Namespace != "" {
		cr.Namespace = spec.Namespace
	}
	if spec.Name != "" {
		oldName := cr.Name
		cr.Name = spec.Name
		// Update AuthSecret to match the new name, preserving the suffix
		// convention (e.g. "powerstore-config" → "powerstore-config").
		if cr.Spec.Driver.AuthSecret != "" && strings.HasPrefix(cr.Spec.Driver.AuthSecret, oldName) {
			suffix := strings.TrimPrefix(cr.Spec.Driver.AuthSecret, oldName)
			cr.Spec.Driver.AuthSecret = spec.Name + suffix
		}
	}

	// Reset managed fields / status to keep output clean
	cr.ManagedFields = nil
	cr.Status = csmv1.ContainerStorageModuleStatus{}
	cr.ObjectMeta = metav1.ObjectMeta{
		Name:      cr.Name,
		Namespace: cr.Namespace,
	}

	for _, moduleName := range spec.EnableModules {
		enableModule(cr, moduleName)
	}
	for moduleName, components := range spec.EnableComponents {
		for _, compName := range components {
			enableComponent(cr, moduleName, compName)
		}
	}
	for envName, envValue := range spec.DriverEnvOverrides {
		setDriverEnv(cr, envName, envValue)
	}
	for key, envMap := range spec.ComponentEnvOverrides {
		moduleName, componentName := splitModuleComponent(key)
		for envName, envValue := range envMap {
			setComponentEnv(cr, moduleName, componentName, envName, envValue)
		}
	}
	if spec.EnableHealthMonitor {
		applyHealthMonitor(cr)
	}
	if len(spec.RemoveDriverEnvs) > 0 {
		removeDriverEnvs(cr, spec.RemoveDriverEnvs)
	}
}

// removeDriverEnvs removes named env vars from driver.common.envs,
// initContainers, and sideCars.
func removeDriverEnvs(cr *csmv1.ContainerStorageModule, names []string) {
	remove := make(map[string]bool, len(names))
	for _, n := range names {
		remove[n] = true
	}
	filterEnvs := func(envs []corev1.EnvVar) []corev1.EnvVar {
		filtered := make([]corev1.EnvVar, 0, len(envs))
		for _, env := range envs {
			if !remove[env.Name] {
				filtered = append(filtered, env)
			}
		}
		return filtered
	}
	if cr.Spec.Driver.Common != nil {
		cr.Spec.Driver.Common.Envs = filterEnvs(cr.Spec.Driver.Common.Envs)
	}
	for i := range cr.Spec.Driver.InitContainers {
		cr.Spec.Driver.InitContainers[i].Envs = filterEnvs(cr.Spec.Driver.InitContainers[i].Envs)
	}
	for i := range cr.Spec.Driver.SideCars {
		cr.Spec.Driver.SideCars[i].Envs = filterEnvs(cr.Spec.Driver.SideCars[i].Envs)
	}
}

// enableModule sets a module's Enabled field to true.
func enableModule(cr *csmv1.ContainerStorageModule, moduleName string) {
	for i := range cr.Spec.Modules {
		if string(cr.Spec.Modules[i].Name) == moduleName {
			cr.Spec.Modules[i].Enabled = true
			return
		}
	}
}

// enableComponent sets a component's Enabled field to true within a module.
func enableComponent(cr *csmv1.ContainerStorageModule, moduleName, componentName string) {
	for i := range cr.Spec.Modules {
		if string(cr.Spec.Modules[i].Name) == moduleName {
			for j := range cr.Spec.Modules[i].Components {
				if cr.Spec.Modules[i].Components[j].Name == componentName {
					t := true
					cr.Spec.Modules[i].Components[j].Enabled = &t
					return
				}
			}
		}
	}
}

// setDriverEnv sets or adds an env var in driver.common.envs.
func setDriverEnv(cr *csmv1.ContainerStorageModule, name, value string) {
	if cr.Spec.Driver.Common == nil {
		cr.Spec.Driver.Common = &csmv1.ContainerTemplate{}
	}
	for i := range cr.Spec.Driver.Common.Envs {
		if cr.Spec.Driver.Common.Envs[i].Name == name {
			cr.Spec.Driver.Common.Envs[i].Value = value
			return
		}
	}
	cr.Spec.Driver.Common.Envs = append(cr.Spec.Driver.Common.Envs, corev1.EnvVar{Name: name, Value: value})
}

// setComponentEnv sets or adds an env var in a module component's envs.
func setComponentEnv(cr *csmv1.ContainerStorageModule, moduleName, componentName, envName, envValue string) {
	for i := range cr.Spec.Modules {
		if string(cr.Spec.Modules[i].Name) == moduleName {
			for j := range cr.Spec.Modules[i].Components {
				if cr.Spec.Modules[i].Components[j].Name == componentName {
					for k := range cr.Spec.Modules[i].Components[j].Envs {
						if cr.Spec.Modules[i].Components[j].Envs[k].Name == envName {
							cr.Spec.Modules[i].Components[j].Envs[k].Value = envValue
							return
						}
					}
					cr.Spec.Modules[i].Components[j].Envs = append(
						cr.Spec.Modules[i].Components[j].Envs,
						corev1.EnvVar{Name: envName, Value: envValue},
					)
					return
				}
			}
		}
	}
}

// applyHealthMonitor enables the health monitor sidecar and sets health monitor envs.
func applyHealthMonitor(cr *csmv1.ContainerStorageModule) {
	// Enable the csi-external-health-monitor-controller sidecar
	for i := range cr.Spec.Driver.SideCars {
		if cr.Spec.Driver.SideCars[i].Name == "csi-external-health-monitor-controller" ||
			cr.Spec.Driver.SideCars[i].Name == "external-health-monitor" {
			t := true
			cr.Spec.Driver.SideCars[i].Enabled = &t
		}
	}

	// Set X_CSI_HEALTH_MONITOR_ENABLED in controller.envs
	if cr.Spec.Driver.Controller != nil {
		setContainerEnv(&cr.Spec.Driver.Controller.Envs, "X_CSI_HEALTH_MONITOR_ENABLED", "true")
	}

	// Set X_CSI_HEALTH_MONITOR_ENABLED in node.envs
	if cr.Spec.Driver.Node != nil {
		setContainerEnv(&cr.Spec.Driver.Node.Envs, "X_CSI_HEALTH_MONITOR_ENABLED", "true")
	}
}

// setContainerEnv sets or adds an env var in a slice of EnvVar.
func setContainerEnv(envs *[]corev1.EnvVar, name, value string) {
	for i := range *envs {
		if (*envs)[i].Name == name {
			(*envs)[i].Value = value
			return
		}
	}
	*envs = append(*envs, corev1.EnvVar{Name: name, Value: value})
}

// splitModuleComponent splits "moduleName/componentName" into its parts.
func splitModuleComponent(key string) (string, string) {
	for i, c := range key {
		if c == '/' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}

// authProxyHost is the PROXY_HOST value used for authorization test variants.
// The data plane service is auto-created by nginx-gateway-fabric as <ns>-gateway-nginx.
const authProxyHost = "${E2E_NS_AUTH}-gateway-nginx.${E2E_NS_AUTH}.svc.cluster.local"

// authProxyHostN1 is the PROXY_HOST value for n-1 authorization test variants
// that use ingress-nginx (Ingress API) instead of nginx-gateway-fabric (Gateway API).
const authProxyHostN1 = "${E2E_NS_AUTH}-ingress-nginx-controller.${E2E_NS_AUTH}.svc.cluster.local"

// testfileSpecs returns the complete list of 40 TestfileSpec entries.
func testfileSpecs() []TestfileSpec {
	return []TestfileSpec{
		// ── Group 1: Base + Module Variants from v2.17.0 ──

		// PowerFlex (1-7)
		{
			OutputFilename:   "storage_csm_powerflex.yaml",
			SamplePath:       "v2.17.0/storage_csm_powerflex_v2170.yaml",
			Namespace:        "${E2E_NS_POWERFLEX}",
			Name:             "vxflexos",
			RemoveDriverEnvs: []string{"X_CSI_FS_CHECK_ENABLED", "X_CSI_FS_CHECK_MODE", "MDM"},
		},
		{
			OutputFilename: "storage_csm_powerflex_auth.yaml",
			SamplePath:     "v2.17.0/storage_csm_powerflex_v2170.yaml",
			Namespace:      "${E2E_NS_POWERFLEX}",
			Name:           "vxflexos",
			EnableModules:  []string{"authorization"},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": authProxyHost},
			},
		},
		{
			OutputFilename: "storage_csm_powerflex_observability.yaml",
			SamplePath:     "v2.17.0/storage_csm_powerflex_v2170.yaml",
			Namespace:      "${E2E_NS_POWERFLEX}",
			Name:           "vxflexos",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerflex"},
			},
		},
		{
			OutputFilename: "storage_csm_powerflex_observability_auth.yaml",
			SamplePath:     "v2.17.0/storage_csm_powerflex_v2170.yaml",
			Namespace:      "${E2E_NS_POWERFLEX}",
			Name:           "vxflexos",
			EnableModules:  []string{"authorization", "observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerflex"},
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": authProxyHost},
			},
		},
		{
			OutputFilename: "storage_csm_powerflex_replica.yaml",
			SamplePath:     "v2.17.0/storage_csm_powerflex_v2170.yaml",
			Namespace:      "${E2E_NS_POWERFLEX}",
			Name:           "vxflexos",
			EnableModules:  []string{"replication"},
		},
		{
			OutputFilename: "storage_csm_powerflex_resiliency.yaml",
			SamplePath:     "v2.17.0/storage_csm_powerflex_v2170.yaml",
			Namespace:      "${E2E_NS_POWERFLEX}",
			Name:           "vxflexos",
			EnableModules:  []string{"resiliency"},
		},
		{
			OutputFilename:      "storage_csm_powerflex_health_monitor.yaml",
			SamplePath:          "v2.17.0/storage_csm_powerflex_v2170.yaml",
			Namespace:           "${E2E_NS_POWERFLEX}",
			Name:                "powerflex",
			EnableHealthMonitor: true,
		},

		// PowerScale (8-14)
		{
			OutputFilename: "storage_csm_powerscale.yaml",
			SamplePath:     "v2.17.0/storage_csm_powerscale_v2170.yaml",
			Namespace:      "${E2E_NS_POWERSCALE}",
		},
		{
			OutputFilename: "storage_csm_powerscale_auth.yaml",
			SamplePath:     "v2.17.0/storage_csm_powerscale_v2170.yaml",
			Namespace:      "${E2E_NS_POWERSCALE}",
			Name:           "isilon",
			EnableModules:  []string{"authorization"},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": authProxyHost},
			},
		},
		{
			OutputFilename: "storage_csm_powerscale_observability.yaml",
			SamplePath:     "v2.17.0/storage_csm_powerscale_v2170.yaml",
			Namespace:      "${E2E_NS_POWERSCALE}",
			Name:           "isilon",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerscale"},
			},
		},
		{
			OutputFilename: "storage_csm_powerscale_observability_auth.yaml",
			SamplePath:     "v2.17.0/storage_csm_powerscale_v2170.yaml",
			Namespace:      "${E2E_NS_POWERSCALE}",
			Name:           "isilon",
			EnableModules:  []string{"authorization", "observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerscale"},
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": authProxyHost},
			},
		},
		{
			OutputFilename: "storage_csm_powerscale_replica.yaml",
			SamplePath:     "v2.17.0/storage_csm_powerscale_v2170.yaml",
			Namespace:      "${E2E_NS_POWERSCALE}",
			Name:           "isilon",
			EnableModules:  []string{"replication"},
		},
		{
			OutputFilename: "storage_csm_powerscale_resiliency.yaml",
			SamplePath:     "v2.17.0/storage_csm_powerscale_v2170.yaml",
			Namespace:      "${E2E_NS_POWERSCALE}",
			Name:           "isilon",
			EnableModules:  []string{"resiliency"},
		},
		{
			OutputFilename:      "storage_csm_powerscale_health_monitor.yaml",
			SamplePath:          "v2.17.0/storage_csm_powerscale_v2170.yaml",
			Namespace:           "${E2E_NS_OPERATOR}",
			Name:                "powerscale",
			EnableHealthMonitor: true,
		},

		// PowerStore (15-19)
		{
			OutputFilename:   "storage_csm_powerstore.yaml",
			SamplePath:       "v2.17.0/storage_csm_powerstore_v2170.yaml",
			Namespace:        "${E2E_NS_POWERSTORE}",
			Name:             "powerstore",
			RemoveDriverEnvs: []string{"X_CSI_FS_CHECK_ENABLED", "X_CSI_FS_CHECK_MODE"},
		},
		{
			OutputFilename: "storage_csm_powerstore_auth.yaml",
			SamplePath:     "v2.17.0/storage_csm_powerstore_v2170.yaml",
			Namespace:      "${E2E_NS_POWERSTORE}",
			Name:           "powerstore",
			EnableModules:  []string{"authorization"},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": authProxyHost},
			},
		},
		{
			OutputFilename: "storage_csm_powerstore_observability.yaml",
			SamplePath:     "v2.17.0/storage_csm_powerstore_v2170.yaml",
			Namespace:      "${E2E_NS_POWERSTORE}",
			Name:           "powerstore",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerstore"},
			},
		},
		{
			OutputFilename: "storage_csm_powerstore_replication.yaml",
			SamplePath:     "v2.17.0/storage_csm_powerstore_v2170.yaml",
			Namespace:      "${E2E_NS_POWERSTORE}",
			Name:           "powerstore",
			EnableModules:  []string{"replication"},
		},
		{
			OutputFilename: "storage_csm_powerstore_resiliency.yaml",
			SamplePath:     "v2.17.0/storage_csm_powerstore_v2170.yaml",
			Namespace:      "${E2E_NS_POWERSTORE}",
			Name:           "powerstore",
			EnableModules:  []string{"resiliency"},
		},

		// PowerMax (20-24)
		{
			OutputFilename: "storage_csm_powermax.yaml",
			SamplePath:     "v2.17.0/storage_csm_powermax_v2170.yaml",
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "false"},
			},
		},
		{
			OutputFilename: "storage_csm_powermax_authorization.yaml",
			SamplePath:     "v2.17.0/storage_csm_powermax_v2170.yaml",
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			EnableModules:  []string{"authorization"},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": authProxyHost},
			},
		},
		{
			OutputFilename: "storage_csm_powermax_observability.yaml",
			SamplePath:     "v2.17.0/storage_csm_powermax_v2170.yaml",
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powermax"},
			},
		},
		{
			OutputFilename: "storage_csm_powermax_observability_authorization.yaml",
			SamplePath:     "v2.17.0/storage_csm_powermax_v2170.yaml",
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			EnableModules:  []string{"authorization", "observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powermax"},
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": authProxyHost},
			},
		},
		{
			OutputFilename: "storage_csm_powermax_resiliency.yaml",
			SamplePath:     "v2.17.0/storage_csm_powermax_v2170.yaml",
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			EnableModules:  []string{"resiliency"},
		},

		// Unity (25)
		{
			OutputFilename: "storage_csm_unity.yaml",
			SamplePath:     "v2.17.0/storage_csm_unity_v2170.yaml",
			Namespace:      "${E2E_NS_UNITY}",
			Name:           "unity",
		},

		// PowerFlex from v2.16.0 (26-27)
		{
			OutputFilename: "storage_csm_powerflex_downgrade.yaml",
			SamplePath:     "v2.16.0/storage_csm_powerflex_v2161.yaml",
			Namespace:      "${E2E_NS_POWERFLEX}",
			Name:           "vxflexos",
		},
		{
			OutputFilename: "storage_csm_powerflex_auth_n_minus_1.yaml",
			SamplePath:     "v2.16.0/storage_csm_powerflex_v2161.yaml",
			Namespace:      "${E2E_NS_POWERFLEX}",
			Name:           "vxflexos",
			EnableModules:  []string{"authorization"},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": authProxyHostN1},
			},
		},

		// ── Group 2: PowerMax Reverse Proxy Variants (28-33) ──
		{
			OutputFilename: "storage_csm_powermax_tls.yaml",
			SamplePath:     "v2.17.0/storage_csm_powermax_v2170.yaml",
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			DriverEnvOverrides: map[string]string{
				"X_CSI_POWERMAX_SKIP_CERTIFICATE_VALIDATION": "false",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "false"},
			},
		},
		{
			OutputFilename: "storage_csm_powermax_sidecar.yaml",
			SamplePath:     "v2.17.0/storage_csm_powermax_v2170.yaml",
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
		},
		{
			OutputFilename: "storage_csm_powermax_sidecar_tls.yaml",
			SamplePath:     "v2.17.0/storage_csm_powermax_v2170.yaml",
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			DriverEnvOverrides: map[string]string{
				"X_CSI_POWERMAX_SKIP_CERTIFICATE_VALIDATION": "false",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
		},
		{
			OutputFilename: "storage_csm_powermax_secret.yaml",
			SamplePath:     "v2.17.0/storage_csm_powermax_v2170.yaml",
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			DriverEnvOverrides: map[string]string{
				"X_CSI_REVPROXY_USE_SECRET": "true",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "false"},
			},
		},
		{
			OutputFilename: "storage_csm_powermax_secret_sidecar.yaml",
			SamplePath:     "v2.17.0/storage_csm_powermax_v2170.yaml",
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			DriverEnvOverrides: map[string]string{
				"X_CSI_REVPROXY_USE_SECRET": "true",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
		},
		{
			OutputFilename: "storage_csm_powermax_secret_auth_v2.yaml",
			SamplePath:     "v2.17.0/storage_csm_powermax_v2170.yaml",
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			EnableModules:  []string{"authorization"},
			DriverEnvOverrides: map[string]string{
				"X_CSI_REVPROXY_USE_SECRET": "true",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": authProxyHost},
			},
		},

		// ── Group 3: Using-Version Variants (34-37) ──
		{
			OutputFilename: "storage_csm_powerflex_using_version_with_configmap.yaml",
			SamplePath:     "v2.16.0/storage_csm_powerflex_v2161.yaml",
			Namespace:      "${E2E_NS_POWERFLEX}",
			Name:           "vxflexos",
			EnableModules:  []string{"observability", "resiliency"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerflex"},
			},
		},
		{
			OutputFilename: "storage_csm_powermax_using_version_with_configmap.yaml",
			SamplePath:     "v2.16.0/storage_csm_powermax_v2162.yaml",
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			EnableModules:  []string{"replication"},
		},
		{
			OutputFilename:  "storage_csm_powerscale_using_version.yaml",
			SamplePath:      "v2.15.0/storage_csm_powerscale_v2150.yaml",
			Namespace:       "${E2E_NS_POWERSCALE}",
			Name:            "isilon",
			VersionOverride: "v1.15.0",
		},
		{
			OutputFilename:  "storage_csm_powerstore_using_version.yaml",
			SamplePath:      "v2.15.0/storage_csm_powerstore_v2150.yaml",
			Namespace:       "${E2E_NS_POWERSTORE}",
			Name:            "powerstore",
			VersionOverride: "v1.15.0",
		},

		// ── Group 4: Using-Version + Module Variants (38-40) ──
		{
			OutputFilename:  "storage_csm_powerscale_observability_using_version.yaml",
			SamplePath:      "v2.15.0/storage_csm_powerscale_v2150.yaml",
			Namespace:       "${E2E_NS_POWERSCALE}",
			Name:            "isilon",
			VersionOverride: "v1.15.0",
			EnableModules:   []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerscale"},
			},
		},
		{
			OutputFilename:  "storage_csm_powerscale_replica_using_version.yaml",
			SamplePath:      "v2.15.0/storage_csm_powerscale_v2150.yaml",
			Namespace:       "${E2E_NS_POWERSCALE}",
			Name:            "isilon",
			VersionOverride: "v1.15.0",
			EnableModules:   []string{"replication"},
		},
		{
			OutputFilename:  "storage_csm_powerscale_resiliency_using_version.yaml",
			SamplePath:      "v2.15.0/storage_csm_powerscale_v2150.yaml",
			Namespace:       "${E2E_NS_POWERSCALE}",
			Name:            "isilon",
			VersionOverride: "v1.15.0",
			EnableModules:   []string{"resiliency"},
		},
	}
}
