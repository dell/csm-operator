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

// ---------------------------------------------------------------------------
// csm-versions.yaml schema
// ---------------------------------------------------------------------------

// CSMVersions is the top-level structure of csm-versions.yaml.
type CSMVersions struct {
	CSM          CSMSection              `yaml:"csm"`
	Dependencies map[string]ProductEntry `yaml:"dependencies"`
	CSI          CSISection              `yaml:"csi"`
}

// CSMSection holds drivers, modules, and tools under the "csm" key.
type CSMSection struct {
	Version         string                  `yaml:"version"`
	DefaultRegistry string                  `yaml:"defaultRegistry"`
	Tools           map[string]ProductEntry `yaml:"tools"`
	Drivers         map[string]ProductEntry `yaml:"drivers"`
	Modules         map[string]ProductEntry `yaml:"modules"`
}

// ProductEntry is a versioned product with optional images and aliases.
// Used for tools, drivers, modules, dependencies, and sidecars.
// Aliases lists additional operator configVersion names that should receive
// the same version (e.g. "authorization-proxy-server" for "authorization").
type ProductEntry struct {
	Version string       `yaml:"version"`
	Images  []ImageEntry `yaml:"images"`
	Aliases []string     `yaml:"aliases,omitempty"`
}

// ImageEntry describes a container image. Registry overrides the parent
// defaultRegistry; Tag overrides the parent version.
type ImageEntry struct {
	Name     string `yaml:"name"`
	Registry string `yaml:"registry,omitempty"`
	Tag      string `yaml:"tag,omitempty"`
}



// CSISection holds sidecar images under the "csi" key.
type CSISection struct {
	Version         string                  `yaml:"version"`
	DefaultRegistry string                  `yaml:"defaultRegistry"`
	Sidecars        map[string]ProductEntry `yaml:"sidecars"`
}

// ---------------------------------------------------------------------------
// Internal representation used by the update engine
// ---------------------------------------------------------------------------

// Input is the flat representation consumed by Run().
type Input struct {
	Images         map[string]string
	ConfigVersions map[string]string
	VersionDirs    map[string]string
	NMinus1        map[string]NMinus1Override
	// VersionValues maps driver names to their target version for
	// version-values.yaml. The tool reads the existing file to learn
	// which modules each driver supports.
	VersionValues map[string]string
}

// NMinus1Override specifies the N-1 configVersion and image tags for a module.
type NMinus1Override struct {
	ConfigVersion string
	Images        map[string]string
}

// ---------------------------------------------------------------------------
// Loading and transformation
// ---------------------------------------------------------------------------

// LoadInput reads csm-versions.yaml and transforms it into a flat Input.
func LoadInput(path string) (*Input, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var versions CSMVersions
	if err := yaml.Unmarshal(data, &versions); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	input, err := versions.ToInput()
	if err != nil {
		return nil, fmt.Errorf("transforming %s: %w", path, err)
	}

	if len(input.Images) == 0 {
		return nil, fmt.Errorf("input file %s produced no images", path)
	}
	return input, nil
}

// ToInput transforms the hierarchical CSMVersions into a flat Input.
func (cv *CSMVersions) ToInput() (*Input, error) {
	input := &Input{
		Images:         make(map[string]string),
		ConfigVersions: make(map[string]string),
		VersionDirs:    make(map[string]string),
		NMinus1:        make(map[string]NMinus1Override),
		VersionValues:  make(map[string]string),
	}

	csmReg := cv.CSM.DefaultRegistry

	// --- Modules ---
	// The YAML key is the operator configVersion name and version dir name.
	for name, mod := range cv.CSM.Modules {
		addImages(input.Images, mod.Images, csmReg, mod.Version)

		input.ConfigVersions[name] = mod.Version
		input.VersionDirs[name] = mod.Version

		for _, alias := range mod.Aliases {
			input.ConfigVersions[alias] = mod.Version
		}

		// Auto-compute N-1 overrides for every module.
		nm1Ver, err := SemverNMinusOne(mod.Version)
		if err == nil && nm1Ver != mod.Version {
			nm1Images := make(map[string]string)
			for _, img := range mod.Images {
				reg := img.Registry
				if reg == "" {
					reg = csmReg
				}
				nm1Images[reg+"/"+img.Name] = nm1Ver
			}
			input.NMinus1[name] = NMinus1Override{
				ConfigVersion: nm1Ver,
				Images:        nm1Images,
			}
			for _, alias := range mod.Aliases {
				input.NMinus1[alias] = NMinus1Override{
					ConfigVersion: nm1Ver,
					Images:        nm1Images,
				}
			}
		}
	}

	// --- Tools ---
	for _, tool := range cv.CSM.Tools {
		addImages(input.Images, tool.Images, csmReg, tool.Version)
	}

	// --- Drivers ---
	// The YAML key is the driver name in version-values.yaml.
	for name, driver := range cv.CSM.Drivers {
		addImages(input.Images, driver.Images, csmReg, driver.Version)
		input.VersionValues[name] = driver.Version
	}

	// --- Dependencies ---
	for _, dep := range cv.Dependencies {
		addImages(input.Images, dep.Images, csmReg, dep.Version)
	}

	// --- CSI Sidecars ---
	csiReg := cv.CSI.DefaultRegistry
	for _, sidecar := range cv.CSI.Sidecars {
		addImages(input.Images, sidecar.Images, csiReg, sidecar.Version)
	}

	return input, nil
}

// addImages resolves each ImageEntry to a fully-qualified image reference
// and adds it to the images map.
func addImages(images map[string]string, entries []ImageEntry, defaultRegistry, defaultTag string) {
	for _, img := range entries {
		reg := img.Registry
		if reg == "" {
			reg = defaultRegistry
		}
		tag := img.Tag
		if tag == "" {
			tag = defaultTag
		}
		if reg == "" || img.Name == "" || tag == "" {
			continue
		}
		images[reg+"/"+img.Name] = tag
	}
}
