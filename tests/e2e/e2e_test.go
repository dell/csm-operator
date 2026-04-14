//  Copyright © 2022-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package e2e

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	csmv1 "eos2git.cec.lab.emc.com/CSM/csm-operator/api/v1"
	"eos2git.cec.lab.emc.com/CSM/csm-operator/tests/e2e/scripts/junit"
	step "eos2git.cec.lab.emc.com/CSM/csm-operator/tests/e2e/steps"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	interval         = time.Second * 10
	valuesFileEnvVar = "E2E_SCENARIOS_FILE"
)

var (
	testScenarios []step.Scenario
	tagsSpecified []string
	stepRunner    *step.Runner
	beautify      string
	moduleTags    = []string{"authorization", "replication", "observability", "authorizationproxyserver", "resiliency", "zoning"}
	platformTags  = []string{"powerflex", "powerscale", "powermax", "powerstore", "unity", "cosi"}
	// optInTags are module tags that require an explicit --flag to run.
	// Unlike standard modules (which run when no module filter is given),
	// scenarios carrying an opt-in tag are skipped unless that tag appears
	// in the user-specified filter.
	optInTags = []string{"zoning"}
	// optInPlatformTags are platform tags that require an explicit --flag to run.
	// Unlike standard platforms (which run when no platform filter is given),
	// scenarios carrying an opt-in platform tag are skipped unless that tag
	// appears in the user-specified filter.
	optInPlatformTags = []string{"cosi"}
	// exclusiveModuleTags are module tags that require explicit specification
	// when other module filters are active. Unlike optInTags (which are also
	// skipped when no filter is set), exclusive tags DO run when no module
	// filter is given (i.e. "run everything"). This prevents e.g. auth+obs
	// combo scenarios from running when only --obs is passed without --auth
	// or --auth-proxy.
	exclusiveModuleTags = []string{"authorization", "authorizationproxyserver"}
)

func Contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

// ContainsTag checks whether a scenario should run based on specified tags.
// Platform tags (powerflex, powerstore, etc.) and module/feature tags
// (authorizationproxyserver, observability, etc.) are ANDed together:
//   - Only platforms specified  → scenario must match a platform
//   - Only modules specified    → scenario must match a module
//   - Both specified            → scenario must match at least one platform AND at least one module
func ContainsTag(scenarioTags []string, tagsSpecified []string) bool {
	var platformsSpec, modulesSpec []string
	for _, tag := range tagsSpecified {
		if Contains(platformTags, tag) {
			platformsSpec = append(platformsSpec, tag)
		} else {
			modulesSpec = append(modulesSpec, tag)
		}
	}

	// No platform filter → all platforms match; otherwise need at least one hit
	platformMatch := len(platformsSpec) == 0
	if platformMatch {
		// Even with no explicit platform filter, scenarios tagged with an
		// opt-in platform tag (e.g. "cosi") still require that tag to be
		// present in tagsSpecified; they never run by default.
		for _, tag := range scenarioTags {
			if Contains(optInPlatformTags, tag) {
				platformMatch = false
				break
			}
		}
	}
	for _, p := range platformsSpec {
		if Contains(scenarioTags, p) {
			platformMatch = true
			break
		}
	}

	// No module filter → all modules match; otherwise need at least one hit
	moduleMatch := len(modulesSpec) == 0
	if moduleMatch {
		// Even with no explicit module filter, scenarios tagged with an
		// opt-in tag (e.g. "zoning") still require that tag to be present
		// in tagsSpecified; they never run by default.
		for _, tag := range scenarioTags {
			if Contains(optInTags, tag) {
				moduleMatch = false
				break
			}
		}
	}
	for _, m := range modulesSpec {
		if Contains(scenarioTags, m) {
			moduleMatch = true
			break
		}
	}

	// Guard: scenarios carrying exclusive module tags (e.g. authorization)
	// must have at least one of those tags explicitly specified when the
	// user provided a module filter. This prevents combined auth+obs
	// scenarios from running when only --obs is passed.
	if moduleMatch && len(modulesSpec) > 0 {
		scenarioHasExclusive := false
		exclusiveSpecified := false
		for _, tag := range scenarioTags {
			if Contains(exclusiveModuleTags, tag) {
				scenarioHasExclusive = true
				if Contains(modulesSpec, tag) {
					exclusiveSpecified = true
				}
			}
		}
		if scenarioHasExclusive && !exclusiveSpecified {
			moduleMatch = false
		}
	}

	if platformMatch && moduleMatch {
		return true
	}
	By(fmt.Sprintf("No matching tags for scenario"))
	return false
}

// if --no-modules was passed in, we need to make sure any test with modules is filtered out
// even if it contains matching tags
// return value of true - --no-modules will prevent this test from running
// return value of false - --no-modules will not prevent this test from running
func CheckNoModules(scenarioTags []string) bool {
	if os.Getenv("NOMODULES") == "false" || os.Getenv("NOMODULES") == "" {
		return false
	}
	for _, tag := range scenarioTags {
		if Contains(moduleTags, tag) {
			By(fmt.Sprintf("--no-modules specified, skipping"))
			return true
		}
	}
	return false
}

// scenarioResult captures the outcome of a single scenario for the final report.
type scenarioResult struct {
	Name    string
	Status  string // "PASS", "FAIL", "SKIP"
	Elapsed time.Duration
	Error   string
}

// padDashes returns a dash string to frame content inside an 80-char line.
// Short content gets wide padding; long content still gets a minimum of 6
// dashes so that every line is visually framed.
func padDashes(content string) string {
	const lineWidth = 80
	const minDashes = 6
	pad := (lineWidth - len(content) - 4) / 2 // 4 = 2 spaces on each side
	if pad < minDashes {
		pad = minDashes
	}
	return strings.Repeat("-", pad)
}

// continueOnFailure returns true when E2E_CONTINUE_ON_FAILURE is set to "true".
// Default behaviour (env var unset or any other value) is fail-fast.
func continueOnFailure() bool {
	return strings.EqualFold(os.Getenv("E2E_CONTINUE_ON_FAILURE"), "true")
}

// pollStep retries a step function until it succeeds or the timeout expires.
// Unlike Eventually().Should(BeNil()), it returns an error instead of panicking.
func pollStep(runner *step.Runner, stepName string, test step.Resource,
	timeout, poll time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	// try once immediately
	if err := runner.RunStep(stepName, test); err == nil {
		return nil
	}

	var lastErr error
	for {
		select {
		case <-deadline:
			return fmt.Errorf("step %q timed out after %s: %v", stepName, timeout, lastErr)
		case <-ticker.C:
			if err := runner.RunStep(stepName, test); err != nil {
				lastErr = err
			} else {
				return nil
			}
		}
	}
}

// printReport prints a formatted table of scenario results with totals.
func printReport(results []scenarioResult, totalElapsed time.Duration) {
	const separator = "=========================================================================================="

	passed, failed, aborted, skipped := 0, 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "PASS":
			passed++
		case "FAIL":
			failed++
		case "ABORT":
			aborted++
		case "SKIP":
			skipped++
		}
	}

	// Sort results by status: SKIP first, then PASS, then FAIL/ABORT.
	// The sort operates on a copy, so the original results slice is
	// untouched — the JUnit XML report still gets execution-order results.
	statusOrder := map[string]int{"SKIP": 0, "PASS": 1, "FAIL": 2, "ABORT": 3}
	sorted := make([]scenarioResult, len(results))
	copy(sorted, results)
	sort.SliceStable(sorted, func(i, j int) bool {
		return statusOrder[sorted[i].Status] < statusOrder[sorted[j].Status]
	})

	By(fmt.Sprintf(""))
	By(fmt.Sprintf(separator))
	By(fmt.Sprintf("  %-6s  %-60s %s", "STATUS", "SCENARIO", "TIME"))
	By(fmt.Sprintf(separator))

	for _, r := range sorted {
		switch r.Status {
		case "SKIP":
			By(fmt.Sprintf("  %-6s  %s", r.Status, r.Name))
		default:
			By(fmt.Sprintf("  %-6s  %-60s [%s]", r.Status, r.Name, r.Elapsed))
		}
		if r.Error != "" {
			By(fmt.Sprintf("           Error: %s", r.Error))
		}
	}

	By(fmt.Sprintf(separator))
	ran := passed + failed + aborted
	By(fmt.Sprintf("  Total: %d | Passed: %d | Failed: %d | Aborted: %d | Skipped: %d | Time: %s",
		ran, passed, failed, aborted, skipped, totalElapsed.Round(time.Second)))
	By(fmt.Sprintf(separator))
}

// TestE2E -
func TestE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	initializeFramework()
	RegisterFailHandler(Fail)
	RunSpecs(t, "CSM Operator End-to-End Tests")
}

var _ = BeforeSuite(func() {
	tagEnvVars := []string{"AUTHORIZATION", "REPLICATION", "OBSERVABILITY", "AUTHORIZATIONPROXYSERVER", "RESILIENCY", "POWERFLEX", "POWERSCALE", "POWERMAX", "POWERSTORE", "UNITY", "SANITY", "ZONING", "COSI"}
	By("Getting test environment variables")
	valuesFile := os.Getenv(valuesFileEnvVar)
	Expect(valuesFile).NotTo(BeEmpty(), "Missing environment variable required for tests. E2E_SCENARIOS_FILE must be set.")

	for _, tagEnvVar := range tagEnvVars {
		enabled := os.Getenv(tagEnvVar)
		if enabled == "true" {
			tagsSpecified = append(tagsSpecified, strings.ToLower(tagEnvVar))
		}
	}

	customTag := os.Getenv("ADD_SCENARIO_TAG")
	if customTag != "" {
		tagsSpecified = append(tagsSpecified, customTag)
	}

	By(fmt.Sprint(tagsSpecified))

	By("Setting E2E_CREG_VERSION fallback for custom registry tests")
	if os.Getenv("E2E_CREG_VERSION") == "" {
		os.Setenv("E2E_CREG_VERSION", "v1.16.0") // fallback to last stable release
	}

	By("Generating minimal testfiles")
	if err := step.GenerateMinimalTestfiles("testfiles/minimal-testfiles"); err != nil {
		framework.Failf("Failed to generate minimal testfiles: %v", err)
	}

	By("Initializing on-demand testfile generation from samples")
	step.InitTestfileGeneration("testfiles", "../../samples")

	By("Parsing scenarios")
	var err error
	testScenarios, err = step.ParseScenarios(valuesFile)
	if err != nil {
		framework.Failf("Failed to parse scenarios: %v", err)
	}

	By("Getting a k8s client")
	ctrlClient, err := client.New(config.GetConfigOrDie(), client.Options{})
	if err != nil {
		framework.Failf("Failed to create control runtime client: %v", err)
	}
	csmv1.AddToScheme(ctrlClient.Scheme())

	clientSet, err := kubernetes.NewForConfig(config.GetConfigOrDie())
	if err != nil {
		framework.Failf("Failed to create kubernetes clientset : %v", err)
	}

	stepRunner = &step.Runner{}
	step.StepRunnerInit(stepRunner, ctrlClient, clientSet)

	beautify = "    "
})

var _ = AfterSuite(func() {
	By("Cleaning up generated minimal testfiles")
	step.CleanupMinimalTestfiles("testfiles/minimal-testfiles")

	By("Cleaning up generated testfiles from samples")
	step.CleanupGeneratedTestfiles("testfiles")
})

var _ = Describe("[run-e2e-test] E2E Testing", func() {
	It("Running all test Given Test Scenarios", func() {
		keepGoing := continueOnFailure()
		suiteStart := time.Now()
		var results []scenarioResult

		// Track the scenario that is currently executing so that an
		// interrupt (Ctrl+C) can mark it as ABORT in the report.
		var inProgress string
		var inProgressStart time.Time

		// DeferCleanup runs after this It block finishes — whether it
		// completes normally, calls Fail(), or is interrupted by SIGINT.
		DeferCleanup(func() {
			if inProgress != "" {
				elapsed := time.Since(inProgressStart).Round(time.Second)
				results = append(results, scenarioResult{
					Name: inProgress, Status: "ABORT",
					Elapsed: elapsed, Error: "test run interrupted",
				})
			}
			totalElapsed := time.Since(suiteStart)
			printReport(results, totalElapsed)

			junitResults := make([]junit.Result, len(results))
			for i, r := range results {
				junitResults[i] = junit.Result{
					Name: r.Name, Status: r.Status,
					Elapsed: r.Elapsed, Error: r.Error,
				}
			}
			if path, err := junit.WriteReport(junitResults, totalElapsed); err != nil {
				By(fmt.Sprintf("WARNING: %v", err))
			} else {
				By(fmt.Sprintf("JUnit report written to %s", path))
			}
		})

		for _, scenario := range testScenarios {
			startContent := fmt.Sprintf("%sSTARTING  %s", beautify, scenario.Scenario)
			By(fmt.Sprintf("%s %s %s", padDashes(startContent), startContent, padDashes(startContent)))

			if !ContainsTag(scenario.Tags, tagsSpecified) {
				By(fmt.Sprintf("Not tagged for this test run, skipping"))
				results = append(results, scenarioResult{Name: scenario.Scenario, Status: "SKIP"})
				continue
			}

			// Override config.enableSftpSDC using env var
			if strings.Contains(strings.Join(scenario.Tags, ","), "powerflex") {
				if scenario.Config == nil {
					scenario.Config = map[string]string{}
				}
				scenario.Config["enableSftpSDC"] = os.Getenv("POWERFLEX_SDC_SFTP_REPO_ENABLED")
			}

			// if no-modules are enabled, skip this test if it has a module tag
			if CheckNoModules(scenario.Tags) {
				results = append(results, scenarioResult{Name: scenario.Scenario, Status: "SKIP"})
				continue
			}

			inProgress = scenario.Scenario
			inProgressStart = time.Now()

			// --- run the scenario, capturing any error ---
			scenarioErr := func() error {
				// Generate needed testfiles and load CRs for this scenario on the fly.
				test, err := step.LoadResourceForScenario(scenario)
				if err != nil {
					return fmt.Errorf("load resources: %v", err)
				}

				// Clean up the temporary test directory and in-memory state
				// to avoid unintended use of rendered templates or stale
				// flags from previous tests.
				step.ResetPerScenarioState()
				if err := os.RemoveAll("temp"); err != nil {
					return fmt.Errorf("clean temp dir: %v", err)
				}
				if err := os.MkdirAll("temp", 0o700); err != nil {
					return fmt.Errorf("create temp dir: %v", err)
				}

				for _, stepName := range test.Scenario.Steps {
					stepName = os.ExpandEnv(stepName)
					By(fmt.Sprintf("%s Executing  %s", beautify, stepName))
					stepTimeout := step.StepTimeout(stepName)
					if err := pollStep(stepRunner, stepName, test, stepTimeout, interval); err != nil {
						return err
					}
				}
				return nil
			}()

			scenarioElapsed := time.Since(inProgressStart).Round(time.Second)
			inProgress = "" // scenario finished; clear before recording result

			if scenarioErr != nil {
				errMsg := scenarioErr.Error()
				content := fmt.Sprintf("%sFAILED  %s  [%s]", beautify, scenario.Scenario, scenarioElapsed)
				By(fmt.Sprintf("%s %s %s", padDashes(content), content, padDashes(content)))
				By(fmt.Sprintf("    Error: %s", errMsg))
				results = append(results, scenarioResult{
					Name: scenario.Scenario, Status: "FAIL",
					Elapsed: scenarioElapsed, Error: errMsg,
				})

				if !keepGoing {
					Fail(fmt.Sprintf("scenario %q failed: %s", scenario.Scenario, errMsg))
				}
			} else {
				content := fmt.Sprintf("%sSUCCEEDED  %s  [%s]", beautify, scenario.Scenario, scenarioElapsed)
				By(fmt.Sprintf("%s %s %s", padDashes(content), content, padDashes(content)))
				results = append(results, scenarioResult{
					Name: scenario.Scenario, Status: "PASS", Elapsed: scenarioElapsed,
				})
			}
			time.Sleep(2 * time.Second)
		}

		// Count failures to decide whether to Fail() the overall test.
		// The report itself is printed by DeferCleanup above.
		for _, r := range results {
			if r.Status == "FAIL" {
				Fail(fmt.Sprintf("%d scenario(s) failed", countByStatus(results, "FAIL")))
			}
		}
	})
})

// countByStatus returns how many results have the given status.
func countByStatus(results []scenarioResult, status string) int {
	n := 0
	for _, r := range results {
		if r.Status == status {
			n++
		}
	}
	return n
}
