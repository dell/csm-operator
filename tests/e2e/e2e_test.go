package e2e

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	step "github.com/dell/csm-operator/tests/e2e/steps"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"
	k8sConfig "k8s.io/kubernetes/test/e2e/framework/config"
)

const (
	timeout  = time.Minute * 1
	interval = time.Second * 2
)

var (
	testResources []step.Resource
	stepRunner    *step.Runner
	beautify      string
)

const kubeconfigEnvVar = "KUBECONFIG"

// TestE2E -
func TestE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	initializeFramework()
	RegisterFailHandler(Fail)
	RunSpecs(t, "CSM Operator End-to-End Tests")
}

func initializeFramework() {
	// k8s.io/kubernetes/tests/e2e/framework requires env KUBECONFIG to be set
	// it does not fall back to defaults
	if os.Getenv(kubeconfigEnvVar) == "" {
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		os.Setenv(kubeconfigEnvVar, kubeconfig)
	}
	framework.AfterReadingAllFlags(&framework.TestContext)

	k8sConfig.CopyFlags(k8sConfig.Flags, flag.CommandLine)
	framework.RegisterCommonFlags(flag.CommandLine)
	flag.Parse()
}

var _ = BeforeSuite(func() {
	By("Getting test environment variables")
	valuesFile := os.Getenv("E2E_VALUES_FILE")
	Expect(valuesFile).NotTo(BeEmpty(), "Missing environment variable required for tests. E2E_VALUES_FILE must both be set.")

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
