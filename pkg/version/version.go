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

// Package version is the single source of truth for CSM Operator version
// information. It is a top-level package (rather than an e2e-test helper) so
// that both csm-operator itself (pkg/operatorutils, pkg/drivers, controllers)
// and the e2e test suite resolve versions the same way, from the same file.
//
// It reads operatorconfig/common/csm-releases.yaml -- which merges what used
// to be csm-version-mapping.yaml, moduleconfig/common/version-values.yaml,
// and the per-version driverconfig/{driver}/{version}/upgrade-path.yaml and
// moduleconfig/{module}/{version}/upgrade-path.yaml files -- to expose sorted
// CSM operator versions with indexed access for latest, n-1, and n-2
// releases, plus driver/module config-version and upgrade-path lookups.
package version

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Index constants for accessing specific CSM operator versions.
// CSMVersions are sorted newest-first, one representative per minor release.
const (
	Latest    = iota // Most recent CSM operator version
	NMinusOne        // Previous minor release
	NMinusTwo        // Two minor releases back
)

// rawEntity mirrors one entity (driver or the standalone
// authorization-proxy-server module) entry under a CSM release in
// csm-releases.yaml.
type rawEntity struct {
	Version        string            `yaml:"version"`
	MinUpgradeFrom string            `yaml:"minUpgradeFrom"`
	Modules        map[string]string `yaml:"modules"`
}

// Info holds parsed version information from csm-releases.yaml.
type Info struct {
	// CSMVersions holds the representative CSM operator version for each
	// minor release (highest patch per minor), sorted newest-first.
	// Use Latest, NMinusOne, NMinusTwo constants to index.
	CSMVersions []string

	// configVersions maps entity name (driver or authorization-proxy-server)
	// to CSM version → config version. E.g.
	// configVersions["powerflex"]["<csmVer>"] = "<driverVer>".
	configVersions map[string]map[string]string

	// minUpgradeFrom maps entity name → entity configVersion → the oldest
	// configVersion of that entity that can be upgraded directly into it.
	minUpgradeFrom map[string]map[string]string

	// moduleVersions maps driver → driver configVersion → module → module
	// configVersion.
	moduleVersions map[string]map[string]map[string]string
}

// Cached singleton loaded via Init.
var (
	cachedInfo *Info
	initOnce   sync.Once
	initErr    error

	loadCache   = map[string]*Info{}
	loadCacheMu sync.RWMutex
)

// ResetForTest clears the cached singleton so Init can be called again.
// Intended for use in unit tests only.
func ResetForTest() {
	initOnce = sync.Once{}
	cachedInfo = nil
	initErr = nil

	loadCacheMu.Lock()
	loadCache = map[string]*Info{}
	loadCacheMu.Unlock()
}

// Init loads version information from the given csm-releases.yaml path and
// caches the result. Safe to call multiple times; only the first call loads.
func Init(csmReleasesPath string) error {
	initOnce.Do(func() {
		cachedInfo, initErr = Load(csmReleasesPath)
	})
	return initErr
}

// GetInfo returns the cached Info loaded by Init. Returns nil if Init has not
// been called or failed.
func GetInfo() *Info {
	return cachedInfo
}

// Load reads csm-releases.yaml and returns parsed version info.
func Load(csmReleasesPath string) (*Info, error) {
	loadCacheMu.RLock()
	if info, ok := loadCache[csmReleasesPath]; ok {
		loadCacheMu.RUnlock()
		return info, nil
	}
	loadCacheMu.RUnlock()

	loadCacheMu.Lock()
	defer loadCacheMu.Unlock()
	if info, ok := loadCache[csmReleasesPath]; ok {
		return info, nil
	}

	data, err := os.ReadFile(csmReleasesPath) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("read csm releases file %s: %w", csmReleasesPath, err)
	}

	var raw map[string]map[string]rawEntity
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal csm releases file: %w", err)
	}

	configVersions := map[string]map[string]string{}
	minUpgradeFrom := map[string]map[string]string{}
	moduleVersions := map[string]map[string]map[string]string{}
	allVersions := map[string]bool{}

	for csmVer, entities := range raw {
		allVersions[csmVer] = true
		for entity, e := range entities {
			if configVersions[entity] == nil {
				configVersions[entity] = map[string]string{}
			}
			configVersions[entity][csmVer] = e.Version

			if e.MinUpgradeFrom != "" {
				if minUpgradeFrom[entity] == nil {
					minUpgradeFrom[entity] = map[string]string{}
				}
				minUpgradeFrom[entity][e.Version] = e.MinUpgradeFrom
			}

			if len(e.Modules) > 0 {
				if moduleVersions[entity] == nil {
					moduleVersions[entity] = map[string]map[string]string{}
				}
				moduleVersions[entity][e.Version] = e.Modules
			}
		}
	}

	// Parse and group by (major, minor), keeping highest patch per group.
	type minorKey struct{ major, minor int }
	groups := map[minorKey]ParsedVersion{}
	for v := range allVersions {
		pv, err := ParseSemver(v)
		if err != nil {
			return nil, fmt.Errorf("invalid CSM version %q in %s: %w", v, csmReleasesPath, err)
		}
		key := minorKey{pv.Major, pv.Minor}
		if existing, ok := groups[key]; !ok || pv.Patch > existing.Patch {
			groups[key] = pv
		}
	}

	// Sort representatives by (major, minor) descending.
	reps := make([]ParsedVersion, 0, len(groups))
	for _, pv := range groups {
		reps = append(reps, pv)
	}
	sort.Slice(reps, func(i, j int) bool {
		if reps[i].Major != reps[j].Major {
			return reps[i].Major > reps[j].Major
		}
		return reps[i].Minor > reps[j].Minor
	})

	csmVersions := make([]string, len(reps))
	for i, pv := range reps {
		csmVersions[i] = pv.raw
	}

	info := &Info{
		CSMVersions:    csmVersions,
		configVersions: configVersions,
		minUpgradeFrom: minUpgradeFrom,
		moduleVersions: moduleVersions,
	}
	loadCache[csmReleasesPath] = info
	return info, nil
}

// CSMVersion returns the CSM operator version at the given index.
// Returns empty string if the index is out of range.
func (info *Info) CSMVersion(idx int) string {
	if idx < 0 || idx >= len(info.CSMVersions) {
		return ""
	}
	return info.CSMVersions[idx]
}

// ConfigVersion returns the driver/module config version for a given entity
// name (e.g. "powerflex", "authorization-proxy-server") and CSM version.
// Returns empty string if not found.
func (info *Info) ConfigVersion(entity, csmVersion string) string {
	if m, ok := info.configVersions[entity]; ok {
		return m[csmVersion]
	}
	return ""
}

// SupportedCSMVersions returns the list of CSM release versions recorded for
// entity, and false if entity is not present in csm-releases.yaml at all
// (as opposed to simply not having the requested CSM version).
func (info *Info) SupportedCSMVersions(entity string) ([]string, bool) {
	m, ok := info.configVersions[entity]
	if !ok {
		return nil, false
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, true
}

// ConfigVersionAtIndex returns the config version for an entity at a given
// version index (Latest, NMinusOne, NMinusTwo) based on the entity's own
// available CSM minor releases. This ensures a driver that did not release at a
// particular CSM patch still resolves to its highest available CSM version for
// the matching minor release.
func (info *Info) ConfigVersionAtIndex(entity string, idx int) string {
	m, ok := info.configVersions[entity]
	if !ok {
		return ""
	}

	// Group available CSM versions by (major, minor), keeping highest patch.
	type minorKey struct{ major, minor int }
	groups := map[minorKey]ParsedVersion{}
	for csmVer := range m {
		pv, err := ParseSemver(csmVer)
		if err != nil {
			continue
		}
		key := minorKey{pv.Major, pv.Minor}
		if existing, ok := groups[key]; !ok || pv.Patch > existing.Patch {
			groups[key] = pv
		}
	}

	// Sort minor releases newest-first.
	versions := make([]ParsedVersion, 0, len(groups))
	for _, pv := range groups {
		versions = append(versions, pv)
	}
	sort.Slice(versions, func(i, j int) bool {
		if versions[i].Major != versions[j].Major {
			return versions[i].Major > versions[j].Major
		}
		return versions[i].Minor > versions[j].Minor
	})

	if idx < 0 || idx >= len(versions) {
		return ""
	}
	return m[versions[idx].raw]
}

// ModuleVersionAtIndex returns the module configVersion for a given driver type,
// module name, and version index. It chains: CSM version → driver configVersion
// → module configVersion.
func (info *Info) ModuleVersionAtIndex(driverType, moduleName string, idx int) string {
	driverCfgVer := info.ConfigVersionAtIndex(driverType, idx)
	if driverCfgVer == "" {
		return ""
	}
	return info.ModuleVersion(driverType, driverCfgVer, moduleName)
}

// ModuleVersion returns the module configVersion for a given driver type,
// driver configVersion, and module name.
func (info *Info) ModuleVersion(driverType, driverConfigVersion, moduleName string) string {
	if info.moduleVersions == nil {
		return ""
	}
	if driverMap, ok := info.moduleVersions[driverType]; ok {
		if modMap, ok := driverMap[driverConfigVersion]; ok {
			return modMap[moduleName]
		}
	}
	return ""
}

// MinUpgradeFrom returns the oldest configVersion of entity that can be
// upgraded directly into configVersion. Returns empty string if no
// upgrade-path information is recorded for that (entity, configVersion) pair.
func (info *Info) MinUpgradeFrom(entity, configVersion string) string {
	if m, ok := info.minUpgradeFrom[entity]; ok {
		return m[configVersion]
	}
	return ""
}

// pathTokenRe matches version tokens like {entity} (latest) or {entity:n-1}.
var pathTokenRe = regexp.MustCompile(`\{([^}:]+)(?::([^}]+))?\}`)

// ExpandPathTokens replaces version tokens in a path string.
// Supported formats:
//   - {entity}         → config version at Latest (e.g. {authorization-proxy-server} → N)
//   - {entity:n-1}     → config version at NMinusOne
//   - {entity:n-2}     → config version at NMinusTwo
func (info *Info) ExpandPathTokens(path string) string {
	return pathTokenRe.ReplaceAllStringFunc(path, func(match string) string {
		parts := pathTokenRe.FindStringSubmatch(match)
		entity := parts[1]
		keyword := parts[2] // empty string means latest
		idx := Latest
		switch keyword {
		case "n-1":
			idx = NMinusOne
		case "n-2":
			idx = NMinusTwo
		case "", "latest":
			idx = Latest
		}
		if v := info.ConfigVersionAtIndex(entity, idx); v != "" {
			return v
		}
		return match // leave unchanged if not resolvable
	})
}

// ParsedVersion holds a parsed semantic version.
type ParsedVersion struct {
	Major, Minor, Patch int
	raw                 string
}

// IsLessThan returns true if this version is less than the given major.minor threshold.
func (pv ParsedVersion) IsLessThan(major, minor int) bool {
	if pv.Major < major {
		return true
	}
	if pv.Major == major && pv.Minor < minor {
		return true
	}
	return false
}

// ParseSemver parses a "vMAJOR.MINOR.PATCH" string. It does not support
// pre-release or build metadata.
func ParseSemver(v string) (ParsedVersion, error) {
	trimmed := strings.TrimPrefix(v, "v")
	parts := strings.Split(trimmed, ".")
	if len(parts) != 3 {
		return ParsedVersion{}, fmt.Errorf("invalid version format: %s", v)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return ParsedVersion{}, fmt.Errorf("invalid major in %s: %w", v, err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return ParsedVersion{}, fmt.Errorf("invalid minor in %s: %w", v, err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return ParsedVersion{}, fmt.Errorf("invalid patch in %s: %w", v, err)
	}
	return ParsedVersion{Major: major, Minor: minor, Patch: patch, raw: v}, nil
}
