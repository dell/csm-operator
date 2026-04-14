//  Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//       http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

// Package junit writes E2E scenario results as JUnit XML reports.
package junit

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Result captures the outcome of a single E2E scenario.
type Result struct {
	Name    string
	Status  string // "PASS", "FAIL", "ABORT", "SKIP"
	Elapsed time.Duration
	Error   string
}

// JUnit XML structures.

type testSuite struct {
	XMLName  xml.Name   `xml:"testsuite"`
	Name     string     `xml:"name,attr"`
	Tests    int        `xml:"tests,attr"`
	Failures int        `xml:"failures,attr"`
	Skipped  int        `xml:"skipped,attr"`
	Time     float64    `xml:"time,attr"`
	Cases    []testCase `xml:"testcase"`
}

type testCase struct {
	Name    string   `xml:"name,attr"`
	Time    float64  `xml:"time,attr"`
	Failure *failure `xml:"failure,omitempty"`
	Skipped *skipped `xml:"skipped,omitempty"`
}

type failure struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

type skipped struct {
	Message string `xml:"message,attr,omitempty"`
}

// WriteReport writes results as JUnit XML.
// The output path defaults to "e2e-junit-report.xml" in the current directory
// and can be overridden via the E2E_JUNIT_REPORT env var.
// It returns the path written to and any error encountered.
func WriteReport(results []Result, totalElapsed time.Duration) (string, error) {
	path := os.Getenv("E2E_JUNIT_REPORT")
	if path == "" {
		path = "e2e-junit-report.xml"
	}

	var suite testSuite
	suite.Name = "CSM Operator E2E"
	suite.Time = totalElapsed.Seconds()

	for _, r := range results {
		tc := testCase{
			Name: r.Name,
			Time: r.Elapsed.Seconds(),
		}
		switch r.Status {
		case "FAIL":
			suite.Failures++
			tc.Failure = &failure{Message: r.Error, Body: r.Error}
		case "ABORT":
			suite.Failures++
			tc.Failure = &failure{Message: "test run interrupted", Body: r.Error}
		case "SKIP":
			suite.Skipped++
			tc.Skipped = &skipped{Message: "Not tagged for this test run"}
		}
		suite.Cases = append(suite.Cases, tc)
	}
	suite.Tests = len(suite.Cases)

	data, err := xml.MarshalIndent(suite, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal JUnit XML: %w", err)
	}
	payload := append([]byte(xml.Header), data...)
	if err := os.WriteFile(filepath.Clean(path), payload, 0o644); err != nil {
		return "", fmt.Errorf("write JUnit report to %s: %w", path, err)
	}
	return path, nil
}
