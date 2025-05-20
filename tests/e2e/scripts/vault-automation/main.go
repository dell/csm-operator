// Copyright © 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	values = `
injector:
  enabled: false
server:
  service:
    port: 8400
    targetPort: 8400
  extraEnvironmentVars:
    KUBERNETES_HOST: %s
  extraArgs: "-config=/config/%s-config.hcl"
  standalone:
    enabled: true
  authDelegator:
    enabled: false
  dev:
    enabled: true
  volumeMounts:
    - name: config
      mountPath: /config
  volumes:
    - name: config
      projected:
        sources:
        - secret:
            name: %s-ca
        - secret:
            name: %s-tls
        - configMap:
            name: kube-root-ca.crt
        - configMap:
            name: %s-config
ui:
  enabled: true
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
	powerflexSecretPath  = "PFLEX_VAULT_STORAGE_PATH"
	powerflexUsername    = "PFLEX_USER"
	powerflexPassword    = "PFLEX_PASS"
	powerscaleSecretPath = "PSCALE_VAULT_STORAGE_PATH"
	powerscaleUsername   = "PSCALE_USER"
	powerscalePassword   = "PSCALE_PASS"
	powermaxSecretPath   = "PMAX_VAULT_STORAGE_PATH"
	powermaxUsername     = "PMAX_USER"
	powermaxPassword     = "PMAX_PASS"

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

	kubeconfig string
	namespace  string
	kube       *kubernetes.Clientset

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
	validateClientCert := flag.Bool("validate-client-cert", false, "Validate client certificate")
	csmAuthorizationNamespace := flag.String("csm-authorization-namespace", "authorization", "CSM Authorization namespace")
	secretPath := flag.String("secret-path", "", "Vault secret path")
	username := flag.String("username", "", "storage username")
	password := flag.String("password", "", "storage password")
	fileConfig := flag.String("file-config", "", "path to credential config")
	envConfig := flag.Bool("env-config", false, "use environment variables for credential config")
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

	// execute steps for each vault
	for _, name := range vaultNames {
		seq := &sequence{
			name:               name,
			kubeconfig:         *kubeconfig,
			namespace:          *csmAuthorizationNamespace,
			kube:               kube,
			validateClientCert: *validateClientCert,
			secretPath:         *secretPath,
			username:           *username,
			password:           *password,
			fileConfig:         *fileConfig,
			envConfig:          *envConfig,
		}
		seq.steps = []step{
			seq.generateCACert,
			seq.generateVaultCert,
			seq.generateClientCert,
			seq.writeCertFiles,
			seq.createConfigurationConfigMap,
			seq.createTLSSecret,
			seq.createCASecret,
			seq.configureValues,
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

		// cleanup

		_ = os.Remove(fmt.Sprintf("%s-values.yaml", seq.name))
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

func (s *sequence) configureValues() error {
	log.Println("Configuring values file for Vault helm chart")

	var b bytes.Buffer
	cmd := exec.Command("kubectl", "config", "view", "--raw", "--minify", "--flatten", `--output=jsonpath={.clusters[].cluster.server}`)
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", s.kubeconfig))
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%v: %s", err, b.String())
	}
	k8sHost := b.String()

	err = os.WriteFile(fmt.Sprintf("%s-values.yaml", s.name), []byte(fmt.Sprintf(values, k8sHost, s.name, s.name, s.name, s.name)), 0644)
	if err != nil {
		return err
	}

	return nil
}

func (s *sequence) installVault() error {
	log.Printf("Installing %s via helm", s.name)

	var b bytes.Buffer
	cmd := exec.Command("helm", "repo", "add", "hashicorp", "https://helm.releases.hashicorp.com")
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%v: %s", err, b.String())
	}

	b.Reset()
	cmd = exec.Command("helm", "install", s.name, "hashicorp/vault", "-f", fmt.Sprintf("%s-values.yaml", s.name))
	cmd.Stdout = &b
	cmd.Stderr = &b
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("%v: %s", err, b.String())
	}

	s.vaultPodName = fmt.Sprintf("pod/%s-0", s.name)
	return nil
}

func (s *sequence) waitForVault() error {
	log.Printf("Waiting for %s to be Ready\n", s.vaultPodName)

	var b bytes.Buffer
	cmd := exec.Command("kubectl", "wait", "--for=condition=Ready", "--timeout", "5m", s.vaultPodName)
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
	cmd := exec.Command("kubectl", "exec", s.vaultPodName, "--", "sh", "-c", "vault auth enable kubernetes")
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
	cmd := exec.Command("kubectl", "exec", s.vaultPodName, "--",
		"sh", "-c", `vault write auth/kubernetes/config kubernetes_host="$KUBERNETES_HOST" kubernetes_ca_cert=@/config/ca.crt disable_local_ca_jwt=true`)
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
	cmd := exec.Command("kubectl", "exec", s.vaultPodName, "--", "sh", "-c", fmt.Sprintf("vault policy write csm-authorization /config/%s-policy.hcl", s.name))
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
	vaultCmd := fmt.Sprintf("vault write auth/kubernetes/role/csm-authorization token_ttl=60s bound_service_account_names=storage-service bound_service_account_namespaces=%s policies=csm-authorization", s.namespace)
	cmd := exec.Command("kubectl", "exec", s.vaultPodName, "--", "sh", "-c", vaultCmd)
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
	Password string `yaml:"password"`
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
	b, err := os.ReadFile(path)
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
	return nil
}

func (s *sequence) putVaultSecret(path, username, password string) error {
	log.Printf("Writing secret %s in %s", path, s.name)
	var b bytes.Buffer
	vaultCmd := fmt.Sprintf("vault kv put -mount=secret %s password=%s username=%s", path, password, username)
	cmd := exec.Command("kubectl", "exec", s.vaultPodName, "--", "sh", "-c", vaultCmd)
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%v: %s", err, b.String())
	}
	return nil
}
