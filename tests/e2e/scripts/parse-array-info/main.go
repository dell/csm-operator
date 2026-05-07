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

// parse-array-info reads an array-info YAML file, filters sections by
// active platforms and features, and outputs shell export statements
// for non-empty values.
//
// Usage:
//
//	go run main.go -platforms powerflex,powerstore -features auth,zoning -file ../../array-info.yaml
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	platforms := flag.String("platforms", "", "comma-separated active platforms (e.g. powerflex,powerstore)")
	features := flag.String("features", "", "comma-separated active features (e.g. auth,zoning,replication,oidc,sftp,auth-common)")
	file := flag.String("file", "array-info.yaml", "path to YAML config file")
	flag.Parse()

	data, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", *file, err)
		os.Exit(1)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing YAML: %v\n", err)
		os.Exit(1)
	}

	if root.Content == nil || len(root.Content) == 0 {
		fmt.Fprintf(os.Stderr, "error: empty YAML document\n")
		os.Exit(1)
	}

	// Build set of active platforms
	activePlatforms := map[string]bool{}
	for _, p := range strings.Split(*platforms, ",") {
		if p = strings.TrimSpace(p); p != "" {
			activePlatforms[p] = true
		}
	}

	// Build set of active features
	activeFeatures := map[string]bool{}
	for _, f := range strings.Split(*features, ",") {
		if f = strings.TrimSpace(f); f != "" {
			activeFeatures[f] = true
		}
	}

	// Walk top-level mapping: each key is a section name, value is a mapping of env vars
	//
	// Section naming convention:
	//   "global"        -> always loaded (namespace prefix, shared settings)
	//   "powerflex"         -> platform="powerflex", feature=""        (base section, always loaded if platform active)
	//   "powerflex-auth"    -> platform="powerflex", feature="auth"    (loaded if platform active AND feature active)
	//   "powerflex-zoning"  -> platform="powerflex", feature="zoning"  (loaded if platform active AND feature active)
	//   "auth-common"   -> standalone section, loaded only if "auth-common" is in features
	mapping := root.Content[0]
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		sectionName := mapping.Content[i].Value
		sectionBody := mapping.Content[i+1]

		// "global" section is always loaded
		if sectionName == "global" {
			// fall through to export
		} else if activeFeatures[sectionName] {
			// Standalone feature section (e.g. "auth-common") — loaded if listed as a feature
		} else {
			platform := sectionName
			feature := ""
			if idx := strings.Index(sectionName, "-"); idx > 0 {
				platform = sectionName[:idx]
				feature = sectionName[idx+1:]
			}

			// Platform must be active
			if !activePlatforms[platform] {
				continue
			}
			// If section has a feature suffix, that feature must be active
			if feature != "" && !activeFeatures[feature] {
				continue
			}
		}

		// Export each non-empty key-value pair
		if sectionBody == nil || sectionBody.Content == nil {
			continue
		}
		for j := 0; j < len(sectionBody.Content)-1; j += 2 {
			key := sectionBody.Content[j].Value
			val := sectionBody.Content[j+1].Value

			if val == "" {
				continue
			}

			// Shell-escape single quotes in values
			escaped := strings.ReplaceAll(val, "'", "'\\''")
			fmt.Printf("export %s='%s'\n", key, escaped)
		}
	}
}
