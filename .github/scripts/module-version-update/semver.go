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
	"sort"
	"strconv"
	"strings"
)

// Semver holds a parsed semantic version.
type Semver struct {
	Major, Minor, Patch int
	Raw                 string // original string, e.g. "v1.2.3"
}

// ParseSemver parses a semver string with an optional "v" prefix.
// Returns the parsed version and true, or a zero value and false.
func ParseSemver(s string) (Semver, bool) {
	raw := s
	s = strings.TrimPrefix(s, "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return Semver{}, false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	patch, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return Semver{}, false
	}
	return Semver{Major: major, Minor: minor, Patch: patch, Raw: raw}, true
}

// IsSemver returns true if s looks like a semver string (vX.Y.Z or X.Y.Z).
func IsSemver(s string) bool {
	_, ok := ParseSemver(s)
	return ok
}

// Less returns true if a < b in semver ordering.
func (a Semver) Less(b Semver) bool {
	if a.Major != b.Major {
		return a.Major < b.Major
	}
	if a.Minor != b.Minor {
		return a.Minor < b.Minor
	}
	return a.Patch < b.Patch
}

// SortSemvers sorts a slice of semver strings in ascending order.
// Non-semver strings are sorted lexicographically after valid semvers.
func SortSemvers(versions []string) {
	sort.Slice(versions, func(i, j int) bool {
		a, aok := ParseSemver(versions[i])
		b, bok := ParseSemver(versions[j])
		if !aok || !bok {
			return versions[i] < versions[j]
		}
		return a.Less(b)
	})
}


// SemverNMinusOne decrements the minor version of a semver string.
// For example, "v2.4.0" -> "v2.3.0". If minor is already 0, the original
// version is returned unchanged.
func SemverNMinusOne(ver string) (string, error) {
	sv, ok := ParseSemver(ver)
	if !ok {
		return ver, fmt.Errorf("not a valid semver: %s", ver)
	}
	prefix := ""
	if len(ver) > 0 && ver[0] == 'v' {
		prefix = "v"
	}
	if sv.Minor > 0 {
		sv.Minor--
	}
	return fmt.Sprintf("%s%d.%d.%d", prefix, sv.Major, sv.Minor, sv.Patch), nil
}
