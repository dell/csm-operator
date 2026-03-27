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

// module-version-update walks a repository tree and updates container image
// tags and configVersion fields based on a declarative input file. It
// auto-detects versioned directories (containing semver-named subdirs),
// only updates the latest (N) version, and can rotate version directories
// when a new release is introduced.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	repoRoot := flag.String("repo", "", "Repository root (default: $GITHUB_WORKSPACE or cwd)")
	inputFile := flag.String("input", "", "Input YAML file with image/module versions (required)")
	dryRun := flag.Bool("dry-run", false, "Print what would change without writing files")
	scopeFlag := flag.String("scope", "all", `Scope of updates: "drivers", "modules", "sidecars", or "all"`)
	flag.Parse()

	if *repoRoot == "" {
		if ws := os.Getenv("GITHUB_WORKSPACE"); ws != "" {
			*repoRoot = ws
		} else {
			*repoRoot = "."
		}
	}
	if *inputFile == "" {
		flag.Usage()
		log.Fatal("--input is required")
	}
	if !IsValidScope(*scopeFlag) {
		log.Fatalf("invalid --scope value %q: must be one of: drivers, modules, sidecars, all", *scopeFlag)
	}

	input, err := LoadInput(*inputFile, Scope(*scopeFlag))
	if err != nil {
		log.Fatalf("loading input: %v", err)
	}

	stats, err := Run(*repoRoot, input, *dryRun)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	verb := "Updated"
	if *dryRun {
		verb = "Would update"
	}
	fmt.Printf("\n%s %d files, skipped %d pinned files.\n", verb, stats.Updated, stats.Pinned)
}
