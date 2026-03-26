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
	"os"

	"gopkg.in/yaml.v3"
)

// Input is the declarative specification of what to update. It is loaded
// from a YAML file passed via --input.
type Input struct {
	// Images maps fully-qualified image references to their target tags.
	// Every file in the repo tree containing a matching image:tag string
	// will be updated (unless pinned or in an older version directory).
	//
	// Example:
	//   quay.io/dell/container-storage-modules/podmon: v1.17.0
	Images map[string]string `yaml:"images"`

	// ConfigVersions maps YAML module names to their target configVersion.
	// The tool finds "name: <module>" fields and updates the nearest
	// "configVersion" field to the specified version.
	//
	// Example:
	//   observability: v1.16.0
	//   resiliency: v1.17.0
	ConfigVersions map[string]string `yaml:"configVersions"`

	// VersionDirs maps directory names (matched by basename, not full path)
	// to their target version. If the target version directory does not
	// already exist, it is created by copying the current latest and the
	// oldest version directory is removed.
	VersionDirs map[string]string `yaml:"versionDirs"`

	// NMinus1 defines per-module overrides for files annotated with
	// "# csm-version-n-minus-1: <module>". These files receive the N-1
	// configVersion and image tags for the annotated modules instead of
	// the current versions.
	NMinus1 map[string]NMinus1Override `yaml:"nMinus1"`

	// VersionValues defines driver entries to add/update in the
	// version-values.yaml file. Each driver specifies its version and
	// which module configVersions to include in the entry.
	VersionValues map[string]VersionValuesEntry `yaml:"versionValues"`
}

// NMinus1Override specifies the N-1 configVersion and image tags for a module.
type NMinus1Override struct {
	ConfigVersion string            `yaml:"configVersion"`
	Images        map[string]string `yaml:"images"`
}

// VersionValuesEntry specifies a driver version and its associated modules
// for the version-values.yaml compatibility matrix.
type VersionValuesEntry struct {
	DriverVersion string   `yaml:"driverVersion"`
	Modules       []string `yaml:"modules"`
}

// LoadInput reads and parses the input YAML file.
func LoadInput(path string) (*Input, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var input Input
	if err := yaml.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(input.Images) == 0 && len(input.ConfigVersions) == 0 {
		return nil, fmt.Errorf("input file %s specifies no images or configVersions", path)
	}
	return &input, nil
}
