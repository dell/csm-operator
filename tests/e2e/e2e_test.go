package e2e

import (
	"fmt"
	"os"
	"testing"
	"time"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	step "github.com/dell/csm-operator/tests/e2e/steps"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	timeout  = time.Minute * 10
	interval = time.Second * 10
	valuesFileEnvVar = "E2E_VALUES_FILE"
)


var (
	testResources []step.Resource
	installedModules []string
	stepRunner    *step.Runner
	beautify      string
)

func Contains(slice []string, str string) bool {
  for _, v := range slice {
    if v == str {
      return true
    }
  }
  return false
}

func ContainsModules(modulesRequired []string, modulesInstalled []string) bool {
    fmt.Println(modulesRequired)
    fmt.Println(modulesInstalled)
    if len(modulesRequired) == 0 && len(modulesInstalled) == 0 {
        return true
    }

    for _, moduleName := range modulesRequired {
	// check to see if we have modules required
        if Contains(modulesInstalled, moduleName) == false {
            return false
        }
    }
    return true
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
	moduleEnvVars := [2]string{"AUTHORIZATION", "REPLICATION"}
	By("Getting test environment variables")
	valuesFile := os.Getenv(valuesFileEnvVar)
	Expect(valuesFile).NotTo(BeEmpty(), "Missing environment variable required for tests. E2E_VALUES_FILE must both be set.")

	for _, moduleEnvVar := range moduleEnvVars {
		moduleEnvVar = os.Getenv(moduleEnvVar)
		if moduleEnvVar != "" {
			installedModules = append(installedModules, strings.ToLower(moduleEnvVar))
		}
	}

	By("Reading values file")
	res, err := step.GetTestResources(valuesFile)
	if err != nil {
		framework.Failf("Failed to read values file: %v", err)
	}
	testResources = res

	By("Getting a k8s client")
	ctrlClient, err := client.New(config.GetConfigOrDie(), client.Options{})
	if err != nil {
		framework.Failf("Failed to create controll runtime client: %v", err)
	}
	csmv1.AddToScheme(ctrlClient.Scheme())

	clientSet, err := kubernetes.NewForConfig(config.GetConfigOrDie())
	if err != nil {
		framework.Failf("Failed to create kubernetes  clientset : %v", err)
	}

	stepRunner = &step.Runner{}
	step.StepRunnerInit(stepRunner, ctrlClient, clientSet)

	beautify = "    "

})

var _ = Describe("[run-e2e-test]E2E Testing", func() {
	It("Running all test Given Test Scenarios", func() {
		for _, test := range testResources {
			By(fmt.Sprintf("Starting: %s ", test.Scenario.Scenario))
			if ContainsModules(test.Scenario.Modules, installedModules) == false {
				continue;
			}

			for _, stepName := range test.Scenario.Steps {
				By(fmt.Sprintf("%s Executing  %s", beautify, stepName))
				Eventually(func() error {
					return stepRunner.RunStep(stepName, test)
				}, timeout, interval).Should(BeNil())
			}
			By(fmt.Sprintf("Ending: %s ", test.Scenario.Scenario))
			By("")
			By("")
			time.Sleep(5 * time.Second)
		}
	})
})
