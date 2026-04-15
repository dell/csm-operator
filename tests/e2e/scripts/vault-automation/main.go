// Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	vaultResources = `
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: %[1]s
  namespace: default
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: %[1]s-csi-provider
  namespace: default
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: %[1]s-csi-provider-agent-config
  namespace: default
data:
  config.hcl: |
    vault {
        "address" = "http://%[1]s.default.svc:8400"
    }

    cache {}

    listener "unix" {
        address = "/var/run/vault/agent.sock"
        tls_disable = true
    }
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: %[1]s-csi-provider-clusterrole
rules:
- apiGroups:
  - ""
  resources:
  - serviceaccounts/token
  verbs:
  - create
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %[1]s-csi-provider-clusterrolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: %[1]s-csi-provider-clusterrole
subjects:
- kind: ServiceAccount
  name: %[1]s-csi-provider
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: %[1]s-csi-provider-role
  namespace: default
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]
  resourceNames:
  - vault-csi-provider-hmac-key
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: %[1]s-csi-provider-rolebinding
  namespace: default
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: %[1]s-csi-provider-role
subjects:
- kind: ServiceAccount
  name: %[1]s-csi-provider
  namespace: default
---
apiVersion: v1
kind: Service
metadata:
  name: %[1]s-internal
  namespace: default
spec:
  clusterIP: None
  publishNotReadyAddresses: true
  ports:
  - name: http
    port: 8400
    targetPort: 8400
  - name: https-internal
    port: 8201
    targetPort: 8201
  selector:
    app.kubernetes.io/name: vault
    app.kubernetes.io/instance: %[1]s
    component: server
---
apiVersion: v1
kind: Service
metadata:
  name: %[1]s
  namespace: default
spec:
  publishNotReadyAddresses: true
  ports:
  - name: http
    port: 8400
    targetPort: 8400
  - name: https-internal
    port: 8201
    targetPort: 8201
  selector:
    app.kubernetes.io/name: vault
    app.kubernetes.io/instance: %[1]s
    component: server
`

	vaultCSIDaemonSet = `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: %[1]s-csi-provider
  namespace: default
spec:
  updateStrategy:
    type: RollingUpdate
  selector:
    matchLabels:
      app.kubernetes.io/name: vault-csi-provider
      app.kubernetes.io/instance: %[1]s
  template:
    metadata:
      labels:
        app.kubernetes.io/name: vault-csi-provider
        app.kubernetes.io/instance: %[1]s
    spec:
      serviceAccountName: %[1]s-csi-provider
      containers:
      - name: vault-csi-provider
        image: hashicorp/vault-csi-provider:latest
        imagePullPolicy: IfNotPresent
        args:
        - --endpoint=/provider/vault.sock
        - --log-level=info
        - --hmac-secret-name=vault-csi-provider-hmac-key
        env:
        - name: VAULT_ADDR
          value: "unix:///var/run/vault/agent.sock"
        volumeMounts:
        - name: providervol
          mountPath: /provider
        - name: agent-unix-socket
          mountPath: /var/run/vault
        - name: config
          mountPath: /config
      - name: vault-agent
        image: %[2]s:latest
        imagePullPolicy: IfNotPresent
        command:
        - vault
        args:
        - agent
        - -config=/etc/vault/config.hcl
        env:
        - name: VAULT_LOG_LEVEL
          value: "info"
        - name: VAULT_LOG_FORMAT
          value: "standard"
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
          runAsGroup: 1000
          runAsNonRoot: true
          runAsUser: 100
        volumeMounts:
        - name: agent-config
          mountPath: /etc/vault/config.hcl
          subPath: config.hcl
          readOnly: true
        - name: agent-unix-socket
          mountPath: /var/run/vault
        - name: config
          mountPath: /config
      volumes:
      - name: providervol
        hostPath:
          path: /var/run/secrets-store-csi-providers
      - name: agent-config
        configMap:
          name: %[1]s-csi-provider-agent-config
      - name: agent-unix-socket
        emptyDir:
          medium: Memory
      - name: config
        projected:
          sources:
          - secret:
              name: %[1]s-ca
          - secret:
              name: %[1]s-tls
          - configMap:
              name: kube-root-ca.crt
          - configMap:
              name: %[1]s-config
`

	vaultStatefulSet = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: %[1]s
  namespace: default
spec:
  serviceName: %[1]s-internal
  podManagementPolicy: Parallel
  replicas: 1
  updateStrategy:
    type: OnDelete
  selector:
    matchLabels:
      app.kubernetes.io/name: vault
      app.kubernetes.io/instance: %[1]s
      component: server
  template:
    metadata:
      labels:
        app.kubernetes.io/name: vault
        app.kubernetes.io/instance: %[1]s
        component: server
    spec:
      terminationGracePeriodSeconds: 10
      serviceAccountName: %[1]s
      securityContext:
        runAsNonRoot: true
        runAsGroup: 1000
        runAsUser: 100
        fsGroup: 1000
      containers:
      - name: vault
        image: %[2]s:latest
        imagePullPolicy: IfNotPresent
        command:
        - /bin/sh
        - -ec
        args:
        - |
          /usr/local/bin/docker-entrypoint.sh vault server -dev -config=/config/%[1]s-config.hcl
        env:
        - name: HOST_IP
          valueFrom:
            fieldRef:
              fieldPath: status.hostIP
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        - name: VAULT_K8S_POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: VAULT_K8S_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: VAULT_ADDR
          value: "http://127.0.0.1:8200"
        - name: VAULT_API_ADDR
          value: "http://$(POD_IP):8200"
        - name: SKIP_CHOWN
          value: "true"
        - name: SKIP_SETCAP
          value: "true"
        - name: HOSTNAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: VAULT_CLUSTER_ADDR
          value: "https://$(HOSTNAME).%[1]s-internal:8201"
        - name: HOME
          value: "/home/vault"
        - name: VAULT_DEV_ROOT_TOKEN_ID
          value: root
        - name: VAULT_DEV_LISTEN_ADDRESS
          value: "[::]:8200"
        - name: KUBERNETES_HOST
          value: "%[3]s"
        volumeMounts:
        - name: home
          mountPath: /home/vault
        - name: config
          mountPath: /config
        ports:
        - containerPort: 8200
          name: http
        - containerPort: 8201
          name: https-internal
        - containerPort: 8202
          name: http-rep
        readinessProbe:
          exec:
            command: ["/bin/sh", "-ec", "vault status -tls-skip-verify"]
          failureThreshold: 2
          initialDelaySeconds: 5
          periodSeconds: 5
          successThreshold: 1
          timeoutSeconds: 3
        lifecycle:
          preStop:
            exec:
              command:
              - /bin/sh
              - -c
              - sleep 5 && kill -SIGTERM $(pidof vault)
      volumes:
      - name: home
        emptyDir: {}
      - name: config
        projected:
          sources:
          - secret:
              name: %[1]s-ca
          - secret:
              name: %[1]s-tls
          - configMap:
              name: kube-root-ca.crt
          - configMap:
              name: %[1]s-config
`

	policy = `
path "*" {
    capabilities = ["create", "read", "update", "delete", "list"]
}
`

	config = `
listener "tcp" {
	address = "0.0.0.0:8400"
	tls_disable = "false"
	tls_cert_file = "/config/%s.pem"
	tls_key_file  = "/config/%s-key.pem"
	tls_client_ca_file = "/config/%s-ca.pem"
	tls_require_and_verify_client_cert = "%s"
	tls_min_version = "tls12"
}
storage "file" {
	path = "/vault/data"
}
`

	// env variables that align with operator e2e
	powerflexSecretPath  = "POWERFLEX_VAULT_STORAGE_PATH"  // #nosec G101 -- env var, not hardcode
	powerflexUsername    = "POWERFLEX_USER"                // #nosec G101 -- env var, not hardcode
	powerflexPassword    = "POWERFLEX_PASS"                // #nosec G101 -- env var, not hardcode
	powerscaleSecretPath = "POWERSCALE_VAULT_STORAGE_PATH" // #nosec G101 -- env var, not hardcode
	powerscaleUsername   = "POWERSCALE_USER"               // #nosec G101 -- env var, not hardcode
	powerscalePassword   = "POWERSCALE_PASS"               // #nosec G101 -- env var, not hardcode
	powermaxSecretPath   = "POWERMAX_VAULT_STORAGE_PATH"   // #nosec G101 -- env var, not hardcode
	powermaxUsername     = "POWERMAX_USER"                 // #nosec G101 -- env var, not hardcode
	powermaxPassword     = "POWERMAX_PASS"                 // #nosec G101 -- env var, not hardcode
	powerstoreSecretPath = "POWERSTORE_VAULT_STORAGE_PATH" // #nosec G101 -- env var, not hardcode
	powerstoreUsername   = "POWERSTORE_USER"               // #nosec G101 -- env var, not hardcode
	powerstorePassword   = "POWERSTORE_PASS"               // #nosec G101 -- env var, not hardcode
	configObject         = "JWT_SIGNING_SECRET"            // #nosec G101 -- env var, not hardcode
	// timestamps to create certificates
	notBefore = time.Now()
	notAfter  = notBefore.Add(8766 * time.Hour)

	// flag for vault names
	vaultNames vaultList
)

type step func() error

type sequence struct {
	steps []step
	name  string

	kubeconfig            string
	secretsStoreCSIDriver bool
	namespace             string
	kube                  *kubernetes.Clientset
	imageRepository       string
	k8sHost               string

	caCert *x509.Certificate
	caKey  *rsa.PrivateKey

	caPem             []byte
	caKeyPem          []byte
	vaultPem          []byte
	vaultKeyPem       []byte
	vaultClientPem    []byte
	vaultClientKeyPem []byte

	vaultPodName       string
	validateClientCert bool
	secretPath         string
	username           string
	password           string
	fileConfig         string
	envConfig          bool
	openshift          bool
}

type vaultList []string

func (v vaultList) String() string {
	return ""
}

func (v *vaultList) Set(value string) error {
	*v = append(*v, value)
	return nil
}

func main() {
	// init flags
	flag.Var(&vaultNames, "name", "Name of the vault instance")
	kubeconfig := flag.String("kubeconfig", "", "Path to kubeconfig")
	secretsStoreCSIDriver := flag.Bool("secrets-store-csi-driver", false, "Enable secrets-store-csi-driver for vault")
	validateClientCert := flag.Bool("validate-client-cert", false, "Validate client certificate")
	csmAuthorizationNamespace := flag.String("csm-authorization-namespace", "authorization", "CSM Authorization namespace")
	secretPath := flag.String("secret-path", "", "Vault secret path")
	username := flag.String("username", "", "storage username")
	password := flag.String("password", "", "storage password")
	fileConfig := flag.String("file-config", "", "path to credential config")
	envConfig := flag.Bool("env-config", false, "use environment variables for credential config")
	openshift := flag.Bool("openshift", false, "Set to true for OpenShift")
	flag.Parse()

	if len(vaultNames) == 0 {
		log.Fatal("must specify at least one --name (name of vault instance)")
	}

	if *kubeconfig == "" {
		log.Fatal("must specify --kubeconfig (path to kubeconfig)")
	}

	// init k8s client

	kconfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Fatal(err)
	}

	kube, err := kubernetes.NewForConfig(kconfig)
	if err != nil {
		log.Fatal(err)
	}

	var imageRepository string
	if *openshift {
		imageRepository = "registry.connect.redhat.com/hashicorp/vault"
	} else {
		imageRepository = "hashicorp/vault"
	}

	// execute steps for each vault
	for _, name := range vaultNames {
		seq := &sequence{
			name:                  name,
			kubeconfig:            *kubeconfig,
			imageRepository:       imageRepository,
			k8sHost:               kconfig.Host,
			secretsStoreCSIDriver: *secretsStoreCSIDriver,
			namespace:             *csmAuthorizationNamespace,
			kube:                  kube,
			validateClientCert:    *validateClientCert,
			secretPath:            *secretPath,
			username:              *username,
			password:              *password,
			fileConfig:            *fileConfig,
			envConfig:             *envConfig,
			openshift:             *openshift,
		}
		seq.steps = []step{
			seq.generateCACert,
			seq.generateVaultCert,
			seq.generateClientCert,
			seq.writeCertFiles,
			seq.createConfigurationConfigMap,
			seq.createTLSSecret,
			seq.createCASecret,
			seq.installVault,
			seq.waitForVault,
			seq.enableKubernetesAuth,
			seq.configureKubernetesAuth,
			seq.configureVaultPolicy,
			seq.configureVaultRole,
			seq.writeVaultSecret,
		}

		for _, step := range seq.steps {
			err := step()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	}
}

func (s *sequence) generateCACert() error {
	log.Printf("Generating CA Certificate: %s-ca.pem\n", s.name)

	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2048),
		Subject: pkix.Name{
			CommonName: "Dell",
		},
		IsCA:                  true,
		BasicConstraintsValid: true,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
	}
	s.caCert = caTmpl

	caKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}
	s.caKey = caKey

	caBytes, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return err
	}

	s.caPem = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	s.caKeyPem = pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caKey),
	})

	return nil
}

func (s *sequence) generateVaultCert() error {
	log.Printf("Generating Vault server certificate")

	vaultCertTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2048),
		Subject: pkix.Name{
			CommonName: "Vault",
		},
		BasicConstraintsValid: true,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
	}

	vaultCertTmpl.DNSNames = []string{fmt.Sprintf("%s.default.svc.cluster.local", s.name)}

	vaultKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	vaultCertBytes, err := x509.CreateCertificate(rand.Reader, vaultCertTmpl, s.caCert, &vaultKey.PublicKey, s.caKey)
	if err != nil {
		return err
	}

	vaultPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: vaultCertBytes,
	})
	s.vaultPem = vaultPEM

	vaultKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(vaultKey),
	})
	s.vaultKeyPem = vaultKeyPEM

	return nil
}

func (s *sequence) generateClientCert() error {
	log.Printf("Generating Client certificate: %s-client.pem, %s-client-key.pem\n", s.name, s.name)

	clientCertTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2048),
		Subject: pkix.Name{
			CommonName: "Client",
		},
		BasicConstraintsValid: true,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
	}

	clientKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	clientCertBytes, err := x509.CreateCertificate(rand.Reader, clientCertTmpl, s.caCert, &clientKey.PublicKey, s.caKey)
	if err != nil {
		return err
	}

	clientPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: clientCertBytes,
	})
	s.vaultClientPem = clientPEM

	clientKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(clientKey),
	})
	s.vaultClientKeyPem = clientKeyPEM

	return nil
}

type file struct {
	name string
	data []byte
}

func (s *sequence) writeCertFiles() error {
	log.Printf("Writing CA %s and Vault client certificate files %s and %s in working directory\n",
		fmt.Sprintf("%s-ca.pem", s.name),
		fmt.Sprintf("%s-client.pem", s.name),
		fmt.Sprintf("%s-client-key.pem", s.name))

	files := []file{
		{fmt.Sprintf("%s-ca.pem", s.name), s.caPem},
		{fmt.Sprintf("%s-client.pem", s.name), s.vaultClientPem},
		{fmt.Sprintf("%s-client-key.pem", s.name), s.vaultClientKeyPem},
	}

	for _, f := range files {
		// #nosec G306 -- this is a test automation tool
		if err := os.WriteFile(f.name, f.data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func (s *sequence) createConfigurationConfigMap() error {
	log.Printf("Creating %s-config configMap in default namespace\n", s.name)

	vaultPolicy := corev1apply.ConfigMap(fmt.Sprintf("%s-config", s.name), "default")
	vaultPolicy.WithData(map[string]string{
		fmt.Sprintf("%s-policy.hcl", s.name): policy,
		fmt.Sprintf("%s-config.hcl", s.name): fmt.Sprintf(config, s.name, s.name, s.name, strconv.FormatBool(s.validateClientCert)),
	})

	_, err := s.kube.CoreV1().ConfigMaps("default").Apply(context.Background(), vaultPolicy, metav1.ApplyOptions{Force: true, FieldManager: "application/apply-patch"})
	if err != nil {
		return fmt.Errorf("applying %s configmap: %v", fmt.Sprintf("%s-config", s.name), err)
	}

	return nil
}

func (s *sequence) createTLSSecret() error {
	log.Printf("Creating %s-tls secret in default namespace\n", s.name)

	vaultTLS := corev1apply.Secret(fmt.Sprintf("%s-tls", s.name), "default")
	vaultTLS.WithData(map[string][]byte{
		fmt.Sprintf("%s.pem", s.name):     s.vaultPem,
		fmt.Sprintf("%s-key.pem", s.name): s.vaultKeyPem,
	})

	_, err := s.kube.CoreV1().Secrets("default").Apply(context.Background(), vaultTLS, metav1.ApplyOptions{Force: true, FieldManager: "application/apply-patch"})
	if err != nil {
		return fmt.Errorf("applying %s secret: %v", fmt.Sprintf("%s-tls", s.name), err)
	}

	return nil
}

func (s *sequence) createCASecret() error {
	log.Printf("Creating %s-ca secret in default namespace\n", s.name)

	vaultCA := corev1apply.Secret(fmt.Sprintf("%s-ca", s.name), "default")
	vaultCA.WithData(map[string][]byte{
		fmt.Sprintf("%s-ca.pem", s.name): s.caPem,
	})

	_, err := s.kube.CoreV1().Secrets("default").Apply(context.Background(), vaultCA, metav1.ApplyOptions{Force: true, FieldManager: "application/apply-patch"})
	if err != nil {
		return fmt.Errorf("applying %s secret: %v", fmt.Sprintf("%s-ca", s.name), err)
	}

	return nil
}

func (s *sequence) installVault() error {
	log.Printf("Installing %s via Kubernetes manifests", s.name)

	manifest := fmt.Sprintf(vaultResources, s.name)
	if s.secretsStoreCSIDriver {
		manifest += fmt.Sprintf(vaultCSIDaemonSet, s.name, s.imageRepository)
	}
	manifest += fmt.Sprintf(vaultStatefulSet, s.name, s.imageRepository, s.k8sHost)

	var b bytes.Buffer
	cmd := exec.Command("kubectl", "apply", "-f", "-") // #nosec G204 -- this is a test automation tool
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Stdout = &b
	cmd.Stderr = &b
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, b.String())
	}

	s.vaultPodName = fmt.Sprintf("pod/%s-0", s.name)
	return nil
}

func (s *sequence) waitForVault() error {
	log.Printf("Waiting for %s to be Ready\n", s.vaultPodName)

	var b bytes.Buffer
	cmd := exec.Command("kubectl", "wait", "--for=condition=Ready", "--timeout", "5m", s.vaultPodName) // #nosec G204 -- this is a test automation tool
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%v: %s", err, b.String())
	}

	return nil
}

func (s *sequence) enableKubernetesAuth() error {
	log.Printf("Enabling Kubernetes authentication in %s\n", s.name)

	var b bytes.Buffer
	cmd := exec.Command("kubectl", "exec", s.vaultPodName, "--", "sh", "-c", "vault auth enable kubernetes") // #nosec G204 -- this is a test automation tool
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%v: %s", err, b.String())
	}

	return nil
}

func (s *sequence) configureKubernetesAuth() error {
	log.Printf("Configuring Kubernetes authentication in %s\n", s.name)

	var b bytes.Buffer
	cmd := exec.Command("kubectl", "exec", s.vaultPodName, "--", // #nosec G204 -- this is a test automation tool
		"sh", "-c", `vault write auth/kubernetes/config kubernetes_host="$KUBERNETES_HOST" kubernetes_ca_cert=@/config/ca.crt disable_local_ca_jwt=true`) // #nosec G204 -- this is a test automation tool
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%v: %s", err, b.String())
	}

	return nil
}

func (s *sequence) configureVaultPolicy() error {
	log.Printf("Configuring policy %s\n", s.name)

	var b bytes.Buffer
	cmd := exec.Command("kubectl", "exec", s.vaultPodName, "--", "sh", "-c", fmt.Sprintf("vault policy write csm-authorization /config/%s-policy.hcl", s.name)) // #nosec G204 -- this is a test automation tool
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%v: %s", err, b.String())
	}

	return nil
}

func (s *sequence) configureVaultRole() error {
	log.Printf("Configuring role %s\n", s.name)

	var b bytes.Buffer
	vaultCmd := fmt.Sprintf("vault write auth/kubernetes/role/csm-authorization token_ttl=60s bound_service_account_names=storage-service,tenant-service,proxy-server,sentinel,redis bound_service_account_namespaces=%s policies=csm-authorization", s.namespace)
	cmd := exec.Command("kubectl", "exec", s.vaultPodName, "--", "sh", "-c", vaultCmd) // #nosec G204 -- this is a test automation tool
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%v: %s", err, b.String())
	}

	return nil
}

type credentialConfig struct {
	Path     string `yaml:"path"`
	Username string `yaml:"username"`
	Password string `yaml:"password"` //gosec:disable G117
}

func (s *sequence) writeVaultSecret() error {
	if s.secretPath != "" && s.username != "" && s.password != "" {
		return s.putVaultSecret(s.secretPath, s.username, s.password)
	} else if s.fileConfig != "" {
		return s.handleFileConfig(s.fileConfig)
	} else if s.envConfig {
		return s.handleEnvConfig()
	}
	return nil
}

func (s *sequence) handleFileConfig(path string) error {
	b, err := os.ReadFile(path) // #nosec G304 -- this is a test automation tool
	if err != nil {
		return fmt.Errorf("reading credential config %s: %v", path, err)
	}

	var config []credentialConfig
	err = yaml.Unmarshal(b, &config)
	if err != nil {
		return fmt.Errorf("parsing credential config %s: %v", path, err)
	}

	for _, c := range config {
		err := s.putVaultSecret(c.Path, c.Username, c.Password)
		if err != nil {
			return fmt.Errorf("writing secret %s: %v", c.Path, err)
		}
	}
	return nil
}

func (s *sequence) handleEnvConfig() error {
	// redis test automation credentials
	redisPath := "redis"
	redisUsername := "test"
	redisPassword := "test"

	pflexPath := os.Getenv(powerflexSecretPath)
	pflexUsername := os.Getenv(powerflexUsername)
	pflexPassword := os.Getenv(powerflexPassword)

	if pflexPath != "" && pflexUsername != "" && pflexPassword != "" {
		err := s.putVaultSecret(pflexPath, pflexUsername, pflexPassword)
		if err != nil {
			return fmt.Errorf("writing secret %s: %v", pflexPath, err)
		}
	}

	pscalePath := os.Getenv(powerscaleSecretPath)
	pscaleUsername := os.Getenv(powerscaleUsername)
	pscalePassword := os.Getenv(powerscalePassword)

	if pscalePath != "" && pscaleUsername != "" && pscalePassword != "" {
		err := s.putVaultSecret(pscalePath, pscaleUsername, pscalePassword)
		if err != nil {
			return fmt.Errorf("writing secret %s: %v", pscalePath, err)
		}
	}

	pmaxPath := os.Getenv(powermaxSecretPath)
	pmaxUsername := os.Getenv(powermaxUsername)
	pmaxPassword := os.Getenv(powermaxPassword)

	if pmaxPath != "" && pmaxUsername != "" && pmaxPassword != "" {
		err := s.putVaultSecret(pmaxPath, pmaxUsername, pmaxPassword)
		if err != nil {
			return fmt.Errorf("writing secret %s: %v", pmaxPath, err)
		}
	}

	pstorePath := os.Getenv(powerstoreSecretPath)
	pstoreUsername := os.Getenv(powerstoreUsername)
	pstorePassword := os.Getenv(powerstorePassword)

	if pstorePath != "" && pstoreUsername != "" && pstorePassword != "" {
		err := s.putVaultSecret(pstorePath, pstoreUsername, pstorePassword)
		if err != nil {
			return fmt.Errorf("writing secret %s: %v", pstorePath, err)
		}
	}

	// add redis credentials to vault
	err := s.putVaultSecret(redisPath, redisUsername, redisPassword)
	if err != nil {
		return fmt.Errorf("writing secret %s: %v", redisPath, err)
	}

	// add config containing jwt secret to vault
	configPath := "config"
	config := os.Getenv(configObject)
	if config != "" {
		err := s.putVaultConfigSecret(configPath, config)
		if err != nil {
			return fmt.Errorf("writing config secret %s: %v", configPath, err)
		}
	}

	return nil
}

func (s *sequence) putVaultSecret(path, username, password string) error {
	log.Printf("Writing secret %s in %s", path, s.name) //gosec:disable G706 -- this is a test automation tool
	var b bytes.Buffer
	vaultCmd := fmt.Sprintf("vault kv put -mount=secret %s password=%s username=%s", path, password, username)
	cmd := exec.Command("kubectl", "exec", s.vaultPodName, "--", "sh", "-c", vaultCmd) // #nosec G204, G702 -- this is a test automation tool
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%v: %s", err, b.String())
	}
	return nil
}

func (s *sequence) putVaultConfigSecret(path, config string) error {
	log.Printf("Writing config secret %s in %s", path, s.name) //gosec:disable G706 -- this is a test automation tool
	var b bytes.Buffer
	vaultCmd := fmt.Sprintf("vault kv put -mount=secret %s configKey=\"%s\"", path, config)
	cmd := exec.Command("kubectl", "exec", s.vaultPodName, "--", "sh", "-c", vaultCmd) // #nosec G204, G702 -- this is a test automation tool
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%v: %s", err, b.String())
	}
	return nil
}
