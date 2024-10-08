//  Copyright © 2022-2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"strings"
	"testing"
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1"
	step "github.com/dell/csm-operator/tests/e2e/steps"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	timeout          = time.Minute * 10
	interval         = time.Second * 10
	valuesFileEnvVar = "E2E_SCENARIOS_FILE"
)

var (
	testResources []step.Resource
	tagsSpecified []string
	stepRunner    *step.Runner
	beautify      string
	testApex      bool
	moduleTags    = []string{"authorization", "replication", "observability", "authorizationproxyserver", "resiliency", "applicationmobility"}
)

func Contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

func ContainsTag(scenarioTags []string, tagsSpecified []string) bool {
	for _, tag := range tagsSpecified {
		// Check to see if scenario has required tag
		if Contains(scenarioTags, tag) {
			return true
		}
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
		By(fmt.Sprintf("Returning false here"))
		return false
	}
	for _, tag := range scenarioTags {
		if Contains(moduleTags, tag) {
			By(fmt.Sprintf("--no-modules specified, skipping"))
			return true
		}
	}
	By(fmt.Sprintf("Returning false at end"))
	return false
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
	tagEnvVars := []string{"NOMODULES", "AUTHORIZATION", "REPLICATION", "OBSERVABILITY", "AUTHORIZATIONPROXYSERVER", "RESILIENCY", "APPLICATIONMOBILITY", "POWERFLEX", "POWERSCALE", "POWERMAX", "POWERSTORE", "UNITY", "SANITY", "CLIENT"}
	By("Getting test environment variables")
	valuesFile := os.Getenv(valuesFileEnvVar)
	Expect(valuesFile).NotTo(BeEmpty(), "Missing environment variable required for tests. E2E_SCENARIOS_FILE must be set.")

	for _, tagEnvVar := range tagEnvVars {
		enabled := os.Getenv(tagEnvVar)
		if enabled == "true" {
			tagsSpecified = append(tagsSpecified, strings.ToLower(tagEnvVar))
		}
	}

	By(fmt.Sprint(tagsSpecified))

	By("Reading values file")
	var err error
	testResources, err = step.GetTestResources(valuesFile)
	if err != nil {
		framework.Failf("Failed to read values file: %v", err)
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

var _ = Describe("[run-e2e-test] E2E Testing", func() {
	It("Running all test Given Test Scenarios", func() {
		if testApex {
			for _, test := range testResources {
				By(fmt.Sprintf("Starting: %s ", test.Scenario.Scenario))
				if !ContainsTag(test.Scenario.Tags, tagsSpecified) {
					By(fmt.Sprintf("Not tagged for this test run, skipping"))
					By(fmt.Sprintf("Ending: %s\n", test.Scenario.Scenario))
					continue
				}

				// if no-modules are enabled, skip this test if it has a module tag
				if CheckNoModules(test.Scenario.Tags) {
					By(fmt.Sprintf("Ending: %s\n", test.Scenario.Scenario))
					continue
				}

				for _, stepName := range test.Scenario.Steps {
					By(fmt.Sprintf("%s Executing  %s", beautify, stepName))
					Eventually(func() error {
						return stepRunner.RunStepClient(stepName, test)
					}, timeout, interval).Should(BeNil())
				}
				By(fmt.Sprintf("Ending: %s\n", test.Scenario.Scenario))
				time.Sleep(5 * time.Second)
			}
		} else {
			for _, test := range testResources {
				By(fmt.Sprintf("Starting: %s ", test.Scenario.Scenario))
				if ContainsTag(test.Scenario.Tags, tagsSpecified) == false {
					By(fmt.Sprintf("Not tagged for this test run, skipping"))
					By(fmt.Sprintf("Ending: %s\n", test.Scenario.Scenario))
					continue
				}

				// if no-modules are enabled, skip this test if it has a module tag
				if CheckNoModules(test.Scenario.Tags) {
					By(fmt.Sprintf("Ending: %s\n", test.Scenario.Scenario))
					continue
				}

				for _, stepName := range test.Scenario.Steps {
					By(fmt.Sprintf("%s Executing  %s", beautify, stepName))
					Eventually(func() error {
						return stepRunner.RunStep(stepName, test)
					}, timeout, interval).Should(BeNil())
				}
				By(fmt.Sprintf("Ending: %s\n", test.Scenario.Scenario))
				time.Sleep(5 * time.Second)
			}
		}
	})
})
