# Testing for the CSM Operator

This directory contains the testing infrastructure and E2E test implementation for the csm-operator. There are two kinds of tests for the operator: [unit tests](#unit-tests) and [end-to-end (E2E)](#e2e-tests) tests.

## Table of Contents

- [Testing for the CSM Operator](#testing-for-the-csm-operator)
  - [Table of Contents](#table-of-contents)
  - [Unit Tests](#unit-tests)
  - [E2E Tests](#e2e-tests)
  - [Prerequisites](#prerequisites)
    - [Array Information](#array-information)
    - [Application Mobility Prerequisites](#application-mobility-prerequisites)
    - [Authorization Proxy Server Prerequisites](#authorization-proxy-server-prerequisites)
    - [Shared NFS Prerequisites](#shared-nfs-prerequisites)
  - [Run](#run)
    - [Scenarios File](#scenarios-file)
  - [Developing E2E Tests](#developing-e2e-tests)
  - [Directory Layout](#directory-layout)

## Unit Tests

The unit tests are quick, easy-to-run tests that verify the functionality of the methods in different packages. The only major requirement for unit tests is that Go is installed. The unit tests are automatically run by GitHub actions as part of any PR targeting the main branch, and must pass with sufficient coverage for the PR to be merged.

To run unit tests, go to the root directory of this project and run `make <component>-unit-test`. Components include `controller` (controllers package), `module` (modules package), and `driver` (drivers package).

## E2E Tests

The end-to-end tests test the functionality of the operator as a whole by installing different combinations of drivers and modules, enabling and disabling components, and verifying the installed Container Storage Modules pods successfully start. The E2E tests need to be run from the master node of a Kubernetes cluster. All test scenarios are specified in `tests/e2e/testfiles/scenarios.yaml` and are tagged by which test suite(s) they are a part of -- test suites include a test suite for each driver and module, as well as a `sanity` suite.

Any time changes made to the operator are being checked into the main branch, sanity tests should be run (they should take 20-30 minutes to complete, the very first run may take a few minutes more). In addition, if you have made any driver- or module-specific changes, (any changes in `pkg/drivers`, `pkg/modules`, `operatorconfig/driverconfig`, `operatorconfig/moduleconfig`, etc), please run the E2E tests specific to these components as well.

**Note :** To run E2E tests with minimal samples, use the `--minimal` flag when executing the `run-e2e-test.sh` script. This will utilize the `scenarios.yaml` file located at `tests/e2e/testfiles/minimal-testfiles/scenarios.yaml`, which contains the paths to the minimal sample test YAMLs.

## Prerequisites

- A supported environment where the Dell Container Storage Modules Operator is installed
- Fill in the environment variables in `array-info.env` ([more info below](#run)).
- The following namespaces need to be created beforehand:
  - `dell`
  - `authorization`
  - `proxy-ns`
  - (if running sanity, powerflex, or modules suites) `test-vxflexos`
  - (if running sanity, powerscale, or modules suites) `isilon`
  - (if running unity suite) `unity`
  - (if running powermax suite) `powermax`
  - (if running powerstore suite) `powerstore`
- For Authorization V2:
  - The following components must be installed on your cluster:
    - Secrets Store CSI Driver
    - Vault, along with its CSI Provider. You can use the `--install-vault` tag to set up the Vault.
      - Configure array system credentials and redis db credentials in Vault.

      You'll need to configure the `storage_csm_authorization_secret_provider_class.yaml` file in the `testfiles/authorization-templates` directory to point to your Vault instance and paths of credentials. For example:
      ```
        parameters:
          roleName: "csm-authorization"
          vaultAddress: "https://vault.default.svc.cluster.local:8400"
          vaultCACertPath: '/config/vault-ca.pem'
      ```
      where "vault" is the name of the vault service running.
- Dellctl needs to be installed
  - See [here](https://dell.github.io/csm-docs/docs/support/cli/#installation-instructions) for instructions
- In addition, for drivers that do not use the secret and storageclass creation steps, any required secrets, storageclasses, etc. will need to be created beforehand as well as required namespaces.
- Ginkgo v2 is installed. To install, go to `tests/e2e` and run the following commands:

```bash
go install github.com/onsi/ginkgo/v2/ginkgo@latest
go get github.com/onsi/gomega/...
```

### Array Information

For PowerFlex, Unity, PowerScale, PowerStore, Authorization, and Application-Mobility, system-specific information (array login credentials, system IDs, endpoints, etc.) need to be provided in e2e/array-info.env so that all the required resources (secrets, storageclasses, etc.) can be created by the tests. Example values have been inserted; please replace these with values from your system. Refer to [CSM documentation](https://dell.github.io/csm-docs/docs/) for any further questions about driver or module pre-requisites.

In the case of end-to-end tests that involve PowerFlex zoning functionality, a second PowerFlex array will be necessary with its credentials provided in e2e/array-info.env.

Please note that, if tests are stopped in the middle of a run, some files in `testfiles/*-templates` folders may remain in a partially modified state and break subsequent test runs. To undo these changes, you can run `git checkout -- <template file>`.

### Application Mobility Prerequisites

If running the Application Mobility e2e tests, (the sanity suite includes a few simple app-mobility tests), further setup must be done:

- have a MinIO object storage setup, with default credentials
  - At least 2 buckets set up, if instance only has one bucket, set ALT_BUCKET_NAME = BUCKET_NAME
- have all required licenses installed in your testing environment
- have the latest Application Mobility controller and plugin images
  The application-mobility repo has information on all of these pre-requisites up, including a script to install minio.

### Authorization Proxy Server Prerequisites

If running the Authorization proxy server e2e tests, further setup must be done:

- When running tests on an OpenShift cluster, update the `PROXY_HOST` entry in the driver CRs with `router-internal-default.openshift-ingress.svc.cluster.local`.

Notes:

- Authorization V2 scenarios only support PowerFlex, PowerStore, PowerScale and PowerMax
- Upgrade from Authorization V1 to V2 is not supported
- The required host entry `<master node IP> csm-authorization.com` is now automatically added to `/etc/hosts`, so no manual update is needed.

### Shared NFS Prerequisites

If running the Shared NFS e2e tests, further setup must be done:

- On each of the worker nodes, setup the nfs-server service and ensure that mounting with nfs4 is enabled.
- Install the `sshpass` command on the VM where the tests are being run.
- Update the password within `scripts/node_credential` to the password of the worker nodes (this assumes that all nodes share the same password).
- During e2e execution specify the `sharednfs` flag. Otherwise, the tests won't run on its own due to the above prerequisite.

## Run

The tests are run by the `run-e2e-test.sh` script in the `tests/e2e` directory.

- Ensure you meet all [prerequisites](https://github.com/dell/csm-operator/blob/main/tests/README.md#prerequisites).
- Change to the `tests/e2e` directory.
- Create a file named `array-info.env` and populate it with your array information. Use `array-info.env.sample` as a template.
- If you do not have `dellctl` (for app-mobility and authorization proxy server) accessible through your `PATH` variable, pass the path to the executable to the script, like so, `run-e2e-test.sh --dellctl=/path/to/dellctl`, and they will be added to `/usr/local/bin`
- Decide on the test suites you want to run, based on the changes made. Available test suites can be seen by running `run-e2e-test.sh -h` If multiple suites are specified, the union (not intersection) of those suites will be run.
- Run the e2e tests by executing the `run-e2e-test.sh` script with desired options. Three examples are provided:

You have made changes to `controllers/csm_controller.go` and `pkg/drivers/powerflex.go`, and need to run sanity and powerflex test suites:

```bash
run-e2e-test.sh --sanity --powerflex
```

You made some changes to `controllers/csm_controller.go`, and need to run sanity tests:

```bash
run-e2e-test.sh --sanity
```

You made some changes to `pkg/modules/observability.go`, and need to run observability tests:

```bash
run-e2e-test.sh --obs
```

### Scenarios File

An e2e test scenarios file is a yaml file that defines all e2e test scenarios to be run. An excerpt of the file is shown below:

```yaml
- scenario: 'Install PowerScale Driver(Standalone)'
  path: 'testfiles/storage_csm_powerscale.yaml'
  tags:
    - powerscale
    - sanity
  steps:
    - 'Given an environment with k8s or openshift, and CSM operator installed'
    - 'Apply custom resources'
    - 'Validate custom resources'
    - 'Validate [powerscale] driver is installed'
    - 'Run custom test'
    # Last two steps perform Clean Up
    - 'Enable forceRemoveDriver on CR'
    - 'Delete resources'
  customTest:
    - name: Hello World # name of custom test to run
      # Provide command-line argument to run. Ginkgo will run the command and return output
      # The command should be accessible from e2e_tes repo.
      # Example:
      #   ./hello_world.sh
      run:
        - ./hello_world.sh
```

Each test has:

- `scenario`: The name of the test to run
- `path`: The path to the custom resources yaml file that has the specific configuration you want to test.
- `tags`: Each test can belong to one or more groups of tests, specified by tags. To see a list of currently available tags, run `./run-e2e-test.sh -h`.
- `steps`: Steps to take for the specific scenarios. Please note that all steps above and the ones in this sample file `tests/e2e/testfiles/values.yaml` already have a backend implementation. If you desire to use a different step, see [Develop](#develop) for how to add new E2E Test
- `customTest`: An array of entrypoints for users to run custom tests against their environment. There are two methods of running custom tests.

  - You may have `"Run custom test"` as part of your `steps` above if there is _only one custom test in the array of tests_.
  - You may have `"Run [Test Name]"`as part of your `steps` above to _select a custom test from the array by its name_

  An object in this array has the following parameters:

  - `name`: Name of your custom test
  - `run`: A list of command line arguments that will be run by the e2e test.

## Developing E2E Tests

Most steps to cover common use cases already have their respective backend implementations. Sometimes we run into a situation where we may need to add a new step. For the sake of illustration, please follow the constraints and steps below to add a new test scenario called `"Install PowerHello Driver(With a module called World)"` to excerpt of yaml file shown above.

Note: Please be mindful when updating upgrade scenarios for Authorization Proxy Server. We do not support upgrade from V1 to V2 versions.

- Add the new test scenario to the existing values file

  ```yaml
  - scenario: 'Install PowerScale Driver(Standalone)'
    path: '<path-to-cr-for-powerscale-with-auth-disabled>'
    steps:
      - 'Given an environment with k8s or openshift, and CSM operator installed'
      - 'Apply custom resources'
      - 'Validate custom resources'
      - 'Validate [powerscale] driver is installed'
      - 'Run custom test'
      # Last two steps perform Clean Up
      - 'Enable forceRemoveDriver on CR'
      - 'Delete resources'
    customTest:
      # name of custom test to run
      name: Cert CSI
      # Provide command-line argument to run. Ginkgo will run the command and return output
      # The command should be accessible from e2e test repo.
      # Example:
      #   ./hello_world.sh
      run: ./hello_world.sh

  - scenario: 'Install PowerHello Driver(With a module called World)'
    path: '<path-to-cr-for-powerhello-with-world-enabled>'
    steps:
      - 'Given an environment with k8s or openshift, and CSM operator installed'
      - 'Apply custom resources' # fully recycled old step
      - 'Validate custom resources'
      - 'Validate [powerhello] driver is installed' # partially recycled old step
      - 'Validate [world] module is installed'
      - 'Validate Today is Tuesday' # a new simple step without a backend implementation
      - 'Validate it is [raining], [snowing], [sunny], and [pay-day]' # a new templated step without a backend implementation
      - 'Run custom test'
      # Last two steps perform Clean Up
      - 'Enable forceRemoveDriver on CR'
      - 'Delete resources'
    customTest:
      name: 'Hello World'
      run: echo HelloWorld
  ```

- Add backend Support: we will cover three case:

  1.  `Fully recycled old step`: If the desired steps has already been covered in another test scenario, just copy line for line. You don't need any code change
  2.  `partially recycled old step`: In this case, a very similar test has already been cover. This means there's already an entrypoint in `steps`. You should review the `StepRunnerInit` function at [step_runner.go](https://github.com/dell/csm-operator/blob/main/tests/e2e/steps/steps_runner.go) and trace the implementation function that matches your step. For example the step `"Validate [world] module is installed"`. The line that matches this in `StepRunnerInit` is `runner.addStep(`^Validate \[([^"]\*)\] module is installed$`, step.validateModuleInstalled)`. The implementation to trace is `step.validateModuleInstalled`. We will review the implementation function in [steps_def.go](https://github.com/dell/csm-operator/blob/main/tests/e2e/steps/steps_def.go) to decide whether or not we need to do anything. Make the code changes if needed.
  3.  `new step`: New step can be simple such as `"Validate Today is Tuesday"` or a templated such as `["Validate it is [raining]"](https://example.com)`. The workflow to support these two cases is the same and only varies in the signature of the implementation function. The workflow include:

      1. Implement steps in [steps_def.go](https://github.com/dell/csm-operator/blob/main/tests/e2e/steps/steps_def.go). Define a function to implement your step. Note that the steps are stateless! If you want to define a function to test a happy path, your function should return nil if no error occurs and error otherwise. However, if you want to test an error path, your function should return nil if you get error and error otherwise. The constraints of all functions in step_def.go is as follows:

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
                Takes four more arguments for each group as defined here "Validate it is [raining], [snowing], [sunny], and [pay-day]".
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

## Directory Layout

`config` directory includes yaml files consumed by test cases. For example `driverconfig/powerscale/v2.x.y/node.yaml` is consumed by `pkg/drivers/commonconfig_test.go`.

`shared/clientgoclient` implements kubernetes client from client-go package. It has a getter function for each API version like `AppsV1Interface` or `CoreV1Interface`. `AppsV1Interface` is the one that we need as it has getter function for `daemonsetInterface`. The `daemonsetInterface` has all `Create`, `Apply`, `Delete` etc. methods that we will be using to manipulate Kubernetes runtime objects.

`shared/crclient` implements kubernetes client from controller runtime. It has very similar functionalities as the one above except that it can't do apply. These two clients share the same memory to store runtime objects.