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
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	csmv1 "eos2git.cec.lab.emc.com/CSM/csm-operator/api/v1"
	"eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/constants"
	"eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/modules"
	operatorutils "eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/operatorutils"
	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
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
	powerflexAuthSecretMap       = map[string]string{"REPLACE_USER": "POWERFLEX_USER", "REPLACE_PASS": "POWERFLEX_PASS", "REPLACE_SYSTEMID": "POWERFLEX_SYSTEMID", "REPLACE_ENDPOINT": "POWERFLEX_AUTH_ENDPOINT", "REPLACE_MDM": "POWERFLEX_MDM", "REPLACE_PROTOCOL": "POWERFLEX_PROTOCOL"}
	powerscaleSecretMap          = map[string]string{"REPLACE_CLUSTERNAME": "POWERSCALE_CLUSTER", "REPLACE_USER": "POWERSCALE_USER", "REPLACE_PASS": "POWERSCALE_PASS", "REPLACE_ENDPOINT": "POWERSCALE_ENDPOINT", "REPLACE_PORT": "POWERSCALE_PORT", "REPLACE_MULTI_CLUSTERNAME": "POWERSCALE_MULTI_CLUSTER", "REPLACE_MULTI_USER": "POWERSCALE_MULTI_USER", "REPLACE_MULTI_PASS": "POWERSCALE_MULTI_PASS", "REPLACE_MULTI_ENDPOINT": "POWERSCALE_MULTI_ENDPOINT", "REPLACE_MULTI_PORT": "POWERSCALE_MULTI_PORT", "REPLACE_MULTI_AUTH_ENDPOINT": "POWERSCALE_MULTI_AUTH_ENDPOINT", "REPLACE_MULTI_AUTH_PORT": "POWERSCALE_MULTI_AUTH_PORT"} //gosec:disable G101 -- this is a test automation tool
	powerscaleAuthSecretMap      = map[string]string{"REPLACE_CLUSTERNAME": "POWERSCALE_CLUSTER", "REPLACE_USER": "POWERSCALE_USER", "REPLACE_PASS": "POWERSCALE_PASS", "REPLACE_AUTH_ENDPOINT": "POWERSCALE_AUTH_ENDPOINT", "REPLACE_AUTH_PORT": "POWERSCALE_AUTH_PORT", "REPLACE_ENDPOINT": "POWERSCALE_ENDPOINT", "REPLACE_PORT": "POWERSCALE_PORT"}
	powerscaleAuthSidecarMap     = map[string]string{"REPLACE_CLUSTERNAME": "POWERSCALE_CLUSTER", "REPLACE_ENDPOINT": "POWERSCALE_ENDPOINT", "REPLACE_AUTH_ENDPOINT": "POWERSCALE_AUTH_ENDPOINT", "REPLACE_AUTH_PORT": "POWERSCALE_AUTH_PORT", "REPLACE_PORT": "POWERSCALE_PORT"}
	powerscaleEphemeralVolumeMap = map[string]string{"REPLACE_CLUSTERNAME": "POWERSCALE_CLUSTER", "REPLACE_ENDPOINT": "POWERSCALE_ENDPOINT"}
	powerflexEphemeralVolumeMap  = map[string]string{"REPLACE_SYSTEMID": "POWERFLEX_SYSTEMID", "REPLACE_POOL": "POWERFLEX_POOL", "REPLACE_VOLUME": "POWERFLEX_VOLUME"}
	powerflexAuthSidecarMap      = map[string]string{"REPLACE_USER": "POWERFLEX_USER", "REPLACE_PASS": "POWERFLEX_PASS", "REPLACE_SYSTEMID": "POWERFLEX_SYSTEMID", "REPLACE_ENDPOINT": "POWERFLEX_ENDPOINT", "REPLACE_AUTH_ENDPOINT": "POWERFLEX_AUTH_ENDPOINT"}
	powermaxCredMap              = map[string]string{"REPLACE_USER": "POWERMAX_USER_ENCODED", "REPLACE_PASS": "POWERMAX_PASS_ENCODED"} //gosec:disable G101 -- this is a test automation tool
	powermaxSecretMap            = map[string]string{
		"REPLACE_USERNAME": "POWERMAX_USER", "REPLACE_PASSWORD": "POWERMAX_PASS", "REPLACE_SYSTEMID": "POWERMAX_SYSTEMID", "REPLACE_ENDPOINT": "POWERMAX_ENDPOINT",
		"REPLACE_ZONING_USERNAME": "POWERMAX_ZONING_USER", "REPLACE_ZONING_PASSWORD": "POWERMAX_ZONING_PASS", "REPLACE_ZONING_SYSTEMID": "POWERMAX_ZONING_SYSTEMID", "REPLACE_ZONING_ENDPOINT": "POWERMAX_ZONING_ENDPOINT",
	}
	powermaxAuthSidecarMap     = map[string]string{"REPLACE_SYSTEMID": "POWERMAX_SYSTEMID", "REPLACE_ENDPOINT": "POWERMAX_ENDPOINT", "REPLACE_AUTH_ENDPOINT": "POWERMAX_AUTH_ENDPOINT"}
	powermaxStorageMap         = map[string]string{"REPLACE_USER": "POWERMAX_USER", "REPLACE_PASS": "POWERMAX_PASS", "REPLACE_SYSTEMID": "POWERMAX_SYSTEMID", "REPLACE_RESOURCE_POOL": "POWERMAX_POOL_V1", "REPLACE_SERVICE_LEVEL": "POWERMAX_SERVICE_LEVEL"}
	powermaxReverseProxyMap    = map[string]string{"REPLACE_SYSTEMID": "POWERMAX_SYSTEMID", "REPLACE_AUTH_ENDPOINT": "POWERMAX_AUTH_ENDPOINT"}
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
	unitySecretMap               = map[string]string{"REPLACE_USER": "UNITY_USER", "REPLACE_PASS": "UNITY_PASS", "REPLACE_ARRAYID": "UNITY_ARRAYID", "REPLACE_ENDPOINT": "UNITY_ENDPOINT", "REPLACE_POOL": "UNITY_POOL", "REPLACE_NAS": "UNITY_NAS"}
	unityEphemeralVolumeMap      = map[string]string{"REPLACE_ARRAYID": "UNITY_ARRAYID", "REPLACE_POOL": "UNITY_POOL", "REPLACE_NAS": "UNITY_NAS"}

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
	crNum, _ := strconv.Atoi(crNumStr)
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

func (step *Step) applyAuthorizationConjur(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
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
	oldCrNum, _ := strconv.Atoi(oldCrNumStr)
	oldCr := res.CustomResource[oldCrNum-1].(csmv1.ContainerStorageModule)

	newCrNum, _ := strconv.Atoi(newCrNumStr)
	newCr := res.CustomResource[newCrNum-1].(csmv1.ContainerStorageModule)

	time.Sleep(60 * time.Second)

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
		cmd := exec.Command("kubectl", "delete", "-f", "testfiles/cert-manager-crds.yaml") // #nosec G204
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
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return step.ctrlClient.Delete(context.TODO(), &cr)
}

func (step *Step) validateCustomResourceStatus(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
	return checkAllRunningPods(context.TODO(), res.CustomResource[crNum-1].(csmv1.ContainerStorageModule).Namespace, step.clientSet)
}

func (step *Step) validateMinimalCSMDriverSpec(res Resource, driverName string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
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
	crNum, _ := strconv.Atoi(crNumStr)
	return checkNoRunningPods(context.TODO(), res.CustomResource[crNum-1].(csmv1.ContainerStorageModule).Namespace, step.clientSet)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
	return checkNoRunningPods(context.TODO(), res.CustomResource[crNum-1].(csmv1.ContainerStorageModule).Namespace, step.clientSet)
}

func (step *Step) setUpStorageClass(_ Resource, templateFile, crType string) error {
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
		if err != nil {
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
	hostPubKey, err := extractHostPublicKey(string(pubKeyBytes), repoHost)
	if err != nil {
		return err
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
	if err := yaml.Unmarshal([]byte(expanded), &cr); err != nil {
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}

	if found.Spec.Driver.ForceRemoveDriver != nil && !*found.Spec.Driver.ForceRemoveDriver {
		return nil
	}
	return fmt.Errorf("forceRemoveDriver is not set to false")
}

func (step *Step) enableForceRemoveModule(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	found := new(csmv1.ContainerStorageModule)
	if err := step.ctrlClient.Get(context.TODO(), client.ObjectKey{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, found,
	); err != nil {
		return err
	}
	for _, module := range found.Spec.Modules {
		module.ForceRemoveModule = true
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

func (step *Step) createPrereqs(res Resource, module string, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
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

	// provide a brief moment for cluster to settle down
	time.Sleep(5 * time.Second)
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
		cmd = exec.Command("kubectl", "delete", "csm", "-n", cr.Namespace, "--all") // #nosec G204
		b, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to delete all CSM from namespace: %v\nErrMessage:\n%s", err, string(b))
		}

		// Remove finalizers from authorization CRDs that would block namespace deletion
		removeAuthorizationFinalizers(cr.Namespace)

		cmd = exec.Command("kubectl", "delete", "ns", cr.Namespace, "--wait=true", "--timeout=60s") // #nosec G204
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
			_ = exec.Command("kubectl", "delete", item, "-n", namespace, "--wait=false").Run() // #nosec G204,G702
		}
	}
}

// ensureSecretsStoreCSIDriver checks that the secrets-store CSI driver is installed
// and installs it via helm if missing. Authorization tests depend on this driver
// for vault secret synchronization. We check the CSIDriver registration (not just
// CRDs) because CRDs can survive a helm uninstall while the actual driver is gone.
func ensureSecretsStoreCSIDriver() error {
	cmd := exec.Command("kubectl", "get", "csidriver", "secrets-store.csi.k8s.io") // #nosec G204
	if cmd.Run() == nil {
		fmt.Println("secrets-store CSI driver is already installed")
		return nil
	}

	fmt.Println("secrets-store CSI driver not found, installing...")
	// Remove stale helm release if CRDs were deleted but release remains
	_ = exec.Command("helm", "uninstall", "csi-secrets-store", "-n", "kube-system").Run() // #nosec G204

	_ = exec.Command("helm", "repo", "add", "secrets-store-csi-driver", // #nosec G204
		"https://kubernetes-sigs.github.io/secrets-store-csi-driver/charts").Run()

	cmd = exec.Command("helm", "install", "csi-secrets-store", // #nosec G204
		"secrets-store-csi-driver/secrets-store-csi-driver",
		"--wait",
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

	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	var err error
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

	proxyHost = os.Getenv("PROXY_HOST")
	driverNamespace = os.Getenv("DRIVER_NAMESPACE")

	port, err := getPortContainerizedAuth(cr.Namespace)
	if err != nil {
		return err
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
		if err := step.authorizationV2Setup(res, storageType, driver, configVersion, configurationTemplate); err != nil {
			return err
		}
		authV2SetupDone[driver] = true
	} else {
		fmt.Printf("=== Skipping one-time setup for %s (already done) ===\n", driver)
	}

	return step.authorizationV2GenerateAndApplyToken(driver, driverNamespace, proxyHost, port, csmTenantName)
}

// authorizationV2Setup performs the one-time setup: template rendering, admin token creation, and resource application.
func (step *Step) authorizationV2Setup(res Resource, storageType, driver, configVersion string, configurationTemplate string) error {
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

	pathNum, _ := strconv.Atoi(configurationTemplate)
	err := execShell(fmt.Sprintf("mkdir -p temp/authorization-templates && cp %s %s", res.Scenario.Paths[pathNum-1], updatedTemplateFile))
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
		"--jwt-signing-secret", "secret",
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

		if driver == "powerstore" && key == "REPLACE_ENDPOINT" {
			fmt.Println("Replacing PowerStore Endpoint and adding /api/rest/")
			// PowerStore API endpoint requires /api/rest at the end of the URL
			val = val + "/api/rest"
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

	fmt.Println("Waiting 5 seconds before generating token.")
	time.Sleep(5 * time.Second)

	return nil
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
	// Debounce: skip if we recreated less than 60 seconds ago for this driver
	if last, ok := lastAuthCRRecreation[driver]; ok && time.Since(last) < 60*time.Second {
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

	// Wait for deletion to propagate — the tenant-service needs time to process
	// delete events before we create new CRs to avoid a race condition.
	fmt.Println("=== Waiting 15 seconds for delete to propagate ===")
	time.Sleep(15 * time.Second)

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

	// Give the tenant-service time to restart and reconcile the new resources
	fmt.Println("=== Waiting 45 seconds for tenant-service to restart and reconcile ===")
	time.Sleep(45 * time.Second)

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

// Render the Powerflex SFTP CR template into a temporary file with the same name
func (step *Step) configurePowerflexSftpInstall(res Resource, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	crFilePath := res.Scenario.Paths[crNum-1]
	fileString, err := renderTemplate("powerflex", crFilePath)
	if err != nil {
		return err
	}

	filePath, err := writeRenderedFile(crFilePath, fileString)
	if err != nil {
		return err
	}
	fmt.Printf("Rendered template %s into %s\n", crFilePath, filePath)

	return nil
}

func (step *Step) createCustomResourceDefinition(res Resource, crdNumStr string) error {
	crdNum, _ := strconv.Atoi(crdNumStr)
	cmd := exec.Command("kubectl", "apply", "-f", res.Scenario.Paths[crdNum-1]) // #nosec G204
	err := cmd.Run()
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
	crdNum, _ := strconv.Atoi(crdNumStr)
	cmd := exec.Command("kubectl", "delete", "-f", res.Scenario.Paths[crdNum-1]) // #nosec G204
	err := cmd.Run()
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
	cmd := exec.Command("kubectl", "apply", "-f", "testfiles/authorization-templates/csm-images-baseline.yaml") // #nosec G204
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to restore baseline configmap csm-images: %v", err)
	}

	return nil
}

func (step *Step) deleteConfigMap(_ Resource) error {
	cmd := exec.Command("kubectl", "delete", "-f", "testfiles/authorization-templates/csm-images-baseline.yaml") // #nosec G204
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to restore baseline configmap csm-images: %v", err)
	}

	return nil
}

// validateEnvInDriverPod validates environment variables in the generated DaemonSet/Deployment
func (step *Step) validateEnvInDriverPod(res Resource, podType, containerName, envName, expectedValue, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
		envs = &customResource.Spec.Driver.Common.Envs
	case "node":
		envs = &customResource.Spec.Driver.Node.Envs
	case "controller":
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
func (step *Step) setDriverConfigVersionInSpec(res Resource, configVersion, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
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
func (step *Step) setModuleConfigVersionInSpec(res Resource, module, configVersion, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
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
	crNum, _ := strconv.Atoi(crNumStr)
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
func (step *Step) setSpecVersionInSpec(res Resource, version, crNumStr string) error {
	crNum, _ := strconv.Atoi(crNumStr)
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

	customResource.Spec.Version = version

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
	crNum, _ := strconv.Atoi(crNumStr)
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

// validateDeploymentContainerEnvironmentVariable validates that a deployment container's image
// was resolved from the RELATED_IMAGE_* environment variable set on the operator pod.
func (step *Step) validateDeploymentContainerEnvironmentVariable(res Resource, crNumStr string, envVarName string, container string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	staticDp, err := getDriverDeployment(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}

	expectedImage, err := getOperatorPodEnvVar(step.clientSet, envVarName)
	if err != nil {
		return fmt.Errorf("failed to get %s from operator pod: %v", envVarName, err)
	}
	if expectedImage == "" {
		return fmt.Errorf("environment variable %s is not set on the operator pod", envVarName)
	}

	for _, cnt := range staticDp.Spec.Template.Spec.Containers {
		if cnt.Name != container {
			continue
		}

		if cnt.Image == "" {
			return fmt.Errorf("deployment container %s has no image set", container)
		}

		if cnt.Image != expectedImage {
			return fmt.Errorf("expected deployment container %s image to be %s (from %s), got %s", container, expectedImage, envVarName, cnt.Image)
		}
		return nil
	}
	return fmt.Errorf("container %s not found in deployment", container)
}

// validateDaemonSetContainerEnvironmentVariable validates that a daemonset container's image
// was resolved from the RELATED_IMAGE_* environment variable set on the operator pod.
func (step *Step) validateDaemonSetContainerEnvironmentVariable(res Resource, crNumStr string, envVarName string, container string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)
	staticDs, err := getDriverDaemonset(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get daemonset: %v", err)
	}

	expectedImage, err := getOperatorPodEnvVar(step.clientSet, envVarName)
	if err != nil {
		return fmt.Errorf("failed to get %s from operator pod: %v", envVarName, err)
	}
	if expectedImage == "" {
		return fmt.Errorf("environment variable %s is not set on the operator pod", envVarName)
	}

	for _, cnt := range staticDs.Spec.Template.Spec.Containers {
		if cnt.Name != container {
			continue
		}

		if cnt.Image == "" {
			return fmt.Errorf("daemonset container %s has no image set", container)
		}

		if cnt.Image != expectedImage {
			return fmt.Errorf("expected daemonset container %s image to be %s (from %s), got %s", container, expectedImage, envVarName, cnt.Image)
		}
		return nil
	}
	return fmt.Errorf("container %s not found in daemonset", container)
}

// validateSidecarEnvironmentVariable validates that a sidecar container's image
// was resolved from the RELATED_IMAGE_* environment variable set on the operator pod.
func (step *Step) validateSidecarEnvironmentVariable(res Resource, sidecarName string, crNumStr string, envVarName string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	expectedImage, err := getOperatorPodEnvVar(step.clientSet, envVarName)
	if err != nil {
		return fmt.Errorf("failed to get %s from operator pod: %v", envVarName, err)
	}
	if expectedImage == "" {
		return fmt.Errorf("environment variable %s is not set on the operator pod", envVarName)
	}

	// Check both deployment and daemonset for the sidecar
	staticDp, err := getDriverDeployment(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}

	// First check deployment
	for _, cnt := range staticDp.Spec.Template.Spec.Containers {
		if cnt.Name != sidecarName {
			continue
		}

		if cnt.Image != expectedImage {
			return fmt.Errorf("expected sidecar %s image to be %s (from %s), got %s", sidecarName, expectedImage, envVarName, cnt.Image)
		}
		return nil
	}

	// If not found in deployment, check daemonset
	staticDs, err := getDriverDaemonset(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get daemonset: %v", err)
	}

	for _, cnt := range staticDs.Spec.Template.Spec.Containers {
		if cnt.Name != sidecarName {
			continue
		}

		if cnt.Image != expectedImage {
			return fmt.Errorf("expected sidecar %s image to be %s (from %s), got %s", sidecarName, expectedImage, envVarName, cnt.Image)
		}
		return nil
	}

	return fmt.Errorf("sidecar %s not found in deployment or daemonset", sidecarName)
}

// validateDeploymentContainerCustomRegistry validates that a deployment container's image
// was resolved from the RELATED_IMAGE_* env var with custom registry prefix applied.
func (step *Step) validateDeploymentContainerCustomRegistry(res Resource, crNumStr string, envVarName string, container string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	// Check if CR has custom registry
	if cr.Spec.CustomRegistry == "" {
		return fmt.Errorf("expected CR to have custom registry configured")
	}

	envImage, err := getOperatorPodEnvVar(step.clientSet, envVarName)
	if err != nil {
		return fmt.Errorf("failed to get %s from operator pod: %v", envVarName, err)
	}
	if envImage == "" {
		return fmt.Errorf("environment variable %s is not set on the operator pod", envVarName)
	}

	staticDp, err := getDriverDeployment(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}

	for _, cnt := range staticDp.Spec.Template.Spec.Containers {
		if cnt.Name != container {
			continue
		}

		// Check if the image contains the custom registry
		if !strings.Contains(cnt.Image, cr.Spec.CustomRegistry) {
			return fmt.Errorf("expected deployment container %s image to contain custom registry %s, got %s", container, cr.Spec.CustomRegistry, cnt.Image)
		}

		return nil
	}
	return fmt.Errorf("container %s not found in deployment", container)
}

// validateDaemonSetContainerCustomRegistry validates that a daemonset container's image
// was resolved from the RELATED_IMAGE_* env var with custom registry prefix applied.
func (step *Step) validateDaemonSetContainerCustomRegistry(res Resource, crNumStr string, envVarName string, container string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	// Check if CR has custom registry
	if cr.Spec.CustomRegistry == "" {
		return fmt.Errorf("expected CR to have custom registry configured")
	}

	envImage, err := getOperatorPodEnvVar(step.clientSet, envVarName)
	if err != nil {
		return fmt.Errorf("failed to get %s from operator pod: %v", envVarName, err)
	}
	if envImage == "" {
		return fmt.Errorf("environment variable %s is not set on the operator pod", envVarName)
	}

	staticDs, err := getDriverDaemonset(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get daemonset: %v", err)
	}

	for _, cnt := range staticDs.Spec.Template.Spec.Containers {
		if cnt.Name != container {
			continue
		}

		// Check if the image contains the custom registry
		if !strings.Contains(cnt.Image, cr.Spec.CustomRegistry) {
			return fmt.Errorf("expected daemonset container %s image to contain custom registry %s, got %s", container, cr.Spec.CustomRegistry, cnt.Image)
		}

		return nil
	}
	return fmt.Errorf("container %s not found in daemonset", container)
}

// validateDeploymentContainerConfigMapImage validates that a deployment container uses
// ConfigMap image (takes priority over environment variables).
func (step *Step) validateDeploymentContainerConfigMapImage(res Resource, crNumStr string, container string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	staticDp, err := getDriverDeployment(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}

	// Get the env var image to make sure ConfigMap took priority
	envVarKey := strings.TrimPrefix(container, "csi-")
	if envVarKey == "" {
		envVarKey = container
	}
	envImage, _ := getOperatorPodEnvVar(step.clientSet, "RELATED_IMAGE_"+container)

	for _, cnt := range staticDp.Spec.Template.Spec.Containers {
		if cnt.Name != container {
			continue
		}

		// Verify the image is set
		if cnt.Image == "" {
			return fmt.Errorf("expected deployment container %s to have image set from ConfigMap", container)
		}

		// If env var is also set, the images should differ (ConfigMap has priority)
		if envImage != "" && cnt.Image == envImage {
			return fmt.Errorf("deployment container %s image matches environment variable (%s), expected ConfigMap to take priority", container, envImage)
		}

		return nil
	}
	return fmt.Errorf("container %s not found in deployment", container)
}

// validateDaemonSetContainerConfigMapImage validates that a daemonset container uses
// ConfigMap image (takes priority over environment variables).
func (step *Step) validateDaemonSetContainerConfigMapImage(res Resource, crNumStr string, container string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	staticDs, err := getDriverDaemonset(cr, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get daemonset: %v", err)
	}

	// Get the env var image to make sure ConfigMap took priority
	envImage, _ := getOperatorPodEnvVar(step.clientSet, "RELATED_IMAGE_"+container)

	for _, cnt := range staticDs.Spec.Template.Spec.Containers {
		if cnt.Name != container {
			continue
		}

		// Verify the image is set
		if cnt.Image == "" {
			return fmt.Errorf("expected daemonset container %s to have image set from ConfigMap", container)
		}

		// If env var is also set, the images should differ (ConfigMap has priority)
		if envImage != "" && cnt.Image == envImage {
			return fmt.Errorf("daemonset container %s image matches environment variable (%s), expected ConfigMap to take priority", container, envImage)
		}

		return nil
	}
	return fmt.Errorf("container %s not found in daemonset", container)
}

// validateObservabilityDeploymentContainerEnvironmentVariable validates that an observability
// module deployment container's image was resolved from the RELATED_IMAGE_* environment variable.
func (step *Step) validateObservabilityDeploymentContainerEnvironmentVariable(res Resource, crNumStr string, envVarName string, container string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	dp, err := getObservabilityDeployment(cr.Namespace, cr.Spec.Driver.CSIDriverType, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get observability deployment: %v", err)
	}

	expectedImage, err := getOperatorPodEnvVar(step.clientSet, envVarName)
	if err != nil {
		return fmt.Errorf("failed to get %s from operator pod: %v", envVarName, err)
	}
	if expectedImage == "" {
		return fmt.Errorf("environment variable %s is not set on the operator pod", envVarName)
	}

	for _, cnt := range dp.Spec.Template.Spec.Containers {
		if cnt.Name != container {
			continue
		}

		if cnt.Image == "" {
			return fmt.Errorf("observability deployment container %s has no image set", container)
		}

		if cnt.Image != expectedImage {
			return fmt.Errorf("expected observability deployment container %s image to be %s (from %s), got %s", container, expectedImage, envVarName, cnt.Image)
		}
		return nil
	}
	return fmt.Errorf("container %s not found in observability deployment", container)
}

// validateObservabilityDeploymentContainerCustomRegistry validates that an observability
// module deployment container's image was resolved with custom registry prefix applied.
func (step *Step) validateObservabilityDeploymentContainerCustomRegistry(res Resource, crNumStr string, envVarName string, container string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	if cr.Spec.CustomRegistry == "" {
		return fmt.Errorf("expected CR to have custom registry configured")
	}

	envImage, err := getOperatorPodEnvVar(step.clientSet, envVarName)
	if err != nil {
		return fmt.Errorf("failed to get %s from operator pod: %v", envVarName, err)
	}
	if envImage == "" {
		return fmt.Errorf("environment variable %s is not set on the operator pod", envVarName)
	}

	dp, err := getObservabilityDeployment(cr.Namespace, cr.Spec.Driver.CSIDriverType, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get observability deployment: %v", err)
	}

	for _, cnt := range dp.Spec.Template.Spec.Containers {
		if cnt.Name != container {
			continue
		}

		if !strings.Contains(cnt.Image, cr.Spec.CustomRegistry) {
			return fmt.Errorf("expected observability deployment container %s image to contain custom registry %s, got %s", container, cr.Spec.CustomRegistry, cnt.Image)
		}

		return nil
	}
	return fmt.Errorf("container %s not found in observability deployment", container)
}

// validateObservabilityDeploymentContainerConfigMapImage validates that an observability
// module deployment container uses ConfigMap image (takes priority over environment variables).
func (step *Step) validateObservabilityDeploymentContainerConfigMapImage(res Resource, crNumStr string, container string) error {
	crNum, _ := strconv.Atoi(crNumStr)
	cr := res.CustomResource[crNum-1].(csmv1.ContainerStorageModule)

	dp, err := getObservabilityDeployment(cr.Namespace, cr.Spec.Driver.CSIDriverType, step.ctrlClient)
	if err != nil {
		return fmt.Errorf("failed to get observability deployment: %v", err)
	}

	envImage, _ := getOperatorPodEnvVar(step.clientSet, "RELATED_IMAGE_"+container)

	for _, cnt := range dp.Spec.Template.Spec.Containers {
		if cnt.Name != container {
			continue
		}

		if cnt.Image == "" {
			return fmt.Errorf("expected observability deployment container %s to have image set from ConfigMap", container)
		}

		if envImage != "" && cnt.Image == envImage {
			return fmt.Errorf("observability deployment container %s image matches environment variable (%s), expected ConfigMap to take priority", container, envImage)
		}

		return nil
	}
	return fmt.Errorf("container %s not found in observability deployment", container)
}
