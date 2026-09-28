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
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/version"
	"sigs.k8s.io/yaml"
)

func TestStorageClassDeleteNotFoundIsIgnored(t *testing.T) {
	if !isStorageClassDeleteNotFound(fmt.Errorf("Error from server (NotFound): storageclasses.storage.k8s.io \\\"isilon-mtls\\\" not found")) {
		t.Fatal("expected a Kubernetes NotFound delete error to be ignored")
	}
	if isStorageClassDeleteNotFound(fmt.Errorf("delete storageclass: connection refused")) {
		t.Fatal("expected a non-NotFound delete error to be returned")
	}
}

func TestNormalizePowerStoreEndpointHost(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "IPv4", input: "192.0.2.1", want: "192.0.2.1"},
		{name: "bare IPv6", input: "2001:db8::1", want: "[2001:db8::1]"},
		{name: "bracketed IPv6", input: "[2001:db8::1]", want: "[2001:db8::1]"},
		{name: "link-local zone", input: "fe80::1%eth0", want: "[fe80::1%25eth0]"},
		{name: "FQDN", input: "powerstore.example.com", want: "powerstore.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizePowerStoreEndpointHost(tt.input); got != tt.want {
				t.Fatalf("normalizePowerStoreEndpointHost(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateTestfilesFromSamples(t *testing.T) {
	tmpDir := t.TempDir()
	samplesDir := "../../../samples"

	// Verify samples directory exists
	if _, err := os.Stat(samplesDir); os.IsNotExist(err) {
		t.Skipf("samples directory not found at %s; skipping", samplesDir)
	}

	// Initialise the version package (same releases file used by e2e BeforeSuite)
	// so that CSM_VERSION_* env vars resolve correctly.
	releasesFile := "../../../operatorconfig/common/csm-releases.yaml"
	if _, err := os.Stat(releasesFile); os.IsNotExist(err) {
		t.Skipf("csm-releases.yaml not found at %s; skipping", releasesFile)
	}
	version.ResetForTest()
	if err := version.Init(releasesFile); err != nil {
		t.Fatalf("version.Init: %v", err)
	}
	vInfo := version.GetInfo()
	t.Setenv("CSM_VERSION_LATEST", vInfo.CSMVersion(version.Latest))
	t.Setenv("CSM_VERSION_N1", vInfo.CSMVersion(version.NMinusOne))
	t.Setenv("CSM_VERSION_N2", vInfo.CSMVersion(version.NMinusTwo))

	// Generate all testfiles
	if err := GenerateTestfilesFromSamples(tmpDir, samplesDir); err != nil {
		t.Fatalf("GenerateTestfilesFromSamples failed: %v", err)
	}

	specs := testfileSpecs()

	for _, spec := range specs {
		path := filepath.Join(tmpDir, spec.OutputFilename)

		// File must exist
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("file %s not found: %v", spec.OutputFilename, err)
			continue
		}

		// Must unmarshal into ContainerStorageModule
		var cr csmv1.ContainerStorageModule
		if err := yaml.Unmarshal(data, &cr); err != nil {
			t.Errorf("file %s failed to unmarshal: %v", spec.OutputFilename, err)
			continue
		}

		// Check namespace
		if spec.Namespace != "" && cr.Namespace != spec.Namespace {
			t.Errorf("file %s: expected namespace %q, got %q", spec.OutputFilename, spec.Namespace, cr.Namespace)
		}

		// Check name
		if spec.Name != "" && cr.Name != spec.Name {
			t.Errorf("file %s: expected name %q, got %q", spec.OutputFilename, spec.Name, cr.Name)
		}

		// Check TypeMeta
		if cr.APIVersion != "storage.dell.com/v1" {
			t.Errorf("file %s: expected apiVersion storage.dell.com/v1, got %q", spec.OutputFilename, cr.APIVersion)
		}
		if cr.Kind != "ContainerStorageModule" {
			t.Errorf("file %s: expected kind ContainerStorageModule, got %q", spec.OutputFilename, cr.Kind)
		}
		if spec.OutputFilename == "storage_csm_powermax_metrics.yaml" &&
			(cr.Spec.Driver.Metrics == nil || cr.Spec.Driver.Metrics.PrometheusRule == nil || !cr.Spec.Driver.Metrics.PrometheusRule.Enabled) {
			t.Errorf("file %s: driver metrics PrometheusRule should be enabled", spec.OutputFilename)
		}

		// Check enabled modules
		for _, moduleName := range spec.EnableModules {
			found := false
			for _, m := range cr.Spec.Modules {
				if string(m.Name) == moduleName {
					found = true
					if !m.Enabled {
						t.Errorf("file %s: module %q should be enabled", spec.OutputFilename, moduleName)
					}
					break
				}
			}
			if !found {
				t.Errorf("file %s: module %q not found in CR", spec.OutputFilename, moduleName)
			}
		}

		// CRD-compatibility check: if spec.version is set, no images or configVersion
		if cr.Spec.Version != "" {
			if cr.Spec.Driver.ConfigVersion != "" {
				t.Errorf("file %s: driver.configVersion must be empty when spec.version is set, got %q",
					spec.OutputFilename, cr.Spec.Driver.ConfigVersion)
			}
			if cr.Spec.Driver.Common != nil && cr.Spec.Driver.Common.Image != "" {
				t.Errorf("file %s: driver.common.image must be empty when spec.version is set, got %q",
					spec.OutputFilename, cr.Spec.Driver.Common.Image)
			}
			for _, sc := range cr.Spec.Driver.SideCars {
				if sc.Image != "" {
					t.Errorf("file %s: sidecar %q image must be empty when spec.version is set, got %q",
						spec.OutputFilename, sc.Name, sc.Image)
				}
			}
			for _, ic := range cr.Spec.Driver.InitContainers {
				if ic.Image != "" {
					t.Errorf("file %s: initContainer %q image must be empty when spec.version is set, got %q",
						spec.OutputFilename, ic.Name, ic.Image)
				}
			}
			for _, m := range cr.Spec.Modules {
				for _, c := range m.Components {
					if c.Image != "" {
						t.Errorf("file %s: module %q component %q image must be empty when spec.version is set, got %q",
							spec.OutputFilename, m.Name, c.Name, c.Image)
					}
					for _, env := range c.Envs {
						if env.Name == "NGINX_PROXY_IMAGE" {
							t.Errorf("file %s: module %q component %q has forbidden NGINX_PROXY_IMAGE env when spec.version is set",
								spec.OutputFilename, m.Name, c.Name)
						}
					}
				}
			}
		}
	}

	// Test cleanup
	CleanupGeneratedTestfiles(tmpDir)
	for _, spec := range specs {
		path := filepath.Join(tmpDir, spec.OutputFilename)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("file %s should have been cleaned up", spec.OutputFilename)
		}
	}
}
