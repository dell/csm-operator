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

func releasesPath() string {
	return filepath.Join(projectRoot(), "operatorconfig/common/csm-releases.yaml")
}

func TestLoad(t *testing.T) {
	info, err := Load(releasesPath())
	require.NoError(t, err)
	require.NotNil(t, info)

	// Should have at least 3 minor releases (Latest, N-1, N-2).
	assert.GreaterOrEqual(t, len(info.CSMVersions), 3,
		"expected at least 3 CSM minor-release versions")

	// Versions should be sorted newest-first.
	for i := 0; i < len(info.CSMVersions)-1; i++ {
		cur, err := ParseSemver(info.CSMVersions[i])
		require.NoError(t, err, "parsing current CSM version %s", info.CSMVersions[i])
		next, err := ParseSemver(info.CSMVersions[i+1])
		require.NoError(t, err, "parsing next CSM version %s", info.CSMVersions[i+1])
		assert.GreaterOrEqual(t, cur.Major, next.Major,
			"CSMVersions should be sorted by major version descending: %s before %s",
			info.CSMVersions[i], info.CSMVersions[i+1])
		if cur.Major == next.Major {
			assert.Greater(t, cur.Minor, next.Minor,
				"CSMVersions should be sorted by minor version descending: %s before %s",
				info.CSMVersions[i], info.CSMVersions[i+1])
		}
	}
}

func TestCSMVersion(t *testing.T) {
	info, err := Load(releasesPath())
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
	info, err := Load(releasesPath())
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
	info, err := Load(releasesPath())
	require.NoError(t, err)

	for _, driver := range []string{"powerflex", "powermax", "powerscale", "powerstore"} {
		cv := info.ConfigVersionAtIndex(driver, NMinusOne)
		assert.NotEmpty(t, cv, "config version at N-1 for %s should not be empty", driver)
	}

	// Out-of-range index.
	assert.Empty(t, info.ConfigVersionAtIndex("powerflex", 999))
}

func TestModuleVersions(t *testing.T) {
	info, err := Load(releasesPath())
	require.NoError(t, err)

	// powerflex latest should have authorization module version.
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
	info, err := Load(releasesPath())
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
	info, err := Load(releasesPath())
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
		pv, err := ParseSemver(v)
		require.NoError(t, err)
		// Check no other version with the same minor has a higher patch.
		for otherV := range collectAllCSMVersions(info) {
			opv, err := ParseSemver(otherV)
			if err != nil {
				continue
			}
			if opv.Major == pv.Major && opv.Minor == pv.Minor {
				assert.GreaterOrEqual(t, pv.Patch, opv.Patch,
					"index %d (%s) should have highest patch for minor %d, but found %s",
					i, v, pv.Minor, otherV)
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
			pv, err := ParseSemver(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.major, pv.Major)
				assert.Equal(t, tt.minor, pv.Minor)
				assert.Equal(t, tt.patch, pv.Patch)
			}
		})
	}
}

func TestInit(t *testing.T) {
	// Reset cached state for test isolation (use ResetForTest to also clear loadCache)
	ResetForTest()

	// First call should load
	err := Init(releasesPath())
	require.NoError(t, err)
	assert.NotNil(t, GetInfo())

	// Second call should use cached value (no error even if path is invalid)
	err = Init("/nonexistent/path")
	assert.NoError(t, err)
	assert.NotNil(t, GetInfo())
}

func TestGetInfo(t *testing.T) {
	// Reset cached state for test isolation (use ResetForTest to also clear loadCache)
	ResetForTest()

	// Before Init, GetInfo should return nil
	assert.Nil(t, GetInfo())

	err := Init(releasesPath())
	require.NoError(t, err)

	// After Init, GetInfo should return the cached info
	info := GetInfo()
	assert.NotNil(t, info)
	assert.Greater(t, len(info.CSMVersions), 0)
}

func TestModuleVersion(t *testing.T) {
	info, err := Load(releasesPath())
	require.NoError(t, err)

	// Resolve a known driver configVersion dynamically (avoids hardcoding a
	// version string that will go stale).
	driverCfgVer := info.ConfigVersionAtIndex("powerflex", Latest)
	require.NotEmpty(t, driverCfgVer)

	// Test direct ModuleVersion call with known driver config version
	authVer := info.ModuleVersion("powerflex", driverCfgVer, "authorization")
	assert.NotEmpty(t, authVer, "module version should be found for powerflex %s/authorization", driverCfgVer)

	// Test with unknown driver
	assert.Empty(t, info.ModuleVersion("nonexistent", driverCfgVer, "authorization"))

	// Test with unknown driver config version
	assert.Empty(t, info.ModuleVersion("powerflex", "v99.99.99", "authorization"))

	// Test with unknown module
	assert.Empty(t, info.ModuleVersion("powerflex", driverCfgVer, "nonexistent"))
}

func TestMinUpgradeFrom(t *testing.T) {
	info, err := Load(releasesPath())
	require.NoError(t, err)

	driverCfgVer := info.ConfigVersionAtIndex("powerflex", Latest)
	require.NotEmpty(t, driverCfgVer)

	minFrom := info.MinUpgradeFrom("powerflex", driverCfgVer)
	assert.NotEmpty(t, minFrom, "expected a minUpgradeFrom entry for powerflex %s", driverCfgVer)

	// Unknown entity/configVersion returns empty.
	assert.Empty(t, info.MinUpgradeFrom("nonexistent", driverCfgVer))
	assert.Empty(t, info.MinUpgradeFrom("powerflex", "v99.99.99"))
}

func TestResetForTest(t *testing.T) {
	// Reset cached state for test isolation
	cachedInfo = nil
	initOnce = sync.Once{}
	initErr = nil

	require.NoError(t, Init(releasesPath()))
	assert.NotNil(t, GetInfo())

	ResetForTest()
	assert.Nil(t, GetInfo())

	// Init should load again after a reset.
	require.NoError(t, Init(releasesPath()))
	assert.NotNil(t, GetInfo())

	ResetForTest()
}

func TestIsLessThan(t *testing.T) {
	pv, err := ParseSemver("v1.17.0")
	require.NoError(t, err)

	assert.True(t, pv.IsLessThan(2, 0), "1.17 should be less than 2.0")
	assert.True(t, pv.IsLessThan(1, 18), "1.17 should be less than 1.18")
	assert.False(t, pv.IsLessThan(1, 17), "1.17 should not be less than 1.17")
	assert.False(t, pv.IsLessThan(1, 0), "1.17 should not be less than 1.0")
	assert.False(t, pv.IsLessThan(0, 99), "1.17 should not be less than 0.99")
}

func TestExpandPathTokensWithNMinusTwo(t *testing.T) {
	info, err := Load(releasesPath())
	require.NoError(t, err)

	authN2 := info.ConfigVersionAtIndex("authorization-proxy-server", NMinusTwo)
	require.NotEmpty(t, authN2, "n-2 version should exist")

	input := filepath.Join(projectRoot(), "operatorconfig/moduleconfig/authorization/{authorization-proxy-server:n-2}/authorization-crds.yaml")
	expected := filepath.Join(projectRoot(), "operatorconfig/moduleconfig/authorization/"+authN2+"/authorization-crds.yaml")
	assert.Equal(t, expected, info.ExpandPathTokens(input))
}

func TestSupportedCSMVersions(t *testing.T) {
	info, err := Load(releasesPath())
	require.NoError(t, err)

	// Test with known entities that should have CSM versions
	for _, entity := range []string{"powerflex", "powermax", "powerscale", "powerstore", "unity", "authorization-proxy-server"} {
		versions, exists := info.SupportedCSMVersions(entity)
		assert.True(t, exists, "%s should exist in csm-releases.yaml", entity)
		assert.NotEmpty(t, versions, "%s should have at least one CSM version", entity)
		assert.Greater(t, len(versions), 0, "%s should have multiple CSM versions", entity)
	}

	// Test with unknown entity
	versions, exists := info.SupportedCSMVersions("nonexistent")
	assert.False(t, exists, "unknown entity should return false")
	assert.Nil(t, versions, "unknown entity should return nil versions")
}

func TestConfigVersionEdgeCases(t *testing.T) {
	info, err := Load(releasesPath())
	require.NoError(t, err)

	// Test with empty CSM version
	assert.Empty(t, info.ConfigVersion("powerflex", ""))

	// Test with unknown CSM version for known entity
	assert.Empty(t, info.ConfigVersion("powerflex", "v99.99.99"))

	// Test with empty entity
	assert.Empty(t, info.ConfigVersion("", "v1.18.0"))
}

func TestConfigVersionAtIndexEdgeCases(t *testing.T) {
	info, err := Load(releasesPath())
	require.NoError(t, err)

	// Test with unknown entity
	assert.Empty(t, info.ConfigVersionAtIndex("nonexistent", Latest))

	// Test with entity that has no versions (edge case)
	// This tests the path where configVersions[entity] is empty map
	info.configVersions["empty"] = map[string]string{}
	assert.Empty(t, info.ConfigVersionAtIndex("empty", Latest))

	// Test with out-of-range index
	assert.Empty(t, info.ConfigVersionAtIndex("powerflex", 999))
	assert.Empty(t, info.ConfigVersionAtIndex("powerflex", -1))
}

func TestModuleVersionEdgeCases(t *testing.T) {
	info, err := Load(releasesPath())
	require.NoError(t, err)

	// Test with empty driver type
	assert.Empty(t, info.ModuleVersion("", "v2.18.0", "authorization"))

	// Test with unknown driver type
	assert.Empty(t, info.ModuleVersion("nonexistent", "v2.18.0", "authorization"))

	// Test with empty driver config version
	assert.Empty(t, info.ModuleVersion("powerflex", "", "authorization"))

	// Test with unknown driver config version
	assert.Empty(t, info.ModuleVersion("powerflex", "v99.99.99", "authorization"))

	// Test with empty module name
	assert.Empty(t, info.ModuleVersion("powerflex", "v2.18.0", ""))

	// Test with unknown module name
	assert.Empty(t, info.ModuleVersion("powerflex", "v2.18.0", "nonexistent"))

	// Test with nil moduleVersions
	info.moduleVersions = nil
	assert.Empty(t, info.ModuleVersion("powerflex", "v2.18.0", "authorization"))
}

func TestModuleVersionAtIndexEdgeCases(t *testing.T) {
	info, err := Load(releasesPath())
	require.NoError(t, err)

	// Test with empty driver type
	assert.Empty(t, info.ModuleVersionAtIndex("", "authorization", Latest))

	// Test with unknown driver type
	assert.Empty(t, info.ModuleVersionAtIndex("nonexistent", "authorization", Latest))

	// Test with empty module name
	assert.Empty(t, info.ModuleVersionAtIndex("powerflex", "", Latest))

	// Test with unknown module name
	assert.Empty(t, info.ModuleVersionAtIndex("powerflex", "nonexistent", Latest))

	// Test with out-of-range index
	assert.Empty(t, info.ModuleVersionAtIndex("powerflex", "authorization", 999))
	assert.Empty(t, info.ModuleVersionAtIndex("powerflex", "authorization", -1))
}

func TestMinUpgradeFromEdgeCases(t *testing.T) {
	info, err := Load(releasesPath())
	require.NoError(t, err)

	// Test with unknown entity
	assert.Empty(t, info.MinUpgradeFrom("nonexistent", "v2.18.0"))

	// Test with empty config version
	assert.Empty(t, info.MinUpgradeFrom("powerflex", ""))

	// Test with unknown config version
	assert.Empty(t, info.MinUpgradeFrom("powerflex", "v99.99.99"))

	// Test with entity that has no minUpgradeFrom entries
	info.minUpgradeFrom["no-upgrade"] = map[string]string{}
	assert.Empty(t, info.MinUpgradeFrom("no-upgrade", "v2.18.0"))
}

func TestExpandPathTokensEdgeCases(t *testing.T) {
	info, err := Load(releasesPath())
	require.NoError(t, err)

	// Test with empty path
	assert.Equal(t, "", info.ExpandPathTokens(""))

	// Test with multiple tokens in one path
	authLatest := info.ConfigVersionAtIndex("authorization-proxy-server", Latest)
	pflexLatest := info.ConfigVersionAtIndex("powerflex", Latest)
	require.NotEmpty(t, authLatest)
	require.NotEmpty(t, pflexLatest)

	multiTokenPath := filepath.Join(projectRoot(), "operatorconfig/moduleconfig/authorization/{authorization-proxy-server}/{powerflex}/config.yaml")
	expectedPath := filepath.Join(projectRoot(), "operatorconfig/moduleconfig/authorization/"+authLatest+"/"+pflexLatest+"/config.yaml")
	assert.Equal(t, expectedPath, info.ExpandPathTokens(multiTokenPath))

	// Test with "latest" keyword explicitly
	latestPath := filepath.Join(projectRoot(), "operatorconfig/moduleconfig/authorization/{authorization-proxy-server:latest}/config.yaml")
	latestExpected := filepath.Join(projectRoot(), "operatorconfig/moduleconfig/authorization/"+authLatest+"/config.yaml")
	assert.Equal(t, latestExpected, info.ExpandPathTokens(latestPath))

	// Test with malformed tokens (should leave unchanged)
	assert.Equal(t, "{entity}/file.yaml", info.ExpandPathTokens("{entity}/file.yaml"))
	assert.Equal(t, "{entity:invalid}/file.yaml", info.ExpandPathTokens("{entity:invalid}/file.yaml"))

	// Test with nested braces (should only match outermost)
	assert.Equal(t, "{{entity}}/file.yaml", info.ExpandPathTokens("{{entity}}/file.yaml"))
}
