
<!--
Copyright (c) 2021 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
-->

# Dell EMC Container Storage Modules (CSM) Operator
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v2.0%20adopted-ff69b4.svg)](https://github.com/dell/csm/blob/main/docs/CODE_OF_CONDUCT.md)
[![License](https://img.shields.io/github/license/dell/csm-operator)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/dellemc/csm-operator)](https://hub.docker.com/r/dellemc/csm-operator)
[![Go version](https://img.shields.io/github/go-mod/go-version/dell/csm-operator)](go.mod)
[![GitHub release (latest by date including pre-releases)](https://img.shields.io/github/v/release/dell/csm-operator?include_prereleases&label=latest&style=flat-square)](https://github.com/dell/csm-operator/releases/latest)

Dell EMC Container Storage Modules (CSM) Operator is an open-source Kubernetes operator which can be used to install and manage various [Container Storage Modules - Components](#container-storage-modules---components).

## Table of Contents

* [Code of Conduct](./docs/CODE_OF_CONDUCT.md)
* [Maintainer Guide](./docs/MAINTAINER_GUIDE.md)
* [Committer Guide](./docs/COMMITTER_GUIDE.md)
* [Contributing Guide](./docs/CONTRIBUTING.md)
* [Branching Strategy](./docs/BRANCHING.md)
* [List of Adopters](./docs/ADOPTERS.md)
* [Maintainers](./docs/MAINTAINERS.md)
* [Support](./docs/SUPPORT.md)
* [Security](./docs/SECURITY.md)
* [Container Storage Modules - Components](#container-storage-modules---components)
* [Development](#development)
  * [Custom Resource Definitions (CRDs)](#custom-resource-definitions-crds) 
  * [Controllers](#controllers)
  * [How to Build Container Storage Modules (CSM) Operator](#how-to-build-container-storage-modules-csm-operator)
  * [How to Deploy Container Storage Modules (CSM) Operator](#how-to-deploy-container-storage-modules-csm-operator)
* [CSM Operator Certified Bundle](#csm-operator-certified-bundle)
* [Versioning](#versioning)
* [About](#about)
  

## Container Storage Modules - Components

* [Dell EMC Container Storage Modules (CSM) for Authorization](https://github.com/dell/karavi-authorization)
* [Dell EMC Container Storage Modules (CSM) for Observability](https://github.com/dell/karavi-observability)
* [Dell EMC Container Storage Modules (CSM) for Replication](https://github.com/dell/csm-replication)
* [Dell EMC Container Storage Modules (CSM) for Resiliency](https://github.com/dell/karavi-resiliency)
* [Dell EMC Container Storage Modules (CSM) for Volume Group Snapshotter](https://github.com/dell/csi-volumegroup-snapshotter)
* [CSI Driver for Dell EMC PowerFlex](https://github.com/dell/csi-powerflex)
* [CSI Driver for Dell EMC PowerMax](https://github.com/dell/csi-powermax)
* [CSI Driver for Dell EMC PowerScale](https://github.com/dell/csi-powerscale)
* [CSI Driver for Dell EMC PowerStore](https://github.com/dell/csi-powerstore)
* [CSI Driver for Dell EMC Unity](https://github.com/dell/csi-unity)


## Development

Container Storage Modules (CSM) Operator is built, deployed and tested using the toolset provided by Operator [framework](https://github.com/operator-framework) which include:
* [operator-sdk](https://github.com/operator-framework/operator-sdk)
* [operator-lifecycle-manager](https://github.com/operator-framework/operator-lifecycle-manager)
* [operator-registry](https://github.com/operator-framework/operator-registry)

### Custom Resource Definitions (CRDs)

`csm-operator` manages a single [Custom Resource Definitions](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) (CRDs) that supports all [Container Storage Modules - Components](#container-storage-modules---components). This CRD represent a specific CSI Driver installation and option to add other none-driver modules for that installation and is part of the API group `storage.dell.com`. The CRD managed by `csm-operator` is called `CSM`

### Controllers

`csm-operator` utilizes Kubernetes [controller runtime](https://github.com/kubernetes-sigs/controller-runtime) libraries for building controllers which
run as part of the Operator deployment.  
These controllers watch for any requests to create/modify/delete instances of the Custom Resource Definitions (CRDs) and handle the [reconciliation](https://godoc.org/sigs.k8s.io/controller-runtime/pkg/reconcile)
of these requests.

Each instance of a CRD is called a Custom Resource (CR) and can be managed by a client like `kubectl` in the same way a native
Kubernetes resource is managed.  
When you create a Custom Resource, then the corresponding Controller will create the Kubernetes objects required for the driver installation.  

This includes:
* Service Accounts and RBAC configuration
* StatefulSet
* DaemonSet
* Deployment and Service (only for Reverse Proxy for PowerMax Driver)

> __Note__: - There is one controller for a Custom Resource type and each controller runs a single worker 


### How to Build Container Storage Modules (CSM) Operator

If you wish to clone and build the Container Storage Modules (CSM) Operator, a Linux host is required with the following installed:

| Component       | Version   | Additional Information                                                                                                                     |
| --------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| Docker          | v19+      | [Docker installation](https://docs.docker.com/engine/install/)                                                                                                    |
| Docker Registry |           | Access to a local/corporate [Docker registry](https://docs.docker.com/registry/)                                                           |
| Golang          | v1.14+    | [Golang installation](https://github.com/travis-ci/gimme)                                                                                                         |
| operator-sdk          | v1.15.0   |[Operator SDK installation](https://github.com/operator-framework/operator-sdk/releases/download/v1.15.0/operator-sdk_linux_amd64)                                                                                                          |
| OLM            |     | Run ```operator-sdk olm install```                                                                                                       |
| OPM           |   v1.14+  | [OPM installation](https://github.com/operator-framework/operator-registry/releases/download/v1.14.0/linux-amd64-opm)                                                              |
| git             | latest    | [Git installation](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git)                                                                              |
| gcc             |           | Run ```sudo apt install build-essential```                                                                                                 |
| kubectl         | 1.21-1.23 | Ensure you copy the kubeconfig file from the Kubernetes cluster to the linux host. [kubectl installation](https://kubernetes.io/docs/tasks/tools/install-kubectl/) |

> __Note__: While installing `operator-sdk` & `opm`, make sure they are available in your PATH & the binaries have the right names and permissions.


There are multiple Makefile targets available for building the Operator


#### Build Operator image

There are a few available Makefile targets which let you build a docker image for the Operator. 
The docker image is built using the `operator-sdk` binary (which is available in the repository). 
The base image used to build the docker image is UBI (Universal Base Image) provided by Red Hat.

Run the command `make docker-build` to build a docker image for the Operator which will be tagged with git semantic versioning  
The official builds of Operator which are hosted on artifactory are built using the `make docker-build` command

By default, this target will tag the newly built images with the artifactory repo  
Run the command `make docker-build IMG=<local/private/public docker registory>/csm-operator:<your-tag>` to tag the docker image with your own repository


#### Push docker Operator image

Run the command `make docker-push IMG=<local/private/public registory>/csm-operator:<your-tag>`  to push the docker image to `local/private/public docker registry>` 


#### Build Operator image along with bundle

Run the command `make docker-build BUNDLE_IMG=<local/private/public docker registory>/csm-operator-bundle:<your-tag>` to build the operator image and operator bundle image. 

The above command will also push the images to the specified registry.

### How to Deploy Container Storage Modules (CSM) Operator

There are primarily four ways of deploying the Operator -

1. [Deploy Operator without OLM](#deploy-operator-without-olm)
2. [Deploy Operator using OLM](#deploy-operator-using-olm)
3. [Offline installation of operator](#offline-installation-of-operator)
4. [Run locally without deploying any image](#run-the-operator-locally-without-deploying-any-image)
 
After installing successfully, there should be an Operator deployment created in the cluster. You can now query for the CRDs installed in the cluster by running the command `kubectl get crd`. You should see `csm.storage.dell.com` aong the list of crds
 
#### Deploy Operator without OLM

TODO

#### Deploy Operator using OLM

TODO

#### Offline Installation of Operator

TODO

#### Run the Operator locally without deploying any image 

Make sure that a [**KubeConfig**](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/) file pointing to your Kubernetes/OpenShift cluster is present in the default location

Run `make install run` to run the Operator in your cluster without creating a deployment.

The above command will update the CRDs, install the CRDs and then run the Operator.

### How to Install Container Storage Modules (CSM) Components using Operator

TODO

## CSM Operator Certified Bundle 

TODO

## Versioning

This project is adhering to [Semantic Versioning](https://semver.org/).

## About

Dell EMC Container Storage Modules (CSM) Operator is 100% open source and community-driven. All components are available
under [Apache 2 License](https://www.apache.org/licenses/LICENSE-2.0.html) on
GitHub.
