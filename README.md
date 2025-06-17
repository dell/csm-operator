
<!--
Copyright (c) 2022 - 2025 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
-->

# Dell Technologies Container Storage Modules (CSM) Operator

[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v2.0%20adopted-ff69b4.svg)](https://github.com/dell/csm/blob/main/docs/CODE_OF_CONDUCT.md)
[![License](https://img.shields.io/github/license/dell/csm-operator)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/dellemc/dell-csm-operator)](https://hub.docker.com/r/dellemc/dell-csm-operator)
[![Go version](https://img.shields.io/github/go-mod/go-version/dell/csm-operator)](go.mod)
[![GitHub release (latest by date including pre-releases)](https://img.shields.io/github/v/release/dell/csm-operator?include_prereleases&label=latest&style=flat-square)](https://github.com/dell/csm-operator/releases/latest)

Dell Technologies Container Storage Modules (CSM) Operator is an open-source Kubernetes operator which can be used to install and manage various CSI Drivers and CSM Modules.

## Table of Contents

* [Code of Conduct](./docs/CODE_OF_CONDUCT.md)
* [Maintainer Guide](./docs/MAINTAINER_GUIDE.md)
* [Committer Guide](./docs/COMMITTER_GUIDE.md)
* [Contributing Guide](./docs/CONTRIBUTING.md)
* [List of Adopters](./docs/ADOPTERS.md)
* [Bundle Validation](./docs/BUNDLE_VALIDATION.md)
* [Dell support](https://www.dell.com/support/incidents-online/en-us/contactus/product/container-storage-modules)
* [Security](./docs/SECURITY.md)
* [Dell Container Storage Modules Operator](#dell-container-storage-modules-operator)
* [Documentation](#documentation)

# Dell Container Storage Modules Operator

Dell Container Storage Modules Operator is a Kubernetes native application which helps in installing and managing CSI Drivers and CSM Modules provided by Dell Technologies for its various storage platforms.
Dell Container Storage Modules Operator uses Kubernetes CRDs (Custom Resource Definitions) to define a manifest that describes the deployment specifications for each driver to be deployed.

Dell Container Storage Modules Operator is built using the [operator framework](https://github.com/operator-framework) and runs custom Kubernetes controllers to manage the driver installations. These controllers listen for any create/update/delete request for the respective CRDs and try to reconcile the request.

## Documentation
For more detailed information on the driver, please refer to [Container Storage Modules documentation](https://dell.github.io/csm-docs/).
