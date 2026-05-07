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

// Package version provides a single source of truth for CSM operator version
// information. It reads operatorconfig/common/csm-version-mapping.yaml and
// operatorconfig/moduleconfig/common/version-values.yaml to expose sorted CSM
// operator versions with indexed access for latest, n-1, and n-2 releases,
// plus driver and module config-version lookups.
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

// Info holds parsed version information from csm-version-mapping.yaml and
// optionally version-values.yaml.
type Info struct {
	// CSMVersions holds the representative CSM operator version for each
	// minor release (highest patch per minor), sorted newest-first.
	// Use Latest, NMinusOne, NMinusTwo constants to index.
	CSMVersions []string

	// configVersions maps entity name (driver or module) to
	// CSM version → config version. E.g. configVersions["powerflex"]["v1.17.0"] = "v2.17.0".
	configVersions map[string]map[string]string

	// moduleVersions maps driver → driver configVersion → module → module configVersion.
	// Populated from version-values.yaml via LoadModuleVersions.
	moduleVersions map[string]map[string]map[string]string
}

// Cached singleton loaded via Init.
var (
	cachedInfo *Info
	initOnce   sync.Once
	initErr    error
)

// Init loads version information from the given csm-version-mapping.yaml path
// and caches the result. Safe to call multiple times; only the first call loads.
// If moduleVersionsPath is provided, version-values.yaml is also loaded.
func Init(mappingFilePath string, moduleVersionsPaths ...string) error {
	initOnce.Do(func() {
		cachedInfo, initErr = Load(mappingFilePath)
		if initErr != nil {
			return
		}
		for _, p := range moduleVersionsPaths {
			if err := cachedInfo.LoadModuleVersions(p); err != nil {
				initErr = err
				return
			}
		}
	})
	return initErr
}

// GetInfo returns the cached Info loaded by Init. Returns nil if Init has not
// been called or failed.
func GetInfo() *Info {
	return cachedInfo
}

// Load reads csm-version-mapping.yaml and returns parsed version info.
func Load(mappingFilePath string) (*Info, error) {
	data, err := os.ReadFile(mappingFilePath) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("read version mapping %s: %w", mappingFilePath, err)
	}

	var raw map[string]map[string]string
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal version mapping: %w", err)
	}

	// Collect all unique CSM versions across all entities.
	allVersions := map[string]bool{}
	for _, entityMap := range raw {
		for csmVer := range entityMap {
			allVersions[csmVer] = true
		}
	}

	// Parse and group by (major, minor), keeping highest patch per group.
	type minorKey struct{ major, minor int }
	groups := map[minorKey]parsedVersion{}
	for v := range allVersions {
		pv, err := parseSemver(v)
		if err != nil {
			continue
		}
		key := minorKey{pv.major, pv.minor}
		if existing, ok := groups[key]; !ok || pv.patch > existing.patch {
			groups[key] = pv
		}
	}

	// Sort representatives by (major, minor) descending.
	reps := make([]parsedVersion, 0, len(groups))
	for _, pv := range groups {
		reps = append(reps, pv)
	}
	sort.Slice(reps, func(i, j int) bool {
		if reps[i].major != reps[j].major {
			return reps[i].major > reps[j].major
		}
		return reps[i].minor > reps[j].minor
	})

	csmVersions := make([]string, len(reps))
	for i, pv := range reps {
		csmVersions[i] = pv.raw
	}

	return &Info{
		CSMVersions:    csmVersions,
		configVersions: raw,
	}, nil
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

// ConfigVersionAtIndex returns the config version for an entity at a given
// version index (Latest, NMinusOne, NMinusTwo).
func (info *Info) ConfigVersionAtIndex(entity string, idx int) string {
	csmVer := info.CSMVersion(idx)
	if csmVer == "" {
		return ""
	}
	return info.ConfigVersion(entity, csmVer)
}

// LoadModuleVersions reads version-values.yaml and populates module version
// mappings. The file maps: driver → driverConfigVersion → module → moduleConfigVersion.
func (info *Info) LoadModuleVersions(path string) error {
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return fmt.Errorf("read module versions %s: %w", path, err)
	}
	var raw map[string]map[string]map[string]string
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal module versions: %w", err)
	}
	info.moduleVersions = raw
	return nil
}

// ModuleVersionAtIndex returns the module configVersion for a given driver type,
// module name, and version index. It chains: CSM version → driver configVersion
// → module configVersion via version-values.yaml.
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

// pathTokenRe matches version tokens like {entity} (latest) or {entity:n-1}.
var pathTokenRe = regexp.MustCompile(`\{([^}:]+)(?::([^}]+))?\}`)

// ExpandPathTokens replaces version tokens in a path string.
// Supported formats:
//   - {entity}         → config version at Latest (e.g. {authorization-proxy-server} → v2.5.0)
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

// parsedVersion holds a parsed semantic version.
type parsedVersion struct {
	major, minor, patch int
	raw                 string
}

// parseSemver parses a "vMAJOR.MINOR.PATCH" string.
func parseSemver(v string) (parsedVersion, error) {
	trimmed := strings.TrimPrefix(v, "v")
	parts := strings.Split(trimmed, ".")
	if len(parts) != 3 {
		return parsedVersion{}, fmt.Errorf("invalid version format: %s", v)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return parsedVersion{}, fmt.Errorf("invalid major in %s: %w", v, err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return parsedVersion{}, fmt.Errorf("invalid minor in %s: %w", v, err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return parsedVersion{}, fmt.Errorf("invalid patch in %s: %w", v, err)
	}
	return parsedVersion{major: major, minor: minor, patch: patch, raw: v}, nil
}
