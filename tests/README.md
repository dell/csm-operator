# Test for CSM Operator
This is the test directory for CSM operator. 

`config` directory includes yaml files consumed by test cases. For example `driverconfig/powerscale/v2.1.0/node.yaml` is consumed by `pkg/drivers/commonconfig_test.go`.

`shared/clientgoclient` implements kubernetes client from client-go package. It has a getter function for each API version like `AppsV1Interface` or `CoreV1Interface`. `AppsV1Interface` is the one that we need as it has getter function for `daemonsetInterface`. The `daemonsetInterface` has all `Create`, `Apply`, `Delete` etc. methods that we will be using to manipulate Kubernetes runtime objects.

`shared/crclient` implements kubernetes client from controller runtime. It has very similar functionalities as the one above except that it can't do apply. These two clients share the same memory to store runtime objects.

## Unit Test
To run unit test, go to the root directory of this project and run `make unit-test`. It will output a report of tests being run.

## E2E Test

At the moment, the E2E test only test the installation of Dell CSI Drivers and Dell CSM Modules. This mean, to run the test you MUST have the following pre-req

### Prerequisite

- A supported environment where Dell CSM Operator is running
- All prerequisite for a specific driver and modules to test. For documentation, please visit [Container Storage Modules documentation](https://dell.github.io/csm-docs/)


### Run

To run e2e test, go through the following steps:

- Ensure you meet all [prerequisites](#prerequisite)
- Change to the root directory of this project
- Copy or modify the values file at [`tests/e2e/testfiles/values.yaml`](https://github.com/dell/csm-operator/blob/main/tests/e2e/testfiles/values.yaml). Review [Values File](#values-file) for more information.
- Change to the `scripts` directory
- Set your environment variables in the file `env-e2e-test.sh`. You MUST set `VALUES_FILE` to point to the path to the `values.yaml` we created above.
- Run the e2e test by executing the commands below:

```bash
   source env-e2e-test.sh
   source run-e2e-test.sh
```


#### Values File

An e2e test values file is a yaml file that defines all e2e tests to be ran. An excerpt of the file is shown below:

```yaml
- scenario: "Install PowerScale Driver(Standalone)"
  path: "<path-to-cr-for-powerscale-with-auth-disabled>"
  steps:
    - "Given an environment with k8s or openshift, and CSM operator installed"
    - "Apply custom resources"
    - "Validate custom resources"
    - "Validate [powerscale] driver is installed"
    - "Run custom test"
    # Last two steps perform Clean Up
    - "Enable forceRemoveDriver on CR"
    - "Delete resources"
  customTest:
    # name of custom test to run
    name: Cert CSI
    # Provide command-line argument to run. Ginkgo will run the command and return output
    # The command should be accessible from e2e test repo. 
    # Example:
    #   ./hello_world.sh
    #   cert-csi test vio --sc <storage class> --chainNumber 2 --chainLength 2
    run: cert-csi test vio --sc isilon-plain --chainNumber 2 --chainLength 2
```

Each test has:

- `scenario`: The name of the test to run
- `path`: The path to the custom resources yaml file that has the specific configuration you want to test.
- `steps`: Steps to take for the specific scenearios. Please note that all steps above and the oe ones in this sample file `tests/e2e/testfiles/values.yaml` already have a backend implementation. If you desire to use a different step, see [Develop](#develop) for how to add new E2E Test
- `customTest`: An entrypoint for users to run custom test against their environment. You Must have `"Run custom test"` as part of your `steps` above for this custom test to run. This object has the following parameter.
   - `name`: Name of your custom test
    - `run`: A command line argument that wil be ran by the e2e test. To ensure the command is accessible from e2e test repo, use absolute paths if you are running a script. 


### Develop

Most steps to cover common use cases already have their respective backend implementations. Sometimes we run into a situation where we may need to add a new step. For the sake of illustration, please follow the constraints and steps below to add a new test scenario called `"Install PowerHello Driver(With a module called World)"` to excerpt of yaml file shown above.

- Add the new test scenario to the existing values file

```yaml
- scenario: "Install PowerScale Driver(Standalone)"
  path: "<path-to-cr-for-powerscale-with-auth-disabled>"
  steps:
    - "Given an environment with k8s or openshift, and CSM operator installed"
    - "Apply custom resources"
    - "Validate custom resources"
    - "Validate [powerscale] driver is installed"
    - "Run custom test"
    # Last two steps perform Clean Up
    - "Enable forceRemoveDriver on CR"
    - "Delete resources"
  customTest:
    # name of custom test to run
    name: Cert CSI
    # Provide command-line argument to run. Ginkgo will run the command and return output
    # The command should be accessible from e2e test repo. 
    # Example:
    #   ./hello_world.sh
    #   cert-csi test vio --sc <storage class> --chainNumber 2 --chainLength 2
    run: cert-csi test vio --sc isilon-plain --chainNumber 2 --chainLength 2

- scenario: "Install PowerHello Driver(With a module called World)"
  path: "<path-to-cr-for-powerhello-with-world-enabled>"
  steps:
    - "Given an environment with k8s or openshift, and CSM operator installed"
    - "Apply custom resources" # fully re-cycled old step
    - "Validate custom resources"
    - "Validate [powerhello] driver is installed" # partially re-cycled old step
    - "Validate [world] module is installed"
    - "Validate Today is Tuesday"  # a new simple step without a backend implementation
    - "Validate it is [raining], [snowing], [sunny], and [pay-day]"  # a new templated step without a backend implementation
    - "Run custom test"
    # Last two steps perform Clean Up
    - "Enable forceRemoveDriver on CR"
    - "Delete resources"
  customTest:
    name: "Hello World"
    run: echo HelloWorld
```

- Add backend Support: we will cover three case:

   1. `Fully re-cycled old step`: If the desired steps is already covered in another test scenario, just copy line for line. You don't need any code chnage
   2. `partially re-cycled old step`: In this case, a very similar test has already been cover. This means there's already an entrypoint in `steps`. You should review the `StepRunnerInit` function at [step_runner.go](https://github.com/dell/csm-operator/blob/main/tests/e2e/steps/steps_runner.go) and trace the implementation function that matches your step. For example  the step `"Validate [world] module is installed"`. The line that  matches this in `StepRunnerInit` is `runner.addStep(`^Validate \[([^"]*)\] module is installed$`, step.validateModuleInstalled)`. The implementation to trace is `step.validateModuleInstalled`. We will review the implementation function in [steps_def.go](https://github.com/dell/csm-operator/blob/main/tests/e2e/steps/steps_def.go) to decide whether or not we need to do anything. Make the code changes if needed.
   3. `new step`:  New step can be  simple such as `"Validate Today is Tuesday"` or a templated such as `["Validate it is [raining]"](https://jira.cec.lab.emc.com/browse/KRV-3130)`. The workflow to support these two cases is the same and only varies in the signature of the implementation function. The workflow include:
        1. Implement steps in [steps_def.go](https://github.com/dell/csm-operator/blob/main/tests/e2e/steps/steps_def.go). Define a function to implement your step. Note that the steps are stateless! If you want to define a function to test a happy path, your function should return nil if no error occurs and error otherwise. If however you want to test an error path, your function should return nil if you get error and error otherwise. The constraints of all functions in step_def.go is as follows:
             - must return `error` or `nil`
             - must take at least one argument. The first one MUST be type `Resource`(even though it may not be used). If your step has any group(a groups is anything in your step enclosed by `[]`), the remaining arguments should be the groups in the order they appear on the steps(from left to right). For example, the two new functions above will can be implemented as shown below:
             
               ```go
                // take one argument, res, and return error
                func (step *Step) isTodayTuesday(res Resource) error {
                    weekday := time.Now().Weekday()
                    if weekday != "Tuesday" {
                        return fmt.Errorf("expected weekday to be Tuesday. Got: %s", weekday)
                    }
                    return nil
                }

                /*
                    Take four more arguments for each groups as defined here "Validate it is [raining], [snowing], [sunny], and [pay-day]". 
                    Thus function wll be  automatically called with:
                            checkWeather(Resource{}, "raining", "snowing", "sunny", "pay-day")
                    Please see "Validate [powerhello] driver is installed" step and the function signature that implemented it
                */
                func (step *Step) checkWeather(res Resource, weather1, weather2, weather3, weather4) error {
                    allValidWeather := true
                    allValid = allValid && (weather1 =="raining")
                    allValid = allValid && (weather1 =="snowing")
                    allValid = allValid && (weather1 =="sunny")
                    allValid = allValid && (weather1 =="pay-day")
                    if !allValid {
                        return fmt.Errorf("not all weather is valid")
                    }

                    return nil
                }
               ```

        2. Register your new steps in `StepRunnerInit` function at [step_runner.go](https://github.com/dell/csm-operator/blob/main/tests/e2e/steps/steps_runner.go). Please pay special attention to the regex and ensure they actually match your new steps. For instance, the new steps we implemented above can be mapped to their steps in the valus file as follows:
           
            ```go
            func StepRunnerInit(runner *Runner, ctrlClient client.Client, clientSet *kubernetes.Clientset) {
                step := Step{
                    ctrlClient: ctrlClient,
                    clientSet:  clientSet,
                }
                // ... old steps
                runner.addStep(`^Validate Today is Tuesday$`, step.isTodayTuesday)
                runner.addStep(`^^Validate it is \[([^"]*)\], \[([^"]*)\], \[([^"]*)\], and \[([^"]*)\]$`, step.checkWeather)
            }
            ```

        3. [Run your E2E](#run). If you get this error `no method for step: <you step>`, it means you either haven't implemented it or there's a problem with your regex.
