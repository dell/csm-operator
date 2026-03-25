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
	// already exist, it is created by copying the current latest. Directories
	// beyond the 3 most recent (N, N-1, N-2) are pruned.
	//
	// Example:
	//   resiliency: v1.17.0
	//   authorization: v2.6.0
	VersionDirs map[string]string `yaml:"versionDirs"`
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
