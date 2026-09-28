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

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// TestfileSpec describes a single testfile to generate from a sample.
type TestfileSpec struct {
	// OutputFilename is the name of the generated file (e.g., "storage_csm_powerflex.yaml").
	OutputFilename string

	// SamplePath is the path to the source sample relative to samplesBaseDir
	// (e.g., "<driverVer>/storage_csm_powerflex_<driverVerShort>.yaml").
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

	// Replicas, if non-nil, overrides spec.driver.replicas.
	Replicas *int32

	// EnableMetrics, if true, enables spec.driver.metrics and sets default port.
	EnableMetrics              bool
	EnableDriverPrometheusRule bool

	// MetricsTLSSecret, if non-empty, sets spec.driver.metrics.tlsCertSecret.
	MetricsTLSSecret string

	// EnableObsMetrics, if true, enables the observability module's self-metrics endpoint
	// (modules[observability].metrics) with default port 9090 and a ServiceMonitor.
	EnableObsMetrics bool

	// ObsMetricsTLSSecret, if non-empty, enables TLS for observability module self-metrics and
	// sets modules[observability].metrics.tlsCertSecret.
	ObsMetricsTLSSecret string

	// EnableResiliencyMetrics, if true, enables the resiliency module's metrics endpoint
	// (modules[resiliency].metrics) with default port 8444 and Service/PodMonitor.
	EnableResiliencyMetrics bool

	// ResiliencyMetricsTLSSecret, if non-empty, sets modules[resiliency].metrics.tlsCertSecret.
	ResiliencyMetricsTLSSecret string

	// EnableResiliencyPrometheusRule, if true, enables the resiliency module's PrometheusRule
	// (modules[resiliency].metrics.prometheusRule.enabled = true) and ensures the metrics endpoint
	// is active when the rule is created.
	EnableResiliencyPrometheusRule bool

	// EnableReplicationMetrics, if true, enables the replication module's metrics endpoint
	// (modules[replication].metrics) with default port 8445 and Service/ServiceMonitor/PodMonitor.
	EnableReplicationMetrics bool

	// ReplicationMetricsTLSSecret, if non-empty, sets modules[replication].metrics.tlsCertSecret.
	ReplicationMetricsTLSSecret string

	// GenerateMetricsCSV, if non-empty, generates a CSV validation file with this name
	// (e.g., "powermax_metrics_values.csv") based on the metrics configuration.
	GenerateMetricsCSV string

	// GenerateResiliencyMetricsCSV, if non-empty, generates a CSV validation file with this name
	// (e.g., "powerscale_resiliency_metrics_tls_values.csv") for resiliency podmon env vars.
	GenerateResiliencyMetricsCSV string

	// ImagePullPolicy sets the image pull policy for all containers in the generated CR.
	ImagePullPolicy string

	// UpgradePolicy, if non-empty, sets spec.upgrade to this value (e.g., "auto" or "manual").
	UpgradePolicy string
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
	// Resolve parameterized sample paths (e.g., ${N1_VERSION} -> actual N-1 dir)
	resolvedSamplePath := resolveSamplePathPlaceholders(spec.SamplePath)
	cr, err := readSample(filepath.Join(samplesBaseDir, resolvedSamplePath))
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

	genMu.Lock()
	generatedFiles[spec.OutputFilename] = true
	genMu.Unlock()

	// Generate CSV validation files if specified
	if err := generateMetricsCSV(spec, cr, outputDir); err != nil {
		return fmt.Errorf("generate metrics CSV: %w", err)
	}
	if err := generateResiliencyMetricsCSV(spec, cr, outputDir); err != nil {
		return fmt.Errorf("generate resiliency metrics CSV: %w", err)
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

// lookupDriverVersion returns the driver config version for a given CSM version
// and driver type using the version.Info singleton (initialised in BeforeSuite).
// Returns "" if version info has not been loaded or the lookup fails.
func lookupDriverVersion(driverType, csmVersion string) string {
	if info := version.GetInfo(); info != nil {
		return info.ConfigVersion(driverType, csmVersion)
	}
	return ""
}

// driverSampleDir returns the directory for a driver's canonical sample file
// under the version-root samples/ layout.
// Example: driverSampleDir("powerflex", "v1.18.0") → "v1.18.0/drivers/powerflex"
func driverSampleDir(driverType, csmVersion string) string {
	return fmt.Sprintf("%s/drivers/%s", csmVersion, driverType)
}

// driverSampleFilename returns the filename for a driver's canonical sample file.
// Example: driverSampleFilename("powerflex") → "storage_csm_powerflex.yaml"
func driverSampleFilename(driverType string) string {
	return fmt.Sprintf("storage_csm_%s.yaml", driverType)
}

// samplePath builds the full sample file path for a driver at a given CSM version index.
// Example: samplePath("powerflex", version.Latest) → "v1.18.0/drivers/powerflex/storage_csm_powerflex.yaml"
func samplePath(driverType string, idx int) string {
	info := version.GetInfo()
	if info == nil {
		return ""
	}
	csmVer := info.CSMVersion(idx)
	if csmVer == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", driverSampleDir(driverType, csmVer), driverSampleFilename(driverType))
}

// csmVersionAt returns the CSM operator version at the given index.
// Example: csmVersionAt(version.NMinusTwo) → CSM operator version for N-2
func csmVersionAt(idx int) string {
	info := version.GetInfo()
	if info == nil {
		return ""
	}
	return info.CSMVersion(idx)
}

// resolveSamplePathPlaceholders replaces version placeholders in sample paths
// with the version-root directory for that CSM release.
// Supported placeholders:
// - ${N_VERSION}  -> v1.18.0 for CSM_VERSION_LATEST (N)
// - ${N1_VERSION} -> v1.17.2 for CSM_VERSION_N1 (N-1)
// - ${N2_VERSION} -> v1.16.3 for CSM_VERSION_N2 (N-2)
func resolveSamplePathPlaceholders(path string) string {
	csmLatest := os.Getenv("CSM_VERSION_LATEST")
	csmN1 := os.Getenv("CSM_VERSION_N1")
	csmN2 := os.Getenv("CSM_VERSION_N2")

	result := path
	for _, entry := range []struct {
		placeholder string
		csmVersion  string
	}{
		{"${N_VERSION}", csmLatest},
		{"${N1_VERSION}", csmN1},
		{"${N2_VERSION}", csmN2},
	} {
		if entry.csmVersion == "" {
			continue
		}
		// Version-root layout: callers append drivers/<driver>/storage_csm_<driver>.yaml.
		result = strings.ReplaceAll(result, entry.placeholder, entry.csmVersion)
	}

	return result
}

// convertToSpecVersion clears driver.configVersion, sets spec.version,
// strips all explicit image fields, and removes NGINX_PROXY_IMAGE envs
// from module components (forbidden by CEL when spec.version is set).
// Used for N-1 (and older) samples that lack configVersion.
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
	if spec.Replicas != nil {
		cr.Spec.Driver.Replicas = *spec.Replicas
	}
	if spec.EnableMetrics {
		applyMetrics(cr, spec.MetricsTLSSecret)
	}
	if spec.EnableDriverPrometheusRule {
		if cr.Spec.Driver.Metrics == nil {
			cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{}
		}
		if cr.Spec.Driver.Metrics.PrometheusRule == nil {
			cr.Spec.Driver.Metrics.PrometheusRule = &csmv1.MetricsPrometheusRuleConfig{}
		}
		cr.Spec.Driver.Metrics.PrometheusRule.Enabled = true
	}
	if spec.EnableObsMetrics {
		applyObsModuleMetrics(cr, spec.ObsMetricsTLSSecret)
	}
	if spec.EnableResiliencyMetrics {
		applyResiliencyMetrics(cr, spec.ResiliencyMetricsTLSSecret)
	}
	if spec.EnableResiliencyPrometheusRule {
		applyResiliencyPrometheusRule(cr)
	}
	if spec.EnableReplicationMetrics {
		applyReplicationMetrics(cr, spec.ReplicationMetricsTLSSecret)
	}
	if spec.ImagePullPolicy != "" {
		applyImagePullPolicy(cr, corev1.PullPolicy(spec.ImagePullPolicy))
	}
	if spec.UpgradePolicy != "" {
		cr.Spec.Upgrade = csmv1.UpgradePolicy(spec.UpgradePolicy)
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

// applyMetrics enables driver metrics with full configuration and optionally sets TLS cert secret.
func applyMetrics(cr *csmv1.ContainerStorageModule, tlsSecret string) {
	if cr.Spec.Driver.Metrics == nil {
		cr.Spec.Driver.Metrics = &csmv1.DriverMetrics{}
	}
	cr.Spec.Driver.Metrics.Enabled = true
	if cr.Spec.Driver.Metrics.Port == 0 {
		if cr.Spec.Driver.CSIDriverType == csmv1.PowerFlex {
			cr.Spec.Driver.Metrics.Port = 9090
		} else {
			cr.Spec.Driver.Metrics.Port = 8443
		}
	}

	// Set collection defaults
	if cr.Spec.Driver.Metrics.Collection == nil {
		cr.Spec.Driver.Metrics.Collection = &csmv1.MetricsCollectionConfig{}
	}
	if cr.Spec.Driver.Metrics.Collection.Interval == "" {
		cr.Spec.Driver.Metrics.Collection.Interval = "30s"
	}
	if cr.Spec.Driver.Metrics.Collection.CacheTTL == "" {
		cr.Spec.Driver.Metrics.Collection.CacheTTL = "25s"
	}

	// Set array defaults
	if cr.Spec.Driver.Metrics.Array == nil {
		cr.Spec.Driver.Metrics.Array = &csmv1.MetricsArrayConfig{}
	}
	if cr.Spec.Driver.Metrics.Array.RateLimit == 0 {
		cr.Spec.Driver.Metrics.Array.RateLimit = 100
	}
	if cr.Spec.Driver.Metrics.Array.Timeout == "" {
		cr.Spec.Driver.Metrics.Array.Timeout = "30s"
	}
	if cr.Spec.Driver.Metrics.Array.CircuitBreaker == nil {
		cr.Spec.Driver.Metrics.Array.CircuitBreaker = &csmv1.MetricsCircuitBreakerConfig{}
	}
	if cr.Spec.Driver.Metrics.Array.CircuitBreaker.Threshold == 0 {
		cr.Spec.Driver.Metrics.Array.CircuitBreaker.Threshold = 3
	}
	if cr.Spec.Driver.Metrics.Array.CircuitBreaker.ResetTimeout == "" {
		cr.Spec.Driver.Metrics.Array.CircuitBreaker.ResetTimeout = "30s"
	}

	// Set leader election defaults
	if cr.Spec.Driver.Metrics.LeaderElection == nil {
		cr.Spec.Driver.Metrics.LeaderElection = &csmv1.LeaderElectionConfig{}
	}
	enabled := true
	cr.Spec.Driver.Metrics.LeaderElection.Enabled = &enabled
	if cr.Spec.Driver.Metrics.LeaderElection.LeaseDuration == "" {
		cr.Spec.Driver.Metrics.LeaderElection.LeaseDuration = "60s"
	}
	if cr.Spec.Driver.Metrics.LeaderElection.RenewDeadline == "" {
		cr.Spec.Driver.Metrics.LeaderElection.RenewDeadline = "40s"
	}
	if cr.Spec.Driver.Metrics.LeaderElection.RetryPeriod == "" {
		cr.Spec.Driver.Metrics.LeaderElection.RetryPeriod = "5s"
	}

	cr.Spec.Driver.Metrics.TLSCertSecret = tlsSecret
}

// applyObsModuleMetrics enables the observability module's self-metrics endpoint
// (modules[observability].metrics) with default port 8443 and a ServiceMonitor.
// If tlsSecret is non-empty, TLS is enabled and the port is set to 8444.
func applyObsModuleMetrics(cr *csmv1.ContainerStorageModule, tlsSecret string) {
	for i := range cr.Spec.Modules {
		if string(cr.Spec.Modules[i].Name) == "observability" {
			if cr.Spec.Modules[i].Metrics == nil {
				cr.Spec.Modules[i].Metrics = &csmv1.ModuleMetrics{}
			}
			cr.Spec.Modules[i].Metrics.Enabled = true
			cr.Spec.Modules[i].Metrics.TLSCertSecret = tlsSecret
			if tlsSecret != "" {
				cr.Spec.Modules[i].Metrics.Port = 8444
			} else {
				cr.Spec.Modules[i].Metrics.Port = 8443
			}
			if cr.Spec.Modules[i].Metrics.ServiceMonitor == nil {
				cr.Spec.Modules[i].Metrics.ServiceMonitor = &csmv1.MetricsServiceMonitorConfig{}
			}
			cr.Spec.Modules[i].Metrics.ServiceMonitor.Enabled = true
			if cr.Spec.Modules[i].Metrics.ServiceMonitor.Interval == "" {
				cr.Spec.Modules[i].Metrics.ServiceMonitor.Interval = "30s"
			}
			if tlsSecret != "" {
				cr.Spec.Modules[i].Metrics.ServiceMonitor.InsecureSkipVerify = true
			}
			return
		}
	}
}

// applyResiliencyMetrics enables the resiliency module's metrics endpoint.
func applyResiliencyMetrics(cr *csmv1.ContainerStorageModule, tlsSecret string) {
	for i := range cr.Spec.Modules {
		if string(cr.Spec.Modules[i].Name) != "resiliency" {
			continue
		}
		if cr.Spec.Modules[i].Metrics == nil {
			cr.Spec.Modules[i].Metrics = &csmv1.ModuleMetrics{}
		}
		cr.Spec.Modules[i].Metrics.Enabled = true
		if cr.Spec.Modules[i].Metrics.Port == 0 {
			cr.Spec.Modules[i].Metrics.Port = 8444
		}

		if cr.Spec.Modules[i].Metrics.ServiceMonitor == nil {
			cr.Spec.Modules[i].Metrics.ServiceMonitor = &csmv1.MetricsServiceMonitorConfig{}
		}
		cr.Spec.Modules[i].Metrics.ServiceMonitor.Enabled = true
		if cr.Spec.Modules[i].Metrics.ServiceMonitor.Interval == "" {
			cr.Spec.Modules[i].Metrics.ServiceMonitor.Interval = "30s"
		}

		if cr.Spec.Modules[i].Metrics.PodMonitor == nil {
			cr.Spec.Modules[i].Metrics.PodMonitor = &csmv1.MetricsPodMonitorConfig{}
		}
		cr.Spec.Modules[i].Metrics.PodMonitor.Enabled = true
		if cr.Spec.Modules[i].Metrics.PodMonitor.Interval == "" {
			cr.Spec.Modules[i].Metrics.PodMonitor.Interval = "30s"
		}

		if tlsSecret != "" {
			cr.Spec.Modules[i].Metrics.TLSCertSecret = tlsSecret
			cr.Spec.Modules[i].Metrics.ServiceMonitor.InsecureSkipVerify = true
			cr.Spec.Modules[i].Metrics.PodMonitor.InsecureSkipVerify = true
		}
		return
	}
}

// applyResiliencyPrometheusRule enables the resiliency module's PrometheusRule
// (modules[resiliency].metrics.prometheusRule.enabled = true).
// It also ensures metrics, ServiceMonitor, and PodMonitor are enabled so the alert rule has scrape targets.
func applyResiliencyPrometheusRule(cr *csmv1.ContainerStorageModule) {
	for i := range cr.Spec.Modules {
		if string(cr.Spec.Modules[i].Name) != "resiliency" {
			continue
		}
		if cr.Spec.Modules[i].Metrics == nil {
			cr.Spec.Modules[i].Metrics = &csmv1.ModuleMetrics{}
		}
		// Ensure metrics endpoint is active so the PrometheusRule has a scrape target.
		if !cr.Spec.Modules[i].Metrics.Enabled {
			cr.Spec.Modules[i].Metrics.Enabled = true
		}
		if cr.Spec.Modules[i].Metrics.Port == 0 {
			cr.Spec.Modules[i].Metrics.Port = 8444
		}

		// Enable ServiceMonitor for controller metrics
		if cr.Spec.Modules[i].Metrics.ServiceMonitor == nil {
			cr.Spec.Modules[i].Metrics.ServiceMonitor = &csmv1.MetricsServiceMonitorConfig{}
		}
		cr.Spec.Modules[i].Metrics.ServiceMonitor.Enabled = true
		if cr.Spec.Modules[i].Metrics.ServiceMonitor.Interval == "" {
			cr.Spec.Modules[i].Metrics.ServiceMonitor.Interval = "30s"
		}

		// Enable PodMonitor for node metrics
		if cr.Spec.Modules[i].Metrics.PodMonitor == nil {
			cr.Spec.Modules[i].Metrics.PodMonitor = &csmv1.MetricsPodMonitorConfig{}
		}
		cr.Spec.Modules[i].Metrics.PodMonitor.Enabled = true
		if cr.Spec.Modules[i].Metrics.PodMonitor.Interval == "" {
			cr.Spec.Modules[i].Metrics.PodMonitor.Interval = "30s"
		}

		if cr.Spec.Modules[i].Metrics.PrometheusRule == nil {
			cr.Spec.Modules[i].Metrics.PrometheusRule = &csmv1.ModulePrometheusRuleConfig{}
		}
		cr.Spec.Modules[i].Metrics.PrometheusRule.Enabled = true
		return
	}
}

// applyReplicationMetrics enables the replication module's metrics endpoint.
func applyReplicationMetrics(cr *csmv1.ContainerStorageModule, tlsSecret string) {
	for i := range cr.Spec.Modules {
		if string(cr.Spec.Modules[i].Name) != "replication" {
			continue
		}
		if cr.Spec.Modules[i].Metrics == nil {
			cr.Spec.Modules[i].Metrics = &csmv1.ModuleMetrics{}
		}
		cr.Spec.Modules[i].Metrics.Enabled = true
		if cr.Spec.Modules[i].Metrics.Port == 0 {
			cr.Spec.Modules[i].Metrics.Port = 8445
		}

		if cr.Spec.Modules[i].Metrics.ServiceMonitor == nil {
			cr.Spec.Modules[i].Metrics.ServiceMonitor = &csmv1.MetricsServiceMonitorConfig{}
		}
		cr.Spec.Modules[i].Metrics.ServiceMonitor.Enabled = true
		if cr.Spec.Modules[i].Metrics.ServiceMonitor.Interval == "" {
			cr.Spec.Modules[i].Metrics.ServiceMonitor.Interval = "30s"
		}

		if cr.Spec.Modules[i].Metrics.PodMonitor == nil {
			cr.Spec.Modules[i].Metrics.PodMonitor = &csmv1.MetricsPodMonitorConfig{}
		}
		cr.Spec.Modules[i].Metrics.PodMonitor.Enabled = true
		if cr.Spec.Modules[i].Metrics.PodMonitor.Interval == "" {
			cr.Spec.Modules[i].Metrics.PodMonitor.Interval = "30s"
		}

		if tlsSecret != "" {
			cr.Spec.Modules[i].Metrics.TLSCertSecret = tlsSecret
			cr.Spec.Modules[i].Metrics.ServiceMonitor.InsecureSkipVerify = true
			cr.Spec.Modules[i].Metrics.PodMonitor.InsecureSkipVerify = true
		}
		return
	}
}

// applyImagePullPolicy sets the image pull policy on all driver and module containers.
func applyImagePullPolicy(cr *csmv1.ContainerStorageModule, policy corev1.PullPolicy) {
	setPolicy := func(ct *csmv1.ContainerTemplate) {
		if ct == nil {
			return
		}
		ct.ImagePullPolicy = policy
	}

	setPolicy(cr.Spec.Driver.Common)
	setPolicy(cr.Spec.Driver.Controller)
	setPolicy(cr.Spec.Driver.Node)
	for i := range cr.Spec.Driver.InitContainers {
		setPolicy(&cr.Spec.Driver.InitContainers[i])
	}
	for i := range cr.Spec.Driver.SideCars {
		setPolicy(&cr.Spec.Driver.SideCars[i])
	}
	for i := range cr.Spec.Modules {
		for j := range cr.Spec.Modules[i].Components {
			setPolicy(&cr.Spec.Modules[i].Components[j])
		}
		for j := range cr.Spec.Modules[i].InitContainer {
			setPolicy(&cr.Spec.Modules[i].InitContainer[j])
		}
	}
}

// generateMetricsCSV generates a CSV validation file for metrics configuration.
// The CSV file is used by check_parameters.sh to validate that the generated
// YAML produces the correct environment variables in the pods.
func generateMetricsCSV(spec TestfileSpec, cr csmv1.ContainerStorageModule, outputDir string) error {
	if spec.GenerateMetricsCSV == "" {
		return nil
	}

	if cr.Spec.Driver.Metrics == nil || !cr.Spec.Driver.Metrics.Enabled {
		return fmt.Errorf("metrics not enabled, cannot generate CSV")
	}

	var csvLines []string
	csvLines = append(csvLines, "parameter name, grep option for paramater value, parameter value,resource to describe,number of occurences of parameter name")

	port := cr.Spec.Driver.Metrics.Port
	if port == 0 {
		if cr.Spec.Driver.CSIDriverType == csmv1.PowerFlex {
			port = 9090
		} else {
			port = 8443
		}
	}

	// Common metrics env vars for both controller and node
	csvLines = append(csvLines, "X_CSI_METRICS_ENABLED,,true,controller,1")
	csvLines = append(csvLines, fmt.Sprintf("X_CSI_METRICS_PORT,,%d,controller,1", port))
	csvLines = append(csvLines, "X_CSI_METRICS_ENABLED,,true,node,1")
	csvLines = append(csvLines, fmt.Sprintf("X_CSI_METRICS_PORT,,%d,node,1", port))

	// TLS env vars if TLS secret is set
	if cr.Spec.Driver.Metrics.TLSCertSecret != "" {
		csvLines = append(csvLines, "X_CSI_METRICS_TLS_CERT_FILE,,/etc/metrics-tls/tls.crt,controller,1")
		csvLines = append(csvLines, "X_CSI_METRICS_TLS_KEY_FILE,,/etc/metrics-tls/tls.key,controller,1")
		csvLines = append(csvLines, "X_CSI_METRICS_TLS_CERT_FILE,,/etc/metrics-tls/tls.crt,node,1")
		csvLines = append(csvLines, "X_CSI_METRICS_TLS_KEY_FILE,,/etc/metrics-tls/tls.key,node,1")
	} else if cr.Spec.Driver.CSIDriverType != csmv1.PowerFlex {
		// Collection and array config (controller only, not for TLS-only CSV)
		// PowerFlex metrics only expose the common metrics and gateway-monitoring
		// env vars, so skip the generic collection/array/leader-election checks.
		if cr.Spec.Driver.Metrics.Collection != nil {
			interval := cr.Spec.Driver.Metrics.Collection.Interval
			if interval == "" {
				interval = "30s"
			}
			// PowerStore uses POLL_INTERVAL, others use COLLECTION_INTERVAL
			intervalEnvVar := "X_CSI_METRICS_COLLECTION_INTERVAL"
			if cr.Spec.Driver.CSIDriverType == csmv1.PowerStore {
				intervalEnvVar = "X_CSI_METRICS_POLL_INTERVAL"
			}
			csvLines = append(csvLines, fmt.Sprintf("%s,,%s,controller,1", intervalEnvVar, interval))

			cacheTTL := cr.Spec.Driver.Metrics.Collection.CacheTTL
			if cacheTTL == "" {
				cacheTTL = "25s"
			}
			csvLines = append(csvLines, fmt.Sprintf("X_CSI_METRICS_COLLECTION_CACHE_TTL,,%s,controller,1", cacheTTL))
		}

		if cr.Spec.Driver.Metrics.Array != nil {
			rateLimit := cr.Spec.Driver.Metrics.Array.RateLimit
			if rateLimit == 0 {
				rateLimit = 100
			}
			csvLines = append(csvLines, fmt.Sprintf("X_CSI_METRICS_ARRAY_RATE_LIMIT,,%d,controller,1", rateLimit))

			timeout := cr.Spec.Driver.Metrics.Array.Timeout
			if timeout == "" {
				timeout = "30s"
			}
			csvLines = append(csvLines, fmt.Sprintf("X_CSI_METRICS_ARRAY_TIMEOUT,,%s,controller,1", timeout))

			if cr.Spec.Driver.Metrics.Array.CircuitBreaker != nil {
				threshold := cr.Spec.Driver.Metrics.Array.CircuitBreaker.Threshold
				if threshold == 0 {
					threshold = 3
				}
				csvLines = append(csvLines, fmt.Sprintf("X_CSI_METRICS_ARRAY_CB_THRESHOLD,,%d,controller,1", threshold))

				resetTimeout := cr.Spec.Driver.Metrics.Array.CircuitBreaker.ResetTimeout
				if resetTimeout == "" {
					resetTimeout = "30s"
				}
				csvLines = append(csvLines, fmt.Sprintf("X_CSI_METRICS_ARRAY_CB_RESET_TIMEOUT,,%s,controller,1", resetTimeout))
			}
		}

		// Leader election config
		if cr.Spec.Driver.Metrics.LeaderElection != nil {
			enabled := cr.Spec.Driver.Metrics.LeaderElection.Enabled
			if enabled != nil && *enabled {
				csvLines = append(csvLines, "X_CSI_METRICS_LEADER_ELECTION_ENABLED,,true,controller,1")

				leaseDuration := cr.Spec.Driver.Metrics.LeaderElection.LeaseDuration
				if leaseDuration == "" {
					leaseDuration = "60s"
				}
				csvLines = append(csvLines, fmt.Sprintf("X_CSI_METRICS_LEADER_ELECTION_LEASE_DURATION,,%s,controller,1", leaseDuration))

				renewDeadline := cr.Spec.Driver.Metrics.LeaderElection.RenewDeadline
				if renewDeadline == "" {
					renewDeadline = "40s"
				}
				csvLines = append(csvLines, fmt.Sprintf("X_CSI_METRICS_LEADER_ELECTION_RENEW_DEADLINE,,%s,controller,1", renewDeadline))

				retryPeriod := cr.Spec.Driver.Metrics.LeaderElection.RetryPeriod
				if retryPeriod == "" {
					retryPeriod = "5s"
				}
				csvLines = append(csvLines, fmt.Sprintf("X_CSI_METRICS_LEADER_ELECTION_RETRY_PERIOD,,%s,controller,1", retryPeriod))
			}
		}
	}

	// Write CSV file
	csvPath := filepath.Join(outputDir, spec.GenerateMetricsCSV)
	csvContent := strings.Join(csvLines, "\n") + "\n"
	if err := os.WriteFile(csvPath, []byte(csvContent), 0o644); err != nil {
		return fmt.Errorf("failed to write CSV file %s: %w", csvPath, err)
	}

	genMu.Lock()
	generatedFiles[spec.GenerateMetricsCSV] = true
	genMu.Unlock()

	return nil
}

// generateResiliencyMetricsCSV generates a CSV validation file for resiliency module metrics.
func generateResiliencyMetricsCSV(spec TestfileSpec, cr csmv1.ContainerStorageModule, outputDir string) error {
	if spec.GenerateResiliencyMetricsCSV == "" {
		return nil
	}

	var resiliencyModule *csmv1.Module
	for i := range cr.Spec.Modules {
		if string(cr.Spec.Modules[i].Name) == "resiliency" {
			resiliencyModule = &cr.Spec.Modules[i]
			break
		}
	}
	if resiliencyModule == nil || resiliencyModule.Metrics == nil || !resiliencyModule.Metrics.Enabled {
		return fmt.Errorf("resiliency metrics not enabled, cannot generate CSV")
	}

	port := resiliencyModule.Metrics.Port
	if port == 0 {
		port = 8444
	}

	var csvLines []string
	csvLines = append(csvLines, "parameter,namespace,driver,container,expected_value")

	addRows := func(container string) {
		csvLines = append(csvLines, fmt.Sprintf("X_CSI_METRICS_ENABLED,%s,%s,%s,true", cr.Namespace, cr.Spec.Driver.CSIDriverType, container))
		csvLines = append(csvLines, fmt.Sprintf("X_CSI_METRICS_PORT,%s,%s,%s,%d", cr.Namespace, cr.Spec.Driver.CSIDriverType, container, port))
		if resiliencyModule.Metrics.TLSCertSecret != "" {
			csvLines = append(csvLines, fmt.Sprintf("X_CSI_METRICS_TLS_CERT_FILE,%s,%s,%s,/etc/metrics-tls/tls.crt", cr.Namespace, cr.Spec.Driver.CSIDriverType, container))
			csvLines = append(csvLines, fmt.Sprintf("X_CSI_METRICS_TLS_KEY_FILE,%s,%s,%s,/etc/metrics-tls/tls.key", cr.Namespace, cr.Spec.Driver.CSIDriverType, container))
		}
	}

	addRows("podmon-controller")
	addRows("podmon-node")

	csvPath := filepath.Join(outputDir, spec.GenerateResiliencyMetricsCSV)
	csvContent := strings.Join(csvLines, "\n") + "\n"
	if err := os.WriteFile(csvPath, []byte(csvContent), 0o644); err != nil {
		return fmt.Errorf("failed to write CSV file %s: %w", csvPath, err)
	}

	genMu.Lock()
	generatedFiles[spec.GenerateResiliencyMetricsCSV] = true
	genMu.Unlock()

	return nil
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

func int32Ptr(v int32) *int32 { return &v }

// isOpenShift detects if the cluster is OpenShift by checking environment variables
// or the IS_OPENSHIFT flag which can be set explicitly for testing.
func isOpenShift() bool {
	// Check explicit flag first (for testing)
	if os.Getenv("IS_OPENSHIFT") == "true" {
		return true
	}
	// Check for OpenShift-specific environment variables
	if os.Getenv("OPENSHIFT_BUILD_NAME") != "" {
		return true
	}
	// Check for OpenShift API server
	if strings.Contains(os.Getenv("KUBERNETES_SERVICE_HOST"), "openshift") {
		return true
	}
	return false
}

// getAuthProxyHost returns the appropriate PROXY_HOST based on the platform and version.
// For OpenShift, uses the router internal service.
// For Kubernetes with version < 1.17, uses the ingress-nginx controller service.
// For Kubernetes with version >= 1.17, uses the gateway-nginx service.
func getAuthProxyHost(authNamespace string, versionIdx int) string {
	if isOpenShift() {
		return "router-internal-default.openshift-ingress.svc.cluster.local"
	}
	// Check if version is < 1.17
	if info := version.GetInfo(); info != nil {
		csmVer := info.CSMVersion(versionIdx)
		if csmVer != "" {
			pv, err := version.ParseSemver(csmVer)
			if err == nil {
				// Versions < 1.17 use ingress-nginx-controller
				if pv.IsLessThan(1, 17) {
					return fmt.Sprintf("%s-ingress-nginx-controller.%s.svc.cluster.local", authNamespace, authNamespace)
				}
			}
		}
	}
	// Versions >= 1.17 use gateway-nginx
	return fmt.Sprintf("%s-gateway-nginx.%s.svc.cluster.local", authNamespace, authNamespace)
}

// testfileSpecs returns the complete list of TestfileSpec entries for generated files.
func testfileSpecs() []TestfileSpec {
	return []TestfileSpec{
		// ── Group 1: Base + Module Variants (latest) ──

		// PowerFlex (1–7)
		{
			OutputFilename:   "storage_csm_powerflex.yaml",
			SamplePath:       samplePath("powerflex", version.Latest),
			Namespace:        "${E2E_NS_POWERFLEX}",
			Name:             "vxflexos",
			RemoveDriverEnvs: []string{"X_CSI_FS_CHECK_ENABLED", "X_CSI_FS_CHECK_MODE", "MDM"},
			ImagePullPolicy:  "Always",
		},
		{
			OutputFilename: "storage_csm_powerflex_auth.yaml",
			SamplePath:     samplePath("powerflex", version.Latest),
			Namespace:      "${E2E_NS_POWERFLEX}",
			Name:           "vxflexos",
			EnableModules:  []string{"authorization"},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": getAuthProxyHost("${E2E_NS_AUTH}", version.Latest)},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powerflex_observability.yaml",
			SamplePath:     samplePath("powerflex", version.Latest),
			Namespace:      "${E2E_NS_POWERFLEX}",
			Name:           "vxflexos",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerflex"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powerflex_observability_auth.yaml",
			SamplePath:     samplePath("powerflex", version.Latest),
			Namespace:      "${E2E_NS_POWERFLEX}",
			Name:           "vxflexos",
			EnableModules:  []string{"authorization", "observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerflex"},
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": getAuthProxyHost("${E2E_NS_AUTH}", version.Latest)},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powerflex_observability_metrics.yaml",
			SamplePath:     samplePath("powerflex", version.Latest),
			Namespace:      "${E2E_NS_POWERFLEX}",
			Name:           "vxflexos",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerflex"},
			},
			EnableObsMetrics: true,
			ImagePullPolicy:  "Always",
		},
		{
			OutputFilename: "storage_csm_powerflex_observability_metrics_tls.yaml",
			SamplePath:     samplePath("powerflex", version.Latest),
			Namespace:      "${E2E_NS_POWERFLEX}",
			Name:           "vxflexos",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerflex"},
			},
			EnableObsMetrics:    true,
			ObsMetricsTLSSecret: "powerflex-observability-metrics-tls", // #nosec G101 -- test fixture, not a credential
			ImagePullPolicy:     "Always",
		},
		{
			OutputFilename:  "storage_csm_powerflex_replication.yaml",
			SamplePath:      samplePath("powerflex", version.Latest),
			Namespace:       "${E2E_NS_POWERFLEX}",
			Name:            "vxflexos",
			EnableModules:   []string{"replication"},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:           "storage_csm_powerflex_replication_metrics.yaml",
			SamplePath:               samplePath("powerflex", version.Latest),
			Namespace:                "${E2E_NS_POWERFLEX}",
			Name:                     "vxflexos",
			EnableModules:            []string{"replication"},
			EnableReplicationMetrics: true,
			ImagePullPolicy:          "Always",
		},
		{
			OutputFilename:              "storage_csm_powerflex_replication_metrics_tls.yaml",
			SamplePath:                  samplePath("powerflex", version.Latest),
			Namespace:                   "${E2E_NS_POWERFLEX}",
			Name:                        "vxflexos",
			EnableModules:               []string{"replication"},
			EnableReplicationMetrics:    true,
			ReplicationMetricsTLSSecret: "powerflex-replication-metrics-tls", // #nosec G101
			ImagePullPolicy:             "Always",
		},
		{
			OutputFilename:  "storage_csm_powerflex_resiliency.yaml",
			SamplePath:      samplePath("powerflex", version.Latest),
			Namespace:       "${E2E_NS_POWERFLEX}",
			Name:            "vxflexos",
			EnableModules:   []string{"resiliency"},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:          "storage_csm_powerflex_resiliency_metrics.yaml",
			SamplePath:              samplePath("powerflex", version.Latest),
			Namespace:               "${E2E_NS_POWERFLEX}",
			Name:                    "vxflexos",
			EnableModules:           []string{"resiliency"},
			EnableResiliencyMetrics: true,
			Replicas:                int32Ptr(1),
			ImagePullPolicy:         "Always",
		},
		{
			OutputFilename:               "storage_csm_powerflex_resiliency_metrics_tls.yaml",
			SamplePath:                   samplePath("powerflex", version.Latest),
			Namespace:                    "${E2E_NS_POWERFLEX}",
			Name:                         "vxflexos",
			EnableModules:                []string{"resiliency"},
			EnableResiliencyMetrics:      true,
			ResiliencyMetricsTLSSecret:   "powerflex-metrics-tls",
			GenerateResiliencyMetricsCSV: "powerflex_resiliency_metrics_tls_values.csv",
			Replicas:                     int32Ptr(1),
			ImagePullPolicy:              "Always",
		},
		{
			OutputFilename:                 "storage_csm_powerflex_resiliency_prometheusrule.yaml",
			SamplePath:                     samplePath("powerflex", version.Latest),
			Namespace:                      "${E2E_NS_POWERFLEX}",
			Name:                           "vxflexos",
			EnableModules:                  []string{"resiliency"},
			EnableResiliencyPrometheusRule: true,
			Replicas:                       int32Ptr(1),
			ImagePullPolicy:                "Always",
		},
		{
			OutputFilename:      "storage_csm_powerflex_health_monitor.yaml",
			SamplePath:          samplePath("powerflex", version.Latest),
			Namespace:           "${E2E_NS_POWERFLEX}",
			Name:                "powerflex",
			EnableHealthMonitor: true,
			ImagePullPolicy:     "Always",
		},
		{
			OutputFilename:     "storage_csm_powerflex_metrics.yaml",
			SamplePath:         samplePath("powerflex", version.Latest),
			Namespace:          "${E2E_NS_POWERFLEX}",
			Name:               "vxflexos",
			EnableMetrics:      true,
			GenerateMetricsCSV: "powerflex_metrics_values.csv",
		},
		{
			OutputFilename:     "storage_csm_powerflex_metrics_tls.yaml",
			SamplePath:         samplePath("powerflex", version.Latest),
			Namespace:          "${E2E_NS_POWERFLEX}",
			Name:               "vxflexos",
			EnableMetrics:      true,
			MetricsTLSSecret:   "powerflex-metrics-tls",
			GenerateMetricsCSV: "powerflex_metrics_tls_values.csv",
		},

		// PowerScale (8–14)
		{
			OutputFilename:  "storage_csm_powerscale.yaml",
			SamplePath:      samplePath("powerscale", version.Latest),
			Namespace:       "${E2E_NS_POWERSCALE}",
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powerscale_auth.yaml",
			SamplePath:     samplePath("powerscale", version.Latest),
			Namespace:      "${E2E_NS_POWERSCALE}",
			Name:           "isilon",
			EnableModules:  []string{"authorization"},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": getAuthProxyHost("${E2E_NS_AUTH}", version.Latest)},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powerscale_observability.yaml",
			SamplePath:     samplePath("powerscale", version.Latest),
			Namespace:      "${E2E_NS_POWERSCALE}",
			Name:           "isilon",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerscale"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powerscale_observability_auth.yaml",
			SamplePath:     samplePath("powerscale", version.Latest),
			Namespace:      "${E2E_NS_POWERSCALE}",
			Name:           "isilon",
			EnableModules:  []string{"authorization", "observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerscale"},
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": getAuthProxyHost("${E2E_NS_AUTH}", version.Latest)},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powerscale_observability_prometheusrule_val1.yaml",
			SamplePath:     samplePath("powerscale", version.Latest),
			Namespace:      "${E2E_NS_POWERSCALE}",
			Name:           "isilon",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"metrics-powerscale"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powerscale_observability_prometheusrule_custom_cert.yaml",
			SamplePath:     samplePath("powerscale", version.Latest),
			Namespace:      "${E2E_NS_POWERSCALE}",
			Name:           "isilon",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"metrics-powerscale"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:  "storage_csm_powerscale_replica.yaml",
			SamplePath:      samplePath("powerscale", version.Latest),
			Namespace:       "${E2E_NS_POWERSCALE}",
			Name:            "isilon",
			EnableModules:   []string{"replication"},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:           "storage_csm_powerscale_replication_metrics.yaml",
			SamplePath:               samplePath("powerscale", version.Latest),
			Namespace:                "${E2E_NS_POWERSCALE}",
			Name:                     "isilon",
			EnableModules:            []string{"replication"},
			EnableReplicationMetrics: true,
			ImagePullPolicy:          "Always",
		},
		{
			OutputFilename:              "storage_csm_powerscale_replication_metrics_tls.yaml",
			SamplePath:                  samplePath("powerscale", version.Latest),
			Namespace:                   "${E2E_NS_POWERSCALE}",
			Name:                        "isilon",
			EnableModules:               []string{"replication"},
			EnableReplicationMetrics:    true,
			ReplicationMetricsTLSSecret: "powerscale-replication-metrics-tls", // #nosec G101
			ImagePullPolicy:             "Always",
		},
		{
			OutputFilename:  "storage_csm_powerscale_resiliency.yaml",
			SamplePath:      samplePath("powerscale", version.Latest),
			Namespace:       "${E2E_NS_POWERSCALE}",
			Name:            "isilon",
			EnableModules:   []string{"resiliency"},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:          "storage_csm_powerscale_resiliency_metrics.yaml",
			SamplePath:              samplePath("powerscale", version.Latest),
			Namespace:               "${E2E_NS_POWERSCALE}",
			Name:                    "isilon",
			EnableModules:           []string{"resiliency"},
			EnableResiliencyMetrics: true,
			Replicas:                int32Ptr(1),
			ImagePullPolicy:         "Always",
			DriverEnvOverrides: map[string]string{
				"X_CSI_ISI_NODE_NAME_PREFIX": "csi-node",
				"X_CSI_ISI_API_TIMEOUT":      "120s",
				"CSI_LOG_LEVEL":              "debug",
				"CERT_SECRET_COUNT":          "0",
			},
			RemoveDriverEnvs: []string{
				"X_CSI_VERBOSE",
				"X_CSI_ISI_PORT",
				"X_CSI_ISI_PATH",
				"X_CSI_ISI_NO_PROBE_ON_START",
				"X_CSI_ISI_AUTOPROBE",
				"X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION",
				"CSI_LOG_FORMAT",
				"GOISILON_DEBUG",
				"AZ_RECONCILE_INTERVAL",
				"X_CSI_CUSTOM_TOPOLOGY_ENABLED",
			},
		},
		{
			OutputFilename:               "storage_csm_powerscale_resiliency_metrics_tls.yaml",
			SamplePath:                   samplePath("powerscale", version.Latest),
			Namespace:                    "${E2E_NS_POWERSCALE}",
			Name:                         "isilon",
			EnableModules:                []string{"resiliency"},
			EnableResiliencyMetrics:      true,
			ResiliencyMetricsTLSSecret:   "powerscale-metrics-tls",
			GenerateResiliencyMetricsCSV: "powerscale_resiliency_metrics_tls_values.csv",
			Replicas:                     int32Ptr(1),
			ImagePullPolicy:              "Always",
			DriverEnvOverrides: map[string]string{
				"X_CSI_ISI_NODE_NAME_PREFIX": "csi-node",
				"X_CSI_ISI_API_TIMEOUT":      "120s",
				"CSI_LOG_LEVEL":              "debug",
				"CERT_SECRET_COUNT":          "0",
			},
			RemoveDriverEnvs: []string{
				"X_CSI_VERBOSE",
				"X_CSI_ISI_PORT",
				"X_CSI_ISI_PATH",
				"X_CSI_ISI_NO_PROBE_ON_START",
				"X_CSI_ISI_AUTOPROBE",
				"X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION",
				"CSI_LOG_FORMAT",
				"GOISILON_DEBUG",
				"AZ_RECONCILE_INTERVAL",
				"X_CSI_CUSTOM_TOPOLOGY_ENABLED",
			},
		},
		{
			OutputFilename:                 "storage_csm_powerscale_resiliency_prometheusrule.yaml",
			SamplePath:                     samplePath("powerscale", version.Latest),
			Namespace:                      "${E2E_NS_POWERSCALE}",
			Name:                           "isilon",
			EnableModules:                  []string{"resiliency"},
			EnableResiliencyPrometheusRule: true,
			Replicas:                       int32Ptr(1),
			ImagePullPolicy:                "Always",
		},
		{
			OutputFilename:      "storage_csm_powerscale_health_monitor.yaml",
			SamplePath:          samplePath("powerscale", version.Latest),
			Namespace:           "${E2E_NS_OPERATOR}",
			Name:                "powerscale",
			EnableHealthMonitor: true,
			ImagePullPolicy:     "Always",
		},
		{
			OutputFilename:     "storage_csm_powerscale_metrics.yaml",
			SamplePath:         samplePath("powerscale", version.Latest),
			Namespace:          "${E2E_NS_POWERSCALE}",
			Name:               "isilon",
			EnableMetrics:      true,
			GenerateMetricsCSV: "powerscale_metrics_values.csv",
			ImagePullPolicy:    "Always",
		},
		{
			OutputFilename:     "storage_csm_powerscale_metrics_tls.yaml",
			SamplePath:         samplePath("powerscale", version.Latest),
			Namespace:          "${E2E_NS_POWERSCALE}",
			Name:               "isilon",
			EnableMetrics:      true,
			MetricsTLSSecret:   "powerscale-metrics-tls",
			GenerateMetricsCSV: "powerscale_metrics_tls_values.csv",
			ImagePullPolicy:    "Always",
		},
		{
			OutputFilename: "storage_csm_powerscale_observability_both_metrics.yaml",
			SamplePath:     samplePath("powerscale", version.Latest),
			Namespace:      "${E2E_NS_POWERSCALE}",
			Name:           "powerscale",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerscale"},
			},
			EnableMetrics:    true,
			EnableObsMetrics: true,
			ImagePullPolicy:  "Always",
		},
		{
			OutputFilename: "storage_csm_powerscale_observability_self_metrics.yaml",
			SamplePath:     samplePath("powerscale", version.Latest),
			Namespace:      "${E2E_NS_POWERSCALE}",
			Name:           "powerscale",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerscale"},
			},
			EnableObsMetrics: true,
			Replicas:         int32Ptr(1),
			ImagePullPolicy:  "Always",
			RemoveDriverEnvs: []string{
				"X_CSI_VERBOSE",
				"X_CSI_ISI_PORT",
				"X_CSI_ISI_PATH",
				"X_CSI_ISI_NO_PROBE_ON_START",
				"X_CSI_ISI_AUTOPROBE",
				"X_CSI_ISI_AUTH_TYPE",
				"X_CSI_CUSTOM_TOPOLOGY_ENABLED",
				"KUBELET_CONFIG_DIR",
				"CERT_SECRET_COUNT",
				"CSI_LOG_LEVEL",
				"CSI_LOG_FORMAT",
				"GOISILON_DEBUG",
				"AZ_RECONCILE_INTERVAL",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"observability/metrics-powerscale": {
					"ISICLIENT_AUTH_TYPE": "0",
				},
			},
		},
		{
			OutputFilename: "storage_csm_powerscale_observability_self_metrics_tls.yaml",
			SamplePath:     samplePath("powerscale", version.Latest),
			Namespace:      "${E2E_NS_POWERSCALE}",
			Name:           "powerscale",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerscale"},
			},
			EnableObsMetrics:    true,
			ObsMetricsTLSSecret: "powerscale-metrics-tls",
			Replicas:            int32Ptr(1),
			ImagePullPolicy:     "Always",
			RemoveDriverEnvs: []string{
				"X_CSI_VERBOSE",
				"X_CSI_ISI_PORT",
				"X_CSI_ISI_PATH",
				"X_CSI_ISI_NO_PROBE_ON_START",
				"X_CSI_ISI_AUTOPROBE",
				"X_CSI_ISI_AUTH_TYPE",
				"X_CSI_CUSTOM_TOPOLOGY_ENABLED",
				"KUBELET_CONFIG_DIR",
				"CERT_SECRET_COUNT",
				"CSI_LOG_LEVEL",
				"CSI_LOG_FORMAT",
				"GOISILON_DEBUG",
				"AZ_RECONCILE_INTERVAL",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"observability/metrics-powerscale": {
					"ISICLIENT_AUTH_TYPE": "0",
				},
			},
		},

		// PowerStore (15–19)
		{
			OutputFilename:   "storage_csm_powerstore.yaml",
			SamplePath:       samplePath("powerstore", version.Latest),
			Namespace:        "${E2E_NS_POWERSTORE}",
			Name:             "powerstore",
			RemoveDriverEnvs: []string{"X_CSI_FS_CHECK_ENABLED", "X_CSI_FS_CHECK_MODE"},
			ImagePullPolicy:  "Always",
		},
		{
			OutputFilename: "storage_csm_powerstore_auth.yaml",
			SamplePath:     samplePath("powerstore", version.Latest),
			Namespace:      "${E2E_NS_POWERSTORE}",
			Name:           "powerstore",
			EnableModules:  []string{"authorization"},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": getAuthProxyHost("${E2E_NS_AUTH}", version.Latest)},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powerstore_observability.yaml",
			SamplePath:     samplePath("powerstore", version.Latest),
			Namespace:      "${E2E_NS_POWERSTORE}",
			Name:           "powerstore",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerstore"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:     "storage_csm_powerstore_metrics.yaml",
			SamplePath:         samplePath("powerstore", version.Latest),
			Namespace:          "${E2E_NS_POWERSTORE}",
			Name:               "powerstore",
			RemoveDriverEnvs:   []string{"X_CSI_FS_CHECK_ENABLED", "X_CSI_FS_CHECK_MODE"},
			EnableMetrics:      true,
			GenerateMetricsCSV: "powerstore_metrics_values.csv",
			ImagePullPolicy:    "Always",
		},
		{
			OutputFilename:     "storage_csm_powerstore_metrics_tls.yaml",
			SamplePath:         samplePath("powerstore", version.Latest),
			Namespace:          "${E2E_NS_POWERSTORE}",
			Name:               "powerstore",
			RemoveDriverEnvs:   []string{"X_CSI_FS_CHECK_ENABLED", "X_CSI_FS_CHECK_MODE"},
			EnableMetrics:      true,
			MetricsTLSSecret:   "powerstore-metrics-tls",
			GenerateMetricsCSV: "powerstore_metrics_tls_values.csv",
			ImagePullPolicy:    "Always",
		},
		{
			OutputFilename:  "storage_csm_powerstore_replication.yaml",
			SamplePath:      samplePath("powerstore", version.Latest),
			Namespace:       "${E2E_NS_POWERSTORE}",
			Name:            "powerstore",
			EnableModules:   []string{"replication"},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:           "storage_csm_powerstore_replication_metrics.yaml",
			SamplePath:               samplePath("powerstore", version.Latest),
			Namespace:                "${E2E_NS_POWERSTORE}",
			Name:                     "powerstore",
			EnableModules:            []string{"replication"},
			EnableReplicationMetrics: true,
			ImagePullPolicy:          "Always",
		},
		{
			OutputFilename:              "storage_csm_powerstore_replication_metrics_tls.yaml",
			SamplePath:                  samplePath("powerstore", version.Latest),
			Namespace:                   "${E2E_NS_POWERSTORE}",
			Name:                        "powerstore",
			EnableModules:               []string{"replication"},
			EnableReplicationMetrics:    true,
			ReplicationMetricsTLSSecret: "powerstore-replication-metrics-tls", // #nosec G101
			ImagePullPolicy:             "Always",
		},
		{
			OutputFilename:  "storage_csm_powerstore_resiliency.yaml",
			SamplePath:      samplePath("powerstore", version.Latest),
			Namespace:       "${E2E_NS_POWERSTORE}",
			Name:            "powerstore",
			EnableModules:   []string{"resiliency"},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:          "storage_csm_powerstore_resiliency_metrics.yaml",
			SamplePath:              samplePath("powerstore", version.Latest),
			Namespace:               "${E2E_NS_POWERSTORE}",
			Name:                    "powerstore",
			EnableModules:           []string{"resiliency"},
			EnableResiliencyMetrics: true,
			Replicas:                int32Ptr(1),
			ImagePullPolicy:         "Always",
			DriverEnvOverrides: map[string]string{
				"CSI_LOG_LEVEL":     "debug",
				"CERT_SECRET_COUNT": "0",
			},
			RemoveDriverEnvs: []string{
				"X_CSI_FS_CHECK_ENABLED",
				"X_CSI_FS_CHECK_MODE",
				"GOPOWERSTORE_DEBUG",
				"X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION",
				"X_CSI_PODMON_ARRAY_CONNECTIVITY_TIMEOUT",
				"CSI_LOG_FORMAT",
				"AZ_RECONCILE_INTERVAL",
				"X_CSI_CUSTOM_TOPOLOGY_ENABLED",
			},
		},
		{
			OutputFilename:               "storage_csm_powerstore_resiliency_metrics_tls.yaml",
			SamplePath:                   samplePath("powerstore", version.Latest),
			Namespace:                    "${E2E_NS_POWERSTORE}",
			Name:                         "powerstore",
			EnableModules:                []string{"resiliency"},
			EnableResiliencyMetrics:      true,
			ResiliencyMetricsTLSSecret:   "powerstore-metrics-tls",
			GenerateResiliencyMetricsCSV: "powerstore_resiliency_metrics_tls_values.csv",
			Replicas:                     int32Ptr(1),
			ImagePullPolicy:              "Always",
			DriverEnvOverrides: map[string]string{
				"CSI_LOG_LEVEL":     "debug",
				"CERT_SECRET_COUNT": "0",
			},
			RemoveDriverEnvs: []string{
				"X_CSI_FS_CHECK_ENABLED",
				"X_CSI_FS_CHECK_MODE",
				"GOPOWERSTORE_DEBUG",
				"X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION",
				"X_CSI_PODMON_ARRAY_CONNECTIVITY_TIMEOUT",
				"CSI_LOG_FORMAT",
				"AZ_RECONCILE_INTERVAL",
				"X_CSI_CUSTOM_TOPOLOGY_ENABLED",
			},
		},
		{
			OutputFilename:                 "storage_csm_powerstore_resiliency_prometheusrule.yaml",
			SamplePath:                     samplePath("powerstore", version.Latest),
			Namespace:                      "${E2E_NS_POWERSTORE}",
			Name:                           "powerstore",
			EnableModules:                  []string{"resiliency"},
			EnableResiliencyPrometheusRule: true,
			Replicas:                       int32Ptr(1),
			ImagePullPolicy:                "Always",
		},
		{
			OutputFilename: "storage_csm_powerstore_observability_self_metrics.yaml",
			SamplePath:     samplePath("powerstore", version.Latest),
			Namespace:      "${E2E_NS_POWERSTORE}",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerstore"},
			},
			EnableObsMetrics: true,
			Replicas:         int32Ptr(1),
			ImagePullPolicy:  "Always",
			DriverEnvOverrides: map[string]string{
				"CSI_LOG_LEVEL": "debug",
			},
			RemoveDriverEnvs: []string{
				"GOPOWERSTORE_DEBUG",
				"CERT_SECRET_COUNT",
				"X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION",
				"X_CSI_PODMON_ARRAY_CONNECTIVITY_TIMEOUT",
				"CSI_LOG_FORMAT",
				"AZ_RECONCILE_INTERVAL",
				"X_CSI_CUSTOM_TOPOLOGY_ENABLED",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"observability/metrics-powerstore": {
					"POWERSTORE_TOPOLOGY_POLL_FREQUENCY": "10",
					"X_CSI_POWERSTORE_API_TIMEOUT":       "120s",
				},
			},
		},
		{
			OutputFilename: "storage_csm_powerstore_observability_self_metrics_tls.yaml",
			SamplePath:     samplePath("powerstore", version.Latest),
			Namespace:      "${E2E_NS_POWERSTORE}",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerstore"},
			},
			EnableObsMetrics:    true,
			ObsMetricsTLSSecret: "powerstore-metrics-tls",
			Replicas:            int32Ptr(1),
			ImagePullPolicy:     "Always",
			DriverEnvOverrides: map[string]string{
				"CSI_LOG_LEVEL": "debug",
			},
			RemoveDriverEnvs: []string{
				"GOPOWERSTORE_DEBUG",
				"CERT_SECRET_COUNT",
				"X_CSI_POWERSTORE_SKIP_CERTIFICATE_VALIDATION",
				"X_CSI_PODMON_ARRAY_CONNECTIVITY_TIMEOUT",
				"CSI_LOG_FORMAT",
				"AZ_RECONCILE_INTERVAL",
				"X_CSI_CUSTOM_TOPOLOGY_ENABLED",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"observability/metrics-powerstore": {
					"POWERSTORE_TOPOLOGY_POLL_FREQUENCY": "10",
					"X_CSI_POWERSTORE_API_TIMEOUT":       "120s",
				},
			},
		},
		{
			OutputFilename: "storage_csm_powerstore_csiaddons.yaml",
			SamplePath:     samplePath("powerstore", version.Latest),
			Namespace:      "${E2E_NS_POWERSTORE}",
			Name:           "powerstore",
			DriverEnvOverrides: map[string]string{
				"X_CSI_CSIADDONS_REPLICATION_ENABLED": "true",
			},
			ImagePullPolicy: "Always",
		},

		// PowerMax (20–25)
		{
			OutputFilename: "storage_csm_powermax.yaml",
			SamplePath:     samplePath("powermax", version.Latest),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powermax_authorization.yaml",
			SamplePath:     samplePath("powermax", version.Latest),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			EnableModules:  []string{"authorization"},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": getAuthProxyHost("${E2E_NS_AUTH}", version.Latest)},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powermax_observability.yaml",
			SamplePath:     samplePath("powermax", version.Latest),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powermax"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powermax_observability_n_minus_1.yaml",
			SamplePath:     samplePath("powermax", version.NMinusOne),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			Replicas:       int32Ptr(1),
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powermax"},
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powermax_observability_n_minus_2.yaml",
			SamplePath:     samplePath("powermax", version.NMinusTwo),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			Replicas:       int32Ptr(1),
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powermax"},
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powermax_observability_authorization.yaml",
			SamplePath:     samplePath("powermax", version.Latest),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			EnableModules:  []string{"authorization", "observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powermax"},
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": getAuthProxyHost("${E2E_NS_AUTH}", version.Latest)},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powermax_observability_self_metrics.yaml",
			SamplePath:     samplePath("powermax", version.Latest),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powermax"},
			},
			EnableObsMetrics: true,
			ImagePullPolicy:  "Always",
			DriverEnvOverrides: map[string]string{
				"X_CSI_POWERMAX_SKIP_CERTIFICATE_VALIDATION": "true",
			},
			RemoveDriverEnvs: []string{
				"X_CSI_MANAGED_ARRAYS",
				"X_CSI_POWERMAX_PORTGROUPS",
				"X_CSI_TRANSPORT_PROTOCOL",
				"X_CSI_K8S_CLUSTER_PREFIX",
				"KUBELET_CONFIG_DIR",
				"X_CSI_VSPHERE_ENABLED",
				"X_CSI_VSPHERE_PORTGROUP",
				"X_CSI_VSPHERE_HOSTNAME",
				"X_CSI_VCENTER_HOST",
				"CSI_LOG_LEVEL",
				"CSI_LOG_FORMAT",
				"X_CSI_POWERMAX_DEBUG",
				"X_CSI_POWERMAX_SKIP_CERTIFICATE_VALIDATION",
				"X_CSI_REVPROXY_USE_SECRET",
				"X_CSI_DYNAMIC_SG_ENABLED",
				"X_CSI_FS_CHECK_ENABLED",
				"X_CSI_FS_CHECK_MODE",
				"X_CSI_SPACE_RECLAMATION_ENABLED",
				"X_CSI_SPACE_RECLAMATION_SCHEDULE",
				"X_CSI_SPACE_RECLAMATION_MAX_CONCURRENT",
				"X_CSI_SPACE_RECLAMATION_TIMEOUT",
				"X_CSI_CSIADDONS_REPLICATION_ENABLED",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {
					"DeployAsSidecar": "true",
				},
				"observability/metrics-powermax": {
					"POWERMAX_MAX_CONCURRENT_QUERIES":          "10",
					"POWERMAX_CAPACITY_METRICS_ENABLED":        "true",
					"POWERMAX_PERFORMANCE_METRICS_ENABLED":     "true",
					"POWERMAX_TOPOLOGY_METRICS_ENABLED":        "true",
					"POWERMAX_TOPOLOGY_METRICS_POLL_FREQUENCY": "30",
					"POWERMAX_CAPACITY_POLL_FREQUENCY":         "20",
					"POWERMAX_PERFORMANCE_POLL_FREQUENCY":      "20",
					"POWERMAX_LOG_LEVEL":                       "INFO",
					"POWERMAX_LOG_FORMAT":                      "TEXT",
					"COLLECTOR_ADDRESS":                        "otel-collector:55680",
					"X_CSI_CONFIG_MAP_NAME":                    "powermax-reverseproxy-config",
				},
			},
		},
		{
			OutputFilename: "storage_csm_powermax_observability_self_metrics_tls.yaml",
			SamplePath:     samplePath("powermax", version.Latest),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			EnableModules:  []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powermax"},
			},
			EnableObsMetrics:    true,
			ObsMetricsTLSSecret: "powermax-metrics-tls",
			ImagePullPolicy:     "Always",
			DriverEnvOverrides: map[string]string{
				"X_CSI_POWERMAX_SKIP_CERTIFICATE_VALIDATION": "true",
			},
			RemoveDriverEnvs: []string{
				"X_CSI_MANAGED_ARRAYS",
				"X_CSI_POWERMAX_PORTGROUPS",
				"X_CSI_TRANSPORT_PROTOCOL",
				"X_CSI_K8S_CLUSTER_PREFIX",
				"KUBELET_CONFIG_DIR",
				"X_CSI_VSPHERE_ENABLED",
				"X_CSI_VSPHERE_PORTGROUP",
				"X_CSI_VSPHERE_HOSTNAME",
				"X_CSI_VCENTER_HOST",
				"CSI_LOG_LEVEL",
				"CSI_LOG_FORMAT",
				"X_CSI_POWERMAX_DEBUG",
				"X_CSI_POWERMAX_SKIP_CERTIFICATE_VALIDATION",
				"X_CSI_REVPROXY_USE_SECRET",
				"X_CSI_DYNAMIC_SG_ENABLED",
				"X_CSI_FS_CHECK_ENABLED",
				"X_CSI_FS_CHECK_MODE",
				"X_CSI_SPACE_RECLAMATION_ENABLED",
				"X_CSI_SPACE_RECLAMATION_SCHEDULE",
				"X_CSI_SPACE_RECLAMATION_MAX_CONCURRENT",
				"X_CSI_SPACE_RECLAMATION_TIMEOUT",
				"X_CSI_CSIADDONS_REPLICATION_ENABLED",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {
					"DeployAsSidecar": "true",
				},
				"observability/metrics-powermax": {
					"POWERMAX_MAX_CONCURRENT_QUERIES":          "10",
					"POWERMAX_CAPACITY_METRICS_ENABLED":        "true",
					"POWERMAX_PERFORMANCE_METRICS_ENABLED":     "true",
					"POWERMAX_TOPOLOGY_METRICS_ENABLED":        "true",
					"POWERMAX_TOPOLOGY_METRICS_POLL_FREQUENCY": "30",
					"POWERMAX_CAPACITY_POLL_FREQUENCY":         "20",
					"POWERMAX_PERFORMANCE_POLL_FREQUENCY":      "20",
					"POWERMAX_LOG_LEVEL":                       "INFO",
					"POWERMAX_LOG_FORMAT":                      "TEXT",
					"COLLECTOR_ADDRESS":                        "otel-collector:55680",
					"X_CSI_CONFIG_MAP_NAME":                    "powermax-reverseproxy-config",
				},
			},
		},
		{
			OutputFilename:  "storage_csm_powermax_resiliency.yaml",
			SamplePath:      samplePath("powermax", version.Latest),
			Namespace:       "${E2E_NS_POWERMAX}",
			Name:            "powermax",
			EnableModules:   []string{"resiliency"},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:          "storage_csm_powermax_resiliency_metrics.yaml",
			SamplePath:              samplePath("powermax", version.Latest),
			Namespace:               "${E2E_NS_POWERMAX}",
			Name:                    "powermax",
			EnableModules:           []string{"resiliency"},
			EnableResiliencyMetrics: true,
			Replicas:                int32Ptr(1),
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
		},
		{
			OutputFilename:               "storage_csm_powermax_resiliency_metrics_tls.yaml",
			SamplePath:                   samplePath("powermax", version.Latest),
			Namespace:                    "${E2E_NS_POWERMAX}",
			Name:                         "powermax",
			EnableModules:                []string{"resiliency"},
			EnableResiliencyMetrics:      true,
			ResiliencyMetricsTLSSecret:   "powermax-metrics-tls",
			GenerateResiliencyMetricsCSV: "powermax_resiliency_metrics_tls_values.csv",
			Replicas:                     int32Ptr(1),
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
		},
		{
			OutputFilename:                 "storage_csm_powermax_resiliency_prometheusrule.yaml",
			SamplePath:                     samplePath("powermax", version.Latest),
			Namespace:                      "${E2E_NS_POWERMAX}",
			Name:                           "powermax",
			EnableModules:                  []string{"resiliency"},
			EnableResiliencyPrometheusRule: true,
			Replicas:                       int32Ptr(1),
			ImagePullPolicy:                "Always",
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
		},
		{
			OutputFilename: "storage_csm_powermax_csiaddons.yaml",
			SamplePath:     samplePath("powermax", version.Latest),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			DriverEnvOverrides: map[string]string{
				"X_CSI_CSIADDONS_REPLICATION_ENABLED": "true",
				"X_CSI_REVPROXY_USE_SECRET":           "true",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:             "storage_csm_powermax_metrics.yaml",
			SamplePath:                 samplePath("powermax", version.Latest),
			Namespace:                  "${E2E_NS_POWERMAX}",
			Name:                       "powermax",
			EnableMetrics:              true,
			EnableDriverPrometheusRule: true,
			GenerateMetricsCSV:         "powermax_metrics_values.csv",
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:     "storage_csm_powermax_metrics_tls.yaml",
			SamplePath:         samplePath("powermax", version.Latest),
			Namespace:          "${E2E_NS_POWERMAX}",
			Name:               "powermax",
			EnableMetrics:      true,
			MetricsTLSSecret:   "powermax-metrics-tls",
			GenerateMetricsCSV: "powermax_metrics_tls_values.csv",
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
			ImagePullPolicy: "Always",
		},

		// Unity (26)
		{
			OutputFilename:  "storage_csm_unity.yaml",
			SamplePath:      samplePath("unity", version.Latest),
			Namespace:       "${E2E_NS_UNITY}",
			Name:            "unity",
			ImagePullPolicy: "Always",
		},

		// PowerFlex n-2 and n-1 variants (27–28)
		{
			OutputFilename:  "storage_csm_powerflex_downgrade.yaml",
			SamplePath:      samplePath("powerflex", version.NMinusTwo),
			Namespace:       "${E2E_NS_POWERFLEX}",
			Name:            "vxflexos",
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powerflex_auth_n_minus_1.yaml",
			SamplePath:     samplePath("powerflex", version.NMinusOne),
			Namespace:      "${E2E_NS_POWERFLEX}",
			Name:           "vxflexos",
			EnableModules:  []string{"authorization"},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": getAuthProxyHost("${E2E_NS_AUTH}", version.NMinusOne)},
			},
			ImagePullPolicy: "Always",
		},

		// ── Group 2: PowerMax Reverse Proxy Variants (29–34) ──
		{
			OutputFilename: "storage_csm_powermax_tls.yaml",
			SamplePath:     samplePath("powermax", version.Latest),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			DriverEnvOverrides: map[string]string{
				"X_CSI_POWERMAX_SKIP_CERTIFICATE_VALIDATION": "false",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "false"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powermax_sidecar.yaml",
			SamplePath:     samplePath("powermax", version.Latest),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powermax_sidecar_tls.yaml",
			SamplePath:     samplePath("powermax", version.Latest),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			DriverEnvOverrides: map[string]string{
				"X_CSI_POWERMAX_SKIP_CERTIFICATE_VALIDATION": "false",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powermax_secret.yaml",
			SamplePath:     samplePath("powermax", version.Latest),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			DriverEnvOverrides: map[string]string{
				"X_CSI_REVPROXY_USE_SECRET": "true",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "false"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powermax_secret_sidecar.yaml",
			SamplePath:     samplePath("powermax", version.Latest),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			DriverEnvOverrides: map[string]string{
				"X_CSI_REVPROXY_USE_SECRET": "true",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powermax_secret_auth_v2.yaml",
			SamplePath:     samplePath("powermax", version.Latest),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			EnableModules:  []string{"authorization"},
			DriverEnvOverrides: map[string]string{
				"X_CSI_REVPROXY_USE_SECRET": "true",
			},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": getAuthProxyHost("${E2E_NS_AUTH}", version.Latest)},
			},
			ImagePullPolicy: "Always",
		},

		// ── Group 3: Using-Version Variants (35–38) ──
		{
			OutputFilename: "storage_csm_powerflex_using_version_with_configmap.yaml",
			SamplePath:     samplePath("powerflex", version.NMinusTwo),
			Namespace:      "${E2E_NS_POWERFLEX}",
			Name:           "vxflexos",
			EnableModules:  []string{"observability", "resiliency"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerflex"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:  "storage_csm_powermax_using_version_with_configmap.yaml",
			SamplePath:      samplePath("powermax", version.NMinusTwo),
			Namespace:       "${E2E_NS_POWERMAX}",
			Name:            "powermax",
			EnableModules:   []string{"replication"},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:  "storage_csm_powerscale_using_version.yaml",
			SamplePath:      samplePath("powerscale", version.NMinusTwo),
			Namespace:       "${E2E_NS_POWERSCALE}",
			Name:            "isilon",
			VersionOverride: csmVersionAt(version.NMinusTwo),
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:  "storage_csm_powerstore_using_version.yaml",
			SamplePath:      samplePath("powerstore", version.NMinusTwo),
			Namespace:       "${E2E_NS_POWERSTORE}",
			Name:            "powerstore",
			VersionOverride: csmVersionAt(version.NMinusTwo),
			ImagePullPolicy: "Always",
		},

		// ── Group 4: Using-Version + Module Variants (39–41) ──
		{
			OutputFilename:  "storage_csm_powerscale_observability_using_version.yaml",
			SamplePath:      samplePath("powerscale", version.NMinusTwo),
			Namespace:       "${E2E_NS_POWERSCALE}",
			Name:            "isilon",
			VersionOverride: csmVersionAt(version.NMinusTwo),
			EnableModules:   []string{"observability"},
			EnableComponents: map[string][]string{
				"observability": {"otel-collector", "cert-manager", "metrics-powerscale"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:  "storage_csm_powerscale_replica_using_version.yaml",
			SamplePath:      samplePath("powerscale", version.NMinusTwo),
			Namespace:       "${E2E_NS_POWERSCALE}",
			Name:            "isilon",
			VersionOverride: csmVersionAt(version.NMinusTwo),
			EnableModules:   []string{"replication"},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:  "storage_csm_powerscale_resiliency_using_version.yaml",
			SamplePath:      samplePath("powerscale", version.NMinusTwo),
			Namespace:       "${E2E_NS_POWERSCALE}",
			Name:            "isilon",
			VersionOverride: csmVersionAt(version.NMinusTwo),
			EnableModules:   []string{"resiliency"},
			ImagePullPolicy: "Always",
		},

		// ── Group 5: Upgrade/Rollback Driver Variants (42–55) ──
		// n-1 and n-2 versions for upgrade/rollback testing. All use
		// spec.version (no configVersion) so images resolve via the configMap.

		{
			OutputFilename:   "storage_csm_powerstore_n_minus_1.yaml",
			SamplePath:       samplePath("powerstore", version.NMinusOne),
			Namespace:        "${E2E_NS_POWERSTORE}",
			Name:             "powerstore",
			Replicas:         int32Ptr(1),
			RemoveDriverEnvs: []string{"X_CSI_FS_CHECK_ENABLED", "X_CSI_FS_CHECK_MODE"},
			ImagePullPolicy:  "Always",
		},
		{
			OutputFilename:   "storage_csm_powerstore_replication_n_minus_1.yaml",
			SamplePath:       samplePath("powerstore", version.NMinusOne),
			Namespace:        "${E2E_NS_POWERSTORE}",
			Name:             "powerstore",
			Replicas:         int32Ptr(1),
			EnableModules:    []string{"replication"},
			RemoveDriverEnvs: []string{"X_CSI_FS_CHECK_ENABLED", "X_CSI_FS_CHECK_MODE"},
			ImagePullPolicy:  "Always",
		},
		{
			OutputFilename:   "storage_csm_powerstore_auth_n_minus_1.yaml",
			SamplePath:       samplePath("powerstore", version.NMinusOne),
			Namespace:        "${E2E_NS_POWERSTORE}",
			Name:             "powerstore",
			Replicas:         int32Ptr(1),
			EnableModules:    []string{"authorization"},
			RemoveDriverEnvs: []string{"X_CSI_FS_CHECK_ENABLED", "X_CSI_FS_CHECK_MODE"},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": getAuthProxyHost("${E2E_NS_AUTH}", version.NMinusOne)},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:   "storage_csm_powerstore_replication_n_minus_2.yaml",
			SamplePath:       samplePath("powerstore", version.NMinusTwo),
			Namespace:        "${E2E_NS_POWERSTORE}",
			Name:             "powerstore",
			Replicas:         int32Ptr(1),
			EnableModules:    []string{"replication"},
			RemoveDriverEnvs: []string{"X_CSI_FS_CHECK_ENABLED", "X_CSI_FS_CHECK_MODE"},
			ImagePullPolicy:  "Always",
		},
		{
			OutputFilename:   "storage_csm_powerstore_auth_n_minus_2.yaml",
			SamplePath:       samplePath("powerstore", version.NMinusTwo),
			Namespace:        "${E2E_NS_POWERSTORE}",
			Name:             "powerstore",
			Replicas:         int32Ptr(1),
			EnableModules:    []string{"authorization"},
			RemoveDriverEnvs: []string{"X_CSI_FS_CHECK_ENABLED", "X_CSI_FS_CHECK_MODE"},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": getAuthProxyHost("${E2E_NS_AUTH}", version.NMinusTwo)},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:   "storage_csm_powerstore_n_minus_2.yaml",
			SamplePath:       samplePath("powerstore", version.NMinusTwo),
			Namespace:        "${E2E_NS_POWERSTORE}",
			Name:             "powerstore",
			Replicas:         int32Ptr(1),
			RemoveDriverEnvs: []string{"X_CSI_FS_CHECK_ENABLED", "X_CSI_FS_CHECK_MODE"},
			ImagePullPolicy:  "Always",
		},
		{
			OutputFilename:  "storage_csm_powerflex_n_minus_1.yaml",
			SamplePath:      samplePath("powerflex", version.NMinusOne),
			Namespace:       "${E2E_NS_POWERFLEX}",
			Name:            "vxflexos",
			Replicas:        int32Ptr(1),
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:  "storage_csm_powerflex_n_minus_2.yaml",
			SamplePath:      samplePath("powerflex", version.NMinusTwo),
			Namespace:       "${E2E_NS_POWERFLEX}",
			Name:            "vxflexos",
			VersionOverride: csmVersionAt(version.NMinusTwo),
			Replicas:        int32Ptr(1),
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:  "storage_csm_powerscale_n_minus_1.yaml",
			SamplePath:      samplePath("powerscale", version.NMinusOne),
			Namespace:       "${E2E_NS_POWERSCALE}",
			Name:            "isilon",
			Replicas:        int32Ptr(1),
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:  "storage_csm_powerscale_n_minus_2.yaml",
			SamplePath:      samplePath("powerscale", version.NMinusTwo),
			Namespace:       "${E2E_NS_POWERSCALE}",
			Name:            "isilon",
			Replicas:        int32Ptr(1),
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powerscale_auth_n_minus_1.yaml",
			SamplePath:     samplePath("powerscale", version.NMinusOne),
			Namespace:      "${E2E_NS_POWERSCALE}",
			Name:           "isilon",
			Replicas:       int32Ptr(1),
			EnableModules:  []string{"authorization"},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": getAuthProxyHost("${E2E_NS_AUTH}", version.NMinusOne)},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powerscale_auth_n_minus_2.yaml",
			SamplePath:     samplePath("powerscale", version.NMinusTwo),
			Namespace:      "${E2E_NS_POWERSCALE}",
			Name:           "isilon",
			Replicas:       int32Ptr(1),
			EnableModules:  []string{"authorization"},
			ComponentEnvOverrides: map[string]map[string]string{
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": getAuthProxyHost("${E2E_NS_AUTH}", version.NMinusTwo)},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:  "storage_csm_powerscale_replication_n_minus_1.yaml",
			SamplePath:      samplePath("powerscale", version.NMinusOne),
			Namespace:       "${E2E_NS_POWERSCALE}",
			Name:            "isilon",
			Replicas:        int32Ptr(1),
			EnableModules:   []string{"replication"},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:  "storage_csm_powerscale_replication_n_minus_2.yaml",
			SamplePath:      samplePath("powerscale", version.NMinusTwo),
			Namespace:       "${E2E_NS_POWERSCALE}",
			Name:            "isilon",
			Replicas:        int32Ptr(1),
			EnableModules:   []string{"replication"},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powermax_n_minus_1.yaml",
			SamplePath:     samplePath("powermax", version.NMinusOne),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			Replicas:       int32Ptr(1),
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:  "storage_csm_powermax_n_minus_2.yaml",
			SamplePath:      samplePath("powermax", version.NMinusTwo),
			Namespace:       "${E2E_NS_POWERMAX}",
			Name:            "powermax",
			VersionOverride: csmVersionAt(version.NMinusTwo),
			Replicas:        int32Ptr(1),
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powermax_authorization_n_minus_2.yaml",
			SamplePath:     samplePath("powermax", version.NMinusTwo),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			Replicas:       int32Ptr(1),
			EnableModules:  []string{"authorization"},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": getAuthProxyHost("${E2E_NS_AUTH}", version.NMinusTwo)},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename: "storage_csm_powermax_authorization_n_minus_1.yaml",
			SamplePath:     samplePath("powermax", version.NMinusOne),
			Namespace:      "${E2E_NS_POWERMAX}",
			Name:           "powermax",
			Replicas:       int32Ptr(1),
			EnableModules:  []string{"authorization"},
			ComponentEnvOverrides: map[string]map[string]string{
				"csireverseproxy/csipowermax-reverseproxy": {"DeployAsSidecar": "true"},
				"authorization/karavi-authorization-proxy": {"PROXY_HOST": getAuthProxyHost("${E2E_NS_AUTH}", version.NMinusOne)},
			},
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:  "storage_csm_unity_n_minus_1.yaml",
			SamplePath:      samplePath("unity", version.NMinusOne),
			Namespace:       "${E2E_NS_UNITY}",
			Name:            "unity",
			Replicas:        int32Ptr(1),
			ImagePullPolicy: "Always",
		},
		{
			OutputFilename:  "storage_csm_unity_n_minus_2.yaml",
			SamplePath:      samplePath("unity", version.NMinusTwo),
			Namespace:       "${E2E_NS_UNITY}",
			Name:            "unity",
			Replicas:        int32Ptr(1),
			ImagePullPolicy: "Always",
		},

		// ── Group 6: Upgrade/Rollback Module Variants (56–57) ──
		// PowerFlex + Resiliency n-1/n-2 for upgrade/rollback testing.
		// [ECS01G-1108] [ECS01G-1114]

		// PowerFlex n-1 + Resiliency
		{
			OutputFilename:  "storage_csm_powerflex_resiliency_n_minus_1.yaml",
			SamplePath:      samplePath("powerflex", version.NMinusOne),
			Namespace:       "${E2E_NS_POWERFLEX}",
			Name:            "vxflexos",
			Replicas:        int32Ptr(1),
			EnableModules:   []string{"resiliency"},
			ImagePullPolicy: "Always",
		},
		// PowerFlex n-2 + Resiliency
		{
			OutputFilename:  "storage_csm_powerflex_resiliency_n_minus_2.yaml",
			SamplePath:      samplePath("powerflex", version.NMinusTwo),
			Namespace:       "${E2E_NS_POWERFLEX}",
			Name:            "vxflexos",
			Replicas:        int32Ptr(1),
			EnableModules:   []string{"resiliency"},
			ImagePullPolicy: "Always",
		},

		// ── Group 7: Auto-Upgrade Variants ──
		// n-1 and n-2 versions with upgrade policy set for auto-upgrade testing.

		// PowerFlex n-1 with upgrade=manual (deploy first, then switch to auto)
		{
			OutputFilename:  "storage_csm_powerflex_auto_upgrade_n_minus_1.yaml",
			SamplePath:      samplePath("powerflex", version.NMinusOne),
			Namespace:       "${E2E_NS_POWERFLEX}",
			Name:            "vxflexos",
			Replicas:        int32Ptr(1),
			ImagePullPolicy: "Always",
			UpgradePolicy:   "manual",
		},
		// PowerFlex n-2 with upgrade=manual (deploy first, then switch to auto)
		{
			OutputFilename:  "storage_csm_powerflex_auto_upgrade_n_minus_2.yaml",
			SamplePath:      samplePath("powerflex", version.NMinusTwo),
			Namespace:       "${E2E_NS_POWERFLEX}",
			Name:            "vxflexos",
			VersionOverride: csmVersionAt(version.NMinusTwo),
			Replicas:        int32Ptr(1),
			ImagePullPolicy: "Always",
			UpgradePolicy:   "manual",
		},
		// PowerFlex n-1 with upgrade=auto (deploy with auto immediately)
		{
			OutputFilename:  "storage_csm_powerflex_auto_upgrade_immediate_n_minus_1.yaml",
			SamplePath:      samplePath("powerflex", version.NMinusOne),
			Namespace:       "${E2E_NS_POWERFLEX}",
			Name:            "vxflexos",
			Replicas:        int32Ptr(1),
			ImagePullPolicy: "Always",
			UpgradePolicy:   "auto",
		},
	}
}
