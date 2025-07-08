# csm-authorization-vault
Install Vault in Kubernetes/Openshift configured for CSM Authorization

# Prerequisites
- helm
- kubectl

# Installing
There are multiple methods to install and run this tool:

- Clone this repository and execute main.go directly (the examples below use this method)
- Clone this repository, build an executable (`go build -o csm-authorization-vault main.go`), and run the executable

# Running
## Required Flags
`--kubeconfig` path to kubeconfig of the cluster where CSM Authorization will be installed\
`--name`       name of the Vault application

## Optional Flags
`--openshift`                   configure installation to use image `registry.connect.redhat.com/hashicorp/vault` instead of `hashicorp/vault`\
`--secrets-store-csi-driver`    boolean to configure the secrets-store-csi-driver daemonset (defaults to false)\
`--validate-client-cert`        configure Vault to validate the client's certificate\
`--csm-authorization-namespace` namespace of CSM Authorization (defaults to authorization)\
`--secret-path`                 path in Vault to create a secret (must also supply --username and --password flags)\
`--username`                    username key of a credential\
`--password`                    password key of a credential

## Examples
There are multiple ways to configure Vault with key/value secrets with this tool:

- Without any secrets
- With one secret specified in flags
- With one or more secrets specified in a configuration file
- With secrets that align with CSM Operator e2e specified in environment variables

### Install Vault without creating any key/value secrets
```
# go run main.go --kubeconfig ~/.kube/config --name vaultx --secrets-store-csi-driver true
2024/10/29 19:47:20 Generating CA Certificate: vaultx-ca.pem
2024/10/29 19:47:21 Generating Vault server certificate
2024/10/29 19:47:23 Generating Client certificate: vaultx-client.pem, vaultx-client-key.pem
2024/10/29 19:47:26 Writing CA vaultx-ca.pem and Vault client certificate files vaultx-client.pem and vaultx-client-key.pem in working directory
2024/10/29 19:47:26 Creating vaultx-policy configMap in default namespace
2024/10/29 19:47:26 Creating vaultx-tls secret in default namespace
2024/10/29 19:47:26 Creating vaultx-ca secret in default namespace
2024/10/29 19:47:26 Configuring values file for Vault helm chart
2024/10/29 19:47:26 Installing vaultx via helm
2024/10/29 19:47:27 Waiting for pod/vaultx-0 to be Ready
2024/10/29 19:47:37 Enabling Kubernetes authentication in vaultx
2024/10/29 19:47:38 Configuring Kubernetes authentication in vaultx
2024/10/29 19:47:38 Configuring policy vaultx
2024/10/29 19:47:39 Configuring role vaultx
```

### Install Vault and create one key/value secret
```
# go run main.go --kubeconfig ~/.kube/config --name vaultx --secrets-store-csi-driver true --secret-path powerflex --username admin --password Password123!
2024/10/29 19:47:20 Generating CA Certificate: vaultx-ca.pem
2024/10/29 19:47:21 Generating Vault server certificate
2024/10/29 19:47:23 Generating Client certificate: vaultx-client.pem, vaultx-client-key.pem
2024/10/29 19:47:26 Writing CA vaultx-ca.pem and Vault client certificate files vaultx-client.pem and vaultx-client-key.pem in working directory
2024/10/29 19:47:26 Creating vaultx-policy configMap in default namespace
2024/10/29 19:47:26 Creating vaultx-tls secret in default namespace
2024/10/29 19:47:26 Creating vaultx-ca secret in default namespace
2024/10/29 19:47:26 Configuring values file for Vault helm chart
2024/10/29 19:47:26 Installing vaultx via helm
2024/10/29 19:47:27 Waiting for pod/vaultx-0 to be Ready
2024/10/29 19:47:37 Enabling Kubernetes authentication in vaultx
2024/10/29 19:47:38 Configuring Kubernetes authentication in vaultx
2024/10/29 19:47:38 Configuring policy vaultx
2024/10/29 19:47:39 Configuring role vaultx
2024/10/29 19:47:39 Writing secret powerflex in vaultx
```

### Install Vault and create one or more key/value secrets via a configuration file

Example configuration file:
```
- path: "powerflex"
  username: "admin"
  password: "password"
- path: "powerscale"
  username: "root"
  password: "pancake"
```

```
# go run main.go --kubeconfig ~/.kube/config --name vaultx --secrets-store-csi-driver true --file-config /path/to/config.yaml

2025/05/20 17:20:41 Generating CA Certificate: vault0-ca.pem
2025/05/20 17:20:43 Generating Vault server certificate
2025/05/20 17:20:44 Generating Client certificate: vault0-client.pem, vault0-client-key.pem
2025/05/20 17:20:46 Writing CA vault0-ca.pem and Vault client certificate files vault0-client.pem and vault0-client-key.pem in working directory
2025/05/20 17:20:46 Creating vault0-config configMap in default namespace
2025/05/20 17:20:46 Creating vault0-tls secret in default namespace
2025/05/20 17:20:46 Creating vault0-ca secret in default namespace
2025/05/20 17:20:46 Configuring values file for Vault helm chart
2025/05/20 17:20:46 Installing vault0 via helm
2025/05/20 17:20:48 Waiting for pod/vault0-0 to be Ready
2025/05/20 17:20:55 Enabling Kubernetes authentication in vault0
2025/05/20 17:20:56 Configuring Kubernetes authentication in vault0
2025/05/20 17:20:57 Configuring policy vault0
2025/05/20 17:20:57 Configuring role vault0
2025/05/20 17:20:58 Writing secret powerflex in vault0
2025/05/20 17:20:58 Writing secret powerscale in vault0
```

### Install Vault and create key/value secrets via environment variables

This method aligns with CSM Operator E2E tests by reading specific environment variable from the file `e2e/array-info.env` (see `e2e/array-info.env.sample` for reference):

```
PFLEX_VAULT_STORAGE_PATH
PFLEX_USER
PFLEX_PASS

PSCALE_VAULT_STORAGE_PATH
PSCALE_USER
PSCALE_PASS

PMAX_VAULT_STORAGE_PATH
PMAX_USER
PMAX_PASS
```

The variables in each set of three, three variables per platform, must be set prior to running this tool. If any of the three environment variables for a specific platform is not set, that secret will not be written in Vault.

```
# go run main.go --kubeconfig ~/.kube/config --name vaultx --secrets-store-csi-driver true --env-config
2025/05/20 17:26:31 Generating CA Certificate: vault0-ca.pem
2025/05/20 17:26:34 Generating Vault server certificate
2025/05/20 17:26:36 Generating Client certificate: vault0-client.pem, vault0-client-key.pem
2025/05/20 17:26:38 Writing CA vault0-ca.pem and Vault client certificate files vault0-client.pem and vault0-client-key.pem in working directory
2025/05/20 17:26:38 Creating vault0-config configMap in default namespace
2025/05/20 17:26:38 Creating vault0-tls secret in default namespace
2025/05/20 17:26:38 Creating vault0-ca secret in default namespace
2025/05/20 17:26:38 Configuring values file for Vault helm chart
2025/05/20 17:26:38 Installing vault0 via helm
2025/05/20 17:26:40 Waiting for pod/vault0-0 to be Ready
2025/05/20 17:26:47 Enabling Kubernetes authentication in vault0
2025/05/20 17:26:48 Configuring Kubernetes authentication in vault0
2025/05/20 17:26:48 Configuring policy vault0
2025/05/20 17:26:49 Configuring role vault0
2025/05/20 17:26:49 Writing secret storage/powerflex in vault0
2025/05/20 17:26:50 Writing secret storage/powerscale in vault0
2025/05/20 17:26:50 Writing secret storage/powermax in vault0
```

# Post Running
The generated certificate files in the working directory can, but don't have to, be used when installing Authorization.

```
# ls
vaultx-ca.pem vaultx-client.pem vaultx-client-key.pem
```

Vault is accessed within the cluster at `https://<name>.default.svc.cluster.local:8400`. This is the address for Vault you should use when configuring the Authorization CR.

To access the Vault Web UI, use `kubectl port-forward` and login token `root`:

```
kubectl port-forward sts/<name> 8400:8400 --address 0.0.0.0
```

# Uninstall
```
helm delete <name>
```