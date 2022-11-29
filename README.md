
<!--
Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
-->

# Dell Technologies Container Storage Modules (CSM) Operator
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v2.0%20adopted-ff69b4.svg)](https://github.com/dell/csm/blob/main/docs/CODE_OF_CONDUCT.md)
[![License](https://img.shields.io/github/license/dell/csm-operator)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/dellemc/csm-operator)](https://hub.docker.com/r/dellemc/csm-operator)
[![Go version](https://img.shields.io/github/go-mod/go-version/dell/csm-operator)](go.mod)
[![GitHub release (latest by date including pre-releases)](https://img.shields.io/github/v/release/dell/csm-operator?include_prereleases&label=latest&style=flat-square)](https://github.com/dell/csm-operator/releases/latest)

Dell Technologies Container Storage Modules (CSM) Operator is an open-source Kubernetes operator which can be used to install and manage various CSI Drivers and CSM Modules.

## Table of Contents

* [Code of Conduct](./docs/CODE_OF_CONDUCT.md)
* [Maintainer Guide](./docs/MAINTAINER_GUIDE.md)
* [Committer Guide](./docs/COMMITTER_GUIDE.md)
* [Contributing Guide](./docs/CONTRIBUTING.md)
* [List of Adopters](./docs/ADOPTERS.md)
* [Support](./docs/SUPPORT.md)
* [Security](./docs/SECURITY.md)
* [Dell Container Storage Modules Operator](#dell-csm-operator)
  * [Support](#support)
  * [Supported Platforms](#supported-platforms)
  * [Installation](#installation)
  * [Install CSI Drivers and CSM Modules](#install-csi-drivers-and-csm-modules)
  * [Uninstall CSI Drivers and CSM Modules](#uninstall-csi-drivers-and-csm-modules)

# Dell Container Storage Modules Operator
Dell Container Storage Modules Operator is a Kubernetes native application which helps in installing and managing CSI Drivers and CSM Modules provided by Dell Technologies for its various storage platforms. 
Dell Container Storage Modules Operator uses Kubernetes CRDs (Custom Resource Definitions) to define a manifest that describes the deployment specifications for each driver to be deployed.

Dell Container Storage Modules Operator is built using the [operator framework](https://github.com/operator-framework) and runs custom Kubernetes controllers to manage the driver installations. These controllers listen for any create/update/delete request for the respective CRDs and try to reconcile the request.

Currently, the Dell Container Storage Modules Operator can be used to deploy the following CSI drivers provided by Dell Technologies

* CSI Driver for Dell Technologies PowerScale
  * CSM Authorization
  * CSM Replication
  * CSM Observability

**NOTE**: You can refer to additional information about the Dell Container Storage Modules Operator on the new documentation website [here](https://dell.github.io/csm-docs/docs/deployment/csmoperator/)

## Support
The Dell Container Storage Modules Operator image is available on Dockerhub and is officially supported by Dell Technologies.
For any CSM Operator and driver issues, questions or feedback, join the [Dell Technologies Container community](https://www.dell.com/community/Containers/bd-p/Containers) or the [Slack channel for Dell Container Storage Modules](https://dellemccsm.slack.com/).

## Supported Platforms
Dell Container Storage Modules Operator has been tested and qualified with 

    * Upstream Kubernetes cluster v1.23, v1.24, v1.25
    * OpenShift Clusters 4.10, 4.11 with RHEL 8.x & RHCOS worker nodes

## Installation
To install Dell Container Storage Modules Operator please refer the steps given here at [https://dell.github.io/csm-docs/docs/deployment/csmoperator/](https://dell.github.io/csm-docs/docs/deployment/csmoperator/)

## Install CSI Drivers and CSM Modules
To install CSI drivers and CSM modules using the operator please refer here at [https://dell.github.io/csm-docs/docs/deployment/csmoperator/](https://dell.github.io/csm-docs/docs/deployment/csmoperator/)

## Uninstall CSI Drivers and CSM Modules
To uninstall CSI drivers and CSM modules installed using the operator please refer here at [https://dell.github.io/csm-docs/docs/deployment/csmoperator/](https://dell.github.io/csm-docs/docs/deployment/csmoperator/).

## Versioning

This project is adhering to [Semantic Versioning](https://semver.org/).

## About

Dell Technologies Container Storage Modules (CSM) operator is 100% open source and community-driven. All components are available
under [Apache 2 License](https://www.apache.org/licenses/LICENSE-2.0.html) on
GitHub.
