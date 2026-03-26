// Copyright 2025 DELL Inc. or its subsidiaries.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Stats tracks counters across a run.
type Stats struct {
	Updated int
	Pinned  int
}

// versionPinMarker is the comment annotation that opts a file out of updates.
// Place "# csm-version-pin" (YAML) or "// csm-version-pin" (Go) in the
// first 10 lines of a file to exclude it.
const versionPinMarker = "csm-version-pin"

// File extensions processed for updates.
var updatableExtensions = map[string]bool{
	".yaml": true, ".yml": true, ".go": true, ".json": true,
}

// Run is the top-level orchestrator. It discovers versioned directories,
// rotates them if needed, then walks the tree applying updates.
func Run(repoRoot string, input *Input, dryRun bool) (Stats, error) {
	var stats Stats

	// Phase 1: discover versioned directories.
	fmt.Println("Discovering versioned directories...")
	vDirs, err := DiscoverVersionedDirs(repoRoot)
	if err != nil {
		return stats, fmt.Errorf("discovering versioned dirs: %w", err)
	}
	for _, vd := range vDirs {
		fmt.Printf("  %s: %v (latest: %s)\n", vd.Name, vd.Versions, vd.Latest())
	}

	// Phase 2: rotate version directories.
	if len(input.VersionDirs) > 0 {
		fmt.Println("Rotating version directories...")
		for name, targetVer := range input.VersionDirs {
			vd := FindVersionedDir(vDirs, name)
			if vd == nil {
				fmt.Printf("  WARNING: no versioned directory found matching %q\n", name)
				continue
			}
			if err := RotateVersionDir(vd, targetVer); err != nil {
				return stats, fmt.Errorf("rotating %s to %s: %w", name, targetVer, err)
			}
		}
		// Re-discover after rotation so the walk uses updated version lists.
		vDirs, err = DiscoverVersionedDirs(repoRoot)
		if err != nil {
			return stats, fmt.Errorf("re-discovering versioned dirs: %w", err)
		}
	}

	// Phase 3: walk the tree and update files.
	// Build the target-version map so the walk only updates files inside the
	// correct version subdirectory (the target), not the latest (N).
	targetVersions := make(map[string]string, len(input.VersionDirs))
	for name, ver := range input.VersionDirs {
		targetVersions[name] = ver
	}

	fmt.Println("Scanning files...")
	err = filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if defaultSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !updatableExtensions[ext] {
			return nil
		}
		if hasVersionPin(path) {
			stats.Pinned++
			return nil
		}
		if IsInNonTargetVersionDir(path, vDirs, targetVersions) {
			return nil
		}

		changed, updateErr := updateFile(path, input, dryRun)
		if updateErr != nil {
			return fmt.Errorf("updating %s: %w", path, updateErr)
		}
		if changed {
			stats.Updated++
		}
		return nil
	})

	// Phase 4: update version-values.yaml if configured.
	if len(input.VersionValues) > 0 {
		changed, vvErr := updateVersionValuesInRepo(repoRoot, input, dryRun)
		if vvErr != nil {
			return stats, fmt.Errorf("updating version-values.yaml: %w", vvErr)
		}
		if changed {
			stats.Updated++
		}
	}

	return stats, err
}

// updateFile reads a file, applies image tag and configVersion updates, and
// writes it back if anything changed. Returns true if the file was modified.
func updateFile(path string, input *Input, dryRun bool) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	// Build effective maps, applying N-1 overrides for annotated files.
	images := input.Images
	configVersions := input.ConfigVersions
	if nm1Modules := readNMinus1Annotations(path); len(nm1Modules) > 0 {
		images = cloneMap(input.Images)
		configVersions = cloneMap(input.ConfigVersions)
		for _, mod := range nm1Modules {
			if ov, ok := input.NMinus1[mod]; ok {
				configVersions[mod] = ov.ConfigVersion
				for img, tag := range ov.Images {
					images[img] = tag
				}
			}
		}
	}

	result := string(content)

	// Apply image tag updates.
	for image, tag := range images {
		result = replaceImageTag(result, image, tag)
	}

	// Apply configVersion updates.
	for module, version := range configVersions {
		result = updateConfigVersionForModule(result, module, version)
	}

	if result == string(content) {
		return false, nil
	}

	if dryRun {
		fmt.Printf("  would update: %s\n", path)
		return true, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(result), info.Mode()); err != nil {
		return false, err
	}
	fmt.Printf("  updated: %s\n", path)
	return true, nil
}

// ---------------------------------------------------------------------------
// Image tag replacement
// ---------------------------------------------------------------------------

// replaceImageTag replaces all occurrences of image:<any-tag> with image:<newTag>.
func replaceImageTag(content, image, newTag string) string {
	escaped := regexp.QuoteMeta(image)
	re := regexp.MustCompile(escaped + `:[a-zA-Z0-9._-]+`)
	return re.ReplaceAllString(content, image+":"+newTag)
}

// ---------------------------------------------------------------------------
// configVersion replacement
// ---------------------------------------------------------------------------

var (
	// Matches JSON-style: "configVersion": "v1.2.3"
	jsonConfigVersionRe = regexp.MustCompile(`("configVersion"\s*:\s*)"[^"]+"`)
	// Matches YAML-style: configVersion: v1.2.3 (also handles quoted YAML values)
	yamlConfigVersionRe = regexp.MustCompile(`(configVersion:\s*)("?)([^\s,"]+)("?)`)
)

// updateConfigVersionForModule finds occurrences of a module name and updates
// the nearest configVersion field. Works for both YAML and JSON contexts.
func updateConfigVersionForModule(content, moduleName, newVersion string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if !matchesModuleName(line, moduleName) {
			continue
		}
		// Search this line and the next 20 for a configVersion field.
		end := min(len(lines), i+21)
		for j := i; j < end; j++ {
			updated, changed := tryReplaceConfigVersion(lines[j], newVersion)
			if changed {
				lines[j] = updated
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

// matchesModuleName returns true if line contains a YAML/JSON "name" field
// whose value is exactly moduleName.
var moduleNameRe = regexp.MustCompile(`"?name"?\s*:\s*"?([a-zA-Z0-9_-]+)"?`)

func matchesModuleName(line, moduleName string) bool {
	matches := moduleNameRe.FindStringSubmatch(line)
	return matches != nil && matches[1] == moduleName
}

// tryReplaceConfigVersion replaces the version value on a configVersion line.
// Returns the updated line and true if a replacement was made.
func tryReplaceConfigVersion(line, newVersion string) (string, bool) {
	if !strings.Contains(line, "configVersion") {
		return line, false
	}
	// Skip commented-out configVersion lines.
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
		return line, false
	}
	// Try JSON-style first: "configVersion": "v1.2.3"
	if jsonConfigVersionRe.MatchString(line) {
		return jsonConfigVersionRe.ReplaceAllString(line, `${1}"`+newVersion+`"`), true
	}
	// Try YAML-style: configVersion: v1.2.3 or configVersion: "v1.2.3"
	if yamlConfigVersionRe.MatchString(line) {
		return yamlConfigVersionRe.ReplaceAllStringFunc(line, func(match string) string {
			loc := yamlConfigVersionRe.FindStringSubmatchIndex(match)
			if loc == nil {
				return match
			}
			prefix := match[loc[2]:loc[3]]     // "configVersion: "
			openQuote := match[loc[4]:loc[5]]   // "" or "\""
			closeQuote := match[loc[8]:loc[9]]  // "" or "\""
			return prefix + openQuote + newVersion + closeQuote
		}), true
	}
	return line, false
}

// ---------------------------------------------------------------------------
// Version pin detection
// ---------------------------------------------------------------------------

// hasVersionPin checks the first 10 lines of a file for the csm-version-pin
// comment marker.
func hasVersionPin(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for i := 0; i < 10 && scanner.Scan(); i++ {
		if strings.Contains(scanner.Text(), versionPinMarker) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// N-1 version annotations
// ---------------------------------------------------------------------------

const nMinus1Marker = "csm-version-n-minus-1:"

// readNMinus1Annotations scans the first 10 lines of a file for
// "# csm-version-n-minus-1: <module>" annotations and returns the module names.
func readNMinus1Annotations(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var modules []string
	scanner := bufio.NewScanner(f)
	for i := 0; i < 10 && scanner.Scan(); i++ {
		line := scanner.Text()
		idx := strings.Index(line, nMinus1Marker)
		if idx == -1 {
			continue
		}
		rest := strings.TrimSpace(line[idx+len(nMinus1Marker):])
		for _, m := range strings.Split(rest, ",") {
			m = strings.TrimSpace(m)
			if m != "" {
				modules = append(modules, m)
			}
		}
	}
	return modules
}

// cloneMap returns a shallow copy of a string map.
func cloneMap(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}
