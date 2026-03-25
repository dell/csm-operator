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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// VersionedDir represents a directory that contains semver-named
// subdirectories (e.g., operatorconfig/moduleconfig/resiliency/ containing
// v1.14.0/, v1.15.0/, v1.16.0/).
type VersionedDir struct {
	Path     string   // absolute path to the parent directory
	Name     string   // basename (e.g., "resiliency")
	Versions []string // semver subdirectory names, sorted ascending
}

// Latest returns the highest semver subdirectory name (N).
func (vd VersionedDir) Latest() string {
	if len(vd.Versions) == 0 {
		return ""
	}
	return vd.Versions[len(vd.Versions)-1]
}

// Directories that should never be descended into during discovery.
var defaultSkipDirs = map[string]bool{
	".git":   true,
	"vendor": true,
}

// DiscoverVersionedDirs walks the repo tree and finds all directories that
// contain two or more semver-named subdirectories. Results are sorted
// deepest-first so that nested versioned directories (e.g., samples/cosi/)
// are matched before their parents (e.g., samples/).
func DiscoverVersionedDirs(root string) ([]VersionedDir, error) {
	var result []VersionedDir

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if defaultSkipDirs[d.Name()] {
			return filepath.SkipDir
		}

		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return nil // skip unreadable directories
		}

		var versions []string
		for _, e := range entries {
			if e.IsDir() && IsSemver(e.Name()) {
				versions = append(versions, e.Name())
			}
		}
		if len(versions) >= 2 {
			SortSemvers(versions)
			result = append(result, VersionedDir{
				Path:     path,
				Name:     filepath.Base(path),
				Versions: versions,
			})
		}
		return nil
	})

	// Sort deepest-first for correct nested matching in IsInNonTargetVersionDir.
	sort.Slice(result, func(i, j int) bool {
		return strings.Count(result[i].Path, string(os.PathSeparator)) >
			strings.Count(result[j].Path, string(os.PathSeparator))
	})

	return result, err
}

// IsInNonTargetVersionDir checks whether filePath resides inside a versioned
// directory but NOT in the version subdirectory that should be updated.
//
// targetVersions maps versioned directory names to the specific version that
// should receive updates. For versioned directories not present in the map,
// the latest (N) is used as the default target.
func IsInNonTargetVersionDir(filePath string, vDirs []VersionedDir, targetVersions map[string]string) bool {
	for _, vd := range vDirs {
		rel, err := filepath.Rel(vd.Path, filePath)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		parts := strings.SplitN(filepath.ToSlash(rel), "/", 2)
		if !IsSemver(parts[0]) {
			continue
		}
		target, ok := targetVersions[vd.Name]
		if !ok {
			target = vd.Latest()
		}
		return parts[0] != target
	}
	return false
}

// FindVersionedDir returns the first VersionedDir whose basename matches name.
func FindVersionedDir(dirs []VersionedDir, name string) *VersionedDir {
	for i := range dirs {
		if dirs[i].Name == name {
			return &dirs[i]
		}
	}
	return nil
}

// RotateVersionDir ensures that targetVersion exists as a subdirectory of vd.
// If it doesn't exist, it is created by copying the current latest and the
// single oldest version directory is removed.
func RotateVersionDir(vd *VersionedDir, targetVersion string) error {
	targetPath := filepath.Join(vd.Path, targetVersion)

	if dirExists(targetPath) {
		fmt.Printf("  version dir exists: %s/%s\n", vd.Name, targetVersion)
		return nil
	}

	latest := vd.Latest()
	latestPath := filepath.Join(vd.Path, latest)
	fmt.Printf("  creating %s/%s (from %s)\n", vd.Name, targetVersion, latest)
	if err := copyDir(latestPath, targetPath); err != nil {
		return fmt.Errorf("copying %s -> %s: %w", latest, targetVersion, err)
	}

	// Update the in-memory version list.
	vd.Versions = append(vd.Versions, targetVersion)
	SortSemvers(vd.Versions)

	// Remove only the single oldest directory.
	oldest := vd.Versions[0]
	oldestPath := filepath.Join(vd.Path, oldest)
	fmt.Printf("  pruning %s/%s\n", vd.Name, oldest)
	if err := os.RemoveAll(oldestPath); err != nil {
		return fmt.Errorf("removing %s: %w", oldest, err)
	}
	vd.Versions = vd.Versions[1:]

	return nil
}

// copyDir recursively copies a directory tree from src to dst.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// dirExists returns true if path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
