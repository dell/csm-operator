
<!--
Copyright (c) 2022 - 2023 Dell Inc., or its subsidiaries. All Rights Reserved.

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
* [Support](./docs/SUPPORT.md)
* [Security](./docs/SECURITY.md)
* [Dell Container Storage Modules Operator](#dell-container-storage-modules-operator)
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
  * CSM Resiliency

**NOTE**: You can refer to additional information about the Dell Container Storage Modules Operator on the new documentation website [here](https://dell.github.io/csm-docs/docs/deployment/csmoperator/)

## Support

The Dell Container Storage Modules Operator image is available on Dockerhub and is officially supported by Dell Technologies.
For any CSM Operator and driver issues, questions or feedback, join the [Dell Technologies Container community](https://www.dell.com/community/Containers/bd-p/Containers) or the [Slack channel for Dell Container Storage Modules](https://dellemccsm.slack.com/).

## Supported Platforms

Dell Container Storage Modules Operator has been tested and qualified with

    * Upstream Kubernetes cluster v1.28, v1.29, v1.30
    * OpenShift Clusters 4.14, 4.15 with RHEL 8.x & RHCOS worker nodes

## Installation

## Install Operator (both OLM and Non OLM) in dev mode

  1. Clone the repo: `git clone https://github.com/dell/csm-operator.git`
  2. Navigate one level inside the cloned repo: `cd csm-operator`
  3. Execute `make install` to install the CRDs
  4. Execute `make run` to have the operator running.
  5. Install any of the operands (CSI Driver) using another session

NOTE: Closing the session where operator is running will stop the operator.

## Install Operator using scripts

### Operator install on a cluster without OLM

For Non OLM based install of Operator please refer the steps given here at [https://dell.github.io/csm-docs/docs/deployment/csmoperator/#operator-installation-on-a-cluster-without-olm](https://dell.github.io/csm-docs/docs/deployment/csmoperator/#operator-installation-on-a-cluster-without-olm)

To uninstall the Operator, execute `bash scripts/uninstall.sh`

### Operator install on a cluster with OLM

  NOTE: Index image or Catalog image should be used for OLM based install of Operator. This mode of install is used only for internal testing purposes as the bundle and index/catalog images are not posted in Dockerhub.

  1. Clone the repo: `git clone https://github.com/dell/csm-operator.git`
  2. Navigate one level inside the cloned repo: `cd csm-operator`
  3. Update the image specified in `deploy/olm/operator_community.yaml` to the required image.
  4. Execute `bash scripts/install_olm.sh` to install the operator.

  To uninstall the Operator, execute `bash scripts/uninstall_olm.sh`

### Operator install using offline bundle on a cluster without OLM

For Non OLM based install of Operator using offline bundle please refer the steps given here at [https://dell.github.io/csm-docs/docs/deployment/csmoperator/#offline-bundle-installation-on-a-cluster-without-olm](https://dell.github.io/csm-docs/docs/deployment/csmoperator/#offline-bundle-installation-on-a-cluster-without-olm)

## Install CSI Drivers and CSM Modules

To install CSI drivers and CSM modules using the operator please refer here at [https://dell.github.io/csm-docs/docs/deployment/csmoperator/](https://dell.github.io/csm-docs/docs/deployment/csmoperator/)

## Uninstall CSI Drivers and CSM Modules

To uninstall CSI drivers and CSM modules installed using the operator please refer here at [https://dell.github.io/csm-docs/docs/deployment/csmoperator/](https://dell.github.io/csm-docs/docs/deployment/csmoperator/).

## Install Apex Connectivity Client

  1. Ensure that CSM Operator is installed and the operator pods are up and running.
  2. Edit the images to point to the correct location in `connectivity_client_v100.yaml` sample file located at `csm-operator\samples` folder.
  3. To deploy Apex Connectivity Client, execute `kubectl create -f samples\connectivity_client_v100.yaml`.
  4. Ensure that the Apex Connectivity Client pods are up and running.

## Update Apex Connectivity Client

  1. Ensure that CSM Operator is installed and the operator pods are up and running.
  2. Edit the required images to point to the correct location in `connectivity_client_v100.yaml` sample file located at `csm-operator\samples` folder.
  3. To update Apex Connectivity Client, execute `kubectl apply -f samples\connectivity_client_v100.yaml`.
  4. Ensure that the Apex Connectivity Client pods are up and running.

## Uninstall Apex Connectivity Client

  1. Ensure that CSM Operator is installed and the operator pods are up and running.
  2. To uninstall Apex Connectivity Client, execute `kubectl delete -f samples\connectivity_client_v100.yaml`
  3. Ensure that the Apex Connectivity Client pods are deleted.

## Versioning

This project is adhering to [Semantic Versioning](https://semver.org/).

## About

Dell Technologies Container Storage Modules (CSM) operator is 100% open source and community-driven. All components are available
under [Apache 2 License](https://www.apache.org/licenses/LICENSE-2.0.html) on
GitHub.
