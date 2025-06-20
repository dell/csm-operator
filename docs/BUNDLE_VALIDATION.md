<!--
  Copyright © 2025 Dell Inc. or its subsidiaries. All Rights Reserved.

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at
       http://www.apache.org/licenses/LICENSE-2.0
  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
-->

## Bundle Validation Guide

To ensure your Operator bundle meets Operator Framework standards and passes all required checks, follow the steps below.

### Prerequisites

- Operator SDK installed (See https://sdk.operatorframework.io/docs/installation/)
- A valid Operator bundle directory (typically `./bundle/`)

### Basic Validation

Run the following command to validate your bundle:

```bash
operator-sdk bundle validate ./bundle
```

### Optional Suite Validation

To run additional optional checks provided by the Operator Framework, use the `--select-optional` flag:

```bash
operator-sdk bundle validate ./bundle --select-optional suite=operatorframework
```

### Notes

- If you encounter deprecation warnings (e.g., related to `go/v3` layout), check your `PROJECT` file and ensure it uses the correct layout version (e.g., `go.kubebuilder.io/v4`).
- The bundle validation process does **not** require a running Kubernetes cluster.
- Optional checks (enabled via `--select-optional`) are not mandatory but are highly recommended for improving the quality and compliance of your Operator.
