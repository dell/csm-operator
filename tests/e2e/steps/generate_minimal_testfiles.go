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

	"github.com/dell/csm-operator/tests/e2e/pkg/version"
)

// latestCSMVersion returns the latest CSM operator version, dynamically
// resolved from csm-version-mapping.yaml via the version package.
// Requires version.Init to have been called first (done in BeforeSuite).
func latestCSMVersion() string {
	if info := version.GetInfo(); info != nil {
		return info.CSMVersion(version.Latest)
	}
	// Should not happen in normal test flow; Init is called in BeforeSuite.
	panic("version.Init has not been called; cannot determine CSM version")
}

// GenerateMinimalTestfiles writes the base minimal testfiles to the given directory.
// Only one file per driver is generated; variant scenarios use InSpec step
// functions to modify the base CR before applying it.
func GenerateMinimalTestfiles(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %v", dir, err)
	}
	for name, content := range minimalTestfiles() {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil { // #nosec G306
			return fmt.Errorf("write %s: %v", p, err)
		}
	}
	return nil
}

// CleanupMinimalTestfiles removes the dynamically generated testfiles.
func CleanupMinimalTestfiles(dir string) {
	for name := range minimalTestfiles() {
		os.Remove(filepath.Join(dir, name))
	}
}

// minimalTestfiles returns a map of filename -> YAML content for every
// base minimal testfile (one per driver). Each base includes all modules
// and components that any variant scenario might need, all disabled by
// default. Variant scenarios use InSpec step functions (e.g.
// enableModuleInSpec, setForceRemoveDriverInSpec) to modify the CR
// before applying it.
//
// Files use spec.version instead of driver.configVersion; the operator
// resolves driver and module versions via csm-version-mapping.yaml.
func minimalTestfiles() map[string]string {
	m := make(map[string]string, 6)

	// Common env block used by all PowerMax files
	pmaxEnvs := `      envs:
        - name: X_CSI_MANAGED_ARRAYS
          value: "REPLACE_ARRAYS"
        - name: X_CSI_POWERMAX_PORTGROUPS
          value: "REPLACE_PORTGROUPS"
        - name: X_CSI_TRANSPORT_PROTOCOL
          value: "REPLACE_PROTOCOL"`

	// ── PowerStore ──────────────────────────────────────────────────────
	// Includes: authorization (with component), resiliency, replication,
	//           observability (with 3 components) — all disabled.
	m["storage_csm_powerstore.yaml"] = fmt.Sprintf(`apiVersion: storage.dell.com/v1
kind: ContainerStorageModule
metadata:
  name: powerstore
  namespace: ${E2E_NS_POWERSTORE}
spec:
  version: %s
  driver:
    # resiliency test will fail with 2 replicas.
    replicas: 1
    csiDriverType: "powerstore"
  modules:
    - name: authorization
      enabled: false
      components:
        - name: karavi-authorization-proxy
    - name: resiliency
      enabled: false
    - name: replication
      enabled: false
    - name: observability
      enabled: false
      components:
        - name: otel-collector
          enabled: false
        - name: cert-manager
          enabled: false
        - name: metrics-powerstore
          enabled: false
`, latestCSMVersion())

	// ── PowerFlex ───────────────────────────────────────────────────────
	// Includes: authorization (with component), resiliency, replication,
	//           observability (with 3 components) — all disabled.
	m["storage_csm_powerflex.yaml"] = fmt.Sprintf(`apiVersion: storage.dell.com/v1
kind: ContainerStorageModule
metadata:
  name: vxflexos
  namespace: ${E2E_NS_POWERFLEX}
spec:
  version: %s
  driver:
    csiDriverType: "powerflex"
    forceRemoveDriver: true
  modules:
    - name: authorization
      enabled: false
      components:
        - name: karavi-authorization-proxy
    - name: resiliency
      enabled: false
    - name: replication
      enabled: false
    - name: observability
      enabled: false
      components:
        - name: otel-collector
          enabled: false
        - name: cert-manager
          enabled: false
        - name: metrics-powerflex
          enabled: false
`, latestCSMVersion())

	// ── PowerScale ──────────────────────────────────────────────────────
	// Includes: authorization (with component), resiliency, replication,
	//           observability (with 3 components) — all disabled.
	m["storage_csm_powerscale.yaml"] = fmt.Sprintf(`apiVersion: storage.dell.com/v1
kind: ContainerStorageModule
metadata:
  name: isilon
  namespace: ${E2E_NS_POWERSCALE}
spec:
  version: %s
  driver:
    csiDriverType: "isilon"
    forceRemoveDriver: true
  modules:
    - name: authorization
      enabled: false
      components:
        - name: karavi-authorization-proxy
    - name: resiliency
      enabled: false
    - name: replication
      enabled: false
    - name: observability
      enabled: false
      components:
        - name: otel-collector
          enabled: false
        - name: cert-manager
          enabled: false
        - name: metrics-powerscale
          enabled: false
`, latestCSMVersion())

	// ── PowerMax ────────────────────────────────────────────────────────
	// Includes: authorization, resiliency, replication,
	//           observability (with 3 components) — all disabled.
	// Keeps common.envs for PowerMax-specific array configuration.
	m["storage_csm_powermax.yaml"] = fmt.Sprintf(`apiVersion: storage.dell.com/v1
kind: ContainerStorageModule
metadata:
  name: powermax
  namespace: ${E2E_NS_POWERMAX}
spec:
  version: %s
  driver:
    csiDriverType: "powermax"
    forceRemoveDriver: true
    common:
%s
  modules:
    - name: authorization
      enabled: false
    - name: resiliency
      enabled: false
    - name: replication
      enabled: false
    - name: observability
      enabled: false
      components:
        - name: otel-collector
          enabled: false
        - name: cert-manager
          enabled: false
        - name: metrics-powermax
          enabled: false
`, latestCSMVersion(), pmaxEnvs)

	// ── Unity ───────────────────────────────────────────────────────────
	m["storage_csm_unity.yaml"] = fmt.Sprintf(`apiVersion: storage.dell.com/v1
kind: ContainerStorageModule
metadata:
  name: unity
  namespace: ${E2E_NS_UNITY}
spec:
  version: %s
  driver:
    csiDriverType: "unity"
    forceRemoveDriver: true
`, latestCSMVersion())

	// ── COSI ────────────────────────────────────────────────────────────
	m["storage_csm_cosi.yaml"] = fmt.Sprintf(`apiVersion: storage.dell.com/v1
kind: ContainerStorageModule
metadata:
  name: cosi
  namespace: ${E2E_NS_COSI}
spec:
  version: %s
  driver:
    csiDriverType: "cosi"
    forceRemoveDriver: true
`, latestCSMVersion())

	return m
}
