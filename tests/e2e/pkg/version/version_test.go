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

package version

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projectRoot returns the project root directory by locating go.mod.
// Walks up until finding go.mod that's not in tests/e2e directory.
func projectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for {
		// Check if we found go.mod
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// Make sure we're not in the e2e directory
			if !strings.Contains(dir, "/tests/e2e") {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("could not find project root (go.mod)")
		}
		dir = parent
	}
}

func TestLoad(t *testing.T) {
	mappingPath := filepath.Join(projectRoot(), "operatorconfig/common/csm-version-mapping.yaml")
	info, err := Load(mappingPath)
	require.NoError(t, err)
	require.NotNil(t, info)

	// Should have at least 3 minor releases (Latest, N-1, N-2).
	assert.GreaterOrEqual(t, len(info.CSMVersions), 3,
		"expected at least 3 CSM minor-release versions")

	// Versions should be sorted newest-first.
	for i := 0; i < len(info.CSMVersions)-1; i++ {
		cur, _ := parseSemver(info.CSMVersions[i])
		next, _ := parseSemver(info.CSMVersions[i+1])
		assert.Greater(t, cur.minor, next.minor,
			"CSMVersions should be sorted by minor version descending: %s before %s",
			info.CSMVersions[i], info.CSMVersions[i+1])
	}
}

func TestCSMVersion(t *testing.T) {
	mappingPath := filepath.Join(projectRoot(), "operatorconfig/common/csm-version-mapping.yaml")
	info, err := Load(mappingPath)
	require.NoError(t, err)

	latest := info.CSMVersion(Latest)
	assert.NotEmpty(t, latest, "Latest CSM version should not be empty")

	n1 := info.CSMVersion(NMinusOne)
	assert.NotEmpty(t, n1, "N-1 CSM version should not be empty")

	n2 := info.CSMVersion(NMinusTwo)
	assert.NotEmpty(t, n2, "N-2 CSM version should not be empty")

	// Out-of-range index returns empty.
	assert.Empty(t, info.CSMVersion(999))
	assert.Empty(t, info.CSMVersion(-1))
}

func TestConfigVersion(t *testing.T) {
	mappingPath := filepath.Join(projectRoot(), "operatorconfig/common/csm-version-mapping.yaml")
	info, err := Load(mappingPath)
	require.NoError(t, err)

	latest := info.CSMVersion(Latest)

	// Every driver should have a config version for the latest CSM version.
	for _, driver := range []string{"powerflex", "powermax", "powerscale", "powerstore", "unity"} {
		cv := info.ConfigVersion(driver, latest)
		assert.NotEmpty(t, cv, "config version for %s at %s should not be empty", driver, latest)
	}

	// Unknown entity returns empty.
	assert.Empty(t, info.ConfigVersion("nonexistent", latest))
}

func TestConfigVersionAtIndex(t *testing.T) {
	mappingPath := filepath.Join(projectRoot(), "operatorconfig/common/csm-version-mapping.yaml")
	info, err := Load(mappingPath)
	require.NoError(t, err)

	for _, driver := range []string{"powerflex", "powermax", "powerscale", "powerstore"} {
		cv := info.ConfigVersionAtIndex(driver, NMinusOne)
		assert.NotEmpty(t, cv, "config version at N-1 for %s should not be empty", driver)
	}

	// Out-of-range index.
	assert.Empty(t, info.ConfigVersionAtIndex("powerflex", 999))
}

func TestModuleVersions(t *testing.T) {
	mappingPath := filepath.Join(projectRoot(), "operatorconfig/common/csm-version-mapping.yaml")
	modulePath := filepath.Join(projectRoot(), "operatorconfig/moduleconfig/common/version-values.yaml")
	info, err := Load(mappingPath)
	require.NoError(t, err)
	require.NoError(t, info.LoadModuleVersions(modulePath))

	// powerflex latest (v2.17.0) should have authorization module version.
	authVer := info.ModuleVersionAtIndex("powerflex", "authorization", Latest)
	assert.NotEmpty(t, authVer, "authorization module version for powerflex at latest")
	t.Logf("powerflex/authorization at latest: %s", authVer)

	// powermax n-1 should have csireverseproxy module version.
	rpVer := info.ModuleVersionAtIndex("powermax", "csireverseproxy", NMinusOne)
	assert.NotEmpty(t, rpVer, "csireverseproxy module version for powermax at n-1")
	t.Logf("powermax/csireverseproxy at n-1: %s", rpVer)

	// Each driver at latest should have resiliency version.
	for _, driver := range []string{"powerflex", "powermax", "powerscale", "powerstore"} {
		rv := info.ModuleVersionAtIndex(driver, "resiliency", Latest)
		assert.NotEmpty(t, rv, "resiliency version for %s at latest", driver)
	}

	// Unknown module returns empty.
	assert.Empty(t, info.ModuleVersionAtIndex("powerflex", "nonexistent", Latest))
}

func TestExpandPathTokens(t *testing.T) {
	mappingPath := filepath.Join(projectRoot(), "operatorconfig/common/csm-version-mapping.yaml")
	info, err := Load(mappingPath)
	require.NoError(t, err)

	authLatest := info.ConfigVersionAtIndex("authorization-proxy-server", Latest)
	authN1 := info.ConfigVersionAtIndex("authorization-proxy-server", NMinusOne)
	require.NotEmpty(t, authLatest)
	require.NotEmpty(t, authN1)

	tests := []struct {
		input, expected string
	}{
		{
			filepath.Join(projectRoot(), "operatorconfig/moduleconfig/authorization/{authorization-proxy-server}/authorization-crds.yaml"),
			filepath.Join(projectRoot(), "operatorconfig/moduleconfig/authorization/"+authLatest+"/authorization-crds.yaml"),
		},
		{
			filepath.Join(projectRoot(), "operatorconfig/moduleconfig/authorization/{authorization-proxy-server:n-1}/authorization-crds.yaml"),
			filepath.Join(projectRoot(), "operatorconfig/moduleconfig/authorization/"+authN1+"/authorization-crds.yaml"),
		},
		{
			"no-tokens-here.yaml",
			"no-tokens-here.yaml",
		},
		{
			"{nonexistent-entity}/file.yaml",
			"{nonexistent-entity}/file.yaml", // unresolvable token stays unchanged
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, info.ExpandPathTokens(tt.input))
		})
	}
}

func TestLoadInvalidPath(t *testing.T) {
	_, err := Load("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func TestLoadInvalidYAML(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "bad.yaml")
	err := os.WriteFile(tmp, []byte("not: [valid: yaml: mapping"), 0o644)
	require.NoError(t, err)

	_, err = Load(tmp)
	assert.Error(t, err)
}

func TestVersionResolutionSummary(t *testing.T) {
	mappingPath := filepath.Join(projectRoot(), "operatorconfig/common/csm-version-mapping.yaml")
	info, err := Load(mappingPath)
	require.NoError(t, err)

	t.Log("CSM versions (highest patch per minor, newest-first):")
	for i, v := range info.CSMVersions {
		label := ""
		switch i {
		case Latest:
			label = " ← Latest"
		case NMinusOne:
			label = " ← n-1"
		case NMinusTwo:
			label = " ← n-2"
		}
		t.Logf("  [%d] %s%s", i, v, label)
	}

	// Verify that each index picks the highest patch within its minor group.
	for i, v := range info.CSMVersions {
		pv, err := parseSemver(v)
		require.NoError(t, err)
		// Check no other version with the same minor has a higher patch.
		for otherV := range collectAllCSMVersions(info) {
			opv, err := parseSemver(otherV)
			if err != nil {
				continue
			}
			if opv.major == pv.major && opv.minor == pv.minor {
				assert.GreaterOrEqual(t, pv.patch, opv.patch,
					"index %d (%s) should have highest patch for minor %d, but found %s",
					i, v, pv.minor, otherV)
			}
		}
	}

	drivers := []string{"powerflex", "powermax", "powerscale", "powerstore", "unity"}
	for _, d := range drivers {
		t.Logf("%s: latest=%s  n-1=%s  n-2=%s", d,
			info.ConfigVersionAtIndex(d, Latest),
			info.ConfigVersionAtIndex(d, NMinusOne),
			info.ConfigVersionAtIndex(d, NMinusTwo))
	}
}

// collectAllCSMVersions returns all CSM versions across all entities.
func collectAllCSMVersions(info *Info) map[string]bool {
	all := map[string]bool{}
	for _, m := range info.configVersions {
		for v := range m {
			all[v] = true
		}
	}
	return all
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
		major   int
		minor   int
		patch   int
	}{
		{"v1.17.0", false, 1, 17, 0},
		{"v2.16.3", false, 2, 16, 3},
		{"1.15.1", false, 1, 15, 1},
		{"invalid", true, 0, 0, 0},
		{"v1.2", true, 0, 0, 0},
		{"v1.2.x", true, 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pv, err := parseSemver(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.major, pv.major)
				assert.Equal(t, tt.minor, pv.minor)
				assert.Equal(t, tt.patch, pv.patch)
			}
		})
	}
}

func TestInit(t *testing.T) {
	mappingPath := filepath.Join(projectRoot(), "operatorconfig/common/csm-version-mapping.yaml")

	// Reset cached state for test isolation
	cachedInfo = nil
	initOnce = sync.Once{}
	initErr = nil

	// First call should load
	err := Init(mappingPath)
	require.NoError(t, err)
	assert.NotNil(t, GetInfo())

	// Second call should use cached value (no error even if path is invalid)
	err = Init("/nonexistent/path")
	assert.NoError(t, err)
	assert.NotNil(t, GetInfo())
}

func TestGetInfo(t *testing.T) {
	// Reset cached state for test isolation
	cachedInfo = nil
	initOnce = sync.Once{}
	initErr = nil

	// Before Init, GetInfo should return nil
	assert.Nil(t, GetInfo())

	mappingPath := filepath.Join(projectRoot(), "operatorconfig/common/csm-version-mapping.yaml")
	err := Init(mappingPath)
	require.NoError(t, err)

	// After Init, GetInfo should return the cached info
	info := GetInfo()
	assert.NotNil(t, info)
	assert.Greater(t, len(info.CSMVersions), 0)
}

func TestModuleVersion(t *testing.T) {
	mappingPath := filepath.Join(projectRoot(), "operatorconfig/common/csm-version-mapping.yaml")
	modulePath := filepath.Join(projectRoot(), "operatorconfig/moduleconfig/common/version-values.yaml")
	info, err := Load(mappingPath)
	require.NoError(t, err)
	require.NoError(t, info.LoadModuleVersions(modulePath))

	// Test direct ModuleVersion call with known driver config version
	authVer := info.ModuleVersion("powerflex", "v2.17.0", "authorization")
	assert.NotEmpty(t, authVer, "module version should be found for powerflex v2.17.0/authorization")

	// Test with unknown driver
	assert.Empty(t, info.ModuleVersion("nonexistent", "v2.17.0", "authorization"))

	// Test with unknown driver config version
	assert.Empty(t, info.ModuleVersion("powerflex", "v99.99.99", "authorization"))

	// Test with unknown module
	assert.Empty(t, info.ModuleVersion("powerflex", "v2.17.0", "nonexistent"))

	// Test when moduleVersions is not loaded
	infoNoModules, err := Load(mappingPath)
	require.NoError(t, err)
	assert.Empty(t, infoNoModules.ModuleVersion("powerflex", "v2.17.0", "authorization"))
}

func TestLoadModuleVersionsInvalidPath(t *testing.T) {
	mappingPath := filepath.Join(projectRoot(), "operatorconfig/common/csm-version-mapping.yaml")
	info, err := Load(mappingPath)
	require.NoError(t, err)

	err = info.LoadModuleVersions("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func TestLoadModuleVersionsInvalidYAML(t *testing.T) {
	mappingPath := filepath.Join(projectRoot(), "operatorconfig/common/csm-version-mapping.yaml")
	info, err := Load(mappingPath)
	require.NoError(t, err)

	tmp := filepath.Join(t.TempDir(), "bad.yaml")
	err = os.WriteFile(tmp, []byte("not: [valid: yaml: mapping"), 0o644)
	require.NoError(t, err)

	err = info.LoadModuleVersions(tmp)
	assert.Error(t, err)
}

func TestInitWithModuleVersionsError(t *testing.T) {
	mappingPath := filepath.Join(projectRoot(), "operatorconfig/common/csm-version-mapping.yaml")

	// Reset cached state for test isolation
	cachedInfo = nil
	initOnce = sync.Once{}
	initErr = nil

	// Init should fail if LoadModuleVersions fails
	err := Init(mappingPath, "/nonexistent/path.yaml")
	assert.Error(t, err)
	// Note: cachedInfo is still set from the successful Load call, only moduleVersions is missing
	assert.NotNil(t, GetInfo())
}

func TestExpandPathTokensWithNMinusTwo(t *testing.T) {
	mappingPath := filepath.Join(projectRoot(), "operatorconfig/common/csm-version-mapping.yaml")
	info, err := Load(mappingPath)
	require.NoError(t, err)

	authN2 := info.ConfigVersionAtIndex("authorization-proxy-server", NMinusTwo)
	require.NotEmpty(t, authN2, "n-2 version should exist")

	input := filepath.Join(projectRoot(), "operatorconfig/moduleconfig/authorization/{authorization-proxy-server:n-2}/authorization-crds.yaml")
	expected := filepath.Join(projectRoot(), "operatorconfig/moduleconfig/authorization/"+authN2+"/authorization-crds.yaml")
	assert.Equal(t, expected, info.ExpandPathTokens(input))
}
