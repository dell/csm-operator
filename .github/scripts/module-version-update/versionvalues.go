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
// updates it with driver-module version entries from the input.
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
	for driver, entry := range input.VersionValues {
		driverNode := findNodeValue(root, driver)
		if driverNode == nil || driverNode.Kind != yaml.MappingNode {
			continue
		}

		// Build desired module versions for this driver.
		desired := make(map[string]string)
		for _, mod := range entry.Modules {
			if ver, ok := input.ConfigVersions[mod]; ok {
				desired[mod] = ver
			}
		}
		if len(desired) == 0 {
			continue
		}

		verNode := findNodeValue(driverNode, entry.DriverVersion)
		if verNode != nil && verNode.Kind == yaml.MappingNode {
			// Update existing entry.
			for mod, ver := range desired {
				modValNode := findNodeValue(verNode, mod)
				if modValNode != nil {
					if modValNode.Value != ver {
						modValNode.Value = ver
						changed = true
					}
				} else {
					verNode.Content = append(verNode.Content,
						&yaml.Node{Kind: yaml.ScalarNode, Value: mod},
						&yaml.Node{Kind: yaml.ScalarNode, Value: ver},
					)
					changed = true
				}
			}
		} else {
			// Create new driver version entry.
			newMapping := &yaml.Node{Kind: yaml.MappingNode}
			for _, mod := range entry.Modules {
				ver, ok := desired[mod]
				if !ok {
					continue
				}
				newMapping.Content = append(newMapping.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Value: mod},
					&yaml.Node{Kind: yaml.ScalarNode, Value: ver},
				)
			}
			driverNode.Content = append(driverNode.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: entry.DriverVersion},
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

	// yaml.Encoder may add a trailing "...\n"; strip it for consistency.
	out := bytes.TrimSuffix(buf.Bytes(), []byte("...\n"))
	// Ensure file ends with a single newline.
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
