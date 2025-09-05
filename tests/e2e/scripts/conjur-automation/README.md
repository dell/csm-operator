<!--
Copyright (c) 2025 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
-->

# csm-authorization-conjur
Install Conjur and Conjur CSI Provider in Kubernetes/Openshift configured for CSM Authorization

This does not install the Secrets Store CSI Driver. Use the command below to do so.

```
helm install csi-secrets-store \
    secrets-store-csi-driver/secrets-store-csi-driver \
    --wait \
    --namespace kube-system \
    --set 'enableSecretRotation=true' \
    --set 'syncSecret.enabled=true' \
    --set 'tokenRequests[0].audience=conjur'
```

# Prerequisites
- helm
- kubectl
- jq
- docker or podman

# Running
## Required Flags
`--control-node` IP address of the Kubernetes master node or the OpenShift bastion node\

## Optional Flags
`--file-config` path to yaml file containing credential information\
`--env-config`  configure Conjur from environment variables defined in CSM Operator e2e\

## Examples
### Install and create secrets from a configuration file

`conjur.sh --control-node 10.0.0.1 --file-config credentials.yaml`

Example credentials.yaml:
```
- variable: "system-username"
  value: "myUsername"
- variable: "system-password"
  value: "myPassword"
- variable: "system2-username"
  value: "myUsername"
- variable: "system2-password"
  value: "myPassword"
- variable: "redis-username"
  value: "username"
- variable: "redis-password"
  value: "password"
- variable: "config-object"
  value: "web:\n  jwtsigningsecret: secret"
```

A SecretProviderClass named `conjur` will be created in `conjur-spc.yaml`.

The variables are created under `secrets`. When installing CSM Authorization (Operator example), specify the SecretProviderClass name and the full paths of the variables.

```
- name: storage-system-credentials
  secretProviderClasses:
    conjur:
      - name: conjur
        paths:
          - usernamePath: secrets/system-username
            passwordPath: secrets/system-password
          - usernamePath: secrets/system2-username
            passwordPath: secrets/system2-password
```

When creating a storage resource, specify the SecretProviderClass name and the full paths of the variables.

```
storageSystemCredentials:
  secretProviderClass:
    name: conjur
    usernameObjectName: secrets/system-username
    passwordObjectName: secrets/system-password
```

### Install and create secrets from environment variables
This method aligns with CSM Operator e2e by reading specific environment variables in array-info.env. Refer the sample at https://github.com/dell/csm-operator/blob/main/tests/e2e/array-info.env.sample:

```
PFLEX_USER
PFLEX_PASS

PSCALE_USER
PSCALE_PASS

PMAX_USER
PMAX_PASS

PSTORE_USER
PSTORE_PASS

REDIS_USER
REDIS_PASS

CONFIG_OBJECT
```

The variables in each set of two, two variables per platform, must be set prior to running this tool. If any of the two environment variables for a specific platform is not set, that secret will not be written in Conjur.

`conjur.sh --control-node 10.0.0.1 --env-config`

A SecretProviderClass named `conjur` will be created in `conjur-spc.yaml`.

The variables are created under `secrets` at hardcoded paths (see below). When installing CSM Authorization (Operator example), specify the SecretProviderClass name and the full paths of the variables.

The example below shows the Authorization storage-system-credentials configuration if all three platform environment variables are used. If you are not using all three platforms, you only need to specify what you are using.
```
- name: storage-system-credentials
  secretProviderClasses:
    conjur:
      - name: conjur
        paths:
          - usernamePath: secrets/powerflex-username
            passwordPath: secrets/powerflex-password
          - usernamePath: secrets/powermax-username
            passwordPath: secrets/powermax-password
          - usernamePath: secrets/powerscale-username
            passwordPath: secrets/powerscale-password
```

When creating a storage resource, specify the SecretProviderClass name and the full paths of the variables.

```
storageSystemCredentials:
  secretProviderClass:
    name: conjur
    usernameObjectName: secrets/powerflex-username
    passwordObjectName: secrets/powerflex-password
```