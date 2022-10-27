/*
 Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/
package steps

import (
	"fmt"
	"reflect"
	"regexp"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StepDefinition -
type StepDefinition struct {
	Handler reflect.Value
	Expr    *regexp.Regexp
}

// Runner -
type Runner struct {
	Definitions []StepDefinition
}

var (
	errorInterface = reflect.TypeOf((*error)(nil)).Elem()
)

// StepRunnerInit -
func StepRunnerInit(runner *Runner, ctrlClient client.Client, clientSet *kubernetes.Clientset) {
	step := Step{
		ctrlClient: ctrlClient,
		clientSet:  clientSet,
	}
	runner.addStep(`^Given an environment with k8s or openshift, and CSM operator installed$`, step.validateTestEnvironment)
	runner.addStep(`^Apply custom resources$`, step.applyCustomResource)
	runner.addStep(`^Validate custom resources$`, step.validateCustomResourceStatus)
	runner.addStep(`^Validate \[([^"]*)\] driver is installed$`, step.validateDriverInstalled)
	runner.addStep(`^Validate \[([^"]*)\] driver is not installed$`, step.validateDriverNotInstalled)

	runner.addStep(`^Run custom test$`, step.runCustomTest)
	runner.addStep(`^Enable forceRemoveDriver on CR$`, step.enableForceRemoveDriver)
	runner.addStep(`^Delete resources$`, step.deleteCustomResource)
	runner.addStep(`^Enable forceRemoveDriver on CR$`, step.enableForceRemoveDriver)

	runner.addStep(`^Validate \[([^"]*)\] module is installed$`, step.validateModuleInstalled)
	runner.addStep(`^Validate \[([^"]*)\] module is not installed$`, step.validateModuleNotInstalled)

	runner.addStep(`^Enable \[([^"]*)\] module$`, step.enableModule)
	runner.addStep(`^Disable \[([^"]*)\] module$`, step.disableModule)

	runner.addStep(`^Set Driver secret to \[([^"]*)\]$`, step.setDriverSecret)
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

// RunStep -
func (runner *Runner) RunStep(stepName string, res Resource) error {
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
				return err
			}
			return nil
		}

	}

	return fmt.Errorf("no method for step: %s", stepName)

}
