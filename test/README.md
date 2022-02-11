# Test for CSM Operator
This is the test directory for CSM operator. 

`config` directory includes yaml files consumed by test cases. For example `driverconfig/powerscale/v2.1.0/node.yaml` is consumed by `pkg/drivers/commonconfig_test.go`.

`shared/clientgoclient` implements kubernetes client from client-go package. It has a getter function for each API version like `AppsV1Interface` or `CoreV1Interface`. `AppsV1Interface` is the one that we need as it has getter function for `daemonsetInterface`. Then this `daemonsetInterface` has all `Create`, `Apply`, `Delete` etc. methods that we will be using to manipulate Kubernetes runtime objects.

`shared/crclient` implements kubernetes client from controller runtime. It has very similar functionalities as the one above except that it can't do apply. These two clients share the same memory to store runtime objects.

## Unit Test
To run unit test, go to the root directory of this project and run `make unit-test`. It will output a report of tests being run.

