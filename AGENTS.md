# AGENTS.md -- Dell CSM Operator

## Project Overview

Dell Container Storage Modules (CSM) Operator is a Kubernetes operator built with Kubebuilder v4 and Go 1.26. It manages the lifecycle of Dell CSI drivers and CSM modules on Kubernetes clusters. The operator watches a custom resource (`ContainerStorageModule`, API group `storage.dell.com/v1`, namespaced) and reconciles the desired state by deploying, upgrading, or removing driver and module components.

Supported CSI drivers: PowerFlex, PowerMax, PowerScale, PowerStore, Unity, COSI.
Supported modules: Authorization (v1 and v2), Observability, Replication, Resiliency, Reverse Proxy.

Module path: `github.com/dell/csm-operator`

Key dependencies: controller-runtime v0.22.4, k8s.io/api v0.35.0, cert-manager v1.19.3, go.uber.org/zap (structured logging), stretchr/testify (test assertions), Ginkgo v2 (E2E tests).

## Architecture

### Reconciliation Loop

The core logic lives in `controllers/csm_controller.go` (ContainerStorageModuleReconciler). On each reconciliation:

1. The reconciler reads the ContainerStorageModule CR.
2. It selects the correct driver or module configuration from YAML templates in `operatorconfig/`.
3. It performs template substitution (image names, versions, feature flags) to produce Kubernetes manifests.
4. It applies those manifests through typed resource handlers in `pkg/resources/` (one handler per resource kind: Deployment, DaemonSet, ConfigMap, CSIDriver, RBAC, ServiceAccount).
5. It updates the CR status to reflect the current state.

### Driver and Module Implementations

Each driver has a dedicated file in `pkg/drivers/` (e.g., `powerflex.go`, `powermax.go`). Each module has a dedicated file in `pkg/modules/` (e.g., `authorization.go`, `observability.go`). These files contain the logic to parse the CR spec, validate configuration, and produce the set of Kubernetes objects to reconcile.

Common configuration logic shared across drivers or modules lives in `commonconfig.go` in the respective package.

### Operator Config Templates

The `operatorconfig/` directory contains YAML templates that define the Kubernetes resources for each driver and module version:

- `operatorconfig/driverconfig/{driver}/{version}/` -- Per-driver versioned configs. Each version directory contains `controller.yaml`, `node.yaml`, `csidriver.yaml`, `driver-config-params.yaml`, and `upgrade-path.yaml`.
- `operatorconfig/driverconfig/common/` -- Shared driver configuration (e.g., `default.yaml`).
- `operatorconfig/moduleconfig/{module}/` -- Per-module configurations for authorization, observability, replication, resiliency, and csireverseproxy.

When adding a new driver or module version, create a new version directory under the appropriate path and update the upgrade path file.

### Resource Handlers

`pkg/resources/` contains sub-packages for each Kubernetes resource type the operator manages: `deployment/`, `daemonset/`, `configmap/`, `csidriver/`, `rbac/`, `serviceaccount/`. These implement creation, update, and deletion logic and are called by the reconciler.

### Utilities

`pkg/operatorutils/` provides core utility functions (`utils.go`, `common.go`, `status.go`) for manifest parsing, version selection, status management, and common operations used throughout the codebase.

## Build Commands

All commands use the top-level Makefile.

| Command | Description |
|---|---|
| `make build` | Build the manager binary (with `vendor`, outputs `bin/manager`) |
| `make test` | Run all tests with coverage (generates CRDs first) |
| `make unit-test` | Run unit tests with 90% coverage threshold |
| `make controller-unit-test` | Unit tests for `controllers/` only |
| `make driver-unit-test` | Unit tests for `pkg/drivers/` only |
| `make module-unit-test` | Unit tests for `pkg/modules/` only |
| `make operatorutils-unit-test` | Unit tests for `pkg/operatorutils/` only |
| `make manifests` | Generate CRDs and RBAC manifests (controller-gen v0.18.0) |
| `make generate` | Generate DeepCopy/DeepCopyInto/DeepCopyObject methods |
| `make fmt` | Run `go fmt ./...` |
| `make vet` | Run `go vet ./...` |
| `make lint` | Run golangci-lint (builds first) |
| `make run` | Run the controller locally against the current kubeconfig |
| `make deploy` | Deploy the operator to the cluster |
| `make undeploy` | Remove the operator from the cluster |
| `make install` | Install CRDs into the cluster |
| `make uninstall` | Remove CRDs from the cluster |
| `make bundle` | Generate OLM bundle manifests |
| `make images` | Build the container image |
| `make static-manifests` | Generate static CRD and operator YAML in `deploy/` |
| `make vendor` | Run `go mod vendor` (requires GOPRIVATE for internal modules) |
| `make gen-semver` | Regenerate semantic version files in `core/` |
| `make tidy` | Run `go mod tidy` for both the main module and `tests/e2e/` |

The `gen-semver` target generates version information from `core/semver.tpl` and is a prerequisite for `build`, `test`, `run`, `bundle`, and image targets.

## Testing

### Unit Tests

Unit tests live alongside source files with the `_test.go` suffix. The project enforces a 90% code coverage threshold via the `unit-test` target.

Run all unit tests:
```
make unit-test
```

Run component-specific tests:
```
make controller-unit-test
make driver-unit-test
make module-unit-test
make operatorutils-unit-test
```

Tests use `stretchr/testify` for assertions. Controller tests use `setup-envtest` with Kubernetes 1.31 API server binaries.

### E2E Tests

End-to-end tests are in `tests/e2e/` with a separate Go module. They use Ginkgo v2 and Gomega and are organized as scenario-driven step definitions:

- `tests/e2e/e2e_test.go` -- Test entry point
- `tests/e2e/steps/` -- Step definitions (`steps_def.go`, `steps_runner.go`, `step_common.go`)
- `tests/e2e/testfiles/` -- Test CR manifests
- `tests/e2e/scripts/` -- Helper scripts

E2E tests require a running Kubernetes cluster with storage arrays configured. To run them:

1. Copy `tests/e2e/array-info.yaml.sample` to `tests/e2e/array-info.yaml` and fill in your array credentials.
2. Run the test script with a suite flag:
```
cd tests/e2e && ./run-e2e-test.sh --sanity
```

Available suite flags: `--sanity`, `--pflex`, `--pscale`, `--pstore`, `--pmax`, `--unity`, `--cosi`, `--auth`, `--auth-proxy`, `--replication`, `--obs`, `--resiliency`, `--zoning`, `--no-modules`, `--minimal`. Run `./run-e2e-test.sh -h` for full usage.

The script handles environment setup, namespace creation, prerequisite checks, and Ginkgo invocation. Do not invoke `go test` directly — the script sets required environment variables and scenario files.

Test suites cover: sanity, powerflex, powermax, powerscale, powerstore, unity, cosi, and modules.

## Code Style

- Format all Go code with `go fmt` before committing (`make fmt`).
- Run `go vet` to catch common issues (`make vet`).
- Run `golangci-lint` via `make lint`.
- Use structured logging with `go.uber.org/zap` via the logger package at `pkg/logger/`. Do not use `fmt.Println` or the standard `log` package in production code.
- Wrap errors with `fmt.Errorf("context: %w", err)` to preserve error chains.
- All new exported types in `api/v1/` must have JSON struct tags. The CRD types use kubebuilder markers for validation (see `csm_types.go` for CEL validation rules).
- The Apache 2.0 license header is required in all Go source files. The boilerplate is at `hack/boilerplate.go.txt`.
- Kubernetes version-specific configurations go in `operatorconfig/driverconfig/common/` and version directories under each driver.

## Directory Structure

```
main.go                          Entry point; sets up manager, scheme, reconciler
main_test.go                     Tests for main
api/v1/                          CRD type definitions
  csm_types.go                   ContainerStorageModuleSpec/Status with kubebuilder markers
  types.go                       Shared types (driver types, module types, enums)
  groupversion_info.go           GVK registration
  zz_generated.deepcopy.go       Generated DeepCopy methods (do not edit)
controllers/
  csm_controller.go              Main reconciliation logic (ContainerStorageModuleReconciler)
  csm_controller_test.go         Controller unit tests
pkg/
  drivers/                       Per-driver reconciliation logic
    powerflex.go                 PowerFlex driver
    powermax.go                  PowerMax driver
    powerscale.go                PowerScale driver
    powerstore.go                PowerStore driver
    unity.go                     Unity driver
    cosi.go                      COSI driver
    commonconfig.go              Shared driver config utilities
  modules/                       Per-module reconciliation logic
    authorization.go             Authorization module (v1/v2)
    observability.go             Observability module
    replication.go               Replication module
    resiliency.go                Resiliency module
    reverseproxy.go              Reverse Proxy module
    commonconfig.go              Shared module config utilities
  operatorutils/                 Core utilities (manifest parsing, version selection, status)
    utils.go                     Main utility functions
    common.go                    Common helpers
    status.go                    CR status management
  resources/                     Kubernetes resource handlers
    deployment/                  Deployment create/update/delete
    daemonset/                   DaemonSet create/update/delete
    configmap/                   ConfigMap create/update/delete
    csidriver/                   CSIDriver create/update/delete
    rbac/                        ClusterRole/ClusterRoleBinding handling
    serviceaccount/              ServiceAccount handling
  constants/                     Package-level constants
  logger/                        Structured logging setup (zap)
operatorconfig/                  YAML templates for drivers and modules
  driverconfig/
    common/                      Shared driver defaults
    powerflex/{version}/         PowerFlex version configs
    powermax/{version}/          PowerMax version configs
    powerscale/{version}/        PowerScale version configs
    powerstore/{version}/        PowerStore version configs
    unity/{version}/             Unity version configs
    cosi/{version}/              COSI version configs
  moduleconfig/
    authorization/               Authorization module configs
    observability/               Observability module configs
    replication/                 Replication module configs
    resiliency/                  Resiliency module configs
    csireverseproxy/             Reverse Proxy module configs
  common/                        Common operatorconfig
config/                          Kustomize overlays
  crd/                           CRD kustomization
  manager/                       Manager deployment
  rbac/                          RBAC definitions
  samples/                       Sample CR manifests
  manifests/                     OLM manifest generation base
  default/                       Default kustomization
  install/                       Install kustomization
core/                            Version generation (semver)
k8s/                             Kubernetes client utilities
deploy/                          Generated static deployment manifests
  crds/                          Generated CRD YAML
tests/
  e2e/                           E2E test suite (separate Go module, Ginkgo v2)
    steps/                       Step definitions for scenario tests
    testfiles/                   Test CR manifests
    scripts/                     E2E helper scripts
  config/                        Test configuration
  sharedutil/                    Shared test utilities
bundle/                          OLM bundle artifacts
catalog/                         OLM catalog artifacts
samples/                         Example CR manifests
docs/                            Project documentation (CONTRIBUTING, SECURITY, etc.)
hack/                            Build utilities (boilerplate header)
```

## Configuration

### ContainerStorageModule CR

The primary configuration interface is the ContainerStorageModule custom resource. Samples are in `config/samples/` and `samples/`. Key spec fields:

- `spec.driver` -- Driver type, version, and configuration (common image, sidecars, env vars, node/controller settings).
- `spec.modules` -- List of modules to enable with per-module configuration and components.
- `spec.version` -- When set, the operator selects matching images automatically. Mutually exclusive with explicit image fields in driver/module specs (enforced via CEL validation).
- `spec.customRegistry` -- Override image registry (only valid when `spec.version` is set).

### Operator Config Versioning

Each driver version directory (e.g., `operatorconfig/driverconfig/powerflex/v2.17.0/`) contains:

- `controller.yaml` -- Controller Deployment template
- `node.yaml` -- Node DaemonSet template
- `csidriver.yaml` -- CSIDriver object template
- `driver-config-params.yaml` -- Driver-specific ConfigMap parameters
- `upgrade-path.yaml` -- Defines valid upgrade paths from previous versions

The operator selects the correct version directory based on the CR's `configVersion` or `version` field.

### Environment and Build Configuration

- `images.mk` -- Image names, tags, and registry settings.
- `overrides.mk` -- Local overrides (not committed).
- `helper.mk` -- `gen-semver`, `vendor`, and `download-csm-common` targets.

## Common Tasks

### Adding a New Driver Version

1. Create a new version directory under `operatorconfig/driverconfig/{driver}/` (e.g., `v2.18.0/`).
2. Add `controller.yaml`, `node.yaml`, `csidriver.yaml`, `driver-config-params.yaml`, and `upgrade-path.yaml`.
3. Update `upgrade-path.yaml` in the new version to reference the previous version as a valid upgrade source.
4. Update the driver implementation in `pkg/drivers/{driver}.go` if the new version introduces configuration changes.
5. Add or update unit tests in `pkg/drivers/{driver}_test.go`.
6. Run `make unit-test` and verify 90% coverage is maintained.

### Adding a New Module

1. Create the module configuration directory under `operatorconfig/moduleconfig/{module}/`.
2. Add a new implementation file in `pkg/modules/{module}.go`.
3. Add the module type to the enums in `api/v1/types.go`.
4. Update the reconciler in `controllers/csm_controller.go` to handle the new module type.
5. Run `make generate` to regenerate DeepCopy methods if types changed.
6. Run `make manifests` to regenerate CRDs.
7. Add unit tests and E2E test scenarios.

### Code Generation Workflow

When modifying CRD types in `api/v1/`:

1. Edit the type definitions in `api/v1/csm_types.go` or `api/v1/types.go`.
2. Run `make generate` to regenerate `zz_generated.deepcopy.go`. Do not edit this file manually.
3. Run `make manifests` to regenerate CRD YAML in `config/crd/bases/` and RBAC manifests.
4. Run `make static-manifests` to update the static deployment files in `deploy/`.
5. Run `make fmt && make vet` to verify formatting and correctness.

The `make generate` target runs controller-gen v0.18.0 with the `object` generator. The `make manifests` target runs controller-gen with `rbac` and `crd` generators. Both use the boilerplate header from `hack/boilerplate.go.txt`.

### Updating Kubernetes Version Support

1. Add or update version-specific config files in `operatorconfig/driverconfig/common/`.
2. Update `ENVTEST_K8S_VERSION` in the Makefile if the envtest binary version needs to change.
3. Test against the new Kubernetes version using `make test`.

### Building and Deploying

Build and run locally:
```
make build
make run
```

Deploy to a cluster:
```
make install    # Install CRDs
make deploy     # Deploy the operator
```

Build the container image:
```
make images
```

Generate OLM bundle:
```
make bundle
```
