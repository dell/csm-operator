# CSM Operator E2E Test Guide

This document describes how to run end-to-end tests for the Dell Container Storage Modules (CSM) Operator. E2E tests exercise the full operator lifecycle -- installing drivers with various module combinations, upgrading, downgrading, enabling/disabling features -- and verify the resulting pods reach a healthy state on a real Kubernetes cluster.

## Table of Contents

- [Quick Start](#quick-start)
- [Prerequisites](#prerequisites)
- [Array Configuration (`array-info.yaml`)](#array-configuration-array-infoyaml)
  - [Structure](#structure)
  - [Section Loading Rules](#section-loading-rules)
- [Running Tests](#running-tests)
  - [Command-Line Reference](#command-line-reference)
  - [Platform and Module Flags](#platform-and-module-flags)
  - [Filtering Logic](#filtering-logic)
  - [Examples](#examples)
- [Minimal vs Full Test Suites](#minimal-vs-full-test-suites)
- [Test File Generation](#test-file-generation)
  - [On-Demand Generation from Samples](#on-demand-generation-from-samples)
  - [Minimal Test File Generation](#minimal-test-file-generation)
- [Scenario File Format](#scenario-file-format)
- [Automated Infrastructure Setup](#automated-infrastructure-setup)
  - [Namespace Management](#namespace-management)
  - [Vault and Secrets Store CSI Driver](#vault-and-secrets-store-csi-driver)
  - [Authorization Proxy Host](#authorization-proxy-host)
- [Execution Control and Reporting](#execution-control-and-reporting)
  - [Continue-on-Failure Mode](#continue-on-failure-mode)
  - [JUnit XML Reports](#junit-xml-reports)
  - [Step Timeouts](#step-timeouts)
- [Custom Registry Tests](#custom-registry-tests)
- [Developing E2E Tests](#developing-e2e-tests)
  - [Adding a New Scenario](#adding-a-new-scenario)
  - [Adding a New Step](#adding-a-new-step)
- [Troubleshooting](#troubleshooting)

---

## Quick Start

```bash
cd tests/e2e

# 1. Create array-info.yaml from the sample
cp array-info.yaml.sample array-info.yaml
# Edit array-info.yaml with your storage array credentials

# 2. Run sanity tests for a single driver
./run-e2e-test.sh --sanity --powerflex

# 3. Run all scenarios for one driver (standalone + all modules)
./run-e2e-test.sh --powerstore

# 4. Run minimal-sample tests for fast validation
./run-e2e-test.sh --minimal --powerstore
```

---

## Prerequisites

- **Kubernetes cluster** with the CSM Operator already installed (the operator pod must be running).
- **kubectl** configured to access the cluster (kubeconfig resolved via `--kube-cfg` flag, `KUBECONFIG` env, or `~/.kube/config`).
- **Go 1.22+** installed.
- **Ginkgo v2** installed. From the `tests/e2e` directory:
  ```bash
  go install github.com/onsi/ginkgo/v2/ginkgo@latest
  go get github.com/onsi/gomega/...
  ```
- **Helm 3** installed (required for Vault and Secrets Store CSI Driver auto-installation).
- **dellctl** installed and in your `PATH` (required for authorization tests). If not in `PATH`, pass `--dellctl=/path/to/dellctl`. See [CLI installation instructions](https://dell.github.io/csm-docs/docs/support/cli/#installation-instructions).
- **`array-info.yaml`** populated with your storage array credentials (see [below](#array-configuration-array-infoyaml)).

For **Authorization V2** tests additionally:
- Secrets Store CSI Driver (auto-installed by the script when auth tests run).
- Vault with CSI Provider (auto-installed by the script when auth tests run; can also be forced with `--install-vault`).
- For Conjur-based auth tests, use `--install-conjur`.

For **Custom Registry** tests:
- The images must be present in the registry path used by the test CRs. See [Custom Registry Tests](#custom-registry-tests).

---

## Array Configuration (`array-info.yaml`)

The test framework reads storage array credentials and configuration from `array-info.yaml`. Copy the sample as a starting point:

```bash
cp array-info.yaml.sample array-info.yaml
```

Fill in **only the sections for the platforms you are testing**. Empty values are silently skipped.

### Structure

The file is organized into YAML sections by platform and feature:

```yaml
# Always loaded
global:
  NS_PREFIX: "e2e"           # namespace prefix for all E2E namespaces

# Shared auth credentials (loaded when any auth test runs)
auth-common:
  REDIS_USER: ""
  REDIS_PASS: ""
  JWT_SIGNING_SECRET: ""

# Base platform sections (loaded when the platform is selected)
powerflex:
  POWERFLEX_USER: "admin"
  POWERFLEX_PASS: "Password123"
  POWERFLEX_SYSTEMID: "260e3a704877200f"
  POWERFLEX_ENDPOINT: "10.1.1.1"       # do not include https://
  POWERFLEX_MDM: "10.0.0.1,10.0.0.2"
  # ...

# Feature sections (loaded when BOTH platform AND feature are active)
powerflex-auth:
  POWERFLEX_AUTH_ENDPOINT: "localhost:9401"
  POWERFLEX_STORAGE: "powerflex"
  # ...

powerflex-zoning:
  POWERFLEX_ZONING_USER: ""
  # ...
```

**Available sections:** `global`, `auth-common`, `powerflex`, `powerflex-auth`, `powerflex-zoning`, `powerflex-oidc`, `powerflex-sftp`, `powerscale`, `powerscale-auth`, `powerscale-replication`, `powermax`, `powermax-auth`, `powermax-zoning`, `powerstore`, `powerstore-auth`, `unity`, `cosi`.

### Section Loading Rules

The `parse-array-info` tool (in `scripts/parse-array-info/`) reads `array-info.yaml` and exports only the sections relevant to the current test run:

| Section pattern | When loaded |
|---|---|
| `global` | Always |
| `auth-common` | When auth features are active |
| `powerflex` (base) | When `--powerflex` flag is set (or no flags = all platforms) |
| `powerflex-auth` | When `--powerflex` is active AND auth features are active |
| `powerflex-zoning` | When `--powerflex` is active AND `--zoning` flag is set |

This means you only need to fill in credentials for the platforms you intend to test. Sections for unselected platforms are completely ignored.

---

## Running Tests

All tests are run from the `tests/e2e` directory via `run-e2e-test.sh`:

```bash
cd tests/e2e
./run-e2e-test.sh [options...]
```

### Command-Line Reference

| Flag | Description |
|---|---|
| **Platforms** | |
| `--powerflex` | Run PowerFlex driver tests |
| `--powerscale` | Run PowerScale driver tests |
| `--powermax` | Run PowerMax driver tests |
| `--powerstore` | Run PowerStore driver tests |
| `--unity` | Run Unity driver tests |
| `--cosi` | Run COSI driver tests (opt-in, must be explicitly specified) |
| **Modules** | |
| `--auth` | Run authorization module tests |
| `--auth-proxy` | Run authorization proxy server V2 tests |
| `--obs` | Run observability module tests |
| `--replication` | Run replication module tests |
| `--resiliency` | Run resiliency module tests |
| `--zoning` | Run PowerFlex zoning tests (opt-in, requires multiple storage systems) |
| `--sftp` | Enable SFTP for PowerFlex tests (opt-in, loads `powerflex-sftp` config section) |
| `--no-modules` | Run driver-only tests (no module scenarios) |
| `--sanity` | Run sanity test subset |
| **Test files** | |
| `--minimal` | Use minimal-sample scenarios (faster, fewer permutations) |
| `--scenarios=<path>` | Specify a custom scenarios YAML file |
| `--add-tag=<tag>` | Include scenarios matching an additional custom tag |
| **Configuration** | |
| `--kube-cfg=<path>` | Path to kubeconfig (precedence: flag > `KUBECONFIG` env > `~/.kube/config`) |
| `--dellctl=<path>` | Path to dellctl binary (copied to `/usr/local/bin` if not already there) |
| **Infrastructure** | |
| `--install-vault` | Force Vault installation (auto-installed when auth tests run) |
| `--install-conjur` | Install Conjur instance for authorization tests |
| **Execution control** | |
| `--continue-on-fail` | Continue running after a scenario fails (default: stop on first failure) |
| `--no-cleanup-ns` | Keep test namespaces after the run (useful for debugging) |
| `--junit-report=<path>` | Write JUnit XML report to the given file |
| **Utility** | |
| `-h` | Print help text with examples |
| `-c` | Check prerequisites only (don't run tests) |
| `-v` | Enable verbose logging |

### Platform and Module Flags

Platform flags select which **drivers** to test. Module flags select which **features** (authorization, observability, etc.) to test. When combined, only scenarios matching **both** a selected platform **and** a selected module will run.

### Filtering Logic

| Flags given | Behavior |
|---|---|
| No flags | Run **all** scenarios for all platforms and modules |
| Platform only (`--powerstore`) | Run **all** module scenarios for that platform (standalone + auth + obs + resiliency + replication) |
| Module only (`--obs`) | Run observability scenarios for **all** platforms |
| Platform + module (`--powerstore --obs`) | Run only PowerStore observability scenarios |
| `--no-modules` | Run driver-only (standalone) scenarios, skip all module scenarios |
| `--sanity` | Run only scenarios tagged `sanity` |

**Opt-in tags:** `--zoning`, `--sftp`, and `--cosi` are opt-in -- they never run unless explicitly specified. The `--sftp` flag enables SFTP support for PowerFlex tests and loads the `powerflex-sftp` section from `array-info.yaml`. Similarly, `--auth` and `--auth-proxy` are considered "exclusive" module tags: when other module filters are active, auth scenarios only run if explicitly included.

### Examples

```bash
# All scenarios for all platforms and modules
./run-e2e-test.sh

# One platform, all modules
./run-e2e-test.sh --powerstore

# One platform, no modules (driver-only)
./run-e2e-test.sh --powerstore --no-modules

# One platform, one module
./run-e2e-test.sh --powerstore --auth-proxy

# One module across all platforms
./run-e2e-test.sh --obs

# Multiple platforms, one module
./run-e2e-test.sh --powerstore --powermax --resiliency

# Sanity subset of one platform
./run-e2e-test.sh --powermax --sanity

# Minimal scenarios (same filtering rules apply)
./run-e2e-test.sh --minimal --powerstore
./run-e2e-test.sh --minimal --powerstore --resiliency
./run-e2e-test.sh --minimal --auth-proxy

# CI mode: continue on failure and produce JUnit report
./run-e2e-test.sh --powerflex --continue-on-fail --junit-report=results.xml

# Run scenarios matching a custom tag (e.g., only fscheck-tagged scenarios)
./run-e2e-test.sh --add-tag=fscheck

# Combine --add-tag with a platform filter
./run-e2e-test.sh --powerstore --add-tag=sanity
```

---

## Minimal vs Full Test Suites

The framework provides two test suites:

| Suite | Scenarios file | CR files | Best for |
|---|---|---|---|
| **Full** (default) | `testfiles/scenarios.yaml` | Generated on-demand from `samples/` | Comprehensive validation, CI gates |
| **Minimal** (`--minimal`) | `testfiles/minimal-testfiles/scenarios.yaml` | Generated programmatically (6 base files) | Fast iteration, development, smoke tests |

**Minimal tests** use a single base CR per driver with all modules disabled. Individual scenarios then use "InSpec" steps (like `Enable [authorization] module in CR spec [3]`) to modify the CR in memory before applying it. This avoids maintaining dozens of static YAML files and makes it easy to test new combinations.

**Full tests** use on-demand generated CRs derived from the `samples/` directory, with module/version overrides applied. These cover more permutations (auth + obs, TLS variants, sidecar configurations, configMap image overrides, etc.).

---

## Test File Generation

A key improvement in this branch is replacing ~30 hand-maintained CR YAML files with programmatic generation.

### On-Demand Generation from Samples

When a scenario references a test file path (e.g., `testfiles/storage_csm_powerflex.yaml`), the framework checks if it needs to be generated. If so, it:

1. Reads the corresponding sample CR from `samples/` (e.g., `samples/v2.17.0/storage_csm_powerflex_v2170.yaml`).
2. Applies transformations -- namespace override, module enable/disable, environment variable overrides, version overrides.
3. Writes the result to the `testfiles/` directory.

This is handled by `steps/generate_testfiles_from_samples.go`. The generation is:
- **On-demand** -- files are generated only when first referenced by a scenario.
- **Thread-safe** -- protected by a mutex to handle concurrent access.
- **Cached** -- each file is generated at most once per test run.

Currently **40 test files** are generated this way, covering all driver/module combinations, upgrade/downgrade variants, and version-specific configurations.

### Minimal Test File Generation

Handled by `steps/generate_minimal_testfiles.go`. Generates **6 base YAML files** (one per driver: PowerFlex, PowerScale, PowerStore, PowerMax, Unity, COSI) with:
- `spec.version` set (rather than `driver.configVersion`)
- All modules listed but disabled by default
- Placeholder values for environment variables (expanded at load time)

Scenarios then modify these base files in-place using InSpec step functions to enable specific modules and set configuration values before applying them to the cluster.

---

## Scenario File Format

Scenarios are defined in YAML. Each scenario specifies a name, the CR file paths it uses, the tags it belongs to, and an ordered list of steps:

```yaml
- scenario: "Install PowerScale Driver(With Observability)"
  paths:
    - "testfiles/storage_csm_powerscale_observability.yaml"
  tags:
    - "powerscale"
    - "observability"
  steps:
    - "Given an environment with k8s or openshift, and CSM operator installed"
    - "Create StorageClass with template [...] for [powerscale]"
    - "Create Secret with template [...] name [isilon-creds] in namespace [${E2E_NS_POWERSCALE}] for [powerscale]"
    - "Apply custom resource [1]"
    - "Validate custom resource [1]"
    - "Validate [powerscale] driver from CR [1] is installed"
    - "Validate [observability] module from CR [1] is installed"
    # cleanup
    - "Enable forceRemoveDriver on CR [1]"
    - "Delete custom resource [1]"
    - "Validate [powerscale] driver from CR [1] is not installed"
```

**Key fields:**
- **`paths`** -- List of CR YAML paths. Referenced by 1-based index in steps (e.g., `[1]` = first path, `[2]` = second).
- **`tags`** -- Used for filtering by platform/module flags. A scenario runs if its tags match the user's selection.
- **`steps`** -- Ordered step names matched by regex against registered step handlers. Values in `[brackets]` become function arguments.
- **`config`** (optional) -- Key-value map for conditional step execution (e.g., `enableSftpSDC: "true"`).
- **`customTest`** (optional) -- External commands to run as part of the scenario.

**Minimal scenarios** additionally use "InSpec" steps to modify CRs before applying:
```yaml
- "Set metadata name [powerstore] namespace [${E2E_NS_POWERSTORE}] in CR spec [3]"
- "Enable [authorization] module in CR spec [3]"
- "Set [authorization] component [karavi-authorization-proxy] env [PROXY_HOST] to [...] in CR spec [3]"
```

---

## Automated Infrastructure Setup

The test script automates several setup tasks that previously required manual intervention.

### Namespace Management

- **Creation:** Test namespaces are dynamically computed from the `NS_PREFIX` in `array-info.yaml` (default: `e2e`) and created automatically. For example, selecting `--powerflex` creates: `e2e` (operator), `e2e-powerflex`, `e2e-authorization`, `e2e-proxy-ns`.
- **Cleanup:** On exit, all test namespaces are deleted (unless `--no-cleanup-ns` is passed). Authorization CRDs with blocking finalizers are handled automatically -- finalizers are stripped before deletion.
- **Idempotent:** Existing namespaces are deleted and recreated before each run, ensuring a clean state.

### Vault and Secrets Store CSI Driver

When authorization scenarios are going to run, the script **automatically**:
1. Installs the **Secrets Store CSI Driver** via Helm (if not already present).
2. Installs **Vault** with CSI Provider via the `scripts/vault-automation/main.go` automation tool.
3. Configures Vault with the required secrets and roles for authorization testing.

You can force Vault installation with `--install-vault` even when auth tests aren't selected. For Conjur-based authorization, pass `--install-conjur`.

### Authorization Proxy Host

The entry `<cluster-ip> csm-authorization.com` is **automatically added to `/etc/hosts`** when authorization tests run. No manual host file editing is needed. On OpenShift clusters, the script resolves the hostname via `nslookup` against the internal DNS.

---

## Execution Control and Reporting

### Continue-on-Failure Mode

By default, the test run stops at the first scenario failure. Pass `--continue-on-fail` to run all scenarios regardless of failures. A summary report is printed at the end showing pass/fail/skip/abort status for every scenario:

```
==========================================================================================
  STATUS  SCENARIO                                                        TIME
==========================================================================================
  PASS    Install PowerScale Driver(Standalone)                           [22s]
  PASS    Install PowerFlex Driver(With Observability)                    [2m42s]
  FAIL    Install PowerMax Driver(With Authorization V2)                  [5m38s]
           Error: expected custom resource status to be Succeeded. Got: Failed
  SKIP    Install COSI Driver(Standalone)
==========================================================================================
  Total: 95 | Passed: 93 | Failed: 1 | Aborted: 0 | Skipped: 1 | Time: 3h22m19s
==========================================================================================
```

### JUnit XML Reports

Pass `--junit-report=<path>` to generate a JUnit XML report suitable for CI systems (Jenkins, GitLab CI, etc.):

```bash
./run-e2e-test.sh --powerflex --continue-on-fail --junit-report=results/e2e-report.xml
```

### Step Timeouts

Each step has a timeout category to prevent indefinite hangs:

| Category | Timeout | Step patterns |
|---|---|---|
| **Long** | 20 min | Upgrades, third-party installs (`Install [...]`), custom tests, auth proxy configuration |
| **Medium** | 10 min | All `Validate [...]` steps |
| **Fast** | 3 min | Everything else (apply, delete, create, enable/disable) |

Steps are retried every 10 seconds within their timeout window. If a step doesn't succeed before the timeout, the scenario fails.

---

## Custom Registry Tests

Some scenarios test the `spec.customRegistry` feature, which overrides image registries in driver/module CRs. These scenarios require that the referenced images actually exist in the custom registry.

The `E2E_CREG_VERSION` environment variable controls the `spec.version` set on custom-registry test CRs. If not set, it defaults to `v1.16.0`. Override it when a newer stable version is available:

```bash
export E2E_CREG_VERSION=v1.17.0
./run-e2e-test.sh --powermax
```

---

## Developing E2E Tests

### Adding a New Scenario

1. Add the scenario definition to `testfiles/scenarios.yaml` (or `testfiles/minimal-testfiles/scenarios.yaml` for minimal tests).
2. Set appropriate **tags** so platform/module filtering works correctly.
3. Reference existing step definitions where possible -- most common operations already have implementations.
4. If the scenario needs a new test file, either:
   - Add a `TestfileSpec` entry in `steps/generate_testfiles_from_samples.go` for on-demand generation.
   - Or place a static YAML file in `testfiles/`.

### Adding a New Step

1. **Implement** the step handler in `steps/steps_def.go`:
   ```go
   func (step *Step) myNewStep(res Resource, arg1 string) error {
       // Return nil on success, error on failure (triggers retry)
       return nil
   }
   ```
   - First parameter must be `Resource`.
   - Additional parameters correspond to `[capture groups]` in the step name.

2. **Register** the step in `StepRunnerInit()` in `steps/steps_runner.go`:
   ```go
   runner.addStep(`^My new step with \[([^"]*)\]$`, step.myNewStep)
   ```

3. **Reference** the step in a scenario:
   ```yaml
   steps:
     - "My new step with [some-value]"
   ```

4. If you get `no method for step: <your step>`, check that your regex matches the step text exactly.

---

## Troubleshooting

| Problem | Solution |
|---|---|
| `no method for step: <step>` | Check that the step is registered in `steps_runner.go` and the regex matches. |
| Template files modified from a previous failed run | Run `git checkout -- testfiles/*-templates/` to restore them. |
| Auth tests fail with "tenant not found" | This is expected during startup. The framework retries with backoff. If it persists, check that Vault credentials are correct. |
| Namespace deletion hangs | Authorization CRDs with finalizers can block deletion. The script handles this automatically, but if it hangs, manually remove finalizers: `kubectl patch csmtenant <name> -n <ns> -p '{"metadata":{"finalizers":null}}' --type=merge` |
| `--no-cleanup-ns` to debug | Keeps all test namespaces after the run so you can inspect pods, logs, and events. |
| Tests show transient "Failed" status then pass | The operator's status calculation has a brief window where DaemonSet/Deployment counts are still converging. The test framework retries and this is expected behavior. |
| Custom registry tests fail with image pull errors | Ensure `E2E_CREG_VERSION` matches a version whose images exist in your custom registry. |
