//  Copyright © 2022-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//       http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package steps

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/constants"
	"github.com/dell/csm-operator/pkg/modules"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	"github.com/dell/csm-operator/pkg/version"
	"golang.org/x/mod/semver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/kubectl"
	fpod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	roleName           = "CSIGold"
	tenantName         = "PancakeGroup"
	certManagerVersion = "v1.11.0"
)

var (
	authString         = "karavi-authorization-proxy"
	operatorNamespace  = "dell-csm-operator"
	quotaLimit         = "100000000"
	powerflexSecretMap = map[string]string{ // #nosec G101
		"REPLACE_USER": "POWERFLEX_USER", "REPLACE_PASS": "POWERFLEX_PASS", "REPLACE_SYSTEMID": "POWERFLEX_SYSTEMID", "REPLACE_ENDPOINT": "POWERFLEX_ENDPOINT", "REPLACE_MDM": "POWERFLEX_MDM", "REPLACE_PROTOCOL": "POWERFLEX_PROTOCOL", "REPLACE_POOL": "POWERFLEX_POOL", "REPLACE_NAS": "POWERFLEX_NAS", "REPLACE_SFTP_REPO_ADDRESS": "POWERFLEX_SFTP_REPO_ADDRESS", "REPLACE_SFTP_REPO_USER": "POWERFLEX_SFTP_REPO_USER",
		"REPLACE_ZONING_USER": "POWERFLEX_ZONING_USER", "REPLACE_ZONING_PASS": "POWERFLEX_ZONING_PASS", "REPLACE_ZONING_SYSTEMID": "POWERFLEX_ZONING_SYSTEMID", "REPLACE_ZONING_ENDPOINT": "POWERFLEX_ZONING_ENDPOINT", "REPLACE_ZONING_MDM": "POWERFLEX_ZONING_MDM", "REPLACE_ZONING_POOL": "POWERFLEX_ZONING_POOL", "REPLACE_ZONING_NAS": "POWERFLEX_ZONING_NAS",
		"REPLACE_OIDC_CLIENTID": "POWERFLEX_OIDC_CLIENTID", "REPLACE_OIDC_CLIENT_SECRET": "POWERFLEX_OIDC_CLIENT_SECRET", "REPLACE_CIAM_CLIENTID": "POWERFLEX_CIAM_CLIENTID", "REPLACE_CIAM_CLIENT_SECRET": "POWERFLEX_CIAM_CLIENT_SECRET", "REPLACE_ISSUER": "POWERFLEX_ISSUER", "REPLACE_SCOPE": "POWERFLEX_SCOPE",
	} //gosec:disable G101 -- this is a test automation tool
	powerflexAuthSecretMap         = map[string]string{"REPLACE_USER": "POWERFLEX_USER", "REPLACE_PASS": "POWERFLEX_PASS", "REPLACE_SYSTEMID": "POWERFLEX_SYSTEMID", "REPLACE_ENDPOINT": "POWERFLEX_AUTH_ENDPOINT", "REPLACE_MDM": "POWERFLEX_MDM", "REPLACE_PROTOCOL": "POWERFLEX_PROTOCOL"}
	powerscaleSecretMap            = map[string]string{"REPLACE_CLUSTERNAME": "POWERSCALE_CLUSTER", "REPLACE_USER": "POWERSCALE_USER", "REPLACE_PASS": "POWERSCALE_PASS", "REPLACE_ENDPOINT": "POWERSCALE_ENDPOINT", "REPLACE_PORT": "POWERSCALE_PORT", "REPLACE_MULTI_CLUSTERNAME": "POWERSCALE_MULTI_CLUSTER", "REPLACE_MULTI_USER": "POWERSCALE_MULTI_USER", "REPLACE_MULTI_PASS": "POWERSCALE_MULTI_PASS", "REPLACE_MULTI_ENDPOINT": "POWERSCALE_MULTI_ENDPOINT", "REPLACE_MULTI_PORT": "POWERSCALE_MULTI_PORT", "REPLACE_MULTI_AUTH_ENDPOINT": "POWERSCALE_MULTI_AUTH_ENDPOINT", "REPLACE_MULTI_AUTH_PORT": "POWERSCALE_MULTI_AUTH_PORT"} //gosec:disable G101 -- this is a test automation tool
	powerscaleAuthSecretMap        = map[string]string{"REPLACE_CLUSTERNAME": "POWERSCALE_CLUSTER", "REPLACE_USER": "POWERSCALE_USER", "REPLACE_PASS": "POWERSCALE_PASS", "REPLACE_AUTH_ENDPOINT": "POWERSCALE_AUTH_ENDPOINT", "REPLACE_AUTH_PORT": "POWERSCALE_AUTH_PORT", "REPLACE_ENDPOINT": "POWERSCALE_ENDPOINT", "REPLACE_PORT": "POWERSCALE_PORT"}
	powerscaleAuthSidecarMap       = map[string]string{"REPLACE_CLUSTERNAME": "POWERSCALE_CLUSTER", "REPLACE_ENDPOINT": "POWERSCALE_ENDPOINT", "REPLACE_AUTH_ENDPOINT": "POWERSCALE_AUTH_ENDPOINT", "REPLACE_AUTH_PORT": "POWERSCALE_AUTH_PORT", "REPLACE_PORT": "POWERSCALE_PORT"}
	powerscaleEphemeralVolumeMap   = map[string]string{"REPLACE_CLUSTERNAME": "POWERSCALE_CLUSTER", "REPLACE_ENDPOINT": "POWERSCALE_ENDPOINT"}
	powerscaleDirectoryBackedScMap = map[string]string{"REPLACE_SHARED_EXPORT_PATH": "POWERSCALE_SHARED_EXPORT_PATH"}
	powerflexEphemeralVolumeMap    = map[string]string{"REPLACE_SYSTEMID": "POWERFLEX_SYSTEMID", "REPLACE_POOL": "POWERFLEX_POOL", "REPLACE_VOLUME": "POWERFLEX_VOLUME"}
	powerflexAuthSidecarMap        = map[string]string{"REPLACE_USER": "POWERFLEX_USER", "REPLACE_PASS": "POWERFLEX_PASS", "REPLACE_SYSTEMID": "POWERFLEX_SYSTEMID", "REPLACE_ENDPOINT": "POWERFLEX_ENDPOINT", "REPLACE_AUTH_ENDPOINT": "POWERFLEX_AUTH_ENDPOINT"}
	powermaxCredMap                = map[string]string{"REPLACE_USER": "POWERMAX_USER_ENCODED", "REPLACE_PASS": "POWERMAX_PASS_ENCODED"} //gosec:disable G101 -- this is a test automation tool
	powermaxSecretMap              = map[string]string{
		"REPLACE_USERNAME": "POWERMAX_USER", "REPLACE_PASSWORD": "POWERMAX_PASS", "REPLACE_SYSTEMID": "POWERMAX_SYSTEMID", "REPLACE_ENDPOINT": "POWERMAX_ENDPOINT",
		"REPLACE_ZONING_USERNAME": "POWERMAX_ZONING_USER", "REPLACE_ZONING_PASSWORD": "POWERMAX_ZONING_PASS", "REPLACE_ZONING_SYSTEMID": "POWERMAX_ZONING_SYSTEMID", "REPLACE_ZONING_ENDPOINT": "POWERMAX_ZONING_ENDPOINT",
	}
	powermaxAuthSidecarMap     = map[string]string{"REPLACE_SYSTEMID": "POWERMAX_SYSTEMID", "REPLACE_ENDPOINT": "POWERMAX_ENDPOINT", "REPLACE_AUTH_ENDPOINT": "POWERMAX_AUTH_ENDPOINT"}
	powermaxStorageMap         = map[string]string{"REPLACE_USER": "POWERMAX_USER", "REPLACE_PASS": "POWERMAX_PASS", "REPLACE_SYSTEMID": "POWERMAX_SYSTEMID", "REPLACE_RESOURCE_POOL": "POWERMAX_POOL_V1", "REPLACE_SERVICE_LEVEL": "POWERMAX_SERVICE_LEVEL"}
	powermaxReverseProxyMap    = map[string]string{"REPLACE_SYSTEMID": "POWERMAX_SYSTEMID", "REPLACE_ENDPOINT": "POWERMAX_ENDPOINT", "REPLACE_AUTH_ENDPOINT": "POWERMAX_AUTH_ENDPOINT"}
	authSidecarRootCertMap     = map[string]string{}
	powermaxArrayConfigMap     = map[string]string{"REPLACE_PORTGROUPS": "POWERMAX_PORTGROUPS", "REPLACE_PROTOCOL": "POWERMAX_PROTOCOL", "REPLACE_ARRAYS": "POWERMAX_ARRAYS", "REPLACE_ENDPOINT": "POWERMAX_ENDPOINT"}
	powermaxAuthArrayConfigMap = map[string]string{"REPLACE_PORTGROUPS": "POWERMAX_PORTGROUPS", "REPLACE_PROTOCOL": "POWERMAX_PROTOCOL", "REPLACE_ARRAYS": "POWERMAX_ARRAYS", "REPLACE_ENDPOINT": "POWERMAX_AUTH_ENDPOINT"}
	// Auth V2
	powerflexCrMap  = map[string]string{"REPLACE_STORAGE_NAME": "POWERFLEX_STORAGE", "REPLACE_STORAGE_TYPE": "POWERFLEX_STORAGE", "REPLACE_ENDPOINT": "POWERFLEX_ENDPOINT", "REPLACE_SYSTEM_ID": "POWERFLEX_SYSTEMID", "REPLACE_VAULT_STORAGE_PATH": "POWERFLEX_VAULT_STORAGE_PATH", "REPLACE_ROLE_NAME": "POWERFLEX_ROLE", "REPLACE_QUOTA": "POWERFLEX_QUOTA", "REPLACE_STORAGE_POOL_PATH": "POWERFLEX_POOL", "REPLACE_TENANT_NAME": "POWERFLEX_TENANT", "REPLACE_TENANT_ROLES": "POWERFLEX_ROLE", "REPLACE_TENANT_VOLUME_PREFIX": "POWERFLEX_TENANT_PREFIX", "REPLACE_USERNAME_OBJECT_NAME": "secrets/powerflex-username", "REPLACE_PASSWORD_OBJECT_NAME": "secrets/powerflex-password"}
	powerscaleCrMap = map[string]string{"REPLACE_STORAGE_NAME": "POWERSCALE_STORAGE", "REPLACE_STORAGE_TYPE": "POWERSCALE_STORAGE", "REPLACE_ENDPOINT": "POWERSCALE_ENDPOINT", "REPLACE_SYSTEM_ID": "POWERSCALE_CLUSTER", "REPLACE_VAULT_STORAGE_PATH": "POWERSCALE_VAULT_STORAGE_PATH", "REPLACE_ROLE_NAME": "POWERSCALE_ROLE", "REPLACE_QUOTA": "POWERSCALE_QUOTA", "REPLACE_STORAGE_POOL_PATH": "POWERSCALE_POOL_V2", "REPLACE_TENANT_NAME": "POWERSCALE_TENANT", "REPLACE_TENANT_ROLES": "POWERSCALE_ROLE", "REPLACE_TENANT_VOLUME_PREFIX": "POWERSCALE_TENANT_PREFIX", "REPLACE_USERNAME_OBJECT_NAME": "secrets/powerscale-username", "REPLACE_PASSWORD_OBJECT_NAME": "secrets/powerscale-password"}
	powermaxCrMap   = map[string]string{"REPLACE_STORAGE_NAME": "POWERMAX_STORAGE", "REPLACE_STORAGE_TYPE": "POWERMAX_STORAGE", "REPLACE_ENDPOINT": "POWERMAX_ENDPOINT", "REPLACE_SYSTEM_ID": "POWERMAX_SYSTEMID", "REPLACE_VAULT_STORAGE_PATH": "POWERMAX_VAULT_STORAGE_PATH", "REPLACE_ROLE_NAME": "POWERMAX_ROLE", "REPLACE_QUOTA": "POWERMAX_QUOTA", "REPLACE_STORAGE_POOL_PATH": "POWERMAX_POOL_V2", "REPLACE_TENANT_NAME": "POWERMAX_TENANT", "REPLACE_TENANT_ROLES": "POWERMAX_ROLE", "REPLACE_TENANT_VOLUME_PREFIX": "POWERMAX_TENANT_PREFIX", "REPLACE_USERNAME_OBJECT_NAME": "secrets/powermax-username", "REPLACE_PASSWORD_OBJECT_NAME": "secrets/powermax-password"}
	powerstoreCrMap = map[string]string{"REPLACE_STORAGE_NAME": "POWERSTORE_STORAGE", "REPLACE_STORAGE_TYPE": "POWERSTORE_STORAGE", "REPLACE_ENDPOINT": "POWERSTORE_ENDPOINT", "REPLACE_SYSTEM_ID": "POWERSTORE_GLOBALID", "REPLACE_VAULT_STORAGE_PATH": "POWERSTORE_VAULT_STORAGE_PATH", "REPLACE_ROLE_NAME": "POWERSTORE_ROLE", "REPLACE_QUOTA": "POWERSTORE_QUOTA", "REPLACE_STORAGE_POOL_PATH": "POWERSTORE_POOL", "REPLACE_TENANT_NAME": "POWERSTORE_TENANT", "REPLACE_TENANT_ROLES": "POWERSTORE_ROLE", "REPLACE_TENANT_VOLUME_PREFIX": "POWERSTORE_TENANT_PREFIX", "REPLACE_USERNAME_OBJECT_NAME": "secrets/powerstore-username", "REPLACE_PASSWORD_OBJECT_NAME": "secrets/powerstore-password"}

	powerstoreSecretMap          = map[string]string{"REPLACE_USER": "POWERSTORE_USER", "REPLACE_PASS": "POWERSTORE_PASS", "REPLACE_GLOBALID": "POWERSTORE_GLOBALID", "REPLACE_ENDPOINT": "POWERSTORE_ENDPOINT", "REPLACE_PROTOCOL": "POWERSTORE_PROTOCOL"}
	powerstoreEphemeralVolumeMap = map[string]string{"REPLACE_GLOBALID": "POWERSTORE_GLOBALID"}
	powerstoreAuthSecretMap      = map[string]string{"REPLACE_USER": "POWERSTORE_USER", "REPLACE_PASS": "POWERSTORE_PASS", "REPLACE_GLOBALID": "POWERSTORE_GLOBALID", "REPLACE_ENDPOINT": "POWERSTORE_AUTH_ENDPOINT", "REPLACE_PROTOCOL": "POWERSTORE_PROTOCOL"}
	powerstoreAuthSidecarMap     = map[string]string{"REPLACE_USER": "POWERSTORE_USER", "REPLACE_PASS": "POWERSTORE_PASS", "REPLACE_SYSTEMID": "POWERSTORE_GLOBALID", "REPLACE_ENDPOINT": "POWERSTORE_ENDPOINT", "REPLACE_AUTH_ENDPOINT": "POWERSTORE_AUTH_ENDPOINT"}
	// Metro configuration for PowerStore dual-array setup
	powerstoreMetroSecretMap  = map[string]string{"REPLACE_USER": "POWERSTORE_USER", "REPLACE_PASS": "POWERSTORE_PASS", "REPLACE_GLOBALID": "POWERSTORE_GLOBALID", "REPLACE_ENDPOINT": "POWERSTORE_ENDPOINT", "REPLACE_PROTOCOL": "POWERSTORE_PROTOCOL", "REPLACE_METRO_USER": "POWERSTORE_METRO_USER", "REPLACE_METRO_PASS": "POWERSTORE_METRO_PASS", "REPLACE_METRO_GLOBALID": "POWERSTORE_METRO_GLOBALID", "REPLACE_METRO_ENDPOINT": "POWERSTORE_METRO_ENDPOINT"}
	powerstoreMetroStorageMap = map[string]string{"REPLACE_GLOBALID": "POWERSTORE_GLOBALID", "REPLACE_METRO_REMOTE_SYSTEM": "POWERSTORE_METRO_REMOTE_SYSTEM"}
	unitySecretMap            = map[string]string{"REPLACE_USER": "UNITY_USER", "REPLACE_PASS": "UNITY_PASS", "REPLACE_ARRAYID": "UNITY_ARRAYID", "REPLACE_ENDPOINT": "UNITY_ENDPOINT", "REPLACE_POOL": "UNITY_POOL", "REPLACE_NAS": "UNITY_NAS"}
	unityEphemeralVolumeMap   = map[string]string{"REPLACE_ARRAYID": "UNITY_ARRAYID", "REPLACE_POOL": "UNITY_POOL", "REPLACE_NAS": "UNITY_NAS"}

	cosiSecretMap = map[string]string{"REPLACE_USER": "COSI_USER", "REPLACE_PASS": "COSI_PASS", "REPLACE_NAMESPACE": "COSI_NAMESPACE", "REPLACE_MGMT_ENDPOINT": "COSI_MGMT_ENDPOINT", "REPLACE_S3_ENDPOINT": "COSI_S3_ENDPOINT"}

	// authV2SetupDone tracks which drivers have completed the one-time
	// AuthorizationV2 resource setup (template rendering, admin token,
	// kubectl apply). Only the token generation step needs to be retried.
	authV2SetupDone = map[string]bool{}
)

// ResetPerScenarioState clears in-memory state that should not persist across
// scenarios.  Call this at the start of each scenario, alongside the temp/
// directory cleanup, so that auth setup always re-runs with the correct paths
// for the current scenario.
func ResetPerScenarioState() {
	authV2SetupDone = map[string]bool{}
	lastAuthCRRecreation = map[string]time.Time{}
}

var correctlyAuthInjected = func(cr csmv1.ContainerStorageModule, annotations map[string]string, vols []acorev1.VolumeApplyConfiguration, cnt []acorev1.ContainerApplyConfiguration, ctrlClient client.Client) error {
	authModule := csmv1.Module{}
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Authorization {
			authModule = m
			break
		}
	}
	authConfigVersion := authModule.ConfigVersion
	// When spec.version is used instead of module-level configVersion, the operator
	// resolves the config version and stores it in an annotation on the CSM CR.
	if authConfigVersion == "" {
		if crAnnotations := cr.GetAnnotations(); crAnnotations != nil {
			authConfigVersion = crAnnotations["storage.dell.com/CSMOperatorConfigVersion"]
		}
	}

	err := modules.CheckAnnotationAuth(annotations)
	if err != nil {
		return err
	}

	err = modules.CheckApplyVolumesAuth(vols, authConfigVersion, string(cr.Spec.Driver.CSIDriverType), cr, ctrlClient)
	if err != nil {
		return err
	}

	err = modules.CheckApplyContainersAuth(cnt, string(cr.Spec.Driver.CSIDriverType), true, authConfigVersion, cr, ctrlClient)
	if err != nil {
		return err
	}
	return nil
}

// ParseScenarios reads the scenarios YAML file and returns the scenario
// metadata without loading any custom resource files. Use this when you
// want to determine which files will be needed before generating them.
func ParseScenarios(valuesFilePath string) ([]Scenario, error) {
	b, err := os.ReadFile(valuesFilePath) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("failed to read values file: %v", err)
	}
	var scenarios []Scenario
	if err := yaml.Unmarshal(b, &scenarios); err != nil {
		return nil, fmt.Errorf("failed to unmarshal scenarios: %v", err)
	}
	// Expand version tokens (e.g. {authorization-proxy-server}) in paths.
	if info := version.GetInfo(); info != nil {
		for i := range scenarios {
			for j, p := range scenarios[i].Paths {
				scenarios[i].Paths[j] = info.ExpandPathTokens(p)
			}
		}
	}
	return scenarios, nil
}

// LoadResourceForScenario generates any on-demand test files referenced by
// the scenario, then reads and unmarshals the custom resource YAML for each
// path. Call EnsureTestfileGenerated for each path before reading.
func LoadResourceForScenario(scene Scenario) (Resource, error) {
	// Create a deep copy of the Paths array to avoid collisions between scenarios
	copiedScene := scene
	copiedScene.Paths = make([]string, len(scene.Paths))
	copy(copiedScene.Paths, scene.Paths)

	var customResources []interface{}
	for _, path := range copiedScene.Paths {
		if err := EnsureTestfileGenerated(path); err != nil {
			return Resource{}, fmt.Errorf("generate testfile %s: %v", path, err)
		}

		b, err := os.ReadFile(path) // #nosec G304
		if err != nil {
			return Resource{}, fmt.Errorf("failed to read testdata: %v", err)
		}

		expanded := os.ExpandEnv(string(b))

		customResource := csmv1.ContainerStorageModule{}
		if err := yaml.Unmarshal([]byte(expanded), &customResource); err != nil {
			return Resource{}, fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
		}
		customResources = append(customResources, customResource)
	}
	return Resource{
		Scenario:       copiedScene,
		CustomResource: customResources,
	}, nil
}

// GetTestResources -- parse values file
func GetTestResources(valuesFilePath string) ([]Resource, error) {
	b, err := os.ReadFile(valuesFilePath) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("failed to read values file: %v", err)
	}

	scenarios := []Scenario{}
	err = yaml.Unmarshal(b, &scenarios)
	if err != nil {
		return nil, fmt.Errorf("failed to read unmarshal values file: %v", err)
	}

	// Expand version tokens (e.g. {authorization-proxy-server}) in paths.
	if info := version.GetInfo(); info != nil {
		for i := range scenarios {
			for j, p := range scenarios[i].Paths {
				scenarios[i].Paths[j] = info.ExpandPathTokens(p)
			}
		}
	}

	resources := []Resource{}
	for _, scene := range scenarios {
		var customResources []interface{}
		for _, path := range scene.Paths {
			b, err := os.ReadFile(path) // #nosec G304
			if err != nil {
				return nil, fmt.Errorf("failed to read testdata: %v", err)
			}

			// Expand env vars (e.g. ${E2E_NS_POWERFLEX}) so namespace fields
			// resolve to the prefix-based names at load time.
			expanded := os.ExpandEnv(string(b))

			customResource := csmv1.ContainerStorageModule{}
			err = yaml.Unmarshal([]byte(expanded), &customResource)
			if err != nil {
				return nil, fmt.Errorf("failed to read unmarshal CSM custom resource: %v", err)
			}
			customResources = append(customResources, customResource)
		}
		resources = append(resources, Resource{
			Scenario:       scene,
			CustomResource: customResources,
		})
	}

	return resources, nil
}

func (step *Step) applyCustomResource(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	crFilePath := res.Scenario.Paths[crNum-1]

	// If the specified file is a template, assume it was rendered to a temporary file earlier.
	// Attempt to read the rendered file first. If it doesn't exist, assume the specified file
	// is not a template and should be applied as-is.

	tempFilePath := getRenderedFilePath(crFilePath)
	crBuff, err := os.ReadFile(tempFilePath) // #nosec G304
	if os.IsNotExist(err) {
		// There is no corresponding rendered file, use crFilePath
		crBuff, err = os.ReadFile(crFilePath) // #nosec G304
	}
	if err != nil {
		return fmt.Errorf("failed to read testdata: %v", err)
	}

	crContent := os.ExpandEnv(string(crBuff))
	if _, err := kubectl.RunKubectlInput(cr.Namespace, crContent, "apply", "--validate=true", "-f", "-"); err != nil {
		return fmt.Errorf("failed to apply CR %s in namespace %s: %v", cr.Name, cr.Namespace, err)
	}

	return nil
}

func (step *Step) applyRenderedYAML(_ Resource, templateFile, namespace, crType string) error {
	// Render the template with substitution
	fileString, err := renderTemplate(crType, templateFile)
	if err != nil {
		return fmt.Errorf("failed to render template %s: %v", templateFile, err)
	}

	// Substitute environment variables
	fileString = os.ExpandEnv(fileString)

	// Write rendered content to a temp file
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, filepath.Base(templateFile))
	if err := os.WriteFile(tempFile, []byte(fileString), 0644); err != nil {
		return fmt.Errorf("failed to write rendered file: %v", err)
	}
	defer os.Remove(tempFile)

	// Apply the rendered YAML
	if _, err := kubectl.RunKubectl(namespace, "apply", "-f", tempFile); err != nil {
		return fmt.Errorf("failed to apply rendered YAML %s: %v", templateFile, err)
	}

	return nil
}

func (step *Step) applyAuthorizationConjur(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	crFilePath := res.Scenario.Paths[crNum-1]

	// If the specified file is a template, assume it was rendered to a temporary file earlier.
	// Attempt to read the rendered file first. If it doesn't exist, assume the specified file
	// is not a template and should be applied as-is.

	tempFilePath := getRenderedFilePath(crFilePath)
	crBuff, err := os.ReadFile(tempFilePath) // #nosec G304
	if os.IsNotExist(err) {
		// There is no corresponding rendered file, use crFilePath
		crBuff, err = os.ReadFile(crFilePath) // #nosec G304
	}
	if err != nil {
		return fmt.Errorf("failed to read testdata: %v", err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal([]byte(os.ExpandEnv(string(crBuff))), &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	// set the conjur paths in the Authorization CR
loop:
	for moduleIndex, module := range customResource.Spec.Modules {
		if module.Name == "authorization-proxy-server" {
			for compIndex, comp := range module.Components {
				if comp.Name == "storage-system-credentials" {
					powerflexUsername := os.Getenv("POWERFLEX_USER")
					powerflexPassword := os.Getenv("POWERFLEX_PASS")
					powermaxUsername := os.Getenv("POWERMAX_USER")
					powermaxPassword := os.Getenv("POWERMAX_PASS")
					powerscaleUsername := os.Getenv("POWERSCALE_USER")
					powerscalePassword := os.Getenv("POWERSCALE_PASS")
					powerstoreUsername := os.Getenv("POWERSTORE_USER")
					powerstorePassword := os.Getenv("POWERSTORE_PASS")

					var conjurPaths []csmv1.ConjurCredentialPath
					if powerflexUsername != "" && powerflexPassword != "" {
						conjurPaths = append(conjurPaths, csmv1.ConjurCredentialPath{
							UsernamePath: "secrets/powerflex-username",
							PasswordPath: "secrets/powerflex-password",
						})
					}

					if powermaxUsername != "" && powermaxPassword != "" {
						conjurPaths = append(conjurPaths, csmv1.ConjurCredentialPath{
							UsernamePath: "secrets/powermax-username",
							PasswordPath: "secrets/powermax-password",
						})
					}

					if powerscaleUsername != "" && powerscalePassword != "" {
						conjurPaths = append(conjurPaths, csmv1.ConjurCredentialPath{
							UsernamePath: "secrets/powerscale-username",
							PasswordPath: "secrets/powerscale-password",
						})
					}

					if powerstoreUsername != "" && powerstorePassword != "" {
						conjurPaths = append(conjurPaths, csmv1.ConjurCredentialPath{
							UsernamePath: "secrets/powerstore-username",
							PasswordPath: "secrets/powerstore-password",
						})
					}

					customResource.Spec.Modules[moduleIndex].Components[compIndex].SecretProviderClasses.Conjurs[0].Paths = conjurPaths
					break loop
				}
			}
		}
	}

	dataBytes := `
CONCURRENT_STORAGE_REQUESTS: 10
LOG_LEVEL: debug
STORAGE_CAPACITY_POLL_INTERVAL: 30s`
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csm-config-params",
			Namespace: os.Getenv("E2E_NS_AUTH"),
		},
		Data: map[string]string{
			"csm-config-params.yaml": dataBytes,
		},
	}

	cmBuff, err := yaml.Marshal(cm)
	if err != nil {
		return fmt.Errorf("marshalling %s: %v", customResource.Name, err)
	}

	authBuff, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("marshalling %s: %v", customResource.Name, err)
	}

	buff := string(cmBuff) + "\n---\n" + string(authBuff)

	if _, err := kubectl.RunKubectlInput(cr.Namespace, buff, "apply", "--validate=true", "-f", "-"); err != nil {
		return fmt.Errorf("failed to apply CR %s in namespace %s: %v", cr.Name, cr.Namespace, err)
	}

	return nil
}

func (step *Step) upgradeCustomResource(res Resource, oldCrNumStr, newCrNumStr string) error {
	oldCrNum, err := parseIndex(oldCrNumStr)
	if err != nil {
		return err
	}
	oldCr := res.CustomResource[oldCrNum-1].(csmv1.ContainerStorageModule)

	newCrNum, err := parseIndex(newCrNumStr)
	if err != nil {
		return err
	}
	newCr := res.CustomResource[newCrNum-1].(csmv1.ContainerStorageModule)

	// Poll for the CSM resource to be in a ready state instead of sleeping
	// for a fixed 60 seconds. This typically completes much faster.
	fmt.Println("=== Waiting for CSM resource to be ready before upgrade ===")
	pollDeadline := time.After(60 * time.Second)
	pollTicker := time.NewTicker(3 * time.Second)
	defer pollTicker.Stop()
	for {
		select {
		case <-pollDeadline:
			fmt.Println("=== CSM readiness poll timed out after 60s, proceeding with upgrade ===")
			goto proceed
		case <-pollTicker.C:
			check := new(csmv1.ContainerStorageModule)
			if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
				Namespace: oldCr.Namespace,
				Name:      oldCr.Name,
			}, check); err == nil && check.Status.State == constants.Succeeded {
				fmt.Printf("=== CSM resource is ready (state=%s), proceeding with upgrade ===\n", check.Status.State)
				goto proceed
			}
		}
	}
proceed:

	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: oldCr.Namespace,
		Name:      oldCr.Name,
	}, found); err != nil {
		return err
	}

	// Update old CR with the spec of new CR
	found.Spec = newCr.Spec
	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) installThirdPartyModule(_ Resource, thirdPartyModule string) error {
	if thirdPartyModule == "cert-manager" {
		cmd := exec.Command("kubectl", "apply", "-f", "testfiles/cert-manager-crds.yaml")
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("cert-manager install failed: %v", err)
		}
	}
	return nil
}

func (step *Step) uninstallThirdPartyModule(res Resource, thirdPartyModule string) error {
	if thirdPartyModule == "cert-manager" {
		cmd := exec.Command("kubectl", "delete", "-f", "testfiles/cert-manager-crds.yaml", "--ignore-not-found") // #nosec G204
		err := cmd.Run()
		if err != nil {
			// Some deployments are not found since they are deleted already.
			cmd = exec.Command("kubectl", "get", "pods", "-n", "cert-manager") // #nosec G204
			err = cmd.Run()
			if err != nil {
				return fmt.Errorf("cert-manager uninstall failed: %v", err)
			}
		}
	}
	return nil
}

func (step *Step) deleteCustomResource(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	key := client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}
	err = step.ctrlClient.Get(context.TODO(), key, found)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := step.ctrlClient.Delete(context.TODO(), &cr); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// Wait for the CR to be fully removed (finalizers complete) so that the
	// next test scenario does not collide with a terminating resource.
	fmt.Printf("             Waiting for CR %s/%s to be fully deleted\n", cr.Namespace, cr.Name)
	deadline := time.After(5 * time.Minute)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out waiting for CR %s/%s to be deleted", cr.Namespace, cr.Name)
		case <-ticker.C:
			check := new(csmv1.ContainerStorageModule)
			if err := step.ctrlClient.Get(context.TODO(), key, check); err != nil {
				if errors.IsNotFound(err) {
					fmt.Printf("             CR %s/%s fully deleted\n", cr.Namespace, cr.Name)
					return nil
				}
			}
		}
	}
}

func (step *Step) validateCustomResourceStatus(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	err = step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found)
	if err != nil {
		return err
	}
	if found.Status.State != constants.Succeeded {
		return fmt.Errorf("expected custom resource status to be %s. Got: %s", constants.Succeeded, found.Status.State)
	}

	return nil
}

func (step *Step) validateContainerArg(res Resource, crNumStr string, arg string, container string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	dp, err := getDriverDeployment(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}
	containerFound := false
	for _, cnt := range dp.Spec.Template.Spec.Containers {
		if cnt.Name == container {
			containerFound = true
			// iterate through args and see if it was found
			for _, argVal := range cnt.Args {
				if argVal == arg {
					return nil
				}
			}
			return fmt.Errorf("container arg %s not found on container %s", arg, container)
		}
	}
	if !containerFound {
		return fmt.Errorf("container %s not found in deployment", container)
	}

	return fmt.Errorf("unknown error validating container arg")
}

func (step *Step) validateDeploymentContainerImage(res Resource, crNumStr string, expectedImage string, container string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	staticDp, err := getDriverDeployment(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}
	for _, cnt := range staticDp.Spec.Template.Spec.Containers {
		if cnt.Name != container {
			continue
		}
		if cnt.Image != expectedImage {
			return fmt.Errorf("expected deployment container %s image %q, got %q", container, expectedImage, cnt.Image)
		}
		return nil
	}
	return fmt.Errorf("container %s not found in deployment", container)
}

func (step *Step) validateDeploymentContainerImageContains(res Resource, crNumStr string, expectedSubstring string, container string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	staticDp, err := getDriverDeployment(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}
	for _, cnt := range staticDp.Spec.Template.Spec.Containers {
		if cnt.Name != container {
			continue
		}
		if !strings.Contains(cnt.Image, expectedSubstring) {
			return fmt.Errorf("expected deployment container %s image to contain %q, got %q", container, expectedSubstring, cnt.Image)
		}
		return nil
	}
	return fmt.Errorf("container %s not found in deployment", container)
}

func (step *Step) validateDaemonSetContainerImage(res Resource, crNumStr string, expectedImage string, container string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	staticDs, err := getDriverDaemonset(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get daemonset: %v", err)
	}
	for _, cnt := range staticDs.Spec.Template.Spec.Containers {
		if cnt.Name != container {
			continue
		}
		if cnt.Image != expectedImage {
			return fmt.Errorf("expected daemonset container %s image %q, got %q", container, expectedImage, cnt.Image)
		}
		return nil
	}
	return fmt.Errorf("container %s not found in daemonset", container)
}

func (step *Step) validateDaemonSetContainerImageContains(res Resource, crNumStr string, expectedSubstring string, container string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	staticDs, err := getDriverDaemonset(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get daemonset: %v", err)
	}
	for _, cnt := range staticDs.Spec.Template.Spec.Containers {
		if cnt.Name != container {
			continue
		}
		if !strings.Contains(cnt.Image, expectedSubstring) {
			return fmt.Errorf("expected daemonset container %s image to contain %q, got %q", container, expectedSubstring, cnt.Image)
		}
		return nil
	}
	return fmt.Errorf("container %s not found in daemonset", container)
}

func (step *Step) validateDriverInstalled(res Resource, driverName string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	return checkAllRunningPods(context.TODO(), res.CustomResource[crNum-1].(csmv1.ContainerStorageModule).Namespace, step.clientSet)
}

func (step *Step) validateMinimalCSMDriverSpec(res Resource, driverName string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	err = step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("CSM resource '%s' not found in namespace '%s'", cr.Name, cr.Namespace)
		}
		return fmt.Errorf("failed to get CSM resource '%s/%s': %w", cr.Namespace, cr.Name, err)
	}
	driver := found.Spec.Driver

	// Check that the operator has resolved the config version.
	// The operator stores the resolved version in the CSMOperatorConfigVersion
	// annotation (via applyConfigVersionAnnotations), regardless of whether the
	// CR uses spec.version or driver.configVersion. Check annotation first,
	// then fall back to driver.ConfigVersion for backward compatibility.
	annotations := found.GetAnnotations()
	configVersion := annotations["storage.dell.com/CSMOperatorConfigVersion"]
	if configVersion == "" && driver.ConfigVersion == "" {
		return fmt.Errorf("configVersion is missing: neither annotation storage.dell.com/CSMOperatorConfigVersion nor driver.configVersion is set")
	}

	if driver.CSIDriverType == "" {
		return fmt.Errorf("csiDriverType is missing")
	}

	// Ensure that the expected number of controller pods are running.
	status := found.Status
	if status.ControllerStatus.Failed > "0" {
		return fmt.Errorf("replicas should have a non-zero value")
	}

	// Ensure all other fields are empty or nil
	if len(driver.SideCars) > 0 ||
		len(driver.InitContainers) > 0 ||
		len(driver.SnapshotClass) > 0 ||
		driver.Controller != nil ||
		driver.CSIDriverSpec != nil ||
		driver.DNSPolicy != "" ||
		driver.AuthSecret != "" ||
		driver.TLSCertSecret != "" {
		return fmt.Errorf("unexpected fields found in Driver spec: %+v", driver)
	}

	return nil
}

func (step *Step) validateDriverNotInstalled(res Resource, driverName string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	check := new(csmv1.ContainerStorageModule)
	err = step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, check)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check CR status: %v", err)
	}
	if err == nil {
		return fmt.Errorf("CR %s still exists (may be deleting with finalizers)", cr.Name)
	}

	return checkNoRunningPods(context.TODO(), cr.Namespace, step.clientSet)
}

func (step *Step) setNodeLabel(res Resource, label string) error {
	if label == "control-plane" {
		_ = setNodeLabel(label, "node-role.kubernetes.io/control-plane", "")
	} else {
		return fmt.Errorf("Adding node label %s not supported, feel free to add support", label)
	}

	return nil
}

func (step *Step) removeNodeLabel(res Resource, label string) error {
	if label == "control-plane" {
		_ = removeNodeLabel(label, "node-role.kubernetes.io/control-plane")
	} else {
		return fmt.Errorf("Removing node label %s not supported, feel free to add support", label)
	}

	return nil
}

func (step *Step) validateModuleInstalled(res Resource, module string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	for _, m := range found.Spec.Modules {
		if m.Name == csmv1.ModuleType(module) {
			if !m.Enabled {
				return fmt.Errorf("%s module is not enabled in CR", m.Name)
			}
			switch m.Name {
			case csmv1.Authorization:
				return step.validateAuthorizationInstalled(*found)

			case csmv1.Replication:
				return step.validateReplicationInstalled(*found)

			case csmv1.Observability:
				return step.validateObservabilityInstalled(*found)

			case csmv1.AuthorizationServer:
				return step.validateAuthorizationProxyServerInstalled(*found)

			case csmv1.Resiliency:
				return step.validateResiliencyInstalled(*found)

			default:
				return fmt.Errorf("%s module is not found", module)
			}
		}
	}
	return nil
}

func (step *Step) validateModuleNotInstalled(res Resource, module string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	for _, m := range found.Spec.Modules {
		if m.Name == csmv1.ModuleType(module) {
			if m.Enabled {
				return fmt.Errorf("%s module is enabled in CR", m.Name)
			}
			switch m.Name {
			case csmv1.Authorization:
				return step.validateAuthorizationNotInstalled(cr)

			case csmv1.Replication:
				return step.validateReplicationNotInstalled(cr)

			case csmv1.Observability:
				return step.validateObservabilityNotInstalled(cr)

			case csmv1.AuthorizationServer:
				return step.validateAuthorizationProxyServerNotInstalled(cr)

			case csmv1.Resiliency:
				return step.validateResiliencyNotInstalled(cr)
			}
		}
	}

	return nil
}

func (step *Step) validateObservabilityInstalled(cr csmv1.ContainerStorageModule) error {
	instance := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, instance,
	); err != nil {
		return err
	}

	// check installation for all replicas
	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	csmNamespace := cr.Namespace
	clusterClient := operatorutils.GetCluster(context.TODO(), &fakeReconcile)

	// check observability in all clusters
	if err := checkObservabilityRunningPods(context.TODO(), csmNamespace, clusterClient.ClusterK8sClient); err != nil {
		return fmt.Errorf("failed to check for observability installation in %s: %v", clusterClient.ClusterID, err)
	}

	// check observability's authorization
	driverType := cr.Spec.Driver.CSIDriverType
	dpApply, err := getApplyObservabilityDeployment(csmNamespace, driverType, clusterClient.ClusterCTRLClient)
	if err != nil {
		return err
	}
	if authorizationEnabled, _ := operatorutils.IsModuleEnabled(context.TODO(), *instance, csmv1.Authorization); authorizationEnabled {
		if err := correctlyAuthInjected(cr, dpApply.Annotations, dpApply.Spec.Template.Spec.Volumes, dpApply.Spec.Template.Spec.Containers, step.ctrlClient); err != nil {
			return fmt.Errorf("failed to check for observability authorization installation in %s: %v", clusterClient.ClusterID, err)
		}
	} else {
		for _, cnt := range dpApply.Spec.Template.Spec.Containers {
			if *cnt.Name == authString {
				return fmt.Errorf("found observability authorization in deployment: %v, err:%v", dpApply.Name, err)
			}
		}
	}

	return nil
}

func (step *Step) validateObservabilityNotInstalled(cr csmv1.ContainerStorageModule) error {
	// check installation for all replicas
	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	csmNamespace := cr.Namespace
	clusterClient := operatorutils.GetCluster(context.TODO(), &fakeReconcile)

	// check observability is not installed
	if err := checkObservabilityNoRunningPods(context.TODO(), csmNamespace, clusterClient.ClusterK8sClient); err != nil {
		return fmt.Errorf("failed observability installation check %s: %v", clusterClient.ClusterID, err)
	}

	return nil
}

func (step *Step) validateReplicationInstalled(cr csmv1.ContainerStorageModule) error {
	dpApply, _, err := getApplyDeploymentDaemonSet(cr, step.ctrlClient)
	if err != nil {
		return err
	}
	if err := modules.CheckApplyContainersReplica(dpApply.Spec.Template.Spec.Containers, cr); err != nil {
		return err
	}

	// cluster role
	clusterRole := &rbacv1.ClusterRole{}
	err = step.ctrlClient.Get(context.TODO(), types.NamespacedName{
		Name: fmt.Sprintf("%s-controller", cr.Name),
	}, clusterRole)
	if err != nil {
		return err
	}
	if err := modules.CheckClusterRoleReplica(clusterRole.Rules); err != nil {
		return err
	}

	// check installation for all replicas
	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	clusterClient := operatorutils.GetCluster(context.TODO(), &fakeReconcile)

	// check replication controllers in cluster
	if err := checkAllRunningPods(context.TODO(), operatorutils.ReplicationControllerNameSpace, clusterClient.ClusterK8sClient); err != nil {
		return fmt.Errorf("failed to check for  replication controllers installation in %s: %v", clusterClient.ClusterID, err)
	}

	// check driver deployment in cluster
	if err := checkAllRunningPods(context.TODO(), cr.Namespace, clusterClient.ClusterK8sClient); err != nil {
		return fmt.Errorf("failed while check for driver installation in %s: %v", clusterClient.ClusterID, err)
	}

	return nil
}

func (step *Step) validateReplicationNotInstalled(cr csmv1.ContainerStorageModule) error {
	// check installation for all replicas
	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	clusterClient := operatorutils.GetCluster(context.TODO(), &fakeReconcile)

	// check replication  controller is not installed
	if err := checkNoRunningPods(context.TODO(), operatorutils.ReplicationControllerNameSpace, clusterClient.ClusterK8sClient); err != nil {
		return fmt.Errorf("failed replica installation check %s: %v", clusterClient.ClusterID, err)
	}

	// check that replication sidecar is not in source cluster
	dp, err := getDriverDeployment(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}
	for _, cnt := range dp.Spec.Template.Spec.Containers {
		if cnt.Name == operatorutils.ReplicationSideCarName {
			return fmt.Errorf("found %s: %v", operatorutils.ReplicationSideCarName, err)
		}
	}

	return nil
}

func (step *Step) validateAuthorizationInstalled(cr csmv1.ContainerStorageModule) error {
	dpApply, dsApply, err := getApplyDeploymentDaemonSet(cr, step.ctrlClient)
	if err != nil {
		return err
	}

	if err := correctlyAuthInjected(cr, dpApply.Annotations, dpApply.Spec.Template.Spec.Volumes, dpApply.Spec.Template.Spec.Containers, step.ctrlClient); err != nil {
		return err
	}

	return correctlyAuthInjected(cr, dsApply.Annotations, dsApply.Spec.Template.Spec.Volumes, dsApply.Spec.Template.Spec.Containers, step.ctrlClient)
}

func (step *Step) validateAuthorizationNotInstalled(cr csmv1.ContainerStorageModule) error {
	dpApply, dsApply, err := getApplyDeploymentDaemonSet(cr, step.ctrlClient)
	if err != nil {
		return err
	}

	for _, cnt := range dpApply.Spec.Template.Spec.Containers {
		if *cnt.Name == authString {
			return fmt.Errorf("found authorization in deployment: %v", err)
		}
	}

	for _, cnt := range dsApply.Spec.Template.Spec.Containers {
		if *cnt.Name == authString {
			return fmt.Errorf("found authorization in daemonset: %v", err)
		}
	}

	return nil
}

func (step *Step) validateAuthorizationPodsNotInstalled(res Resource, module string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	check := new(csmv1.ContainerStorageModule)
	err = step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, check)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check CR status: %v", err)
	}
	if err == nil {
		return fmt.Errorf("CR %s still exists (may be deleting with finalizers)", cr.Name)
	}

	return checkNoRunningPods(context.TODO(), cr.Namespace, step.clientSet)
}

func isStorageClassDeleteNotFound(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "notfound") || strings.Contains(message, "not found")
}

func (step *Step) setUpStorageClass(_ Resource, templateFile, crType string) error {
	// Skip if creating mTLS StorageClass and environment is not configured
	if strings.Contains(templateFile, "mtls") {
		if err := skipIfMTLSEnvironmentNotConfigured(); err != nil {
			return err
		}
	}

	fileString, err := renderTemplate(crType, templateFile)
	if err != nil {
		return err
	}

	// parse resource name out of the spec
	type NamedResource struct {
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
	}
	var res NamedResource

	err = yaml.Unmarshal([]byte(fileString), &res)
	if err != nil {
		return fmt.Errorf("error unmarshalling template file %s: %v", templateFile, err)
	}
	name := res.Metadata.Name

	// if resource exists - delete it
	if storageClassExists(name) {
		err := execCommand("kubectl", "delete", "sc", name)
		if err != nil && !isStorageClassDeleteNotFound(err) {
			return fmt.Errorf("failed to delete storage class: %v", err)
		}
	}

	filePath, err := writeRenderedFile(templateFile, fileString)
	if err != nil {
		return err
	}

	// create new storage class
	err = execCommand("kubectl", "create", "-f", filePath)
	if err != nil {
		return fmt.Errorf("failed to create storage class with template file %s: %v", templateFile, err)
	}
	return nil
}

func (step *Step) createResourceInNamespaceWithType(_ Resource, templateFile, namespace, crType string) error {
	// Expand environment variables in the namespace parameter (e.g., ${E2E_NS_AUTH})
	expandedNamespace := os.ExpandEnv(namespace)

	// Read the template file and expand environment variables and driver-specific substitutions
	fileString, err := renderTemplate(crType, templateFile)
	if err != nil {
		return err
	}

	filePath, err := writeRenderedFile(templateFile, fileString)
	if err != nil {
		return err
	}

	// Apply the resource to the specified namespace
	err = execCommand("kubectl", "apply", "-n", expandedNamespace, "-f", filePath)
	if err != nil {
		return fmt.Errorf("failed to apply resource spec file %s in namespace %s: %v", filePath, expandedNamespace, err)
	}
	return nil
}

func (step *Step) createResourceInNamespace(_ Resource, templateFile, namespace string) error {
	// Expand environment variables in the namespace parameter (e.g., ${E2E_NS_AUTH})
	expandedNamespace := os.ExpandEnv(namespace)

	// Read the template file and expand environment variables only
	fileString, err := renderTemplate("", templateFile)
	if err != nil {
		return err
	}

	filePath, err := writeRenderedFile(templateFile, fileString)
	if err != nil {
		return err
	}

	// Apply the resource to the specified namespace
	err = execCommand("kubectl", "apply", "-n", expandedNamespace, "-f", filePath)
	if err != nil {
		return fmt.Errorf("failed to apply resource spec file %s in namespace %s: %v", filePath, expandedNamespace, err)
	}
	return nil
}

func (step *Step) createResource(_ Resource, templateFile, crType string) error {
	fileString, err := renderTemplate(crType, templateFile)
	if err != nil {
		return err
	}

	filePath, err := writeRenderedFile(templateFile, fileString)
	if err != nil {
		return err
	}

	err = execCommand("kubectl", "apply", "-f", filePath)
	if err != nil {
		return fmt.Errorf("failed to apply resource spec file %s: %v", filePath, err)
	}
	return nil
}

func (step *Step) setUpConfigMap(res Resource, templateFile, name, namespace, crType string) error {
	fileString, err := renderTemplate(crType, templateFile)
	if err != nil {
		return err
	}

	// if resource exists - delete it
	if configMapExists(namespace, name) {
		err := execCommand("kubectl", "delete", "configmap", "-n", namespace, name)
		if err != nil {
			return fmt.Errorf("failed to delete config map: %v", err)
		}
	}

	filePath, err := writeRenderedFile(templateFile, fileString)
	if err != nil {
		return err
	}

	// create new storage class
	fileArg := "--from-file=config.yaml=" + filePath
	err = execCommand("kubectl", "create", "cm", name, "-n", namespace, fileArg)
	if err != nil {
		return fmt.Errorf("failed to create storage class with template file %s: %v", templateFile, err)
	}
	return nil
}

func (step *Step) setUpSecret(_ Resource, templateFile, name, namespace, crType string) error {
	// Skip if creating mTLS Secret and environment is not configured
	if strings.Contains(templateFile, "mtls") {
		if err := skipIfMTLSEnvironmentNotConfigured(); err != nil {
			return err
		}
	}

	fileString, err := renderTemplate(crType, templateFile)
	if err != nil {
		return err
	}

	// if secret exists - delete it
	if secretExists(namespace, name) {
		err := execCommand("kubectl", "delete", "secret", "-n", namespace, name)
		if err != nil {
			return fmt.Errorf("failed to delete secret: %s", err.Error())
		}
	}

	// create new secret
	fileArg := "--from-literal=config=" + fileString
	err = execCommand("kubectl", "create", "secret", "generic", "-n", namespace, name, fileArg)
	if err != nil {
		return fmt.Errorf("failed to create secret with template file %s: %v", templateFile, err)
	}

	return nil
}

func (step *Step) setUpSecretFromFile(resource Resource, templateFile, name, namespace, crType string) error {
	return step.setUpSecretFromTemplateWithFieldName(resource, templateFile, "", name, namespace, crType)
}

func (step *Step) setUpSecretFromTemplateWithFieldName(_ Resource, templateFile, fieldName, name, namespace, crType string) error {
	fileString, err := renderTemplate(crType, templateFile)
	if err != nil {
		return err
	}

	// if secret exists - delete it
	if secretExists(namespace, name) {
		err := execCommand("kubectl", "delete", "secret", "-n", namespace, name)
		if err != nil {
			return fmt.Errorf("failed to delete secret: %s", err.Error())
		}
	}

	filePath, err := writeRenderedFile(templateFile, fileString)
	if err != nil {
		return err
	}

	// create new secret
	var fileArg string
	if len(fieldName) > 0 {
		fileArg = "--from-file=" + fieldName + "=" + filePath
	} else {
		fileArg = "--from-file=" + filePath
	}

	err = execCommand("kubectl", "create", "secret", "generic", "-n", namespace, name, fileArg)
	if err != nil {
		return fmt.Errorf("failed to create secret from file %s: %v", templateFile, err)
	}

	return nil
}

func (step *Step) generateAndCreateSftpSecrets(_ Resource, privateKeyPath, privateSecretName, publicSecretName, namespace, crType string) error {
	tmpDir := filepath.Join("temp", "sftp", fmt.Sprintf("%d", time.Now().UnixNano()))
	defer os.RemoveAll(tmpDir)

	// Load env vars
	repoAddress, repoUser := os.Getenv("POWERFLEX_SFTP_REPO_ADDRESS"), os.Getenv("POWERFLEX_SFTP_REPO_USER")
	if repoAddress == "" || repoUser == "" {
		return fmt.Errorf("POWERFLEX_SFTP_REPO_ADDRESS and POWERFLEX_SFTP_REPO_USER must be set")
	}
	repoHost := strings.TrimPrefix(repoAddress, "sftp://")
	repoHost = strings.TrimSuffix(repoHost, "/")

	// Prepare temp directories
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Copy private key
	privateKeyFile := filepath.Join(tmpDir, "id_rsa")
	privateKeyData, err := os.ReadFile(filepath.Clean(privateKeyPath))
	if err != nil {
		return fmt.Errorf("failed to read private key: %v", err)
	}

	var hostPubKey string
	// Check if the private key is a dummy/placeholder
	privateKeyStr := string(privateKeyData)
	if strings.Contains(privateKeyStr, "DUMMY_VALUE") || strings.Contains(privateKeyStr, "dummy") {
		// Skip SFTP connection for dummy keys, just create secrets with the dummy key
		fmt.Println("Warning: Using dummy SFTP private key, skipping SFTP connection to populate known_hosts")
		hostPubKey = "# Dummy public key - replace with actual server key"
	} else {
		if err := os.WriteFile(privateKeyFile, privateKeyData, 0o600); err != nil { //gosec:disable G703 -- this is a test automation tool
			return fmt.Errorf("failed to write private key to temp dir: %v", err)
		}

		// Run SFTP session to populate known_hosts
		knownHostsPath := filepath.Join(sshDir, "known_hosts")
		cmd := exec.Command("sftp",
			"-o", "UserKnownHostsFile="+knownHostsPath,
			"-o", "StrictHostKeyChecking=accept-new",
			"-i", privateKeyFile,
			fmt.Sprintf("%s@%s", repoUser, repoHost),
		) // #nosec G204, G702 -- this is a test automation tool
		cmd.Stdin = strings.NewReader("exit\n")
		cmd.Env = append(os.Environ(), "HOME="+tmpDir)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("sftp session failed: %v", err)
		}

		// Extract repo public key from known_hosts
		pubKeyBytes, err := os.ReadFile(filepath.Clean(knownHostsPath))
		if err != nil {
			return fmt.Errorf("failed to read known_hosts: %v", err)
		}
		hostPubKey, err = extractHostPublicKey(string(pubKeyBytes), repoHost)
		if err != nil {
			return err
		}
	}

	// Write key files to disk for secret creation
	privateOut := filepath.Join(tmpDir, "sftp-secret-private.crt")
	publicOut := filepath.Join(tmpDir, "sftp-secret-public.crt")
	if err := os.WriteFile(privateOut, privateKeyData, 0o600); err != nil { //gosec:disable G703 -- this is a test automation tool
		return fmt.Errorf("failed to write private secret file: %v", err)
	}
	if err := os.WriteFile(publicOut, []byte(hostPubKey), 0o600); err != nil { //gosec:disable G703 -- this is a test automation tool
		return fmt.Errorf("failed to write public secret file: %v", err)
	}

	// Delete and recreate secrets
	if err := deleteSecretIfExists(namespace, privateSecretName); err != nil {
		return fmt.Errorf("failed to delete private secret: %w", err)
	}
	if err := deleteSecretIfExists(namespace, publicSecretName); err != nil {
		return fmt.Errorf("failed to delete public secret: %w", err)
	}
	if err := execCommand("kubectl", "create", "secret", "generic", privateSecretName,
		"-n", namespace,
		"--from-file=user_private_rsa_key="+privateOut); err != nil {
		return fmt.Errorf("failed to create private SFTP secret: %v", err)
	}

	if err := execCommand("kubectl", "create", "secret", "generic", publicSecretName,
		"-n", namespace,
		"--from-file=repo_public_rsa_key="+publicOut); err != nil {
		return fmt.Errorf("failed to create public SFTP secret: %v", err)
	}
	fmt.Println("SFTP secrets created successfully.")

	return nil
}

// Extract public key line for host from known_hosts
func extractHostPublicKey(knownHostsContent, repoHost string) (string, error) {
	for _, line := range strings.Split(knownHostsContent, "\n") {
		if strings.HasPrefix(line, repoHost+" ") {
			return line, nil
		}
	}
	return "", fmt.Errorf("could not extract %s public key from known_hosts", repoHost)
}

// Delete secret if exists in a namespace
func deleteSecretIfExists(namespace, secretName string) error {
	if secretExists(namespace, secretName) {
		return execCommand("kubectl", "delete", "secret", "-n", namespace, secretName)
	}
	return nil
}

func renderTemplate(crType string, templateFile string) (string, error) {
	// Check if an InSpec step has already modified this file and written
	// a temp copy. If so, read from the temp file to preserve those
	// modifications (e.g., module enables, driver image changes).
	// We use struct-level substitution (unmarshal → substitute → marshal)
	// instead of raw text substitution to preserve YAML string quoting
	// for numeric values like "000297900536".
	tempFilePath := getRenderedFilePath(templateFile)
	if tempFilePath != templateFile {
		if tempData, err := os.ReadFile(tempFilePath); err == nil { // #nosec G304
			return renderFromInSpecTemp(crType, tempData)
		}
	}

	// Standard path: read original template, use raw text substitution.
	fileContent, err := os.ReadFile(templateFile) // #nosec G304
	if err != nil {
		return "", fmt.Errorf("error reading template file: %v", err)
	}

	// Convert the file content to a string
	fileString := os.ExpandEnv(string(fileContent))

	if crType == "" {
		return fileString, nil
	}

	// find which map to use for secret values
	mapValues, err := determineMap(crType)
	if err != nil {
		return "", err
	}

	// Replace all fields in temporary (in memory) string
	for key, val := range mapValues {
		envVal := os.Getenv(val)
		if envVal == "" && strings.Contains(fileString, key) {
			return "", fmt.Errorf(
				"env var %s is empty but template %s contains placeholder %s; "+
					"check array-info.yaml and ensure the section containing %s is filled in",
				val, templateFile, key, val)
		}
		fileString = strings.ReplaceAll(fileString, key, envVal)
	}
	return fileString, nil
}

// renderFromInSpecTemp renders a template from a temp file previously written
// by an InSpec step. It uses struct-level substitution (unmarshal, substitute,
// re-marshal) instead of raw text substitution. This preserves YAML string
// quoting for numeric values like "000297900536" which would otherwise be
// interpreted as integers by Kubernetes.
func renderFromInSpecTemp(crType string, tempData []byte) (string, error) {
	// Expand env vars (e.g., ${E2E_NS_POWERMAX}) before unmarshalling.
	expanded := os.ExpandEnv(string(tempData))

	if crType == "" {
		return expanded, nil
	}

	mapValues, err := determineMap(crType)
	if err != nil {
		return "", err
	}

	// Try to unmarshal as a CSM CR for type-safe struct-level substitution.
	cr := csmv1.ContainerStorageModule{}
	if err := yaml.Unmarshal([]byte(expanded), &cr); err != nil || cr.Kind != "ContainerStorageModule" {
		// Not a CSM CR (e.g., a Secret). Fall back to text substitution.
		result := expanded
		for key, val := range mapValues {
			envVal := os.Getenv(val)
			result = strings.ReplaceAll(result, key, envVal)
		}
		return result, nil
	}

	// Apply REPLACE_* substitutions to env var values in the struct.
	applyEnvSubstitutions(&cr, mapValues)

	out, err := yaml.Marshal(cr)
	if err != nil {
		return "", fmt.Errorf("failed to marshal CR after substitution: %v", err)
	}
	return string(out), nil
}

// applyEnvSubstitutions replaces REPLACE_* placeholder strings in all
// env var values throughout the CSM CR struct (common, controller, node,
// sidecars, init containers, and module components).
func applyEnvSubstitutions(cr *csmv1.ContainerStorageModule, mapValues map[string]string) {
	substituteEnvSlice := func(envs []corev1.EnvVar) {
		for i, env := range envs {
			for key, val := range mapValues {
				envVal := os.Getenv(val)
				if strings.Contains(env.Value, key) {
					envs[i].Value = strings.ReplaceAll(env.Value, key, envVal)
				}
			}
		}
	}

	// Driver containers
	if cr.Spec.Driver.Common != nil {
		substituteEnvSlice(cr.Spec.Driver.Common.Envs)
	}
	if cr.Spec.Driver.Controller != nil {
		substituteEnvSlice(cr.Spec.Driver.Controller.Envs)
	}
	if cr.Spec.Driver.Node != nil {
		substituteEnvSlice(cr.Spec.Driver.Node.Envs)
	}
	for i := range cr.Spec.Driver.SideCars {
		substituteEnvSlice(cr.Spec.Driver.SideCars[i].Envs)
	}
	for i := range cr.Spec.Driver.InitContainers {
		substituteEnvSlice(cr.Spec.Driver.InitContainers[i].Envs)
	}

	// Module components and init containers
	for i := range cr.Spec.Modules {
		for j := range cr.Spec.Modules[i].Components {
			substituteEnvSlice(cr.Spec.Modules[i].Components[j].Envs)
		}
		for j := range cr.Spec.Modules[i].InitContainer {
			substituteEnvSlice(cr.Spec.Modules[i].InitContainer[j].Envs)
		}
	}
}

func determineMap(crType string) (map[string]string, error) {
	mapValues := map[string]string{}
	if crType == "powerflex" {
		mapValues = powerflexSecretMap
	} else if crType == "powerflexAuth" {
		mapValues = powerflexAuthSecretMap
	} else if crType == "powerflexEphemeral" {
		mapValues = powerflexEphemeralVolumeMap
	} else if crType == "powerscale" {
		mapValues = powerscaleSecretMap
	} else if crType == "powerscaleEphemeral" {
		mapValues = powerscaleEphemeralVolumeMap
	} else if crType == "powerscaleAuth" {
		mapValues = powerscaleAuthSecretMap
	} else if crType == "powerscaleAuthSidecar" {
		mapValues = powerscaleAuthSidecarMap
	} else if crType == "powerscaleDirectoryBacked" {
		mapValues = powerscaleDirectoryBackedScMap
	} else if crType == "powerflexAuthSidecar" {
		mapValues = powerflexAuthSidecarMap
	} else if crType == "powermax" {
		mapValues = powermaxStorageMap
	} else if crType == "powermaxAuthSidecar" {
		mapValues = powermaxAuthSidecarMap
	} else if crType == "powermaxCreds" {
		mapValues = powermaxCredMap
	} else if crType == "powermaxUseSecret" {
		mapValues = powermaxSecretMap
	} else if crType == "powermaxReverseProxy" {
		mapValues = powermaxReverseProxyMap
	} else if crType == "powermaxArrayConfig" {
		mapValues = powermaxArrayConfigMap
	} else if crType == "powermaxAuthArrayConfig" {
		mapValues = powermaxAuthArrayConfigMap
	} else if crType == "authSidecarCert" {
		mapValues = authSidecarRootCertMap
	} else if crType == "powerflexAuthCRs" {
		mapValues = powerflexCrMap
	} else if crType == "powerscaleAuthCRs" {
		mapValues = powerscaleCrMap
	} else if crType == "powermaxAuthCRs" {
		mapValues = powermaxCrMap
	} else if crType == "powerstoreAuthCRs" {
		mapValues = powerstoreCrMap
	} else if crType == "powerstore" {
		mapValues = powerstoreSecretMap
	} else if crType == "powerstoreEphemeral" {
		mapValues = powerstoreEphemeralVolumeMap
	} else if crType == "powerstoreAuthSidecar" {
		mapValues = powerstoreAuthSidecarMap
	} else if crType == "powerstoreAuth" {
		mapValues = powerstoreAuthSecretMap
	} else if crType == "powerstoreMetro" {
		mapValues = powerstoreMetroSecretMap
	} else if crType == "powerstoreMetroStorage" {
		mapValues = powerstoreMetroStorageMap
	} else if crType == "unity" {
		mapValues = unitySecretMap
	} else if crType == "unityEphemeral" {
		mapValues = unityEphemeralVolumeMap
	} else if crType == "cosi" {
		mapValues = cosiSecretMap
	} else if crType == "''" {
		return mapValues, nil
	} else {
		return mapValues, fmt.Errorf("type: %s is not supported", crType)
	}

	return mapValues, nil
}

func secretExists(namespace, name string) bool {
	err := exec.Command("kubectl", "get", "secret", "-n", namespace, name).Run() // #nosec G204
	return err == nil
}

func configMapExists(namespace, name string) bool {
	err := exec.Command("kubectl", "get", "configmap", "-n", namespace, name).Run() // #nosec G204
	return err == nil
}

func storageClassExists(name string) bool {
	err := exec.Command("kubectl", "get", "storageclass", name).Run() // #nosec G204
	return err == nil
}

func normalizePowerStoreEndpointHost(value string) string {
	host := strings.TrimSpace(value)
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	addr, err := netip.ParseAddr(strings.ReplaceAll(host, "%25", "%"))
	if err != nil || !addr.Is6() {
		return host
	}
	return "[" + strings.ReplaceAll(addr.String(), "%", "%25") + "]"
}

func replaceInFile(old, new, templateFile string) error { // TODO delete
	cmdString := "s|" + old + "|" + new + "|g"
	cmd := exec.Command("sed", "-i", cmdString, templateFile) // #nosec G204, G702 -- this is a test automation tool
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to substitute %s with %s in file %s: %s", old, new, templateFile, err.Error())
	}
	return nil
}

func (step *Step) runCustomTest(res Resource) error {
	var (
		stdout string
		stderr string
		err    error
	)
	if len(res.Scenario.CustomTest) != 1 {
		return fmt.Errorf("'customTest' must be a single element array")
	}

	for testNum, customTest := range res.Scenario.CustomTest[0].Run {
		args := strings.Split(os.ExpandEnv(customTest), " ")
		if len(args) == 1 {
			stdout, stderr, err = framework.RunCmd(args[0])
		} else {
			stdout, stderr, err = framework.RunCmd(args[0], args[1:]...)
		}

		if err != nil {
			return fmt.Errorf("error running custom test #%d. Error: %v \n stdout: %s \n stderr: %s", testNum, err, stdout, stderr)
		}
	}

	return nil
}

func (step *Step) runCustomTestSelector(res Resource, testName string) error {
	var (
		stdout string
		stderr string
		err    error
	)

	// retrieve the appropriate test from the list of tests
	var selectedTest CustomTest
	foundTest := false
	for _, test := range res.Scenario.CustomTest {
		if test.Name == testName {
			selectedTest = test
			foundTest = true
			break
		}
	}

	if !foundTest {
		return fmt.Errorf("custom test '%s' not found", testName)
	}

	for testNum, customTest := range selectedTest.Run {
		args := strings.Split(os.ExpandEnv(customTest), " ")
		if len(args) == 1 {
			stdout, stderr, err = framework.RunCmd(args[0])
		} else {
			stdout, stderr, err = framework.RunCmd(args[0], args[1:]...)
		}

		if err != nil {
			return fmt.Errorf("error running custom test #%d. Error: %v \n stdout: %s \n stderr: %s", testNum, err, stdout, stderr)
		}
	}

	return nil
}

func (step *Step) setupEphemeralVolumeProperties(_ Resource, templateFile string, crType string) error {
	if crType == "powerflexEphemeral" {
		_ = os.Setenv("POWERFLEX_VOLUME", fmt.Sprintf("k8s-%s", randomAlphaNumberic(10)))
	}

	fileString, err := renderTemplate(crType, templateFile)
	if err != nil {
		return err
	}

	_, err = writeRenderedFile(templateFile, fileString)
	if err != nil {
		return err
	}

	return nil
}

func randomAlphaNumberic(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyz0123456789"

	var result []byte
	for i := 0; i < length; i++ {
		randomIndex := rand.Intn(len(charset)) // #nosec G404
		result = append(result, charset[randomIndex])
	}

	return string(result)
}

func getRenderedFilePath(templatePath string) string {
	// If already a temp path, return as-is (idempotent for chained step functions)
	if strings.HasPrefix(templatePath, "temp/") {
		return templatePath
	}
	if strings.HasPrefix(templatePath, "testfiles/") {
		return "temp/" + strings.TrimPrefix(templatePath, "testfiles/")
	}
	// For paths outside testfiles (e.g., samples), use temp/ with the base filename
	return filepath.Join("temp", "samples", filepath.Base(templatePath))
}

// To not contaminate the source tree with rendered template files,
// we write all rendered files under the same temp directory, but
// preserve the subdirectories structure. For example, for templatePath
// "testfiles/powerscale-templates/ephemeral.properties" the rendered file
// will be written to "temp/powerscale-templates/ephemeral.properties".
func writeRenderedFile(templatePath, content string) (newPath string, err error) {
	// Preserve trailing YAML documents from the original source file.
	// InSpec functions that roundtrip through yaml.Unmarshal/yaml.Marshal
	// lose any documents after the first one (e.g., a ConfigMap appended
	// after a "---" separator). String-manipulation callers already
	// include them, so we only append when the new content is missing them.
	if !strings.Contains(content, "\n---\n") {
		if trailing := trailingYAMLDocs(templatePath); trailing != "" {
			content = strings.TrimRight(content, "\n") + trailing
		}
	}

	newPath = getRenderedFilePath(templatePath)

	// make sure the base path exist
	err = os.MkdirAll(filepath.Dir(newPath), 0o700)
	if err != nil {
		return "", fmt.Errorf("error creating temp directory %s: %v", filepath.Dir(newPath), err)
	}

	err = os.WriteFile(newPath, []byte(content), 0o644) // #nosec G306 -- this is a test automation tool
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %v", err)
	}

	return newPath, nil
}

// trailingYAMLDocs returns any YAML documents after the first one in the
// original (non-temp) source file for the given path. This is used to
// preserve multi-document YAML files (e.g., a CSM CR followed by a
// ConfigMap separated by "---") when InSpec functions roundtrip the first
// document through yaml.Unmarshal/yaml.Marshal.
func trailingYAMLDocs(templatePath string) string {
	// Resolve to the original (non-temp) source file.
	origPath := templatePath
	if strings.HasPrefix(templatePath, "temp/") {
		origPath = "testfiles/" + strings.TrimPrefix(templatePath, "temp/")
	}

	data, err := os.ReadFile(origPath) // #nosec G304
	if err != nil {
		return ""
	}

	idx := bytes.Index(data, []byte("\n---\n"))
	if idx < 0 {
		return ""
	}
	return string(data[idx:]) // includes the "\n---\n" prefix
}

// readCRFileForInSpec reads a CR file for an InSpec function, preferring
// any previously rendered temp file so that chained InSpec steps accumulate
// their modifications instead of overwriting each other.
func readCRFileForInSpec(crFilePath string) ([]byte, error) {
	tempFilePath := getRenderedFilePath(crFilePath)
	if tempFilePath != crFilePath {
		if data, err := os.ReadFile(tempFilePath); err == nil { // #nosec G304
			return data, nil
		}
	}
	return os.ReadFile(crFilePath) // #nosec G304
}

func (step *Step) enableModule(res Resource, module string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	for i, m := range found.Spec.Modules {
		if !m.Enabled && m.Name == csmv1.ModuleType(module) {
			found.Spec.Modules[i].Enabled = true
			// for observability, enable all components
			if m.Name == csmv1.Observability {
				for j := range m.Components {
					found.Spec.Modules[i].Components[j].Enabled = pointer.Bool(true)
				}
			}
		}
	}

	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) setDriverSecret(res Resource, crNumStr string, driverSecretName string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}
	found.Spec.Driver.AuthSecret = driverSecretName
	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) disableModule(res Resource, module string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	for i, m := range found.Spec.Modules {
		if m.Enabled && m.Name == csmv1.ModuleType(module) {
			found.Spec.Modules[i].Enabled = false

			if m.Name == csmv1.Observability {
				for j := range m.Components {
					found.Spec.Modules[i].Components[j].Enabled = pointer.Bool(false)
				}
			}
		}
	}

	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) configureHealthMonitor(res Resource, action string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	// Determine the enabled state based on action
	enabled := strings.ToLower(action) == "enable"
	fmt.Println("Setting health monitor to: ", enabled)

	// Configure external-health-monitor sidecar
	for i, sideCar := range found.Spec.Driver.SideCars {
		if strings.Contains(sideCar.Name, "external-health-monitor") {
			found.Spec.Driver.SideCars[i].Enabled = pointer.Bool(enabled)
			break
		}
	}

	// Set X_CSI_HEALTH_MONITOR_ENABLED for both controller and node
	healthMonitorValue := "false"
	if enabled {
		healthMonitorValue = "true"
	}

	if found.Spec.Driver.Controller != nil {
		for i, env := range found.Spec.Driver.Controller.Envs {
			if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
				found.Spec.Driver.Controller.Envs[i].Value = healthMonitorValue
				break
			}
		}
	}

	if found.Spec.Driver.Node != nil {
		for i, env := range found.Spec.Driver.Node.Envs {
			if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
				found.Spec.Driver.Node.Envs[i].Value = healthMonitorValue
				break
			}
		}
	}

	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) enableForceRemoveDriver(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	truebool := true
	found.Spec.Driver.ForceRemoveDriver = &truebool
	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) validateForceRemoveDriverEnabled(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	if found.Spec.Driver.ForceRemoveDriver != nil && *found.Spec.Driver.ForceRemoveDriver {
		return nil
	}
	return fmt.Errorf("forceRemoveDriver is not set to true")
}

func (step *Step) validateForceRemoveDriverDisabled(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	// forceRemoveDriver is disabled if it's nil (removed) or explicitly set to false
	if found.Spec.Driver.ForceRemoveDriver == nil || (found.Spec.Driver.ForceRemoveDriver != nil && !*found.Spec.Driver.ForceRemoveDriver) {
		return nil
	}
	return fmt.Errorf("forceRemoveDriver is not set to false")
}

func (step *Step) enableForceRemoveModule(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}
	for i := range found.Spec.Modules {
		found.Spec.Modules[i].ForceRemoveModule = true
	}
	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) validateTestEnvironment(_ Resource) error {
	if os.Getenv("OPERATOR_NAMESPACE") != "" {
		operatorNamespace = os.Getenv("OPERATOR_NAMESPACE")
	}

	pods, err := fpod.GetPodsInNamespace(context.TODO(), step.clientSet, operatorNamespace, map[string]string{})
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return fmt.Errorf("operator is not installed in namespace [%s]", operatorNamespace)
	}

	notReadyMessage := ""
	allReady := true
	for _, pod := range pods {
		if pod.Status.Phase != corev1.PodRunning {
			allReady = false
			notReadyMessage += fmt.Sprintf("\nThe pod(%s) is %s", pod.Name, pod.Status.Phase)
		}
	}

	if !allReady {
		return fmt.Errorf("Bad Operator state:%s", notReadyMessage)
	}

	return nil
}

func (step *Step) validateKubernetesEnvironment(_ Resource) error {
	// Validate we can connect to the cluster
	_, err := step.clientSet.ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to connect to Kubernetes cluster: %v", err)
	}

	fmt.Println("Successfully connected to Kubernetes cluster")

	// Check that CSM operator is not installed using the same approach as validateTestEnvironment
	operatorNamespace := os.Getenv("OPERATOR_NAMESPACE")
	if operatorNamespace == "" {
		operatorNamespace = "dell-csm-operator"
	}

	pods, err := fpod.GetPodsInNamespace(context.TODO(), step.clientSet, operatorNamespace, map[string]string{})
	if err != nil {
		return fmt.Errorf("failed to check pods in namespace [%s]: %v", operatorNamespace, err)
	}

	if len(pods) > 0 {
		return fmt.Errorf("CSM operator is installed in namespace [%s] - found %d pods", operatorNamespace, len(pods))
	}

	fmt.Printf("Verified CSM operator is not installed in namespace [%s]\n", operatorNamespace)
	return nil
}

func (step *Step) createPrereqs(res Resource, module string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.ModuleType(module) {
			switch m.Name {
			case csmv1.AuthorizationServer:
				return step.authProxyServerPrereqs(cr)

			default:
				return fmt.Errorf("%s module is not found", module)
			}
		}
	}

	return nil
}

func (step *Step) validateAuthorizationProxyServerInstalled(cr csmv1.ContainerStorageModule) error {
	instance := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, instance,
	); err != nil {
		return err
	}

	// check installation for all AuthorizationProxyServer
	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	clusterClient := operatorutils.GetCluster(context.TODO(), &fakeReconcile)

	// check AuthorizationProxyServer in all clusters
	if err := checkAuthorizationProxyServerPods(context.TODO(), cr.Namespace, clusterClient.ClusterK8sClient); err != nil {
		return fmt.Errorf("failed to check for AuthorizationProxyServer installation in %s: %v", clusterClient.ClusterID, err)
	}

	return nil
}

func (step *Step) validateAuthorizationProxyServerNotInstalled(cr csmv1.ContainerStorageModule) error {
	// check installation for all AuthorizationProxyServer
	fakeReconcile := operatorutils.FakeReconcileCSM{
		Client:    step.ctrlClient,
		K8sClient: step.clientSet,
	}

	clusterClient := operatorutils.GetCluster(context.TODO(), &fakeReconcile)

	// check AuthorizationProxyServer is not installed
	if err := checkAuthorizationProxyServerNoRunningPods(context.TODO(), cr.Namespace, clusterClient.ClusterK8sClient); err != nil {
		return fmt.Errorf("failed AuthorizationProxyServer installation check %s: %v", clusterClient.ClusterID, err)
	}

	return nil
}

// validatePreUpgradeSnapshot validates that the PreUpgradeSnapshot annotation exists on the CR
func (step *Step) validatePreUpgradeSnapshot(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	annotations := found.GetAnnotations()
	if annotations == nil {
		return fmt.Errorf("CR %s has no annotations", cr.Name)
	}

	if _, exists := annotations["storage.dell.com/PreUpgradeSnapshot"]; !exists {
		return fmt.Errorf("CR %s does not have PreUpgradeSnapshot annotation", cr.Name)
	}

	return nil
}

// validateSnapshotDeleted validates that the PreUpgradeSnapshot annotation is deleted
func (step *Step) validateSnapshotDeleted(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	annotations := found.GetAnnotations()
	if annotations != nil {
		if _, exists := annotations["storage.dell.com/PreUpgradeSnapshot"]; exists {
			return fmt.Errorf("CR %s still has PreUpgradeSnapshot annotation after successful upgrade", cr.Name)
		}
	}

	return nil
}

// validateRollbackCompleted validates that a rollback was completed successfully.
// It uses the in-memory CR spec version as the expected rollback target.
// For minimal tests where InSpec steps mutate the CR version, use
// validateRollbackCompletedWithVersion instead.
func (step *Step) validateRollbackCompleted(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	return step.doValidateRollbackCompleted(cr, cr.Spec.Version)
}

// validateRollbackCompletedWithVersion validates rollback with an explicit expected
// version. Use this in minimal tests where the in-memory CR spec is mutated by
// InSpec steps and no longer reflects the pre-upgrade version.
// The expectedVer parameter supports "n-1" / "n-2" keywords which are resolved
// dynamically, or a literal version string like "v1.17.0".
func (step *Step) validateRollbackCompletedWithVersion(res Resource, crNumStr, expectedVer string) error {
	resolved, err := resolveNMinusCSMVersion(expectedVer)
	if err != nil {
		return err
	}
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	return step.doValidateRollbackCompleted(cr, resolved)
}

// doValidateRollbackCompleted is the shared implementation for rollback validation.
func (step *Step) doValidateRollbackCompleted(cr csmv1.ContainerStorageModule, expectedVersion string) error {
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	// Check that CR state is Succeeded after rollback
	if found.Status.State != constants.Succeeded {
		return fmt.Errorf("CR %s state is %s after rollback, expected Succeeded", cr.Name, found.Status.State)
	}

	// Check that snapshot was deleted after rollback
	annotations := found.GetAnnotations()
	if annotations != nil {
		if _, exists := annotations["storage.dell.com/PreUpgradeSnapshot"]; exists {
			return fmt.Errorf("CR %s still has PreUpgradeSnapshot annotation after rollback", cr.Name)
		}
	}

	// Check that the spec version was reverted to the expected pre-upgrade version
	if expectedVersion == "" {
		return fmt.Errorf("CR %s has no expected version to validate rollback against", cr.Name)
	}
	if found.Spec.Version != expectedVersion {
		return fmt.Errorf("CR %s spec.version is %s after rollback, expected %s", cr.Name, found.Spec.Version, expectedVersion)
	}

	return nil
}

func (step *Step) authProxyServerPrereqs(cr csmv1.ContainerStorageModule) error {
	fmt.Println("=== Creating Authorization Proxy Server Prerequisites ===")

	// Ensure secrets-store CSI driver is installed (vault pods depend on it)
	if err := ensureSecretsStoreCSIDriver(); err != nil {
		return fmt.Errorf("failed to ensure secrets-store CSI driver: %v", err)
	}

	// Ensure vault is running and ready
	if err := ensureVaultReady(); err != nil {
		return fmt.Errorf("failed to ensure vault is ready: %v", err)
	}

	cmd := exec.Command("kubectl", "get", "ns", cr.Namespace) // #nosec G204
	err := cmd.Run()
	if err == nil {

		fmt.Printf("\nDeleting all CSM from namespace: %s \n", cr.Namespace)
		cmd = exec.Command("kubectl", "delete", "csm", "-n", cr.Namespace, "--all", "--ignore-not-found") // #nosec G204
		b, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to delete all CSM from namespace: %v\nErrMessage:\n%s", err, string(b))
		}

		// Remove finalizers from authorization CRDs that would block namespace deletion
		removeAuthorizationFinalizers(cr.Namespace)

		cmd = exec.Command("kubectl", "delete", "ns", cr.Namespace, "--wait=true", "--timeout=60s", "--ignore-not-found") // #nosec G204
		b, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to delete authorization namespace: %v\nErrMessage:\n%s", err, string(b))
		}
	}

	cmd = exec.Command("kubectl", "create",
		"ns", cr.Namespace,
	) // #nosec G204
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create authorization namespace: %v\nErrMessage:\n%s", err, string(b))
	}

	isOpenShift := os.Getenv("IS_OPENSHIFT")
	if isOpenShift == "true" {
		cmd = exec.Command("oc", "label",
			"ns", cr.Namespace,
			"pod-security.kubernetes.io/enforce=privileged",
			"security.openshift.io/MinimallySufficientPodSecurityStandard=privileged",
			"--overwrite",
		) // #nosec G204
		b, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to label authorization namespace: %v\nErrMessage:\n%s", err, string(b))
		}
	}

	cmd = exec.Command("kubectl", "apply",
		"--validate=false", "-f",
		fmt.Sprintf("https://github.com/jetstack/cert-manager/releases/download/%s/cert-manager.crds.yaml",
			certManagerVersion),
	) // #nosec G204
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply cert-manager CRDs: %v\nErrMessage:\n%s", err, string(b))
	}

	cmd = exec.Command("kubectl", "create",
		"secret", "generic",
		"karavi-config-secret",
		"-n", cr.Namespace,
		"--from-file=config.yaml=testfiles/authorization-templates/storage_csm_authorization_config.yaml",
	) // #nosec G204
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create config secret for JWT: %v\nErrMessage:\n%s", err, string(b))
	}

	cmd = exec.Command("kubectl", "create", "-n", cr.Namespace,
		"-f", "testfiles/authorization-templates/storage_csm_authorization_storage_secret.yaml",
	) // #nosec G204
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create storage secret: %v\nErrMessage:\n%s", err, string(b))
	}
	return nil
}

// removeAuthorizationFinalizers strips finalizers from authorization CRDs in the
// given namespace so that namespace deletion does not hang indefinitely.
func removeAuthorizationFinalizers(namespace string) {
	crdTypes := []string{
		"storage.csm-authorization.storage.dell.com",
		"csmrole.csm-authorization.storage.dell.com",
		"csmtenant.csm-authorization.storage.dell.com",
	}
	for _, crd := range crdTypes {
		out, err := exec.Command("kubectl", "get", crd, "-n", namespace, "-o", "name").CombinedOutput() // #nosec G204,G702
		if err != nil {
			continue
		}
		items := strings.TrimSpace(string(out))
		if items == "" {
			continue
		}
		for _, item := range strings.Split(items, "\n") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			fmt.Printf("  Removing finalizers from %s in %s\n", item, namespace)
			_ = exec.Command("kubectl", "patch", item, "-n", namespace, // #nosec G204,G702
				"--type", "merge", "-p", `{"metadata":{"finalizers":null}}`).Run()
			_ = exec.Command("kubectl", "delete", item, "-n", namespace, "--wait=false", "--ignore-not-found").Run() // #nosec G204,G702
		}
	}
}

// ensureSecretsStoreCSIDriver checks that the secrets-store CSI driver is installed
// and installs it via helm if missing. Authorization tests depend on this driver
// for vault secret synchronization. We check the CSIDriver registration (not just
// CRDs) because CRDs can survive a helm uninstall while the actual driver is gone.
// After ensuring the driver is installed, we verify the DaemonSet pods are Ready
// on all nodes so that kubelet can send CSI mount requests for SPC volumes.
func ensureSecretsStoreCSIDriver() error {
	cmd := exec.Command("kubectl", "get", "csidriver", "secrets-store.csi.k8s.io") // #nosec G204
	if cmd.Run() != nil {
		fmt.Println("secrets-store CSI driver not found, installing...")
		// Remove stale helm release if CRDs were deleted but release remains
		_ = exec.Command("helm", "uninstall", "csi-secrets-store", "-n", "kube-system").Run() // #nosec G204

		_ = exec.Command("helm", "repo", "add", "secrets-store-csi-driver", // #nosec G204
			"https://kubernetes-sigs.github.io/secrets-store-csi-driver/charts").Run()

		cmd = exec.Command("helm", "install", "csi-secrets-store", // #nosec G204
			"secrets-store-csi-driver/secrets-store-csi-driver",
			"--wait",
			"--timeout", "10m",
			"--namespace", "kube-system",
			"--set", "enableSecretRotation=true",
			"--set", "syncSecret.enabled=true",
			"--set", "tokenRequests[0].audience=conjur",
		)
		b, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to install secrets-store CSI driver: %v\nErrMessage:\n%s", err, string(b))
		}
		fmt.Println("secrets-store CSI driver installed successfully")
	} else {
		fmt.Println("secrets-store CSI driver is already installed")
	}

	// Always verify DaemonSet pods are Ready regardless of whether we just
	// installed or the driver was already present. A stale CSIDriver resource
	// can exist even when DaemonSet pods are not running (e.g., after a
	// previous test run's cleanup removed pods but not the CSIDriver CRD).
	fmt.Println("Waiting for secrets-store CSI driver DaemonSet pods to be ready...")
	cmd = exec.Command("kubectl", "rollout", "status", "daemonset/csi-secrets-store-secrets-store-csi-driver", // #nosec G204
		"-n", "kube-system", "--timeout=120s")
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("secrets-store CSI driver DaemonSet not ready: %v\nErrMessage:\n%s", err, string(b))
	}
	fmt.Println("secrets-store CSI driver DaemonSet is ready")
	return nil
}

// ensureVaultReady checks that the vault pod is running and ready.
func ensureVaultReady() error {
	cmd := exec.Command("kubectl", "get", "pod", "vault0-0", "-n", "default", // #nosec G204
		"-o", "jsonpath={.status.phase}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("vault0-0 pod not found in default namespace: %v", err)
	}
	phase := strings.TrimSpace(string(out))
	if phase == "Running" {
		fmt.Println("vault0-0 is running")
		return nil
	}
	return fmt.Errorf("vault0-0 is in phase %q, expected Running", phase)
}

func (step *Step) configureAuthorizationProxyServer(res Resource, authConfigurationPath, driver, crNumStr string) error {
	fmt.Println("=== Configuring Authorization Proxy Server ===")

	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	var (
		storageType     = ""
		driverNamespace = ""
		proxyHost       = ""
		csmTenantName   = ""
	)

	// if tests are running multiple scenarios that require differently configured auth servers, we will not be able to use one set of vars
	// this section is for powerflex, other drivers can add their sections as required.
	if driver == "powerflex" {
		_ = os.Setenv("POWERFLEX_STORAGE", "powerflex")
		_ = os.Setenv("DRIVER_NAMESPACE", os.Getenv("E2E_NS_POWERFLEX"))
		storageType = os.Getenv("POWERFLEX_STORAGE")
		csmTenantName = os.Getenv("POWERFLEX_TENANT")
	}

	if driver == "powerscale" {
		_ = os.Setenv("POWERSCALE_STORAGE", "powerscale")
		_ = os.Setenv("DRIVER_NAMESPACE", os.Getenv("E2E_NS_POWERSCALE"))
		storageType = os.Getenv("POWERSCALE_STORAGE")
		csmTenantName = os.Getenv("POWERSCALE_TENANT")
	}

	if driver == "powermax" {
		_ = os.Setenv("POWERMAX_STORAGE", "powermax")
		_ = os.Setenv("DRIVER_NAMESPACE", os.Getenv("E2E_NS_POWERMAX"))
		storageType = os.Getenv("POWERMAX_STORAGE")
		csmTenantName = os.Getenv("POWERMAX_TENANT")
	}

	if driver == "powerstore" {
		_ = os.Setenv("POWERSTORE_STORAGE", "powerstore")
		_ = os.Setenv("DRIVER_NAMESPACE", os.Getenv("E2E_NS_POWERSTORE"))
		storageType = os.Getenv("POWERSTORE_STORAGE")
		csmTenantName = os.Getenv("POWERSTORE_TENANT")
	}

	proxyHost = os.Getenv("AUTHORIZATION_HOST")
	if proxyHost == "" {
		proxyHost = os.Getenv("PROXY_HOST")
	}
	driverNamespace = os.Getenv("DRIVER_NAMESPACE")

	port, err := getPortContainerizedAuth(cr.Namespace)
	if err != nil {
		return err
	}

	// For gateway-nginx (v2.5.0+), use node IP with NodePort and add to /etc/hosts
	// This allows TLS certificate validation to work (certificate issued for csm-authorization.com)
	if strings.Contains(proxyHost, "gateway-nginx") {
		// Get a worker node IP for accessing NodePort from outside cluster
		cmd := exec.Command("kubectl", "get", "nodes", "-o", "jsonpath={.items[0].status.addresses[?(@.type==\"InternalIP\")].address}") // #nosec G204
		output, err := cmd.CombinedOutput()
		if err == nil && strings.TrimSpace(string(output)) != "" {
			nodeIP := strings.TrimSpace(string(output))
			// Add nodeIP -> csm-authorization.com mapping to /etc/hosts for TLS certificate validation
			hostsCmd := exec.Command("sudo", "sh", "-c", fmt.Sprintf("sed -i '/csm-authorization.com/d' /etc/hosts && echo '%s csm-authorization.com' >> /etc/hosts", nodeIP)) // #nosec G204
			hostsCmd.Run()                                                                                                                                                     // Ignore errors - might not have sudo or entry might already exist
			// Use csm-authorization.com as hostname (mapped to nodeIP in /etc/hosts) for TLS certificate validation
			proxyHost = "csm-authorization.com"
		}
	}
	// For ingress-nginx-controller (v2.4.0 n-1), also use node IP with NodePort
	// The ingress-nginx-controller service is typically a LoadBalancer or NodePort
	if strings.Contains(proxyHost, "ingress-nginx-controller") {
		// Get a worker node IP for accessing NodePort from outside cluster
		cmd := exec.Command("kubectl", "get", "nodes", "-o", "jsonpath={.items[0].status.addresses[?(@.type==\"InternalIP\")].address}") // #nosec G204
		output, err := cmd.CombinedOutput()
		if err == nil && strings.TrimSpace(string(output)) != "" {
			nodeIP := strings.TrimSpace(string(output))
			// Add nodeIP -> csm-authorization.com mapping to /etc/hosts for TLS certificate validation
			hostsCmd := exec.Command("sudo", "sh", "-c", fmt.Sprintf("sed -i '/csm-authorization.com/d' /etc/hosts && echo '%s csm-authorization.com' >> /etc/hosts", nodeIP)) // #nosec G204
			hostsCmd.Run()                                                                                                                                                     // Ignore errors - might not have sudo or entry might already exist
			// Use csm-authorization.com as hostname (mapped to nodeIP in /etc/hosts) for TLS certificate validation
			proxyHost = "csm-authorization.com"
		}
	}

	address := proxyHost
	fmt.Printf("Address: %s\n", address)

	return step.AuthorizationV2Resources(res, storageType, driver, driverNamespace, address, port, csmTenantName, cr.Spec.Modules[0].ConfigVersion, authConfigurationPath)
}

// AuthorizationV2Resources creates resources using CRs and dellctl for V2 versions of Authorization Proxy Server.
// The one-time setup (template rendering, admin token, resource creation) runs once per driver.
// Only the token generation and application steps are retried on subsequent calls.
func (step *Step) AuthorizationV2Resources(res Resource, storageType, driver, driverNamespace, proxyHost, port, csmTenantName, configVersion string, configurationTemplate string) error {
	// Re-run setup if temp files were cleaned between scenarios
	if authV2SetupDone[driver] {
		if _, err := os.Stat("temp/adminToken.yaml"); os.IsNotExist(err) {
			fmt.Printf("=== temp/adminToken.yaml missing, re-running setup for %s ===\n", driver)
			authV2SetupDone[driver] = false
		}
	}

	if !authV2SetupDone[driver] {
		if err := step.authorizationV2Setup(res, storageType, driver, configVersion, configurationTemplate, csmTenantName); err != nil {
			return err
		}
		authV2SetupDone[driver] = true
	} else {
		fmt.Printf("=== Skipping one-time setup for %s (already done) ===\n", driver)
	}

	return step.authorizationV2GenerateAndApplyToken(driver, driverNamespace, proxyHost, port, csmTenantName)
}

// authorizationV2Setup performs the one-time setup: template rendering, admin token creation, and resource application.
func (step *Step) authorizationV2Setup(res Resource, storageType, driver, configVersion string, configurationTemplate string, csmTenantName string) error {
	var (
		crMap               = ""
		templateFile        = configurationTemplate
		updatedTemplateFile = ""
	)

	if semver.Compare(configVersion, "v2.3.0") == -1 {
		templateFile = "testfiles/authorization-templates/storage_csm_authorization_v2_template_vault.yaml"
	}

	if driver == "powerflex" {
		crMap = "powerflexAuthCRs"
		updatedTemplateFile = "temp/authorization-templates/storage_csm_authorization_crs_powerflex.yaml"
	} else if driver == "powerscale" {
		crMap = "powerscaleAuthCRs"
		updatedTemplateFile = "temp/authorization-templates/storage_csm_authorization_crs_powerscale.yaml"
	} else if driver == "powermax" {
		crMap = "powermaxAuthCRs"
		updatedTemplateFile = "temp/authorization-templates/storage_csm_authorization_crs_powermax.yaml"
	} else if driver == "powerstore" {
		crMap = "powerstoreAuthCRs"
		updatedTemplateFile = "temp/authorization-templates/storage_csm_authorization_crs_powerstore.yaml"
	}

	pathNum, err := parseIndex(configurationTemplate)
	if err != nil {
		return err
	}
	err = execShell(fmt.Sprintf("mkdir -p temp/authorization-templates && cp %s %s", res.Scenario.Paths[pathNum-1], updatedTemplateFile))
	if err != nil {
		return fmt.Errorf("failed to copy template file %s to %s: %v", templateFile, updatedTemplateFile, err)
	}

	// Expand env vars (e.g. ${E2E_NS_AUTH}) in the copied template file
	raw, err := os.ReadFile(updatedTemplateFile)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %v", updatedTemplateFile, err)
	}
	if err := os.WriteFile(filepath.Clean(updatedTemplateFile), []byte(os.ExpandEnv(string(raw))), 0o644); err != nil { // #nosec G703 -- path is constructed from hardcoded template dirs
		return fmt.Errorf("failed to write expanded template %s: %v", updatedTemplateFile, err)
	}

	// Create Admin Token
	fmt.Printf("=== Generating Admin Token ===\n")
	adminCtx, adminCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer adminCancel()
	adminTkn := exec.CommandContext(adminCtx, "dellctl",
		"admin", "token",
		"--name", "Admin",
		"--jwt-signing-secret", "test-strong-secret-abcdefghijklm",
		"--refresh-token-expiration", fmt.Sprint(30*24*time.Hour),
		"--access-token-expiration", fmt.Sprint(2*time.Hour),
	) // #nosec G204
	b, err := adminTkn.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create admin token: %v\nErrMessage:\n%s", err, string(b))
	}

	fmt.Println("=== Writing Admin Token to Tmp File ===\n ")
	err = os.WriteFile("temp/adminToken.yaml", b, 0o644) // #nosec G303, G306
	if err != nil {
		return fmt.Errorf("failed to write admin token: %v\nErrMessage:\n%s", err, string(b))
	}

	// Create Resources
	fmt.Println("=== Creating Storage, Role, and Tenant ===\n ")
	mapValues, err := determineMap(crMap)
	if err != nil {
		return err
	}

	for key := range mapValues {
		val := os.Getenv(mapValues[key])
		if driver == "powerscale" && key == "REPLACE_ENDPOINT" {
			fmt.Println("Replacing PowerScale Endpoint and adding port...")

			port := os.Getenv(mapValues["REPLACE_PORT"])
			if port == "" {
				port = "8080"
			}

			val = val + ":" + port
		}

		if driver == "powerstore" && (key == "REPLACE_ENDPOINT" || key == "REPLACE_METRO_ENDPOINT") {
			fmt.Println("Replacing PowerStore Endpoint and adding /api/rest/")
			val = normalizePowerStoreEndpointHost(val) + "/api/rest"
		}

		if key == "REPLACE_USERNAME_OBJECT_NAME" {
			val = fmt.Sprintf("secrets/%s-username", driver)
		}

		if key == "REPLACE_PASSWORD_OBJECT_NAME" {
			val = fmt.Sprintf("secrets/%s-password", driver)
		}

		err := replaceInFile(key, val, updatedTemplateFile)
		if err != nil {
			return err
		}
	}
	cmd := exec.Command("kubectl", "apply",
		"-f", updatedTemplateFile,
	)
	fmt.Println("=== Storage, Role, and Tenant === \n", cmd.String())
	b, err = cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(b), "is already registered") {
		return fmt.Errorf("failed to create resources for %s: %v\nErrMessage:\n%s", storageType, err, string(b))
	}

	fmt.Println("Waiting for tenant CR to become Available before generating token.")
	if err := step.waitForTenantReady(csmTenantName); err != nil {
		return fmt.Errorf("tenant %s did not become ready: %v", csmTenantName, err)
	}

	return nil
}

// waitForTenantReady polls the CSMTenant CR status until it reaches "Available" state.
// This replaces the fixed 2-second sleep with proper readiness checking.
func (step *Step) waitForTenantReady(tenantName string) error {
	authNS := os.Getenv("E2E_NS_AUTH")
	if authNS == "" {
		authNS = "e2e-authorization"
	}

	fmt.Printf("=== Polling for tenant %s to become Available in namespace %s ===\n", tenantName, authNS)
	// Poll every 3 seconds for up to 60 seconds
	for i := 0; i < 20; i++ {
		time.Sleep(3 * time.Second)

		cmd := exec.Command("kubectl", "get", "csmtenant", tenantName, "-n", authNS, "-o", "jsonpath='{.status.conditions[?(@.type==\"Available\")].status}'") // #nosec G204, G702 -- tenantName is from controlled test environment
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("=== Warning: failed to get tenant status: %v\n", err)
			continue
		}

		status := strings.Trim(string(output), "'")
		fmt.Printf("=== Tenant Available status: %s (attempt %d/20) ===\n", status, i+1)

		if status == "True" {
			fmt.Printf("=== Tenant %s is Available ===\n", tenantName)
			return nil
		}
	}

	return fmt.Errorf("tenant %s did not reach Available status after 60 seconds", tenantName)
}

// authorizationV2GenerateAndApplyToken generates a tenant token and applies it.
// This is the retry-safe portion of AuthorizationV2Resources.
func (step *Step) authorizationV2GenerateAndApplyToken(driver, driverNamespace, proxyHost, port, csmTenantName string) error {
	// Verify proxy server is reachable before attempting token generation.
	// Without this check, dellctl can hang on an unresponsive endpoint.
	addr := net.JoinHostPort(proxyHost, port)
	fmt.Printf("=== Checking proxy server connectivity at %s ===\n", addr)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second) // #nosec G704
	if err != nil {
		return fmt.Errorf("proxy server not reachable at %s: %v", addr, err)
	}
	conn.Close()

	// Generate tenant token
	fmt.Println("=== Generating token ===\n ")
	genCtx, genCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer genCancel()
	cmd := exec.CommandContext(genCtx, "dellctl",
		"generate", "token",
		"--admin-token", "temp/adminToken.yaml",
		"--access-token-expiration", fmt.Sprint(10*time.Minute),
		"--refresh-token-expiration", "48h",
		"--tenant", csmTenantName,
		"--insecure", "--addr", addr,
	) // #nosec G204, G702 -- this is a test automation tool
	fmt.Println("=== Token ===\n", cmd.String())
	b, err := cmd.CombinedOutput()
	if err != nil {
		// If tenant is not found, delete and recreate the auth CRs so the
		// tenant-service can reconcile them from scratch. The Eventually
		// retry loop will call us again after this returns.
		if strings.Contains(string(b), "tenant not found") {
			step.recreateAuthorizationCRs(driver)
		}
		return fmt.Errorf("failed to generate token for %s: %v\nErrMessage:\n%s", csmTenantName, err, string(b))
	}

	// Apply token to CSI driver host
	fmt.Println("=== Applying token ===\n ")

	err = os.WriteFile("temp/token.yaml", b, 0o644) // #nosec G303, G306, G703 -- this is a test automation tool
	if err != nil {
		return fmt.Errorf("failed to write tenant token: %v\nErrMessage:\n%s", err, string(b))
	}

	cmd = exec.Command("kubectl", "apply",
		"-f", "temp/token.yaml",
		"-n", driverNamespace,
	) // #nosec G204, G702 -- this is a test automation tool
	b, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply token: %v\nErrMessage:\n%s", err, string(b))
	}
	fmt.Println("=== Token Applied ===\n ")

	return nil
}

// lastAuthCRRecreation tracks when we last recreated auth CRs per driver,
// to avoid spamming delete/recreate on every retry iteration.
var lastAuthCRRecreation = map[string]time.Time{}

// recreateAuthorizationCRs deletes and recreates the storage, role, and tenant
// CRs for the given driver. This is a workaround for a race condition where
// the tenant-service does not reconcile CRs created before it was fully ready,
// resulting in a persistent "tenant not found" error during token generation.
func (step *Step) recreateAuthorizationCRs(driver string) {
	// Debounce: skip if we recreated less than 30 seconds ago for this driver
	if last, ok := lastAuthCRRecreation[driver]; ok && time.Since(last) < 30*time.Second {
		fmt.Printf("=== Skipping auth CR recreation for %s (last recreated %v ago) ===\n", driver, time.Since(last).Round(time.Second))
		return
	}
	templateFile := ""
	switch driver {
	case "powerflex":
		templateFile = "temp/authorization-templates/storage_csm_authorization_crs_powerflex.yaml"
	case "powerscale":
		templateFile = "temp/authorization-templates/storage_csm_authorization_crs_powerscale.yaml"
	case "powermax":
		templateFile = "temp/authorization-templates/storage_csm_authorization_crs_powermax.yaml"
	case "powerstore":
		templateFile = "temp/authorization-templates/storage_csm_authorization_crs_powerstore.yaml"
	default:
		fmt.Printf("=== Unknown driver %q, skipping auth CR recreation ===\n", driver)
		return
	}

	if _, err := os.Stat(templateFile); os.IsNotExist(err) {
		fmt.Printf("=== Template file %s not found, skipping auth CR recreation ===\n", templateFile)
		return
	}

	fmt.Printf("=== Tenant not found: deleting and recreating auth CRs for %s ===\n", driver)

	// Strip finalizers first -- auth CRs have finalizers that block deletion.
	authNS := os.Getenv("E2E_NS_AUTH")
	if authNS == "" {
		authNS = "e2e-authorization"
	}
	removeAuthorizationFinalizers(authNS)

	// Delete existing CRs (ignore errors -- they may not exist)
	delCmd := exec.Command("kubectl", "delete", "-f", templateFile, "--ignore-not-found", "--wait=false") // #nosec G204
	if out, err := delCmd.CombinedOutput(); err != nil {
		fmt.Printf("=== Warning: delete auth CRs returned error: %v\n%s\n", err, string(out))
	}

	// Poll for CRs to be deleted instead of a fixed 15-second sleep.
	// Check every 2 seconds for up to 15 seconds.
	fmt.Println("=== Polling for auth CR deletion to propagate ===")
	deletionDone := false
	for i := 0; i < 8; i++ {
		time.Sleep(2 * time.Second)
		checkCmd := exec.Command("kubectl", "get", "-f", templateFile, "--ignore-not-found", "-o", "name") // #nosec G204
		out, _ := checkCmd.CombinedOutput()
		if len(strings.TrimSpace(string(out))) == 0 {
			fmt.Printf("=== Auth CRs deleted after %d seconds ===\n", (i+1)*2)
			deletionDone = true
			break
		}
	}
	if !deletionDone {
		fmt.Println("=== Auth CR deletion poll timed out, proceeding anyway ===")
	}

	// Recreate CRs
	createCmd := exec.Command("kubectl", "apply", "-f", templateFile) // #nosec G204
	if out, err := createCmd.CombinedOutput(); err != nil {
		fmt.Printf("=== Warning: recreate auth CRs returned error: %v\n%s\n", err, string(out))
	} else {
		fmt.Printf("=== Auth CRs recreated for %s ===\n", driver)
	}

	// Restart tenant-service to force it to re-read the new CRs
	fmt.Println("=== Restarting tenant-service to pick up new CRs ===")
	restartCmd := exec.Command("kubectl", "rollout", "restart", "deployment/tenant-service", "-n", authNS) // #nosec G204,G702
	if out, err := restartCmd.CombinedOutput(); err != nil {
		fmt.Printf("=== Warning: restart tenant-service returned error: %v\n%s\n", err, string(out))
	}

	// Poll for tenant-service to become ready instead of a fixed 45-second sleep.
	// Uses "kubectl rollout status" which returns as soon as the rollout completes.
	fmt.Println("=== Waiting for tenant-service rollout to complete ===")
	rolloutCmd := exec.Command("kubectl", "rollout", "status", "deployment/tenant-service", // #nosec G204,G702
		"-n", authNS, "--timeout=45s")
	if out, err := rolloutCmd.CombinedOutput(); err != nil {
		fmt.Printf("=== Warning: tenant-service rollout status: %v\n%s\n", err, string(out))
	} else {
		fmt.Println("=== tenant-service rollout completed ===")
	}
	// Brief pause to let the tenant-service reconcile newly created CRs
	time.Sleep(5 * time.Second)

	lastAuthCRRecreation[driver] = time.Now()
}

func (step *Step) validateResiliencyInstalled(cr csmv1.ContainerStorageModule) error {
	dpApply, dsApply, err := getApplyDeploymentDaemonSet(cr, step.ctrlClient)
	if err != nil {
		return err
	}

	var presentInNode, presentInController bool
	// check whether podmon container is present in cluster or not: for controller
	for _, cnt := range dpApply.Spec.Template.Spec.Containers {
		if *cnt.Name == "podmon" {
			presentInController = true
			break
		}
	}

	// check whether podmon container is present in cluster or not: for node
	for _, cnt := range dsApply.Spec.Template.Spec.Containers {
		if *cnt.Name == "podmon" {
			presentInNode = true
			break
		}
	}

	if !presentInNode || !presentInController {
		return fmt.Errorf("podmon container not found either in controller or node pod")
	}

	return nil
}

func (step *Step) validateResiliencyNotInstalled(cr csmv1.ContainerStorageModule) error {
	// check that resiliency sidecar(podmon) is not in cluster: for controller
	dp, err := getDriverDeployment(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}
	for _, cnt := range dp.Spec.Template.Spec.Containers {
		if cnt.Name == operatorutils.ResiliencySideCarName {
			return fmt.Errorf("found %s: %v", operatorutils.ResiliencySideCarName, err)
		}
	}

	// check that resiliency sidecar(podmon) is not in cluster: for node
	ds, err := getDriverDaemonset(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get daemonset: %v", err)
	}
	for _, cnt := range ds.Spec.Template.Spec.Containers {
		if cnt.Name == operatorutils.ResiliencySideCarName {
			return fmt.Errorf("found %s: %v", operatorutils.ResiliencySideCarName, err)
		}
	}
	return nil
}

// validateResiliencyMetricsService validates that the resiliency metrics Service is created with correct configuration.
func (step *Step) validateResiliencyMetricsService(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedServiceName := cr.Name + "-resiliency-metrics"

	// Determine expected metrics port from CR
	expectedPort := 8444
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Resiliency && m.Metrics != nil && m.Metrics.Port != 0 {
			expectedPort = int(m.Metrics.Port)
		}
	}

	// Check if Service exists using kubectl
	cmd := exec.Command("kubectl", "get", "svc", expectedServiceName, "-n", cr.Namespace)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("resiliency metrics service %s not found: %v, output: %s", expectedServiceName, err, string(output))
	}

	// Check Service port is 8444 (allow port name "metrics" or "resiliency-metrics")
	cmd = exec.Command("kubectl", "get", "svc", expectedServiceName, "-n", cr.Namespace, "-o", "jsonpath={.spec.ports[0].name}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get service port name: %v, output: %s", err, string(output))
	}
	portName := strings.TrimSpace(string(output))
	if portName != "metrics" && portName != "resiliency-metrics" {
		return fmt.Errorf("resiliency metrics service port name is %s, expected metrics or resiliency-metrics", portName)
	}

	cmd = exec.Command("kubectl", "get", "svc", expectedServiceName, "-n", cr.Namespace, "-o", "jsonpath={.spec.ports[0].port}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get service port: %v, output: %s", err, string(output))
	}
	portStr := strings.TrimSpace(string(output))
	if portStr != strconv.Itoa(expectedPort) {
		return fmt.Errorf("resiliency metrics service port is %s, expected %d", portStr, expectedPort)
	}

	// Check Service selector matches controller pods

	// PowerStore and PowerFlex use the "name" label key; others use "app"
	controllerLabelKey := "app"
	if cr.Spec.Driver.CSIDriverType == csmv1.PowerStore || cr.Spec.Driver.CSIDriverType == csmv1.PowerFlex {
		controllerLabelKey = "name"
	}

	cmd = exec.Command("kubectl", "get", "svc", expectedServiceName, "-n", cr.Namespace, "-o", "jsonpath={.spec.selector."+controllerLabelKey+"}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get service selector: %v, output: %s", err, string(output))
	}
	selector := strings.TrimSpace(string(output))
	expectedSelector := cr.Name + "-controller"
	if selector != expectedSelector {
		return fmt.Errorf("resiliency metrics service selector is %s, expected %s", selector, expectedSelector)
	}

	return nil
}

// validateResiliencyServiceMonitor validates that the resiliency ServiceMonitor is created with correct configuration.
func (step *Step) validateResiliencyServiceMonitor(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedSMName := cr.Name + "-resiliency-metrics"

	// Check if ServiceMonitor exists using kubectl
	cmd := exec.Command("kubectl", "get", "servicemonitor", expectedSMName, "-n", cr.Namespace)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("resiliency ServiceMonitor %s not found: %v, output: %s", expectedSMName, err, string(output))
	}

	// PowerStore and PowerFlex use the "name" label key; others use "app"
	controllerLabelKey := "app"
	if cr.Spec.Driver.CSIDriverType == csmv1.PowerStore || cr.Spec.Driver.CSIDriverType == csmv1.PowerFlex {
		controllerLabelKey = "name"
	}

	// Check ServiceMonitor selector matches controller pods
	cmd = exec.Command("kubectl", "get", "servicemonitor", expectedSMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.selector.matchLabels."+controllerLabelKey+"}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get ServiceMonitor selector: %v, output: %s", err, string(output))
	}
	selector := strings.TrimSpace(string(output))
	expectedSelector := cr.Name + "-controller"
	if selector != expectedSelector {
		return fmt.Errorf("resiliency ServiceMonitor selector is %s, expected %s", selector, expectedSelector)
	}

	// Check ServiceMonitor targets metrics port (allow "metrics" or "resiliency-metrics")
	cmd = exec.Command("kubectl", "get", "servicemonitor", expectedSMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.endpoints[0].port}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get ServiceMonitor port: %v, output: %s", err, string(output))
	}
	port := strings.TrimSpace(string(output))
	if port != "metrics" && port != "resiliency-metrics" {
		return fmt.Errorf("resiliency ServiceMonitor port is %s, expected metrics or resiliency-metrics", port)
	}

	return nil
}

// validateResiliencyPodMonitor validates that the resiliency PodMonitor is created with correct configuration.
func (step *Step) validateResiliencyPodMonitor(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedPMName := cr.Name + "-resiliency-metrics"

	// Check if PodMonitor exists using kubectl
	cmd := exec.Command("kubectl", "get", "podmonitor", expectedPMName, "-n", cr.Namespace)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("resiliency PodMonitor %s not found: %v, output: %s", expectedPMName, err, string(output))
	}

	// Check PodMonitor selector matches node pods
	cmd = exec.Command("kubectl", "get", "podmonitor", expectedPMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.selector.matchLabels.app}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PodMonitor selector: %v, output: %s", err, string(output))
	}
	selector := strings.TrimSpace(string(output))
	expectedSelector := cr.Name + "-node"
	if selector != expectedSelector {
		return fmt.Errorf("resiliency PodMonitor selector is %s, expected %s", selector, expectedSelector)
	}

	// Check PodMonitor targets metrics port (allow "metrics" or "resiliency-metrics")
	cmd = exec.Command("kubectl", "get", "podmonitor", expectedPMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.podMetricsEndpoints[0].port}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PodMonitor port: %v, output: %s", err, string(output))
	}
	port := strings.TrimSpace(string(output))
	if port != "metrics" && port != "resiliency-metrics" {
		return fmt.Errorf("resiliency PodMonitor port is %s, expected metrics or resiliency-metrics", port)
	}

	return nil
}

// validateResiliencyMetricsPort validates that the podmon container has the res-metrics port.
func (step *Step) validateResiliencyMetricsPort(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	// Determine expected metrics port from CR
	expectedPort := int32(8444)
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Resiliency && m.Metrics != nil && m.Metrics.Port != 0 {
			expectedPort = m.Metrics.Port
		}
	}

	// Get deployment to check controller pod
	dpApply, _, err := getApplyDeploymentDaemonSet(cr, step.ctrlClient)
	if err != nil {
		return err
	}

	// Check podmon container in controller has port 8444 (allow name "metrics" or "res-metrics")
	for _, cnt := range dpApply.Spec.Template.Spec.Containers {
		if cnt.Name != nil && *cnt.Name == "podmon" {
			foundMetricsPort := false
			for _, port := range cnt.Ports {
				if port.ContainerPort != nil && *port.ContainerPort == expectedPort && port.Name != nil && (*port.Name == "res-metrics" || *port.Name == "metrics") {
					foundMetricsPort = true
					break
				}
			}
			if !foundMetricsPort {
				return fmt.Errorf("podmon container does not have metrics/res-metrics port %d", expectedPort)
			}
		}
	}

	// Get daemonset to check node pod
	_, dsApply, err := getApplyDeploymentDaemonSet(cr, step.ctrlClient)
	if err != nil {
		return err
	}

	// Check podmon container in node has expected metrics/res-metrics port
	for _, cnt := range dsApply.Spec.Template.Spec.Containers {
		if cnt.Name != nil && *cnt.Name == "podmon" {
			foundMetricsPort := false
			for _, port := range cnt.Ports {
				if port.ContainerPort != nil && *port.ContainerPort == expectedPort && port.Name != nil && (*port.Name == "res-metrics" || *port.Name == "metrics") {
					foundMetricsPort = true
					break
				}
			}
			if !foundMetricsPort {
				return fmt.Errorf("podmon container in node does not have metrics/res-metrics port %d", expectedPort)
			}
		}
	}

	return nil
}

// validateResiliencyMetricsTLSVolumeMount validates that the resiliency metrics TLS volume is mounted in podmon containers.
func (step *Step) validateResiliencyMetricsTLSVolumeMount(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	// Get deployment to check controller pod
	dpApply, _, err := getApplyDeploymentDaemonSet(cr, step.ctrlClient)
	if err != nil {
		return err
	}

	// Check podmon container in controller has TLS volume mount
	for _, cnt := range dpApply.Spec.Template.Spec.Containers {
		if cnt.Name != nil && *cnt.Name == "podmon" {
			foundTLSVolume := false
			for _, vm := range cnt.VolumeMounts {
				if vm.Name != nil && *vm.Name == "resiliency-metrics-tls" && vm.MountPath != nil && *vm.MountPath == "/etc/metrics-tls" {
					foundTLSVolume = true
					break
				}
			}
			if !foundTLSVolume {
				return fmt.Errorf("podmon container in controller does not have TLS volume mount")
			}
		}
	}

	// Get daemonset to check node pod
	_, dsApply, err := getApplyDeploymentDaemonSet(cr, step.ctrlClient)
	if err != nil {
		return err
	}

	// Check podmon container in node has TLS volume mount
	for _, cnt := range dsApply.Spec.Template.Spec.Containers {
		if cnt.Name != nil && *cnt.Name == "podmon" {
			foundTLSVolume := false
			for _, vm := range cnt.VolumeMounts {
				if vm.Name != nil && *vm.Name == "resiliency-metrics-tls" && vm.MountPath != nil && *vm.MountPath == "/etc/metrics-tls" {
					foundTLSVolume = true
					break
				}
			}
			if !foundTLSVolume {
				return fmt.Errorf("podmon container in node does not have TLS volume mount")
			}
		}
	}

	return nil
}

// validateResiliencyMetricsServiceMonitorTLS validates that the resiliency metrics ServiceMonitor has TLS configuration.
func (step *Step) validateResiliencyMetricsServiceMonitorTLS(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedSMName := cr.Name + "-resiliency-metrics"

	// Check if ServiceMonitor exists
	cmd := exec.Command("kubectl", "get", "servicemonitor", expectedSMName, "-n", cr.Namespace)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("resiliency ServiceMonitor %s not found: %v, output: %s", expectedSMName, err, string(output))
	}

	// Check ServiceMonitor has scheme: https
	cmd = exec.Command("kubectl", "get", "servicemonitor", expectedSMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.endpoints[0].scheme}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get ServiceMonitor scheme: %v, output: %s", err, string(output))
	}
	scheme := strings.TrimSpace(string(output))
	if scheme != "https" {
		return fmt.Errorf("resiliency ServiceMonitor scheme is %s, expected https", scheme)
	}

	// Check ServiceMonitor has tlsConfig
	cmd = exec.Command("kubectl", "get", "servicemonitor", expectedSMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.endpoints[0].tlsConfig}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get ServiceMonitor tlsConfig: %v, output: %s", err, string(output))
	}
	tlsConfig := strings.TrimSpace(string(output))
	if tlsConfig == "" {
		return fmt.Errorf("resiliency ServiceMonitor does not have tlsConfig")
	}

	return nil
}

// validateResiliencyMetricsPodMonitorTLS validates that the resiliency metrics PodMonitor has TLS configuration.
func (step *Step) validateResiliencyMetricsPodMonitorTLS(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedPMName := cr.Name + "-resiliency-metrics"

	// Check if PodMonitor exists
	cmd := exec.Command("kubectl", "get", "podmonitor", expectedPMName, "-n", cr.Namespace)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("resiliency PodMonitor %s not found: %v, output: %s", expectedPMName, err, string(output))
	}

	// Check PodMonitor has scheme: https
	cmd = exec.Command("kubectl", "get", "podmonitor", expectedPMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.podMetricsEndpoints[0].scheme}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PodMonitor scheme: %v, output: %s", err, string(output))
	}
	scheme := strings.TrimSpace(string(output))
	if scheme != "https" {
		return fmt.Errorf("resiliency PodMonitor scheme is %s, expected https", scheme)
	}

	// Check PodMonitor has tlsConfig
	cmd = exec.Command("kubectl", "get", "podmonitor", expectedPMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.podMetricsEndpoints[0].tlsConfig}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PodMonitor tlsConfig: %v, output: %s", err, string(output))
	}
	tlsConfig := strings.TrimSpace(string(output))
	if tlsConfig == "" {
		return fmt.Errorf("resiliency PodMonitor does not have tlsConfig")
	}

	return nil
}

// getPodNameForSelector returns the name of the first pod in namespace that
// matches any of the provided label keys with value <suffix>, or empty string if none found.
func getPodNameForSelector(namespace, suffix string, labelKeys []string) string {
	for _, key := range labelKeys {
		cmd := exec.Command("kubectl", "get", "pods", "-n", namespace, "-l", key+"="+suffix, "-o", "jsonpath={.items[0].metadata.name}")
		output, err := cmd.CombinedOutput()
		if err == nil {
			podName := strings.TrimSpace(string(output))
			if podName != "" {
				return podName
			}
		}
	}
	return ""
}

// validateResiliencyMetricsTLSEnvVars validates resiliency metrics TLS environment variables using CSV file.
func (step *Step) validateResiliencyMetricsTLSEnvVars(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	// Determine the CSV file based on driver type
	var csvFile string
	driverType := string(cr.Spec.Driver.CSIDriverType)
	switch driverType {
	case "powerstore":
		csvFile = "testfiles/powerstore_resiliency_metrics_tls_values.csv"
	case "powerscale", "isilon":
		csvFile = "testfiles/powerscale_resiliency_metrics_tls_values.csv"
	case "powermax":
		csvFile = "testfiles/powermax_resiliency_metrics_tls_values.csv"
	case "powerflex":
		csvFile = "testfiles/powerflex_resiliency_metrics_tls_values.csv"
	default:
		return fmt.Errorf("unsupported driver type: %s", driverType)
	}

	// Read CSV file
	content, err := os.ReadFile(csvFile)
	if err != nil {
		return fmt.Errorf("failed to read CSV file %s: %v", csvFile, err)
	}

	// Expand environment placeholders (e.g., ${E2E_NS_POWERSCALE}) before parsing.
	expandedContent := os.ExpandEnv(string(content))

	// Parse CSV
	reader := csv.NewReader(strings.NewReader(expandedContent))
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to parse CSV file: %v", err)
	}

	// Skip header row
	for i := 1; i < len(records); i++ {
		record := records[i]
		if len(record) < 5 {
			continue
		}
		parameter := record[0]
		namespace := record[1]
		driver := record[2]
		container := record[3]
		expectedValue := record[4]

		// Skip if namespace doesn't match
		if namespace != cr.Namespace {
			continue
		}

		// Skip if driver doesn't match
		if driver != driverType {
			continue
		}

		// Get the pod name based on container type
		var podName string
		if container == "podmon-controller" {
			// Get the controller pod. Different drivers use different labels
			// (app for PowerScale, name for PowerStore), so try both.
			podName = getPodNameForSelector(cr.Namespace, cr.Name+"-controller", []string{"app", "name"})
			if podName == "" {
				return fmt.Errorf("no controller pod found")
			}
		} else if container == "podmon-node" {
			// Get a node pod
			podName = getPodNameForSelector(cr.Namespace, cr.Name+"-node", []string{"app", "name"})
			if podName == "" {
				return fmt.Errorf("no node pod found")
			}
		} else {
			continue
		}

		// Get the environment variable value from the pod
		cmd := exec.Command("kubectl", "exec", podName, "-n", cr.Namespace, "-c", "podmon", "--", "sh", "-c", fmt.Sprintf("echo $%s", parameter))
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to get env var %s from pod %s: %v, output: %s", parameter, podName, err, string(output))
		}
		actualValue := strings.TrimSpace(string(output))

		// Compare with expected value
		if actualValue != expectedValue {
			return fmt.Errorf("env var %s in pod %s is %s, expected %s", parameter, podName, actualValue, expectedValue)
		}
	}

	return nil
}

// validateReplicationMetricsService validates that the replication metrics Service is created with correct configuration.
func (step *Step) validateReplicationMetricsService(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedServiceName := cr.Name + "-replication-metrics"

	cmd := exec.Command("kubectl", "get", "svc", expectedServiceName, "-n", cr.Namespace)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("replication metrics service %s not found: %v, output: %s", expectedServiceName, err, string(output))
	}

	cmd = exec.Command("kubectl", "get", "svc", expectedServiceName, "-n", cr.Namespace, "-o", "jsonpath={.spec.ports[?(@.name==\"metrics\")].port}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get service port: %v, output: %s", err, string(output))
	}
	portStr := strings.TrimSpace(string(output))
	if portStr != "8445" {
		return fmt.Errorf("replication metrics service port is %s, expected 8445", portStr)
	}

	// PowerStore and PowerFlex use the "name" label key; others use "app"
	controllerLabelKey := "app"
	if cr.Spec.Driver.CSIDriverType == csmv1.PowerStore || cr.Spec.Driver.CSIDriverType == csmv1.PowerFlex {
		controllerLabelKey = "name"
	}

	cmd = exec.Command("kubectl", "get", "svc", expectedServiceName, "-n", cr.Namespace, "-o", "jsonpath={.spec.selector."+controllerLabelKey+"}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get service selector: %v, output: %s", err, string(output))
	}
	selector := strings.TrimSpace(string(output))
	expectedSelector := cr.Name + "-controller"
	if selector != expectedSelector {
		return fmt.Errorf("replication metrics service selector is %s, expected %s", selector, expectedSelector)
	}

	return nil
}

// validateReplicationMetricsServiceMonitor validates that the replication metrics ServiceMonitor is created with correct configuration.
func (step *Step) validateReplicationMetricsServiceMonitor(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedSMName := cr.Name + "-replication-metrics"

	cmd := exec.Command("kubectl", "get", "servicemonitor", expectedSMName, "-n", cr.Namespace)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("replication ServiceMonitor %s not found: %v, output: %s", expectedSMName, err, string(output))
	}

	// PowerStore and PowerFlex use the "name" label key; others use "app"
	controllerLabelKey := "app"
	if cr.Spec.Driver.CSIDriverType == csmv1.PowerStore || cr.Spec.Driver.CSIDriverType == csmv1.PowerFlex {
		controllerLabelKey = "name"
	}

	cmd = exec.Command("kubectl", "get", "servicemonitor", expectedSMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.selector.matchLabels."+controllerLabelKey+"}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get ServiceMonitor selector: %v, output: %s", err, string(output))
	}
	selector := strings.TrimSpace(string(output))
	expectedSelector := cr.Name + "-controller"
	if selector != expectedSelector {
		return fmt.Errorf("replication ServiceMonitor selector is %s, expected %s", selector, expectedSelector)
	}

	cmd = exec.Command("kubectl", "get", "servicemonitor", expectedSMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.endpoints[0].port}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get ServiceMonitor port: %v, output: %s", err, string(output))
	}
	port := strings.TrimSpace(string(output))
	if port != "metrics" {
		return fmt.Errorf("replication ServiceMonitor port is %s, expected metrics", port)
	}

	return nil
}

// validateReplicationMetricsPodMonitor validates that the replication metrics PodMonitor is created with correct configuration.
func (step *Step) validateReplicationMetricsPodMonitor(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedPMName := cr.Name + "-replication-metrics"

	cmd := exec.Command("kubectl", "get", "podmonitor", expectedPMName, "-n", cr.Namespace)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("replication PodMonitor %s not found: %v, output: %s", expectedPMName, err, string(output))
	}

	cmd = exec.Command("kubectl", "get", "podmonitor", expectedPMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.selector.matchLabels.app}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PodMonitor selector: %v, output: %s", err, string(output))
	}
	selector := strings.TrimSpace(string(output))
	expectedSelector := cr.Name + "-node"
	if selector != expectedSelector {
		return fmt.Errorf("replication PodMonitor selector is %s, expected %s", selector, expectedSelector)
	}

	cmd = exec.Command("kubectl", "get", "podmonitor", expectedPMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.podMetricsEndpoints[0].port}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PodMonitor port: %v, output: %s", err, string(output))
	}
	port := strings.TrimSpace(string(output))
	if port != "metrics" {
		return fmt.Errorf("replication PodMonitor port is %s, expected metrics", port)
	}

	return nil
}

// validateReplicationMetricsPort validates that the dell-csi-replicator container has the replication metrics port 8445.
func (step *Step) validateReplicationMetricsPort(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	dpApply, _, err := getApplyDeploymentDaemonSet(cr, step.ctrlClient)
	if err != nil {
		return err
	}

	for _, cnt := range dpApply.Spec.Template.Spec.Containers {
		if cnt.Name != nil && *cnt.Name == "dell-csi-replicator" {
			foundMetricsPort := false
			for _, port := range cnt.Ports {
				if port.ContainerPort != nil && *port.ContainerPort == 8445 && port.Name != nil && *port.Name == "metrics" {
					foundMetricsPort = true
					break
				}
			}
			if !foundMetricsPort {
				return fmt.Errorf("dell-csi-replicator container in controller does not have metrics port 8445")
			}
		}
	}

	return nil
}

// validateReplicationMetricsTLSVolumeMount validates that the replication metrics TLS volume is mounted in the dell-csi-replicator container.
func (step *Step) validateReplicationMetricsTLSVolumeMount(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	dpApply, _, err := getApplyDeploymentDaemonSet(cr, step.ctrlClient)
	if err != nil {
		return err
	}

	for _, cnt := range dpApply.Spec.Template.Spec.Containers {
		if cnt.Name != nil && *cnt.Name == "dell-csi-replicator" {
			foundTLSVolume := false
			for _, vm := range cnt.VolumeMounts {
				if vm.Name != nil && *vm.Name == "replication-metrics-tls" && vm.MountPath != nil && *vm.MountPath == "/etc/replication-metrics-tls" {
					foundTLSVolume = true
					break
				}
			}
			if !foundTLSVolume {
				return fmt.Errorf("dell-csi-replicator container in controller does not have TLS volume mount")
			}
		}
	}

	return nil
}

// validateReplicationMetricsServiceMonitorTLS validates that the replication metrics ServiceMonitor has TLS configuration.
func (step *Step) validateReplicationMetricsServiceMonitorTLS(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedSMName := cr.Name + "-replication-metrics"

	cmd := exec.Command("kubectl", "get", "servicemonitor", expectedSMName, "-n", cr.Namespace)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("replication ServiceMonitor %s not found: %v, output: %s", expectedSMName, err, string(output))
	}

	cmd = exec.Command("kubectl", "get", "servicemonitor", expectedSMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.endpoints[0].scheme}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get ServiceMonitor scheme: %v, output: %s", err, string(output))
	}
	scheme := strings.TrimSpace(string(output))
	if scheme != "https" {
		return fmt.Errorf("replication ServiceMonitor scheme is %s, expected https", scheme)
	}

	cmd = exec.Command("kubectl", "get", "servicemonitor", expectedSMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.endpoints[0].tlsConfig}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get ServiceMonitor tlsConfig: %v, output: %s", err, string(output))
	}
	tlsConfig := strings.TrimSpace(string(output))
	if tlsConfig == "" {
		return fmt.Errorf("replication ServiceMonitor does not have tlsConfig")
	}

	return nil
}

// validateReplicationMetricsPodMonitorTLS validates that the replication metrics PodMonitor has TLS configuration.
func (step *Step) validateReplicationMetricsPodMonitorTLS(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedPMName := cr.Name + "-replication-metrics"

	cmd := exec.Command("kubectl", "get", "podmonitor", expectedPMName, "-n", cr.Namespace)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("replication PodMonitor %s not found: %v, output: %s", expectedPMName, err, string(output))
	}

	cmd = exec.Command("kubectl", "get", "podmonitor", expectedPMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.podMetricsEndpoints[0].scheme}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PodMonitor scheme: %v, output: %s", err, string(output))
	}
	scheme := strings.TrimSpace(string(output))
	if scheme != "https" {
		return fmt.Errorf("replication PodMonitor scheme is %s, expected https", scheme)
	}

	cmd = exec.Command("kubectl", "get", "podmonitor", expectedPMName, "-n", cr.Namespace, "-o", "jsonpath={.spec.podMetricsEndpoints[0].tlsConfig}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PodMonitor tlsConfig: %v, output: %s", err, string(output))
	}
	tlsConfig := strings.TrimSpace(string(output))
	if tlsConfig == "" {
		return fmt.Errorf("replication PodMonitor does not have tlsConfig")
	}

	return nil
}

// copySecret copies a secret from one namespace to another.
func (step *Step) copySecret(res Resource, secretName, fromNamespace, toNamespace string) error {
	// Delete the secret if it already exists in target namespace
	cmd := exec.Command("kubectl", "delete", "secret", secretName, "-n", toNamespace, "--ignore-not-found")
	_ = cmd.Run()

	// Get the secret YAML and change the namespace
	cmd = exec.Command("sh", "-c", fmt.Sprintf("kubectl get secret %s -n %s -o yaml | sed 's/namespace: %s/namespace: %s/g' | kubectl apply -f -", secretName, fromNamespace, fromNamespace, toNamespace))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy secret: %v, output: %s", err, string(output))
	}

	return nil
}

func (step *Step) validateCSIAddonsSidecarInstalled(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	dpApply, _, err := getApplyDeploymentDaemonSet(*found, step.ctrlClient)
	if err != nil {
		return err
	}
	for _, cnt := range dpApply.Spec.Template.Spec.Containers {
		if cnt.Name != nil && *cnt.Name == "csi-addons" {
			return checkAllRunningPods(context.TODO(), found.Namespace, step.clientSet)
		}
	}
	return fmt.Errorf("container %s not found in controller deployment", "csi-addons")
}

// Configure Powerflex SFTP by setting X_CSI_SDC_SFTP_REPO_ENABLED, REPO_ADDRESS, and REPO_USER in the CR
func (step *Step) configurePowerflexSftpInstall(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}

	// Get SFTP configuration from environment variables
	repoAddress := os.Getenv("POWERFLEX_SFTP_REPO_ADDRESS")
	repoUser := os.Getenv("POWERFLEX_SFTP_REPO_USER")

	// First, set the SFTP environment variable to true
	err = step.setEnvInSpec(res, "node", "X_CSI_SDC_SFTP_REPO_ENABLED", "true", crNumStr)
	if err != nil {
		return fmt.Errorf("failed to set X_CSI_SDC_SFTP_REPO_ENABLED to true: %v", err)
	}

	// Use yq to set REPO_ADDRESS in initContainers if provided
	if repoAddress != "" {
		crFilePath := res.Scenario.Paths[crNum-1]
		// Read the file, modify with yq, and write back
		cmd := exec.Command("yq", "-i",
			fmt.Sprintf("(.spec.driver.initContainers[] | select(.name == \"sdc\") | .envs[] | select(.name == \"REPO_ADDRESS\") | .value) = \"%s\"", repoAddress),
			crFilePath) // #nosec G702 -- this is a test automation tool
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set REPO_ADDRESS with yq: %v", err)
		}
	}

	// Use yq to set REPO_USER in initContainers if provided
	if repoUser != "" {
		crFilePath := res.Scenario.Paths[crNum-1]
		// Read the file, modify with yq, and write back
		cmd := exec.Command("yq", "-i",
			fmt.Sprintf("(.spec.driver.initContainers[] | select(.name == \"sdc\") | .envs[] | select(.name == \"REPO_USER\") | .value) = \"%s\"", repoUser),
			crFilePath) // #nosec G702 -- this is a test automation tool
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set REPO_USER with yq: %v", err)
		}
	}

	// Then render the template to apply any other substitutions
	crFilePath := res.Scenario.Paths[crNum-1]
	fileString, err := renderTemplate("powerflex", crFilePath)
	if err != nil {
		return err
	}

	filePath, err := writeRenderedFile(crFilePath, fileString)
	if err != nil {
		return err
	}
	fmt.Printf("Configured SFTP and rendered template %s into %s\n", crFilePath, filePath)

	return nil
}

// Configure Powerflex SFTP: combines secrets generation and CR configuration
func (step *Step) configurePowerflexSFTP(res Resource, privateKeyPath, privateSecretName, publicSecretName, namespace, crType, crNumStr string) error {
	// First, generate and create SFTP secrets
	err := step.generateAndCreateSftpSecrets(res, privateKeyPath, privateSecretName, publicSecretName, namespace, crType)
	if err != nil {
		return fmt.Errorf("failed to generate and create SFTP secrets: %v", err)
	}

	// Then, configure the CR with SFTP settings
	err = step.configurePowerflexSftpInstall(res, crNumStr)
	if err != nil {
		return fmt.Errorf("failed to configure Powerflex SFTP CR: %v", err)
	}

	return nil
}

func (step *Step) createCustomResourceDefinition(res Resource, crdNumStr string) error {
	crdNum, err := parseIndex(crdNumStr)
	if err != nil {
		return err
	}
	cmd := exec.Command("kubectl", "apply", "-f", res.Scenario.Paths[crdNum-1]) // #nosec G204
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("csm authorization crds install failed: %v", err)
	}

	return nil
}

func (step *Step) validateCustomResourceDefinition(res Resource, crdName string) error {
	cmd := exec.Command("kubectl", "get", "crd", fmt.Sprintf("%s.csm-authorization.storage.dell.com", crdName)) // #nosec G204
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to validate csm authorization crd [%s]: %v", crdName, err)
	}

	return nil
}

// deleteAuthorizationCRs will delete storage, role, and tenant objects
func (step *Step) deleteAuthorizationCRs(_ Resource, driver string) error {
	updatedTemplateFile := ""
	if driver == "powerflex" {
		updatedTemplateFile = "temp/authorization-templates/storage_csm_authorization_crs_powerflex.yaml"
	} else if driver == "powerscale" {
		updatedTemplateFile = "temp/authorization-templates/storage_csm_authorization_crs_powerscale.yaml"
	} else if driver == "powermax" {
		updatedTemplateFile = "temp/authorization-templates/storage_csm_authorization_crs_powermax.yaml"
	} else if driver == "powerstore" {
		updatedTemplateFile = "temp/authorization-templates/storage_csm_authorization_crs_powerstore.yaml"
	}

	// Strip finalizers from authorization CRs before deletion so that the
	// kubectl delete does not hang indefinitely waiting for the authorization
	// controller to process them.
	authNS := os.Getenv("E2E_NS_AUTH")
	if authNS == "" {
		authNS = "e2e-authorization"
	}
	removeAuthorizationFinalizers(authNS)

	cmd := exec.Command("kubectl", "delete", "-f", updatedTemplateFile, "--wait=false", "--ignore-not-found") // #nosec G204
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to delete csm authorization CRs: %v", err)
	}
	return nil
}

func (step *Step) deleteCustomResourceDefinition(res Resource, crdNumStr string) error {
	crdNum, err := parseIndex(crdNumStr)
	if err != nil {
		return err
	}
	cmd := exec.Command("kubectl", "delete", "-f", res.Scenario.Paths[crdNum-1], "--ignore-not-found") // #nosec G204
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("csm authorization crds uninstall failed: %v", err)
	}
	return nil
}

func (step *Step) removeAuthorizationFinalizers(_ Resource, namespace string) error {
	removeAuthorizationFinalizers(namespace)
	return nil
}

func (step *Step) setUpReverseProxy(_ Resource, namespace string) error {
	// Check if the revproxy-certs secret exists
	revproxyExists := false
	cmd := exec.Command("kubectl", "get", "secret", "revproxy-certs", "-n", namespace) // #nosec G204
	err := cmd.Run()
	if err == nil {
		fmt.Println("revproxy-certs secret already exists, skipping creation.")
		revproxyExists = true
	}

	// Check if the csirevproxy-tls-secret exists
	csirevproxyExists := false
	cmd = exec.Command("kubectl", "get", "secret", "csirevproxy-tls-secret", "-n", namespace) // #nosec G204
	err = cmd.Run()
	if err == nil {
		fmt.Println("csirevproxy-tls-secret already exists, skipping creation.")
		csirevproxyExists = true
	}

	// If both secrets exist, no need to generate TLS key and certificate
	if revproxyExists && csirevproxyExists {
		return nil
	}

	// Paths for the key and certificate files
	keyPath := "temp/tls.key"
	crtPath := "temp/tls.crt"

	// Generate TLS key
	cmd = exec.Command("openssl", "genrsa", "-out", keyPath, "2048") // #nosec G204
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to generate TLS key: %v", err)
	}

	// Generate TLS certificate
	cmd = exec.Command("openssl", "req", "-new", "-x509", "-sha256", "-key", keyPath, "-out", crtPath, "-days", "3650", "-subj", "/CN=US") // #nosec G204
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %v", err)
	}

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return fmt.Errorf("key file does not exist: %s", keyPath)
	}
	if _, err := os.Stat(crtPath); os.IsNotExist(err) {
		return fmt.Errorf("cert file does not exist: %s", crtPath)
	}

	// Create Kubernetes secret for revproxy-certs if it does not exist
	if !revproxyExists {
		cmd = exec.Command("kubectl", "create", "secret", "-n", namespace, "tls", "revproxy-certs", "--cert="+crtPath, "--key="+keyPath) // #nosec G204
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to create revproxy-certs secret: %v", err)
		}
	}

	// Create Kubernetes secret for csirevproxy-tls-secret if it does not exist
	if !csirevproxyExists {
		cmd = exec.Command("kubectl", "create", "secret", "-n", namespace, "tls", "csirevproxy-tls-secret", "--cert="+crtPath, "--key="+keyPath) // #nosec G204
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to create csirevproxy-tls-secret: %v", err)
		}
	}

	return nil
}

func (step *Step) setUpDriverMetricsTLSSecret(driverType string, namespace string) error {
	// Driver-specific configuration
	driverSecretNames := map[string]string{
		"powerflex":  "powerflex-metrics-tls",
		"powerscale": "powerscale-metrics-tls",
		"powerstore": "powerstore-metrics-tls",
		"powermax":   "powermax-metrics-tls",
	}
	driverCNs := map[string]string{
		"powerflex":  "powerflex-metrics",
		"powerscale": "powerscale-metrics",
		"powerstore": "powerstore-metrics",
		"powermax":   "powermax-metrics",
	}

	secretName, ok := driverSecretNames[driverType]
	if !ok {
		return fmt.Errorf("unsupported driver type: %s", driverType)
	}
	cn := driverCNs[driverType]

	cmd := exec.Command("kubectl", "get", "secret", secretName, "-n", namespace) // #nosec G204
	if err := cmd.Run(); err == nil {
		fmt.Printf("%s secret already exists, skipping creation.\n", secretName)
		return nil
	}

	keyPath := fmt.Sprintf("temp/%s-tls.key", driverType)
	crtPath := fmt.Sprintf("temp/%s-tls.crt", driverType)

	cmd = exec.Command("openssl", "genrsa", "-out", keyPath, "2048") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS key: %v", err)
	}

	subj := fmt.Sprintf("/CN=%s", cn)
	cmd = exec.Command("openssl", "req", "-new", "-x509", "-sha256", "-key", keyPath, "-out", crtPath, "-days", "3650", "-subj", subj) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %v", err)
	}

	cmd = exec.Command("kubectl", "create", "secret", "-n", namespace, "tls", secretName, "--cert="+crtPath, "--key="+keyPath) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create %s secret: %v", secretName, err)
	}

	return nil
}

// setUpMetricsTLSSecret is kept for backward compatibility
func (step *Step) setUpMetricsTLSSecret(_ Resource, namespace string) error {
	return step.setUpDriverMetricsTLSSecret("powerflex", namespace)
}

func (step *Step) setUpAuthorizationMetricsTLSSecret(_ Resource, namespace string) error {
	const secretName = "proxy-server-metrics-tls"

	cmd := exec.Command("kubectl", "get", "secret", secretName, "-n", namespace) // #nosec G204
	if err := cmd.Run(); err == nil {
		fmt.Printf("%s secret already exists, skipping creation.\n", secretName)
		return nil
	}

	keyPath := "temp/proxy-server-metrics-tls.key"
	crtPath := "temp/proxy-server-metrics-tls.crt"

	cmd = exec.Command("openssl", "genrsa", "-out", keyPath, "2048") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS key: %v", err)
	}

	cmd = exec.Command("openssl", "req", "-new", "-x509", "-sha256", "-key", keyPath, "-out", crtPath, "-days", "3650", "-subj", "/CN=proxy-server-metrics") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %v", err)
	}

	cmd = exec.Command("kubectl", "create", "secret", "-n", namespace, "tls", secretName, "--cert="+crtPath, "--key="+keyPath) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create %s secret: %v", secretName, err)
	}

	return nil
}

// setUpPowerScaleMetricsTLSSecret is kept for backward compatibility
func (step *Step) setUpPowerScaleMetricsTLSSecret(_ Resource, namespace string) error {
	const secretName = "powerscale-metrics-tls"

	cmd := exec.Command("kubectl", "get", "secret", secretName, "-n", namespace) // #nosec G204
	if err := cmd.Run(); err == nil {
		fmt.Printf("%s secret already exists, skipping creation.\n", secretName)
		return nil
	}

	keyPath := "temp/powerscale-tls.key"
	crtPath := "temp/powerscale-tls.crt"

	cmd = exec.Command("openssl", "genrsa", "-out", keyPath, "2048") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS key: %v", err)
	}

	cmd = exec.Command("openssl", "req", "-new", "-x509", "-sha256", "-key", keyPath, "-out", crtPath, "-days", "3650", "-subj", "/CN=powerscale-metrics") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %v", err)
	}

	cmd = exec.Command("kubectl", "create", "secret", "-n", namespace, "tls", secretName, "--cert="+crtPath, "--key="+keyPath) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create %s secret: %v", secretName, err)
	}

	return nil
}

// setUpPowerScaleObservabilityTLSSecret creates TLS secrets for PowerScale observability components
func (step *Step) setUpPowerScaleObservabilityTLSSecret(_ Resource, namespace string) error {
	const secretName = "otel-collector-tls" // #nosec G101 -- secret name, not a credential

	cmd := exec.Command("kubectl", "get", "secret", secretName, "-n", namespace) // #nosec G204
	if err := cmd.Run(); err == nil {
		fmt.Printf("%s secret already exists, skipping creation.\n", secretName)
		return nil
	}

	keyPath := "temp/otel-collector-tls.key"
	crtPath := "temp/otel-collector-tls.crt"

	// Ensure cleanup happens even if secret creation fails
	defer func() {
		os.Remove(keyPath)
		os.Remove(crtPath)
	}()

	cmd = exec.Command("openssl", "genrsa", "-out", keyPath, "2048") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS key: %v", err)
	}

	cmd = exec.Command("openssl", "req", "-new", "-x509", "-sha256", "-key", keyPath, "-out", crtPath, "-days", "3650", "-subj", "/CN=otel-collector") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %v", err)
	}

	cmd = exec.Command("kubectl", "create", "secret", "-n", namespace, "tls", secretName, "--cert="+crtPath, "--key="+keyPath) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create %s secret: %v", secretName, err)
	}

	return nil
}

// setUpPowerStoreMetricsTLSSecret is kept for backward compatibility
func (step *Step) setUpPowerStoreMetricsTLSSecret(_ Resource, namespace string) error {
	const secretName = "powerstore-metrics-tls"

	cmd := exec.Command("kubectl", "get", "secret", secretName, "-n", namespace) // #nosec G204
	if err := cmd.Run(); err == nil {
		fmt.Printf("%s secret already exists, skipping creation.\n", secretName)
		return nil
	}

	keyPath := "temp/powerstore-tls.key"
	crtPath := "temp/powerstore-tls.crt"

	cmd = exec.Command("openssl", "genrsa", "-out", keyPath, "2048") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS key: %v", err)
	}

	cmd = exec.Command("openssl", "req", "-new", "-x509", "-sha256", "-key", keyPath, "-out", crtPath, "-days", "3650", "-subj", "/CN=powerstore-metrics") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %v", err)
	}

	cmd = exec.Command("kubectl", "create", "secret", "-n", namespace, "tls", secretName, "--cert="+crtPath, "--key="+keyPath) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create %s secret: %v", secretName, err)
	}

	return nil
}

// setUpPowerMaxMetricsTLSSecret is kept for backward compatibility
func (step *Step) setUpPowerMaxMetricsTLSSecret(_ Resource, namespace string) error {
	const secretName = "powermax-metrics-tls"

	cmd := exec.Command("kubectl", "get", "secret", secretName, "-n", namespace) // #nosec G204
	if err := cmd.Run(); err == nil {
		fmt.Printf("%s secret already exists, skipping creation.\n", secretName)
		return nil
	}

	keyPath := "temp/powermax-tls.key"
	crtPath := "temp/powermax-tls.crt"

	cmd = exec.Command("openssl", "genrsa", "-out", keyPath, "2048") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS key: %v", err)
	}

	cmd = exec.Command("openssl", "req", "-new", "-x509", "-sha256", "-key", keyPath, "-out", crtPath, "-days", "3650", "-subj", "/CN=powermax-metrics") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %v", err)
	}

	cmd = exec.Command("kubectl", "create", "secret", "-n", namespace, "tls", secretName, "--cert="+crtPath, "--key="+keyPath) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create %s secret: %v", secretName, err)
	}

	return nil
}

func (step *Step) setUpReplicationMetricsTLSSecret(_ Resource, namespace string) error {
	const secretName = "powerflex-replication-metrics-tls" // #nosec G101

	cmd := exec.Command("kubectl", "get", "secret", secretName, "-n", namespace) // #nosec G204
	if err := cmd.Run(); err == nil {
		fmt.Printf("%s secret already exists, skipping creation.\n", secretName)
		return nil
	}

	keyPath := "temp/powerflex-replication-tls.key"
	crtPath := "temp/powerflex-replication-tls.crt"

	cmd = exec.Command("openssl", "genrsa", "-out", keyPath, "2048") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS key: %v", err)
	}

	cmd = exec.Command("openssl", "req", "-new", "-x509", "-sha256", "-key", keyPath, "-out", crtPath, "-days", "3650", "-subj", "/CN=powerflex-replication-metrics") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %v", err)
	}

	cmd = exec.Command("kubectl", "create", "secret", "-n", namespace, "tls", secretName, "--cert="+crtPath, "--key="+keyPath) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create %s secret: %v", secretName, err)
	}

	return nil
}

func (step *Step) setUpPowerStoreReplicationMetricsTLSSecret(_ Resource, namespace string) error {
	const secretName = "powerstore-replication-metrics-tls" // #nosec G101

	cmd := exec.Command("kubectl", "get", "secret", secretName, "-n", namespace) // #nosec G204
	if err := cmd.Run(); err == nil {
		fmt.Printf("%s secret already exists, skipping creation.\n", secretName)
		return nil
	}

	keyPath := "temp/powerstore-replication-tls.key"
	crtPath := "temp/powerstore-replication-tls.crt"

	cmd = exec.Command("openssl", "genrsa", "-out", keyPath, "2048") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS key: %v", err)
	}

	cmd = exec.Command("openssl", "req", "-new", "-x509", "-sha256", "-key", keyPath, "-out", crtPath, "-days", "3650", "-subj", "/CN=powerstore-replication-metrics") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %v", err)
	}

	cmd = exec.Command("kubectl", "create", "secret", "-n", namespace, "tls", secretName, "--cert="+crtPath, "--key="+keyPath) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create %s secret: %v", secretName, err)
	}

	return nil
}

func (step *Step) setUpPowerMaxReplicationMetricsTLSSecret(_ Resource, namespace string) error {
	const secretName = "powermax-replication-metrics-tls" // #nosec G101

	cmd := exec.Command("kubectl", "get", "secret", secretName, "-n", namespace) // #nosec G204
	if err := cmd.Run(); err == nil {
		fmt.Printf("%s secret already exists, skipping creation.\n", secretName)
		return nil
	}

	keyPath := "temp/powermax-replication-tls.key"
	crtPath := "temp/powermax-replication-tls.crt"

	cmd = exec.Command("openssl", "genrsa", "-out", keyPath, "2048") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS key: %v", err)
	}

	cmd = exec.Command("openssl", "req", "-new", "-x509", "-sha256", "-key", keyPath, "-out", crtPath, "-days", "3650", "-subj", "/CN=powermax-replication-metrics") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %v", err)
	}

	cmd = exec.Command("kubectl", "create", "secret", "-n", namespace, "tls", secretName, "--cert="+crtPath, "--key="+keyPath) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create %s secret: %v", secretName, err)
	}

	return nil
}

func (step *Step) setUpPowerScaleReplicationMetricsTLSSecret(_ Resource, namespace string) error {
	const secretName = "powerscale-replication-metrics-tls" // #nosec G101

	cmd := exec.Command("kubectl", "get", "secret", secretName, "-n", namespace) // #nosec G204
	if err := cmd.Run(); err == nil {
		fmt.Printf("%s secret already exists, skipping creation.\n", secretName)
		return nil
	}

	keyPath := "temp/powerscale-replication-tls.key"
	crtPath := "temp/powerscale-replication-tls.crt"

	// Ensure cleanup happens even if secret creation fails
	defer func() {
		os.Remove(keyPath)
		os.Remove(crtPath)
	}()

	cmd = exec.Command("openssl", "genrsa", "-out", keyPath, "2048") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS key: %v", err)
	}

	cmd = exec.Command("openssl", "req", "-new", "-x509", "-sha256", "-key", keyPath, "-out", crtPath, "-days", "3650", "-subj", "/CN=powerscale-replication-metrics") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %v", err)
	}

	cmd = exec.Command("kubectl", "create", "secret", "-n", namespace, "tls", secretName, "--cert="+crtPath, "--key="+keyPath) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create %s secret: %v", secretName, err)
	}

	return nil
}

func (step *Step) enableMetrics(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	if found.Spec.Driver.Metrics == nil {
		found.Spec.Driver.Metrics = &csmv1.DriverMetrics{Enabled: true}
	} else {
		found.Spec.Driver.Metrics.Enabled = true
	}

	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) disableMetrics(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	if found.Spec.Driver.Metrics != nil {
		found.Spec.Driver.Metrics.Enabled = false
	}

	return step.ctrlClient.Update(context.TODO(), found)
}

func (step *Step) validateMetricsEndpoints(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	ctx := context.TODO()

	workloads := []struct {
		name   string
		labels map[string]string
	}{
		{name: "controller", labels: map[string]string{"app": cr.Name + "-controller"}},
		{name: "node", labels: map[string]string{"app": cr.Name + "-node"}},
	}

	for _, workload := range workloads {
		pods, err := fpod.GetPodsInNamespace(ctx, step.clientSet, cr.Namespace, workload.labels)
		if err != nil {
			return err
		}
		if len(pods) == 0 {
			return fmt.Errorf("no %s pod found", workload.name)
		}

		for _, pod := range pods {
			foundMetricsPort := false
			for _, container := range pod.Spec.Containers {
				for _, port := range container.Ports {
					if port.Name == "metrics" && port.ContainerPort == 8443 {
						foundMetricsPort = true
						break
					}
				}
				if foundMetricsPort {
					break
				}
			}
			if !foundMetricsPort {
				return fmt.Errorf("metrics port not found in %s pod %s", workload.name, pod.Name)
			}
		}
	}

	return nil
}

func (step *Step) setUpTLSSecretWithSAN(res Resource, namespace string) error {
	// Paths for the key, CSR, and certificate files
	keyPath := "temp/tls.key"
	csrPath := "temp/tls.csr"
	crtPath := "temp/tls.crt"
	sanConfigPath := "testfiles/powermax-templates/san.cnf"

	// Generate TLS key
	cmd := exec.Command("openssl", "genrsa", "-out", keyPath, "2048") // #nosec G204
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to generate TLS key: %v", err)
	}

	// Generate CSR
	cmd = exec.Command("openssl", "req", "-new", "-key", keyPath, "-out", csrPath, "-config", sanConfigPath) // #nosec G204
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to generate CSR: %v", err)
	}

	// Generate TLS certificate
	cmd = exec.Command("openssl", "x509", "-req", "-in", csrPath, "-signkey", keyPath, "-out", crtPath, "-days", "3650", "-extensions", "v3_req", "-extfile", sanConfigPath) // #nosec G204
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %v", err)
	}

	// Create or update Kubernetes secret for revproxy-certs
	cmd = exec.Command("kubectl", "create", "secret", "tls", "revproxy-certs", "--cert="+crtPath, "--key="+keyPath, "-n", namespace, "-o", "yaml", "--dry-run=client") // #nosec G204
	cmdOut, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to prepare revproxy-certs secret: %v", err)
	}

	cmd = exec.Command("kubectl", "apply", "-f", "-") // #nosec G204
	cmd.Stdin = bytes.NewReader(cmdOut)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to apply revproxy-certs secret: %v", err)
	}

	// Create or update Kubernetes secret for csirevproxy-tls-secret
	cmd = exec.Command("kubectl", "create", "secret", "tls", "csirevproxy-tls-secret", "--cert="+crtPath, "--key="+keyPath, "-n", namespace, "-o", "yaml", "--dry-run=client") // #nosec G204
	cmdOut, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to prepare csirevproxy-tls-secret: %v", err)
	}

	cmd = exec.Command("kubectl", "apply", "-f", "-") // #nosec G204
	cmd.Stdin = bytes.NewReader(cmdOut)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to apply csirevproxy-tls-secret: %v", err)
	}

	return nil
}

func (step *Step) restoreConfigMap(_ Resource) error {
	cmd := exec.Command("kubectl", "apply", "-f", "testfiles/common-templates/csm-images-baseline.yaml") // #nosec G204
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to restore baseline configmap csm-images: %v", err)
	}

	return nil
}

func (step *Step) deleteConfigMap(_ Resource) error {
	cmd := exec.Command("kubectl", "delete", "-f", "testfiles/common-templates/csm-images-baseline.yaml", "--ignore-not-found") // #nosec G204
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to delete baseline configmap csm-images: %v", err)
	}

	return nil
}

// validateEnvInDriverPod validates environment variables in the generated DaemonSet/Deployment
func (step *Step) validateEnvInDriverPod(res Resource, podType, containerName, envName, expectedValue, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	var envVars []corev1.EnvVar

	if podType == "node" {
		// Get DaemonSet using clientSet
		daemonSet, err := step.clientSet.AppsV1().DaemonSets(cr.Namespace).Get(context.TODO(), cr.Name+"-node", metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get driver DaemonSet: %v", err)
		}
		// Find the specified container
		for _, container := range daemonSet.Spec.Template.Spec.Containers {
			if container.Name == containerName {
				envVars = container.Env
				break
			}
		}
	} else if podType == "controller" {
		// Get Deployment using clientSet
		deployment, err := step.clientSet.AppsV1().Deployments(cr.Namespace).Get(context.TODO(), cr.Name+"-controller", metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get driver Deployment: %v", err)
		}
		// Find the specified container
		for _, container := range deployment.Spec.Template.Spec.Containers {
			if container.Name == containerName {
				envVars = container.Env
				break
			}
		}
	} else {
		return fmt.Errorf("invalid podType: %s (must be 'node' or 'controller')", podType)
	}

	// Check if container was found
	if len(envVars) == 0 {
		return fmt.Errorf("container '%s' not found in %s pod", containerName, podType)
	}

	// Find the environment variable
	var foundValue string
	found := false
	for _, env := range envVars {
		if env.Name == envName {
			foundValue = env.Value
			found = true
			break
		}
	}

	// Handle default values
	if !found && expectedValue != "" {
		return fmt.Errorf("environment variable %s not found in %s pod", envName, podType)
	} else if found && foundValue != expectedValue {
		return fmt.Errorf("environment variable %s has value [%s] but expected [%s] in %s pod", envName, foundValue, expectedValue, podType)
	}

	return nil
}

// validateEnvInCSMCR validates environment variables in the CSM CustomResource in the cluster
func (step *Step) validateEnvInCSMCR(res Resource, section, envName, expectedValue, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	// Get the CSM CR from the cluster
	csm := &csmv1.ContainerStorageModule{}
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, csm); err != nil {
		return fmt.Errorf("failed to get CSM CR %s/%s from cluster: %v", cr.Namespace, cr.Name, err)
	}

	var envVars []corev1.EnvVar

	// Navigate to the specified section
	switch section {
	case "common":
		if csm.Spec.Driver.Common.Envs != nil {
			envVars = csm.Spec.Driver.Common.Envs
		}
	case "node":
		if csm.Spec.Driver.Node.Envs != nil {
			envVars = csm.Spec.Driver.Node.Envs
		}
	case "controller":
		if csm.Spec.Driver.Controller.Envs != nil {
			envVars = csm.Spec.Driver.Controller.Envs
		}
	default:
		return fmt.Errorf("unsupported section: %s", section)
	}

	// Find the environment variable
	var foundValue string
	found := false
	for _, env := range envVars {
		if env.Name == envName {
			foundValue = env.Value
			found = true
			break
		}
	}

	if !found && expectedValue != "" {
		return fmt.Errorf("environment variable %s not found in CSM CR %s section", envName, section)
	} else if found && foundValue != expectedValue {
		return fmt.Errorf("environment variable %s has value [%s] but expected [%s] in CSM CR %s section", envName, foundValue, expectedValue, section)
	}

	return nil
}

// setEnvInSpec modifies environment variables in a CR file and writes to temp directory
func (step *Step) setEnvInSpec(res Resource, section, envName, envValue, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	// Read the original CR file
	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	// Unmarshal into CSM struct
	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal([]byte(os.ExpandEnv(string(crBuff))), &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	// Get the appropriate envs slice based on section
	var envs *[]corev1.EnvVar
	switch section {
	case "common":
		if customResource.Spec.Driver.Common == nil {
			customResource.Spec.Driver.Common = &csmv1.ContainerTemplate{}
		}
		envs = &customResource.Spec.Driver.Common.Envs
	case "node":
		if customResource.Spec.Driver.Node == nil {
			customResource.Spec.Driver.Node = &csmv1.ContainerTemplate{}
		}
		envs = &customResource.Spec.Driver.Node.Envs
	case "controller":
		if customResource.Spec.Driver.Controller == nil {
			customResource.Spec.Driver.Controller = &csmv1.ContainerTemplate{}
		}
		envs = &customResource.Spec.Driver.Controller.Envs
	default:
		return fmt.Errorf("unsupported env section: %s", section)
	}

	// Find and update the env var, or add it if not found
	found := false
	for i, env := range *envs {
		if env.Name == envName {
			(*envs)[i].Value = envValue
			found = true
			break
		}
	}

	if !found {
		// Add new env var
		newEnv := corev1.EnvVar{
			Name:  envName,
			Value: envValue,
		}
		*envs = append(*envs, newEnv)
	}

	// Write to temporary file
	modifiedYAML, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	// Update the scenario path to use the temp file
	res.Scenario.Paths[crNum-1] = tempPath

	return nil
}

// setForceRemoveDriverInSpec modifies forceRemoveDriver in a CR file and writes to temp directory.
// Use value "true"/"false" to set the field, or "remove" to remove it entirely.
func (step *Step) setForceRemoveDriverInSpec(res Resource, value string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(crBuff, &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	switch strings.ToLower(value) {
	case "true":
		trueBool := true
		customResource.Spec.Driver.ForceRemoveDriver = &trueBool
	case "false":
		falseBool := false
		customResource.Spec.Driver.ForceRemoveDriver = &falseBool
	case "remove":
		customResource.Spec.Driver.ForceRemoveDriver = nil
	default:
		return fmt.Errorf("unsupported forceRemoveDriver value: %s (use true, false, or remove)", value)
	}

	modifiedYAML, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	res.Scenario.Paths[crNum-1] = tempPath
	return nil
}

// removeFieldFromSpec removes any field from a CR file and writes to temp directory.
// Supports field paths like "version", "driver.forceRemoveDriver", "driver.configVersion", etc.
func (step *Step) removeFieldFromSpec(res Resource, fieldPath string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(crBuff, &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	// Convert to map to manipulate fields dynamically
	crMap := make(map[string]interface{})
	err = yaml.Unmarshal(crBuff, &crMap)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM to map: %v", err)
	}

	// Remove the field using dot notation
	fields := strings.Split(fieldPath, ".")
	removeFieldFromMap(crMap, fields)

	modifiedYAML, err := yaml.Marshal(crMap)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	res.Scenario.Paths[crNum-1] = tempPath
	return nil
}

// removeFieldFromMap recursively removes a field from a nested map using dot notation
func removeFieldFromMap(m map[string]interface{}, fields []string) {
	if len(fields) == 0 {
		return
	}

	field := fields[0]
	if len(fields) == 1 {
		// Remove the final field
		delete(m, field)
		return
	}

	// Recurse into nested map
	if nextMap, ok := m[field].(map[string]interface{}); ok {
		removeFieldFromMap(nextMap, fields[1:])
		// If the nested map becomes empty, remove it too
		if len(nextMap) == 0 {
			delete(m, field)
		}
	}
}

// enableModuleInSpec enables a module in a CR file before applying it.
func (step *Step) enableModuleInSpec(res Resource, module string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(crBuff, &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	found := false
	for i, m := range customResource.Spec.Modules {
		if m.Name == csmv1.ModuleType(module) {
			customResource.Spec.Modules[i].Enabled = true
			// for observability, enable all components
			if m.Name == csmv1.Observability {
				for j := range m.Components {
					customResource.Spec.Modules[i].Components[j].Enabled = pointer.Bool(true)
				}
			}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("module %s not found in CR spec", module)
	}

	modifiedYAML, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	res.Scenario.Paths[crNum-1] = tempPath
	return nil
}

// setEnvFromEnvVarInSpec sets an environment variable in the CR spec from a system environment variable.
func (step *Step) setEnvFromEnvVarInSpec(res Resource, section, envName, envVarName, crNumStr string) error {
	envValue := os.Getenv(envVarName)
	return step.setEnvInSpec(res, section, envName, envValue, crNumStr)
}

// setDriverImage sets spec.driver.common.image in a CR file before applying it.
func (step *Step) setDriverImage(res Resource, image string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(crBuff, &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	if customResource.Spec.Driver.Common == nil {
		customResource.Spec.Driver.Common = &csmv1.ContainerTemplate{}
	}
	customResource.Spec.Driver.Common.Image = csmv1.ImageType(image)

	modifiedYAML, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	res.Scenario.Paths[crNum-1] = tempPath
	return nil
}

// setReplicasInSpec sets spec.driver.replicas in a CR file before applying it.
func (step *Step) setReplicasInSpec(res Resource, replicasStr string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	replicas, err := strconv.ParseInt(replicasStr, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid replicas value %s: %v", replicasStr, err)
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(crBuff, &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	customResource.Spec.Driver.Replicas = int32(replicas)

	modifiedYAML, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	res.Scenario.Paths[crNum-1] = tempPath
	return nil
}

// setMetadataInSpec sets metadata.name and metadata.namespace in a CR file before applying it.
func (step *Step) setMetadataInSpec(res Resource, name, namespace, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(crBuff, &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	customResource.Name = name
	customResource.Namespace = namespace

	modifiedYAML, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	res.Scenario.Paths[crNum-1] = tempPath
	return nil
}

// setImagePullPolicyInSpec sets spec.driver.common.imagePullPolicy in a CR file before applying it.
func (step *Step) setImagePullPolicyInSpec(res Resource, policy, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(crBuff, &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	if customResource.Spec.Driver.Common == nil {
		customResource.Spec.Driver.Common = &csmv1.ContainerTemplate{}
	}
	customResource.Spec.Driver.Common.ImagePullPolicy = corev1.PullPolicy(policy)

	modifiedYAML, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	res.Scenario.Paths[crNum-1] = tempPath
	return nil
}

// setModuleComponentImageInSpec sets a component's image within a module in a CR file before applying it.
func (step *Step) setModuleComponentImageInSpec(res Resource, module, component, image, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(crBuff, &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	found := false
	for i, m := range customResource.Spec.Modules {
		if m.Name == csmv1.ModuleType(module) {
			for j, c := range m.Components {
				if c.Name == component {
					customResource.Spec.Modules[i].Components[j].Image = csmv1.ImageType(image)
					found = true
					break
				}
			}
			break
		}
	}

	if !found {
		return fmt.Errorf("component %s not found in module %s", component, module)
	}

	modifiedYAML, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	res.Scenario.Paths[crNum-1] = tempPath
	return nil
}

// setModuleComponentEnvInSpec sets an environment variable on a component within a module in a CR file before applying it.
func (step *Step) setModuleComponentEnvInSpec(res Resource, module, component, envName, envValue, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(crBuff, &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	moduleFound := false
	for i, m := range customResource.Spec.Modules {
		if m.Name == csmv1.ModuleType(module) {
			for j, c := range m.Components {
				if c.Name == component {
					envFound := false
					for k, env := range c.Envs {
						if env.Name == envName {
							customResource.Spec.Modules[i].Components[j].Envs[k].Value = envValue
							envFound = true
							break
						}
					}
					if !envFound {
						customResource.Spec.Modules[i].Components[j].Envs = append(
							customResource.Spec.Modules[i].Components[j].Envs,
							corev1.EnvVar{Name: envName, Value: envValue},
						)
					}
					moduleFound = true
					break
				}
			}
			break
		}
	}

	if !moduleFound {
		return fmt.Errorf("component %s not found in module %s", component, module)
	}

	modifiedYAML, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	res.Scenario.Paths[crNum-1] = tempPath
	return nil
}

// setDriverConfigVersionInSpec sets the driver configVersion in a CR file before applying it.
// configVersion may be a literal version string (e.g. "v2.16.0") or one of
// the keywords "n-1" / "n-2", which are resolved dynamically from
// csm-releases.yaml based on the driver type in the CR.
func (step *Step) setDriverConfigVersionInSpec(res Resource, configVersion, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(crBuff, &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	resolved, err := resolveNMinusConfigVersion(configVersion, string(customResource.Spec.Driver.CSIDriverType))
	if err != nil {
		return err
	}
	configVersion = resolved

	customResource.Spec.Driver.ConfigVersion = configVersion

	modifiedYAML, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	res.Scenario.Paths[crNum-1] = tempPath
	return nil
}

// setModuleConfigVersionInSpec sets the configVersion of a module in a CR file before applying it.
// configVersion may be a literal version string or one of the keywords "n-1" / "n-2",
// which are resolved dynamically using the version package. Resolution uses the
// driver type from the CR and csm-releases.yaml to find the correct module version.
func (step *Step) setModuleConfigVersionInSpec(res Resource, module, configVersion, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(crBuff, &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	resolved, err := resolveNMinusModuleVersion(configVersion, module, string(customResource.Spec.Driver.CSIDriverType))
	if err != nil {
		return err
	}
	configVersion = resolved

	found := false
	for i, m := range customResource.Spec.Modules {
		if m.Name == csmv1.ModuleType(module) {
			customResource.Spec.Modules[i].ConfigVersion = configVersion
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("module %s not found in CR", module)
	}

	modifiedYAML, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	res.Scenario.Paths[crNum-1] = tempPath
	return nil
}

// setModuleComponentEnabledInSpec sets a component's enabled flag within a module in a CR file before applying it.
func (step *Step) setModuleComponentEnabledInSpec(res Resource, module, component, enabledStr, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(crBuff, &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	enabled := strings.ToLower(enabledStr) == "true"
	found := false
	for i, m := range customResource.Spec.Modules {
		if m.Name == csmv1.ModuleType(module) {
			for j, c := range m.Components {
				if c.Name == component {
					customResource.Spec.Modules[i].Components[j].Enabled = pointer.Bool(enabled)
					found = true
					break
				}
			}
			break
		}
	}

	if !found {
		return fmt.Errorf("component %s not found in module %s", component, module)
	}

	modifiedYAML, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	res.Scenario.Paths[crNum-1] = tempPath
	return nil
}

// setSpecVersionInSpec sets spec.version on a CR file before applying it.
// version may be a literal version string (e.g. "v1.16.3") or one of the
// keywords "n" / "n-1" / "n-2", which are resolved dynamically from
// csm-releases.yaml.
func (step *Step) setSpecVersionInSpec(res Resource, ver, crNumStr string) error {
	resolved, err := resolveNMinusCSMVersion(ver)
	if err != nil {
		return err
	}
	ver = resolved

	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(crBuff, &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	customResource.Spec.Version = ver

	modifiedYAML, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	res.Scenario.Paths[crNum-1] = tempPath
	return nil
}

// setCustomRegistryInSpec sets spec.customRegistry on a CR file before applying it.
func (step *Step) setCustomRegistryInSpec(res Resource, registry, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(crBuff, &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	customResource.Spec.CustomRegistry = registry

	modifiedYAML, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	res.Scenario.Paths[crNum-1] = tempPath
	return nil
}

// resolveNMinusCSMVersion resolves "n" / "n-1" / "n-2" keywords to actual CSM
// operator versions using the version package. Literal version strings are
// returned as-is.
func resolveNMinusCSMVersion(ver string) (string, error) {
	idx, ok := nMinusIndex(ver)
	if !ok {
		return ver, nil // literal version, no resolution needed
	}
	info := version.GetInfo()
	if info == nil {
		return "", fmt.Errorf("version.Init has not been called; cannot resolve %q", ver)
	}
	resolved := info.CSMVersion(idx)
	if resolved == "" {
		return "", fmt.Errorf("no CSM version available at index %d for keyword %q", idx, ver)
	}
	fmt.Printf("             Resolved spec version keyword %q → %s\n", ver, resolved)
	return resolved, nil
}

// resolveNMinusConfigVersion resolves "n" / "n-1" / "n-2" keywords to actual driver
// configVersion strings using the version package. Literal version strings
// are returned as-is.
func resolveNMinusConfigVersion(configVersion, driverType string) (string, error) {
	idx, ok := nMinusIndex(configVersion)
	if !ok {
		return configVersion, nil // literal version, no resolution needed
	}
	info := version.GetInfo()
	if info == nil {
		return "", fmt.Errorf("version.Init has not been called; cannot resolve %q", configVersion)
	}
	entity := driverTypeToMappingKey(driverType)
	resolved := info.ConfigVersionAtIndex(entity, idx)
	if resolved == "" {
		return "", fmt.Errorf("no config version for driver %q (%s) at index %d for keyword %q",
			driverType, entity, idx, configVersion)
	}
	fmt.Printf("             Resolved configVersion keyword %q for %s → %s\n", configVersion, entity, resolved)
	return resolved, nil
}

// nMinusIndex maps version keywords to version index constants:
// "n" → Latest, "n-1" → NMinusOne, "n-2" → NMinusTwo.
// Returns false if the string is not a recognised keyword.
func nMinusIndex(s string) (int, bool) {
	switch s {
	case "n":
		return version.Latest, true
	case "n-1":
		return version.NMinusOne, true
	case "n-2":
		return version.NMinusTwo, true
	default:
		return 0, false
	}
}

// resolveNMinusModuleVersion resolves "n" / "n-1" / "n-2" keywords to actual module
// configVersion strings using the version package. It chains through
// csm-releases.yaml: CSM version → driver configVersion → module configVersion.
// Literal version strings are returned as-is.
func resolveNMinusModuleVersion(configVersion, moduleName, driverType string) (string, error) {
	idx, ok := nMinusIndex(configVersion)
	if !ok {
		return configVersion, nil // literal version, no resolution needed
	}
	info := version.GetInfo()
	if info == nil {
		return "", fmt.Errorf("version.Init has not been called; cannot resolve %q", configVersion)
	}
	entity := driverTypeToMappingKey(driverType)
	resolved := info.ModuleVersionAtIndex(entity, moduleName, idx)
	if resolved == "" {
		return "", fmt.Errorf("no module version for %s/%s (driver %q) at index %d for keyword %q",
			entity, moduleName, driverType, idx, configVersion)
	}
	fmt.Printf("             Resolved module configVersion keyword %q for %s/%s → %s\n",
		configVersion, entity, moduleName, resolved)
	return resolved, nil
}

// driverTypeToMappingKey translates a CSIDriverType value (e.g. "isilon",
// "vxflexos") to the entity key used in csm-releases.yaml.
func driverTypeToMappingKey(driverType string) string {
	switch driverType {
	case "isilon":
		return "powerscale"
	case "vxflexos":
		return "powerflex"
	default:
		return driverType
	}
}

// validateCustomResourceFailed checks that the CR status is Failed.
func (step *Step) validateCustomResourceFailed(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	// Use clientset to get uncached CR status
	var found csmv1.ContainerStorageModule
	err = step.clientSet.RESTClient().Get().
		AbsPath("apis", "storage.dell.com", "v1").
		Namespace(cr.Namespace).
		Resource("containerstoragemodules").
		Name(cr.Name).
		Do(context.TODO()).
		Into(&found)
	if err != nil {
		return err
	}
	if found.Status.State != constants.Failed {
		return fmt.Errorf("expected custom resource status to be %s. Got: %s", constants.Failed, found.Status.State)
	}
	return nil
}

// deleteAllCSMFromNamespace deletes all ContainerStorageModule resources in the specified namespace.
func (step *Step) deleteAllCSMFromNamespace(_ Resource, namespace string) error {
	cmd := exec.Command("kubectl", "delete", "csm", "-n", namespace, "--all", "--ignore-not-found") // #nosec G204
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete all CSM from namespace %s: %v\nErrMessage:\n%s", namespace, err, string(b))
	}
	return nil
}

// setOperatorFailureGracePeriod patches the operator deployment to set a custom
// FAILURE_GRACE_PERIOD env var and waits for the rollout to complete.
func (step *Step) setOperatorFailureGracePeriod(res Resource, duration string) error {
	ns := os.Getenv("OPERATOR_NAMESPACE")
	if ns == "" {
		ns = "dell-csm-operator"
	}
	deployName := "dell-csm-operator-controller-manager"

	dep, err := step.clientSet.AppsV1().Deployments(ns).Get(context.TODO(), deployName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get operator deployment: %v", err)
	}

	// Update or add the FAILURE_GRACE_PERIOD env var on the first container
	found := false
	for i, container := range dep.Spec.Template.Spec.Containers {
		if container.Name == "manager" {
			for j, env := range container.Env {
				if env.Name == "FAILURE_GRACE_PERIOD" {
					dep.Spec.Template.Spec.Containers[i].Env[j].Value = duration
					found = true
					break
				}
			}
			if !found {
				dep.Spec.Template.Spec.Containers[i].Env = append(
					dep.Spec.Template.Spec.Containers[i].Env,
					corev1.EnvVar{Name: "FAILURE_GRACE_PERIOD", Value: duration},
				)
			}
			break
		}
	}

	_, err = step.clientSet.AppsV1().Deployments(ns).Update(context.TODO(), dep, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update operator deployment: %v", err)
	}

	// Wait for the operator to roll out (new pod ready)
	fmt.Printf("             Waiting for operator rollout with FAILURE_GRACE_PERIOD=%s\n", duration)
	return waitForDeploymentReady(step.clientSet, ns, deployName, 3*time.Minute)
}

// setConfigMapImage patches the csm-images ConfigMap to override a specific image key
// with the given value. Use "Restore ConfigMap" step to undo the change after the test.
func (step *Step) setConfigMapImage(res Resource, imageKey string, imageValue string) error {
	ns := os.Getenv("OPERATOR_NAMESPACE")
	if ns == "" {
		ns = "dell-csm-operator"
	}

	cm, err := step.clientSet.CoreV1().ConfigMaps(ns).Get(context.TODO(), "csm-images", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get csm-images ConfigMap: %v", err)
	}

	versionsYAML, ok := cm.Data["versions.yaml"]
	if !ok {
		return fmt.Errorf("csm-images ConfigMap missing versions.yaml key")
	}

	// Replace the image value for the given key in the YAML string.
	// The format is "        <key>: <old-image>" — find and replace the line.
	lines := strings.Split(versionsYAML, "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, imageKey+":") {
			// Preserve the leading whitespace
			indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
			lines[i] = indent + imageKey + ": " + imageValue
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("image key %q not found in csm-images ConfigMap versions.yaml", imageKey)
	}

	cm.Data["versions.yaml"] = strings.Join(lines, "\n")
	_, err = step.clientSet.CoreV1().ConfigMaps(ns).Update(context.TODO(), cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update csm-images ConfigMap: %v", err)
	}
	fmt.Printf("             Updated csm-images ConfigMap: %s → %s\n", imageKey, imageValue)
	return nil
}

func (step *Step) validateObsMetricsService(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	svcName := "karavi-metrics-powerscale"
	cmd := exec.Command("kubectl", "get", "svc", svcName, "-n", cr.Namespace) // #nosec G204
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("observability metrics service %s not found in %s: %v, output: %s", svcName, cr.Namespace, err, string(output))
	}
	return nil
}

func (step *Step) validateObsMetricsServiceMonitor(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	smName := "karavi-metrics-powerscale-obs-monitor"
	cmd := exec.Command("kubectl", "get", "servicemonitor", smName, "-n", cr.Namespace) // #nosec G204
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("observability metrics ServiceMonitor %s not found in %s: %v, output: %s", smName, cr.Namespace, err, string(output))
	}
	return nil
}

func (step *Step) validateObsMetricsPort(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	listCmd := exec.Command("kubectl", "get", "pods", "-n", cr.Namespace, // #nosec G204
		"--field-selector=status.phase=Running",
		"-o", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`)
	listOut, err := listCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list pods in %s: %v", cr.Namespace, err)
	}

	podName := ""
	for _, name := range strings.Split(strings.TrimSpace(string(listOut)), "\n") {
		if strings.HasPrefix(name, "karavi-metrics-powerscale-") {
			podName = name
			break
		}
	}
	if podName == "" {
		return fmt.Errorf("no running karavi-metrics-powerscale pod found in namespace %s", cr.Namespace)
	}

	portCmd := exec.Command("kubectl", "get", "pod", podName, "-n", cr.Namespace, // #nosec G204
		"-o", `jsonpath={.spec.containers[?(@.name=="karavi-metrics-powerscale")].ports[?(@.name=="obs-metrics")].containerPort}`)
	portOut, err := portCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to query port for pod %s: %v, output: %s", podName, err, string(portOut))
	}
	if strings.TrimSpace(string(portOut)) == "" {
		return fmt.Errorf("karavi-metrics-powerscale container in pod %s does not expose obs-metrics port", podName)
	}
	return nil
}

func (step *Step) validateObsMetricsConfigMap(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedPort := int32(8443)
	for _, module := range cr.Spec.Modules {
		if string(module.Name) == "observability" {
			if module.Metrics != nil && module.Metrics.Port > 0 {
				expectedPort = module.Metrics.Port
			}
			break
		}
	}

	cmd := exec.Command("kubectl", "get", "configmap", "karavi-metrics-powerscale-configmap", // #nosec G204
		"-n", cr.Namespace, "-o", "yaml")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get karavi-metrics-powerscale-configmap in %s: %v, output: %s", cr.Namespace, err, string(out))
	}

	configContent := string(out)
	if !strings.Contains(configContent, "X_CSI_METRICS_ENABLED: true") {
		return fmt.Errorf("karavi-metrics-powerscale-configmap missing X_CSI_METRICS_ENABLED: true")
	}
	expectedPortLine := fmt.Sprintf("X_CSI_METRICS_PORT: %d", expectedPort)
	if !strings.Contains(configContent, expectedPortLine) {
		return fmt.Errorf("karavi-metrics-powerscale-configmap missing %q", expectedPortLine)
	}
	return nil
}

func (step *Step) validateObsMetricsServiceMonitorHTTPS(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	smName := "karavi-metrics-powerscale-obs-monitor"
	cmd := exec.Command("kubectl", "get", "servicemonitor", smName, "-n", cr.Namespace, // #nosec G204
		"-o", `jsonpath={.spec.endpoints[0].scheme}`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get ServiceMonitor %s in %s: %v, output: %s", smName, cr.Namespace, err, string(out))
	}
	if strings.TrimSpace(string(out)) != "https" {
		return fmt.Errorf("ServiceMonitor %s does not use HTTPS scheme, got: %q", smName, strings.TrimSpace(string(out)))
	}
	return nil
}

func (step *Step) validateAuthorizationMetricsService(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedPort := int32(2112)
	for _, module := range cr.Spec.Modules {
		if module.Name == csmv1.AuthorizationServer {
			if module.Metrics != nil && module.Metrics.Port > 0 {
				expectedPort = module.Metrics.Port
			}
			break
		}
	}

	cmd := exec.Command("kubectl", "get", "svc", "proxy-server-metrics", "-n", cr.Namespace) // #nosec G204
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("authorization metrics service proxy-server-metrics not found in %s: %v, output: %s", cr.Namespace, err, string(output))
	}

	cmd = exec.Command("kubectl", "get", "svc", "proxy-server-metrics", "-n", cr.Namespace, "-o", "jsonpath={.spec.ports[?(@.name==\"metrics\")].port}") // #nosec G204
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get authorization metrics service port: %v, output: %s", err, string(output))
	}
	if strings.TrimSpace(string(output)) != fmt.Sprintf("%d", expectedPort) {
		return fmt.Errorf("authorization metrics service port is %s, expected %d", strings.TrimSpace(string(output)), expectedPort)
	}

	cmd = exec.Command("kubectl", "get", "svc", "proxy-server-metrics", "-n", cr.Namespace, "-o", "jsonpath={.spec.selector.app}") // #nosec G204
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get authorization metrics service selector: %v, output: %s", err, string(output))
	}
	if strings.TrimSpace(string(output)) != "proxy-server" {
		return fmt.Errorf("authorization metrics service selector is %s, expected proxy-server", strings.TrimSpace(string(output)))
	}

	return nil
}

func (step *Step) validateAuthorizationMetricsServiceMonitor(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	cmd := exec.Command("kubectl", "get", "servicemonitor", "proxy-server-metrics-monitor", "-n", cr.Namespace) // #nosec G204
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("authorization metrics ServiceMonitor proxy-server-metrics-monitor not found in %s: %v, output: %s", cr.Namespace, err, string(output))
	}

	cmd = exec.Command("kubectl", "get", "servicemonitor", "proxy-server-metrics-monitor", "-n", cr.Namespace, "-o", "jsonpath={.spec.selector.matchLabels.app}") // #nosec G204
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get authorization metrics ServiceMonitor selector: %v, output: %s", err, string(output))
	}
	if strings.TrimSpace(string(output)) != "proxy-server" {
		return fmt.Errorf("authorization metrics ServiceMonitor selector is %s, expected proxy-server", strings.TrimSpace(string(output)))
	}

	cmd = exec.Command("kubectl", "get", "servicemonitor", "proxy-server-metrics-monitor", "-n", cr.Namespace, "-o", "jsonpath={.spec.endpoints[0].port}") // #nosec G204
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get authorization metrics ServiceMonitor port: %v, output: %s", err, string(output))
	}
	if strings.TrimSpace(string(output)) != "metrics" {
		return fmt.Errorf("authorization metrics ServiceMonitor port is %s, expected metrics", strings.TrimSpace(string(output)))
	}

	return nil
}

func (step *Step) validateAuthorizationMetricsPort(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedPort := int32(2112)
	for _, module := range cr.Spec.Modules {
		if module.Name == csmv1.AuthorizationServer {
			if module.Metrics != nil && module.Metrics.Port > 0 {
				expectedPort = module.Metrics.Port
			}
			break
		}
	}

	cmd := exec.Command("kubectl", "get", "deployment", "proxy-server", "-n", cr.Namespace, "-o", "jsonpath={.spec.template.spec.containers[?(@.name==\"proxy-server\")].ports[?(@.name==\"metrics\")].containerPort}") // #nosec G204
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get authorization proxy-server metrics port: %v, output: %s", err, string(output))
	}
	if strings.TrimSpace(string(output)) != fmt.Sprintf("%d", expectedPort) {
		return fmt.Errorf("authorization proxy-server metrics port is %s, expected %d", strings.TrimSpace(string(output)), expectedPort)
	}

	return nil
}

func (step *Step) validateAuthorizationMetricsServiceMonitorHTTPS(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	cmd := exec.Command("kubectl", "get", "servicemonitor", "proxy-server-metrics-monitor", "-n", cr.Namespace, "-o", "jsonpath={.spec.endpoints[0].scheme}") // #nosec G204
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get authorization metrics ServiceMonitor scheme: %v, output: %s", err, string(output))
	}
	if strings.TrimSpace(string(output)) != "https" {
		return fmt.Errorf("authorization metrics ServiceMonitor does not use HTTPS scheme, got: %q", strings.TrimSpace(string(output)))
	}

	return nil
}

func (step *Step) validatePowerStoreObsMetricsService(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	svcName := "karavi-metrics-powerstore"
	cmd := exec.Command("kubectl", "get", "svc", svcName, "-n", cr.Namespace) // #nosec G204
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("observability metrics service %s not found in %s: %v, output: %s", svcName, cr.Namespace, err, string(output))
	}
	return nil
}

func (step *Step) validatePowerStoreObsMetricsServiceMonitor(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	smName := "karavi-metrics-powerstore-obs-monitor"
	cmd := exec.Command("kubectl", "get", "servicemonitor", smName, "-n", cr.Namespace) // #nosec G204
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("observability metrics ServiceMonitor %s not found in %s: %v, output: %s", smName, cr.Namespace, err, string(output))
	}
	return nil
}

func (step *Step) validatePowerStoreObsMetricsPort(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	listCmd := exec.Command("kubectl", "get", "pods", "-n", cr.Namespace, // #nosec G204
		"--field-selector=status.phase=Running",
		"-o", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`)
	listOut, err := listCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list pods in %s: %v", cr.Namespace, err)
	}

	podName := ""
	for _, name := range strings.Split(strings.TrimSpace(string(listOut)), "\n") {
		if strings.HasPrefix(name, "karavi-metrics-powerstore-") {
			podName = name
			break
		}
	}
	if podName == "" {
		return fmt.Errorf("no running karavi-metrics-powerstore pod found in namespace %s", cr.Namespace)
	}

	portCmd := exec.Command("kubectl", "get", "pod", podName, "-n", cr.Namespace, // #nosec G204
		"-o", `jsonpath={.spec.containers[?(@.name=="karavi-metrics-powerstore")].ports[?(@.name=="obs-metrics")].containerPort}`)
	portOut, err := portCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to query port for pod %s: %v, output: %s", podName, err, string(portOut))
	}
	if strings.TrimSpace(string(portOut)) == "" {
		return fmt.Errorf("karavi-metrics-powerstore container in pod %s does not expose obs-metrics port", podName)
	}
	return nil
}

func (step *Step) validatePowerStoreObsMetricsConfigMap(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedPort := int32(8443)
	for _, module := range cr.Spec.Modules {
		if string(module.Name) == "observability" {
			if module.Metrics != nil && module.Metrics.Port > 0 {
				expectedPort = module.Metrics.Port
			}
			break
		}
	}

	cmd := exec.Command("kubectl", "get", "configmap", "karavi-metrics-powerstore-configmap", // #nosec G204
		"-n", cr.Namespace, "-o", "yaml")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get karavi-metrics-powerstore-configmap in %s: %v, output: %s", cr.Namespace, err, string(out))
	}

	configContent := string(out)
	if !strings.Contains(configContent, "X_CSI_METRICS_ENABLED: true") {
		return fmt.Errorf("karavi-metrics-powerstore-configmap missing X_CSI_METRICS_ENABLED: true")
	}
	expectedPortLine := fmt.Sprintf("X_CSI_METRICS_PORT: %d", expectedPort)
	if !strings.Contains(configContent, expectedPortLine) {
		return fmt.Errorf("karavi-metrics-powerstore-configmap missing %q", expectedPortLine)
	}
	return nil
}

func (step *Step) validatePowerStoreObsMetricsServiceMonitorHTTPS(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	smName := "karavi-metrics-powerstore-obs-monitor"
	cmd := exec.Command("kubectl", "get", "servicemonitor", smName, "-n", cr.Namespace, // #nosec G204
		"-o", `jsonpath={.spec.endpoints[0].scheme}`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get ServiceMonitor %s in %s: %v, output: %s", smName, cr.Namespace, err, string(out))
	}
	if strings.TrimSpace(string(out)) != "https" {
		return fmt.Errorf("ServiceMonitor %s does not use HTTPS scheme, got: %q", smName, strings.TrimSpace(string(out)))
	}
	return nil
}

func (step *Step) validatePowerMaxObsMetricsService(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	svcName := "karavi-metrics-powermax"
	cmd := exec.Command("kubectl", "get", "svc", svcName, "-n", cr.Namespace) // #nosec G204
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("observability metrics service %s not found in %s: %v, output: %s", svcName, cr.Namespace, err, string(output))
	}
	return nil
}

func (step *Step) validatePowerMaxObsMetricsServiceMonitor(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	smName := "karavi-metrics-powermax-obs-monitor"
	cmd := exec.Command("kubectl", "get", "servicemonitor", smName, "-n", cr.Namespace) // #nosec G204
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("observability metrics ServiceMonitor %s not found in %s: %v, output: %s", smName, cr.Namespace, err, string(output))
	}
	return nil
}

func (step *Step) validatePowerMaxObsMetricsPort(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	listCmd := exec.Command("kubectl", "get", "pods", "-n", cr.Namespace, // #nosec G204
		"--field-selector=status.phase=Running",
		"-o", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`)
	listOut, err := listCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list pods in %s: %v", cr.Namespace, err)
	}

	podName := ""
	for _, name := range strings.Split(strings.TrimSpace(string(listOut)), "\n") {
		if strings.HasPrefix(name, "karavi-metrics-powermax-") {
			podName = name
			break
		}
	}
	if podName == "" {
		return fmt.Errorf("no running karavi-metrics-powermax pod found in namespace %s", cr.Namespace)
	}

	portCmd := exec.Command("kubectl", "get", "pod", podName, "-n", cr.Namespace, // #nosec G204
		"-o", `jsonpath={.spec.containers[?(@.name=="karavi-metrics-powermax")].ports[?(@.name=="obs-metrics")].containerPort}`)
	portOut, err := portCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to query port for pod %s: %v, output: %s", podName, err, string(portOut))
	}
	if strings.TrimSpace(string(portOut)) == "" {
		return fmt.Errorf("karavi-metrics-powermax container in pod %s does not expose obs-metrics port", podName)
	}
	return nil
}

func (step *Step) validatePowerMaxObsMetricsConfigMap(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedPort := int32(8443)
	for _, module := range cr.Spec.Modules {
		if string(module.Name) == "observability" {
			if module.Metrics != nil && module.Metrics.Port > 0 {
				expectedPort = module.Metrics.Port
			}
			break
		}
	}

	cmd := exec.Command("kubectl", "get", "configmap", "karavi-metrics-powermax-configmap", // #nosec G204
		"-n", cr.Namespace, "-o", "yaml")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get karavi-metrics-powermax-configmap in %s: %v, output: %s", cr.Namespace, err, string(out))
	}

	configContent := string(out)
	if !strings.Contains(configContent, "X_CSI_METRICS_ENABLED: true") {
		return fmt.Errorf("karavi-metrics-powermax-configmap missing X_CSI_METRICS_ENABLED: true")
	}
	expectedPortLine := fmt.Sprintf("X_CSI_METRICS_PORT: %d", expectedPort)
	if !strings.Contains(configContent, expectedPortLine) {
		return fmt.Errorf("karavi-metrics-powermax-configmap missing %q", expectedPortLine)
	}
	return nil
}

func (step *Step) validatePowerMaxObsMetricsServiceMonitorHTTPS(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	smName := "karavi-metrics-powermax-obs-monitor"
	cmd := exec.Command("kubectl", "get", "servicemonitor", smName, "-n", cr.Namespace, // #nosec G204
		"-o", `jsonpath={.spec.endpoints[0].scheme}`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get ServiceMonitor %s in %s: %v, output: %s", smName, cr.Namespace, err, string(out))
	}
	if strings.TrimSpace(string(out)) != "https" {
		return fmt.Errorf("ServiceMonitor %s does not use HTTPS scheme, got: %q", smName, strings.TrimSpace(string(out)))
	}
	return nil
}

// setUpPowerFlexObsMetricsTLSSecret creates the TLS secret for PowerFlex observability self-metrics.
func (step *Step) setUpPowerFlexObsMetricsTLSSecret(_ Resource, namespace string) error {
	const secretName = "powerflex-observability-metrics-tls" // #nosec G101 -- secret name, not a credential

	cmd := exec.Command("kubectl", "get", "secret", secretName, "-n", namespace) // #nosec G204
	if err := cmd.Run(); err == nil {
		fmt.Printf("%s secret already exists, skipping creation.\n", secretName)
		return nil
	}

	keyPath := "temp/powerflex-observability-metrics-tls.key"
	crtPath := "temp/powerflex-observability-metrics-tls.crt"

	cmd = exec.Command("openssl", "genrsa", "-out", keyPath, "2048") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS key: %v", err)
	}

	cmd = exec.Command("openssl", "req", "-new", "-x509", "-sha256", "-key", keyPath, "-out", crtPath, "-days", "3650", "-subj", "/CN=powerflex-observability-metrics") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %v", err)
	}

	cmd = exec.Command("kubectl", "create", "secret", "-n", namespace, "tls", secretName, "--cert="+crtPath, "--key="+keyPath) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create %s secret: %v", secretName, err)
	}

	return nil
}

// PowerFlex observability self-metrics validation steps

func (step *Step) validatePowerFlexObsMetricsService(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	svcName := "karavi-metrics-powerflex"
	cmd := exec.Command("kubectl", "get", "svc", svcName, "-n", cr.Namespace) // #nosec G204
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("observability metrics service %s not found in %s: %v, output: %s", svcName, cr.Namespace, err, string(output))
	}
	return nil
}

func (step *Step) validatePowerFlexObsMetricsServiceMonitor(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	smName := "karavi-metrics-powerflex-obs-monitor"
	cmd := exec.Command("kubectl", "get", "servicemonitor", smName, "-n", cr.Namespace) // #nosec G204
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("observability metrics ServiceMonitor %s not found in %s: %v, output: %s", smName, cr.Namespace, err, string(output))
	}
	return nil
}

func (step *Step) validatePowerFlexObsMetricsPort(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	listCmd := exec.Command("kubectl", "get", "pods", "-n", cr.Namespace, // #nosec G204
		"--field-selector=status.phase=Running",
		"-o", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`)
	listOut, err := listCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list pods in %s: %v", cr.Namespace, err)
	}

	podName := ""
	for _, name := range strings.Split(strings.TrimSpace(string(listOut)), "\n") {
		if strings.HasPrefix(name, "karavi-metrics-powerflex-") {
			podName = name
			break
		}
	}
	if podName == "" {
		return fmt.Errorf("no running karavi-metrics-powerflex pod found in namespace %s", cr.Namespace)
	}

	portCmd := exec.Command("kubectl", "get", "pod", podName, "-n", cr.Namespace, // #nosec G204
		"-o", `jsonpath={.spec.containers[?(@.name=="karavi-metrics-powerflex")].ports[?(@.name=="obs-metrics")].containerPort}`)
	portOut, err := portCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to query port for pod %s: %v, output: %s", podName, err, string(portOut))
	}
	if strings.TrimSpace(string(portOut)) == "" {
		return fmt.Errorf("karavi-metrics-powerflex container in pod %s does not expose obs-metrics port", podName)
	}
	return nil
}

func (step *Step) validatePowerFlexObsMetricsConfigMap(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedPort := int32(8443)
	for _, module := range cr.Spec.Modules {
		if string(module.Name) == "observability" {
			if module.Metrics != nil && module.Metrics.Port > 0 {
				expectedPort = module.Metrics.Port
			}
			break
		}
	}

	cmd := exec.Command("kubectl", "get", "configmap", "karavi-metrics-powerflex-configmap", // #nosec G204
		"-n", cr.Namespace, "-o", "yaml")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get karavi-metrics-powerflex-configmap in %s: %v, output: %s", cr.Namespace, err, string(out))
	}

	configContent := string(out)
	if !strings.Contains(configContent, "X_CSI_METRICS_ENABLED: true") {
		return fmt.Errorf("karavi-metrics-powerflex-configmap missing X_CSI_METRICS_ENABLED: true")
	}
	expectedPortLine := fmt.Sprintf("X_CSI_METRICS_PORT: %d", expectedPort)
	if !strings.Contains(configContent, expectedPortLine) {
		return fmt.Errorf("karavi-metrics-powerflex-configmap missing %q", expectedPortLine)
	}
	return nil
}

func (step *Step) validatePowerFlexObsMetricsServiceMonitorHTTPS(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	smName := "karavi-metrics-powerflex-obs-monitor"
	cmd := exec.Command("kubectl", "get", "servicemonitor", smName, "-n", cr.Namespace, // #nosec G204
		"-o", `jsonpath={.spec.endpoints[0].scheme}`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get ServiceMonitor %s in %s: %v, output: %s", smName, cr.Namespace, err, string(out))
	}
	if strings.TrimSpace(string(out)) != "https" {
		return fmt.Errorf("ServiceMonitor %s does not use HTTPS scheme, got: %q", smName, strings.TrimSpace(string(out)))
	}
	return nil
}

func (step *Step) validatePowerFlexObsMetricsTLSVolumeMount(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	listCmd := exec.Command("kubectl", "get", "pods", "-n", cr.Namespace, // #nosec G204
		"--field-selector=status.phase=Running",
		"-o", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`)
	listOut, err := listCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list pods in %s: %v", cr.Namespace, err)
	}

	podName := ""
	for _, name := range strings.Split(strings.TrimSpace(string(listOut)), "\n") {
		if strings.HasPrefix(name, "karavi-metrics-powerflex-") {
			podName = name
			break
		}
	}
	if podName == "" {
		return fmt.Errorf("no running karavi-metrics-powerflex pod found in namespace %s", cr.Namespace)
	}

	volCmd := exec.Command("kubectl", "get", "pod", podName, "-n", cr.Namespace, // #nosec G204
		"-o", `jsonpath={.spec.containers[?(@.name=="karavi-metrics-powerflex")].volumeMounts[?(@.name=="metrics-tls")].mountPath}`)
	volOut, err := volCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to query volume mounts for pod %s: %v, output: %s", podName, err, string(volOut))
	}
	if strings.TrimSpace(string(volOut)) != "/etc/metrics-tls" {
		return fmt.Errorf("karavi-metrics-powerflex container in pod %s does not have metrics-tls volume mounted at /etc/metrics-tls", podName)
	}
	return nil
}

func (step *Step) validateDriverMetricsServiceMonitor(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	smName := cr.Name + "-metrics-monitor"
	cmd := exec.Command("kubectl", "get", "servicemonitor", smName, "-n", cr.Namespace) // #nosec G204
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("driver metrics ServiceMonitor %s not found in %s: %v, output: %s", smName, cr.Namespace, err, string(output))
	}
	return nil
}

func (step *Step) validateDriverMetricsPodMonitor(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	pmName := cr.Name + "-node-metrics-monitor"
	cmd := exec.Command("kubectl", "get", "podmonitor", pmName, "-n", cr.Namespace) // #nosec G204
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("driver metrics PodMonitor %s not found in %s: %v, output: %s", pmName, cr.Namespace, err, string(output))
	}
	return nil
}

// setUpgradePolicyInSpec sets spec.upgrade on a CR file before applying it.
// policy should be "auto" or "manual".
func (step *Step) setUpgradePolicyInSpec(res Resource, policy, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	crFilePath := res.Scenario.Paths[crNum-1]

	crBuff, err := readCRFileForInSpec(crFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CR file %s: %v", crFilePath, err)
	}

	customResource := csmv1.ContainerStorageModule{}
	err = yaml.Unmarshal(crBuff, &customResource)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSM custom resource: %v", err)
	}

	customResource.Spec.Upgrade = csmv1.UpgradePolicy(policy)

	modifiedYAML, err := yaml.Marshal(customResource)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	tempPath, err := writeRenderedFile(crFilePath, string(modifiedYAML))
	if err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	res.Scenario.Paths[crNum-1] = tempPath
	return nil
}

// setUpgradePolicy updates spec.upgrade on a live CR in the cluster.
// policy should be "auto" or "manual".
func (step *Step) setUpgradePolicy(res Resource, policy, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	found.Spec.Upgrade = csmv1.UpgradePolicy(policy)
	return step.ctrlClient.Update(context.TODO(), found)
}

// validateAutoUpgradeCompleted polls until the CR reaches Succeeded state with
// spec.version updated to the expected latest version. The expectedVer parameter
// supports "n" / "n-1" / "n-2" keywords which are resolved dynamically, or a
// literal version string like "v1.18.0".
func (step *Step) validateAutoUpgradeCompleted(res Resource, crNumStr, expectedVer string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedVersion, err := resolveNMinusCSMVersion(expectedVer)
	if err != nil {
		return fmt.Errorf("failed to resolve expected version %q: %v", expectedVer, err)
	}

	fmt.Printf("=== Waiting for auto-upgrade to complete on CR %s (expected version: %s) ===\n", cr.Name, expectedVersion)
	pollDeadline := time.After(5 * time.Minute)
	pollTicker := time.NewTicker(5 * time.Second)
	defer pollTicker.Stop()

	for {
		select {
		case <-pollDeadline:
			// Get current state for a useful error message
			found := new(csmv1.ContainerStorageModule)
			if getErr := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
				Namespace: cr.Namespace,
				Name:      cr.Name,
			}, found); getErr == nil {
				return fmt.Errorf("auto-upgrade timed out after 5m: CR %s state=%s version=%s (expected version %s)",
					cr.Name, found.Status.State, found.Spec.Version, expectedVersion)
			}
			return fmt.Errorf("auto-upgrade timed out after 5m for CR %s", cr.Name)
		case <-pollTicker.C:
			found := new(csmv1.ContainerStorageModule)
			if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
				Namespace: cr.Namespace,
				Name:      cr.Name,
			}, found); err != nil {
				continue
			}
			if found.Status.State == constants.Succeeded && found.Spec.Version == expectedVersion {
				fmt.Printf("=== Auto-upgrade completed: CR %s is at version %s (state=%s) ===\n",
					cr.Name, found.Spec.Version, found.Status.State)
				return nil
			}
		}
	}
}

// waitForDeploymentReady polls until the deployment has the expected number of ready replicas.
func waitForDeploymentReady(clientSet *kubernetes.Clientset, ns, name string, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out waiting for deployment %s/%s to be ready", ns, name)
		case <-ticker.C:
			dep, err := clientSet.AppsV1().Deployments(ns).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				continue
			}
			if dep.Status.ReadyReplicas > 0 && dep.Status.ReadyReplicas == dep.Status.Replicas &&
				dep.Status.UpdatedReplicas == dep.Status.Replicas {
				return nil
			}
		}
	}
}

func (step *Step) validateModuleMetricsInstalled(res Resource, module string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	// Currently only resiliency module supports metrics
	if module == string(csmv1.Resiliency) {
		return step.validateResiliencyMetricsInstalled(*found)
	}

	return fmt.Errorf("metrics validation not implemented for module: %s", module)
}

// checkPodmonMetricsEnabled checks if X_CSI_METRICS_ENABLED is set to true in podmon container.
func checkPodmonMetricsEnabled(containers []acorev1.ContainerApplyConfiguration, podType string) error {
	for _, cnt := range containers {
		if cnt.Name != nil && *cnt.Name == "podmon" {
			metricsEnabled := false
			for _, env := range cnt.Env {
				if env.Name != nil && env.Value != nil && *env.Name == "X_CSI_METRICS_ENABLED" && *env.Value == "true" {
					metricsEnabled = true
					break
				}
			}
			if !metricsEnabled {
				return fmt.Errorf("X_CSI_METRICS_ENABLED not set to true in %s podmon container", podType)
			}
			break
		}
	}
	return nil
}

func (step *Step) validateResiliencyMetricsInstalled(cr csmv1.ContainerStorageModule) error {
	// Find resiliency module
	var resiliencyModule *csmv1.Module
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Resiliency {
			resiliencyModule = &m
			break
		}
	}

	if resiliencyModule == nil {
		return fmt.Errorf("resiliency module not found in CR")
	}

	// If metrics not enabled, nothing to validate
	if resiliencyModule.Metrics == nil || !resiliencyModule.Metrics.Enabled {
		return nil
	}

	ctx := context.TODO()

	// Validate Service exists
	svc := &corev1.Service{}
	serviceName := cr.Name + "-resiliency-metrics"
	if err := step.ctrlClient.Get(ctx, client.ObjectKey{
		Name:      serviceName,
		Namespace: cr.Namespace,
	}, svc); err != nil {
		return fmt.Errorf("failed to get resiliency metrics Service: %v", err)
	}

	// Validate Service port
	expectedPort := int32(8444)
	if resiliencyModule.Metrics.Port != 0 {
		expectedPort = resiliencyModule.Metrics.Port
	}
	if len(svc.Spec.Ports) == 0 {
		return fmt.Errorf("Service has no ports defined")
	}
	if svc.Spec.Ports[0].Port != expectedPort {
		return fmt.Errorf("Service port mismatch: expected %d, got %d", expectedPort, svc.Spec.Ports[0].Port)
	}

	// Validate ServiceMonitor if enabled
	if resiliencyModule.Metrics.ServiceMonitor != nil && resiliencyModule.Metrics.ServiceMonitor.Enabled {
		sm := &unstructured.Unstructured{}
		sm.SetAPIVersion("monitoring.coreos.com/v1")
		sm.SetKind("ServiceMonitor")
		smName := cr.Name + "-resiliency-metrics"
		if err := step.ctrlClient.Get(ctx, client.ObjectKey{
			Name:      smName,
			Namespace: cr.Namespace,
		}, sm); err != nil {
			return fmt.Errorf("failed to get resiliency metrics ServiceMonitor: %v", err)
		}
	}

	// Validate PodMonitor if enabled
	if resiliencyModule.Metrics.PodMonitor != nil && resiliencyModule.Metrics.PodMonitor.Enabled {
		pm := &unstructured.Unstructured{}
		pm.SetAPIVersion("monitoring.coreos.com/v1")
		pm.SetKind("PodMonitor")
		pmName := cr.Name + "-resiliency-metrics"
		if err := step.ctrlClient.Get(ctx, client.ObjectKey{
			Name:      pmName,
			Namespace: cr.Namespace,
		}, pm); err != nil {
			return fmt.Errorf("failed to get resiliency metrics PodMonitor: %v", err)
		}
	}

	// Validate metrics environment variables in podmon containers
	dpApply, dsApply, err := getApplyDeploymentDaemonSet(cr, step.ctrlClient)
	if err != nil {
		return err
	}

	// Check controller pod
	if err := checkPodmonMetricsEnabled(dpApply.Spec.Template.Spec.Containers, "controller"); err != nil {
		return err
	}

	// Check node pod
	if err := checkPodmonMetricsEnabled(dsApply.Spec.Template.Spec.Containers, "node"); err != nil {
		return err
	}

	return nil
}

// createMetroPVC creates a PVC using the metro StorageClass
func (step *Step) createMetroPVC(_ Resource, templateFile, namespace string) error {
	ns := os.ExpandEnv(namespace)
	content, err := os.ReadFile(templateFile)
	if err != nil {
		return fmt.Errorf("failed to read template file %s: %v", templateFile, err)
	}

	yamlContent := strings.ReplaceAll(string(content), "REPLACE_NAMESPACE", ns)

	err = execCommandWithStdin(yamlContent, "kubectl", "apply", "-f", "-")
	if err != nil {
		return fmt.Errorf("failed to create metro PVC: %v", err)
	}
	fmt.Printf("             Created metro PVC in namespace %s\n", ns)
	return nil
}

// waitForPVCBound waits for a PVC to be bound
func (step *Step) waitForPVCBound(_ Resource, pvcName, namespace string) error {
	ns := os.ExpandEnv(namespace)
	timeout := 180 * time.Second
	deadline := time.After(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out waiting for PVC %s/%s to be bound", ns, pvcName)
		case <-ticker.C:
			pvc, err := step.clientSet.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), pvcName, metav1.GetOptions{})
			if err != nil {
				continue
			}
			if pvc.Status.Phase == corev1.ClaimBound {
				fmt.Printf("             PVC %s/%s is bound to PV %s\n", ns, pvcName, pvc.Spec.VolumeName)
				return nil
			}
		}
	}
}

// validatePVCIsMetro validates that a PVC's volume handle indicates metro configuration
func (step *Step) validatePVCIsMetro(_ Resource, pvcName, namespace string) error {
	ns := os.ExpandEnv(namespace)
	pvc, err := step.clientSet.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get PVC %s/%s: %v", ns, pvcName, err)
	}

	if pvc.Spec.VolumeName == "" {
		return fmt.Errorf("PVC %s/%s is not bound to a PV", ns, pvcName)
	}

	// Get the PV to check the volume handle
	output, err := execCommandWithOutput("kubectl", "get", "pv", pvc.Spec.VolumeName, "-o", "jsonpath={.spec.csi.volumeHandle}")
	if err != nil {
		return fmt.Errorf("failed to get PV volume handle: %v", err)
	}

	volumeHandle := string(output)
	// Metro volume handle format: localID/localArrayID/scsi:remoteID/remoteArrayID
	if !strings.Contains(volumeHandle, ":") || strings.Count(volumeHandle, "/") < 3 {
		return fmt.Errorf("PVC %s/%s volume handle %s is not in metro format (expected localID/localArrayID/scsi:remoteID/remoteArrayID)", ns, pvcName, volumeHandle)
	}

	fmt.Printf("             PVC %s/%s has metro volume handle: %s\n", ns, pvcName, volumeHandle)
	return nil
}

// validatePVCIsNotMetro validates that a PVC's volume handle indicates non-metro configuration
func (step *Step) validatePVCIsNotMetro(_ Resource, pvcName, namespace string) error {
	ns := os.ExpandEnv(namespace)
	pvc, err := step.clientSet.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get PVC %s/%s: %v", ns, pvcName, err)
	}

	if pvc.Spec.VolumeName == "" {
		return fmt.Errorf("PVC %s/%s is not bound to a PV", ns, pvcName)
	}

	// Get the PV to check the volume handle
	output, err := execCommandWithOutput("kubectl", "get", "pv", pvc.Spec.VolumeName, "-o", "jsonpath={.spec.csi.volumeHandle}")
	if err != nil {
		return fmt.Errorf("failed to get PV volume handle: %v", err)
	}

	volumeHandle := string(output)
	// Non-metro volume handle format: localID/localArrayID/scsi (no colon after scsi)
	parts := strings.Split(volumeHandle, "/")
	if len(parts) < 3 {
		return fmt.Errorf("PVC %s/%s volume handle %s is malformed (expected localID/localArrayID/scsi...)", ns, pvcName, volumeHandle)
	}
	if strings.Contains(parts[2], ":") {
		return fmt.Errorf("PVC %s/%s volume handle %s appears to be metro format (should be non-metro)", ns, pvcName, volumeHandle)
	}

	fmt.Printf("             PVC %s/%s has non-metro volume handle: %s\n", ns, pvcName, volumeHandle)
	return nil
}

// createVolumeSnapshot creates a VolumeSnapshot from a PVC
func (step *Step) createVolumeSnapshot(_ Resource, templateFile, namespace string) error {
	ns := os.ExpandEnv(namespace)
	content, err := os.ReadFile(templateFile)
	if err != nil {
		return fmt.Errorf("failed to read template file %s: %v", templateFile, err)
	}

	yamlContent := strings.ReplaceAll(string(content), "REPLACE_NAMESPACE", ns)

	err = execCommandWithStdin(yamlContent, "kubectl", "apply", "-f", "-")
	if err != nil {
		return fmt.Errorf("failed to create VolumeSnapshot: %v", err)
	}
	fmt.Printf("             Created VolumeSnapshot in namespace %s\n", ns)
	return nil
}

// waitForSnapshotReady waits for a VolumeSnapshot to be ready
func (step *Step) waitForSnapshotReady(_ Resource, snapshotName, namespace string) error {
	ns := os.ExpandEnv(namespace)
	timeout := 120 * time.Second
	deadline := time.After(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out waiting for VolumeSnapshot %s/%s to be ready", ns, snapshotName)
		case <-ticker.C:
			output, err := execCommandWithOutput("kubectl", "get", "volumesnapshot", snapshotName, "-n", ns, "-o", "jsonpath={.status.readyToUse}")
			if err != nil {
				continue
			}
			if strings.TrimSpace(string(output)) == "true" {
				fmt.Printf("             VolumeSnapshot %s/%s is ready\n", ns, snapshotName)
				return nil
			}
		}
	}
}

// createPod creates a pod from a template
func (step *Step) createPod(_ Resource, templateFile, namespace string) error {
	ns := os.ExpandEnv(namespace)
	content, err := os.ReadFile(templateFile)
	if err != nil {
		return fmt.Errorf("failed to read template file %s: %v", templateFile, err)
	}

	yamlContent := strings.ReplaceAll(string(content), "REPLACE_NAMESPACE", ns)

	err = execCommandWithStdin(yamlContent, "kubectl", "apply", "-f", "-")
	if err != nil {
		return fmt.Errorf("failed to create pod: %v", err)
	}
	fmt.Printf("             Created pod in namespace %s\n", ns)
	return nil
}

// waitForPodRunning waits for a pod to be running
func (step *Step) waitForPodRunning(_ Resource, podName, namespace string) error {
	ns := os.ExpandEnv(namespace)
	timeout := 180 * time.Second
	deadline := time.After(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out waiting for pod %s/%s to be running", ns, podName)
		case <-ticker.C:
			pod, err := step.clientSet.CoreV1().Pods(ns).Get(context.TODO(), podName, metav1.GetOptions{})
			if err != nil {
				continue
			}
			if pod.Status.Phase == corev1.PodRunning {
				fmt.Printf("             Pod %s/%s is running\n", ns, podName)
				return nil
			}
		}
	}
}

// writeDataToPod writes test data to a file in a pod
func (step *Step) writeDataToPod(_ Resource, data, filePath, podName, namespace string) error {
	ns := os.ExpandEnv(namespace)
	err := execCommand("kubectl", "exec", "-n", ns, podName, "--", "sh", "-c", fmt.Sprintf("printf '%%s' %q > %q", data, filePath))
	if err != nil {
		return fmt.Errorf("failed to write data to pod %s/%s: %v", ns, podName, err)
	}
	fmt.Printf("             Wrote data to %s in pod %s/%s\n", filePath, ns, podName)
	return nil
}

// syncDataInPod runs sync command in a pod to flush data to disk
func (step *Step) syncDataInPod(_ Resource, podName, namespace string) error {
	ns := os.ExpandEnv(namespace)
	err := execCommand("kubectl", "exec", "-n", ns, podName, "--", "sync")
	if err != nil {
		return fmt.Errorf("failed to sync data in pod %s/%s: %v", ns, podName, err)
	}
	// Add a small delay to ensure data is flushed
	time.Sleep(2 * time.Second)
	fmt.Printf("             Synced data in pod %s/%s\n", ns, podName)
	return nil
}

// verifyDataInPod verifies that expected data exists in a file in a pod
func (step *Step) verifyDataInPod(_ Resource, expectedData, filePath, podName, namespace string) error {
	ns := os.ExpandEnv(namespace)
	output, err := execCommandWithOutput("kubectl", "exec", "-n", ns, podName, "--", "cat", filePath)
	if err != nil {
		return fmt.Errorf("failed to read data from pod %s/%s: %v", ns, podName, err)
	}

	actualData := strings.TrimSpace(string(output))
	if actualData != expectedData {
		return fmt.Errorf("data mismatch in pod %s/%s: expected %q, got %q", ns, podName, expectedData, actualData)
	}
	fmt.Printf("             Verified data in pod %s/%s matches expected: %s\n", ns, podName, expectedData)
	return nil
}

// validatePVCPending validates that a PVC remains in Pending state (for failure scenarios)
func (step *Step) validatePVCPending(_ Resource, pvcName, namespace string) error {
	ns := os.ExpandEnv(namespace)
	// Wait a bit to ensure the PVC has time to attempt provisioning
	time.Sleep(15 * time.Second)

	pvc, err := step.clientSet.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get PVC %s/%s: %v", ns, pvcName, err)
	}

	if pvc.Status.Phase != corev1.ClaimPending {
		return fmt.Errorf("expected PVC %s/%s to be Pending, but got %s", ns, pvcName, pvc.Status.Phase)
	}
	fmt.Printf("             PVC %s/%s is correctly in Pending state\n", ns, pvcName)
	return nil
}

// validatePVCEventContains validates that a PVC has an event containing the expected message
func (step *Step) validatePVCEventContains(_ Resource, pvcName, namespace, expectedMessage string) error {
	ns := os.ExpandEnv(namespace)
	output, err := execCommandWithOutput("kubectl", "describe", "pvc", pvcName, "-n", ns)
	if err != nil {
		return fmt.Errorf("failed to describe PVC %s/%s: %v", ns, pvcName, err)
	}

	if !strings.Contains(string(output), expectedMessage) {
		return fmt.Errorf("PVC %s/%s events do not contain expected message: %s", ns, pvcName, expectedMessage)
	}
	fmt.Printf("             PVC %s/%s has event containing: %s\n", ns, pvcName, expectedMessage)
	return nil
}

// deletePod deletes a pod
func (step *Step) deletePod(_ Resource, podName, namespace string) error {
	ns := os.ExpandEnv(namespace)
	err := step.clientSet.CoreV1().Pods(ns).Delete(context.TODO(), podName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete pod %s/%s: %v", ns, podName, err)
	}
	fmt.Printf("             Deleted pod %s/%s\n", ns, podName)
	return nil
}

// deletePVC deletes a PVC
func (step *Step) deletePVC(_ Resource, pvcName, namespace string) error {
	ns := os.ExpandEnv(namespace)
	err := step.clientSet.CoreV1().PersistentVolumeClaims(ns).Delete(context.TODO(), pvcName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete PVC %s/%s: %v", ns, pvcName, err)
	}
	fmt.Printf("             Deleted PVC %s/%s\n", ns, pvcName)
	return nil
}

// deleteVolumeSnapshot deletes a VolumeSnapshot
func (step *Step) deleteVolumeSnapshot(_ Resource, snapshotName, namespace string) error {
	ns := os.ExpandEnv(namespace)
	err := execCommand("kubectl", "delete", "volumesnapshot", snapshotName, "-n", ns, "--ignore-not-found")
	if err != nil {
		return fmt.Errorf("failed to delete VolumeSnapshot %s/%s: %v", ns, snapshotName, err)
	}
	fmt.Printf("             Deleted VolumeSnapshot %s/%s\n", ns, snapshotName)
	return nil
}

// createVolumeSnapshotClass creates a VolumeSnapshotClass
func (step *Step) createVolumeSnapshotClass(_ Resource, templateFile string) error {
	err := execCommand("kubectl", "apply", "-f", templateFile)
	if err != nil {
		return fmt.Errorf("failed to create VolumeSnapshotClass: %v", err)
	}
	fmt.Printf("             Created VolumeSnapshotClass from %s\n", templateFile)
	return nil
}

// deleteVolumeSnapshotClass deletes a VolumeSnapshotClass
func (step *Step) deleteVolumeSnapshotClass(_ Resource, snapshotClassName string) error {
	err := execCommand("kubectl", "delete", "volumesnapshotclass", snapshotClassName, "--ignore-not-found")
	if err != nil {
		return fmt.Errorf("failed to delete VolumeSnapshotClass %s: %v", snapshotClassName, err)
	}
	fmt.Printf("             Deleted VolumeSnapshotClass %s\n", snapshotClassName)
	return nil
}

// deleteResourceFromTemplate deletes resources defined in a template file
func (step *Step) deleteResourceFromTemplate(_ Resource, templateFile, namespace string) error {
	ns := os.ExpandEnv(namespace)
	err := execCommand("kubectl", "delete", "-f", templateFile, "-n", ns, "--ignore-not-found")
	if err != nil {
		return fmt.Errorf("failed to delete resources from template %s in namespace %s: %v", templateFile, ns, err)
	}
	fmt.Printf("             Deleted resources from template %s in namespace %s\n", templateFile, ns)
	return nil
}

// waitForPodsDeleted waits for all pods in a namespace to be deleted
func (step *Step) waitForPodsDeleted(_ Resource, namespace string) error {
	ns := os.ExpandEnv(namespace)
	maxRetries := 60 // Wait up to 60 seconds
	for i := 0; i < maxRetries; i++ {
		pods, err := step.clientSet.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list pods in namespace %s: %v", ns, err)
		}
		if len(pods.Items) == 0 {
			fmt.Printf("             All pods deleted in namespace %s\n", ns)
			return nil
		}
		// Force delete any remaining pods
		for _, pod := range pods.Items {
			_ = step.clientSet.CoreV1().Pods(ns).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{
				GracePeriodSeconds: new(int64), // 0 seconds
			})
		}
		time.Sleep(1 * time.Second)
	}
	// List remaining pods for error message
	pods, _ := step.clientSet.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{})
	podNames := make([]string, 0, len(pods.Items))
	for _, pod := range pods.Items {
		podNames = append(podNames, pod.Name)
	}
	return fmt.Errorf("timed out waiting for pods to be deleted in namespace %s, remaining: %v", ns, podNames)
}

// ============================================================================
// mTLS NFS Transport Step Definitions (ER-K8S-BR99506-001-powerscale-mtls-nfs-transport)
// ============================================================================

// isMTLSEnvironmentConfigured checks if the mTLS environment is properly configured.
// Returns true if OneFS supports mTLS and POWERSCALE_MTLS_FQDN is set.
func isMTLSEnvironmentConfigured() bool {
	return isOneFSVersionMTLSSupported() && os.Getenv("POWERSCALE_MTLS_FQDN") != ""
}

func isOneFSVersionMTLSSupported() bool {
	version := strings.Split(os.Getenv("POWERSCALE_ONEFS_VERSION"), ".")
	if len(version) < 2 {
		return false
	}
	major, majorErr := strconv.Atoi(version[0])
	minor, minorErr := strconv.Atoi(version[1])
	if majorErr != nil || minorErr != nil {
		return false
	}
	return major > 9 || (major == 9 && minor >= 16)
}

// skipIfMTLSEnvironmentNotConfigured returns a skip error if mTLS environment is not configured.
// This allows mTLS tests to gracefully skip when:
// 1. OneFS is older than 9.16.0, OR
// 2. POWERSCALE_MTLS_FQDN environment variable is not set
func skipIfMTLSEnvironmentNotConfigured() error {
	if !isOneFSVersionMTLSSupported() {
		return fmt.Errorf("SKIP: PowerScale OneFS %q does not support mTLS; OneFS 9.16.0 or later is required", os.Getenv("POWERSCALE_ONEFS_VERSION"))
	}

	if os.Getenv("POWERSCALE_MTLS_FQDN") == "" {
		return fmt.Errorf("SKIP: POWERSCALE_MTLS_FQDN environment variable not set. Set it to the PowerScale SmartConnect zone FQDN to enable mTLS testing")
	}

	return nil
}

// validateMTLSEnvVars validates that mTLS environment variables are set in the driver pods.
func (step *Step) validateMTLSEnvVars(res Resource, crNumStr string) error {
	// Skip test if mTLS environment is not configured
	if err := skipIfMTLSEnvironmentNotConfigured(); err != nil {
		return err
	}
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	// Check controller deployment
	controllerDeployment := &appsv1.Deployment{}
	err = step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name + "-controller",
	}, controllerDeployment)
	if err != nil {
		return fmt.Errorf("failed to get controller deployment: %v", err)
	}

	// Validate mTLS environment variables in controller
	// Get the expected FQDN from environment variable, or use default for validation
	expectedFQDN := os.Getenv("POWERSCALE_MTLS_FQDN")
	if expectedFQDN == "" {
		expectedFQDN = "powerscale.test.local" // Default for backward compatibility
	}

	requiredEnvVars := map[string]string{
		"X_CSI_ISI_NFS_MOUNT_FQDN": expectedFQDN,
	}

	// Optional environment variables (may not be present in all driver versions)
	optionalEnvVars := map[string]string{
		"X_CSI_ISI_TLS_HANDSHAKE_TIMEOUT_SECONDS": "30",
	}

	for _, container := range controllerDeployment.Spec.Template.Spec.Containers {
		if container.Name == "driver" {
			envMap := make(map[string]string)
			for _, env := range container.Env {
				envMap[env.Name] = env.Value
			}

			// Check required environment variables
			for key, expectedValue := range requiredEnvVars {
				if actualValue, exists := envMap[key]; !exists {
					return fmt.Errorf("environment variable %s not found in controller", key)
				} else if actualValue != expectedValue {
					return fmt.Errorf("environment variable %s has value %s, expected %s", key, actualValue, expectedValue)
				}
			}

			// Check optional environment variables (log warning if not present)
			for key, expectedValue := range optionalEnvVars {
				if actualValue, exists := envMap[key]; exists {
					if actualValue != expectedValue {
						return fmt.Errorf("environment variable %s has value %s, expected %s", key, actualValue, expectedValue)
					}
				}
			}
		}
	}

	// Check node daemonset
	nodeDaemonSet := &appsv1.DaemonSet{}
	err = step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name + "-node",
	}, nodeDaemonSet)
	if err != nil {
		return fmt.Errorf("failed to get node daemonset: %v", err)
	}

	// Validate mTLS environment variables in node
	for _, container := range nodeDaemonSet.Spec.Template.Spec.Containers {
		if container.Name == "driver" {
			envMap := make(map[string]string)
			for _, env := range container.Env {
				envMap[env.Name] = env.Value
			}

			// Check required environment variables
			for key, expectedValue := range requiredEnvVars {
				if actualValue, exists := envMap[key]; !exists {
					return fmt.Errorf("environment variable %s not found in node", key)
				} else if actualValue != expectedValue {
					return fmt.Errorf("environment variable %s has value %s, expected %s in node", key, actualValue, expectedValue)
				}
			}

			// Check optional environment variables (log warning if not present)
			for key, expectedValue := range optionalEnvVars {
				if actualValue, exists := envMap[key]; exists {
					if actualValue != expectedValue {
						return fmt.Errorf("environment variable %s has value %s, expected %s in node", key, actualValue, expectedValue)
					}
				}
			}
		}
	}

	return nil
}

// validateMTLSMount validates that the mTLS mount is working correctly in a pod.
func (step *Step) validateMTLSMount(res Resource, podName, namespace string) error {
	// Skip test if mTLS environment is not configured
	if err := skipIfMTLSEnvironmentNotConfigured(); err != nil {
		return err
	}

	// Wait for pod to be running
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for pod %s to be running", podName)
		case <-ticker.C:
			cmd := exec.Command("kubectl", "get", "pod", podName, "-n", namespace, "-o", "jsonpath={.status.phase}")
			output, err := cmd.CombinedOutput()
			if err != nil {
				continue
			}
			if strings.TrimSpace(string(output)) == "Running" {
				// Pod is running, now check the mount
				cmd = exec.Command("kubectl", "exec", podName, "-n", namespace, "--", "mount")
				output, err := cmd.CombinedOutput()
				if err != nil {
					return fmt.Errorf("failed to check mount in pod: %v, output: %s", err, string(output))
				}

				mountOutput := string(output)
				// Verify NFS mount exists
				if !strings.Contains(mountOutput, "nfs") {
					return fmt.Errorf("no NFS mount found in pod %s", podName)
				}

				// Verify mount options include xprtsec=mtls
				if !strings.Contains(mountOutput, "xprtsec=mtls") {
					return fmt.Errorf("mount does not have xprtsec=mtls option in pod %s. Mount output: %s", podName, mountOutput)
				}

				// Verify NFSv4.1 is used
				if !strings.Contains(mountOutput, "vers=4.1") && !strings.Contains(mountOutput, "nfs4") {
					return fmt.Errorf("mount does not use NFSv4.1 in pod %s. Mount output: %s", podName, mountOutput)
				}

				// Test write operation
				cmd = exec.Command("kubectl", "exec", podName, "-n", namespace, "--", "sh", "-c", "echo 'mTLS test' > /data/mtls-test.txt && cat /data/mtls-test.txt")
				output, err = cmd.CombinedOutput()
				if err != nil {
					return fmt.Errorf("failed to write to mTLS mount: %v, output: %s", err, string(output))
				}

				if !strings.Contains(string(output), "mTLS test") {
					return fmt.Errorf("failed to read back test data from mTLS mount")
				}

				return nil
			}
		}
	}
}

// validateMTLSMountOptions validates that the mount options include xprtsec=mtls.
func (step *Step) validateMTLSMountOptions(res Resource, podName, namespace string) error {
	// Skip test if mTLS environment is not configured
	if err := skipIfMTLSEnvironmentNotConfigured(); err != nil {
		return err
	}

	cmd := exec.Command("kubectl", "exec", podName, "-n", namespace, "--", "mount")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check mount options: %v, output: %s", err, string(output))
	}

	mountOutput := string(output)

	// Check for required mount options
	requiredOptions := []string{"xprtsec=mtls", "vers=4.1", "hard"}
	for _, option := range requiredOptions {
		if !strings.Contains(mountOutput, option) {
			return fmt.Errorf("mount does not have required option %s. Mount output: %s", option, mountOutput)
		}
	}

	return nil
}

// validateStorageClassFQDNPrecedence validates that StorageClass FQDN takes precedence.
// Note: Linux NFS mounts resolve hostnames to IP addresses, so the mount output shows the
// resolved IP, not the original FQDN. We validate FQDN precedence by:
//  1. Verifying the StorageClass has the correct SmartConnectZoneFQDN parameter
//  2. Verifying the PV volume attributes contain the resolved FQDN
//  3. Verifying mTLS is working (xprtsec=mtls in mount) - this proves the FQDN was used
//     because mTLS requires certificate validation against the FQDN
//  4. Verifying the mount is functional (read/write test)
func (step *Step) validateStorageClassFQDNPrecedence(res Resource, podName, namespace string) error {
	// Skip test if mTLS environment is not configured
	if err := skipIfMTLSEnvironmentNotConfigured(); err != nil {
		return err
	}

	expectedFQDN := os.Getenv("POWERSCALE_MTLS_FQDN")
	if expectedFQDN == "" {
		expectedFQDN = "powerscale.test.local"
	}

	// Get the PVC used by the pod
	cmd := exec.Command("kubectl", "get", "pod", podName, "-n", namespace, "-o", "jsonpath={.spec.volumes[0].persistentVolumeClaim.claimName}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PVC name: %v", err)
	}
	pvcName := strings.TrimSpace(string(output))

	// Get the StorageClass from the PVC
	cmd = exec.Command("kubectl", "get", "pvc", pvcName, "-n", namespace, "-o", "jsonpath={.spec.storageClassName}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get StorageClass name: %v", err)
	}
	scName := strings.TrimSpace(string(output))

	// Verify StorageClass has SmartConnectZoneFQDN parameter
	cmd = exec.Command("kubectl", "get", "storageclass", scName, "-o", "jsonpath={.parameters.SmartConnectZoneFQDN}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get StorageClass FQDN: %v", err)
	}
	scFQDN := strings.TrimSpace(string(output))

	if scFQDN != expectedFQDN {
		return fmt.Errorf("StorageClass FQDN is %s, expected %s", scFQDN, expectedFQDN)
	}

	// Get the PV name from the PVC and verify FQDN in PV volume attributes
	cmd = exec.Command("kubectl", "get", "pvc", pvcName, "-n", namespace, "-o", "jsonpath={.spec.volumeName}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PV name: %v", err)
	}
	pvName := strings.TrimSpace(string(output))

	cmd = exec.Command("kubectl", "get", "pv", pvName, "-o", "jsonpath={.spec.csi.volumeAttributes.SmartConnectZoneFQDN}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PV FQDN attribute: %v", err)
	}
	pvFQDN := strings.TrimSpace(string(output))

	if pvFQDN != expectedFQDN {
		return fmt.Errorf("PV SmartConnectZoneFQDN is '%s', expected '%s' (proves FQDN was resolved during CreateVolume)", pvFQDN, expectedFQDN)
	}

	// Verify the mount has xprtsec=mtls - this proves the FQDN was used because
	// mTLS requires certificate validation against the FQDN (IP addresses are rejected)
	cmd = exec.Command("kubectl", "exec", podName, "-n", namespace, "--", "mount")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check mount: %v", err)
	}

	mountOutput := string(output)

	// Check for xprtsec=mtls which proves FQDN was used (mTLS requires FQDN for cert validation)
	if !strings.Contains(mountOutput, "xprtsec=mtls") {
		return fmt.Errorf("mount does not have xprtsec=mtls, indicating FQDN was not used for mTLS. Mount output: %s", mountOutput)
	}

	// Verify the mount is functional by writing and reading a test file
	cmd = exec.Command("kubectl", "exec", podName, "-n", namespace, "--", "sh", "-c", "echo 'fqdn-test' > /data/fqdn-test.txt && cat /data/fqdn-test.txt")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write/read test file on mTLS mount: %v, output: %s", err, string(output))
	}

	if !strings.Contains(string(output), "fqdn-test") {
		return fmt.Errorf("failed to read back test data from mTLS mount, indicating mount is not functional")
	}

	return nil
}

// validateFQDNFallbackPV validates a fallback source independently by checking the
// resolved FQDN and transport security persisted in the PV volume attributes.
func (step *Step) validateFQDNFallbackPV(res Resource, source, pvcName, expectedFQDN, namespace string) error {
	if err := skipIfMTLSEnvironmentNotConfigured(); err != nil {
		return err
	}

	cmd := exec.Command("kubectl", "get", "pvc", pvcName, "-n", namespace, "-o", "jsonpath={.spec.storageClassName}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get StorageClass name for PVC %s: %v", pvcName, err)
	}
	scName := strings.TrimSpace(string(output))

	cmd = exec.Command("kubectl", "get", "storageclass", scName, "-o", "jsonpath={.parameters.SmartConnectZoneFQDN}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get StorageClass FQDN: %v", err)
	}
	if scFQDN := strings.TrimSpace(string(output)); scFQDN != "" {
		return fmt.Errorf("StorageClass has SmartConnectZoneFQDN='%s', but this test requires it to be empty", scFQDN)
	}

	cmd = exec.Command("kubectl", "get", "pvc", pvcName, "-n", namespace, "-o", "jsonpath={.spec.volumeName}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PV name for PVC %s: %v", pvcName, err)
	}
	pvName := strings.TrimSpace(string(output))

	cmd = exec.Command("kubectl", "get", "pv", pvName, "-o", "jsonpath={.spec.csi.volumeAttributes.SmartConnectZoneFQDN}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PV FQDN attribute: %v", err)
	}
	if pvFQDN := strings.TrimSpace(string(output)); pvFQDN != expectedFQDN {
		return fmt.Errorf("PV SmartConnectZoneFQDN is '%s', expected '%s' from %s fallback", pvFQDN, expectedFQDN, source)
	}

	cmd = exec.Command("kubectl", "get", "pv", pvName, "-o", "jsonpath={.spec.csi.volumeAttributes.NFSTransportSecurity}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PV NFSTransportSecurity attribute: %v", err)
	}
	if transportSecurity := strings.TrimSpace(string(output)); transportSecurity != "mtls" {
		return fmt.Errorf("PV NFSTransportSecurity is '%s', expected 'mtls'", transportSecurity)
	}

	return nil
}

// validateFQDNFallbackPrecedence validates that FQDN is resolved from Secret/environment
// when StorageClass SmartConnectZoneFQDN is not set. This proves the fallback chain works.
// Use with powerscale-storageclass-mtls-no-fqdn-template.yaml which omits SmartConnectZoneFQDN.
func (step *Step) validateFQDNFallbackPrecedence(res Resource, podName, namespace string) error {
	// Skip test if mTLS environment is not configured
	if err := skipIfMTLSEnvironmentNotConfigured(); err != nil {
		return err
	}

	expectedFQDN := os.Getenv("POWERSCALE_MTLS_FQDN")
	if expectedFQDN == "" {
		return fmt.Errorf("POWERSCALE_MTLS_FQDN must be set for fallback precedence test")
	}

	// Get the PVC used by the pod
	cmd := exec.Command("kubectl", "get", "pod", podName, "-n", namespace, "-o", "jsonpath={.spec.volumes[0].persistentVolumeClaim.claimName}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PVC name: %v", err)
	}
	pvcName := strings.TrimSpace(string(output))

	// Get the StorageClass and verify it does NOT have SmartConnectZoneFQDN
	cmd = exec.Command("kubectl", "get", "pvc", pvcName, "-n", namespace, "-o", "jsonpath={.spec.storageClassName}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get StorageClass name: %v", err)
	}
	scName := strings.TrimSpace(string(output))

	cmd = exec.Command("kubectl", "get", "storageclass", scName, "-o", "jsonpath={.parameters.SmartConnectZoneFQDN}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get StorageClass FQDN: %v", err)
	}
	scFQDN := strings.TrimSpace(string(output))

	if scFQDN != "" {
		return fmt.Errorf("StorageClass has SmartConnectZoneFQDN='%s', but this test requires it to be empty to test fallback", scFQDN)
	}

	// Get the PV and verify FQDN was resolved from Secret or environment
	cmd = exec.Command("kubectl", "get", "pvc", pvcName, "-n", namespace, "-o", "jsonpath={.spec.volumeName}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PV name: %v", err)
	}
	pvName := strings.TrimSpace(string(output))

	cmd = exec.Command("kubectl", "get", "pv", pvName, "-o", "jsonpath={.spec.csi.volumeAttributes.SmartConnectZoneFQDN}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PV FQDN attribute: %v", err)
	}
	pvFQDN := strings.TrimSpace(string(output))

	if pvFQDN != expectedFQDN {
		return fmt.Errorf("PV SmartConnectZoneFQDN is '%s', expected '%s' (proves FQDN fallback from Secret/environment)", pvFQDN, expectedFQDN)
	}

	// Verify NFSTransportSecurity is set (StorageClass-only, no inheritance)
	cmd = exec.Command("kubectl", "get", "pv", pvName, "-o", "jsonpath={.spec.csi.volumeAttributes.NFSTransportSecurity}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get PV NFSTransportSecurity attribute: %v", err)
	}
	pvTransportSecurity := strings.TrimSpace(string(output))

	if pvTransportSecurity != "mtls" {
		return fmt.Errorf("PV NFSTransportSecurity is '%s', expected 'mtls'", pvTransportSecurity)
	}

	// Verify the mount has xprtsec=mtls
	cmd = exec.Command("kubectl", "exec", podName, "-n", namespace, "--", "mount")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check mount: %v", err)
	}

	if !strings.Contains(string(output), "xprtsec=mtls") {
		return fmt.Errorf("mount does not have xprtsec=mtls. Mount output: %s", string(output))
	}

	return nil
}

// validateTLSHandshakeTimeout validates that the TLS handshake timeout is set correctly.
func (step *Step) validateTLSHandshakeTimeout(res Resource, timeoutStr, crNumStr string) error {
	// Skip test if mTLS environment is not configured
	if err := skipIfMTLSEnvironmentNotConfigured(); err != nil {
		return err
	}

	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	if crNum > len(res.CustomResource) {
		return fmt.Errorf("CR index %d out of range (total CRs: %d)", crNum, len(res.CustomResource))
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	// Check controller deployment
	controllerDeployment := &appsv1.Deployment{}
	err = step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name + "-controller",
	}, controllerDeployment)
	if err != nil {
		return fmt.Errorf("failed to get controller deployment: %v", err)
	}

	// Check environment variable
	for _, container := range controllerDeployment.Spec.Template.Spec.Containers {
		if container.Name == "driver" {
			for _, env := range container.Env {
				if env.Name == "X_CSI_ISI_TLS_HANDSHAKE_TIMEOUT_SECONDS" {
					if env.Value != timeoutStr {
						return fmt.Errorf("TLS handshake timeout is %s, expected %s", env.Value, timeoutStr)
					}
					return nil
				}
			}
		}
	}

	return fmt.Errorf("TLS handshake timeout environment variable not found")
}

// validatePVCBound validates that a PVC is bound.
// For mTLS PVCs, skips if mTLS environment is not configured.
func (step *Step) validatePVCBound(res Resource, pvcName, namespace string) error {
	// Skip mTLS PVC validation if mTLS environment is not configured
	// This check must happen BEFORE any PVC operations to prevent timeouts
	if strings.Contains(pvcName, "mtls") {
		if err := skipIfMTLSEnvironmentNotConfigured(); err != nil {
			return err
		}
	}

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for PVC %s to be bound", pvcName)
		case <-ticker.C:
			cmd := exec.Command("kubectl", "get", "pvc", pvcName, "-n", namespace, "-o", "jsonpath={.status.phase}")
			output, err := cmd.CombinedOutput()
			if err != nil {
				continue
			}
			if strings.TrimSpace(string(output)) == "Bound" {
				return nil
			}
		}
	}
}

// validatePodRunning validates that a pod is running.
// For mTLS pods, skips if mTLS environment is not configured.
func (step *Step) validatePodRunning(res Resource, podName, namespace string) error {
	// Skip mTLS pod validation if mTLS environment is not configured
	if strings.Contains(podName, "mtls") {
		if err := skipIfMTLSEnvironmentNotConfigured(); err != nil {
			return err
		}
	}

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			// Get pod events for debugging
			cmd := exec.Command("kubectl", "describe", "pod", podName, "-n", namespace)
			output, _ := cmd.CombinedOutput()
			return fmt.Errorf("timeout waiting for pod %s to be running. Pod description:\n%s", podName, string(output))
		case <-ticker.C:
			cmd := exec.Command("kubectl", "get", "pod", podName, "-n", namespace, "-o", "jsonpath={.status.phase}")
			output, err := cmd.CombinedOutput()
			if err != nil {
				continue
			}
			phase := strings.TrimSpace(string(output))
			if phase == "Running" {
				return nil
			}
			if phase == "Failed" || phase == "CrashLoopBackOff" {
				cmd = exec.Command("kubectl", "describe", "pod", podName, "-n", namespace)
				output, _ := cmd.CombinedOutput()
				return fmt.Errorf("pod %s failed. Pod description:\n%s", podName, string(output))
			}
		}
	}
}

// createPVCWithNonMTLSStorageClass creates a PVC with non-mTLS StorageClass.
// Uses isilon-non-mtls StorageClass which has the same IsiPath as the mTLS StorageClass
// to ensure the path exists on the PowerScale array.
// Skips if mTLS environment is not configured since this test is part of mTLS test suite.
func (step *Step) createPVCWithNonMTLSStorageClass(res Resource, namespace string) error {
	// Skip test if mTLS environment is not configured
	if err := skipIfMTLSEnvironmentNotConfigured(); err != nil {
		return err
	}

	pvcYAML := `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: pscale-pvc-non-mtls
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
  storageClassName: isilon-non-mtls
`
	cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", namespace)
	cmd.Stdin = strings.NewReader(pvcYAML)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create non-mTLS PVC: %v, output: %s", err, string(output))
	}
	return nil
}

// validateNonMTLSPVCBound validates that a non-mTLS PVC is bound.
// Skips if mTLS environment is not configured since this test is part of mTLS test suite.
func (step *Step) validateNonMTLSPVCBound(res Resource, namespace string) error {
	// Skip test if mTLS environment is not configured
	if err := skipIfMTLSEnvironmentNotConfigured(); err != nil {
		return err
	}
	return step.validatePVCBound(res, "pscale-pvc-non-mtls", namespace)
}

// deleteNonMTLSPVC deletes the non-mTLS PVC.
// Skips if mTLS environment is not configured since this test is part of mTLS test suite.
func (step *Step) deleteNonMTLSPVC(res Resource, namespace string) error {
	// Skip test if mTLS environment is not configured
	if err := skipIfMTLSEnvironmentNotConfigured(); err != nil {
		return err
	}

	cmd := exec.Command("kubectl", "delete", "pvc", "pscale-pvc-non-mtls", "-n", namespace, "--ignore-not-found=true")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete non-mTLS PVC: %v, output: %s", err, string(output))
	}
	return nil
}

// updateSecretWithTemplate updates a secret with a new template.
// Skips if mTLS environment is not configured when updating to mTLS template.
func (step *Step) updateSecretWithTemplate(res Resource, templatePath, secretName, namespace, driverType string) error {
	// Skip if updating to mTLS template and mTLS environment is not configured
	if strings.Contains(templatePath, "mtls") {
		if err := skipIfMTLSEnvironmentNotConfigured(); err != nil {
			return err
		}
	}

	// Delete existing secret
	cmd := exec.Command("kubectl", "delete", "secret", secretName, "-n", namespace, "--ignore-not-found=true")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete existing secret: %v", err)
	}

	// Create new secret with updated template
	return step.setUpSecret(Resource{}, templatePath, secretName, namespace, driverType)
}

// validateDriverMetricsPrometheusRule validates that the PrometheusRule resource
// named <cr-name>-alerts is present in the cluster. It polls until the resource
// exists or the timeout is reached, to allow for operator reconciliation time.
func (step *Step) validateDriverMetricsPrometheusRule(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	prName := cr.Name + "-alerts"
	fmt.Printf("=== Waiting for PrometheusRule %s in %s to be created ===\n", prName, cr.Namespace)

	deadline := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out waiting for PrometheusRule %s to be created in %s", prName, cr.Namespace)
		case <-ticker.C:
			cmd := exec.Command("kubectl", "get", "prometheusrule", prName, "-n", cr.Namespace) // #nosec G204
			if err := cmd.Run(); err == nil {
				fmt.Printf("=== PrometheusRule %s found in %s ===\n", prName, cr.Namespace)
				return nil
			}
		}
	}
}

// validateDriverMetricsPrometheusRuleAbsent validates that the PrometheusRule
// named <cr-name>-alerts is NOT present in the cluster. It polls until the
// resource is gone or the timeout is reached.
func (step *Step) validateDriverMetricsPrometheusRuleAbsent(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	prName := cr.Name + "-alerts"
	fmt.Printf("=== Waiting for PrometheusRule %s in %s to be absent ===\n", prName, cr.Namespace)

	deadline := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out waiting for PrometheusRule %s to be deleted in %s", prName, cr.Namespace)
		case <-ticker.C:
			cmd := exec.Command("kubectl", "get", "prometheusrule", prName, "-n", cr.Namespace) // #nosec G204
			output, err := cmd.CombinedOutput()
			if err != nil && strings.Contains(string(output), "not found") {
				fmt.Printf("=== PrometheusRule %s confirmed absent in %s ===\n", prName, cr.Namespace)
				return nil
			}
		}
	}
}

// enablePrometheusRuleInCR sets spec.driver.metrics.prometheusRule.enabled = true on the live CR.
func (step *Step) enablePrometheusRuleInCR(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	if found.Spec.Driver.Metrics == nil {
		found.Spec.Driver.Metrics = &csmv1.DriverMetrics{}
	}
	if found.Spec.Driver.Metrics.PrometheusRule == nil {
		found.Spec.Driver.Metrics.PrometheusRule = &csmv1.MetricsPrometheusRuleConfig{}
	}
	found.Spec.Driver.Metrics.PrometheusRule.Enabled = true

	return step.ctrlClient.Update(context.TODO(), found)
}

// disablePrometheusRuleInCR sets spec.driver.metrics.prometheusRule.enabled = false on the live CR.
func (step *Step) disablePrometheusRuleInCR(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	if found.Spec.Driver.Metrics != nil && found.Spec.Driver.Metrics.PrometheusRule != nil {
		found.Spec.Driver.Metrics.PrometheusRule.Enabled = false
	}

	return step.ctrlClient.Update(context.TODO(), found)
}

// validateObservabilityPrometheusRule validates that the PrometheusRule resource
// named karavi-observability-alerts is present in the cluster. It polls until the
// resource exists or the timeout is reached, to allow for operator reconciliation time.
func (step *Step) validateObservabilityPrometheusRule(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	prName := "karavi-observability-alerts"
	fmt.Printf("=== Waiting for PrometheusRule %s in %s to be created ===\n", prName, cr.Namespace)

	deadline := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out waiting for PrometheusRule %s to be created in %s", prName, cr.Namespace)
		case <-ticker.C:
			cmd := exec.Command("kubectl", "get", "prometheusrule", prName, "-n", cr.Namespace) // #nosec G204
			if err := cmd.Run(); err == nil {
				fmt.Printf("=== PrometheusRule %s found in %s ===\n", prName, cr.Namespace)
				return nil
			}
		}
	}
}

// validateObservabilityPrometheusRuleAbsent validates that the PrometheusRule
// named karavi-observability-alerts is NOT present in the cluster. It polls until
// the resource is gone or the timeout is reached.
func (step *Step) validateObservabilityPrometheusRuleAbsent(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	prName := "karavi-observability-alerts"
	fmt.Printf("=== Waiting for PrometheusRule %s in %s to be absent ===\n", prName, cr.Namespace)

	deadline := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out waiting for PrometheusRule %s to be deleted in %s", prName, cr.Namespace)
		case <-ticker.C:
			cmd := exec.Command("kubectl", "get", "prometheusrule", prName, "-n", cr.Namespace) // #nosec G204
			output, err := cmd.CombinedOutput()
			if err != nil && strings.Contains(string(output), "not found") {
				fmt.Printf("=== PrometheusRule %s confirmed absent in %s ===\n", prName, cr.Namespace)
				return nil
			}
		}
	}
}

// enableObservabilityPrometheusRuleInCR sets spec.modules[].metrics.prometheusRule.enabled = true on the live CR.
func (step *Step) enableObservabilityPrometheusRuleInCR(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	// Find observability module and enable prometheusRule
	for i, module := range found.Spec.Modules {
		if module.Name == csmv1.Observability {
			if found.Spec.Modules[i].Metrics == nil {
				found.Spec.Modules[i].Metrics = &csmv1.ModuleMetrics{}
			}
			if found.Spec.Modules[i].Metrics.PrometheusRule == nil {
				found.Spec.Modules[i].Metrics.PrometheusRule = &csmv1.ModulePrometheusRuleConfig{}
			}
			found.Spec.Modules[i].Metrics.PrometheusRule.Enabled = true
			break
		}
	}

	return step.ctrlClient.Update(context.TODO(), found)
}

// disableObservabilityPrometheusRuleInCR sets spec.modules[].metrics.prometheusRule.enabled = false on the live CR.
func (step *Step) disableObservabilityPrometheusRuleInCR(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	// Find observability module and disable prometheusRule
	for i, module := range found.Spec.Modules {
		if module.Name == csmv1.Observability {
			if found.Spec.Modules[i].Metrics != nil && found.Spec.Modules[i].Metrics.PrometheusRule != nil {
				found.Spec.Modules[i].Metrics.PrometheusRule.Enabled = false
			}
			break
		}
	}

	return step.ctrlClient.Update(context.TODO(), found)
}

// validateResiliencyPrometheusRule validates that the PrometheusRule resource
// named <cr-name>-resiliency-alerts is present in the cluster. It polls until
// the resource exists or the timeout is reached, to allow for operator reconciliation time.
func (step *Step) validateResiliencyPrometheusRule(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	prName := cr.Name + "-resiliency-alerts"
	fmt.Printf("=== Waiting for PrometheusRule %s in %s to be created ===\n", prName, cr.Namespace)

	deadline := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out waiting for PrometheusRule %s to be created in %s", prName, cr.Namespace)
		case <-ticker.C:
			cmd := exec.Command("kubectl", "get", "prometheusrule", prName, "-n", cr.Namespace) // #nosec G204
			if err := cmd.Run(); err == nil {
				fmt.Printf("=== PrometheusRule %s found in %s ===\n", prName, cr.Namespace)
				return nil
			}
		}
	}
}

// validateResiliencyPrometheusRuleAbsent validates that the PrometheusRule
// named <cr-name>-resiliency-alerts is NOT present in the cluster. It polls until
// the resource is gone or the timeout is reached.
func (step *Step) validateResiliencyPrometheusRuleAbsent(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	prName := cr.Name + "-resiliency-alerts"
	fmt.Printf("=== Waiting for PrometheusRule %s in %s to be absent ===\n", prName, cr.Namespace)

	deadline := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out waiting for PrometheusRule %s to be deleted in %s", prName, cr.Namespace)
		case <-ticker.C:
			cmd := exec.Command("kubectl", "get", "prometheusrule", prName, "-n", cr.Namespace) // #nosec G204
			output, err := cmd.CombinedOutput()
			if err != nil && strings.Contains(string(output), "not found") {
				fmt.Printf("=== PrometheusRule %s confirmed absent in %s ===\n", prName, cr.Namespace)
				return nil
			}
		}
	}
}

// enableResiliencyPrometheusRuleInCR sets spec.modules[resiliency].metrics.prometheusRule.enabled = true on the live CR.
func (step *Step) enableResiliencyPrometheusRuleInCR(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	// Find resiliency module and enable prometheusRule
	for i, module := range found.Spec.Modules {
		if module.Name == csmv1.Resiliency {
			if found.Spec.Modules[i].Metrics == nil {
				found.Spec.Modules[i].Metrics = &csmv1.ModuleMetrics{}
			}
			if found.Spec.Modules[i].Metrics.PrometheusRule == nil {
				found.Spec.Modules[i].Metrics.PrometheusRule = &csmv1.ModulePrometheusRuleConfig{}
			}
			found.Spec.Modules[i].Metrics.PrometheusRule.Enabled = true
			break
		}
	}

	return step.ctrlClient.Update(context.TODO(), found)
}

// disableResiliencyPrometheusRuleInCR sets spec.modules[resiliency].metrics.prometheusRule.enabled = false on the live CR.
func (step *Step) disableResiliencyPrometheusRuleInCR(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	// Find resiliency module and disable prometheusRule
	for i, module := range found.Spec.Modules {
		if module.Name == csmv1.Resiliency {
			if found.Spec.Modules[i].Metrics != nil && found.Spec.Modules[i].Metrics.PrometheusRule != nil {
				found.Spec.Modules[i].Metrics.PrometheusRule.Enabled = false
			}
			break
		}
	}

	return step.ctrlClient.Update(context.TODO(), found)
}

// enableObservabilityModuleInCR enables the observability module in the CR
func (step *Step) enableObservabilityModuleInCR(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	// Find and enable observability module
	for i, module := range found.Spec.Modules {
		if module.Name == csmv1.Observability {
			found.Spec.Modules[i].Enabled = true
			if found.Spec.Modules[i].Metrics == nil {
				found.Spec.Modules[i].Metrics = &csmv1.ModuleMetrics{Enabled: true}
			}
			break
		}
	}

	return step.ctrlClient.Update(context.TODO(), found)
}

// getAuthorizationModuleMetricsConfig returns the Metrics configuration for the
// authorization-proxy-server module in the given CR, or nil if not present.
func getAuthorizationModuleMetricsConfig(cr csmv1.ContainerStorageModule) *csmv1.ModuleMetrics {
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.AuthorizationServer {
			return m.Metrics
		}
	}
	return nil
}

// getAuthorizationPrometheusRuleName returns the expected PrometheusRule name for
// the CSM Authorization module based on the CR name.
func getAuthorizationPrometheusRuleName(cr csmv1.ContainerStorageModule) string {
	return cr.Name + "-csm-authorization-alerts"
}

// waitForAuthorizationPrometheusRule polls until the authorization PrometheusRule
// is either present or absent, depending on shouldExist.
func waitForAuthorizationPrometheusRule(cr csmv1.ContainerStorageModule, shouldExist bool) error {
	prName := getAuthorizationPrometheusRuleName(cr)
	action := "created"
	if !shouldExist {
		action = "deleted"
	}
	fmt.Printf("=== Waiting for Authorization PrometheusRule %s in %s to be %s ===\n", prName, cr.Namespace, action)

	deadline := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out waiting for Authorization PrometheusRule %s to be %s in %s", prName, action, cr.Namespace)
		case <-ticker.C:
			cmd := exec.Command("kubectl", "get", "prometheusrule", prName, "-n", cr.Namespace) // #nosec G204
			if shouldExist {
				if err := cmd.Run(); err == nil {
					fmt.Printf("=== Authorization PrometheusRule %s found in %s ===\n", prName, cr.Namespace)
					return nil
				}
			} else {
				output, err := cmd.CombinedOutput()
				if err != nil && strings.Contains(string(output), "not found") {
					fmt.Printf("=== Authorization PrometheusRule %s confirmed absent in %s ===\n", prName, cr.Namespace)
					return nil
				}
			}
		}
	}
}

// getAuthorizationPrometheusRuleJSON fetches the PrometheusRule resource as JSON.
func getAuthorizationPrometheusRuleJSON(cr csmv1.ContainerStorageModule) (map[string]interface{}, error) {
	prName := getAuthorizationPrometheusRuleName(cr)
	cmd := exec.Command("kubectl", "get", "prometheusrule", prName, "-n", cr.Namespace, "-o", "json") // #nosec G204
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get Authorization PrometheusRule %s: %v, output: %s", prName, err, string(output))
	}
	var pr map[string]interface{}
	if err := json.Unmarshal(output, &pr); err != nil {
		return nil, fmt.Errorf("failed to parse PrometheusRule %s JSON: %v", prName, err)
	}
	return pr, nil
}

// getAuthorizationAlertRules extracts the alert rules slice from the PrometheusRule JSON.
func getAuthorizationAlertRules(pr map[string]interface{}) ([]interface{}, error) {
	spec, ok := pr["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("PrometheusRule spec not found or invalid")
	}
	groups, ok := spec["groups"].([]interface{})
	if !ok || len(groups) == 0 {
		return nil, fmt.Errorf("PrometheusRule spec.groups not found or invalid")
	}
	group, ok := groups[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("PrometheusRule spec.groups[0] not a map")
	}
	rules, ok := group["rules"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("PrometheusRule spec.groups[0].rules not found or invalid")
	}
	return rules, nil
}

// validateAuthorizationPrometheusRuleCreated validates that the CSM Authorization
// PrometheusRule resource is present in the cluster.
func (step *Step) validateAuthorizationPrometheusRuleCreated(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	return waitForAuthorizationPrometheusRule(cr, true)
}

// validateAuthorizationPrometheusRuleAbsent validates that the CSM Authorization
// PrometheusRule resource is not present in the cluster.
func (step *Step) validateAuthorizationPrometheusRuleAbsent(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	return waitForAuthorizationPrometheusRule(cr, false)
}

// validateAuthorizationPrometheusRuleAlertCount validates that the CSM Authorization
// PrometheusRule contains the expected number of alert rules.
func (step *Step) validateAuthorizationPrometheusRuleAlertCount(res Resource, expectedCountStr string, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedCount, err := strconv.Atoi(expectedCountStr)
	if err != nil {
		return fmt.Errorf("invalid expected alert count %q: %v", expectedCountStr, err)
	}

	pr, err := getAuthorizationPrometheusRuleJSON(cr)
	if err != nil {
		return err
	}
	rules, err := getAuthorizationAlertRules(pr)
	if err != nil {
		return err
	}
	if len(rules) != expectedCount {
		return fmt.Errorf("expected %d alert rules in Authorization PrometheusRule %s, got %d", expectedCount, getAuthorizationPrometheusRuleName(cr), len(rules))
	}
	fmt.Printf("=== Authorization PrometheusRule %s contains %d alert rules ===\n", getAuthorizationPrometheusRuleName(cr), len(rules))
	return nil
}

// validateAuthorizationPrometheusRuleOwnerReference validates that the CSM Authorization
// PrometheusRule has the CSM CR as an owner reference.
func (step *Step) validateAuthorizationPrometheusRuleOwnerReference(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	cmd := exec.Command("kubectl", "get", "prometheusrule", getAuthorizationPrometheusRuleName(cr), "-n", cr.Namespace,
		"-o", "jsonpath={range .metadata.ownerReferences[?(@.kind=='ContainerStorageModule')]}{.name}{end}") // #nosec G204
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get ownerReference for PrometheusRule %s: %v, output: %s", getAuthorizationPrometheusRuleName(cr), err, string(output))
	}
	if strings.TrimSpace(string(output)) != cr.Name {
		return fmt.Errorf("PrometheusRule %s ownerReference name is %q, expected %q", getAuthorizationPrometheusRuleName(cr), strings.TrimSpace(string(output)), cr.Name)
	}
	fmt.Printf("=== Authorization PrometheusRule %s has owner reference %s/%s ===\n", getAuthorizationPrometheusRuleName(cr), cr.Kind, cr.Name)
	return nil
}

// validateAuthorizationPrometheusRuleThresholds validates that the rendered CSM
// Authorization PrometheusRule expressions contain the threshold values from the CR spec.
func (step *Step) validateAuthorizationPrometheusRuleThresholds(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	metrics := getAuthorizationModuleMetricsConfig(cr)
	var promRuleCfg *csmv1.ModulePrometheusRuleConfig
	if metrics != nil {
		promRuleCfg = metrics.PrometheusRule
	}

	var failureRate int32 = 10
	var latencyQuantile int32 = 95
	var latencySeconds int32 = 2
	if promRuleCfg != nil {
		if promRuleCfg.AuthFailureRateThreshold != nil {
			failureRate = *promRuleCfg.AuthFailureRateThreshold
		}
		if promRuleCfg.AuthLatencyQuantile != nil {
			latencyQuantile = *promRuleCfg.AuthLatencyQuantile
		}
		if promRuleCfg.AuthLatencyThresholdSeconds != nil {
			latencySeconds = *promRuleCfg.AuthLatencyThresholdSeconds
		}
	}

	pr, err := getAuthorizationPrometheusRuleJSON(cr)
	if err != nil {
		return err
	}
	rules, err := getAuthorizationAlertRules(pr)
	if err != nil {
		return err
	}

	// Convert integer percentages to decimal strings for comparison with Prometheus expressions
	expectedFailureRate := fmt.Sprintf("%.2f", float64(failureRate)/100.0)
	expectedLatencyQuantile := fmt.Sprintf("%.2f", float64(latencyQuantile)/100.0)
	expectedLatencySeconds := fmt.Sprintf("%d", latencySeconds)

	var failures []string
	for _, r := range rules {
		rule, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		alert, _ := rule["alert"].(string)
		expr, _ := rule["expr"].(string)
		switch alert {
		case "HighAuthorizationFailureRate":
			if !strings.Contains(expr, expectedFailureRate) {
				failures = append(failures, fmt.Sprintf("HighAuthorizationFailureRate expr %q does not contain failure rate threshold %s", expr, expectedFailureRate))
			}
		case "HighAuthorizationLatency":
			if strings.Contains(expr, "histogram_quantile") {
				if !strings.Contains(expr, expectedLatencyQuantile) {
					failures = append(failures, fmt.Sprintf("HighAuthorizationLatency expr %q does not contain latency quantile %s", expr, expectedLatencyQuantile))
				}
				if !strings.Contains(expr, expectedLatencySeconds) {
					failures = append(failures, fmt.Sprintf("HighAuthorizationLatency expr %q does not contain latency threshold %s", expr, expectedLatencySeconds))
				}
			}
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("Authorization PrometheusRule threshold validation failed:\n%s", strings.Join(failures, "\n"))
	}
	fmt.Printf("=== Authorization PrometheusRule %s contains expected threshold values ===\n", getAuthorizationPrometheusRuleName(cr))
	return nil
}

// enableAuthorizationPrometheusRule enables spec.modules[*].metrics.prometheusRule.enabled
// for the authorization-proxy-server module.
func (step *Step) enableAuthorizationPrometheusRule(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	for i, m := range found.Spec.Modules {
		if m.Name == csmv1.AuthorizationServer {
			if found.Spec.Modules[i].Metrics == nil {
				found.Spec.Modules[i].Metrics = &csmv1.ModuleMetrics{}
			}
			if found.Spec.Modules[i].Metrics.PrometheusRule == nil {
				found.Spec.Modules[i].Metrics.PrometheusRule = &csmv1.ModulePrometheusRuleConfig{}
			}
			found.Spec.Modules[i].Metrics.PrometheusRule.Enabled = true
			break
		}
	}
	return step.ctrlClient.Update(context.TODO(), found)
}

// disableAuthorizationPrometheusRule disables spec.modules[*].metrics.prometheusRule.enabled
// for the authorization-proxy-server module.
func (step *Step) disableAuthorizationPrometheusRule(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	for i, m := range found.Spec.Modules {
		if m.Name == csmv1.AuthorizationServer {
			if found.Spec.Modules[i].Metrics != nil && found.Spec.Modules[i].Metrics.PrometheusRule != nil {
				found.Spec.Modules[i].Metrics.PrometheusRule.Enabled = false
			}
			break
		}
	}
	return step.ctrlClient.Update(context.TODO(), found)
}

// enableAuthorizationMetrics enables spec.modules[*].metrics.enabled for the
// authorization-proxy-server module.
func (step *Step) enableAuthorizationMetrics(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	for i, m := range found.Spec.Modules {
		if m.Name == csmv1.AuthorizationServer {
			if found.Spec.Modules[i].Metrics == nil {
				found.Spec.Modules[i].Metrics = &csmv1.ModuleMetrics{}
			}
			found.Spec.Modules[i].Metrics.Enabled = true
			break
		}
	}
	return step.ctrlClient.Update(context.TODO(), found)
}

// disableAuthorizationMetrics disables spec.modules[*].metrics.enabled for the
// authorization-proxy-server module.
func (step *Step) disableAuthorizationMetrics(res Resource, crNumStr string) error {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return err
	}
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found); err != nil {
		return err
	}

	for i, m := range found.Spec.Modules {
		if m.Name == csmv1.AuthorizationServer {
			if found.Spec.Modules[i].Metrics != nil {
				found.Spec.Modules[i].Metrics.Enabled = false
			}
			break
		}
	}
	return step.ctrlClient.Update(context.TODO(), found)
}

func prometheusRuleCR(res Resource, crNumStr string) (csmv1.ContainerStorageModule, error) {
	crNum, err := parseIndex(crNumStr)
	if err != nil {
		return csmv1.ContainerStorageModule{}, err
	}
	if crNum < 1 || crNum > len(res.CustomResource) {
		return csmv1.ContainerStorageModule{}, fmt.Errorf("custom resource index %d is out of range", crNum)
	}
	cr, ok := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	if !ok {
		return csmv1.ContainerStorageModule{}, fmt.Errorf("custom resource %d is not a ContainerStorageModule", crNum)
	}
	return cr, nil
}

func (step *Step) getPrometheusRule(res Resource, crNumStr, suffix string) (*unstructured.Unstructured, error) {
	cr, err := prometheusRuleCR(res, crNumStr)
	if err != nil {
		return nil, err
	}
	prometheusRule := &unstructured.Unstructured{}
	prometheusRule.SetAPIVersion("monitoring.coreos.com/v1")
	prometheusRule.SetKind("PrometheusRule")
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{Name: cr.Name + suffix, Namespace: cr.Namespace}, prometheusRule); err != nil {
		return nil, err
	}
	return prometheusRule, nil
}

func prometheusRuleContainsAlert(prometheusRule *unstructured.Unstructured, alertName string) bool {
	groups, found, err := unstructured.NestedSlice(prometheusRule.Object, "spec", "groups")
	if err != nil || !found {
		return false
	}
	for _, group := range groups {
		groupMap, ok := group.(map[string]interface{})
		if !ok {
			continue
		}
		rules, ok := groupMap["rules"].([]interface{})
		if !ok {
			continue
		}
		for _, rule := range rules {
			ruleMap, ok := rule.(map[string]interface{})
			if ok && ruleMap["alert"] == alertName {
				return true
			}
		}
	}
	return false
}

func (step *Step) validateDriverPrometheusRuleContainsAlert(res Resource, alertName, crNumStr string) error {
	prometheusRule, err := step.getPrometheusRule(res, crNumStr, "-alerts")
	if err != nil {
		return err
	}
	if !prometheusRuleContainsAlert(prometheusRule, alertName) {
		return fmt.Errorf("PrometheusRule %s does not contain alert %s", prometheusRule.GetName(), alertName)
	}
	return nil
}

func (step *Step) validateReplicationPrometheusRuleCreated(res Resource, crNumStr string) error {
	_, err := step.getPrometheusRule(res, crNumStr, "-replication-alerts")
	return err
}

func (step *Step) validateReplicationPrometheusRuleContainsAlert(res Resource, alertName, crNumStr string) error {
	prometheusRule, err := step.getPrometheusRule(res, crNumStr, "-replication-alerts")
	if err != nil {
		return err
	}
	if !prometheusRuleContainsAlert(prometheusRule, alertName) {
		return fmt.Errorf("PrometheusRule %s does not contain alert %s", prometheusRule.GetName(), alertName)
	}
	return nil
}

func (step *Step) validateReplicationPrometheusRuleAbsent(res Resource, crNumStr string) error {
	cr, err := prometheusRuleCR(res, crNumStr)
	if err != nil {
		return err
	}
	prometheusRule := &unstructured.Unstructured{}
	prometheusRule.SetAPIVersion("monitoring.coreos.com/v1")
	prometheusRule.SetKind("PrometheusRule")
	err = step.ctrlClient.Get(context.TODO(), client.ObjectKey{Name: cr.Name + "-replication-alerts", Namespace: cr.Namespace}, prometheusRule)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("PrometheusRule %s still exists in namespace %s", prometheusRule.GetName(), cr.Namespace)
}

func (step *Step) enableReplicationPrometheusRule(res Resource, crNumStr string) error {
	cr, err := prometheusRuleCR(res, crNumStr)
	if err != nil {
		return err
	}
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{Name: cr.Name, Namespace: cr.Namespace}, found); err != nil {
		return err
	}
	for i := range found.Spec.Modules {
		module := &found.Spec.Modules[i]
		if module.Name != csmv1.Replication {
			continue
		}
		if module.Metrics == nil {
			module.Metrics = &csmv1.ModuleMetrics{}
		}
		if module.Metrics.PrometheusRule == nil {
			module.Metrics.PrometheusRule = &csmv1.ModulePrometheusRuleConfig{}
		}
		module.Metrics.PrometheusRule.Enabled = true
		return step.ctrlClient.Update(context.TODO(), found)
	}
	return fmt.Errorf("replication module is not configured in CR %s", cr.Name)
}

func (step *Step) disableReplicationPrometheusRule(res Resource, crNumStr string) error {
	cr, err := prometheusRuleCR(res, crNumStr)
	if err != nil {
		return err
	}
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{Name: cr.Name, Namespace: cr.Namespace}, found); err != nil {
		return err
	}
	for i := range found.Spec.Modules {
		module := &found.Spec.Modules[i]
		if module.Name != csmv1.Replication {
			continue
		}
		if module.Metrics == nil || module.Metrics.PrometheusRule == nil {
			return fmt.Errorf("replication PrometheusRule is not configured in CR %s", cr.Name)
		}
		module.Metrics.PrometheusRule.Enabled = false
		return step.ctrlClient.Update(context.TODO(), found)
	}
	return fmt.Errorf("replication module is not configured in CR %s", cr.Name)
}
