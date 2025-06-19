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

package steps

import (
	"fmt"
	"reflect"
	"regexp"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StepDefinition - definition of a step
type StepDefinition struct {
	Handler reflect.Value
	Expr    *regexp.Regexp
}

// Runner -
type Runner struct {
	Definitions []StepDefinition
}

var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

// StepRunnerInit -
func StepRunnerInit(runner *Runner, ctrlClient client.Client, clientSet *kubernetes.Clientset) {
	step := Step{
		ctrlClient: ctrlClient,
		clientSet:  clientSet,
	}
	runner.addStep(`^Given an environment with k8s or openshift, and CSM operator installed$`, step.validateTestEnvironment)
	runner.addStep(`^Install \[([^"]*)\]$`, step.installThirdPartyModule)
	runner.addStep(`^Uninstall \[([^"]*)\]$`, step.uninstallThirdPartyModule)
	runner.addStep(`^Apply custom resource \[(\d+)\]$`, step.applyCustomResource)
	runner.addStep(`^Validate \[(\d+)\] CSM has forceRemoveDriver set to true$`, step.validateForceRemoveDriverEnabled)
	runner.addStep(`^Validate \[(\d+)\] CSM has forceRemoveDriver set to false$`, step.validateForceRemoveDriverDisabled)
	runner.addStep(`^Upgrade from custom resource \[(\d+)\] to \[(\d+)\]$`, step.upgradeCustomResource)
	runner.addStep(`^Validate custom resource \[(\d+)\]$`, step.validateCustomResourceStatus)
	runner.addStep(`^Validate \[([^"]*)\] driver from CR \[(\d+)\] is installed$`, step.validateDriverInstalled)
	runner.addStep(`^Validate \[([^"]*)\] driver spec from CR \[(\d+)\]$`, step.validateMinimalCSMDriverSpec)
	runner.addStep(`^Validate \[([^"]*)\] driver from CR \[(\d+)\] is not installed$`, step.validateDriverNotInstalled)

	runner.addStep(`^Run custom test$`, step.runCustomTest)         // legacy support - original e2e was designed only to run ONE custom test
	runner.addStep(`^Run \[([^"]*)\]$`, step.runCustomTestSelector) // support for multiple custom tests
	runner.addStep(`^Enable forceRemoveDriver on CR \[(\d+)\]$`, step.enableForceRemoveDriver)
	runner.addStep(`^Enable forceRemoveModule on CR \[(\d+)\]$`, step.enableForceRemoveModule)
	runner.addStep(`^Delete custom resource \[(\d+)\]$`, step.deleteCustomResource)

	runner.addStep(`^Validate \[([^"]*)\] module from CR \[(\d+)\] is installed$`, step.validateModuleInstalled)
	runner.addStep(`^Validate \[([^"]*)\] module from CR \[(\d+)\] is not installed$`, step.validateModuleNotInstalled)
	runner.addStep(`^Validate \[([^"]*)\] module pods from CR \[(\d+)\] is not installed$`, step.validateAuthorizationPodsNotInstalled)

	runner.addStep(`^Enable \[([^"]*)\] module from CR \[(\d+)\]$`, step.enableModule)
	runner.addStep(`^Disable \[([^"]*)\] module from CR \[(\d+)\]$`, step.disableModule)

	runner.addStep(`^Set \[([^"]*)\] node label$`, step.setNodeLabel)
	runner.addStep(`^Remove \[([^"]*)\] node label$`, step.removeNodeLabel)

	runner.addStep(`^Set secret for driver from CR \[(\d+)\] to \[([^"]*)\]$`, step.setDriverSecret)
	runner.addStep(`^Create Secret with template \[([^"]*)\] name \[([^"]*)\] in namespace \[([^"]*)\] for \[([^"]*)\]`, step.setUpSecret)
	runner.addStep(`^Generate and Create SFTP Secrets from template \[([^"]*)\] private-secret \[([^"]*)\] public-secret \[([^"]*)\] in namespace \[([^"]*)\] for \[([^"]*)\]$`, step.generateAndCreateSftpSecrets)
	runner.addStep(`^Create ConfigMap with template \[([^"]*)\] name \[([^"]*)\] in namespace \[([^"]*)\] for \[([^"]*)\]`, step.setUpConfigMap)
	runner.addStep(`^Create resource with template \[([^"]*)\] for \[([^"]*)\]`, step.createResource)
	runner.addStep(`^Create StorageClass with template \[([^"]*)\] for \[([^"]*)\]`, step.setUpStorageClass)
	runner.addStep(`^Create \[([^"]*)\] prerequisites from CR \[(\d+)\]$`, step.createPrereqs)
	runner.addStep(`^Set up ephemeral volume properties \[([^"]*)\] for \[([^"]*)\]`, step.setupEphemeralVolumeProperties)

	// Configure authorization-proxy-server for [powerflex]
	runner.addStep(`^Configure authorization-proxy-server for \[([^"]*)\] for CR \[(\d+)\]$`, step.configureAuthorizationProxyServer)
	// Authorization Proxy Server V2 additional steps
	runner.addStep(`^Install Authorization CRDs \[(\d+)\]$`, step.createCustomResourceDefinition)
	runner.addStep(`^Validate \[([^"]*)\] CRD for Authorization is installed$`, step.validateCustomResourceDefinition)
	runner.addStep(`^Delete Authorization CRs for \[([^"]*)\]$`, step.deleteAuthorizationCRs)
	runner.addStep(`^Delete Authorization CRDs \[(\d+)\]$`, step.deleteCustomResourceDefinition)
	runner.addStep(`^Set up application mobility CR \[([^"]*)\]$`, step.configureAMInstall)
	runner.addStep(`^Set up reverse proxy tls secret namespace \[([^"]*)\]`, step.setUpReverseProxy)
	runner.addStep(`^Set up reverse proxy tls secret with SAN namespace \[([^"]*)\]`, step.setUpTLSSecretWithSAN)
}

func (runner *Runner) addStep(expr string, stepFunc interface{}) {
	re := regexp.MustCompile(expr)

	v := reflect.ValueOf(stepFunc)
	typ := v.Type()
	if typ.Kind() != reflect.Func {
		panic(fmt.Sprintf("expected handler to be func, but got: %T", stepFunc))
	}

	if typ.NumOut() == 1 {
		typ = typ.Out(0)
		switch typ.Kind() {
		case reflect.Interface:
			if !typ.Implements(errorInterface) {
				panic(fmt.Sprintf("expected handler to return an error but got: %s", typ.Kind()))
			}
		default:
			panic(fmt.Sprintf("expected handler to return an error, but got: %s", typ.Kind()))
		}

	} else {
		panic(fmt.Sprintf("expected handler to return only one value, but got: %d", typ.NumOut()))
	}

	runner.Definitions = append(runner.Definitions, StepDefinition{
		Handler: v,
		Expr:    re,
	})
}

// RunStep - runs a step
func (runner *Runner) RunStep(stepName string, res Resource) error {
	// Support conditional execution: "If config.enableSftpSDC is true: ..."
	const conditionalPrefix = "If config.enableSftpSDC is true: "
	if len(stepName) > len(conditionalPrefix) && stepName[:len(conditionalPrefix)] == conditionalPrefix {
		if res.Scenario.Config["enableSftpSDC"] != "true" {
			// Skip the step if the config is not enabled
			fmt.Printf("Skipping conditional step (enableSftpSDC is not true): %s\n", stepName)
			return nil
		}
		// Run the actual step (remove the prefix)
		stepName = stepName[len(conditionalPrefix):]
	}

	for _, stepDef := range runner.Definitions {
		if stepDef.Expr.MatchString(stepName) {
			var values []reflect.Value
			groups := stepDef.Expr.FindStringSubmatch(stepName)

			typ := stepDef.Handler.Type()
			numArgs := typ.NumIn()
			if numArgs > len(groups) {
				return fmt.Errorf("expected handler method to take %d but got: %d", numArgs, len(groups))
			}

			values = append(values, reflect.ValueOf(res))
			for i := 1; i < len(groups); i++ {
				values = append(values, reflect.ValueOf(groups[i]))
			}

			res := stepDef.Handler.Call(values)
			if err, ok := res[0].Interface().(error); ok {
				fmt.Printf("\nerr: %+v\n", err)
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("no method for step: %s", stepName)
}

// RunStepClient - runs a step
func (runner *Runner) RunStepClient(stepName string, res Resource) error {
	for _, stepDef := range runner.Definitions {
		if stepDef.Expr.MatchString(stepName) {
			var values []reflect.Value
			groups := stepDef.Expr.FindStringSubmatch(stepName)

			typ := stepDef.Handler.Type()
			numArgs := typ.NumIn()
			if numArgs > len(groups) {
				return fmt.Errorf("expected handler method to take %d but got: %d", numArgs, len(groups))
			}

			values = append(values, reflect.ValueOf(res))
			for i := 1; i < len(groups); i++ {
				values = append(values, reflect.ValueOf(groups[i]))
			}

			res := stepDef.Handler.Call(values)
			if err, ok := res[0].Interface().(error); ok {
				fmt.Printf("\nerr: %+v\n", err)
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("no method for step: %s", stepName)
}
