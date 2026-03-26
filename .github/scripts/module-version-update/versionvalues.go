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
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const versionValuesFilename = "version-values.yaml"

// updateVersionValuesInRepo finds version-values.yaml by walking the tree and
// updates it with driver version entries from the input.
func updateVersionValuesInRepo(repoRoot string, input *Input, dryRun bool) (bool, error) {
	var found string
	_ = filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && defaultSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == versionValuesFilename {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if found == "" {
		fmt.Println("  version-values.yaml not found, skipping")
		return false, nil
	}
	return updateVersionValuesFile(found, input, dryRun)
}

// updateVersionValuesFile reads version-values.yaml as a yaml.Node tree,
// adds or updates driver version entries, and writes it back.
//
// For each driver in input.VersionValues (driver name → driver version):
//   - If the driver version entry already exists, update its module values
//     using input.ConfigVersions.
//   - If it doesn't exist, copy the structure from the latest existing entry,
//     update the module values, and append it.
//
// The tool learns which modules each driver supports from the existing file
// structure rather than from the input configuration.
func updateVersionValuesFile(path string, input *Input, dryRun bool) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false, fmt.Errorf("parsing %s: %w", path, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return false, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return false, nil
	}

	changed := false
	for driver, driverVersion := range input.VersionValues {
		driverNode := findNodeValue(root, driver)
		if driverNode == nil || driverNode.Kind != yaml.MappingNode {
			continue
		}

		verNode := findNodeValue(driverNode, driverVersion)
		if verNode != nil && verNode.Kind == yaml.MappingNode {
			// Entry exists — update module versions from configVersions.
			if updateMappingValues(verNode, input.ConfigVersions) {
				changed = true
			}
		} else {
			// Entry doesn't exist — copy structure from the latest entry.
			latestNode := lastMappingValue(driverNode)
			if latestNode == nil {
				continue
			}
			newMapping := &yaml.Node{Kind: yaml.MappingNode}
			for i := 0; i < len(latestNode.Content)-1; i += 2 {
				mod := latestNode.Content[i].Value
				ver := latestNode.Content[i+1].Value
				if newVer, ok := input.ConfigVersions[mod]; ok {
					ver = newVer
				}
				newMapping.Content = append(newMapping.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Value: mod},
					&yaml.Node{Kind: yaml.ScalarNode, Value: ver},
				)
			}
			driverNode.Content = append(driverNode.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: driverVersion},
				newMapping,
			)
			changed = true
		}
	}

	if !changed {
		return false, nil
	}
	if dryRun {
		fmt.Printf("  would update: %s\n", path)
		return true, nil
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return false, fmt.Errorf("encoding %s: %w", path, err)
	}
	enc.Close()

	// Strip trailing "...\n" that yaml.Encoder may add.
	out := bytes.TrimSuffix(buf.Bytes(), []byte("...\n"))
	out = append(bytes.TrimRight(out, "\n"), '\n')

	info, _ := os.Stat(path)
	if err := os.WriteFile(path, out, info.Mode()); err != nil {
		return false, err
	}
	fmt.Printf("  updated: %s\n", path)
	return true, nil
}

// findNodeValue finds the value node for a given key in a YAML mapping node.
func findNodeValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if strings.EqualFold(mapping.Content[i].Value, key) {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// lastMappingValue returns the value node of the last key-value pair in a
// mapping. Used to find the "latest" driver version entry to copy from.
func lastMappingValue(mapping *yaml.Node) *yaml.Node {
	if mapping.Kind != yaml.MappingNode || len(mapping.Content) < 2 {
		return nil
	}
	return mapping.Content[len(mapping.Content)-1]
}

// updateMappingValues updates values in a YAML mapping node using the provided
// map. Only keys that exist in both the mapping and the values map are updated.
// Returns true if any value was changed.
func updateMappingValues(mapping *yaml.Node, values map[string]string) bool {
	changed := false
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		key := mapping.Content[i].Value
		if newVal, ok := values[key]; ok {
			if mapping.Content[i+1].Value != newVal {
				mapping.Content[i+1].Value = newVal
				changed = true
			}
		}
	}
	return changed
}
